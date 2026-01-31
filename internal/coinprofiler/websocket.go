// Package coinprofiler provides real-time coin data collection via WebSocket
// for the Chain Trading System.
package coinprofiler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// LogPrefixWS is the standard prefix for WebSocket-related log messages.
const LogPrefixWS = "[COIN-PROFILER-WS]"

// CoinUpdateCallback is called when coin data is updated in real-time.
// The map contains: symbol, price, timeframe, and full timeframe data.
type CoinUpdateCallback func(coinData map[string]interface{})

// WebSocketManager manages WebSocket connections to Binance Futures for the Coin Profiler.
// It handles connection lifecycle, subscriptions, message parsing, and reconnection logic.
type WebSocketManager struct {
	// Configuration
	config *CoinProfilerConfig

	// Parent profiler for data updates
	profiler *CoinProfiler

	// Callback for real-time coin updates (to broadcast via WebSocket)
	onCoinUpdate CoinUpdateCallback

	// Connection state
	conn            *websocket.Conn
	connectionState ConnectionState
	reconnectCount  int32 // atomic
	lastError       string

	// Subscription management
	activeSubscriptions map[string]bool // stream name -> subscribed
	pendingSubscribes   []string        // streams to subscribe after connect
	pendingUnsubscribes []string        // streams to unsubscribe

	// Message processing
	messageCount int64 // atomic - total messages received
	errorCount   int64 // atomic - total errors

	// Control
	ctx        context.Context
	cancelFunc context.CancelFunc
	stopChan   chan struct{}
	wg         sync.WaitGroup

	// Synchronization
	mu     sync.RWMutex
	connMu sync.Mutex // separate mutex for connection operations
}

// NewWebSocketManager creates a new WebSocket manager for the Coin Profiler.
func NewWebSocketManager(config *CoinProfilerConfig, profiler *CoinProfiler) *WebSocketManager {
	if config == nil {
		config = DefaultCoinProfilerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WebSocketManager{
		config:              config,
		profiler:            profiler,
		connectionState:     ConnectionStateDisconnected,
		activeSubscriptions: make(map[string]bool),
		pendingSubscribes:   make([]string, 0),
		pendingUnsubscribes: make([]string, 0),
		ctx:                 ctx,
		cancelFunc:          cancel,
		stopChan:            make(chan struct{}),
	}
}

// SetCoinUpdateCallback sets the callback function for real-time coin updates.
// This is used to broadcast updates to WebSocket clients.
func (wsm *WebSocketManager) SetCoinUpdateCallback(callback CoinUpdateCallback) {
	wsm.onCoinUpdate = callback
}

// Start starts the WebSocket manager and establishes the connection.
func (wsm *WebSocketManager) Start() error {
	wsm.mu.Lock()
	if wsm.connectionState != ConnectionStateDisconnected {
		wsm.mu.Unlock()
		return fmt.Errorf("%s already started or connecting", LogPrefixWS)
	}
	wsm.connectionState = ConnectionStateConnecting
	wsm.ctx, wsm.cancelFunc = context.WithCancel(context.Background())
	wsm.stopChan = make(chan struct{})
	wsm.mu.Unlock()

	wsm.log("Starting WebSocket manager")

	// Start connection loop in goroutine
	wsm.wg.Add(1)
	go wsm.connectionLoop()

	// Start ping loop
	wsm.wg.Add(1)
	go wsm.pingLoop()

	return nil
}

// Stop stops the WebSocket manager and closes the connection.
func (wsm *WebSocketManager) Stop() error {
	wsm.mu.Lock()
	if wsm.connectionState == ConnectionStateDisconnected {
		wsm.mu.Unlock()
		return nil // Already stopped
	}

	stopChan := wsm.stopChan
	wsm.mu.Unlock()

	wsm.log("Stopping WebSocket manager")

	// Signal shutdown
	wsm.cancelFunc()

	select {
	case <-stopChan:
		// Already closed
	default:
		close(stopChan)
	}

	// Close connection
	wsm.connMu.Lock()
	if wsm.conn != nil {
		wsm.conn.Close()
		wsm.conn = nil
	}
	wsm.connMu.Unlock()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		wsm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wsm.log("WebSocket manager stopped gracefully")
	case <-time.After(10 * time.Second):
		wsm.log("WebSocket manager stop timed out")
	}

	wsm.mu.Lock()
	wsm.connectionState = ConnectionStateDisconnected
	// Clear subscription state for clean restart
	wsm.activeSubscriptions = make(map[string]bool)
	wsm.pendingSubscribes = wsm.pendingSubscribes[:0]
	wsm.pendingUnsubscribes = wsm.pendingUnsubscribes[:0]
	wsm.mu.Unlock()

	return nil
}

// Subscribe adds streams to the subscription list.
// If connected, sends subscribe request immediately. Otherwise queues for later.
// Streams should be in format: "btcusdt@kline_5m", "ethusdt@kline_15m"
func (wsm *WebSocketManager) Subscribe(streams []string) error {
	if len(streams) == 0 {
		return nil
	}

	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// Build a set of already pending streams for duplicate detection
	pendingSet := make(map[string]bool)
	for _, s := range wsm.pendingSubscribes {
		pendingSet[s] = true
	}

	// Check for duplicates (in both active and pending)
	newStreams := make([]string, 0, len(streams))
	for _, stream := range streams {
		normalizedStream := strings.ToLower(stream)
		if !wsm.activeSubscriptions[normalizedStream] && !pendingSet[normalizedStream] {
			newStreams = append(newStreams, normalizedStream)
			pendingSet[normalizedStream] = true // Prevent duplicates within same call
		}
	}

	if len(newStreams) == 0 {
		return nil // All already subscribed or pending
	}

	// Check subscription limit (active + pending + new)
	totalSubs := len(wsm.activeSubscriptions) + len(wsm.pendingSubscribes) + len(newStreams)
	if totalSubs > wsm.config.MaxSubscriptions {
		return fmt.Errorf("%s subscription limit exceeded: %d > %d",
			LogPrefixWS, totalSubs, wsm.config.MaxSubscriptions)
	}

	if wsm.connectionState == ConnectionStateConnected && wsm.conn != nil {
		// Subscribe immediately
		return wsm.sendSubscribeUnlocked(newStreams)
	}

	// Queue for later subscription
	wsm.pendingSubscribes = append(wsm.pendingSubscribes, newStreams...)
	wsm.logDebug("Queued %d streams for subscription", len(newStreams))
	return nil
}

// Unsubscribe removes streams from the subscription list.
func (wsm *WebSocketManager) Unsubscribe(streams []string) error {
	if len(streams) == 0 {
		return nil
	}

	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// Find which streams are actually subscribed
	toUnsubscribe := make([]string, 0, len(streams))
	for _, stream := range streams {
		normalizedStream := strings.ToLower(stream)
		if wsm.activeSubscriptions[normalizedStream] {
			toUnsubscribe = append(toUnsubscribe, normalizedStream)
		}
	}

	if len(toUnsubscribe) == 0 {
		return nil // None subscribed
	}

	if wsm.connectionState == ConnectionStateConnected && wsm.conn != nil {
		// Unsubscribe immediately
		return wsm.sendUnsubscribeUnlocked(toUnsubscribe)
	}

	// Queue for later unsubscription
	wsm.pendingUnsubscribes = append(wsm.pendingUnsubscribes, toUnsubscribe...)
	wsm.logDebug("Queued %d streams for unsubscription", len(toUnsubscribe))
	return nil
}

// UpdateSubscriptions updates subscriptions based on combined requirements.
// It calculates the diff and subscribes/unsubscribes as needed.
func (wsm *WebSocketManager) UpdateSubscriptions(requirements *CombinedRequirements) error {
	if requirements == nil {
		return nil
	}

	// Calculate required streams from requirements
	requiredStreams := make(map[string]bool)
	for symbol, symReq := range requirements.BySymbol {
		for _, tf := range symReq.Timeframes {
			stream := buildKlineStream(symbol, tf)
			requiredStreams[stream] = true
		}
	}

	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// Calculate diff: what to subscribe and what to unsubscribe
	toSubscribe := make([]string, 0)
	for stream := range requiredStreams {
		if !wsm.activeSubscriptions[stream] {
			toSubscribe = append(toSubscribe, stream)
		}
	}

	toUnsubscribe := make([]string, 0)
	for stream := range wsm.activeSubscriptions {
		if !requiredStreams[stream] {
			toUnsubscribe = append(toUnsubscribe, stream)
		}
	}

	// Check subscription limit
	newTotal := len(wsm.activeSubscriptions) + len(toSubscribe) - len(toUnsubscribe)
	if newTotal > wsm.config.MaxSubscriptions {
		return fmt.Errorf("%s subscription limit would be exceeded: %d > %d",
			LogPrefixWS, newTotal, wsm.config.MaxSubscriptions)
	}

	wsm.logDebug("Subscription update: +%d -%d (total: %d)",
		len(toSubscribe), len(toUnsubscribe), newTotal)

	// Queue operations
	if len(toSubscribe) > 0 {
		wsm.pendingSubscribes = append(wsm.pendingSubscribes, toSubscribe...)
	}
	if len(toUnsubscribe) > 0 {
		wsm.pendingUnsubscribes = append(wsm.pendingUnsubscribes, toUnsubscribe...)
	}

	// Process immediately if connected
	if wsm.connectionState == ConnectionStateConnected && wsm.conn != nil {
		return wsm.processPendingUnlocked()
	}

	return nil
}

// GetStatus returns the current WebSocket connection status.
func (wsm *WebSocketManager) GetStatus() WebSocketStatus {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	return WebSocketStatus{
		Connected:         wsm.connectionState == ConnectionStateConnected,
		ConnectionState:   wsm.connectionState.String(),
		SubscriptionCount: len(wsm.activeSubscriptions),
		ReconnectCount:    int(atomic.LoadInt32(&wsm.reconnectCount)),
		MessageCount:      atomic.LoadInt64(&wsm.messageCount),
		ErrorCount:        atomic.LoadInt64(&wsm.errorCount),
		LastError:         wsm.lastError,
	}
}

// WebSocketStatus represents the current status of the WebSocket connection.
type WebSocketStatus struct {
	Connected         bool   `json:"connected"`
	ConnectionState   string `json:"connection_state"`
	SubscriptionCount int    `json:"subscription_count"`
	ReconnectCount    int    `json:"reconnect_count"`
	MessageCount      int64  `json:"message_count"`
	ErrorCount        int64  `json:"error_count"`
	LastError         string `json:"last_error,omitempty"`
}

// ============================================================================
// INTERNAL METHODS
// ============================================================================

// connectionLoop manages the WebSocket connection lifecycle.
func (wsm *WebSocketManager) connectionLoop() {
	defer wsm.wg.Done()

	backoff := wsm.config.ReconnectDelay

	for {
		select {
		case <-wsm.ctx.Done():
			wsm.log("Connection loop stopped (context cancelled)")
			return
		case <-wsm.stopChan:
			wsm.log("Connection loop stopped (stop signal)")
			return
		default:
		}

		// Attempt connection
		err := wsm.connect()
		if err != nil {
			wsm.setLastError(err.Error())
			wsm.logError("Connection failed: %v", err)

			// Wait before reconnecting
			select {
			case <-wsm.ctx.Done():
				return
			case <-wsm.stopChan:
				return
			case <-time.After(backoff):
			}

			// Exponential backoff
			backoff = time.Duration(float64(backoff) * wsm.config.ReconnectBackoff)
			if backoff > wsm.config.MaxReconnectDelay {
				backoff = wsm.config.MaxReconnectDelay
			}

			atomic.AddInt32(&wsm.reconnectCount, 1)
			continue
		}

		// Connected successfully - reset backoff
		backoff = wsm.config.ReconnectDelay

		// Process pending subscriptions
		wsm.mu.Lock()
		if err := wsm.processPendingUnlocked(); err != nil {
			wsm.logError("Failed to process pending subscriptions: %v", err)
		}
		wsm.mu.Unlock()

		// Start reading messages (blocks until disconnected)
		wsm.readLoop()

		// Check if we should reconnect
		select {
		case <-wsm.ctx.Done():
			return
		case <-wsm.stopChan:
			return
		default:
			// Disconnected unexpectedly, will reconnect
			wsm.mu.Lock()
			wsm.connectionState = ConnectionStateReconnecting
			wsm.mu.Unlock()
			wsm.log("Connection lost, reconnecting...")
		}
	}
}

// connect establishes the WebSocket connection to Binance.
func (wsm *WebSocketManager) connect() error {
	wsm.mu.Lock()
	wsm.connectionState = ConnectionStateConnecting
	endpoint := wsm.config.WebSocketEndpoint
	wsm.mu.Unlock()

	wsm.log("Connecting to %s", endpoint)

	// Parse and construct WebSocket URL
	wsURL := endpoint
	if !strings.HasPrefix(wsURL, "ws://") && !strings.HasPrefix(wsURL, "wss://") {
		wsURL = "wss://" + wsURL
	}

	// Connect with timeout
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(wsm.ctx, wsURL, nil)
	if err != nil {
		wsm.mu.Lock()
		wsm.connectionState = ConnectionStateDisconnected
		wsm.mu.Unlock()
		return fmt.Errorf("dial failed: %w", err)
	}

	// Configure connection
	conn.SetReadLimit(1024 * 1024) // 1MB max message size
	conn.SetPongHandler(func(appData string) error {
		wsm.logDebug("Received pong")
		return nil
	})

	wsm.connMu.Lock()
	wsm.conn = conn
	wsm.connMu.Unlock()

	wsm.mu.Lock()
	wsm.connectionState = ConnectionStateConnected
	wsm.mu.Unlock()

	wsm.log("Connected successfully")
	return nil
}

// readLoop reads and processes incoming WebSocket messages.
func (wsm *WebSocketManager) readLoop() {
	for {
		select {
		case <-wsm.ctx.Done():
			return
		case <-wsm.stopChan:
			return
		default:
		}

		wsm.connMu.Lock()
		conn := wsm.conn
		wsm.connMu.Unlock()

		if conn == nil {
			return
		}

		// Set read deadline for timeout detection
		conn.SetReadDeadline(time.Now().Add(wsm.config.PongTimeout + wsm.config.PingInterval))

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				wsm.log("Connection closed normally")
			} else if websocket.IsUnexpectedCloseError(err, websocket.CloseAbnormalClosure) {
				wsm.logError("Connection closed unexpectedly: %v", err)
			} else {
				wsm.logError("Read error: %v", err)
			}
			return
		}

		if messageType == websocket.TextMessage {
			atomic.AddInt64(&wsm.messageCount, 1)
			wsm.processMessage(message)
		}
	}
}

// processMessage handles an incoming WebSocket message.
func (wsm *WebSocketManager) processMessage(message []byte) {
	// Log first few messages for debugging
	if atomic.LoadInt64(&wsm.messageCount) <= 5 {
		wsm.log("Received message #%d: %s", atomic.LoadInt64(&wsm.messageCount), string(message[:min(len(message), 200)]))
	}

	// Check for combined stream format (from combined streams endpoint)
	var streamWrapper struct {
		Stream string          `json:"stream"`
		Data   json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(message, &streamWrapper); err == nil && streamWrapper.Stream != "" {
		// Combined stream format
		wsm.processStreamMessage(streamWrapper.Stream, streamWrapper.Data)
		return
	}

	// Direct message format (subscription response or error)
	var response struct {
		Result interface{} `json:"result"`
		ID     int64       `json:"id"`
		Error  *struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		} `json:"error"`
	}

	if err := json.Unmarshal(message, &response); err == nil {
		if response.Error != nil {
			wsm.logError("Subscription error: code=%d msg=%s", response.Error.Code, response.Error.Msg)
			atomic.AddInt64(&wsm.errorCount, 1)
			return
		}
		if response.ID != 0 {
			wsm.logDebug("Subscription response received: id=%d", response.ID)
			return
		}
		// Not a subscription response, continue to check other formats
	}

	// Try parsing as direct event (e.g., kline event without wrapper)
	// Note: Binance messages have both "e" (event type, string) and "E" (event time, number).
	// Go's JSON is case-insensitive, so it may try to match "E" to the "e" tag and fail.
	// We check if EventType was successfully set regardless of other parsing errors.
	var baseEvent struct {
		EventType string `json:"e"`
	}
	_ = json.Unmarshal(message, &baseEvent) // Ignore error - we just need the event type
	if baseEvent.EventType != "" {
		wsm.processDirectEvent(baseEvent.EventType, message)
		return
	}
	wsm.logDebug("Unknown message format: %s", string(message[:min(len(message), 100)]))
}

// processStreamMessage handles messages from combined stream format.
func (wsm *WebSocketManager) processStreamMessage(stream string, data []byte) {
	// Parse stream name to determine type
	// Format: <symbol>@kline_<interval>
	if strings.Contains(stream, "@kline_") {
		wsm.processKlineMessage(data)
		return
	}

	// Handle other stream types as needed
	wsm.logDebug("Unhandled stream type: %s", stream)
}

// processDirectEvent handles direct event messages (not wrapped in stream).
func (wsm *WebSocketManager) processDirectEvent(eventType string, message []byte) {
	switch eventType {
	case "kline":
		wsm.processKlineMessage(message)
	default:
		wsm.logDebug("Unhandled event type: %s", eventType)
	}
}

// processKlineMessage parses and processes a kline/candlestick message.
func (wsm *WebSocketManager) processKlineMessage(data []byte) {
	var klineMsg KlineMessage
	if err := json.Unmarshal(data, &klineMsg); err != nil {
		wsm.logError("Failed to parse kline message: %v", err)
		atomic.AddInt64(&wsm.errorCount, 1)
		return
	}

	// Validate the message
	if wsm.config.EnableDataValidation {
		if klineMsg.Symbol == "" || klineMsg.Kline.Interval == "" {
			wsm.logError("Invalid kline message: missing symbol or interval")
			atomic.AddInt64(&wsm.errorCount, 1)
			return
		}
	}

	// Parse string values to float64
	open, _ := strconv.ParseFloat(klineMsg.Kline.Open, 64)
	high, _ := strconv.ParseFloat(klineMsg.Kline.High, 64)
	low, _ := strconv.ParseFloat(klineMsg.Kline.Low, 64)
	closePrice, _ := strconv.ParseFloat(klineMsg.Kline.Close, 64)
	volume, _ := strconv.ParseFloat(klineMsg.Kline.Volume, 64)
	quoteVolume, _ := strconv.ParseFloat(klineMsg.Kline.QuoteVolume, 64)
	takerBuyVol, _ := strconv.ParseFloat(klineMsg.Kline.TakerBuyVol, 64)

	// Calculate taker sell volume
	takerSellVol := volume - takerBuyVol

	// Create timeframe data
	tfData := &TimeframeData{
		Timeframe:    klineMsg.Kline.Interval,
		Open:         open,
		High:         high,
		Low:          low,
		Close:        closePrice,
		Volume:       volume,
		TakerBuyVol:  takerBuyVol,
		TakerSellVol: takerSellVol,
		QuoteVolume:  quoteVolume,
		TradeCount:   klineMsg.Kline.TradeCount,
		IsClosedBar:  klineMsg.Kline.IsClosed,
		OpenTime:     time.UnixMilli(klineMsg.Kline.StartTime),
		CloseTime:    time.UnixMilli(klineMsg.Kline.CloseTime),
		UpdatedAt:    time.Now(),
	}

	// Update profiler data
	if wsm.profiler != nil {
		wsm.updateCoinData(klineMsg.Symbol, tfData)

		// Send to profiler's update channel
		select {
		case wsm.profiler.updateChan <- tfData:
		default:
			// Channel full, data will be dropped
			wsm.logDebug("Update channel full, dropping data for %s", klineMsg.Symbol)
		}
	}
}

// updateCoinData updates the CoinData in the profiler with new timeframe data.
func (wsm *WebSocketManager) updateCoinData(symbol string, tfData *TimeframeData) {
	if wsm.profiler == nil {
		return
	}

	wsm.profiler.mu.Lock()

	// Get or create CoinData
	coinData, exists := wsm.profiler.coinData[symbol]
	if !exists {
		coinData = &CoinData{
			Symbol:     symbol,
			Timeframes: make(map[string]*TimeframeData),
			UpdatedAt:  time.Now(),
		}
		wsm.profiler.coinData[symbol] = coinData
	}

	// Update timeframe data (deep copy to prevent race conditions)
	tfCopy := *tfData
	coinData.Timeframes[tfData.Timeframe] = &tfCopy

	// Update price and timestamp
	coinData.Price = tfData.Close
	coinData.UpdatedAt = time.Now()

	// Update profiler metrics
	wsm.profiler.updateCount++
	wsm.profiler.lastUpdateTime = time.Now()

	wsm.profiler.mu.Unlock()

	// Store closed candles in history for pattern detection (outside main lock)
	if tfData.IsClosedBar {
		historicalCandle := HistoricalCandle{
			OpenTime:     tfData.OpenTime,
			CloseTime:    tfData.CloseTime,
			Open:         tfData.Open,
			High:         tfData.High,
			Low:          tfData.Low,
			Close:        tfData.Close,
			Volume:       tfData.Volume,
			TakerBuyVol:  tfData.TakerBuyVol,
			TakerSellVol: tfData.TakerSellVol,
			QuoteVolume:  tfData.QuoteVolume,
			TradeCount:   tfData.TradeCount,
		}
		wsm.profiler.AddClosedCandle(symbol, tfData.Timeframe, historicalCandle)
	}

	// Broadcast real-time update to WebSocket clients (outside lock)
	if wsm.onCoinUpdate != nil {
		wsm.onCoinUpdate(map[string]interface{}{
			"symbol":    symbol,
			"price":     tfData.Close,
			"timeframe": tfData.Timeframe,
			"data": map[string]interface{}{
				"open":          tfData.Open,
				"high":          tfData.High,
				"low":           tfData.Low,
				"close":         tfData.Close,
				"volume":        tfData.Volume,
				"taker_buy_vol": tfData.TakerBuyVol,
				"taker_sell_vol": tfData.TakerSellVol,
				"quote_volume":  tfData.QuoteVolume,
				"trade_count":   tfData.TradeCount,
				"is_closed_bar": tfData.IsClosedBar,
				"open_time":     tfData.OpenTime,
				"close_time":    tfData.CloseTime,
				"updated_at":    tfData.UpdatedAt,
			},
		})
	}

	// Call price update callback for tick-level breakout detection (outside lock)
	// This enables immediate entry signal detection without waiting for candle close
	if wsm.profiler != nil && !tfData.IsClosedBar {
		if priceCallback := wsm.profiler.GetPriceUpdateCallback(); priceCallback != nil {
			priceCallback(symbol, tfData.Timeframe, tfData.Close, tfData.High, tfData.Low)
		}
	}
}

// pingLoop sends periodic pings to keep the connection alive.
func (wsm *WebSocketManager) pingLoop() {
	defer wsm.wg.Done()

	ticker := time.NewTicker(wsm.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wsm.ctx.Done():
			return
		case <-wsm.stopChan:
			return
		case <-ticker.C:
			wsm.connMu.Lock()
			conn := wsm.conn
			wsm.connMu.Unlock()

			if conn == nil {
				continue
			}

			wsm.mu.RLock()
			state := wsm.connectionState
			wsm.mu.RUnlock()

			if state != ConnectionStateConnected {
				continue
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				wsm.logError("Ping failed: %v", err)
			} else {
				wsm.logDebug("Sent ping")
			}
		}
	}
}

// sendSubscribeUnlocked sends a subscribe request. Must be called with mu held.
func (wsm *WebSocketManager) sendSubscribeUnlocked(streams []string) error {
	if len(streams) == 0 {
		return nil
	}

	wsm.connMu.Lock()
	conn := wsm.conn
	wsm.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	// Batch subscriptions per Binance limits
	batchSize := wsm.config.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	for i := 0; i < len(streams); i += batchSize {
		end := i + batchSize
		if end > len(streams) {
			end = len(streams)
		}
		batch := streams[i:end]

		msg := map[string]interface{}{
			"method": "SUBSCRIBE",
			"params": batch,
			"id":     time.Now().UnixNano(),
		}

		if err := conn.WriteJSON(msg); err != nil {
			return fmt.Errorf("subscribe failed: %w", err)
		}

		// Mark as subscribed
		for _, stream := range batch {
			wsm.activeSubscriptions[stream] = true
		}

		wsm.log("Subscribed to %d streams", len(batch))

		// Small delay between batches to avoid rate limiting
		if end < len(streams) {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// sendUnsubscribeUnlocked sends an unsubscribe request. Must be called with mu held.
func (wsm *WebSocketManager) sendUnsubscribeUnlocked(streams []string) error {
	if len(streams) == 0 {
		return nil
	}

	wsm.connMu.Lock()
	conn := wsm.conn
	wsm.connMu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"method": "UNSUBSCRIBE",
		"params": streams,
		"id":     time.Now().UnixNano(),
	}

	if err := conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("unsubscribe failed: %w", err)
	}

	// Mark as unsubscribed
	for _, stream := range streams {
		delete(wsm.activeSubscriptions, stream)
	}

	wsm.log("Unsubscribed from %d streams", len(streams))
	return nil
}

// processPendingUnlocked processes pending subscribe/unsubscribe requests. Must be called with mu held.
func (wsm *WebSocketManager) processPendingUnlocked() error {
	// Process unsubscribes first
	if len(wsm.pendingUnsubscribes) > 0 {
		if err := wsm.sendUnsubscribeUnlocked(wsm.pendingUnsubscribes); err != nil {
			return err
		}
		wsm.pendingUnsubscribes = wsm.pendingUnsubscribes[:0]
	}

	// Then process subscribes
	if len(wsm.pendingSubscribes) > 0 {
		if err := wsm.sendSubscribeUnlocked(wsm.pendingSubscribes); err != nil {
			return err
		}
		wsm.pendingSubscribes = wsm.pendingSubscribes[:0]
	}

	return nil
}

// setLastError stores the last error message.
func (wsm *WebSocketManager) setLastError(errMsg string) {
	wsm.mu.Lock()
	wsm.lastError = errMsg
	wsm.mu.Unlock()
}

// ============================================================================
// LOGGING HELPERS
// ============================================================================

// log logs a message with the WebSocket prefix.
func (wsm *WebSocketManager) log(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if wsm.profiler != nil && wsm.profiler.logger != nil {
		wsm.profiler.logger.Info(LogPrefixWS + " " + msg)
	} else {
		fmt.Printf("%s %s\n", LogPrefixWS, msg)
	}
}

// logDebug logs a debug message (only if debug mode is enabled).
func (wsm *WebSocketManager) logDebug(format string, args ...interface{}) {
	if wsm.config != nil && wsm.config.DebugMode {
		msg := fmt.Sprintf(format, args...)
		if wsm.profiler != nil && wsm.profiler.logger != nil {
			wsm.profiler.logger.Debug(LogPrefixWS + " " + msg)
		} else {
			fmt.Printf("%s [DEBUG] %s\n", LogPrefixWS, msg)
		}
	}
}

// logError logs an error message.
func (wsm *WebSocketManager) logError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if wsm.profiler != nil && wsm.profiler.logger != nil {
		wsm.profiler.logger.Error(LogPrefixWS + " " + msg)
	} else {
		fmt.Printf("%s [ERROR] %s\n", LogPrefixWS, msg)
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// buildKlineStream constructs a kline stream name from symbol and timeframe.
// Returns format: btcusdt@kline_5m
func buildKlineStream(symbol, timeframe string) string {
	return strings.ToLower(symbol) + "@kline_" + timeframe
}

// parseKlineStream extracts symbol and timeframe from a kline stream name.
// Returns symbol (uppercase) and timeframe.
func parseKlineStream(stream string) (symbol, timeframe string, ok bool) {
	// Format: btcusdt@kline_5m
	parts := strings.Split(stream, "@kline_")
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.ToUpper(parts[0]), parts[1], true
}

// BuildKlineStreamsForRequirements creates kline stream names from combined requirements.
func BuildKlineStreamsForRequirements(requirements *CombinedRequirements) []string {
	if requirements == nil {
		return nil
	}

	streams := make([]string, 0)
	for symbol, symReq := range requirements.BySymbol {
		for _, tf := range symReq.Timeframes {
			streams = append(streams, buildKlineStream(symbol, tf))
		}
	}

	return streams
}

// BuildCombinedStreamURL constructs the combined streams WebSocket URL.
// Example: wss://fstream.binance.com/stream?streams=btcusdt@kline_5m/ethusdt@kline_5m
func BuildCombinedStreamURL(baseURL string, streams []string) string {
	if len(streams) == 0 {
		return baseURL
	}

	// Remove /ws suffix if present
	base := strings.TrimSuffix(baseURL, "/ws")

	// URL encode the streams
	streamParam := url.QueryEscape(strings.Join(streams, "/"))

	return fmt.Sprintf("%s/stream?streams=%s", base, streamParam)
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
