package analysis

import (
	"binance-trading-bot/internal/binance"
)

// TrendDirection represents market trend
type TrendDirection string

const (
	TrendBullish  TrendDirection = "bullish"
	TrendBearish  TrendDirection = "bearish"
	TrendSideways TrendDirection = "sideways"
)

// MarketStructure represents analyzed market conditions
type MarketStructure struct {
	Trend           TrendDirection
	TrendStrength   float64 // 0.0 to 1.0
	HigherHighs     int
	HigherLows      int
	LowerHighs      int
	LowerLows       int
	SwingHighs      []SwingPoint
	SwingLows       []SwingPoint
	SupportLevels   []float64
	ResistanceLevels []float64
	CurrentPhase    string // "markup", "markdown", "accumulation", "distribution"
}

// SwingPoint represents a significant price level
type SwingPoint struct {
	Price       float64
	CandleIndex int
	Type        string // "high" or "low"
	Confirmed   bool
}

// TrendAnalyzer analyzes market trend and structure
type TrendAnalyzer struct {
	swingLookback int // Candles to look back for swing points
}

// NewTrendAnalyzer creates a new trend analyzer
func NewTrendAnalyzer(swingLookback int) *TrendAnalyzer {
	if swingLookback <= 0 {
		swingLookback = 5 // Default 5-candle swing
	}
	return &TrendAnalyzer{
		swingLookback: swingLookback,
	}
}

// AnalyzeStructure performs comprehensive market structure analysis
func (ta *TrendAnalyzer) AnalyzeStructure(candles []binance.Kline) *MarketStructure {
	if len(candles) < ta.swingLookback*2 {
		return nil
	}

	structure := &MarketStructure{
		SwingHighs: make([]SwingPoint, 0),
		SwingLows:  make([]SwingPoint, 0),
	}

	// 1. Identify swing highs and lows
	structure.SwingHighs = ta.FindSwingHighs(candles)
	structure.SwingLows = ta.FindSwingLows(candles)

	// 2. Count higher highs, higher lows, etc.
	structure.HigherHighs = ta.CountHigherHighs(structure.SwingHighs)
	structure.HigherLows = ta.CountHigherLows(structure.SwingLows)
	structure.LowerHighs = ta.CountLowerHighs(structure.SwingHighs)
	structure.LowerLows = ta.CountLowerLows(structure.SwingLows)

	// 3. Determine trend direction
	structure.Trend = ta.DetermineTrend(structure)

	// 4. Calculate trend strength
	structure.TrendStrength = ta.CalculateTrendStrength(structure)

	// 5. Identify support and resistance levels
	structure.SupportLevels = ta.IdentifySupportLevels(structure.SwingLows)
	structure.ResistanceLevels = ta.IdentifyResistanceLevels(structure.SwingHighs)

	// 6. Determine market phase
	structure.CurrentPhase = ta.DetermineMarketPhase(candles, structure)

	return structure
}

// FindSwingHighs identifies swing high points
func (ta *TrendAnalyzer) FindSwingHighs(candles []binance.Kline) []SwingPoint {
	var swingHighs []SwingPoint

	for i := ta.swingLookback; i < len(candles)-ta.swingLookback; i++ {
		isSwingHigh := true
		currentHigh := candles[i].High

		// Check if this is the highest point in the lookback window
		for j := i - ta.swingLookback; j <= i+ta.swingLookback; j++ {
			if j != i && candles[j].High >= currentHigh {
				isSwingHigh = false
				break
			}
		}

		if isSwingHigh {
			swingHighs = append(swingHighs, SwingPoint{
				Price:       currentHigh,
				CandleIndex: i,
				Type:        "high",
				Confirmed:   i < len(candles)-ta.swingLookback,
			})
		}
	}

	return swingHighs
}

// FindSwingLows identifies swing low points
func (ta *TrendAnalyzer) FindSwingLows(candles []binance.Kline) []SwingPoint {
	var swingLows []SwingPoint

	for i := ta.swingLookback; i < len(candles)-ta.swingLookback; i++ {
		isSwingLow := true
		currentLow := candles[i].Low

		// Check if this is the lowest point in the lookback window
		for j := i - ta.swingLookback; j <= i+ta.swingLookback; j++ {
			if j != i && candles[j].Low <= currentLow {
				isSwingLow = false
				break
			}
		}

		if isSwingLow {
			swingLows = append(swingLows, SwingPoint{
				Price:       currentLow,
				CandleIndex: i,
				Type:        "low",
				Confirmed:   i < len(candles)-ta.swingLookback,
			})
		}
	}

	return swingLows
}

// CountHigherHighs counts higher highs in swing points
func (ta *TrendAnalyzer) CountHigherHighs(swingHighs []SwingPoint) int {
	if len(swingHighs) < 2 {
		return 0
	}

	count := 0
	for i := 1; i < len(swingHighs); i++ {
		if swingHighs[i].Price > swingHighs[i-1].Price {
			count++
		}
	}
	return count
}

// CountHigherLows counts higher lows in swing points
func (ta *TrendAnalyzer) CountHigherLows(swingLows []SwingPoint) int {
	if len(swingLows) < 2 {
		return 0
	}

	count := 0
	for i := 1; i < len(swingLows); i++ {
		if swingLows[i].Price > swingLows[i-1].Price {
			count++
		}
	}
	return count
}

// CountLowerHighs counts lower highs in swing points
func (ta *TrendAnalyzer) CountLowerHighs(swingHighs []SwingPoint) int {
	if len(swingHighs) < 2 {
		return 0
	}

	count := 0
	for i := 1; i < len(swingHighs); i++ {
		if swingHighs[i].Price < swingHighs[i-1].Price {
			count++
		}
	}
	return count
}

// CountLowerLows counts lower lows in swing points
func (ta *TrendAnalyzer) CountLowerLows(swingLows []SwingPoint) int {
	if len(swingLows) < 2 {
		return 0
	}

	count := 0
	for i := 1; i < len(swingLows); i++ {
		if swingLows[i].Price < swingLows[i-1].Price {
			count++
		}
	}
	return count
}

// DetermineTrend determines overall trend direction
func (ta *TrendAnalyzer) DetermineTrend(structure *MarketStructure) TrendDirection {
	// Bullish: Higher highs AND higher lows
	if structure.HigherHighs > 0 && structure.HigherLows > 0 {
		if structure.HigherHighs >= structure.LowerHighs &&
			structure.HigherLows >= structure.LowerLows {
			return TrendBullish
		}
	}

	// Bearish: Lower highs AND lower lows
	if structure.LowerHighs > 0 && structure.LowerLows > 0 {
		if structure.LowerHighs >= structure.HigherHighs &&
			structure.LowerLows >= structure.HigherLows {
			return TrendBearish
		}
	}

	// Sideways: Mixed signals
	return TrendSideways
}

// CalculateTrendStrength calculates strength of trend (0-1)
func (ta *TrendAnalyzer) CalculateTrendStrength(structure *MarketStructure) float64 {
	totalSwings := structure.HigherHighs + structure.HigherLows +
		structure.LowerHighs + structure.LowerLows

	if totalSwings == 0 {
		return 0.0
	}

	if structure.Trend == TrendBullish {
		// Strength = % of swings that are bullish
		bullishSwings := structure.HigherHighs + structure.HigherLows
		return float64(bullishSwings) / float64(totalSwings)
	} else if structure.Trend == TrendBearish {
		// Strength = % of swings that are bearish
		bearishSwings := structure.LowerHighs + structure.LowerLows
		return float64(bearishSwings) / float64(totalSwings)
	}

	// Sideways trend has low strength
	return 0.3
}

// IdentifySupportLevels identifies key support price levels
func (ta *TrendAnalyzer) IdentifySupportLevels(swingLows []SwingPoint) []float64 {
	if len(swingLows) < 2 {
		return nil
	}

	// Cluster swing lows within 1% range
	var supports []float64
	tolerance := 0.01 // 1% tolerance

	for _, swing := range swingLows {
		found := false
		for i, support := range supports {
			if abs(swing.Price-support)/support < tolerance {
				// Update to average
				supports[i] = (support + swing.Price) / 2
				found = true
				break
			}
		}
		if !found {
			supports = append(supports, swing.Price)
		}
	}

	return supports
}

// IdentifyResistanceLevels identifies key resistance price levels
func (ta *TrendAnalyzer) IdentifyResistanceLevels(swingHighs []SwingPoint) []float64 {
	if len(swingHighs) < 2 {
		return nil
	}

	// Cluster swing highs within 1% range
	var resistances []float64
	tolerance := 0.01

	for _, swing := range swingHighs {
		found := false
		for i, resistance := range resistances {
			if abs(swing.Price-resistance)/resistance < tolerance {
				resistances[i] = (resistance + swing.Price) / 2
				found = true
				break
			}
		}
		if !found {
			resistances = append(resistances, swing.Price)
		}
	}

	return resistances
}

// DetermineMarketPhase identifies current market phase
func (ta *TrendAnalyzer) DetermineMarketPhase(candles []binance.Kline, structure *MarketStructure) string {
	if structure.Trend == TrendBullish && structure.TrendStrength > 0.7 {
		return "markup" // Strong uptrend
	} else if structure.Trend == TrendBearish && structure.TrendStrength > 0.7 {
		return "markdown" // Strong downtrend
	} else if structure.Trend == TrendSideways {
		// Determine if accumulation or distribution based on recent volume
		// For now, simple heuristic
		recentCandles := candles[len(candles)-20:]
		avgPrice := 0.0
		for _, c := range recentCandles {
			avgPrice += c.Close
		}
		avgPrice /= float64(len(recentCandles))

		currentPrice := candles[len(candles)-1].Close
		if currentPrice > avgPrice {
			return "accumulation" // Building support
		} else {
			return "distribution" // Building resistance
		}
	}

	return "transitional"
}

// IsPriceAtSupport checks if current price is near a support level
func (ta *TrendAnalyzer) IsPriceAtSupport(currentPrice float64, supportLevels []float64, tolerance float64) bool {
	for _, support := range supportLevels {
		if abs(currentPrice-support)/support < tolerance {
			return true
		}
	}
	return false
}

// IsPriceAtResistance checks if current price is near a resistance level
func (ta *TrendAnalyzer) IsPriceAtResistance(currentPrice float64, resistanceLevels []float64, tolerance float64) bool {
	for _, resistance := range resistanceLevels {
		if abs(currentPrice-resistance)/resistance < tolerance {
			return true
		}
	}
	return false
}
