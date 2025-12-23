package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ============================================================================
// USER-SCOPED TRADES
// ============================================================================

// CreateTradeForUser inserts a new trade for a specific user
func (r *Repository) CreateTradeForUser(ctx context.Context, userID string, trade *Trade) error {
	if trade.TradeSource == "" {
		trade.TradeSource = TradeSourceManual
	}
	query := `
		INSERT INTO trades (user_id, symbol, side, entry_price, quantity, entry_time, stop_loss, take_profit, strategy_name, status, trade_source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		userID, trade.Symbol, trade.Side, trade.EntryPrice, trade.Quantity, trade.EntryTime,
		trade.StopLoss, trade.TakeProfit, trade.StrategyName, trade.Status, trade.TradeSource,
	).Scan(&trade.ID, &trade.CreatedAt, &trade.UpdatedAt)
}

// GetTradeByIDForUser retrieves a trade by ID for a specific user
func (r *Repository) GetTradeByIDForUser(ctx context.Context, userID string, id int64) (*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, ai_decision_id
		FROM trades
		WHERE id = $1 AND user_id = $2
	`
	trade := &Trade{}
	err := r.db.Pool.QueryRow(ctx, query, id, userID).Scan(
		&trade.ID, &trade.Symbol, &trade.Side, &trade.EntryPrice, &trade.ExitPrice,
		&trade.Quantity, &trade.EntryTime, &trade.ExitTime, &trade.StopLoss, &trade.TakeProfit,
		&trade.PnL, &trade.PnLPercent, &trade.StrategyName, &trade.Status,
		&trade.CreatedAt, &trade.UpdatedAt, &trade.TradeSource, &trade.AIDecisionID,
	)
	if err != nil {
		return nil, err
	}
	return trade, nil
}

// GetOpenTradesForUser retrieves all open trades for a specific user
func (r *Repository) GetOpenTradesForUser(ctx context.Context, userID string) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, ai_decision_id
		FROM trades
		WHERE status = 'OPEN' AND user_id = $1
		ORDER BY entry_time DESC
	`
	return r.queryTrades(ctx, query, userID)
}

// GetTradeHistoryForUser retrieves closed trades for a specific user with pagination
func (r *Repository) GetTradeHistoryForUser(ctx context.Context, userID string, limit, offset int) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, NULL as ai_decision_id
		FROM trades
		WHERE status = 'CLOSED' AND user_id = $1
		ORDER BY exit_time DESC
		LIMIT $2 OFFSET $3
	`
	return r.queryTrades(ctx, query, userID, limit, offset)
}

// GetTradesBySymbolForUser retrieves trades for a specific symbol and user
func (r *Repository) GetTradesBySymbolForUser(ctx context.Context, userID, symbol string) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, NULL as ai_decision_id
		FROM trades
		WHERE symbol = $1 AND user_id = $2
		ORDER BY entry_time DESC
	`
	return r.queryTrades(ctx, query, symbol, userID)
}

// UpdateTradeForUser updates an existing trade for a specific user
func (r *Repository) UpdateTradeForUser(ctx context.Context, userID string, trade *Trade) error {
	query := `
		UPDATE trades
		SET exit_price = $2, exit_time = $3, pnl = $4, pnl_percent = $5, status = $6
		WHERE id = $1 AND user_id = $7
	`
	_, err := r.db.Pool.Exec(
		ctx, query,
		trade.ID, trade.ExitPrice, trade.ExitTime, trade.PnL, trade.PnLPercent, trade.Status, userID,
	)
	return err
}

// CountOpenTradesForUser counts open trades for a user
func (r *Repository) CountOpenTradesForUser(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM trades WHERE status = 'OPEN' AND user_id = $1`
	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// ============================================================================
// USER-SCOPED ORDERS
// ============================================================================

// CreateOrderForUser inserts a new order for a specific user
func (r *Repository) CreateOrderForUser(ctx context.Context, userID string, order *Order) error {
	query := `
		INSERT INTO orders (id, user_id, symbol, order_type, side, price, quantity, executed_qty, status, time_in_force, created_at, trade_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING updated_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		order.ID, userID, order.Symbol, order.OrderType, order.Side, order.Price,
		order.Quantity, order.ExecutedQty, order.Status, order.TimeInForce,
		order.CreatedAt, order.TradeID,
	).Scan(&order.UpdatedAt)
}

// GetOrderByIDForUser retrieves an order by ID for a specific user
func (r *Repository) GetOrderByIDForUser(ctx context.Context, userID string, id int64) (*Order, error) {
	query := `
		SELECT id, symbol, order_type, side, price, quantity, executed_qty, status,
		       time_in_force, created_at, updated_at, filled_at, trade_id
		FROM orders
		WHERE id = $1 AND user_id = $2
	`
	order := &Order{}
	err := r.db.Pool.QueryRow(ctx, query, id, userID).Scan(
		&order.ID, &order.Symbol, &order.OrderType, &order.Side, &order.Price,
		&order.Quantity, &order.ExecutedQty, &order.Status, &order.TimeInForce,
		&order.CreatedAt, &order.UpdatedAt, &order.FilledAt, &order.TradeID,
	)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// GetActiveOrdersForUser retrieves all active orders for a specific user
func (r *Repository) GetActiveOrdersForUser(ctx context.Context, userID string) ([]*Order, error) {
	query := `
		SELECT id, symbol, order_type, side, price, quantity, executed_qty, status,
		       time_in_force, created_at, updated_at, filled_at, trade_id
		FROM orders
		WHERE status IN ('NEW', 'PARTIALLY_FILLED') AND user_id = $1
		ORDER BY created_at DESC
	`
	return r.queryOrders(ctx, query, userID)
}

// GetOrderHistoryForUser retrieves order history for a specific user with pagination
func (r *Repository) GetOrderHistoryForUser(ctx context.Context, userID string, limit, offset int) ([]*Order, error) {
	query := `
		SELECT id, symbol, order_type, side, price, quantity, executed_qty, status,
		       time_in_force, created_at, updated_at, filled_at, trade_id
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	return r.queryOrders(ctx, query, userID, limit, offset)
}

// UpdateOrderStatusForUser updates an order's status for a specific user
func (r *Repository) UpdateOrderStatusForUser(ctx context.Context, userID string, orderID int64, status string, executedQty float64, filledAt *time.Time) error {
	query := `
		UPDATE orders
		SET status = $2, executed_qty = $3, filled_at = $4
		WHERE id = $1 AND user_id = $5
	`
	_, err := r.db.Pool.Exec(ctx, query, orderID, status, executedQty, filledAt, userID)
	return err
}

// ============================================================================
// USER-SCOPED SIGNALS
// ============================================================================

// CreateSignalForUser inserts a new signal for a specific user
func (r *Repository) CreateSignalForUser(ctx context.Context, userID string, signal *Signal) error {
	query := `
		INSERT INTO signals (user_id, strategy_name, symbol, signal_type, entry_price, stop_loss, take_profit, quantity, reason, timestamp, executed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		userID, signal.StrategyName, signal.Symbol, signal.SignalType, signal.EntryPrice,
		signal.StopLoss, signal.TakeProfit, signal.Quantity, signal.Reason,
		signal.Timestamp, signal.Executed,
	).Scan(&signal.ID, &signal.CreatedAt)
}

// GetRecentSignalsForUser retrieves recent signals for a specific user
func (r *Repository) GetRecentSignalsForUser(ctx context.Context, userID string, limit int) ([]*Signal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, stop_loss, take_profit,
		       quantity, reason, timestamp, executed, created_at
		FROM signals
		WHERE user_id = $1
		ORDER BY timestamp DESC
		LIMIT $2
	`
	rows, err := r.db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []*Signal
	for rows.Next() {
		signal := &Signal{}
		err := rows.Scan(
			&signal.ID, &signal.StrategyName, &signal.Symbol, &signal.SignalType,
			&signal.EntryPrice, &signal.StopLoss, &signal.TakeProfit, &signal.Quantity,
			&signal.Reason, &signal.Timestamp, &signal.Executed, &signal.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		signals = append(signals, signal)
	}
	return signals, rows.Err()
}

// MarkSignalExecutedForUser marks a signal as executed for a specific user
func (r *Repository) MarkSignalExecutedForUser(ctx context.Context, userID string, id int64) error {
	query := `UPDATE signals SET executed = TRUE WHERE id = $1 AND user_id = $2`
	_, err := r.db.Pool.Exec(ctx, query, id, userID)
	return err
}

// ============================================================================
// USER-SCOPED PENDING SIGNALS
// ============================================================================

// CreatePendingSignalForUser inserts a new pending signal for a specific user
func (r *Repository) CreatePendingSignalForUser(ctx context.Context, userID string, signal *PendingSignal) error {
	conditionsJSON, err := json.Marshal(signal.ConditionsMet)
	if err != nil {
		return fmt.Errorf("failed to marshal conditions: %w", err)
	}

	query := `
		INSERT INTO pending_signals (user_id, strategy_name, symbol, signal_type, entry_price, current_price,
			stop_loss, take_profit, quantity, reason, conditions_met, timestamp, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		userID, signal.StrategyName, signal.Symbol, signal.SignalType, signal.EntryPrice, signal.CurrentPrice,
		signal.StopLoss, signal.TakeProfit, signal.Quantity, signal.Reason, conditionsJSON,
		signal.Timestamp, signal.Status,
	).Scan(&signal.ID, &signal.CreatedAt)
}

// GetPendingSignalsForUser retrieves all pending signals for a specific user
func (r *Repository) GetPendingSignalsForUser(ctx context.Context, userID string) ([]*PendingSignal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, current_price, stop_loss, take_profit,
			   quantity, reason, conditions_met, timestamp, status, confirmed_at, rejected_at, archived, archived_at, created_at
		FROM pending_signals
		WHERE status = 'PENDING' AND user_id = $1
		ORDER BY timestamp DESC
	`
	return r.queryPendingSignals(ctx, query, userID)
}

// GetPendingSignalByIDForUser retrieves a pending signal by ID for a specific user
func (r *Repository) GetPendingSignalByIDForUser(ctx context.Context, userID string, id int64) (*PendingSignal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, current_price, stop_loss, take_profit,
			   quantity, reason, conditions_met, timestamp, status, confirmed_at, rejected_at, archived, archived_at, created_at
		FROM pending_signals
		WHERE id = $1 AND user_id = $2
	`
	signal := &PendingSignal{}
	var conditionsJSON []byte
	err := r.db.Pool.QueryRow(ctx, query, id, userID).Scan(
		&signal.ID, &signal.StrategyName, &signal.Symbol, &signal.SignalType,
		&signal.EntryPrice, &signal.CurrentPrice, &signal.StopLoss, &signal.TakeProfit,
		&signal.Quantity, &signal.Reason, &conditionsJSON, &signal.Timestamp,
		&signal.Status, &signal.ConfirmedAt, &signal.RejectedAt, &signal.Archived, &signal.ArchivedAt, &signal.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(conditionsJSON) > 0 {
		if err := json.Unmarshal(conditionsJSON, &signal.ConditionsMet); err != nil {
			return nil, err
		}
	}
	return signal, nil
}

// UpdatePendingSignalStatusForUser updates the status of a pending signal for a specific user
func (r *Repository) UpdatePendingSignalStatusForUser(ctx context.Context, userID string, id int64, status string, currentPrice float64) error {
	var query string
	var args []interface{}

	if status == "CONFIRMED" {
		query = `UPDATE pending_signals SET status = $2, current_price = $3, confirmed_at = CURRENT_TIMESTAMP WHERE id = $1 AND user_id = $4`
		args = []interface{}{id, status, currentPrice, userID}
	} else if status == "REJECTED" {
		query = `UPDATE pending_signals SET status = $2, current_price = $3, rejected_at = CURRENT_TIMESTAMP WHERE id = $1 AND user_id = $4`
		args = []interface{}{id, status, currentPrice, userID}
	} else {
		query = `UPDATE pending_signals SET status = $2, current_price = $3 WHERE id = $1 AND user_id = $4`
		args = []interface{}{id, status, currentPrice, userID}
	}

	_, err := r.db.Pool.Exec(ctx, query, args...)
	return err
}

// ArchivePendingSignalForUser soft deletes a pending signal for a specific user
func (r *Repository) ArchivePendingSignalForUser(ctx context.Context, userID string, id int64) error {
	query := `UPDATE pending_signals SET archived = TRUE, archived_at = CURRENT_TIMESTAMP WHERE id = $1 AND user_id = $2`
	_, err := r.db.Pool.Exec(ctx, query, id, userID)
	return err
}

// Helper function to query pending signals
func (r *Repository) queryPendingSignals(ctx context.Context, query string, args ...interface{}) ([]*PendingSignal, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var signals []*PendingSignal
	for rows.Next() {
		signal := &PendingSignal{}
		var conditionsJSON []byte
		err := rows.Scan(
			&signal.ID, &signal.StrategyName, &signal.Symbol, &signal.SignalType,
			&signal.EntryPrice, &signal.CurrentPrice, &signal.StopLoss, &signal.TakeProfit,
			&signal.Quantity, &signal.Reason, &conditionsJSON, &signal.Timestamp,
			&signal.Status, &signal.ConfirmedAt, &signal.RejectedAt, &signal.Archived, &signal.ArchivedAt, &signal.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(conditionsJSON) > 0 {
			if err := json.Unmarshal(conditionsJSON, &signal.ConditionsMet); err != nil {
				return nil, err
			}
		}
		signals = append(signals, signal)
	}
	return signals, rows.Err()
}

// ============================================================================
// USER-SCOPED STRATEGY CONFIGS
// ============================================================================

// CreateStrategyConfigForUser inserts a new strategy configuration for a specific user
func (r *Repository) CreateStrategyConfigForUser(ctx context.Context, userID string, config *StrategyConfig) error {
	configJSON, err := json.Marshal(config.ConfigParams)
	if err != nil {
		return fmt.Errorf("failed to marshal config params: %w", err)
	}

	query := `
		INSERT INTO strategy_configs (user_id, name, symbol, timeframe, indicator_type, autopilot, enabled,
			position_size, stop_loss_percent, take_profit_percent, config_params)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		userID, config.Name, config.Symbol, config.Timeframe, config.IndicatorType, config.Autopilot,
		config.Enabled, config.PositionSize, config.StopLossPercent, config.TakeProfitPercent, configJSON,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
}

// GetAllStrategyConfigsForUser retrieves all strategy configurations for a specific user
func (r *Repository) GetAllStrategyConfigsForUser(ctx context.Context, userID string) ([]*StrategyConfig, error) {
	query := `
		SELECT id, name, symbol, timeframe, indicator_type, autopilot, enabled,
			   position_size, stop_loss_percent, take_profit_percent, config_params, created_at, updated_at
		FROM strategy_configs
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*StrategyConfig
	for rows.Next() {
		config := &StrategyConfig{}
		var configJSON []byte
		err := rows.Scan(
			&config.ID, &config.Name, &config.Symbol, &config.Timeframe, &config.IndicatorType,
			&config.Autopilot, &config.Enabled, &config.PositionSize, &config.StopLossPercent,
			&config.TakeProfitPercent, &configJSON, &config.CreatedAt, &config.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(configJSON) > 0 {
			if err := json.Unmarshal(configJSON, &config.ConfigParams); err != nil {
				return nil, err
			}
		}
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

// UpdateStrategyConfigForUser updates an existing strategy configuration for a specific user
func (r *Repository) UpdateStrategyConfigForUser(ctx context.Context, userID string, config *StrategyConfig) error {
	configJSON, err := json.Marshal(config.ConfigParams)
	if err != nil {
		return fmt.Errorf("failed to marshal config params: %w", err)
	}

	query := `
		UPDATE strategy_configs
		SET symbol = $2, timeframe = $3, indicator_type = $4, autopilot = $5, enabled = $6,
			position_size = $7, stop_loss_percent = $8, take_profit_percent = $9, config_params = $10
		WHERE id = $1 AND user_id = $11
	`
	_, err = r.db.Pool.Exec(
		ctx, query,
		config.ID, config.Symbol, config.Timeframe, config.IndicatorType, config.Autopilot,
		config.Enabled, config.PositionSize, config.StopLossPercent, config.TakeProfitPercent, configJSON, userID,
	)
	return err
}

// DeleteStrategyConfigForUser deletes a strategy configuration for a specific user
func (r *Repository) DeleteStrategyConfigForUser(ctx context.Context, userID string, id int64) error {
	query := `DELETE FROM strategy_configs WHERE id = $1 AND user_id = $2`
	_, err := r.db.Pool.Exec(ctx, query, id, userID)
	return err
}

// ============================================================================
// USER-SCOPED WATCHLIST
// ============================================================================

// GetWatchlistForUser retrieves all watchlist items for a specific user
func (r *Repository) GetWatchlistForUser(ctx context.Context, userID string) ([]*WatchlistItem, error) {
	query := `SELECT id, symbol, notes, added_at, created_at FROM watchlist WHERE user_id = $1 ORDER BY added_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query watchlist: %w", err)
	}
	defer rows.Close()

	items := []*WatchlistItem{}
	for rows.Next() {
		item := &WatchlistItem{}
		err := rows.Scan(&item.ID, &item.Symbol, &item.Notes, &item.AddedAt, &item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan watchlist item: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}

// AddToWatchlistForUser adds a symbol to the watchlist for a specific user
func (r *Repository) AddToWatchlistForUser(ctx context.Context, userID, symbol string, notes *string) error {
	query := `INSERT INTO watchlist (user_id, symbol, notes) VALUES ($1, $2, $3) ON CONFLICT (user_id, symbol) DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, query, userID, symbol, notes)
	if err != nil {
		return fmt.Errorf("failed to add to watchlist: %w", err)
	}
	return nil
}

// RemoveFromWatchlistForUser removes a symbol from the watchlist for a specific user
func (r *Repository) RemoveFromWatchlistForUser(ctx context.Context, userID, symbol string) error {
	query := `DELETE FROM watchlist WHERE user_id = $1 AND symbol = $2`
	_, err := r.db.Pool.Exec(ctx, query, userID, symbol)
	if err != nil {
		return fmt.Errorf("failed to remove from watchlist: %w", err)
	}
	return nil
}

// IsInWatchlistForUser checks if a symbol is in the watchlist for a specific user
func (r *Repository) IsInWatchlistForUser(ctx context.Context, userID, symbol string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM watchlist WHERE user_id = $1 AND symbol = $2)`
	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, userID, symbol).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check watchlist: %w", err)
	}
	return exists, nil
}

// ============================================================================
// USER-SCOPED METRICS
// ============================================================================

// GetTradingMetricsForUser calculates and returns trading metrics for a specific user
func (r *Repository) GetTradingMetricsForUser(ctx context.Context, userID string) (*TradingMetrics, error) {
	metrics := &TradingMetrics{}

	// Get trade statistics
	tradeQuery := `
		SELECT
			COUNT(*) as total_trades,
			COUNT(*) FILTER (WHERE pnl > 0) as winning_trades,
			COUNT(*) FILTER (WHERE pnl < 0) as losing_trades,
			COALESCE(SUM(pnl), 0) as total_pnl,
			COALESCE(AVG(pnl), 0) as average_pnl,
			COALESCE(AVG(pnl) FILTER (WHERE pnl > 0), 0) as average_win,
			COALESCE(AVG(pnl) FILTER (WHERE pnl < 0), 0) as average_loss,
			COALESCE(MAX(pnl), 0) as largest_win,
			COALESCE(MIN(pnl), 0) as largest_loss,
			MAX(exit_time) as last_trade_time
		FROM trades
		WHERE status = 'CLOSED' AND pnl IS NOT NULL AND user_id = $1
	`

	err := r.db.Pool.QueryRow(ctx, tradeQuery, userID).Scan(
		&metrics.TotalTrades, &metrics.WinningTrades, &metrics.LosingTrades,
		&metrics.TotalPnL, &metrics.AveragePnL, &metrics.AverageWin, &metrics.AverageLoss,
		&metrics.LargestWin, &metrics.LargestLoss, &metrics.LastTradeTime,
	)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Calculate win rate
	if metrics.TotalTrades > 0 {
		metrics.WinRate = float64(metrics.WinningTrades) / float64(metrics.TotalTrades) * 100
	}

	// Calculate profit factor
	totalWins := metrics.AverageWin * float64(metrics.WinningTrades)
	totalLosses := metrics.AverageLoss * float64(metrics.LosingTrades)
	if totalLosses != 0 {
		metrics.ProfitFactor = totalWins / (-totalLosses)
	}

	// Get open positions count
	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM trades WHERE status = 'OPEN' AND user_id = $1`, userID).Scan(&metrics.OpenPositions)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Get active orders count
	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status IN ('NEW', 'PARTIALLY_FILLED') AND user_id = $1`, userID).Scan(&metrics.ActiveOrders)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Get signal statistics
	signalQuery := `
		SELECT
			COUNT(*) as total_signals,
			COUNT(*) FILTER (WHERE executed = TRUE) as executed_signals
		FROM signals
		WHERE user_id = $1
	`
	err = r.db.Pool.QueryRow(ctx, signalQuery, userID).Scan(&metrics.TotalSignals, &metrics.ExecutedSignals)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	return metrics, nil
}

// ============================================================================
// USER-SCOPED POSITION SNAPSHOTS
// ============================================================================

// CreatePositionSnapshotForUser inserts a position snapshot for a specific user
func (r *Repository) CreatePositionSnapshotForUser(ctx context.Context, userID string, snapshot *PositionSnapshot) error {
	query := `
		INSERT INTO position_snapshots (user_id, symbol, entry_price, current_price, quantity, pnl, pnl_percent, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		userID, snapshot.Symbol, snapshot.EntryPrice, snapshot.CurrentPrice, snapshot.Quantity,
		snapshot.PnL, snapshot.PnLPercent, snapshot.Timestamp,
	).Scan(&snapshot.ID, &snapshot.CreatedAt)
}

// ============================================================================
// DAILY TRADING STATS FOR LIMITS
// ============================================================================

// GetDailyTradeCountForUser gets the number of trades placed today for a user
func (r *Repository) GetDailyTradeCountForUser(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM trades
		WHERE user_id = $1 AND entry_time >= CURRENT_DATE
	`
	var count int
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// GetDailyLossForUser gets the total loss for today for a user
func (r *Repository) GetDailyLossForUser(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(pnl), 0) FROM trades
		WHERE user_id = $1 AND exit_time >= CURRENT_DATE AND pnl < 0
	`
	var loss float64
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&loss)
	return loss, err
}

// GetDailyPnLForUser gets the total PnL for today for a user
func (r *Repository) GetDailyPnLForUser(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(pnl), 0) FROM trades
		WHERE user_id = $1 AND exit_time >= CURRENT_DATE
	`
	var pnl float64
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(&pnl)
	return pnl, err
}

// ============================================================================
// TIER LIMIT CHECKS
// ============================================================================

// CanUserOpenPosition checks if a user can open a new position based on their tier limits
func (r *Repository) CanUserOpenPosition(ctx context.Context, userID string, maxPositions int) (bool, error) {
	count, err := r.CountOpenTradesForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	return count < maxPositions, nil
}

// GetUserTierLimits returns the trading limits for a user based on their trading config
func (r *Repository) GetUserTierLimits(ctx context.Context, userID string) (*UserTradingConfig, error) {
	return r.GetUserTradingConfig(ctx, userID)
}
