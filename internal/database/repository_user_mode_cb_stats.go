package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER MODE CIRCUIT BREAKER STATS CRUD OPERATIONS
// =====================================================

// GetUserModeCBStats retrieves circuit breaker stats for a specific mode
// Returns nil if not found (allows calling code to initialize defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserModeCBStats(ctx context.Context, userID, modeName string) (*UserModeCBStats, error) {
	query := `
		SELECT id, user_id, mode_name,
			trades_this_minute, trades_this_hour, trades_this_day,
			total_trades, total_wins, consecutive_losses,
			current_hour_loss, current_day_loss,
			is_paused, COALESCE(paused_until, TIMESTAMP '1970-01-01'), COALESCE(pause_reason, ''),
			last_minute_reset, last_hour_reset, last_day_reset,
			created_at, updated_at
		FROM user_mode_circuit_breaker_stats
		WHERE user_id = $1 AND mode_name = $2
	`

	stats := &UserModeCBStats{}
	err := r.db.Pool.QueryRow(ctx, query, userID, modeName).Scan(
		&stats.ID,
		&stats.UserID,
		&stats.ModeName,
		&stats.TradesThisMinute,
		&stats.TradesThisHour,
		&stats.TradesThisDay,
		&stats.TotalTrades,
		&stats.TotalWins,
		&stats.ConsecutiveLosses,
		&stats.CurrentHourLoss,
		&stats.CurrentDayLoss,
		&stats.IsPaused,
		&stats.PausedUntil,
		&stats.PauseReason,
		&stats.LastMinuteReset,
		&stats.LastHourReset,
		&stats.LastDayReset,
		&stats.CreatedAt,
		&stats.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should initialize
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user mode CB stats for mode %s: %w", modeName, err)
	}

	return stats, nil
}

// SaveUserModeCBStats saves or updates mode circuit breaker stats (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserModeCBStats(ctx context.Context, stats *UserModeCBStats) error {
	query := `
		INSERT INTO user_mode_circuit_breaker_stats (
			user_id, mode_name,
			trades_this_minute, trades_this_hour, trades_this_day,
			total_trades, total_wins, consecutive_losses,
			current_hour_loss, current_day_loss,
			is_paused, paused_until, pause_reason,
			last_minute_reset, last_hour_reset, last_day_reset
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (user_id, mode_name) DO UPDATE SET
			trades_this_minute = EXCLUDED.trades_this_minute,
			trades_this_hour = EXCLUDED.trades_this_hour,
			trades_this_day = EXCLUDED.trades_this_day,
			total_trades = EXCLUDED.total_trades,
			total_wins = EXCLUDED.total_wins,
			consecutive_losses = EXCLUDED.consecutive_losses,
			current_hour_loss = EXCLUDED.current_hour_loss,
			current_day_loss = EXCLUDED.current_day_loss,
			is_paused = EXCLUDED.is_paused,
			paused_until = EXCLUDED.paused_until,
			pause_reason = EXCLUDED.pause_reason,
			last_minute_reset = EXCLUDED.last_minute_reset,
			last_hour_reset = EXCLUDED.last_hour_reset,
			last_day_reset = EXCLUDED.last_day_reset,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		stats.UserID,
		stats.ModeName,
		stats.TradesThisMinute,
		stats.TradesThisHour,
		stats.TradesThisDay,
		stats.TotalTrades,
		stats.TotalWins,
		stats.ConsecutiveLosses,
		stats.CurrentHourLoss,
		stats.CurrentDayLoss,
		stats.IsPaused,
		stats.PausedUntil,
		stats.PauseReason,
		stats.LastMinuteReset,
		stats.LastHourReset,
		stats.LastDayReset,
	)
	if err != nil {
		return fmt.Errorf("failed to save user mode CB stats for mode %s: %w", stats.ModeName, err)
	}

	return nil
}

// GetAllUserModeCBStats retrieves all mode circuit breaker stats for a user
// Returns a map of mode_name -> stats
// Returns an empty map if no stats exist
// Returns error only for actual database errors
func (r *Repository) GetAllUserModeCBStats(ctx context.Context, userID string) (map[string]*UserModeCBStats, error) {
	query := `
		SELECT id, user_id, mode_name,
			trades_this_minute, trades_this_hour, trades_this_day,
			total_trades, total_wins, consecutive_losses,
			current_hour_loss, current_day_loss,
			is_paused, COALESCE(paused_until, TIMESTAMP '1970-01-01'), COALESCE(pause_reason, ''),
			last_minute_reset, last_hour_reset, last_day_reset,
			created_at, updated_at
		FROM user_mode_circuit_breaker_stats
		WHERE user_id = $1
		ORDER BY mode_name
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user mode CB stats: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*UserModeCBStats)
	for rows.Next() {
		stats := &UserModeCBStats{}
		err := rows.Scan(
			&stats.ID,
			&stats.UserID,
			&stats.ModeName,
			&stats.TradesThisMinute,
			&stats.TradesThisHour,
			&stats.TradesThisDay,
			&stats.TotalTrades,
			&stats.TotalWins,
			&stats.ConsecutiveLosses,
			&stats.CurrentHourLoss,
			&stats.CurrentDayLoss,
			&stats.IsPaused,
			&stats.PausedUntil,
			&stats.PauseReason,
			&stats.LastMinuteReset,
			&stats.LastHourReset,
			&stats.LastDayReset,
			&stats.CreatedAt,
			&stats.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mode CB stats row: %w", err)
		}

		result[stats.ModeName] = stats
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user mode CB stats: %w", err)
	}

	return result, nil
}

// UpdateModeCBLoss updates loss tracking for a specific mode
// This increments the loss counters and consecutive loss count
func (r *Repository) UpdateModeCBLoss(ctx context.Context, userID, modeName string, lossAmount float64) error {
	query := `
		INSERT INTO user_mode_circuit_breaker_stats (
			user_id, mode_name, current_hour_loss, current_day_loss, consecutive_losses,
			is_paused, last_minute_reset, last_hour_reset, last_day_reset
		) VALUES ($1, $2, $3, $3, 1, false, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_DATE)
		ON CONFLICT (user_id, mode_name) DO UPDATE SET
			current_hour_loss = user_mode_circuit_breaker_stats.current_hour_loss + $3,
			current_day_loss = user_mode_circuit_breaker_stats.current_day_loss + $3,
			consecutive_losses = user_mode_circuit_breaker_stats.consecutive_losses + 1,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, modeName, lossAmount)
	if err != nil {
		return fmt.Errorf("failed to update mode CB loss for mode %s: %w", modeName, err)
	}

	return nil
}

// ResetModeConsecutiveLosses resets the consecutive loss counter for a mode
// Should be called when a winning trade occurs
func (r *Repository) ResetModeConsecutiveLosses(ctx context.Context, userID, modeName string) error {
	query := `
		UPDATE user_mode_circuit_breaker_stats
		SET consecutive_losses = 0,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode_name = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, modeName)
	if err != nil {
		return fmt.Errorf("failed to reset mode consecutive losses for mode %s: %w", modeName, err)
	}

	return nil
}

// ResetModeDailyLoss resets the daily loss counter for a mode
// Should be called at the start of each trading day
func (r *Repository) ResetModeDailyLoss(ctx context.Context, userID, modeName string) error {
	query := `
		UPDATE user_mode_circuit_breaker_stats
		SET current_day_loss = 0,
			last_day_reset = CURRENT_DATE,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode_name = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, modeName)
	if err != nil {
		return fmt.Errorf("failed to reset mode daily loss for mode %s: %w", modeName, err)
	}

	return nil
}

// ResetModeHourlyLoss resets the hourly loss counter for a mode
// Should be called at the start of each hour
func (r *Repository) ResetModeHourlyLoss(ctx context.Context, userID, modeName string) error {
	query := `
		UPDATE user_mode_circuit_breaker_stats
		SET current_hour_loss = 0,
			last_hour_reset = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode_name = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, modeName)
	if err != nil {
		return fmt.Errorf("failed to reset mode hourly loss for mode %s: %w", modeName, err)
	}

	return nil
}

// TripModeCircuitBreaker trips the circuit breaker for a specific mode
func (r *Repository) TripModeCircuitBreaker(ctx context.Context, userID, modeName, reason string) error {
	query := `
		UPDATE user_mode_circuit_breaker_stats
		SET is_paused = true,
			pause_reason = $3,
			paused_until = CURRENT_TIMESTAMP + INTERVAL '30 minutes',
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode_name = $2
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, modeName, reason)
	if err != nil {
		return fmt.Errorf("failed to trip mode circuit breaker for mode %s: %w", modeName, err)
	}

	if result.RowsAffected() == 0 {
		// Stats don't exist, create with paused state
		stats := DefaultUserModeCBStats(userID, modeName)
		stats.IsPaused = true
		stats.PauseReason = reason
		return r.SaveUserModeCBStats(ctx, stats)
	}

	return nil
}

// ResetModeCircuitBreaker resets the circuit breaker for a specific mode
func (r *Repository) ResetModeCircuitBreaker(ctx context.Context, userID, modeName string) error {
	query := `
		UPDATE user_mode_circuit_breaker_stats
		SET is_paused = false,
			pause_reason = '',
			paused_until = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND mode_name = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, modeName)
	if err != nil {
		return fmt.Errorf("failed to reset mode circuit breaker for mode %s: %w", modeName, err)
	}

	return nil
}
