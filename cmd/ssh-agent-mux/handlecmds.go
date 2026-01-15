package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/na4ma4/go-slogtool"
	"github.com/na4ma4/go-timestring"
	"github.com/na4ma4/ssh-agent-mux/api"
	"github.com/na4ma4/ssh-agent-mux/internal/muxclient"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/protojson"
)

func handleCommand(ctx context.Context, logger *slog.Logger, command string) error {
	logger.DebugContext(ctx, "Executing command", slog.String("command", command))

	var socketPath string
	{
		socketPaths := viper.GetStringSlice("backend-agent")
		if len(socketPaths) == 0 {
			if socketPath = viper.GetString("socket"); socketPath == "" {
				return errors.New("no backend agent socket specified for command mode")
			}
		}
		socketPath = socketPaths[0]
	}

	var socket *muxclient.MuxClient
	{
		var err error
		socket, err = muxclient.NewMuxClient(logger, socketPath)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to create mux client", slogtool.ErrorAttr(err))
			return err
		}

		if !socket.IsSocketWorking(ctx) && viper.GetString("socket") != socketPath {
			logger.DebugContext(ctx, "Socket is not active, attempting to connect to default socket",
				slog.String("socket-path", socketPath),
				slog.String("default-socket-path", viper.GetString("socket")),
			)

			socket, err = muxclient.NewMuxClient(logger, viper.GetString("socket"))
			if err != nil {
				logger.ErrorContext(ctx, "Failed to create mux client for default socket", slogtool.ErrorAttr(err))
				return err
			}
		}
	}

	switch command {
	case "ping":
		return handleCommandPing(ctx, logger, socket)
	case "shutdown", "close", "stop":
		return handleCommandShutdown(ctx, logger, socket)
	case "config", "config-json":
		return handleCommandConfig(ctx, logger, socket, command)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func handleCommandPing(ctx context.Context, logger *slog.Logger, socket *muxclient.MuxClient) error {
	logger.DebugContext(ctx, "handleCommandPing()")
	pongMsg, err := socket.Ping(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Ping command failed", slogtool.ErrorAttr(err))
		return err
	}
	recvTS := time.Now()

	fmt.Fprintln(os.Stdout, "Received ping reply")
	fmt.Fprintf(os.Stdout, "  ID=%s\n", pongMsg.GetId())
	fmt.Fprintf(os.Stdout, "  PID=%d\n", pongMsg.GetPid())
	fmt.Fprintf(os.Stdout, "  Version=%s\n", pongMsg.GetVersion())

	if pongMsg.HasStartTime() {
		fmt.Fprintf(os.Stdout, "  Mux Agent Start Time=%s (%s)\n",
			pongMsg.GetStartTime().AsTime().String(),
			timestring.ShortProcess.String(time.Since(pongMsg.GetStartTime().AsTime())),
		)
	}

	fmt.Fprintln(os.Stdout, "  Latency:")
	fmt.Fprintf(os.Stdout, "    Recv TS=%s\n", pongMsg.GetTs().AsTime().String())

	if pongMsg.HasPingTs() {
		fmt.Fprintf(os.Stdout, "    Sent TS=%s\n", pongMsg.GetPingTs().AsTime().String())
		fmt.Fprintf(os.Stdout, "    Latency To Mux=%s\n",
			timestring.ShortProcess.String(pongMsg.GetTs().AsTime().Sub(pongMsg.GetPingTs().AsTime())),
		)

		fmt.Fprintf(os.Stdout, "    Latency From Mux=%s\n",
			timestring.ShortProcess.String(recvTS.Sub(pongMsg.GetTs().AsTime())),
		)

		fmt.Fprintf(os.Stdout, "    Total Roundtrip Latency=%s\n",
			timestring.ShortProcess.String(recvTS.Sub(pongMsg.GetPingTs().AsTime())),
		)
	}

	return nil
}

func handleCommandShutdown(ctx context.Context, logger *slog.Logger, socket *muxclient.MuxClient) error {
	shutdownMsg, err := socket.Shutdown(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create mux client", slogtool.ErrorAttr(err))
		return err
	}

	fmt.Fprintf(os.Stdout, "Received shutdown response: ID=%s, TS=%s, Status=%t, Message=%s\n",
		shutdownMsg.GetId(), shutdownMsg.GetTs().AsTime().String(),
		shutdownMsg.GetSuccess(), shutdownMsg.GetMessage(),
	)

	return nil
}

func handleCommandConfig(ctx context.Context, logger *slog.Logger, socket *muxclient.MuxClient, command string) error {
	var configMsg *api.Config
	{
		var err error
		configMsg, err = socket.GetConfig(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "Config command failed", slogtool.ErrorAttr(err))
			return err
		}
	}

	if command == "config-json" {
		configJSON, err := protojson.Marshal(configMsg)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to marshal config to JSON", slogtool.ErrorAttr(err))
			return err
		}

		fmt.Fprintln(os.Stdout, string(configJSON))
		return nil
	}

	fmt.Fprintln(os.Stdout, "Received config:")
	fmt.Fprintf(os.Stdout, "  Socket Path: %s\n", configMsg.GetSocketPath())
	fmt.Fprintln(os.Stdout, "  Backend Socket Paths:")
	for _, backendPath := range configMsg.GetBackendSocketPath() {
		fmt.Fprintf(os.Stdout, "   - %s\n", backendPath)
	}
	fmt.Fprintf(os.Stdout, "  PID: %d\n", configMsg.GetPid())
	//nolint:gosmopolitan // I want local time here
	fmt.Fprintf(os.Stdout, "  Start Time: %s\n", configMsg.GetStartTime().AsTime().Local().String())
	fmt.Fprintf(os.Stdout, "  Version: %s\n", configMsg.GetVersion())

	return nil
}
