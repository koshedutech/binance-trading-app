package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Repository provides data access methods
type Repository struct {
	db *DB
}

// NewRepository creates a new repository
func NewRepository(db *DB) *Repository {
	return &Repository{db: db}
}

// HealthCheck performs a database health check
func (r *Repository) HealthCheck(ctx context.Context) error {
	return r.db.Pool.Ping(ctx)
}

// GetDB returns the underlying DB instance for direct access to futures methods
func (r *Repository) GetDB() *DB {
	return r.db
}

// ============================================================================
// TRADES
// ============================================================================

// CreateTrade inserts a new trade
func (r *Repository) CreateTrade(ctx context.Context, trade *Trade) error {
	// Set default trade source if not specified
	if trade.TradeSource == "" {
		trade.TradeSource = TradeSourceManual
	}
	query := `
		INSERT INTO trades (symbol, side, entry_price, quantity, entry_time, stop_loss, take_profit, strategy_name, status, trade_source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		trade.Symbol, trade.Side, trade.EntryPrice, trade.Quantity, trade.EntryTime,
		trade.StopLoss, trade.TakeProfit, trade.StrategyName, trade.Status, trade.TradeSource,
	).Scan(&trade.ID, &trade.CreatedAt, &trade.UpdatedAt)
}

// UpdateTrade updates an existing trade
func (r *Repository) UpdateTrade(ctx context.Context, trade *Trade) error {
	query := `
		UPDATE trades
		SET exit_price = $2, exit_time = $3, pnl = $4, pnl_percent = $5, status = $6
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(
		ctx, query,
		trade.ID, trade.ExitPrice, trade.ExitTime, trade.PnL, trade.PnLPercent, trade.Status,
	)
	return err
}

// GetTradeByID retrieves a trade by ID
func (r *Repository) GetTradeByID(ctx context.Context, id int64) (*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source
		FROM trades
		WHERE id = $1
	`
	trade := &Trade{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&trade.ID, &trade.Symbol, &trade.Side, &trade.EntryPrice, &trade.ExitPrice,
		&trade.Quantity, &trade.EntryTime, &trade.ExitTime, &trade.StopLoss, &trade.TakeProfit,
		&trade.PnL, &trade.PnLPercent, &trade.StrategyName, &trade.Status,
		&trade.CreatedAt, &trade.UpdatedAt, &trade.TradeSource,
	)
	if err != nil {
		return nil, err
	}
	return trade, nil
}

// GetOpenTrades retrieves all open trades
func (r *Repository) GetOpenTrades(ctx context.Context) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, ai_decision_id
		FROM trades
		WHERE status = 'OPEN'
		ORDER BY entry_time DESC
	`
	return r.queryTrades(ctx, query)
}

// GetTradeHistory retrieves closed trades with pagination
func (r *Repository) GetTradeHistory(ctx context.Context, limit, offset int) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, ai_decision_id
		FROM trades
		WHERE status = 'CLOSED'
		ORDER BY exit_time DESC
		LIMIT $1 OFFSET $2
	`
	return r.queryTrades(ctx, query, limit, offset)
}

// GetTradesBySymbol retrieves trades for a specific symbol
func (r *Repository) GetTradesBySymbol(ctx context.Context, symbol string) ([]*Trade, error) {
	query := `
		SELECT id, symbol, side, entry_price, exit_price, quantity, entry_time, exit_time,
		       stop_loss, take_profit, pnl, pnl_percent, strategy_name, status, created_at, updated_at, trade_source, ai_decision_id
		FROM trades
		WHERE symbol = $1
		ORDER BY entry_time DESC
	`
	return r.queryTrades(ctx, query, symbol)
}

func (r *Repository) queryTrades(ctx context.Context, query string, args ...interface{}) ([]*Trade, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		trade := &Trade{}
		err := rows.Scan(
			&trade.ID, &trade.Symbol, &trade.Side, &trade.EntryPrice, &trade.ExitPrice,
			&trade.Quantity, &trade.EntryTime, &trade.ExitTime, &trade.StopLoss, &trade.TakeProfit,
			&trade.PnL, &trade.PnLPercent, &trade.StrategyName, &trade.Status,
			&trade.CreatedAt, &trade.UpdatedAt, &trade.TradeSource, &trade.AIDecisionID,
		)
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, rows.Err()
}

// ============================================================================
// ORDERS
// ============================================================================

// CreateOrder inserts a new order
func (r *Repository) CreateOrder(ctx context.Context, order *Order) error {
	query := `
		INSERT INTO orders (id, symbol, order_type, side, price, quantity, executed_qty, status, time_in_force, created_at, trade_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING updated_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		order.ID, order.Symbol, order.OrderType, order.Side, order.Price,
		order.Quantity, order.ExecutedQty, order.Status, order.TimeInForce,
		order.CreatedAt, order.TradeID,
	).Scan(&order.UpdatedAt)
}

// UpdateOrderStatus updates an order's status
func (r *Repository) UpdateOrderStatus(ctx context.Context, orderID int64, status string, executedQty float64, filledAt *time.Time) error {
	query := `
		UPDATE orders
		SET status = $2, executed_qty = $3, filled_at = $4
		WHERE id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, orderID, status, executedQty, filledAt)
	return err
}

// GetOrderByID retrieves an order by ID
func (r *Repository) GetOrderByID(ctx context.Context, id int64) (*Order, error) {
	query := `
		SELECT id, symbol, order_type, side, price, quantity, executed_qty, status,
		       time_in_force, created_at, updated_at, filled_at, trade_id
		FROM orders
		WHERE id = $1
	`
	order := &Order{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&order.ID, &order.Symbol, &order.OrderType, &order.Side, &order.Price,
		&order.Quantity, &order.ExecutedQty, &order.Status, &order.TimeInForce,
		&order.CreatedAt, &order.UpdatedAt, &order.FilledAt, &order.TradeID,
	)
	if err != nil {
		return nil, err
	}
	return order, nil
}

// GetActiveOrders retrieves all active orders
func (r *Repository) GetActiveOrders(ctx context.Context) ([]*Order, error) {
	query := `
		SELECT id, symbol, order_type, side, price, quantity, executed_qty, status,
		       time_in_force, created_at, updated_at, filled_at, trade_id
		FROM orders
		WHERE status IN ('NEW', 'PARTIALLY_FILLED')
		ORDER BY created_at DESC
	`
	return r.queryOrders(ctx, query)
}

// GetOrderHistory retrieves order history with pagination
func (r *Repository) GetOrderHistory(ctx context.Context, limit, offset int) ([]*Order, error) {
	query := `
		SELECT id, symbol, order_type, side, price, quantity, executed_qty, status,
		       time_in_force, created_at, updated_at, filled_at, trade_id
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	return r.queryOrders(ctx, query, limit, offset)
}

func (r *Repository) queryOrders(ctx context.Context, query string, args ...interface{}) ([]*Order, error) {
	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		err := rows.Scan(
			&order.ID, &order.Symbol, &order.OrderType, &order.Side, &order.Price,
			&order.Quantity, &order.ExecutedQty, &order.Status, &order.TimeInForce,
			&order.CreatedAt, &order.UpdatedAt, &order.FilledAt, &order.TradeID,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

// ============================================================================
// SIGNALS
// ============================================================================

// CreateSignal inserts a new signal
func (r *Repository) CreateSignal(ctx context.Context, signal *Signal) error {
	query := `
		INSERT INTO signals (strategy_name, symbol, signal_type, entry_price, stop_loss, take_profit, quantity, reason, timestamp, executed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		signal.StrategyName, signal.Symbol, signal.SignalType, signal.EntryPrice,
		signal.StopLoss, signal.TakeProfit, signal.Quantity, signal.Reason,
		signal.Timestamp, signal.Executed,
	).Scan(&signal.ID, &signal.CreatedAt)
}

// MarkSignalExecuted marks a signal as executed
func (r *Repository) MarkSignalExecuted(ctx context.Context, id int64) error {
	query := `UPDATE signals SET executed = TRUE WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// GetRecentSignals retrieves recent signals
func (r *Repository) GetRecentSignals(ctx context.Context, limit int) ([]*Signal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, stop_loss, take_profit,
		       quantity, reason, timestamp, executed, created_at
		FROM signals
		ORDER BY timestamp DESC
		LIMIT $1
	`
	rows, err := r.db.Pool.Query(ctx, query, limit)
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

// ============================================================================
// POSITION SNAPSHOTS
// ============================================================================

// CreatePositionSnapshot inserts a position snapshot
func (r *Repository) CreatePositionSnapshot(ctx context.Context, snapshot *PositionSnapshot) error {
	query := `
		INSERT INTO position_snapshots (symbol, entry_price, current_price, quantity, pnl, pnl_percent, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		snapshot.Symbol, snapshot.EntryPrice, snapshot.CurrentPrice, snapshot.Quantity,
		snapshot.PnL, snapshot.PnLPercent, snapshot.Timestamp,
	).Scan(&snapshot.ID, &snapshot.CreatedAt)
}

// ============================================================================
// SCREENER RESULTS
// ============================================================================

// CreateScreenerResult inserts a screener result
func (r *Repository) CreateScreenerResult(ctx context.Context, result *ScreenerResult) error {
	query := `
		INSERT INTO screener_results (symbol, last_price, price_change_percent, volume, quote_volume, high_24h, low_24h, signals, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		result.Symbol, result.LastPrice, result.PriceChangePercent, result.Volume,
		result.QuoteVolume, result.High24h, result.Low24h, result.Signals, result.Timestamp,
	).Scan(&result.ID, &result.CreatedAt)
}

// GetLatestScreenerResults retrieves the most recent screener results
func (r *Repository) GetLatestScreenerResults(ctx context.Context, limit int) ([]*ScreenerResult, error) {
	query := `
		SELECT DISTINCT ON (symbol) id, symbol, last_price, price_change_percent, volume,
		       quote_volume, high_24h, low_24h, signals, timestamp, created_at
		FROM screener_results
		ORDER BY symbol, timestamp DESC
		LIMIT $1
	`
	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*ScreenerResult
	for rows.Next() {
		result := &ScreenerResult{}
		err := rows.Scan(
			&result.ID, &result.Symbol, &result.LastPrice, &result.PriceChangePercent,
			&result.Volume, &result.QuoteVolume, &result.High24h, &result.Low24h,
			&result.Signals, &result.Timestamp, &result.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// ============================================================================
// SYSTEM EVENTS
// ============================================================================

// CreateSystemEvent inserts a system event
func (r *Repository) CreateSystemEvent(ctx context.Context, event *SystemEvent) error {
	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	query := `
		INSERT INTO system_events (event_type, source, message, data, timestamp)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		event.EventType, event.Source, event.Message, dataJSON, event.Timestamp,
	).Scan(&event.ID, &event.CreatedAt)
}

// GetRecentSystemEvents retrieves recent system events
func (r *Repository) GetRecentSystemEvents(ctx context.Context, limit int) ([]*SystemEvent, error) {
	query := `
		SELECT id, event_type, source, message, data, timestamp, created_at
		FROM system_events
		ORDER BY timestamp DESC
		LIMIT $1
	`
	rows, err := r.db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*SystemEvent
	for rows.Next() {
		event := &SystemEvent{}
		var dataJSON []byte
		err := rows.Scan(
			&event.ID, &event.EventType, &event.Source, &event.Message,
			&dataJSON, &event.Timestamp, &event.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &event.Data); err != nil {
				return nil, err
			}
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// ============================================================================
// METRICS
// ============================================================================

// GetTradingMetrics calculates and returns trading metrics
func (r *Repository) GetTradingMetrics(ctx context.Context) (*TradingMetrics, error) {
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
		WHERE status = 'CLOSED' AND pnl IS NOT NULL
	`

	err := r.db.Pool.QueryRow(ctx, tradeQuery).Scan(
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
	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM trades WHERE status = 'OPEN'`).Scan(&metrics.OpenPositions)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Get active orders count
	err = r.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE status IN ('NEW', 'PARTIALLY_FILLED')`).Scan(&metrics.ActiveOrders)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	// Get signal statistics
	signalQuery := `
		SELECT
			COUNT(*) as total_signals,
			COUNT(*) FILTER (WHERE executed = TRUE) as executed_signals
		FROM signals
	`
	err = r.db.Pool.QueryRow(ctx, signalQuery).Scan(&metrics.TotalSignals, &metrics.ExecutedSignals)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}

	return metrics, nil
}

// ============================================================================
// STRATEGY CONFIGS
// ============================================================================

// CreateStrategyConfig inserts a new strategy configuration
func (r *Repository) CreateStrategyConfig(ctx context.Context, config *StrategyConfig) error {
	configJSON, err := json.Marshal(config.ConfigParams)
	if err != nil {
		return fmt.Errorf("failed to marshal config params: %w", err)
	}

	query := `
		INSERT INTO strategy_configs (name, symbol, timeframe, indicator_type, autopilot, enabled,
			position_size, stop_loss_percent, take_profit_percent, config_params)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		config.Name, config.Symbol, config.Timeframe, config.IndicatorType, config.Autopilot,
		config.Enabled, config.PositionSize, config.StopLossPercent, config.TakeProfitPercent, configJSON,
	).Scan(&config.ID, &config.CreatedAt, &config.UpdatedAt)
}

// UpdateStrategyConfig updates an existing strategy configuration
func (r *Repository) UpdateStrategyConfig(ctx context.Context, config *StrategyConfig) error {
	configJSON, err := json.Marshal(config.ConfigParams)
	if err != nil {
		return fmt.Errorf("failed to marshal config params: %w", err)
	}

	query := `
		UPDATE strategy_configs
		SET symbol = $2, timeframe = $3, indicator_type = $4, autopilot = $5, enabled = $6,
			position_size = $7, stop_loss_percent = $8, take_profit_percent = $9, config_params = $10,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	_, err = r.db.Pool.Exec(
		ctx, query,
		config.ID, config.Symbol, config.Timeframe, config.IndicatorType, config.Autopilot,
		config.Enabled, config.PositionSize, config.StopLossPercent, config.TakeProfitPercent, configJSON,
	)
	return err
}

// GetStrategyConfigByID retrieves a strategy config by ID
func (r *Repository) GetStrategyConfigByID(ctx context.Context, id int64) (*StrategyConfig, error) {
	query := `
		SELECT id, name, symbol, timeframe, indicator_type, autopilot, enabled,
			   position_size, stop_loss_percent, take_profit_percent, config_params, created_at, updated_at
		FROM strategy_configs
		WHERE id = $1
	`
	config := &StrategyConfig{}
	var configJSON []byte
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
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
	return config, nil
}

// GetAllStrategyConfigs retrieves all strategy configurations
func (r *Repository) GetAllStrategyConfigs(ctx context.Context) ([]*StrategyConfig, error) {
	query := `
		SELECT id, name, symbol, timeframe, indicator_type, autopilot, enabled,
			   position_size, stop_loss_percent, take_profit_percent, config_params, created_at, updated_at
		FROM strategy_configs
		ORDER BY created_at DESC
	`
	rows, err := r.db.Pool.Query(ctx, query)
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

// DeleteStrategyConfig deletes a strategy configuration
func (r *Repository) DeleteStrategyConfig(ctx context.Context, id int64) error {
	query := `DELETE FROM strategy_configs WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// ============================================================================
// PENDING SIGNALS
// ============================================================================

// CreatePendingSignal inserts a new pending signal
func (r *Repository) CreatePendingSignal(ctx context.Context, signal *PendingSignal) error {
	conditionsJSON, err := json.Marshal(signal.ConditionsMet)
	if err != nil {
		return fmt.Errorf("failed to marshal conditions: %w", err)
	}

	query := `
		INSERT INTO pending_signals (strategy_name, symbol, signal_type, entry_price, current_price,
			stop_loss, take_profit, quantity, reason, conditions_met, timestamp, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`
	return r.db.Pool.QueryRow(
		ctx, query,
		signal.StrategyName, signal.Symbol, signal.SignalType, signal.EntryPrice, signal.CurrentPrice,
		signal.StopLoss, signal.TakeProfit, signal.Quantity, signal.Reason, conditionsJSON,
		signal.Timestamp, signal.Status,
	).Scan(&signal.ID, &signal.CreatedAt)
}

// UpdatePendingSignalStatus updates the status of a pending signal
func (r *Repository) UpdatePendingSignalStatus(ctx context.Context, id int64, status string, currentPrice float64) error {
	var query string
	var args []interface{}

	if status == "CONFIRMED" {
		query = `UPDATE pending_signals SET status = $2, current_price = $3, confirmed_at = CURRENT_TIMESTAMP WHERE id = $1`
		args = []interface{}{id, status, currentPrice}
	} else if status == "REJECTED" {
		query = `UPDATE pending_signals SET status = $2, current_price = $3, rejected_at = CURRENT_TIMESTAMP WHERE id = $1`
		args = []interface{}{id, status, currentPrice}
	} else {
		query = `UPDATE pending_signals SET status = $2, current_price = $3 WHERE id = $1`
		args = []interface{}{id, status, currentPrice}
	}

	_, err := r.db.Pool.Exec(ctx, query, args...)
	return err
}

// GetPendingSignals retrieves all pending signals
func (r *Repository) GetPendingSignals(ctx context.Context) ([]*PendingSignal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, current_price, stop_loss, take_profit,
			   quantity, reason, conditions_met, timestamp, status, confirmed_at, rejected_at, archived, archived_at, created_at
		FROM pending_signals
		WHERE status = 'PENDING'
		ORDER BY timestamp DESC
	`
	rows, err := r.db.Pool.Query(ctx, query)
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

// GetPendingSignalByID retrieves a pending signal by ID
func (r *Repository) GetPendingSignalByID(ctx context.Context, id int64) (*PendingSignal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, current_price, stop_loss, take_profit,
			   quantity, reason, conditions_met, timestamp, status, confirmed_at, rejected_at, archived, archived_at, created_at
		FROM pending_signals
		WHERE id = $1
	`
	signal := &PendingSignal{}
	var conditionsJSON []byte
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
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

// GetPendingSignalsByStatus retrieves pending signals filtered by status
func (r *Repository) GetPendingSignalsByStatus(ctx context.Context, status string, includeArchived bool, limit int) ([]*PendingSignal, error) {
	query := `
		SELECT id, strategy_name, symbol, signal_type, entry_price, current_price, stop_loss, take_profit,
			   quantity, reason, conditions_met, timestamp, status, confirmed_at, rejected_at, archived, archived_at, created_at
		FROM pending_signals
		WHERE status = $1 AND ($2 OR archived = FALSE)
		ORDER BY timestamp DESC
		LIMIT $3
	`
	rows, err := r.db.Pool.Query(ctx, query, status, includeArchived, limit)
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

// ArchivePendingSignal soft deletes a pending signal
func (r *Repository) ArchivePendingSignal(ctx context.Context, id int64) error {
	query := `UPDATE pending_signals SET archived = TRUE, archived_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// DeletePendingSignal hard deletes a pending signal
func (r *Repository) DeletePendingSignal(ctx context.Context, id int64) error {
	query := `DELETE FROM pending_signals WHERE id = $1`
	_, err := r.db.Pool.Exec(ctx, query, id)
	return err
}

// DuplicatePendingSignal creates a copy of a pending signal with PENDING status
func (r *Repository) DuplicatePendingSignal(ctx context.Context, id int64) (*PendingSignal, error) {
	// Fetch the original signal
	original, err := r.GetPendingSignalByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch original signal: %w", err)
	}

	// Create new signal with PENDING status
	conditionsJSON, err := json.Marshal(original.ConditionsMet)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal conditions: %w", err)
	}

	newSignal := &PendingSignal{
		StrategyName:  original.StrategyName,
		Symbol:        original.Symbol,
		SignalType:    original.SignalType,
		EntryPrice:    original.EntryPrice,
		CurrentPrice:  original.CurrentPrice,
		StopLoss:      original.StopLoss,
		TakeProfit:    original.TakeProfit,
		Quantity:      original.Quantity,
		Reason:        original.Reason,
		ConditionsMet: original.ConditionsMet,
		Timestamp:     time.Now(),
		Status:        "PENDING",
	}

	query := `
		INSERT INTO pending_signals (strategy_name, symbol, signal_type, entry_price, current_price,
			stop_loss, take_profit, quantity, reason, conditions_met, timestamp, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at
	`
	err = r.db.Pool.QueryRow(
		ctx, query,
		newSignal.StrategyName, newSignal.Symbol, newSignal.SignalType, newSignal.EntryPrice, newSignal.CurrentPrice,
		newSignal.StopLoss, newSignal.TakeProfit, newSignal.Quantity, newSignal.Reason, conditionsJSON,
		newSignal.Timestamp, newSignal.Status,
	).Scan(&newSignal.ID, &newSignal.CreatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to insert duplicated signal: %w", err)
	}

	return newSignal, nil
}

// ============================================================================
// WATCHLIST
// ============================================================================

// GetWatchlist retrieves all watchlist items
func (r *Repository) GetWatchlist(ctx context.Context) ([]*WatchlistItem, error) {
	query := `SELECT id, symbol, notes, added_at, created_at FROM watchlist ORDER BY added_at DESC`

	rows, err := r.db.Pool.Query(ctx, query)
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

// AddToWatchlist adds a symbol to the watchlist
func (r *Repository) AddToWatchlist(ctx context.Context, symbol string, notes *string) error {
	query := `INSERT INTO watchlist (symbol, notes) VALUES ($1, $2) ON CONFLICT (symbol) DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, query, symbol, notes)
	if err != nil {
		return fmt.Errorf("failed to add to watchlist: %w", err)
	}
	return nil
}

// RemoveFromWatchlist removes a symbol from the watchlist
func (r *Repository) RemoveFromWatchlist(ctx context.Context, symbol string) error {
	query := `DELETE FROM watchlist WHERE symbol = $1`
	_, err := r.db.Pool.Exec(ctx, query, symbol)
	if err != nil {
		return fmt.Errorf("failed to remove from watchlist: %w", err)
	}
	return nil
}

// IsInWatchlist checks if a symbol is in the watchlist
func (r *Repository) IsInWatchlist(ctx context.Context, symbol string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM watchlist WHERE symbol = $1)`
	var exists bool
	err := r.db.Pool.QueryRow(ctx, query, symbol).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check watchlist: %w", err)
	}
	return exists, nil
}

// ============================================================================
// MODE SAFETY AND ALLOCATION - WRAPPER METHODS
// ============================================================================

// RecordModeSafetyEvent records a safety control event for a mode
func (r *Repository) RecordModeSafetyEvent(ctx context.Context, event *ModeSafetyHistory) error {
	return r.db.RecordModeSafetyEvent(ctx, event)
}

// GetModeSafetyHistory retrieves safety events for a mode
func (r *Repository) GetModeSafetyHistory(ctx context.Context, mode string, limit int) ([]ModeSafetyHistory, error) {
	return r.db.GetModeSafetyHistory(ctx, mode, limit)
}

// RecordModeAllocation records a capital allocation snapshot for a mode
func (r *Repository) RecordModeAllocation(ctx context.Context, alloc *ModeAllocationHistory) error {
	return r.db.RecordModeAllocation(ctx, alloc)
}

// GetModeAllocationHistory retrieves allocation history for a mode
func (r *Repository) GetModeAllocationHistory(ctx context.Context, mode string, limit int) ([]ModeAllocationHistory, error) {
	return r.db.GetModeAllocationHistory(ctx, mode, limit)
}

// UpdateModePerformanceStats updates or inserts performance stats for a mode
func (r *Repository) UpdateModePerformanceStats(ctx context.Context, stats *ModePerformanceStats) error {
	return r.db.UpdateModePerformanceStats(ctx, stats)
}

// GetModePerformanceStats retrieves performance stats for a mode
func (r *Repository) GetModePerformanceStats(ctx context.Context, mode string) (*ModePerformanceStats, error) {
	return r.db.GetModePerformanceStats(ctx, mode)
}

// GetAllModePerformanceStats retrieves performance stats for all modes
func (r *Repository) GetAllModePerformanceStats(ctx context.Context) (map[string]*ModePerformanceStats, error) {
	return r.db.GetAllModePerformanceStats(ctx)
}
