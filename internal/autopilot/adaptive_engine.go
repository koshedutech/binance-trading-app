package autopilot

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
	"fmt"
	"math"
	"sync"
	"time"
)

// MarketContext holds current market state for adaptive decisions
type MarketContext struct {
	// Volatility
	CurrentATR      float64 `json:"current_atr"`
	AvgATR          float64 `json:"avg_atr"`
	VolatilityRatio float64 `json:"volatility_ratio"` // Current/Avg - >1.5 = high volatility

	// Fear & Greed
	FearGreedIndex int    `json:"fear_greed_index"`
	FearGreedLabel string `json:"fear_greed_label"`

	// Recent Performance
	RecentWins      int     `json:"recent_wins"`
	RecentLosses    int     `json:"recent_losses"`
	WinStreak       int     `json:"win_streak"`
	LossStreak      int     `json:"loss_streak"`
	RecentPnL       float64 `json:"recent_pnl"`
	DailyPnL        float64 `json:"daily_pnl"`
	DailyDrawdown   float64 `json:"daily_drawdown"`
	MaxDailyLoss    float64 `json:"max_daily_loss"`

	// Account Health
	MarginRatio     float64 `json:"margin_ratio"` // Used margin / Total balance
	AvailableMargin float64 `json:"available_margin"`

	// Time Context
	Hour        int  `json:"hour"`
	IsWeekend   bool `json:"is_weekend"`
	IsLowVolume bool `json:"is_low_volume"` // 0-4 UTC typically

	// Position Context
	OpenPositions     int     `json:"open_positions"`
	MaxPositions      int     `json:"max_positions"`
	TotalExposure     float64 `json:"total_exposure"`
	MaxExposureUSD    float64 `json:"max_exposure_usd"`

	// Trend Context
	MarketTrend       string  `json:"market_trend"` // "bullish", "bearish", "ranging"
	TrendStrength     float64 `json:"trend_strength"`
	BTCTrend          string  `json:"btc_trend"` // BTC often leads market
}

// AdaptiveDecision represents the engine's decision with reasoning
type AdaptiveDecision struct {
	// Core Decision
	Action     string  `json:"action"`      // "open_long", "open_short", "hold", "average_down", "hedge", "close"
	Approved   bool    `json:"approved"`
	Confidence float64 `json:"confidence"`

	// Adaptive Adjustments
	PositionSizeMultiplier float64 `json:"position_size_multiplier"` // 0.5-1.5 based on context
	LeverageAdjustment     int     `json:"leverage_adjustment"`      // Reduce leverage in high volatility

	// Risk Parameters
	SuggestedStopLoss   float64 `json:"suggested_stop_loss"`
	SuggestedTakeProfit float64 `json:"suggested_take_profit"`

	// For Existing Positions
	ShouldAverage   bool    `json:"should_average"`
	AveragePercent  float64 `json:"average_percent"`
	ShouldHedge     bool    `json:"should_hedge"`
	HedgePercent    float64 `json:"hedge_percent"`
	ShouldClose     bool    `json:"should_close"`
	CloseReason     string  `json:"close_reason"`

	// Reasoning (human-readable)
	PrimaryReason   string   `json:"primary_reason"`
	ContextFactors  []string `json:"context_factors"`
	RiskWarnings    []string `json:"risk_warnings"`

	// Signal Analysis
	SignalStrength    float64                      `json:"signal_strength"`
	SignalConfluence  int                          `json:"signal_confluence"`
	SignalBreakdown   map[string]SignalContribution `json:"signal_breakdown"`
}

// AdaptiveDecisionEngine makes human-like trading decisions
type AdaptiveDecisionEngine struct {
	// Core dependencies
	signalAggregator *SignalAggregator
	futuresClient    binance.FuturesClient
	logger           *logging.Logger

	// Current context
	context     *MarketContext
	contextLock sync.RWMutex

	// Trading history for adaptation
	recentTrades   []TradeResult
	tradeHistoryMu sync.RWMutex

	// Configuration
	styleConfig *TradingStyleConfig
	styleLock   sync.RWMutex

	// Cool-down tracking
	lastTradeTime   map[string]time.Time
	cooldownPeriod  time.Duration
	cooldownLock    sync.RWMutex
}

// TradeResult tracks recent trade outcomes for adaptation
type TradeResult struct {
	Symbol    string    `json:"symbol"`
	Direction string    `json:"direction"`
	PnL       float64   `json:"pnl"`
	IsWin     bool      `json:"is_win"`
	Timestamp time.Time `json:"timestamp"`
}

// NewAdaptiveDecisionEngine creates a new adaptive engine
func NewAdaptiveDecisionEngine(
	signalAggregator *SignalAggregator,
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
) *AdaptiveDecisionEngine {
	return &AdaptiveDecisionEngine{
		signalAggregator: signalAggregator,
		futuresClient:    futuresClient,
		logger:           logger,
		context:          &MarketContext{},
		recentTrades:     make([]TradeResult, 0, 50),
		lastTradeTime:    make(map[string]time.Time),
		cooldownPeriod:   2 * time.Minute, // Default 2 min cooldown per symbol
	}
}

// SetStyleConfig updates the trading style configuration
func (ade *AdaptiveDecisionEngine) SetStyleConfig(config *TradingStyleConfig) {
	ade.styleLock.Lock()
	defer ade.styleLock.Unlock()
	ade.styleConfig = config
}

// UpdateContext updates the market context
func (ade *AdaptiveDecisionEngine) UpdateContext(ctx *MarketContext) {
	ade.contextLock.Lock()
	defer ade.contextLock.Unlock()
	ade.context = ctx
}

// SetMarketContext is an alias for UpdateContext (for convenience)
func (ade *AdaptiveDecisionEngine) SetMarketContext(ctx *MarketContext) {
	ade.UpdateContext(ctx)
}

// RecordTradeResult records a trade outcome for adaptation
func (ade *AdaptiveDecisionEngine) RecordTradeResult(result TradeResult) {
	ade.tradeHistoryMu.Lock()
	defer ade.tradeHistoryMu.Unlock()

	// Keep last 50 trades
	if len(ade.recentTrades) >= 50 {
		ade.recentTrades = ade.recentTrades[1:]
	}
	ade.recentTrades = append(ade.recentTrades, result)

	// Update context with recent performance
	ade.updateRecentPerformance()
}

// updateRecentPerformance calculates win/loss streaks and recent PnL
func (ade *AdaptiveDecisionEngine) updateRecentPerformance() {
	ade.contextLock.Lock()
	defer ade.contextLock.Unlock()

	// Calculate last 10 trades performance
	wins := 0
	losses := 0
	recentPnL := 0.0
	winStreak := 0
	lossStreak := 0
	countingWins := true
	countingLosses := true

	startIdx := len(ade.recentTrades) - 10
	if startIdx < 0 {
		startIdx = 0
	}

	for i := len(ade.recentTrades) - 1; i >= startIdx; i-- {
		trade := ade.recentTrades[i]
		recentPnL += trade.PnL

		if trade.IsWin {
			wins++
			if countingWins {
				winStreak++
			}
			countingLosses = false
		} else {
			losses++
			if countingLosses {
				lossStreak++
			}
			countingWins = false
		}
	}

	ade.context.RecentWins = wins
	ade.context.RecentLosses = losses
	ade.context.RecentPnL = recentPnL
	ade.context.WinStreak = winStreak
	ade.context.LossStreak = lossStreak
}

// MakeDecision makes an adaptive trading decision
func (ade *AdaptiveDecisionEngine) MakeDecision(
	symbol string,
	currentPrice float64,
	klines []binance.Kline,
	existingPosition *FuturesAutopilotPosition,
) *AdaptiveDecision {
	decision := &AdaptiveDecision{
		Action:                 "hold",
		Approved:               false,
		PositionSizeMultiplier: 1.0,
		ContextFactors:         make([]string, 0),
		RiskWarnings:           make([]string, 0),
		SignalBreakdown:        make(map[string]SignalContribution),
	}

	// Get current style config
	ade.styleLock.RLock()
	styleConfig := ade.styleConfig
	ade.styleLock.RUnlock()

	if styleConfig == nil {
		decision.PrimaryReason = "Trading style not configured"
		return decision
	}

	// Get current context
	ade.contextLock.RLock()
	ctx := ade.context
	ade.contextLock.RUnlock()

	// === PHASE 1: Pre-trade Risk Checks ===
	if !ade.passesRiskChecks(ctx, decision, symbol) {
		return decision
	}

	// === PHASE 2: Existing Position Analysis ===
	if existingPosition != nil {
		return ade.analyzeExistingPosition(existingPosition, currentPrice, klines, ctx, styleConfig, decision)
	}

	// === PHASE 3: New Position Analysis ===
	return ade.analyzeNewPosition(symbol, currentPrice, klines, ctx, styleConfig, decision)
}

// passesRiskChecks performs pre-trade risk evaluation
func (ade *AdaptiveDecisionEngine) passesRiskChecks(ctx *MarketContext, decision *AdaptiveDecision, symbol string) bool {
	// Check daily drawdown limit (only if limit is actually set)
	if ctx.MaxDailyLoss > 0 && ctx.DailyDrawdown >= ctx.MaxDailyLoss {
		decision.PrimaryReason = "Daily loss limit reached"
		decision.RiskWarnings = append(decision.RiskWarnings,
			fmt.Sprintf("Daily drawdown %.2f%% exceeds limit %.2f%%", ctx.DailyDrawdown, ctx.MaxDailyLoss))
		return false
	}

	// Check loss streak (human would take a break after 3+ consecutive losses)
	if ctx.LossStreak >= 3 {
		decision.PrimaryReason = "On losing streak - waiting for better setup"
		decision.RiskWarnings = append(decision.RiskWarnings,
			fmt.Sprintf("Losing streak of %d trades - reducing activity", ctx.LossStreak))
		decision.PositionSizeMultiplier = 0.5 // Reduce size significantly
	}

	// Check margin usage
	if ctx.MarginRatio > 0.7 {
		decision.PrimaryReason = "Margin usage too high"
		decision.RiskWarnings = append(decision.RiskWarnings,
			fmt.Sprintf("Margin ratio %.2f%% - reduce exposure", ctx.MarginRatio*100))
		return false
	}

	// Check max positions
	if ctx.OpenPositions >= ctx.MaxPositions {
		decision.PrimaryReason = "Maximum positions reached"
		return false
	}

	// Check volatility (avoid trading in extreme volatility)
	if ctx.VolatilityRatio > 2.0 {
		decision.RiskWarnings = append(decision.RiskWarnings,
			fmt.Sprintf("High volatility (%.1fx normal) - reducing position size", ctx.VolatilityRatio))
		decision.PositionSizeMultiplier *= 0.5
	}

	// Check cooldown per symbol
	ade.cooldownLock.RLock()
	lastTrade, exists := ade.lastTradeTime[symbol]
	ade.cooldownLock.RUnlock()

	if exists && time.Since(lastTrade) < ade.cooldownPeriod {
		remaining := ade.cooldownPeriod - time.Since(lastTrade)
		decision.PrimaryReason = fmt.Sprintf("Symbol cooldown - wait %.0fs", remaining.Seconds())
		decision.ContextFactors = append(decision.ContextFactors, "Cooldown active for this symbol")
		return false
	}

	// Check low volume hours (0-4 UTC)
	if ctx.IsLowVolume {
		decision.ContextFactors = append(decision.ContextFactors, "Low volume period - extra caution")
		decision.PositionSizeMultiplier *= 0.7
	}

	return true
}

// analyzeExistingPosition makes decisions about existing positions
func (ade *AdaptiveDecisionEngine) analyzeExistingPosition(
	pos *FuturesAutopilotPosition,
	currentPrice float64,
	klines []binance.Kline,
	ctx *MarketContext,
	styleConfig *TradingStyleConfig,
	decision *AdaptiveDecision,
) *AdaptiveDecision {
	// Calculate current P&L percentage
	pnlPercent := 0.0
	if pos.EntryPrice > 0 {
		if pos.Side == "LONG" {
			pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		} else {
			pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
		}
	}

	decision.ContextFactors = append(decision.ContextFactors,
		fmt.Sprintf("Existing %s position, P&L: %.2f%%", pos.Side, pnlPercent))

	// === Check if position is in profit ===
	if pnlPercent > 0 {
		return ade.handleProfitablePosition(pos, pnlPercent, currentPrice, klines, styleConfig, decision)
	}

	// === Position is in loss ===
	return ade.handleLosingPosition(pos, pnlPercent, currentPrice, klines, ctx, styleConfig, decision)
}

// handleProfitablePosition makes decisions about profitable positions
func (ade *AdaptiveDecisionEngine) handleProfitablePosition(
	pos *FuturesAutopilotPosition,
	pnlPercent float64,
	currentPrice float64,
	klines []binance.Kline,
	styleConfig *TradingStyleConfig,
	decision *AdaptiveDecision,
) *AdaptiveDecision {
	// For scalping, quick profit is key
	if styleConfig.Style == StyleScalping && styleConfig.QuickProfitEnabled {
		if pnlPercent >= styleConfig.QuickProfitPercent {
			decision.Action = "close"
			decision.ShouldClose = true
			decision.CloseReason = fmt.Sprintf("Quick profit target %.2f%% reached", styleConfig.QuickProfitPercent)
			decision.Approved = true
			decision.PrimaryReason = "Scalping profit target hit"
			return decision
		}
	}

	// Check if signals are reversing
	signals, _ := ade.signalAggregator.CollectAllSignals(pos.Symbol, currentPrice, klines, styleConfig.Style)

	oppositeSignals := 0
	for _, sig := range signals {
		if (pos.Side == "LONG" && sig.Direction == "short") ||
			(pos.Side == "SHORT" && sig.Direction == "long") {
			oppositeSignals++
		}
	}

	// If majority signals reversing and we have decent profit, consider closing
	if oppositeSignals >= len(signals)/2 && pnlPercent > 1.0 {
		decision.Action = "close"
		decision.ShouldClose = true
		decision.CloseReason = "Signals reversing - securing profit"
		decision.Approved = true
		decision.PrimaryReason = fmt.Sprintf("Secured %.2f%% profit on signal reversal", pnlPercent)
		return decision
	}

	// Let winners run (trailing stop should handle this)
	decision.Action = "hold"
	decision.PrimaryReason = fmt.Sprintf("Letting winner run (%.2f%% profit)", pnlPercent)
	return decision
}

// handleLosingPosition makes decisions about losing positions
func (ade *AdaptiveDecisionEngine) handleLosingPosition(
	pos *FuturesAutopilotPosition,
	pnlPercent float64,
	currentPrice float64,
	klines []binance.Kline,
	ctx *MarketContext,
	styleConfig *TradingStyleConfig,
	decision *AdaptiveDecision,
) *AdaptiveDecision {
	lossPercent := math.Abs(pnlPercent)

	// === Consider Hedging (Position trading only) ===
	if styleConfig.AllowHedging && lossPercent >= 5.0 {
		// Check if opposite signal is strong
		signals, _ := ade.signalAggregator.CollectAllSignals(pos.Symbol, currentPrice, klines, styleConfig.Style)

		oppositeConfidence := 0.0
		oppositeCount := 0
		for _, sig := range signals {
			if (pos.Side == "LONG" && sig.Direction == "short") ||
				(pos.Side == "SHORT" && sig.Direction == "long") {
				oppositeConfidence += sig.Confidence
				oppositeCount++
			}
		}

		if oppositeCount > 0 {
			avgOppositeConfidence := oppositeConfidence / float64(oppositeCount)

			// Only hedge if opposite signals are strong (75%+)
			if avgOppositeConfidence >= 0.75 && oppositeCount >= 2 {
				decision.Action = "hedge"
				decision.ShouldHedge = true
				decision.HedgePercent = 50.0 // Start with 50% hedge
				if lossPercent >= 10.0 {
					decision.HedgePercent = 75.0 // Increase hedge if deeper in loss
				}
				decision.Approved = true
				decision.PrimaryReason = fmt.Sprintf("Hedging %.0f%% - opposite signals strong (%.0f%% confidence)",
					decision.HedgePercent, avgOppositeConfidence*100)
				decision.ContextFactors = append(decision.ContextFactors,
					fmt.Sprintf("Position down %.2f%%, %d opposite signals", lossPercent, oppositeCount))
				return decision
			}
		}
	}

	// === Consider Averaging Down (Swing/Position only) ===
	if styleConfig.AllowAveraging && pos.EntryCount < styleConfig.MaxEntries {
		// Only average if original signals STILL agree with our direction
		signals, _ := ade.signalAggregator.CollectAllSignals(pos.Symbol, currentPrice, klines, styleConfig.Style)

		sameDirectionCount := 0
		sameDirectionConfidence := 0.0
		for _, sig := range signals {
			if (pos.Side == "LONG" && sig.Direction == "long") ||
				(pos.Side == "SHORT" && sig.Direction == "short") {
				sameDirectionCount++
				sameDirectionConfidence += sig.Confidence
			}
		}

		// Require majority of signals still agreeing with higher confidence
		requiredConfidence := 0.70 // Higher bar for averaging
		if sameDirectionCount >= len(signals)/2 && len(signals) > 0 {
			avgConfidence := sameDirectionConfidence / float64(sameDirectionCount)

			if avgConfidence >= requiredConfidence {
				// Check we're at a reasonable loss level for averaging (2-8%)
				if lossPercent >= 2.0 && lossPercent <= 8.0 {
					decision.Action = "average_down"
					decision.ShouldAverage = true
					decision.AveragePercent = 50.0 // Add 50% to position
					decision.Approved = true
					decision.Confidence = avgConfidence
					decision.PrimaryReason = fmt.Sprintf("Averaging down - %d/%d signals still agree (%.0f%% confidence)",
						sameDirectionCount, len(signals), avgConfidence*100)
					decision.ContextFactors = append(decision.ContextFactors,
						fmt.Sprintf("Entry #%d of max %d", pos.EntryCount+1, styleConfig.MaxEntries))
					return decision
				}
			}
		}
	}

	// === Just Hold ===
	decision.Action = "hold"
	decision.PrimaryReason = fmt.Sprintf("Holding losing position (%.2f%% loss) - waiting for reversal or SL", lossPercent)
	return decision
}

// analyzeNewPosition makes decisions about opening new positions
func (ade *AdaptiveDecisionEngine) analyzeNewPosition(
	symbol string,
	currentPrice float64,
	klines []binance.Kline,
	ctx *MarketContext,
	styleConfig *TradingStyleConfig,
	decision *AdaptiveDecision,
) *AdaptiveDecision {
	// Collect signals
	signals, llmAnalysis := ade.signalAggregator.CollectAllSignals(symbol, currentPrice, klines, styleConfig.Style)

	if len(signals) == 0 {
		decision.PrimaryReason = "No signals available"
		return decision
	}

	// Convert to breakdown for logging
	for _, sig := range signals {
		decision.SignalBreakdown[string(sig.Source)] = SignalContribution{
			Direction:  sig.Direction,
			Confidence: sig.Confidence,
			Reasoning:  sig.Reasoning,
		}
	}

	// Use aggregator for base decision
	action, confidence, approved, reason := ade.signalAggregator.AggregateDecision(
		signals,
		styleConfig.Style,
		styleConfig.MinConfidence,
		styleConfig.RequiredConfluence,
	)

	decision.Action = action
	decision.Confidence = confidence
	decision.SignalConfluence = countDirectionalSignals(signals, action)

	if !approved {
		decision.Approved = false
		decision.PrimaryReason = reason
		return decision
	}

	// === Apply Adaptive Adjustments ===

	// 1. Boost confidence if on winning streak
	if ctx.WinStreak >= 3 {
		decision.ContextFactors = append(decision.ContextFactors,
			fmt.Sprintf("Winning streak of %d - signal confidence boosted", ctx.WinStreak))
		confidence = math.Min(confidence*1.1, 0.95) // 10% boost, max 95%
	}

	// 2. Reduce size if on losing streak
	if ctx.LossStreak >= 2 {
		reduction := 0.2 * float64(ctx.LossStreak) // 20% per loss
		decision.PositionSizeMultiplier *= (1.0 - reduction)
		decision.ContextFactors = append(decision.ContextFactors,
			fmt.Sprintf("Losing streak of %d - size reduced to %.0f%%",
				ctx.LossStreak, decision.PositionSizeMultiplier*100))
	}

	// 3. Adjust for volatility
	if ctx.VolatilityRatio > 1.5 {
		// High volatility - reduce size, widen stops
		decision.PositionSizeMultiplier *= 0.7
		decision.LeverageAdjustment = -2 // Reduce leverage by 2
		decision.ContextFactors = append(decision.ContextFactors,
			fmt.Sprintf("High volatility (%.1fx) - reduced exposure", ctx.VolatilityRatio))
	} else if ctx.VolatilityRatio < 0.5 {
		// Low volatility - can be more aggressive
		decision.PositionSizeMultiplier *= 1.2
		decision.ContextFactors = append(decision.ContextFactors,
			"Low volatility - increased position size")
	}

	// 4. Check BTC trend alignment (crypto-specific)
	if ctx.BTCTrend != "" {
		btcAligned := (action == "open_long" && ctx.BTCTrend == "bullish") ||
			(action == "open_short" && ctx.BTCTrend == "bearish")
		if !btcAligned {
			decision.RiskWarnings = append(decision.RiskWarnings,
				fmt.Sprintf("Trading against BTC trend (%s)", ctx.BTCTrend))
			decision.PositionSizeMultiplier *= 0.8
		} else {
			decision.ContextFactors = append(decision.ContextFactors,
				fmt.Sprintf("Aligned with BTC trend (%s)", ctx.BTCTrend))
		}
	}

	// 5. Fear & Greed context
	if ctx.FearGreedIndex > 0 {
		if ctx.FearGreedIndex <= 25 && action == "open_long" {
			decision.ContextFactors = append(decision.ContextFactors,
				"Extreme fear - contrarian long may work")
			decision.PositionSizeMultiplier *= 1.1
		} else if ctx.FearGreedIndex >= 75 && action == "open_short" {
			decision.ContextFactors = append(decision.ContextFactors,
				"Extreme greed - contrarian short may work")
			decision.PositionSizeMultiplier *= 1.1
		}
	}

	// 6. Use LLM analysis for SL/TP if available
	if llmAnalysis != nil {
		if llmAnalysis.StopLoss != nil && *llmAnalysis.StopLoss > 0 {
			decision.SuggestedStopLoss = *llmAnalysis.StopLoss
		}
		if llmAnalysis.TakeProfit != nil && *llmAnalysis.TakeProfit > 0 {
			decision.SuggestedTakeProfit = *llmAnalysis.TakeProfit
		}
	}

	// Cap position size multiplier
	if decision.PositionSizeMultiplier < 0.3 {
		decision.PositionSizeMultiplier = 0.3
	} else if decision.PositionSizeMultiplier > 1.5 {
		decision.PositionSizeMultiplier = 1.5
	}

	// Final decision
	decision.Approved = true
	decision.Confidence = confidence
	decision.SignalStrength = confidence
	decision.PrimaryReason = fmt.Sprintf("%s signal with %.0f%% confidence (%d signals agree)",
		action, confidence*100, decision.SignalConfluence)

	// Record this trade attempt
	ade.cooldownLock.Lock()
	ade.lastTradeTime[symbol] = time.Now()
	ade.cooldownLock.Unlock()

	return decision
}

// countDirectionalSignals counts signals matching a direction
func countDirectionalSignals(signals []EnhancedSignal, action string) int {
	targetDir := "long"
	if action == "open_short" {
		targetDir = "short"
	}

	count := 0
	for _, s := range signals {
		if s.Direction == targetDir {
			count++
		}
	}
	return count
}

// GetContext returns the current market context
func (ade *AdaptiveDecisionEngine) GetContext() *MarketContext {
	ade.contextLock.RLock()
	defer ade.contextLock.RUnlock()

	// Return a copy
	ctx := *ade.context
	return &ctx
}

// GetRecentPerformance returns recent trading performance
func (ade *AdaptiveDecisionEngine) GetRecentPerformance() map[string]interface{} {
	ade.contextLock.RLock()
	ctx := ade.context
	ade.contextLock.RUnlock()

	return map[string]interface{}{
		"recent_wins":    ctx.RecentWins,
		"recent_losses":  ctx.RecentLosses,
		"win_streak":     ctx.WinStreak,
		"loss_streak":    ctx.LossStreak,
		"recent_pnl":     ctx.RecentPnL,
		"daily_pnl":      ctx.DailyPnL,
		"daily_drawdown": ctx.DailyDrawdown,
	}
}

// SetCooldownPeriod sets the per-symbol cooldown period
func (ade *AdaptiveDecisionEngine) SetCooldownPeriod(period time.Duration) {
	ade.cooldownLock.Lock()
	defer ade.cooldownLock.Unlock()
	ade.cooldownPeriod = period
}

// GetStatus returns the current status of the adaptive engine for API
func (ade *AdaptiveDecisionEngine) GetStatus() map[string]interface{} {
	if ade == nil {
		return map[string]interface{}{
			"enabled":        false,
			"message":        "Adaptive engine not initialized",
			"style":          "unknown",
			"market_context": nil,
		}
	}

	ade.styleLock.RLock()
	styleName := "unknown"
	if ade.styleConfig != nil {
		styleName = string(ade.styleConfig.Style)
	}
	styleConfig := ade.styleConfig
	ade.styleLock.RUnlock()

	ade.contextLock.RLock()
	ctx := ade.context
	ade.contextLock.RUnlock()

	// Get cooldown info
	ade.cooldownLock.RLock()
	activeCooldowns := make(map[string]int64)
	for symbol, lastTime := range ade.lastTradeTime {
		cooldownEnd := lastTime.Add(ade.cooldownPeriod)
		if time.Now().Before(cooldownEnd) {
			activeCooldowns[symbol] = time.Until(cooldownEnd).Milliseconds()
		}
	}
	ade.cooldownLock.RUnlock()

	// Build style info
	styleInfo := map[string]interface{}{
		"name": styleName,
	}
	if styleConfig != nil {
		styleInfo["max_entries"] = styleConfig.MaxEntries
		styleInfo["allow_averaging"] = styleConfig.AllowAveraging
		styleInfo["allow_hedging"] = styleConfig.AllowHedging
		styleInfo["min_confidence"] = styleConfig.MinConfidence
		styleInfo["required_confluence"] = styleConfig.RequiredConfluence
		styleInfo["min_hold_time"] = styleConfig.MinHoldTime.String()
	}

	// Build context info
	contextInfo := map[string]interface{}{}
	if ctx != nil {
		contextInfo = map[string]interface{}{
			"volatility_ratio": ctx.VolatilityRatio,
			"fear_greed_index": ctx.FearGreedIndex,
			"recent_wins":      ctx.RecentWins,
			"recent_losses":    ctx.RecentLosses,
			"win_streak":       ctx.WinStreak,
			"loss_streak":      ctx.LossStreak,
			"recent_pnl":       ctx.RecentPnL,
			"daily_pnl":        ctx.DailyPnL,
			"daily_drawdown":   ctx.DailyDrawdown,
			"open_positions":   ctx.OpenPositions,
			"max_positions":    ctx.MaxPositions,
			"margin_ratio":     ctx.MarginRatio,
			"btc_trend":        ctx.BTCTrend,
			"market_trend":     ctx.MarketTrend,
		}
	}

	return map[string]interface{}{
		"enabled":           true,
		"style":             styleInfo,
		"market_context":    contextInfo,
		"active_cooldowns":  activeCooldowns,
		"cooldown_period":   ade.cooldownPeriod.String(),
		"decision_features": []string{
			"human_like_reasoning",
			"risk_context_awareness",
			"adaptive_position_sizing",
			"smart_averaging",
			"intelligent_hedging",
			"loss_streak_protection",
			"volatility_adjustment",
			"per_symbol_cooldown",
		},
	}
}
