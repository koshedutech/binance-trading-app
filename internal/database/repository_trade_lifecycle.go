package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CreateTradeLifecycleEvent inserts a new trade lifecycle event
func (db *DB) CreateTradeLifecycleEvent(ctx context.Context, event *TradeLifecycleEvent) error {
	if db.Pool == nil {
		return nil // No database configured
	}

	conditionsJSON, err := json.Marshal(event.ConditionsMet)
	if err != nil {
		conditionsJSON = []byte("{}")
	}

	detailsJSON, err := json.Marshal(event.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	query := `
		INSERT INTO trade_lifecycle_events (
			futures_trade_id, user_id, event_type, event_subtype, timestamp,
			trigger_price, old_value, new_value, mode, source,
			tp_level, quantity_closed, pnl_realized, pnl_percent,
			sl_revision_count, conditions_met, reason, details, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17, $18, $19
		) RETURNING id`

	now := time.Now()
	if event.Timestamp.IsZero() {
		event.Timestamp = now
	}

	err = db.Pool.QueryRow(ctx, query,
		event.FuturesTradeID,
		event.UserID,
		event.EventType,
		event.EventSubtype,
		event.Timestamp,
		event.TriggerPrice,
		event.OldValue,
		event.NewValue,
		event.Mode,
		event.Source,
		event.TPLevel,
		event.QuantityClosed,
		event.PnLRealized,
		event.PnLPercent,
		event.SLRevisionCount,
		conditionsJSON,
		event.Reason,
		detailsJSON,
		now,
	).Scan(&event.ID)

	if err != nil {
		return fmt.Errorf("failed to create trade lifecycle event: %w", err)
	}

	event.CreatedAt = now
	return nil
}

// GetTradeLifecycleEvents retrieves all events for a specific trade
func (db *DB) GetTradeLifecycleEvents(ctx context.Context, futuresTradeID int64) ([]TradeLifecycleEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, futures_trade_id, user_id, event_type, event_subtype, timestamp,
			trigger_price, old_value, new_value, mode, source,
			tp_level, quantity_closed, pnl_realized, pnl_percent,
			sl_revision_count, conditions_met, reason, details, created_at
		FROM trade_lifecycle_events
		WHERE futures_trade_id = $1
		ORDER BY timestamp ASC`

	rows, err := db.Pool.Query(ctx, query, futuresTradeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade lifecycle events: %w", err)
	}
	defer rows.Close()

	var events []TradeLifecycleEvent
	for rows.Next() {
		var event TradeLifecycleEvent
		var conditionsJSON, detailsJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.FuturesTradeID,
			&event.UserID,
			&event.EventType,
			&event.EventSubtype,
			&event.Timestamp,
			&event.TriggerPrice,
			&event.OldValue,
			&event.NewValue,
			&event.Mode,
			&event.Source,
			&event.TPLevel,
			&event.QuantityClosed,
			&event.PnLRealized,
			&event.PnLPercent,
			&event.SLRevisionCount,
			&conditionsJSON,
			&event.Reason,
			&detailsJSON,
			&event.CreatedAt,
		)
		if err != nil {
			continue
		}

		if len(conditionsJSON) > 0 {
			json.Unmarshal(conditionsJSON, &event.ConditionsMet)
		}
		if len(detailsJSON) > 0 {
			json.Unmarshal(detailsJSON, &event.Details)
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetTradeLifecycleEventsByType retrieves events of a specific type
func (db *DB) GetTradeLifecycleEventsByType(ctx context.Context, futuresTradeID int64, eventType string) ([]TradeLifecycleEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, futures_trade_id, user_id, event_type, event_subtype, timestamp,
			trigger_price, old_value, new_value, mode, source,
			tp_level, quantity_closed, pnl_realized, pnl_percent,
			sl_revision_count, conditions_met, reason, details, created_at
		FROM trade_lifecycle_events
		WHERE futures_trade_id = $1 AND event_type = $2
		ORDER BY timestamp ASC`

	rows, err := db.Pool.Query(ctx, query, futuresTradeID, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to get trade lifecycle events by type: %w", err)
	}
	defer rows.Close()

	var events []TradeLifecycleEvent
	for rows.Next() {
		var event TradeLifecycleEvent
		var conditionsJSON, detailsJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.FuturesTradeID,
			&event.UserID,
			&event.EventType,
			&event.EventSubtype,
			&event.Timestamp,
			&event.TriggerPrice,
			&event.OldValue,
			&event.NewValue,
			&event.Mode,
			&event.Source,
			&event.TPLevel,
			&event.QuantityClosed,
			&event.PnLRealized,
			&event.PnLPercent,
			&event.SLRevisionCount,
			&conditionsJSON,
			&event.Reason,
			&detailsJSON,
			&event.CreatedAt,
		)
		if err != nil {
			continue
		}

		if len(conditionsJSON) > 0 {
			json.Unmarshal(conditionsJSON, &event.ConditionsMet)
		}
		if len(detailsJSON) > 0 {
			json.Unmarshal(detailsJSON, &event.Details)
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetTradeLifecycleSummary returns an aggregated summary of a trade's lifecycle
func (db *DB) GetTradeLifecycleSummary(ctx context.Context, futuresTradeID int64) (*TradeLifecycleEventSummary, error) {
	if db.Pool == nil {
		return nil, nil
	}

	// Get all events for this trade
	events, err := db.GetTradeLifecycleEvents(ctx, futuresTradeID)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	summary := &TradeLifecycleEventSummary{
		FuturesTradeID: futuresTradeID,
		TotalEvents:    len(events),
	}

	for _, event := range events {
		switch event.EventType {
		case EventTypePositionOpened:
			summary.StartTime = event.Timestamp
		case EventTypeSLRevised:
			summary.SLRevisions++
		case EventTypeTPHit:
			summary.TPLevelsHit++
		case EventTypeMovedToBreakeven:
			summary.MovedToBreakeven = true
		case EventTypeTrailingActivated:
			summary.TrailingActivated = true
		case EventTypePositionClosed, EventTypeExternalClose:
			summary.EndTime = &event.Timestamp
			if event.Reason != nil {
				summary.CloseReason = *event.Reason
			}
			summary.CloseSource = event.Source
		}
	}

	// Calculate duration if trade is closed
	if summary.EndTime != nil {
		duration := int64(summary.EndTime.Sub(summary.StartTime).Seconds())
		summary.Duration = &duration
	}

	return summary, nil
}

// CountSLRevisions returns the number of SL revisions for a trade
func (db *DB) CountSLRevisions(ctx context.Context, futuresTradeID int64) (int, error) {
	if db.Pool == nil {
		return 0, nil
	}

	query := `
		SELECT COUNT(*) FROM trade_lifecycle_events
		WHERE futures_trade_id = $1 AND event_type = $2`

	var count int
	err := db.Pool.QueryRow(ctx, query, futuresTradeID, EventTypeSLRevised).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count SL revisions: %w", err)
	}

	return count, nil
}

// GetRecentTradeLifecycleEvents retrieves recent events across all trades
func (db *DB) GetRecentTradeLifecycleEvents(ctx context.Context, limit int) ([]TradeLifecycleEvent, error) {
	if db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT id, futures_trade_id, user_id, event_type, event_subtype, timestamp,
			trigger_price, old_value, new_value, mode, source,
			tp_level, quantity_closed, pnl_realized, pnl_percent,
			sl_revision_count, conditions_met, reason, details, created_at
		FROM trade_lifecycle_events
		ORDER BY timestamp DESC
		LIMIT $1`

	rows, err := db.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent trade lifecycle events: %w", err)
	}
	defer rows.Close()

	var events []TradeLifecycleEvent
	for rows.Next() {
		var event TradeLifecycleEvent
		var conditionsJSON, detailsJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.FuturesTradeID,
			&event.UserID,
			&event.EventType,
			&event.EventSubtype,
			&event.Timestamp,
			&event.TriggerPrice,
			&event.OldValue,
			&event.NewValue,
			&event.Mode,
			&event.Source,
			&event.TPLevel,
			&event.QuantityClosed,
			&event.PnLRealized,
			&event.PnLPercent,
			&event.SLRevisionCount,
			&conditionsJSON,
			&event.Reason,
			&detailsJSON,
			&event.CreatedAt,
		)
		if err != nil {
			continue
		}

		if len(conditionsJSON) > 0 {
			json.Unmarshal(conditionsJSON, &event.ConditionsMet)
		}
		if len(detailsJSON) > 0 {
			json.Unmarshal(detailsJSON, &event.Details)
		}

		events = append(events, event)
	}

	return events, rows.Err()
}
