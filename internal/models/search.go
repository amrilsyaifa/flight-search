package models

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

var ErrInvalidRequest = errors.New("invalid requesr")

type SearchCriteria struct {
	Origin        string
	Destination   string
	DepartureDate Date
	ReturnDate    *Date
	Passengers    int
	CabinClass    CabinClass
}

var airportCode = regexp.MustCompile(`^[A-Z]{3}$`)

type SearchResult struct {
	Criteria SearchCriteria
	Flights  []Flight
	Metadata SearchMetadata
}

type SearchMetadata struct {
	ProvidersQueried   int
	ProvidersSucceeded int
	ProvidersFailed    int
	SearchTime         time.Duration
	CacheHit           bool
}

type Date struct {
	Year  int
	Month time.Month
	Day   int
}

func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}
