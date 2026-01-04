package autopilot

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"binance-trading-bot/internal/binance"
)

// ============ SCALP RE-ENTRY CORE LOGIC ============

// initScalpReentry initializes scalp re-entry status for a position
func (g *GinieAutopilot) initScalpReentry(pos *GiniePosition) *ScalpReentryStatus {
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig

	status := NewScalpReentryStatus(pos.EntryPrice, pos.OriginalQty, config)
	status.AddDebugLog(fmt.Sprintf("Initialized scalp_reentry for %s %s @ %.8f", pos.Symbol, pos.Side, pos.EntryPrice))

	return status
}

// monitorScalpReentryPosition monitors a position in scalp_reentry mode
func (g *GinieAutopilot) monitorScalpReentryPosition(pos *GiniePosition) error {
	if pos.ScalpReentry == nil {
		pos.ScalpReentry = g.initScalpReentry(pos)
	}

	sr := pos.ScalpReentry
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	currentPrice := g.getCurrentPrice(pos.Symbol)

	// Log monitoring
	sr.AddDebugLog(fmt.Sprintf("Monitoring: price=%.8f, tpUnlocked=%d, blocked=%v, cycles=%d",
		currentPrice, sr.TPLevelUnlocked, sr.NextTPBlocked, len(sr.Cycles)))

	// Step 0: Check if exchange LIMIT TP has filled (qty decreased from user data stream)
	// Exchange-based LIMIT TPs save on fees compared to internal MARKET orders
	if g.detectExchangeTPFill(pos, currentPrice) {
		// Exchange TP filled - state already updated, proceed with re-entry logic
		return nil
	}

	// Step 1: Check if we're in final portion mode (after 80% sold at 1%)
	if sr.FinalPortionActive {
		return g.monitorFinalTrailing(pos, currentPrice)
	}

	// Step 2: Check if we're waiting for a re-entry
	if sr.NextTPBlocked && sr.IsWaitingForReentry() {
		return g.checkAndExecuteReentry(pos, currentPrice)
	}

	// Step 3: Check if TP level is reached but exchange order didn't fill
	// This is a fallback - normally exchange LIMIT TP should fill first
	// Only execute if exchange TP hasn't processed this level yet
	if !sr.NextTPBlocked || sr.CanProceedToNextTP() {
		nextTPLevel := sr.TPLevelUnlocked + 1
		if nextTPLevel <= 3 {
			tpHit, _ := g.checkScalpReentryTP(pos, currentPrice, nextTPLevel)
			if tpHit {
				// Check if position qty already reduced (exchange filled)
				expectedQtyAfterTP := sr.RemainingQuantity
				actualQty := pos.RemainingQty
				if actualQty < expectedQtyAfterTP*0.95 {
					// Exchange already filled - just update state
					return g.processExchangeTPFill(pos, nextTPLevel, currentPrice)
				}
				// Exchange didn't fill - use internal execution as fallback
				log.Printf("[SCALP-REENTRY] %s: TP%d hit but exchange order not filled, using internal execution", pos.Symbol, nextTPLevel)
				return g.executeTPSell(pos, nextTPLevel)
			}
		}
	}

	// Step 4: If dynamic SL is active, update it
	if sr.DynamicSLActive {
		return g.updateDynamicSL(pos, currentPrice, config)
	}

	return nil
}

// detectExchangeTPFill checks if exchange LIMIT TP order has filled
// Returns true if a TP fill was detected and processed
func (g *GinieAutopilot) detectExchangeTPFill(pos *GiniePosition, currentPrice float64) bool {
	sr := pos.ScalpReentry
	if sr == nil {
		return false
	}

	// Compare actual position qty with our tracked qty
	actualQty := pos.RemainingQty
	expectedQty := sr.RemainingQuantity

	// If actual qty is significantly less than expected, exchange TP must have filled
	// Use 5% tolerance to account for rounding
	if actualQty >= expectedQty*0.95 {
		return false // No reduction detected
	}

	// Calculate which TP level was hit based on qty reduction
	nextTPLevel := sr.TPLevelUnlocked + 1
	if nextTPLevel > 3 {
		return false // All TPs already hit
	}

	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	_, sellPercent := config.GetTPConfig(nextTPLevel)
	sellPercentExact := float64(int(sellPercent)) / 100.0
	expectedSellQty := expectedQty * sellPercentExact
	actualReduction := expectedQty - actualQty

	// Check if reduction matches expected TP sell qty (within 20% tolerance)
	if actualReduction >= expectedSellQty*0.8 && actualReduction <= expectedSellQty*1.2 {
		sr.AddDebugLog(fmt.Sprintf("Exchange TP%d filled: qty reduced %.4f -> %.4f (sold %.4f)",
			nextTPLevel, expectedQty, actualQty, actualReduction))
		return g.processExchangeTPFill(pos, nextTPLevel, currentPrice) == nil
	}

	return false
}

// processExchangeTPFill updates state after exchange LIMIT TP order fills
// This is called instead of executeTPSell since exchange already executed the order
func (g *GinieAutopilot) processExchangeTPFill(pos *GiniePosition, tpLevel int, currentPrice float64) error {
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	sr := pos.ScalpReentry

	tpPercent, sellPercent := config.GetTPConfig(tpLevel)
	sellPercentExact := float64(int(sellPercent)) / 100.0
	sellQty := sr.RemainingQuantity * sellPercentExact
	sellQty = g.roundQuantity(pos.Symbol, sellQty)

	sr.AddDebugLog(fmt.Sprintf("TP%d EXCHANGE FILL: profit %.2f%%, sold %.4f (%.0f%% of remaining)",
		tpLevel, tpPercent, sellQty, sellPercent))
	log.Printf("[SCALP-REENTRY] %s: Exchange LIMIT TP%d filled (saved taker fees!)", pos.Symbol, tpLevel)

	// Calculate PnL for this sell
	var pnl float64
	if pos.Side == "LONG" {
		pnl = (currentPrice - pos.EntryPrice) * sellQty
	} else {
		pnl = (pos.EntryPrice - currentPrice) * sellQty
	}

	// Calculate PnL percent for logging
	pnlPercent := 0.0
	if pos.EntryPrice > 0 {
		if pos.Side == "LONG" {
			pnlPercent = ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		} else {
			pnlPercent = ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
		}
	}

	// LOG TP HIT TO DATABASE - critical for tracking scalp_reentry TPs
	go g.eventLogger.LogTPHit(
		context.Background(),
		pos.FuturesTradeID,
		pos.Symbol,
		tpLevel,
		currentPrice,
		sellQty,
		pnl,
		pnlPercent,
	)

	// Create new cycle
	cycle := ReentryCycle{
		CycleNumber:  len(sr.Cycles) + 1,
		TPLevel:      tpLevel,
		Mode:         string(GinieModeScalpReentry),
		Side:         pos.Side,
		SellPrice:    currentPrice,
		SellQuantity: sellQty,
		SellPnL:      pnl,
		SellTime:     time.Now(),
		StartTime:    time.Now(),
		ReentryState: ReentryStateNone,
	}

	// Update position state - use actual qty from exchange
	sr.RemainingQuantity = pos.RemainingQty // Trust exchange qty
	sr.AccumulatedProfit += pnl
	sr.TPLevelUnlocked = tpLevel

	// Sync main position TP state
	pos.CurrentTPLevel = tpLevel
	if tpLevel > 0 && tpLevel <= len(pos.TakeProfits) {
		pos.TakeProfits[tpLevel-1].Status = "hit"
	}

	// Handle position fully closed
	if sr.RemainingQuantity <= 0 {
		sr.AddDebugLog(fmt.Sprintf("TP%d: Position fully closed via exchange. Total profit: %.4f", tpLevel, sr.AccumulatedProfit))
		cycle.ReentryState = ReentryStateNone
		cycle.Outcome = "full_close_exchange"
		cycle.OutcomeReason = "exchange_tp_filled_all"
		cycle.EndTime = time.Now()
		sr.Cycles = append(sr.Cycles, cycle)
		sr.TotalCyclesCompleted++
		sr.TotalCyclePnL = sr.AccumulatedProfit
		sr.LastUpdate = time.Now()
		pos.IsClosing = true
		go g.SavePositionState()
		return nil
	}

	// Handle TP3 (1%) - activate dynamic SL and final trailing
	if tpLevel == 3 {
		sr.FinalPortionActive = true
		sr.FinalPortionQty = sr.RemainingQuantity
		sr.FinalTrailingPeak = currentPrice
		sr.FinalTrailingPercent = config.FinalTrailingPercent
		sr.DynamicSLActive = true
		sr.ProtectedProfit = sr.AccumulatedProfit * (config.DynamicSLProtectPct / 100)
		sr.MaxAllowableLoss = sr.AccumulatedProfit * (config.DynamicSLMaxLossPct / 100)
		cycle.ReentryState = ReentryStateNone
		sr.AddDebugLog(fmt.Sprintf("TP3 exchange fill! Final portion mode. Qty=%.4f, Trailing=%.1f%%",
			sr.FinalPortionQty, sr.FinalTrailingPercent))
	} else {
		// For TP1/TP2, set up re-entry and place next TP order
		reentryTargetPrice := roundPrice(pos.Symbol, sr.CurrentBreakeven)
		cycle.ReentryTargetPrice = reentryTargetPrice
		reentryPercent := float64(int(config.ReentryPercent)) / 100.0
		cycle.ReentryQuantity = sellQty * reentryPercent
		cycle.ReentryState = ReentryStateWaiting
		sr.NextTPBlocked = true
		sr.AddDebugLog(fmt.Sprintf("Waiting for re-entry at breakeven %.8f", reentryTargetPrice))

		// Place next TP order on exchange for next level
		// Note: placeNextTPOrder takes the CURRENT level and places order for level+1
		if tpLevel < 3 && tpLevel < len(pos.TakeProfits) {
			log.Printf("[SCALP-REENTRY] %s: Placing next TP order for level %d", pos.Symbol, tpLevel+1)
			go g.placeNextTPOrder(pos, tpLevel)
		}
	}

	sr.Cycles = append(sr.Cycles, cycle)
	sr.CurrentCycle = len(sr.Cycles)
	sr.LastUpdate = time.Now()
	go g.SavePositionState()

	return nil
}

// checkScalpReentryTP checks if a TP level has been reached
func (g *GinieAutopilot) checkScalpReentryTP(pos *GiniePosition, currentPrice float64, tpLevel int) (bool, float64) {
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	tpPercent, _ := config.GetTPConfig(tpLevel)

	if tpPercent == 0 {
		return false, 0
	}

	// Calculate TP price based on side
	var tpPrice float64
	if pos.Side == "LONG" {
		tpPrice = pos.EntryPrice * (1 + tpPercent/100)
		// Round TP price for proper precision comparison
		tpPrice = roundPriceForTP(pos.Symbol, tpPrice, pos.Side)
		return currentPrice >= tpPrice, tpPrice
	} else {
		tpPrice = pos.EntryPrice * (1 - tpPercent/100)
		// Round TP price for proper precision comparison
		tpPrice = roundPriceForTP(pos.Symbol, tpPrice, pos.Side)
		return currentPrice <= tpPrice, tpPrice
	}
}

// executeTPSell executes a partial sell at a TP level
func (g *GinieAutopilot) executeTPSell(pos *GiniePosition, tpLevel int) error {
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	sr := pos.ScalpReentry
	currentPrice := g.getCurrentPrice(pos.Symbol)

	tpPercent, sellPercent := config.GetTPConfig(tpLevel)

	sr.AddDebugLog(fmt.Sprintf("TP%d: Target profit %.2f%%, sell %.0f%% of position", tpLevel, tpPercent, sellPercent))

	// Calculate quantity to sell
	// Use integer percentage to avoid floating point errors: int(30) / 100.0 = 0.3 exactly
	sellPercentExact := float64(int(sellPercent)) / 100.0
	var sellQty float64
	if tpLevel == 3 {
		// At TP3 (1%), sell 80% of remaining, keep 20%
		sellQty = sr.RemainingQuantity * sellPercentExact
	} else {
		// At TP1/TP2, sell configured percentage
		sellQty = sr.RemainingQuantity * sellPercentExact
	}

	// Round quantity to appropriate precision
	sellQty = g.roundQuantity(pos.Symbol, sellQty)
	minQty := g.getMinQuantity(pos.Symbol)

	// Handle small positions: if partial sell is below minimum, close entire remaining
	if sellQty < minQty {
		remainingQty := g.roundQuantity(pos.Symbol, sr.RemainingQuantity)
		if remainingQty >= minQty {
			// Position is small - close 100% instead of partial
			sr.AddDebugLog(fmt.Sprintf("TP%d: Partial qty %.4f below min, closing 100%% (%.4f) instead", tpLevel, sellQty, remainingQty))
			sellQty = remainingQty
			sellPercent = 100.0
		} else {
			// ALERT: Position is stuck - too small to execute any trade
			alertMsg := fmt.Sprintf("TP%d: Position too small (%.4f < min %.4f) - NEEDS MANUAL INTERVENTION", tpLevel, remainingQty, minQty)
			sr.AddDebugLog(alertMsg)

			// Set visible alert flags for UI
			sr.NeedsManualIntervention = true
			sr.ManualInterventionReason = fmt.Sprintf("Position quantity %.6f is below minimum tradeable %.6f. Cannot execute TP%d sell. Please close manually.", remainingQty, minQty, tpLevel)
			sr.ManualInterventionAlertAt = time.Now().Format(time.RFC3339)
			sr.LastUpdate = time.Now()

			// Log prominently for visibility
			log.Printf("[STUCK-POSITION-ALERT] %s %s: %s (remaining: %.6f, min: %.6f, entry: %.4f)",
				pos.Symbol, pos.Side, alertMsg, remainingQty, minQty, pos.EntryPrice)

			return nil
		}
	}

	sr.AddDebugLog(fmt.Sprintf("TP%d hit at %.8f! Selling %.4f (%.0f%% of remaining)",
		tpLevel, currentPrice, sellQty, sellPercent))

	// Calculate PnL for this sell
	var pnl float64
	if pos.Side == "LONG" {
		pnl = (currentPrice - pos.EntryPrice) * sellQty
	} else {
		pnl = (pos.EntryPrice - currentPrice) * sellQty
	}

	// Create new cycle
	cycle := ReentryCycle{
		CycleNumber: len(sr.Cycles) + 1,
		TPLevel:     tpLevel,
		Mode:        string(GinieModeScalpReentry),
		Side:        pos.Side,
		SellPrice:   currentPrice,
		SellQuantity: sellQty,
		SellPnL:     pnl,
		SellTime:    time.Now(),
		StartTime:   time.Now(),
		ReentryState: ReentryStateNone,
	}

	// Execute the partial close
	err := g.executeScalpPartialClose(pos, sellQty, fmt.Sprintf("scalp_reentry_TP%d", tpLevel))
	if err != nil {
		sr.AddDebugLog(fmt.Sprintf("TP%d sell failed: %v", tpLevel, err))
		return err
	}

	// Calculate PnL percent for logging
	pnlPercent := 0.0
	if pos.EntryPrice > 0 {
		if pos.Side == "LONG" {
			pnlPercent = ((currentPrice - pos.EntryPrice) / pos.EntryPrice) * 100
		} else {
			pnlPercent = ((pos.EntryPrice - currentPrice) / pos.EntryPrice) * 100
		}
	}

	// LOG TP HIT TO DATABASE - critical for tracking scalp_reentry TPs
	go g.eventLogger.LogTPHit(
		context.Background(),
		pos.FuturesTradeID,
		pos.Symbol,
		tpLevel,
		currentPrice,
		sellQty,
		pnl,
		pnlPercent,
	)

	// Update position state
	sr.RemainingQuantity -= sellQty
	sr.AccumulatedProfit += pnl
	sr.TPLevelUnlocked = tpLevel

	// CRITICAL: Sync pos.RemainingQty with sr.RemainingQuantity to avoid divergence
	// sr.RemainingQuantity is the source of truth for scalp_reentry mode
	pos.RemainingQty = sr.RemainingQuantity

	// CRITICAL: Sync main position TP state with scalp_reentry
	// This prevents protection system from trying to re-place TP orders
	pos.CurrentTPLevel = tpLevel
	if tpLevel > 0 && tpLevel <= len(pos.TakeProfits) {
		pos.TakeProfits[tpLevel-1].Status = "hit"
	}

	// Handle small position full close - no re-entry when position fully closed
	if sr.RemainingQuantity <= 0 || sellPercent >= 100.0 {
		sr.AddDebugLog(fmt.Sprintf("TP%d: Position fully closed (small position handling). Total profit: %.4f", tpLevel, sr.AccumulatedProfit))
		cycle.ReentryState = ReentryStateNone
		cycle.Outcome = "full_close_small_position"
		cycle.OutcomeReason = "position_below_min_qty"
		cycle.EndTime = time.Now()
		sr.Cycles = append(sr.Cycles, cycle)
		sr.TotalCyclesCompleted++
		sr.TotalCyclePnL = sr.AccumulatedProfit
		sr.LastUpdate = time.Now()
		pos.IsClosing = true
		go g.SavePositionState()
		return nil
	}

	// Handle TP3 (1%) - activate dynamic SL and final trailing
	if tpLevel == 3 {
		sr.FinalPortionActive = true
		sr.FinalPortionQty = sr.RemainingQuantity
		sr.FinalTrailingPeak = currentPrice
		sr.FinalTrailingPercent = config.FinalTrailingPercent

		// Activate dynamic SL
		sr.DynamicSLActive = true
		sr.ProtectedProfit = sr.AccumulatedProfit * (config.DynamicSLProtectPct / 100)
		sr.MaxAllowableLoss = sr.AccumulatedProfit * (config.DynamicSLMaxLossPct / 100)

		cycle.ReentryState = ReentryStateNone // No re-entry after TP3
		sr.AddDebugLog(fmt.Sprintf("TP3 hit! Final portion mode activated. Qty=%.4f, Trailing=%.1f%%, Dynamic SL active",
			sr.FinalPortionQty, sr.FinalTrailingPercent))
	} else {
		// For TP1/TP2, set up re-entry
		// Round the reentry target price for proper precision
		reentryTargetPrice := roundPrice(pos.Symbol, sr.CurrentBreakeven)
		cycle.ReentryTargetPrice = reentryTargetPrice
		// Use integer percentage to avoid floating point errors: int(80) / 100.0 = 0.8
		reentryPercent := float64(int(config.ReentryPercent)) / 100.0
		cycle.ReentryQuantity = sellQty * reentryPercent
		cycle.ReentryState = ReentryStateWaiting

		sr.NextTPBlocked = true
		sr.AddDebugLog(fmt.Sprintf("Waiting for re-entry at breakeven %.8f, target qty %.4f",
			cycle.ReentryTargetPrice, cycle.ReentryQuantity))
	}

	sr.Cycles = append(sr.Cycles, cycle)
	sr.CurrentCycle = len(sr.Cycles)
	sr.LastUpdate = time.Now()

	// CRITICAL: Save position state after TP hit to survive restarts
	// This ensures scalp_reentry doesn't reset to TP1 on refresh/restart
	go g.SavePositionState()

	return nil
}

// checkAndExecuteReentry checks if re-entry conditions are met and executes
func (g *GinieAutopilot) checkAndExecuteReentry(pos *GiniePosition, currentPrice float64) error {
	sr := pos.ScalpReentry
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	cycle := sr.GetCurrentCycle()

	if cycle == nil || cycle.ReentryState != ReentryStateWaiting {
		return nil
	}

	// Check if price is near breakeven
	reentryTargetPrice := cycle.ReentryTargetPrice
	bufferPercent := config.ReentryPriceBuffer / 100

	var withinBuffer bool
	if pos.Side == "LONG" {
		// For LONG, price should come down to or near breakeven
		upperBound := reentryTargetPrice * (1 + bufferPercent)
		lowerBound := reentryTargetPrice * (1 - bufferPercent)
		withinBuffer = currentPrice >= lowerBound && currentPrice <= upperBound
	} else {
		// For SHORT, price should come up to or near breakeven
		upperBound := reentryTargetPrice * (1 + bufferPercent)
		lowerBound := reentryTargetPrice * (1 - bufferPercent)
		withinBuffer = currentPrice >= lowerBound && currentPrice <= upperBound
	}

	if !withinBuffer {
		// Not at breakeven yet - check for timeout
		if time.Since(cycle.StartTime) > time.Duration(config.ReentryTimeoutSec)*time.Second {
			sr.AddDebugLog(fmt.Sprintf("Re-entry timeout after %ds, skipping", config.ReentryTimeoutSec))
			cycle.ReentryState = ReentryStateSkipped
			cycle.EndTime = time.Now()
			cycle.Outcome = "skipped"
			cycle.OutcomeReason = "timeout"
			sr.SkippedReentries++
			sr.NextTPBlocked = false
			return nil
		}
		return nil
	}

	// Price is at breakeven - get AI decision if enabled
	if config.UseAIDecisions {
		shouldReenter, aiDecision := g.getAIReentryDecision(pos, cycle)
		cycle.AIDecision = aiDecision

		if !shouldReenter {
			sr.AddDebugLog(fmt.Sprintf("AI decided to skip re-entry: %s", aiDecision.Reasoning))
			cycle.ReentryState = ReentryStateSkipped
			cycle.EndTime = time.Now()
			cycle.Outcome = "skipped"
			cycle.OutcomeReason = "ai_decision"
			sr.SkippedReentries++
			sr.NextTPBlocked = false
			return nil
		}

		// Adjust quantity based on AI recommendation
		if aiDecision.RecommendedQtyPct > 0 && aiDecision.RecommendedQtyPct < 1.0 {
			cycle.ReentryQuantity = cycle.ReentryQuantity * aiDecision.RecommendedQtyPct
		}
	}

	// Execute re-entry
	sr.AddDebugLog(fmt.Sprintf("Executing re-entry at %.8f, qty %.4f", currentPrice, cycle.ReentryQuantity))
	cycle.ReentryState = ReentryStateExecuting

	reentryQty := g.roundQuantity(pos.Symbol, cycle.ReentryQuantity)

	// Validate minimum quantity after rounding
	minQty := g.getMinQuantity(pos.Symbol)
	if reentryQty < minQty {
		sr.AddDebugLog(fmt.Sprintf("Re-entry qty %.8f below minimum %.8f after rounding, skipping", reentryQty, minQty))
		cycle.ReentryState = ReentryStateSkipped
		cycle.EndTime = time.Now()
		cycle.Outcome = "skipped"
		cycle.OutcomeReason = "below_minimum_qty"
		sr.SkippedReentries++
		sr.NextTPBlocked = false
		return nil
	}

	err := g.executeReentryOrder(pos, reentryQty, currentPrice)
	if err != nil {
		cycle.ReentryAttempts++
		if cycle.ReentryAttempts >= config.MaxReentryAttempts {
			sr.AddDebugLog(fmt.Sprintf("Re-entry failed after %d attempts: %v", cycle.ReentryAttempts, err))
			cycle.ReentryState = ReentryStateFailed
			cycle.EndTime = time.Now()
			cycle.Outcome = "failed"
			cycle.OutcomeReason = err.Error()
			sr.NextTPBlocked = false
		}
		return err
	}

	// Success
	cycle.ReentryState = ReentryStateCompleted
	cycle.ReentryFilledPrice = currentPrice
	cycle.ReentryFilledQty = reentryQty
	cycle.ReentryFillTime = time.Now()
	cycle.EndTime = time.Now()
	cycle.Outcome = "profit"
	cycle.Duration = cycle.EndTime.Sub(cycle.StartTime).String()

	sr.RemainingQuantity += reentryQty
	sr.TotalReentries++
	sr.SuccessfulReentries++
	sr.NextTPBlocked = false

	// CRITICAL: Sync pos.RemainingQty with sr.RemainingQuantity after reentry
	// sr.RemainingQuantity is the source of truth for scalp_reentry mode
	pos.RemainingQty = sr.RemainingQuantity

	// Update breakeven after re-entry
	sr.CurrentBreakeven = g.calculateNewBreakeven(pos, sr)

	sr.AddDebugLog(fmt.Sprintf("Re-entry complete! New remaining qty %.4f, new BE %.8f",
		sr.RemainingQuantity, sr.CurrentBreakeven))

	// CRITICAL: Update SL order for new quantity after reentry
	// The old SL order covers the old quantity, but now we have more position
	// Place SL at the new breakeven minus the SL percent
	// Use configured SL percent from ScalpReentryConfig (default 2.0%)
	slPercent := config.StopLossPercent
	if slPercent <= 0 {
		slPercent = 2.0 // Fallback to 2.0% if not configured
	}

	var newSL float64
	if pos.Side == "LONG" {
		newSL = sr.CurrentBreakeven * (1 - slPercent/100)
	} else {
		newSL = sr.CurrentBreakeven * (1 + slPercent/100)
	}

	// Update the SL order on Binance with new quantity and price
	if err := g.updatePositionStopLoss(pos, newSL); err != nil {
		log.Printf("[SCALP-REENTRY] %s: Failed to update SL after reentry: %v", pos.Symbol, err)
		sr.AddDebugLog(fmt.Sprintf("WARNING: Failed to update SL after reentry: %v", err))
	} else {
		log.Printf("[SCALP-REENTRY] %s: SL updated for new position size %.4f, BE=%.8f, SL=%.8f",
			pos.Symbol, sr.RemainingQuantity, sr.CurrentBreakeven, newSL)
		sr.AddDebugLog(fmt.Sprintf("SL updated for reentry: new SL=%.8f (BE=%.8f, SL%%=%.1f)", newSL, sr.CurrentBreakeven, slPercent))
	}

	// CRITICAL: Save position state after re-entry to survive restarts
	go g.SavePositionState()

	return nil
}

// monitorFinalTrailing monitors the final 20% position with trailing stop
func (g *GinieAutopilot) monitorFinalTrailing(pos *GiniePosition, currentPrice float64) error {
	sr := pos.ScalpReentry
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig

	// Update peak price
	if pos.Side == "LONG" {
		if currentPrice > sr.FinalTrailingPeak {
			sr.FinalTrailingPeak = currentPrice
			sr.FinalTrailingActive = true
			sr.AddDebugLog(fmt.Sprintf("New peak %.8f, trailing from %.1f%%", currentPrice, sr.FinalTrailingPercent))
		}
	} else {
		if currentPrice < sr.FinalTrailingPeak {
			sr.FinalTrailingPeak = currentPrice
			sr.FinalTrailingActive = true
			sr.AddDebugLog(fmt.Sprintf("New low %.8f (SHORT), trailing from %.1f%%", currentPrice, sr.FinalTrailingPercent))
		}
	}

	// Check trailing stop hit
	var trailingSLPrice float64
	var trailingHit bool

	if pos.Side == "LONG" {
		trailingSLPrice = sr.FinalTrailingPeak * (1 - sr.FinalTrailingPercent/100)
		trailingHit = currentPrice <= trailingSLPrice
	} else {
		trailingSLPrice = sr.FinalTrailingPeak * (1 + sr.FinalTrailingPercent/100)
		trailingHit = currentPrice >= trailingSLPrice
	}

	// Check dynamic SL hit
	dynamicSLHit := false
	if sr.DynamicSLActive && sr.DynamicSLPrice > 0 {
		if pos.Side == "LONG" {
			dynamicSLHit = currentPrice <= sr.DynamicSLPrice
		} else {
			dynamicSLHit = currentPrice >= sr.DynamicSLPrice
		}
	}

	// Exit if either trailing or dynamic SL hit
	if trailingHit || dynamicSLHit {
		reason := "trailing_stop"
		if dynamicSLHit {
			reason = "dynamic_sl"
		}
		sr.AddDebugLog(fmt.Sprintf("Final exit triggered: %s at %.8f", reason, currentPrice))

		// Validate and round the final portion quantity
		finalQty := g.roundQuantity(pos.Symbol, sr.FinalPortionQty)
		minQty := g.getMinQuantity(pos.Symbol)
		if finalQty < minQty {
			sr.AddDebugLog(fmt.Sprintf("Final portion qty %.8f below minimum %.8f, using minimum", finalQty, minQty))
			// If below minimum, try to close whatever is available
			if sr.RemainingQuantity > 0 {
				finalQty = g.roundQuantity(pos.Symbol, sr.RemainingQuantity)
			}
			if finalQty < minQty {
				// ALERT: Final portion stuck - too small to close
				alertMsg := "Final portion too small to close - NEEDS MANUAL INTERVENTION"
				sr.AddDebugLog(alertMsg)

				sr.NeedsManualIntervention = true
				sr.ManualInterventionReason = fmt.Sprintf("Final portion quantity %.6f is below minimum %.6f. Cannot close trailing position. Please close manually.", sr.RemainingQuantity, minQty)
				sr.ManualInterventionAlertAt = time.Now().Format(time.RFC3339)
				sr.LastUpdate = time.Now()

				log.Printf("[STUCK-POSITION-ALERT] %s %s: %s (remaining: %.6f, min: %.6f)",
					pos.Symbol, pos.Side, alertMsg, sr.RemainingQuantity, minQty)

				return nil
			}
		}

		// Close final portion
		err := g.executeScalpPartialClose(pos, finalQty, fmt.Sprintf("scalp_reentry_final_%s", reason))
		if err != nil {
			return err
		}

		// Calculate final PnL
		var finalPnl float64
		if pos.Side == "LONG" {
			finalPnl = (currentPrice - pos.EntryPrice) * sr.FinalPortionQty
		} else {
			finalPnl = (pos.EntryPrice - currentPrice) * sr.FinalPortionQty
		}
		sr.AccumulatedProfit += finalPnl
		sr.TotalCyclePnL = sr.AccumulatedProfit

		sr.AddDebugLog(fmt.Sprintf("Scalp re-entry complete! Total PnL: $%.2f, Cycles: %d, Reentries: %d/%d",
			sr.AccumulatedProfit, len(sr.Cycles), sr.SuccessfulReentries, sr.TotalReentries))

		// Position is now fully closed
		sr.FinalPortionActive = false
		sr.FinalPortionQty = 0
		sr.RemainingQuantity = 0
		sr.Enabled = false

		// CRITICAL: Sync pos.RemainingQty to zero after full close
		pos.RemainingQty = 0

		// Check if we should use AI for exit decision
		if config.UseAIDecisions {
			// AI exit decision can be used for logging/learning
			g.recordFinalExitForLearning(pos, reason, finalPnl)
		}
	}

	return nil
}

// updateDynamicSL updates the dynamic stop loss after 1% threshold
func (g *GinieAutopilot) updateDynamicSL(pos *GiniePosition, currentPrice float64, config ScalpReentryConfig) error {
	sr := pos.ScalpReentry

	// Calculate current unrealized PnL for remaining position
	var unrealizedPnL float64
	if pos.Side == "LONG" {
		unrealizedPnL = (currentPrice - pos.EntryPrice) * sr.RemainingQuantity
	} else {
		unrealizedPnL = (pos.EntryPrice - currentPrice) * sr.RemainingQuantity
	}

	// Total potential profit = accumulated + unrealized
	totalPotentialProfit := sr.AccumulatedProfit + unrealizedPnL

	// Calculate new dynamic SL to protect 60% of total profit
	protectedAmount := totalPotentialProfit * (config.DynamicSLProtectPct / 100)
	maxLoss := totalPotentialProfit * (config.DynamicSLMaxLossPct / 100)

	// Calculate SL price that protects the required profit
	// If we lose maxLoss from current position, where would price be?
	var newSLPrice float64
	if pos.Side == "LONG" {
		// For LONG: price drop that causes maxLoss
		// maxLoss = (entryPrice - slPrice) * qty (when price goes down past entry)
		// If in profit: maxLoss = currentPrice - slPrice
		priceDropAllowed := maxLoss / sr.RemainingQuantity
		newSLPrice = currentPrice - priceDropAllowed
		// Never set SL below entry
		if newSLPrice < pos.EntryPrice && sr.AccumulatedProfit > 0 {
			newSLPrice = pos.EntryPrice
		}
	} else {
		// For SHORT: price rise that causes maxLoss
		priceRiseAllowed := maxLoss / sr.RemainingQuantity
		newSLPrice = currentPrice + priceRiseAllowed
		// Never set SL above entry for SHORT
		if newSLPrice > pos.EntryPrice && sr.AccumulatedProfit > 0 {
			newSLPrice = pos.EntryPrice
		}
	}

	// Only update if new SL is better (tighter)
	shouldUpdate := false
	if sr.DynamicSLPrice == 0 {
		shouldUpdate = true
	} else if pos.Side == "LONG" && newSLPrice > sr.DynamicSLPrice {
		shouldUpdate = true
	} else if pos.Side == "SHORT" && newSLPrice < sr.DynamicSLPrice {
		shouldUpdate = true
	}

	if shouldUpdate {
		oldSL := sr.DynamicSLPrice
		// Round the SL price for proper precision before storing and placing order
		roundedSLPrice := roundPriceForSL(pos.Symbol, newSLPrice, pos.Side)
		sr.DynamicSLPrice = roundedSLPrice
		sr.ProtectedProfit = protectedAmount
		sr.MaxAllowableLoss = maxLoss
		sr.LastUpdate = time.Now()

		sr.AddDebugLog(fmt.Sprintf("Dynamic SL updated: %.8f -> %.8f (protecting $%.2f, max loss $%.2f)",
			oldSL, roundedSLPrice, protectedAmount, maxLoss))

		// Update the actual SL order on exchange (already rounded)
		return g.updatePositionStopLoss(pos, roundedSLPrice)
	}

	return nil
}

// ============ HELPER FUNCTIONS ============

// getAIReentryDecision gets AI decision for re-entry
func (g *GinieAutopilot) getAIReentryDecision(pos *GiniePosition, cycle *ReentryCycle) (bool, *ReentryAIDecision) {
	config := GetSettingsManager().GetCurrentSettings().ScalpReentryConfig
	currentPrice := g.getCurrentPrice(pos.Symbol)

	// Default decision if AI is unavailable
	defaultDecision := &ReentryAIDecision{
		ShouldReenter:   true,
		Confidence:      0.7,
		RecommendedQtyPct: 1.0,
		Reasoning:       "Default decision - AI unavailable",
		MarketCondition: "unknown",
		TrendAlignment:  true,
		RiskLevel:       "medium",
		Timestamp:       time.Now(),
	}

	if g.analyzer == nil {
		return true, defaultDecision
	}

	// Build market data for analysis
	// In a full implementation, we would fetch actual indicators here
	distanceFromBE := math.Abs(currentPrice-cycle.ReentryTargetPrice) / cycle.ReentryTargetPrice * 100

	// Call AI analyzer (this is a placeholder - actual implementation would call the analyzer)
	// For now, return a simple heuristic decision
	aiDecision := &ReentryAIDecision{
		ShouldReenter:     true,
		Confidence:        0.75,
		RecommendedQtyPct: 1.0,
		Reasoning:         fmt.Sprintf("Price near breakeven (%.2f%% away), trend favorable", distanceFromBE),
		MarketCondition:   "ranging",
		TrendAlignment:    true,
		RiskLevel:         "medium",
		Timestamp:         time.Now(),
	}

	// Check confidence threshold
	if aiDecision.Confidence < config.AIMinConfidence {
		aiDecision.ShouldReenter = false
		aiDecision.Reasoning = fmt.Sprintf("Confidence %.2f below threshold %.2f", aiDecision.Confidence, config.AIMinConfidence)
	}

	return aiDecision.ShouldReenter, aiDecision
}

// executeReentryOrder executes a re-entry order
func (g *GinieAutopilot) executeReentryOrder(pos *GiniePosition, qty float64, price float64) error {
	// Determine order side based on position side
	orderSide := "BUY"
	positionSide := "LONG"
	if pos.Side == "SHORT" {
		orderSide = "SELL"
		positionSide = "SHORT"
	}

	// Use LIMIT order with slight offset for quick fill
	// For BUY: place slightly above current price to ensure fill
	// For SELL: place slightly below current price to ensure fill
	limitPrice := price
	if orderSide == "BUY" {
		limitPrice = price * 1.0005 // 0.05% above for quick fill
	} else {
		limitPrice = price * 0.9995 // 0.05% below for quick fill
	}
	limitPrice = roundPrice(pos.Symbol, limitPrice)

	order := &FuturesOrder{
		Symbol:       pos.Symbol,
		Side:         orderSide,
		PositionSide: positionSide,
		Type:         "LIMIT",
		Quantity:     qty,
		Price:        limitPrice,
	}

	log.Printf("[SCALP-REENTRY] %s: Placing LIMIT re-entry order at %.8f (current: %.8f)", pos.Symbol, limitPrice, price)

	return g.placeOrder(order)
}

// calculateNewBreakeven calculates the new breakeven after re-entry
// CORRECT FORMULA: breakeven = netCost / remainingQty
// where netCost = (original_cost - sold_value + reentry_costs)
// and remainingQty = (original_qty - sold_qty + reentry_qty)
func (g *GinieAutopilot) calculateNewBreakeven(pos *GiniePosition, sr *ScalpReentryStatus) float64 {
	// Start with original entry
	netCost := pos.EntryPrice * pos.OriginalQty
	netQty := pos.OriginalQty

	// Process each cycle: subtract sells, add reentries
	for _, cycle := range sr.Cycles {
		// ALWAYS subtract the sold quantity and value
		// This was the BUG - we were not subtracting sold quantities!
		if cycle.SellQuantity > 0 {
			netCost -= cycle.SellPrice * cycle.SellQuantity
			netQty -= cycle.SellQuantity
		}

		// Add back reentry if completed
		if cycle.ReentryState == ReentryStateCompleted && cycle.ReentryFilledQty > 0 {
			netCost += cycle.ReentryFilledPrice * cycle.ReentryFilledQty
			netQty += cycle.ReentryFilledQty
		}
	}

	// Guard against division by zero or negative qty
	if netQty <= 0 {
		return pos.EntryPrice
	}

	// Round the breakeven price for consistency
	breakeven := netCost / netQty
	return roundPrice(pos.Symbol, breakeven)
}

// recordFinalExitForLearning records exit data for adaptive learning
func (g *GinieAutopilot) recordFinalExitForLearning(pos *GiniePosition, reason string, pnl float64) {
	// This will be implemented in the learning module
	// For now, just log
	if pos.ScalpReentry != nil {
		pos.ScalpReentry.AddDebugLog(fmt.Sprintf("Learning record: exit=%s, pnl=$%.2f", reason, pnl))
	}
}

// ============ PLACEHOLDER METHODS ============
// These methods should be implemented or already exist in ginie_autopilot.go

// getCurrentPrice gets current price for a symbol from Binance API
func (g *GinieAutopilot) getCurrentPrice(symbol string) float64 {
	if g.futuresClient == nil {
		log.Printf("[SCALP-REENTRY] %s: futuresClient is nil, cannot get price", symbol)
		return 0
	}
	price, err := g.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		log.Printf("[SCALP-REENTRY] %s: Failed to get current price: %v", symbol, err)
		return 0
	}
	return price
}

// roundQuantity rounds quantity to appropriate precision for a symbol
func (g *GinieAutopilot) roundQuantity(symbol string, qty float64) float64 {
	// Use the proper precision lookup from futures_controller.go
	precision := getQuantityPrecision(symbol)
	multiplier := math.Pow(10, float64(precision))
	return math.Floor(qty*multiplier) / multiplier
}

// NOTE: getMinQuantity is implemented in ginie_autopilot.go

// executeScalpPartialClose executes a partial close for scalp re-entry mode
// This differs from the standard executePartialClose by taking explicit qty and reason
func (g *GinieAutopilot) executeScalpPartialClose(pos *GiniePosition, qty float64, reason string) error {
	// Use LIMIT order for better price execution
	if qty <= 0 {
		return fmt.Errorf("invalid quantity: %.8f", qty)
	}

	// Determine side for closing
	var closeSide string
	if pos.Side == "LONG" {
		closeSide = "SELL"
	} else {
		closeSide = "BUY"
	}

	// Get current price for LIMIT order
	currentPrice := g.getCurrentPrice(pos.Symbol)
	if currentPrice <= 0 {
		// Fallback to market order if price unavailable
		log.Printf("[SCALP-REENTRY] %s: Price unavailable, using MARKET order for close", pos.Symbol)
		order := &FuturesOrder{
			Symbol:       pos.Symbol,
			Side:         closeSide,
			PositionSide: pos.Side,
			Type:         "MARKET",
			Quantity:     qty,
		}
		return g.placeOrder(order)
	}

	// Calculate LIMIT price with slight offset for quick fill
	// For SELL (closing LONG): place slightly below current price
	// For BUY (closing SHORT): place slightly above current price
	limitPrice := currentPrice
	if closeSide == "SELL" {
		limitPrice = currentPrice * 0.9995 // 0.05% below for quick fill
	} else {
		limitPrice = currentPrice * 1.0005 // 0.05% above for quick fill
	}
	limitPrice = roundPrice(pos.Symbol, limitPrice)

	// Place LIMIT close order
	order := &FuturesOrder{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: pos.Side,
		Type:         "LIMIT",
		Quantity:     qty,
		Price:        limitPrice,
	}

	log.Printf("[SCALP-REENTRY] %s: Executing LIMIT partial close at %.8f (current: %.8f): qty=%.8f, reason=%s",
		pos.Symbol, limitPrice, currentPrice, qty, reason)

	return g.placeOrder(order)
}

// placeOrder places a futures order using the actual Binance API
// Uses SymbolValidator for proper precision and pre-validation
func (g *GinieAutopilot) placeOrder(order *FuturesOrder) error {
	if order == nil {
		return fmt.Errorf("order is nil")
	}

	// Validate quantity
	if order.Quantity <= 0 {
		return fmt.Errorf("invalid order quantity: %.8f", order.Quantity)
	}

	// Get effective position side for hedge mode compatibility
	effectivePositionSide := g.getEffectivePositionSide(binance.PositionSide(order.PositionSide))

	// Use SymbolValidator for proper rounding and validation
	validator := GetSymbolValidator()
	isMarketOrder := order.Type == "MARKET"

	// Validate order BEFORE placement
	validation := validator.ValidateOrder(order.Symbol, order.Quantity, order.Price, isMarketOrder)

	// Log validation result
	if !validation.Valid {
		for _, verr := range validation.Errors {
			log.Printf("[SCALP-ORDER] %s: Validation failed - %s", order.Symbol, verr.Message)
		}
		if len(validation.Errors) > 0 {
			return fmt.Errorf("order validation failed for %s: %s", order.Symbol, validation.Errors[0].Message)
		}
		return fmt.Errorf("order validation failed for %s: unknown error", order.Symbol)
	}

	// Use validated and rounded values
	roundedQty := validation.RoundedQty
	roundedPrice := validation.RoundedPrice

	if roundedQty <= 0 {
		return fmt.Errorf("rounded quantity is 0 for %s (original: %.8f)", order.Symbol, order.Quantity)
	}

	// Log any warnings
	for _, warning := range validation.Warnings {
		log.Printf("[SCALP-ORDER] %s: Warning - %s", order.Symbol, warning)
	}

	// Build order params with validated values
	orderParams := binance.FuturesOrderParams{
		Symbol:       order.Symbol,
		Side:         order.Side,
		PositionSide: effectivePositionSide,
		Type:         binance.FuturesOrderType(order.Type),
		Quantity:     roundedQty,
	}

	// Add price for limit orders (use rounded price)
	if order.Type == "LIMIT" && roundedPrice > 0 {
		orderParams.Price = roundedPrice
		orderParams.TimeInForce = binance.TimeInForceGTC
	}

	log.Printf("[SCALP-ORDER] %s: Placing %s %s order: qty=%.8f (was %.8f), side=%s, positionSide=%s",
		order.Symbol, order.Type, order.Side, roundedQty, order.Quantity, order.Side, effectivePositionSide)

	// Place the order
	result, err := g.futuresClient.PlaceFuturesOrder(orderParams)
	if err != nil {
		log.Printf("[SCALP-ORDER] %s: Order failed: %v", order.Symbol, err)
		return fmt.Errorf("failed to place order: %w", err)
	}

	log.Printf("[SCALP-ORDER] %s: Order placed successfully, orderId=%d, status=%s",
		order.Symbol, result.OrderId, result.Status)

	return nil
}

// updatePositionStopLoss updates the stop loss for a position
func (g *GinieAutopilot) updatePositionStopLoss(pos *GiniePosition, newSL float64) error {
	if newSL <= 0 {
		return fmt.Errorf("invalid stop loss price: %.8f", newSL)
	}

	// Update position's stop loss
	oldSL := pos.StopLoss
	pos.StopLoss = newSL

	log.Printf("[SCALP-SL] %s: Updating stop loss from %.8f to %.8f", pos.Symbol, oldSL, newSL)

	// Cancel existing SL order if present
	if pos.StopLossAlgoID > 0 {
		if err := g.futuresClient.CancelAlgoOrder(pos.Symbol, pos.StopLossAlgoID); err != nil {
			log.Printf("[SCALP-SL] %s: Failed to cancel old SL order %d: %v", pos.Symbol, pos.StopLossAlgoID, err)
			// Continue anyway to place new SL
		} else {
			log.Printf("[SCALP-SL] %s: Cancelled old SL order %d", pos.Symbol, pos.StopLossAlgoID)
		}
		pos.StopLossAlgoID = 0
	}

	// Place new SL order
	var closeSide string
	if pos.Side == "LONG" {
		closeSide = "SELL"
	} else {
		closeSide = "BUY"
	}

	effectivePositionSide := g.getEffectivePositionSide(binance.PositionSide(pos.Side))
	roundedSL := roundPriceForSL(pos.Symbol, newSL, pos.Side)

	// Get quantity for SL
	slQty := pos.RemainingQty
	if slQty <= 0 {
		if pos.ScalpReentry != nil && pos.ScalpReentry.RemainingQuantity > 0 {
			slQty = pos.ScalpReentry.RemainingQuantity
		} else if pos.OriginalQty > 0 {
			slQty = pos.OriginalQty
		}
	}
	roundedQty := g.roundQuantity(pos.Symbol, slQty)

	slParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: effectivePositionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		Quantity:     roundedQty,
		TriggerPrice: roundedSL,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	slOrder, err := g.futuresClient.PlaceAlgoOrder(slParams)
	if err != nil {
		log.Printf("[SCALP-SL] %s: Failed to place new SL order: %v", pos.Symbol, err)
		return fmt.Errorf("failed to place new SL order: %w", err)
	}

	pos.StopLossAlgoID = slOrder.AlgoId
	log.Printf("[SCALP-SL] %s: New SL order placed, algoId=%d, triggerPrice=%.8f",
		pos.Symbol, slOrder.AlgoId, roundedSL)

	return nil
}

// FuturesOrder represents a futures order
type FuturesOrder struct {
	Symbol       string
	Side         string
	PositionSide string
	Type         string
	Quantity     float64
	Price        float64
	StopPrice    float64
}
