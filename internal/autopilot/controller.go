package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/ai/ml"
	"binance-trading-bot/internal/ai/sentiment"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/continuous"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/scalping"
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// AutopilotConfig holds autopilot configuration
type AutopilotConfig struct {
	Enabled              bool    `json:"enabled"`
	RiskLevel            string  `json:"risk_level"` // conservative, moderate, aggressive
	MaxDailyLoss         float64 `json:"max_daily_loss"`
	MaxPositionSize      float64 `json:"max_position_size"`       // % of balance
	MinConfidence        float64 `json:"min_confidence"`          // 0-1
	RequireMultiSignal   bool    `json:"require_multi_signal"`    // Require multiple confirming signals
	EnableScalping       bool    `json:"enable_scalping"`
	EnableBigCandle      bool    `json:"enable_big_candle"`
	EnableLLM            bool    `json:"enable_llm"`
	EnableML             bool    `json:"enable_ml"`
	EnableSentiment      bool    `json:"enable_sentiment"`
	DecisionIntervalSecs int     `json:"decision_interval_secs"`
	DryRun               bool    `json:"dry_run"` // Paper trading mode
}

// DefaultAutopilotConfig returns default moderate configuration
func DefaultAutopilotConfig() *AutopilotConfig {
	return &AutopilotConfig{
		Enabled:              false, // Off by default for safety
		RiskLevel:            "moderate",
		MaxDailyLoss:         5.0,
		MaxPositionSize:      10.0,
		MinConfidence:        0.65,
		RequireMultiSignal:   true,
		EnableScalping:       true,
		EnableBigCandle:      true,
		EnableLLM:            true,
		EnableML:             true,
		EnableSentiment:      true,
		DecisionIntervalSecs: 5,
		DryRun:               true, // Dry run by default
	}
}

// TradingDecision represents an autopilot trading decision
type TradingDecision struct {
	Symbol          string            `json:"symbol"`
	Action          string            `json:"action"` // buy, sell, hold
	Direction       string            `json:"direction"` // long, short
	EntryPrice      float64           `json:"entry_price"`
	StopLoss        float64           `json:"stop_loss"`
	TakeProfit      float64           `json:"take_profit"`
	PositionSize    float64           `json:"position_size"`
	Confidence      float64           `json:"confidence"`
	Signals         map[string]Signal `json:"signals"`
	Reasoning       string            `json:"reasoning"`
	Timestamp       time.Time         `json:"timestamp"`
	Approved        bool              `json:"approved"`
	RejectionReason string            `json:"rejection_reason,omitempty"`
	AIDecisionID    int64             `json:"ai_decision_id,omitempty"`
}

// Signal represents a signal from one source
type Signal struct {
	Source     string  `json:"source"` // ml, llm, sentiment, big_candle, scalping
	Direction  string  `json:"direction"` // long, short, neutral
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// Controller orchestrates all AI components for autonomous trading
type Controller struct {
	config          *AutopilotConfig
	client          binance.BinanceClient
	circuitBreaker  *circuit.CircuitBreaker
	mlPredictor     *ml.Predictor
	llmAnalyzer     *llm.Analyzer
	sentimentAnalyzer *sentiment.Analyzer
	bigCandleDetector *continuous.BigCandleDetector
	scalpingStrategy *scalping.ScalpingStrategy

	activePositions map[string]*Position
	decisions       []*TradingDecision
	stats           *AutopilotStats

	mu              sync.RWMutex
	stopChan        chan struct{}
	running         bool

	onDecision      func(*TradingDecision)
	onTrade         func(*Trade)
	repository      *database.Repository
	orderManager    *OrderManager
}

// Position tracks an active position
type Position struct {
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"` // long, short
	EntryPrice   float64   `json:"entry_price"`
	Size         float64   `json:"size"`
	StopLoss     float64   `json:"stop_loss"`
	TakeProfit   float64   `json:"take_profit"`
	OpenedAt     time.Time `json:"opened_at"`
	Source       string    `json:"source"` // Which signal triggered this
	AIDecisionID *int64    `json:"ai_decision_id,omitempty"`
}

// Trade represents an executed trade
type Trade struct {
	Symbol       string    `json:"symbol"`
	Side         string    `json:"side"`
	Price        float64   `json:"price"`
	Size         float64   `json:"size"`
	PnL          float64   `json:"pnl"`
	PnLPercent   float64   `json:"pnl_percent"`
	Source       string    `json:"source"`
	Timestamp    time.Time `json:"timestamp"`
	AIDecisionID *int64    `json:"ai_decision_id,omitempty"`
}

// AutopilotStats tracks performance statistics
type AutopilotStats struct {
	TotalDecisions   int       `json:"total_decisions"`
	ApprovedDecisions int      `json:"approved_decisions"`
	RejectedDecisions int      `json:"rejected_decisions"`
	TotalTrades      int       `json:"total_trades"`
	WinningTrades    int       `json:"winning_trades"`
	LosingTrades     int       `json:"losing_trades"`
	TotalPnL         float64   `json:"total_pnl"`
	DailyPnL         float64   `json:"daily_pnl"`
	WinRate          float64   `json:"win_rate"`
	StartedAt        time.Time `json:"started_at"`
	LastDecisionAt   time.Time `json:"last_decision_at"`
}

// NewController creates a new autopilot controller
func NewController(
	config *AutopilotConfig,
	client binance.BinanceClient,
	circuitBreaker *circuit.CircuitBreaker,
) *Controller {
	if config == nil {
		config = DefaultAutopilotConfig()
	}

	return &Controller{
		config:          config,
		client:          client,
		circuitBreaker:  circuitBreaker,
		activePositions: make(map[string]*Position),
		decisions:       make([]*TradingDecision, 0),
		stats: &AutopilotStats{
			StartedAt: time.Now(),
		},
		stopChan: make(chan struct{}),
	}
}

// SetMLPredictor sets the ML predictor
func (c *Controller) SetMLPredictor(predictor *ml.Predictor) {
	c.mlPredictor = predictor
}

// SetLLMAnalyzer sets the LLM analyzer
func (c *Controller) SetLLMAnalyzer(analyzer *llm.Analyzer) {
	c.llmAnalyzer = analyzer
}

// SetSentimentAnalyzer sets the sentiment analyzer
func (c *Controller) SetSentimentAnalyzer(analyzer *sentiment.Analyzer) {
	c.sentimentAnalyzer = analyzer
}

// SetBigCandleDetector sets the big candle detector
func (c *Controller) SetBigCandleDetector(detector *continuous.BigCandleDetector) {
	c.bigCandleDetector = detector
}

// SetScalpingStrategy sets the scalping strategy
func (c *Controller) SetScalpingStrategy(strategy *scalping.ScalpingStrategy) {
	c.scalpingStrategy = strategy
}

// SetOrderManager sets the order manager for auto TP/SL
func (c *Controller) SetOrderManager(om *OrderManager) {
	c.orderManager = om
}

// OnDecision sets the callback for decisions
func (c *Controller) OnDecision(handler func(*TradingDecision)) {
	c.onDecision = handler
}

// OnTrade sets the callback for executed trades
func (c *Controller) OnTrade(handler func(*Trade)) {
	c.onTrade = handler
}

// Start begins the autopilot controller
func (c *Controller) Start() error {
	if !c.config.Enabled {
		return fmt.Errorf("autopilot is disabled")
	}

	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return fmt.Errorf("autopilot already running")
	}
	c.running = true
	c.mu.Unlock()

	log.Printf("[Autopilot] Starting with risk level: %s, dry run: %v", c.config.RiskLevel, c.config.DryRun)

	go c.runLoop()

	return nil
}

// Stop stops the autopilot controller
func (c *Controller) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	close(c.stopChan)
	log.Printf("[Autopilot] Stopped")
}

// runLoop is the main autopilot decision loop
func (c *Controller) runLoop() {
	interval := time.Duration(c.config.DecisionIntervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	symbols := []string{"BTCUSDT", "ETHUSDT"} // Start with major pairs

	for {
		select {
		case <-ticker.C:
			for _, symbol := range symbols {
				c.evaluateSymbol(symbol)
			}
		case <-c.stopChan:
			return
		}
	}
}

// evaluateSymbol evaluates a symbol for trading opportunities
func (c *Controller) evaluateSymbol(symbol string) {
	// Check circuit breaker first
	canTrade, reason := c.circuitBreaker.CanTrade()
	if !canTrade {
		log.Printf("[Autopilot] Circuit breaker prevents trading: %s", reason)
		return
	}

	// Fetch market data
	klines, err := c.client.GetKlines(symbol, "1m", 100)
	if err != nil {
		log.Printf("[Autopilot] Failed to get klines for %s: %v", symbol, err)
		return
	}

	currentPrice, err := c.client.GetCurrentPrice(symbol)
	if err != nil {
		log.Printf("[Autopilot] Failed to get price for %s: %v", symbol, err)
		return
	}

	// Collect signals from all sources
	signals := c.collectSignals(symbol, klines, currentPrice)

	// Log collected signals
	if len(signals) > 0 {
		log.Printf("[Autopilot] %s @ $%.2f - Signals collected:", symbol, currentPrice)
		for source, sig := range signals {
			log.Printf("  - %s: %s (confidence: %.1f%%) - %s", source, sig.Direction, sig.Confidence*100, sig.Reason)
		}
	}

	// Make decision based on signals
	decision := c.makeDecision(symbol, currentPrice, signals)

	c.stats.TotalDecisions++
	c.stats.LastDecisionAt = time.Now()

	// Log decision reasoning
	log.Printf("[Autopilot] %s Decision: %s - %s (confidence: %.1f%%)",
		symbol, decision.Action, decision.Reasoning, decision.Confidence*100)

	// Save decision to database
	c.saveDecisionToDB(symbol, currentPrice, decision, signals)

	if decision.Action == "hold" {
		return
	}

	// Validate decision through risk checks
	decision = c.validateDecision(decision)

	// Store decision
	c.mu.Lock()
	c.decisions = append(c.decisions, decision)
	if len(c.decisions) > 1000 {
		c.decisions = c.decisions[100:] // Keep last 900
	}
	c.mu.Unlock()

	// Trigger callback
	if c.onDecision != nil {
		c.onDecision(decision)
	}

	// Execute if approved
	if decision.Approved {
		c.stats.ApprovedDecisions++
		c.executeDecision(decision)
	} else {
		c.stats.RejectedDecisions++
		log.Printf("[Autopilot] Decision rejected: %s", decision.RejectionReason)
	}
}

// collectSignals gathers signals from all AI components
func (c *Controller) collectSignals(symbol string, klines []binance.Kline, currentPrice float64) map[string]Signal {
	signals := make(map[string]Signal)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// ML Prediction
	if c.config.EnableML && c.mlPredictor != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prediction, err := c.mlPredictor.Predict(symbol, klines, currentPrice, ml.Timeframe60s)
			if err == nil && prediction != nil {
				mu.Lock()
				signals["ml"] = Signal{
					Source:     "ml",
					Direction:  prediction.Direction,
					Confidence: prediction.Confidence,
					Reason:     fmt.Sprintf("ML: %.2f%% move predicted", prediction.PredictedMove*100),
				}
				mu.Unlock()
			}
		}()
	}

	// LLM Analysis
	if c.config.EnableLLM && c.llmAnalyzer != nil && c.llmAnalyzer.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			analysis, err := c.llmAnalyzer.AnalyzeMarket(symbol, "1m", klines)
			if err == nil && analysis != nil {
				mu.Lock()
				signals["llm"] = Signal{
					Source:     "llm",
					Direction:  analysis.Direction,
					Confidence: analysis.Confidence,
					Reason:     analysis.Reasoning,
				}
				mu.Unlock()
			}
		}()
	}

	// Sentiment Analysis
	if c.config.EnableSentiment && c.sentimentAnalyzer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bias, confidence := c.sentimentAnalyzer.GetTradingBias()
			if bias != "neutral" {
				direction := "long"
				if bias == "bearish" {
					direction = "short"
				}
				mu.Lock()
				signals["sentiment"] = Signal{
					Source:     "sentiment",
					Direction:  direction,
					Confidence: confidence,
					Reason:     fmt.Sprintf("Market sentiment: %s", bias),
				}
				mu.Unlock()
			}
		}()
	}

	// Big Candle Detection
	if c.config.EnableBigCandle && c.bigCandleDetector != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			event := c.bigCandleDetector.Detect(symbol, "1m", klines)
			if event != nil && event.Confidence >= 0.5 {
				direction := "long"
				if event.Direction == "bearish" {
					direction = "short"
				}
				mu.Lock()
				signals["big_candle"] = Signal{
					Source:     "big_candle",
					Direction:  direction,
					Confidence: event.Confidence,
					Reason:     fmt.Sprintf("Big %s candle: %.1fx size", event.Direction, event.SizeMultiplier),
				}
				mu.Unlock()
			}
		}()
	}

	// Scalping
	if c.config.EnableScalping && c.scalpingStrategy != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opp := c.scalpingStrategy.DetectOpportunity(klines, currentPrice)
			if opp != nil && opp.Confidence >= 0.5 {
				mu.Lock()
				signals["scalping"] = Signal{
					Source:     "scalping",
					Direction:  opp.Direction,
					Confidence: opp.Confidence,
					Reason:     opp.Reason,
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return signals
}

// makeDecision decides whether to trade based on collected signals
func (c *Controller) makeDecision(symbol string, currentPrice float64, signals map[string]Signal) *TradingDecision {
	decision := &TradingDecision{
		Symbol:    symbol,
		Action:    "hold",
		Signals:   signals,
		Timestamp: time.Now(),
	}

	if len(signals) == 0 {
		decision.Reasoning = "No signals detected"
		return decision
	}

	// Count directional signals
	longSignals := 0
	shortSignals := 0
	totalConfidence := 0.0
	signalCount := 0

	for _, sig := range signals {
		if sig.Direction == "long" {
			longSignals++
		} else if sig.Direction == "short" {
			shortSignals++
		}
		totalConfidence += sig.Confidence
		signalCount++
	}

	avgConfidence := totalConfidence / float64(signalCount)

	// Check minimum confidence
	if avgConfidence < c.config.MinConfidence {
		decision.Reasoning = fmt.Sprintf("Low confidence: %.2f < %.2f", avgConfidence, c.config.MinConfidence)
		return decision
	}

	// Check if we require multiple confirming signals
	if c.config.RequireMultiSignal {
		if longSignals < 2 && shortSignals < 2 {
			decision.Reasoning = "Insufficient signal confluence"
			return decision
		}
	}

	// Determine minimum signals required for a trade
	minSignals := 1
	if c.config.RequireMultiSignal {
		minSignals = 2
	}

	// Determine direction
	if longSignals > shortSignals && longSignals >= minSignals {
		decision.Action = "buy"
		decision.Direction = "long"
		decision.Confidence = avgConfidence
		decision.EntryPrice = currentPrice
		decision.StopLoss = currentPrice * (1 - c.getStopLossPercent())
		decision.TakeProfit = currentPrice * (1 + c.getTakeProfitPercent())
		decision.PositionSize = c.calculatePositionSize(currentPrice)
		decision.Reasoning = fmt.Sprintf("%d/%d signals favor long", longSignals, signalCount)
	} else if shortSignals > longSignals && shortSignals >= minSignals {
		decision.Action = "sell"
		decision.Direction = "short"
		decision.Confidence = avgConfidence
		decision.EntryPrice = currentPrice
		decision.StopLoss = currentPrice * (1 + c.getStopLossPercent())
		decision.TakeProfit = currentPrice * (1 - c.getTakeProfitPercent())
		decision.PositionSize = c.calculatePositionSize(currentPrice)
		decision.Reasoning = fmt.Sprintf("%d/%d signals favor short", shortSignals, signalCount)
	} else {
		decision.Reasoning = "Mixed signals, no clear direction"
	}

	return decision
}

// validateDecision validates the decision against risk rules
func (c *Controller) validateDecision(decision *TradingDecision) *TradingDecision {
	// Check if already have position in this symbol
	c.mu.RLock()
	_, hasPosition := c.activePositions[decision.Symbol]
	c.mu.RUnlock()

	if hasPosition {
		decision.Approved = false
		decision.RejectionReason = "Already have position in this symbol"
		return decision
	}

	// Check daily loss limit
	if c.stats.DailyPnL <= -c.config.MaxDailyLoss {
		decision.Approved = false
		decision.RejectionReason = "Daily loss limit reached"
		return decision
	}

	// Check sentiment for extreme conditions
	if c.sentimentAnalyzer != nil {
		avoid, reason := c.sentimentAnalyzer.ShouldAvoidTrading()
		if avoid {
			decision.Approved = false
			decision.RejectionReason = reason
			return decision
		}

		// NEW: Check if trading against strong sentiment
		// Don't go long when sentiment is strongly bearish
		// Don't go short when sentiment is strongly bullish
		bias, confidence := c.sentimentAnalyzer.GetTradingBias()
		strongSentimentThreshold := 0.6 // 60% confidence threshold

		if confidence >= strongSentimentThreshold {
			if decision.Direction == "long" && bias == "bearish" {
				decision.Approved = false
				decision.RejectionReason = fmt.Sprintf("Trading against strong bearish sentiment (%.0f%%)", confidence*100)
				return decision
			}
			if decision.Direction == "short" && bias == "bullish" {
				decision.Approved = false
				decision.RejectionReason = fmt.Sprintf("Trading against strong bullish sentiment (%.0f%%)", confidence*100)
				return decision
			}
		}
	}

	// All checks passed
	decision.Approved = true
	return decision
}

// executeDecision executes an approved trading decision
func (c *Controller) executeDecision(decision *TradingDecision) {
	// Get AI decision ID for linking
	var aiDecisionID *int64
	if decision.AIDecisionID > 0 {
		aiDecisionID = &decision.AIDecisionID
	}

	if c.config.DryRun {
		side := "BUY"
		if decision.Action == "sell" {
			side = "SELL"
		}

		log.Printf("[Autopilot][DRY RUN] Would execute: %s %s @ %.8f, size: %.8f (AI Decision: %d)",
			decision.Action, decision.Symbol, decision.EntryPrice, decision.PositionSize, decision.AIDecisionID)

		// Save trade to database even in dry run mode
		var tradeID int64
		if c.repository != nil {
			ctx := context.Background()
			strategyName := "autopilot"
			trade := &database.Trade{
				Symbol:       decision.Symbol,
				Side:         side,
				EntryPrice:   decision.EntryPrice,
				Quantity:     decision.PositionSize,
				EntryTime:    time.Now(),
				StopLoss:     &decision.StopLoss,
				TakeProfit:   &decision.TakeProfit,
				StrategyName: &strategyName,
				Status:       "OPEN",
				AIDecisionID: aiDecisionID,
				TradeSource:  database.TradeSourceAI,
			}
			if err := c.repository.CreateTrade(ctx, trade); err != nil {
				log.Printf("[Autopilot] Failed to save dry run trade: %v", err)
			} else {
				tradeID = trade.ID
				log.Printf("[Autopilot][DRY RUN] Trade saved to database: ID=%d, %s %s @ %.2f", tradeID, side, decision.Symbol, decision.EntryPrice)
			}
		}

		// Simulate position tracking for dry run
		c.mu.Lock()
		c.activePositions[decision.Symbol] = &Position{
			Symbol:       decision.Symbol,
			Side:         decision.Direction,
			EntryPrice:   decision.EntryPrice,
			Size:         decision.PositionSize,
			StopLoss:     decision.StopLoss,
			TakeProfit:   decision.TakeProfit,
			OpenedAt:     time.Now(),
			Source:       "autopilot",
			AIDecisionID: aiDecisionID,
		}
		c.mu.Unlock()

		// Register with order manager for simulated TP/SL tracking
		if c.orderManager != nil {
			c.orderManager.RegisterPosition(tradeID, decision.Symbol, side, decision.EntryPrice, decision.PositionSize, aiDecisionID)
		}
		return
	}

	// Real execution
	side := "BUY"
	if decision.Action == "sell" {
		side = "SELL"
	}

	params := map[string]string{
		"symbol":   decision.Symbol,
		"side":     side,
		"type":     "MARKET",
		"quantity": fmt.Sprintf("%.8f", decision.PositionSize),
	}

	order, err := c.client.PlaceOrder(params)
	if err != nil {
		log.Printf("[Autopilot] Failed to place order: %v", err)
		return
	}

	log.Printf("[Autopilot] Order placed: %s %s @ %.8f, ID: %d (AI Decision: %d)",
		side, decision.Symbol, order.Price, order.OrderId, decision.AIDecisionID)

	// Track position
	c.mu.Lock()
	c.activePositions[decision.Symbol] = &Position{
		Symbol:       decision.Symbol,
		Side:         decision.Direction,
		EntryPrice:   order.Price,
		Size:         decision.PositionSize,
		StopLoss:     decision.StopLoss,
		TakeProfit:   decision.TakeProfit,
		OpenedAt:     time.Now(),
		Source:       "autopilot",
		AIDecisionID: aiDecisionID,
	}
	c.mu.Unlock()

	c.stats.TotalTrades++

	// Register with order manager for auto TP/SL and trailing stop
	if c.orderManager != nil {
		tradeID := int64(order.OrderId) // Use order ID as trade ID for now
		c.orderManager.RegisterPosition(tradeID, decision.Symbol, side, order.Price, decision.PositionSize, aiDecisionID)
	}

	if c.onTrade != nil {
		c.onTrade(&Trade{
			Symbol:       decision.Symbol,
			Side:         side,
			Price:        order.Price,
			Size:         decision.PositionSize,
			Source:       "autopilot",
			Timestamp:    time.Now(),
			AIDecisionID: aiDecisionID,
		})
	}
}

// getStopLossPercent returns stop loss % based on risk level
// Increased slightly to reduce noise stops in volatile crypto markets
func (c *Controller) getStopLossPercent() float64 {
	switch c.config.RiskLevel {
	case "conservative":
		return 0.0075 // 0.75% (was 0.5%)
	case "aggressive":
		return 0.025 // 2.5% (was 2%)
	default: // moderate
		return 0.015 // 1.5% (was 1%)
	}
}

// getTakeProfitPercent returns take profit % based on risk level
// Adjusted to maintain good risk/reward ratio
func (c *Controller) getTakeProfitPercent() float64 {
	switch c.config.RiskLevel {
	case "conservative":
		return 0.005 // 0.5% (was 0.3%)
	case "aggressive":
		return 0.02 // 2% (was 1.5%)
	default: // moderate
		return 0.01 // 1% (was 0.5%)
	}
}

// calculatePositionSize calculates position size
func (c *Controller) calculatePositionSize(price float64) float64 {
	// In real implementation, this would use actual balance
	// For now, return a fixed small amount for safety
	baseSize := 0.001 // Small base size

	switch c.config.RiskLevel {
	case "conservative":
		return baseSize * 0.5
	case "aggressive":
		return baseSize * 2
	default:
		return baseSize
	}
}

// GetStats returns current statistics
func (c *Controller) GetStats() *AutopilotStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.stats.TotalTrades > 0 {
		c.stats.WinRate = float64(c.stats.WinningTrades) / float64(c.stats.TotalTrades)
	}

	return c.stats
}

// GetRecentDecisions returns recent decisions
func (c *Controller) GetRecentDecisions(limit int) []*TradingDecision {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.decisions) <= limit {
		return c.decisions
	}
	return c.decisions[len(c.decisions)-limit:]
}

// GetActivePositions returns active positions
func (c *Controller) GetActivePositions() map[string]*Position {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*Position)
	for k, v := range c.activePositions {
		result[k] = v
	}
	return result
}

// IsRunning returns if autopilot is running
func (c *Controller) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// SetDryRun enables/disables dry run mode
func (c *Controller) SetDryRun(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.DryRun = enabled
}
