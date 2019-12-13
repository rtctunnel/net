package websocketmanager

import (
	"time"
)

type config struct {
	idleTimeout time.Duration
}

// An option customizes the config used for the apprtc Network.
type Option interface {
	apply(*config)
}

type funcOption func(*config)

func (f funcOption) apply(o *config) {
	f(o)
}

// WithIdleTimeout sets the idle timeout in the config.
func WithIdleTimeout(idleTimeout time.Duration) Option {
	return funcOption(func(o *config) {
		o.idleTimeout = idleTimeout
	})
}

func getConfig(options ...Option) *config {
	cfg := new(config)
	for _, o := range append([]Option{
		WithIdleTimeout(time.Minute),
	}, options...) {
		o.apply(cfg)
	}
	return cfg
}
