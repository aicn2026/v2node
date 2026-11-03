package sb

import (
	"fmt"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	hy2in "github.com/sagernet/sing-box/protocol/hysteria2"
	ssin "github.com/sagernet/sing-box/protocol/shadowsocks"
	tuicin "github.com/sagernet/sing-box/protocol/tuic"
	vlessin "github.com/sagernet/sing-box/protocol/vless"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/format"
)

func (e *Engine) hotUpdateUsers(tag string, st *nodeState) error {
	if st == nil || st.instance == nil || st.info == nil {
		return fmt.Errorf("instance or node info missing")
	}
	ib, ok := st.instance.Inbound().Get(tag)
	if !ok {
		return fmt.Errorf("inbound %s not found", tag)
	}
	return applyUsersToInbound(ib, tag, st)
}

func applyUsersToInbound(ib adapter.Inbound, tag string, st *nodeState) error {
	switch st.info.Type {
	case "vless":
		v, ok := ib.(*vlessin.Inbound)
		if !ok {
			return fmt.Errorf("inbound %s is not vless", tag)
		}
		flow := ""
		if st.info.Common != nil {
			flow = st.info.Common.Flow
		}
		users := make([]option.VLESSUser, 0, len(st.users))
		for _, u := range st.users {
			users = append(users, option.VLESSUser{
				Name: format.UserTag(tag, u.Uuid),
				UUID: u.Uuid,
				Flow: flow,
			})
		}
		v.UpdateUsers(users)
		return nil
	case "tuic":
		ti, ok := ib.(*tuicin.Inbound)
		if !ok {
			return fmt.Errorf("inbound %s is not tuic", tag)
		}
		users := make([]option.TUICUser, 0, len(st.users))
		for _, u := range st.users {
			users = append(users, option.TUICUser{
				Name:     format.UserTag(tag, u.Uuid),
				UUID:     u.Uuid,
				Password: u.Uuid,
			})
		}
		return ti.UpdateUsers(users)
	case "hysteria2":
		h, ok := ib.(*hy2in.Inbound)
		if !ok {
			return fmt.Errorf("inbound %s is not hysteria2", tag)
		}
		users := make([]option.Hysteria2User, 0, len(st.users))
		for _, u := range st.users {
			users = append(users, option.Hysteria2User{
				Name:     format.UserTag(tag, u.Uuid),
				Password: u.Uuid,
			})
		}
		h.UpdateUsers(users)
		return nil
	case "shadowsocks":
		mi, ok := ib.(*ssin.MultiInbound)
		if !ok {
			return fmt.Errorf("inbound %s is not shadowsocks multi-user", tag)
		}
		cm := st.info.Common
		method := cm.Cipher
		if method == "" {
			method = "aes-128-gcm"
		}
		names := make([]string, 0, len(st.users))
		passwords := make([]string, 0, len(st.users))
		for _, u := range st.users {
			names = append(names, format.UserTag(tag, u.Uuid))
			passwords = append(passwords, ssUserPassword(u.Uuid, method, cm.ServerKey))
		}
		return mi.UpdateUsers(names, passwords)
	default:
		return fmt.Errorf("hot update unsupported for %s", st.info.Type)
	}
}

func mergeUsers(existing, added []panel.UserInfo) []panel.UserInfo {
	byUUID := map[string]panel.UserInfo{}
	for _, u := range existing {
		byUUID[u.Uuid] = u
	}
	for _, u := range added {
		byUUID[u.Uuid] = u
	}
	out := make([]panel.UserInfo, 0, len(byUUID))
	for _, u := range byUUID {
		out = append(out, u)
	}
	return out
}
