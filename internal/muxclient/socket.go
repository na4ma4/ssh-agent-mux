package muxclient

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/na4ma4/go-slogtool"
)

func (c *MuxClient) RemoveSocket(ctx context.Context) error {
	if err := os.Remove(c.socketPath); err != nil {
		c.logger.DebugContext(ctx, "Failed to remove socket",
			slog.String("socket-path", c.socketPath), slogtool.ErrorAttr(err),
		)
		return fmt.Errorf("failed to remove socket %s: %w", c.socketPath, err)
	}

	return nil
}

func (c *MuxClient) SocketExists(ctx context.Context) (bool, error) {
	// check if socket exists
	stat, err := os.Stat(c.socketPath)
	if os.IsNotExist(err) {
		c.logger.DebugContext(ctx, "Socket does not exist", slog.String("socket-path", c.socketPath))
		return false, nil
	} else if err != nil {
		c.logger.DebugContext(ctx,
			"Failed to stat socket",
			slog.String("socket-path", c.socketPath),
			slogtool.ErrorAttr(err),
		)
		return false, err
	}

	// check if file is a socket
	if stat.Mode()&os.ModeSocket == 0 {
		c.logger.DebugContext(ctx, "File exists and is not a socket", slog.String("socket-path", c.socketPath))
		return true, fmt.Errorf("file %s exists and is not a socket", c.socketPath)
	}

	return true, nil
}

func (c *MuxClient) IsSocketWorking(ctx context.Context) bool {
	// attempt to connect to socket to see if it is active
	if resp, err := c.Ping(ctx); err == nil {
		c.logger.DebugContext(ctx, "Socket is active",
			slog.String("socket-path", c.socketPath),
			slog.String("response.ID", resp.GetId()),
			slog.String("response.Version", resp.GetVersion()),
			slog.Int64("response.PID", resp.GetPid()),
			slog.Time("response.Timestamp", resp.GetTs().AsTime()),
		)
		return true
	}

	return false
}

// func IsSocketWorking(ctx context.Context, logger *slog.Logger, socketPath string) bool {
// 	// attempt to connect to socket to see if it is active
// 	conn, _ := NewMuxClient(logger, socketPath)
// 	if resp, err := conn.Ping(ctx); err == nil {
// 		// if active return true
// 		logger.DebugContext(ctx, "Socket is active",
// 			slog.String("socket-path", socketPath),
// 			slog.String("response.ID", resp.GetId()),
// 			slog.Time("response.Timestamp", resp.GetTs().AsTime()),
// 		)
// 		return true
// 	}

// 	return false
// }
