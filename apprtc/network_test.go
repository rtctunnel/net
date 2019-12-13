package apprtc

import (
	"context"
	"github.com/rtctunnel/crypt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

func TestNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	k1, _ := crypt.Generate()
	k2, _ := crypt.Generate()
	n := NewNetwork()
	defer n.Close()

	var eg errgroup.Group
	eg.Go(func() error {
		return n.Send(ctx, k1, k2.PublicKey(), []byte("HELLO WORLD"))
	})
	eg.Go(func() error {
		remote, data, err := n.Recv(ctx, k2)
		assert.Equal(t, k1.PublicKey(), remote)
		assert.Equal(t, []byte("HELLO WORLD"), data)
		return err
	})
	assert.NoError(t, eg.Wait())
}
