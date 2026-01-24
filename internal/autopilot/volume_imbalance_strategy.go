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
// RAVINDRA'S VOLUME IMBALANCE STRATEGY - 3-STEP MODEL
// ============================================================================
//
// This strategy detects institutional volume imbalance patterns using Ravindra
// Rokade's (Stock Niti) methodology. The pattern identifies high-probability
// breakout entries after institutional accumulation.
//
// THE PATTERN (3 Steps - Refined):
//
// STEP 1: ACCUMULATION START (Institutional First Buy)
//   - Institution punches a BIG BUY order
//   - Creates the HIGHEST VOLUME spike (2x+ average) in recent history
//   - This becomes the REFERENCE CANDLE
//   - The candle's HIGH becomes our ENTRY TRIGGER level
//   - FOOTPRINT: Massive executed volume visible on candlestick
//
// STEP 2: SIDEWAYS CONSOLIDATION (Market Digesting)
//   - Volume DECLINING over multiple candles
//   - Price stays in SIDEWAYS RANGE (not necessarily declining)
//   - Institutions accumulating quietly
//   - Retail losing interest (declining volume = no liquidity)
//   - Price respects range: doesn't break reference high/low
//   - DURATION: Multiple candles with decreasing volatility
//
// STEP 3: BREAKOUT ENTRY (Institutional Push)
//   - Volume SURGES again (50%+ above consolidation average)
//   - Price BREAKS ABOVE the reference candle HIGH
//   - This is our ENTRY POINT
//   - LLM validates to filter false breakouts
//
// WHY 3 STEPS (NOT 5)?
// The original 5-step model was overly complex. Ravindra's actual approach:
// - Reference Candle → Step 1: Accumulation Start
// - Liquidity Drain + Exhaustion → Step 2: Sideways Consolidation (combined)
// - Pump + Entry → Step 3: Breakout Entry (combined)
//
// TIMEFRAME VALIDATION (Backtested):
// - 15-minute for Scalp mode (validated by backtesting)
// - Institutions need 30-60+ minutes to accumulate quietly
// - 5-minute consolidation too short - produces false signals
//
// Risk Management:
//   - Entry: At or above reference candle high
//   - Stop Loss: Below consolidation low (with buffer)
//   - Take Profit: Entry + (Risk × 4) for 1:4 R:R
//   - Trailing Stop: Move SL to breakeven at 1:2, move to 1:1 at 1:3

// VolumeImbalanceState represents the current state of 3-step pattern detection
type VolumeImbalanceState string

const (
	VIStateWatching      VolumeImbalanceState = "WATCHING"      // Step 1: Looking for accumulation start (volume spike)
	VIStateConsolidating VolumeImbalanceState = "CONSOLIDATING" // Step 2: Volume declining, price sideways
	VIStateReady         VolumeImbalanceState = "READY"         // Step 3: Breakout detected, ready for entry
	VIStateEntered       VolumeImbalanceState = "ENTERED"       // Position taken
	VIStateTrailing      VolumeImbalanceState = "TRAILING"      // Managing position with trailing SL
	VIStateInvalid       VolumeImbalanceState = "INVALID"       // Pattern invalidated

	// Legacy states for backward compatibility
	VIStateAccumulating VolumeImbalanceState = "ACCUMULATING" // Deprecated: maps to CONSOLIDATING
	VIStateExhausted    VolumeImbalanceState = "EXHAUSTED"    // Deprecated: not used in 3-step
	VIStatePumping      VolumeImbalanceState = "PUMPING"      // Deprecated: merged into READY
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

// VolumeImbalancePattern tracks the 3-step pattern state for a symbol
type VolumeImbalancePattern struct {
	Symbol string           `json:"symbol"`
	Mode   GinieTradingMode `json:"mode"` // scalp, swing, position

	// Step 1: Accumulation Start (Reference Candle - high volume spike)
	ReferenceCandle struct {
		Time           time.Time `json:"time"`
		High           float64   `json:"high"`    // Entry trigger level
		Low            float64   `json:"low"`     // Part of consolidation range
		Close          float64   `json:"close"`
		Volume         float64   `json:"volume"`  // High volume spike (2x+ average)
		TakerBuyVolume float64   `json:"taker_buy_volume"`
		Index          int       `json:"index"`   // Index in candle array when detected
	} `json:"reference_candle"`

	// Pattern state
	State        VolumeImbalanceState `json:"state"`
	StateChanges []string             `json:"state_changes"` // History of state changes for debugging

	// Step 2: Sideways Consolidation tracking
	ConsolidationStartIndex int       `json:"consolidation_start_index"` // When consolidation began
	ConsolidationCandles    int       `json:"consolidation_candles"`     // Number of candles in consolidation
	ConsolidationLow        float64   `json:"consolidation_low"`         // Lowest low during consolidation (SL reference)
	ConsolidationHigh       float64   `json:"consolidation_high"`        // Highest high during consolidation
	ConsolidationAvgVolume  float64   `json:"consolidation_avg_volume"`  // Average volume during consolidation
	VolumeTrend             float64   `json:"volume_trend"`              // Volume trend slope (should be negative)

	// Step 3: Breakout Entry
	BreakoutCandle struct {
		Time   time.Time `json:"time"`
		High   float64   `json:"high"`
		Volume float64   `json:"volume"` // Volume surge (50%+ above consolidation avg)
	} `json:"breakout_candle"`

	// Legacy fields for backward compatibility (maps to new structure)
	DeclineCandles     int     `json:"decline_candles"`      // Maps to ConsolidationCandles
	DeclineVolumeSlope float64 `json:"decline_volume_slope"` // Maps to VolumeTrend
	DeclinePriceSlope  float64 `json:"decline_price_slope"`  // Deprecated in 3-step model
	LowestPrice        float64 `json:"lowest_price"`         // Maps to ConsolidationLow
	LowestVolume       float64 `json:"lowest_volume"`        // Legacy field
	ExhaustionTime     time.Time `json:"exhaustion_time"`    // Deprecated
	ExhaustionIndex    int     `json:"exhaustion_index"`     // Deprecated
	PumpStartVolume    float64 `json:"pump_start_volume"`    // Maps to BreakoutCandle.Volume
	PumpStartPrice     float64 `json:"pump_start_price"`     // Deprecated
	PumpCandles        int     `json:"pump_candles"`         // Deprecated
	PumpStartTime      time.Time `json:"pump_start_time"`    // Maps to BreakoutCandle.Time

	// Timing
	DetectedAt time.Time `json:"detected_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ExpiresAt  time.Time `json:"expires_at"` // Pattern expires if entry not triggered

	// Validation
	IsValid       bool   `json:"is_valid"`
	InvalidReason string `json:"invalid_reason"`
}

// VolumeImbalanceConfig holds configurable thresholds for the 3-step strategy
type VolumeImbalanceConfig struct {
	// Enable/disable the strategy
	Enabled bool `json:"enabled"`

	// Step 1: Accumulation Start detection
	MinVolumeSpikeMultiplier float64 `json:"min_volume_spike_multiplier"` // Default: 2.0 (2x avg volume for spike)
	LookbackPeriod           int     `json:"lookback_period"`             // Default: 20 candles to calculate average

	// Step 2: Sideways Consolidation thresholds
	MinConsolidationCandles       int     `json:"min_consolidation_candles"`        // Default: 2
	MaxConsolidationCandles       int     `json:"max_consolidation_candles"`        // Default: 6
	ConsolidationRangeTolerance   float64 `json:"consolidation_range_tolerance"`    // Default: 0.01 (1% tolerance)

	// Step 3: Breakout Entry thresholds
	BreakoutVolumeSurge float64 `json:"breakout_volume_surge"` // Default: 1.5 (50% above consolidation avg)

	// Legacy fields for backward compatibility
	MinDeclineCandles         int     `json:"min_decline_candles"`          // Maps to MinConsolidationCandles
	MaxDeclineCandles         int     `json:"max_decline_candles"`          // Maps to MaxConsolidationCandles
	MinVolumeDeclination      float64 `json:"min_volume_declination"`       // Deprecated in 3-step
	MinPriceDeclination       float64 `json:"min_price_declination"`        // Deprecated in 3-step
	ExhaustionVolumeThreshold float64 `json:"exhaustion_volume_threshold"`  // Deprecated
	PumpVolumeIncrease        float64 `json:"pump_volume_increase"`         // Maps to BreakoutVolumeSurge
	MinPumpCandles            int     `json:"min_pump_candles"`             // Deprecated

	// Risk/Reward
	DefaultRiskRewardRatio float64 `json:"default_risk_reward_ratio"` // Default: 4.0 (1:4)
	StopLossBuffer         float64 `json:"stop_loss_buffer"`          // Default: 0.001 (0.1% below lowest)

	// Trailing stop milestones (Ravindra's approach)
	BreakevenRRLevel float64 `json:"breakeven_rr_level"` // Default: 2.0 (move to BE at 1:2)
	OneRRLevel       float64 `json:"one_rr_level"`       // Default: 3.0 (move to 1:1 at 1:3)

	// Pattern expiration
	PatternExpirationMinutes int `json:"pattern_expiration_minutes"` // Default: 60 (1 hour)

	// Mode-specific timeframes (VALIDATED BY BACKTESTING)
	ScalpTimeframe    string `json:"scalp_timeframe"`    // Default: "15m" (validated - institutions need 30-60+ min)
	SwingTimeframe    string `json:"swing_timeframe"`    // Default: "1h"
	PositionTimeframe string `json:"position_timeframe"` // Default: "4h"
}

// DefaultVolumeImbalanceConfig returns the default configuration for 3-step model
func DefaultVolumeImbalanceConfig() *VolumeImbalanceConfig {
	return &VolumeImbalanceConfig{
		Enabled:                     true,

		// Step 1: Accumulation Start
		MinVolumeSpikeMultiplier:    2.0,
		LookbackPeriod:              20,

		// Step 2: Sideways Consolidation
		MinConsolidationCandles:     2,
		MaxConsolidationCandles:     6,
		ConsolidationRangeTolerance: 0.01,

		// Step 3: Breakout Entry
		BreakoutVolumeSurge:         1.5,

		// Legacy mappings
		MinDeclineCandles:           2,
		MaxDeclineCandles:           6,
		MinVolumeDeclination:        0.30,
		MinPriceDeclination:         0.005,
		ExhaustionVolumeThreshold:   0.25,
		PumpVolumeIncrease:          1.5,
		MinPumpCandles:              1,

		// Risk/Reward
		DefaultRiskRewardRatio:      4.0,
		StopLossBuffer:              0.001,

		// Trailing stop
		BreakevenRRLevel:            2.0,
		OneRRLevel:                  3.0,

		// Expiration
		PatternExpirationMinutes:    60,

		// Timeframes (VALIDATED)
		ScalpTimeframe:              "15m", // Changed from 5m - validated by backtesting
		SwingTimeframe:              "1h",
		PositionTimeframe:           "4h",
	}
}

// VolumeImbalanceDetector handles 3-step pattern detection and trade signal generation
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
// CORE 3-STEP DETECTION FUNCTIONS
// ============================================================================

// AnalyzeForVolumeImbalance performs complete 3-step volume imbalance analysis for a symbol
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

	// Check if pattern has expired (while still holding the lock)
	patternExpired := !pattern.ExpiresAt.IsZero() && time.Now().After(pattern.ExpiresAt)
	v.mu.Unlock()

	if patternExpired {
		v.resetPattern(pattern, "Pattern expired")
	}

	// Process based on current state using 3-step model
	analysis := &VolumeImbalanceAnalysis{
		Symbol:    symbol,
		Mode:      mode,
		Timestamp: time.Now(),
	}

	switch pattern.State {
	case VIStateWatching:
		// STEP 1: Look for Accumulation Start (high volume spike)
		refCandle, refIndex := v.detectAccumulationStart(candles)
		if refCandle != nil && refIndex >= 0 {
			v.setReferenceCandle(pattern, refCandle, refIndex)
			v.transitionState(pattern, VIStateConsolidating)
			v.logPhase(symbol, "STEP 1: ACCUMULATION START DETECTED",
				"high", refCandle.High,
				"volume", refCandle.Volume,
				"avg_volume", v.calculateAverageVolume(candles, v.config.LookbackPeriod),
				"taker_buy", refCandle.TakerBuyVolume)
		}

	case VIStateConsolidating, VIStateAccumulating: // Handle legacy state
		// STEP 2: Track Sideways Consolidation (volume declining, price in range)
		if v.isConsolidating(pattern, candles) {
			// Check if breakout conditions are met (Step 3)
			if v.isBreakoutReady(pattern, candles) {
				v.transitionState(pattern, VIStateReady)
				currentCandle := candles[len(candles)-1]
				v.logPhase(symbol, "STEP 3: BREAKOUT ENTRY READY",
					"current_price", currentCandle.Close,
					"reference_high", pattern.ReferenceCandle.High,
					"breakout_volume", currentCandle.Volume,
					"consolidation_avg_vol", pattern.ConsolidationAvgVolume)
			}
		} else {
			// Check for pattern invalidation
			if v.isPatternInvalid(pattern, candles) {
				v.resetPattern(pattern, "Consolidation pattern invalid")
			}
		}

	case VIStateExhausted, VIStatePumping:
		// Legacy state handling - redirect to consolidating check
		pattern.State = VIStateConsolidating
		if v.isBreakoutReady(pattern, candles) {
			v.transitionState(pattern, VIStateReady)
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

		// Initialize trailing stop manager for position management after entry
		// Caller should use this to manage SL according to Ravindra's R:R milestones
		analysis.TrailingStop = NewTrailingStopManager(currentPrice, sl, tp, v.config)
	}

	return analysis, nil
}

// detectAccumulationStart identifies Step 1: High volume spike (2x+ average)
// Returns both the candle and its index in the candles array, or (nil, -1) if not found
func (v *VolumeImbalanceDetector) detectAccumulationStart(candles []VolumeImbalanceCandle) (*VolumeImbalanceCandle, int) {
	if len(candles) < v.config.LookbackPeriod {
		return nil, -1
	}

	// Calculate average volume for comparison
	avgVolume := v.calculateAverageVolume(candles, v.config.LookbackPeriod)

	// Look for a candle with volume spike (2x+ average)
	lookbackStart := len(candles) - v.config.LookbackPeriod
	if lookbackStart < 0 {
		lookbackStart = 0
	}

	var bestCandle *VolumeImbalanceCandle
	var bestScore float64
	bestIndex := -1

	for i := lookbackStart; i < len(candles)-1; i++ { // -1 to not pick the current candle
		c := candles[i]

		// Check if volume is significantly above average (2x+ threshold)
		if c.Volume < avgVolume*v.config.MinVolumeSpikeMultiplier {
			continue
		}

		// Score based on volume spike magnitude
		volumeScore := c.Volume / avgVolume

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

		totalScore := volumeScore * bullishBonus * takerBuyRatio

		if totalScore > bestScore {
			bestScore = totalScore
			candleCopy := c
			bestCandle = &candleCopy
			bestIndex = i
		}
	}

	return bestCandle, bestIndex
}

// isConsolidating checks Step 2: Volume declining, price staying in range (sideways)
func (v *VolumeImbalanceDetector) isConsolidating(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	if pattern.ReferenceCandle.Volume == 0 {
		return false
	}

	refIdx := pattern.ReferenceCandle.Index
	if refIdx >= len(candles) {
		return false
	}

	currentIdx := len(candles) - 1
	candlesSinceRef := currentIdx - refIdx

	if candlesSinceRef < v.config.MinConsolidationCandles {
		return false
	}

	// Bounds check: ensure refIdx+1 is valid before accessing
	if refIdx+1 >= len(candles) {
		return false
	}

	// Track consolidation metrics
	referenceHigh := pattern.ReferenceCandle.High
	referenceLow := pattern.ReferenceCandle.Low
	tolerance := v.config.ConsolidationRangeTolerance

	var volumeSum float64
	consolidationLow := candles[refIdx+1].Low
	consolidationHigh := candles[refIdx+1].High
	volumes := make([]float64, 0, candlesSinceRef)

	for i := refIdx + 1; i <= currentIdx; i++ {
		c := candles[i]
		volumeSum += c.Volume
		volumes = append(volumes, c.Volume)

		// Track consolidation range
		if c.Low < consolidationLow {
			consolidationLow = c.Low
		}
		if c.High > consolidationHigh {
			consolidationHigh = c.High
		}

		// Check if price breaks out of range (sideways check)
		if c.High > referenceHigh*(1+tolerance) {
			// Price broke above reference high - not consolidating, could be breakout
			return false
		}
		if c.Low < referenceLow*(1-tolerance*2) {
			// Price broke significantly below - pattern may be failing
			return false
		}
	}

	// Calculate volume trend (should be declining)
	volumeTrend := v.calculateTrend(volumes)

	// Update pattern with consolidation data
	pattern.ConsolidationCandles = candlesSinceRef
	pattern.ConsolidationLow = consolidationLow
	pattern.ConsolidationHigh = consolidationHigh
	pattern.ConsolidationAvgVolume = volumeSum / float64(candlesSinceRef)
	pattern.VolumeTrend = volumeTrend

	// Legacy field mappings
	pattern.DeclineCandles = candlesSinceRef
	pattern.DeclineVolumeSlope = volumeTrend
	pattern.LowestPrice = consolidationLow
	// LowestVolume is a deprecated legacy field - not used in 3-step model
	// Setting to 0 to avoid confusion (it previously incorrectly stored consolidationLow price)
	pattern.LowestVolume = 0

	pattern.UpdatedAt = time.Now()

	// Volume must be declining (negative trend)
	if volumeTrend >= 0 {
		return false
	}

	return true
}

// isBreakoutReady checks Step 3: Volume surge + price breaks reference high
func (v *VolumeImbalanceDetector) isBreakoutReady(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	if len(candles) < 2 {
		return false
	}

	currentCandle := candles[len(candles)-1]

	// Check 1: Volume must surge (50%+ above consolidation average)
	if pattern.ConsolidationAvgVolume <= 0 {
		return false
	}

	volumeSurge := currentCandle.Volume / pattern.ConsolidationAvgVolume
	if volumeSurge < v.config.BreakoutVolumeSurge {
		return false
	}

	// Check 2: Price must break reference high
	if currentCandle.High < pattern.ReferenceCandle.High {
		return false
	}

	// Confirm the close is also above (not just a wick)
	if currentCandle.Close < pattern.ReferenceCandle.High*0.998 { // Within 0.2%
		return false
	}

	// Record breakout candle data
	pattern.BreakoutCandle.Time = currentCandle.Time
	pattern.BreakoutCandle.High = currentCandle.High
	pattern.BreakoutCandle.Volume = currentCandle.Volume

	// Legacy field mapping
	pattern.PumpStartVolume = currentCandle.Volume
	pattern.PumpStartTime = currentCandle.Time

	return true
}

// isPatternInvalid checks if the consolidation pattern has become invalid
func (v *VolumeImbalanceDetector) isPatternInvalid(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	if len(candles) < 2 {
		return false
	}

	currentCandle := candles[len(candles)-1]

	// If price breaks significantly below consolidation low, pattern fails
	if currentCandle.Low < pattern.ConsolidationLow*0.97 { // 3% below
		return true
	}

	// If consolidation takes too long, pattern expires
	if pattern.ConsolidationCandles > v.config.MaxConsolidationCandles {
		return true
	}

	return false
}

// DetectReferenceCandle - legacy method for backward compatibility
func (v *VolumeImbalanceDetector) DetectReferenceCandle(candles []VolumeImbalanceCandle) *VolumeImbalanceCandle {
	candle, _ := v.detectAccumulationStart(candles)
	return candle
}

// TrackDecline - legacy method for backward compatibility
func (v *VolumeImbalanceDetector) TrackDecline(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	return v.isConsolidating(pattern, candles)
}

// CheckEntryTrigger - legacy method for backward compatibility
func (v *VolumeImbalanceDetector) CheckEntryTrigger(pattern *VolumeImbalancePattern, currentCandle VolumeImbalanceCandle) bool {
	if pattern.ReferenceCandle.High == 0 {
		return false
	}

	// Price must cross above the reference candle's high
	if currentCandle.High >= pattern.ReferenceCandle.High {
		// Confirm the close is also above (not just a wick)
		if currentCandle.Close >= pattern.ReferenceCandle.High*0.998 {
			return true
		}
	}

	return false
}

// CalculateRiskReward calculates SL, TP, and R:R ratio using 3-step model
func (v *VolumeImbalanceDetector) CalculateRiskReward(pattern *VolumeImbalancePattern, entryPrice float64) (stopLoss, takeProfit, ratio float64) {
	// Stop loss = below the consolidation low (with buffer)
	stopLoss = pattern.ConsolidationLow * (1 - v.config.StopLossBuffer)

	// Fallback to legacy field if consolidation low not set
	if stopLoss <= 0 && pattern.LowestPrice > 0 {
		stopLoss = pattern.LowestPrice * (1 - v.config.StopLossBuffer)
	}

	// Calculate risk (entry to stop loss)
	risk := entryPrice - stopLoss
	if risk <= 0 {
		// Fallback if stop loss is somehow above entry
		risk = entryPrice * 0.02 // 2% default risk
		stopLoss = entryPrice - risk
	}

	// Take profit = entry + (risk * R:R ratio) for 1:4 R:R
	takeProfit = entryPrice + (risk * v.config.DefaultRiskRewardRatio)
	ratio = v.config.DefaultRiskRewardRatio

	return stopLoss, takeProfit, ratio
}

// ============================================================================
// TRAILING STOP MANAGER (Ravindra's Approach)
// ============================================================================

// TrailingStopManager manages trailing stops according to Ravindra's approach
// Milestones:
// - At 1:2 R:R → Move SL to entry (breakeven, 0 risk)
// - At 1:3 R:R → Move SL to 1:1 level (lock profit)
// - At 1:4 R:R → Take profit (target reached)
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

	// Check for take profit (1:4 R:R target)
	if currentPrice >= t.TakeProfit {
		action = "TAKE_PROFIT"
		return newStopLoss, action
	}

	// At 1:3 R:R → Move SL to 1:1 level (lock profit)
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

	// At 1:2 R:R → Move SL to entry (breakeven, 0 risk)
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

// VolumeImbalanceAnalysis contains the complete 3-step analysis result
type VolumeImbalanceAnalysis struct {
	Symbol    string           `json:"symbol"`
	Mode      GinieTradingMode `json:"mode"`
	Timestamp time.Time        `json:"timestamp"`

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

	// Trailing stop manager for position management (Ravindra's approach)
	// Use this after entry to manage SL according to R:R milestones:
	// - At 1:2 R:R -> Move SL to entry (breakeven)
	// - At 1:3 R:R -> Move SL to 1:1 level (lock profit)
	// - At 1:4 R:R -> Take profit (target reached)
	TrailingStop *TrailingStopManager `json:"trailing_stop,omitempty"`

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

// calculateTrend returns the linear regression slope of values
// Negative slope = declining, Positive slope = increasing
func (v *VolumeImbalanceDetector) calculateTrend(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	n := float64(len(values))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}

func (v *VolumeImbalanceDetector) setReferenceCandle(pattern *VolumeImbalancePattern, candle *VolumeImbalanceCandle, index int) {
	pattern.ReferenceCandle.Time = candle.Time
	pattern.ReferenceCandle.High = candle.High
	pattern.ReferenceCandle.Low = candle.Low
	pattern.ReferenceCandle.Close = candle.Close
	pattern.ReferenceCandle.Volume = candle.Volume
	pattern.ReferenceCandle.TakerBuyVolume = candle.TakerBuyVolume
	pattern.ReferenceCandle.Index = index

	// Initialize consolidation tracking
	pattern.ConsolidationStartIndex = index + 1
	pattern.ConsolidationLow = candle.Low
	pattern.ConsolidationHigh = candle.High

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
	pattern.ConsolidationCandles = 0
	pattern.ConsolidationLow = 0
	pattern.ConsolidationHigh = 0
	pattern.ConsolidationAvgVolume = 0
	pattern.VolumeTrend = 0
	pattern.BreakoutCandle = struct {
		Time   time.Time `json:"time"`
		High   float64   `json:"high"`
		Volume float64   `json:"volume"`
	}{}

	// Reset legacy fields
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

// isDeclineInvalid - legacy method for backward compatibility
func (v *VolumeImbalanceDetector) isDeclineInvalid(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	return v.isPatternInvalid(pattern, candles)
}

// checkExhaustion - legacy method (deprecated in 3-step model)
func (v *VolumeImbalanceDetector) checkExhaustion(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	// In 3-step model, exhaustion is part of consolidation
	return v.isConsolidating(pattern, candles)
}

// detectPump - legacy method (deprecated in 3-step model)
func (v *VolumeImbalanceDetector) detectPump(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) bool {
	// In 3-step model, pump is merged with breakout
	return v.isBreakoutReady(pattern, candles)
}

func (v *VolumeImbalanceDetector) calculateConfidence(pattern *VolumeImbalancePattern, candles []VolumeImbalanceCandle) float64 {
	// Base confidence
	confidence := 50.0

	// Volume spike quality in Step 1 (up to +15)
	avgVol := v.calculateAverageVolume(candles, v.config.LookbackPeriod)
	if avgVol > 0 {
		volumeSpike := pattern.ReferenceCandle.Volume / avgVol
		volumeBonus := math.Min(volumeSpike-2.0, 3.0) * 5 // Up to +15 for 5x volume
		confidence += volumeBonus
	}

	// Consolidation quality in Step 2 (up to +15)
	if pattern.ConsolidationCandles >= 2 && pattern.ConsolidationCandles <= 4 {
		confidence += 15 // Ideal consolidation length
	} else if pattern.ConsolidationCandles >= 2 && pattern.ConsolidationCandles <= 6 {
		confidence += 10
	}

	// Volume trend during consolidation (should be declining)
	if pattern.VolumeTrend < -0.1 {
		confidence += 10 // Strong volume decline
	} else if pattern.VolumeTrend < 0 {
		confidence += 5
	}

	// Breakout volume quality in Step 3 (up to +10)
	if pattern.BreakoutCandle.Volume > 0 && pattern.ConsolidationAvgVolume > 0 {
		breakoutRatio := pattern.BreakoutCandle.Volume / pattern.ConsolidationAvgVolume
		if breakoutRatio >= 2.0 {
			confidence += 10 // Strong breakout volume
		} else if breakoutRatio >= 1.5 {
			confidence += 5
		}
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
		v.logger.Info("[VOLUME_IMBALANCE_3STEP]", allArgs...)
	}
}

// GetTimeframe returns the appropriate timeframe for the trading mode
// IMPORTANT: 15m for scalp validated by backtesting
func (v *VolumeImbalanceDetector) GetTimeframe(mode GinieTradingMode) string {
	switch mode {
	case GinieModeScalp:
		return v.config.ScalpTimeframe // 15m (validated)
	case GinieModeSwing:
		return v.config.SwingTimeframe // 1h
	case GinieModePosition:
		return v.config.PositionTimeframe // 4h
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
