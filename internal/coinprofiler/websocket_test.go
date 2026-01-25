package coinprofiler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// mockWebSocketServer creates a mock WebSocket server for testing
type mockWebSocketServer struct {
	server       *httptest.Server
	upgrader     websocket.Upgrader
	connections  []*websocket.Conn
	mu           sync.Mutex
	onMessage    func(msg []byte) []byte
	onConnect    func(conn *websocket.Conn)
	onDisconnect func(conn *websocket.Conn)
}

func newMockWebSocketServer() *mockWebSocketServer {
	mock := &mockWebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make([]*websocket.Conn, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := mock.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		mock.mu.Lock()
		mock.connections = append(mock.connections, conn)
		mock.mu.Unlock()

		if mock.onConnect != nil {
			mock.onConnect(conn)
		}

		// Handle messages
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}

			if mock.onMessage != nil {
				response := mock.onMessage(msg)
				if response != nil {
					conn.WriteMessage(websocket.TextMessage, response)
				}
			}
		}

		if mock.onDisconnect != nil {
			mock.onDisconnect(conn)
		}
	}))

	return mock
}

func (m *mockWebSocketServer) URL() string {
	return "ws" + strings.TrimPrefix(m.server.URL, "http")
}

func (m *mockWebSocketServer) Close() {
	m.mu.Lock()
	for _, conn := range m.connections {
		conn.Close()
	}
	m.mu.Unlock()
	m.server.Close()
}

func (m *mockWebSocketServer) SendToAll(msg []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, conn := range m.connections {
		conn.WriteMessage(websocket.TextMessage, msg)
	}
}

// ============================================================================
// WEBSOCKET MANAGER TESTS
// ============================================================================

func TestNewWebSocketManager(t *testing.T) {
	t.Run("creates with default config when nil", func(t *testing.T) {
		wsm := NewWebSocketManager(nil, nil)
		if wsm == nil {
			t.Fatal("expected non-nil WebSocketManager")
		}
		if wsm.config == nil {
			t.Error("expected config to be initialized")
		}
		if wsm.connectionState != ConnectionStateDisconnected {
			t.Errorf("expected disconnected state, got %s", wsm.connectionState.String())
		}
	})

	t.Run("creates with custom config", func(t *testing.T) {
		config := &CoinProfilerConfig{
			WebSocketEndpoint:  "wss://custom.endpoint/ws",
			PingInterval:       1 * time.Minute,
			MaxSubscriptions:   100,
			ReconnectDelay:     2 * time.Second,
			MaxReconnectDelay:  1 * time.Minute,
			ReconnectBackoff:   1.5,
		}

		wsm := NewWebSocketManager(config, nil)
		if wsm.config.WebSocketEndpoint != "wss://custom.endpoint/ws" {
			t.Errorf("expected custom endpoint, got %s", wsm.config.WebSocketEndpoint)
		}
		if wsm.config.MaxSubscriptions != 100 {
			t.Errorf("expected max subscriptions 100, got %d", wsm.config.MaxSubscriptions)
		}
	})

	t.Run("creates with profiler reference", func(t *testing.T) {
		profiler := NewCoinProfiler(nil, nil)
		wsm := NewWebSocketManager(nil, profiler)
		if wsm.profiler != profiler {
			t.Error("expected profiler reference to be set")
		}
	})
}

func TestWebSocketManager_StartStop(t *testing.T) {
	t.Run("start changes state to connecting", func(t *testing.T) {
		wsm := NewWebSocketManager(&CoinProfilerConfig{
			WebSocketEndpoint: "wss://invalid.endpoint/ws", // Won't connect
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 200 * time.Millisecond,
			ReconnectBackoff:  1.5,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
		}, nil)

		err := wsm.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}

		// Give a moment for the goroutine to start
		time.Sleep(50 * time.Millisecond)

		wsm.mu.RLock()
		state := wsm.connectionState
		wsm.mu.RUnlock()

		// State should be connecting or disconnected (if connection attempt failed quickly)
		if state != ConnectionStateConnecting && state != ConnectionStateDisconnected && state != ConnectionStateReconnecting {
			t.Errorf("unexpected state: %s", state.String())
		}

		// Stop should not error
		wsm.Stop()
	})

	t.Run("double start returns error", func(t *testing.T) {
		wsm := NewWebSocketManager(&CoinProfilerConfig{
			WebSocketEndpoint: "wss://invalid.endpoint/ws",
			ReconnectDelay:    1 * time.Second,
			MaxReconnectDelay: 2 * time.Second,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
		}, nil)

		wsm.Start()
		defer wsm.Stop()

		// Wait for state to change
		time.Sleep(50 * time.Millisecond)

		err := wsm.Start()
		if err == nil {
			t.Error("expected error on double start")
		}
	})

	t.Run("stop on stopped manager does not error", func(t *testing.T) {
		wsm := NewWebSocketManager(nil, nil)
		err := wsm.Stop()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestWebSocketManager_ConnectionLifecycle(t *testing.T) {
	t.Run("connects to mock server successfully", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		connectCount := int32(0)
		mock.onConnect = func(conn *websocket.Conn) {
			atomic.AddInt32(&connectCount, 1)
		}

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		err := wsm.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer wsm.Stop()

		// Wait for connection
		time.Sleep(200 * time.Millisecond)

		if atomic.LoadInt32(&connectCount) != 1 {
			t.Errorf("expected 1 connection, got %d", connectCount)
		}

		status := wsm.GetStatus()
		if !status.Connected {
			t.Error("expected connected status")
		}
	})

	t.Run("reconnects after disconnect", func(t *testing.T) {
		mock := newMockWebSocketServer()

		connectCount := int32(0)
		mock.onConnect = func(conn *websocket.Conn) {
			count := atomic.AddInt32(&connectCount, 1)
			// Close first connection to trigger reconnect
			if count == 1 {
				time.Sleep(100 * time.Millisecond)
				conn.Close()
			}
		}

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    50 * time.Millisecond,
			MaxReconnectDelay: 200 * time.Millisecond,
			ReconnectBackoff:  1.5,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		err := wsm.Start()
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		defer func() {
			mock.Close()
			wsm.Stop()
		}()

		// Wait for reconnect
		time.Sleep(500 * time.Millisecond)

		if atomic.LoadInt32(&connectCount) < 2 {
			t.Errorf("expected at least 2 connections (reconnect), got %d", connectCount)
		}

		// Note: reconnect count is incremented when connection fails and reconnect starts,
		// but since we're successfully reconnecting, the count may vary by timing
		// The important assertion is that we got at least 2 connections
	})
}

func TestWebSocketManager_Subscriptions(t *testing.T) {
	t.Run("subscribe queues when not connected", func(t *testing.T) {
		wsm := NewWebSocketManager(&CoinProfilerConfig{
			MaxSubscriptions: 200,
		}, nil)

		err := wsm.Subscribe([]string{"btcusdt@kline_5m", "ethusdt@kline_15m"})
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		wsm.mu.RLock()
		pendingCount := len(wsm.pendingSubscribes)
		wsm.mu.RUnlock()

		if pendingCount != 2 {
			t.Errorf("expected 2 pending subscriptions, got %d", pendingCount)
		}
	})

	t.Run("subscribe sends request when connected", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		subscriptions := make([]string, 0)
		var mu sync.Mutex

		mock.onMessage = func(msg []byte) []byte {
			var req struct {
				Method string   `json:"method"`
				Params []string `json:"params"`
				ID     int64    `json:"id"`
			}
			if err := json.Unmarshal(msg, &req); err == nil && req.Method == "SUBSCRIBE" {
				mu.Lock()
				subscriptions = append(subscriptions, req.Params...)
				mu.Unlock()

				// Send success response
				resp, _ := json.Marshal(map[string]interface{}{
					"result": nil,
					"id":     req.ID,
				})
				return resp
			}
			return nil
		}

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
			BatchSize:         50,
		}

		wsm := NewWebSocketManager(config, nil)
		wsm.Start()
		defer wsm.Stop()

		// Wait for connection
		time.Sleep(200 * time.Millisecond)

		// Subscribe while connected
		err := wsm.Subscribe([]string{"btcusdt@kline_5m", "ethusdt@kline_15m"})
		if err != nil {
			t.Fatalf("subscribe failed: %v", err)
		}

		// Wait for subscription to be processed
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		subCount := len(subscriptions)
		mu.Unlock()

		if subCount != 2 {
			t.Errorf("expected 2 subscriptions, got %d", subCount)
		}

		wsm.mu.RLock()
		activeCount := len(wsm.activeSubscriptions)
		wsm.mu.RUnlock()

		if activeCount != 2 {
			t.Errorf("expected 2 active subscriptions, got %d", activeCount)
		}
	})

	t.Run("unsubscribe removes streams", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		unsubscribed := make([]string, 0)
		var mu sync.Mutex

		mock.onMessage = func(msg []byte) []byte {
			var req struct {
				Method string   `json:"method"`
				Params []string `json:"params"`
				ID     int64    `json:"id"`
			}
			if err := json.Unmarshal(msg, &req); err == nil {
				if req.Method == "UNSUBSCRIBE" {
					mu.Lock()
					unsubscribed = append(unsubscribed, req.Params...)
					mu.Unlock()
				}

				resp, _ := json.Marshal(map[string]interface{}{
					"result": nil,
					"id":     req.ID,
				})
				return resp
			}
			return nil
		}

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		wsm.Start()
		defer wsm.Stop()

		// Wait for connection and subscribe
		time.Sleep(200 * time.Millisecond)
		wsm.Subscribe([]string{"btcusdt@kline_5m", "ethusdt@kline_15m"})
		time.Sleep(100 * time.Millisecond)

		// Unsubscribe
		err := wsm.Unsubscribe([]string{"btcusdt@kline_5m"})
		if err != nil {
			t.Fatalf("unsubscribe failed: %v", err)
		}

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		unsubCount := len(unsubscribed)
		mu.Unlock()

		if unsubCount != 1 {
			t.Errorf("expected 1 unsubscription, got %d", unsubCount)
		}
	})

	t.Run("subscription limit enforced", func(t *testing.T) {
		wsm := NewWebSocketManager(&CoinProfilerConfig{
			MaxSubscriptions: 2,
		}, nil)

		// Should succeed
		err := wsm.Subscribe([]string{"btcusdt@kline_5m", "ethusdt@kline_15m"})
		if err != nil {
			t.Fatalf("first subscribe failed: %v", err)
		}

		// Should fail - exceeds limit
		err = wsm.Subscribe([]string{"bnbusdt@kline_5m"})
		if err == nil {
			t.Error("expected error when exceeding subscription limit")
		}
	})

	t.Run("duplicate subscriptions ignored", func(t *testing.T) {
		wsm := NewWebSocketManager(&CoinProfilerConfig{
			MaxSubscriptions: 200,
		}, nil)

		wsm.Subscribe([]string{"btcusdt@kline_5m"})
		wsm.Subscribe([]string{"btcusdt@kline_5m"}) // Duplicate

		wsm.mu.RLock()
		pendingCount := len(wsm.pendingSubscribes)
		wsm.mu.RUnlock()

		if pendingCount != 1 {
			t.Errorf("expected 1 pending subscription (duplicate ignored), got %d", pendingCount)
		}
	})
}

func TestWebSocketManager_MessageParsing(t *testing.T) {
	t.Run("parses kline message correctly", func(t *testing.T) {
		// Test the processKlineMessage function directly to avoid WebSocket timing issues
		profiler := NewCoinProfiler(nil, nil)
		config := &CoinProfilerConfig{
			WebSocketEndpoint:    "wss://test.endpoint/ws",
			MaxSubscriptions:     200,
			EnableDataValidation: true,
		}

		wsm := NewWebSocketManager(config, profiler)

		// Kline message to be processed
		klineData := []byte(`{
			"e": "kline",
			"E": 1672531200000,
			"s": "BTCUSDT",
			"k": {
				"t": 1672531140000,
				"T": 1672531199999,
				"s": "BTCUSDT",
				"i": "5m",
				"f": 100,
				"L": 200,
				"o": "16500.00",
				"c": "16600.00",
				"h": "16650.00",
				"l": "16450.00",
				"v": "100.50",
				"n": 500,
				"x": false,
				"q": "1660000.00",
				"V": "60.30",
				"Q": "998000.00"
			}
		}`)

		// Process the message directly
		wsm.processKlineMessage(klineData)

		// Check that profiler received the data
		coinData := profiler.GetCoinData("BTCUSDT")
		if coinData == nil {
			t.Fatal("expected coin data to be created")
		}

		if coinData.Price != 16600.0 {
			t.Errorf("expected price 16600.0, got %f", coinData.Price)
		}

		tfData := coinData.Timeframes["5m"]
		if tfData == nil {
			t.Fatal("expected 5m timeframe data")
		}

		if tfData.Open != 16500.0 {
			t.Errorf("expected open 16500.0, got %f", tfData.Open)
		}
		if tfData.High != 16650.0 {
			t.Errorf("expected high 16650.0, got %f", tfData.High)
		}
		if tfData.Low != 16450.0 {
			t.Errorf("expected low 16450.0, got %f", tfData.Low)
		}
		if tfData.Close != 16600.0 {
			t.Errorf("expected close 16600.0, got %f", tfData.Close)
		}
		if tfData.Volume != 100.50 {
			t.Errorf("expected volume 100.50, got %f", tfData.Volume)
		}
		if tfData.TakerBuyVol != 60.30 {
			t.Errorf("expected taker buy vol 60.30, got %f", tfData.TakerBuyVol)
		}
		if tfData.TakerSellVol != 40.20 { // 100.50 - 60.30
			t.Errorf("expected taker sell vol 40.20, got %f", tfData.TakerSellVol)
		}
		if tfData.TradeCount != 500 {
			t.Errorf("expected trade count 500, got %d", tfData.TradeCount)
		}
	})

	t.Run("parses combined stream format", func(t *testing.T) {
		// Test the processStreamMessage function directly to avoid WebSocket timing issues
		profiler := NewCoinProfiler(nil, nil)
		config := &CoinProfilerConfig{
			WebSocketEndpoint:    "wss://test.endpoint/ws",
			MaxSubscriptions:     200,
			EnableDataValidation: true,
		}

		wsm := NewWebSocketManager(config, profiler)

		// Kline data from a combined stream (the "data" portion)
		klineData := []byte(`{
			"e": "kline",
			"E": 1672531200000,
			"s": "ETHUSDT",
			"k": {
				"t": 1672531140000,
				"T": 1672531199999,
				"s": "ETHUSDT",
				"i": "15m",
				"f": 100,
				"L": 200,
				"o": "1200.00",
				"c": "1210.00",
				"h": "1220.00",
				"l": "1190.00",
				"v": "500.00",
				"n": 1000,
				"x": true,
				"q": "600000.00",
				"V": "300.00",
				"Q": "360000.00"
			}
		}`)

		// Process the message directly (simulating combined stream format)
		wsm.processStreamMessage("ethusdt@kline_15m", klineData)

		// Check that profiler received the data
		coinData := profiler.GetCoinData("ETHUSDT")
		if coinData == nil {
			t.Fatal("expected coin data to be created")
		}

		tfData := coinData.Timeframes["15m"]
		if tfData == nil {
			t.Fatal("expected 15m timeframe data")
		}

		if !tfData.IsClosedBar {
			t.Error("expected closed bar flag to be true")
		}

		if tfData.Close != 1210.0 {
			t.Errorf("expected close 1210.0, got %f", tfData.Close)
		}
	})

	t.Run("handles subscription response", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		wsm.Start()
		defer wsm.Stop()

		// Wait for connection
		time.Sleep(200 * time.Millisecond)

		// Send subscription response (should not cause errors)
		respMsg := `{"result": null, "id": 1}`
		mock.SendToAll([]byte(respMsg))

		time.Sleep(100 * time.Millisecond)

		// Check no errors were recorded
		status := wsm.GetStatus()
		if status.ErrorCount > 0 {
			t.Errorf("expected no errors, got %d", status.ErrorCount)
		}
	})

	t.Run("handles subscription error response", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		wsm.Start()
		defer wsm.Stop()

		// Wait for connection
		time.Sleep(200 * time.Millisecond)

		// Send error response
		errMsg := `{"error": {"code": -1, "msg": "Invalid stream"}, "id": 1}`
		mock.SendToAll([]byte(errMsg))

		time.Sleep(100 * time.Millisecond)

		status := wsm.GetStatus()
		if status.ErrorCount != 1 {
			t.Errorf("expected 1 error, got %d", status.ErrorCount)
		}
	})
}

func TestWebSocketManager_UpdateSubscriptions(t *testing.T) {
	t.Run("updates subscriptions from requirements", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		subscribed := make(map[string]bool)
		var mu sync.Mutex

		mock.onMessage = func(msg []byte) []byte {
			var req struct {
				Method string   `json:"method"`
				Params []string `json:"params"`
				ID     int64    `json:"id"`
			}
			if err := json.Unmarshal(msg, &req); err == nil {
				mu.Lock()
				if req.Method == "SUBSCRIBE" {
					for _, s := range req.Params {
						subscribed[s] = true
					}
				} else if req.Method == "UNSUBSCRIBE" {
					for _, s := range req.Params {
						delete(subscribed, s)
					}
				}
				mu.Unlock()

				resp, _ := json.Marshal(map[string]interface{}{
					"result": nil,
					"id":     req.ID,
				})
				return resp
			}
			return nil
		}

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		wsm.Start()
		defer wsm.Stop()

		// Wait for connection
		time.Sleep(200 * time.Millisecond)

		// Create requirements
		requirements := &CombinedRequirements{
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {
					Symbol:     "BTCUSDT",
					Timeframes: []string{"5m", "15m"},
				},
				"ETHUSDT": {
					Symbol:     "ETHUSDT",
					Timeframes: []string{"1h"},
				},
			},
		}

		err := wsm.UpdateSubscriptions(requirements)
		if err != nil {
			t.Fatalf("update subscriptions failed: %v", err)
		}

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		subCount := len(subscribed)
		hasBTC5m := subscribed["btcusdt@kline_5m"]
		hasBTC15m := subscribed["btcusdt@kline_15m"]
		hasETH1h := subscribed["ethusdt@kline_1h"]
		mu.Unlock()

		if subCount != 3 {
			t.Errorf("expected 3 subscriptions, got %d", subCount)
		}
		if !hasBTC5m {
			t.Error("expected btcusdt@kline_5m subscription")
		}
		if !hasBTC15m {
			t.Error("expected btcusdt@kline_15m subscription")
		}
		if !hasETH1h {
			t.Error("expected ethusdt@kline_1h subscription")
		}
	})
}

func TestWebSocketManager_GetStatus(t *testing.T) {
	t.Run("returns correct status when disconnected", func(t *testing.T) {
		wsm := NewWebSocketManager(nil, nil)

		status := wsm.GetStatus()
		if status.Connected {
			t.Error("expected connected to be false")
		}
		if status.ConnectionState != "disconnected" {
			t.Errorf("expected disconnected state, got %s", status.ConnectionState)
		}
	})

	t.Run("returns correct status when connected", func(t *testing.T) {
		mock := newMockWebSocketServer()
		defer mock.Close()

		config := &CoinProfilerConfig{
			WebSocketEndpoint: mock.URL(),
			ReconnectDelay:    100 * time.Millisecond,
			MaxReconnectDelay: 500 * time.Millisecond,
			ReconnectBackoff:  2.0,
			PingInterval:      1 * time.Second,
			PongTimeout:       500 * time.Millisecond,
			MaxSubscriptions:  200,
		}

		wsm := NewWebSocketManager(config, nil)
		wsm.Start()
		defer wsm.Stop()

		// Wait for connection
		time.Sleep(200 * time.Millisecond)

		status := wsm.GetStatus()
		if !status.Connected {
			t.Error("expected connected to be true")
		}
		if status.ConnectionState != "connected" {
			t.Errorf("expected connected state, got %s", status.ConnectionState)
		}
	})
}

// ============================================================================
// HELPER FUNCTION TESTS
// ============================================================================

func TestBuildKlineStream(t *testing.T) {
	tests := []struct {
		symbol    string
		timeframe string
		expected  string
	}{
		{"BTCUSDT", "5m", "btcusdt@kline_5m"},
		{"btcusdt", "15m", "btcusdt@kline_15m"},
		{"ETHUSDT", "1h", "ethusdt@kline_1h"},
		{"BNBusdt", "4h", "bnbusdt@kline_4h"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol+"_"+tt.timeframe, func(t *testing.T) {
			result := buildKlineStream(tt.symbol, tt.timeframe)
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestParseKlineStream(t *testing.T) {
	tests := []struct {
		stream      string
		wantSymbol  string
		wantTF      string
		wantOK      bool
	}{
		{"btcusdt@kline_5m", "BTCUSDT", "5m", true},
		{"ethusdt@kline_1h", "ETHUSDT", "1h", true},
		{"bnbusdt@kline_4h", "BNBUSDT", "4h", true},
		{"invalid_stream", "", "", false},
		{"btcusdt@depth", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.stream, func(t *testing.T) {
			symbol, tf, ok := parseKlineStream(tt.stream)
			if ok != tt.wantOK {
				t.Errorf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if ok && symbol != tt.wantSymbol {
				t.Errorf("symbol: got %s, want %s", symbol, tt.wantSymbol)
			}
			if ok && tf != tt.wantTF {
				t.Errorf("timeframe: got %s, want %s", tf, tt.wantTF)
			}
		})
	}
}

func TestBuildKlineStreamsForRequirements(t *testing.T) {
	t.Run("returns nil for nil requirements", func(t *testing.T) {
		streams := BuildKlineStreamsForRequirements(nil)
		if streams != nil {
			t.Error("expected nil streams")
		}
	})

	t.Run("builds streams from requirements", func(t *testing.T) {
		requirements := &CombinedRequirements{
			BySymbol: map[string]*SymbolRequirements{
				"BTCUSDT": {
					Symbol:     "BTCUSDT",
					Timeframes: []string{"5m", "15m"},
				},
				"ETHUSDT": {
					Symbol:     "ETHUSDT",
					Timeframes: []string{"1h"},
				},
			},
		}

		streams := BuildKlineStreamsForRequirements(requirements)
		if len(streams) != 3 {
			t.Errorf("expected 3 streams, got %d", len(streams))
		}

		// Check streams are present (order may vary)
		streamSet := make(map[string]bool)
		for _, s := range streams {
			streamSet[s] = true
		}

		expected := []string{"btcusdt@kline_5m", "btcusdt@kline_15m", "ethusdt@kline_1h"}
		for _, e := range expected {
			if !streamSet[e] {
				t.Errorf("expected stream %s not found", e)
			}
		}
	})
}

func TestBuildCombinedStreamURL(t *testing.T) {
	tests := []struct {
		baseURL  string
		streams  []string
		contains string
	}{
		{"wss://fstream.binance.com/ws", []string{}, "wss://fstream.binance.com/ws"},
		{"wss://fstream.binance.com/ws", []string{"btcusdt@kline_5m"}, "stream?streams="},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			result := BuildCombinedStreamURL(tt.baseURL, tt.streams)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected URL to contain %s, got %s", tt.contains, result)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{-1, 0, -1},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

// ============================================================================
// CONCURRENCY TESTS
// ============================================================================

func TestWebSocketManager_ConcurrentOperations(t *testing.T) {
	t.Run("concurrent subscribe calls are safe", func(t *testing.T) {
		wsm := NewWebSocketManager(&CoinProfilerConfig{
			MaxSubscriptions: 1000,
		}, nil)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				stream := "btcusdt@kline_" + time.Duration(n).String()
				wsm.Subscribe([]string{stream})
			}(i)
		}
		wg.Wait()

		// Should not panic
		wsm.mu.RLock()
		pendingCount := len(wsm.pendingSubscribes)
		wsm.mu.RUnlock()

		if pendingCount == 0 {
			t.Error("expected some pending subscriptions")
		}
	})

	t.Run("concurrent status reads are safe", func(t *testing.T) {
		wsm := NewWebSocketManager(nil, nil)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = wsm.GetStatus()
			}()
		}
		wg.Wait()
		// Should not panic
	})
}
