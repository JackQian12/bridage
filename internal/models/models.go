package models

import (
	"time"

	"github.com/google/uuid"
)

// ProviderAdapterType identifies the request/response protocol used by a provider.
type ProviderAdapterType string

const (
	AdapterOpenAICompatible ProviderAdapterType = "openai_compatible"
	AdapterAnthropic        ProviderAdapterType = "anthropic"
	AdapterGemini           ProviderAdapterType = "gemini"
)

// Provider represents an upstream LLM service configuration.
type Provider struct {
	ID              uuid.UUID            `json:"id"`
	Name            string               `json:"name"`
	DisplayName     string               `json:"display_name"`
	AdapterType     ProviderAdapterType  `json:"adapter_type"`
	BaseURL         string               `json:"base_url"`
	AuthHeader      string               `json:"auth_header"` // e.g. "Authorization" or "x-api-key"
	AuthScheme      string               `json:"auth_scheme"` // e.g. "Bearer" or ""
	EncryptedAPIKey string               `json:"-"`           // AES-GCM encrypted; never in JSON output
	Enabled         bool                 `json:"enabled"`
	Capabilities    ProviderCapabilities `json:"capabilities"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// ProviderCapabilities flags what a provider supports.
type ProviderCapabilities struct {
	Chat       bool `json:"chat"`
	Responses  bool `json:"responses"`
	Embeddings bool `json:"embeddings"`
	Images     bool `json:"images"`
	Streaming  bool `json:"streaming"`
}

// Model is an entry in the model catalog.
type Model struct {
	ID             uuid.UUID `json:"id"`
	ProviderID     uuid.UUID `json:"provider_id"`
	Name           string    `json:"name"`           // canonical name shown to downstream users
	ProviderModel  string    `json:"provider_model"` // actual model name sent to upstream
	Description    string    `json:"description"`
	Enabled        bool      `json:"enabled"`
	SupportsImages bool      `json:"supports_images"`
	ContextWindow  int       `json:"context_window"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// APIKeyStatus represents the lifecycle state of a downstream API key.
type APIKeyStatus string

const (
	APIKeyStatusActive   APIKeyStatus = "active"
	APIKeyStatusDisabled APIKeyStatus = "disabled"
	APIKeyStatusExpired  APIKeyStatus = "expired"
)

// APIKey is a downstream API key issued to an end user / third party.
type APIKey struct {
	ID          uuid.UUID    `json:"id"`
	Name        string       `json:"name"`
	KeyHash     string       `json:"-"` // bcrypt hash
	KeyIndex    string       `json:"-"` // SHA-256 fast lookup index
	Status      APIKeyStatus `json:"status"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	RateLimit   int          `json:"rate_limit"`   // requests per minute; 0 = unlimited
	QuotaTokens int64        `json:"quota_tokens"` // total token budget; 0 = unlimited
	UsedTokens  int64        `json:"used_tokens"`
	QuotaReqs   int64        `json:"quota_requests"` // total request budget; 0 = unlimited
	UsedReqs    int64        `json:"used_requests"`
	ProviderID  *uuid.UUID   `json:"provider_id,omitempty"` // nil = key may use any enabled provider
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// APIKeyModelRule controls whether a downstream key may use a specific model.
type APIKeyModelRule struct {
	ID       uuid.UUID `json:"id"`
	APIKeyID uuid.UUID `json:"api_key_id"`
	ModelID  uuid.UUID `json:"model_id"`
	Allow    bool      `json:"allow"` // true = allowlist entry; false = denylist entry
}

// UsageLog records a single completed relay call.
type UsageLog struct {
	ID               uuid.UUID `json:"id"`
	APIKeyID         uuid.UUID `json:"api_key_id"`
	ProviderID       uuid.UUID `json:"provider_id"`
	ModelID          uuid.UUID `json:"model_id"`
	Endpoint         string    `json:"endpoint"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	DurationMs       int64     `json:"duration_ms"`
	StatusCode       int       `json:"status_code"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// AdminUser is a server administrator with access to the management API.
type AdminUser struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProviderPreset is a built-in template for quickly adding a well-known provider.
type ProviderPreset struct {
	Slug          string               `json:"slug"`
	DisplayName   string               `json:"display_name"`
	AdapterType   ProviderAdapterType  `json:"adapter_type"`
	BaseURL       string               `json:"base_url"`
	AuthHeader    string               `json:"auth_header"`
	AuthScheme    string               `json:"auth_scheme"`
	Capabilities  ProviderCapabilities `json:"capabilities"`
	DefaultModels []PresetModel        `json:"default_models"`
}

// PresetModel is a model included with a ProviderPreset.
type PresetModel struct {
	Name           string `json:"name"`
	ProviderModel  string `json:"provider_model"`
	Description    string `json:"description"`
	ContextWindow  int    `json:"context_window"`
	SupportsImages bool   `json:"supports_images"`
}
