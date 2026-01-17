// Package autopilot provides trading automation for Binance futures.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.11: Position State Tracking Integration
//
// This file provides integration between GinieAutopilot and the PositionTracker
// for tracking position states from entry fill to close.
package autopilot

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"binance-trading-bot/internal/orders"

	"github.com/rs/zerolog"
)

// parseUserIDToInt64 safely converts a string user ID to int64.
// Returns an error if the string is empty or not a valid integer.
func parseUserIDToInt64(userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("empty user ID")
	}
	return strconv.ParseInt(userID, 10, 64)
}

// PositionStateTracker interface defines methods needed by GinieAutopilot
// for tracking position states. This allows for dependency injection and testing.
type PositionStateTracker interface {
	OnEntryFilled(ctx context.Context, event orders.EntryFilledEvent) (*orders.PositionState, error)
	OnPartialClose(ctx context.Context, userID int64, event orders.PartialCloseEvent) error
	OnPositionClosed(ctx context.Context, userID int64, chainID string, realizedPnL float64, closeReason string) error
	GetPositionByChainID(ctx context.Context, userID int64, chainID string) (*orders.PositionState, error)
	GetActivePositions(ctx context.Context, userID int64) ([]*orders.PositionState, error)
	LoadActivePositions(ctx context.Context, userID int64) error
}

// PositionStateIntegration provides helper methods for tracking position states
// in the GinieAutopilot. This is a companion struct that can be embedded or used
// as a dependency in GinieAutopilot.
type PositionStateIntegration struct {
	tracker PositionStateTracker
	logger  zerolog.Logger
}

// NewPositionStateIntegration creates a new integration instance
func NewPositionStateIntegration(tracker PositionStateTracker, logger zerolog.Logger) *PositionStateIntegration {
	return &PositionStateIntegration{
		tracker: tracker,
		logger:  logger.With().Str("component", "PositionStateIntegration").Logger(),
	}
}

// RecordEntryFill records a position state when an entry order fills.
// This should be called after a position is created in GinieAutopilot.
//
// Parameters:
//   - userID: The user ID (from ga.userID)
//   - orderID: The Binance order ID from the order response
//   - clientOrderID: The client order ID (e.g., "SCA-17JAN-00001-E")
//   - symbol: Trading pair (e.g., "BTCUSDT")
//   - side: "BUY" for LONG, "SELL" for SHORT
//   - fillPrice: The average fill price
//   - fillQty: The quantity filled
//   - commission: Trading fees paid
//   - updateTime: When the order was filled (Unix timestamp in ms)
func (psi *PositionStateIntegration) RecordEntryFill(
	ctx context.Context,
	userID string,
	orderID int64,
	clientOrderID string,
	symbol string,
	side string,
	fillPrice float64,
	fillQty float64,
	commission float64,
	updateTime int64,
) error {
	if psi.tracker == nil {
		psi.logger.Debug().Msg("Position tracker not configured, skipping entry fill recording")
		return nil
	}

	// Parse user ID to int64
	userIDInt, err := parseUserIDToInt64(userID)
	if err != nil {
		psi.logger.Warn().Str("user_id", userID).Err(err).Msg("Failed to parse user ID")
		return err
	}

	event := orders.EntryFilledEvent{
		UserID:        userIDInt,
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
		Symbol:        symbol,
		Side:          side,
		AvgPrice:      fillPrice,
		ExecutedQty:   fillQty,
		Commission:    commission,
		UpdateTime:    updateTime,
	}

	_, err = psi.tracker.OnEntryFilled(ctx, event)
	if err != nil {
		psi.logger.Error().
			Err(err).
			Str("symbol", symbol).
			Str("client_order_id", clientOrderID).
			Msg("Failed to record entry fill")
		return err
	}

	psi.logger.Info().
		Str("symbol", symbol).
		Str("client_order_id", clientOrderID).
		Float64("fill_price", fillPrice).
		Float64("fill_qty", fillQty).
		Msg("Position state created from entry fill")

	return nil
}

// RecordPartialClose records when a take profit order hits and partially closes the position.
//
// Parameters:
//   - chainID: The order chain ID (e.g., "SCA-17JAN-00001")
//   - orderType: The order type that closed (TP1, TP2, TP3, etc.)
//   - closeQty: The quantity closed
//   - closePrice: The fill price
//   - closePnL: The realized P&L from this partial close
func (psi *PositionStateIntegration) RecordPartialClose(
	ctx context.Context,
	userID string,
	chainID string,
	orderType orders.OrderType,
	closeQty float64,
	closePrice float64,
	closePnL float64,
) error {
	if psi.tracker == nil {
		psi.logger.Debug().Msg("Position tracker not configured, skipping partial close recording")
		return nil
	}

	// Parse user ID to int64
	userIDInt, err := parseUserIDToInt64(userID)
	if err != nil {
		psi.logger.Warn().Str("user_id", userID).Err(err).Msg("Failed to parse user ID")
		return err
	}

	event := orders.PartialCloseEvent{
		ChainID:    chainID,
		ClosedQty:  closeQty,
		ClosePrice: closePrice,
		ClosePnL:   closePnL,
		CloseTime:  time.Now(),
		OrderType:  orderType,
	}

	err = psi.tracker.OnPartialClose(ctx, userIDInt, event)
	if err != nil {
		psi.logger.Error().
			Err(err).
			Str("chain_id", chainID).
			Str("order_type", string(orderType)).
			Msg("Failed to record partial close")
		return err
	}

	psi.logger.Info().
		Str("chain_id", chainID).
		Str("order_type", string(orderType)).
		Float64("close_qty", closeQty).
		Float64("close_pnl", closePnL).
		Msg("Position partial close recorded")

	return nil
}

// RecordPositionClose records when a position is fully closed.
//
// Parameters:
//   - chainID: The order chain ID
//   - realizedPnL: The total realized P&L
//   - closeReason: Why the position was closed (e.g., "sl_hit", "manual", "tp_hit")
func (psi *PositionStateIntegration) RecordPositionClose(
	ctx context.Context,
	userID string,
	chainID string,
	realizedPnL float64,
	closeReason string,
) error {
	if psi.tracker == nil {
		psi.logger.Debug().Msg("Position tracker not configured, skipping position close recording")
		return nil
	}

	// Parse user ID to int64
	userIDInt, err := parseUserIDToInt64(userID)
	if err != nil {
		psi.logger.Warn().Str("user_id", userID).Err(err).Msg("Failed to parse user ID")
		return err
	}

	err = psi.tracker.OnPositionClosed(ctx, userIDInt, chainID, realizedPnL, closeReason)
	if err != nil {
		psi.logger.Error().
			Err(err).
			Str("chain_id", chainID).
			Str("close_reason", closeReason).
			Msg("Failed to record position close")
		return err
	}

	psi.logger.Info().
		Str("chain_id", chainID).
		Float64("realized_pnl", realizedPnL).
		Str("close_reason", closeReason).
		Msg("Position close recorded")

	return nil
}

// GetPositionState retrieves the current position state for a chain.
func (psi *PositionStateIntegration) GetPositionState(
	ctx context.Context,
	userID string,
	chainID string,
) (*orders.PositionState, error) {
	if psi.tracker == nil {
		return nil, nil
	}

	// Parse user ID to int64
	userIDInt, err := parseUserIDToInt64(userID)
	if err != nil {
		psi.logger.Warn().Str("user_id", userID).Err(err).Msg("Failed to parse user ID")
		return nil, err
	}

	return psi.tracker.GetPositionByChainID(ctx, userIDInt, chainID)
}

// GetActivePositionStates retrieves all active position states for a user.
func (psi *PositionStateIntegration) GetActivePositionStates(
	ctx context.Context,
	userID string,
) ([]*orders.PositionState, error) {
	if psi.tracker == nil {
		return nil, nil
	}

	// Parse user ID to int64
	userIDInt, err := parseUserIDToInt64(userID)
	if err != nil {
		psi.logger.Warn().Str("user_id", userID).Err(err).Msg("Failed to parse user ID")
		return nil, err
	}

	return psi.tracker.GetActivePositions(ctx, userIDInt)
}

// LoadActivePositions loads active positions into the tracker's cache.
// Call this during initialization to warm up the cache.
func (psi *PositionStateIntegration) LoadActivePositions(ctx context.Context, userID string) error {
	if psi.tracker == nil {
		return nil
	}

	// Parse user ID to int64
	userIDInt, err := parseUserIDToInt64(userID)
	if err != nil {
		psi.logger.Warn().Str("user_id", userID).Err(err).Msg("Failed to parse user ID")
		return err
	}

	return psi.tracker.LoadActivePositions(ctx, userIDInt)
}

// PositionSideToEntrySide converts position side to entry order side.
// LONG positions are entered with BUY, SHORT positions with SELL.
func PositionSideToEntrySide(positionSide string) string {
	if positionSide == "LONG" {
		return "BUY"
	}
	return "SELL"
}

// EntrySideToPositionSide converts entry order side to position side.
// BUY orders create LONG positions, SELL orders create SHORT positions.
func EntrySideToPositionSide(entrySide string) string {
	if entrySide == "BUY" {
		return "LONG"
	}
	return "SHORT"
}

// ExtractChainIDFromClientOrderID extracts the base chain ID from a client order ID.
// For example, "SCA-17JAN-00001-E" returns "SCA-17JAN-00001".
func ExtractChainIDFromClientOrderID(clientOrderID string) string {
	parsed := orders.ParseClientOrderId(clientOrderID)
	if parsed == nil {
		return ""
	}
	return parsed.ChainId
}
