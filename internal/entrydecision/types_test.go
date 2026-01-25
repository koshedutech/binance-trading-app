// Package entrydecision provides tests for the Entry Decision System data structures.
// Epic 14: Chain Trading System - Story 14.8: Strategy-First Data Structure
package entrydecision

import (
	"testing"
	"time"
)

func TestStrategyType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		st       StrategyType
		expected bool
	}{
		{"pattern type is valid", StrategyTypePattern, true},
		{"score type is valid", StrategyTypeScore, true},
		{"empty is invalid", StrategyType(""), false},
		{"unknown is invalid", StrategyType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.st.IsValid(); got != tt.expected {
				t.Errorf("StrategyType.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPatternStatus_IsEntryReady(t *testing.T) {
	tests := []struct {
		name     string
		ps       PatternStatus
		expected bool
	}{
		{"ready status is entry ready", PatternStatusReady, true},
		{"watching is not entry ready", PatternStatusWatching, false},
		{"accumulation is not entry ready", PatternStatusAccumulation, false},
		{"consolidating is not entry ready", PatternStatusConsolidating, false},
		{"failed is not entry ready", PatternStatusFailed, false},
		{"expired is not entry ready", PatternStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ps.IsEntryReady(); got != tt.expected {
				t.Errorf("PatternStatus.IsEntryReady() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPatternStatus_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		ps       PatternStatus
		expected bool
	}{
		{"watching is active", PatternStatusWatching, true},
		{"accumulation is active", PatternStatusAccumulation, true},
		{"consolidating is active", PatternStatusConsolidating, true},
		{"ready is active", PatternStatusReady, true},
		{"failed is not active", PatternStatusFailed, false},
		{"expired is not active", PatternStatusExpired, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ps.IsActive(); got != tt.expected {
				t.Errorf("PatternStatus.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewPatternCoinMatch(t *testing.T) {
	cm := NewPatternCoinMatch("BTCUSDT", 2, PatternStatusConsolidating, "3/6 candles")

	if cm.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %v, want BTCUSDT", cm.Symbol)
	}
	if cm.Step != 2 {
		t.Errorf("Step = %v, want 2", cm.Step)
	}
	if cm.Status != PatternStatusConsolidating {
		t.Errorf("Status = %v, want consolidating", cm.Status)
	}
	if cm.Details != "3/6 candles" {
		t.Errorf("Details = %v, want '3/6 candles'", cm.Details)
	}
	if cm.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestNewScoreCoinMatch(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		score         int
		threshold     int
		expectedReady bool
	}{
		{"score above threshold", "BTCUSDT", 82, 75, true},
		{"score at threshold", "ETHUSDT", 75, 75, true},
		{"score below threshold", "SOLUSDT", 68, 75, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := NewScoreCoinMatch(tt.symbol, tt.score, tt.threshold)

			if cm.Symbol != tt.symbol {
				t.Errorf("Symbol = %v, want %v", cm.Symbol, tt.symbol)
			}
			if cm.Score != tt.score {
				t.Errorf("Score = %v, want %v", cm.Score, tt.score)
			}
			if cm.Ready != tt.expectedReady {
				t.Errorf("Ready = %v, want %v", cm.Ready, tt.expectedReady)
			}
		})
	}
}

func TestCoinMatch_IsReady(t *testing.T) {
	tests := []struct {
		name     string
		cm       CoinMatch
		expected bool
	}{
		{
			name:     "pattern-based ready",
			cm:       CoinMatch{Symbol: "BTCUSDT", Status: PatternStatusReady, Step: 3},
			expected: true,
		},
		{
			name:     "pattern-based not ready",
			cm:       CoinMatch{Symbol: "BTCUSDT", Status: PatternStatusConsolidating, Step: 2},
			expected: false,
		},
		{
			name:     "score-based ready",
			cm:       CoinMatch{Symbol: "BTCUSDT", Score: 82, Ready: true},
			expected: true,
		},
		{
			name:     "score-based not ready",
			cm:       CoinMatch{Symbol: "BTCUSDT", Score: 68, Ready: false},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cm.IsReady(); got != tt.expected {
				t.Errorf("CoinMatch.IsReady() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCoinMatch_String(t *testing.T) {
	tests := []struct {
		name     string
		cm       CoinMatch
		contains string
	}{
		{
			name:     "pattern-based string",
			cm:       CoinMatch{Symbol: "BTCUSDT", Step: 2, Status: PatternStatusConsolidating, Details: "3/6 candles"},
			contains: "BTCUSDT",
		},
		{
			name:     "score-based string",
			cm:       CoinMatch{Symbol: "ETHUSDT", Score: 82, Ready: true},
			contains: "ETHUSDT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cm.String()
			if got == "" {
				t.Error("String() should not be empty")
			}
		})
	}
}

func TestNewStrategyMatch(t *testing.T) {
	sm := NewStrategyMatch("scalp", "volume_imbalance", "ravindra_volume_imbalance", StrategyTypePattern, "15m")

	if sm.Mode != "scalp" {
		t.Errorf("Mode = %v, want scalp", sm.Mode)
	}
	if sm.Strategy != "volume_imbalance" {
		t.Errorf("Strategy = %v, want volume_imbalance", sm.Strategy)
	}
	if sm.SubStrategy != "ravindra_volume_imbalance" {
		t.Errorf("SubStrategy = %v, want ravindra_volume_imbalance", sm.SubStrategy)
	}
	if sm.Type != StrategyTypePattern {
		t.Errorf("Type = %v, want pattern", sm.Type)
	}
	if sm.Timeframe != "15m" {
		t.Errorf("Timeframe = %v, want 15m", sm.Timeframe)
	}
	if !sm.Enabled {
		t.Error("Enabled should be true by default")
	}
	if sm.Coins == nil {
		t.Error("Coins should not be nil")
	}
}

func TestStrategyMatch_ChainMethods(t *testing.T) {
	sm := NewStrategyMatch("swing", "trend_following", "", StrategyTypeScore, "1h").
		WithThreshold(75).
		WithTimeframes([]string{"1h", "4h"}).
		WithRequirements(&StrategyRequirements{
			Timeframes: []string{"1h", "4h"},
			Filters:    map[string]interface{}{"min_volume": 1000000},
		})

	if sm.Threshold != 75 {
		t.Errorf("Threshold = %v, want 75", sm.Threshold)
	}
	if len(sm.Timeframes) != 2 {
		t.Errorf("Timeframes length = %v, want 2", len(sm.Timeframes))
	}
	if sm.Requirements == nil {
		t.Error("Requirements should not be nil")
	}
}

func TestStrategyMatch_AddCoin(t *testing.T) {
	sm := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")

	sm.AddCoin(CoinMatch{Symbol: "BTCUSDT", Step: 2, Status: PatternStatusConsolidating})
	sm.AddCoin(CoinMatch{Symbol: "ETHUSDT", Step: 3, Status: PatternStatusReady})
	sm.AddCoin(CoinMatch{Symbol: "SOLUSDT", Step: 1, Status: PatternStatusAccumulation})

	if len(sm.Coins) != 3 {
		t.Errorf("Coins length = %v, want 3", len(sm.Coins))
	}
}

func TestStrategyMatch_GetReadyCoins(t *testing.T) {
	sm := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")

	sm.AddCoin(CoinMatch{Symbol: "BTCUSDT", Step: 2, Status: PatternStatusConsolidating})
	sm.AddCoin(CoinMatch{Symbol: "ETHUSDT", Step: 3, Status: PatternStatusReady})
	sm.AddCoin(CoinMatch{Symbol: "SOLUSDT", Step: 3, Status: PatternStatusReady})
	sm.AddCoin(CoinMatch{Symbol: "DOGEUSDT", Step: 1, Status: PatternStatusWatching})

	ready := sm.GetReadyCoins()
	if len(ready) != 2 {
		t.Errorf("GetReadyCoins() length = %v, want 2", len(ready))
	}

	if sm.GetReadyCount() != 2 {
		t.Errorf("GetReadyCount() = %v, want 2", sm.GetReadyCount())
	}

	if sm.GetWatchingCount() != 2 {
		t.Errorf("GetWatchingCount() = %v, want 2", sm.GetWatchingCount())
	}
}

func TestStrategyMatch_UniqueKey(t *testing.T) {
	tests := []struct {
		name        string
		sm          *StrategyMatch
		expectedKey string
	}{
		{
			name:        "with sub-strategy",
			sm:          NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m"),
			expectedKey: "scalp:volume_imbalance:ravindra:15m",
		},
		{
			name:        "without sub-strategy",
			sm:          NewStrategyMatch("swing", "trend_following", "", StrategyTypeScore, "1h"),
			expectedKey: "swing:trend_following:1h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sm.UniqueKey(); got != tt.expectedKey {
				t.Errorf("UniqueKey() = %v, want %v", got, tt.expectedKey)
			}
		})
	}
}

func TestModeStrategies(t *testing.T) {
	ms := NewModeStrategies("scalp", true)

	sm1 := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	sm1.Enabled = true
	sm2 := NewStrategyMatch("scalp", "breakout", "", StrategyTypePattern, "5m")
	sm2.Enabled = false

	ms.AddStrategy(*sm1)
	ms.AddStrategy(*sm2)

	if ms.GetStrategyCount() != 2 {
		t.Errorf("GetStrategyCount() = %v, want 2", ms.GetStrategyCount())
	}

	if ms.GetEnabledCount() != 1 {
		t.Errorf("GetEnabledCount() = %v, want 1", ms.GetEnabledCount())
	}
}

func TestModeStrategies_GetTotalReadyCoins(t *testing.T) {
	ms := NewModeStrategies("scalp", true)

	sm1 := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	sm1.AddCoin(CoinMatch{Symbol: "BTCUSDT", Status: PatternStatusReady})
	sm1.AddCoin(CoinMatch{Symbol: "ETHUSDT", Status: PatternStatusConsolidating})

	sm2 := NewStrategyMatch("scalp", "breakout", "", StrategyTypePattern, "5m")
	sm2.AddCoin(CoinMatch{Symbol: "SOLUSDT", Status: PatternStatusReady})
	sm2.AddCoin(CoinMatch{Symbol: "DOGEUSDT", Status: PatternStatusReady})

	ms.AddStrategy(*sm1)
	ms.AddStrategy(*sm2)

	if ms.GetTotalReadyCoins() != 3 {
		t.Errorf("GetTotalReadyCoins() = %v, want 3", ms.GetTotalReadyCoins())
	}
}

func TestEntryDecisionResponse(t *testing.T) {
	edr := NewEntryDecisionResponse()

	sm1 := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	sm1.Enabled = true
	sm1.AddCoin(CoinMatch{Symbol: "BTCUSDT", Status: PatternStatusReady})
	sm1.AddCoin(CoinMatch{Symbol: "ETHUSDT", Status: PatternStatusConsolidating})

	sm2 := NewStrategyMatch("swing", "trend_following", "", StrategyTypeScore, "1h")
	sm2.Enabled = true
	sm2.AddCoin(CoinMatch{Symbol: "BTCUSDT", Score: 82, Ready: true})
	sm2.AddCoin(CoinMatch{Symbol: "SOLUSDT", Score: 68, Ready: false})

	edr.AddStrategy(*sm1)
	edr.AddStrategy(*sm2)

	if edr.TotalStrategies != 2 {
		t.Errorf("TotalStrategies = %v, want 2", edr.TotalStrategies)
	}

	if edr.EnabledStrategies != 2 {
		t.Errorf("EnabledStrategies = %v, want 2", edr.EnabledStrategies)
	}

	if edr.TotalCoinsReady != 2 {
		t.Errorf("TotalCoinsReady = %v, want 2", edr.TotalCoinsReady)
	}

	if edr.TotalCoinsWatching != 2 {
		t.Errorf("TotalCoinsWatching = %v, want 2", edr.TotalCoinsWatching)
	}
}

func TestEntryDecisionResponse_GroupByMode(t *testing.T) {
	edr := NewEntryDecisionResponse()

	// Add strategies from different modes
	sm1 := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	sm2 := NewStrategyMatch("scalp", "breakout", "", StrategyTypePattern, "5m")
	sm3 := NewStrategyMatch("swing", "trend_following", "", StrategyTypeScore, "1h")
	sm4 := NewStrategyMatch("position", "momentum", "", StrategyTypeScore, "4h")

	edr.AddStrategy(*sm1)
	edr.AddStrategy(*sm2)
	edr.AddStrategy(*sm3)
	edr.AddStrategy(*sm4)

	edr.GroupByMode()

	if len(edr.ByMode) != 3 {
		t.Errorf("ByMode length = %v, want 3", len(edr.ByMode))
	}

	// Check ordering: scalp, swing, position
	if edr.ByMode[0].Mode != "scalp" {
		t.Errorf("First mode = %v, want scalp", edr.ByMode[0].Mode)
	}
	if edr.ByMode[1].Mode != "swing" {
		t.Errorf("Second mode = %v, want swing", edr.ByMode[1].Mode)
	}
	if edr.ByMode[2].Mode != "position" {
		t.Errorf("Third mode = %v, want position", edr.ByMode[2].Mode)
	}

	// Scalp should have 2 strategies
	if len(edr.ByMode[0].Strategies) != 2 {
		t.Errorf("Scalp strategies = %v, want 2", len(edr.ByMode[0].Strategies))
	}
}

func TestEntryDecisionResponse_CalculateSummary(t *testing.T) {
	edr := NewEntryDecisionResponse()

	sm1 := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	sm1.Enabled = true
	sm1.AddCoin(CoinMatch{Symbol: "BTCUSDT", Status: PatternStatusReady})
	sm1.AddCoin(CoinMatch{Symbol: "ETHUSDT", Status: PatternStatusConsolidating})

	sm2 := NewStrategyMatch("swing", "trend_following", "", StrategyTypeScore, "1h")
	sm2.Enabled = false // disabled

	edr.Strategies = append(edr.Strategies, *sm1, *sm2)
	edr.CalculateSummary()

	if edr.TotalStrategies != 2 {
		t.Errorf("TotalStrategies = %v, want 2", edr.TotalStrategies)
	}

	if edr.EnabledStrategies != 1 {
		t.Errorf("EnabledStrategies = %v, want 1", edr.EnabledStrategies)
	}
}

func TestEntryCandidatesResponse(t *testing.T) {
	ecr := NewEntryCandidatesResponse()

	candidate := EntryCandidate{
		Symbol:       "BTCUSDT",
		CurrentPrice: 43150.20,
		Mode:         "scalp",
		Strategy:     "volume_imbalance",
		SubStrategy:  "ravindra",
		Type:         StrategyTypePattern,
		Timeframe:    "15m",
		Step:         3,
		Status:       PatternStatusReady,
		Direction:    "long",
		Priority:     1,
	}

	ecr.AddCandidate(candidate)

	if ecr.TotalCandidates != 1 {
		t.Errorf("TotalCandidates = %v, want 1", ecr.TotalCandidates)
	}

	if len(ecr.Candidates) != 1 {
		t.Errorf("Candidates length = %v, want 1", len(ecr.Candidates))
	}
}

func TestPatternProgress(t *testing.T) {
	pp := NewPatternProgress("BTCUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)

	if pp.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %v, want BTCUSDT", pp.Symbol)
	}

	if pp.CurrentStep != 1 {
		t.Errorf("CurrentStep = %v, want 1", pp.CurrentStep)
	}

	if pp.TotalSteps != 3 {
		t.Errorf("TotalSteps = %v, want 3", pp.TotalSteps)
	}

	if pp.Status != PatternStatusWatching {
		t.Errorf("Status = %v, want watching", pp.Status)
	}
}

func TestPatternProgress_AdvanceStep(t *testing.T) {
	pp := NewPatternProgress("BTCUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)

	// Initialize step details
	pp.StepDetails[0] = StepDetail{StepNumber: 1, Name: "Volume Spike"}
	pp.StepDetails[1] = StepDetail{StepNumber: 2, Name: "Consolidation"}
	pp.StepDetails[2] = StepDetail{StepNumber: 3, Name: "Breakout"}

	// Advance from step 1 to 2
	if !pp.AdvanceStep("Volume spike detected") {
		t.Error("AdvanceStep should return true for step 1->2")
	}
	if pp.CurrentStep != 2 {
		t.Errorf("CurrentStep = %v, want 2", pp.CurrentStep)
	}

	// Advance from step 2 to 3
	if !pp.AdvanceStep("Consolidation complete") {
		t.Error("AdvanceStep should return true for step 2->3")
	}
	if pp.CurrentStep != 3 {
		t.Errorf("CurrentStep = %v, want 3", pp.CurrentStep)
	}

	// Advance from step 3 to completion
	if !pp.AdvanceStep("Breakout confirmed") {
		t.Error("AdvanceStep should return true for step 3->complete")
	}
	if pp.CurrentStep != 4 {
		t.Errorf("CurrentStep = %v, want 4 (past total)", pp.CurrentStep)
	}
	if pp.Status != PatternStatusReady {
		t.Errorf("Status = %v, want ready", pp.Status)
	}

	// Cannot advance past completion
	if pp.AdvanceStep("Should fail") {
		t.Error("AdvanceStep should return false when already complete")
	}
}

func TestPatternProgress_SetStatus(t *testing.T) {
	pp := NewPatternProgress("BTCUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)

	pp.SetStatus(PatternStatusAccumulation)
	if pp.Status != PatternStatusAccumulation {
		t.Errorf("Status = %v, want accumulation", pp.Status)
	}

	pp.SetStatus(PatternStatusFailed)
	if pp.Status != PatternStatusFailed {
		t.Errorf("Status = %v, want failed", pp.Status)
	}
}

func TestPatternProgress_IsComplete(t *testing.T) {
	pp := NewPatternProgress("BTCUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)

	if pp.IsComplete() {
		t.Error("IsComplete should be false initially")
	}

	pp.SetStatus(PatternStatusReady)
	if !pp.IsComplete() {
		t.Error("IsComplete should be true when status is ready")
	}
}

func TestPatternProgress_IsExpired(t *testing.T) {
	pp := NewPatternProgress("BTCUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)

	if pp.IsExpired() {
		t.Error("IsExpired should be false initially")
	}

	pp.SetStatus(PatternStatusExpired)
	if !pp.IsExpired() {
		t.Error("IsExpired should be true when status is expired")
	}

	// Test with ExpiresAt
	pp2 := NewPatternProgress("ETHUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)
	pp2.ExpiresAt = time.Now().Add(-1 * time.Hour) // Expired 1 hour ago
	if !pp2.IsExpired() {
		t.Error("IsExpired should be true when ExpiresAt is in the past")
	}
}

func TestPatternProgress_ToCoinMatch(t *testing.T) {
	pp := NewPatternProgress("BTCUSDT", "volume_imbalance", "ravindra", "scalp", "15m", 3)
	pp.CurrentStep = 2
	pp.Status = PatternStatusConsolidating
	pp.StepDetails[1] = StepDetail{
		StepNumber: 2,
		Name:       "Consolidation",
		Progress:   "3/6 candles",
	}

	cm := pp.ToCoinMatch()

	if cm.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %v, want BTCUSDT", cm.Symbol)
	}
	if cm.Step != 2 {
		t.Errorf("Step = %v, want 2", cm.Step)
	}
	if cm.Status != PatternStatusConsolidating {
		t.Errorf("Status = %v, want consolidating", cm.Status)
	}
	if cm.Details != "3/6 candles" {
		t.Errorf("Details = %v, want '3/6 candles'", cm.Details)
	}
}

// Test handling same strategy in different modes
func TestSameStrategyDifferentModes(t *testing.T) {
	// Volume Imbalance in Scalp mode (15m)
	smScalp := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	smScalp.AddCoin(CoinMatch{Symbol: "BTCUSDT", Step: 2, Status: PatternStatusConsolidating})

	// Volume Imbalance in Swing mode (1h)
	smSwing := NewStrategyMatch("swing", "volume_imbalance", "ravindra", StrategyTypePattern, "1h")
	smSwing.AddCoin(CoinMatch{Symbol: "BTCUSDT", Step: 1, Status: PatternStatusAccumulation})

	// Volume Imbalance in Position mode (4h)
	smPosition := NewStrategyMatch("position", "volume_imbalance", "ravindra", StrategyTypePattern, "4h")
	smPosition.AddCoin(CoinMatch{Symbol: "BTCUSDT", Step: 3, Status: PatternStatusReady})

	// All should have unique keys
	keyScalp := smScalp.UniqueKey()
	keySwing := smSwing.UniqueKey()
	keyPosition := smPosition.UniqueKey()

	if keyScalp == keySwing || keySwing == keyPosition || keyScalp == keyPosition {
		t.Errorf("Keys should be unique: scalp=%v, swing=%v, position=%v", keyScalp, keySwing, keyPosition)
	}

	// Same coin (BTCUSDT) can have different status in each mode
	edr := NewEntryDecisionResponse()
	edr.AddStrategy(*smScalp)
	edr.AddStrategy(*smSwing)
	edr.AddStrategy(*smPosition)

	// Only position mode has BTCUSDT ready
	if edr.TotalCoinsReady != 1 {
		t.Errorf("TotalCoinsReady = %v, want 1 (only position mode)", edr.TotalCoinsReady)
	}
}

// Test mixed pattern and score strategies
func TestMixedStrategyTypes(t *testing.T) {
	edr := NewEntryDecisionResponse()

	// Pattern-based strategy
	smPattern := NewStrategyMatch("scalp", "volume_imbalance", "ravindra", StrategyTypePattern, "15m")
	smPattern.AddCoin(CoinMatch{Symbol: "BTCUSDT", Step: 3, Status: PatternStatusReady})
	smPattern.AddCoin(CoinMatch{Symbol: "ETHUSDT", Step: 2, Status: PatternStatusConsolidating})

	// Score-based strategy
	smScore := NewStrategyMatch("swing", "trend_following", "", StrategyTypeScore, "1h").WithThreshold(75)
	smScore.AddCoin(CoinMatch{Symbol: "BTCUSDT", Score: 82, Ready: true})
	smScore.AddCoin(CoinMatch{Symbol: "ETHUSDT", Score: 68, Ready: false})
	smScore.AddCoin(CoinMatch{Symbol: "SOLUSDT", Score: 77, Ready: true})

	edr.AddStrategy(*smPattern)
	edr.AddStrategy(*smScore)

	// Total ready: 1 from pattern (BTCUSDT) + 2 from score (BTCUSDT, SOLUSDT) = 3
	if edr.TotalCoinsReady != 3 {
		t.Errorf("TotalCoinsReady = %v, want 3", edr.TotalCoinsReady)
	}

	// Total watching: 1 from pattern (ETHUSDT) + 1 from score (ETHUSDT) = 2
	if edr.TotalCoinsWatching != 2 {
		t.Errorf("TotalCoinsWatching = %v, want 2", edr.TotalCoinsWatching)
	}
}

// ============================================================================
// TEST: Nil Safety and Edge Cases (Story 14.8 Code Review Fixes)
// ============================================================================

func TestCoinMatch_NilSafety(t *testing.T) {
	t.Run("IsReady on nil returns false", func(t *testing.T) {
		var cm *CoinMatch
		if cm.IsReady() {
			t.Error("expected false for nil CoinMatch")
		}
	})

	t.Run("String on nil returns empty", func(t *testing.T) {
		var cm *CoinMatch
		if cm.String() != "" {
			t.Error("expected empty string for nil CoinMatch")
		}
	})
}

func TestPatternProgress_NilSafety(t *testing.T) {
	t.Run("ToCoinMatch on nil returns empty", func(t *testing.T) {
		var pp *PatternProgress
		cm := pp.ToCoinMatch()
		if cm.Symbol != "" {
			t.Error("expected empty CoinMatch for nil PatternProgress")
		}
	})
}

func TestNewScoreCoinMatch_Validation(t *testing.T) {
	t.Run("clamps negative score to 0", func(t *testing.T) {
		cm := NewScoreCoinMatch("BTCUSDT", -10, 50)
		if cm.Score != 0 {
			t.Errorf("expected score 0, got %d", cm.Score)
		}
		if cm.Ready {
			t.Error("expected Ready=false when score is clamped to 0")
		}
	})

	t.Run("clamps score above 100 to 100", func(t *testing.T) {
		cm := NewScoreCoinMatch("BTCUSDT", 150, 50)
		if cm.Score != 100 {
			t.Errorf("expected score 100, got %d", cm.Score)
		}
		if !cm.Ready {
			t.Error("expected Ready=true when score is 100")
		}
	})

	t.Run("clamps negative threshold to 0", func(t *testing.T) {
		cm := NewScoreCoinMatch("BTCUSDT", 50, -10)
		// With threshold clamped to 0, score 50 should be ready
		if !cm.Ready {
			t.Error("expected Ready=true when threshold clamped to 0")
		}
	})

	t.Run("clamps threshold above 100 to 100", func(t *testing.T) {
		cm := NewScoreCoinMatch("BTCUSDT", 99, 150)
		// With threshold clamped to 100, score 99 should not be ready
		if cm.Ready {
			t.Error("expected Ready=false when threshold clamped to 100")
		}
	})

	t.Run("handles edge case score equals clamped threshold", func(t *testing.T) {
		cm := NewScoreCoinMatch("BTCUSDT", 100, 200)
		// Both get clamped to 100, so score == threshold = ready
		if !cm.Ready {
			t.Error("expected Ready=true when score equals threshold at 100")
		}
	})
}

func TestPatternProgress_ToCoinMatch_BoundsCheck(t *testing.T) {
	t.Run("handles CurrentStep = 0", func(t *testing.T) {
		pp := &PatternProgress{
			Symbol:      "BTCUSDT",
			CurrentStep: 0,
			TotalSteps:  3,
			Status:      PatternStatusWatching,
			StepDetails: make([]StepDetail, 3),
		}

		// Should not panic
		cm := pp.ToCoinMatch()

		if cm.Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", cm.Symbol)
		}
		// Details should be the fallback format
		expectedDetails := "Step 0/3"
		if cm.Details != expectedDetails {
			t.Errorf("expected details '%s', got '%s'", expectedDetails, cm.Details)
		}
	})

	t.Run("handles CurrentStep > TotalSteps", func(t *testing.T) {
		pp := &PatternProgress{
			Symbol:      "BTCUSDT",
			CurrentStep: 5, // Beyond total steps
			TotalSteps:  3,
			Status:      PatternStatusReady,
			StepDetails: make([]StepDetail, 3),
		}

		// Should not panic
		cm := pp.ToCoinMatch()

		if cm.Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", cm.Symbol)
		}
		// Details should be the fallback format
		expectedDetails := "Step 5/3"
		if cm.Details != expectedDetails {
			t.Errorf("expected details '%s', got '%s'", expectedDetails, cm.Details)
		}
	})

	t.Run("handles empty StepDetails", func(t *testing.T) {
		pp := &PatternProgress{
			Symbol:      "BTCUSDT",
			CurrentStep: 2,
			TotalSteps:  3,
			Status:      PatternStatusConsolidating,
			StepDetails: nil, // Empty/nil
		}

		// Should not panic
		cm := pp.ToCoinMatch()

		if cm.Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", cm.Symbol)
		}
	})

	t.Run("uses Progress when in bounds", func(t *testing.T) {
		pp := &PatternProgress{
			Symbol:      "BTCUSDT",
			CurrentStep: 2,
			TotalSteps:  3,
			Status:      PatternStatusConsolidating,
			StepDetails: []StepDetail{
				{StepNumber: 1, Progress: "Step 1 done"},
				{StepNumber: 2, Progress: "3/6 candles"},
				{StepNumber: 3, Progress: ""},
			},
		}

		cm := pp.ToCoinMatch()

		if cm.Details != "3/6 candles" {
			t.Errorf("expected details '3/6 candles', got '%s'", cm.Details)
		}
	})
}
