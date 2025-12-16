package analysis

import (
	"fmt"
	"time"
	"binance-trading-bot/internal/binance"
)

// FVGType represents the type of Fair Value Gap
type FVGType string

const (
	BullishFVG FVGType = "bullish"
	BearishFVG FVGType = "bearish"
)

// FVG represents a Fair Value Gap in price action
type FVG struct {
	ID          string
	Symbol      string
	Timeframe   string
	Type        FVGType
	TopPrice    float64
	BottomPrice float64
	CreatedAt   time.Time
	CandleIndex int
	Filled      bool
	FilledAt    *time.Time
	FilledPrice *float64
}

// FVGDetector detects Fair Value Gaps in candlestick data
type FVGDetector struct {
	minGapPercent float64 // Minimum gap size as percentage
}

// NewFVGDetector creates a new FVG detector
func NewFVGDetector(minGapPercent float64) *FVGDetector {
	if minGapPercent <= 0 {
		minGapPercent = 0.1 // Default 0.1% minimum gap
	}
	return &FVGDetector{
		minGapPercent: minGapPercent,
	}
}

// DetectFVGs identifies all Fair Value Gaps in the given candles
func (fd *FVGDetector) DetectFVGs(symbol, timeframe string, candles []binance.Kline) []FVG {
	if len(candles) < 3 {
		return nil
	}

	var fvgs []FVG

	// Scan for FVGs (need 3 consecutive candles)
	for i := 0; i < len(candles)-2; i++ {
		c1 := candles[i]
		c2 := candles[i+1] // Middle candle (gap creator)
		c3 := candles[i+2]

		// Check for Bullish FVG
		// Condition: c1.High < c3.Low (gap between them)
		if c1.High < c3.Low {
			gapSize := ((c3.Low - c1.High) / c1.High) * 100

			if gapSize >= fd.minGapPercent {
				fvg := FVG{
					ID:          generateFVGID(symbol, timeframe, i),
					Symbol:      symbol,
					Timeframe:   timeframe,
					Type:        BullishFVG,
					TopPrice:    c3.Low,
					BottomPrice: c1.High,
					CreatedAt:   time.Unix(c2.CloseTime/1000, 0),
					CandleIndex: i,
					Filled:      false,
				}
				fvgs = append(fvgs, fvg)
			}
		}

		// Check for Bearish FVG
		// Condition: c1.Low > c3.High (gap between them)
		if c1.Low > c3.High {
			gapSize := ((c1.Low - c3.High) / c3.High) * 100

			if gapSize >= fd.minGapPercent {
				fvg := FVG{
					ID:          generateFVGID(symbol, timeframe, i),
					Symbol:      symbol,
					Timeframe:   timeframe,
					Type:        BearishFVG,
					TopPrice:    c1.Low,
					BottomPrice: c3.High,
					CreatedAt:   time.Unix(c2.CloseTime/1000, 0),
					CandleIndex: i,
					Filled:      false,
				}
				fvgs = append(fvgs, fvg)
			}
		}
	}

	return fvgs
}

// IsPriceInFVG checks if current price is within an FVG zone
func (fd *FVGDetector) IsPriceInFVG(price float64, fvg FVG) bool {
	return price >= fvg.BottomPrice && price <= fvg.TopPrice
}

// IsPriceNearFVG checks if price is within a certain percentage of FVG
func (fd *FVGDetector) IsPriceNearFVG(price float64, fvg FVG, proximityPercent float64) bool {
	if fd.IsPriceInFVG(price, fvg) {
		return true
	}

	// Calculate proximity threshold
	gapSize := fvg.TopPrice - fvg.BottomPrice
	threshold := gapSize * (proximityPercent / 100)

	// Check if price is near the FVG zone
	distanceToTop := abs(price - fvg.TopPrice)
	distanceToBottom := abs(price - fvg.BottomPrice)

	return distanceToTop <= threshold || distanceToBottom <= threshold
}

// UpdateFVGStatus checks if an FVG has been filled by price action
func (fd *FVGDetector) UpdateFVGStatus(fvg *FVG, candles []binance.Kline) {
	if fvg.Filled {
		return // Already filled
	}

	for _, candle := range candles {
		// Check if candle wicked into the FVG zone
		if fvg.Type == BullishFVG {
			// For bullish FVG, check if price came back down into the gap
			if candle.Low <= fvg.TopPrice && candle.Low >= fvg.BottomPrice {
				fvg.Filled = true
				now := time.Unix(candle.CloseTime/1000, 0)
				fvg.FilledAt = &now
				fillPrice := candle.Low
				fvg.FilledPrice = &fillPrice
				return
			}
		} else if fvg.Type == BearishFVG {
			// For bearish FVG, check if price came back up into the gap
			if candle.High >= fvg.BottomPrice && candle.High <= fvg.TopPrice {
				fvg.Filled = true
				now := time.Unix(candle.CloseTime/1000, 0)
				fvg.FilledAt = &now
				fillPrice := candle.High
				fvg.FilledPrice = &fillPrice
				return
			}
		}
	}
}

// GetUnfilledFVGs returns only FVGs that haven't been filled yet
func (fd *FVGDetector) GetUnfilledFVGs(fvgs []FVG) []FVG {
	var unfilled []FVG
	for _, fvg := range fvgs {
		if !fvg.Filled {
			unfilled = append(unfilled, fvg)
		}
	}
	return unfilled
}

// Helper functions

func generateFVGID(symbol, timeframe string, index int) string {
	return fmt.Sprintf("%s_%s_%d_%d", symbol, timeframe, index, time.Now().Unix())
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
