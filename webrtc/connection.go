package webrtc

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/pion/webrtc/v2"
	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/net"
)

type connection struct {
	nfp *networkForPeer
	dc  *webrtc.DataChannel

	buf      bytes.Buffer
	incoming chan []byte

	openOnce sync.Once
	opened   chan struct{}

	closeOnce sync.Once
	closed    chan struct{}
}

func newConnection(nfp *networkForPeer, dc *webrtc.DataChannel) *connection {
	c := &connection{
		nfp: nfp,
		dc:  dc,

		incoming: make(chan []byte, 1),
		opened:   make(chan struct{}),
		closed:   make(chan struct{}),
	}
	c.dc.OnClose(func() {
		log.Debug().
			Str("label", c.dc.Label()).
			Str("local", c.nfp.local.PublicKey().String()).
			Str("remote", c.nfp.remote.String()).
			Msg("[datachannel] onclose")
		_ = c.Close()
	})
	c.dc.OnError(func(err error) {
		log.Debug().
			Str("label", c.dc.Label()).
			Str("local", c.nfp.local.PublicKey().String()).
			Str("remote", c.nfp.remote.String()).
			Err(err).
			Msg("[datachannel] onerror")
		_ = c.Close()
	})
	c.dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		log.Debug().
			Str("label", c.dc.Label()).
			Str("local", c.nfp.local.PublicKey().String()).
			Str("remote", c.nfp.remote.String()).
			Int("size", len(msg.Data)).
			Msg("[datachannel] onmessage")
		select {
		case c.incoming <- msg.Data:
		case <-c.closed:
		}
	})
	c.dc.OnOpen(func() {
		log.Debug().
			Str("label", c.dc.Label()).
			Str("local", c.nfp.local.PublicKey().String()).
			Str("remote", c.nfp.remote.String()).
			Msg("[datachannel] onopen")
		time.Sleep(time.Millisecond * 100)
		c.openOnce.Do(func() {
			close(c.opened)
		})
	})
	return c
}

func (c *connection) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.dc.Close()
		close(c.closed)
	})
	return err
}

func (c *connection) ReadContext(ctx context.Context, dst []byte) (n int, err error) {
	for {
		if c.buf.Len() > 0 {
			return c.buf.Read(dst)
		}

		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-c.closed:
			return 0, net.ErrClosed
		case msg := <-c.incoming:
			c.buf.Write(msg)
		}
	}
}

func (c *connection) WriteContext(ctx context.Context, src []byte) (n int, err error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-c.closed:
		return 0, net.ErrClosed
	case <-c.opened:
	}

	err = c.dc.Send(src)
	if err != nil {
		return 0, err
	}

	return len(src), nil
}
