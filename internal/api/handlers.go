package api

import (
	"binance-trading-bot/internal/database"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// BOT HANDLERS
// ============================================================================

// handleBotStatus returns the current bot status
func (s *Server) handleBotStatus(c *gin.Context) {
	status := s.botAPI.GetStatus()
	successResponse(c, status)
}

// handleBotConfig returns the bot configuration
func (s *Server) handleBotConfig(c *gin.Context) {
	// This would return config info (safe parts only, no API keys)
	config := map[string]interface{}{
		"testnet": true, // Get from actual config
		// Add other safe config fields
	}
	successResponse(c, config)
}

// ============================================================================
// POSITION HANDLERS
// ============================================================================

// handleGetPositions returns all open positions
func (s *Server) handleGetPositions(c *gin.Context) {
	positions := s.botAPI.GetOpenPositions()
	successResponse(c, positions)
}

// handleGetPositionHistory returns historical positions
func (s *Server) handleGetPositionHistory(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination parameters
	limit := 50
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get closed trades from database
	trades, err := s.repo.GetTradeHistory(ctx, limit, offset)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch position history")
		return
	}

	successResponse(c, trades)
}

// handleClosePosition closes an open position
func (s *Server) handleClosePosition(c *gin.Context) {
	symbol := c.Param("symbol")

	if symbol == "" {
		errorResponse(c, http.StatusBadRequest, "Symbol is required")
		return
	}

	if err := s.botAPI.ClosePosition(symbol); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	successResponse(c, gin.H{"message": "Position closed successfully"})
}

// ============================================================================
// ORDER HANDLERS
// ============================================================================

// PlaceOrderRequest represents a manual order placement request
type PlaceOrderRequest struct {
	Symbol    string  `json:"symbol" binding:"required"`
	Side      string  `json:"side" binding:"required,oneof=BUY SELL"`
	OrderType string  `json:"order_type" binding:"required,oneof=MARKET LIMIT"`
	Quantity  float64 `json:"quantity" binding:"required,gt=0"`
	Price     float64 `json:"price"`
}

// handlePlaceOrder places a manual order
func (s *Server) handlePlaceOrder(c *gin.Context) {
	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate LIMIT orders have price
	if req.OrderType == "LIMIT" && req.Price <= 0 {
		errorResponse(c, http.StatusBadRequest, "Price is required for LIMIT orders")
		return
	}

	// Place order through bot API
	orderID, err := s.botAPI.PlaceOrder(req.Symbol, req.Side, req.OrderType, req.Quantity, req.Price)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to place order: "+err.Error())
		return
	}

	successResponse(c, gin.H{
		"order_id": orderID,
		"message":  "Order placed successfully",
	})
}

// handleCancelOrder cancels an order
func (s *Server) handleCancelOrder(c *gin.Context) {
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid order ID")
		return
	}

	if err := s.botAPI.CancelOrder(orderID); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to cancel order: "+err.Error())
		return
	}

	successResponse(c, gin.H{"message": "Order cancelled successfully"})
}

// handleGetActiveOrders returns all active orders
func (s *Server) handleGetActiveOrders(c *gin.Context) {
	ctx := c.Request.Context()

	orders, err := s.repo.GetActiveOrders(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch active orders")
		return
	}

	successResponse(c, orders)
}

// handleGetOrderHistory returns order history
func (s *Server) handleGetOrderHistory(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination
	limit := 50
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	orders, err := s.repo.GetOrderHistory(ctx, limit, offset)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch order history")
		return
	}

	successResponse(c, orders)
}

// ============================================================================
// STRATEGY HANDLERS
// ============================================================================

// handleGetStrategies returns all registered strategies
func (s *Server) handleGetStrategies(c *gin.Context) {
	strategies := s.botAPI.GetStrategies()
	successResponse(c, strategies)
}

// ToggleStrategyRequest represents a strategy toggle request
type ToggleStrategyRequest struct {
	Enabled bool `json:"enabled"`
}

// handleToggleStrategy enables or disables a strategy
func (s *Server) handleToggleStrategy(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		errorResponse(c, http.StatusBadRequest, "Strategy name is required")
		return
	}

	var req ToggleStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	if err := s.botAPI.ToggleStrategy(name, req.Enabled); err != nil {
		errorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	action := "disabled"
	if req.Enabled {
		action = "enabled"
	}

	successResponse(c, gin.H{
		"message": "Strategy " + action + " successfully",
		"name":    name,
		"enabled": req.Enabled,
	})
}

// ============================================================================
// SIGNAL HANDLERS
// ============================================================================

// handleGetSignals returns recent trading signals
func (s *Server) handleGetSignals(c *gin.Context) {
	ctx := c.Request.Context()

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	signals, err := s.repo.GetRecentSignals(ctx, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch signals")
		return
	}

	successResponse(c, signals)
}

// ============================================================================
// SCREENER HANDLERS
// ============================================================================

// handleGetScreenerResults returns latest market screener results
func (s *Server) handleGetScreenerResults(c *gin.Context) {
	ctx := c.Request.Context()

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results, err := s.repo.GetLatestScreenerResults(ctx, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch screener results")
		return
	}

	successResponse(c, results)
}

// ============================================================================
// METRICS HANDLERS
// ============================================================================

// handleGetMetrics returns trading metrics and statistics
func (s *Server) handleGetMetrics(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	metrics, err := s.repo.GetTradingMetrics(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch metrics")
		return
	}

	successResponse(c, metrics)
}

// ============================================================================
// SYSTEM EVENT HANDLERS
// ============================================================================

// handleGetSystemEvents returns recent system events
func (s *Server) handleGetSystemEvents(c *gin.Context) {
	ctx := c.Request.Context()

	limit := 100
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	events, err := s.repo.GetRecentSystemEvents(ctx, limit)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch system events")
		return
	}

	successResponse(c, events)
}

// ============================================================================
// STRATEGY CONFIG HANDLERS
// ============================================================================

// CreateStrategyConfigRequest represents a request to create a strategy configuration
type CreateStrategyConfigRequest struct {
	Name              string                 `json:"name" binding:"required"`
	Symbol            string                 `json:"symbol" binding:"required"`
	Timeframe         string                 `json:"timeframe" binding:"required"`
	IndicatorType     string                 `json:"indicator_type" binding:"required"`
	Autopilot         bool                   `json:"autopilot"`
	Enabled           bool                   `json:"enabled"`
	PositionSize      float64                `json:"position_size" binding:"required,gt=0"`
	StopLossPercent   float64                `json:"stop_loss_percent" binding:"required,gt=0"`
	TakeProfitPercent float64                `json:"take_profit_percent" binding:"required,gt=0"`
	ConfigParams      map[string]interface{} `json:"config_params"`
}

// handleCreateStrategyConfig creates a new strategy configuration
func (s *Server) handleCreateStrategyConfig(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateStrategyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	config := &database.StrategyConfig{
		Name:              req.Name,
		Symbol:            req.Symbol,
		Timeframe:         req.Timeframe,
		IndicatorType:     req.IndicatorType,
		Autopilot:         req.Autopilot,
		Enabled:           req.Enabled,
		PositionSize:      req.PositionSize,
		StopLossPercent:   req.StopLossPercent,
		TakeProfitPercent: req.TakeProfitPercent,
		ConfigParams:      req.ConfigParams,
	}

	if err := s.repo.CreateStrategyConfig(ctx, config); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to create strategy config: "+err.Error())
		return
	}

	successResponse(c, config)
}

// handleGetStrategyConfigs returns all strategy configurations
func (s *Server) handleGetStrategyConfigs(c *gin.Context) {
	ctx := c.Request.Context()

	configs, err := s.repo.GetAllStrategyConfigs(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch strategy configs")
		return
	}

	successResponse(c, configs)
}

// handleGetStrategyConfig returns a specific strategy configuration
func (s *Server) handleGetStrategyConfig(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy config ID")
		return
	}

	config, err := s.repo.GetStrategyConfigByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Strategy config not found")
		return
	}

	successResponse(c, config)
}

// UpdateStrategyConfigRequest represents a request to update a strategy configuration
type UpdateStrategyConfigRequest struct {
	Symbol            string                 `json:"symbol"`
	Timeframe         string                 `json:"timeframe"`
	IndicatorType     string                 `json:"indicator_type"`
	Autopilot         *bool                  `json:"autopilot"`
	Enabled           *bool                  `json:"enabled"`
	PositionSize      *float64               `json:"position_size"`
	StopLossPercent   *float64               `json:"stop_loss_percent"`
	TakeProfitPercent *float64               `json:"take_profit_percent"`
	ConfigParams      map[string]interface{} `json:"config_params"`
}

// handleUpdateStrategyConfig updates a strategy configuration
func (s *Server) handleUpdateStrategyConfig(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy config ID")
		return
	}

	// Get existing config
	config, err := s.repo.GetStrategyConfigByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Strategy config not found")
		return
	}

	var req UpdateStrategyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Update fields
	if req.Symbol != "" {
		config.Symbol = req.Symbol
	}
	if req.Timeframe != "" {
		config.Timeframe = req.Timeframe
	}
	if req.IndicatorType != "" {
		config.IndicatorType = req.IndicatorType
	}
	if req.Autopilot != nil {
		config.Autopilot = *req.Autopilot
	}
	if req.Enabled != nil {
		config.Enabled = *req.Enabled
	}
	if req.PositionSize != nil {
		config.PositionSize = *req.PositionSize
	}
	if req.StopLossPercent != nil {
		config.StopLossPercent = *req.StopLossPercent
	}
	if req.TakeProfitPercent != nil {
		config.TakeProfitPercent = *req.TakeProfitPercent
	}
	if req.ConfigParams != nil {
		config.ConfigParams = req.ConfigParams
	}

	if err := s.repo.UpdateStrategyConfig(ctx, config); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update strategy config: "+err.Error())
		return
	}

	successResponse(c, config)
}

// handleDeleteStrategyConfig deletes a strategy configuration
func (s *Server) handleDeleteStrategyConfig(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid strategy config ID")
		return
	}

	if err := s.repo.DeleteStrategyConfig(ctx, id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to delete strategy config: "+err.Error())
		return
	}

	successResponse(c, gin.H{"message": "Strategy config deleted successfully"})
}

// ============================================================================
// PENDING SIGNAL HANDLERS
// ============================================================================

// handleGetPendingSignals returns all pending signals awaiting confirmation
func (s *Server) handleGetPendingSignals(c *gin.Context) {
	ctx := c.Request.Context()

	signals, err := s.repo.GetPendingSignals(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch pending signals")
		return
	}

	successResponse(c, signals)
}

// handleGetPendingSignal returns a specific pending signal
func (s *Server) handleGetPendingSignal(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid signal ID")
		return
	}

	signal, err := s.repo.GetPendingSignalByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Signal not found")
		return
	}

	successResponse(c, signal)
}

// ConfirmSignalRequest represents a request to confirm or reject a signal
type ConfirmSignalRequest struct {
	Action string `json:"action" binding:"required,oneof=CONFIRM REJECT"`
}

// handleConfirmPendingSignal confirms or rejects a pending signal
func (s *Server) handleConfirmPendingSignal(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid signal ID")
		return
	}

	var req ConfirmSignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Get the pending signal
	signal, err := s.repo.GetPendingSignalByID(ctx, id)
	if err != nil {
		errorResponse(c, http.StatusNotFound, "Signal not found")
		return
	}

	if signal.Status != "PENDING" {
		errorResponse(c, http.StatusBadRequest, "Signal already processed")
		return
	}

	// Get current price for the symbol
	currentPrice := signal.CurrentPrice // Use existing price as fallback

	if req.Action == "CONFIRM" {
		// Execute the trade through bot API
		if err := s.botAPI.ExecutePendingSignal(signal); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to execute signal: "+err.Error())
			return
		}

		// Update signal status
		if err := s.repo.UpdatePendingSignalStatus(ctx, id, "CONFIRMED", currentPrice); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to update signal status")
			return
		}

		successResponse(c, gin.H{
			"message": "Signal confirmed and trade executed",
			"signal_id": id,
		})
	} else {
		// Reject the signal
		if err := s.repo.UpdatePendingSignalStatus(ctx, id, "REJECTED", currentPrice); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to update signal status")
			return
		}

		successResponse(c, gin.H{
			"message": "Signal rejected",
			"signal_id": id,
		})
	}
}

// ============================================================================
// BINANCE DATA HANDLERS
// ============================================================================

// handleGetBinanceSymbols returns all available trading symbols from Binance
func (s *Server) handleGetBinanceSymbols(c *gin.Context) {
	client := s.botAPI.GetBinanceClient()

	// Type assert to get the actual Binance client
	type BinanceClient interface {
		GetExchangeInfo() (*struct {
			Symbols []struct {
				Symbol               string
				Status               string
				BaseAsset            string
				QuoteAsset           string
				IsSpotTradingAllowed bool
			}
		}, error)
	}

	binanceClient, ok := client.(BinanceClient)
	if !ok {
		errorResponse(c, http.StatusInternalServerError, "Failed to get Binance client")
		return
	}

	// Get exchange info
	exchangeInfo, err := binanceClient.GetExchangeInfo()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to fetch symbols from Binance: "+err.Error())
		return
	}

	// Filter to only active USDT pairs
	var symbols []string
	for _, s := range exchangeInfo.Symbols {
		if s.Status == "TRADING" && s.QuoteAsset == "USDT" && s.IsSpotTradingAllowed {
			symbols = append(symbols, s.Symbol)
		}
	}

	successResponse(c, gin.H{
		"symbols": symbols,
		"count":   len(symbols),
	})
}
