package sb

import (
	"context"
	"net"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/wyx2685/v2node/common/counter"
)

// trafficTracker implements adapter.ConnectionTracker and feeds panel traffic counters.
type trafficTracker struct {
	counter *counter.TrafficCounter
}

func (t *trafficTracker) RoutedConnection(_ context.Context, conn net.Conn, metadata adapter.InboundContext, _ adapter.Rule, _ adapter.Outbound) net.Conn {
	user := metadata.User
	if user == "" || t.counter == nil {
		return conn
	}
	ts := t.counter.GetCounter(user)
	return bufio.NewInt64CounterConn(conn, []*atomic.Int64{&ts.UpCounter}, []*atomic.Int64{&ts.DownCounter})
}

func (t *trafficTracker) RoutedPacketConnection(_ context.Context, conn N.PacketConn, metadata adapter.InboundContext, _ adapter.Rule, _ adapter.Outbound) N.PacketConn {
	user := metadata.User
	if user == "" || t.counter == nil {
		return conn
	}
	ts := t.counter.GetCounter(user)
	return bufio.NewInt64CounterPacketConn(
		conn,
		[]*atomic.Int64{&ts.UpCounter},
		nil,
		[]*atomic.Int64{&ts.DownCounter},
		nil,
	)
}
