package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER LLM CONFIGURATION CRUD OPERATIONS
// =====================================================

// GetUserLLMConfig retrieves LLM configuration for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserLLMConfig(ctx context.Context, userID string) (*UserLLMConfig, error) {
	query := `
		SELECT id, user_id, enabled, provider, model,
			COALESCE(fallback_provider, ''), COALESCE(fallback_model, ''),
			timeout_ms, retry_count, cache_duration_sec,
			created_at, updated_at
		FROM user_llm_config
		WHERE user_id = $1
	`

	config := &UserLLMConfig{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.Enabled,
		&config.Provider,
		&config.Model,
		&config.FallbackProvider,
		&config.FallbackModel,
		&config.TimeoutMs,
		&config.RetryCount,
		&config.CacheDurationSec,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user LLM config: %w", err)
	}

	return config, nil
}

// SaveUserLLMConfig saves or updates LLM configuration (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserLLMConfig(ctx context.Context, config *UserLLMConfig) error {
	query := `
		INSERT INTO user_llm_config (
			user_id, enabled, provider, model,
			fallback_provider, fallback_model,
			timeout_ms, retry_count, cache_duration_sec
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			provider = EXCLUDED.provider,
			model = EXCLUDED.model,
			fallback_provider = EXCLUDED.fallback_provider,
			fallback_model = EXCLUDED.fallback_model,
			timeout_ms = EXCLUDED.timeout_ms,
			retry_count = EXCLUDED.retry_count,
			cache_duration_sec = EXCLUDED.cache_duration_sec,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.Enabled,
		config.Provider,
		config.Model,
		config.FallbackProvider,
		config.FallbackModel,
		config.TimeoutMs,
		config.RetryCount,
		config.CacheDurationSec,
	)
	if err != nil {
		return fmt.Errorf("failed to save user LLM config: %w", err)
	}

	return nil
}

// InitializeUserLLMConfigDefaults creates default LLM configuration for a new user
// Safe to call even if config already exists (no-op on conflict)
func (r *Repository) InitializeUserLLMConfigDefaults(ctx context.Context, userID string) error {
	defaults := DefaultUserLLMConfig()
	defaults.UserID = userID

	query := `
		INSERT INTO user_llm_config (
			user_id, enabled, provider, model,
			fallback_provider, fallback_model,
			timeout_ms, retry_count, cache_duration_sec
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		defaults.UserID,
		defaults.Enabled,
		defaults.Provider,
		defaults.Model,
		defaults.FallbackProvider,
		defaults.FallbackModel,
		defaults.TimeoutMs,
		defaults.RetryCount,
		defaults.CacheDurationSec,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize user LLM config defaults: %w", err)
	}

	return nil
}
