package providers

import (
	"context"
	"io"
)

// ─── Normalized request / response types ─────────────────────────────────────

// ChatMessage is a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"` // "system" | "user" | "assistant" | "tool"
	Content string `json:"content"`
}

// ChatRequest is the normalized input for a chat completion call.
type ChatRequest struct {
	Model       string        `json:"model"` // canonical model name
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stream      bool          `json:"stream"`
	// Upstream model name; set by relay after model catalog lookup.
	UpstreamModel string `json:"-"`
}

// ChatChoice is one completion candidate.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// Usage contains token counts returned by the provider.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is the normalized output for a chat completion call.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

// EmbeddingsRequest is the normalized input for an embeddings call.
type EmbeddingsRequest struct {
	Model         string   `json:"model"`
	Input         []string `json:"input"`
	UpstreamModel string   `json:"-"`
}

// EmbeddingObject is a single embedding vector result.
type EmbeddingObject struct {
	Index     int       `json:"index"`
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
}

// EmbeddingsResponse is the normalized output for an embeddings call.
type EmbeddingsResponse struct {
	Object string            `json:"object"`
	Model  string            `json:"model"`
	Data   []EmbeddingObject `json:"data"`
	Usage  Usage             `json:"usage"`
}

// ImagesRequest is the normalized input for an image generation call.
type ImagesRequest struct {
	Model         string `json:"model"`
	Prompt        string `json:"prompt"`
	N             int    `json:"n"`
	Size          string `json:"size"`
	UpstreamModel string `json:"-"`
}

// ImageObject represents a single generated image URL or b64.
type ImageObject struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

// ImagesResponse is the normalized output for image generation.
type ImagesResponse struct {
	Created int64         `json:"created"`
	Data    []ImageObject `json:"data"`
}

// ─── Adapter interface ────────────────────────────────────────────────────────

// Adapter is the interface every provider must satisfy.
type Adapter interface {
	// Chat performs a non-streaming chat completion.
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	// ChatStream returns an io.ReadCloser producing server-sent events in
	// OpenAI-compatible format (data: {...}\n\n). The caller must close it.
	ChatStream(ctx context.Context, req *ChatRequest) (io.ReadCloser, error)
	// Embeddings produces vector embeddings.
	Embeddings(ctx context.Context, req *EmbeddingsRequest) (*EmbeddingsResponse, error)
	// Images generates images from a prompt.
	Images(ctx context.Context, req *ImagesRequest) (*ImagesResponse, error)
}
