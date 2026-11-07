package sb

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"

	box "github.com/sagernet/sing-box"
	sbconst "github.com/sagernet/sing-box/constant"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/counter"
	"github.com/wyx2685/v2node/common/format"
	"github.com/wyx2685/v2node/conf"
)

// Engine is the sing-box dataplane behind v2node's panel-facing API.
// Each panel node runs in its own sing-box instance so protocol/port
// changes only restart that node and leave others connected.
type Engine struct {
	conf    *conf.Conf
	access  sync.Mutex
	nodes   sync.Map // tag -> *nodeState
	uidMap  map[string]int
	uidLock sync.RWMutex
	traffic *counter.TrafficCounter
	tracker *trafficTracker
}

func NewEngine(c *conf.Conf) *Engine {
	tr := &trafficTracker{counter: counter.NewTrafficCounter()}
	return &Engine{
		conf:    c,
		uidMap:  make(map[string]int),
		traffic: tr.counter,
		tracker: tr,
	}
}

func (e *Engine) Start(_ []*panel.NodeInfo) error {
	// Inbounds are added via AddNode after Start (same as Xray path).
	return nil
}

func (e *Engine) Close() error {
	e.access.Lock()
	defer e.access.Unlock()
	var firstErr error
	e.nodes.Range(func(key, value any) bool {
		st := value.(*nodeState)
		if err := e.closeNodeLocked(st); err != nil && firstErr == nil {
			firstErr = err
		}
		return true
	})
	return firstErr
}

func (e *Engine) AddNode(tag string, info *panel.NodeInfo) error {
	if info == nil {
		return fmt.Errorf("nil node info")
	}
	switch info.Type {
	case "vless", "shadowsocks":
	case "tuic", "hysteria2":
		if !quicEnabled {
			return fmt.Errorf("sing-box dataplane: %s requires build tag with_quic", info.Type)
		}
	default:
		return fmt.Errorf("sing-box dataplane: protocol %q not supported (supported: vless, tuic, hysteria2, shadowsocks)", info.Type)
	}
	e.access.Lock()
	defer e.access.Unlock()
	var users []panel.UserInfo
	if v, ok := e.nodes.Load(tag); ok {
		old := v.(*nodeState)
		users = old.users
		// Close previous instance before replacing state (same tag port/protocol change).
		if err := e.closeNodeLocked(old); err != nil {
			log.WithField("err", err).WithField("tag", tag).Warn("close previous node instance")
		}
	}
	st := &nodeState{info: info, users: users}
	e.nodes.Store(tag, st)
	return e.rebuildNodeLocked(tag, st)
}

func (e *Engine) DelNode(tag string) error {
	e.access.Lock()
	defer e.access.Unlock()
	v, ok := e.nodes.Load(tag)
	if !ok {
		return nil
	}
	st := v.(*nodeState)
	if err := e.closeNodeLocked(st); err != nil {
		log.WithField("err", err).WithField("tag", tag).Warn("close node instance")
	}
	e.nodes.Delete(tag)
	e.clearUIDForTag(tag)
	return nil
}

func (e *Engine) AddUsers(tag string, users []panel.UserInfo, info *panel.NodeInfo) (int, error) {
	e.access.Lock()
	defer e.access.Unlock()
	stAny, ok := e.nodes.Load(tag)
	if !ok {
		if info == nil {
			return 0, fmt.Errorf("node %s not found", tag)
		}
		stAny = &nodeState{info: info}
		e.nodes.Store(tag, stAny)
	}
	st := stAny.(*nodeState)
	if info != nil {
		st.info = info
	}
	for _, u := range users {
		e.uidLock.Lock()
		e.uidMap[format.UserTag(tag, u.Uuid)] = u.Id
		e.uidLock.Unlock()
	}
	st.users = mergeUsers(st.users, users)

	// Prefer hot UpdateUsers (no listener restart). Fall back to rebuild this node only.
	if err := e.hotUpdateUsers(tag, st); err != nil {
		log.WithField("err", err).WithField("tag", tag).Debug("hot AddUsers fallback to node rebuild")
		if err := e.rebuildNodeLocked(tag, st); err != nil {
			return 0, err
		}
	}
	return len(users), nil
}

func (e *Engine) DelUsers(users []panel.UserInfo, tag string) error {
	e.access.Lock()
	defer e.access.Unlock()
	stAny, ok := e.nodes.Load(tag)
	if !ok {
		return nil
	}
	st := stAny.(*nodeState)
	del := map[string]struct{}{}
	for _, u := range users {
		del[u.Uuid] = struct{}{}
		email := format.UserTag(tag, u.Uuid)
		e.uidLock.Lock()
		delete(e.uidMap, email)
		e.uidLock.Unlock()
		e.traffic.Delete(email)
		e.tracker.closeUser(email) // kick live sessions like Xray LinkManager.CloseAll
	}
	kept := st.users[:0]
	for _, u := range st.users {
		if _, drop := del[u.Uuid]; !drop {
			kept = append(kept, u)
		}
	}
	st.users = kept

	if err := e.hotUpdateUsers(tag, st); err != nil {
		log.WithField("err", err).WithField("tag", tag).Debug("hot DelUsers fallback to node rebuild")
		return e.rebuildNodeLocked(tag, st)
	}
	return nil
}

func (e *Engine) GetUserTrafficSlice(tag string, minTraffic int) ([]panel.UserTraffic, error) {
	trafficSlice := make([]panel.UserTraffic, 0)
	e.uidLock.RLock()
	defer e.uidLock.RUnlock()
	e.traffic.Counters.Range(func(key, value any) bool {
		email := key.(string)
		// only this inbound tag
		if tag != "" && (len(email) < len(tag)+1 || email[:len(tag)] != tag || email[len(tag)] != '|') {
			return true
		}
		ts := value.(*counter.TrafficStorage)
		up := ts.UpCounter.Load()
		down := ts.DownCounter.Load()
		if up+down <= int64(minTraffic*1000) {
			return true
		}
		ts.UpCounter.Store(0)
		ts.DownCounter.Store(0)
		uid := e.uidMap[email]
		if uid == 0 {
			e.traffic.Delete(email)
			return true
		}
		trafficSlice = append(trafficSlice, panel.UserTraffic{
			UID:      uid,
			Upload:   up,
			Download: down,
		})
		return true
	})
	if len(trafficSlice) == 0 {
		return nil, nil
	}
	return trafficSlice, nil
}

func (e *Engine) clearUIDForTag(tag string) {
	prefix := tag + "|"
	e.uidLock.Lock()
	defer e.uidLock.Unlock()
	for email := range e.uidMap {
		if strings.HasPrefix(email, prefix) {
			delete(e.uidMap, email)
			e.traffic.Delete(email)
			e.tracker.closeUser(email)
		}
	}
}

func (e *Engine) closeNodeLocked(st *nodeState) error {
	if st == nil || st.instance == nil {
		return nil
	}
	err := st.instance.Close()
	st.instance = nil
	return err
}

func (e *Engine) rebuildNodeLocked(tag string, st *nodeState) error {
	opts, err := e.buildNodeOptions(tag, st)
	if err != nil {
		return err
	}
	if err := e.closeNodeLocked(st); err != nil {
		log.WithField("err", err).WithField("tag", tag).Warn("close previous node instance")
	}
	ctx := boxContext(context.Background())
	instance, err := box.New(box.Options{
		Context:          ctx,
		Options:          opts,
		DefaultLogWriter: newNodeLogWriter(tag),
	})
	if err != nil {
		return fmt.Errorf("create sing-box for %s: %w", tag, err)
	}
	instance.Router().AppendTracker(e.tracker)
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		return fmt.Errorf("start sing-box for %s: %w", tag, err)
	}
	st.instance = instance
	log.Infof("sing-box %s node ready tag=%s", sbconst.Version, tag)
	return nil
}
