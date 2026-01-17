package database

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER GLOBAL TRADING CRUD OPERATIONS
// Story 6.2: Complete User Settings Cache
// =====================================================

// GetUserGlobalTrading retrieves global trading config for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserGlobalTrading(ctx context.Context, userID string) (*UserGlobalTrading, error) {
	query := `
		SELECT id, user_id, risk_level, max_usd_allocation,
			profit_reinvest_percent, profit_reinvest_risk_level,
			COALESCE(timezone, 'UTC') as timezone,
			COALESCE(timezone_offset, '+00:00') as timezone_offset,
			created_at, updated_at
		FROM user_global_trading
		WHERE user_id = $1
	`

	config := &UserGlobalTrading{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.RiskLevel,
		&config.MaxUSDAllocation,
		&config.ProfitReinvestPercent,
		&config.ProfitReinvestRiskLevel,
		&config.Timezone,
		&config.TimezoneOffset,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user global trading: %w", err)
	}

	return config, nil
}

// SaveUserGlobalTrading saves or updates global trading config (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserGlobalTrading(ctx context.Context, config *UserGlobalTrading) error {
	query := `
		INSERT INTO user_global_trading (
			user_id, risk_level, max_usd_allocation,
			profit_reinvest_percent, profit_reinvest_risk_level,
			timezone, timezone_offset
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			risk_level = EXCLUDED.risk_level,
			max_usd_allocation = EXCLUDED.max_usd_allocation,
			profit_reinvest_percent = EXCLUDED.profit_reinvest_percent,
			profit_reinvest_risk_level = EXCLUDED.profit_reinvest_risk_level,
			timezone = EXCLUDED.timezone,
			timezone_offset = EXCLUDED.timezone_offset,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.RiskLevel,
		config.MaxUSDAllocation,
		config.ProfitReinvestPercent,
		config.ProfitReinvestRiskLevel,
		config.Timezone,
		config.TimezoneOffset,
	)
	if err != nil {
		return fmt.Errorf("failed to save user global trading: %w", err)
	}

	return nil
}

// DeleteUserGlobalTrading deletes global trading config for a user
func (r *Repository) DeleteUserGlobalTrading(ctx context.Context, userID string) error {
	query := `DELETE FROM user_global_trading WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user global trading: %w", err)
	}
	return nil
}

// InitializeUserGlobalTrading creates default global trading config for a new user
func (r *Repository) InitializeUserGlobalTrading(ctx context.Context, userID string) error {
	config := DefaultUserGlobalTrading()
	config.UserID = userID

	if err := r.SaveUserGlobalTrading(ctx, config); err != nil {
		log.Printf("[USER-GLOBAL-TRADING] Warning: Failed to initialize global trading: %v", err)
		return err
	}

	log.Printf("[USER-GLOBAL-TRADING] Initialized global trading for user %s", userID)
	return nil
}
