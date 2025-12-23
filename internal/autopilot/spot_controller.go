package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/logging"
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// SpotControllerConfig holds configuration for Spot AI autopilot
type SpotControllerConfig struct {
	Enabled           bool    `json:"enabled"`
	MaxPositions      int     `json:"max_positions"`
	MaxUSDPerPosition float64 `json:"max_usd_per_position"`
	TotalMaxUSD       float64 `json:"total_max_usd"`
	DryRun            bool    `json:"dry_run"`
	RiskLevel         string  `json:"risk_level"` // conservative, moderate, aggressive

	// Take Profit / Stop Loss percentages
	TakeProfitPercent float64 `json:"take_profit_percent"`
	StopLossPercent   float64 `json:"stop_loss_percent"`

	// Minimum confidence to trade
	MinConfidence float64 `json:"min_confidence"`

	// Scan interval in seconds
	ScanInterval int `json:"scan_interval"`

	// Circuit breaker settings
	CircuitBreakerEnabled  bool    `json:"circuit_breaker_enabled"`
	CBMaxLossPerHour       float64 `json:"cb_max_loss_per_hour"`
	CBMaxDailyLoss         float64 `json:"cb_max_daily_loss"`
	CBMaxConsecutiveLosses int     `json:"cb_max_consecutive_losses"`
	CBCooldownMinutes      int     `json:"cb_cooldown_minutes"`
}

// DefaultSpotControllerConfig returns default configuration
func DefaultSpotControllerConfig() *SpotControllerConfig {
	return &SpotControllerConfig{
		Enabled:           false,
		MaxPositions:      5,
		MaxUSDPerPosition: 100,
		TotalMaxUSD:       500,
		DryRun:            true,
		RiskLevel:         "moderate",

		TakeProfitPercent: 3.0,
		StopLossPercent:   2.0,
		MinConfidence:     60.0,
		ScanInterval:      60,

		CircuitBreakerEnabled:  true,
		CBMaxLossPerHour:       5.0,  // 5% max loss per hour
		CBMaxDailyLoss:         10.0, // 10% max daily loss
		CBMaxConsecutiveLosses: 3,
		CBCooldownMinutes:      30,
	}
}

// SpotPosition represents an open spot position
type SpotPosition struct {
	Symbol               string    `json:"symbol"`
	EntryPrice           float64   `json:"entry_price"`
	Quantity             float64   `json:"quantity"`
	EntryTime            time.Time `json:"entry_time"`
	TakeProfit           float64   `json:"take_profit"`
	StopLoss             float64   `json:"stop_loss"`
	HighestPrice         float64   `json:"highest_price"`
	UnrealizedPnL        float64   `json:"unrealized_pnl"`
	UnrealizedPnLPercent float64   `json:"unrealized_pnl_percent"`
}

// SpotDecision represents an AI trading decision
type SpotDecision struct {
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"` // BUY, SELL, HOLD
	Confidence float64   `json:"confidence"`
	Reasoning  string    `json:"reasoning"`
	Timestamp  time.Time `json:"timestamp"`
	Executed   bool      `json:"executed"`
	Rejected   bool      `json:"rejected"`
	RejectReason string  `json:"reject_reason,omitempty"`
}

// SpotController manages Spot AI autopilot trading
type SpotController struct {
	mu sync.RWMutex

	config *SpotControllerConfig
	client binance.BinanceClient
	logger *logging.Logger

	// AI components
	llmAnalyzer *llm.Analyzer

	// Circuit breaker
	circuitBreaker *circuit.CircuitBreaker

	// Position tracking
	positions map[string]*SpotPosition

	// Trading stats
	dailyTrades     int
	dailyPnL        float64
	totalPnL        float64
	winningTrades   int
	totalTrades     int

	// Decision history
	recentDecisions []SpotDecision

	// Control
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewSpotController creates a new Spot AI controller
func NewSpotController(client binance.BinanceClient, logger *logging.Logger) *SpotController {
	config := DefaultSpotControllerConfig()

	// Load settings from SettingsManager
	sm := GetSettingsManager()
	settings := sm.GetCurrentSettings()

	config.Enabled = settings.SpotAutopilotEnabled
	config.DryRun = settings.SpotDryRunMode
	config.RiskLevel = settings.SpotRiskLevel
	if settings.SpotMaxPositions > 0 {
		config.MaxPositions = settings.SpotMaxPositions
	}
	if settings.SpotMaxUSDPerPosition > 0 {
		config.MaxUSDPerPosition = settings.SpotMaxUSDPerPosition
	}

	sc := &SpotController{
		config:          config,
		client:          client,
		logger:          logger,
		positions:       make(map[string]*SpotPosition),
		recentDecisions: make([]SpotDecision, 0),
		stopChan:        make(chan struct{}),
	}

	// Initialize circuit breaker if enabled
	if config.CircuitBreakerEnabled {
		sc.circuitBreaker = circuit.NewCircuitBreaker(&circuit.CircuitBreakerConfig{
			MaxLossPerHour:       config.CBMaxLossPerHour,
			MaxDailyLoss:         config.CBMaxDailyLoss,
			MaxConsecutiveLosses: config.CBMaxConsecutiveLosses,
			CooldownMinutes:      config.CBCooldownMinutes,
		})
	}

	return sc
}

// SetConfig updates the controller configuration
func (sc *SpotController) SetConfig(config *SpotControllerConfig) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config = config
}

// GetConfig returns current configuration
func (sc *SpotController) GetConfig() *SpotControllerConfig {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.config
}

// SetDryRun sets the dry run mode
func (sc *SpotController) SetDryRun(enabled bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config.DryRun = enabled
}

// SetRiskLevel sets the risk level
func (sc *SpotController) SetRiskLevel(level string) error {
	if level != "conservative" && level != "moderate" && level != "aggressive" {
		return fmt.Errorf("invalid risk level: %s", level)
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config.RiskLevel = level
	return nil
}

// GetRiskLevel returns current risk level
func (sc *SpotController) GetRiskLevel() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.config.RiskLevel
}

// SetMaxUSDPerPosition sets max USD per position
func (sc *SpotController) SetMaxUSDPerPosition(maxUSD float64) error {
	if maxUSD <= 0 {
		return fmt.Errorf("max USD must be positive")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config.MaxUSDPerPosition = maxUSD
	return nil
}

// GetMaxUSDPerPosition returns max USD per position
func (sc *SpotController) GetMaxUSDPerPosition() float64 {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.config.MaxUSDPerPosition
}

// SetMaxPositions sets max concurrent positions
func (sc *SpotController) SetMaxPositions(max int) error {
	if max <= 0 {
		return fmt.Errorf("max positions must be positive")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config.MaxPositions = max
	return nil
}

// GetMaxPositions returns max concurrent positions
func (sc *SpotController) GetMaxPositions() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.config.MaxPositions
}

// SetTPSLPercent sets take profit and stop loss percentages
func (sc *SpotController) SetTPSLPercent(tp, sl float64) error {
	if tp <= 0 || sl <= 0 {
		return fmt.Errorf("TP/SL must be positive")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config.TakeProfitPercent = tp
	sc.config.StopLossPercent = sl
	return nil
}

// GetTPSLPercent returns take profit and stop loss percentages
func (sc *SpotController) GetTPSLPercent() (float64, float64) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.config.TakeProfitPercent, sc.config.StopLossPercent
}

// SetMinConfidence sets minimum confidence threshold
func (sc *SpotController) SetMinConfidence(conf float64) error {
	if conf < 0 || conf > 100 {
		return fmt.Errorf("min confidence must be between 0 and 100")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config.MinConfidence = conf
	return nil
}

// GetMinConfidence returns minimum confidence threshold
func (sc *SpotController) GetMinConfidence() float64 {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.config.MinConfidence
}

// SetLLMAnalyzer sets the LLM analyzer for AI decisions
func (sc *SpotController) SetLLMAnalyzer(analyzer *llm.Analyzer) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.llmAnalyzer = analyzer
}

// IsRunning returns whether the controller is active
func (sc *SpotController) IsRunning() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.running
}

// Start begins the Spot autopilot trading loop
func (sc *SpotController) Start() error {
	sc.mu.Lock()
	if sc.running {
		sc.mu.Unlock()
		return fmt.Errorf("spot controller already running")
	}

	if sc.client == nil {
		sc.mu.Unlock()
		return fmt.Errorf("spot client not configured")
	}

	sc.running = true
	sc.stopChan = make(chan struct{})
	sc.mu.Unlock()

	sc.wg.Add(1)
	go sc.runMainLoop()

	log.Printf("[SPOT-AUTOPILOT] Started - DryRun: %v, Risk: %s", sc.config.DryRun, sc.config.RiskLevel)
	return nil
}

// Stop stops the Spot autopilot
func (sc *SpotController) Stop() error {
	sc.mu.Lock()
	if !sc.running {
		sc.mu.Unlock()
		return fmt.Errorf("spot controller not running")
	}
	sc.running = false
	close(sc.stopChan)
	sc.mu.Unlock()

	sc.wg.Wait()
	log.Printf("[SPOT-AUTOPILOT] Stopped")
	return nil
}

// runMainLoop is the main trading loop
func (sc *SpotController) runMainLoop() {
	defer sc.wg.Done()

	scanInterval := time.Duration(sc.config.ScanInterval) * time.Second
	if scanInterval < 10*time.Second {
		scanInterval = 60 * time.Second
	}

	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sc.stopChan:
			return
		case <-ticker.C:
			sc.scanAndTrade()
		}
	}
}

// scanAndTrade scans for opportunities and executes trades
func (sc *SpotController) scanAndTrade() {
	// Check circuit breaker
	if sc.circuitBreaker != nil {
		canTrade, _ := sc.circuitBreaker.CanTrade()
		if !canTrade {
			return
		}
	}

	sc.mu.RLock()
	config := sc.config
	positionCount := len(sc.positions)
	sc.mu.RUnlock()

	// Check if we can open more positions
	if positionCount >= config.MaxPositions {
		return
	}

	// Get market data for analysis
	tickers, err := sc.client.Get24hrTickers()
	if err != nil {
		sc.logger.Debug("Failed to get tickers for scan", "error", err)
		return
	}

	// Filter to USDT pairs with good volume
	var candidates []string
	for _, t := range tickers {
		if len(t.Symbol) > 4 && t.Symbol[len(t.Symbol)-4:] == "USDT" {
			if t.QuoteVolume > 10000000 { // > $10M volume
				candidates = append(candidates, t.Symbol)
			}
		}
	}

	// Limit candidates
	if len(candidates) > 20 {
		candidates = candidates[:20]
	}

	// Analyze each candidate
	for _, symbol := range candidates {
		sc.mu.RLock()
		_, hasPosition := sc.positions[symbol]
		currentPositions := len(sc.positions)
		sc.mu.RUnlock()

		if hasPosition || currentPositions >= config.MaxPositions {
			continue
		}

		decision := sc.analyzeSymbol(symbol)
		if decision != nil && decision.Action == "BUY" && decision.Confidence >= config.MinConfidence {
			sc.executeBuy(symbol, decision)
		}
	}

	// Monitor existing positions
	sc.monitorPositions()
}

// analyzeSymbol uses AI to analyze a symbol
func (sc *SpotController) analyzeSymbol(symbol string) *SpotDecision {
	if sc.llmAnalyzer == nil {
		return nil
	}

	// Get klines for analysis
	klines, err := sc.client.GetKlines(symbol, "1h", 24)
	if err != nil {
		return nil
	}

	if len(klines) < 10 {
		return nil
	}

	// Calculate simple indicators
	var closes []float64
	for _, k := range klines {
		closes = append(closes, k.Close)
	}

	// Simple momentum check
	recentAvg := (closes[len(closes)-1] + closes[len(closes)-2] + closes[len(closes)-3]) / 3
	olderAvg := (closes[len(closes)-10] + closes[len(closes)-11] + closes[len(closes)-12]) / 3

	momentum := (recentAvg - olderAvg) / olderAvg * 100

	// Create decision based on momentum
	decision := &SpotDecision{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	if momentum > 2.0 {
		decision.Action = "BUY"
		decision.Confidence = math.Min(80, 50+momentum*5)
		decision.Reasoning = fmt.Sprintf("Positive momentum: %.2f%%", momentum)
	} else if momentum < -2.0 {
		decision.Action = "SELL"
		decision.Confidence = math.Min(80, 50+math.Abs(momentum)*5)
		decision.Reasoning = fmt.Sprintf("Negative momentum: %.2f%%", momentum)
	} else {
		decision.Action = "HOLD"
		decision.Confidence = 30
		decision.Reasoning = "No clear momentum"
	}

	// Record decision
	sc.mu.Lock()
	sc.recentDecisions = append(sc.recentDecisions, *decision)
	if len(sc.recentDecisions) > 100 {
		sc.recentDecisions = sc.recentDecisions[len(sc.recentDecisions)-100:]
	}
	sc.mu.Unlock()

	return decision
}

// executeBuy executes a buy order
func (sc *SpotController) executeBuy(symbol string, decision *SpotDecision) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Double-check we don't have a position
	if _, exists := sc.positions[symbol]; exists {
		return
	}

	// Get current price
	price, err := sc.client.GetCurrentPrice(symbol)
	if err != nil {
		sc.logger.Error("Failed to get price for buy", "symbol", symbol, "error", err)
		return
	}

	// Calculate quantity based on position size
	quantity := sc.config.MaxUSDPerPosition / price

	// Round to appropriate precision
	quantity = math.Floor(quantity*1000) / 1000

	if quantity <= 0 {
		return
	}

	sc.logger.Info("Spot autopilot executing buy",
		"symbol", symbol,
		"price", price,
		"quantity", quantity,
		"confidence", decision.Confidence,
		"dry_run", sc.config.DryRun)

	if !sc.config.DryRun {
		// Place market buy order
		params := map[string]string{
			"symbol": symbol,
			"side":   "BUY",
			"type":   "MARKET",
			"quantity": fmt.Sprintf("%.8f", quantity),
		}

		_, err := sc.client.PlaceOrder(params)
		if err != nil {
			sc.logger.Error("Spot buy order failed", "symbol", symbol, "error", err)
			decision.Rejected = true
			decision.RejectReason = err.Error()
			return
		}
	}

	// Create position record
	position := &SpotPosition{
		Symbol:       symbol,
		EntryPrice:   price,
		Quantity:     quantity,
		EntryTime:    time.Now(),
		TakeProfit:   price * (1 + sc.config.TakeProfitPercent/100),
		StopLoss:     price * (1 - sc.config.StopLossPercent/100),
		HighestPrice: price,
	}

	sc.positions[symbol] = position
	sc.dailyTrades++
	sc.totalTrades++
	decision.Executed = true

	sc.logger.Info("Spot position opened",
		"symbol", symbol,
		"entry_price", price,
		"quantity", quantity,
		"tp", position.TakeProfit,
		"sl", position.StopLoss)
}

// monitorPositions checks existing positions for TP/SL
func (sc *SpotController) monitorPositions() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	for symbol, pos := range sc.positions {
		currentPrice, err := sc.client.GetCurrentPrice(symbol)
		if err != nil {
			continue
		}

		// Update highest price
		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}

		// Calculate PnL
		pnl := (currentPrice - pos.EntryPrice) * pos.Quantity
		pos.UnrealizedPnL = pnl
		pos.UnrealizedPnLPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100

		// Check take profit
		if currentPrice >= pos.TakeProfit {
			sc.closePosition(symbol, pos, currentPrice, "take_profit")
			continue
		}

		// Check stop loss
		if currentPrice <= pos.StopLoss {
			sc.closePosition(symbol, pos, currentPrice, "stop_loss")
			continue
		}
	}
}

// closePosition closes a position
func (sc *SpotController) closePosition(symbol string, pos *SpotPosition, currentPrice float64, reason string) {
	pnl := (currentPrice - pos.EntryPrice) * pos.Quantity
	pnlPercent := (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100

	sc.logger.Info("Spot closing position",
		"symbol", symbol,
		"reason", reason,
		"entry_price", pos.EntryPrice,
		"exit_price", currentPrice,
		"pnl", pnl,
		"pnl_percent", pnlPercent)

	if !sc.config.DryRun {
		// Place market sell order
		params := map[string]string{
			"symbol":   symbol,
			"side":     "SELL",
			"type":     "MARKET",
			"quantity": fmt.Sprintf("%.8f", pos.Quantity),
		}

		_, err := sc.client.PlaceOrder(params)
		if err != nil {
			sc.logger.Error("Spot sell order failed", "symbol", symbol, "error", err)
			return
		}
	}

	// Update stats
	sc.dailyPnL += pnl
	sc.totalPnL += pnl
	if pnl > 0 {
		sc.winningTrades++
	}

	// Record to circuit breaker
	if sc.circuitBreaker != nil {
		sc.circuitBreaker.RecordTrade(pnlPercent)
	}

	// Remove position
	delete(sc.positions, symbol)

	sc.logger.Info("Spot position closed",
		"symbol", symbol,
		"reason", reason,
		"pnl", pnl,
		"total_pnl", sc.totalPnL)
}

// GetStatus returns the controller status
func (sc *SpotController) GetStatus() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	positions := make([]map[string]interface{}, 0)
	for _, pos := range sc.positions {
		positions = append(positions, map[string]interface{}{
			"symbol":         pos.Symbol,
			"entry_price":    pos.EntryPrice,
			"quantity":       pos.Quantity,
			"take_profit":    pos.TakeProfit,
			"stop_loss":      pos.StopLoss,
			"unrealized_pnl": pos.UnrealizedPnL,
			"entry_time":     pos.EntryTime,
		})
	}

	var cbStatus string
	var cbCanTrade bool
	if sc.circuitBreaker != nil {
		cbStatus = string(sc.circuitBreaker.GetState())
		cbCanTrade, _ = sc.circuitBreaker.CanTrade()
	}

	return map[string]interface{}{
		"enabled":              sc.config.Enabled,
		"running":              sc.running,
		"dry_run":              sc.config.DryRun,
		"risk_level":           sc.config.RiskLevel,
		"max_positions":        sc.config.MaxPositions,
		"max_usd_per_position": sc.config.MaxUSDPerPosition,
		"position_count":       len(sc.positions),
		"positions":            positions,
		"daily_trades":         sc.dailyTrades,
		"daily_pnl":            sc.dailyPnL,
		"total_pnl":            sc.totalPnL,
		"winning_trades":       sc.winningTrades,
		"total_trades":         sc.totalTrades,
		"circuit_breaker": map[string]interface{}{
			"enabled":   sc.config.CircuitBreakerEnabled,
			"state":     cbStatus,
			"can_trade": cbCanTrade,
		},
	}
}

// GetRecentDecisions returns recent AI decisions (with optional limit)
func (sc *SpotController) GetRecentDecisions(limits ...int) []SpotDecision {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	limit := len(sc.recentDecisions)
	if len(limits) > 0 && limits[0] > 0 && limits[0] < limit {
		limit = limits[0]
	}

	// Return most recent first
	result := make([]SpotDecision, limit)
	for i := 0; i < limit; i++ {
		result[i] = sc.recentDecisions[len(sc.recentDecisions)-1-i]
	}

	return result
}

// GetCircuitBreaker returns the circuit breaker for external status checks
func (sc *SpotController) GetCircuitBreaker() *circuit.CircuitBreaker {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.circuitBreaker
}

// GetProfitStats returns profit statistics
func (sc *SpotController) GetProfitStats() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	winRate := 0.0
	if sc.totalTrades > 0 {
		winRate = float64(sc.winningTrades) / float64(sc.totalTrades) * 100
	}

	return map[string]interface{}{
		"daily_pnl":      sc.dailyPnL,
		"total_pnl":      sc.totalPnL,
		"daily_trades":   sc.dailyTrades,
		"total_trades":   sc.totalTrades,
		"winning_trades": sc.winningTrades,
		"win_rate":       winRate,
	}
}

// GetCircuitBreakerStatus returns circuit breaker status
func (sc *SpotController) GetCircuitBreakerStatus() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if sc.circuitBreaker == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	canTrade, reason := sc.circuitBreaker.CanTrade()
	return map[string]interface{}{
		"enabled":      sc.config.CircuitBreakerEnabled,
		"state":        string(sc.circuitBreaker.GetState()),
		"can_trade":    canTrade,
		"block_reason": reason,
		"config": map[string]interface{}{
			"max_loss_per_hour":       sc.config.CBMaxLossPerHour,
			"max_daily_loss":          sc.config.CBMaxDailyLoss,
			"max_consecutive_losses":  sc.config.CBMaxConsecutiveLosses,
			"cooldown_minutes":        sc.config.CBCooldownMinutes,
		},
	}
}

// ResetCircuitBreaker resets the circuit breaker
func (sc *SpotController) ResetCircuitBreaker() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.circuitBreaker != nil {
		sc.circuitBreaker.ForceReset()
	}
	return nil
}

// UpdateCircuitBreakerConfig updates circuit breaker configuration
func (sc *SpotController) UpdateCircuitBreakerConfig(config *circuit.CircuitBreakerConfig) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.config.CBMaxLossPerHour = config.MaxLossPerHour
	sc.config.CBMaxDailyLoss = config.MaxDailyLoss
	sc.config.CBMaxConsecutiveLosses = config.MaxConsecutiveLosses
	sc.config.CBCooldownMinutes = config.CooldownMinutes

	if sc.circuitBreaker != nil {
		sc.circuitBreaker.UpdateConfig(config)
	}
	return nil
}

// SetCircuitBreakerEnabled enables or disables the circuit breaker
func (sc *SpotController) SetCircuitBreakerEnabled(enabled bool) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.config.CircuitBreakerEnabled = enabled
	if sc.circuitBreaker != nil {
		sc.circuitBreaker.SetEnabled(enabled)
	}
	return nil
}

// SpotCoinPreferences holds coin preference settings
type SpotCoinPreferences struct {
	Blacklist    []string `json:"blacklist"`
	Whitelist    []string `json:"whitelist"`
	UseWhitelist bool     `json:"use_whitelist"`
}

// GetCoinPreferences returns coin preferences
func (sc *SpotController) GetCoinPreferences() *SpotCoinPreferences {
	sm := GetSettingsManager()
	settings := sm.GetCurrentSettings()

	return &SpotCoinPreferences{
		Blacklist:    settings.SpotCoinBlacklist,
		Whitelist:    settings.SpotCoinWhitelist,
		UseWhitelist: settings.SpotUseWhitelist,
	}
}

// SetCoinPreferences updates coin preferences (accepts 3 separate args for handler compatibility)
func (sc *SpotController) SetCoinPreferences(blacklist, whitelist []string, useWhitelist bool) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sm := GetSettingsManager()
	settings := sm.GetCurrentSettings()

	settings.SpotCoinBlacklist = blacklist
	settings.SpotCoinWhitelist = whitelist
	settings.SpotUseWhitelist = useWhitelist

	return sm.SaveSettings(settings)
}

// SetCoinBlacklist sets the coin blacklist
func (sc *SpotController) SetCoinBlacklist(coins []string) error {
	prefs := sc.GetCoinPreferences()
	return sc.SetCoinPreferences(coins, prefs.Whitelist, prefs.UseWhitelist)
}

// SetCoinWhitelist sets the coin whitelist
func (sc *SpotController) SetCoinWhitelist(coins []string, useWhitelist bool) error {
	prefs := sc.GetCoinPreferences()
	return sc.SetCoinPreferences(prefs.Blacklist, coins, useWhitelist)
}

// GetDecisionStats returns decision statistics
func (sc *SpotController) GetDecisionStats() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	buyCount := 0
	sellCount := 0
	holdCount := 0
	executedCount := 0
	rejectedCount := 0

	for _, d := range sc.recentDecisions {
		switch d.Action {
		case "BUY":
			buyCount++
		case "SELL":
			sellCount++
		case "HOLD":
			holdCount++
		}
		if d.Executed {
			executedCount++
		}
		if d.Rejected {
			rejectedCount++
		}
	}

	return map[string]interface{}{
		"total_decisions": len(sc.recentDecisions),
		"buy_signals":     buyCount,
		"sell_signals":    sellCount,
		"hold_signals":    holdCount,
		"executed":        executedCount,
		"rejected":        rejectedCount,
	}
}

// GetPositions returns all open positions
func (sc *SpotController) GetPositions() []*SpotPosition {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	positions := make([]*SpotPosition, 0, len(sc.positions))
	for _, pos := range sc.positions {
		positions = append(positions, pos)
	}
	return positions
}

// ClosePosition closes a specific position by symbol
func (sc *SpotController) ClosePosition(symbol string) error {
	sc.mu.Lock()
	pos, exists := sc.positions[symbol]
	sc.mu.Unlock()

	if !exists {
		return fmt.Errorf("no position found for symbol: %s", symbol)
	}

	currentPrice, err := sc.client.GetCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %v", err)
	}

	sc.mu.Lock()
	sc.closePosition(symbol, pos, currentPrice, "manual_close")
	sc.mu.Unlock()

	return nil
}

// CloseAllPositions closes all open positions
func (sc *SpotController) CloseAllPositions() error {
	sc.mu.RLock()
	symbols := make([]string, 0, len(sc.positions))
	for symbol := range sc.positions {
		symbols = append(symbols, symbol)
	}
	sc.mu.RUnlock()

	for _, symbol := range symbols {
		if err := sc.ClosePosition(symbol); err != nil {
			log.Printf("Error closing position %s: %v", symbol, err)
		}
	}

	return nil
}
