// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"math"
	"testing"
	"time"
)

func TestSegmentConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  SegmentConfig
		wantErr bool
	}{
		{
			name: "valid simple config",
			config: SegmentConfig{
				Segment:            SegmentTrend,
				SelectedIndicators: []string{"ema_cross", "macd"},
				AveragingMethod:    "simple",
				Enabled:            true,
			},
			wantErr: false,
		},
		{
			name: "valid weighted config",
			config: SegmentConfig{
				Segment:            SegmentMomentum,
				SelectedIndicators: []string{"rsi", "stochastic"},
				AveragingMethod:    "weighted",
				Weights: map[string]float64{
					"rsi":        0.6,
					"stochastic": 0.4,
				},
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "invalid segment",
			config: SegmentConfig{
				Segment:            IndicatorSegment("invalid"),
				SelectedIndicators: []string{"rsi"},
				AveragingMethod:    "simple",
			},
			wantErr: true,
		},
		{
			name: "no indicators selected",
			config: SegmentConfig{
				Segment:            SegmentTrend,
				SelectedIndicators: []string{},
				AveragingMethod:    "simple",
			},
			wantErr: true,
		},
		{
			name: "too many indicators",
			config: SegmentConfig{
				Segment:            SegmentTrend,
				SelectedIndicators: []string{"a", "b", "c", "d", "e"},
				AveragingMethod:    "simple",
			},
			wantErr: true,
		},
		{
			name: "invalid averaging method",
			config: SegmentConfig{
				Segment:            SegmentTrend,
				SelectedIndicators: []string{"rsi"},
				AveragingMethod:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "weighted without weights",
			config: SegmentConfig{
				Segment:            SegmentTrend,
				SelectedIndicators: []string{"rsi"},
				AveragingMethod:    "weighted",
				Weights:            nil,
			},
			wantErr: true,
		},
		{
			name: "weights don't sum to 1",
			config: SegmentConfig{
				Segment:            SegmentTrend,
				SelectedIndicators: []string{"rsi", "macd"},
				AveragingMethod:    "weighted",
				Weights: map[string]float64{
					"rsi":  0.5,
					"macd": 0.3, // Sum is 0.8, not 1.0
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SegmentConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSegmentCalculator_Calculate(t *testing.T) {
	// Create a custom registry for testing
	registry := NewIndicatorRegistry()

	// Register mock indicators
	mockTrend := NewMockIndicator("mock_trend", SegmentTrend, nil)
	mockTrend.calcValue = 80.0 // High score

	mockTrend2 := NewMockIndicator("mock_trend2", SegmentTrend, nil)
	mockTrend2.calcValue = 60.0

	registry.Register(mockTrend)
	registry.Register(mockTrend2)

	calculator := NewSegmentCalculator(registry)

	// Test calculation with simple averaging
	config := &SegmentConfig{
		Segment:            SegmentTrend,
		SelectedIndicators: []string{"mock_trend", "mock_trend2"},
		AveragingMethod:    "simple",
		Enabled:            true,
	}

	data := NewIndicatorData("BTCUSDT")
	data.Price = 50000.0

	result, err := calculator.Calculate(config, data)
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	// Simple average of 80 and 60 should be 70
	expectedScore := 70.0
	if math.Abs(result.Score-expectedScore) > 0.01 {
		t.Errorf("Expected score %.2f, got %.2f", expectedScore, result.Score)
	}

	// Check individual scores were recorded
	if len(result.IndividualScores) != 2 {
		t.Errorf("Expected 2 individual scores, got %d", len(result.IndividualScores))
	}
}

func TestSegmentCalculator_Calculate_Weighted(t *testing.T) {
	registry := NewIndicatorRegistry()

	mock1 := NewMockIndicator("mock1", SegmentMomentum, nil)
	mock1.calcValue = 100.0 // Weight 0.7

	mock2 := NewMockIndicator("mock2", SegmentMomentum, nil)
	mock2.calcValue = 0.0 // Weight 0.3

	registry.Register(mock1)
	registry.Register(mock2)

	calculator := NewSegmentCalculator(registry)

	config := &SegmentConfig{
		Segment:            SegmentMomentum,
		SelectedIndicators: []string{"mock1", "mock2"},
		AveragingMethod:    "weighted",
		Weights: map[string]float64{
			"mock1": 0.7,
			"mock2": 0.3,
		},
		Enabled: true,
	}

	data := NewIndicatorData("ETHUSDT")

	result, err := calculator.Calculate(config, data)
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	// Weighted average: 100*0.7 + 0*0.3 = 70
	expectedScore := 70.0
	if math.Abs(result.Score-expectedScore) > 0.01 {
		t.Errorf("Expected score %.2f, got %.2f", expectedScore, result.Score)
	}
}

func TestSegmentCalculator_Calculate_Disabled(t *testing.T) {
	registry := NewIndicatorRegistry()
	registry.Register(NewMockIndicator("test", SegmentTrend, nil))

	calculator := NewSegmentCalculator(registry)

	config := &SegmentConfig{
		Segment:            SegmentTrend,
		SelectedIndicators: []string{"test"},
		AveragingMethod:    "simple",
		Enabled:            false, // Disabled
	}

	data := NewIndicatorData("BTCUSDT")

	result, err := calculator.Calculate(config, data)
	if err != nil {
		t.Fatalf("Calculate() error: %v", err)
	}

	// Disabled segments should return neutral score
	if result.Score != 50 {
		t.Errorf("Expected neutral score 50 for disabled segment, got %.2f", result.Score)
	}
}

func TestSegmentCalculator_Calculate_InvalidIndicator(t *testing.T) {
	registry := NewIndicatorRegistry()
	calculator := NewSegmentCalculator(registry)

	config := &SegmentConfig{
		Segment:            SegmentTrend,
		SelectedIndicators: []string{"non_existent"},
		AveragingMethod:    "simple",
		Enabled:            true,
	}

	data := NewIndicatorData("BTCUSDT")

	_, err := calculator.Calculate(config, data)
	if err == nil {
		t.Error("Expected error for non-existent indicator")
	}
}

func TestSegmentCalculator_CalculateAll(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Register indicators for each segment
	registry.Register(NewMockIndicator("t1", SegmentTrend, nil))
	registry.Register(NewMockIndicator("m1", SegmentMomentum, nil))
	registry.Register(NewMockIndicator("vo1", SegmentVolatility, nil))
	registry.Register(NewMockIndicator("v1", SegmentVolume, nil))

	calculator := NewSegmentCalculator(registry)

	configs := map[IndicatorSegment]*SegmentConfig{
		SegmentTrend: {
			Segment:            SegmentTrend,
			SelectedIndicators: []string{"t1"},
			AveragingMethod:    "simple",
			Enabled:            true,
		},
		SegmentMomentum: {
			Segment:            SegmentMomentum,
			SelectedIndicators: []string{"m1"},
			AveragingMethod:    "simple",
			Enabled:            true,
		},
		SegmentVolatility: {
			Segment:            SegmentVolatility,
			SelectedIndicators: []string{"vo1"},
			AveragingMethod:    "simple",
			Enabled:            true,
		},
		SegmentVolume: {
			Segment:            SegmentVolume,
			SelectedIndicators: []string{"v1"},
			AveragingMethod:    "simple",
			Enabled:            true,
		},
	}

	data := NewIndicatorData("BTCUSDT")

	results, err := calculator.CalculateAll(configs, data)
	if err != nil {
		t.Fatalf("CalculateAll() error: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// Verify each segment has a result
	for _, segment := range AllSegments() {
		if _, ok := results[segment]; !ok {
			t.Errorf("Missing result for segment %s", segment)
		}
	}
}

func TestSegmentResult_AddError(t *testing.T) {
	result := NewSegmentResult(SegmentTrend, "simple")

	if result.HasErrors() {
		t.Error("New result should not have errors")
	}

	result.AddError("test error 1")
	result.AddError("test error 2")

	if !result.HasErrors() {
		t.Error("Result should have errors after AddError")
	}

	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}
}

func TestIndicatorSettings_Clone(t *testing.T) {
	settings := DefaultIndicatorSettings()

	clone := settings.Clone()

	// Verify it's a separate instance
	if clone == settings {
		t.Error("Clone should be a different instance")
	}

	// Verify values are equal
	if clone.Trend.Segment != settings.Trend.Segment {
		t.Error("Cloned trend segment doesn't match")
	}

	// Modify clone and verify original is unchanged
	clone.Trend.SelectedIndicators = []string{"modified"}
	if len(settings.Trend.SelectedIndicators) == 1 && settings.Trend.SelectedIndicators[0] == "modified" {
		t.Error("Modifying clone affected original")
	}
}

func TestIndicatorSettings_EnabledSegments(t *testing.T) {
	settings := DefaultIndicatorSettings()

	enabled := settings.EnabledSegments()
	if len(enabled) != 4 {
		t.Errorf("Expected 4 enabled segments, got %d", len(enabled))
	}

	// Disable one segment
	settings.Trend.Enabled = false

	enabled = settings.EnabledSegments()
	if len(enabled) != 3 {
		t.Errorf("Expected 3 enabled segments after disabling trend, got %d", len(enabled))
	}
}

func TestIndicatorSettings_GetSegmentConfig(t *testing.T) {
	settings := DefaultIndicatorSettings()

	// Test getting each segment
	for _, segment := range AllSegments() {
		config := settings.GetSegmentConfig(segment)
		if config == nil {
			t.Errorf("GetSegmentConfig(%s) returned nil", segment)
		}
		if config.Segment != segment {
			t.Errorf("Expected segment %s, got %s", segment, config.Segment)
		}
	}

	// Test invalid segment
	config := settings.GetSegmentConfig(IndicatorSegment("invalid"))
	if config != nil {
		t.Error("Expected nil for invalid segment")
	}
}

func TestIndicatorValue_IsStale(t *testing.T) {
	value := NewIndicatorValue("test", 50, 50)

	// Just created, should not be stale
	if value.IsStale(100 * time.Second) { // 100 seconds
		t.Error("New value should not be stale")
	}

	// Simulate old timestamp
	value.Timestamp = value.Timestamp - 200000 // 200 seconds ago

	if !value.IsStale(100 * time.Second) { // 100 seconds max age
		t.Error("Old value should be stale")
	}
}

func TestIndicatorData_HasSufficientData(t *testing.T) {
	data := NewIndicatorData("BTCUSDT")

	// No data
	if data.HasSufficientData(10) {
		t.Error("Empty data should not have sufficient data")
	}

	// Add some prices
	data.Prices = make([]float64, 20)
	data.Closes = data.Prices

	if !data.HasSufficientData(10) {
		t.Error("Data with 20 prices should be sufficient for 10 periods")
	}

	if data.HasSufficientData(30) {
		t.Error("Data with 20 prices should not be sufficient for 30 periods")
	}
}

func TestIndicatorData_SetGetOtherValue(t *testing.T) {
	data := NewIndicatorData("BTCUSDT")

	// Set a value
	data.SetOtherValue("rsi", 65.5)

	// Get the value
	value, ok := data.GetOtherValue("rsi")
	if !ok {
		t.Error("Expected to find 'rsi' value")
	}
	if value != 65.5 {
		t.Errorf("Expected 65.5, got %f", value)
	}

	// Get non-existent value
	_, ok = data.GetOtherValue("non_existent")
	if ok {
		t.Error("Should not find non-existent value")
	}
}
