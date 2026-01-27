// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Trend Features
package indicators

// TrendFeatures holds all trend-related features for a candle.
// These features capture trend direction and strength.
type TrendFeatures struct {
	// Moving Averages
	SMA10 float64 `json:"sma_10"` // 10-period SMA
	SMA20 float64 `json:"sma_20"` // 20-period SMA
	SMA50 float64 `json:"sma_50"` // 50-period SMA
	EMA9  float64 `json:"ema_9"`  // 9-period EMA
	EMA21 float64 `json:"ema_21"` // 21-period EMA

	// Price vs Moving Average
	PriceVsSMA20 float64 `json:"price_vs_sma_20"` // Price distance from SMA20 (%)

	// Moving Average Crossovers
	SMACross int `json:"sma_cross"` // 1=bullish cross, -1=bearish cross, 0=none

	// ADX (Average Directional Index)
	ADX           float64 `json:"adx"`            // ADX value
	PlusDI        float64 `json:"plus_di"`        // +DI
	MinusDI       float64 `json:"minus_di"`       // -DI
	TrendStrength float64 `json:"trend_strength"` // ADX-based trend strength (0-1)

	// Trend direction
	TrendDirection int `json:"trend_direction"` // 1=up, -1=down, 0=sideways
}

// CalculatePriceVsSMA calculates price distance from SMA as percentage.
func CalculatePriceVsSMA(price, sma float64) float64 {
	if sma == 0 || IsNaN(sma) {
		return NaN
	}
	return ((price - sma) / sma) * 100
}

// CalculateSMACross detects SMA crossover.
// Returns 1 for bullish cross (fast crosses above slow), -1 for bearish, 0 for none.
func CalculateSMACross(closes []float64, fastPeriod, slowPeriod int) int {
	if len(closes) < slowPeriod+2 {
		return 0
	}

	// Calculate SMAs for current and previous candles
	// Current
	currentFast := CalculateSMA(closes, fastPeriod)
	currentSlow := CalculateSMA(closes, slowPeriod)

	// Previous (exclude last candle)
	prevCloses := closes[:len(closes)-1]
	prevFast := CalculateSMA(prevCloses, fastPeriod)
	prevSlow := CalculateSMA(prevCloses, slowPeriod)

	if IsNaN(currentFast) || IsNaN(currentSlow) || IsNaN(prevFast) || IsNaN(prevSlow) {
		return 0
	}

	// Check for crossover
	// Bullish: was below, now above
	if prevFast <= prevSlow && currentFast > currentSlow {
		return 1
	}
	// Bearish: was above, now below
	if prevFast >= prevSlow && currentFast < currentSlow {
		return -1
	}

	return 0
}

// CalculateDMI calculates the Directional Movement Index components.
// Returns +DI, -DI
func CalculateDMI(highs, lows, closes []float64, period int) (float64, float64) {
	n := len(closes)
	if n < period+1 || len(highs) != n || len(lows) != n {
		return NaN, NaN
	}

	// Calculate +DM, -DM, and TR for each candle
	plusDMSum := 0.0
	minusDMSum := 0.0
	trSum := 0.0

	startIdx := n - period

	for i := startIdx; i < n; i++ {
		// Calculate directional movement
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]

		plusDM := 0.0
		minusDM := 0.0

		if upMove > downMove && upMove > 0 {
			plusDM = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM = downMove
		}

		plusDMSum += plusDM
		minusDMSum += minusDM

		// Calculate True Range
		tr := CalculateTrueRange(highs[i], lows[i], closes[i-1])
		trSum += tr
	}

	if trSum == 0 {
		return 0, 0
	}

	// Calculate +DI and -DI
	plusDI := (plusDMSum / trSum) * 100
	minusDI := (minusDMSum / trSum) * 100

	return plusDI, minusDI
}

// CalculateADX calculates the Average Directional Index.
// ADX measures trend strength regardless of direction.
func CalculateADX(highs, lows, closes []float64, period int) float64 {
	n := len(closes)
	if n < period*2+1 || len(highs) != n || len(lows) != n {
		return NaN
	}

	// Calculate DX values over time for smoothing
	dxValues := make([]float64, 0, period)

	for offset := period; offset >= 0; offset-- {
		endIdx := n - offset
		if endIdx < period+1 {
			continue
		}

		subHighs := highs[:endIdx]
		subLows := lows[:endIdx]
		subCloses := closes[:endIdx]

		plusDI, minusDI := CalculateDMI(subHighs, subLows, subCloses, period)
		if IsNaN(plusDI) || IsNaN(minusDI) {
			continue
		}

		// Calculate DX
		diSum := plusDI + minusDI
		if diSum == 0 {
			dxValues = append(dxValues, 0)
		} else {
			dx := (Abs(plusDI-minusDI) / diSum) * 100
			dxValues = append(dxValues, dx)
		}
	}

	if len(dxValues) == 0 {
		return NaN
	}

	// ADX is the smoothed average of DX
	// Use Wilder smoothing (equivalent to EMA with period 2*n-1)
	adx := dxValues[0]
	for i := 1; i < len(dxValues); i++ {
		adx = (adx*float64(period-1) + dxValues[i]) / float64(period)
	}

	return adx
}

// CalculateTrendStrength calculates trend strength from ADX.
// Returns a normalized value between 0 and 1.
// ADX < 20 = weak, 20-40 = moderate, 40-60 = strong, > 60 = very strong
func CalculateTrendStrength(adx float64) float64 {
	if IsNaN(adx) {
		return NaN
	}

	// Normalize ADX to 0-1 range
	// Cap at 100 (rarely exceeded)
	if adx < 0 {
		return 0
	}
	if adx > 100 {
		return 1
	}

	return adx / 100
}

// CalculateTrendDirection determines trend direction based on price and MAs.
// Returns 1 for uptrend, -1 for downtrend, 0 for sideways.
func CalculateTrendDirection(closes []float64, shortPeriod, longPeriod int) int {
	if len(closes) < longPeriod {
		return 0
	}

	shortMA := CalculateSMA(closes, shortPeriod)
	longMA := CalculateSMA(closes, longPeriod)
	currentPrice := closes[len(closes)-1]

	if IsNaN(shortMA) || IsNaN(longMA) {
		return 0
	}

	// Uptrend: price > short MA > long MA
	if currentPrice > shortMA && shortMA > longMA {
		return 1
	}

	// Downtrend: price < short MA < long MA
	if currentPrice < shortMA && shortMA < longMA {
		return -1
	}

	return 0 // Sideways or mixed signals
}

// CalculateTrendFeatures calculates all trend features.
func CalculateTrendFeatures(candles []OHLCV) *TrendFeatures {
	if len(candles) == 0 {
		return &TrendFeatures{
			SMA10:          NaN,
			SMA20:          NaN,
			SMA50:          NaN,
			EMA9:           NaN,
			EMA21:          NaN,
			PriceVsSMA20:   NaN,
			SMACross:       0,
			ADX:            NaN,
			PlusDI:         NaN,
			MinusDI:        NaN,
			TrendStrength:  NaN,
			TrendDirection: 0,
		}
	}

	// Extract data arrays
	n := len(candles)
	closes := make([]float64, n)
	highs := make([]float64, n)
	lows := make([]float64, n)

	for i, c := range candles {
		closes[i] = c.Close
		highs[i] = c.High
		lows[i] = c.Low
	}

	currentPrice := closes[n-1]

	// Calculate Moving Averages
	sma10 := CalculateSMA(closes, 10)
	sma20 := CalculateSMA(closes, 20)
	sma50 := CalculateSMA(closes, 50)
	ema9 := CalculateEMA(closes, 9)
	ema21 := CalculateEMA(closes, 21)

	// Calculate DMI/ADX
	plusDI, minusDI := CalculateDMI(highs, lows, closes, 14)
	adx := CalculateADX(highs, lows, closes, 14)

	features := &TrendFeatures{
		// Moving Averages
		SMA10: sma10,
		SMA20: sma20,
		SMA50: sma50,
		EMA9:  ema9,
		EMA21: ema21,

		// Price vs MA
		PriceVsSMA20: CalculatePriceVsSMA(currentPrice, sma20),

		// Crossover
		SMACross: CalculateSMACross(closes, 10, 20),

		// ADX/DMI
		ADX:           adx,
		PlusDI:        plusDI,
		MinusDI:       minusDI,
		TrendStrength: CalculateTrendStrength(adx),

		// Trend direction (using EMA9/EMA21 for faster response)
		TrendDirection: CalculateTrendDirection(closes, 9, 21),
	}

	return features
}
