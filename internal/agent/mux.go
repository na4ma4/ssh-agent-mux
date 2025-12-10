package agent

import (
	"crypto"
	"fmt"
	"net"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// MuxAgent implements an SSH agent that stores keys locally and falls back to another agent
type MuxAgent struct {
	localKeys     map[string]*agent.AddedKey
	keysMutex     sync.RWMutex
	fallbackConn  net.Conn
	fallbackAgent agent.ExtendedAgent
}

// NewMuxAgent creates a new multiplexing SSH agent
func NewMuxAgent(fallbackSocketPath string) (*MuxAgent, error) {
	m := &MuxAgent{
		localKeys: make(map[string]*agent.AddedKey),
	}

	// Connect to fallback agent if path is provided
	if fallbackSocketPath != "" {
		conn, err := net.Dial("unix", fallbackSocketPath)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to fallback agent: %w", err)
		}
		m.fallbackConn = conn
		m.fallbackAgent = agent.NewClient(conn)
	}

	return m, nil
}

// Close closes the connection to the fallback agent
func (m *MuxAgent) Close() error {
	if m.fallbackConn != nil {
		return m.fallbackConn.Close()
	}
	return nil
}

// List returns the identities known to the agent
func (m *MuxAgent) List() ([]*agent.Key, error) {
	m.keysMutex.RLock()
	defer m.keysMutex.RUnlock()

	var keys []*agent.Key

	// Add local keys first
	for _, addedKey := range m.localKeys {
		// Convert PrivateKey interface{} to crypto.Signer
		signer, ok := addedKey.PrivateKey.(crypto.Signer)
		if !ok {
			continue
		}

		pubKey := signer.Public()
		sshPubKey, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			continue
		}

		key := &agent.Key{
			Format:  sshPubKey.Type(),
			Blob:    sshPubKey.Marshal(),
			Comment: addedKey.Comment,
		}
		keys = append(keys, key)
	}

	// Add keys from fallback agent
	if m.fallbackAgent != nil {
		fallbackKeys, err := m.fallbackAgent.List()
		if err == nil {
			keys = append(keys, fallbackKeys...)
		}
	}

	return keys, nil
}

// Sign signs data with the key identified by the given public key
func (m *MuxAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return m.SignWithFlags(key, data, 0)
}

// SignWithFlags signs data with the key identified by the given public key and flags
func (m *MuxAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	keyBlob := key.Marshal()
	keyString := string(keyBlob)

	// Try local keys first
	m.keysMutex.RLock()
	addedKey, found := m.localKeys[keyString]
	m.keysMutex.RUnlock()

	if found {
		signer, err := ssh.NewSignerFromKey(addedKey.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create signer from local key: %w", err)
		}

		// Handle signature flags
		if flags != 0 {
			if algoSigner, ok := signer.(ssh.AlgorithmSigner); ok {
				var algorithm string
				switch flags {
				case agent.SignatureFlagRsaSha256:
					algorithm = ssh.SigAlgoRSASHA2256
				case agent.SignatureFlagRsaSha512:
					algorithm = ssh.SigAlgoRSASHA2512
				default:
					algorithm = key.Type()
				}
				return algoSigner.SignWithAlgorithm(nil, data, algorithm)
			}
		}

		return signer.Sign(nil, data)
	}

	// Fall back to fallback agent
	if m.fallbackAgent != nil {
		if extAgent, ok := m.fallbackAgent.(agent.ExtendedAgent); ok {
			return extAgent.SignWithFlags(key, data, flags)
		}
		return m.fallbackAgent.Sign(key, data)
	}

	return nil, fmt.Errorf("key not found")
}

// Add adds a private key to the local agent
func (m *MuxAgent) Add(key agent.AddedKey) error {
	m.keysMutex.Lock()
	defer m.keysMutex.Unlock()

	// Convert PrivateKey interface{} to crypto.Signer to get public key
	signer, ok := key.PrivateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("private key does not implement crypto.Signer")
	}

	pubKey := signer.Public()
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to convert public key: %w", err)
	}

	keyBlob := sshPubKey.Marshal()
	keyString := string(keyBlob)

	m.localKeys[keyString] = &key

	return nil
}

// Remove removes a key from the local agent
func (m *MuxAgent) Remove(key ssh.PublicKey) error {
	m.keysMutex.Lock()
	defer m.keysMutex.Unlock()

	keyBlob := key.Marshal()
	keyString := string(keyBlob)

	delete(m.localKeys, keyString)

	return nil
}

// RemoveAll removes all keys from the local agent
func (m *MuxAgent) RemoveAll() error {
	m.keysMutex.Lock()
	defer m.keysMutex.Unlock()

	m.localKeys = make(map[string]*agent.AddedKey)

	return nil
}

// Lock is not implemented (not needed for this use case)
func (m *MuxAgent) Lock(passphrase []byte) error {
	return fmt.Errorf("locking not supported")
}

// Unlock is not implemented (not needed for this use case)
func (m *MuxAgent) Unlock(passphrase []byte) error {
	return fmt.Errorf("unlocking not supported")
}

// Signers returns signers for all local keys
func (m *MuxAgent) Signers() ([]ssh.Signer, error) {
	m.keysMutex.RLock()
	defer m.keysMutex.RUnlock()

	var signers []ssh.Signer
	for _, addedKey := range m.localKeys {
		signer, err := ssh.NewSignerFromKey(addedKey.PrivateKey)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}

	// Add signers from fallback agent
	if m.fallbackAgent != nil {
		fallbackSigners, err := m.fallbackAgent.Signers()
		if err == nil {
			signers = append(signers, fallbackSigners...)
		}
	}

	return signers, nil
}

// Extension processes extension requests (not implemented)
func (m *MuxAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	// Try fallback agent for extensions
	if m.fallbackAgent != nil {
		return m.fallbackAgent.Extension(extensionType, contents)
	}
	return nil, agent.ErrExtensionUnsupported
}
