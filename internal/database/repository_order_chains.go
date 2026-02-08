package database

import (
	"context"
	"fmt"
	"time"

	"binance-trading-bot/internal/orders"

	"github.com/jackc/pgx/v5"
)

// CreateOrderChain inserts a new order chain record
func (db *DB) CreateOrderChain(ctx context.Context, chain *orders.OrderChain) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		INSERT INTO order_chains (
			user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason, close_price,
			realized_pnl, total_fees,
			sl_binance_order_id, sl_limit_price, sl_fill_price, sl_fill_time, sl_status, sl_quantity,
			tp_binance_order_id, tp_limit_price, tp_fill_price, tp_fill_time, tp_status, tp_quantity,
			mode, strategy_group, sub_strategy, timeframe,
			entry_context
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12,
			$13, $14, $15, $16,
			$17,
			$18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, $27, $28, $29,
			$30, $31,
			$32, $33, $34, $35, $36, $37,
			$38, $39, $40, $41, $42, $43,
			$44, $45, $46, $47,
			$48
		)
		ON CONFLICT (user_id, chain_id) DO UPDATE SET
			status = EXCLUDED.status,
			entry_price = EXCLUDED.entry_price,
			entry_quantity = EXCLUDED.entry_quantity,
			entry_filled_at = EXCLUDED.entry_filled_at,
			entry_binance_order_id = EXCLUDED.entry_binance_order_id,
			current_sl_price = EXCLUDED.current_sl_price,
			current_tp_price = EXCLUDED.current_tp_price,
			position_opt_enabled = EXCLUDED.position_opt_enabled,
			current_tp1_price = EXCLUDED.current_tp1_price,
			current_tp2_price = EXCLUDED.current_tp2_price,
			current_tp3_price = EXCLUDED.current_tp3_price,
			remaining_quantity = EXCLUDED.remaining_quantity,
			hedge_chain_id = EXCLUDED.hedge_chain_id,
			sl_modification_count = EXCLUDED.sl_modification_count,
			tp_modification_count = EXCLUDED.tp_modification_count,
			event_count = EXCLUDED.event_count,
			last_event_seq = EXCLUDED.last_event_seq,
			updated_at = EXCLUDED.updated_at,
			closed_at = EXCLUDED.closed_at,
			close_reason = EXCLUDED.close_reason,
			close_price = COALESCE(EXCLUDED.close_price, order_chains.close_price),
			realized_pnl = EXCLUDED.realized_pnl,
			total_fees = EXCLUDED.total_fees,
			sl_binance_order_id = COALESCE(EXCLUDED.sl_binance_order_id, order_chains.sl_binance_order_id),
			sl_limit_price = COALESCE(EXCLUDED.sl_limit_price, order_chains.sl_limit_price),
			sl_fill_price = COALESCE(EXCLUDED.sl_fill_price, order_chains.sl_fill_price),
			sl_fill_time = COALESCE(EXCLUDED.sl_fill_time, order_chains.sl_fill_time),
			sl_status = COALESCE(EXCLUDED.sl_status, order_chains.sl_status),
			sl_quantity = COALESCE(EXCLUDED.sl_quantity, order_chains.sl_quantity),
			tp_binance_order_id = COALESCE(EXCLUDED.tp_binance_order_id, order_chains.tp_binance_order_id),
			tp_limit_price = COALESCE(EXCLUDED.tp_limit_price, order_chains.tp_limit_price),
			tp_fill_price = COALESCE(EXCLUDED.tp_fill_price, order_chains.tp_fill_price),
			tp_fill_time = COALESCE(EXCLUDED.tp_fill_time, order_chains.tp_fill_time),
			tp_status = COALESCE(EXCLUDED.tp_status, order_chains.tp_status),
			tp_quantity = COALESCE(EXCLUDED.tp_quantity, order_chains.tp_quantity),
			mode = COALESCE(EXCLUDED.mode, order_chains.mode),
			strategy_group = COALESCE(EXCLUDED.strategy_group, order_chains.strategy_group),
			sub_strategy = COALESCE(EXCLUDED.sub_strategy, order_chains.sub_strategy),
			timeframe = COALESCE(EXCLUDED.timeframe, order_chains.timeframe),
			entry_context = COALESCE(EXCLUDED.entry_context, order_chains.entry_context)
		RETURNING id, created_at`

	now := time.Now()
	if chain.CreatedAt.IsZero() {
		chain.CreatedAt = now
	}
	if chain.UpdatedAt.IsZero() {
		chain.UpdatedAt = now
	}

	err := db.Pool.QueryRow(ctx, query,
		chain.UserID,
		chain.ChainID,
		chain.Symbol,
		chain.Side,
		chain.ModeCode,
		chain.Status,
		chain.EntryPrice,
		chain.EntryQuantity,
		chain.EntryFilledAt,
		chain.EntryBinanceOrderID,
		chain.CurrentSLPrice,
		chain.CurrentTPPrice,
		chain.PositionOptEnabled,
		chain.CurrentTP1Price,
		chain.CurrentTP2Price,
		chain.CurrentTP3Price,
		chain.RemainingQuantity,
		chain.HedgeChainID,
		chain.IsHedge,
		chain.ParentChainID,
		chain.SLModificationCount,
		chain.TPModificationCount,
		chain.EventCount,
		chain.LastEventSeq,
		chain.CreatedAt,
		chain.UpdatedAt,
		chain.ClosedAt,
		chain.CloseReason,
		chain.ClosePrice,
		chain.RealizedPnL,
		chain.TotalFees,
		chain.SLBinanceOrderID,
		chain.SLLimitPrice,
		chain.SLFillPrice,
		chain.SLFillTime,
		chain.SLStatus,
		chain.SLQuantity,
		chain.TPBinanceOrderID,
		chain.TPLimitPrice,
		chain.TPFillPrice,
		chain.TPFillTime,
		chain.TPStatus,
		chain.TPQuantity,
		chain.Mode,
		chain.StrategyGroup,
		chain.SubStrategy,
		chain.Timeframe,
		chain.EntryContext,
	).Scan(&chain.ID, &chain.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create order chain: %w", err)
	}

	return nil
}

// UpdateOrderChain updates an existing order chain record
func (db *DB) UpdateOrderChain(ctx context.Context, chain *orders.OrderChain) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			status = $2,
			entry_price = $3,
			entry_quantity = $4,
			entry_filled_at = $5,
			entry_binance_order_id = $6,
			current_sl_price = $7,
			current_tp_price = $8,
			position_opt_enabled = $9,
			current_tp1_price = $10,
			current_tp2_price = $11,
			current_tp3_price = $12,
			remaining_quantity = $13,
			hedge_chain_id = $14,
			sl_modification_count = $15,
			tp_modification_count = $16,
			event_count = $17,
			last_event_seq = $18,
			updated_at = $19,
			closed_at = $20,
			close_reason = $21,
			close_price = $22,
			realized_pnl = $23,
			total_fees = $24,
			sl_binance_order_id = $25,
			sl_limit_price = $26,
			sl_fill_price = $27,
			sl_fill_time = $28,
			sl_status = $29,
			sl_quantity = $30,
			tp_binance_order_id = $31,
			tp_limit_price = $32,
			tp_fill_price = $33,
			tp_fill_time = $34,
			tp_status = $35,
			tp_quantity = $36
		WHERE id = $1`

	chain.UpdatedAt = time.Now()

	_, err := db.Pool.Exec(ctx, query,
		chain.ID,
		chain.Status,
		chain.EntryPrice,
		chain.EntryQuantity,
		chain.EntryFilledAt,
		chain.EntryBinanceOrderID,
		chain.CurrentSLPrice,
		chain.CurrentTPPrice,
		chain.PositionOptEnabled,
		chain.CurrentTP1Price,
		chain.CurrentTP2Price,
		chain.CurrentTP3Price,
		chain.RemainingQuantity,
		chain.HedgeChainID,
		chain.SLModificationCount,
		chain.TPModificationCount,
		chain.EventCount,
		chain.LastEventSeq,
		chain.UpdatedAt,
		chain.ClosedAt,
		chain.CloseReason,
		chain.ClosePrice,
		chain.RealizedPnL,
		chain.TotalFees,
		chain.SLBinanceOrderID,
		chain.SLLimitPrice,
		chain.SLFillPrice,
		chain.SLFillTime,
		chain.SLStatus,
		chain.SLQuantity,
		chain.TPBinanceOrderID,
		chain.TPLimitPrice,
		chain.TPFillPrice,
		chain.TPFillTime,
		chain.TPStatus,
		chain.TPQuantity,
	)

	if err != nil {
		return fmt.Errorf("failed to update order chain: %w", err)
	}

	return nil
}

// GetOrderChainByID retrieves an order chain by chain ID
func (db *DB) GetOrderChainByID(ctx context.Context, userID, chainID string) (*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains WHERE user_id = $1 AND chain_id = $2`

	chain, err := scanOrderChainRow(db.Pool.QueryRow(ctx, query, userID, chainID))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order chain: %w", err)
	}

	return chain, nil
}

// GetOrderChainByChainIDOnly retrieves an order chain by chain ID only (without userID).
// This is used by ChainEventWriter when the userID is not known at the time of update.
// NOTE: chain_id is unique, so this is safe to use.
func (db *DB) GetOrderChainByChainIDOnly(ctx context.Context, chainID string) (*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains WHERE chain_id = $1`

	chain, err := scanOrderChainRow(db.Pool.QueryRow(ctx, query, chainID))
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get order chain by chain_id: %w", err)
	}

	return chain, nil
}

// GetOrderChainsByUserID retrieves all order chains for a user, optionally filtered by status
func (db *DB) GetOrderChainsByUserID(ctx context.Context, userID string, status orders.OrderChainStatus) ([]*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	var query string
	var args []interface{}

	if status != "" {
		query = `SELECT ` + orderChainSelectColumns + ` FROM order_chains
			WHERE user_id = $1 AND status = $2
			ORDER BY created_at DESC`
		args = []interface{}{userID, status}
	} else {
		query = `SELECT ` + orderChainSelectColumns + ` FROM order_chains
			WHERE user_id = $1
			ORDER BY created_at DESC`
		args = []interface{}{userID}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get order chains: %w", err)
	}
	defer rows.Close()

	return scanOrderChains(rows)
}

// GetActiveOrderChains retrieves all active (ACTIVE or PARTIAL) order chains for a user
func (db *DB) GetActiveOrderChains(ctx context.Context, userID string) ([]*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1 AND status IN ('ACTIVE', 'PARTIAL')
		ORDER BY created_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active order chains: %w", err)
	}
	defer rows.Close()

	return scanOrderChains(rows)
}

// GetOpenOrderChains retrieves all non-closed order chains for a user.
// Includes PENDING, ENTRY_PLACED, ACTIVE, and PARTIAL statuses.
// Used for capacity counting where pending entries should also count.
func (db *DB) GetOpenOrderChains(ctx context.Context, userID string) ([]*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1 AND status IN ('PENDING', 'ENTRY_PLACED', 'ACTIVE', 'PARTIAL')
		ORDER BY created_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get open order chains: %w", err)
	}
	defer rows.Close()

	return scanOrderChains(rows)
}

// GetOrderChainsByChainIDs retrieves order chains by their chain IDs (regardless of status)
// This is used to enrich chains created from Binance orders with entry data from DB
func (db *DB) GetOrderChainsByChainIDs(ctx context.Context, userID string, chainIDs []string) (map[string]*orders.OrderChain, error) {
	result := make(map[string]*orders.OrderChain)
	if db.Pool == nil || len(chainIDs) == 0 {
		return result, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1 AND chain_id = ANY($2)`

	rows, err := db.Pool.Query(ctx, query, userID, chainIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get order chains by IDs: %w", err)
	}
	defer rows.Close()

	chains, err := scanOrderChains(rows)
	if err != nil {
		return nil, err
	}

	for _, chain := range chains {
		if chain != nil {
			result[chain.ChainID] = chain
		}
	}

	return result, nil
}

// GetUsersWithActiveChains retrieves all user IDs that have active chains
func (db *DB) GetUsersWithActiveChains(ctx context.Context) ([]string, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT DISTINCT user_id
		FROM order_chains
		WHERE status IN ('ACTIVE', 'PARTIAL')`

	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get users with active chains: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("failed to scan user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user IDs: %w", err)
	}

	return userIDs, nil
}

// GetRecentOrderChains retrieves recent order chains for a user
func (db *DB) GetRecentOrderChains(ctx context.Context, userID string, limit int) ([]*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent order chains: %w", err)
	}
	defer rows.Close()

	return scanOrderChains(rows)
}

// OrderChainFilter contains filter options for querying order chains
type OrderChainFilter struct {
	UserID   string
	Status   string // active, partial, closed, cancelled, or empty for all
	Symbol   string
	ModeCode string
	DateFrom *time.Time
	DateTo   *time.Time
	Limit    int
	Offset   int
}

// GetOrderChainsWithFilter retrieves order chains with flexible filtering
// Supports date range, status, symbol, and mode filtering
func (db *DB) GetOrderChainsWithFilter(ctx context.Context, filter OrderChainFilter) ([]*orders.OrderChain, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1`

	args := []interface{}{filter.UserID}
	argIndex := 2

	// Add status filter
	if filter.Status != "" {
		switch filter.Status {
		case "active":
			query += fmt.Sprintf(" AND status IN ('ACTIVE', 'PENDING', 'ENTRY_PLACED')")
		case "partial":
			query += fmt.Sprintf(" AND status = 'PARTIAL'")
		case "closed", "completed":
			query += fmt.Sprintf(" AND status = 'CLOSED'")
		case "cancelled":
			query += fmt.Sprintf(" AND status = 'CANCELLED'")
		default:
			query += fmt.Sprintf(" AND status = $%d", argIndex)
			args = append(args, filter.Status)
			argIndex++
		}
	}

	// Add symbol filter
	if filter.Symbol != "" {
		query += fmt.Sprintf(" AND symbol = $%d", argIndex)
		args = append(args, filter.Symbol)
		argIndex++
	}

	// Add mode filter
	if filter.ModeCode != "" {
		query += fmt.Sprintf(" AND mode_code = $%d", argIndex)
		args = append(args, filter.ModeCode)
		argIndex++
	}

	// Add date range filter (on created_at)
	if filter.DateFrom != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, *filter.DateFrom)
		argIndex++
	}
	if filter.DateTo != nil {
		// Add 1 day to DateTo to include the entire day
		dateTo := filter.DateTo.Add(24 * time.Hour)
		query += fmt.Sprintf(" AND created_at < $%d", argIndex)
		args = append(args, dateTo)
		argIndex++
	}

	// Order by created_at descending
	query += " ORDER BY created_at DESC"

	// Add limit/offset for pagination
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
		argIndex++
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get order chains with filter: %w", err)
	}
	defer rows.Close()

	return scanOrderChains(rows)
}

// UpdateOrderChainSLPrice updates the current SL price and increments modification count
func (db *DB) UpdateOrderChainSLPrice(ctx context.Context, chainID string, newPrice float64) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			current_sl_price = $2,
			sl_modification_count = sl_modification_count + 1,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, newPrice)
	if err != nil {
		return fmt.Errorf("failed to update order chain SL price: %w", err)
	}

	return nil
}

// UpdateOrderChainTPPrice updates the current TP price and increments modification count
func (db *DB) UpdateOrderChainTPPrice(ctx context.Context, chainID string, newPrice float64) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			current_tp_price = $2,
			tp_modification_count = tp_modification_count + 1,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, newPrice)
	if err != nil {
		return fmt.Errorf("failed to update order chain TP price: %w", err)
	}

	return nil
}

// CloseOrderChain marks an order chain as closed
func (db *DB) CloseOrderChain(ctx context.Context, chainID string, closeReason string, realizedPnL, totalFees float64, closePrice *float64) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			status = 'CLOSED',
			closed_at = NOW(),
			close_reason = $2,
			realized_pnl = $3,
			total_fees = $4,
			remaining_quantity = 0,
			close_price = COALESCE($5, close_price),
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, closeReason, realizedPnL, totalFees, closePrice)
	if err != nil {
		return fmt.Errorf("failed to close order chain: %w", err)
	}

	return nil
}

// ReactivateOrderChain sets a chain's status back to ACTIVE and clears close_reason and closed_at
func (db *DB) ReactivateOrderChain(ctx context.Context, chainID string) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			status = 'ACTIVE',
			close_reason = NULL,
			closed_at = NULL,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID)
	if err != nil {
		return fmt.Errorf("failed to reactivate order chain: %w", err)
	}

	return nil
}

// IncrementOrderChainEventCount increments the event count and updates last_event_seq
func (db *DB) IncrementOrderChainEventCount(ctx context.Context, chainID string, newSeq int) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			event_count = event_count + 1,
			last_event_seq = $2,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, newSeq)
	if err != nil {
		return fmt.Errorf("failed to increment order chain event count: %w", err)
	}

	return nil
}

// LinkHedgeChain links a hedge chain to a primary chain
func (db *DB) LinkHedgeChain(ctx context.Context, primaryChainID, hedgeChainID string) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			hedge_chain_id = $2,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, primaryChainID, hedgeChainID)
	if err != nil {
		return fmt.Errorf("failed to link hedge chain: %w", err)
	}

	return nil
}

// UpdateOrderChainSLDetails persists SL order details when the SL order is placed
func (db *DB) UpdateOrderChainSLDetails(ctx context.Context, chainID string, binanceOrderID int64, limitPrice float64, quantity float64) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			sl_binance_order_id = $2,
			sl_limit_price = $3,
			sl_quantity = $4,
			sl_status = 'NEW',
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, binanceOrderID, limitPrice, quantity)
	if err != nil {
		return fmt.Errorf("failed to update order chain SL details: %w", err)
	}

	return nil
}

// UpdateOrderChainTPDetails persists TP order details when the TP order is placed
func (db *DB) UpdateOrderChainTPDetails(ctx context.Context, chainID string, binanceOrderID int64, limitPrice float64, quantity float64) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			tp_binance_order_id = $2,
			tp_limit_price = $3,
			tp_quantity = $4,
			tp_status = 'NEW',
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, binanceOrderID, limitPrice, quantity)
	if err != nil {
		return fmt.Errorf("failed to update order chain TP details: %w", err)
	}

	return nil
}

// UpdateOrderChainSLFilled records SL fill details and cancels TP
func (db *DB) UpdateOrderChainSLFilled(ctx context.Context, chainID string, fillPrice float64, fillTime time.Time) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			sl_fill_price = $2,
			sl_fill_time = $3,
			sl_status = 'FILLED',
			tp_status = 'CANCELED',
			close_price = $2,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, fillPrice, fillTime)
	if err != nil {
		return fmt.Errorf("failed to update order chain SL filled: %w", err)
	}

	return nil
}

// UpdateOrderChainTPFilled records TP fill details and cancels SL
func (db *DB) UpdateOrderChainTPFilled(ctx context.Context, chainID string, fillPrice float64, fillTime time.Time) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			tp_fill_price = $2,
			tp_fill_time = $3,
			tp_status = 'FILLED',
			sl_status = 'CANCELED',
			close_price = $2,
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, fillPrice, fillTime)
	if err != nil {
		return fmt.Errorf("failed to update order chain TP filled: %w", err)
	}

	return nil
}

// FindChainByBinanceOrderID looks up a chain by SL or TP Binance order ID.
// Returns the chain, an indicator of which order matched ("sl" or "tp"), and error.
// Returns (nil, "", nil) if no match found.
func (db *DB) FindChainByBinanceOrderID(ctx context.Context, userID string, binanceOrderID int64) (*orders.OrderChain, string, error) {
	if db.Pool == nil {
		return nil, "", nil
	}

	// Try SL first
	query := `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1 AND sl_binance_order_id = $2 AND status IN ('ACTIVE', 'PARTIAL')
		LIMIT 1`

	chain, err := scanOrderChainRow(db.Pool.QueryRow(ctx, query, userID, binanceOrderID))
	if err == nil {
		return chain, "sl", nil
	}
	if err != pgx.ErrNoRows {
		return nil, "", fmt.Errorf("failed to find chain by SL order ID: %w", err)
	}

	// Try TP
	query = `SELECT ` + orderChainSelectColumns + ` FROM order_chains
		WHERE user_id = $1 AND tp_binance_order_id = $2 AND status IN ('ACTIVE', 'PARTIAL')
		LIMIT 1`

	chain, err = scanOrderChainRow(db.Pool.QueryRow(ctx, query, userID, binanceOrderID))
	if err == nil {
		return chain, "tp", nil
	}
	if err != pgx.ErrNoRows {
		return nil, "", fmt.Errorf("failed to find chain by TP order ID: %w", err)
	}

	return nil, "", nil
}

// UpdateOrderChainSLCanceled marks the SL order as CANCELED on a chain
func (db *DB) UpdateOrderChainSLCanceled(ctx context.Context, chainID string, canceledAt time.Time) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			sl_status = 'CANCELED',
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID)
	if err != nil {
		return fmt.Errorf("failed to update order chain SL canceled: %w", err)
	}

	return nil
}

// UpdateOrderChainTPCanceled marks the TP order as CANCELED on a chain
func (db *DB) UpdateOrderChainTPCanceled(ctx context.Context, chainID string, canceledAt time.Time) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		UPDATE order_chains SET
			tp_status = 'CANCELED',
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID)
	if err != nil {
		return fmt.Errorf("failed to update order chain TP canceled: %w", err)
	}

	return nil
}

// CountActiveChains returns the number of active (ACTIVE or PARTIAL) chains for a user
func (db *DB) CountActiveChains(ctx context.Context, userID string) (int, error) {
	if db.Pool == nil {
		return 0, nil
	}

	query := `SELECT COUNT(*) FROM order_chains WHERE user_id = $1 AND status IN ('PENDING', 'ENTRY_PLACED', 'ACTIVE', 'PARTIAL')`

	var count int
	err := db.Pool.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active chains: %w", err)
	}

	return count, nil
}

// UpdateClosedChainPnL updates the realized_pnl and total_fees for an already-closed chain.
// This is called when ORDER_TRADE_UPDATE arrives after ALGO_UPDATE with real trade data
// (ALGO_UPDATE has no RealizedProfit/Commission, so calculated values need correction).
func (db *DB) UpdateClosedChainPnL(ctx context.Context, chainID string, realizedPnL float64, totalFees float64) error {
	if db.Pool == nil {
		return nil
	}

	_, err := db.Pool.Exec(ctx,
		`UPDATE order_chains SET realized_pnl = $1, total_fees = $2, updated_at = NOW() WHERE chain_id = $3 AND status = 'CLOSED'`,
		realizedPnL, totalFees, chainID)
	return err
}

// orderChainSelectColumns is the standard set of columns selected for order chain queries.
// This must match the order in scanOrderChainRow.
const orderChainSelectColumns = `id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason, close_price,
			realized_pnl, total_fees,
			sl_binance_order_id, sl_limit_price, sl_fill_price, sl_fill_time, sl_status, sl_quantity,
			tp_binance_order_id, tp_limit_price, tp_fill_price, tp_fill_time, tp_status, tp_quantity,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe, entry_context`

// scanOrderChainRow scans a single row into an OrderChain. Column order must match orderChainSelectColumns.
func scanOrderChainRow(scanner interface{ Scan(dest ...interface{}) error }) (*orders.OrderChain, error) {
	chain := &orders.OrderChain{}
	err := scanner.Scan(
		&chain.ID,
		&chain.UserID,
		&chain.ChainID,
		&chain.Symbol,
		&chain.Side,
		&chain.ModeCode,
		&chain.Status,
		&chain.EntryPrice,
		&chain.EntryQuantity,
		&chain.EntryFilledAt,
		&chain.EntryBinanceOrderID,
		&chain.CurrentSLPrice,
		&chain.CurrentTPPrice,
		&chain.PositionOptEnabled,
		&chain.CurrentTP1Price,
		&chain.CurrentTP2Price,
		&chain.CurrentTP3Price,
		&chain.RemainingQuantity,
		&chain.HedgeChainID,
		&chain.IsHedge,
		&chain.ParentChainID,
		&chain.SLModificationCount,
		&chain.TPModificationCount,
		&chain.EventCount,
		&chain.LastEventSeq,
		&chain.CreatedAt,
		&chain.UpdatedAt,
		&chain.ClosedAt,
		&chain.CloseReason,
		&chain.ClosePrice,
		&chain.RealizedPnL,
		&chain.TotalFees,
		&chain.SLBinanceOrderID,
		&chain.SLLimitPrice,
		&chain.SLFillPrice,
		&chain.SLFillTime,
		&chain.SLStatus,
		&chain.SLQuantity,
		&chain.TPBinanceOrderID,
		&chain.TPLimitPrice,
		&chain.TPFillPrice,
		&chain.TPFillTime,
		&chain.TPStatus,
		&chain.TPQuantity,
		&chain.Mode,
		&chain.StrategyGroup,
		&chain.SubStrategy,
		&chain.Timeframe,
		&chain.EntryContext,
	)
	return chain, err
}

// scanOrderChains scans rows into order chain slice
func scanOrderChains(rows pgx.Rows) ([]*orders.OrderChain, error) {
	var chains []*orders.OrderChain
	for rows.Next() {
		chain, err := scanOrderChainRow(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order chain row: %w", err)
		}
		chains = append(chains, chain)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating order chain rows: %w", err)
	}

	return chains, nil
}
