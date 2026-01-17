// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.12: Order Modification Event Log
package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Modification source constants
const (
	ModificationSourceLLMAuto      = "LLM_AUTO"      // Ginie autopilot automated modification
	ModificationSourceUserManual   = "USER_MANUAL"   // User-initiated manual modification
	ModificationSourceTrailingStop = "TRAILING_STOP" // Trailing stop automatic adjustment
)

// Event type constants
const (
	EventTypePlaced    = "PLACED"    // Initial order placement
	EventTypeModified  = "MODIFIED"  // Price modification
	EventTypeCancelled = "CANCELLED" // Order cancelled
	EventTypeFilled    = "FILLED"    // Order filled
)

// Impact direction constants
const (
	ImpactDirectionBetter  = "BETTER"  // Change favors trader (TP further, more profit potential)
	ImpactDirectionWorse   = "WORSE"   // Change against trader (TP closer, less profit potential)
	ImpactDirectionTighter = "TIGHTER" // SL closer to entry (less risk, locked profit)
	ImpactDirectionWider   = "WIDER"   // SL further from entry (more risk, more potential loss)
	ImpactDirectionInitial = "INITIAL" // Initial placement, no comparison
)

// OrderModificationEvent represents a single modification event for an SL/TP order
type OrderModificationEvent struct {
	ID                 int64                  `json:"id"`
	UserID             int64                  `json:"user_id"`
	ChainID            string                 `json:"chain_id"`
	OrderType          string                 `json:"order_type"` // SL, TP1, TP2, etc.
	BinanceOrderID     *int64                 `json:"binance_order_id,omitempty"`
	EventType          string                 `json:"event_type"`           // PLACED, MODIFIED, CANCELLED, FILLED
	ModificationSource string                 `json:"modification_source"`  // LLM_AUTO, USER_MANUAL, TRAILING_STOP
	Version            int                    `json:"version"`
	OldPrice           *float64               `json:"old_price,omitempty"`
	NewPrice           float64                `json:"new_price"`
	PriceDelta         *float64               `json:"price_delta,omitempty"`
	PriceDeltaPercent  *float64               `json:"price_delta_percent,omitempty"`
	PositionQuantity   float64                `json:"position_quantity"`
	PositionEntryPrice float64                `json:"position_entry_price"`
	DollarImpact       float64                `json:"dollar_impact"`
	ImpactDirection    string                 `json:"impact_direction"`
	ModificationReason string                 `json:"modification_reason"`
	LLMDecisionID      string                 `json:"llm_decision_id,omitempty"`
	LLMConfidence      *float64               `json:"llm_confidence,omitempty"`
	MarketContext      map[string]interface{} `json:"market_context,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
}

// ModificationEventRepository defines the interface for modification event persistence
type ModificationEventRepository interface {
	CreateModificationEvent(ctx context.Context, event *OrderModificationEvent) error
	GetModificationEvents(ctx context.Context, chainID, orderType string) ([]*OrderModificationEvent, error)
	GetLatestModificationVersion(ctx context.Context, chainID, orderType string) (int, error)
	GetModificationEventsByUser(ctx context.Context, userID int64, limit int) ([]*OrderModificationEvent, error)
	GetModificationEventsBySource(ctx context.Context, userID int64, source string, limit int) ([]*OrderModificationEvent, error)
}

// PlaceOrderEvent contains the data for when an SL/TP order is first placed
type PlaceOrderEvent struct {
	UserID          int64
	ChainID         string
	OrderType       string // SL, TP1, TP2, etc.
	BinanceOrderID  *int64
	Price           float64
	PositionQty     float64
	EntryPrice      float64
	Side            string // BUY or SELL (position side)
	Source          string // LLM_AUTO, USER_MANUAL, TRAILING_STOP
	Reason          string
	DecisionID      string
	Confidence      *float64
	MarketContext   map[string]interface{}
}

// ModifyOrderEvent contains the data for when an SL/TP order price is modified
type ModifyOrderEvent struct {
	UserID          int64
	ChainID         string
	OrderType       string // SL, TP1, TP2, etc.
	BinanceOrderID  *int64
	OldPrice        float64
	NewPrice        float64
	PositionQty     float64
	EntryPrice      float64
	Side            string // BUY or SELL (position side)
	Source          string // LLM_AUTO, USER_MANUAL, TRAILING_STOP
	Reason          string
	DecisionID      string
	Confidence      *float64
	MarketContext   map[string]interface{}
}

// CancelOrderEvent contains the data for when an SL/TP order is cancelled
type CancelOrderEvent struct {
	UserID         int64
	ChainID        string
	OrderType      string
	BinanceOrderID *int64
	LastPrice      float64
	PositionQty    float64
	EntryPrice     float64
	Side           string
	Source         string
	Reason         string
}

// FillOrderEvent contains the data for when an SL/TP order is filled
type FillOrderEvent struct {
	UserID         int64
	ChainID        string
	OrderType      string
	BinanceOrderID *int64
	FillPrice      float64
	PositionQty    float64
	EntryPrice     float64
	Side           string
}

// ModificationTracker tracks all modifications to SL/TP orders
type ModificationTracker struct {
	mu     sync.RWMutex
	repo   ModificationEventRepository
	logger zerolog.Logger

	// In-memory cache of latest versions per order (chainID:orderType -> version)
	latestVersions map[string]int
}

// NewModificationTracker creates a new ModificationTracker instance
func NewModificationTracker(repo ModificationEventRepository, logger zerolog.Logger) *ModificationTracker {
	return &ModificationTracker{
		repo:           repo,
		logger:         logger.With().Str("component", "ModificationTracker").Logger(),
		latestVersions: make(map[string]int),
	}
}

// versionKey returns the cache key for a chainID and orderType
func versionKey(chainID, orderType string) string {
	return fmt.Sprintf("%s:%s", chainID, orderType)
}

// OnOrderPlaced is called when an SL/TP order is first placed
func (mt *ModificationTracker) OnOrderPlaced(ctx context.Context, req PlaceOrderEvent) error {
	// Calculate initial dollar impact (distance from entry)
	dollarImpact := mt.calculateInitialImpact(req.EntryPrice, req.Price, req.PositionQty, req.OrderType, req.Side)

	event := &OrderModificationEvent{
		UserID:             req.UserID,
		ChainID:            req.ChainID,
		OrderType:          req.OrderType,
		BinanceOrderID:     req.BinanceOrderID,
		EventType:          EventTypePlaced,
		ModificationSource: req.Source,
		Version:            1,
		OldPrice:           nil, // No old price for initial placement
		NewPrice:           req.Price,
		PriceDelta:         nil,
		PriceDeltaPercent:  nil,
		PositionQuantity:   req.PositionQty,
		PositionEntryPrice: req.EntryPrice,
		DollarImpact:       dollarImpact,
		ImpactDirection:    ImpactDirectionInitial,
		ModificationReason: req.Reason,
		LLMDecisionID:      req.DecisionID,
		LLMConfidence:      req.Confidence,
		MarketContext:      req.MarketContext,
		CreatedAt:          time.Now(),
	}

	// Store in database
	if mt.repo != nil {
		if err := mt.repo.CreateModificationEvent(ctx, event); err != nil {
			mt.logger.Error().
				Err(err).
				Str("chain_id", req.ChainID).
				Str("order_type", req.OrderType).
				Msg("Failed to create modification event")
			return fmt.Errorf("failed to create modification event: %w", err)
		}
	}

	// Update version cache
	mt.mu.Lock()
	mt.latestVersions[versionKey(req.ChainID, req.OrderType)] = 1
	mt.mu.Unlock()

	mt.logger.Info().
		Str("chain_id", req.ChainID).
		Str("order_type", req.OrderType).
		Str("source", req.Source).
		Float64("price", req.Price).
		Float64("dollar_impact", dollarImpact).
		Msg("Order placed event logged")

	return nil
}

// OnOrderModified is called when an SL/TP order price is modified
func (mt *ModificationTracker) OnOrderModified(ctx context.Context, req ModifyOrderEvent) error {
	// Calculate price delta
	priceDelta := req.NewPrice - req.OldPrice
	priceDeltaPercent := 0.0
	if req.OldPrice != 0 {
		priceDeltaPercent = (priceDelta / req.OldPrice) * 100
	}

	// Calculate dollar impact
	dollarImpact := mt.calculateDollarImpact(
		req.EntryPrice,
		req.OldPrice,
		req.NewPrice,
		req.PositionQty,
		req.OrderType,
		req.Side,
	)

	// Determine impact direction
	impactDirection := mt.determineImpactDirection(req.OrderType, priceDelta, req.Side)

	// Lock during version fetch AND database insert to prevent race conditions
	// This ensures two concurrent modifications for the same order get sequential versions
	key := versionKey(req.ChainID, req.OrderType)
	mt.mu.Lock()
	version := mt.getNextVersionLocked(ctx, req.ChainID, req.OrderType)

	event := &OrderModificationEvent{
		UserID:             req.UserID,
		ChainID:            req.ChainID,
		OrderType:          req.OrderType,
		BinanceOrderID:     req.BinanceOrderID,
		EventType:          EventTypeModified,
		ModificationSource: req.Source,
		Version:            version,
		OldPrice:           &req.OldPrice,
		NewPrice:           req.NewPrice,
		PriceDelta:         &priceDelta,
		PriceDeltaPercent:  &priceDeltaPercent,
		PositionQuantity:   req.PositionQty,
		PositionEntryPrice: req.EntryPrice,
		DollarImpact:       dollarImpact,
		ImpactDirection:    impactDirection,
		ModificationReason: req.Reason,
		LLMDecisionID:      req.DecisionID,
		LLMConfidence:      req.Confidence,
		MarketContext:      req.MarketContext,
		CreatedAt:          time.Now(),
	}

	// Store in database while holding lock
	if mt.repo != nil {
		if err := mt.repo.CreateModificationEvent(ctx, event); err != nil {
			mt.mu.Unlock()
			mt.logger.Error().
				Err(err).
				Str("chain_id", req.ChainID).
				Str("order_type", req.OrderType).
				Msg("Failed to create modification event")
			return fmt.Errorf("failed to create modification event: %w", err)
		}
	}

	// Update version cache while still holding lock
	mt.latestVersions[key] = version
	mt.mu.Unlock()

	mt.logger.Info().
		Str("chain_id", req.ChainID).
		Str("order_type", req.OrderType).
		Str("source", req.Source).
		Int("version", version).
		Float64("old_price", req.OldPrice).
		Float64("new_price", req.NewPrice).
		Float64("price_delta", priceDelta).
		Float64("dollar_impact", dollarImpact).
		Str("impact_direction", impactDirection).
		Msg("Order modified event logged")

	return nil
}

// OnOrderCancelled is called when an SL/TP order is cancelled
func (mt *ModificationTracker) OnOrderCancelled(ctx context.Context, req CancelOrderEvent) error {
	version := mt.getNextVersion(ctx, req.ChainID, req.OrderType)

	event := &OrderModificationEvent{
		UserID:             req.UserID,
		ChainID:            req.ChainID,
		OrderType:          req.OrderType,
		BinanceOrderID:     req.BinanceOrderID,
		EventType:          EventTypeCancelled,
		ModificationSource: req.Source,
		Version:            version,
		NewPrice:           req.LastPrice,
		PositionQuantity:   req.PositionQty,
		PositionEntryPrice: req.EntryPrice,
		DollarImpact:       0, // Cancelled orders have no further impact
		ImpactDirection:    ImpactDirectionInitial,
		ModificationReason: req.Reason,
		CreatedAt:          time.Now(),
	}

	if mt.repo != nil {
		if err := mt.repo.CreateModificationEvent(ctx, event); err != nil {
			mt.logger.Error().
				Err(err).
				Str("chain_id", req.ChainID).
				Str("order_type", req.OrderType).
				Msg("Failed to create cancellation event")
			return fmt.Errorf("failed to create cancellation event: %w", err)
		}
	}

	mt.logger.Info().
		Str("chain_id", req.ChainID).
		Str("order_type", req.OrderType).
		Str("reason", req.Reason).
		Msg("Order cancelled event logged")

	return nil
}

// OnOrderFilled is called when an SL/TP order is filled
func (mt *ModificationTracker) OnOrderFilled(ctx context.Context, req FillOrderEvent) error {
	version := mt.getNextVersion(ctx, req.ChainID, req.OrderType)

	// Calculate realized impact
	dollarImpact := mt.calculateRealizedImpact(req.EntryPrice, req.FillPrice, req.PositionQty, req.OrderType, req.Side)

	event := &OrderModificationEvent{
		UserID:             req.UserID,
		ChainID:            req.ChainID,
		OrderType:          req.OrderType,
		BinanceOrderID:     req.BinanceOrderID,
		EventType:          EventTypeFilled,
		ModificationSource: "", // Filled by market
		Version:            version,
		NewPrice:           req.FillPrice,
		PositionQuantity:   req.PositionQty,
		PositionEntryPrice: req.EntryPrice,
		DollarImpact:       dollarImpact,
		ImpactDirection:    ImpactDirectionInitial,
		ModificationReason: "Order filled at market",
		CreatedAt:          time.Now(),
	}

	if mt.repo != nil {
		if err := mt.repo.CreateModificationEvent(ctx, event); err != nil {
			mt.logger.Error().
				Err(err).
				Str("chain_id", req.ChainID).
				Str("order_type", req.OrderType).
				Msg("Failed to create fill event")
			return fmt.Errorf("failed to create fill event: %w", err)
		}
	}

	mt.logger.Info().
		Str("chain_id", req.ChainID).
		Str("order_type", req.OrderType).
		Float64("fill_price", req.FillPrice).
		Float64("dollar_impact", dollarImpact).
		Msg("Order filled event logged")

	return nil
}

// GetModificationHistory retrieves the modification history for an order
func (mt *ModificationTracker) GetModificationHistory(ctx context.Context, chainID, orderType string) ([]*OrderModificationEvent, error) {
	if mt.repo == nil {
		return nil, nil
	}
	return mt.repo.GetModificationEvents(ctx, chainID, orderType)
}

// GetModificationSummary returns summary statistics for modification history
func (mt *ModificationTracker) GetModificationSummary(events []*OrderModificationEvent) *ModificationSummary {
	if len(events) == 0 {
		return nil
	}

	summary := &ModificationSummary{
		TotalModifications: len(events) - 1, // Exclude initial placement
		InitialPrice:       events[0].NewPrice,
		CurrentPrice:       events[len(events)-1].NewPrice,
	}

	// Calculate net changes
	for i := 1; i < len(events); i++ {
		if events[i].DollarImpact != 0 {
			summary.NetDollarImpact += events[i].DollarImpact
		}
		if events[i].PriceDelta != nil {
			summary.NetPriceChange += *events[i].PriceDelta
		}
	}

	if len(events) > 0 {
		summary.LastModifiedAt = events[len(events)-1].CreatedAt
	}

	return summary
}

// ModificationSummary provides aggregate statistics for modification history
type ModificationSummary struct {
	TotalModifications int       `json:"total_modifications"`
	NetPriceChange     float64   `json:"net_price_change"`
	NetDollarImpact    float64   `json:"net_dollar_impact"`
	InitialPrice       float64   `json:"initial_price"`
	CurrentPrice       float64   `json:"current_price"`
	LastModifiedAt     time.Time `json:"last_modified_at"`
}

// getNextVersion returns the next version number for an order
func (mt *ModificationTracker) getNextVersion(ctx context.Context, chainID, orderType string) int {
	key := versionKey(chainID, orderType)

	// Check cache first
	mt.mu.RLock()
	version, exists := mt.latestVersions[key]
	mt.mu.RUnlock()

	if exists {
		return version + 1
	}

	// Fetch from database
	if mt.repo != nil {
		dbVersion, err := mt.repo.GetLatestModificationVersion(ctx, chainID, orderType)
		if err == nil && dbVersion > 0 {
			mt.mu.Lock()
			mt.latestVersions[key] = dbVersion
			mt.mu.Unlock()
			return dbVersion + 1
		}
	}

	return 1
}

// getNextVersionLocked returns the next version number for an order
// IMPORTANT: Caller must already hold mt.mu lock
func (mt *ModificationTracker) getNextVersionLocked(ctx context.Context, chainID, orderType string) int {
	key := versionKey(chainID, orderType)

	// Check cache first (no lock needed, already held by caller)
	version, exists := mt.latestVersions[key]
	if exists {
		return version + 1
	}

	// Fetch from database
	if mt.repo != nil {
		dbVersion, err := mt.repo.GetLatestModificationVersion(ctx, chainID, orderType)
		if err == nil && dbVersion > 0 {
			mt.latestVersions[key] = dbVersion
			return dbVersion + 1
		}
	}

	return 1
}

// calculateInitialImpact calculates the initial dollar impact (potential loss/gain from entry)
func (mt *ModificationTracker) calculateInitialImpact(entryPrice, orderPrice, quantity float64, orderType, side string) float64 {
	// Calculate distance from entry
	distance := math.Abs(orderPrice - entryPrice)
	return distance * quantity
}

// calculateDollarImpact calculates how a price change affects potential P&L
func (mt *ModificationTracker) calculateDollarImpact(entryPrice, oldPrice, newPrice, quantity float64, orderType, side string) float64 {
	// Calculate the change in distance from entry
	oldDistance := math.Abs(oldPrice - entryPrice) * quantity
	newDistance := math.Abs(newPrice - entryPrice) * quantity

	return newDistance - oldDistance
}

// calculateRealizedImpact calculates the realized P&L when an order fills
func (mt *ModificationTracker) calculateRealizedImpact(entryPrice, fillPrice, quantity float64, orderType, side string) float64 {
	// For LONG positions: fillPrice - entryPrice is P&L per unit
	// For SHORT positions: entryPrice - fillPrice is P&L per unit
	isLong := side == "BUY"

	if isLong {
		return (fillPrice - entryPrice) * quantity
	}
	return (entryPrice - fillPrice) * quantity
}

// determineImpactDirection determines if a modification improves or worsens the position
func (mt *ModificationTracker) determineImpactDirection(orderType string, priceDelta float64, side string) string {
	isLong := side == "BUY"
	isSL := orderType == "SL" || orderType == "HSL"

	if isSL {
		if isLong {
			// LONG position: SL moved up = tighter (less loss potential), down = wider (more loss)
			if priceDelta > 0 {
				return ImpactDirectionTighter
			}
			return ImpactDirectionWider
		}
		// SHORT position: SL moved down = tighter, up = wider
		if priceDelta < 0 {
			return ImpactDirectionTighter
		}
		return ImpactDirectionWider
	}

	// For TP orders
	if isLong {
		// LONG position: TP moved up = better (more profit), down = worse (less profit)
		if priceDelta > 0 {
			return ImpactDirectionBetter
		}
		return ImpactDirectionWorse
	}
	// SHORT position: TP moved down = better, up = worse
	if priceDelta < 0 {
		return ImpactDirectionBetter
	}
	return ImpactDirectionWorse
}

// MarshalMarketContext converts market context to JSON for storage
func MarshalMarketContext(ctx map[string]interface{}) ([]byte, error) {
	if ctx == nil {
		return nil, nil
	}
	return json.Marshal(ctx)
}

// UnmarshalMarketContext parses JSON market context
func UnmarshalMarketContext(data []byte) (map[string]interface{}, error) {
	if data == nil {
		return nil, nil
	}
	var ctx map[string]interface{}
	err := json.Unmarshal(data, &ctx)
	return ctx, err
}

// ClearVersionCache clears the version cache (for testing)
func (mt *ModificationTracker) ClearVersionCache() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.latestVersions = make(map[string]int)
}
