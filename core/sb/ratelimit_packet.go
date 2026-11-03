package sb

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/wyx2685/v2node/common/rate"
)

type rateLimitedPacketConn struct {
	N.PacketConn
	up, down *rate.DynamicBucket
}

func newRateLimitedPacketConn(conn N.PacketConn, lim *rate.DualLimiter) N.PacketConn {
	if lim == nil {
		return conn
	}
	return &rateLimitedPacketConn{PacketConn: conn, up: lim.Up, down: lim.Down}
}

func (c *rateLimitedPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil && buffer.Len() > 0 && c.up != nil {
		if bkt := c.up.Get(); bkt != nil {
			bkt.Wait(int64(buffer.Len()))
		}
	}
	return destination, err
}

func (c *rateLimitedPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	n := buffer.Len()
	err := c.PacketConn.WritePacket(buffer, destination)
	if err == nil && n > 0 && c.down != nil {
		if bkt := c.down.Get(); bkt != nil {
			bkt.Wait(int64(n))
		}
	}
	return err
}

func (c *rateLimitedPacketConn) Upstream() any {
	return c.PacketConn
}
