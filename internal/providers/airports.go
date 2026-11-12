package providers

var airportCities = map[string]string{
	"CGK": "Jakarta",
	"DPS": "Denpasar",
	"SUB": "Surabaya",
	"UPG": "Makassar",
	"SOC": "Solo",
	// Add more city
}

func CityOf(airport string) string {
	return airportCities[airport]
}
