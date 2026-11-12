package providers

import (
	"fmt"
	"strings"

	"github.com/amrilsyaifa/flight-search/internal/models"
)

func ParseCabinClass(s string) (models.CabinClass, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "economy":
		return models.CabinEconomy, nil
	case "premium_economy", "premium economy":
		return models.CabinPremiumEconomy, nil
	case "business":
		return models.CabinBusiness, nil
	case "first":
		return models.CabinFirst, nil
	}
	return "", fmt.Errorf("unknown cabin class %q", s)
}
