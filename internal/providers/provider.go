package providers

import (
	"context"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

type Provider interface {
	Name() string
	Search(ctx context.Context) ([]models.Flight, error)
}
