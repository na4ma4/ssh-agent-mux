package muxagent

import (
	"crypto"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"syscall"

	"github.com/dosquad/go-cliversion"
	"github.com/google/uuid"
	"github.com/na4ma4/go-contextual"
	"github.com/na4ma4/go-slogtool"
	"github.com/na4ma4/ssh-agent-mux/api"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrUnimplemented indicates that a method is not implemented.
var ErrUnimplemented = errors.New("not implemented")

// MuxAgent implements an SSH agent that stores keys locally and checks backend agents for readonly keys.
type MuxAgent struct {
	ctx       contextual.Context
	logger    *slog.Logger
	localKeys map[string]*agent.AddedKey
	keysMutex sync.RWMutex
	config    *api.Config
}

// NewMuxAgent creates a new multiplexing SSH agent.
func NewMuxAgent(ctx contextual.Context, logger *slog.Logger, config *api.Config) (*MuxAgent, error) {
	logger.DebugContext(ctx, "Creating new MuxAgent",
		slog.Any("backend-socket-path", config.GetBackendSocketPath()),
	)

	m := &MuxAgent{
		ctx:       ctx,
		logger:    logger,
		localKeys: make(map[string]*agent.AddedKey),
		config:    config,
	}

	return m, nil
}

var errExitBackendLoop = errors.New("exit backend loop")

func (m *MuxAgent) runAgainstBackends(f func(agent.ExtendedAgent) error) error {
	for _, socketPath := range m.config.GetBackendSocketPath() {
		fb, fbClose, err := m.backendConnect(socketPath)
		if err != nil {
			m.logger.DebugContext(m.ctx, "Failed to connect to backend agent",
				slog.String("socket-path", socketPath),
				slogtool.ErrorAttr(err),
			)
			continue
		}

		// Call function and ensure connection is closed
		err = f(fb)
		fbClose()

		if err != nil {
			m.logger.DebugContext(m.ctx, "Function against backend agent failed",
				slog.String("socket-path", socketPath),
				slogtool.ErrorAttr(err),
			)

			return err
		}
	}

	return nil
}

func (m *MuxAgent) backendConnect(socketPath string) (agent.ExtendedAgent, func(), error) {
	if socketPath == "" {
		return nil, nil, ErrUnimplemented
	}

	conn, err := (&net.Dialer{}).DialContext(m.ctx, "unix", socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to backend agent: %w", err)
	}

	return agent.NewClient(conn), func() { _ = conn.Close() }, nil
}

// Close closes the connection to the backend agent.
func (m *MuxAgent) Close() error {
	m.logger.DebugContext(m.ctx, "Close called")

	return nil
}

// func (m *MuxAgent) cleanupLocalKeys() {
// 	m.keysMutex.Lock()
// 	defer m.keysMutex.Unlock()

// 	newLocalKeys := make(map[string]*agent.AddedKey)
// 	for keyString, addedKey := range m.localKeys {
// 		newLocalKeys[keyString] = addedKey
// 	}

// 	m.localKeys = newLocalKeys
// }

// List returns the identities known to the agent.
func (m *MuxAgent) List() ([]*agent.Key, error) {
	m.logger.DebugContext(m.ctx, "List called")

	m.keysMutex.RLock()
	defer m.keysMutex.RUnlock()

	keys := make([]*agent.Key, 0, len(m.localKeys))
	m.logger.DebugContext(m.ctx, "Listing local keys", slog.Int("local-key-count", len(m.localKeys)))

	// Add local keys first
	for _, addedKey := range m.localKeys {
		m.logger.DebugContext(m.ctx, "Processing local key with comment", slog.String("key-comment", addedKey.Comment))
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

	if err := m.runAgainstBackends(func(fb agent.ExtendedAgent) error {
		// Add keys from backend agent
		backendKeys, err := fb.List()
		if err != nil {
			return fmt.Errorf("failed to list keys from backend agent: %w", err)
		}
		m.logger.DebugContext(m.ctx, "Listing backend keys", slog.Int("backend-key-count", len(backendKeys)))
		keys = append(keys, backendKeys...)
		return nil
	}); err != nil {
		m.logger.DebugContext(m.ctx, "Failed to list keys from backend agents", slogtool.ErrorAttr(err))
	}

	return keys, nil
}

// Sign signs data with the key identified by the given public key.
func (m *MuxAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	m.logger.DebugContext(m.ctx, "Sign called with key", slog.String("key-type", key.Type()))
	return m.SignWithFlags(key, data, 0)
}

// SignWithFlags signs data with the key identified by the given public key and flags.
func (m *MuxAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	m.logger.DebugContext(m.ctx, "SignWithFlags called with key",
		slog.String("key-type", key.Type()),
		slog.Int("flags", int(flags)),
	)
	keyBlob := key.Marshal()

	// Try local keys first
	m.keysMutex.RLock()
	addedKey, found := m.localKeys[string(keyBlob)]
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
					algorithm = ssh.KeyAlgoRSASHA256
				case agent.SignatureFlagRsaSha512:
					algorithm = ssh.KeyAlgoRSASHA512
				case agent.SignatureFlagReserved:
					algorithm = key.Type()
				default:
					algorithm = key.Type()
				}
				return algoSigner.SignWithAlgorithm(rand.Reader, data, algorithm)
			}
		}

		return signer.Sign(rand.Reader, data)
	}

	var returnedSig *ssh.Signature
	if err := m.runAgainstBackends(func(fb agent.ExtendedAgent) error {
		sig, err := fb.SignWithFlags(key, data, flags)
		if err != nil {
			return err
		}
		if sig != nil {
			m.logger.DebugContext(m.ctx, "Signature obtained from backend agent",
				slog.String("key-type", key.Type()),
			)
			returnedSig = sig
			return errExitBackendLoop
		}
		return errors.New("key not found in backend agent")
	}); errors.Is(err, errExitBackendLoop) {
		return returnedSig, nil
	}

	return nil, errors.New("key not found")
}

// Add adds a private key to the local agent.
func (m *MuxAgent) Add(key agent.AddedKey) error {
	m.logger.DebugContext(m.ctx, "Add called with key comment", slog.String("key-comment", key.Comment))

	m.keysMutex.Lock()
	defer m.keysMutex.Unlock()

	// Convert PrivateKey interface{} to crypto.Signer to get public key
	signer, ok := key.PrivateKey.(crypto.Signer)
	if !ok {
		return errors.New("private key does not implement crypto.Signer")
	}

	pubKey := signer.Public()
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to convert public key: %w", err)
	}

	keyBlob := sshPubKey.Marshal()
	keyString := string(keyBlob)

	m.logger.DebugContext(m.ctx, "Storing local key with comment",
		slog.String("key-string", keyString),
		slog.String("key-type", sshPubKey.Type()),
		slog.String("key-comment", key.Comment),
	)

	m.localKeys[keyString] = &key

	return nil
}

// Remove removes a key from the local agent.
func (m *MuxAgent) Remove(key ssh.PublicKey) error {
	m.logger.DebugContext(m.ctx, "Remove called with key", slog.String("key-type", key.Type()))

	m.keysMutex.Lock()
	defer m.keysMutex.Unlock()

	keyBlob := key.Marshal()
	keyString := string(keyBlob)

	delete(m.localKeys, keyString)

	return nil
}

// RemoveAll removes all keys from the local agent.
func (m *MuxAgent) RemoveAll() error {
	m.logger.DebugContext(m.ctx, "RemoveAll called")

	m.keysMutex.Lock()
	defer m.keysMutex.Unlock()

	m.localKeys = make(map[string]*agent.AddedKey)

	return nil
}

// Lock is not implemented (not needed for this use case).
func (m *MuxAgent) Lock(_ []byte) error {
	m.logger.DebugContext(m.ctx, "Lock called")

	return fmt.Errorf("locking %w", ErrUnimplemented)
}

// Unlock is not implemented (not needed for this use case).
func (m *MuxAgent) Unlock(_ []byte) error {
	m.logger.DebugContext(m.ctx, "Unlock called")

	return fmt.Errorf("unlocking %w", ErrUnimplemented)
}

// Signers returns signers for all local keys.
func (m *MuxAgent) Signers() ([]ssh.Signer, error) {
	m.logger.DebugContext(m.ctx, "Signers called")

	m.keysMutex.RLock()
	defer m.keysMutex.RUnlock()

	signers := make([]ssh.Signer, 0, len(m.localKeys))
	for _, addedKey := range m.localKeys {
		signer, err := ssh.NewSignerFromKey(addedKey.PrivateKey)
		if err != nil {
			continue
		}
		signers = append(signers, signer)
	}

	if err := m.runAgainstBackends(func(fb agent.ExtendedAgent) error {
		// Add signers from backend agent
		backendSigners, err := fb.Signers()
		if err != nil {
			return fmt.Errorf("failed to get signers from backend agent: %w", err)
		}
		signers = append(signers, backendSigners...)
		return nil
	}); err != nil {
		m.logger.DebugContext(m.ctx, "Failed to get signers from backend agents", slogtool.ErrorAttr(err))
	}

	return signers, nil
}

// Extension processes extension requests (not implemented).
func (m *MuxAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	m.logger.DebugContext(m.ctx, "Extension called with type", slog.String("extension-type", extensionType))

	{
		v, err := m.handleMuxExtension(extensionType, contents)
		if err == nil || !errors.Is(err, agent.ErrExtensionUnsupported) {
			return v, err
		}
	}

	if err := m.runAgainstBackends(func(fb agent.ExtendedAgent) error {
		resp, err := fb.Extension(extensionType, contents)
		if err != nil {
			return err
		}
		if resp != nil {
			m.logger.DebugContext(m.ctx, "Extension response obtained from backend agent",
				slog.String("extension-type", extensionType),
			)
			contents = resp
			return errExitBackendLoop
		}
		return agent.ErrExtensionUnsupported
	}); errors.Is(err, errExitBackendLoop) {
		return contents, nil
	}

	return nil, agent.ErrExtensionUnsupported
}

func (m *MuxAgent) handleMuxExtension(extensionType string, contents []byte) ([]byte, error) {
	m.logger.DebugContext(m.ctx, "handleMuxExtension called with type", slog.String("extension-type", extensionType))

	switch extensionType {
	case "ping":
		return HandleExtensionProto(contents, m.handlePing)
	case "config":
		return HandleExtensionProto(contents, m.handleConfig)
	case "shutdown":
		defer m.ctx.Cancel()
		return HandleExtensionProto(contents, m.handleShutdown)
	}

	// Handle custom extensions here
	return nil, agent.ErrExtensionUnsupported
}

func (m *MuxAgent) handlePing(msg *api.Ping) (*api.Pong, error) {
	m.logger.DebugContext(m.ctx, "handlePing called", slog.String("msg-id", msg.GetId()))

	pong := api.Pong_builder{
		Id:        proto.String(msg.GetId()),
		PingTs:    msg.GetTs(),
		Ts:        timestamppb.Now(),
		Pid:       proto.Int64(int64(syscall.Getpid())),
		StartTime: m.config.GetStartTime(),
		Version:   proto.String(cliversion.Get().VersionString()),
	}.Build()

	return pong, nil
}

func (m *MuxAgent) handleShutdown(msg *api.ShutdownRequest) (*api.CommandResponse, error) {
	m.logger.DebugContext(m.ctx, "handleShutdown called", slog.String("msg-id", msg.GetId()))

	resp := api.CommandResponse_builder{
		Id:      proto.String(msg.GetId()),
		Ts:      msg.GetTs(),
		Success: proto.Bool(true),
		Message: proto.String(
			fmt.Sprintf("shutdown received for pid %d", os.Getpid()),
		),
	}.Build()

	return resp, nil
}

func (m *MuxAgent) handleConfig(msg *api.ConfigRequest) (*api.Config, error) {
	m.logger.DebugContext(m.ctx, "handleConfig called", slog.String("msg-id", msg.GetId()))

	var cfg *api.Config
	{
		var ok bool
		cfg, ok = proto.Clone(m.config).(*api.Config)
		if !ok {
			return nil, errors.New("failed to clone config")
		}
	}

	cfg.SetId(uuid.NewString())
	cfg.SetTs(timestamppb.Now())

	return cfg, nil
}
