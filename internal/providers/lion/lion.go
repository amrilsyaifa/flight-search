package lion

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

const providerName = "Lion Air"

const timeLayout = "2006-01-02T15:04:05"

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
		return nil, fmt.Errorf("lion: fetch: %w", err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("lion: decode response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("lion: success=false: %w", providers.ErrProviderUnavailable)
	}

	flights := make([]models.Flight, 0, len(resp.Data.AvailableFlights))
	for _, raw := range resp.Data.AvailableFlights {
		f, err := normalize(raw)
		if err != nil {
			p.logger.WarnContext(ctx, "skipping invalid flight", "provider", providerName, "flight", raw.ID, "error", err)
			continue
		}
		flights = append(flights, f)
	}
	return flights, nil
}

func normalize(raw flight) (models.Flight, error) {
	if raw.ID == "" {
		return models.Flight{}, errors.New("missing id")
	}

	depTime, err := parseLocal(raw.Schedule.Departure, raw.Schedule.DepartureTimezone)
	if err != nil {
		return models.Flight{}, fmt.Errorf("departure: %w", err)
	}

	arrTime, err := parseLocal(raw.Schedule.Arrival, raw.Schedule.ArrivalTimezone)
	if err != nil {
		return models.Flight{}, fmt.Errorf("arrival: %w", err)
	}

	duration := int(arrTime.Sub(depTime).Minutes())
	if duration <= 0 {
		return models.Flight{}, fmt.Errorf("arrival %s is not after departure %s", arrTime, depTime)
	}

	layovers, err := toLayovers(raw, duration)
	if err != nil {
		return models.Flight{}, err
	}

	cabin, err := providers.ParseCabinClass(raw.Pricing.FareType)
	if err != nil {
		return models.Flight{}, err
	}

	if raw.Pricing.Total <= 0 {
		return models.Flight{}, fmt.Errorf("invalid price %d", raw.Pricing.Total)
	}

	return models.Flight{
		ID:           raw.ID + "_" + providerName,
		Provider:     providerName,
		Airline:      models.Airline{Name: raw.Carrier.Name, Code: raw.Carrier.IATA},
		FlightNumber: raw.ID,
		Departure: models.FlightPoint{
			Airport: raw.Route.From.Code,
			City:    cityOr(raw.Route.From),
			Time:    depTime,
		},
		Arrival: models.FlightPoint{
			Airport: raw.Route.To.Code,
			City:    cityOr(raw.Route.To),
			Time:    arrTime,
		},
		DurationMinutes: duration,
		Layovers:        layovers,
		Price:           models.Price{Amount: raw.Pricing.Total, Currency: raw.Pricing.Currency},
		AvailableSeats:  raw.SeatsLeft,
		CabinClass:      cabin,
		Aircraft:        raw.PlaneType,
		Amenities:       amenities(raw.Services),
		Baggage: models.Baggage{
			CarryOn: raw.Services.BaggageAllowance.Cabin,
			Checked: raw.Services.BaggageAllowance.Hold,
		},
	}, nil
}

// parseLocal reads a time without offset as local time in the named zone.
// "2025-12-15T05:30:00" + "Asia/Jakarta" -> 2025-12-15T05:30:00+07:00
func parseLocal(value, zone string) (time.Time, error) {
	loc, err := providers.LoadLocation(zone)
	if err != nil {
		return time.Time{}, err
	}

	t, err := time.ParseInLocation(timeLayout, value, loc)

	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q: %w", value, err)
	}

	return t, nil
}

func toLayovers(raw flight, totalMinutes int) ([]models.Layover, error) {
	if raw.IsDirect && len(raw.Layovers) > 0 {
		return nil, errors.New("is_direct=true but layovers are present")
	}

	if !raw.IsDirect && len(raw.Layovers) == 0 {
		return nil, errors.New("is_direct=false but no layovers given")
	}

	if raw.StopCount != 0 && raw.StopCount != len(raw.Layovers) {
		return nil, fmt.Errorf("stop_count=%d but %d layovers", raw.StopCount, len(raw.Layovers))
	}

	layovers := make([]models.Layover, 0, len(raw.Layovers))
	sum := 0

	for _, l := range raw.Layovers {
		if l.DurationMinutes < 0 {
			return nil, fmt.Errorf("negative layover at %s", l.Airport)
		}
		sum += l.DurationMinutes
		layovers = append(layovers, models.Layover{Airport: l.Airport, DurationMinutes: l.DurationMinutes})
	}

	if sum >= totalMinutes {
		return nil, fmt.Errorf("layovers (%dm) do not fit in trip duration (%dm)", sum, totalMinutes)
	}

	return layovers, nil
}

func cityOr(a airport) string {
	if a.City != "" {
		return a.City
	}

	return providers.CityOf(a.Code)
}

func amenities(s services) []string {
	var out []string
	if s.WifiAvailable {
		out = append(out, "wifi")
	}
	if s.MealsIncluded {
		out = append(out, "meal")
	}
	return out
}
