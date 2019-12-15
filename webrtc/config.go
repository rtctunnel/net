package webrtc

import (
	"github.com/rtctunnel/net"
	"github.com/rtctunnel/net/apprtc"
)

type config struct {
	signal net.PacketNetwork
}

// An option customizes the config used for the apprtc Network.
type Option interface {
	apply(*config)
}

type funcOption func(*config)

func (f funcOption) apply(o *config) {
	f(o)
}

// WithSignal sets the signal in the config
func WithSignal(signal net.PacketNetwork) Option {
	return funcOption(func(cfg *config) {
		cfg.signal = signal
	})
}

func getConfig(options ...Option) *config {
	cfg := new(config)
	for _, o := range append([]Option{
		WithSignal(apprtc.NewNetwork()),
	}, options...) {
		o.apply(cfg)
	}
	return cfg
}
