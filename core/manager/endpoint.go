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

var _ adapter.EndpointManager = (*EndpointManager)(nil)

type EndpointManager struct {
	registry      adapter.EndpointRegistry
	scopes        componentScopes
	access        sync.Mutex
	endpoints     []adapter.Endpoint
	endpointByTag map[string]adapter.Endpoint
}

func NewEndpointManager(logger log.ContextLogger, registry adapter.EndpointRegistry) *EndpointManager {
	return &EndpointManager{
		registry:      registry,
		scopes:        newComponentScopes(logger),
		endpointByTag: make(map[string]adapter.Endpoint),
	}
}

func endpointName(endpoint adapter.Endpoint) string {
	return "endpoint/" + endpoint.Type() + "[" + endpoint.Tag() + "]"
}

func (m *EndpointManager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.scopes.attach(stage, scope)
	// The outbound manager starts endpoints alongside outbounds, in dependency
	// order, through StartEndpoint.
	if stage == adapter.StartStateStart {
		return nil
	}
	m.access.Lock()
	endpoints := m.endpoints
	m.access.Unlock()
	for _, endpoint := range endpoints {
		err := m.scopes.start(endpointName(endpoint), endpoint, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *EndpointManager) StartEndpoint(endpoint adapter.Endpoint) error {
	return m.scopes.start(endpointName(endpoint), endpoint, adapter.StartStateStart)
}

func (m *EndpointManager) Endpoints() []adapter.Endpoint {
	m.access.Lock()
	defer m.access.Unlock()
	return m.endpoints
}

func (m *EndpointManager) Get(tag string) (adapter.Endpoint, bool) {
	m.access.Lock()
	defer m.access.Unlock()
	endpoint, found := m.endpointByTag[tag]
	return endpoint, found
}

func (m *EndpointManager) Remove(tag string) error {
	m.access.Lock()
	endpoint, found := m.endpointByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	delete(m.endpointByTag, tag)
	m.endpoints = slices.DeleteFunc(slices.Clone(m.endpoints), func(it adapter.Endpoint) bool {
		return it == endpoint
	})
	m.access.Unlock()
	return m.scopes.close(endpoint)
}

func (m *EndpointManager) Create(ctx context.Context, router adapter.Router, logger log.ContextLogger, tag string, endpointType string, options any) error {
	endpoint, err := m.registry.Create(ctx, router, logger, tag, endpointType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	_, loaded := m.endpointByTag[tag]
	m.access.Unlock()
	if loaded {
		return E.New("duplicate endpoint tag: ", tag)
	}
	// Started outside the lock: starting may look components up again.
	if started, stage := m.scopes.current(); started {
		err = m.scopes.startHot(endpointName(endpoint), endpoint, stage)
		if err != nil {
			return err
		}
	}
	m.access.Lock()
	defer m.access.Unlock()
	if _, loaded = m.endpointByTag[tag]; loaded {
		return E.Append(E.New("duplicate endpoint tag: ", tag), m.scopes.close(endpoint), func(err error) error {
			return E.Cause(err, "close ", endpointName(endpoint))
		})
	}
	m.endpoints = append(slices.Clone(m.endpoints), endpoint)
	m.endpointByTag[tag] = endpoint
	return nil
}
