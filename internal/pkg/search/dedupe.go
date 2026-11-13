package search

import (
	"strconv"
	"strings"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

func Dedupe(flights []models.Flight) []models.Flight {
	out := make([]models.Flight, 0, len(flights))
	indexByKey := make(map[string]int, len(flights))

	for _, f := range flights {
		key := flightKey(f)
		i, seen := indexByKey[key]
		if !seen {
			indexByKey[key] = len(out)
			out = append(out, f)
			continue
		}
		if f.Price.Amount < out[i].Price.Amount {
			out[i] = f
		}
	}
	return out
}

// flightKey identifies a physical flight on a specific day.
//   - Flight number alone is not enough: GA400 flies every day.
//   - The departure is compared as a Unix instant, so "06:00+07:00" and
//     "23:00Z the day before" from two providers still match.
//   - Cabin class is part of the key: economy and business on the same plane
//     are different products and must not replace each other.
func flightKey(f models.Flight) string {
	number := strings.ToUpper(strings.ReplaceAll(f.FlightNumber, " ", "")) // "GA 400" == "GA400"
	return number + "|" + strconv.FormatInt(f.Departure.Time.Unix(), 10) + "|" + string(f.CabinClass)
}
