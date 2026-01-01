package llm

import (
	"binance-trading-bot/internal/binance"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// stripMarkdownCodeBlock removes markdown code block formatting from LLM responses
// Handles formats like: ```json\n{...}\n``` or ```\n{...}\n```
func stripMarkdownCodeBlock(response string) string {
	response = strings.TrimSpace(response)

	// Pattern to match ```json or ``` at start and ``` at end
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*\\n?(.*?)\\n?```$")
	if matches := re.FindStringSubmatch(response); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If no code block, return as-is
	return response
}

// AnalyzerConfig holds analyzer configuration
type AnalyzerConfig struct {
	Enabled           bool          `json:"enabled"`
	Provider          Provider      `json:"provider"`
	APIKey            string        `json:"api_key"`
	Model             string        `json:"model"`
	MaxTokens         int           `json:"max_tokens"`
	Temperature       float64       `json:"temperature"`
	MinConfidence     float64       `json:"min_confidence"`
	CacheDuration     time.Duration `json:"cache_duration"`
	RateLimitPerMin   int           `json:"rate_limit_per_min"`
	EnablePatterns    bool          `json:"enable_patterns"`
	EnableRiskCheck   bool          `json:"enable_risk_check"`
	EnableBigCandle   bool          `json:"enable_big_candle"`
}

// DefaultAnalyzerConfig returns default configuration
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		Enabled:           true,
		Provider:          ProviderClaude,
		Model:             "claude-sonnet-4-20250514",
		MaxTokens:         1024,
		Temperature:       0.3,
		MinConfidence:     0.6,
		CacheDuration:     5 * time.Minute,
		RateLimitPerMin:   10,
		EnablePatterns:    true,
		EnableRiskCheck:   true,
		EnableBigCandle:   true,
	}
}

// MarketAnalysis represents the LLM's market analysis
type MarketAnalysis struct {
	Direction   string   `json:"direction"`
	Confidence  float64  `json:"confidence"`
	EntryPrice  *float64 `json:"entry_price"`
	StopLoss    *float64 `json:"stop_loss"`
	TakeProfit  *float64 `json:"take_profit"`
	Reasoning   string   `json:"reasoning"`
	KeyLevels   struct {
		Support    []float64 `json:"support"`
		Resistance []float64 `json:"resistance"`
	} `json:"key_levels"`
	Timeframe string `json:"timeframe"`
	RiskLevel string `json:"risk_level"`
}

// PatternAnalysis represents pattern recognition results
type PatternAnalysis struct {
	PatternsFound []struct {
		PatternName          string   `json:"pattern_name"`
		PatternType          string   `json:"pattern_type"`
		Direction            string   `json:"direction"`
		CompletionPercentage float64  `json:"completion_percentage"`
		Confidence           float64  `json:"confidence"`
		TargetPrice          *float64 `json:"target_price"`
		InvalidationPrice    *float64 `json:"invalidation_price"`
	} `json:"patterns_found"`
	TrendAnalysis struct {
		PrimaryTrend  string  `json:"primary_trend"`
		TrendStrength float64 `json:"trend_strength"`
	} `json:"trend_analysis"`
	KeyLevels struct {
		Support    []float64 `json:"support"`
		Resistance []float64 `json:"resistance"`
	} `json:"key_levels"`
	OverallBias string `json:"overall_bias"`
}

// BigCandleAnalysis represents big candle analysis results
type BigCandleAnalysis struct {
	CandleType               string   `json:"candle_type"`
	FollowThroughProbability float64  `json:"follow_through_probability"`
	ExpectedMovement         string   `json:"expected_movement"`
	EntryRecommendation      string   `json:"entry_recommendation"`
	Confidence               float64  `json:"confidence"`
	Reasoning                string   `json:"reasoning"`
	CautionFlags             []string `json:"caution_flags"`
}

// RiskAssessment represents risk analysis results
type RiskAssessment struct {
	RiskScore                  float64  `json:"risk_score"`
	RiskLevel                  string   `json:"risk_level"`
	Concerns                   []string `json:"concerns"`
	Recommendations            []string `json:"recommendations"`
	PositionSizeRecommendation string   `json:"position_size_recommendation"`
	ShouldProceed              bool     `json:"should_proceed"`
	Reasoning                  string   `json:"reasoning"`
}

// ReversalAnalysis represents LLM's reversal pattern confirmation
type ReversalAnalysis struct {
	IsReversal        bool     `json:"is_reversal"`
	Confidence        float64  `json:"confidence"`
	ReversalType      string   `json:"reversal_type"` // exhaustion, capitulation, structural, false_signal
	EntryPrice        float64  `json:"entry_price"`
	StopLossPrice     float64  `json:"stop_loss_price"`
	TakeProfitPrice   float64  `json:"take_profit_price"`
	Reasoning         string   `json:"reasoning"`
	CautionFlags      []string `json:"caution_flags"`
	NearestSupport    float64  `json:"nearest_support"`
	NearestResistance float64  `json:"nearest_resistance"`
}

// AutoTradingDecision represents LLM's autonomous trading decisions
// When Auto Mode is enabled, the LLM decides everything: size, leverage, coins, averaging
type AutoTradingDecision struct {
	MarketAssessment MarketAssessmentData `json:"market_assessment"`
	TradingDecisions []TradingDecisionData `json:"trading_decisions"`
	PortfolioAllocation PortfolioAllocationData `json:"portfolio_allocation"`
	RiskManagement RiskManagementData `json:"risk_management"`
	WaitConditions WaitConditionsData `json:"wait_conditions"`
}

// MarketAssessmentData contains overall market assessment
type MarketAssessmentData struct {
	OverallSentiment string `json:"overall_sentiment"` // bullish, bearish, neutral, mixed
	VolatilityLevel  string `json:"volatility_level"`  // low, medium, high, extreme
	BestStrategy     string `json:"best_strategy"`     // trend_following, mean_reversion, breakout, scalping, wait
	MarketPhase      string `json:"market_phase"`      // accumulation, markup, distribution, markdown, ranging
}

// TradingDecisionData contains decision for a single symbol
type TradingDecisionData struct {
	Symbol              string    `json:"symbol"`
	Action              string    `json:"action"` // open_long, open_short, close, average_down, average_up, take_profit, hold, skip
	PositionSizeUSD     float64   `json:"position_size_usd"`
	Leverage            int       `json:"leverage"`
	Confidence          float64   `json:"confidence"`
	EntryZone           EntryZoneData `json:"entry_zone"`
	StopLossPercent     float64   `json:"stop_loss_percent"`
	TakeProfitPercent   float64   `json:"take_profit_percent"`
	Reasoning           string    `json:"reasoning"`
	Priority            int       `json:"priority"`
	HoldDuration        string    `json:"hold_duration"` // short, medium, long
	ShouldAverageIfDown bool      `json:"should_average_if_down"`
	MaxAverageCount     int       `json:"max_average_count"`
	ReentryAfterTP      bool      `json:"reentry_after_tp"`
}

// EntryZoneData contains min/max entry prices
type EntryZoneData struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// PortfolioAllocationData contains portfolio allocation guidance
type PortfolioAllocationData struct {
	TotalUSDToDeploy        float64 `json:"total_usd_to_deploy"`
	ReservePercent          float64 `json:"reserve_percent"`
	MaxSinglePositionPercent float64 `json:"max_single_position_percent"`
}

// RiskManagementData contains risk management guidance
type RiskManagementData struct {
	OverallRiskLevel     string  `json:"overall_risk_level"` // conservative, moderate, aggressive
	CorrelationWarning   string  `json:"correlation_warning"`
	MaxDrawdownTolerance float64 `json:"max_drawdown_tolerance"`
}

// WaitConditionsData contains conditions when to wait instead of trade
type WaitConditionsData struct {
	ShouldWait bool   `json:"should_wait"`
	WaitReason string `json:"wait_reason"`
	ResumeWhen string `json:"resume_when"`
}

// AutoModeConstraints contains the hard limits for auto mode trading
type AutoModeConstraints struct {
	MaxPositions      int
	MaxLeverage       int
	MaxPositionSizeUSD float64
	MaxTotalUSD       float64
	AllowAveraging    bool
	MaxAverages       int
	MinHoldMinutes    int
	QuickProfitMode   bool
	MinProfitForExit  float64
}

// ExistingPositionInfo contains info about an existing position for auto mode analysis
type ExistingPositionInfo struct {
	Symbol        string
	Side          string
	EntryPrice    float64
	Quantity      float64
	UnrealizedPnL float64
	PnLPercent    float64
	EntryCount    int
	HoldMinutes   int
}

// PositionSLTPAnalysis represents LLM analysis for position SL/TP management
type PositionSLTPAnalysis struct {
	RecommendedSL  float64 `json:"recommended_sl"`
	RecommendedTP  float64 `json:"recommended_tp"`
	SLReasoning    string  `json:"sl_reasoning"`
	TPReasoning    string  `json:"tp_reasoning"`
	Urgency        string  `json:"urgency"`         // immediate, normal, hold
	RiskAssessment string  `json:"risk_assessment"` // low, medium, high
	Action         string  `json:"action"`          // tighten_sl, widen_sl, move_to_breakeven, trail_stop, hold_current, close_now
	Confidence     float64 `json:"confidence"`
}

// PositionInfo holds current position details for LLM analysis
type PositionInfo struct {
	Symbol        string
	Side          string  // LONG or SHORT
	EntryPrice    float64
	CurrentPrice  float64
	Quantity      float64
	UnrealizedPnL float64
	PnLPercent    float64
	CurrentSL     float64
	CurrentTP     float64
	HoldDuration  string // e.g., "2h 15m"
	Mode          string // scalp, swing, position
}

// MultiTimeframePositionAnalysis represents LLM analysis using multiple timeframes for early warning
type MultiTimeframePositionAnalysis struct {
	Action                string            `json:"action"`                   // close_now, tighten_sl, move_to_breakeven, hold
	Confidence            float64           `json:"confidence"`               // 0.0-1.0
	Reasoning             string            `json:"reasoning"`                // Detailed explanation
	RecommendedSL         float64           `json:"recommended_sl"`           // New SL price if tightening
	TrendReversalDetected bool              `json:"trend_reversal_detected"`  // True if reversal detected
	ReversalStrength      string            `json:"reversal_strength"`        // weak, moderate, strong
	TimeframeSummary      map[string]string `json:"timeframe_summary"`        // 1m: bullish, 3m: bullish...
	Urgency               string            `json:"urgency"`                  // immediate, high, normal, low
	MomentumAgainstPosition bool            `json:"momentum_against_position"` // True if momentum working against us
	RecommendedAction     string            `json:"recommended_action"`       // Specific action recommendation
}

// TimeframeData holds candle data summary for a single timeframe
type TimeframeData struct {
	Timeframe string  `json:"timeframe"`
	Trend     string  `json:"trend"`     // bullish, bearish, neutral
	Momentum  string  `json:"momentum"`  // strong, moderate, weak
	ADX       float64 `json:"adx"`       // Trend strength
	RSI       float64 `json:"rsi"`       // Oversold/overbought
	LastClose float64 `json:"last_close"`
}

// CachedAnalysis holds cached analysis result
type CachedAnalysis struct {
	Analysis  interface{}
	Timestamp time.Time
}

// Analyzer orchestrates LLM-based market analysis
type Analyzer struct {
	config       *AnalyzerConfig
	client       *Client
	cache        map[string]*CachedAnalysis
	requestCount int
	lastReset    time.Time
	mu           sync.RWMutex
}

// NewAnalyzer creates a new LLM analyzer
func NewAnalyzer(config *AnalyzerConfig) *Analyzer {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}

	clientConfig := &ClientConfig{
		Provider:    config.Provider,
		APIKey:      config.APIKey,
		Model:       config.Model,
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
		Timeout:     120 * time.Second, // Increased for complex LLM requests (coin selection)
	}

	return &Analyzer{
		config:    config,
		client:    NewClient(clientConfig),
		cache:     make(map[string]*CachedAnalysis),
		lastReset: time.Now(),
	}
}

// AnalyzeMarket performs comprehensive market analysis
func (a *Analyzer) AnalyzeMarket(symbol, timeframe string, klines []binance.Kline) (*MarketAnalysis, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled or configured")
	}

	// Check cache
	cacheKey := fmt.Sprintf("market_%s_%s", symbol, timeframe)
	if cached := a.getFromCache(cacheKey); cached != nil {
		if analysis, ok := cached.(*MarketAnalysis); ok {
			return analysis, nil
		}
	}

	// Check rate limit
	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Build kline data string
	klineData := formatKlines(klines)
	indicators := calculateIndicatorsSummary(klines)

	prompt := BuildMarketAnalysisPrompt(symbol, timeframe, klineData, indicators)

	response, err := a.client.Complete(SystemPromptMarketAnalysis, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Strip markdown code blocks if present (DeepSeek often wraps JSON in ```)
	cleanResponse := stripMarkdownCodeBlock(response)

	var analysis MarketAnalysis
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Cache the result
	a.setCache(cacheKey, &analysis)

	return &analysis, nil
}

// AnalyzePatterns performs chart pattern recognition
func (a *Analyzer) AnalyzePatterns(symbol, timeframe string, klines []binance.Kline) (*PatternAnalysis, error) {
	if !a.config.Enabled || !a.config.EnablePatterns || !a.client.IsConfigured() {
		return nil, fmt.Errorf("pattern analysis not enabled")
	}

	cacheKey := fmt.Sprintf("patterns_%s_%s", symbol, timeframe)
	if cached := a.getFromCache(cacheKey); cached != nil {
		if analysis, ok := cached.(*PatternAnalysis); ok {
			return analysis, nil
		}
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	klineData := formatKlines(klines)
	prompt := BuildPatternPrompt(symbol, timeframe, klineData)

	response, err := a.client.Complete(SystemPromptPatternRecognition, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis PatternAnalysis
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	a.setCache(cacheKey, &analysis)
	return &analysis, nil
}

// AnalyzeBigCandle analyzes a detected big candle
func (a *Analyzer) AnalyzeBigCandle(symbol string, bigCandle binance.Kline, contextKlines []binance.Kline) (*BigCandleAnalysis, error) {
	if !a.config.Enabled || !a.config.EnableBigCandle || !a.client.IsConfigured() {
		return nil, fmt.Errorf("big candle analysis not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	candleData := fmt.Sprintf("Open: %.8f, High: %.8f, Low: %.8f, Close: %.8f, Volume: %.2f",
		bigCandle.Open, bigCandle.High, bigCandle.Low, bigCandle.Close, bigCandle.Volume)

	contextData := formatKlines(contextKlines) + "\n\nIndicators:\n" + calculateIndicatorsSummary(contextKlines)

	prompt := BuildBigCandlePrompt(symbol, candleData, contextData)

	response, err := a.client.Complete(SystemPromptBigCandleAnalysis, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis BigCandleAnalysis
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &analysis, nil
}

// AssessRisk evaluates the risk of a proposed trade
func (a *Analyzer) AssessRisk(tradeDetails map[string]interface{}, accountBalance float64, marketVolatility float64) (*RiskAssessment, error) {
	if !a.config.Enabled || !a.config.EnableRiskCheck || !a.client.IsConfigured() {
		return nil, fmt.Errorf("risk assessment not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	tradeJSON, _ := json.Marshal(tradeDetails)
	accountInfo := fmt.Sprintf("Balance: $%.2f", accountBalance)
	marketConditions := fmt.Sprintf("Current Volatility: %.2f%%", marketVolatility*100)

	prompt := BuildRiskAssessmentPrompt(string(tradeJSON), accountInfo, marketConditions)

	response, err := a.client.Complete(SystemPromptRiskAssessment, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var assessment RiskAssessment
	if err := json.Unmarshal([]byte(cleanResponse), &assessment); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &assessment, nil
}

// AnalyzeReversalProbability uses LLM to confirm a potential reversal pattern
// Takes multi-timeframe kline data and pattern info to determine if reversal is genuine
func (a *Analyzer) AnalyzeReversalProbability(
	symbol string,
	patternType string, // "lower_lows" or "higher_highs"
	direction string, // "LONG" or "SHORT"
	alignedCount int, // How many timeframes aligned (1-3)
	klines5m []binance.Kline,
	klines15m []binance.Kline,
	klines1h []binance.Kline,
) (*ReversalAnalysis, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled or configured")
	}

	// Check cache (short TTL for reversal since it's time-sensitive)
	cacheKey := fmt.Sprintf("reversal_%s_%s", symbol, direction)
	if cached := a.getFromCache(cacheKey); cached != nil {
		if analysis, ok := cached.(*ReversalAnalysis); ok {
			return analysis, nil
		}
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Build pattern info string
	patternInfo := fmt.Sprintf(`Pattern Type: %s
Direction: %s
Timeframes Aligned: %d/3
Signal: %s reversal detected (consecutive %s across %d timeframes)`,
		patternType,
		direction,
		alignedCount,
		direction,
		patternType,
		alignedCount)

	// Format klines for each timeframe (limit to last 15 candles for context)
	klines5mStr := formatKlinesLimited(klines5m, 15)
	klines15mStr := formatKlinesLimited(klines15m, 15)
	klines1hStr := formatKlinesLimited(klines1h, 15)

	// Build the prompt
	prompt := BuildReversalAnalysisPrompt(symbol, patternInfo, klines5mStr, klines15mStr, klines1hStr)

	// Call LLM with reversal analysis system prompt
	response, err := a.client.Complete(SystemPromptReversalAnalysis, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Strip markdown code blocks if present
	cleanResponse := stripMarkdownCodeBlock(response)

	var analysis ReversalAnalysis
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Cache the result (short duration since reversal signals are time-sensitive)
	a.setCache(cacheKey, &analysis)

	return &analysis, nil
}

// formatKlinesLimited formats kline data for LLM consumption, limited to N candles
func formatKlinesLimited(klines []binance.Kline, limit int) string {
	if len(klines) == 0 {
		return "No data"
	}

	// Limit to last N candles
	start := 0
	if len(klines) > limit {
		start = len(klines) - limit
	}

	result := "Time | Open | High | Low | Close | Volume\n"
	for i := start; i < len(klines); i++ {
		k := klines[i]
		openTime := time.Unix(k.OpenTime/1000, 0)
		result += fmt.Sprintf("%s | %.8f | %.8f | %.8f | %.8f | %.2f\n",
			openTime.Format("15:04"), k.Open, k.High, k.Low, k.Close, k.Volume)
	}
	return result
}

// AnalyzeAutoTrading performs autonomous trading analysis
// LLM decides position sizes, leverage, which coins to trade, averaging decisions
func (a *Analyzer) AnalyzeAutoTrading(
	watchlist []string,
	klinesBySymbol map[string][]binance.Kline,
	existingPositions []ExistingPositionInfo,
	constraints AutoModeConstraints,
	accountBalance float64,
	allocatedUSD float64,
) (*AutoTradingDecision, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled or configured")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Build watchlist string
	watchlistStr := strings.Join(watchlist, ", ")

	// Build constraints string
	constraintsStr := fmt.Sprintf(`Max Concurrent Positions: %d
Max Leverage: %dx
Max Position Size: $%.2f
Max Total USD Deployed: $%.2f
Allow Averaging: %v
Max Averages Per Position: %d
Min Hold Time: %d minutes
Quick Profit Mode: %v
Min Profit for Quick Exit: %.2f%%`,
		constraints.MaxPositions,
		constraints.MaxLeverage,
		constraints.MaxPositionSizeUSD,
		constraints.MaxTotalUSD,
		constraints.AllowAveraging,
		constraints.MaxAverages,
		constraints.MinHoldMinutes,
		constraints.QuickProfitMode,
		constraints.MinProfitForExit)

	// Build existing positions string
	existingPosStr := "None"
	if len(existingPositions) > 0 {
		existingPosStr = ""
		for _, pos := range existingPositions {
			existingPosStr += fmt.Sprintf(`
- %s: %s at $%.4f, Qty: %.4f, PnL: $%.2f (%.2f%%), Entries: %d, Held: %d min`,
				pos.Symbol,
				pos.Side,
				pos.EntryPrice,
				pos.Quantity,
				pos.UnrealizedPnL,
				pos.PnLPercent,
				pos.EntryCount,
				pos.HoldMinutes)
		}
	}

	// Build market data by symbol
	marketDataStr := ""
	for symbol, klines := range klinesBySymbol {
		if len(klines) > 0 {
			klineData := formatKlines(klines)
			indicators := calculateIndicatorsSummary(klines)
			marketDataStr += fmt.Sprintf("\n=== %s ===\n%s\n\nIndicators:\n%s\n", symbol, klineData, indicators)
		}
	}

	// Build account balance string
	availableUSD := accountBalance - allocatedUSD
	accountStr := fmt.Sprintf(`Total Balance: $%.2f
Currently Allocated: $%.2f
Available for Trading: $%.2f`,
		accountBalance,
		allocatedUSD,
		availableUSD)

	prompt := BuildAutoTradingPrompt(watchlistStr, marketDataStr, existingPosStr, constraintsStr, accountStr)

	response, err := a.client.Complete(SystemPromptAutoTradingDecision, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var decision AutoTradingDecision
	if err := json.Unmarshal([]byte(cleanResponse), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Enforce hard limits on the decisions
	decision = a.enforceAutoModeConstraints(decision, constraints)

	return &decision, nil
}

// enforceAutoModeConstraints ensures the LLM decisions don't exceed hard limits
func (a *Analyzer) enforceAutoModeConstraints(decision AutoTradingDecision, constraints AutoModeConstraints) AutoTradingDecision {
	validDecisions := make([]TradingDecisionData, 0)
	totalAllocated := 0.0
	positionCount := 0

	for _, d := range decision.TradingDecisions {
		// Skip if we've hit max positions
		if positionCount >= constraints.MaxPositions {
			continue
		}

		// Enforce max leverage
		if d.Leverage > constraints.MaxLeverage {
			d.Leverage = constraints.MaxLeverage
		}
		if d.Leverage < 1 {
			d.Leverage = 1
		}

		// Enforce max position size
		if d.PositionSizeUSD > constraints.MaxPositionSizeUSD {
			d.PositionSizeUSD = constraints.MaxPositionSizeUSD
		}

		// Enforce total USD limit
		if totalAllocated+d.PositionSizeUSD > constraints.MaxTotalUSD {
			remaining := constraints.MaxTotalUSD - totalAllocated
			if remaining > 10 { // Only if there's meaningful amount left
				d.PositionSizeUSD = remaining
			} else {
				continue
			}
		}

		// Enforce averaging constraints
		if !constraints.AllowAveraging && (d.Action == "average_down" || d.Action == "average_up") {
			continue
		}
		if d.MaxAverageCount > constraints.MaxAverages {
			d.MaxAverageCount = constraints.MaxAverages
		}

		validDecisions = append(validDecisions, d)
		totalAllocated += d.PositionSizeUSD
		if d.Action == "open_long" || d.Action == "open_short" {
			positionCount++
		}
	}

	decision.TradingDecisions = validDecisions
	return decision
}

// AnalyzePositionSLTP analyzes current position and recommends optimal SL/TP levels
func (a *Analyzer) AnalyzePositionSLTP(pos *PositionInfo, klines []binance.Kline) (*PositionSLTPAnalysis, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Build position info string
	pnlStatus := "PROFIT"
	if pos.UnrealizedPnL < 0 {
		pnlStatus = "LOSS"
	}

	positionInfo := fmt.Sprintf(`Symbol: %s
Side: %s
Entry Price: %.8f
Current Price: %.8f
Quantity: %.4f
Unrealized P&L: $%.2f (%s, %.2f%%)
Current Stop Loss: %.8f
Current Take Profit: %.8f
Position Duration: %s
Trading Mode: %s`,
		pos.Symbol,
		pos.Side,
		pos.EntryPrice,
		pos.CurrentPrice,
		pos.Quantity,
		pos.UnrealizedPnL,
		pnlStatus,
		pos.PnLPercent,
		pos.CurrentSL,
		pos.CurrentTP,
		pos.HoldDuration,
		pos.Mode)

	// Build market data string
	klineData := formatKlines(klines)
	indicators := calculateIndicatorsSummary(klines)

	prompt := BuildPositionSLTPPrompt(positionInfo, klineData, indicators)

	response, err := a.client.Complete(SystemPromptPositionSLTP, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis PositionSLTPAnalysis
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &analysis, nil
}

// AnalyzePositionHealth performs multi-timeframe analysis to detect early warning signs
// This is called every 30 seconds for underwater positions after 1 minute hold time
func (a *Analyzer) AnalyzePositionHealth(
	pos *PositionInfo,
	klines1m, klines3m, klines5m, klines15m []binance.Kline,
) (*MultiTimeframePositionAnalysis, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Build position summary
	pnlStatus := "PROFIT"
	if pos.PnLPercent < 0 {
		pnlStatus = "LOSS"
	}

	// Calculate indicators for each timeframe
	tf1m := calculateTimeframeSummary("1m", klines1m)
	tf3m := calculateTimeframeSummary("3m", klines3m)
	tf5m := calculateTimeframeSummary("5m", klines5m)
	tf15m := calculateTimeframeSummary("15m", klines15m)

	// Build the prompt with all timeframe data
	prompt := fmt.Sprintf(`URGENT: Multi-Timeframe Position Health Analysis

CURRENT POSITION (NEEDS IMMEDIATE EVALUATION):
Symbol: %s
Side: %s (we are %s)
Entry Price: %.8f
Current Price: %.8f
P&L: %.2f%% (%s)
Current Stop Loss: %.8f
Hold Duration: %s
Mode: %s

MULTI-TIMEFRAME ANALYSIS:

=== 1-MINUTE TIMEFRAME ===
%s

=== 3-MINUTE TIMEFRAME ===
%s

=== 5-MINUTE TIMEFRAME ===
%s

=== 15-MINUTE TIMEFRAME ===
%s

CRITICAL QUESTION:
We have a %s position that is currently showing %s.
Analyze ALL 4 timeframes to determine:
1. Is there a trend reversal forming AGAINST our position?
2. Is momentum accelerating AGAINST our position?
3. Should we EXIT NOW to minimize loss, or HOLD?
4. If we should tighten SL, what price?

Consider:
- If all timeframes show opposite trend to our position = HIGH URGENCY to exit
- If momentum is accelerating against us = TIGHTEN SL immediately
- If reversal signals present = Consider early exit

Respond in JSON format:
{
  "action": "close_now|tighten_sl|move_to_breakeven|hold",
  "confidence": 0.0-1.0,
  "reasoning": "detailed explanation of multi-timeframe analysis",
  "recommended_sl": price_or_0_if_not_applicable,
  "trend_reversal_detected": true/false,
  "reversal_strength": "weak|moderate|strong",
  "timeframe_summary": {"1m": "bullish/bearish/neutral", "3m": "...", "5m": "...", "15m": "..."},
  "urgency": "immediate|high|normal|low",
  "momentum_against_position": true/false,
  "recommended_action": "specific action description"
}`,
		pos.Symbol,
		pos.Side,
		pos.Side,
		pos.EntryPrice,
		pos.CurrentPrice,
		pos.PnLPercent,
		pnlStatus,
		pos.CurrentSL,
		pos.HoldDuration,
		pos.Mode,
		tf1m,
		tf3m,
		tf5m,
		tf15m,
		pos.Side,
		pnlStatus)

	systemPrompt := `You are an expert crypto trading analyst specializing in multi-timeframe analysis and position management.
Your job is to analyze open positions and detect early warning signs of trend reversals or momentum shifts.
When multiple timeframes align AGAINST a position, recommend immediate action to minimize losses.
Be decisive - if 3+ timeframes show opposite trend, recommend closing the position.
Always prioritize capital preservation over hoping for a reversal.`

	response, err := a.client.Complete(systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis MultiTimeframePositionAnalysis
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &analysis, nil
}

// calculateTimeframeSummary builds a summary string for a single timeframe
func calculateTimeframeSummary(tf string, klines []binance.Kline) string {
	if len(klines) < 10 {
		return fmt.Sprintf("Insufficient data for %s timeframe", tf)
	}

	// Calculate basic indicators
	closes := make([]float64, len(klines))
	highs := make([]float64, len(klines))
	lows := make([]float64, len(klines))
	volumes := make([]float64, len(klines))

	for i, k := range klines {
		closes[i] = k.Close
		highs[i] = k.High
		lows[i] = k.Low
		volumes[i] = k.Volume
	}

	// EMA calculations
	ema9 := calculateEMA(closes, 9)
	ema21 := calculateEMA(closes, 21)

	// RSI
	rsi := calculateRSI(closes, 14)

	// Simple trend detection
	trend := "neutral"
	if len(ema9) > 0 && len(ema21) > 0 {
		if ema9[len(ema9)-1] > ema21[len(ema21)-1] {
			trend = "bullish"
		} else if ema9[len(ema9)-1] < ema21[len(ema21)-1] {
			trend = "bearish"
		}
	}

	// Momentum (price vs EMA)
	momentum := "neutral"
	lastClose := closes[len(closes)-1]
	if len(ema9) > 0 {
		if lastClose > ema9[len(ema9)-1]*1.002 {
			momentum = "bullish"
		} else if lastClose < ema9[len(ema9)-1]*0.998 {
			momentum = "bearish"
		}
	}

	// Recent price action
	priceChange := 0.0
	if len(closes) >= 5 {
		priceChange = ((closes[len(closes)-1] - closes[len(closes)-5]) / closes[len(closes)-5]) * 100
	}

	// Volume trend
	avgVol := 0.0
	for _, v := range volumes {
		avgVol += v
	}
	avgVol /= float64(len(volumes))
	volTrend := "normal"
	if len(volumes) > 0 && volumes[len(volumes)-1] > avgVol*1.5 {
		volTrend = "high"
	} else if len(volumes) > 0 && volumes[len(volumes)-1] < avgVol*0.5 {
		volTrend = "low"
	}

	return fmt.Sprintf(`Trend: %s
Momentum: %s
RSI(14): %.1f
EMA9: %.8f
EMA21: %.8f
Last Close: %.8f
5-bar Change: %.2f%%
Volume: %s
Recent Candles: %s`,
		trend,
		momentum,
		rsi,
		ema9[len(ema9)-1],
		ema21[len(ema21)-1],
		lastClose,
		priceChange,
		volTrend,
		formatRecentCandles(klines, 5))
}

// formatRecentCandles formats the last N candles for the prompt
func formatRecentCandles(klines []binance.Kline, n int) string {
	if len(klines) < n {
		n = len(klines)
	}

	var result []string
	start := len(klines) - n
	for i := start; i < len(klines); i++ {
		k := klines[i]
		candleType := "DOJI"
		if k.Close > k.Open {
			candleType = "GREEN"
		} else if k.Close < k.Open {
			candleType = "RED"
		}
		result = append(result, fmt.Sprintf("%s(%.2f%%)", candleType, ((k.Close-k.Open)/k.Open)*100))
	}
	return strings.Join(result, " → ")
}

// calculateEMA calculates Exponential Moving Average
func calculateEMA(data []float64, period int) []float64 {
	if len(data) < period {
		return []float64{data[len(data)-1]}
	}

	multiplier := 2.0 / float64(period+1)
	ema := make([]float64, len(data))

	// Start with SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	ema[period-1] = sum / float64(period)

	// Calculate EMA
	for i := period; i < len(data); i++ {
		ema[i] = (data[i]-ema[i-1])*multiplier + ema[i-1]
	}

	return ema
}

// calculateRSI calculates Relative Strength Index
func calculateRSI(closes []float64, period int) float64 {
	if len(closes) < period+1 {
		return 50.0
	}

	gains := 0.0
	losses := 0.0

	for i := len(closes) - period; i < len(closes); i++ {
		change := closes[i] - closes[i-1]
		if change > 0 {
			gains += change
		} else {
			losses -= change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

// GetSignalFromAnalysis converts market analysis to trading signal
func (a *Analyzer) GetSignalFromAnalysis(analysis *MarketAnalysis, currentPrice float64) (direction string, confidence float64, entry, stop, target float64) {
	if analysis == nil || analysis.Confidence < a.config.MinConfidence {
		return "neutral", 0, 0, 0, 0
	}

	direction = analysis.Direction
	confidence = analysis.Confidence

	if analysis.EntryPrice != nil {
		entry = *analysis.EntryPrice
	} else {
		entry = currentPrice
	}

	if analysis.StopLoss != nil {
		stop = *analysis.StopLoss
	}

	if analysis.TakeProfit != nil {
		target = *analysis.TakeProfit
	}

	return direction, confidence, entry, stop, target
}

// getFromCache retrieves cached analysis
func (a *Analyzer) getFromCache(key string) interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if cached, exists := a.cache[key]; exists {
		if time.Since(cached.Timestamp) < a.config.CacheDuration {
			return cached.Analysis
		}
	}
	return nil
}

// setCache stores analysis in cache
func (a *Analyzer) setCache(key string, analysis interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cache[key] = &CachedAnalysis{
		Analysis:  analysis,
		Timestamp: time.Now(),
	}
}

// checkRateLimit checks if we're within rate limits
func (a *Analyzer) checkRateLimit() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Reset counter every minute
	if time.Since(a.lastReset) > time.Minute {
		a.requestCount = 0
		a.lastReset = time.Now()
	}

	if a.requestCount >= a.config.RateLimitPerMin {
		return false
	}

	a.requestCount++
	return true
}

// IsEnabled returns if the analyzer is enabled
func (a *Analyzer) IsEnabled() bool {
	return a.config.Enabled && a.client.IsConfigured()
}

// GetClient returns the underlying LLM client for direct use
func (a *Analyzer) GetClient() *Client {
	return a.client
}

// ClearCache clears the analysis cache
func (a *Analyzer) ClearCache() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cache = make(map[string]*CachedAnalysis)
}

// formatKlines formats kline data for LLM consumption
func formatKlines(klines []binance.Kline) string {
	if len(klines) == 0 {
		return "No data"
	}

	// Limit to last 50 candles for token efficiency
	start := 0
	if len(klines) > 50 {
		start = len(klines) - 50
	}

	result := "Time | Open | High | Low | Close | Volume\n"
	for i := start; i < len(klines); i++ {
		k := klines[i]
		// Convert Unix milliseconds to time
		openTime := time.Unix(k.OpenTime/1000, 0)
		result += fmt.Sprintf("%s | %.8f | %.8f | %.8f | %.8f | %.2f\n",
			openTime.Format("15:04"), k.Open, k.High, k.Low, k.Close, k.Volume)
	}
	return result
}

// calculateIndicatorsSummary calculates basic indicators for LLM context
func calculateIndicatorsSummary(klines []binance.Kline) string {
	if len(klines) < 20 {
		return "Insufficient data for indicators"
	}

	// Calculate simple moving averages
	sma20 := calculateSMA(klines, 20)
	sma50 := 0.0
	if len(klines) >= 50 {
		sma50 = calculateSMA(klines, 50)
	}

	// Calculate RSI
	rsi := calculateRSIFromKlines(klines, 14)

	// Calculate volatility (ATR-like)
	volatility := calculateVolatility(klines, 14)

	// Current price and change
	current := klines[len(klines)-1]
	prev := klines[len(klines)-2]
	priceChange := (current.Close - prev.Close) / prev.Close * 100

	result := fmt.Sprintf(`Current Price: %.8f
Price Change: %.2f%%
SMA20: %.8f
SMA50: %.8f
RSI(14): %.2f
Volatility: %.2f%%
Volume (current): %.2f
Volume (avg): %.2f`,
		current.Close,
		priceChange,
		sma20,
		sma50,
		rsi,
		volatility*100,
		current.Volume,
		calculateAvgVolume(klines, 20))

	return result
}

// calculateSMA calculates simple moving average
func calculateSMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	return sum / float64(period)
}

// calculateRSIFromKlines calculates RSI indicator from klines
func calculateRSIFromKlines(klines []binance.Kline, period int) float64 {
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
			losses += -change
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

// calculateVolatility calculates price volatility
func calculateVolatility(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		range_ := (klines[i].High - klines[i].Low) / klines[i].Close
		sum += range_
	}
	return sum / float64(period)
}

// calculateAvgVolume calculates average volume
func calculateAvgVolume(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Volume
	}
	return sum / float64(period)
}

// ============ SCALP RE-ENTRY ANALYSIS METHODS ============

// ScalpReentryAnalysisRequest contains data for re-entry analysis
type ScalpReentryAnalysisRequest struct {
	Symbol            string
	Side              string
	EntryPrice        float64
	CurrentPrice      float64
	Breakeven         float64
	DistanceFromBE    float64
	TPLevel           int
	TPPercent         float64
	SoldQty           float64
	ReentryQty        float64
	ReentryPercent    float64
	Trend5m           string
	TrendStrength5m   float64
	Trend15m          string
	RSI14             float64
	VolumeRatio       float64
	ADX               float64
	ATR               float64
	PriceChange1m     float64
	PriceChange5m     float64
	PriceChange15m    float64
	DistanceToSupport float64
	DistanceToResistance float64
}

// ScalpReentryAnalysisResponse contains AI decision for re-entry
type ScalpReentryAnalysisResponse struct {
	ShouldReenter     bool     `json:"should_reenter"`
	Confidence        float64  `json:"confidence"`
	RecommendedQtyPct float64  `json:"recommended_qty_pct"`
	Reasoning         string   `json:"reasoning"`
	MarketCondition   string   `json:"market_condition"`
	TrendAligned      bool     `json:"trend_aligned"`
	RiskLevel         string   `json:"risk_level"`
	CautionFlags      []string `json:"caution_flags"`
}

// AnalyzeScalpReentry analyzes whether to execute a re-entry
func (a *Analyzer) AnalyzeScalpReentry(req *ScalpReentryAnalysisRequest) (*ScalpReentryAnalysisResponse, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	prompt := fmt.Sprintf(`You are an AI trading assistant analyzing a scalp re-entry opportunity.

## Current Position
- Symbol: %s
- Side: %s
- Entry Price: $%.8f
- Current Price: $%.8f
- Breakeven: $%.8f
- Distance from BE: %.2f%%
- TP Level Just Hit: %d (%.2f%%)
- Sold Quantity: %.4f
- Potential Re-entry Qty: %.4f (%.0f%% of sold)

## Market Context
- 5m Trend: %s (strength: %.0f)
- 15m Trend: %s
- RSI (14): %.1f
- Volume Ratio: %.2fx average
- ADX: %.1f
- ATR: %.8f

## Recent Price Action
- 1m Change: %.2f%%
- 5m Change: %.2f%%
- 15m Change: %.2f%%
- Distance to Support: %.2f%%
- Distance to Resistance: %.2f%%

## Task
Analyze whether we should execute this re-entry when price returns to breakeven.

Consider:
1. Is the trend still favorable for our position side?
2. Is there enough momentum for another push to the next TP level?
3. What's the risk/reward ratio of re-entering vs staying flat?
4. Are there any warning signs (divergences, exhaustion, reversal patterns)?
5. Volume profile - is it confirming the move?

Respond ONLY with valid JSON (no markdown, no code blocks):
{
  "should_reenter": true or false,
  "confidence": 0.0 to 1.0,
  "recommended_qty_pct": 0.5 to 1.0 (of configured reentry%%),
  "reasoning": "brief 1-2 sentence explanation",
  "market_condition": "trending" or "ranging" or "volatile" or "calm",
  "trend_aligned": true or false,
  "risk_level": "low" or "medium" or "high",
  "caution_flags": ["list", "of", "warnings"] or []
}`,
		req.Symbol,
		req.Side,
		req.EntryPrice,
		req.CurrentPrice,
		req.Breakeven,
		req.DistanceFromBE,
		req.TPLevel,
		req.TPPercent,
		req.SoldQty,
		req.ReentryQty,
		req.ReentryPercent,
		req.Trend5m,
		req.TrendStrength5m,
		req.Trend15m,
		req.RSI14,
		req.VolumeRatio,
		req.ADX,
		req.ATR,
		req.PriceChange1m,
		req.PriceChange5m,
		req.PriceChange15m,
		req.DistanceToSupport,
		req.DistanceToResistance)

	systemPrompt := `You are an expert crypto scalp trading AI specializing in re-entry decisions.
Your job is to analyze whether a position should re-enter after taking partial profit.
Consider trend alignment, momentum, volume, and risk carefully.
Be conservative - only recommend re-entry if the setup is favorable.`

	response, err := a.client.Complete(systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis ScalpReentryAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &analysis, nil
}

// TPTimingAnalysisRequest contains data for TP timing analysis
type TPTimingAnalysisRequest struct {
	Symbol              string
	Side                string
	EntryPrice          float64
	CurrentPrice        float64
	CurrentProfitPct    float64
	TargetTPLevel       int
	TargetTPPercent     float64
	RemainingQty        float64
	AccumulatedProfit   float64
	RSI14               float64
	MACDHist            float64
	MACDTrend           string
	VolumeTrend         string
	ADX                 float64
	DistanceToTP        float64
	NearResistance      bool
	ResistanceLevel     float64
	SupportLevel        float64
}

// TPTimingAnalysisResponse contains AI decision for TP timing
type TPTimingAnalysisResponse struct {
	ShouldTakeNow   bool    `json:"should_take_now"`
	Confidence      float64 `json:"confidence"`
	OptimalPercent  float64 `json:"optimal_percent"`
	Reasoning       string  `json:"reasoning"`
	MomentumStatus  string  `json:"momentum_status"`
	VolumeStatus    string  `json:"volume_status"`
}

// AnalyzeTPTiming analyzes optimal TP timing
func (a *Analyzer) AnalyzeTPTiming(req *TPTimingAnalysisRequest) (*TPTimingAnalysisResponse, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	prompt := fmt.Sprintf(`You are an AI trading assistant optimizing take profit timing.

## Current Position
- Symbol: %s
- Side: %s
- Entry Price: $%.8f
- Current Price: $%.8f
- Current Profit: %.2f%%
- Target TP Level: %d at %.2f%%
- Remaining Quantity: %.4f
- Accumulated Profit: $%.2f

## Market Momentum
- RSI: %.1f
- MACD Histogram: %.6f (%s)
- Volume Trend: %s
- Trend Strength (ADX): %.1f

## Price Structure
- Distance to TP: %.2f%%
- Near Resistance: %t
- Resistance Level: $%.8f
- Support Level: $%.8f

## Task
Decide if we should take profit NOW or wait for the configured TP level.

Consider:
1. Is momentum accelerating or decelerating?
2. Is there strong resistance preventing further upside?
3. Is volume confirming the move?
4. Are there signs of exhaustion or reversal?

Respond ONLY with valid JSON:
{
  "should_take_now": true or false,
  "confidence": 0.0 to 1.0,
  "optimal_percent": 0 to 100 (%% to close now, 0 = wait),
  "reasoning": "brief explanation",
  "momentum_status": "accelerating" or "stable" or "decelerating" or "reversing",
  "volume_status": "increasing" or "stable" or "decreasing"
}`,
		req.Symbol,
		req.Side,
		req.EntryPrice,
		req.CurrentPrice,
		req.CurrentProfitPct,
		req.TargetTPLevel,
		req.TargetTPPercent,
		req.RemainingQty,
		req.AccumulatedProfit,
		req.RSI14,
		req.MACDHist,
		req.MACDTrend,
		req.VolumeTrend,
		req.ADX,
		req.DistanceToTP,
		req.NearResistance,
		req.ResistanceLevel,
		req.SupportLevel)

	systemPrompt := `You are an expert crypto trading AI specializing in take profit optimization.
Your job is to determine the optimal moment to take profit.
Balance between capturing gains and not exiting too early.
Consider momentum, volume, and resistance levels carefully.`

	response, err := a.client.Complete(systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis TPTimingAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &analysis, nil
}

// DynamicSLAnalysisRequest contains data for dynamic SL analysis
type DynamicSLAnalysisRequest struct {
	Symbol            string
	Side              string
	EntryPrice        float64
	CurrentPrice      float64
	AccumulatedProfit float64
	ProtectedProfit   float64
	MaxAllowableLoss  float64
	CurrentSL         float64
	ATR14             float64
	ATRPercent        float64
	VolatilityRegime  string
	RecentSwingRange  float64
	NearestSupport    float64
	NearestResistance float64
}

// DynamicSLAnalysisResponse contains AI decision for dynamic SL
type DynamicSLAnalysisResponse struct {
	RecommendedSL    float64 `json:"recommended_sl"`
	ProtectionPct    float64 `json:"protection_percent"`
	Reasoning        string  `json:"reasoning"`
	SLBasis          string  `json:"sl_basis"`
	VolatilityBuffer float64 `json:"volatility_buffer"`
	Confidence       float64 `json:"confidence"`
}

// AnalyzeDynamicSL analyzes optimal dynamic stop loss
func (a *Analyzer) AnalyzeDynamicSL(req *DynamicSLAnalysisRequest) (*DynamicSLAnalysisResponse, error) {
	if !a.config.Enabled || !a.client.IsConfigured() {
		return nil, fmt.Errorf("LLM analyzer not enabled")
	}

	if !a.checkRateLimit() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	prompt := fmt.Sprintf(`You are an AI risk manager calculating dynamic stop loss.

## Position Details
- Symbol: %s
- Side: %s
- Entry Price: $%.8f
- Current Price: $%.8f
- Accumulated Profit: $%.2f
- Protected Profit (60%%): $%.2f
- Max Allowable Loss (40%%): $%.2f
- Current SL: $%.8f

## Market Volatility
- ATR (14): %.8f
- ATR %%: %.2f%%
- Volatility Regime: %s
- Recent Swings: %.2f%%

## Support/Resistance
- Nearest Support: $%.8f
- Nearest Resistance: $%.8f

## Task
Calculate the optimal dynamic stop loss that:
1. Protects at least 60%% of accumulated profit
2. Allows room for normal volatility
3. Is placed at a logical technical level (support/resistance)
4. Balances protection with not getting stopped out prematurely

Respond ONLY with valid JSON:
{
  "recommended_sl": price as number,
  "protection_percent": 60 to 80 (%% of profit protected),
  "reasoning": "brief technical explanation",
  "sl_basis": "atr" or "support" or "swing_low" or "hybrid",
  "volatility_buffer": 0.0 to 2.0 (ATR multiplier used),
  "confidence": 0.0 to 1.0
}`,
		req.Symbol,
		req.Side,
		req.EntryPrice,
		req.CurrentPrice,
		req.AccumulatedProfit,
		req.ProtectedProfit,
		req.MaxAllowableLoss,
		req.CurrentSL,
		req.ATR14,
		req.ATRPercent,
		req.VolatilityRegime,
		req.RecentSwingRange,
		req.NearestSupport,
		req.NearestResistance)

	systemPrompt := `You are an expert crypto risk management AI.
Your job is to calculate optimal stop loss levels that protect profits while allowing trades to breathe.
Balance between protection and not getting stopped out on normal volatility.
Always ensure the recommended SL protects at least 60% of accumulated profit.`

	response, err := a.client.Complete(systemPrompt, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	cleanResponse := stripMarkdownCodeBlock(response)
	var analysis DynamicSLAnalysisResponse
	if err := json.Unmarshal([]byte(cleanResponse), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return &analysis, nil
}
