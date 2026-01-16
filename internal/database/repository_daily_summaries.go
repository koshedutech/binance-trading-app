// Package database provides repository methods for daily mode summaries.
// Epic 8 Story 8.3: Daily Summary Storage
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// DailyModeSummary represents a daily P&L summary for a trading mode
type DailyModeSummary struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"user_id"`
	SummaryDate         time.Time  `json:"summary_date"`
	Mode                string     `json:"mode"`

	// Trade metrics
	TradeCount          int        `json:"trade_count"`
	WinCount            int        `json:"win_count"`
	LossCount           int        `json:"loss_count"`
	WinRate             float64    `json:"win_rate"`

	// P&L metrics
	RealizedPnL         float64    `json:"realized_pnl"`
	UnrealizedPnL       float64    `json:"unrealized_pnl"`
	UnrealizedPnLChange float64    `json:"unrealized_pnl_change"`
	TotalPnL            float64    `json:"total_pnl"`

	// Trade details
	LargestWin          float64    `json:"largest_win"`
	LargestLoss         float64    `json:"largest_loss"`
	TotalVolume         float64    `json:"total_volume"`
	AvgTradeSize        float64    `json:"avg_trade_size"`

	// Capital metrics
	StartingBalance     *float64   `json:"starting_balance,omitempty"`
	EndingBalance       *float64   `json:"ending_balance,omitempty"`
	MaxCapitalUsed      *float64   `json:"max_capital_used,omitempty"`
	AvgCapitalUsed      *float64   `json:"avg_capital_used,omitempty"`
	MaxDrawdown         *float64   `json:"max_drawdown,omitempty"`
	PeakBalance         *float64   `json:"peak_balance,omitempty"`

	// Fees
	TotalFees           float64    `json:"total_fees"`

	// Settlement metadata
	SettlementStatus    string     `json:"settlement_status"`
	SettlementError     *string    `json:"settlement_error,omitempty"`
	SettlementTime      time.Time  `json:"settlement_time"`
	UserTimezone        string     `json:"user_timezone"`

	// Data quality
	DataQualityFlag     bool       `json:"data_quality_flag"`
	DataQualityNotes    *string    `json:"data_quality_notes,omitempty"`
	ReviewedBy          *string    `json:"reviewed_by,omitempty"`
	ReviewedAt          *time.Time `json:"reviewed_at,omitempty"`
	Alerted             bool       `json:"alerted"`

	// Timestamps
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// SaveDailyModeSummary saves or updates a daily mode summary (upsert)
func (r *Repository) SaveDailyModeSummary(ctx context.Context, summary *DailyModeSummary) error {
	query := `
		INSERT INTO daily_mode_summaries (
			user_id, summary_date, mode,
			trade_count, win_count, loss_count, win_rate,
			realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
			largest_win, largest_loss, total_volume, avg_trade_size,
			starting_balance, ending_balance, max_capital_used, avg_capital_used,
			max_drawdown, peak_balance, total_fees,
			settlement_status, settlement_error, user_timezone,
			data_quality_flag, data_quality_notes
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, $7,
			$8, $9, $10, $11,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20, $21, $22,
			$23, $24, $25,
			$26, $27
		)
		ON CONFLICT (user_id, summary_date, mode)
		DO UPDATE SET
			trade_count = EXCLUDED.trade_count,
			win_count = EXCLUDED.win_count,
			loss_count = EXCLUDED.loss_count,
			win_rate = EXCLUDED.win_rate,
			realized_pnl = EXCLUDED.realized_pnl,
			unrealized_pnl = EXCLUDED.unrealized_pnl,
			unrealized_pnl_change = EXCLUDED.unrealized_pnl_change,
			total_pnl = EXCLUDED.total_pnl,
			largest_win = EXCLUDED.largest_win,
			largest_loss = EXCLUDED.largest_loss,
			total_volume = EXCLUDED.total_volume,
			avg_trade_size = EXCLUDED.avg_trade_size,
			starting_balance = EXCLUDED.starting_balance,
			ending_balance = EXCLUDED.ending_balance,
			max_capital_used = EXCLUDED.max_capital_used,
			avg_capital_used = EXCLUDED.avg_capital_used,
			max_drawdown = EXCLUDED.max_drawdown,
			peak_balance = EXCLUDED.peak_balance,
			total_fees = EXCLUDED.total_fees,
			settlement_status = EXCLUDED.settlement_status,
			settlement_error = EXCLUDED.settlement_error,
			data_quality_flag = EXCLUDED.data_quality_flag,
			data_quality_notes = EXCLUDED.data_quality_notes,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, created_at, updated_at, settlement_time
	`

	err := r.db.Pool.QueryRow(ctx, query,
		summary.UserID, summary.SummaryDate, summary.Mode,
		summary.TradeCount, summary.WinCount, summary.LossCount, summary.WinRate,
		summary.RealizedPnL, summary.UnrealizedPnL, summary.UnrealizedPnLChange, summary.TotalPnL,
		summary.LargestWin, summary.LargestLoss, summary.TotalVolume, summary.AvgTradeSize,
		summary.StartingBalance, summary.EndingBalance, summary.MaxCapitalUsed, summary.AvgCapitalUsed,
		summary.MaxDrawdown, summary.PeakBalance, summary.TotalFees,
		summary.SettlementStatus, summary.SettlementError, summary.UserTimezone,
		summary.DataQualityFlag, summary.DataQualityNotes,
	).Scan(&summary.ID, &summary.CreatedAt, &summary.UpdatedAt, &summary.SettlementTime)

	if err != nil {
		return fmt.Errorf("failed to save daily mode summary: %w", err)
	}

	return nil
}

// SaveDailyModeSummaries saves multiple summaries in a transaction
func (r *Repository) SaveDailyModeSummaries(ctx context.Context, summaries []DailyModeSummary) error {
	if len(summaries) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i := range summaries {
		summary := &summaries[i]
		query := `
			INSERT INTO daily_mode_summaries (
				user_id, summary_date, mode,
				trade_count, win_count, loss_count, win_rate,
				realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
				largest_win, largest_loss, total_volume, avg_trade_size,
				starting_balance, ending_balance, max_capital_used, avg_capital_used,
				max_drawdown, peak_balance, total_fees,
				settlement_status, settlement_error, user_timezone,
				data_quality_flag, data_quality_notes
			) VALUES (
				$1, $2, $3,
				$4, $5, $6, $7,
				$8, $9, $10, $11,
				$12, $13, $14, $15,
				$16, $17, $18, $19,
				$20, $21, $22,
				$23, $24, $25,
				$26, $27
			)
			ON CONFLICT (user_id, summary_date, mode)
			DO UPDATE SET
				trade_count = EXCLUDED.trade_count,
				win_count = EXCLUDED.win_count,
				loss_count = EXCLUDED.loss_count,
				win_rate = EXCLUDED.win_rate,
				realized_pnl = EXCLUDED.realized_pnl,
				unrealized_pnl = EXCLUDED.unrealized_pnl,
				unrealized_pnl_change = EXCLUDED.unrealized_pnl_change,
				total_pnl = EXCLUDED.total_pnl,
				largest_win = EXCLUDED.largest_win,
				largest_loss = EXCLUDED.largest_loss,
				total_volume = EXCLUDED.total_volume,
				avg_trade_size = EXCLUDED.avg_trade_size,
				starting_balance = EXCLUDED.starting_balance,
				ending_balance = EXCLUDED.ending_balance,
				max_capital_used = EXCLUDED.max_capital_used,
				avg_capital_used = EXCLUDED.avg_capital_used,
				max_drawdown = EXCLUDED.max_drawdown,
				peak_balance = EXCLUDED.peak_balance,
				total_fees = EXCLUDED.total_fees,
				settlement_status = EXCLUDED.settlement_status,
				settlement_error = EXCLUDED.settlement_error,
				data_quality_flag = EXCLUDED.data_quality_flag,
				data_quality_notes = EXCLUDED.data_quality_notes,
				updated_at = CURRENT_TIMESTAMP
		`

		_, err := tx.Exec(ctx, query,
			summary.UserID, summary.SummaryDate, summary.Mode,
			summary.TradeCount, summary.WinCount, summary.LossCount, summary.WinRate,
			summary.RealizedPnL, summary.UnrealizedPnL, summary.UnrealizedPnLChange, summary.TotalPnL,
			summary.LargestWin, summary.LargestLoss, summary.TotalVolume, summary.AvgTradeSize,
			summary.StartingBalance, summary.EndingBalance, summary.MaxCapitalUsed, summary.AvgCapitalUsed,
			summary.MaxDrawdown, summary.PeakBalance, summary.TotalFees,
			summary.SettlementStatus, summary.SettlementError, summary.UserTimezone,
			summary.DataQualityFlag, summary.DataQualityNotes,
		)
		if err != nil {
			return fmt.Errorf("failed to save summary for mode %s: %w", summary.Mode, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetDailyModeSummaries retrieves summaries for a user on a specific date
func (r *Repository) GetDailyModeSummaries(ctx context.Context, userID string, summaryDate time.Time) ([]DailyModeSummary, error) {
	query := `
		SELECT id, user_id, summary_date, mode,
			trade_count, win_count, loss_count, win_rate,
			realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
			largest_win, largest_loss, total_volume, avg_trade_size,
			starting_balance, ending_balance, max_capital_used, avg_capital_used,
			max_drawdown, peak_balance, total_fees,
			settlement_status, settlement_error, settlement_time, user_timezone,
			data_quality_flag, data_quality_notes, reviewed_by, reviewed_at, alerted,
			created_at, updated_at
		FROM daily_mode_summaries
		WHERE user_id = $1 AND summary_date = $2
		ORDER BY mode
	`

	return r.scanDailyModeSummaries(ctx, query, userID, summaryDate)
}

// GetDailyModeSummariesDateRange retrieves summaries for a user within a date range
func (r *Repository) GetDailyModeSummariesDateRange(ctx context.Context, userID string, startDate, endDate time.Time, mode string) ([]DailyModeSummary, error) {
	var query string
	var args []interface{}

	if mode != "" {
		query = `
			SELECT id, user_id, summary_date, mode,
				trade_count, win_count, loss_count, win_rate,
				realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
				largest_win, largest_loss, total_volume, avg_trade_size,
				starting_balance, ending_balance, max_capital_used, avg_capital_used,
				max_drawdown, peak_balance, total_fees,
				settlement_status, settlement_error, settlement_time, user_timezone,
				data_quality_flag, data_quality_notes, reviewed_by, reviewed_at, alerted,
				created_at, updated_at
			FROM daily_mode_summaries
			WHERE user_id = $1 AND summary_date >= $2 AND summary_date <= $3 AND mode = $4
			ORDER BY summary_date DESC, mode
		`
		args = []interface{}{userID, startDate, endDate, mode}
	} else {
		query = `
			SELECT id, user_id, summary_date, mode,
				trade_count, win_count, loss_count, win_rate,
				realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
				largest_win, largest_loss, total_volume, avg_trade_size,
				starting_balance, ending_balance, max_capital_used, avg_capital_used,
				max_drawdown, peak_balance, total_fees,
				settlement_status, settlement_error, settlement_time, user_timezone,
				data_quality_flag, data_quality_notes, reviewed_by, reviewed_at, alerted,
				created_at, updated_at
			FROM daily_mode_summaries
			WHERE user_id = $1 AND summary_date >= $2 AND summary_date <= $3
			ORDER BY summary_date DESC, mode
		`
		args = []interface{}{userID, startDate, endDate}
	}

	return r.scanDailyModeSummaries(ctx, query, args...)
}

// GetYesterdayUnrealizedPnL gets unrealized P&L from yesterday's snapshot for a specific mode
func (r *Repository) GetYesterdayUnrealizedPnL(ctx context.Context, userID string, yesterdayDate time.Time, mode string) (float64, error) {
	query := `
		SELECT COALESCE(unrealized_pnl, 0)
		FROM daily_mode_summaries
		WHERE user_id = $1 AND summary_date = $2 AND mode = $3
	`

	var unrealizedPnL float64
	err := r.db.Pool.QueryRow(ctx, query, userID, yesterdayDate, mode).Scan(&unrealizedPnL)
	if err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("failed to get yesterday's unrealized P&L: %w", err)
	}

	return unrealizedPnL, nil
}

// UpdateSettlementStatus updates the settlement status for a summary
// FIX: Now checks rows affected to detect silent failures (Issue #4)
func (r *Repository) UpdateSettlementStatus(ctx context.Context, userID string, summaryDate time.Time, mode, status string, errorMsg *string) error {
	query := `
		UPDATE daily_mode_summaries
		SET settlement_status = $4, settlement_error = $5, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND summary_date = $2 AND mode = $3
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, summaryDate, mode, status, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update settlement status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no summary found for user %s, date %s, mode %s", userID, summaryDate.Format("2006-01-02"), mode)
	}

	return nil
}

// MarkSettlementAlerted marks that an alert has been sent for a failed settlement
// FIX: Now checks rows affected to detect silent failures (Issue #4)
func (r *Repository) MarkSettlementAlerted(ctx context.Context, userID string, summaryDate time.Time) error {
	query := `
		UPDATE daily_mode_summaries
		SET alerted = true, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND summary_date = $2
	`

	result, err := r.db.Pool.Exec(ctx, query, userID, summaryDate)
	if err != nil {
		return fmt.Errorf("failed to mark settlement as alerted: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no summaries found for user %s, date %s", userID, summaryDate.Format("2006-01-02"))
	}

	return nil
}

// GetFailedSettlements gets all failed settlements that haven't been alerted
func (r *Repository) GetFailedSettlements(ctx context.Context, olderThan time.Duration) ([]DailyModeSummary, error) {
	query := `
		SELECT DISTINCT ON (user_id, summary_date)
			id, user_id, summary_date, mode,
			trade_count, win_count, loss_count, win_rate,
			realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
			largest_win, largest_loss, total_volume, avg_trade_size,
			starting_balance, ending_balance, max_capital_used, avg_capital_used,
			max_drawdown, peak_balance, total_fees,
			settlement_status, settlement_error, settlement_time, user_timezone,
			data_quality_flag, data_quality_notes, reviewed_by, reviewed_at, alerted,
			created_at, updated_at
		FROM daily_mode_summaries
		WHERE settlement_status = 'failed'
			AND alerted = false
			AND settlement_time < $1
		ORDER BY user_id, summary_date, mode
	`

	cutoffTime := time.Now().Add(-olderThan)
	return r.scanDailyModeSummaries(ctx, query, cutoffTime)
}

// GetReviewQueue gets all summaries flagged for data quality review
func (r *Repository) GetReviewQueue(ctx context.Context, limit, offset int) ([]DailyModeSummary, int, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM daily_mode_summaries
		WHERE data_quality_flag = true AND reviewed_at IS NULL
	`

	var totalCount int
	err := r.db.Pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count review queue: %w", err)
	}

	query := `
		SELECT id, user_id, summary_date, mode,
			trade_count, win_count, loss_count, win_rate,
			realized_pnl, unrealized_pnl, unrealized_pnl_change, total_pnl,
			largest_win, largest_loss, total_volume, avg_trade_size,
			starting_balance, ending_balance, max_capital_used, avg_capital_used,
			max_drawdown, peak_balance, total_fees,
			settlement_status, settlement_error, settlement_time, user_timezone,
			data_quality_flag, data_quality_notes, reviewed_by, reviewed_at, alerted,
			created_at, updated_at
		FROM daily_mode_summaries
		WHERE data_quality_flag = true AND reviewed_at IS NULL
		ORDER BY summary_date DESC
		LIMIT $1 OFFSET $2
	`

	summaries, err := r.scanDailyModeSummaries(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	return summaries, totalCount, nil
}

// ApproveSummary marks a summary as reviewed and approved
// FIX: Now checks rows affected to detect silent failures (Issue #4)
func (r *Repository) ApproveSummary(ctx context.Context, summaryID string, reviewerID string) error {
	query := `
		UPDATE daily_mode_summaries
		SET reviewed_by = $2, reviewed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query, summaryID, reviewerID)
	if err != nil {
		return fmt.Errorf("failed to approve summary: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("summary not found: %s", summaryID)
	}

	return nil
}

// Helper function to scan daily mode summaries from rows
func (r *Repository) scanDailyModeSummaries(ctx context.Context, query string, args ...interface{}) ([]DailyModeSummary, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily mode summaries: %w", err)
	}
	defer rows.Close()

	var summaries []DailyModeSummary
	for rows.Next() {
		var s DailyModeSummary
		err := rows.Scan(
			&s.ID, &s.UserID, &s.SummaryDate, &s.Mode,
			&s.TradeCount, &s.WinCount, &s.LossCount, &s.WinRate,
			&s.RealizedPnL, &s.UnrealizedPnL, &s.UnrealizedPnLChange, &s.TotalPnL,
			&s.LargestWin, &s.LargestLoss, &s.TotalVolume, &s.AvgTradeSize,
			&s.StartingBalance, &s.EndingBalance, &s.MaxCapitalUsed, &s.AvgCapitalUsed,
			&s.MaxDrawdown, &s.PeakBalance, &s.TotalFees,
			&s.SettlementStatus, &s.SettlementError, &s.SettlementTime, &s.UserTimezone,
			&s.DataQualityFlag, &s.DataQualityNotes, &s.ReviewedBy, &s.ReviewedAt, &s.Alerted,
			&s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan daily mode summary: %w", err)
		}
		summaries = append(summaries, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating daily mode summaries: %w", err)
	}

	return summaries, nil
}

// GetAdminDailySummaries retrieves all users' summaries for admin dashboard
// Supports filtering by date range, user, mode, and pagination
type AdminSummaryFilter struct {
	StartDate  time.Time
	EndDate    time.Time
	UserID     string
	Mode       string
	Status     string
	SortBy     string
	SortOrder  string
	Limit      int
	Offset     int
}

type AdminSummaryResult struct {
	Summaries   []DailyModeSummary `json:"summaries"`
	TotalCount  int                `json:"total_count"`
	TotalPnL    float64            `json:"total_pnl"`
	TotalTrades int                `json:"total_trades"`
	TotalFees   float64            `json:"total_fees"`
	AvgWinRate  float64            `json:"avg_win_rate"`
}

func (r *Repository) GetAdminDailySummaries(ctx context.Context, filter AdminSummaryFilter) (*AdminSummaryResult, error) {
	// Build dynamic query
	baseQuery := `
		SELECT d.id, d.user_id, d.summary_date, d.mode,
			d.trade_count, d.win_count, d.loss_count, d.win_rate,
			d.realized_pnl, d.unrealized_pnl, d.unrealized_pnl_change, d.total_pnl,
			d.largest_win, d.largest_loss, d.total_volume, d.avg_trade_size,
			d.starting_balance, d.ending_balance, d.max_capital_used, d.avg_capital_used,
			d.max_drawdown, d.peak_balance, d.total_fees,
			d.settlement_status, d.settlement_error, d.settlement_time, d.user_timezone,
			d.data_quality_flag, d.data_quality_notes, d.reviewed_by, d.reviewed_at, d.alerted,
			d.created_at, d.updated_at
		FROM daily_mode_summaries d
		WHERE d.summary_date >= $1 AND d.summary_date <= $2
	`

	countQuery := `
		SELECT COUNT(*),
			COALESCE(SUM(total_pnl), 0),
			COALESCE(SUM(trade_count), 0),
			COALESCE(SUM(total_fees), 0),
			COALESCE(AVG(win_rate), 0)
		FROM daily_mode_summaries d
		WHERE d.summary_date >= $1 AND d.summary_date <= $2
	`

	args := []interface{}{filter.StartDate, filter.EndDate}
	argCount := 2

	// Add optional filters
	if filter.UserID != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND d.user_id = $%d", argCount)
		countQuery += fmt.Sprintf(" AND d.user_id = $%d", argCount)
		args = append(args, filter.UserID)
	}

	if filter.Mode != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND d.mode = $%d", argCount)
		countQuery += fmt.Sprintf(" AND d.mode = $%d", argCount)
		args = append(args, filter.Mode)
	}

	if filter.Status != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND d.settlement_status = $%d", argCount)
		countQuery += fmt.Sprintf(" AND d.settlement_status = $%d", argCount)
		args = append(args, filter.Status)
	}

	// Get totals
	result := &AdminSummaryResult{}
	err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(
		&result.TotalCount, &result.TotalPnL, &result.TotalTrades, &result.TotalFees, &result.AvgWinRate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin summary totals: %w", err)
	}

	// Add sorting
	sortColumn := "summary_date"
	if filter.SortBy != "" {
		// Whitelist valid sort columns
		validColumns := map[string]bool{
			"summary_date": true, "total_pnl": true, "trade_count": true,
			"win_rate": true, "total_fees": true, "mode": true,
		}
		if validColumns[filter.SortBy] {
			sortColumn = filter.SortBy
		}
	}
	sortOrder := "DESC"
	if filter.SortOrder == "asc" {
		sortOrder = "ASC"
	}
	baseQuery += fmt.Sprintf(" ORDER BY d.%s %s", sortColumn, sortOrder)

	// Add pagination
	if filter.Limit > 0 {
		argCount++
		baseQuery += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		argCount++
		baseQuery += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	// Get summaries
	result.Summaries, err = r.scanDailyModeSummaries(ctx, baseQuery, args...)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetWeeklySummary retrieves weekly aggregated summaries for a user
func (r *Repository) GetWeeklySummary(ctx context.Context, userID string, startDate, endDate time.Time) ([]PeriodSummary, error) {
	query := `
		SELECT
			DATE_TRUNC('week', summary_date) as period_start,
			SUM(trade_count) as total_trades,
			SUM(win_count) as total_wins,
			SUM(loss_count) as total_losses,
			CASE WHEN SUM(trade_count) > 0
				THEN SUM(win_count)::float / SUM(trade_count) * 100
				ELSE 0
			END as win_rate,
			SUM(realized_pnl) as realized_pnl,
			SUM(total_pnl) as total_pnl,
			SUM(total_volume) as total_volume,
			SUM(total_fees) as total_fees
		FROM daily_mode_summaries
		WHERE user_id = $1
			AND summary_date >= $2
			AND summary_date <= $3
			AND mode = 'ALL'
		GROUP BY DATE_TRUNC('week', summary_date)
		ORDER BY period_start DESC
	`

	return r.scanPeriodSummaries(ctx, query, userID, startDate, endDate)
}

// GetMonthlySummary retrieves monthly aggregated summaries for a user
func (r *Repository) GetMonthlySummary(ctx context.Context, userID string, startDate, endDate time.Time) ([]PeriodSummary, error) {
	query := `
		SELECT
			DATE_TRUNC('month', summary_date) as period_start,
			SUM(trade_count) as total_trades,
			SUM(win_count) as total_wins,
			SUM(loss_count) as total_losses,
			CASE WHEN SUM(trade_count) > 0
				THEN SUM(win_count)::float / SUM(trade_count) * 100
				ELSE 0
			END as win_rate,
			SUM(realized_pnl) as realized_pnl,
			SUM(total_pnl) as total_pnl,
			SUM(total_volume) as total_volume,
			SUM(total_fees) as total_fees
		FROM daily_mode_summaries
		WHERE user_id = $1
			AND summary_date >= $2
			AND summary_date <= $3
			AND mode = 'ALL'
		GROUP BY DATE_TRUNC('month', summary_date)
		ORDER BY period_start DESC
	`

	return r.scanPeriodSummaries(ctx, query, userID, startDate, endDate)
}

// PeriodSummary represents aggregated summary for a time period
type PeriodSummary struct {
	PeriodStart  time.Time `json:"period_start"`
	TotalTrades  int       `json:"total_trades"`
	TotalWins    int       `json:"total_wins"`
	TotalLosses  int       `json:"total_losses"`
	WinRate      float64   `json:"win_rate"`
	RealizedPnL  float64   `json:"realized_pnl"`
	TotalPnL     float64   `json:"total_pnl"`
	TotalVolume  float64   `json:"total_volume"`
	TotalFees    float64   `json:"total_fees"`
}

func (r *Repository) scanPeriodSummaries(ctx context.Context, query string, args ...interface{}) ([]PeriodSummary, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query period summaries: %w", err)
	}
	defer rows.Close()

	var summaries []PeriodSummary
	for rows.Next() {
		var s PeriodSummary
		err := rows.Scan(
			&s.PeriodStart, &s.TotalTrades, &s.TotalWins, &s.TotalLosses,
			&s.WinRate, &s.RealizedPnL, &s.TotalPnL, &s.TotalVolume, &s.TotalFees,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan period summary: %w", err)
		}
		summaries = append(summaries, s)
	}

	return summaries, nil
}

// GetModeComparison retrieves side-by-side mode performance for a date range
func (r *Repository) GetModeComparison(ctx context.Context, userID string, startDate, endDate time.Time) ([]ModeComparisonResult, error) {
	query := `
		SELECT
			mode,
			SUM(trade_count) as total_trades,
			SUM(win_count) as total_wins,
			SUM(loss_count) as total_losses,
			CASE WHEN SUM(trade_count) > 0
				THEN SUM(win_count)::float / SUM(trade_count) * 100
				ELSE 0
			END as win_rate,
			SUM(realized_pnl) as realized_pnl,
			SUM(total_pnl) as total_pnl,
			SUM(total_volume) as total_volume,
			SUM(total_fees) as total_fees
		FROM daily_mode_summaries
		WHERE user_id = $1
			AND summary_date >= $2
			AND summary_date <= $3
			AND mode != 'ALL'
		GROUP BY mode
		ORDER BY total_pnl DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query mode comparison: %w", err)
	}
	defer rows.Close()

	var results []ModeComparisonResult
	for rows.Next() {
		var r ModeComparisonResult
		err := rows.Scan(
			&r.Mode, &r.TotalTrades, &r.TotalWins, &r.TotalLosses,
			&r.WinRate, &r.RealizedPnL, &r.TotalPnL, &r.TotalVolume, &r.TotalFees,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mode comparison: %w", err)
		}
		results = append(results, r)
	}

	return results, nil
}

// ModeComparisonResult represents aggregated comparison data per mode
type ModeComparisonResult struct {
	Mode        string  `json:"mode"`
	TotalTrades int     `json:"total_trades"`
	TotalWins   int     `json:"total_wins"`
	TotalLosses int     `json:"total_losses"`
	WinRate     float64 `json:"win_rate"`
	RealizedPnL float64 `json:"realized_pnl"`
	TotalPnL    float64 `json:"total_pnl"`
	TotalVolume float64 `json:"total_volume"`
	TotalFees   float64 `json:"total_fees"`
}
