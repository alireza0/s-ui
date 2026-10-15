package manager

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.OutboundManager = (*OutboundManager)(nil)

type OutboundManager struct {
	logger                  log.ContextLogger
	registry                adapter.OutboundRegistry
	endpoint                adapter.EndpointManager
	defaultTag              string
	scopes                  componentScopes
	access                  sync.RWMutex
	outbounds               []adapter.Outbound
	outboundByTag           map[string]adapter.Outbound
	defaultOutbound         adapter.Outbound
	defaultOutboundFallback func() (adapter.Outbound, error)
}

func NewOutboundManager(logger log.ContextLogger, registry adapter.OutboundRegistry, endpoint adapter.EndpointManager, defaultTag string) *OutboundManager {
	return &OutboundManager{
		logger:        logger,
		registry:      registry,
		endpoint:      endpoint,
		defaultTag:    defaultTag,
		scopes:        newComponentScopes(logger),
		outboundByTag: make(map[string]adapter.Outbound),
	}
}

func outboundName(outbound adapter.Outbound) string {
	return "outbound/" + outbound.Type() + "[" + outbound.Tag() + "]"
}

func (m *OutboundManager) Initialize(defaultOutboundFallback func() (adapter.Outbound, error)) {
	m.defaultOutboundFallback = defaultOutboundFallback
}

func (m *OutboundManager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.scopes.attach(stage, scope)
	m.access.Lock()
	if stage == adapter.StartStateInitialize {
		if m.defaultTag != "" && m.defaultOutbound == nil {
			defaultEndpoint, loaded := m.endpoint.Get(m.defaultTag)
			if !loaded {
				m.access.Unlock()
				return E.New("default outbound not found: ", m.defaultTag)
			}
			m.defaultOutbound = defaultEndpoint
		}
		if m.defaultOutbound == nil {
			directOutbound, err := m.defaultOutboundFallback()
			if err != nil {
				m.access.Unlock()
				return E.Cause(err, "create direct outbound for fallback")
			}
			m.outbounds = append(m.outbounds, directOutbound)
			m.outboundByTag[directOutbound.Tag()] = directOutbound
			m.defaultOutbound = directOutbound
		}
	}
	outbounds := m.outbounds
	m.access.Unlock()
	if stage == adapter.StartStateStart {
		return m.startOutbounds(append(slices.Clone(outbounds), common.Map(m.endpoint.Endpoints(), func(it adapter.Endpoint) adapter.Outbound { return it })...))
	}
	for _, outbound := range outbounds {
		lifecycle, isLifecycle := outbound.(adapter.Lifecycle)
		if !isLifecycle {
			continue
		}
		err := m.scopes.start(outboundName(outbound), lifecycle, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *OutboundManager) startOutbounds(outbounds []adapter.Outbound) error {
	started := make(map[string]bool)
	for {
		canContinue := false
	startOne:
		for _, outboundToStart := range outbounds {
			outboundTag := outboundToStart.Tag()
			if started[outboundTag] {
				continue
			}
			dependencies := outboundToStart.Dependencies()
			for _, dependency := range dependencies {
				if !started[dependency] {
					continue startOne
				}
			}
			started[outboundTag] = true
			canContinue = true
			if endpoint, isEndpoint := outboundToStart.(adapter.Endpoint); isEndpoint {
				err := m.endpoint.StartEndpoint(endpoint)
				if err != nil {
					return err
				}
				continue
			}
			lifecycle, isLifecycle := outboundToStart.(adapter.Lifecycle)
			if !isLifecycle {
				continue
			}
			err := m.scopes.start(outboundName(outboundToStart), lifecycle, adapter.StartStateStart)
			if err != nil {
				return err
			}
		}
		if len(started) == len(outbounds) {
			break
		}
		if canContinue {
			continue
		}
		currentOutbound := common.Find(outbounds, func(it adapter.Outbound) bool {
			return !started[it.Tag()]
		})
		var lintOutbound func(oTree []string, oCurrent adapter.Outbound) error
		lintOutbound = func(oTree []string, oCurrent adapter.Outbound) error {
			problemOutboundTag := common.Find(oCurrent.Dependencies(), func(it string) bool {
				return !started[it]
			})
			if common.Contains(oTree, problemOutboundTag) {
				return E.New("circular outbound dependency: ", strings.Join(oTree, " -> "), " -> ", problemOutboundTag)
			}
			problemOutbound := common.Find(outbounds, func(it adapter.Outbound) bool {
				return it.Tag() == problemOutboundTag
			})
			if problemOutbound == nil {
				return E.New("dependency[", problemOutboundTag, "] not found for outbound[", oCurrent.Tag(), "]")
			}
			return lintOutbound(append(oTree, problemOutboundTag), problemOutbound)
		}
		return lintOutbound([]string{currentOutbound.Tag()}, currentOutbound)
	}
	return nil
}

func (m *OutboundManager) Outbounds() []adapter.Outbound {
	m.access.RLock()
	defer m.access.RUnlock()
	return m.outbounds
}

func (m *OutboundManager) Outbound(tag string) (adapter.Outbound, bool) {
	m.access.RLock()
	outbound, found := m.outboundByTag[tag]
	m.access.RUnlock()
	if found {
		return outbound, true
	}
	return m.endpoint.Get(tag)
}

func (m *OutboundManager) Default() adapter.Outbound {
	m.access.RLock()
	defer m.access.RUnlock()
	return m.defaultOutbound
}

// Remove refuses an outbound that another one still depends on, since the
// dependent keeps dialing through it.
func (m *OutboundManager) Remove(tag string) error {
	m.access.Lock()
	outbound, found := m.outboundByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	var dependBy []string
	for _, it := range m.outbounds {
		if common.Contains(it.Dependencies(), tag) {
			dependBy = append(dependBy, it.Tag())
		}
	}
	for _, it := range m.endpoint.Endpoints() {
		if common.Contains(it.Dependencies(), tag) {
			dependBy = append(dependBy, it.Tag())
		}
	}
	if len(dependBy) > 0 {
		m.access.Unlock()
		return E.New("outbound[", tag, "] is depended by ", strings.Join(dependBy, ", "))
	}
	delete(m.outboundByTag, tag)
	m.outbounds = slices.DeleteFunc(slices.Clone(m.outbounds), func(it adapter.Outbound) bool {
		return it == outbound
	})
	if m.defaultOutbound == outbound {
		if len(m.outbounds) > 0 {
			m.defaultOutbound = m.outbounds[0]
			m.logger.Info("updated default outbound to ", m.defaultOutbound.Tag())
		} else {
			m.defaultOutbound = nil
		}
	}
	m.access.Unlock()
	lifecycle, isLifecycle := outbound.(adapter.Lifecycle)
	if !isLifecycle {
		return nil
	}
	return m.scopes.close(lifecycle)
}

func (m *OutboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, outboundType string, options any) error {
	if tag == "" {
		return os.ErrInvalid
	}
	outbound, err := m.registry.CreateOutbound(ctx, router, logger, tag, outboundType, options)
	if err != nil {
		return err
	}
	m.access.RLock()
	_, loaded := m.outboundByTag[tag]
	m.access.RUnlock()
	if loaded {
		return E.New("duplicate outbound tag: ", tag)
	}
	lifecycle, isLifecycle := outbound.(adapter.Lifecycle)
	// Started outside the lock: starting looks the dependencies up again.
	if started, stage := m.scopes.current(); started && isLifecycle {
		err = m.scopes.startHot(outboundName(outbound), lifecycle, stage)
		if err != nil {
			return err
		}
	}
	m.access.Lock()
	defer m.access.Unlock()
	if _, loaded = m.outboundByTag[tag]; loaded {
		err = E.New("duplicate outbound tag: ", tag)
		if isLifecycle {
			err = E.Append(err, m.scopes.close(lifecycle), func(err error) error {
				return E.Cause(err, "close ", outboundName(outbound))
			})
		}
		return err
	}
	m.outbounds = append(slices.Clone(m.outbounds), outbound)
	m.outboundByTag[tag] = outbound
	if tag == m.defaultTag || (m.defaultTag == "" && m.defaultOutbound == nil) {
		m.defaultOutbound = outbound
		if started, _ := m.scopes.current(); started {
			m.logger.Info("updated default outbound to ", outbound.Tag())
		}
	}
	return nil
}
