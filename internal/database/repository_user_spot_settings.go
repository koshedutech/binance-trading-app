package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER SPOT SETTINGS CRUD OPERATIONS
// =====================================================

// GetUserSpotSettings retrieves spot trading settings for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserSpotSettings(ctx context.Context, userID string) (*UserSpotSettings, error) {
	query := `
		SELECT id, user_id, enabled, dry_run_mode, risk_level, max_positions,
			max_usd_per_position, take_profit_percent, stop_loss_percent, min_confidence,
			circuit_breaker_enabled, cb_max_loss_per_hour, cb_max_daily_loss,
			cb_max_consecutive_losses, cb_cooldown_minutes, cb_max_trades_per_minute,
			cb_max_daily_trades, COALESCE(coin_blacklist, '{}'), COALESCE(coin_whitelist, '{}'),
			use_whitelist, total_pnl, daily_pnl, total_trades, winning_trades, daily_trades,
			pnl_last_update, created_at, updated_at
		FROM user_spot_settings
		WHERE user_id = $1
	`

	config := &UserSpotSettings{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.Enabled,
		&config.DryRunMode,
		&config.RiskLevel,
		&config.MaxPositions,
		&config.MaxUSDPerPosition,
		&config.TakeProfitPercent,
		&config.StopLossPercent,
		&config.MinConfidence,
		&config.CircuitBreakerEnabled,
		&config.CBMaxLossPerHour,
		&config.CBMaxDailyLoss,
		&config.CBMaxConsecutiveLosses,
		&config.CBCooldownMinutes,
		&config.CBMaxTradesPerMinute,
		&config.CBMaxDailyTrades,
		&config.CoinBlacklist,
		&config.CoinWhitelist,
		&config.UseWhitelist,
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
		return nil, fmt.Errorf("failed to get user spot settings: %w", err)
	}

	return config, nil
}

// SaveUserSpotSettings saves or updates spot trading settings (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserSpotSettings(ctx context.Context, config *UserSpotSettings) error {
	query := `
		INSERT INTO user_spot_settings (
			user_id, enabled, dry_run_mode, risk_level, max_positions,
			max_usd_per_position, take_profit_percent, stop_loss_percent, min_confidence,
			circuit_breaker_enabled, cb_max_loss_per_hour, cb_max_daily_loss,
			cb_max_consecutive_losses, cb_cooldown_minutes, cb_max_trades_per_minute,
			cb_max_daily_trades, coin_blacklist, coin_whitelist, use_whitelist,
			total_pnl, daily_pnl, total_trades, winning_trades, daily_trades,
			pnl_last_update
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		ON CONFLICT (user_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			dry_run_mode = EXCLUDED.dry_run_mode,
			risk_level = EXCLUDED.risk_level,
			max_positions = EXCLUDED.max_positions,
			max_usd_per_position = EXCLUDED.max_usd_per_position,
			take_profit_percent = EXCLUDED.take_profit_percent,
			stop_loss_percent = EXCLUDED.stop_loss_percent,
			min_confidence = EXCLUDED.min_confidence,
			circuit_breaker_enabled = EXCLUDED.circuit_breaker_enabled,
			cb_max_loss_per_hour = EXCLUDED.cb_max_loss_per_hour,
			cb_max_daily_loss = EXCLUDED.cb_max_daily_loss,
			cb_max_consecutive_losses = EXCLUDED.cb_max_consecutive_losses,
			cb_cooldown_minutes = EXCLUDED.cb_cooldown_minutes,
			cb_max_trades_per_minute = EXCLUDED.cb_max_trades_per_minute,
			cb_max_daily_trades = EXCLUDED.cb_max_daily_trades,
			coin_blacklist = EXCLUDED.coin_blacklist,
			coin_whitelist = EXCLUDED.coin_whitelist,
			use_whitelist = EXCLUDED.use_whitelist,
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
		config.Enabled,
		config.DryRunMode,
		config.RiskLevel,
		config.MaxPositions,
		config.MaxUSDPerPosition,
		config.TakeProfitPercent,
		config.StopLossPercent,
		config.MinConfidence,
		config.CircuitBreakerEnabled,
		config.CBMaxLossPerHour,
		config.CBMaxDailyLoss,
		config.CBMaxConsecutiveLosses,
		config.CBCooldownMinutes,
		config.CBMaxTradesPerMinute,
		config.CBMaxDailyTrades,
		config.CoinBlacklist,
		config.CoinWhitelist,
		config.UseWhitelist,
		config.TotalPnL,
		config.DailyPnL,
		config.TotalTrades,
		config.WinningTrades,
		config.DailyTrades,
		config.PnLLastUpdate,
	)
	if err != nil {
		return fmt.Errorf("failed to save user spot settings: %w", err)
	}

	return nil
}
