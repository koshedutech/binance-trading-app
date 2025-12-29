package binance

import (
	"log"
	"sync"
	"time"
)

// UserDataCache holds cached user data (positions, account info) with TTL
type UserDataCache struct {
	mu sync.RWMutex

	// Account info cache
	accountInfo       *FuturesAccountInfo
	accountInfoTime   time.Time
	accountInfoTTL    time.Duration

	// Positions cache
	positions         []FuturesPosition
	positionsTime     time.Time
	positionsTTL      time.Duration

	// Open orders cache
	openOrders        map[string][]FuturesOrder // keyed by symbol ("" for all)
	openOrdersTime    map[string]time.Time
	openOrdersTTL     time.Duration

	// Open algo orders cache
	openAlgoOrders    map[string][]AlgoOrder
	openAlgoOrdersTime map[string]time.Time
	openAlgoOrdersTTL time.Duration
}

// NewUserDataCache creates a new user data cache with configurable TTLs
func NewUserDataCache(accountTTL, positionsTTL, ordersTTL time.Duration) *UserDataCache {
	return &UserDataCache{
		accountInfoTTL:     accountTTL,
		positionsTTL:       positionsTTL,
		openOrdersTTL:      ordersTTL,
		openAlgoOrdersTTL:  ordersTTL,
		openOrders:         make(map[string][]FuturesOrder),
		openOrdersTime:     make(map[string]time.Time),
		openAlgoOrders:     make(map[string][]AlgoOrder),
		openAlgoOrdersTime: make(map[string]time.Time),
	}
}

// inFlightRequest tracks a pending API request to allow deduplication
type inFlightRequest struct {
	done   chan struct{}
	result []Kline
	err    error
}

// KlineCacheStats tracks cache performance metrics
type KlineCacheStats struct {
	Hits             int64   `json:"hits"`
	Misses           int64   `json:"misses"`
	Deduplicated     int64   `json:"deduplicated"`      // Requests that waited for in-flight
	PrefetchHits     int64   `json:"prefetch_hits"`     // Hits from prefetched data
	PrefetchRequests int64   `json:"prefetch_requests"` // Total prefetch calls
	HitRate          float64 `json:"hit_rate"`
	DedupeRate       float64 `json:"dedupe_rate"`
}

// CachedFuturesClient wraps a FuturesClient with cache-first logic for market data
// This reduces REST API calls by using WebSocket-populated cache
type CachedFuturesClient struct {
	client        FuturesClient
	cache         *MarketDataCache
	userDataCache *UserDataCache
	mu            sync.RWMutex

	// In-flight request deduplication for klines
	inFlightKlines   map[string]*inFlightRequest // "symbol:interval" -> pending request
	inFlightMu       sync.Mutex
	deduplicatedReqs int64
	prefetchHits     int64
	prefetchReqs     int64
}

// NewCachedFuturesClient creates a new cache-aware futures client wrapper
func NewCachedFuturesClient(client FuturesClient, cache *MarketDataCache) *CachedFuturesClient {
	return &CachedFuturesClient{
		client:         client,
		cache:          cache,
		userDataCache:  NewUserDataCache(5*time.Second, 5*time.Second, 3*time.Second),
		inFlightKlines: make(map[string]*inFlightRequest),
	}
}

// SetCache updates the cache reference
func (c *CachedFuturesClient) SetCache(cache *MarketDataCache) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = cache
}

// GetCache returns the current cache
func (c *CachedFuturesClient) GetCache() *MarketDataCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

// ==================== ACCOUNT (cached with short TTL to reduce API calls) ====================

func (c *CachedFuturesClient) GetFuturesAccountInfo() (*FuturesAccountInfo, error) {
	c.mu.RLock()
	udc := c.userDataCache
	c.mu.RUnlock()

	if udc != nil {
		udc.mu.RLock()
		if udc.accountInfo != nil && time.Since(udc.accountInfoTime) < udc.accountInfoTTL {
			result := udc.accountInfo
			udc.mu.RUnlock()
			return result, nil
		}
		udc.mu.RUnlock()
	}

	// Cache miss or expired - fetch from API
	result, err := c.client.GetFuturesAccountInfo()
	if err != nil {
		return nil, err
	}

	// Update cache
	if udc != nil {
		udc.mu.Lock()
		udc.accountInfo = result
		udc.accountInfoTime = time.Now()
		udc.mu.Unlock()
		log.Printf("[USER-DATA-CACHE] Account info cached (TTL: %v)", udc.accountInfoTTL)
	}

	return result, nil
}

func (c *CachedFuturesClient) GetPositions() ([]FuturesPosition, error) {
	c.mu.RLock()
	udc := c.userDataCache
	c.mu.RUnlock()

	if udc != nil {
		udc.mu.RLock()
		if udc.positions != nil && time.Since(udc.positionsTime) < udc.positionsTTL {
			result := make([]FuturesPosition, len(udc.positions))
			copy(result, udc.positions)
			udc.mu.RUnlock()
			return result, nil
		}
		udc.mu.RUnlock()
	}

	// Cache miss or expired - fetch from API
	result, err := c.client.GetPositions()
	if err != nil {
		return nil, err
	}

	// Update cache
	if udc != nil {
		udc.mu.Lock()
		udc.positions = result
		udc.positionsTime = time.Now()
		udc.mu.Unlock()
		log.Printf("[USER-DATA-CACHE] Positions cached (%d positions, TTL: %v)", len(result), udc.positionsTTL)
	}

	return result, nil
}

func (c *CachedFuturesClient) GetPositionBySymbol(symbol string) (*FuturesPosition, error) {
	// Use cached positions if available
	positions, err := c.GetPositions()
	if err != nil {
		return nil, err
	}

	for i := range positions {
		if positions[i].Symbol == symbol {
			if positions[i].PositionAmt != 0 {
				return &positions[i], nil
			}
		}
	}

	// No active position found - return first match or nil
	for i := range positions {
		if positions[i].Symbol == symbol {
			return &positions[i], nil
		}
	}

	// Fall back to direct API call for specific symbol
	return c.client.GetPositionBySymbol(symbol)
}

// InvalidateUserDataCache invalidates all user data caches (call after trades)
func (c *CachedFuturesClient) InvalidateUserDataCache() {
	c.mu.RLock()
	udc := c.userDataCache
	c.mu.RUnlock()

	if udc != nil {
		udc.mu.Lock()
		udc.accountInfo = nil
		udc.positions = nil
		udc.openOrders = make(map[string][]FuturesOrder)
		udc.openOrdersTime = make(map[string]time.Time)
		udc.openAlgoOrders = make(map[string][]AlgoOrder)
		udc.openAlgoOrdersTime = make(map[string]time.Time)
		udc.mu.Unlock()
		log.Printf("[USER-DATA-CACHE] Cache invalidated")
	}
}

// ==================== LEVERAGE & MARGIN (no caching) ====================

func (c *CachedFuturesClient) SetLeverage(symbol string, leverage int) (*LeverageResponse, error) {
	return c.client.SetLeverage(symbol, leverage)
}

func (c *CachedFuturesClient) SetMarginType(symbol string, marginType MarginType) error {
	return c.client.SetMarginType(symbol, marginType)
}

func (c *CachedFuturesClient) SetPositionMode(dualSidePosition bool) error {
	return c.client.SetPositionMode(dualSidePosition)
}

func (c *CachedFuturesClient) GetPositionMode() (*PositionModeResponse, error) {
	return c.client.GetPositionMode()
}

// ==================== TRADING (invalidates cache after changes) ====================

func (c *CachedFuturesClient) PlaceFuturesOrder(params FuturesOrderParams) (*FuturesOrderResponse, error) {
	result, err := c.client.PlaceFuturesOrder(params)
	if err == nil {
		c.InvalidateUserDataCache() // Order placed - invalidate cache
	}
	return result, err
}

func (c *CachedFuturesClient) CancelFuturesOrder(symbol string, orderId int64) error {
	err := c.client.CancelFuturesOrder(symbol, orderId)
	if err == nil {
		c.InvalidateUserDataCache() // Order cancelled - invalidate cache
	}
	return err
}

func (c *CachedFuturesClient) CancelAllFuturesOrders(symbol string) error {
	err := c.client.CancelAllFuturesOrders(symbol)
	if err == nil {
		c.InvalidateUserDataCache() // Orders cancelled - invalidate cache
	}
	return err
}

func (c *CachedFuturesClient) GetOpenOrders(symbol string) ([]FuturesOrder, error) {
	c.mu.RLock()
	udc := c.userDataCache
	c.mu.RUnlock()

	cacheKey := symbol // Empty string for all symbols

	if udc != nil {
		udc.mu.RLock()
		if orders, ok := udc.openOrders[cacheKey]; ok {
			if cacheTime, timeOk := udc.openOrdersTime[cacheKey]; timeOk && time.Since(cacheTime) < udc.openOrdersTTL {
				result := make([]FuturesOrder, len(orders))
				copy(result, orders)
				udc.mu.RUnlock()
				return result, nil
			}
		}
		udc.mu.RUnlock()
	}

	// Cache miss or expired - fetch from API
	result, err := c.client.GetOpenOrders(symbol)
	if err != nil {
		return nil, err
	}

	// Update cache
	if udc != nil {
		udc.mu.Lock()
		udc.openOrders[cacheKey] = result
		udc.openOrdersTime[cacheKey] = time.Now()
		udc.mu.Unlock()
	}

	return result, nil
}

func (c *CachedFuturesClient) GetOrder(symbol string, orderId int64) (*FuturesOrder, error) {
	return c.client.GetOrder(symbol, orderId)
}

// ==================== ALGO ORDERS (cached with invalidation) ====================

func (c *CachedFuturesClient) PlaceAlgoOrder(params AlgoOrderParams) (*AlgoOrderResponse, error) {
	result, err := c.client.PlaceAlgoOrder(params)
	if err == nil {
		c.InvalidateUserDataCache() // Algo order placed - invalidate cache
	}
	return result, err
}

func (c *CachedFuturesClient) GetOpenAlgoOrders(symbol string) ([]AlgoOrder, error) {
	c.mu.RLock()
	udc := c.userDataCache
	c.mu.RUnlock()

	cacheKey := symbol // Empty string for all symbols

	if udc != nil {
		udc.mu.RLock()
		if orders, ok := udc.openAlgoOrders[cacheKey]; ok {
			if cacheTime, timeOk := udc.openAlgoOrdersTime[cacheKey]; timeOk && time.Since(cacheTime) < udc.openAlgoOrdersTTL {
				result := make([]AlgoOrder, len(orders))
				copy(result, orders)
				udc.mu.RUnlock()
				return result, nil
			}
		}
		udc.mu.RUnlock()
	}

	// Cache miss or expired - fetch from API
	result, err := c.client.GetOpenAlgoOrders(symbol)
	if err != nil {
		return nil, err
	}

	// Update cache
	if udc != nil {
		udc.mu.Lock()
		udc.openAlgoOrders[cacheKey] = result
		udc.openAlgoOrdersTime[cacheKey] = time.Now()
		udc.mu.Unlock()
	}

	return result, nil
}

func (c *CachedFuturesClient) CancelAlgoOrder(symbol string, algoId int64) error {
	err := c.client.CancelAlgoOrder(symbol, algoId)
	if err == nil {
		c.InvalidateUserDataCache() // Algo order cancelled - invalidate cache
	}
	return err
}

func (c *CachedFuturesClient) CancelAllAlgoOrders(symbol string) error {
	err := c.client.CancelAllAlgoOrders(symbol)
	if err == nil {
		c.InvalidateUserDataCache() // Algo orders cancelled - invalidate cache
	}
	return err
}

func (c *CachedFuturesClient) GetAllAlgoOrders(symbol string, limit int) ([]AlgoOrder, error) {
	return c.client.GetAllAlgoOrders(symbol, limit)
}

// ==================== MARKET DATA (CACHED) ====================

// GetMarkPrice returns cached mark price if available, otherwise falls back to REST API
func (c *CachedFuturesClient) GetMarkPrice(symbol string) (*MarkPrice, error) {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	if cache != nil {
		if cached := cache.GetMarkPrice(symbol); cached != nil {
			return cached, nil
		}
	}

	// Cache miss - fall back to REST API
	result, err := c.client.GetMarkPrice(symbol)
	if err != nil {
		return nil, err
	}

	// Update cache with fresh data
	if cache != nil && result != nil {
		cache.UpdateMarkPrice(symbol, result.MarkPrice, result.IndexPrice, result.LastFundingRate, result.NextFundingTime)
	}

	return result, nil
}

// GetAllMarkPrices - not cached, returns all mark prices
func (c *CachedFuturesClient) GetAllMarkPrices() ([]MarkPrice, error) {
	return c.client.GetAllMarkPrices()
}

// GetFuturesCurrentPrice returns cached price if available, otherwise falls back to REST API
func (c *CachedFuturesClient) GetFuturesCurrentPrice(symbol string) (float64, error) {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	if cache != nil {
		if price, ok := cache.GetCurrentPrice(symbol); ok {
			return price, nil
		}
	}

	// Cache miss - fall back to REST API
	return c.client.GetFuturesCurrentPrice(symbol)
}

// GetFuturesKlines returns cached klines if available, otherwise falls back to REST API
// Uses in-flight request deduplication to avoid redundant API calls for the same symbol:interval
func (c *CachedFuturesClient) GetFuturesKlines(symbol, interval string, limit int) ([]Kline, error) {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	// Check cache first
	if cache != nil {
		if cached := cache.GetKlines(symbol, interval, limit); cached != nil && len(cached) >= limit {
			return cached, nil
		}
	}

	// Cache miss - check if there's already an in-flight request for this symbol:interval
	key := symbol + ":" + interval

	c.inFlightMu.Lock()
	if inFlight, exists := c.inFlightKlines[key]; exists {
		// Another goroutine is already fetching this data - wait for it
		c.deduplicatedReqs++
		c.inFlightMu.Unlock()

		// Wait for the in-flight request to complete
		<-inFlight.done

		// Return the result from the in-flight request
		if inFlight.err != nil {
			return nil, inFlight.err
		}

		// Re-check cache (the in-flight request should have populated it)
		if cache != nil {
			if cached := cache.GetKlines(symbol, interval, limit); cached != nil && len(cached) >= limit {
				return cached, nil
			}
		}

		// Fall back to the in-flight result
		if len(inFlight.result) >= limit {
			return inFlight.result[len(inFlight.result)-limit:], nil
		}
		return inFlight.result, nil
	}

	// No in-flight request - create one and become the fetcher
	inFlight := &inFlightRequest{
		done: make(chan struct{}),
	}
	c.inFlightKlines[key] = inFlight
	c.inFlightMu.Unlock()

	// Fetch from REST API
	result, err := c.client.GetFuturesKlines(symbol, interval, limit)

	// Store result in the in-flight request for waiters
	inFlight.result = result
	inFlight.err = err

	// Update cache with fresh data
	if err == nil && cache != nil && len(result) > 0 {
		cache.SetKlines(symbol, interval, result)
	}

	// Signal completion to all waiters
	close(inFlight.done)

	// Clean up the in-flight map
	c.inFlightMu.Lock()
	delete(c.inFlightKlines, key)
	c.inFlightMu.Unlock()

	return result, err
}

// GetOrderBookDepth returns cached order book if available, otherwise falls back to REST API
func (c *CachedFuturesClient) GetOrderBookDepth(symbol string, limit int) (*OrderBookDepth, error) {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	if cache != nil {
		if cached := cache.GetOrderBook(symbol); cached != nil {
			// Note: Cached order book might not have the exact limit requested
			// but for most use cases the top levels are sufficient
			return cached, nil
		}
	}

	// Cache miss - fall back to REST API
	return c.client.GetOrderBookDepth(symbol, limit)
}

// Get24hrTicker returns 24hr ticker for a symbol (no caching - relatively infrequent calls)
func (c *CachedFuturesClient) Get24hrTicker(symbol string) (*Futures24hrTicker, error) {
	return c.client.Get24hrTicker(symbol)
}

// GetAll24hrTickers returns all 24hr tickers (no caching)
func (c *CachedFuturesClient) GetAll24hrTickers() ([]Futures24hrTicker, error) {
	return c.client.GetAll24hrTickers()
}

// ==================== FUNDING RATES (cached from WebSocket mark price stream) ====================

func (c *CachedFuturesClient) GetFundingRate(symbol string) (*FundingRate, error) {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	// Try cache first - funding rate comes from mark price WebSocket stream
	if cache != nil {
		if cached := cache.GetFundingRate(symbol); cached != nil {
			return cached, nil
		}
	}

	// Cache miss - fall back to REST API
	return c.client.GetFundingRate(symbol)
}

func (c *CachedFuturesClient) GetFundingRateHistory(symbol string, limit int) ([]FundingRate, error) {
	return c.client.GetFundingRateHistory(symbol, limit)
}

// ==================== EXCHANGE INFO (no caching) ====================

func (c *CachedFuturesClient) GetFuturesExchangeInfo() (*FuturesExchangeInfo, error) {
	return c.client.GetFuturesExchangeInfo()
}

func (c *CachedFuturesClient) GetFuturesSymbols() ([]string, error) {
	return c.client.GetFuturesSymbols()
}

// ==================== HISTORY (no caching) ====================

func (c *CachedFuturesClient) GetTradeHistory(symbol string, limit int) ([]FuturesTrade, error) {
	return c.client.GetTradeHistory(symbol, limit)
}

func (c *CachedFuturesClient) GetFundingFeeHistory(symbol string, limit int) ([]FundingFeeRecord, error) {
	return c.client.GetFundingFeeHistory(symbol, limit)
}

func (c *CachedFuturesClient) GetAllOrders(symbol string, limit int) ([]FuturesOrder, error) {
	return c.client.GetAllOrders(symbol, limit)
}

func (c *CachedFuturesClient) GetIncomeHistory(incomeType string, startTime, endTime int64, limit int) ([]IncomeRecord, error) {
	return c.client.GetIncomeHistory(incomeType, startTime, endTime, limit)
}

// ==================== WEBSOCKET (no caching) ====================

func (c *CachedFuturesClient) GetListenKey() (string, error) {
	return c.client.GetListenKey()
}

func (c *CachedFuturesClient) KeepAliveListenKey(listenKey string) error {
	return c.client.KeepAliveListenKey(listenKey)
}

func (c *CachedFuturesClient) CloseListenKey(listenKey string) error {
	return c.client.CloseListenKey(listenKey)
}

// ==================== KLINE PREFETCH & STATS ====================

// PrefetchKlines fetches multiple timeframes for a symbol in parallel
// This is useful when we know all timeframes will be needed (e.g., at scan start)
// Returns the number of successful prefetches
func (c *CachedFuturesClient) PrefetchKlines(symbol string, intervals []string, limit int) int {
	c.inFlightMu.Lock()
	c.prefetchReqs++
	c.inFlightMu.Unlock()

	if len(intervals) == 0 {
		return 0
	}

	type prefetchResult struct {
		interval string
		success  bool
	}

	results := make(chan prefetchResult, len(intervals))
	var wg sync.WaitGroup

	for _, interval := range intervals {
		wg.Add(1)
		go func(intv string) {
			defer wg.Done()

			// Check if already cached
			c.mu.RLock()
			cache := c.cache
			c.mu.RUnlock()

			if cache != nil {
				if cached := cache.GetKlines(symbol, intv, limit); cached != nil && len(cached) >= limit {
					// Already cached - count as prefetch hit
					c.inFlightMu.Lock()
					c.prefetchHits++
					c.inFlightMu.Unlock()
					results <- prefetchResult{intv, true}
					return
				}
			}

			// Not cached - fetch it (deduplication handled by GetFuturesKlines)
			_, err := c.GetFuturesKlines(symbol, intv, limit)
			results <- prefetchResult{intv, err == nil}
		}(interval)
	}

	// Wait for all prefetches to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	successCount := 0
	for result := range results {
		if result.success {
			successCount++
		}
	}

	return successCount
}

// PrefetchKlinesForMode prefetches all timeframes needed for a trading mode
// Returns the number of successful prefetches
func (c *CachedFuturesClient) PrefetchKlinesForMode(symbol string, mode string, limit int) int {
	var intervals []string

	switch mode {
	case "scalp":
		intervals = []string{"1m", "5m", "15m", "1h"}
	case "swing":
		intervals = []string{"1m", "15m", "1h"}
	case "position":
		intervals = []string{"1m", "15m", "1h", "4h"}
	default:
		// Default: fetch common timeframes
		intervals = []string{"1m", "15m", "1h"}
	}

	return c.PrefetchKlines(symbol, intervals, limit)
}

// GetKlineCacheStats returns comprehensive cache statistics
func (c *CachedFuturesClient) GetKlineCacheStats() KlineCacheStats {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	var hits, misses int64
	if cache != nil {
		hits, misses, _ = cache.GetStats()
	}

	c.inFlightMu.Lock()
	deduplicated := c.deduplicatedReqs
	prefetchHits := c.prefetchHits
	prefetchReqs := c.prefetchReqs
	c.inFlightMu.Unlock()

	total := hits + misses
	var hitRate, dedupeRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}
	totalReqs := total + deduplicated
	if totalReqs > 0 {
		dedupeRate = float64(deduplicated) / float64(totalReqs) * 100
	}

	return KlineCacheStats{
		Hits:             hits,
		Misses:           misses,
		Deduplicated:     deduplicated,
		PrefetchHits:     prefetchHits,
		PrefetchRequests: prefetchReqs,
		HitRate:          hitRate,
		DedupeRate:       dedupeRate,
	}
}

// ResetKlineCacheStats resets the cache statistics
func (c *CachedFuturesClient) ResetKlineCacheStats() {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	if cache != nil {
		cache.ClearStats()
	}

	c.inFlightMu.Lock()
	c.deduplicatedReqs = 0
	c.prefetchHits = 0
	c.prefetchReqs = 0
	c.inFlightMu.Unlock()
}

// Compile-time check that CachedFuturesClient implements FuturesClient
var _ FuturesClient = (*CachedFuturesClient)(nil)
