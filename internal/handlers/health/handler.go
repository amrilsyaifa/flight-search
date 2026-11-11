package health

import (
	"context"

	"github.com/amrilsyaifa/flight-search/internal/oapi"
)

type Handler struct{}

func New() *Handler { return &Handler{} }

func (h *Handler) GetHealth(_ context.Context, _ oapi.GetHealthRequestObject) (oapi.GetHealthResponseObject, error) {
	return oapi.GetHealth200JSONResponse{Status: "ok"}, nil
}
