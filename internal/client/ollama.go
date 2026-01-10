package client

import (
	"context"
	"fmt"
	"time"

	"github.com/liliang-cn/ollama-go"
)

// Client wraps the ollama-go client with our interface
type Client struct {
	ollamaClient *ollama.Client
	host         string
}

// Model represents an Ollama model
type Model struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Digest     string `json:"digest"`
	ModifiedAt string `json:"modified_at"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ChatResponse represents a streaming chat response chunk
type ChatResponse struct {
	Model     string       `json:"model"`
	Message   ChatMessage  `json:"message"`
	Done      bool         `json:"done"`
	DoneReason string      `json:"done_reason,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// New creates a new Ollama client
func New(host string) (*Client, error) {
	ollamaClient, err := ollama.NewClient(ollama.WithHost(host))
	if err != nil {
		return nil, fmt.Errorf("failed to create ollama client: %w", err)
	}

	return &Client{
		ollamaClient: ollamaClient,
		host:         host,
	}, nil
}

// ListModels returns all available models
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	listResponse, err := c.ollamaClient.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	result := make([]Model, 0, len(listResponse.Models))
	for _, m := range listResponse.Models {
		var modifiedAt string
		if m.ModifiedAt != nil {
			modifiedAt = m.ModifiedAt.Format(time.RFC3339)
		}

		result = append(result, Model{
			Name:       m.Model,
			Size:       m.Size,
			Digest:     m.Digest,
			ModifiedAt: modifiedAt,
		})
	}

	return result, nil
}

// ChatStream handles streaming chat requests
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	// Convert our message format to ollama-go format
	messages := make([]ollama.Message, 0, len(req.Messages))
	for _, msg := range req.Messages {
		messages = append(messages, ollama.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Create ollama chat request
	stream := true
	ollamaReq := &ollama.ChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   &stream, // Always stream
	}

	// Get the streams (response channel and error channel)
	responseChan, errorChan := c.ollamaClient.ChatStream(ctx, ollamaReq)

	// Convert stream to our format
	ourResponseChan := make(chan ChatResponse, 10)

	go func() {
		defer close(ourResponseChan)

		for {
			select {
			case <-ctx.Done():
				return

			case err, ok := <-errorChan:
				if !ok {
					return
				}
				if err != nil {
					ourResponseChan <- ChatResponse{
						Done:  true,
						Error: err.Error(),
					}
					return
				}

			case response, ok := <-responseChan:
				if !ok {
					return
				}

				ourResponseChan <- ChatResponse{
					Model:      response.Model,
					Message: ChatMessage{
						Role:    response.Message.Role,
						Content: response.Message.Content,
					},
					Done:       response.Done,
					DoneReason: response.DoneReason,
				}

				if response.Done {
					return
				}
			}
		}
	}()

	return ourResponseChan, nil
}