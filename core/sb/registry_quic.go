//go:build with_quic

package sb

import (
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/protocol/tuic"
)

func registerQUICInbounds(registry *inbound.Registry) {
	tuic.RegisterInbound(registry)
}

func registerQUICOutbounds(registry *outbound.Registry) {
	tuic.RegisterOutbound(registry)
}
