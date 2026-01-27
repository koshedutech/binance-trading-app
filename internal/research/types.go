// Package research provides data structures and services for research infrastructure.
// This includes historical candle data storage and retrieval for backtesting and analysis.
//
// Epic 15: Research Infrastructure - Data, Features & Backtesting
// Story 15.1: Database Schema for Historical Candles
package research

import (
	"errors"
	"math"
	"time"
)

// Timeframe represents a candle interval.
type Timeframe string

const (
	Timeframe1m  Timeframe = "1m"
	Timeframe5m  Timeframe = "5m"
	Timeframe15m Timeframe = "15m"
	Timeframe30m Timeframe = "30m"
	Timeframe1h  Timeframe = "1h"
	Timeframe4h  Timeframe = "4h"
	Timeframe1d  Timeframe = "1d"
)

// String returns the string representation of the timeframe.
func (t Timeframe) String() string {
	return string(t)
}

// IsValid checks if the timeframe is a valid supported value.
func (t Timeframe) IsValid() bool {
	switch t {
	case Timeframe1m, Timeframe5m, Timeframe15m, Timeframe30m, Timeframe1h, Timeframe4h, Timeframe1d:
		return true
	default:
		return false
	}
}

// Minutes returns the duration of the timeframe in minutes.
func (t Timeframe) Minutes() int {
	switch t {
	case Timeframe1m:
		return 1
	case Timeframe5m:
		return 5
	case Timeframe15m:
		return 15
	case Timeframe30m:
		return 30
	case Timeframe1h:
		return 60
	case Timeframe4h:
		return 240
	case Timeframe1d:
		return 1440
	default:
		return 0
	}
}

// Duration returns the timeframe as a time.Duration.
func (t Timeframe) Duration() time.Duration {
	return time.Duration(t.Minutes()) * time.Minute
}

// ResearchCandle represents a single historical candle with all Binance kline fields.
// This maps directly to the research_candles database table.
type ResearchCandle struct {
	// Database fields
	ID int64 `json:"id" db:"id"`

	// Candle identification
	Symbol    string    `json:"symbol" db:"symbol"`       // e.g., "BTCUSDT"
	Timeframe Timeframe `json:"timeframe" db:"timeframe"` // e.g., "15m"
	OpenTime  time.Time `json:"open_time" db:"open_time"` // Candle open time (UTC)
	CloseTime time.Time `json:"close_time" db:"close_time"` // Candle close time (UTC)

	// OHLC price data
	Open  float64 `json:"open" db:"open"`   // Opening price
	High  float64 `json:"high" db:"high"`   // Highest price
	Low   float64 `json:"low" db:"low"`     // Lowest price
	Close float64 `json:"close" db:"close"` // Closing price

	// Volume data
	Volume      float64 `json:"volume" db:"volume"`             // Base asset volume
	QuoteVolume float64 `json:"quote_volume" db:"quote_volume"` // Quote asset volume (USDT)

	// Trade activity
	TradeCount int `json:"trade_count" db:"trade_count"` // Number of trades

	// Taker buy/sell data (for order flow analysis)
	TakerBuyVolume float64 `json:"taker_buy_volume" db:"taker_buy_volume"` // Taker buy base volume
	TakerBuyQuote  float64 `json:"taker_buy_quote" db:"taker_buy_quote"`   // Taker buy quote volume

	// Metadata
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// NewResearchCandle creates a new ResearchCandle with the given parameters.
// CloseTime is automatically calculated based on the timeframe duration.
func NewResearchCandle(symbol string, timeframe Timeframe, openTime time.Time) *ResearchCandle {
	return &ResearchCandle{
		Symbol:    symbol,
		Timeframe: timeframe,
		OpenTime:  openTime,
		CloseTime: openTime.Add(timeframe.Duration()),
		CreatedAt: time.Now(),
	}
}

// WithOHLC sets the OHLC price data.
func (c *ResearchCandle) WithOHLC(open, high, low, close float64) *ResearchCandle {
	c.Open = open
	c.High = high
	c.Low = low
	c.Close = close
	return c
}

// WithVolume sets the volume data.
func (c *ResearchCandle) WithVolume(volume, quoteVolume float64) *ResearchCandle {
	c.Volume = volume
	c.QuoteVolume = quoteVolume
	return c
}

// WithTradeCount sets the trade count.
func (c *ResearchCandle) WithTradeCount(count int) *ResearchCandle {
	c.TradeCount = count
	return c
}

// WithTakerData sets the taker buy/sell data.
func (c *ResearchCandle) WithTakerData(takerBuyVolume, takerBuyQuote float64) *ResearchCandle {
	c.TakerBuyVolume = takerBuyVolume
	c.TakerBuyQuote = takerBuyQuote
	return c
}

// IsBullish returns true if the candle closed higher than it opened.
func (c *ResearchCandle) IsBullish() bool {
	return c.Close > c.Open
}

// IsBearish returns true if the candle closed lower than it opened.
func (c *ResearchCandle) IsBearish() bool {
	return c.Close < c.Open
}

// IsDoji returns true if the candle has a very small body (open ~= close).
// Threshold is 10% of the candle range (body <= 10% of high-low range).
func (c *ResearchCandle) IsDoji() bool {
	body := math.Abs(c.Close - c.Open)
	candleRange := c.High - c.Low
	if candleRange == 0 {
		return true
	}
	threshold := candleRange * 0.10 // 10% of range
	return body <= threshold
}

// BodySize returns the absolute size of the candle body.
func (c *ResearchCandle) BodySize() float64 {
	return math.Abs(c.Close - c.Open)
}

// Range returns the full range of the candle (high - low).
func (c *ResearchCandle) Range() float64 {
	return c.High - c.Low
}

// UpperWick returns the size of the upper wick.
func (c *ResearchCandle) UpperWick() float64 {
	if c.IsBullish() {
		return c.High - c.Close
	}
	return c.High - c.Open
}

// LowerWick returns the size of the lower wick.
func (c *ResearchCandle) LowerWick() float64 {
	if c.IsBullish() {
		return c.Open - c.Low
	}
	return c.Close - c.Low
}

// TakerSellVolume calculates the taker sell volume (total volume - taker buy volume).
func (c *ResearchCandle) TakerSellVolume() float64 {
	return c.Volume - c.TakerBuyVolume
}

// TakerSellQuote calculates the taker sell quote volume.
func (c *ResearchCandle) TakerSellQuote() float64 {
	return c.QuoteVolume - c.TakerBuyQuote
}

// BuyVolumeRatio returns the ratio of taker buy volume to total volume.
// Returns 0 if total volume is zero.
func (c *ResearchCandle) BuyVolumeRatio() float64 {
	if c.Volume == 0 {
		return 0
	}
	return c.TakerBuyVolume / c.Volume
}

// Validate checks if the candle has valid data.
// Returns an error if any required field is invalid.
func (c *ResearchCandle) Validate() error {
	if c.Symbol == "" {
		return errors.New("symbol is required")
	}
	if !c.Timeframe.IsValid() {
		return errors.New("invalid timeframe")
	}
	if c.OpenTime.IsZero() {
		return errors.New("open_time is required")
	}
	if c.CloseTime.IsZero() {
		return errors.New("close_time is required")
	}
	if c.CloseTime.Before(c.OpenTime) {
		return errors.New("close_time must be after open_time")
	}
	if c.High < c.Low {
		return errors.New("high must be >= low")
	}
	if c.Open < c.Low || c.Open > c.High {
		return errors.New("open must be within high-low range")
	}
	if c.Close < c.Low || c.Close > c.High {
		return errors.New("close must be within high-low range")
	}
	if c.Volume < 0 {
		return errors.New("volume cannot be negative")
	}
	if c.TakerBuyVolume < 0 || c.TakerBuyVolume > c.Volume {
		return errors.New("taker_buy_volume must be between 0 and volume")
	}
	return nil
}

// CandleQuery represents query parameters for fetching candles.
type CandleQuery struct {
	Symbol     string    `json:"symbol"`
	Timeframe  Timeframe `json:"timeframe"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Limit      int       `json:"limit,omitempty"`      // Max candles to return (0 = no limit)
	Descending bool      `json:"descending,omitempty"` // If true, return newest first
}

// NewCandleQuery creates a new CandleQuery with required parameters.
func NewCandleQuery(symbol string, timeframe Timeframe, startTime, endTime time.Time) *CandleQuery {
	return &CandleQuery{
		Symbol:    symbol,
		Timeframe: timeframe,
		StartTime: startTime,
		EndTime:   endTime,
	}
}

// WithLimit sets the maximum number of candles to return.
func (q *CandleQuery) WithLimit(limit int) *CandleQuery {
	q.Limit = limit
	return q
}

// WithDescending sets the sort order to newest first.
func (q *CandleQuery) WithDescending() *CandleQuery {
	q.Descending = true
	return q
}

// Validate checks if the query parameters are valid.
// Returns an error if any required field is invalid.
func (q *CandleQuery) Validate() error {
	if q.Symbol == "" {
		return errors.New("symbol is required")
	}
	if !q.Timeframe.IsValid() {
		return errors.New("invalid timeframe")
	}
	if q.StartTime.IsZero() {
		return errors.New("start_time is required")
	}
	if q.EndTime.IsZero() {
		return errors.New("end_time is required")
	}
	if q.EndTime.Before(q.StartTime) {
		return errors.New("end_time must be after start_time")
	}
	if q.Limit < 0 {
		return errors.New("limit cannot be negative")
	}
	return nil
}

// CandleStats provides summary statistics for a set of candles.
type CandleStats struct {
	Symbol      string    `json:"symbol"`
	Timeframe   Timeframe `json:"timeframe"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Count       int       `json:"count"`
	HighPrice   float64   `json:"high_price"`   // Highest high in range
	LowPrice    float64   `json:"low_price"`    // Lowest low in range
	AvgVolume   float64   `json:"avg_volume"`   // Average volume
	TotalVolume float64   `json:"total_volume"` // Total volume
}

// BulkInsertResult contains the result of a bulk candle insert operation.
type BulkInsertResult struct {
	Inserted int           `json:"inserted"` // Number of new candles inserted
	Updated  int           `json:"updated"`  // Number of existing candles updated (upsert)
	Skipped  int           `json:"skipped"`  // Number of candles skipped (duplicates)
	Errors   int           `json:"errors"`   // Number of errors
	Duration time.Duration `json:"duration"` // Total operation duration
}

// AvailableData represents the available data range for a symbol/timeframe.
type AvailableData struct {
	Symbol      string    `json:"symbol"`
	Timeframe   Timeframe `json:"timeframe"`
	FirstCandle time.Time `json:"first_candle"` // Earliest candle timestamp
	LastCandle  time.Time `json:"last_candle"`  // Latest candle timestamp
	CandleCount int64     `json:"candle_count"` // Total number of candles
	Gaps        []DataGap `json:"gaps,omitempty"` // Any gaps in the data
}

// DataGap represents a gap in the candle data.
type DataGap struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Missing   int       `json:"missing"` // Number of missing candles
}
