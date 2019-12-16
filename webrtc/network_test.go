package webrtc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
	"github.com/rtctunnel/net/memory"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

func TestNetwork(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	k1, _ := crypt.Generate()
	k2, _ := crypt.Generate()

	signal := memory.NewPacketNetwork()
	defer signal.Close()

	n1 := NewNetwork(WithSignal(signal))
	defer n1.Close()

	n2 := NewNetwork(WithSignal(signal), WithAcceptPeers(k1.PublicKey()))
	defer n2.Close()

	msg := []byte("Hello World")

	var eg errgroup.Group
	eg.Go(func() error {
		stream, err := n1.Open(ctx, k1, k2.PublicKey(), 5000)
		if assert.NoError(t, err) {
			n, err := stream.WriteContext(ctx, msg)
			assert.Equal(t, len(msg), n)
			assert.NoError(t, err)
		}
		return nil
	})
	eg.Go(func() error {
		remote, port, stream, err := n2.Accept(ctx, k2)
		if assert.NoError(t, err) {
			assert.Equal(t, k1.PublicKey(), remote)
			assert.Equal(t, 5000, port)

			bs := make([]byte, len(msg))
			_, err := io.ReadFull(net.StreamReader(stream), bs)
			assert.Equal(t, msg, bs)
			assert.NoError(t, err)

			stream.Close()
		}
		return nil
	})

	eg.Wait()

}
