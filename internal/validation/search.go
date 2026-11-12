package validation

import (
	"fmt"
	"regexp"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

var airportCode = regexp.MustCompile(`^[A-Z]{3}$`)

func SearchCriteria(c models.SearchCriteria) error {
	switch {
	case !airportCode.MatchString(c.Origin):
		return invalid("origin must be a 3-letter IATA code")
	case !airportCode.MatchString(c.Destination):
		return invalid("destination must be a 3-letter IATA code")
	case c.Origin == c.Destination:
		return invalid("destination must differ from origin")
	case c.Passengers < 1 || c.Passengers > 9:
		return invalid("passengers must be between 1 and 9")
	}

	switch c.CabinClass {
	case models.CabinEconomy, models.CabinPremiumEconomy, models.CabinBusiness, models.CabinFirst:
	default:
		return invalid(fmt.Sprintf("unsupported cabin class %q", c.CabinClass))
	}
	return nil
}

func invalid(msg string) error {
	return fmt.Errorf("%w: %s", models.ErrInvalidRequest, msg)
}
