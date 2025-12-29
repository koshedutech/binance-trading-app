package database

import (
	"time"
)

// TradeLifecycleEvent represents a single event in a trade's lifecycle
// This includes position opening, SL/TP placements, revisions, TP hits, trailing stops, and closures
type TradeLifecycleEvent struct {
	ID             int64                  `json:"id"`
	FuturesTradeID *int64                 `json:"futures_trade_id,omitempty"`
	UserID         *string                `json:"user_id,omitempty"`

	// Event identification
	EventType    string  `json:"event_type"`    // position_opened, sltp_placed, sl_revised, tp_hit, etc.
	EventSubtype *string `json:"event_subtype,omitempty"` // e.g., tp1_hit, moved_to_breakeven

	// Timing
	Timestamp time.Time `json:"timestamp"`

	// Price data
	TriggerPrice *float64 `json:"trigger_price,omitempty"` // Price that triggered the event
	OldValue     *float64 `json:"old_value,omitempty"`     // Previous SL/TP value
	NewValue     *float64 `json:"new_value,omitempty"`     // New SL/TP value

	// Context
	Mode   *string `json:"mode,omitempty"`   // scalp, swing, position, ultra_fast
	Source string  `json:"source"`           // ginie, manual, binance, trailing

	// For TP events
	TPLevel        *int     `json:"tp_level,omitempty"`
	QuantityClosed *float64 `json:"quantity_closed,omitempty"`
	PnLRealized    *float64 `json:"pnl_realized,omitempty"`
	PnLPercent     *float64 `json:"pnl_percent,omitempty"`

	// For SL events
	SLRevisionCount *int `json:"sl_revision_count,omitempty"`

	// Conditions that triggered the event (signals, indicators, etc.)
	ConditionsMet map[string]interface{} `json:"conditions_met,omitempty"`

	// Additional context
	Reason  *string                `json:"reason,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
}

// Event type constants
const (
	EventTypePositionOpened    = "position_opened"
	EventTypeSLTPPlaced        = "sltp_placed"
	EventTypeSLRevised         = "sl_revised"
	EventTypeTPRevised         = "tp_revised"
	EventTypeTPHit             = "tp_hit"
	EventTypeSLHit             = "sl_hit"
	EventTypeMovedToBreakeven  = "moved_to_breakeven"
	EventTypeTrailingActivated = "trailing_activated"
	EventTypeTrailingUpdated   = "trailing_updated"
	EventTypePositionClosed    = "position_closed"
	EventTypeOrderCancelled    = "order_cancelled"
	EventTypeExternalClose     = "external_close"
)

// Event source constants
const (
	EventSourceGinie    = "ginie"
	EventSourceManual   = "manual"
	EventSourceBinance  = "binance"
	EventSourceTrailing = "trailing"
	EventSourceExternal = "external"
)

// TradeLifecycleEventSummary provides aggregated stats for a trade's lifecycle
type TradeLifecycleEventSummary struct {
	FuturesTradeID    int64     `json:"futures_trade_id"`
	TotalEvents       int       `json:"total_events"`
	SLRevisions       int       `json:"sl_revisions"`
	TPLevelsHit       int       `json:"tp_levels_hit"`
	TrailingActivated bool      `json:"trailing_activated"`
	MovedToBreakeven  bool      `json:"moved_to_breakeven"`
	CloseReason       string    `json:"close_reason"`
	CloseSource       string    `json:"close_source"` // internal or external
	StartTime         time.Time `json:"start_time"`
	EndTime           *time.Time `json:"end_time,omitempty"`
	Duration          *int64    `json:"duration_seconds,omitempty"`

	// Trade details (fetched from futures_trades table)
	Symbol            string    `json:"symbol,omitempty"`
	Mode              string    `json:"mode,omitempty"`
	PositionSide      string    `json:"position_side,omitempty"`
	EntryPrice        float64   `json:"entry_price,omitempty"`
	ExitPrice         *float64  `json:"exit_price,omitempty"`
	Quantity          float64   `json:"quantity,omitempty"`
	Leverage          int       `json:"leverage,omitempty"`
	FinalPnL          *float64  `json:"final_pnl,omitempty"`
	FinalPnLPercent   *float64  `json:"final_pnl_percent,omitempty"`
	TradeStatus       string    `json:"trade_status,omitempty"`

	// Event type breakdown
	EventsByType      map[string]int `json:"events_by_type,omitempty"`
	TrailingUpdates   int       `json:"trailing_updates"`
}
