// Package exitdecision provides exit signal monitoring for the Chain Trading System.
// This file implements the critical safeguards (Story 10.1 Phase 2).
package exitdecision

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================================================
// SAFEGUARDS IMPLEMENTATION (Story 10.1 Phase 2)
// ============================================================================
// These safeguards prevent common failure scenarios in exit decisions:
// S1: Minimum hold time before efficiency exit
// S2: Consecutive signal requirement (whipsaw prevention)
// S3: Breakeven verification before efficiency exit
// S4: Stale data detection
// S5: Epic 11 integration safeguards (fallback to Classic mode)
// S6: Decision mode consistency

// Safeguards implements all exit decision safeguards.
type Safeguards struct {
	config *SafeguardsConfig

	// S2: Consecutive signal tracking per symbol
	signalTrackers map[string]*ConsecutiveSignalTracker
	trackerMu      sync.RWMutex

	// Logging
	debugMode bool
}

// NewSafeguards creates a new Safeguards instance with the given configuration.
func NewSafeguards(config *SafeguardsConfig, debugMode bool) *Safeguards {
	if config == nil {
		config = DefaultSafeguardsConfig()
	}

	return &Safeguards{
		config:         config,
		signalTrackers: make(map[string]*ConsecutiveSignalTracker),
		debugMode:      debugMode,
	}
}

// ============================================================================
// S1: MINIMUM HOLD TIME BEFORE EFFICIENCY EXIT
// ============================================================================

// CheckMinHoldTime verifies that the position has been held long enough
// before allowing an efficiency-based exit.
// Returns (blocked, reason) - if blocked is true, efficiency exit should not proceed.
func (s *Safeguards) CheckMinHoldTime(pos Position) (bool, string) {
	if !s.config.EnableMinHoldTime {
		return false, ""
	}

	mode := pos.GetMode()
	entryTime := pos.GetEntryTime()

	// Get minimum hold time for this mode
	minHoldMins, exists := s.config.MinHoldBeforeEfficiencyMins[mode]
	if !exists {
		// Default to 2 minutes if mode not configured
		minHoldMins = 2
	}

	minHoldDuration := time.Duration(minHoldMins) * time.Minute
	holdDuration := time.Since(entryTime)

	if holdDuration < minHoldDuration {
		reason := fmt.Sprintf("S1: Position held for %s, minimum hold time is %s",
			formatDuration(holdDuration), formatDuration(minHoldDuration))
		s.logDebug(reason)
		return true, reason
	}

	return false, ""
}

// ============================================================================
// S2: CONSECUTIVE SIGNAL REQUIREMENT (WHIPSAW PREVENTION)
// ============================================================================

// CheckConsecutiveSignals tracks consecutive efficiency signals and returns
// whether enough consecutive signals have been received to allow exit.
// Returns (blocked, reason) - if blocked is true, efficiency exit should not proceed.
func (s *Safeguards) CheckConsecutiveSignals(symbol string, isBelowThreshold bool) (bool, string) {
	if !s.config.EnableWhipsawPrevention {
		return false, ""
	}

	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()

	tracker, exists := s.signalTrackers[symbol]
	if !exists {
		tracker = &ConsecutiveSignalTracker{
			Symbol:        symbol,
			LastResetTime: time.Now(),
		}
		s.signalTrackers[symbol] = tracker
	}

	now := time.Now()
	windowDuration := time.Duration(s.config.ConsecutiveSignalWindowSecs) * time.Second

	// Check if we're outside the signal window - reset if so
	if !tracker.LastBelowThresholdTime.IsZero() {
		timeSinceLastSignal := now.Sub(tracker.LastBelowThresholdTime)
		if timeSinceLastSignal > windowDuration {
			// Window expired, reset counter
			tracker.ConsecutiveBelowThreshold = 0
			tracker.LastResetTime = now
			s.logDebug("S2: Reset consecutive signal counter for %s (window expired)", symbol)
		}
	}

	if isBelowThreshold {
		tracker.ConsecutiveBelowThreshold++
		tracker.LastBelowThresholdTime = now
		s.logDebug("S2: %s consecutive signal #%d", symbol, tracker.ConsecutiveBelowThreshold)
	} else {
		// Efficiency recovered, reset counter
		if tracker.ConsecutiveBelowThreshold > 0 {
			s.logDebug("S2: %s efficiency recovered, resetting counter", symbol)
		}
		tracker.ConsecutiveBelowThreshold = 0
		tracker.LastBelowThresholdTime = time.Time{}
		return true, "S2: Efficiency recovered above threshold"
	}

	// Check if we have enough consecutive signals
	if tracker.ConsecutiveBelowThreshold < s.config.ConsecutiveSignalsRequired {
		reason := fmt.Sprintf("S2: Waiting for %d consecutive signals, have %d",
			s.config.ConsecutiveSignalsRequired, tracker.ConsecutiveBelowThreshold)
		return true, reason
	}

	// Enough consecutive signals received
	s.logDebug("S2: %s reached %d consecutive signals, allowing exit",
		symbol, tracker.ConsecutiveBelowThreshold)
	return false, ""
}

// ResetConsecutiveSignals resets the consecutive signal tracker for a symbol.
// Call this when a position is closed.
func (s *Safeguards) ResetConsecutiveSignals(symbol string) {
	s.trackerMu.Lock()
	defer s.trackerMu.Unlock()
	delete(s.signalTrackers, symbol)
}

// GetConsecutiveSignalCount returns the current consecutive signal count for a symbol.
func (s *Safeguards) GetConsecutiveSignalCount(symbol string) int {
	s.trackerMu.RLock()
	defer s.trackerMu.RUnlock()

	if tracker, exists := s.signalTrackers[symbol]; exists {
		return tracker.ConsecutiveBelowThreshold
	}
	return 0
}

// ============================================================================
// S3: BREAKEVEN VERIFICATION BEFORE EFFICIENCY EXIT
// ============================================================================

// CheckBreakevenVerification verifies that the position is above breakeven
// before allowing an efficiency-based exit.
// Returns (blocked, reason) - if blocked is true, efficiency exit should not proceed.
func (s *Safeguards) CheckBreakevenVerification(pos Position, currentPrice float64) (bool, string) {
	if !s.config.EnableBreakevenVerification {
		return false, ""
	}

	side := pos.GetSide()
	bePrice := pos.GetBreakevenPrice()

	// If no breakeven price set, use entry price
	if bePrice <= 0 {
		bePrice = pos.GetEntryPrice()
	}

	// Check if below breakeven
	if side == "LONG" || side == "long" {
		if currentPrice <= bePrice {
			reason := fmt.Sprintf("S3: Price %.4f is at or below breakeven %.4f for LONG position",
				currentPrice, bePrice)
			s.logDebug(reason)
			return true, reason
		}
	} else {
		if currentPrice >= bePrice {
			reason := fmt.Sprintf("S3: Price %.4f is at or above breakeven %.4f for SHORT position",
				currentPrice, bePrice)
			s.logDebug(reason)
			return true, reason
		}
	}

	// Also verify profit is positive
	currentProfit := pos.GetCurrentProfit()
	if currentProfit <= 0 {
		reason := fmt.Sprintf("S3: Current profit %.2f%% is not positive", currentProfit)
		s.logDebug(reason)
		return true, reason
	}

	return false, ""
}

// ============================================================================
// S4: STALE DATA DETECTION
// ============================================================================

// CheckPriceDataFreshness verifies that price data is recent enough for decisions.
// Returns (blocked, reason) - if blocked is true, exit decision should not proceed.
func (s *Safeguards) CheckPriceDataFreshness(pos Position) (bool, string) {
	if !s.config.EnableStaleDataDetection {
		return false, ""
	}

	lastUpdate := pos.GetLastPriceUpdate()
	if lastUpdate.IsZero() {
		// If no update time available, assume fresh
		return false, ""
	}

	maxAge := time.Duration(s.config.MaxPriceDataAgeSecs) * time.Second
	priceAge := time.Since(lastUpdate)

	if priceAge > maxAge {
		reason := fmt.Sprintf("S4: Price data is stale (age: %s, max: %s)",
			formatDuration(priceAge), formatDuration(maxAge))
		s.logDebug(reason)
		return true, reason
	}

	return false, ""
}

// CheckTrendDataFreshness verifies that trend data is recent enough for decisions.
// trendTimestamp is the timestamp of the last trend analysis.
// Returns (blocked, reason) - if blocked is true, trend-based exit should not proceed.
func (s *Safeguards) CheckTrendDataFreshness(trendTimestamp time.Time) (bool, string) {
	if !s.config.EnableStaleDataDetection {
		return false, ""
	}

	if trendTimestamp.IsZero() {
		// If no trend data available, block trend-based decisions
		return true, "S4: No trend data available"
	}

	maxAge := time.Duration(s.config.MaxTrendDataAgeSecs) * time.Second
	trendAge := time.Since(trendTimestamp)

	if trendAge > maxAge {
		reason := fmt.Sprintf("S4: Trend data is stale (age: %s, max: %s)",
			formatDuration(trendAge), formatDuration(maxAge))
		s.logDebug(reason)
		return true, reason
	}

	return false, ""
}

// ============================================================================
// S5: EPIC 11 INTEGRATION SAFEGUARDS (NEW ENGINE MODE)
// ============================================================================

// NewEngineCheckResult holds the result of New Engine availability check.
type NewEngineCheckResult struct {
	Available    bool   `json:"available"`
	UseFallback  bool   `json:"use_fallback"`
	Reason       string `json:"reason"`
	IndicatorCount int  `json:"indicator_count"`
}

// CheckNewEngineAvailability verifies that New Engine components are available.
// Returns whether to use fallback mode and the reason.
func (s *Safeguards) CheckNewEngineAvailability(
	decisionEngineAvailable bool,
	strategyAvailable bool,
	indicatorCount int,
) *NewEngineCheckResult {

	if !s.config.EnableNewEngineFallback {
		return &NewEngineCheckResult{
			Available:    decisionEngineAvailable && strategyAvailable,
			UseFallback:  false,
			Reason:       "Fallback disabled",
			IndicatorCount: indicatorCount,
		}
	}

	// Check Decision Engine availability
	if !decisionEngineAvailable {
		reason := "S5: DecisionEngine unavailable, falling back to Classic mode"
		s.logDebug(reason)
		return &NewEngineCheckResult{
			Available:    false,
			UseFallback:  s.config.FallbackToClassicOnError,
			Reason:       reason,
			IndicatorCount: 0,
		}
	}

	// Check strategy availability
	if !strategyAvailable {
		reason := "S5: No active strategy, falling back to Classic mode"
		s.logDebug(reason)
		return &NewEngineCheckResult{
			Available:    false,
			UseFallback:  s.config.FallbackToClassicOnError,
			Reason:       reason,
			IndicatorCount: 0,
		}
	}

	// Check minimum indicators
	if indicatorCount < s.config.MinIndicatorsRequired {
		reason := fmt.Sprintf("S5: Only %d indicators (need %d), falling back to Classic mode",
			indicatorCount, s.config.MinIndicatorsRequired)
		s.logDebug(reason)
		return &NewEngineCheckResult{
			Available:    false,
			UseFallback:  s.config.FallbackToClassicOnError,
			Reason:       reason,
			IndicatorCount: indicatorCount,
		}
	}

	return &NewEngineCheckResult{
		Available:    true,
		UseFallback:  false,
		Reason:       "",
		IndicatorCount: indicatorCount,
	}
}

// RegimeChangeTracker tracks regime changes for confirmation.
type RegimeChangeTracker struct {
	Symbol               string
	OriginalRegime       string
	DetectedRegime       string
	DetectedAt           time.Time
	ConfirmationRequired time.Duration
}

// regimeTrackers holds regime change tracking state per symbol.
var regimeTrackers = make(map[string]*RegimeChangeTracker)
var regimeTrackerMu sync.RWMutex

// CheckRegimeChangeConfirmation verifies that a regime change has been stable
// long enough before acting on it.
// Returns (confirmed, tracker) - if confirmed is true, the regime change is valid.
func (s *Safeguards) CheckRegimeChangeConfirmation(
	symbol string,
	entryRegime string,
	currentRegime string,
) (bool, *RegimeChangeTracker) {

	if !s.config.EnableNewEngineFallback {
		// If safeguard disabled, always confirm
		return entryRegime != currentRegime, nil
	}

	regimeTrackerMu.Lock()
	defer regimeTrackerMu.Unlock()

	tracker, exists := regimeTrackers[symbol]
	confirmationDuration := time.Duration(s.config.RegimeChangeConfirmationSecs) * time.Second

	// No regime change
	if entryRegime == currentRegime {
		// Reset tracker if exists
		if exists {
			delete(regimeTrackers, symbol)
		}
		return false, nil
	}

	// Regime has changed
	if !exists || tracker.DetectedRegime != currentRegime {
		// New regime change detected, start tracking
		tracker = &RegimeChangeTracker{
			Symbol:               symbol,
			OriginalRegime:       entryRegime,
			DetectedRegime:       currentRegime,
			DetectedAt:           time.Now(),
			ConfirmationRequired: confirmationDuration,
		}
		regimeTrackers[symbol] = tracker
		s.logDebug("S5: Regime change detected for %s: %s -> %s, waiting for confirmation",
			symbol, entryRegime, currentRegime)
		return false, tracker
	}

	// Check if confirmation period has passed
	elapsed := time.Since(tracker.DetectedAt)
	if elapsed >= confirmationDuration {
		s.logDebug("S5: Regime change confirmed for %s after %s",
			symbol, formatDuration(elapsed))
		delete(regimeTrackers, symbol)
		return true, tracker
	}

	remaining := confirmationDuration - elapsed
	s.logDebug("S5: Waiting for regime confirmation for %s, %s remaining",
		symbol, formatDuration(remaining))
	return false, tracker
}

// ResetRegimeTracker resets the regime change tracker for a symbol.
func (s *Safeguards) ResetRegimeTracker(symbol string) {
	regimeTrackerMu.Lock()
	defer regimeTrackerMu.Unlock()
	delete(regimeTrackers, symbol)
}

// ============================================================================
// S6: DECISION MODE CONSISTENCY
// ============================================================================

// ModeChangeValidation holds the result of a mode change validation.
type ModeChangeValidation struct {
	Allowed           bool     `json:"allowed"`
	Reason            string   `json:"reason"`
	OpenPositionCount int      `json:"open_position_count"`
	OpenSymbols       []string `json:"open_symbols"`
}

// CheckModeChangeAllowed verifies whether a decision mode change is allowed.
// Returns validation result with details.
func (s *Safeguards) CheckModeChangeAllowed(
	openPositions []string,
	currentMode string,
	newMode string,
) *ModeChangeValidation {

	if !s.config.BlockModeChangeWithOpenPositions {
		return &ModeChangeValidation{
			Allowed:           true,
			Reason:            "Mode change restriction disabled",
			OpenPositionCount: len(openPositions),
			OpenSymbols:       openPositions,
		}
	}

	if len(openPositions) == 0 {
		return &ModeChangeValidation{
			Allowed:           true,
			Reason:            "No open positions",
			OpenPositionCount: 0,
			OpenSymbols:       nil,
		}
	}

	reason := fmt.Sprintf(
		"S6: Cannot change decision mode from '%s' to '%s' while %d positions are open. "+
			"Close all positions first or wait for them to exit.",
		currentMode, newMode, len(openPositions))
	s.logDebug(reason)

	return &ModeChangeValidation{
		Allowed:           false,
		Reason:            reason,
		OpenPositionCount: len(openPositions),
		OpenSymbols:       openPositions,
	}
}

// ============================================================================
// COMBINED SAFEGUARD CHECK FOR EFFICIENCY EXIT
// ============================================================================

// EfficiencyExitSafeguardResult combines all safeguard checks for efficiency exit.
type EfficiencyExitSafeguardResult struct {
	Allowed             bool     `json:"allowed"`
	BlockedBy           []string `json:"blocked_by"`          // List of safeguards that blocked
	Reasons             []string `json:"reasons"`             // Reasons for blocking
	ConsecutiveSignals  int      `json:"consecutive_signals"` // Current count
	HoldTimeMet         bool     `json:"hold_time_met"`
	AboveBreakeven      bool     `json:"above_breakeven"`
	PriceDataFresh      bool     `json:"price_data_fresh"`
}

// CheckEfficiencyExitSafeguards performs all relevant safeguard checks for efficiency exit.
// Returns a comprehensive result with all check statuses.
func (s *Safeguards) CheckEfficiencyExitSafeguards(
	pos Position,
	currentPrice float64,
	isBelowThreshold bool,
) *EfficiencyExitSafeguardResult {

	result := &EfficiencyExitSafeguardResult{
		Allowed:    true,
		BlockedBy:  make([]string, 0),
		Reasons:    make([]string, 0),
	}

	symbol := pos.GetSymbol()

	// S1: Check minimum hold time
	if blocked, reason := s.CheckMinHoldTime(pos); blocked {
		result.Allowed = false
		result.BlockedBy = append(result.BlockedBy, "S1")
		result.Reasons = append(result.Reasons, reason)
		result.HoldTimeMet = false
	} else {
		result.HoldTimeMet = true
	}

	// S2: Check consecutive signals
	if blocked, reason := s.CheckConsecutiveSignals(symbol, isBelowThreshold); blocked {
		result.Allowed = false
		result.BlockedBy = append(result.BlockedBy, "S2")
		result.Reasons = append(result.Reasons, reason)
	}
	result.ConsecutiveSignals = s.GetConsecutiveSignalCount(symbol)

	// S3: Check breakeven verification
	if blocked, reason := s.CheckBreakevenVerification(pos, currentPrice); blocked {
		result.Allowed = false
		result.BlockedBy = append(result.BlockedBy, "S3")
		result.Reasons = append(result.Reasons, reason)
		result.AboveBreakeven = false
	} else {
		result.AboveBreakeven = true
	}

	// S4: Check price data freshness
	if blocked, reason := s.CheckPriceDataFreshness(pos); blocked {
		result.Allowed = false
		result.BlockedBy = append(result.BlockedBy, "S4")
		result.Reasons = append(result.Reasons, reason)
		result.PriceDataFresh = false
	} else {
		result.PriceDataFresh = true
	}

	return result
}

// ============================================================================
// CONFIGURATION METHODS
// ============================================================================

// SetConfig updates the safeguards configuration.
func (s *Safeguards) SetConfig(config *SafeguardsConfig) {
	if config == nil {
		return
	}
	s.config = config
}

// GetConfig returns the current safeguards configuration.
func (s *Safeguards) GetConfig() *SafeguardsConfig {
	return s.config
}

// SetDebugMode enables or disables debug logging.
func (s *Safeguards) SetDebugMode(debug bool) {
	s.debugMode = debug
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// logDebug logs a debug message if debug mode is enabled.
func (s *Safeguards) logDebug(format string, args ...interface{}) {
	if s.debugMode {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[EXIT-DECISION][SAFEGUARD][DEBUG] %s", msg)
	}
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
