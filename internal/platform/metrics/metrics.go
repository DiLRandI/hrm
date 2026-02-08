package metrics

import (
	"sync/atomic"
	"time"
)

type Collector struct {
	totalRequests   uint64
	errorRequests   uint64
	rateLimited     uint64
	totalDurationMs uint64
}

func New() *Collector {
	return &Collector{}
}

func (c *Collector) Record(status int, duration time.Duration) {
	atomic.AddUint64(&c.totalRequests, 1)
	if status >= 500 {
		atomic.AddUint64(&c.errorRequests, 1)
	}
	if status == 429 {
		atomic.AddUint64(&c.rateLimited, 1)
	}
	atomic.AddUint64(&c.totalDurationMs, uint64(duration.Milliseconds()))
}

func (c *Collector) Snapshot() map[string]any {
	total := atomic.LoadUint64(&c.totalRequests)
	errs := atomic.LoadUint64(&c.errorRequests)
	limited := atomic.LoadUint64(&c.rateLimited)
	totalMs := atomic.LoadUint64(&c.totalDurationMs)
	avg := float64(0)
	if total > 0 {
		avg = float64(totalMs) / float64(total)
	}
	return map[string]any{
		"requestsTotal":    total,
		"errorsTotal":      errs,
		"rateLimitedTotal": limited,
		"avgDurationMs":    avg,
		"totalDurationMs":  totalMs,
	}
}
