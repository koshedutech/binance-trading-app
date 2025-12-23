package strategy

import (
	"binance-trading-bot/internal/binance"
	"fmt"
	"time"
)

// Strategy defines the interface for trading strategies
type Strategy interface {
	// Name returns the strategy name
	Name() string
	
	// Evaluate checks if conditions are met for order placement
	Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error)
	
	// GetSymbol returns the symbol this strategy trades
	GetSymbol() string
	
	// GetInterval returns the candle interval
	GetInterval() string
}

// Signal represents a trading signal
type Signal struct {
	Type       SignalType
	Symbol     string
	EntryPrice float64
	StopLoss   float64
	TakeProfit float64
	Quantity   float64
	OrderType  string // LIMIT, MARKET, STOP_LOSS_LIMIT
	Side       string // BUY, SELL
	Reason     string
	Timestamp  time.Time
}

type SignalType string

const (
	SignalBuy  SignalType = "BUY"
	SignalSell SignalType = "SELL"
	SignalNone SignalType = "NONE"
)

// BreakoutConfig configures the breakout strategy
type BreakoutConfig struct {
	Symbol       string
	Interval     string
	OrderType    string
	OrderSide    string
	PositionSize float64 // As percentage of balance
	StopLoss     float64 // As percentage
	TakeProfit   float64 // As percentage
	MinVolume    float64 // Minimum volume requirement
	// Enhanced filters
	RequireTrendAlignment bool    // Only trade in direction of trend
	RequireVolumeSpike    bool    // Require volume confirmation
	VolumeMultiplier      float64 // Volume spike multiplier (e.g., 1.5)
	RequireRSIFilter      bool    // Use RSI to avoid overbought/oversold
	RSIOverbought         float64 // RSI level considered overbought (default 70)
	RSIOversold           float64 // RSI level considered oversold (default 30)
	EMAPeriod             int     // EMA period for trend (default 20)
	RSIPeriod             int     // RSI period (default 14)
	VolumePeriod          int     // Volume average period (default 20)
}

// BreakoutStrategy implements a strategy that triggers when price breaks above previous candle's high
// Enhanced with trend, volume, and RSI filters for improved win rate
type BreakoutStrategy struct {
	config *BreakoutConfig
}

func NewBreakoutStrategy(config *BreakoutConfig) *BreakoutStrategy {
	// Set defaults for enhanced filters
	if config.VolumeMultiplier == 0 {
		config.VolumeMultiplier = 1.5
	}
	if config.RSIOverbought == 0 {
		config.RSIOverbought = 70
	}
	if config.RSIOversold == 0 {
		config.RSIOversold = 30
	}
	if config.EMAPeriod == 0 {
		config.EMAPeriod = 20
	}
	if config.RSIPeriod == 0 {
		config.RSIPeriod = 14
	}
	if config.VolumePeriod == 0 {
		config.VolumePeriod = 20
	}
	// Enable filters by default for better win rate
	config.RequireTrendAlignment = true
	config.RequireVolumeSpike = true
	config.RequireRSIFilter = true

	return &BreakoutStrategy{
		config: config,
	}
}

func (s *BreakoutStrategy) Name() string {
	return fmt.Sprintf("Breakout-%s-%s", s.config.Symbol, s.config.Interval)
}

func (s *BreakoutStrategy) GetSymbol() string {
	return s.config.Symbol
}

func (s *BreakoutStrategy) GetInterval() string {
	return s.config.Interval
}

func (s *BreakoutStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < s.config.EMAPeriod+1 {
		return &Signal{Type: SignalNone}, nil
	}

	// Get the last completed candle
	lastCandle := klines[len(klines)-2]

	// Check if minimum volume requirement is met
	if s.config.MinVolume > 0 && lastCandle.Volume < s.config.MinVolume {
		return &Signal{Type: SignalNone}, nil
	}

	// Check if current price breaks above the last candle's high
	if currentPrice <= lastCandle.High {
		return &Signal{Type: SignalNone}, nil
	}

	// ==================== ENHANCED FILTERS ====================

	// 1. TREND FILTER: Only go long if price is above EMA (uptrend)
	if s.config.RequireTrendAlignment {
		ema := CalculateEMA(klines, s.config.EMAPeriod)
		if currentPrice < ema {
			// Price below EMA = downtrend, skip long breakout
			return &Signal{Type: SignalNone}, nil
		}
	}

	// 2. VOLUME FILTER: Require volume spike on breakout
	if s.config.RequireVolumeSpike {
		if !IsVolumeSpike(klines, s.config.VolumePeriod, s.config.VolumeMultiplier) {
			// No volume confirmation, weak breakout
			return &Signal{Type: SignalNone}, nil
		}
	}

	// 3. RSI FILTER: Avoid buying when overbought
	if s.config.RequireRSIFilter {
		rsi := CalculateRSI(klines, s.config.RSIPeriod)
		if rsi > s.config.RSIOverbought {
			// RSI overbought, skip entry
			return &Signal{Type: SignalNone}, nil
		}
	}

	// ==================== ALL FILTERS PASSED ====================

	entryPrice := currentPrice
	stopLoss := entryPrice * (1 - s.config.StopLoss)
	takeProfit := entryPrice * (1 + s.config.TakeProfit)

	// Build reason with confirmation details
	ema := CalculateEMA(klines, s.config.EMAPeriod)
	rsi := CalculateRSI(klines, s.config.RSIPeriod)
	reason := fmt.Sprintf("Breakout: Price %.2f > High %.2f | EMA20: %.2f ✓ | RSI: %.1f ✓ | Volume ✓",
		currentPrice, lastCandle.High, ema, rsi)

	return &Signal{
		Type:       SignalBuy,
		Symbol:     s.config.Symbol,
		EntryPrice: entryPrice,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
		OrderType:  s.config.OrderType,
		Side:       s.config.OrderSide,
		Reason:     reason,
		Timestamp:  time.Now(),
	}, nil
}

// SupportConfig configures the support strategy
type SupportConfig struct {
	Symbol        string
	Interval      string
	OrderType     string
	OrderSide     string
	PositionSize  float64
	StopLoss      float64
	TakeProfit    float64
	TouchDistance float64 // Distance threshold to consider price "touching" the low
	// Enhanced filters
	RequireTrendAlignment bool    // Only trade in direction of trend
	RequireVolumeSpike    bool    // Require volume on bounce
	VolumeMultiplier      float64 // Volume spike multiplier
	RequireRSIFilter      bool    // Use RSI to confirm oversold
	RSIOversold           float64 // RSI level considered oversold (default 35)
	RequireBounceCandle   bool    // Require bullish candle to confirm bounce
	EMAPeriod             int     // EMA period for trend
	RSIPeriod             int     // RSI period
	VolumePeriod          int     // Volume average period
}

// SupportStrategy implements a strategy that triggers when price comes near previous candle's low
// Enhanced with trend, volume, RSI, and bounce confirmation filters
type SupportStrategy struct {
	config *SupportConfig
}

func NewSupportStrategy(config *SupportConfig) *SupportStrategy {
	// Set defaults for enhanced filters
	if config.VolumeMultiplier == 0 {
		config.VolumeMultiplier = 1.3 // Slightly lower for support bounces
	}
	if config.RSIOversold == 0 {
		config.RSIOversold = 35 // More lenient for support
	}
	if config.EMAPeriod == 0 {
		config.EMAPeriod = 20
	}
	if config.RSIPeriod == 0 {
		config.RSIPeriod = 14
	}
	if config.VolumePeriod == 0 {
		config.VolumePeriod = 20
	}
	// Enable filters by default
	config.RequireTrendAlignment = true
	config.RequireRSIFilter = true
	config.RequireBounceCandle = true
	config.RequireVolumeSpike = false // Volume less critical for support

	return &SupportStrategy{
		config: config,
	}
}

func (s *SupportStrategy) Name() string {
	return fmt.Sprintf("Support-%s-%s", s.config.Symbol, s.config.Interval)
}

func (s *SupportStrategy) GetSymbol() string {
	return s.config.Symbol
}

func (s *SupportStrategy) GetInterval() string {
	return s.config.Interval
}

func (s *SupportStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < s.config.EMAPeriod+1 {
		return &Signal{Type: SignalNone}, nil
	}

	// Get the last two completed candles
	lastCandle := klines[len(klines)-2]
	prevCandle := klines[len(klines)-3]

	// Calculate the distance threshold
	touchThreshold := lastCandle.Low * (1 + s.config.TouchDistance)

	// Check if current price is near the last candle's low (within threshold)
	if currentPrice > touchThreshold || currentPrice < lastCandle.Low*0.995 {
		return &Signal{Type: SignalNone}, nil
	}

	// ==================== ENHANCED FILTERS ====================

	// 1. TREND FILTER: For support buys, we want price not too far below EMA
	//    (catching dips in uptrend, or oversold in sideways market)
	if s.config.RequireTrendAlignment {
		ema := CalculateEMA(klines, s.config.EMAPeriod)
		// Allow support plays within 3% of EMA (not deep downtrend)
		if currentPrice < ema*0.97 {
			// Too far below EMA, deep downtrend - dangerous to catch falling knife
			return &Signal{Type: SignalNone}, nil
		}
	}

	// 2. RSI FILTER: For support, we WANT oversold conditions (good for bounce)
	if s.config.RequireRSIFilter {
		rsi := CalculateRSI(klines, s.config.RSIPeriod)
		// Only buy support if RSI shows some weakness (not overbought)
		if rsi > 60 {
			// RSI not showing oversold, less likely to bounce
			return &Signal{Type: SignalNone}, nil
		}
	}

	// 3. BOUNCE CANDLE FILTER: Last candle should show buying pressure
	if s.config.RequireBounceCandle {
		// Check if last candle closed above its midpoint (buying pressure)
		candleMid := (lastCandle.High + lastCandle.Low) / 2
		if lastCandle.Close < candleMid {
			// Candle closed weak, no bounce confirmation
			return &Signal{Type: SignalNone}, nil
		}
		// Also check for higher low (early reversal sign)
		if lastCandle.Low < prevCandle.Low*0.998 {
			// Making new lows, not bouncing yet
			return &Signal{Type: SignalNone}, nil
		}
	}

	// 4. VOLUME FILTER (optional): Volume on bounce
	if s.config.RequireVolumeSpike {
		if !IsVolumeSpike(klines, s.config.VolumePeriod, s.config.VolumeMultiplier) {
			return &Signal{Type: SignalNone}, nil
		}
	}

	// ==================== ALL FILTERS PASSED ====================

	entryPrice := currentPrice
	stopLoss := entryPrice * (1 - s.config.StopLoss)
	takeProfit := entryPrice * (1 + s.config.TakeProfit)

	// Build reason with confirmation details
	ema := CalculateEMA(klines, s.config.EMAPeriod)
	rsi := CalculateRSI(klines, s.config.RSIPeriod)
	reason := fmt.Sprintf("Support Bounce: Price %.2f near Low %.2f | EMA20: %.2f | RSI: %.1f ✓ | Bounce ✓",
		currentPrice, lastCandle.Low, ema, rsi)

	return &Signal{
		Type:       SignalBuy,
		Symbol:     s.config.Symbol,
		EntryPrice: entryPrice,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
		OrderType:  s.config.OrderType,
		Side:       s.config.OrderSide,
		Reason:     reason,
		Timestamp:  time.Now(),
	}, nil
}
