// Package orders provides client order ID generation for Binance futures trading.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.11: Position State Tracking
package orders

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Position state status constants
const (
	PositionStatusActive  = "ACTIVE"  // Full position open
	PositionStatusPartial = "PARTIAL" // Some TPs have hit
	PositionStatusClosed  = "CLOSED"  // Position fully closed
)

// PositionState represents the state of a position from entry fill to close
type PositionState struct {
	ID                 int64      `json:"id"`
	UserID             int64      `json:"user_id"`
	ChainID            string     `json:"chain_id"`
	Symbol             string     `json:"symbol"`
	EntryOrderID       int64      `json:"entry_order_id"`
	EntryClientOrderID string     `json:"entry_client_order_id"`
	EntrySide          string     `json:"entry_side"` // BUY or SELL
	EntryPrice         float64    `json:"entry_price"`
	EntryQuantity      float64    `json:"entry_quantity"`
	EntryValue         float64    `json:"entry_value"`
	EntryFees          float64    `json:"entry_fees"`
	EntryFilledAt      time.Time  `json:"entry_filled_at"`
	Status             string     `json:"status"` // ACTIVE, PARTIAL, CLOSED
	RemainingQuantity  float64    `json:"remaining_quantity"`
	RealizedPnL        float64    `json:"realized_pnl"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ClosedAt           *time.Time `json:"closed_at,omitempty"`
}

// PositionStateRepository defines the interface for position state persistence
type PositionStateRepository interface {
	CreatePositionState(ctx context.Context, position *PositionState) error
	UpdatePositionState(ctx context.Context, position *PositionState) error
	GetPositionByChainID(ctx context.Context, userID int64, chainID string) (*PositionState, error)
	GetPositionsByUserID(ctx context.Context, userID int64, status string) ([]*PositionState, error)
	GetPositionBySymbol(ctx context.Context, userID int64, symbol string, status string) (*PositionState, error)
	GetPositionByEntryOrderID(ctx context.Context, entryOrderID int64) (*PositionState, error)
}

// PositionTracker manages position state tracking
type PositionTracker struct {
	mu     sync.RWMutex
	repo   PositionStateRepository
	logger zerolog.Logger

	// In-memory cache for active positions (keyed by chainID)
	activePositions map[string]*PositionState
}

// NewPositionTracker creates a new PositionTracker instance
func NewPositionTracker(repo PositionStateRepository, logger zerolog.Logger) *PositionTracker {
	return &PositionTracker{
		repo:            repo,
		logger:          logger.With().Str("component", "PositionTracker").Logger(),
		activePositions: make(map[string]*PositionState),
	}
}

// Errors for position tracking
var (
	ErrPositionNotFound      = errors.New("position not found")
	ErrPositionAlreadyExists = errors.New("position already exists for chain")
	ErrNotEntryOrder         = errors.New("not an entry order")
	ErrInvalidQuantity       = errors.New("invalid quantity")
)

// EntryFilledEvent represents the data when an entry order fills
type EntryFilledEvent struct {
	UserID          int64
	OrderID         int64
	ClientOrderID   string
	Symbol          string
	Side            string  // BUY or SELL
	AvgPrice        float64 // Average fill price
	ExecutedQty     float64 // Quantity filled
	Commission      float64 // Fees paid
	UpdateTime      int64   // Unix timestamp in milliseconds
}

// OnEntryFilled is called when an entry order status changes to FILLED
// Creates a new position state record
func (pt *PositionTracker) OnEntryFilled(ctx context.Context, event EntryFilledEvent) (*PositionState, error) {
	// Parse chain ID from client order ID
	parsed := ParseClientOrderId(event.ClientOrderID)
	if parsed == nil {
		pt.logger.Warn().
			Str("client_order_id", event.ClientOrderID).
			Msg("Could not parse client order ID, using fallback")
		// Create a fallback chain ID from the order ID
		parsed = &ParsedOrderId{
			ChainId:   fmt.Sprintf("ORDER-%d", event.OrderID),
			OrderType: OrderTypeEntry,
		}
	}

	if parsed.OrderType != OrderTypeEntry {
		return nil, ErrNotEntryOrder
	}

	if event.ExecutedQty <= 0 {
		return nil, ErrInvalidQuantity
	}

	// Calculate entry value
	entryValue := event.AvgPrice * event.ExecutedQty

	// Parse fill time
	entryFilledAt := time.UnixMilli(event.UpdateTime)
	if event.UpdateTime == 0 {
		entryFilledAt = time.Now()
	}

	// Create position state
	position := &PositionState{
		UserID:             event.UserID,
		ChainID:            parsed.ChainId,
		Symbol:             event.Symbol,
		EntryOrderID:       event.OrderID,
		EntryClientOrderID: event.ClientOrderID,
		EntrySide:          event.Side,
		EntryPrice:         event.AvgPrice,
		EntryQuantity:      event.ExecutedQty,
		EntryValue:         entryValue,
		EntryFees:          event.Commission,
		EntryFilledAt:      entryFilledAt,
		Status:             PositionStatusActive,
		RemainingQuantity:  event.ExecutedQty,
		RealizedPnL:        0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	// Persist to database
	if pt.repo != nil {
		err := pt.repo.CreatePositionState(ctx, position)
		if err != nil {
			pt.logger.Error().
				Err(err).
				Str("chain_id", position.ChainID).
				Msg("Failed to create position state in database")
			return nil, fmt.Errorf("failed to create position state: %w", err)
		}
	}

	// Cache for quick access
	pt.mu.Lock()
	pt.activePositions[position.ChainID] = position
	pt.mu.Unlock()

	pt.logger.Info().
		Str("chain_id", position.ChainID).
		Str("symbol", position.Symbol).
		Str("side", position.EntrySide).
		Float64("entry_price", position.EntryPrice).
		Float64("quantity", position.EntryQuantity).
		Msg("Position state created from entry fill")

	return position, nil
}

// PartialCloseEvent represents data when a partial close occurs (TP hit)
type PartialCloseEvent struct {
	ChainID      string
	ClosedQty    float64
	ClosePrice   float64
	ClosePnL     float64
	CloseTime    time.Time
	OrderType    OrderType // TP1, TP2, TP3, etc.
}

// OnPartialClose is called when a take profit order hits
func (pt *PositionTracker) OnPartialClose(ctx context.Context, userID int64, event PartialCloseEvent) error {
	if event.ClosedQty <= 0 {
		return ErrInvalidQuantity
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Try to get from cache first
	position, exists := pt.activePositions[event.ChainID]
	if !exists && pt.repo != nil {
		// Try to load from database
		var err error
		position, err = pt.repo.GetPositionByChainID(ctx, userID, event.ChainID)
		if err != nil {
			pt.logger.Error().
				Err(err).
				Str("chain_id", event.ChainID).
				Msg("Failed to get position from database")
			return ErrPositionNotFound
		}
		if position != nil {
			pt.activePositions[event.ChainID] = position
		}
	}

	if position == nil {
		return ErrPositionNotFound
	}

	// Update position state
	position.RemainingQuantity -= event.ClosedQty
	position.RealizedPnL += event.ClosePnL
	position.UpdatedAt = time.Now()

	if position.RemainingQuantity <= 0 {
		position.Status = PositionStatusClosed
		now := time.Now()
		position.ClosedAt = &now
		position.RemainingQuantity = 0 // Ensure non-negative

		// Remove from active cache
		delete(pt.activePositions, event.ChainID)
	} else {
		position.Status = PositionStatusPartial
	}

	// Persist to database
	if pt.repo != nil {
		err := pt.repo.UpdatePositionState(ctx, position)
		if err != nil {
			pt.logger.Error().
				Err(err).
				Str("chain_id", event.ChainID).
				Msg("Failed to update position state in database")
			return fmt.Errorf("failed to update position state: %w", err)
		}
	}

	pt.logger.Info().
		Str("chain_id", event.ChainID).
		Str("order_type", string(event.OrderType)).
		Float64("closed_qty", event.ClosedQty).
		Float64("close_pnl", event.ClosePnL).
		Float64("remaining_qty", position.RemainingQuantity).
		Str("new_status", position.Status).
		Msg("Position partially closed")

	return nil
}

// OnPositionClosed is called when a position is fully closed (SL hit, manual close, etc.)
func (pt *PositionTracker) OnPositionClosed(ctx context.Context, userID int64, chainID string, realizedPnL float64, closeReason string) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Try to get from cache first
	position, exists := pt.activePositions[chainID]
	if !exists && pt.repo != nil {
		var err error
		position, err = pt.repo.GetPositionByChainID(ctx, userID, chainID)
		if err != nil {
			return ErrPositionNotFound
		}
	}

	if position == nil {
		return ErrPositionNotFound
	}

	// Mark as closed
	now := time.Now()
	position.Status = PositionStatusClosed
	position.ClosedAt = &now
	position.RemainingQuantity = 0
	position.RealizedPnL += realizedPnL
	position.UpdatedAt = now

	// Remove from active cache
	delete(pt.activePositions, chainID)

	// Persist to database
	if pt.repo != nil {
		err := pt.repo.UpdatePositionState(ctx, position)
		if err != nil {
			pt.logger.Error().
				Err(err).
				Str("chain_id", chainID).
				Msg("Failed to update position state on close")
			return fmt.Errorf("failed to update position state: %w", err)
		}
	}

	pt.logger.Info().
		Str("chain_id", chainID).
		Float64("realized_pnl", position.RealizedPnL).
		Str("close_reason", closeReason).
		Msg("Position closed")

	return nil
}

// GetPositionByChainID retrieves a position by chain ID
func (pt *PositionTracker) GetPositionByChainID(ctx context.Context, userID int64, chainID string) (*PositionState, error) {
	// Check cache first
	pt.mu.RLock()
	position, exists := pt.activePositions[chainID]
	pt.mu.RUnlock()

	if exists {
		return position, nil
	}

	// Try database
	if pt.repo != nil {
		return pt.repo.GetPositionByChainID(ctx, userID, chainID)
	}

	return nil, ErrPositionNotFound
}

// GetActivePositions returns all active positions for a user
func (pt *PositionTracker) GetActivePositions(ctx context.Context, userID int64) ([]*PositionState, error) {
	if pt.repo != nil {
		return pt.repo.GetPositionsByUserID(ctx, userID, PositionStatusActive)
	}

	// Return from cache if no repo
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var positions []*PositionState
	for _, pos := range pt.activePositions {
		if pos.UserID == userID && pos.Status == PositionStatusActive {
			positions = append(positions, pos)
		}
	}
	return positions, nil
}

// GetPositionsByStatus returns positions filtered by status
func (pt *PositionTracker) GetPositionsByStatus(ctx context.Context, userID int64, status string) ([]*PositionState, error) {
	if pt.repo != nil {
		return pt.repo.GetPositionsByUserID(ctx, userID, status)
	}

	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var positions []*PositionState
	for _, pos := range pt.activePositions {
		if pos.UserID == userID && pos.Status == status {
			positions = append(positions, pos)
		}
	}
	return positions, nil
}

// GetPositionBySymbol returns the active position for a symbol
func (pt *PositionTracker) GetPositionBySymbol(ctx context.Context, userID int64, symbol string) (*PositionState, error) {
	if pt.repo != nil {
		return pt.repo.GetPositionBySymbol(ctx, userID, symbol, PositionStatusActive)
	}

	pt.mu.RLock()
	defer pt.mu.RUnlock()

	for _, pos := range pt.activePositions {
		if pos.UserID == userID && pos.Symbol == symbol && pos.Status == PositionStatusActive {
			return pos, nil
		}
	}
	return nil, ErrPositionNotFound
}

// LoadActivePositions loads active positions from database into cache
func (pt *PositionTracker) LoadActivePositions(ctx context.Context, userID int64) error {
	if pt.repo == nil {
		return nil
	}

	positions, err := pt.repo.GetPositionsByUserID(ctx, userID, PositionStatusActive)
	if err != nil {
		return fmt.Errorf("failed to load active positions: %w", err)
	}

	pt.mu.Lock()
	defer pt.mu.Unlock()

	for _, pos := range positions {
		pt.activePositions[pos.ChainID] = pos
	}

	pt.logger.Info().
		Int64("user_id", userID).
		Int("count", len(positions)).
		Msg("Loaded active positions into cache")

	return nil
}

// GetCachedPositionCount returns the number of positions in cache (for testing/debugging)
func (pt *PositionTracker) GetCachedPositionCount() int {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return len(pt.activePositions)
}

// ClearCache clears the in-memory position cache (for testing)
func (pt *PositionTracker) ClearCache() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.activePositions = make(map[string]*PositionState)
}
