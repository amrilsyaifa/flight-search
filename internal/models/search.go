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
	Criteria      SearchCriteria
	Flights       []Flight
	ReturnFlights []Flight
	Metadata      SearchMetadata
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

func (d Date) Before(other Date) bool {
	if d.Year != other.Year {
		return d.Year < other.Year
	}
	if d.Month != other.Month {
		return d.Month < other.Month
	}
	return d.Day < other.Day
}

func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

func (c SearchCriteria) IsRoundTrip() bool { return c.ReturnDate != nil }

func (c SearchCriteria) ReturnLeg() SearchCriteria {
	back := c
	back.Origin, back.Destination = c.Destination, c.Origin
	back.DepartureDate = *c.ReturnDate
	back.ReturnDate = nil
	return back
}
