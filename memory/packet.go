package memory

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
)

type message struct {
	from, to crypt.PublicKey
	data     []byte
}

type pendingMessage struct {
	ready bool
	msg   message
	wait  chan struct{}
}

// A PacketNetwork sends packets of data between peers in-memory.
type PacketNetwork struct {
	mu       sync.Mutex
	messages map[crypt.PublicKey]pendingMessage

	closeOnce sync.Once
	closed    chan struct{}
}

// NewPacketNetwork creates a new PacketNetwork.
func NewPacketNetwork() *PacketNetwork {
	return &PacketNetwork{
		messages: make(map[crypt.PublicKey]pendingMessage),
		closed:   make(chan struct{}),
	}
}

// Close closes the packet network. Any pending operations will return an error.
func (p *PacketNetwork) Close() error {
	p.closeOnce.Do(func() {
		close(p.closed)
	})
	return nil
}

// Recv receives a packet of data from a peer in-memory.
func (p *PacketNetwork) Recv(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, data []byte, err error) {
	defer func() {
		log.Debug().
			Str("local-id", local.PublicKey().String()).
			Str("remote-id", remote.String()).
			Bytes("data", data).
			Err(err).
			Msg("[memory] recv")
	}()

	dst := local.PublicKey()
	for {
		var wait chan struct{}

		p.mu.Lock()
		pmsg, ok := p.messages[dst]
		if !ok {
			wait = make(chan struct{})
			p.messages[dst] = pendingMessage{
				wait: wait,
			}
		} else if pmsg.ready {
			remote = pmsg.msg.from
			data = pmsg.msg.data
			delete(p.messages, dst)
			close(pmsg.wait)
		} else {
			wait = pmsg.wait
		}
		p.mu.Unlock()

		if wait == nil {
			return remote, data, nil
		}

		select {
		case <-ctx.Done():
			return remote, nil, ctx.Err()
		case <-p.closed:
			return remote, nil, net.ErrClosed
		case <-wait:
		}
	}
}

// Send sends a packet of data to a peer in-memory.
func (p *PacketNetwork) Send(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, data []byte) error {
	dst := remote

	log.Debug().
		Str("local-id", local.PublicKey().String()).
		Str("remote-id", remote.String()).
		Bytes("data", data).
		Msg("[memory] send")

	for {
		var wait chan struct{}

		p.mu.Lock()
		pmsg, ok := p.messages[dst]
		if !ok {
			p.messages[dst] = pendingMessage{
				ready: true,
				msg:   message{from: local.PublicKey(), to: remote, data: data},
				wait:  make(chan struct{}),
			}
		} else if pmsg.ready {
			wait = pmsg.wait
		} else {
			p.messages[dst] = pendingMessage{
				ready: true,
				msg:   message{from: local.PublicKey(), to: remote, data: data},
				wait:  make(chan struct{}),
			}
			close(pmsg.wait)
		}
		p.mu.Unlock()

		if wait == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.closed:
			return net.ErrClosed
		case <-wait:
		}
	}
}
