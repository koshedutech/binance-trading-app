package database

import (
	"context"
	"fmt"
	"time"

	"binance-trading-bot/internal/orders"

	"github.com/jackc/pgx/v5"
)

// InsertChainEvent inserts a new chain event record
func (db *DB) InsertChainEvent(ctx context.Context, event *orders.ChainEvent) error {
	if db.Pool == nil {
		return nil
	}

	query := `
		INSERT INTO order_chain_events (
			chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9, $10,
			$11, $12,
			$13, $14,
			$15, $16, $17,
			$18, $19
		)
		RETURNING id, created_at`

	now := time.Now()
	if event.CreatedAt.IsZero() {
		event.CreatedAt = now
	}

	err := db.Pool.QueryRow(ctx, query,
		event.ChainID,
		event.EventSequence,
		event.EventType,
		event.OrderType,
		event.BinanceOrderID,
		event.BinanceClientOrderID,
		event.Price,
		event.OldPrice,
		event.Quantity,
		event.FilledQuantity,
		event.OrderStatus,
		event.OrderSide,
		event.ModificationSource,
		event.ModificationReason,
		event.MarketPrice,
		event.PnL,
		event.Fees,
		event.BinanceTimestamp,
		event.CreatedAt,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert chain event: %w", err)
	}

	return nil
}

// GetChainEvents retrieves all events for a chain in sequence order
func (db *DB) GetChainEvents(ctx context.Context, chainID string) ([]*orders.ChainEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		FROM order_chain_events
		WHERE chain_id = $1
		ORDER BY event_sequence ASC`

	rows, err := db.Pool.Query(ctx, query, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain events: %w", err)
	}
	defer rows.Close()

	return scanChainEvents(rows)
}

// GetChainEventsByType retrieves events of a specific type for a chain
func (db *DB) GetChainEventsByType(ctx context.Context, chainID string, eventType orders.EventType) ([]*orders.ChainEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		FROM order_chain_events
		WHERE chain_id = $1 AND event_type = $2
		ORDER BY event_sequence ASC`

	rows, err := db.Pool.Query(ctx, query, chainID, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain events by type: %w", err)
	}
	defer rows.Close()

	return scanChainEvents(rows)
}

// GetChainEventsByOrderType retrieves events for a specific order type (SL, TP, etc.)
func (db *DB) GetChainEventsByOrderType(ctx context.Context, chainID string, orderType string) ([]*orders.ChainEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		FROM order_chain_events
		WHERE chain_id = $1 AND order_type = $2
		ORDER BY event_sequence ASC`

	rows, err := db.Pool.Query(ctx, query, chainID, orderType)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain events by order type: %w", err)
	}
	defer rows.Close()

	return scanChainEvents(rows)
}

// GetNextEventSequence returns the next event sequence number for a chain
func (db *DB) GetNextEventSequence(ctx context.Context, chainID string) (int, error) {
	if db.Pool == nil {
		return 1, nil
	}

	query := `
		SELECT COALESCE(MAX(event_sequence), 0) + 1
		FROM order_chain_events
		WHERE chain_id = $1`

	var nextSeq int
	err := db.Pool.QueryRow(ctx, query, chainID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to get next event sequence: %w", err)
	}

	return nextSeq, nil
}

// GetLatestChainEvent retrieves the most recent event for a chain
func (db *DB) GetLatestChainEvent(ctx context.Context, chainID string) (*orders.ChainEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		FROM order_chain_events
		WHERE chain_id = $1
		ORDER BY event_sequence DESC
		LIMIT 1`

	event := &orders.ChainEvent{}
	err := db.Pool.QueryRow(ctx, query, chainID).Scan(
		&event.ID,
		&event.ChainID,
		&event.EventSequence,
		&event.EventType,
		&event.OrderType,
		&event.BinanceOrderID,
		&event.BinanceClientOrderID,
		&event.Price,
		&event.OldPrice,
		&event.Quantity,
		&event.FilledQuantity,
		&event.OrderStatus,
		&event.OrderSide,
		&event.ModificationSource,
		&event.ModificationReason,
		&event.MarketPrice,
		&event.PnL,
		&event.Fees,
		&event.BinanceTimestamp,
		&event.CreatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest chain event: %w", err)
	}

	return event, nil
}

// GetRecentEventsForUser retrieves recent events across all chains for a user
func (db *DB) GetRecentEventsForUser(ctx context.Context, userID string, limit int) ([]*orders.ChainEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT e.id, e.chain_id, e.event_sequence, e.event_type, e.order_type,
			e.binance_order_id, e.binance_client_order_id,
			e.price, e.old_price, e.quantity, e.filled_quantity,
			e.order_status, e.order_side,
			e.modification_source, e.modification_reason,
			e.market_price, e.pnl, e.fees,
			e.binance_timestamp, e.created_at
		FROM order_chain_events e
		INNER JOIN order_chains c ON e.chain_id = c.chain_id
		WHERE c.user_id = $1
		ORDER BY e.created_at DESC
		LIMIT $2`

	rows, err := db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent events for user: %w", err)
	}
	defer rows.Close()

	return scanChainEvents(rows)
}

// GetSLModificationEvents retrieves all SL_MODIFIED events for a chain
func (db *DB) GetSLModificationEvents(ctx context.Context, chainID string) ([]*orders.ChainEvent, error) {
	return db.GetChainEventsByType(ctx, chainID, orders.EventSLModified)
}

// GetTPModificationEvents retrieves all TP_MODIFIED events for a chain
func (db *DB) GetTPModificationEvents(ctx context.Context, chainID string) ([]*orders.ChainEvent, error) {
	return db.GetChainEventsByType(ctx, chainID, orders.EventTPModified)
}

// GetOrderChainWithEvents retrieves a chain with all its events
func (db *DB) GetOrderChainWithEvents(ctx context.Context, userID, chainID string) (*orders.OrderChainWithEvents, error) {
	if db.Pool == nil {
		return nil, nil
	}

	chain, err := db.GetOrderChainByID(ctx, userID, chainID)
	if err != nil {
		return nil, err
	}
	if chain == nil {
		return nil, nil
	}

	events, err := db.GetChainEvents(ctx, chainID)
	if err != nil {
		return nil, err
	}

	return &orders.OrderChainWithEvents{
		Chain:  chain,
		Events: events,
	}, nil
}

// CountEventsByType counts events of each type for a chain
func (db *DB) CountEventsByType(ctx context.Context, chainID string) (map[orders.EventType]int, error) {
	if db.Pool == nil {
		return make(map[orders.EventType]int), nil
	}

	query := `
		SELECT event_type, COUNT(*)
		FROM order_chain_events
		WHERE chain_id = $1
		GROUP BY event_type`

	rows, err := db.Pool.Query(ctx, query, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to count events by type: %w", err)
	}
	defer rows.Close()

	counts := make(map[orders.EventType]int)
	for rows.Next() {
		var eventType orders.EventType
		var count int
		if err := rows.Scan(&eventType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan event count: %w", err)
		}
		counts[eventType] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating event counts: %w", err)
	}

	return counts, nil
}

// GetEventsForChains retrieves events for multiple chains (batch query for tree view)
func (db *DB) GetEventsForChains(ctx context.Context, chainIDs []string) (map[string][]*orders.ChainEvent, error) {
	if db.Pool == nil || len(chainIDs) == 0 {
		return make(map[string][]*orders.ChainEvent), nil
	}

	query := `
		SELECT id, chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		FROM order_chain_events
		WHERE chain_id = ANY($1)
		ORDER BY chain_id, event_sequence ASC`

	rows, err := db.Pool.Query(ctx, query, chainIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get events for chains: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]*orders.ChainEvent)
	for rows.Next() {
		event := &orders.ChainEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ChainID,
			&event.EventSequence,
			&event.EventType,
			&event.OrderType,
			&event.BinanceOrderID,
			&event.BinanceClientOrderID,
			&event.Price,
			&event.OldPrice,
			&event.Quantity,
			&event.FilledQuantity,
			&event.OrderStatus,
			&event.OrderSide,
			&event.ModificationSource,
			&event.ModificationReason,
			&event.MarketPrice,
			&event.PnL,
			&event.Fees,
			&event.BinanceTimestamp,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chain event row: %w", err)
		}
		result[event.ChainID] = append(result[event.ChainID], event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chain events: %w", err)
	}

	return result, nil
}

// GetSLModificationEventsByChainIDs retrieves SL_MODIFIED events for multiple chains in one query.
// Also includes the initial SL_PLACED event for complete history display.
// Returns map[chainID] -> []*ChainEvent sorted by event_sequence ASC.
func (db *DB) GetSLModificationEventsByChainIDs(ctx context.Context, chainIDs []string) (map[string][]*orders.ChainEvent, error) {
	if db.Pool == nil || len(chainIDs) == 0 {
		return make(map[string][]*orders.ChainEvent), nil
	}

	query := `
		SELECT id, chain_id, event_sequence, event_type, order_type,
			binance_order_id, binance_client_order_id,
			price, old_price, quantity, filled_quantity,
			order_status, order_side,
			modification_source, modification_reason,
			market_price, pnl, fees,
			binance_timestamp, created_at
		FROM order_chain_events
		WHERE chain_id = ANY($1) AND event_type IN ('SL_PLACED', 'SL_MODIFIED')
		ORDER BY chain_id, event_sequence ASC`

	rows, err := db.Pool.Query(ctx, query, chainIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get SL modification events for chains: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]*orders.ChainEvent)
	for rows.Next() {
		event := &orders.ChainEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ChainID,
			&event.EventSequence,
			&event.EventType,
			&event.OrderType,
			&event.BinanceOrderID,
			&event.BinanceClientOrderID,
			&event.Price,
			&event.OldPrice,
			&event.Quantity,
			&event.FilledQuantity,
			&event.OrderStatus,
			&event.OrderSide,
			&event.ModificationSource,
			&event.ModificationReason,
			&event.MarketPrice,
			&event.PnL,
			&event.Fees,
			&event.BinanceTimestamp,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan SL modification event row: %w", err)
		}
		result[event.ChainID] = append(result[event.ChainID], event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating SL modification event rows: %w", err)
	}

	return result, nil
}

// scanChainEvents scans rows into chain event slice
func scanChainEvents(rows pgx.Rows) ([]*orders.ChainEvent, error) {
	var events []*orders.ChainEvent
	for rows.Next() {
		event := &orders.ChainEvent{}
		err := rows.Scan(
			&event.ID,
			&event.ChainID,
			&event.EventSequence,
			&event.EventType,
			&event.OrderType,
			&event.BinanceOrderID,
			&event.BinanceClientOrderID,
			&event.Price,
			&event.OldPrice,
			&event.Quantity,
			&event.FilledQuantity,
			&event.OrderStatus,
			&event.OrderSide,
			&event.ModificationSource,
			&event.ModificationReason,
			&event.MarketPrice,
			&event.PnL,
			&event.Fees,
			&event.BinanceTimestamp,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chain event row: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chain event rows: %w", err)
	}

	return events, nil
}
