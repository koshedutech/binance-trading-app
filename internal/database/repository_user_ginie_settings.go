package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER GINIE SETTINGS CRUD OPERATIONS
// =====================================================

// GetUserGinieSettings retrieves Ginie settings for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserGinieSettings(ctx context.Context, userID string) (*UserGinieSettings, error) {
	query := `
		SELECT id, user_id, dry_run_mode, auto_start, max_positions,
			auto_mode_enabled, auto_mode_max_positions, auto_mode_max_leverage,
			auto_mode_max_position_size, auto_mode_max_total_usd, auto_mode_allow_averaging,
			auto_mode_max_averages, auto_mode_min_hold_minutes, auto_mode_quick_profit_mode,
			auto_mode_min_profit_exit, total_pnl, daily_pnl, total_trades, winning_trades,
			daily_trades, pnl_last_update, created_at, updated_at
		FROM user_ginie_settings
		WHERE user_id = $1
	`

	config := &UserGinieSettings{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.DryRunMode,
		&config.AutoStart,
		&config.MaxPositions,
		&config.AutoModeEnabled,
		&config.AutoModeMaxPositions,
		&config.AutoModeMaxLeverage,
		&config.AutoModeMaxPositionSize,
		&config.AutoModeMaxTotalUSD,
		&config.AutoModeAllowAveraging,
		&config.AutoModeMaxAverages,
		&config.AutoModeMinHoldMinutes,
		&config.AutoModeQuickProfitMode,
		&config.AutoModeMinProfitExit,
		&config.TotalPnL,
		&config.DailyPnL,
		&config.TotalTrades,
		&config.WinningTrades,
		&config.DailyTrades,
		&config.PnLLastUpdate,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user Ginie settings: %w", err)
	}

	return config, nil
}

// SaveUserGinieSettings saves or updates Ginie settings (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserGinieSettings(ctx context.Context, config *UserGinieSettings) error {
	query := `
		INSERT INTO user_ginie_settings (
			user_id, dry_run_mode, auto_start, max_positions,
			auto_mode_enabled, auto_mode_max_positions, auto_mode_max_leverage,
			auto_mode_max_position_size, auto_mode_max_total_usd, auto_mode_allow_averaging,
			auto_mode_max_averages, auto_mode_min_hold_minutes, auto_mode_quick_profit_mode,
			auto_mode_min_profit_exit, total_pnl, daily_pnl, total_trades, winning_trades,
			daily_trades, pnl_last_update
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (user_id) DO UPDATE SET
			dry_run_mode = EXCLUDED.dry_run_mode,
			auto_start = EXCLUDED.auto_start,
			max_positions = EXCLUDED.max_positions,
			auto_mode_enabled = EXCLUDED.auto_mode_enabled,
			auto_mode_max_positions = EXCLUDED.auto_mode_max_positions,
			auto_mode_max_leverage = EXCLUDED.auto_mode_max_leverage,
			auto_mode_max_position_size = EXCLUDED.auto_mode_max_position_size,
			auto_mode_max_total_usd = EXCLUDED.auto_mode_max_total_usd,
			auto_mode_allow_averaging = EXCLUDED.auto_mode_allow_averaging,
			auto_mode_max_averages = EXCLUDED.auto_mode_max_averages,
			auto_mode_min_hold_minutes = EXCLUDED.auto_mode_min_hold_minutes,
			auto_mode_quick_profit_mode = EXCLUDED.auto_mode_quick_profit_mode,
			auto_mode_min_profit_exit = EXCLUDED.auto_mode_min_profit_exit,
			total_pnl = EXCLUDED.total_pnl,
			daily_pnl = EXCLUDED.daily_pnl,
			total_trades = EXCLUDED.total_trades,
			winning_trades = EXCLUDED.winning_trades,
			daily_trades = EXCLUDED.daily_trades,
			pnl_last_update = EXCLUDED.pnl_last_update,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.DryRunMode,
		config.AutoStart,
		config.MaxPositions,
		config.AutoModeEnabled,
		config.AutoModeMaxPositions,
		config.AutoModeMaxLeverage,
		config.AutoModeMaxPositionSize,
		config.AutoModeMaxTotalUSD,
		config.AutoModeAllowAveraging,
		config.AutoModeMaxAverages,
		config.AutoModeMinHoldMinutes,
		config.AutoModeQuickProfitMode,
		config.AutoModeMinProfitExit,
		config.TotalPnL,
		config.DailyPnL,
		config.TotalTrades,
		config.WinningTrades,
		config.DailyTrades,
		config.PnLLastUpdate,
	)
	if err != nil {
		return fmt.Errorf("failed to save user Ginie settings: %w", err)
	}

	return nil
}

// UpdateGiniePnLStats updates the P&L statistics after a trade closes
// Automatically updates total_pnl, daily_pnl, and trade counters
func (r *Repository) UpdateGiniePnLStats(ctx context.Context, userID string, pnl float64, isWin bool) error {
	var query string
	if isWin {
		query = `
			UPDATE user_ginie_settings
			SET total_trades = total_trades + 1,
				winning_trades = winning_trades + 1,
				daily_trades = daily_trades + 1,
				total_pnl = total_pnl + $2,
				daily_pnl = daily_pnl + $2,
				pnl_last_update = CURRENT_TIMESTAMP,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = $1
		`
	} else {
		query = `
			UPDATE user_ginie_settings
			SET total_trades = total_trades + 1,
				daily_trades = daily_trades + 1,
				total_pnl = total_pnl + $2,
				daily_pnl = daily_pnl + $2,
				pnl_last_update = CURRENT_TIMESTAMP,
				updated_at = CURRENT_TIMESTAMP
			WHERE user_id = $1
		`
	}

	result, err := r.db.Pool.Exec(ctx, query, userID, pnl)
	if err != nil {
		return fmt.Errorf("failed to update Ginie P&L stats: %w", err)
	}

	if result.RowsAffected() == 0 {
		// Settings don't exist, create with defaults and then update stats
		defaults := DefaultUserGinieSettings()
		defaults.UserID = userID
		if err := r.SaveUserGinieSettings(ctx, defaults); err != nil {
			return err
		}
		// Retry the update
		_, err = r.db.Pool.Exec(ctx, query, userID, pnl)
		if err != nil {
			return fmt.Errorf("failed to update Ginie P&L stats after initialization: %w", err)
		}
	}

	return nil
}

// ResetDailyGiniePnL resets the daily P&L and trade counters
// Should be called at the start of each trading day
func (r *Repository) ResetDailyGiniePnL(ctx context.Context, userID string) error {
	query := `
		UPDATE user_ginie_settings
		SET daily_pnl = 0,
			daily_trades = 0,
			pnl_last_update = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to reset daily Ginie P&L: %w", err)
	}

	return nil
}
