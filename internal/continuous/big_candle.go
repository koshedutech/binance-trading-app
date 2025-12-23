package continuous

import (
	"binance-trading-bot/internal/binance"
	"math"
	"sync"
	"time"
)

// BigCandleConfig holds configuration for big candle detection
type BigCandleConfig struct {
	Enabled            bool
	SizeMultiplier     float64 // 1.5x to 2x of average candle
	LookbackPeriod     int     // Candles to calculate average
	VolumeConfirmation bool    // Require volume spike
	ReactImmediately   bool    // Enter on detection
	MinVolumeRatio     float64 // Minimum volume ratio (e.g., 1.5)
}

// DefaultBigCandleConfig returns default configuration
func DefaultBigCandleConfig() *BigCandleConfig {
	return &BigCandleConfig{
		Enabled:            true,
		SizeMultiplier:     1.5,
		LookbackPeriod:     20,
		VolumeConfirmation: true,
		ReactImmediately:   true,
		MinVolumeRatio:     1.5,
	}
}

// BigCandleEvent represents a detected big candle
type BigCandleEvent struct {
	Symbol          string    `json:"symbol"`
	Timeframe       string    `json:"timeframe"`
	Direction       string    `json:"direction"` // "bullish" or "bearish"
	SizeMultiplier  float64   `json:"size_multiplier"` // How many times larger
	VolumeRatio     float64   `json:"volume_ratio"`
	OpenPrice       float64   `json:"open_price"`
	ClosePrice      float64   `json:"close_price"`
	HighPrice       float64   `json:"high_price"`
	LowPrice        float64   `json:"low_price"`
	CandleSize      float64   `json:"candle_size"` // Absolute body size
	AverageSize     float64   `json:"average_size"` // Average candle size
	VolumeConfirmed bool      `json:"volume_confirmed"`
	Timestamp       time.Time `json:"timestamp"`
	Confidence      float64   `json:"confidence"` // 0-1 confidence score
}

// BigCandleDetector detects large candle movements
type BigCandleDetector struct {
	config    *BigCandleConfig
	handlers  []func(*BigCandleEvent)
	lastEvent map[string]*BigCandleEvent
	mu        sync.RWMutex
}

// NewBigCandleDetector creates a new big candle detector
func NewBigCandleDetector(config *BigCandleConfig) *BigCandleDetector {
	if config == nil {
		config = DefaultBigCandleConfig()
	}
	return &BigCandleDetector{
		config:    config,
		handlers:  make([]func(*BigCandleEvent), 0),
		lastEvent: make(map[string]*BigCandleEvent),
	}
}

// OnBigCandle registers a handler for big candle events
func (d *BigCandleDetector) OnBigCandle(handler func(*BigCandleEvent)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, handler)
}

// Detect checks klines for a big candle
func (d *BigCandleDetector) Detect(symbol, timeframe string, klines []binance.Kline) *BigCandleEvent {
	if !d.config.Enabled || len(klines) < d.config.LookbackPeriod+1 {
		return nil
	}

	// Get the most recent complete candle (second to last)
	// The last candle might still be forming
	currentCandle := klines[len(klines)-1]

	// Calculate average candle body size from lookback period
	avgBodySize := d.calculateAverageBodySize(klines[:len(klines)-1])
	if avgBodySize == 0 {
		return nil
	}

	// Calculate current candle body size
	currentBodySize := math.Abs(currentCandle.Close - currentCandle.Open)

	// Check if candle is big enough
	sizeMultiplier := currentBodySize / avgBodySize
	if sizeMultiplier < d.config.SizeMultiplier {
		return nil
	}

	// Calculate volume ratio
	avgVolume := d.calculateAverageVolume(klines[:len(klines)-1])
	volumeRatio := 0.0
	if avgVolume > 0 {
		volumeRatio = currentCandle.Volume / avgVolume
	}

	// Check volume confirmation if required
	volumeConfirmed := true
	if d.config.VolumeConfirmation {
		volumeConfirmed = volumeRatio >= d.config.MinVolumeRatio
	}

	// Determine direction
	direction := "bearish"
	if currentCandle.Close > currentCandle.Open {
		direction = "bullish"
	}

	// Calculate confidence score
	confidence := d.calculateConfidence(sizeMultiplier, volumeRatio, volumeConfirmed)

	event := &BigCandleEvent{
		Symbol:          symbol,
		Timeframe:       timeframe,
		Direction:       direction,
		SizeMultiplier:  sizeMultiplier,
		VolumeRatio:     volumeRatio,
		OpenPrice:       currentCandle.Open,
		ClosePrice:      currentCandle.Close,
		HighPrice:       currentCandle.High,
		LowPrice:        currentCandle.Low,
		CandleSize:      currentBodySize,
		AverageSize:     avgBodySize,
		VolumeConfirmed: volumeConfirmed,
		Timestamp:       time.Now(),
		Confidence:      confidence,
	}

	// Check for duplicate event (same candle)
	d.mu.Lock()
	lastEvent, exists := d.lastEvent[symbol]
	if exists && lastEvent.OpenPrice == currentCandle.Open && lastEvent.ClosePrice == currentCandle.Close {
		d.mu.Unlock()
		return nil // Already detected this candle
	}
	d.lastEvent[symbol] = event
	d.mu.Unlock()

	// Trigger handlers
	for _, handler := range d.handlers {
		go handler(event)
	}

	return event
}

// DetectFromLatest checks only the latest candle (real-time)
func (d *BigCandleDetector) DetectFromLatest(symbol, timeframe string, klines []binance.Kline) *BigCandleEvent {
	if !d.config.Enabled || len(klines) < d.config.LookbackPeriod+1 {
		return nil
	}

	// Use the last candle (currently forming)
	currentCandle := klines[len(klines)-1]

	// Calculate metrics from previous candles
	avgBodySize := d.calculateAverageBodySize(klines[:len(klines)-1])
	if avgBodySize == 0 {
		return nil
	}

	// For a forming candle, use current range
	currentBodySize := math.Abs(currentCandle.Close - currentCandle.Open)
	currentRange := currentCandle.High - currentCandle.Low

	// Use the larger of body or half the range
	effectiveSize := math.Max(currentBodySize, currentRange*0.5)

	sizeMultiplier := effectiveSize / avgBodySize
	if sizeMultiplier < d.config.SizeMultiplier {
		return nil
	}

	// Calculate volume ratio
	avgVolume := d.calculateAverageVolume(klines[:len(klines)-1])
	volumeRatio := 0.0
	if avgVolume > 0 {
		volumeRatio = currentCandle.Volume / avgVolume
	}

	volumeConfirmed := !d.config.VolumeConfirmation || volumeRatio >= d.config.MinVolumeRatio

	// Determine direction from current price action
	direction := "bearish"
	if currentCandle.Close > currentCandle.Open {
		direction = "bullish"
	}

	confidence := d.calculateConfidence(sizeMultiplier, volumeRatio, volumeConfirmed)

	return &BigCandleEvent{
		Symbol:          symbol,
		Timeframe:       timeframe,
		Direction:       direction,
		SizeMultiplier:  sizeMultiplier,
		VolumeRatio:     volumeRatio,
		OpenPrice:       currentCandle.Open,
		ClosePrice:      currentCandle.Close,
		HighPrice:       currentCandle.High,
		LowPrice:        currentCandle.Low,
		CandleSize:      effectiveSize,
		AverageSize:     avgBodySize,
		VolumeConfirmed: volumeConfirmed,
		Timestamp:       time.Now(),
		Confidence:      confidence,
	}
}

// calculateAverageBodySize calculates average candle body size
func (d *BigCandleDetector) calculateAverageBodySize(klines []binance.Kline) float64 {
	if len(klines) == 0 {
		return 0
	}

	lookback := d.config.LookbackPeriod
	if len(klines) < lookback {
		lookback = len(klines)
	}

	sum := 0.0
	for i := len(klines) - lookback; i < len(klines); i++ {
		bodySize := math.Abs(klines[i].Close - klines[i].Open)
		sum += bodySize
	}

	return sum / float64(lookback)
}

// calculateAverageVolume calculates average volume
func (d *BigCandleDetector) calculateAverageVolume(klines []binance.Kline) float64 {
	if len(klines) == 0 {
		return 0
	}

	lookback := d.config.LookbackPeriod
	if len(klines) < lookback {
		lookback = len(klines)
	}

	sum := 0.0
	for i := len(klines) - lookback; i < len(klines); i++ {
		sum += klines[i].Volume
	}

	return sum / float64(lookback)
}

// calculateConfidence calculates confidence score for the detection
func (d *BigCandleDetector) calculateConfidence(sizeMultiplier, volumeRatio float64, volumeConfirmed bool) float64 {
	confidence := 0.0

	// Size contribution (0.4 weight)
	// 1.5x = 0.4, 2x = 0.6, 2.5x+ = 0.8
	sizeScore := math.Min((sizeMultiplier-1)/1.5, 1.0) * 0.4
	confidence += sizeScore

	// Volume contribution (0.3 weight)
	if volumeConfirmed {
		volumeScore := math.Min((volumeRatio-1)/1.5, 1.0) * 0.3
		confidence += volumeScore
	}

	// Confirmation bonus (0.3 weight)
	if volumeConfirmed && sizeMultiplier >= 2.0 {
		confidence += 0.3
	} else if volumeConfirmed {
		confidence += 0.15
	}

	return math.Min(confidence, 1.0)
}

// GetLastEvent returns the last detected event for a symbol
func (d *BigCandleDetector) GetLastEvent(symbol string) *BigCandleEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastEvent[symbol]
}

// GetSignalDirection suggests trading direction based on big candle
func (e *BigCandleEvent) GetSignalDirection() string {
	// For a big bullish candle, momentum suggests continuation
	// For a big bearish candle, momentum suggests continuation
	return e.Direction
}

// GetEntryPrice suggests entry price based on the candle
func (e *BigCandleEvent) GetEntryPrice() float64 {
	// Enter at close price
	return e.ClosePrice
}

// GetStopLoss suggests stop loss level
func (e *BigCandleEvent) GetStopLoss() float64 {
	if e.Direction == "bullish" {
		// Stop below the candle low
		return e.LowPrice * 0.999 // Small buffer
	}
	// Stop above the candle high
	return e.HighPrice * 1.001
}

// GetTakeProfit suggests take profit level
func (e *BigCandleEvent) GetTakeProfit(riskRewardRatio float64) float64 {
	stopDistance := math.Abs(e.ClosePrice - e.GetStopLoss())
	targetDistance := stopDistance * riskRewardRatio

	if e.Direction == "bullish" {
		return e.ClosePrice + targetDistance
	}
	return e.ClosePrice - targetDistance
}

// ShouldTrade determines if this event warrants a trade
func (e *BigCandleEvent) ShouldTrade(minConfidence float64) bool {
	return e.Confidence >= minConfidence && e.VolumeConfirmed
}
