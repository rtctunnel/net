package apprtc

type config struct {
	url string
}

// An option customizes the config used for the apprtc Network.
type Option interface {
	apply(*config)
}

type funcOption func(*config)

func (f funcOption) apply(o *config) {
	f(o)
}

// WithURL sets the URL in the config
func WithURL(url string) Option {
	return funcOption(func(o *config) {
		o.url = url
	})
}

func getConfig(options ...Option) *config {
	cfg := new(config)
	for _, o := range append([]Option{
		WithURL("https://apprtc-ws.webrtc.org"),
	}, options...) {
		o.apply(cfg)
	}
	return cfg
}
