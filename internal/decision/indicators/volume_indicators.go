// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"fmt"
	"math"
)

// OBVIndicator calculates On-Balance Volume trend.
type OBVIndicator struct {
	BaseIndicator
	Period int
}

// NewOBVIndicator creates a new OBV indicator.
func NewOBVIndicator() *OBVIndicator {
	return &OBVIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "obv",
			IndicatorSegment: SegmentVolume,
			IndicatorDeps:    []string{},
		},
		Period: 20,
	}
}

// Description returns a human-readable explanation.
func (o *OBVIndicator) Description() string {
	return fmt.Sprintf("On-Balance Volume trend over %d periods. Rising = bullish volume, falling = bearish.", o.Period)
}

// Calculate computes the OBV trend direction.
func (o *OBVIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated OBV
	if obvTrend, hasOBV := data.GetOtherValue("obv_trend"); hasOBV {
		return NewIndicatorValue(o.Name(), obvTrend, o.Normalize(obvTrend)), nil
	}

	if !data.HasSufficientData(o.Period) {
		return nil, fmt.Errorf("insufficient data for OBV calculation: need %d periods", o.Period)
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	n := len(prices)

	// Ensure volumes array matches prices length for safe indexing
	if len(data.Volumes) < n {
		// Return neutral if insufficient volume data
		return NewIndicatorValue(o.Name(), 0, 50).WithConfidence(0.5).WithSource("no_volume_data"), nil
	}

	start := n - o.Period

	// Calculate OBV
	obv := 0.0
	obvStart := 0.0
	for i := start + 1; i < n; i++ {
		if prices[i] > prices[i-1] {
			obv += data.Volumes[i]
		} else if prices[i] < prices[i-1] {
			obv -= data.Volumes[i]
		}
		// If price unchanged, OBV stays the same

		if i == start+1 {
			obvStart = obv
		}
	}

	// Calculate OBV trend as percentage change
	var obvTrend float64
	if math.Abs(obvStart) > 0 {
		obvTrend = (obv - obvStart) / math.Abs(obvStart) * 100
	}

	normalized := o.Normalize(obvTrend)

	return NewIndicatorValue(o.Name(), obvTrend, normalized), nil
}

// Normalize converts OBV trend to 0-100 scale.
func (o *OBVIndicator) Normalize(value float64) float64 {
	// OBV trend can range significantly; use -50% to +50% as typical range
	const maxTrend = 50.0

	if value >= maxTrend {
		return 100
	}
	if value <= -maxTrend {
		return 0
	}

	return 50 + (value/maxTrend)*50
}

// VolumeSMAIndicator calculates volume ratio relative to its SMA.
type VolumeSMAIndicator struct {
	BaseIndicator
	Period int
}

// NewVolumeSMAIndicator creates a new Volume SMA ratio indicator.
func NewVolumeSMAIndicator() *VolumeSMAIndicator {
	return &VolumeSMAIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "volume_sma",
			IndicatorSegment: SegmentVolume,
			IndicatorDeps:    []string{},
		},
		Period: 20,
	}
}

// Description returns a human-readable explanation.
func (v *VolumeSMAIndicator) Description() string {
	return fmt.Sprintf("Volume vs %d-period SMA ratio. >1 = above average, <1 = below average.", v.Period)
}

// Calculate computes the volume to SMA ratio.
func (v *VolumeSMAIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated ratio
	if ratio, hasRatio := data.GetOtherValue("volume_ratio"); hasRatio {
		return NewIndicatorValue(v.Name(), ratio, v.Normalize(ratio)), nil
	}

	if len(data.Volumes) < v.Period {
		// Return neutral if no volume data
		return NewIndicatorValue(v.Name(), 1.0, 50).WithConfidence(0.5).WithSource("no_volume_data"), nil
	}

	n := len(data.Volumes)
	start := n - v.Period

	// Calculate SMA of volume
	var sumVolume float64
	for i := start; i < n; i++ {
		sumVolume += data.Volumes[i]
	}
	smaVolume := sumVolume / float64(v.Period)

	// Calculate ratio of current volume to SMA
	currentVolume := data.Volumes[n-1]
	var ratio float64
	if smaVolume > 0 {
		ratio = currentVolume / smaVolume
	} else {
		ratio = 1.0 // Default to neutral
	}

	normalized := v.Normalize(ratio)

	return NewIndicatorValue(v.Name(), ratio, normalized), nil
}

// Normalize converts volume ratio to 0-100 scale.
func (v *VolumeSMAIndicator) Normalize(value float64) float64 {
	// Ratio typically ranges from 0.2 to 3.0
	// 1.0 = average = 50 points
	// Higher = more confirmation = higher score

	if value <= 0.2 {
		return 10
	}
	if value >= 3.0 {
		return 100
	}

	if value < 1.0 {
		// Below average: 10-50 range
		return 10 + (value-0.2)/(1.0-0.2)*40
	}
	// Above average: 50-100 range
	return 50 + (value-1.0)/(3.0-1.0)*50
}

// VWAPIndicator calculates price position relative to VWAP.
type VWAPIndicator struct {
	BaseIndicator
}

// NewVWAPIndicator creates a new VWAP proximity indicator.
func NewVWAPIndicator() *VWAPIndicator {
	return &VWAPIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "vwap",
			IndicatorSegment: SegmentVolume,
			IndicatorDeps:    []string{},
		},
	}
}

// Description returns a human-readable explanation.
func (v *VWAPIndicator) Description() string {
	return "Price position relative to VWAP. Above VWAP = bullish, below = bearish."
}

// Calculate computes the VWAP proximity.
func (v *VWAPIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	// Check for pre-calculated VWAP
	if vwap, hasVWAP := data.GetOtherValue("vwap"); hasVWAP {
		// Calculate distance from VWAP as percentage
		var distance float64
		if vwap > 0 {
			distance = (data.Price - vwap) / vwap * 100
		}
		return NewIndicatorValue(v.Name(), distance, v.Normalize(distance)), nil
	}

	// Calculate VWAP from available data
	if len(data.Volumes) == 0 {
		// Return neutral if no volume data
		return NewIndicatorValue(v.Name(), 0, 50).WithConfidence(0.5).WithSource("no_volume_data"), nil
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	if len(prices) != len(data.Volumes) {
		return nil, fmt.Errorf("price and volume data length mismatch")
	}

	// Calculate VWAP = sum(price * volume) / sum(volume)
	var sumPV, sumV float64
	for i := 0; i < len(prices); i++ {
		sumPV += prices[i] * data.Volumes[i]
		sumV += data.Volumes[i]
	}

	var vwap float64
	if sumV > 0 {
		vwap = sumPV / sumV
	} else {
		vwap = data.Price // Default to current price
	}

	// Calculate distance from VWAP as percentage
	var distance float64
	if vwap > 0 {
		distance = (data.Price - vwap) / vwap * 100
	}

	normalized := v.Normalize(distance)

	return NewIndicatorValue(v.Name(), distance, normalized), nil
}

// Normalize converts VWAP distance to 0-100 scale.
func (v *VWAPIndicator) Normalize(value float64) float64 {
	// Distance from VWAP typically -3% to +3%
	// At VWAP (0) = 50 points (neutral)
	// Above VWAP = higher score, below = lower score

	const maxDistance = 3.0

	if value >= maxDistance {
		return 100
	}
	if value <= -maxDistance {
		return 0
	}

	return 50 + (value/maxDistance)*50
}

// VolumeProfileIndicator calculates volume profile position.
type VolumeProfileIndicator struct {
	BaseIndicator
	Period int
}

// NewVolumeProfileIndicator creates a new volume profile indicator.
func NewVolumeProfileIndicator() *VolumeProfileIndicator {
	return &VolumeProfileIndicator{
		BaseIndicator: BaseIndicator{
			IndicatorName:    "volume_profile",
			IndicatorSegment: SegmentVolume,
			IndicatorDeps:    []string{},
		},
		Period: 50,
	}
}

// Description returns a human-readable explanation.
func (vp *VolumeProfileIndicator) Description() string {
	return fmt.Sprintf("Volume profile over %d periods. High volume price levels indicate support/resistance.", vp.Period)
}

// Calculate computes the volume profile score.
func (vp *VolumeProfileIndicator) Calculate(data *IndicatorData) (*IndicatorValue, error) {
	if data == nil {
		return nil, fmt.Errorf("indicator data is nil")
	}

	if len(data.Volumes) < vp.Period {
		// Return neutral if insufficient data
		return NewIndicatorValue(vp.Name(), 0.5, 50).WithConfidence(0.5).WithSource("insufficient_data"), nil
	}

	prices := data.Closes
	if len(prices) == 0 {
		prices = data.Prices
	}

	n := len(prices)
	start := n - vp.Period

	// Find price range
	var minPrice, maxPrice float64
	minPrice = prices[start]
	maxPrice = prices[start]

	for i := start; i < n; i++ {
		if prices[i] < minPrice {
			minPrice = prices[i]
		}
		if prices[i] > maxPrice {
			maxPrice = prices[i]
		}
	}

	priceRange := maxPrice - minPrice
	if priceRange <= 0 {
		return NewIndicatorValue(vp.Name(), 0.5, 50), nil
	}

	// Calculate volume at different price levels (simplified 10-bucket approach)
	buckets := make([]float64, 10)
	bucketSize := priceRange / 10

	for i := start; i < n; i++ {
		bucketIdx := int((prices[i] - minPrice) / bucketSize)
		if bucketIdx >= 10 {
			bucketIdx = 9
		}
		if bucketIdx < 0 {
			bucketIdx = 0
		}
		buckets[bucketIdx] += data.Volumes[i]
	}

	// Find which bucket current price is in
	currentBucket := int((data.Price - minPrice) / bucketSize)
	if currentBucket >= 10 {
		currentBucket = 9
	}
	if currentBucket < 0 {
		currentBucket = 0
	}

	// Find max volume bucket
	var maxVol float64
	var maxVolBucket int
	for i, vol := range buckets {
		if vol > maxVol {
			maxVol = vol
			maxVolBucket = i
		}
	}

	// Score based on proximity to high-volume zones
	// Being near high volume = support/resistance = good for entries
	distance := math.Abs(float64(currentBucket - maxVolBucket))
	proximityScore := 1.0 - (distance / 10.0)

	normalized := vp.Normalize(proximityScore)

	return NewIndicatorValue(vp.Name(), proximityScore, normalized), nil
}

// Normalize converts proximity score to 0-100 scale.
func (vp *VolumeProfileIndicator) Normalize(value float64) float64 {
	// Proximity score is 0-1, map to 0-100
	return value * 100
}

// init registers volume indicators with the global registry.
func init() {
	registry := GetGlobalIndicatorRegistry()

	// Register all volume indicators, log errors if any
	indicators := []Indicator{
		NewOBVIndicator(),
		NewVolumeSMAIndicator(),
		NewVWAPIndicator(),
		NewVolumeProfileIndicator(),
	}

	for _, ind := range indicators {
		if err := registry.Register(ind); err != nil {
			// Log registration errors - in production this would use a proper logger
			// For now, panic on startup to surface misconfigurations early
			panic(fmt.Sprintf("failed to register volume indicator %s: %v", ind.Name(), err))
		}
	}
}
