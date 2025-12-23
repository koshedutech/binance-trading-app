package scalping

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/strategy"
	"fmt"
	"math"
	"sync"
	"time"
)

// ScalpingConfig holds configuration for scalping strategy
type ScalpingConfig struct {
	Enabled          bool    `json:"enabled"`
	Symbol           string  `json:"symbol"`
	Interval         string  `json:"interval"`           // 1s, 5s, 15s, 1m
	MinProfitPercent float64 `json:"min_profit_percent"` // 0.05-0.1%
	MaxLossPercent   float64 `json:"max_loss_percent"`   // 0.1%
	MaxHoldSeconds   int     `json:"max_hold_seconds"`   // 60-120 seconds
	PositionSize     float64 `json:"position_size"`      // Position size as % of balance
	MinVolume        float64 `json:"min_volume"`         // Minimum volume required
	MomentumPeriod   int     `json:"momentum_period"`    // Candles for momentum calc
	VolumeMultiplier float64 `json:"volume_multiplier"`  // Volume spike threshold
	UseMarketOrders  bool    `json:"use_market_orders"`  // Use market vs limit orders
}

// DefaultScalpingConfig returns default configuration
func DefaultScalpingConfig() *ScalpingConfig {
	return &ScalpingConfig{
		Enabled:          true,
		Symbol:           "BTCUSDT",
		Interval:         "1s",
		MinProfitPercent: 0.05,
		MaxLossPercent:   0.1,
		MaxHoldSeconds:   60,
		PositionSize:     10.0,
		MinVolume:        0,
		MomentumPeriod:   10,
		VolumeMultiplier: 1.5,
		UseMarketOrders:  true,
	}
}

// ScalpingOpportunity represents a detected scalping opportunity
type ScalpingOpportunity struct {
	Symbol           string    `json:"symbol"`
	Direction        string    `json:"direction"` // "long" or "short"
	EntryPrice       float64   `json:"entry_price"`
	TargetPrice      float64   `json:"target_price"`
	StopPrice        float64   `json:"stop_price"`
	Confidence       float64   `json:"confidence"`
	MomentumScore    float64   `json:"momentum_score"`
	VolumeRatio      float64   `json:"volume_ratio"`
	Reason           string    `json:"reason"`
	DetectedAt       time.Time `json:"detected_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// ScalpingStrategy implements ultra-fast scalping strategy
type ScalpingStrategy struct {
	config          *ScalpingConfig
	activePositions map[string]*ActiveScalpPosition
	opportunities   []*ScalpingOpportunity
	mu              sync.RWMutex
	lastSignalTime  time.Time
	minSignalGap    time.Duration
}

// ActiveScalpPosition tracks an active scalping position
type ActiveScalpPosition struct {
	Symbol     string    `json:"symbol"`
	EntryPrice float64   `json:"entry_price"`
	EntryTime  time.Time `json:"entry_time"`
	TargetExit float64   `json:"target_exit"`
	StopLoss   float64   `json:"stop_loss"`
	MaxHoldEnd time.Time `json:"max_hold_end"`
}

// NewScalpingStrategy creates a new scalping strategy
func NewScalpingStrategy(config *ScalpingConfig) *ScalpingStrategy {
	if config == nil {
		config = DefaultScalpingConfig()
	}
	return &ScalpingStrategy{
		config:          config,
		activePositions: make(map[string]*ActiveScalpPosition),
		opportunities:   make([]*ScalpingOpportunity, 0),
		minSignalGap:    time.Second * 2, // Minimum 2 seconds between signals
	}
}

// Name returns the strategy name
func (s *ScalpingStrategy) Name() string {
	return fmt.Sprintf("Scalping-%s-%s", s.config.Symbol, s.config.Interval)
}

// GetSymbol returns the symbol this strategy trades
func (s *ScalpingStrategy) GetSymbol() string {
	return s.config.Symbol
}

// GetInterval returns the candle interval
func (s *ScalpingStrategy) GetInterval() string {
	return s.config.Interval
}

// Evaluate checks for scalping opportunities
func (s *ScalpingStrategy) Evaluate(klines []binance.Kline, currentPrice float64) (*strategy.Signal, error) {
	if !s.config.Enabled || len(klines) < s.config.MomentumPeriod+2 {
		return &strategy.Signal{Type: strategy.SignalNone}, nil
	}

	// Rate limit signals
	if time.Since(s.lastSignalTime) < s.minSignalGap {
		return &strategy.Signal{Type: strategy.SignalNone}, nil
	}

	// Check if we should exit existing position
	s.mu.RLock()
	activePos, hasPosition := s.activePositions[s.config.Symbol]
	s.mu.RUnlock()

	if hasPosition {
		return s.evaluateExit(activePos, currentPrice)
	}

	// Look for entry opportunity
	return s.evaluateEntry(klines, currentPrice)
}

// evaluateEntry looks for scalping entry opportunities
func (s *ScalpingStrategy) evaluateEntry(klines []binance.Kline, currentPrice float64) (*strategy.Signal, error) {
	// Calculate momentum
	momentum := s.calculateMomentum(klines)

	// Calculate volume ratio
	volumeRatio := s.calculateVolumeRatio(klines)

	// Check for volume spike confirmation
	hasVolume := volumeRatio >= s.config.VolumeMultiplier

	// Detect micro-trend direction
	microTrend := s.detectMicroTrend(klines)

	// Calculate confidence based on signals
	confidence := s.calculateEntryConfidence(momentum, volumeRatio, microTrend)

	// Minimum confidence threshold for scalping
	if confidence < 0.6 {
		return &strategy.Signal{Type: strategy.SignalNone}, nil
	}

	// Determine direction and entry
	var signalType strategy.SignalType
	var side string
	var stopLoss, takeProfit float64
	var reason string

	if momentum > 0.3 && microTrend == "up" && hasVolume {
		// Long scalp opportunity
		signalType = strategy.SignalBuy
		side = "BUY"
		stopLoss = currentPrice * (1 - s.config.MaxLossPercent/100)
		takeProfit = currentPrice * (1 + s.config.MinProfitPercent/100)
		reason = fmt.Sprintf("Scalp LONG: momentum=%.2f, volume=%.2fx, trend=%s", momentum, volumeRatio, microTrend)
	} else if momentum < -0.3 && microTrend == "down" && hasVolume {
		// Short scalp opportunity (or close long)
		signalType = strategy.SignalSell
		side = "SELL"
		stopLoss = currentPrice * (1 + s.config.MaxLossPercent/100)
		takeProfit = currentPrice * (1 - s.config.MinProfitPercent/100)
		reason = fmt.Sprintf("Scalp SHORT: momentum=%.2f, volume=%.2fx, trend=%s", momentum, volumeRatio, microTrend)
	} else {
		return &strategy.Signal{Type: strategy.SignalNone}, nil
	}

	s.lastSignalTime = time.Now()

	// Record active position
	s.mu.Lock()
	s.activePositions[s.config.Symbol] = &ActiveScalpPosition{
		Symbol:     s.config.Symbol,
		EntryPrice: currentPrice,
		EntryTime:  time.Now(),
		TargetExit: takeProfit,
		StopLoss:   stopLoss,
		MaxHoldEnd: time.Now().Add(time.Duration(s.config.MaxHoldSeconds) * time.Second),
	}
	s.mu.Unlock()

	orderType := "LIMIT"
	if s.config.UseMarketOrders {
		orderType = "MARKET"
	}

	return &strategy.Signal{
		Type:       signalType,
		Symbol:     s.config.Symbol,
		EntryPrice: currentPrice,
		StopLoss:   stopLoss,
		TakeProfit: takeProfit,
		OrderType:  orderType,
		Side:       side,
		Reason:     reason,
		Timestamp:  time.Now(),
	}, nil
}

// evaluateExit checks if we should exit the position
func (s *ScalpingStrategy) evaluateExit(pos *ActiveScalpPosition, currentPrice float64) (*strategy.Signal, error) {
	now := time.Now()

	// Check time-based exit
	if now.After(pos.MaxHoldEnd) {
		s.clearPosition(pos.Symbol)
		return &strategy.Signal{
			Type:       strategy.SignalSell,
			Symbol:     pos.Symbol,
			EntryPrice: currentPrice,
			OrderType:  "MARKET",
			Side:       "SELL",
			Reason:     fmt.Sprintf("Time exit: held for %ds", s.config.MaxHoldSeconds),
			Timestamp:  now,
		}, nil
	}

	// Check stop loss
	if currentPrice <= pos.StopLoss {
		s.clearPosition(pos.Symbol)
		pnl := (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		return &strategy.Signal{
			Type:       strategy.SignalSell,
			Symbol:     pos.Symbol,
			EntryPrice: currentPrice,
			OrderType:  "MARKET",
			Side:       "SELL",
			Reason:     fmt.Sprintf("Stop loss hit: %.4f%% loss", pnl),
			Timestamp:  now,
		}, nil
	}

	// Check take profit
	if currentPrice >= pos.TargetExit {
		s.clearPosition(pos.Symbol)
		pnl := (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
		return &strategy.Signal{
			Type:       strategy.SignalSell,
			Symbol:     pos.Symbol,
			EntryPrice: currentPrice,
			OrderType:  "MARKET",
			Side:       "SELL",
			Reason:     fmt.Sprintf("Take profit hit: +%.4f%% profit", pnl),
			Timestamp:  now,
		}, nil
	}

	return &strategy.Signal{Type: strategy.SignalNone}, nil
}

// clearPosition removes an active position
func (s *ScalpingStrategy) clearPosition(symbol string) {
	s.mu.Lock()
	delete(s.activePositions, symbol)
	s.mu.Unlock()
}

// calculateMomentum calculates short-term momentum
func (s *ScalpingStrategy) calculateMomentum(klines []binance.Kline) float64 {
	if len(klines) < s.config.MomentumPeriod {
		return 0
	}

	recent := klines[len(klines)-s.config.MomentumPeriod:]

	// Calculate price change momentum
	var upMoves, downMoves float64
	for i := 1; i < len(recent); i++ {
		change := recent[i].Close - recent[i-1].Close
		if change > 0 {
			upMoves += change
		} else {
			downMoves += -change
		}
	}

	total := upMoves + downMoves
	if total == 0 {
		return 0
	}

	// RSI-style momentum: -1 (strong down) to +1 (strong up)
	return (upMoves - downMoves) / total
}

// calculateVolumeRatio calculates current volume vs average
func (s *ScalpingStrategy) calculateVolumeRatio(klines []binance.Kline) float64 {
	if len(klines) < s.config.MomentumPeriod+1 {
		return 1.0
	}

	// Average volume from lookback period
	lookback := klines[len(klines)-s.config.MomentumPeriod-1 : len(klines)-1]
	var avgVolume float64
	for _, k := range lookback {
		avgVolume += k.Volume
	}
	avgVolume /= float64(len(lookback))

	if avgVolume == 0 {
		return 1.0
	}

	// Current candle volume
	currentVolume := klines[len(klines)-1].Volume
	return currentVolume / avgVolume
}

// detectMicroTrend detects very short-term trend direction
func (s *ScalpingStrategy) detectMicroTrend(klines []binance.Kline) string {
	if len(klines) < 5 {
		return "neutral"
	}

	recent := klines[len(klines)-5:]

	// Check last 5 candles
	upCount := 0
	downCount := 0

	for _, k := range recent {
		if k.Close > k.Open {
			upCount++
		} else if k.Close < k.Open {
			downCount++
		}
	}

	if upCount >= 3 {
		return "up"
	}
	if downCount >= 3 {
		return "down"
	}
	return "neutral"
}

// calculateEntryConfidence calculates confidence score for entry
func (s *ScalpingStrategy) calculateEntryConfidence(momentum, volumeRatio float64, trend string) float64 {
	confidence := 0.0

	// Momentum contribution (0-0.4)
	momentumScore := math.Min(math.Abs(momentum), 1.0) * 0.4
	confidence += momentumScore

	// Volume contribution (0-0.3)
	if volumeRatio >= s.config.VolumeMultiplier {
		volumeScore := math.Min((volumeRatio-1)/2, 1.0) * 0.3
		confidence += volumeScore
	}

	// Trend alignment (0-0.3)
	if (momentum > 0 && trend == "up") || (momentum < 0 && trend == "down") {
		confidence += 0.3
	} else if trend == "neutral" {
		confidence += 0.1
	}

	return math.Min(confidence, 1.0)
}

// GetActivePosition returns the active position for a symbol
func (s *ScalpingStrategy) GetActivePosition(symbol string) *ActiveScalpPosition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activePositions[symbol]
}

// HasActivePosition checks if there's an active position
func (s *ScalpingStrategy) HasActivePosition(symbol string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.activePositions[symbol]
	return exists
}

// GetConfig returns the strategy configuration
func (s *ScalpingStrategy) GetConfig() *ScalpingConfig {
	return s.config
}

// DetectOpportunity detects a scalping opportunity without generating a signal
func (s *ScalpingStrategy) DetectOpportunity(klines []binance.Kline, currentPrice float64) *ScalpingOpportunity {
	if !s.config.Enabled || len(klines) < s.config.MomentumPeriod+2 {
		return nil
	}

	momentum := s.calculateMomentum(klines)
	volumeRatio := s.calculateVolumeRatio(klines)
	microTrend := s.detectMicroTrend(klines)
	confidence := s.calculateEntryConfidence(momentum, volumeRatio, microTrend)

	if confidence < 0.5 {
		return nil
	}

	direction := "long"
	targetPrice := currentPrice * (1 + s.config.MinProfitPercent/100)
	stopPrice := currentPrice * (1 - s.config.MaxLossPercent/100)
	reason := "Momentum scalp opportunity"

	if momentum < 0 {
		direction = "short"
		targetPrice = currentPrice * (1 - s.config.MinProfitPercent/100)
		stopPrice = currentPrice * (1 + s.config.MaxLossPercent/100)
	}

	return &ScalpingOpportunity{
		Symbol:        s.config.Symbol,
		Direction:     direction,
		EntryPrice:    currentPrice,
		TargetPrice:   targetPrice,
		StopPrice:     stopPrice,
		Confidence:    confidence,
		MomentumScore: momentum,
		VolumeRatio:   volumeRatio,
		Reason:        reason,
		DetectedAt:    time.Now(),
		ExpiresAt:     time.Now().Add(time.Duration(s.config.MaxHoldSeconds) * time.Second),
	}
}
