// Package database provides repository methods for daily position snapshots.
// Epic 8 Story 8.1: EOD Snapshot of Open Positions
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DailyPositionSnapshot represents a daily position snapshot in the database
// Note: Named "Daily" to distinguish from the existing PositionSnapshot in models.go
type DailyPositionSnapshot struct {
	ID            string     `json:"id"`
	UserID        string     `json:"user_id"`
	SnapshotDate  time.Time  `json:"snapshot_date"`
	Symbol        string     `json:"symbol"`
	PositionSide  string     `json:"position_side"`
	Quantity      float64    `json:"quantity"`
	EntryPrice    float64    `json:"entry_price"`
	MarkPrice     float64    `json:"mark_price"`
	UnrealizedPnL float64    `json:"unrealized_pnl"`
	Mode          string     `json:"mode"`
	ClientOrderID *string    `json:"client_order_id,omitempty"`
	Leverage      int        `json:"leverage"`
	MarginType    string     `json:"margin_type"`
	CreatedAt     time.Time  `json:"created_at"`
}

// SaveDailyPositionSnapshot saves a single position snapshot to the database
// Uses ON CONFLICT to upsert (update if exists for same user/date/symbol/side)
func (r *Repository) SaveDailyPositionSnapshot(ctx context.Context, snapshot *DailyPositionSnapshot) error {
	query := `
		INSERT INTO daily_position_snapshots (
			user_id, snapshot_date, symbol, position_side, quantity,
			entry_price, mark_price, unrealized_pnl, mode, client_order_id,
			leverage, margin_type
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id, snapshot_date, symbol, position_side)
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			entry_price = EXCLUDED.entry_price,
			mark_price = EXCLUDED.mark_price,
			unrealized_pnl = EXCLUDED.unrealized_pnl,
			mode = EXCLUDED.mode,
			client_order_id = EXCLUDED.client_order_id,
			leverage = EXCLUDED.leverage,
			margin_type = EXCLUDED.margin_type
		RETURNING id, created_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		snapshot.UserID,
		snapshot.SnapshotDate,
		snapshot.Symbol,
		snapshot.PositionSide,
		snapshot.Quantity,
		snapshot.EntryPrice,
		snapshot.MarkPrice,
		snapshot.UnrealizedPnL,
		snapshot.Mode,
		snapshot.ClientOrderID,
		snapshot.Leverage,
		snapshot.MarginType,
	).Scan(&snapshot.ID, &snapshot.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save position snapshot: %w", err)
	}

	return nil
}

// SaveDailyPositionSnapshots saves multiple position snapshots in a batch
// Uses a transaction for atomicity
func (r *Repository) SaveDailyPositionSnapshots(ctx context.Context, snapshots []DailyPositionSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO daily_position_snapshots (
			user_id, snapshot_date, symbol, position_side, quantity,
			entry_price, mark_price, unrealized_pnl, mode, client_order_id,
			leverage, margin_type
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (user_id, snapshot_date, symbol, position_side)
		DO UPDATE SET
			quantity = EXCLUDED.quantity,
			entry_price = EXCLUDED.entry_price,
			mark_price = EXCLUDED.mark_price,
			unrealized_pnl = EXCLUDED.unrealized_pnl,
			mode = EXCLUDED.mode,
			client_order_id = EXCLUDED.client_order_id,
			leverage = EXCLUDED.leverage,
			margin_type = EXCLUDED.margin_type
	`

	for _, snapshot := range snapshots {
		_, err := tx.Exec(ctx, query,
			snapshot.UserID,
			snapshot.SnapshotDate,
			snapshot.Symbol,
			snapshot.PositionSide,
			snapshot.Quantity,
			snapshot.EntryPrice,
			snapshot.MarkPrice,
			snapshot.UnrealizedPnL,
			snapshot.Mode,
			snapshot.ClientOrderID,
			snapshot.Leverage,
			snapshot.MarginType,
		)
		if err != nil {
			return fmt.Errorf("failed to save position snapshot for %s: %w", snapshot.Symbol, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetDailyPositionSnapshots retrieves position snapshots for a user on a specific date
func (r *Repository) GetDailyPositionSnapshots(ctx context.Context, userID string, snapshotDate time.Time) ([]DailyPositionSnapshot, error) {
	query := `
		SELECT id, user_id, snapshot_date, symbol, position_side, quantity,
			entry_price, mark_price, unrealized_pnl, mode, client_order_id,
			leverage, margin_type, created_at
		FROM daily_position_snapshots
		WHERE user_id = $1 AND snapshot_date = $2
		ORDER BY symbol, position_side
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, snapshotDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query position snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []DailyPositionSnapshot
	for rows.Next() {
		var s DailyPositionSnapshot
		err := rows.Scan(
			&s.ID, &s.UserID, &s.SnapshotDate, &s.Symbol, &s.PositionSide,
			&s.Quantity, &s.EntryPrice, &s.MarkPrice, &s.UnrealizedPnL,
			&s.Mode, &s.ClientOrderID, &s.Leverage, &s.MarginType, &s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position snapshot: %w", err)
		}
		snapshots = append(snapshots, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating position snapshots: %w", err)
	}

	return snapshots, nil
}

// GetDailyPositionSnapshotsDateRange retrieves position snapshots for a user within a date range
func (r *Repository) GetDailyPositionSnapshotsDateRange(ctx context.Context, userID string, startDate, endDate time.Time) ([]DailyPositionSnapshot, error) {
	query := `
		SELECT id, user_id, snapshot_date, symbol, position_side, quantity,
			entry_price, mark_price, unrealized_pnl, mode, client_order_id,
			leverage, margin_type, created_at
		FROM daily_position_snapshots
		WHERE user_id = $1 AND snapshot_date >= $2 AND snapshot_date <= $3
		ORDER BY snapshot_date, symbol, position_side
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query position snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []DailyPositionSnapshot
	for rows.Next() {
		var s DailyPositionSnapshot
		err := rows.Scan(
			&s.ID, &s.UserID, &s.SnapshotDate, &s.Symbol, &s.PositionSide,
			&s.Quantity, &s.EntryPrice, &s.MarkPrice, &s.UnrealizedPnL,
			&s.Mode, &s.ClientOrderID, &s.Leverage, &s.MarginType, &s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan position snapshot: %w", err)
		}
		snapshots = append(snapshots, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating position snapshots: %w", err)
	}

	return snapshots, nil
}

// GetModeBreakdownForDate retrieves P&L breakdown by mode for a user on a specific date
func (r *Repository) GetModeBreakdownForDate(ctx context.Context, userID string, snapshotDate time.Time) ([]ModeBreakdown, error) {
	query := `
		SELECT mode, COUNT(*) as position_count, SUM(unrealized_pnl) as total_pnl
		FROM daily_position_snapshots
		WHERE user_id = $1 AND snapshot_date = $2
		GROUP BY mode
		ORDER BY mode
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, snapshotDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query mode breakdown: %w", err)
	}
	defer rows.Close()

	var breakdowns []ModeBreakdown
	for rows.Next() {
		var mb ModeBreakdown
		err := rows.Scan(&mb.Mode, &mb.PositionCount, &mb.UnrealizedPnL)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mode breakdown: %w", err)
		}
		breakdowns = append(breakdowns, mb)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating mode breakdown: %w", err)
	}

	return breakdowns, nil
}

// ModeBreakdown represents P&L breakdown by trading mode
type ModeBreakdown struct {
	Mode          string  `json:"mode"`
	PositionCount int     `json:"position_count"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

// HasDailySnapshotForDate checks if a snapshot already exists for a user on a date
func (r *Repository) HasDailySnapshotForDate(ctx context.Context, userID string, snapshotDate time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM daily_position_snapshots WHERE user_id = $1 AND snapshot_date = $2)`
	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, snapshotDate).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check snapshot existence: %w", err)
	}
	return exists, nil
}

// DeleteDailySnapshotsForDate deletes all snapshots for a user on a specific date
// Used for re-running settlement or testing
func (r *Repository) DeleteDailySnapshotsForDate(ctx context.Context, userID string, snapshotDate time.Time) error {
	query := `DELETE FROM daily_position_snapshots WHERE user_id = $1 AND snapshot_date = $2`
	_, err := r.db.Pool.Exec(ctx, query, userID, snapshotDate)
	if err != nil {
		return fmt.Errorf("failed to delete snapshots: %w", err)
	}
	return nil
}

// GetLatestDailySnapshotDate returns the most recent snapshot date for a user
func (r *Repository) GetLatestDailySnapshotDate(ctx context.Context, userID string) (*time.Time, error) {
	query := `SELECT MAX(snapshot_date) FROM daily_position_snapshots WHERE user_id = $1`
	var latestDate *time.Time
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&latestDate)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to get latest snapshot date: %w", err)
	}
	return latestDate, nil
}
