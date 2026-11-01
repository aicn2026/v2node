package core

import (
	"fmt"

	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/format"
)

func (vc *V2Core) DelUsers(users []panel.UserInfo, tag string, _ *panel.NodeInfo) error {
	vc.users.mapLock.Lock()
	for i := range users {
		delete(vc.users.uidMap, format.UserTag(tag, users[i].Uuid))
	}
	vc.users.mapLock.Unlock()
	if err := vc.engine.DelUsers(users, tag); err != nil {
		return fmt.Errorf("del users error: %s", err)
	}
	return nil
}

func (vc *V2Core) GetUserTrafficSlice(tag string, mintraffic int) ([]panel.UserTraffic, error) {
	return vc.engine.GetUserTrafficSlice(tag, mintraffic)
}

func (v *V2Core) AddUsers(p *AddUsersParams) (added int, err error) {
	v.users.mapLock.Lock()
	for i := range p.Users {
		v.users.uidMap[format.UserTag(p.Tag, p.Users[i].Uuid)] = p.Users[i].Id
	}
	v.users.mapLock.Unlock()
	n, err := v.engine.AddUsers(p.Tag, p.Users, p.NodeInfo)
	if err != nil {
		return 0, err
	}
	return n, nil
}
