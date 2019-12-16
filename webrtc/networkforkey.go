package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
	"github.com/rtctunnel/net/backoff"
)

type networkForKey struct {
	network *Network
	key     crypt.PrivateKey

	mu    sync.Mutex
	peers map[crypt.PublicKey]*networkForPeer

	incoming chan *networkForPeer

	closeOnce sync.Once
	closed    chan struct{}
}

func newNetworkForKey(network *Network, key crypt.PrivateKey) *networkForKey {
	nfk := &networkForKey{
		network:  network,
		key:      key,
		peers:    make(map[crypt.PublicKey]*networkForPeer),
		incoming: make(chan *networkForPeer, 16),
		closed:   make(chan struct{}),
	}
	go nfk.receiveStartStopper()
	return nfk
}

func (nfk *networkForKey) receiveStartStopper() {
	ctx, cancel := context.WithCancel(context.Background())
	running := false
	check := func() {
		nfk.mu.Lock()
		defer nfk.mu.Unlock()

		excess := map[crypt.PublicKey]struct{}{}
		for key := range nfk.peers {
			excess[key ] = struct{}{}
		}
		seen := 0
		for _, peer := range nfk.network.cfg.acceptPeers {
			delete(excess, peer)
			_, ok := nfk.peers[peer]
			if ok {
				seen++
			}
		}

		if running {
			if seen == len(nfk.network.cfg.acceptPeers) && len(excess) == 0 {
				cancel()
				running = false
			}
		} else {
			if nfk.network.cfg.acceptAllPeers || seen != len(nfk.network.cfg.acceptPeers) || len(excess) > 0 {
				ctx, cancel = context.WithCancel(context.Background())
				go nfk.receiver(ctx)
				running = true
			}
		}
	}
	check()

	if nfk.network.cfg.acceptAllPeers {
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-nfk.closed:
			return
		case <-ticker.C:
		}
		check()
	}
}

func (nfk *networkForKey) receiver(ctx context.Context) {
	b := backoff.New()

	for {
		// receive the next packet from the signal
		remote, data, err := nfk.network.cfg.signal.Recv(ctx, nfk.key)
		if errors.Is(err, context.Canceled) {
			return
		} else if err != nil {
			log.Error().Err(err).Msg("[webrtc] received error from signal")
			time.Sleep(b.Next())
			continue
		}

		// get the peer connection for this network
		nfp, isNew, err := nfk.getNetworkForPeer(remote)
		if err != nil {
			log.Error().Err(err).Msg("[webrtc] error creating peer connection")
			time.Sleep(b.Next())
			continue
		}

		b.Reset()

		var msg Message
		err = json.Unmarshal(data, &msg)
		if err != nil {
			log.Warn().Err(err).Msg("[webrtc] invalid message received from signal")
			continue
		}

		err = nfp.handle(ctx, msg)
		if err != nil {
			log.Warn().Err(err).Msg("[webrtc] error handling signal message, closing peer connection")
			_ = nfp.Close()
			continue
		}

		if isNew {
			select {
			case <-ctx.Done():
				return
			case nfk.incoming <- nfp:
			default:
				log.Warn().Err(err).Msg("[webrtc] accept buffer is full, closing peer connection")
				_ = nfp.Close()
			}
		}
	}
}

func (nfk *networkForKey) accept(ctx context.Context) (remote crypt.PublicKey, port int, stream net.Stream, err error) {
	var nfp *networkForPeer
	select {
	case <-ctx.Done():
		return remote, port, stream, ctx.Err()
	case nfp = <-nfk.incoming:
	}

	port, stream, err = nfp.accept(ctx)
	return nfp.remote, port, stream, err
}

func (nfk *networkForKey) open(ctx context.Context, remote crypt.PublicKey, port int) (stream net.Stream, err error) {
	nfp, _, err := nfk.getNetworkForPeer(remote)
	if err != nil {
		return nil, err
	}

	return nfp.open(ctx, port)
}

func (nfk *networkForKey) onClosePeer(nfp *networkForPeer) {
	nfk.mu.Lock()
	defer nfk.mu.Unlock()

	p, ok := nfk.peers[nfp.remote]
	if ok && p == nfp {
		delete(nfk.peers, nfp.remote)
	}
}

func (nfk *networkForKey) getNetworkForPeer(remote crypt.PublicKey) (nfp *networkForPeer, isNew bool, err error) {
	nfk.mu.Lock()
	defer nfk.mu.Unlock()
	nfp, ok := nfk.peers[remote]
	if !ok {
		isNew = true

		nfp, err = newNetworkForPeer(nfk.network, nfk.key, remote, func() {
			nfk.onClosePeer(nfp)
		})
		if err != nil {
			return nfp, isNew, err
		}

		nfk.peers[remote] = nfp
	}
	return nfp, isNew, err
}

func (nfk *networkForKey) Close() error {
	nfk.closeOnce.Do(func() {
		for _, conn := range nfk.peers {
			_ = conn.Close()
		}
		close(nfk.closed)
	})
	return nil
}
