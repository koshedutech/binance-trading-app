package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER MODE CONFIGURATION CRUD OPERATIONS
// =====================================================

// UserModeConfig represents a user's custom configuration for a specific trading mode
// Maps to user_mode_configs table
type UserModeConfig struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	ModeName   string `json:"mode_name"` // "ultra_fast", "scalp", "swing", "position", "scalp_reentry"
	Enabled    bool   `json:"enabled"`
	ConfigJSON []byte `json:"config_json"` // JSONB column containing ModeFullConfig
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// GetUserModeConfig retrieves a single mode configuration for a user
// Returns the raw JSON config data
// Returns nil if the configuration doesn't exist (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserModeConfig(ctx context.Context, userID, modeName string) ([]byte, error) {
	query := `
		SELECT config_json
		FROM user_mode_configs
		WHERE user_id = $1 AND mode_name = $2
	`

	var configJSON []byte
	err := r.db.Pool.QueryRow(ctx, query, userID, modeName).Scan(&configJSON)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use system defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user mode config for mode %s: %w", modeName, err)
	}

	return configJSON, nil
}

// GetUserModeConfigWithEnabled retrieves a mode config including the dedicated enabled column
// The enabled column is the SOURCE OF TRUTH for mode enabled status (Story 4.11)
// Returns (enabled, configJSON, error)
func (r *Repository) GetUserModeConfigWithEnabled(ctx context.Context, userID, modeName string) (bool, []byte, error) {
	query := `
		SELECT enabled, config_json
		FROM user_mode_configs
		WHERE user_id = $1 AND mode_name = $2
	`

	var enabled bool
	var configJSON []byte
	err := r.db.Pool.QueryRow(ctx, query, userID, modeName).Scan(&enabled, &configJSON)

	if err == pgx.ErrNoRows {
		return false, nil, nil // Not found
	}
	if err != nil {
		return false, nil, fmt.Errorf("failed to get user mode config for mode %s: %w", modeName, err)
	}

	return enabled, configJSON, nil
}

// GetAllUserModeConfigs retrieves all mode configurations for a user
// Returns a map of mode_name -> raw JSON config data
// Returns an empty map if no configs exist (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetAllUserModeConfigs(ctx context.Context, userID string) (map[string][]byte, error) {
	query := `
		SELECT mode_name, config_json
		FROM user_mode_configs
		WHERE user_id = $1
		ORDER BY mode_name
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user mode configs: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]byte)
	for rows.Next() {
		var modeName string
		var configJSON []byte

		if err := rows.Scan(&modeName, &configJSON); err != nil {
			return nil, fmt.Errorf("failed to scan mode config row: %w", err)
		}

		result[modeName] = configJSON
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user mode configs: %w", err)
	}

	return result, nil
}

// SaveUserModeConfig saves/updates a mode configuration (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
// The configJSON parameter is the raw JSON bytes of the mode configuration
func (r *Repository) SaveUserModeConfig(ctx context.Context, userID, modeName string, enabled bool, configJSON []byte) error {
	query := `
		INSERT INTO user_mode_configs (user_id, mode_name, enabled, config_json)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, mode_name) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			config_json = EXCLUDED.config_json,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, modeName, enabled, configJSON)
	if err != nil {
		return fmt.Errorf("failed to save user mode config for mode %s: %w", modeName, err)
	}

	return nil
}

// UpdateUserModeEnabled updates only the enabled flag for a mode
// This is a partial update - does NOT modify the config JSON
func (r *Repository) UpdateUserModeEnabled(ctx context.Context, userID, modeName string, enabled bool) error {
	query := `
		UPDATE user_mode_configs
		SET enabled = $3, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode_name = $2
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, modeName, enabled)
	if err != nil {
		return fmt.Errorf("failed to update mode enabled status for mode %s: %w", modeName, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("mode config not found for user %s and mode %s", userID, modeName)
	}

	return nil
}

// NOTE: InitializeUserModeConfigs removed to avoid circular dependency with autopilot package
// Initialization should be done at the API/handler layer where both packages are available

// DeleteUserModeConfig removes a specific mode configuration for a user
// This will cause the system to fall back to default configuration for that mode
func (r *Repository) DeleteUserModeConfig(ctx context.Context, userID, modeName string) error {
	query := `DELETE FROM user_mode_configs WHERE user_id = $1 AND mode_name = $2`

	result, err := r.db.Pool.Exec(ctx, query, userID, modeName)
	if err != nil {
		return fmt.Errorf("failed to delete user mode config for mode %s: %w", modeName, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("mode config not found for user %s and mode %s", userID, modeName)
	}

	return nil
}

// DeleteAllUserModeConfigs removes all mode configurations for a user
// This will cause the system to fall back to default configurations for all modes
func (r *Repository) DeleteAllUserModeConfigs(ctx context.Context, userID string) error {
	query := `DELETE FROM user_mode_configs WHERE user_id = $1`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete all user mode configs: %w", err)
	}

	return nil
}


// GetEnabledUserModes retrieves all enabled mode names for a user
// Returns an empty slice if no modes are enabled or no configs exist
func (r *Repository) GetEnabledUserModes(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT mode_name
		FROM user_mode_configs
		WHERE user_id = $1 AND enabled = true
		ORDER BY mode_name
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query enabled modes: %w", err)
	}
	defer rows.Close()

	var modes []string
	for rows.Next() {
		var modeName string
		if err := rows.Scan(&modeName); err != nil {
			return nil, fmt.Errorf("failed to scan mode name: %w", err)
		}
		modes = append(modes, modeName)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating enabled modes: %w", err)
	}

	return modes, nil
}

// IsModeEnabledForUser checks if a specific mode is enabled for a user
// Returns false if the config doesn't exist (system default behavior)
func (r *Repository) IsModeEnabledForUser(ctx context.Context, userID, modeName string) (bool, error) {
	query := `SELECT enabled FROM user_mode_configs WHERE user_id = $1 AND mode_name = $2`

	var enabled bool
	err := r.db.Pool.QueryRow(ctx, query, userID, modeName).Scan(&enabled)

	if err == pgx.ErrNoRows {
		return false, nil // Config doesn't exist - treat as disabled
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if mode %s is enabled: %w", modeName, err)
	}

	return enabled, nil
}

// =====================================================
// SCALP REENTRY CONFIGURATION CRUD OPERATIONS
// =====================================================

// GetUserScalpReentryConfig retrieves the scalp_reentry mode configuration for a user
// Returns the config JSON or nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserScalpReentryConfig(ctx context.Context, userID string) ([]byte, error) {
	return r.GetUserModeConfig(ctx, userID, "scalp_reentry")
}

// SaveUserScalpReentryConfig saves/updates the scalp_reentry mode configuration (UPSERT)
// The configJSON parameter is the raw JSON bytes of the ScalpReentryConfig
// IMPORTANT: This now extracts the 'enabled' value from the config JSON itself
func (r *Repository) SaveUserScalpReentryConfig(ctx context.Context, userID string, configJSON []byte) error {
	// Extract enabled status from the config JSON (source of truth is what caller provided)
	var config struct {
		Enabled bool `json:"enabled"`
	}
	enabled := false
	if err := json.Unmarshal(configJSON, &config); err == nil {
		enabled = config.Enabled
	}

	return r.SaveUserModeConfig(ctx, userID, "scalp_reentry", enabled, configJSON)
}

