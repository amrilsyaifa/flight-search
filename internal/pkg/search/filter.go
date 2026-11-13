package search

import "github.com/amrilsyaifa/flight-search/internal/models"

func Filter(flights []models.Flight, c models.SearchCriteria, f models.SearchFilters) []models.Flight {
	airlines := toSet(f.Airlines)

	out := make([]models.Flight, 0, len(flights))
	for _, fl := range flights {
		if matchesCriteria(fl, c) && matchesFilters(fl, f, airlines) {
			out = append(out, fl)
		}
	}
	return out
}

func matchesCriteria(fl models.Flight, c models.SearchCriteria) bool {
	return fl.Departure.Airport == c.Origin &&
		fl.Arrival.Airport == c.Destination &&
		models.DateOf(fl.Departure.Time) == c.DepartureDate &&
		fl.CabinClass == c.CabinClass &&
		fl.AvailableSeats >= c.Passengers
}

func matchesFilters(fl models.Flight, f models.SearchFilters, airlines map[string]struct{}) bool {
	if f.MinPrice != nil && fl.Price.Amount < *f.MinPrice {
		return false
	}
	if f.MaxPrice != nil && fl.Price.Amount > *f.MaxPrice {
		return false
	}
	if f.MaxStops != nil && fl.Stops() > *f.MaxStops {
		return false
	}
	if f.MaxDurationMinutes != nil && fl.DurationMinutes > *f.MaxDurationMinutes {
		return false
	}
	if len(airlines) > 0 {
		if _, ok := airlines[fl.Airline.Code]; !ok {
			return false
		}
	}
	if !inTimeRange(models.TimeOfDayOf(fl.Departure.Time), f.DepartureFrom, f.DepartureTo) {
		return false
	}
	if !inTimeRange(models.TimeOfDayOf(fl.Arrival.Time), f.ArrivalFrom, f.ArrivalTo) {
		return false
	}
	return true
}

func inTimeRange(t models.TimeOfDay, from, to *models.TimeOfDay) bool {
	switch {
	case from == nil && to == nil:
		return true
	case from == nil:
		return t <= *to
	case to == nil:
		return t >= *from
	case *from <= *to:
		return t >= *from && t <= *to
	default:
		return t >= *from || t <= *to
	}
}

func toSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		set[v] = struct{}{}
	}
	return set
}
