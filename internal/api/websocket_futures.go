package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/events"

	"github.com/gorilla/websocket"
)

const (
	// Binance Futures WebSocket URLs
	FuturesWSBaseURL     = "wss://fstream.binance.com/ws"
	FuturesWSTestnetURL  = "wss://stream.binancefuture.com/ws"

	// Stream types
	StreamTypeDepth      = "depth"
	StreamTypeDepthFast  = "depth@100ms"
	StreamTypeTrade      = "trade"
	StreamTypeMarkPrice  = "markPrice"
	StreamTypeKline      = "kline"
)

// FuturesWSClient manages connections to Binance Futures WebSocket streams
type FuturesWSClient struct {
	baseURL         string
	conn            *websocket.Conn
	subscriptions   map[string]bool
	mu              sync.RWMutex
	hub             *WSHub
	done            chan struct{}
	reconnectCh     chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	marketDataCache *binance.MarketDataCache
}

// OrderBookUpdate represents an order book depth update
type OrderBookUpdate struct {
	Type         string          `json:"type"`
	EventType    string          `json:"e"`
	EventTime    int64           `json:"E"`
	TransactTime int64           `json:"T"`
	Symbol       string          `json:"s"`
	FirstID      int64           `json:"U"`
	FinalID      int64           `json:"u"`
	PrevFinalID  int64           `json:"pu"`
	Bids         [][]string      `json:"b"` // [price, qty]
	Asks         [][]string      `json:"a"` // [price, qty]
}

// MarkPriceUpdate represents a mark price update
type MarkPriceUpdate struct {
	Type            string  `json:"type"`
	EventType       string  `json:"e"`
	EventTime       int64   `json:"E"`
	Symbol          string  `json:"s"`
	MarkPrice       string  `json:"p"`
	IndexPrice      string  `json:"i"`
	FundingRate     string  `json:"r"`
	NextFundingTime int64   `json:"T"`
}

// AggTradeUpdate represents an aggregated trade update
type AggTradeUpdate struct {
	Type         string `json:"type"`
	EventType    string `json:"e"`
	EventTime    int64  `json:"E"`
	Symbol       string `json:"s"`
	TradeID      int64  `json:"a"`
	Price        string `json:"p"`
	Quantity     string `json:"q"`
	FirstTradeID int64  `json:"f"`
	LastTradeID  int64  `json:"l"`
	TradeTime    int64  `json:"T"`
	IsBuyerMaker bool   `json:"m"`
}

// NewFuturesWSClient creates a new Futures WebSocket client
func NewFuturesWSClient(testnet bool, hub *WSHub) *FuturesWSClient {
	baseURL := FuturesWSBaseURL
	if testnet {
		baseURL = FuturesWSTestnetURL
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &FuturesWSClient{
		baseURL:       baseURL,
		subscriptions: make(map[string]bool),
		hub:           hub,
		done:          make(chan struct{}),
		reconnectCh:   make(chan struct{}, 1),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetMarketDataCache sets the cache for storing WebSocket data
func (c *FuturesWSClient) SetMarketDataCache(cache *binance.MarketDataCache) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.marketDataCache = cache
}

// GetMarketDataCache returns the current market data cache
func (c *FuturesWSClient) GetMarketDataCache() *binance.MarketDataCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.marketDataCache
}

// Connect establishes connection to Binance Futures WebSocket
func (c *FuturesWSClient) Connect(streams []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build stream URL
	streamURL := c.baseURL
	if len(streams) > 0 {
		streamURL = fmt.Sprintf("%s/stream?streams=%s", c.baseURL[:len(c.baseURL)-3], url.QueryEscape(joinStreams(streams)))
	}

	log.Printf("Connecting to Futures WebSocket: %s", streamURL)

	conn, _, err := websocket.DefaultDialer.Dial(streamURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Futures WebSocket: %w", err)
	}

	c.conn = conn

	// Track subscriptions
	for _, stream := range streams {
		c.subscriptions[stream] = true
	}

	// Start reading messages
	go c.readMessages()

	// Start ping/pong handler
	go c.pingLoop()

	log.Printf("Futures WebSocket connected, subscribed to %d streams", len(streams))

	return nil
}

// SubscribeDepth subscribes to order book depth stream
func (c *FuturesWSClient) SubscribeDepth(symbol string, fast bool) error {
	stream := fmt.Sprintf("%s@depth", normalizeSymbol(symbol))
	if fast {
		stream = fmt.Sprintf("%s@depth@100ms", normalizeSymbol(symbol))
	}
	return c.subscribe(stream)
}

// SubscribeMarkPrice subscribes to mark price stream
func (c *FuturesWSClient) SubscribeMarkPrice(symbol string) error {
	stream := fmt.Sprintf("%s@markPrice", normalizeSymbol(symbol))
	return c.subscribe(stream)
}

// SubscribeAllMarkPrices subscribes to all mark prices
func (c *FuturesWSClient) SubscribeAllMarkPrices() error {
	return c.subscribe("!markPrice@arr")
}

// SubscribeAggTrade subscribes to aggregated trade stream
func (c *FuturesWSClient) SubscribeAggTrade(symbol string) error {
	stream := fmt.Sprintf("%s@aggTrade", normalizeSymbol(symbol))
	return c.subscribe(stream)
}

// SubscribeKline subscribes to kline/candlestick stream
func (c *FuturesWSClient) SubscribeKline(symbol, interval string) error {
	stream := fmt.Sprintf("%s@kline_%s", normalizeSymbol(symbol), interval)
	return c.subscribe(stream)
}

// UnsubscribeKline unsubscribes from a kline/candlestick stream
func (c *FuturesWSClient) UnsubscribeKline(symbol, interval string) error {
	stream := fmt.Sprintf("%s@kline_%s", normalizeSymbol(symbol), interval)
	return c.Unsubscribe(stream)
}

// Unsubscribe removes a stream subscription
func (c *FuturesWSClient) Unsubscribe(stream string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"method": "UNSUBSCRIBE",
		"params": []string{stream},
		"id":     time.Now().UnixNano(),
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	delete(c.subscriptions, stream)
	return nil
}

// Close closes the WebSocket connection
func (c *FuturesWSClient) Close() {
	c.cancel()
	close(c.done)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// subscribe sends a subscription request
func (c *FuturesWSClient) subscribe(stream string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	if c.subscriptions[stream] {
		return nil // Already subscribed
	}

	msg := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{stream},
		"id":     time.Now().UnixNano(),
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", stream, err)
	}

	c.subscriptions[stream] = true
	log.Printf("Subscribed to Futures stream: %s", stream)
	return nil
}

// readMessages reads and processes incoming WebSocket messages
func (c *FuturesWSClient) readMessages() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Futures WebSocket reader panic: %v", r)
		}
		c.tryReconnect()
	}()

	for {
		select {
		case <-c.done:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Futures WebSocket read error: %v", err)
			}
			return
		}

		c.processMessage(message)
	}
}

// processMessage processes incoming WebSocket messages
func (c *FuturesWSClient) processMessage(message []byte) {
	// Parse the stream wrapper
	var streamWrapper struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &streamWrapper); err != nil {
		// Not a wrapped message, try direct parse
		c.processDirectMessage(message)
		return
	}

	// Process based on stream type
	stream := streamWrapper.Stream
	data := streamWrapper.Data

	switch {
	case containsString(stream, "@depth"):
		c.processDepthUpdate(data)
	case stream == "!markPrice@arr" || containsString(stream, "@markPrice"):
		c.processMarkPriceUpdate(data)
	case containsString(stream, "@aggTrade"):
		c.processAggTradeUpdate(data)
	case containsString(stream, "@kline"):
		c.processKlineUpdate(data)
	default:
		// Only log truly unknown streams
		if stream != "" {
			log.Printf("Unknown stream type: %s", stream)
		}
	}
}

// processDirectMessage processes non-wrapped messages
func (c *FuturesWSClient) processDirectMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	// Handle subscription responses
	if _, ok := msg["result"]; ok {
		return // Subscription confirmation
	}

	// Handle events with event type
	if eventType, ok := msg["e"].(string); ok {
		switch eventType {
		case "depthUpdate":
			c.processDepthUpdate(message)
		case "markPriceUpdate":
			c.processMarkPriceUpdate(message)
		case "aggTrade":
			c.processAggTradeUpdate(message)
		}
	}
}

// processDepthUpdate processes order book depth updates
func (c *FuturesWSClient) processDepthUpdate(data []byte) {
	var update OrderBookUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		log.Printf("Failed to parse depth update: %v", err)
		return
	}

	// Update the cache with fresh order book data
	c.mu.RLock()
	cache := c.marketDataCache
	c.mu.RUnlock()

	if cache != nil {
		cache.UpdateOrderBook(update.Symbol, update.Bids, update.Asks)
	}

	// Broadcast to connected clients
	event := events.Event{
		Type:      events.EventType("FUTURES_ORDERBOOK_UPDATE"),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":    update.Symbol,
			"bids":      update.Bids,
			"asks":      update.Asks,
			"eventTime": update.EventTime,
		},
	}

	if c.hub != nil {
		c.hub.BroadcastEvent(event)
	}
}

// processMarkPriceUpdate processes mark price updates
func (c *FuturesWSClient) processMarkPriceUpdate(data []byte) {
	var update MarkPriceUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		// Try array format for !markPrice@arr
		var updates []MarkPriceUpdate
		if err := json.Unmarshal(data, &updates); err != nil {
			log.Printf("Failed to parse mark price update: %v", err)
			return
		}
		// Process each update
		for _, u := range updates {
			c.broadcastMarkPrice(u)
		}
		return
	}

	c.broadcastMarkPrice(update)
}

// broadcastMarkPrice broadcasts a single mark price update
func (c *FuturesWSClient) broadcastMarkPrice(update MarkPriceUpdate) {
	// Update the cache with fresh mark price data
	c.mu.RLock()
	cache := c.marketDataCache
	c.mu.RUnlock()

	if cache != nil {
		cache.UpdateMarkPriceFromStrings(
			update.Symbol,
			update.MarkPrice,
			update.IndexPrice,
			update.FundingRate,
			update.NextFundingTime,
		)
	}

	// Broadcast to frontend clients
	event := events.Event{
		Type:      events.EventType("FUTURES_MARK_PRICE_UPDATE"),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":          update.Symbol,
			"markPrice":       update.MarkPrice,
			"indexPrice":      update.IndexPrice,
			"fundingRate":     update.FundingRate,
			"nextFundingTime": update.NextFundingTime,
		},
	}

	if c.hub != nil {
		c.hub.BroadcastEvent(event)
	}
}

// processAggTradeUpdate processes aggregated trade updates
func (c *FuturesWSClient) processAggTradeUpdate(data []byte) {
	var update AggTradeUpdate
	if err := json.Unmarshal(data, &update); err != nil {
		log.Printf("Failed to parse agg trade update: %v", err)
		return
	}

	event := events.Event{
		Type:      events.EventType("FUTURES_TRADE_UPDATE"),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":       update.Symbol,
			"price":        update.Price,
			"quantity":     update.Quantity,
			"tradeTime":    update.TradeTime,
			"isBuyerMaker": update.IsBuyerMaker,
		},
	}

	if c.hub != nil {
		c.hub.BroadcastEvent(event)
	}
}

// processKlineUpdate processes kline/candlestick updates
func (c *FuturesWSClient) processKlineUpdate(data []byte) {
	// Use interface{} for numeric fields as Binance can send string or number
	var update struct {
		EventType string `json:"e"`
		EventTime int64  `json:"E"`
		Symbol    string `json:"s"`
		Kline     struct {
			StartTime    int64       `json:"t"`
			CloseTime    int64       `json:"T"`
			Symbol       string      `json:"s"`
			Interval     string      `json:"i"`
			Open         interface{} `json:"o"`
			Close        interface{} `json:"c"`
			High         interface{} `json:"h"`
			Low          interface{} `json:"l"`
			Volume       interface{} `json:"v"`
			TradeCount   int         `json:"n"`
			IsFinal      bool        `json:"x"`
			QuoteVolume  interface{} `json:"q"`
		} `json:"k"`
	}

	if err := json.Unmarshal(data, &update); err != nil {
		log.Printf("Failed to parse kline update: %v", err)
		return
	}

	// Helper to convert interface{} to float64
	toFloat64 := func(v interface{}) float64 {
		switch val := v.(type) {
		case string:
			f, _ := strconv.ParseFloat(val, 64)
			return f
		case float64:
			return val
		case int64:
			return float64(val)
		case int:
			return float64(val)
		default:
			return 0
		}
	}

	// Parse values
	open := toFloat64(update.Kline.Open)
	high := toFloat64(update.Kline.High)
	low := toFloat64(update.Kline.Low)
	closePrice := toFloat64(update.Kline.Close)
	volume := toFloat64(update.Kline.Volume)
	quoteVol := toFloat64(update.Kline.QuoteVolume)

	// Update the cache with fresh kline data
	c.mu.RLock()
	cache := c.marketDataCache
	c.mu.RUnlock()

	if cache != nil {
		kline := binance.Kline{
			OpenTime:         update.Kline.StartTime,
			Open:             open,
			High:             high,
			Low:              low,
			Close:            closePrice,
			Volume:           volume,
			CloseTime:        update.Kline.CloseTime,
			QuoteAssetVolume: quoteVol,
			NumberOfTrades:   update.Kline.TradeCount,
		}

		cache.UpdateKline(update.Symbol, update.Kline.Interval, kline)
	}

	// Broadcast to frontend clients
	event := events.Event{
		Type:      events.EventType("FUTURES_KLINE_UPDATE"),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":    update.Symbol,
			"interval":  update.Kline.Interval,
			"open":      open,
			"close":     closePrice,
			"high":      high,
			"low":       low,
			"volume":    volume,
			"isFinal":   update.Kline.IsFinal,
			"startTime": update.Kline.StartTime,
			"closeTime": update.Kline.CloseTime,
		},
	}

	if c.hub != nil {
		c.hub.BroadcastEvent(event)
	}
}

// pingLoop sends periodic pings to keep connection alive
func (c *FuturesWSClient) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn != nil {
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					log.Printf("Futures WebSocket ping error: %v", err)
				}
			}
			c.mu.Unlock()
		}
	}
}

// tryReconnect attempts to reconnect after disconnection
func (c *FuturesWSClient) tryReconnect() {
	select {
	case <-c.done:
		return
	case c.reconnectCh <- struct{}{}:
	default:
		return // Already reconnecting
	}

	go func() {
		defer func() { <-c.reconnectCh }()

		backoff := time.Second
		maxBackoff := 30 * time.Second

		for {
			select {
			case <-c.done:
				return
			case <-c.ctx.Done():
				return
			default:
			}

			log.Printf("Attempting to reconnect to Futures WebSocket in %v...", backoff)
			time.Sleep(backoff)

			// Get current subscriptions
			c.mu.RLock()
			streams := make([]string, 0, len(c.subscriptions))
			for stream := range c.subscriptions {
				streams = append(streams, stream)
			}
			c.mu.RUnlock()

			// Clear subscriptions for fresh connection
			c.mu.Lock()
			c.subscriptions = make(map[string]bool)
			c.mu.Unlock()

			if err := c.Connect(streams); err != nil {
				log.Printf("Reconnection failed: %v", err)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			log.Println("Futures WebSocket reconnected successfully")
			return
		}
	}()
}

// GetSubscriptions returns current subscriptions
func (c *FuturesWSClient) GetSubscriptions() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	streams := make([]string, 0, len(c.subscriptions))
	for stream := range c.subscriptions {
		streams = append(streams, stream)
	}
	return streams
}

// Helper functions

func normalizeSymbol(symbol string) string {
	// Binance WebSocket requires lowercase symbols
	result := ""
	for _, c := range symbol {
		if c >= 'A' && c <= 'Z' {
			result += string(c + 32) // Convert to lowercase
		} else {
			result += string(c)
		}
	}
	return result
}

func joinStreams(streams []string) string {
	result := ""
	for i, s := range streams {
		if i > 0 {
			result += "/"
		}
		result += s
	}
	return result
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Global Futures WebSocket client
var futuresWSClient *FuturesWSClient

// InitFuturesWebSocket initializes the Futures WebSocket client
func InitFuturesWebSocket(testnet bool, hub *WSHub) *FuturesWSClient {
	futuresWSClient = NewFuturesWSClient(testnet, hub)
	return futuresWSClient
}

// GetFuturesWSClient returns the global Futures WebSocket client
func GetFuturesWSClient() *FuturesWSClient {
	return futuresWSClient
}

// BroadcastFuturesOrderBook broadcasts order book update to all clients
func BroadcastFuturesOrderBook(symbol string, bids, asks [][]string) {
	if wsHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventType("FUTURES_ORDERBOOK_UPDATE"),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol": symbol,
			"bids":   bids,
			"asks":   asks,
		},
	}

	wsHub.BroadcastEvent(event)
}

// BroadcastFuturesPosition broadcasts futures position update
func BroadcastFuturesPosition(positions interface{}) {
	if wsHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventType("FUTURES_POSITION_UPDATE"),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"positions": positions,
		},
	}

	wsHub.BroadcastEvent(event)
}
