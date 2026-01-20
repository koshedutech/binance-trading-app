package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// USER COIN CONFLUENCE CONFIG CRUD OPERATIONS
// Story 9.12 Phase 3: Migrate confluence config to database
// =====================================================

// UserCoinConfluenceConfig represents per-coin confluence settings in the database
type UserCoinConfluenceConfig struct {
	ID                 string                 `json:"id"`
	UserID             string                 `json:"user_id"`
	Symbol             string                 `json:"symbol"`
	Tier               string                 `json:"tier"`
	Enabled            bool                   `json:"enabled"`
	ADXMultiplier      float64                `json:"adx_multiplier"`
	ScalpADX           float64                `json:"scalp_adx"`
	SwingADX           float64                `json:"swing_adx"`
	PositionADX        float64                `json:"position_adx"`
	VolumeMultiplier   float64                `json:"volume_multiplier"`
	VolumePeriod       int                    `json:"volume_period"`
	VWAPPeriod         int                    `json:"vwap_period"`
	VWAPEnabled        bool                   `json:"vwap_enabled"`
	PivotProximity     float64                `json:"pivot_proximity"`
	PivotEnabled       bool                   `json:"pivot_enabled"`
	EMAFastPeriod      int                    `json:"ema_fast_period"`
	EMASlowPeriod      int                    `json:"ema_slow_period"`
	EMAEnabled         bool                   `json:"ema_enabled"`
	MinConfluence      int                    `json:"min_confluence"`
	ScalpMinConfluence int                    `json:"scalp_min_confluence"`
	CustomWeights      map[string]interface{} `json:"custom_weights,omitempty"`
	Notes              string                 `json:"notes"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// DefaultUserCoinConfluenceConfig returns default confluence config for a symbol
// tier can be: "blue_chip", "major_alt", "mid_cap", "small_cap", "custom"
func DefaultUserCoinConfluenceConfig(symbol, tier string) *UserCoinConfluenceConfig {
	config := &UserCoinConfluenceConfig{
		Symbol:             symbol,
		Tier:               tier,
		Enabled:            true,
		ADXMultiplier:      1.0,
		ScalpADX:           0,
		SwingADX:           0,
		PositionADX:        0,
		VolumeMultiplier:   1.0,
		VolumePeriod:       20,
		VWAPPeriod:         20,
		VWAPEnabled:        true,
		PivotProximity:     0.5,
		PivotEnabled:       true,
		EMAFastPeriod:      20,
		EMASlowPeriod:      50,
		EMAEnabled:         true,
		MinConfluence:      2,
		ScalpMinConfluence: 2,
	}

	// Apply tier-specific defaults
	switch tier {
	case "blue_chip":
		config.ADXMultiplier = 0.5
		config.VolumeMultiplier = 0.8
		config.MinConfluence = 2
		config.ScalpMinConfluence = 2
		config.Notes = "Blue chip - trends at lower ADX"
	case "major_alt":
		config.ADXMultiplier = 0.7
		config.VolumeMultiplier = 0.9
		config.MinConfluence = 2
		config.ScalpMinConfluence = 2
		config.Notes = "Major alt - relaxed settings"
	case "mid_cap":
		config.ADXMultiplier = 0.8
		config.VolumeMultiplier = 1.0
		config.MinConfluence = 2
		config.ScalpMinConfluence = 2
		config.Notes = "Mid cap - moderate confirmation"
	case "small_cap":
		config.ADXMultiplier = 0.9
		config.VolumeMultiplier = 1.2
		config.MinConfluence = 3
		config.ScalpMinConfluence = 2
		config.Notes = "Small cap - reasonable confirmation"
	}

	return config
}

// GetUserCoinConfluenceConfig retrieves confluence config for a specific user/symbol
// Returns nil if not found (allows calling code to use tier-based defaults)
// Returns error only for actual database errors
func (r *Repository) GetUserCoinConfluenceConfig(ctx context.Context, userID, symbol string) (*UserCoinConfluenceConfig, error) {
	query := `
		SELECT id, user_id, symbol, tier, enabled,
			adx_multiplier, scalp_adx, swing_adx, position_adx,
			volume_multiplier, volume_period,
			vwap_period, vwap_enabled,
			pivot_proximity, pivot_enabled,
			ema_fast_period, ema_slow_period, ema_enabled,
			min_confluence, scalp_min_confluence,
			custom_weights, COALESCE(notes, ''),
			created_at, updated_at
		FROM user_coin_confluence_config
		WHERE user_id = $1 AND symbol = $2
	`

	config := &UserCoinConfluenceConfig{}
	var customWeightsJSON []byte

	err := r.db.Pool.QueryRow(ctx, query, userID, symbol).Scan(
		&config.ID,
		&config.UserID,
		&config.Symbol,
		&config.Tier,
		&config.Enabled,
		&config.ADXMultiplier,
		&config.ScalpADX,
		&config.SwingADX,
		&config.PositionADX,
		&config.VolumeMultiplier,
		&config.VolumePeriod,
		&config.VWAPPeriod,
		&config.VWAPEnabled,
		&config.PivotProximity,
		&config.PivotEnabled,
		&config.EMAFastPeriod,
		&config.EMASlowPeriod,
		&config.EMAEnabled,
		&config.MinConfluence,
		&config.ScalpMinConfluence,
		&customWeightsJSON,
		&config.Notes,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use tier-based defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user coin confluence config for %s: %w", symbol, err)
	}

	// Parse custom weights JSON if present
	if len(customWeightsJSON) > 0 {
		if err := json.Unmarshal(customWeightsJSON, &config.CustomWeights); err != nil {
			log.Printf("[WARN] Failed to unmarshal custom_weights for symbol %s: %v", config.Symbol, err)
			config.CustomWeights = nil
		}
	}

	return config, nil
}

// GetAllUserCoinConfluenceConfigs retrieves all confluence configs for a user
// Returns a map of symbol -> config
// Returns empty map if no configs exist
// Returns error only for actual database errors
func (r *Repository) GetAllUserCoinConfluenceConfigs(ctx context.Context, userID string) (map[string]*UserCoinConfluenceConfig, error) {
	query := `
		SELECT id, user_id, symbol, tier, enabled,
			adx_multiplier, scalp_adx, swing_adx, position_adx,
			volume_multiplier, volume_period,
			vwap_period, vwap_enabled,
			pivot_proximity, pivot_enabled,
			ema_fast_period, ema_slow_period, ema_enabled,
			min_confluence, scalp_min_confluence,
			custom_weights, COALESCE(notes, ''),
			created_at, updated_at
		FROM user_coin_confluence_config
		WHERE user_id = $1
		ORDER BY symbol
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user coin confluence configs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*UserCoinConfluenceConfig)
	for rows.Next() {
		config := &UserCoinConfluenceConfig{}
		var customWeightsJSON []byte

		err := rows.Scan(
			&config.ID,
			&config.UserID,
			&config.Symbol,
			&config.Tier,
			&config.Enabled,
			&config.ADXMultiplier,
			&config.ScalpADX,
			&config.SwingADX,
			&config.PositionADX,
			&config.VolumeMultiplier,
			&config.VolumePeriod,
			&config.VWAPPeriod,
			&config.VWAPEnabled,
			&config.PivotProximity,
			&config.PivotEnabled,
			&config.EMAFastPeriod,
			&config.EMASlowPeriod,
			&config.EMAEnabled,
			&config.MinConfluence,
			&config.ScalpMinConfluence,
			&customWeightsJSON,
			&config.Notes,
			&config.CreatedAt,
			&config.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan coin confluence config row: %w", err)
		}

		// Parse custom weights JSON if present
		if len(customWeightsJSON) > 0 {
			if err := json.Unmarshal(customWeightsJSON, &config.CustomWeights); err != nil {
				log.Printf("[WARN] Failed to unmarshal custom_weights for symbol %s: %v", config.Symbol, err)
				config.CustomWeights = nil
			}
		}

		result[config.Symbol] = config
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user coin confluence configs: %w", err)
	}

	return result, nil
}

// UpsertUserCoinConfluenceConfig creates or updates a confluence config (UPSERT)
// Performs upsert operation - creates if doesn't exist, updates if exists
func (r *Repository) UpsertUserCoinConfluenceConfig(ctx context.Context, config *UserCoinConfluenceConfig) error {
	// Serialize custom weights to JSON
	var customWeightsJSON []byte
	if config.CustomWeights != nil {
		var err error
		customWeightsJSON, err = json.Marshal(config.CustomWeights)
		if err != nil {
			return fmt.Errorf("failed to marshal custom weights: %w", err)
		}
	}

	query := `
		INSERT INTO user_coin_confluence_config (
			user_id, symbol, tier, enabled,
			adx_multiplier, scalp_adx, swing_adx, position_adx,
			volume_multiplier, volume_period,
			vwap_period, vwap_enabled,
			pivot_proximity, pivot_enabled,
			ema_fast_period, ema_slow_period, ema_enabled,
			min_confluence, scalp_min_confluence,
			custom_weights, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		ON CONFLICT (user_id, symbol) DO UPDATE SET
			tier = EXCLUDED.tier,
			enabled = EXCLUDED.enabled,
			adx_multiplier = EXCLUDED.adx_multiplier,
			scalp_adx = EXCLUDED.scalp_adx,
			swing_adx = EXCLUDED.swing_adx,
			position_adx = EXCLUDED.position_adx,
			volume_multiplier = EXCLUDED.volume_multiplier,
			volume_period = EXCLUDED.volume_period,
			vwap_period = EXCLUDED.vwap_period,
			vwap_enabled = EXCLUDED.vwap_enabled,
			pivot_proximity = EXCLUDED.pivot_proximity,
			pivot_enabled = EXCLUDED.pivot_enabled,
			ema_fast_period = EXCLUDED.ema_fast_period,
			ema_slow_period = EXCLUDED.ema_slow_period,
			ema_enabled = EXCLUDED.ema_enabled,
			min_confluence = EXCLUDED.min_confluence,
			scalp_min_confluence = EXCLUDED.scalp_min_confluence,
			custom_weights = EXCLUDED.custom_weights,
			notes = EXCLUDED.notes,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := r.db.Pool.Exec(ctx, query,
		config.UserID,
		config.Symbol,
		config.Tier,
		config.Enabled,
		config.ADXMultiplier,
		config.ScalpADX,
		config.SwingADX,
		config.PositionADX,
		config.VolumeMultiplier,
		config.VolumePeriod,
		config.VWAPPeriod,
		config.VWAPEnabled,
		config.PivotProximity,
		config.PivotEnabled,
		config.EMAFastPeriod,
		config.EMASlowPeriod,
		config.EMAEnabled,
		config.MinConfluence,
		config.ScalpMinConfluence,
		customWeightsJSON,
		config.Notes,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert user coin confluence config for %s: %w", config.Symbol, err)
	}

	return nil
}

// DeleteUserCoinConfluenceConfig deletes a confluence config for a user/symbol
// Used when user wants to revert to tier-based defaults
func (r *Repository) DeleteUserCoinConfluenceConfig(ctx context.Context, userID, symbol string) error {
	query := `
		DELETE FROM user_coin_confluence_config
		WHERE user_id = $1 AND symbol = $2
	`

	_, err := r.db.Pool.Exec(ctx, query, userID, symbol)
	if err != nil {
		return fmt.Errorf("failed to delete user coin confluence config for %s: %w", symbol, err)
	}

	return nil
}

// DeleteAllUserCoinConfluenceConfigs deletes all confluence configs for a user
// Used during user data cleanup or reset
func (r *Repository) DeleteAllUserCoinConfluenceConfigs(ctx context.Context, userID string) error {
	query := `
		DELETE FROM user_coin_confluence_config
		WHERE user_id = $1
	`

	_, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete all user coin confluence configs: %w", err)
	}

	return nil
}
