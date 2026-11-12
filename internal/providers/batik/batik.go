package batik

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/providers"
)

const providerName = "Batik Air"

const timeLayout = "2006-01-02T15:04:05-0700"

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
		return nil, fmt.Errorf("batik: fetch: %w", err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("batik: decode response: %w", err)
	}

	if resp.Code != http.StatusOK {
		return nil, fmt.Errorf("batik: code %d %q: %w", resp.Code, resp.Message, providers.ErrProviderUnavailable)
	}

	flights := make([]models.Flight, 0, len(resp.Results))

	for _, raw := range resp.Results {
		f, err := normalize(raw)
		if err != nil {
			p.logger.WarnContext(ctx, "skipping invalid flight",
				"provider", providerName, "flight", raw.FlightNumber, "error", err)
			continue
		}
		p.checkReportedDuration(ctx, raw, f)
		flights = append(flights, f)
	}

	return flights, nil
}

func (p *Provider) checkReportedDuration(ctx context.Context, raw flight, f models.Flight) {
	reported, err := parseDuration(raw.TravelTime)
	if err != nil {
		return
	}
	if int(reported.Minutes()) != f.DurationMinutes {
		p.logger.WarnContext(ctx, "provider duration mismatch, using timestamps",
			"provider", providerName, "flight", raw.FlightNumber,
			"reported_minutes", int(reported.Minutes()), "computed_minutes", f.DurationMinutes)
	}
}

func normalize(raw flight) (models.Flight, error) {
	if raw.FlightNumber == "" {
		return models.Flight{}, errors.New("missing flightNumber")
	}

	depTime, err := time.Parse(timeLayout, raw.DepartureDateTime)
	if err != nil {
		return models.Flight{}, fmt.Errorf("invalid departure time %q: %w", raw.DepartureDateTime, err)
	}

	arrTime, err := time.Parse(timeLayout, raw.ArrivalDateTime)
	if err != nil {
		return models.Flight{}, fmt.Errorf("invalid arrival time %q: %w", raw.ArrivalDateTime, err)
	}

	duration := int(arrTime.Sub(depTime).Minutes())
	if duration <= 0 {
		return models.Flight{}, fmt.Errorf("arrival %s is not after departure %s", arrTime, depTime)
	}

	layovers, err := toLayovers(raw, duration)
	if err != nil {
		return models.Flight{}, err
	}

	cabin, err := cabinFromFareClass(raw.Fare.Class)
	if err != nil {
		return models.Flight{}, err
	}

	if raw.Fare.TotalPrice <= 0 {
		return models.Flight{}, fmt.Errorf("invalid price %d", raw.Fare.TotalPrice)
	}

	carryOn, checked := parseBaggage(raw.BaggageInfo)

	return models.Flight{
		ID:              raw.FlightNumber + "_" + providerName,
		Provider:        providerName,
		Airline:         models.Airline{Name: raw.AirlineName, Code: raw.AirlineIATA},
		FlightNumber:    raw.FlightNumber,
		Departure:       models.FlightPoint{Airport: raw.Origin, City: providers.CityOf(raw.Origin), Time: depTime},
		Arrival:         models.FlightPoint{Airport: raw.Destination, City: providers.CityOf(raw.Destination), Time: arrTime},
		DurationMinutes: duration,
		Layovers:        layovers,
		Price:           models.Price{Amount: raw.Fare.TotalPrice, Currency: raw.Fare.CurrencyCode},
		AvailableSeats:  raw.SeatsAvailable,
		CabinClass:      cabin,
		Aircraft:        raw.AircraftModel,
		Amenities:       amenities(raw.OnboardServices),
		Baggage:         models.Baggage{CarryOn: carryOn, Checked: checked},
	}, nil

}

func toLayovers(raw flight, totalMinutes int) ([]models.Layover, error) {
	if raw.NumberOfStops != len(raw.Connections) {
		return nil, fmt.Errorf("numberOfStops=%d but %d connections", raw.NumberOfStops, len(raw.Connections))
	}

	layovers := make([]models.Layover, 0, len(raw.Connections))
	sum := 0

	for _, c := range raw.Connections {
		d, err := parseDuration(c.StopDuration)
		if err != nil {
			return nil, fmt.Errorf("connection at %s: %w", c.StopAirport, err)
		}
		minutes := int(d.Minutes())
		sum += minutes
		layovers = append(layovers, models.Layover{Airport: c.StopAirport, DurationMinutes: minutes})
	}

	if sum >= totalMinutes {
		return nil, fmt.Errorf("layovers (%dm) do not fit in trip duration (%dm)", sum, totalMinutes)
	}

	return layovers, nil
}

func parseDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}

	if d < 0 {
		return 0, fmt.Errorf("negative duration %q", s)
	}

	return d, nil
}

func cabinFromFareClass(code string) (models.CabinClass, error) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "Y":
		return models.CabinEconomy, nil
	case "W":
		return models.CabinPremiumEconomy, nil
	case "C", "J":
		return models.CabinBusiness, nil
	case "F":
		return models.CabinFirst, nil
	}

	return "", fmt.Errorf("unknown fare class %q", code)

}

func parseBaggage(info string) (carryOn, checked string) {
	for _, part := range strings.Split(info, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasSuffix(part, " cabin"):
			carryOn = strings.TrimSuffix(part, " cabin")
		case strings.HasSuffix(part, " checked"):
			checked = strings.TrimSuffix(part, " checked")
		}
	}
	return carryOn, checked
}

func amenities(services []string) []string {
	out := make([]string, 0, len(services))
	for _, s := range services {
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}
