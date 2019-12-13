package backoff

import (
	"math/rand"
	"time"
)

type config struct {
	initialDelay time.Duration
	jitter       bool
	multiplier   float64
	maxDelay     time.Duration
	random       *rand.Rand
}

type Option interface {
	apply(*config)
}

type funcOption func(*config)

func (f funcOption) apply(cfg *config) {
	f(cfg)
}

// WithInitialDelay sets the initial delay option.
func WithInitialDelay(delay time.Duration) Option {
	return funcOption(func(cfg *config) {
		cfg.initialDelay = delay
	})
}

// WithJitter sets the jitter option.
func WithJitter(jitter bool) Option {
	return funcOption(func(cfg *config) {
		cfg.jitter = jitter
	})
}

// WithMaxDelay sets the max delay option.
func WithMaxDelay(delay time.Duration) Option {
	return funcOption(func(cfg *config) {
		cfg.maxDelay = delay
	})
}

// WithMultiplier sets the multiplier option.
func WithMultiplier(multiplier float64) Option {
	return funcOption(func(cfg *config) {
		cfg.multiplier = multiplier
	})
}

// WithRandom sets the random option.
func WithRandom(random *rand.Rand) Option {
	return funcOption(func(cfg *config) {
		cfg.random = random
	})
}

func getConfig(options ...Option) *config {
	cfg := new(config)
	for _, o := range append([]Option{
		WithInitialDelay(time.Millisecond * 100),
		WithMaxDelay(time.Second * 30),
		WithJitter(true),
		WithMultiplier(1.5),
		WithRandom(rand.New(rand.NewSource(time.Now().UnixNano()))),
	}, options...) {
		o.apply(cfg)
	}
	return cfg
}
