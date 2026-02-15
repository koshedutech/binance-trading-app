package api

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/orders"

	"github.com/gin-gonic/gin"
)

// ==================== INPUT VALIDATION HELPERS ====================
// Commercial-grade validation for trading API inputs

var (
	// symbolRegex validates trading pair format (alphanumeric, 2-20 chars, typically ends in USDT/BUSD)
	symbolRegex = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)

	// orderChainsCache caches the response from handleGetOrderChainsWithState to reduce Binance REST API calls.
	// Key: userID + ":" + statusFilter, Value: cached JSON response
	orderChainsCacheMu   sync.Mutex
	orderChainsCacheData = make(map[string]orderChainsCacheEntry)
)

type orderChainsCacheEntry struct {
	data      interface{}
	timestamp time.Time
}

func init() {
	events.RegisterChainLifecycleHook(InvalidateOrderChainsCacheForUser)
}

// InvalidateOrderChainsCacheForUser removes all cached order chain responses for a specific user.
// Called when chain lifecycle events occur (SL/TP fill, chain close) to prevent stale data.
func InvalidateOrderChainsCacheForUser(userID string) {
	orderChainsCacheMu.Lock()
	for key := range orderChainsCacheData {
		if strings.HasPrefix(key, userID+":") {
			delete(orderChainsCacheData, key)
		}
	}
	orderChainsCacheMu.Unlock()
}

// validateSymbol validates a trading symbol for security and format
func validateSymbol(symbol string) (string, error) {
	// Normalize to uppercase
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	// Check format
	if !symbolRegex.MatchString(symbol) {
		return "", &ValidationError{Field: "symbol", Message: "invalid symbol format"}
	}

	return symbol, nil
}

// validateLeverage validates leverage value
func validateLeverage(leverage int) error {
	if leverage < 1 || leverage > 125 {
		return &ValidationError{Field: "leverage", Message: "leverage must be between 1 and 125"}
	}
	return nil
}

// validateQuantity validates order quantity
func validateQuantity(quantity float64) error {
	if quantity <= 0 {
		return &ValidationError{Field: "quantity", Message: "quantity must be positive"}
	}
	if quantity > 1000000 { // Reasonable upper limit
		return &ValidationError{Field: "quantity", Message: "quantity exceeds maximum"}
	}
	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// ==================== END INPUT VALIDATION HELPERS ====================

// FuturesAPI interface defines methods for futures trading
type FuturesAPI interface {
	GetFuturesClient() binance.FuturesClient
}

// ==================== REQUEST TYPES ====================

// PlaceFuturesOrderRequest represents a request to place a futures order
type PlaceFuturesOrderRequest struct {
	Symbol       string  `json:"symbol" binding:"required"`
	Side         string  `json:"side" binding:"required,oneof=BUY SELL"`
	PositionSide string  `json:"position_side" binding:"required,oneof=LONG SHORT BOTH"`
	OrderType    string  `json:"order_type" binding:"required"`
	Quantity     float64 `json:"quantity" binding:"required,gt=0"`
	Price        float64 `json:"price"`
	StopPrice    float64 `json:"stop_price"`
	TimeInForce  string  `json:"time_in_force"`
	ReduceOnly   bool    `json:"reduce_only"`
	ClosePosition bool   `json:"close_position"`
	TakeProfit   float64 `json:"take_profit"`
	StopLoss     float64 `json:"stop_loss"`
	WorkingType  string  `json:"working_type"`
}

// SetLeverageRequest represents a request to set leverage
type SetLeverageRequest struct {
	Symbol   string `json:"symbol" binding:"required"`
	Leverage int    `json:"leverage" binding:"required,min=1,max=125"`
}

// SetMarginTypeRequest represents a request to set margin type
type SetMarginTypeRequest struct {
	Symbol     string `json:"symbol" binding:"required"`
	MarginType string `json:"margin_type" binding:"required,oneof=CROSSED ISOLATED"`
}

// SetPositionModeRequest represents a request to set position mode
type SetPositionModeRequest struct {
	DualSidePosition bool `json:"dual_side_position"`
}

// ==================== HANDLER FUNCTIONS ====================

// handleGetFuturesAccountInfo returns futures account information
func (s *Server) handleGetFuturesAccountInfo(c *gin.Context) {
	// Get user ID from auth context
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// Check if we're in dry run mode - use per-user mode if authenticated
	isSimulated := false
	if userID != "" {
		// Get per-user trading mode from database
		dryRun, err := s.repo.GetUserDryRunMode(ctx, userID)
		if err != nil {
			log.Printf("[FUTURES-ACCOUNT] Error getting user dry run mode for %s: %v, defaulting to paper", userID, err)
			dryRun = true
		}
		isSimulated = dryRun
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		// If in LIVE mode but no client, return clear error
		if !isSimulated {
			log.Printf("[FUTURES-ACCOUNT] User %s in LIVE mode but no client - API key configuration needed", userID)
			c.JSON(http.StatusOK, gin.H{
				"total_wallet_balance":              0.0,
				"total_unrealized_profit":           0.0,
				"total_margin_balance":              0.0,
				"total_position_initial_margin":     0.0,
				"total_open_order_initial_margin":   0.0,
				"total_cross_wallet_balance":        0.0,
				"total_cross_unrealized_pnl":        0.0,
				"available_balance":                 0.0,
				"max_withdraw_amount":               0.0,
				"assets":                            []interface{}{},
				"positions":                         []interface{}{},
				"can_trade":                         false,
				"can_deposit":                       false,
				"can_withdraw":                      false,
				"is_simulated":                      false,
				"error":                             "api_keys_required",
				"message":                           "Please configure your Binance API keys in Settings to access live trading",
			})
			return
		}
		// Return mock account info if in paper trading mode
		// Get paper balance from database
		paperBalance, _, err := s.repo.GetUserPaperBalance(ctx, userID)
		if err != nil {
			log.Printf("[FUTURES-ACCOUNT] Error getting paper balance for %s: %v, using default", userID, err)
			paperBalance = 10000.0 // fallback default
		}
		if paperBalance == 0 {
			paperBalance = 10000.0 // fallback for zero balance
		}
		availableBalance := paperBalance * 0.95 // 5% margin buffer

		c.JSON(http.StatusOK, gin.H{
			"total_wallet_balance":              paperBalance,
			"total_unrealized_profit":           0.0,
			"total_margin_balance":              paperBalance,
			"total_position_initial_margin":     0.0,
			"total_open_order_initial_margin":   0.0,
			"total_cross_wallet_balance":        paperBalance,
			"total_cross_unrealized_pnl":        0.0,
			"available_balance":                 availableBalance,
			"max_withdraw_amount":               availableBalance,
			"assets":                            []interface{}{},
			"positions":                         []interface{}{},
			"can_trade":                         true,
			"can_deposit":                       true,
			"can_withdraw":                      true,
			"is_simulated":                      true,
		})
		return
	}

	accountInfo, err := futuresClient.GetFuturesAccountInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build response with is_simulated field
	c.JSON(http.StatusOK, gin.H{
		"total_wallet_balance":      accountInfo.TotalWalletBalance,
		"total_unrealized_profit":   accountInfo.TotalUnrealizedProfit,
		"total_margin_balance":      accountInfo.TotalMarginBalance,
		"total_position_initial_margin": accountInfo.TotalPositionInitialMargin,
		"total_open_order_initial_margin": accountInfo.TotalOpenOrderInitialMargin,
		"total_cross_wallet_balance": accountInfo.TotalCrossWalletBalance,
		"total_cross_unrealized_pnl": accountInfo.TotalCrossUnPnl,
		"available_balance":         accountInfo.AvailableBalance,
		"max_withdraw_amount":       accountInfo.MaxWithdrawAmount,
		"assets":                    accountInfo.Assets,
		"positions":                 accountInfo.Positions,
		"can_trade":                 accountInfo.CanTrade,
		"can_deposit":               accountInfo.CanDeposit,
		"can_withdraw":              accountInfo.CanWithdraw,
		"update_time":               accountInfo.UpdateTime,
		"is_simulated":              isSimulated,
	})
}

// handleGetCommissionRate returns user's actual maker/taker fee rates from Binance
func (s *Server) handleGetCommissionRate(c *gin.Context) {
	symbol := c.DefaultQuery("symbol", "BTCUSDT")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled - API keys required"})
		return
	}

	rate, err := futuresClient.GetCommissionRate(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Calculate percentages for display
	makerPercent := rate.MakerCommissionRate * 100
	takerPercent := rate.TakerCommissionRate * 100

	c.JSON(http.StatusOK, gin.H{
		"symbol":                  rate.Symbol,
		"maker_commission_rate":   rate.MakerCommissionRate,
		"taker_commission_rate":   rate.TakerCommissionRate,
		"maker_percent":           makerPercent,
		"taker_percent":           takerPercent,
		"description":             fmt.Sprintf("Maker: %.4f%% | Taker: %.4f%%", makerPercent, takerPercent),
	})
}

// handleGetFuturesPositions returns all futures positions
func (s *Server) handleGetFuturesPositions(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	positions, err := futuresClient.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter out empty positions
	activePositions := make([]binance.FuturesPosition, 0)
	for _, pos := range positions {
		if pos.PositionAmt != 0 {
			activePositions = append(activePositions, pos)
		}
	}

	// Enrich positions with custom ROI data from Ginie Autopilot and Settings
	enrichedPositions := make([]map[string]interface{}, len(activePositions))

	customROIMap := make(map[string]interface{}) // Can be *float64 (position-level) or float64 (symbol-level)

	// First, try to get position-level custom ROI from Ginie Autopilot
	if controller := s.getFuturesAutopilot(); controller != nil {
		if autopilot := controller.GetGinieAutopilot(); autopilot != nil {
			giniePositions := autopilot.GetPositions()
			for _, gPos := range giniePositions {
				if gPos.CustomROIPercent != nil {
					customROIMap[gPos.Symbol] = gPos.CustomROIPercent
				}
			}
		}
	}

	// Story 9.12: Get symbol-level custom ROI from cache service (replaces GetSettingsManager() singleton)
	// Use cache-first approach for symbol settings instead of loading full settings from DB
	userID := s.getUserID(c)
	if userID != "" && s.settingsCacheService != nil {
		symbolSettings, loadErr := s.settingsCacheService.GetAllSymbolSettings(c.Request.Context(), userID)
		if loadErr != nil {
			// Log but don't fail - this is optional enrichment
			log.Printf("[FUTURES-POS] WARNING: Failed to load symbol settings for custom ROI enrichment: %v", loadErr)
		} else {
			for symbol, settings := range symbolSettings {
				// Only add symbol-level ROI if we don't already have position-level ROI
				if settings != nil && settings.CustomROIPercent > 0 {
					if _, exists := customROIMap[symbol]; !exists {
						customROIMap[symbol] = settings.CustomROIPercent
					}
				}
			}
		}
	}

	// Build response with enriched data
	for i, pos := range activePositions {
		enrichedPos := map[string]interface{}{
			"symbol":               pos.Symbol,
			"positionAmt":          pos.PositionAmt,
			"entryPrice":           pos.EntryPrice,
			"markPrice":            pos.MarkPrice,
			"unRealizedProfit":     pos.UnrealizedProfit,
			"liquidationPrice":     pos.LiquidationPrice,
			"leverage":             pos.Leverage,
			"maxNotionalValue":     pos.MaxNotionalValue,
			"marginType":           pos.MarginType,
			"positionSide":         pos.PositionSide,
			"notional":             pos.Notional,
			"isolatedWallet":       pos.IsolatedWallet,
			"isolatedMargin":       pos.IsolatedMargin,
			"isAutoAddMargin":      pos.IsAutoAddMargin,
			"updateTime":           pos.UpdateTime,
		}

		// Add custom ROI if present (either position-level *float64 or symbol-level float64)
		if customROI, exists := customROIMap[pos.Symbol]; exists {
			enrichedPos["custom_roi_percent"] = customROI
		}

		enrichedPositions[i] = enrichedPos
	}

	c.JSON(http.StatusOK, enrichedPositions)
}

// handleSetLeverage sets leverage for a symbol
func (s *Server) handleSetLeverage(c *gin.Context) {
	var req SetLeverageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Additional security validation
	validatedSymbol, err := validateSymbol(req.Symbol)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Symbol = validatedSymbol

	if err := validateLeverage(req.Leverage); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	resp, err := futuresClient.SetLeverage(req.Symbol, req.Leverage)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to database
	ctx := context.Background()
	settings := &database.FuturesAccountSettings{
		Symbol:   req.Symbol,
		Leverage: req.Leverage,
	}
	s.repo.GetDB().UpsertFuturesAccountSettings(ctx, settings)

	c.JSON(http.StatusOK, resp)
}

// handleSetMarginType sets margin type for a symbol
func (s *Server) handleSetMarginType(c *gin.Context) {
	var req SetMarginTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	marginType := binance.MarginType(req.MarginType)
	err := futuresClient.SetMarginType(req.Symbol, marginType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to database
	ctx := context.Background()
	settings, _ := s.repo.GetDB().GetFuturesAccountSettings(ctx, req.Symbol)
	settings.MarginType = req.MarginType
	s.repo.GetDB().UpsertFuturesAccountSettings(ctx, settings)

	c.JSON(http.StatusOK, gin.H{"message": "Margin type updated", "symbol": req.Symbol, "marginType": req.MarginType})
}

// handleSetPositionMode sets position mode (hedge/one-way)
func (s *Server) handleSetPositionMode(c *gin.Context) {
	var req SetPositionModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	err := futuresClient.SetPositionMode(req.DualSidePosition)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	mode := "ONE_WAY"
	if req.DualSidePosition {
		mode = "HEDGE"
	}

	c.JSON(http.StatusOK, gin.H{"message": "Position mode updated", "dualSidePosition": req.DualSidePosition, "mode": mode})
}

// handleGetPositionMode gets current position mode
func (s *Server) handleGetPositionMode(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	resp, err := futuresClient.GetPositionMode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handlePlaceFuturesOrder places a new futures order
func (s *Server) handlePlaceFuturesOrder(c *gin.Context) {
	var req PlaceFuturesOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Additional security validation for symbol
	validatedSymbol, err := validateSymbol(req.Symbol)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Symbol = validatedSymbol

	// Validate quantity
	if err := validateQuantity(req.Quantity); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Build order params
	params := binance.FuturesOrderParams{
		Symbol:        req.Symbol,
		Side:          req.Side,
		PositionSide:  binance.PositionSide(req.PositionSide),
		Type:          binance.FuturesOrderType(req.OrderType),
		Quantity:      req.Quantity,
		Price:         req.Price,
		StopPrice:     req.StopPrice,
		TimeInForce:   binance.TimeInForce(req.TimeInForce),
		ReduceOnly:    req.ReduceOnly,
		ClosePosition: req.ClosePosition,
	}

	if req.WorkingType != "" {
		params.WorkingType = binance.WorkingType(req.WorkingType)
	}

	// Place the main order
	orderResp, err := futuresClient.PlaceFuturesOrder(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Place TP/SL orders if specified using NEW Algo Order API (mandatory since 2025-12-09)
	var tpOrderResp, slOrderResp *binance.AlgoOrderResponse
	var tpError, slError string

	if req.TakeProfit > 0 {
		tpParams := binance.AlgoOrderParams{
			Symbol:       req.Symbol,
			Side:         getOppositeSide(req.Side),
			PositionSide: binance.PositionSide(req.PositionSide),
			Type:         binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:     req.Quantity,
			TriggerPrice: req.TakeProfit,
			WorkingType:  binance.WorkingTypeMarkPrice,
		}
		var err error
		tpOrderResp, err = futuresClient.PlaceAlgoOrder(tpParams)
		if err != nil {
			tpError = err.Error()
			log.Printf("Failed to place Take Profit order: %v", err)
		}
	}

	if req.StopLoss > 0 {
		slParams := binance.AlgoOrderParams{
			Symbol:       req.Symbol,
			Side:         getOppositeSide(req.Side),
			PositionSide: binance.PositionSide(req.PositionSide),
			Type:         binance.FuturesOrderTypeStopMarket,
			Quantity:     req.Quantity,
			TriggerPrice: req.StopLoss,
			WorkingType:  binance.WorkingTypeMarkPrice,
		}
		var err error
		slOrderResp, err = futuresClient.PlaceAlgoOrder(slParams)
		if err != nil {
			slError = err.Error()
			log.Printf("Failed to place Stop Loss order: %v", err)
		}
	}

	// Save trade to database
	ctx := context.Background()
	settings, _ := s.repo.GetDB().GetFuturesAccountSettings(ctx, req.Symbol)

	trade := &database.FuturesTrade{
		Symbol:       req.Symbol,
		PositionSide: req.PositionSide,
		Side:         req.Side,
		EntryPrice:   orderResp.AvgPrice,
		Quantity:     req.Quantity,
		Leverage:     settings.Leverage,
		MarginType:   settings.MarginType,
		Status:       "OPEN",
		EntryTime:    time.Now(),
		TradeSource:  "manual",
	}

	if req.StopLoss > 0 {
		trade.StopLoss = &req.StopLoss
	}
	if req.TakeProfit > 0 {
		trade.TakeProfit = &req.TakeProfit
	}

	s.repo.GetDB().CreateFuturesTrade(ctx, trade)

	response := gin.H{
		"order":      orderResp,
		"takeProfit": tpOrderResp,
		"stopLoss":   slOrderResp,
		"tradeId":    trade.ID,
	}

	// Include TP/SL errors in response if any
	if tpError != "" {
		response["takeProfitError"] = tpError
	}
	if slError != "" {
		response["stopLossError"] = slError
	}

	// Broadcast ORDER_UPDATE to WebSocket clients for instant UI refresh
	userID := s.getUserID(c)
	if userID != "" {
		events.BroadcastOrderUpdate(userID, map[string]interface{}{
			"action":   "placed",
			"orderId":  orderResp.OrderId,
			"symbol":   req.Symbol,
			"side":     req.Side,
			"type":     "regular",
			"quantity": req.Quantity,
		})
	}

	c.JSON(http.StatusOK, response)
}

// handleCancelFuturesOrder cancels a futures order
func (s *Server) handleCancelFuturesOrder(c *gin.Context) {
	symbol := c.Param("symbol")
	orderIdStr := c.Param("id")

	orderId, err := strconv.ParseInt(orderIdStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	err = futuresClient.CancelFuturesOrder(symbol, orderId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast ORDER_UPDATE to WebSocket clients for instant UI refresh
	userID := s.getUserID(c)
	if userID != "" {
		events.BroadcastOrderUpdate(userID, map[string]interface{}{
			"action":  "cancelled",
			"orderId": orderId,
			"symbol":  symbol,
			"type":    "regular",
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order canceled", "orderId": orderId})
}

// handleCancelAllFuturesOrders cancels all futures orders for a symbol
func (s *Server) handleCancelAllFuturesOrders(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	err := futuresClient.CancelAllFuturesOrders(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast ORDER_UPDATE to WebSocket clients for instant UI refresh
	userID := s.getUserID(c)
	if userID != "" {
		events.BroadcastOrderUpdate(userID, map[string]interface{}{
			"action": "cancelled_all",
			"symbol": symbol,
			"type":   "regular",
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "All orders canceled", "symbol": symbol})
}

// handleGetFuturesOpenOrders returns open futures orders
func (s *Server) handleGetFuturesOpenOrders(c *gin.Context) {
	symbol := c.Query("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	orders, err := futuresClient.GetOpenOrders(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// handleGetAllFuturesOrders returns all open orders (regular + conditional/algo)
func (s *Server) handleGetAllFuturesOrders(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Get regular open orders
	regularOrders, err := futuresClient.GetOpenOrders("")
	if err != nil {
		regularOrders = []binance.FuturesOrder{}
	}

	// Get algo/conditional orders (TP/SL orders)
	algoOrders, err := futuresClient.GetOpenAlgoOrders("")
	if err != nil {
		algoOrders = []binance.AlgoOrder{}
	}

	// Format response
	c.JSON(http.StatusOK, gin.H{
		"regular_orders": regularOrders,
		"algo_orders":    algoOrders,
		"total_regular":  len(regularOrders),
		"total_algo":     len(algoOrders),
	})
}

// ==================== ORDER CHAINS WITH STATE ====================
// Story 7.14: Order Chain Backend Integration

// EfficiencyInfo contains efficiency metrics for position analytics
type EfficiencyInfo struct {
	PeakProfit        float64 `json:"peak_profit"`
	CurrentProfit     float64 `json:"current_profit"`
	EfficiencyPercent float64 `json:"efficiency_percent"`
	ThresholdPercent  float64 `json:"threshold_percent"`
}

// ClassicScoresInfo contains classic indicator scores
type ClassicScoresInfo struct {
	ADX              float64 `json:"adx"`
	ADXThreshold     float64 `json:"adx_threshold"`
	RSI              float64 `json:"rsi"`
	RSIState         string  `json:"rsi_state"` // "oversold", "normal", "overbought"
	ReversalSignals  int     `json:"reversal_signals"`
	ReversalRequired int     `json:"reversal_required"`
}

// NewEngineScoresInfo contains new engine indicator scores
type NewEngineScoresInfo struct {
	Technical float64 `json:"technical"`
	Context   float64 `json:"context"`
	LLM       float64 `json:"llm"`
	History   float64 `json:"history"`
	Final     float64 `json:"final"`
	Regime    string  `json:"regime"`
	Strategy  string  `json:"strategy"`
}

// PositionAnalyticsInfo contains position analytics for the chain view
// Story 11.40: Position Analytics Integration
type PositionAnalyticsInfo struct {
	Stage           string               `json:"stage"`                      // RISK_ZONE, BREAKEVEN, TP1, EFFICIENCY
	StageEntryTime  int64                `json:"stage_entry_time,omitempty"` // Unix ms
	CurrentPrice    float64              `json:"current_price"`
	BreakevenPrice  *float64             `json:"breakeven_price,omitempty"`
	TP1Price        *float64             `json:"tp1_price,omitempty"`
	TP2Price        *float64             `json:"tp2_price,omitempty"`
	TP3Price        *float64             `json:"tp3_price,omitempty"`
	StopLoss        *float64             `json:"stop_loss,omitempty"`
	Efficiency      *EfficiencyInfo      `json:"efficiency,omitempty"`
	DecisionMode    string               `json:"decision_mode"` // classic, new_engine
	ClassicScores   *ClassicScoresInfo   `json:"classic_scores,omitempty"`
	NewEngineScores *NewEngineScoresInfo `json:"new_engine_scores,omitempty"`
	UnrealizedPnL   float64              `json:"unrealized_pnl"`
	ROE             float64              `json:"roe"`
}

// OrderChainWithState represents an order chain with associated position state and modification counts
type OrderChainWithState struct {
	ChainID            string                 `json:"chain_id"`
	ModeCode           string                 `json:"mode_code"`
	Symbol             string                 `json:"symbol"`
	PositionSide       string                 `json:"position_side"`
	Orders             []ChainOrderInfo       `json:"orders"`
	PositionState      *PositionStateInfo     `json:"position_state,omitempty"`
	PositionAnalytics  *PositionAnalyticsInfo `json:"position_analytics,omitempty"` // Story 11.40
	ModificationCounts map[string]int         `json:"modification_counts"`
	Status             string                 `json:"status"` // active, partial, closed
	TotalValue         float64                `json:"total_value"`
	FilledValue        float64                `json:"filled_value"`
	CreatedAt          int64                  `json:"created_at"`
	UpdatedAt          int64                  `json:"updated_at"`
	// Strategy identification fields
	Mode          string `json:"mode,omitempty"`           // Full mode name: scalp, swing, position, ultra_fast
	StrategyGroup string `json:"strategy_group,omitempty"` // e.g., breakout, trending
	SubStrategy   string `json:"sub_strategy,omitempty"`   // e.g., ravindra_volume_imbalance
	Timeframe     string `json:"timeframe,omitempty"`      // e.g., 3m, 5m, 15m from pattern detection
	// Trailing stop status from RavindraPositionMonitor
	TrailingStopStatus interface{} `json:"trailing_stop_status,omitempty"`
	// SL modification history from chain events (SL_PLACED + SL_MODIFIED)
	SLModifications []SLModificationInfo `json:"sl_modifications,omitempty"`
}

// SLModificationInfo represents a single SL modification event for API response
type SLModificationInfo struct {
	Sequence       int      `json:"sequence"`
	OldPrice       *float64 `json:"old_price,omitempty"`
	NewPrice       float64  `json:"new_price"`
	Reason         string   `json:"reason,omitempty"`
	Source         string   `json:"source,omitempty"`
	BinanceOrderID *int64   `json:"binance_order_id,omitempty"`
	Timestamp      string   `json:"timestamp"` // ISO 8601
}

// ChainOrderInfo represents order info within a chain
type ChainOrderInfo struct {
	OrderID       int64   `json:"order_id"`
	ClientOrderID string  `json:"client_order_id"`
	OrderType     string  `json:"order_type"` // E, SL, TP1, TP2, etc.
	Symbol        string  `json:"symbol"`
	Side          string  `json:"side"`
	Type          string  `json:"type"` // MARKET, LIMIT, STOP_MARKET
	Status        string  `json:"status"`
	Price         float64 `json:"price"`
	StopPrice     float64 `json:"stop_price,omitempty"`
	Quantity      float64 `json:"quantity"`
	ExecutedQty   float64 `json:"executed_qty"`
	AvgPrice      float64 `json:"avg_price,omitempty"`
	Time          int64   `json:"time"`
	UpdateTime    int64   `json:"update_time"`
	IsAlgo        bool    `json:"is_algo"` // True if this is an algo/conditional order
}

// PositionStateInfo represents position state info for API response
type PositionStateInfo struct {
	ID                 int64   `json:"id"`
	ChainID            string  `json:"chain_id"`
	Symbol             string  `json:"symbol"`
	EntryOrderID       int64   `json:"entry_order_id"`
	EntryClientOrderID string  `json:"entry_client_order_id"`
	EntrySide          string  `json:"entry_side"` // BUY or SELL
	EntryPrice         float64 `json:"entry_price"`
	EntryQuantity      float64 `json:"entry_quantity"`
	EntryValue         float64 `json:"entry_value"`
	EntryFees          float64 `json:"entry_fees"`
	EntryFilledAt      string  `json:"entry_filled_at"` // ISO 8601
	Status             string  `json:"status"`          // ACTIVE, PARTIAL, CLOSED
	RemainingQuantity  float64 `json:"remaining_quantity"`
	RealizedPnL        float64 `json:"realized_pnl"`
	CreatedAt          string  `json:"created_at"` // ISO 8601
	UpdatedAt          string  `json:"updated_at"` // ISO 8601
	ClosedAt           string  `json:"closed_at,omitempty"` // ISO 8601
	ClosePrice         float64 `json:"close_price,omitempty"`
	CloseReason        string  `json:"close_reason,omitempty"` // SL_HIT, TP_HIT, MANUAL
}

// handleGetOrderChainsWithState returns order chains with position states and modification counts
// Story 7.14: Order Chain Backend Integration
// GET /api/futures/order-chains
// NOTE: This endpoint now also performs automatic stale order reconciliation
func (s *Server) handleGetOrderChainsWithState(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Parse query filters
	symbolFilter := c.Query("symbol")
	modeFilter := c.Query("mode")
	statusFilter := c.Query("status") // active, partial, closed

	// Handler-level response cache (5s to reduce DB load)
	cacheKey := userID + ":" + statusFilter + ":" + symbolFilter + ":" + modeFilter
	orderChainsCacheMu.Lock()
	if entry, ok := orderChainsCacheData[cacheKey]; ok && time.Since(entry.timestamp) < 5*time.Second {
		orderChainsCacheMu.Unlock()
		c.JSON(http.StatusOK, entry.data)
		return
	}
	orderChainsCacheMu.Unlock()

	// DB-ONLY chain building (no Binance REST API calls)
	// Chain status is authoritative from DB, updated by PositionLifecycleCoordinator via WebSocket.
	// Live position data (unrealized PnL, mark price) comes from WebSocket cache via getPositionAnalyticsForChain.
	chains := make(map[string]*OrderChainWithState)
	chainIDs := make([]string, 0)

	// Fetch active order chains from DB (PENDING, ENTRY_PLACED, ACTIVE, PARTIAL)
	activeOrderChains, err := s.repo.GetDB().GetActiveOrderChains(ctx, userID)
	if err != nil {
		log.Printf("[ORDER-CHAINS] Error fetching active order chains: %v", err)
		activeOrderChains = []*orders.OrderChain{}
	}

	// Build dbOrderChainsMap from active chains
	dbOrderChainsMap := make(map[string]*orders.OrderChain)
	for _, dbChain := range activeOrderChains {
		if dbChain != nil {
			dbOrderChainsMap[dbChain.ChainID] = dbChain
		}
	}

	// Build chain objects from DB active chains
	for _, dbChain := range activeOrderChains {
		if dbChain == nil {
			continue
		}
		chainID := dbChain.ChainID
		if chains[chainID] == nil {
			// Apply mode filter
			if modeFilter != "" {
				modeCode := extractModeCodeFromChainID(chainID)
				if modeCode != strings.ToUpper(modeFilter) {
					continue
				}
			}
			// Apply symbol filter
			if symbolFilter != "" && dbChain.Symbol != symbolFilter {
				continue
			}

			// Determine position side from DB side field
			// order_chains.side stores "LONG" or "SHORT" directly (not BUY/SELL)
			positionSide := "LONG"
			if strings.ToUpper(dbChain.Side) == "SHORT" || strings.ToUpper(dbChain.Side) == "SELL" {
				positionSide = "SHORT"
			}

			// Create chain entry from database order_chains table
			// Map PENDING/ENTRY_PLACED to "active" for frontend compatibility
			chainStatus := strings.ToLower(string(dbChain.Status))
			if chainStatus == "pending" || chainStatus == "entry_placed" {
				chainStatus = "active"
			}
			chains[chainID] = &OrderChainWithState{
				ChainID:            chainID,
				ModeCode:           dbChain.ModeCode,
				Symbol:             dbChain.Symbol,
				PositionSide:       positionSide,
				Orders:             []ChainOrderInfo{},
				ModificationCounts: make(map[string]int),
				Status:             chainStatus,
				CreatedAt:          dbChain.CreatedAt.UnixMilli(),
				UpdatedAt:          dbChain.UpdatedAt.UnixMilli(),
				Mode:               dbChain.Mode,
				StrategyGroup:      dbChain.StrategyGroup,
				SubStrategy:        dbChain.SubStrategy,
				Timeframe:          dbChain.Timeframe,
			}
			chainIDs = append(chainIDs, chainID)
		}
	}

	// 4a-2. Enrich ALL chains with entry data from DB (including chains created from Binance TP/SL orders)
	// This fixes the issue where TP/SL orders exist on Binance but entry order is not shown
	for chainID, chain := range chains {
		// Check if chain already has an entry order
		hasEntry := false
		for _, order := range chain.Orders {
			if order.OrderType == "E" {
				hasEntry = true
				break
			}
		}

		// If no entry order, try to get it from DB order_chains
		if !hasEntry {
			if dbChain, exists := dbOrderChainsMap[chainID]; exists && dbChain != nil {
				if dbChain.EntryPrice != nil && *dbChain.EntryPrice > 0 {
					var entryOrderID int64
					if dbChain.EntryBinanceOrderID != nil {
						entryOrderID = *dbChain.EntryBinanceOrderID
					}
					var entryPrice, entryQty float64
					if dbChain.EntryPrice != nil {
						entryPrice = *dbChain.EntryPrice
					}
					if dbChain.EntryQuantity != nil {
						entryQty = *dbChain.EntryQuantity
					}

					// Convert position side (LONG/SHORT) to entry side (BUY/SELL)
					entrySide := "BUY"
					if strings.ToUpper(dbChain.Side) == "SHORT" || strings.ToUpper(dbChain.Side) == "SELL" {
						entrySide = "SELL"
					}

					// Determine entry status and time based on whether the entry has filled
					entryStatus := "NEW" // Default: pending entry order
					var entryTime int64
					var avgPrice float64
					var executedQty float64
					if dbChain.EntryFilledAt != nil {
						// Entry has filled
						entryStatus = "FILLED"
						entryTime = dbChain.EntryFilledAt.UnixMilli()
						avgPrice = entryPrice
						executedQty = entryQty
					} else {
						// Entry is still pending (ENTRY_PLACED status)
						entryTime = dbChain.CreatedAt.UnixMilli()
					}

					entryOrder := ChainOrderInfo{
						OrderID:       entryOrderID,
						ClientOrderID: chainID + "-E",
						OrderType:     "E",
						Symbol:        dbChain.Symbol,
						Side:          entrySide,
						Type:          "LIMIT",
						Status:        entryStatus,
						Price:         entryPrice,
						AvgPrice:      avgPrice,
						Quantity:      entryQty,
						ExecutedQty:   executedQty,
						Time:          entryTime,
						UpdateTime:    entryTime,
						IsAlgo:        false,
					}
					chain.Orders = append([]ChainOrderInfo{entryOrder}, chain.Orders...)

					log.Printf("[ORDER-CHAINS] Added entry order from DB for chain %s: price=%.4f, qty=%.4f, status=%s",
						chainID, entryPrice, entryQty, entryStatus)
				}
			}
		}

		// Enrich chain with strategy information from DB (even if entry order already exists)
		if dbChain, exists := dbOrderChainsMap[chainID]; exists && dbChain != nil {
			// Only set if not already set (preserve any values already populated)
			if chain.Mode == "" && dbChain.Mode != "" {
				chain.Mode = dbChain.Mode
			}
			if chain.StrategyGroup == "" && dbChain.StrategyGroup != "" {
				chain.StrategyGroup = dbChain.StrategyGroup
			}
			if chain.SubStrategy == "" && dbChain.SubStrategy != "" {
				chain.SubStrategy = dbChain.SubStrategy
			}
			if chain.Timeframe == "" && dbChain.Timeframe != "" {
				chain.Timeframe = dbChain.Timeframe
			}

			// Reconstruct SL/TP orders from persisted DB columns if not already present
			// This handles active chains from DB that have no open algo orders on Binance
			hasSL := false
			hasTP := false
			for _, order := range chain.Orders {
				if order.OrderType == "SL" || strings.HasPrefix(order.OrderType, "SL") {
					hasSL = true
				}
				if order.OrderType == "TP" || strings.HasPrefix(order.OrderType, "TP") {
					hasTP = true
				}
			}

			positionSide := chain.PositionSide
			closeSide := "SELL"
			if positionSide == "SHORT" {
				closeSide = "BUY"
			}

			if !hasSL && dbChain.SLLimitPrice != nil && *dbChain.SLLimitPrice > 0 {
				slStatus := "NEW"
				if dbChain.SLStatus != nil {
					slStatus = *dbChain.SLStatus
				}
				var slOrderID int64
				if dbChain.SLBinanceOrderID != nil {
					slOrderID = *dbChain.SLBinanceOrderID
				}
				var slQty float64
				if dbChain.SLQuantity != nil {
					slQty = *dbChain.SLQuantity
				}
				slPrice := *dbChain.SLLimitPrice
				var slAvgPrice, slExecQty float64
				if dbChain.SLFillPrice != nil {
					slAvgPrice = *dbChain.SLFillPrice
				}
				if slStatus == "FILLED" {
					slExecQty = slQty
				}

				chain.Orders = append(chain.Orders, ChainOrderInfo{
					OrderID:       slOrderID,
					ClientOrderID: chainID + "-SL",
					OrderType:     "SL",
					Symbol:        dbChain.Symbol,
					Side:          closeSide,
					Type:          "STOP",
					Status:        slStatus,
					Price:         slPrice,
					StopPrice:     slPrice,
					Quantity:      slQty,
					ExecutedQty:   slExecQty,
					AvgPrice:      slAvgPrice,
					Time:          dbChain.CreatedAt.UnixMilli(),
					UpdateTime:    dbChain.UpdatedAt.UnixMilli(),
					IsAlgo:        true,
				})
			}

			if !hasTP && dbChain.TPLimitPrice != nil && *dbChain.TPLimitPrice > 0 {
				tpStatus := "NEW"
				if dbChain.TPStatus != nil {
					tpStatus = *dbChain.TPStatus
				}
				var tpOrderID int64
				if dbChain.TPBinanceOrderID != nil {
					tpOrderID = *dbChain.TPBinanceOrderID
				}
				var tpQty float64
				if dbChain.TPQuantity != nil {
					tpQty = *dbChain.TPQuantity
				}
				tpPrice := *dbChain.TPLimitPrice
				var tpAvgPrice, tpExecQty float64
				if dbChain.TPFillPrice != nil {
					tpAvgPrice = *dbChain.TPFillPrice
				}
				if tpStatus == "FILLED" {
					tpExecQty = tpQty
				}

				chain.Orders = append(chain.Orders, ChainOrderInfo{
					OrderID:       tpOrderID,
					ClientOrderID: chainID + "-TP",
					OrderType:     "TP",
					Symbol:        dbChain.Symbol,
					Side:          closeSide,
					Type:          "TAKE_PROFIT",
					Status:        tpStatus,
					Price:         tpPrice,
					StopPrice:     tpPrice,
					Quantity:      tpQty,
					ExecutedQty:   tpExecQty,
					AvgPrice:      tpAvgPrice,
					Time:          dbChain.CreatedAt.UnixMilli(),
					UpdateTime:    dbChain.UpdatedAt.UnixMilli(),
					IsAlgo:        true,
				})
			}
		}
	}

	// 4b. Fetch position states for all chain IDs (using UUID string directly)
	positionStates, err := s.repo.GetDB().GetPositionStatesByChainIDs(ctx, userID, chainIDs)
	if err != nil {
		log.Printf("[ORDER-CHAINS] Error fetching position states: %v", err)
		positionStates = make(map[string]*orders.PositionState)
	}

	// 5. Fetch modification counts for all chain IDs (with user_id filter for security)
	modCounts, err := s.repo.GetDB().GetModificationCountsByChainIDs(ctx, userID, chainIDs)
	if err != nil {
		log.Printf("[ORDER-CHAINS] Error fetching modification counts: %v", err)
		modCounts = make(map[string]map[string]int)
	}

	// 6. Merge position states and modification counts into chains
	for chainID, chain := range chains {
		// Calculate TotalValue and FilledValue from orders first
		// Use stopPrice for SL/TP orders (algo orders), price for regular orders
		for _, order := range chain.Orders {
			// Determine the effective price for value calculation
			effectivePrice := order.Price
			if order.StopPrice > 0 {
				effectivePrice = order.StopPrice
			}
			if effectivePrice > 0 {
				chain.TotalValue += order.Quantity * effectivePrice
			}
			// Filled value uses avgPrice if available, otherwise effective price
			if order.ExecutedQty > 0 {
				filledPrice := order.AvgPrice
				if filledPrice <= 0 {
					filledPrice = effectivePrice
				}
				chain.FilledValue += order.ExecutedQty * filledPrice
			}
		}

		// Add position state (overwrites calculated values with more accurate entry data)
		if posState, exists := positionStates[chainID]; exists && posState != nil {
			chain.PositionState = &PositionStateInfo{
				ID:                 posState.ID,
				ChainID:            posState.ChainID,
				Symbol:             posState.Symbol,
				EntryOrderID:       posState.EntryOrderID,
				EntryClientOrderID: posState.EntryClientOrderID,
				EntrySide:          posState.EntrySide,
				EntryPrice:         posState.EntryPrice,
				EntryQuantity:      posState.EntryQuantity,
				EntryValue:         posState.EntryValue,
				EntryFees:          posState.EntryFees,
				EntryFilledAt:      posState.EntryFilledAt.Format(time.RFC3339),
				Status:             posState.Status,
				RemainingQuantity:  posState.RemainingQuantity,
				RealizedPnL:        posState.RealizedPnL,
				CreatedAt:          posState.CreatedAt.Format(time.RFC3339),
				UpdatedAt:          posState.UpdatedAt.Format(time.RFC3339),
			}
			if posState.ClosedAt != nil {
				chain.PositionState.ClosedAt = posState.ClosedAt.Format(time.RFC3339)
			}

			// Chain status comes from DB order_chains.status (set by PositionLifecycleCoordinator)
			// Do NOT override with position_states.status — it can be stale and cause race conditions

			// Use position state entry value for more accurate total/filled values
			// Entry value is calculated at fill time with actual prices
			if posState.EntryValue > 0 {
				chain.FilledValue = posState.EntryValue
				// TotalValue includes entry + pending SL/TP orders
				if chain.TotalValue < posState.EntryValue {
					chain.TotalValue = posState.EntryValue
				}
			}

			// Update chain UpdatedAt if position state was updated more recently
			posUpdateTime := posState.UpdatedAt.UnixMilli()
			if posUpdateTime > chain.UpdatedAt {
				chain.UpdatedAt = posUpdateTime
			}
		}

		// Add modification counts
		if counts, exists := modCounts[chainID]; exists {
			chain.ModificationCounts = counts
		}

		// 6a. BUILD SYNTHETIC POSITION STATE from DB order_chains data
		// When DB has no position_state record, build from order_chains entry data
		// Status comes from DB chain status (set by PositionLifecycleCoordinator)
		if chain.PositionState == nil {
			if dbChain, hasDBChain := dbOrderChainsMap[chainID]; hasDBChain && dbChain != nil {
				if dbChain.EntryFilledAt != nil && dbChain.EntryPrice != nil && *dbChain.EntryPrice > 0 {
					var entryPrice, entryQty float64
					if dbChain.EntryPrice != nil {
						entryPrice = *dbChain.EntryPrice
					}
					if dbChain.EntryQuantity != nil {
						entryQty = *dbChain.EntryQuantity
					}
					entryValue := entryPrice * entryQty

					// Determine entry side
					entrySide := "BUY"
					if strings.ToUpper(dbChain.Side) == "SELL" || strings.ToUpper(dbChain.Side) == "SHORT" {
						entrySide = "SELL"
					}

					// Status from DB chain status (authoritative, set by coordinator)
					posStatus := strings.ToUpper(string(dbChain.Status))
					if posStatus == "ACTIVE" || posStatus == "PARTIAL" || posStatus == "ENTRY_PLACED" || posStatus == "PENDING" {
						posStatus = "ACTIVE"
					} else {
						posStatus = "CLOSED"
					}

					chain.PositionState = &PositionStateInfo{
						ID:                 0,
						ChainID:            chainID,
						Symbol:             chain.Symbol,
						EntryOrderID:       0,
						EntryClientOrderID: chainID + "-E",
						EntrySide:          entrySide,
						EntryPrice:         entryPrice,
						EntryQuantity:      entryQty,
						EntryValue:         entryValue,
						EntryFees:          0,
						EntryFilledAt:      dbChain.EntryFilledAt.Format(time.RFC3339),
						Status:             posStatus,
						RemainingQuantity:  entryQty,
						RealizedPnL:        0,
						CreatedAt:          dbChain.CreatedAt.Format(time.RFC3339),
						UpdatedAt:          dbChain.UpdatedAt.Format(time.RFC3339),
					}

					chain.FilledValue = entryValue
					if chain.TotalValue < entryValue {
						chain.TotalValue = entryValue
					}

					log.Printf("[ORDER-CHAINS] Built synthetic positionState for chain %s from DB: qty=%.4f, entry=%.4f, status=%s",
						chainID, entryQty, entryPrice, posStatus)
				}
			}
		}

		// Story 11.40: Add position analytics for active positions (now works with synthetic positionState too)
		if chain.PositionState != nil && chain.Status == "active" {
			analytics := s.getPositionAnalyticsForChain(c, chain.Symbol)
			if analytics != nil {
				chain.PositionAnalytics = analytics
			}
		}
	}

	// 6b. INLINE STATUS VERIFICATION - DISABLED
	// PositionLifecycleCoordinator now handles chain closes via WebSocket events.
	// Inline verification was racing with WebSocket position cache updates:
	// position created → frontend fetchOrders() → inline check sees no cached position yet → falsely marks closed.
	// The coordinator provides authoritative close via WebSocket SL/TP fill events.
	// The 5-minute FuturesController sync loop remains as a safety net for missed events.
	// if apiDataReliable {
	// 	for chainID, chain := range chains { ... }
	// }

	// 6c. FETCH CLOSED CHAINS FROM DB when status filter requests them
	// Closed chains have no Binance orders, so they must be loaded from the database
	if statusFilter == "" || strings.EqualFold(statusFilter, "closed") {
		closedDBChains, err := s.repo.GetDB().GetOrderChainsWithFilter(ctx, database.OrderChainFilter{
			UserID:   userID,
			Status:   "closed",
			Symbol:   symbolFilter,
			ModeCode: modeFilter,
			Limit:    50,
		})
		if err != nil {
			log.Printf("[ORDER-CHAINS] Error fetching closed chains: %v", err)
		} else {
			for _, dbChain := range closedDBChains {
				if dbChain == nil {
					continue
				}
				chainID := dbChain.ChainID
				// Skip if already in the map (active chain that was inline-closed)
				if chains[chainID] != nil {
					continue
				}

				// Build chain entry from DB
				positionSide := dbChain.Side // Use order_chains.side directly (LONG/SHORT)

				chainEntry := &OrderChainWithState{
					ChainID:            chainID,
					ModeCode:           dbChain.ModeCode,
					Symbol:             dbChain.Symbol,
					PositionSide:       positionSide,
					Orders:             []ChainOrderInfo{},
					ModificationCounts: make(map[string]int),
					Status:             "closed",
					CreatedAt:          dbChain.CreatedAt.UnixMilli(),
					UpdatedAt:          dbChain.UpdatedAt.UnixMilli(),
					Mode:               dbChain.Mode,
					StrategyGroup:      dbChain.StrategyGroup,
					SubStrategy:        dbChain.SubStrategy,
					Timeframe:          dbChain.Timeframe,
				}

				// Reconstruct entry order from DB
				if dbChain.EntryFilledAt != nil && dbChain.EntryPrice != nil && *dbChain.EntryPrice > 0 {
					entryTime := dbChain.EntryFilledAt.UnixMilli()
					var entryOrderID int64
					if dbChain.EntryBinanceOrderID != nil {
						entryOrderID = *dbChain.EntryBinanceOrderID
					}
					var entryPrice, entryQty float64
					if dbChain.EntryPrice != nil {
						entryPrice = *dbChain.EntryPrice
					}
					if dbChain.EntryQuantity != nil {
						entryQty = *dbChain.EntryQuantity
					}

					// Determine entry side from direction
					entrySide := "BUY"
					if positionSide == "SHORT" {
						entrySide = "SELL"
					}

					chainEntry.Orders = append(chainEntry.Orders, ChainOrderInfo{
						OrderID:       entryOrderID,
						ClientOrderID: chainID + "-E",
						OrderType:     "E",
						Symbol:        dbChain.Symbol,
						Side:          entrySide,
						Type:          "LIMIT",
						Status:        "FILLED",
						Price:         entryPrice,
						AvgPrice:      entryPrice,
						Quantity:      entryQty,
						ExecutedQty:   entryQty,
						Time:          entryTime,
						UpdateTime:    entryTime,
						IsAlgo:        false,
					})
				}

				// Determine close reason for SL/TP status derivation
				closeReason := ""
				if dbChain.CloseReason != nil {
					closeReason = *dbChain.CloseReason
				}

				// Reconstruct SL order from persisted DB columns
				if dbChain.SLLimitPrice != nil && *dbChain.SLLimitPrice > 0 {
					slStatus := "NEW"
					if dbChain.SLStatus != nil {
						slStatus = *dbChain.SLStatus
					} else if closeReason == "SL_HIT" {
						slStatus = "FILLED"
					} else if closeReason == "TP_HIT" || closeReason == "MANUAL" {
						slStatus = "CANCELED"
					}
					var slOrderID int64
					if dbChain.SLBinanceOrderID != nil {
						slOrderID = *dbChain.SLBinanceOrderID
					}
					var slQty float64
					if dbChain.SLQuantity != nil {
						slQty = *dbChain.SLQuantity
					}
					// Determine close side (opposite of position)
					closeSide := "SELL"
					if positionSide == "SHORT" {
						closeSide = "BUY"
					}
					slPrice := *dbChain.SLLimitPrice
					var slAvgPrice float64
					var slExecQty float64
					var slTime int64
					if dbChain.SLFillPrice != nil {
						slAvgPrice = *dbChain.SLFillPrice
					}
					if slStatus == "FILLED" {
						slExecQty = slQty
					}
					if dbChain.SLFillTime != nil {
						slTime = dbChain.SLFillTime.UnixMilli()
					} else {
						slTime = dbChain.CreatedAt.UnixMilli()
					}

					chainEntry.Orders = append(chainEntry.Orders, ChainOrderInfo{
						OrderID:       slOrderID,
						ClientOrderID: chainID + "-SL",
						OrderType:     "SL",
						Symbol:        dbChain.Symbol,
						Side:          closeSide,
						Type:          "STOP",
						Status:        slStatus,
						Price:         slPrice,
						StopPrice:     slPrice,
						Quantity:      slQty,
						ExecutedQty:   slExecQty,
						AvgPrice:      slAvgPrice,
						Time:          slTime,
						UpdateTime:    slTime,
						IsAlgo:        true,
					})
				}

				// Reconstruct TP order from persisted DB columns
				if dbChain.TPLimitPrice != nil && *dbChain.TPLimitPrice > 0 {
					tpStatus := "NEW"
					if dbChain.TPStatus != nil {
						tpStatus = *dbChain.TPStatus
					} else if closeReason == "TP_HIT" {
						tpStatus = "FILLED"
					} else if closeReason == "SL_HIT" || closeReason == "MANUAL" {
						tpStatus = "CANCELED"
					}
					var tpOrderID int64
					if dbChain.TPBinanceOrderID != nil {
						tpOrderID = *dbChain.TPBinanceOrderID
					}
					var tpQty float64
					if dbChain.TPQuantity != nil {
						tpQty = *dbChain.TPQuantity
					}
					closeSide := "SELL"
					if positionSide == "SHORT" {
						closeSide = "BUY"
					}
					tpPrice := *dbChain.TPLimitPrice
					var tpAvgPrice float64
					var tpExecQty float64
					var tpTime int64
					if dbChain.TPFillPrice != nil {
						tpAvgPrice = *dbChain.TPFillPrice
					}
					if tpStatus == "FILLED" {
						tpExecQty = tpQty
					}
					if dbChain.TPFillTime != nil {
						tpTime = dbChain.TPFillTime.UnixMilli()
					} else {
						tpTime = dbChain.CreatedAt.UnixMilli()
					}

					chainEntry.Orders = append(chainEntry.Orders, ChainOrderInfo{
						OrderID:       tpOrderID,
						ClientOrderID: chainID + "-TP",
						OrderType:     "TP",
						Symbol:        dbChain.Symbol,
						Side:          closeSide,
						Type:          "TAKE_PROFIT",
						Status:        tpStatus,
						Price:         tpPrice,
						StopPrice:     tpPrice,
						Quantity:      tpQty,
						ExecutedQty:   tpExecQty,
						AvgPrice:      tpAvgPrice,
						Time:          tpTime,
						UpdateTime:    tpTime,
						IsAlgo:        true,
					})
				}

				// Build position state for closed chain
				if dbChain.EntryPrice != nil && *dbChain.EntryPrice > 0 {
					var entryPrice, entryQty float64
					if dbChain.EntryPrice != nil {
						entryPrice = *dbChain.EntryPrice
					}
					if dbChain.EntryQuantity != nil {
						entryQty = *dbChain.EntryQuantity
					}
					entrySide := "BUY"
					if positionSide == "SHORT" {
						entrySide = "SELL"
					}
					entryFilledAt := ""
					if dbChain.EntryFilledAt != nil {
						entryFilledAt = dbChain.EntryFilledAt.Format(time.RFC3339)
					}
					closedAt := ""
					if dbChain.ClosedAt != nil {
						closedAt = dbChain.ClosedAt.Format(time.RFC3339)
					}
					var realizedPnL float64
					if dbChain.RealizedPnL != nil {
						realizedPnL = *dbChain.RealizedPnL
					}

					var closePrice float64
					if dbChain.ClosePrice != nil {
						closePrice = *dbChain.ClosePrice
					}
					var closeReason string
					if dbChain.CloseReason != nil {
						closeReason = *dbChain.CloseReason
					}

					chainEntry.PositionState = &PositionStateInfo{
						ChainID:            chainID,
						Symbol:             dbChain.Symbol,
						EntryClientOrderID: chainID + "-E",
						EntrySide:          entrySide,
						EntryPrice:         entryPrice,
						EntryQuantity:      entryQty,
						EntryValue:         entryPrice * entryQty,
						EntryFilledAt:      entryFilledAt,
						Status:             "CLOSED",
						RemainingQuantity:  0,
						RealizedPnL:        realizedPnL,
						CreatedAt:          dbChain.CreatedAt.Format(time.RFC3339),
						UpdatedAt:          dbChain.UpdatedAt.Format(time.RFC3339),
						ClosedAt:           closedAt,
						ClosePrice:         closePrice,
						CloseReason:        closeReason,
					}

					chainEntry.FilledValue = entryPrice * entryQty
					chainEntry.TotalValue = entryPrice * entryQty
				}

				chains[chainID] = chainEntry
			}
		}
	}

	// 6d. Fix positionSide for ALL chains: always use order_chains.side (LONG/SHORT)
	for chainID, chain := range chains {
		if dbChain, exists := dbOrderChainsMap[chainID]; exists && dbChain != nil {
			// order_chains.side stores LONG or SHORT directly
			if dbChain.Side == "LONG" || dbChain.Side == "SHORT" {
				chain.PositionSide = dbChain.Side
			}
		}
	}

	// 7. Apply status filter
	if statusFilter != "" {
		filteredChains := make(map[string]*OrderChainWithState)
		for chainID, chain := range chains {
			if strings.EqualFold(chain.Status, statusFilter) {
				filteredChains[chainID] = chain
			}
		}
		chains = filteredChains
	}

	// 8. Enrich active chains with trailing stop status from RavindraPositionMonitor
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.RavindraPositionMonitor != nil {
			ravPositions := instance.RavindraPositionMonitor.GetAllPositions()
			for chainID, chain := range chains {
				if chain.Status == "active" {
					if ravPos, ok := ravPositions[chainID]; ok && ravPos.TrailingStop != nil {
						chain.TrailingStopStatus = ravPos.TrailingStop.GetStatus()
					}
				}
			}
		}
	}

	// 8b. Enrich chains with SL modification history from chain events
	// Fetch for any chain that has an SL order (initial SL_PLACED event should always be shown)
	slModChainIDs := make([]string, 0)
	for chainID, chain := range chains {
		hasSLOrder := false
		for _, order := range chain.Orders {
			if order.OrderType == "SL" {
				hasSLOrder = true
				break
			}
		}
		if hasSLOrder {
			slModChainIDs = append(slModChainIDs, chainID)
		}
	}
	if len(slModChainIDs) > 0 {
		slEvents, err := s.repo.GetDB().GetSLModificationEventsByChainIDs(ctx, slModChainIDs)
		if err != nil {
			log.Printf("[ORDER-CHAINS] Error fetching SL modification events: %v", err)
		} else {
			for chainID, events := range slEvents {
				if chain, ok := chains[chainID]; ok && len(events) > 0 {
					mods := make([]SLModificationInfo, 0, len(events))
					for _, evt := range events {
						mod := SLModificationInfo{
							Sequence:       evt.EventSequence,
							BinanceOrderID: evt.BinanceOrderID,
							Timestamp:      evt.CreatedAt.Format(time.RFC3339),
						}
						if evt.Price != nil {
							mod.NewPrice = *evt.Price
						}
						if evt.OldPrice != nil {
							mod.OldPrice = evt.OldPrice
						}
						if evt.ModificationReason != nil {
							mod.Reason = *evt.ModificationReason
						}
						if evt.ModificationSource != nil {
							mod.Source = string(*evt.ModificationSource)
						}
						mods = append(mods, mod)
					}
					chain.SLModifications = mods
				}
			}
		}
	}

	// 9. Convert map to slice for response
	result := make([]*OrderChainWithState, 0, len(chains))
	for _, chain := range chains {
		result = append(result, chain)
	}

	// Sort by creation date descending (newest first) for stable display order
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})

	responseData := gin.H{
		"chains":      result,
		"total":       len(result),
		"chain_count": len(result),
	}

	// Cache the response for 15s to reduce Binance REST API calls
	orderChainsCacheMu.Lock()
	orderChainsCacheData[cacheKey] = orderChainsCacheEntry{data: responseData, timestamp: time.Now()}
	orderChainsCacheMu.Unlock()

	c.JSON(http.StatusOK, responseData)
}

// determinePositionSide converts entry side (BUY/SELL) to position side (LONG/SHORT)
func determinePositionSide(entrySide string) string {
	if strings.ToUpper(entrySide) == "BUY" {
		return "LONG"
	}
	return "SHORT"
}

// parseChainIDFromClientOrderID extracts the chain ID from a client order ID
// Example: "SCA-06JAN-00001-TP1" -> "SCA-06JAN-00001"
func parseChainIDFromClientOrderID(clientOrderID string) string {
	if clientOrderID == "" {
		return ""
	}
	parts := strings.Split(clientOrderID, "-")
	if len(parts) < 4 {
		return ""
	}
	// Join first 3 parts for normal IDs: MODE-DATE-SEQ
	// For fallback IDs: MODE-FALLBACK-UUID (3 parts)
	if parts[1] == "FALLBACK" && len(parts) >= 3 {
		return strings.Join(parts[:3], "-")
	}
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "-")
	}
	return ""
}

// extractModeCodeFromChainID extracts the mode code from a chain ID
// Example: "SCA-06JAN-00001" -> "SCA"
func extractModeCodeFromChainID(chainID string) string {
	parts := strings.Split(chainID, "-")
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// extractOrderTypeFromClientOrderID extracts the order type from a client order ID
// Example: "SCA-06JAN-00001-TP1" -> "TP1"
func extractOrderTypeFromClientOrderID(clientOrderID string) string {
	if clientOrderID == "" {
		return ""
	}
	parts := strings.Split(clientOrderID, "-")
	if len(parts) >= 4 {
		return parts[len(parts)-1]
	}
	return ""
}

// getPositionAnalyticsForChain gets position analytics for a chain symbol
// Story 11.40: Position Analytics Integration
func (s *Server) getPositionAnalyticsForChain(c *gin.Context, symbol string) *PositionAnalyticsInfo {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return nil
	}

	if s.userAutopilotManager == nil {
		return nil
	}

	instance := s.userAutopilotManager.GetInstance(userID)
	if instance == nil || instance.Autopilot == nil {
		return nil
	}

	data := instance.Autopilot.GetPositionAnalytics(symbol)
	if data == nil {
		return nil
	}

	// Convert autopilot.PositionAnalyticsData to API response type PositionAnalyticsInfo
	analytics := &PositionAnalyticsInfo{
		Stage:         data.Stage,
		StageEntryTime: data.StageEntryTime,
		CurrentPrice:  data.CurrentPrice,
		DecisionMode:  data.DecisionMode,
		UnrealizedPnL: data.UnrealizedPnL,
		ROE:           data.ROE,
	}

	// Copy optional pointer fields
	if data.BreakevenPrice != nil {
		analytics.BreakevenPrice = data.BreakevenPrice
	}
	if data.TP1Price != nil {
		analytics.TP1Price = data.TP1Price
	}
	if data.TP2Price != nil {
		analytics.TP2Price = data.TP2Price
	}
	if data.TP3Price != nil {
		analytics.TP3Price = data.TP3Price
	}
	if data.StopLoss != nil {
		analytics.StopLoss = data.StopLoss
	}

	// Convert efficiency data
	if data.Efficiency != nil {
		analytics.Efficiency = &EfficiencyInfo{
			PeakProfit:        data.Efficiency.PeakProfit,
			CurrentProfit:     data.Efficiency.CurrentProfit,
			EfficiencyPercent: data.Efficiency.EfficiencyPercent,
			ThresholdPercent:  data.Efficiency.ThresholdPercent,
		}
	}

	// Convert classic scores
	if data.ClassicScores != nil {
		analytics.ClassicScores = &ClassicScoresInfo{
			ADX:              data.ClassicScores.ADX,
			ADXThreshold:     data.ClassicScores.ADXThreshold,
			RSI:              data.ClassicScores.RSI,
			RSIState:         data.ClassicScores.RSIState,
			ReversalSignals:  data.ClassicScores.ReversalSignals,
			ReversalRequired: data.ClassicScores.ReversalRequired,
		}
	}

	// Convert new engine scores
	if data.NewEngineScores != nil {
		analytics.NewEngineScores = &NewEngineScoresInfo{
			Technical: data.NewEngineScores.Technical,
			Context:   data.NewEngineScores.Context,
			LLM:       data.NewEngineScores.LLM,
			History:   data.NewEngineScores.History,
			Final:     data.NewEngineScores.Final,
			Regime:    data.NewEngineScores.Regime,
			Strategy:  data.NewEngineScores.Strategy,
		}
	}

	return analytics
}

// ==================== END ORDER CHAINS WITH STATE ====================

// handleCloseFuturesPosition closes a futures position
func (s *Server) handleCloseFuturesPosition(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Get the current position
	position, err := futuresClient.GetPositionBySymbol(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if position.PositionAmt == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No open position for this symbol"})
		return
	}

	// Determine close side and quantity
	var side string
	var quantity float64
	if position.PositionAmt > 0 {
		side = "SELL"
		quantity = position.PositionAmt
	} else {
		side = "BUY"
		quantity = -position.PositionAmt
	}

	// Place market order to close
	// In hedge mode (position side is LONG or SHORT), ReduceOnly is not required
	// The position side parameter tells the exchange which position to close
	params := binance.FuturesOrderParams{
		Symbol:       symbol,
		Side:         side,
		PositionSide: binance.PositionSide(position.PositionSide),
		Type:         binance.FuturesOrderTypeMarket,
		Quantity:     quantity,
		ReduceOnly:   position.PositionSide == "BOTH", // Only use ReduceOnly in one-way mode
	}

	orderResp, err := futuresClient.PlaceFuturesOrder(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Position closed",
		"symbol":   symbol,
		"order":    orderResp,
	})
}

// handleGetPositionOrders returns TP/SL orders for a position
// Includes both traditional orders and new Algo orders (since 2025-12-09)
func (s *Server) handleGetPositionOrders(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Get all open orders for this symbol (traditional API)
	orders, err := futuresClient.GetOpenOrders(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get all open algo orders for this symbol (new API since 2025-12-09)
	algoOrders, algoErr := futuresClient.GetOpenAlgoOrders(symbol)
	if algoErr != nil {
		// Silently continue - algo orders API may not be available
		algoOrders = nil
	}

	// Categorize orders
	var takeProfitOrders []interface{}
	var stopLossOrders []interface{}
	var trailingStopOrders []interface{}
	var otherOrders []interface{}

	// Process traditional orders (may still have some from before migration)
	for _, order := range orders {
		orderData := gin.H{
			"orderId":      order.OrderId,
			"symbol":       order.Symbol,
			"side":         order.Side,
			"positionSide": order.PositionSide,
			"type":         order.Type,
			"origQty":      order.OrigQty,
			"price":        order.Price,
			"stopPrice":    order.StopPrice,
			"status":       order.Status,
			"time":         order.Time,
			"updateTime":   order.UpdateTime,
			"isAlgoOrder":  false,
		}

		switch order.Type {
		case "TAKE_PROFIT", "TAKE_PROFIT_MARKET":
			takeProfitOrders = append(takeProfitOrders, orderData)
		case "STOP", "STOP_MARKET":
			stopLossOrders = append(stopLossOrders, orderData)
		case "TRAILING_STOP_MARKET":
			trailingStopOrders = append(trailingStopOrders, orderData)
		default:
			otherOrders = append(otherOrders, orderData)
		}
	}

	// Process algo orders (new API)
	for _, order := range algoOrders {
		orderData := gin.H{
			"algoId":       order.AlgoId,
			"orderId":      order.AlgoId, // Use algoId as orderId for UI compatibility
			"symbol":       order.Symbol,
			"side":         order.Side,
			"positionSide": order.PositionSide,
			"type":         order.OrderType,
			"origQty":      order.Quantity,
			"price":        order.Price,
			"stopPrice":    order.TriggerPrice, // TriggerPrice is the stopPrice equivalent
			"status":       order.AlgoStatus,
			"time":         order.CreateTime,
			"updateTime":   order.UpdateTime,
			"isAlgoOrder":  true,
			"workingType":  order.WorkingType,
		}

		switch order.OrderType {
		case "TAKE_PROFIT", "TAKE_PROFIT_MARKET":
			takeProfitOrders = append(takeProfitOrders, orderData)
		case "STOP", "STOP_MARKET":
			stopLossOrders = append(stopLossOrders, orderData)
		case "TRAILING_STOP_MARKET":
			trailingStopOrders = append(trailingStopOrders, orderData)
		default:
			otherOrders = append(otherOrders, orderData)
		}
	}

	// Also get historical algo orders
	allAlgoOrders, allAlgoErr := futuresClient.GetAllAlgoOrders(symbol, 20)
	if allAlgoErr != nil {
		// Silently continue - algo orders API may not be available
		allAlgoOrders = nil
	}

	// Format historical algo orders for response
	var historicalAlgoOrders []interface{}
	for _, order := range allAlgoOrders {
		historicalAlgoOrders = append(historicalAlgoOrders, gin.H{
			"algoId":       order.AlgoId,
			"symbol":       order.Symbol,
			"side":         order.Side,
			"positionSide": order.PositionSide,
			"type":         order.OrderType,
			"quantity":     order.Quantity,
			"triggerPrice": order.TriggerPrice,
			"price":        order.Price,
			"status":       order.AlgoStatus,
			"createTime":   order.CreateTime,
			"updateTime":   order.UpdateTime,
			"triggerTime":  order.TriggerTime,
			"executedQty":  order.ExecutedQty,
			"workingType":  order.WorkingType,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"symbol":                  symbol,
		"open_orders":             otherOrders,
		"take_profit_orders":      takeProfitOrders,
		"stop_loss_orders":        stopLossOrders,
		"trailing_stop_orders":    trailingStopOrders,
		"historical_algo_orders":  historicalAlgoOrders,
	})
}

// handleCancelAlgoOrder cancels a single algo order (TP/SL)
func (s *Server) handleCancelAlgoOrder(c *gin.Context) {
	symbol := c.Param("symbol")
	algoIdStr := c.Param("id")

	algoId, err := strconv.ParseInt(algoIdStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid algo ID"})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	err = futuresClient.CancelAlgoOrder(symbol, algoId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast ORDER_UPDATE to WebSocket clients for instant UI refresh
	userID := s.getUserID(c)
	if userID != "" {
		events.BroadcastOrderUpdate(userID, map[string]interface{}{
			"action": "cancelled",
			"algoId": algoId,
			"symbol": symbol,
			"type":   "algo",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Algo order canceled",
		"algoId":  algoId,
		"symbol":  symbol,
	})
}

// handleCancelAllAlgoOrders cancels all algo orders (TP/SL) for a symbol
func (s *Server) handleCancelAllAlgoOrders(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	err := futuresClient.CancelAllAlgoOrders(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast ORDER_UPDATE to WebSocket clients for instant UI refresh
	userID := s.getUserID(c)
	if userID != "" {
		events.BroadcastOrderUpdate(userID, map[string]interface{}{
			"action": "cancelled_all",
			"symbol": symbol,
			"type":   "algo",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "All algo orders canceled",
		"symbol":  symbol,
	})
}

// handleSetPositionTPSL sets take profit and stop loss for a position
// Uses the new Algo Order API (mandatory since 2025-12-09)
func (s *Server) handleSetPositionTPSL(c *gin.Context) {
	symbol := c.Param("symbol")

	var req struct {
		PositionSide string   `json:"position_side"`
		TakeProfit   *float64 `json:"take_profit"`
		StopLoss     *float64 `json:"stop_loss"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Get current position to determine side and quantity
	position, err := futuresClient.GetPositionBySymbol(symbol)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get position: " + err.Error()})
		return
	}

	if position.PositionAmt == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No open position for this symbol"})
		return
	}

	// Get position side from Binance response
	// In ONE_WAY mode, Binance returns "BOTH" - use it directly
	// In HEDGE mode, Binance returns "LONG" or "SHORT"
	positionSide := binance.PositionSide(position.PositionSide)
	if req.PositionSide != "" {
		positionSide = binance.PositionSide(req.PositionSide)
	}

	// If position side is empty (shouldn't happen), default to BOTH for ONE_WAY mode
	if positionSide == "" {
		positionSide = binance.PositionSideBoth
	}

	log.Printf("[TP/SL] Setting TP/SL for %s, position_side=%s, position_amt=%.4f",
		symbol, positionSide, position.PositionAmt)

	// Determine close side based on position amount
	closeSide := "SELL"
	if position.PositionAmt < 0 {
		closeSide = "BUY"
	}

	var tpOrder, slOrder *binance.AlgoOrderResponse
	var errors []string

	// Cancel existing TP/SL algo orders for this position first
	algoOrders, _ := futuresClient.GetOpenAlgoOrders(symbol)
	for _, order := range algoOrders {
		if order.PositionSide == string(positionSide) {
			if order.OrderType == "TAKE_PROFIT" || order.OrderType == "TAKE_PROFIT_MARKET" ||
				order.OrderType == "STOP" || order.OrderType == "STOP_MARKET" {
				futuresClient.CancelAlgoOrder(symbol, order.AlgoId)
			}
		}
	}

	// Also cancel any old-style orders (for backwards compatibility)
	existingOrders, _ := futuresClient.GetOpenOrders(symbol)
	for _, order := range existingOrders {
		if order.PositionSide == string(positionSide) {
			if order.Type == "TAKE_PROFIT" || order.Type == "TAKE_PROFIT_MARKET" ||
				order.Type == "STOP" || order.Type == "STOP_MARKET" {
				futuresClient.CancelFuturesOrder(symbol, order.OrderId)
			}
		}
	}

	// Place Take Profit order using NEW Algo Order API
	if req.TakeProfit != nil && *req.TakeProfit > 0 {
		tpParams := binance.AlgoOrderParams{
			Symbol:        symbol,
			Side:          closeSide,
			PositionSide:  positionSide,
			Type:          binance.FuturesOrderTypeTakeProfitMarket,
			TriggerPrice:  *req.TakeProfit,
			ClosePosition: true,
			WorkingType:   binance.WorkingTypeMarkPrice,
		}
		order, err := futuresClient.PlaceAlgoOrder(tpParams)
		if err != nil {
			errors = append(errors, "TP: "+err.Error())
		} else {
			tpOrder = order
		}
	}

	// Place Stop Loss order using NEW Algo Order API
	if req.StopLoss != nil && *req.StopLoss > 0 {
		slParams := binance.AlgoOrderParams{
			Symbol:        symbol,
			Side:          closeSide,
			PositionSide:  positionSide,
			Type:          binance.FuturesOrderTypeStopMarket,
			TriggerPrice:  *req.StopLoss,
			ClosePosition: true,
			WorkingType:   binance.WorkingTypeMarkPrice,
		}
		order, err := futuresClient.PlaceAlgoOrder(slParams)
		if err != nil {
			errors = append(errors, "SL: "+err.Error())
		} else {
			slOrder = order
		}
	}

	response := gin.H{
		"success": len(errors) == 0,
		"message": "TP/SL orders placed via Algo Order API",
		"symbol":  symbol,
	}

	if tpOrder != nil {
		response["take_profit_order"] = tpOrder
	}
	if slOrder != nil {
		response["stop_loss_order"] = slOrder
	}
	if len(errors) > 0 {
		response["errors"] = errors
		response["message"] = "Some orders failed"
	}

	// Broadcast ORDER_UPDATE to WebSocket clients for instant UI refresh
	userID := s.getUserID(c)
	if userID != "" {
		events.BroadcastOrderUpdate(userID, map[string]interface{}{
			"action":       "tpsl_updated",
			"symbol":       symbol,
			"type":         "algo",
			"positionSide": req.PositionSide,
		})
	}

	c.JSON(http.StatusOK, response)
}

// handleGetFundingRate returns the current funding rate
func (s *Server) handleGetFundingRate(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	fundingRate, err := futuresClient.GetFundingRate(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, fundingRate)
}

// handleGetOrderBookDepth returns the order book
func (s *Server) handleGetOrderBookDepth(c *gin.Context) {
	symbol := c.Param("symbol")
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	if limit <= 0 || limit > 1000 {
		limit = 20
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	orderBook, err := futuresClient.GetOrderBookDepth(symbol, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orderBook)
}

// handleGetFuturesSymbols returns available futures symbols
func (s *Server) handleGetFuturesSymbols(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	symbols, err := futuresClient.GetFuturesSymbols()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, symbols)
}

// handleGetFuturesTradeHistory returns futures trade history from database
func (s *Server) handleGetFuturesTradeHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	includeAI := c.DefaultQuery("include_ai", "false") == "true"
	includeOpen := c.DefaultQuery("include_open", "false") == "true"

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	ctx := context.Background()

	var err error
	var trades []database.FuturesTrade

	if includeAI {
		// Get trades with AI decisions
		trades, err = s.repo.GetDB().GetFuturesTradeHistoryWithAI(ctx, limit, offset, includeOpen)
	} else {
		// Get trades without AI decisions
		trades, err = s.repo.GetDB().GetFuturesTradeHistory(ctx, limit, offset)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, trades)
}

// handleGetFundingFeeHistory returns funding fee history
func (s *Server) handleGetFundingFeeHistory(c *gin.Context) {
	symbol := c.Query("symbol")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	ctx := context.Background()
	fees, err := s.repo.GetDB().GetFundingFeeHistory(ctx, symbol, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, fees)
}

// handleGetFuturesTransactionHistory returns transaction history
func (s *Server) handleGetFuturesTransactionHistory(c *gin.Context) {
	symbol := c.Query("symbol")
	incomeType := c.Query("income_type")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	ctx := context.Background()
	transactions, err := s.repo.GetDB().GetFuturesTransactionHistory(ctx, symbol, incomeType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, transactions)
}

// Metrics cache to avoid rate limiting
var (
	metricsCache     map[string]interface{}
	metricsCacheTime time.Time
	metricsCacheTTL  = 5 * time.Minute

	// Binance Income PnL cache (separate from metrics cache for accuracy)
	binancePnLCache struct {
		DailyPnL  float64
		TotalPnL  float64
		CacheTime time.Time
	}
	binancePnLCacheTTL = 2 * time.Minute // More frequent updates for PnL
)

// handleGetFuturesMetrics returns futures trading metrics from Binance Income History API
// Daily PnL and Total PnL come from Binance /fapi/v1/income endpoint with incomeType=REALIZED_PNL
// Results are cached for 5 minutes to avoid rate limiting
func (s *Server) handleGetFuturesMetrics(c *gin.Context) {
	// Return cached metrics if still valid
	if metricsCache != nil && time.Since(metricsCacheTime) < metricsCacheTTL {
		c.JSON(http.StatusOK, metricsCache)
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Get user's timezone from settings (auth middleware uses "user_id")
	userID := c.GetString("user_id")
	ctx := c.Request.Context()
	userLoc, tzName, tzOffset := s.getUserTimezone(ctx, userID)

	// Calculate time boundaries for daily PnL (user's timezone)
	now := time.Now().In(userLoc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, userLoc)
	startOfDayMs := startOfDay.UnixMilli()
	endOfDay := startOfDay.Add(24 * time.Hour)

	// Weekly boundaries (last 7 days from start of today)
	startOfWeek := startOfDay.AddDate(0, 0, -6) // 7 days including today
	startOfWeekMs := startOfWeek.UnixMilli()

	// For total PnL, fetch last 7 days (matches Binance UI default view)
	sevenDaysAgo := now.AddDate(0, 0, -7)
	startTimeMs := sevenDaysAgo.UnixMilli()

	// Fetch income history from Binance API (REALIZED_PNL only)
	log.Printf("[METRICS] Fetching income history from Binance: startTime=%d, endTime=now", startTimeMs)
	allIncomeRecords, err := futuresClient.GetIncomeHistory("REALIZED_PNL", startTimeMs, 0, 1000)
	if err != nil {
		log.Printf("[METRICS] Error fetching income history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch income history: " + err.Error()})
		return
	}

	// Get funding fee history
	var allFundingFees []binance.FundingFeeRecord
	fundingRecords, err := futuresClient.GetIncomeHistory("FUNDING_FEE", startTimeMs, 0, 1000)
	if err == nil {
		for _, record := range fundingRecords {
			allFundingFees = append(allFundingFees, binance.FundingFeeRecord{
				Symbol:    record.Symbol,
				Income:    record.Income,
				Asset:     record.Asset,
				Time:      record.Time,
				Timestamp: record.Timestamp,
			})
		}
	}

	// Get commission (trading fees) history
	var allCommissions []binance.IncomeRecord
	commissionRecords, err := futuresClient.GetIncomeHistory("COMMISSION", startTimeMs, 0, 1000)
	if err == nil {
		allCommissions = commissionRecords
	}

	// Calculate PnL metrics from income records
	var totalRealizedPnl float64
	var dailyRealizedPnl float64
	var totalFundingFees float64
	var dailyFundingFees float64
	var weeklyFundingFees float64
	var totalCommission float64
	var dailyCommission float64
	var weeklyCommission float64
	var weeklyRealizedPnl float64
	var winningTrades, losingTrades int
	var dailyWins, dailyLosses, dailyTrades int
	var weeklyWins, weeklyLosses, weeklyTrades int
	var largestWin, largestLoss float64
	var totalWin, totalLoss float64
	var dailyWin, dailyLoss float64
	var weeklyWin, weeklyLoss float64
	var lastTradeTime int64

	// Process income records
	for _, record := range allIncomeRecords {
		totalRealizedPnl += record.Income

		// Track winning/losing trades
		if record.Income > 0 {
			winningTrades++
			totalWin += record.Income
			if record.Income > largestWin {
				largestWin = record.Income
			}
		} else if record.Income < 0 {
			losingTrades++
			totalLoss += record.Income
			if record.Income < largestLoss {
				largestLoss = record.Income
			}
		}

		// Weekly stats
		if record.Time >= startOfWeekMs {
			weeklyRealizedPnl += record.Income
			weeklyTrades++
			if record.Income > 0 {
				weeklyWins++
				weeklyWin += record.Income
			} else if record.Income < 0 {
				weeklyLosses++
				weeklyLoss += record.Income
			}
		}

		// Daily stats
		if record.Time >= startOfDayMs {
			dailyRealizedPnl += record.Income
			dailyTrades++
			if record.Income > 0 {
				dailyWins++
				dailyWin += record.Income
			} else if record.Income < 0 {
				dailyLosses++
				dailyLoss += record.Income
			}
		}

		// Track last trade time
		if record.Time > lastTradeTime {
			lastTradeTime = record.Time
		}
	}

	// Calculate funding fees (total, daily, weekly)
	for _, fee := range allFundingFees {
		totalFundingFees += fee.Income
		if fee.Time >= startOfDayMs {
			dailyFundingFees += fee.Income
		}
		if fee.Time >= startOfWeekMs {
			weeklyFundingFees += fee.Income
		}
	}

	// Calculate commissions (total, daily, weekly)
	for _, comm := range allCommissions {
		totalCommission += comm.Income // Note: commission is negative
		if comm.Time >= startOfDayMs {
			dailyCommission += comm.Income
		}
		if comm.Time >= startOfWeekMs {
			weeklyCommission += comm.Income
		}
	}

	// Calculate derived metrics
	totalTrades := winningTrades + losingTrades
	var winRate, averagePnl, averageWin, averageLoss, profitFactor, dailyWinRate float64

	if totalTrades > 0 {
		winRate = float64(winningTrades) / float64(totalTrades) * 100
		averagePnl = totalRealizedPnl / float64(totalTrades)
	}
	if winningTrades > 0 {
		averageWin = totalWin / float64(winningTrades)
	}
	if losingTrades > 0 {
		averageLoss = totalLoss / float64(losingTrades)
	}
	if totalLoss != 0 {
		profitFactor = totalWin / (-totalLoss)
	}
	if dailyTrades > 0 {
		dailyWinRate = float64(dailyWins) / float64(dailyTrades) * 100
	}

	// Get positions and orders count (single API call each)
	positions, _ := futuresClient.GetPositions()
	openPositions := 0
	var totalLeverage int
	for _, pos := range positions {
		if pos.PositionAmt != 0 {
			openPositions++
			totalLeverage += pos.Leverage
		}
	}

	avgLeverage := 0.0
	if openPositions > 0 {
		avgLeverage = float64(totalLeverage) / float64(openPositions)
	}

	openOrders, _ := futuresClient.GetOpenOrders("")
	openOrderCount := len(openOrders)

	// Get unrealized PnL from account
	accountInfo, _ := futuresClient.GetFuturesAccountInfo()
	totalUnrealizedPnl := 0.0
	if accountInfo != nil {
		totalUnrealizedPnl = accountInfo.TotalUnrealizedProfit
	}

	// Format last trade time
	var lastTradeTimeStr string
	if lastTradeTime > 0 {
		lastTradeTimeStr = time.UnixMilli(lastTradeTime).Format(time.RFC3339)
	}

	// Calculate weekly win rate
	var weeklyWinRate float64
	if weeklyTrades > 0 {
		weeklyWinRate = float64(weeklyWins) / float64(weeklyTrades) * 100
	}

	// Calculate gross profit (wins only, before accounting for losses and fees)
	// Daily gross = sum of winning trades today
	// Daily total fees = commission (trading fees) + funding fees (negative values)
	dailyTotalFees := -dailyCommission + (-dailyFundingFees) // Convert to positive for display
	weeklyTotalFees := -weeklyCommission + (-weeklyFundingFees)

	// Net PnL = Gross Profit - Gross Loss - Fees
	// Since dailyRealizedPnl already includes wins and losses, the net is:
	// dailyNetPnl = dailyRealizedPnl (which is sum of all trades)
	// But for breakdown: dailyGrossProfit = dailyWin, dailyGrossLoss = dailyLoss

	metrics := map[string]interface{}{
		"totalTrades":        totalTrades,
		"winningTrades":      winningTrades,
		"losingTrades":       losingTrades,
		"winRate":            winRate,
		"totalRealizedPnl":   totalRealizedPnl,    // From Binance Income API (last 7 days)
		"totalUnrealizedPnl": totalUnrealizedPnl,
		"totalFundingFees":   totalFundingFees,
		"totalCommission":    totalCommission, // Trading fees (negative)
		"averagePnl":         averagePnl,
		"averageWin":         averageWin,
		"averageLoss":        averageLoss,
		"largestWin":         largestWin,
		"largestLoss":        largestLoss,
		"profitFactor":       profitFactor,
		"averageLeverage":    avgLeverage,
		"openPositions":      openPositions,
		"openOrders":         openOrderCount,

		// Daily stats (detailed breakdown for Daily Net PNL card)
		"dailyRealizedPnl": dailyRealizedPnl, // Net PnL from trades (today only)
		"dailyGrossProfit": dailyWin,         // Sum of winning trades
		"dailyGrossLoss":   dailyLoss,        // Sum of losing trades (negative)
		"dailyCommission":  dailyCommission,  // Trading fees (negative)
		"dailyFundingFees": dailyFundingFees, // Funding fees (can be + or -)
		"dailyTotalFees":   dailyTotalFees,   // Total fees as positive number
		"dailyTrades":      dailyTrades,
		"dailyWins":        dailyWins,
		"dailyLosses":      dailyLosses,
		"dailyWinRate":     dailyWinRate,

		// Weekly stats (detailed breakdown for Weekly Net PNL card)
		"weeklyRealizedPnl": weeklyRealizedPnl, // Net PnL from trades (last 7 days)
		"weeklyGrossProfit": weeklyWin,         // Sum of winning trades
		"weeklyGrossLoss":   weeklyLoss,        // Sum of losing trades (negative)
		"weeklyCommission":  weeklyCommission,  // Trading fees (negative)
		"weeklyFundingFees": weeklyFundingFees, // Funding fees (can be + or -)
		"weeklyTotalFees":   weeklyTotalFees,   // Total fees as positive number
		"weeklyTrades":      weeklyTrades,
		"weeklyWins":        weeklyWins,
		"weeklyLosses":      weeklyLosses,
		"weeklyWinRate":     weeklyWinRate,

		// Time boundaries (for countdown timers and period display)
		"dailyResetTime":    endOfDay.UnixMilli(),             // Next daily reset (user's timezone midnight)
		"weeklyStartDate":   startOfWeek.Format("2006-01-02"), // Week start date
		"weeklyEndDate":     startOfDay.Format("2006-01-02"),  // Week end date (today)
		"serverTimeUTC":     time.Now().UTC().UnixMilli(),     // Current server time in UTC
		"timezone":          tzName,                           // User's timezone identifier
		"timezoneOffset":    tzOffset,                         // User's timezone offset (e.g., "+05:30")

		"lastTradeTime": lastTradeTimeStr,
	}

	// Cache the metrics
	metricsCache = metrics
	metricsCacheTime = time.Now()

	log.Printf("[METRICS] Calculated from Binance Income API: daily=$%.2f, total(7d)=$%.2f, trades=%d, records=%d",
		dailyRealizedPnl, totalRealizedPnl, totalTrades, len(allIncomeRecords))

	c.JSON(http.StatusOK, metrics)
}

// GetCachedDailyPnL returns cached daily and total P/L from metrics cache
// DEPRECATED: Use GetBinancePnLForAutopilot for accurate Binance Income History data
func (s *Server) GetCachedDailyPnL() (dailyPnL float64, totalPnL float64) {
	// Check if Binance PnL cache is still valid
	if time.Since(binancePnLCache.CacheTime) < binancePnLCacheTTL {
		return binancePnLCache.DailyPnL, binancePnLCache.TotalPnL
	}

	// Fallback to metrics cache
	if metricsCache != nil && time.Since(metricsCacheTime) < metricsCacheTTL {
		if daily, ok := metricsCache["dailyRealizedPnl"].(float64); ok {
			dailyPnL = daily
		}
		if total, ok := metricsCache["totalRealizedPnl"].(float64); ok {
			totalPnL = total
		}
		return dailyPnL, totalPnL
	}

	// Return cached Binance values even if stale (better than zeros)
	if !binancePnLCache.CacheTime.IsZero() {
		return binancePnLCache.DailyPnL, binancePnLCache.TotalPnL
	}

	return 0.0, 0.0
}

// GetBinancePnLForAutopilot fetches PnL directly from Binance Income History API
// Uses a per-user cache with 2-minute TTL for accuracy
// This is the preferred method for getting accurate PnL data
// Paginates through ALL income records to get accurate total PnL
func (s *Server) GetBinancePnLForAutopilot(ga *autopilot.GinieAutopilot) (dailyPnL float64, totalPnL float64) {
	if ga == nil {
		log.Printf("[PNL-SYNC] No autopilot provided, returning cached values")
		return s.GetCachedDailyPnL()
	}

	// Check if Binance PnL cache is still valid
	if time.Since(binancePnLCache.CacheTime) < binancePnLCacheTTL {
		return binancePnLCache.DailyPnL, binancePnLCache.TotalPnL
	}

	// Get the autopilot's futures client
	futuresClient := ga.GetFuturesClient()
	if futuresClient == nil {
		log.Printf("[PNL-SYNC] No futures client available, returning cached values")
		return s.GetCachedDailyPnL()
	}

	// Get user's timezone for accurate daily PnL calculation
	userID := ga.GetUserID()
	ctx := context.Background()
	userLoc, _, _ := s.getUserTimezone(ctx, userID)

	// Calculate time boundaries using user's timezone
	now := time.Now().In(userLoc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, userLoc)
	startOfDayMs := startOfDay.UnixMilli()

	// For "total" PnL, fetch last 7 days (matches Binance UI default view)
	// Binance Futures PnL widget defaults to 7-day view
	sevenDaysAgo := now.AddDate(0, 0, -7)
	startTimeMs := sevenDaysAgo.UnixMilli()

	// Paginate through income records for the last 7 days
	// Binance API returns records in descending order (newest first)
	var allRecords []binance.IncomeRecord
	var endTime int64 = 0 // 0 means no limit (get latest records first)
	maxPages := 5         // Safety limit: 5 pages * 1000 = 5,000 records max for 7 days

	for page := 0; page < maxPages; page++ {
		records, err := futuresClient.GetIncomeHistory("REALIZED_PNL", startTimeMs, endTime, 1000)
		if err != nil {
			log.Printf("[PNL-SYNC] Failed to fetch page %d: %v", page, err)
			break
		}

		if len(records) == 0 {
			break // No more records
		}

		allRecords = append(allRecords, records...)

		// If we got less than 1000, we've reached the end
		if len(records) < 1000 {
			break
		}

		// Set endTime to oldest record's time - 1ms for next page
		oldestTime := records[len(records)-1].Time
		endTime = oldestTime - 1

		// Stop if we've gone past our start time
		if endTime < startTimeMs {
			break
		}

		// Small delay to avoid rate limits
		time.Sleep(50 * time.Millisecond)
	}

	// Sum up PnL from all records
	for _, record := range allRecords {
		totalPnL += record.Income
		if record.Time >= startOfDayMs {
			dailyPnL += record.Income
		}
	}

	// Update cache
	binancePnLCache.DailyPnL = dailyPnL
	binancePnLCache.TotalPnL = totalPnL
	binancePnLCache.CacheTime = time.Now()

	log.Printf("[PNL-SYNC] Fetched from Binance: daily=$%.2f, 7d_total=$%.2f (%d records)", dailyPnL, totalPnL, len(allRecords))
	return dailyPnL, totalPnL
}

// pnlRefreshInProgress tracks if a background refresh is already running
var pnlRefreshInProgress bool
var pnlRefreshMutex sync.Mutex

// GetBinancePnLNonBlocking returns cached PnL immediately and triggers background refresh if needed
// This prevents API timeouts when Binance is slow
func (s *Server) GetBinancePnLNonBlocking(ga *autopilot.GinieAutopilot) (dailyPnL float64, totalPnL float64) {
	// Always return cached values immediately (even if stale)
	dailyPnL = binancePnLCache.DailyPnL
	totalPnL = binancePnLCache.TotalPnL

	// Check if cache is expired and needs refresh
	cacheExpired := time.Since(binancePnLCache.CacheTime) >= binancePnLCacheTTL

	if cacheExpired && ga != nil {
		// Check if a refresh is already in progress
		pnlRefreshMutex.Lock()
		if !pnlRefreshInProgress {
			pnlRefreshInProgress = true
			pnlRefreshMutex.Unlock()

			// Trigger background refresh
			go func() {
				defer func() {
					pnlRefreshMutex.Lock()
					pnlRefreshInProgress = false
					pnlRefreshMutex.Unlock()
				}()

				// Call the blocking version in background
				s.GetBinancePnLForAutopilot(ga)
			}()
		} else {
			pnlRefreshMutex.Unlock()
		}
	}

	return dailyPnL, totalPnL
}

// handleGetTradeSourceStats returns trading stats grouped by trade source (AI, Strategy, Manual)
func (s *Server) handleGetTradeSourceStats(c *gin.Context) {
	// Initialize stats for each source
	type SourceStats struct {
		TotalTrades   int     `json:"totalTrades"`
		WinningTrades int     `json:"winningTrades"`
		LosingTrades  int     `json:"losingTrades"`
		WinRate       float64 `json:"winRate"`
		TotalPnL      float64 `json:"totalPnl"`
		TPHits        int     `json:"tpHits"`
		SLHits        int     `json:"slHits"`
		AvgPnL        float64 `json:"avgPnl"`
	}

	stats := map[string]*SourceStats{
		"ai":       {},
		"strategy": {},
		"manual":   {},
	}

	// Get futures client to fetch actual trades from Binance
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusOK, gin.H{
			"ai":       stats["ai"],
			"strategy": stats["strategy"],
			"manual":   stats["manual"],
		})
		return
	}

	// Fetch trades for common symbols from Binance API
	symbols := []string{
		"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
		"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT",
		"DOTUSDT", "LTCUSDT", "ATOMUSDT", "UNIUSDT", "NEARUSDT",
	}

	// Track unique position closes to avoid counting partial fills as separate trades
	type positionKey struct {
		symbol   string
		orderId  int64
	}
	closedPositions := make(map[positionKey]float64) // orderId -> total PnL

	for _, sym := range symbols {
		trades, err := futuresClient.GetTradeHistory(sym, 100)
		if err != nil {
			continue // Skip symbols with errors
		}

		// Group trades by orderId and sum PnL
		for _, trade := range trades {
			if trade.RealizedPnl != 0 { // Only count trades that closed positions (have PnL)
				key := positionKey{symbol: sym, orderId: trade.OrderId}
				closedPositions[key] += trade.RealizedPnl
			}
		}
	}

	// Calculate stats from closed positions
	// Since autopilot is managing all trades, attribute to AI
	aiStats := stats["ai"]

	for _, pnl := range closedPositions {
		aiStats.TotalTrades++
		aiStats.TotalPnL += pnl

		if pnl > 0 {
			aiStats.WinningTrades++
			aiStats.TPHits++ // Positive PnL typically means TP hit
		} else if pnl < 0 {
			aiStats.LosingTrades++
			aiStats.SLHits++ // Negative PnL typically means SL hit
		}
	}

	// Calculate percentages
	for _, st := range stats {
		if st.TotalTrades > 0 {
			st.WinRate = float64(st.WinningTrades) / float64(st.TotalTrades) * 100
			st.AvgPnL = st.TotalPnL / float64(st.TotalTrades)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ai":       stats["ai"],
		"strategy": stats["strategy"],
		"manual":   stats["manual"],
	})
}

// handleGetPositionTradeSources returns trade source (AI/Strategy/Manual) for open positions
func (s *Server) handleGetPositionTradeSources(c *gin.Context) {
	ctx := context.Background()

	// Create a map of symbol -> trade source
	sources := make(map[string]string)

	// First, check autopilot's active positions - these are AI trades
	controller := s.getFuturesAutopilot()
	if controller != nil {
		autopilotSymbols := controller.GetActivePositionSymbols()
		for _, symbol := range autopilotSymbols {
			sources[symbol] = "ai"
		}
	}

	// Then check database for any trades not in autopilot
	trades, err := s.repo.GetDB().GetOpenFuturesTrades(ctx)
	if err == nil {
		for _, trade := range trades {
			// Only set if not already set by autopilot
			if _, exists := sources[trade.Symbol]; !exists {
				if trade.TradeSource != "" {
					sources[trade.Symbol] = trade.TradeSource
				} else {
					sources[trade.Symbol] = "manual"
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
	})
}

// handleGetFuturesAccountSettings returns account settings for a symbol
func (s *Server) handleGetFuturesAccountSettings(c *gin.Context) {
	symbol := c.Param("symbol")

	ctx := context.Background()
	settings, err := s.repo.GetDB().GetFuturesAccountSettings(ctx, symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// handleGetMarkPrice returns mark price for a symbol
func (s *Server) handleGetMarkPrice(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	markPrice, err := futuresClient.GetMarkPrice(symbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, markPrice)
}

// handleGetFuturesKlines returns klines for futures
func (s *Server) handleGetFuturesKlines(c *gin.Context) {
	symbol := c.Query("symbol")
	interval := c.DefaultQuery("interval", "1h")
	limitStr := c.DefaultQuery("limit", "100")

	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	limit, _ := strconv.Atoi(limitStr)

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	klines, err := futuresClient.GetFuturesKlines(symbol, interval, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, klines)
}

// handleGetFuturesWalletBalance returns the futures wallet balance
func (s *Server) handleGetFuturesWalletBalance(c *gin.Context) {
	// Get user ID from auth context
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// Check if we're in dry run mode - use per-user mode if authenticated
	isSimulated := false
	if userID != "" {
		// Get per-user trading mode from database
		dryRun, err := s.repo.GetUserDryRunMode(ctx, userID)
		if err != nil {
			log.Printf("[FUTURES-WALLET] Error getting user dry run mode for %s: %v, defaulting to paper", userID, err)
			dryRun = true
		}
		isSimulated = dryRun
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		// If in LIVE mode but no client, user needs to configure API keys
		if !isSimulated {
			log.Printf("[FUTURES-WALLET] User %s in LIVE mode but no client - API key configuration needed", userID)
			c.JSON(http.StatusOK, gin.H{
				"total_balance":        0.0,
				"available_balance":    0.0,
				"total_margin_balance": 0.0,
				"total_unrealized_pnl": 0.0,
				"currency":             "USDT",
				"is_simulated":         false,
				"error":                "api_keys_required",
				"message":              "Please configure your Binance API keys in Settings to access live trading",
				"assets":               []gin.H{},
			})
			return
		}
		// Return mock balance if in paper trading mode
		// Get paper balance from database
		paperBalance, _, err := s.repo.GetUserPaperBalance(ctx, userID)
		if err != nil {
			log.Printf("[FUTURES-WALLET] Error getting paper balance for %s: %v, using default", userID, err)
			paperBalance = 10000.0 // fallback default
		}
		if paperBalance == 0 {
			paperBalance = 10000.0 // fallback for zero balance
		}
		availableBalance := paperBalance * 0.95 // 5% margin buffer

		c.JSON(http.StatusOK, gin.H{
			"total_balance":        paperBalance,
			"available_balance":    availableBalance,
			"total_margin_balance": paperBalance,
			"total_unrealized_pnl": 0.0,
			"currency":             "USDT",
			"is_simulated":         true,
			"assets": []gin.H{
				{"asset": "USDT", "wallet_balance": paperBalance, "cross_wallet": paperBalance, "available_balance": availableBalance, "unrealized_profit": 0.0},
			},
		})
		return
	}

	accountInfo, err := futuresClient.GetFuturesAccountInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build assets list from account positions
	assets := make([]gin.H, 0)
	for _, asset := range accountInfo.Assets {
		if asset.WalletBalance > 0 || asset.CrossWalletBalance > 0 {
			assets = append(assets, gin.H{
				"asset":              asset.Asset,
				"wallet_balance":     asset.WalletBalance,
				"cross_wallet":       asset.CrossWalletBalance,
				"available_balance":  asset.AvailableBalance,
				"unrealized_profit":  asset.UnrealizedProfit,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_balance":        accountInfo.TotalWalletBalance,
		"available_balance":    accountInfo.AvailableBalance,
		"total_margin_balance": accountInfo.TotalMarginBalance,
		"total_unrealized_pnl": accountInfo.TotalUnrealizedProfit,
		"currency":             "USDT",
		"is_simulated":         isSimulated,
		"assets":               assets,
	})
}

// handleCloseAllFuturesPositions closes all open futures positions (PANIC BUTTON)
func (s *Server) handleCloseAllFuturesPositions(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// Get all positions
	positions, err := futuresClient.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter active positions
	activePositions := make([]binance.FuturesPosition, 0)
	for _, pos := range positions {
		if pos.PositionAmt != 0 {
			activePositions = append(activePositions, pos)
		}
	}

	if len(activePositions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"message":  "No open positions to close",
			"closed":   0,
			"total":    0,
			"errors":   []string{},
		})
		return
	}

	// Close all positions
	closed := 0
	errors := []string{}
	closedPositions := []gin.H{}

	for _, position := range activePositions {
		// Determine close side and quantity
		var side string
		var quantity float64
		if position.PositionAmt > 0 {
			side = "SELL"
			quantity = position.PositionAmt
		} else {
			side = "BUY"
			quantity = -position.PositionAmt
		}

		// Place market order to close
		// In hedge mode, ReduceOnly is not required (position side is used instead)
		params := binance.FuturesOrderParams{
			Symbol:       position.Symbol,
			Side:         side,
			PositionSide: binance.PositionSide(position.PositionSide),
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     quantity,
			ReduceOnly:   position.PositionSide == "BOTH", // Only use in one-way mode
		}

		orderResp, err := futuresClient.PlaceFuturesOrder(params)
		if err != nil {
			errors = append(errors, position.Symbol+": "+err.Error())
		} else {
			closed++
			closedPositions = append(closedPositions, gin.H{
				"symbol":   position.Symbol,
				"side":     side,
				"quantity": quantity,
				"order_id": orderResp.OrderId,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Panic close completed",
		"closed":           closed,
		"total":            len(activePositions),
		"errors":           errors,
		"closed_positions": closedPositions,
	})
}

// handleGetFuturesAccountTrades returns trade history directly from Binance API
func (s *Server) handleGetFuturesAccountTrades(c *gin.Context) {
	symbol := c.Query("symbol")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	// If no symbol specified, get trades for common symbols
	symbols := []string{symbol}
	if symbol == "" {
		symbols = []string{
			"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
			"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT",
			"DOTUSDT", "LTCUSDT", "ATOMUSDT", "UNIUSDT", "NEARUSDT",
		}
	}

	allTrades := []gin.H{}
	errors := []string{}

	for _, sym := range symbols {
		if sym == "" {
			continue
		}
		trades, err := futuresClient.GetTradeHistory(sym, limit)
		if err != nil {
			errors = append(errors, sym+": "+err.Error())
			continue
		}

		for _, trade := range trades {
			allTrades = append(allTrades, gin.H{
				"symbol":          sym,
				"id":              trade.ID,
				"orderId":         trade.OrderId,
				"side":            trade.Side,
				"positionSide":    trade.PositionSide,
				"price":           trade.Price,
				"qty":             trade.Qty,
				"realizedPnl":     trade.RealizedPnl,
				"marginAsset":     trade.MarginAsset,
				"quoteQty":        trade.QuoteQty,
				"commission":      trade.Commission,
				"commissionAsset": trade.CommissionAsset,
				"time":            trade.Time,
				"buyer":           trade.Buyer,
				"maker":           trade.Maker,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"trades": allTrades,
		"errors": errors,
		"count":  len(allTrades),
	})
}

// ==================== HELPER FUNCTIONS ====================

// getFuturesClient returns the futures client from botAPI if available
// This uses the global client configured at startup
func (s *Server) getFuturesClient() binance.FuturesClient {
	if futuresAPI, ok := s.botAPI.(FuturesAPI); ok {
		return futuresAPI.GetFuturesClient()
	}
	return nil
}

// getFuturesClientForUser returns a futures client for the authenticated user
// User must have API keys configured in the database - no global fallback
// Returns nil if user has no API keys (caller should return error to user)
func (s *Server) getFuturesClientForUser(c *gin.Context) binance.FuturesClient {
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// Check if in paper trading mode - use per-user mode if authenticated
	if userID != "" {
		// Get per-user trading mode from database
		dryRun, err := s.repo.GetUserDryRunMode(ctx, userID)
		if err != nil {
			log.Printf("[DEBUG] getFuturesClientForUser: Error getting user dry run mode: %v, defaulting to paper", err)
			dryRun = true
		}
		if dryRun {
			log.Printf("[DEBUG] getFuturesClientForUser: User %s in paper trading mode, using mock client", userID)
			return s.getFuturesClient() // Returns mock client in paper mode
		}
	} else {
		// No user authentication - return nil
		log.Printf("[DEBUG] getFuturesClientForUser: No user authentication, cannot provide client")
		return nil
	}

	// Live mode - must use user-specific keys from database
	if s.authEnabled && s.apiKeyService != nil {
		log.Printf("[DEBUG] getFuturesClientForUser: authEnabled=%v, userID=%s in LIVE mode", s.authEnabled, userID)
		if userID != "" {
			// Check cache first (5 minute TTL)
			s.userFuturesClientsMu.RLock()
			if entry, ok := s.userFuturesClients[userID]; ok && time.Since(entry.createdAt) < 5*time.Minute {
				s.userFuturesClientsMu.RUnlock()
				return entry.client
			}
			s.userFuturesClientsMu.RUnlock()

			// Try mainnet first, then testnet
			keys, err := s.apiKeyService.GetActiveBinanceKey(ctx, userID, false)
			if err != nil {
				log.Printf("[DEBUG] getFuturesClientForUser: mainnet key lookup failed: %v, trying testnet", err)
				keys, err = s.apiKeyService.GetActiveBinanceKey(ctx, userID, true)
			}
			if err == nil && keys != nil && keys.APIKey != "" && keys.SecretKey != "" {
				log.Printf("[DEBUG] getFuturesClientForUser: Found user keys, creating cached client (testnet=%v)", keys.IsTestnet)
				rawClient := binance.NewFuturesClient(keys.APIKey, keys.SecretKey, keys.IsTestnet)
				if rawClient != nil {
					// Wrap with CachedFuturesClient for response caching (30s TTL)
					cachedClient := binance.NewCachedFuturesClient(rawClient, nil)
					// Store in cache
					s.userFuturesClientsMu.Lock()
					if s.userFuturesClients == nil {
						s.userFuturesClients = make(map[string]*userFuturesClientEntry)
					}
					s.userFuturesClients[userID] = &userFuturesClientEntry{
						client:    cachedClient,
						createdAt: time.Now(),
					}
					s.userFuturesClientsMu.Unlock()
					return cachedClient
				}
			} else {
				log.Printf("[DEBUG] getFuturesClientForUser: No valid keys found, err=%v, keys=%v", err, keys != nil)
			}
		}
	} else {
		log.Printf("[DEBUG] getFuturesClientForUser: auth not enabled or no apiKeyService (authEnabled=%v, hasService=%v)", s.authEnabled, s.apiKeyService != nil)
	}

	// No user API keys found - return nil (caller should return error)
	log.Printf("[DEBUG] getFuturesClientForUser: No user API keys - user must configure keys in Settings")
	return nil
}

// handleGetIncomeHistory retrieves income history from Binance (realized PnL, fees, funding)
// GET /api/futures/income-history?type=&limit=100&start_time=&end_time=
// type: REALIZED_PNL, FUNDING_FEE, COMMISSION, TRANSFER, or empty for all
func (s *Server) handleGetIncomeHistory(c *gin.Context) {
	client := s.getFuturesClientForUser(c)
	if client == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "no_api_keys",
			"message": "Please configure your Binance API keys in Settings",
		})
		return
	}

	// Parse query parameters
	incomeType := c.Query("type") // REALIZED_PNL, FUNDING_FEE, COMMISSION, etc.
	limitStr := c.DefaultQuery("limit", "100")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 100
	}

	var startTime, endTime int64
	if startTimeStr != "" {
		startTime, _ = strconv.ParseInt(startTimeStr, 10, 64)
	}
	if endTimeStr != "" {
		endTime, _ = strconv.ParseInt(endTimeStr, 10, 64)
	}

	// Fetch income history from Binance
	records, err := client.GetIncomeHistory(incomeType, startTime, endTime, limit)
	if err != nil {
		log.Printf("[ERROR] handleGetIncomeHistory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed_to_fetch",
			"message": err.Error(),
		})
		return
	}

	// Calculate summaries by type
	summary := make(map[string]float64)
	for _, r := range records {
		summary[r.IncomeType] += r.Income
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"records": records,
		"count":   len(records),
		"summary": summary,
	})
}

// getOppositeSide returns the opposite side for TP/SL orders
func getOppositeSide(side string) string {
	if side == "BUY" {
		return "SELL"
	}
	return "BUY"
}

// ==================== TIMEZONE HELPERS ====================
// Timezone handling for PnL calculations based on system TZ

var (
	cachedTimezone *time.Location
	timezoneString string
	timezoneOffset int
	tzOnce         sync.Once
)

// getSystemTimezone returns the system timezone from TZ environment variable
func getSystemTimezone() *time.Location {
	tzOnce.Do(func() {
		tz := os.Getenv("TZ")
		if tz != "" {
			loc, err := time.LoadLocation(tz)
			if err == nil {
				cachedTimezone = loc
				// Calculate offset and string representation
				now := time.Now().In(loc)
				_, offset := now.Zone()
				timezoneOffset = offset / 3600
				if timezoneOffset >= 0 {
					timezoneString = fmt.Sprintf("GMT+%d", timezoneOffset)
				} else {
					timezoneString = fmt.Sprintf("GMT%d", timezoneOffset)
				}
				log.Printf("[TIMEZONE] Using system timezone: %s (%s)", tz, timezoneString)
				return
			}
			log.Printf("[TIMEZONE] Failed to load timezone %s: %v, using UTC", tz, err)
		}
		cachedTimezone = time.UTC
		timezoneString = "UTC"
		timezoneOffset = 0
	})
	return cachedTimezone
}

// getStartOfDayInSystemTimezone returns the start of the current day in system timezone as Unix milliseconds
func getStartOfDayInSystemTimezone() int64 {
	loc := getSystemTimezone()
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return startOfDay.UnixMilli()
}

// getWeekStartInSystemTimezone returns the start of the week (Thursday) in system timezone
// Binance resets weekly PnL on Thursday 00:00 UTC, we adjust for local timezone
func getWeekStartInSystemTimezone() (int64, string, string) {
	loc := getSystemTimezone()
	now := time.Now().In(loc)

	// Find the most recent Thursday (Binance weekly reset day)
	daysFromThursday := int(now.Weekday()) - int(time.Thursday)
	if daysFromThursday < 0 {
		daysFromThursday += 7
	}

	weekStart := time.Date(now.Year(), now.Month(), now.Day()-daysFromThursday, 0, 0, 0, 0, loc)
	weekEnd := weekStart.AddDate(0, 0, 6)

	weekStartStr := weekStart.Format("Jan 2")
	weekEndStr := weekEnd.Format("Jan 2")

	return weekStart.UnixMilli(), weekStartStr, weekEndStr
}

// getTimeUntilMidnightInSystemTimezone returns seconds until midnight in system timezone
func getTimeUntilMidnightInSystemTimezone() int64 {
	loc := getSystemTimezone()
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	return int64(midnight.Sub(now).Seconds())
}

// =====================================================
// PER-USER TIMEZONE FUNCTIONS
// Story: User timezone from user_global_trading table
// =====================================================

// getUserTimezone loads the user's timezone from user_global_trading settings
// Falls back to UTC if not set or on error
func (s *Server) getUserTimezone(ctx context.Context, userID string) (*time.Location, string, string) {
	// Default values
	tzName := "UTC"
	tzOffset := "+00:00"
	found := false

	// Try to get user's timezone from cache first
	if s.settingsCacheService != nil {
		globalTrading, err := s.settingsCacheService.GetGlobalTrading(ctx, userID)
		if err == nil && globalTrading != nil && globalTrading.Timezone != "" {
			tzName = globalTrading.Timezone
			if globalTrading.TimezoneOffset != "" {
				tzOffset = globalTrading.TimezoneOffset
			}
			found = true
			log.Printf("[USER-TIMEZONE] Loaded from cache: user=%s, tz=%s, offset=%s", userID, tzName, tzOffset)
		}
	}

	// Fallback to direct DB query if cache didn't have the data
	if !found && s.repo != nil {
		globalTrading, err := s.repo.GetUserGlobalTrading(ctx, userID)
		if err == nil && globalTrading != nil && globalTrading.Timezone != "" {
			tzName = globalTrading.Timezone
			if globalTrading.TimezoneOffset != "" {
				tzOffset = globalTrading.TimezoneOffset
			}
			found = true
			log.Printf("[USER-TIMEZONE] Loaded from DB: user=%s, tz=%s, offset=%s", userID, tzName, tzOffset)
		} else if err != nil {
			log.Printf("[USER-TIMEZONE] DB query failed for user %s: %v", userID, err)
		}
	}

	if !found {
		log.Printf("[USER-TIMEZONE] No timezone found for user %s, using UTC", userID)
	}

	// Load the timezone location
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Printf("[USER-TIMEZONE] Failed to load timezone %s for user %s: %v, using UTC", tzName, userID, err)
		return time.UTC, "UTC", "+00:00"
	}

	return loc, tzName, tzOffset
}

// getStartOfDayForUser returns the start of the current day in UTC as Unix milliseconds
// Binance uses UTC 00:00 as the daily reset time for PnL calculations
func getStartOfDayForUser(loc *time.Location) int64 {
	// Use UTC for Binance PnL calculations (daily reset at 00:00 UTC)
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return startOfDay.UnixMilli()
}

// getWeekStartForUser returns the start of the last 7 days (rolling week) in UTC
// Binance PnL Analysis shows "7 days" as the last 7 days, not a fixed Thursday-Wednesday week
func getWeekStartForUser(loc *time.Location) (int64, string, string) {
	now := time.Now().UTC()

	// Last 7 days: today is the end, 6 days ago is the start (7 days total including today)
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-6, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	weekStartStr := weekStart.Format("Jan 2")
	weekEndStr := weekEnd.Format("Jan 2")

	return weekStart.UnixMilli(), weekStartStr, weekEndStr
}

// getTimeUntilMidnightForUser returns seconds until UTC midnight (Binance daily reset)
func getTimeUntilMidnightForUser(loc *time.Location) int64 {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return int64(midnight.Sub(now).Seconds())
}

// handleGetPnLSummary returns Binance PnL with timezone info and countdown to reset
// Includes breakdown: realized PnL, commission fees (maker/taker), and funding fees
// GET /api/futures/pnl-summary
func (s *Server) handleGetPnLSummary(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "no_api_keys",
			"message": "Please configure your Binance API keys in Settings",
		})
		return
	}

	// Get user ID for per-user timezone (auth middleware uses "user_id")
	userID := c.GetString("user_id")
	log.Printf("[PNL-SUMMARY] Request from userID=%s", userID)

	// Get user's timezone from global_trading settings
	userLoc, userTzName, userTzOffset := s.getUserTimezone(c.Request.Context(), userID)
	log.Printf("[PNL-SUMMARY] Timezone for user %s: tz=%s, offset=%s", userID, userTzName, userTzOffset)

	// Get time boundaries (UTC-based to match Binance)
	_, weekStartDate, weekEndDate := getWeekStartForUser(userLoc)
	secondsUntilReset := getTimeUntilMidnightForUser(userLoc)

	log.Printf("[PNL-SUMMARY] Week range: %s - %s", weekStartDate, weekEndDate)

	// Calculate per-day PnL breakdown for the last 7 days (calendar view)
	// Query each day separately to handle high-volume trading that exceeds 1000 records
	// This approach ensures accurate data for each day regardless of total trade volume
	nowUTC := time.Now().UTC()

	// Structure to hold each day's data
	type DayData struct {
		Date       string  `json:"date"`
		Day        int     `json:"day"`
		DayName    string  `json:"day_name"`
		PnL        float64 `json:"pnl"`
		Commission float64 `json:"commission"`
		Rebate     float64 `json:"rebate"`  // Commission rebates (BNB discount, etc.)
		Funding    float64 `json:"funding"` // Net funding: positive = paid, negative = received
		Other      float64 `json:"other"`   // Any other income types
		NetPnL     float64 `json:"net_pnl"`
		TradeCount int     `json:"trade_count"`
		IsProfit   bool    `json:"is_profit"`
		IsToday    bool    `json:"is_today"`
	}

	dailyData := make([]DayData, 7)

	for i := 0; i < 7; i++ {
		// Day index: 0 = 6 days ago, 6 = today
		dayOffset := 6 - i
		dayStart := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day()-dayOffset, 0, 0, 0, 0, time.UTC)
		dayEnd := dayStart.Add(24 * time.Hour)
		dayStartMs := dayStart.UnixMilli()
		// CRITICAL: Binance API includes records AT endTime (inclusive)
		// Subtract 1ms to make it exclusive - records at 00:00:00 belong to next day
		dayEndMs := dayEnd.UnixMilli() - 1

		// Add small delay between API calls to avoid rate limits (except first call)
		if i > 0 {
			time.Sleep(150 * time.Millisecond)
		}

		// Query this specific day's records from Binance
		// Using startTime and endTime ensures we get all records for this day
		dayRecords, dayErr := futuresClient.GetIncomeHistory("", dayStartMs, dayEndMs, 1000)
		if dayErr != nil {
			log.Printf("[PNL-SUMMARY] Error fetching day %s records: %v", dayStart.Format("2006-01-02"), dayErr)
		}

		// Log boundary timestamps for debugging
		if len(dayRecords) > 0 {
			firstRec := dayRecords[0]
			lastRec := dayRecords[len(dayRecords)-1]
			firstTime := time.UnixMilli(firstRec.Time).UTC()
			lastTime := time.UnixMilli(lastRec.Time).UTC()
			log.Printf("[PNL-BOUNDARY] Day %s: count=%d, first=%s (type=%s, %.8f), last=%s (type=%s, %.8f)",
				dayStart.Format("2006-01-02"), len(dayRecords),
				firstTime.Format("2006-01-02 15:04:05"), firstRec.IncomeType, firstRec.Income,
				lastTime.Format("2006-01-02 15:04:05"), lastRec.IncomeType, lastRec.Income)
		}

		// Sum PnL for this specific day - track ALL income types
		var dayPnL, dayCommission float64
		var dayFundingReceived, dayFundingPaid float64 // Track separately
		var dayRebate float64                          // Commission rebates (BNB discount, API rebate, etc.)
		var dayOther float64                           // Any other income types
		var dayTradeCount int
		otherTypes := make(map[string]float64) // Track unknown types

		for _, record := range dayRecords {
			switch record.IncomeType {
			case "REALIZED_PNL":
				dayPnL += record.Income
				dayTradeCount++
			case "COMMISSION":
				// Commission is negative (fee paid), store as positive for display
				dayCommission += -record.Income
			case "FUNDING_FEE":
				// Funding fee: positive income = received, negative income = paid
				if record.Income > 0 {
					dayFundingReceived += record.Income // We received funding
				} else {
					dayFundingPaid += -record.Income // We paid funding (make positive)
				}
			case "COMMISSION_REBATE", "API_REBATE", "REFERRAL_KICKBACK":
				// Rebates are positive (money back to us)
				dayRebate += record.Income
			default:
				// Track any other income types we might be missing
				dayOther += record.Income
				otherTypes[record.IncomeType] += record.Income
			}
		}

		// Log any unknown income types
		if len(otherTypes) > 0 {
			log.Printf("[PNL-SUMMARY] Day %s has OTHER income types: %v", dayStart.Format("2006-01-02"), otherTypes)
		}

		// Net funding fee: how much we paid net (positive = net paid, negative = net received)
		dayNetFunding := dayFundingPaid - dayFundingReceived

		// Net PnL = Realized PnL - Commission + Rebates - Net Funding + Other
		// Rebates add to profit (we get money back)
		// Other income types are added directly (could be positive or negative)
		rawNetPnL := dayPnL - dayCommission + dayRebate - dayNetFunding + dayOther

		// Truncate final net PnL to 2 decimal places (matches Binance display)
		// Binance calculates with full precision but truncates for display
		netPnL := math.Trunc(rawNetPnL*100) / 100

		// Log raw vs truncated for verification
		log.Printf("[PNL-DEBUG] Day %s: RAW=%.8f, TRUNC=%.2f", dayStart.Format("2006-01-02"), rawNetPnL, netPnL)

		dailyData[i] = DayData{
			Date:       dayStart.Format("2006-01-02"),
			Day:        dayStart.Day(),
			DayName:    dayStart.Format("Mon"),
			PnL:        dayPnL,
			Commission: dayCommission,
			Rebate:     dayRebate,
			Funding:    dayNetFunding,
			Other:      dayOther,
			NetPnL:     netPnL,
			TradeCount: dayTradeCount,
			IsProfit:   netPnL >= 0,
			IsToday:    dayOffset == 0,
		}

		// Log with precision for debugging
		log.Printf("[PNL-SUMMARY] Day %s [records=%d]: pnl=$%.2f, comm=$%.2f, fund=$%.2f, NET=$%.2f, trades=%d",
			dayStart.Format("2006-01-02"), len(dayRecords), dayPnL, dayCommission, dayNetFunding, netPnL, dayTradeCount)
	}

	// Calculate weekly totals from sum of all 7 daily breakdowns
	// This ensures weekly total matches the sum of the calendar days shown
	var weeklyPnL, weeklyCommission, weeklyFunding, weeklyNetPnL float64
	var weeklyTradeCount int

	for _, day := range dailyData {
		weeklyPnL += day.PnL
		weeklyCommission += day.Commission
		weeklyFunding += day.Funding
		weeklyNetPnL += day.NetPnL
		weeklyTradeCount += day.TradeCount
	}

	// Today's data is the last element (index 6)
	todayData := dailyData[6]
	dailyPnL := todayData.PnL
	dailyCommission := todayData.Commission
	dailyFunding := todayData.Funding
	dailyTradeCount := todayData.TradeCount

	// Convert to gin.H for JSON response
	dailyBreakdown := make([]gin.H, 7)
	for i, day := range dailyData {
		dailyBreakdown[i] = gin.H{
			"date":        day.Date,
			"day":         day.Day,
			"day_name":    day.DayName,
			"pnl":         day.PnL,
			"commission":  day.Commission,
			"rebate":      day.Rebate,
			"funding":     day.Funding,
			"other":       day.Other,
			"net_pnl":     day.NetPnL,
			"trade_count": day.TradeCount,
			"is_profit":   day.IsProfit,
			"is_today":    day.IsToday,
		}
	}

	log.Printf("[PNL-SUMMARY] Results: daily_net=$%.4f (trades=%d), weekly_net=$%.4f (trades=%d)",
		todayData.NetPnL, dailyTradeCount, weeklyNetPnL, weeklyTradeCount)

	// Format reset countdown
	hours := secondsUntilReset / 3600
	minutes := (secondsUntilReset % 3600) / 60
	resetCountdown := fmt.Sprintf("%dh %dm", hours, minutes)

	// Calculate when UTC midnight is in user's local time (for display)
	utcMidnight := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day()+1, 0, 0, 0, 0, time.UTC)
	resetTimeLocal := utcMidnight.In(userLoc).Format("15:04")

	c.JSON(http.StatusOK, gin.H{
		// Daily breakdown (UTC-based, matches Binance)
		"daily_pnl":         dailyPnL,
		"daily_commission":  dailyCommission,
		"daily_funding":     dailyFunding,
		"daily_trade_count": dailyTradeCount,
		"reset_countdown":   resetCountdown,
		"seconds_to_reset":  secondsUntilReset,
		"reset_time_local":  resetTimeLocal, // When daily reset happens in user's timezone

		// Weekly breakdown (last 7 days rolling, UTC-based)
		"weekly_pnl":         weeklyPnL,
		"weekly_commission":  weeklyCommission,
		"weekly_funding":     weeklyFunding,
		"weekly_trade_count": weeklyTradeCount,
		"week_start_date":    weekStartDate,
		"week_end_date":      weekEndDate,
		"week_range":         fmt.Sprintf("%s - %s", weekStartDate, weekEndDate),

		// Per-day breakdown for calendar view (7 boxes)
		"daily_breakdown": dailyBreakdown,

		// Timezone info
		"pnl_timezone":    "UTC",       // PnL calculations use UTC to match Binance
		"timezone":        userTzName,  // User's display timezone
		"timezone_offset": userTzOffset,

		// Fetch timestamp
		"fetched_at": time.Now().In(userLoc).Format(time.RFC3339),
	})
}

// handleTestDailyPnLFromTrades tests daily PnL calculation using userTrades endpoint
// This is a test endpoint to verify real-time PnL from trades vs Income History
// GET /api/futures/test-daily-pnl
func (s *Server) handleTestDailyPnLFromTrades(c *gin.Context) {
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no_api_keys"})
		return
	}

	// Calculate today's UTC boundaries (Binance uses UTC)
	now := time.Now().UTC()
	startOfDayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	startOfDayMs := startOfDayUTC.UnixMilli()

	log.Printf("[TEST-DAILY-PNL] Testing trades-based PnL calculation")
	log.Printf("[TEST-DAILY-PNL] Today UTC: %s, startOfDayMs: %d", startOfDayUTC.Format(time.RFC3339), startOfDayMs)

	// Get symbols that have been traded recently - check positions first
	positions, err := futuresClient.GetPositions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get positions", "details": err.Error()})
		return
	}

	// Collect symbols with non-zero positions or recent activity
	symbolsToCheck := make(map[string]bool)
	for _, pos := range positions {
		if pos.PositionAmt != 0 || pos.UnrealizedProfit != 0 {
			symbolsToCheck[pos.Symbol] = true
		}
	}

	// Also check common symbols if no positions found
	if len(symbolsToCheck) == 0 {
		// Add recently traded symbol from logs
		symbolsToCheck["AAVEUSDT"] = true
	}

	log.Printf("[TEST-DAILY-PNL] Checking symbols: %v", symbolsToCheck)

	// Query trades for each symbol
	var totalDailyPnL float64
	var totalCommission float64
	var tradeCount int
	var tradeDetails []map[string]interface{}

	for symbol := range symbolsToCheck {
		trades, err := futuresClient.GetTradeHistoryByDateRange(symbol, startOfDayMs, 0, 1000)
		if err != nil {
			log.Printf("[TEST-DAILY-PNL] Error fetching trades for %s: %v", symbol, err)
			continue
		}

		for _, trade := range trades {
			// Only count trades from today (double-check)
			if trade.Time >= startOfDayMs {
				totalDailyPnL += trade.RealizedPnl
				totalCommission += trade.Commission
				tradeCount++

				// Log each trade for debugging
				if trade.RealizedPnl != 0 {
					tradeDetails = append(tradeDetails, map[string]interface{}{
						"symbol":      trade.Symbol,
						"side":        trade.Side,
						"qty":         trade.Qty,
						"price":       trade.Price,
						"realizedPnl": trade.RealizedPnl,
						"commission":  trade.Commission,
						"time":        time.UnixMilli(trade.Time).UTC().Format(time.RFC3339),
					})
				}
			}
		}
	}

	// Also get Income History for comparison
	incomeRecords, _ := futuresClient.GetIncomeHistory("REALIZED_PNL", startOfDayMs, 0, 1000)
	var incomeHistoryPnL float64
	for _, record := range incomeRecords {
		if record.Time >= startOfDayMs {
			incomeHistoryPnL += record.Income
		}
	}

	log.Printf("[TEST-DAILY-PNL] Results: trades_pnl=$%.4f, income_history_pnl=$%.4f, trade_count=%d",
		totalDailyPnL, incomeHistoryPnL, tradeCount)

	c.JSON(http.StatusOK, gin.H{
		"test_name": "Daily PnL from User Trades vs Income History",
		"date_utc":  startOfDayUTC.Format("2006-01-02"),
		"current_utc": now.Format(time.RFC3339),

		// From userTrades endpoint (real-time)
		"trades_based": gin.H{
			"daily_pnl":        totalDailyPnL,
			"daily_commission": totalCommission,
			"net_pnl":          totalDailyPnL - totalCommission,
			"trade_count":      tradeCount,
			"source":           "/fapi/v1/userTrades",
		},

		// From income history endpoint (may have delay)
		"income_history_based": gin.H{
			"daily_pnl":     incomeHistoryPnL,
			"record_count":  len(incomeRecords),
			"source":        "/fapi/v1/income",
		},

		// Difference
		"difference": totalDailyPnL - incomeHistoryPnL,

		// Trade details for verification
		"trade_details": tradeDetails,
	})
}

// ==================== PNL HISTORY ENDPOINTS ====================
// Story 13.1: PNL Summary Caching & Historical Navigation

// handleGetPnLHistory returns historical P&L data for extended date ranges
// GET /api/futures/pnl-history?start_date=2026-01-01&end_date=2026-01-18
func (s *Server) handleGetPnLHistory(c *gin.Context) {
	// Use timeout to prevent long-running requests from exhausting resources
	// 120 seconds matches the client-side timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "no_api_keys",
			"message": "Please configure your Binance API keys in Settings",
		})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id_required"})
		return
	}

	// Parse date range from query params
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	// Default to last 30 days if no dates provided
	nowUTC := time.Now().UTC()
	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start_date format, use YYYY-MM-DD"})
			return
		}
	} else {
		startDate = time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day()-29, 0, 0, 0, 0, time.UTC)
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end_date format, use YYYY-MM-DD"})
			return
		}
	} else {
		endDate = time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	}

	// Validate start_date is before or equal to end_date
	if startDate.After(endDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date cannot be after end_date"})
		return
	}

	// Limit to 365 days max for safety
	if endDate.Sub(startDate) > 365*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date range cannot exceed 365 days"})
		return
	}

	log.Printf("[PNL-HISTORY] Fetching for user=%s, range=%s to %s", userID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	// Get user's timezone for display
	userLoc, userTzName, userTzOffset := s.getUserTimezone(ctx, userID)

	// Today's date (UTC) - we never cache today
	todayUTC := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	// Build list of dates to fetch
	var results []database.DailyPnLBreakdown
	var datesToFetchFromBinance []time.Time

	// First, check which dates we have in cache
	cachedData, err := s.repo.GetUserDailyPnLRange(ctx, userID, startDate, endDate)
	if err != nil {
		log.Printf("[PNL-HISTORY] Error getting cached data: %v", err)
		cachedData = []database.UserDailyPnLSummary{}
	}

	// Build a map of cached dates for quick lookup
	cachedMap := make(map[string]*database.UserDailyPnLSummary)
	for i := range cachedData {
		dateStr := cachedData[i].Date.Format("2006-01-02")
		cachedMap[dateStr] = &cachedData[i]
	}

	// Iterate through each day in the range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		isToday := d.Equal(todayUTC)

		if isToday {
			// TODAY: Always fetch live from Binance
			datesToFetchFromBinance = append(datesToFetchFromBinance, d)
		} else if cached, exists := cachedMap[dateStr]; exists {
			// Past day with cache: Use cached data
			breakdown := cached.ToDailyBreakdown(false)
			results = append(results, breakdown)
			log.Printf("[PNL-HISTORY] Day %s: from cache", dateStr)
		} else {
			// Past day without cache: Need to fetch from Binance
			datesToFetchFromBinance = append(datesToFetchFromBinance, d)
		}
	}

	// Fetch missing days from Binance (including today)
	var summariesToSave []database.UserDailyPnLSummary

	for _, d := range datesToFetchFromBinance {
		dateStr := d.Format("2006-01-02")
		isToday := d.Equal(todayUTC)

		// Add small delay between API calls to avoid rate limits
		if len(datesToFetchFromBinance) > 1 {
			time.Sleep(150 * time.Millisecond)
		}

		// Fetch from Binance
		dayData := s.fetchDayPnLFromBinance(futuresClient, d)

		breakdown := database.DailyPnLBreakdown{
			Date:       dateStr,
			Day:        d.Day(),
			DayName:    d.Format("Mon"),
			PnL:        dayData.PnL,
			Commission: dayData.Commission,
			Rebate:     dayData.Rebate,
			Funding:    dayData.Funding,
			Other:      dayData.Other,
			NetPnL:     dayData.NetPnL,
			TradeCount: dayData.TradeCount,
			IsProfit:   dayData.NetPnL >= 0,
			IsToday:    isToday,
			IsCached:   false,
		}
		results = append(results, breakdown)

		// Save to cache if NOT today
		if !isToday {
			summary := database.NewUserDailyPnLSummary(
				userID, d,
				dayData.PnL, dayData.Commission, dayData.Funding,
				dayData.Rebate, dayData.Other, dayData.NetPnL, dayData.TradeCount,
			)
			summariesToSave = append(summariesToSave, *summary)
			log.Printf("[PNL-HISTORY] Day %s: fetched from Binance, will cache", dateStr)
		} else {
			log.Printf("[PNL-HISTORY] Day %s: fetched from Binance (TODAY, not cached)", dateStr)
		}
	}

	// Save fetched historical data to cache
	if len(summariesToSave) > 0 {
		if err := s.repo.BulkSaveUserDailyPnL(ctx, summariesToSave); err != nil {
			log.Printf("[PNL-HISTORY] Error saving to cache: %v", err)
		} else {
			log.Printf("[PNL-HISTORY] Cached %d days", len(summariesToSave))
		}
	}

	// Sort results by date descending (most recent first) using efficient O(n log n) sort
	sort.Slice(results, func(i, j int) bool {
		return results[i].Date > results[j].Date
	})

	// Calculate totals
	var totalPnL, totalCommission, totalFunding, totalNetPnL float64
	var totalTrades int
	for _, r := range results {
		totalPnL += r.PnL
		totalCommission += r.Commission
		totalFunding += r.Funding
		totalNetPnL += r.NetPnL
		totalTrades += r.TradeCount
	}

	c.JSON(http.StatusOK, gin.H{
		"daily_records": results,
		"totals": gin.H{
			"pnl":         totalPnL,
			"commission":  totalCommission,
			"funding":     totalFunding,
			"net_pnl":     totalNetPnL,
			"trade_count": totalTrades,
			"days_count":  len(results),
		},
		"date_range": gin.H{
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Format("2006-01-02"),
		},
		"cache_stats": gin.H{
			"cached_days":  len(cachedData),
			"fetched_days": len(datesToFetchFromBinance),
		},
		"timezone":        userTzName,
		"timezone_offset": userTzOffset,
		"fetched_at":      time.Now().In(userLoc).Format(time.RFC3339),
	})
}

// DayPnLData holds the raw data fetched from Binance for a single day
type DayPnLData struct {
	PnL        float64
	Commission float64
	Rebate     float64
	Funding    float64
	Other      float64
	NetPnL     float64
	TradeCount int
}

// fetchDayPnLFromBinance fetches a single day's P&L data from Binance income history
func (s *Server) fetchDayPnLFromBinance(futuresClient binance.FuturesClient, date time.Time) DayPnLData {
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)
	dayStartMs := dayStart.UnixMilli()
	// Binance API includes records AT endTime (inclusive), so subtract 1ms
	dayEndMs := dayEnd.UnixMilli() - 1

	dayRecords, err := futuresClient.GetIncomeHistory("", dayStartMs, dayEndMs, 1000)
	if err != nil {
		log.Printf("[FETCH-PNL] Error fetching day %s: %v", dayStart.Format("2006-01-02"), err)
		return DayPnLData{}
	}

	var dayPnL, dayCommission float64
	var dayFundingReceived, dayFundingPaid float64
	var dayRebate, dayOther float64
	var dayTradeCount int

	for _, record := range dayRecords {
		switch record.IncomeType {
		case "REALIZED_PNL":
			dayPnL += record.Income
			dayTradeCount++
		case "COMMISSION":
			dayCommission += -record.Income // Store as positive
		case "FUNDING_FEE":
			if record.Income > 0 {
				dayFundingReceived += record.Income
			} else {
				dayFundingPaid += -record.Income
			}
		case "COMMISSION_REBATE", "API_REBATE", "REFERRAL_KICKBACK":
			dayRebate += record.Income
		default:
			dayOther += record.Income
		}
	}

	dayNetFunding := dayFundingPaid - dayFundingReceived
	rawNetPnL := dayPnL - dayCommission + dayRebate - dayNetFunding + dayOther
	// Truncate to 2 decimal places (matches Binance display)
	netPnL := math.Trunc(rawNetPnL*100) / 100

	return DayPnLData{
		PnL:        dayPnL,
		Commission: dayCommission,
		Rebate:     dayRebate,
		Funding:    dayNetFunding,
		Other:      dayOther,
		NetPnL:     netPnL,
		TradeCount: dayTradeCount,
	}
}

// ==================== ORDER CHAIN CACHE ENDPOINTS ====================
// Story 7.20: Order Chain Redis Cache Layer

// handleGetCachedOrderChains returns all active order chains from cache (with PostgreSQL fallback)
// GET /api/futures/order-chains/cached
func (s *Server) handleGetCachedOrderChains(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Try cache first
	if s.orderChainCache != nil && s.orderChainCache.IsHealthy() {
		chains, err := s.orderChainCache.GetAllActiveChainsForUser(ctx, userID)
		if err == nil && chains != nil && len(chains) > 0 {
			c.JSON(http.StatusOK, gin.H{
				"chains": chains,
				"source": "cache",
				"count":  len(chains),
			})
			return
		}
	}

	// Fallback to PostgreSQL
	if s.repo != nil && s.repo.GetDB() != nil {
		chains, err := s.repo.GetDB().GetActiveOrderChains(ctx, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order chains"})
			return
		}

		// Convert to response format
		response := make([]map[string]interface{}, 0, len(chains))
		for _, chain := range chains {
			response = append(response, map[string]interface{}{
				"chainId":            chain.ChainID,
				"userId":             chain.UserID,
				"symbol":             chain.Symbol,
				"side":               chain.Side,
				"modeCode":           chain.ModeCode,
				"status":             string(chain.Status),
				"entryPrice":         chain.EntryPrice,
				"entryQuantity":      chain.EntryQuantity,
				"currentSlPrice":     chain.CurrentSLPrice,
				"currentTpPrice":     chain.CurrentTPPrice,
				"remainingQuantity":  chain.RemainingQuantity,
				"slModificationCount": chain.SLModificationCount,
				"tpModificationCount": chain.TPModificationCount,
				"eventCount":         chain.EventCount,
				"createdAt":          chain.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				"updatedAt":          chain.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}

		// Try to warm cache for future requests
		if s.orderChainCache != nil {
			go func() {
				// Import startup package would create circular dependency
				// So we just log here - the cache warmer will handle bulk operations
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"chains": response,
			"source": "database",
			"count":  len(response),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"chains": []interface{}{},
		"source": "none",
		"count":  0,
	})
}

// handleGetCachedOrderChain returns a single order chain from cache (with PostgreSQL fallback)
// GET /api/futures/order-chains/cached/:chainId
func (s *Server) handleGetCachedOrderChain(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	chainID := c.Param("chainId")
	if chainID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Chain ID is required"})
		return
	}

	// Try cache first
	if s.orderChainCache != nil && s.orderChainCache.IsHealthy() {
		chain, err := s.orderChainCache.GetChainState(ctx, userID, chainID)
		if err == nil && chain != nil {
			c.JSON(http.StatusOK, gin.H{
				"chain":  chain,
				"source": "cache",
			})
			return
		}
	}

	// Fallback to PostgreSQL
	if s.repo != nil && s.repo.GetDB() != nil {
		chain, err := s.repo.GetDB().GetOrderChainByID(ctx, userID, chainID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order chain"})
			return
		}
		if chain == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order chain not found"})
			return
		}

		// Convert to response format
		response := map[string]interface{}{
			"chainId":            chain.ChainID,
			"userId":             chain.UserID,
			"symbol":             chain.Symbol,
			"side":               chain.Side,
			"modeCode":           chain.ModeCode,
			"status":             string(chain.Status),
			"entryPrice":         chain.EntryPrice,
			"entryQuantity":      chain.EntryQuantity,
			"currentSlPrice":     chain.CurrentSLPrice,
			"currentTpPrice":     chain.CurrentTPPrice,
			"remainingQuantity":  chain.RemainingQuantity,
			"slModificationCount": chain.SLModificationCount,
			"tpModificationCount": chain.TPModificationCount,
			"eventCount":         chain.EventCount,
			"createdAt":          chain.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updatedAt":          chain.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		c.JSON(http.StatusOK, gin.H{
			"chain":  response,
			"source": "database",
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Order chain not found"})
}

// handleGetHistoricalOrderChains returns order chains from database with flexible filtering
// Supports status, symbol, mode, and date range filters
// GET /api/futures/order-chains/history
func (s *Server) handleGetHistoricalOrderChains(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.repo == nil || s.repo.GetDB() == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	// Parse query filters
	statusFilter := c.Query("status")   // active, partial, closed, cancelled
	symbolFilter := c.Query("symbol")
	modeFilter := strings.ToUpper(c.Query("mode"))
	dateFromStr := c.Query("dateFrom")  // YYYY-MM-DD
	dateToStr := c.Query("dateTo")      // YYYY-MM-DD
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	// Parse limit and offset
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	// Parse dates
	var dateFrom, dateTo *time.Time
	if dateFromStr != "" {
		if t, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			dateFrom = &t
		}
	}
	if dateToStr != "" {
		if t, err := time.Parse("2006-01-02", dateToStr); err == nil {
			dateTo = &t
		}
	}

	// Build filter
	filter := database.OrderChainFilter{
		UserID:   userID,
		Status:   statusFilter,
		Symbol:   symbolFilter,
		ModeCode: modeFilter,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Limit:    limit,
		Offset:   offset,
	}

	// Query database
	chains, err := s.repo.GetDB().GetOrderChainsWithFilter(ctx, filter)
	if err != nil {
		log.Printf("[ORDER-CHAINS] Failed to get historical order chains: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order chains"})
		return
	}

	// Convert to response format
	response := make([]map[string]interface{}, 0, len(chains))
	for _, chain := range chains {
		chainData := map[string]interface{}{
			"chainId":             chain.ChainID,
			"userId":              chain.UserID,
			"symbol":              chain.Symbol,
			"side":                chain.Side,
			"modeCode":            chain.ModeCode,
			"status":              string(chain.Status),
			"entryPrice":          chain.EntryPrice,
			"entryQuantity":       chain.EntryQuantity,
			"currentSlPrice":      chain.CurrentSLPrice,
			"currentTpPrice":      chain.CurrentTPPrice,
			"remainingQuantity":   chain.RemainingQuantity,
			"slModificationCount": chain.SLModificationCount,
			"tpModificationCount": chain.TPModificationCount,
			"eventCount":          chain.EventCount,
			"realizedPnl":         chain.RealizedPnL,
			"totalFees":           chain.TotalFees,
			"closeReason":         chain.CloseReason,
			"createdAt":           chain.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updatedAt":           chain.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if chain.ClosedAt != nil && !chain.ClosedAt.IsZero() {
			chainData["closedAt"] = chain.ClosedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		response = append(response, chainData)
	}

	c.JSON(http.StatusOK, gin.H{
		"chains": response,
		"count":  len(response),
		"filter": map[string]interface{}{
			"status":   statusFilter,
			"symbol":   symbolFilter,
			"mode":     modeFilter,
			"dateFrom": dateFromStr,
			"dateTo":   dateToStr,
			"limit":    limit,
			"offset":   offset,
		},
	})
}

// handleSyncOrderState reconciles local order chain state with Binance's actual state
// This is used to fix stale orders that were closed externally (e.g., manually on Binance)
// POST /api/futures/order-chains/sync
func (s *Server) handleSyncOrderState(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	log.Printf("[ORDER-SYNC] Starting order state sync for user %s", userID)

	// 1. Get all open orders from Binance (ground truth)
	openOrders, err := futuresClient.GetOpenOrders("")
	if err != nil {
		log.Printf("[ORDER-SYNC] Failed to fetch open orders from Binance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch orders from Binance"})
		return
	}

	// Track which CLIENT ORDER IDs are actually open on Binance
	// This is the key - we match by our client order ID (e.g., "ULT-260202-00008-E"), not Binance's numeric ID
	openClientOrderIDs := make(map[string]bool)
	openOrdersByClientID := make(map[string]binance.FuturesOrder)
	for _, order := range openOrders {
		if order.ClientOrderId != "" {
			openClientOrderIDs[order.ClientOrderId] = true
			openOrdersByClientID[order.ClientOrderId] = order
			log.Printf("[ORDER-SYNC] Open order on Binance: %s (symbol=%s, status=%s)",
				order.ClientOrderId, order.Symbol, order.Status)
		}
	}
	log.Printf("[ORDER-SYNC] Found %d open orders on Binance", len(openOrders))

	// 1b. Get algo/conditional orders (SL/TP) from Binance
	algoOrders, err := futuresClient.GetOpenAlgoOrders("")
	if err != nil {
		algoOrders = []binance.AlgoOrder{}
		log.Printf("[ORDER-SYNC] Failed to fetch algo orders (non-fatal): %v", err)
	}

	// Build set of open algo order client IDs (for SL/TP detection)
	openAlgoClientIDs := make(map[string]bool)
	for _, ao := range algoOrders {
		if ao.ClientAlgoId != "" {
			openAlgoClientIDs[ao.ClientAlgoId] = true
			log.Printf("[ORDER-SYNC] Open algo order on Binance: %s (symbol=%s, type=%s)",
				ao.ClientAlgoId, ao.Symbol, ao.OrderType)
		}
	}
	log.Printf("[ORDER-SYNC] Found %d open algo orders on Binance", len(algoOrders))

	// 2. Get active order chains from our database
	activeChains, err := s.repo.GetDB().GetActiveOrderChains(ctx, userID)
	if err != nil {
		log.Printf("[ORDER-SYNC] Failed to get active chains from database: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chains from database"})
		return
	}
	log.Printf("[ORDER-SYNC] Found %d active chains in database", len(activeChains))

	// 3. Get current positions from Binance (to check if position still exists)
	positions, err := futuresClient.GetPositions()
	positionsBySymbol := make(map[string]bool)
	if err == nil {
		for _, pos := range positions {
			if pos.PositionAmt != 0 {
				positionsBySymbol[pos.Symbol] = true
				log.Printf("[ORDER-SYNC] Active position on Binance: %s (amt=%.4f)", pos.Symbol, pos.PositionAmt)
			}
		}
	}
	log.Printf("[ORDER-SYNC] Found %d active positions on Binance", len(positionsBySymbol))

	// Get ChainEventWriter for proper close cascade (callbacks, broadcast, cache)
	var chainWriter *orders.ChainEventWriter
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			chainWriter = instance.Autopilot.GetChainEventWriter()
		}
	}

	// 4. Find and close stale chains by comparing CLIENT ORDER IDs
	closedChains := 0
	for _, chain := range activeChains {
		// Build the entry client order ID: ChainID + "-E"
		// e.g., "ULT-260202-00008" -> "ULT-260202-00008-E"
		entryClientOrderID := chain.ChainID + "-E"

		log.Printf("[ORDER-SYNC] Checking chain %s (symbol=%s, entryClientID=%s)",
			chain.ChainID, chain.Symbol, entryClientOrderID)

		// Check if entry order is still open on Binance (by client order ID)
		if openClientOrderIDs[entryClientOrderID] {
			log.Printf("[ORDER-SYNC] Chain %s entry order still open on Binance - skipping", chain.ChainID)
			continue
		}

		// Entry order is NOT in open orders
		// Check if position still exists for this symbol
		if positionsBySymbol[chain.Symbol] {
			log.Printf("[ORDER-SYNC] Chain %s: entry not open but position exists for %s - skipping",
				chain.ChainID, chain.Symbol)
			continue
		}

		// Check if chain has active SL or TP algo orders
		slClientID := chain.ChainID + "-SL"
		tpClientID := chain.ChainID + "-TP"
		if openAlgoClientIDs[slClientID] || openAlgoClientIDs[tpClientID] {
			log.Printf("[ORDER-SYNC] Chain %s has active algo order (SL/TP) - skipping", chain.ChainID)
			continue
		}

		// Grace period: don't close chains that became active very recently
		// This prevents race condition with WebSocket SL/TP fill processing
		if chain.EntryFilledAt != nil {
			timeSinceFill := time.Since(*chain.EntryFilledAt)
			if timeSinceFill < 30*time.Second {
				log.Printf("[ORDER-SYNC] Chain %s filled %.0fs ago - skipping reconciliation (grace period)", chain.ChainID, timeSinceFill.Seconds())
				continue
			}
		} else if time.Since(chain.UpdatedAt) < 30*time.Second {
			log.Printf("[ORDER-SYNC] Chain %s updated %.0fs ago - skipping reconciliation (grace period)", chain.ChainID, time.Since(chain.UpdatedAt).Seconds())
			continue
		}

		// Neither entry order open nor position exists - this is a stale chain
		log.Printf("[ORDER-SYNC] Chain %s is STALE - entry not open, no position, no algo orders for %s",
			chain.ChainID, chain.Symbol)

		// Try to get close price from mark price
		var closePrice *float64
		if markPriceData, mpErr := futuresClient.GetMarkPrice(chain.Symbol); mpErr == nil && markPriceData.MarkPrice > 0 {
			mp := markPriceData.MarkPrice
			closePrice = &mp
		} else if chain.CurrentSLPrice != nil && *chain.CurrentSLPrice > 0 {
			closePrice = chain.CurrentSLPrice // fallback to SL price
		} else if chain.EntryPrice != nil && *chain.EntryPrice > 0 {
			closePrice = chain.EntryPrice // fallback to entry price
		}

		// Close the chain using ChainEventWriter for proper cascade
		closeReason := "CLOSED_EXTERNALLY"
		if chainWriter != nil {
			err = chainWriter.CloseChain(ctx, chain.ChainID, closeReason, 0.0, 0.0, closePrice)
		} else {
			err = s.repo.GetDB().CloseOrderChain(ctx, chain.ChainID, closeReason, 0.0, 0.0, closePrice)
		}
		if err != nil {
			log.Printf("[ORDER-SYNC] Failed to close chain %s: %v", chain.ChainID, err)
		} else {
			log.Printf("[ORDER-SYNC] Closed stale chain %s (reason: %s)", chain.ChainID, closeReason)
			closedChains++
		}

		// Update SL/TP status to CANCELED in DB
		s.repo.GetDB().UpdateOrderChainSLCanceled(ctx, chain.ChainID, time.Now())
		s.repo.GetDB().UpdateOrderChainTPCanceled(ctx, chain.ChainID, time.Now())

		// Also close position state if exists
		posState, err := s.repo.GetDB().GetPositionByChainID(ctx, userID, chain.ChainID)
		if err == nil && posState != nil && posState.Status != "CLOSED" {
			posState.Status = "CLOSED"
			now := time.Now()
			posState.ClosedAt = &now
			err = s.repo.GetDB().UpdatePositionState(ctx, posState)
			if err != nil {
				log.Printf("[ORDER-SYNC] Failed to close position state for chain %s: %v", chain.ChainID, err)
			}
		}
	}

	log.Printf("[ORDER-SYNC] Sync complete: closed %d stale chains", closedChains)
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        fmt.Sprintf("Synced order state: closed %d stale chains", closedChains),
		"open_orders":    len(openOrders),
		"algo_orders":    len(algoOrders),
		"active_chains":  len(activeChains),
		"closed_chains":  closedChains,
	})
}

var (
	reconcileLastRun   = make(map[string]time.Time) // userID -> last reconciliation time
	reconcileLastRunMu sync.Mutex
)

// reconcileStaleOrderChains runs in the background to automatically close stale order chains
// Called when user views order chains - compares database state with Binance open orders
func (s *Server) reconcileStaleOrderChains(userID string, openOrders []binance.FuturesOrder, openAlgoOrders []binance.AlgoOrder, positions []binance.FuturesPosition, futuresClient binance.FuturesClient) {
	// Throttle: only run reconciliation once per 60 seconds per user
	reconcileLastRunMu.Lock()
	lastRun, exists := reconcileLastRun[userID]
	if exists && time.Since(lastRun) < 60*time.Second {
		reconcileLastRunMu.Unlock()
		return
	}
	reconcileLastRun[userID] = time.Now()
	reconcileLastRunMu.Unlock()

	ctx := context.Background()

	// Build map of client order IDs that are open on Binance (regular orders)
	openClientOrderIDs := make(map[string]bool)
	for _, order := range openOrders {
		if order.ClientOrderId != "" {
			openClientOrderIDs[order.ClientOrderId] = true
		}
	}

	// Build set of open algo order client IDs (for SL/TP detection)
	openAlgoClientIDs := make(map[string]bool)
	for _, ao := range openAlgoOrders {
		if ao.ClientAlgoId != "" {
			openAlgoClientIDs[ao.ClientAlgoId] = true
		}
	}

	// Use positions passed from caller (already fetched - avoid redundant REST API call)
	positionsBySymbol := make(map[string]bool)
	positionsBySymbolSide := make(map[string]bool) // "SYMBOL:SIDE" -> bool for hedge mode
	for _, pos := range positions {
		if pos.PositionAmt != 0 {
			positionsBySymbol[pos.Symbol] = true
			if pos.PositionSide != "" && pos.PositionSide != "BOTH" {
				key := pos.Symbol + ":" + pos.PositionSide
				positionsBySymbolSide[key] = true
			}
		}
	}

	// Get active order chains from database
	activeChains, err := s.repo.GetDB().GetActiveOrderChains(ctx, userID)
	if err != nil {
		log.Printf("[AUTO-SYNC] Failed to get active chains: %v", err)
		return
	}

	// Get ChainEventWriter for proper close cascade (callbacks, broadcast, cache)
	var chainWriter *orders.ChainEventWriter
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.Autopilot != nil {
			chainWriter = instance.Autopilot.GetChainEventWriter()
		}
	}

	// Find and close stale chains
	closedCount := 0
	for _, chain := range activeChains {
		entryClientOrderID := chain.ChainID + "-E"

		// Check if entry order is open OR position exists (with hedge mode side awareness)
		hasPosition := positionsBySymbol[chain.Symbol]
		if chain.Side != "" && chain.Side != "BOTH" {
			sideKey := chain.Symbol + ":" + chain.Side
			hasPosition = positionsBySymbolSide[sideKey]
		}
		if openClientOrderIDs[entryClientOrderID] || hasPosition {
			continue // Not stale
		}

		// Check if chain has active SL or TP algo orders
		slClientID := chain.ChainID + "-SL"
		tpClientID := chain.ChainID + "-TP"
		if openAlgoClientIDs[slClientID] || openAlgoClientIDs[tpClientID] {
			// SL or TP algo order is still active - chain is NOT closed
			continue
		}

		// Grace period: don't close chains that became active very recently
		// This prevents race condition with WebSocket SL/TP fill processing
		if chain.EntryFilledAt != nil {
			timeSinceFill := time.Since(*chain.EntryFilledAt)
			if timeSinceFill < 120*time.Second {
				log.Printf("[AUTO-SYNC] Chain %s filled %.0fs ago - skipping reconciliation (grace period)", chain.ChainID, timeSinceFill.Seconds())
				continue
			}
		} else if time.Since(chain.UpdatedAt) < 120*time.Second {
			log.Printf("[AUTO-SYNC] Chain %s updated %.0fs ago - skipping reconciliation (grace period)", chain.ChainID, time.Since(chain.UpdatedAt).Seconds())
			continue
		}

		// This chain is stale - close it
		log.Printf("[AUTO-SYNC] Closing stale chain %s (symbol=%s)", chain.ChainID, chain.Symbol)

		// Try to get close price from mark price
		var closePrice *float64
		if markPriceData, mpErr := futuresClient.GetMarkPrice(chain.Symbol); mpErr == nil && markPriceData.MarkPrice > 0 {
			mp := markPriceData.MarkPrice
			closePrice = &mp
		} else if chain.CurrentSLPrice != nil && *chain.CurrentSLPrice > 0 {
			closePrice = chain.CurrentSLPrice
		} else if chain.EntryPrice != nil && *chain.EntryPrice > 0 {
			closePrice = chain.EntryPrice
		}

		if chainWriter != nil {
			err = chainWriter.CloseChain(ctx, chain.ChainID, "CLOSED_EXTERNALLY", 0.0, 0.0, closePrice)
		} else {
			err = s.repo.GetDB().CloseOrderChain(ctx, chain.ChainID, "CLOSED_EXTERNALLY", 0.0, 0.0, closePrice)
		}
		if err != nil {
			log.Printf("[AUTO-SYNC] Failed to close chain %s: %v", chain.ChainID, err)
			continue
		}
		closedCount++

		// Update SL/TP status to CANCELED in DB
		s.repo.GetDB().UpdateOrderChainSLCanceled(ctx, chain.ChainID, time.Now())
		s.repo.GetDB().UpdateOrderChainTPCanceled(ctx, chain.ChainID, time.Now())

		// Also close position state
		posState, err := s.repo.GetDB().GetPositionByChainID(ctx, userID, chain.ChainID)
		if err == nil && posState != nil && posState.Status != "CLOSED" {
			posState.Status = "CLOSED"
			now := time.Now()
			posState.ClosedAt = &now
			s.repo.GetDB().UpdatePositionState(ctx, posState)
		}
	}

	if closedCount > 0 {
		log.Printf("[AUTO-SYNC] Closed %d stale chains for user %s", closedCount, userID)
	}
}

// handleReplaceChainOrders re-places SL and TP algo orders for an active chain.
// This is used when orders were cancelled externally on Binance and need to be restored.
// POST /api/futures/order-chains/:chainId/replace-orders
func (s *Server) handleReplaceChainOrders(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	chainID := c.Param("chainId")
	if chainID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chainId parameter is required"})
		return
	}

	// Parse optional override prices from request body
	type ReplaceOrdersRequest struct {
		SLPrice float64 `json:"sl_price"` // If 0, use current_sl_price from chain
		TPPrice float64 `json:"tp_price"` // If 0, use current_tp_price from chain
	}

	var req ReplaceOrdersRequest
	// Bind JSON body if present (it's optional)
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
			return
		}
	}

	// Look up the chain from DB
	ctx := c.Request.Context()
	chain, err := s.repo.GetDB().GetOrderChainByID(ctx, userID, chainID)
	if err != nil {
		log.Printf("[REPLACE-ORDERS] Error looking up chain %s: %v", chainID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to look up order chain"})
		return
	}
	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("order chain %s not found", chainID)})
		return
	}

	// Accept both ACTIVE and CLOSED chains (CLOSED chains will be re-activated)
	wasClosedChain := !chain.IsActive()

	// Validate chain has entry data
	if chain.EntryPrice == nil || chain.EntryQuantity == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("chain %s missing entry_price or entry_quantity", chainID),
		})
		return
	}

	// Determine SL and TP prices (use overrides if provided, else from chain)
	slPrice := req.SLPrice
	if slPrice == 0 {
		if chain.CurrentSLPrice != nil {
			slPrice = *chain.CurrentSLPrice
		}
	}
	tpPrice := req.TPPrice
	if tpPrice == 0 {
		if chain.CurrentTPPrice != nil {
			tpPrice = *chain.CurrentTPPrice
		}
	}

	if slPrice == 0 && tpPrice == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "no SL or TP price available (provide in request body or ensure chain has prices in DB)",
		})
		return
	}

	// Get Binance futures client
	futuresClient := s.getFuturesClientForUser(c)
	if futuresClient == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no Binance API keys configured"})
		return
	}

	// Determine order parameters based on position side
	// For closing a position: SHORT position closes with BUY, LONG position closes with SELL
	var closeSide string
	var positionSide binance.PositionSide
	if chain.Side == "SHORT" {
		closeSide = "BUY"
		positionSide = binance.PositionSideShort
	} else {
		closeSide = "SELL"
		positionSide = binance.PositionSideLong
	}

	quantity := *chain.EntryQuantity
	if chain.RemainingQuantity != nil && *chain.RemainingQuantity > 0 {
		quantity = *chain.RemainingQuantity
	}

	// Get precision from exchange info
	pricePrecision := 8
	qtyPrecision := 8
	exchangeInfo, err := futuresClient.GetFuturesExchangeInfo()
	if err == nil && exchangeInfo != nil {
		for _, sym := range exchangeInfo.Symbols {
			if sym.Symbol == chain.Symbol {
				pricePrecision = sym.PricePrecision
				qtyPrecision = sym.QuantityPrecision
				break
			}
		}
	} else {
		log.Printf("[REPLACE-ORDERS] Warning: could not get exchange info, using default precision: %v", err)
	}

	// Generate client order IDs using chain ID format
	slClientOrderID := fmt.Sprintf("%s-SL", chainID)
	tpClientOrderID := fmt.Sprintf("%s-TP", chainID)

	var tpResp, slResp *binance.AlgoOrderResponse
	var tpError, slError string

	// Place TP algo order (TAKE_PROFIT_MARKET - executes as MARKET order when trigger hits, always fills)
	if tpPrice > 0 {
		tpParams := binance.AlgoOrderParams{
			Symbol:            chain.Symbol,
			Side:              closeSide,
			PositionSide:      positionSide,
			Type:              binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:          quantity,
			TriggerPrice:      tpPrice,
			ClosePosition:     false,
			WorkingType:       binance.WorkingTypeMarkPrice,
			ClientAlgoId:      tpClientOrderID,
			PricePrecision:    pricePrecision,
			QuantityPrecision: qtyPrecision,
		}

		log.Printf("[REPLACE-ORDERS] Placing TAKE_PROFIT_MARKET for chain %s: symbol=%s, side=%s, triggerPrice=%.6f, qty=%.6f",
			chainID, chain.Symbol, closeSide, tpPrice, quantity)

		tpResp, err = futuresClient.PlaceAlgoOrder(tpParams)
		if err != nil {
			tpError = err.Error()
			log.Printf("[REPLACE-ORDERS] Failed to place TP order for chain %s: %v", chainID, err)
		} else {
			log.Printf("[REPLACE-ORDERS] TP order placed: chain=%s, algoID=%d, price=%.6f", chainID, tpResp.AlgoId, tpPrice)

			// Update TP details in DB
			if err := s.repo.GetDB().UpdateOrderChainTPDetails(ctx, chainID, tpResp.AlgoId, tpPrice, quantity); err != nil {
				log.Printf("[REPLACE-ORDERS] Warning: failed to persist TP details: %v", err)
			}
		}
	}

	// Place SL algo order (STOP_MARKET - executes as MARKET order when trigger hits, always fills)
	if slPrice > 0 {
		slParams := binance.AlgoOrderParams{
			Symbol:            chain.Symbol,
			Side:              closeSide,
			PositionSide:      positionSide,
			Type:              binance.FuturesOrderTypeStopMarket,
			Quantity:          quantity,
			TriggerPrice:      slPrice,
			ClosePosition:     false,
			WorkingType:       binance.WorkingTypeMarkPrice,
			ClientAlgoId:      slClientOrderID,
			PricePrecision:    pricePrecision,
			QuantityPrecision: qtyPrecision,
		}

		log.Printf("[REPLACE-ORDERS] Placing STOP_MARKET SL for chain %s: symbol=%s, side=%s, triggerPrice=%.6f, qty=%.6f",
			chainID, chain.Symbol, closeSide, slPrice, quantity)

		slResp, err = futuresClient.PlaceAlgoOrder(slParams)
		if err != nil {
			slError = err.Error()
			log.Printf("[REPLACE-ORDERS] Failed to place SL order for chain %s: %v", chainID, err)
		} else {
			log.Printf("[REPLACE-ORDERS] SL order placed: chain=%s, algoID=%d, price=%.6f", chainID, slResp.AlgoId, slPrice)

			// Update SL details in DB and increment modification count
			if err := s.repo.GetDB().UpdateOrderChainSLDetails(ctx, chainID, slResp.AlgoId, slPrice, quantity); err != nil {
				log.Printf("[REPLACE-ORDERS] Warning: failed to persist SL details: %v", err)
			}
			// Update current_sl_price and increment sl_modification_count
			if err := s.repo.GetDB().UpdateOrderChainSLPrice(ctx, chainID, slPrice); err != nil {
				log.Printf("[REPLACE-ORDERS] Warning: failed to update SL price: %v", err)
			}
		}
	}

	// Update current TP price in DB if TP was placed successfully
	if tpResp != nil && tpPrice > 0 {
		if err := s.repo.GetDB().UpdateOrderChainTPPrice(ctx, chainID, tpPrice); err != nil {
			log.Printf("[REPLACE-ORDERS] Warning: failed to update TP price: %v", err)
		}
	}

	// Re-activate chain if it was closed (atomic: orders placed first, then status updated)
	if wasClosedChain && (tpResp != nil || slResp != nil) {
		if err := s.repo.GetDB().ReactivateOrderChain(ctx, chainID); err != nil {
			log.Printf("[REPLACE-ORDERS] Warning: failed to re-activate chain %s: %v", chainID, err)
		} else {
			log.Printf("[REPLACE-ORDERS] Chain %s re-activated (was %s)", chainID, chain.Status)
		}
	}

	// Build response
	response := gin.H{
		"success":  tpError == "" || slError == "",
		"chain_id": chainID,
		"symbol":   chain.Symbol,
		"side":     chain.Side,
		"quantity": quantity,
	}

	if tpResp != nil {
		response["tp_order"] = gin.H{
			"algo_id": tpResp.AlgoId,
			"price":   tpPrice,
			"status":  tpResp.AlgoStatus,
		}
	}
	if tpError != "" {
		response["tp_error"] = tpError
	}

	if slResp != nil {
		response["sl_order"] = gin.H{
			"algo_id": slResp.AlgoId,
			"price":   slPrice,
			"status":  slResp.AlgoStatus,
		}
	}
	if slError != "" {
		response["sl_error"] = slError
	}

	// Broadcast order update to WebSocket clients
	events.BroadcastOrderUpdate(userID, map[string]interface{}{
		"action":   "replace_orders",
		"chain_id": chainID,
		"symbol":   chain.Symbol,
		"type":     "algo",
	})

	statusCode := http.StatusOK
	if tpError != "" && slError != "" {
		statusCode = http.StatusInternalServerError
	}

	c.JSON(statusCode, response)
}
