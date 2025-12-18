package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

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
	futuresClient := s.getFuturesClient()
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
		return
	}

	accountInfo, err := futuresClient.GetFuturesAccountInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, accountInfo)
}

// handleGetFuturesPositions returns all futures positions
func (s *Server) handleGetFuturesPositions(c *gin.Context) {
	futuresClient := s.getFuturesClient()
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

	c.JSON(http.StatusOK, activePositions)
}

// handleSetLeverage sets leverage for a symbol
func (s *Server) handleSetLeverage(c *gin.Context) {
	var req SetLeverageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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
	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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

	// Place TP/SL orders if specified
	var tpOrderResp, slOrderResp *binance.FuturesOrderResponse

	if req.TakeProfit > 0 {
		tpParams := binance.FuturesOrderParams{
			Symbol:       req.Symbol,
			Side:         getOppositeSide(req.Side),
			PositionSide: binance.PositionSide(req.PositionSide),
			Type:         binance.FuturesOrderTypeTakeProfitMarket,
			Quantity:     req.Quantity,
			StopPrice:    req.TakeProfit,
			ReduceOnly:   true,
			WorkingType:  binance.WorkingTypeMarkPrice,
		}
		tpOrderResp, _ = futuresClient.PlaceFuturesOrder(tpParams)
	}

	if req.StopLoss > 0 {
		slParams := binance.FuturesOrderParams{
			Symbol:       req.Symbol,
			Side:         getOppositeSide(req.Side),
			PositionSide: binance.PositionSide(req.PositionSide),
			Type:         binance.FuturesOrderTypeStopMarket,
			Quantity:     req.Quantity,
			StopPrice:    req.StopLoss,
			ReduceOnly:   true,
			WorkingType:  binance.WorkingTypeMarkPrice,
		}
		slOrderResp, _ = futuresClient.PlaceFuturesOrder(slParams)
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

	c.JSON(http.StatusOK, gin.H{
		"order":      orderResp,
		"takeProfit": tpOrderResp,
		"stopLoss":   slOrderResp,
		"tradeId":    trade.ID,
	})
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

	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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

// handleCloseFuturesPosition closes a futures position
func (s *Server) handleCloseFuturesPosition(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClient()
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
	params := binance.FuturesOrderParams{
		Symbol:       symbol,
		Side:         side,
		PositionSide: binance.PositionSide(position.PositionSide),
		Type:         binance.FuturesOrderTypeMarket,
		Quantity:     quantity,
		ReduceOnly:   true,
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

// handleGetFundingRate returns the current funding rate
func (s *Server) handleGetFundingRate(c *gin.Context) {
	symbol := c.Param("symbol")

	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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
	futuresClient := s.getFuturesClient()
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

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	ctx := context.Background()
	trades, err := s.repo.GetDB().GetFuturesTradeHistory(ctx, limit, offset)
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

// handleGetFuturesMetrics returns futures trading metrics
func (s *Server) handleGetFuturesMetrics(c *gin.Context) {
	ctx := context.Background()
	metrics, err := s.repo.GetDB().GetFuturesTradingMetrics(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, metrics)
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

	futuresClient := s.getFuturesClient()
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

	futuresClient := s.getFuturesClient()
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
	// Check if we're in dry run mode via settings API
	settingsAPI := s.getSettingsAPI()
	isSimulated := true
	if settingsAPI != nil {
		isSimulated = settingsAPI.GetDryRunMode()
	}

	futuresClient := s.getFuturesClient()
	if futuresClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Futures trading not enabled"})
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
	futuresClient := s.getFuturesClient()
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
		params := binance.FuturesOrderParams{
			Symbol:       position.Symbol,
			Side:         side,
			PositionSide: binance.PositionSide(position.PositionSide),
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     quantity,
			ReduceOnly:   true,
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

// ==================== HELPER FUNCTIONS ====================

// getFuturesClient returns the futures client from botAPI if available
func (s *Server) getFuturesClient() binance.FuturesClient {
	if futuresAPI, ok := s.botAPI.(FuturesAPI); ok {
		return futuresAPI.GetFuturesClient()
	}
	return nil
}

// getOppositeSide returns the opposite side for TP/SL orders
func getOppositeSide(side string) string {
	if side == "BUY" {
		return "SELL"
	}
	return "BUY"
}
