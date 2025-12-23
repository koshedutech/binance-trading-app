package autopilot

import (
	"time"
)

// TradingStyle represents the user's selected trading style
type TradingStyle string

const (
	StyleUltraFast TradingStyle = "ultra_fast"
	StyleScalping  TradingStyle = "scalping"
	StyleSwing     TradingStyle = "swing"
	StylePosition  TradingStyle = "position"
)

// TradingStyleConfig holds configuration for a trading style
type TradingStyleConfig struct {
	// Style identifier
	Style TradingStyle `json:"style"`
	Name  string       `json:"name"`

	// Leverage settings
	DefaultLeverage int `json:"default_leverage"`
	MaxLeverage     int `json:"max_leverage"`

	// SL/TP ATR multiples
	SLATRMultiple float64 `json:"sl_atr_multiple"`
	TPATRMultiple float64 `json:"tp_atr_multiple"`

	// Hold time constraints
	MinHoldTime time.Duration `json:"min_hold_time"`
	MaxHoldTime time.Duration `json:"max_hold_time"` // 0 = unlimited

	// Position management
	AllowAveraging  bool `json:"allow_averaging"`
	MaxEntries      int  `json:"max_entries"`       // Max position entries for averaging
	AllowHedging    bool `json:"allow_hedging"`

	// Signal requirements
	MinConfidence      float64 `json:"min_confidence"`
	RequiredConfluence int     `json:"required_confluence"`

	// Timeframe settings
	TrendTimeframes  []string `json:"trend_timeframes"`   // For MTF analysis
	SignalTimeframe  string   `json:"signal_timeframe"`   // For AI signals
	EntryTimeframe   string   `json:"entry_timeframe"`    // For entry timing

	// Profit taking
	QuickProfitEnabled   bool    `json:"quick_profit_enabled"`
	QuickProfitPercent   float64 `json:"quick_profit_percent"`
	TrailingStopEnabled  bool    `json:"trailing_stop_enabled"`
	TrailingStopPercent  float64 `json:"trailing_stop_percent"`

	// Risk management
	MaxPositionPercent   float64 `json:"max_position_percent"`   // Max % of balance per position
	MaxDailyDrawdown     float64 `json:"max_daily_drawdown"`     // Max daily loss %
}

// GetDefaultStyleConfig returns the default configuration for a trading style
func GetDefaultStyleConfig(style TradingStyle) *TradingStyleConfig {
	switch style {
	case StyleUltraFast:
		return &TradingStyleConfig{
			Style:                style,
			Name:                 "Ultra-Fast Scalping",
			DefaultLeverage:      10,
			MaxLeverage:          20,
			SLATRMultiple:        0.3,  // Tight SL for quick exits
			TPATRMultiple:        0.5,  // Dynamic TP based on fees + ATR
			MinHoldTime:          100 * time.Millisecond,
			MaxHoldTime:          3 * time.Second,              // Force exit after 3 seconds
			AllowAveraging:       false,
			MaxEntries:           1,
			AllowHedging:         false,
			MinConfidence:        0.50,  // Moderately low - catching quick moves
			RequiredConfluence:   1,
			TrendTimeframes:      []string{"5m"},              // 5m candles for volatility regime
			SignalTimeframe:      "1m",                        // 1m candles for entry triggers
			EntryTimeframe:       "1m",                        // Monitor 1m candles
			QuickProfitEnabled:   true,                        // Always quick profit
			QuickProfitPercent:   0.0,  // Dynamic calculation (fee-aware)
			TrailingStopEnabled:  false,                       // No trailing on ultra-fast
			TrailingStopPercent:  0.0,
			MaxPositionPercent:   3.0,   // Conservative - 3% of balance per trade
			MaxDailyDrawdown:     2.0,   // Tight daily loss control
		}
	case StyleScalping:
		return &TradingStyleConfig{
			Style:                style,
			Name:                 "Scalping",
			DefaultLeverage:      10,
			MaxLeverage:          20,
			SLATRMultiple:        0.5,
			TPATRMultiple:        1.0,
			MinHoldTime:          30 * time.Second,
			MaxHoldTime:          15 * time.Minute,
			AllowAveraging:       false,
			MaxEntries:           1,
			AllowHedging:         false,
			MinConfidence:        0.15, // Lowered to allow more trades
			RequiredConfluence:   1,
			TrendTimeframes:      []string{"15m"},
			SignalTimeframe:      "1m",
			EntryTimeframe:       "1m",
			QuickProfitEnabled:   true,
			QuickProfitPercent:   0.5, // Take profit at 0.5%
			TrailingStopEnabled:  false,
			TrailingStopPercent:  0.3,
			MaxPositionPercent:   5.0,  // 5% of balance per trade
			MaxDailyDrawdown:     3.0,  // 3% max daily loss
		}
	case StyleSwing:
		return &TradingStyleConfig{
			Style:                style,
			Name:                 "Swing Trading",
			DefaultLeverage:      5,
			MaxLeverage:          10,
			SLATRMultiple:        1.5,
			TPATRMultiple:        3.0,
			MinHoldTime:          1 * time.Hour,
			MaxHoldTime:          0, // Unlimited
			AllowAveraging:       true,
			MaxEntries:           3,
			AllowHedging:         false,
			MinConfidence:        0.60,
			RequiredConfluence:   2,
			TrendTimeframes:      []string{"1d", "4h", "1h"},
			SignalTimeframe:      "15m",
			EntryTimeframe:       "5m",
			QuickProfitEnabled:   false,
			QuickProfitPercent:   0,
			TrailingStopEnabled:  true,
			TrailingStopPercent:  1.5,
			MaxPositionPercent:   10.0, // 10% of balance per trade
			MaxDailyDrawdown:     5.0,  // 5% max daily loss
		}
	case StylePosition:
		return &TradingStyleConfig{
			Style:                style,
			Name:                 "Position Trading",
			DefaultLeverage:      2,
			MaxLeverage:          3,
			SLATRMultiple:        3.0,
			TPATRMultiple:        6.0,
			MinHoldTime:          24 * time.Hour,
			MaxHoldTime:          0, // Unlimited
			AllowAveraging:       true,
			MaxEntries:           5,
			AllowHedging:         true, // Only position trading allows hedging
			MinConfidence:        0.70,
			RequiredConfluence:   3,
			TrendTimeframes:      []string{"1w", "1d", "4h"},
			SignalTimeframe:      "4h",
			EntryTimeframe:       "1h",
			QuickProfitEnabled:   false,
			QuickProfitPercent:   0,
			TrailingStopEnabled:  true,
			TrailingStopPercent:  3.0,
			MaxPositionPercent:   20.0, // 20% of balance per trade
			MaxDailyDrawdown:     10.0, // 10% max daily loss (longer term, more room)
		}
	default:
		// Return swing as default
		return GetDefaultStyleConfig(StyleSwing)
	}
}

// TradingStyleSettings stores user-customized style settings
type TradingStyleSettings struct {
	// Currently selected style
	ActiveStyle TradingStyle `json:"active_style"`

	// Per-style custom configurations (overrides)
	ScalpingOverrides  map[string]interface{} `json:"scalping_overrides,omitempty"`
	SwingOverrides     map[string]interface{} `json:"swing_overrides,omitempty"`
	PositionOverrides  map[string]interface{} `json:"position_overrides,omitempty"`
}

// NewDefaultTradingStyleSettings creates default settings
func NewDefaultTradingStyleSettings() *TradingStyleSettings {
	return &TradingStyleSettings{
		ActiveStyle: StyleSwing, // Default to swing trading
	}
}

// GetStyleDescription returns a human-readable description of the style
func GetStyleDescription(style TradingStyle) string {
	switch style {
	case StyleUltraFast:
		return "1-3 second exits with fee-aware profit targets. Extreme leverage, 500ms monitoring, adaptive volatility-based re-entry. Maximum risk control."
	case StyleScalping:
		return "Quick entry and exit for small profits. High leverage, tight SL/TP, no position averaging."
	case StyleSwing:
		return "Hold for hours to days. Medium leverage, allows position averaging up to 3 entries."
	case StylePosition:
		return "Long-term holds for large moves. Low leverage, wide SL/TP, allows averaging and hedging."
	default:
		return "Unknown trading style"
	}
}

// StyleComparisonTable returns a comparison table for UI display
type StyleComparison struct {
	Parameter   string `json:"parameter"`
	UltraFast   string `json:"ultra_fast"`
	Scalping    string `json:"scalping"`
	Swing       string `json:"swing"`
	Position    string `json:"position"`
}

func GetStyleComparisonTable() []StyleComparison {
	return []StyleComparison{
		{"Default Leverage", "10x", "10x", "5x", "2x"},
		{"Max Leverage", "20x", "20x", "10x", "3x"},
		{"SL ATR Multiple", "0.3x", "0.5x", "1.5x", "3.0x"},
		{"TP ATR Multiple", "Dynamic", "1.0x", "3.0x", "6.0x"},
		{"Min Hold Time", "100ms", "30s", "1h", "24h"},
		{"Max Hold Time", "3s", "15m", "Unlimited", "Unlimited"},
		{"Allow Averaging", "No", "No", "Yes (3)", "Yes (5)"},
		{"Allow Hedging", "No", "No", "No", "Yes"},
		{"Min Confidence", "50%", "55%", "60%", "70%"},
		{"Required Confluence", "1", "1", "2", "3"},
		{"Trend Timeframes", "5m", "15m", "1D,4H,1H", "1W,1D,4H"},
		{"Signal Timeframe", "1m", "1m", "15m", "4h"},
		{"Entry Timeframe", "1m", "1m", "5m", "1h"},
		{"Monitor Interval", "500ms", "5s", "60s", "5m"},
		{"Position % Max", "3%", "5%", "10%", "20%"},
	}
}

// ValidateStyle validates if a trading style string is valid
func ValidateStyle(style string) (TradingStyle, bool) {
	switch TradingStyle(style) {
	case StyleUltraFast, StyleScalping, StyleSwing, StylePosition:
		return TradingStyle(style), true
	default:
		return "", false
	}
}

// GetAllStyles returns all available trading styles
func GetAllStyles() []TradingStyle {
	return []TradingStyle{StyleUltraFast, StyleScalping, StyleSwing, StylePosition}
}
