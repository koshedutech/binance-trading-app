// Package exitdecision provides exit signal monitoring for the Chain Trading System.
package exitdecision

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ============================================================================
// EXIT MONITOR
// ============================================================================
// ExitMonitor handles the logic for checking positions against exit conditions.
// It evaluates TP/SL levels, trailing stops, and efficiency-based exits.

// ExitMonitor monitors positions for exit conditions.
type ExitMonitor struct {
	config     *Config
	safeguards *Safeguards
}

// NewExitMonitor creates a new exit monitor with the given configuration.
func NewExitMonitor(config *Config) *ExitMonitor {
	if config == nil {
		config = DefaultConfig()
	}

	var safeguards *Safeguards
	if config.Safeguards != nil {
		safeguards = NewSafeguards(config.Safeguards, config.DebugMode)
	} else {
		safeguards = NewSafeguards(DefaultSafeguardsConfig(), config.DebugMode)
	}

	return &ExitMonitor{
		config:     config,
		safeguards: safeguards,
	}
}

// GetSafeguards returns the safeguards instance for direct access.
func (em *ExitMonitor) GetSafeguards() *Safeguards {
	return em.safeguards
}

// CheckPosition evaluates a position for exit conditions.
// Returns an ExitSignal if any exit condition is met, nil otherwise.
func (em *ExitMonitor) CheckPosition(ctx context.Context, pos Position, currentPrice float64, userID string) (*ExitSignal, error) {
	if pos == nil {
		return nil, fmt.Errorf("position is nil")
	}

	symbol := pos.GetSymbol()
	if symbol == "" {
		return nil, fmt.Errorf("position symbol is empty")
	}

	// Check each exit condition in priority order
	// Priority: SL > TP > Trailing > Efficiency

	// 1. Check Stop Loss (highest priority - immediate exit)
	if em.config.EnableSLMonitoring {
		if signal := em.checkStopLoss(pos, currentPrice, userID); signal != nil {
			return signal, nil
		}
	}

	// 2. Check Take Profit levels
	if em.config.EnableTPMonitoring {
		if signal := em.checkTakeProfit(pos, currentPrice, userID); signal != nil {
			return signal, nil
		}
	}

	// 3. Check Trailing Stop
	if em.config.EnableTrailingMonitoring {
		if signal := em.checkTrailingStop(pos, currentPrice, userID); signal != nil {
			return signal, nil
		}
	}

	// 4. Check Efficiency-based exit
	if em.config.EnableEfficiencyMonitoring {
		if signal := em.checkEfficiencyExit(pos, currentPrice, userID); signal != nil {
			return signal, nil
		}
	}

	return nil, nil
}

// ============================================================================
// STOP LOSS CHECK
// ============================================================================

// checkStopLoss checks if the current price has hit the stop loss level.
func (em *ExitMonitor) checkStopLoss(pos Position, currentPrice float64, userID string) *ExitSignal {
	slPrice := pos.GetStopLossPrice()
	if slPrice <= 0 {
		return nil
	}

	side := pos.GetSide()
	entryPrice := pos.GetEntryPrice()

	// Calculate buffer (trigger slightly before actual SL)
	buffer := slPrice * (em.config.SLBufferPercent / 100)

	var triggered bool
	var distancePct float64

	if side == "LONG" || side == "long" {
		// For long positions, SL triggers when price falls below SL
		triggerPrice := slPrice + buffer
		triggered = currentPrice <= triggerPrice
		if slPrice > 0 {
			distancePct = ((currentPrice - slPrice) / slPrice) * 100
		}
	} else {
		// For short positions, SL triggers when price rises above SL
		triggerPrice := slPrice - buffer
		triggered = currentPrice >= triggerPrice
		if slPrice > 0 {
			distancePct = ((slPrice - currentPrice) / slPrice) * 100
		}
	}

	if triggered {
		pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)
		return &ExitSignal{
			Symbol:               pos.GetSymbol(),
			Mode:                 pos.GetMode(),
			Side:                 side,
			ExitType:             ExitTypeSL,
			TriggerPrice:         slPrice,
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			Urgency:              UrgencyImmediate,
			UnrealizedPnLPercent: pnlPct,
			DistanceToExitPct:    distancePct,
			Reason:               fmt.Sprintf("Stop loss triggered at %.4f (current: %.4f)", slPrice, currentPrice),
			Timestamp:            time.Now(),
			UserID:               userID,
		}
	}

	// Check if approaching SL (within 0.5%)
	if math.Abs(distancePct) < 0.5 {
		pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)
		return &ExitSignal{
			Symbol:               pos.GetSymbol(),
			Mode:                 pos.GetMode(),
			Side:                 side,
			ExitType:             ExitTypeSL,
			TriggerPrice:         slPrice,
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			Urgency:              UrgencyWarning,
			UnrealizedPnLPercent: pnlPct,
			DistanceToExitPct:    distancePct,
			Reason:               fmt.Sprintf("Approaching stop loss (%.2f%% away)", math.Abs(distancePct)),
			Timestamp:            time.Now(),
			UserID:               userID,
		}
	}

	return nil
}

// ============================================================================
// TAKE PROFIT CHECK
// ============================================================================

// checkTakeProfit checks if the current price has hit any take profit level.
func (em *ExitMonitor) checkTakeProfit(pos Position, currentPrice float64, userID string) *ExitSignal {
	tpLevels := pos.GetTakeProfitLevels()
	if len(tpLevels) == 0 {
		return nil
	}

	side := pos.GetSide()
	entryPrice := pos.GetEntryPrice()
	currentTPLevel := pos.GetCurrentTPLevel()

	// Check each TP level that hasn't been hit yet
	for _, tp := range tpLevels {
		// Skip already hit levels
		if tp.Level <= currentTPLevel || tp.Status == "hit" {
			continue
		}

		if tp.Price <= 0 {
			continue
		}

		// Calculate buffer (trigger slightly before actual TP)
		buffer := tp.Price * (em.config.TPBufferPercent / 100)

		var triggered bool
		var distancePct float64

		if side == "LONG" || side == "long" {
			// For long positions, TP triggers when price rises above TP
			triggerPrice := tp.Price - buffer
			triggered = currentPrice >= triggerPrice
			if tp.Price > 0 {
				distancePct = ((tp.Price - currentPrice) / tp.Price) * 100
			}
		} else {
			// For short positions, TP triggers when price falls below TP
			triggerPrice := tp.Price + buffer
			triggered = currentPrice <= triggerPrice
			if tp.Price > 0 {
				distancePct = ((currentPrice - tp.Price) / tp.Price) * 100
			}
		}

		if triggered {
			pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)
			return &ExitSignal{
				Symbol:               pos.GetSymbol(),
				Mode:                 pos.GetMode(),
				Side:                 side,
				ExitType:             ExitTypeTP,
				TPLevel:              tp.Level,
				TriggerPrice:         tp.Price,
				CurrentPrice:         currentPrice,
				EntryPrice:           entryPrice,
				Urgency:              UrgencyImmediate,
				UnrealizedPnLPercent: pnlPct,
				DistanceToExitPct:    distancePct,
				Reason:               fmt.Sprintf("Take profit level %d triggered at %.4f (current: %.4f)", tp.Level, tp.Price, currentPrice),
				Timestamp:            time.Now(),
				UserID:               userID,
			}
		}

		// Check if approaching TP (within 0.5%)
		if math.Abs(distancePct) < 0.5 {
			pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)
			return &ExitSignal{
				Symbol:               pos.GetSymbol(),
				Mode:                 pos.GetMode(),
				Side:                 side,
				ExitType:             ExitTypeTP,
				TPLevel:              tp.Level,
				TriggerPrice:         tp.Price,
				CurrentPrice:         currentPrice,
				EntryPrice:           entryPrice,
				Urgency:              UrgencyMonitor,
				UnrealizedPnLPercent: pnlPct,
				DistanceToExitPct:    distancePct,
				Reason:               fmt.Sprintf("Approaching take profit level %d (%.2f%% away)", tp.Level, math.Abs(distancePct)),
				Timestamp:            time.Now(),
				UserID:               userID,
			}
		}
	}

	return nil
}

// ============================================================================
// TRAILING STOP CHECK
// ============================================================================

// checkTrailingStop checks if the trailing stop has been triggered.
func (em *ExitMonitor) checkTrailingStop(pos Position, currentPrice float64, userID string) *ExitSignal {
	if !pos.IsTrailingActive() {
		return nil
	}

	side := pos.GetSide()
	entryPrice := pos.GetEntryPrice()
	trailingPct := pos.GetTrailingPercent()
	if trailingPct <= 0 {
		trailingPct = em.config.DefaultTrailingPercent
	}

	var trailingSLPrice float64
	var triggered bool
	var distancePct float64

	if side == "LONG" || side == "long" {
		// For long, trailing SL is below highest price
		highestPrice := pos.GetHighestPrice()
		if highestPrice <= 0 {
			highestPrice = currentPrice
		}
		trailingSLPrice = highestPrice * (1 - trailingPct/100)
		triggered = currentPrice <= trailingSLPrice
		distancePct = ((currentPrice - trailingSLPrice) / trailingSLPrice) * 100
	} else {
		// For short, trailing SL is above lowest price
		lowestPrice := pos.GetLowestPrice()
		if lowestPrice <= 0 {
			lowestPrice = currentPrice
		}
		trailingSLPrice = lowestPrice * (1 + trailingPct/100)
		triggered = currentPrice >= trailingSLPrice
		distancePct = ((trailingSLPrice - currentPrice) / trailingSLPrice) * 100
	}

	if triggered {
		pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)
		return &ExitSignal{
			Symbol:               pos.GetSymbol(),
			Mode:                 pos.GetMode(),
			Side:                 side,
			ExitType:             ExitTypeTrailing,
			TriggerPrice:         trailingSLPrice,
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			Urgency:              UrgencyImmediate,
			UnrealizedPnLPercent: pnlPct,
			DistanceToExitPct:    distancePct,
			Reason:               fmt.Sprintf("Trailing stop triggered at %.4f (%.2f%% from peak, current: %.4f)", trailingSLPrice, trailingPct, currentPrice),
			Timestamp:            time.Now(),
			UserID:               userID,
		}
	}

	// Check if approaching trailing SL (within 0.3%)
	if math.Abs(distancePct) < 0.3 {
		pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)
		return &ExitSignal{
			Symbol:               pos.GetSymbol(),
			Mode:                 pos.GetMode(),
			Side:                 side,
			ExitType:             ExitTypeTrailing,
			TriggerPrice:         trailingSLPrice,
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			Urgency:              UrgencyWarning,
			UnrealizedPnLPercent: pnlPct,
			DistanceToExitPct:    distancePct,
			Reason:               fmt.Sprintf("Approaching trailing stop (%.2f%% away)", math.Abs(distancePct)),
			Timestamp:            time.Now(),
			UserID:               userID,
		}
	}

	return nil
}

// ============================================================================
// EFFICIENCY EXIT CHECK
// ============================================================================

// checkEfficiencyExit checks if efficiency has dropped below threshold.
// This method now includes safeguard checks (Story 10.1 Phase 2).
func (em *ExitMonitor) checkEfficiencyExit(pos Position, currentPrice float64, userID string) *ExitSignal {
	if !pos.IsEfficiencyActive() {
		return nil
	}

	efficiency := pos.GetEfficiency()
	threshold := em.config.DefaultEfficiencyThreshold

	// Determine if efficiency is below threshold
	isBelowThreshold := efficiency < threshold && efficiency > 0

	// Apply safeguards if efficiency is below threshold
	if isBelowThreshold && em.safeguards != nil {
		safeguardResult := em.safeguards.CheckEfficiencyExitSafeguards(pos, currentPrice, isBelowThreshold)

		if !safeguardResult.Allowed {
			// Exit blocked by safeguards - return warning signal instead of immediate exit
			side := pos.GetSide()
			entryPrice := pos.GetEntryPrice()
			pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)

			// Build reason with safeguard info
			blockedBy := ""
			if len(safeguardResult.BlockedBy) > 0 {
				blockedBy = fmt.Sprintf(" [Blocked by: %v]", safeguardResult.BlockedBy)
			}

			return &ExitSignal{
				Symbol:               pos.GetSymbol(),
				Mode:                 pos.GetMode(),
				Side:                 side,
				ExitType:             ExitTypeEfficiency,
				TriggerPrice:         0,
				CurrentPrice:         currentPrice,
				EntryPrice:           entryPrice,
				Urgency:              UrgencyMonitor, // Downgraded from Immediate due to safeguards
				UnrealizedPnLPercent: pnlPct,
				DistanceToExitPct:    0,
				Reason:               fmt.Sprintf("Efficiency at %.1f%% (threshold: %.1f%%) - exit pending safeguard checks%s", efficiency*100, threshold*100, blockedBy),
				Timestamp:            time.Now(),
				UserID:               userID,
			}
		}
	}

	// Efficiency is currentProfit / peakProfit
	// When efficiency drops below threshold, we should exit to lock in remaining profit
	if isBelowThreshold {
		side := pos.GetSide()
		entryPrice := pos.GetEntryPrice()
		pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)

		return &ExitSignal{
			Symbol:               pos.GetSymbol(),
			Mode:                 pos.GetMode(),
			Side:                 side,
			ExitType:             ExitTypeEfficiency,
			TriggerPrice:         0, // Not price-based
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			Urgency:              UrgencyImmediate,
			UnrealizedPnLPercent: pnlPct,
			DistanceToExitPct:    0,
			Reason:               fmt.Sprintf("Efficiency dropped to %.1f%% (threshold: %.1f%%), peak profit: %.2f%%, current: %.2f%%", efficiency*100, threshold*100, pos.GetPeakProfit(), pos.GetCurrentProfit()),
			Timestamp:            time.Now(),
			UserID:               userID,
		}
	}

	// Check if approaching efficiency threshold (within 10% of threshold)
	warningThreshold := threshold + 0.1
	if efficiency < warningThreshold && efficiency >= threshold {
		side := pos.GetSide()
		entryPrice := pos.GetEntryPrice()
		pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)

		return &ExitSignal{
			Symbol:               pos.GetSymbol(),
			Mode:                 pos.GetMode(),
			Side:                 side,
			ExitType:             ExitTypeEfficiency,
			TriggerPrice:         0,
			CurrentPrice:         currentPrice,
			EntryPrice:           entryPrice,
			Urgency:              UrgencyWarning,
			UnrealizedPnLPercent: pnlPct,
			DistanceToExitPct:    0,
			Reason:               fmt.Sprintf("Efficiency approaching threshold (%.1f%% / %.1f%%)", efficiency*100, threshold*100),
			Timestamp:            time.Now(),
			UserID:               userID,
		}
	}

	return nil
}

// CheckEfficiencyExitWithSafeguards is a public method to check efficiency exit with full safeguard details.
// Returns the exit signal and safeguard result for transparency.
func (em *ExitMonitor) CheckEfficiencyExitWithSafeguards(pos Position, currentPrice float64, userID string) (*ExitSignal, *EfficiencyExitSafeguardResult) {
	if !pos.IsEfficiencyActive() || em.safeguards == nil {
		return em.checkEfficiencyExit(pos, currentPrice, userID), nil
	}

	efficiency := pos.GetEfficiency()
	threshold := em.config.DefaultEfficiencyThreshold
	isBelowThreshold := efficiency < threshold && efficiency > 0

	safeguardResult := em.safeguards.CheckEfficiencyExitSafeguards(pos, currentPrice, isBelowThreshold)
	signal := em.checkEfficiencyExit(pos, currentPrice, userID)

	return signal, safeguardResult
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// calculatePnLPercent calculates the unrealized P&L percentage.
func (em *ExitMonitor) calculatePnLPercent(entryPrice, currentPrice float64, side string) float64 {
	if entryPrice <= 0 {
		return 0
	}

	var pnlPct float64
	if side == "LONG" || side == "long" {
		pnlPct = ((currentPrice - entryPrice) / entryPrice) * 100
	} else {
		pnlPct = ((entryPrice - currentPrice) / entryPrice) * 100
	}

	return pnlPct
}

// ShouldActivateTrailing checks if trailing stop should be activated based on profit.
func (em *ExitMonitor) ShouldActivateTrailing(pos Position, currentPrice float64) bool {
	activationPct := pos.GetTrailingActivationPct()
	if activationPct <= 0 {
		activationPct = em.config.DefaultTrailingActivationPct
	}

	side := pos.GetSide()
	entryPrice := pos.GetEntryPrice()
	pnlPct := em.calculatePnLPercent(entryPrice, currentPrice, side)

	return pnlPct >= activationPct
}

// GetMonitoringStatus returns a summary of what's being monitored for a position.
func (em *ExitMonitor) GetMonitoringStatus(pos Position) map[string]interface{} {
	return map[string]interface{}{
		"tp_monitoring":         em.config.EnableTPMonitoring && len(pos.GetTakeProfitLevels()) > 0,
		"sl_monitoring":         em.config.EnableSLMonitoring && pos.GetStopLossPrice() > 0,
		"trailing_monitoring":   em.config.EnableTrailingMonitoring && pos.IsTrailingActive(),
		"efficiency_monitoring": em.config.EnableEfficiencyMonitoring && pos.IsEfficiencyActive(),
		"current_tp_level":      pos.GetCurrentTPLevel(),
		"tp_levels_count":       len(pos.GetTakeProfitLevels()),
		"stop_loss_price":       pos.GetStopLossPrice(),
		"trailing_active":       pos.IsTrailingActive(),
		"trailing_percent":      pos.GetTrailingPercent(),
		"efficiency_active":     pos.IsEfficiencyActive(),
	}
}
