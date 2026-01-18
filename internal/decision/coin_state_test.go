// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.1: Redis State Management
package decision

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================================================
// CoinState Struct Tests
// ============================================================================

func TestNewCoinState(t *testing.T) {
	cs := NewCoinState("BTCUSDT")

	if cs.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT, got %s", cs.Symbol)
	}

	if cs.Regime != RegimeConsolidating {
		t.Errorf("Expected default regime CONSOLIDATING, got %s", cs.Regime)
	}

	if cs.Decision != DecisionPending {
		t.Errorf("Expected default decision PENDING, got %s", cs.Decision)
	}

	if cs.BlockingReasons == nil || len(cs.BlockingReasons) != 0 {
		t.Error("Expected empty blocking reasons slice")
	}

	if cs.LastUpdated <= 0 {
		t.Error("Expected LastUpdated to be set")
	}
}

func TestCoinStateKey(t *testing.T) {
	testCases := []struct {
		userID   string
		symbol   string
		expected string
	}{
		{"user123", "BTCUSDT", "decision:user:user123:coin:BTCUSDT"},
		{"abc-def", "ETHUSDT", "decision:user:abc-def:coin:ETHUSDT"},
		{"123", "SOLUSDT", "decision:user:123:coin:SOLUSDT"},
	}

	for _, tc := range testCases {
		key := CoinStateKey(tc.userID, tc.symbol)
		if key != tc.expected {
			t.Errorf("CoinStateKey(%s, %s) = %s; expected %s", tc.userID, tc.symbol, key, tc.expected)
		}
	}
}

func TestCoinStatePattern(t *testing.T) {
	pattern := CoinStatePattern("user123")
	expected := "decision:user:user123:coin:*"

	if pattern != expected {
		t.Errorf("CoinStatePattern(user123) = %s; expected %s", pattern, expected)
	}
}

// ============================================================================
// ToMap and FromMap Conversion Tests
// ============================================================================

func TestToMapAndFromMap_RoundTrip(t *testing.T) {
	original := &CoinState{
		Symbol:          "BTCUSDT",
		Price:           45000.50,
		Regime:          RegimeTrending,
		ActiveStrategy:  "trend_following",
		ADX:             28.5,
		RSI:             62.3,
		EMA9:            44850.00,
		EMA21:           44720.00,
		Trend1H:         TrendBullish,
		Trend15M:        TrendBullish,
		ScoreTechnical:  72,
		ScoreContext:    65,
		ScoreLLM:        58,
		ScoreHistory:    70,
		ScoreFinal:      68,
		BlockingReasons: []string{"circuit_breaker", "max_positions"},
		LastUpdated:     1705312800000,
		Decision:        DecisionBlocked,
	}

	// Convert to map
	m := original.ToMap()

	// Verify key fields in map
	if m["symbol"] != "BTCUSDT" {
		t.Errorf("Expected symbol BTCUSDT in map, got %v", m["symbol"])
	}
	if m["regime"] != "TRENDING" {
		t.Errorf("Expected regime TRENDING in map, got %v", m["regime"])
	}

	// Convert back from map
	stringMap := make(map[string]string)
	for k, v := range m {
		stringMap[k] = v.(string)
	}

	parsed, err := FromMap(stringMap)
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	// Verify all fields match
	if parsed.Symbol != original.Symbol {
		t.Errorf("Symbol mismatch: got %s, expected %s", parsed.Symbol, original.Symbol)
	}
	if parsed.Price != original.Price {
		t.Errorf("Price mismatch: got %f, expected %f", parsed.Price, original.Price)
	}
	if parsed.Regime != original.Regime {
		t.Errorf("Regime mismatch: got %s, expected %s", parsed.Regime, original.Regime)
	}
	if parsed.ActiveStrategy != original.ActiveStrategy {
		t.Errorf("ActiveStrategy mismatch: got %s, expected %s", parsed.ActiveStrategy, original.ActiveStrategy)
	}
	if parsed.ADX != original.ADX {
		t.Errorf("ADX mismatch: got %f, expected %f", parsed.ADX, original.ADX)
	}
	if parsed.RSI != original.RSI {
		t.Errorf("RSI mismatch: got %f, expected %f", parsed.RSI, original.RSI)
	}
	if parsed.EMA9 != original.EMA9 {
		t.Errorf("EMA9 mismatch: got %f, expected %f", parsed.EMA9, original.EMA9)
	}
	if parsed.EMA21 != original.EMA21 {
		t.Errorf("EMA21 mismatch: got %f, expected %f", parsed.EMA21, original.EMA21)
	}
	if parsed.Trend1H != original.Trend1H {
		t.Errorf("Trend1H mismatch: got %s, expected %s", parsed.Trend1H, original.Trend1H)
	}
	if parsed.Trend15M != original.Trend15M {
		t.Errorf("Trend15M mismatch: got %s, expected %s", parsed.Trend15M, original.Trend15M)
	}
	if parsed.ScoreTechnical != original.ScoreTechnical {
		t.Errorf("ScoreTechnical mismatch: got %d, expected %d", parsed.ScoreTechnical, original.ScoreTechnical)
	}
	if parsed.ScoreContext != original.ScoreContext {
		t.Errorf("ScoreContext mismatch: got %d, expected %d", parsed.ScoreContext, original.ScoreContext)
	}
	if parsed.ScoreLLM != original.ScoreLLM {
		t.Errorf("ScoreLLM mismatch: got %d, expected %d", parsed.ScoreLLM, original.ScoreLLM)
	}
	if parsed.ScoreHistory != original.ScoreHistory {
		t.Errorf("ScoreHistory mismatch: got %d, expected %d", parsed.ScoreHistory, original.ScoreHistory)
	}
	if parsed.ScoreFinal != original.ScoreFinal {
		t.Errorf("ScoreFinal mismatch: got %d, expected %d", parsed.ScoreFinal, original.ScoreFinal)
	}
	if parsed.LastUpdated != original.LastUpdated {
		t.Errorf("LastUpdated mismatch: got %d, expected %d", parsed.LastUpdated, original.LastUpdated)
	}
	if parsed.Decision != original.Decision {
		t.Errorf("Decision mismatch: got %s, expected %s", parsed.Decision, original.Decision)
	}

	// Check blocking reasons
	if len(parsed.BlockingReasons) != len(original.BlockingReasons) {
		t.Errorf("BlockingReasons length mismatch: got %d, expected %d", len(parsed.BlockingReasons), len(original.BlockingReasons))
	} else {
		for i, reason := range original.BlockingReasons {
			if parsed.BlockingReasons[i] != reason {
				t.Errorf("BlockingReasons[%d] mismatch: got %s, expected %s", i, parsed.BlockingReasons[i], reason)
			}
		}
	}
}

func TestFromMap_EmptyBlockingReasons(t *testing.T) {
	m := map[string]string{
		"symbol":           "BTCUSDT",
		"blocking_reasons": "[]",
	}

	cs, err := FromMap(m)
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	if cs.BlockingReasons == nil {
		t.Error("BlockingReasons should not be nil")
	}

	if len(cs.BlockingReasons) != 0 {
		t.Errorf("BlockingReasons should be empty, got %d items", len(cs.BlockingReasons))
	}
}

func TestFromMap_MissingFields(t *testing.T) {
	// Minimal map with only required field
	m := map[string]string{
		"symbol": "BTCUSDT",
	}

	cs, err := FromMap(m)
	if err != nil {
		t.Fatalf("FromMap failed: %v", err)
	}

	if cs.Symbol != "BTCUSDT" {
		t.Errorf("Symbol mismatch: got %s, expected BTCUSDT", cs.Symbol)
	}

	// All other fields should be zero values
	if cs.Price != 0 {
		t.Errorf("Price should be 0, got %f", cs.Price)
	}
	if cs.Regime != "" {
		t.Errorf("Regime should be empty, got %s", cs.Regime)
	}
}

func TestToMap_BlockingReasonsJSON(t *testing.T) {
	cs := &CoinState{
		Symbol:          "BTCUSDT",
		BlockingReasons: []string{"reason1", "reason2"},
	}

	m := cs.ToMap()

	// Verify blocking_reasons is valid JSON
	blockingJSON := m["blocking_reasons"].(string)
	var reasons []string
	if err := json.Unmarshal([]byte(blockingJSON), &reasons); err != nil {
		t.Fatalf("blocking_reasons is not valid JSON: %v", err)
	}

	if len(reasons) != 2 {
		t.Errorf("Expected 2 reasons, got %d", len(reasons))
	}
	if reasons[0] != "reason1" || reasons[1] != "reason2" {
		t.Errorf("Unexpected reasons: %v", reasons)
	}
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestValidate_ValidCoinState(t *testing.T) {
	cs := &CoinState{
		Symbol:         "BTCUSDT",
		Regime:         RegimeTrending,
		Decision:       DecisionReady,
		Trend1H:        TrendBullish,
		Trend15M:       TrendNeutral,
		ScoreTechnical: 75,
		ScoreContext:   60,
		ScoreLLM:       50,
		ScoreHistory:   70,
		ScoreFinal:     65,
	}

	if err := cs.Validate(); err != nil {
		t.Errorf("Expected valid CoinState, got error: %v", err)
	}
}

func TestValidate_MissingSymbol(t *testing.T) {
	cs := &CoinState{
		Regime:   RegimeTrending,
		Decision: DecisionReady,
	}

	err := cs.Validate()
	if err == nil {
		t.Error("Expected error for missing symbol")
	}
}

func TestValidate_InvalidRegime(t *testing.T) {
	cs := &CoinState{
		Symbol: "BTCUSDT",
		Regime: "INVALID",
	}

	err := cs.Validate()
	if err == nil {
		t.Error("Expected error for invalid regime")
	}
}

func TestValidate_InvalidDecision(t *testing.T) {
	cs := &CoinState{
		Symbol:   "BTCUSDT",
		Decision: "INVALID",
	}

	err := cs.Validate()
	if err == nil {
		t.Error("Expected error for invalid decision")
	}
}

func TestValidate_InvalidTrendDirection(t *testing.T) {
	testCases := []struct {
		name    string
		trend1h TrendDirection
		trend15m TrendDirection
	}{
		{"invalid trend_1h", "INVALID", TrendBullish},
		{"invalid trend_15m", TrendBullish, "INVALID"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := &CoinState{
				Symbol:   "BTCUSDT",
				Trend1H:  tc.trend1h,
				Trend15M: tc.trend15m,
			}

			err := cs.Validate()
			if err == nil {
				t.Error("Expected error for invalid trend direction")
			}
		})
	}
}

func TestValidate_ScoreOutOfRange(t *testing.T) {
	testCases := []struct {
		name  string
		field string
		value int
	}{
		{"score_technical negative", "score_technical", -1},
		{"score_technical too high", "score_technical", 101},
		{"score_context negative", "score_context", -10},
		{"score_llm too high", "score_llm", 200},
		{"score_history negative", "score_history", -5},
		{"score_final too high", "score_final", 150},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := &CoinState{Symbol: "BTCUSDT"}

			switch tc.field {
			case "score_technical":
				cs.ScoreTechnical = tc.value
			case "score_context":
				cs.ScoreContext = tc.value
			case "score_llm":
				cs.ScoreLLM = tc.value
			case "score_history":
				cs.ScoreHistory = tc.value
			case "score_final":
				cs.ScoreFinal = tc.value
			}

			err := cs.Validate()
			if err == nil {
				t.Errorf("Expected error for %s = %d", tc.field, tc.value)
			}
		})
	}
}

func TestValidate_ValidScoreBoundaries(t *testing.T) {
	testCases := []struct {
		name  string
		value int
	}{
		{"zero", 0},
		{"one", 1},
		{"fifty", 50},
		{"ninety-nine", 99},
		{"hundred", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := &CoinState{
				Symbol:         "BTCUSDT",
				ScoreTechnical: tc.value,
				ScoreContext:   tc.value,
				ScoreLLM:       tc.value,
				ScoreHistory:   tc.value,
				ScoreFinal:     tc.value,
			}

			if err := cs.Validate(); err != nil {
				t.Errorf("Expected valid score %d, got error: %v", tc.value, err)
			}
		})
	}
}

// ============================================================================
// Helper Method Tests
// ============================================================================

func TestIsBlocked(t *testing.T) {
	testCases := []struct {
		name            string
		blockingReasons []string
		decision        Decision
		expected        bool
	}{
		{"no reasons, ready", []string{}, DecisionReady, false},
		{"no reasons, pending", []string{}, DecisionPending, false},
		{"has reasons", []string{"circuit_breaker"}, DecisionBlocked, true},
		{"no reasons, blocked decision", []string{}, DecisionBlocked, true},
		{"both", []string{"reason"}, DecisionBlocked, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cs := &CoinState{
				Symbol:          "BTCUSDT",
				BlockingReasons: tc.blockingReasons,
				Decision:        tc.decision,
			}

			if cs.IsBlocked() != tc.expected {
				t.Errorf("IsBlocked() = %v; expected %v", cs.IsBlocked(), tc.expected)
			}
		})
	}
}

func TestAddBlockingReason(t *testing.T) {
	cs := NewCoinState("BTCUSDT")
	cs.Decision = DecisionReady

	// Add first reason
	cs.AddBlockingReason("circuit_breaker")

	if len(cs.BlockingReasons) != 1 {
		t.Errorf("Expected 1 reason, got %d", len(cs.BlockingReasons))
	}
	if cs.BlockingReasons[0] != "circuit_breaker" {
		t.Errorf("Expected 'circuit_breaker', got '%s'", cs.BlockingReasons[0])
	}
	if cs.Decision != DecisionBlocked {
		t.Errorf("Decision should be BLOCKED after adding reason")
	}

	// Add second reason
	cs.AddBlockingReason("max_positions")

	if len(cs.BlockingReasons) != 2 {
		t.Errorf("Expected 2 reasons, got %d", len(cs.BlockingReasons))
	}

	// Try to add duplicate
	cs.AddBlockingReason("circuit_breaker")

	if len(cs.BlockingReasons) != 2 {
		t.Errorf("Duplicate should not be added, expected 2 reasons, got %d", len(cs.BlockingReasons))
	}
}

func TestClearBlockingReasons(t *testing.T) {
	cs := &CoinState{
		Symbol:          "BTCUSDT",
		BlockingReasons: []string{"reason1", "reason2"},
		Decision:        DecisionBlocked,
	}

	cs.ClearBlockingReasons()

	if len(cs.BlockingReasons) != 0 {
		t.Errorf("Expected 0 reasons after clear, got %d", len(cs.BlockingReasons))
	}
	if cs.Decision != DecisionReady {
		t.Errorf("Decision should be READY after clearing reasons")
	}
}

func TestClearBlockingReasons_NotBlockedDecision(t *testing.T) {
	cs := &CoinState{
		Symbol:          "BTCUSDT",
		BlockingReasons: []string{"reason1"},
		Decision:        DecisionPending,
	}

	cs.ClearBlockingReasons()

	// Decision should remain PENDING if it wasn't BLOCKED
	if cs.Decision != DecisionPending {
		t.Errorf("Decision should remain PENDING, got %s", cs.Decision)
	}
}

func TestUpdateTimestamp(t *testing.T) {
	cs := &CoinState{
		Symbol:      "BTCUSDT",
		LastUpdated: 1000, // Old timestamp
	}

	beforeUpdate := time.Now().UnixMilli()
	cs.UpdateTimestamp()
	afterUpdate := time.Now().UnixMilli()

	if cs.LastUpdated < beforeUpdate || cs.LastUpdated > afterUpdate {
		t.Errorf("LastUpdated %d should be between %d and %d", cs.LastUpdated, beforeUpdate, afterUpdate)
	}
}

// ============================================================================
// Enum Tests
// ============================================================================

func TestMarketRegimeValues(t *testing.T) {
	regimes := []MarketRegime{
		RegimeTrending,
		RegimeRanging,
		RegimeVolatile,
		RegimeConsolidating,
	}

	expected := []string{"TRENDING", "RANGING", "VOLATILE", "CONSOLIDATING"}

	for i, regime := range regimes {
		if string(regime) != expected[i] {
			t.Errorf("MarketRegime[%d] = %s; expected %s", i, regime, expected[i])
		}
	}
}

func TestTrendDirectionValues(t *testing.T) {
	trends := []TrendDirection{
		TrendBullish,
		TrendBearish,
		TrendNeutral,
	}

	expected := []string{"BULLISH", "BEARISH", "NEUTRAL"}

	for i, trend := range trends {
		if string(trend) != expected[i] {
			t.Errorf("TrendDirection[%d] = %s; expected %s", i, trend, expected[i])
		}
	}
}

func TestDecisionValues(t *testing.T) {
	decisions := []Decision{
		DecisionReady,
		DecisionBlocked,
		DecisionPending,
	}

	expected := []string{"READY", "BLOCKED", "PENDING"}

	for i, decision := range decisions {
		if string(decision) != expected[i] {
			t.Errorf("Decision[%d] = %s; expected %s", i, decision, expected[i])
		}
	}
}

// ============================================================================
// Constants Tests
// ============================================================================

func TestDefaultCoinStateTTL(t *testing.T) {
	expected := 5 * time.Minute

	if DefaultCoinStateTTL != expected {
		t.Errorf("DefaultCoinStateTTL = %v; expected %v", DefaultCoinStateTTL, expected)
	}
}

func TestPrefixCoinState(t *testing.T) {
	expected := "decision:user:%s:coin:%s"

	if PrefixCoinState != expected {
		t.Errorf("PrefixCoinState = %s; expected %s", PrefixCoinState, expected)
	}
}

// ============================================================================
// Security Tests - Key Sanitization
// ============================================================================

func TestCoinStateKey_Sanitization(t *testing.T) {
	testCases := []struct {
		name     string
		userID   string
		symbol   string
		expected string
	}{
		{
			name:     "clean inputs",
			userID:   "user123",
			symbol:   "BTCUSDT",
			expected: "decision:user:user123:coin:BTCUSDT",
		},
		{
			name:     "userID with colons (injection attempt)",
			userID:   "user:*:coin:ETHUSDT",
			symbol:   "BTCUSDT",
			expected: "decision:user:usercoinETHUSDT:coin:BTCUSDT",
		},
		{
			name:     "symbol with asterisk (glob injection)",
			userID:   "user123",
			symbol:   "*",
			expected: "decision:user:user123:coin:",
		},
		{
			name:     "userID with newline",
			userID:   "user\n123",
			symbol:   "BTCUSDT",
			expected: "decision:user:user123:coin:BTCUSDT",
		},
		{
			name:     "symbol with special chars",
			userID:   "user123",
			symbol:   "BTC[USDT]",
			expected: "decision:user:user123:coin:BTCUSDT",
		},
		{
			name:     "allows dash and underscore",
			userID:   "user-123_abc",
			symbol:   "BTC-USDT_PERP",
			expected: "decision:user:user-123_abc:coin:BTC-USDT_PERP",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := CoinStateKey(tc.userID, tc.symbol)
			if key != tc.expected {
				t.Errorf("CoinStateKey(%q, %q) = %q; expected %q", tc.userID, tc.symbol, key, tc.expected)
			}
		})
	}
}

func TestCoinStatePattern_Sanitization(t *testing.T) {
	testCases := []struct {
		name     string
		userID   string
		expected string
	}{
		{
			name:     "clean userID",
			userID:   "user123",
			expected: "decision:user:user123:coin:*",
		},
		{
			name:     "userID with injection attempt",
			userID:   "user*:coin:*",
			expected: "decision:user:usercoin:coin:*",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := CoinStatePattern(tc.userID)
			if pattern != tc.expected {
				t.Errorf("CoinStatePattern(%q) = %q; expected %q", tc.userID, pattern, tc.expected)
			}
		})
	}
}
