// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.17: Chain Event Writer Service
// Story 7.20: Order Chain Redis Cache Layer (write-through caching)
//
// ChainEventWriter is the ONLY service that writes to the order_chain_events
// and order_chains tables, ensuring consistency through atomic transactions.
// It also maintains write-through caching to Redis for active chains.
package orders

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// ChainEventWriterDB defines the database interface for chain event operations
type ChainEventWriterDB interface {
	// Order chain operations
	CreateOrderChain(ctx context.Context, chain *OrderChain) error
	UpdateOrderChain(ctx context.Context, chain *OrderChain) error
	GetOrderChainByID(ctx context.Context, userID, chainID string) (*OrderChain, error)
	GetOrderChainByChainIDOnly(ctx context.Context, chainID string) (*OrderChain, error) // For lookups without userID
	GetActiveOrderChains(ctx context.Context, userID string) ([]*OrderChain, error)
	CloseOrderChain(ctx context.Context, chainID string, closeReason string, realizedPnL, totalFees float64) error
	UpdateOrderChainSLPrice(ctx context.Context, chainID string, newPrice float64) error
	UpdateOrderChainTPPrice(ctx context.Context, chainID string, newPrice float64) error
	LinkHedgeChain(ctx context.Context, primaryChainID, hedgeChainID string) error
	IncrementOrderChainEventCount(ctx context.Context, chainID string, newSeq int) error

	// Chain event operations
	InsertChainEvent(ctx context.Context, event *ChainEvent) error
	GetChainEvents(ctx context.Context, chainID string) ([]*ChainEvent, error)
	GetNextEventSequence(ctx context.Context, chainID string) (int, error)
}

// ChainCacheInterface defines the cache interface for order chain caching.
// This interface allows the ChainEventWriter to perform write-through caching
// without directly depending on the cache package (avoiding circular imports).
type ChainCacheInterface interface {
	SetChainState(ctx context.Context, chain *ChainCacheState) error
	DeleteChainState(ctx context.Context, userID, chainID string) error
	AddToActiveChains(ctx context.Context, userID, chainID string) error
	RemoveFromActiveChains(ctx context.Context, userID, chainID string) error
	PushRecentEvent(ctx context.Context, userID string, event *ChainCacheEventSummary) error
}

// ChainCacheState mirrors cache.OrderChainState for the interface
type ChainCacheState struct {
	ChainID            string
	UserID             string
	Symbol             string
	Side               string
	ModeCode           string
	Status             string
	PositionOptEnabled bool
	IsHedge            bool
	HedgeChainID       *string
	ParentChainID      *string
	EventCount         int
	LastEventSeq       int
	RealizedPnL        *float64
	TotalFees          *float64
	CreatedAt          string
	UpdatedAt          string
	ClosedAt           *string

	// Entry
	EntryPrice          *float64
	EntryQuantity       *float64
	EntryStatus         string
	EntryFilledAt       *string
	EntryBinanceOrderID *int64

	// Position
	PositionStatus            string
	PositionEntryPrice        float64
	PositionQuantity          float64
	PositionRemainingQuantity float64

	// Stop Loss
	SLCurrentPrice      *float64
	SLBinanceOrderID    *int64
	SLModificationCount int
	SLLastModified      *string

	// Take Profit (normal mode)
	TPMode              string
	TPCurrentPrice      *float64
	TPBinanceOrderID    *int64
	TPModificationCount int
	TPLastModified      *string

	// Position Optimization TPs
	TP1Price          *float64
	TP1Quantity       *float64
	TP1Status         string
	TP1BinanceOrderID *int64
	TP2Price          *float64
	TP2Quantity       *float64
	TP2Status         string
	TP2BinanceOrderID *int64
	TP3Price          *float64
	TP3Quantity       *float64
	TP3Status         string
	TP3BinanceOrderID *int64
}

// ChainCacheEventSummary mirrors cache.ChainEventSummary for the interface
type ChainCacheEventSummary struct {
	ChainID       string
	EventType     string
	OrderType     *string
	Price         *float64
	OldPrice      *float64
	Quantity      *float64
	PnL           *float64
	Timestamp     string
	EventSequence int
}

// ChainClosedCallback is called when a chain is closed with P&L
// Parameters: userID, mode, strategyGroup, subStrategy, realizedPnL
type ChainClosedCallback func(ctx context.Context, userID, mode, strategyGroup, subStrategy string, realizedPnL float64)

// ChainEventWriter writes events to the order chain event store
type ChainEventWriter struct {
	db              ChainEventWriterDB
	cache           ChainCacheInterface  // Optional: nil if caching disabled
	logger          zerolog.Logger
	mu              sync.Mutex           // Protects sequence number generation
	onChainClosed   ChainClosedCallback  // Optional: callback for equity updates
}

// NewChainEventWriter creates a new ChainEventWriter instance
func NewChainEventWriter(db ChainEventWriterDB, logger zerolog.Logger) *ChainEventWriter {
	return &ChainEventWriter{
		db:     db,
		cache:  nil, // No cache by default
		logger: logger.With().Str("component", "ChainEventWriter").Logger(),
	}
}

// NewChainEventWriterWithCache creates a ChainEventWriter with cache support
func NewChainEventWriterWithCache(db ChainEventWriterDB, cache ChainCacheInterface, logger zerolog.Logger) *ChainEventWriter {
	return &ChainEventWriter{
		db:     db,
		cache:  cache,
		logger: logger.With().Str("component", "ChainEventWriter").Logger(),
	}
}

// SetCache sets the cache interface (allows late binding)
func (w *ChainEventWriter) SetCache(cache ChainCacheInterface) {
	w.cache = cache
}

// SetOnChainClosed sets the callback for when a chain is closed.
// This is used for updating sub-strategy equity (compounding).
func (w *ChainEventWriter) SetOnChainClosed(callback ChainClosedCallback) {
	w.onChainClosed = callback
}

// === Chain Lifecycle Methods ===

// CreateChainRequest contains the data needed to create a new chain
type CreateChainRequest struct {
	UserID        string
	ChainID       string
	Symbol        string
	Side          string // "LONG" or "SHORT"
	ModeCode      string // "ULT", "SCA", "SWI", "POS"
	IsHedge       bool
	ParentChainID string // If hedge, link to parent

	// Strategy identification (for compounding equity)
	Mode          string // Full mode name: "scalp", "swing", "position", "ultra_fast"
	StrategyGroup string // e.g., "breakout", "trending"
	SubStrategy   string // e.g., "ravindra_volume_imbalance"
	Timeframe     string // e.g., "3m", "5m", "15m" - from pattern detection
}

// CreateChain creates a new order chain when an entry order is about to be placed
func (w *ChainEventWriter) CreateChain(ctx context.Context, req CreateChainRequest) (*OrderChain, error) {
	chain := &OrderChain{
		UserID:        req.UserID,
		ChainID:       req.ChainID,
		Symbol:        req.Symbol,
		Side:          req.Side,
		ModeCode:      req.ModeCode,
		Status:        OrderChainStatusPending,
		IsHedge:       req.IsHedge,
		Mode:          req.Mode,
		StrategyGroup: req.StrategyGroup,
		SubStrategy:   req.SubStrategy,
		Timeframe:     req.Timeframe,
	}

	if req.ParentChainID != "" {
		chain.ParentChainID = &req.ParentChainID
	}

	if err := w.db.CreateOrderChain(ctx, chain); err != nil {
		return nil, fmt.Errorf("failed to create order chain: %w", err)
	}

	// Update cache (write-through) - Story 7.20
	w.updateCacheAfterWrite(ctx, req.UserID, req.ChainID)

	w.logger.Info().
		Str("chain_id", req.ChainID).
		Str("symbol", req.Symbol).
		Str("side", req.Side).
		Str("mode", req.ModeCode).
		Bool("is_hedge", req.IsHedge).
		Msg("Order chain created")

	return chain, nil
}

// CloseChain marks a chain as closed when the position is fully exited
func (w *ChainEventWriter) CloseChain(ctx context.Context, chainID string, reason string, totalPnL, totalFees float64) error {
	// Get the chain first to get userID for cache operations
	chain, _ := w.db.GetOrderChainByID(ctx, "", chainID)
	userID := ""
	if chain != nil {
		userID = chain.UserID
	}

	// Record the chain close event
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	event := &ChainEvent{
		ChainID:       chainID,
		EventSequence: seq,
		EventType:     EventChainClosed,
		PnL:           &totalPnL,
		Fees:          &totalFees,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert chain close event: %w", err)
	}

	// Update the chain status
	if err := w.db.CloseOrderChain(ctx, chainID, reason, totalPnL, totalFees); err != nil {
		return fmt.Errorf("failed to close order chain: %w", err)
	}

	// Delete from cache on close - Story 7.20 (AC6)
	if userID != "" {
		w.deleteCacheOnClose(ctx, userID, chainID)
		// Push close event to recent events
		w.pushRecentEventToCache(ctx, userID, chainID, EventChainClosed, nil, nil, nil, nil, &totalPnL, seq)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Str("reason", reason).
		Float64("total_pnl", totalPnL).
		Float64("total_fees", totalFees).
		Msg("Order chain closed")

	// Call the onChainClosed callback for equity updates (compounding)
	if w.onChainClosed != nil && chain != nil && chain.Mode != "" && chain.SubStrategy != "" {
		w.onChainClosed(ctx, chain.UserID, chain.Mode, chain.StrategyGroup, chain.SubStrategy, totalPnL)
	}

	return nil
}

// === Entry Order Events ===

// ChainEntryPlacedEvent contains data for entry order placement
type ChainEntryPlacedEvent struct {
	BinanceOrderID       int64
	BinanceClientOrderID string
	Price                float64
	Quantity             float64
	BinanceTimestamp     int64
}

// RecordEntryPlaced records when an entry order is placed on Binance
func (w *ChainEventWriter) RecordEntryPlaced(ctx context.Context, chainID string, req ChainEntryPlacedEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "E"
	event := &ChainEvent{
		ChainID:              chainID,
		EventSequence:        seq,
		EventType:            EventEntryPlaced,
		OrderType:            &orderType,
		BinanceOrderID:       &req.BinanceOrderID,
		BinanceClientOrderID: &req.BinanceClientOrderID,
		Price:                &req.Price,
		Quantity:             &req.Quantity,
		BinanceTimestamp:     &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert entry placed event: %w", err)
	}

	w.logger.Debug().
		Str("chain_id", chainID).
		Int64("binance_order_id", req.BinanceOrderID).
		Float64("price", req.Price).
		Msg("Entry placed event recorded")

	return nil
}

// ChainEntryFilledEvent contains data for entry order fill
type ChainEntryFilledEvent struct {
	FilledPrice      float64
	FilledQuantity   float64
	Fees             float64
	BinanceTimestamp int64
}

// RecordEntryFilled records when an entry order is fully filled
func (w *ChainEventWriter) RecordEntryFilled(ctx context.Context, chainID string, req ChainEntryFilledEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "E"
	event := &ChainEvent{
		ChainID:          chainID,
		EventSequence:    seq,
		EventType:        EventEntryFilled,
		OrderType:        &orderType,
		Price:            &req.FilledPrice,
		FilledQuantity:   &req.FilledQuantity,
		Fees:             &req.Fees,
		BinanceTimestamp: &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert entry filled event: %w", err)
	}

	// Also record position opened event
	if err := w.recordPositionOpened(ctx, chainID, req.FilledPrice, req.FilledQuantity); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to record position opened event")
	}

	// Update chain status to ACTIVE
	// Use GetOrderChainByChainIDOnly since we don't have userID at this point
	chain, err := w.db.GetOrderChainByChainIDOnly(ctx, chainID)
	if err == nil && chain != nil {
		chain.Status = OrderChainStatusActive
		chain.EntryPrice = &req.FilledPrice
		chain.EntryQuantity = &req.FilledQuantity
		chain.RemainingQuantity = &req.FilledQuantity
		now := time.Now()
		chain.EntryFilledAt = &now
		if err := w.db.UpdateOrderChain(ctx, chain); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to update chain status to ACTIVE")
		}

		// Update cache and add to active chains - Story 7.20 (AC4)
		w.updateCacheAfterWrite(ctx, chain.UserID, chainID)
		// Push entry filled event to recent events
		w.pushRecentEventToCache(ctx, chain.UserID, chainID, EventEntryFilled, &orderType, &req.FilledPrice, nil, &req.FilledQuantity, nil, seq)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Float64("filled_price", req.FilledPrice).
		Float64("filled_qty", req.FilledQuantity).
		Msg("Entry filled event recorded")

	return nil
}

// recordPositionOpened records the POSITION_OPENED event
func (w *ChainEventWriter) recordPositionOpened(ctx context.Context, chainID string, entryPrice, quantity float64) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	event := &ChainEvent{
		ChainID:       chainID,
		EventSequence: seq,
		EventType:     EventPositionOpened,
		Price:         &entryPrice,
		Quantity:      &quantity,
	}

	return w.db.InsertChainEvent(ctx, event)
}

// RecordEntryCancelled records when an entry order is cancelled
func (w *ChainEventWriter) RecordEntryCancelled(ctx context.Context, chainID string, reason string) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "E"
	event := &ChainEvent{
		ChainID:            chainID,
		EventSequence:      seq,
		EventType:          EventEntryCancelled,
		OrderType:          &orderType,
		ModificationReason: &reason,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert entry cancelled event: %w", err)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Str("reason", reason).
		Msg("Entry cancelled event recorded")

	return nil
}

// === Stop Loss Events ===

// ChainSLPlacedEvent contains data for SL order placement
type ChainSLPlacedEvent struct {
	BinanceOrderID       int64
	BinanceClientOrderID string
	Price                float64
	BinanceTimestamp     int64
}

// RecordSLPlaced records when a stop loss order is initially placed
func (w *ChainEventWriter) RecordSLPlaced(ctx context.Context, chainID string, req ChainSLPlacedEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "SL"
	event := &ChainEvent{
		ChainID:              chainID,
		EventSequence:        seq,
		EventType:            EventSLPlaced,
		OrderType:            &orderType,
		BinanceOrderID:       &req.BinanceOrderID,
		BinanceClientOrderID: &req.BinanceClientOrderID,
		Price:                &req.Price,
		BinanceTimestamp:     &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert SL placed event: %w", err)
	}

	// Update chain's current SL price
	if err := w.db.UpdateOrderChainSLPrice(ctx, chainID, req.Price); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update chain SL price")
	}

	w.logger.Debug().
		Str("chain_id", chainID).
		Float64("price", req.Price).
		Msg("SL placed event recorded")

	return nil
}

// ChainSLModifiedEvent contains data for SL price modification (the key event)
type ChainSLModifiedEvent struct {
	BinanceOrderID     int64
	OldPrice           float64
	NewPrice           float64
	ModificationSource ModificationSource
	ModificationReason string
	MarketPrice        float64
	BinanceTimestamp   int64
}

// RecordSLModified records when a stop loss price is modified
func (w *ChainEventWriter) RecordSLModified(ctx context.Context, chainID string, req ChainSLModifiedEvent) error {
	// Get userID for cache updates
	chain, _ := w.db.GetOrderChainByID(ctx, "", chainID)
	userID := ""
	if chain != nil {
		userID = chain.UserID
	}

	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "SL"
	event := &ChainEvent{
		ChainID:            chainID,
		EventSequence:      seq,
		EventType:          EventSLModified,
		OrderType:          &orderType,
		BinanceOrderID:     &req.BinanceOrderID,
		Price:              &req.NewPrice,
		OldPrice:           &req.OldPrice,
		ModificationSource: &req.ModificationSource,
		ModificationReason: &req.ModificationReason,
		MarketPrice:        &req.MarketPrice,
		BinanceTimestamp:   &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert SL modified event: %w", err)
	}

	// Update chain's current SL price (increments modification count)
	if err := w.db.UpdateOrderChainSLPrice(ctx, chainID, req.NewPrice); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update chain SL price")
	}

	// Update cache (write-through) - Story 7.20 (AC4)
	if userID != "" {
		w.updateCacheAfterWrite(ctx, userID, chainID)
		// Push SL modified event to recent events for WebSocket
		w.pushRecentEventToCache(ctx, userID, chainID, EventSLModified, &orderType, &req.NewPrice, &req.OldPrice, nil, nil, seq)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Float64("old_price", req.OldPrice).
		Float64("new_price", req.NewPrice).
		Str("source", string(req.ModificationSource)).
		Msg("SL modified event recorded")

	return nil
}

// ChainSLFilledEvent contains data when SL is hit
type ChainSLFilledEvent struct {
	FilledPrice      float64
	PnL              float64
	Fees             float64
	BinanceTimestamp int64
}

// RecordSLFilled records when a stop loss order is filled
func (w *ChainEventWriter) RecordSLFilled(ctx context.Context, chainID string, req ChainSLFilledEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "SL"
	event := &ChainEvent{
		ChainID:          chainID,
		EventSequence:    seq,
		EventType:        EventSLFilled,
		OrderType:        &orderType,
		Price:            &req.FilledPrice,
		PnL:              &req.PnL,
		Fees:             &req.Fees,
		BinanceTimestamp: &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert SL filled event: %w", err)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Float64("filled_price", req.FilledPrice).
		Float64("pnl", req.PnL).
		Msg("SL filled event recorded")

	return nil
}

// === Take Profit Events (Normal Mode) ===

// ChainTPPlacedEvent contains data for TP order placement
type ChainTPPlacedEvent struct {
	BinanceOrderID       int64
	BinanceClientOrderID string
	Price                float64
	BinanceTimestamp     int64
}

// RecordTPPlaced records when a take profit order is placed (normal mode)
func (w *ChainEventWriter) RecordTPPlaced(ctx context.Context, chainID string, req ChainTPPlacedEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "TP"
	event := &ChainEvent{
		ChainID:              chainID,
		EventSequence:        seq,
		EventType:            EventTPPlaced,
		OrderType:            &orderType,
		BinanceOrderID:       &req.BinanceOrderID,
		BinanceClientOrderID: &req.BinanceClientOrderID,
		Price:                &req.Price,
		BinanceTimestamp:     &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert TP placed event: %w", err)
	}

	// Update chain's current TP price
	if err := w.db.UpdateOrderChainTPPrice(ctx, chainID, req.Price); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update chain TP price")
	}

	w.logger.Debug().
		Str("chain_id", chainID).
		Float64("price", req.Price).
		Msg("TP placed event recorded")

	return nil
}

// ChainTPModifiedEvent contains data for TP price modification
type ChainTPModifiedEvent struct {
	BinanceOrderID     int64
	OldPrice           float64
	NewPrice           float64
	ModificationSource ModificationSource
	ModificationReason string
	MarketPrice        float64
	BinanceTimestamp   int64
}

// RecordTPModified records when a take profit price is modified (normal mode only)
func (w *ChainEventWriter) RecordTPModified(ctx context.Context, chainID string, req ChainTPModifiedEvent) error {
	// Get userID for cache updates
	chain, _ := w.db.GetOrderChainByID(ctx, "", chainID)
	userID := ""
	if chain != nil {
		userID = chain.UserID
	}

	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "TP"
	event := &ChainEvent{
		ChainID:            chainID,
		EventSequence:      seq,
		EventType:          EventTPModified,
		OrderType:          &orderType,
		BinanceOrderID:     &req.BinanceOrderID,
		Price:              &req.NewPrice,
		OldPrice:           &req.OldPrice,
		ModificationSource: &req.ModificationSource,
		ModificationReason: &req.ModificationReason,
		MarketPrice:        &req.MarketPrice,
		BinanceTimestamp:   &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert TP modified event: %w", err)
	}

	// Update chain's current TP price (increments modification count)
	if err := w.db.UpdateOrderChainTPPrice(ctx, chainID, req.NewPrice); err != nil {
		w.logger.Warn().Err(err).Msg("Failed to update chain TP price")
	}

	// Update cache (write-through) - Story 7.20 (AC4)
	if userID != "" {
		w.updateCacheAfterWrite(ctx, userID, chainID)
		// Push TP modified event to recent events for WebSocket
		w.pushRecentEventToCache(ctx, userID, chainID, EventTPModified, &orderType, &req.NewPrice, &req.OldPrice, nil, nil, seq)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Float64("old_price", req.OldPrice).
		Float64("new_price", req.NewPrice).
		Str("source", string(req.ModificationSource)).
		Msg("TP modified event recorded")

	return nil
}

// ChainTPFilledEvent contains data when TP is hit
type ChainTPFilledEvent struct {
	FilledPrice      float64
	PnL              float64
	Fees             float64
	BinanceTimestamp int64
}

// RecordTPFilled records when a take profit order is filled
func (w *ChainEventWriter) RecordTPFilled(ctx context.Context, chainID string, req ChainTPFilledEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := "TP"
	event := &ChainEvent{
		ChainID:          chainID,
		EventSequence:    seq,
		EventType:        EventTPFilled,
		OrderType:        &orderType,
		Price:            &req.FilledPrice,
		PnL:              &req.PnL,
		Fees:             &req.Fees,
		BinanceTimestamp: &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert TP filled event: %w", err)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Float64("filled_price", req.FilledPrice).
		Float64("pnl", req.PnL).
		Msg("TP filled event recorded")

	return nil
}

// === Take Profit Events (Position Optimization) ===

// ChainTPLevelPlacedEvent contains data for TP1/TP2/TP3 placement
type ChainTPLevelPlacedEvent struct {
	Level                int
	BinanceOrderID       int64
	BinanceClientOrderID string
	Price                float64
	Quantity             float64 // Percentage-based quantity
	BinanceTimestamp     int64
}

// RecordTPLevelPlaced records when a TP level (TP1/TP2/TP3) is placed
func (w *ChainEventWriter) RecordTPLevelPlaced(ctx context.Context, chainID string, level int, req ChainTPLevelPlacedEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := fmt.Sprintf("TP%d", level)
	var eventType EventType
	switch level {
	case 1:
		eventType = EventTP1Placed
	case 2:
		eventType = EventTP2Placed
	case 3:
		eventType = EventTP3Placed
	default:
		return fmt.Errorf("invalid TP level: %d", level)
	}

	event := &ChainEvent{
		ChainID:              chainID,
		EventSequence:        seq,
		EventType:            eventType,
		OrderType:            &orderType,
		BinanceOrderID:       &req.BinanceOrderID,
		BinanceClientOrderID: &req.BinanceClientOrderID,
		Price:                &req.Price,
		Quantity:             &req.Quantity,
		BinanceTimestamp:     &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert TP%d placed event: %w", level, err)
	}

	w.logger.Debug().
		Str("chain_id", chainID).
		Int("level", level).
		Float64("price", req.Price).
		Msg("TP level placed event recorded")

	return nil
}

// ChainTPLevelFilledEvent contains data when TP1/TP2/TP3 is hit
type ChainTPLevelFilledEvent struct {
	Level            int
	FilledPrice      float64
	FilledQuantity   float64
	PnL              float64
	Fees             float64
	BinanceTimestamp int64
}

// RecordTPLevelFilled records when a TP level is filled
func (w *ChainEventWriter) RecordTPLevelFilled(ctx context.Context, chainID string, level int, req ChainTPLevelFilledEvent) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	orderType := fmt.Sprintf("TP%d", level)
	var eventType EventType
	switch level {
	case 1:
		eventType = EventTP1Filled
	case 2:
		eventType = EventTP2Filled
	case 3:
		eventType = EventTP3Filled
	default:
		return fmt.Errorf("invalid TP level: %d", level)
	}

	event := &ChainEvent{
		ChainID:          chainID,
		EventSequence:    seq,
		EventType:        eventType,
		OrderType:        &orderType,
		Price:            &req.FilledPrice,
		FilledQuantity:   &req.FilledQuantity,
		PnL:              &req.PnL,
		Fees:             &req.Fees,
		BinanceTimestamp: &req.BinanceTimestamp,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert TP%d filled event: %w", level, err)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Int("level", level).
		Float64("filled_price", req.FilledPrice).
		Float64("pnl", req.PnL).
		Msg("TP level filled event recorded")

	return nil
}

// === Hedge Events ===

// LinkHedgeChain links a hedge chain to a primary chain
func (w *ChainEventWriter) LinkHedgeChain(ctx context.Context, primaryChainID, hedgeChainID string) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, primaryChainID)
	if err != nil {
		return err
	}

	event := &ChainEvent{
		ChainID:       primaryChainID,
		EventSequence: seq,
		EventType:     EventHedgeLinked,
	}

	// Store hedge chain ID in modification reason for reference
	event.ModificationReason = &hedgeChainID

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert hedge linked event: %w", err)
	}

	// Update the chain to link the hedge
	if err := w.db.LinkHedgeChain(ctx, primaryChainID, hedgeChainID); err != nil {
		return fmt.Errorf("failed to link hedge chain in database: %w", err)
	}

	w.logger.Info().
		Str("primary_chain_id", primaryChainID).
		Str("hedge_chain_id", hedgeChainID).
		Msg("Hedge chain linked")

	return nil
}

// RecordHedgeClosed records when a hedge position is closed
func (w *ChainEventWriter) RecordHedgeClosed(ctx context.Context, chainID string, pnl float64) error {
	seq, err := w.getNextSequenceAndIncrement(ctx, chainID)
	if err != nil {
		return err
	}

	event := &ChainEvent{
		ChainID:       chainID,
		EventSequence: seq,
		EventType:     EventHedgeClosed,
		PnL:           &pnl,
	}

	if err := w.db.InsertChainEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to insert hedge closed event: %w", err)
	}

	w.logger.Info().
		Str("chain_id", chainID).
		Float64("pnl", pnl).
		Msg("Hedge closed event recorded")

	return nil
}

// === Query Methods ===

// GetChain retrieves the current state of a chain
func (w *ChainEventWriter) GetChain(ctx context.Context, userID, chainID string) (*OrderChain, error) {
	return w.db.GetOrderChainByID(ctx, userID, chainID)
}

// GetChainEvents retrieves all events for a chain
func (w *ChainEventWriter) GetChainEvents(ctx context.Context, chainID string) ([]*ChainEvent, error) {
	return w.db.GetChainEvents(ctx, chainID)
}

// GetActiveChains retrieves all active chains for a user
func (w *ChainEventWriter) GetActiveChains(ctx context.Context, userID string) ([]*OrderChain, error) {
	return w.db.GetActiveOrderChains(ctx, userID)
}

// === Helper Methods ===

// getNextSequenceAndIncrement atomically gets the next sequence number and increments the chain's event count
func (w *ChainEventWriter) getNextSequenceAndIncrement(ctx context.Context, chainID string) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	seq, err := w.db.GetNextEventSequence(ctx, chainID)
	if err != nil {
		return 0, fmt.Errorf("failed to get next event sequence: %w", err)
	}

	// Update the chain's event count
	if err := w.db.IncrementOrderChainEventCount(ctx, chainID, seq); err != nil {
		w.logger.Warn().Err(err).Int("seq", seq).Msg("Failed to increment event count")
	}

	return seq, nil
}

// === Cache Helper Methods (Story 7.20) ===

// updateCacheAfterWrite performs write-through cache update after a PostgreSQL write.
// This is called asynchronously to avoid blocking the main write path.
func (w *ChainEventWriter) updateCacheAfterWrite(ctx context.Context, userID, chainID string) {
	if w.cache == nil {
		return
	}

	// Run cache update in background to avoid blocking
	go func() {
		// Use a fresh context with timeout since the original may be cancelled
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Get fresh chain state from DB
		chain, err := w.db.GetOrderChainByID(cacheCtx, userID, chainID)
		if err != nil {
			w.logger.Warn().Err(err).Str("chain_id", chainID).Msg("Failed to get chain for cache update")
			return
		}
		if chain == nil {
			return
		}

		// Convert to cache state
		cacheState := w.convertToCacheState(chain)

		// Update cache
		if err := w.cache.SetChainState(cacheCtx, cacheState); err != nil {
			w.logger.Warn().Err(err).Str("chain_id", chainID).Msg("Failed to update cache")
		}

		// Ensure chain is in active set if it's active
		if chain.IsActive() {
			if err := w.cache.AddToActiveChains(cacheCtx, userID, chainID); err != nil {
				w.logger.Warn().Err(err).Str("chain_id", chainID).Msg("Failed to add to active chains")
			}
		}
	}()
}

// pushRecentEventToCache pushes an event summary to the recent events list for WebSocket
func (w *ChainEventWriter) pushRecentEventToCache(ctx context.Context, userID, chainID string, eventType EventType, orderType *string, price, oldPrice, quantity, pnl *float64, seq int) {
	if w.cache == nil {
		return
	}

	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		event := &ChainCacheEventSummary{
			ChainID:       chainID,
			EventType:     string(eventType),
			OrderType:     orderType,
			Price:         price,
			OldPrice:      oldPrice,
			Quantity:      quantity,
			PnL:           pnl,
			Timestamp:     time.Now().Format(time.RFC3339),
			EventSequence: seq,
		}

		if err := w.cache.PushRecentEvent(cacheCtx, userID, event); err != nil {
			w.logger.Warn().Err(err).Str("chain_id", chainID).Msg("Failed to push recent event to cache")
		}
	}()
}

// deleteCacheOnClose removes the chain from cache when it's closed
func (w *ChainEventWriter) deleteCacheOnClose(ctx context.Context, userID, chainID string) {
	if w.cache == nil {
		return
	}

	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Delete chain state (this also removes from active chains set)
		if err := w.cache.DeleteChainState(cacheCtx, userID, chainID); err != nil {
			w.logger.Warn().Err(err).Str("chain_id", chainID).Msg("Failed to delete chain from cache")
		}
	}()
}

// convertToCacheState converts an OrderChain to ChainCacheState
func (w *ChainEventWriter) convertToCacheState(chain *OrderChain) *ChainCacheState {
	state := &ChainCacheState{
		ChainID:            chain.ChainID,
		UserID:             chain.UserID,
		Symbol:             chain.Symbol,
		Side:               chain.Side,
		ModeCode:           chain.ModeCode,
		Status:             string(chain.Status),
		PositionOptEnabled: chain.PositionOptEnabled,
		IsHedge:            chain.IsHedge,
		HedgeChainID:       chain.HedgeChainID,
		ParentChainID:      chain.ParentChainID,
		EventCount:         chain.EventCount,
		LastEventSeq:       chain.LastEventSeq,
		RealizedPnL:        chain.RealizedPnL,
		TotalFees:          chain.TotalFees,
		CreatedAt:          chain.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          chain.UpdatedAt.Format(time.RFC3339),
		SLModificationCount: chain.SLModificationCount,
		TPModificationCount: chain.TPModificationCount,
	}

	// ClosedAt
	if chain.ClosedAt != nil {
		s := chain.ClosedAt.Format(time.RFC3339)
		state.ClosedAt = &s
	}

	// Entry details
	if chain.EntryPrice != nil {
		state.EntryPrice = chain.EntryPrice
		state.EntryQuantity = chain.EntryQuantity
		state.EntryStatus = "PENDING"
		if chain.EntryFilledAt != nil {
			state.EntryStatus = "FILLED"
			s := chain.EntryFilledAt.Format(time.RFC3339)
			state.EntryFilledAt = &s
		}
		state.EntryBinanceOrderID = chain.EntryBinanceOrderID
	}

	// Position state
	if chain.Status == OrderChainStatusActive || chain.Status == OrderChainStatusPartial {
		state.PositionStatus = string(chain.Status)
		if chain.EntryPrice != nil {
			state.PositionEntryPrice = *chain.EntryPrice
		}
		if chain.EntryQuantity != nil {
			state.PositionQuantity = *chain.EntryQuantity
		}
		if chain.RemainingQuantity != nil {
			state.PositionRemainingQuantity = *chain.RemainingQuantity
		}
	}

	// Stop Loss
	state.SLCurrentPrice = chain.CurrentSLPrice

	// Take Profit
	if chain.PositionOptEnabled {
		state.TPMode = "POSITION_OPT"
		state.TP1Price = chain.CurrentTP1Price
		state.TP2Price = chain.CurrentTP2Price
		state.TP3Price = chain.CurrentTP3Price
	} else {
		state.TPMode = "NORMAL"
		state.TPCurrentPrice = chain.CurrentTPPrice
	}

	return state
}
