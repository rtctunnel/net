package webrtc

import (
	"context"
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
)

type Network struct {

}

func (n *Network) Accept(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, port int, stream net.Stream, err error) {
	panic("implement me")
}

func (n *Network) Open(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, port int) (stream net.Stream, err error) {
	panic("implement me")
}
