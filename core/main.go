package core

import (
	"context"
	"sync"

	"github.com/alireza0/s-ui/util/common"

	sb "github.com/sagernet/sing-box"
	_ "github.com/sagernet/sing-box/experimental/clashapi"
	_ "github.com/sagernet/sing-box/experimental/v2rayapi"
	"github.com/sagernet/sing-box/option"
	_ "github.com/sagernet/sing-box/transport/v2rayquic"
)

// Core owns the running sing-box instance. Everything mutable lives behind mu,
// and the managers are read off the Box rather than cached in package vars, so
// a caller cannot mix managers from one box with an instance from another.
//
// RWMutex because the panel's status poll calls IsRunning on every request and
// must not queue behind a start.
//
// Lock ordering: service.lifecycleMu (outer) before Core.mu (inner).
type Core struct {
	mu        sync.RWMutex
	isRunning bool
	instance  *Box

	// Carries the protocol registries. Built once in NewCore and never
	// reassigned, so it needs no lock.
	ctx context.Context
}

func NewCore() *Core {
	return NewCoreWithContext(context.Background())
}

// NewCoreWithContext lets standalone users bind asynchronous provider work to
// the lifetime of one runtime instance. The panel's default constructor stays
// unchanged; a replacement node runtime must cancel its retired instance.
func NewCoreWithContext(parent context.Context) *Core {
	ctx := sb.Context(parent, InboundRegistry(), OutboundRegistry(),
		EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())
	return &Core{ctx: ctx}
}

func (c *Core) GetCtx() context.Context {
	return c.ctx
}

func (c *Core) GetInstance() *Box {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instance
}

func (c *Core) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

// running returns the live box, or an error when the core is stopped. One
// RLock, so a caller cannot see isRunning true and then find instance nil.
func (c *Core) running() (*Box, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.isRunning || c.instance == nil {
		return nil, common.NewError("sing-box is not running")
	}
	return c.instance, nil
}

// Start builds a box from the config and publishes it. The build and the start
// run outside the lock -- they bind listeners and take seconds -- and only the
// publish is locked, so a half-built box is never visible.
func (c *Core) Start(sbConfig []byte) error {
	var opt option.Options
	// Returned, not just logged: an empty option set starts a box with zero
	// inbounds that reports itself healthy, so the watchdog never retries.
	if err := opt.UnmarshalJSONContext(c.ctx, sbConfig); err != nil {
		return common.NewErrorf("unmarshal config: %v", err)
	}

	instance, err := NewBox(Options{
		Context: c.ctx,
		Options: opt,
	})
	if err != nil {
		return err
	}

	if err = instance.Start(); err != nil {
		_ = instance.Close()
		return err
	}

	c.mu.Lock()
	c.instance = instance
	c.isRunning = true
	c.mu.Unlock()
	return nil
}

// Stop detaches the instance under the lock and closes it outside, since
// Box.Close shuts down ten manager subsystems in sequence.
func (c *Core) Stop() error {
	c.mu.Lock()
	instance := c.instance
	c.instance = nil
	c.isRunning = false
	c.mu.Unlock()

	if instance == nil {
		return nil
	}
	return instance.Close()
}
