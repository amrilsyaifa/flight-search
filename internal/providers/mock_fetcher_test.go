package providers

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"testing"
	"time"
)

func seededRNG() *rand.Rand {
	return rand.New(rand.NewPCG(42, 42))
}

func TestMockFetcher_ReturnsData(t *testing.T) {
	data := []byte(`{"status":"ok"}`)
	f := NewMockFetcher(data, MockConfig{}, seededRNG())

	got, err := f.Fetch(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestMockFetcher_AlwaysFails(t *testing.T) {
	f := NewMockFetcher([]byte(`{}`), MockConfig{FailureRate: 1.0}, seededRNG())

	got, err := f.Fetch(context.Background())

	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("err = %v, want ErrProviderUnavailable", err)
	}
	if got != nil {
		t.Errorf("data should be nil on failure, got %q", got)
	}
}

func TestMockFetcher_StopsWhenContextIsCancelled(t *testing.T) {
	f := NewMockFetcher([]byte(`{}`), MockConfig{MinDelay: 5 * time.Second, MaxDelay: 5 * time.Second}, seededRNG())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := f.Fetch(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}

	if elapsed > time.Second {
		t.Errorf("Fetch took %v; it should return right after the 50ms timeout", elapsed)
	}
}

func TestMockFetcher_WaitsAtLeastMinDelay(t *testing.T) {
	const delay = 30 * time.Millisecond
	f := NewMockFetcher([]byte(`{}`), MockConfig{MinDelay: delay, MaxDelay: delay}, seededRNG())

	start := time.Now()
	if _, err := f.Fetch(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if elapsed := time.Since(start); elapsed < delay {
		t.Errorf("returned after %v, want at least %v", elapsed, delay)
	}
}

func TestMockFetcher_ReturnsACopy(t *testing.T) {
	f := NewMockFetcher([]byte(`{"a":1}`), MockConfig{}, seededRNG())

	first, _ := f.Fetch(context.Background())
	first[0] = 'X'

	second, _ := f.Fetch(context.Background())
	if string(second) != `{"a":1}` {
		t.Errorf("shared data was modified by a caller: got %q", second)
	}
}

func TestMockFetcher_RollStaysWithinBounds(t *testing.T) {
	tests := []struct {
		name string
		cfg  MockConfig
	}{
		{"garuda", MockConfig{MinDelay: 50 * time.Millisecond, MaxDelay: 100 * time.Millisecond}},
		{"batik", MockConfig{MinDelay: 200 * time.Millisecond, MaxDelay: 400 * time.Millisecond}},
		{"fixed delay", MockConfig{MinDelay: 10 * time.Millisecond, MaxDelay: 10 * time.Millisecond}},
		{"zero", MockConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewMockFetcher(nil, tt.cfg, seededRNG())
			for i := 0; i < 1000; i++ {
				delay, _ := f.roll()
				if delay < tt.cfg.MinDelay || delay > tt.cfg.MaxDelay {
					t.Fatalf("roll %d: delay %v outside [%v, %v]", i, delay, tt.cfg.MinDelay, tt.cfg.MaxDelay)
				}
			}
		})
	}
}

func TestMockFetcher_FailureRateIsRoughlyRespected(t *testing.T) {
	f := NewMockFetcher(nil, MockConfig{FailureRate: 0.1}, seededRNG())

	const n = 10_000
	failures := 0
	for i := 0; i < n; i++ {
		if _, fail := f.roll(); fail {
			failures++
		}
	}

	rate := float64(failures) / n
	if rate < 0.08 || rate > 0.12 {
		t.Errorf("failure rate = %.3f, want about 0.10", rate)
	}
}

func TestMockFetcher_ConcurrentUse(t *testing.T) {
	f := NewMockFetcher([]byte(`{}`), MockConfig{FailureRate: 0.5}, seededRNG())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.Fetch(context.Background())
			if err != nil && !errors.Is(err, ErrProviderUnavailable) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
}
