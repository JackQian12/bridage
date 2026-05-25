package relay

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nuts/bridage/internal/models"
	"github.com/nuts/bridage/internal/providers"
	"github.com/nuts/bridage/internal/security"
	"github.com/nuts/bridage/internal/store/postgres"
	"go.uber.org/zap"
)

// ─── Rate limiter (sliding window, in-memory) ─────────────────────────────────

type rateBucket struct {
	mu         sync.Mutex
	timestamps []time.Time
}

func (b *rateBucket) allow(rpmLimit int) bool {
	if rpmLimit == 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)
	// prune old entries
	j := 0
	for _, t := range b.timestamps {
		if t.After(cutoff) {
			b.timestamps[j] = t
			j++
		}
	}
	b.timestamps = b.timestamps[:j]
	if len(b.timestamps) >= rpmLimit {
		return false
	}
	b.timestamps = append(b.timestamps, now)
	return true
}

// ─── Service ──────────────────────────────────────────────────────────────────

// Service is the relay orchestration core.
type Service struct {
	apiKeys   *postgres.APIKeyStore
	providers *postgres.ProviderStore
	models    *postgres.ModelStore
	usage     *postgres.UsageStore
	masterKey string
	timeout   time.Duration
	retries   int
	log       *zap.Logger

	rateBuckets sync.Map // map[uuid.UUID]*rateBucket
}

// NewService builds a relay Service.
func NewService(
	apiKeys *postgres.APIKeyStore,
	provStore *postgres.ProviderStore,
	modelStore *postgres.ModelStore,
	usageStore *postgres.UsageStore,
	masterKey string,
	timeout time.Duration,
	retries int,
	log *zap.Logger,
) *Service {
	return &Service{
		apiKeys:   apiKeys,
		providers: provStore,
		models:    modelStore,
		usage:     usageStore,
		masterKey: masterKey,
		timeout:   timeout,
		retries:   retries,
		log:       log,
	}
}

// AuthResult is returned by Authenticate.
type AuthResult struct {
	Key      *models.APIKey
	Provider *models.Provider
}

// Authenticate validates a raw downstream API key, returning the key record
// and the (possibly overridden) provider to use.
func (s *Service) Authenticate(ctx context.Context, rawKey string) (*AuthResult, error) {
	index := security.HashAPIKeyFast(rawKey)
	key, err := s.apiKeys.GetByIndex(ctx, index)
	if err != nil {
		return nil, fmt.Errorf("db lookup: %w", err)
	}
	if key == nil {
		return nil, ErrUnauthorized
	}
	if !security.VerifyAPIKey(rawKey, key.KeyHash) {
		return nil, ErrUnauthorized
	}
	if key.Status != models.APIKeyStatusActive {
		return nil, ErrKeyDisabled
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, ErrKeyExpired
	}
	return &AuthResult{Key: key}, nil
}

// CheckRateLimit returns an error if the key has exceeded its RPM limit.
func (s *Service) CheckRateLimit(key *models.APIKey) error {
	if key.RateLimit == 0 {
		return nil
	}
	v, _ := s.rateBuckets.LoadOrStore(key.ID, &rateBucket{})
	bucket := v.(*rateBucket)
	if !bucket.allow(key.RateLimit) {
		return ErrRateLimited
	}
	return nil
}

// CheckQuota returns an error if the key has exhausted its token or request quota.
func (s *Service) CheckQuota(key *models.APIKey) error {
	if key.QuotaTokens > 0 && key.UsedTokens >= key.QuotaTokens {
		return ErrTokenQuotaExceeded
	}
	if key.QuotaReqs > 0 && key.UsedReqs >= key.QuotaReqs {
		return ErrRequestQuotaExceeded
	}
	return nil
}

// ResolveModel finds the model record and checks key model rules.
func (s *Service) ResolveModel(ctx context.Context, key *models.APIKey, modelName string) (*models.Model, error) {
	// Find an enabled model with this canonical name across all enabled providers.
	allModels, err := s.models.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	var found *models.Model
	for _, m := range allModels {
		if m.Name == modelName {
			found = m
			break
		}
	}
	if found == nil {
		return nil, ErrModelNotFound
	}

	// If the key has a provider override, enforce it.
	if key.ProviderID != nil && found.ProviderID != *key.ProviderID {
		return nil, ErrModelNotFound
	}

	// Check model rules.
	rules, err := s.apiKeys.GetModelRules(ctx, key.ID)
	if err != nil {
		return nil, fmt.Errorf("get model rules: %w", err)
	}
	if len(rules) > 0 {
		allowed := false
		denied := false
		for _, r := range rules {
			if r.ModelID == found.ID {
				if r.Allow {
					allowed = true
				} else {
					denied = true
				}
			}
		}
		if denied {
			return nil, ErrModelNotAllowed
		}
		// If any allowlist rules exist and this model is not in them, deny.
		hasAllowRules := false
		for _, r := range rules {
			if r.Allow {
				hasAllowRules = true
				break
			}
		}
		if hasAllowRules && !allowed {
			return nil, ErrModelNotAllowed
		}
	}
	return found, nil
}

// GetAdapter returns the appropriate provider adapter for a model.
func (s *Service) GetAdapter(ctx context.Context, providerID uuid.UUID) (providers.Adapter, *models.Provider, error) {
	prov, err := s.providers.GetByID(ctx, providerID)
	if err != nil {
		return nil, nil, fmt.Errorf("get provider: %w", err)
	}
	if prov == nil || !prov.Enabled {
		return nil, nil, fmt.Errorf("provider not available")
	}

	plainKey, err := security.DecryptSecret(s.masterKey, prov.EncryptedAPIKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt provider key: %w", err)
	}

	var adapter providers.Adapter
	switch prov.AdapterType {
	case models.AdapterOpenAICompatible:
		adapter = providers.NewOpenAICompatAdapter(prov.BaseURL, prov.AuthHeader, prov.AuthScheme, plainKey, s.timeout)
	case models.AdapterAnthropic:
		adapter = providers.NewAnthropicAdapter(prov.BaseURL, plainKey, s.timeout)
	case models.AdapterGemini:
		adapter = providers.NewGeminiAdapter(prov.BaseURL, plainKey, s.timeout)
	default:
		return nil, nil, fmt.Errorf("unknown adapter type: %s", prov.AdapterType)
	}
	return adapter, prov, nil
}

// Chat executes a non-streaming chat completion end to end.
func (s *Service) Chat(ctx context.Context, key *models.APIKey, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	model, err := s.ResolveModel(ctx, key, req.Model)
	if err != nil {
		return nil, err
	}
	req.UpstreamModel = model.ProviderModel

	adapter, prov, err := s.GetAdapter(ctx, model.ProviderID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, callErr := adapter.Chat(ctx, req)
	elapsed := time.Since(start)

	statusCode := 200
	errMsg := ""
	if callErr != nil {
		statusCode = 502
		errMsg = callErr.Error()
	}

	tokens := 0
	if resp != nil {
		tokens = resp.Usage.TotalTokens
	}

	s.recordUsage(ctx, key, prov, model, "/v1/chat/completions", 0, 0, tokens, elapsed, statusCode, errMsg)
	if callErr != nil {
		return nil, callErr
	}
	s.incrementQuota(ctx, key.ID, tokens, 1)
	return resp, nil
}

// ChatStream opens a streaming chat completion, returning the raw SSE body.
// Usage is recorded asynchronously from approximate token count.
func (s *Service) ChatStream(ctx context.Context, key *models.APIKey, req *providers.ChatRequest) (io.ReadCloser, *models.Provider, *models.Model, error) {
	model, err := s.ResolveModel(ctx, key, req.Model)
	if err != nil {
		return nil, nil, nil, err
	}
	req.UpstreamModel = model.ProviderModel

	adapter, prov, err := s.GetAdapter(ctx, model.ProviderID)
	if err != nil {
		return nil, nil, nil, err
	}

	body, err := adapter.ChatStream(ctx, req)
	if err != nil {
		s.recordUsage(ctx, key, prov, model, "/v1/chat/completions", 0, 0, 0, 0, 502, err.Error())
		return nil, nil, nil, err
	}
	return body, prov, model, nil
}

// RecordStreamUsage writes a usage log entry after a streaming call completes.
func (s *Service) RecordStreamUsage(ctx context.Context, key *models.APIKey, prov *models.Provider, model *models.Model, promptTokens, completionTokens int, elapsed time.Duration) {
	total := promptTokens + completionTokens
	s.recordUsage(ctx, key, prov, model, "/v1/chat/completions", promptTokens, completionTokens, total, elapsed, 200, "")
	s.incrementQuota(ctx, key.ID, total, 1)
}

// Embeddings executes an embeddings call.
func (s *Service) Embeddings(ctx context.Context, key *models.APIKey, req *providers.EmbeddingsRequest) (*providers.EmbeddingsResponse, error) {
	model, err := s.ResolveModel(ctx, key, req.Model)
	if err != nil {
		return nil, err
	}
	req.UpstreamModel = model.ProviderModel

	adapter, prov, err := s.GetAdapter(ctx, model.ProviderID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, callErr := adapter.Embeddings(ctx, req)
	elapsed := time.Since(start)

	statusCode := 200
	errMsg := ""
	if callErr != nil {
		statusCode = 502
		errMsg = callErr.Error()
	}
	tokens := 0
	if resp != nil {
		tokens = resp.Usage.TotalTokens
	}
	s.recordUsage(ctx, key, prov, model, "/v1/embeddings", 0, 0, tokens, elapsed, statusCode, errMsg)
	if callErr != nil {
		return nil, callErr
	}
	s.incrementQuota(ctx, key.ID, tokens, 1)
	return resp, nil
}

// Images executes an image generation call.
func (s *Service) Images(ctx context.Context, key *models.APIKey, req *providers.ImagesRequest) (*providers.ImagesResponse, error) {
	model, err := s.ResolveModel(ctx, key, req.Model)
	if err != nil {
		return nil, err
	}
	req.UpstreamModel = model.ProviderModel

	adapter, prov, err := s.GetAdapter(ctx, model.ProviderID)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, callErr := adapter.Images(ctx, req)
	elapsed := time.Since(start)

	statusCode := 200
	errMsg := ""
	if callErr != nil {
		statusCode = 502
		errMsg = callErr.Error()
	}
	s.recordUsage(ctx, key, prov, model, "/v1/images/generations", 0, 0, 0, elapsed, statusCode, errMsg)
	if callErr != nil {
		return nil, callErr
	}
	s.incrementQuota(ctx, key.ID, 0, 1)
	return resp, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (s *Service) recordUsage(ctx context.Context, key *models.APIKey, prov *models.Provider, model *models.Model, endpoint string, prompt, completion, total int, elapsed time.Duration, status int, errMsg string) {
	u := &models.UsageLog{
		APIKeyID:         key.ID,
		ProviderID:       prov.ID,
		ModelID:          model.ID,
		Endpoint:         endpoint,
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		DurationMs:       elapsed.Milliseconds(),
		StatusCode:       status,
		Error:            errMsg,
	}
	if err := s.usage.Append(ctx, u); err != nil {
		s.log.Warn("failed to write usage log", zap.Error(err))
	}
}

func (s *Service) incrementQuota(ctx context.Context, keyID uuid.UUID, tokens, reqs int) {
	if err := s.apiKeys.IncrementUsage(ctx, keyID, tokens, reqs); err != nil {
		s.log.Warn("failed to increment key usage", zap.Error(err))
	}
}

// ─── Sentinel errors ──────────────────────────────────────────────────────────

var (
	ErrUnauthorized         = fmt.Errorf("unauthorized")
	ErrKeyDisabled          = fmt.Errorf("api key is disabled")
	ErrKeyExpired           = fmt.Errorf("api key has expired")
	ErrRateLimited          = fmt.Errorf("rate limit exceeded")
	ErrTokenQuotaExceeded   = fmt.Errorf("token quota exceeded")
	ErrRequestQuotaExceeded = fmt.Errorf("request quota exceeded")
	ErrModelNotFound        = fmt.Errorf("model not found")
	ErrModelNotAllowed      = fmt.Errorf("model not allowed for this api key")
)
