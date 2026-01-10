package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER GLOBAL CIRCUIT BREAKER CRUD OPERATIONS
// =====================================================

// GetUserGlobalCircuitBreaker retrieves global circuit breaker config for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserGlobalCircuitBreaker(ctx context.Context, userID string) (*UserGlobalCircuitBreaker, error) {
	query := `
		SELECT id, user_id, enabled, max_loss_per_hour, max_daily_loss,
			max_consecutive_losses, cooldown_minutes, max_trades_per_minute,
			max_daily_trades, created_at, updated_at
		FROM user_global_circuit_breaker
		WHERE user_id = $1
	`

	config := &UserGlobalCircuitBreaker{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.Enabled,
		&config.MaxLossPerHour,
		&config.MaxDailyLoss,
		&config.MaxConsecutiveLosses,
		&config.CooldownMinutes,
		&config.MaxTradesPerMinute,
		&config.MaxDailyTrades,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user global circuit breaker: %w", err)
	}

	return config, nil
}

// SaveUserGlobalCircuitBreaker saves or updates global circuit breaker config (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserGlobalCircuitBreaker(ctx context.Context, config *UserGlobalCircuitBreaker) error {
	query := `
		INSERT INTO user_global_circuit_breaker (
			user_id, enabled, max_loss_per_hour, max_daily_loss,
			max_consecutive_losses, cooldown_minutes, max_trades_per_minute,
			max_daily_trades
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			max_loss_per_hour = EXCLUDED.max_loss_per_hour,
			max_daily_loss = EXCLUDED.max_daily_loss,
			max_consecutive_losses = EXCLUDED.max_consecutive_losses,
			cooldown_minutes = EXCLUDED.cooldown_minutes,
			max_trades_per_minute = EXCLUDED.max_trades_per_minute,
			max_daily_trades = EXCLUDED.max_daily_trades,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.Enabled,
		config.MaxLossPerHour,
		config.MaxDailyLoss,
		config.MaxConsecutiveLosses,
		config.CooldownMinutes,
		config.MaxTradesPerMinute,
		config.MaxDailyTrades,
	)
	if err != nil {
		return fmt.Errorf("failed to save user global circuit breaker: %w", err)
	}

	return nil
}

