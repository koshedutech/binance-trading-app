// Package settlement provides the orchestrating service for daily settlement.
// Epic 8 Story 8.3: Daily Summary Storage - Main settlement orchestrator
package settlement

import (
	"context"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
)

// SettlementService orchestrates the daily settlement process
// It coordinates position snapshots, P&L aggregation, and storage
type SettlementService struct {
	repo            *database.Repository
	clientFactory   *binance.ClientFactory
	snapshotService *PositionSnapshotService
	pnlAggregator   *PnLAggregator
}

// NewSettlementService creates a new settlement service
func NewSettlementService(repo *database.Repository, clientFactory *binance.ClientFactory) *SettlementService {
	return &SettlementService{
		repo:            repo,
		clientFactory:   clientFactory,
		snapshotService: NewPositionSnapshotService(repo, clientFactory),
		pnlAggregator:   NewPnLAggregator(clientFactory),
	}
}

// SettlementResult represents the complete result of a daily settlement
type SettlementResult struct {
	UserID         string                 `json:"user_id"`
	Date           time.Time              `json:"date"`
	Timezone       string                 `json:"timezone"`
	SnapshotResult *SnapshotResult        `json:"snapshot_result"`
	PnLResult      *DailyPnLAggregation   `json:"pnl_result"`
	Summaries      []database.DailyModeSummary `json:"summaries"`
	Success        bool                   `json:"success"`
	Error          string                 `json:"error,omitempty"`
	Duration       time.Duration          `json:"duration"`
}

// RunDailySettlement performs the complete daily settlement for a user
// This includes: position snapshot, P&L aggregation, unrealized P&L calculation, and storage
func (s *SettlementService) RunDailySettlement(ctx context.Context, userID string, settlementDate time.Time, timezone string) (*SettlementResult, error) {
	start := time.Now()
	result := &SettlementResult{
		UserID:   userID,
		Date:     settlementDate,
		Timezone: timezone,
		Success:  false,
	}

	log.Printf("[SETTLEMENT-SERVICE] Starting settlement for user %s on %s (timezone: %s)",
		userID, settlementDate.Format("2006-01-02"), timezone)

	// Load timezone
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("[SETTLEMENT-SERVICE] Invalid timezone %s, using UTC", timezone)
		loc = time.UTC
	}

	// Calculate the trading day boundaries in user's timezone
	dayStart := time.Date(settlementDate.Year(), settlementDate.Month(), settlementDate.Day(), 0, 0, 0, 0, loc)
	dayEnd := dayStart.Add(24 * time.Hour).Add(-time.Second) // End of day

	// Step 1: Take position snapshot (for unrealized P&L tracking)
	snapshotResult, err := s.snapshotService.SnapshotOpenPositions(ctx, userID, settlementDate)
	if err != nil {
		result.Error = fmt.Sprintf("snapshot failed: %v", err)
		result.Duration = time.Since(start)
		return result, err
	}
	result.SnapshotResult = snapshotResult

	// Step 2: Aggregate realized P&L from closed trades
	pnlResult, err := s.pnlAggregator.AggregatePnLByMode(ctx, userID, dayStart, dayEnd)
	if err != nil {
		result.Error = fmt.Sprintf("P&L aggregation failed: %v", err)
		result.Duration = time.Since(start)
		return result, err
	}
	result.PnLResult = pnlResult

	// Step 3: Get yesterday's unrealized P&L for comparison (Story 8.4)
	yesterday := settlementDate.AddDate(0, 0, -1)
	yesterdayUnrealized, err := s.getYesterdayUnrealizedByMode(ctx, userID, yesterday)
	if err != nil {
		log.Printf("[SETTLEMENT-SERVICE] Warning: failed to get yesterday's unrealized P&L: %v", err)
		// Continue with empty map - new positions will have 0 yesterday unrealized
		yesterdayUnrealized = make(map[string]float64)
	}

	// Step 4: Calculate today's unrealized P&L by mode from snapshots
	todayUnrealized := s.calculateUnrealizedByMode(snapshotResult.Snapshots)

	// Step 5: Create daily summaries for each mode
	summaries := s.createDailySummaries(userID, settlementDate, timezone, pnlResult, todayUnrealized, yesterdayUnrealized)
	result.Summaries = summaries

	// Step 6: Save summaries to database
	err = s.repo.SaveDailyModeSummaries(ctx, summaries)
	if err != nil {
		result.Error = fmt.Sprintf("failed to save summaries: %v", err)
		result.Duration = time.Since(start)

		// Mark as failed in database
		errMsg := result.Error
		for _, summary := range summaries {
			_ = s.repo.UpdateSettlementStatus(ctx, userID, settlementDate, summary.Mode, "failed", &errMsg)
		}
		return result, err
	}

	result.Success = true
	result.Duration = time.Since(start)

	log.Printf("[SETTLEMENT-SERVICE] Settlement completed for user %s: %d modes, %d trades, total P&L: %.2f, duration: %v",
		userID, len(summaries), pnlResult.TotalTrades, pnlResult.TotalPnL, result.Duration)

	return result, nil
}

// getYesterdayUnrealizedByMode retrieves yesterday's unrealized P&L grouped by mode
func (s *SettlementService) getYesterdayUnrealizedByMode(ctx context.Context, userID string, yesterdayDate time.Time) (map[string]float64, error) {
	summaries, err := s.repo.GetDailyModeSummaries(ctx, userID, yesterdayDate)
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64)
	for _, summary := range summaries {
		result[summary.Mode] = summary.UnrealizedPnL
	}

	return result, nil
}

// calculateUnrealizedByMode calculates total unrealized P&L grouped by mode from snapshots
func (s *SettlementService) calculateUnrealizedByMode(snapshots []PositionSnapshot) map[string]float64 {
	result := make(map[string]float64)
	totalUnrealized := 0.0

	for _, snapshot := range snapshots {
		result[snapshot.Mode] += snapshot.UnrealizedPnL
		totalUnrealized += snapshot.UnrealizedPnL
	}

	// Add "ALL" mode total
	result[ModeAll] = totalUnrealized

	return result
}

// createDailySummaries creates DailyModeSummary records for all modes
// FIX: Now includes modes with unrealized P&L but no trades (Issue #1, #6)
func (s *SettlementService) createDailySummaries(
	userID string,
	settlementDate time.Time,
	timezone string,
	pnlResult *DailyPnLAggregation,
	todayUnrealized map[string]float64,
	yesterdayUnrealized map[string]float64,
) []database.DailyModeSummary {
	var summaries []database.DailyModeSummary
	processedModes := make(map[string]bool)

	// First pass: Process modes WITH trades
	for mode, modePnL := range pnlResult.ModeResults {
		processedModes[mode] = true

		// Get unrealized P&L values
		todayUnr := todayUnrealized[mode]
		yesterdayUnr := yesterdayUnrealized[mode]
		unrealizedChange := todayUnr - yesterdayUnr

		// Total P&L = Realized + Unrealized Change (matches Binance's daily P&L method)
		totalPnL := modePnL.RealizedPnL + unrealizedChange

		summary := database.DailyModeSummary{
			UserID:              userID,
			SummaryDate:         settlementDate,
			Mode:                mode,
			TradeCount:          modePnL.TradeCount,
			WinCount:            modePnL.WinCount,
			LossCount:           modePnL.LossCount,
			WinRate:             modePnL.WinRate,
			RealizedPnL:         modePnL.RealizedPnL,
			UnrealizedPnL:       todayUnr,
			UnrealizedPnLChange: unrealizedChange,
			TotalPnL:            totalPnL,
			LargestWin:          modePnL.LargestWin,
			LargestLoss:         modePnL.LargestLoss,
			TotalVolume:         modePnL.TotalVolume,
			AvgTradeSize:        modePnL.AvgTradeSize,
			SettlementStatus:    "completed",
			UserTimezone:        timezone,
		}

		summaries = append(summaries, summary)
	}

	// Second pass: Process modes with unrealized P&L but NO trades
	// This captures overnight positions that didn't trade today
	for mode, todayUnr := range todayUnrealized {
		if processedModes[mode] {
			continue // Already processed in first pass
		}
		if mode == ModeAll {
			continue // Handle ALL mode separately below
		}

		yesterdayUnr := yesterdayUnrealized[mode]
		unrealizedChange := todayUnr - yesterdayUnr

		// No trades, so total P&L is just the unrealized change
		summary := database.DailyModeSummary{
			UserID:              userID,
			SummaryDate:         settlementDate,
			Mode:                mode,
			TradeCount:          0,
			WinCount:            0,
			LossCount:           0,
			WinRate:             0,
			RealizedPnL:         0,
			UnrealizedPnL:       todayUnr,
			UnrealizedPnLChange: unrealizedChange,
			TotalPnL:            unrealizedChange, // Only unrealized change, no realized
			SettlementStatus:    "completed",
			UserTimezone:        timezone,
		}

		summaries = append(summaries, summary)
		processedModes[mode] = true
	}

	// Ensure we always have an "ALL" mode summary
	if !processedModes[ModeAll] {
		// Calculate ALL mode unrealized values
		allTodayUnr := todayUnrealized[ModeAll]
		allYesterdayUnr := yesterdayUnrealized[ModeAll]
		allUnrealizedChange := allTodayUnr - allYesterdayUnr

		summaries = append(summaries, database.DailyModeSummary{
			UserID:              userID,
			SummaryDate:         settlementDate,
			Mode:                ModeAll,
			TradeCount:          0,
			WinCount:            0,
			LossCount:           0,
			WinRate:             0,
			RealizedPnL:         0,
			UnrealizedPnL:       allTodayUnr,
			UnrealizedPnLChange: allUnrealizedChange,
			TotalPnL:            allUnrealizedChange,
			SettlementStatus:    "completed",
			UserTimezone:        timezone,
		})
	}

	return summaries
}

// GetSettlementService returns the underlying services for direct access
func (s *SettlementService) GetSnapshotService() *PositionSnapshotService {
	return s.snapshotService
}

func (s *SettlementService) GetPnLAggregator() *PnLAggregator {
	return s.pnlAggregator
}
