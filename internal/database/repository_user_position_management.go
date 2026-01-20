package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER POSITION MANAGEMENT CRUD OPERATIONS
// Story 10.1: Position Decision Configuration
// =====================================================

// GetUserPositionManagement retrieves position management config for a user
// Returns nil if not found (allows calling code to use defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserPositionManagement(ctx context.Context, userID string) (*UserPositionManagement, error) {
	query := `
		SELECT id, user_id, decision_mode,
			classic_adx_reversal_threshold, classic_ema_reversal_periods,
			classic_rsi_overbought, classic_rsi_oversold, classic_reversal_confirmations,
			new_engine_use_active_strategy, new_engine_strategy_name,
			new_engine_use_strategy_exit_rules, new_engine_exit_on_regime_change,
			efficiency_exit_enabled, efficiency_exit_historical_window_hours,
			efficiency_exit_minimum_hold_minutes, efficiency_exit_consecutive_signals_required,
			dynamic_sltp_enabled, dynamic_sltp_sl_atr_multiplier,
			dynamic_sltp_tp_atr_multiplier, dynamic_sltp_update_on_binance,
			created_at, updated_at
		FROM user_position_management
		WHERE user_id = $1
	`

	config := &UserPositionManagement{}
	var emaPeriodsJSON []byte

	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&config.ID,
		&config.UserID,
		&config.DecisionMode,
		&config.ClassicADXReversalThreshold,
		&emaPeriodsJSON,
		&config.ClassicRSIOverbought,
		&config.ClassicRSIOversold,
		&config.ClassicReversalConfirmations,
		&config.NewEngineUseActiveStrategy,
		&config.NewEngineStrategyName,
		&config.NewEngineUseStrategyExitRules,
		&config.NewEngineExitOnRegimeChange,
		&config.EfficiencyExitEnabled,
		&config.EfficiencyExitHistoricalWindowHours,
		&config.EfficiencyExitMinimumHoldMinutes,
		&config.EfficiencyExitConsecutiveSignalsRequired,
		&config.DynamicSLTPEnabled,
		&config.DynamicSLTPSLATRMultiplier,
		&config.DynamicSLTPTPATRMultiplier,
		&config.DynamicSLTPUpdateOnBinance,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user position management: %w", err)
	}

	// Parse EMA periods from JSON
	if len(emaPeriodsJSON) > 0 {
		if err := json.Unmarshal(emaPeriodsJSON, &config.ClassicEMAReversalPeriods); err != nil {
			log.Printf("[POSITION-MANAGEMENT] Warning: Failed to parse EMA periods: %v", err)
			config.ClassicEMAReversalPeriods = []int{9, 21} // Default
		}
	}

	return config, nil
}

// SaveUserPositionManagement saves or updates position management config (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) SaveUserPositionManagement(ctx context.Context, config *UserPositionManagement) error {
	// Serialize EMA periods to JSON
	emaPeriodsJSON, err := json.Marshal(config.ClassicEMAReversalPeriods)
	if err != nil {
		return fmt.Errorf("failed to serialize EMA periods: %w", err)
	}

	query := `
		INSERT INTO user_position_management (
			user_id, decision_mode,
			classic_adx_reversal_threshold, classic_ema_reversal_periods,
			classic_rsi_overbought, classic_rsi_oversold, classic_reversal_confirmations,
			new_engine_use_active_strategy, new_engine_strategy_name,
			new_engine_use_strategy_exit_rules, new_engine_exit_on_regime_change,
			efficiency_exit_enabled, efficiency_exit_historical_window_hours,
			efficiency_exit_minimum_hold_minutes, efficiency_exit_consecutive_signals_required,
			dynamic_sltp_enabled, dynamic_sltp_sl_atr_multiplier,
			dynamic_sltp_tp_atr_multiplier, dynamic_sltp_update_on_binance
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (user_id) DO UPDATE SET
			decision_mode = EXCLUDED.decision_mode,
			classic_adx_reversal_threshold = EXCLUDED.classic_adx_reversal_threshold,
			classic_ema_reversal_periods = EXCLUDED.classic_ema_reversal_periods,
			classic_rsi_overbought = EXCLUDED.classic_rsi_overbought,
			classic_rsi_oversold = EXCLUDED.classic_rsi_oversold,
			classic_reversal_confirmations = EXCLUDED.classic_reversal_confirmations,
			new_engine_use_active_strategy = EXCLUDED.new_engine_use_active_strategy,
			new_engine_strategy_name = EXCLUDED.new_engine_strategy_name,
			new_engine_use_strategy_exit_rules = EXCLUDED.new_engine_use_strategy_exit_rules,
			new_engine_exit_on_regime_change = EXCLUDED.new_engine_exit_on_regime_change,
			efficiency_exit_enabled = EXCLUDED.efficiency_exit_enabled,
			efficiency_exit_historical_window_hours = EXCLUDED.efficiency_exit_historical_window_hours,
			efficiency_exit_minimum_hold_minutes = EXCLUDED.efficiency_exit_minimum_hold_minutes,
			efficiency_exit_consecutive_signals_required = EXCLUDED.efficiency_exit_consecutive_signals_required,
			dynamic_sltp_enabled = EXCLUDED.dynamic_sltp_enabled,
			dynamic_sltp_sl_atr_multiplier = EXCLUDED.dynamic_sltp_sl_atr_multiplier,
			dynamic_sltp_tp_atr_multiplier = EXCLUDED.dynamic_sltp_tp_atr_multiplier,
			dynamic_sltp_update_on_binance = EXCLUDED.dynamic_sltp_update_on_binance,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.DecisionMode,
		config.ClassicADXReversalThreshold,
		emaPeriodsJSON,
		config.ClassicRSIOverbought,
		config.ClassicRSIOversold,
		config.ClassicReversalConfirmations,
		config.NewEngineUseActiveStrategy,
		config.NewEngineStrategyName,
		config.NewEngineUseStrategyExitRules,
		config.NewEngineExitOnRegimeChange,
		config.EfficiencyExitEnabled,
		config.EfficiencyExitHistoricalWindowHours,
		config.EfficiencyExitMinimumHoldMinutes,
		config.EfficiencyExitConsecutiveSignalsRequired,
		config.DynamicSLTPEnabled,
		config.DynamicSLTPSLATRMultiplier,
		config.DynamicSLTPTPATRMultiplier,
		config.DynamicSLTPUpdateOnBinance,
	)
	if err != nil {
		return fmt.Errorf("failed to save user position management: %w", err)
	}

	return nil
}

// DeleteUserPositionManagement deletes position management config for a user
func (r *Repository) DeleteUserPositionManagement(ctx context.Context, userID string) error {
	query := `DELETE FROM user_position_management WHERE user_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user position management: %w", err)
	}
	return nil
}

// InitializeUserPositionManagement creates default position management config for a new user
func (r *Repository) InitializeUserPositionManagement(ctx context.Context, userID string) error {
	config := DefaultUserPositionManagement()
	config.UserID = userID

	if err := r.SaveUserPositionManagement(ctx, config); err != nil {
		log.Printf("[POSITION-MANAGEMENT] Warning: Failed to initialize position management: %v", err)
		return err
	}

	log.Printf("[POSITION-MANAGEMENT] Initialized position management for user %s", userID)
	return nil
}
