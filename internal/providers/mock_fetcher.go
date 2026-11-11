package providers

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

type MockConfig struct {
	MinDelay    time.Duration
	MaxDelay    time.Duration
	FailureRate float64 // 0.0 = never fails, 1.0 = always fails
}

type MockFetcher struct {
	data []byte
	cfg  MockConfig
	mu   sync.Mutex
	rng  *rand.Rand
}

func NewMockFetcher(data []byte, cfg MockConfig, rng *rand.Rand) *MockFetcher {
	if rng == nil {
		rng = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0))
	}
	return &MockFetcher{data: data, cfg: cfg, rng: rng}
}

func (m *MockFetcher) Fetch(ctx context.Context) ([]byte, error) {
	delay, fail := m.roll()
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}

	if fail {
		return nil, ErrProviderUnavailable
	}

	out := make([]byte, len(m.data))
	copy(out, m.data)
	return out, nil
}

func (m *MockFetcher) roll() (time.Duration, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delay := m.cfg.MinDelay
	if spread := m.cfg.MaxDelay - m.cfg.MinDelay; spread > 0 {
		delay += time.Duration(m.rng.Int64N(int64(spread) + 1))
	}

	fail := m.rng.Float64() < m.cfg.FailureRate
	return delay, fail
}
