package muxclient

import (
	"context"
	"log/slog"
	"net"
	"sync"

	"golang.org/x/crypto/ssh/agent"
)

// A CloseFunc tells an operation to abandon its work.
// A CloseFunc does not wait for the work to stop.
// A CloseFunc may be called by multiple goroutines simultaneously.
// After the first call, subsequent calls to a CloseFunc do nothing.
type CloseFunc func()

// MuxClient represents a client connection to a mux agent.
type MuxClient struct {
	logger        *slog.Logger
	socketPath    string
	closeFuncOnce sync.Once
}

// NewMuxClient creates a new MuxClient connected to the specified socket path.
func NewMuxClient(logger *slog.Logger, socketPath string) (*MuxClient, error) {
	return &MuxClient{
		logger:        logger,
		socketPath:    socketPath,
		closeFuncOnce: sync.Once{},
	}, nil
}

// connect establishes a connection to the mux agent and returns an ExtendedAgent client.
func (c *MuxClient) connect(ctx context.Context) (agent.ExtendedAgent, CloseFunc, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return nil, nil, err
	}

	muxClient := agent.NewClient(conn)

	return muxClient, func() { c.closeFuncOnce.Do(func() { _ = conn.Close() }) }, nil
}
