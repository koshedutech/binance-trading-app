package autopilot

import (
	"context"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
)

// StrategyPerformance represents performance metrics for a single strategy
type StrategyPerformance struct {
	StrategyID    int64     `json:"strategy_id"`
	StrategyName  string    `json:"strategy_name"`
	TotalTrades   int       `json:"total_trades"`
	WinningTrades int       `json:"winning_trades"`
	LosingTrades  int       `json:"losing_trades"`
	TotalPnL      float64   `json:"total_pnl"`
	WinRate       float64   `json:"win_rate"`
	AvgPnL        float64   `json:"avg_pnl"`
	AvgWin        float64   `json:"avg_win"`
	AvgLoss       float64   `json:"avg_loss"`
	LargestWin    float64   `json:"largest_win"`
	LargestLoss   float64   `json:"largest_loss"`
	LastTradeTime time.Time `json:"last_trade_time"`
}

// SourcePerformance represents aggregated performance by trade source (AI vs Strategy)
type SourcePerformance struct {
	Source        string  `json:"source"` // "ai" or "strategy"
	TotalTrades   int     `json:"total_trades"`
	WinningTrades int     `json:"winning_trades"`
	TotalPnL      float64 `json:"total_pnl"`
	WinRate       float64 `json:"win_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
}

// StrategyStatsManager handles strategy performance tracking
type StrategyStatsManager struct {
	db     *database.Repository
	logger *logging.Logger
}

// NewStrategyStatsManager creates a new strategy stats manager
func NewStrategyStatsManager(db *database.Repository, logger *logging.Logger) *StrategyStatsManager {
	return &StrategyStatsManager{
		db:     db,
		logger: logger,
	}
}

// GetStrategyPerformance returns performance metrics for all strategies that have traded
func (sm *StrategyStatsManager) GetStrategyPerformance(ctx context.Context) ([]StrategyPerformance, error) {
	if sm.db == nil {
		return nil, nil
	}

	dbInstance := sm.db.GetDB()
	if dbInstance == nil || dbInstance.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT
			strategy_id,
			strategy_name,
			COUNT(*) as total_trades,
			SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END) as winning_trades,
			SUM(CASE WHEN realized_pnl <= 0 THEN 1 ELSE 0 END) as losing_trades,
			COALESCE(SUM(realized_pnl), 0) as total_pnl,
			COALESCE(AVG(realized_pnl), 0) as avg_pnl,
			COALESCE(AVG(CASE WHEN realized_pnl > 0 THEN realized_pnl END), 0) as avg_win,
			COALESCE(AVG(CASE WHEN realized_pnl < 0 THEN realized_pnl END), 0) as avg_loss,
			COALESCE(MAX(realized_pnl), 0) as largest_win,
			COALESCE(MIN(realized_pnl), 0) as largest_loss,
			MAX(exit_time) as last_trade_time
		FROM futures_trades
		WHERE strategy_id IS NOT NULL
			AND status = 'CLOSED'
			AND realized_pnl IS NOT NULL
		GROUP BY strategy_id, strategy_name
		ORDER BY total_pnl DESC`

	rows, err := dbInstance.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var performances []StrategyPerformance
	for rows.Next() {
		var perf StrategyPerformance
		var lastTradeTime *time.Time

		err := rows.Scan(
			&perf.StrategyID,
			&perf.StrategyName,
			&perf.TotalTrades,
			&perf.WinningTrades,
			&perf.LosingTrades,
			&perf.TotalPnL,
			&perf.AvgPnL,
			&perf.AvgWin,
			&perf.AvgLoss,
			&perf.LargestWin,
			&perf.LargestLoss,
			&lastTradeTime,
		)
		if err != nil {
			sm.logger.Error("Failed to scan strategy performance", "error", err)
			continue
		}

		if lastTradeTime != nil {
			perf.LastTradeTime = *lastTradeTime
		}

		// Calculate win rate
		if perf.TotalTrades > 0 {
			perf.WinRate = float64(perf.WinningTrades) / float64(perf.TotalTrades) * 100
		}

		performances = append(performances, perf)
	}

	return performances, nil
}

// GetSourcePerformance returns aggregated performance by trade source (AI vs Strategy)
func (sm *StrategyStatsManager) GetSourcePerformance(ctx context.Context) ([]SourcePerformance, error) {
	if sm.db == nil {
		return nil, nil
	}

	dbInstance := sm.db.GetDB()
	if dbInstance == nil || dbInstance.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT
			COALESCE(trade_source, 'unknown') as source,
			COUNT(*) as total_trades,
			SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END) as winning_trades,
			COALESCE(SUM(realized_pnl), 0) as total_pnl,
			COALESCE(AVG(realized_pnl), 0) as avg_pnl
		FROM futures_trades
		WHERE status = 'CLOSED'
			AND realized_pnl IS NOT NULL
		GROUP BY trade_source
		ORDER BY total_pnl DESC`

	rows, err := dbInstance.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var performances []SourcePerformance
	for rows.Next() {
		var perf SourcePerformance
		err := rows.Scan(
			&perf.Source,
			&perf.TotalTrades,
			&perf.WinningTrades,
			&perf.TotalPnL,
			&perf.AvgPnL,
		)
		if err != nil {
			sm.logger.Error("Failed to scan source performance", "error", err)
			continue
		}

		// Calculate win rate
		if perf.TotalTrades > 0 {
			perf.WinRate = float64(perf.WinningTrades) / float64(perf.TotalTrades) * 100
		}

		performances = append(performances, perf)
	}

	return performances, nil
}

// GetPerformanceByStrategy returns performance for a specific strategy
func (sm *StrategyStatsManager) GetPerformanceByStrategy(ctx context.Context, strategyID int64) (*StrategyPerformance, error) {
	if sm.db == nil {
		return nil, nil
	}

	dbInstance := sm.db.GetDB()
	if dbInstance == nil || dbInstance.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT
			strategy_id,
			strategy_name,
			COUNT(*) as total_trades,
			SUM(CASE WHEN realized_pnl > 0 THEN 1 ELSE 0 END) as winning_trades,
			SUM(CASE WHEN realized_pnl <= 0 THEN 1 ELSE 0 END) as losing_trades,
			COALESCE(SUM(realized_pnl), 0) as total_pnl,
			COALESCE(AVG(realized_pnl), 0) as avg_pnl,
			COALESCE(AVG(CASE WHEN realized_pnl > 0 THEN realized_pnl END), 0) as avg_win,
			COALESCE(AVG(CASE WHEN realized_pnl < 0 THEN realized_pnl END), 0) as avg_loss,
			COALESCE(MAX(realized_pnl), 0) as largest_win,
			COALESCE(MIN(realized_pnl), 0) as largest_loss,
			MAX(exit_time) as last_trade_time
		FROM futures_trades
		WHERE strategy_id = $1
			AND status = 'CLOSED'
			AND realized_pnl IS NOT NULL
		GROUP BY strategy_id, strategy_name`

	var perf StrategyPerformance
	var lastTradeTime *time.Time

	err := dbInstance.Pool.QueryRow(ctx, query, strategyID).Scan(
		&perf.StrategyID,
		&perf.StrategyName,
		&perf.TotalTrades,
		&perf.WinningTrades,
		&perf.LosingTrades,
		&perf.TotalPnL,
		&perf.AvgPnL,
		&perf.AvgWin,
		&perf.AvgLoss,
		&perf.LargestWin,
		&perf.LargestLoss,
		&lastTradeTime,
	)
	if err != nil {
		return nil, err
	}

	if lastTradeTime != nil {
		perf.LastTradeTime = *lastTradeTime
	}

	// Calculate win rate
	if perf.TotalTrades > 0 {
		perf.WinRate = float64(perf.WinningTrades) / float64(perf.TotalTrades) * 100
	}

	return &perf, nil
}
