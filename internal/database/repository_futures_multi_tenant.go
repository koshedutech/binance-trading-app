package database

import (
	"context"
	"fmt"
	"time"
)

// ==================== USER-SCOPED FUTURES TRADES ====================

// CreateFuturesTradeForUser creates a new futures trade for a specific user
func (db *DB) CreateFuturesTradeForUser(ctx context.Context, userID string, trade *FuturesTrade) error {
	query := `
		INSERT INTO futures_trades (
			user_id, symbol, position_side, side, entry_price, quantity, leverage,
			margin_type, isolated_margin, liquidation_price, stop_loss, take_profit,
			status, entry_time, trade_source, notes, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		) RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
		userID,
		trade.Symbol,
		trade.PositionSide,
		trade.Side,
		trade.EntryPrice,
		trade.Quantity,
		trade.Leverage,
		trade.MarginType,
		trade.IsolatedMargin,
		trade.LiquidationPrice,
		trade.StopLoss,
		trade.TakeProfit,
		trade.Status,
		trade.EntryTime,
		trade.TradeSource,
		trade.Notes,
		now,
		now,
	).Scan(&trade.ID)

	if err != nil {
		return fmt.Errorf("failed to create futures trade: %w", err)
	}

	trade.CreatedAt = now
	trade.UpdatedAt = now
	return nil
}

// UpdateFuturesTradeForUser updates an existing futures trade for a specific user
func (db *DB) UpdateFuturesTradeForUser(ctx context.Context, userID string, trade *FuturesTrade) error {
	query := `
		UPDATE futures_trades SET
			exit_price = $2,
			mark_price = $3,
			realized_pnl = $4,
			unrealized_pnl = $5,
			realized_pnl_percent = $6,
			stop_loss = $7,
			take_profit = $8,
			trailing_stop = $9,
			status = $10,
			exit_time = $11,
			notes = $12,
			trading_mode = $13,
			hedge_mode_active = $14,
			updated_at = $15
		WHERE id = $1 AND user_id = $16`

	now := time.Now()
	_, err := db.Pool.Exec(ctx, query,
		trade.ID,
		trade.ExitPrice,
		trade.MarkPrice,
		trade.RealizedPnL,
		trade.UnrealizedPnL,
		trade.RealizedPnLPercent,
		trade.StopLoss,
		trade.TakeProfit,
		trade.TrailingStop,
		trade.Status,
		trade.ExitTime,
		trade.Notes,
		trade.TradingMode,
		trade.HedgeModeActive,
		now,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update futures trade: %w", err)
	}

	trade.UpdatedAt = now
	return nil
}

// GetFuturesTradeByIDForUser retrieves a futures trade by ID for a specific user
func (db *DB) GetFuturesTradeByIDForUser(ctx context.Context, userID string, id int64) (*FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, created_at, updated_at
		FROM futures_trades WHERE id = $1 AND user_id = $2`

	trade := &FuturesTrade{}
	err := db.Pool.QueryRow(ctx, query, id, userID).Scan(
		&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
		&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
		&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
		&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
		&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
		&trade.TradeSource, &trade.Notes, &trade.CreatedAt, &trade.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get futures trade: %w", err)
	}

	return trade, nil
}

// GetOpenFuturesTradesForUser retrieves all open futures positions for a specific user
func (db *DB) GetOpenFuturesTradesForUser(ctx context.Context, userID string) ([]FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, created_at, updated_at
		FROM futures_trades WHERE status = 'OPEN' AND user_id = $1
		ORDER BY entry_time DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get open futures trades: %w", err)
	}
	defer rows.Close()

	var trades []FuturesTrade
	for rows.Next() {
		var trade FuturesTrade
		err := rows.Scan(
			&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
			&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
			&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
			&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
			&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
			&trade.TradeSource, &trade.Notes, &trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures trade: %w", err)
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetFuturesTradeHistoryForUser retrieves closed futures trades for a specific user
func (db *DB) GetFuturesTradeHistoryForUser(ctx context.Context, userID string, limit, offset int) ([]FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, created_at, updated_at
		FROM futures_trades WHERE status != 'OPEN' AND user_id = $1
		ORDER BY exit_time DESC NULLS LAST
		LIMIT $2 OFFSET $3`

	rows, err := db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get futures trade history: %w", err)
	}
	defer rows.Close()

	var trades []FuturesTrade
	for rows.Next() {
		var trade FuturesTrade
		err := rows.Scan(
			&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
			&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
			&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
			&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
			&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
			&trade.TradeSource, &trade.Notes, &trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures trade: %w", err)
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// CountOpenFuturesTradesForUser counts open futures trades for a user
func (db *DB) CountOpenFuturesTradesForUser(ctx context.Context, userID string) (int, error) {
	query := `SELECT COUNT(*) FROM futures_trades WHERE status = 'OPEN' AND user_id = $1`
	var count int
	err := db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// GetOpenFuturesTradeBySymbolForUser retrieves an open futures trade for a specific symbol and user
// Returns nil, nil if no open trade exists for this symbol/user combination
func (db *DB) GetOpenFuturesTradeBySymbolForUser(ctx context.Context, userID, symbol string) (*FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, created_at, updated_at
		FROM futures_trades
		WHERE status = 'OPEN' AND user_id = $1 AND symbol = $2
		ORDER BY entry_time DESC
		LIMIT 1`

	var trade FuturesTrade
	err := db.Pool.QueryRow(ctx, query, userID, symbol).Scan(
		&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
		&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
		&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
		&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
		&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
		&trade.TradeSource, &trade.Notes, &trade.CreatedAt, &trade.UpdatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No open trade exists - this is not an error
		}
		return nil, fmt.Errorf("failed to get open futures trade by symbol: %w", err)
	}
	return &trade, nil
}

// SymbolPerformanceStats holds aggregated performance metrics for a single symbol
type SymbolPerformanceStats struct {
	Symbol        string
	TotalTrades   int
	WinningTrades int
	LosingTrades  int
	TotalPnL      float64
	AvgPnL        float64
	AvgWin        float64
	AvgLoss       float64
}

// GetSymbolPerformanceStatsForUser aggregates performance metrics by symbol from closed trades
func (db *DB) GetSymbolPerformanceStatsForUser(ctx context.Context, userID string) (map[string]*SymbolPerformanceStats, error) {
	query := `
		SELECT
			symbol,
			COUNT(*) as total_trades,
			COUNT(CASE WHEN realized_pnl > 0 THEN 1 END) as winning_trades,
			COUNT(CASE WHEN realized_pnl <= 0 THEN 1 END) as losing_trades,
			COALESCE(SUM(realized_pnl), 0) as total_pnl,
			COALESCE(AVG(realized_pnl), 0) as avg_pnl,
			COALESCE(AVG(CASE WHEN realized_pnl > 0 THEN realized_pnl END), 0) as avg_win,
			COALESCE(ABS(AVG(CASE WHEN realized_pnl <= 0 THEN realized_pnl END)), 0) as avg_loss
		FROM futures_trades
		WHERE user_id = $1 AND status IN ('CLOSED', 'closed', 'LIQUIDATED', 'liquidated')
		GROUP BY symbol
		ORDER BY total_pnl DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol performance stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]*SymbolPerformanceStats)
	for rows.Next() {
		s := &SymbolPerformanceStats{}
		err := rows.Scan(
			&s.Symbol,
			&s.TotalTrades,
			&s.WinningTrades,
			&s.LosingTrades,
			&s.TotalPnL,
			&s.AvgPnL,
			&s.AvgWin,
			&s.AvgLoss,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan symbol performance stats: %w", err)
		}
		stats[s.Symbol] = s
	}

	return stats, nil
}

// ==================== USER-SCOPED FUTURES ORDERS ====================

// CreateFuturesOrderForUser creates a new futures order for a specific user
func (db *DB) CreateFuturesOrderForUser(ctx context.Context, userID string, order *FuturesOrder) error {
	query := `
		INSERT INTO futures_orders (
			user_id, order_id, symbol, position_side, side, order_type, price, stop_price,
			quantity, time_in_force, reduce_only, close_position, working_type,
			status, futures_trade_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		) RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
		userID,
		order.OrderID,
		order.Symbol,
		order.PositionSide,
		order.Side,
		order.OrderType,
		order.Price,
		order.StopPrice,
		order.Quantity,
		order.TimeInForce,
		order.ReduceOnly,
		order.ClosePosition,
		order.WorkingType,
		order.Status,
		order.FuturesTradeID,
		now,
		now,
	).Scan(&order.ID)

	if err != nil {
		return fmt.Errorf("failed to create futures order: %w", err)
	}

	order.CreatedAt = now
	order.UpdatedAt = now
	return nil
}

// UpdateFuturesOrderForUser updates an existing futures order for a specific user
func (db *DB) UpdateFuturesOrderForUser(ctx context.Context, userID string, order *FuturesOrder) error {
	query := `
		UPDATE futures_orders SET
			avg_price = $2,
			executed_qty = $3,
			status = $4,
			filled_at = $5,
			updated_at = $6
		WHERE id = $1 AND user_id = $7`

	now := time.Now()
	_, err := db.Pool.Exec(ctx, query,
		order.ID,
		order.AvgPrice,
		order.ExecutedQty,
		order.Status,
		order.FilledAt,
		now,
		userID,
	)

	if err != nil {
		return fmt.Errorf("failed to update futures order: %w", err)
	}

	order.UpdatedAt = now
	return nil
}

// GetOpenFuturesOrdersForUser retrieves all open futures orders for a specific user
func (db *DB) GetOpenFuturesOrdersForUser(ctx context.Context, userID string, symbol string) ([]FuturesOrder, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE status = 'NEW' AND user_id = $1 AND symbol = $2
			ORDER BY created_at DESC`
		args = []interface{}{userID, symbol}
	} else {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE status = 'NEW' AND user_id = $1
			ORDER BY created_at DESC`
		args = []interface{}{userID}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get open futures orders: %w", err)
	}
	defer rows.Close()

	var orders []FuturesOrder
	for rows.Next() {
		var order FuturesOrder
		err := rows.Scan(
			&order.ID, &order.OrderID, &order.Symbol, &order.PositionSide, &order.Side,
			&order.OrderType, &order.Price, &order.AvgPrice, &order.StopPrice, &order.Quantity,
			&order.ExecutedQty, &order.TimeInForce, &order.ReduceOnly, &order.ClosePosition,
			&order.WorkingType, &order.Status, &order.FuturesTradeID, &order.CreatedAt,
			&order.UpdatedAt, &order.FilledAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, nil
}

// GetFuturesOrderHistoryForUser retrieves futures order history for a specific user
func (db *DB) GetFuturesOrderHistoryForUser(ctx context.Context, userID string, symbol string, limit, offset int) ([]FuturesOrder, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE user_id = $1 AND symbol = $2
			ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		args = []interface{}{userID, symbol, limit, offset}
	} else {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE user_id = $1
			ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{userID, limit, offset}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get futures order history: %w", err)
	}
	defer rows.Close()

	var orders []FuturesOrder
	for rows.Next() {
		var order FuturesOrder
		err := rows.Scan(
			&order.ID, &order.OrderID, &order.Symbol, &order.PositionSide, &order.Side,
			&order.OrderType, &order.Price, &order.AvgPrice, &order.StopPrice, &order.Quantity,
			&order.ExecutedQty, &order.TimeInForce, &order.ReduceOnly, &order.ClosePosition,
			&order.WorkingType, &order.Status, &order.FuturesTradeID, &order.CreatedAt,
			&order.UpdatedAt, &order.FilledAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures order: %w", err)
		}
		orders = append(orders, order)
	}

	return orders, nil
}

// ==================== USER-SCOPED FUNDING FEES ====================

// CreateFundingFeeForUser creates a new funding fee record for a specific user
func (db *DB) CreateFundingFeeForUser(ctx context.Context, userID string, fee *FundingFee) error {
	query := `
		INSERT INTO funding_fees (user_id, symbol, funding_rate, funding_fee, position_amt, asset, timestamp, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
		userID,
		fee.Symbol,
		fee.FundingRate,
		fee.FundingFee,
		fee.PositionAmt,
		fee.Asset,
		fee.Timestamp,
		now,
	).Scan(&fee.ID)

	if err != nil {
		return fmt.Errorf("failed to create funding fee: %w", err)
	}

	fee.CreatedAt = now
	return nil
}

// GetFundingFeeHistoryForUser retrieves funding fee history for a specific user
func (db *DB) GetFundingFeeHistoryForUser(ctx context.Context, userID string, symbol string, limit, offset int) ([]FundingFee, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, symbol, funding_rate, funding_fee, position_amt, asset, timestamp, created_at
			FROM funding_fees WHERE user_id = $1 AND symbol = $2
			ORDER BY timestamp DESC LIMIT $3 OFFSET $4`
		args = []interface{}{userID, symbol, limit, offset}
	} else {
		query = `
			SELECT id, symbol, funding_rate, funding_fee, position_amt, asset, timestamp, created_at
			FROM funding_fees WHERE user_id = $1
			ORDER BY timestamp DESC LIMIT $2 OFFSET $3`
		args = []interface{}{userID, limit, offset}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get funding fee history: %w", err)
	}
	defer rows.Close()

	var fees []FundingFee
	for rows.Next() {
		var fee FundingFee
		err := rows.Scan(
			&fee.ID, &fee.Symbol, &fee.FundingRate, &fee.FundingFee,
			&fee.PositionAmt, &fee.Asset, &fee.Timestamp, &fee.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan funding fee: %w", err)
		}
		fees = append(fees, fee)
	}

	return fees, nil
}

// GetTotalFundingFeesForUser calculates total funding fees for a specific user
func (db *DB) GetTotalFundingFeesForUser(ctx context.Context, userID string, symbol string) (float64, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `SELECT COALESCE(SUM(funding_fee), 0) FROM funding_fees WHERE user_id = $1 AND symbol = $2`
		args = []interface{}{userID, symbol}
	} else {
		query = `SELECT COALESCE(SUM(funding_fee), 0) FROM funding_fees WHERE user_id = $1`
		args = []interface{}{userID}
	}

	var total float64
	err := db.Pool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total funding fees: %w", err)
	}

	return total, nil
}

// ==================== USER-SCOPED FUTURES TRANSACTIONS ====================

// CreateFuturesTransactionForUser creates a new futures transaction for a specific user
func (db *DB) CreateFuturesTransactionForUser(ctx context.Context, userID string, tx *FuturesTransaction) error {
	query := `
		INSERT INTO futures_transactions (
			user_id, transaction_id, symbol, income_type, income, asset, info, timestamp, futures_trade_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (transaction_id) DO NOTHING
		RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
		userID,
		tx.TransactionID,
		tx.Symbol,
		tx.IncomeType,
		tx.Income,
		tx.Asset,
		tx.Info,
		tx.Timestamp,
		tx.FuturesTradeID,
		now,
	).Scan(&tx.ID)

	if err != nil {
		// Ignore if transaction already exists
		return nil
	}

	tx.CreatedAt = now
	return nil
}

// GetFuturesTransactionHistoryForUser retrieves futures transaction history for a specific user
func (db *DB) GetFuturesTransactionHistoryForUser(ctx context.Context, userID string, symbol string, incomeType string, limit, offset int) ([]FuturesTransaction, error) {
	query := `
		SELECT id, transaction_id, symbol, income_type, income, asset, info, timestamp, futures_trade_id, created_at
		FROM futures_transactions
		WHERE user_id = $1 AND ($2 = '' OR symbol = $2) AND ($3 = '' OR income_type = $3)
		ORDER BY timestamp DESC LIMIT $4 OFFSET $5`

	rows, err := db.Pool.Query(ctx, query, userID, symbol, incomeType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get futures transaction history: %w", err)
	}
	defer rows.Close()

	var transactions []FuturesTransaction
	for rows.Next() {
		var tx FuturesTransaction
		err := rows.Scan(
			&tx.ID, &tx.TransactionID, &tx.Symbol, &tx.IncomeType, &tx.Income,
			&tx.Asset, &tx.Info, &tx.Timestamp, &tx.FuturesTradeID, &tx.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// ==================== USER-SCOPED ACCOUNT SETTINGS ====================

// GetFuturesAccountSettingsForUser retrieves account settings for a user and symbol
func (db *DB) GetFuturesAccountSettingsForUser(ctx context.Context, userID string, symbol string) (*FuturesAccountSettings, error) {
	query := `
		SELECT id, symbol, leverage, margin_type, position_mode, created_at, updated_at
		FROM futures_account_settings WHERE user_id = $1 AND symbol = $2`

	settings := &FuturesAccountSettings{}
	err := db.Pool.QueryRow(ctx, query, userID, symbol).Scan(
		&settings.ID, &settings.Symbol, &settings.Leverage,
		&settings.MarginType, &settings.PositionMode, &settings.CreatedAt, &settings.UpdatedAt,
	)

	if err != nil {
		// Return default settings if not found
		return &FuturesAccountSettings{
			Symbol:       symbol,
			Leverage:     10,
			MarginType:   "CROSSED",
			PositionMode: "ONE_WAY",
		}, nil
	}

	return settings, nil
}

// UpsertFuturesAccountSettingsForUser creates or updates account settings for a user
func (db *DB) UpsertFuturesAccountSettingsForUser(ctx context.Context, userID string, settings *FuturesAccountSettings) error {
	query := `
		INSERT INTO futures_account_settings (user_id, symbol, leverage, margin_type, position_mode, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, symbol) DO UPDATE SET
			leverage = EXCLUDED.leverage,
			margin_type = EXCLUDED.margin_type,
			position_mode = EXCLUDED.position_mode,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
		userID,
		settings.Symbol,
		settings.Leverage,
		settings.MarginType,
		settings.PositionMode,
		now,
		now,
	).Scan(&settings.ID)

	if err != nil {
		return fmt.Errorf("failed to upsert account settings: %w", err)
	}

	settings.UpdatedAt = now
	return nil
}

// ==================== USER-SCOPED METRICS ====================

// GetFuturesTradingMetricsForUser calculates trading metrics for a specific user
func (db *DB) GetFuturesTradingMetricsForUser(ctx context.Context, userID string) (*FuturesTradingMetrics, error) {
	metrics := &FuturesTradingMetrics{}

	// Get total trades count
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status != 'OPEN' AND user_id = $1`, userID).Scan(&metrics.TotalTrades)
	if err != nil {
		return nil, err
	}

	// Get winning/losing trades
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl > 0 AND user_id = $1`, userID).Scan(&metrics.WinningTrades)
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl < 0 AND user_id = $1`, userID).Scan(&metrics.LosingTrades)

	// Calculate win rate
	if metrics.TotalTrades > 0 {
		metrics.WinRate = float64(metrics.WinningTrades) / float64(metrics.TotalTrades) * 100
	}

	// Get PnL stats
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(realized_pnl), 0) FROM futures_trades WHERE status != 'OPEN' AND user_id = $1`, userID).Scan(&metrics.TotalRealizedPnL)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(unrealized_pnl), 0) FROM futures_trades WHERE status = 'OPEN' AND user_id = $1`, userID).Scan(&metrics.TotalUnrealizedPnL)

	// Get funding fees total
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(funding_fee), 0) FROM funding_fees WHERE user_id = $1`, userID).Scan(&metrics.TotalFundingFees)

	// Get averages
	if metrics.TotalTrades > 0 {
		metrics.AveragePnL = metrics.TotalRealizedPnL / float64(metrics.TotalTrades)
	}
	db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl > 0 AND user_id = $1`, userID).Scan(&metrics.AverageWin)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl < 0 AND user_id = $1`, userID).Scan(&metrics.AverageLoss)

	// Get largest win/loss
	db.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND user_id = $1`, userID).Scan(&metrics.LargestWin)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(MIN(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND user_id = $1`, userID).Scan(&metrics.LargestLoss)

	// Calculate profit factor
	var totalWins, totalLosses float64
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl > 0 AND user_id = $1`, userID).Scan(&totalWins)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(ABS(SUM(realized_pnl)), 1) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl < 0 AND user_id = $1`, userID).Scan(&totalLosses)
	if totalLosses > 0 {
		metrics.ProfitFactor = totalWins / totalLosses
	}

	// Get average leverage
	db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(leverage), 10) FROM futures_trades WHERE user_id = $1`, userID).Scan(&metrics.AverageLeverage)

	// Get open positions count
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'OPEN' AND user_id = $1`, userID).Scan(&metrics.OpenPositions)

	// Get open orders count
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_orders WHERE status = 'NEW' AND user_id = $1`, userID).Scan(&metrics.OpenOrders)

	// Get last trade time
	db.Pool.QueryRow(ctx, `SELECT MAX(exit_time) FROM futures_trades WHERE status != 'OPEN' AND user_id = $1`, userID).Scan(&metrics.LastTradeTime)

	return metrics, nil
}

// ==================== DAILY STATS FOR USER LIMITS ====================

// GetDailyFuturesTradeCountForUser gets the number of futures trades placed today for a user
func (db *DB) GetDailyFuturesTradeCountForUser(ctx context.Context, userID string) (int, error) {
	query := `
		SELECT COUNT(*) FROM futures_trades
		WHERE user_id = $1 AND entry_time >= CURRENT_DATE
	`
	var count int
	err := db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	return count, err
}

// GetDailyFuturesLossForUser gets the total loss for today for a user in futures
func (db *DB) GetDailyFuturesLossForUser(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(realized_pnl), 0) FROM futures_trades
		WHERE user_id = $1 AND exit_time >= CURRENT_DATE AND realized_pnl < 0
	`
	var loss float64
	err := db.Pool.QueryRow(ctx, query, userID).Scan(&loss)
	return loss, err
}

// GetDailyFuturesPnLForUser gets the total PnL for today for a user in futures
func (db *DB) GetDailyFuturesPnLForUser(ctx context.Context, userID string) (float64, error) {
	query := `
		SELECT COALESCE(SUM(realized_pnl), 0) FROM futures_trades
		WHERE user_id = $1 AND exit_time >= CURRENT_DATE
	`
	var pnl float64
	err := db.Pool.QueryRow(ctx, query, userID).Scan(&pnl)
	return pnl, err
}
