package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/na4ma4/ssh-agent-mux/internal/agent"
	sshagent "golang.org/x/crypto/ssh/agent"
)

var (
	socketPath   = flag.String("socket", getDefaultSocketPath(), "Path to Unix socket for the agent")
	fallbackPath = flag.String("fallback", os.Getenv("SSH_AUTH_SOCK"), "Path to fallback SSH agent socket")
	verbose      = flag.Bool("verbose", false, "Enable verbose logging")
)

func getDefaultSocketPath() string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, fmt.Sprintf("ssh-agent-mux-%d.sock", os.Getuid()))
}

func main() {
	flag.Parse()

	if *verbose {
		log.Println("Starting SSH Agent Multiplexer")
		log.Printf("Socket path: %s", *socketPath)
		log.Printf("Fallback agent: %s", *fallbackPath)
	}

	// Create the multiplexing agent
	muxAgent, err := agent.NewMuxAgent(*fallbackPath)
	if err != nil {
		log.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	// Remove existing socket if it exists
	if err := os.Remove(*socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove existing socket: %v", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on socket: %v", err)
	}
	defer listener.Close()
	defer os.Remove(*socketPath)

	// Set socket permissions
	if err := os.Chmod(*socketPath, 0600); err != nil {
		log.Fatalf("Failed to set socket permissions: %v", err)
	}

	if *verbose {
		log.Printf("Listening on %s", *socketPath)
	}

	fmt.Printf("SSH Agent Multiplexer started\n")
	fmt.Printf("Set SSH_AUTH_SOCK=%s in your environment to use this agent\n", *socketPath)

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Accept connections in a goroutine
	connChan := make(chan net.Conn)
	errChan := make(chan error)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				errChan <- err
				return
			}
			connChan <- conn
		}
	}()

	// Main event loop
	for {
		select {
		case sig := <-sigChan:
			if *verbose {
				log.Printf("Received signal %v, shutting down", sig)
			}
			return
		case err := <-errChan:
			log.Printf("Listener error: %v", err)
			return
		case conn := <-connChan:
			if *verbose {
				log.Println("New connection accepted")
			}
			go handleConnection(conn, muxAgent, *verbose)
		}
	}
}

func handleConnection(conn net.Conn, muxAgent *agent.MuxAgent, verbose bool) {
	defer conn.Close()

	if verbose {
		log.Printf("Handling connection from %s", conn.RemoteAddr())
	}

	// Serve the agent protocol on this connection
	if err := sshagent.ServeAgent(muxAgent, conn); err != nil {
		if verbose {
			log.Printf("Error serving agent: %v", err)
		}
	}

	if verbose {
		log.Printf("Connection closed: %s", conn.RemoteAddr())
	}
}
