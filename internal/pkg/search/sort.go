package search

import (
	"cmp"
	"slices"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

func Sort(flights []models.Flight, opt models.SortOption) {
	primary := comparator(opt.By)

	slices.SortStableFunc(flights, func(a, b models.Flight) int {
		c := primary(a, b)
		if opt.Order == models.SortDesc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return tieBreak(a, b)
	})
}

func comparator(by models.SortField) func(a, b models.Flight) int {
	switch by {
	case models.SortByDuration:
		return func(a, b models.Flight) int { return cmp.Compare(a.DurationMinutes, b.DurationMinutes) }
	case models.SortByDepartureTime:
		return func(a, b models.Flight) int { return a.Departure.Time.Compare(b.Departure.Time) }
	case models.SortByArrivalTime:
		return func(a, b models.Flight) int { return a.Arrival.Time.Compare(b.Arrival.Time) }
	default: // models.SortByPrice
		return func(a, b models.Flight) int { return cmp.Compare(a.Price.Amount, b.Price.Amount) }
	}
}

func tieBreak(a, b models.Flight) int {
	if c := cmp.Compare(a.Price.Amount, b.Price.Amount); c != 0 {
		return c
	}
	if c := a.Departure.Time.Compare(b.Departure.Time); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}
