// Package manager holds copies of sing-box's inbound, outbound, endpoint and
// service managers. Upstream dropped hot add and remove from them in v1.14.3;
// the panel edits a running core, so these keep both on top of the scope
// lifecycle that replaced Close.
package manager

import (
	"context"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	E "github.com/sagernet/sing/common/exceptions"
)

// componentScopes gives every component its own scope under the manager's, so
// a single one can be closed on removal without tearing down the rest.
type componentScopes struct {
	logger  log.ContextLogger
	access  sync.Mutex
	ctx     context.Context
	started bool
	stage   adapter.StartStage
	scopes  map[adapter.Lifecycle]*adapter.Scope
	order   []adapter.Lifecycle
}

func newComponentScopes(logger log.ContextLogger) componentScopes {
	return componentScopes{
		logger: logger,
		scopes: make(map[adapter.Lifecycle]*adapter.Scope),
	}
}

// attach records the stage the manager reached. The first call ties every
// component scope to the manager's scope, which closes them all with it.
func (s *componentScopes) attach(stage adapter.StartStage, scope *adapter.Scope) {
	s.access.Lock()
	defer s.access.Unlock()
	if !s.started {
		s.started = true
		s.ctx = scope.Context()
		scope.Add(s.closeAll)
	}
	s.stage = stage
}

// current reports whether the manager has started and the last stage it ran.
func (s *componentScopes) current() (bool, adapter.StartStage) {
	s.access.Lock()
	defer s.access.Unlock()
	return s.started, s.stage
}

func (s *componentScopes) start(name string, component adapter.Lifecycle, stage adapter.StartStage) error {
	s.access.Lock()
	if s.ctx == nil {
		s.access.Unlock()
		return E.New("start ", name, ": manager not started")
	}
	scope, loaded := s.scopes[component]
	if !loaded {
		scope = adapter.NewScope(s.ctx, s.logger)
		s.scopes[component] = scope
		s.order = append(s.order, component)
	}
	s.access.Unlock()
	return scope.Start(name, component, stage)
}

// startHot runs a component created on a running manager through every stage
// the manager has already passed, closing it again if one fails.
func (s *componentScopes) startHot(name string, component adapter.Lifecycle, current adapter.StartStage) error {
	for _, stage := range adapter.ListStartStages {
		if stage > current {
			break
		}
		err := s.start(name, component, stage)
		if err != nil {
			return E.Append(err, s.close(component), func(err error) error {
				return E.Cause(err, "close ", name)
			})
		}
	}
	return nil
}

func (s *componentScopes) close(component adapter.Lifecycle) error {
	s.access.Lock()
	scope, loaded := s.scopes[component]
	if loaded {
		delete(s.scopes, component)
		s.order = slices.DeleteFunc(s.order, func(it adapter.Lifecycle) bool {
			return it == component
		})
	}
	s.access.Unlock()
	if !loaded {
		return nil
	}
	return scope.Close()
}

func (s *componentScopes) closeAll() error {
	s.access.Lock()
	order := s.order
	scopes := s.scopes
	s.order = nil
	s.scopes = make(map[adapter.Lifecycle]*adapter.Scope)
	s.access.Unlock()
	var err error
	for _, component := range slices.Backward(order) {
		err = E.Errors(err, scopes[component].Close())
	}
	return err
}
