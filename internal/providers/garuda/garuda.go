package garuda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/providers"
)

const providerName = "Garuda Indonesia"

type Provider struct {
	fetcher providers.Fetcher
	logger  *slog.Logger
}

var _ providers.Provider = (*Provider)(nil)

func New(fetcher providers.Fetcher, logger *slog.Logger) *Provider {
	return &Provider{fetcher: fetcher, logger: logger}
}

func (p *Provider) Name() string {
	return providerName
}

func (p *Provider) Search(ctx context.Context) ([]models.Flight, error) {
	body, err := p.fetcher.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("garuda: fetch: %w", err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("garuda: decode response: %w", err)
	}

	if resp.Status != "success" {
		return nil, fmt.Errorf("garuda: status %q: %w", resp.Status, providers.ErrProviderUnavailable)
	}

	flights := make([]models.Flight, 0, len(resp.Flights))

	for _, raw := range resp.Flights {
		flg, err := normalize(raw)
		if err != nil {
			p.logger.WarnContext(ctx, "skipping invalid flight", "provider", providerName, "flight", raw.FlightID, "error", err)
			continue
		}

		flights = append(flights, flg)
	}
	return flights, nil
}

// convert garuda flight to response flight
func normalize(raw flight) (models.Flight, error) {
	if raw.FlightID == "" {
		return models.Flight{}, errors.New("missing flight id")
	}

	dep, arr, layovers, err := route(raw)
	if err != nil {
		return models.Flight{}, err
	}

	duration := int(arr.Time.Sub(dep.Time).Minutes())
	if duration <= 0 {
		return models.Flight{}, fmt.Errorf("arrival %s is not after departure %s", arr.Time, dep.Time)
	}

	cabin, err := providers.ParseCabinClass(raw.FareClass)
	if err != nil {
		return models.Flight{}, err
	}

	if raw.Price.Amount <= 0 {
		return models.Flight{}, fmt.Errorf("invalid price %d", raw.Price.Amount)
	}

	return models.Flight{
		ID:              raw.FlightID + "_" + providerName,
		Provider:        providerName,
		Airline:         models.Airline{Name: raw.Airline, Code: raw.AirlineCode},
		FlightNumber:    raw.FlightID,
		Departure:       dep,
		Arrival:         arr,
		DurationMinutes: duration,
		Layovers:        layovers,
		Price:           models.Price{Amount: raw.Price.Amount, Currency: raw.Price.Currency},
		AvailableSeats:  raw.AvailableSeats,
		CabinClass:      cabin,
		Aircraft:        raw.Aircraft,
		Amenities:       raw.Amenities,
		Baggage: models.Baggage{
			CarryOn: pieces(raw.Baggage.CarryOn),
			Checked: pieces(raw.Baggage.Checked),
		},
	}, nil
}

func route(raw flight) (models.FlightPoint, models.FlightPoint, []models.Layover, error) {
	var none models.FlightPoint

	if len(raw.Segments) == 0 {
		dep, err := toPoint(raw.Departure)
		if err != nil {
			return none, none, nil, err
		}
		arr, err := toPoint(raw.Arrival)
		if err != nil {
			return none, none, nil, err
		}
		return dep, arr, nil, nil
	}

	segs := raw.Segments
	dep, err := toPoint(segs[0].Departure)
	if err != nil {
		return none, none, nil, err
	}
	arr, err := toPoint(segs[len(segs)-1].Arrival)
	if err != nil {
		return none, none, nil, err
	}

	var layovers []models.Layover
	for i := 1; i < len(segs); i++ {
		prev, next := segs[i-1].Arrival, segs[i].Departure
		if prev.Airport != next.Airport {
			return none, none, nil, fmt.Errorf("segments not connected: arrive %s, depart %s", prev.Airport, next.Airport)
		}

		landed, err := parseTime(prev.Time)
		if err != nil {
			return none, none, nil, err
		}
		takeoff, err := parseTime(next.Time)
		if err != nil {
			return none, none, nil, err
		}

		gap := takeoff.Sub(landed)
		if gap < 0 {
			return none, none, nil, fmt.Errorf("segment departs %s before previous one lands", next.Airport)
		}

		layovers = append(layovers, models.Layover{
			Airport:         next.Airport,
			DurationMinutes: int(gap.Minutes()),
		})
	}
	return dep, arr, layovers, nil
}

func toPoint(e endpoint) (models.FlightPoint, error) {
	t, err := parseTime(e.Time)
	if err != nil {
		return models.FlightPoint{}, err
	}

	city := e.City
	if city == "" {
		city = providers.CityOf(e.Airport)
	}

	return models.FlightPoint{Airport: e.Airport, City: city, Time: t}, nil
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return t, nil
}

func pieces(n int) string {
	switch {
	case n <= 0:
		return "Not included"
	case n == 1:
		return "1 piece"
	default:
		return fmt.Sprintf("%d pieces", n)
	}
}
