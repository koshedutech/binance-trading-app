// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"math"
)

// RSIIndicator wraps the Relative Strength Index.
type RSIIndicator struct {
	BaseIndicator
	Period int
}

// NewRSIIndicator creates a new RSI indicator with default 14 period.
func NewRSIIndicator() *RSIIndicator {
	return &RSIIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "rsi",
			IndicatorSegment: SegmentMomentum,
			IndicatorDeps:    []string{},
		},
		Period: 14,
	}
}

// Description returns a human-readable explanation.
func (r *RSIIndicator) Description() string {
	return fmt.Sprintf("RSI %d period. Oversold <30, overbought >70, optimal 40-60.", r.Period)
}

// Calculate computes the RSI value.
func (r *RSIIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated RSI
	if rsi, hasRSI := data.GetOtherValue("rsi"); hasRSI {
		return NewIndicatorValue(r.Name(), rsi, r.Normalize(rsi)), nil
	}

	// Calculate from prices
	if !data.HasSufficientData(r.Period + 1) {
		return nil, fmt.Errorf("insufficient data for RSI calculation: need %d periods", r.Period+1)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	rsi := calculateRSI(prices, r.Period)
	normalized := r.Normalize(rsi)

	return NewIndicatorValue(r.Name(), rsi, normalized), nil
}

// Normalize converts RSI to 0-100 scoring scale.
// For entry scoring, optimal range is 40-60 (not overbought/oversold).
func (r *RSIIndicator) Normalize(value float64) float64 {
	// RSI is already 0-100, but we score differently:
	// Optimal entry: RSI 40-60 = 100 points
	// Moderate: RSI 30-40 or 60-70 = 70 points
	// Extended: RSI 20-30 or 70-80 = 40 points
	// Extreme: RSI <20 or >80 = 20 points

	if value >= 40 && value <= 60 {
		return 100
	}
	if (value >= 30 && value < 40) || (value > 60 && value <= 70) {
		return 70
	}
	if (value >= 20 && value < 30) || (value > 70 && value <= 80) {
		return 40
	}
	return 20
}

// StochasticIndicator calculates Stochastic %K and %D oscillator.
type StochasticIndicator struct {
	BaseIndicator
	KPeriod int
	DPeriod int
}

// NewStochasticIndicator creates a new Stochastic indicator.
func NewStochasticIndicator() *StochasticIndicator {
	return &StochasticIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "stochastic",
			IndicatorSegment: SegmentMomentum,
			IndicatorDeps:    []string{},
		},
		KPeriod: 14,
		DPeriod: 3,
	}
}

// Description returns a human-readable explanation.
func (s *StochasticIndicator) Description() string {
	return fmt.Sprintf("Stochastic %%K(%d)/%%D(%d). Oversold <20, overbought >80.", s.KPeriod, s.DPeriod)
}

// Calculate computes the Stochastic %K value.
func (s *StochasticIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated Stochastic
	if stoch, hasStoch := data.GetOtherValue("stochastic_k"); hasStoch {
		return NewIndicatorValue(s.Name(), stoch, s.Normalize(stoch)), nil
	}

	if !data.HasSufficientData(s.KPeriod) {
		return nil, fmt.Errorf("insufficient data for Stochastic calculation: need %d periods", s.KPeriod)
	}

	// Need highs, lows, and closes
	if len(data.Highs) < s.KPeriod || len(data.Lows) < s.KPeriod {
		return nil, fmt.Errorf("high/low data required for Stochastic calculation")
	}

	closes := data.Closes
	if len(closes) == 0 {
		closes = data.Prices
	}

	// Calculate %K = (Current Close - Lowest Low) / (Highest High - Lowest Low) * 100
	n := len(closes)
	start := n - s.KPeriod

	var highestHigh, lowestLow float64
	highestHigh = data.Highs[start]
	lowestLow = data.Lows[start]

	for i := start; i < n; i++ {
		if data.Highs[i] > highestHigh {
			highestHigh = data.Highs[i]
		}
		if data.Lows[i] < lowestLow {
			lowestLow = data.Lows[i]
		}
	}

	var percentK float64
	if highestHigh > lowestLow {
		currentClose := closes[n-1]
		percentK = (currentClose - lowestLow) / (highestHigh - lowestLow) * 100
	} else {
		percentK = 50 // Default to neutral if no range
	}

	normalized := s.Normalize(percentK)

	return NewIndicatorValue(s.Name(), percentK, normalized), nil
}

// Normalize converts Stochastic to scoring scale.
// Similar to RSI - optimal range is 20-80.
func (s *StochasticIndicator) Normalize(value float64) float64 {
	// Optimal: 20-80 range (not extreme)
	// Full score at 50 (perfectly neutral)
	// Lower score at extremes

	if value >= 20 && value <= 80 {
		// Score based on distance from center
		distance := math.Abs(value - 50)
		return 100 - (distance / 30 * 30) // 70-100 range
	}
	if value < 20 {
		return value / 20 * 40 // 0-40 range
	}
	return (100 - value) / 20 * 40 // 0-40 range
}

// CCIIndicator calculates the Commodity Channel Index.
type CCIIndicator struct {
	BaseIndicator
	Period int
}

// NewCCIIndicator creates a new CCI indicator.
func NewCCIIndicator() *CCIIndicator {
	return &CCIIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "cci",
			IndicatorSegment: SegmentMomentum,
			IndicatorDeps:    []string{},
		},
		Period: 20,
	}
}

// Description returns a human-readable explanation.
func (c *CCIIndicator) Description() string {
	return fmt.Sprintf("CCI %d period. Oversold <-100, overbought >100, neutral -100 to 100.", c.Period)
}

// Calculate computes the CCI value.
func (c *CCIIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated CCI
	if cci, hasCCI := data.GetOtherValue("cci"); hasCCI {
		return NewIndicatorValue(c.Name(), cci, c.Normalize(cci)), nil
	}

	if !data.HasSufficientData(c.Period) {
		return nil, fmt.Errorf("insufficient data for CCI calculation: need %d periods", c.Period)
	}

	// Need high, low, close
	if len(data.Highs) < c.Period || len(data.Lows) < c.Period {
		return nil, fmt.Errorf("high/low data required for CCI calculation")
	}

	closes := data.Closes
	if len(closes) == 0 {
		closes = data.Prices
	}

	n := len(closes)
	start := n - c.Period

	// Calculate Typical Price (TP) = (High + Low + Close) / 3
	typicalPrices := make([]float64, c.Period)
	for i := 0; i < c.Period; i++ {
		idx := start + i
		typicalPrices[i] = (data.Highs[idx] + data.Lows[idx] + closes[idx]) / 3
	}

	// Calculate SMA of TP
	var sumTP float64
	for _, tp := range typicalPrices {
		sumTP += tp
	}
	smaTP := sumTP / float64(c.Period)

	// Calculate Mean Deviation
	var sumDev float64
	for _, tp := range typicalPrices {
		sumDev += math.Abs(tp - smaTP)
	}
	meanDev := sumDev / float64(c.Period)

	// Calculate CCI = (TP - SMA(TP)) / (0.015 * Mean Deviation)
	currentTP := typicalPrices[c.Period-1]
	var cci float64
	if meanDev > 0 {
		cci = (currentTP - smaTP) / (0.015 * meanDev)
	}

	normalized := c.Normalize(cci)

	return NewIndicatorValue(c.Name(), cci, normalized), nil
}

// Normalize converts CCI to 0-100 scoring scale.
func (c *CCIIndicator) Normalize(value float64) float64 {
	// CCI typically ranges from -200 to +200
	// Neutral zone: -100 to +100 = optimal for entries

	if value >= -100 && value <= 100 {
		// In neutral zone - score based on centrality
		return 60 + (1-math.Abs(value)/100)*40 // 60-100 range
	}
	if value < -100 {
		// Oversold - decreasing score
		return math.Max(0, 60+value+100) // 0-60 range
	}
	// Overbought - decreasing score
	return math.Max(0, 60-(value-100)) // 0-60 range
}

// MomentumIndicator calculates simple price momentum.
type MomentumIndicator struct {
	BaseIndicator
	Period int
}

// NewMomentumIndicator creates a new momentum indicator.
func NewMomentumIndicator() *MomentumIndicator {
	return &MomentumIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "momentum",
			IndicatorSegment: SegmentMomentum,
			IndicatorDeps:    []string{},
		},
		Period: 10,
	}
}

// Description returns a human-readable explanation.
func (m *MomentumIndicator) Description() string {
	return fmt.Sprintf("Price momentum over %d periods. Positive = bullish, negative = bearish.", m.Period)
}

// Calculate computes the momentum value.
func (m *MomentumIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	if !data.HasSufficientData(m.Period + 1) {
		return nil, fmt.Errorf("insufficient data for momentum calculation: need %d periods", m.Period+1)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	n := len(prices)
	currentPrice := prices[n-1]
	pastPrice := prices[n-1-m.Period]

	// Momentum as percentage change
	var momentum float64
	if pastPrice > 0 {
		momentum = (currentPrice - pastPrice) / pastPrice * 100
	}

	normalized := m.Normalize(momentum)

	return NewIndicatorValue(m.Name(), momentum, normalized), nil
}

// Normalize converts momentum to 0-100 scale.
func (m *MomentumIndicator) Normalize(value float64) float64 {
	// Momentum typically ranges from -10% to +10% for most timeframes
	// Map to 0-100

	const maxMomentum = 10.0

	if value >= maxMomentum {
		return 100
	}
	if value <= -maxMomentum {
		return 0
	}

	return 50 + (value/maxMomentum)*50
}

// Helper function to calculate RSI
func calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50 // Return neutral if insufficient data
	}

	var gains, losses float64
	n := len(prices)

	// Calculate initial average gain/loss
	for i := n - period; i < n; i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change // Make positive
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100 // All gains, max RSI
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// init registers momentum indicators with the global registry.
func init() {
	registry := GetGlobalIndicatorRegistry()

	// Register all momentum indicators, log errors if any
	indicators := []Indicator{
		NewRSIIndicator(),
		NewStochasticIndicator(),
		NewCCIIndicator(),
		NewMomentumIndicator(),
	}

	for _, ind := range indicators {
		if err := registry.Register(ind); err != nil {
			// Log registration errors - in production this would use a proper logger
			// For now, panic on startup to surface misconfigurations early
			panic(fmt.Sprintf("failed to register momentum indicator %s: %v", ind.Name(), err))
		}
	}
}
