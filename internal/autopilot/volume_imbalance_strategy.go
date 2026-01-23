package autopilot

import (
	"fmt"
	"math"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
)

// ============================================================================
// RAVINDRA'S VOLUME IMBALANCE STRATEGY
// ============================================================================
//
// This strategy detects institutional volume imbalance patterns that indicate
// a high-probability pump after liquidity has been collected from retail traders.
//
// THE PATTERN (5 Steps):
//
// STEP 1: REFERENCE CANDLE (Institutional Entry)
//   - Institution punches a BIG BUY order
//   - Creates the HIGHEST VOLUME + HIGHEST PRICE candle in recent history
//   - This candle's HIGH becomes our ENTRY TRIGGER level
//
// STEP 2: LIQUIDITY DRAIN (Retail Stop-Loss Collection)
//   - Volume DECLINING slowly over multiple candles
//   - Price DECLINING slowly over same candles
//   - Retail stop-losses provide liquidity to institution
//
// STEP 3: LIQUIDITY EXHAUSTION (All Stops Triggered)
//   - Volume reaches MINIMUM
//   - Price at LOCAL LOW
//   - All retail stops have been triggered
//
// STEP 4: THE PUMP (Institutional Re-Entry)
//   - Volume INCREASES again (institution buying)
//   - Price RAPIDLY shoots up
//   - Pulls liquidity from profit-booking orders above
//
// STEP 5: ENTRY TRIGGER
//   - Price CROSSES the HIGH of REFERENCE CANDLE (from Step 1)
//   - This is the ENTRY POINT
//
// Risk Management:
//   - Stop Loss: Below the lowest point (Step 3)
//   - Take Profit: Entry + (Entry - StopLoss) * 4 (1:4 R:R)
//   - Trailing Stop: Move SL to breakeven at 1:2, move to 1:1 at 1:3

// VolumeImbalanceState represents the current state of pattern detection
type VolumeImbalanceState string

const (
	VIStateWatching     VolumeImbalanceState = "WATCHING"     // Looking for reference candle
	VIStateAccumulating VolumeImbalanceState = "ACCUMULATING" // Tracking decline phase
	VIStateExhausted    VolumeImbalanceState = "EXHAUSTED"    // Liquidity exhausted
	VIStatePumping      VolumeImbalanceState = "PUMPING"      // Pump detected
	VIStateReady        VolumeImbalanceState = "READY"        // Ready for entry
	VIStateInvalid      VolumeImbalanceState = "INVALID"      // Pattern invalidated
)

// VolumeImbalanceCandle represents candle data for the strategy
type VolumeImbalanceCandle struct {
	Time           time.Time `json:"time"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	Close          float64   `json:"close"`
	Volume         float64   `json:"volume"`
	TakerBuyVolume float64   `json:"taker_buy_volume"` // More accurate for detecting institutional buying
}

// VolumeImbalancePattern tracks the pattern state for a symbol
type VolumeImbalancePattern struct {
	Symbol string           `json:"symbol"`
	Mode   GinieTradingMode `json:"mode"` // scalp, swing, position

	// Step 1: Reference candle (highest volume + highest price)
	ReferenceCandle struct {
		Time           time.Time `json:"time"`
		High           float64   `json:"high"`
		Low            float64   `json:"low"`
		Close          float64   `json:"close"`
		Volume         float64   `json:"volume"`
		TakerBuyVolume float64   `json:"taker_buy_volume"`
		Index          int       `json:"index"` // Index in candle array when detected
	} `json:"reference_candle"`

	// Pattern state
	State        VolumeImbalanceState `json:"state"`
	StateChanges []string             `json:"state_changes"` // History of state changes for debugging

	// Step 2: Decline tracking
	DeclineCandles    int     `json:"decline_candles"`     // Number of candles in decline
	DeclineVolumeSlope float64 `json:"decline_volume_slope"` // Average volume decline per candle
	DeclinePriceSlope float64 `json:"decline_price_slope"`  // Average price decline per candle

	// Step 3: Exhaustion point
	LowestPrice     float64   `json:"lowest_price"`      // Local low (SL level)
	LowestVolume    float64   `json:"lowest_volume"`     // Minimum volume
	ExhaustionTime  time.Time `json:"exhaustion_time"`   // When exhaustion was detected
	ExhaustionIndex int       `json:"exhaustion_index"`  // Index when exhaustion detected

	// Step 4: Pump detection
	PumpStartVolume float64   `json:"pump_start_volume"` // Volume when pump started
	PumpStartPrice  float64   `json:"pump_start_price"`  // Price when pump started
	PumpCandles     int       `json:"pump_candles"`      // Number of pump candles
	PumpStartTime   time.Time `json:"pump_start_time"`

	// Timing
	DetectedAt time.Time `json:"detected_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at"` // Pattern expires if entry not triggered

	// Validation
	IsValid      bool   `json:"is_valid"`
	InvalidReason string `json:"invalid_reason"`
}

// VolumeImbalanceConfig holds configurable thresholds for the strategy
type VolumeImbalanceConfig struct {
	// Enable/disable the strategy
	Enabled bool `json:"enabled"`

	// Reference candle detection
	MinVolumeSpikeMultiplier float64 `json:"min_volume_spike_multiplier"` // Default: 2.0 (2x avg volume)
	LookbackPeriod           int     `json:"lookback_period"`             // Default: 20 candles to find reference

	// Decline phase thresholds
	MinDeclineCandles      int     `json:"min_decline_candles"`       // Default: 3
	MaxDeclineCandles      int     `json:"max_decline_candles"`       // Default: 15
	MinVolumeDeclination   float64 `json:"min_volume_declination"`    // Default: 0.3 (30% decline from ref)
	MinPriceDeclination    float64 `json:"min_price_declination"`     // Default: 0.005 (0.5% from ref high)

	// Exhaustion detection
	ExhaustionVolumeThreshold float64 `json:"exhaustion_volume_threshold"` // Default: 0.25 (25% of ref volume)

	// Pump detection
	PumpVolumeIncrease float64 `json:"pump_volume_increase"` // Default: 1.5 (50% increase from exhaustion)
	MinPumpCandles     int     `json:"min_pump_candles"`     // Default: 1

	// Risk/Reward
	DefaultRiskRewardRatio float64 `json:"default_risk_reward_ratio"` // Default: 4.0 (1:4)
	StopLossBuffer         float64 `json:"stop_loss_buffer"`          // Default: 0.001 (0.1% below lowest)

	// Trailing stop milestones
	BreakevenRRLevel float64 `json:"breakeven_rr_level"` // Default: 2.0 (move to BE at 1:2)
	OneRRLevel       float64 `json:"one_rr_level"`       // Default: 3.0 (move to 1:1 at 1:3)

	// Pattern expiration
	PatternExpirationMinutes int `json:"pattern_expiration_minutes"` // Default: 60 (1 hour)

	// Mode-specific timeframes
	ScalpTimeframe    string `json:"scalp_timeframe"`    // Default: "5m"
	SwingTimeframe    string `json:"swing_timeframe"`    // Default: "15m"
	PositionTimeframe string `json:"position_timeframe"` // Default: "1h"
}

// DefaultVolumeImbalanceConfig returns the default configuration
func DefaultVolumeImbalanceConfig() *VolumeImbalanceConfig {
	return &VolumeImbalanceConfig{
		Enabled:                   true,
		MinVolumeSpikeMultiplier:  2.0,
		LookbackPeriod:            20,
		MinDeclineCandles:         3,
		MaxDeclineCandles:         15,
		MinVolumeDeclination:      0.30,
		MinPriceDeclination:       0.005,
		ExhaustionVolumeThreshold: 0.25,
		PumpVolumeIncrease:        1.5,
		MinPumpCandles:            1,
		DefaultRiskRewardRatio:    4.0,
		StopLossBuffer:            0.001,
		BreakevenRRLevel:          2.0,
		OneRRLevel:                3.0,
		PatternExpirationMinutes:  60,
		ScalpTimeframe:            "5m",
		SwingTimeframe:            "15m",
		PositionTimeframe:         "1h",
	}
}

// VolumeImbalanceDetector handles pattern detection and trade signal generation
type VolumeImbalanceDetector struct {
	futuresClient binance.FuturesClient
	config        *VolumeImbalanceConfig
	logger        *logging.Logger

	// Active patterns by symbol
	patterns map[string]*VolumeImbalancePattern
	mu       sync.RWMutex
}

// NewVolumeImbalanceDetector creates a new volume imbalance detector
func NewVolumeImbalanceDetector(client binance.FuturesClient, config *VolumeImbalanceConfig, logger *logging.Logger) *VolumeImbalanceDetector {
	if config == nil {
		config = DefaultVolumeImbalanceConfig()
	}
	return &VolumeImbalanceDetector{
		futuresClient: client,
		config:        config,
		logger:        logger,
		patterns:      make(map[string]*VolumeImbalancePattern),
	}
}

// ============================================================================
// CORE DETECTION FUNCTIONS
// ============================================================================

// AnalyzeForVolumeImbalance performs complete volume imbalance analysis for a symbol
func (v *VolumeImbalanceDetector) AnalyzeForVolumeImbalance(symbol string, mode GinieTradingMode, klines []binance.Kline) (*VolumeImbalanceAnalysis, error) {
	if !v.config.Enabled {
		return nil, nil
	}

	if len(klines) < v.config.LookbackPeriod+5 {
		return nil, fmt.Errorf("insufficient klines for volume imbalance analysis: need %d, got %d",
			v.config.LookbackPeriod+5, len(klines))
	}

	// Convert klines to our candle format
	candles := make([]VolumeImbalanceCandle, len(klines))
	for i, k := range klines {
		candles[i] = VolumeImbalanceCandle{
			Time:           time.UnixMilli(k.OpenTime),
			Open:           k.Open,
			High:           k.High,
			Low:            k.Low,
			Close:          k.Close,
			Volume:         k.Volume,
			TakerBuyVolume: k.TakerBuyBaseAssetVolume,
		}
	}

	// Get or create pattern for this symbol
	v.mu.Lock()
	pattern, exists := v.patterns[symbol]
	if !exists || pattern.Mode != mode {
		pattern = &VolumeImbalancePattern{
			Symbol:       symbol,
			Mode:         mode,
			State:        VIStateWatching,
			StateChanges: []string{},
			DetectedAt:   time.Now(),
			UpdatedAt:    time.Now(),
			IsValid:      true,
		}
		v.patterns[symbol] = pattern
	}
	v.mu.Unlock()

	// Check if pattern has expired
	if !pattern.ExpiresAt.IsZero() && time.Now().After(pattern.ExpiresAt) {
		v.resetPattern(pattern, "Pattern expired")
	}

	// Process based on current state
	analysis := &VolumeImbalanceAnalysis{
		Symbol:    symbol,
		Mode:      mode,
		Timestamp: time.Now(),
	}

	switch pattern.State {
	case VIStateWatching:
		// Step 1: Look for reference candle
		refCandle := v.DetectReferenceCandle(candles)
		if refCandle != nil {
			v.setReferenceCandle(pattern, refCandle, len(candles)-1)
			v.transitionState(pattern, VIStateAccumulating)
			v.logPhase(symbol, "REFERENCE CANDLE DETECTED",
				"high", refCandle.High,
				"volume", refCandle.Volume,
				"taker_buy", refCandle.TakerBuyVolume)
		}

	case VIStateAccumulating:
		// Step 2: Track decline phase
		declined := v.TrackDecline(pattern, candles)
		if declined {
			// Check if exhaustion reached
			if v.checkExhaustion(pattern, candles) {
				v.transitionState(pattern, VIStateExhausted)
				v.logPhase(symbol, "LIQUIDITY EXHAUSTION DETECTED",
					"lowest_price", pattern.LowestPrice,
					"lowest_volume", pattern.LowestVolume,
					"decline_candles", pattern.DeclineCandles)
			}
		} else {
			// Check for pattern invalidation
			if v.isDeclineInvalid(pattern, candles) {
				v.resetPattern(pattern, "Decline phase invalid")
			}
		}

	case VIStateExhausted:
		// Step 4: Look for pump
		if v.detectPump(pattern, candles) {
			v.transitionState(pattern, VIStatePumping)
			v.logPhase(symbol, "PUMP DETECTED",
				"pump_volume", pattern.PumpStartVolume,
				"pump_price", pattern.PumpStartPrice)
		}

	case VIStatePumping:
		// Step 5: Check for entry trigger
		currentCandle := candles[len(candles)-1]
		if v.CheckEntryTrigger(pattern, currentCandle) {
			v.transitionState(pattern, VIStateReady)
			v.logPhase(symbol, "ENTRY TRIGGER - PRICE CROSSED REFERENCE HIGH",
				"current_price", currentCandle.Close,
				"reference_high", pattern.ReferenceCandle.High)
		}
	}

	// Update analysis with current pattern state
	analysis.Pattern = pattern
	analysis.PatternDetected = pattern.State == VIStateReady
	analysis.PatternState = string(pattern.State)

	if pattern.State == VIStateReady {
		// Calculate entry parameters
		currentPrice := candles[len(candles)-1].Close
		sl, tp, rr := v.CalculateRiskReward(pattern, currentPrice)
		analysis.SuggestedEntry = currentPrice
		analysis.SuggestedSL = sl
		analysis.SuggestedTP = tp
		analysis.RiskReward = rr
		analysis.Direction = "LONG"
		analysis.Confidence = v.calculateConfidence(pattern, candles)
	}

	return analysis, nil
}

// DetectReferenceCandle finds the candle with highest volume + highest price in lookback
func (v *VolumeImbalanceDetector) DetectReferenceCandle(candles []VolumeImbalanceCandle) *VolumeImbalanceCandle {
	if len(candles) < v.config.LookbackPeriod {
		return nil
	}

	// Calculate average volume for comparison
	avgVolume := v.calculateAverageVolume(candles, v.config.LookbackPeriod)

	// Find the candle in lookback period with highest volume that also has significant price
	lookbackStart := len(candles) - v.config.LookbackPeriod
	if lookbackStart < 0 {
		lookbackStart = 0
	}

	var bestCandle *VolumeImbalanceCandle
	var bestScore float64

	// Find highest high in the period for reference
	var highestHigh float64
	for i := lookbackStart; i < len(candles); i++ {
		if candles[i].High > highestHigh {
			highestHigh = candles[i].High
		}
	}

	for i := lookbackStart; i < len(candles)-1; i++ { // -1 to not pick the current candle
		c := candles[i]

		// Check if volume is significantly above average
		if c.Volume < avgVolume*v.config.MinVolumeSpikeMultiplier {
			continue
		}

		// Check if this candle created a local high
		// The high should be at or near the highest high
		highProximity := c.High / highestHigh
		if highProximity < 0.99 { // Within 1% of the highest high
			continue
		}

		// Score combines volume spike and price position
		volumeScore := c.Volume / avgVolume
		priceScore := highProximity * 100

		// Bonus for bullish candles (institutions buying)
		bullishBonus := 1.0
		if c.Close > c.Open {
			bullishBonus = 1.2
		}

		// Bonus for high taker buy volume (aggressive buying)
		takerBuyRatio := 1.0
		if c.Volume > 0 {
			takerBuyRatio = 1 + (c.TakerBuyVolume/c.Volume)*0.5
		}

		totalScore := volumeScore * priceScore * bullishBonus * takerBuyRatio

		if totalScore > bestScore {
			bestScore = totalScore
			candleCopy := c
			bestCandle = &candleCopy
		}
	}

	return bestCandle
}

// TrackDecline monitors the decline phase after reference candle
func (v *VolumeImbalanceDetector) TrackDecline(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	if pattern.ReferenceCandle.Volume == 0 {
		return false
	}

	// Count candles since reference
	refIdx := pattern.ReferenceCandle.Index
	if refIdx >= len(candles) {
		return false
	}

	currentIdx := len(candles) - 1
	candlesSinceRef := currentIdx - refIdx

	if candlesSinceRef < v.config.MinDeclineCandles {
		return false
	}

	if candlesSinceRef > v.config.MaxDeclineCandles {
		// Too many candles in decline - pattern may be invalid
		return false
	}

	// Check if volume is declining
	refVolume := pattern.ReferenceCandle.Volume
	var volumeSum float64
	var priceSum float64
	declineCount := 0

	// Track lowest price and volume
	lowestPrice := candles[refIdx].Low
	lowestVolume := candles[refIdx].Volume

	for i := refIdx + 1; i <= currentIdx; i++ {
		volumeSum += candles[i].Volume
		priceSum += candles[i].Close

		if candles[i].Low < lowestPrice {
			lowestPrice = candles[i].Low
			pattern.ExhaustionIndex = i
		}
		if candles[i].Volume < lowestVolume {
			lowestVolume = candles[i].Volume
		}

		// Count declining candles
		if i > refIdx+1 && candles[i].Close < candles[i-1].Close {
			declineCount++
		}
	}

	avgDeclineVolume := volumeSum / float64(candlesSinceRef)
	volumeDeclination := 1 - (avgDeclineVolume / refVolume)

	// Check if price declined from reference high
	priceDeclination := (pattern.ReferenceCandle.High - lowestPrice) / pattern.ReferenceCandle.High

	// Update pattern tracking
	pattern.DeclineCandles = candlesSinceRef
	pattern.DeclineVolumeSlope = volumeDeclination / float64(candlesSinceRef)
	pattern.DeclinePriceSlope = priceDeclination / float64(candlesSinceRef)
	pattern.LowestPrice = lowestPrice
	pattern.LowestVolume = lowestVolume
	pattern.UpdatedAt = time.Now()

	// Validate decline characteristics
	if volumeDeclination < v.config.MinVolumeDeclination {
		return false // Volume not declining enough
	}

	if priceDeclination < v.config.MinPriceDeclination {
		return false // Price not declining enough
	}

	// At least half of candles should be declining
	if float64(declineCount) < float64(candlesSinceRef)*0.4 {
		return false
	}

	return true
}

// checkExhaustion determines if liquidity exhaustion has occurred
func (v *VolumeImbalanceDetector) checkExhaustion(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	refVolume := pattern.ReferenceCandle.Volume
	exhaustionThreshold := refVolume * v.config.ExhaustionVolumeThreshold

	// Check if recent volume is very low (exhaustion)
	if len(candles) < 3 {
		return false
	}

	// Volume should be at or near minimum
	if pattern.LowestVolume > exhaustionThreshold {
		return false
	}

	// Price should be at a local low
	currentCandle := candles[len(candles)-1]
	prevCandle := candles[len(candles)-2]

	// Confirm we're at or near the lowest point
	if currentCandle.Low > pattern.LowestPrice*1.005 { // Within 0.5% of lowest
		return false
	}

	// Volume should show signs of stabilizing or starting to increase
	// (the exhaustion point often shows a slight uptick before the pump)
	volumeRatio := currentCandle.Volume / prevCandle.Volume
	if volumeRatio >= 0.9 { // Volume stabilizing or increasing slightly
		pattern.ExhaustionTime = time.Now()
		return true
	}

	return false
}

// detectPump identifies the start of the pump phase
func (v *VolumeImbalanceDetector) detectPump(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	if len(candles) < 2 {
		return false
	}

	exhaustionIdx := pattern.ExhaustionIndex
	if exhaustionIdx == 0 || exhaustionIdx >= len(candles)-1 {
		return false
	}

	currentCandle := candles[len(candles)-1]
	prevCandle := candles[len(candles)-2]

	// Volume should increase significantly from exhaustion
	volumeIncrease := currentCandle.Volume / pattern.LowestVolume
	if volumeIncrease < v.config.PumpVolumeIncrease {
		return false
	}

	// Price should be moving up
	if currentCandle.Close <= prevCandle.Close {
		return false
	}

	// Candle should be bullish
	if currentCandle.Close <= currentCandle.Open {
		return false
	}

	// Taker buy volume should be significant (institutions buying aggressively)
	if currentCandle.Volume > 0 {
		takerBuyRatio := currentCandle.TakerBuyVolume / currentCandle.Volume
		if takerBuyRatio < 0.5 { // At least 50% taker buy
			return false
		}
	}

	// Record pump start
	pattern.PumpStartVolume = currentCandle.Volume
	pattern.PumpStartPrice = currentCandle.Close
	pattern.PumpStartTime = time.Now()
	pattern.PumpCandles = 1

	return true
}

// CheckEntryTrigger determines if price has crossed the reference candle high
func (v *VolumeImbalanceDetector) CheckEntryTrigger(pattern *VolumeImbalancePattern, currentCandle VolumeImbalanceCandle) bool {
	if pattern.ReferenceCandle.High == 0 {
		return false
	}

	// Price must cross above the reference candle's high
	if currentCandle.High >= pattern.ReferenceCandle.High {
		// Confirm the close is also above (not just a wick)
		if currentCandle.Close >= pattern.ReferenceCandle.High*0.998 { // Within 0.2%
			return true
		}
	}

	// Also check if price opened above and closed above
	if currentCandle.Open >= pattern.ReferenceCandle.High &&
		currentCandle.Close >= pattern.ReferenceCandle.High {
		return true
	}

	return false
}

// CalculateRiskReward calculates SL, TP, and R:R ratio
func (v *VolumeImbalanceDetector) CalculateRiskReward(pattern *VolumeImbalancePattern, entryPrice float64) (stopLoss, takeProfit, ratio float64) {
	// Stop loss = below the lowest point (Step 3) with buffer
	stopLoss = pattern.LowestPrice * (1 - v.config.StopLossBuffer)

	// Calculate risk (entry to stop loss)
	risk := entryPrice - stopLoss
	if risk <= 0 {
		// Fallback if lowest price is somehow above entry
		risk = entryPrice * 0.02 // 2% default risk
		stopLoss = entryPrice - risk
	}

	// Take profit = entry + (risk * R:R ratio)
	takeProfit = entryPrice + (risk * v.config.DefaultRiskRewardRatio)
	ratio = v.config.DefaultRiskRewardRatio

	return stopLoss, takeProfit, ratio
}

// ============================================================================
// TRAILING STOP MANAGER
// ============================================================================

// TrailingStopManager manages trailing stops according to Ravindra's approach
type TrailingStopManager struct {
	EntryPrice float64 `json:"entry_price"`
	StopLoss   float64 `json:"stop_loss"`
	TakeProfit float64 `json:"take_profit"`

	// Risk amount (entry - initial SL)
	RiskAmount float64 `json:"risk_amount"`

	// Current state
	CurrentRR        float64 `json:"current_rr"`
	HighestPrice     float64 `json:"highest_price"`
	MovedToBreakeven bool    `json:"moved_to_breakeven"` // At 1:2 R:R
	MovedTo1R        bool    `json:"moved_to_1r"`        // At 1:3 R:R

	// Config
	BreakevenRRLevel float64 `json:"breakeven_rr_level"` // Default: 2.0
	OneRRLevel       float64 `json:"one_rr_level"`       // Default: 3.0
}

// NewTrailingStopManager creates a new trailing stop manager
func NewTrailingStopManager(entryPrice, stopLoss, takeProfit float64, config *VolumeImbalanceConfig) *TrailingStopManager {
	riskAmount := entryPrice - stopLoss
	if riskAmount <= 0 {
		riskAmount = entryPrice * 0.01 // 1% fallback
	}

	breakevenLevel := 2.0
	oneRRLevel := 3.0
	if config != nil {
		breakevenLevel = config.BreakevenRRLevel
		oneRRLevel = config.OneRRLevel
	}

	return &TrailingStopManager{
		EntryPrice:       entryPrice,
		StopLoss:         stopLoss,
		TakeProfit:       takeProfit,
		RiskAmount:       riskAmount,
		HighestPrice:     entryPrice,
		BreakevenRRLevel: breakevenLevel,
		OneRRLevel:       oneRRLevel,
	}
}

// Update checks current price and returns updated stop loss with action description
// Returns: (newStopLoss, action)
// Actions: "HOLD" (no change), "MOVE_TO_BREAKEVEN", "MOVE_TO_1R", "TAKE_PROFIT"
func (t *TrailingStopManager) Update(currentPrice float64) (newStopLoss float64, action string) {
	// Track highest price
	if currentPrice > t.HighestPrice {
		t.HighestPrice = currentPrice
	}

	// Calculate current R:R achieved
	profit := currentPrice - t.EntryPrice
	t.CurrentRR = profit / t.RiskAmount

	newStopLoss = t.StopLoss
	action = "HOLD"

	// Check for take profit
	if currentPrice >= t.TakeProfit {
		action = "TAKE_PROFIT"
		return newStopLoss, action
	}

	// At 1:3 R:R → Move SL to 1:1 level
	if t.CurrentRR >= t.OneRRLevel && !t.MovedTo1R {
		// 1:1 level = entry + risk
		oneRLevel := t.EntryPrice + t.RiskAmount
		if oneRLevel > t.StopLoss {
			t.StopLoss = oneRLevel
			newStopLoss = t.StopLoss
			t.MovedTo1R = true
			t.MovedToBreakeven = true // Also mark breakeven as done
			action = "MOVE_TO_1R"
			return newStopLoss, action
		}
	}

	// At 1:2 R:R → Move SL to entry (breakeven)
	if t.CurrentRR >= t.BreakevenRRLevel && !t.MovedToBreakeven {
		if t.EntryPrice > t.StopLoss {
			t.StopLoss = t.EntryPrice
			newStopLoss = t.StopLoss
			t.MovedToBreakeven = true
			action = "MOVE_TO_BREAKEVEN"
			return newStopLoss, action
		}
	}

	return newStopLoss, action
}

// GetStatus returns the current trailing stop status
func (t *TrailingStopManager) GetStatus() TrailingStopStatus {
	return TrailingStopStatus{
		EntryPrice:       t.EntryPrice,
		CurrentStopLoss:  t.StopLoss,
		TakeProfit:       t.TakeProfit,
		RiskAmount:       t.RiskAmount,
		CurrentRR:        t.CurrentRR,
		HighestPrice:     t.HighestPrice,
		AtBreakeven:      t.MovedToBreakeven,
		At1R:             t.MovedTo1R,
		BreakevenLevel:   t.EntryPrice,
		OneRLevel:        t.EntryPrice + t.RiskAmount,
		BreakevenTrigger: t.EntryPrice + (t.RiskAmount * t.BreakevenRRLevel),
		OneRTrigger:      t.EntryPrice + (t.RiskAmount * t.OneRRLevel),
	}
}

// TrailingStopStatus provides a snapshot of trailing stop state
type TrailingStopStatus struct {
	EntryPrice       float64 `json:"entry_price"`
	CurrentStopLoss  float64 `json:"current_stop_loss"`
	TakeProfit       float64 `json:"take_profit"`
	RiskAmount       float64 `json:"risk_amount"`
	CurrentRR        float64 `json:"current_rr"`
	HighestPrice     float64 `json:"highest_price"`
	AtBreakeven      bool    `json:"at_breakeven"`
	At1R             bool    `json:"at_1r"`
	BreakevenLevel   float64 `json:"breakeven_level"`   // Price where SL moves to BE
	OneRLevel        float64 `json:"one_r_level"`       // Price where SL moves to 1:1
	BreakevenTrigger float64 `json:"breakeven_trigger"` // Trigger price for BE move
	OneRTrigger      float64 `json:"one_r_trigger"`     // Trigger price for 1R move
}

// ============================================================================
// ANALYSIS RESULT
// ============================================================================

// VolumeImbalanceAnalysis contains the complete analysis result
type VolumeImbalanceAnalysis struct {
	Symbol      string           `json:"symbol"`
	Mode        GinieTradingMode `json:"mode"`
	Timestamp   time.Time        `json:"timestamp"`

	// Pattern detection
	PatternDetected bool                    `json:"pattern_detected"`
	PatternState    string                  `json:"pattern_state"`
	Pattern         *VolumeImbalancePattern `json:"pattern,omitempty"`

	// Trade recommendation (only when PatternDetected = true)
	Direction      string  `json:"direction"`       // "LONG" (this strategy is long-only)
	SuggestedEntry float64 `json:"suggested_entry"`
	SuggestedSL    float64 `json:"suggested_sl"`
	SuggestedTP    float64 `json:"suggested_tp"`
	RiskReward     float64 `json:"risk_reward"`
	Confidence     float64 `json:"confidence"` // 0-100

	// Debugging info
	StateHistory []string `json:"state_history,omitempty"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func (v *VolumeImbalanceDetector) calculateAverageVolume(candles []VolumeImbalanceCandle, period int) float64 {
	if len(candles) < period {
		period = len(candles)
	}
	if period == 0 {
		return 0
	}

	var sum float64
	start := len(candles) - period
	if start < 0 {
		start = 0
	}

	for i := start; i < len(candles); i++ {
		sum += candles[i].Volume
	}
	return sum / float64(period)
}

func (v *VolumeImbalanceDetector) setReferenceCandle(pattern *VolumeImbalancePattern, candle *VolumeImbalanceCandle, index int) {
	pattern.ReferenceCandle.Time = candle.Time
	pattern.ReferenceCandle.High = candle.High
	pattern.ReferenceCandle.Low = candle.Low
	pattern.ReferenceCandle.Close = candle.Close
	pattern.ReferenceCandle.Volume = candle.Volume
	pattern.ReferenceCandle.TakerBuyVolume = candle.TakerBuyVolume
	pattern.ReferenceCandle.Index = index

	// Set expiration
	pattern.ExpiresAt = time.Now().Add(time.Duration(v.config.PatternExpirationMinutes) * time.Minute)
}

func (v *VolumeImbalanceDetector) transitionState(pattern *VolumeImbalancePattern, newState VolumeImbalanceState) {
	oldState := pattern.State
	pattern.State = newState
	pattern.UpdatedAt = time.Now()

	transition := fmt.Sprintf("%s -> %s at %s", oldState, newState, pattern.UpdatedAt.Format("15:04:05"))
	pattern.StateChanges = append(pattern.StateChanges, transition)

	// Keep only last 10 state changes
	if len(pattern.StateChanges) > 10 {
		pattern.StateChanges = pattern.StateChanges[len(pattern.StateChanges)-10:]
	}
}

func (v *VolumeImbalanceDetector) resetPattern(pattern *VolumeImbalancePattern, reason string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	pattern.State = VIStateWatching
	pattern.ReferenceCandle = struct {
		Time           time.Time `json:"time"`
		High           float64   `json:"high"`
		Low            float64   `json:"low"`
		Close          float64   `json:"close"`
		Volume         float64   `json:"volume"`
		TakerBuyVolume float64   `json:"taker_buy_volume"`
		Index          int       `json:"index"`
	}{}
	pattern.DeclineCandles = 0
	pattern.DeclineVolumeSlope = 0
	pattern.DeclinePriceSlope = 0
	pattern.LowestPrice = 0
	pattern.LowestVolume = 0
	pattern.ExhaustionIndex = 0
	pattern.PumpStartVolume = 0
	pattern.PumpStartPrice = 0
	pattern.PumpCandles = 0
	pattern.ExpiresAt = time.Time{}
	pattern.UpdatedAt = time.Now()
	pattern.InvalidReason = reason

	pattern.StateChanges = append(pattern.StateChanges, fmt.Sprintf("RESET: %s", reason))
}

func (v *VolumeImbalanceDetector) isDeclineInvalid(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	if len(candles) < 2 {
		return false
	}

	currentCandle := candles[len(candles)-1]

	// If price goes above reference high before exhaustion, pattern is invalid
	if currentCandle.High > pattern.ReferenceCandle.High*1.005 {
		return true
	}

	// If too many candles pass without exhaustion
	if pattern.DeclineCandles > v.config.MaxDeclineCandles {
		return true
	}

	return false
}

func (v *VolumeImbalanceDetector) calculateConfidence(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) float64 {
	// Base confidence
	confidence := 50.0

	// Volume spike quality (up to +15)
	avgVol := v.calculateAverageVolume(candles, v.config.LookbackPeriod)
	if avgVol > 0 {
		volumeSpike := pattern.ReferenceCandle.Volume / avgVol
		volumeBonus := math.Min(volumeSpike-2.0, 3.0) * 5 // Up to +15 for 5x volume
		confidence += volumeBonus
	}

	// Clear decline pattern (up to +10)
	if pattern.DeclineCandles >= 5 && pattern.DeclineCandles <= 10 {
		confidence += 10
	} else if pattern.DeclineCandles >= 3 {
		confidence += 5
	}

	// Strong exhaustion (up to +10)
	if pattern.LowestVolume < pattern.ReferenceCandle.Volume*0.2 {
		confidence += 10
	} else if pattern.LowestVolume < pattern.ReferenceCandle.Volume*0.3 {
		confidence += 5
	}

	// Strong pump (up to +15)
	if pattern.PumpStartVolume > pattern.LowestVolume*2 {
		confidence += 15
	} else if pattern.PumpStartVolume > pattern.LowestVolume*1.5 {
		confidence += 10
	}

	// Taker buy volume in reference candle (institutional buying)
	if pattern.ReferenceCandle.Volume > 0 {
		takerBuyRatio := pattern.ReferenceCandle.TakerBuyVolume / pattern.ReferenceCandle.Volume
		if takerBuyRatio > 0.6 {
			confidence += 5
		}
	}

	return math.Min(confidence, 95.0)
}

func (v *VolumeImbalanceDetector) logPhase(symbol string, phase string, args ...interface{}) {
	if v.logger != nil {
		allArgs := append([]interface{}{"symbol", symbol, "phase", phase}, args...)
		v.logger.Info("[VOLUME_IMBALANCE]", allArgs...)
	}
}

// GetTimeframe returns the appropriate timeframe for the trading mode
func (v *VolumeImbalanceDetector) GetTimeframe(mode GinieTradingMode) string {
	switch mode {
	case GinieModeScalp:
		return v.config.ScalpTimeframe
	case GinieModeSwing:
		return v.config.SwingTimeframe
	case GinieModePosition:
		return v.config.PositionTimeframe
	default:
		return v.config.ScalpTimeframe
	}
}

// GetPattern returns the current pattern for a symbol (thread-safe)
func (v *VolumeImbalanceDetector) GetPattern(symbol string) *VolumeImbalancePattern {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.patterns[symbol]
}

// ClearPattern removes the pattern for a symbol (e.g., after trade entry)
func (v *VolumeImbalanceDetector) ClearPattern(symbol string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.patterns, symbol)
}

// GetAllPatterns returns all active patterns (thread-safe)
func (v *VolumeImbalanceDetector) GetAllPatterns() map[string]*VolumeImbalancePattern {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make(map[string]*VolumeImbalancePattern)
	for k, v := range v.patterns {
		result[k] = v
	}
	return result
}

// CleanupExpiredPatterns removes patterns that have expired
func (v *VolumeImbalanceDetector) CleanupExpiredPatterns() int {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()
	removed := 0

	for symbol, pattern := range v.patterns {
		if !pattern.ExpiresAt.IsZero() && now.After(pattern.ExpiresAt) {
			delete(v.patterns, symbol)
			removed++
		}
	}

	return removed
}
