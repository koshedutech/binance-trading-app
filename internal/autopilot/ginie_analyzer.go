package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// GinieAnalyzer implements the Adaptive Crypto Trading AI (Ginie)
type GinieAnalyzer struct {
	futuresClient binance.FuturesClient
	logger        *logging.Logger
	config        *GinieConfig
	settings      *AutopilotSettings // Settings for trend timeframes and divergence detection

	// LLM client for AI-based coin selection
	llmClient *llm.Client

	// Signal aggregator for getting market signals
	signalAggregator *SignalAggregator

	// Cached data
	coinScans      map[string]*GinieCoinScan
	scanLock       sync.RWMutex
	lastScanTime   time.Time

	// LLM coin selection cache
	llmCoinsCache     []string
	llmCoinsCacheTime time.Time
	llmCoinsCacheTTL  time.Duration

	// Decision history
	decisions      []GinieDecisionReport
	decisionLock   sync.RWMutex
	maxDecisions   int

	// Performance tracking
	dailyPnL       float64
	dailyTrades    int
	wins           int
	losses         int

	// State
	enabled        bool
	activeMode     GinieTradingMode
	watchSymbols   []string
}

// NewGinieAnalyzer creates a new Ginie AI analyzer
func NewGinieAnalyzer(
	futuresClient binance.FuturesClient,
	signalAggregator *SignalAggregator,
	logger *logging.Logger,
) *GinieAnalyzer {
	// Core coins to always include (essential for the market)
	coreSymbols := []string{
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
	}

	sm := GetSettingsManager()
	settings := sm.GetCurrentSettings()

	g := &GinieAnalyzer{
		futuresClient:    futuresClient,
		signalAggregator: signalAggregator,
		logger:           logger,
		config:           DefaultGinieConfig(),
		settings:         settings,
		coinScans:        make(map[string]*GinieCoinScan),
		decisions:        make([]GinieDecisionReport, 0, 500),
		maxDecisions:     500, // Increased for study purposes
		enabled:          true,
		activeMode:       GinieModeSwing,
		watchSymbols:     coreSymbols, // Start with core coins, LLM will update
		llmCoinsCacheTTL: 30 * time.Minute, // Refresh LLM coin list every 30 minutes
	}

	// Load LLM-selected coins in background (will call DeepSeek for 50 coins)
	go g.LoadLLMSelectedCoins()

	return g
}

// SetLLMClient sets the LLM client for AI-based coin selection
func (g *GinieAnalyzer) SetLLMClient(client *llm.Client) {
	g.llmClient = client
	if g.logger != nil {
		g.logger.Info("Ginie LLM client configured", "provider", client.GetProvider())
	}
	// Trigger coin selection when LLM is set
	go g.LoadLLMSelectedCoins()
}

// RefreshSettings reloads settings from SettingsManager
func (g *GinieAnalyzer) RefreshSettings() {
	sm := GetSettingsManager()
	g.scanLock.Lock()
	g.settings = sm.GetCurrentSettings()
	g.scanLock.Unlock()
}

// DetectTrendDivergence compares two TrendHealth analyses to detect divergence
func (g *GinieAnalyzer) DetectTrendDivergence(
	scanTrend TrendHealth,
	decisionTrend TrendHealth,
	blockOnDivergence bool,
) *TrendDivergence {
	div := &TrendDivergence{
		Detected:          false,
		ScanTimeframe:     scanTrend.Timeframe,
		ScanTrend:         scanTrend.TrendDirection,
		DecisionTimeframe: decisionTrend.Timeframe,
		DecisionTrend:     decisionTrend.TrendDirection,
		Severity:          "none",
		ShouldBlock:       false,
	}

	// No divergence if timeframes are the same
	if scanTrend.Timeframe == decisionTrend.Timeframe {
		return div
	}

	// SEVERE: Opposite directions (bullish vs bearish)
	if (scanTrend.TrendDirection == "bullish" && decisionTrend.TrendDirection == "bearish") ||
		(scanTrend.TrendDirection == "bearish" && decisionTrend.TrendDirection == "bullish") {
		div.Detected = true
		div.Severity = "severe"
		div.Reason = fmt.Sprintf("Opposite trends: %s shows %s but %s shows %s",
			scanTrend.Timeframe, scanTrend.TrendDirection,
			decisionTrend.Timeframe, decisionTrend.TrendDirection)
		div.ShouldBlock = blockOnDivergence
		return div
	}

	// MODERATE: One trending, one neutral/ranging
	if (scanTrend.TrendDirection != "neutral" && decisionTrend.TrendDirection == "neutral") ||
		(scanTrend.TrendDirection == "neutral" && decisionTrend.TrendDirection != "neutral") {
		div.Detected = true
		div.Severity = "moderate"
		div.Reason = fmt.Sprintf("Trend mismatch: %s is %s but %s is %s",
			scanTrend.Timeframe, scanTrend.TrendDirection,
			decisionTrend.Timeframe, decisionTrend.TrendDirection)
		div.ShouldBlock = blockOnDivergence
		return div
	}

	// MINOR: Same direction but significantly different ADX strengths
	if scanTrend.TrendDirection == decisionTrend.TrendDirection &&
		scanTrend.TrendDirection != "neutral" {
		adxDiff := math.Abs(scanTrend.ADXValue - decisionTrend.ADXValue)
		if adxDiff > 15 {
			div.Detected = true
			div.Severity = "minor"
			div.Reason = fmt.Sprintf("Same trend direction but ADX differs significantly: %s (%.1f) vs %s (%.1f)",
				scanTrend.Timeframe, scanTrend.ADXValue,
				decisionTrend.Timeframe, decisionTrend.ADXValue)
			div.ShouldBlock = false // Never block on minor divergence
			return div
		}
	}

	return div
}

// LLMCoinSelectionResponse represents the LLM response for coin selection
type LLMCoinSelectionResponse struct {
	Coins []struct {
		Symbol   string `json:"symbol"`
		Category string `json:"category"` // high_volume, gainer, loser, 1h_volume, stable, medium, volatile, most_traded
		Reason   string `json:"reason"`
	} `json:"coins"`
}

// VolatilityRegime classifies market volatility and provides adaptive parameters
type VolatilityRegime struct {
	Level            string        `json:"level"`            // extreme, high, medium, low
	ATRRatio         float64       `json:"atr_ratio"`        // Ratio of current ATR to baseline
	BBWidthPercent   float64       `json:"bb_width_percent"` // Bollinger Band width as % of price
	ReEntryDelay     time.Duration `json:"re_entry_delay"`   // Adaptive delay between trades
	MaxTradesPerHour int           `json:"max_trades_per_hour"` // Rate limit based on volatility
	LastUpdate       time.Time     `json:"last_update"`
}

// UltraFastSignal represents a multi-layer signal for ultra-fast scalping
type UltraFastSignal struct {
	Symbol           string            `json:"symbol"`
	TrendBias        string            `json:"trend_bias"`        // LONG, SHORT, NEUTRAL
	TrendStrength    float64           `json:"trend_strength"`    // 0-100
	ADXValue         float64           `json:"adx_value"`         // Raw ADX value for tracking
	VolatilityRegime *VolatilityRegime `json:"volatility_regime"`
	EntryConfidence  float64           `json:"entry_confidence"`  // 0-100
	MinProfitTarget  float64           `json:"min_profit_target"` // % (fee + ATR buffer)
	MaxHoldTime      time.Duration     `json:"max_hold_time"`     // Maximum time to hold
	SignalTime       time.Time         `json:"signal_time"`       // When signal was generated
	GeneratedAt      time.Time         `json:"generated_at"`

	// Signal quality filter results
	VolumeConfirmed     bool     `json:"volume_confirmed"`      // Volume > threshold
	VolumeMultiplier    float64  `json:"volume_multiplier"`     // Actual volume/avg ratio
	MomentumStrength    float64  `json:"momentum_strength"`     // Price momentum %
	MomentumConfirmed   bool     `json:"momentum_confirmed"`    // Meets momentum threshold
	AvgCandleBodyPct    float64  `json:"avg_candle_body_pct"`   // Avg body size of entry candles
	CandleBodyConfirmed bool     `json:"candle_body_confirmed"` // Meets body size threshold
	FiltersApplied      []string `json:"filters_applied"`       // List of filters that passed
	FiltersFailed       []string `json:"filters_failed"`        // List of filters that failed

	// Multi-timeframe trend alignment (5m/3m/1m weighted consensus)
	TrendAligned          bool    `json:"trend_aligned"`            // Weighted consensus agrees
	Trend5mBias           string  `json:"trend_5m_bias"`            // 5m trend direction
	Trend5mStrength       float64 `json:"trend_5m_strength"`        // 5m trend strength 0-100
	Trend3mBias           string  `json:"trend_3m_bias"`            // 3m trend direction
	Trend3mStrength       float64 `json:"trend_3m_strength"`        // 3m trend strength 0-100
	Trend1mBias           string  `json:"trend_1m_bias"`            // 1m trend direction
	Trend1mStrength       float64 `json:"trend_1m_strength"`        // 1m trend strength 0-100
	CombinedTrendStrength float64 `json:"combined_trend_strength"`  // Weighted sum of all timeframes
	TimeframeConsensus    int     `json:"timeframe_consensus"`      // Count of aligned timeframes (0-3)
	AlignmentReason       string  `json:"alignment_reason"`         // Why aligned or not

	// Trend stability check (last 3 candles)
	TrendStable       bool   `json:"trend_stable"`        // True if trend hasn't flipped in last 3 candles
	TrendFlipCount    int    `json:"trend_flip_count"`    // Number of direction changes in last 3 candles
	StabilityReason   string `json:"stability_reason"`    // Why stable or unstable
}

// LoadLLMSelectedCoins asks DeepSeek to provide 100 coins based on market criteria
func (g *GinieAnalyzer) LoadLLMSelectedCoins() error {
	// Check cache first
	if len(g.llmCoinsCache) > 0 && time.Since(g.llmCoinsCacheTime) < g.llmCoinsCacheTTL {
		g.watchSymbols = g.llmCoinsCache
		return nil
	}

	if g.llmClient == nil || !g.llmClient.IsConfigured() {
		if g.logger != nil {
			g.logger.Warn("LLM client not configured, falling back to market movers")
		}
		// Fallback to market movers
		return g.LoadDynamicSymbols(25)
	}

	if g.logger != nil {
		g.logger.Info("Ginie requesting AI-selected coins from LLM")
	}

	// Get current market data to provide context
	tickers, err := g.futuresClient.GetAll24hrTickers()
	if err != nil {
		if g.logger != nil {
			g.logger.Error("Failed to get tickers for LLM context", "error", err)
		}
		return g.LoadDynamicSymbols(25)
	}

	// Build market summary for LLM
	marketSummary := g.buildMarketSummaryForLLM(tickers)

	systemPrompt := `You are a cryptocurrency trading AI assistant specializing in selecting coins for futures trading on Binance.
Your task is to analyze current market conditions and select exactly 100 cryptocurrency trading pairs (USDT perpetual futures) based on the following criteria:

Categories to include (aim for roughly equal distribution):
1. HIGH_VOLUME: Top coins by 24h trading volume (most liquid)
2. GAINER: Top gainers with positive 24h price change (momentum plays)
3. LOSER: Top losers with negative 24h price change (reversal/short opportunities)
4. 1H_VOLUME: Coins with high volume in the last 1 hour (current activity)
5. STABLE: Low volatility coins with steady price action (range trading)
6. MEDIUM: Medium volatility coins (balanced risk/reward)
7. VOLATILE: High volatility coins (high risk/high reward scalping)
8. MOST_TRADED: Currently most actively traded coins

IMPORTANT RULES:
- Only include coins that end with USDT (e.g., BTCUSDT, ETHUSDT)
- Exclude stablecoins (USDCUSDT, BUSDUSDT, TUSDUSDT, DAIUSDT, FDUSDUSDT)
- Prioritize coins with daily volume > $1M USD
- Include major coins (BTC, ETH, BNB, SOL, XRP) regardless of other criteria
- Return EXACTLY 100 unique coins

Respond ONLY with a valid JSON object in this exact format (no markdown, no explanation):
{"coins":[{"symbol":"BTCUSDT","category":"high_volume","reason":"highest volume"},...]}`

	userPrompt := fmt.Sprintf(`Based on the current Binance Futures market data below, select 100 coins for trading:

%s

Return EXACTLY 100 unique USDT perpetual futures symbols in the JSON format specified.`, marketSummary)

	response, err := g.llmClient.Complete(systemPrompt, userPrompt)
	if err != nil {
		if g.logger != nil {
			g.logger.Error("LLM coin selection failed", "error", err)
		}
		return g.LoadDynamicSymbols(25)
	}

	// Parse the response
	coins, err := g.parseLLMCoinResponse(response)
	if err != nil {
		if g.logger != nil {
			g.logger.Error("Failed to parse LLM coin response", "error", err)
		}
		return g.LoadDynamicSymbols(25)
	}

	// Validate and deduplicate
	uniqueCoins := make(map[string]bool)
	validCoins := make([]string, 0, 100)

	// Always include core coins first
	coreCoins := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}
	for _, coin := range coreCoins {
		uniqueCoins[coin] = true
		validCoins = append(validCoins, coin)
	}

	// Add LLM-selected coins
	for _, coin := range coins {
		if !uniqueCoins[coin] && strings.HasSuffix(coin, "USDT") {
			uniqueCoins[coin] = true
			validCoins = append(validCoins, coin)
		}
		if len(validCoins) >= 100 {
			break
		}
	}

	// If we don't have enough, supplement with market movers
	if len(validCoins) < 100 {
		if g.logger != nil {
			g.logger.Warn("LLM returned insufficient coins, supplementing with market movers", "llm_count", len(validCoins))
		}
		movers, _ := g.GetMarketMovers(30)
		if movers != nil {
			for _, coin := range movers.TopVolume {
				if !uniqueCoins[coin] {
					uniqueCoins[coin] = true
					validCoins = append(validCoins, coin)
				}
			}
			for _, coin := range movers.TopGainers {
				if !uniqueCoins[coin] {
					uniqueCoins[coin] = true
					validCoins = append(validCoins, coin)
				}
			}
			for _, coin := range movers.TopLosers {
				if !uniqueCoins[coin] {
					uniqueCoins[coin] = true
					validCoins = append(validCoins, coin)
				}
			}
			for _, coin := range movers.HighVolatility {
				if !uniqueCoins[coin] {
					uniqueCoins[coin] = true
					validCoins = append(validCoins, coin)
				}
			}
		}
	}

	// Update cache and watchlist
	g.llmCoinsCache = validCoins
	g.llmCoinsCacheTime = time.Now()
	g.watchSymbols = validCoins

	if g.logger != nil {
		g.logger.Info("Ginie loaded AI-selected coins", map[string]interface{}{
			"total_coins": len(validCoins),
			"source":      "llm",
			"sample":      validCoins[:min(10, len(validCoins))],
		})
	}

	return nil
}

// buildMarketSummaryForLLM creates a market summary for the LLM prompt
func (g *GinieAnalyzer) buildMarketSummaryForLLM(tickers []binance.Futures24hrTicker) string {
	// Filter and sort tickers
	var validTickers []binance.Futures24hrTicker
	stablecoins := map[string]bool{
		"USDCUSDT": true, "BUSDUSDT": true, "TUSDUSDT": true,
		"DAIUSDT": true, "FDUSDUSDT": true, "EURUSDT": true,
	}

	for _, t := range tickers {
		if strings.HasSuffix(t.Symbol, "USDT") && !stablecoins[t.Symbol] && t.QuoteVolume > 100000 {
			validTickers = append(validTickers, t)
		}
	}

	// Sort by volume
	sort.Slice(validTickers, func(i, j int) bool {
		return validTickers[i].QuoteVolume > validTickers[j].QuoteVolume
	})

	// Build summary (limit to top 150 to give LLM enough context for 100 coin selection)
	limit := 150
	if len(validTickers) < limit {
		limit = len(validTickers)
	}

	var sb strings.Builder
	sb.WriteString("CURRENT MARKET DATA (Top coins by volume):\n")
	sb.WriteString("Symbol | 24h Volume (USD) | 24h Change % | Last Price\n")
	sb.WriteString("-------------------------------------------------\n")

	for i := 0; i < limit; i++ {
		t := validTickers[i]
		sb.WriteString(fmt.Sprintf("%s | $%.0f | %.2f%% | %.8f\n",
			t.Symbol, t.QuoteVolume, t.PriceChangePercent, t.LastPrice))
	}

	return sb.String()
}

// parseLLMCoinResponse parses the LLM response to extract coins
func (g *GinieAnalyzer) parseLLMCoinResponse(response string) ([]string, error) {
	// Strip markdown code blocks if present
	response = strings.TrimSpace(response)
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*\\n?(.*?)\\n?```$")
	if matches := re.FindStringSubmatch(response); len(matches) > 1 {
		response = strings.TrimSpace(matches[1])
	}

	var parsed LLMCoinSelectionResponse
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		// Try to extract symbols manually if JSON parsing fails
		return g.extractSymbolsFromText(response), nil
	}

	coins := make([]string, 0, len(parsed.Coins))
	for _, c := range parsed.Coins {
		if c.Symbol != "" {
			coins = append(coins, strings.ToUpper(c.Symbol))
		}
	}

	return coins, nil
}

// extractSymbolsFromText extracts USDT trading pairs from text response
func (g *GinieAnalyzer) extractSymbolsFromText(text string) []string {
	// Match patterns like BTCUSDT, ETHUSDT etc
	re := regexp.MustCompile(`[A-Z0-9]+USDT`)
	matches := re.FindAllString(strings.ToUpper(text), -1)

	// Deduplicate
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	return result
}

// RefreshLLMCoins forces a refresh of the LLM coin selection
func (g *GinieAnalyzer) RefreshLLMCoins() (int, error) {
	g.llmCoinsCacheTime = time.Time{} // Clear cache
	err := g.LoadLLMSelectedCoins()
	return len(g.watchSymbols), err
}

// LoadAllSymbols loads all available futures trading symbols from Binance
func (g *GinieAnalyzer) LoadAllSymbols() error {
	if g.logger != nil {
		g.logger.Info("Ginie loading all futures symbols from Binance", nil)
	}

	symbols, err := g.futuresClient.GetFuturesSymbols()
	if err != nil {
		if g.logger != nil {
			g.logger.Error("Failed to load futures symbols, using market movers fallback", "error", err)
		}
		// Use market movers as fallback instead of hardcoded list
		return g.LoadDynamicSymbols(25)
	}

	// Filter for USDT perpetual contracts only
	usdtSymbols := make([]string, 0)
	for _, s := range symbols {
		if len(s) > 4 && s[len(s)-4:] == "USDT" {
			usdtSymbols = append(usdtSymbols, s)
		}
	}

	g.watchSymbols = usdtSymbols

	if g.logger != nil {
		g.logger.Info("Ginie loaded futures symbols from Binance", "count", len(g.watchSymbols))
	}

	return nil
}

// RefreshSymbols manually refreshes the symbol list
func (g *GinieAnalyzer) RefreshSymbols() (int, error) {
	err := g.LoadAllSymbols()
	return len(g.watchSymbols), err
}

// SetWatchSymbols allows manual override of watched symbols
func (g *GinieAnalyzer) SetWatchSymbols(symbols []string) {
	g.watchSymbols = symbols
	if g.logger != nil {
		g.logger.Info("Ginie watch symbols updated", "count", len(symbols))
	}
}

// GetWatchSymbols returns the current watched symbols
func (g *GinieAnalyzer) GetWatchSymbols() []string {
	return g.watchSymbols
}

// GetLLMSelectedCoins returns the cached LLM-selected coins
func (g *GinieAnalyzer) GetLLMSelectedCoins() []string {
	if len(g.llmCoinsCache) > 0 {
		return g.llmCoinsCache
	}
	return g.watchSymbols // Fallback to watch symbols if no LLM cache
}

// MarketMoverCategory represents different types of market movers
type MarketMoverCategory struct {
	TopGainers    []string // Highest 24h % gain
	TopLosers     []string // Highest 24h % loss
	TopVolume     []string // Highest 24h volume
	HighVolatility []string // High price movement + volume
}

// GetMarketMovers fetches dynamic market movers from Binance 24hr ticker data
func (g *GinieAnalyzer) GetMarketMovers(topN int) (*MarketMoverCategory, error) {
	if topN <= 0 {
		topN = 20
	}

	// Get all 24hr tickers
	tickers, err := g.futuresClient.GetAll24hrTickers()
	if err != nil {
		return nil, fmt.Errorf("failed to get 24hr tickers: %w", err)
	}

	// Filter for USDT pairs only and exclude stablecoins
	var validTickers []binance.Futures24hrTicker
	stablecoins := map[string]bool{
		"USDCUSDT": true, "BUSDUSDT": true, "TUSDUSDT": true,
		"DAIUSDT": true, "FDUSDUSDT": true, "EURUSDT": true,
	}

	for _, t := range tickers {
		if strings.HasSuffix(t.Symbol, "USDT") && !stablecoins[t.Symbol] {
			// Filter out very low volume coins (less than $1M daily volume)
			if t.QuoteVolume > 1000000 {
				validTickers = append(validTickers, t)
			}
		}
	}

	result := &MarketMoverCategory{
		TopGainers:     make([]string, 0, topN),
		TopLosers:      make([]string, 0, topN),
		TopVolume:      make([]string, 0, topN),
		HighVolatility: make([]string, 0, topN),
	}

	// Sort by price change % (gainers - descending)
	sort.Slice(validTickers, func(i, j int) bool {
		return validTickers[i].PriceChangePercent > validTickers[j].PriceChangePercent
	})
	for i := 0; i < topN && i < len(validTickers); i++ {
		result.TopGainers = append(result.TopGainers, validTickers[i].Symbol)
	}

	// Sort by price change % (losers - ascending)
	sort.Slice(validTickers, func(i, j int) bool {
		return validTickers[i].PriceChangePercent < validTickers[j].PriceChangePercent
	})
	for i := 0; i < topN && i < len(validTickers); i++ {
		result.TopLosers = append(result.TopLosers, validTickers[i].Symbol)
	}

	// Sort by 24hr quote volume (descending)
	sort.Slice(validTickers, func(i, j int) bool {
		return validTickers[i].QuoteVolume > validTickers[j].QuoteVolume
	})
	for i := 0; i < topN && i < len(validTickers); i++ {
		result.TopVolume = append(result.TopVolume, validTickers[i].Symbol)
	}

	// High volatility = high absolute price change % + high volume
	// Score = abs(priceChange%) * log(volume)
	sort.Slice(validTickers, func(i, j int) bool {
		scoreI := math.Abs(validTickers[i].PriceChangePercent) * math.Log10(validTickers[i].QuoteVolume+1)
		scoreJ := math.Abs(validTickers[j].PriceChangePercent) * math.Log10(validTickers[j].QuoteVolume+1)
		return scoreI > scoreJ
	})
	for i := 0; i < topN && i < len(validTickers); i++ {
		result.HighVolatility = append(result.HighVolatility, validTickers[i].Symbol)
	}

	if g.logger != nil {
		g.logger.Info("Fetched market movers", map[string]interface{}{
			"gainers":    len(result.TopGainers),
			"losers":     len(result.TopLosers),
			"volume":     len(result.TopVolume),
			"volatility": len(result.HighVolatility),
		})
	}

	return result, nil
}

// LoadDynamicSymbols loads symbols based on market movers (gainers, losers, volume, volatility)
func (g *GinieAnalyzer) LoadDynamicSymbols(topN int) error {
	movers, err := g.GetMarketMovers(topN)
	if err != nil {
		return err
	}

	// Combine all categories into a unique set
	symbolSet := make(map[string]bool)

	// Always include core coins
	coreCoin := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT"}
	for _, s := range coreCoin {
		symbolSet[s] = true
	}

	// Add all market movers
	for _, s := range movers.TopGainers {
		symbolSet[s] = true
	}
	for _, s := range movers.TopLosers {
		symbolSet[s] = true
	}
	for _, s := range movers.TopVolume {
		symbolSet[s] = true
	}
	for _, s := range movers.HighVolatility {
		symbolSet[s] = true
	}

	// Convert to slice
	symbols := make([]string, 0, len(symbolSet))
	for s := range symbolSet {
		symbols = append(symbols, s)
	}

	// Sort alphabetically for consistency
	sort.Strings(symbols)

	g.watchSymbols = symbols

	if g.logger != nil {
		g.logger.Info("Loaded dynamic market mover symbols", map[string]interface{}{
			"total_symbols": len(symbols),
			"top_gainers":   movers.TopGainers[:min(5, len(movers.TopGainers))],
			"top_losers":    movers.TopLosers[:min(5, len(movers.TopLosers))],
		})
	}

	return nil
}

// SetConfig updates the configuration
func (g *GinieAnalyzer) SetConfig(config *GinieConfig) {
	g.config = config
}

// GetConfig returns current configuration
func (g *GinieAnalyzer) GetConfig() *GinieConfig {
	return g.config
}

// Enable enables the Ginie analyzer
func (g *GinieAnalyzer) Enable() {
	g.enabled = true
}

// Disable disables the Ginie analyzer
func (g *GinieAnalyzer) Disable() {
	g.enabled = false
}

// IsEnabled returns if Ginie is enabled
func (g *GinieAnalyzer) IsEnabled() bool {
	return g.enabled
}

// GetStatus returns current Ginie status
// NOTE: This function copies data under locks and releases them before returning
// to avoid deadlock when callers subsequently call ScanCoin() or GenerateDecision()
// which need write locks on the same mutexes.
func (g *GinieAnalyzer) GetStatus() *GinieStatus {
	// Copy scan-related data under scanLock
	g.scanLock.RLock()
	lastScanTime := g.lastScanTime
	scannedSymbols := len(g.coinScans)
	// Make a copy of watchSymbols slice to avoid data race
	watchSymbols := make([]string, len(g.watchSymbols))
	copy(watchSymbols, g.watchSymbols)
	g.scanLock.RUnlock()

	// Copy decision-related data under decisionLock
	g.decisionLock.RLock()
	// Get recent decisions (last 10) - copy while holding lock
	recentDecisions := make([]GinieDecisionReport, 0)
	start := len(g.decisions) - 10
	if start < 0 {
		start = 0
	}
	for i := start; i < len(g.decisions); i++ {
		recentDecisions = append(recentDecisions, g.decisions[i])
	}
	g.decisionLock.RUnlock()

	// These fields are either atomic or don't require locking
	winRate := 0.0
	total := g.wins + g.losses
	if total > 0 {
		winRate = float64(g.wins) / float64(total) * 100
	}

	maxPos := g.config.MaxSwingPositions
	switch g.activeMode {
	case GinieModeScalp:
		maxPos = g.config.MaxScalpPositions
	case GinieModePosition:
		maxPos = g.config.MaxPositionPositions
	}

	return &GinieStatus{
		Enabled:          g.enabled,
		ActiveMode:       g.activeMode,
		ActivePositions:  0, // Will be updated from controller
		MaxPositions:     maxPos,
		LastScanTime:     lastScanTime,
		LastDecisionTime: time.Now(),
		DailyPnL:         g.dailyPnL,
		DailyTrades:      g.dailyTrades,
		WinRate:          winRate,
		Config:           g.config,
		RecentDecisions:  recentDecisions,
		WatchedSymbols:   watchSymbols,
		ScannedSymbols:   scannedSymbols,
	}
}

// ScanCoin performs the pre-trade coin scan
func (g *GinieAnalyzer) ScanCoin(symbol string) (*GinieCoinScan, error) {
	if g.logger != nil {
		g.logger.Info("Ginie scanning coin", "symbol", symbol)
	}

	scan := &GinieCoinScan{
		Symbol:    symbol,
		Timestamp: time.Now(),
	}

	// Get klines for analysis
	klines, err := g.futuresClient.GetFuturesKlines(symbol, "1h", 200)
	if err != nil {
		return nil, fmt.Errorf("failed to get klines: %w", err)
	}

	if len(klines) < 50 {
		scan.Status = ScanStatusAvoid
		scan.TradeReady = false
		scan.Reason = "Insufficient data"
		return scan, nil
	}

	// Get 24h ticker for volume
	ticker, err := g.futuresClient.Get24hrTicker(symbol)
	if err != nil {
		// Continue without ticker data
		ticker = &binance.Futures24hrTicker{}
	}

	// Get current price
	price := klines[len(klines)-1].Close

	// 1. Liquidity Check
	scan.Liquidity = g.checkLiquidity(ticker, price)

	// 2. Volatility Profile
	scan.Volatility = g.analyzeVolatility(klines)

	// Add 24h volatility from ticker for mode selection
	if ticker.PriceChangePercent != 0 {
		// Absolute value of 24h price change
		priceChange := ticker.PriceChangePercent
		if priceChange < 0 {
			priceChange = -priceChange
		}
		scan.Volatility.PriceChange24h = priceChange

		// Calculate 24h high-low range
		if ticker.LowPrice > 0 {
			scan.Volatility.HighLowRange24h = ((ticker.HighPrice - ticker.LowPrice) / ticker.LowPrice) * 100
		}
	}

	// 3. Trend Health
	scan.Trend = g.analyzeTrend(klines, "1h")

	// 4. Market Structure
	scan.Structure = g.analyzeStructure(klines)

	// 5. Correlation Check (simplified - would need BTC data)
	scan.Correlation = g.analyzeCorrelation(symbol)

	// 6. Price Action Analysis (FVG and Order Blocks)
	scan.PriceAction = g.analyzePriceAction(klines, price, "")

	// Calculate overall score and determine status
	g.calculateScanScore(scan)

	// Cache the scan
	g.scanLock.Lock()
	g.coinScans[symbol] = scan
	g.lastScanTime = time.Now()
	g.scanLock.Unlock()

	return scan, nil
}

// checkLiquidity assesses liquidity
func (g *GinieAnalyzer) checkLiquidity(ticker *binance.Futures24hrTicker, price float64) LiquidityCheck {
	liq := LiquidityCheck{
		Volume24h:   ticker.Volume,
		VolumeUSD:   ticker.QuoteVolume,
		SlippageRisk: "medium",
	}

	// Calculate spread if we have bid/ask
	if ticker.LastPrice > 0 {
		// Estimate spread from price movement
		liq.SpreadPercent = math.Abs(ticker.PriceChangePercent) * 0.01
		if liq.SpreadPercent < 0.1 {
			liq.SpreadPercent = 0.05
		}
	}

	// Score liquidity
	score := 0.0
	if liq.VolumeUSD >= 5000000 {
		score += 40
		liq.PassedScalp = true
		liq.PassedSwing = true
	} else if liq.VolumeUSD >= 1000000 {
		score += 25
		liq.PassedSwing = true
	} else if liq.VolumeUSD >= 500000 {
		score += 15
	}

	if liq.SpreadPercent <= 0.05 {
		score += 30
		liq.SlippageRisk = "low"
	} else if liq.SpreadPercent <= 0.1 {
		score += 20
		liq.SlippageRisk = "medium"
	} else {
		score += 5
		liq.SlippageRisk = "high"
	}

	liq.LiquidityScore = score
	return liq
}

// analyzeVolatility analyzes volatility profile
func (g *GinieAnalyzer) analyzeVolatility(klines []binance.Kline) VolatilityProfile {
	vol := VolatilityProfile{}

	if len(klines) < 20 {
		return vol
	}

	// Calculate ATR(14)
	atr14 := g.calculateATR(klines, 14)
	atr20Avg := g.calculateATR(klines, 20)

	currentPrice := klines[len(klines)-1].Close
	vol.ATR14 = atr14
	vol.ATRPercent = (atr14 / currentPrice) * 100
	vol.AvgATR20 = atr20Avg
	if atr20Avg > 0 {
		vol.ATRRatio = atr14 / atr20Avg
	}

	// Bollinger Band Width
	sma, upper, lower := g.calculateBollingerBands(klines, 20, 2)
	if sma > 0 {
		vol.BBWidth = upper - lower
		vol.BBWidthPercent = (vol.BBWidth / sma) * 100
	}

	// Classify volatility regime
	if vol.ATRRatio >= 2.0 {
		vol.Regime = "Extreme"
		vol.VolatilityScore = 30 // High volatility = lower score for swing
	} else if vol.ATRRatio >= 1.5 {
		vol.Regime = "High"
		vol.VolatilityScore = 50
	} else if vol.ATRRatio >= 0.8 {
		vol.Regime = "Medium"
		vol.VolatilityScore = 80
	} else {
		vol.Regime = "Low"
		vol.VolatilityScore = 70
	}

	return vol
}

// analyzeTrend analyzes trend health
func (g *GinieAnalyzer) analyzeTrend(klines []binance.Kline, timeframe string) TrendHealth {
	trend := TrendHealth{
		Timeframe: timeframe,
	}

	if len(klines) < 50 {
		return trend
	}

	// Calculate ADX
	adx, plusDI, minusDI := g.calculateADX(klines, 14)
	trend.ADXValue = adx

	// Classify ADX strength
	if adx < 20 {
		trend.ADXStrength = "weak"
		trend.IsRanging = true
	} else if adx < 30 {
		trend.ADXStrength = "moderate"
		trend.IsTrending = true
	} else if adx < 50 {
		trend.ADXStrength = "strong"
		trend.IsTrending = true
	} else {
		trend.ADXStrength = "very_strong"
		trend.IsTrending = true
	}

	// Determine direction
	if plusDI > minusDI {
		trend.TrendDirection = "bullish"
	} else if minusDI > plusDI {
		trend.TrendDirection = "bearish"
	} else {
		trend.TrendDirection = "neutral"
	}

	// Calculate EMA distances
	currentPrice := klines[len(klines)-1].Close
	ema20 := g.calculateEMA(klines, 20)
	ema50 := g.calculateEMA(klines, 50)
	ema200 := g.calculateEMA(klines, 200)

	if ema20 > 0 {
		trend.EMA20Distance = ((currentPrice - ema20) / ema20) * 100
	}
	if ema50 > 0 {
		trend.EMA50Distance = ((currentPrice - ema50) / ema50) * 100
	}
	if ema200 > 0 {
		trend.EMA200Distance = ((currentPrice - ema200) / ema200) * 100
	}

	// Multi-timeframe alignment check (simplified)
	trend.MTFAlignment = (trend.EMA20Distance > 0 && trend.EMA50Distance > 0) ||
		(trend.EMA20Distance < 0 && trend.EMA50Distance < 0)

	// Score trend
	trend.TrendScore = 0
	if trend.IsTrending {
		trend.TrendScore += 40
	}
	if trend.MTFAlignment {
		trend.TrendScore += 30
	}
	if adx > 25 {
		trend.TrendScore += 20
	}

	return trend
}

// analyzeStructure analyzes market structure
func (g *GinieAnalyzer) analyzeStructure(klines []binance.Kline) MarketStructure {
	structure := MarketStructure{}

	if len(klines) < 30 {
		return structure
	}

	// Find swing highs and lows
	highs, lows := g.findSwingPoints(klines, 5)

	if len(highs) >= 2 && len(lows) >= 2 {
		// Check for HH/HL or LH/LL pattern
		lastHigh := highs[len(highs)-1]
		prevHigh := highs[len(highs)-2]
		lastLow := lows[len(lows)-1]
		prevLow := lows[len(lows)-2]

		if lastHigh > prevHigh && lastLow > prevLow {
			structure.Pattern = "HH/HL" // Uptrend
		} else if lastHigh < prevHigh && lastLow < prevLow {
			structure.Pattern = "LH/LL" // Downtrend
		} else {
			structure.Pattern = "ranging"
		}
	}

	currentPrice := klines[len(klines)-1].Close

	// Set key levels
	if len(highs) >= 3 {
		structure.KeyResistances = highs[len(highs)-3:]
		structure.NearestResistance = findNearestAbove(currentPrice, highs)
	}
	if len(lows) >= 3 {
		structure.KeySupports = lows[len(lows)-3:]
		structure.NearestSupport = findNearestBelow(currentPrice, lows)
	}

	// Calculate breakout potential
	if structure.NearestResistance > 0 {
		structure.BreakoutPotential = ((structure.NearestResistance - currentPrice) / currentPrice) * 100
	}
	if structure.NearestSupport > 0 {
		structure.BreakdownPotential = ((currentPrice - structure.NearestSupport) / currentPrice) * 100
	}

	// Score structure
	structure.StructureScore = 50
	if structure.Pattern == "HH/HL" || structure.Pattern == "LH/LL" {
		structure.StructureScore = 80
	}

	return structure
}

// analyzeCorrelation checks correlation with BTC/ETH
func (g *GinieAnalyzer) analyzeCorrelation(symbol string) CorrelationCheck {
	corr := CorrelationCheck{
		BTCCorrelation:    0.7,  // Default high correlation
		ETHCorrelation:    0.6,
		IndependentCapable: false,
		CorrelationScore:   50,
	}

	// Major coins have high correlation
	switch symbol {
	case "BTCUSDT":
		corr.BTCCorrelation = 1.0
		corr.ETHCorrelation = 0.9
		corr.IndependentCapable = true
	case "ETHUSDT":
		corr.BTCCorrelation = 0.9
		corr.ETHCorrelation = 1.0
		corr.IndependentCapable = true
	case "SOLUSDT", "AVAXUSDT":
		corr.BTCCorrelation = 0.8
		corr.ETHCorrelation = 0.85
		corr.IndependentCapable = true
	default:
		corr.IndependentCapable = false
	}

	return corr
}

// calculateScanScore calculates overall scan score and status
func (g *GinieAnalyzer) calculateScanScore(scan *GinieCoinScan) {
	// Weight the scores
	score := scan.Liquidity.LiquidityScore*0.25 +
		scan.Volatility.VolatilityScore*0.2 +
		scan.Trend.TrendScore*0.3 +
		scan.Structure.StructureScore*0.15 +
		scan.Correlation.CorrelationScore*0.1

	scan.Score = score

	// Determine status based on conditions
	if !scan.Liquidity.PassedSwing {
		scan.Status = ScanStatusAvoid
		scan.TradeReady = false
		scan.Reason = "Insufficient liquidity"
		return
	}

	// Determine best mode using 24h volatility-based routing
	// Priority: Volatility-based selection first, then ADX refinement
	volatility24h := scan.Volatility.HighLowRange24h // Use high-low range for better volatility measure
	if volatility24h == 0 {
		volatility24h = scan.Volatility.PriceChange24h // Fallback to price change
	}

	// === VOLATILITY-BASED MODE SELECTION ===
	// Ultra-fast: High volatility coins (30%+ 24h range) - quick in/out trades
	// Scalp: Moderate volatility (10-30% 24h range) - short-term momentum
	// Swing: Low-moderate volatility (5-15% 24h range) - trend following
	// Position: Stable coins (<10% 24h range) - long-term holds

	if volatility24h >= 30 && scan.Liquidity.PassedScalp {
		// High volatility = Ultra-fast mode (quick scalps on volatile moves)
		scan.Status = ScanStatusUltraFastReady
		scan.TradeReady = true
		scan.Reason = fmt.Sprintf("High volatility (%.1f%% 24h) - ideal for ultra-fast scalping", volatility24h)
	} else if volatility24h >= 10 && volatility24h < 30 && scan.Liquidity.PassedScalp {
		// Moderate volatility = Scalp mode
		scan.Status = ScanStatusScalpReady
		scan.TradeReady = true
		scan.Reason = fmt.Sprintf("Moderate volatility (%.1f%% 24h) - ideal for scalping", volatility24h)
	} else if volatility24h < 10 && scan.Trend.IsTrending && scan.Trend.ADXValue > 35 && scan.Trend.MTFAlignment {
		// Stable + strong trend + MTF alignment = Position mode
		scan.Status = ScanStatusPositionReady
		scan.TradeReady = true
		scan.Reason = fmt.Sprintf("Stable coin (%.1f%% 24h) with strong trend - ideal for position trading", volatility24h)
	} else if volatility24h >= 5 && volatility24h < 15 && scan.Trend.IsTrending && scan.Trend.ADXValue >= 25 {
		// Low-moderate volatility with trend = Swing mode
		scan.Status = ScanStatusSwingReady
		scan.TradeReady = true
		scan.Reason = fmt.Sprintf("Low volatility (%.1f%% 24h) with trend - ideal for swing trading", volatility24h)
	} else if scan.Volatility.Regime == "Extreme" || score < 40 {
		// Extreme conditions
		scan.Status = ScanStatusHedgeRequired
		scan.TradeReady = true
		scan.Reason = "High risk environment - hedge recommended"
	} else if score < 30 {
		scan.Status = ScanStatusAvoid
		scan.TradeReady = false
		scan.Reason = "Poor trading conditions"
	} else {
		// Default to swing for anything else that passes liquidity
		scan.Status = ScanStatusSwingReady
		scan.TradeReady = true
		scan.Reason = fmt.Sprintf("Acceptable conditions (%.1f%% 24h volatility) - swing trading", volatility24h)
	}
}

// SelectMode determines the best trading mode based on 24h volatility scan
func (g *GinieAnalyzer) SelectMode(scan *GinieCoinScan) GinieTradingMode {
	// Auto-select based on scan results (volatility-based)
	switch scan.Status {
	case ScanStatusUltraFastReady:
		return GinieModeUltraFast
	case ScanStatusScalpReady:
		return GinieModeScalp
	case ScanStatusPositionReady:
		return GinieModePosition
	case ScanStatusSwingReady:
		return GinieModeSwing
	default:
		return GinieModeSwing
	}
}

// GenerateSignals generates signals for the selected mode
func (g *GinieAnalyzer) GenerateSignals(symbol string, mode GinieTradingMode, klines []binance.Kline) *GinieSignalSet {
	signalSet := &GinieSignalSet{
		Mode:            mode,
		PrimarySignals:  make([]GinieSignal, 0),
		SecondarySignals: make([]GinieSignal, 0),
	}

	if len(klines) < 50 {
		return signalSet
	}

	currentPrice := klines[len(klines)-1].Close

	switch mode {
	case GinieModeUltraFast:
		// Ultra-fast uses faster signals for high volatility coins
		signalSet.PrimaryTimeframe = "1m"
		signalSet.ConfirmTimeframe = "5m"
		signalSet.PrimaryRequired = 2 // Lower requirement for quick entries
		g.generateScalpSignals(signalSet, klines, currentPrice) // Reuse scalp signals
	case GinieModeScalp:
		signalSet.PrimaryTimeframe = "1m"
		signalSet.ConfirmTimeframe = "15m"
		signalSet.PrimaryRequired = 3
		g.generateScalpSignals(signalSet, klines, currentPrice)
	case GinieModeSwing:
		signalSet.PrimaryTimeframe = "4h"
		signalSet.ConfirmTimeframe = "1d"
		signalSet.PrimaryRequired = 4
		g.generateSwingSignals(signalSet, klines, currentPrice)
	case GinieModePosition:
		signalSet.PrimaryTimeframe = "1w"
		signalSet.ConfirmTimeframe = "1m"
		signalSet.PrimaryRequired = 4
		g.generatePositionSignals(signalSet, klines, currentPrice)
	}

	// Count met signals
	for _, sig := range signalSet.PrimarySignals {
		if sig.Met {
			signalSet.PrimaryMet++
		}
	}
	for _, sig := range signalSet.SecondarySignals {
		if sig.Met {
			signalSet.SecondaryMet++
		}
	}

	signalSet.PrimaryPassed = signalSet.PrimaryMet >= signalSet.PrimaryRequired

	// Determine direction and strength
	longScore := 0.0
	shortScore := 0.0
	for _, sig := range signalSet.PrimarySignals {
		if sig.Met {
			if sig.Value > 0 {
				longScore += sig.Weight
			} else {
				shortScore += sig.Weight
			}
		}
	}

	if longScore > shortScore {
		signalSet.Direction = "long"
	} else if shortScore > longScore {
		signalSet.Direction = "short"
	} else {
		signalSet.Direction = "neutral"
	}

	// Signal strength
	totalWeight := 0.0
	metWeight := 0.0
	for _, sig := range signalSet.PrimarySignals {
		totalWeight += sig.Weight
		if sig.Met {
			metWeight += sig.Weight
		}
	}

	if totalWeight > 0 {
		signalSet.StrengthScore = (metWeight / totalWeight) * 100
	}

	if signalSet.StrengthScore >= 80 {
		signalSet.SignalStrength = "Very Strong"
	} else if signalSet.StrengthScore >= 60 {
		signalSet.SignalStrength = "Strong"
	} else if signalSet.StrengthScore >= 40 {
		signalSet.SignalStrength = "Moderate"
	} else {
		signalSet.SignalStrength = "Weak"
	}

	return signalSet
}

// generateScalpSignals generates scalping signals
func (g *GinieAnalyzer) generateScalpSignals(ss *GinieSignalSet, klines []binance.Kline, price float64) {
	rsi7 := g.calculateRSI(klines, 7)
	stochRSI := g.calculateStochRSI(klines, 14, 3, 3)
	ema9 := g.calculateEMA(klines, 9)
	ema21 := g.calculateEMA(klines, 21)

	// RSI Signal
	rsiSignal := GinieSignal{
		Name:      "RSI(7) Crossover",
		Weight:    0.3,
		Value:     rsi7,
		Threshold: 30,
	}
	if rsi7 < 30 {
		rsiSignal.Met = true
		rsiSignal.Status = "met"
		rsiSignal.Description = "RSI oversold - potential long"
		rsiSignal.Value = 1
	} else if rsi7 > 70 {
		rsiSignal.Met = true
		rsiSignal.Status = "met"
		rsiSignal.Description = "RSI overbought - potential short"
		rsiSignal.Value = -1
	} else {
		rsiSignal.Status = "not_met"
		rsiSignal.Description = "RSI neutral"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, rsiSignal)

	// Stochastic RSI Signal
	stochSignal := GinieSignal{
		Name:      "Stochastic RSI Cross",
		Weight:    0.25,
		Value:     stochRSI,
		Threshold: 20,
	}
	if stochRSI < 20 {
		stochSignal.Met = true
		stochSignal.Status = "met"
		stochSignal.Description = "StochRSI oversold zone"
		stochSignal.Value = 1
	} else if stochRSI > 80 {
		stochSignal.Met = true
		stochSignal.Status = "met"
		stochSignal.Description = "StochRSI overbought zone"
		stochSignal.Value = -1
	} else {
		stochSignal.Status = "not_met"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, stochSignal)

	// EMA Signal
	emaSignal := GinieSignal{
		Name:      "EMA 9/21 Position",
		Weight:    0.25,
		Threshold: 0,
	}
	if ema9 > ema21 && price > ema9 {
		emaSignal.Met = true
		emaSignal.Status = "met"
		emaSignal.Description = "Price above rising EMAs"
		emaSignal.Value = 1
	} else if ema9 < ema21 && price < ema9 {
		emaSignal.Met = true
		emaSignal.Status = "met"
		emaSignal.Description = "Price below falling EMAs"
		emaSignal.Value = -1
	} else {
		emaSignal.Status = "not_met"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, emaSignal)

	// Volume Signal (simplified)
	volSignal := GinieSignal{
		Name:      "Volume Confirmation",
		Weight:    0.2,
		Threshold: 1.0,
	}
	if len(klines) > 5 {
		lastVol := klines[len(klines)-1].Volume
		avgVol := 0.0
		for i := len(klines) - 6; i < len(klines)-1; i++ {
			avgVol += klines[i].Volume
		}
		avgVol /= 5
		volRatio := lastVol / avgVol
		volSignal.Value = volRatio
		if volRatio > 1.0 {
			volSignal.Met = true
			volSignal.Status = "met"
			volSignal.Description = fmt.Sprintf("Volume %.1fx average", volRatio)
		} else {
			volSignal.Status = "not_met"
		}
	}
	ss.PrimarySignals = append(ss.PrimarySignals, volSignal)
}

// generateSwingSignals generates swing trading signals
func (g *GinieAnalyzer) generateSwingSignals(ss *GinieSignalSet, klines []binance.Kline, price float64) {
	ema50 := g.calculateEMA(klines, 50)
	rsi14 := g.calculateRSI(klines, 14)
	macd, signal, _ := g.calculateMACD(klines)
	adx, plusDI, minusDI := g.calculateADX(klines, 14)

	// EMA 50 Position
	emaSignal := GinieSignal{
		Name:      "Price vs EMA 50",
		Weight:    0.25,
		Threshold: 0,
	}
	if price > ema50 {
		emaSignal.Met = true
		emaSignal.Status = "met"
		emaSignal.Description = "Price above EMA 50"
		emaSignal.Value = 1
	} else {
		emaSignal.Met = true
		emaSignal.Status = "met"
		emaSignal.Description = "Price below EMA 50"
		emaSignal.Value = -1
	}
	ss.PrimarySignals = append(ss.PrimarySignals, emaSignal)

	// RSI 14 Signal
	rsiSignal := GinieSignal{
		Name:      "RSI(14) Trend",
		Weight:    0.2,
		Value:     rsi14,
		Threshold: 50,
	}
	if rsi14 > 50 && rsi14 < 70 {
		rsiSignal.Met = true
		rsiSignal.Status = "met"
		rsiSignal.Description = "RSI bullish zone"
		rsiSignal.Value = 1
	} else if rsi14 < 50 && rsi14 > 30 {
		rsiSignal.Met = true
		rsiSignal.Status = "met"
		rsiSignal.Description = "RSI bearish zone"
		rsiSignal.Value = -1
	} else {
		rsiSignal.Status = "not_met"
		rsiSignal.Description = "RSI extreme zone"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, rsiSignal)

	// MACD Signal
	macdSignal := GinieSignal{
		Name:      "MACD Cross",
		Weight:    0.2,
		Threshold: 0,
	}
	if macd > signal {
		macdSignal.Met = true
		macdSignal.Status = "met"
		macdSignal.Description = "MACD above signal line"
		macdSignal.Value = 1
	} else {
		macdSignal.Met = true
		macdSignal.Status = "met"
		macdSignal.Description = "MACD below signal line"
		macdSignal.Value = -1
	}
	ss.PrimarySignals = append(ss.PrimarySignals, macdSignal)

	// ADX/DMI Signal
	adxSignal := GinieSignal{
		Name:      "ADX/DMI Trend",
		Weight:    0.25,
		Value:     adx,
		Threshold: 30,
	}
	if adx > 30 {
		adxSignal.Met = true
		if plusDI > minusDI {
			adxSignal.Status = "met"
			adxSignal.Description = fmt.Sprintf("ADX %.0f with bullish DMI", adx)
			adxSignal.Value = 1
		} else {
			adxSignal.Status = "met"
			adxSignal.Description = fmt.Sprintf("ADX %.0f with bearish DMI", adx)
			adxSignal.Value = -1
		}
	} else {
		adxSignal.Status = "not_met"
		adxSignal.Description = "ADX too weak for trending"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, adxSignal)

	// Volume confirmation
	volSignal := GinieSignal{
		Name:      "Volume Profile",
		Weight:    0.15,
		Threshold: 1.0,
	}
	if len(klines) > 10 {
		lastVol := klines[len(klines)-1].Volume
		avgVol := 0.0
		for i := len(klines) - 11; i < len(klines)-1; i++ {
			avgVol += klines[i].Volume
		}
		avgVol /= 10
		if lastVol > avgVol {
			volSignal.Met = true
			volSignal.Status = "met"
			volSignal.Description = "Above average volume"
			volSignal.Value = 1
		} else {
			volSignal.Status = "not_met"
		}
	}
	ss.PrimarySignals = append(ss.PrimarySignals, volSignal)
}

// generatePositionSignals generates position trading signals
func (g *GinieAnalyzer) generatePositionSignals(ss *GinieSignalSet, klines []binance.Kline, price float64) {
	ema20 := g.calculateEMA(klines, 20)
	ema50 := g.calculateEMA(klines, 50)
	rsi14 := g.calculateRSI(klines, 14)
	macd, signal, _ := g.calculateMACD(klines)

	// Weekly EMA position
	emaSignal := GinieSignal{
		Name:      "Weekly EMA 20 Position",
		Weight:    0.3,
		Threshold: 0,
	}
	if price > ema20 && ema20 > ema50 {
		emaSignal.Met = true
		emaSignal.Status = "met"
		emaSignal.Description = "Price above rising EMA structure"
		emaSignal.Value = 1
	} else if price < ema20 && ema20 < ema50 {
		emaSignal.Met = true
		emaSignal.Status = "met"
		emaSignal.Description = "Price below falling EMA structure"
		emaSignal.Value = -1
	} else {
		emaSignal.Status = "partial"
		emaSignal.Description = "Mixed EMA signals"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, emaSignal)

	// Monthly RSI
	rsiSignal := GinieSignal{
		Name:      "Monthly RSI Trend",
		Weight:    0.25,
		Value:     rsi14,
		Threshold: 50,
	}
	if rsi14 > 50 {
		rsiSignal.Met = true
		rsiSignal.Status = "met"
		rsiSignal.Description = "Monthly RSI bullish"
		rsiSignal.Value = 1
	} else {
		rsiSignal.Met = true
		rsiSignal.Status = "met"
		rsiSignal.Description = "Monthly RSI bearish"
		rsiSignal.Value = -1
	}
	ss.PrimarySignals = append(ss.PrimarySignals, rsiSignal)

	// Weekly MACD
	macdSignal := GinieSignal{
		Name:      "Weekly MACD",
		Weight:    0.25,
		Threshold: 0,
	}
	if macd > signal && macd > 0 {
		macdSignal.Met = true
		macdSignal.Status = "met"
		macdSignal.Description = "MACD bullish expansion"
		macdSignal.Value = 1
	} else if macd < signal && macd < 0 {
		macdSignal.Met = true
		macdSignal.Status = "met"
		macdSignal.Description = "MACD bearish expansion"
		macdSignal.Value = -1
	} else {
		macdSignal.Status = "partial"
	}
	ss.PrimarySignals = append(ss.PrimarySignals, macdSignal)

	// Trend structure
	structSignal := GinieSignal{
		Name:      "Macro Trend Structure",
		Weight:    0.2,
		Threshold: 0,
	}
	// Check for HH/HL pattern
	if len(klines) > 30 {
		highs, lows := g.findSwingPoints(klines, 10)
		if len(highs) >= 2 && len(lows) >= 2 {
			if highs[len(highs)-1] > highs[len(highs)-2] && lows[len(lows)-1] > lows[len(lows)-2] {
				structSignal.Met = true
				structSignal.Status = "met"
				structSignal.Description = "Higher highs and higher lows"
				structSignal.Value = 1
			} else if highs[len(highs)-1] < highs[len(highs)-2] && lows[len(lows)-1] < lows[len(lows)-2] {
				structSignal.Met = true
				structSignal.Status = "met"
				structSignal.Description = "Lower highs and lower lows"
				structSignal.Value = -1
			}
		}
	}
	ss.PrimarySignals = append(ss.PrimarySignals, structSignal)
}

// GenerateDecision generates a trading decision for a symbol (auto-selects mode based on market conditions)
// This is the legacy function - use GenerateDecisionForMode for explicit mode control
func (g *GinieAnalyzer) GenerateDecision(symbol string) (*GinieDecisionReport, error) {
	// Scan the coin
	scan, err := g.ScanCoin(symbol)
	if err != nil {
		return nil, err
	}

	// Auto-select mode based on market conditions
	mode := g.SelectMode(scan)

	// Delegate to mode-specific function
	return g.generateDecisionInternal(symbol, mode, scan)
}

// GenerateDecisionForMode generates a trading decision for a symbol using a specific mode
// This allows scanForMode to check each mode independently without auto-selection
func (g *GinieAnalyzer) GenerateDecisionForMode(symbol string, mode GinieTradingMode) (*GinieDecisionReport, error) {
	// Scan the coin
	scan, err := g.ScanCoin(symbol)
	if err != nil {
		return nil, err
	}

	// Use the explicitly provided mode instead of auto-selecting
	return g.generateDecisionInternal(symbol, mode, scan)
}

// generateDecisionInternal is the core decision generation logic used by both GenerateDecision and GenerateDecisionForMode
func (g *GinieAnalyzer) generateDecisionInternal(symbol string, mode GinieTradingMode, scan *GinieCoinScan) (*GinieDecisionReport, error) {
	// Initialize rejection tracker to capture all rejection reasons
	rejectionTracker := NewRejectionTracker()

	// Get klines for signal generation (always 1h)
	klines, err := g.futuresClient.GetFuturesKlines(symbol, "1h", 200)
	if err != nil {
		return nil, err
	}

	// Determine target trend timeframe based on mode using ModeConfigs
	modeToConfigKey := map[string]string{
		string(GinieModeScalp):    "scalp",
		string(GinieModeSwing):    "swing",
		string(GinieModePosition): "position",
	}
	modeDefaults := map[string]string{
		"scalp":    "15m",
		"swing":    "1h",
		"position": "4h",
	}

	var targetTimeframe string
	modeKey, ok := modeToConfigKey[string(mode)]
	if !ok {
		targetTimeframe = "1h" // Default fallback
	} else {
		targetTimeframe = modeDefaults[modeKey] // Mode-specific default
		if g.settings != nil {
			if modeConfig := g.settings.ModeConfigs[modeKey]; modeConfig != nil {
				if modeConfig.Timeframe != nil && modeConfig.Timeframe.TrendTimeframe != "" {
					targetTimeframe = modeConfig.Timeframe.TrendTimeframe
				}
			}
		}
	}

	// Fetch klines for target timeframe if different from scan
	var trendAnalysis TrendHealth = scan.Trend
	var divergence *TrendDivergence

	if targetTimeframe != scan.Trend.Timeframe {
		targetKlines, err := g.futuresClient.GetFuturesKlines(symbol, targetTimeframe, 200)
		if err != nil {
			if g.logger != nil {
				g.logger.Warn("Failed to fetch target timeframe klines, using scan trend",
					"symbol", symbol,
					"target_timeframe", targetTimeframe,
					"error", err.Error())
			}
			trendAnalysis = scan.Trend // Fallback
		} else if len(targetKlines) >= 50 {
			trendAnalysis = g.analyzeTrend(targetKlines, targetTimeframe)

			if g.logger != nil {
				g.logger.Debug("Using configurable trend timeframe",
					"symbol", symbol,
					"mode", mode,
					"scan_timeframe", scan.Trend.Timeframe,
					"target_timeframe", targetTimeframe,
					"trend", trendAnalysis.TrendDirection,
					"adx", trendAnalysis.ADXValue)
			}

			// Detect divergence - read from ModeConfigs
			blockOnDivergence := false
			if g.settings != nil && modeKey != "" {
				if modeConfig := g.settings.ModeConfigs[modeKey]; modeConfig != nil {
					if modeConfig.TrendDivergence != nil {
						blockOnDivergence = modeConfig.TrendDivergence.BlockOnDivergence
					}
				}
			}
			divergence = g.DetectTrendDivergence(scan.Trend, trendAnalysis, blockOnDivergence)

			if divergence.Detected && g.logger != nil {
				g.logger.Warn("Trend divergence detected",
					"symbol", symbol,
					"severity", divergence.Severity,
					"reason", divergence.Reason,
					"should_block", divergence.ShouldBlock)
			}
		}
	}

	// Generate signals
	signals := g.GenerateSignals(symbol, mode, klines)

	currentPrice := klines[len(klines)-1].Close

	// Build decision report
	report := &GinieDecisionReport{
		Symbol:            symbol,
		Timestamp:         time.Now(),
		ScanStatus:        scan.Status,
		SelectedMode:      mode,
		TrendDivergence:   divergence,
		RejectionTracking: rejectionTracker,
	}

	// Market conditions
	report.MarketConditions.Trend = trendAnalysis.TrendDirection
	report.MarketConditions.ADX = trendAnalysis.ADXValue
	report.MarketConditions.Volatility = scan.Volatility.Regime
	report.MarketConditions.ATR = scan.Volatility.ATR14
	if scan.Liquidity.VolumeUSD > 10000000 {
		report.MarketConditions.Volume = "Above Avg"
	} else if scan.Liquidity.VolumeUSD > 5000000 {
		report.MarketConditions.Volume = "Average"
	} else {
		report.MarketConditions.Volume = "Below Avg"
	}
	report.MarketConditions.BTCCorr = scan.Correlation.BTCCorrelation
	report.MarketConditions.Sentiment = "Neutral"
	report.MarketConditions.SentimentVal = 50

	// Signal analysis
	report.SignalAnalysis = *signals

	// CRITICAL: Block trade if divergence detected and blocking enabled
	if divergence != nil && divergence.ShouldBlock {
		// Track trend divergence rejection
		rejectionTracker.TrendDivergence = &TrendDivergenceRejection{
			Blocked:           true,
			ScanTimeframe:     divergence.ScanTimeframe,
			ScanTrend:         divergence.ScanTrend,
			DecisionTimeframe: divergence.DecisionTimeframe,
			DecisionTrend:     divergence.DecisionTrend,
			Severity:          divergence.Severity,
			Reason:            divergence.Reason,
		}
		rejectionTracker.AddRejection(fmt.Sprintf("Trend Divergence (%s): %s trend on %s vs %s trend on %s",
			divergence.Severity, divergence.ScanTrend, divergence.ScanTimeframe,
			divergence.DecisionTrend, divergence.DecisionTimeframe))

		report.Recommendation = RecommendationSkip
		report.RecommendationNote = fmt.Sprintf("BLOCKED: %s", divergence.Reason)
		report.ConfidenceScore = 0

		if g.logger != nil {
			g.logger.Info("Trade blocked due to trend divergence",
				"symbol", symbol,
				"severity", divergence.Severity,
				"reason", divergence.Reason)
		}

		return report, nil
	}

	// Track signal and scan quality issues (even if not blocking yet)
	if !signals.PrimaryPassed {
		// Collect failed signals
		failedSignals := []string{}
		for _, sig := range signals.PrimarySignals {
			if !sig.Met {
				failedSignals = append(failedSignals, sig.Name)
			}
		}
		rejectionTracker.SignalStrength = &SignalStrengthRejection{
			Blocked:         true,
			SignalsMet:      signals.PrimaryMet,
			SignalsRequired: signals.PrimaryRequired,
			FailedSignals:   failedSignals,
			Reason:          fmt.Sprintf("Insufficient signals: %d/%d met (need %d)", signals.PrimaryMet, len(signals.PrimarySignals), signals.PrimaryRequired),
		}
		rejectionTracker.AddRejection(fmt.Sprintf("Signals: %d/%d required (missing: %v)", signals.PrimaryMet, signals.PrimaryRequired, failedSignals))
	}

	if !scan.TradeReady {
		rejectionTracker.ScanQuality = &ScanQualityRejection{
			Blocked:    true,
			ScanScore:  scan.Score,
			MinScore:   30.0, // Minimum score threshold
			TradeReady: scan.TradeReady,
			ScanStatus: string(scan.Status),
			Reason:     fmt.Sprintf("Scan not ready: %s (score: %.1f)", scan.Reason, scan.Score),
		}
		rejectionTracker.AddRejection(fmt.Sprintf("Scan Quality: %s (score: %.1f)", scan.Reason, scan.Score))
	}

	// Track liquidity issues
	if !scan.Liquidity.PassedSwing {
		rejectionTracker.Liquidity = &LiquidityRejection{
			Blocked:        true,
			Volume24h:      scan.Liquidity.VolumeUSD,
			RequiredVolume: 1000000, // $1M for swing
			BidAskSpread:   scan.Liquidity.SpreadPercent,
			MaxSpread:      0.1,
			Reason:         fmt.Sprintf("Low liquidity: $%.0f volume (need $1M+)", scan.Liquidity.VolumeUSD),
		}
		rejectionTracker.AddRejection(fmt.Sprintf("Liquidity: $%.0f volume (need $1M+)", scan.Liquidity.VolumeUSD))
	}

	// === REVERSAL ENTRY CHECK (Scalp Mode Only) ===
	// For scalp mode, check for reversal patterns before regular signal-based entry
	var useReversalEntry bool
	var reversalDirection string
	var reversalEntryPrice float64

	if mode == GinieModeScalp && g.signalAggregator != nil {
		log.Printf("[REVERSAL-CHECK] %s: Starting reversal check for SCALP mode", symbol)

		// Analyze multi-timeframe reversal patterns (3 consecutive LL or HH)
		mtfReversal := g.AnalyzeMTFReversal(symbol, 3) // 3 consecutive candles

		if mtfReversal != nil && mtfReversal.Aligned && mtfReversal.AlignedCount >= 2 {
			log.Printf("[REVERSAL-CHECK] %s: MTF Reversal ALIGNED! direction=%s, alignedCount=%d, entryPrice=%.6f",
				symbol, mtfReversal.Direction, mtfReversal.AlignedCount, mtfReversal.EntryPrice)
			// Get LLM confirmation for reversal
			llmAnalyzer := g.signalAggregator.GetLLMAnalyzer()
			if llmAnalyzer != nil && llmAnalyzer.IsEnabled() {
				// Fetch klines for LLM analysis (already have some from scan)
				klines5m, _ := g.futuresClient.GetFuturesKlines(symbol, "5m", 30)
				klines15m, _ := g.futuresClient.GetFuturesKlines(symbol, "15m", 30)
				klines1h, _ := g.futuresClient.GetFuturesKlines(symbol, "1h", 30)

				patternType := "lower_lows"
				if mtfReversal.Direction == "SHORT" {
					patternType = "higher_highs"
				}

				llmConfirm, err := llmAnalyzer.AnalyzeReversalProbability(
					symbol,
					patternType,
					mtfReversal.Direction,
					mtfReversal.AlignedCount,
					klines5m,
					klines15m,
					klines1h,
				)

				if err == nil && llmConfirm != nil && llmConfirm.IsReversal && llmConfirm.Confidence >= 0.65 {
					// LLM confirmed reversal - use LIMIT entry
					useReversalEntry = true
					reversalDirection = mtfReversal.Direction
					reversalEntryPrice = mtfReversal.EntryPrice

					if g.logger != nil {
						g.logger.Info("Reversal entry confirmed by LLM",
							"symbol", symbol,
							"direction", reversalDirection,
							"entry_price", reversalEntryPrice,
							"llm_confidence", llmConfirm.Confidence,
							"pattern", patternType,
							"aligned_tfs", mtfReversal.AlignedCount)
					}
				} else if err != nil && g.logger != nil {
					g.logger.Warn("LLM reversal confirmation failed",
						"symbol", symbol,
						"error", err.Error())
				}
			} else if g.logger != nil {
				g.logger.Debug("Reversal pattern detected but LLM not available for confirmation",
					"symbol", symbol,
					"direction", mtfReversal.Direction,
					"aligned_tfs", mtfReversal.AlignedCount)
			}
		} else if mtfReversal != nil {
			log.Printf("[REVERSAL-CHECK] %s: MTF Reversal NOT aligned (alignedCount=%d, need 2+)",
				symbol, mtfReversal.AlignedCount)
		}

		log.Printf("[REVERSAL-CHECK] %s: Final result - useReversalEntry=%v", symbol, useReversalEntry)
	} else if mode != GinieModeScalp {
		log.Printf("[REVERSAL-CHECK] %s: Skipping reversal check (mode=%s, not scalp)", symbol, mode)
	}

	// Trade execution
	if signals.PrimaryPassed && scan.TradeReady {
		report.TradeExecution.Action = "LONG"
		if signals.Direction == "short" {
			report.TradeExecution.Action = "SHORT"
		}

		// Apply reversal entry override if detected and confirmed
		if useReversalEntry {
			report.TradeExecution.Action = reversalDirection
			report.TradeExecution.UseReversal = true
			report.TradeExecution.EntryType = "LIMIT"
			report.TradeExecution.LimitEntryPrice = reversalEntryPrice

			if g.logger != nil {
				g.logger.Info("Using reversal LIMIT entry",
					"symbol", symbol,
					"action", reversalDirection,
					"limit_price", reversalEntryPrice)
			}
		}

		// Entry zone based on ATR
		atrPct := scan.Volatility.ATRPercent
		if atrPct == 0 {
			atrPct = 1.0 // Fallback 1%
		}
		report.TradeExecution.EntryLow = currentPrice * (1 - atrPct/100*0.3)
		report.TradeExecution.EntryHigh = currentPrice * (1 + atrPct/100*0.1)

		// === ADAPTIVE SL/TP CALCULATION ===
		// Get LLM analysis for intelligent SL/TP
		var llmSLPct, llmTPPct float64
		var llmUsed bool
		if g.signalAggregator != nil {
			llmAnalysis := g.signalAggregator.GetCachedLLMAnalysis(symbol)
			if llmAnalysis != nil {
				// Extract LLM suggested SL/TP as percentages
				if llmAnalysis.StopLoss != nil && *llmAnalysis.StopLoss > 0 {
					if signals.Direction == "long" {
						llmSLPct = ((currentPrice - *llmAnalysis.StopLoss) / currentPrice) * 100
					} else {
						llmSLPct = ((*llmAnalysis.StopLoss - currentPrice) / currentPrice) * 100
					}
					if llmSLPct > 0 {
						llmUsed = true
					}
				}
				if llmAnalysis.TakeProfit != nil && *llmAnalysis.TakeProfit > 0 {
					if signals.Direction == "long" {
						llmTPPct = ((*llmAnalysis.TakeProfit - currentPrice) / currentPrice) * 100
					} else {
						llmTPPct = ((currentPrice - *llmAnalysis.TakeProfit) / currentPrice) * 100
					}
					if llmTPPct > 0 {
						llmUsed = true
					}
				}
			}
		}

		// Mode-specific base multipliers and limits
		var baseSLMultiplier, baseTPMultiplier float64
		var minSL, maxSL, minTP, maxTP float64
		var positionPct, leverage int

		switch mode {
		case GinieModeScalp:
			positionPct = 5
			leverage = 10
			baseSLMultiplier = 0.5  // 0.5x ATR for tight SL
			baseTPMultiplier = 1.0  // 1x ATR for quick TP
			minSL, maxSL = 0.2, 0.8 // Strict limits for scalp
			minTP, maxTP = 0.3, 2.0
		case GinieModeSwing:
			positionPct = 10
			leverage = 5
			baseSLMultiplier = 1.5  // 1.5x ATR
			baseTPMultiplier = 3.0  // 3x ATR
			minSL, maxSL = 1.0, 5.0 // Wider limits for swing
			minTP, maxTP = 2.0, 15.0
		case GinieModePosition:
			positionPct = 15
			leverage = 2
			baseSLMultiplier = 2.5  // 2.5x ATR
			baseTPMultiplier = 5.0  // 5x ATR
			minSL, maxSL = 3.0, 15.0
			minTP, maxTP = 5.0, 50.0
		}

		report.TradeExecution.PositionPct = float64(positionPct)
		report.TradeExecution.Leverage = leverage

		// Calculate AI/LLM suggested position size
		// This provides an AI-driven sizing recommendation based on market conditions
		llmSizeUSD, llmSizeReasoning := g.calculateLLMPositionSize(
			symbol,
			mode,
			report.ConfidenceScore,
			scan.Volatility.Regime,
			atrPct,
			scan.Trend.TrendDirection,
		)
		report.TradeExecution.LLMSuggestedSizeUSD = llmSizeUSD
		report.TradeExecution.LLMSizeReasoning = llmSizeReasoning

		// Calculate ATR-based SL/TP
		atrSLPct := atrPct * baseSLMultiplier
		atrTPPct := atrPct * baseTPMultiplier

		// Blend LLM and ATR (70% LLM, 30% ATR if LLM available)
		var finalSLPct, finalTPPct float64
		if llmUsed && llmSLPct > 0 {
			finalSLPct = llmSLPct*0.7 + atrSLPct*0.3
		} else {
			finalSLPct = atrSLPct
		}
		if llmUsed && llmTPPct > 0 {
			finalTPPct = llmTPPct*0.7 + atrTPPct*0.3
		} else {
			finalTPPct = atrTPPct
		}

		// Clamp to mode-specific limits
		if finalSLPct < minSL {
			finalSLPct = minSL
		}
		if finalSLPct > maxSL {
			finalSLPct = maxSL
		}
		if finalTPPct < minTP {
			finalTPPct = minTP
		}
		if finalTPPct > maxTP {
			finalTPPct = maxTP
		}

		report.TradeExecution.StopLossPct = finalSLPct

		// Generate 4 TP levels proportionally (25% each at 25%, 50%, 75%, 100% of target)
		report.TradeExecution.TakeProfits = []GinieTakeProfitLevel{
			{Level: 1, Percent: 25, GainPct: finalTPPct * 0.25}, // 25% of target
			{Level: 2, Percent: 25, GainPct: finalTPPct * 0.50}, // 50% of target
			{Level: 3, Percent: 25, GainPct: finalTPPct * 0.75}, // 75% of target
			{Level: 4, Percent: 25, GainPct: finalTPPct * 1.00}, // 100% of target (trailing)
		}

		// Log adaptive calculation
		g.logger.Debug("Ginie adaptive SL/TP calculated",
			"symbol", symbol,
			"mode", mode,
			"atr_pct", fmt.Sprintf("%.2f%%", atrPct),
			"llm_used", llmUsed,
			"sl_pct", fmt.Sprintf("%.2f%%", finalSLPct),
			"tp_pct", fmt.Sprintf("%.2f%%", finalTPPct))

		// Calculate stop loss and TP prices
		direction := 1.0
		if signals.Direction == "short" {
			direction = -1.0
		}
		report.TradeExecution.StopLoss = currentPrice * (1 - direction*report.TradeExecution.StopLossPct/100)
		for i := range report.TradeExecution.TakeProfits {
			report.TradeExecution.TakeProfits[i].Price = currentPrice * (1 + direction*report.TradeExecution.TakeProfits[i].GainPct/100)
		}

		// Risk:Reward
		if len(report.TradeExecution.TakeProfits) > 0 {
			avgTP := 0.0
			for _, tp := range report.TradeExecution.TakeProfits {
				avgTP += tp.GainPct * tp.Percent / 100
			}
			report.TradeExecution.RiskReward = avgTP / report.TradeExecution.StopLossPct
		}
	} else {
		report.TradeExecution.Action = "WAIT"
	}

	// Hedge recommendation
	if scan.Status == ScanStatusHedgeRequired || scan.Volatility.Regime == "Extreme" {
		report.Hedge.Required = true
		report.Hedge.HedgeType = "direct"
		report.Hedge.HedgeSize = 50
		report.Hedge.Reason = "High volatility environment"
	}

	// Invalidation conditions
	report.InvalidationConditions = []string{
		fmt.Sprintf("Price breaks below $%.2f", scan.Structure.NearestSupport),
		"ADX drops below 20",
		"Volume drops significantly",
	}

	// Re-evaluate conditions
	report.ReEvaluateConditions = []string{
		"New high/low formed",
		"Major news event",
		"BTC correlation breaks",
	}

	// Next review based on mode
	switch mode {
	case GinieModeScalp:
		report.NextReview = "15 minutes"
	case GinieModeSwing:
		report.NextReview = "4 hours"
	case GinieModePosition:
		report.NextReview = "1 day"
	}

	// === ADAPTIVE ADX STRENGTH FILTER ===
	// Check if trend is strong enough for the selected mode
	adxPassed, adxPenalty := g.checkADXStrengthRequirement(trendAnalysis.ADXValue, mode)
	if !adxPassed {
		// Determine threshold based on mode
		var adxThreshold float64
		switch mode {
		case GinieModeScalp:
			adxThreshold = 25
		case GinieModeSwing:
			adxThreshold = 30
		case GinieModePosition:
			adxThreshold = 35
		default:
			adxThreshold = 20
		}

		rejectionTracker.ADXStrength = &ADXStrengthRejection{
			Blocked:   false, // Not a hard block, just penalty
			ADXValue:  trendAnalysis.ADXValue,
			Threshold: adxThreshold,
			Penalty:   adxPenalty,
			Reason:    fmt.Sprintf("Weak trend: ADX %.1f (threshold %.0f for %s mode) - 10%% confidence penalty applied", trendAnalysis.ADXValue, adxThreshold, mode),
		}
		// Don't add to rejection list since it's soft (penalty, not block)

		if g.logger != nil {
			g.logger.Warn("Weak trend detected - applying ADX strength penalty",
				"symbol", symbol,
				"adx", trendAnalysis.ADXValue,
				"mode", mode,
				"penalty", "10%")
		}
	}

	// === LLM TRADING ANALYSIS INTEGRATION ===
	// Perform LLM analysis if enabled for this mode
	var llmResponse *LLMAnalysisResponse
	var decisionContext *DecisionContext

	// Calculate base technical confidence (0-100 scale)
	// StrengthScore is already 0-100, scan.Score is 0-100, adxPenalty is 0-1
	technicalConfidence := int(signals.StrengthScore * (scan.Score / 100) * adxPenalty)
	technicalDirection := signals.Direction

	// Attempt LLM analysis
	llmResponse, decisionContext, _ = g.PerformLLMAnalysis(symbol, klines, mode)

	// Initialize decision context if not set
	if decisionContext == nil {
		decisionContext = &DecisionContext{
			SkippedLLM: true,
			SkipReason: "LLM analysis not available",
		}
	}

	// Store technical values in context
	decisionContext.TechnicalConfidence = technicalConfidence
	decisionContext.TechnicalDirection = technicalDirection

	// Fuse confidence if LLM response is available
	if llmResponse != nil && !decisionContext.SkippedLLM {
		// Get mode-specific LLM weight from SettingsManager
		sm := GetSettingsManager()
		modeLLMSettings := sm.GetModeLLMSettings(mode)
		llmWeight := modeLLMSettings.LLMWeight
		if g.logger != nil {
			g.logger.Debug("[LLM] Using mode-specific LLM weight",
				"mode", mode,
				"llm_weight", llmWeight)
		}

		finalConfidence, finalDirection, agreement := g.FuseConfidence(
			technicalConfidence,
			technicalDirection,
			llmResponse,
			llmWeight,
		)

		decisionContext.FinalConfidence = finalConfidence
		decisionContext.Agreement = agreement

		// Update report confidence score (keep 0-100 scale for comparison with thresholds)
		report.ConfidenceScore = float64(finalConfidence)

		// If LLM and technical disagree strongly, consider adjusting direction
		if !agreement && llmResponse.Confidence > 70 && technicalConfidence < 50 {
			// LLM has high confidence, technical has low - consider LLM direction
			if g.logger != nil {
				g.logger.Info("[LLM] High LLM confidence overriding weak technical signal",
					"symbol", symbol,
					"tech_direction", technicalDirection,
					"llm_direction", llmResponse.Recommendation,
					"llm_confidence", llmResponse.Confidence)
			}
		}

		// Log fusion result
		if g.logger != nil {
			g.logger.Info("[LLM] Confidence fusion applied",
				"symbol", symbol,
				"tech_confidence", technicalConfidence,
				"llm_confidence", llmResponse.Confidence,
				"final_confidence", finalConfidence,
				"agreement", agreement,
				"final_direction", finalDirection)
		}
	} else {
		// No LLM - use technical only
		decisionContext.FinalConfidence = technicalConfidence
		report.ConfidenceScore = float64(technicalConfidence)

		if g.logger != nil {
			g.logger.Debug("[LLM] Using technical analysis only",
				"symbol", symbol,
				"reason", decisionContext.SkipReason)
		}
	}

	// Attach decision context to report
	report.DecisionContext = decisionContext

	// === STRICT COUNTER-TREND FILTER ===
	// Block counter-trend trades unless they have strong reversal signals
	if signals.Direction != "neutral" && trendAnalysis.TrendDirection != "neutral" {
		// Check if signal direction matches trend direction
		signalIsBullish := signals.Direction == "long"
		trendIsBullish := trendAnalysis.TrendDirection == "bullish"

		if signalIsBullish != trendIsBullish {
			// Signal contradicts trend - this is a counter-trend trade (bounce trade)
			// Validate with strict reversal signal requirements
			if !g.isValidReversalTrade(signals.Direction, report.ConfidenceScore, klines) {
				// Track counter-trend rejection
				rejectionTracker.CounterTrend = &CounterTrendRejection{
					Blocked:         true,
					SignalDirection: signals.Direction,
					TrendDirection:  trendAnalysis.TrendDirection,
					MissingRequirements: []string{"RSI extreme zone", "ADX weakening", "Reversal pattern"},
					Reason:          fmt.Sprintf("Counter-trend blocked: %s signal vs %s trend", signals.Direction, trendAnalysis.TrendDirection),
				}
				rejectionTracker.AddRejection(fmt.Sprintf("Counter-trend: %s signal vs %s trend (missing reversal signals)", signals.Direction, trendAnalysis.TrendDirection))

				if g.logger != nil {
					g.logger.Info("Blocking counter-trend trade - insufficient reversal signals",
						"symbol", symbol,
						"signal", signals.Direction,
						"trend", trendAnalysis.TrendDirection,
						"confidence", report.ConfidenceScore)
				}

				// Return report with rejection tracking attached
				counterTrendReport := &GinieDecisionReport{
					Symbol:             symbol,
					Timestamp:          time.Now(),
					ScanStatus:         scan.Status,
					SelectedMode:       mode,
					Recommendation:     RecommendationSkip,
					RecommendationNote: "Counter-trend trade rejected - missing required reversal signals (RSI extreme zone, ADX weakening, reversal pattern)",
					ConfidenceScore:    0.0,
					RejectionTracking:  rejectionTracker,
				}
				return counterTrendReport, nil
			}

			if g.logger != nil {
				g.logger.Info("Allowing counter-trend trade with strong reversal signals",
					"symbol", symbol,
					"signal", signals.Direction,
					"trend", trendAnalysis.TrendDirection,
					"confidence", report.ConfidenceScore)
			}
		}
	}

	// === FVG/ORDER BLOCK CONFLUENCE BOOST ===
	// Apply confidence boost/penalty based on price action confluence
	priceActionBoost := 0.0
	if signals.Direction == "long" && scan.PriceAction.HasBullishSetup {
		// Bullish setup aligns with long signal
		priceActionBoost = scan.PriceAction.ConfluenceScore * 0.15 // Up to +15 confidence
		if scan.PriceAction.FVG.InFVGZone && scan.PriceAction.FVG.FVGZoneType == "bullish" {
			priceActionBoost += 5 // Extra boost for being in FVG zone
		}
		if scan.PriceAction.OrderBlocks.InOBZone && scan.PriceAction.OrderBlocks.OBZoneType == "bullish" {
			priceActionBoost += 5 // Extra boost for being in OB zone
		}
	} else if signals.Direction == "short" && scan.PriceAction.HasBearishSetup {
		// Bearish setup aligns with short signal
		priceActionBoost = scan.PriceAction.ConfluenceScore * 0.15
		if scan.PriceAction.FVG.InFVGZone && scan.PriceAction.FVG.FVGZoneType == "bearish" {
			priceActionBoost += 5
		}
		if scan.PriceAction.OrderBlocks.InOBZone && scan.PriceAction.OrderBlocks.OBZoneType == "bearish" {
			priceActionBoost += 5
		}
	} else if (signals.Direction == "long" && scan.PriceAction.HasBearishSetup) ||
		(signals.Direction == "short" && scan.PriceAction.HasBullishSetup) {
		// Price action setup contradicts signal - apply penalty
		priceActionBoost = -10
	}

	if priceActionBoost != 0 {
		oldConfidence := report.ConfidenceScore
		report.ConfidenceScore += priceActionBoost
		if report.ConfidenceScore > 100 {
			report.ConfidenceScore = 100
		}
		if report.ConfidenceScore < 0 {
			report.ConfidenceScore = 0
		}

		if g.logger != nil {
			g.logger.Debug("Price action confluence applied",
				"symbol", symbol,
				"fvg_zone", scan.PriceAction.FVG.InFVGZone,
				"ob_zone", scan.PriceAction.OrderBlocks.InOBZone,
				"confluence_score", scan.PriceAction.ConfluenceScore,
				"setup_quality", scan.PriceAction.SetupQuality,
				"boost", priceActionBoost,
				"old_confidence", oldConfidence,
				"new_confidence", report.ConfidenceScore)
		}
	}

	// Final recommendation (ConfidenceScore is 0-100, thresholds are 0-100)
	// Execute if >= 30%, Wait if >= 20%, otherwise Skip
	if report.ConfidenceScore >= 30.0 {
		report.Recommendation = RecommendationExecute
		report.RecommendationNote = "Strong signals with good market conditions"
	} else if report.ConfidenceScore >= 20.0 {
		report.Recommendation = RecommendationWait
		report.RecommendationNote = "Signals present but consider waiting for better entry"

		// Track confidence issue (not a hard block, but worth noting)
		rejectionTracker.Confidence = &ConfidenceRejection{
			Blocked:          false,
			ConfidenceScore:  report.ConfidenceScore,
			ExecuteThreshold: 30.0,
			WaitThreshold:    20.0,
			Reason:           fmt.Sprintf("Confidence %.1f%% below execute threshold (30%%)", report.ConfidenceScore),
		}
	} else {
		report.Recommendation = RecommendationSkip
		report.RecommendationNote = "Insufficient confluence or poor conditions"

		// Track low confidence rejection
		rejectionTracker.Confidence = &ConfidenceRejection{
			Blocked:          true,
			ConfidenceScore:  report.ConfidenceScore,
			ExecuteThreshold: 30.0,
			WaitThreshold:    20.0,
			Reason:           fmt.Sprintf("Low confidence: %.1f%% (need 20%% for WAIT, 30%% for EXECUTE)", report.ConfidenceScore),
		}
		rejectionTracker.AddRejection(fmt.Sprintf("Confidence: %.1f%% (need 30%% to execute)", report.ConfidenceScore))
	}

	// Store decision
	g.decisionLock.Lock()
	g.decisions = append(g.decisions, *report)
	if len(g.decisions) > g.maxDecisions {
		g.decisions = g.decisions[1:]
	}
	g.decisionLock.Unlock()

	return report, nil
}

// GetRecentDecisions returns recent decisions
func (g *GinieAnalyzer) GetRecentDecisions(limit int) []GinieDecisionReport {
	g.decisionLock.RLock()
	defer g.decisionLock.RUnlock()

	if limit <= 0 || limit > len(g.decisions) {
		limit = len(g.decisions)
	}

	start := len(g.decisions) - limit
	if start < 0 {
		start = 0
	}

	result := make([]GinieDecisionReport, limit)
	copy(result, g.decisions[start:])
	return result
}

// UpdateDecisionRecommendation updates the recommendation of the most recent decision for a symbol.
// This is used when MTF rejects a trade after the initial decision was made.
func (g *GinieAnalyzer) UpdateDecisionRecommendation(symbol string, newRecommendation GenieRecommendation, reason string) {
	g.decisionLock.Lock()
	defer g.decisionLock.Unlock()

	// Find the most recent decision for this symbol and update it
	for i := len(g.decisions) - 1; i >= 0; i-- {
		if g.decisions[i].Symbol == symbol {
			g.decisions[i].Recommendation = newRecommendation
			g.decisions[i].RecommendationNote = fmt.Sprintf("[MTF REJECTED] %s", reason)
			log.Printf("[DECISION-UPDATE] %s: Updated recommendation to %s - %s", symbol, newRecommendation, reason)
			return
		}
	}
}


// ===== LLM TREND CONFIRMATION SYSTEM =====
// trendConfirmationCache stores cached LLM trend confirmations
type trendConfirmationCache struct {
	mu          sync.RWMutex
	cache       map[string]*TrendConfirmation
	lastUpdated map[string]time.Time
}

var llmTrendCache = &trendConfirmationCache{
	cache:       make(map[string]*TrendConfirmation),
	lastUpdated: make(map[string]time.Time),
}

// ===== LLM TRADING ANALYSIS INTEGRATION =====

// cachedLLMResponse stores a cached LLM analysis response with expiry
type cachedLLMResponse struct {
	response  *LLMAnalysisResponse
	timestamp time.Time
	expiry    time.Duration
}

// llmAnalysisCache stores cached LLM analysis responses
type llmAnalysisCache struct {
	mu    sync.RWMutex
	cache map[string]*cachedLLMResponse
}

var llmResponseCache = &llmAnalysisCache{
	cache: make(map[string]*cachedLLMResponse),
}

// GetCachedLLMResponse retrieves a cached LLM response for a symbol if not expired
func (g *GinieAnalyzer) GetCachedLLMResponse(symbol string) (*LLMAnalysisResponse, bool) {
	llmResponseCache.mu.RLock()
	defer llmResponseCache.mu.RUnlock()

	cached, exists := llmResponseCache.cache[symbol]
	if !exists {
		return nil, false
	}

	// Check if cache has expired
	if time.Since(cached.timestamp) > cached.expiry {
		return nil, false
	}

	if g.logger != nil {
		g.logger.Debug("[LLM] Using cached response", "symbol", symbol, "age_seconds", int(time.Since(cached.timestamp).Seconds()))
	}

	return cached.response, true
}

// CacheLLMResponse stores an LLM response in the cache with specified duration
func (g *GinieAnalyzer) CacheLLMResponse(symbol string, response *LLMAnalysisResponse, durationSec int) {
	llmResponseCache.mu.Lock()
	defer llmResponseCache.mu.Unlock()

	llmResponseCache.cache[symbol] = &cachedLLMResponse{
		response:  response,
		timestamp: time.Now(),
		expiry:    time.Duration(durationSec) * time.Second,
	}

	if g.logger != nil {
		g.logger.Debug("[LLM] Cached response", "symbol", symbol, "expiry_seconds", durationSec)
	}
}

// ClearLLMCache clears all cached LLM responses
func (g *GinieAnalyzer) ClearLLMCache() {
	llmResponseCache.mu.Lock()
	defer llmResponseCache.mu.Unlock()
	llmResponseCache.cache = make(map[string]*cachedLLMResponse)
	if g.logger != nil {
		g.logger.Info("[LLM] Cache cleared")
	}
}

// BuildLLMPrompt constructs the system and user prompts for LLM trading analysis
func (g *GinieAnalyzer) BuildLLMPrompt(symbol string, klines []binance.Kline, mode GinieTradingMode) (systemPrompt string, userPrompt string, err error) {
	if len(klines) < 50 {
		return "", "", fmt.Errorf("insufficient kline data: need at least 50, got %d", len(klines))
	}

	// Build system prompt for crypto trading analyst
	systemPrompt = `You are an expert cryptocurrency trading analyst specializing in technical analysis and market sentiment.
Your task is to analyze the provided market data and give a clear trading recommendation.

IMPORTANT RULES:
1. Be conservative with confidence scores - only high confidence (>70) when multiple strong signals align
2. Always consider risk management - suggest appropriate SL/TP levels
3. Consider the trading mode when making recommendations
4. Provide clear, concise reasoning for your decision
5. Identify the key factors driving your recommendation

Your response must be in valid JSON format with this EXACT structure (no markdown, no explanation):
{
  "recommendation": "LONG" | "SHORT" | "HOLD",
  "confidence": 0-100,
  "reasoning": "Brief 1-2 sentence explanation",
  "key_factors": ["factor1", "factor2", "factor3"],
  "risk_level": "low" | "moderate" | "high",
  "suggested_sl_percent": 1.5,
  "suggested_tp_percent": 3.0,
  "time_horizon": "minutes" | "hours" | "days"
}

Trading Mode Guidelines:
- SCALP: Quick 1-15 minute trades, tight SL (0.3-1%), small TP (0.5-2%), high confidence required
- SWING: 4h-1d trades, moderate SL (1-3%), larger TP (3-10%), look for trend continuation
- POSITION: 1d+ trades, wider SL (3-10%), larger TP (10-30%), focus on major trend direction`

	// Calculate technical indicators for the prompt
	currentPrice := klines[len(klines)-1].Close
	rsi14 := g.calculateRSI(klines, 14)
	macd, signal, hist := g.calculateMACD(klines)
	ema20 := g.calculateEMA(klines, 20)
	ema50 := g.calculateEMA(klines, 50)
	ema200 := g.calculateEMA(klines, 200)
	adx, plusDI, minusDI := g.calculateADX(klines, 14)
	sma20, bbUpper, bbLower := g.calculateBollingerBands(klines, 20, 2)
	atr14 := g.calculateATR(klines, 14)
	atrPercent := (atr14 / currentPrice) * 100

	// Calculate price changes
	priceChange1h := 0.0
	priceChange4h := 0.0
	priceChange24h := 0.0
	if len(klines) >= 2 {
		priceChange1h = ((currentPrice - klines[len(klines)-2].Close) / klines[len(klines)-2].Close) * 100
	}
	if len(klines) >= 5 {
		priceChange4h = ((currentPrice - klines[len(klines)-5].Close) / klines[len(klines)-5].Close) * 100
	}
	if len(klines) >= 25 {
		priceChange24h = ((currentPrice - klines[len(klines)-25].Close) / klines[len(klines)-25].Close) * 100
	}

	// Determine trend from EMAs
	emaAlignment := "neutral"
	if currentPrice > ema20 && ema20 > ema50 && ema50 > ema200 {
		emaAlignment = "bullish (price > EMA20 > EMA50 > EMA200)"
	} else if currentPrice < ema20 && ema20 < ema50 && ema50 < ema200 {
		emaAlignment = "bearish (price < EMA20 < EMA50 < EMA200)"
	} else if currentPrice > ema50 {
		emaAlignment = "bullish bias (price above EMA50)"
	} else if currentPrice < ema50 {
		emaAlignment = "bearish bias (price below EMA50)"
	}

	// MACD signal
	macdSignal := "neutral"
	if macd > signal && hist > 0 {
		macdSignal = "bullish (MACD above signal, positive histogram)"
	} else if macd < signal && hist < 0 {
		macdSignal = "bearish (MACD below signal, negative histogram)"
	} else if hist > 0 {
		macdSignal = "bullish momentum"
	} else if hist < 0 {
		macdSignal = "bearish momentum"
	}

	// RSI interpretation
	rsiInterpretation := "neutral"
	if rsi14 > 70 {
		rsiInterpretation = "overbought (potential reversal)"
	} else if rsi14 < 30 {
		rsiInterpretation = "oversold (potential reversal)"
	} else if rsi14 > 55 {
		rsiInterpretation = "bullish momentum"
	} else if rsi14 < 45 {
		rsiInterpretation = "bearish momentum"
	}

	// ADX trend strength
	trendStrength := "no trend (ranging)"
	if adx > 40 {
		trendStrength = "very strong trend"
	} else if adx > 25 {
		trendStrength = "strong trend"
	} else if adx > 20 {
		trendStrength = "moderate trend"
	}

	trendDirection := "neutral"
	if plusDI > minusDI {
		trendDirection = "bullish"
	} else if minusDI > plusDI {
		trendDirection = "bearish"
	}

	// BB position
	bbPosition := "middle of bands"
	bbWidth := ((bbUpper - bbLower) / sma20) * 100
	if currentPrice > bbUpper {
		bbPosition = "above upper band (extended)"
	} else if currentPrice < bbLower {
		bbPosition = "below lower band (extended)"
	} else if currentPrice > sma20 {
		bbPosition = "above middle band (bullish bias)"
	} else if currentPrice < sma20 {
		bbPosition = "below middle band (bearish bias)"
	}

	// Build user prompt with market data
	userPrompt = fmt.Sprintf(`Analyze this cryptocurrency for a %s trade:

=== SYMBOL & PRICE ===
Symbol: %s
Current Price: $%.8f
Mode: %s

=== PRICE CHANGES ===
1h Change: %.2f%%
4h Change: %.2f%%
24h Change: %.2f%%

=== TECHNICAL INDICATORS ===
RSI(14): %.2f - %s
MACD: %.6f, Signal: %.6f, Histogram: %.6f - %s
EMA Alignment: %s
  - EMA20: $%.8f (%.2f%% from price)
  - EMA50: $%.8f (%.2f%% from price)
  - EMA200: $%.8f (%.2f%% from price)
ADX(14): %.2f - %s, Direction: %s
  - +DI: %.2f, -DI: %.2f
Bollinger Bands: %s
  - Upper: $%.8f, Middle: $%.8f, Lower: $%.8f
  - Band Width: %.2f%%
ATR(14): $%.8f (%.2f%% of price) - Volatility gauge

=== RECENT CANDLES (Last 5) ===
`,
		mode, symbol, currentPrice, mode,
		priceChange1h, priceChange4h, priceChange24h,
		rsi14, rsiInterpretation,
		macd, signal, hist, macdSignal,
		emaAlignment,
		ema20, ((currentPrice-ema20)/ema20)*100,
		ema50, ((currentPrice-ema50)/ema50)*100,
		ema200, ((currentPrice-ema200)/ema200)*100,
		adx, trendStrength, trendDirection,
		plusDI, minusDI,
		bbPosition, bbUpper, sma20, bbLower, bbWidth,
		atr14, atrPercent,
	)

	// Add last 5 candles
	start := len(klines) - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		k := klines[i]
		candleType := "NEUTRAL"
		bodyPercent := math.Abs((k.Close-k.Open)/k.Open) * 100
		if k.Close > k.Open {
			candleType = "GREEN"
		} else if k.Close < k.Open {
			candleType = "RED"
		}
		userPrompt += fmt.Sprintf("  %s: O:%.8f H:%.8f L:%.8f C:%.8f (%.2f%% body)\n",
			candleType, k.Open, k.High, k.Low, k.Close, bodyPercent)
	}

	userPrompt += "\nProvide your analysis in the specified JSON format only."

	return systemPrompt, userPrompt, nil
}

// ParseLLMResponse parses the JSON response from LLM and validates it
func (g *GinieAnalyzer) ParseLLMResponse(response string) (*LLMAnalysisResponse, error) {
	// Clean the response - strip markdown code blocks if present
	response = strings.TrimSpace(response)

	// Handle ```json ... ``` blocks
	re := regexp.MustCompile("(?s)^```(?:json)?\\s*\\n?(.*?)\\n?```$")
	if matches := re.FindStringSubmatch(response); len(matches) > 1 {
		response = strings.TrimSpace(matches[1])
	}

	// Also try to find JSON within the response if not wrapped in code blocks
	if !strings.HasPrefix(response, "{") {
		// Try to extract JSON from the response
		jsonStart := strings.Index(response, "{")
		jsonEnd := strings.LastIndex(response, "}")
		if jsonStart >= 0 && jsonEnd > jsonStart {
			response = response[jsonStart : jsonEnd+1]
		}
	}

	var parsed LLMAnalysisResponse
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		if g.logger != nil {
			g.logger.Error("[LLM] Failed to parse response", "error", err, "response_preview", response[:min(200, len(response))])
		}
		return nil, fmt.Errorf("failed to parse LLM response as JSON: %w", err)
	}

	// Validate recommendation
	parsed.Recommendation = strings.ToUpper(parsed.Recommendation)
	if parsed.Recommendation != "LONG" && parsed.Recommendation != "SHORT" && parsed.Recommendation != "HOLD" {
		if g.logger != nil {
			g.logger.Warn("[LLM] Invalid recommendation, defaulting to HOLD", "got", parsed.Recommendation)
		}
		parsed.Recommendation = "HOLD"
	}

	// Validate confidence (0-100)
	if parsed.Confidence < 0 {
		parsed.Confidence = 0
	}
	if parsed.Confidence > 100 {
		parsed.Confidence = 100
	}

	// Validate reasoning is non-empty
	if strings.TrimSpace(parsed.Reasoning) == "" {
		parsed.Reasoning = "No reasoning provided by LLM"
	}

	// Validate risk level
	parsed.RiskLevel = strings.ToLower(parsed.RiskLevel)
	if parsed.RiskLevel != "low" && parsed.RiskLevel != "moderate" && parsed.RiskLevel != "high" {
		parsed.RiskLevel = "moderate"
	}

	// Validate SL/TP percentages (must be positive)
	if parsed.SuggestedSLPercent <= 0 {
		parsed.SuggestedSLPercent = 2.0 // Default 2%
	}
	if parsed.SuggestedTPPercent <= 0 {
		parsed.SuggestedTPPercent = 4.0 // Default 4%
	}

	// Validate time horizon
	parsed.TimeHorizon = strings.ToLower(parsed.TimeHorizon)
	if parsed.TimeHorizon != "minutes" && parsed.TimeHorizon != "hours" && parsed.TimeHorizon != "days" {
		parsed.TimeHorizon = "hours"
	}

	if g.logger != nil {
		g.logger.Debug("[LLM] Parsed response successfully",
			"recommendation", parsed.Recommendation,
			"confidence", parsed.Confidence,
			"risk_level", parsed.RiskLevel)
	}

	return &parsed, nil
}

// FuseConfidence combines technical and LLM confidence using weighted fusion
// Returns: (finalConfidence, finalDirection, agreement)
func (g *GinieAnalyzer) FuseConfidence(technicalConfidence int, technicalDirection string, llmResponse *LLMAnalysisResponse, llmWeight float64) (int, string, bool) {
	if llmResponse == nil {
		// No LLM response, return technical only
		return technicalConfidence, technicalDirection, false
	}

	// Clamp llmWeight to valid range
	if llmWeight < 0 {
		llmWeight = 0
	}
	if llmWeight > 1 {
		llmWeight = 1
	}

	llmConfidence := llmResponse.Confidence
	llmDirection := strings.ToLower(llmResponse.Recommendation)
	if llmDirection == "hold" {
		llmDirection = "neutral"
	}

	// Normalize technical direction
	techDir := strings.ToLower(technicalDirection)
	if techDir != "long" && techDir != "short" {
		techDir = "neutral"
	}

	// === WEIGHTED CONFIDENCE BLEND ===
	// When LLM says HOLD/neutral, it contributes 0 directional confidence instead of vetoing
	llmDirectionalConfidence := float64(llmConfidence)
	if llmDirection == "neutral" {
		llmDirectionalConfidence = 0 // HOLD = no directional conviction
		if g.logger != nil {
			g.logger.Debug("[LLM] LLM recommends HOLD - contributing 0 directional confidence",
				"llm_recommendation", llmResponse.Recommendation,
				"llm_confidence", llmConfidence)
		}
	}

	// Pure weighted blend: finalConfidence = (techConfidence × techWeight) + (llmDirectionalConfidence × llmWeight)
	techWeight := 1.0 - llmWeight
	baseFusion := (float64(technicalConfidence) * techWeight) + (llmDirectionalConfidence * llmWeight)

	// Check for agreement/disagreement
	agreement := false
	adjustment := 0.0

	if techDir == llmDirection && techDir != "neutral" {
		// Directions agree - add bonus
		agreement = true
		adjustment = 10.0
		if g.logger != nil {
			g.logger.Debug("[LLM] Direction agreement - adding bonus",
				"tech_direction", techDir,
				"llm_direction", llmDirection,
				"bonus", "+10")
		}
	} else if llmDirection == "neutral" {
		// LLM is neutral (HOLD) - no penalty, no bonus
		agreement = false
		adjustment = 0.0
		if g.logger != nil {
			g.logger.Debug("[LLM] LLM is neutral - no adjustment applied",
				"tech_direction", techDir,
				"llm_direction", llmDirection)
		}
	} else if (techDir == "long" && llmDirection == "short") || (techDir == "short" && llmDirection == "long") {
		// Directions conflict - apply penalty
		agreement = false
		adjustment = -15.0
		if g.logger != nil {
			g.logger.Warn("[LLM] Direction conflict - applying penalty",
				"tech_direction", techDir,
				"llm_direction", llmDirection,
				"penalty", "-15")
		}
	}

	// Apply adjustment
	finalConfidence := baseFusion + adjustment

	// Clamp to 0-100
	if finalConfidence < 0 {
		finalConfidence = 0
	}
	if finalConfidence > 100 {
		finalConfidence = 100
	}

	// Determine final direction
	// If they agree, use that direction
	// If they conflict, use the one with higher confidence
	// If one is neutral, use the non-neutral one
	finalDirection := techDir
	if agreement {
		finalDirection = techDir
	} else if techDir == "neutral" {
		finalDirection = llmDirection
	} else if llmDirection == "neutral" {
		finalDirection = techDir
	} else {
		// Conflict - use higher confidence direction
		if llmConfidence > technicalConfidence {
			finalDirection = llmDirection
		}
	}

	if g.logger != nil {
		g.logger.Info("[LLM] Confidence fusion complete",
			"tech_confidence", technicalConfidence,
			"tech_direction", techDir,
			"llm_confidence", llmConfidence,
			"llm_direction", llmDirection,
			"llm_directional_confidence", llmDirectionalConfidence,
			"llm_weight", llmWeight,
			"tech_weight", techWeight,
			"base_fusion", baseFusion,
			"adjustment", adjustment,
			"final_confidence", int(finalConfidence),
			"final_direction", finalDirection,
			"agreement", agreement)
	}

	return int(finalConfidence), finalDirection, agreement
}

// IsLLMEnabledForMode checks if LLM analysis is enabled for the given trading mode
func (g *GinieAnalyzer) IsLLMEnabledForMode(mode GinieTradingMode) bool {
	// LLM is enabled if we have a configured client
	if g.llmClient == nil || !g.llmClient.IsConfigured() {
		return false
	}

	// Check settings for mode-specific LLM enablement
	if g.settings != nil {
		switch mode {
		case GinieModeScalp:
			// Scalping typically needs faster decisions - LLM might add latency
			// Could be controlled by a setting, but default to enabled
			return true
		case GinieModeSwing:
			// Swing trading benefits from LLM analysis
			return true
		case GinieModePosition:
			// Position trading definitely benefits from LLM analysis
			return true
		case GinieModeUltraFast:
			// Ultra-fast is too quick for LLM - disable by default
			return false
		}
	}

	return true
}

// GetLLMCacheDuration returns the cache duration in seconds based on trading mode
func (g *GinieAnalyzer) GetLLMCacheDuration(mode GinieTradingMode) int {
	switch mode {
	case GinieModeScalp:
		return 60 // 1 minute cache for scalp
	case GinieModeSwing:
		return 300 // 5 minutes for swing
	case GinieModePosition:
		return 900 // 15 minutes for position
	case GinieModeUltraFast:
		return 30 // 30 seconds for ultra-fast (if enabled)
	default:
		return 120 // 2 minutes default
	}
}

// PerformLLMAnalysis performs LLM analysis for a symbol and returns the response
// Uses caching to avoid redundant API calls
func (g *GinieAnalyzer) PerformLLMAnalysis(symbol string, klines []binance.Kline, mode GinieTradingMode) (*LLMAnalysisResponse, *DecisionContext, error) {
	startTime := time.Now()
	ctx := &DecisionContext{
		SkippedLLM: true,
	}

	// Check if LLM is enabled for this mode
	if !g.IsLLMEnabledForMode(mode) {
		ctx.SkipReason = fmt.Sprintf("LLM disabled for mode %s", mode)
		if g.logger != nil {
			g.logger.Debug("[LLM] Skipping - disabled for mode", "mode", mode)
		}
		return nil, ctx, nil
	}

	// Check cache first
	if cached, ok := g.GetCachedLLMResponse(symbol); ok {
		ctx.SkippedLLM = false
		ctx.UsedCache = true
		ctx.LLMConfidence = cached.Confidence
		ctx.LLMDirection = cached.Recommendation
		ctx.LLMReasoning = cached.Reasoning
		ctx.LLMKeyFactors = cached.KeyFactors
		ctx.LLMLatencyMs = 0 // Cache hit has no latency
		if g.llmClient != nil {
			ctx.LLMProvider = string(g.llmClient.GetProvider())
		}
		return cached, ctx, nil
	}

	// Build prompt
	systemPrompt, userPrompt, err := g.BuildLLMPrompt(symbol, klines, mode)
	if err != nil {
		ctx.SkipReason = fmt.Sprintf("Failed to build prompt: %v", err)
		if g.logger != nil {
			g.logger.Error("[LLM] Failed to build prompt", "symbol", symbol, "error", err)
		}
		return nil, ctx, err
	}

	// Call LLM
	if g.llmClient == nil {
		ctx.SkipReason = "LLM client not configured"
		return nil, ctx, nil
	}

	if g.logger != nil {
		g.logger.Info("[LLM] Calling LLM for analysis", "symbol", symbol, "mode", mode)
	}

	response, err := g.llmClient.Complete(systemPrompt, userPrompt)
	latencyMs := time.Since(startTime).Milliseconds()

	if err != nil {
		ctx.SkipReason = fmt.Sprintf("LLM API error: %v", err)
		ctx.LLMLatencyMs = latencyMs
		if g.logger != nil {
			g.logger.Error("[LLM] API call failed", "symbol", symbol, "error", err, "latency_ms", latencyMs)
		}
		return nil, ctx, err
	}

	// Parse response
	parsed, err := g.ParseLLMResponse(response)
	if err != nil {
		ctx.SkipReason = fmt.Sprintf("Failed to parse response: %v", err)
		ctx.LLMLatencyMs = latencyMs
		return nil, ctx, err
	}

	// Cache the response
	cacheDuration := g.GetLLMCacheDuration(mode)
	g.CacheLLMResponse(symbol, parsed, cacheDuration)

	// Build context
	ctx.SkippedLLM = false
	ctx.UsedCache = false
	ctx.LLMConfidence = parsed.Confidence
	ctx.LLMDirection = parsed.Recommendation
	ctx.LLMReasoning = parsed.Reasoning
	ctx.LLMKeyFactors = parsed.KeyFactors
	ctx.LLMLatencyMs = latencyMs
	ctx.LLMProvider = string(g.llmClient.GetProvider())

	if g.logger != nil {
		g.logger.Info("[LLM] Analysis complete",
			"symbol", symbol,
			"recommendation", parsed.Recommendation,
			"confidence", parsed.Confidence,
			"latency_ms", latencyMs)
	}

	return parsed, ctx, nil
}

func (g *GinieAnalyzer) checkADXStrengthRequirement(adx float64, mode GinieTradingMode) (bool, float64) {
	thresholds := map[GinieTradingMode]float64{
		GinieModeUltraFast: 30.0, // Scalp needs strong trends
		GinieModeScalp:     30.0,
		GinieModeSwing:     20.0, // Swing more forgiving
		GinieModePosition:  25.0, // Position needs moderate strength
	}

	threshold, exists := thresholds[mode]
	if !exists {
		threshold = 25.0 // Default
	}

	if adx < threshold {
		// Weak trend - apply 10% confidence penalty
		return false, 0.90
	}

	// Strong enough trend - no penalty
	return true, 1.0
}

// ===== COUNTER-TREND TRADE VALIDATION =====
// isValidReversalTrade validates if a counter-trend trade has sufficient reversal signals
// Returns true only if the trade passes configured requirements (controlled by GinieConfig)
func (g *GinieAnalyzer) isValidReversalTrade(
	direction string,
	confidence float64,
	klines []binance.Kline,
) bool {
	// Check if counter-trend trading is disabled
	if !g.config.AllowCounterTrend {
		return false
	}

	// Check confidence threshold (configurable, default 50%)
	if confidence < g.config.CounterTrendMinConfidence {
		return false
	}

	if len(klines) < 50 {
		return false
	}

	// Check for reversal pattern confirmation (market structure) - if required
	if g.config.CounterTrendRequireReversal {
		if !g.hasReversalPattern(klines, direction) {
			return false
		}
	}

	// ADX must be weakening (trend losing strength) - if required
	if g.config.CounterTrendRequireADXWeakening {
		if len(klines) < 35 {
			return false
		}
		currentADX, _, _ := g.calculateADX(klines, 14)
		previousADX, _, _ := g.calculateADX(klines[:len(klines)-5], 14)
		if currentADX >= previousADX {
			return false // Trend still strengthening, not weakening
		}
	}

	// RSI must be in extreme zone - if required
	if g.config.CounterTrendRequireRSIExtreme {
		rsi := g.calculateRSI(klines, 14)
		if direction == "long" && rsi > 30 {
			return false // Not oversold enough for long bounce
		}
		if direction == "short" && rsi < 70 {
			return false // Not overbought enough for short bounce
		}
	}

	// Always block reversals in extreme volatility (safety measure)
	atr := g.calculateATR(klines, 14)
	avgATR := g.calculateAverageATR(klines, 50)
	if atr > avgATR*2.0 {
		return false // Too volatile, risk too high
	}

	return true
}

// hasReversalPattern checks if there's a reversal pattern in the klines
func (g *GinieAnalyzer) hasReversalPattern(klines []binance.Kline, direction string) bool {
	if len(klines) < 5 {
		return false
	}

	// Get swing points
	highs, lows := g.findSwingPoints(klines, 5)

	if len(highs) < 2 || len(lows) < 2 {
		return false
	}

	lastHigh := highs[len(highs)-1]
	prevHigh := highs[len(highs)-2]
	lastLow := lows[len(lows)-1]
	prevLow := lows[len(lows)-2]

	// For LONG reversal, expect LL (Lower Low) or test of previous support
	if direction == "long" {
		// Check if we have a potential reversal from downtrend
		// LH/LL pattern indicates downtrend reversal potential
		return (lastHigh > prevHigh && lastLow > prevLow) || // Starting HH/HL
			   (lastLow < prevLow) // Lower low suggests potential reversal
	}

	// For SHORT reversal, expect HH (Higher High) or test of previous resistance
	if direction == "short" {
		// Check if we have a potential reversal from uptrend
		// HH/HL pattern indicates uptrend reversal potential
		return (lastHigh < prevHigh && lastLow < prevLow) || // Starting LL/LH
			   (lastHigh > prevHigh) // Higher high suggests potential reversal
	}

	return false
}

// calculateAverageATR calculates the average ATR over a period
func (g *GinieAnalyzer) calculateAverageATR(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		period = len(klines)
	}

	var sum float64
	for i := len(klines) - period; i < len(klines); i++ {
		atr := g.calculateATR(klines[:i+1], 14)
		sum += atr
	}

	if period == 0 {
		return 0
	}
	return sum / float64(period)
}

// Helper functions

func (g *GinieAnalyzer) calculateATR(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 0
	}

	var trSum float64
	for i := len(klines) - period; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))
		trSum += tr
	}

	return trSum / float64(period)
}

func (g *GinieAnalyzer) calculateBollingerBands(klines []binance.Kline, period int, stdDev float64) (sma, upper, lower float64) {
	if len(klines) < period {
		return 0, 0, 0
	}

	// Calculate SMA
	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}
	sma = sum / float64(period)

	// Calculate standard deviation
	variance := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		diff := klines[i].Close - sma
		variance += diff * diff
	}
	sd := math.Sqrt(variance / float64(period))

	upper = sma + stdDev*sd
	lower = sma - stdDev*sd

	return sma, upper, lower
}

func (g *GinieAnalyzer) calculateEMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	multiplier := 2.0 / float64(period+1)

	// Start with SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += klines[i].Close
	}
	ema := sum / float64(period)

	// Calculate EMA
	for i := period; i < len(klines); i++ {
		ema = (klines[i].Close-ema)*multiplier + ema
	}

	return ema
}

func (g *GinieAnalyzer) calculateRSI(klines []binance.Kline, period int) float64 {
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

func (g *GinieAnalyzer) calculateStochRSI(klines []binance.Kline, rsiPeriod, kPeriod, dPeriod int) float64 {
	// Simplified StochRSI
	rsi := g.calculateRSI(klines, rsiPeriod)

	// Normalize to 0-100 range (simplified)
	return rsi
}

func (g *GinieAnalyzer) calculateMACD(klines []binance.Kline) (macd, signal, histogram float64) {
	ema12 := g.calculateEMA(klines, 12)
	ema26 := g.calculateEMA(klines, 26)
	macd = ema12 - ema26

	// Signal line (9-period EMA of MACD) - simplified
	signal = macd * 0.9 // Approximation
	histogram = macd - signal

	return macd, signal, histogram
}

func (g *GinieAnalyzer) calculateADX(klines []binance.Kline, period int) (adx, plusDI, minusDI float64) {
	if len(klines) < period*2 {
		return 25, 50, 50 // Default values
	}

	// Calculate +DM, -DM, and TR
	var plusDMSum, minusDMSum, trSum float64

	for i := len(klines) - period; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevHigh := klines[i-1].High
		prevLow := klines[i-1].Low
		prevClose := klines[i-1].Close

		plusDM := 0.0
		minusDM := 0.0

		upMove := high - prevHigh
		downMove := prevLow - low

		if upMove > downMove && upMove > 0 {
			plusDM = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM = downMove
		}

		tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))

		plusDMSum += plusDM
		minusDMSum += minusDM
		trSum += tr
	}

	if trSum > 0 {
		plusDI = (plusDMSum / trSum) * 100
		minusDI = (minusDMSum / trSum) * 100
	}

	// Calculate DX and ADX (simplified)
	if plusDI+minusDI > 0 {
		dx := math.Abs(plusDI-minusDI) / (plusDI + minusDI) * 100
		adx = dx // Simplified - should be smoothed
	}

	return adx, plusDI, minusDI
}

func (g *GinieAnalyzer) findSwingPoints(klines []binance.Kline, lookback int) (highs, lows []float64) {
	highs = make([]float64, 0)
	lows = make([]float64, 0)

	for i := lookback; i < len(klines)-lookback; i++ {
		isHigh := true
		isLow := true

		for j := i - lookback; j <= i+lookback; j++ {
			if j == i {
				continue
			}
			if klines[j].High > klines[i].High {
				isHigh = false
			}
			if klines[j].Low < klines[i].Low {
				isLow = false
			}
		}

		if isHigh {
			highs = append(highs, klines[i].High)
		}
		if isLow {
			lows = append(lows, klines[i].Low)
		}
	}

	return highs, lows
}

func findNearestAbove(price float64, levels []float64) float64 {
	nearest := 0.0
	minDiff := math.MaxFloat64

	for _, level := range levels {
		if level > price {
			diff := level - price
			if diff < minDiff {
				minDiff = diff
				nearest = level
			}
		}
	}

	return nearest
}

func findNearestBelow(price float64, levels []float64) float64 {
	nearest := 0.0
	minDiff := math.MaxFloat64

	for _, level := range levels {
		if level < price {
			diff := price - level
			if diff < minDiff {
				minDiff = diff
				nearest = level
			}
		}
	}

	return nearest
}

// ============================================================================
// PHASE 2: ULTRA-FAST MULTI-LAYER SIGNAL GENERATION
// ============================================================================

// ClassifyVolatilityRegime analyzes market volatility and returns adaptive parameters
// Layer 2 in ultra-fast signal system: Classifies volatility and sets re-entry delays
func (g *GinieAnalyzer) ClassifyVolatilityRegime(symbol string) (*VolatilityRegime, error) {
	// Get 5m klines for volatility calculation
	klines5m, err := g.futuresClient.GetFuturesKlines(symbol, "5m", 30) // 30 * 5m = 150m of history
	if err != nil || len(klines5m) < 14 {
		// Fallback to medium volatility on error
		return &VolatilityRegime{
			Level:            "medium",
			ATRRatio:         1.0,
			BBWidthPercent:   4.0,
			ReEntryDelay:     5 * time.Second,
			MaxTradesPerHour: 12,
			LastUpdate:       time.Now(),
		}, nil
	}

	// Calculate ATR on 5m candles
	atrValues := make([]float64, 0)
	for i := 1; i < len(klines5m); i++ {
		high := klines5m[i].High
		low := klines5m[i].Low
		prevClose := klines5m[i-1].Close

		tr := high - low
		if high-prevClose > tr {
			tr = high - prevClose
		}
		if prevClose-low > tr {
			tr = prevClose - low
		}
		atrValues = append(atrValues, tr)
	}

	// Calculate ATR14
	if len(atrValues) < 14 {
		return &VolatilityRegime{
			Level:            "medium",
			ATRRatio:         1.0,
			BBWidthPercent:   4.0,
			ReEntryDelay:     5 * time.Second,
			MaxTradesPerHour: 12,
			LastUpdate:       time.Now(),
		}, nil
	}

	atr14 := 0.0
	for i := 0; i < 14; i++ {
		atr14 += atrValues[i]
	}
	atr14 /= 14.0

	// Get current price for ATR ratio calculation
	currentPrice := klines5m[len(klines5m)-1].Close
	atrPercent := (atr14 / currentPrice) * 100

	// Calculate Bollinger Band width
	// Simplified: use standard deviation of last 20 closes
	closes := make([]float64, 0)
	for i := len(klines5m) - 20; i < len(klines5m); i++ {
		if i >= 0 {
			closes = append(closes, klines5m[i].Close)
		}
	}

	mean := 0.0
	for _, c := range closes {
		mean += c
	}
	mean /= float64(len(closes))

	variance := 0.0
	for _, c := range closes {
		variance += (c - mean) * (c - mean)
	}
	variance /= float64(len(closes))
	stdDev := math.Sqrt(variance)

	bbWidth := (stdDev * 2) / mean * 100 // Bollinger Band width as % of price

	// Classify regime based on ATR ratio
	regime := &VolatilityRegime{
		ATRRatio:   atrPercent / 0.8, // Baseline ~0.8%
		BBWidthPercent: bbWidth,
		LastUpdate: time.Now(),
	}

	if atrPercent > 2.0 || bbWidth > 8.0 {
		regime.Level = "extreme"
		regime.ReEntryDelay = 0 * time.Second
		regime.MaxTradesPerHour = 30
	} else if atrPercent > 1.5 || bbWidth > 5.0 {
		regime.Level = "high"
		regime.ReEntryDelay = 1 * time.Second
		regime.MaxTradesPerHour = 20
	} else if atrPercent > 0.8 || bbWidth > 3.0 {
		regime.Level = "medium"
		regime.ReEntryDelay = 5 * time.Second
		regime.MaxTradesPerHour = 12
	} else {
		regime.Level = "low"
		regime.ReEntryDelay = 60 * time.Second
		regime.MaxTradesPerHour = 6
	}

	return regime, nil
}

// CalculateFeeAwareTP calculates minimum profit target accounting for trading fees and volatility
// Formula: MinProfitTarget% = (EntryFee + ExitFee) / PositionUSD × 100 + (0.5 × ATR%)
func (g *GinieAnalyzer) CalculateFeeAwareTP(symbol string, positionUSD float64, leverage int, atrPercent float64) float64 {
	// Binance taker fee: 0.04% per order
	const binanceTakerFee = 0.0004

	// Calculate notional value
	notionalValue := positionUSD * float64(leverage)

	// Calculate fees (entry + exit)
	entryFee := notionalValue * binanceTakerFee
	exitFee := notionalValue * binanceTakerFee
	totalFee := entryFee + exitFee

	// Fee as % of position
	feePercent := (totalFee / positionUSD) * 100

	// ATR buffer (0.5x of ATR volatility)
	atrBuffer := 0.5 * atrPercent

	// Minimum profit target
	minProfitTarget := feePercent + atrBuffer

	// Ensure minimum of 0.5% (in case calculation is very low)
	if minProfitTarget < 0.5 {
		minProfitTarget = 0.5
	}

	// Cap at 3% for safety
	if minProfitTarget > 3.0 {
		minProfitTarget = 3.0
	}

	return minProfitTarget
}

// GenerateUltraFastSignal generates a 4-layer signal for ultra-fast scalping
// Layer 1: Trend Filter (1h) → Layer 2: Volatility Regime (5m) →
// Layer 3: Entry Trigger (1m) → Layer 4: Dynamic TP calculation
func (g *GinieAnalyzer) GenerateUltraFastSignal(symbol string) (*UltraFastSignal, error) {
	signal := &UltraFastSignal{
		Symbol:      symbol,
		SignalTime:  time.Now(),
		GeneratedAt: time.Now(),
	}

	// Layer 1: Trend Filter from 1h candles
	klines1h, err := g.futuresClient.GetFuturesKlines(symbol, "1h", 20)
	if err != nil || len(klines1h) < 3 {
		return nil, fmt.Errorf("failed to get 1h klines for %s: %w", symbol, err)
	}

	close1h := klines1h[len(klines1h)-1].Close
	close1hPrev := klines1h[len(klines1h)-2].Close
	ema20Idx := len(klines1h) - 1
	if ema20Idx >= 20 {
		ema20Idx = 20
	}

	// Multi-tiered trend check: more sensitive to catch more opportunities
	// Strong trend: ±0.3% or more
	// Weak trend: ±0.1% to ±0.3%
	// Neutral: less than ±0.1%
	priceDiffPct := ((close1h - close1hPrev) / close1hPrev) * 100.0

	if priceDiffPct >= 0.3 { // Strong uptrend
		signal.TrendBias = "LONG"
		signal.TrendStrength = 80
	} else if priceDiffPct >= 0.1 { // Weak uptrend
		signal.TrendBias = "LONG"
		signal.TrendStrength = 55
	} else if priceDiffPct <= -0.3 { // Strong downtrend
		signal.TrendBias = "SHORT"
		signal.TrendStrength = 80
	} else if priceDiffPct <= -0.1 { // Weak downtrend
		signal.TrendBias = "SHORT"
		signal.TrendStrength = 55
	} else {
		signal.TrendBias = "NEUTRAL"
		signal.TrendStrength = 40
	}

	// Calculate ADX for trend strength tracking (used by adaptive learning)
	adx, _, _ := g.calculateADX(klines1h, 14)
	signal.ADXValue = adx

	// Layer 1.5: Multi-timeframe Trend Alignment (5m/3m/1m weighted consensus)
	settings := GetSettingsManager().GetCurrentSettings()

	// Use new MTF weighted consensus if enabled, otherwise fall back to legacy 5m alignment
	if settings.UltraFastMTFEnabled {
		// Fetch 5m, 3m, 1m klines in parallel for faster signal generation
		type tfResult struct {
			tf       string
			bias     string
			strength float64
			klines   []binance.Kline
			err      error
		}

		results := make(chan tfResult, 3)
		timeframes := []string{"5m", "3m", "1m"}

		for _, tf := range timeframes {
			go func(timeframe string) {
				klines, err := g.futuresClient.GetFuturesKlines(symbol, timeframe, 10)
				if err != nil || len(klines) < 3 {
					results <- tfResult{tf: timeframe, err: fmt.Errorf("failed to get %s klines", timeframe)}
					return
				}

				// Calculate trend bias and strength from price movement
				closeNow := klines[len(klines)-1].Close
				closePrev := klines[len(klines)-2].Close
				priceDiffPct := ((closeNow - closePrev) / closePrev) * 100.0

				var bias string
				var strength float64

				// Thresholds adjusted per timeframe (shorter = more sensitive)
				var strongThreshold, weakThreshold float64
				switch timeframe {
				case "5m":
					strongThreshold, weakThreshold = 0.20, 0.05
				case "3m":
					strongThreshold, weakThreshold = 0.15, 0.04
				case "1m":
					strongThreshold, weakThreshold = 0.10, 0.03
				}

				if priceDiffPct >= strongThreshold {
					bias, strength = "LONG", 80
				} else if priceDiffPct >= weakThreshold {
					bias, strength = "LONG", 55
				} else if priceDiffPct <= -strongThreshold {
					bias, strength = "SHORT", 80
				} else if priceDiffPct <= -weakThreshold {
					bias, strength = "SHORT", 55
				} else {
					bias, strength = "NEUTRAL", 40
				}

				results <- tfResult{tf: timeframe, bias: bias, strength: strength, klines: klines}
			}(tf)
		}

		// Collect results
		tfData := make(map[string]tfResult)
		for i := 0; i < 3; i++ {
			r := <-results
			if r.err == nil {
				tfData[r.tf] = r
			}
		}

		// Get weights from settings
		weights := map[string]float64{
			"5m": settings.UltraFast5mWeight,
			"3m": settings.UltraFast3mWeight,
			"1m": settings.UltraFast1mWeight,
		}

		// Normalize weights if they don't sum to 1
		totalWeight := weights["5m"] + weights["3m"] + weights["1m"]
		if totalWeight > 0 && totalWeight != 1.0 {
			for k := range weights {
				weights[k] /= totalWeight
			}
		}

		// Calculate weighted scores and consensus
		var longScore, shortScore float64
		var longConsensus, shortConsensus int
		var alignmentDetails []string

		for tf, data := range tfData {
			weight := weights[tf]
			switch tf {
			case "5m":
				signal.Trend5mBias = data.bias
				signal.Trend5mStrength = data.strength
			case "3m":
				signal.Trend3mBias = data.bias
				signal.Trend3mStrength = data.strength
			case "1m":
				signal.Trend1mBias = data.bias
				signal.Trend1mStrength = data.strength
			}

			if data.bias == "LONG" {
				longScore += weight * data.strength
				if data.strength >= 50 {
					longConsensus++
				}
				alignmentDetails = append(alignmentDetails, fmt.Sprintf("%s:LONG(%.0f)", tf, data.strength))
			} else if data.bias == "SHORT" {
				shortScore += weight * data.strength
				if data.strength >= 50 {
					shortConsensus++
				}
				alignmentDetails = append(alignmentDetails, fmt.Sprintf("%s:SHORT(%.0f)", tf, data.strength))
			} else {
				alignmentDetails = append(alignmentDetails, fmt.Sprintf("%s:NEUTRAL", tf))
			}
		}

		// Determine final bias based on weighted scores and consensus
		minConsensus := settings.UltraFastMinConsensus
		minStrength := settings.UltraFastMinWeightedStrength

		if longScore > shortScore && longConsensus >= minConsensus && longScore >= minStrength {
			signal.TrendBias = "LONG"
			signal.CombinedTrendStrength = longScore
			signal.TimeframeConsensus = longConsensus
			signal.TrendAligned = true
			signal.AlignmentReason = fmt.Sprintf("MTF LONG consensus=%d/3 strength=%.0f [%s]",
				longConsensus, longScore, strings.Join(alignmentDetails, ", "))
		} else if shortScore > longScore && shortConsensus >= minConsensus && shortScore >= minStrength {
			signal.TrendBias = "SHORT"
			signal.CombinedTrendStrength = shortScore
			signal.TimeframeConsensus = shortConsensus
			signal.TrendAligned = true
			signal.AlignmentReason = fmt.Sprintf("MTF SHORT consensus=%d/3 strength=%.0f [%s]",
				shortConsensus, shortScore, strings.Join(alignmentDetails, ", "))
		} else {
			// No clear consensus - use strongest signal but mark as not aligned
			if longScore > shortScore {
				signal.TrendBias = "LONG"
				signal.CombinedTrendStrength = longScore
				signal.TimeframeConsensus = longConsensus
			} else if shortScore > longScore {
				signal.TrendBias = "SHORT"
				signal.CombinedTrendStrength = shortScore
				signal.TimeframeConsensus = shortConsensus
			} else {
				signal.TrendBias = "NEUTRAL"
				signal.CombinedTrendStrength = 40
				signal.TimeframeConsensus = 0
			}
			signal.TrendAligned = false
			signal.AlignmentReason = fmt.Sprintf("MTF weak consensus=%d<%d or strength=%.0f<%.0f [%s]",
				max(longConsensus, shortConsensus), minConsensus,
				math.Max(longScore, shortScore), minStrength,
				strings.Join(alignmentDetails, ", "))
		}

		// Trend stability check: ensure trend hasn't flipped in last 3 candles
		if settings.UltraFastTrendStabilityCheck && signal.TrendAligned {
			// Use 1m klines for stability check (most sensitive to recent changes)
			if data1m, ok := tfData["1m"]; ok && len(data1m.klines) >= 4 {
				klines := data1m.klines
				flipCount := 0
				var directions []string

				// Check last 3 candle directions (compare close vs open for each)
				for i := len(klines) - 3; i < len(klines); i++ {
					k := klines[i]
					if k.Close > k.Open {
						directions = append(directions, "↑")
					} else if k.Close < k.Open {
						directions = append(directions, "↓")
					} else {
						directions = append(directions, "→")
					}
				}

				// Count direction changes (flips)
				for i := 1; i < len(directions); i++ {
					if directions[i] != directions[i-1] && directions[i] != "→" && directions[i-1] != "→" {
						flipCount++
					}
				}

				signal.TrendFlipCount = flipCount
				signal.TrendStable = flipCount == 0

				if !signal.TrendStable {
					signal.TrendAligned = false
					signal.StabilityReason = fmt.Sprintf("trend flipped %d times in last 3 candles [%s]",
						flipCount, strings.Join(directions, ""))
					signal.AlignmentReason = fmt.Sprintf("%s - UNSTABLE: %s", signal.AlignmentReason, signal.StabilityReason)
				} else {
					signal.StabilityReason = fmt.Sprintf("stable trend [%s]", strings.Join(directions, ""))
				}
			} else {
				// Default to stable if we can't check
				signal.TrendStable = true
				signal.StabilityReason = "insufficient data for stability check"
			}
		} else if !settings.UltraFastTrendStabilityCheck {
			signal.TrendStable = true
			signal.StabilityReason = "stability check disabled"
		}
	} else if settings.UltraFastTrendAlignmentEnabled {
		// Legacy: Simple 5m vs 1h alignment check
		klines5m, err := g.futuresClient.GetFuturesKlines(symbol, "5m", 10)
		if err == nil && len(klines5m) >= 3 {
			close5m := klines5m[len(klines5m)-1].Close
			close5mPrev := klines5m[len(klines5m)-2].Close
			priceDiff5mPct := ((close5m - close5mPrev) / close5mPrev) * 100.0

			if priceDiff5mPct >= 0.2 {
				signal.Trend5mBias, signal.Trend5mStrength = "LONG", 80
			} else if priceDiff5mPct >= 0.05 {
				signal.Trend5mBias, signal.Trend5mStrength = "LONG", 55
			} else if priceDiff5mPct <= -0.2 {
				signal.Trend5mBias, signal.Trend5mStrength = "SHORT", 80
			} else if priceDiff5mPct <= -0.05 {
				signal.Trend5mBias, signal.Trend5mStrength = "SHORT", 55
			} else {
				signal.Trend5mBias, signal.Trend5mStrength = "NEUTRAL", 40
			}

			signal.CombinedTrendStrength = signal.TrendStrength + signal.Trend5mStrength

			if signal.TrendBias == signal.Trend5mBias && signal.TrendBias != "NEUTRAL" {
				signal.TrendAligned = true
				signal.AlignmentReason = fmt.Sprintf("5m(%s) confirms 1h(%s)", signal.Trend5mBias, signal.TrendBias)
			} else {
				signal.TrendAligned = false
				signal.AlignmentReason = fmt.Sprintf("5m(%s) vs 1h(%s)", signal.Trend5mBias, signal.TrendBias)
			}
		} else {
			signal.TrendAligned = signal.TrendStrength >= 70
			signal.Trend5mBias = "UNKNOWN"
			signal.AlignmentReason = "5m data unavailable"
		}
	} else {
		// All alignment checks disabled
		signal.TrendAligned = true
		signal.AlignmentReason = "alignment check disabled"
	}

	// Layer 2: Volatility Regime classification
	regime, err := g.ClassifyVolatilityRegime(symbol)
	if err != nil {
		regime = &VolatilityRegime{
			Level:            "medium",
			ATRRatio:         1.0,
			BBWidthPercent:   4.0,
			ReEntryDelay:     5 * time.Second,
			MaxTradesPerHour: 12,
		}
	}
	signal.VolatilityRegime = regime

	// Layer 3: Entry Trigger from 1m candles
	klines1m, err := g.futuresClient.GetFuturesKlines(symbol, "1m", 10)
	if err != nil || len(klines1m) < 5 {
		// Can't evaluate entry trigger, return with NEUTRAL bias
		signal.EntryConfidence = 30
	} else {
		// Count bullish candles in last 5 candles
		bullishCount := 0
		for i := len(klines1m) - 5; i < len(klines1m); i++ {
			open := klines1m[i].Open
			close := klines1m[i].Close
			if close > open {
				bullishCount++
			}
		}

		// Entry confidence based on candle alignment and trend strength
		// Strong trends get boosted confidence when 1m confirms
		// Weak trends can still enter if 1m candles strongly confirm
		if signal.TrendBias == "LONG" {
			if bullishCount >= 4 { // 4-5 bullish candles = very strong
				signal.EntryConfidence = 85
			} else if bullishCount >= 3 { // 3 bullish = strong
				if signal.TrendStrength >= 70 {
					signal.EntryConfidence = 80
				} else {
					signal.EntryConfidence = 70 // Weak trend but good 1m confirmation
				}
			} else {
				signal.EntryConfidence = 40 // Trend not confirmed by 1m
			}
		} else if signal.TrendBias == "SHORT" {
			if bullishCount <= 1 { // 0-1 bullish = very bearish
				signal.EntryConfidence = 85
			} else if bullishCount <= 2 { // 2 bullish = bearish
				if signal.TrendStrength >= 70 {
					signal.EntryConfidence = 80
				} else {
					signal.EntryConfidence = 70 // Weak trend but good 1m confirmation
				}
			} else {
				signal.EntryConfidence = 40 // Trend not confirmed by 1m
			}
		} else if signal.TrendBias == "NEUTRAL" {
			signal.EntryConfidence = 50
		}
	}

	// Layer 4: Dynamic profit target based on fees and volatility
	// Get ATR for profit calculation
	klines5m, err := g.futuresClient.GetFuturesKlines(symbol, "5m", 20)
	if err != nil || len(klines5m) < 14 {
		signal.MinProfitTarget = 1.0 // Default 1%
	} else {
		atrPercent := 1.0 // Default
		// Simple ATR calculation for TP
		highs := make([]float64, 0)
		for _, k := range klines5m {
			highs = append(highs, k.High)
		}
		sort.Float64s(highs)
		if len(highs) > 0 {
			avgRange := (highs[len(highs)-1] - highs[0]) / float64(len(highs))
			lastClose := klines5m[len(klines5m)-1].Close
			atrPercent = (avgRange / lastClose) * 100
		}

		signal.MinProfitTarget = g.CalculateFeeAwareTP(symbol, 200, 10, atrPercent)
	}

	// Set max hold time to 3 seconds for ultra-fast
	signal.MaxHoldTime = 3 * time.Second

	// Apply signal quality filters
	g.applyUltraFastQualityFilters(signal, klines1m)

	return signal, nil
}

// applyUltraFastQualityFilters applies volume, momentum, and candle body filters to the signal
func (g *GinieAnalyzer) applyUltraFastQualityFilters(signal *UltraFastSignal, klines1m []binance.Kline) {
	settings := GetSettingsManager().GetCurrentSettings()
	filtersApplied := []string{}
	filtersFailed := []string{}

	// Volume Confirmation Filter
	if settings.UltraFastVolumeFilterEnabled {
		threshold := settings.UltraFastVolumeMultiplier
		if threshold <= 0 {
			threshold = 1.5
		}
		confirmed, multiplier := g.checkUltraFastVolumeConfirmation(klines1m, threshold)
		signal.VolumeMultiplier = multiplier
		signal.VolumeConfirmed = confirmed
		if confirmed {
			filtersApplied = append(filtersApplied, fmt.Sprintf("Volume:%.2fx", multiplier))
		} else {
			filtersFailed = append(filtersFailed, fmt.Sprintf("Volume:%.2fx<%.2fx", multiplier, threshold))
		}
	} else {
		signal.VolumeConfirmed = true // Disabled = always pass
	}

	// Momentum Strength Filter
	if settings.UltraFastMomentumFilterEnabled {
		minMomentum := settings.UltraFastMinMomentum
		if minMomentum <= 0 {
			minMomentum = 0.05
		}
		confirmed, momentum := g.checkUltraFastMomentumStrength(klines1m, minMomentum)
		signal.MomentumStrength = momentum
		signal.MomentumConfirmed = confirmed
		if confirmed {
			filtersApplied = append(filtersApplied, fmt.Sprintf("Momentum:%.3f%%", momentum))
		} else {
			filtersFailed = append(filtersFailed, fmt.Sprintf("Momentum:%.3f%%<%.3f%%", momentum, minMomentum))
		}
	} else {
		signal.MomentumConfirmed = true // Disabled = always pass
	}

	// Candle Body Size Filter
	if settings.UltraFastCandleBodyFilterEnabled {
		minBodyPct := settings.UltraFastMinCandleBodyPct
		if minBodyPct <= 0 {
			minBodyPct = 0.1
		}
		confirmed, avgBody := g.checkUltraFastCandleBodySize(klines1m, minBodyPct, 3)
		signal.AvgCandleBodyPct = avgBody
		signal.CandleBodyConfirmed = confirmed
		if confirmed {
			filtersApplied = append(filtersApplied, fmt.Sprintf("Body:%.3f%%", avgBody))
		} else {
			filtersFailed = append(filtersFailed, fmt.Sprintf("Body:%.3f%%<%.3f%%", avgBody, minBodyPct))
		}
	} else {
		signal.CandleBodyConfirmed = true // Disabled = always pass
	}

	// Trend strength filter
	minTrendStrength := settings.UltraFastMinTrendStrength
	if minTrendStrength <= 0 {
		minTrendStrength = 60.0
	}
	if signal.TrendStrength >= minTrendStrength {
		filtersApplied = append(filtersApplied, fmt.Sprintf("Trend:%.1f", signal.TrendStrength))
	} else {
		filtersFailed = append(filtersFailed, fmt.Sprintf("Trend:%.1f<%.1f", signal.TrendStrength, minTrendStrength))
	}

	signal.FiltersApplied = filtersApplied
	signal.FiltersFailed = filtersFailed

	// Reduce confidence based on failed filters (10% penalty per failed filter)
	if len(filtersFailed) > 0 {
		penalty := float64(len(filtersFailed)) * 10.0
		signal.EntryConfidence -= penalty
		if signal.EntryConfidence < 0 {
			signal.EntryConfidence = 0
		}
	}
}

// checkUltraFastVolumeConfirmation checks if 1m volume is above threshold
// Returns (confirmed bool, volumeMultiplier float64)
func (g *GinieAnalyzer) checkUltraFastVolumeConfirmation(klines1m []binance.Kline, threshold float64) (bool, float64) {
	if len(klines1m) < 6 {
		return false, 0
	}

	// Current candle volume
	currentVolume := klines1m[len(klines1m)-1].Volume

	// Calculate average of previous 5 candles
	avgVolume := 0.0
	for i := len(klines1m) - 6; i < len(klines1m)-1; i++ {
		avgVolume += klines1m[i].Volume
	}
	avgVolume /= 5

	if avgVolume == 0 {
		return false, 0
	}

	multiplier := currentVolume / avgVolume
	return multiplier >= threshold, multiplier
}

// checkUltraFastMomentumStrength calculates price momentum from recent 1m candles
// Returns (confirmed bool, momentumPct float64)
func (g *GinieAnalyzer) checkUltraFastMomentumStrength(klines1m []binance.Kline, minMomentumPct float64) (bool, float64) {
	if len(klines1m) < 3 {
		return false, 0
	}

	// Calculate momentum as % change over last 3 candles
	startPrice := klines1m[len(klines1m)-3].Open
	endPrice := klines1m[len(klines1m)-1].Close

	if startPrice == 0 {
		return false, 0
	}

	momentumPct := math.Abs((endPrice-startPrice)/startPrice) * 100
	return momentumPct >= minMomentumPct, momentumPct
}

// checkUltraFastCandleBodySize validates candles have sufficient body (not doji)
// Returns (confirmed bool, avgBodyPct float64)
func (g *GinieAnalyzer) checkUltraFastCandleBodySize(klines1m []binance.Kline, minBodyPct float64, count int) (bool, float64) {
	if len(klines1m) < count {
		return false, 0
	}

	totalBodyPct := 0.0
	validCount := 0

	// Check last 'count' candles
	for i := len(klines1m) - count; i < len(klines1m); i++ {
		k := klines1m[i]
		if k.Open == 0 {
			continue
		}
		bodyPct := math.Abs((k.Close-k.Open)/k.Open) * 100
		totalBodyPct += bodyPct
		validCount++
	}

	if validCount == 0 {
		return false, 0
	}

	avgBodyPct := totalBodyPct / float64(validCount)
	return avgBodyPct >= minBodyPct, avgBodyPct
}

// calculateLLMPositionSize calculates an AI-suggested position size based on market conditions
// This considers volatility, confidence, mode, and market sentiment to suggest optimal sizing
// Returns (suggestedSizeUSD float64, reasoning string)
func (g *GinieAnalyzer) calculateLLMPositionSize(symbol string, mode GinieTradingMode, confidence float64, volatility string, atrPct float64, sentiment string) (float64, string) {
	// Get mode-specific base size from settings
	var baseSizeUSD float64 = 50.0 // Default
	var maxSizeUSD float64 = 100.0 // Default max

	if g.settings != nil {
		modeKey := string(mode)
		if modeConfig, ok := g.settings.ModeConfigs[modeKey]; ok && modeConfig != nil && modeConfig.Size != nil {
			if modeConfig.Size.BaseSizeUSD > 0 {
				baseSizeUSD = modeConfig.Size.BaseSizeUSD
			}
			if modeConfig.Size.MaxSizeUSD > 0 {
				maxSizeUSD = modeConfig.Size.MaxSizeUSD
			}
		}
	}

	// AI-driven size calculation factors:
	// 1. Volatility adjustment (high vol = smaller size)
	volMultiplier := 1.0
	switch volatility {
	case "Low":
		volMultiplier = 1.2 // Can be more aggressive in low vol
	case "Medium":
		volMultiplier = 1.0 // Standard
	case "High":
		volMultiplier = 0.7 // Reduce size in high vol
	case "Extreme":
		volMultiplier = 0.4 // Significantly reduce in extreme vol
	}

	// 2. Confidence adjustment (higher confidence = larger size)
	// Scale: 50% conf = 0.7x, 70% = 1.0x, 90% = 1.3x
	confMultiplier := 0.5 + (confidence / 100.0 * 0.8)
	if confMultiplier > 1.5 {
		confMultiplier = 1.5
	}

	// 3. Sentiment adjustment
	sentMultiplier := 1.0
	switch sentiment {
	case "strongly_bullish", "strongly_bearish":
		sentMultiplier = 1.1 // Slight boost when sentiment aligns
	case "bullish", "bearish":
		sentMultiplier = 1.0
	case "neutral", "mixed":
		sentMultiplier = 0.9 // Slightly reduce when unclear
	}

	// 4. ATR-based volatility fine-tuning
	// If ATR% > 3%, reduce size; if < 1%, increase slightly
	atrMultiplier := 1.0
	if atrPct > 5.0 {
		atrMultiplier = 0.5 // Very volatile
	} else if atrPct > 3.0 {
		atrMultiplier = 0.7
	} else if atrPct > 2.0 {
		atrMultiplier = 0.85
	} else if atrPct < 1.0 {
		atrMultiplier = 1.15 // Low volatility, can size up
	}

	// Calculate final size
	suggestedSize := baseSizeUSD * volMultiplier * confMultiplier * sentMultiplier * atrMultiplier

	// Ensure within bounds
	if suggestedSize < 10.0 {
		suggestedSize = 10.0 // Minimum viable size
	}
	if suggestedSize > maxSizeUSD {
		suggestedSize = maxSizeUSD
	}

	// Build reasoning string
	reasoning := fmt.Sprintf("AI sizing: base=$%.0f, vol=%s(%.2fx), conf=%.0f%%(%.2fx), sent=%s(%.2fx), atr=%.1f%%(%.2fx) → $%.2f",
		baseSizeUSD, volatility, volMultiplier, confidence, confMultiplier, sentiment, sentMultiplier, atrPct, atrMultiplier, suggestedSize)

	if g.logger != nil {
		g.logger.Debug("LLM position size calculated",
			"symbol", symbol,
			"mode", mode,
			"base_size_usd", baseSizeUSD,
			"vol_multiplier", volMultiplier,
			"conf_multiplier", confMultiplier,
			"sent_multiplier", sentMultiplier,
			"atr_multiplier", atrMultiplier,
			"suggested_size_usd", suggestedSize)
	}

	return suggestedSize, reasoning
}

// detectFairValueGaps identifies Fair Value Gaps (FVGs) in price action
// FVG occurs when candle 2's body doesn't overlap with candle 1 and candle 3's wicks
// Bullish FVG: gap between candle 1 high and candle 3 low (price should fill up)
// Bearish FVG: gap between candle 1 low and candle 3 high (price should fill down)
func (g *GinieAnalyzer) detectFairValueGaps(klines []binance.Kline, currentPrice float64) FVGAnalysis {
	analysis := FVGAnalysis{
		BullishFVGs: []FairValueGap{},
		BearishFVGs: []FairValueGap{},
	}

	if len(klines) < 3 {
		return analysis
	}

	// Look at last 50 candles for FVGs
	lookback := 50
	if len(klines) < lookback {
		lookback = len(klines)
	}

	for i := lookback - 1; i >= 2; i-- {
		candle1 := klines[i-2] // First candle
		candle2 := klines[i-1] // Middle candle (the big move)
		candle3 := klines[i]   // Third candle

		// Check for Bullish FVG: gap between candle1 high and candle3 low
		if candle3.Low > candle1.High {
			gapSize := candle3.Low - candle1.High
			gapPercent := (gapSize / currentPrice) * 100
			midPrice := (candle3.Low + candle1.High) / 2

			// Only consider significant gaps (> 0.1%)
			if gapPercent > 0.1 {
				fvg := FairValueGap{
					Type:        "bullish",
					TopPrice:    candle3.Low,
					BottomPrice: candle1.High,
					MidPrice:    midPrice,
					GapSize:     gapSize,
					GapPercent:  gapPercent,
					CandleIndex: i,
					Timestamp:   time.Unix(candle2.OpenTime/1000, 0),
					Filled:      currentPrice <= candle1.High, // Price has come back to fill
					Tested:      currentPrice >= candle1.High && currentPrice <= candle3.Low,
					Strength:    g.classifyFVGStrength(gapPercent),
				}
				analysis.BullishFVGs = append(analysis.BullishFVGs, fvg)
			}
		}

		// Check for Bearish FVG: gap between candle1 low and candle3 high
		if candle3.High < candle1.Low {
			gapSize := candle1.Low - candle3.High
			gapPercent := (gapSize / currentPrice) * 100
			midPrice := (candle1.Low + candle3.High) / 2

			// Only consider significant gaps (> 0.1%)
			if gapPercent > 0.1 {
				fvg := FairValueGap{
					Type:        "bearish",
					TopPrice:    candle1.Low,
					BottomPrice: candle3.High,
					MidPrice:    midPrice,
					GapSize:     gapSize,
					GapPercent:  gapPercent,
					CandleIndex: i,
					Timestamp:   time.Unix(candle2.OpenTime/1000, 0),
					Filled:      currentPrice >= candle1.Low, // Price has come back to fill
					Tested:      currentPrice <= candle1.Low && currentPrice >= candle3.High,
					Strength:    g.classifyFVGStrength(gapPercent),
				}
				analysis.BearishFVGs = append(analysis.BearishFVGs, fvg)
			}
		}
	}

	// Find nearest unfilled FVGs to current price
	for i := range analysis.BullishFVGs {
		if !analysis.BullishFVGs[i].Filled {
			if analysis.NearestBullish == nil ||
				analysis.BullishFVGs[i].TopPrice > analysis.NearestBullish.TopPrice {
				fvg := analysis.BullishFVGs[i]
				analysis.NearestBullish = &fvg
			}
		}
	}

	for i := range analysis.BearishFVGs {
		if !analysis.BearishFVGs[i].Filled {
			if analysis.NearestBearish == nil ||
				analysis.BearishFVGs[i].BottomPrice < analysis.NearestBearish.BottomPrice {
				fvg := analysis.BearishFVGs[i]
				analysis.NearestBearish = &fvg
			}
		}
	}

	// Count unfilled FVGs
	for _, fvg := range analysis.BullishFVGs {
		if !fvg.Filled {
			analysis.TotalUnfilled++
		}
	}
	for _, fvg := range analysis.BearishFVGs {
		if !fvg.Filled {
			analysis.TotalUnfilled++
		}
	}

	// Check if current price is in an FVG zone
	for _, fvg := range analysis.BullishFVGs {
		if !fvg.Filled && currentPrice >= fvg.BottomPrice && currentPrice <= fvg.TopPrice {
			analysis.InFVGZone = true
			analysis.FVGZoneType = "bullish"
			break
		}
	}
	if !analysis.InFVGZone {
		for _, fvg := range analysis.BearishFVGs {
			if !fvg.Filled && currentPrice >= fvg.BottomPrice && currentPrice <= fvg.TopPrice {
				analysis.InFVGZone = true
				analysis.FVGZoneType = "bearish"
				break
			}
		}
	}

	return analysis
}

// classifyFVGStrength classifies FVG strength based on gap percentage
func (g *GinieAnalyzer) classifyFVGStrength(gapPercent float64) string {
	if gapPercent > 1.0 {
		return "strong"
	} else if gapPercent > 0.5 {
		return "medium"
	}
	return "weak"
}

// detectOrderBlocks identifies Order Blocks in price action
// Bullish OB: Last bearish candle before a strong bullish move (demand zone)
// Bearish OB: Last bullish candle before a strong bearish move (supply zone)
func (g *GinieAnalyzer) detectOrderBlocks(klines []binance.Kline, currentPrice float64) OrderBlockAnalysis {
	analysis := OrderBlockAnalysis{
		BullishOBs: []OrderBlock{},
		BearishOBs: []OrderBlock{},
	}

	if len(klines) < 5 {
		return analysis
	}

	// Look at last 100 candles for Order Blocks
	lookback := 100
	if len(klines) < lookback {
		lookback = len(klines)
	}

	// Threshold for "strong move" - at least 1% move in following candles
	strongMoveThreshold := 1.0

	for i := lookback - 1; i >= 3; i-- {
		candle := klines[i]
		isBullish := candle.Close > candle.Open
		isBearish := candle.Close < candle.Open

		// Calculate the subsequent move (next 3 candles)
		maxHigh := candle.High
		minLow := candle.Low
		for j := i + 1; j < i+4 && j < len(klines); j++ {
			if klines[j].High > maxHigh {
				maxHigh = klines[j].High
			}
			if klines[j].Low < minLow {
				minLow = klines[j].Low
			}
		}

		// Check for Bullish Order Block (bearish candle before bullish move)
		if isBearish {
			moveUp := ((maxHigh - candle.High) / candle.High) * 100
			if moveUp >= strongMoveThreshold {
				ob := OrderBlock{
					Type:        "bullish",
					HighPrice:   candle.High,
					LowPrice:    candle.Low,
					MidPrice:    (candle.High + candle.Low) / 2,
					OpenPrice:   candle.Open,
					ClosePrice:  candle.Close,
					Volume:      candle.Volume,
					CandleIndex: i,
					Timestamp:   time.Unix(candle.OpenTime/1000, 0),
					Mitigated:   currentPrice < candle.Low, // Price has gone through the OB
					Tested:      currentPrice >= candle.Low && currentPrice <= candle.High,
					Strength:    g.classifyOBStrength(moveUp),
					MovePercent: moveUp,
				}
				// Count tests (how many times price touched this zone)
				for j := i + 1; j < len(klines); j++ {
					if klines[j].Low <= candle.High && klines[j].High >= candle.Low {
						ob.TestCount++
					}
				}
				analysis.BullishOBs = append(analysis.BullishOBs, ob)
			}
		}

		// Check for Bearish Order Block (bullish candle before bearish move)
		if isBullish {
			moveDown := ((candle.Low - minLow) / candle.Low) * 100
			if moveDown >= strongMoveThreshold {
				ob := OrderBlock{
					Type:        "bearish",
					HighPrice:   candle.High,
					LowPrice:    candle.Low,
					MidPrice:    (candle.High + candle.Low) / 2,
					OpenPrice:   candle.Open,
					ClosePrice:  candle.Close,
					Volume:      candle.Volume,
					CandleIndex: i,
					Timestamp:   time.Unix(candle.OpenTime/1000, 0),
					Mitigated:   currentPrice > candle.High, // Price has gone through the OB
					Tested:      currentPrice >= candle.Low && currentPrice <= candle.High,
					Strength:    g.classifyOBStrength(moveDown),
					MovePercent: moveDown,
				}
				// Count tests
				for j := i + 1; j < len(klines); j++ {
					if klines[j].Low <= candle.High && klines[j].High >= candle.Low {
						ob.TestCount++
					}
				}
				analysis.BearishOBs = append(analysis.BearishOBs, ob)
			}
		}
	}

	// Find nearest unmitigated Order Blocks
	for i := range analysis.BullishOBs {
		if !analysis.BullishOBs[i].Mitigated {
			if analysis.NearestBullish == nil ||
				analysis.BullishOBs[i].HighPrice > analysis.NearestBullish.HighPrice {
				ob := analysis.BullishOBs[i]
				analysis.NearestBullish = &ob
			}
		}
	}

	for i := range analysis.BearishOBs {
		if !analysis.BearishOBs[i].Mitigated {
			if analysis.NearestBearish == nil ||
				analysis.BearishOBs[i].LowPrice < analysis.NearestBearish.LowPrice {
				ob := analysis.BearishOBs[i]
				analysis.NearestBearish = &ob
			}
		}
	}

	// Count unmitigated OBs
	for _, ob := range analysis.BullishOBs {
		if !ob.Mitigated {
			analysis.TotalUnmitigated++
		}
	}
	for _, ob := range analysis.BearishOBs {
		if !ob.Mitigated {
			analysis.TotalUnmitigated++
		}
	}

	// Check if current price is in an OB zone
	for _, ob := range analysis.BullishOBs {
		if !ob.Mitigated && currentPrice >= ob.LowPrice && currentPrice <= ob.HighPrice {
			analysis.InOBZone = true
			analysis.OBZoneType = "bullish"
			break
		}
	}
	if !analysis.InOBZone {
		for _, ob := range analysis.BearishOBs {
			if !ob.Mitigated && currentPrice >= ob.LowPrice && currentPrice <= ob.HighPrice {
				analysis.InOBZone = true
				analysis.OBZoneType = "bearish"
				break
			}
		}
	}

	return analysis
}

// classifyOBStrength classifies Order Block strength based on the move percentage
func (g *GinieAnalyzer) classifyOBStrength(movePercent float64) string {
	if movePercent > 3.0 {
		return "strong"
	} else if movePercent > 1.5 {
		return "medium"
	}
	return "weak"
}

// analyzePriceAction performs complete price action analysis including FVG, Order Blocks, and Chart Patterns
func (g *GinieAnalyzer) analyzePriceAction(klines []binance.Kline, currentPrice float64, tradeDirection string) PriceActionAnalysis {
	fvgAnalysis := g.detectFairValueGaps(klines, currentPrice)
	obAnalysis := g.detectOrderBlocks(klines, currentPrice)
	chartPatterns := g.detectChartPatterns(klines, currentPrice)

	analysis := PriceActionAnalysis{
		FVG:           fvgAnalysis,
		OrderBlocks:   obAnalysis,
		ChartPatterns: chartPatterns,
	}

	// Calculate confluence score (0-100)
	confluenceScore := 0.0

	// Check for bullish setup
	hasBullishFVG := fvgAnalysis.NearestBullish != nil && !fvgAnalysis.NearestBullish.Filled
	hasBullishOB := obAnalysis.NearestBullish != nil && !obAnalysis.NearestBullish.Mitigated
	inBullishFVG := fvgAnalysis.InFVGZone && fvgAnalysis.FVGZoneType == "bullish"
	inBullishOB := obAnalysis.InOBZone && obAnalysis.OBZoneType == "bullish"

	if tradeDirection == "long" || tradeDirection == "" {
		if hasBullishFVG {
			confluenceScore += 15
			if fvgAnalysis.NearestBullish.Strength == "strong" {
				confluenceScore += 10
			}
		}
		if hasBullishOB {
			confluenceScore += 20
			if obAnalysis.NearestBullish.Strength == "strong" {
				confluenceScore += 10
			}
			if obAnalysis.NearestBullish.TestCount == 0 {
				confluenceScore += 5 // Fresh OB is stronger
			}
		}
		if inBullishFVG {
			confluenceScore += 15 // Currently in the FVG zone
		}
		if inBullishOB {
			confluenceScore += 20 // Currently in the OB zone
		}
		// Confluence bonus: FVG and OB overlap
		if hasBullishFVG && hasBullishOB {
			fvgTop := fvgAnalysis.NearestBullish.TopPrice
			fvgBottom := fvgAnalysis.NearestBullish.BottomPrice
			obTop := obAnalysis.NearestBullish.HighPrice
			obBottom := obAnalysis.NearestBullish.LowPrice
			// Check if zones overlap
			if fvgBottom <= obTop && fvgTop >= obBottom {
				confluenceScore += 20 // Strong confluence
				analysis.FVG.FVGConfluence = true
				analysis.OrderBlocks.OBConfluence = true
			}
		}
		analysis.HasBullishSetup = confluenceScore > 30
	}

	// Check for bearish setup
	hasBearishFVG := fvgAnalysis.NearestBearish != nil && !fvgAnalysis.NearestBearish.Filled
	hasBearishOB := obAnalysis.NearestBearish != nil && !obAnalysis.NearestBearish.Mitigated
	inBearishFVG := fvgAnalysis.InFVGZone && fvgAnalysis.FVGZoneType == "bearish"
	inBearishOB := obAnalysis.InOBZone && obAnalysis.OBZoneType == "bearish"

	if tradeDirection == "short" || tradeDirection == "" {
		bearishScore := 0.0
		if hasBearishFVG {
			bearishScore += 15
			if fvgAnalysis.NearestBearish.Strength == "strong" {
				bearishScore += 10
			}
		}
		if hasBearishOB {
			bearishScore += 20
			if obAnalysis.NearestBearish.Strength == "strong" {
				bearishScore += 10
			}
			if obAnalysis.NearestBearish.TestCount == 0 {
				bearishScore += 5
			}
		}
		if inBearishFVG {
			bearishScore += 15
		}
		if inBearishOB {
			bearishScore += 20
		}
		// Confluence bonus
		if hasBearishFVG && hasBearishOB {
			fvgTop := fvgAnalysis.NearestBearish.TopPrice
			fvgBottom := fvgAnalysis.NearestBearish.BottomPrice
			obTop := obAnalysis.NearestBearish.HighPrice
			obBottom := obAnalysis.NearestBearish.LowPrice
			if fvgBottom <= obTop && fvgTop >= obBottom {
				bearishScore += 20
				analysis.FVG.FVGConfluence = true
				analysis.OrderBlocks.OBConfluence = true
			}
		}
		if tradeDirection == "short" {
			confluenceScore = bearishScore
		} else if bearishScore > confluenceScore {
			confluenceScore = bearishScore
		}
		analysis.HasBearishSetup = bearishScore > 30
	}

	// Add chart pattern confluence
	if chartPatterns.TotalPatterns > 0 {
		// Check if chart patterns align with trade direction
		if (tradeDirection == "long" || tradeDirection == "") && chartPatterns.HasBullishPattern {
			confluenceScore += chartPatterns.PatternScore * 0.3 // Up to 30 points bonus
			analysis.ChartPatterns.PatternConfluence = true
		}
		if (tradeDirection == "short" || tradeDirection == "") && chartPatterns.HasBearishPattern {
			confluenceScore += chartPatterns.PatternScore * 0.3
			analysis.ChartPatterns.PatternConfluence = true
		}
		// Near breakout bonus
		if chartPatterns.NearBreakout {
			confluenceScore += 10
		}
	}

	analysis.ConfluenceScore = confluenceScore

	// Classify setup quality
	if confluenceScore >= 70 {
		analysis.SetupQuality = "excellent"
	} else if confluenceScore >= 50 {
		analysis.SetupQuality = "good"
	} else if confluenceScore >= 30 {
		analysis.SetupQuality = "moderate"
	} else {
		analysis.SetupQuality = "weak"
	}

	return analysis
}
