package muxagent

import "golang.org/x/crypto/ssh/agent"

func (m *MuxAgent) GetLocalKeys() map[string]*agent.AddedKey {
	m.keysMutex.RLock()
	defer m.keysMutex.RUnlock()

	if m.localKeys == nil {
		return nil
	}

	// Create a copy to prevent external modification
	keysCopy := make(map[string]*agent.AddedKey)
	for k, v := range m.localKeys {
		keysCopy[k] = v
	}
	return keysCopy
}
