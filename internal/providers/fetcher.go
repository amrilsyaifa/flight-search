package providers

import (
	"context"
	"errors"
)

type Fetcher interface {
	Fetch(ctx context.Context) ([]byte, error)
}

var ErrProviderUnavailable = errors.New("provider unavailable")
