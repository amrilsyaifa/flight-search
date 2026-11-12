package airasia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/providers"
)

const providerName = "AirAsia"

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
		return nil, fmt.Errorf("airasia: fetch: %w", err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("airasia: decode response: %w", err)
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("airasia: status %q: %w", resp.Status, providers.ErrProviderUnavailable)
	}

	flights := make([]models.Flight, 0, len(resp.Flights))

	for _, raw := range resp.Flights {
		f, err := normalize(raw)
		if err != nil {
			p.logger.WarnContext(ctx, "skipping invalid flight",
				"provider", providerName, "flight", raw.FlightCode, "error", err)
			continue
		}
		p.checkReportedDuration(ctx, raw, f)
		flights = append(flights, f)
	}
	return flights, nil
}

func (p *Provider) checkReportedDuration(ctx context.Context, raw flight, f models.Flight) {
	reported := int(math.Round(raw.DurationHours * 60))
	if diff := reported - f.DurationMinutes; diff > 1 || diff < -1 {
		p.logger.WarnContext(ctx, "provider duration mismatch, using timestamps",
			"provider", providerName, "flight", raw.FlightCode,
			"reported_minutes", reported, "computed_minutes", f.DurationMinutes)
	}
}

func normalize(raw flight) (models.Flight, error) {
	code, err := airlineCode(raw.FlightCode)
	if err != nil {
		return models.Flight{}, err
	}

	depTime, err := time.Parse(time.RFC3339, raw.DepartTime)
	if err != nil {
		return models.Flight{}, fmt.Errorf("invalid depart_time %q: %w", raw.DepartTime, err)
	}

	arrTime, err := time.Parse(time.RFC3339, raw.ArriveTime)
	if err != nil {
		return models.Flight{}, fmt.Errorf("invalid arrive_time %q: %w", raw.ArriveTime, err)
	}

	duration := int(arrTime.Sub(depTime).Minutes())
	if duration <= 0 {
		return models.Flight{}, fmt.Errorf("arrival %s is not after departure %s", arrTime, depTime)
	}

	layovers, err := toLayovers(raw, duration)
	if err != nil {
		return models.Flight{}, err
	}

	cabin, err := providers.ParseCabinClass(raw.CabinClass)
	if err != nil {
		return models.Flight{}, err
	}

	if raw.PriceIDR <= 0 {
		return models.Flight{}, fmt.Errorf("invalid price %d", raw.PriceIDR)
	}

	carryOn, checked := parseBaggage(raw.BaggageNote)

	return models.Flight{
		ID:              raw.FlightCode + "_" + providerName,
		Provider:        providerName,
		Airline:         models.Airline{Name: raw.Airline, Code: code},
		FlightNumber:    raw.FlightCode,
		Departure:       models.FlightPoint{Airport: raw.FromAirport, City: providers.CityOf(raw.FromAirport), Time: depTime},
		Arrival:         models.FlightPoint{Airport: raw.ToAirport, City: providers.CityOf(raw.ToAirport), Time: arrTime},
		DurationMinutes: duration,
		Layovers:        layovers,
		Price:           models.Price{Amount: raw.PriceIDR, Currency: "IDR"},
		AvailableSeats:  raw.Seats,
		CabinClass:      cabin,
		Aircraft:        "",
		Amenities:       nil,
		Baggage:         models.Baggage{CarryOn: carryOn, Checked: checked},
	}, nil

}

func airlineCode(flightCode string) (string, error) {
	if len(flightCode) < 3 {
		return "", fmt.Errorf("invalid flight_code %q", flightCode)
	}

	return strings.ToUpper(flightCode[:2]), nil
}

func toLayovers(raw flight, totalMinutes int) ([]models.Layover, error) {
	if raw.DirectFlight && len(raw.Stops) > 0 {
		return nil, errors.New("direct_flight=true but stops are present")
	}

	if !raw.DirectFlight && len(raw.Stops) == 0 {
		return nil, errors.New("direct_flight=false but no stops given")
	}

	layovers := make([]models.Layover, 0, len(raw.Stops))
	sum := 0
	for _, s := range raw.Stops {
		if s.WaitTimeMinutes < 0 {
			return nil, fmt.Errorf("negative wait time at %s", s.Airport)
		}
		sum += s.WaitTimeMinutes
		layovers = append(layovers, models.Layover{Airport: s.Airport, DurationMinutes: s.WaitTimeMinutes})
	}

	if sum >= totalMinutes {
		return nil, fmt.Errorf("layovers (%dm) do not fit in trip duration (%dm)", sum, totalMinutes)
	}
	return layovers, nil
}

func parseBaggage(note string) (carryOn, checked string) {
	before, after, found := strings.Cut(note, ",")
	carryOn = strings.TrimSpace(before)
	if !found {
		return carryOn, ""
	}

	checked = strings.TrimSpace(after)
	if rest, ok := cutPrefixFold(checked, "checked bags "); ok {
		checked = rest
	}
	return carryOn, capitalize(checked)
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):], true
	}
	return s, false
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
