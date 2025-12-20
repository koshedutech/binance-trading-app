package ml

import (
	"binance-trading-bot/internal/binance"
	"math"
	"sync"
	"time"
)

// PredictionTimeframe represents prediction time horizons
type PredictionTimeframe string

const (
	Timeframe1s  PredictionTimeframe = "1s"
	Timeframe2s  PredictionTimeframe = "2s"
	Timeframe5s  PredictionTimeframe = "5s"
	Timeframe10s PredictionTimeframe = "10s"
	Timeframe30s PredictionTimeframe = "30s"
	Timeframe60s PredictionTimeframe = "60s"
)

// PriceFeatures holds extracted features for prediction
type PriceFeatures struct {
	// Price features
	Returns          []float64 // Recent returns
	Volatility       float64   // Rolling volatility
	PriceVelocity    float64   // Rate of price change
	PriceAcceleration float64  // Acceleration of price change
	MomentumScore    float64   // Combined momentum indicator

	// Technical indicators
	RSI              float64 // Relative Strength Index
	RSISlope         float64 // RSI direction
	MACD             float64 // MACD value
	MACDSignal       float64 // MACD signal line
	MACDHistogram    float64 // MACD histogram
	BollingerPosition float64 // Position within bands (-1 to 1)

	// Volume features
	VolumeRatio      float64 // Current vs average volume
	BuyPressure      float64 // Estimated buy pressure
	VolumeAcceleration float64 // Volume momentum

	// Trend features
	TrendStrength    float64 // Trend strength (-1 to 1)
	TrendConsistency float64 // How consistent the trend is
}

// PricePrediction holds the prediction result
type PricePrediction struct {
	Symbol          string              `json:"symbol"`
	Timeframe       PredictionTimeframe `json:"timeframe"`
	Direction       string              `json:"direction"` // "up", "down", "sideways"
	PredictedMove   float64             `json:"predicted_move"` // Percentage
	Confidence      float64             `json:"confidence"` // 0-1
	PredictedPrice  float64             `json:"predicted_price"`
	CurrentPrice    float64             `json:"current_price"`
	PredictionTime  time.Time           `json:"prediction_time"`
	ValidUntil      time.Time           `json:"valid_until"`
	Signals         map[string]float64  `json:"signals"` // Individual signal contributions
}

// Predictor implements ML-based price prediction
type Predictor struct {
	config     *PredictorConfig
	cache      map[string]*PricePrediction
	cacheMu    sync.RWMutex
	stats      *PredictionStats
}

// PredictorConfig holds predictor configuration
type PredictorConfig struct {
	MomentumWeight    float64 // Weight for momentum signals
	MeanReversionWeight float64 // Weight for mean reversion
	VolumeWeight      float64 // Weight for volume signals
	TrendWeight       float64 // Weight for trend following
	MinConfidence     float64 // Minimum confidence to return prediction
}

// PredictionStats tracks prediction accuracy
type PredictionStats struct {
	TotalPredictions int
	CorrectPredictions int
	AverageError     float64
	mu               sync.RWMutex
}

// DefaultPredictorConfig returns default config
func DefaultPredictorConfig() *PredictorConfig {
	return &PredictorConfig{
		MomentumWeight:    0.3,
		MeanReversionWeight: 0.2,
		VolumeWeight:      0.25,
		TrendWeight:       0.25,
		MinConfidence:     0.5,
	}
}

// NewPredictor creates a new ML predictor
func NewPredictor(config *PredictorConfig) *Predictor {
	if config == nil {
		config = DefaultPredictorConfig()
	}
	return &Predictor{
		config: config,
		cache:  make(map[string]*PricePrediction),
		stats:  &PredictionStats{},
	}
}

// Predict generates a price prediction for the given timeframe
func (p *Predictor) Predict(symbol string, klines []binance.Kline, currentPrice float64, tf PredictionTimeframe) (*PricePrediction, error) {
	if len(klines) < 30 {
		return nil, nil // Not enough data
	}

	// Extract features
	features := p.extractFeatures(klines, currentPrice)

	// Calculate individual signals
	signals := make(map[string]float64)

	// 1. Momentum signal (-1 to 1)
	momentumSignal := p.calculateMomentumSignal(features)
	signals["momentum"] = momentumSignal

	// 2. Mean reversion signal (-1 to 1)
	meanRevSignal := p.calculateMeanReversionSignal(features)
	signals["mean_reversion"] = meanRevSignal

	// 3. Volume signal (-1 to 1)
	volumeSignal := p.calculateVolumeSignal(features)
	signals["volume"] = volumeSignal

	// 4. Trend signal (-1 to 1)
	trendSignal := p.calculateTrendSignal(features)
	signals["trend"] = trendSignal

	// Combine signals with weights
	combinedSignal :=
		momentumSignal * p.config.MomentumWeight +
		meanRevSignal * p.config.MeanReversionWeight +
		volumeSignal * p.config.VolumeWeight +
		trendSignal * p.config.TrendWeight

	// Calculate confidence based on signal agreement
	confidence := p.calculateConfidence(signals)

	// Determine direction
	var direction string
	if combinedSignal > 0.1 {
		direction = "up"
	} else if combinedSignal < -0.1 {
		direction = "down"
	} else {
		direction = "sideways"
	}

	// Estimate move size based on volatility and signal strength
	predictedMove := combinedSignal * features.Volatility * 2

	// Cap the predicted move
	maxMove := 0.5 // 0.5% max prediction
	if predictedMove > maxMove {
		predictedMove = maxMove
	} else if predictedMove < -maxMove {
		predictedMove = -maxMove
	}

	// Calculate valid duration based on timeframe
	validDuration := p.getTimeframeDuration(tf)

	prediction := &PricePrediction{
		Symbol:          symbol,
		Timeframe:       tf,
		Direction:       direction,
		PredictedMove:   predictedMove,
		Confidence:      confidence,
		PredictedPrice:  currentPrice * (1 + predictedMove/100),
		CurrentPrice:    currentPrice,
		PredictionTime:  time.Now(),
		ValidUntil:      time.Now().Add(validDuration),
		Signals:         signals,
	}

	// Cache prediction
	p.cacheMu.Lock()
	cacheKey := symbol + "_" + string(tf)
	p.cache[cacheKey] = prediction
	p.cacheMu.Unlock()

	return prediction, nil
}

// extractFeatures extracts trading features from klines
func (p *Predictor) extractFeatures(klines []binance.Kline, currentPrice float64) *PriceFeatures {
	features := &PriceFeatures{}

	// Calculate returns
	returns := make([]float64, 0, len(klines)-1)
	for i := 1; i < len(klines); i++ {
		ret := (klines[i].Close - klines[i-1].Close) / klines[i-1].Close * 100
		returns = append(returns, ret)
	}
	features.Returns = returns

	// Volatility (standard deviation of returns)
	features.Volatility = calculateStdDev(returns)

	// Price velocity (average of last 5 returns)
	if len(returns) >= 5 {
		sum := 0.0
		for i := len(returns) - 5; i < len(returns); i++ {
			sum += returns[i]
		}
		features.PriceVelocity = sum / 5
	}

	// Price acceleration (change in velocity)
	if len(returns) >= 10 {
		vel1 := 0.0
		vel2 := 0.0
		for i := len(returns) - 5; i < len(returns); i++ {
			vel2 += returns[i]
		}
		for i := len(returns) - 10; i < len(returns)-5; i++ {
			vel1 += returns[i]
		}
		features.PriceAcceleration = (vel2 - vel1) / 5
	}

	// RSI
	features.RSI = calculateRSI(klines, 14)

	// RSI slope
	if len(klines) >= 17 {
		rsiPrev := calculateRSI(klines[:len(klines)-3], 14)
		features.RSISlope = features.RSI - rsiPrev
	}

	// MACD
	macd, signal, histogram := calculateMACD(klines, 12, 26, 9)
	features.MACD = macd
	features.MACDSignal = signal
	features.MACDHistogram = histogram

	// Bollinger position
	upper, middle, lower := calculateBollingerBands(klines, 20, 2.0)
	if upper != lower {
		features.BollingerPosition = (currentPrice - middle) / (upper - middle)
	}

	// Volume ratio
	avgVolume := calculateAverageVolume(klines, 20)
	if avgVolume > 0 {
		features.VolumeRatio = klines[len(klines)-1].Volume / avgVolume
	}

	// Buy pressure estimation (based on close position in candle)
	lastCandle := klines[len(klines)-1]
	candleRange := lastCandle.High - lastCandle.Low
	if candleRange > 0 {
		features.BuyPressure = (lastCandle.Close - lastCandle.Low) / candleRange
	}

	// Volume acceleration
	if len(klines) >= 10 {
		recentVol := 0.0
		prevVol := 0.0
		for i := len(klines) - 5; i < len(klines); i++ {
			recentVol += klines[i].Volume
		}
		for i := len(klines) - 10; i < len(klines)-5; i++ {
			prevVol += klines[i].Volume
		}
		if prevVol > 0 {
			features.VolumeAcceleration = (recentVol - prevVol) / prevVol
		}
	}

	// Trend strength using EMA
	ema20 := calculateEMA(klines, 20)
	ema50 := calculateEMA(klines, 50)
	if ema50 > 0 {
		features.TrendStrength = (ema20 - ema50) / ema50 * 100
	}

	// Trend consistency (how many recent candles follow the trend)
	bullishCount := 0
	for i := len(klines) - 10; i < len(klines); i++ {
		if klines[i].Close > klines[i].Open {
			bullishCount++
		}
	}
	features.TrendConsistency = float64(bullishCount-5) / 5 // -1 to 1

	return features
}

// calculateMomentumSignal calculates momentum-based signal
func (p *Predictor) calculateMomentumSignal(f *PriceFeatures) float64 {
	signal := 0.0

	// Price velocity contribution
	signal += clamp(f.PriceVelocity/0.5, -1, 1) * 0.4

	// Price acceleration contribution
	signal += clamp(f.PriceAcceleration/0.2, -1, 1) * 0.3

	// MACD histogram contribution
	signal += clamp(f.MACDHistogram/0.01, -1, 1) * 0.3

	return clamp(signal, -1, 1)
}

// calculateMeanReversionSignal calculates mean reversion signal
func (p *Predictor) calculateMeanReversionSignal(f *PriceFeatures) float64 {
	signal := 0.0

	// RSI overbought/oversold
	if f.RSI > 70 {
		signal -= (f.RSI - 70) / 30 // Bearish signal
	} else if f.RSI < 30 {
		signal += (30 - f.RSI) / 30 // Bullish signal
	}

	// Bollinger band position
	if f.BollingerPosition > 1 {
		signal -= (f.BollingerPosition - 1) * 0.5 // Overbought
	} else if f.BollingerPosition < -1 {
		signal += (-1 - f.BollingerPosition) * 0.5 // Oversold
	}

	return clamp(signal, -1, 1)
}

// calculateVolumeSignal calculates volume-based signal
func (p *Predictor) calculateVolumeSignal(f *PriceFeatures) float64 {
	signal := 0.0

	// High volume with buy pressure = bullish
	if f.VolumeRatio > 1.5 {
		signal += (f.BuyPressure - 0.5) * (f.VolumeRatio - 1) * 0.5
	}

	// Volume acceleration
	signal += clamp(f.VolumeAcceleration*0.5, -0.5, 0.5)

	return clamp(signal, -1, 1)
}

// calculateTrendSignal calculates trend following signal
func (p *Predictor) calculateTrendSignal(f *PriceFeatures) float64 {
	signal := 0.0

	// Trend strength
	signal += clamp(f.TrendStrength/2, -1, 1) * 0.6

	// Trend consistency
	signal += f.TrendConsistency * 0.4

	return clamp(signal, -1, 1)
}

// calculateConfidence calculates prediction confidence
func (p *Predictor) calculateConfidence(signals map[string]float64) float64 {
	// Count signals agreeing on direction
	positive := 0
	negative := 0

	for _, s := range signals {
		if s > 0.1 {
			positive++
		} else if s < -0.1 {
			negative++
		}
	}

	// Confidence based on signal agreement
	total := len(signals)
	maxAgree := max(positive, negative)

	baseConfidence := float64(maxAgree) / float64(total)

	// Boost confidence if all signals agree
	if maxAgree == total {
		baseConfidence = 0.9
	}

	// Calculate average signal strength
	avgStrength := 0.0
	for _, s := range signals {
		avgStrength += math.Abs(s)
	}
	avgStrength /= float64(total)

	// Combine agreement and strength
	confidence := baseConfidence * 0.6 + avgStrength * 0.4

	return clamp(confidence, 0, 1)
}

// getTimeframeDuration returns duration for timeframe
func (p *Predictor) getTimeframeDuration(tf PredictionTimeframe) time.Duration {
	switch tf {
	case Timeframe1s:
		return time.Second
	case Timeframe2s:
		return 2 * time.Second
	case Timeframe5s:
		return 5 * time.Second
	case Timeframe10s:
		return 10 * time.Second
	case Timeframe30s:
		return 30 * time.Second
	case Timeframe60s:
		return 60 * time.Second
	default:
		return 60 * time.Second
	}
}

// GetStats returns prediction statistics
func (p *Predictor) GetStats() map[string]interface{} {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()

	accuracy := 0.0
	if p.stats.TotalPredictions > 0 {
		accuracy = float64(p.stats.CorrectPredictions) / float64(p.stats.TotalPredictions)
	}

	return map[string]interface{}{
		"total_predictions":   p.stats.TotalPredictions,
		"correct_predictions": p.stats.CorrectPredictions,
		"accuracy":            accuracy,
		"average_error":       p.stats.AverageError,
	}
}

// RecordOutcome records actual outcome for learning
func (p *Predictor) RecordOutcome(symbol string, tf PredictionTimeframe, actualMove float64) {
	p.cacheMu.RLock()
	cacheKey := symbol + "_" + string(tf)
	prediction, exists := p.cache[cacheKey]
	p.cacheMu.RUnlock()

	if !exists || time.Now().After(prediction.ValidUntil) {
		return
	}

	p.stats.mu.Lock()
	defer p.stats.mu.Unlock()

	p.stats.TotalPredictions++

	// Check if prediction was correct
	predictedUp := prediction.PredictedMove > 0
	actualUp := actualMove > 0
	if (predictedUp && actualUp) || (!predictedUp && !actualUp) {
		p.stats.CorrectPredictions++
	}

	// Update average error
	error := math.Abs(prediction.PredictedMove - actualMove)
	p.stats.AverageError = (p.stats.AverageError*float64(p.stats.TotalPredictions-1) + error) / float64(p.stats.TotalPredictions)
}

// Helper functions

func calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))

	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

func calculateRSI(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 50
	}

	gains := 0.0
	losses := 0.0

	for i := len(klines) - period; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func calculateMACD(klines []binance.Kline, fast, slow, signal int) (float64, float64, float64) {
	if len(klines) < slow+signal {
		return 0, 0, 0
	}

	// Calculate EMAs
	prices := make([]float64, len(klines))
	for i, k := range klines {
		prices[i] = k.Close
	}

	fastEMA := calculateEMAFromPrices(prices, fast)
	slowEMA := calculateEMAFromPrices(prices, slow)

	macd := fastEMA - slowEMA

	// Signal line (EMA of MACD)
	// Simplified: just use recent MACD values
	signalLine := macd * 0.9 // Approximation

	histogram := macd - signalLine

	return macd, signalLine, histogram
}

func calculateBollingerBands(klines []binance.Kline, period int, stdDev float64) (float64, float64, float64) {
	if len(klines) < period {
		lastPrice := klines[len(klines)-1].Close
		return lastPrice, lastPrice, lastPrice
	}

	// Calculate SMA
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	middle := sum / float64(period)

	// Calculate standard deviation
	variance := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		variance += (klines[i].Close - middle) * (klines[i].Close - middle)
	}
	variance /= float64(period)
	sd := math.Sqrt(variance)

	upper := middle + stdDev*sd
	lower := middle - stdDev*sd

	return upper, middle, lower
}

func calculateAverageVolume(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		period = len(klines)
	}

	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Volume
	}
	return sum / float64(period)
}

func calculateEMA(klines []binance.Kline, period int) float64 {
	prices := make([]float64, len(klines))
	for i, k := range klines {
		prices[i] = k.Close
	}
	return calculateEMAFromPrices(prices, period)
}

func calculateEMAFromPrices(prices []float64, period int) float64 {
	if len(prices) < period {
		return prices[len(prices)-1]
	}

	multiplier := 2.0 / float64(period+1)

	// Start with SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += prices[i]
	}
	ema := sum / float64(period)

	// Calculate EMA
	for i := period; i < len(prices); i++ {
		ema = (prices[i]-ema)*multiplier + ema
	}

	return ema
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
