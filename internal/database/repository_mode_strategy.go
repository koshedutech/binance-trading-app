package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// MODE+STRATEGY SETTINGS CRUD OPERATIONS
// Story 11.29: Mode-Strategy Database Schema
// =====================================================

// ModeStrategySettingsRow represents a row in the user_mode_strategy_settings table
type ModeStrategySettingsRow struct {
	ID        int             `db:"id" json:"id"`
	UserID    string          `db:"user_id" json:"user_id"`
	Mode      string          `db:"mode" json:"mode"`
	Strategy  string          `db:"strategy" json:"strategy"`
	Enabled   bool            `db:"enabled" json:"enabled"`
	Priority  int             `db:"priority" json:"priority"`
	Settings  json.RawMessage `db:"settings" json:"settings"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`
}

// CreateModeStrategySettings creates a new mode+strategy configuration for a user
func (r *Repository) CreateModeStrategySettings(ctx context.Context, userID string, mode, strategy string, enabled bool, priority int, settings json.RawMessage) error {
	query := `
		INSERT INTO user_mode_strategy_settings (user_id, mode, strategy, enabled, priority, settings)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, mode, strategy, enabled, priority, settings)
	if err != nil {
		return fmt.Errorf("failed to create mode strategy settings: %w", err)
	}

	return nil
}

// GetModeStrategySettings retrieves a specific mode+strategy configuration for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetModeStrategySettings(ctx context.Context, userID string, mode, strategy string) (*ModeStrategySettingsRow, error) {
	query := `
		SELECT id, user_id, mode, strategy, enabled, priority, settings, created_at, updated_at
		FROM user_mode_strategy_settings
		WHERE user_id = $1 AND mode = $2 AND strategy = $3
	`

	row := &ModeStrategySettingsRow{}
	err := r.db.Pool.QueryRow(ctx, query, userID, mode, strategy).Scan(
		&row.ID,
		&row.UserID,
		&row.Mode,
		&row.Strategy,
		&row.Enabled,
		&row.Priority,
		&row.Settings,
		&row.CreatedAt,
		&row.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get mode strategy settings: %w", err)
	}

	return row, nil
}

// GetAllModeStrategySettings retrieves all mode+strategy configurations for a user
func (r *Repository) GetAllModeStrategySettings(ctx context.Context, userID string) ([]*ModeStrategySettingsRow, error) {
	query := `
		SELECT id, user_id, mode, strategy, enabled, priority, settings, created_at, updated_at
		FROM user_mode_strategy_settings
		WHERE user_id = $1
		ORDER BY mode, strategy
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get all mode strategy settings: %w", err)
	}
	defer rows.Close()

	var result []*ModeStrategySettingsRow
	for rows.Next() {
		row := &ModeStrategySettingsRow{}
		err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Mode,
			&row.Strategy,
			&row.Enabled,
			&row.Priority,
			&row.Settings,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mode strategy settings row: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating mode strategy settings rows: %w", err)
	}

	return result, nil
}

// GetModeStrategies retrieves all strategies for a specific mode for a user
func (r *Repository) GetModeStrategies(ctx context.Context, userID string, mode string) ([]*ModeStrategySettingsRow, error) {
	query := `
		SELECT id, user_id, mode, strategy, enabled, priority, settings, created_at, updated_at
		FROM user_mode_strategy_settings
		WHERE user_id = $1 AND mode = $2
		ORDER BY priority DESC, strategy
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode strategies: %w", err)
	}
	defer rows.Close()

	var result []*ModeStrategySettingsRow
	for rows.Next() {
		row := &ModeStrategySettingsRow{}
		err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Mode,
			&row.Strategy,
			&row.Enabled,
			&row.Priority,
			&row.Settings,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mode strategy row: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating mode strategy rows: %w", err)
	}

	return result, nil
}

// UpdateModeStrategySettings updates the settings for a specific mode+strategy
func (r *Repository) UpdateModeStrategySettings(ctx context.Context, userID string, mode, strategy string, settings json.RawMessage) error {
	query := `
		UPDATE user_mode_strategy_settings
		SET settings = $4, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode = $2 AND strategy = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, strategy, settings)
	if err != nil {
		return fmt.Errorf("failed to update mode strategy settings: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("mode strategy settings not found for user %s, mode %s, strategy %s", userID, mode, strategy)
	}

	return nil
}

// UpdateModeStrategyEnabled updates the enabled status for a specific mode+strategy
func (r *Repository) UpdateModeStrategyEnabled(ctx context.Context, userID string, mode, strategy string, enabled bool) error {
	query := `
		UPDATE user_mode_strategy_settings
		SET enabled = $4, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode = $2 AND strategy = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, strategy, enabled)
	if err != nil {
		return fmt.Errorf("failed to update mode strategy enabled: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("mode strategy settings not found for user %s, mode %s, strategy %s", userID, mode, strategy)
	}

	return nil
}

// DeleteModeStrategySettings deletes a specific mode+strategy configuration
func (r *Repository) DeleteModeStrategySettings(ctx context.Context, userID string, mode, strategy string) error {
	query := `
		DELETE FROM user_mode_strategy_settings
		WHERE user_id = $1 AND mode = $2 AND strategy = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, strategy)
	if err != nil {
		return fmt.Errorf("failed to delete mode strategy settings: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("mode strategy settings not found for user %s, mode %s, strategy %s", userID, mode, strategy)
	}

	return nil
}

// BulkCreateModeStrategySettings creates multiple mode+strategy configurations in a single transaction
func (r *Repository) BulkCreateModeStrategySettings(ctx context.Context, userID string, configs []ModeStrategySettingsRow) error {
	if len(configs) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO user_mode_strategy_settings (user_id, mode, strategy, enabled, priority, settings)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, mode, strategy) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			priority = EXCLUDED.priority,
			settings = EXCLUDED.settings,
			updated_at = CURRENT_TIMESTAMP
	`

	for _, config := range configs {
		_, err := tx.Exec(ctx, query,
			userID,
			config.Mode,
			config.Strategy,
			config.Enabled,
			config.Priority,
			config.Settings,
		)
		if err != nil {
			return fmt.Errorf("failed to insert mode strategy config (%s, %s): %w", config.Mode, config.Strategy, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
