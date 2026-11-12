package garuda

import (
	"testing"
	"time"
)

// GA315 from the assignment data: the top level claims it lands in SUB with
// 0 stops, but the segments show CGK -> SUB -> DPS.
func TestNormalize_MultiSegmentFlight(t *testing.T) {
	raw := flight{
		FlightID:    "GA315",
		Airline:     "Garuda Indonesia",
		AirlineCode: "GA",
		Departure:   endpoint{Airport: "CGK", City: "Jakarta", Time: "2025-12-15T14:00:00+07:00"},
		Arrival:     endpoint{Airport: "SUB", City: "Surabaya", Time: "2025-12-15T15:30:00+07:00"},
		Stops:       0,
		Price:       price{Amount: 1850000, Currency: "IDR"},
		FareClass:   "economy",
		Segments: []segment{
			{
				FlightNumber: "GA315",
				Departure:    endpoint{Airport: "CGK", Time: "2025-12-15T14:00:00+07:00"},
				Arrival:      endpoint{Airport: "SUB", Time: "2025-12-15T15:30:00+07:00"},
			},
			{
				FlightNumber: "GA332",
				Departure:    endpoint{Airport: "SUB", Time: "2025-12-15T17:15:00+07:00"},
				Arrival:      endpoint{Airport: "DPS", Time: "2025-12-15T18:45:00+08:00"},
			},
		},
	}

	got, err := normalize(raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Arrival.Airport != "DPS" {
		t.Errorf("arrival airport = %q, want DPS (from the last segment)", got.Arrival.Airport)
	}
	if got.Arrival.City != "Denpasar" {
		t.Errorf("arrival city = %q, want Denpasar (looked up, segments have no city)", got.Arrival.City)
	}
	wantArrival := time.Date(2025, 12, 15, 18, 45, 0, 0, time.FixedZone("", 8*3600))
	if !got.Arrival.Time.Equal(wantArrival) {
		t.Errorf("arrival time = %v, want %v", got.Arrival.Time, wantArrival)
	}
	if got.DurationMinutes != 225 {
		t.Errorf("duration = %d, want 225 (14:00 WIB to 18:45 WITA)", got.DurationMinutes)
	}
	if got.Stops() != 1 {
		t.Fatalf("stops = %d, want 1", got.Stops())
	}
	if l := got.Layovers[0]; l.Airport != "SUB" || l.DurationMinutes != 105 {
		t.Errorf("layover = %+v, want SUB 105m", l)
	}
}

// TestNormalize_DirectFlight
// TestNormalize_RejectsArrivalBeforeDeparture
