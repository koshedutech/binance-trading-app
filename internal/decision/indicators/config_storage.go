// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import "time"

// IndicatorSettings holds the complete indicator configuration for all segments.
// This integrates with DecisionEngineSettings from Story 11.23.
type IndicatorSettings struct {
	// Trend contains the configuration for trend indicators.
	Trend SegmentConfig `json:"trend"`
	// Momentum contains the configuration for momentum indicators.
	Momentum SegmentConfig `json:"momentum"`
	// Volatility contains the configuration for volatility indicators.
	Volatility SegmentConfig `json:"volatility"`
	// Volume contains the configuration for volume indicators.
	Volume SegmentConfig `json:"volume"`
	// LastUpdated is when these settings were last modified (Unix milliseconds).
	LastUpdated int64 `json:"last_updated"`
	// Version is the settings schema version for migration support.
	Version int `json:"version"`
}

// DefaultIndicatorSettings returns sensible defaults for all segments.
// These defaults provide a balanced starting point for most trading strategies.
func DefaultIndicatorSettings() *IndicatorSettings {
	return &IndicatorSettings{
		Trend: SegmentConfig{
			Segment:            SegmentTrend,
			SelectedIndicators: []string{"ema_cross", "macd"},
			AveragingMethod:    "weighted",
			Weights: map[string]float64{
				"ema_cross": 0.6,
				"macd":      0.4,
			},
			Enabled: true,
		},
		Momentum: SegmentConfig{
			Segment:            SegmentMomentum,
			SelectedIndicators: []string{"rsi", "stochastic"},
			AveragingMethod:    "weighted",
			Weights: map[string]float64{
				"rsi":        0.6,
				"stochastic": 0.4,
			},
			Enabled: true,
		},
		Volatility: SegmentConfig{
			Segment:            SegmentVolatility,
			SelectedIndicators: []string{"atr", "bollinger_width"},
			AveragingMethod:    "simple",
			Weights:            nil,
			Enabled:            true,
		},
		Volume: SegmentConfig{
			Segment:            SegmentVolume,
			SelectedIndicators: []string{"volume_sma", "vwap"},
			AveragingMethod:    "simple",
			Weights:            nil,
			Enabled:            true,
		},
		LastUpdated: time.Now().UnixMilli(),
		Version:     1,
	}
}

// NewIndicatorSettings creates a new IndicatorSettings with initialized segments.
func NewIndicatorSettings() *IndicatorSettings {
	return &IndicatorSettings{
		Trend: SegmentConfig{
			Segment:            SegmentTrend,
			SelectedIndicators: []string{},
			AveragingMethod:    "simple",
			Weights:            make(map[string]float64),
			Enabled:            true,
		},
		Momentum: SegmentConfig{
			Segment:            SegmentMomentum,
			SelectedIndicators: []string{},
			AveragingMethod:    "simple",
			Weights:            make(map[string]float64),
			Enabled:            true,
		},
		Volatility: SegmentConfig{
			Segment:            SegmentVolatility,
			SelectedIndicators: []string{},
			AveragingMethod:    "simple",
			Weights:            make(map[string]float64),
			Enabled:            true,
		},
		Volume: SegmentConfig{
			Segment:            SegmentVolume,
			SelectedIndicators: []string{},
			AveragingMethod:    "simple",
			Weights:            make(map[string]float64),
			Enabled:            true,
		},
		LastUpdated: time.Now().UnixMilli(),
		Version:     1,
	}
}

// Validate checks if all segment configurations are valid.
func (is *IndicatorSettings) Validate() error {
	// Validate each segment that has indicators
	if len(is.Trend.SelectedIndicators) > 0 {
		if err := is.Trend.Validate(); err != nil {
			return err
		}
	}
	if len(is.Momentum.SelectedIndicators) > 0 {
		if err := is.Momentum.Validate(); err != nil {
			return err
		}
	}
	if len(is.Volatility.SelectedIndicators) > 0 {
		if err := is.Volatility.Validate(); err != nil {
			return err
		}
	}
	if len(is.Volume.SelectedIndicators) > 0 {
		if err := is.Volume.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Clone returns a deep copy of the IndicatorSettings.
func (is *IndicatorSettings) Clone() *IndicatorSettings {
	if is == nil {
		return nil
	}

	clone := &IndicatorSettings{
		Trend:       cloneSegmentConfig(is.Trend),
		Momentum:    cloneSegmentConfig(is.Momentum),
		Volatility:  cloneSegmentConfig(is.Volatility),
		Volume:      cloneSegmentConfig(is.Volume),
		LastUpdated: is.LastUpdated,
		Version:     is.Version,
	}

	return clone
}

// GetSegmentConfig returns the configuration for a specific segment.
func (is *IndicatorSettings) GetSegmentConfig(segment IndicatorSegment) *SegmentConfig {
	switch segment {
	case SegmentTrend:
		return &is.Trend
	case SegmentMomentum:
		return &is.Momentum
	case SegmentVolatility:
		return &is.Volatility
	case SegmentVolume:
		return &is.Volume
	default:
		return nil
	}
}

// SetSegmentConfig updates the configuration for a specific segment.
func (is *IndicatorSettings) SetSegmentConfig(config SegmentConfig) {
	switch config.Segment {
	case SegmentTrend:
		is.Trend = config
	case SegmentMomentum:
		is.Momentum = config
	case SegmentVolatility:
		is.Volatility = config
	case SegmentVolume:
		is.Volume = config
	}
	is.LastUpdated = time.Now().UnixMilli()
}

// UpdateTimestamp updates the LastUpdated field to current time.
func (is *IndicatorSettings) UpdateTimestamp() {
	is.LastUpdated = time.Now().UnixMilli()
}

// EnabledSegments returns a list of enabled segments.
func (is *IndicatorSettings) EnabledSegments() []IndicatorSegment {
	var enabled []IndicatorSegment

	if is.Trend.Enabled && len(is.Trend.SelectedIndicators) > 0 {
		enabled = append(enabled, SegmentTrend)
	}
	if is.Momentum.Enabled && len(is.Momentum.SelectedIndicators) > 0 {
		enabled = append(enabled, SegmentMomentum)
	}
	if is.Volatility.Enabled && len(is.Volatility.SelectedIndicators) > 0 {
		enabled = append(enabled, SegmentVolatility)
	}
	if is.Volume.Enabled && len(is.Volume.SelectedIndicators) > 0 {
		enabled = append(enabled, SegmentVolume)
	}

	return enabled
}

// AllSelectedIndicators returns all selected indicators across all segments.
func (is *IndicatorSettings) AllSelectedIndicators() []string {
	var all []string

	if is.Trend.Enabled {
		all = append(all, is.Trend.SelectedIndicators...)
	}
	if is.Momentum.Enabled {
		all = append(all, is.Momentum.SelectedIndicators...)
	}
	if is.Volatility.Enabled {
		all = append(all, is.Volatility.SelectedIndicators...)
	}
	if is.Volume.Enabled {
		all = append(all, is.Volume.SelectedIndicators...)
	}

	return all
}

// Helper function to clone a SegmentConfig
func cloneSegmentConfig(sc SegmentConfig) SegmentConfig {
	clone := SegmentConfig{
		Segment:            sc.Segment,
		SelectedIndicators: make([]string, len(sc.SelectedIndicators)),
		AveragingMethod:    sc.AveragingMethod,
		Enabled:            sc.Enabled,
	}

	copy(clone.SelectedIndicators, sc.SelectedIndicators)

	if sc.Weights != nil {
		clone.Weights = make(map[string]float64, len(sc.Weights))
		for k, v := range sc.Weights {
			clone.Weights[k] = v
		}
	}

	return clone
}

// IndicatorSettingsForJSON provides the JSON-ready structure for default-settings.json.
// This is the structure that will be added to the settings file.
type IndicatorSettingsForJSON struct {
	Trend      SegmentConfigJSON `json:"trend"`
	Momentum   SegmentConfigJSON `json:"momentum"`
	Volatility SegmentConfigJSON `json:"volatility"`
	Volume     SegmentConfigJSON `json:"volume"`
}

// SegmentConfigJSON is the JSON structure for a segment configuration.
type SegmentConfigJSON struct {
	Segment            string             `json:"segment"`
	SelectedIndicators []string           `json:"selected_indicators"`
	AveragingMethod    string             `json:"averaging_method"`
	Weights            map[string]float64 `json:"weights,omitempty"`
	Enabled            bool               `json:"enabled"`
}

// ToJSON converts IndicatorSettings to the JSON-ready structure.
func (is *IndicatorSettings) ToJSON() *IndicatorSettingsForJSON {
	return &IndicatorSettingsForJSON{
		Trend:      segmentConfigToJSON(is.Trend),
		Momentum:   segmentConfigToJSON(is.Momentum),
		Volatility: segmentConfigToJSON(is.Volatility),
		Volume:     segmentConfigToJSON(is.Volume),
	}
}

func segmentConfigToJSON(sc SegmentConfig) SegmentConfigJSON {
	return SegmentConfigJSON{
		Segment:            string(sc.Segment),
		SelectedIndicators: sc.SelectedIndicators,
		AveragingMethod:    sc.AveragingMethod,
		Weights:            sc.Weights,
		Enabled:            sc.Enabled,
	}
}

// FromJSON creates IndicatorSettings from the JSON structure.
func FromJSONIndicatorSettings(json *IndicatorSettingsForJSON) *IndicatorSettings {
	if json == nil {
		return DefaultIndicatorSettings()
	}

	return &IndicatorSettings{
		Trend:       segmentConfigFromJSON(json.Trend),
		Momentum:    segmentConfigFromJSON(json.Momentum),
		Volatility:  segmentConfigFromJSON(json.Volatility),
		Volume:      segmentConfigFromJSON(json.Volume),
		LastUpdated: time.Now().UnixMilli(),
		Version:     1,
	}
}

func segmentConfigFromJSON(json SegmentConfigJSON) SegmentConfig {
	return SegmentConfig{
		Segment:            IndicatorSegment(json.Segment),
		SelectedIndicators: json.SelectedIndicators,
		AveragingMethod:    json.AveragingMethod,
		Weights:            json.Weights,
		Enabled:            json.Enabled,
	}
}
