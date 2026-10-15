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

var _ adapter.ServiceManager = (*ServiceManager)(nil)

type ServiceManager struct {
	registry     adapter.ServiceRegistry
	scopes       componentScopes
	access       sync.Mutex
	services     []adapter.Service
	serviceByTag map[string]adapter.Service
}

func NewServiceManager(logger log.ContextLogger, registry adapter.ServiceRegistry) *ServiceManager {
	return &ServiceManager{
		registry:     registry,
		scopes:       newComponentScopes(logger),
		serviceByTag: make(map[string]adapter.Service),
	}
}

func serviceName(service adapter.Service) string {
	return "service/" + service.Type() + "[" + service.Tag() + "]"
}

func (m *ServiceManager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.scopes.attach(stage, scope)
	m.access.Lock()
	services := m.services
	m.access.Unlock()
	for _, service := range services {
		err := m.scopes.start(serviceName(service), service, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *ServiceManager) Services() []adapter.Service {
	m.access.Lock()
	defer m.access.Unlock()
	return m.services
}

func (m *ServiceManager) Get(tag string) (adapter.Service, bool) {
	m.access.Lock()
	service, found := m.serviceByTag[tag]
	m.access.Unlock()
	return service, found
}

func (m *ServiceManager) Remove(tag string) error {
	m.access.Lock()
	service, found := m.serviceByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	delete(m.serviceByTag, tag)
	m.services = slices.DeleteFunc(slices.Clone(m.services), func(it adapter.Service) bool {
		return it == service
	})
	m.access.Unlock()
	return m.scopes.close(service)
}

func (m *ServiceManager) Create(ctx context.Context, logger log.ContextLogger, tag string, serviceType string, options any) error {
	service, err := m.registry.Create(ctx, logger, tag, serviceType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	_, loaded := m.serviceByTag[tag]
	m.access.Unlock()
	if loaded {
		return E.New("duplicate service tag: ", tag)
	}
	// Started outside the lock: starting may look components up again.
	if started, stage := m.scopes.current(); started {
		err = m.scopes.startHot(serviceName(service), service, stage)
		if err != nil {
			return err
		}
	}
	m.access.Lock()
	defer m.access.Unlock()
	if _, loaded = m.serviceByTag[tag]; loaded {
		return E.Append(E.New("duplicate service tag: ", tag), m.scopes.close(service), func(err error) error {
			return E.Cause(err, "close ", serviceName(service))
		})
	}
	m.services = append(slices.Clone(m.services), service)
	m.serviceByTag[tag] = service
	return nil
}
