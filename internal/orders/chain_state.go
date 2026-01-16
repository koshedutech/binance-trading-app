// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.3: Chain State Models
package orders

import (
	"time"
)

// ChainStatus represents the lifecycle status of an order chain
type ChainStatus string

const (
	// ChainStatusActive indicates the chain has an open position with pending orders
	ChainStatusActive ChainStatus = "active"

	// ChainStatusPartial indicates some orders in the chain have been filled
	ChainStatusPartial ChainStatus = "partial"

	// ChainStatusCompleted indicates all orders in the chain have been filled or the position is closed
	ChainStatusCompleted ChainStatus = "completed"

	// ChainStatusCancelled indicates the chain was cancelled (e.g., entry rejected)
	ChainStatusCancelled ChainStatus = "cancelled"
)

// Direction represents the position direction (LONG or SHORT)
type Direction string

const (
	DirectionLong  Direction = "LONG"
	DirectionShort Direction = "SHORT"
)

// ChainState represents the state of an order chain in memory
type ChainState struct {
	// BaseID is the chain identifier (e.g., "SCA-15JAN-00001")
	BaseID string `json:"baseId"`

	// Symbol is the trading pair (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`

	// Mode is the trading mode used for this chain
	Mode TradingMode `json:"mode"`

	// Direction is the position direction (LONG or SHORT)
	Direction Direction `json:"direction"`

	// Status is the current chain status
	Status ChainStatus `json:"status"`

	// CreatedAt is when the chain was created
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the chain was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// FilledOrders tracks which order types have been filled
	FilledOrders map[OrderType]bool `json:"filledOrders"`

	// PendingOrders tracks which order types are still pending
	PendingOrders map[OrderType]bool `json:"pendingOrders"`

	// Metadata stores additional chain-specific data
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewChainState creates a new ChainState with the given parameters
func NewChainState(baseID, symbol string, mode TradingMode, direction Direction) *ChainState {
	now := time.Now()
	return &ChainState{
		BaseID:        baseID,
		Symbol:        symbol,
		Mode:          mode,
		Direction:     direction,
		Status:        ChainStatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
		FilledOrders:  make(map[OrderType]bool),
		PendingOrders: make(map[OrderType]bool),
		Metadata:      make(map[string]interface{}),
	}
}

// MarkOrderFilled marks an order type as filled and moves it from pending to filled
func (c *ChainState) MarkOrderFilled(orderType OrderType) {
	delete(c.PendingOrders, orderType)
	c.FilledOrders[orderType] = true
	c.UpdatedAt = time.Now()
}

// MarkOrderPending marks an order type as pending
func (c *ChainState) MarkOrderPending(orderType OrderType) {
	c.PendingOrders[orderType] = true
	c.UpdatedAt = time.Now()
}

// MarkOrderCancelled removes an order type from pending (cancelled orders are not tracked)
func (c *ChainState) MarkOrderCancelled(orderType OrderType) {
	delete(c.PendingOrders, orderType)
	c.UpdatedAt = time.Now()
}

// IsOrderFilled checks if a specific order type has been filled
func (c *ChainState) IsOrderFilled(orderType OrderType) bool {
	return c.FilledOrders[orderType]
}

// IsOrderPending checks if a specific order type is pending
func (c *ChainState) IsOrderPending(orderType OrderType) bool {
	return c.PendingOrders[orderType]
}

// HasEntryFilled checks if the entry order has been filled
func (c *ChainState) HasEntryFilled() bool {
	return c.IsOrderFilled(OrderTypeEntry)
}

// IsActive returns true if the chain is in active or partial status
func (c *ChainState) IsActive() bool {
	return c.Status == ChainStatusActive || c.Status == ChainStatusPartial
}

// IsCompleted returns true if the chain is completed or cancelled
func (c *ChainState) IsCompleted() bool {
	return c.Status == ChainStatusCompleted || c.Status == ChainStatusCancelled
}

// GetPendingOrderCount returns the number of pending orders
func (c *ChainState) GetPendingOrderCount() int {
	return len(c.PendingOrders)
}

// GetFilledOrderCount returns the number of filled orders
func (c *ChainState) GetFilledOrderCount() int {
	return len(c.FilledOrders)
}

// SetMetadata sets a metadata value for the chain
func (c *ChainState) SetMetadata(key string, value interface{}) {
	c.Metadata[key] = value
	c.UpdatedAt = time.Now()
}

// GetMetadata retrieves a metadata value from the chain
func (c *ChainState) GetMetadata(key string) (interface{}, bool) {
	value, exists := c.Metadata[key]
	return value, exists
}

// ValidChainStatuses returns all valid chain status values
func ValidChainStatuses() []ChainStatus {
	return []ChainStatus{
		ChainStatusActive,
		ChainStatusPartial,
		ChainStatusCompleted,
		ChainStatusCancelled,
	}
}

// ValidDirections returns all valid direction values
func ValidDirections() []Direction {
	return []Direction{
		DirectionLong,
		DirectionShort,
	}
}

// IsValidChainStatus checks if a status string is a valid ChainStatus
func IsValidChainStatus(status string) bool {
	switch ChainStatus(status) {
	case ChainStatusActive, ChainStatusPartial, ChainStatusCompleted, ChainStatusCancelled:
		return true
	default:
		return false
	}
}

// IsValidDirection checks if a direction string is a valid Direction
func IsValidDirection(direction string) bool {
	switch Direction(direction) {
	case DirectionLong, DirectionShort:
		return true
	default:
		return false
	}
}
