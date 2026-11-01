package sb

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/wyx2685/v2node/common/counter"
	"github.com/wyx2685/v2node/common/rate"
	"github.com/wyx2685/v2node/limiter"
)

// trafficTracker implements adapter.ConnectionTracker: traffic, speed/device limits, kick-on-delete.
type trafficTracker struct {
	counter *counter.TrafficCounter
	links   sync.Map // email -> *connGroup
}

func (t *trafficTracker) closeUser(email string) {
	if v, ok := t.links.Load(email); ok {
		v.(*connGroup).closeAll()
		t.links.Delete(email)
	}
}

func (t *trafficTracker) groupFor(email string) *connGroup {
	if v, ok := t.links.Load(email); ok {
		return v.(*connGroup)
	}
	g := newConnGroup()
	if actual, loaded := t.links.LoadOrStore(email, g); loaded {
		return actual.(*connGroup)
	}
	return g
}

func (t *trafficTracker) applyLimit(email, inboundTag, ip string, isTCP bool) (bucket *rate.DynamicBucket, reject bool) {
	lim, err := limiter.GetLimiter(inboundTag)
	if err != nil {
		return nil, false
	}
	return lim.CheckLimit(email, ip, isTCP)
}

func sourceIP(metadata adapter.InboundContext) string {
	if metadata.Source.Addr.IsValid() {
		return metadata.Source.Addr.String()
	}
	return metadata.Source.Fqdn
}

func (t *trafficTracker) RoutedConnection(_ context.Context, conn net.Conn, metadata adapter.InboundContext, _ adapter.Rule, _ adapter.Outbound) net.Conn {
	user := metadata.User
	if user == "" {
		return conn
	}
	bucket, reject := t.applyLimit(user, metadata.Inbound, sourceIP(metadata), true)
	if reject {
		log.Infof("Limited %s by conn or ip", user)
		_ = conn.Close()
		return conn
	}
	if bucket != nil {
		conn = rate.NewDynamicConnRateLimiter(conn, bucket)
	}
	if t.counter != nil {
		ts := t.counter.GetCounter(user)
		conn = bufio.NewInt64CounterConn(conn, []*atomic.Int64{&ts.UpCounter}, []*atomic.Int64{&ts.DownCounter})
	}
	g := t.groupFor(user)
	tc := &trackedNetConn{Conn: conn, group: g}
	g.add(tc)
	return tc
}

func (t *trafficTracker) RoutedPacketConnection(_ context.Context, conn N.PacketConn, metadata adapter.InboundContext, _ adapter.Rule, _ adapter.Outbound) N.PacketConn {
	user := metadata.User
	if user == "" {
		return conn
	}
	bucket, reject := t.applyLimit(user, metadata.Inbound, sourceIP(metadata), false)
	if reject {
		log.Infof("Limited %s by conn or ip", user)
		_ = conn.Close()
		return conn
	}
	if bucket != nil {
		conn = newRateLimitedPacketConn(conn, bucket)
	}
	if t.counter != nil {
		ts := t.counter.GetCounter(user)
		conn = bufio.NewInt64CounterPacketConn(
			conn,
			[]*atomic.Int64{&ts.UpCounter},
			nil,
			[]*atomic.Int64{&ts.DownCounter},
			nil,
		)
	}
	g := t.groupFor(user)
	tc := &trackedPacketConn{PacketConn: conn, group: g}
	g.add(tc)
	return tc
}
