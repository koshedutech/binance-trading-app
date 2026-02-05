// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.16: Order Chain Event Store
package orders

import (
	"encoding/json"
	"time"
)

// OrderChainStatus represents the lifecycle status of an order chain
type OrderChainStatus string

const (
	OrderChainStatusPending     OrderChainStatus = "PENDING"      // Chain created, waiting for entry
	OrderChainStatusEntryPlaced OrderChainStatus = "ENTRY_PLACED" // Entry order placed on Binance
	OrderChainStatusActive      OrderChainStatus = "ACTIVE"       // Entry filled, position open
	OrderChainStatusPartial     OrderChainStatus = "PARTIAL"      // Some TPs hit, partial position
	OrderChainStatusClosed      OrderChainStatus = "CLOSED"       // Position fully closed
	OrderChainStatusCancelled   OrderChainStatus = "CANCELLED"    // Chain cancelled/aborted
)

// ChainCloseReason represents how the chain was closed
type ChainCloseReason string

const (
	CloseReasonSLHit    ChainCloseReason = "SL_HIT"    // Stop loss triggered
	CloseReasonTPHit    ChainCloseReason = "TP_HIT"    // Take profit triggered
	CloseReasonManual   ChainCloseReason = "MANUAL"    // Manually closed by user
	CloseReasonCancelled ChainCloseReason = "CANCELLED" // Chain was cancelled
)

// EventType represents the type of chain event
type EventType string

const (
	// Entry events
	EventEntryPlaced          EventType = "ENTRY_PLACED"
	EventEntryPartiallyFilled EventType = "ENTRY_PARTIALLY_FILLED"
	EventEntryFilled          EventType = "ENTRY_FILLED"
	EventEntryCancelled       EventType = "ENTRY_CANCELLED"

	// Position events
	EventPositionOpened EventType = "POSITION_OPENED"

	// Stop Loss events
	EventSLPlaced    EventType = "SL_PLACED"
	EventSLModified  EventType = "SL_MODIFIED"
	EventSLFilled    EventType = "SL_FILLED"
	EventSLCancelled EventType = "SL_CANCELLED"

	// Take Profit events (normal mode)
	EventTPPlaced    EventType = "TP_PLACED"
	EventTPModified  EventType = "TP_MODIFIED"
	EventTPFilled    EventType = "TP_FILLED"
	EventTPCancelled EventType = "TP_CANCELLED"

	// Take Profit events (position optimization)
	EventTP1Placed EventType = "TP1_PLACED"
	EventTP1Filled EventType = "TP1_FILLED"
	EventTP2Placed EventType = "TP2_PLACED"
	EventTP2Filled EventType = "TP2_FILLED"
	EventTP3Placed EventType = "TP3_PLACED"
	EventTP3Filled EventType = "TP3_FILLED"

	// Hedge events
	EventHedgeLinked      EventType = "HEDGE_LINKED"
	EventHedgeEntryPlaced EventType = "HEDGE_ENTRY_PLACED"
	EventHedgeEntryFilled EventType = "HEDGE_ENTRY_FILLED"
	EventHedgeSLPlaced    EventType = "HEDGE_SL_PLACED"
	EventHedgeTPPlaced    EventType = "HEDGE_TP_PLACED"
	EventHedgeClosed      EventType = "HEDGE_CLOSED"

	// Chain events
	EventChainClosed EventType = "CHAIN_CLOSED"
)

// ModificationSource indicates who/what triggered a modification
type ModificationSource string

const (
	ModSourceLLMAuto      ModificationSource = "LLM_AUTO"      // Ginie autopilot decision
	ModSourceUserManual   ModificationSource = "USER_MANUAL"   // User changed manually
	ModSourceTrailingStop ModificationSource = "TRAILING_STOP" // Trailing stop adjustment
	ModSourceSystem       ModificationSource = "SYSTEM"        // System-level change
)

// OrderChain represents the master state of an order chain (denormalized for fast reads)
type OrderChain struct {
	ID       int64  `json:"id"`
	UserID   string `json:"user_id"`
	ChainID  string `json:"chain_id"` // e.g., "ULT-18JAN-00001"
	Symbol   string `json:"symbol"`   // e.g., "BTCUSDT"
	Side     string `json:"side"`     // "LONG" or "SHORT"
	ModeCode string `json:"mode_code"` // "ULT", "SCA", "SWI", "POS"

	// Strategy identification (for compounding equity)
	Mode          string `json:"mode,omitempty"`           // Full mode name: "scalp", "swing", "position", "ultra_fast"
	StrategyGroup string `json:"strategy_group,omitempty"` // e.g., "breakout", "trending"
	SubStrategy   string `json:"sub_strategy,omitempty"`   // e.g., "ravindra_volume_imbalance"
	Timeframe     string `json:"timeframe,omitempty"`      // e.g., "3m", "5m", "15m" - from strategy pattern detection

	// Current state
	Status OrderChainStatus `json:"status"`

	// Entry details
	EntryPrice          *float64   `json:"entry_price,omitempty"`
	EntryQuantity       *float64   `json:"entry_quantity,omitempty"`
	EntryFilledAt       *time.Time `json:"entry_filled_at,omitempty"`
	EntryBinanceOrderID *int64     `json:"entry_binance_order_id,omitempty"`

	// Current SL/TP prices
	CurrentSLPrice *float64 `json:"current_sl_price,omitempty"`
	CurrentTPPrice *float64 `json:"current_tp_price,omitempty"` // Single TP (no position opt)

	// Position optimization TPs
	PositionOptEnabled bool     `json:"position_opt_enabled"`
	CurrentTP1Price    *float64 `json:"current_tp1_price,omitempty"`
	CurrentTP2Price    *float64 `json:"current_tp2_price,omitempty"`
	CurrentTP3Price    *float64 `json:"current_tp3_price,omitempty"`

	// Remaining quantity
	RemainingQuantity *float64 `json:"remaining_quantity,omitempty"`

	// Hedging
	HedgeChainID  *string `json:"hedge_chain_id,omitempty"`  // Linked hedge chain
	IsHedge       bool    `json:"is_hedge"`
	ParentChainID *string `json:"parent_chain_id,omitempty"` // If hedge, link to parent

	// Counts for quick display
	SLModificationCount int `json:"sl_modification_count"`
	TPModificationCount int `json:"tp_modification_count"`
	EventCount          int `json:"event_count"`
	LastEventSeq        int `json:"last_event_seq"`

	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`

	// Close details
	CloseReason *string `json:"close_reason,omitempty"`

	// P&L
	RealizedPnL *float64 `json:"realized_pnl,omitempty"`
	TotalFees   *float64 `json:"total_fees,omitempty"`

	// Entry decision context (stored as JSON, populated when order is placed)
	EntryContext json.RawMessage `json:"entry_context,omitempty"`
}

// ChainEvent represents a single event in the order chain lifecycle
type ChainEvent struct {
	ID            int64  `json:"id"`
	ChainID       string `json:"chain_id"`
	EventSequence int    `json:"event_sequence"` // Monotonically increasing per chain

	// Event classification
	EventType EventType `json:"event_type"`
	OrderType *string   `json:"order_type,omitempty"` // E, SL, TP, TP1, TP2, TP3, H, HSL, HTP

	// Binance reference
	BinanceOrderID       *int64  `json:"binance_order_id,omitempty"`
	BinanceClientOrderID *string `json:"binance_client_order_id,omitempty"`

	// Price data
	Price          *float64 `json:"price,omitempty"`
	OldPrice       *float64 `json:"old_price,omitempty"` // For modifications
	Quantity       *float64 `json:"quantity,omitempty"`
	FilledQuantity *float64 `json:"filled_quantity,omitempty"`

	// Order details
	OrderStatus *string `json:"order_status,omitempty"` // NEW, FILLED, CANCELLED
	OrderSide   *string `json:"order_side,omitempty"`   // BUY, SELL

	// Modification context
	ModificationSource *ModificationSource `json:"modification_source,omitempty"`
	ModificationReason *string             `json:"modification_reason,omitempty"`

	// Market context
	MarketPrice *float64 `json:"market_price,omitempty"` // Price when event occurred

	// P&L for close events
	PnL  *float64 `json:"pnl,omitempty"`
	Fees *float64 `json:"fees,omitempty"`

	// Timestamps
	BinanceTimestamp *int64    `json:"binance_timestamp,omitempty"` // Binance server time (ms)
	CreatedAt        time.Time `json:"created_at"`
}

// OrderChainWithEvents combines a chain with its events for tree display
type OrderChainWithEvents struct {
	Chain  *OrderChain    `json:"chain"`
	Events []*ChainEvent  `json:"events"`
}

// Helper methods

// IsActive returns true if the chain is in an active state
func (c *OrderChain) IsActive() bool {
	return c.Status == OrderChainStatusActive || c.Status == OrderChainStatusPartial
}

// IsClosed returns true if the chain is closed or cancelled
func (c *OrderChain) IsClosed() bool {
	return c.Status == OrderChainStatusClosed || c.Status == OrderChainStatusCancelled
}

// GetSide returns "LONG" for BUY entry, "SHORT" for SELL entry
func (c *OrderChain) GetSide() string {
	return c.Side
}

// HasHedge returns true if this chain has a linked hedge chain
func (c *OrderChain) HasHedge() bool {
	return c.HedgeChainID != nil && *c.HedgeChainID != ""
}

// IsHedgeChain returns true if this chain is a hedge position
func (c *OrderChain) IsHedgeChain() bool {
	return c.IsHedge
}

// GetCurrentTP returns the appropriate TP price based on position optimization
func (c *OrderChain) GetCurrentTP() *float64 {
	if c.PositionOptEnabled {
		// Return the next unfilled TP level
		if c.CurrentTP1Price != nil {
			return c.CurrentTP1Price
		}
		if c.CurrentTP2Price != nil {
			return c.CurrentTP2Price
		}
		if c.CurrentTP3Price != nil {
			return c.CurrentTP3Price
		}
		return nil
	}
	return c.CurrentTPPrice
}

// IsSLEvent checks if the event is a stop loss related event
func (e *ChainEvent) IsSLEvent() bool {
	switch e.EventType {
	case EventSLPlaced, EventSLModified, EventSLFilled, EventSLCancelled:
		return true
	default:
		return false
	}
}

// IsTPEvent checks if the event is a take profit related event
func (e *ChainEvent) IsTPEvent() bool {
	switch e.EventType {
	case EventTPPlaced, EventTPModified, EventTPFilled, EventTPCancelled,
		EventTP1Placed, EventTP1Filled, EventTP2Placed, EventTP2Filled,
		EventTP3Placed, EventTP3Filled:
		return true
	default:
		return false
	}
}

// IsModificationEvent checks if this is a modification event
func (e *ChainEvent) IsModificationEvent() bool {
	return e.EventType == EventSLModified || e.EventType == EventTPModified
}

// IsFilledEvent checks if this is a fill event
func (e *ChainEvent) IsFilledEvent() bool {
	switch e.EventType {
	case EventEntryFilled, EventSLFilled, EventTPFilled,
		EventTP1Filled, EventTP2Filled, EventTP3Filled,
		EventHedgeEntryFilled:
		return true
	default:
		return false
	}
}

// GetDisplayTimestamp returns the best timestamp for display
func (e *ChainEvent) GetDisplayTimestamp() time.Time {
	if e.BinanceTimestamp != nil && *e.BinanceTimestamp > 0 {
		return time.UnixMilli(*e.BinanceTimestamp)
	}
	return e.CreatedAt
}
