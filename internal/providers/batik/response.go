package batik

type searchResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Results []flight `json:"results"`
}

type flight struct {
	FlightNumber      string       `json:"flightNumber"`
	AirlineName       string       `json:"airlineName"`
	AirlineIATA       string       `json:"airlineIATA"`
	Origin            string       `json:"origin"`
	Destination       string       `json:"destination"`
	DepartureDateTime string       `json:"departureDateTime"`
	ArrivalDateTime   string       `json:"arrivalDateTime"`
	TravelTime        string       `json:"travelTime"`
	NumberOfStops     int          `json:"numberOfStops"`
	Connections       []connection `json:"connections"`
	Fare              fare         `json:"fare"`
	SeatsAvailable    int          `json:"seatsAvailable"`
	AircraftModel     string       `json:"aircraftModel"`
	BaggageInfo       string       `json:"baggageInfo"`
	OnboardServices   []string     `json:"onboardServices"`
}

type connection struct {
	StopAirport  string `json:"stopAirport"`
	StopDuration string `json:"stopDuration"`
}

type fare struct {
	BasePrice    int64  `json:"basePrice"`
	Taxes        int64  `json:"taxes"`
	TotalPrice   int64  `json:"totalPrice"`
	CurrencyCode string `json:"currencyCode"`
	Class        string `json:"class"`
}
