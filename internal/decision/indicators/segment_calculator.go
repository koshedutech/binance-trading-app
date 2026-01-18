// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"time"
)

// SegmentConfig holds user's indicator selection for a segment.
type SegmentConfig struct {
	// Segment is which segment this config applies to.
	Segment IndicatorSegment `json:"segment"`
	// SelectedIndicators lists the 2-3 indicators chosen for this segment.
	SelectedIndicators []string `json:"selected_indicators"`
	// AveragingMethod determines how to combine indicator scores: "simple" or "weighted".
	AveragingMethod string `json:"averaging_method"`
	// Weights maps indicator name to weight (only used when averaging_method is "weighted").
	Weights map[string]float64 `json:"weights,omitempty"`
	// Enabled indicates if this segment is active in scoring.
	Enabled bool `json:"enabled"`
}

// NewSegmentConfig creates a new SegmentConfig with defaults.
func NewSegmentConfig(segment IndicatorSegment) *SegmentConfig {
	return &SegmentConfig{
		Segment:            segment,
		SelectedIndicators: []string{},
		AveragingMethod:    "simple",
		Weights:            make(map[string]float64),
		Enabled:            true,
	}
}

// Validate checks if the segment config is valid.
func (sc *SegmentConfig) Validate() error {
	if !sc.Segment.IsValid() {
		return fmt.Errorf("invalid segment: %s", sc.Segment)
	}

	if len(sc.SelectedIndicators) == 0 {
		return fmt.Errorf("at least one indicator must be selected for segment %s", sc.Segment)
	}

	if len(sc.SelectedIndicators) > 4 {
		return fmt.Errorf("maximum 4 indicators allowed per segment, got %d", len(sc.SelectedIndicators))
	}

	if sc.AveragingMethod != "simple" && sc.AveragingMethod != "weighted" {
		return fmt.Errorf("invalid averaging method: %s (must be 'simple' or 'weighted')", sc.AveragingMethod)
	}

	if sc.AveragingMethod == "weighted" {
		// Validate weights exist for all selected indicators
		for _, name := range sc.SelectedIndicators {
			if _, ok := sc.Weights[name]; !ok {
				return fmt.Errorf("missing weight for indicator: %s", name)
			}
		}

		// Validate weights sum to approximately 1.0
		var totalWeight float64
		for _, name := range sc.SelectedIndicators {
			totalWeight += sc.Weights[name]
		}
		if totalWeight < 0.99 || totalWeight > 1.01 {
			return fmt.Errorf("weights must sum to 1.0, got %.2f", totalWeight)
		}
	}

	return nil
}

// SegmentResult holds the calculated segment score.
type SegmentResult struct {
	// Segment identifies which segment this result is for.
	Segment IndicatorSegment `json:"segment"`
	// Score is the combined score for this segment (0-100).
	Score float64 `json:"score"`
	// IndividualScores contains the normalized score for each indicator.
	IndividualScores map[string]float64 `json:"individual_scores"`
	// RawValues contains the raw (unnormalized) values for each indicator.
	RawValues map[string]float64 `json:"raw_values"`
	// Method indicates how scores were combined ("simple" or "weighted").
	Method string `json:"method"`
	// CalculatedAt is when this result was computed (Unix milliseconds).
	CalculatedAt int64 `json:"calculated_at"`
	// Confidence is the average confidence of the indicator values (0-1).
	Confidence float64 `json:"confidence"`
	// Errors lists any errors encountered during calculation.
	Errors []string `json:"errors,omitempty"`
}

// NewSegmentResult creates a new SegmentResult with initialized maps.
func NewSegmentResult(segment IndicatorSegment, method string) *SegmentResult {
	return &SegmentResult{
		Segment:          segment,
		IndividualScores: make(map[string]float64),
		RawValues:        make(map[string]float64),
		Method:           method,
		CalculatedAt:     time.Now().UnixMilli(),
		Confidence:       1.0,
	}
}

// AddIndicatorResult adds an indicator's result to the segment result.
func (sr *SegmentResult) AddIndicatorResult(name string, normalized, raw float64) {
	sr.IndividualScores[name] = normalized
	sr.RawValues[name] = raw
}

// AddError records an error that occurred during calculation.
func (sr *SegmentResult) AddError(err string) {
	sr.Errors = append(sr.Errors, err)
}

// HasErrors returns true if any errors occurred.
func (sr *SegmentResult) HasErrors() bool {
	return len(sr.Errors) > 0
}

// SegmentCalculator calculates segment scores using the indicator registry.
type SegmentCalculator struct {
	registry *IndicatorRegistry
}

// NewSegmentCalculator creates a new SegmentCalculator with the given registry.
// If registry is nil, uses the global registry.
func NewSegmentCalculator(registry *IndicatorRegistry) *SegmentCalculator {
	if registry == nil {
		registry = GetGlobalIndicatorRegistry()
	}
	return &SegmentCalculator{
		registry: registry,
	}
}

// Calculate computes the segment score based on config and data.
func (s *SegmentCalculator) Calculate(config *SegmentConfig, data *IndicatorData) (*SegmentResult, error) {
	if config == nil {
		return nil, fmt.Errorf("segment config is nil")
	}

	if !config.Enabled {
		result := NewSegmentResult(config.Segment, config.AveragingMethod)
		result.Score = 50 // Return neutral score for disabled segments
		return result, nil
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Validate all selected indicators exist and belong to this segment
	if err := s.registry.ValidateForSegment(config.SelectedIndicators, config.Segment); err != nil {
		return nil, fmt.Errorf("indicator validation failed: %w", err)
	}

	// Resolve dependencies and get indicators in calculation order
	indicators, err := s.registry.ResolveDependencies(config.SelectedIndicators)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution failed: %w", err)
	}

	result := NewSegmentResult(config.Segment, config.AveragingMethod)

	// Calculate each indicator
	var totalConfidence float64
	var validCount int

	for _, ind := range indicators {
		value, calcErr := ind.Calculate(data)
		if calcErr != nil {
			result.AddError(fmt.Sprintf("%s: %v", ind.Name(), calcErr))
			continue
		}

		// Store the result
		result.AddIndicatorResult(ind.Name(), value.Normalized, value.Value)

		// Also add to OtherValues for dependent indicators
		data.SetOtherValue(ind.Name(), value.Value)

		totalConfidence += value.Confidence
		validCount++
	}

	if validCount == 0 {
		result.Score = 50 // Return neutral if no indicators calculated
		result.Confidence = 0
		return result, nil
	}

	// Calculate combined score
	result.Score = s.combineScores(config, result.IndividualScores)
	result.Confidence = totalConfidence / float64(validCount)

	return result, nil
}

// combineScores combines individual indicator scores using the configured method.
func (s *SegmentCalculator) combineScores(config *SegmentConfig, scores map[string]float64) float64 {
	if len(scores) == 0 {
		return 50 // Neutral
	}

	switch config.AveragingMethod {
	case "weighted":
		return s.weightedAverage(config.SelectedIndicators, config.Weights, scores)
	default: // "simple"
		return s.simpleAverage(scores)
	}
}

// simpleAverage calculates the arithmetic mean of all scores.
func (s *SegmentCalculator) simpleAverage(scores map[string]float64) float64 {
	if len(scores) == 0 {
		return 50
	}

	var sum float64
	for _, score := range scores {
		sum += score
	}
	return sum / float64(len(scores))
}

// weightedAverage calculates the weighted average of scores.
func (s *SegmentCalculator) weightedAverage(indicators []string, weights, scores map[string]float64) float64 {
	var sum float64
	var totalWeight float64

	for _, name := range indicators {
		score, hasScore := scores[name]
		weight, hasWeight := weights[name]

		if !hasScore || !hasWeight {
			continue
		}

		sum += score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 50
	}

	return sum / totalWeight
}

// CalculateAll computes all four segment scores at once.
func (s *SegmentCalculator) CalculateAll(configs map[IndicatorSegment]*SegmentConfig, data *IndicatorData) (map[IndicatorSegment]*SegmentResult, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	results := make(map[IndicatorSegment]*SegmentResult)

	for segment, config := range configs {
		if config == nil {
			continue
		}

		result, err := s.Calculate(config, data)
		if err != nil {
			// Create error result instead of failing entirely
			result = NewSegmentResult(segment, "error")
			result.Score = 50
			result.AddError(err.Error())
		}

		results[segment] = result
	}

	return results, nil
}

// CalculateAllFromSettings computes segment scores using IndicatorSettings.
func (s *SegmentCalculator) CalculateAllFromSettings(settings *IndicatorSettings, data *IndicatorData) (map[IndicatorSegment]*SegmentResult, error) {
	if settings == nil {
		return nil, fmt.Errorf("indicator settings is nil")
	}

	configs := map[IndicatorSegment]*SegmentConfig{
		SegmentTrend:      &settings.Trend,
		SegmentMomentum:   &settings.Momentum,
		SegmentVolatility: &settings.Volatility,
		SegmentVolume:     &settings.Volume,
	}

	return s.CalculateAll(configs, data)
}

// GetAvailableIndicatorsForSegment returns all registered indicators for a segment.
func (s *SegmentCalculator) GetAvailableIndicatorsForSegment(segment IndicatorSegment) []*IndicatorInfo {
	indicators := s.registry.GetBySegment(segment)

	infos := make([]*IndicatorInfo, 0, len(indicators))
	for _, ind := range indicators {
		if info, ok := s.registry.GetInfo(ind.Name()); ok {
			infos = append(infos, info)
		}
	}

	return infos
}

// ValidateIndicatorSelection validates that the selected indicators are valid for the segment.
func (s *SegmentCalculator) ValidateIndicatorSelection(segment IndicatorSegment, indicators []string) error {
	return s.registry.ValidateForSegment(indicators, segment)
}
