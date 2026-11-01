package rate

import (
	"net"

	"github.com/juju/ratelimit"
)

func NewConnRateLimiter(c net.Conn, l *ratelimit.Bucket) *Conn {
	return &Conn{
		Conn:    c,
		limiter: l,
	}
}

type Conn struct {
	net.Conn
	limiter *ratelimit.Bucket
}

func (c *Conn) Read(b []byte) (n int, err error) {
	c.limiter.Wait(int64(len(b)))
	return c.Conn.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	c.limiter.Wait(int64(len(b)))
	return c.Conn.Write(b)
}

// NewDynamicConnRateLimiter wraps conn with a hot-swappable DynamicBucket (panel speed limit).
func NewDynamicConnRateLimiter(c net.Conn, l *DynamicBucket) net.Conn {
	if l == nil {
		return c
	}
	return &DynamicConn{Conn: c, limiter: l}
}

type DynamicConn struct {
	net.Conn
	limiter *DynamicBucket
}

func (c *DynamicConn) Read(b []byte) (n int, err error) {
	if bkt := c.limiter.Get(); bkt != nil {
		bkt.Wait(int64(len(b)))
	}
	return c.Conn.Read(b)
}

func (c *DynamicConn) Write(b []byte) (n int, err error) {
	if bkt := c.limiter.Get(); bkt != nil {
		bkt.Wait(int64(len(b)))
	}
	return c.Conn.Write(b)
}

/*
type PacketConnCounter struct {
	network.PacketConn
	limiter *ratelimit.Bucket
}

func NewPacketConnCounter(conn network.PacketConn, l *ratelimit.Bucket) network.PacketConn {
	return &PacketConnCounter{
		PacketConn: conn,
		limiter:    l,
	}
}

func (p *PacketConnCounter) ReadPacket(buff *buf.Buffer) (destination M.Socksaddr, err error) {
	pLen := buff.Len()
	destination, err = p.PacketConn.ReadPacket(buff)
	p.limiter.Wait(int64(buff.Len() - pLen))
	return destination, err
}

func (p *PacketConnCounter) WritePacket(buff *buf.Buffer, destination M.Socksaddr) (err error) {
	p.limiter.Wait(int64(buff.Len()))
	return p.PacketConn.WritePacket(buff, destination)
}
*/
