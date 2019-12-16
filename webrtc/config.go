package webrtc

import (
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
	"github.com/rtctunnel/net/apprtc"
)

type config struct {
	signal         net.PacketNetwork
	acceptAllPeers bool
	acceptPeers    []crypt.PublicKey
}

// An option customizes the config used for the apprtc Network.
type Option interface {
	apply(*config)
}

type funcOption func(*config)

func (f funcOption) apply(o *config) {
	f(o)
}

// WithAcceptAllPeers tells the WebRTC network to continuously accept peer connections.
func WithAcceptAllPeers() Option {
	return funcOption(func(cfg *config) {
		cfg.acceptAllPeers = true
	})
}

// WithAcceptPeers tells the WebRTC network to accept a static list of peers. Once those peers are connected the WebRTC
// network will stop listening for messages from the signal network.
func WithAcceptPeers(peers ...crypt.PublicKey) Option {
	return funcOption(func(cfg *config) {
		cfg.acceptAllPeers = false
		cfg.acceptPeers = append(cfg.acceptPeers, peers...)
	})
}

// WithSignal sets the signal in the config.
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
