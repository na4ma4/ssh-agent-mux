package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/na4ma4/go-slogtool"
	"github.com/na4ma4/ssh-agent-mux/internal/muxclient"
)

var ErrSocketActive = errors.New("socket is active and ssh-agent-mux is running")

func removeSocketIfExists(ctx context.Context, logger *slog.Logger, socketPath string) error {
	conn, _ := muxclient.NewMuxClient(logger, socketPath)

	// check if socket exists
	if socketExists, err := conn.SocketExists(ctx); err != nil {
		return err
	} else if !socketExists {
		return nil
	}

	if _, err := conn.Ping(ctx); err == nil {
		// if active return error
		logger.DebugContext(ctx, "Socket is active", slog.String("socket-path", socketPath))
		return fmt.Errorf("%w: %s is active, not removing", ErrSocketActive, socketPath)
	}

	// if not active, remove socket file
	if err := conn.RemoveSocket(ctx); err != nil {
		return err
	}

	logger.DebugContext(ctx, "Removed stale socket", slog.String("socket-path", socketPath))
	return nil
}

func printRunningConfig(ctx context.Context, logger *slog.Logger, socketPath string) error {
	logger.DebugContext(ctx, "ssh-agent-mux is already running on the socket",
		slog.String("socket-path", socketPath),
	)

	var muxClient *muxclient.MuxClient
	{
		var err error
		muxClient, err = muxclient.NewMuxClient(logger, socketPath)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to create mux client", slogtool.ErrorAttr(err))
			return err
		}
	}

	printCfg, err := muxClient.GetConfig(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "retrieving running ssh-agent-mux config failed", slogtool.ErrorAttr(err))
		return err
	}

	PrintConfig(printCfg)

	return nil
}

func waitForSocketToExist(ctx context.Context, logger *slog.Logger, socketPath string) error {
	logger.DebugContext(ctx, "Waiting for socket to be created", slog.String("socket-path", socketPath))

	for {
		logger.DebugContext(ctx, "Checking if socket exists")
		_, err := os.Stat(socketPath)
		if err == nil {
			logger.DebugContext(ctx, "Socket file exists", slog.String("socket-path", socketPath))
			break
		}
		if !os.IsNotExist(err) {
			logger.ErrorContext(ctx, "Error stating socket file",
				slog.String("socket-path", socketPath), slogtool.ErrorAttr(err),
			)
			return err
		}
		select {
		case <-ctx.Done():
			logger.DebugContext(ctx, "Context cancelled while waiting for socket to exist")
			return ctx.Err()
		default:
			// continue waiting
			time.Sleep(retryInterval)
		}
	}
	return nil
}
