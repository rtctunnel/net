package backoff

import (
	"time"
)

// A Backoff is used to retry operations with exponential, jittered backoff.
type Backoff struct {
	cfg *config
	cur time.Duration
}

// New creates a new Backoff.
func New(options ...Option) *Backoff {
	return &Backoff{
		cfg: getConfig(options...),
	}
}

// Next gets the next backoff duration.
func (b *Backoff) Next() time.Duration {
	if b.cur < b.cfg.initialDelay {
		b.cur = b.cfg.initialDelay
	} else {
		b.cur = time.Duration(float64(b.cur) * b.cfg.multiplier)
	}

	if b.cur > b.cfg.maxDelay {
		b.cur = b.cfg.maxDelay
	}

	next := b.cur
	// choose a value within [initial, next)
	if b.cfg.jitter {
		diff := float64(next - b.cfg.initialDelay)
		next = b.cfg.initialDelay + time.Duration(diff*b.cfg.random.Float64())
	}
	return next
}

// Reset resets the backoff
func (b *Backoff) Reset() {
	b.cur = 0
}
