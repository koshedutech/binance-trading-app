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
}

// BreakoutStrategy implements a strategy that triggers when price breaks above previous candle's high
type BreakoutStrategy struct {
	config *BreakoutConfig
}

func NewBreakoutStrategy(config *BreakoutConfig) *BreakoutStrategy {
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
	if len(klines) < 2 {
		return &Signal{Type: SignalNone}, nil
	}

	// Get the last completed candle
	lastCandle := klines[len(klines)-2]
	
	// Check if minimum volume requirement is met
	if s.config.MinVolume > 0 && lastCandle.Volume < s.config.MinVolume {
		return &Signal{Type: SignalNone}, nil
	}

	// Check if current price breaks above the last candle's high
	if currentPrice > lastCandle.High {
		entryPrice := currentPrice
		stopLoss := entryPrice * (1 - s.config.StopLoss)
		takeProfit := entryPrice * (1 + s.config.TakeProfit)

		return &Signal{
			Type:       SignalBuy,
			Symbol:     s.config.Symbol,
			EntryPrice: entryPrice,
			StopLoss:   stopLoss,
			TakeProfit: takeProfit,
			OrderType:  s.config.OrderType,
			Side:       s.config.OrderSide,
			Reason:     fmt.Sprintf("Price %.2f broke above last candle high %.2f", currentPrice, lastCandle.High),
			Timestamp:  time.Now(),
		}, nil
	}

	return &Signal{Type: SignalNone}, nil
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
}

// SupportStrategy implements a strategy that triggers when price comes near previous candle's low
type SupportStrategy struct {
	config *SupportConfig
}

func NewSupportStrategy(config *SupportConfig) *SupportStrategy {
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
	if len(klines) < 2 {
		return &Signal{Type: SignalNone}, nil
	}

	// Get the last completed candle
	lastCandle := klines[len(klines)-2]
	
	// Calculate the distance threshold
	touchThreshold := lastCandle.Low * (1 + s.config.TouchDistance)

	// Check if current price is near the last candle's low (within threshold)
	if currentPrice <= touchThreshold && currentPrice >= lastCandle.Low {
		entryPrice := currentPrice
		stopLoss := entryPrice * (1 - s.config.StopLoss)
		takeProfit := entryPrice * (1 + s.config.TakeProfit)

		return &Signal{
			Type:       SignalBuy,
			Symbol:     s.config.Symbol,
			EntryPrice: entryPrice,
			StopLoss:   stopLoss,
			TakeProfit: takeProfit,
			OrderType:  s.config.OrderType,
			Side:       s.config.OrderSide,
			Reason:     fmt.Sprintf("Price %.2f touched near last candle low %.2f", currentPrice, lastCandle.Low),
			Timestamp:  time.Now(),
		}, nil
	}

	return &Signal{Type: SignalNone}, nil
}
