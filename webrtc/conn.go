package webrtc

import (
	"fmt"
	"github.com/pion/webrtc/v2"
)

type MessageType string

const (
	MessageTypeOffer  MessageType = "offer"
	MessageTypeAnswer MessageType = "answer"
	MessageTypeReject MessageType = "reject"
)

type Message struct {
	Type          MessageType
	Port          int
	SDP           string
	ICECandidates []webrtc.ICECandidateInit
}

type connection struct {
	peerConnection *webrtc.PeerConnection
}

func (c *connection) handleMessage(recv Message) (send []Message, err error) {
	switch c.peerConnection.SignalingState() {
	case webrtc.SignalingStateStable:
	case webrtc.SignalingStateHaveLocalOffer:
	case webrtc.SignalingStateHaveRemoteOffer:
	case webrtc.SignalingStateClosed:
		switch recv.Type {
		case MessageTypeOffer:
			return c.handleOffer(recv)
		}
	}

	return nil, fmt.Errorf("unexpected message (type=%s signaling-state=%s)", recv.Type, c.peerConnection.SignalingState().String())
}

func (c *connection) handleOffer(recv Message) (send []Message, err error) {
	err = c.peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  recv.SDP,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting remote description: %w", err)
	}

	for _, candidate := range recv.ICECandidates {
		err = c.peerConnection.AddICECandidate(candidate)
		if err != nil {
			return nil, fmt.Errorf("error adding remote ice candidate: %w", err)
		}
	}

	sdp, err := c.peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating local answer: %w", err)
	}

	err = c.peerConnection.SetLocalDescription(sdp)
	if err != nil {
		return nil, fmt.Errorf("error setting local description: %w", err)
	}

	return []Message{
		{
			Type: MessageTypeAnswer,
			Port: recv.Port,
			SDP:  sdp.SDP,
		},
	}, nil
}
