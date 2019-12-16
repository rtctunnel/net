package webrtc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/pion/webrtc/v2"
	"github.com/rs/zerolog/log"
	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
)

type MessageType string

const (
	MessageTypeOffer  MessageType = "offer"
	MessageTypeAnswer MessageType = "answer"
	MessageTypeReject MessageType = "reject"
)

type Message struct {
	Type          MessageType
	SDP           string
	ICECandidates []webrtc.ICECandidateInit
}

type networkForPeer struct {
	network *Network
	local   crypt.PrivateKey
	remote  crypt.PublicKey
	onClose func()

	pc       *webrtc.PeerConnection
	incoming chan *webrtc.DataChannel

	closeOnce sync.Once
	closed    chan struct{}

	iceCandidatesReadyOnce sync.Once
	iceCandidatesReady     chan struct{}
	iceCandidates          []webrtc.ICECandidateInit
}

func newNetworkForPeer(network *Network, local crypt.PrivateKey, remote crypt.PublicKey, onClose func()) (*networkForPeer, error) {
	nfp := &networkForPeer{
		network: network,
		local:   local,
		remote:  remote,
		onClose: onClose,

		incoming:           make(chan *webrtc.DataChannel, 1),
		closed:             make(chan struct{}),
		iceCandidatesReady: make(chan struct{}),
	}
	var err error
	nfp.pc, err = webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}},
	})
	if err != nil {
		return nil, err
	}

	nfp.pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Debug().
			Str("local", nfp.local.PublicKey().String()).
			Str("remote", nfp.remote.String()).
			Str("state", state.String()).
			Msg("[webrtc] onconnectionstatechange")
		if state == webrtc.PeerConnectionStateClosed {
			_ = nfp.Close()
		}
	})
	nfp.pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Debug().
			Str("local", nfp.local.PublicKey().String()).
			Str("remote", nfp.remote.String()).
			Str("label", dc.Label()).
			Msg("[webrtc] ondatachannel")
		select {
		case nfp.incoming <- dc:
		case <-nfp.closed:
		}
	})
	nfp.pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			nfp.iceCandidatesReadyOnce.Do(func() {
				close(nfp.iceCandidatesReady)
			})
		} else {
			log.Debug().
				Str("local", nfp.local.PublicKey().String()).
				Str("remote", nfp.remote.String()).
				Interface("candidate", candidate.ToJSON()).
				Msg("[webrtc] onicecandidate")

			nfp.iceCandidates = append(nfp.iceCandidates, candidate.ToJSON())
		}
	})
	nfp.pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Debug().
			Str("local", nfp.local.PublicKey().String()).
			Str("remote", nfp.remote.String()).
			Str("state", state.String()).
			Msg("[webrtc] oniceconnectionstatechange")
	})
	nfp.pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		log.Debug().
			Str("local", nfp.local.PublicKey().String()).
			Str("remote", nfp.remote.String()).
			Str("state", state.String()).
			Msg("[webrtc] onsignalingstatechange")
	})

	return nfp, nil
}

func (nfp *networkForPeer) accept(ctx context.Context) (port int, stream net.Stream, err error) {
	for {
		var dc *webrtc.DataChannel
		select {
		case <-ctx.Done():
			return 0, nil, ctx.Err()
		case <-nfp.closed:
			return 0, nil, net.ErrClosed
		case dc = <-nfp.incoming:
		}

		port, ok := parsePort(dc.Label())
		if !ok {
			_ = dc.Close()
			log.Warn().
				Str("label", dc.Label()).
				Str("local", nfp.local.PublicKey().String()).
				Str("remote", nfp.remote.String()).
				Msg("rejecting datachannel because it has an invalid label")
			continue
		}

		return port, newConnection(nfp, dc), nil
	}
}

func (nfp *networkForPeer) open(ctx context.Context, port int) (stream net.Stream, err error) {
	dc, err := nfp.pc.CreateDataChannel(fmt.Sprintf("rtctunnel:%d", port), nil)
	if err != nil {
		return nil, fmt.Errorf("[webrtc] error creating datachannel: %w", err)
	}

	if nfp.pc.ConnectionState() == webrtc.PeerConnectionStateNew {
		err = nfp.beginHandshake(ctx)
		if err != nil {
			_ = nfp.Close()
			return nil, err
		}
	}

	return newConnection(nfp, dc), nil
}

func (nfp *networkForPeer) handle(ctx context.Context, msg Message) error {
	switch msg.Type {
	case MessageTypeAnswer:
		return nfp.handleAnswer(ctx, msg)
	case MessageTypeOffer:
		return nfp.handleOffer(ctx, msg)
	case MessageTypeReject:
		return fmt.Errorf("peer connection was rejected")
	}

	return fmt.Errorf("unexpected message: %v", msg)
}

func (nfp *networkForPeer) handleAnswer(ctx context.Context, answer Message) error {
	if nfp.pc.SignalingState() != webrtc.SignalingStateHaveLocalOffer {
		return fmt.Errorf("unexpected message for signaling state (%s): %v", nfp.pc.SignalingState().String(), answer)
	}

	err := nfp.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  answer.SDP,
	})
	if err != nil {
		return fmt.Errorf("error setting remote answer description: %w", err)
	}

	for _, candidate := range answer.ICECandidates {
		err = nfp.pc.AddICECandidate(candidate)
		if err != nil {
			return fmt.Errorf("error adding ice candidate: %w", err)
		}
	}

	return nil
}

func (nfp *networkForPeer) handleOffer(ctx context.Context, offer Message) error {
	if nfp.pc.SignalingState() != webrtc.SignalingStateStable {
		return fmt.Errorf("unexpected message for signaling state (%s): %v", nfp.pc.SignalingState().String(), offer)
	}

	err := nfp.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offer.SDP,
	})
	if err != nil {
		return fmt.Errorf("error setting offer remote description: %w", err)
	}

	for _, candidate := range offer.ICECandidates {
		err = nfp.pc.AddICECandidate(candidate)
		if err != nil {
			return fmt.Errorf("error adding ice candidate: %w", err)
		}
	}

	answer, err := nfp.pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("error creating answer: %w", err)
	}

	err = nfp.pc.SetLocalDescription(answer)
	if err != nil {
		return fmt.Errorf("error setting answer local description: %w", err)
	}

	select {
	case <-ctx.Done():
		_ = nfp.Close()
		return ctx.Err()
	case <-nfp.closed:
		return net.ErrClosed
	case <-nfp.iceCandidatesReady:
	}

	msg := Message{
		Type:          MessageTypeAnswer,
		SDP:           answer.SDP,
		ICECandidates: nfp.iceCandidates,
	}
	err = nfp.send(ctx, msg)
	if err != nil {
		return err
	}

	return nil
}

func (nfp *networkForPeer) beginHandshake(ctx context.Context) error {
	offer, err := nfp.pc.CreateOffer(nil)
	if err != nil {
		_ = nfp.Close()
		return fmt.Errorf("error creating peerconnection offer: %w", err)
	}

	err = nfp.pc.SetLocalDescription(offer)
	if err != nil {
		_ = nfp.Close()
		return fmt.Errorf("error setting offer local description: %w", err)
	}

	select {
	case <-ctx.Done():
		_ = nfp.Close()
		return ctx.Err()
	case <-nfp.closed:
		return net.ErrClosed
	case <-nfp.iceCandidatesReady:
	}

	msg := Message{
		Type:          MessageTypeOffer,
		SDP:           offer.SDP,
		ICECandidates: nfp.iceCandidates,
	}
	err = nfp.send(ctx, msg)
	if err != nil {
		return err
	}

	return nil
}

func (nfp *networkForPeer) send(ctx context.Context, msg Message) error {
	wait := make(chan struct{})
	defer close(wait)

	ctx, onClose := context.WithCancel(ctx)
	go func() {
		select {
		case <-wait:
			return
		case <-nfp.closed:
			onClose()
		}
	}()

	bs, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshaling message: %w", err)
	}

	err = nfp.network.cfg.signal.Send(ctx, nfp.local, nfp.remote, bs)
	if err != nil {
		return fmt.Errorf("error sending message over signal: %w", err)
	}

	return nil
}

func (nfp *networkForPeer) Close() error {
	var err error
	nfp.closeOnce.Do(func() {
		err = nfp.pc.Close()
		close(nfp.closed)
		if nfp.onClose != nil {
			go nfp.onClose()
		}
	})
	return err
}

func parsePort(label string) (port int, ok bool) {
	idx := strings.LastIndexByte(label, ':')
	if idx < 0 {
		return 0, false
	}
	name := label[:idx]
	if name != "rtctunnel" {
		return 0, false
	}

	portstr := label[idx+1:]
	port, err := strconv.Atoi(portstr)
	if err != nil {
		return 0, false
	}

	return port, true
}
