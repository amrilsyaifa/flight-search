package ratelimit

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Memory struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
}

func NewMemory() *Memory {
	return &Memory{buckets: make(map[string]*rate.Limiter)}
}

func (m *Memory) Allow(_ context.Context, key string, perSecond int) (bool, time.Duration, error) {
	r := m.bucket(key, perSecond).Reserve()
	if d := r.Delay(); d > 0 {
		r.Cancel()
		return false, d, nil
	}
	return true, 0, nil
}

func (m *Memory) bucket(key string, perSecond int) *rate.Limiter {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.buckets[key]
	if !ok {
		b = rate.NewLimiter(rate.Limit(perSecond), perSecond)
		m.buckets[key] = b
	}
	return b
}
