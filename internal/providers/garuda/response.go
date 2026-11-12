package garuda

type searchResponse struct {
	Status  string   `json:"status"`
	Flights []flight `json:"flights"`
}

type flight struct {
	FlightID        string    `json:"flight_id"`
	Airline         string    `json:"airline"`
	AirlineCode     string    `json:"airline_code"`
	Departure       endpoint  `json:"departure"`
	Arrival         endpoint  `json:"arrival"`
	DurationMinutes int       `json:"duration_minutes"`
	Stops           int       `json:"stops"`
	Aircraft        string    `json:"aircraft"`
	Price           price     `json:"price"`
	AvailableSeats  int       `json:"available_seats"`
	FareClass       string    `json:"fare_class"`
	Baggage         baggage   `json:"baggage"`
	Amenities       []string  `json:"amenities"`
	Segments        []segment `json:"segments"`
}

type endpoint struct {
	Airport  string `json:"airport"`
	City     string `json:"city"`
	Time     string `json:"time"`
	Terminal string `json:"terminal"`
}

type price struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

type baggage struct {
	CarryOn int `json:"carry_on"`
	Checked int `json:"checked"`
}

type segment struct {
	FlightNumber    string   `json:"flight_number"`
	Departure       endpoint `json:"departure"`
	Arrival         endpoint `json:"arrival"`
	DurationMinutes int      `json:"duration_minutes"`
	LayoverMinutes  int      `json:"layover_minutes"`
}
