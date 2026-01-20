// Package autopilot provides automated trading functionality.
// Epic 10, Story 10.1: Trend-Based Exit for Classic Mode
package autopilot

import (
	"fmt"
	"log"
	"math"
	"time"
)

// ClassicDecisionSettings holds configuration for Classic Mode trend reversal detection.
// These settings control the technical indicator thresholds used to detect trend reversals.
type ClassicDecisionSettings struct {
	// ADX Settings
	ADXThreshold float64 `json:"adx_threshold"` // Minimum ADX for trend confirmation (default: 20)

	// RSI Settings
	RSIUpperBound float64 `json:"rsi_upper_bound"` // RSI level indicating overbought for LONG exit (default: 70)
	RSILowerBound float64 `json:"rsi_lower_bound"` // RSI level indicating oversold for SHORT exit (default: 30)
	RSITurningDown float64 `json:"rsi_turning_down"` // RSI decrease threshold to confirm turning (default: 5)
	RSITurningUp   float64 `json:"rsi_turning_up"`   // RSI increase threshold to confirm turning (default: 5)

	// EMA Settings
	EMAFastPeriod int `json:"ema_fast_period"` // Fast EMA period (default: 9)
	EMASlowPeriod int `json:"ema_slow_period"` // Slow EMA period (default: 21)

	// Confirmation Settings
	ConfirmationCount int  `json:"confirmation_count"` // Number of signals required to confirm reversal (default: 2)
	RequireAllSignals bool `json:"require_all_signals"` // If true, require all signals; if false, use confirmation_count

	// Lookback Settings
	LowerLowsLookback   int `json:"lower_lows_lookback"`   // Candles to check for lower lows pattern (default: 3)
	HigherHighsLookback int `json:"higher_highs_lookback"` // Candles to check for higher highs pattern (default: 3)

	// Timing
	MinHoldMinutes int `json:"min_hold_minutes"` // Minimum minutes before reversal exit is considered (default: 5)
}

// DefaultClassicDecisionSettings returns sensible defaults for Classic Mode.
func DefaultClassicDecisionSettings() *ClassicDecisionSettings {
	return &ClassicDecisionSettings{
		ADXThreshold:        20.0,
		RSIUpperBound:       70.0,
		RSILowerBound:       30.0,
		RSITurningDown:      5.0,
		RSITurningUp:        5.0,
		EMAFastPeriod:       9,
		EMASlowPeriod:       21,
		ConfirmationCount:   2,
		RequireAllSignals:   false,
		LowerLowsLookback:   3,
		HigherHighsLookback: 3,
		MinHoldMinutes:      5,
	}
}

// ClassicTrendIndicators holds calculated indicators for classic mode trend detection.
type ClassicTrendIndicators struct {
	// ADX and Directional Movement
	ADX    float64 `json:"adx"`
	PlusDI float64 `json:"plus_di"`
	MinusDI float64 `json:"minus_di"`
	DIFlipped bool `json:"di_flipped"` // True if DI has flipped against position direction

	// EMA Values
	EMA9  float64 `json:"ema_9"`
	EMA21 float64 `json:"ema_21"`
	EMABearishCross bool `json:"ema_bearish_cross"` // EMA9 < EMA21 (bearish for LONG)
	EMABullishCross bool `json:"ema_bullish_cross"` // EMA9 > EMA21 (bullish for SHORT)

	// RSI
	RSI         float64 `json:"rsi"`
	PreviousRSI float64 `json:"previous_rsi"`
	RSITurning  bool    `json:"rsi_turning"` // RSI turning against position direction

	// Price Patterns
	LowerLows   bool `json:"lower_lows"`   // Price making lower lows (bearish for LONG)
	HigherHighs bool `json:"higher_highs"` // Price making higher highs (bullish for SHORT)

	// Calculation timestamp
	CalculatedAt time.Time `json:"calculated_at"`
}

// ClassicExitSignal represents a signal component for exit decision.
type ClassicExitSignal struct {
	Name        string  `json:"name"`
	Triggered   bool    `json:"triggered"`
	Value       float64 `json:"value,omitempty"`
	Threshold   float64 `json:"threshold,omitempty"`
	Description string  `json:"description"`
}

// ClassicExitResult holds the result of classic trend reversal detection.
type ClassicExitResult struct {
	ShouldExit       bool                 `json:"should_exit"`
	Signals          []ClassicExitSignal  `json:"signals"`
	TriggeredCount   int                  `json:"triggered_count"`
	RequiredCount    int                  `json:"required_count"`
	Indicators       *ClassicTrendIndicators `json:"indicators"`
	Reason           string               `json:"reason"`
	AnalyzedAt       time.Time            `json:"analyzed_at"`
}

// detectTrendReversalClassic detects trend reversal signals for Classic Mode.
// For LONG positions: Check bearish signals (ADX > 20 + DI flip, EMA9 < EMA21, RSI > 70 turning down, lower lows)
// For SHORT positions: Check bullish signals (mirror logic)
// Returns true if position should exit due to trend reversal.
func detectTrendReversalClassic(pos *GiniePosition, indicators *ClassicTrendIndicators, settings *ClassicDecisionSettings) *ClassicExitResult {
	if pos == nil || indicators == nil {
		return &ClassicExitResult{
			ShouldExit: false,
			Reason:     "Invalid position or indicators",
			AnalyzedAt: time.Now(),
		}
	}

	if settings == nil {
		settings = DefaultClassicDecisionSettings()
	}

	result := &ClassicExitResult{
		ShouldExit:    false,
		Signals:       make([]ClassicExitSignal, 0),
		RequiredCount: settings.ConfirmationCount,
		Indicators:    indicators,
		AnalyzedAt:    time.Now(),
	}

	// Check minimum hold time
	holdDuration := time.Since(pos.EntryTime)
	if holdDuration.Minutes() < float64(settings.MinHoldMinutes) {
		result.Reason = "Position too young for trend reversal exit"
		return result
	}

	if pos.Side == "LONG" {
		// For LONG positions, check BEARISH reversal signals
		result.Signals = detectBearishSignals(indicators, settings)
	} else if pos.Side == "SHORT" {
		// For SHORT positions, check BULLISH reversal signals
		result.Signals = detectBullishSignals(indicators, settings)
	} else {
		result.Reason = "Unknown position side"
		return result
	}

	// Count triggered signals
	triggeredSignals := []string{}
	for _, signal := range result.Signals {
		if signal.Triggered {
			result.TriggeredCount++
			triggeredSignals = append(triggeredSignals, signal.Name)
		}
	}

	// Determine if we should exit
	if settings.RequireAllSignals {
		result.ShouldExit = result.TriggeredCount == len(result.Signals)
		if result.ShouldExit {
			result.Reason = "All reversal signals triggered"
		} else {
			result.Reason = "Not all signals triggered (require all)"
		}
	} else {
		result.ShouldExit = result.TriggeredCount >= settings.ConfirmationCount
		if result.ShouldExit {
			result.Reason = joinSignalNames(triggeredSignals)
		} else {
			result.Reason = formatNotEnoughSignals(result.TriggeredCount, settings.ConfirmationCount)
		}
	}

	if result.ShouldExit {
		log.Printf("[CLASSIC-EXIT] %s %s: TREND REVERSAL DETECTED! Triggered %d/%d signals: %v",
			pos.Symbol, pos.Side, result.TriggeredCount, len(result.Signals), triggeredSignals)
	}

	return result
}

// detectBearishSignals checks for bearish reversal signals (for LONG position exit).
func detectBearishSignals(indicators *ClassicTrendIndicators, settings *ClassicDecisionSettings) []ClassicExitSignal {
	signals := make([]ClassicExitSignal, 0, 4)

	// Signal 1: ADX Strong + DI Flip (ADX > threshold and -DI > +DI)
	adxDISignal := ClassicExitSignal{
		Name:        "ADX_DI_BEARISH",
		Value:       indicators.ADX,
		Threshold:   settings.ADXThreshold,
		Description: "ADX strong trend with -DI > +DI",
	}
	adxDISignal.Triggered = indicators.ADX >= settings.ADXThreshold && indicators.MinusDI > indicators.PlusDI
	signals = append(signals, adxDISignal)

	// Signal 2: EMA Bearish Cross (EMA9 < EMA21)
	emaCrossSignal := ClassicExitSignal{
		Name:        "EMA_BEARISH_CROSS",
		Value:       indicators.EMA9 - indicators.EMA21,
		Threshold:   0,
		Description: "EMA9 crossed below EMA21",
	}
	emaCrossSignal.Triggered = indicators.EMA9 < indicators.EMA21
	signals = append(signals, emaCrossSignal)

	// Signal 3: RSI Overbought and Turning Down
	rsiSignal := ClassicExitSignal{
		Name:        "RSI_OVERBOUGHT_TURNING",
		Value:       indicators.RSI,
		Threshold:   settings.RSIUpperBound,
		Description: "RSI overbought and turning down",
	}
	rsiTurnDown := indicators.PreviousRSI > 0 && (indicators.PreviousRSI - indicators.RSI) >= settings.RSITurningDown
	rsiSignal.Triggered = indicators.RSI >= settings.RSIUpperBound || (indicators.RSI >= settings.RSIUpperBound-10 && rsiTurnDown)
	signals = append(signals, rsiSignal)

	// Signal 4: Lower Lows Pattern
	lowerLowsSignal := ClassicExitSignal{
		Name:        "LOWER_LOWS",
		Description: "Price making lower lows (bearish pattern)",
	}
	lowerLowsSignal.Triggered = indicators.LowerLows
	signals = append(signals, lowerLowsSignal)

	return signals
}

// detectBullishSignals checks for bullish reversal signals (for SHORT position exit).
func detectBullishSignals(indicators *ClassicTrendIndicators, settings *ClassicDecisionSettings) []ClassicExitSignal {
	signals := make([]ClassicExitSignal, 0, 4)

	// Signal 1: ADX Strong + DI Flip (ADX > threshold and +DI > -DI)
	adxDISignal := ClassicExitSignal{
		Name:        "ADX_DI_BULLISH",
		Value:       indicators.ADX,
		Threshold:   settings.ADXThreshold,
		Description: "ADX strong trend with +DI > -DI",
	}
	adxDISignal.Triggered = indicators.ADX >= settings.ADXThreshold && indicators.PlusDI > indicators.MinusDI
	signals = append(signals, adxDISignal)

	// Signal 2: EMA Bullish Cross (EMA9 > EMA21)
	emaCrossSignal := ClassicExitSignal{
		Name:        "EMA_BULLISH_CROSS",
		Value:       indicators.EMA9 - indicators.EMA21,
		Threshold:   0,
		Description: "EMA9 crossed above EMA21",
	}
	emaCrossSignal.Triggered = indicators.EMA9 > indicators.EMA21
	signals = append(signals, emaCrossSignal)

	// Signal 3: RSI Oversold and Turning Up
	rsiSignal := ClassicExitSignal{
		Name:        "RSI_OVERSOLD_TURNING",
		Value:       indicators.RSI,
		Threshold:   settings.RSILowerBound,
		Description: "RSI oversold and turning up",
	}
	rsiTurnUp := indicators.PreviousRSI > 0 && (indicators.RSI - indicators.PreviousRSI) >= settings.RSITurningUp
	rsiSignal.Triggered = indicators.RSI <= settings.RSILowerBound || (indicators.RSI <= settings.RSILowerBound+10 && rsiTurnUp)
	signals = append(signals, rsiSignal)

	// Signal 4: Higher Highs Pattern
	higherHighsSignal := ClassicExitSignal{
		Name:        "HIGHER_HIGHS",
		Description: "Price making higher highs (bullish pattern)",
	}
	higherHighsSignal.Triggered = indicators.HigherHighs
	signals = append(signals, higherHighsSignal)

	return signals
}

// calculateClassicTrendIndicators calculates indicators needed for classic trend detection.
// This function requires klines data and the GinieAnalyzer for technical calculations.
func calculateClassicTrendIndicators(g *GinieAnalyzer, symbol string, timeframe string) (*ClassicTrendIndicators, error) {
	if g == nil || g.futuresClient == nil {
		return nil, fmt.Errorf("analyzer or futures client is nil")
	}

	// Fetch klines - need at least 50 candles for reliable indicator calculation
	klines, err := g.futuresClient.GetFuturesKlines(symbol, timeframe, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch klines: %w", err)
	}

	if len(klines) < 30 {
		return nil, fmt.Errorf("insufficient klines data: got %d, need at least 30", len(klines))
	}

	indicators := &ClassicTrendIndicators{
		CalculatedAt: time.Now(),
	}

	// Extract price data for calculations
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
	}

	// Calculate ADX and DI
	indicators.ADX, indicators.PlusDI, indicators.MinusDI = calculateADX(highs, lows, closes, 14)

	// Calculate EMAs
	indicators.EMA9 = calculateEMA(closes, 9)
	indicators.EMA21 = calculateEMA(closes, 21)
	indicators.EMABearishCross = indicators.EMA9 < indicators.EMA21
	indicators.EMABullishCross = indicators.EMA9 > indicators.EMA21

	// Calculate RSI
	indicators.RSI = calculateRSI(closes, 14)

	// Calculate previous RSI for turning detection
	if len(closes) > 15 {
		prevCloses := closes[:len(closes)-1]
		indicators.PreviousRSI = calculateRSI(prevCloses, 14)
	}

	// Check Lower Lows pattern (last 3 candles making consecutive lower lows)
	indicators.LowerLows = checkLowerLows(lows, 3)

	// Check Higher Highs pattern (last 3 candles making consecutive higher highs)
	indicators.HigherHighs = checkHigherHighs(highs, 3)

	return indicators, nil
}

// calculateADX calculates the Average Directional Index and Directional Indicators.
func calculateADX(highs, lows, closes []float64, period int) (adx, plusDI, minusDI float64) {
	if len(closes) < period+1 {
		return 0, 0, 0
	}

	// Calculate True Range, +DM, -DM
	trs := make([]float64, len(closes)-1)
	plusDMs := make([]float64, len(closes)-1)
	minusDMs := make([]float64, len(closes)-1)

	for i := 1; i < len(closes); i++ {
		// True Range
		tr1 := highs[i] - lows[i]
		tr2 := math.Abs(highs[i] - closes[i-1])
		tr3 := math.Abs(lows[i] - closes[i-1])
		trs[i-1] = math.Max(tr1, math.Max(tr2, tr3))

		// +DM and -DM
		upMove := highs[i] - highs[i-1]
		downMove := lows[i-1] - lows[i]

		if upMove > downMove && upMove > 0 {
			plusDMs[i-1] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDMs[i-1] = downMove
		}
	}

	// Calculate smoothed values
	atr := smoothedAverage(trs, period)
	smoothedPlusDM := smoothedAverage(plusDMs, period)
	smoothedMinusDM := smoothedAverage(minusDMs, period)

	if atr == 0 {
		return 0, 0, 0
	}

	plusDI = (smoothedPlusDM / atr) * 100
	minusDI = (smoothedMinusDM / atr) * 100

	// Calculate DX and ADX
	diSum := plusDI + minusDI
	if diSum == 0 {
		return 0, plusDI, minusDI
	}
	dx := math.Abs(plusDI-minusDI) / diSum * 100

	// ADX is smoothed DX (simplified - using last DX value)
	adx = dx

	return adx, plusDI, minusDI
}

// calculateEMA calculates the Exponential Moving Average.
func calculateEMA(prices []float64, period int) float64 {
	if len(prices) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)
	ema := prices[0]

	for i := 1; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}

// calculateRSI calculates the Relative Strength Index.
func calculateRSI(prices []float64, period int) float64 {
	if len(prices) < period+1 {
		return 50 // Default neutral
	}

	gains := make([]float64, 0)
	losses := make([]float64, 0)

	for i := 1; i < len(prices); i++ {
		change := prices[i] - prices[i-1]
		if change > 0 {
			gains = append(gains, change)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, math.Abs(change))
		}
	}

	// Use the last 'period' values
	startIdx := len(gains) - period
	if startIdx < 0 {
		startIdx = 0
	}

	avgGain := 0.0
	avgLoss := 0.0
	for i := startIdx; i < len(gains); i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

// checkLowerLows checks if the last N candles are making consecutive lower lows.
func checkLowerLows(lows []float64, count int) bool {
	if len(lows) < count+1 {
		return false
	}

	// Check last 'count' candles (excluding current)
	startIdx := len(lows) - count - 1
	for i := startIdx + 1; i < len(lows)-1; i++ {
		if lows[i] >= lows[i-1] {
			return false
		}
	}
	return true
}

// checkHigherHighs checks if the last N candles are making consecutive higher highs.
func checkHigherHighs(highs []float64, count int) bool {
	if len(highs) < count+1 {
		return false
	}

	// Check last 'count' candles (excluding current)
	startIdx := len(highs) - count - 1
	for i := startIdx + 1; i < len(highs)-1; i++ {
		if highs[i] <= highs[i-1] {
			return false
		}
	}
	return true
}

// smoothedAverage calculates a smoothed average (Wilder's smoothing).
func smoothedAverage(values []float64, period int) float64 {
	if len(values) < period {
		return 0
	}

	// First smoothed value is SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	smoothed := sum / float64(period)

	// Subsequent values use Wilder's smoothing
	for i := period; i < len(values); i++ {
		smoothed = (smoothed*float64(period-1) + values[i]) / float64(period)
	}

	return smoothed
}

// Helper functions for formatting results

func joinSignalNames(names []string) string {
	if len(names) == 0 {
		return "No signals triggered"
	}
	result := "Trend reversal: "
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}

func formatNotEnoughSignals(triggered, required int) string {
	return fmt.Sprintf("Only %d/%d reversal signals (need %d)", triggered, required, required)
}
