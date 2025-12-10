package muxclient

import (
	"context"
	"net"

	"github.com/google/uuid"
	"github.com/na4ma4/ssh-agent-mux/api"
	"github.com/na4ma4/ssh-agent-mux/internal/muxagent"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MuxClient represents a client connection to a mux agent.
type MuxClient struct {
	socketPath string
}

// NewMuxClient creates a new MuxClient connected to the specified socket path.
func NewMuxClient(socketPath string) (*MuxClient, error) {
	return &MuxClient{
		socketPath: socketPath,
	}, nil
}

// connect establishes a connection to the mux agent and returns an ExtendedAgent client.
func (c *MuxClient) connect(ctx context.Context) (agent.ExtendedAgent, func(), error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return nil, nil, err
	}

	muxClient := agent.NewClient(conn)

	return muxClient, func() { _ = conn.Close() }, nil
}

// Ping sends a ping request to the mux agent and returns the pong response.
func (c *MuxClient) Ping(ctx context.Context) (*api.Pong, error) {
	client, cancel, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	msg, err := muxagent.HandleExtensionProtoInvert[
		api.Ping, api.Pong,
	](
		api.Ping_builder{
			Id: proto.String(uuid.NewString()),
			Ts: timestamppb.Now(),
		}.Build(),
		func(inBytes []byte) ([]byte, error) {
			return client.Extension("ping", inBytes)
		},
	)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// GetConfig retrieves the current configuration from the mux agent.
func (c *MuxClient) GetConfig(ctx context.Context) (*api.Config, error) {
	client, cancel, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	configMsg, err := muxagent.HandleExtensionProtoInvert[
		api.ConfigRequest, api.Config,
	](
		api.ConfigRequest_builder{
			Id: proto.String(uuid.NewString()),
			Ts: timestamppb.Now(),
		}.Build(),
		func(inBytes []byte) ([]byte, error) {
			return client.Extension("config", inBytes)
		},
	)
	if err != nil {
		return nil, err
	}

	return configMsg, nil
}

// Shutdown sends a shutdown request to the mux agent and returns the response.
func (c *MuxClient) Shutdown(ctx context.Context) (*api.CommandResponse, error) {
	client, cancel, err := c.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer cancel()

	msg, err := muxagent.HandleExtensionProtoInvert[
		api.ShutdownRequest, api.CommandResponse,
	](
		api.ShutdownRequest_builder{
			Id: proto.String(uuid.NewString()),
			Ts: timestamppb.Now(),
		}.Build(),
		func(inBytes []byte) ([]byte, error) {
			return client.Extension("shutdown", inBytes)
		},
	)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
