package core

import (
	"s-ui/logger"
	"s-ui/util/common"

	"github.com/sagernet/sing-box/option"
)

func (c *Core) AddInbound(config []byte) error {
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var inbound_config option.Inbound
	err = inbound_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	err = inbound_manager.Create(
		globalCtx,
		router,
		factory.NewLogger("inbound/"+inbound_config.Type+"["+inbound_config.Tag+"]"),
		inbound_config.Tag,
		inbound_config.Type,
		inbound_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveInbound(tag string) error {
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove inbound: ", tag)
	return inbound_manager.Remove(tag)
}

func (c *Core) AddOutbound(config []byte) error {
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var outbound_config option.Outbound

	err = outbound_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	err = outbound_manager.Create(
		globalCtx,
		router,
		factory.NewLogger("outbound/"+outbound_config.Type+"["+outbound_config.Tag+"]"),
		outbound_config.Tag,
		outbound_config.Type,
		outbound_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveOutbound(tag string) error {
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove outbound: ", tag)
	return outbound_manager.Remove(tag)
}

func (c *Core) AddEndpoint(config []byte) error {
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	var err error
	var endpoint_config option.Endpoint

	err = endpoint_config.UnmarshalJSONContext(globalCtx, config)
	if err != nil {
		return err
	}

	err = endpoint_manager.Create(
		globalCtx,
		router,
		factory.NewLogger("endpoint/"+endpoint_config.Type+"["+endpoint_config.Tag+"]"),
		endpoint_config.Tag,
		endpoint_config.Type,
		endpoint_config.Options)
	if err != nil {
		return err
	}

	return nil
}

func (c *Core) RemoveEndpoint(tag string) error {
	if !c.isRunning {
		return common.NewError("sing-box is not running")
	}
	logger.Info("remove endpoint: ", tag)
	return endpoint_manager.Remove(tag)
}
