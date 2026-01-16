// Package settlement provides the position snapshot service for Epic 8 Story 8.1.
// This service captures end-of-day position snapshots for daily P&L tracking.
package settlement

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/orders"
)

// PositionSnapshotService handles end-of-day position snapshots
type PositionSnapshotService struct {
	repo          *database.Repository
	clientFactory *binance.ClientFactory
}

// NewPositionSnapshotService creates a new position snapshot service
func NewPositionSnapshotService(repo *database.Repository, clientFactory *binance.ClientFactory) *PositionSnapshotService {
	return &PositionSnapshotService{
		repo:          repo,
		clientFactory: clientFactory,
	}
}

// SnapshotOpenPositions captures all open positions for a user at a specific time
// This is the main entry point for EOD position snapshots
func (s *PositionSnapshotService) SnapshotOpenPositions(ctx context.Context, userID string, snapshotDate time.Time) (*SnapshotResult, error) {
	startTime := time.Now()
	result := &SnapshotResult{
		UserID:       userID,
		SnapshotDate: snapshotDate,
		Snapshots:    make([]PositionSnapshot, 0),
		Success:      false,
	}

	// Validate userID
	if userID == "" {
		result.Error = "userID cannot be empty"
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("userID cannot be empty")
	}

	// Get Binance client for user
	client, err := s.clientFactory.GetFuturesClientForUser(ctx, userID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get Binance client: %v", err)
		result.Duration = time.Since(startTime)
		log.Printf("[SETTLEMENT] Error getting Binance client for user %s: %v", userID, err)
		return result, err
	}

	// Fetch all positions from Binance
	positions, err := client.GetPositions()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get positions from Binance: %v", err)
		result.Duration = time.Since(startTime)
		log.Printf("[SETTLEMENT] Error fetching positions for user %s: %v", userID, err)
		return result, err
	}

	log.Printf("[SETTLEMENT] Fetched %d positions for user %s", len(positions), userID)

	// Process each position
	var dbSnapshots []database.DailyPositionSnapshot
	var totalUnrealizedPnL float64

	for _, pos := range positions {
		// Skip zero-quantity positions (closed)
		if pos.PositionAmt == 0 {
			continue
		}

		// Extract mode and clientOrderId from orders
		// Note: FuturesPosition doesn't include clientOrderId directly
		// We'll need to get it from open orders or default to UNKNOWN
		mode, clientOrderID := s.extractModeForPosition(ctx, client, pos)

		// Create snapshot
		snapshot := PositionSnapshot{
			UserID:        userID,
			SnapshotDate:  snapshotDate,
			Symbol:        pos.Symbol,
			PositionSide:  pos.PositionSide,
			Quantity:      math.Abs(pos.PositionAmt),
			EntryPrice:    pos.EntryPrice,
			MarkPrice:     pos.MarkPrice,
			UnrealizedPnL: pos.UnrealizedProfit,
			Mode:          mode,
			ClientOrderID: clientOrderID,
			Leverage:      pos.Leverage,
			MarginType:    pos.MarginType,
		}

		result.Snapshots = append(result.Snapshots, snapshot)
		totalUnrealizedPnL += pos.UnrealizedProfit

		// Convert to database snapshot
		var clientOrderIDPtr *string
		if clientOrderID != "" {
			clientOrderIDPtr = &clientOrderID
		}

		dbSnapshot := database.DailyPositionSnapshot{
			UserID:        userID,
			SnapshotDate:  snapshotDate,
			Symbol:        pos.Symbol,
			PositionSide:  pos.PositionSide,
			Quantity:      math.Abs(pos.PositionAmt),
			EntryPrice:    pos.EntryPrice,
			MarkPrice:     pos.MarkPrice,
			UnrealizedPnL: pos.UnrealizedProfit,
			Mode:          mode,
			ClientOrderID: clientOrderIDPtr,
			Leverage:      pos.Leverage,
			MarginType:    pos.MarginType,
		}
		dbSnapshots = append(dbSnapshots, dbSnapshot)
	}

	result.PositionCount = len(result.Snapshots)
	result.TotalUnrealizedPnL = totalUnrealizedPnL

	// Save snapshots to database
	if len(dbSnapshots) > 0 {
		if err := s.repo.SaveDailyPositionSnapshots(ctx, dbSnapshots); err != nil {
			result.Error = fmt.Sprintf("failed to save snapshots: %v", err)
			result.Duration = time.Since(startTime)
			log.Printf("[SETTLEMENT] Error saving snapshots for user %s: %v", userID, err)
			return result, err
		}
		log.Printf("[SETTLEMENT] Saved %d position snapshots for user %s", len(dbSnapshots), userID)
	} else {
		log.Printf("[SETTLEMENT] No open positions to snapshot for user %s", userID)
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	log.Printf("[SETTLEMENT] Snapshot completed for user %s: %d positions, total unrealized P&L: %.2f, duration: %v",
		userID, result.PositionCount, result.TotalUnrealizedPnL, result.Duration)

	return result, nil
}

// extractModeForPosition tries to determine the trading mode for a position
// by looking at open orders with clientOrderId
// Returns both the mode and the clientOrderID that was found
func (s *PositionSnapshotService) extractModeForPosition(ctx context.Context, client binance.FuturesClient, pos binance.FuturesPosition) (string, string) {
	// Try to get open orders for this symbol to find clientOrderId
	openOrders, err := client.GetOpenOrders(pos.Symbol)
	if err != nil {
		log.Printf("[SETTLEMENT] Warning: failed to get open orders for %s: %v", pos.Symbol, err)
		return ModeUnknown, ""
	}

	// Look for orders that match this position's side
	for _, order := range openOrders {
		if order.PositionSide == pos.PositionSide && order.ClientOrderId != "" {
			mode := extractModeFromClientOrderID(order.ClientOrderId)
			if mode != ModeUnknown {
				return mode, order.ClientOrderId
			}
		}
	}

	// If no matching order found with clientOrderId, check all orders
	allOrders, err := client.GetAllOrders(pos.Symbol, 100)
	if err != nil {
		log.Printf("[SETTLEMENT] Warning: failed to get all orders for %s: %v", pos.Symbol, err)
		return ModeUnknown, ""
	}

	// Look for filled orders that match this position's side
	for _, order := range allOrders {
		if order.PositionSide == pos.PositionSide && order.ClientOrderId != "" {
			mode := extractModeFromClientOrderID(order.ClientOrderId)
			if mode != ModeUnknown {
				return mode, order.ClientOrderId
			}
		}
	}

	return ModeUnknown, ""
}

// extractModeFromClientOrderID uses ParseClientOrderId from Epic 7 to extract the mode
func extractModeFromClientOrderID(clientOrderID string) string {
	if clientOrderID == "" {
		return ModeUnknown
	}

	parsed := orders.ParseClientOrderId(clientOrderID)
	if parsed == nil {
		return ModeUnknown
	}

	// Convert TradingMode to our mode string
	switch parsed.Mode {
	case orders.ModeScalp:
		return ModeScalp
	case orders.ModeSwing:
		return ModeSwing
	case orders.ModePosition:
		return ModePosition
	case orders.ModeUltraFast:
		return ModeUltraFast
	default:
		return ModeUnknown
	}
}

// SnapshotOpenPositionsWithClientOrderID is an alternative method that uses provided clientOrderIDs
// This is useful when we already know the clientOrderID for each position (e.g., from Redis position state)
func (s *PositionSnapshotService) SnapshotOpenPositionsWithClientOrderID(
	ctx context.Context,
	userID string,
	snapshotDate time.Time,
	positionClientOrderIDs map[string]string, // symbol -> clientOrderID
) (*SnapshotResult, error) {
	startTime := time.Now()
	result := &SnapshotResult{
		UserID:       userID,
		SnapshotDate: snapshotDate,
		Snapshots:    make([]PositionSnapshot, 0),
		Success:      false,
	}

	// Get Binance client for user
	client, err := s.clientFactory.GetFuturesClientForUser(ctx, userID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get Binance client: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Fetch all positions from Binance
	positions, err := client.GetPositions()
	if err != nil {
		result.Error = fmt.Sprintf("failed to get positions from Binance: %v", err)
		result.Duration = time.Since(startTime)
		return result, err
	}

	// Process each position
	var dbSnapshots []database.DailyPositionSnapshot
	var totalUnrealizedPnL float64

	for _, pos := range positions {
		// Skip zero-quantity positions
		if pos.PositionAmt == 0 {
			continue
		}

		// Use provided clientOrderID if available
		clientOrderID := positionClientOrderIDs[pos.Symbol]
		mode := extractModeFromClientOrderID(clientOrderID)

		// Create database snapshot
		var clientOrderIDPtr *string
		if clientOrderID != "" {
			clientOrderIDPtr = &clientOrderID
		}

		snapshot := PositionSnapshot{
			UserID:        userID,
			SnapshotDate:  snapshotDate,
			Symbol:        pos.Symbol,
			PositionSide:  pos.PositionSide,
			Quantity:      math.Abs(pos.PositionAmt),
			EntryPrice:    pos.EntryPrice,
			MarkPrice:     pos.MarkPrice,
			UnrealizedPnL: pos.UnrealizedProfit,
			Mode:          mode,
			ClientOrderID: clientOrderID,
			Leverage:      pos.Leverage,
			MarginType:    pos.MarginType,
		}

		result.Snapshots = append(result.Snapshots, snapshot)
		totalUnrealizedPnL += pos.UnrealizedProfit

		dbSnapshot := database.DailyPositionSnapshot{
			UserID:        userID,
			SnapshotDate:  snapshotDate,
			Symbol:        pos.Symbol,
			PositionSide:  pos.PositionSide,
			Quantity:      math.Abs(pos.PositionAmt),
			EntryPrice:    pos.EntryPrice,
			MarkPrice:     pos.MarkPrice,
			UnrealizedPnL: pos.UnrealizedProfit,
			Mode:          mode,
			ClientOrderID: clientOrderIDPtr,
			Leverage:      pos.Leverage,
			MarginType:    pos.MarginType,
		}
		dbSnapshots = append(dbSnapshots, dbSnapshot)
	}

	result.PositionCount = len(result.Snapshots)
	result.TotalUnrealizedPnL = totalUnrealizedPnL

	// Save snapshots
	if len(dbSnapshots) > 0 {
		if err := s.repo.SaveDailyPositionSnapshots(ctx, dbSnapshots); err != nil {
			result.Error = fmt.Sprintf("failed to save snapshots: %v", err)
			result.Duration = time.Since(startTime)
			return result, err
		}
	}

	result.Success = true
	result.Duration = time.Since(startTime)
	return result, nil
}

// GetSnapshotSummary returns a summary of snapshots for a user on a specific date
func (s *PositionSnapshotService) GetSnapshotSummary(ctx context.Context, userID string, snapshotDate time.Time) (*SnapshotSummary, error) {
	snapshots, err := s.repo.GetDailyPositionSnapshots(ctx, userID, snapshotDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots: %w", err)
	}

	modeBreakdown, err := s.repo.GetModeBreakdownForDate(ctx, userID, snapshotDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get mode breakdown: %w", err)
	}

	var totalPnL float64
	for _, s := range snapshots {
		totalPnL += s.UnrealizedPnL
	}

	// Convert database ModeBreakdown to settlement ModeBreakdown
	var breakdowns []ModeBreakdown
	for _, mb := range modeBreakdown {
		breakdowns = append(breakdowns, ModeBreakdown{
			Mode:          mb.Mode,
			PositionCount: mb.PositionCount,
			UnrealizedPnL: mb.UnrealizedPnL,
		})
	}

	return &SnapshotSummary{
		UserID:             userID,
		SnapshotDate:       snapshotDate,
		TotalPositions:     len(snapshots),
		TotalUnrealizedPnL: totalPnL,
		ModeBreakdowns:     breakdowns,
	}, nil
}
