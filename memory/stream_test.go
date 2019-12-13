package memory

import (
	"context"
	"github.com/rtctunnel/crypt"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

func TestStreamNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	k1, _ := crypt.Generate()
	k2, _ := crypt.Generate()
	sn := NewStreamNetwork()

	var eg errgroup.Group
	eg.Go(func() error {
		stream, err := sn.Open(ctx, k1, k2.PublicKey(), 80)
		assert.NotNil(t, stream)
		return err
	})
	eg.Go(func() error {
		remote, port, stream, err := sn.Accept(ctx, k2)
		assert.Equal(t, k1.PublicKey(), remote)
		assert.Equal(t, 80, port)
		assert.NotNil(t, stream)
		return err
	})

	assert.NoError(t, eg.Wait())
}

func TestStreamNetworkCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	k1, _ := crypt.Generate()
	k2, _ := crypt.Generate()
	sn := NewStreamNetwork()

	var eg errgroup.Group
	eg.Go(func() error {
		cancel()
		return nil
	})
	eg.Go(func() error {
		_, err := sn.Open(ctx, k1, k2.PublicKey(), 80)
		assert.Equal(t, context.Canceled, err, "context cancellation should cancel Open")
		return nil
	})
	eg.Go(func() error {
		_, _, _, err := sn.Accept(ctx, k2)
		assert.Equal(t, context.Canceled, err, "context cancelation should cancel Accept")
		return nil
	})

	assert.NoError(t, eg.Wait())
}
