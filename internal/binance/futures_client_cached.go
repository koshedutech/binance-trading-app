package binance

import (
	"sync"
)

// CachedFuturesClient wraps a FuturesClient with cache-first logic for market data
// This reduces REST API calls by using WebSocket-populated cache
type CachedFuturesClient struct {
	client FuturesClient
	cache  *MarketDataCache
	mu     sync.RWMutex
}

// NewCachedFuturesClient creates a new cache-aware futures client wrapper
func NewCachedFuturesClient(client FuturesClient, cache *MarketDataCache) *CachedFuturesClient {
	return &CachedFuturesClient{
		client: client,
		cache:  cache,
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

// ==================== ACCOUNT (no caching - always fresh) ====================

func (c *CachedFuturesClient) GetFuturesAccountInfo() (*FuturesAccountInfo, error) {
	return c.client.GetFuturesAccountInfo()
}

func (c *CachedFuturesClient) GetPositions() ([]FuturesPosition, error) {
	return c.client.GetPositions()
}

func (c *CachedFuturesClient) GetPositionBySymbol(symbol string) (*FuturesPosition, error) {
	return c.client.GetPositionBySymbol(symbol)
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

// ==================== TRADING (no caching) ====================

func (c *CachedFuturesClient) PlaceFuturesOrder(params FuturesOrderParams) (*FuturesOrderResponse, error) {
	return c.client.PlaceFuturesOrder(params)
}

func (c *CachedFuturesClient) CancelFuturesOrder(symbol string, orderId int64) error {
	return c.client.CancelFuturesOrder(symbol, orderId)
}

func (c *CachedFuturesClient) CancelAllFuturesOrders(symbol string) error {
	return c.client.CancelAllFuturesOrders(symbol)
}

func (c *CachedFuturesClient) GetOpenOrders(symbol string) ([]FuturesOrder, error) {
	return c.client.GetOpenOrders(symbol)
}

func (c *CachedFuturesClient) GetOrder(symbol string, orderId int64) (*FuturesOrder, error) {
	return c.client.GetOrder(symbol, orderId)
}

// ==================== ALGO ORDERS (no caching) ====================

func (c *CachedFuturesClient) PlaceAlgoOrder(params AlgoOrderParams) (*AlgoOrderResponse, error) {
	return c.client.PlaceAlgoOrder(params)
}

func (c *CachedFuturesClient) GetOpenAlgoOrders(symbol string) ([]AlgoOrder, error) {
	return c.client.GetOpenAlgoOrders(symbol)
}

func (c *CachedFuturesClient) CancelAlgoOrder(symbol string, algoId int64) error {
	return c.client.CancelAlgoOrder(symbol, algoId)
}

func (c *CachedFuturesClient) CancelAllAlgoOrders(symbol string) error {
	return c.client.CancelAllAlgoOrders(symbol)
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
func (c *CachedFuturesClient) GetFuturesKlines(symbol, interval string, limit int) ([]Kline, error) {
	c.mu.RLock()
	cache := c.cache
	c.mu.RUnlock()

	if cache != nil {
		if cached := cache.GetKlines(symbol, interval, limit); cached != nil && len(cached) >= limit {
			return cached, nil
		}
	}

	// Cache miss or insufficient data - fall back to REST API
	result, err := c.client.GetFuturesKlines(symbol, interval, limit)
	if err != nil {
		return nil, err
	}

	// Update cache with fresh data
	if cache != nil && len(result) > 0 {
		cache.SetKlines(symbol, interval, result)
	}

	return result, nil
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

// Compile-time check that CachedFuturesClient implements FuturesClient
var _ FuturesClient = (*CachedFuturesClient)(nil)
