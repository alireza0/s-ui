package manager

import (
	"context"
	"os"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.InboundManager = (*InboundManager)(nil)

type InboundManager struct {
	registry     adapter.InboundRegistry
	endpoint     adapter.EndpointManager
	scopes       componentScopes
	access       sync.Mutex
	inbounds     []adapter.Inbound
	inboundByTag map[string]adapter.Inbound
}

func NewInboundManager(logger log.ContextLogger, registry adapter.InboundRegistry, endpoint adapter.EndpointManager) *InboundManager {
	return &InboundManager{
		registry:     registry,
		endpoint:     endpoint,
		scopes:       newComponentScopes(logger),
		inboundByTag: make(map[string]adapter.Inbound),
	}
}

func inboundName(inbound adapter.Inbound) string {
	return "inbound/" + inbound.Type() + "[" + inbound.Tag() + "]"
}

func (m *InboundManager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.scopes.attach(stage, scope)
	m.access.Lock()
	inbounds := m.inbounds
	m.access.Unlock()
	for _, inbound := range inbounds {
		err := m.scopes.start(inboundName(inbound), inbound, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *InboundManager) Inbounds() []adapter.Inbound {
	m.access.Lock()
	defer m.access.Unlock()
	return m.inbounds
}

func (m *InboundManager) Get(tag string) (adapter.Inbound, bool) {
	m.access.Lock()
	inbound, found := m.inboundByTag[tag]
	m.access.Unlock()
	if found {
		return inbound, true
	}
	return m.endpoint.Get(tag)
}

func (m *InboundManager) Remove(tag string) error {
	m.access.Lock()
	inbound, found := m.inboundByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	delete(m.inboundByTag, tag)
	m.inbounds = slices.DeleteFunc(slices.Clone(m.inbounds), func(it adapter.Inbound) bool {
		return it == inbound
	})
	m.access.Unlock()
	return m.scopes.close(inbound)
}

func (m *InboundManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, inboundType string, options any) error {
	inbound, err := m.registry.Create(ctx, router, logger, tag, inboundType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	_, loaded := m.inboundByTag[tag]
	m.access.Unlock()
	if loaded {
		return E.New("duplicate inbound tag: ", tag)
	}
	// Started outside the lock: starting may look components up again.
	if started, stage := m.scopes.current(); started {
		err = m.scopes.startHot(inboundName(inbound), inbound, stage)
		if err != nil {
			return err
		}
	}
	m.access.Lock()
	defer m.access.Unlock()
	if _, loaded = m.inboundByTag[tag]; loaded {
		return E.Append(E.New("duplicate inbound tag: ", tag), m.scopes.close(inbound), func(err error) error {
			return E.Cause(err, "close ", inboundName(inbound))
		})
	}
	m.inbounds = append(slices.Clone(m.inbounds), inbound)
	m.inboundByTag[tag] = inbound
	return nil
}
