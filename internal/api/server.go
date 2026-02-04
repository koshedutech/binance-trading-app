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
	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/decision"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/license"
	"binance-trading-bot/internal/research"
	"binance-trading-bot/internal/settlement"
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

	// Story 6.5: Settings cache service for cache-first API pattern
	settingsCacheService *cache.SettingsCacheService

	// Story 6.4: Admin defaults cache service for settings comparison
	adminDefaultsCacheService *cache.AdminDefaultsCacheService

	// Epic 8: Settlement service for daily P&L and analytics
	settlementService *settlement.SettlementService

	// Story 7.20: Order chain cache for fast UI reads
	orderChainCache *cache.OrderChainCache

	// Story 11.14: Indicator performance tracker for trade analysis
	indicatorPerfTracker *decision.IndicatorPerformanceTracker

	// Epic 11: State manager for coin state management
	stateManager *decision.StateManager

	// Story 11.45: Strategy hierarchy cache service for sub-strategy settings
	strategyHierarchyCacheService *cache.StrategyHierarchyCacheService

	// Epic 15, Story 15.3: Data downloader for historical candle data
	dataDownloader *research.DataDownloader

	// Epic 15, Story 15.6: Feature calculation background job service
	featureJobService *research.FeatureJobService

	// Epic 15, Story 15.9: Backtest and walk-forward engines
	backtestEngine    *research.BacktestEngine
	walkForwardEngine *research.WalkForwardEngine
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
		"/api/futures/ginie/rate-limiter/status":       true,
		// LLM & Adaptive AI endpoints (internal state only - Story 2.8)
		"/api/futures/ginie/llm-config":                true,
		"/api/futures/ginie/adaptive-recommendations":  true,
		"/api/futures/ginie/llm-diagnostics-v2":        true,
		"/api/futures/ginie/trade-history-ai":          true,
		// Instance Control endpoints (Story 9.6 - Redis state only, no Binance API)
		"/api/futures/ginie/instance-status":           true,
		"/api/futures/ginie/take-control":              true,
		"/api/futures/ginie/release-control":           true,
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
		// Decision Engine coin state (Redis state only)
		"/api/futures/decision/coins":                  true,
		"/api/futures/decision/coins/count":            true,
		// Mode-Strategy endpoints (cache/DB only, no Binance API - Story 11.41)
		"/api/futures/modes/:mode/strategies":                             true,
		"/api/futures/modes/:mode/strategies/:strategy":                   true,
		"/api/futures/modes/:mode/strategies/:strategy/compare":           true,
		"/api/futures/modes/:mode/strategies/:strategy/enable":            true,
		"/api/futures/modes/:mode/strategies/:strategy/disable":           true,
		"/api/futures/modes/:mode/strategies/:strategy/reset":             true,
		"/api/futures/modes/:mode/strategies/:strategy/sections":          true,
		"/api/futures/modes/:mode/strategies/:strategy/sections/:section": true,
		// Strategy Hierarchy endpoints (cache/DB only, no Binance API - Story 11.45)
		"/api/futures/strategy-groups/:mode":                             true,
		"/api/futures/strategy-groups/:mode/:group":                      true,
		"/api/futures/strategy-groups/:mode/:group/compare":              true,
		"/api/futures/sub-strategies/:mode/:group":                       true,
		"/api/futures/sub-strategies/:mode/:group/:strategy":             true,
		"/api/futures/sub-strategies/:mode/:group/:strategy/compare":     true,
		"/api/futures/patterns/volume-imbalance":                         true,
		"/api/futures/patterns/volume-imbalance/:symbol":                 true,
		"/api/futures/enabled-strategies":                                true,
		// Chain Entry Runner endpoints (internal state only)
		"/api/futures/chain-entry/status":                                true,
		// Coin Profiler endpoints (internal state only - Epic 14)
		"/api/futures/coin-profiler/status":                              true,
		"/api/futures/coin-profiler/coins":                               true,
		"/api/futures/coin-profiler/coins/:symbol":                       true,
		"/api/futures/coin-profiler/requirements":                        true,
		// Entry Decision API endpoints (internal state only - Story 14.12)
		"/api/futures/entry-decision/strategies":                         true,
		"/api/futures/entry-decision/strategies/:mode":                   true,
		"/api/futures/entry-decision/candidates":                         true,
		"/api/futures/entry-decision/pattern/:symbol":                    true,
		"/api/futures/entry-decision/score/:symbol":                      true,
		// Position Controller endpoints (internal state only - Story 10.4)
		"/api/futures/position-controller/status":                        true,
		// Research Infrastructure endpoints (internal state only - Epic 15)
		"/api/research/download-data":                                    true,
		"/api/research/download-status/:job_id":                          true,
		"/api/research/data-availability":                                true,
		"/api/research/download-cancel/:job_id":                          true,
		"/api/research/download-resume/:job_id":                          true,
		"/api/research/download-jobs":                                    true,
		// Backtest endpoints (Story 15.9 - internal computation, no Binance API)
		"/api/research/backtest":                                         true,
		"/api/research/walk-forward":                                     true,
		"/api/research/features":                                         true,
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

	// Convenience alias for circuit breaker status
	s.router.GET("/api/circuit-breaker/status", func(c *gin.Context) {
		c.Redirect(http.StatusTemporaryRedirect, "/api/settings/circuit-breaker")
	})

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
			// Paper balance endpoints (Story PAPER-002)
			settings.GET("/paper-balance", s.handleGetPaperBalance)
			settings.PUT("/paper-balance", s.handleUpdatePaperBalance)
			settings.POST("/sync-paper-balance", s.handleSyncPaperBalance)
			// Load defaults endpoints (Story 4.14)
			settings.GET("/diff/modes/:mode", s.handleGetModeDiff)
			settings.POST("/load-defaults", s.handleLoadAllDefaults)

			// System control switches (Epic 7: Chain-based order tracking)
			settings.GET("/system-control", s.handleGetSystemControl)
			settings.PUT("/system-control", s.handleUpdateSystemControl)
			settings.PUT("/system-control/order-tracking", s.handleSetOrderTrackingSystem)
			settings.PUT("/system-control/position-management", s.handleSetPositionManagementSystem)
			settings.PUT("/system-control/entry-decision", s.handleSetEntryDecisionSystem)
		}

		// Trading Controller endpoints (Story 14.14: Chain Trading System ON/OFF)
		// Controls Entry Decision and Exit Decision independently of Ginie Autopilot
		trading := api.Group("/trading")
		{
			trading.GET("/state", s.handleGetTradingState)
			trading.PUT("/state", s.handleSetTradingState)
			trading.POST("/enable", s.handleEnableTrading)
			trading.POST("/disable", s.handleDisableTrading)
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

			// Settings comparison and reset (Story 4.16)
			user.GET("/settings/comparison", s.handleGetSettingsComparison)
			user.POST("/settings/reset", s.handleResetSingleSetting)

			// Global Circuit Breaker endpoints (Story 5.3)
			user.GET("/global-circuit-breaker", s.handleGetGlobalCircuitBreaker)
			user.PUT("/global-circuit-breaker", s.handleUpdateGlobalCircuitBreaker)

			// Timezone endpoints (Story 7.6)
			user.GET("/timezone", s.handleGetUserTimezone)
			user.PUT("/timezone", s.handleUpdateUserTimezone)
			user.GET("/timezone/presets", s.handleGetTimezonePresets)
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
			futures.GET("/commission-rate", s.handleGetCommissionRate)
			futures.GET("/wallet-balance", s.handleGetFuturesWalletBalance)
			futures.GET("/positions", s.handleGetFuturesPositions)
			futures.POST("/positions/close-all", s.handleCloseAllFuturesPositions) // Panic button - must be before :symbol route
			futures.POST("/positions/:symbol/close", s.handleCloseFuturesPosition)
			futures.GET("/positions/:symbol/orders", s.handleGetPositionOrders)   // Get TP/SL orders for position
			futures.POST("/positions/:symbol/tpsl", s.handleSetPositionTPSL)       // Set TP/SL for position
			futures.GET("/positions/:symbol/expanded", s.handleGetExpandedPositionData)  // Story 10.1: Expanded position card data
			futures.GET("/positions/:symbol/exit-decision", s.handleGetExitDecisionState) // Story 10.3: Exit decision monitoring

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

			// Order chains with state endpoint (Story 7.14)
			futures.GET("/order-chains", s.handleGetOrderChainsWithState)

			// Order chain cache endpoints (Story 7.20)
			futures.GET("/order-chains/cached", s.handleGetCachedOrderChains)
			futures.GET("/order-chains/cached/:chainId", s.handleGetCachedOrderChain)

			// Order chain history endpoint (supports date range filtering)
			futures.GET("/order-chains/history", s.handleGetHistoricalOrderChains)

			// Order chain sync endpoint (reconciles with Binance state)
			futures.POST("/order-chains/sync", s.handleSyncOrderState)

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
			futures.GET("/income-history", s.handleGetIncomeHistory) // PnL, fees, funding from Binance
			futures.GET("/pnl-summary", s.handleGetPnLSummary)      // Daily/Weekly PnL with fees breakdown
			futures.GET("/pnl-history", s.handleGetPnLHistory)      // Story 13.1: Extended date range P&L with caching
			futures.GET("/test-daily-pnl", s.handleTestDailyPnLFromTrades) // Test: Compare trades vs income history
			futures.GET("/metrics", s.handleGetFuturesMetrics)
			futures.GET("/trade-source-stats", s.handleGetTradeSourceStats)
			futures.GET("/position-trade-sources", s.handleGetPositionTradeSources)

			// Trade lifecycle events endpoints
			futures.GET("/trades/:tradeId/events", s.handleGetTradeLifecycleEvents)
			futures.GET("/trades/:tradeId/events/:eventType", s.handleGetTradeLifecycleEventsByType)
			futures.GET("/trades/:tradeId/lifecycle-summary", s.handleGetTradeLifecycleSummary)
			futures.GET("/trades/:tradeId/sl-revisions", s.handleGetTradeSLRevisionCount)
			futures.GET("/trade-events/recent", s.handleGetRecentTradeLifecycleEvents)

			// Position state endpoints (Story 7.11)
			// Note: Static routes must come BEFORE parameterized routes to avoid routing conflicts
			futures.GET("/position-states", s.handleGetActivePositionStates)
			futures.GET("/position-states/recent", s.handleGetRecentPositionStates)
			futures.GET("/position-states/symbol/:symbol", s.handleGetPositionStateBySymbol)
			futures.GET("/position-states/:chainId", s.handleGetPositionStateByChainID)

			// Order modification event endpoints (Story 7.12)
			// Note: Static routes must come BEFORE parameterized routes
			futures.GET("/modification-events/recent", s.handleGetRecentModificationEvents)
			futures.GET("/trade-lifecycle/:chainId/modifications", s.handleGetOrderModificationHistory)
			futures.GET("/trade-lifecycle/:chainId/modifications/summary", s.handleGetChainModificationSummary)
			futures.GET("/trade-lifecycle/:chainId/modifications/all", s.handleGetAllChainModifications)

			// Autopilot endpoints
			futures.GET("/autopilot/status", s.handleGetFuturesAutopilotStatus)
			futures.POST("/autopilot/toggle", s.handleToggleFuturesAutopilot)
			futures.POST("/autopilot/dry-run", s.handleSetFuturesAutopilotDryRun)
			futures.POST("/autopilot/allocation", s.handleSetFuturesAutopilotAllocation)
			futures.POST("/autopilot/profit-reinvest", s.handleSetFuturesAutopilotProfitReinvest)
			futures.GET("/autopilot/profit-stats", s.handleGetFuturesAutopilotProfitStats)
			futures.POST("/autopilot/reset-allocation", s.handleResetFuturesAutopilotAllocation)
			futures.POST("/autopilot/tpsl", s.handleSetFuturesAutopilotTPSL)
			futures.POST("/autopilot/leverage", s.handleSetFuturesAutopilotLeverage)
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

			// Convenience aliases for shorter autopilot config path
			futures.GET("/autopilot/config", s.handleGetGinieAutopilotConfig)
			futures.POST("/autopilot/config", s.handleUpdateGinieAutopilotConfig)
			futures.POST("/ginie/autopilot/start", s.handleStartGinieAutopilot)
			futures.POST("/ginie/autopilot/stop", s.handleStopGinieAutopilot)
			futures.GET("/ginie/autopilot/positions", s.handleGetGinieAutopilotPositions)
			futures.GET("/ginie/autopilot/stuck", s.handleGetStuckPositions)
			futures.GET("/ginie/autopilot/history", s.handleGetGinieAutopilotTradeHistory)
			futures.POST("/ginie/autopilot/clear", s.handleClearGinieAutopilotPositions)
			futures.POST("/ginie/refresh-symbols", s.handleRefreshGinieSymbols)

			// Bulletproof Protection Status (SL/TP health monitoring)
			futures.GET("/ginie/protection/status", s.handleGetProtectionStatus)

			// Per-symbol performance settings endpoints
			futures.GET("/autopilot/symbols", s.handleGetSymbolSettings)
			futures.GET("/autopilot/symbols/report", s.handleGetSymbolPerformanceReport)
			futures.POST("/autopilot/symbols/refresh-performance", s.handleRefreshSymbolPerformance)
			futures.GET("/autopilot/symbols/category/:category", s.handleGetSymbolsByCategory)
			futures.GET("/autopilot/symbols/:symbol", s.handleGetSingleSymbolSettings)
			futures.PUT("/autopilot/symbols/:symbol", s.handleUpdateSymbolSettings)
			futures.POST("/autopilot/symbols/:symbol/blacklist", s.handleBlacklistSymbol)
			futures.DELETE("/autopilot/symbols/:symbol/blacklist", s.handleUnblacklistSymbol)
			futures.POST("/autopilot/category-config", s.handleUpdateCategorySettings)

			// Symbol Blocking endpoints (daily worst performer blocking)
			futures.GET("/autopilot/symbols/blocked", s.handleGetBlockedSymbols)
			futures.POST("/autopilot/symbols/auto-block-worst", s.handleAutoBlockWorstPerformers)
			futures.POST("/autopilot/symbols/clear-expired-blocks", s.handleClearExpiredBlocks)
			futures.POST("/autopilot/symbols/:symbol/block-day", s.handleBlockSymbolForDay)
			futures.POST("/autopilot/symbols/:symbol/unblock", s.handleUnblockSymbol)
			futures.GET("/autopilot/symbols/:symbol/block-status", s.handleGetSymbolBlockStatus)

			// Morning auto-block configuration (scheduled daily worst performer blocking)
			futures.GET("/autopilot/morning-auto-block/config", s.handleGetMorningAutoBlockConfig)
			futures.POST("/autopilot/morning-auto-block/config", s.handleUpdateMorningAutoBlockConfig)

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


			// Ginie Market Movers endpoints (dynamic symbol selection)
			futures.GET("/ginie/market-movers", s.handleGetMarketMovers)
			futures.GET("/ginie/all-gainers", s.handleGetAllMarketMovers) // No volume filter - shows real top gainers
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

			// Ginie Pending Orders endpoint - shows unfilled limit orders
			futures.GET("/ginie/pending-orders", s.handleGetPendingOrders)

			// Ginie Trade Conditions endpoint - shows all pre-trade condition checks
			futures.GET("/ginie/trade-conditions", s.handleGetTradeConditions)

			// Ginie Rate Limiter status endpoint
			futures.GET("/ginie/rate-limiter/status", s.handleGetRateLimiterStatus)

			// Ginie SLTP Configuration endpoints
			futures.GET("/ginie/sltp-config", s.handleGetGinieSLTPConfig)
			futures.POST("/ginie/sltp-config/:mode", s.handleUpdateGinieSLTPConfig)

			// Ginie Trend Timeframe endpoints
			futures.GET("/ginie/trend-timeframes", s.handleGetGinieTrendTimeframes)
			futures.POST("/ginie/trend-timeframes", s.handleUpdateGinieTrendTimeframes)

			// Per-Coin Confluence Configuration endpoints
			futures.GET("/ginie/coin-confluence", s.handleGetAllCoinConfluenceConfigs)
			futures.GET("/ginie/coin-confluence/:symbol", s.handleGetCoinConfluenceConfig)
			futures.POST("/ginie/coin-confluence/:symbol", s.handleUpdateCoinConfluenceConfig)
			futures.DELETE("/ginie/coin-confluence/:symbol", s.handleDeleteCoinConfluenceConfig)
			futures.GET("/ginie/coin-tier/:symbol", s.handleGetCoinTier)

			// Ultra-Fast Mode Configuration endpoints
			futures.GET("/ultrafast/config", s.handleGetUltraFastConfig)
			futures.POST("/ultrafast/config", s.handleUpdateUltraFastConfig)
			futures.POST("/ultrafast/toggle", s.handleToggleUltraFast)


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
			futures.POST("/ginie/mode-config/:mode/toggle", s.handleToggleModeEnabled)
			futures.POST("/ginie/mode-config/reset", s.handleResetModeConfigs)
			futures.GET("/ginie/mode-circuit-breaker-status", s.handleGetModeCircuitBreakerStatus)
			futures.POST("/ginie/mode-circuit-breaker/:mode/reset", s.handleResetModeCircuitBreaker)

			// Load Defaults endpoints (Story 4.14)
			futures.POST("/ginie/modes/:mode/load-defaults", s.handleLoadModeDefaults)
			futures.POST("/ginie/modes/:mode/groups/:group/reset", s.handleResetModeGroup) // Reset specific group within a mode
			futures.POST("/ginie/modes/load-defaults", s.handleLoadAllModeDefaults)
			futures.GET("/ginie/default-settings", s.handleGetAllDefaultSettings) // Get all defaults from default-settings.json (Story 9.4)

			// Config Reset endpoints (Story 4.17)
			futures.POST("/ginie/circuit-breaker/load-defaults", s.handleLoadCircuitBreakerDefaults)
			futures.POST("/ginie/llm-config/load-defaults", s.handleLoadLLMConfigDefaults)
			futures.POST("/ginie/capital-allocation/load-defaults", s.handleLoadCapitalAllocationDefaults)
			futures.POST("/ginie/global-trading/load-defaults", s.handleLoadGlobalTradingDefaults)
			futures.PUT("/ginie/global-trading", s.handleUpdateGlobalTrading)
			futures.POST("/ginie/hedge-mode/load-defaults", s.handleLoadHedgeDefaults)
			futures.POST("/ginie/safety-settings/load-defaults", s.handleLoadSafetySettingsDefaults)
			futures.POST("/ginie/position-optimization/load-defaults", s.handleLoadPositionOptimizationDefaults)

			// Position Management endpoints (Story 10.1: Position Decision Configuration)
			futures.GET("/ginie/position-management", s.handleGetPositionManagement)
			futures.PUT("/ginie/position-management", s.handleUpdatePositionManagement)
			futures.POST("/ginie/position-management/load-defaults", s.handleLoadPositionManagementDefaults)

			// Batch Reset endpoints (reset multiple settings at once)
			futures.POST("/ginie/modes/reset-all", s.handleResetAllModes)
			futures.POST("/ginie/other-settings/reset-all", s.handleResetAllOtherSettings)

			// Safety Settings CRUD endpoints (Story 9.4)
			futures.GET("/ginie/safety-settings", s.handleGetUserSafetySettings)
			futures.PUT("/ginie/safety-settings/:mode", s.handleUpdateUserSafetySettings)

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

			// Scalp Re-entry Mode Configuration endpoints
			futures.GET("/ginie/scalp-reentry-config", s.handleGetScalpReentryConfig)
			futures.POST("/ginie/scalp-reentry-config", s.handleUpdateScalpReentryConfig)
			futures.POST("/ginie/scalp-reentry/toggle", s.handleToggleScalpReentry)

			// Scalp Re-entry Monitor endpoints
			futures.GET("/ginie/scalp-reentry/positions", s.handleGetScalpReentryPositions)
			futures.GET("/ginie/scalp-reentry/positions/:symbol", s.handleGetScalpReentryPositionStatus)

			// Hedge Mode Configuration endpoints (DCA + Hedge Grid)
			futures.GET("/ginie/hedge-config", s.handleGetHedgeModeConfig)
			futures.POST("/ginie/hedge-config", s.handleUpdateHedgeModeConfig)
			futures.POST("/ginie/hedge-mode/toggle", s.handleToggleHedgeMode)
			futures.GET("/ginie/hedge-mode/positions", s.handleGetHedgeModePositions)

			// Position Mode Conversion endpoint
			futures.POST("/ginie/positions/:symbol/convert-mode", s.handleConvertPositionMode)

			// Instance Control endpoints (Story 9.6 - Active/Standby Container Control)
			// These endpoints manage multi-instance coordination for zero-downtime protection
			futures.GET("/ginie/instance-status", s.handleGetInstanceStatus)
			futures.POST("/ginie/take-control", s.handleTakeControl)
			futures.POST("/ginie/release-control", s.handleReleaseControl)

			// Decision Engine Settings endpoints (Story 11.24)
			// User-configurable strategy settings for the Position Decision Engine
			futures.GET("/decision-engine/settings", s.handleGetDecisionEngineSettings)
			futures.PUT("/decision-engine/settings", s.handleUpdateDecisionEngineSettings)
			futures.POST("/decision-engine/settings/reset", s.handleResetDecisionEngineSettings)
			futures.GET("/decision-engine/settings/compare", s.handleCompareDecisionEngineSettings)
			futures.GET("/decision-engine/strategies", s.handleListDecisionEngineStrategies)
			futures.PUT("/decision-engine/strategy/:name", s.handleUpdateDecisionEngineStrategy)
			futures.POST("/decision-engine/strategy/:name/reset", s.handleResetDecisionEngineStrategy)
			futures.PUT("/decision-engine/active-strategy", s.handleSetActiveStrategy)
			futures.POST("/decision-engine/strategy/:name/enable", s.handleEnableStrategy)
			futures.POST("/decision-engine/strategy/:name/disable", s.handleDisableStrategy)

			// Mode-Strategy API endpoints (Story 11.31)
			// CRUD operations for Mode+Strategy configuration
			futures.GET("/modes", s.handleGetAllModes)
			futures.GET("/modes/:mode", s.handleGetMode)
			futures.GET("/modes/:mode/strategies", s.handleGetModeStrategies)
			futures.GET("/modes/:mode/strategies/:strategy", s.handleGetModeStrategy)
			futures.PUT("/modes/:mode/strategies/:strategy", s.handleUpdateModeStrategy)
			futures.POST("/modes/:mode/strategies/:strategy/reset", s.handleResetModeStrategy)
			futures.POST("/modes/:mode/strategies/:strategy/enable", s.handleEnableModeStrategy)
			futures.POST("/modes/:mode/strategies/:strategy/disable", s.handleDisableModeStrategy)
			futures.GET("/modes/:mode/strategies/:strategy/compare", s.handleCompareModeStrategy)
			futures.POST("/modes/:mode/reset-all", s.handleResetAllModeStrategies)

			// Story 11.41: Section-level endpoints for mode+strategy config
			futures.GET("/modes/:mode/strategies/:strategy/sections", s.handleListModeStrategySections)
			futures.GET("/modes/:mode/strategies/:strategy/sections/:section", s.handleGetModeStrategySection)
			futures.PUT("/modes/:mode/strategies/:strategy/sections/:section", s.handleUpdateModeStrategySection)
			futures.POST("/modes/:mode/strategies/:strategy/sections/:section/reset", s.handleResetModeStrategySection)

			// Calibration Data Lifecycle endpoints (Story 11.26)
			// Manual reset option and confidence indicator for calibration
			futures.POST("/calibration/reset", s.handleResetCalibration)
			futures.GET("/calibration/confidence/:strategy", s.handleGetCalibrationConfidence)
			futures.GET("/calibration/history/:strategy", s.handleGetCalibrationHistory)
			futures.GET("/calibration/data/:strategy", s.handleGetCalibrationData)

			// Indicator Performance Tracker endpoints (Story 11.14)
			// Track and analyze indicator performance across trades
			futures.GET("/indicators/performance/:strategy", s.handleGetIndicatorPerformanceGin)
			futures.GET("/indicators/recommendations/:strategy", s.handleGetIndicatorRecommendationsGin)
			futures.GET("/indicators/correlations/:strategy", s.handleGetIndicatorCorrelationsGin)
			futures.GET("/indicators/top-combinations/:strategy", s.handleGetTopIndicatorCombinationsGin)

			// Decision Engine Dashboard endpoint (Story 11.22)
			// Aggregated dashboard statistics for performance analysis
			futures.GET("/decision/dashboard/stats", s.handleGetDecisionDashboardStats)

			// Coin State API (Epic 11: Position Decision Engine)
			// Real-time coin state for PositionCard UI
			futures.GET("/decision/coin/:symbol", s.handleGetCoinState)
			futures.GET("/decision/coins", s.handleGetAllCoinStates)
			futures.GET("/decision/coins/count", s.handleGetCoinStateCount)
			// Story 11.40: Gap Analysis UI
			futures.GET("/decision/coins/gaps", s.handleGetAllCoinStatesWithGaps)
			futures.POST("/decision/override", s.handleOverrideEntry)

			// Position Analytics Dashboard endpoints (Story 10.2)
			// Historical efficiency analysis, trade categorization, performance metrics
			futures.GET("/position-analytics/summary", s.handleGetPositionAnalyticsSummary)
			futures.GET("/position-analytics/efficiency-timeline", s.handleGetEfficiencyTimeline)
			futures.GET("/position-analytics/distribution", s.handleGetTradeDistribution)
			futures.GET("/position-analytics/export", s.handleExportPositionAnalytics)

			// Strategy Hierarchy endpoints (Story 11.45: Volume Imbalance API)
			// Mode -> Strategy Group -> Sub-Strategy architecture
			futures.GET("/strategy-groups/:mode", s.handleGetStrategyGroupsForMode)
			futures.PUT("/strategy-groups/:mode/:group", s.handleUpdateStrategyGroup)
			futures.GET("/strategy-groups/:mode/:group/compare", s.handleCompareStrategyGroup)

			// Sub-Strategy endpoints
			futures.GET("/sub-strategies/:mode/:group", s.handleGetSubStrategiesForGroup)
			futures.PUT("/sub-strategies/:mode/:group/:strategy", s.handleUpdateSubStrategy)
			futures.GET("/sub-strategies/:mode/:group/:strategy/compare", s.handleCompareSubStrategy)

			// Pattern State endpoints (Volume Imbalance detector states)
			futures.GET("/patterns/volume-imbalance", s.handleGetVolumeImbalancePatterns)
			futures.GET("/patterns/volume-imbalance/:symbol", s.handleGetVolumeImbalancePatternForSymbol)

			// Enabled Strategies (quick lookup for active sub-strategies)
			futures.GET("/enabled-strategies", s.handleGetEnabledStrategies)

			// Chain Entry Runner endpoints (Epic 11: Automatic chain-based entries)
			// These run independently of Ginie Autopilot when entry_decision_system = "chain"
			futures.GET("/chain-entry/status", s.handleGetChainEntryRunnerStatus)
			futures.POST("/chain-entry/start", s.handleStartChainEntryRunner)
			futures.POST("/chain-entry/stop", s.handleStopChainEntryRunner)

			// Coin Profiler endpoints (Epic 14: Chain Trading System)
			// Real-time WebSocket data collection for Entry Decision and Exit Decision
			futures.GET("/coin-profiler/status", s.handleGetCoinProfilerStatus)
			futures.GET("/coin-profiler/coins", s.handleGetCoinProfilerCoins)
			futures.GET("/coin-profiler/coins/:symbol", s.handleGetCoinProfilerCoin)
			futures.GET("/coin-profiler/requirements", s.handleGetCoinProfilerRequirements)
			futures.POST("/coin-profiler/start", s.handleStartCoinProfiler)
			futures.POST("/coin-profiler/stop", s.handleStopCoinProfiler)
			futures.GET("/coin-profiler/diagnostics", s.handleGetCoinProfilerDiagnostics)

			// Entry Decision API endpoints (Story 14.12: Entry Decision System API)
			// Strategy-first view of entry opportunities
			futures.GET("/entry-decision/strategies", s.handleGetEntryDecisionStrategies)
			futures.GET("/entry-decision/strategies/:mode", s.handleGetEntryDecisionStrategiesForMode)
			futures.GET("/entry-decision/candidates", s.handleGetEntryCandidates)
			futures.GET("/entry-decision/pattern/:symbol", s.handleGetPatternProgress)
			futures.GET("/entry-decision/score/:symbol", s.handleGetScoreBreakdown)

			// Position Controller endpoints (Story 10.4: Exit Signal Executor)
			// Executes exit signals from Exit Decision Service on Binance
			futures.GET("/position-controller/status", s.GetPositionControllerStatus)
			futures.POST("/position-controller/heal", s.TriggerPositionControllerHeal)
			futures.POST("/position-controller/start", s.StartPositionController)
			futures.POST("/position-controller/stop", s.StopPositionController)
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
			spot.POST("/autopilot/allocation", s.handleSetSpotAutopilotAllocation)
			spot.POST("/autopilot/max-positions", s.handleSetSpotAutopilotMaxPositions)
			spot.POST("/autopilot/tpsl", s.handleSetSpotAutopilotTPSL)
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

		// ==================== RESEARCH INFRASTRUCTURE ENDPOINTS ====================
		// Epic 15: Pattern Discovery Agent - Data download and availability
		research := api.Group("/research")
		{
			// Data download endpoints (Story 15.3)
			research.POST("/download-data", s.handleStartDownload)
			research.GET("/download-status/:job_id", s.handleGetDownloadStatus)
			research.GET("/data-availability", s.handleGetDataAvailability)
			research.POST("/download-cancel/:job_id", s.handleCancelDownload)
			research.POST("/download-resume/:job_id", s.handleResumeDownload)
			research.GET("/download-jobs", s.handleListDownloadJobs)

			// Feature calculation endpoints (Story 15.6)
			research.POST("/calculate-features", s.handleTriggerFeatureCalculation)
			research.GET("/feature-job/:job_id", s.handleGetFeatureJobStatus)
			research.GET("/feature-jobs", s.handleListFeatureJobs)
			research.GET("/feature-job-stats", s.handleGetFeatureJobStats)

			// Backtest endpoints (Story 15.9)
			research.POST("/backtest", s.handleRunResearchBacktest)
			research.POST("/walk-forward", s.handleRunResearchWalkForward)
			research.GET("/features", s.handleGetResearchFeatures)
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

		// Admin settings sync (Story 4.15)
		admin.POST("/sync-defaults", s.handleAdminSyncDefaults)
		admin.GET("/sync-status", s.handleAdminSyncStatus)
		admin.POST("/restore-backup", s.handleAdminRestoreBackup)

		// Admin defaults editing (Story 9.4)
		admin.POST("/defaults/:configType", s.handleAdminSaveDefaults)

		// Settlement management (Epic 8 Stories 8.5, 8.8, 8.9, 8.10)
		admin.GET("/daily-summaries/all", s.handleAdminDailySummariesGin)
		admin.GET("/daily-summaries/export", s.handleAdminExportCSVGin)
		admin.GET("/settlements/status", s.handleAdminSettlementStatusGin)
		admin.POST("/settlements/retry/:user_id/:date", s.handleAdminSettlementRetryGin)
		admin.GET("/settlements/review-queue", s.handleAdminReviewQueueGin)
		admin.POST("/settlements/approve/:id", s.handleAdminApproveSummaryGin)
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
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second, // Increased for LLM calls and slow operations
		IdleTimeout:  120 * time.Second,
	}

	// Start Entry Decision broadcast service for real-time WebSocket updates
	s.StartEntryDecisionBroadcast()
	log.Println("Entry Decision broadcast service started")

	log.Printf("Starting HTTP server on %s", addr)

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server...")

	// Stop Entry Decision broadcast service
	StopEntryDecisionBroadcast()

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

// SetSettingsCacheService sets the settings cache service for cache-first API pattern
// Story 6.5: Cache-First Read Pattern APIs
func (s *Server) SetSettingsCacheService(svc *cache.SettingsCacheService) {
	s.settingsCacheService = svc
}

// GetSettingsCacheService returns the settings cache service
func (s *Server) GetSettingsCacheService() *cache.SettingsCacheService {
	return s.settingsCacheService
}

// SetAdminDefaultsCacheService sets the admin defaults cache service for settings comparison
// Story 6.4: Cache-first settings comparison
func (s *Server) SetAdminDefaultsCacheService(svc *cache.AdminDefaultsCacheService) {
	s.adminDefaultsCacheService = svc
}

// GetAdminDefaultsCacheService returns the admin defaults cache service
func (s *Server) GetAdminDefaultsCacheService() *cache.AdminDefaultsCacheService {
	return s.adminDefaultsCacheService
}

// SetSettlementService sets the settlement service for daily P&L analytics
// Epic 8: Daily Settlement & Mode Analytics
func (s *Server) SetSettlementService(svc *settlement.SettlementService) {
	s.settlementService = svc
}

// GetSettlementService returns the settlement service
func (s *Server) GetSettlementService() *settlement.SettlementService {
	return s.settlementService
}

// SetOrderChainCache sets the order chain cache for fast UI reads
// Story 7.20: Order Chain Redis Cache Layer
func (s *Server) SetOrderChainCache(cache *cache.OrderChainCache) {
	s.orderChainCache = cache
}

// GetOrderChainCache returns the order chain cache
func (s *Server) GetOrderChainCache() *cache.OrderChainCache {
	return s.orderChainCache
}

// SetIndicatorPerformanceTracker sets the indicator performance tracker.
// Story 11.14: Indicator Performance Tracker
func (s *Server) SetIndicatorPerformanceTracker(tracker *decision.IndicatorPerformanceTracker) {
	s.indicatorPerfTracker = tracker
}

// GetIndicatorPerformanceTracker returns the indicator performance tracker.
func (s *Server) GetIndicatorPerformanceTracker() *decision.IndicatorPerformanceTracker {
	return s.indicatorPerfTracker
}

// SetStateManager sets the decision engine state manager.
// Epic 11: Position Decision Engine
func (s *Server) SetStateManager(sm *decision.StateManager) {
	s.stateManager = sm
}

// GetStateManager returns the decision engine state manager.
func (s *Server) GetStateManager() *decision.StateManager {
	return s.stateManager
}

// SetStrategyHierarchyCacheService sets the strategy hierarchy cache service.
// Story 11.45: Volume Imbalance API Endpoints
func (s *Server) SetStrategyHierarchyCacheService(svc *cache.StrategyHierarchyCacheService) {
	s.strategyHierarchyCacheService = svc
}

// GetStrategyHierarchyCacheService returns the strategy hierarchy cache service.
func (s *Server) GetStrategyHierarchyCacheService() *cache.StrategyHierarchyCacheService {
	return s.strategyHierarchyCacheService
}

// SetDataDownloader sets the data downloader for research infrastructure.
// Epic 15, Story 15.3: Data Download API Endpoints
func (s *Server) SetDataDownloader(dl *research.DataDownloader) {
	s.dataDownloader = dl
}

// GetDataDownloader returns the data downloader.
func (s *Server) GetDataDownloader() *research.DataDownloader {
	return s.dataDownloader
}

// SetFeatureJobService sets the feature job service for research infrastructure.
// Epic 15, Story 15.6: Feature Calculation Background Job
func (s *Server) SetFeatureJobService(svc *research.FeatureJobService) {
	s.featureJobService = svc
}

// GetFeatureJobService returns the feature job service.
func (s *Server) GetFeatureJobService() *research.FeatureJobService {
	return s.featureJobService
}

// SetBacktestEngine sets the backtest engine for research infrastructure.
// Epic 15, Story 15.9: Backtest API Endpoints
func (s *Server) SetBacktestEngine(engine *research.BacktestEngine) {
	s.backtestEngine = engine
}

// GetBacktestEngine returns the backtest engine.
func (s *Server) GetBacktestEngine() *research.BacktestEngine {
	return s.backtestEngine
}

// SetWalkForwardEngine sets the walk-forward engine for research infrastructure.
// Epic 15, Story 15.9: Backtest API Endpoints
func (s *Server) SetWalkForwardEngine(engine *research.WalkForwardEngine) {
	s.walkForwardEngine = engine
}

// GetWalkForwardEngine returns the walk-forward engine.
func (s *Server) GetWalkForwardEngine() *research.WalkForwardEngine {
	return s.walkForwardEngine
}
