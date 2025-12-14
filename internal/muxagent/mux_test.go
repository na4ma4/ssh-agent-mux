package muxagent_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"log/slog"
	"os"
	"testing"

	"github.com/na4ma4/go-contextual"
	"github.com/na4ma4/go-permbits"
	"github.com/na4ma4/ssh-agent-mux/api"
	"github.com/na4ma4/ssh-agent-mux/internal/muxagent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/protobuf/proto"
)

func defaultConfig() *api.Config {
	return api.Config_builder{
		SocketPath:        proto.String(""),
		BackendSocketPath: []string{},
	}.Build()
}

func TestPermbits(t *testing.T) {
	var expected os.FileMode = 0o0600
	actual := permbits.MustString("u=rw,a=")
	if actual != expected {
		t.Errorf("Expected perm string %s, got %s", expected, actual)
	}
}

func TestNewMuxAgent(t *testing.T) {
	// Test creating agent without backend sockets
	muxAgent, err := muxagent.NewMuxAgent(contextual.New(t.Context()), slog.New(slog.DiscardHandler), defaultConfig())
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	if muxAgent.GetLocalKeys() == nil {
		t.Error("localKeys map should be initialized")
	}
}

func TestAddAndListKeys(t *testing.T) {
	muxAgent, err := muxagent.NewMuxAgent(contextual.New(t.Context()), slog.New(slog.DiscardHandler), defaultConfig())
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	// Generate a test key
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add the key
	addedKey := agent.AddedKey{
		PrivateKey: privateKey,
		Comment:    "test-key",
	}
	err = muxAgent.Add(addedKey)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	// List keys
	keys, err := muxAgent.List()
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(keys))
	}

	if keys[0].Comment != "test-key" {
		t.Errorf("Expected comment 'test-key', got '%s'", keys[0].Comment)
	}
}

func TestRemoveKey(t *testing.T) {
	muxAgent, err := muxagent.NewMuxAgent(contextual.New(t.Context()), slog.New(slog.DiscardHandler), defaultConfig())
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	// Generate and add a test key
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	addedKey := agent.AddedKey{
		PrivateKey: privateKey,
		Comment:    "test-key",
	}
	err = muxAgent.Add(addedKey)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	// Convert to SSH public key
	sshPubKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert to SSH public key: %v", err)
	}

	// Remove the key
	err = muxAgent.Remove(sshPubKey)
	if err != nil {
		t.Fatalf("Failed to remove key: %v", err)
	}

	// Verify key was removed
	keys, err := muxAgent.List()
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}

	if len(keys) != 0 {
		t.Fatalf("Expected 0 keys after removal, got %d", len(keys))
	}
}

func TestRemoveAllKeys(t *testing.T) {
	var muxAgent *muxagent.MuxAgent
	{
		var err error
		muxAgent, err = muxagent.NewMuxAgent(
			contextual.New(t.Context()), slog.New(slog.DiscardHandler), defaultConfig(),
		)
		if err != nil {
			t.Fatalf("Failed to create mux agent: %v", err)
		}
		defer muxAgent.Close()
	}

	// Add multiple keys
	for range 3 {
		var privateKey ed25519.PrivateKey
		{
			var err error
			_, privateKey, err = ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatalf("Failed to generate key: %v", err)
			}
		}

		addedKey := agent.AddedKey{
			PrivateKey: privateKey,
			Comment:    "test-key",
		}
		if err := muxAgent.Add(addedKey); err != nil {
			t.Fatalf("Failed to add key: %v", err)
		}
	}

	// Verify keys were added
	var keys []*agent.Key
	{
		var err error
		keys, err = muxAgent.List()
		if err != nil {
			t.Fatalf("Failed to list keys: %v", err)
		}
		if len(keys) != 3 {
			t.Fatalf("Expected 3 keys, got %d", len(keys))
		}
	}

	// Remove all keys
	if err := muxAgent.RemoveAll(); err != nil {
		t.Fatalf("Failed to remove all keys: %v", err)
	}

	// Verify all keys were removed
	{
		var err error
		keys, err = muxAgent.List()
		if err != nil {
			t.Fatalf("Failed to list keys: %v", err)
		}
		if len(keys) != 0 {
			t.Fatalf("Expected 0 keys after RemoveAll, got %d", len(keys))
		}
	}
}

func TestSignWithKey(t *testing.T) {
	muxAgent, err := muxagent.NewMuxAgent(contextual.New(t.Context()), slog.New(slog.DiscardHandler), defaultConfig())
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	// Generate a test key
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Add the key to the agent
	addedKey := agent.AddedKey{
		PrivateKey: privateKey,
		Comment:    "test-key",
	}
	err = muxAgent.Add(addedKey)
	if err != nil {
		t.Fatalf("Failed to add key: %v", err)
	}

	// Convert to SSH public key
	sshPubKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert to SSH public key: %v", err)
	}

	// Test data to sign
	data := []byte("test data to sign")

	// Sign the data
	signature, err := muxAgent.Sign(sshPubKey, data)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	// Verify the signature
	err = sshPubKey.Verify(data, signature)
	if err != nil {
		t.Fatalf("Signature verification failed: %v", err)
	}
}

func TestSignWithNonExistentKey(t *testing.T) {
	muxAgent, err := muxagent.NewMuxAgent(contextual.New(t.Context()), slog.New(slog.DiscardHandler), defaultConfig())
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	// Generate a key but don't add it
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Convert to SSH public key
	sshPubKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to convert to SSH public key: %v", err)
	}

	// Try to sign with a key that's not in the agent
	data := []byte("test data to sign")
	_, err = muxAgent.Sign(sshPubKey, data)
	if err == nil {
		t.Error("Expected error when signing with non-existent key, got nil")
	}
}
