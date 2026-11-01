package core

import (
	"sync"

	log "github.com/sirupsen/logrus"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/conf"
	"github.com/wyx2685/v2node/core/sb"
)

type AddUsersParams struct {
	Tag   string
	Users []panel.UserInfo
	*panel.NodeInfo
}

// V2Core is the panel-facing dataplane façade. Data plane is sing-box
// (game-compatible path); Xray builders remain in-tree unused for rollback.
type V2Core struct {
	Config   *conf.Conf
	ReloadCh chan struct{}
	access   sync.Mutex
	engine   *sb.Engine
	users    *UserMap
}

type UserMap struct {
	uidMap  map[string]int
	mapLock sync.RWMutex
}

func New(config *conf.Conf) *V2Core {
	return &V2Core{
		Config: config,
		users: &UserMap{
			uidMap: make(map[string]int),
		},
		engine: sb.NewEngine(config),
	}
}

func (v *V2Core) Start(infos []*panel.NodeInfo) error {
	v.access.Lock()
	defer v.access.Unlock()
	if err := v.engine.Start(infos); err != nil {
		return err
	}
	log.Info("Dataplane: sing-box (panel API unchanged)")
	return nil
}

func (v *V2Core) Close() error {
	v.access.Lock()
	defer v.access.Unlock()
	v.Config = nil
	if v.engine != nil {
		return v.engine.Close()
	}
	return nil
}
