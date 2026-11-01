package sb

import (
	"io"
	"net"
	"sync"

	N "github.com/sagernet/sing/common/network"
)

// connGroup tracks live connections for one panel user (email = tag|uuid).
// DelUsers closes them, matching Xray LinkManager.CloseAll behavior.
type connGroup struct {
	mu     sync.Mutex
	closed bool
	conns  map[io.Closer]struct{}
}

func newConnGroup() *connGroup {
	return &connGroup{conns: make(map[io.Closer]struct{})}
}

func (g *connGroup) add(c io.Closer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		_ = c.Close()
		return
	}
	g.conns[c] = struct{}{}
}

func (g *connGroup) remove(c io.Closer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.conns, c)
}

func (g *connGroup) closeAll() {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.closed = true
	conns := g.conns
	g.conns = make(map[io.Closer]struct{})
	g.mu.Unlock()
	for c := range conns {
		_ = c.Close()
	}
}

type trackedNetConn struct {
	net.Conn
	group *connGroup
	once  sync.Once
}

func (c *trackedNetConn) Close() error {
	var err error
	c.once.Do(func() {
		c.group.remove(c)
		err = c.Conn.Close()
	})
	return err
}

type trackedPacketConn struct {
	N.PacketConn
	group *connGroup
	once  sync.Once
}

func (c *trackedPacketConn) Close() error {
	var err error
	c.once.Do(func() {
		c.group.remove(c)
		err = c.PacketConn.Close()
	})
	return err
}
