// Package indicators provides technical indicator calculations for research infrastructure.
// Story 15.4: Feature Calculator Engine
package indicators

import (
	"math"
)

// NaN is a constant for representing missing/invalid values
var NaN = math.NaN()

// IsNaN checks if a value is NaN
func IsNaN(v float64) bool {
	return math.IsNaN(v)
}

// SafeDiv performs safe division, returning NaN if divisor is zero
func SafeDiv(numerator, denominator float64) float64 {
	if denominator == 0 {
		return NaN
	}
	return numerator / denominator
}

// SafePercent calculates percentage change safely
// Returns NaN for division by zero, Inf results, or extreme values
func SafePercent(current, previous float64) float64 {
	if previous == 0 {
		return NaN
	}
	result := (current - previous) / previous * 100
	// Guard against overflow/Inf from extremely small previous values
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return NaN
	}
	return result
}

// Max returns the maximum of two float64 values
func Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Min returns the minimum of two float64 values
func Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Abs returns the absolute value of a float64
func Abs(v float64) float64 {
	return math.Abs(v)
}

// Sqrt returns the square root of a float64
func Sqrt(v float64) float64 {
	return math.Sqrt(v)
}

// CandleData represents the minimum data needed for indicator calculations.
// This is an interface to decouple indicators from specific candle implementations.
type CandleData interface {
	GetOpen() float64
	GetHigh() float64
	GetLow() float64
	GetClose() float64
	GetVolume() float64
	GetQuoteVolume() float64
	GetTakerBuyVolume() float64
	GetTradeCount() int
}

// SimpleCandleAdapter adapts ResearchCandle to CandleData interface.
// Used internally when we need to pass candles to indicator functions.
type SimpleCandleAdapter struct {
	Open           float64
	High           float64
	Low            float64
	Close          float64
	Volume         float64
	QuoteVolume    float64
	TakerBuyVolume float64
	TradeCount     int
}

func (s *SimpleCandleAdapter) GetOpen() float64           { return s.Open }
func (s *SimpleCandleAdapter) GetHigh() float64           { return s.High }
func (s *SimpleCandleAdapter) GetLow() float64            { return s.Low }
func (s *SimpleCandleAdapter) GetClose() float64          { return s.Close }
func (s *SimpleCandleAdapter) GetVolume() float64         { return s.Volume }
func (s *SimpleCandleAdapter) GetQuoteVolume() float64    { return s.QuoteVolume }
func (s *SimpleCandleAdapter) GetTakerBuyVolume() float64 { return s.TakerBuyVolume }
func (s *SimpleCandleAdapter) GetTradeCount() int         { return s.TradeCount }

// OHLCV is a simple struct for price/volume data used in calculations
type OHLCV struct {
	Open           float64
	High           float64
	Low            float64
	Close          float64
	Volume         float64
	QuoteVolume    float64
	TakerBuyVolume float64
	TradeCount     int
}

func (o *OHLCV) GetOpen() float64           { return o.Open }
func (o *OHLCV) GetHigh() float64           { return o.High }
func (o *OHLCV) GetLow() float64            { return o.Low }
func (o *OHLCV) GetClose() float64          { return o.Close }
func (o *OHLCV) GetVolume() float64         { return o.Volume }
func (o *OHLCV) GetQuoteVolume() float64    { return o.QuoteVolume }
func (o *OHLCV) GetTakerBuyVolume() float64 { return o.TakerBuyVolume }
func (o *OHLCV) GetTradeCount() int         { return o.TradeCount }
