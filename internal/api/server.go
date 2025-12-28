package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"binance-trading-bot/internal/apikeys"
	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/billing"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/license"
	"binance-trading-bot/internal/vault"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// RateLimiter provides simple in-memory rate limiting per endpoint
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.Mutex
	limit    int           // max requests
	window   time.Duration // time window
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request is allowed for the given key
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-r.window)

	// Filter out old requests
	var recent []time.Time
	for _, t := range r.requests[key] {
		if t.After(windowStart) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= r.limit {
		r.requests[key] = recent
		return false
	}

	r.requests[key] = append(recent, now)
	return true
}

// Server represents the HTTP API server
type Server struct {
	router         *gin.Engine
	httpServer     *http.Server
	repo           *database.Repository
	eventBus       *events.EventBus
	botAPI         BotAPI
	config         ServerConfig
	authService    *auth.Service
	authEnabled    bool
	vaultClient    *vault.Client
	billingService *billing.StripeService
	licenseInfo    *license.LicenseInfo
	rateLimiter    *RateLimiter        // API rate limiter to prevent Binance bans
	apiKeyService  *apikeys.Service    // Service to get user-specific API keys

	// Multi-user autopilot manager (per-user autopilot instances)
	userAutopilotManager *autopilot.UserAutopilotManager
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port            int
	Host            string
	ProductionMode  bool
	StaticFilesPath string
}

// BotAPI interface defines methods the bot must expose to the API
type BotAPI interface {
	GetStatus() map[string]interface{}
	GetOpenPositions() []map[string]interface{}
	GetStrategies() []map[string]interface{}
	PlaceOrder(symbol, side, orderType string, quantity, price float64) (int64, error)
	CancelOrder(orderID int64) error
	ClosePosition(symbol string) error
	ToggleStrategy(name string, enabled bool) error
	GetBinanceClient() interface{}
	GetClient() interface{} // Returns *binance.Client for backtest
	ExecutePendingSignal(signal *database.PendingSignal) error
	GetScanner() interface{} // Returns *scanner.Scanner
}

// NewServer creates a new API server
func NewServer(
	config ServerConfig,
	repo *database.Repository,
	eventBus *events.EventBus,
	botAPI BotAPI,
	authService *auth.Service, // Can be nil if auth is disabled
	vaultClient *vault.Client, // Can be nil if vault is disabled
	billingService *billing.StripeService, // Can be nil if billing is disabled
	licenseInfo *license.LicenseInfo, // Can be nil for trial mode
) *Server {
	// Set Gin mode
	if config.ProductionMode {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS middleware
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:5173", "http://localhost:8088", "http://localhost:8090"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsConfig.ExposeHeaders = []string{"Content-Length"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

	server := &Server{
		router:         router,
		repo:           repo,
		eventBus:       eventBus,
		botAPI:         botAPI,
		config:         config,
		authService:    authService,
		authEnabled:    authService != nil,
		vaultClient:    vaultClient,
		billingService: billingService,
		licenseInfo:    licenseInfo,
		rateLimiter:    NewRateLimiter(120, time.Minute), // 120 requests per minute per endpoint (Binance allows 1200/min)
		apiKeyService:  apikeys.NewService(repo),         // Service for user-specific API keys
	}

	server.setupRoutes()

	// Initialize user-aware WebSocket hub for real-time event broadcasting
	InitUserWebSocket(eventBus)

	return server
}

// rateLimitMiddleware creates a middleware that rate limits requests by endpoint
func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	// Endpoints that don't call Binance API - no rate limiting needed
	noRateLimitPaths := map[string]bool{
		// Ginie endpoints (internal state only)
		"/api/futures/ginie/status":                    true,
		"/api/futures/ginie/config":                    true,
		"/api/futures/ginie/autopilot/status":          true,
		"/api/futures/ginie/autopilot/config":          true,
		"/api/futures/ginie/autopilot/positions":       true,
		"/api/futures/ginie/autopilot/history":         true,
		"/api/futures/ginie/protection/status":         true,
		"/api/futures/ginie/trade-history":             true,
		"/api/futures/ginie/performance-metrics":       true,
		"/api/futures/ginie/llm-diagnostics":           true,
		"/api/futures/ginie/circuit-breaker/status":    true,
		"/api/futures/ginie/decisions":                 true,
		"/api/futures/ginie/blocked-coins":             true,
		"/api/futures/ginie/risk-level":                true,
		"/api/futures/ginie/rate-limiter/status":       true,
		// LLM & Adaptive AI endpoints (internal state only - Story 2.8)
		"/api/futures/ginie/llm-config":                true,
		"/api/futures/ginie/adaptive-recommendations":  true,
		"/api/futures/ginie/llm-diagnostics-v2":        true,
		"/api/futures/ginie/trade-history-ai":          true,
		// Autopilot status endpoints (internal state)
		"/api/futures/autopilot/status":                true,
		"/api/futures/autopilot/circuit-breaker/status": true,
		"/api/futures/autopilot/recent-decisions":      true,
		"/api/futures/autopilot/investigate":           true,
		"/api/futures/autopilot/averaging/status":      true,
		"/api/futures/autopilot/dynamic-sltp":          true,
		"/api/futures/autopilot/scalping":              true,
		"/api/futures/autopilot/coin-preferences":      true,
		"/api/futures/autopilot/trading-style":         true,
		// Hedging status (internal state)
		"/api/futures/autopilot/hedging/status":        true,
		"/api/futures/autopilot/hedging/config":        true,
		"/api/futures/autopilot/hedging/history":       true,
		// Adaptive engine (internal state)
		"/api/futures/autopilot/adaptive-engine/status": true,
		// Trade history from DB (not Binance)
		"/api/futures/trades/history":                  true,
		"/api/futures/metrics":                         true,
		"/api/futures/trade-source-stats":              true,
		// Spot autopilot endpoints (internal state)
		"/api/spot/autopilot/status":                   true,
		"/api/spot/autopilot/profit-stats":             true,
		"/api/spot/circuit-breaker/status":             true,
		"/api/spot/coin-preferences":                   true,
		"/api/spot/ai-decisions":                       true,
		"/api/spot/ai-decisions/stats":                 true,
		"/api/spot/positions":                          true,
	}

	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Skip rate limiting for internal endpoints
		if noRateLimitPaths[path] {
			c.Next()
			return
		}

		if !s.rateLimiter.Allow(path) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests to this endpoint. Please slow down to avoid Binance API bans.",
				"path":    path,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", s.handleHealth)

	// Auth routes (public, no authentication required)
	if s.authEnabled {
		authHandlers := auth.NewHandlers(s.authService)
		authGroup := s.router.Group("/api/auth")
		authHandlers.RegisterRoutes(authGroup, s.authService.GetJWTManager())
	}

	// Auth status endpoint (always available, returns whether auth is enabled)
	s.router.GET("/api/auth/status", func(c *gin.Context) {
		subscriptionEnabled := os.Getenv("SUBSCRIPTION_ENABLED")
		isSubscriptionEnabled := subscriptionEnabled != "" && strings.ToLower(subscriptionEnabled) == "true"

		c.JSON(200, gin.H{
			"auth_enabled":         s.authEnabled,
			"subscription_enabled": isSubscriptionEnabled,
		})
	})

	// Public API endpoints (no auth required)
	s.router.GET("/api/health/status", s.handleGetAPIHealthStatus)

	// API routes (protected when auth is enabled)
	api := s.router.Group("/api")

	// Apply auth middleware if enabled
	if s.authEnabled {
		// Required auth middleware - all API routes require authentication
		api.Use(auth.Middleware(s.authService.GetJWTManager()))
	}

	{
		// Bot endpoints
		api.GET("/bot/status", s.handleBotStatus)
		api.GET("/bot/config", s.handleBotConfig)

		// Position endpoints
		api.GET("/positions", s.handleGetPositions)
		api.GET("/positions/history", s.handleGetPositionHistory)
		api.POST("/positions/:symbol/close", s.handleClosePosition)
		api.POST("/positions/close-all", s.handleCloseAllPositions)

		// Order endpoints
		api.GET("/orders", s.handleGetActiveOrders)
		api.GET("/orders/history", s.handleGetOrderHistory)
		api.POST("/orders", s.handlePlaceOrder)
		api.DELETE("/orders/:id", s.handleCancelOrder)

		// Strategy endpoints
		api.GET("/strategies", s.handleGetStrategies)
		api.PUT("/strategies/:name/toggle", s.handleToggleStrategy)

		// Strategy config endpoints
		api.GET("/strategy-configs", s.handleGetStrategyConfigs)
		api.POST("/strategy-configs", s.handleCreateStrategyConfig)
		api.GET("/strategy-configs/:id", s.handleGetStrategyConfig)
		api.PUT("/strategy-configs/:id", s.handleUpdateStrategyConfig)
		api.DELETE("/strategy-configs/:id", s.handleDeleteStrategyConfig)

		// Visual strategy & backtest endpoints
		api.GET("/binance/klines", s.handleGetKlines)
		api.POST("/strategy-configs/:id/backtest", s.handleRunBacktest)
		api.GET("/strategy-configs/:id/backtest-results", s.handleGetBacktestResults)
		api.GET("/backtest-results/:id/trades", s.handleGetBacktestTrades)

		// Signal endpoints
		api.GET("/signals", s.handleGetSignals)

		// Pending signal endpoints
		api.GET("/pending-signals", s.handleGetPendingSignals)
		api.GET("/pending-signals/:id", s.handleGetPendingSignal)
		api.POST("/pending-signals/:id/confirm", s.handleConfirmPendingSignal)
		api.POST("/pending-signals/:id/archive", s.handleArchivePendingSignal)
		api.DELETE("/pending-signals/:id", s.handleDeletePendingSignal)
		api.POST("/pending-signals/:id/duplicate", s.handleDuplicatePendingSignal)

		// Screener endpoints
		api.GET("/screener/results", s.handleGetScreenerResults)

		// Binance data endpoints
		api.GET("/binance/symbols", s.handleGetBinanceSymbols)
		api.GET("/binance/all-symbols", s.handleGetAllSymbols)

		// Pattern scanner endpoints
		api.POST("/pattern-scanner/scan", s.handleScanPatterns)

		// Strategy scanner endpoints
		api.GET("/strategy-scanner/scan", s.handleGetScanResults)
		api.POST("/strategy-scanner/refresh", s.handleRefreshScan)

		// Watchlist endpoints
		api.GET("/watchlist", s.handleGetWatchlist)
		api.POST("/watchlist", s.handleAddToWatchlist)
		api.DELETE("/watchlist/:symbol", s.handleRemoveFromWatchlist)

		// Metrics endpoints
		api.GET("/metrics", s.handleGetMetrics)

		// System events
		api.GET("/events", s.handleGetSystemEvents)

		// AI Signals endpoints
		api.GET("/ai-decisions", s.handleGetAIDecisions)
		api.GET("/ai-decisions/stats", s.handleGetAIDecisionStats)
		api.GET("/ai-decisions/:id", s.handleGetAIDecisionByID)

		// Strategy Performance endpoints
		api.GET("/strategy-performance", s.handleGetStrategyPerformance)
		api.GET("/strategy-performance/overall", s.handleGetOverallPerformance)
		api.GET("/strategy-performance/historical", s.handleGetHistoricalSuccessRate)

		// License endpoints
		api.GET("/license", s.handleGetLicenseInfo)
		api.GET("/license/feature/:feature", s.handleCheckFeature)

		// Settings & Control endpoints
		settings := api.Group("/settings")
		{
			settings.GET("/trading-mode", s.handleGetTradingMode)
			settings.POST("/trading-mode", s.handleSetTradingMode)
			settings.GET("/wallet-balance", s.handleGetWalletBalance)
			settings.GET("/autopilot", s.handleGetAutopilotStatus)
			settings.POST("/autopilot/toggle", s.handleToggleAutopilot)
			settings.POST("/autopilot/rules", s.handleSetAutopilotRules)
			settings.GET("/circuit-breaker", s.handleGetCircuitBreakerStatus)
			settings.POST("/circuit-breaker/reset", s.handleResetCircuitBreaker)
			settings.POST("/circuit-breaker/config", s.handleUpdateCircuitBreakerConfig)
		}

		// User profile and API keys endpoints (requires auth)
		user := api.Group("/user")
		{
			user.PUT("/profile", s.handleUpdateProfile)
			user.POST("/change-password", s.handleChangePassword)
			user.GET("/api-keys", s.handleGetAPIKeys)
			user.POST("/api-keys", s.handleAddAPIKey)
			user.DELETE("/api-keys/:id", s.handleDeleteAPIKey)
			user.POST("/api-keys/:id/test", s.handleTestAPIKey)

			// AI API Keys
			user.GET("/ai-keys", s.handleGetAIKeys)
			user.POST("/ai-keys", s.handleAddAIKey)
			user.DELETE("/ai-keys/:id", s.handleDeleteAIKey)
			user.POST("/ai-keys/:id/test", s.handleTestAIKey)

			// User utilities
			user.GET("/ip-address", s.handleGetUserIPAddress)
			user.GET("/api-status", s.handleGetUserAPIStatus)
		}

		// Billing endpoints (requires auth)
		billing := api.Group("/billing")
		{
			billing.GET("/profit-history", s.handleGetProfitHistory)
			billing.GET("/invoices", s.handleGetInvoices)
			billing.POST("/checkout", s.handleCreateCheckoutSession)
			billing.POST("/portal", s.handleCreateCustomerPortal)
		}

		// Futures trading endpoints (rate limited to prevent Binance API bans)
		futures := api.Group("/futures")
		futures.Use(s.rateLimitMiddleware()) // Apply rate limiting to all futures endpoints
		{
			// Account endpoints
			futures.GET("/account", s.handleGetFuturesAccountInfo)
			futures.GET("/wallet-balance", s.handleGetFuturesWalletBalance)
			futures.GET("/positions", s.handleGetFuturesPositions)
			futures.POST("/positions/close-all", s.handleCloseAllFuturesPositions) // Panic button - must be before :symbol route
			futures.POST("/positions/:symbol/close", s.handleCloseFuturesPosition)
			futures.GET("/positions/:symbol/orders", s.handleGetPositionOrders)   // Get TP/SL orders for position
			futures.POST("/positions/:symbol/tpsl", s.handleSetPositionTPSL)       // Set TP/SL for position

			// Settings endpoints
			futures.POST("/leverage", s.handleSetLeverage)
			futures.POST("/margin-type", s.handleSetMarginType)
			futures.POST("/position-mode", s.handleSetPositionMode)
			futures.GET("/position-mode", s.handleGetPositionMode)
			futures.GET("/settings/:symbol", s.handleGetFuturesAccountSettings)

			// Order endpoints
			futures.POST("/orders", s.handlePlaceFuturesOrder)
			futures.DELETE("/orders/:symbol/:id", s.handleCancelFuturesOrder)
			futures.DELETE("/orders/:symbol/all", s.handleCancelAllFuturesOrders)
			futures.GET("/orders/open", s.handleGetFuturesOpenOrders)
			futures.GET("/orders/all", s.handleGetAllFuturesOrders)

			// Algo Order endpoints (TP/SL orders since 2025-12-09)
			futures.DELETE("/algo-orders/:symbol/:id", s.handleCancelAlgoOrder)
			futures.DELETE("/algo-orders/:symbol/all", s.handleCancelAllAlgoOrders)

			// Market data endpoints
			futures.GET("/funding-rate/:symbol", s.handleGetFundingRate)
			futures.GET("/orderbook/:symbol", s.handleGetOrderBookDepth)
			futures.GET("/mark-price/:symbol", s.handleGetMarkPrice)
			futures.GET("/symbols", s.handleGetFuturesSymbols)
			futures.GET("/klines", s.handleGetFuturesKlines)

			// History endpoints
			futures.GET("/trades/history", s.handleGetFuturesTradeHistory)
			futures.GET("/account/trades", s.handleGetFuturesAccountTrades) // Direct from Binance API
			futures.GET("/funding-fees/history", s.handleGetFundingFeeHistory)
			futures.GET("/transactions/history", s.handleGetFuturesTransactionHistory)
			futures.GET("/metrics", s.handleGetFuturesMetrics)
			futures.GET("/trade-source-stats", s.handleGetTradeSourceStats)
			futures.GET("/position-trade-sources", s.handleGetPositionTradeSources)

			// Trade lifecycle events endpoints
			futures.GET("/trades/:tradeId/events", s.handleGetTradeLifecycleEvents)
			futures.GET("/trades/:tradeId/events/:eventType", s.handleGetTradeLifecycleEventsByType)
			futures.GET("/trades/:tradeId/lifecycle-summary", s.handleGetTradeLifecycleSummary)
			futures.GET("/trades/:tradeId/sl-revisions", s.handleGetTradeSLRevisionCount)
			futures.GET("/trade-events/recent", s.handleGetRecentTradeLifecycleEvents)

			// Autopilot endpoints
			futures.GET("/autopilot/status", s.handleGetFuturesAutopilotStatus)
			futures.POST("/autopilot/toggle", s.handleToggleFuturesAutopilot)
			futures.POST("/autopilot/dry-run", s.handleSetFuturesAutopilotDryRun)
			futures.POST("/autopilot/risk-level", s.handleSetFuturesAutopilotRiskLevel)
			futures.POST("/autopilot/allocation", s.handleSetFuturesAutopilotAllocation)
			futures.POST("/autopilot/profit-reinvest", s.handleSetFuturesAutopilotProfitReinvest)
			futures.GET("/autopilot/profit-stats", s.handleGetFuturesAutopilotProfitStats)
			futures.POST("/autopilot/reset-allocation", s.handleResetFuturesAutopilotAllocation)
			futures.POST("/autopilot/tpsl", s.handleSetFuturesAutopilotTPSL)
			futures.POST("/autopilot/leverage", s.handleSetFuturesAutopilotLeverage)
			futures.POST("/autopilot/min-confidence", s.handleSetFuturesAutopilotMinConfidence)
			futures.POST("/autopilot/confluence", s.handleSetFuturesAutopilotConfluence)
			futures.POST("/autopilot/max-position-size", s.handleSetFuturesAutopilotMaxPositionSize)

			// Circuit breaker endpoints for futures loss control
			futures.GET("/autopilot/circuit-breaker/status", s.handleGetFuturesCircuitBreakerStatus)
			futures.POST("/autopilot/circuit-breaker/reset", s.handleResetFuturesCircuitBreaker)
			futures.POST("/autopilot/circuit-breaker/config", s.handleUpdateFuturesCircuitBreakerConfig)
			futures.POST("/autopilot/circuit-breaker/toggle", s.handleToggleFuturesCircuitBreaker)

			// Recent decisions endpoint for UI
			futures.GET("/autopilot/recent-decisions", s.handleGetFuturesAutopilotRecentDecisions)

			// Sentiment & News endpoints
			futures.GET("/sentiment/news", s.handleGetSentimentNews)
			futures.GET("/sentiment/breaking", s.handleGetBreakingNews)

			// Position averaging endpoints
			futures.GET("/autopilot/averaging/status", s.handleGetAveragingStatus)
			futures.POST("/autopilot/averaging/config", s.handleSetAveragingConfig)

			// Dynamic SL/TP endpoints (volatility-based per coin)
			futures.GET("/autopilot/dynamic-sltp", s.handleGetDynamicSLTPConfig)
			futures.POST("/autopilot/dynamic-sltp", s.handleSetDynamicSLTPConfig)

			// Scalping mode endpoints
			futures.GET("/autopilot/scalping", s.handleGetScalpingConfig)
			futures.POST("/autopilot/scalping", s.handleSetScalpingConfig)

			// Investigate/diagnostics endpoints
			futures.GET("/autopilot/investigate", s.handleGetInvestigateStatus)
			futures.POST("/autopilot/clear-cooldown", s.handleClearFlipFlopCooldown)
			futures.POST("/autopilot/force-sync", s.handleForceSyncPositions)
			futures.POST("/autopilot/recalculate-allocation", s.handleRecalculateAllocation)

			// Coin classification endpoints
			futures.GET("/autopilot/coin-classifications", s.handleGetCoinClassifications)
			futures.GET("/autopilot/coin-classifications/summary", s.handleGetCoinClassificationSummary)
			futures.POST("/autopilot/coin-classifications/refresh", s.handleRefreshCoinClassifications)
			futures.POST("/autopilot/coin-preference", s.handleUpdateCoinPreference)
			futures.POST("/autopilot/coin-preferences/bulk", s.handleBulkUpdateCoinPreferences)
			futures.GET("/autopilot/coin-preferences", s.handleGetCoinPreferences)
			futures.GET("/autopilot/coins/eligible", s.handleGetEligibleCoins)
			futures.POST("/autopilot/coins/enable-all", s.handleEnableAllCoins)
			futures.POST("/autopilot/coins/disable-all", s.handleDisableAllCoins)
			futures.POST("/autopilot/category-allocation", s.handleUpdateCategoryAllocation)

			// Trading style endpoints
			futures.GET("/autopilot/trading-style", s.handleGetTradingStyle)
			futures.POST("/autopilot/trading-style", s.handleSetTradingStyle)

			// Hedging endpoints
			futures.GET("/autopilot/hedging/status", s.handleGetHedgingStatus)
			futures.GET("/autopilot/hedging/config", s.handleGetHedgingConfig)
			futures.POST("/autopilot/hedging/config", s.handleUpdateHedgingConfig)
			futures.POST("/autopilot/hedging/manual", s.handleExecuteManualHedge)
			futures.POST("/autopilot/hedging/close", s.handleCloseHedge)
			futures.POST("/autopilot/hedging/enable-mode", s.handleEnableHedgeMode)
			futures.POST("/autopilot/hedging/clear-all", s.handleClearAllHedges)
			futures.GET("/autopilot/hedging/history", s.handleGetHedgeHistory)

			// Adaptive engine (human-like AI decision making)
			futures.GET("/autopilot/adaptive-engine/status", s.handleGetAdaptiveEngineStatus)

			// Auto Mode endpoints (LLM-driven trading decisions)
			futures.GET("/autopilot/auto-mode", s.handleGetAutoModeConfig)
			futures.POST("/autopilot/auto-mode", s.handleSetAutoModeConfig)
			futures.POST("/autopilot/auto-mode/toggle", s.handleToggleAutoMode)

			// Ginie AI Trader endpoints (advanced multi-mode trading)
			futures.GET("/ginie/status", s.handleGetGinieStatus)
			futures.GET("/ginie/config", s.handleGetGinieConfig)
			futures.POST("/ginie/config", s.handleUpdateGinieConfig)
			futures.POST("/ginie/toggle", s.handleToggleGinie)
			futures.GET("/ginie/scan", s.handleGinieScanCoin)
			futures.GET("/ginie/decision", s.handleGinieGenerateDecision)
			futures.GET("/ginie/decisions", s.handleGinieGetDecisions)
			futures.POST("/ginie/scan-all", s.handleGinieScanAll)
			futures.POST("/ginie/analyze-all", s.handleGinieAnalyzeAll)

			// Ginie Autopilot endpoints (autonomous multi-mode trading)
			futures.GET("/ginie/autopilot/status", s.handleGetGinieAutopilotStatus)
			futures.GET("/ginie/autopilot/config", s.handleGetGinieAutopilotConfig)
			futures.POST("/ginie/autopilot/config", s.handleUpdateGinieAutopilotConfig)
			futures.POST("/ginie/autopilot/start", s.handleStartGinieAutopilot)
			futures.POST("/ginie/autopilot/stop", s.handleStopGinieAutopilot)
			futures.GET("/ginie/autopilot/positions", s.handleGetGinieAutopilotPositions)
			futures.GET("/ginie/autopilot/history", s.handleGetGinieAutopilotTradeHistory)
			futures.POST("/ginie/autopilot/clear", s.handleClearGinieAutopilotPositions)
			futures.POST("/ginie/refresh-symbols", s.handleRefreshGinieSymbols)

			// Bulletproof Protection Status (SL/TP health monitoring)
			futures.GET("/ginie/protection/status", s.handleGetProtectionStatus)

			// Per-symbol performance settings endpoints
			futures.GET("/autopilot/symbols", s.handleGetSymbolSettings)
			futures.GET("/autopilot/symbols/report", s.handleGetSymbolPerformanceReport)
			futures.GET("/autopilot/symbols/category/:category", s.handleGetSymbolsByCategory)
			futures.GET("/autopilot/symbols/:symbol", s.handleGetSingleSymbolSettings)
			futures.PUT("/autopilot/symbols/:symbol", s.handleUpdateSymbolSettings)
			futures.POST("/autopilot/symbols/:symbol/blacklist", s.handleBlacklistSymbol)
			futures.DELETE("/autopilot/symbols/:symbol/blacklist", s.handleUnblacklistSymbol)
			futures.POST("/autopilot/category-config", s.handleUpdateCategorySettings)

			// Ginie Circuit Breaker endpoints (separate from FuturesController)
			futures.GET("/ginie/circuit-breaker/status", s.handleGetGinieCircuitBreakerStatus)
			futures.POST("/ginie/circuit-breaker/reset", s.handleResetGinieCircuitBreaker)
			futures.POST("/ginie/circuit-breaker/toggle", s.handleToggleGinieCircuitBreaker)
			futures.POST("/ginie/circuit-breaker/config", s.handleUpdateGinieCircuitBreakerConfig)

			// Ginie Per-Position ROI Target (custom early profit booking ROI%)
			// NOTE: This must come AFTER specific routes like /close-all, /sync, /recalc-sltp
			// because Gin matches routes in order and :symbol is a catch-all parameter

			// Ginie Position Sync (sync with exchange)
			futures.POST("/ginie/positions/sync", s.handleSyncGiniePositions)

			// Ginie Panic Button (closes only Ginie positions)
			futures.POST("/ginie/positions/close-all", s.handleCloseAllGiniePositions)

			// Ginie Adaptive SL/TP (recalculate for naked positions)
			futures.POST("/ginie/positions/recalc-sltp", s.handleRecalculateAdaptiveSLTP)
			futures.GET("/ginie/positions/recalc-sltp/status/:job_id", s.handleGetSLTPJobStatus)
			futures.GET("/ginie/positions/recalc-sltp/jobs", s.handleListSLTPJobs)

			// Per-Position ROI Target (MUST be registered LAST due to :symbol parameter)
			futures.POST("/ginie/positions/:symbol/roi-target", s.handleSetPositionROITarget)

			// Ginie Risk Level endpoints
			futures.GET("/ginie/risk-level", s.handleGetGinieRiskLevel)
			futures.POST("/ginie/risk-level", s.handleSetGinieRiskLevel)

			// Ginie Market Movers endpoints (dynamic symbol selection)
			futures.GET("/ginie/market-movers", s.handleGetMarketMovers)
			futures.POST("/ginie/symbols/refresh-dynamic", s.handleRefreshDynamicSymbols)

			// Ginie Blocked Coins endpoints (per-coin circuit breaker)
			futures.GET("/ginie/blocked-coins", s.handleGetGinieBlockedCoins)
			futures.POST("/ginie/blocked-coins/:symbol/unblock", s.handleUnblockGinieCoin)
			futures.POST("/ginie/blocked-coins/:symbol/reset-history", s.handleResetGinieCoinBlockHistory)

			// Ginie LLM SL Validation endpoints (kill switch after 3 bad calls)
			futures.GET("/ginie/llm-sl/status", s.handleGetGinieLLMSLStatus)
			futures.POST("/ginie/llm-sl/reset/:symbol", s.handleResetGinieLLMSL)

			// Ginie Signal Logs endpoints (all signals with executed/rejected status)
			futures.GET("/ginie/signals", s.handleGetGinieSignalLogs)
			futures.GET("/ginie/signals/stats", s.handleGetGinieSignalStats)

			// Ginie SL Update History endpoints
			futures.GET("/ginie/sl-history", s.handleGetGinieSLHistory)
			futures.GET("/ginie/sl-history/stats", s.handleGetGinieSLStats)

			// Ginie Diagnostics endpoint
			futures.GET("/ginie/diagnostics", s.handleGetGinieDiagnostics)

			// Ginie Rate Limiter status endpoint
			futures.GET("/ginie/rate-limiter/status", s.handleGetRateLimiterStatus)

			// Ginie Trend Timeframes endpoints (multi-timeframe divergence detection)
			futures.GET("/ginie/trend-timeframes", s.handleGetGinieTrendTimeframes)
			futures.POST("/ginie/trend-timeframes", s.handleUpdateGinieTrendTimeframes)

			// SL/TP configuration endpoints
			futures.GET("/ginie/sltp-config", s.handleGetGinieSLTPConfig)
			futures.POST("/ginie/sltp/:mode", s.handleUpdateGinieSLTP)  // :mode = scalp/swing/position
			futures.POST("/ginie/tp-mode", s.handleUpdateGinieTPMode)

			// Ultrafast scalping mode configuration
			futures.GET("/ultrafast/config", s.handleGetUltraFastConfig)
			futures.POST("/ultrafast/config", s.handleUpdateUltraFastConfig)
			futures.POST("/ultrafast/toggle", s.handleToggleUltraFast)
			futures.POST("/ultrafast/reset-stats", s.handleResetUltraFastStats)

			// Enhanced Trade History and Performance Metrics (with date filtering)
			futures.GET("/ginie/trade-history", s.handleGetGinieTradeHistoryWithDateRange)
			futures.GET("/ginie/performance-metrics", s.handleGetGiniePerformanceMetrics)

			// LLM Diagnostics endpoints (track LLM coin enable/disable events)
			futures.GET("/ginie/llm-diagnostics", s.handleGetGinieLLMDiagnostics)
			futures.POST("/ginie/llm-diagnostics/reset", s.handleResetGinieLLMDiagnostics)

			// Strategy Performance endpoints (AI vs Strategy comparison)
			futures.GET("/ginie/strategy-performance", s.handleGetStrategyPerformance)
			futures.GET("/ginie/source-performance", s.handleGetSourcePerformance)
			futures.GET("/ginie/positions/filter", s.handleGetPositionsBySource)
			futures.GET("/ginie/history/filter", s.handleGetTradeHistoryBySource)

			// Mode Configuration CRUD endpoints (Story 2.7 Task 2.7.10)
			futures.GET("/ginie/mode-configs", s.handleGetModeConfigs)
			futures.GET("/ginie/mode-config/:mode", s.handleGetModeConfig)
			futures.PUT("/ginie/mode-config/:mode", s.handleUpdateModeConfig)
			futures.POST("/ginie/mode-config/reset", s.handleResetModeConfigs)
			futures.GET("/ginie/mode-circuit-breaker-status", s.handleGetModeCircuitBreakerStatus)

			// Mode Allocation endpoints (per-mode capital management)
			futures.GET("/modes/allocations", s.handleGetModeAllocations)
			futures.POST("/modes/allocations", s.handleUpdateModeAllocations)
			futures.GET("/modes/allocations/history", s.handleGetModeAllocationHistory)
			futures.GET("/modes/allocations/:mode", s.handleGetModeAllocationStatus)

			// Mode Safety endpoints (per-mode safety controls)
			futures.GET("/modes/safety", s.handleGetModeSafetyStatus)
			futures.POST("/modes/safety/:mode/resume", s.handleResumeMode)
			futures.GET("/modes/safety/history", s.handleGetModeSafetyHistory)
			futures.GET("/modes/safety/:mode/history", s.handleGetModeSafetyEventHistory)

			// Mode Performance endpoints (per-mode performance metrics)
			futures.GET("/modes/performance", s.handleGetModePerformance)
			futures.GET("/modes/performance/:mode", s.handleGetModePerformanceSingle)

			// LLM & Adaptive AI endpoints (Story 2.8)
			futures.GET("/ginie/llm-config", s.handleGetLLMConfig)
			futures.PUT("/ginie/llm-config", s.handleUpdateLLMConfig)
			futures.PUT("/ginie/llm-config/:mode", s.handleUpdateModeLLMSettings)
			futures.GET("/ginie/adaptive-recommendations", s.handleGetAdaptiveRecommendations)
			futures.POST("/ginie/adaptive-recommendations/:id/apply", s.handleApplyRecommendation)
			futures.POST("/ginie/adaptive-recommendations/:id/dismiss", s.handleDismissRecommendation)
			futures.POST("/ginie/adaptive-recommendations/apply-all", s.handleApplyAllRecommendations)
			futures.GET("/ginie/llm-diagnostics-v2", s.handleGetLLMDiagnosticsV2)
			futures.POST("/ginie/llm-diagnostics-v2/reset", s.handleResetLLMDiagnosticsV2)
			futures.GET("/ginie/trade-history-ai", s.handleGetTradeHistoryWithAI)

			// Scan Source Configuration (per-user coin source settings)
			futures.GET("/ginie/scan-config", s.handleGetScanSourceConfig)
			futures.POST("/ginie/scan-config", s.handleUpdateScanSourceConfig)
			futures.GET("/ginie/saved-coins", s.handleGetSavedCoins)
			futures.POST("/ginie/saved-coins", s.handleUpdateSavedCoins)
			futures.GET("/ginie/scan-preview", s.handleGetScanPreview)
		}

		// ==================== SPOT AUTOPILOT ENDPOINTS ====================
		// Separate AI trading system for Spot market
		spot := api.Group("/spot")
		spot.Use(s.rateLimitMiddleware()) // Apply rate limiting
		{
			// Autopilot status & control
			spot.GET("/autopilot/status", s.handleGetSpotAutopilotStatus)
			spot.POST("/autopilot/toggle", s.handleToggleSpotAutopilot)
			spot.POST("/autopilot/dry-run", s.handleSetSpotAutopilotDryRun)
			spot.POST("/autopilot/risk-level", s.handleSetSpotAutopilotRiskLevel)
			spot.POST("/autopilot/allocation", s.handleSetSpotAutopilotAllocation)
			spot.POST("/autopilot/max-positions", s.handleSetSpotAutopilotMaxPositions)
			spot.POST("/autopilot/tpsl", s.handleSetSpotAutopilotTPSL)
			spot.POST("/autopilot/min-confidence", s.handleSetSpotAutopilotMinConfidence)
			spot.GET("/autopilot/profit-stats", s.handleGetSpotAutopilotProfitStats)

			// Circuit breaker
			spot.GET("/circuit-breaker/status", s.handleGetSpotCircuitBreakerStatus)
			spot.POST("/circuit-breaker/reset", s.handleResetSpotCircuitBreaker)
			spot.POST("/circuit-breaker/config", s.handleUpdateSpotCircuitBreakerConfig)
			spot.POST("/circuit-breaker/toggle", s.handleToggleSpotCircuitBreaker)

			// Coin preferences
			spot.GET("/coin-preferences", s.handleGetSpotCoinPreferences)
			spot.POST("/coin-preferences", s.handleSetSpotCoinPreferences)

			// AI decisions
			spot.GET("/ai-decisions", s.handleGetSpotAutopilotRecentDecisions)
			spot.GET("/ai-decisions/stats", s.handleGetSpotDecisionStats)

			// Positions
			spot.GET("/positions", s.handleGetSpotPositions)
			spot.POST("/positions/:symbol/close", s.handleCloseSpotPosition)
			spot.POST("/positions/close-all", s.handleCloseAllSpotPositions)
		}
	}

	// Admin endpoints (requires admin role)
	admin := api.Group("/admin")
	admin.Use(s.adminMiddleware())
	{
		// User management
		admin.GET("/users", s.handleAdminListUsers)

		// License management
		admin.POST("/licenses/generate", s.handleAdminGenerateLicense)
		admin.POST("/licenses/bulk-generate", s.handleAdminBulkGenerateLicenses)
		admin.GET("/licenses", s.handleAdminListLicenses)
		admin.GET("/licenses/stats", s.handleAdminGetLicenseStats)
		admin.GET("/licenses/:id", s.handleAdminGetLicense)
		admin.PUT("/licenses/:id", s.handleAdminUpdateLicense)
		admin.POST("/licenses/:id/deactivate", s.handleAdminDeactivateLicense)
		admin.DELETE("/licenses/:id", s.handleAdminDeleteLicense)
		admin.POST("/licenses/validate", s.handleAdminValidateLicense)

		// System settings management
		admin.GET("/settings", s.handleAdminGetAllSettings)
		admin.GET("/settings/smtp", s.handleAdminGetSMTPSettings)
		admin.PUT("/settings/smtp", s.handleAdminUpdateSMTPSettings)
		admin.POST("/settings/smtp/test", s.handleAdminTestSMTP)
		admin.GET("/settings/:key", s.handleAdminGetSetting)
		admin.PUT("/settings/:key", s.handleAdminUpdateSetting)
		admin.DELETE("/settings/:key", s.handleAdminDeleteSetting)
	}

	// WebSocket endpoints
	// Legacy public WebSocket (for price updates, market data - no user-specific data)
	s.router.GET("/ws", s.handleWebSocket)
	// User-authenticated WebSocket (for user-specific data: positions, balance, PnL)
	s.router.GET("/ws/user", AuthenticatedWSHandler(s))

	// Stripe webhook endpoint (no auth required - uses signature verification)
	s.router.POST("/api/billing/webhook", s.handleStripeWebhook)

	// Serve static files (React build) in production
	if s.config.StaticFilesPath != "" {
		s.router.Static("/assets", s.config.StaticFilesPath+"/assets")
		s.router.StaticFile("/", s.config.StaticFilesPath+"/index.html")

		// Catch-all for undefined API routes - return 404 JSON
		s.router.NoRoute(func(c *gin.Context) {
			// If this is an API request path that wasn't matched by any handler,
			// return 404 JSON instead of serving index.html
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "API endpoint not found",
					"path":    c.Request.URL.Path,
					"method":  c.Request.Method,
					"message": "This API endpoint does not exist. Check your request path and HTTP method.",
				})
				return
			}

			// For non-API paths, serve React's index.html to support client-side routing
			c.File(s.config.StaticFilesPath + "/index.html")
		})
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting HTTP server on %s", addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// handleHealth returns server health status
func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Check database health
	dbHealthy := true
	if err := s.repo.HealthCheck(ctx); err != nil {
		dbHealthy = false
	}

	status := "healthy"
	if !dbHealthy {
		status = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   status,
			"database": "unhealthy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   status,
		"database": "healthy",
		"uptime":   time.Now().Format(time.RFC3339),
	})
}

// errorResponse is a helper to send error responses
func errorResponse(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error":   true,
		"message": message,
	})
}

// successResponse is a helper to send success responses
func successResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    data,
	})
}

// getUserID returns the user ID from the context, or empty string if not authenticated
func (s *Server) getUserID(c *gin.Context) string {
	if !s.authEnabled {
		// Return default admin user ID for backward compatibility when auth is disabled
		return "00000000-0000-0000-0000-000000000000"
	}
	return auth.GetUserID(c)
}

// getUserIDRequired returns the user ID from the context and sends error if not authenticated
func (s *Server) getUserIDRequired(c *gin.Context) (string, bool) {
	userID := s.getUserID(c)
	if userID == "" && s.authEnabled {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "UNAUTHORIZED",
			"message": "authentication required",
		})
		return "", false
	}
	return userID, true
}

// getUserTier returns the user's subscription tier
func (s *Server) getUserTier(c *gin.Context) string {
	if !s.authEnabled {
		return "whale" // Unlimited access when auth is disabled
	}
	return auth.GetUserTier(c)
}

// isUserAdmin checks if the current user is an admin
func (s *Server) isUserAdmin(c *gin.Context) bool {
	if !s.authEnabled {
		return true // Admin access when auth is disabled
	}
	return auth.IsAdmin(c)
}

// SetUserAutopilotManager sets the multi-user autopilot manager
func (s *Server) SetUserAutopilotManager(mgr *autopilot.UserAutopilotManager) {
	s.userAutopilotManager = mgr
}

// GetUserAutopilotManager returns the multi-user autopilot manager
func (s *Server) GetUserAutopilotManager() *autopilot.UserAutopilotManager {
	return s.userAutopilotManager
}
