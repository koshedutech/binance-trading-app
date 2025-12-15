package strategy

import (
	"binance-trading-bot/internal/binance"
	"fmt"
	"time"
)

// SwingTradingConfig configures the advanced swing trading strategy
type SwingTradingConfig struct {
	Symbol            string
	Timeframe         string
	PositionSize      float64
	StopLossPercent   float64
	TakeProfitPercent float64
	Autopilot         bool

	// Indicator periods
	FastEMAPeriod   int
	SlowEMAPeriod   int
	RSIPeriod       int
	MACDFast        int
	MACDSlow        int
	MACDSignal      int
	VolumePeriod    int
	VolumeMultiplier float64

	// Thresholds
	RSIOversold      float64
	RSIOverbought    float64
	MinADX           float64
}

// SwingTradingStrategy implements advanced swing trading with multiple confirmations
type SwingTradingStrategy struct {
	config *SwingTradingConfig
}

// SignalWithConditions represents a signal with detailed conditions met
type SignalWithConditions struct {
	Signal     *Signal
	Conditions map[string]interface{}
}

func NewSwingTradingStrategy(config *SwingTradingConfig) *SwingTradingStrategy {
	// Set defaults if not provided
	if config.FastEMAPeriod == 0 {
		config.FastEMAPeriod = 20
	}
	if config.SlowEMAPeriod == 0 {
		config.SlowEMAPeriod = 50
	}
	if config.RSIPeriod == 0 {
		config.RSIPeriod = 14
	}
	if config.MACDFast == 0 {
		config.MACDFast = 12
	}
	if config.MACDSlow == 0 {
		config.MACDSlow = 26
	}
	if config.MACDSignal == 0 {
		config.MACDSignal = 9
	}
	if config.VolumePeriod == 0 {
		config.VolumePeriod = 20
	}
	if config.VolumeMultiplier == 0 {
		config.VolumeMultiplier = 1.5
	}
	if config.RSIOversold == 0 {
		config.RSIOversold = 30
	}
	if config.RSIOverbought == 0 {
		config.RSIOverbought = 70
	}
	if config.MinADX == 0 {
		config.MinADX = 20
	}

	return &SwingTradingStrategy{config: config}
}

func (s *SwingTradingStrategy) Name() string {
	return fmt.Sprintf("SwingTrading-%s-%s", s.config.Symbol, s.config.Timeframe)
}

func (s *SwingTradingStrategy) GetSymbol() string {
	return s.config.Symbol
}

func (s *SwingTradingStrategy) GetInterval() string {
	return s.config.Timeframe
}

// Evaluate checks multiple conditions for entry signals
func (s *SwingTradingStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	result := s.EvaluateWithConditions(klines, currentPrice)
	if result == nil {
		return &Signal{Type: SignalNone}, nil
	}
	return result.Signal, nil
}

// EvaluateWithConditions provides detailed breakdown of conditions
func (s *SwingTradingStrategy) EvaluateWithConditions(klines []binance.Kline, currentPrice float64) *SignalWithConditions {
	if len(klines) < s.config.SlowEMAPeriod+10 {
		return nil
	}

	conditions := make(map[string]interface{})
	conditions["timestamp"] = time.Now()

	// ========================================================================
	// 1. TREND ANALYSIS
	// ========================================================================
	fastEMA := CalculateEMA(klines, s.config.FastEMAPeriod)
	slowEMA := CalculateEMA(klines, s.config.SlowEMAPeriod)
	trend := DetectTrend(klines, s.config.FastEMAPeriod, s.config.SlowEMAPeriod)

	conditions["fast_ema"] = map[string]interface{}{
		"value": fastEMA,
		"period": s.config.FastEMAPeriod,
	}
	conditions["slow_ema"] = map[string]interface{}{
		"value": slowEMA,
		"period": s.config.SlowEMAPeriod,
	}
	conditions["trend"] = map[string]interface{}{
		"direction": string(trend),
		"met": trend == TrendUp,
	}

	// ========================================================================
	// 2. MOMENTUM INDICATORS
	// ========================================================================
	rsi := CalculateRSI(klines, s.config.RSIPeriod)
	macd := CalculateMACD(klines, s.config.MACDFast, s.config.MACDSlow, s.config.MACDSignal)

	conditions["rsi"] = map[string]interface{}{
		"value": rsi,
		"oversold": rsi < s.config.RSIOversold,
		"overbought": rsi > s.config.RSIOverbought,
		"neutral": rsi >= 50,
	}

	conditions["macd"] = map[string]interface{}{
		"value": macd.MACD,
		"signal": macd.Signal,
		"histogram": macd.Histogram,
		"bullish_crossover": macd.Histogram > 0,
	}

	// ========================================================================
	// 3. VOLUME ANALYSIS
	// ========================================================================
	avgVolume := CalculateAverageVolume(klines, s.config.VolumePeriod)
	currentVolume := klines[len(klines)-1].Volume
	volumeSpike := currentVolume >= avgVolume*s.config.VolumeMultiplier

	conditions["volume"] = map[string]interface{}{
		"current": currentVolume,
		"average": avgVolume,
		"spike": volumeSpike,
		"ratio": currentVolume / avgVolume,
	}

	// ========================================================================
	// 4. VOLATILITY
	// ========================================================================
	atr := CalculateATR(klines, 14)
	adx := CalculateADX(klines, 14)

	conditions["volatility"] = map[string]interface{}{
		"atr": atr,
		"adx": adx,
		"trending": adx >= s.config.MinADX,
	}

	// ========================================================================
	// 5. CANDLESTICK PATTERNS
	// ========================================================================
	patterns := DetectAllPatterns(klines)
	bullishPatterns := []string{}
	bearishPatterns := []string{}

	for _, pattern := range patterns {
		if pattern.Type == "BULLISH" {
			bullishPatterns = append(bullishPatterns, pattern.Name)
		} else if pattern.Type == "BEARISH" {
			bearishPatterns = append(bearishPatterns, pattern.Name)
		}
	}

	conditions["patterns"] = map[string]interface{}{
		"bullish": bullishPatterns,
		"bearish": bearishPatterns,
		"count": len(patterns),
	}

	// ========================================================================
	// 6. SUPPORT/RESISTANCE
	// ========================================================================
	support, resistance := FindSupportResistance(klines, 20)
	fibonacci := CalculateFibonacciLevels(klines, 20)

	conditions["levels"] = map[string]interface{}{
		"support": support,
		"resistance": resistance,
		"fibonacci_382": fibonacci.Level382,
		"fibonacci_618": fibonacci.Level618,
	}

	// ========================================================================
	// 7. PIVOT POINTS
	// ========================================================================
	pivots := CalculateStandardPivotPoints(klines)
	pivotBreakout, pivotLevel := CheckPivotBreakout(currentPrice, pivots, 0.001)
	abovePivot := IsPriceAbovePivot(currentPrice, pivots)

	conditions["pivot_points"] = map[string]interface{}{
		"pp": pivots.PP,
		"r1": pivots.R1,
		"r2": pivots.R2,
		"r3": pivots.R3,
		"s1": pivots.S1,
		"s2": pivots.S2,
		"s3": pivots.S3,
		"above_pivot": abovePivot,
		"near_level": pivotBreakout,
		"level_name": pivotLevel,
	}

	// ========================================================================
	// ENTRY LOGIC: BULLISH SETUP
	// ========================================================================

	// Condition checks for LONG entry
	conditionsMet := []string{}
	conditionsFailed := []string{}

	// 1. Trend Filter: Price above 50 EMA
	if currentPrice > slowEMA {
		conditionsMet = append(conditionsMet, "Price above 50 EMA (uptrend)")
	} else {
		conditionsFailed = append(conditionsFailed, "Price not above 50 EMA")
	}

	// 2. Pullback: Price near 20 EMA
	pullbackToEMA := currentPrice >= fastEMA*0.99 && currentPrice <= fastEMA*1.01
	if pullbackToEMA {
		conditionsMet = append(conditionsMet, "Price near 20 EMA (pullback entry)")
	}

	// 3. RSI: Above 50 or oversold recovery
	rsiCondition := rsi > 50 || (rsi < s.config.RSIOversold && rsi > rsi-5)
	if rsiCondition {
		conditionsMet = append(conditionsMet, fmt.Sprintf("RSI %.2f confirms momentum", rsi))
	} else {
		conditionsFailed = append(conditionsFailed, fmt.Sprintf("RSI %.2f too weak", rsi))
	}

	// 4. MACD: Bullish crossover
	if macd.Histogram > 0 {
		conditionsMet = append(conditionsMet, "MACD bullish crossover")
	} else {
		conditionsFailed = append(conditionsFailed, "MACD not bullish")
	}

	// 5. Volume: Above average
	if volumeSpike {
		conditionsMet = append(conditionsMet, fmt.Sprintf("Volume spike %.2fx average", currentVolume/avgVolume))
	} else if currentVolume > avgVolume {
		conditionsMet = append(conditionsMet, "Volume above average")
	} else {
		conditionsFailed = append(conditionsFailed, "Volume below average")
	}

	// 6. Bullish candlestick pattern
	if len(bullishPatterns) > 0 {
		conditionsMet = append(conditionsMet, fmt.Sprintf("Bullish pattern: %s", bullishPatterns[0]))
	}

	// 7. Trend strength (ADX)
	if adx >= s.config.MinADX {
		conditionsMet = append(conditionsMet, fmt.Sprintf("Strong trend (ADX %.2f)", adx))
	}

	// 8. Pivot point confirmation
	if pivotBreakout && abovePivot {
		conditionsMet = append(conditionsMet, fmt.Sprintf("Price near %s pivot level", pivotLevel))
	}

	conditions["conditions_met"] = conditionsMet
	conditions["conditions_failed"] = conditionsFailed
	conditions["score"] = len(conditionsMet)

	// ========================================================================
	// SIGNAL GENERATION
	// ========================================================================

	// Require at least 5 out of 7 conditions to be met
	minimumScore := 5
	if len(conditionsMet) >= minimumScore {
		// Calculate entry, stop loss, and take profit
		entryPrice := currentPrice

		// Dynamic stop loss based on ATR
		stopLossDistance := atr * 1.5
		if s.config.StopLossPercent > 0 {
			stopLossDistance = entryPrice * (s.config.StopLossPercent / 100)
		}

		// Risk/Reward ratio 1:2.5
		takeProfitDistance := stopLossDistance * 2.5
		if s.config.TakeProfitPercent > 0 {
			takeProfitDistance = entryPrice * (s.config.TakeProfitPercent / 100)
		}

		stopLoss := entryPrice - stopLossDistance
		takeProfit := entryPrice + takeProfitDistance

		reasonParts := conditionsMet
		reason := "Swing Trading Entry: " + fmt.Sprintf("%d conditions met - ", len(conditionsMet))
		for i, cond := range reasonParts {
			if i < 3 { // Show top 3 reasons
				reason += cond + "; "
			}
		}

		signal := &Signal{
			Type:       SignalBuy,
			Symbol:     s.config.Symbol,
			EntryPrice: entryPrice,
			StopLoss:   stopLoss,
			TakeProfit: takeProfit,
			OrderType:  "LIMIT",
			Side:       "BUY",
			Reason:     reason,
			Timestamp:  time.Now(),
		}

		return &SignalWithConditions{
			Signal:     signal,
			Conditions: conditions,
		}
	}

	// No signal if not enough conditions met
	return nil
}

// GetAutopilotMode returns whether this strategy is in autopilot mode
func (s *SwingTradingStrategy) GetAutopilotMode() bool {
	return s.config.Autopilot
}
