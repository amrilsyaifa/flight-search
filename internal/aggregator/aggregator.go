package aggregator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/providers"
)

type Result struct {
	Flights   []models.Flight
	Queried   int
	Succeeded int
	Failed    int
}

type Aggregator struct {
	providers []providers.Provider
	timeout   time.Duration
	logger    *slog.Logger
}

type outcome struct {
	name    string
	flights []models.Flight
	err     error
}

func New(ps []providers.Provider, timeout time.Duration, logger *slog.Logger) *Aggregator {
	return &Aggregator{providers: ps, timeout: timeout, logger: logger}
}

func (a *Aggregator) Search(ctx context.Context) Result {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	outcomes := make([]outcome, len(a.providers))

	var wg sync.WaitGroup
	for i, p := range a.providers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcomes[i] = a.callProvider(ctx, p)
		}()
	}
	wg.Wait()

	result := Result{Queried: len(a.providers), Flights: []models.Flight{}}

	for _, o := range outcomes {
		if o.err != nil {
			a.logger.WarnContext(ctx, "provider failed", "provider", o.name, "error", o.err)
			result.Failed++
			continue
		}
		result.Succeeded++
		result.Flights = append(result.Flights, o.flights...)
	}
	return result
}

func (a *Aggregator) callProvider(ctx context.Context, p providers.Provider) (o outcome) {
	o.name = p.Name()
	defer func() {
		if r := recover(); r != nil {
			o.err = fmt.Errorf("provider panicked: %v", r)
		}
	}()
	o.flights, o.err = p.Search(ctx)
	return o
}
