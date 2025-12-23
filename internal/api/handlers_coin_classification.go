package api

import (
	"binance-trading-bot/internal/autopilot"
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetCoinClassifications returns all coin classifications
func (s *Server) handleGetCoinClassifications(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	classifications := classifier.GetAllClassifications()

	// Convert to slice for JSON response
	result := make([]autopilot.CoinClassification, 0, len(classifications))
	for _, c := range classifications {
		result = append(result, *c)
	}

	c.JSON(http.StatusOK, gin.H{
		"classifications": result,
		"settings":        classifier.GetSettings(),
	})
}

// handleGetCoinClassificationSummary returns classification summary
func (s *Server) handleGetCoinClassificationSummary(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	summary := classifier.GetSummary()
	if summary == nil {
		c.JSON(http.StatusOK, gin.H{
			"message": "Classifications not yet loaded",
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// UpdateCoinPreferenceRequest is the request body for updating coin preference
type UpdateCoinPreferenceRequest struct {
	Symbol   string `json:"symbol" binding:"required"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// handleUpdateCoinPreference updates preference for a specific coin
func (s *Server) handleUpdateCoinPreference(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	var req UpdateCoinPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	classifier.UpdateCoinPreference(req.Symbol, req.Enabled, req.Priority)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin preference updated",
		"symbol":  req.Symbol,
		"enabled": req.Enabled,
	})
}

// UpdateCategoryAllocationRequest is the request body for updating category allocation
type UpdateCategoryAllocationRequest struct {
	Category          string  `json:"category" binding:"required"` // e.g., "volatility:stable", "market_cap:blue_chip"
	Enabled           bool    `json:"enabled"`
	AllocationPercent float64 `json:"allocation_percent"`
	MaxPositions      int     `json:"max_positions"`
}

// handleUpdateCategoryAllocation updates allocation for a category
func (s *Server) handleUpdateCategoryAllocation(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	var req UpdateCategoryAllocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	alloc := &autopilot.CategoryAllocation{
		Enabled:           req.Enabled,
		AllocationPercent: req.AllocationPercent,
		MaxPositions:      req.MaxPositions,
	}

	// Parse category type:class format
	// e.g., "volatility:stable", "market_cap:blue_chip", "momentum:gainer"
	switch {
	case len(req.Category) > 11 && req.Category[:11] == "volatility:":
		class := autopilot.VolatilityClass(req.Category[11:])
		classifier.UpdateVolatilityAllocation(class, alloc)
	case len(req.Category) > 11 && req.Category[:11] == "market_cap:":
		class := autopilot.MarketCapClass(req.Category[11:])
		classifier.UpdateMarketCapAllocation(class, alloc)
	case len(req.Category) > 9 && req.Category[:9] == "momentum:":
		class := autopilot.MomentumClass(req.Category[9:])
		classifier.UpdateMomentumAllocation(class, alloc)
	default:
		errorResponse(c, http.StatusBadRequest, "Invalid category format. Use volatility:class, market_cap:class, or momentum:class")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Category allocation updated",
		"category": req.Category,
	})
}

// handleGetTradingStyle returns the current trading style
func (s *Server) handleGetTradingStyle(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	style := controller.GetTradingStyle()
	config := controller.GetStyleConfig()

	c.JSON(http.StatusOK, gin.H{
		"style":  style,
		"config": config,
	})
}

// SetTradingStyleRequest is the request body for setting trading style
type SetTradingStyleRequest struct {
	Style string `json:"style" binding:"required"` // "scalping", "swing", "position"
}

// handleSetTradingStyle sets the trading style
func (s *Server) handleSetTradingStyle(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	var req SetTradingStyleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	var style autopilot.TradingStyle
	switch req.Style {
	case "scalping":
		style = autopilot.StyleScalping
	case "swing":
		style = autopilot.StyleSwing
	case "position":
		style = autopilot.StylePosition
	default:
		errorResponse(c, http.StatusBadRequest, "Invalid style. Use 'scalping', 'swing', or 'position'")
		return
	}

	controller.SetTradingStyle(style)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Trading style updated",
		"style":   req.Style,
		"config":  controller.GetStyleConfig(),
	})
}

// handleGetCoinPreferences returns all coin preferences
func (s *Server) handleGetCoinPreferences(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	settings := classifier.GetSettings()
	classifications := classifier.GetAllClassifications()

	// Build a comprehensive list of all coins with their preferences
	type CoinInfo struct {
		Symbol      string                       `json:"symbol"`
		Enabled     bool                         `json:"enabled"`
		Priority    int                          `json:"priority"`
		Volatility  string                       `json:"volatility,omitempty"`
		MarketCap   string                       `json:"market_cap,omitempty"`
		Momentum    string                       `json:"momentum,omitempty"`
		ATRPercent  float64                      `json:"atr_percent,omitempty"`
		Change24h   float64                      `json:"change_24h,omitempty"`
	}

	coins := make([]CoinInfo, 0)
	for symbol, class := range classifications {
		enabled := true // Default enabled
		priority := 0

		if pref, exists := settings.CoinPreferences[symbol]; exists {
			enabled = pref.Enabled
			priority = pref.Priority
		}

		coins = append(coins, CoinInfo{
			Symbol:     symbol,
			Enabled:    enabled,
			Priority:   priority,
			Volatility: string(class.Volatility),
			MarketCap:  string(class.MarketCap),
			Momentum:   string(class.Momentum),
			ATRPercent: class.VolatilityATR,
			Change24h:  class.Momentum24hPct,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"coins":    coins,
		"total":    len(coins),
		"settings": settings,
	})
}

// BulkCoinPreferenceRequest is the request body for bulk updating coin preferences
type BulkCoinPreferenceRequest struct {
	Coins []struct {
		Symbol   string `json:"symbol"`
		Enabled  bool   `json:"enabled"`
		Priority int    `json:"priority"`
	} `json:"coins"`
}

// handleBulkUpdateCoinPreferences bulk updates coin preferences
func (s *Server) handleBulkUpdateCoinPreferences(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	var req BulkCoinPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body: "+err.Error())
		return
	}

	updated := 0
	for _, coin := range req.Coins {
		classifier.UpdateCoinPreference(coin.Symbol, coin.Enabled, coin.Priority)
		updated++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin preferences updated",
		"updated": updated,
	})
}

// handleGetEligibleCoins returns only coins eligible for trading based on preferences and allocations
func (s *Server) handleGetEligibleCoins(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	eligibleSymbols := classifier.GetEligibleSymbols()

	// Get detailed info for eligible coins
	classifications := classifier.GetAllClassifications()
	settings := classifier.GetSettings()

	type EligibleCoin struct {
		Symbol      string  `json:"symbol"`
		Priority    int     `json:"priority"`
		Volatility  string  `json:"volatility"`
		MarketCap   string  `json:"market_cap"`
		Momentum    string  `json:"momentum"`
		ATRPercent  float64 `json:"atr_percent"`
		Change24h   float64 `json:"change_24h"`
	}

	eligible := make([]EligibleCoin, 0)
	for _, symbol := range eligibleSymbols {
		class, exists := classifications[symbol]
		if !exists {
			continue
		}

		priority := 0
		if pref, exists := settings.CoinPreferences[symbol]; exists {
			priority = pref.Priority
		}

		eligible = append(eligible, EligibleCoin{
			Symbol:     symbol,
			Priority:   priority,
			Volatility: string(class.Volatility),
			MarketCap:  string(class.MarketCap),
			Momentum:   string(class.Momentum),
			ATRPercent: class.VolatilityATR,
			Change24h:  class.Momentum24hPct,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"eligible_coins": eligible,
		"total":          len(eligible),
	})
}

// handleRefreshCoinClassifications manually triggers a refresh of coin classifications
func (s *Server) handleRefreshCoinClassifications(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	// Trigger refresh in background
	go classifier.RefreshAllClassifications()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin classification refresh triggered",
	})
}

// handleEnableAllCoins enables all coins for trading
func (s *Server) handleEnableAllCoins(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	classifications := classifier.GetAllClassifications()
	enabled := 0
	for symbol := range classifications {
		classifier.UpdateCoinPreference(symbol, true, 0)
		enabled++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All coins enabled",
		"enabled": enabled,
	})
}

// handleDisableAllCoins disables all coins for trading
func (s *Server) handleDisableAllCoins(c *gin.Context) {
	controller := s.getFuturesAutopilot()
	if controller == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Futures controller not initialized")
		return
	}

	classifier := controller.GetCoinClassifier()
	if classifier == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Coin classifier not initialized")
		return
	}

	classifications := classifier.GetAllClassifications()
	disabled := 0
	for symbol := range classifications {
		classifier.UpdateCoinPreference(symbol, false, 0)
		disabled++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All coins disabled",
		"disabled": disabled,
	})
}
