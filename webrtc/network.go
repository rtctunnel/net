package webrtc

import (
	"context"
	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
	"sync"
)

type Network struct {
	cfg *config

	mu       sync.Mutex
	networks map[crypt.PublicKey]*networkForKey
}

// NewNetwork creates a new Network.
func NewNetwork(options ...Option) *Network {
	n := &Network{
		cfg:      getConfig(options...),
		networks: map[crypt.PublicKey]*networkForKey{},
	}
	return n
}

func (n *Network) AddRoute(remote crypt.PublicKey, port int) {
	panic("AddRoute not implemented")
}

func (n *Network) Accept(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, port int, stream net.Stream, err error) {
	return n.getNetworkForKey(local).accept(ctx)
}

func (n *Network) Open(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, port int) (stream net.Stream, err error) {
	return n.getNetworkForKey(local).open(ctx, remote, port)
}

func (n *Network) getNetworkForKey(key crypt.PrivateKey) *networkForKey {
	n.mu.Lock()
	nfk, ok := n.networks[key.PublicKey()]
	if !ok {
		nfk = newNetworkForKey(n, key)
		n.networks[key.PublicKey()] = nfk
	}
	n.mu.Unlock()
	return nfk
}

type networkForKey struct {
	network *Network
	key     crypt.PrivateKey
}

func newNetworkForKey(network *Network, key crypt.PrivateKey) *networkForKey {
	nfk := &networkForKey{
		network: network,
		key:     key,
	}
	return nfk
}

func (nfk *networkForKey) handler() {
	for {
		remote, data, err := nfk.network.cfg.signal.Recv(context.Background(), nfk.key)
		if err != nil {
			log.Error().Err(err).Msg("[webrtc] received error from signal")
			continue
		}


	}
}

func (nfk *networkForKey) accept(ctx context.Context) (remote crypt.PublicKey, port int, stream net.Stream, err error) {
	panic("Accept not implemented")
}

func (nfk *networkForKey) open(ctx context.Context, remote crypt.PublicKey, port int) (stream net.Stream, err error) {
	panic("Open not implemented")
}
