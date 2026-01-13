package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/ai/ml"
	"binance-trading-bot/internal/ai/sentiment"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/patterns"
	"binance-trading-bot/internal/strategy"
	"fmt"
	"sync"
	"time"
)

// SignalSource represents the origin of a trading signal
type SignalSource string

const (
	SourceMLPredictor    SignalSource = "ml_predictor"
	SourceLLMAnalyzer    SignalSource = "llm_analyzer"
	SourceSentiment      SignalSource = "sentiment"
	SourcePatternScanner SignalSource = "pattern_scanner"
	SourceTechnical      SignalSource = "technical"
	// NOTE: SourceMultiTimeframe removed - MTF logic now handled by TrendFilterValidator as blocking filter
)

// EnhancedSignal represents a signal from any source with rich metadata
type EnhancedSignal struct {
	Source          SignalSource           `json:"source"`
	Symbol          string                 `json:"symbol"`
	Direction       string                 `json:"direction"` // "long", "short", "neutral"
	Confidence      float64                `json:"confidence"`
	Reasoning       string                 `json:"reasoning"`
	VolumeConfirmed bool                   `json:"volume_confirmed"`
	TrendAligned    bool                   `json:"trend_aligned"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// SignalWeight defines the weight for each signal source based on trading style
// NOTE: MultiTimeframe removed - MTF logic now handled by TrendFilterValidator as blocking filter
type SignalWeight struct {
	MLPredictor    float64
	LLMAnalyzer    float64
	Sentiment      float64
	PatternScanner float64
	Technical      float64
}

// DefaultSignalWeights returns weights for different trading styles
// NOTE: MultiTimeframe weights removed - MTF logic now handled by TrendFilterValidator as blocking filter
func GetSignalWeights(style TradingStyle) SignalWeight {
	switch style {
	case StyleScalping:
		return SignalWeight{
			MLPredictor:    0.30, // ML is very important for quick decisions
			LLMAnalyzer:    0.15, // Less important for scalping (slow)
			Sentiment:      0.05, // Minimal importance
			PatternScanner: 0.25, // Important for entry timing
			Technical:      0.25, // RSI/MACD important for momentum
		}
	case StyleSwing:
		return SignalWeight{
			MLPredictor:    0.25,
			LLMAnalyzer:    0.25, // LLM important for swing analysis
			Sentiment:      0.10,
			PatternScanner: 0.20,
			Technical:      0.20,
		}
	case StylePosition:
		return SignalWeight{
			MLPredictor:    0.15,
			LLMAnalyzer:    0.25,
			Sentiment:      0.20, // Sentiment matters for long-term
			PatternScanner: 0.15,
			Technical:      0.25,
		}
	default:
		// Balanced weights
		return SignalWeight{
			MLPredictor:    0.25,
			LLMAnalyzer:    0.20,
			Sentiment:      0.10,
			PatternScanner: 0.20,
			Technical:      0.25,
		}
	}
}

// llmCacheEntry holds cached LLM analysis with timestamp
type llmCacheEntry struct {
	analysis  *llm.MarketAnalysis
	timestamp time.Time
}

// SignalAggregator collects and aggregates signals from all sources
type SignalAggregator struct {
	// AI components
	mlPredictor       *ml.Predictor
	llmAnalyzer       *llm.Analyzer
	sentimentAnalyzer *sentiment.Analyzer

	// Technical analysis components
	patternDetector *patterns.PatternDetector

	// Binance client for multi-timeframe data
	futuresClient binance.FuturesClient

	// LLM analysis cache (symbol -> cached analysis)
	llmCache      map[string]*llmCacheEntry
	llmCacheTTL   time.Duration

	logger *logging.Logger
	mu     sync.RWMutex
}

// NewSignalAggregator creates a new signal aggregator
func NewSignalAggregator(
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
) *SignalAggregator {
	return &SignalAggregator{
		futuresClient:   futuresClient,
		patternDetector: patterns.NewPatternDetector(0.5), // 0.5% min body size
		llmCache:        make(map[string]*llmCacheEntry),
		llmCacheTTL:     5 * time.Minute, // Cache LLM analysis for 5 minutes
		logger:          logger,
	}
}

// GetCachedLLMAnalysis returns cached LLM analysis for a symbol if available and fresh
func (sa *SignalAggregator) GetCachedLLMAnalysis(symbol string) *llm.MarketAnalysis {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	entry, exists := sa.llmCache[symbol]
	if !exists {
		return nil
	}

	// Check if cache is still valid
	if time.Since(entry.timestamp) > sa.llmCacheTTL {
		return nil
	}

	return entry.analysis
}

// CacheLLMAnalysis stores LLM analysis in cache
func (sa *SignalAggregator) CacheLLMAnalysis(symbol string, analysis *llm.MarketAnalysis) {
	if analysis == nil {
		return
	}

	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.llmCache[symbol] = &llmCacheEntry{
		analysis:  analysis,
		timestamp: time.Now(),
	}
}

// SetMLPredictor sets the ML predictor
func (sa *SignalAggregator) SetMLPredictor(p *ml.Predictor) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.mlPredictor = p
}

// SetLLMAnalyzer sets the LLM analyzer
func (sa *SignalAggregator) SetLLMAnalyzer(a *llm.Analyzer) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.llmAnalyzer = a
}

// GetLLMAnalyzer returns the LLM analyzer for direct use
func (sa *SignalAggregator) GetLLMAnalyzer() *llm.Analyzer {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.llmAnalyzer
}

// SetSentimentAnalyzer sets the sentiment analyzer
func (sa *SignalAggregator) SetSentimentAnalyzer(a *sentiment.Analyzer) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.sentimentAnalyzer = a
}

// CollectAllSignals collects signals from all available sources
func (sa *SignalAggregator) CollectAllSignals(
	symbol string,
	currentPrice float64,
	klines []binance.Kline,
	style TradingStyle,
) ([]EnhancedSignal, *llm.MarketAnalysis) {
	var signals []EnhancedSignal
	var llmAnalysis *llm.MarketAnalysis
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 1. ML Predictor Signal
	sa.mu.RLock()
	mlPredictor := sa.mlPredictor
	sa.mu.RUnlock()

	if mlPredictor != nil && len(klines) >= 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signal := sa.collectMLSignal(symbol, currentPrice, klines)
			if signal != nil {
				mu.Lock()
				signals = append(signals, *signal)
				mu.Unlock()
			}
		}()
	} else {
		sa.logger.Debug("ML Predictor signal skipped",
			"symbol", symbol,
			"predictor_nil", mlPredictor == nil,
			"klines_count", len(klines),
			"required", 30)
	}

	// 2. LLM Analyzer Signal
	sa.mu.RLock()
	llmAnalyzer := sa.llmAnalyzer
	sa.mu.RUnlock()

	if llmAnalyzer != nil && llmAnalyzer.IsEnabled() && len(klines) >= 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signal, analysis := sa.collectLLMSignal(symbol, klines)
			if signal != nil {
				mu.Lock()
				signals = append(signals, *signal)
				llmAnalysis = analysis
				mu.Unlock()
				// Cache LLM analysis for Ginie to use
				sa.CacheLLMAnalysis(symbol, analysis)
			}
		}()
	} else {
		sa.logger.Debug("LLM Analyzer signal skipped",
			"symbol", symbol,
			"analyzer_nil", llmAnalyzer == nil,
			"enabled", llmAnalyzer != nil && llmAnalyzer.IsEnabled(),
			"klines_count", len(klines),
			"required", 20)
	}

	// 3. Sentiment Signal
	sa.mu.RLock()
	sentimentAnalyzer := sa.sentimentAnalyzer
	sa.mu.RUnlock()

	if sentimentAnalyzer != nil && sentimentAnalyzer.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signal := sa.collectSentimentSignal(symbol)
			if signal != nil {
				mu.Lock()
				signals = append(signals, *signal)
				mu.Unlock()
			}
		}()
	} else {
		sa.logger.Debug("Sentiment signal skipped",
			"symbol", symbol,
			"analyzer_nil", sentimentAnalyzer == nil,
			"enabled", sentimentAnalyzer != nil && sentimentAnalyzer.IsEnabled())
	}

	// 4. Pattern Scanner Signal (NEW)
	if len(klines) >= 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signal := sa.collectPatternSignal(symbol, klines)
			if signal != nil {
				mu.Lock()
				signals = append(signals, *signal)
				mu.Unlock()
			}
		}()
	} else {
		sa.logger.Debug("Pattern Scanner signal skipped",
			"symbol", symbol,
			"klines_count", len(klines),
			"required", 3)
	}

	// 5. Technical Indicators Signal (NEW)
	if len(klines) >= 50 { // Need enough data for 50 EMA
		wg.Add(1)
		go func() {
			defer wg.Done()
			signal := sa.collectTechnicalSignal(symbol, currentPrice, klines)
			if signal != nil {
				mu.Lock()
				signals = append(signals, *signal)
				mu.Unlock()
			}
		}()
	} else {
		sa.logger.Debug("Technical signal skipped",
			"symbol", symbol,
			"klines_count", len(klines),
			"required", 50)
	}

	// NOTE: Multi-Timeframe Signal removed from weighted signals
	// MTF logic is now handled by TrendFilterValidator as a blocking filter (Story 9.5)
	// This ensures ALL trading styles (including scalp) check higher timeframes before entry

	wg.Wait()

	// Log all collected signals
	sa.logSignals(symbol, signals)

	return signals, llmAnalysis
}

// collectMLSignal collects signal from ML predictor
func (sa *SignalAggregator) collectMLSignal(symbol string, currentPrice float64, klines []binance.Kline) *EnhancedSignal {
	sa.mu.RLock()
	predictor := sa.mlPredictor
	sa.mu.RUnlock()

	if predictor == nil {
		return nil
	}

	prediction, err := predictor.Predict(symbol, klines, currentPrice, ml.Timeframe60s)
	if err != nil || prediction == nil {
		sa.logger.Debug("ML prediction failed", "symbol", symbol, "error", err)
		return nil
	}

	direction := "neutral"
	if prediction.Direction == "up" && prediction.Confidence > 0.5 {
		direction = "long"
	} else if prediction.Direction == "down" && prediction.Confidence > 0.5 {
		direction = "short"
	}

	return &EnhancedSignal{
		Source:     SourceMLPredictor,
		Symbol:     symbol,
		Direction:  direction,
		Confidence: prediction.Confidence,
		Reasoning:  fmt.Sprintf("ML predicted: %s (%.1f%%)", prediction.Direction, prediction.PredictedMove*100),
		Metadata: map[string]interface{}{
			"predicted_move": prediction.PredictedMove,
			"model":          "ml_predictor",
		},
	}
}

// collectLLMSignal collects signal from LLM analyzer
func (sa *SignalAggregator) collectLLMSignal(symbol string, klines []binance.Kline) (*EnhancedSignal, *llm.MarketAnalysis) {
	sa.mu.RLock()
	analyzer := sa.llmAnalyzer
	sa.mu.RUnlock()

	if analyzer == nil {
		return nil, nil
	}

	analysis, err := analyzer.AnalyzeMarket(symbol, "1m", klines)
	if err != nil {
		sa.logger.Debug("LLM analysis error", "symbol", symbol, "error", err.Error())
		return nil, nil
	}
	if analysis == nil {
		return nil, nil
	}

	direction := "neutral"
	if analysis.Direction == "long" && analysis.Confidence >= 0.5 {
		direction = "long"
	} else if analysis.Direction == "short" && analysis.Confidence >= 0.5 {
		direction = "short"
	}

	signal := &EnhancedSignal{
		Source:     SourceLLMAnalyzer,
		Symbol:     symbol,
		Direction:  direction,
		Confidence: analysis.Confidence,
		Reasoning:  analysis.Reasoning,
		Metadata: map[string]interface{}{
			"key_levels":  analysis.KeyLevels,
			"risk_level":  analysis.RiskLevel,
			"stop_loss":   analysis.StopLoss,
			"take_profit": analysis.TakeProfit,
		},
	}

	return signal, analysis
}

// collectSentimentSignal collects signal from sentiment analyzer
func (sa *SignalAggregator) collectSentimentSignal(symbol string) *EnhancedSignal {
	sa.mu.RLock()
	analyzer := sa.sentimentAnalyzer
	sa.mu.RUnlock()

	if analyzer == nil {
		return nil
	}

	score := analyzer.GetSentiment()
	if score == nil {
		sa.logger.Debug("Sentiment score unavailable", "symbol", symbol)
		return nil
	}

	direction := "neutral"
	confidence := 0.5
	if score.Overall > 0.3 {
		direction = "long"
		confidence = score.Overall
	} else if score.Overall < -0.3 {
		direction = "short"
		confidence = -score.Overall
	}

	return &EnhancedSignal{
		Source:     SourceSentiment,
		Symbol:     symbol,
		Direction:  direction,
		Confidence: confidence,
		Reasoning:  fmt.Sprintf("Fear/Greed: %d (%s)", score.FearGreedIndex, score.FearGreedLabel),
		Metadata: map[string]interface{}{
			"fear_greed_index": score.FearGreedIndex,
			"fear_greed_label": score.FearGreedLabel,
			"overall":          score.Overall,
		},
	}
}

// collectPatternSignal collects signal from pattern scanner
func (sa *SignalAggregator) collectPatternSignal(symbol string, klines []binance.Kline) *EnhancedSignal {
	if sa.patternDetector == nil || len(klines) < 3 {
		return nil
	}

	// Detect patterns on 1m timeframe
	detectedPatterns := sa.patternDetector.DetectPatterns(symbol, "1m", klines)
	if len(detectedPatterns) == 0 {
		sa.logger.Debug("No patterns detected", "symbol", symbol)
		return nil
	}

	// Find the most confident recent pattern
	var bestPattern *patterns.DetectedPattern
	for i := range detectedPatterns {
		p := &detectedPatterns[i]
		// Only consider patterns from the last few candles
		if p.CandleIndex >= len(klines)-5 {
			if bestPattern == nil || p.Confidence > bestPattern.Confidence {
				bestPattern = p
			}
		}
	}

	if bestPattern == nil {
		return nil
	}

	direction := "neutral"
	if bestPattern.Direction == "bullish" {
		direction = "long"
	} else if bestPattern.Direction == "bearish" {
		direction = "short"
	}

	return &EnhancedSignal{
		Source:          SourcePatternScanner,
		Symbol:          symbol,
		Direction:       direction,
		Confidence:      bestPattern.Confidence,
		Reasoning:       fmt.Sprintf("Pattern: %s (%.0f%% confidence)", bestPattern.Type, bestPattern.Confidence*100),
		VolumeConfirmed: bestPattern.VolumeConfirmed,
		TrendAligned:    bestPattern.TrendAligned,
		Metadata: map[string]interface{}{
			"pattern_type":   string(bestPattern.Type),
			"candle_index":   bestPattern.CandleIndex,
			"volume_ratio":   bestPattern.VolumeRatio,
			"trend_strength": bestPattern.TrendStrength,
		},
	}
}

// collectTechnicalSignal collects signal from technical indicators
func (sa *SignalAggregator) collectTechnicalSignal(symbol string, currentPrice float64, klines []binance.Kline) *EnhancedSignal {
	if len(klines) < 50 {
		return nil
	}

	// Calculate indicators
	ema20 := strategy.CalculateEMA(klines, 20)
	ema50 := strategy.CalculateEMA(klines, 50)
	rsi := strategy.CalculateRSI(klines, 14)
	macd := strategy.CalculateMACD(klines, 12, 26, 9)

	// Score-based system
	bullishScore := 0
	bearishScore := 0
	reasons := []string{}

	// EMA Analysis
	if currentPrice > ema20 && ema20 > ema50 {
		bullishScore += 2
		reasons = append(reasons, "Price > EMA20 > EMA50")
	} else if currentPrice < ema20 && ema20 < ema50 {
		bearishScore += 2
		reasons = append(reasons, "Price < EMA20 < EMA50")
	} else if currentPrice > ema20 {
		bullishScore++
		reasons = append(reasons, "Price > EMA20")
	} else if currentPrice < ema20 {
		bearishScore++
		reasons = append(reasons, "Price < EMA20")
	}

	// RSI Analysis
	if rsi < 30 {
		bullishScore += 2
		reasons = append(reasons, fmt.Sprintf("RSI oversold (%.1f)", rsi))
	} else if rsi > 70 {
		bearishScore += 2
		reasons = append(reasons, fmt.Sprintf("RSI overbought (%.1f)", rsi))
	} else if rsi < 45 {
		bullishScore++
		reasons = append(reasons, fmt.Sprintf("RSI bullish zone (%.1f)", rsi))
	} else if rsi > 55 {
		bearishScore++
		reasons = append(reasons, fmt.Sprintf("RSI bearish zone (%.1f)", rsi))
	}

	// MACD Analysis
	if macd != nil {
		if macd.Histogram > 0 && macd.MACD > macd.Signal {
			bullishScore++
			reasons = append(reasons, "MACD bullish crossover")
		} else if macd.Histogram < 0 && macd.MACD < macd.Signal {
			bearishScore++
			reasons = append(reasons, "MACD bearish crossover")
		}
	}

	// Determine direction and confidence
	direction := "neutral"
	confidence := 0.5
	totalScore := bullishScore + bearishScore
	if totalScore > 0 {
		if bullishScore > bearishScore {
			direction = "long"
			confidence = 0.5 + (float64(bullishScore-bearishScore) / float64(totalScore+2) * 0.4)
		} else if bearishScore > bullishScore {
			direction = "short"
			confidence = 0.5 + (float64(bearishScore-bullishScore) / float64(totalScore+2) * 0.4)
		}
	}

	// Build reasoning string
	reasoningStr := "Technical: "
	if len(reasons) > 0 {
		reasoningStr += reasons[0]
		for i := 1; i < len(reasons) && i < 3; i++ {
			reasoningStr += ", " + reasons[i]
		}
	}

	return &EnhancedSignal{
		Source:     SourceTechnical,
		Symbol:     symbol,
		Direction:  direction,
		Confidence: confidence,
		Reasoning:  reasoningStr,
		Metadata: map[string]interface{}{
			"ema20":         ema20,
			"ema50":         ema50,
			"rsi":           rsi,
			"macd":          macd.MACD,
			"macd_signal":   macd.Signal,
			"macd_hist":     macd.Histogram,
			"bullish_score": bullishScore,
			"bearish_score": bearishScore,
		},
	}
}

// NOTE: collectMultiTimeframeSignal removed (Story 9.5)
// MTF logic is now handled by TrendFilterValidator in ginie_trend_filters.go
// This change ensures:
// 1. ALL trading styles (including scalp) check higher timeframes
// 2. MTF acts as a BLOCKING filter, not a weighted signal contributor
// 3. Higher TF disagreement blocks trades entirely instead of just reducing confidence

// logSignals logs all collected signals for debugging
func (sa *SignalAggregator) logSignals(symbol string, signals []EnhancedSignal) {
	if len(signals) == 0 {
		sa.logger.Warn("No signals collected",
			"symbol", symbol,
			"check", "Ensure ML/LLM/Sentiment analyzers are configured and enabled")
		return
	}

	// Count by direction
	longCount := 0
	shortCount := 0
	neutralCount := 0
	totalConfidence := 0.0

	for _, s := range signals {
		switch s.Direction {
		case "long":
			longCount++
		case "short":
			shortCount++
		default:
			neutralCount++
		}
		totalConfidence += s.Confidence
	}

	avgConfidence := totalConfidence / float64(len(signals))

	sa.logger.Info("Signal collection complete",
		"symbol", symbol,
		"total_signals", len(signals),
		"long_signals", longCount,
		"short_signals", shortCount,
		"neutral_signals", neutralCount,
		"avg_confidence", fmt.Sprintf("%.2f", avgConfidence))

	// Log each signal source
	for _, s := range signals {
		sa.logger.Debug("Signal detail",
			"symbol", symbol,
			"source", string(s.Source),
			"direction", s.Direction,
			"confidence", fmt.Sprintf("%.2f", s.Confidence),
			"reasoning", s.Reasoning)
	}
}

// AggregateDecision aggregates signals into a trading decision
func (sa *SignalAggregator) AggregateDecision(
	signals []EnhancedSignal,
	style TradingStyle,
	minConfidence float64,
	confluenceRequired int,
) (action string, confidence float64, approved bool, reason string) {
	if len(signals) == 0 {
		return "hold", 0, false, "No signals available"
	}

	weights := GetSignalWeights(style)

	// Calculate weighted scores
	longScore := 0.0
	shortScore := 0.0
	totalWeight := 0.0
	longCount := 0
	shortCount := 0

	for _, s := range signals {
		weight := sa.getWeight(s.Source, weights)
		totalWeight += weight

		switch s.Direction {
		case "long":
			longScore += s.Confidence * weight
			longCount++
		case "short":
			shortScore += s.Confidence * weight
			shortCount++
		}
	}

	// Normalize scores
	if totalWeight > 0 {
		longScore /= totalWeight
		shortScore /= totalWeight
	}

	// Determine action
	action = "hold"
	if longScore > shortScore && longCount > 0 {
		action = "open_long"
		confidence = longScore
	} else if shortScore > longScore && shortCount > 0 {
		action = "open_short"
		confidence = shortScore
	}

	// Check confluence requirement
	if confluenceRequired == 0 {
		// Any signal is enough
		if confidence >= minConfidence {
			approved = true
		} else {
			reason = fmt.Sprintf("Confidence %.2f below minimum %.2f", confidence, minConfidence)
		}
	} else {
		directionCount := longCount
		if action == "open_short" {
			directionCount = shortCount
		}
		if directionCount >= confluenceRequired && confidence >= minConfidence {
			approved = true
		} else if directionCount < confluenceRequired {
			reason = fmt.Sprintf("Insufficient confluence: %d signals, need %d", directionCount, confluenceRequired)
		} else {
			reason = fmt.Sprintf("Confidence %.2f below minimum %.2f", confidence, minConfidence)
		}
	}

	return action, confidence, approved, reason
}

// getWeight returns the weight for a signal source
func (sa *SignalAggregator) getWeight(source SignalSource, weights SignalWeight) float64 {
	switch source {
	case SourceMLPredictor:
		return weights.MLPredictor
	case SourceLLMAnalyzer:
		return weights.LLMAnalyzer
	case SourceSentiment:
		return weights.Sentiment
	case SourcePatternScanner:
		return weights.PatternScanner
	case SourceTechnical:
		return weights.Technical
	// NOTE: SourceMultiTimeframe case removed - MTF now handled by TrendFilterValidator
	default:
		return 0.1
	}
}

// ConvertToSignalBreakdown converts enhanced signals to the legacy format
func ConvertToSignalBreakdown(signals []EnhancedSignal) map[string]SignalContribution {
	breakdown := make(map[string]SignalContribution)
	for _, s := range signals {
		breakdown[string(s.Source)] = SignalContribution{
			Direction:  s.Direction,
			Confidence: s.Confidence,
			Reasoning:  s.Reasoning,
		}
	}
	return breakdown
}
