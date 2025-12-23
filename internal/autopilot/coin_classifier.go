package autopilot

import (
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/strategy"
	"context"
	"sort"
	"sync"
	"time"
)

// CoinClassifier classifies coins by volatility, market cap, and momentum
type CoinClassifier struct {
	futuresClient binance.FuturesClient
	logger        *logging.Logger
	settings      *CoinClassificationSettings

	// Cache
	classifications map[string]*CoinClassification
	summary         *ClassificationSummary
	lastRefresh     time.Time
	mu              sync.RWMutex

	// Background refresh
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	refreshWg  sync.WaitGroup
}

// NewCoinClassifier creates a new coin classifier
func NewCoinClassifier(
	futuresClient binance.FuturesClient,
	logger *logging.Logger,
) *CoinClassifier {
	ctx, cancel := context.WithCancel(context.Background())
	return &CoinClassifier{
		futuresClient:   futuresClient,
		logger:          logger,
		settings:        NewDefaultCoinClassificationSettings(),
		classifications: make(map[string]*CoinClassification),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// SetSettings updates the classification settings
func (cc *CoinClassifier) SetSettings(settings *CoinClassificationSettings) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.settings = settings
}

// GetSettings returns the current settings
func (cc *CoinClassifier) GetSettings() *CoinClassificationSettings {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.settings
}

// Start begins background classification refresh
func (cc *CoinClassifier) Start() {
	cc.mu.Lock()
	if cc.running {
		cc.mu.Unlock()
		return
	}
	cc.running = true
	cc.mu.Unlock()

	cc.refreshWg.Add(1)
	go cc.refreshLoop()

	cc.logger.Info("Coin classifier started")
}

// Stop stops the background refresh
func (cc *CoinClassifier) Stop() {
	cc.mu.Lock()
	if !cc.running {
		cc.mu.Unlock()
		return
	}
	cc.running = false
	cc.mu.Unlock()

	cc.cancel()
	cc.refreshWg.Wait()

	cc.logger.Info("Coin classifier stopped")
}

// refreshLoop periodically refreshes classifications
func (cc *CoinClassifier) refreshLoop() {
	defer cc.refreshWg.Done()

	// Initial refresh
	cc.RefreshAllClassifications()

	cc.mu.RLock()
	interval := time.Duration(cc.settings.RefreshIntervalSecs) * time.Second
	cc.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-cc.ctx.Done():
			return
		case <-ticker.C:
			cc.RefreshAllClassifications()
		}
	}
}

// RefreshAllClassifications refreshes all coin classifications
func (cc *CoinClassifier) RefreshAllClassifications() {
	cc.logger.Debug("Refreshing coin classifications")

	// Get all available symbols
	symbols, err := cc.futuresClient.GetFuturesSymbols()
	if err != nil {
		cc.logger.Error("Failed to get futures symbols", "error", err)
		return
	}

	// Get all 24hr tickers
	tickers, err := cc.futuresClient.GetAll24hrTickers()
	if err != nil {
		cc.logger.Error("Failed to get 24hr tickers", "error", err)
		return
	}

	// Create ticker map for quick lookup
	tickerMap := make(map[string]*binance.Futures24hrTicker)
	for i := range tickers {
		tickerMap[tickers[i].Symbol] = &tickers[i]
	}

	// Classify each symbol in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	newClassifications := make(map[string]*CoinClassification)

	cc.mu.RLock()
	settings := cc.settings
	cc.mu.RUnlock()

	for _, symbol := range symbols {
		// Only classify USDT perpetuals
		if len(symbol) < 4 || symbol[len(symbol)-4:] != "USDT" {
			continue
		}

		ticker, exists := tickerMap[symbol]
		if !exists {
			continue
		}

		// Skip low volume coins
		if ticker.QuoteVolume < settings.MinVolume24h {
			continue
		}

		wg.Add(1)
		go func(sym string, tick *binance.Futures24hrTicker) {
			defer wg.Done()

			classification := cc.classifySymbol(sym, tick, settings)
			if classification != nil {
				mu.Lock()
				newClassifications[sym] = classification
				mu.Unlock()
			}
		}(symbol, ticker)
	}

	wg.Wait()

	// Update cache
	cc.mu.Lock()
	cc.classifications = newClassifications
	cc.lastRefresh = time.Now()
	cc.summary = cc.buildSummary(newClassifications)
	cc.mu.Unlock()

	cc.logger.Info("Coin classifications refreshed",
		"total", len(newClassifications),
		"enabled", cc.countEnabled(newClassifications))
}

// classifySymbol classifies a single symbol
func (cc *CoinClassifier) classifySymbol(
	symbol string,
	ticker *binance.Futures24hrTicker,
	settings *CoinClassificationSettings,
) *CoinClassification {
	// Calculate ATR for volatility
	atrPercent := cc.calculateATRPercent(symbol, ticker.LastPrice, settings)

	// Get classifications
	volatility := GetVolatilityClass(atrPercent, settings)
	marketCap := GetMarketCapClass(symbol)
	momentum := GetMomentumClass(ticker.PriceChangePercent, settings)

	classification := &CoinClassification{
		Symbol:         symbol,
		LastPrice:      ticker.LastPrice,
		Volatility:     volatility,
		VolatilityATR:  atrPercent,
		MarketCap:      marketCap,
		Momentum:       momentum,
		Momentum24hPct: ticker.PriceChangePercent,
		Volume24h:      ticker.Volume,
		QuoteVolume24h: ticker.QuoteVolume,
		LastUpdated:    time.Now(),
		Enabled:        true,
	}

	// Calculate scores
	classification.RiskScore = CalculateRiskScore(classification)
	classification.OpportunityScore = CalculateOpportunityScore(classification)

	// Check if eligible based on settings
	classification.Enabled = classification.IsEligible(settings)

	return classification
}

// calculateATRPercent calculates ATR as a percentage of current price
func (cc *CoinClassifier) calculateATRPercent(symbol string, currentPrice float64, settings *CoinClassificationSettings) float64 {
	// Fetch klines for ATR calculation
	klines, err := cc.futuresClient.GetFuturesKlines(symbol, settings.ATRTimeframe, settings.ATRPeriod+1)
	if err != nil || len(klines) < settings.ATRPeriod+1 {
		// Default to medium volatility if we can't calculate
		return 4.5
	}

	// Calculate ATR
	atr := strategy.CalculateATR(klines, settings.ATRPeriod)

	// Convert to percentage
	if currentPrice > 0 {
		return (atr / currentPrice) * 100
	}
	return 4.5
}

// GetClassification returns the classification for a symbol
func (cc *CoinClassifier) GetClassification(symbol string) *CoinClassification {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.classifications[symbol]
}

// GetAllClassifications returns all classifications
func (cc *CoinClassifier) GetAllClassifications() map[string]*CoinClassification {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	// Return a copy to prevent concurrent modification
	result := make(map[string]*CoinClassification, len(cc.classifications))
	for k, v := range cc.classifications {
		result[k] = v
	}
	return result
}

// GetEligibleSymbols returns symbols that pass all filters
func (cc *CoinClassifier) GetEligibleSymbols() []string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var eligible []string
	for symbol, classification := range cc.classifications {
		if classification.Enabled {
			eligible = append(eligible, symbol)
		}
	}

	// Sort by opportunity score descending
	sort.Slice(eligible, func(i, j int) bool {
		ci := cc.classifications[eligible[i]]
		cj := cc.classifications[eligible[j]]
		return ci.OpportunityScore > cj.OpportunityScore
	})

	return eligible
}

// GetSymbolsByVolatility returns symbols in a volatility class
func (cc *CoinClassifier) GetSymbolsByVolatility(class VolatilityClass) []string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var result []string
	for symbol, classification := range cc.classifications {
		if classification.Volatility == class && classification.Enabled {
			result = append(result, symbol)
		}
	}
	return result
}

// GetSymbolsByMarketCap returns symbols in a market cap class
func (cc *CoinClassifier) GetSymbolsByMarketCap(class MarketCapClass) []string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var result []string
	for symbol, classification := range cc.classifications {
		if classification.MarketCap == class && classification.Enabled {
			result = append(result, symbol)
		}
	}
	return result
}

// GetSymbolsByMomentum returns symbols in a momentum class
func (cc *CoinClassifier) GetSymbolsByMomentum(class MomentumClass) []string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var result []string
	for symbol, classification := range cc.classifications {
		if classification.Momentum == class && classification.Enabled {
			result = append(result, symbol)
		}
	}
	return result
}

// GetSummary returns the classification summary
func (cc *CoinClassifier) GetSummary() *ClassificationSummary {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.summary
}

// buildSummary builds a summary of all classifications
func (cc *CoinClassifier) buildSummary(classifications map[string]*CoinClassification) *ClassificationSummary {
	summary := &ClassificationSummary{
		TotalSymbols:   len(classifications),
		ByVolatility:   make(map[VolatilityClass][]string),
		ByMarketCap:    make(map[MarketCapClass][]string),
		ByMomentum:     make(map[MomentumClass][]string),
		LastUpdated:    time.Now(),
	}

	// Initialize maps
	summary.ByVolatility[VolatilityStable] = []string{}
	summary.ByVolatility[VolatilityMedium] = []string{}
	summary.ByVolatility[VolatilityHigh] = []string{}
	summary.ByMarketCap[MarketCapBlueChip] = []string{}
	summary.ByMarketCap[MarketCapLarge] = []string{}
	summary.ByMarketCap[MarketCapMidSmall] = []string{}
	summary.ByMomentum[MomentumGainer] = []string{}
	summary.ByMomentum[MomentumNeutral] = []string{}
	summary.ByMomentum[MomentumLoser] = []string{}

	// Collect all classifications
	var all []CoinClassification
	for _, c := range classifications {
		all = append(all, *c)

		if c.Enabled {
			summary.EnabledSymbols++
		}

		summary.ByVolatility[c.Volatility] = append(summary.ByVolatility[c.Volatility], c.Symbol)
		summary.ByMarketCap[c.MarketCap] = append(summary.ByMarketCap[c.MarketCap], c.Symbol)
		summary.ByMomentum[c.Momentum] = append(summary.ByMomentum[c.Momentum], c.Symbol)
	}

	// Sort by 24h change for top gainers/losers
	sort.Slice(all, func(i, j int) bool {
		return all[i].Momentum24hPct > all[j].Momentum24hPct
	})

	// Top 5 gainers
	for i := 0; i < len(all) && i < 5; i++ {
		summary.TopGainers = append(summary.TopGainers, all[i])
	}

	// Top 5 losers (from the end)
	for i := len(all) - 1; i >= 0 && len(summary.TopLosers) < 5; i-- {
		summary.TopLosers = append(summary.TopLosers, all[i])
	}

	// Sort by volume for top volume
	sort.Slice(all, func(i, j int) bool {
		return all[i].QuoteVolume24h > all[j].QuoteVolume24h
	})

	for i := 0; i < len(all) && i < 5; i++ {
		summary.TopVolume = append(summary.TopVolume, all[i])
	}

	return summary
}

// countEnabled counts enabled symbols
func (cc *CoinClassifier) countEnabled(classifications map[string]*CoinClassification) int {
	count := 0
	for _, c := range classifications {
		if c.Enabled {
			count++
		}
	}
	return count
}

// UpdateCoinPreference updates preference for a specific coin
func (cc *CoinClassifier) UpdateCoinPreference(symbol string, enabled bool, priority int) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.settings.CoinPreferences == nil {
		cc.settings.CoinPreferences = make(map[string]*CoinPreference)
	}

	cc.settings.CoinPreferences[symbol] = &CoinPreference{
		Symbol:   symbol,
		Enabled:  enabled,
		Priority: priority,
	}

	// Update classification if exists
	if c, exists := cc.classifications[symbol]; exists {
		c.Enabled = enabled && c.IsEligible(cc.settings)
	}
}

// UpdateCategoryAllocation updates allocation for a category
func (cc *CoinClassifier) UpdateVolatilityAllocation(class VolatilityClass, alloc *CategoryAllocation) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.settings.VolatilityAllocations == nil {
		cc.settings.VolatilityAllocations = make(map[VolatilityClass]*CategoryAllocation)
	}
	cc.settings.VolatilityAllocations[class] = alloc
}

func (cc *CoinClassifier) UpdateMarketCapAllocation(class MarketCapClass, alloc *CategoryAllocation) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.settings.MarketCapAllocations == nil {
		cc.settings.MarketCapAllocations = make(map[MarketCapClass]*CategoryAllocation)
	}
	cc.settings.MarketCapAllocations[class] = alloc
}

func (cc *CoinClassifier) UpdateMomentumAllocation(class MomentumClass, alloc *CategoryAllocation) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.settings.MomentumAllocations == nil {
		cc.settings.MomentumAllocations = make(map[MomentumClass]*CategoryAllocation)
	}
	cc.settings.MomentumAllocations[class] = alloc
}

// GetAllocationForPosition determines how much to allocate for a position
// based on the coin's classifications
func (cc *CoinClassifier) GetAllocationForPosition(symbol string, totalBalance float64) float64 {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	c, exists := cc.classifications[symbol]
	if !exists || !c.Enabled {
		return 0
	}

	// Use the minimum of all category allocations
	minAllocation := 100.0

	if alloc, ok := cc.settings.VolatilityAllocations[c.Volatility]; ok && alloc.Enabled {
		if alloc.AllocationPercent < minAllocation {
			minAllocation = alloc.AllocationPercent
		}
	}

	if alloc, ok := cc.settings.MarketCapAllocations[c.MarketCap]; ok && alloc.Enabled {
		if alloc.AllocationPercent < minAllocation {
			minAllocation = alloc.AllocationPercent
		}
	}

	if alloc, ok := cc.settings.MomentumAllocations[c.Momentum]; ok && alloc.Enabled {
		if alloc.AllocationPercent < minAllocation {
			minAllocation = alloc.AllocationPercent
		}
	}

	// Calculate allocation amount
	return totalBalance * (minAllocation / 100.0)
}
