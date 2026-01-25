// Package entrydecision provides pattern matching logic for the Entry Decision System.
// Epic 14: Chain Trading System - Story 14.9: Pattern Matcher for Volume Imbalance
package entrydecision

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// VOLUME IMBALANCE PATTERN MATCHER
// ============================================================================
//
// This pattern matcher implements the 3-step Volume Imbalance pattern detection
// for the Entry Decision System. It tracks multi-step patterns across symbols
// and determines entry readiness based on Ravindra's Volume Imbalance methodology.
//
// THE 3-STEP PATTERN:
//
// Step 1: VOLUME SPIKE (Accumulation Start)
//   - Detects significant volume above average (2x+ threshold)
//   - Creates reference candle for entry trigger level
//
// Step 2: CONSOLIDATION (Market Digesting)
//   - Volume declining over multiple candles
//   - Price stays in sideways range (not breaking reference high/low)
//
// Step 3: BREAKOUT (Institutional Push)
//   - Volume surges (50%+ above consolidation average)
//   - Price breaks above reference high
//   - Entry signal generated

// Candle represents OHLCV data for pattern analysis.
type Candle struct {
	Time           time.Time `json:"time"`
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	Close          float64   `json:"close"`
	Volume         float64   `json:"volume"`
	TakerBuyVolume float64   `json:"taker_buy_volume,omitempty"` // Optional: for enhanced analysis
}

// PatternMatcherConfig holds configurable thresholds for pattern detection.
type PatternMatcherConfig struct {
	// Step 1: Volume Spike Detection
	MinVolumeSpikeMultiplier float64 `json:"min_volume_spike_multiplier"` // Default: 2.0 (2x avg volume)
	LookbackPeriod           int     `json:"lookback_period"`             // Default: 20 candles

	// Step 2: Consolidation Thresholds
	MinConsolidationCandles     int     `json:"min_consolidation_candles"`      // Default: 2
	MaxConsolidationCandles     int     `json:"max_consolidation_candles"`      // Default: 6
	ConsolidationRangeTolerance float64 `json:"consolidation_range_tolerance"`  // Default: 0.01 (1%)

	// Step 3: Breakout Thresholds
	BreakoutVolumeSurge float64 `json:"breakout_volume_surge"` // Default: 1.5 (50% above avg)

	// Pattern Expiration
	PatternExpirationMinutes int `json:"pattern_expiration_minutes"` // Default: 60 (1 hour)
}

// DefaultPatternMatcherConfig returns the default configuration.
func DefaultPatternMatcherConfig() *PatternMatcherConfig {
	return &PatternMatcherConfig{
		MinVolumeSpikeMultiplier:    2.0,
		LookbackPeriod:              20,
		MinConsolidationCandles:     2,
		MaxConsolidationCandles:     6,
		ConsolidationRangeTolerance: 0.01,
		BreakoutVolumeSurge:         1.5,
		PatternExpirationMinutes:    60,
	}
}

// PatternState holds the internal state for pattern tracking.
type PatternState struct {
	// Reference candle from Step 1
	ReferenceCandle     *Candle   `json:"reference_candle,omitempty"`
	ReferenceCandleIdx  int       `json:"reference_candle_idx"`
	AverageVolumeAtSpike float64  `json:"average_volume_at_spike"` // Avg volume when spike detected

	// Consolidation tracking from Step 2
	ConsolidationStartIdx int       `json:"consolidation_start_idx"`
	ConsolidationCandles  int       `json:"consolidation_candles"`
	ConsolidationLow      float64   `json:"consolidation_low"`
	ConsolidationHigh     float64   `json:"consolidation_high"`
	ConsolidationAvgVol   float64   `json:"consolidation_avg_vol"`
	VolumeTrend           float64   `json:"volume_trend"` // Slope of volume (negative = declining)

	// Breakout candle from Step 3
	BreakoutCandle *Candle `json:"breakout_candle,omitempty"`

	// Direction detection
	Direction string `json:"direction"` // "long" or "short"
}

// VolumeImbalancePatternMatcher tracks and matches Volume Imbalance patterns.
type VolumeImbalancePatternMatcher struct {
	config *PatternMatcherConfig

	// Active pattern tracking (symbol -> PatternProgress)
	patterns map[string]*PatternProgress
	// Internal state tracking (symbol -> PatternState)
	states   map[string]*PatternState
	mu       sync.RWMutex
}

// NewVolumeImbalancePatternMatcher creates a new pattern matcher with the given config.
func NewVolumeImbalancePatternMatcher(config *PatternMatcherConfig) *VolumeImbalancePatternMatcher {
	if config == nil {
		config = DefaultPatternMatcherConfig()
	}
	return &VolumeImbalancePatternMatcher{
		config:   config,
		patterns: make(map[string]*PatternProgress),
		states:   make(map[string]*PatternState),
	}
}

// ============================================================================
// MAIN PATTERN MATCHING INTERFACE
// ============================================================================

// MatchPattern analyzes candles for a symbol and returns the pattern match result.
// This is the main entry point for pattern matching.
// Returns a CoinMatch if pattern is found/progressed, nil if no pattern detected.
func (m *VolumeImbalancePatternMatcher) MatchPattern(
	symbol string,
	mode string,
	timeframe string,
	candles []Candle,
) *CoinMatch {
	if len(candles) < m.config.LookbackPeriod+5 {
		return nil // Not enough data
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Get or create pattern progress
	patternKey := m.patternKey(symbol, mode, timeframe)
	progress := m.patterns[patternKey]
	state := m.states[patternKey]

	if progress == nil {
		progress = NewPatternProgress(symbol, "volume_imbalance", "ravindra_volume_imbalance", mode, timeframe, 3)
		progress.ExpiresAt = time.Now().Add(time.Duration(m.config.PatternExpirationMinutes) * time.Minute)
		m.patterns[patternKey] = progress

		state = &PatternState{}
		m.states[patternKey] = state
	}

	// Check expiration
	if progress.IsExpired() {
		m.resetPatternUnlocked(patternKey, "Pattern expired")
		progress.SetStatus(PatternStatusExpired)
		return m.createCoinMatch(progress, state)
	}

	// Process based on current status
	switch progress.Status {
	case PatternStatusWatching:
		m.processStep1(patternKey, progress, state, candles)

	case PatternStatusAccumulation:
		m.processStep2(patternKey, progress, state, candles)

	case PatternStatusConsolidating:
		m.processStep3(patternKey, progress, state, candles)

	case PatternStatusReady, PatternStatusFailed, PatternStatusExpired:
		// Terminal states - return current match
	}

	return m.createCoinMatch(progress, state)
}

// ============================================================================
// STEP 1: VOLUME SPIKE DETECTION
// ============================================================================

// processStep1 detects volume spikes (accumulation start).
func (m *VolumeImbalancePatternMatcher) processStep1(
	patternKey string,
	progress *PatternProgress,
	state *PatternState,
	candles []Candle,
) {
	spike, spikeIdx, avgVol := m.detectVolumeSpike(candles)
	if spike == nil {
		return // No spike detected
	}

	// Update state with reference candle
	state.ReferenceCandle = spike
	state.ReferenceCandleIdx = spikeIdx
	state.AverageVolumeAtSpike = avgVol
	state.ConsolidationLow = spike.Low
	state.ConsolidationHigh = spike.High

	// Update step details
	progress.StepDetails[0] = StepDetail{
		StepNumber:  1,
		Name:        "Volume Spike",
		Completed:   true,
		CompletedAt: time.Now(),
		Progress:    fmt.Sprintf("%.1fx avg", spike.Volume/avgVol),
		Details:     fmt.Sprintf("Reference high: %.6f", spike.High),
	}

	// Advance to Step 2
	progress.AdvanceStep(fmt.Sprintf("Volume spike at %.2fx average", spike.Volume/avgVol))
	progress.SetStatus(PatternStatusAccumulation)
	progress.ExpiresAt = time.Now().Add(time.Duration(m.config.PatternExpirationMinutes) * time.Minute)
}

// detectVolumeSpike finds a candle with volume significantly above average.
// Returns the spike candle, its index, and the average volume, or (nil, -1, 0) if not found.
func (m *VolumeImbalancePatternMatcher) detectVolumeSpike(candles []Candle) (*Candle, int, float64) {
	if len(candles) < m.config.LookbackPeriod {
		return nil, -1, 0
	}

	avgVolume := m.calculateAverageVolume(candles, m.config.LookbackPeriod)
	if avgVolume == 0 {
		return nil, -1, 0
	}

	// Look for spike in recent candles (not the most current one)
	lookbackStart := len(candles) - m.config.LookbackPeriod
	if lookbackStart < 0 {
		lookbackStart = 0
	}

	var bestCandle *Candle
	var bestScore float64
	bestIndex := -1

	for i := lookbackStart; i < len(candles)-1; i++ {
		c := &candles[i]

		// Check volume threshold
		if c.Volume < avgVolume*m.config.MinVolumeSpikeMultiplier {
			continue
		}

		// Score based on volume magnitude
		volumeScore := c.Volume / avgVolume

		// Bonus for bullish candles (close > open)
		bullishBonus := 1.0
		if c.Close > c.Open {
			bullishBonus = 1.2
		}

		// Bonus for taker buy volume if available
		takerBonus := 1.0
		if c.Volume > 0 && c.TakerBuyVolume > 0 {
			takerBonus = 1 + (c.TakerBuyVolume/c.Volume)*0.5
		}

		totalScore := volumeScore * bullishBonus * takerBonus

		if totalScore > bestScore {
			bestScore = totalScore
			candleCopy := *c
			bestCandle = &candleCopy
			bestIndex = i
		}
	}

	return bestCandle, bestIndex, avgVolume
}

// ============================================================================
// STEP 2: CONSOLIDATION DETECTION
// ============================================================================

// processStep2 tracks consolidation after volume spike.
func (m *VolumeImbalancePatternMatcher) processStep2(
	patternKey string,
	progress *PatternProgress,
	state *PatternState,
	candles []Candle,
) {
	if state.ReferenceCandle == nil {
		m.resetPatternUnlocked(patternKey, "Missing reference candle")
		return
	}

	isConsolidating, candleCount, consolidationData := m.isInConsolidation(state, candles)

	if !isConsolidating {
		// Check if pattern should be invalidated
		if m.isPatternInvalid(state, candles) {
			m.resetPatternUnlocked(patternKey, "Pattern invalidated during consolidation")
			progress.SetStatus(PatternStatusFailed)
			return
		}
		// Not enough candles yet - keep waiting
		return
	}

	// Update consolidation state
	state.ConsolidationCandles = candleCount
	state.ConsolidationLow = consolidationData.low
	state.ConsolidationHigh = consolidationData.high
	state.ConsolidationAvgVol = consolidationData.avgVolume
	state.VolumeTrend = consolidationData.volumeTrend

	// Update step details
	progress.StepDetails[1] = StepDetail{
		StepNumber:  2,
		Name:        "Consolidation",
		Completed:   true,
		CompletedAt: time.Now(),
		Progress:    fmt.Sprintf("%d candles", candleCount),
		Details:     fmt.Sprintf("Volume trend: %.4f", consolidationData.volumeTrend),
	}

	// Advance to Step 3
	progress.AdvanceStep(fmt.Sprintf("Consolidation: %d candles, volume declining", candleCount))
	progress.SetStatus(PatternStatusConsolidating)
}

// consolidationData holds intermediate consolidation metrics.
type consolidationData struct {
	low         float64
	high        float64
	avgVolume   float64
	volumeTrend float64
}

// isInConsolidation checks if price is consolidating after volume spike.
// Returns true if consolidation criteria are met, along with candle count and metrics.
func (m *VolumeImbalancePatternMatcher) isInConsolidation(
	state *PatternState,
	candles []Candle,
) (bool, int, *consolidationData) {
	if state.ReferenceCandle == nil || state.ReferenceCandleIdx >= len(candles) {
		return false, 0, nil
	}

	refIdx := state.ReferenceCandleIdx
	currentIdx := len(candles) - 1
	candlesSinceRef := currentIdx - refIdx

	if candlesSinceRef < m.config.MinConsolidationCandles {
		return false, 0, nil
	}

	// Bounds check
	if refIdx+1 >= len(candles) {
		return false, 0, nil
	}

	referenceHigh := state.ReferenceCandle.High
	referenceLow := state.ReferenceCandle.Low
	tolerance := m.config.ConsolidationRangeTolerance

	var volumeSum float64
	consolidationLow := candles[refIdx+1].Low
	consolidationHigh := candles[refIdx+1].High
	volumes := make([]float64, 0, candlesSinceRef)

	for i := refIdx + 1; i <= currentIdx; i++ {
		c := &candles[i]
		volumeSum += c.Volume
		volumes = append(volumes, c.Volume)

		// Track consolidation range
		if c.Low < consolidationLow {
			consolidationLow = c.Low
		}
		if c.High > consolidationHigh {
			consolidationHigh = c.High
		}

		// Check if price breaks out of range (not consolidating)
		if c.High > referenceHigh*(1+tolerance) {
			return false, 0, nil
		}
		if c.Low < referenceLow*(1-tolerance*2) {
			return false, 0, nil
		}
	}

	// Calculate volume trend (should be declining = negative)
	volumeTrend := m.calculateTrend(volumes)
	if volumeTrend >= 0 {
		return false, 0, nil // Volume not declining
	}

	data := &consolidationData{
		low:         consolidationLow,
		high:        consolidationHigh,
		avgVolume:   volumeSum / float64(candlesSinceRef),
		volumeTrend: volumeTrend,
	}

	return true, candlesSinceRef, data
}

// ============================================================================
// STEP 3: BREAKOUT DETECTION
// ============================================================================

// processStep3 detects breakout with volume surge.
func (m *VolumeImbalancePatternMatcher) processStep3(
	patternKey string,
	progress *PatternProgress,
	state *PatternState,
	candles []Candle,
) {
	if state.ReferenceCandle == nil || state.ConsolidationAvgVol == 0 {
		return
	}

	// Check if pattern should be invalidated
	if m.isPatternInvalid(state, candles) {
		m.resetPatternUnlocked(patternKey, "Pattern invalidated waiting for breakout")
		progress.SetStatus(PatternStatusFailed)
		return
	}

	// Check for breakout
	isBreakout, direction, breakoutCandle := m.isBreakoutReady(state, candles)
	if !isBreakout {
		// Update step progress while waiting
		progress.StepDetails[2] = StepDetail{
			StepNumber: 3,
			Name:       "Breakout",
			Completed:  false,
			Progress:   "Waiting for breakout",
			Details:    fmt.Sprintf("Target: %.6f", state.ReferenceCandle.High),
		}
		return
	}

	// Breakout detected!
	state.BreakoutCandle = breakoutCandle
	state.Direction = direction

	// Update step details
	progress.StepDetails[2] = StepDetail{
		StepNumber:  3,
		Name:        "Breakout",
		Completed:   true,
		CompletedAt: time.Now(),
		Progress:    fmt.Sprintf("%.1fx vol surge", breakoutCandle.Volume/state.ConsolidationAvgVol),
		Details:     fmt.Sprintf("Direction: %s, price: %.6f", direction, breakoutCandle.Close),
	}

	// Pattern complete!
	progress.AdvanceStep(fmt.Sprintf("Breakout confirmed, direction: %s", direction))
	progress.SetStatus(PatternStatusReady)
}

// isBreakoutReady checks if current candle shows breakout with volume surge.
// Returns (true, direction, candle) if breakout is detected.
func (m *VolumeImbalancePatternMatcher) isBreakoutReady(
	state *PatternState,
	candles []Candle,
) (bool, string, *Candle) {
	if len(candles) < 2 {
		return false, "", nil
	}

	currentCandle := &candles[len(candles)-1]

	// Check volume surge (50%+ above consolidation average)
	if state.ConsolidationAvgVol <= 0 {
		return false, "", nil
	}

	volumeSurge := currentCandle.Volume / state.ConsolidationAvgVol
	if volumeSurge < m.config.BreakoutVolumeSurge {
		return false, "", nil
	}

	// Check for LONG breakout (price breaks above reference high)
	if currentCandle.High >= state.ReferenceCandle.High {
		// Confirm close is also above (not just a wick)
		if currentCandle.Close >= state.ReferenceCandle.High*0.998 {
			candleCopy := *currentCandle
			return true, "long", &candleCopy
		}
	}

	// Check for SHORT breakout (price breaks below consolidation low)
	if currentCandle.Low <= state.ConsolidationLow {
		// Confirm close is also below (not just a wick)
		if currentCandle.Close <= state.ConsolidationLow*1.002 {
			candleCopy := *currentCandle
			return true, "short", &candleCopy
		}
	}

	return false, "", nil
}

// ============================================================================
// PATTERN INVALIDATION AND RESET
// ============================================================================

// isPatternInvalid checks if the pattern should be marked as failed.
func (m *VolumeImbalancePatternMatcher) isPatternInvalid(state *PatternState, candles []Candle) bool {
	if len(candles) < 2 || state.ReferenceCandle == nil {
		return false
	}

	currentCandle := &candles[len(candles)-1]

	// Price breaks significantly below consolidation low (3% below)
	if state.ConsolidationLow > 0 && currentCandle.Low < state.ConsolidationLow*0.97 {
		return true
	}

	// Consolidation taking too long
	if state.ReferenceCandleIdx > 0 {
		candlesSinceRef := len(candles) - 1 - state.ReferenceCandleIdx
		if candlesSinceRef > m.config.MaxConsolidationCandles+2 {
			return true
		}
	}

	return false
}

// resetPatternUnlocked resets the pattern to watching state.
// Must be called while holding the lock.
func (m *VolumeImbalancePatternMatcher) resetPatternUnlocked(patternKey string, reason string) {
	progress := m.patterns[patternKey]
	if progress == nil {
		return
	}

	// Reset to watching state
	progress.CurrentStep = 1
	progress.Status = PatternStatusWatching
	progress.UpdatedAt = time.Now()
	progress.StepDetails = make([]StepDetail, 3)

	// Reset internal state
	m.states[patternKey] = &PatternState{}
}

// ResetPattern resets a pattern to watching state (thread-safe).
func (m *VolumeImbalancePatternMatcher) ResetPattern(symbol, mode, timeframe string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	patternKey := m.patternKey(symbol, mode, timeframe)
	m.resetPatternUnlocked(patternKey, "Manual reset")
}

// ============================================================================
// PATTERN RETRIEVAL AND MANAGEMENT
// ============================================================================

// GetPattern returns the current pattern progress for a symbol (thread-safe).
func (m *VolumeImbalancePatternMatcher) GetPattern(symbol, mode, timeframe string) *PatternProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patternKey := m.patternKey(symbol, mode, timeframe)
	return m.patterns[patternKey]
}

// GetAllPatterns returns all tracked patterns (thread-safe).
func (m *VolumeImbalancePatternMatcher) GetAllPatterns() []*PatternProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PatternProgress, 0, len(m.patterns))
	for _, p := range m.patterns {
		result = append(result, p)
	}
	return result
}

// CleanupExpiredPatterns removes expired patterns and returns the count.
func (m *VolumeImbalancePatternMatcher) CleanupExpiredPatterns() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, progress := range m.patterns {
		if progress.IsExpired() || now.After(progress.ExpiresAt) {
			delete(m.patterns, key)
			delete(m.states, key)
			removed++
		}
	}

	return removed
}

// GetReadyPatterns returns all patterns that are ready for entry.
func (m *VolumeImbalancePatternMatcher) GetReadyPatterns() []*PatternProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PatternProgress, 0)
	for _, p := range m.patterns {
		if p.Status == PatternStatusReady {
			result = append(result, p)
		}
	}
	return result
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// patternKey creates a unique key for pattern tracking.
func (m *VolumeImbalancePatternMatcher) patternKey(symbol, mode, timeframe string) string {
	return fmt.Sprintf("%s:%s:%s", symbol, mode, timeframe)
}

// createCoinMatch converts pattern progress to a CoinMatch for API responses.
func (m *VolumeImbalancePatternMatcher) createCoinMatch(progress *PatternProgress, state *PatternState) *CoinMatch {
	if progress == nil {
		return nil
	}

	cm := progress.ToCoinMatch()

	// Add direction and price from state if available
	if state != nil {
		cm.Direction = state.Direction
		if state.BreakoutCandle != nil {
			cm.CurrentPrice = state.BreakoutCandle.Close
		} else if state.ReferenceCandle != nil {
			cm.CurrentPrice = state.ReferenceCandle.Close
		}
	}

	return &cm
}

// calculateAverageVolume computes average volume over the lookback period.
func (m *VolumeImbalancePatternMatcher) calculateAverageVolume(candles []Candle, period int) float64 {
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

// calculateTrend computes linear regression slope of values.
// Negative = declining, Positive = increasing.
func (m *VolumeImbalancePatternMatcher) calculateTrend(values []float64) float64 {
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

// ============================================================================
// BATCH MATCHING FOR MULTIPLE SYMBOLS
// ============================================================================

// MatchResult represents the result of pattern matching for one symbol.
type MatchResult struct {
	Symbol    string     `json:"symbol"`
	Mode      string     `json:"mode"`
	Timeframe string     `json:"timeframe"`
	Match     *CoinMatch `json:"match,omitempty"`
	Error     error      `json:"error,omitempty"`
}

// MatchPatterns analyzes multiple symbols and returns results.
// This is useful for batch processing multiple coins.
func (m *VolumeImbalancePatternMatcher) MatchPatterns(
	mode string,
	timeframe string,
	candlesBySymbol map[string][]Candle,
) []MatchResult {
	results := make([]MatchResult, 0, len(candlesBySymbol))

	for symbol, candles := range candlesBySymbol {
		result := MatchResult{
			Symbol:    symbol,
			Mode:      mode,
			Timeframe: timeframe,
		}

		match := m.MatchPattern(symbol, mode, timeframe, candles)
		result.Match = match

		results = append(results, result)
	}

	return results
}

// ============================================================================
// STRATEGY MATCH GENERATION
// ============================================================================

// ToStrategyMatch converts all tracked patterns to a StrategyMatch for API responses.
func (m *VolumeImbalancePatternMatcher) ToStrategyMatch(mode, timeframe string) *StrategyMatch {
	sm := NewStrategyMatch(mode, "volume_imbalance", "ravindra_volume_imbalance", StrategyTypePattern, timeframe)
	sm.WithTimeframes([]string{timeframe})
	sm.WithRequirements(&StrategyRequirements{
		Timeframes: []string{timeframe},
		DataFields: []string{"volume", "ohlc"},
		Description: "3-step Volume Imbalance pattern: Volume Spike -> Consolidation -> Breakout",
	})

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Add all patterns for this mode/timeframe as coins
	for key, progress := range m.patterns {
		if progress.Mode == mode && progress.Timeframe == timeframe {
			state := m.states[key]
			cm := progress.ToCoinMatch()
			if state != nil {
				cm.Direction = state.Direction
				if state.BreakoutCandle != nil {
					cm.CurrentPrice = state.BreakoutCandle.Close
				}
			}
			sm.AddCoin(cm)
		}
	}

	return sm
}
