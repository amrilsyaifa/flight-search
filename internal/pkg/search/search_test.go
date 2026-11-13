package search

import (
	"testing"
	"time"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

var (
	wib  = time.FixedZone("WIB", 7*3600)
	wita = time.FixedZone("WITA", 8*3600)
)

// flight builds a valid CGK->DPS economy flight; each test tweaks what it needs.
func flight(id string, price int64, depHour, depMin, durationMin, stops int, airline string) models.Flight {
	dep := time.Date(2025, 12, 15, depHour, depMin, 0, 0, wib)
	f := models.Flight{
		ID:              id,
		Airline:         models.Airline{Code: airline},
		Departure:       models.FlightPoint{Airport: "CGK", Time: dep},
		Arrival:         models.FlightPoint{Airport: "DPS", Time: dep.Add(time.Duration(durationMin) * time.Minute).In(wita)},
		DurationMinutes: durationMin,
		Price:           models.Price{Amount: price, Currency: "IDR"},
		AvailableSeats:  10,
		CabinClass:      models.CabinEconomy,
	}
	for i := 0; i < stops; i++ {
		f.Layovers = append(f.Layovers, models.Layover{Airport: "SUB", DurationMinutes: 60})
	}
	return f
}

var criteria = models.SearchCriteria{
	Origin: "CGK", Destination: "DPS",
	DepartureDate: models.Date{Year: 2025, Month: time.December, Day: 15},
	Passengers:    1, CabinClass: models.CabinEconomy,
}

func ptr[T any](v T) *T { return &v }

func ids(flights []models.Flight) []string {
	out := make([]string, len(flights))
	for i, f := range flights {
		out[i] = f.ID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilter(t *testing.T) {
	all := []models.Flight{
		flight("early", 650000, 4, 45, 100, 0, "QZ"), // 04:45 WIB = 21:45 UTC on the 14th
		flight("cheap", 485000, 15, 15, 260, 1, "QZ"),
		flight("garuda", 1250000, 6, 0, 110, 0, "GA"),
		flight("night", 595000, 23, 30, 100, 0, "JT"),
	}

	tests := []struct {
		name    string
		filters models.SearchFilters
		want    []string
	}{
		{"no filters keeps all (local date, not UTC)", models.SearchFilters{}, []string{"early", "cheap", "garuda", "night"}},
		{"max price", models.SearchFilters{MaxPrice: ptr(int64(600000))}, []string{"cheap", "night"}},
		{"direct only (maxStops=0)", models.SearchFilters{MaxStops: ptr(0)}, []string{"early", "garuda", "night"}},
		{"airlines", models.SearchFilters{Airlines: []string{"GA", "JT"}}, []string{"garuda", "night"}},
		{"max duration", models.SearchFilters{MaxDurationMinutes: ptr(105)}, []string{"early", "night"}},
		{"morning departures", models.SearchFilters{DepartureFrom: ptr(models.TimeOfDay(5 * 60)), DepartureTo: ptr(models.TimeOfDay(12 * 60))}, []string{"garuda"}},
		{"overnight range wraps midnight", models.SearchFilters{DepartureFrom: ptr(models.TimeOfDay(22 * 60)), DepartureTo: ptr(models.TimeOfDay(5 * 60))}, []string{"early", "night"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ids(Filter(all, criteria, tt.filters))
			if !equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilter_CriteriaMismatchesAreRemoved(t *testing.T) {
	wrongRoute := flight("sub", 500000, 8, 0, 90, 0, "GA")
	wrongRoute.Arrival.Airport = "SUB"
	full := flight("full", 500000, 9, 0, 90, 0, "GA")
	full.AvailableSeats = 1
	nextDay := flight("next-day", 500000, 9, 0, 90, 0, "GA")
	nextDay.Departure.Time = nextDay.Departure.Time.AddDate(0, 0, 1)

	c := criteria
	c.Passengers = 2

	if got := Filter([]models.Flight{wrongRoute, full, nextDay}, c, models.SearchFilters{}); len(got) != 0 {
		t.Errorf("want no flights, got %v", ids(got))
	}
}

func TestFilter_DoesNotModifyInput(t *testing.T) {
	in := []models.Flight{flight("a", 900000, 8, 0, 100, 0, "GA"), flight("b", 400000, 9, 0, 100, 0, "GA")}

	_ = Filter(in, criteria, models.SearchFilters{MaxPrice: ptr(int64(500000))})

	if !equal(ids(in), []string{"a", "b"}) {
		t.Errorf("input was modified: %v", ids(in))
	}
}

func TestSort(t *testing.T) {
	base := func() []models.Flight {
		return []models.Flight{
			flight("A", 900000, 10, 0, 120, 0, "GA"),
			flight("B", 500000, 6, 0, 300, 1, "JT"),
			flight("C", 700000, 18, 0, 90, 0, "QZ"),
			flight("D", 500000, 8, 0, 100, 0, "ID"), // same price as B, departs later
		}
	}

	tests := []struct {
		opt  models.SortOption
		want []string
	}{
		{models.SortOption{By: models.SortByPrice, Order: models.SortAsc}, []string{"B", "D", "C", "A"}}, // B before D: tie broken by departure
		{models.SortOption{By: models.SortByPrice, Order: models.SortDesc}, []string{"A", "C", "B", "D"}},
		{models.SortOption{By: models.SortByDuration, Order: models.SortAsc}, []string{"C", "D", "A", "B"}},
		{models.SortOption{By: models.SortByDepartureTime, Order: models.SortAsc}, []string{"B", "D", "A", "C"}},
		{models.SortOption{By: models.SortByArrivalTime, Order: models.SortDesc}, []string{"C", "A", "B", "D"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.opt.By)+"_"+string(tt.opt.Order), func(t *testing.T) {
			flights := base()
			Sort(flights, tt.opt)
			if got := ids(flights); !equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBestValueScores(t *testing.T) {
	cheapButSlow := flight("QZ7250", 485000, 15, 15, 260, 1, "QZ")
	directAndFast := flight("QZ532", 595000, 19, 30, 100, 0, "QZ")
	expensive := flight("GA315", 1850000, 14, 0, 225, 1, "GA")

	scores := BestValueScores([]models.Flight{cheapButSlow, directAndFast, expensive})

	if scores["QZ532"] >= scores["QZ7250"] {
		t.Errorf("direct+fast should beat cheapest-with-stop: QZ532=%.3f, QZ7250=%.3f",
			scores["QZ532"], scores["QZ7250"])
	}
	if scores["GA315"] <= scores["QZ7250"] {
		t.Errorf("expensive with stop should be worst: GA315=%.3f", scores["GA315"])
	}
}

func TestBestValueScores_SingleFlightDoesNotDivideByZero(t *testing.T) {
	scores := BestValueScores([]models.Flight{flight("only", 500000, 8, 0, 100, 0, "GA")})

	if s := scores["only"]; s != 0 {
		t.Errorf("score = %v, want 0 (nothing to compare against)", s)
	}
}

func TestSort_BestValue(t *testing.T) {
	flights := []models.Flight{
		flight("QZ7250", 485000, 15, 15, 260, 1, "QZ"),
		flight("GA315", 1850000, 14, 0, 225, 1, "GA"),
		flight("QZ532", 595000, 19, 30, 100, 0, "QZ"),
	}

	Sort(flights, models.SortOption{By: models.SortByBestValue, Order: models.SortAsc})

	if got := ids(flights); !equal(got, []string{"QZ532", "QZ7250", "GA315"}) {
		t.Errorf("got %v", got)
	}
}
