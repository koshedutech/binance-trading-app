// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"time"
)

// IndicatorSegment represents the four indicator segments for scoring.
type IndicatorSegment string

const (
	// SegmentTrend contains indicators for direction determination.
	SegmentTrend IndicatorSegment = "trend"
	// SegmentMomentum contains indicators for strength measurement.
	SegmentMomentum IndicatorSegment = "momentum"
	// SegmentVolatility contains indicators for risk assessment.
	SegmentVolatility IndicatorSegment = "volatility"
	// SegmentVolume contains indicators for confirmation.
	SegmentVolume IndicatorSegment = "volume"
)

// AllSegments returns all available indicator segments.
func AllSegments() []IndicatorSegment {
	return []IndicatorSegment{
		SegmentTrend,
		SegmentMomentum,
		SegmentVolatility,
		SegmentVolume,
	}
}

// IsValid checks if the segment is a valid indicator segment.
func (s IndicatorSegment) IsValid() bool {
	switch s {
	case SegmentTrend, SegmentMomentum, SegmentVolatility, SegmentVolume:
		return true
	default:
		return false
	}
}

// String returns the string representation of the segment.
func (s IndicatorSegment) String() string {
	return string(s)
}

// IndicatorValue holds a calculated indicator value with metadata.
type IndicatorValue struct {
	// Name is the indicator name (e.g., "rsi", "ema_cross").
	Name string `json:"name"`
	// Value is the raw calculated value.
	Value float64 `json:"value"`
	// Normalized is the value scaled to 0-100 range for scoring.
	Normalized float64 `json:"normalized"`
	// Timestamp is when the value was calculated (Unix milliseconds).
	Timestamp int64 `json:"timestamp"`
	// Confidence is the data quality indicator (0-1 scale).
	Confidence float64 `json:"confidence"`
	// Source indicates where the data came from (e.g., "calculated", "cached", "external").
	Source string `json:"source"`
}

// NewIndicatorValue creates a new IndicatorValue with the current timestamp.
func NewIndicatorValue(name string, value, normalized float64) *IndicatorValue {
	return &IndicatorValue{
		Name:       name,
		Value:      value,
		Normalized: normalized,
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 1.0,
		Source:     "calculated",
	}
}

// WithConfidence sets the confidence level and returns the value.
func (iv *IndicatorValue) WithConfidence(confidence float64) *IndicatorValue {
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	iv.Confidence = confidence
	return iv
}

// WithSource sets the data source and returns the value.
func (iv *IndicatorValue) WithSource(source string) *IndicatorValue {
	iv.Source = source
	return iv
}

// IsStale checks if the indicator value is older than the given duration.
func (iv *IndicatorValue) IsStale(maxAge time.Duration) bool {
	return time.Since(time.UnixMilli(iv.Timestamp)) > maxAge
}

// Indicator interface for all indicators.
// Indicators are responsible for calculating their values from market data
// and normalizing them to a 0-100 scale for consistent scoring.
type Indicator interface {
	// Name returns the unique identifier for this indicator.
	Name() string

	// Segment returns which segment this indicator belongs to.
	Segment() IndicatorSegment

	// Calculate computes the indicator value from the provided data.
	Calculate(data *IndicatorData) (*IndicatorValue, error)

	// Normalize converts a raw value to the 0-100 scale.
	// This is also called internally by Calculate, but exposed for flexibility.
	Normalize(value float64) float64

	// Dependencies returns the names of other indicators that must be calculated first.
	// Returns an empty slice if there are no dependencies.
	Dependencies() []string
}

// IndicatorDescriber is an optional interface for indicators that provide descriptions.
type IndicatorDescriber interface {
	// Description returns a human-readable explanation of the indicator.
	Description() string
}

// IndicatorData provides the market data needed for indicator calculations.
type IndicatorData struct {
	// Symbol is the trading pair (e.g., "BTCUSDT").
	Symbol string `json:"symbol"`

	// Current price
	Price float64 `json:"price"`

	// Historical OHLCV data (most recent last)
	Prices     []float64 `json:"prices"`     // Close prices
	Highs      []float64 `json:"highs"`      // High prices
	Lows       []float64 `json:"lows"`       // Low prices
	Opens      []float64 `json:"opens"`      // Open prices
	Closes     []float64 `json:"closes"`     // Alias for Prices for clarity
	Volumes    []float64 `json:"volumes"`    // Volume data
	Timestamps []int64   `json:"timestamps"` // Unix timestamps

	// Pre-calculated indicator values (for dependencies)
	// Key is indicator name, value is the calculated result
	OtherValues map[string]float64 `json:"other_values"`

	// Additional context
	Timeframe string `json:"timeframe"` // e.g., "1h", "15m"
}

// NewIndicatorData creates a new IndicatorData with initialized maps.
func NewIndicatorData(symbol string) *IndicatorData {
	return &IndicatorData{
		Symbol:      symbol,
		OtherValues: make(map[string]float64),
	}
}

// SetPrice sets the current price and returns the data for chaining.
func (id *IndicatorData) SetPrice(price float64) *IndicatorData {
	id.Price = price
	return id
}

// SetOHLCV sets the historical OHLCV data and returns the data for chaining.
func (id *IndicatorData) SetOHLCV(opens, highs, lows, closes, volumes []float64) *IndicatorData {
	id.Opens = opens
	id.Highs = highs
	id.Lows = lows
	id.Closes = closes
	id.Prices = closes // Alias
	id.Volumes = volumes
	return id
}

// SetOtherValue sets a pre-calculated indicator value.
func (id *IndicatorData) SetOtherValue(name string, value float64) *IndicatorData {
	if id.OtherValues == nil {
		id.OtherValues = make(map[string]float64)
	}
	id.OtherValues[name] = value
	return id
}

// GetOtherValue retrieves a pre-calculated indicator value.
// Returns 0 and false if the value doesn't exist.
func (id *IndicatorData) GetOtherValue(name string) (float64, bool) {
	if id.OtherValues == nil {
		return 0, false
	}
	v, ok := id.OtherValues[name]
	return v, ok
}

// HasSufficientData checks if there's enough historical data for calculation.
func (id *IndicatorData) HasSufficientData(minPeriods int) bool {
	return len(id.Closes) >= minPeriods || len(id.Prices) >= minPeriods
}

// Validate checks if the IndicatorData has the minimum required fields.
func (id *IndicatorData) Validate() error {
	if id.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	return nil
}

// IndicatorInfo provides metadata about a registered indicator.
type IndicatorInfo struct {
	// Name is the indicator's unique identifier.
	Name string `json:"name"`
	// Segment is which segment the indicator belongs to.
	Segment IndicatorSegment `json:"segment"`
	// Description is a human-readable explanation.
	Description string `json:"description"`
	// Dependencies lists other indicators that must be calculated first.
	Dependencies []string `json:"dependencies"`
	// RegisteredAt is when this indicator was registered (Unix milliseconds).
	RegisteredAt int64 `json:"registered_at"`
	// Enabled indicates if this indicator is available for use.
	Enabled bool `json:"enabled"`
}

// NewIndicatorInfo creates an IndicatorInfo from an Indicator.
func NewIndicatorInfo(ind Indicator) *IndicatorInfo {
	info := &IndicatorInfo{
		Name:         ind.Name(),
		Segment:      ind.Segment(),
		Dependencies: ind.Dependencies(),
		RegisteredAt: time.Now().UnixMilli(),
		Enabled:      true,
	}

	// Check if indicator implements IndicatorDescriber for description
	if describer, ok := ind.(IndicatorDescriber); ok {
		info.Description = describer.Description()
	}

	return info
}

// BaseIndicator provides common functionality for indicator implementations.
// Embed this in concrete indicator types to get default implementations.
type BaseIndicator struct {
	IndicatorName    string
	IndicatorSegment IndicatorSegment
	IndicatorDeps    []string
}

// Name returns the indicator name.
func (bi *BaseIndicator) Name() string {
	return bi.IndicatorName
}

// Segment returns the indicator segment.
func (bi *BaseIndicator) Segment() IndicatorSegment {
	return bi.IndicatorSegment
}

// Dependencies returns the indicator dependencies.
func (bi *BaseIndicator) Dependencies() []string {
	if bi.IndicatorDeps == nil {
		return []string{}
	}
	return bi.IndicatorDeps
}
