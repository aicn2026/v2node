package sb

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/format"
)

type nodeState struct {
	info   *panel.NodeInfo
	users  []panel.UserInfo
	outTag string // direct or egress-* from panel default_out
}

func (e *Engine) buildOptions() (option.Options, error) {
	e.applyDefaultOutTags()

	var inbounds []option.Inbound
	var outbounds []option.Outbound
	var rules []option.Rule
	seenOut := map[string]bool{"direct": true, "block": true}

	outbounds = append(outbounds,
		option.Outbound{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}},
		option.Outbound{Type: C.TypeBlock, Tag: "block", Options: &option.StubOptions{}},
	)

	var buildErr error
	e.nodes.Range(func(key, value any) bool {
		tag := key.(string)
		st := value.(*nodeState)
		if st.info == nil {
			return true
		}
		inbound, err := buildInbound(tag, st)
		if err != nil {
			buildErr = err
			return false
		}
		inbounds = append(inbounds, inbound)

		outTag := st.outTag
		if outTag == "" {
			outTag = "direct"
		}
		if outTag != "direct" && !seenOut[outTag] {
			// outbound created in applyDefaultOutTags via e.extraOutbounds
			seenOut[outTag] = true
		}

		rules = append(rules, option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{
					Inbound: badoption.Listable[string]{tag},
				},
				RuleAction: option.RuleAction{
					Action: C.RuleActionTypeRoute,
					RouteOptions: option.RouteActionOptions{
						Outbound: outTag,
					},
				},
			},
		})

		for _, route := range st.info.Common.Routes {
			switch route.Action {
			case "block":
				if len(route.Match) == 0 {
					continue
				}
				rules = append(rules, option.Rule{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: badoption.Listable[string]{tag},
							Domain:  badoption.Listable[string](route.Match),
						},
						RuleAction: option.RuleAction{Action: C.RuleActionTypeReject},
					},
				})
			case "block_ip":
				if len(route.Match) == 0 {
					continue
				}
				rules = append(rules, option.Rule{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: badoption.Listable[string]{tag},
							IPCIDR:  badoption.Listable[string](route.Match),
						},
						RuleAction: option.RuleAction{Action: C.RuleActionTypeReject},
					},
				})
			case "block_port":
				ports := parsePorts(strings.Join(route.Match, ","))
				if len(ports) == 0 {
					continue
				}
				rules = append(rules, option.Rule{
					Type: C.RuleTypeDefault,
					DefaultOptions: option.DefaultRule{
						RawDefaultRule: option.RawDefaultRule{
							Inbound: badoption.Listable[string]{tag},
							Port:    badoption.Listable[uint16](ports),
						},
						RuleAction: option.RuleAction{Action: C.RuleActionTypeReject},
					},
				})
			}
		}
		return true
	})
	if buildErr != nil {
		return option.Options{}, buildErr
	}

	outbounds = append(outbounds, e.extraOutbounds...)

	return option.Options{
		Log: &option.LogOptions{
			Level: e.logLevel(),
		},
		Inbounds:  inbounds,
		Outbounds: outbounds,
		Route: &option.RouteOptions{
			Rules: rules,
			Final: "direct",
		},
	}, nil
}

func (e *Engine) logLevel() string {
	if e.conf != nil && e.conf.LogConfig.Level != "" {
		return e.conf.LogConfig.Level
	}
	return "info"
}

// applyDefaultOutTags parses panel default_out (Xray freedom JSON) into sing-box
// direct outbounds with inet4/6_bind_address and sets each node's outTag.
func (e *Engine) applyDefaultOutTags() {
	e.extraOutbounds = nil
	seen := map[string]bool{}
	e.nodes.Range(func(key, value any) bool {
		tag := key.(string)
		st := value.(*nodeState)
		if st.info == nil || st.info.Common == nil {
			return true
		}
		st.outTag = "direct"
		for _, route := range st.info.Common.Routes {
			if route.Action != "default_out" || route.ActionValue == nil {
				continue
			}
			var xf struct {
				Tag         string `json:"tag"`
				SendThrough string `json:"sendThrough"`
			}
			if err := json.Unmarshal([]byte(*route.ActionValue), &xf); err != nil {
				continue
			}
			outTag := xf.Tag
			if outTag == "" {
				outTag = "egress-" + tag
			}
			st.outTag = outTag
			if seen[outTag] {
				continue
			}
			seen[outTag] = true
			dopts := &option.DirectOutboundOptions{}
			if xf.SendThrough != "" {
				if addr, err := netip.ParseAddr(xf.SendThrough); err == nil {
					a := badoption.Addr(addr)
					if addr.Is4() {
						dopts.Inet4BindAddress = &a
					} else {
						dopts.Inet6BindAddress = &a
					}
				}
			}
			e.extraOutbounds = append(e.extraOutbounds, option.Outbound{
				Type:    C.TypeDirect,
				Tag:     outTag,
				Options: dopts,
			})
		}
		return true
	})
}

func buildInbound(tag string, st *nodeState) (option.Inbound, error) {
	switch st.info.Type {
	case "vless":
		return buildVLESSInbound(tag, st)
	case "tuic":
		return buildTUICInbound(tag, st)
	default:
		return option.Inbound{}, fmt.Errorf("sing-box dataplane: protocol %q not supported yet (phase1: vless, tuic)", st.info.Type)
	}
}

func listenAddr(listenIP string) *badoption.Addr {
	if listenIP == "" || listenIP == "0.0.0.0" {
		a := badoption.Addr(netip.IPv4Unspecified())
		return &a
	}
	if listenIP == "::" || listenIP == "[::]" {
		a := badoption.Addr(netip.IPv6Unspecified())
		return &a
	}
	addr, err := netip.ParseAddr(strings.Trim(listenIP, "[]"))
	if err != nil {
		a := badoption.Addr(netip.IPv4Unspecified())
		return &a
	}
	a := badoption.Addr(addr)
	return &a
}

func buildVLESSInbound(tag string, st *nodeState) (option.Inbound, error) {
	info := st.info
	cm := info.Common
	users := make([]option.VLESSUser, 0, len(st.users))
	flow := cm.Flow
	for _, u := range st.users {
		users = append(users, option.VLESSUser{
			Name: format.UserTag(tag, u.Uuid),
			UUID: u.Uuid,
			Flow: flow,
		})
	}
	vlessOpts := &option.VLESSInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     listenAddr(cm.ListenIP),
			ListenPort: uint16(cm.ServerPort),
		},
		Users: users,
	}
	switch info.Security {
	case panel.Reality:
		serverName := cm.TlsSettings.PrimaryServerName()
		dest := cm.TlsSettings.Dest
		if dest == "" {
			dest = serverName
		}
		destPort := uint16(443)
		if p := cm.TlsSettings.ServerPort; p != "" {
			if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
				destPort = uint16(n)
			}
		}
		vlessOpts.TLS = &option.InboundTLSOptions{
			Enabled:    true,
			ServerName: serverName,
			Reality: &option.InboundRealityOptions{
				Enabled: true,
				Handshake: option.InboundRealityHandshakeOptions{
					ServerOptions: option.ServerOptions{
						Server:     dest,
						ServerPort: destPort,
					},
				},
				PrivateKey: cm.TlsSettings.PrivateKey,
				ShortID:    badoption.Listable[string](cm.TlsSettings.EffectiveShortIds()),
			},
		}
	case panel.Tls:
		serverName := cm.TlsSettings.PrimaryServerName()
		tlsOpts := &option.InboundTLSOptions{
			Enabled:    true,
			ServerName: serverName,
		}
		if cm.CertInfo != nil {
			tlsOpts.CertificatePath = cm.CertInfo.CertFile
			tlsOpts.KeyPath = cm.CertInfo.KeyFile
		}
		vlessOpts.TLS = tlsOpts
	}
	return option.Inbound{Type: C.TypeVLESS, Tag: tag, Options: vlessOpts}, nil
}

func buildTUICInbound(tag string, st *nodeState) (option.Inbound, error) {
	info := st.info
	cm := info.Common
	users := make([]option.TUICUser, 0, len(st.users))
	for _, u := range st.users {
		users = append(users, option.TUICUser{
			Name:     format.UserTag(tag, u.Uuid),
			UUID:     u.Uuid,
			Password: u.Uuid,
		})
	}
	congestion := cm.CongestionControl
	if congestion == "" {
		congestion = "bbr"
	}
	tuicOpts := &option.TUICInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     listenAddr(cm.ListenIP),
			ListenPort: uint16(cm.ServerPort),
		},
		Users:             users,
		CongestionControl: congestion,
		ZeroRTTHandshake:  cm.ZeroRTTHandshake,
	}
	serverName := cm.ServerName
	if serverName == "" {
		serverName = cm.TlsSettings.PrimaryServerName()
	}
	tlsOpts := &option.InboundTLSOptions{
		Enabled:    true,
		ServerName: serverName,
		ALPN:       badoption.Listable[string]{"h3"},
	}
	if cm.CertInfo != nil {
		tlsOpts.CertificatePath = cm.CertInfo.CertFile
		tlsOpts.KeyPath = cm.CertInfo.KeyFile
	}
	tuicOpts.TLS = tlsOpts
	return option.Inbound{Type: C.TypeTUIC, Tag: tag, Options: tuicOpts}, nil
}

func parsePorts(s string) []uint16 {
	var out []uint16
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			out = append(out, uint16(n))
		}
	}
	return out
}
