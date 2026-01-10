package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aristath/gollama-ui/internal/client"
)

// ChatHandler handles chat-related requests
type ChatHandler struct {
	ollamaClient ChatClientInterface
}

// ChatClientInterface defines the interface for chat operations
type ChatClientInterface interface {
	ChatStream(ctx context.Context, req client.ChatRequest) (<-chan client.ChatResponse, error)
}

// NewChatHandler creates a new chat handler
func NewChatHandler(client ChatClientInterface) *ChatHandler {
	return &ChatHandler{
		ollamaClient: client,
	}
}

// Stream handles POST /api/chat with streaming support
func (h *ChatHandler) Stream(w http.ResponseWriter, r *http.Request) {
	var req client.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Model == "" {
		http.Error(w, "model is required", http.StatusBadRequest)
		return
	}

	if len(req.Messages) == 0 {
		http.Error(w, "messages array is required", http.StatusBadRequest)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Start streaming
	stream, err := h.ollamaClient.ChatStream(ctx, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to start chat: %v", err), http.StatusInternalServerError)
		return
	}

	// Set up Server-Sent Events
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Stream responses
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(w, "data: %s\n\n", `{"done": true, "error": "context cancelled"}`)
			flusher.Flush()
			return

		case response, ok := <-stream:
			if !ok {
				return
			}

			data, err := json.Marshal(response)
			if err != nil {
				fmt.Fprintf(w, "data: %s\n\n", `{"done": true, "error": "failed to marshal response"}`)
				flusher.Flush()
				return
			}

			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()

			if response.Done {
				return
			}
		}
	}
}