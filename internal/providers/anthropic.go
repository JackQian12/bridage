package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicAdapter translates OpenAI-style requests to the Anthropic Messages API.
type AnthropicAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewAnthropicAdapter creates an adapter for the Anthropic API.
func NewAnthropicAdapter(baseURL, apiKey string, timeout time.Duration) *AnthropicAdapter {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}
}

// anthropicVersion is the API version header value required by Anthropic.
const anthropicVersion = "2023-06-01"

// ─── Request/response wire types ─────────────────────────────────────────────

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Stream    bool               `json:"stream"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Model      string             `json:"model"`
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
	Error      *anthropicError    `json:"error,omitempty"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ─── Translation helpers ──────────────────────────────────────────────────────

func buildAnthropicRequest(req *ChatRequest) anthropicRequest {
	ar := anthropicRequest{
		Model:     req.UpstreamModel,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
	if ar.MaxTokens == 0 {
		ar.MaxTokens = 4096 // Anthropic requires max_tokens
	}
	for _, m := range req.Messages {
		if m.Role == "system" {
			ar.System = m.Content
		} else {
			ar.Messages = append(ar.Messages, anthropicMessage{Role: m.Role, Content: m.Content})
		}
	}
	return ar
}

func anthropicResponseToChatResponse(ar *anthropicResponse, model string) *ChatResponse {
	text := ""
	for _, c := range ar.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	finishReason := ar.StopReason
	if finishReason == "end_turn" {
		finishReason = "stop"
	}
	total := ar.Usage.InputTokens + ar.Usage.OutputTokens
	return &ChatResponse{
		ID:     ar.ID,
		Object: "chat.completion",
		Model:  model,
		Choices: []ChatChoice{{
			Index:        0,
			Message:      ChatMessage{Role: "assistant", Content: text},
			FinishReason: finishReason,
		}},
		Usage: Usage{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
			TotalTokens:      total,
		},
	}
}

// ─── API calls ────────────────────────────────────────────────────────────────

func (a *AnthropicAdapter) post(ctx context.Context, path string, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	return a.client.Do(req)
}

// Chat performs a non-streaming Anthropic Messages call and returns a normalized ChatResponse.
func (a *AnthropicAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	ar := buildAnthropicRequest(req)
	ar.Stream = false

	resp, err := a.post(ctx, "/v1/messages", ar)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return nil, fmt.Errorf("anthropic %d %s: %s", resp.StatusCode, parsed.Error.Type, parsed.Error.Message)
		}
		return nil, fmt.Errorf("anthropic %d: %s", resp.StatusCode, string(raw))
	}
	return anthropicResponseToChatResponse(&parsed, req.Model), nil
}

// ChatStream opens an Anthropic streaming Messages call and returns the raw SSE body.
// The caller receives raw "text_delta" events; the relay service wraps them into
// OpenAI-compatible SSE format before forwarding to the downstream client.
func (a *AnthropicAdapter) ChatStream(ctx context.Context, req *ChatRequest) (io.ReadCloser, error) {
	ar := buildAnthropicRequest(req)
	ar.Stream = true

	resp, err := a.post(ctx, "/v1/messages", ar)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic stream %d: %s", resp.StatusCode, string(raw))
	}
	return resp.Body, nil
}

// Embeddings is not supported by the Anthropic API.
func (a *AnthropicAdapter) Embeddings(_ context.Context, _ *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	return nil, fmt.Errorf("anthropic does not support embeddings")
}

// Images is not supported by the Anthropic API.
func (a *AnthropicAdapter) Images(_ context.Context, _ *ImagesRequest) (*ImagesResponse, error) {
	return nil, fmt.Errorf("anthropic does not support image generation")
}
