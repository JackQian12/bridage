package publicapi

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nuts/bridage/internal/models"
	"github.com/nuts/bridage/internal/providers"
	"github.com/nuts/bridage/internal/relay"
	"github.com/nuts/bridage/internal/store/postgres"
)

// ContextKey is used to pass the authenticated API key through Gin's context.
const ContextAPIKey = "api_key"

// Handler holds the dependencies for the public /v1 API.
type Handler struct {
	relay      *relay.Service
	modelStore *postgres.ModelStore
	usageStore *postgres.UsageStore
}

// NewHandler creates a new public API handler.
func NewHandler(r *relay.Service, m *postgres.ModelStore, u *postgres.UsageStore) *Handler {
	return &Handler{relay: r, modelStore: m, usageStore: u}
}

// ─── GET /v1/account/key ──────────────────────────────────────────────────────

// GetAccountKey returns non-sensitive info about the authenticated downstream API key.
func (h *Handler) GetAccountKey(c *gin.Context) {
	key := c.MustGet(ContextAPIKey).(*models.APIKey)
	c.JSON(http.StatusOK, gin.H{
		"id":             key.ID,
		"name":           key.Name,
		"status":         key.Status,
		"expires_at":     key.ExpiresAt,
		"rate_limit":     key.RateLimit,
		"quota_tokens":   key.QuotaTokens,
		"used_tokens":    key.UsedTokens,
		"quota_requests": key.QuotaReqs,
		"used_requests":  key.UsedReqs,
		"provider_id":    key.ProviderID,
		"created_at":     key.CreatedAt,
	})
}

// GetAccountUsage returns the last 50 usage log entries for the authenticated key.
func (h *Handler) GetAccountUsage(c *gin.Context) {
	key := c.MustGet(ContextAPIKey).(*models.APIKey)
	logs, err := h.usageStore.ListByKey(c.Request.Context(), key.ID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorEnvelope("internal_error", err.Error()))
		return
	}
	if logs == nil {
		logs = []*models.UsageLog{}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": logs})
}

// ─── GET /v1/models ───────────────────────────────────────────────────────────

// ListModels returns all enabled models in the OpenAI model list format.
func (h *Handler) ListModels(c *gin.Context) {
	ctx := c.Request.Context()
	list, err := h.modelStore.ListAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorEnvelope("internal_error", err.Error()))
		return
	}
	data := make([]gin.H, 0, len(list))
	for _, m := range list {
		data = append(data, gin.H{
			"id":       m.Name,
			"object":   "model",
			"created":  m.CreatedAt.Unix(),
			"owned_by": "bridage",
		})
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
}

// ─── POST /v1/chat/completions ────────────────────────────────────────────────

type chatCompletionRequest struct {
	Model       string                  `json:"model" binding:"required"`
	Messages    []providers.ChatMessage `json:"messages" binding:"required"`
	MaxTokens   int                     `json:"max_tokens"`
	Temperature *float64                `json:"temperature"`
	TopP        *float64                `json:"top_p"`
	Stream      bool                    `json:"stream"`
}

// ChatCompletions handles POST /v1/chat/completions.
func (h *Handler) ChatCompletions(c *gin.Context) {
	key := c.MustGet(ContextAPIKey).(*models.APIKey)

	var req chatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorEnvelope("invalid_request", err.Error()))
		return
	}

	provReq := &providers.ChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	if err := h.relay.CheckRateLimit(key); err != nil {
		c.JSON(http.StatusTooManyRequests, errorEnvelope("rate_limit_exceeded", err.Error()))
		return
	}
	if err := h.relay.CheckQuota(key); err != nil {
		c.JSON(http.StatusPaymentRequired, errorEnvelope("quota_exceeded", err.Error()))
		return
	}

	ctx := c.Request.Context()

	if req.Stream {
		body, prov, model, err := h.relay.ChatStream(ctx, key, provReq)
		if err != nil {
			c.JSON(relayStatusCode(err), errorEnvelope("upstream_error", err.Error()))
			return
		}
		defer body.Close()

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("X-Accel-Buffering", "no")

		start := time.Now()
		promptTokens, completionTokens := 0, 0
		flusher, canFlush := c.Writer.(http.Flusher)

		sc := bufio.NewScanner(body)
		for sc.Scan() {
			line := sc.Text()
			fmt.Fprintf(c.Writer, "%s\n", line)
			if canFlush {
				flusher.Flush()
			}
			// approximate completion token count from streaming chunks
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data != "[DONE]" {
					completionTokens++ // rough approximation; real count from usage chunk if available
					_ = data
				}
			}
		}
		h.relay.RecordStreamUsage(ctx, key, prov, model, promptTokens, completionTokens, time.Since(start))
		return
	}

	resp, err := h.relay.Chat(ctx, key, provReq)
	if err != nil {
		c.JSON(relayStatusCode(err), errorEnvelope("upstream_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── POST /v1/embeddings ──────────────────────────────────────────────────────

type embeddingsRequest struct {
	Model string   `json:"model" binding:"required"`
	Input []string `json:"input" binding:"required"`
}

// Embeddings handles POST /v1/embeddings.
func (h *Handler) Embeddings(c *gin.Context) {
	key := c.MustGet(ContextAPIKey).(*models.APIKey)

	var req embeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorEnvelope("invalid_request", err.Error()))
		return
	}

	if err := h.relay.CheckRateLimit(key); err != nil {
		c.JSON(http.StatusTooManyRequests, errorEnvelope("rate_limit_exceeded", err.Error()))
		return
	}
	if err := h.relay.CheckQuota(key); err != nil {
		c.JSON(http.StatusPaymentRequired, errorEnvelope("quota_exceeded", err.Error()))
		return
	}

	resp, err := h.relay.Embeddings(c.Request.Context(), key, &providers.EmbeddingsRequest{
		Model: req.Model,
		Input: req.Input,
	})
	if err != nil {
		c.JSON(relayStatusCode(err), errorEnvelope("upstream_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── POST /v1/images/generations ─────────────────────────────────────────────

type imagesRequest struct {
	Model  string `json:"model" binding:"required"`
	Prompt string `json:"prompt" binding:"required"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

// Images handles POST /v1/images/generations.
func (h *Handler) Images(c *gin.Context) {
	key := c.MustGet(ContextAPIKey).(*models.APIKey)

	var req imagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorEnvelope("invalid_request", err.Error()))
		return
	}

	if err := h.relay.CheckRateLimit(key); err != nil {
		c.JSON(http.StatusTooManyRequests, errorEnvelope("rate_limit_exceeded", err.Error()))
		return
	}
	if err := h.relay.CheckQuota(key); err != nil {
		c.JSON(http.StatusPaymentRequired, errorEnvelope("quota_exceeded", err.Error()))
		return
	}

	resp, err := h.relay.Images(c.Request.Context(), key, &providers.ImagesRequest{
		Model:  req.Model,
		Prompt: req.Prompt,
		N:      req.N,
		Size:   req.Size,
	})
	if err != nil {
		c.JSON(relayStatusCode(err), errorEnvelope("upstream_error", err.Error()))
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ─── Passthrough for OpenAI Responses API ─────────────────────────────────────

// Responses is a thin passthrough placeholder for /v1/responses.
func (h *Handler) Responses(c *gin.Context) {
	// Treat /v1/responses as a chat completion alias for now.
	h.ChatCompletions(c)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func errorEnvelope(code, message string) gin.H {
	return gin.H{"error": gin.H{"type": code, "message": message}}
}

func relayStatusCode(err error) int {
	switch err {
	case relay.ErrUnauthorized:
		return http.StatusUnauthorized
	case relay.ErrKeyDisabled, relay.ErrKeyExpired:
		return http.StatusForbidden
	case relay.ErrRateLimited:
		return http.StatusTooManyRequests
	case relay.ErrTokenQuotaExceeded, relay.ErrRequestQuotaExceeded:
		return http.StatusPaymentRequired
	case relay.ErrModelNotFound:
		return http.StatusNotFound
	case relay.ErrModelNotAllowed:
		return http.StatusForbidden
	default:
		return http.StatusBadGateway
	}
}

// ensure io import is used
var _ = io.EOF
