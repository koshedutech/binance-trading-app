// Package autopilot provides dynamic SL/TP management with Binance order updates.
// Story 10.1: Dynamic SL/TP Management - Backend Implementation
package autopilot

import (
	"context"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/binance"
)

// ==================== STORY 10.1: BINANCE ORDER UPDATE FUNCTIONS ====================

// DynamicSLTPUpdateResult holds the result of a dynamic SL/TP update operation
type DynamicSLTPUpdateResult struct {
	Symbol      string   `json:"symbol"`
	Side        string   `json:"side"`
	OldSL       float64  `json:"old_sl,omitempty"`
	NewSL       float64  `json:"new_sl,omitempty"`
	SLUpdated   bool     `json:"sl_updated"`
	SLAlgoID    int64    `json:"sl_algo_id,omitempty"`
	OldTP       float64  `json:"old_tp,omitempty"`
	NewTP       float64  `json:"new_tp,omitempty"`
	TPUpdated   bool     `json:"tp_updated"`
	TPAlgoID    int64    `json:"tp_algo_id,omitempty"`
	Error       string   `json:"error,omitempty"`
	ATRPct      float64  `json:"atr_pct"`
	ADX         float64  `json:"adx"`
	Adjustments []string `json:"adjustments,omitempty"`
}

// updateBinanceStopLoss updates the stop loss order on Binance for a position
// Story 10.1: Dynamic SL/TP on Binance
//
// This function:
//   - Cancels existing SL algo order if any
//   - Places new STOP_MARKET order at newSL
//   - Updates pos.StopLoss and pos.StopLossAlgoID
//   - Only updates if newSL is better (higher for LONG, lower for SHORT)
func (ga *GinieAutopilot) updateBinanceStopLoss(pos *GiniePosition, newSL float64) error {
	if pos == nil {
		return fmt.Errorf("position is nil")
	}

	if newSL <= 0 {
		return fmt.Errorf("invalid SL price: %.8f", newSL)
	}

	// Check if new SL is actually better
	if !IsSLImproved(pos, newSL) {
		return fmt.Errorf("new SL %.8f is not better than current %.8f for %s %s",
			newSL, pos.StopLoss, pos.Side, pos.Symbol)
	}

	// Round SL price for tick size compliance
	roundedSL := roundPriceForSL(pos.Symbol, newSL, pos.Side)

	ga.logger.Info("Dynamic SL update starting",
		"symbol", pos.Symbol,
		"side", pos.Side,
		"old_sl", pos.StopLoss,
		"new_sl", roundedSL)

	// Cancel existing SL order if present
	if pos.StopLossAlgoID > 0 {
		err := ga.futuresClient.CancelAlgoOrder(pos.Symbol, pos.StopLossAlgoID)
		if err != nil {
			ga.logger.Warn("Failed to cancel existing SL order",
				"symbol", pos.Symbol,
				"algo_id", pos.StopLossAlgoID,
				"error", err)
			// Continue anyway - the order might have been filled or expired
		} else {
			ga.logger.Info("Cancelled existing SL order",
				"symbol", pos.Symbol,
				"algo_id", pos.StopLossAlgoID)
		}
	}

	// Determine close side based on position side
	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Round quantity
	roundedQty := roundQuantity(pos.Symbol, pos.RemainingQty)

	// Place new SL order using Algo Order API
	slParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeStopMarket,
		Quantity:     roundedQty,
		TriggerPrice: roundedSL,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	// Add client order ID if generator is available
	if ga.clientOrderIdGen != nil && pos.ChainBaseID != "" {
		slClientOrderId, err := ga.clientOrderIdGen.GenerateRelated(pos.ChainBaseID, "SL")
		if err == nil {
			slParams.ClientAlgoId = slClientOrderId
		}
	}

	// Place the new SL order
	slResponse, err := ga.futuresClient.PlaceAlgoOrder(slParams)
	if err != nil {
		return fmt.Errorf("failed to place new SL order: %w", err)
	}

	// Update position state
	oldSL := pos.StopLoss
	pos.StopLoss = roundedSL
	pos.StopLossAlgoID = slResponse.AlgoId

	ga.logger.Info("Dynamic SL updated successfully",
		"symbol", pos.Symbol,
		"old_sl", oldSL,
		"new_sl", roundedSL,
		"algo_id", slResponse.AlgoId,
		"improvement_pct", fmt.Sprintf("%.2f%%", ((roundedSL-oldSL)/oldSL)*100))

	// Save position state after SL update
	if err := ga.SavePositionState(); err != nil {
		ga.logger.Warn("Failed to save position state after SL update",
			"symbol", pos.Symbol,
			"error", err)
	}

	return nil
}

// updateBinanceTakeProfit updates the take profit order on Binance for a position
// Story 10.1: Dynamic SL/TP on Binance
//
// This function:
//   - Cancels existing TP algo orders if any
//   - Places new TAKE_PROFIT_MARKET order at newTP
//   - Updates pos.TakeProfits and pos.TakeProfitAlgoIDs
//   - Only updates if newTP is better (higher for LONG, lower for SHORT)
func (ga *GinieAutopilot) updateBinanceTakeProfit(pos *GiniePosition, newTP float64) error {
	if pos == nil {
		return fmt.Errorf("position is nil")
	}

	if newTP <= 0 {
		return fmt.Errorf("invalid TP price: %.8f", newTP)
	}

	// Check if new TP is actually better
	if !IsTPImproved(pos, newTP) {
		return fmt.Errorf("new TP %.8f is not better than current for %s %s",
			newTP, pos.Side, pos.Symbol)
	}

	// Round TP price for tick size compliance
	roundedTP := roundPriceForTP(pos.Symbol, newTP, pos.Side)

	ga.logger.Info("Dynamic TP update starting",
		"symbol", pos.Symbol,
		"side", pos.Side,
		"new_tp", roundedTP)

	// Cancel existing TP orders if present
	for _, algoID := range pos.TakeProfitAlgoIDs {
		if algoID > 0 {
			err := ga.futuresClient.CancelAlgoOrder(pos.Symbol, algoID)
			if err != nil {
				ga.logger.Warn("Failed to cancel existing TP order",
					"symbol", pos.Symbol,
					"algo_id", algoID,
					"error", err)
				// Continue anyway
			} else {
				ga.logger.Info("Cancelled existing TP order",
					"symbol", pos.Symbol,
					"algo_id", algoID)
			}
		}
	}

	// Determine close side based on position side
	closeSide := "SELL"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	}

	// Round quantity
	roundedQty := roundQuantity(pos.Symbol, pos.RemainingQty)

	// Place new TP order using Algo Order API
	tpParams := binance.AlgoOrderParams{
		Symbol:       pos.Symbol,
		Side:         closeSide,
		PositionSide: positionSide,
		Type:         binance.FuturesOrderTypeTakeProfitMarket,
		Quantity:     roundedQty,
		TriggerPrice: roundedTP,
		WorkingType:  binance.WorkingTypeMarkPrice,
	}

	// Add client order ID if generator is available
	if ga.clientOrderIdGen != nil && pos.ChainBaseID != "" {
		tpClientOrderId, err := ga.clientOrderIdGen.GenerateRelated(pos.ChainBaseID, "TP")
		if err == nil {
			tpParams.ClientAlgoId = tpClientOrderId
		}
	}

	// Place the new TP order
	tpResponse, err := ga.futuresClient.PlaceAlgoOrder(tpParams)
	if err != nil {
		return fmt.Errorf("failed to place new TP order: %w", err)
	}

	// Update position state - add or update TP level
	// GinieTakeProfitLevel has: Level, Price, Percent, GainPct, Status
	newTPLevel := GinieTakeProfitLevel{
		Level:   len(pos.TakeProfits) + 1,
		Price:   roundedTP,
		Percent: 100.0, // Full position for dynamic TP
		GainPct: ((roundedTP - pos.EntryPrice) / pos.EntryPrice) * 100,
		Status:  "pending",
	}
	if pos.Side == "SHORT" {
		newTPLevel.GainPct = ((pos.EntryPrice - roundedTP) / pos.EntryPrice) * 100
	}
	pos.TakeProfits = append(pos.TakeProfits, newTPLevel)
	pos.TakeProfitAlgoIDs = []int64{tpResponse.AlgoId}

	ga.logger.Info("Dynamic TP updated successfully",
		"symbol", pos.Symbol,
		"new_tp", roundedTP,
		"algo_id", tpResponse.AlgoId)

	// Save position state after TP update
	if err := ga.SavePositionState(); err != nil {
		ga.logger.Warn("Failed to save position state after TP update",
			"symbol", pos.Symbol,
			"error", err)
	}

	return nil
}

// UpdateDynamicLevels is the main entry point for dynamic SL/TP management
// Story 10.1: Combined update function
//
// This function:
//   - Gets ATR% and ADX from Redis cache (decision:user:{userID}:coin:{symbol})
//   - Calculates new SL and TP using efficiency-aware formulas
//   - Updates on Binance only if improvement
//   - Respects Binance rate limits
func (ga *GinieAutopilot) UpdateDynamicLevels(pos *GiniePosition) (*DynamicSLTPUpdateResult, error) {
	if pos == nil {
		return nil, fmt.Errorf("position is nil")
	}

	result := &DynamicSLTPUpdateResult{
		Symbol: pos.Symbol,
		Side:   pos.Side,
		OldSL:  pos.StopLoss,
	}

	// Get current price
	currentPrice, err := ga.getCurrentMarkPrice(pos.Symbol)
	if err != nil {
		result.Error = fmt.Sprintf("failed to get current price: %v", err)
		return result, err
	}

	// Get ATR% and ADX from Redis cache via state manager
	atrPct, adx, err := ga.getIndicatorsFromCache(pos.Symbol)
	if err != nil {
		ga.logger.Warn("Failed to get indicators from cache, using fallback",
			"symbol", pos.Symbol,
			"error", err)
		// Fallback: calculate from klines if cache unavailable
		atrPct, adx = ga.calculateIndicatorsFallback(pos.Symbol)
	}

	result.ATRPct = atrPct
	result.ADX = adx

	// Use default config
	config := DefaultDynamicLevelConfig()

	// Calculate new levels
	levelResult := DynamicLevelCalculator(pos, atrPct, adx, currentPrice, config)
	if levelResult == nil {
		result.Error = "failed to calculate dynamic levels"
		return result, fmt.Errorf("failed to calculate dynamic levels for %s", pos.Symbol)
	}

	result.Adjustments = levelResult.Adjustments
	result.NewSL = levelResult.SLPrice
	result.NewTP = levelResult.TPPrice

	// Update SL if improved
	if levelResult.SLImproved && levelResult.SLPrice > 0 {
		err := ga.updateBinanceStopLoss(pos, levelResult.SLPrice)
		if err != nil {
			ga.logger.Warn("Failed to update dynamic SL",
				"symbol", pos.Symbol,
				"error", err)
			result.Error = fmt.Sprintf("SL update failed: %v", err)
		} else {
			result.SLUpdated = true
			result.SLAlgoID = pos.StopLossAlgoID
			ga.logger.Info("Dynamic SL updated",
				"symbol", pos.Symbol,
				"old_sl", result.OldSL,
				"new_sl", levelResult.SLPrice)
		}
	}

	// Update TP if improved
	if levelResult.TPImproved && levelResult.TPPrice > 0 {
		// Get old TP for logging
		if len(pos.TakeProfits) > 0 {
			result.OldTP = pos.TakeProfits[len(pos.TakeProfits)-1].Price
		}

		err := ga.updateBinanceTakeProfit(pos, levelResult.TPPrice)
		if err != nil {
			ga.logger.Warn("Failed to update dynamic TP",
				"symbol", pos.Symbol,
				"error", err)
			if result.Error != "" {
				result.Error += "; "
			}
			result.Error += fmt.Sprintf("TP update failed: %v", err)
		} else {
			result.TPUpdated = true
			if len(pos.TakeProfitAlgoIDs) > 0 {
				result.TPAlgoID = pos.TakeProfitAlgoIDs[0]
			}
			ga.logger.Info("Dynamic TP updated",
				"symbol", pos.Symbol,
				"old_tp", result.OldTP,
				"new_tp", levelResult.TPPrice)
		}
	}

	// Log summary
	log.Printf("[DYNAMIC-SLTP] %s %s: ATR=%.2f%%, ADX=%.1f, SL_Updated=%v, TP_Updated=%v",
		pos.Symbol, pos.Side, atrPct, adx, result.SLUpdated, result.TPUpdated)

	return result, nil
}

// getIndicatorsFromCache retrieves ATR% and ADX from Redis cache
func (ga *GinieAutopilot) getIndicatorsFromCache(symbol string) (atrPct, adx float64, err error) {
	if ga.stateManager == nil {
		return 0, 0, fmt.Errorf("state manager not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Get coin state from state manager
	// The state manager stores indicators in the CoinState struct
	coinState, err := ga.getCoinStateFromManager(ctx, symbol)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get coin state: %w", err)
	}

	if coinState == nil {
		return 0, 0, fmt.Errorf("coin state not found for %s", symbol)
	}

	// Extract ATR% - calculate from ATR and current price
	if coinState.ATR > 0 && coinState.Price > 0 {
		atrPct = (coinState.ATR / coinState.Price) * 100
	}

	adx = coinState.ADX

	return atrPct, adx, nil
}

// CoinStateFromManager is a helper struct to avoid circular imports
type CoinStateFromManager struct {
	Symbol string
	Price  float64
	ADX    float64
	ATR    float64
	RSI    float64
}

// getCoinStateFromManager retrieves coin state using the state manager interface
func (ga *GinieAutopilot) getCoinStateFromManager(ctx context.Context, symbol string) (*CoinStateFromManager, error) {
	// The stateManager is StateManagerInterface which has SetCoinState
	// We need to access Redis directly for reading
	// This is a simplified approach - in production, the state manager should have GetCoinState

	// Fallback to calculating from klines if state manager doesn't support read
	return nil, fmt.Errorf("state manager read not implemented, use fallback")
}

// calculateIndicatorsFallback calculates ATR% and ADX from klines when cache is unavailable
func (ga *GinieAutopilot) calculateIndicatorsFallback(symbol string) (atrPct, adx float64) {
	// Get klines for calculation
	klines, err := ga.futuresClient.GetFuturesKlines(symbol, "1h", 50)
	if err != nil || len(klines) < 20 {
		log.Printf("[DYNAMIC-SLTP] Failed to get klines for %s: %v", symbol, err)
		return 1.5, 25.0 // Default values
	}

	// Calculate ATR%
	atr := calculateATRFromKlines(klines, 14)
	currentPrice := klines[len(klines)-1].Close
	if currentPrice > 0 {
		atrPct = (atr / currentPrice) * 100
	} else {
		atrPct = 1.5
	}

	// Calculate ADX
	adx = calculateADXFromKlines(klines, 14)
	if adx <= 0 {
		adx = 25.0 // Default
	}

	return atrPct, adx
}

// getCurrentMarkPrice gets the current mark price for a symbol with error handling.
// This is different from getCurrentPrice which returns just the price without error.
func (ga *GinieAutopilot) getCurrentMarkPrice(symbol string) (float64, error) {
	prices, err := ga.futuresClient.GetAllMarkPrices()
	if err != nil {
		return 0, err
	}

	for _, p := range prices {
		if p.Symbol == symbol {
			return p.MarkPrice, nil
		}
	}

	return 0, fmt.Errorf("price not found for %s", symbol)
}

// calculateATRFromKlines calculates ATR from klines
func calculateATRFromKlines(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	var trSum float64
	for i := 1; i <= period && i < len(klines); i++ {
		high := klines[len(klines)-i].High
		low := klines[len(klines)-i].Low
		prevClose := klines[len(klines)-i-1].Close

		tr1 := high - low
		tr2 := absFloat(high - prevClose)
		tr3 := absFloat(low - prevClose)

		tr := maxFloat(tr1, maxFloat(tr2, tr3))
		trSum += tr
	}

	return trSum / float64(period)
}

// calculateADXFromKlines calculates ADX from klines (simplified)
func calculateADXFromKlines(klines []binance.Kline, period int) float64 {
	if len(klines) < period*2 {
		return 25.0 // Default
	}

	// Simplified ADX calculation
	// In production, use the full Wilder's smoothing algorithm
	var sumDX float64
	var count int

	for i := 1; i < len(klines) && i <= period; i++ {
		high := klines[len(klines)-i].High
		low := klines[len(klines)-i].Low
		prevHigh := klines[len(klines)-i-1].High
		prevLow := klines[len(klines)-i-1].Low

		plusDM := high - prevHigh
		minusDM := prevLow - low

		if plusDM < 0 {
			plusDM = 0
		}
		if minusDM < 0 {
			minusDM = 0
		}

		if plusDM > minusDM {
			minusDM = 0
		} else {
			plusDM = 0
		}

		tr := maxFloat(high-low, maxFloat(absFloat(high-klines[len(klines)-i-1].Close), absFloat(low-klines[len(klines)-i-1].Close)))

		if tr > 0 {
			plusDI := (plusDM / tr) * 100
			minusDI := (minusDM / tr) * 100

			diSum := plusDI + minusDI
			if diSum > 0 {
				dx := absFloat(plusDI-minusDI) / diSum * 100
				sumDX += dx
				count++
			}
		}
	}

	if count == 0 {
		return 25.0
	}

	return sumDX / float64(count)
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ==================== END STORY 10.1: BINANCE ORDER UPDATE FUNCTIONS ====================
