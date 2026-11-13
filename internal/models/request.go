package models

import (
	"fmt"
	"time"
)

type SearchRequest struct {
	Criteria SearchCriteria
	Filters  SearchFilters
	Sort     SortOption
}

type SearchFilters struct {
	MinPrice           *int64
	MaxPrice           *int64
	MaxStops           *int
	DepartureFrom      *TimeOfDay
	DepartureTo        *TimeOfDay
	ArrivalFrom        *TimeOfDay
	ArrivalTo          *TimeOfDay
	Airlines           []string
	MaxDurationMinutes *int
}

type SortField string

const (
	SortByPrice         SortField = "price"
	SortByDuration      SortField = "duration"
	SortByDepartureTime SortField = "departure_time"
	SortByArrivalTime   SortField = "arrival_time"
)

type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

type SortOption struct {
	By    SortField
	Order SortOrder
}

var DefaultSort = SortOption{By: SortByPrice, Order: SortAsc}

type TimeOfDay int

// ParseTimeOfDay parses "HH:MM" (24h, zero-padded).
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != len("15:04") {
		return 0, fmt.Errorf("invalid time %q, want HH:MM", s)
	}
	return TimeOfDay(t.Hour()*60 + t.Minute()), nil
}

// TimeOfDayOf returns t's local time of day, in t's own location.
func TimeOfDayOf(t time.Time) TimeOfDay {
	return TimeOfDay(t.Hour()*60 + t.Minute())
}
