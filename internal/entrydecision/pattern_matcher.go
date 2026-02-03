// Package entrydecision provides pattern matching logic for the Entry Decision System.
// Epic 14: Chain Trading System - Story 14.9: Pattern Matcher for Volume Imbalance
package entrydecision

import (
	"encoding/json"
	"fmt"
	"log"
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
	OpenTime       time.Time `json:"open_time"`                  // Candle open time (use this to identify the candle)
	Time           time.Time `json:"time"`                       // Candle close time (for backward compatibility)
	Open           float64   `json:"open"`
	High           float64   `json:"high"`
	Low            float64   `json:"low"`
	Close          float64   `json:"close"`
	Volume         float64   `json:"volume"`
	TakerBuyVolume float64   `json:"taker_buy_volume,omitempty"` // Optional: for enhanced analysis
}

// PatternMatcherConfig holds configurable thresholds for pattern detection.
// IMPORTANT: Default values are from BACKTESTED walk-forward testing (Dec 2025 - Jan 2026).
type PatternMatcherConfig struct {
	// Trade Direction: "long", "short", or "both"
	// - "long": Only GREEN (bullish) candles qualify as reference, look for upward breakout
	// - "short": Only RED (bearish) candles qualify as reference, look for downward breakout
	// - "both": Accept any candle, direction determined by candle color
	Direction string `json:"direction"` // Default: "long"

	// Step 1: Volume Spike Detection
	MinVolumeSpikeMultiplier float64 `json:"min_volume_spike_multiplier"` // Default: 3.0 (3x avg volume - BACKTESTED)
	LookbackPeriod           int     `json:"lookback_period"`             // Default: 5 candles (BACKTESTED)

	// Step 2: Consolidation Thresholds
	MinConsolidationCandles     int     `json:"min_consolidation_candles"`      // Default: 2
	MaxConsolidationCandles     int     `json:"max_consolidation_candles"`      // Default: 6
	ConsolidationRangeTolerance float64 `json:"consolidation_range_tolerance"`  // Default: 0.01 (1%)

	// Step 3: Breakout Thresholds
	BreakoutVolumeSurge float64 `json:"breakout_volume_surge"` // Default: 1.5 (50% above avg)

	// Pattern Expiration
	PatternExpirationMinutes int `json:"pattern_expiration_minutes"` // Default: 60 (1 hour)
	ReadyExpirationSeconds   int `json:"ready_expiration_seconds"`   // Default: 60 (ready patterns expire after 60s if not executed)

	// ============================================================================
	// OPTIMIZED FILTERS (from backtesting)
	// These filters were validated to improve win rate from 18% to 40%+
	// ============================================================================

	// Pre-trend Direction Filter: Pattern works best after a pullback (DOWN trend)
	// Requires the 5 candles before reference to show downward movement
	RequirePreTrendDown bool `json:"require_pre_trend_down"` // Default: true (recommended)

	// Reference Candle Body Filter: Strong bodies indicate institutional activity
	// Body ratio = |close - open| / (high - low)
	MinReferenceBodyRatio float64 `json:"min_reference_body_ratio"` // Default: 0.5 (50% body)

	// Entry Breakout Filter: Entry must break consolidation high for confirmation
	RequireEntryBreakout bool `json:"require_entry_breakout"` // Default: true (recommended)

	// Time Filter: Pattern works best during US market hours (14 UTC = 9 AM EST)
	// Set to -1 to disable time filtering
	PreferredHourUTC int `json:"preferred_hour_utc"` // Default: -1 (disabled, 14 = US open)
}

// DefaultPatternMatcherConfig returns the default configuration.
// NOTE: These defaults are from BACKTESTED walk-forward testing (Dec 2025 - Jan 2026).
// Results: 51 trades, 47.1% WR, +1147% net return.
// The pre-trend and entry breakout filters improve win rate from ~18% to ~40%.
//
// IMPORTANT: Use NewPatternMatcherConfigFromSettings() to load user-configured values
// from the database/cache. Hard-coded defaults should only be used as fallback.
func DefaultPatternMatcherConfig() *PatternMatcherConfig {
	return &PatternMatcherConfig{
		// Trade direction - determines which candle colors qualify as reference
		Direction: "long", // Default: only GREEN candles, look for upward breakout

		// Core pattern detection - BACKTESTED VALUES
		MinVolumeSpikeMultiplier:    3.0,  // Was 2.0 - higher threshold filters noise (BACKTESTED)
		LookbackPeriod:              5,    // Was 20 - shorter lookback for faster patterns (BACKTESTED)
		MinConsolidationCandles:     1,    // Was 2 - allow immediate breakout (BACKTESTED)
		MaxConsolidationCandles:     999,  // Was 6 - no upper limit (BACKTESTED)
		ConsolidationRangeTolerance: 0.01,
		BreakoutVolumeSurge:         1.0,  // Was 1.5 - equal to consolidation avg is sufficient (BACKTESTED)
		PatternExpirationMinutes:    60,
		ReadyExpirationSeconds:      60,   // Ready patterns expire after 60 seconds if not executed

		// Optimized filters (recommended: all enabled)
		RequirePreTrendDown:   false, // BACKTESTED: Original strategy did NOT use this filter
		MinReferenceBodyRatio: 0.0,  // Disabled by default (0.5 for strict mode)
		RequireEntryBreakout:  true, // Entry must break consolidation high
		PreferredHourUTC:      -1,   // Disabled (-1), set to 14 for US market hours
	}
}

// NewPatternMatcherConfigFromSettings creates a PatternMatcherConfig from user's
// sub-strategy settings loaded from database/cache.
//
// This is the PREFERRED way to create a pattern matcher config - it respects
// user-configured values from default-settings.json → database → cache.
//
// Expected settings map structure (from SubStrategySettings.Settings):
//
//	{
//	  "pattern_detection": {
//	    "direction": "long",          // "long", "short", or "both"
//	    "reference_lookback_candles": 5,
//	    "volume_spike_threshold": 3.0,
//	    "breakout_volume_surge": 1.0,
//	    "max_sl_percent": 1.5,
//	    "max_pattern_age_mins": 60
//	  }
//	}
func NewPatternMatcherConfigFromSettings(settings map[string]interface{}) *PatternMatcherConfig {
	// Start with defaults (backtested values)
	config := DefaultPatternMatcherConfig()

	if settings == nil {
		return config
	}

	// Extract pattern_detection settings
	patternDetection, ok := settings["pattern_detection"].(map[string]interface{})
	if !ok {
		return config
	}

	// Override with user-configured values

	// Direction: "long", "short", or "both"
	if val, ok := patternDetection["direction"].(string); ok && val != "" {
		// Validate direction value
		if val == "long" || val == "short" || val == "both" {
			config.Direction = val
		}
	}

	if val, ok := patternDetection["reference_lookback_candles"].(float64); ok && val > 0 {
		config.LookbackPeriod = int(val)
	}

	if val, ok := patternDetection["volume_spike_threshold"].(float64); ok && val > 0 {
		config.MinVolumeSpikeMultiplier = val
	}

	if val, ok := patternDetection["breakout_volume_surge"].(float64); ok && val > 0 {
		config.BreakoutVolumeSurge = val
	}

	if val, ok := patternDetection["min_consolidation_candles"].(float64); ok && val >= 0 {
		config.MinConsolidationCandles = int(val)
	}

	if val, ok := patternDetection["max_consolidation_candles"].(float64); ok && val > 0 {
		config.MaxConsolidationCandles = int(val)
	}

	if val, ok := patternDetection["max_pattern_age_mins"].(float64); ok && val > 0 {
		config.PatternExpirationMinutes = int(val)
	}

	// Optional filters
	if val, ok := patternDetection["require_pre_trend_down"].(bool); ok {
		config.RequirePreTrendDown = val
	}

	if val, ok := patternDetection["min_reference_body_ratio"].(float64); ok {
		config.MinReferenceBodyRatio = val
	}

	if val, ok := patternDetection["require_entry_breakout"].(bool); ok {
		config.RequireEntryBreakout = val
	}

	if val, ok := patternDetection["preferred_hour_utc"].(float64); ok {
		config.PreferredHourUTC = int(val)
	}

	return config
}

// ParseSubStrategySettingsJSON parses JSON bytes from database into a settings map.
// This is used to convert SubStrategySettings.Settings (json.RawMessage) to the
// map[string]interface{} format expected by NewPatternMatcherConfigFromSettings.
func ParseSubStrategySettingsJSON(data []byte) map[string]interface{} {
	if len(data) == 0 || string(data) == "{}" || string(data) == "null" {
		return nil
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil
	}
	return settings
}

// PatternState holds the internal state for pattern tracking.
type PatternState struct {
	// Reference candle from Step 1
	ReferenceCandle      *Candle   `json:"reference_candle,omitempty"`
	ReferenceCandleIdx   int       `json:"reference_candle_idx"`
	ReferenceDetectedAt  time.Time `json:"reference_detected_at"`    // When reference candle was detected
	AverageVolumeAtSpike float64   `json:"average_volume_at_spike"`  // Avg volume when spike detected

	// Consolidation tracking from Step 2
	ConsolidationStartIdx int       `json:"consolidation_start_idx"`
	ConsolidationCandles  int       `json:"consolidation_candles"`
	ConsolidationLow      float64   `json:"consolidation_low"`
	ConsolidationHigh     float64   `json:"consolidation_high"`
	ConsolidationAvgVol   float64   `json:"consolidation_avg_vol"`
	VolumeTrend           float64   `json:"volume_trend"` // Slope of volume (negative = declining)

	// Step 2 Progress Bar fields (for UI display)
	ConsolidationAvgVolumeMultiplier float64 `json:"consolidation_avg_volume_multiplier"` // Avg vol multiplier from ref to last candle
	ConsolidationAvgPrice            float64 `json:"consolidation_avg_price"`             // Avg price from ref to last candle
	CurrentCandleVolumeMultiplier    float64 `json:"current_candle_volume_multiplier"`    // Current candle's vol multiplier

	// Breakout candle from Step 3
	BreakoutCandle *Candle `json:"breakout_candle,omitempty"`

	// Direction detection
	Direction string `json:"direction"` // "long" or "short"

	// Ready state tracking
	ReadyAt            time.Time `json:"ready_at"`              // When pattern became ready (UTC)
	EntryPrice         float64   `json:"entry_price"`           // Calculated entry price for order
	BreakoutVolumeMultiplier float64 `json:"breakout_volume_multiplier"` // Volume multiplier at breakout

	// Day High/Low tracking (efficient approach - fetched once, updated in real-time)
	DayHigh           float64   `json:"day_high"`             // Day's highest price
	DayLow            float64   `json:"day_low"`              // Day's lowest price
	DayHighLowFetched bool      `json:"day_high_low_fetched"` // Flag: was initial 24h data fetched?
	AvgPrice5         float64   `json:"avg_price_5"`          // 5-candle average price (for progress bar)
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

// ReloadConfig updates the pattern matcher configuration without resetting pattern progress.
// This allows dynamic configuration updates (e.g., when user changes strategy settings)
// while preserving existing pattern detection state.
func (m *VolumeImbalancePatternMatcher) ReloadConfig(newConfig *PatternMatcherConfig) {
	if newConfig == nil {
		log.Printf("[PATTERN] ReloadConfig called with nil config, ignoring")
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	oldDirection := ""
	if m.config != nil {
		oldDirection = m.config.Direction
	}

	m.config = newConfig

	log.Printf("[PATTERN] Config reloaded: direction=%s (was %s), volume_spike=%.1fx, lookback=%d, breakout_surge=%.1fx",
		newConfig.Direction, oldDirection,
		newConfig.MinVolumeSpikeMultiplier,
		newConfig.LookbackPeriod,
		newConfig.BreakoutVolumeSurge)
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
		// 2-step pattern: Step 1 = Reference Candle (Volume Spike), Step 2 = Breakout Entry
		// After Step 1, we WATCH for breakout (consolidation happens but isn't a separate "step")
		// CurrentStep starts at 1 (watching for Step 1)
		progress = NewPatternProgress(symbol, "volume_imbalance", "ravindra_volume_imbalance", mode, timeframe, 2)
		progress.ExpiresAt = time.Now().Add(time.Duration(m.config.PatternExpirationMinutes) * time.Minute)
		m.patterns[patternKey] = progress

		state = &PatternState{}
		m.states[patternKey] = state

		// Initialize day high/low from available candles
		// This gives us a starting point; real-time updates will refine it
		if len(candles) > 0 {
			state.DayHigh = candles[0].High
			state.DayLow = candles[0].Low
			for _, c := range candles {
				if c.High > state.DayHigh {
					state.DayHigh = c.High
				}
				if c.Low < state.DayLow {
					state.DayLow = c.Low
				}
			}
			state.DayHighLowFetched = true // Mark as initialized from candles
		}
	}

	// Check expiration
	if progress.IsExpired() {
		m.resetPatternUnlocked(patternKey, "Pattern expired")
		progress.SetStatus(PatternStatusExpired)
		return m.createCoinMatchWithCandles(progress, state, candles)
	}

	// Process based on current status
	// 2-STEP PATTERN:
	// Step 1: Volume Spike (Reference Candle) - status goes WATCHING → ACCUMULATION
	// Step 2: Breakout Entry - status goes ACCUMULATION/CONSOLIDATING → READY
	switch progress.Status {
	case PatternStatusWatching:
		// Looking for Step 1: Volume Spike (Reference Candle)
		m.processStep1(patternKey, progress, state, candles)

	case PatternStatusAccumulation, PatternStatusConsolidating:
		// Step 1 complete, now watching for Step 2: Breakout
		// processStep2 handles consolidation monitoring AND breakout detection
		m.processStep2(patternKey, progress, state, candles)

	case PatternStatusReady:
		// Check if ready pattern has expired (not executed within timeout)
		if !state.ReadyAt.IsZero() && m.config.ReadyExpirationSeconds > 0 {
			elapsed := time.Since(state.ReadyAt).Seconds()
			if elapsed > float64(m.config.ReadyExpirationSeconds) {
				log.Printf("[PATTERN] %s - Ready pattern expired after %.0f seconds (limit: %d)",
					symbol, elapsed, m.config.ReadyExpirationSeconds)
				m.resetPatternUnlocked(patternKey, "Ready pattern expired - not executed in time")
				progress.SetStatus(PatternStatusExpired)
			}
		}

	case PatternStatusFailed, PatternStatusExpired:
		// Terminal failed/expired states - return current match
	}

	return m.createCoinMatchWithCandles(progress, state, candles)
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
	state.ReferenceDetectedAt = time.Now()
	state.AverageVolumeAtSpike = avgVol
	state.ConsolidationLow = spike.Low
	state.ConsolidationHigh = spike.High

	// Determine direction from reference candle color
	// When direction is "both", the actual direction is determined by candle color
	// GREEN (close > open) = looking for LONG breakout
	// RED (close < open) = looking for SHORT breakdown
	isBullish := spike.Close > spike.Open
	switch m.config.Direction {
	case "long":
		state.Direction = "long"
	case "short":
		state.Direction = "short"
	case "both":
		if isBullish {
			state.Direction = "long"
		} else {
			state.Direction = "short"
		}
	default:
		state.Direction = "long"
	}

	// Format direction for display
	directionLabel := "Long Setup"
	if state.Direction == "short" {
		directionLabel = "Short Setup"
	}

	// Update step details with direction
	progress.StepDetails[0] = StepDetail{
		StepNumber:  1,
		Name:        "Volume Spike",
		Completed:   true,
		CompletedAt: time.Now(),
		Progress:    fmt.Sprintf("%.1fx avg (%s)", spike.Volume/avgVol, directionLabel),
		Details:     fmt.Sprintf("Reference: H=%.6f L=%.6f", spike.High, spike.Low),
	}

	// Advance to Step 2
	progress.AdvanceStep(fmt.Sprintf("Volume spike %.2fx - %s", spike.Volume/avgVol, directionLabel))
	progress.SetStatus(PatternStatusAccumulation)
	progress.ExpiresAt = time.Now().Add(time.Duration(m.config.PatternExpirationMinutes) * time.Minute)
}

// detectVolumeSpike finds a candle with volume significantly above average.
// Returns the spike candle, its index, and the average volume, or (nil, -1, 0) if not found.
//
// OPTIMIZED FILTERS (based on backtesting 600+ trades):
// - Pre-trend DOWN: Pattern works best after a pullback
// - Reference body ratio: Strong bodies (>50%) indicate institutional activity
// - Time filter: Best during US market hours (14 UTC = 9 AM EST)
func (m *VolumeImbalancePatternMatcher) detectVolumeSpike(candles []Candle) (*Candle, int, float64) {
	if len(candles) < m.config.LookbackPeriod+5 { // +5 for pre-trend check
		return nil, -1, 0
	}

	avgVolume := m.calculateAverageVolume(candles, m.config.LookbackPeriod)
	if avgVolume == 0 {
		return nil, -1, 0
	}

	// Look for spike in recent candles INCLUDING the most recently closed one
	// IMPORTANT: When OnCandleClose is called, the last candle IS closed and should be checked
	// Previously we excluded it (< len(candles)-1) which caused volume spikes to be missed
	lookbackStart := len(candles) - m.config.LookbackPeriod
	if lookbackStart < 5 { // Ensure room for pre-trend check
		lookbackStart = 5
	}

	var bestCandle *Candle
	var bestScore float64
	bestIndex := -1

	// Debug counters
	var skippedPreTrend, skippedVolume, skippedDirection, skippedBodyRatio, candidatesChecked int
	var maxVolumeRatio float64

	// Include the last candle (len(candles)) - this is the just-closed candle that may have the volume spike
	for i := lookbackStart; i < len(candles); i++ {
		c := &candles[i]
		candidatesChecked++

		// Track max volume ratio for debugging
		if avgVolume > 0 {
			volRatio := c.Volume / avgVolume
			if volRatio > maxVolumeRatio {
				maxVolumeRatio = volRatio
			}
		}

		// ============================================================
		// OPTIMIZED FILTER 1: Pre-trend Direction
		// Pattern works best after a pullback (DOWN trend)
		// ============================================================
		if m.config.RequirePreTrendDown && i >= 5 {
			preTrendStart := candles[i-5].Open
			preTrendEnd := candles[i-1].Close
			if preTrendEnd >= preTrendStart {
				skippedPreTrend++
				continue // Skip - pre-trend is UP, not DOWN
			}
		}

		// ============================================================
		// OPTIMIZED FILTER 2: Time of Day (optional)
		// Best results at 14 UTC (US market open)
		// ============================================================
		if m.config.PreferredHourUTC >= 0 && m.config.PreferredHourUTC < 24 {
			candleHour := c.Time.Hour()
			// Allow ±1 hour window around preferred hour
			hourDiff := candleHour - m.config.PreferredHourUTC
			if hourDiff < 0 {
				hourDiff = -hourDiff
			}
			if hourDiff > 1 {
				continue // Skip - outside preferred trading window
			}
		}

		// Check volume threshold
		if c.Volume < avgVolume*m.config.MinVolumeSpikeMultiplier {
			skippedVolume++
			continue
		}

		// ============================================================
		// DIRECTION FILTER: Candle color must match configured direction
		// - "long": Only GREEN (bullish) candles (close > open)
		// - "short": Only RED (bearish) candles (close < open)
		// - "both": Accept any candle
		// ============================================================
		isBullish := c.Close > c.Open
		isBearish := c.Close < c.Open

		switch m.config.Direction {
		case "long":
			if !isBullish {
				skippedDirection++
				continue // Skip - need GREEN candle for long direction
			}
		case "short":
			if !isBearish {
				skippedDirection++
				continue // Skip - need RED candle for short direction
			}
		case "both":
			// Accept any candle (doji included)
		default:
			// Default to long behavior if direction not set
			if !isBullish {
				skippedDirection++
				continue
			}
		}

		// ============================================================
		// OPTIMIZED FILTER 3: Reference Candle Body Ratio
		// Strong bodies (>50%) indicate institutional conviction
		// ============================================================
		candleRange := c.High - c.Low
		if candleRange > 0 && m.config.MinReferenceBodyRatio > 0 {
			bodySize := c.Close - c.Open
			if bodySize < 0 {
				bodySize = -bodySize
			}
			bodyRatio := bodySize / candleRange
			if bodyRatio < m.config.MinReferenceBodyRatio {
				skippedBodyRatio++
				continue // Skip - body too small (likely indecision)
			}
		}

		// Score based on volume magnitude (no bullish bonus - direction is explicit)
		volumeScore := c.Volume / avgVolume

		// Bonus for taker buy volume if available (direction-aware)
		takerBonus := 1.0
		if c.Volume > 0 && c.TakerBuyVolume > 0 {
			takerBuyRatio := c.TakerBuyVolume / c.Volume
			// For long: higher taker buy = better
			// For short: lower taker buy (more selling) = better
			if m.config.Direction == "short" {
				takerBonus = 1 + (1-takerBuyRatio)*0.5
			} else {
				takerBonus = 1 + takerBuyRatio*0.5
			}
		}

		totalScore := volumeScore * takerBonus

		if totalScore > bestScore {
			bestScore = totalScore
			candleCopy := *c
			bestCandle = &candleCopy
			bestIndex = i
		}
	}

	// Log debug info if no spike found
	if bestCandle == nil && candidatesChecked > 0 {
		log.Printf("[SPIKE-DEBUG] No spike found: checked=%d, skippedVol=%d, maxRatio=%.2fx (need %.1fx), avgVol=%.0f",
			candidatesChecked, skippedVolume, maxVolumeRatio, m.config.MinVolumeSpikeMultiplier, avgVolume)
	}

	return bestCandle, bestIndex, avgVolume
}

// ============================================================================
// STEP 2: CONSOLIDATION DETECTION
// ============================================================================

// processStep2 tracks consolidation after volume spike and checks for breakout.
// NOTE: This is NOT a separate "step" - consolidation is the monitoring phase
// between Step 1 (reference candle) and Step 2 (breakout).
// We stay at CurrentStep=1 until breakout is detected, then advance to Step 2.
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

	// Check if pattern should be invalidated
	if m.isPatternInvalid(state, candles) {
		m.resetPatternUnlocked(patternKey, "Pattern invalidated during consolidation")
		progress.SetStatus(PatternStatusFailed)
		return
	}

	// Track consolidation metrics (for display purposes)
	isConsolidating, candleCount, consolidationData := m.isInConsolidation(state, candles)

	if isConsolidating && consolidationData != nil {
		// Update consolidation state (for entry level calculations)
		state.ConsolidationCandles = candleCount
		state.ConsolidationLow = consolidationData.low
		state.ConsolidationHigh = consolidationData.high
		state.ConsolidationAvgVol = consolidationData.avgVolume
		state.VolumeTrend = consolidationData.volumeTrend

		// Calculate Step 2 Progress Bar fields
		// Average volume multiplier from reference candle to last candle (before current)
		if state.AverageVolumeAtSpike > 0 {
			state.ConsolidationAvgVolumeMultiplier = consolidationData.avgVolume / state.AverageVolumeAtSpike
		}

		// Average price from reference candle to last candle
		state.ConsolidationAvgPrice = consolidationData.avgPrice

		// Current candle's volume multiplier (last candle in the series)
		if len(candles) > 0 && state.AverageVolumeAtSpike > 0 {
			currentCandle := candles[len(candles)-1]
			state.CurrentCandleVolumeMultiplier = currentCandle.Volume / state.AverageVolumeAtSpike
		}

		// Update Step 1 details to show consolidation progress
		// (We're still on Step 1, just showing what's happening)
		progress.StepDetails[0].Details = fmt.Sprintf("Watching for breakout (%d candles, vol trend: %.2f)", candleCount, consolidationData.volumeTrend)
	}

	// Check for BREAKOUT (this completes Step 2 - the final step)
	isBreakout, direction, breakoutCandle := m.isBreakoutReady(state, candles)
	if isBreakout {
		// Breakout detected! Pattern is now complete (Step 2/2)
		state.BreakoutCandle = breakoutCandle
		state.Direction = direction
		state.ReadyAt = time.Now().UTC() // Track when pattern became ready (UTC)

		// Calculate volume surge for display
		volSurge := 1.0
		if state.ConsolidationAvgVol > 0 {
			volSurge = breakoutCandle.Volume / state.ConsolidationAvgVol
		}
		state.BreakoutVolumeMultiplier = volSurge

		// Calculate entry price (reference high for long, reference low for short)
		if direction == "long" && state.ReferenceCandle != nil {
			state.EntryPrice = state.ReferenceCandle.High
		} else if direction == "short" && state.ReferenceCandle != nil {
			state.EntryPrice = state.ReferenceCandle.Low
		} else {
			state.EntryPrice = breakoutCandle.Close
		}

		// Mark Step 2 as complete (we're already on Step 2, just mark it done)
		progress.StepDetails[1] = StepDetail{
			StepNumber:  2,
			Name:        "Breakout",
			Completed:   true,
			CompletedAt: time.Now().UTC(),
			Progress:    fmt.Sprintf("%.1fx vol surge", volSurge),
			Details:     fmt.Sprintf("Direction: %s, entry: %.6f", direction, state.EntryPrice),
		}

		// Pattern is READY - do NOT call AdvanceStep (we're already at step 2)
		// Just mark the status as ready
		progress.SetStatus(PatternStatusReady)
		progress.UpdatedAt = time.Now().UTC()
	} else {
		// Still waiting for breakout - update status to show we're consolidating
		progress.SetStatus(PatternStatusConsolidating)
	}
}

// consolidationData holds intermediate consolidation metrics.
type consolidationData struct {
	low         float64
	high        float64
	avgVolume   float64
	avgPrice    float64 // Average price from reference to last candle (for Step 2 progress bar)
	volumeTrend float64
}

// isInConsolidation checks if price is consolidating after volume spike.
// Returns true if consolidation criteria are met, along with candle count and metrics.
func (m *VolumeImbalancePatternMatcher) isInConsolidation(
	state *PatternState,
	candles []Candle,
) (bool, int, *consolidationData) {
	if state.ReferenceCandle == nil {
		return false, 0, nil
	}

	// Use helper to find current index (handles circular buffer shifts)
	refIdx := getReferenceCandleIdx(state, candles)
	if refIdx < 0 || refIdx >= len(candles) {
		return false, 0, nil
	}

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
	var priceSum float64 // For average price calculation
	consolidationLow := candles[refIdx+1].Low
	consolidationHigh := candles[refIdx+1].High
	volumes := make([]float64, 0, candlesSinceRef)

	for i := refIdx + 1; i <= currentIdx; i++ {
		c := &candles[i]
		volumeSum += c.Volume
		volumes = append(volumes, c.Volume)

		// Calculate average price (using OHLC midpoint)
		priceSum += (c.Open + c.High + c.Low + c.Close) / 4.0

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
		avgPrice:    priceSum / float64(candlesSinceRef), // Average price for Step 2 progress bar
		volumeTrend: volumeTrend,
	}

	return true, candlesSinceRef, data
}

// ============================================================================
// BREAKOUT DETECTION
// ============================================================================
// NOTE: processStep3 has been removed. Breakout detection is now integrated
// into processStep2 for the simplified 2-step pattern model.
// Step 1: Reference Candle (Volume Spike) → Step 2: Breakout Entry

// isBreakoutReady checks if current candle shows breakout with volume surge.
// Returns (true, direction, candle) if breakout is detected.
//
// OPTIMIZED FILTER: Entry Breakout Confirmation
// Backtesting shows entries that break consolidation high have ~24% win rate
// vs ~18% for entries that don't break. This filter is enabled by default.
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

	// ============================================================
	// OPTIMIZED FILTER: Entry Breakout Confirmation
	// Entry must break above consolidation high (not just reference high)
	// This confirms continuation momentum, not just a retest
	// ============================================================
	if m.config.RequireEntryBreakout && state.ConsolidationHigh > 0 {
		if currentCandle.High <= state.ConsolidationHigh {
			return false, "", nil // Entry doesn't break consolidation high
		}
	}

	// ============================================================
	// DIRECTION-BASED BREAKOUT DETECTION
	// Only check for breakout in the configured direction
	// ============================================================

	// Check for LONG breakout (price breaks above reference high)
	// Only if direction is "long" or "both"
	if m.config.Direction == "long" || m.config.Direction == "both" || m.config.Direction == "" {
		if currentCandle.High >= state.ReferenceCandle.High {
			// Confirm close is also above (not just a wick)
			if currentCandle.Close >= state.ReferenceCandle.High*0.998 {
				candleCopy := *currentCandle
				return true, "long", &candleCopy
			}
		}
	}

	// Check for SHORT breakout (price breaks below consolidation low)
	// Only if direction is "short" or "both"
	if m.config.Direction == "short" || m.config.Direction == "both" {
		if currentCandle.Low <= state.ConsolidationLow {
			// Confirm close is also below (not just a wick)
			if currentCandle.Close <= state.ConsolidationLow*1.002 {
				candleCopy := *currentCandle
				return true, "short", &candleCopy
			}
		}
	}

	return false, "", nil
}

// ============================================================================
// CANDLE INDEX HELPER
// ============================================================================

// findCandleIdxByTime finds the index of a candle by its close time.
// Returns -1 if not found. This is needed because the circular buffer
// shifts indices when new candles are added, so stored indices become invalid.
func findCandleIdxByTime(candles []Candle, targetTime time.Time) int {
	for i, c := range candles {
		// Match within 1 second to handle any timestamp precision issues
		if c.Time.Sub(targetTime).Abs() < time.Second {
			return i
		}
	}
	return -1
}

// getReferenceCandleIdx finds the current index of the reference candle.
// If the stored ReferenceCandleIdx is valid (candle at that index matches),
// use it. Otherwise, look up by timestamp.
func getReferenceCandleIdx(state *PatternState, candles []Candle) int {
	if state.ReferenceCandle == nil {
		return -1
	}

	// First, check if stored index is still valid
	if state.ReferenceCandleIdx >= 0 && state.ReferenceCandleIdx < len(candles) {
		storedCandle := &candles[state.ReferenceCandleIdx]
		// Check if it's the same candle (by timestamp)
		if storedCandle.Time.Sub(state.ReferenceCandle.Time).Abs() < time.Second {
			return state.ReferenceCandleIdx
		}
	}

	// Index invalid (circular buffer shifted), find by timestamp
	return findCandleIdxByTime(candles, state.ReferenceCandle.Time)
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

	// Consolidation taking too long - use helper to find current reference index
	refIdx := getReferenceCandleIdx(state, candles)
	if refIdx >= 0 {
		candlesSinceRef := len(candles) - 1 - refIdx
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

// GetCoinMatch returns a full CoinMatch with tracking data for a symbol (thread-safe).
// This includes ReferenceCandle, candles_since_reference, seconds_since_reference,
// proximity_to_breakout, and other tracking fields.
func (m *VolumeImbalancePatternMatcher) GetCoinMatch(symbol, mode, timeframe string) *CoinMatch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patternKey := m.patternKey(symbol, mode, timeframe)
	progress := m.patterns[patternKey]
	state := m.states[patternKey]

	if progress == nil {
		return nil
	}

	return m.createCoinMatchWithCandles(progress, state, nil)
}

// GetAllCoinMatches returns all tracked patterns as CoinMatches with full tracking data.
// Use this for API responses and WebSocket broadcasts to include reference candle info.
func (m *VolumeImbalancePatternMatcher) GetAllCoinMatches() []*CoinMatch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CoinMatch, 0, len(m.patterns))
	for key, progress := range m.patterns {
		state := m.states[key]
		cm := m.createCoinMatchWithCandles(progress, state, nil)
		if cm != nil {
			result = append(result, cm)
		}
	}
	return result
}

// GetCoinMatchesForStrategy returns coin matches filtered by mode and timeframe.
// This prevents duplicate coins from appearing when the same coin is tracked
// across multiple strategies (e.g., scalp/3m and swing/1h).
func (m *VolumeImbalancePatternMatcher) GetCoinMatchesForStrategy(mode, timeframe string) []*CoinMatch {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CoinMatch, 0)
	for key, progress := range m.patterns {
		// Filter by mode and timeframe
		if progress.Mode != mode || progress.Timeframe != timeframe {
			continue
		}
		state := m.states[key]
		cm := m.createCoinMatchWithCandles(progress, state, nil)
		if cm != nil {
			result = append(result, cm)
		}
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

// ClearAllPatterns removes all tracked patterns and resets internal state.
// This should be called when the CoinProfiler restarts to ensure fresh pattern
// detection without stale expiration timestamps from previous sessions.
// Returns the count of patterns that were cleared.
func (m *VolumeImbalancePatternMatcher) ClearAllPatterns() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := len(m.patterns)
	m.patterns = make(map[string]*PatternProgress)
	m.states = make(map[string]*PatternState)

	if count > 0 {
		log.Printf("[PATTERN] Cleared %d patterns for fresh start", count)
	}

	return count
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
	return m.createCoinMatchWithCandles(progress, state, nil)
}

// createCoinMatchWithCandles converts pattern progress to a CoinMatch with full tracking details.
func (m *VolumeImbalancePatternMatcher) createCoinMatchWithCandles(
	progress *PatternProgress,
	state *PatternState,
	candles []Candle,
) *CoinMatch {
	if progress == nil {
		return nil
	}

	cm := progress.ToCoinMatch()

	// Always populate volume metrics for progress bar display (especially for watching state)
	if candles != nil && len(candles) >= m.config.LookbackPeriod {
		avgVolume := m.calculateAverageVolume(candles, m.config.LookbackPeriod)
		if avgVolume > 0 {
			currentCandle := &candles[len(candles)-1]
			currentVolume := currentCandle.Volume
			volumeMultiplier := currentVolume / avgVolume
			volumeThreshold := m.config.MinVolumeSpikeMultiplier

			// Set volume metrics on CoinMatch
			cm.AvgVolume = avgVolume
			cm.CurrentVolume = currentVolume
			cm.VolumeThreshold = volumeThreshold
			cm.VolumeMultiplier = volumeMultiplier

			// Calculate distance to threshold as percentage
			// Negative = below threshold, Positive = above threshold
			if volumeThreshold > 0 {
				cm.VolumeDistancePercent = ((volumeMultiplier / volumeThreshold) - 1) * 100
			}

			// Set current price from latest candle
			cm.CurrentPrice = currentCandle.Close
		}
	}

	// Price context metrics for price progress bar
	// Always copy day high/low from state (even when candles are nil for API responses)
	if state != nil {
		cm.DayHigh = state.DayHigh
		cm.DayLow = state.DayLow
		cm.AvgPrice5 = state.AvgPrice5
	}

	// Update day high/low and avg price from current candle (real-time tracking)
	if candles != nil && len(candles) > 0 {
		currentCandle := &candles[len(candles)-1]

		if state != nil {
			// Update state's day high/low from current candle
			if currentCandle.High > state.DayHigh && state.DayHigh > 0 {
				state.DayHigh = currentCandle.High
				cm.DayHigh = state.DayHigh
			}
			if (currentCandle.Low < state.DayLow || state.DayLow == 0) && currentCandle.Low > 0 {
				state.DayLow = currentCandle.Low
				cm.DayLow = state.DayLow
			}
		}

		// Calculate 5-candle average price (same lookback as volume)
		if len(candles) >= m.config.LookbackPeriod {
			avgPrice := m.calculateAveragePrice(candles, m.config.LookbackPeriod)
			cm.AvgPrice5 = avgPrice
			if state != nil {
				state.AvgPrice5 = avgPrice // Store in state for API responses
			}
		}
	}

	// Add direction and price from state if available
	if state != nil {
		cm.Direction = state.Direction
		if state.BreakoutCandle != nil {
			cm.CurrentPrice = state.BreakoutCandle.Close
		} else if state.ReferenceCandle != nil {
			cm.CurrentPrice = state.ReferenceCandle.Close
		}

		// Add reference candle tracking data
		if state.ReferenceCandle != nil {
			refDetectedAt := state.ReferenceDetectedAt
			cm.ReferenceDetectedAt = &refDetectedAt

			// Convert internal Candle to ReferenceCandle struct
			// Use OpenTime to identify the candle (aligned to candle boundaries like :00, :03, :06)
			cm.ReferenceCandle = &ReferenceCandle{
				OpenTime:         state.ReferenceCandle.OpenTime, // Candle open time (proper candle identifier)
				CloseTime:        state.ReferenceCandle.Time,     // Candle close time
				Open:             state.ReferenceCandle.Open,
				High:             state.ReferenceCandle.High,
				Low:              state.ReferenceCandle.Low,
				Close:            state.ReferenceCandle.Close,
				Volume:           state.ReferenceCandle.Volume,
				VolumeMultiplier: 0, // Will be calculated below
			}

			// Calculate volume multiplier
			if state.AverageVolumeAtSpike > 0 {
				cm.ReferenceCandle.VolumeMultiplier = state.ReferenceCandle.Volume / state.AverageVolumeAtSpike
			}

			// Calculate seconds since reference detection
			if !state.ReferenceDetectedAt.IsZero() {
				cm.SecondsSinceReference = int(time.Since(state.ReferenceDetectedAt).Seconds())
			}

			// Calculate candles since reference (use helper to handle circular buffer shifts)
			if candles != nil {
				refIdx := getReferenceCandleIdx(state, candles)
				if refIdx >= 0 {
					cm.CandlesSinceReference = len(candles) - 1 - refIdx
				}
			}

			// Calculate proximity to breakout
			if cm.CurrentPrice > 0 && state.ReferenceCandle.High > 0 {
				cm.ProximityToBreakout = (cm.CurrentPrice - state.ReferenceCandle.High) / state.ReferenceCandle.High * 100
			}

			// Check for potential breakout
			if candles != nil && len(candles) > 0 {
				currentCandle := &candles[len(candles)-1]
				cm.CurrentPrice = currentCandle.Close

				// Recalculate proximity with latest price
				if state.ReferenceCandle.High > 0 {
					cm.ProximityToBreakout = (currentCandle.Close - state.ReferenceCandle.High) / state.ReferenceCandle.High * 100
				}

				// Potential breakout: price approaching or exceeding reference high with volume
				if currentCandle.High >= state.ReferenceCandle.High*0.99 {
					// Check for volume surge (at least 1.3x consolidation avg or 1.5x reference volume)
					isVolumeSurge := false
					if state.ConsolidationAvgVol > 0 {
						isVolumeSurge = currentCandle.Volume >= state.ConsolidationAvgVol*1.3
					} else if state.AverageVolumeAtSpike > 0 {
						isVolumeSurge = currentCandle.Volume >= state.AverageVolumeAtSpike*1.0
					}
					cm.PotentialBreakout = isVolumeSurge
				}
			}
		}

		// Add entry candle data when pattern is ready
		if state.BreakoutCandle != nil && progress.Status == PatternStatusReady {
			readyAt := state.ReadyAt
			cm.ReadyAt = &readyAt

			// Create EntryCandle from breakout data
			// Use OpenTime to identify the candle (aligned to candle boundaries)
			cm.EntryCandle = &EntryCandle{
				OpenTime:         state.BreakoutCandle.OpenTime, // Candle open time (proper identifier)
				CloseTime:        state.BreakoutCandle.Time,     // Candle close time
				Open:             state.BreakoutCandle.Open,
				High:             state.BreakoutCandle.High,
				Low:              state.BreakoutCandle.Low,
				Close:            state.BreakoutCandle.Close,
				Volume:           state.BreakoutCandle.Volume,
				VolumeMultiplier: state.BreakoutVolumeMultiplier,
				EntryPrice:       state.EntryPrice,
				DetectedAt:       state.ReadyAt,
				Direction:        state.Direction,
			}

			// Calculate seconds until expiry for ready patterns
			if !state.ReadyAt.IsZero() && m.config.ReadyExpirationSeconds > 0 {
				elapsed := time.Since(state.ReadyAt).Seconds()
				remaining := float64(m.config.ReadyExpirationSeconds) - elapsed
				if remaining > 0 {
					cm.SecondsUntilExpiry = int(remaining)
				}
			}
		}

		// Populate entry levels from pattern state for actual SL/TP prices
		// These are calculated from Support Price (Consolidation Low) at pattern detection time
		if state.ConsolidationLow > 0 {
			cm.SupportPrice = state.ConsolidationLow
		}
		if state.ConsolidationHigh > 0 {
			cm.ResistancePrice = state.ConsolidationHigh
		}
		if state.EntryPrice > 0 {
			cm.EntryLevelPrice = state.EntryPrice

			// Default R:R ratio of 4.0 (1:4) - matches backtested defaults
			// This will be overridden by sub-strategy settings if available
			const defaultRRRatio = 4.0

			// Calculate SL, TP, and Risk based on direction
			if state.Direction == "long" && state.ConsolidationLow > 0 {
				// For LONG: SL = Support Price (with small buffer), TP = Entry + Risk × R:R
				cm.StopLossPrice = state.ConsolidationLow * 0.999 // 0.1% buffer below support
				cm.RiskAmount = state.EntryPrice - cm.StopLossPrice
				if cm.RiskAmount > 0 {
					cm.RiskPercent = (cm.RiskAmount / state.EntryPrice) * 100
					// TP at R:R ratio
					cm.TakeProfitPrice = state.EntryPrice + (cm.RiskAmount * defaultRRRatio)
				}
			} else if state.Direction == "short" && state.ConsolidationHigh > 0 {
				// For SHORT: SL = Resistance Price (with small buffer), TP = Entry - Risk × R:R
				cm.StopLossPrice = state.ConsolidationHigh * 1.001 // 0.1% buffer above resistance
				cm.RiskAmount = cm.StopLossPrice - state.EntryPrice
				if cm.RiskAmount > 0 {
					cm.RiskPercent = (cm.RiskAmount / state.EntryPrice) * 100
					// TP at R:R ratio
					cm.TakeProfitPrice = state.EntryPrice - (cm.RiskAmount * defaultRRRatio)
				}
			}
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

// calculateAveragePrice calculates the average close price over the given period.
// Uses the same lookback period as volume for consistency.
func (m *VolumeImbalancePatternMatcher) calculateAveragePrice(candles []Candle, period int) float64 {
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
		sum += candles[i].Close
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
