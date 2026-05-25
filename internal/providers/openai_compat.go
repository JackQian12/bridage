package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatAdapter calls any OpenAI-compatible HTTP API.
type OpenAICompatAdapter struct {
	baseURL    string
	authHeader string
	authScheme string
	apiKey     string
	client     *http.Client
}

// NewOpenAICompatAdapter builds an adapter for an OpenAI-compatible provider.
func NewOpenAICompatAdapter(baseURL, authHeader, authScheme, apiKey string, timeout time.Duration) *OpenAICompatAdapter {
	return &OpenAICompatAdapter{
		baseURL:    strings.TrimRight(baseURL, "/"),
		authHeader: authHeader,
		authScheme: authScheme,
		apiKey:     apiKey,
		client:     &http.Client{Timeout: timeout},
	}
}

func (a *OpenAICompatAdapter) authValue() string {
	if a.authScheme == "" {
		return a.apiKey
	}
	return a.authScheme + " " + a.apiKey
}

func (a *OpenAICompatAdapter) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(a.authHeader, a.authValue())
	return a.client.Do(req)
}

// ─── Chat ─────────────────────────────────────────────────────────────────────

func (a *OpenAICompatAdapter) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body := map[string]any{
		"model":    req.UpstreamModel,
		"messages": req.Messages,
		"stream":   false,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}

	resp, err := a.do(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(raw))
	}
	var cr ChatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	return &cr, nil
}

// ChatStream returns the raw SSE stream from the upstream provider as-is.
// The relay service is responsible for forwarding the bytes to the client.
func (a *OpenAICompatAdapter) ChatStream(ctx context.Context, req *ChatRequest) (io.ReadCloser, error) {
	body := map[string]any{
		"model":    req.UpstreamModel,
		"messages": req.Messages,
		"stream":   true,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		body["top_p"] = *req.TopP
	}

	// Use a detached context so the upstream connection is not killed when we
	// return from this function; the caller owns resp.Body.
	resp, err := a.do(ctx, http.MethodPost, "/chat/completions", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(raw))
	}
	return resp.Body, nil
}

// ─── Embeddings ───────────────────────────────────────────────────────────────

func (a *OpenAICompatAdapter) Embeddings(ctx context.Context, req *EmbeddingsRequest) (*EmbeddingsResponse, error) {
	body := map[string]any{
		"model": req.UpstreamModel,
		"input": req.Input,
	}
	resp, err := a.do(ctx, http.MethodPost, "/embeddings", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(raw))
	}
	var er EmbeddingsResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return nil, fmt.Errorf("decode embeddings response: %w", err)
	}
	return &er, nil
}

// ─── Images ───────────────────────────────────────────────────────────────────

func (a *OpenAICompatAdapter) Images(ctx context.Context, req *ImagesRequest) (*ImagesResponse, error) {
	n := req.N
	if n == 0 {
		n = 1
	}
	size := req.Size
	if size == "" {
		size = "1024x1024"
	}
	body := map[string]any{
		"model":  req.UpstreamModel,
		"prompt": req.Prompt,
		"n":      n,
		"size":   size,
	}
	resp, err := a.do(ctx, http.MethodPost, "/images/generations", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream %d: %s", resp.StatusCode, string(raw))
	}
	var ir ImagesResponse
	if err := json.Unmarshal(raw, &ir); err != nil {
		return nil, fmt.Errorf("decode images response: %w", err)
	}
	return &ir, nil
}

// ─── SSE passthrough helper ───────────────────────────────────────────────────

// ScanSSELines scans an SSE stream and calls fn for each "data: ..." line.
// The function is exported so the relay service can use it when it needs to
// inspect streaming tokens for quota accounting.
func ScanSSELines(r io.Reader, fn func(data string) error) error {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		if err := fn(data); err != nil {
			return err
		}
	}
	return sc.Err()
}
