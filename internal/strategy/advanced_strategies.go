package strategy

import (
	"binance-trading-bot/internal/binance"
	"fmt"
	"time"
)

// RSIStrategyConfig configures the RSI strategy
type RSIStrategyConfig struct {
	Symbol        string
	Interval      string
	RSIPeriod     int
	OversoldLevel float64 // e.g., 30
	OverboughtLevel float64 // e.g., 70
	PositionSize  float64
	StopLoss      float64
	TakeProfit    float64
}

// RSIStrategy implements a strategy based on RSI indicator
type RSIStrategy struct {
	config *RSIStrategyConfig
}

func NewRSIStrategy(config *RSIStrategyConfig) *RSIStrategy {
	return &RSIStrategy{config: config}
}

func (s *RSIStrategy) Name() string {
	return fmt.Sprintf("RSI-%s-%s", s.config.Symbol, s.config.Interval)
}

func (s *RSIStrategy) GetSymbol() string {
	return s.config.Symbol
}

func (s *RSIStrategy) GetInterval() string {
	return s.config.Interval
}

func (s *RSIStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < s.config.RSIPeriod+1 {
		return &Signal{Type: SignalNone}, nil
	}

	// Calculate RSI
	rsi := calculateRSI(klines, s.config.RSIPeriod)

	// Oversold - Buy signal
	if rsi < s.config.OversoldLevel {
		return &Signal{
			Type:       SignalBuy,
			Symbol:     s.config.Symbol,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 - s.config.StopLoss),
			TakeProfit: currentPrice * (1 + s.config.TakeProfit),
			OrderType:  "LIMIT",
			Side:       "BUY",
			Reason:     fmt.Sprintf("RSI oversold: %.2f", rsi),
			Timestamp:  time.Now(),
		}, nil
	}

	// Overbought - Sell signal
	if rsi > s.config.OverboughtLevel {
		return &Signal{
			Type:       SignalSell,
			Symbol:     s.config.Symbol,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 + s.config.StopLoss),
			TakeProfit: currentPrice * (1 - s.config.TakeProfit),
			OrderType:  "LIMIT",
			Side:       "SELL",
			Reason:     fmt.Sprintf("RSI overbought: %.2f", rsi),
			Timestamp:  time.Now(),
		}, nil
	}

	return &Signal{Type: SignalNone}, nil
}

// MovingAverageCrossoverConfig configures the MA crossover strategy
type MovingAverageCrossoverConfig struct {
	Symbol         string
	Interval       string
	FastPeriod     int
	SlowPeriod     int
	PositionSize   float64
	StopLoss       float64
	TakeProfit     float64
}

// MovingAverageCrossoverStrategy implements MA crossover strategy
type MovingAverageCrossoverStrategy struct {
	config      *MovingAverageCrossoverConfig
	lastCrossover string // "bullish" or "bearish"
}

func NewMovingAverageCrossoverStrategy(config *MovingAverageCrossoverConfig) *MovingAverageCrossoverStrategy {
	return &MovingAverageCrossoverStrategy{
		config: config,
	}
}

func (s *MovingAverageCrossoverStrategy) Name() string {
	return fmt.Sprintf("MACross-%s-%s", s.config.Symbol, s.config.Interval)
}

func (s *MovingAverageCrossoverStrategy) GetSymbol() string {
	return s.config.Symbol
}

func (s *MovingAverageCrossoverStrategy) GetInterval() string {
	return s.config.Interval
}

func (s *MovingAverageCrossoverStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < s.config.SlowPeriod {
		return &Signal{Type: SignalNone}, nil
	}

	// Calculate current MAs
	fastMA := calculateSMA(klines, s.config.FastPeriod)
	slowMA := calculateSMA(klines, s.config.SlowPeriod)

	// Calculate previous MAs
	prevKlines := klines[:len(klines)-1]
	prevFastMA := calculateSMA(prevKlines, s.config.FastPeriod)
	prevSlowMA := calculateSMA(prevKlines, s.config.SlowPeriod)

	// Detect bullish crossover (fast crosses above slow)
	if prevFastMA <= prevSlowMA && fastMA > slowMA && s.lastCrossover != "bullish" {
		s.lastCrossover = "bullish"
		return &Signal{
			Type:       SignalBuy,
			Symbol:     s.config.Symbol,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 - s.config.StopLoss),
			TakeProfit: currentPrice * (1 + s.config.TakeProfit),
			OrderType:  "LIMIT",
			Side:       "BUY",
			Reason:     fmt.Sprintf("Bullish MA crossover: Fast %.2f > Slow %.2f", fastMA, slowMA),
			Timestamp:  time.Now(),
		}, nil
	}

	// Detect bearish crossover (fast crosses below slow)
	if prevFastMA >= prevSlowMA && fastMA < slowMA && s.lastCrossover != "bearish" {
		s.lastCrossover = "bearish"
		return &Signal{
			Type:       SignalSell,
			Symbol:     s.config.Symbol,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 + s.config.StopLoss),
			TakeProfit: currentPrice * (1 - s.config.TakeProfit),
			OrderType:  "LIMIT",
			Side:       "SELL",
			Reason:     fmt.Sprintf("Bearish MA crossover: Fast %.2f < Slow %.2f", fastMA, slowMA),
			Timestamp:  time.Now(),
		}, nil
	}

	return &Signal{Type: SignalNone}, nil
}

// VolumeSpikeConfig configures the volume spike strategy
type VolumeSpikeConfig struct {
	Symbol          string
	Interval        string
	VolumeMultiplier float64 // e.g., 2.0 means 2x average volume
	MinPriceChange   float64 // Minimum price change %
	LookbackPeriod   int
	PositionSize     float64
	StopLoss         float64
	TakeProfit       float64
}

// VolumeSpikeStrategy triggers on significant volume increases
type VolumeSpikeStrategy struct {
	config *VolumeSpikeConfig
}

func NewVolumeSpikeStrategy(config *VolumeSpikeConfig) *VolumeSpikeStrategy {
	return &VolumeSpikeStrategy{config: config}
}

func (s *VolumeSpikeStrategy) Name() string {
	return fmt.Sprintf("VolumeSpike-%s-%s", s.config.Symbol, s.config.Interval)
}

func (s *VolumeSpikeStrategy) GetSymbol() string {
	return s.config.Symbol
}

func (s *VolumeSpikeStrategy) GetInterval() string {
	return s.config.Interval
}

func (s *VolumeSpikeStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*Signal, error) {
	if len(klines) < s.config.LookbackPeriod+1 {
		return &Signal{Type: SignalNone}, nil
	}

	lastCandle := klines[len(klines)-2]
	
	// Calculate average volume
	avgVolume := calculateAverageVolume(klines[:len(klines)-1], s.config.LookbackPeriod)
	
	// Check for volume spike
	if lastCandle.Volume < avgVolume*s.config.VolumeMultiplier {
		return &Signal{Type: SignalNone}, nil
	}

	// Calculate price change
	priceChange := ((lastCandle.Close - lastCandle.Open) / lastCandle.Open) * 100

	// Bullish volume spike
	if priceChange > s.config.MinPriceChange {
		return &Signal{
			Type:       SignalBuy,
			Symbol:     s.config.Symbol,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 - s.config.StopLoss),
			TakeProfit: currentPrice * (1 + s.config.TakeProfit),
			OrderType:  "LIMIT",
			Side:       "BUY",
			Reason:     fmt.Sprintf("Bullish volume spike: %.2fx avg, price +%.2f%%", lastCandle.Volume/avgVolume, priceChange),
			Timestamp:  time.Now(),
		}, nil
	}

	// Bearish volume spike
	if priceChange < -s.config.MinPriceChange {
		return &Signal{
			Type:       SignalSell,
			Symbol:     s.config.Symbol,
			EntryPrice: currentPrice,
			StopLoss:   currentPrice * (1 + s.config.StopLoss),
			TakeProfit: currentPrice * (1 - s.config.TakeProfit),
			OrderType:  "LIMIT",
			Side:       "SELL",
			Reason:     fmt.Sprintf("Bearish volume spike: %.2fx avg, price %.2f%%", lastCandle.Volume/avgVolume, priceChange),
			Timestamp:  time.Now(),
		}, nil
	}

	return &Signal{Type: SignalNone}, nil
}

// Helper functions for technical indicators

func calculateRSI(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 50.0 // Neutral RSI
	}

	gains := 0.0
	losses := 0.0

	// Calculate initial average gain and loss
	for i := len(klines) - period; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	rsi := 100 - (100 / (1 + rs))

	return rsi
}

func calculateSMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	sum := 0.0
	startIdx := len(klines) - period
	
	for i := startIdx; i < len(klines); i++ {
		sum += klines[i].Close
	}

	return sum / float64(period)
}

func calculateAverageVolume(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		period = len(klines)
	}

	sum := 0.0
	startIdx := len(klines) - period
	
	for i := startIdx; i < len(klines); i++ {
		sum += klines[i].Volume
	}

	return sum / float64(period)
}
