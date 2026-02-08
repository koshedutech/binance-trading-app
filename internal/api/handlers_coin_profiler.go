package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/orders"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// Epic 14: Coin Profiler API Endpoints
// Story 14.6: API Endpoints for Coin Profiler
// ============================================================================
// This file provides REST API endpoints for the Coin Profiler service.
// The Coin Profiler collects real-time WebSocket data for the Chain Trading System.
//
// Endpoints:
// - GET  /api/futures/coin-profiler/status       - Get profiler status
// - GET  /api/futures/coin-profiler/coins        - Get all tracked coins
// - GET  /api/futures/coin-profiler/coins/:symbol - Get specific coin data
// - GET  /api/futures/coin-profiler/requirements - Get aggregated requirements
// - POST /api/futures/coin-profiler/start        - Start profiler
// - POST /api/futures/coin-profiler/stop         - Stop profiler
// ============================================================================

// handleGetCoinProfilerStatus returns the current status of the Coin Profiler.
// GET /api/futures/coin-profiler/status
func (s *Server) handleGetCoinProfilerStatus(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	status := s.userAutopilotManager.GetCoinProfilerStatus(userID)
	c.JSON(http.StatusOK, status)
}

// handleGetCoinProfilerCoins returns all tracked coins with current data.
// GET /api/futures/coin-profiler/coins
func (s *Server) handleGetCoinProfilerCoins(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusOK, gin.H{
			"coins": []interface{}{},
			"total": 0,
		})
		return
	}

	allCoinData := profiler.GetAllCoinData()

	// Get active positions to dynamically update source
	// Check BOTH Ginie positions AND Chain Entry order chains
	activePositionSymbols := make(map[string]bool)
	// Track timeframes from active chains (for position-source coins)
	activeChainTimeframes := make(map[string]string) // symbol -> timeframe

	// Source 1: Ginie Autopilot positions
	if controller := s.getFuturesAutopilot(); controller != nil {
		if autopilot := controller.GetGinieAutopilot(); autopilot != nil {
			for _, pos := range autopilot.GetPositions() {
				if pos.RemainingQty > 0 {
					activePositionSymbols[pos.Symbol] = true
				}
			}
		}
	}

	// Source 2: Chain Entry order chains (from database)
	// Use the manager's chain event writer directly - the per-instance GinieAutopilot
	// may not have it set if it was created before the chain event writer was initialized.
	if s.userAutopilotManager != nil {
		chainWriter := s.userAutopilotManager.GetChainEventWriter()
		if chainWriter != nil {
			ctx := c.Request.Context()
			chains, err := chainWriter.GetActiveChains(ctx, userID)
			if err == nil {
				for _, chain := range chains {
					activePositionSymbols[chain.Symbol] = true
					if chain.Timeframe != "" {
						activeChainTimeframes[chain.Symbol] = chain.Timeframe
					}
				}
			}
		}
	}

	// Convert map to slice and enrich source based on active positions
	coins := make([]map[string]interface{}, 0, len(allCoinData)+len(activePositionSymbols))
	processedSymbols := make(map[string]bool)

	for _, coin := range allCoinData {
		processedSymbols[coin.Symbol] = true

		// Create a copy of the coin data as a map
		coinMap := map[string]interface{}{
			"symbol":     coin.Symbol,
			"price":      coin.Price,
			"volume_24h": coin.Volume24h,
			"volatility": coin.Volatility,
			"timeframes": coin.Timeframes,
			"strategies": coin.Strategies,
			"updated_at": coin.UpdatedAt,
		}

		// Dynamically determine source based on active positions
		if activePositionSymbols[coin.Symbol] {
			if coin.Source == coinprofiler.DataSourceStrategy {
				coinMap["source"] = "both" // Has both strategy and position
			} else {
				coinMap["source"] = "position"
			}
			// Include position's entry timeframe if available
			if tf, ok := activeChainTimeframes[coin.Symbol]; ok {
				coinMap["position_timeframe"] = tf
			}
		} else {
			coinMap["source"] = string(coin.Source) // Keep original source (strategy)
		}

		coins = append(coins, coinMap)
	}

	// CRITICAL: Add position coins that aren't tracked by the profiler
	// This ensures positions always appear even if not in any strategy's watchlist
	for symbol := range activePositionSymbols {
		if !processedSymbols[symbol] {
			// Position exists but coin not tracked by profiler - add it
			coinMap := map[string]interface{}{
				"symbol":     symbol,
				"price":      0, // Will be updated via WebSocket
				"volume_24h": 0,
				"volatility": 0,
				"timeframes": map[string]interface{}{},
				"strategies": []interface{}{},
				"source":     "position",
				"updated_at": "",
			}
			if tf, ok := activeChainTimeframes[symbol]; ok {
				coinMap["position_timeframe"] = tf
			}
			coins = append(coins, coinMap)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"coins": coins,
		"total": len(coins),
	})
}

// handleGetCoinProfilerCoin returns data for a specific coin/symbol.
// GET /api/futures/coin-profiler/coins/:symbol
func (s *Server) handleGetCoinProfilerCoin(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": "Symbol parameter is required",
		})
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "Coin profiler not initialized",
		})
		return
	}

	coinData := profiler.GetCoinData(symbol)
	if coinData == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "No data found for symbol: " + symbol,
		})
		return
	}

	c.JSON(http.StatusOK, coinData)
}

// handleGetCoinProfilerRequirements returns the current aggregated data requirements.
// This shows what timeframes and data fields are needed based on enabled strategies and positions.
// GET /api/futures/coin-profiler/requirements
func (s *Server) handleGetCoinProfilerRequirements(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusOK, gin.H{
			"all_timeframes":    []string{},
			"all_data_fields":   []string{},
			"all_symbols":       []string{},
			"strategy_count":    0,
			"position_count":    0,
			"from_strategies":   []interface{}{},
			"from_positions":    []interface{}{},
			"subscriptions":     map[string]interface{}{},
		})
		return
	}

	// Get combined requirements which has detailed strategy and position info
	combined := profiler.GetCombinedRequirements()

	// Build response with detailed breakdown
	response := gin.H{
		"all_timeframes":     []string{},
		"all_data_fields":    []string{"ohlc", "volume", "taker_buy_volume"},
		"all_symbols":        []string{},
		"strategy_count":     0,
		"position_count":     0,
		"from_strategies":    []interface{}{},
		"from_positions":     []interface{}{},
		"subscriptions":      map[string]interface{}{},
		"timeframes_by_mode": map[string][]string{},
	}

	// Get capacity information for each enabled sub-strategy
	ctx := c.Request.Context()
	capacityInfo := s.getStrategyCapacityInfo(ctx, userID)

	// Build timeframes_by_mode mapping from enabled strategies
	timeframesByMode := make(map[string][]string)
	for _, cap := range capacityInfo {
		// Get the timeframes for this mode+strategy combination from the registry
		stratReqs := coinprofiler.GetStrategyRequirements(cap.Mode, cap.StrategyGroup, cap.SubStrategy)
		if stratReqs != nil && len(stratReqs.Timeframes) > 0 {
			// Add timeframes for this mode (deduplicated)
			existing := timeframesByMode[cap.Mode]
			tfSet := make(map[string]bool)
			for _, tf := range existing {
				tfSet[tf] = true
			}
			for _, tf := range stratReqs.Timeframes {
				if !tfSet[tf] {
					existing = append(existing, tf)
					tfSet[tf] = true
				}
			}
			timeframesByMode[cap.Mode] = existing
		}
	}
	response["timeframes_by_mode"] = timeframesByMode

	if combined != nil {
		response["all_timeframes"] = combined.AllTimeframes
		response["all_data_fields"] = combined.AllDataFields
		response["all_symbols"] = combined.AllSymbols
		response["strategy_count"] = combined.StrategyCount
		response["position_count"] = combined.PositionCount

		// Extract strategy sources (symbols tracked for entry decisions)
		fromStrategies := []interface{}{}
		fromPositions := []interface{}{}

		for _, symReq := range combined.BySymbol {
			if symReq.Source == "strategy" || symReq.Source == "both" {
				// Build strategy info with capacity
				stratInfo := map[string]interface{}{
					"symbol":     symReq.Symbol,
					"timeframes": symReq.Timeframes,
					"strategies": s.enrichStrategiesWithCapacity(symReq.Strategies, capacityInfo),
				}
				fromStrategies = append(fromStrategies, stratInfo)
			}

			if symReq.Source == "position" || symReq.Source == "both" {
				// Build position info
				for _, pos := range symReq.Positions {
					posInfo := map[string]interface{}{
						"symbol":     pos.Symbol,
						"mode":       pos.Mode,
						"side":       pos.Side,
						"exit_mode":  pos.ExitMode,
						"timeframes": pos.Timeframes,
					}
					fromPositions = append(fromPositions, posInfo)
				}
			}
		}

		response["from_strategies"] = fromStrategies
		response["from_positions"] = fromPositions

		// Build subscriptions map
		subsMap := make(map[string]interface{})
		for symbol, symReq := range combined.BySymbol {
			subsMap[symbol] = map[string]interface{}{
				"timeframes":  symReq.Timeframes,
				"data_fields": symReq.DataFields,
				"source":      symReq.Source,
				"strategies":  s.enrichStrategiesWithCapacity(symReq.Strategies, capacityInfo),
				"positions":   symReq.Positions,
			}
		}
		response["subscriptions"] = subsMap
	} else {
		// Fallback to subscriptions if combined requirements not available
		subscriptions := profiler.GetSubscriptions()
		subsMap := make(map[string]interface{})
		timeframeSet := make(map[string]bool)

		for symbol, sub := range subscriptions {
			for _, tf := range sub.Timeframes {
				timeframeSet[tf] = true
			}
			subsMap[symbol] = map[string]interface{}{
				"timeframes": sub.Timeframes,
				"source":     sub.Source,
			}
		}

		timeframes := make([]string, 0, len(timeframeSet))
		for tf := range timeframeSet {
			timeframes = append(timeframes, tf)
		}
		response["all_timeframes"] = timeframes
		response["subscriptions"] = subsMap
	}

	// Add global capacity summary
	response["capacity_summary"] = capacityInfo

	c.JSON(http.StatusOK, response)
}

// StrategyCapacityInfo holds position limit information for a sub-strategy
type StrategyCapacityInfo struct {
	Mode             string `json:"mode"`
	StrategyGroup    string `json:"strategy_group"`
	SubStrategy      string `json:"sub_strategy"`
	CurrentPositions int    `json:"current_positions"`
	MaxPositions     int    `json:"max_positions"`
	CapacityStatus   string `json:"capacity_status"` // "available", "limited", "at_limit"
}

// getStrategyCapacityInfo returns capacity information for all enabled sub-strategies
func (s *Server) getStrategyCapacityInfo(ctx context.Context, userID string) []StrategyCapacityInfo {
	result := []StrategyCapacityInfo{}

	if s.repo == nil {
		return result
	}

	// Get all enabled strategies
	enabledStrategies, err := s.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		return result
	}

	// Get active chains (ACTIVE + PARTIAL) for counting current positions per strategy.
	// Only count chains with actual positions - not PENDING or ENTRY_PLACED which are
	// pre-fill states that may be stale/abandoned and inflate the capacity count.
	// Use the manager's chain event writer directly - the per-instance GinieAutopilot
	// may not have it set if it was created before the chain event writer was initialized.
	activeChains := []*orders.OrderChain{}
	if s.userAutopilotManager != nil {
		chainWriter := s.userAutopilotManager.GetChainEventWriter()
		if chainWriter != nil {
			chains, err := chainWriter.GetActiveChains(ctx, userID)
			if err == nil {
				activeChains = chains
			}
		}
	}

	// Build capacity info for each enabled sub-strategy
	for _, es := range enabledStrategies {
		// Get max_concurrent_trades from sub-strategy settings
		maxConcurrentTrades := 1 // Default

		subSettings, err := s.repo.GetSubStrategySettings(ctx, userID, es.Mode, es.StrategyGroup, es.SubStrategy)
		if err == nil && subSettings != nil && len(subSettings.Settings) > 0 {
			var settingsMap map[string]interface{}
			if err := json.Unmarshal(subSettings.Settings, &settingsMap); err == nil {
				if budgetAlloc, ok := settingsMap["budget_allocation"].(map[string]interface{}); ok {
					if maxTrades, ok := budgetAlloc["max_concurrent_trades"].(float64); ok && maxTrades > 0 {
						maxConcurrentTrades = int(maxTrades)
					}
				}
			}
		}

		// Count current positions for this specific sub-strategy
		// NOTE: Legacy chains may not have Mode/StrategyGroup/SubStrategy populated
		// In that case, match by ModeCode or count all chains for that mode
		currentPositions := 0
		for _, chain := range activeChains {
			// Exact match if new fields are populated
			if chain.Mode != "" {
				if chain.Mode == es.Mode &&
					chain.StrategyGroup == es.StrategyGroup &&
					chain.SubStrategy == es.SubStrategy {
					currentPositions++
				}
			} else {
				// Legacy chain: match by ModeCode
				// ModeCode mapping: SCA=scalp, SWI=swing, POS=position, ULT=ultra_fast
				modeCodeMatches := false
				switch es.Mode {
				case "scalp":
					modeCodeMatches = chain.ModeCode == "SCA"
				case "swing":
					modeCodeMatches = chain.ModeCode == "SWI"
				case "position":
					modeCodeMatches = chain.ModeCode == "POS"
				case "ultra_fast":
					modeCodeMatches = chain.ModeCode == "ULT"
				}
				if modeCodeMatches {
					currentPositions++
				}
			}
		}

		// Determine capacity status
		capacityStatus := "available"
		if currentPositions >= maxConcurrentTrades {
			capacityStatus = "at_limit"
		} else if currentPositions > 0 && float64(currentPositions)/float64(maxConcurrentTrades) >= 0.5 {
			capacityStatus = "limited"
		}

		result = append(result, StrategyCapacityInfo{
			Mode:             es.Mode,
			StrategyGroup:    es.StrategyGroup,
			SubStrategy:      es.SubStrategy,
			CurrentPositions: currentPositions,
			MaxPositions:     maxConcurrentTrades,
			CapacityStatus:   capacityStatus,
		})
	}

	return result
}

// enrichStrategiesWithCapacity adds capacity information to strategy references
func (s *Server) enrichStrategiesWithCapacity(strategies []coinprofiler.StrategyRef, capacityInfo []StrategyCapacityInfo) []map[string]interface{} {
	result := []map[string]interface{}{}

	for _, strat := range strategies {
		enriched := map[string]interface{}{
			"mode":         strat.Mode,
			"strategy":     strat.Strategy,
			"sub_strategy": strat.SubStrategy,
		}

		// Find matching capacity info
		for _, cap := range capacityInfo {
			if cap.Mode == strat.Mode &&
				cap.StrategyGroup == strat.Strategy &&
				cap.SubStrategy == strat.SubStrategy {
				enriched["current_positions"] = cap.CurrentPositions
				enriched["max_positions"] = cap.MaxPositions
				enriched["capacity_status"] = cap.CapacityStatus
				break
			}
		}

		result = append(result, enriched)
	}

	return result
}

// handleStartCoinProfiler starts the Coin Profiler for the authenticated user.
// POST /api/futures/coin-profiler/start
func (s *Server) handleStartCoinProfiler(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	ctx := c.Request.Context()

	// Check if any strategies are enabled before starting
	enabledCount, err := s.userAutopilotManager.GetEnabledStrategiesCount(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "VALIDATION_FAILED",
			"message": "Failed to check enabled strategies: " + err.Error(),
		})
		return
	}

	if enabledCount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":              "NO_STRATEGIES_ENABLED",
			"message":            "No strategies are enabled. Please enable at least one strategy in Futures settings before starting the Coin Profiler.",
			"enabled_strategies": 0,
		})
		return
	}

	if err := s.userAutopilotManager.StartCoinProfiler(ctx, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "START_FAILED",
			"message": err.Error(),
		})
		return
	}

	status := s.userAutopilotManager.GetCoinProfilerStatus(userID)
	c.JSON(http.StatusOK, gin.H{
		"success":            true,
		"message":            fmt.Sprintf("Coin profiler started with %d enabled strategies", enabledCount),
		"status":             status,
		"enabled_strategies": enabledCount,
	})
}

// handleStopCoinProfiler stops the Coin Profiler for the authenticated user.
// POST /api/futures/coin-profiler/stop
func (s *Server) handleStopCoinProfiler(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	if err := s.userAutopilotManager.StopCoinProfiler(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "STOP_FAILED",
			"message": err.Error(),
		})
		return
	}

	status := s.userAutopilotManager.GetCoinProfilerStatus(userID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Coin profiler stopped",
		"status":  status,
	})
}

// handleGetCoinProfilerDiagnostics returns diagnostic information about pattern detection.
// GET /api/futures/coin-profiler/diagnostics
// This endpoint helps debug why patterns are not being detected.
func (s *Server) handleGetCoinProfilerDiagnostics(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	if s.userAutopilotManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Autopilot manager not initialized",
		})
		return
	}

	profiler := s.userAutopilotManager.GetCoinProfiler(userID)
	if profiler == nil {
		c.JSON(http.StatusOK, gin.H{
			"error":   "NO_PROFILER",
			"message": "Coin profiler not active for this user",
		})
		return
	}

	// Get all coin data
	allCoinData := profiler.GetAllCoinData()
	diagnostics := make([]map[string]interface{}, 0)

	for symbol, coinData := range allCoinData {
		// Get candle history for the coin
		for tfName := range coinData.Timeframes {
			history := profiler.GetCandleHistory(symbol, tfName)
			candleCount := len(history)

			// Calculate average volume if we have enough candles
			avgVolume := 0.0
			maxVolume := 0.0
			maxVolumeMultiplier := 0.0
			hasVolumeSpikeCandidate := false
			preTrendAnalysis := "N/A"

			if candleCount >= 10 { // Need at least 10 candles (5 lookback + 5 pre-trend)
				// Calculate 5-candle average volume (BACKTESTED lookback period)
				sum := 0.0
				for i := candleCount - 5; i < candleCount; i++ {
					sum += history[i].Volume
				}
				avgVolume = sum / 5.0

				// Find max volume and check for spikes (using 3x threshold - BACKTESTED)
				for i := 5; i < candleCount-1; i++ {
					vol := history[i].Volume
					if vol > maxVolume {
						maxVolume = vol
						if avgVolume > 0 {
							maxVolumeMultiplier = vol / avgVolume
						}
					}
					if avgVolume > 0 && vol >= avgVolume*3.0 { // 3x threshold (BACKTESTED)
						hasVolumeSpikeCandidate = true
					}
				}

				// Check pre-trend for recent candles
				if candleCount >= 10 {
					preTrendStart := history[candleCount-6].Open
					preTrendEnd := history[candleCount-2].Close
					if preTrendEnd < preTrendStart {
						change := ((preTrendEnd - preTrendStart) / preTrendStart) * 100
						preTrendAnalysis = "DOWN " + formatFloat(change, 2) + "% (qualifies)"
					} else {
						change := ((preTrendEnd - preTrendStart) / preTrendStart) * 100
						preTrendAnalysis = "UP +" + formatFloat(change, 2) + "% (rejected by filter)"
					}
				}
			}

			diag := map[string]interface{}{
				"symbol":                  symbol,
				"timeframe":               tfName,
				"candles_stored":          candleCount,
				"candles_needed":          10, // 5 lookback + 5 pre-trend (BACKTESTED)
				"has_enough_data":         candleCount >= 10,
				"avg_volume_5":            avgVolume, // 5-candle lookback (BACKTESTED)
				"max_volume_in_range":     maxVolume,
				"max_volume_multiplier":   maxVolumeMultiplier,
				"spike_threshold":         "3.0x", // BACKTESTED threshold
				"has_spike_candidate":     hasVolumeSpikeCandidate,
				"pre_trend_5_candles":     preTrendAnalysis,
				"current_price":           coinData.Price,
				"filter_require_pullback": true,
				"filter_explanation":      "BACKTESTED: Pattern requires 5 candles BEFORE spike to show downward movement, then breakout above reference candle high",
			}
			diagnostics = append(diagnostics, diag)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"diagnostics":       diagnostics,
		"total_symbols":     len(allCoinData),
		"pattern_config": map[string]interface{}{
			"volume_spike_multiplier": 3.0,  // BACKTESTED (Dec 2025 - Jan 2026)
			"lookback_period":         5,    // BACKTESTED (Dec 2025 - Jan 2026)
			"require_pre_trend_down":  true,
			"min_body_ratio":          0.0,
			"preferred_hour_utc":      -1,
		},
		"explanation": "BACKTESTED: Patterns require: 1) Volume spike >= 3x average (5 candle lookback), 2) Pre-trend DOWN (5 candles before must show pullback), 3) Then wait for breakout above reference candle high",
	})
}

// formatFloat formats a float with specified decimal places
func formatFloat(f float64, decimals int) string {
	format := "%." + string(rune('0'+decimals)) + "f"
	return fmt.Sprintf(format, f)
}
