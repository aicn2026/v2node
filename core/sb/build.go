package sb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	box "github.com/sagernet/sing-box"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/format"
)

type nodeState struct {
	info     *panel.NodeInfo
	users    []panel.UserInfo
	outTag   string // direct or egress-* from panel default_out
	instance *box.Box
}

func (e *Engine) buildNodeOptions(tag string, st *nodeState) (option.Options, error) {
	if st == nil || st.info == nil {
		return option.Options{}, fmt.Errorf("node %s missing info", tag)
	}

	extraOut := applyNodeOutTag(tag, st)

	inbound, err := buildInbound(tag, st)
	if err != nil {
		return option.Options{}, err
	}

	outbounds := []option.Outbound{
		{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}},
		{Type: C.TypeBlock, Tag: "block", Options: &option.StubOptions{}},
	}
	outbounds = append(outbounds, extraOut...)

	outTag := st.outTag
	if outTag == "" {
		outTag = "direct"
	}

	var rules []option.Rule
	// Block rules MUST come before the catch-all inbound→outbound route
	// (sing-box uses first-match-wins).
	if st.info.Common != nil {
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

	return option.Options{
		Log: &option.LogOptions{
			Level: e.logLevel(),
		},
		Inbounds:  []option.Inbound{inbound},
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

// applyNodeOutTag parses panel default_out (Xray freedom JSON) into sing-box
// direct outbounds with inet4/6_bind_address — same multi-IP egress semantics as Xray.
func applyNodeOutTag(tag string, st *nodeState) []option.Outbound {
	st.outTag = "direct"
	if st.info == nil || st.info.Common == nil {
		return nil
	}
	var extra []option.Outbound
	seen := map[string]bool{}
	for _, route := range st.info.Common.Routes {
		if route.Action != "default_out" || route.ActionValue == nil {
			continue
		}
		var xf struct {
			Tag            string `json:"tag"`
			SendThrough    string `json:"sendThrough"`
			StreamSettings *struct {
				Sockopt *struct {
					DomainStrategy string `json:"domainStrategy"`
				} `json:"sockopt"`
			} `json:"streamSettings"`
			Settings *struct {
				DomainStrategy string `json:"domainStrategy"`
			} `json:"settings"`
		}
		if err := json.Unmarshal([]byte(*route.ActionValue), &xf); err != nil {
			continue
		}
		outTag := xf.Tag
		if outTag == "" {
			outTag = "egress-" + tag
		}
		st.outTag = outTag
		if seen[outTag] || outTag == "direct" {
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
		ds := ""
		if xf.StreamSettings != nil && xf.StreamSettings.Sockopt != nil {
			ds = xf.StreamSettings.Sockopt.DomainStrategy
		}
		if ds == "" && xf.Settings != nil {
			ds = xf.Settings.DomainStrategy
		}
		if mapped := mapDomainStrategy(ds); mapped != 0 {
			dopts.DomainStrategy = mapped
		}
		extra = append(extra, option.Outbound{
			Type:    C.TypeDirect,
			Tag:     outTag,
			Options: dopts,
		})
	}
	return extra
}

func mapDomainStrategy(s string) option.DomainStrategy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "useip", "asis", "":
		return 0
	case "useipv4", "ipv4_only", "ipv4only":
		return option.DomainStrategy(C.DomainStrategyIPv4Only)
	case "useipv6", "ipv6_only", "ipv6only":
		return option.DomainStrategy(C.DomainStrategyIPv6Only)
	case "useipv4v6", "prefer_ipv4", "preferipv4":
		return option.DomainStrategy(C.DomainStrategyPreferIPv4)
	case "useipv6v4", "prefer_ipv6", "preferipv6":
		return option.DomainStrategy(C.DomainStrategyPreferIPv6)
	default:
		return 0
	}
}

func buildInbound(tag string, st *nodeState) (option.Inbound, error) {
	switch st.info.Type {
	case "vless":
		return buildVLESSInbound(tag, st)
	case "tuic":
		return buildTUICInbound(tag, st)
	case "hysteria2":
		return buildHysteria2Inbound(tag, st)
	case "shadowsocks":
		return buildShadowsocksInbound(tag, st)
	default:
		return option.Inbound{}, fmt.Errorf("sing-box dataplane: protocol %q not supported (supported: vless, tuic, hysteria2, shadowsocks)", st.info.Type)
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

func buildHysteria2Inbound(tag string, st *nodeState) (option.Inbound, error) {
	info := st.info
	cm := info.Common
	users := make([]option.Hysteria2User, 0, len(st.users))
	for _, u := range st.users {
		users = append(users, option.Hysteria2User{
			Name:     format.UserTag(tag, u.Uuid),
			Password: u.Uuid,
		})
	}
	hyOpts := &option.Hysteria2InboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     listenAddr(cm.ListenIP),
			ListenPort: uint16(cm.ServerPort),
		},
		Users:                 users,
		IgnoreClientBandwidth: cm.Ignore_Client_Bandwidth,
	}
	if !cm.Ignore_Client_Bandwidth {
		hyOpts.UpMbps = cm.UpMbps
		hyOpts.DownMbps = cm.DownMbps
	}
	if cm.Obfs != "" && cm.ObfsPassword != "" {
		obfsType := cm.Obfs
		if obfsType == "salamander" || obfsType == "" {
			obfsType = "salamander"
		}
		hyOpts.Obfs = &option.Hysteria2Obfs{
			Type:     obfsType,
			Password: cm.ObfsPassword,
		}
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
	hyOpts.TLS = tlsOpts
	return option.Inbound{Type: C.TypeHysteria2, Tag: tag, Options: hyOpts}, nil
}

func buildShadowsocksInbound(tag string, st *nodeState) (option.Inbound, error) {
	info := st.info
	cm := info.Common
	method := cm.Cipher
	if method == "" {
		method = "aes-128-gcm"
	}
	users := make([]option.ShadowsocksUser, 0, len(st.users))
	for _, u := range st.users {
		users = append(users, option.ShadowsocksUser{
			Name:     format.UserTag(tag, u.Uuid),
			Password: ssUserPassword(u.Uuid, method, cm.ServerKey),
		})
	}
	ssOpts := &option.ShadowsocksInboundOptions{
		ListenOptions: option.ListenOptions{
			Listen:     listenAddr(cm.ListenIP),
			ListenPort: uint16(cm.ServerPort),
		},
		Method:  method,
		Users:   users,
		Managed: true, // force multi-user inbound so UpdateUsers works with empty initial set
	}
	if cm.ServerKey != "" {
		ssOpts.Password = cm.ServerKey
	}
	return option.Inbound{Type: C.TypeShadowsocks, Tag: tag, Options: ssOpts}, nil
}

// ssUserPassword matches Xray buildSSUser: classic uses uuid; 2022 uses base64(uuid[:keyLen]).
func ssUserPassword(uuid, cipher, serverKey string) string {
	if serverKey == "" {
		return uuid
	}
	keyLength := 16
	switch cipher {
	case "2022-blake3-aes-128-gcm":
		keyLength = 16
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		keyLength = 32
	}
	if len(uuid) < keyLength {
		return base64.StdEncoding.EncodeToString([]byte(uuid))
	}
	return base64.StdEncoding.EncodeToString([]byte(uuid[:keyLength]))
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
