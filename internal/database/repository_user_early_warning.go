package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER EARLY WARNING CRUD OPERATIONS
// =====================================================

// GetUserEarlyWarning retrieves early warning configuration for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserEarlyWarning(ctx context.Context, userID string) (*UserEarlyWarning, error) {
	query := `
		SELECT id, user_id, enabled, start_after_minutes, check_interval_secs,
			only_underwater, min_loss_percent, close_on_reversal,
			created_at, updated_at
		FROM user_early_warning
		WHERE user_id = $1
	`

	config := &UserEarlyWarning{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.Enabled,
		&config.StartAfterMinutes,
		&config.CheckIntervalSecs,
		&config.OnlyUnderwater,
		&config.MinLossPercent,
		&config.CloseOnReversal,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user early warning: %w", err)
	}

	return config, nil
}

// SaveUserEarlyWarning saves or updates early warning configuration (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserEarlyWarning(ctx context.Context, config *UserEarlyWarning) error {
	query := `
		INSERT INTO user_early_warning (
			user_id, enabled, start_after_minutes, check_interval_secs,
			only_underwater, min_loss_percent, close_on_reversal
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			start_after_minutes = EXCLUDED.start_after_minutes,
			check_interval_secs = EXCLUDED.check_interval_secs,
			only_underwater = EXCLUDED.only_underwater,
			min_loss_percent = EXCLUDED.min_loss_percent,
			close_on_reversal = EXCLUDED.close_on_reversal,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.Enabled,
		config.StartAfterMinutes,
		config.CheckIntervalSecs,
		config.OnlyUnderwater,
		config.MinLossPercent,
		config.CloseOnReversal,
	)
	if err != nil {
		return fmt.Errorf("failed to save user early warning: %w", err)
	}

	return nil
}
