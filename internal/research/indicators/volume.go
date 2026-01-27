// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Volume Features
package indicators

// VolumeFeatures holds all volume-related features for a candle.
// These features capture volume dynamics and order flow characteristics.
type VolumeFeatures struct {
	// Volume ratios vs historical averages
	VolumeRatio5  float64 `json:"volume_ratio_5"`  // Current volume / 5-period SMA
	VolumeRatio10 float64 `json:"volume_ratio_10"` // Current volume / 10-period SMA
	VolumeRatio20 float64 `json:"volume_ratio_20"` // Current volume / 20-period SMA

	// Volume trend
	VolumeTrend5 float64 `json:"volume_trend_5"` // Volume change over 5 candles (%)

	// Order flow analysis
	TakerBuyRatio float64 `json:"taker_buy_ratio"` // Taker buy volume / Total volume (0-1)

	// Trade activity
	TradeCountValue float64 `json:"trade_count_value"` // Normalized trade count
	AvgTradeSize    float64 `json:"avg_trade_size"`    // Average trade size (volume/trade_count)

	// Accumulation/Distribution indicators
	OBV            float64 `json:"obv"`               // On-Balance Volume
	OBVTrend       float64 `json:"obv_trend"`         // OBV change over period (%)
	VolumePriceTrend float64 `json:"volume_price_trend"` // VPT indicator
	AccumDist      float64 `json:"accum_dist"`        // Accumulation/Distribution
	MoneyFlow      float64 `json:"money_flow"`        // Money Flow Index input (typical price * volume)
}

// CalculateVolumeRatio calculates current volume relative to average.
// Returns NaN if insufficient data or zero average.
func CalculateVolumeRatio(volumes []float64, period int) float64 {
	if len(volumes) < period+1 {
		return NaN
	}

	// Calculate average of previous 'period' candles (excluding current)
	startIdx := len(volumes) - period - 1
	endIdx := len(volumes) - 1

	sum := 0.0
	for i := startIdx; i < endIdx; i++ {
		sum += volumes[i]
	}
	avg := sum / float64(period)

	if avg == 0 {
		return NaN
	}

	currentVolume := volumes[len(volumes)-1]
	return currentVolume / avg
}

// CalculateVolumeTrend calculates volume percentage change over a period.
func CalculateVolumeTrend(volumes []float64, period int) float64 {
	if len(volumes) < period+1 {
		return NaN
	}

	currentIdx := len(volumes) - 1
	pastIdx := currentIdx - period

	return SafePercent(volumes[currentIdx], volumes[pastIdx])
}

// CalculateTakerBuyRatio calculates the taker buy ratio.
func CalculateTakerBuyRatio(totalVolume, takerBuyVolume float64) float64 {
	if totalVolume == 0 {
		return NaN
	}
	return takerBuyVolume / totalVolume
}

// CalculateAvgTradeSize calculates average trade size.
func CalculateAvgTradeSize(volume float64, tradeCount int) float64 {
	if tradeCount == 0 {
		return NaN
	}
	return volume / float64(tradeCount)
}

// CalculateOBV calculates On-Balance Volume.
// OBV adds volume on up days, subtracts on down days.
func CalculateOBV(closes, volumes []float64) float64 {
	if len(closes) < 2 || len(volumes) < 2 || len(closes) != len(volumes) {
		return NaN
	}

	obv := 0.0
	for i := 1; i < len(closes); i++ {
		if closes[i] > closes[i-1] {
			obv += volumes[i]
		} else if closes[i] < closes[i-1] {
			obv -= volumes[i]
		}
		// If closes[i] == closes[i-1], OBV unchanged
	}

	return obv
}

// CalculateOBVTrend calculates the percentage change in OBV over a period.
func CalculateOBVTrend(closes, volumes []float64, period int) float64 {
	if len(closes) < period+2 || len(volumes) < period+2 {
		return NaN
	}

	// Calculate current OBV
	currentOBV := CalculateOBV(closes, volumes)
	if IsNaN(currentOBV) {
		return NaN
	}

	// Calculate OBV at 'period' candles ago
	pastCloses := closes[:len(closes)-period]
	pastVolumes := volumes[:len(volumes)-period]
	pastOBV := CalculateOBV(pastCloses, pastVolumes)
	if IsNaN(pastOBV) {
		return NaN
	}

	// Calculate percentage change
	if pastOBV == 0 {
		if currentOBV == 0 {
			return 0
		}
		return NaN
	}

	return (currentOBV - pastOBV) / Abs(pastOBV) * 100
}

// CalculateVolumePriceTrend calculates the Volume Price Trend indicator.
// VPT = Previous VPT + Volume * ((Close - Previous Close) / Previous Close)
func CalculateVolumePriceTrend(closes, volumes []float64) float64 {
	if len(closes) < 2 || len(volumes) < 2 || len(closes) != len(volumes) {
		return NaN
	}

	vpt := 0.0
	for i := 1; i < len(closes); i++ {
		if closes[i-1] != 0 {
			priceChange := (closes[i] - closes[i-1]) / closes[i-1]
			vpt += volumes[i] * priceChange
		}
	}

	return vpt
}

// CalculateAccumulationDistribution calculates the A/D line value for a single candle.
// ADL = ((Close - Low) - (High - Close)) / (High - Low) * Volume
// Also known as the Money Flow Multiplier * Volume
func CalculateAccumulationDistribution(high, low, close, volume float64) float64 {
	priceRange := high - low
	if priceRange == 0 {
		return 0
	}

	// Money Flow Multiplier (CLV - Close Location Value)
	mfm := ((close - low) - (high - close)) / priceRange

	return mfm * volume
}

// CalculateAccumulationDistributionLine calculates cumulative A/D line.
func CalculateAccumulationDistributionLine(highs, lows, closes, volumes []float64) float64 {
	if len(highs) == 0 || len(lows) == 0 || len(closes) == 0 || len(volumes) == 0 {
		return NaN
	}

	n := len(closes)
	if len(highs) != n || len(lows) != n || len(volumes) != n {
		return NaN
	}

	adl := 0.0
	for i := 0; i < n; i++ {
		adl += CalculateAccumulationDistribution(highs[i], lows[i], closes[i], volumes[i])
	}

	return adl
}

// CalculateMoneyFlow calculates the raw money flow (typical price * volume).
// This is the input to MFI calculation.
func CalculateMoneyFlow(high, low, close, volume float64) float64 {
	typicalPrice := (high + low + close) / 3
	return typicalPrice * volume
}

// CalculateVolumeFeatures calculates all volume features for the current candle.
func CalculateVolumeFeatures(candles []OHLCV) *VolumeFeatures {
	if len(candles) == 0 {
		return &VolumeFeatures{
			VolumeRatio5:     NaN,
			VolumeRatio10:    NaN,
			VolumeRatio20:    NaN,
			VolumeTrend5:     NaN,
			TakerBuyRatio:    NaN,
			AvgTradeSize:     NaN,
			OBV:              NaN,
			OBVTrend:         NaN,
			VolumePriceTrend: NaN,
			AccumDist:        NaN,
			MoneyFlow:        NaN,
		}
	}

	// Extract data arrays
	n := len(candles)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)
	volumes := make([]float64, n)

	for i, c := range candles {
		closes[i] = c.Close
		highs[i] = c.High
		lows[i] = c.Low
		volumes[i] = c.Volume
	}

	current := candles[n-1]

	features := &VolumeFeatures{
		// Volume ratios
		VolumeRatio5:  CalculateVolumeRatio(volumes, 5),
		VolumeRatio10: CalculateVolumeRatio(volumes, 10),
		VolumeRatio20: CalculateVolumeRatio(volumes, 20),

		// Volume trend
		VolumeTrend5: CalculateVolumeTrend(volumes, 5),

		// Order flow
		TakerBuyRatio: CalculateTakerBuyRatio(current.Volume, current.TakerBuyVolume),

		// Trade activity
		TradeCountValue: float64(current.TradeCount),
		AvgTradeSize:    CalculateAvgTradeSize(current.Volume, current.TradeCount),

		// A/D indicators
		OBV:              CalculateOBV(closes, volumes),
		OBVTrend:         CalculateOBVTrend(closes, volumes, 5),
		VolumePriceTrend: CalculateVolumePriceTrend(closes, volumes),
		AccumDist:        CalculateAccumulationDistributionLine(highs, lows, closes, volumes),
		MoneyFlow:        CalculateMoneyFlow(current.High, current.Low, current.Close, current.Volume),
	}

	return features
}
