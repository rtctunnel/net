package memory

import (
	"bytes"
	"context"
	"sync"

	"github.com/rtctunnel/crypt"
	"github.com/rtctunnel/net"
)

type openRequest struct {
	client crypt.PublicKey
	port   int
	stream *Stream
}

type StreamNetwork struct {
	mu       sync.Mutex
	incoming map[crypt.PublicKey]chan openRequest

	closeOnce sync.Once
	closed    chan struct{}
}

// NewStreamNetwork creates a new StreamNetwork.
func NewStreamNetwork() *StreamNetwork {
	return &StreamNetwork{
		incoming: make(map[crypt.PublicKey]chan openRequest),
		closed:   make(chan struct{}),
	}
}

// Accept accepts a new stream to local from the given remote key and port.
func (sn *StreamNetwork) Accept(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, port int, stream net.Stream, err error) {
	ch := sn.getChannel(local.PublicKey())
	select {
	case <-sn.closed:
		return remote, port, stream, net.ErrClosed
	case <-ctx.Done():
		return remote, port, stream, ctx.Err()
	case req := <-ch:
		return req.client, req.port, req.stream, nil
	}
}

// Close closes the StreamNetwork so that Accept/Open will return an error. Any currently opened streams are not closed.
func (sn *StreamNetwork) Close() error {
	sn.closeOnce.Do(func() {
		close(sn.closed)
	})
	return nil
}

// Open opens a new stream to remote key on the given port.
func (sn *StreamNetwork) Open(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, port int) (stream net.Stream, err error) {
	ch := sn.getChannel(remote)
	req := openRequest{
		client: local.PublicKey(),
		port:   port,
		stream: NewStream(),
	}
	select {
	case <-sn.closed:
		_ = req.stream.Close()
		return stream, net.ErrClosed
	case <-ctx.Done():
		_ = req.stream.Close()
		return stream, ctx.Err()
	case ch <- req:
		return req.stream, nil
	}
}

func (sn *StreamNetwork) getChannel(server crypt.PublicKey) chan openRequest {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	ch, ok := sn.incoming[server]
	if !ok {
		ch = make(chan openRequest)
		sn.incoming[server] = ch
	}
	return ch
}

// A Stream is a full-duplex, in-memory stream of bytes.
type Stream struct {
	buf bytes.Buffer
	ch  chan []byte

	closeOnce sync.Once
	closed    chan struct{}
}

// NewStream creates a new in-memory stream.
func NewStream() *Stream {
	return &Stream{
		ch:     make(chan []byte),
		closed: make(chan struct{}),
	}
}

// Close closes the stream.
func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	return nil
}

// ReadContext reads up to len(p) bytes into p. It returns the number of bytes read (0 <= n <= len(p))
// and any error encountered. Even if Read returns n < len(p), it may use all of p as scratch
// space during the call. If some data is available but not len(p) bytes, Read returns what is
// available instead of waiting for more.
func (s *Stream) ReadContext(ctx context.Context, dst []byte) (n int, err error) {
	select {
	case <-s.closed:
		return 0, net.ErrClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	for {
		if s.buf.Len() > 0 {
			return s.buf.Read(dst)
		}

		select {
		case <-s.closed:
			return 0, net.ErrClosed
		case <-ctx.Done():
			return 0, ctx.Err()
		case data := <-s.ch:
			s.buf.Write(data)
		}
	}
}

// WriteContext writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written from p (0 <= n <= len(p)) and any error
// encountered that caused the write to stop early.
//
// WriteContext will return a non-nil error if it returns n < len(p).
// It neither retains nor modifies the data in p.
func (s *Stream) WriteContext(ctx context.Context, p []byte) (n int, err error) {
	data := make([]byte, len(p))
	copy(data, p)
	select {
	case <-s.closed:
		return 0, net.ErrClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	case s.ch <- data:
		return len(data), nil
	}
}
