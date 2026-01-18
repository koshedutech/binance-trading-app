// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.16: Enhanced LLM Context
package decision

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// LLMContext represents comprehensive context for LLM-based position decisions.
// This struct provides all relevant market data in a structured format that
// can be easily formatted for LLM prompt injection.
type LLMContext struct {
	// Symbol identification
	Symbol    string `json:"symbol"`
	Timestamp int64  `json:"timestamp"`

	// Market Regime
	MarketRegime     MarketRegime `json:"market_regime"`
	RegimeConfidence float64      `json:"regime_confidence"` // 0.0-1.0

	// Trend Analysis
	ADXValue          float64        `json:"adx_value"`
	ADXInterpretation string         `json:"adx_interpretation"`
	Trend1H           TrendDirection `json:"trend_1h"`
	Trend15M          TrendDirection `json:"trend_15m"`
	TrendAlignment    bool           `json:"trend_alignment"`

	// Price Position
	CurrentPrice float64 `json:"current_price"`
	EMA9         float64 `json:"ema_9"`
	EMA21        float64 `json:"ema_21"`
	EMA50        float64 `json:"ema_50"`
	PriceVsEMAs  string  `json:"price_vs_emas"`

	// VWAP
	VWAPValue   float64 `json:"vwap_value"`
	PriceVsVWAP string  `json:"price_vs_vwap"`

	// Market Structure
	MarketStructure string `json:"market_structure"` // HH/HL, LH/LL, Mixed

	// BTC Context
	BTCTrend       string  `json:"btc_trend"`
	BTCCorrelation float64 `json:"btc_correlation"`

	// Support/Resistance
	NearestSupport       float64 `json:"nearest_support"`
	NearestResistance    float64 `json:"nearest_resistance"`
	DistanceToSupport    float64 `json:"distance_to_support"`    // percentage
	DistanceToResistance float64 `json:"distance_to_resistance"` // percentage

	// Volume
	CurrentVolume        float64 `json:"current_volume"`
	AverageVolume        float64 `json:"average_volume"`
	VolumeRatio          float64 `json:"volume_ratio"`
	VolumeInterpretation string  `json:"volume_interpretation"`

	// Momentum
	RSI               float64 `json:"rsi"`
	RSIInterpretation string  `json:"rsi_interpretation"`

	// ATR (Volatility)
	ATR               float64 `json:"atr"`
	ATRAverage        float64 `json:"atr_average"`
	ATRInterpretation string  `json:"atr_interpretation"`

	// Scoring Summary (from ScoreBreakdown if available)
	TechnicalScore int `json:"technical_score"` // 0-100
	ContextScore   int `json:"context_score"`   // 0-100
	FinalScore     int `json:"final_score"`     // 0-100

	// Blocking Summary
	IsBlocked       bool     `json:"is_blocked"`
	BlockingReasons []string `json:"blocking_reasons"`

	// Active Strategy
	ActiveStrategy string `json:"active_strategy"`
}

// NewLLMContext creates a new LLMContext for a symbol
func NewLLMContext(symbol string) *LLMContext {
	return &LLMContext{
		Symbol:          symbol,
		Timestamp:       time.Now().UnixMilli(),
		BlockingReasons: []string{},
	}
}

// BuildLLMContext creates a comprehensive LLMContext from a CoinState.
// This is the main entry point for building context for LLM prompts.
func BuildLLMContext(state *CoinState) *LLMContext {
	if state == nil {
		return nil
	}

	ctx := NewLLMContext(state.Symbol)

	// Copy basic data from CoinState
	ctx.CurrentPrice = state.Price
	ctx.ADXValue = state.ADX
	ctx.RSI = state.RSI
	ctx.EMA9 = state.EMA9
	ctx.EMA21 = state.EMA21
	ctx.ATR = state.ATR
	ctx.ATRAverage = state.ATRAverage

	// Market Regime
	ctx.MarketRegime = state.Regime
	ctx.RegimeConfidence = calculateRegimeConfidence(state)
	ctx.ActiveStrategy = state.ActiveStrategy

	// Trend Analysis
	ctx.Trend1H = state.Trend1H
	ctx.Trend15M = state.Trend15M
	ctx.TrendAlignment = (state.Trend1H == state.Trend15M) && state.Trend1H != TrendNeutral && state.Trend1H != ""
	ctx.ADXInterpretation = InterpretADX(state.ADX)

	// Price vs EMAs
	ctx.PriceVsEMAs = InterpretPriceVsEMA(state.Price, ctx.EMA9, ctx.EMA21, ctx.EMA50)

	// RSI Interpretation
	ctx.RSIInterpretation = InterpretRSI(state.RSI)

	// ATR Interpretation
	ctx.ATRInterpretation = InterpretATR(state.ATR, state.ATRAverage)

	// Scores from state
	ctx.TechnicalScore = state.ScoreTechnical
	ctx.ContextScore = state.ScoreContext
	ctx.FinalScore = state.ScoreFinal

	// Blocking - copy slice to avoid shared reference
	ctx.IsBlocked = state.IsBlocked()
	if len(state.BlockingReasons) > 0 {
		ctx.BlockingReasons = make([]string, len(state.BlockingReasons))
		copy(ctx.BlockingReasons, state.BlockingReasons)
	}

	return ctx
}

// BuildLLMContextExtended creates a comprehensive LLMContext with extended market data.
// Use this when you have additional data beyond CoinState (e.g., VWAP, support/resistance).
func BuildLLMContextExtended(state *CoinState, ext *ExtendedMarketData) *LLMContext {
	ctx := BuildLLMContext(state)
	if ctx == nil {
		return nil
	}

	if ext != nil {
		// VWAP
		ctx.VWAPValue = ext.VWAPValue
		ctx.PriceVsVWAP = InterpretPriceVsVWAP(state.Price, ext.VWAPValue)

		// Support/Resistance - validate values are positive and calculate distances
		// Support should be at or below price; resistance at or above price
		if ext.NearestSupport > 0 {
			ctx.NearestSupport = ext.NearestSupport
		}
		if ext.NearestResistance > 0 {
			ctx.NearestResistance = ext.NearestResistance
		}
		if state.Price > 0 {
			if ctx.NearestSupport > 0 {
				// Distance to support (positive = price above support, negative = price below support)
				ctx.DistanceToSupport = ((state.Price - ctx.NearestSupport) / state.Price) * 100
			}
			if ctx.NearestResistance > 0 {
				// Distance to resistance (positive = resistance above price, negative = already breached)
				ctx.DistanceToResistance = ((ctx.NearestResistance - state.Price) / state.Price) * 100
			}
		}

		// Volume
		ctx.CurrentVolume = ext.CurrentVolume
		ctx.AverageVolume = ext.AverageVolume
		if ext.AverageVolume > 0 {
			ctx.VolumeRatio = ext.CurrentVolume / ext.AverageVolume
		}
		ctx.VolumeInterpretation = InterpretVolume(ext.CurrentVolume, ext.AverageVolume)

		// Market Structure
		ctx.MarketStructure = InterpretMarketStructure(ext.RecentHighs, ext.RecentLows)

		// BTC Context
		ctx.BTCTrend = ext.BTCTrend
		ctx.BTCCorrelation = ext.BTCCorrelation

		// EMA50 if available
		if ext.EMA50 > 0 {
			ctx.EMA50 = ext.EMA50
			ctx.PriceVsEMAs = InterpretPriceVsEMA(state.Price, ctx.EMA9, ctx.EMA21, ctx.EMA50)
		}
	}

	return ctx
}

// ExtendedMarketData holds additional market data not in CoinState
type ExtendedMarketData struct {
	VWAPValue         float64   `json:"vwap_value"`
	NearestSupport    float64   `json:"nearest_support"`
	NearestResistance float64   `json:"nearest_resistance"`
	CurrentVolume     float64   `json:"current_volume"`
	AverageVolume     float64   `json:"average_volume"`
	RecentHighs       []float64 `json:"recent_highs"` // For market structure analysis
	RecentLows        []float64 `json:"recent_lows"`
	BTCTrend          string    `json:"btc_trend"`       // "BULLISH", "BEARISH", "NEUTRAL"
	BTCCorrelation    float64   `json:"btc_correlation"` // Correlation coefficient
	EMA50             float64   `json:"ema_50"`
}

// ========== Interpretation Helper Functions ==========

// InterpretADX provides a human-readable interpretation of ADX value.
// ADX measures trend strength regardless of direction.
func InterpretADX(value float64) string {
	if value <= 0 {
		return "No data"
	}
	switch {
	case value >= 40:
		return fmt.Sprintf("Very strong trend (%.1f)", value)
	case value >= 25:
		return fmt.Sprintf("Strong trend (%.1f)", value)
	case value >= 20:
		return fmt.Sprintf("Moderate trend (%.1f)", value)
	case value >= 10:
		return fmt.Sprintf("Weak trend (%.1f)", value)
	default:
		return fmt.Sprintf("No trend/Consolidating (%.1f)", value)
	}
}

// InterpretPriceVsEMA analyzes price position relative to EMAs.
// Provides context for trend direction and strength.
func InterpretPriceVsEMA(price, ema9, ema21, ema50 float64) string {
	if price <= 0 {
		return "No price data"
	}

	// Check which EMAs we have data for
	hasEMA9 := ema9 > 0
	hasEMA21 := ema21 > 0
	hasEMA50 := ema50 > 0

	if !hasEMA9 && !hasEMA21 && !hasEMA50 {
		return "No EMA data available"
	}

	aboveEMA9 := hasEMA9 && price > ema9
	aboveEMA21 := hasEMA21 && price > ema21
	aboveEMA50 := hasEMA50 && price > ema50

	// Build position description based on available EMAs
	var parts []string

	if hasEMA9 && hasEMA21 && hasEMA50 {
		// All EMAs available
		if aboveEMA9 && aboveEMA21 && aboveEMA50 {
			return "Above all EMAs (Strong bullish)"
		}
		if !aboveEMA9 && !aboveEMA21 && !aboveEMA50 {
			return "Below all EMAs (Strong bearish)"
		}
		if aboveEMA9 && aboveEMA21 && !aboveEMA50 {
			return "Above EMA9/21, below EMA50 (Bullish but testing long-term)"
		}
		if aboveEMA9 && !aboveEMA21 && !aboveEMA50 {
			return "Above EMA9 only (Weak bullish, short-term bounce)"
		}
		if !aboveEMA9 && !aboveEMA21 && aboveEMA50 {
			return "Above EMA50 only (Short-term bearish, long-term support)"
		}
		if !aboveEMA9 && aboveEMA21 && aboveEMA50 {
			return "Below EMA9, above EMA21/50 (Pullback in uptrend)"
		}
	}

	// Partial EMA data - describe what we have
	if hasEMA9 {
		if aboveEMA9 {
			parts = append(parts, "Above EMA9")
		} else {
			parts = append(parts, "Below EMA9")
		}
	}
	if hasEMA21 {
		if aboveEMA21 {
			parts = append(parts, "Above EMA21")
		} else {
			parts = append(parts, "Below EMA21")
		}
	}
	if hasEMA50 {
		if aboveEMA50 {
			parts = append(parts, "Above EMA50")
		} else {
			parts = append(parts, "Below EMA50")
		}
	}

	return strings.Join(parts, ", ")
}

// InterpretPriceVsVWAP analyzes price position relative to VWAP.
func InterpretPriceVsVWAP(price, vwap float64) string {
	if price <= 0 || vwap <= 0 {
		return "No VWAP data"
	}

	diff := ((price - vwap) / vwap) * 100
	if diff > 0 {
		return fmt.Sprintf("Above VWAP by %.2f%%", diff)
	} else if diff < 0 {
		return fmt.Sprintf("Below VWAP by %.2f%%", math.Abs(diff))
	}
	return "At VWAP"
}

// InterpretRSI provides a human-readable interpretation of RSI value.
func InterpretRSI(value float64) string {
	if value <= 0 {
		return "No data"
	}
	switch {
	case value >= 80:
		return fmt.Sprintf("Extremely overbought (%.1f)", value)
	case value >= 70:
		return fmt.Sprintf("Overbought (%.1f)", value)
	case value >= 60:
		return fmt.Sprintf("Bullish momentum (%.1f)", value)
	case value >= 40:
		return fmt.Sprintf("Neutral (%.1f)", value)
	case value >= 30:
		return fmt.Sprintf("Bearish momentum (%.1f)", value)
	case value >= 20:
		return fmt.Sprintf("Oversold (%.1f)", value)
	default:
		return fmt.Sprintf("Extremely oversold (%.1f)", value)
	}
}

// InterpretATR provides a human-readable interpretation of ATR (volatility).
func InterpretATR(atr, atrAverage float64) string {
	if atr <= 0 {
		return "No data"
	}
	if atrAverage <= 0 {
		return fmt.Sprintf("ATR: %.4f (no baseline)", atr)
	}

	ratio := atr / atrAverage
	switch {
	case ratio >= 2.0:
		return fmt.Sprintf("Extremely high volatility (%.1fx avg)", ratio)
	case ratio >= 1.5:
		return fmt.Sprintf("High volatility (%.1fx avg)", ratio)
	case ratio >= 1.0:
		return fmt.Sprintf("Normal volatility (%.1fx avg)", ratio)
	case ratio >= 0.5:
		return fmt.Sprintf("Low volatility (%.1fx avg)", ratio)
	default:
		return fmt.Sprintf("Very low volatility (%.1fx avg)", ratio)
	}
}

// InterpretVolume provides a human-readable interpretation of volume.
func InterpretVolume(current, average float64) string {
	if current <= 0 {
		return "No volume data"
	}
	if average <= 0 {
		return fmt.Sprintf("Volume: %.2f (no baseline)", current)
	}

	ratio := current / average
	switch {
	case ratio >= 3.0:
		return fmt.Sprintf("Extremely high volume (%.1fx avg)", ratio)
	case ratio >= 2.0:
		return fmt.Sprintf("Very high volume (%.1fx avg)", ratio)
	case ratio >= 1.5:
		return fmt.Sprintf("Above average volume (%.1fx avg)", ratio)
	case ratio >= 0.8:
		return fmt.Sprintf("Normal volume (%.1fx avg)", ratio)
	case ratio >= 0.5:
		return fmt.Sprintf("Below average volume (%.1fx avg)", ratio)
	default:
		return fmt.Sprintf("Very low volume (%.1fx avg)", ratio)
	}
}

// InterpretMarketStructure analyzes recent highs and lows to determine market structure.
// HH/HL = Higher highs, higher lows (uptrend)
// LH/LL = Lower highs, lower lows (downtrend)
func InterpretMarketStructure(highs, lows []float64) string {
	if len(highs) < 2 || len(lows) < 2 {
		return "Insufficient data"
	}

	// Analyze the last 3-5 swing points
	n := len(highs)
	if n > 5 {
		highs = highs[n-5:]
	}
	n = len(lows)
	if n > 5 {
		lows = lows[n-5:]
	}

	// Check for higher highs
	higherHighs := 0
	lowerHighs := 0
	for i := 1; i < len(highs); i++ {
		if highs[i] > highs[i-1] {
			higherHighs++
		} else if highs[i] < highs[i-1] {
			lowerHighs++
		}
	}

	// Check for higher lows
	higherLows := 0
	lowerLows := 0
	for i := 1; i < len(lows); i++ {
		if lows[i] > lows[i-1] {
			higherLows++
		} else if lows[i] < lows[i-1] {
			lowerLows++
		}
	}

	// Determine structure
	isUptrend := higherHighs > lowerHighs && higherLows > lowerLows
	isDowntrend := lowerHighs > higherHighs && lowerLows > higherLows

	if isUptrend {
		return "HH/HL (Uptrend confirmed)"
	}
	if isDowntrend {
		return "LH/LL (Downtrend confirmed)"
	}
	if higherHighs > lowerHighs {
		return "Higher highs, mixed lows (Potential reversal)"
	}
	if lowerLows > higherLows {
		return "Lower lows, mixed highs (Potential reversal)"
	}
	return "Mixed structure (Consolidation/Ranging)"
}

// InterpretTrendAlignment provides context about multi-timeframe alignment.
func InterpretTrendAlignment(trend1H, trend15M TrendDirection) string {
	if trend1H == "" && trend15M == "" {
		return "No trend data"
	}

	if trend1H == trend15M && trend1H != TrendNeutral {
		return fmt.Sprintf("Aligned %s on both 1H and 15M", trend1H)
	}

	if trend1H == TrendNeutral && trend15M == TrendNeutral {
		return "Both timeframes neutral (No clear direction)"
	}

	if trend1H == TrendNeutral || trend15M == TrendNeutral {
		nonNeutral := trend1H
		if trend1H == TrendNeutral {
			nonNeutral = trend15M
		}
		return fmt.Sprintf("Partial alignment: One timeframe %s, other neutral", nonNeutral)
	}

	return fmt.Sprintf("Divergent: 1H=%s, 15M=%s (Conflicting signals)", trend1H, trend15M)
}

// calculateRegimeConfidence estimates confidence in the regime classification.
func calculateRegimeConfidence(state *CoinState) float64 {
	if state == nil {
		return 0
	}

	confidence := 0.5 // Base confidence

	// ADX clarity adds confidence
	if state.ADX >= 30 || state.ADX < 15 {
		confidence += 0.2 // Clear ADX reading
	}

	// Trend alignment adds confidence
	if state.Trend1H == state.Trend15M && state.Trend1H != TrendNeutral {
		confidence += 0.15
	}

	// ATR consistency adds confidence
	if state.ATR > 0 && state.ATRAverage > 0 {
		ratio := state.ATR / state.ATRAverage
		if ratio > 1.3 || ratio < 0.7 {
			confidence += 0.15 // Clear volatility signal
		}
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// ========== LLM Formatting Methods ==========

// FormatForLLM returns a human-readable string optimized for LLM prompt injection.
// This format is designed for clear parsing by language models.
func (ctx *LLMContext) FormatForLLM() string {
	if ctx == nil {
		return ""
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%s Analysis:\n", ctx.Symbol))

	// Market Regime
	sb.WriteString(fmt.Sprintf("- Market Regime: %s (confidence: %.2f)\n", ctx.MarketRegime, ctx.RegimeConfidence))
	if ctx.ActiveStrategy != "" {
		sb.WriteString(fmt.Sprintf("- Active Strategy: %s\n", ctx.ActiveStrategy))
	}

	// ADX/Trend Strength
	sb.WriteString(fmt.Sprintf("- ADX: %s\n", ctx.ADXInterpretation))

	// Trend Analysis
	trendStatus := ""
	if ctx.TrendAlignment {
		trendStatus = "Aligned"
	} else {
		trendStatus = "Divergent"
	}
	sb.WriteString(fmt.Sprintf("- Trend: 1H=%s, 15M=%s (%s)\n", ctx.Trend1H, ctx.Trend15M, trendStatus))

	// Price Position
	if ctx.CurrentPrice > 0 {
		sb.WriteString(fmt.Sprintf("- Price Position: $%.2f - %s", ctx.CurrentPrice, ctx.PriceVsEMAs))
		if ctx.EMA9 > 0 || ctx.EMA21 > 0 || ctx.EMA50 > 0 {
			sb.WriteString(" (")
			var emas []string
			if ctx.EMA9 > 0 {
				emas = append(emas, fmt.Sprintf("EMA9: $%.2f", ctx.EMA9))
			}
			if ctx.EMA21 > 0 {
				emas = append(emas, fmt.Sprintf("EMA21: $%.2f", ctx.EMA21))
			}
			if ctx.EMA50 > 0 {
				emas = append(emas, fmt.Sprintf("EMA50: $%.2f", ctx.EMA50))
			}
			sb.WriteString(strings.Join(emas, ", "))
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}

	// VWAP
	if ctx.VWAPValue > 0 {
		sb.WriteString(fmt.Sprintf("- VWAP: %s\n", ctx.PriceVsVWAP))
	}

	// Market Structure
	if ctx.MarketStructure != "" && ctx.MarketStructure != "Insufficient data" {
		sb.WriteString(fmt.Sprintf("- Market Structure: %s\n", ctx.MarketStructure))
	}

	// BTC Context
	if ctx.BTCTrend != "" {
		corrStr := ""
		if ctx.BTCCorrelation != 0 {
			corrStr = fmt.Sprintf(" (Correlation: %.2f)", ctx.BTCCorrelation)
		}
		sb.WriteString(fmt.Sprintf("- BTC: %s%s\n", ctx.BTCTrend, corrStr))
	}

	// Support/Resistance
	if ctx.NearestSupport > 0 || ctx.NearestResistance > 0 {
		sb.WriteString("- ")
		if ctx.NearestSupport > 0 {
			sb.WriteString(fmt.Sprintf("Support: $%.2f (%.1f%% away)", ctx.NearestSupport, ctx.DistanceToSupport))
		}
		if ctx.NearestSupport > 0 && ctx.NearestResistance > 0 {
			sb.WriteString(" | ")
		}
		if ctx.NearestResistance > 0 {
			sb.WriteString(fmt.Sprintf("Resistance: $%.2f (%.1f%% away)", ctx.NearestResistance, ctx.DistanceToResistance))
		}
		sb.WriteString("\n")
	}

	// Volume
	if ctx.VolumeInterpretation != "" && ctx.VolumeInterpretation != "No volume data" {
		sb.WriteString(fmt.Sprintf("- Volume: %s\n", ctx.VolumeInterpretation))
	}

	// ATR/Volatility
	if ctx.ATRInterpretation != "" && ctx.ATRInterpretation != "No data" {
		sb.WriteString(fmt.Sprintf("- Volatility: %s\n", ctx.ATRInterpretation))
	}

	// RSI
	sb.WriteString(fmt.Sprintf("- RSI: %s\n", ctx.RSIInterpretation))

	// Scores
	if ctx.FinalScore > 0 {
		sb.WriteString(fmt.Sprintf("- Score: %d/100 (Technical: %d%%, Context: %d%%)\n",
			ctx.FinalScore, ctx.TechnicalScore, ctx.ContextScore))
	}

	// Blocking Status
	if ctx.IsBlocked && len(ctx.BlockingReasons) > 0 {
		sb.WriteString(fmt.Sprintf("- BLOCKED: %s\n", strings.Join(ctx.BlockingReasons, "; ")))
	}

	return sb.String()
}

// FormatForLLMCompact returns a shorter format suitable for token-constrained prompts.
func (ctx *LLMContext) FormatForLLMCompact() string {
	if ctx == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s: ", ctx.Symbol))

	// Key metrics only
	parts := []string{}

	parts = append(parts, fmt.Sprintf("Regime=%s", ctx.MarketRegime))

	if ctx.ADXValue > 0 {
		parts = append(parts, fmt.Sprintf("ADX=%.1f", ctx.ADXValue))
	}

	parts = append(parts, fmt.Sprintf("1H=%s/15M=%s", ctx.Trend1H, ctx.Trend15M))

	if ctx.RSI > 0 {
		parts = append(parts, fmt.Sprintf("RSI=%.1f", ctx.RSI))
	}

	if ctx.FinalScore > 0 {
		parts = append(parts, fmt.Sprintf("Score=%d", ctx.FinalScore))
	}

	if ctx.IsBlocked {
		parts = append(parts, "BLOCKED")
	}

	sb.WriteString(strings.Join(parts, " | "))

	return sb.String()
}

// llmJSONOutput is an internal struct for generating valid JSON output.
// This ensures proper JSON escaping and valid output format.
type llmJSONOutput struct {
	Symbol            string  `json:"symbol"`
	Regime            string  `json:"regime"`
	RegimeConfidence  float64 `json:"regime_confidence"`
	ADX               float64 `json:"adx"`
	ADXInterpretation string  `json:"adx_interpretation"`
	Trend1H           string  `json:"trend_1h"`
	Trend15M          string  `json:"trend_15m"`
	TrendAligned      bool    `json:"trend_aligned"`
	Price             float64 `json:"price"`
	PriceVsEMAs       string  `json:"price_vs_emas"`
	RSI               float64 `json:"rsi"`
	RSIInterpretation string  `json:"rsi_interpretation"`
	FinalScore        int     `json:"final_score"`
	IsBlocked         bool    `json:"is_blocked"`
}

// FormatForLLMJSON returns the context as valid JSON.
// Uses encoding/json for proper escaping and valid JSON output.
// This is useful when LLMs prefer structured data.
func (ctx *LLMContext) FormatForLLMJSON() string {
	if ctx == nil {
		return "{}"
	}

	output := llmJSONOutput{
		Symbol:            ctx.Symbol,
		Regime:            string(ctx.MarketRegime),
		RegimeConfidence:  ctx.RegimeConfidence,
		ADX:               ctx.ADXValue,
		ADXInterpretation: ctx.ADXInterpretation,
		Trend1H:           string(ctx.Trend1H),
		Trend15M:          string(ctx.Trend15M),
		TrendAligned:      ctx.TrendAlignment,
		Price:             ctx.CurrentPrice,
		PriceVsEMAs:       ctx.PriceVsEMAs,
		RSI:               ctx.RSI,
		RSIInterpretation: ctx.RSIInterpretation,
		FinalScore:        ctx.FinalScore,
		IsBlocked:         ctx.IsBlocked,
	}

	// Use MarshalIndent for readable output
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		// Fallback to minimal valid JSON on error
		return "{}"
	}

	return string(data)
}

// ToPromptSection returns a formatted section suitable for embedding in LLM prompts.
// Includes clear section markers for easy parsing.
func (ctx *LLMContext) ToPromptSection() string {
	if ctx == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("=== MARKET ANALYSIS ===\n")
	sb.WriteString(ctx.FormatForLLM())
	sb.WriteString("=== END ANALYSIS ===\n")

	return sb.String()
}

// GetDirectionalBias returns a simple directional recommendation based on all factors.
// Returns "BULLISH", "BEARISH", or "NEUTRAL" with a confidence score.
func (ctx *LLMContext) GetDirectionalBias() (string, float64) {
	if ctx == nil {
		return "NEUTRAL", 0
	}

	bullishPoints := 0.0
	bearishPoints := 0.0
	totalFactors := 0.0

	// Trend alignment
	if ctx.TrendAlignment {
		totalFactors++
		if ctx.Trend1H == TrendBullish {
			bullishPoints++
		} else if ctx.Trend1H == TrendBearish {
			bearishPoints++
		}
	}

	// RSI
	if ctx.RSI > 0 {
		totalFactors++
		if ctx.RSI > 50 && ctx.RSI < 70 {
			bullishPoints += 0.5
		} else if ctx.RSI < 50 && ctx.RSI > 30 {
			bearishPoints += 0.5
		}
	}

	// Market Structure
	if strings.Contains(ctx.MarketStructure, "Uptrend") {
		totalFactors++
		bullishPoints++
	} else if strings.Contains(ctx.MarketStructure, "Downtrend") {
		totalFactors++
		bearishPoints++
	}

	// BTC Trend
	if ctx.BTCTrend == "BULLISH" {
		totalFactors++
		bullishPoints += 0.5
	} else if ctx.BTCTrend == "BEARISH" {
		totalFactors++
		bearishPoints += 0.5
	}

	// Calculate bias
	if totalFactors == 0 {
		return "NEUTRAL", 0
	}

	netScore := (bullishPoints - bearishPoints) / totalFactors
	confidence := math.Abs(netScore)

	if netScore > 0.2 {
		return "BULLISH", confidence
	} else if netScore < -0.2 {
		return "BEARISH", confidence
	}
	return "NEUTRAL", confidence
}
