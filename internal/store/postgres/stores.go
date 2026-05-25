package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nuts/bridage/internal/models"
)

// ProviderStore handles persistence for upstream providers.
type ProviderStore struct {
	db *pgxpool.Pool
}

// NewProviderStore creates a ProviderStore backed by the given pool.
func NewProviderStore(db *pgxpool.Pool) *ProviderStore {
	return &ProviderStore{db: db}
}

const providerCols = `id, name, display_name, adapter_type, base_url, auth_header, auth_scheme,
    encrypted_api_key, enabled, cap_chat, cap_responses, cap_embeddings, cap_images, cap_streaming,
    created_at, updated_at`

func scanProvider(row pgx.Row) (*models.Provider, error) {
	p := &models.Provider{}
	err := row.Scan(
		&p.ID, &p.Name, &p.DisplayName, &p.AdapterType, &p.BaseURL,
		&p.AuthHeader, &p.AuthScheme, &p.EncryptedAPIKey, &p.Enabled,
		&p.Capabilities.Chat, &p.Capabilities.Responses, &p.Capabilities.Embeddings,
		&p.Capabilities.Images, &p.Capabilities.Streaming,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// Create inserts a new provider and returns it with the generated ID and timestamps.
func (s *ProviderStore) Create(ctx context.Context, p *models.Provider) (*models.Provider, error) {
	const q = `INSERT INTO providers (name, display_name, adapter_type, base_url, auth_header, auth_scheme,
        encrypted_api_key, enabled, cap_chat, cap_responses, cap_embeddings, cap_images, cap_streaming)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
        RETURNING ` + providerCols

	row := s.db.QueryRow(ctx, q,
		p.Name, p.DisplayName, p.AdapterType, p.BaseURL, p.AuthHeader, p.AuthScheme,
		p.EncryptedAPIKey, p.Enabled,
		p.Capabilities.Chat, p.Capabilities.Responses, p.Capabilities.Embeddings,
		p.Capabilities.Images, p.Capabilities.Streaming,
	)
	return scanProvider(row)
}

// GetByID fetches a provider by its UUID.
func (s *ProviderStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	row := s.db.QueryRow(ctx, `SELECT `+providerCols+` FROM providers WHERE id=$1`, id)
	p, err := scanProvider(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return p, err
}

// List returns all providers, optionally filtered to enabled-only.
func (s *ProviderStore) List(ctx context.Context, enabledOnly bool) ([]*models.Provider, error) {
	q := `SELECT ` + providerCols + ` FROM providers`
	if enabledOnly {
		q += ` WHERE enabled = TRUE`
	}
	q += ` ORDER BY display_name`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()
	var providers []*models.Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// Update overwrites mutable fields of an existing provider.
func (s *ProviderStore) Update(ctx context.Context, p *models.Provider) (*models.Provider, error) {
	const q = `UPDATE providers SET display_name=$1, base_url=$2, auth_header=$3, auth_scheme=$4,
        encrypted_api_key=$5, enabled=$6, cap_chat=$7, cap_responses=$8, cap_embeddings=$9,
        cap_images=$10, cap_streaming=$11
        WHERE id=$12
        RETURNING ` + providerCols

	row := s.db.QueryRow(ctx, q,
		p.DisplayName, p.BaseURL, p.AuthHeader, p.AuthScheme, p.EncryptedAPIKey, p.Enabled,
		p.Capabilities.Chat, p.Capabilities.Responses, p.Capabilities.Embeddings,
		p.Capabilities.Images, p.Capabilities.Streaming,
		p.ID,
	)
	return scanProvider(row)
}

// Delete removes a provider by ID.
func (s *ProviderStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM providers WHERE id=$1`, id)
	return err
}

// ─── Model store ──────────────────────────────────────────────────────────────

// ModelStore handles persistence for model catalog entries.
type ModelStore struct {
	db *pgxpool.Pool
}

// NewModelStore creates a ModelStore backed by the given pool.
func NewModelStore(db *pgxpool.Pool) *ModelStore {
	return &ModelStore{db: db}
}

const modelCols = `id, provider_id, name, provider_model, description, enabled, supports_images, context_window, created_at, updated_at`

func scanModel(row pgx.Row) (*models.Model, error) {
	m := &models.Model{}
	err := row.Scan(&m.ID, &m.ProviderID, &m.Name, &m.ProviderModel, &m.Description,
		&m.Enabled, &m.SupportsImages, &m.ContextWindow, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Create inserts a new model.
func (s *ModelStore) Create(ctx context.Context, m *models.Model) (*models.Model, error) {
	const q = `INSERT INTO models (provider_id, name, provider_model, description, enabled, supports_images, context_window)
        VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING ` + modelCols
	row := s.db.QueryRow(ctx, q, m.ProviderID, m.Name, m.ProviderModel, m.Description, m.Enabled, m.SupportsImages, m.ContextWindow)
	return scanModel(row)
}

// GetByID fetches a model by UUID.
func (s *ModelStore) GetByID(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	row := s.db.QueryRow(ctx, `SELECT `+modelCols+` FROM models WHERE id=$1`, id)
	m, err := scanModel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// GetByName looks up a model by its canonical name within a provider.
func (s *ModelStore) GetByName(ctx context.Context, providerID uuid.UUID, name string) (*models.Model, error) {
	row := s.db.QueryRow(ctx, `SELECT `+modelCols+` FROM models WHERE provider_id=$1 AND name=$2 AND enabled=TRUE`, providerID, name)
	m, err := scanModel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return m, err
}

// ListByProvider returns all models for a given provider.
func (s *ModelStore) ListByProvider(ctx context.Context, providerID uuid.UUID, enabledOnly bool) ([]*models.Model, error) {
	q := `SELECT ` + modelCols + ` FROM models WHERE provider_id=$1`
	if enabledOnly {
		q += ` AND enabled=TRUE`
	}
	q += ` ORDER BY name`
	rows, err := s.db.Query(ctx, q, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Model
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// Update overwrites mutable fields of an existing model.
func (s *ModelStore) Update(ctx context.Context, m *models.Model) (*models.Model, error) {
	const q = `UPDATE models SET name=$1, provider_model=$2, description=$3,
        enabled=$4, supports_images=$5, context_window=$6
        WHERE id=$7
        RETURNING ` + modelCols
	row := s.db.QueryRow(ctx, q,
		m.Name, m.ProviderModel, m.Description, m.Enabled, m.SupportsImages, m.ContextWindow, m.ID)
	return scanModel(row)
}

// Delete removes a model by ID.
func (s *ModelStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM models WHERE id=$1`, id)
	return err
}

// ListAll returns all enabled models across all enabled providers.
func (s *ModelStore) ListAll(ctx context.Context) ([]*models.Model, error) {
	const q = `SELECT m.id, m.provider_id, m.name, m.provider_model, m.description, m.enabled, m.supports_images, m.context_window, m.created_at, m.updated_at
        FROM models m
        JOIN providers p ON p.id = m.provider_id
        WHERE m.enabled=TRUE AND p.enabled=TRUE ORDER BY m.name`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Model
	for rows.Next() {
		m, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// ─── APIKey store ─────────────────────────────────────────────────────────────

// APIKeyStore handles persistence for downstream API keys.
type APIKeyStore struct {
	db *pgxpool.Pool
}

// NewAPIKeyStore creates an APIKeyStore.
func NewAPIKeyStore(db *pgxpool.Pool) *APIKeyStore {
	return &APIKeyStore{db: db}
}

const apiKeyCols = `id, name, key_hash, key_index, status, expires_at, rate_limit,
    quota_tokens, used_tokens, quota_reqs, used_reqs, provider_id, created_at, updated_at`

func scanAPIKey(row pgx.Row) (*models.APIKey, error) {
	k := &models.APIKey{}
	err := row.Scan(&k.ID, &k.Name, &k.KeyHash, &k.KeyIndex, &k.Status, &k.ExpiresAt,
		&k.RateLimit, &k.QuotaTokens, &k.UsedTokens, &k.QuotaReqs, &k.UsedReqs,
		&k.ProviderID, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return k, nil
}

// Create inserts a new API key.
func (s *APIKeyStore) Create(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	const q = `INSERT INTO api_keys (name, key_hash, key_index, status, expires_at, rate_limit, quota_tokens, quota_reqs, provider_id)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING ` + apiKeyCols
	row := s.db.QueryRow(ctx, q, k.Name, k.KeyHash, k.KeyIndex, k.Status, k.ExpiresAt, k.RateLimit, k.QuotaTokens, k.QuotaReqs, k.ProviderID)
	return scanAPIKey(row)
}

// GetByIndex looks up an API key by its SHA-256 index (fast path before bcrypt).
func (s *APIKeyStore) GetByIndex(ctx context.Context, keyIndex string) (*models.APIKey, error) {
	row := s.db.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM api_keys WHERE key_index=$1`, keyIndex)
	k, err := scanAPIKey(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// GetByID fetches an API key by UUID.
func (s *APIKeyStore) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	row := s.db.QueryRow(ctx, `SELECT `+apiKeyCols+` FROM api_keys WHERE id=$1`, id)
	k, err := scanAPIKey(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return k, err
}

// List returns all API keys, ordered by creation date.
func (s *APIKeyStore) List(ctx context.Context) ([]*models.APIKey, error) {
	rows, err := s.db.Query(ctx, `SELECT `+apiKeyCols+` FROM api_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, k)
	}
	return list, rows.Err()
}

// Update overwrites mutable fields of an API key.
func (s *APIKeyStore) Update(ctx context.Context, k *models.APIKey) (*models.APIKey, error) {
	const q = `UPDATE api_keys SET name=$1, status=$2, expires_at=$3, rate_limit=$4,
        quota_tokens=$5, quota_reqs=$6, provider_id=$7
        WHERE id=$8 RETURNING ` + apiKeyCols
	row := s.db.QueryRow(ctx, q, k.Name, k.Status, k.ExpiresAt, k.RateLimit,
		k.QuotaTokens, k.QuotaReqs, k.ProviderID, k.ID)
	return scanAPIKey(row)
}

// Delete removes an API key by ID.
func (s *APIKeyStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, id)
	return err
}

// IncrementUsage atomically adds tokens and request count to an API key.
func (s *APIKeyStore) IncrementUsage(ctx context.Context, id uuid.UUID, tokens int, reqs int) error {
	_, err := s.db.Exec(ctx,
		`UPDATE api_keys SET used_tokens=used_tokens+$1, used_reqs=used_reqs+$2 WHERE id=$3`,
		tokens, reqs, id)
	return err
}

// GetModelRules returns the model allow/deny rules for a key.
func (s *APIKeyStore) GetModelRules(ctx context.Context, keyID uuid.UUID) ([]*models.APIKeyModelRule, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, api_key_id, model_id, allow FROM api_key_model_rules WHERE api_key_id=$1`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []*models.APIKeyModelRule
	for rows.Next() {
		r := &models.APIKeyModelRule{}
		if err := rows.Scan(&r.ID, &r.APIKeyID, &r.ModelID, &r.Allow); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// SetModelRules replaces all model rules for a key in a single transaction.
func (s *APIKeyStore) SetModelRules(ctx context.Context, keyID uuid.UUID, rules []*models.APIKeyModelRule) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM api_key_model_rules WHERE api_key_id=$1`, keyID); err != nil {
		return err
	}
	for _, r := range rules {
		if _, err = tx.Exec(ctx,
			`INSERT INTO api_key_model_rules (api_key_id, model_id, allow) VALUES ($1,$2,$3)`,
			keyID, r.ModelID, r.Allow); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ─── Usage log store ──────────────────────────────────────────────────────────

// UsageStore handles persistence for usage logs.
type UsageStore struct {
	db *pgxpool.Pool
}

// NewUsageStore creates a UsageStore.
func NewUsageStore(db *pgxpool.Pool) *UsageStore {
	return &UsageStore{db: db}
}

// Append writes a usage log entry.
func (s *UsageStore) Append(ctx context.Context, u *models.UsageLog) error {
	const q = `INSERT INTO usage_logs
        (api_key_id, provider_id, model_id, endpoint, prompt_tokens, completion_tokens, total_tokens, duration_ms, status_code, error)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	_, err := s.db.Exec(ctx, q, u.APIKeyID, u.ProviderID, u.ModelID, u.Endpoint,
		u.PromptTokens, u.CompletionTokens, u.TotalTokens, u.DurationMs, u.StatusCode, u.Error)
	return err
}

// ListByKey returns the most recent N usage logs for an API key.
func (s *UsageStore) ListByKey(ctx context.Context, keyID uuid.UUID, limit int) ([]*models.UsageLog, error) {
	const q = `SELECT id, api_key_id, provider_id, model_id, endpoint, prompt_tokens, completion_tokens,
        total_tokens, duration_ms, status_code, error, created_at
        FROM usage_logs WHERE api_key_id=$1 ORDER BY created_at DESC LIMIT $2`
	rows, err := s.db.Query(ctx, q, keyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.UsageLog
	for rows.Next() {
		u := &models.UsageLog{}
		if err := rows.Scan(&u.ID, &u.APIKeyID, &u.ProviderID, &u.ModelID, &u.Endpoint,
			&u.PromptTokens, &u.CompletionTokens, &u.TotalTokens, &u.DurationMs,
			&u.StatusCode, &u.Error, &u.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, u)
	}
	return list, rows.Err()
}

// ─── Admin user store ─────────────────────────────────────────────────────────

// AdminStore handles persistence for admin users.
type AdminStore struct {
	db *pgxpool.Pool
}

// NewAdminStore creates an AdminStore.
func NewAdminStore(db *pgxpool.Pool) *AdminStore {
	return &AdminStore{db: db}
}

// Create inserts a new admin user.
func (s *AdminStore) Create(ctx context.Context, username, passwordHash string) (*models.AdminUser, error) {
	u := &models.AdminUser{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO admin_users (username, password_hash) VALUES ($1,$2) RETURNING id, username, password_hash, created_at`,
		username, passwordHash).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	return u, err
}

// GetByUsername fetches an admin user by username.
func (s *AdminStore) GetByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	u := &models.AdminUser{}
	err := s.db.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at FROM admin_users WHERE username=$1`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return u, err
}

// Count returns the number of admin users (used for bootstrap check).
func (s *AdminStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&n)
	return n, err
}

// AuditStore handles persistence for audit logs.
type AuditStore struct {
	db *pgxpool.Pool
}

// NewAuditStore creates an AuditStore.
func NewAuditStore(db *pgxpool.Pool) *AuditStore {
	return &AuditStore{db: db}
}

// Append writes an audit log entry.
func (s *AuditStore) Append(ctx context.Context, actor, action, resource string, detail map[string]any) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO audit_logs (actor, action, resource, detail) VALUES ($1,$2,$3,$4)`,
		actor, action, resource, detail)
	return err
}

// List returns the most recent N audit log entries.
func (s *AuditStore) List(ctx context.Context, limit int) ([]map[string]any, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, actor, action, resource, detail, created_at FROM audit_logs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		var id, actor, action, resource string
		var detail map[string]any
		var createdAt time.Time
		if err := rows.Scan(&id, &actor, &action, &resource, &detail, &createdAt); err != nil {
			return nil, err
		}
		list = append(list, map[string]any{
			"id": id, "actor": actor, "action": action,
			"resource": resource, "detail": detail, "created_at": createdAt,
		})
	}
	return list, rows.Err()
}
