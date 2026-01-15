package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/dosquad/go-cliversion"
	"github.com/na4ma4/go-contextual"
	"github.com/na4ma4/go-permbits"
	"github.com/na4ma4/go-slogtool"
	"github.com/na4ma4/ssh-agent-mux/api"
	"github.com/na4ma4/ssh-agent-mux/internal/daemon"
	"github.com/na4ma4/ssh-agent-mux/internal/muxagent"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var rootCmd = &cobra.Command{
	Use:     "ssh-agent-mux",
	Short:   "ssh-agent-mux - SSH Agent Multiplexer",
	Long:    `ssh-agent-mux is an SSH agent that multiplexes local keys with one or more backend SSH agents.`,
	RunE:    mainCommand,
	Version: cliversion.Get().VersionString(),
}

// ErrSignalReceived indicates that a termination signal was received.
var ErrSignalReceived = errors.New("signal received, shutting down")

func init() {
	_ = rootCmd.PersistentFlags().BoolP("debug", "d", false, "Debug output")
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindEnv("debug", "DEBUG")

	_ = rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Quiet output")
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))

	_ = rootCmd.PersistentFlags().BoolP("foreground", "f", false, "Run in foreground (do not daemonize)")
	_ = viper.BindPFlag("foreground", rootCmd.PersistentFlags().Lookup("foreground"))
	_ = viper.BindEnv("foreground", "SSH_AGENT_MUX_FOREGROUND")

	_ = rootCmd.PersistentFlags().StringP("socket", "s", getDefaultSocketPath(),
		"Path to Unix socket for the agent")
	_ = viper.BindPFlag("socket", rootCmd.PersistentFlags().Lookup("socket"))
	_ = viper.BindEnv("socket", "SSH_AGENT_MUX_SOCKET")

	_ = rootCmd.PersistentFlags().StringArrayP("backend-agent", "p", []string{os.Getenv("SSH_AUTH_SOCK")},
		"Path to proxied SSH agent socket")
	_ = viper.BindPFlag("backend-agent", rootCmd.PersistentFlags().Lookup("backend-agent"))
	_ = viper.BindEnv("backend-agent", "SSH_AUTH_SOCK")

	_ = rootCmd.PersistentFlags().StringP("log-path", "l", "",
		"Path to log file (default: stderr)")
	_ = viper.BindPFlag("log-path", rootCmd.PersistentFlags().Lookup("log-path"))
	_ = viper.BindEnv("log-path", "SSH_AGENT_MUX_LOGPATH")

	_ = rootCmd.PersistentFlags().StringP("command", "c", "",
		"Command to run instead of the agent")
	_ = viper.BindPFlag("command", rootCmd.PersistentFlags().Lookup("command"))
	_ = viper.BindEnv("command", "SSH_AGENT_MUX_COMMAND")
}

func getDefaultSocketPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}

	return filepath.Join(homeDir, ".ssh", "ssh-agent-mux.sock")
}

func main() {
	_ = rootCmd.Execute()
}

func getLogger() (*slog.Logger, func()) {
	logLevel := slog.LevelInfo
	if viper.GetBool("debug") {
		logLevel = slog.LevelDebug
	}

	if logPath := viper.GetString("log-path"); logPath != "" {
		f, err := os.Create(logPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open log file: %v", err)
			return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})), func() {}
		}

		return slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: logLevel})), func() { _ = f.Close() }
	}

	if !viper.GetBool("debug") {
		return slog.New(slog.DiscardHandler), func() {}
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})), func() {}
}

func mainCommand(cmd *cobra.Command, _ []string) error {
	ctx := contextual.NewCancellable(context.Background())
	defer ctx.Cancel()

	cmd.SilenceUsage = true

	var logger *slog.Logger
	{
		var closer func()
		logger, closer = getLogger()
		defer closer()
	}

	if command := viper.GetString("command"); command != "" {
		return handleCommand(ctx, logger, command)
	}

	config := api.Config_builder{
		SocketPath: proto.String(viper.GetString("socket")),
		BackendSocketPath: append(
			[]string{},
			viper.GetStringSlice("backend-agent")...,
		),
		StartTime:   timestamppb.Now(),
		Version:     proto.String(cliversion.Get().VersionString()),
		VersionInfo: cliversion.Get(),
		Pid:         proto.Int64(int64(os.Getpid())),
	}.Build()

	if viper.GetBool("foreground") {
		logger.DebugContext(ctx, "Running in foreground mode")
	} else {
		logger.DebugContext(ctx, "Daemonizing process")
		go daemon.Ize(daemon.WithNoRestart(), daemon.WithNoExit())

		if !daemon.AmI() {
			return runMainProgramDaemonMode(ctx, logger)
		}
	}

	logger.DebugContext(ctx, "Starting SSH Agent Multiplexer",
		slog.String("socket-path", viper.GetString("socket")),
		slog.String("backend-socket-path", viper.GetString("backend-agent")),
	)

	// Check socket and remove if it exists and is not active
	if rmErr := removeSocketIfExists(ctx, logger, viper.GetString("socket")); rmErr != nil {
		if errors.Is(rmErr, ErrSocketActive) {
			return printRunningConfig(ctx, logger, viper.GetString("socket"))
		}

		logger.ErrorContext(ctx, "Failed to remove existing socket", slogtool.ErrorAttr(rmErr))
		return rmErr
	}

	// Create the multiplexing agent
	var muxAgent *muxagent.MuxAgent
	{
		var err error
		muxAgent, err = muxagent.NewMuxAgent(ctx, logger, config)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to create mux agent", slogtool.ErrorAttr(err))
			return err
		}
		defer muxAgent.Close()
	}

	// Create Unix socket listener
	var listener net.Listener
	{
		var err error
		var cancel func()
		listener, cancel, err = makeListener(ctx, viper.GetString("socket"))
		if err != nil {
			logger.ErrorContext(ctx, "Failed to listen on socket", slogtool.ErrorAttr(err))
			return err
		}
		defer cancel()
	}

	// Set socket permissions
	if err := os.Chmod(viper.GetString("socket"), permbits.MustString("u=rw,a=")); err != nil {
		logger.ErrorContext(ctx, "Failed to set socket permissions", slogtool.ErrorAttr(err))
		return err
	}

	logger.DebugContext(ctx, "Listening", slog.String("socket-path", viper.GetString("socket")))

	PrintConfig(config)

	// Run the main event loop
	return wrapEventLoop(ctx, logger, listener, muxAgent)
}

func runMainProgramDaemonMode(ctx context.Context, logger *slog.Logger) error {
	logger.DebugContext(ctx, "Main Process in Daemon Procedure, returning config and exiting.")
	ctx, cancel := contextual.WithTimeout(ctx, timeoutForSocketCreation)
	defer cancel()

	if err := waitForSocketToExist(ctx, logger, viper.GetString("socket")); err != nil {
		logger.ErrorContext(ctx, "Timeout waiting for socket to be created", slogtool.ErrorAttr(err))
		return err
	}

	// Check socket and remove if it exists and is not active
	if err := removeSocketIfExists(ctx, logger, viper.GetString("socket")); err != nil {
		if errors.Is(err, ErrSocketActive) {
			return printRunningConfig(ctx, logger, viper.GetString("socket"))
		}

		logger.ErrorContext(ctx, "Failed to remove existing socket", slogtool.ErrorAttr(err))
		return err
	}

	return printRunningConfig(ctx, logger, viper.GetString("socket"))
}

const (
	defaultSignalChannelBufferSize = 1
)

func wrapEventLoop(ctx context.Context, logger *slog.Logger, listener net.Listener, muxAgent *muxagent.MuxAgent) error {
	if err := runEventLoop(ctx, logger, listener, muxAgent); err != nil {
		if errors.Is(err, context.Canceled) {
			logger.DebugContext(ctx, "Shutting down gracefully", slog.String("reason", err.Error()))
			return nil
		}
		if errors.Is(err, io.EOF) {
			logger.DebugContext(ctx, "Shutting down on EOF", slog.String("reason", err.Error()))
			return nil
		}
		if errors.Is(err, ErrSignalReceived) {
			logger.InfoContext(ctx, "Shutting down on signal", slog.String("reason", err.Error()))
			return nil
		}

		logger.ErrorContext(ctx, "Error in event loop", slogtool.ErrorAttr(err))
		return err
	}

	return nil
}

func runEventLoop(ctx context.Context, logger *slog.Logger, listener net.Listener, muxAgent *muxagent.MuxAgent) error {
	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, defaultSignalChannelBufferSize)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Track active connections for graceful shutdown
	var wg sync.WaitGroup

	// Accept connections in a goroutine
	connChan := make(chan net.Conn)
	errChan := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case errChan <- err:
				default:
					// Channel full or main loop already exited, ignore
				}
				return
			}
			select {
			case connChan <- conn:
			case <-ctx.Done():
				// Context cancelled, close the connection and exit
				_ = conn.Close()
				return
			}
		}
	}()

	// Main event loop
	for {
		select {
		case <-ctx.Done():
			// Close listener to unblock Accept goroutine
			_ = listener.Close()
			// Wait for active connections to finish
			wg.Wait()
			return ctx.Err()
		case sig := <-sigChan:
			logger.DebugContext(ctx, "Received signal, shutting down", slog.String("signal", sig.String()))
			// Close listener to unblock Accept goroutine
			_ = listener.Close()
			// Wait for active connections to finish
			wg.Wait()
			return fmt.Errorf("%w: %s", ErrSignalReceived, sig.String())
		case err := <-errChan:
			logger.ErrorContext(ctx, "Listener error", slogtool.ErrorAttr(err))
			// Wait for active connections to finish
			wg.Wait()
			return err
		case conn := <-connChan:
			logger.DebugContext(ctx, "New connection accepted", slog.String("remote-addr", conn.RemoteAddr().String()))
			wg.Add(1)
			go handleConnection(ctx, logger, conn, muxAgent, &wg)
		}
	}
}

func handleConnection(
	ctx context.Context, logger *slog.Logger, conn net.Conn,
	muxAgent *muxagent.MuxAgent, wg *sync.WaitGroup,
) {
	defer wg.Done()
	defer conn.Close()

	logger.DebugContext(ctx, "Handling connection", slog.String("remote-addr", conn.RemoteAddr().String()))

	// Serve the agent protocol on this connection
	if err := agent.ServeAgent(muxAgent, conn); err != nil && !errors.Is(err, io.EOF) {
		logger.ErrorContext(ctx, "Error serving agent", slogtool.ErrorAttr(err))
	}

	logger.DebugContext(ctx, "Connection closed",
		slog.String("local-addr", conn.LocalAddr().String()),
		slog.String("remote-addr", conn.RemoteAddr().String()),
	)
}

func makeListener(ctx context.Context, socketPath string) (net.Listener, func(), error) {
	// Create Unix socket listener
	listenConfig := &net.ListenConfig{}
	listener, err := listenConfig.Listen(ctx, "unix", socketPath)
	if err != nil {
		return nil, nil, err
	}

	return listener, func() {
		_ = listener.Close()
		_ = os.Remove(socketPath)
	}, nil
}
