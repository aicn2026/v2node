package rate

import (
	"sync/atomic"
	"time"

	"github.com/juju/ratelimit"
)

// DynamicBucket is a hot-swappable ratelimit.Bucket used by the panel limiter.
// Same as v0.1.5: 1s quantum, capacity = 1s of traffic.
type DynamicBucket struct {
	v atomic.Value // *ratelimit.Bucket
}

func NewDynamicBucket(rate int64) *DynamicBucket {
	b := ratelimit.NewBucketWithQuantum(time.Second, rate, rate)
	d := &DynamicBucket{}
	d.v.Store(b)
	return d
}

func (d *DynamicBucket) Get() *ratelimit.Bucket {
	return d.v.Load().(*ratelimit.Bucket)
}

func (d *DynamicBucket) Update(rate int64) {
	newB := ratelimit.NewBucketWithQuantum(time.Second, rate, rate)
	d.v.Store(newB)
}
