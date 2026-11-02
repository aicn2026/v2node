package sb

import (
	"context"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/protocol/block"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/shadowsocks"
	"github.com/sagernet/sing-box/protocol/vless"
)

// boxContext registers only protocols needed by the panel dataplane.
// Avoids pulling naive/v2rayhttp (breaks with newer golang.org/x/net).
func boxContext(ctx context.Context) context.Context {
	return box.Context(ctx, inboundRegistry(), outboundRegistry(), endpoint.NewRegistry(), dnsTransportRegistry(), service.NewRegistry())
}

func inboundRegistry() *inbound.Registry {
	registry := inbound.NewRegistry()
	direct.RegisterInbound(registry)
	vless.RegisterInbound(registry)
	shadowsocks.RegisterInbound(registry)
	registerQUICInbounds(registry)
	return registry
}

func outboundRegistry() *outbound.Registry {
	registry := outbound.NewRegistry()
	direct.RegisterOutbound(registry)
	block.RegisterOutbound(registry)
	vless.RegisterOutbound(registry)
	shadowsocks.RegisterOutbound(registry)
	registerQUICOutbounds(registry)
	return registry
}

func dnsTransportRegistry() *dns.TransportRegistry {
	registry := dns.NewTransportRegistry()
	transport.RegisterTCP(registry)
	transport.RegisterUDP(registry)
	transport.RegisterTLS(registry)
	transport.RegisterHTTPS(registry)
	local.RegisterTransport(registry)
	return registry
}
