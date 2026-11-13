package flight

import (
	"context"
	"errors"
	"strings"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
)

func (h *Handler) SearchMultiCity(ctx context.Context, req oapi.SearchMultiCityRequestObject) (oapi.SearchMultiCityResponseObject, error) {
	if req.Body == nil {
		return oapi.SearchMultiCity400JSONResponse(errorBody("invalid_request", "request body is required")), nil
	}

	multiReq, err := toMultiCityRequest(*req.Body)
	if err != nil {
		return oapi.SearchMultiCity400JSONResponse(errorBody("invalid_request", err.Error())), nil
	}

	result, err := h.search.SearchMultiCity(ctx, multiReq)
	if err != nil {
		if errors.Is(err, models.ErrInvalidRequest) {
			return oapi.SearchMultiCity400JSONResponse(errorBody("invalid_request", err.Error())), nil
		}
		h.logger.ErrorContext(ctx, "multi-city search failed", "error", err)
		return oapi.SearchMultiCity500JSONResponse(errorBody("internal_error", "unexpected error")), nil
	}

	return oapi.SearchMultiCity200JSONResponse(toMultiCityResponse(result, *req.Body)), nil
}

func toMultiCityRequest(b oapi.MultiCitySearchRequest) (models.MultiCityRequest, error) {
	filters, err := toFilters(b.Filters)
	if err != nil {
		return models.MultiCityRequest{}, err
	}

	legs := make([]models.SearchCriteria, 0, len(b.Legs))
	for _, l := range b.Legs {
		legs = append(legs, models.SearchCriteria{
			Origin:        strings.ToUpper(strings.TrimSpace(l.Origin)),
			Destination:   strings.ToUpper(strings.TrimSpace(l.Destination)),
			DepartureDate: models.DateOf(l.DepartureDate.Time),
			Passengers:    b.Passengers,
			CabinClass:    models.CabinClass(b.CabinClass),
		})
	}

	return models.MultiCityRequest{Legs: legs, Filters: filters, Sort: toSort(b.Sort)}, nil
}

func toMultiCityResponse(r models.MultiCityResult, b oapi.MultiCitySearchRequest) oapi.MultiCitySearchResponse {
	legs := make([]oapi.LegResult, 0, len(r.Legs))
	total := 0
	for _, l := range r.Legs {
		total += len(l.Flights)
		legs = append(legs, oapi.LegResult{
			Origin:        l.Criteria.Origin,
			Destination:   l.Criteria.Destination,
			DepartureDate: toAPIDate(l.Criteria.DepartureDate),
			Flights:       toAPIFlights(l.Flights),
		})
	}

	return oapi.MultiCitySearchResponse{
		SearchCriteria: oapi.MultiCityCriteria{Passengers: b.Passengers, CabinClass: b.CabinClass},
		Metadata: oapi.SearchMetadata{
			TotalResults:       total,
			ProvidersQueried:   r.Metadata.ProvidersQueried,
			ProvidersSucceeded: r.Metadata.ProvidersSucceeded,
			ProvidersFailed:    r.Metadata.ProvidersFailed,
			SearchTimeMs:       r.Metadata.SearchTime.Milliseconds(),
			CacheHit:           r.Metadata.CacheHit,
		},
		Legs: legs,
	}
}
