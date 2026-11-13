package flight

import (
	"context"
	"errors"
	"log/slog"

	"github.com/amrilsyaifa/flight-search/internal/models"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
)

type SearchService interface {
	Search(ctx context.Context, req models.SearchRequest) (models.SearchResult, error)
}

type Handler struct {
	search SearchService
	logger *slog.Logger
}

func New(search SearchService, logger *slog.Logger) *Handler {
	return &Handler{search: search, logger: logger}
}

func (h *Handler) SearchFlights(ctx context.Context, req oapi.SearchFlightsRequestObject) (oapi.SearchFlightsResponseObject, error) {
	if req.Body == nil {
		return oapi.SearchFlights400JSONResponse(errorBody("invalid_request", "request body is required")), nil
	}

	searchReq, err := toSearchRequest(*req.Body)
	if err != nil {
		return oapi.SearchFlights400JSONResponse(errorBody("invalid_request", err.Error())), nil
	}

	result, err := h.search.Search(ctx, searchReq)
	if err != nil {
		if errors.Is(err, models.ErrInvalidRequest) {
			return oapi.SearchFlights400JSONResponse(errorBody("invalid_request", err.Error())), nil
		}
		h.logger.ErrorContext(ctx, "search failed", "error", err)
		return oapi.SearchFlights500JSONResponse(errorBody("internal_error", "unexpected error")), nil
	}

	return oapi.SearchFlights200JSONResponse(toResponse(result)), nil
}

func errorBody(code, message string) oapi.ErrorResponse {
	return oapi.ErrorResponse{Error: oapi.ErrorDetail{Code: code, Message: message}}
}
