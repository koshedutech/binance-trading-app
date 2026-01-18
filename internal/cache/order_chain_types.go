// Package cache provides Redis-based caching for order chains.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.20: Order Chain Redis Cache Layer
package cache

import (
	"time"
)

// OrderChainState represents the cached state of an order chain in Redis.
// This is a denormalized structure optimized for fast UI reads.
type OrderChainState struct {
	// Identity
	ChainID  string `json:"chainId"`
	UserID   string `json:"userId"`
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`     // "LONG" or "SHORT"
	ModeCode string `json:"modeCode"` // "ULT", "SCA", "SWI", "POS"
	Status   string `json:"status"`   // "PENDING", "ACTIVE", "PARTIAL", "CLOSED", "CANCELLED"

	// Entry details
	Entry *OrderChainEntry `json:"entry,omitempty"`

	// Position state
	Position *OrderChainPosition `json:"position,omitempty"`

	// Stop Loss state
	StopLoss *OrderChainStopLoss `json:"stopLoss,omitempty"`

	// Take Profit state (normal mode)
	TakeProfit *OrderChainTakeProfit `json:"takeProfit,omitempty"`

	// Position Optimization TPs
	PositionOptEnabled bool                  `json:"positionOptEnabled"`
	TP1                *OrderChainTPLevel    `json:"tp1,omitempty"`
	TP2                *OrderChainTPLevel    `json:"tp2,omitempty"`
	TP3                *OrderChainTPLevel    `json:"tp3,omitempty"`

	// Hedging
	HedgeChainID  *string `json:"hedgeChainId,omitempty"`
	IsHedge       bool    `json:"isHedge"`
	ParentChainID *string `json:"parentChainId,omitempty"`

	// Event tracking
	EventCount   int `json:"eventCount"`
	LastEventSeq int `json:"lastEventSeq"`

	// P&L (for closed chains)
	RealizedPnL *float64 `json:"realizedPnl,omitempty"`
	TotalFees   *float64 `json:"totalFees,omitempty"`

	// Timestamps
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
	ClosedAt  *string `json:"closedAt,omitempty"`
}

// OrderChainEntry represents entry order state
type OrderChainEntry struct {
	Price          float64 `json:"price"`
	Quantity       float64 `json:"quantity"`
	Status         string  `json:"status"` // "PENDING", "FILLED", "CANCELLED"
	FilledAt       *string `json:"filledAt,omitempty"`
	BinanceOrderID *int64  `json:"binanceOrderId,omitempty"`
}

// OrderChainPosition represents current position state
type OrderChainPosition struct {
	Status            string  `json:"status"` // "ACTIVE", "PARTIAL", "CLOSED"
	EntryPrice        float64 `json:"entryPrice"`
	Quantity          float64 `json:"quantity"`
	RemainingQuantity float64 `json:"remainingQuantity"`
}

// OrderChainStopLoss represents stop loss state
type OrderChainStopLoss struct {
	CurrentPrice      float64 `json:"currentPrice"`
	BinanceOrderID    *int64  `json:"binanceOrderId,omitempty"`
	ModificationCount int     `json:"modificationCount"`
	LastModified      *string `json:"lastModified,omitempty"`
}

// OrderChainTakeProfit represents take profit state (normal mode)
type OrderChainTakeProfit struct {
	Mode              string  `json:"mode"` // "NORMAL" or "POSITION_OPT"
	CurrentPrice      float64 `json:"currentPrice"`
	BinanceOrderID    *int64  `json:"binanceOrderId,omitempty"`
	ModificationCount int     `json:"modificationCount"`
	LastModified      *string `json:"lastModified,omitempty"`
}

// OrderChainTPLevel represents a TP level for position optimization
type OrderChainTPLevel struct {
	Price          float64 `json:"price"`
	Quantity       float64 `json:"quantity"`
	Status         string  `json:"status"` // "PENDING", "PLACED", "FILLED"
	BinanceOrderID *int64  `json:"binanceOrderId,omitempty"`
	FilledAt       *string `json:"filledAt,omitempty"`
}

// ChainEventSummary represents a summarized event for the recent events list
type ChainEventSummary struct {
	ChainID       string  `json:"chainId"`
	EventType     string  `json:"eventType"`
	OrderType     *string `json:"orderType,omitempty"`
	Price         *float64 `json:"price,omitempty"`
	OldPrice      *float64 `json:"oldPrice,omitempty"`
	Quantity      *float64 `json:"quantity,omitempty"`
	PnL           *float64 `json:"pnl,omitempty"`
	Timestamp     string  `json:"timestamp"`
	EventSequence int     `json:"eventSequence"`
}

// Redis key prefixes for order chains
const (
	// PrefixOrderChain is the prefix for individual chain state
	// Full key: order_chain:{user_id}:{chain_id}
	PrefixOrderChain = "order_chain:%s:%s"

	// PrefixActiveChains is the prefix for the active chains set per user
	// Full key: order_chains:active:{user_id}
	PrefixActiveChains = "order_chains:active:%s"

	// PrefixRecentEvents is the prefix for recent events list per user
	// Full key: order_chain_events:recent:{user_id}
	PrefixRecentEvents = "order_chain_events:recent:%s"
)

// RecentEventsTTL is the TTL for recent events list (1 hour)
const RecentEventsTTL = time.Hour

// RecentEventsMaxLength is the maximum number of recent events to keep
const RecentEventsMaxLength = 100

// OrderChainKey generates a cache key for an individual order chain
func OrderChainKey(userID, chainID string) string {
	return "order_chain:" + userID + ":" + chainID
}

// ActiveChainsKey generates a cache key for a user's active chains set
func ActiveChainsKey(userID string) string {
	return "order_chains:active:" + userID
}

// RecentEventsKey generates a cache key for a user's recent events list
func RecentEventsKey(userID string) string {
	return "order_chain_events:recent:" + userID
}
