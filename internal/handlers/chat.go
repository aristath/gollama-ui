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
	ollamaClient  ChatClientInterface
	toolExecutor  *ToolExecutor
	chatTimeout   time.Duration
}

// ChatClientInterface defines the interface for chat operations
type ChatClientInterface interface {
	ChatStream(ctx context.Context, req client.ChatRequest) (<-chan client.ChatResponse, error)
}

// NewChatHandler creates a new chat handler
func NewChatHandler(client ChatClientInterface, toolExecutor *ToolExecutor) *ChatHandler {
	return NewChatHandlerWithTimeout(client, toolExecutor, 24*time.Hour)
}

// NewChatHandlerWithTimeout creates a new chat handler with a custom timeout
func NewChatHandlerWithTimeout(client ChatClientInterface, toolExecutor *ToolExecutor, timeout time.Duration) *ChatHandler {
	return &ChatHandler{
		ollamaClient: client,
		toolExecutor: toolExecutor,
		chatTimeout:  timeout,
	}
}

// Stream handles POST /api/chat with streaming support and function calling
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
	ctx, cancel := context.WithTimeout(r.Context(), h.chatTimeout)
	defer cancel()

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

	// Function calling loop - may need multiple rounds if tool calls are made
	h.streamWithFunctionCalling(ctx, w, flusher, &req)
}

// streamWithFunctionCalling handles the function calling loop
func (h *ChatHandler) streamWithFunctionCalling(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, req *client.ChatRequest) {
	// Add tool definitions to request
	if h.toolExecutor != nil {
		req.Tools = h.toolExecutor.GetAvailableTools()
	}

	// Start streaming from llama.cpp
	stream, err := h.ollamaClient.ChatStream(ctx, *req)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", `{"done": true, "error": "Failed to start chat"}`)
		flusher.Flush()
		return
	}

	// Collect response data
	var assistantContent string
	toolCallsMap := make(map[string]client.ToolCall) // Map tool calls by ID to handle partial streaming
	var finishReason string

	// Stream responses and collect tool calls
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(w, "data: %s\n\n", `{"done": true, "error": "context cancelled"}`)
			flusher.Flush()
			return

		case response, ok := <-stream:
			if !ok {
				// Stream closed, check if we need to handle tool calls
				if len(toolCallsMap) > 0 {
					// Convert map back to slice, filtering out incomplete/empty tool calls
					toolCalls := make([]client.ToolCall, 0)
					for _, tc := range toolCallsMap {
						// Only include tool calls with valid data
						if tc.ID != "" && tc.Function.Name != "" {
							toolCalls = append(toolCalls, tc)
						}
					}
					if len(toolCalls) > 0 {
						// Execute tool calls and loop back
						h.executeAndContinue(ctx, w, flusher, req, assistantContent, toolCalls)
						return
					}
				}
				// No tool calls, we're done
				return
			}

			// Collect tool calls - merge partial updates from streaming
			if len(response.Message.ToolCalls) > 0 {
				for _, tc := range response.Message.ToolCalls {
					// If this chunk has an ID, use it as the key
					if tc.ID != "" {
						existing := toolCallsMap[tc.ID]
						if tc.Type != "" {
							existing.Type = tc.Type
						}
						existing.ID = tc.ID
						if tc.Function.Name != "" {
							existing.Function.Name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							existing.Function.Arguments += tc.Function.Arguments
						}
						toolCallsMap[tc.ID] = existing
					} else if tc.Function.Arguments != "" && tc.Function.Name == "" {
						// This chunk only has Arguments (no ID or name) - find the latest tool call and append to it
						// This handles streaming where arguments come in separate chunks after ID chunk
						for _, existing := range toolCallsMap {
							if existing.ID != "" && existing.Function.Name != "" {
								// Update the tool call with this argument chunk
								existing.Function.Arguments += tc.Function.Arguments
								toolCallsMap[existing.ID] = existing
								break // Only update the first matching one
							}
						}
					}
				}
			}

			// Collect assistant content
			if response.Message.Content != "" {
				assistantContent += response.Message.Content
			}

			// Collect finish reason
			if response.DoneReason != "" {
				finishReason = response.DoneReason
			}

			// Forward content chunks to frontend
			if response.Message.Content != "" || len(response.Message.ToolCalls) > 0 {
				data, err := json.Marshal(response)
				if err != nil {
					fmt.Fprintf(w, "data: %s\n\n", `{"done": true, "error": "failed to marshal response"}`)
					flusher.Flush()
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				flusher.Flush()
			}

			// Check if stream is done
			if response.Done {
				// If we have tool calls, execute them and continue
				if len(toolCallsMap) > 0 && finishReason == "tool_calls" {
					// Convert map back to slice, filtering out incomplete tool calls
					toolCalls := make([]client.ToolCall, 0)
					for _, tc := range toolCallsMap {
						if tc.ID != "" && tc.Function.Name != "" {
							toolCalls = append(toolCalls, tc)
						}
					}
					if len(toolCalls) > 0 {
						// Debug logging
						for _, tc := range toolCalls {
							fmt.Printf("  Tool: %s, Args: %s\n", tc.Function.Name, tc.Function.Arguments)
						}
						h.executeAndContinue(ctx, w, flusher, req, assistantContent, toolCalls)
						return
					}
				}
				// No tool calls, we're truly done
				return
			}
		}
	}
}

// executeAndContinue executes tool calls and gets final response
func (h *ChatHandler) executeAndContinue(ctx context.Context, w http.ResponseWriter, flusher http.Flusher,
	req *client.ChatRequest, assistantContent string, toolCalls []client.ToolCall) {

	// Add assistant message with tool calls to history
	req.Messages = append(req.Messages, client.ChatMessage{
		Role:      "assistant",
		Content:   assistantContent,
		ToolCalls: toolCalls,
	})

	// Execute each tool call and add results
	for _, toolCall := range toolCalls {
		result, err := h.toolExecutor.ExecuteToolCall(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
		if err != nil {
			result = fmt.Sprintf("Error executing tool %s: %v", toolCall.Function.Name, err)
		} else {
		}

		// Add tool result to messages
		req.Messages = append(req.Messages, client.ChatMessage{
			Role:       "tool",
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	// Get final response from llama.cpp with tool results
	stream, err := h.ollamaClient.ChatStream(ctx, *req)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"done": true, "error": "Failed to get final response: %v"}`, err))
		flusher.Flush()
		return
	}

	// Stream final response
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
