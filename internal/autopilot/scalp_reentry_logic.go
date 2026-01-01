package autopilot

import (
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

	// Step 1: Check if we're in final portion mode (after 80% sold at 1%)
	if sr.FinalPortionActive {
		return g.monitorFinalTrailing(pos, currentPrice)
	}

	// Step 2: Check if we're waiting for a re-entry
	if sr.NextTPBlocked && sr.IsWaitingForReentry() {
		return g.checkAndExecuteReentry(pos, currentPrice)
	}

	// Step 3: Check if TP level is reached (only if not blocked)
	if !sr.NextTPBlocked || sr.CanProceedToNextTP() {
		nextTPLevel := sr.TPLevelUnlocked + 1
		if nextTPLevel <= 3 {
			tpHit, _ := g.checkScalpReentryTP(pos, currentPrice, nextTPLevel)
			if tpHit {
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
		return currentPrice >= tpPrice, tpPrice
	} else {
		tpPrice = pos.EntryPrice * (1 - tpPercent/100)
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
	var sellQty float64
	if tpLevel == 3 {
		// At TP3 (1%), sell 80% of remaining, keep 20%
		sellQty = sr.RemainingQuantity * (sellPercent / 100)
	} else {
		// At TP1/TP2, sell configured percentage
		sellQty = sr.RemainingQuantity * (sellPercent / 100)
	}

	// Round quantity to appropriate precision
	sellQty = g.roundQuantity(pos.Symbol, sellQty)

	if sellQty < g.getMinQuantity(pos.Symbol) {
		sr.AddDebugLog(fmt.Sprintf("TP%d: Sell qty %.4f below minimum, skipping", tpLevel, sellQty))
		return nil
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

	// Update position state
	sr.RemainingQuantity -= sellQty
	sr.AccumulatedProfit += pnl
	sr.TPLevelUnlocked = tpLevel

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
		cycle.ReentryTargetPrice = sr.CurrentBreakeven
		cycle.ReentryQuantity = sellQty * (config.ReentryPercent / 100)
		cycle.ReentryState = ReentryStateWaiting

		sr.NextTPBlocked = true
		sr.AddDebugLog(fmt.Sprintf("Waiting for re-entry at breakeven %.8f, target qty %.4f",
			cycle.ReentryTargetPrice, cycle.ReentryQuantity))
	}

	sr.Cycles = append(sr.Cycles, cycle)
	sr.CurrentCycle = len(sr.Cycles)
	sr.LastUpdate = time.Now()

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

	// Update breakeven after re-entry
	sr.CurrentBreakeven = g.calculateNewBreakeven(pos, sr)

	sr.AddDebugLog(fmt.Sprintf("Re-entry complete! New remaining qty %.4f, new BE %.8f",
		sr.RemainingQuantity, sr.CurrentBreakeven))

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

		// Close final portion
		err := g.executeScalpPartialClose(pos, sr.FinalPortionQty, fmt.Sprintf("scalp_reentry_final_%s", reason))
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
		sr.DynamicSLPrice = newSLPrice
		sr.ProtectedProfit = protectedAmount
		sr.MaxAllowableLoss = maxLoss
		sr.LastUpdate = time.Now()

		sr.AddDebugLog(fmt.Sprintf("Dynamic SL updated: %.8f -> %.8f (protecting $%.2f, max loss $%.2f)",
			oldSL, newSLPrice, protectedAmount, maxLoss))

		// Update the actual SL order on exchange
		return g.updatePositionStopLoss(pos, newSLPrice)
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

	// Use market order for immediate execution
	order := &FuturesOrder{
		Symbol:       pos.Symbol,
		Side:         orderSide,
		PositionSide: positionSide,
		Type:         "MARKET",
		Quantity:     qty,
	}

	return g.placeOrder(order)
}

// calculateNewBreakeven calculates the new breakeven after re-entry
func (g *GinieAutopilot) calculateNewBreakeven(pos *GiniePosition, sr *ScalpReentryStatus) float64 {
	// Weighted average of all entries
	totalCost := 0.0
	totalQty := 0.0

	// Original entry
	totalCost += pos.EntryPrice * pos.OriginalQty
	totalQty += pos.OriginalQty

	// Subtract sold quantities and add re-entries
	for _, cycle := range sr.Cycles {
		if cycle.ReentryState == ReentryStateCompleted {
			totalCost += cycle.ReentryFilledPrice * cycle.ReentryFilledQty
			totalQty += cycle.ReentryFilledQty
		}
	}

	if totalQty == 0 {
		return pos.EntryPrice
	}

	return totalCost / totalQty
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

// getCurrentPrice gets current price for a symbol
func (g *GinieAutopilot) getCurrentPrice(symbol string) float64 {
	// This should fetch from price cache or API
	// Placeholder returns 0, actual implementation exists in ginie_autopilot.go
	return 0
}

// roundQuantity rounds quantity to appropriate precision for a symbol
func (g *GinieAutopilot) roundQuantity(symbol string, qty float64) float64 {
	// Placeholder - actual implementation should use symbol info
	return math.Floor(qty*1000) / 1000
}

// NOTE: getMinQuantity is implemented in ginie_autopilot.go

// executeScalpPartialClose executes a partial close for scalp re-entry mode
// This differs from the standard executePartialClose by taking explicit qty and reason
func (g *GinieAutopilot) executeScalpPartialClose(pos *GiniePosition, qty float64, reason string) error {
	// Use the existing partial close mechanism via market order
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

	// Place market close order
	order := &FuturesOrder{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: pos.Side,
		Type:         "MARKET",
		Quantity:     qty,
	}

	log.Printf("[SCALP-REENTRY] %s: Executing partial close: qty=%.8f, reason=%s", pos.Symbol, qty, reason)

	return g.placeOrder(order)
}

// placeOrder places a futures order using the actual Binance API
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

	// Round quantity for the symbol
	roundedQty := g.roundQuantity(order.Symbol, order.Quantity)
	if roundedQty <= 0 {
		return fmt.Errorf("rounded quantity is 0 for %s", order.Symbol)
	}

	// Build order params
	orderParams := binance.FuturesOrderParams{
		Symbol:       order.Symbol,
		Side:         order.Side,
		PositionSide: effectivePositionSide,
		Type:         binance.FuturesOrderType(order.Type),
		Quantity:     roundedQty,
	}

	// Add price for limit orders
	if order.Type == "LIMIT" && order.Price > 0 {
		orderParams.Price = order.Price
		orderParams.TimeInForce = binance.TimeInForceGTC
	}

	log.Printf("[SCALP-ORDER] %s: Placing %s %s order: qty=%.8f, side=%s, positionSide=%s",
		order.Symbol, order.Type, order.Side, roundedQty, order.Side, effectivePositionSide)

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
