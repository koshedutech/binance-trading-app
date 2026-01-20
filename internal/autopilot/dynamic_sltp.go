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
		MaxSLPercent:    3.5,
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

// ==================== STORY 10.1: EFFICIENCY-AWARE DYNAMIC SL/TP ====================
// These functions calculate adaptive SL/TP levels based on ATR, ADX, profit, and efficiency.

// DynamicLevelConfig holds configuration for efficiency-aware dynamic SL/TP
type DynamicLevelConfig struct {
	// SL Configuration
	MinSLPercent float64 // Floor: 0.3%
	MaxSLPercent float64 // Cap: 3.0%

	// TP Configuration
	MinTPPercent float64 // Floor: 1.5%
	MaxTPPercent float64 // Cap: 8.0%

	// Enabled flags
	SLEnabled bool
	TPEnabled bool
}

// DefaultDynamicLevelConfig returns default configuration for efficiency-aware SL/TP
func DefaultDynamicLevelConfig() *DynamicLevelConfig {
	return &DynamicLevelConfig{
		MinSLPercent: 0.3,
		MaxSLPercent: 3.0,
		MinTPPercent: 1.5,
		MaxTPPercent: 8.0,
		SLEnabled:    true,
		TPEnabled:    true,
	}
}

// DynamicLevelResult holds the calculated SL/TP values with adjustment details
type DynamicLevelResult struct {
	SLPercent      float64  `json:"sl_percent"`       // Stop loss percentage from peak
	TPPercent      float64  `json:"tp_percent"`       // Take profit percentage from peak
	SLPrice        float64  `json:"sl_price"`         // Calculated SL price
	TPPrice        float64  `json:"tp_price"`         // Calculated TP price
	ATRPct         float64  `json:"atr_pct"`          // ATR percentage used
	ADX            float64  `json:"adx"`              // ADX value used
	ProfitPct      float64  `json:"profit_pct"`       // Current profit percentage
	Efficiency     float64  `json:"efficiency"`       // Position efficiency (0-1)
	Adjustments    []string `json:"adjustments"`      // List of adjustments applied
	SLImproved     bool     `json:"sl_improved"`      // True if SL is better than current
	TPImproved     bool     `json:"tp_improved"`      // True if TP is better than current
}

// CalculateDynamicSL calculates efficiency-aware stop loss percentage
// Story 10.1: Dynamic SL/TP Management
//
// Formula:
//   - BASE: 1.5x ATR% from peak price
//   - ADJUST for ADX: if ADX > 30, tighten 20%; if ADX < 20, widen 30%
//   - ADJUST for profit: if profit > 1%, tighten 10%
//   - ADJUST for efficiency: if efficiency < 50%, tighten 15%
//   - CLAMP to 0.3% - 3.0%
func CalculateDynamicSL(pos *GiniePosition, atrPct, adx float64) float64 {
	if pos == nil || atrPct <= 0 {
		return 0
	}

	// BASE: 1.5x ATR from peak price
	baseSL := atrPct * 1.5

	// ADJUST for ADX (trend strength)
	// Strong trend (ADX > 30): tighten by 20% (trend likely to continue)
	// Weak trend (ADX < 20): widen by 30% (more volatility expected)
	if adx > 30 {
		baseSL *= 0.8 // Tighten 20%
	} else if adx < 20 {
		baseSL *= 1.3 // Widen 30%
	}

	// ADJUST for profit (only tighten when profitable)
	// Protect profits by tightening SL when position is profitable
	profitPct := calculatePositionProfit(pos)
	if profitPct > 1.0 {
		baseSL *= 0.9 // Tighten 10%
	}

	// ADJUST for efficiency
	// Low efficiency means price is retracing from peak - be more conservative
	if pos.Efficiency > 0 && pos.Efficiency < 0.5 {
		baseSL *= 0.85 // Tighten 15%
	}

	// CLAMP to bounds
	return clamp(baseSL, 0.3, 3.0)
}

// CalculateDynamicTP calculates efficiency-aware take profit percentage
// Story 10.1: Dynamic SL/TP Management
//
// Formula:
//   - BASE: 3x ATR% above peak price
//   - ADJUST for ADX: if ADX > 35, aim 50% higher; if ADX < 20, aim 30% lower
//   - CLAMP to 1.5% - 8.0%
func CalculateDynamicTP(pos *GiniePosition, atrPct, adx float64) float64 {
	if pos == nil || atrPct <= 0 {
		return 0
	}

	// BASE: 3x ATR above current price
	baseTP := atrPct * 3.0

	// ADJUST for ADX (trend strength)
	// Very strong trend (ADX > 35): aim 50% higher (ride the trend)
	// Weak trend (ADX < 20): aim 30% lower (take profits quickly)
	if adx > 35 {
		baseTP *= 1.5 // Aim 50% higher
	} else if adx < 20 {
		baseTP *= 0.7 // Aim 30% lower
	}

	// CLAMP to bounds
	return clamp(baseTP, 1.5, 8.0)
}

// calculatePositionProfit calculates the current profit percentage for a position
func calculatePositionProfit(pos *GiniePosition) float64 {
	if pos == nil || pos.EntryPrice <= 0 {
		return 0
	}

	// Use highest/lowest price for profit calculation
	var peakPrice float64
	if pos.Side == "LONG" {
		peakPrice = pos.HighestPrice
		if peakPrice <= 0 {
			return 0
		}
		return ((peakPrice - pos.EntryPrice) / pos.EntryPrice) * 100
	} else {
		peakPrice = pos.LowestPrice
		if peakPrice <= 0 {
			return 0
		}
		return ((pos.EntryPrice - peakPrice) / pos.EntryPrice) * 100
	}
}

// CalculateUpdatedSLPrice calculates the new SL price based on dynamic percentage
// Returns the actual price level for the SL order
func CalculateUpdatedSLPrice(pos *GiniePosition, slPercent float64) float64 {
	if pos == nil || slPercent <= 0 {
		return 0
	}

	// Get peak price for SL calculation
	var peakPrice float64
	if pos.Side == "LONG" {
		peakPrice = pos.HighestPrice
		if peakPrice <= 0 {
			peakPrice = pos.EntryPrice
		}
		// SL is below peak for LONG
		return peakPrice * (1 - slPercent/100)
	} else {
		peakPrice = pos.LowestPrice
		if peakPrice <= 0 {
			peakPrice = pos.EntryPrice
		}
		// SL is above peak for SHORT
		return peakPrice * (1 + slPercent/100)
	}
}

// CalculateUpdatedTPPrice calculates the new TP price based on dynamic percentage
// Returns the actual price level for the TP order
func CalculateUpdatedTPPrice(pos *GiniePosition, tpPercent float64, currentPrice float64) float64 {
	if pos == nil || tpPercent <= 0 || currentPrice <= 0 {
		return 0
	}

	// TP is calculated from current price, not peak
	if pos.Side == "LONG" {
		// TP is above current for LONG
		return currentPrice * (1 + tpPercent/100)
	} else {
		// TP is below current for SHORT
		return currentPrice * (1 - tpPercent/100)
	}
}

// IsSLImproved checks if the new SL is better (more protective) than the current one
// For LONG: higher SL is better (locks in more profit)
// For SHORT: lower SL is better (locks in more profit)
func IsSLImproved(pos *GiniePosition, newSL float64) bool {
	if pos == nil || newSL <= 0 || pos.StopLoss <= 0 {
		return false
	}

	if pos.Side == "LONG" {
		return newSL > pos.StopLoss
	}
	return newSL < pos.StopLoss
}

// IsTPImproved checks if the new TP is better (more ambitious) than the current one
// For LONG: higher TP is better (more profit potential)
// For SHORT: lower TP is better (more profit potential)
func IsTPImproved(pos *GiniePosition, newTP float64) bool {
	if pos == nil || newTP <= 0 {
		return false
	}

	// If no current TP levels, any TP is an improvement
	if len(pos.TakeProfits) == 0 {
		return true
	}

	// Compare with the last (highest) TP level
	lastTP := pos.TakeProfits[len(pos.TakeProfits)-1].Price

	if pos.Side == "LONG" {
		return newTP > lastTP
	}
	return newTP < lastTP
}

// DynamicLevelCalculator provides a comprehensive dynamic SL/TP calculation
// that combines all adjustments and returns detailed results
func DynamicLevelCalculator(pos *GiniePosition, atrPct, adx, currentPrice float64, config *DynamicLevelConfig) *DynamicLevelResult {
	if pos == nil {
		return nil
	}

	if config == nil {
		config = DefaultDynamicLevelConfig()
	}

	result := &DynamicLevelResult{
		ATRPct:      atrPct,
		ADX:         adx,
		ProfitPct:   calculatePositionProfit(pos),
		Efficiency:  pos.Efficiency,
		Adjustments: make([]string, 0),
	}

	// Calculate SL
	if config.SLEnabled {
		// Start with base calculation
		baseSL := atrPct * 1.5
		result.Adjustments = append(result.Adjustments, formatAdjustment("Base SL", atrPct*1.5))

		// ADX adjustments
		if adx > 30 {
			baseSL *= 0.8
			result.Adjustments = append(result.Adjustments, "ADX>30: tighten 20%")
		} else if adx < 20 {
			baseSL *= 1.3
			result.Adjustments = append(result.Adjustments, "ADX<20: widen 30%")
		}

		// Profit adjustment
		if result.ProfitPct > 1.0 {
			baseSL *= 0.9
			result.Adjustments = append(result.Adjustments, "Profit>1%: tighten 10%")
		}

		// Efficiency adjustment
		if pos.Efficiency > 0 && pos.Efficiency < 0.5 {
			baseSL *= 0.85
			result.Adjustments = append(result.Adjustments, "Efficiency<50%: tighten 15%")
		}

		// Clamp
		result.SLPercent = clamp(baseSL, config.MinSLPercent, config.MaxSLPercent)
		result.SLPrice = CalculateUpdatedSLPrice(pos, result.SLPercent)
		result.SLImproved = IsSLImproved(pos, result.SLPrice)
	}

	// Calculate TP
	if config.TPEnabled {
		baseTP := atrPct * 3.0
		result.Adjustments = append(result.Adjustments, formatAdjustment("Base TP", atrPct*3.0))

		// ADX adjustments
		if adx > 35 {
			baseTP *= 1.5
			result.Adjustments = append(result.Adjustments, "ADX>35: aim 50% higher")
		} else if adx < 20 {
			baseTP *= 0.7
			result.Adjustments = append(result.Adjustments, "ADX<20: aim 30% lower")
		}

		// Clamp
		result.TPPercent = clamp(baseTP, config.MinTPPercent, config.MaxTPPercent)
		result.TPPrice = CalculateUpdatedTPPrice(pos, result.TPPercent, currentPrice)
		result.TPImproved = IsTPImproved(pos, result.TPPrice)
	}

	return result
}

func formatAdjustment(name string, value float64) string {
	return name + ": " + formatPercent(value)
}
