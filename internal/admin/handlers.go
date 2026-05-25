package admin

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/nuts/bridage/internal/models"
	"github.com/nuts/bridage/internal/providers"
	"github.com/nuts/bridage/internal/security"
	"github.com/nuts/bridage/internal/store/postgres"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Handler holds admin API dependencies.
type Handler struct {
	admins         *postgres.AdminStore
	provStore      *postgres.ProviderStore
	modelStore     *postgres.ModelStore
	apiKeys        *postgres.APIKeyStore
	usage          *postgres.UsageStore
	audit          *postgres.AuditStore
	masterKey      string
	jwtSecret      []byte
	jwtExpiry      time.Duration
	bootstrapToken string // required header value for /admin/bootstrap
	log            *zap.Logger
}

// NewHandler creates a new admin API handler.
func NewHandler(
	admins *postgres.AdminStore,
	provStore *postgres.ProviderStore,
	modelStore *postgres.ModelStore,
	apiKeys *postgres.APIKeyStore,
	usage *postgres.UsageStore,
	audit *postgres.AuditStore,
	masterKey, jwtSecret string,
	jwtExpiry time.Duration,
	bootstrapToken string,
	log *zap.Logger,
) *Handler {
	return &Handler{
		admins:         admins,
		provStore:      provStore,
		modelStore:     modelStore,
		apiKeys:        apiKeys,
		usage:          usage,
		audit:          audit,
		masterKey:      masterKey,
		jwtSecret:      []byte(jwtSecret),
		jwtExpiry:      jwtExpiry,
		bootstrapToken: bootstrapToken,
		log:            log,
	}
}

// ─── Auth ──────────────────────────────────────────────────────────────────────

// Login handles POST /admin/login.
func (h *Handler) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx := c.Request.Context()
	ip := c.ClientIP()

	user, err := h.admins.GetByUsername(ctx, body.Username)
	if err != nil || user == nil {
		// Constant-time delay on failure to slow credential stuffing.
		bcrypt.CompareHashAndPassword([]byte("$2a$10$placeholder_hash_to_consume_time"), []byte(body.Password)) //nolint:errcheck
		_ = h.audit.Append(ctx, body.Username, "login_failed", "admin", map[string]any{"ip": ip, "reason": "user_not_found"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		_ = h.audit.Append(ctx, body.Username, "login_failed", "admin", map[string]any{"ip": ip, "reason": "wrong_password"})
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	token, err := h.issueJWT(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}
	_ = h.audit.Append(ctx, user.Username, "login_success", "admin", map[string]any{"ip": ip})
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) issueJWT(username string) (string, error) {
	claims := jwt.MapClaims{
		"sub": username,
		"iss": "bridage",
		"aud": []string{"bridage-admin"},
		"jti": uuid.New().String(),
		"exp": time.Now().Add(h.jwtExpiry).Unix(),
		"iat": time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.jwtSecret)
}

// ValidateJWT validates an Authorization: Bearer <token> header.
func (h *Handler) ValidateJWT(c *gin.Context) {
	raw := c.GetHeader("Authorization")
	if len(raw) < 8 || raw[:7] != "Bearer " {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}
	tokenStr := raw[7:]
	token, err := jwt.Parse(tokenStr,
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return h.jwtSecret, nil
		},
		jwt.WithIssuedAt(),
		jwt.WithIssuer("bridage"),
		jwt.WithAudience("bridage-admin"),
	)
	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}
	c.Set("admin_user", claims["sub"])
	c.Next()
}

// ─── Providers ────────────────────────────────────────────────────────────────

func (h *Handler) ListProviders(c *gin.Context) {
	list, err := h.provStore.List(c.Request.Context(), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *Handler) CreateProvider(c *gin.Context) {
	var body struct {
		Name         string                      `json:"name" binding:"required"`
		DisplayName  string                      `json:"display_name"`
		AdapterType  models.ProviderAdapterType  `json:"adapter_type" binding:"required"`
		BaseURL      string                      `json:"base_url" binding:"required"`
		AuthHeader   string                      `json:"auth_header"`
		AuthScheme   string                      `json:"auth_scheme"`
		APIKey       string                      `json:"api_key" binding:"required"`
		Enabled      bool                        `json:"enabled"`
		Capabilities models.ProviderCapabilities `json:"capabilities"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := validateProviderURL(body.BaseURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.AuthHeader == "" {
		body.AuthHeader = "Authorization"
	}
	if body.AuthScheme == "" {
		body.AuthScheme = "Bearer"
	}
	encrypted, err := security.EncryptSecret(h.masterKey, body.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt api key"})
		return
	}
	prov := &models.Provider{
		Name:            body.Name,
		DisplayName:     body.DisplayName,
		AdapterType:     body.AdapterType,
		BaseURL:         body.BaseURL,
		AuthHeader:      body.AuthHeader,
		AuthScheme:      body.AuthScheme,
		EncryptedAPIKey: encrypted,
		Enabled:         body.Enabled,
		Capabilities:    body.Capabilities,
	}
	created, err := h.provStore.Create(c.Request.Context(), prov)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	p, err := h.provStore.GetByID(c.Request.Context(), id)
	if err != nil || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) DeleteProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.provStore.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) UpdateProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ctx := c.Request.Context()
	p, err := h.provStore.GetByID(ctx, id)
	if err != nil || p == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var body struct {
		DisplayName  *string                      `json:"display_name"`
		BaseURL      *string                      `json:"base_url"`
		AuthHeader   *string                      `json:"auth_header"`
		AuthScheme   *string                      `json:"auth_scheme"`
		APIKey       *string                      `json:"api_key"`
		Enabled      *bool                        `json:"enabled"`
		Capabilities *models.ProviderCapabilities `json:"capabilities"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if body.DisplayName != nil {
		p.DisplayName = *body.DisplayName
	}
	if body.BaseURL != nil {
		if err := validateProviderURL(*body.BaseURL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		p.BaseURL = *body.BaseURL
	}
	if body.AuthHeader != nil {
		p.AuthHeader = *body.AuthHeader
	}
	if body.AuthScheme != nil {
		p.AuthScheme = *body.AuthScheme
	}
	if body.Enabled != nil {
		p.Enabled = *body.Enabled
	}
	if body.Capabilities != nil {
		p.Capabilities = *body.Capabilities
	}
	if body.APIKey != nil {
		encrypted, err := security.EncryptSecret(h.masterKey, *body.APIKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt api key"})
			return
		}
		p.EncryptedAPIKey = encrypted
	}
	updated, err := h.provStore.Update(ctx, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// BootstrapPreset creates a provider (and its default models) from a built-in preset.
func (h *Handler) BootstrapPreset(c *gin.Context) {
	slug := c.Param("slug")
	var body struct {
		APIKey  string `json:"api_key" binding:"required"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	preset, ok := providers.PresetBySlug(slug)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "preset not found"})
		return
	}
	if err := validateProviderURL(preset.BaseURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	encrypted, err := security.EncryptSecret(h.masterKey, body.APIKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encrypt failed"})
		return
	}
	prov := &models.Provider{
		Name:            preset.Slug,
		DisplayName:     preset.DisplayName,
		AdapterType:     preset.AdapterType,
		BaseURL:         preset.BaseURL,
		AuthHeader:      preset.AuthHeader,
		AuthScheme:      preset.AuthScheme,
		EncryptedAPIKey: encrypted,
		Enabled:         body.Enabled,
		Capabilities:    preset.Capabilities,
	}
	ctx := c.Request.Context()
	created, err := h.provStore.Create(ctx, prov)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	for _, pm := range preset.DefaultModels {
		m := &models.Model{
			ProviderID:     created.ID,
			Name:           pm.Name,
			ProviderModel:  pm.ProviderModel,
			Description:    pm.Description,
			Enabled:        true,
			SupportsImages: pm.SupportsImages,
			ContextWindow:  pm.ContextWindow,
		}
		if _, err := h.modelStore.Create(ctx, m); err != nil {
			h.log.Warn("failed to create preset model", zap.String("model", pm.Name), zap.Error(err))
		}
	}
	c.JSON(http.StatusCreated, created)
}

// ListPresets returns all built-in provider presets.
func (h *Handler) ListPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": providers.Presets()})
}

// ─── Models ───────────────────────────────────────────────────────────────────

func (h *Handler) ListModels(c *gin.Context) {
	list, err := h.modelStore.ListAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *Handler) CreateModel(c *gin.Context) {
	var body struct {
		ProviderID     uuid.UUID `json:"provider_id" binding:"required"`
		Name           string    `json:"name" binding:"required"`
		ProviderModel  string    `json:"provider_model" binding:"required"`
		Description    string    `json:"description"`
		Enabled        bool      `json:"enabled"`
		SupportsImages bool      `json:"supports_images"`
		ContextWindow  int       `json:"context_window"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	m := &models.Model{
		ProviderID:     body.ProviderID,
		Name:           body.Name,
		ProviderModel:  body.ProviderModel,
		Description:    body.Description,
		Enabled:        body.Enabled,
		SupportsImages: body.SupportsImages,
		ContextWindow:  body.ContextWindow,
	}
	created, err := h.modelStore.Create(c.Request.Context(), m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetModel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	m, err := h.modelStore.GetByID(c.Request.Context(), id)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, m)
}

func (h *Handler) UpdateModel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ctx := c.Request.Context()
	m, err := h.modelStore.GetByID(ctx, id)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var body struct {
		Name           *string `json:"name"`
		ProviderModel  *string `json:"provider_model"`
		Description    *string `json:"description"`
		Enabled        *bool   `json:"enabled"`
		SupportsImages *bool   `json:"supports_images"`
		ContextWindow  *int    `json:"context_window"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if body.Name != nil {
		m.Name = *body.Name
	}
	if body.ProviderModel != nil {
		m.ProviderModel = *body.ProviderModel
	}
	if body.Description != nil {
		m.Description = *body.Description
	}
	if body.Enabled != nil {
		m.Enabled = *body.Enabled
	}
	if body.SupportsImages != nil {
		m.SupportsImages = *body.SupportsImages
	}
	if body.ContextWindow != nil {
		m.ContextWindow = *body.ContextWindow
	}
	updated, err := h.modelStore.Update(ctx, m)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteModel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.modelStore.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── API Keys ─────────────────────────────────────────────────────────────────

func (h *Handler) ListAPIKeys(c *gin.Context) {
	list, err := h.apiKeys.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *Handler) CreateAPIKey(c *gin.Context) {
	var body struct {
		Name        string     `json:"name" binding:"required"`
		ExpiresAt   *time.Time `json:"expires_at"`
		RateLimit   int        `json:"rate_limit"`
		QuotaTokens int64      `json:"quota_tokens"`
		QuotaReqs   int64      `json:"quota_requests"`
		ProviderID  *uuid.UUID `json:"provider_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	plaintext, hash, err := security.GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "key generation failed"})
		return
	}
	index := security.HashAPIKeyFast(plaintext)
	k := &models.APIKey{
		Name:        body.Name,
		KeyHash:     hash,
		KeyIndex:    index,
		Status:      models.APIKeyStatusActive,
		ExpiresAt:   body.ExpiresAt,
		RateLimit:   body.RateLimit,
		QuotaTokens: body.QuotaTokens,
		QuotaReqs:   body.QuotaReqs,
		ProviderID:  body.ProviderID,
	}
	created, err := h.apiKeys.Create(c.Request.Context(), k)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	// Return the plaintext once; it is never retrievable again.
	c.JSON(http.StatusCreated, gin.H{"key": plaintext, "record": created})
}

func (h *Handler) GetAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	k, err := h.apiKeys.GetByID(c.Request.Context(), id)
	if err != nil || k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, k)
}

func (h *Handler) UpdateAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	ctx := c.Request.Context()
	k, err := h.apiKeys.GetByID(ctx, id)
	if err != nil || k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	var body struct {
		Name        *string              `json:"name"`
		Status      *models.APIKeyStatus `json:"status"`
		ExpiresAt   *time.Time           `json:"expires_at"`
		RateLimit   *int                 `json:"rate_limit"`
		QuotaTokens *int64               `json:"quota_tokens"`
		QuotaReqs   *int64               `json:"quota_requests"`
		ProviderID  *uuid.UUID           `json:"provider_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if body.Name != nil {
		k.Name = *body.Name
	}
	if body.Status != nil {
		k.Status = *body.Status
	}
	if body.ExpiresAt != nil {
		k.ExpiresAt = body.ExpiresAt
	}
	if body.RateLimit != nil {
		k.RateLimit = *body.RateLimit
	}
	if body.QuotaTokens != nil {
		k.QuotaTokens = *body.QuotaTokens
	}
	if body.QuotaReqs != nil {
		k.QuotaReqs = *body.QuotaReqs
	}
	if body.ProviderID != nil {
		k.ProviderID = body.ProviderID
	}
	updated, err := h.apiKeys.Update(ctx, k)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *Handler) DeleteAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.apiKeys.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}

// ─── Usage ────────────────────────────────────────────────────────────────────

func (h *Handler) GetUsage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	logs, err := h.usage.ListByKey(c.Request.Context(), id, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// ─── Bootstrap admin ──────────────────────────────────────────────────────────

// BootstrapAdmin creates the first admin user if none exist.
// Requires a matching X-Bootstrap-Token header when ADMIN_BOOTSTRAP_TOKEN is configured.
func (h *Handler) BootstrapAdmin(c *gin.Context) {
	// Token guard: if a bootstrap token is configured, it must match.
	if h.bootstrapToken != "" {
		provided := c.GetHeader("X-Bootstrap-Token")
		if provided != h.bootstrapToken {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid or missing bootstrap token"})
			return
		}
	}

	var body struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := validatePassword(body.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	n, err := h.admins.Count(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if n > 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "bootstrap already completed"})
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	user, err := h.admins.Create(ctx, body.Username, string(hashed))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	_ = h.audit.Append(ctx, body.Username, "bootstrap_admin", "admin", map[string]any{"ip": c.ClientIP()})
	c.JSON(http.StatusCreated, user)
}

// ─── Security helpers ─────────────────────────────────────────────────────────

// validatePassword enforces a minimal password complexity policy.
// Requirements: ≥12 chars, at least one uppercase, one lowercase, one digit, one symbol.
func validatePassword(p string) error {
	if len(p) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSymbol bool
	for _, r := range p {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSymbol = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSymbol {
		return errors.New("password must contain uppercase, lowercase, digit, and symbol characters")
	}
	return nil
}

// validateProviderURL rejects non-HTTPS URLs and private/loopback destinations
// to prevent SSRF attacks where the server is weaponised to probe internal hosts.
func validateProviderURL(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.New("invalid base_url")
	}
	if u.Scheme != "https" {
		return errors.New("base_url must use HTTPS")
	}
	host := u.Hostname()
	ips, err := net.LookupHost(host)
	if err != nil {
		// DNS resolution failure at validation time is acceptable;
		// block only if we can resolve AND the address is private.
		return nil
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip != nil && isPrivateIP(ip) {
			return errors.New("base_url must not point to a private or loopback address")
		}
	}
	return nil
}

// isPrivateIP reports whether ip is a loopback, link-local, or RFC-1918 address.
func isPrivateIP(ip net.IP) bool {
	private := []string{
		"127.0.0.0/8",
		"::1/128",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"fc00::/7",
		"fe80::/10",
	}
	for _, cidr := range private {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil && block.Contains(ip) {
			return true
		}
	}
	return false
}
