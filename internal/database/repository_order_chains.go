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
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12,
			$13, $14, $15, $16,
			$17,
			$18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, $27, $28,
			$29, $30,
			$31, $32, $33, $34
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
			realized_pnl = EXCLUDED.realized_pnl,
			total_fees = EXCLUDED.total_fees,
			mode = COALESCE(EXCLUDED.mode, order_chains.mode),
			strategy_group = COALESCE(EXCLUDED.strategy_group, order_chains.strategy_group),
			sub_strategy = COALESCE(EXCLUDED.sub_strategy, order_chains.sub_strategy),
			timeframe = COALESCE(EXCLUDED.timeframe, order_chains.timeframe)
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
		chain.RealizedPnL,
		chain.TotalFees,
		chain.Mode,
		chain.StrategyGroup,
		chain.SubStrategy,
		chain.Timeframe,
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
			realized_pnl = $22,
			total_fees = $23
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
		chain.RealizedPnL,
		chain.TotalFees,
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

	query := `
		SELECT id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		FROM order_chains
		WHERE user_id = $1 AND chain_id = $2`

	chain := &orders.OrderChain{}
	err := db.Pool.QueryRow(ctx, query, userID, chainID).Scan(
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
		&chain.RealizedPnL,
		&chain.TotalFees,
		&chain.Mode,
		&chain.StrategyGroup,
		&chain.SubStrategy,
		&chain.Timeframe,
	)

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

	query := `
		SELECT id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		FROM order_chains
		WHERE chain_id = $1`

	chain := &orders.OrderChain{}
	err := db.Pool.QueryRow(ctx, query, chainID).Scan(
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
		&chain.RealizedPnL,
		&chain.TotalFees,
		&chain.Mode,
		&chain.StrategyGroup,
		&chain.SubStrategy,
		&chain.Timeframe,
	)

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
		query = `
			SELECT id, user_id, chain_id, symbol, side, mode_code, status,
				entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
				current_sl_price, current_tp_price,
				position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
				remaining_quantity,
				hedge_chain_id, is_hedge, parent_chain_id,
				sl_modification_count, tp_modification_count, event_count, last_event_seq,
				created_at, updated_at, closed_at, close_reason,
				realized_pnl, total_fees,
				COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
			FROM order_chains
			WHERE user_id = $1 AND status = $2
			ORDER BY created_at DESC`
		args = []interface{}{userID, status}
	} else {
		query = `
			SELECT id, user_id, chain_id, symbol, side, mode_code, status,
				entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
				current_sl_price, current_tp_price,
				position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
				remaining_quantity,
				hedge_chain_id, is_hedge, parent_chain_id,
				sl_modification_count, tp_modification_count, event_count, last_event_seq,
				created_at, updated_at, closed_at, close_reason,
				realized_pnl, total_fees,
				COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
			FROM order_chains
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

	query := `
		SELECT id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		FROM order_chains
		WHERE user_id = $1 AND status IN ('ACTIVE', 'PARTIAL')
		ORDER BY created_at DESC`

	rows, err := db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active order chains: %w", err)
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

	query := `
		SELECT id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		FROM order_chains
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

	query := `
		SELECT id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		FROM order_chains
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

	query := `
		SELECT id, user_id, chain_id, symbol, side, mode_code, status,
			entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
			current_sl_price, current_tp_price,
			position_opt_enabled, current_tp1_price, current_tp2_price, current_tp3_price,
			remaining_quantity,
			hedge_chain_id, is_hedge, parent_chain_id,
			sl_modification_count, tp_modification_count, event_count, last_event_seq,
			created_at, updated_at, closed_at, close_reason,
			realized_pnl, total_fees,
			COALESCE(mode, '') as mode, COALESCE(strategy_group, '') as strategy_group, COALESCE(sub_strategy, '') as sub_strategy, COALESCE(timeframe, '') as timeframe
		FROM order_chains
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
func (db *DB) CloseOrderChain(ctx context.Context, chainID string, closeReason string, realizedPnL, totalFees float64) error {
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
			updated_at = NOW()
		WHERE chain_id = $1`

	_, err := db.Pool.Exec(ctx, query, chainID, closeReason, realizedPnL, totalFees)
	if err != nil {
		return fmt.Errorf("failed to close order chain: %w", err)
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

// scanOrderChains scans rows into order chain slice
func scanOrderChains(rows pgx.Rows) ([]*orders.OrderChain, error) {
	var chains []*orders.OrderChain
	for rows.Next() {
		chain := &orders.OrderChain{}
		err := rows.Scan(
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
			&chain.RealizedPnL,
			&chain.TotalFees,
			&chain.Mode,
			&chain.StrategyGroup,
			&chain.SubStrategy,
			&chain.Timeframe,
		)
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
