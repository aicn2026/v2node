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
	n, err = c.Conn.Read(b)
	if n > 0 {
		c.limiter.Wait(int64(n))
	}
	return n, err
}

func (c *Conn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if n > 0 {
		c.limiter.Wait(int64(n))
	}
	return n, err
}

// NewDualDynamicConnRateLimiter caps upload (Read) and download (Write) independently.
// For an inbound client conn: Read=user upload, Write=user download.
func NewDualDynamicConnRateLimiter(c net.Conn, lim *DualLimiter) net.Conn {
	if lim == nil {
		return c
	}
	return &DynamicConn{Conn: c, up: lim.Up, down: lim.Down}
}

// Deprecated alias kept for call sites that still pass a single bucket.
func NewDynamicConnRateLimiter(c net.Conn, l *DynamicBucket) net.Conn {
	if l == nil {
		return c
	}
	return &DynamicConn{Conn: c, up: l, down: l}
}

type DynamicConn struct {
	net.Conn
	up, down *DynamicBucket
}

func (c *DynamicConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if n > 0 && c.up != nil {
		if bkt := c.up.Get(); bkt != nil {
			bkt.Wait(int64(n))
		}
	}
	return n, err
}

func (c *DynamicConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if n > 0 && c.down != nil {
		if bkt := c.down.Get(); bkt != nil {
			bkt.Wait(int64(n))
		}
	}
	return n, err
}

func (c *DynamicConn) Upstream() any {
	return c.Conn
}
