package core

import (
	"github.com/alireza0/s-ui/logger"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
)

// Each method below takes the live box once via running() and then works from
// that single reference. The managers come off the box rather than from
// package-level vars, so an add cannot be applied to a manager belonging to a
// box that has since been replaced by a restart.

func (c *Core) AddInbound(config []byte) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	var inbound_config option.Inbound
	if err = inbound_config.UnmarshalJSONContext(box.scope.Context(), config); err != nil {
		return err
	}

	return box.inbound.Create(
		box.scope.Context(),
		box.router,
		box.logFactory.NewLogger("inbound/"+inbound_config.Type+"["+inbound_config.Tag+"]"),
		inbound_config.Tag,
		inbound_config.Type,
		inbound_config.Options)
}

func (c *Core) RemoveInbound(tag string) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	logger.Info("remove inbound: ", tag)
	return box.inbound.Remove(tag)
}

func (c *Core) AddOutbound(config []byte) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	var outbound_config option.Outbound
	if err = outbound_config.UnmarshalJSONContext(box.scope.Context(), config); err != nil {
		return err
	}

	outboundCtx := adapter.WithContext(box.scope.Context(), &adapter.InboundContext{
		Outbound: outbound_config.Tag,
	})

	return box.outbound.Create(
		outboundCtx,
		box.router,
		box.logFactory.NewLogger("outbound/"+outbound_config.Type+"["+outbound_config.Tag+"]"),
		outbound_config.Tag,
		outbound_config.Type,
		outbound_config.Options)
}

func (c *Core) RemoveOutbound(tag string) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	logger.Info("remove outbound: ", tag)
	return box.outbound.Remove(tag)
}

func (c *Core) AddEndpoint(config []byte) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	var endpoint_config option.Endpoint
	if err = endpoint_config.UnmarshalJSONContext(box.scope.Context(), config); err != nil {
		return err
	}

	return box.endpoint.Create(
		box.scope.Context(),
		box.router,
		box.logFactory.NewLogger("endpoint/"+endpoint_config.Type+"["+endpoint_config.Tag+"]"),
		endpoint_config.Tag,
		endpoint_config.Type,
		endpoint_config.Options)
}

func (c *Core) RemoveEndpoint(tag string) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	logger.Info("remove endpoint: ", tag)
	return box.endpoint.Remove(tag)
}

func (c *Core) AddService(config []byte) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	var srv_config option.Service
	if err = srv_config.UnmarshalJSONContext(box.scope.Context(), config); err != nil {
		return err
	}

	return box.service.Create(
		box.scope.Context(),
		box.logFactory.NewLogger("service/"+srv_config.Type+"["+srv_config.Tag+"]"),
		srv_config.Tag,
		srv_config.Type,
		srv_config.Options)
}

func (c *Core) RemoveService(tag string) error {
	box, err := c.running()
	if err != nil {
		return err
	}
	logger.Info("remove service: ", tag)
	return box.service.Remove(tag)
}
