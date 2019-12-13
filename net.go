package net

import (
	"context"
	"fmt"
	"io"

	"github.com/rtctunnel/crypt"
)

// Errors
var (
	ErrClosed = fmt.Errorf("closed: %w", io.EOF)
)

type PacketNetwork interface {
	io.Closer
	Recv(ctx context.Context, local crypt.PrivateKey) (remote crypt.PublicKey, data []byte, err error)
	Send(ctx context.Context, local crypt.PrivateKey, remote crypt.PublicKey, data []byte) error
}
