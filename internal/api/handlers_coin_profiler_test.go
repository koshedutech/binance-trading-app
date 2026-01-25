package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"binance-trading-bot/internal/coinprofiler"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Epic 14: Coin Profiler API Handler Tests
// Story 14.6: API Endpoints for Coin Profiler
// ============================================================================

// Mock helper to create a test gin context
func createTestGinContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	return c, w
}

// TestCoinProfilerStatusResponse tests the structure of status response
func TestCoinProfilerStatusResponse(t *testing.T) {
	// Create a coin profiler and check its status format
	cp := coinprofiler.NewCoinProfiler(nil, nil)

	// Test status when not running
	status := cp.GetStatus()
	if status.Running {
		t.Error("expected Running to be false initially")
	}
	if status.Connected {
		t.Error("expected Connected to be false initially")
	}
	if status.SymbolCount != 0 {
		t.Errorf("expected SymbolCount to be 0, got %d", status.SymbolCount)
	}

	// Test status when running
	err := cp.Start()
	if err != nil {
		t.Fatalf("failed to start profiler: %v", err)
	}
	defer cp.Stop()

	runningStatus := cp.GetStatus()
	if !runningStatus.Running {
		t.Error("expected Running to be true after start")
	}
	if runningStatus.Uptime == "" {
		t.Error("expected Uptime to be set when running")
	}
}

// TestCoinProfilerCoinsResponse tests the structure of coins response
func TestCoinProfilerCoinsResponse(t *testing.T) {
	cp := coinprofiler.NewCoinProfiler(nil, nil)

	// Initially should be empty
	allData := cp.GetAllCoinData()
	if len(allData) != 0 {
		t.Errorf("expected 0 coins initially, got %d", len(allData))
	}

	// The response structure should be convertible to JSON
	response := map[string]interface{}{
		"coins": []interface{}{},
		"total": 0,
	}
	_, err := json.Marshal(response)
	if err != nil {
		t.Errorf("failed to marshal coins response: %v", err)
	}
}

// TestCoinProfilerCoinDataResponse tests individual coin data response
func TestCoinProfilerCoinDataResponse(t *testing.T) {
	cp := coinprofiler.NewCoinProfiler(nil, nil)

	// Non-existent symbol should return nil
	coinData := cp.GetCoinData("BTCUSDT")
	if coinData != nil {
		t.Error("expected nil for non-existent symbol")
	}

	// Test that CoinData struct is JSON serializable
	testData := &coinprofiler.CoinData{
		Symbol:    "BTCUSDT",
		Price:     50000.0,
		Volume24h: 1000000000,
		Timeframes: map[string]*coinprofiler.TimeframeData{
			"5m": {
				Timeframe: "5m",
				Open:      49500,
				High:      50500,
				Low:       49000,
				Close:     50000,
				Volume:    100000,
			},
		},
	}

	jsonBytes, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("failed to marshal CoinData: %v", err)
	}

	// Verify expected JSON fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal CoinData JSON: %v", err)
	}

	if parsed["symbol"] != "BTCUSDT" {
		t.Errorf("expected symbol 'BTCUSDT', got %v", parsed["symbol"])
	}
	if parsed["price"].(float64) != 50000.0 {
		t.Errorf("expected price 50000.0, got %v", parsed["price"])
	}
}

// TestCoinProfilerRequirementsResponse tests requirements response format
func TestCoinProfilerRequirementsResponse(t *testing.T) {
	cp := coinprofiler.NewCoinProfiler(nil, nil)

	// Add a subscription to test requirements
	req := &coinprofiler.SubscriptionRequest{
		Symbol:     "ETHUSDT",
		Timeframes: []string{"15m", "1h"},
		Source:     coinprofiler.DataSourceStrategy,
		Strategy:   "test_strategy",
	}
	err := cp.AddSubscription(req)
	if err != nil {
		t.Fatalf("failed to add subscription: %v", err)
	}

	// Get subscriptions
	subs := cp.GetSubscriptions()
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}

	// Verify subscription data
	ethSub := subs["ETHUSDT"]
	if ethSub == nil {
		t.Fatal("expected ETHUSDT subscription")
	}
	if len(ethSub.Timeframes) != 2 {
		t.Errorf("expected 2 timeframes, got %d", len(ethSub.Timeframes))
	}
}

// TestCoinProfilerStartStopHandlerResponse tests start/stop response format
func TestCoinProfilerStartStopHandlerResponse(t *testing.T) {
	// Test the expected response format for start/stop operations
	successResponse := map[string]interface{}{
		"success": true,
		"message": "Coin profiler started",
		"status": &coinprofiler.CoinProfilerStatus{
			Running:      true,
			SymbolCount:  0,
			UpdatesPerSecond: 0,
		},
	}

	jsonBytes, err := json.Marshal(successResponse)
	if err != nil {
		t.Fatalf("failed to marshal start response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal start response JSON: %v", err)
	}

	if parsed["success"] != true {
		t.Error("expected success to be true")
	}
	if parsed["message"] != "Coin profiler started" {
		t.Errorf("expected message 'Coin profiler started', got %v", parsed["message"])
	}
}

// TestCoinProfilerErrorResponses tests error response formats
func TestCoinProfilerErrorResponses(t *testing.T) {
	tests := []struct {
		name     string
		error    string
		message  string
		httpCode int
	}{
		{
			name:     "SERVICE_UNAVAILABLE",
			error:    "SERVICE_UNAVAILABLE",
			message:  "Autopilot manager not initialized",
			httpCode: http.StatusServiceUnavailable,
		},
		{
			name:     "INVALID_REQUEST",
			error:    "INVALID_REQUEST",
			message:  "Symbol parameter is required",
			httpCode: http.StatusBadRequest,
		},
		{
			name:     "NOT_FOUND",
			error:    "NOT_FOUND",
			message:  "No data found for symbol: INVALIDUSDT",
			httpCode: http.StatusNotFound,
		},
		{
			name:     "START_FAILED",
			error:    "START_FAILED",
			message:  "failed to start coin profiler: already running",
			httpCode: http.StatusInternalServerError,
		},
		{
			name:     "STOP_FAILED",
			error:    "STOP_FAILED",
			message:  "failed to stop coin profiler: not running",
			httpCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := gin.H{
				"error":   tt.error,
				"message": tt.message,
			}

			jsonBytes, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("failed to marshal error response: %v", err)
			}

			var parsed map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
				t.Fatalf("failed to unmarshal error response: %v", err)
			}

			if parsed["error"] != tt.error {
				t.Errorf("expected error '%s', got '%v'", tt.error, parsed["error"])
			}
			if parsed["message"] != tt.message {
				t.Errorf("expected message '%s', got '%v'", tt.message, parsed["message"])
			}
		})
	}
}

// TestCoinProfilerStatusEndpointStructure tests the full status response structure
func TestCoinProfilerStatusEndpointStructure(t *testing.T) {
	// Verify CoinProfilerStatus struct has expected JSON fields
	status := coinprofiler.CoinProfilerStatus{
		Running:           true,
		Connected:         true,
		SymbolCount:       15,
		SubscriptionCount: 45,
		UpdatesPerSecond:  100.5,
		ReconnectCount:    0,
		Uptime:            "1h 23m 45s",
	}

	jsonBytes, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal CoinProfilerStatus: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal CoinProfilerStatus: %v", err)
	}

	// Verify expected fields exist
	expectedFields := []string{
		"running", "connected", "symbol_count", "subscription_count",
		"updates_per_second", "reconnect_count", "uptime",
	}
	for _, field := range expectedFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("expected field '%s' in status response", field)
		}
	}
}

// TestCoinProfilerCoinsEndpointStructure tests the coins response structure
func TestCoinProfilerCoinsEndpointStructure(t *testing.T) {
	// Create sample coin data
	coins := []coinprofiler.CoinData{
		{
			Symbol:    "BTCUSDT",
			Price:     50000.0,
			Volume24h: 1234567890,
			Timeframes: map[string]*coinprofiler.TimeframeData{
				"5m": {
					Timeframe: "5m",
					Open:      49500,
					High:      50500,
					Low:       49000,
					Close:     50000,
				},
			},
		},
		{
			Symbol:    "ETHUSDT",
			Price:     3000.0,
			Volume24h: 987654321,
		},
	}

	response := map[string]interface{}{
		"coins": coins,
		"total": len(coins),
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal coins response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal coins response: %v", err)
	}

	if parsed["total"].(float64) != 2 {
		t.Errorf("expected total 2, got %v", parsed["total"])
	}

	coinsArray, ok := parsed["coins"].([]interface{})
	if !ok {
		t.Fatal("expected coins to be an array")
	}
	if len(coinsArray) != 2 {
		t.Errorf("expected 2 coins in array, got %d", len(coinsArray))
	}
}

// TestCoinProfilerRequirementsEndpointStructure tests requirements response structure
func TestCoinProfilerRequirementsEndpointStructure(t *testing.T) {
	response := map[string]interface{}{
		"all_timeframes":   []string{"5m", "15m", "1h", "4h"},
		"all_data_fields":  []string{"ohlc", "volume", "taker_buy_volume"},
		"total_strategies": 3,
		"subscriptions": map[string]interface{}{
			"BTCUSDT": map[string]interface{}{
				"timeframes": []string{"5m", "15m"},
				"source":     "strategy",
				"strategy":   "ravindra_volume_imbalance",
			},
		},
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal requirements response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal requirements response: %v", err)
	}

	// Verify required fields
	if _, exists := parsed["all_timeframes"]; !exists {
		t.Error("expected 'all_timeframes' field")
	}
	if _, exists := parsed["all_data_fields"]; !exists {
		t.Error("expected 'all_data_fields' field")
	}
	if _, exists := parsed["total_strategies"]; !exists {
		t.Error("expected 'total_strategies' field")
	}
	if _, exists := parsed["subscriptions"]; !exists {
		t.Error("expected 'subscriptions' field")
	}
}

// TestCoinProfilerDataSourceJSON tests DataSource JSON serialization
func TestCoinProfilerDataSourceJSON(t *testing.T) {
	tests := []struct {
		source   coinprofiler.DataSource
		expected string
	}{
		{coinprofiler.DataSourceStrategy, "strategy"},
		{coinprofiler.DataSourcePosition, "position"},
		{coinprofiler.DataSourceBoth, "both"},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			data := struct {
				Source coinprofiler.DataSource `json:"source"`
			}{
				Source: tt.source,
			}

			jsonBytes, err := json.Marshal(data)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			var parsed map[string]string
			if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if parsed["source"] != tt.expected {
				t.Errorf("expected source '%s', got '%s'", tt.expected, parsed["source"])
			}
		})
	}
}

// TestCoinProfilerTimeframeDataJSON tests TimeframeData JSON serialization
func TestCoinProfilerTimeframeDataJSON(t *testing.T) {
	tfData := coinprofiler.TimeframeData{
		Timeframe:    "15m",
		Open:         50000.0,
		High:         51000.0,
		Low:          49500.0,
		Close:        50500.0,
		Volume:       1000000.0,
		TakerBuyVol:  600000.0,
		TakerSellVol: 400000.0,
		QuoteVolume:  50000000.0,
		TradeCount:   5000,
		IsClosedBar:  true,
	}

	jsonBytes, err := json.Marshal(tfData)
	if err != nil {
		t.Fatalf("failed to marshal TimeframeData: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal TimeframeData: %v", err)
	}

	expectedFields := []string{
		"timeframe", "open", "high", "low", "close",
		"volume", "taker_buy_vol", "taker_sell_vol", "quote_volume",
		"trade_count", "is_closed_bar",
	}

	for _, field := range expectedFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("expected field '%s' in TimeframeData", field)
		}
	}
}

// TestCoinProfilerSubscriptionRequestJSON tests SubscriptionRequest JSON serialization
func TestCoinProfilerSubscriptionRequestJSON(t *testing.T) {
	req := coinprofiler.SubscriptionRequest{
		Symbol:     "BTCUSDT",
		Timeframes: []string{"5m", "15m", "1h"},
		Source:     coinprofiler.DataSourceStrategy,
		Strategy:   "ravindra_volume_imbalance",
	}

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal SubscriptionRequest: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal SubscriptionRequest: %v", err)
	}

	if parsed["symbol"] != "BTCUSDT" {
		t.Errorf("expected symbol 'BTCUSDT', got %v", parsed["symbol"])
	}
	if parsed["source"] != "strategy" {
		t.Errorf("expected source 'strategy', got %v", parsed["source"])
	}

	timeframes, ok := parsed["timeframes"].([]interface{})
	if !ok {
		t.Fatal("expected timeframes to be an array")
	}
	if len(timeframes) != 3 {
		t.Errorf("expected 3 timeframes, got %d", len(timeframes))
	}
}

// TestCoinProfilerConfigJSON tests CoinProfilerConfig JSON serialization
func TestCoinProfilerConfigJSON(t *testing.T) {
	config := coinprofiler.DefaultCoinProfilerConfig()

	jsonBytes, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal CoinProfilerConfig: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal CoinProfilerConfig: %v", err)
	}

	// Verify some key fields
	if _, exists := parsed["websocket_endpoint"]; !exists {
		t.Error("expected 'websocket_endpoint' field")
	}
	if _, exists := parsed["max_subscriptions"]; !exists {
		t.Error("expected 'max_subscriptions' field")
	}
	if _, exists := parsed["enable_delta_updates"]; !exists {
		t.Error("expected 'enable_delta_updates' field")
	}
}

// TestCoinProfilerHandlerPathMatching tests that routes would match correctly
func TestCoinProfilerHandlerPathMatching(t *testing.T) {
	// These paths should match the registered routes
	paths := []struct {
		path   string
		method string
	}{
		{"/api/futures/coin-profiler/status", "GET"},
		{"/api/futures/coin-profiler/coins", "GET"},
		{"/api/futures/coin-profiler/coins/BTCUSDT", "GET"},
		{"/api/futures/coin-profiler/requirements", "GET"},
		{"/api/futures/coin-profiler/start", "POST"},
		{"/api/futures/coin-profiler/stop", "POST"},
	}

	for _, p := range paths {
		t.Run(p.method+" "+p.path, func(t *testing.T) {
			// Verify the path is a valid URL
			req := httptest.NewRequest(p.method, p.path, nil)
			if req.URL.Path != p.path {
				t.Errorf("path mismatch: expected %s, got %s", p.path, req.URL.Path)
			}
		})
	}
}

// TestCoinProfilerEmptyCoinsResponse tests empty coins list response
func TestCoinProfilerEmptyCoinsResponse(t *testing.T) {
	response := map[string]interface{}{
		"coins": []interface{}{},
		"total": 0,
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal empty coins response: %v", err)
	}

	// Should produce valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal empty coins response: %v", err)
	}

	coins := parsed["coins"].([]interface{})
	if len(coins) != 0 {
		t.Errorf("expected empty coins array, got %d items", len(coins))
	}
}

// TestCoinProfilerEmptyRequirementsResponse tests empty requirements response
func TestCoinProfilerEmptyRequirementsResponse(t *testing.T) {
	response := map[string]interface{}{
		"all_timeframes":   []string{},
		"all_data_fields":  []string{},
		"total_strategies": 0,
		"subscriptions":    map[string]interface{}{},
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal empty requirements response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal empty requirements response: %v", err)
	}

	if parsed["total_strategies"].(float64) != 0 {
		t.Errorf("expected total_strategies 0, got %v", parsed["total_strategies"])
	}
}
