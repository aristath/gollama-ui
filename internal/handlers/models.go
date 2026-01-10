package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aristath/gollama-ui/internal/client"
)

// ModelsHandler handles model-related requests
type ModelsHandler struct {
	ollamaClient ModelsClientInterface
}

// ModelsClientInterface defines the interface for model operations
type ModelsClientInterface interface {
	ListModels(ctx context.Context) ([]client.Model, error)
}

// NewModelsHandler creates a new models handler
func NewModelsHandler(client ModelsClientInterface) *ModelsHandler {
	return &ModelsHandler{
		ollamaClient: client,
	}
}

// List handles GET /api/models
func (h *ModelsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	models, err := h.ollamaClient.ListModels(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list models: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"models": models,
	}); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}