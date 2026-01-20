// Package autopilot provides position efficiency tracking and breakeven calculation.
// Epic 10 / Story 10.1: Position Management Optimization - Efficiency-Based Exit
//
// This file implements:
// - Breakeven price calculation (entry price + fees + buffer)
// - Efficiency tracking (currentProfit / peakProfit)
// - Position stage management (RISK_ZONE -> BREAKEVEN -> TP1_PENDING -> EFFICIENCY)
//
// The efficiency formula is simple:
//   EFFICIENCY = currentProfit / peakProfit
//
// Exit when efficiency drops below the historical baseline threshold.
// This captures profit before it evaporates while letting winners run.
package autopilot

import (
	"time"
)

// Position Stage Constants
// Positions progress through these stages as they move into profit
const (
	// StageRiskZone: Position is in loss or minimal profit (below breakeven)
	// No efficiency tracking active, use normal SL protection
	StageRiskZone = "RISK_ZONE"

	// StageBreakeven: Position has reached breakeven price
	// SL can be moved to breakeven to lock in zero loss
	StageBreakeven = "BREAKEVEN"

	// StageTP1Pending: Position is profitable, waiting for TP1 hit
	// Some profit locked, but efficiency tracking not yet active
	StageTP1Pending = "TP1_PENDING"

	// StageEfficiency: TP1 has been hit, efficiency tracking is active
	// Monitor efficiency and exit when it drops below threshold
	StageEfficiency = "EFFICIENCY"
)

// Default fee percentages for breakeven calculation
const (
	// DefaultTakerFeePercent is the default taker fee (0.05% = 0.0005)
	DefaultTakerFeePercent = 0.0005

	// DefaultMakerFeePercent is the default maker fee (0.02% = 0.0002)
	DefaultMakerFeePercent = 0.0002

	// DefaultBreakevenBuffer is extra buffer above breakeven (0.1% = 0.001)
	// This accounts for slippage and ensures true breakeven
	DefaultBreakevenBuffer = 0.001
)

// CalculateBreakevenPrice calculates the breakeven price for a position.
// Breakeven = Entry Price adjusted for:
// - Entry fee (already paid)
// - Expected exit fee (will be paid on close)
// - Buffer for slippage
//
// For LONG positions: BE = entryPrice * (1 + totalFees + buffer)
// For SHORT positions: BE = entryPrice * (1 - totalFees - buffer)
//
// Parameters:
// - entryPrice: The entry price of the position
// - side: "LONG" or "SHORT"
// - entryFeePercent: Fee percentage for entry (e.g., 0.0005 for 0.05%)
// - exitFeePercent: Expected fee percentage for exit
// - bufferPercent: Additional buffer for slippage (e.g., 0.001 for 0.1%)
//
// Returns the breakeven price that accounts for all fees
func CalculateBreakevenPrice(entryPrice float64, side string, entryFeePercent, exitFeePercent, bufferPercent float64) float64 {
	// Total cost = entry fee + exit fee + buffer
	totalCost := entryFeePercent + exitFeePercent + bufferPercent

	if side == "LONG" {
		// For LONG: price must go UP to cover fees
		// BE = entry * (1 + totalCost)
		return entryPrice * (1 + totalCost)
	}

	// For SHORT: price must go DOWN to cover fees
	// BE = entry * (1 - totalCost)
	return entryPrice * (1 - totalCost)
}

// CalculateBreakevenPriceSimple calculates breakeven with default fee assumptions.
// Uses taker fee for both entry and exit (conservative estimate).
//
// Parameters:
// - entryPrice: The entry price of the position
// - side: "LONG" or "SHORT"
//
// Returns the breakeven price using default fee percentages
func CalculateBreakevenPriceSimple(entryPrice float64, side string) float64 {
	return CalculateBreakevenPrice(
		entryPrice,
		side,
		DefaultTakerFeePercent,
		DefaultTakerFeePercent,
		DefaultBreakevenBuffer,
	)
}

// CalculateProfitPercent calculates the current profit percentage of a position.
//
// For LONG: profit% = (currentPrice - entryPrice) / entryPrice * 100
// For SHORT: profit% = (entryPrice - currentPrice) / entryPrice * 100
//
// Parameters:
// - entryPrice: The entry price of the position
// - currentPrice: The current market price
// - side: "LONG" or "SHORT"
//
// Returns the profit percentage (positive = profit, negative = loss)
func CalculateProfitPercent(entryPrice, currentPrice float64, side string) float64 {
	if entryPrice <= 0 {
		return 0
	}

	if side == "LONG" {
		return (currentPrice - entryPrice) / entryPrice * 100
	}

	// SHORT
	return (entryPrice - currentPrice) / entryPrice * 100
}

// CalculateEfficiency calculates the efficiency ratio.
//
// EFFICIENCY = currentProfit / peakProfit
//
// This measures how much of the peak profit is still captured.
// - Efficiency = 1.0 means we're at the peak
// - Efficiency = 0.5 means we've given back 50% of peak profit
// - Efficiency < 0 means we've given back all profit and are in loss
//
// Parameters:
// - currentProfit: Current profit percentage
// - peakProfit: Highest profit percentage achieved
//
// Returns efficiency ratio (0-1.0 in profitable scenarios)
func CalculateEfficiency(currentProfit, peakProfit float64) float64 {
	// If peak is zero or negative, efficiency is undefined
	// Return 1.0 (at peak) to prevent false triggers
	if peakProfit <= 0 {
		return 1.0
	}

	// If current profit is negative but peak was positive,
	// we've given back all profit and more - efficiency < 0
	efficiency := currentProfit / peakProfit

	// Cap at 1.0 (can't be more than 100% efficient)
	if efficiency > 1.0 {
		return 1.0
	}

	return efficiency
}

// UpdatePositionEfficiency updates the efficiency tracking fields on a position.
// This should be called on every price tick while the position is open.
//
// The function:
// 1. Calculates current profit %
// 2. Updates peak profit if we have a new high
// 3. Calculates current efficiency ratio
// 4. Updates position stage based on current state
//
// Parameters:
// - pos: The position to update (modified in place)
// - currentPrice: The current market price
//
// Note: This function only updates tracking fields. It does NOT trigger exits.
// The efficiency exit logic should check pos.Efficiency against the baseline threshold.
func UpdatePositionEfficiency(pos *GiniePosition, currentPrice float64) {
	if pos == nil || pos.EntryPrice <= 0 {
		return
	}

	// Calculate current profit percentage
	pos.CurrentProfit = CalculateProfitPercent(pos.EntryPrice, currentPrice, pos.Side)

	// Initialize BE price if not set
	if pos.BEPrice == 0 {
		pos.BEPrice = CalculateBreakevenPriceSimple(pos.EntryPrice, pos.Side)
	}

	// Check if breakeven has been achieved
	if !pos.BEAchieved {
		beAchieved := false
		if pos.Side == "LONG" {
			beAchieved = currentPrice >= pos.BEPrice
		} else {
			beAchieved = currentPrice <= pos.BEPrice
		}

		if beAchieved {
			pos.BEAchieved = true
			pos.BETime = time.Now()
		}
	}

	// Update peak profit tracking (only when efficiency tracking is active)
	if pos.EffActive {
		if pos.CurrentProfit > pos.PeakProfit {
			pos.PeakProfit = pos.CurrentProfit
			pos.PeakPrice = currentPrice
			pos.PeakTime = time.Now()
		}

		// Calculate efficiency
		pos.Efficiency = CalculateEfficiency(pos.CurrentProfit, pos.PeakProfit)
	}

	// Update position stage
	updatePositionStage(pos, currentPrice)
}

// updatePositionStage determines the current stage of the position.
// Stages progress: RISK_ZONE -> BREAKEVEN -> TP1_PENDING -> EFFICIENCY
func updatePositionStage(pos *GiniePosition, currentPrice float64) {
	// If efficiency tracking is already active, we're in EFFICIENCY stage
	if pos.EffActive {
		pos.Stage = StageEfficiency
		return
	}

	// Check if TP1 has been hit (CurrentTPLevel >= 1)
	if pos.CurrentTPLevel >= 1 {
		// TP1 hit - activate efficiency tracking
		pos.Stage = StageTP1Pending
		// Note: Efficiency tracking should be activated separately
		// when we want to start monitoring for efficiency exits
		return
	}

	// Check if breakeven achieved
	if pos.BEAchieved {
		pos.Stage = StageBreakeven
		return
	}

	// Default: still in risk zone
	pos.Stage = StageRiskZone
}

// ActivateEfficiencyTracking activates efficiency tracking for a position.
// This should be called after TP1 has been hit and we want to start
// monitoring efficiency for exit decisions.
//
// Parameters:
// - pos: The position to activate (modified in place)
// - currentPrice: The current market price
//
// The function initializes peak profit tracking from the current state.
func ActivateEfficiencyTracking(pos *GiniePosition, currentPrice float64) {
	if pos == nil || pos.EffActive {
		return
	}

	// Calculate initial profit
	currentProfit := CalculateProfitPercent(pos.EntryPrice, currentPrice, pos.Side)

	// Initialize efficiency tracking
	pos.EffActive = true
	pos.PeakProfit = currentProfit
	pos.PeakPrice = currentPrice
	pos.PeakTime = time.Now()
	pos.CurrentProfit = currentProfit
	pos.Efficiency = 1.0 // At peak initially
	pos.Stage = StageEfficiency
}

// ShouldExitOnEfficiency checks if a position should exit based on efficiency.
// This is a simple comparison against the threshold.
//
// Parameters:
// - pos: The position to check
// - threshold: The efficiency threshold (e.g., 0.50 for 50%)
//
// Returns true if efficiency has dropped below threshold and we should exit.
func ShouldExitOnEfficiency(pos *GiniePosition, threshold float64) bool {
	// Only check if efficiency tracking is active
	if pos == nil || !pos.EffActive {
		return false
	}

	// Must have achieved some peak profit first
	if pos.PeakProfit <= 0 {
		return false
	}

	// Exit if efficiency drops below threshold
	return pos.Efficiency < threshold
}

// GetEfficiencyExitInfo returns formatted info about potential efficiency exit.
// Useful for logging and debugging.
func GetEfficiencyExitInfo(pos *GiniePosition) map[string]interface{} {
	if pos == nil {
		return nil
	}

	return map[string]interface{}{
		"symbol":         pos.Symbol,
		"side":           pos.Side,
		"mode":           pos.Mode,
		"stage":          pos.Stage,
		"entry_price":    pos.EntryPrice,
		"be_price":       pos.BEPrice,
		"be_achieved":    pos.BEAchieved,
		"eff_active":     pos.EffActive,
		"current_profit": pos.CurrentProfit,
		"peak_profit":    pos.PeakProfit,
		"efficiency":     pos.Efficiency,
		"tp_level":       pos.CurrentTPLevel,
	}
}
