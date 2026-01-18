// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"testing"
)

// MockIndicator is a simple indicator for testing.
type MockIndicator struct {
	BaseIndicator
	calcValue float64
	calcErr   error
}

func NewMockIndicator(name string, segment IndicatorSegment, deps []string) *MockIndicator {
	return &MockIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    name,
			IndicatorSegment: segment,
			IndicatorDeps:    deps,
		},
		calcValue: 50.0,
	}
}

func (m *MockIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if m.calcErr != nil {
		return nil, m.calcErr
	}
	return NewIndicatorValue(m.Name(), m.calcValue, m.Normalize(m.calcValue)), nil
}

func (m *MockIndicator) Normalize(value float64) float64 {
	return value // Already normalized for testing
}

func (m *MockIndicator) Description() string {
	return "Mock indicator for testing"
}

func TestIndicatorRegistry_Register(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Test successful registration
	ind := NewMockIndicator("test_indicator", SegmentTrend, nil)
	err := registry.Register(ind)
	if err != nil {
		t.Errorf("Expected successful registration, got error: %v", err)
	}

	// Test duplicate registration
	err = registry.Register(ind)
	if err == nil {
		t.Error("Expected error for duplicate registration, got nil")
	}

	// Test nil indicator
	err = registry.Register(nil)
	if err == nil {
		t.Error("Expected error for nil indicator, got nil")
	}

	// Test empty name indicator
	emptyName := NewMockIndicator("", SegmentTrend, nil)
	err = registry.Register(emptyName)
	if err == nil {
		t.Error("Expected error for empty name indicator, got nil")
	}
}

func TestIndicatorRegistry_Get(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Register an indicator
	ind := NewMockIndicator("rsi_test", SegmentMomentum, nil)
	registry.Register(ind)

	// Test successful retrieval
	retrieved, ok := registry.Get("rsi_test")
	if !ok {
		t.Error("Expected to find registered indicator")
	}
	if retrieved.Name() != "rsi_test" {
		t.Errorf("Expected name 'rsi_test', got '%s'", retrieved.Name())
	}

	// Test non-existent indicator
	_, ok = registry.Get("non_existent")
	if ok {
		t.Error("Expected not to find non-existent indicator")
	}
}

func TestIndicatorRegistry_GetBySegment(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Register indicators in different segments
	registry.Register(NewMockIndicator("trend1", SegmentTrend, nil))
	registry.Register(NewMockIndicator("trend2", SegmentTrend, nil))
	registry.Register(NewMockIndicator("momentum1", SegmentMomentum, nil))

	// Test trend segment
	trendIndicators := registry.GetBySegment(SegmentTrend)
	if len(trendIndicators) != 2 {
		t.Errorf("Expected 2 trend indicators, got %d", len(trendIndicators))
	}

	// Test momentum segment
	momentumIndicators := registry.GetBySegment(SegmentMomentum)
	if len(momentumIndicators) != 1 {
		t.Errorf("Expected 1 momentum indicator, got %d", len(momentumIndicators))
	}

	// Test empty segment
	volumeIndicators := registry.GetBySegment(SegmentVolume)
	if len(volumeIndicators) != 0 {
		t.Errorf("Expected 0 volume indicators, got %d", len(volumeIndicators))
	}
}

func TestIndicatorRegistry_ListAll(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Register multiple indicators
	registry.Register(NewMockIndicator("zebra", SegmentTrend, nil))
	registry.Register(NewMockIndicator("alpha", SegmentMomentum, nil))
	registry.Register(NewMockIndicator("beta", SegmentVolatility, nil))

	names := registry.ListAll()
	if len(names) != 3 {
		t.Errorf("Expected 3 indicators, got %d", len(names))
	}

	// Verify sorted order
	if names[0] != "alpha" || names[1] != "beta" || names[2] != "zebra" {
		t.Errorf("Expected sorted order [alpha, beta, zebra], got %v", names)
	}
}

func TestIndicatorRegistry_Validate(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Register some indicators
	registry.Register(NewMockIndicator("rsi", SegmentMomentum, nil))
	registry.Register(NewMockIndicator("macd", SegmentTrend, nil))

	// Test valid indicators
	err := registry.Validate([]string{"rsi", "macd"})
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}

	// Test missing indicators
	err = registry.Validate([]string{"rsi", "non_existent"})
	if err == nil {
		t.Error("Expected validation to fail for missing indicator")
	}
}

func TestIndicatorRegistry_ValidateForSegment(t *testing.T) {
	registry := NewIndicatorRegistry()

	registry.Register(NewMockIndicator("rsi", SegmentMomentum, nil))
	registry.Register(NewMockIndicator("macd", SegmentTrend, nil))

	// Test correct segment
	err := registry.ValidateForSegment([]string{"rsi"}, SegmentMomentum)
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}

	// Test wrong segment
	err = registry.ValidateForSegment([]string{"rsi"}, SegmentTrend)
	if err == nil {
		t.Error("Expected validation to fail for wrong segment")
	}
}

func TestIndicatorRegistry_Unregister(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Register and then unregister
	registry.Register(NewMockIndicator("temp", SegmentTrend, nil))

	err := registry.Unregister("temp")
	if err != nil {
		t.Errorf("Expected successful unregister, got error: %v", err)
	}

	// Verify it's gone
	_, ok := registry.Get("temp")
	if ok {
		t.Error("Indicator should be unregistered")
	}

	// Test unregistering non-existent
	err = registry.Unregister("non_existent")
	if err == nil {
		t.Error("Expected error for unregistering non-existent indicator")
	}
}

func TestIndicatorRegistry_ResolveDependencies(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Create indicators with dependencies
	// atr has no dependencies
	atr := NewMockIndicator("atr", SegmentVolatility, nil)
	// keltner depends on atr
	keltner := NewMockIndicator("keltner", SegmentVolatility, []string{"atr"})
	// supertrend depends on atr
	supertrend := NewMockIndicator("supertrend", SegmentTrend, []string{"atr"})

	registry.Register(atr)
	registry.Register(keltner)
	registry.Register(supertrend)

	// Test dependency resolution
	indicators, err := registry.ResolveDependencies([]string{"keltner"})
	if err != nil {
		t.Errorf("Expected successful resolution, got error: %v", err)
	}

	// Should return [atr, keltner] in that order
	if len(indicators) != 2 {
		t.Errorf("Expected 2 indicators, got %d", len(indicators))
	}

	if indicators[0].Name() != "atr" {
		t.Errorf("Expected first indicator to be 'atr', got '%s'", indicators[0].Name())
	}
	if indicators[1].Name() != "keltner" {
		t.Errorf("Expected second indicator to be 'keltner', got '%s'", indicators[1].Name())
	}
}

func TestIndicatorRegistry_ResolveDependencies_Circular(t *testing.T) {
	registry := NewIndicatorRegistry()

	// Create circular dependency: a -> b -> a
	a := NewMockIndicator("a", SegmentTrend, []string{"b"})
	b := NewMockIndicator("b", SegmentTrend, []string{"a"})

	registry.Register(a)
	registry.Register(b)

	// Should detect circular dependency
	_, err := registry.ResolveDependencies([]string{"a"})
	if err == nil {
		t.Error("Expected error for circular dependency")
	}
}

func TestIndicatorRegistry_SetEnabled(t *testing.T) {
	registry := NewIndicatorRegistry()

	registry.Register(NewMockIndicator("test", SegmentTrend, nil))

	// Initially enabled
	if !registry.IsEnabled("test") {
		t.Error("Expected indicator to be initially enabled")
	}

	// Disable
	err := registry.SetEnabled("test", false)
	if err != nil {
		t.Errorf("Expected successful disable, got error: %v", err)
	}

	if registry.IsEnabled("test") {
		t.Error("Expected indicator to be disabled")
	}

	// Re-enable
	err = registry.SetEnabled("test", true)
	if err != nil {
		t.Errorf("Expected successful enable, got error: %v", err)
	}

	if !registry.IsEnabled("test") {
		t.Error("Expected indicator to be re-enabled")
	}
}

func TestIndicatorRegistry_GetEnabledBySegment(t *testing.T) {
	registry := NewIndicatorRegistry()

	registry.Register(NewMockIndicator("enabled1", SegmentTrend, nil))
	registry.Register(NewMockIndicator("enabled2", SegmentTrend, nil))
	registry.Register(NewMockIndicator("disabled", SegmentTrend, nil))

	registry.SetEnabled("disabled", false)

	enabled := registry.GetEnabledBySegment(SegmentTrend)
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled indicators, got %d", len(enabled))
	}
}

func TestGlobalIndicatorRegistry(t *testing.T) {
	// Test global registry singleton
	registry1 := GetGlobalIndicatorRegistry()
	registry2 := GetGlobalIndicatorRegistry()

	if registry1 != registry2 {
		t.Error("Expected same global registry instance")
	}
}

func TestIndicatorSegment_IsValid(t *testing.T) {
	tests := []struct {
		segment IndicatorSegment
		valid   bool
	}{
		{SegmentTrend, true},
		{SegmentMomentum, true},
		{SegmentVolatility, true},
		{SegmentVolume, true},
		{IndicatorSegment("invalid"), false},
		{IndicatorSegment(""), false},
	}

	for _, test := range tests {
		if test.segment.IsValid() != test.valid {
			t.Errorf("Segment '%s'.IsValid() = %v, expected %v", test.segment, test.segment.IsValid(), test.valid)
		}
	}
}
