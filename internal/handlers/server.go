package handlers

import (
	"context"

	"github.com/amrilsyaifa/flight-search/internal/handlers/health"
	"github.com/amrilsyaifa/flight-search/internal/oapi"
)

type Server struct {
	healthHandler *health.Handler
}

var _ oapi.StrictServerInterface = (*Server)(nil)

func NewServer(h *health.Handler) *Server {
	return &Server{healthHandler: h}
}

func (s *Server) GetHealth(ctx context.Context, req oapi.GetHealthRequestObject) (oapi.GetHealthResponseObject, error) {
	return s.healthHandler.GetHealth(ctx, req)
}
