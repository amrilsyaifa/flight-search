package lion

type searchResponse struct {
	Success bool `json:"success"`
	Data    struct {
		AvailableFlights []flight `json:"available_flights"`
	} `json:"data"`
}

type flight struct {
	ID         string    `json:"id"`
	Carrier    carrier   `json:"carrier"`
	Route      route     `json:"route"`
	Schedule   schedule  `json:"schedule"`
	FlightTime int       `json:"flight_time"`
	IsDirect   bool      `json:"is_direct"`
	StopCount  int       `json:"stop_count"`
	Layovers   []layover `json:"layovers"`
	Pricing    pricing   `json:"pricing"`
	SeatsLeft  int       `json:"seats_left"`
	PlaneType  string    `json:"plane_type"`
	Services   services  `json:"services"`
}

type carrier struct {
	Name string `json:"name"`
	IATA string `json:"iata"`
}

type route struct {
	From airport `json:"from"`
	To   airport `json:"to"`
}

type airport struct {
	Code string `json:"code"`
	Name string `json:"name"`
	City string `json:"city"`
}

type schedule struct {
	Departure         string `json:"departure"`
	DepartureTimezone string `json:"departure_timezone"`
	Arrival           string `json:"arrival"`
	ArrivalTimezone   string `json:"arrival_timezone"`
}

type layover struct {
	Airport         string `json:"airport"`
	DurationMinutes int    `json:"duration_minutes"`
}

type pricing struct {
	Total    int64  `json:"total"`
	Currency string `json:"currency"`
	FareType string `json:"fare_type"`
}

type services struct {
	WifiAvailable    bool `json:"wifi_available"`
	MealsIncluded    bool `json:"meals_included"`
	BaggageAllowance struct {
		Cabin string `json:"cabin"`
		Hold  string `json:"hold"`
	} `json:"baggage_allowance"`
}
