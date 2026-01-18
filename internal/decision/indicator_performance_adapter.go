// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.14: Indicator Performance Tracker
package decision

import (
	"context"
	"time"
)

// IndicatorPerformanceAdapter adapts the IndicatorPerformanceTracker for use in autopilot.
// It implements the autopilot.IndicatorPerformanceRecorder interface using individual
// parameters to avoid circular import issues with autopilot types.
type IndicatorPerformanceAdapter struct {
	tracker *IndicatorPerformanceTracker
}

// NewIndicatorPerformanceAdapter creates a new adapter wrapping the tracker.
func NewIndicatorPerformanceAdapter(tracker *IndicatorPerformanceTracker) *IndicatorPerformanceAdapter {
	return &IndicatorPerformanceAdapter{tracker: tracker}
}

// GetTracker returns the underlying tracker for direct access (e.g., for API handlers).
func (a *IndicatorPerformanceAdapter) GetTracker() *IndicatorPerformanceTracker {
	return a.tracker
}

// RecordTradeEntryAutopilot logs indicator values when a trade is opened.
// This implements the autopilot.IndicatorPerformanceRecorder interface.
// Parameters are passed individually to avoid circular import with autopilot types.
func (a *IndicatorPerformanceAdapter) RecordTradeEntryAutopilot(
	ctx context.Context,
	tradeID string,
	userID int,
	symbol, strategy string,
	indicators map[string]float64,
	entryScore int,
	entryTime time.Time,
	marketRegime, side string,
) error {
	if a.tracker == nil {
		return nil
	}

	// Convert to internal type
	snapshot := &TradeIndicatorSnapshot{
		TradeID:      tradeID,
		UserID:       userID,
		Symbol:       symbol,
		Strategy:     strategy,
		Indicators:   indicators,
		EntryScore:   entryScore,
		EntryTime:    entryTime,
		MarketRegime: marketRegime,
		Side:         side,
	}

	return a.tracker.RecordTradeEntry(ctx, snapshot)
}

// RecordTradeExitAutopilot records the trade outcome and updates performance stats.
// This implements the autopilot.IndicatorPerformanceRecorder interface.
func (a *IndicatorPerformanceAdapter) RecordTradeExitAutopilot(
	ctx context.Context,
	tradeID string,
	isWin bool,
	pnlPercent float64,
	exitTime time.Time,
	exitReason string,
) error {
	if a.tracker == nil {
		return nil
	}

	// Convert to internal type
	outcome := &TradeOutcome{
		TradeID:    tradeID,
		IsWin:      isWin,
		PnLPercent: pnlPercent,
		ExitTime:   exitTime,
		ExitReason: exitReason,
	}

	return a.tracker.RecordTradeExit(ctx, outcome)
}
