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

// GeminiAdapter translates OpenAI-style requests to the Google Gemini generateContent API.
type GeminiAdapter struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewGeminiAdapter creates an adapter for the Google Gemini API.
func NewGeminiAdapter(baseURL, apiKey string, timeout time.Duration) *GeminiAdapter {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &GeminiAdapter{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{Timeout: timeout},
	}
}

// ─── Wire types ───────────────────────────────────────────────────────────────

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"` // "user" | "model"
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
	Index        int           `json:"index"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
	Error         *geminiError        `json:"error,omitempty"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type geminiEmbedContentRequest struct {
	Model   string        `json:"model"`
	Content geminiContent `json:"content"`
}

type geminiEmbedContentResponse struct {
	Embedding struct {
		Values []float64 `json:"values"`
	} `json:"embedding"`
	Error *geminiError `json:"error,omitempty"`
}

// ─── Translation helpers ──────────────────────────────────────────────────────

func buildGeminiRequest(req *ChatRequest) geminiRequest {
	gr := geminiRequest{}
	for _, m := range req.Messages {
		if m.Role == "system" {
			gr.SystemInstruction = &geminiContent{Parts: []geminiPart{{Text: m.Content}}}
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		gr.Contents = append(gr.Contents, geminiContent{
			Role:  role,
			Parts: []geminiPart{{Text: m.Content}},
		})
	}
	if req.MaxTokens > 0 || req.Temperature != nil || req.TopP != nil {
		cfg := &geminiGenerationConfig{
			Temperature: req.Temperature,
			TopP:        req.TopP,
		}
		if req.MaxTokens > 0 {
			cfg.MaxOutputTokens = &req.MaxTokens
		}
		gr.GenerationConfig = cfg
	}
	return gr
}

func geminiResponseToChatResponse(gr *geminiResponse, model string) *ChatResponse {
	text := ""
	finishReason := "stop"
	if len(gr.Candidates) > 0 {
		c := gr.Candidates[0]
		for _, p := range c.Content.Parts {
			text += p.Text
		}
		finishReason = strings.ToLower(c.FinishReason)
		if finishReason == "stop" || finishReason == "max_tokens" {
			// fine as-is
		} else {
			finishReason = "stop"
		}
	}
	return &ChatResponse{
		Object: "chat.completion",
		Model:  model,
		Choices: []ChatChoice{{
			Index:        0,
			Message:      ChatMessage{Role: "assistant", Content: text},
			FinishReason: finishReason,
		}},
		Usage: Usage{
			PromptTokens:     gr.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gr.UsageMetadata.TotalTokenCount,
		},
	}
}

// ─── API calls ────────────────────────────────────────────────────────────────

func (a *GeminiAdapter) postJSON(ctx context.Context, url string, body any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", a.apiKey)
	return a.client.Do(req)
}

// Chat performs a non-streaming Gemini generateContent call.
func (a *GeminiAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", a.baseURL, req.UpstreamModel)
	gr := buildGeminiRequest(req)

	resp, err := a.postJSON(ctx, url, gr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var parsed geminiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if parsed.Error != nil {
			return nil, fmt.Errorf("gemini %d %s: %s", resp.StatusCode, parsed.Error.Status, parsed.Error.Message)
		}
		return nil, fmt.Errorf("gemini %d: %s", resp.StatusCode, string(raw))
	}
	return geminiResponseToChatResponse(&parsed, req.Model), nil
}

// ChatStream opens a Gemini streamGenerateContent call and returns the raw SSE body.
func (a *GeminiAdapter) ChatStream(ctx context.Context, req *ChatRequest) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", a.baseURL, req.UpstreamModel)
	gr := buildGeminiRequest(req)

	resp, err := a.postJSON(ctx, url, gr)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini stream %d: %s", resp.StatusCode, string(raw))
	}
	return resp.Body, nil
}

// Embeddings calls the Gemini embedContent endpoint for each input string.
func (a *GeminiAdapter) Embeddings(ctx context.Context, req *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	var data []EmbeddingObject
	totalTokens := 0
	for i, text := range req.Input {
		url := fmt.Sprintf("%s/v1beta/models/%s:embedContent", a.baseURL, req.UpstreamModel)
		body := geminiEmbedContentRequest{
			Model: "models/" + req.UpstreamModel,
			Content: geminiContent{
				Parts: []geminiPart{{Text: text}},
			},
		}
		resp, err := a.postJSON(ctx, url, body)
		if err != nil {
			return nil, err
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var parsed geminiEmbedContentResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("decode gemini embedding: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			if parsed.Error != nil {
				return nil, fmt.Errorf("gemini embed %d: %s", resp.StatusCode, parsed.Error.Message)
			}
			return nil, fmt.Errorf("gemini embed %d: %s", resp.StatusCode, string(raw))
		}
		data = append(data, EmbeddingObject{
			Index:     i,
			Object:    "embedding",
			Embedding: parsed.Embedding.Values,
		})
		totalTokens += len(strings.Fields(text)) // approximate
	}
	return &EmbeddingsResponse{
		Object: "list",
		Model:  req.Model,
		Data:   data,
		Usage:  Usage{TotalTokens: totalTokens},
	}, nil
}

// Images is not supported by the Gemini API via this adapter.
func (a *GeminiAdapter) Images(_ context.Context, _ *ImagesRequest) (*ImagesResponse, error) {
	return nil, fmt.Errorf("gemini image generation is not supported via this adapter")
}
