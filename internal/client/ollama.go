package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client wraps the llama.cpp OpenAI API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	host       string
}

// Model represents a model (compatible with llama.cpp response)
type Model struct {
	Name       string `json:"name"`
	Size       int64  `json:"size,omitempty"`
	Digest     string `json:"digest,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

// Tool calling support structures
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest represents a chat request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
	Tools    []Tool        `json:"tools,omitempty"`
}

// ChatResponse represents a streaming chat response chunk
type ChatResponse struct {
	Model     string       `json:"model"`
	Message   ChatMessage  `json:"message"`
	Done      bool         `json:"done"`
	DoneReason string      `json:"done_reason,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// llama.cpp API structures
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

type OpenAIChoice struct {
	Index        int         `json:"index"`
	Delta        ChatMessage `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

type OpenAIChatChunk struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

// New creates a new llama.cpp client
func New(host string) (*Client, error) {
	if host == "" {
		host = "http://localhost:8080"
	}

	// Ensure host has protocol
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}

	return &Client{
		baseURL: host,
		httpClient: &http.Client{
			Timeout: 0, // No timeout for streaming responses
		},
		host: host,
	}, nil
}

// ListModels returns all available models
func (c *Client) ListModels(ctx context.Context) ([]Model, error) {
	url := fmt.Sprintf("%s/v1/models", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list models: status %d: %s", resp.StatusCode, string(body))
	}

	var openAIResp OpenAIModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert OpenAI format to our format
	result := make([]Model, 0, len(openAIResp.Data))
	for _, m := range openAIResp.Data {
		result = append(result, Model{
			Name:   m.ID,
			Digest: m.ID, // Use ID as digest since OpenAI format doesn't have digest
		})
	}

	return result, nil
}

// ChatStream handles streaming chat requests
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatResponse, error) {
	url := fmt.Sprintf("%s/v1/chat/completions", c.baseURL)

	// Convert to OpenAI format
	openAIReq := map[string]interface{}{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   true,
	}

	// Note: llama.cpp server does NOT support the tools parameter with any model.
	// Sending tools causes the server to close the connection (exit 52).
	// Tools are not sent to the backend. Instead, they are handled at the
	// application layer in gollama-ui if needed for future use or model upgrades.
	// See: https://github.com/ggml-org/llama.cpp/discussions/12601

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	fmt.Printf("DEBUG: Sending request to %s with model %s\n", url, req.Model)
	fmt.Printf("DEBUG: Request body: %s\n", string(body))

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		fmt.Printf("DEBUG: HTTP request error: %v\n", err)
		return nil, fmt.Errorf("failed to start chat: %w", err)
	}

	fmt.Printf("DEBUG: Got response status %d\n", resp.StatusCode)
	fmt.Printf("DEBUG: Response headers: %+v\n", resp.Header)

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("DEBUG: llama.cpp status %d, body: %s\n", resp.StatusCode, string(body))
		return nil, fmt.Errorf("failed to start chat: status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("DEBUG: Chat stream started successfully, model: %s\n", req.Model)

	// Create output channel
	responseChan := make(chan ChatResponse, 10)

	// Handle streaming in a goroutine
	go func() {
		defer close(responseChan)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip empty lines
			if line == "" {
				continue
			}

			// Check for [DONE] marker
			if line == "data: [DONE]" {
				responseChan <- ChatResponse{
					Model: req.Model,
					Done:  true,
				}
				return
			}

			// Parse Server-Sent Events format
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			jsonStr := strings.TrimPrefix(line, "data: ")
			var chunk OpenAIChatChunk
			if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
				responseChan <- ChatResponse{
					Model: req.Model,
					Done:  true,
					Error: fmt.Sprintf("failed to parse chunk: %v", err),
				}
				return
			}

			// Extract content from choices
			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]

				responseChan <- ChatResponse{
					Model: chunk.Model,
					Message: ChatMessage{
						Role:      choice.Delta.Role,
						Content:   choice.Delta.Content,
						ToolCalls: choice.Delta.ToolCalls,
					},
					Done:       choice.FinishReason != nil,
					DoneReason: func() string { if choice.FinishReason != nil { return *choice.FinishReason } ; return "" }(),
				}

				// If finished, return
				if choice.FinishReason != nil {
					return
				}
			}
		}

		if err := scanner.Err(); err != nil {
			responseChan <- ChatResponse{
				Model: req.Model,
				Done:  true,
				Error: fmt.Sprintf("scanner error: %v", err),
			}
		}
	}()

	return responseChan, nil
}

// UnloadModel is not supported by llama.cpp
// Returns an error indicating the operation is not supported
func (c *Client) UnloadModel(ctx context.Context, modelName string) error {
	return fmt.Errorf("unload model is not supported by llama.cpp server")
}
