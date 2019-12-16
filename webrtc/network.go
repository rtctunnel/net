package webrtc

import (
	"context"
	"sync"

	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
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

func (n *Network) Accept(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, port int, stream net.Stream, err error) {
	return n.getNetworkForKey(local).accept(ctx)
}

func (n *Network) Close() error {
	var err error
	n.mu.Lock()
	for key, nfk := range n.networks {
		e := nfk.Close()
		if err == nil {
			err = e
		}
		delete(n.networks, key)
	}
	n.mu.Unlock()
	return err
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
