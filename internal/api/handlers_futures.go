package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// ==================== INPUT VALIDATION HELPERS ====================
// Commercial-grade validation for trading API inputs

var (
	// symbolRegex validates trading pair format (alphanumeric, 2-20 chars, typically ends in USDT/BUSD)
	symbolRegex = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)
)

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

	// FIXED: Get symbol-level custom ROI from Settings using database call, not GetDefaultSettings()
	// This requires user authentication to load from database
	userID := s.getUserID(c)
	if userID != "" {
		settingsManager := autopilot.GetSettingsManager()
		if settingsManager != nil {
			settings, loadErr := settingsManager.LoadSettingsFromDB(c.Request.Context(), s.repo, userID)
			if loadErr != nil {
				// Log but don't fail - this is optional enrichment
				log.Printf("[FUTURES-POS] WARNING: Failed to load settings for custom ROI enrichment: %v", loadErr)
			} else if settings != nil && settings.SymbolSettings != nil {
				for symbol, symbolSettings := range settings.SymbolSettings {
					// Only add symbol-level ROI if we don't already have position-level ROI
					if symbolSettings != nil && symbolSettings.CustomROIPercent > 0 {
						if _, exists := customROIMap[symbol]; !exists {
							customROIMap[symbol] = symbolSettings.CustomROIPercent
						}
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

	// Calculate time boundaries for daily PnL (UTC-based)
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
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
		"dailyResetTime":    endOfDay.UnixMilli(),                   // Next daily reset (UTC midnight)
		"weeklyStartDate":   startOfWeek.Format("2006-01-02"),       // Week start date
		"weeklyEndDate":     startOfDay.Format("2006-01-02"),        // Week end date (today)
		"serverTimeUTC":     now.UnixMilli(),                        // Current server time
		"timezoneOffset":    0,                                      // UTC offset (0 for UTC-based calculation)

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

	// Calculate time boundaries
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
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
			// Try mainnet first, then testnet
			keys, err := s.apiKeyService.GetActiveBinanceKey(ctx, userID, false)
			if err != nil {
				log.Printf("[DEBUG] getFuturesClientForUser: mainnet key lookup failed: %v, trying testnet", err)
				// Try testnet
				keys, err = s.apiKeyService.GetActiveBinanceKey(ctx, userID, true)
			}
			if err == nil && keys != nil && keys.APIKey != "" && keys.SecretKey != "" {
				log.Printf("[DEBUG] getFuturesClientForUser: Found user keys, creating client (testnet=%v, keyLen=%d)", keys.IsTestnet, len(keys.APIKey))
				// Create user-specific futures client
				client := binance.NewFuturesClient(keys.APIKey, keys.SecretKey, keys.IsTestnet)
				if client != nil {
					return client
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
// Story: User timezone in global_trading settings
// =====================================================

// getUserTimezone loads the user's timezone from global_trading settings
// Falls back to UTC if not set or on error
func (s *Server) getUserTimezone(ctx context.Context, userID string) (*time.Location, string, string) {
	// Default values
	tzName := "UTC"
	tzOffset := "+00:00"

	// Try to get user's timezone from global_trading settings
	if s.settingsCacheService != nil {
		globalTrading, err := s.settingsCacheService.GetGlobalTrading(ctx, userID)
		if err == nil && globalTrading != nil {
			if globalTrading.Timezone != "" {
				tzName = globalTrading.Timezone
			}
			if globalTrading.TimezoneOffset != "" {
				tzOffset = globalTrading.TimezoneOffset
			}
		}
	}

	// Load the timezone location
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		log.Printf("[USER-TIMEZONE] Failed to load timezone %s for user %s: %v, using UTC", tzName, userID, err)
		return time.UTC, "UTC", "+00:00"
	}

	return loc, tzName, tzOffset
}

// getStartOfDayForUser returns the start of the current day in user's timezone as Unix milliseconds
func getStartOfDayForUser(loc *time.Location) int64 {
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	return startOfDay.UnixMilli()
}

// getWeekStartForUser returns the start of the week (Thursday) in user's timezone
func getWeekStartForUser(loc *time.Location) (int64, string, string) {
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

// getTimeUntilMidnightForUser returns seconds until midnight in user's timezone
func getTimeUntilMidnightForUser(loc *time.Location) int64 {
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
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

	// Get user ID for per-user timezone
	userID := c.GetString("userID")

	// Get user's timezone from global_trading settings
	userLoc, userTzName, userTzOffset := s.getUserTimezone(c.Request.Context(), userID)

	// Get time boundaries using user's timezone
	startOfDayMs := getStartOfDayForUser(userLoc)
	weekStartMs, weekStartDate, weekEndDate := getWeekStartForUser(userLoc)
	secondsUntilReset := getTimeUntilMidnightForUser(userLoc)

	// Fetch all income types from Binance (empty string = all types)
	allRecords, err := futuresClient.GetIncomeHistory("", weekStartMs, 0, 1000)
	if err != nil {
		log.Printf("[PNL-SUMMARY] Error fetching income history: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "fetch_failed",
			"message": "Failed to fetch income history from Binance",
		})
		return
	}

	// Calculate values by income type for daily and weekly
	var dailyPnL, dailyCommission, dailyFunding float64
	var weeklyPnL, weeklyCommission, weeklyFunding float64
	var dailyTradeCount, weeklyTradeCount int

	for _, record := range allRecords {
		isToday := record.Time >= startOfDayMs

		switch record.IncomeType {
		case "REALIZED_PNL":
			weeklyPnL += record.Income
			weeklyTradeCount++
			if isToday {
				dailyPnL += record.Income
				dailyTradeCount++
			}
		case "COMMISSION":
			// Commission is negative (fee paid), we store as positive for display
			weeklyCommission += -record.Income
			if isToday {
				dailyCommission += -record.Income
			}
		case "FUNDING_FEE":
			// Funding fee can be positive (received) or negative (paid)
			// We store as paid amount (positive = paid, negative = received)
			weeklyFunding += -record.Income
			if isToday {
				dailyFunding += -record.Income
			}
		}
	}

	// Format reset countdown
	hours := secondsUntilReset / 3600
	minutes := (secondsUntilReset % 3600) / 60
	resetCountdown := fmt.Sprintf("%dh %dm", hours, minutes)

	c.JSON(http.StatusOK, gin.H{
		// Daily breakdown
		"daily_pnl":         dailyPnL,
		"daily_commission":  dailyCommission,
		"daily_funding":     dailyFunding,
		"daily_trade_count": dailyTradeCount,
		"reset_countdown":   resetCountdown,
		"seconds_to_reset":  secondsUntilReset,

		// Weekly breakdown
		"weekly_pnl":         weeklyPnL,
		"weekly_commission":  weeklyCommission,
		"weekly_funding":     weeklyFunding,
		"weekly_trade_count": weeklyTradeCount,
		"week_start_date":    weekStartDate,
		"week_end_date":      weekEndDate,
		"week_range":         fmt.Sprintf("%s - %s", weekStartDate, weekEndDate),

		// Timezone info (from user's global_trading settings)
		"timezone":        userTzName,
		"timezone_offset": userTzOffset,

		// Fetch timestamp
		"fetched_at": time.Now().In(userLoc).Format(time.RFC3339),
	})
}
