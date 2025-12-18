package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP API server
type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	repo       *database.Repository
	eventBus   *events.EventBus
	botAPI     BotAPI
	config     ServerConfig
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
	corsConfig.AllowOrigins = []string{"http://localhost:5173", "http://localhost:8088"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsConfig.ExposeHeaders = []string{"Content-Length"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

	server := &Server{
		router:   router,
		repo:     repo,
		eventBus: eventBus,
		botAPI:   botAPI,
		config:   config,
	}

	server.setupRoutes()

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", s.handleHealth)

	// API routes
	api := s.router.Group("/api")
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

		// Futures trading endpoints
		futures := api.Group("/futures")
		{
			// Account endpoints
			futures.GET("/account", s.handleGetFuturesAccountInfo)
			futures.GET("/wallet-balance", s.handleGetFuturesWalletBalance)
			futures.GET("/positions", s.handleGetFuturesPositions)
			futures.POST("/positions/close-all", s.handleCloseAllFuturesPositions) // Panic button - must be before :symbol route
			futures.POST("/positions/:symbol/close", s.handleCloseFuturesPosition)

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

			// Market data endpoints
			futures.GET("/funding-rate/:symbol", s.handleGetFundingRate)
			futures.GET("/orderbook/:symbol", s.handleGetOrderBookDepth)
			futures.GET("/mark-price/:symbol", s.handleGetMarkPrice)
			futures.GET("/symbols", s.handleGetFuturesSymbols)
			futures.GET("/klines", s.handleGetFuturesKlines)

			// History endpoints
			futures.GET("/trades/history", s.handleGetFuturesTradeHistory)
			futures.GET("/funding-fees/history", s.handleGetFundingFeeHistory)
			futures.GET("/transactions/history", s.handleGetFuturesTransactionHistory)
			futures.GET("/metrics", s.handleGetFuturesMetrics)

			// Autopilot endpoints
			futures.GET("/autopilot/status", s.handleGetFuturesAutopilotStatus)
			futures.POST("/autopilot/toggle", s.handleToggleFuturesAutopilot)
			futures.POST("/autopilot/dry-run", s.handleSetFuturesAutopilotDryRun)
			futures.POST("/autopilot/risk-level", s.handleSetFuturesAutopilotRiskLevel)
			futures.POST("/autopilot/allocation", s.handleSetFuturesAutopilotAllocation)
			futures.POST("/autopilot/profit-reinvest", s.handleSetFuturesAutopilotProfitReinvest)
			futures.GET("/autopilot/profit-stats", s.handleGetFuturesAutopilotProfitStats)
			futures.POST("/autopilot/reset-allocation", s.handleResetFuturesAutopilotAllocation)
		}
	}

	// WebSocket endpoint
	s.router.GET("/ws", s.handleWebSocket)

	// Serve static files (React build) in production
	if s.config.StaticFilesPath != "" {
		s.router.Static("/assets", s.config.StaticFilesPath+"/assets")
		s.router.StaticFile("/", s.config.StaticFilesPath+"/index.html")

		// Catch-all route for React Router (SPA)
		s.router.NoRoute(func(c *gin.Context) {
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
