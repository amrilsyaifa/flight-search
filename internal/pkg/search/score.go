package search

import "github.com/amrilsyaifa/flight-search/internal/models"

const (
	priceWeight    = 0.5
	durationWeight = 0.3
	stopsWeight    = 0.2
)

func BestValueScores(flights []models.Flight) map[string]float64 {
	scores := make(map[string]float64, len(flights))
	if len(flights) == 0 {
		return scores
	}

	minPrice, minDur := flights[0].Price.Amount, flights[0].DurationMinutes
	for _, f := range flights {
		minPrice = min(minPrice, f.Price.Amount)
		minDur = min(minDur, f.DurationMinutes)
	}

	// Pass 2: score each flight.
	for _, f := range flights {
		price := relativeExtra(float64(f.Price.Amount), float64(minPrice))
		duration := relativeExtra(float64(f.DurationMinutes), float64(minDur))
		stops := float64(f.Stops())

		scores[f.ID] = priceWeight*price + durationWeight*duration + stopsWeight*stops
	}
	return scores
}

// relativeExtra returns how much bigger v is than best, as a fraction:
// relativeExtra(120, 100) = 0.2. Validation guarantees prices and durations
// are positive; the guard just avoids dividing by zero.
func relativeExtra(v, best float64) float64 {
	if best <= 0 {
		return 0
	}
	return v/best - 1
}
