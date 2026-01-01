package database

import (
	"context"
	"fmt"
	"time"
)

// ==================== FUTURES TRADES ====================

// CreateFuturesTrade creates a new futures trade
func (db *DB) CreateFuturesTrade(ctx context.Context, trade *FuturesTrade) error {
	query := `
		INSERT INTO futures_trades (
			user_id, symbol, position_side, side, entry_price, quantity, leverage,
			margin_type, isolated_margin, liquidation_price, stop_loss, take_profit,
			status, entry_time, trade_source, notes, ai_decision_id,
			strategy_id, strategy_name, trading_mode, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
		) RETURNING id`

	now := time.Now()
	// Handle empty UserID - pass nil for NULL in database
	var userID interface{} = trade.UserID
	if trade.UserID == "" {
		userID = nil
	}
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
		trade.AIDecisionID,
		trade.StrategyID,
		trade.StrategyName,
		trade.TradingMode,
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

// UpdateFuturesTrade updates an existing futures trade
func (db *DB) UpdateFuturesTrade(ctx context.Context, trade *FuturesTrade) error {
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
			updated_at = $13
		WHERE id = $1`

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
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to update futures trade: %w", err)
	}

	trade.UpdatedAt = now
	return nil
}

// GetFuturesTradeByID retrieves a futures trade by ID
func (db *DB) GetFuturesTradeByID(ctx context.Context, id int64) (*FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, ai_decision_id,
			strategy_id, strategy_name, created_at, updated_at
		FROM futures_trades WHERE id = $1`

	trade := &FuturesTrade{}
	err := db.Pool.QueryRow(ctx, query, id).Scan(
		&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
		&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
		&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
		&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
		&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
		&trade.TradeSource, &trade.Notes, &trade.AIDecisionID,
		&trade.StrategyID, &trade.StrategyName, &trade.CreatedAt, &trade.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get futures trade: %w", err)
	}

	return trade, nil
}

// GetOpenFuturesTrades retrieves all open futures positions
func (db *DB) GetOpenFuturesTrades(ctx context.Context) ([]FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, ai_decision_id,
			strategy_id, strategy_name, created_at, updated_at
		FROM futures_trades WHERE status = 'OPEN'
		ORDER BY entry_time DESC`

	rows, err := db.Pool.Query(ctx, query)
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
			&trade.TradeSource, &trade.Notes, &trade.AIDecisionID,
			&trade.StrategyID, &trade.StrategyName, &trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures trade: %w", err)
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// GetOpenFuturesTradeBySymbol retrieves an open futures trade for a specific symbol
func (db *DB) GetOpenFuturesTradeBySymbol(ctx context.Context, symbol string) (*FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, ai_decision_id,
			strategy_id, strategy_name, created_at, updated_at
		FROM futures_trades WHERE symbol = $1 AND status = 'OPEN'
		ORDER BY entry_time DESC
		LIMIT 1`

	trade := &FuturesTrade{}
	err := db.Pool.QueryRow(ctx, query, symbol).Scan(
		&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
		&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
		&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
		&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
		&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
		&trade.TradeSource, &trade.Notes, &trade.AIDecisionID,
		&trade.StrategyID, &trade.StrategyName, &trade.CreatedAt, &trade.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No open trade found
		}
		return nil, fmt.Errorf("failed to get open futures trade by symbol: %w", err)
	}

	return trade, nil
}

// GetFuturesTradeHistory retrieves closed futures trades
func (db *DB) GetFuturesTradeHistory(ctx context.Context, limit, offset int) ([]FuturesTrade, error) {
	query := `
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, ai_decision_id,
			strategy_id, strategy_name, created_at, updated_at
		FROM futures_trades WHERE status != 'OPEN'
		ORDER BY exit_time DESC NULLS LAST
		LIMIT $1 OFFSET $2`

	rows, err := db.Pool.Query(ctx, query, limit, offset)
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
			&trade.TradeSource, &trade.Notes, &trade.AIDecisionID,
			&trade.StrategyID, &trade.StrategyName, &trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures trade: %w", err)
		}
		trades = append(trades, trade)
	}

	return trades, nil
}

// ==================== FUTURES ORDERS ====================

// CreateFuturesOrder creates a new futures order
func (db *DB) CreateFuturesOrder(ctx context.Context, order *FuturesOrder) error {
	query := `
		INSERT INTO futures_orders (
			order_id, symbol, position_side, side, order_type, price, stop_price,
			quantity, time_in_force, reduce_only, close_position, working_type,
			status, futures_trade_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		) RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
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

// UpdateFuturesOrder updates an existing futures order
func (db *DB) UpdateFuturesOrder(ctx context.Context, order *FuturesOrder) error {
	query := `
		UPDATE futures_orders SET
			avg_price = $2,
			executed_qty = $3,
			status = $4,
			filled_at = $5,
			updated_at = $6
		WHERE id = $1`

	now := time.Now()
	_, err := db.Pool.Exec(ctx, query,
		order.ID,
		order.AvgPrice,
		order.ExecutedQty,
		order.Status,
		order.FilledAt,
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to update futures order: %w", err)
	}

	order.UpdatedAt = now
	return nil
}

// GetOpenFuturesOrders retrieves all open futures orders
func (db *DB) GetOpenFuturesOrders(ctx context.Context, symbol string) ([]FuturesOrder, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE status = 'NEW' AND symbol = $1
			ORDER BY created_at DESC`
		args = []interface{}{symbol}
	} else {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE status = 'NEW'
			ORDER BY created_at DESC`
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

// GetFuturesOrderHistory retrieves futures order history
func (db *DB) GetFuturesOrderHistory(ctx context.Context, symbol string, limit, offset int) ([]FuturesOrder, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders WHERE symbol = $1
			ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{symbol, limit, offset}
	} else {
		query = `
			SELECT id, order_id, symbol, position_side, side, order_type, price, avg_price,
				stop_price, quantity, executed_qty, time_in_force, reduce_only, close_position,
				working_type, status, futures_trade_id, created_at, updated_at, filled_at
			FROM futures_orders
			ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
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

// ==================== FUNDING FEES ====================

// CreateFundingFee creates a new funding fee record
func (db *DB) CreateFundingFee(ctx context.Context, fee *FundingFee) error {
	query := `
		INSERT INTO funding_fees (symbol, funding_rate, funding_fee, position_amt, asset, timestamp, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
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

// GetFundingFeeHistory retrieves funding fee history
func (db *DB) GetFundingFeeHistory(ctx context.Context, symbol string, limit, offset int) ([]FundingFee, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `
			SELECT id, symbol, funding_rate, funding_fee, position_amt, asset, timestamp, created_at
			FROM funding_fees WHERE symbol = $1
			ORDER BY timestamp DESC LIMIT $2 OFFSET $3`
		args = []interface{}{symbol, limit, offset}
	} else {
		query = `
			SELECT id, symbol, funding_rate, funding_fee, position_amt, asset, timestamp, created_at
			FROM funding_fees
			ORDER BY timestamp DESC LIMIT $1 OFFSET $2`
		args = []interface{}{limit, offset}
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

// GetTotalFundingFees calculates total funding fees
func (db *DB) GetTotalFundingFees(ctx context.Context, symbol string) (float64, error) {
	var query string
	var args []interface{}

	if symbol != "" {
		query = `SELECT COALESCE(SUM(funding_fee), 0) FROM funding_fees WHERE symbol = $1`
		args = []interface{}{symbol}
	} else {
		query = `SELECT COALESCE(SUM(funding_fee), 0) FROM funding_fees`
	}

	var total float64
	err := db.Pool.QueryRow(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to get total funding fees: %w", err)
	}

	return total, nil
}

// ==================== FUTURES TRANSACTIONS ====================

// CreateFuturesTransaction creates a new futures transaction
func (db *DB) CreateFuturesTransaction(ctx context.Context, tx *FuturesTransaction) error {
	query := `
		INSERT INTO futures_transactions (
			transaction_id, symbol, income_type, income, asset, info, timestamp, futures_trade_id, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (transaction_id) DO NOTHING
		RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
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

// GetFuturesTransactionHistory retrieves futures transaction history
func (db *DB) GetFuturesTransactionHistory(ctx context.Context, symbol string, incomeType string, limit, offset int) ([]FuturesTransaction, error) {
	query := `
		SELECT id, transaction_id, symbol, income_type, income, asset, info, timestamp, futures_trade_id, created_at
		FROM futures_transactions
		WHERE ($1 = '' OR symbol = $1) AND ($2 = '' OR income_type = $2)
		ORDER BY timestamp DESC LIMIT $3 OFFSET $4`

	rows, err := db.Pool.Query(ctx, query, symbol, incomeType, limit, offset)
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

// ==================== ACCOUNT SETTINGS ====================

// GetFuturesAccountSettings retrieves account settings for a symbol
func (db *DB) GetFuturesAccountSettings(ctx context.Context, symbol string) (*FuturesAccountSettings, error) {
	query := `
		SELECT id, symbol, leverage, margin_type, position_mode, created_at, updated_at
		FROM futures_account_settings WHERE symbol = $1`

	settings := &FuturesAccountSettings{}
	err := db.Pool.QueryRow(ctx, query, symbol).Scan(
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

// UpsertFuturesAccountSettings creates or updates account settings
func (db *DB) UpsertFuturesAccountSettings(ctx context.Context, settings *FuturesAccountSettings) error {
	query := `
		INSERT INTO futures_account_settings (symbol, leverage, margin_type, position_mode, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (symbol) DO UPDATE SET
			leverage = EXCLUDED.leverage,
			margin_type = EXCLUDED.margin_type,
			position_mode = EXCLUDED.position_mode,
			updated_at = EXCLUDED.updated_at
		RETURNING id`

	now := time.Now()
	err := db.Pool.QueryRow(ctx, query,
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

// ==================== METRICS ====================

// GetFuturesTradeHistoryWithAI retrieves futures trades with AI decisions
func (db *DB) GetFuturesTradeHistoryWithAI(ctx context.Context, limit, offset int, includeOpen bool) ([]FuturesTrade, error) {
	statusFilter := "status != 'OPEN'"
	if includeOpen {
		statusFilter = "1=1"
	}

	query := fmt.Sprintf(`
		SELECT id, symbol, position_side, side, entry_price, exit_price, mark_price,
			quantity, leverage, margin_type, isolated_margin, realized_pnl, unrealized_pnl,
			realized_pnl_percent, liquidation_price, stop_loss, take_profit, trailing_stop,
			status, entry_time, exit_time, trade_source, notes, COALESCE(ai_decision_id, 0), created_at, updated_at
		FROM futures_trades WHERE %s
		ORDER BY COALESCE(exit_time, entry_time) DESC
		LIMIT $1 OFFSET $2`, statusFilter)

	rows, err := db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get futures trade history: %w", err)
	}
	defer rows.Close()

	var trades []FuturesTrade
	for rows.Next() {
		var trade FuturesTrade
		var aiDecisionID int64
		err := rows.Scan(
			&trade.ID, &trade.Symbol, &trade.PositionSide, &trade.Side, &trade.EntryPrice,
			&trade.ExitPrice, &trade.MarkPrice, &trade.Quantity, &trade.Leverage,
			&trade.MarginType, &trade.IsolatedMargin, &trade.RealizedPnL, &trade.UnrealizedPnL,
			&trade.RealizedPnLPercent, &trade.LiquidationPrice, &trade.StopLoss, &trade.TakeProfit,
			&trade.TrailingStop, &trade.Status, &trade.EntryTime, &trade.ExitTime,
			&trade.TradeSource, &trade.Notes, &aiDecisionID, &trade.CreatedAt, &trade.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan futures trade: %w", err)
		}

		// Load AI decision if present
		if aiDecisionID > 0 {
			trade.AIDecisionID = &aiDecisionID
			// Get the AI decision directly
			aiDecision, err := db.getAIDecisionByID(ctx, aiDecisionID)
			if err == nil && aiDecision != nil {
				trade.AIDecision = aiDecision
			}
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

// LinkFuturesTradeToAIDecision links a futures trade to an AI decision
func (db *DB) LinkFuturesTradeToAIDecision(ctx context.Context, tradeID, aiDecisionID int64) error {
	query := `UPDATE futures_trades SET ai_decision_id = $1 WHERE id = $2`
	_, err := db.Pool.Exec(ctx, query, aiDecisionID, tradeID)
	if err != nil {
		return fmt.Errorf("failed to link futures trade to AI decision: %w", err)
	}
	return nil
}

// GetFuturesTradingMetrics calculates trading metrics
func (db *DB) GetFuturesTradingMetrics(ctx context.Context) (*FuturesTradingMetrics, error) {
	metrics := &FuturesTradingMetrics{}

	// Get total trades count
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status != 'OPEN'`).Scan(&metrics.TotalTrades)
	if err != nil {
		return nil, err
	}

	// Get winning/losing trades
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl > 0`).Scan(&metrics.WinningTrades)
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl < 0`).Scan(&metrics.LosingTrades)

	// Calculate win rate
	if metrics.TotalTrades > 0 {
		metrics.WinRate = float64(metrics.WinningTrades) / float64(metrics.TotalTrades) * 100
	}

	// Get PnL stats
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(realized_pnl), 0) FROM futures_trades WHERE status != 'OPEN'`).Scan(&metrics.TotalRealizedPnL)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(unrealized_pnl), 0) FROM futures_trades WHERE status = 'OPEN'`).Scan(&metrics.TotalUnrealizedPnL)

	// Get funding fees total
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(funding_fee), 0) FROM funding_fees`).Scan(&metrics.TotalFundingFees)

	// Get averages
	if metrics.TotalTrades > 0 {
		metrics.AveragePnL = metrics.TotalRealizedPnL / float64(metrics.TotalTrades)
	}
	db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl > 0`).Scan(&metrics.AverageWin)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl < 0`).Scan(&metrics.AverageLoss)

	// Get largest win/loss
	db.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED'`).Scan(&metrics.LargestWin)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(MIN(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED'`).Scan(&metrics.LargestLoss)

	// Calculate profit factor
	var totalWins, totalLosses float64
	db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(realized_pnl), 0) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl > 0`).Scan(&totalWins)
	db.Pool.QueryRow(ctx, `SELECT COALESCE(ABS(SUM(realized_pnl)), 1) FROM futures_trades WHERE status = 'CLOSED' AND realized_pnl < 0`).Scan(&totalLosses)
	if totalLosses > 0 {
		metrics.ProfitFactor = totalWins / totalLosses
	}

	// Get average leverage
	db.Pool.QueryRow(ctx, `SELECT COALESCE(AVG(leverage), 10) FROM futures_trades`).Scan(&metrics.AverageLeverage)

	// Get open positions count
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_trades WHERE status = 'OPEN'`).Scan(&metrics.OpenPositions)

	// Get open orders count
	db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM futures_orders WHERE status = 'NEW'`).Scan(&metrics.OpenOrders)

	// Get last trade time
	db.Pool.QueryRow(ctx, `SELECT MAX(exit_time) FROM futures_trades WHERE status != 'OPEN'`).Scan(&metrics.LastTradeTime)

	return metrics, nil
}

// getAIDecisionByID is a helper to get an AI decision by ID (for futures trades)
func (db *DB) getAIDecisionByID(ctx context.Context, id int64) (*AIDecision, error) {
	query := `
		SELECT id, symbol, current_price, action, confidence, reasoning,
			ml_direction, ml_confidence, sentiment_direction, sentiment_confidence,
			llm_direction, llm_confidence, pattern_direction, pattern_confidence,
			bigcandle_direction, bigcandle_confidence, confluence_count, risk_level, executed, created_at
		FROM ai_decisions WHERE id = $1`

	var d AIDecision
	err := db.Pool.QueryRow(ctx, query, id).Scan(
		&d.ID, &d.Symbol, &d.CurrentPrice, &d.Action, &d.Confidence, &d.Reasoning,
		&d.MLDirection, &d.MLConfidence, &d.SentimentDirection, &d.SentimentConfidence,
		&d.LLMDirection, &d.LLMConfidence, &d.PatternDirection, &d.PatternConfidence,
		&d.BigCandleDirection, &d.BigCandleConfidence, &d.ConfluenceCount, &d.RiskLevel, &d.Executed, &d.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// === MODE SAFETY AND ALLOCATION TRACKING ===

// RecordModeSafetyEvent records a safety control event for a mode
func (db *DB) RecordModeSafetyEvent(ctx context.Context, event *ModeSafetyHistory) error {
	if db.Pool == nil {
		return nil // No database configured
	}

	query := `
		INSERT INTO mode_safety_history (mode, event_type, trigger_reason, win_rate, profit_window_pct, trades_per_minute, pause_duration_mins)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := db.Pool.Exec(ctx, query,
		event.Mode,
		event.EventType,
		event.TriggerReason,
		event.WinRate,
		event.ProfitWindowPct,
		event.TradesPerMinute,
		event.PauseDurationMins,
	)

	return err
}

// GetModeSafetyHistory retrieves safety events for a mode
func (db *DB) GetModeSafetyHistory(ctx context.Context, mode string, limit int) ([]ModeSafetyHistory, error) {
	if db.Pool == nil {
		return []ModeSafetyHistory{}, nil
	}

	query := `
		SELECT id, mode, event_type, trigger_reason, win_rate, profit_window_pct, trades_per_minute, pause_duration_mins, created_at
		FROM mode_safety_history
		WHERE mode = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.Pool.Query(ctx, query, mode, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ModeSafetyHistory
	for rows.Next() {
		var event ModeSafetyHistory
		if err := rows.Scan(
			&event.ID, &event.Mode, &event.EventType, &event.TriggerReason,
			&event.WinRate, &event.ProfitWindowPct, &event.TradesPerMinute,
			&event.PauseDurationMins, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// RecordModeAllocation records a capital allocation snapshot for a mode
func (db *DB) RecordModeAllocation(ctx context.Context, alloc *ModeAllocationHistory) error {
	if db.Pool == nil {
		return nil // No database configured
	}

	query := `
		INSERT INTO mode_allocation_history (mode, allocated_percent, allocated_usd, used_usd, available_usd, current_positions, max_positions, capacity_percent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := db.Pool.Exec(ctx, query,
		alloc.Mode,
		alloc.AllocatedPercent,
		alloc.AllocatedUSD,
		alloc.UsedUSD,
		alloc.AvailableUSD,
		alloc.CurrentPositions,
		alloc.MaxPositions,
		alloc.CapacityPercent,
	)

	return err
}

// GetModeAllocationHistory retrieves allocation history for a mode
func (db *DB) GetModeAllocationHistory(ctx context.Context, mode string, limit int) ([]ModeAllocationHistory, error) {
	if db.Pool == nil {
		return []ModeAllocationHistory{}, nil
	}

	query := `
		SELECT id, mode, allocated_percent, allocated_usd, used_usd, available_usd, current_positions, max_positions, capacity_percent, created_at
		FROM mode_allocation_history
		WHERE mode = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := db.Pool.Query(ctx, query, mode, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var allocations []ModeAllocationHistory
	for rows.Next() {
		var alloc ModeAllocationHistory
		if err := rows.Scan(
			&alloc.ID, &alloc.Mode, &alloc.AllocatedPercent, &alloc.AllocatedUSD,
			&alloc.UsedUSD, &alloc.AvailableUSD, &alloc.CurrentPositions,
			&alloc.MaxPositions, &alloc.CapacityPercent, &alloc.CreatedAt,
		); err != nil {
			return nil, err
		}
		allocations = append(allocations, alloc)
	}

	return allocations, rows.Err()
}

// UpdateModePerformanceStats updates or inserts performance stats for a mode
func (db *DB) UpdateModePerformanceStats(ctx context.Context, stats *ModePerformanceStats) error {
	if db.Pool == nil {
		return nil // No database configured
	}

	query := `
		INSERT INTO mode_performance_stats (mode, total_trades, winning_trades, losing_trades, win_rate, total_pnl_usd, total_pnl_percent, avg_pnl_per_trade, max_drawdown_usd, max_drawdown_percent, avg_hold_seconds, last_trade_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (mode) DO UPDATE SET
			total_trades = $2,
			winning_trades = $3,
			losing_trades = $4,
			win_rate = $5,
			total_pnl_usd = $6,
			total_pnl_percent = $7,
			avg_pnl_per_trade = $8,
			max_drawdown_usd = $9,
			max_drawdown_percent = $10,
			avg_hold_seconds = $11,
			last_trade_time = $12,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.Pool.Exec(ctx, query,
		stats.Mode,
		stats.TotalTrades,
		stats.WinningTrades,
		stats.LosingTrades,
		stats.WinRate,
		stats.TotalPnLUSD,
		stats.TotalPnLPercent,
		stats.AvgPnLPerTrade,
		stats.MaxDrawdownUSD,
		stats.MaxDrawdownPercent,
		stats.AvgHoldSeconds,
		stats.LastTradeTime,
	)

	return err
}

// GetModePerformanceStats retrieves performance stats for a mode
func (db *DB) GetModePerformanceStats(ctx context.Context, mode string) (*ModePerformanceStats, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, mode, total_trades, winning_trades, losing_trades, win_rate, total_pnl_usd, total_pnl_percent, avg_pnl_per_trade, max_drawdown_usd, max_drawdown_percent, avg_hold_seconds, last_trade_time, updated_at
		FROM mode_performance_stats
		WHERE mode = $1
	`

	row := db.Pool.QueryRow(ctx, query, mode)

	var stats ModePerformanceStats
	err := row.Scan(
		&stats.ID, &stats.Mode, &stats.TotalTrades, &stats.WinningTrades, &stats.LosingTrades,
		&stats.WinRate, &stats.TotalPnLUSD, &stats.TotalPnLPercent, &stats.AvgPnLPerTrade,
		&stats.MaxDrawdownUSD, &stats.MaxDrawdownPercent, &stats.AvgHoldSeconds,
		&stats.LastTradeTime, &stats.UpdatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // No stats yet
		}
		return nil, err
	}

	return &stats, nil
}

// GetAllModePerformanceStats retrieves performance stats for all modes
func (db *DB) GetAllModePerformanceStats(ctx context.Context) (map[string]*ModePerformanceStats, error) {
	if db.Pool == nil {
		return make(map[string]*ModePerformanceStats), nil
	}

	query := `
		SELECT id, mode, total_trades, winning_trades, losing_trades, win_rate, total_pnl_usd, total_pnl_percent, avg_pnl_per_trade, max_drawdown_usd, max_drawdown_percent, avg_hold_seconds, last_trade_time, updated_at
		FROM mode_performance_stats
		ORDER BY mode
	`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statsMap := make(map[string]*ModePerformanceStats)
	for rows.Next() {
		var stats ModePerformanceStats
		if err := rows.Scan(
			&stats.ID, &stats.Mode, &stats.TotalTrades, &stats.WinningTrades, &stats.LosingTrades,
			&stats.WinRate, &stats.TotalPnLUSD, &stats.TotalPnLPercent, &stats.AvgPnLPerTrade,
			&stats.MaxDrawdownUSD, &stats.MaxDrawdownPercent, &stats.AvgHoldSeconds,
			&stats.LastTradeTime, &stats.UpdatedAt,
		); err != nil {
			return nil, err
		}
		statsCopy := stats
		statsMap[stats.Mode] = &statsCopy
	}

	return statsMap, rows.Err()
}
