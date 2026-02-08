package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
)

// PatternStateRecord represents a persisted pattern state for restart recovery.
// It captures the essential state of a pattern detection in progress.
type PatternStateRecord struct {
	ID              int64           `json:"id"`
	UserID          string          `json:"user_id"`
	Symbol          string          `json:"symbol"`
	Mode            string          `json:"mode"`
	Timeframe       string          `json:"timeframe"`
	Strategy        string          `json:"strategy"`
	SubStrategy     string          `json:"sub_strategy"`
	Status          string          `json:"status"`
	CurrentStep     int             `json:"current_step"`
	Direction       string          `json:"direction"`
	ReferenceCandle json.RawMessage `json:"reference_candle,omitempty"`
	ConsolidationData json.RawMessage `json:"consolidation_data,omitempty"`
	EntryLevels     json.RawMessage `json:"entry_levels,omitempty"`
	PatternData     json.RawMessage `json:"pattern_data,omitempty"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
}

// SavePatternState upserts a pattern state record for a user.
// Uses ON CONFLICT to update existing records for the same user/symbol/mode/timeframe.
func (db *DB) SavePatternState(ctx context.Context, record *PatternStateRecord) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		INSERT INTO pattern_states (
			user_id, symbol, mode, timeframe, strategy, sub_strategy,
			status, current_step, direction,
			reference_candle, consolidation_data, entry_levels, pattern_data,
			started_at, updated_at, expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16
		)
		ON CONFLICT (user_id, symbol, mode, timeframe) DO UPDATE SET
			strategy = EXCLUDED.strategy,
			sub_strategy = EXCLUDED.sub_strategy,
			status = EXCLUDED.status,
			current_step = EXCLUDED.current_step,
			direction = EXCLUDED.direction,
			reference_candle = EXCLUDED.reference_candle,
			consolidation_data = EXCLUDED.consolidation_data,
			entry_levels = EXCLUDED.entry_levels,
			pattern_data = EXCLUDED.pattern_data,
			started_at = EXCLUDED.started_at,
			updated_at = EXCLUDED.updated_at,
			expires_at = EXCLUDED.expires_at
		RETURNING id`

	now := time.Now()
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = now
	}

	err := db.Pool.QueryRow(ctx, query,
		record.UserID,
		record.Symbol,
		record.Mode,
		record.Timeframe,
		record.Strategy,
		record.SubStrategy,
		record.Status,
		record.CurrentStep,
		record.Direction,
		record.ReferenceCandle,
		record.ConsolidationData,
		record.EntryLevels,
		record.PatternData,
		record.StartedAt,
		record.UpdatedAt,
		record.ExpiresAt,
	).Scan(&record.ID)

	if err != nil {
		return fmt.Errorf("failed to save pattern state: %w", err)
	}

	return nil
}

// GetPatternStates retrieves all active pattern states for a user.
// Only returns patterns that haven't expired yet.
func (db *DB) GetPatternStates(ctx context.Context, userID string) ([]PatternStateRecord, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, symbol, mode, timeframe, strategy, sub_strategy,
			status, current_step, direction,
			reference_candle, consolidation_data, entry_levels, pattern_data,
			started_at, updated_at, expires_at
		FROM pattern_states
		WHERE user_id = $1
			AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY updated_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pattern states: %w", err)
	}
	defer rows.Close()

	return scanPatternStates(rows)
}

// DeletePatternState removes a specific pattern state record.
func (db *DB) DeletePatternState(ctx context.Context, userID, symbol, mode, timeframe string) error {
	if db.Pool == nil {
		return nil
	}

	query := `DELETE FROM pattern_states WHERE user_id = $1 AND symbol = $2 AND mode = $3 AND timeframe = $4`
	_, err := db.Pool.Exec(ctx, query, userID, symbol, mode, timeframe)
	if err != nil {
		return fmt.Errorf("failed to delete pattern state: %w", err)
	}

	return nil
}

// DeleteAllPatternStates removes all pattern states for a user.
func (db *DB) DeleteAllPatternStates(ctx context.Context, userID string) error {
	if db.Pool == nil {
		return nil
	}

	query := `DELETE FROM pattern_states WHERE user_id = $1`
	result, err := db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete all pattern states: %w", err)
	}

	count := result.RowsAffected()
	if count > 0 {
		log.Printf("[PATTERN-DB] Deleted %d pattern states for user %s", count, userID)
	}

	return nil
}

// DeleteNonPositionPatternStates removes all pattern states for a user EXCEPT those
// with position_running status. This preserves active position tracking while clearing stale patterns.
func (db *DB) DeleteNonPositionPatternStates(ctx context.Context, userID string) error {
	if db.Pool == nil {
		return nil
	}

	query := `DELETE FROM pattern_states WHERE user_id = $1 AND status != 'position_running'`
	result, err := db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete non-position pattern states: %w", err)
	}

	count := result.RowsAffected()
	if count > 0 {
		log.Printf("[PATTERN-DB] Deleted %d non-position pattern states for user %s (preserved position_running)", count, userID)
	}

	return nil
}

// CleanupExpiredPatternStates removes expired pattern states from all users.
// This can be called periodically to clean up stale data.
func (db *DB) CleanupExpiredPatternStates(ctx context.Context) (int64, error) {
	if db.Pool == nil {
		return 0, nil
	}

	query := `DELETE FROM pattern_states WHERE expires_at IS NOT NULL AND expires_at < NOW()`
	result, err := db.Pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired pattern states: %w", err)
	}

	return result.RowsAffected(), nil
}

// scanPatternStates scans rows into PatternStateRecord slice.
func scanPatternStates(rows pgx.Rows) ([]PatternStateRecord, error) {
	var records []PatternStateRecord
	for rows.Next() {
		var r PatternStateRecord
		err := rows.Scan(
			&r.ID,
			&r.UserID,
			&r.Symbol,
			&r.Mode,
			&r.Timeframe,
			&r.Strategy,
			&r.SubStrategy,
			&r.Status,
			&r.CurrentStep,
			&r.Direction,
			&r.ReferenceCandle,
			&r.ConsolidationData,
			&r.EntryLevels,
			&r.PatternData,
			&r.StartedAt,
			&r.UpdatedAt,
			&r.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pattern state: %w", err)
		}
		records = append(records, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pattern state rows: %w", err)
	}

	return records, nil
}

// ============================================================================
// REPOSITORY WRAPPER METHODS
// ============================================================================

// SavePatternState saves a pattern state via the repository.
func (r *Repository) SavePatternState(ctx context.Context, record *PatternStateRecord) error {
	return r.db.SavePatternState(ctx, record)
}

// GetPatternStates retrieves all active pattern states for a user via the repository.
func (r *Repository) GetPatternStates(ctx context.Context, userID string) ([]PatternStateRecord, error) {
	return r.db.GetPatternStates(ctx, userID)
}

// DeletePatternState removes a specific pattern state via the repository.
func (r *Repository) DeletePatternState(ctx context.Context, userID, symbol, mode, timeframe string) error {
	return r.db.DeletePatternState(ctx, userID, symbol, mode, timeframe)
}

// DeleteAllPatternStates removes all pattern states for a user via the repository.
func (r *Repository) DeleteAllPatternStates(ctx context.Context, userID string) error {
	return r.db.DeleteAllPatternStates(ctx, userID)
}

// DeleteNonPositionPatternStates removes non-position_running pattern states via the repository.
func (r *Repository) DeleteNonPositionPatternStates(ctx context.Context, userID string) error {
	return r.db.DeleteNonPositionPatternStates(ctx, userID)
}
