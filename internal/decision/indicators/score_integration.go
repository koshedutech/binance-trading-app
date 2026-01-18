// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"sync"

	"binance-trading-bot/internal/decision"
)

// ScoreIntegration provides integration between the indicator framework and the score calculator.
// It maps segment scores to the technical score components used by the additive scoring system.
// This type is thread-safe for concurrent access.
type ScoreIntegration struct {
	mu         sync.RWMutex
	calculator *SegmentCalculator
	settings   *IndicatorSettings
}

// NewScoreIntegration creates a new ScoreIntegration with the given settings.
// If settings is nil, uses default settings.
func NewScoreIntegration(settings *IndicatorSettings) *ScoreIntegration {
	if settings == nil {
		settings = DefaultIndicatorSettings()
	}
	return &ScoreIntegration{
		calculator: NewSegmentCalculator(nil), // Uses global registry
		settings:   settings,
	}
}

// NewScoreIntegrationWithRegistry creates a new ScoreIntegration with a custom registry.
func NewScoreIntegrationWithRegistry(settings *IndicatorSettings, registry *IndicatorRegistry) *ScoreIntegration {
	if settings == nil {
		settings = DefaultIndicatorSettings()
	}
	return &ScoreIntegration{
		calculator: NewSegmentCalculator(registry),
		settings:   settings,
	}
}

// GetSettings returns the current indicator settings.
func (si *ScoreIntegration) GetSettings() *IndicatorSettings {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.settings.Clone()
}

// SetSettings updates the indicator settings.
// This method is thread-safe.
func (si *ScoreIntegration) SetSettings(settings *IndicatorSettings) {
	if settings != nil {
		si.mu.Lock()
		defer si.mu.Unlock()
		si.settings = settings.Clone()
	}
}

// TechnicalScoreComponents holds the component scores for the technical dimension.
type TechnicalScoreComponents struct {
	TrendAlignment float64 `json:"trend_alignment"` // 0-15 points
	Momentum       float64 `json:"momentum"`        // 0-10 points
	Volatility     float64 `json:"volatility"`      // 0-10 points
	Volume         float64 `json:"volume"`          // 0-5 points
	TotalScore     float64 `json:"total_score"`     // 0-40 points

	// Segment results for detailed breakdown
	TrendResult      *SegmentResult `json:"trend_result,omitempty"`
	MomentumResult   *SegmentResult `json:"momentum_result,omitempty"`
	VolatilityResult *SegmentResult `json:"volatility_result,omitempty"`
	VolumeResult     *SegmentResult `json:"volume_result,omitempty"`
}

// CalculateTechnicalComponents computes the technical score components from CoinState.
// This method is designed to integrate with the existing ScoreCalculator.
// This method is thread-safe.
func (si *ScoreIntegration) CalculateTechnicalComponents(state *decision.CoinState) (*TechnicalScoreComponents, error) {
	if state == nil {
		return &TechnicalScoreComponents{
			TrendAlignment: 0,
			Momentum:       0,
			Volatility:     0,
			Volume:         0,
			TotalScore:     0,
		}, nil
	}

	// Build IndicatorData from CoinState
	data := si.buildIndicatorDataFromCoinState(state)

	// Get settings with read lock
	si.mu.RLock()
	settings := si.settings
	si.mu.RUnlock()

	// Calculate all segments
	results, err := si.calculator.CalculateAllFromSettings(settings, data)
	if err != nil {
		// Return defaults on error
		return &TechnicalScoreComponents{
			TrendAlignment: decision.MaxTrendAlignmentScore / 2,
			Momentum:       decision.MaxMomentumScore / 2,
			Volatility:     decision.MaxVolatilityScore / 2,
			Volume:         decision.MaxVolumeScore / 2,
			TotalScore:     decision.MaxTechnicalScore / 2,
		}, err
	}

	components := &TechnicalScoreComponents{}

	// Map segment scores to technical components
	// Each segment score is 0-100, map to the component max score

	// Trend segment -> TrendAlignment (0-15)
	if trendResult, ok := results[SegmentTrend]; ok && trendResult != nil {
		components.TrendAlignment = (trendResult.Score / 100) * decision.MaxTrendAlignmentScore
		components.TrendResult = trendResult
	} else {
		components.TrendAlignment = decision.MaxTrendAlignmentScore / 2 // Default to neutral
	}

	// Momentum segment -> Momentum (0-10)
	if momentumResult, ok := results[SegmentMomentum]; ok && momentumResult != nil {
		components.Momentum = (momentumResult.Score / 100) * decision.MaxMomentumScore
		components.MomentumResult = momentumResult
	} else {
		components.Momentum = decision.MaxMomentumScore / 2
	}

	// Volatility segment -> Volatility (0-10)
	if volatilityResult, ok := results[SegmentVolatility]; ok && volatilityResult != nil {
		components.Volatility = (volatilityResult.Score / 100) * decision.MaxVolatilityScore
		components.VolatilityResult = volatilityResult
	} else {
		components.Volatility = decision.MaxVolatilityScore / 2
	}

	// Volume segment -> Volume (0-5)
	if volumeResult, ok := results[SegmentVolume]; ok && volumeResult != nil {
		components.Volume = (volumeResult.Score / 100) * decision.MaxVolumeScore
		components.VolumeResult = volumeResult
	} else {
		components.Volume = decision.MaxVolumeScore / 2
	}

	// Calculate total
	components.TotalScore = components.TrendAlignment + components.Momentum + components.Volatility + components.Volume

	return components, nil
}

// buildIndicatorDataFromCoinState creates IndicatorData from CoinState.
// This extracts the available indicator values from CoinState for use by the indicator framework.
func (si *ScoreIntegration) buildIndicatorDataFromCoinState(state *decision.CoinState) *IndicatorData {
	data := NewIndicatorData(state.Symbol)
	data.Price = state.Price

	// Map CoinState fields to OtherValues for indicators
	if state.ADX > 0 {
		data.SetOtherValue("adx", state.ADX)
	}
	if state.ATR > 0 {
		data.SetOtherValue("atr", state.ATR)
	}
	if state.ATRAverage > 0 {
		data.SetOtherValue("atr_average", state.ATRAverage)
	}
	if state.RSI > 0 {
		data.SetOtherValue("rsi", state.RSI)
	}
	if state.EMA9 > 0 {
		data.SetOtherValue("ema_9", state.EMA9)
	}
	if state.EMA21 > 0 {
		data.SetOtherValue("ema_21", state.EMA21)
	}

	// Map trend directions to numeric values
	// BULLISH = 1, NEUTRAL = 0, BEARISH = -1
	data.SetOtherValue("trend_1h", trendToValue(state.Trend1H))
	data.SetOtherValue("trend_15m", trendToValue(state.Trend15M))

	return data
}

// trendToValue converts TrendDirection to a numeric value.
func trendToValue(trend decision.TrendDirection) float64 {
	switch trend {
	case decision.TrendBullish:
		return 1.0
	case decision.TrendBearish:
		return -1.0
	default:
		return 0.0
	}
}

// CreateComponentScore creates a ComponentScore for the technical dimension.
// This is designed to be used directly with ScoreBreakdown.AddComponent().
func (si *ScoreIntegration) CreateComponentScore(state *decision.CoinState) (*decision.ComponentScore, error) {
	cs := decision.NewComponentScore(decision.DimensionTechnical, decision.MaxTechnicalScore)

	components, err := si.CalculateTechnicalComponents(state)
	if err != nil {
		// Set default score on error
		cs.SetScore(decision.MaxTechnicalScore / 2)
		return cs, err
	}

	// Add sub-component details
	cs.Details["trend_alignment"] = components.TrendAlignment
	cs.Details["momentum"] = components.Momentum
	cs.Details["volatility"] = components.Volatility
	cs.Details["volume"] = components.Volume

	// Add segment breakdown if available
	if components.TrendResult != nil {
		for name, score := range components.TrendResult.IndividualScores {
			cs.Details["trend_"+name] = score
		}
	}
	if components.MomentumResult != nil {
		for name, score := range components.MomentumResult.IndividualScores {
			cs.Details["momentum_"+name] = score
		}
	}
	if components.VolatilityResult != nil {
		for name, score := range components.VolatilityResult.IndividualScores {
			cs.Details["volatility_"+name] = score
		}
	}
	if components.VolumeResult != nil {
		for name, score := range components.VolumeResult.IndividualScores {
			cs.Details["volume_"+name] = score
		}
	}

	cs.SetScore(components.TotalScore)

	return cs, nil
}

// ValidateUserSelection validates that the user's indicator selections are valid.
func (si *ScoreIntegration) ValidateUserSelection(segment IndicatorSegment, indicators []string) error {
	return si.calculator.ValidateIndicatorSelection(segment, indicators)
}

// GetAvailableIndicators returns all available indicators for a segment.
func (si *ScoreIntegration) GetAvailableIndicators(segment IndicatorSegment) []*IndicatorInfo {
	return si.calculator.GetAvailableIndicatorsForSegment(segment)
}

// GetAllAvailableIndicators returns all registered indicators grouped by segment.
func (si *ScoreIntegration) GetAllAvailableIndicators() map[IndicatorSegment][]*IndicatorInfo {
	result := make(map[IndicatorSegment][]*IndicatorInfo)

	for _, segment := range AllSegments() {
		result[segment] = si.calculator.GetAvailableIndicatorsForSegment(segment)
	}

	return result
}
