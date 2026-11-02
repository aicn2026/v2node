package rate

// DualLimiter holds independent upload/download buckets at the same panel rate,
// so sequential speed tests can peak at the configured Mbps in each direction.
type DualLimiter struct {
	Up   *DynamicBucket
	Down *DynamicBucket
}

func NewDualLimiter(bytesPerSec int64) *DualLimiter {
	return &DualLimiter{
		Up:   NewDynamicBucket(bytesPerSec),
		Down: NewDynamicBucket(bytesPerSec),
	}
}

func (d *DualLimiter) Update(bytesPerSec int64) {
	d.Up.Update(bytesPerSec)
	d.Down.Update(bytesPerSec)
}
