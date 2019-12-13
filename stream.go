package net

import (
	"context"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/rtctunnel/crypt"
)

// A StreamNetwork is a network that supports creating streams.
type StreamNetwork interface {
	Accept(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, port int, stream Stream, err error)
	Open(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, port int) (stream Stream, err error)
}

// A Stream is a connection-oriented network connection that can send and receive a stream of data.
type Stream interface {
	io.Closer
	ReadContext(ctx context.Context, dst []byte) (n int, err error)
	WriteContext(ctx context.Context, src []byte) (n int, err error)
}

type streamAddr struct {
	label string
}

func (sa streamAddr) Network() string {
	return "stream"
}

func (sa streamAddr) String() string {
	return sa.label
}

type streamReader struct {
	Stream
}

// StreamReader wraps a stream so that it can be used like an io.Reader.
func StreamReader(stream Stream) io.ReadCloser {
	return streamReader{stream}
}

func (sr streamReader) Read(p []byte) (n int, err error) {
	return sr.Stream.ReadContext(context.Background(), p)
}

type streamWriter struct {
	Stream
}

// StreamWriter wraps a stream so that it can be used like an io.Writer.
func StreamWriter(stream Stream) io.WriteCloser {
	return streamWriter{stream}
}

func (sw streamWriter) Write(p []byte) (n int, err error) {
	return sw.Stream.WriteContext(context.Background(), p)
}

type streamConn struct {
	stream Stream

	readDeadline, writeDeadline atomic.Value
}

// StreamConn wraps a stream so that it can be used like a net.Conn.
func StreamConn(stream Stream) net.Conn {
	sc := &streamConn{
		stream: stream,
	}
	sc.readDeadline.Store(time.Time{})
	sc.writeDeadline.Store(time.Time{})
	return sc
}

func (sc *streamConn) Close() error {
	return sc.stream.Close()
}

func (sc *streamConn) Read(b []byte) (n int, err error) {
	ctx := context.Background()
	readDeadline := sc.readDeadline.Load().(time.Time)
	if !readDeadline.IsZero() {
		var cancel func()
		ctx, cancel = context.WithDeadline(ctx, readDeadline)
		defer cancel()
	}
	return sc.stream.ReadContext(ctx, b)
}

func (sc *streamConn) Write(b []byte) (n int, err error) {
	ctx := context.Background()
	writeDeadline := sc.writeDeadline.Load().(time.Time)
	if !writeDeadline.IsZero() {
		var cancel func()
		ctx, cancel = context.WithDeadline(ctx, writeDeadline)
		defer cancel()
	}
	return sc.stream.WriteContext(ctx, b)
}

func (sc *streamConn) LocalAddr() net.Addr {
	return streamAddr{"local"}
}

func (sc *streamConn) RemoteAddr() net.Addr {
	return streamAddr{"remote"}
}

func (sc *streamConn) SetDeadline(t time.Time) error {
	var err error
	if e := sc.SetReadDeadline(t); e != nil {
		err = e
	}
	if e := sc.SetWriteDeadline(t); e != nil {
		err = e
	}
	return err
}

func (sc *streamConn) SetReadDeadline(t time.Time) error {
	sc.readDeadline.Store(t)
	return nil
}

func (sc *streamConn) SetWriteDeadline(t time.Time) error {
	sc.writeDeadline.Store(t)
	return nil
}
