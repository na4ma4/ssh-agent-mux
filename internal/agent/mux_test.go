package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestNewMuxAgent(t *testing.T) {
	// Test creating agent without fallback
	muxAgent, err := NewMuxAgent("")
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	if muxAgent.localKeys == nil {
		t.Error("localKeys map should be initialized")
	}
}

func TestAddAndListKeys(t *testing.T) {
	muxAgent, err := NewMuxAgent("")
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
	muxAgent, err := NewMuxAgent("")
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
	muxAgent, err := NewMuxAgent("")
	if err != nil {
		t.Fatalf("Failed to create mux agent: %v", err)
	}
	defer muxAgent.Close()

	// Add multiple keys
	for i := 0; i < 3; i++ {
		_, privateKey, err := ed25519.GenerateKey(rand.Reader)
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
	}

	// Verify keys were added
	keys, err := muxAgent.List()
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("Expected 3 keys, got %d", len(keys))
	}

	// Remove all keys
	err = muxAgent.RemoveAll()
	if err != nil {
		t.Fatalf("Failed to remove all keys: %v", err)
	}

	// Verify all keys were removed
	keys, err = muxAgent.List()
	if err != nil {
		t.Fatalf("Failed to list keys: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("Expected 0 keys after RemoveAll, got %d", len(keys))
	}
}

func TestSignWithKey(t *testing.T) {
	muxAgent, err := NewMuxAgent("")
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
	muxAgent, err := NewMuxAgent("")
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
