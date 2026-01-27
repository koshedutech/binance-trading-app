// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine - Momentum Features
package indicators

// MomentumFeatures holds all momentum-related features for a candle.
// These features capture price momentum and oscillator values.
type MomentumFeatures struct {
	// RSI - Relative Strength Index
	RSI7  float64 `json:"rsi_7"`  // 7-period RSI
	RSI14 float64 `json:"rsi_14"` // 14-period RSI
	RSI21 float64 `json:"rsi_21"` // 21-period RSI

	// Stochastic Oscillator
	StochK float64 `json:"stoch_k"` // Stochastic %K
	StochD float64 `json:"stoch_d"` // Stochastic %D (SMA of %K)

	// MACD
	MACDLine      float64 `json:"macd_line"`      // MACD line (12 EMA - 26 EMA)
	MACDSignal    float64 `json:"macd_signal"`    // Signal line (9 EMA of MACD)
	MACDHistogram float64 `json:"macd_histogram"` // MACD - Signal

	// Rate of Change
	ROC5  float64 `json:"roc_5"`  // 5-period ROC
	ROC10 float64 `json:"roc_10"` // 10-period ROC
	ROC20 float64 `json:"roc_20"` // 20-period ROC

	// Price Momentum
	Momentum5  float64 `json:"momentum_5"`  // 5-period price momentum
	Momentum10 float64 `json:"momentum_10"` // 10-period price momentum

	// Williams %R
	WilliamsR float64 `json:"williams_r"` // Williams %R (14 period)
}

// CalculateRSI calculates the Relative Strength Index.
// Uses the Wilder smoothing method.
func CalculateRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return NaN
	}

	// Calculate initial average gain and loss
	var gains, losses float64
	for i := 1; i <= period; i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change // Make positive
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	// Apply Wilder smoothing for remaining data
	for i := period + 1; i < len(closes); i++ {
		change := closes[i] - closes[i-1]

		var currentGain, currentLoss float64
		if change > 0 {
			currentGain = change
		} else {
			currentLoss = -change
		}

		avgGain = (avgGain*float64(period-1) + currentGain) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + currentLoss) / float64(period)
	}

	if avgLoss == 0 {
		if avgGain == 0 {
			return 50 // No movement, neutral
		}
		return 100 // Only gains
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

// CalculateStochastic calculates Stochastic Oscillator %K and %D.
// kPeriod: lookback for %K (typically 14)
// dPeriod: smoothing for %D (typically 3)
func CalculateStochastic(highs, lows, closes []float64, kPeriod, dPeriod int) (float64, float64) {
	n := len(closes)
	if n < kPeriod || len(highs) != n || len(lows) != n {
		return NaN, NaN
	}

	// Calculate %K values for the last dPeriod candles to compute %D
	kValues := make([]float64, 0, dPeriod)

	for offset := dPeriod - 1; offset >= 0; offset-- {
		endIdx := n - offset
		startIdx := endIdx - kPeriod
		if startIdx < 0 {
			continue
		}

		// Find highest high and lowest low
		highestHigh := highs[startIdx]
		lowestLow := lows[startIdx]

		for i := startIdx + 1; i < endIdx; i++ {
			if highs[i] > highestHigh {
				highestHigh = highs[i]
			}
			if lows[i] < lowestLow {
				lowestLow = lows[i]
			}
		}

		currentClose := closes[endIdx-1]
		priceRange := highestHigh - lowestLow

		var k float64
		if priceRange == 0 {
			k = 50 // No range, neutral
		} else {
			k = ((currentClose - lowestLow) / priceRange) * 100
		}
		kValues = append(kValues, k)
	}

	if len(kValues) == 0 {
		return NaN, NaN
	}

	// Current %K is the last value
	stochK := kValues[len(kValues)-1]

	// %D is SMA of %K values
	stochD := 0.0
	for _, k := range kValues {
		stochD += k
	}
	stochD /= float64(len(kValues))

	return stochK, stochD
}

// CalculateEMA calculates Exponential Moving Average.
func CalculateEMA(values []float64, period int) float64 {
	if len(values) < period {
		return NaN
	}

	// Calculate initial SMA
	sma := 0.0
	for i := 0; i < period; i++ {
		sma += values[i]
	}
	sma /= float64(period)

	// Calculate EMA
	multiplier := 2.0 / float64(period+1)
	ema := sma

	for i := period; i < len(values); i++ {
		ema = (values[i]-ema)*multiplier + ema
	}

	return ema
}

// CalculateSMA calculates Simple Moving Average.
func CalculateSMA(values []float64, period int) float64 {
	if len(values) < period {
		return NaN
	}

	startIdx := len(values) - period
	sum := 0.0
	for i := startIdx; i < len(values); i++ {
		sum += values[i]
	}

	return sum / float64(period)
}

// CalculateMACD calculates MACD, Signal, and Histogram.
// fastPeriod: typically 12, slowPeriod: typically 26, signalPeriod: typically 9
// Uses incremental EMA calculation for O(n) performance.
func CalculateMACD(closes []float64, fastPeriod, slowPeriod, signalPeriod int) (float64, float64, float64) {
	n := len(closes)
	if n < slowPeriod+signalPeriod {
		return NaN, NaN, NaN
	}

	// Calculate EMA multipliers
	fastMult := 2.0 / float64(fastPeriod+1)
	slowMult := 2.0 / float64(slowPeriod+1)
	signalMult := 2.0 / float64(signalPeriod+1)

	// Initialize fast EMA with SMA of first fastPeriod values
	fastEMA := 0.0
	for i := 0; i < fastPeriod; i++ {
		fastEMA += closes[i]
	}
	fastEMA /= float64(fastPeriod)

	// Initialize slow EMA with SMA of first slowPeriod values
	slowEMA := 0.0
	for i := 0; i < slowPeriod; i++ {
		slowEMA += closes[i]
	}
	slowEMA /= float64(slowPeriod)

	// Calculate MACD values incrementally
	macdValues := make([]float64, 0, n-slowPeriod+1)

	// Update EMAs from fastPeriod onwards (until slowPeriod-1)
	for i := fastPeriod; i < slowPeriod; i++ {
		fastEMA = (closes[i]-fastEMA)*fastMult + fastEMA
	}

	// Start collecting MACD values from slowPeriod onwards
	for i := slowPeriod; i < n; i++ {
		fastEMA = (closes[i]-fastEMA)*fastMult + fastEMA
		slowEMA = (closes[i]-slowEMA)*slowMult + slowEMA
		macdValues = append(macdValues, fastEMA-slowEMA)
	}

	if len(macdValues) < signalPeriod {
		return NaN, NaN, NaN
	}

	// Calculate signal line (EMA of MACD values)
	signalEMA := 0.0
	for i := 0; i < signalPeriod; i++ {
		signalEMA += macdValues[i]
	}
	signalEMA /= float64(signalPeriod)

	for i := signalPeriod; i < len(macdValues); i++ {
		signalEMA = (macdValues[i]-signalEMA)*signalMult + signalEMA
	}

	macdLine := macdValues[len(macdValues)-1]
	histogram := macdLine - signalEMA

	return macdLine, signalEMA, histogram
}

// CalculateROC calculates Rate of Change.
// ROC = ((Current - n periods ago) / n periods ago) * 100
func CalculateROC(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return NaN
	}

	currentIdx := len(closes) - 1
	pastIdx := currentIdx - period

	if closes[pastIdx] == 0 {
		return NaN
	}

	return ((closes[currentIdx] - closes[pastIdx]) / closes[pastIdx]) * 100
}

// CalculateMomentum calculates price momentum (difference from n periods ago).
func CalculateMomentum(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return NaN
	}

	currentIdx := len(closes) - 1
	pastIdx := currentIdx - period

	return closes[currentIdx] - closes[pastIdx]
}

// CalculateWilliamsR calculates Williams %R.
// %R = (Highest High - Close) / (Highest High - Lowest Low) * -100
func CalculateWilliamsR(highs, lows, closes []float64, period int) float64 {
	n := len(closes)
	if n < period || len(highs) != n || len(lows) != n {
		return NaN
	}

	startIdx := n - period

	// Find highest high and lowest low
	highestHigh := highs[startIdx]
	lowestLow := lows[startIdx]

	for i := startIdx + 1; i < n; i++ {
		if highs[i] > highestHigh {
			highestHigh = highs[i]
		}
		if lows[i] < lowestLow {
			lowestLow = lows[i]
		}
	}

	currentClose := closes[n-1]
	priceRange := highestHigh - lowestLow

	if priceRange == 0 {
		return -50 // No range, neutral
	}

	return ((highestHigh - currentClose) / priceRange) * -100
}

// CalculateMomentumFeatures calculates all momentum features.
func CalculateMomentumFeatures(candles []OHLCV) *MomentumFeatures {
	if len(candles) == 0 {
		return &MomentumFeatures{
			RSI7:          NaN,
			RSI14:         NaN,
			RSI21:         NaN,
			StochK:        NaN,
			StochD:        NaN,
			MACDLine:      NaN,
			MACDSignal:    NaN,
			MACDHistogram: NaN,
			ROC5:          NaN,
			ROC10:         NaN,
			ROC20:         NaN,
			Momentum5:     NaN,
			Momentum10:    NaN,
			WilliamsR:     NaN,
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

	// Calculate Stochastic
	stochK, stochD := CalculateStochastic(highs, lows, closes, 14, 3)

	// Calculate MACD
	macdLine, macdSignal, macdHist := CalculateMACD(closes, 12, 26, 9)

	features := &MomentumFeatures{
		// RSI
		RSI7:  CalculateRSI(closes, 7),
		RSI14: CalculateRSI(closes, 14),
		RSI21: CalculateRSI(closes, 21),

		// Stochastic
		StochK: stochK,
		StochD: stochD,

		// MACD
		MACDLine:      macdLine,
		MACDSignal:    macdSignal,
		MACDHistogram: macdHist,

		// ROC
		ROC5:  CalculateROC(closes, 5),
		ROC10: CalculateROC(closes, 10),
		ROC20: CalculateROC(closes, 20),

		// Momentum
		Momentum5:  CalculateMomentum(closes, 5),
		Momentum10: CalculateMomentum(closes, 10),

		// Williams %R
		WilliamsR: CalculateWilliamsR(highs, lows, closes, 14),
	}

	return features
}
