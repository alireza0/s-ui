//go:build with_naive_outbound

package core

import (
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/protocol/naive"
	_ "github.com/sagernet/sing-box/protocol/naive/quic"
)

func registerNaiveOutbound(registry *outbound.Registry) {
	naive.RegisterOutbound(registry)
}

func registerNaiveInbound(registry *inbound.Registry) {
	naive.RegisterInbound(registry)
}
