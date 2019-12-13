package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
	"github.com/stretchr/testify/assert"
)

func TestPacketNetwork(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	k1, err := crypt.Generate()
	assert.NoError(t, err)
	k2, err := crypt.Generate()
	assert.NoError(t, err)

	var pn net.PacketNetwork = NewPacketNetwork()
	defer pn.Close()

	cnt := 5

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < cnt; i++ {
			err := pn.Send(ctx, k1, k2.PublicKey(), []byte{byte(i)})
			assert.NoError(t, err)
		}
	}()

	for i := 0; i < cnt; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			from, data, err := pn.Recv(ctx, k2)
			if assert.NoError(t, err) {
				assert.Len(t, data, 1)
				assert.Equal(t, k1.PublicKey(), from)
			}
		}()
	}
	wg.Wait()

}
