package database

import (
	"context"
	"fmt"
	"time"
)

// BacktestResult represents a backtest execution result
type BacktestResult struct {
	ID                       int64     `json:"id"`
	StrategyConfigID         int64     `json:"strategy_config_id"`
	Symbol                   string    `json:"symbol"`
	Interval                 string    `json:"interval"`
	StartDate                time.Time `json:"start_date"`
	EndDate                  time.Time `json:"end_date"`
	TotalTrades              int       `json:"total_trades"`
	WinningTrades            int       `json:"winning_trades"`
	LosingTrades             int       `json:"losing_trades"`
	WinRate                  float64   `json:"win_rate"`
	TotalPnL                 float64   `json:"total_pnl"`
	TotalFees                float64   `json:"total_fees"`
	NetPnL                   float64   `json:"net_pnl"`
	AverageWin               float64   `json:"average_win"`
	AverageLoss              float64   `json:"average_loss"`
	LargestWin               float64   `json:"largest_win"`
	LargestLoss              float64   `json:"largest_loss"`
	ProfitFactor             float64   `json:"profit_factor"`
	MaxDrawdown              float64   `json:"max_drawdown"`
	MaxDrawdownPercent       float64   `json:"max_drawdown_percent"`
	AvgTradeDurationMinutes  int       `json:"avg_trade_duration_minutes"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

// BacktestTrade represents a single trade from a backtest
type BacktestTrade struct {
	ID               int64     `json:"id"`
	BacktestResultID int64     `json:"backtest_result_id"`
	EntryTime        time.Time `json:"entry_time"`
	EntryPrice       float64   `json:"entry_price"`
	EntryReason      string    `json:"entry_reason"`
	ExitTime         time.Time `json:"exit_time"`
	ExitPrice        float64   `json:"exit_price"`
	ExitReason       string    `json:"exit_reason"`
	Quantity         float64   `json:"quantity"`
	Side             string    `json:"side"`
	PnL              float64   `json:"pnl"`
	PnLPercent       float64   `json:"pnl_percent"`
	Fees             float64   `json:"fees"`
	DurationMinutes  int       `json:"duration_minutes"`
	CreatedAt        time.Time `json:"created_at"`
}

// SaveBacktestResult saves a backtest result and its trades in a transaction
func (r *Repository) SaveBacktestResult(ctx context.Context, result *BacktestResult, trades []BacktestTrade) (int64, error) {
	// Start transaction
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert backtest result
	query := `
		INSERT INTO backtest_results (
			strategy_config_id, symbol, interval, start_date, end_date,
			total_trades, winning_trades, losing_trades, win_rate,
			total_pnl, total_fees, net_pnl,
			average_win, average_loss, largest_win, largest_loss,
			profit_factor, max_drawdown, max_drawdown_percent,
			avg_trade_duration_minutes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id
	`

	var resultID int64
	err = tx.QueryRow(ctx, query,
		result.StrategyConfigID, result.Symbol, result.Interval, result.StartDate, result.EndDate,
		result.TotalTrades, result.WinningTrades, result.LosingTrades, result.WinRate,
		result.TotalPnL, result.TotalFees, result.NetPnL,
		result.AverageWin, result.AverageLoss, result.LargestWin, result.LargestLoss,
		result.ProfitFactor, result.MaxDrawdown, result.MaxDrawdownPercent,
		result.AvgTradeDurationMinutes,
	).Scan(&resultID)

	if err != nil {
		return 0, fmt.Errorf("failed to insert backtest result: %w", err)
	}

	// Insert trades
	if len(trades) > 0 {
		tradeQuery := `
			INSERT INTO backtest_trades (
				backtest_result_id, entry_time, entry_price, entry_reason,
				exit_time, exit_price, exit_reason, quantity, side,
				pnl, pnl_percent, fees, duration_minutes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		`

		for _, trade := range trades {
			_, err = tx.Exec(ctx, tradeQuery,
				resultID, trade.EntryTime, trade.EntryPrice, trade.EntryReason,
				trade.ExitTime, trade.ExitPrice, trade.ExitReason, trade.Quantity, trade.Side,
				trade.PnL, trade.PnLPercent, trade.Fees, trade.DurationMinutes,
			)

			if err != nil {
				return 0, fmt.Errorf("failed to insert backtest trade: %w", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return resultID, nil
}

// GetBacktestResults retrieves backtest results for a strategy config
func (r *Repository) GetBacktestResults(ctx context.Context, strategyConfigID int64, limit int) ([]BacktestResult, error) {
	query := `
		SELECT id, strategy_config_id, symbol, interval, start_date, end_date,
			   total_trades, winning_trades, losing_trades, win_rate,
			   total_pnl, total_fees, net_pnl,
			   average_win, average_loss, largest_win, largest_loss,
			   profit_factor, max_drawdown, max_drawdown_percent,
			   avg_trade_duration_minutes, created_at, updated_at
		FROM backtest_results
		WHERE strategy_config_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := r.db.Pool.Query(ctx, query, strategyConfigID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query backtest results: %w", err)
	}
	defer rows.Close()

	results := []BacktestResult{}
	for rows.Next() {
		var result BacktestResult
		err := rows.Scan(
			&result.ID, &result.StrategyConfigID, &result.Symbol, &result.Interval,
			&result.StartDate, &result.EndDate,
			&result.TotalTrades, &result.WinningTrades, &result.LosingTrades, &result.WinRate,
			&result.TotalPnL, &result.TotalFees, &result.NetPnL,
			&result.AverageWin, &result.AverageLoss, &result.LargestWin, &result.LargestLoss,
			&result.ProfitFactor, &result.MaxDrawdown, &result.MaxDrawdownPercent,
			&result.AvgTradeDurationMinutes, &result.CreatedAt, &result.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backtest result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating backtest results: %w", err)
	}

	return results, nil
}

// GetBacktestTrades retrieves trades for a specific backtest result
func (r *Repository) GetBacktestTrades(ctx context.Context, backtestResultID int64) ([]BacktestTrade, error) {
	query := `
		SELECT id, backtest_result_id, entry_time, entry_price, entry_reason,
			   exit_time, exit_price, exit_reason, quantity, side,
			   pnl, pnl_percent, fees, duration_minutes, created_at
		FROM backtest_trades
		WHERE backtest_result_id = $1
		ORDER BY entry_time ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, backtestResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to query backtest trades: %w", err)
	}
	defer rows.Close()

	trades := []BacktestTrade{}
	for rows.Next() {
		var trade BacktestTrade
		err := rows.Scan(
			&trade.ID, &trade.BacktestResultID, &trade.EntryTime, &trade.EntryPrice, &trade.EntryReason,
			&trade.ExitTime, &trade.ExitPrice, &trade.ExitReason, &trade.Quantity, &trade.Side,
			&trade.PnL, &trade.PnLPercent, &trade.Fees, &trade.DurationMinutes, &trade.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan backtest trade: %w", err)
		}
		trades = append(trades, trade)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating backtest trades: %w", err)
	}

	return trades, nil
}

// GetBacktestResult retrieves a single backtest result by ID
func (r *Repository) GetBacktestResult(ctx context.Context, id int64) (*BacktestResult, error) {
	query := `
		SELECT id, strategy_config_id, symbol, interval, start_date, end_date,
			   total_trades, winning_trades, losing_trades, win_rate,
			   total_pnl, total_fees, net_pnl,
			   average_win, average_loss, largest_win, largest_loss,
			   profit_factor, max_drawdown, max_drawdown_percent,
			   avg_trade_duration_minutes, created_at, updated_at
		FROM backtest_results
		WHERE id = $1
	`

	var result BacktestResult
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&result.ID, &result.StrategyConfigID, &result.Symbol, &result.Interval,
		&result.StartDate, &result.EndDate,
		&result.TotalTrades, &result.WinningTrades, &result.LosingTrades, &result.WinRate,
		&result.TotalPnL, &result.TotalFees, &result.NetPnL,
		&result.AverageWin, &result.AverageLoss, &result.LargestWin, &result.LargestLoss,
		&result.ProfitFactor, &result.MaxDrawdown, &result.MaxDrawdownPercent,
		&result.AvgTradeDurationMinutes, &result.CreatedAt, &result.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get backtest result: %w", err)
	}

	return &result, nil
}
