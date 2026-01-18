// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.4: Market Regime Classifier
package decision

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RegimeConfig holds the configurable thresholds for regime classification.
type RegimeConfig struct {
	// ADXTrendingThreshold is the minimum ADX value for TRENDING regime.
	// Default: 25
	ADXTrendingThreshold float64

	// ADXRangingThreshold is the maximum ADX value for RANGING/VOLATILE/CONSOLIDATING.
	// Default: 20
	ADXRangingThreshold float64

	// ATRVolatileMultiplier is the multiplier vs ATR average for VOLATILE regime.
	// If ATR > ATRAverage * ATRVolatileMultiplier, regime is VOLATILE.
	// Default: 1.5
	ATRVolatileMultiplier float64

	// ATRConsolidatingDivisor is the divisor vs ATR average for CONSOLIDATING regime.
	// If ATR < ATRAverage * ATRConsolidatingDivisor, regime is CONSOLIDATING.
	// Default: 0.5
	ATRConsolidatingDivisor float64

	// MinRegimeDuration is the minimum number of candles before allowing regime change.
	// This prevents rapid switching between regimes.
	// Default: 3
	MinRegimeDuration int

	// HistoryMaxSize is the maximum number of regime changes to track per symbol.
	// Default: 100
	HistoryMaxSize int
}

// DefaultRegimeConfig returns a RegimeConfig with sensible defaults.
func DefaultRegimeConfig() *RegimeConfig {
	return &RegimeConfig{
		ADXTrendingThreshold:    25.0,
		ADXRangingThreshold:     20.0,
		ATRVolatileMultiplier:   1.5,
		ATRConsolidatingDivisor: 0.5,
		MinRegimeDuration:       3,
		HistoryMaxSize:          100,
	}
}

// RegimeIndicators holds the indicator values at the time of classification.
type RegimeIndicators struct {
	ADX        float64 `json:"adx"`
	ATR        float64 `json:"atr"`
	ATRAverage float64 `json:"atr_average"`
}

// RegimeChange records a single regime transition.
type RegimeChange struct {
	From       MarketRegime     `json:"from"`
	To         MarketRegime     `json:"to"`
	Timestamp  int64            `json:"timestamp"`
	Reason     string           `json:"reason"`
	Indicators RegimeIndicators `json:"indicators"`
}

// SymbolRegimeState tracks the current regime state for a symbol.
type SymbolRegimeState struct {
	CurrentRegime  MarketRegime
	CandlesInState int // How many candles/updates in current regime
}

// RegimeClassifier performs market regime classification for coins.
// It maintains historical regime changes and supports configurable thresholds.
type RegimeClassifier struct {
	config *RegimeConfig

	// history tracks regime changes per symbol
	history   map[string][]RegimeChange
	historyMu sync.RWMutex

	// currentState tracks current regime and duration per symbol
	currentState   map[string]*SymbolRegimeState
	currentStateMu sync.RWMutex

	// StateManager for Redis updates (optional, can be nil for classification-only mode)
	sm *StateManager
}

// NewRegimeClassifier creates a new RegimeClassifier with the given configuration.
// If config is nil, default configuration is used.
// StateManager is optional - if nil, classifier operates in classification-only mode.
func NewRegimeClassifier(config *RegimeConfig, sm *StateManager) *RegimeClassifier {
	if config == nil {
		config = DefaultRegimeConfig()
	}
	return &RegimeClassifier{
		config:       config,
		history:      make(map[string][]RegimeChange),
		currentState: make(map[string]*SymbolRegimeState),
		sm:           sm,
	}
}

// GetConfig returns the current configuration (read-only).
func (rc *RegimeClassifier) GetConfig() *RegimeConfig {
	return rc.config
}

// Classify determines the market regime based on the coin state.
// This is a pure classification function - it does not update state or history.
func (rc *RegimeClassifier) Classify(state *CoinState) MarketRegime {
	if state == nil {
		return RegimeConsolidating
	}

	return rc.ClassifyWithIndicators(state.ADX, state.ATR, state.ATRAverage)
}

// ClassifyWithIndicators determines the market regime from raw indicator values.
// Parameters:
//   - adx: Average Directional Index value
//   - atr: Average True Range value
//   - atrAverage: Moving average of ATR for comparison
func (rc *RegimeClassifier) ClassifyWithIndicators(adx, atr, atrAverage float64) MarketRegime {
	// High ADX indicates trending market
	if adx >= rc.config.ADXTrendingThreshold {
		return RegimeTrending
	}

	// Low ADX could be ranging, volatile, or consolidating
	if adx < rc.config.ADXRangingThreshold {
		// If we have ATR data, differentiate between volatile and consolidating
		if atr > 0 && atrAverage > 0 {
			if atr > atrAverage*rc.config.ATRVolatileMultiplier {
				return RegimeVolatile
			}
			if atr < atrAverage*rc.config.ATRConsolidatingDivisor {
				return RegimeConsolidating
			}
		}
		// Default to ranging for low ADX without clear ATR signals
		return RegimeRanging
	}

	// Mid-range ADX (between ranging and trending thresholds)
	// This is an indeterminate zone - default to ranging
	return RegimeRanging
}

// ClassifyAndRecord classifies the regime and records any change in history.
// Returns the new regime and whether a change occurred.
func (rc *RegimeClassifier) ClassifyAndRecord(symbol string, adx, atr, atrAverage float64) (MarketRegime, bool) {
	newRegime := rc.ClassifyWithIndicators(adx, atr, atrAverage)

	rc.currentStateMu.Lock()
	defer rc.currentStateMu.Unlock()

	// Get or create current state
	current, exists := rc.currentState[symbol]
	if !exists {
		// First classification for this symbol
		rc.currentState[symbol] = &SymbolRegimeState{
			CurrentRegime:  newRegime,
			CandlesInState: 1,
		}
		// Record initial regime as a change from nothing
		rc.recordChangeInternal(symbol, "", newRegime, "initial_classification",
			RegimeIndicators{ADX: adx, ATR: atr, ATRAverage: atrAverage})
		return newRegime, true
	}

	// Same regime - increment counter
	if newRegime == current.CurrentRegime {
		current.CandlesInState++
		return newRegime, false
	}

	// Regime changed - check minimum duration
	if current.CandlesInState < rc.config.MinRegimeDuration {
		// Not enough candles in current regime to allow change
		current.CandlesInState++
		return current.CurrentRegime, false
	}

	// Regime change is valid - record it
	oldRegime := current.CurrentRegime
	reason := rc.getChangeReason(oldRegime, newRegime, adx, atr, atrAverage)

	rc.recordChangeInternal(symbol, oldRegime, newRegime, reason,
		RegimeIndicators{ADX: adx, ATR: atr, ATRAverage: atrAverage})

	// Update current state
	current.CurrentRegime = newRegime
	current.CandlesInState = 1

	return newRegime, true
}

// recordChangeInternal records a regime change in history (must hold lock).
func (rc *RegimeClassifier) recordChangeInternal(symbol string, from, to MarketRegime, reason string, indicators RegimeIndicators) {
	rc.historyMu.Lock()
	defer rc.historyMu.Unlock()

	change := RegimeChange{
		From:       from,
		To:         to,
		Timestamp:  time.Now().UnixMilli(),
		Reason:     reason,
		Indicators: indicators,
	}

	history := rc.history[symbol]
	history = append(history, change)

	// Trim history if needed
	if len(history) > rc.config.HistoryMaxSize {
		history = history[len(history)-rc.config.HistoryMaxSize:]
	}

	rc.history[symbol] = history
}

// getChangeReason generates a human-readable reason for the regime change.
func (rc *RegimeClassifier) getChangeReason(from, to MarketRegime, adx, atr, atrAverage float64) string {
	switch to {
	case RegimeTrending:
		return fmt.Sprintf("ADX rose to %.2f (>= %.2f threshold)", adx, rc.config.ADXTrendingThreshold)
	case RegimeRanging:
		return fmt.Sprintf("ADX at %.2f, ATR normal", adx)
	case RegimeVolatile:
		return fmt.Sprintf("ATR %.4f > %.4f (%.1fx average)", atr, atrAverage*rc.config.ATRVolatileMultiplier, rc.config.ATRVolatileMultiplier)
	case RegimeConsolidating:
		return fmt.Sprintf("ATR %.4f < %.4f (%.1fx average)", atr, atrAverage*rc.config.ATRConsolidatingDivisor, rc.config.ATRConsolidatingDivisor)
	default:
		return fmt.Sprintf("Regime changed from %s to %s", from, to)
	}
}

// UpdateRegime updates the coin state with the new regime via StateManager.
// This method:
// 1. Classifies the regime based on indicators (ADX, ATR, ATRAverage)
// 2. Records the change in history
// 3. Updates Redis state via StateManager (if available)
// 4. Returns the new regime and recommended strategy
func (rc *RegimeClassifier) UpdateRegime(ctx context.Context, userID, symbol string, state *CoinState) (MarketRegime, string, error) {
	if state == nil {
		return RegimeConsolidating, "", fmt.Errorf("state cannot be nil")
	}

	// Classify using full indicator set including ATR for volatility detection
	newRegime, changed := rc.ClassifyAndRecord(symbol, state.ADX, state.ATR, state.ATRAverage)

	// Get the recommended strategy for this regime
	strategy := GetStrategyForRegime(newRegime)

	// Update state
	state.Regime = newRegime
	state.ActiveStrategy = strategy

	// Update Redis if StateManager is available and regime changed
	if rc.sm != nil && changed {
		err := rc.sm.UpdateRegime(ctx, userID, symbol, newRegime, strategy)
		if err != nil {
			return newRegime, strategy, fmt.Errorf("failed to update regime in Redis: %w", err)
		}
	}

	return newRegime, strategy, nil
}

// GetHistory returns the regime change history for a symbol.
// Returns a copy to prevent external modification.
func (rc *RegimeClassifier) GetHistory(symbol string) []RegimeChange {
	rc.historyMu.RLock()
	defer rc.historyMu.RUnlock()

	history := rc.history[symbol]
	if history == nil {
		return []RegimeChange{}
	}

	// Return a copy
	result := make([]RegimeChange, len(history))
	copy(result, history)
	return result
}

// GetCurrentStreak returns the current regime and how many candles/updates it has been active.
func (rc *RegimeClassifier) GetCurrentStreak(symbol string) (MarketRegime, int) {
	rc.currentStateMu.RLock()
	defer rc.currentStateMu.RUnlock()

	state, exists := rc.currentState[symbol]
	if !exists {
		return RegimeConsolidating, 0
	}

	return state.CurrentRegime, state.CandlesInState
}

// GetLatestChange returns the most recent regime change for a symbol, if any.
func (rc *RegimeClassifier) GetLatestChange(symbol string) *RegimeChange {
	rc.historyMu.RLock()
	defer rc.historyMu.RUnlock()

	history := rc.history[symbol]
	if len(history) == 0 {
		return nil
	}

	// Return a copy of the latest change
	latest := history[len(history)-1]
	return &latest
}

// ClearHistory removes all history for a symbol.
// This is typically used for testing or when resetting state.
// NOTE: Acquires currentStateMu first, then historyMu to prevent deadlock
// (same order as ClassifyAndRecord -> recordChangeInternal).
func (rc *RegimeClassifier) ClearHistory(symbol string) {
	rc.currentStateMu.Lock()
	defer rc.currentStateMu.Unlock()

	delete(rc.currentState, symbol)

	rc.historyMu.Lock()
	defer rc.historyMu.Unlock()

	delete(rc.history, symbol)
}

// ClearAllHistory removes all history for all symbols.
// NOTE: Acquires currentStateMu first, then historyMu to prevent deadlock
// (same order as ClassifyAndRecord -> recordChangeInternal).
func (rc *RegimeClassifier) ClearAllHistory() {
	rc.currentStateMu.Lock()
	defer rc.currentStateMu.Unlock()

	rc.currentState = make(map[string]*SymbolRegimeState)

	rc.historyMu.Lock()
	defer rc.historyMu.Unlock()

	rc.history = make(map[string][]RegimeChange)
}

// GetStrategyForRegime returns the recommended trading strategy for a regime.
func GetStrategyForRegime(regime MarketRegime) string {
	switch regime {
	case RegimeTrending:
		return "trend_following"
	case RegimeRanging:
		return "mean_reversion"
	case RegimeVolatile:
		return "breakout"
	case RegimeConsolidating:
		return "range_trading"
	default:
		return "mean_reversion"
	}
}

// GetAllKnownSymbols returns all symbols that have regime history.
func (rc *RegimeClassifier) GetAllKnownSymbols() []string {
	rc.historyMu.RLock()
	defer rc.historyMu.RUnlock()

	symbols := make([]string, 0, len(rc.history))
	for symbol := range rc.history {
		symbols = append(symbols, symbol)
	}
	return symbols
}
