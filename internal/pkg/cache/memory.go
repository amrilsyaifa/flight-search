package cache

import (
	"context"
	"sync"
	"time"
)

type Memory struct {
	mu      sync.RWMutex
	entries map[string]entry
	now     func() time.Time
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

func NewMemory() *Memory {
	return &Memory{entries: make(map[string]entry), now: time.Now}
}

func (m *Memory) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	e, ok := m.entries[key]
	m.mu.RUnlock()

	if !ok || m.now().After(e.expiresAt) {
		return nil, false, nil
	}
	return e.value, true, nil
}

func (m *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.deleteExpired()
	m.entries[key] = entry{value: value, expiresAt: m.now().Add(ttl)}
	return nil
}

// deleteExpired must be called with m.mu held.
func (m *Memory) deleteExpired() {
	now := m.now()
	for k, e := range m.entries {
		if now.After(e.expiresAt) {
			delete(m.entries, k)
		}
	}
}
