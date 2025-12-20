package autopilot

import (
	"math"
	"strconv"

	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/strategy"
)

// DynamicSLTPConfig holds configuration for dynamic SL/TP calculation
type DynamicSLTPConfig struct {
	Enabled         bool    `json:"enabled"`
	ATRPeriod       int     `json:"atr_period"`        // Default 14
	ATRMultiplierSL float64 `json:"atr_multiplier_sl"` // Default 1.5
	ATRMultiplierTP float64 `json:"atr_multiplier_tp"` // Default 2.0
	MinSLPercent    float64 `json:"min_sl_percent"`    // Floor: 0.3%
	MaxSLPercent    float64 `json:"max_sl_percent"`    // Cap: 3%
	MinTPPercent    float64 `json:"min_tp_percent"`    // Floor: 0.5%
	MaxTPPercent    float64 `json:"max_tp_percent"`    // Cap: 5%
	LLMWeight       float64 `json:"llm_weight"`        // Weight for LLM adjustment (0-1)
}

// DefaultDynamicSLTPConfig returns sensible default configuration
func DefaultDynamicSLTPConfig() *DynamicSLTPConfig {
	return &DynamicSLTPConfig{
		Enabled:         false,
		ATRPeriod:       14,
		ATRMultiplierSL: 1.5,
		ATRMultiplierTP: 2.0,
		MinSLPercent:    0.3,
		MaxSLPercent:    3.0,
		MinTPPercent:    0.5,
		MaxTPPercent:    5.0,
		LLMWeight:       0.3, // 30% LLM, 70% ATR by default
	}
}

// DynamicSLTPResult holds the calculated SL/TP values
type DynamicSLTPResult struct {
	StopLossPrice    float64 `json:"stop_loss_price"`
	TakeProfitPrice  float64 `json:"take_profit_price"`
	StopLossPercent  float64 `json:"stop_loss_percent"`
	TakeProfitPercent float64 `json:"take_profit_percent"`
	ATRValue         float64 `json:"atr_value"`
	ATRPercent       float64 `json:"atr_percent"`
	UsedLLM          bool    `json:"used_llm"`
	Reasoning        string  `json:"reasoning"`
}

// CalculateDynamicSLTP calculates SL/TP based on ATR and optionally LLM context
func CalculateDynamicSLTP(
	symbol string,
	currentPrice float64,
	klines []binance.Kline,
	side string, // "LONG" or "SHORT"
	llmAnalysis *llm.MarketAnalysis,
	config *DynamicSLTPConfig,
) *DynamicSLTPResult {
	if config == nil {
		config = DefaultDynamicSLTPConfig()
	}

	result := &DynamicSLTPResult{
		Reasoning: "Dynamic SL/TP calculation: ",
	}

	// Calculate ATR
	atr := strategy.CalculateATR(klines, config.ATRPeriod)
	if atr == 0 {
		// Fallback if not enough klines
		atr = currentPrice * 0.01 // 1% fallback
		result.Reasoning += "insufficient data, using 1% fallback; "
	}

	result.ATRValue = atr
	result.ATRPercent = (atr / currentPrice) * 100

	// Calculate base SL/TP from ATR
	baseSLPercent := (atr * config.ATRMultiplierSL / currentPrice) * 100
	baseTPPercent := (atr * config.ATRMultiplierTP / currentPrice) * 100

	result.Reasoning += "ATR-based SL: " + formatPercent(baseSLPercent) + ", TP: " + formatPercent(baseTPPercent) + "; "

	// Apply LLM adjustments if available
	finalSLPercent := baseSLPercent
	finalTPPercent := baseTPPercent

	if llmAnalysis != nil && config.LLMWeight > 0 {
		llmSLPercent := 0.0
		llmTPPercent := 0.0

		if llmAnalysis.StopLoss != nil && *llmAnalysis.StopLoss > 0 {
			// Calculate LLM SL percent from price
			if side == "LONG" {
				llmSLPercent = ((currentPrice - *llmAnalysis.StopLoss) / currentPrice) * 100
			} else {
				llmSLPercent = ((*llmAnalysis.StopLoss - currentPrice) / currentPrice) * 100
			}
			if llmSLPercent > 0 {
				result.UsedLLM = true
			}
		}

		if llmAnalysis.TakeProfit != nil && *llmAnalysis.TakeProfit > 0 {
			// Calculate LLM TP percent from price
			if side == "LONG" {
				llmTPPercent = ((*llmAnalysis.TakeProfit - currentPrice) / currentPrice) * 100
			} else {
				llmTPPercent = ((currentPrice - *llmAnalysis.TakeProfit) / currentPrice) * 100
			}
			if llmTPPercent > 0 {
				result.UsedLLM = true
			}
		}

		// Blend ATR and LLM suggestions
		if llmSLPercent > 0 {
			finalSLPercent = baseSLPercent*(1-config.LLMWeight) + llmSLPercent*config.LLMWeight
			result.Reasoning += "LLM SL adjustment applied (" + formatPercent(config.LLMWeight*100) + " weight); "
		}

		if llmTPPercent > 0 {
			finalTPPercent = baseTPPercent*(1-config.LLMWeight) + llmTPPercent*config.LLMWeight
			result.Reasoning += "LLM TP adjustment applied; "
		}

		// Adjust based on LLM risk level
		if llmAnalysis.RiskLevel != "" {
			switch llmAnalysis.RiskLevel {
			case "high":
				// Tighter SL for high risk
				finalSLPercent *= 0.8
				result.Reasoning += "high risk -> tighter SL; "
			case "low":
				// Wider SL for low risk, higher TP target
				finalSLPercent *= 1.2
				finalTPPercent *= 1.2
				result.Reasoning += "low risk -> wider SL/TP; "
			}
		}
	}

	// Clamp to min/max bounds
	finalSLPercent = clamp(finalSLPercent, config.MinSLPercent, config.MaxSLPercent)
	finalTPPercent = clamp(finalTPPercent, config.MinTPPercent, config.MaxTPPercent)

	result.StopLossPercent = finalSLPercent
	result.TakeProfitPercent = finalTPPercent

	// Calculate actual prices
	if side == "LONG" {
		result.StopLossPrice = currentPrice * (1 - finalSLPercent/100)
		result.TakeProfitPrice = currentPrice * (1 + finalTPPercent/100)
	} else {
		// SHORT position
		result.StopLossPrice = currentPrice * (1 + finalSLPercent/100)
		result.TakeProfitPrice = currentPrice * (1 - finalTPPercent/100)
	}

	result.Reasoning += "final SL: " + formatPercent(finalSLPercent) + ", TP: " + formatPercent(finalTPPercent)

	return result
}

// CalculateSymbolVolatility returns the volatility percentage for a symbol based on ATR
func CalculateSymbolVolatility(klines []binance.Kline, period int) float64 {
	if len(klines) == 0 {
		return 1.0 // Default 1% if no data
	}

	atr := strategy.CalculateATR(klines, period)
	currentPrice := klines[len(klines)-1].Close

	if currentPrice == 0 {
		return 1.0
	}

	return (atr / currentPrice) * 100
}

// GetVolatilityLevel returns a human-readable volatility level
func GetVolatilityLevel(volatilityPercent float64) string {
	switch {
	case volatilityPercent < 0.5:
		return "very_low"
	case volatilityPercent < 1.0:
		return "low"
	case volatilityPercent < 2.0:
		return "moderate"
	case volatilityPercent < 3.0:
		return "high"
	default:
		return "very_high"
	}
}

// Helper functions

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func formatPercent(value float64) string {
	return strconv.FormatFloat(math.Round(value*100)/100, 'f', 2, 64) + "%"
}
