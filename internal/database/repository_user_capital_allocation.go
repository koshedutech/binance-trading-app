package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER CAPITAL ALLOCATION CRUD OPERATIONS
// =====================================================

// GetUserCapitalAllocation retrieves capital allocation configuration for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserCapitalAllocation(ctx context.Context, userID string) (*UserCapitalAllocation, error) {
	query := `
		SELECT user_id, ultra_fast_percent, scalp_percent, swing_percent, position_percent,
			max_ultra_fast_positions, max_scalp_positions,
			max_swing_positions, max_position_positions,
			max_ultra_fast_usd_per_position, max_scalp_usd_per_position,
			max_swing_usd_per_position,
			max_position_usd_per_position, allow_dynamic_rebalance,
			rebalance_threshold_pct, created_at, updated_at
		FROM user_capital_allocation
		WHERE user_id = $1
	`

	config := &UserCapitalAllocation{}
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.UserID,
		&config.UltraFastPercent,
		&config.ScalpPercent,
		&config.SwingPercent,
		&config.PositionPercent,
		&config.MaxUltraFastPositions,
		&config.MaxScalpPositions,
		&config.MaxSwingPositions,
		&config.MaxPositionPositions,
		&config.MaxUltraFastUSDPerPosition,
		&config.MaxScalpUSDPerPosition,
		&config.MaxSwingUSDPerPosition,
		&config.MaxPositionUSDPerPosition,
		&config.AllowDynamicRebalance,
		&config.RebalanceThresholdPct,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user capital allocation: %w", err)
	}

	return config, nil
}

// SaveUserCapitalAllocation saves or updates capital allocation configuration (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserCapitalAllocation(ctx context.Context, config *UserCapitalAllocation) error {
	query := `
		INSERT INTO user_capital_allocation (
			user_id, ultra_fast_percent, scalp_percent, swing_percent, position_percent,
			max_ultra_fast_positions, max_scalp_positions,
			max_swing_positions, max_position_positions,
			max_ultra_fast_usd_per_position, max_scalp_usd_per_position,
			max_swing_usd_per_position,
			max_position_usd_per_position, allow_dynamic_rebalance, rebalance_threshold_pct
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (user_id) DO UPDATE SET
			ultra_fast_percent = EXCLUDED.ultra_fast_percent,
			scalp_percent = EXCLUDED.scalp_percent,
			swing_percent = EXCLUDED.swing_percent,
			position_percent = EXCLUDED.position_percent,
			max_ultra_fast_positions = EXCLUDED.max_ultra_fast_positions,
			max_scalp_positions = EXCLUDED.max_scalp_positions,
			max_swing_positions = EXCLUDED.max_swing_positions,
			max_position_positions = EXCLUDED.max_position_positions,
			max_ultra_fast_usd_per_position = EXCLUDED.max_ultra_fast_usd_per_position,
			max_scalp_usd_per_position = EXCLUDED.max_scalp_usd_per_position,
			max_swing_usd_per_position = EXCLUDED.max_swing_usd_per_position,
			max_position_usd_per_position = EXCLUDED.max_position_usd_per_position,
			allow_dynamic_rebalance = EXCLUDED.allow_dynamic_rebalance,
			rebalance_threshold_pct = EXCLUDED.rebalance_threshold_pct,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.UltraFastPercent,
		config.ScalpPercent,
		config.SwingPercent,
		config.PositionPercent,
		config.MaxUltraFastPositions,
		config.MaxScalpPositions,
		config.MaxSwingPositions,
		config.MaxPositionPositions,
		config.MaxUltraFastUSDPerPosition,
		config.MaxScalpUSDPerPosition,
		config.MaxSwingUSDPerPosition,
		config.MaxPositionUSDPerPosition,
		config.AllowDynamicRebalance,
		config.RebalanceThresholdPct,
	)
	if err != nil {
		return fmt.Errorf("failed to save user capital allocation: %w", err)
	}

	return nil
}

// InitializeUserCapitalAllocationDefaults creates default capital allocation for a new user
// Safe to call even if config already exists (no-op on conflict)
func (r *Repository) InitializeUserCapitalAllocationDefaults(ctx context.Context, userID string) error {
	defaults := DefaultUserCapitalAllocation()
	defaults.UserID = userID

	query := `
		INSERT INTO user_capital_allocation (
			user_id, ultra_fast_percent, scalp_percent, swing_percent, position_percent,
			max_ultra_fast_positions, max_scalp_positions,
			max_swing_positions, max_position_positions,
			max_ultra_fast_usd_per_position, max_scalp_usd_per_position,
			max_swing_usd_per_position,
			max_position_usd_per_position, allow_dynamic_rebalance, rebalance_threshold_pct
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (user_id) DO NOTHING
	`

	_, err := r.db.Pool.Exec(ctx, query,
		defaults.UserID,
		defaults.UltraFastPercent,
		defaults.ScalpPercent,
		defaults.SwingPercent,
		defaults.PositionPercent,
		defaults.MaxUltraFastPositions,
		defaults.MaxScalpPositions,
		defaults.MaxSwingPositions,
		defaults.MaxPositionPositions,
		defaults.MaxUltraFastUSDPerPosition,
		defaults.MaxScalpUSDPerPosition,
		defaults.MaxSwingUSDPerPosition,
		defaults.MaxPositionUSDPerPosition,
		defaults.AllowDynamicRebalance,
		defaults.RebalanceThresholdPct,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize user capital allocation defaults: %w", err)
	}

	return nil
}
