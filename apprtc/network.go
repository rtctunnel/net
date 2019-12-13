package apprtc

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/mr-tron/base58"
	"github.com/rtctunnel/net"
	"github.com/rtctunnel/net/websocketmanager"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net/backoff"
)

// Network is an rtctunnel/net.Network implemented using the apprtc websocket.
type Network struct {
	cfg *config
	mgr *websocketmanager.Manager

	closeOnce sync.Once
	closed    chan struct{}
}

// NewNetwork creates a new Network.
func NewNetwork(options ...Option) *Network {
	return &Network{
		cfg:    getConfig(options...),
		mgr:    websocketmanager.New(),
		closed: make(chan struct{}),
	}
}

func (p *Network) Close() error {
	p.closeOnce.Do(func() {
		close(p.closed)
		p.mgr.Close()
	})
	return nil
}

func (p *Network) Recv(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, data []byte, err error) {
	b := backoff.New()
	url := p.cfg.url + "/ws"

	for {
		var encoded string
		err := p.mgr.WithConnection(ctx, url, func(conn *websocket.Conn, isNew bool) error {
			if isNew {
				err := conn.WriteJSON(map[string]interface{}{
					"cmd":      "register",
					"roomid":   local.PublicKey().String(),
					"clientid": local.PublicKey().String(),
				})
				if err != nil {
					return fmt.Errorf("error registering apprtc room: %w", err)
				}
			}

			var packet struct {
				Message string `json:"msg"`
				Error   string `json:"error"`
			}
			if deadline, ok := ctx.Deadline(); ok {
				conn.SetReadDeadline(deadline)
				defer conn.SetReadDeadline(time.Time{})
			}
			err := conn.ReadJSON(&packet)
			if err != nil {
				return err
			}
			if packet.Error != "" {
				return errors.New(packet.Error)
			}

			encoded = packet.Message

			return nil
		})
		var encrypted []byte
		if err == nil {
			encrypted, err = base58.Decode(encoded)
		}
		if err == nil {
			remote, data, err = local.Decrypt(encrypted)
		}
		if err == nil {
			log.Debug().
				Str("url", url).
				Str("local", local.PublicKey().String()).
				Str("remote", remote.String()).
				Int("data-size", len(data)).
				Msg("[apprtc] recv")
			return remote, data, nil
		}

		log.Error().
			Err(err).
			Str("url", url).
			Str("local", local.PublicKey().String()).
			Str("remote", remote.String()).
			Msg("[apprtc] recv returned an error")

		select {
		case <-p.closed:
			return remote, data, net.ErrClosed
		case <-ctx.Done():
			return remote, data, ctx.Err()
		case <-time.After(b.Next()):
		}
	}
}

// Send sends a packet of data to the remote peer over apprtc.
func (p *Network) Send(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, data []byte) error {
	b := backoff.New()
	encrypted := local.Encrypt(remote, data)
	encoded := base58.Encode(encrypted)

	url := p.cfg.url + "/" + remote.String() + "/$"
	req, err := http.NewRequest("POST", url, strings.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("error creating http request: %w", err)
	}
	req = req.WithContext(ctx)

	log.Debug().
		Str("url", url).
		Str("local", local.PublicKey().String()).
		Str("remote", remote.String()).
		Int("encoded-size", len(encoded)).
		Msg("[apprtc] send")

	for {
		res, err := http.DefaultClient.Do(req)
		if res != nil && res.Body != nil {
			_, _ = ioutil.ReadAll(res.Body)
			_ = res.Body.Close()
		}
		if err == nil {
			return nil
		}

		log.Error().
			Err(err).
			Str("url", url).
			Str("local", local.PublicKey().String()).
			Str("remote", remote.String()).
			Msg("[apprtc] error sending message")

		select {
		case <-time.After(b.Next()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
