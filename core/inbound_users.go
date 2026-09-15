package core

import (
	suiAnytls "github.com/alireza0/s-ui/core/protocol/anytls"
	suiHysteria "github.com/alireza0/s-ui/core/protocol/hysteria"
	suiHysteria2 "github.com/alireza0/s-ui/core/protocol/hysteria2"
	suiShadowsocks "github.com/alireza0/s-ui/core/protocol/shadowsocks"
	suiSnell "github.com/alireza0/s-ui/core/protocol/snell"
	suiTrojan "github.com/alireza0/s-ui/core/protocol/trojan"
	suiTuic "github.com/alireza0/s-ui/core/protocol/tuic"
	suiVless "github.com/alireza0/s-ui/core/protocol/vless"
	suiVmess "github.com/alireza0/s-ui/core/protocol/vmess"

	"github.com/alireza0/s-ui/core/usersession"

	"github.com/sagernet/sing-box/option"
	sbCommon "github.com/sagernet/sing/common"
)

func (c *Core) UpdateInboundUsers(config []byte) (bool, error) {
	box, err := c.running()
	if err != nil {
		return false, err
	}
	var inboundConfig option.Inbound
	if err = inboundConfig.UnmarshalJSONContext(box.ctx, config); err != nil {
		return false, err
	}
	inb, found := box.inbound.Get(inboundConfig.Tag)
	if !found {
		return false, nil
	}
	switch options := inboundConfig.Options.(type) {
	case *option.VLESSInboundOptions:
		if in, ok := inb.(*suiVless.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.VMessInboundOptions:
		if in, ok := inb.(*suiVmess.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.TrojanInboundOptions:
		if in, ok := inb.(*suiTrojan.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.TUICInboundOptions:
		if in, ok := inb.(*suiTuic.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.HysteriaInboundOptions:
		if in, ok := inb.(*suiHysteria.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.Hysteria2InboundOptions:
		if in, ok := inb.(*suiHysteria2.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.AnyTLSInboundOptions:
		if in, ok := inb.(*suiAnytls.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.SnellInboundOptions:
		if len(options.Users) == 0 {
			return false, nil
		}
		if in, ok := inb.(*suiSnell.Inbound); ok {
			return true, in.UpdateUsers(options.Users)
		}
	case *option.ShadowsocksInboundOptions:
		if options.Managed || len(options.Users) == 0 {
			return false, nil
		}
		if in, ok := inb.(*suiShadowsocks.MultiInbound); ok {
			return true, in.UpdateUsers(sbCommon.Map(options.Users, func(it option.ShadowsocksUser) string {
				return it.Name
			}), sbCommon.Map(options.Users, func(it option.ShadowsocksUser) string {
				return it.Password
			}))
		}
	}
	return false, nil
}

// CloseInboundUserSessions cuts the sessions of users that are no longer
// enabled on the inbound. Protocols that authenticate once per session keep
// serving a removed user until the session itself is cut, so closing the routed
// connections is not enough for them.
func (c *Core) CloseInboundUserSessions(tag string, keep map[string]struct{}) int {
	box, err := c.running()
	if err != nil {
		return 0
	}
	inb, found := box.inbound.Get(tag)
	if !found {
		return 0
	}
	closer, ok := inb.(usersession.Closer)
	if !ok {
		return 0
	}
	return closer.CloseUserSessions(keep)
}

// KickUserSessions cuts the protocol-level sessions of one user across every
// inbound that has them, for a disconnect asked for from the panel. Unlike a
// removal, nothing is blocked afterwards: the client may connect again.
func (c *Core) KickUserSessions(user string) int {
	box, err := c.running()
	if err != nil {
		return 0
	}
	kicked := 0
	for _, inb := range box.inbound.Inbounds() {
		if closer, ok := inb.(usersession.Closer); ok {
			kicked += closer.KickUserSessions(user)
		}
	}
	return kicked
}
