//go:build with_quic

package sb

import (
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/protocol/hysteria2"
	"github.com/sagernet/sing-box/protocol/tuic"
)

func registerQUICInbounds(registry *inbound.Registry) {
	tuic.RegisterInbound(registry)
	hysteria2.RegisterInbound(registry)
}

func registerQUICOutbounds(registry *outbound.Registry) {
	tuic.RegisterOutbound(registry)
	hysteria2.RegisterOutbound(registry)
}
