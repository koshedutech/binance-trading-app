package autopilot

import (
	"binance-trading-bot/config"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
	"fmt"
	"sync"
	"time"
)

// HedgeTrigger represents the reason a hedge was initiated
type HedgeTrigger string

const (
	HedgeTriggerPriceDrop     HedgeTrigger = "price_drop"
	HedgeTriggerUnrealizedLoss HedgeTrigger = "unrealized_loss"
	HedgeTriggerAIRecommend   HedgeTrigger = "ai_recommendation"
	HedgeTriggerManual        HedgeTrigger = "manual"
)

// HedgeEvent tracks a hedging action
type HedgeEvent struct {
	Timestamp    time.Time    `json:"timestamp"`
	Trigger      HedgeTrigger `json:"trigger"`
	Action       string       `json:"action"` // "open", "close", "partial_close"
	HedgePercent float64      `json:"hedge_percent"`
	HedgePrice   float64      `json:"hedge_price"`
	Quantity     float64      `json:"quantity"`
	PnL          float64      `json:"pnl,omitempty"`
	Reason       string       `json:"reason"`
}

// HedgePositionInfo tracks the hedge position details
type HedgePositionInfo struct {
	Symbol         string       `json:"symbol"`
	Side           string       `json:"side"` // Opposite of main position
	EntryPrice     float64      `json:"entry_price"`
	Quantity       float64      `json:"quantity"`
	Leverage       int          `json:"leverage"`
	TriggerReason  HedgeTrigger `json:"trigger_reason"`
	TriggerPrice   float64      `json:"trigger_price"`   // Price at which hedge was triggered
	OpenTime       time.Time    `json:"open_time"`
	CurrentPnL     float64      `json:"current_pnl"`
	CurrentPnLPct  float64      `json:"current_pnl_pct"`
}

// HedgeablePosition extends FuturesAutopilotPosition with hedge tracking
type HedgeablePosition struct {
	*FuturesAutopilotPosition
	IsHedged         bool               `json:"is_hedged"`
	HedgePosition    *HedgePositionInfo `json:"hedge_position,omitempty"`
	HedgeHistory     []HedgeEvent       `json:"hedge_history,omitempty"`
	TotalHedgeProfit float64            `json:"total_hedge_profit"`
}

// HedgingManager manages all hedging operations
type HedgingManager struct {
	config        *config.FuturesAutopilotConfig
	futuresClient binance.FuturesClient
	logger        *logging.Logger

	// Hedge mode status
	hedgeModeEnabled bool
	hedgeModeChecked bool

	// Active hedges
	activeHedges map[string]*HedgePositionInfo
	hedgeHistory map[string][]HedgeEvent
	mu           sync.RWMutex
}

// NewHedgingManager creates a new hedging manager
func NewHedgingManager(
	cfg *config.FuturesAutopilotConfig,
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
) *HedgingManager {
	return &HedgingManager{
		config:        cfg,
		futuresClient: futuresClient,
		logger:        logger,
		activeHedges:  make(map[string]*HedgePositionInfo),
		hedgeHistory:  make(map[string][]HedgeEvent),
	}
}

// EnsureHedgeMode ensures Binance is in HEDGE position mode
func (hm *HedgingManager) EnsureHedgeMode() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Only check once per session
	if hm.hedgeModeChecked && hm.hedgeModeEnabled {
		return nil
	}

	// Get current position mode
	modeResp, err := hm.futuresClient.GetPositionMode()
	if err != nil {
		hm.logger.Warn("Failed to get position mode", "error", err)
		// Try to switch anyway
		if setErr := hm.futuresClient.SetPositionMode(true); setErr != nil {
			hm.logger.Error("Failed to enable HEDGE position mode",
				"error", setErr,
				"note", "This may fail if you have open positions")
			return fmt.Errorf("failed to enable hedge mode: %w", setErr)
		}
		hm.hedgeModeEnabled = true
		hm.hedgeModeChecked = true
		hm.logger.Info("Successfully enabled HEDGE position mode")
		return nil
	}

	if modeResp.DualSidePosition {
		hm.hedgeModeEnabled = true
		hm.hedgeModeChecked = true
		hm.logger.Info("Binance is already in HEDGE position mode")
		return nil
	}

	// Try to switch to hedge mode
	err = hm.futuresClient.SetPositionMode(true)
	if err != nil {
		hm.logger.Error("Failed to enable HEDGE position mode",
			"error", err,
			"note", "This may fail if you have open positions")
		return fmt.Errorf("failed to enable hedge mode: %w", err)
	}

	hm.hedgeModeEnabled = true
	hm.hedgeModeChecked = true
	hm.logger.Info("Successfully enabled HEDGE position mode")
	return nil
}

// IsHedgingEnabled returns whether hedging is enabled
func (hm *HedgingManager) IsHedgingEnabled() bool {
	return hm.config.HedgingEnabled
}

// SetHedgingEnabled enables or disables hedging
func (hm *HedgingManager) SetHedgingEnabled(enabled bool) {
	hm.config.HedgingEnabled = enabled
	hm.logger.Info("Hedging enabled status changed", "enabled", enabled)
}

// GetActiveHedgesCount returns the number of active hedges
func (hm *HedgingManager) GetActiveHedgesCount() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.activeHedges)
}

// EvaluateHedgingNeed checks if a position needs hedging
func (hm *HedgingManager) EvaluateHedgingNeed(
	symbol string,
	position *FuturesAutopilotPosition,
	currentPrice float64,
	aiRecommendHedge bool,
	aiConfidence float64,
) (needsHedge bool, trigger HedgeTrigger, hedgePercent float64) {

	if !hm.config.HedgingEnabled {
		return false, "", 0
	}

	// Check if already at max simultaneous hedges
	hm.mu.RLock()
	activeCount := len(hm.activeHedges)
	alreadyHedged := hm.activeHedges[symbol] != nil
	hm.mu.RUnlock()

	if alreadyHedged {
		return false, "", 0
	}

	if activeCount >= hm.config.HedgeMaxSimultaneous {
		return false, "", 0
	}

	// Calculate position loss
	var lossPct float64
	var unrealizedLoss float64

	if position.Side == "LONG" {
		lossPct = (position.EntryPrice - currentPrice) / position.EntryPrice * 100
		unrealizedLoss = (position.EntryPrice - currentPrice) * position.Quantity
	} else {
		lossPct = (currentPrice - position.EntryPrice) / position.EntryPrice * 100
		unrealizedLoss = (currentPrice - position.EntryPrice) * position.Quantity
	}

	// Only consider hedging if we're in a losing position
	if unrealizedLoss <= 0 {
		return false, "", 0
	}

	// Check AI recommendation (highest priority)
	if hm.config.HedgeAIEnabled && aiRecommendHedge && aiConfidence >= hm.config.HedgeAIConfidenceMin {
		hm.logger.Info("AI recommends hedging",
			"symbol", symbol,
			"confidence", aiConfidence,
			"loss_pct", lossPct)
		return true, HedgeTriggerAIRecommend, hm.getHedgePercentForLoss(lossPct)
	}

	// Check unrealized loss threshold
	if unrealizedLoss >= hm.config.HedgeUnrealizedLossTrigger {
		hm.logger.Info("Unrealized loss threshold reached",
			"symbol", symbol,
			"loss", unrealizedLoss,
			"threshold", hm.config.HedgeUnrealizedLossTrigger)
		return true, HedgeTriggerUnrealizedLoss, hm.getHedgePercentForLoss(lossPct)
	}

	// Check price drop percentage
	if lossPct >= hm.config.HedgePriceDropTriggerPct {
		hm.logger.Info("Price drop threshold reached",
			"symbol", symbol,
			"loss_pct", lossPct,
			"threshold", hm.config.HedgePriceDropTriggerPct)
		return true, HedgeTriggerPriceDrop, hm.getHedgePercentForLoss(lossPct)
	}

	return false, "", 0
}

// getHedgePercentForLoss calculates the appropriate hedge percentage based on loss
func (hm *HedgingManager) getHedgePercentForLoss(lossPct float64) float64 {
	steps := hm.config.HedgePartialSteps
	if len(steps) == 0 {
		return hm.config.HedgeDefaultPercent
	}

	// Graduate based on loss percentage
	// 5% loss = first step, 10% = second step, etc.
	triggerPct := hm.config.HedgePriceDropTriggerPct
	stepMultiple := int(lossPct / triggerPct)

	if stepMultiple >= len(steps) {
		return steps[len(steps)-1]
	}

	return steps[stepMultiple]
}

// ExecuteHedge opens a hedge position opposite to the main position
func (hm *HedgingManager) ExecuteHedge(
	symbol string,
	position *FuturesAutopilotPosition,
	hedgePercent float64,
	trigger HedgeTrigger,
	dryRun bool,
) (*HedgePositionInfo, error) {

	// Ensure hedge mode is enabled
	if err := hm.EnsureHedgeMode(); err != nil {
		return nil, fmt.Errorf("cannot execute hedge without HEDGE mode: %w", err)
	}

	currentPrice, err := hm.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get current price: %w", err)
	}

	// Calculate hedge quantity
	hedgeQty := position.Quantity * (hedgePercent / 100)
	hedgeQty = roundQuantity(symbol, hedgeQty)

	// Determine hedge side (opposite of main position)
	var hedgeSide string
	var positionSide binance.PositionSide
	if position.Side == "LONG" {
		hedgeSide = "SHORT"
		positionSide = binance.PositionSideShort
	} else {
		hedgeSide = "LONG"
		positionSide = binance.PositionSideLong
	}

	hm.logger.Info("Executing hedge position",
		"symbol", symbol,
		"main_side", position.Side,
		"hedge_side", hedgeSide,
		"hedge_qty", hedgeQty,
		"hedge_percent", hedgePercent,
		"trigger", trigger,
		"dry_run", dryRun)

	hedgeInfo := &HedgePositionInfo{
		Symbol:        symbol,
		Side:          hedgeSide,
		EntryPrice:    currentPrice,
		Quantity:      hedgeQty,
		Leverage:      position.Leverage,
		TriggerReason: trigger,
		TriggerPrice:  currentPrice,
		OpenTime:      time.Now(),
	}

	if !dryRun {
		// Place hedge order
		orderSide := "BUY"
		if hedgeSide == "SHORT" {
			orderSide = "SELL"
		}

		orderParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         orderSide,
			PositionSide: positionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     hedgeQty,
		}

		order, err := hm.futuresClient.PlaceFuturesOrder(orderParams)
		if err != nil {
			return nil, fmt.Errorf("failed to place hedge order: %w", err)
		}

		hedgeInfo.EntryPrice = order.AvgPrice
		hm.logger.Info("Hedge order placed successfully",
			"symbol", symbol,
			"order_id", order.OrderId,
			"avg_price", order.AvgPrice)
	}

	// Track the hedge
	hm.mu.Lock()
	hm.activeHedges[symbol] = hedgeInfo
	hm.hedgeHistory[symbol] = append(hm.hedgeHistory[symbol], HedgeEvent{
		Timestamp:    time.Now(),
		Trigger:      trigger,
		Action:       "open",
		HedgePercent: hedgePercent,
		HedgePrice:   hedgeInfo.EntryPrice,
		Quantity:     hedgeQty,
		Reason:       fmt.Sprintf("Hedge triggered by %s", trigger),
	})
	hm.mu.Unlock()

	return hedgeInfo, nil
}

// MonitorHedge checks if a hedge should be closed (take profit or recovery)
func (hm *HedgingManager) MonitorHedge(
	symbol string,
	mainPosition *FuturesAutopilotPosition,
	currentPrice float64,
) (shouldClose bool, reason string) {

	hm.mu.RLock()
	hedge, exists := hm.activeHedges[symbol]
	hm.mu.RUnlock()

	if !exists {
		return false, ""
	}

	// Calculate hedge PnL
	var hedgePnL float64
	var hedgePnLPct float64
	if hedge.Side == "LONG" {
		hedgePnL = (currentPrice - hedge.EntryPrice) * hedge.Quantity
		hedgePnLPct = (currentPrice - hedge.EntryPrice) / hedge.EntryPrice * 100
	} else {
		hedgePnL = (hedge.EntryPrice - currentPrice) * hedge.Quantity
		hedgePnLPct = (hedge.EntryPrice - currentPrice) / hedge.EntryPrice * 100
	}

	// Update hedge current PnL
	hm.mu.Lock()
	hedge.CurrentPnL = hedgePnL
	hedge.CurrentPnLPct = hedgePnLPct
	hm.mu.Unlock()

	// Check if hedge has hit profit target
	if hedgePnLPct >= hm.config.HedgeProfitTakePct {
		return true, fmt.Sprintf("hedge_profit_target_%.2f%%", hedgePnLPct)
	}

	// Check if main position has recovered enough to close hedge
	var mainPnLPct float64
	if mainPosition.Side == "LONG" {
		mainPnLPct = (currentPrice - mainPosition.EntryPrice) / mainPosition.EntryPrice * 100
	} else {
		mainPnLPct = (mainPosition.EntryPrice - currentPrice) / mainPosition.EntryPrice * 100
	}

	// If main position has recovered past the trigger point
	if mainPnLPct >= -hm.config.HedgeCloseOnRecoveryPct {
		return true, fmt.Sprintf("main_position_recovered_%.2f%%", mainPnLPct)
	}

	return false, ""
}

// CloseHedge closes a hedge position
func (hm *HedgingManager) CloseHedge(symbol string, reason string, dryRun bool) (float64, error) {
	hm.mu.Lock()
	hedge, exists := hm.activeHedges[symbol]
	if !exists {
		hm.mu.Unlock()
		return 0, fmt.Errorf("no active hedge for %s", symbol)
	}
	delete(hm.activeHedges, symbol)
	hm.mu.Unlock()

	currentPrice, err := hm.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get current price: %w", err)
	}

	// Calculate hedge PnL
	var pnl float64
	if hedge.Side == "LONG" {
		pnl = (currentPrice - hedge.EntryPrice) * hedge.Quantity
	} else {
		pnl = (hedge.EntryPrice - currentPrice) * hedge.Quantity
	}

	hm.logger.Info("Closing hedge position",
		"symbol", symbol,
		"reason", reason,
		"hedge_side", hedge.Side,
		"entry", hedge.EntryPrice,
		"exit", currentPrice,
		"pnl", pnl,
		"dry_run", dryRun)

	if !dryRun {
		// Place close order
		var closeSide string
		var positionSide binance.PositionSide
		if hedge.Side == "LONG" {
			closeSide = "SELL"
			positionSide = binance.PositionSideLong
		} else {
			closeSide = "BUY"
			positionSide = binance.PositionSideShort
		}

		closeParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         closeSide,
			PositionSide: positionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     hedge.Quantity,
			ReduceOnly:   true,
		}

		_, err := hm.futuresClient.PlaceFuturesOrder(closeParams)
		if err != nil {
			hm.logger.Error("Failed to close hedge order", "symbol", symbol, "error", err)
			return 0, fmt.Errorf("failed to close hedge: %w", err)
		}
	}

	// Record hedge close event
	hm.mu.Lock()
	hm.hedgeHistory[symbol] = append(hm.hedgeHistory[symbol], HedgeEvent{
		Timestamp: time.Now(),
		Trigger:   hedge.TriggerReason,
		Action:    "close",
		HedgePrice: currentPrice,
		Quantity:  hedge.Quantity,
		PnL:       pnl,
		Reason:    reason,
	})
	hm.mu.Unlock()

	return pnl, nil
}

// GetHedgeStatus returns the current status of all hedges
func (hm *HedgingManager) GetHedgeStatus() map[string]interface{} {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	activeHedges := make([]map[string]interface{}, 0)
	for symbol, hedge := range hm.activeHedges {
		activeHedges = append(activeHedges, map[string]interface{}{
			"symbol":         symbol,
			"side":           hedge.Side,
			"entry_price":    hedge.EntryPrice,
			"quantity":       hedge.Quantity,
			"trigger":        hedge.TriggerReason,
			"trigger_price":  hedge.TriggerPrice,
			"current_pnl":    hedge.CurrentPnL,
			"current_pnl_pct": hedge.CurrentPnLPct,
			"open_time":      hedge.OpenTime,
		})
	}

	return map[string]interface{}{
		"enabled":             hm.config.HedgingEnabled,
		"hedge_mode_enabled":  hm.hedgeModeEnabled,
		"active_hedges":       activeHedges,
		"active_count":        len(hm.activeHedges),
		"max_simultaneous":    hm.config.HedgeMaxSimultaneous,
		"price_drop_trigger":  hm.config.HedgePriceDropTriggerPct,
		"loss_trigger":        hm.config.HedgeUnrealizedLossTrigger,
		"ai_enabled":          hm.config.HedgeAIEnabled,
		"default_percent":     hm.config.HedgeDefaultPercent,
		"profit_take_pct":     hm.config.HedgeProfitTakePct,
		"close_on_recovery":   hm.config.HedgeCloseOnRecoveryPct,
	}
}

// GetHedgeHistory returns hedge history for a symbol
func (hm *HedgingManager) GetHedgeHistory(symbol string) []HedgeEvent {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	history := hm.hedgeHistory[symbol]
	result := make([]HedgeEvent, len(history))
	copy(result, history)
	return result
}

// IsSymbolHedged returns whether a symbol has an active hedge
func (hm *HedgingManager) IsSymbolHedged(symbol string) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.activeHedges[symbol] != nil
}

// GetActiveHedge returns the active hedge for a symbol if any
func (hm *HedgingManager) GetActiveHedge(symbol string) *HedgePositionInfo {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.activeHedges[symbol]
}

// UpdateConfig updates the hedging configuration
func (hm *HedgingManager) UpdateConfig(cfg *config.FuturesAutopilotConfig) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.config = cfg
	hm.logger.Info("Hedging configuration updated",
		"enabled", cfg.HedgingEnabled,
		"price_drop_trigger", cfg.HedgePriceDropTriggerPct,
		"ai_enabled", cfg.HedgeAIEnabled)
}

// UpdateSettings updates individual hedging settings
func (hm *HedgingManager) UpdateSettings(
	enabled *bool,
	priceDropTriggerPct *float64,
	unrealizedLossTrigger *float64,
	aiEnabled *bool,
	aiConfidenceMin *float64,
	defaultPercent *float64,
	partialSteps []float64,
	profitTakePct *float64,
	closeOnRecoveryPct *float64,
	maxSimultaneous *int,
) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if enabled != nil {
		hm.config.HedgingEnabled = *enabled
	}
	if priceDropTriggerPct != nil {
		hm.config.HedgePriceDropTriggerPct = *priceDropTriggerPct
	}
	if unrealizedLossTrigger != nil {
		hm.config.HedgeUnrealizedLossTrigger = *unrealizedLossTrigger
	}
	if aiEnabled != nil {
		hm.config.HedgeAIEnabled = *aiEnabled
	}
	if aiConfidenceMin != nil {
		hm.config.HedgeAIConfidenceMin = *aiConfidenceMin
	}
	if defaultPercent != nil {
		hm.config.HedgeDefaultPercent = *defaultPercent
	}
	if len(partialSteps) > 0 {
		hm.config.HedgePartialSteps = partialSteps
	}
	if profitTakePct != nil {
		hm.config.HedgeProfitTakePct = *profitTakePct
	}
	if closeOnRecoveryPct != nil {
		hm.config.HedgeCloseOnRecoveryPct = *closeOnRecoveryPct
	}
	if maxSimultaneous != nil {
		hm.config.HedgeMaxSimultaneous = *maxSimultaneous
	}

	hm.logger.Info("Hedging settings updated",
		"enabled", hm.config.HedgingEnabled,
		"price_drop_trigger", hm.config.HedgePriceDropTriggerPct)
}

// ClearAllHedges closes all active hedges (for emergency use)
func (hm *HedgingManager) ClearAllHedges(dryRun bool) error {
	hm.mu.Lock()
	symbols := make([]string, 0, len(hm.activeHedges))
	for symbol := range hm.activeHedges {
		symbols = append(symbols, symbol)
	}
	hm.mu.Unlock()

	for _, symbol := range symbols {
		_, err := hm.CloseHedge(symbol, "emergency_clear_all", dryRun)
		if err != nil {
			hm.logger.Error("Failed to close hedge", "symbol", symbol, "error", err)
		}
	}

	return nil
}
