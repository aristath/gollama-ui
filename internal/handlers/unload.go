package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// UnloadHandler handles model unloading requests
type UnloadHandler struct {
	ollamaClient UnloadClientInterface
}

// UnloadClientInterface defines the interface for unload operations
type UnloadClientInterface interface {
	UnloadModel(ctx context.Context, modelName string) error
}

// NewUnloadHandler creates a new unload handler
func NewUnloadHandler(client UnloadClientInterface) *UnloadHandler {
	return &UnloadHandler{
		ollamaClient: client,
	}
}

// Unload handles POST /api/models/{model}/unload
func (h *UnloadHandler) Unload(w http.ResponseWriter, r *http.Request) {
	modelName := chi.URLParam(r, "model")
	if modelName == "" {
		http.Error(w, "model name is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	
	if err := h.ollamaClient.UnloadModel(ctx, modelName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to unload model: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"model":   modelName,
		"message": fmt.Sprintf("Model %s unloaded from memory", modelName),
	}); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}