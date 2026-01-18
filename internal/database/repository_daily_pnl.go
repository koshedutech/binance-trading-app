// Package database provides repository methods for daily P&L summaries.
// Epic 13 Story 13.1: PNL Summary Caching & Historical Navigation
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// GetUserDailyPnL retrieves a single daily P&L summary for a user and date
// Returns nil, nil if no record exists (not an error condition)
func (r *Repository) GetUserDailyPnL(ctx context.Context, userID string, date time.Time) (*UserDailyPnLSummary, error) {
	query := `
		SELECT id, user_id, date, pnl, commission, funding, net_pnl,
			   trade_count, rebate, other, fetched_at, created_at
		FROM user_daily_pnl_summaries
		WHERE user_id = $1 AND date = $2
	`

	var s UserDailyPnLSummary
	err := r.db.Pool.QueryRow(ctx, query, userID, date).Scan(
		&s.ID, &s.UserID, &s.Date, &s.PnL, &s.Commission, &s.Funding, &s.NetPnL,
		&s.TradeCount, &s.Rebate, &s.Other, &s.FetchedAt, &s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("failed to get user daily pnl: %w", err)
	}

	return &s, nil
}

// GetUserDailyPnLRange retrieves daily P&L summaries for a user within a date range
// Results are ordered by date descending (most recent first)
func (r *Repository) GetUserDailyPnLRange(ctx context.Context, userID string, startDate, endDate time.Time) ([]UserDailyPnLSummary, error) {
	query := `
		SELECT id, user_id, date, pnl, commission, funding, net_pnl,
			   trade_count, rebate, other, fetched_at, created_at
		FROM user_daily_pnl_summaries
		WHERE user_id = $1 AND date >= $2 AND date <= $3
		ORDER BY date DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query user daily pnl range: %w", err)
	}
	defer rows.Close()

	var summaries []UserDailyPnLSummary
	for rows.Next() {
		// Check for context cancellation during iteration
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var s UserDailyPnLSummary
		err := rows.Scan(
			&s.ID, &s.UserID, &s.Date, &s.PnL, &s.Commission, &s.Funding, &s.NetPnL,
			&s.TradeCount, &s.Rebate, &s.Other, &s.FetchedAt, &s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user daily pnl: %w", err)
		}
		summaries = append(summaries, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user daily pnl rows: %w", err)
	}

	return summaries, nil
}

// SaveUserDailyPnL saves a daily P&L summary (upsert)
// If a record exists for this user/date, it will be updated
func (r *Repository) SaveUserDailyPnL(ctx context.Context, summary *UserDailyPnLSummary) error {
	query := `
		INSERT INTO user_daily_pnl_summaries (
			user_id, date, pnl, commission, funding, net_pnl,
			trade_count, rebate, other, fetched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, date)
		DO UPDATE SET
			pnl = EXCLUDED.pnl,
			commission = EXCLUDED.commission,
			funding = EXCLUDED.funding,
			net_pnl = EXCLUDED.net_pnl,
			trade_count = EXCLUDED.trade_count,
			rebate = EXCLUDED.rebate,
			other = EXCLUDED.other,
			fetched_at = EXCLUDED.fetched_at
		RETURNING id, created_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		summary.UserID, summary.Date, summary.PnL, summary.Commission,
		summary.Funding, summary.NetPnL, summary.TradeCount,
		summary.Rebate, summary.Other, summary.FetchedAt,
	).Scan(&summary.ID, &summary.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to save user daily pnl: %w", err)
	}

	return nil
}

// BulkSaveUserDailyPnL saves multiple daily P&L summaries in a transaction
func (r *Repository) BulkSaveUserDailyPnL(ctx context.Context, summaries []UserDailyPnLSummary) error {
	if len(summaries) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO user_daily_pnl_summaries (
			user_id, date, pnl, commission, funding, net_pnl,
			trade_count, rebate, other, fetched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, date)
		DO UPDATE SET
			pnl = EXCLUDED.pnl,
			commission = EXCLUDED.commission,
			funding = EXCLUDED.funding,
			net_pnl = EXCLUDED.net_pnl,
			trade_count = EXCLUDED.trade_count,
			rebate = EXCLUDED.rebate,
			other = EXCLUDED.other,
			fetched_at = EXCLUDED.fetched_at
	`

	for _, summary := range summaries {
		_, err := tx.Exec(ctx, query,
			summary.UserID, summary.Date, summary.PnL, summary.Commission,
			summary.Funding, summary.NetPnL, summary.TradeCount,
			summary.Rebate, summary.Other, summary.FetchedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to save summary for date %s: %w", summary.Date.Format("2006-01-02"), err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetUserDailyPnLMultipleDates retrieves P&L summaries for specific dates
// Returns a map of date string -> summary for easy lookup
func (r *Repository) GetUserDailyPnLMultipleDates(ctx context.Context, userID string, dates []time.Time) (map[string]*UserDailyPnLSummary, error) {
	if len(dates) == 0 {
		return make(map[string]*UserDailyPnLSummary), nil
	}

	// Build date list for IN clause
	dateStrings := make([]interface{}, len(dates))
	placeholders := ""
	for i, d := range dates {
		dateStrings[i] = d.Format("2006-01-02")
		if i > 0 {
			placeholders += ", "
		}
		placeholders += fmt.Sprintf("$%d::date", i+2)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, date, pnl, commission, funding, net_pnl,
			   trade_count, rebate, other, fetched_at, created_at
		FROM user_daily_pnl_summaries
		WHERE user_id = $1 AND date IN (%s)
	`, placeholders)

	args := make([]interface{}, 0, len(dates)+1)
	args = append(args, userID)
	args = append(args, dateStrings...)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query multiple dates: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*UserDailyPnLSummary)
	for rows.Next() {
		var s UserDailyPnLSummary
		err := rows.Scan(
			&s.ID, &s.UserID, &s.Date, &s.PnL, &s.Commission, &s.Funding, &s.NetPnL,
			&s.TradeCount, &s.Rebate, &s.Other, &s.FetchedAt, &s.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		result[s.Date.Format("2006-01-02")] = &s
	}

	return result, nil
}

// DeleteUserDailyPnL deletes a specific day's P&L record (for testing/admin)
func (r *Repository) DeleteUserDailyPnL(ctx context.Context, userID string, date time.Time) error {
	query := `
		DELETE FROM user_daily_pnl_summaries
		WHERE user_id = $1 AND date = $2
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, date)
	if err != nil {
		return fmt.Errorf("failed to delete user daily pnl: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no record found for user %s date %s", userID, date.Format("2006-01-02"))
	}

	return nil
}

// GetUserPnLTotalsForRange calculates aggregated P&L totals for a date range
func (r *Repository) GetUserPnLTotalsForRange(ctx context.Context, userID string, startDate, endDate time.Time) (*PnLTotals, error) {
	query := `
		SELECT
			COALESCE(SUM(pnl), 0) as total_pnl,
			COALESCE(SUM(commission), 0) as total_commission,
			COALESCE(SUM(funding), 0) as total_funding,
			COALESCE(SUM(net_pnl), 0) as total_net_pnl,
			COALESCE(SUM(trade_count), 0) as total_trades,
			COUNT(*) as days_count
		FROM user_daily_pnl_summaries
		WHERE user_id = $1 AND date >= $2 AND date <= $3
	`

	var totals PnLTotals
	err := r.db.Pool.QueryRow(ctx, query, userID, startDate, endDate).Scan(
		&totals.TotalPnL, &totals.TotalCommission, &totals.TotalFunding,
		&totals.TotalNetPnL, &totals.TotalTrades, &totals.DaysCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get pnl totals: %w", err)
	}

	return &totals, nil
}

// PnLTotals holds aggregated P&L data for a period
type PnLTotals struct {
	TotalPnL        float64 `json:"total_pnl"`
	TotalCommission float64 `json:"total_commission"`
	TotalFunding    float64 `json:"total_funding"`
	TotalNetPnL     float64 `json:"total_net_pnl"`
	TotalTrades     int     `json:"total_trades"`
	DaysCount       int     `json:"days_count"`
}
