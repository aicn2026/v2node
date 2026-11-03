package rate

// DualLimiter holds independent upload/download buckets at the same panel rate
// (v0.1.5 behavior), so sequential speed tests can peak near the configured
// Mbps in each direction without sharing one bucket.
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
