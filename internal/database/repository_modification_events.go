package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"binance-trading-bot/internal/orders"

	"github.com/jackc/pgx/v5"
)

// CreateModificationEvent inserts a new order modification event
func (db *DB) CreateModificationEvent(ctx context.Context, event *orders.OrderModificationEvent) error {
	if db.Pool == nil {
		return nil // No database configured
	}

	// Marshal market context to JSON
	var marketContextJSON []byte
	var err error
	if event.MarketContext != nil {
		marketContextJSON, err = json.Marshal(event.MarketContext)
		if err != nil {
			return fmt.Errorf("failed to marshal market context: %w", err)
		}
	}

	query := `
		INSERT INTO order_modification_events (
			user_id, chain_id, order_type, binance_order_id,
			event_type, modification_source, version,
			old_price, new_price, price_delta, price_delta_percent,
			position_quantity, position_entry_price,
			dollar_impact, impact_direction,
			modification_reason, llm_decision_id, llm_confidence,
			market_context, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		)
		RETURNING id, created_at`

	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}

	err = db.Pool.QueryRow(ctx, query,
		event.UserID,
		event.ChainID,
		event.OrderType,
		event.BinanceOrderID,
		event.EventType,
		event.ModificationSource,
		event.Version,
		event.OldPrice,
		event.NewPrice,
		event.PriceDelta,
		event.PriceDeltaPercent,
		event.PositionQuantity,
		event.PositionEntryPrice,
		event.DollarImpact,
		event.ImpactDirection,
		event.ModificationReason,
		nilIfEmpty(event.LLMDecisionID),
		event.LLMConfidence,
		marketContextJSON,
		event.CreatedAt,
	).Scan(&event.ID, &event.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create modification event: %w", err)
	}

	return nil
}

// GetModificationEvents retrieves all modification events for a chain and order type
func (db *DB) GetModificationEvents(ctx context.Context, chainID, orderType string) ([]*orders.OrderModificationEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, chain_id, order_type, binance_order_id,
			event_type, modification_source, version,
			old_price, new_price, price_delta, price_delta_percent,
			position_quantity, position_entry_price,
			dollar_impact, impact_direction,
			modification_reason, llm_decision_id, llm_confidence,
			market_context, created_at
		FROM order_modification_events
		WHERE chain_id = $1 AND order_type = $2
		ORDER BY version ASC`

	rows, err := db.Pool.Query(ctx, query, chainID, orderType)
	if err != nil {
		return nil, fmt.Errorf("failed to get modification events: %w", err)
	}
	defer rows.Close()

	return scanModificationEvents(rows)
}

// GetModificationEventsByChain retrieves all modification events for a chain (all order types)
func (db *DB) GetModificationEventsByChain(ctx context.Context, chainID string) ([]*orders.OrderModificationEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, user_id, chain_id, order_type, binance_order_id,
			event_type, modification_source, version,
			old_price, new_price, price_delta, price_delta_percent,
			position_quantity, position_entry_price,
			dollar_impact, impact_direction,
			modification_reason, llm_decision_id, llm_confidence,
			market_context, created_at
		FROM order_modification_events
		WHERE chain_id = $1
		ORDER BY order_type, version ASC`

	rows, err := db.Pool.Query(ctx, query, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get modification events by chain: %w", err)
	}
	defer rows.Close()

	return scanModificationEvents(rows)
}

// GetLatestModificationVersion returns the latest version number for an order
func (db *DB) GetLatestModificationVersion(ctx context.Context, chainID, orderType string) (int, error) {
	if db.Pool == nil {
		return 0, nil
	}

	query := `
		SELECT COALESCE(MAX(version), 0)
		FROM order_modification_events
		WHERE chain_id = $1 AND order_type = $2`

	var version int
	err := db.Pool.QueryRow(ctx, query, chainID, orderType).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest modification version: %w", err)
	}

	return version, nil
}

// GetModificationEventsByUser retrieves recent modification events for a user
func (db *DB) GetModificationEventsByUser(ctx context.Context, userID int64, limit int) ([]*orders.OrderModificationEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT id, user_id, chain_id, order_type, binance_order_id,
			event_type, modification_source, version,
			old_price, new_price, price_delta, price_delta_percent,
			position_quantity, position_entry_price,
			dollar_impact, impact_direction,
			modification_reason, llm_decision_id, llm_confidence,
			market_context, created_at
		FROM order_modification_events
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := db.Pool.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get modification events by user: %w", err)
	}
	defer rows.Close()

	return scanModificationEvents(rows)
}

// GetModificationEventsBySource retrieves modification events filtered by source
func (db *DB) GetModificationEventsBySource(ctx context.Context, userID int64, source string, limit int) ([]*orders.OrderModificationEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT id, user_id, chain_id, order_type, binance_order_id,
			event_type, modification_source, version,
			old_price, new_price, price_delta, price_delta_percent,
			position_quantity, position_entry_price,
			dollar_impact, impact_direction,
			modification_reason, llm_decision_id, llm_confidence,
			market_context, created_at
		FROM order_modification_events
		WHERE user_id = $1 AND modification_source = $2
		ORDER BY created_at DESC
		LIMIT $3`

	rows, err := db.Pool.Query(ctx, query, userID, source, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get modification events by source: %w", err)
	}
	defer rows.Close()

	return scanModificationEvents(rows)
}

// GetModificationSummaryByChain returns summary statistics for all orders in a chain
func (db *DB) GetModificationSummaryByChain(ctx context.Context, chainID string) (map[string]*orders.ModificationSummary, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT order_type,
			COUNT(*) - 1 as total_modifications,
			SUM(COALESCE(price_delta, 0)) as net_price_change,
			SUM(COALESCE(dollar_impact, 0)) as net_dollar_impact,
			MIN(CASE WHEN version = 1 THEN new_price END) as initial_price,
			MAX(CASE WHEN version = (SELECT MAX(version) FROM order_modification_events ome2
				WHERE ome2.chain_id = order_modification_events.chain_id
				AND ome2.order_type = order_modification_events.order_type) THEN new_price END) as current_price,
			MAX(created_at) as last_modified_at
		FROM order_modification_events
		WHERE chain_id = $1
		GROUP BY order_type`

	rows, err := db.Pool.Query(ctx, query, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get modification summary: %w", err)
	}
	defer rows.Close()

	summaries := make(map[string]*orders.ModificationSummary)
	for rows.Next() {
		var orderType string
		summary := &orders.ModificationSummary{}
		err := rows.Scan(
			&orderType,
			&summary.TotalModifications,
			&summary.NetPriceChange,
			&summary.NetDollarImpact,
			&summary.InitialPrice,
			&summary.CurrentPrice,
			&summary.LastModifiedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan modification summary: %w", err)
		}
		summaries[orderType] = summary
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating modification summary rows: %w", err)
	}

	return summaries, nil
}

// GetModificationCountsByChainIDs retrieves modification counts per order type for multiple chains (batch query)
// Story 7.14: Order Chain Backend Integration
// Returns: map[chainID]map[orderType]count (e.g., {"ULT-17JAN-00001": {"SL": 3, "TP1": 2}})
// Security: Requires userID to ensure users can only see their own modification counts
func (db *DB) GetModificationCountsByChainIDs(ctx context.Context, userID int64, chainIDs []string) (map[string]map[string]int, error) {
	result := make(map[string]map[string]int)
	if db.Pool == nil || len(chainIDs) == 0 {
		return result, nil
	}

	query := `
		SELECT chain_id, order_type, COUNT(*) - 1 as modification_count
		FROM order_modification_events
		WHERE user_id = $1 AND chain_id = ANY($2)
		GROUP BY chain_id, order_type
		HAVING COUNT(*) > 1`

	rows, err := db.Pool.Query(ctx, query, userID, chainIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get modification counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var chainID, orderType string
		var count int
		if err := rows.Scan(&chainID, &orderType, &count); err != nil {
			return nil, fmt.Errorf("failed to scan modification count row: %w", err)
		}

		if result[chainID] == nil {
			result[chainID] = make(map[string]int)
		}
		result[chainID][orderType] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating modification count rows: %w", err)
	}

	return result, nil
}

// DeleteModificationEventsByChain deletes all events for a chain (for testing/cleanup)
func (db *DB) DeleteModificationEventsByChain(ctx context.Context, chainID string) error {
	if db.Pool == nil {
		return nil
	}

	query := `DELETE FROM order_modification_events WHERE chain_id = $1`
	_, err := db.Pool.Exec(ctx, query, chainID)
	if err != nil {
		return fmt.Errorf("failed to delete modification events: %w", err)
	}

	return nil
}

// scanModificationEvents scans rows into OrderModificationEvent slice
func scanModificationEvents(rows pgx.Rows) ([]*orders.OrderModificationEvent, error) {
	var events []*orders.OrderModificationEvent
	for rows.Next() {
		event := &orders.OrderModificationEvent{}
		var marketContextJSON []byte
		var llmDecisionID *string

		err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.ChainID,
			&event.OrderType,
			&event.BinanceOrderID,
			&event.EventType,
			&event.ModificationSource,
			&event.Version,
			&event.OldPrice,
			&event.NewPrice,
			&event.PriceDelta,
			&event.PriceDeltaPercent,
			&event.PositionQuantity,
			&event.PositionEntryPrice,
			&event.DollarImpact,
			&event.ImpactDirection,
			&event.ModificationReason,
			&llmDecisionID,
			&event.LLMConfidence,
			&marketContextJSON,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan modification event row: %w", err)
		}

		// Set LLM decision ID
		if llmDecisionID != nil {
			event.LLMDecisionID = *llmDecisionID
		}

		// Parse market context JSON
		if len(marketContextJSON) > 0 {
			var ctx map[string]interface{}
			if err := json.Unmarshal(marketContextJSON, &ctx); err == nil {
				event.MarketContext = ctx
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating modification event rows: %w", err)
	}

	return events, nil
}

// nilIfEmpty returns nil if string is empty, otherwise returns pointer to string
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ModificationEventDBAdapter adapts the DB type to implement ModificationEventRepository interface
type ModificationEventDBAdapter struct {
	db *DB
}

// NewModificationEventDBAdapter creates a new adapter
func NewModificationEventDBAdapter(db *DB) *ModificationEventDBAdapter {
	return &ModificationEventDBAdapter{db: db}
}

// CreateModificationEvent implements ModificationEventRepository
func (a *ModificationEventDBAdapter) CreateModificationEvent(ctx context.Context, event *orders.OrderModificationEvent) error {
	return a.db.CreateModificationEvent(ctx, event)
}

// GetModificationEvents implements ModificationEventRepository
func (a *ModificationEventDBAdapter) GetModificationEvents(ctx context.Context, chainID, orderType string) ([]*orders.OrderModificationEvent, error) {
	return a.db.GetModificationEvents(ctx, chainID, orderType)
}

// GetLatestModificationVersion implements ModificationEventRepository
func (a *ModificationEventDBAdapter) GetLatestModificationVersion(ctx context.Context, chainID, orderType string) (int, error) {
	return a.db.GetLatestModificationVersion(ctx, chainID, orderType)
}

// GetModificationEventsByUser implements ModificationEventRepository
func (a *ModificationEventDBAdapter) GetModificationEventsByUser(ctx context.Context, userID int64, limit int) ([]*orders.OrderModificationEvent, error) {
	return a.db.GetModificationEventsByUser(ctx, userID, limit)
}

// GetModificationEventsBySource implements ModificationEventRepository
func (a *ModificationEventDBAdapter) GetModificationEventsBySource(ctx context.Context, userID int64, source string, limit int) ([]*orders.OrderModificationEvent, error) {
	return a.db.GetModificationEventsBySource(ctx, userID, source, limit)
}
