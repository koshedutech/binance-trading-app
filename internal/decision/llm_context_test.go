// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.16: Enhanced LLM Context - Tests
package decision

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewLLMContext(t *testing.T) {
	ctx := NewLLMContext("BTCUSDT")

	if ctx == nil {
		t.Fatal("NewLLMContext returned nil")
	}

	if ctx.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, want BTCUSDT", ctx.Symbol)
	}

	if ctx.Timestamp <= 0 {
		t.Error("Timestamp should be set")
	}

	if ctx.BlockingReasons == nil {
		t.Error("BlockingReasons should be initialized")
	}
}

func TestBuildLLMContext_NilState(t *testing.T) {
	ctx := BuildLLMContext(nil)
	if ctx != nil {
		t.Error("BuildLLMContext(nil) should return nil")
	}
}

func TestBuildLLMContext_BasicState(t *testing.T) {
	state := &CoinState{
		Symbol:       "BTCUSDT",
		Price:        45000.0,
		ADX:          28.5,
		RSI:          55.0,
		EMA9:         44900.0,
		EMA21:        44700.0,
		ATR:          500.0,
		ATRAverage:   400.0,
		Trend1H:      TrendBullish,
		Trend15M:     TrendBullish,
		Regime:       RegimeTrending,
		ScoreFinal:   72,
		ScoreTechnical: 80,
		ScoreContext: 65,
	}

	ctx := BuildLLMContext(state)

	if ctx == nil {
		t.Fatal("BuildLLMContext returned nil for valid state")
	}

	// Check basic data copy
	if ctx.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, want BTCUSDT", ctx.Symbol)
	}
	if ctx.CurrentPrice != 45000.0 {
		t.Errorf("CurrentPrice = %f, want 45000", ctx.CurrentPrice)
	}
	if ctx.ADXValue != 28.5 {
		t.Errorf("ADXValue = %f, want 28.5", ctx.ADXValue)
	}
	if ctx.RSI != 55.0 {
		t.Errorf("RSI = %f, want 55", ctx.RSI)
	}

	// Check trend alignment
	if !ctx.TrendAlignment {
		t.Error("TrendAlignment should be true when 1H and 15M match")
	}

	// Check interpretations are set
	if ctx.ADXInterpretation == "" {
		t.Error("ADXInterpretation should be set")
	}
	if ctx.RSIInterpretation == "" {
		t.Error("RSIInterpretation should be set")
	}

	// Check scores
	if ctx.FinalScore != 72 {
		t.Errorf("FinalScore = %d, want 72", ctx.FinalScore)
	}
}

func TestBuildLLMContext_TrendMisalignment(t *testing.T) {
	state := &CoinState{
		Symbol:  "ETHUSDT",
		Price:   3000.0,
		Trend1H: TrendBullish,
		Trend15M: TrendBearish,
	}

	ctx := BuildLLMContext(state)

	if ctx.TrendAlignment {
		t.Error("TrendAlignment should be false when 1H and 15M differ")
	}
}

func TestBuildLLMContextExtended(t *testing.T) {
	state := &CoinState{
		Symbol:  "BTCUSDT",
		Price:   45000.0,
		ADX:     25.0,
		RSI:     60.0,
		Trend1H: TrendBullish,
	}

	ext := &ExtendedMarketData{
		VWAPValue:         44800.0,
		NearestSupport:    44000.0,
		NearestResistance: 46000.0,
		CurrentVolume:     1500.0,
		AverageVolume:     1000.0,
		BTCTrend:          "BULLISH",
		BTCCorrelation:    0.85,
		EMA50:             43500.0,
	}

	ctx := BuildLLMContextExtended(state, ext)

	if ctx == nil {
		t.Fatal("BuildLLMContextExtended returned nil")
	}

	// Check extended data
	if ctx.VWAPValue != 44800.0 {
		t.Errorf("VWAPValue = %f, want 44800", ctx.VWAPValue)
	}
	if ctx.NearestSupport != 44000.0 {
		t.Errorf("NearestSupport = %f, want 44000", ctx.NearestSupport)
	}
	if ctx.NearestResistance != 46000.0 {
		t.Errorf("NearestResistance = %f, want 46000", ctx.NearestResistance)
	}

	// Check distance calculations
	if ctx.DistanceToSupport <= 0 {
		t.Error("DistanceToSupport should be positive")
	}
	if ctx.DistanceToResistance <= 0 {
		t.Error("DistanceToResistance should be positive")
	}

	// Check volume ratio
	if ctx.VolumeRatio != 1.5 {
		t.Errorf("VolumeRatio = %f, want 1.5", ctx.VolumeRatio)
	}

	// Check BTC context
	if ctx.BTCTrend != "BULLISH" {
		t.Errorf("BTCTrend = %s, want BULLISH", ctx.BTCTrend)
	}
	if ctx.BTCCorrelation != 0.85 {
		t.Errorf("BTCCorrelation = %f, want 0.85", ctx.BTCCorrelation)
	}

	// Check EMA50 was set
	if ctx.EMA50 != 43500.0 {
		t.Errorf("EMA50 = %f, want 43500", ctx.EMA50)
	}
}

// ========== Interpretation Helper Tests ==========

func TestInterpretADX(t *testing.T) {
	tests := []struct {
		value    float64
		contains string
	}{
		{0, "No data"},
		{5, "No trend"},
		{15, "Weak trend"},
		{22, "Moderate trend"},
		{30, "Strong trend"},
		{45, "Very strong trend"},
	}

	for _, tt := range tests {
		result := InterpretADX(tt.value)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretADX(%f) = %s, should contain %q", tt.value, result, tt.contains)
		}
	}
}

func TestInterpretRSI(t *testing.T) {
	tests := []struct {
		value    float64
		contains string
	}{
		{0, "No data"},
		{15, "Extremely oversold"},
		{25, "Oversold"},
		{35, "Bearish momentum"},
		{50, "Neutral"},
		{65, "Bullish momentum"},
		{75, "Overbought"},
		{85, "Extremely overbought"},
	}

	for _, tt := range tests {
		result := InterpretRSI(tt.value)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretRSI(%f) = %s, should contain %q", tt.value, result, tt.contains)
		}
	}
}

func TestInterpretPriceVsEMA(t *testing.T) {
	tests := []struct {
		price, ema9, ema21, ema50 float64
		contains                  string
	}{
		{0, 100, 100, 100, "No price data"},
		{100, 0, 0, 0, "No EMA data"},
		{100, 90, 80, 70, "Above all EMAs"},
		{60, 90, 80, 70, "Below all EMAs"},
		{85, 90, 80, 70, "Pullback"},       // Above 21/50, below 9 - returns "Below EMA9, above EMA21/50 (Pullback in uptrend)"
		{75, 90, 80, 70, "Above EMA50"},    // Above 50, below 9/21 - returns "Above EMA50 only (Short-term bearish, long-term support)"
	}

	for _, tt := range tests {
		result := InterpretPriceVsEMA(tt.price, tt.ema9, tt.ema21, tt.ema50)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretPriceVsEMA(%f, %f, %f, %f) = %s, should contain %q",
				tt.price, tt.ema9, tt.ema21, tt.ema50, result, tt.contains)
		}
	}
}

func TestInterpretPriceVsVWAP(t *testing.T) {
	tests := []struct {
		price, vwap float64
		contains    string
	}{
		{0, 100, "No VWAP data"},
		{100, 0, "No VWAP data"},
		{105, 100, "Above VWAP"},
		{95, 100, "Below VWAP"},
		{100, 100, "At VWAP"},
	}

	for _, tt := range tests {
		result := InterpretPriceVsVWAP(tt.price, tt.vwap)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretPriceVsVWAP(%f, %f) = %s, should contain %q",
				tt.price, tt.vwap, result, tt.contains)
		}
	}
}

func TestInterpretVolume(t *testing.T) {
	tests := []struct {
		current, avg float64
		contains     string
	}{
		{0, 100, "No volume data"},
		{100, 0, "no baseline"},
		{350, 100, "Extremely high"},
		{220, 100, "Very high"},
		{160, 100, "Above average"},
		{100, 100, "Normal"},
		{60, 100, "Below average"},
		{30, 100, "Very low"},
	}

	for _, tt := range tests {
		result := InterpretVolume(tt.current, tt.avg)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretVolume(%f, %f) = %s, should contain %q",
				tt.current, tt.avg, result, tt.contains)
		}
	}
}

func TestInterpretATR(t *testing.T) {
	tests := []struct {
		atr, atrAvg float64
		contains    string
	}{
		{0, 100, "No data"},
		{100, 0, "no baseline"},
		{250, 100, "Extremely high volatility"},
		{160, 100, "High volatility"},
		{110, 100, "Normal volatility"},
		{60, 100, "Low volatility"},
		{30, 100, "Very low volatility"},
	}

	for _, tt := range tests {
		result := InterpretATR(tt.atr, tt.atrAvg)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretATR(%f, %f) = %s, should contain %q",
				tt.atr, tt.atrAvg, result, tt.contains)
		}
	}
}

func TestInterpretMarketStructure(t *testing.T) {
	tests := []struct {
		name     string
		highs    []float64
		lows     []float64
		contains string
	}{
		{
			name:     "insufficient data",
			highs:    []float64{100},
			lows:     []float64{90},
			contains: "Insufficient data",
		},
		{
			name:     "uptrend HH/HL",
			highs:    []float64{100, 105, 110, 115, 120},
			lows:     []float64{90, 95, 100, 105, 110},
			contains: "Uptrend",
		},
		{
			name:     "downtrend LH/LL",
			highs:    []float64{120, 115, 110, 105, 100},
			lows:     []float64{110, 105, 100, 95, 90},
			contains: "Downtrend",
		},
		{
			name:     "mixed/consolidation",
			highs:    []float64{100, 105, 103, 107, 104},
			lows:     []float64{90, 92, 89, 93, 91},
			contains: "Mixed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InterpretMarketStructure(tt.highs, tt.lows)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("InterpretMarketStructure() = %s, should contain %q", result, tt.contains)
			}
		})
	}
}

func TestInterpretTrendAlignment(t *testing.T) {
	tests := []struct {
		trend1H, trend15M TrendDirection
		contains          string
	}{
		{"", "", "No trend data"},
		{TrendBullish, TrendBullish, "Aligned BULLISH"},
		{TrendBearish, TrendBearish, "Aligned BEARISH"},
		{TrendNeutral, TrendNeutral, "neutral"},
		{TrendBullish, TrendNeutral, "Partial"},
		{TrendBullish, TrendBearish, "Divergent"},
	}

	for _, tt := range tests {
		result := InterpretTrendAlignment(tt.trend1H, tt.trend15M)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("InterpretTrendAlignment(%s, %s) = %s, should contain %q",
				tt.trend1H, tt.trend15M, result, tt.contains)
		}
	}
}

// ========== LLM Formatting Tests ==========

func TestFormatForLLM(t *testing.T) {
	state := &CoinState{
		Symbol:         "BTCUSDT",
		Price:          45000.0,
		ADX:            28.5,
		RSI:            55.0,
		EMA9:           44900.0,
		EMA21:          44700.0,
		Trend1H:        TrendBullish,
		Trend15M:       TrendBullish,
		Regime:         RegimeTrending,
		ActiveStrategy: "trend_following",
		ScoreFinal:     72,
		ScoreTechnical: 80,
		ScoreContext:   65,
	}

	ctx := BuildLLMContext(state)
	output := ctx.FormatForLLM()

	// Check that output contains expected elements
	checks := []string{
		"BTCUSDT Analysis",
		"Market Regime: TRENDING",
		"ADX:",
		"Trend:",
		"BULLISH",
		"Aligned",
		"Price Position:",
		"45000",
		"RSI:",
		"Score:",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("FormatForLLM() output missing %q\nGot:\n%s", check, output)
		}
	}
}

func TestFormatForLLM_NilContext(t *testing.T) {
	var ctx *LLMContext
	output := ctx.FormatForLLM()
	if output != "" {
		t.Errorf("FormatForLLM() on nil should return empty, got %q", output)
	}
}

func TestFormatForLLMCompact(t *testing.T) {
	state := &CoinState{
		Symbol:     "BTCUSDT",
		ADX:        30.0,
		RSI:        55.0,
		Trend1H:    TrendBullish,
		Trend15M:   TrendBullish,
		Regime:     RegimeTrending,
		ScoreFinal: 75,
	}

	ctx := BuildLLMContext(state)
	output := ctx.FormatForLLMCompact()

	// Check format
	if !strings.HasPrefix(output, "BTCUSDT:") {
		t.Errorf("FormatForLLMCompact() should start with symbol, got %s", output)
	}

	// Should contain key metrics separated by |
	if !strings.Contains(output, "|") {
		t.Error("FormatForLLMCompact() should use | separator")
	}

	if !strings.Contains(output, "Regime=TRENDING") {
		t.Errorf("FormatForLLMCompact() missing regime, got %s", output)
	}
}

func TestFormatForLLMCompact_NilContext(t *testing.T) {
	var ctx *LLMContext
	output := ctx.FormatForLLMCompact()
	if output != "" {
		t.Errorf("FormatForLLMCompact() on nil should return empty, got %q", output)
	}
}

func TestFormatForLLMJSON(t *testing.T) {
	state := &CoinState{
		Symbol:  "BTCUSDT",
		Price:   45000.0,
		ADX:     28.5,
		RSI:     55.0,
		Trend1H: TrendBullish,
		Regime:  RegimeTrending,
	}

	ctx := BuildLLMContext(state)
	output := ctx.FormatForLLMJSON()

	// Check JSON-like structure
	if !strings.HasPrefix(output, "{") || !strings.HasSuffix(output, "}") {
		t.Errorf("FormatForLLMJSON() should be wrapped in braces, got %s", output)
	}

	checks := []string{
		`"symbol": "BTCUSDT"`,
		`"regime": "TRENDING"`,
		`"adx":`,
		`"rsi":`,
		`"trend_1h":`,
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("FormatForLLMJSON() missing %q\nGot:\n%s", check, output)
		}
	}
}

func TestFormatForLLMJSON_NilContext(t *testing.T) {
	var ctx *LLMContext
	output := ctx.FormatForLLMJSON()
	if output != "{}" {
		t.Errorf("FormatForLLMJSON() on nil should return {}, got %q", output)
	}
}

func TestToPromptSection(t *testing.T) {
	state := &CoinState{
		Symbol:  "BTCUSDT",
		Price:   45000.0,
		Trend1H: TrendBullish,
	}

	ctx := BuildLLMContext(state)
	output := ctx.ToPromptSection()

	// Check section markers
	if !strings.Contains(output, "=== MARKET ANALYSIS ===") {
		t.Error("ToPromptSection() missing start marker")
	}
	if !strings.Contains(output, "=== END ANALYSIS ===") {
		t.Error("ToPromptSection() missing end marker")
	}
}

func TestToPromptSection_NilContext(t *testing.T) {
	var ctx *LLMContext
	output := ctx.ToPromptSection()
	if output != "" {
		t.Errorf("ToPromptSection() on nil should return empty, got %q", output)
	}
}

// ========== Directional Bias Tests ==========

func TestGetDirectionalBias(t *testing.T) {
	tests := []struct {
		name           string
		ctx            *LLMContext
		expectedBias   string
		minConfidence  float64
	}{
		{
			name:         "nil context",
			ctx:          nil,
			expectedBias: "NEUTRAL",
		},
		{
			name: "bullish alignment",
			ctx: &LLMContext{
				TrendAlignment:  true,
				Trend1H:         TrendBullish,
				RSI:             55,
				MarketStructure: "HH/HL (Uptrend confirmed)",
				BTCTrend:        "BULLISH",
			},
			expectedBias:  "BULLISH",
			minConfidence: 0.2,
		},
		{
			name: "bearish alignment",
			ctx: &LLMContext{
				TrendAlignment:  true,
				Trend1H:         TrendBearish,
				RSI:             45,
				MarketStructure: "LH/LL (Downtrend confirmed)",
				BTCTrend:        "BEARISH",
			},
			expectedBias:  "BEARISH",
			minConfidence: 0.2,
		},
		{
			name: "neutral mixed signals",
			ctx: &LLMContext{
				TrendAlignment:  false,
				RSI:             50,
				MarketStructure: "Mixed structure",
			},
			expectedBias: "NEUTRAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bias, confidence := tt.ctx.GetDirectionalBias()
			if bias != tt.expectedBias {
				t.Errorf("GetDirectionalBias() bias = %s, want %s", bias, tt.expectedBias)
			}
			if tt.minConfidence > 0 && confidence < tt.minConfidence {
				t.Errorf("GetDirectionalBias() confidence = %f, want >= %f", confidence, tt.minConfidence)
			}
		})
	}
}

// ========== Regime Confidence Tests ==========

func TestCalculateRegimeConfidence(t *testing.T) {
	tests := []struct {
		name          string
		state         *CoinState
		minConfidence float64
		maxConfidence float64
	}{
		{
			name:          "nil state",
			state:         nil,
			minConfidence: 0,
			maxConfidence: 0,
		},
		{
			name: "clear signals",
			state: &CoinState{
				ADX:        35,
				ATR:        100,
				ATRAverage: 50,
				Trend1H:    TrendBullish,
				Trend15M:   TrendBullish,
			},
			minConfidence: 0.8,
			maxConfidence: 1.0,
		},
		{
			name: "unclear signals",
			state: &CoinState{
				ADX:        22,
				ATR:        100,
				ATRAverage: 100,
				Trend1H:    TrendNeutral,
				Trend15M:   TrendBullish,
			},
			minConfidence: 0.4,
			maxConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateRegimeConfidence(tt.state)
			if confidence < tt.minConfidence || confidence > tt.maxConfidence {
				t.Errorf("calculateRegimeConfidence() = %f, want between %f and %f",
					confidence, tt.minConfidence, tt.maxConfidence)
			}
		})
	}
}

// ========== Extended Context Tests ==========

func TestBuildLLMContextExtended_NilExtended(t *testing.T) {
	state := &CoinState{
		Symbol: "BTCUSDT",
		Price:  45000.0,
	}

	ctx := BuildLLMContextExtended(state, nil)
	if ctx == nil {
		t.Error("BuildLLMContextExtended with nil ext should still return basic context")
	}

	// Should still have basic fields
	if ctx.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %s, want BTCUSDT", ctx.Symbol)
	}
}

func TestBuildLLMContextExtended_NilState(t *testing.T) {
	ext := &ExtendedMarketData{
		VWAPValue: 45000.0,
	}

	ctx := BuildLLMContextExtended(nil, ext)
	if ctx != nil {
		t.Error("BuildLLMContextExtended with nil state should return nil")
	}
}

// ========== Output Format Integration Tests ==========

func TestFormatForLLM_WithBlocking(t *testing.T) {
	state := &CoinState{
		Symbol:          "BTCUSDT",
		Price:           45000.0,
		Trend1H:         TrendBullish,
		Trend15M:        TrendBearish,
		Decision:        DecisionBlocked,
		BlockingReasons: []string{"Trend divergence", "Low ADX"},
	}

	ctx := BuildLLMContext(state)
	output := ctx.FormatForLLM()

	if !strings.Contains(output, "BLOCKED") {
		t.Error("FormatForLLM() should show BLOCKED status")
	}
	if !strings.Contains(output, "Trend divergence") {
		t.Error("FormatForLLM() should show blocking reasons")
	}
}

func TestFormatForLLM_WithSupportResistance(t *testing.T) {
	state := &CoinState{
		Symbol: "BTCUSDT",
		Price:  45000.0,
	}

	ext := &ExtendedMarketData{
		NearestSupport:    44000.0,
		NearestResistance: 46000.0,
	}

	ctx := BuildLLMContextExtended(state, ext)
	output := ctx.FormatForLLM()

	if !strings.Contains(output, "Support:") {
		t.Error("FormatForLLM() should show support level")
	}
	if !strings.Contains(output, "Resistance:") {
		t.Error("FormatForLLM() should show resistance level")
	}
	if !strings.Contains(output, "44000") {
		t.Errorf("FormatForLLM() should show support value\nGot:\n%s", output)
	}
}

func TestFormatForLLMCompact_WithBlocking(t *testing.T) {
	state := &CoinState{
		Symbol:          "BTCUSDT",
		Decision:        DecisionBlocked,
		BlockingReasons: []string{"Test block"},
	}

	ctx := BuildLLMContext(state)
	output := ctx.FormatForLLMCompact()

	if !strings.Contains(output, "BLOCKED") {
		t.Error("FormatForLLMCompact() should indicate blocking")
	}
}

// ========== Edge Cases ==========

func TestInterpretPriceVsEMA_PartialData(t *testing.T) {
	// Only EMA9 available
	result := InterpretPriceVsEMA(100, 90, 0, 0)
	if !strings.Contains(result, "Above EMA9") {
		t.Errorf("InterpretPriceVsEMA with only EMA9 = %s", result)
	}

	// Only EMA21 available
	result = InterpretPriceVsEMA(80, 0, 90, 0)
	if !strings.Contains(result, "Below EMA21") {
		t.Errorf("InterpretPriceVsEMA with only EMA21 = %s", result)
	}
}

func TestInterpretMarketStructure_ExcessiveData(t *testing.T) {
	// More than 5 points should use last 5
	highs := []float64{100, 102, 104, 106, 108, 110, 112, 114}
	lows := []float64{95, 97, 99, 101, 103, 105, 107, 109}

	result := InterpretMarketStructure(highs, lows)
	if result == "Insufficient data" {
		t.Error("Should handle data with more than 5 points")
	}
}

func TestCalculateRegimeConfidence_BoundaryConditions(t *testing.T) {
	// High ADX, aligned trends, clear ATR
	state := &CoinState{
		ADX:        50,
		Trend1H:    TrendBullish,
		Trend15M:   TrendBullish,
		ATR:        200,
		ATRAverage: 100,
	}

	confidence := calculateRegimeConfidence(state)
	if confidence > 1.0 {
		t.Errorf("Confidence should not exceed 1.0, got %f", confidence)
	}
	if confidence != 1.0 {
		t.Errorf("With all clear signals, confidence should be 1.0, got %f", confidence)
	}
}

// ========== Additional Edge Case Tests (Code Review Additions) ==========

func TestBuildLLMContext_EmptyTrendsNotAligned(t *testing.T) {
	// Both trends empty should NOT be considered aligned
	state := &CoinState{
		Symbol:  "BTCUSDT",
		Price:   45000.0,
		Trend1H: "",
		Trend15M: "",
	}

	ctx := BuildLLMContext(state)

	if ctx.TrendAlignment {
		t.Error("Empty trends should NOT be considered aligned")
	}
}

func TestGetDirectionalBias_RSIEdgeValues(t *testing.T) {
	tests := []struct {
		name string
		rsi  float64
	}{
		{"RSI exactly at 30", 30},
		{"RSI exactly at 50", 50},
		{"RSI exactly at 70", 70},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &LLMContext{
				RSI: tt.rsi,
			}
			// Should not panic and should return valid result
			bias, _ := ctx.GetDirectionalBias()
			if bias != "BULLISH" && bias != "BEARISH" && bias != "NEUTRAL" {
				t.Errorf("GetDirectionalBias() returned invalid bias: %s", bias)
			}
		})
	}
}

func TestGetDirectionalBias_PartialMarketStructure(t *testing.T) {
	ctx := &LLMContext{
		MarketStructure: "Mixed structure (Consolidation/Ranging)",
	}

	bias, _ := ctx.GetDirectionalBias()
	if bias != "NEUTRAL" {
		t.Errorf("Mixed market structure should result in NEUTRAL bias, got %s", bias)
	}
}

func TestInterpretMarketStructure_EqualConsecutiveValues(t *testing.T) {
	// Test flat/sideways market with equal consecutive values
	highs := []float64{100, 100, 100, 100, 100}
	lows := []float64{90, 90, 90, 90, 90}

	result := InterpretMarketStructure(highs, lows)

	// Should indicate consolidation/ranging, not uptrend or downtrend
	if strings.Contains(result, "Uptrend") || strings.Contains(result, "Downtrend") {
		t.Errorf("Equal values should not indicate trend, got: %s", result)
	}
	if !strings.Contains(result, "Mixed") && !strings.Contains(result, "Consolidation") {
		t.Errorf("Equal values should indicate mixed/consolidation, got: %s", result)
	}
}

func TestFormatForLLMJSON_ValidJSON(t *testing.T) {
	state := &CoinState{
		Symbol:  "BTCUSDT",
		Price:   45000.0,
		ADX:     28.5,
		RSI:     55.0,
		Trend1H: TrendBullish,
		Regime:  RegimeTrending,
	}

	ctx := BuildLLMContext(state)
	output := ctx.FormatForLLMJSON()

	// Verify it's valid JSON by attempting to parse
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("FormatForLLMJSON() produced invalid JSON: %v\nOutput:\n%s", err, output)
	}

	// Verify expected fields exist
	if _, ok := parsed["symbol"]; !ok {
		t.Error("FormatForLLMJSON() missing 'symbol' field")
	}
	if _, ok := parsed["regime"]; !ok {
		t.Error("FormatForLLMJSON() missing 'regime' field")
	}
}

func TestFormatForLLMJSON_SpecialCharacters(t *testing.T) {
	// Test that special characters are properly escaped
	ctx := &LLMContext{
		Symbol:            "BTC\"USDT",  // Contains quote
		ADXInterpretation: "Strong \"trend\" (test)",
		PriceVsEMAs:       "Above\nall\tEMAs",  // Contains newline and tab
	}

	output := ctx.FormatForLLMJSON()

	// Verify it's valid JSON despite special characters
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Errorf("FormatForLLMJSON() failed with special characters: %v\nOutput:\n%s", err, output)
	}
}

func TestBuildLLMContext_BlockingReasonsCopied(t *testing.T) {
	// Verify that BlockingReasons are copied, not referenced
	state := &CoinState{
		Symbol:          "BTCUSDT",
		BlockingReasons: []string{"Reason1", "Reason2"},
		Decision:        DecisionBlocked,
	}

	ctx := BuildLLMContext(state)

	// Modify original state's blocking reasons
	state.BlockingReasons[0] = "Modified"

	// Context should still have original value
	if ctx.BlockingReasons[0] == "Modified" {
		t.Error("BlockingReasons should be copied, not referenced")
	}
	if ctx.BlockingReasons[0] != "Reason1" {
		t.Errorf("BlockingReasons[0] = %s, want Reason1", ctx.BlockingReasons[0])
	}
}

func TestBuildLLMContextExtended_NegativeValues(t *testing.T) {
	state := &CoinState{
		Symbol: "BTCUSDT",
		Price:  45000.0,
	}

	ext := &ExtendedMarketData{
		NearestSupport:    -100.0,  // Invalid negative value
		NearestResistance: -50.0,   // Invalid negative value
	}

	ctx := BuildLLMContextExtended(state, ext)

	// Negative values should not be set
	if ctx.NearestSupport != 0 {
		t.Errorf("Negative support should not be set, got %f", ctx.NearestSupport)
	}
	if ctx.NearestResistance != 0 {
		t.Errorf("Negative resistance should not be set, got %f", ctx.NearestResistance)
	}
}
