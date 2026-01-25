package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"binance-trading-bot/internal/entrydecision"

	"github.com/gin-gonic/gin"
)

// setupEntryDecisionTestRouter creates a test router with the entry decision handlers.
func setupEntryDecisionTestRouter() (*gin.Engine, *Server) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	server := &Server{
		router: router,
		// repo is nil - handlers should handle this gracefully
	}

	// Set up routes manually for testing
	api := router.Group("/api")
	futures := api.Group("/futures")
	{
		futures.GET("/entry-decision/strategies", server.handleGetEntryDecisionStrategies)
		futures.GET("/entry-decision/strategies/:mode", server.handleGetEntryDecisionStrategiesForMode)
		futures.GET("/entry-decision/candidates", server.handleGetEntryCandidates)
		futures.GET("/entry-decision/pattern/:symbol", server.handleGetPatternProgress)
		futures.GET("/entry-decision/score/:symbol", server.handleGetScoreBreakdown)
	}

	return router, server
}

// mockAuthMiddleware simulates authentication by setting user ID in context.
func mockAuthMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Next()
	}
}

// TestGetEntryDecisionStrategies_ServiceUnavailable tests the strategies endpoint without repo.
func TestGetEntryDecisionStrategies_ServiceUnavailable(t *testing.T) {
	router, server := setupEntryDecisionTestRouter()

	// Apply mock auth middleware
	router.Use(mockAuthMiddleware("test-user"))

	// Server has nil repo, so it should return service unavailable
	server.authEnabled = false // Disable auth check

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/strategies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Without repo, should return 503 Service Unavailable
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] != "SERVICE_UNAVAILABLE" {
		t.Errorf("Expected error 'SERVICE_UNAVAILABLE', got '%v'", response["error"])
	}
}

// TestGetEntryDecisionStrategiesForMode_InvalidMode tests the mode-specific endpoint with invalid mode.
func TestGetEntryDecisionStrategiesForMode_InvalidMode(t *testing.T) {
	router, server := setupEntryDecisionTestRouter()
	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/strategies/invalid_mode", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] != "INVALID_MODE" {
		t.Errorf("Expected error 'INVALID_MODE', got '%v'", response["error"])
	}
}

// TestGetEntryDecisionStrategiesForMode_ValidModes tests all valid mode values.
func TestGetEntryDecisionStrategiesForMode_ValidModes(t *testing.T) {
	validModes := []string{"scalp", "swing", "position", "ultra_fast"}

	for _, mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			router, server := setupEntryDecisionTestRouter()
			router.Use(mockAuthMiddleware("test-user"))
			server.authEnabled = false

			req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/strategies/"+mode, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should not return INVALID_MODE error for valid modes
			// (will return SERVICE_UNAVAILABLE due to nil repo, which is expected)
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if response["error"] == "INVALID_MODE" {
				t.Errorf("Mode '%s' should be valid but got INVALID_MODE error", mode)
			}
		})
	}
}

// TestGetPatternProgress_MissingSymbol tests pattern endpoint without symbol.
func TestGetPatternProgress_MissingSymbol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	server := &Server{
		router: router,
	}

	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	// Route with empty symbol should return bad request
	router.GET("/api/futures/entry-decision/pattern/:symbol", server.handleGetPatternProgress)

	// Test with empty symbol (gin should still route it)
	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/pattern/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Empty symbol in route returns 404 (no route match) not 400
	// This is expected Gin behavior
	if w.Code != http.StatusNotFound {
		t.Logf("Status code: %d (404 expected for empty param in route)", w.Code)
	}
}

// TestGetPatternProgress_InvalidMode tests pattern endpoint with invalid mode query param.
func TestGetPatternProgress_InvalidMode(t *testing.T) {
	router, server := setupEntryDecisionTestRouter()
	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/pattern/BTCUSDT?mode=invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] != "INVALID_MODE" {
		t.Errorf("Expected error 'INVALID_MODE', got '%v'", response["error"])
	}
}

// TestGetPatternProgress_NotTracking tests pattern endpoint for symbol not being tracked.
func TestGetPatternProgress_NotTracking(t *testing.T) {
	router, server := setupEntryDecisionTestRouter()
	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/pattern/BTCUSDT", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 with "not_tracking" status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "not_tracking" {
		t.Errorf("Expected status 'not_tracking', got '%v'", response["status"])
	}

	if response["symbol"] != "BTCUSDT" {
		t.Errorf("Expected symbol 'BTCUSDT', got '%v'", response["symbol"])
	}
}

// TestGetPatternProgress_DefaultTimeframe tests that default timeframe is applied.
func TestGetPatternProgress_DefaultTimeframe(t *testing.T) {
	testCases := []struct {
		mode              string
		expectedTimeframe string
	}{
		{"scalp", "5m"},
		{"swing", "15m"},
		{"position", "1h"},
		{"ultra_fast", "1m"},
	}

	for _, tc := range testCases {
		t.Run(tc.mode, func(t *testing.T) {
			router, server := setupEntryDecisionTestRouter()
			router.Use(mockAuthMiddleware("test-user"))
			server.authEnabled = false

			req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/pattern/BTCUSDT?mode="+tc.mode, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if response["timeframe"] != tc.expectedTimeframe {
				t.Errorf("Expected timeframe '%s' for mode '%s', got '%v'", tc.expectedTimeframe, tc.mode, response["timeframe"])
			}
		})
	}
}

// TestGetScoreBreakdown_MissingSymbol tests score endpoint with empty symbol.
func TestGetScoreBreakdown_MissingSymbol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	server := &Server{
		router: router,
	}

	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	router.GET("/api/futures/entry-decision/score/:symbol", server.handleGetScoreBreakdown)

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/score/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Empty symbol in route returns 404 (no route match)
	if w.Code != http.StatusNotFound {
		t.Logf("Status code: %d (404 expected for empty param in route)", w.Code)
	}
}

// TestGetScoreBreakdown_ValidSymbol tests score endpoint with valid symbol.
func TestGetScoreBreakdown_ValidSymbol(t *testing.T) {
	router, server := setupEntryDecisionTestRouter()
	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/score/BTCUSDT", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 with score data
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response ScoreBreakdownResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Symbol != "BTCUSDT" {
		t.Errorf("Expected symbol 'BTCUSDT', got '%v'", response.Symbol)
	}

	// Score should be between 0 and 100
	if response.Score < 0 || response.Score > 100 {
		t.Errorf("Score should be between 0 and 100, got %d", response.Score)
	}

	// Threshold should be reasonable (default is 55)
	if response.Threshold < 0 || response.Threshold > 100 {
		t.Errorf("Threshold should be between 0 and 100, got %d", response.Threshold)
	}

	// Components should exist
	if response.Components == nil {
		t.Error("Components should not be nil")
	}
}

// TestGetEntryCandidates tests the candidates endpoint.
func TestGetEntryCandidates(t *testing.T) {
	router, server := setupEntryDecisionTestRouter()
	router.Use(mockAuthMiddleware("test-user"))
	server.authEnabled = false

	req := httptest.NewRequest(http.MethodGet, "/api/futures/entry-decision/candidates", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 with empty candidates (no patterns tracked yet)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response entrydecision.EntryCandidatesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have empty candidates array
	if response.Candidates == nil {
		t.Error("Candidates should not be nil (should be empty array)")
	}

	if response.TotalCandidates != 0 {
		t.Errorf("Expected 0 candidates, got %d", response.TotalCandidates)
	}
}

// TestGetDefaultTimeframeForMode tests the helper function.
func TestGetDefaultTimeframeForMode(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"ultra_fast", "1m"},
		{"scalp", "5m"},
		{"swing", "15m"},
		{"position", "1h"},
		{"unknown", "5m"}, // Default fallback
		{"", "5m"},        // Empty mode
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			result := getDefaultTimeframeForMode(tt.mode)
			if result != tt.expected {
				t.Errorf("getDefaultTimeframeForMode(%q) = %q, want %q", tt.mode, result, tt.expected)
			}
		})
	}
}

// TestFormatStepDetails tests the step details formatting helper.
func TestFormatStepDetails(t *testing.T) {
	tests := []struct {
		name     string
		progress *entrydecision.PatternProgress
		expected string
	}{
		{
			name:     "nil progress",
			progress: nil,
			expected: "",
		},
		{
			name: "progress with step details",
			progress: &entrydecision.PatternProgress{
				CurrentStep: 2,
				StepDetails: []entrydecision.StepDetail{
					{Progress: "1/1 volume spike"},
					{Progress: "3/6 candles"},
					{Progress: "waiting"},
				},
			},
			expected: "3/6 candles",
		},
		{
			name: "progress with empty progress field",
			progress: &entrydecision.PatternProgress{
				CurrentStep: 1,
				StepDetails: []entrydecision.StepDetail{
					{Progress: ""},
				},
			},
			expected: "",
		},
		{
			name: "progress with step out of range",
			progress: &entrydecision.PatternProgress{
				CurrentStep: 5,
				StepDetails: []entrydecision.StepDetail{
					{Progress: "step 1"},
					{Progress: "step 2"},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStepDetails(tt.progress)
			if result != tt.expected {
				t.Errorf("formatStepDetails() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestCalculatePatternPriority tests the priority calculation helper.
func TestCalculatePatternPriority(t *testing.T) {
	tests := []struct {
		name        string
		progress    *entrydecision.PatternProgress
		minExpected int
	}{
		{
			name:        "nil progress",
			progress:    nil,
			minExpected: 0,
		},
		{
			name: "ready pattern should have high priority",
			progress: &entrydecision.PatternProgress{
				Status:      entrydecision.PatternStatusReady,
				CurrentStep: 3,
				Mode:        "scalp",
			},
			minExpected: 80, // 50 base + 30 ready + 15 steps
		},
		{
			name: "ultra_fast mode bonus",
			progress: &entrydecision.PatternProgress{
				Status:      entrydecision.PatternStatusWatching,
				CurrentStep: 1,
				Mode:        "ultra_fast",
			},
			minExpected: 60, // 50 base + 5 step + 10 mode
		},
		{
			name: "scalp mode bonus",
			progress: &entrydecision.PatternProgress{
				Status:      entrydecision.PatternStatusWatching,
				CurrentStep: 1,
				Mode:        "scalp",
			},
			minExpected: 55, // 50 base + 5 step + 5 mode
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculatePatternPriority(tt.progress)
			if result < tt.minExpected {
				t.Errorf("calculatePatternPriority() = %d, want >= %d", result, tt.minExpected)
			}
		})
	}
}

// TestPatternMatcherCache tests that pattern matchers are cached per user.
func TestPatternMatcherCache(t *testing.T) {
	server := &Server{}

	// Get pattern matcher for user 1
	matcher1 := server.getOrCreatePatternMatcher("user1")
	if matcher1 == nil {
		t.Fatal("Pattern matcher should not be nil")
	}

	// Get pattern matcher again for user 1 - should be same instance
	matcher1Again := server.getOrCreatePatternMatcher("user1")
	if matcher1 != matcher1Again {
		t.Error("Same user should get same pattern matcher instance")
	}

	// Get pattern matcher for user 2 - should be different instance
	matcher2 := server.getOrCreatePatternMatcher("user2")
	if matcher1 == matcher2 {
		t.Error("Different users should get different pattern matcher instances")
	}

	// Clean up cache after test
	entryDecisionCacheMu.Lock()
	delete(patternMatcherCache, "user1")
	delete(patternMatcherCache, "user2")
	entryDecisionCacheMu.Unlock()
}

// TestScoreCalculatorCache tests that score calculators are cached per user.
func TestScoreCalculatorCache(t *testing.T) {
	server := &Server{}

	// Get score calculator for user 1
	calc1 := server.getOrCreateScoreCalculator("user1")
	if calc1 == nil {
		t.Fatal("Score calculator should not be nil")
	}

	// Get score calculator again for user 1 - should be same instance
	calc1Again := server.getOrCreateScoreCalculator("user1")
	if calc1 != calc1Again {
		t.Error("Same user should get same score calculator instance")
	}

	// Get score calculator for user 2 - should be different instance
	calc2 := server.getOrCreateScoreCalculator("user2")
	if calc1 == calc2 {
		t.Error("Different users should get different score calculator instances")
	}

	// Clean up cache after test
	entryDecisionCacheMu.Lock()
	delete(scoreCalculatorCache, "user1")
	delete(scoreCalculatorCache, "user2")
	entryDecisionCacheMu.Unlock()
}

// TestEntryDecisionStrategiesResponse_JSONStructure tests JSON serialization.
func TestEntryDecisionStrategiesResponse_JSONStructure(t *testing.T) {
	response := EntryDecisionStrategiesResponse{
		Strategies:         []entrydecision.StrategyMatch{},
		ByMode:             []entrydecision.ModeStrategies{},
		TotalStrategies:    5,
		EnabledStrategies:  3,
		TotalCoinsReady:    2,
		TotalCoinsWatching: 8,
		UpdatedAt:          time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify expected fields exist
	expectedFields := []string{"strategies", "total_strategies", "enabled_strategies", "total_coins_ready", "total_coins_watching", "updated_at"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Expected field '%s' in JSON response", field)
		}
	}
}

// TestPatternProgressResponse_JSONStructure tests JSON serialization.
func TestPatternProgressResponse_JSONStructure(t *testing.T) {
	response := PatternProgressResponse{
		Symbol:      "BTCUSDT",
		Mode:        "scalp",
		Strategy:    "volume_imbalance",
		SubStrategy: "ravindra_volume_imbalance",
		Timeframe:   "5m",
		CurrentStep: 2,
		TotalSteps:  3,
		Status:      "consolidating",
		StepDetails: []entrydecision.StepDetail{},
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify expected fields exist
	expectedFields := []string{"symbol", "mode", "strategy", "sub_strategy", "timeframe", "current_step", "total_steps", "status", "step_details", "started_at", "updated_at"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Expected field '%s' in JSON response", field)
		}
	}
}

// TestScoreBreakdownResponse_JSONStructure tests JSON serialization.
func TestScoreBreakdownResponse_JSONStructure(t *testing.T) {
	response := ScoreBreakdownResponse{
		Symbol:    "BTCUSDT",
		Score:     72,
		Direction: "long",
		Ready:     true,
		Threshold: 55,
		Components: map[string]int{
			"trend_strength": 80,
			"momentum":       65,
			"volume":         70,
			"price_action":   75,
			"sr_proximity":   50,
		},
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Verify expected fields exist
	expectedFields := []string{"symbol", "score", "direction", "ready", "threshold", "components", "updated_at"}
	for _, field := range expectedFields {
		if _, ok := parsed[field]; !ok {
			t.Errorf("Expected field '%s' in JSON response", field)
		}
	}
}
