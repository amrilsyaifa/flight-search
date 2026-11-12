package handlers

import (
	"context"

	"github.com/amrilsyaifa/flight-search/internal/handlers/flight"
	"github.com/amrilsyaifa/flight-search/internal/handlers/health"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
)

type Server struct {
	healthHandler *health.Handler
	flightHandler *flight.Handler
}

var _ oapi.StrictServerInterface = (*Server)(nil)

func NewServer(h *health.Handler, f *flight.Handler) *Server {
	return &Server{healthHandler: h, flightHandler: f}
}

func (s *Server) GetHealth(ctx context.Context, req oapi.GetHealthRequestObject) (oapi.GetHealthResponseObject, error) {
	return s.healthHandler.GetHealth(ctx, req)
}

func (s *Server) SearchFlights(ctx context.Context, req oapi.SearchFlightsRequestObject) (oapi.SearchFlightsResponseObject, error) {
	return s.flightHandler.SearchFlights(ctx, req)
}
