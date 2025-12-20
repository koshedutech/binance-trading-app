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
		Timeout:     30 * time.Second,
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
	rsi := calculateRSI(klines, 14)

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

// calculateRSI calculates RSI indicator
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
