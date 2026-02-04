package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// =====================================================
// STRATEGY HIERARCHY REPOSITORY
// Story 11.44: Volume Imbalance Database Schema & Repository
// Supports Mode -> Strategy Group -> Sub-Strategy architecture
// =====================================================

// =====================================================
// TYPES
// =====================================================

// StrategyGroupSettings represents a strategy group configuration
// Groups: breakout, trending, range, volatile
type StrategyGroupSettings struct {
	ID                  string    `db:"id" json:"id"`
	UserID              string    `db:"user_id" json:"user_id"`
	Mode                string    `db:"mode" json:"mode"`                                   // scalp, swing, position, ultra_fast
	StrategyGroup       string    `db:"strategy_group" json:"strategy_group"`               // breakout, trending, range, volatile
	Enabled             bool      `db:"enabled" json:"enabled"`
	Timeframe           string    `db:"timeframe" json:"timeframe"`                         // 15m, 1h, 4h, etc.
	PositionSizePercent float64   `db:"position_size_percent" json:"position_size_percent"` // 2.0 = 2%
	MaxLeverage         int       `db:"max_leverage" json:"max_leverage"`
	MaxPositions        int       `db:"max_positions" json:"max_positions"`
	MinVolumeUSDT       float64   `db:"min_volume_usdt" json:"min_volume_usdt"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}

// SubStrategySettings represents a sub-strategy configuration
// e.g., ravindra_volume_imbalance, classic_breakout
type SubStrategySettings struct {
	ID            string          `db:"id" json:"id"`
	UserID        string          `db:"user_id" json:"user_id"`
	Mode          string          `db:"mode" json:"mode"`
	StrategyGroup string          `db:"strategy_group" json:"strategy_group"`
	SubStrategy   string          `db:"sub_strategy" json:"sub_strategy"`
	Enabled       bool            `db:"enabled" json:"enabled"`
	Settings      json.RawMessage `db:"settings" json:"settings"` // Strategy-specific JSONB settings
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at" json:"updated_at"`
}

// EnabledSubStrategy is a compact representation for quick lookups
type EnabledSubStrategy struct {
	Mode          string `json:"mode"`
	StrategyGroup string `json:"strategy_group"`
	SubStrategy   string `json:"sub_strategy"`
}

// =====================================================
// VALID VALUES
// =====================================================

// ValidStrategyGroups returns the list of valid strategy groups
func ValidStrategyGroups() []string {
	return []string{"breakout", "trending", "range", "volatile"}
}

// ValidSubStrategies returns known sub-strategies (extensible)
func ValidSubStrategies() []string {
	return []string{
		"ravindra_volume_imbalance",
		"classic_breakout",
		// Future sub-strategies can be added here
	}
}

// =====================================================
// INPUT VALIDATION (Story 11.44 Fix)
// =====================================================

// isValidMode checks if the mode is valid
func isValidMode(mode string) bool {
	for _, valid := range ValidModes() {
		if mode == valid {
			return true
		}
	}
	return false
}

// isValidStrategyGroup checks if the strategy group is valid
func isValidStrategyGroup(group string) bool {
	for _, valid := range ValidStrategyGroups() {
		if group == valid {
			return true
		}
	}
	return false
}

// validateModeAndGroup validates both mode and strategy group
func validateModeAndGroup(mode, group string) error {
	if !isValidMode(mode) {
		return fmt.Errorf("invalid mode: %s", mode)
	}
	if !isValidStrategyGroup(group) {
		return fmt.Errorf("invalid strategy group: %s", group)
	}
	return nil
}

// =====================================================
// STRATEGY GROUP OPERATIONS
// =====================================================

// GetStrategyGroupSettings retrieves a specific strategy group for a user
// Returns nil if not found (allows calling code to use defaults)
func (r *Repository) GetStrategyGroupSettings(ctx context.Context, userID, mode, group string) (*StrategyGroupSettings, error) {
	// Validate inputs (Story 11.44 Fix)
	if err := validateModeAndGroup(mode, group); err != nil {
		return nil, err
	}

	query := `
		SELECT id, user_id, mode, strategy_group, enabled,
		       timeframe, position_size_percent, max_leverage, max_positions, min_volume_usdt,
		       created_at, updated_at
		FROM user_strategy_group_settings
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3
	`

	row := &StrategyGroupSettings{}
	err := r.db.Pool.QueryRow(ctx, query, userID, mode, group).Scan(
		&row.ID,
		&row.UserID,
		&row.Mode,
		&row.StrategyGroup,
		&row.Enabled,
		&row.Timeframe,
		&row.PositionSizePercent,
		&row.MaxLeverage,
		&row.MaxPositions,
		&row.MinVolumeUSDT,
		&row.CreatedAt,
		&row.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found - caller should use defaults
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy group settings: %w", err)
	}

	return row, nil
}

// GetAllStrategyGroups retrieves all strategy groups for a user's mode
func (r *Repository) GetAllStrategyGroups(ctx context.Context, userID, mode string) ([]*StrategyGroupSettings, error) {
	// Validate mode (Story 11.44 Fix)
	if !isValidMode(mode) {
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	query := `
		SELECT id, user_id, mode, strategy_group, enabled,
		       timeframe, position_size_percent, max_leverage, max_positions, min_volume_usdt,
		       created_at, updated_at
		FROM user_strategy_group_settings
		WHERE user_id = $1 AND mode = $2
		ORDER BY strategy_group
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get strategy groups: %w", err)
	}
	defer rows.Close()

	var result []*StrategyGroupSettings
	for rows.Next() {
		row := &StrategyGroupSettings{}
		err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Mode,
			&row.StrategyGroup,
			&row.Enabled,
			&row.Timeframe,
			&row.PositionSizePercent,
			&row.MaxLeverage,
			&row.MaxPositions,
			&row.MinVolumeUSDT,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan strategy group row: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating strategy group rows: %w", err)
	}

	return result, nil
}

// UpsertStrategyGroupSettings creates or updates a strategy group configuration
func (r *Repository) UpsertStrategyGroupSettings(ctx context.Context, settings *StrategyGroupSettings) error {
	// Validate inputs (Story 11.44 Fix)
	if err := validateModeAndGroup(settings.Mode, settings.StrategyGroup); err != nil {
		return err
	}

	query := `
		INSERT INTO user_strategy_group_settings
		    (user_id, mode, strategy_group, enabled, timeframe, position_size_percent, max_leverage, max_positions, min_volume_usdt)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, mode, strategy_group) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			timeframe = EXCLUDED.timeframe,
			position_size_percent = EXCLUDED.position_size_percent,
			max_leverage = EXCLUDED.max_leverage,
			max_positions = EXCLUDED.max_positions,
			min_volume_usdt = EXCLUDED.min_volume_usdt,
			updated_at = NOW()
	`

	_, err := r.db.Pool.Exec(ctx, query,
		settings.UserID,
		settings.Mode,
		settings.StrategyGroup,
		settings.Enabled,
		settings.Timeframe,
		settings.PositionSizePercent,
		settings.MaxLeverage,
		settings.MaxPositions,
		settings.MinVolumeUSDT,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert strategy group settings: %w", err)
	}

	return nil
}

// UpdateStrategyGroupEnabled toggles the enabled status for a strategy group
func (r *Repository) UpdateStrategyGroupEnabled(ctx context.Context, userID, mode, group string, enabled bool) error {
	query := `
		UPDATE user_strategy_group_settings
		SET enabled = $4, updated_at = NOW()
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group, enabled)
	if err != nil {
		return fmt.Errorf("failed to update strategy group enabled: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("strategy group not found: user=%s, mode=%s, group=%s", userID, mode, group)
	}

	return nil
}

// DeleteStrategyGroupSettings removes a strategy group configuration
func (r *Repository) DeleteStrategyGroupSettings(ctx context.Context, userID, mode, group string) error {
	query := `
		DELETE FROM user_strategy_group_settings
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group)
	if err != nil {
		return fmt.Errorf("failed to delete strategy group: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("strategy group not found: user=%s, mode=%s, group=%s", userID, mode, group)
	}

	return nil
}

// =====================================================
// SUB-STRATEGY OPERATIONS
// =====================================================

// GetSubStrategySettings retrieves a specific sub-strategy for a user
// Returns nil if not found
func (r *Repository) GetSubStrategySettings(ctx context.Context, userID, mode, group, subStrategy string) (*SubStrategySettings, error) {
	// Validate inputs (Story 11.44 Fix)
	if err := validateModeAndGroup(mode, group); err != nil {
		return nil, err
	}

	query := `
		SELECT id, user_id, mode, strategy_group, sub_strategy, enabled, settings, created_at, updated_at
		FROM user_sub_strategy_settings
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
	`

	row := &SubStrategySettings{}
	err := r.db.Pool.QueryRow(ctx, query, userID, mode, group, subStrategy).Scan(
		&row.ID,
		&row.UserID,
		&row.Mode,
		&row.StrategyGroup,
		&row.SubStrategy,
		&row.Enabled,
		&row.Settings,
		&row.CreatedAt,
		&row.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-strategy settings: %w", err)
	}

	return row, nil
}

// GetAllSubStrategies retrieves all sub-strategies for a user's mode/group
func (r *Repository) GetAllSubStrategies(ctx context.Context, userID, mode, group string) ([]*SubStrategySettings, error) {
	query := `
		SELECT id, user_id, mode, strategy_group, sub_strategy, enabled, settings, created_at, updated_at
		FROM user_sub_strategy_settings
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3
		ORDER BY sub_strategy
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, mode, group)
	if err != nil {
		return nil, fmt.Errorf("failed to get sub-strategies: %w", err)
	}
	defer rows.Close()

	var result []*SubStrategySettings
	for rows.Next() {
		row := &SubStrategySettings{}
		err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Mode,
			&row.StrategyGroup,
			&row.SubStrategy,
			&row.Enabled,
			&row.Settings,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan sub-strategy row: %w", err)
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sub-strategy rows: %w", err)
	}

	return result, nil
}

// UpsertSubStrategySettings creates or updates a sub-strategy configuration
func (r *Repository) UpsertSubStrategySettings(ctx context.Context, settings *SubStrategySettings) error {
	// Validate inputs (Story 11.44 Fix)
	if err := validateModeAndGroup(settings.Mode, settings.StrategyGroup); err != nil {
		return err
	}

	query := `
		INSERT INTO user_sub_strategy_settings
		    (user_id, mode, strategy_group, sub_strategy, enabled, settings)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, mode, strategy_group, sub_strategy) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			settings = EXCLUDED.settings,
			updated_at = NOW()
	`

	_, err := r.db.Pool.Exec(ctx, query,
		settings.UserID,
		settings.Mode,
		settings.StrategyGroup,
		settings.SubStrategy,
		settings.Enabled,
		settings.Settings,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert sub-strategy settings: %w", err)
	}

	return nil
}

// UpdateSubStrategyEnabled toggles the enabled status for a sub-strategy
func (r *Repository) UpdateSubStrategyEnabled(ctx context.Context, userID, mode, group, subStrategy string, enabled bool) error {
	query := `
		UPDATE user_sub_strategy_settings
		SET enabled = $5, updated_at = NOW()
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group, subStrategy, enabled)
	if err != nil {
		return fmt.Errorf("failed to update sub-strategy enabled: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sub-strategy not found: user=%s, mode=%s, group=%s, sub=%s", userID, mode, group, subStrategy)
	}

	return nil
}

// UpdateSubStrategySettings updates just the JSONB settings for a sub-strategy
func (r *Repository) UpdateSubStrategySettingsJSON(ctx context.Context, userID, mode, group, subStrategy string, settings json.RawMessage) error {
	query := `
		UPDATE user_sub_strategy_settings
		SET settings = $5, updated_at = NOW()
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group, subStrategy, settings)
	if err != nil {
		return fmt.Errorf("failed to update sub-strategy settings: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sub-strategy not found: user=%s, mode=%s, group=%s, sub=%s", userID, mode, group, subStrategy)
	}

	return nil
}

// DeleteSubStrategySettings removes a sub-strategy configuration
func (r *Repository) DeleteSubStrategySettings(ctx context.Context, userID, mode, group, subStrategy string) error {
	query := `
		DELETE FROM user_sub_strategy_settings
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group, subStrategy)
	if err != nil {
		return fmt.Errorf("failed to delete sub-strategy: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sub-strategy not found: user=%s, mode=%s, group=%s, sub=%s", userID, mode, group, subStrategy)
	}

	return nil
}

// UpdateSubStrategyEquity updates the current_equity value in the budget_allocation settings.
// This is used for compounding profits/losses when position_sizing is "all_in".
// The pnl parameter is the realized P&L to add (positive for profit, negative for loss).
func (r *Repository) UpdateSubStrategyEquity(ctx context.Context, userID, mode, group, subStrategy string, pnl float64) error {
	// Use PostgreSQL JSONB operations to update just the current_equity field atomically
	// This avoids race conditions when multiple positions close simultaneously
	query := `
		UPDATE user_sub_strategy_settings
		SET settings = jsonb_set(
			settings,
			'{budget_allocation,current_equity}',
			to_jsonb(COALESCE(
				(settings->'budget_allocation'->>'current_equity')::numeric,
				(settings->'budget_allocation'->>'assigned_budget_usd')::numeric,
				0
			) + $5)
		),
		updated_at = NOW()
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
		AND settings->'budget_allocation' IS NOT NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group, subStrategy, pnl)
	if err != nil {
		return fmt.Errorf("failed to update sub-strategy equity: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sub-strategy not found or no budget_allocation: user=%s, mode=%s, group=%s, sub=%s",
			userID, mode, group, subStrategy)
	}

	return nil
}

// GetSubStrategyEquity returns the current_equity for a sub-strategy.
// Returns assigned_budget_usd if current_equity is not set.
func (r *Repository) GetSubStrategyEquity(ctx context.Context, userID, mode, group, subStrategy string) (float64, error) {
	query := `
		SELECT COALESCE(
			(settings->'budget_allocation'->>'current_equity')::numeric,
			(settings->'budget_allocation'->>'assigned_budget_usd')::numeric,
			0
		)
		FROM user_sub_strategy_settings
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
	`

	var equity float64
	err := r.db.Pool.QueryRow(ctx, query, userID, mode, group, subStrategy).Scan(&equity)
	if err != nil {
		return 0, fmt.Errorf("failed to get sub-strategy equity: %w", err)
	}

	return equity, nil
}

// ResetSubStrategyEquity resets the current_equity to assigned_budget_usd.
// This is used to start fresh after a series of losses or by user request.
func (r *Repository) ResetSubStrategyEquity(ctx context.Context, userID, mode, group, subStrategy string) error {
	query := `
		UPDATE user_sub_strategy_settings
		SET settings = jsonb_set(
			settings,
			'{budget_allocation,current_equity}',
			settings->'budget_allocation'->'assigned_budget_usd'
		),
		updated_at = NOW()
		WHERE user_id = $1 AND mode = $2 AND strategy_group = $3 AND sub_strategy = $4
		AND settings->'budget_allocation' IS NOT NULL
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, mode, group, subStrategy)
	if err != nil {
		return fmt.Errorf("failed to reset sub-strategy equity: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("sub-strategy not found or no budget_allocation: user=%s, mode=%s, group=%s, sub=%s",
			userID, mode, group, subStrategy)
	}

	return nil
}

// =====================================================
// QUERY OPERATIONS
// =====================================================

// GetEnabledStrategies returns all enabled sub-strategies for a user
// This is the primary query for finding active trading strategies
func (r *Repository) GetEnabledStrategies(ctx context.Context, userID string) ([]EnabledSubStrategy, error) {
	query := `
		SELECT s.mode, s.strategy_group, s.sub_strategy
		FROM user_sub_strategy_settings s
		INNER JOIN user_strategy_group_settings g
			ON s.user_id = g.user_id
			AND s.mode = g.mode
			AND s.strategy_group = g.strategy_group
		WHERE s.user_id = $1
			AND s.enabled = true
			AND g.enabled = true
		ORDER BY s.mode, s.strategy_group, s.sub_strategy
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled strategies: %w", err)
	}
	defer rows.Close()

	var result []EnabledSubStrategy
	for rows.Next() {
		var es EnabledSubStrategy
		if err := rows.Scan(&es.Mode, &es.StrategyGroup, &es.SubStrategy); err != nil {
			return nil, fmt.Errorf("failed to scan enabled strategy: %w", err)
		}
		result = append(result, es)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating enabled strategies: %w", err)
	}

	return result, nil
}

// GetEnabledStrategiesForMode returns enabled sub-strategies for a specific mode
func (r *Repository) GetEnabledStrategiesForMode(ctx context.Context, userID, mode string) ([]EnabledSubStrategy, error) {
	query := `
		SELECT s.mode, s.strategy_group, s.sub_strategy
		FROM user_sub_strategy_settings s
		INNER JOIN user_strategy_group_settings g
			ON s.user_id = g.user_id
			AND s.mode = g.mode
			AND s.strategy_group = g.strategy_group
		WHERE s.user_id = $1
			AND s.mode = $2
			AND s.enabled = true
			AND g.enabled = true
		ORDER BY s.strategy_group, s.sub_strategy
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled strategies for mode: %w", err)
	}
	defer rows.Close()

	var result []EnabledSubStrategy
	for rows.Next() {
		var es EnabledSubStrategy
		if err := rows.Scan(&es.Mode, &es.StrategyGroup, &es.SubStrategy); err != nil {
			return nil, fmt.Errorf("failed to scan enabled strategy: %w", err)
		}
		result = append(result, es)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating enabled strategies: %w", err)
	}

	return result, nil
}

// GetFullStrategyHierarchy retrieves the complete strategy hierarchy for a user
// Returns map[mode]map[group][]sub-strategy with full settings
func (r *Repository) GetFullStrategyHierarchy(ctx context.Context, userID string) (map[string]map[string][]*SubStrategySettings, error) {
	query := `
		SELECT id, user_id, mode, strategy_group, sub_strategy, enabled, settings, created_at, updated_at
		FROM user_sub_strategy_settings
		WHERE user_id = $1
		ORDER BY mode, strategy_group, sub_strategy
	`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get full strategy hierarchy: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string][]*SubStrategySettings)
	for rows.Next() {
		row := &SubStrategySettings{}
		err := rows.Scan(
			&row.ID,
			&row.UserID,
			&row.Mode,
			&row.StrategyGroup,
			&row.SubStrategy,
			&row.Enabled,
			&row.Settings,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan strategy row: %w", err)
		}

		if result[row.Mode] == nil {
			result[row.Mode] = make(map[string][]*SubStrategySettings)
		}
		result[row.Mode][row.StrategyGroup] = append(result[row.Mode][row.StrategyGroup], row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating strategy hierarchy: %w", err)
	}

	return result, nil
}

// =====================================================
// BULK OPERATIONS
// =====================================================

// BulkUpsertStrategyGroups creates/updates multiple strategy groups in a transaction
func (r *Repository) BulkUpsertStrategyGroups(ctx context.Context, settings []*StrategyGroupSettings) error {
	if len(settings) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO user_strategy_group_settings
		    (user_id, mode, strategy_group, enabled, timeframe, position_size_percent, max_leverage, max_positions, min_volume_usdt)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, mode, strategy_group) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			timeframe = EXCLUDED.timeframe,
			position_size_percent = EXCLUDED.position_size_percent,
			max_leverage = EXCLUDED.max_leverage,
			max_positions = EXCLUDED.max_positions,
			min_volume_usdt = EXCLUDED.min_volume_usdt,
			updated_at = NOW()
	`

	for _, s := range settings {
		_, err := tx.Exec(ctx, query,
			s.UserID,
			s.Mode,
			s.StrategyGroup,
			s.Enabled,
			s.Timeframe,
			s.PositionSizePercent,
			s.MaxLeverage,
			s.MaxPositions,
			s.MinVolumeUSDT,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert strategy group %s/%s: %w", s.Mode, s.StrategyGroup, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// BulkUpsertSubStrategies creates/updates multiple sub-strategies in a transaction
func (r *Repository) BulkUpsertSubStrategies(ctx context.Context, settings []*SubStrategySettings) error {
	if len(settings) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO user_sub_strategy_settings
		    (user_id, mode, strategy_group, sub_strategy, enabled, settings)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, mode, strategy_group, sub_strategy) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			settings = EXCLUDED.settings,
			updated_at = NOW()
	`

	for _, s := range settings {
		_, err := tx.Exec(ctx, query,
			s.UserID,
			s.Mode,
			s.StrategyGroup,
			s.SubStrategy,
			s.Enabled,
			s.Settings,
		)
		if err != nil {
			return fmt.Errorf("failed to upsert sub-strategy %s/%s/%s: %w", s.Mode, s.StrategyGroup, s.SubStrategy, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
