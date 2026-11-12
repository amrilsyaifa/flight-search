package models

import "time"

type CabinClass string

const (
	CabinEconomy        CabinClass = "economy"
	CabinPremiumEconomy CabinClass = "premium_economy"
	CabinBusiness       CabinClass = "business"
	CabinFirst          CabinClass = "first"
)

type Flight struct {
	ID              string
	Provider        string
	Airline         Airline
	FlightNumber    string
	Departure       FlightPoint
	Arrival         FlightPoint
	DurationMinutes int
	Layovers        []Layover
	Price           Price
	AvailableSeats  int
	CabinClass      CabinClass
	Aircraft        string
	Amenities       []string
	Baggage         Baggage
}

func (f Flight) Stops() int {
	return len(f.Layovers)
}

type Airline struct {
	Name string
	Code string
}

type FlightPoint struct {
	Airport string
	City    string
	Time    time.Time
}

type Layover struct {
	Airport         string
	DurationMinutes int
}

type Price struct {
	Amount   int64
	Currency string
}

type Baggage struct {
	CarryOn string
	Checked string
}
