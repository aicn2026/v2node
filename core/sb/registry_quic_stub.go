//go:build !with_quic

package sb

import (
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
)

func registerQUICInbounds(_ *inbound.Registry)  {}
func registerQUICOutbounds(_ *outbound.Registry) {}
