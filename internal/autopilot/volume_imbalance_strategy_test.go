package autopilot

import (
	"testing"
	"time"

	"binance-trading-bot/internal/binance"
)

// ============================================================================
// VOLUME IMBALANCE STRATEGY TESTS - 3-STEP MODEL
// ============================================================================

// TestDefaultVolumeImbalanceConfig verifies default configuration values
func TestDefaultVolumeImbalanceConfig(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()

	// Step 1: Accumulation Start defaults
	if config.MinVolumeSpikeMultiplier != 2.0 {
		t.Errorf("Expected MinVolumeSpikeMultiplier 2.0, got %f", config.MinVolumeSpikeMultiplier)
	}
	if config.LookbackPeriod != 20 {
		t.Errorf("Expected LookbackPeriod 20, got %d", config.LookbackPeriod)
	}

	// Step 2: Consolidation defaults
	if config.MinConsolidationCandles != 2 {
		t.Errorf("Expected MinConsolidationCandles 2, got %d", config.MinConsolidationCandles)
	}
	if config.MaxConsolidationCandles != 6 {
		t.Errorf("Expected MaxConsolidationCandles 6, got %d", config.MaxConsolidationCandles)
	}

	// Step 3: Breakout defaults
	if config.BreakoutVolumeSurge != 1.5 {
		t.Errorf("Expected BreakoutVolumeSurge 1.5, got %f", config.BreakoutVolumeSurge)
	}

	// Risk/Reward defaults
	if config.DefaultRiskRewardRatio != 4.0 {
		t.Errorf("Expected DefaultRiskRewardRatio 4.0, got %f", config.DefaultRiskRewardRatio)
	}

	// Trailing stop milestone defaults
	if config.BreakevenRRLevel != 2.0 {
		t.Errorf("Expected BreakevenRRLevel 2.0, got %f", config.BreakevenRRLevel)
	}
	if config.OneRRLevel != 3.0 {
		t.Errorf("Expected OneRRLevel 3.0, got %f", config.OneRRLevel)
	}

	// Timeframe defaults (VALIDATED BY BACKTESTING)
	if config.ScalpTimeframe != "15m" {
		t.Errorf("Expected ScalpTimeframe '15m', got '%s'", config.ScalpTimeframe)
	}
	if config.SwingTimeframe != "1h" {
		t.Errorf("Expected SwingTimeframe '1h', got '%s'", config.SwingTimeframe)
	}
	if config.PositionTimeframe != "4h" {
		t.Errorf("Expected PositionTimeframe '4h', got '%s'", config.PositionTimeframe)
	}
}

// TestNewVolumeImbalanceDetector verifies detector creation
func TestNewVolumeImbalanceDetector(t *testing.T) {
	// Test with nil config (should use defaults)
	detector := NewVolumeImbalanceDetector(nil, nil, nil)
	if detector == nil {
		t.Error("Expected non-nil detector")
	}
	if detector.config == nil {
		t.Error("Expected config to be initialized with defaults")
	}
	if detector.patterns == nil {
		t.Error("Expected patterns map to be initialized")
	}

	// Test with custom config
	customConfig := &VolumeImbalanceConfig{
		Enabled:                  true,
		MinVolumeSpikeMultiplier: 3.0,
	}
	detector2 := NewVolumeImbalanceDetector(nil, customConfig, nil)
	if detector2.config.MinVolumeSpikeMultiplier != 3.0 {
		t.Errorf("Expected custom config to be used")
	}
}

// TestGetTimeframe verifies timeframe selection per mode
func TestGetTimeframe(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	tests := []struct {
		mode     GinieTradingMode
		expected string
	}{
		{GinieModeScalp, "15m"},
		{GinieModeSwing, "1h"},
		{GinieModePosition, "4h"},
		{"unknown_mode", "15m"}, // Default to scalp
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			result := detector.GetTimeframe(tt.mode)
			if result != tt.expected {
				t.Errorf("For mode %s, expected timeframe '%s', got '%s'", tt.mode, tt.expected, result)
			}
		})
	}
}

// TestDetectAccumulationStart tests Step 1: Volume spike detection
func TestDetectAccumulationStart(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	baseTime := time.Now()

	tests := []struct {
		name          string
		candles       []VolumeImbalanceCandle
		expectFound   bool
		description   string
	}{
		{
			name: "Clear volume spike detected",
			candles: generateCandlesWithVolumeSpike(baseTime, 25, 100000, 250000, 15),
			expectFound: true,
			description: "Volume spike at 2.5x average should be detected",
		},
		{
			name: "No volume spike",
			candles: generateUniformCandles(baseTime, 25, 100000),
			expectFound: false,
			description: "Uniform volume should not trigger detection",
		},
		{
			name: "Insufficient candles",
			candles: generateUniformCandles(baseTime, 10, 100000),
			expectFound: false,
			description: "Less than lookback period should return nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, idx := detector.detectAccumulationStart(tt.candles)
			if tt.expectFound && result == nil {
				t.Errorf("%s: expected volume spike to be found, got nil", tt.description)
			}
			if tt.expectFound && idx < 0 {
				t.Errorf("%s: expected valid index, got %d", tt.description, idx)
			}
			if !tt.expectFound && result != nil {
				t.Errorf("%s: expected no volume spike, got candle with volume %f", tt.description, result.Volume)
			}
		})
	}
}

// TestIsConsolidating tests Step 2: Sideways consolidation detection
func TestIsConsolidating(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	baseTime := time.Now()

	// Create a pattern with reference candle set
	pattern := &VolumeImbalancePattern{
		Symbol: "BTCUSDT",
		Mode:   GinieModeScalp,
		State:  VIStateConsolidating,
	}
	pattern.ReferenceCandle.Index = 0
	pattern.ReferenceCandle.Volume = 200000
	pattern.ReferenceCandle.High = 100.0
	pattern.ReferenceCandle.Low = 98.0
	pattern.ReferenceCandle.Close = 99.5

	tests := []struct {
		name           string
		candles        []VolumeImbalanceCandle
		expectConsolidating bool
		description    string
	}{
		{
			name: "Valid consolidation pattern",
			candles: generateConsolidationCandles(baseTime, pattern.ReferenceCandle.High, pattern.ReferenceCandle.Low, 200000, 5),
			expectConsolidating: true,
			description: "Declining volume with sideways price should be consolidating",
		},
		{
			name: "Price breaks above reference high",
			candles: generateBreakoutCandles(baseTime, pattern.ReferenceCandle.High, pattern.ReferenceCandle.Low, 200000, 5, true),
			expectConsolidating: false,
			description: "Price breaking above reference high invalidates consolidation",
		},
		{
			name: "Insufficient candles for consolidation",
			candles: generateConsolidationCandles(baseTime, pattern.ReferenceCandle.High, pattern.ReferenceCandle.Low, 200000, 1),
			expectConsolidating: false,
			description: "Less than min consolidation candles",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset pattern index for test
			pattern.ReferenceCandle.Index = 0
			result := detector.isConsolidating(pattern, tt.candles)
			if result != tt.expectConsolidating {
				t.Errorf("%s: expected consolidating=%v, got %v", tt.description, tt.expectConsolidating, result)
			}
		})
	}
}

// TestIsBreakoutReady tests Step 3: Breakout detection
func TestIsBreakoutReady(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	baseTime := time.Now()

	// Create pattern with consolidation data
	pattern := &VolumeImbalancePattern{
		Symbol:                 "BTCUSDT",
		Mode:                   GinieModeScalp,
		State:                  VIStateConsolidating,
		ConsolidationAvgVolume: 50000,
	}
	pattern.ReferenceCandle.High = 100.0
	pattern.ReferenceCandle.Low = 98.0

	tests := []struct {
		name          string
		currentCandle VolumeImbalanceCandle
		expectReady   bool
		description   string
	}{
		{
			name: "Valid breakout",
			currentCandle: VolumeImbalanceCandle{
				Time:   baseTime,
				Open:   100.0,
				High:   101.5,
				Low:    99.5,
				Close:  101.0,
				Volume: 100000, // 2x consolidation avg (above 1.5x threshold)
			},
			expectReady: true,
			description: "High volume + price above reference high = breakout",
		},
		{
			name: "Insufficient volume",
			currentCandle: VolumeImbalanceCandle{
				Time:   baseTime,
				Open:   100.0,
				High:   101.5,
				Low:    99.5,
				Close:  101.0,
				Volume: 60000, // Only 1.2x (below 1.5x threshold)
			},
			expectReady: false,
			description: "Volume surge below threshold",
		},
		{
			name: "Price not breaking reference high",
			currentCandle: VolumeImbalanceCandle{
				Time:   baseTime,
				Open:   98.5,
				High:   99.5,
				Low:    98.0,
				Close:  99.0,
				Volume: 100000, // High volume but no price breakout
			},
			expectReady: false,
			description: "High volume but price below reference high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candles := []VolumeImbalanceCandle{{}, tt.currentCandle} // At least 2 candles needed
			result := detector.isBreakoutReady(pattern, candles)
			if result != tt.expectReady {
				t.Errorf("%s: expected breakout ready=%v, got %v", tt.description, tt.expectReady, result)
			}
		})
	}
}

// TestCalculateRiskReward tests SL/TP calculation
func TestCalculateRiskReward(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	pattern := &VolumeImbalancePattern{
		ConsolidationLow: 95.0, // SL reference
	}

	entryPrice := 100.0

	sl, tp, rr := detector.CalculateRiskReward(pattern, entryPrice)

	// Expected SL: 95.0 * (1 - 0.001) = 94.905
	expectedSL := 94.905
	if viAbsFloat(sl-expectedSL) > 0.01 {
		t.Errorf("Expected SL ~%.3f, got %.3f", expectedSL, sl)
	}

	// Risk = 100 - 94.905 = 5.095
	// TP = 100 + (5.095 * 4) = 120.38
	expectedTP := entryPrice + ((entryPrice - expectedSL) * 4.0)
	if viAbsFloat(tp-expectedTP) > 0.1 {
		t.Errorf("Expected TP ~%.2f, got %.2f", expectedTP, tp)
	}

	// R:R should be 4.0
	if rr != 4.0 {
		t.Errorf("Expected R:R 4.0, got %f", rr)
	}
}

// ============================================================================
// TRAILING STOP MANAGER TESTS
// ============================================================================

// TestTrailingStopManagerMilestones tests the trailing stop milestone logic
func TestTrailingStopManagerMilestones(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()

	// Entry at 100, SL at 98, TP at 108 (risk = 2, target = 1:4 R:R)
	entry := 100.0
	sl := 98.0
	tp := 108.0
	risk := entry - sl // 2.0

	tsm := NewTrailingStopManager(entry, sl, tp, config)

	// Verify initial state
	if tsm.RiskAmount != risk {
		t.Errorf("Expected risk amount %f, got %f", risk, tsm.RiskAmount)
	}

	// Test price scenarios
	tests := []struct {
		name           string
		price          float64
		expectedAction string
		expectedSL     float64
		description    string
	}{
		{
			name:           "Price at entry - no change",
			price:          100.0,
			expectedAction: "HOLD",
			expectedSL:     98.0,
			description:    "At entry, SL should remain at original",
		},
		{
			name:           "Price at 1:1 - no change yet",
			price:          102.0, // 1:1 R:R achieved
			expectedAction: "HOLD",
			expectedSL:     98.0,
			description:    "At 1:1, SL should not move yet",
		},
		{
			name:           "Price at 1:2 - move to breakeven",
			price:          104.0, // 1:2 R:R achieved
			expectedAction: "MOVE_TO_BREAKEVEN",
			expectedSL:     100.0, // Entry price
			description:    "At 1:2, SL should move to entry (breakeven)",
		},
		{
			name:           "Price at 1:3 - move to 1:1",
			price:          106.0, // 1:3 R:R achieved
			expectedAction: "MOVE_TO_1R",
			expectedSL:     102.0, // Entry + risk = 1:1 level
			description:    "At 1:3, SL should move to 1:1 profit level",
		},
		{
			name:           "Price at take profit",
			price:          108.0, // 1:4 R:R achieved (TP)
			expectedAction: "TAKE_PROFIT",
			expectedSL:     102.0, // Stays at 1:1
			description:    "At 1:4, should signal take profit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset manager for each test
			tsm = NewTrailingStopManager(entry, sl, tp, config)

			// For tests after breakeven, we need to trigger the earlier milestones first
			if tt.price >= 104.0 && tt.expectedAction != "MOVE_TO_BREAKEVEN" {
				// Trigger breakeven first
				tsm.Update(104.0)
			}
			if tt.price >= 106.0 && tt.expectedAction == "TAKE_PROFIT" {
				// Trigger 1:1 move
				tsm.Update(106.0)
			}

			newSL, action := tsm.Update(tt.price)

			if action != tt.expectedAction {
				t.Errorf("%s: expected action '%s', got '%s'", tt.description, tt.expectedAction, action)
			}
			if viAbsFloat(newSL-tt.expectedSL) > 0.01 {
				t.Errorf("%s: expected SL %.2f, got %.2f", tt.description, tt.expectedSL, newSL)
			}
		})
	}
}

// TestTrailingStopManagerStatus tests the GetStatus method
func TestTrailingStopManagerStatus(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	tsm := NewTrailingStopManager(100.0, 98.0, 108.0, config)

	// Move price to trigger breakeven
	tsm.Update(104.0)

	status := tsm.GetStatus()

	if status.EntryPrice != 100.0 {
		t.Errorf("Expected entry price 100.0, got %f", status.EntryPrice)
	}
	if status.CurrentStopLoss != 100.0 { // Should be at breakeven
		t.Errorf("Expected current SL 100.0 (breakeven), got %f", status.CurrentStopLoss)
	}
	if !status.AtBreakeven {
		t.Error("Expected AtBreakeven to be true")
	}
	if status.At1R {
		t.Error("Expected At1R to be false")
	}
}

// ============================================================================
// PATTERN LIFECYCLE TESTS
// ============================================================================

// TestPatternLifecycle tests the complete pattern detection lifecycle
func TestPatternLifecycle(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	// Create klines that should trigger the full pattern
	klines := generateFullPatternKlines()

	// Run analysis
	analysis, err := detector.AnalyzeForVolumeImbalance("BTCUSDT", GinieModeScalp, klines)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	if analysis == nil {
		t.Fatal("Expected analysis result, got nil")
	}

	// Verify pattern was created
	pattern := detector.GetPattern("BTCUSDT")
	if pattern == nil {
		t.Fatal("Expected pattern to be stored")
	}
}

// TestClearPattern tests pattern cleanup
func TestClearPattern(t *testing.T) {
	detector := NewVolumeImbalanceDetector(nil, nil, nil)

	// Add a pattern
	detector.patterns["BTCUSDT"] = &VolumeImbalancePattern{Symbol: "BTCUSDT"}

	// Verify it exists
	if detector.GetPattern("BTCUSDT") == nil {
		t.Fatal("Pattern should exist before clear")
	}

	// Clear it
	detector.ClearPattern("BTCUSDT")

	// Verify it's gone
	if detector.GetPattern("BTCUSDT") != nil {
		t.Error("Pattern should be nil after clear")
	}
}

// TestCleanupExpiredPatterns tests automatic cleanup of expired patterns
func TestCleanupExpiredPatterns(t *testing.T) {
	detector := NewVolumeImbalanceDetector(nil, nil, nil)

	// Add expired pattern
	detector.patterns["EXPIRED"] = &VolumeImbalancePattern{
		Symbol:    "EXPIRED",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	// Add valid pattern
	detector.patterns["VALID"] = &VolumeImbalancePattern{
		Symbol:    "VALID",
		ExpiresAt: time.Now().Add(1 * time.Hour), // Expires in 1 hour
	}

	removed := detector.CleanupExpiredPatterns()

	if removed != 1 {
		t.Errorf("Expected 1 pattern removed, got %d", removed)
	}
	if detector.GetPattern("EXPIRED") != nil {
		t.Error("Expired pattern should be removed")
	}
	if detector.GetPattern("VALID") == nil {
		t.Error("Valid pattern should still exist")
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func viAbsFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// generateCandlesWithVolumeSpike creates candles with a volume spike at specified index
func generateCandlesWithVolumeSpike(baseTime time.Time, count int, baseVolume, spikeVolume float64, spikeIndex int) []VolumeImbalanceCandle {
	candles := make([]VolumeImbalanceCandle, count)
	for i := 0; i < count; i++ {
		volume := baseVolume
		if i == spikeIndex {
			volume = spikeVolume
		}
		candles[i] = VolumeImbalanceCandle{
			Time:           baseTime.Add(time.Duration(i) * 15 * time.Minute),
			Open:           100.0 + float64(i)*0.1,
			High:           100.0 + float64(i)*0.1 + 0.5,
			Low:            100.0 + float64(i)*0.1 - 0.5,
			Close:          100.0 + float64(i)*0.1 + 0.2,
			Volume:         volume,
			TakerBuyVolume: volume * 0.6,
		}
	}
	return candles
}

// generateUniformCandles creates candles with uniform volume
func generateUniformCandles(baseTime time.Time, count int, volume float64) []VolumeImbalanceCandle {
	candles := make([]VolumeImbalanceCandle, count)
	for i := 0; i < count; i++ {
		candles[i] = VolumeImbalanceCandle{
			Time:           baseTime.Add(time.Duration(i) * 15 * time.Minute),
			Open:           100.0,
			High:           100.5,
			Low:            99.5,
			Close:          100.2,
			Volume:         volume,
			TakerBuyVolume: volume * 0.5,
		}
	}
	return candles
}

// generateConsolidationCandles creates candles that represent sideways consolidation with declining volume
func generateConsolidationCandles(baseTime time.Time, refHigh, refLow, startVolume float64, count int) []VolumeImbalanceCandle {
	candles := make([]VolumeImbalanceCandle, count+1) // +1 for reference candle at index 0

	// Reference candle at index 0
	candles[0] = VolumeImbalanceCandle{
		Time:           baseTime,
		Open:           refLow,
		High:           refHigh,
		Low:            refLow,
		Close:          (refHigh + refLow) / 2,
		Volume:         startVolume,
		TakerBuyVolume: startVolume * 0.6,
	}

	// Consolidation candles with declining volume
	for i := 1; i <= count; i++ {
		// Volume declines
		volume := startVolume * (1.0 - float64(i)*0.15) // Decline by 15% per candle
		if volume < startVolume*0.3 {
			volume = startVolume * 0.3
		}

		// Price stays in range (sideways)
		mid := (refHigh + refLow) / 2
		rangeSize := (refHigh - refLow) * 0.5

		candles[i] = VolumeImbalanceCandle{
			Time:           baseTime.Add(time.Duration(i) * 15 * time.Minute),
			Open:           mid - rangeSize/4,
			High:           mid + rangeSize/2,
			Low:            mid - rangeSize/2,
			Close:          mid + rangeSize/4,
			Volume:         volume,
			TakerBuyVolume: volume * 0.5,
		}
	}
	return candles
}

// generateBreakoutCandles creates candles where price breaks above reference high
func generateBreakoutCandles(baseTime time.Time, refHigh, refLow, startVolume float64, count int, breakAbove bool) []VolumeImbalanceCandle {
	candles := generateConsolidationCandles(baseTime, refHigh, refLow, startVolume, count-1)

	// Last candle breaks out
	lastIdx := len(candles) - 1
	if breakAbove {
		candles[lastIdx].High = refHigh * 1.05 // 5% above reference high
		candles[lastIdx].Close = refHigh * 1.03
	} else {
		candles[lastIdx].Low = refLow * 0.95 // 5% below reference low
		candles[lastIdx].Close = refLow * 0.97
	}

	return candles
}

// generateFullPatternKlines creates klines that should trigger a complete pattern
func generateFullPatternKlines() []binance.Kline {
	baseTime := time.Now().Add(-2 * time.Hour)
	count := 30
	klines := make([]binance.Kline, count)

	for i := 0; i < count; i++ {
		t := baseTime.Add(time.Duration(i) * 15 * time.Minute)

		// Default values
		open := 100.0
		high := 100.5
		low := 99.5
		close := 100.2
		volume := 100000.0

		// Step 1: Volume spike at candle 10
		if i == 10 {
			volume = 300000.0 // 3x average
			high = 102.0
			close = 101.5
		}

		// Step 2: Consolidation (candles 11-15) with declining volume
		if i > 10 && i <= 15 {
			volume = 100000.0 * (1.0 - float64(i-10)*0.15) // Declining
			high = 101.0
			low = 99.5
			close = 100.0
		}

		// Step 3: Breakout at candle 16
		if i >= 16 {
			if i == 16 {
				volume = 200000.0 // Volume surge
				high = 103.0
				close = 102.5
			}
		}

		klines[i] = binance.Kline{
			OpenTime:              t.UnixMilli(),
			Open:                  open,
			High:                  high,
			Low:                   low,
			Close:                 close,
			Volume:                volume,
			TakerBuyBaseAssetVolume: volume * 0.6,
		}
	}

	return klines
}

// TestCalculateTrend tests the linear regression trend calculation
func TestCalculateTrend(t *testing.T) {
	detector := NewVolumeImbalanceDetector(nil, nil, nil)

	tests := []struct {
		name       string
		values     []float64
		wantNegative bool
		description string
	}{
		{
			name:       "Declining trend",
			values:     []float64{100, 80, 60, 40, 20},
			wantNegative: true,
			description: "Decreasing values should have negative slope",
		},
		{
			name:       "Increasing trend",
			values:     []float64{20, 40, 60, 80, 100},
			wantNegative: false,
			description: "Increasing values should have positive slope",
		},
		{
			name:       "Flat trend",
			values:     []float64{50, 50, 50, 50, 50},
			wantNegative: false,
			description: "Constant values should have zero slope",
		},
		{
			name:       "Single value",
			values:     []float64{100},
			wantNegative: false,
			description: "Single value should return 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trend := detector.calculateTrend(tt.values)
			isNegative := trend < 0

			if tt.wantNegative && !isNegative {
				t.Errorf("%s: expected negative trend, got %f", tt.description, trend)
			}
			if !tt.wantNegative && isNegative && trend != 0 {
				t.Errorf("%s: expected non-negative trend, got %f", tt.description, trend)
			}
		})
	}
}

// TestDisabledStrategy tests that disabled strategy returns nil
func TestDisabledStrategy(t *testing.T) {
	config := &VolumeImbalanceConfig{
		Enabled: false,
	}
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	klines := generateFullPatternKlines()
	analysis, err := detector.AnalyzeForVolumeImbalance("BTCUSDT", GinieModeScalp, klines)

	if err != nil {
		t.Errorf("Should not return error when disabled, got: %v", err)
	}
	if analysis != nil {
		t.Error("Should return nil analysis when disabled")
	}
}

// TestInsufficientKlines tests error handling for insufficient data
func TestInsufficientKlines(t *testing.T) {
	config := DefaultVolumeImbalanceConfig()
	detector := NewVolumeImbalanceDetector(nil, config, nil)

	// Only 5 klines (need lookback + 5)
	klines := make([]binance.Kline, 5)
	for i := 0; i < 5; i++ {
		klines[i] = binance.Kline{
			OpenTime: time.Now().UnixMilli(),
			Volume:   100000,
		}
	}

	_, err := detector.AnalyzeForVolumeImbalance("BTCUSDT", GinieModeScalp, klines)
	if err == nil {
		t.Error("Should return error for insufficient klines")
	}
}
