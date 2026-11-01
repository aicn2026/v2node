package sb

import (
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/wyx2685/v2node/common/rate"
)

type rateLimitedPacketConn struct {
	N.PacketConn
	limiter *rate.DynamicBucket
}

func newRateLimitedPacketConn(conn N.PacketConn, limiter *rate.DynamicBucket) N.PacketConn {
	if limiter == nil {
		return conn
	}
	return &rateLimitedPacketConn{PacketConn: conn, limiter: limiter}
}

func (c *rateLimitedPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = c.PacketConn.ReadPacket(buffer)
	if err == nil {
		if bkt := c.limiter.Get(); bkt != nil {
			bkt.Wait(int64(buffer.Len()))
		}
	}
	return destination, err
}

func (c *rateLimitedPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	if bkt := c.limiter.Get(); bkt != nil {
		bkt.Wait(int64(buffer.Len()))
	}
	return c.PacketConn.WritePacket(buffer, destination)
}
