package api

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"binance-trading-bot/internal/coinprofiler"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/entrydecision"
	"binance-trading-bot/internal/events"

	"github.com/gin-gonic/gin"
)

// ==================== STORY 14.12: Entry Decision API Endpoints ====================
// API endpoints for the Entry Decision System (Epic 14: Chain Trading System)
// Provides a strategy-first view of entry opportunities across all enabled strategies.
// ===================================================================================

// ==================== REQUEST/RESPONSE TYPES ====================

// EntryDecisionStrategiesResponse is the API response for GET /entry-decision/strategies
type EntryDecisionStrategiesResponse struct {
	Strategies         []entrydecision.StrategyMatch  `json:"strategies"`
	ByMode             []entrydecision.ModeStrategies `json:"by_mode,omitempty"`
	TotalStrategies    int                            `json:"total_strategies"`
	EnabledStrategies  int                            `json:"enabled_strategies"`
	TotalCoinsReady    int                            `json:"total_coins_ready"`
	TotalCoinsWatching int                            `json:"total_coins_watching"`
	UpdatedAt          time.Time                      `json:"updated_at"`
}

// PatternProgressResponse is the API response for GET /entry-decision/pattern/:symbol
type PatternProgressResponse struct {
	Symbol      string                     `json:"symbol"`
	Mode        string                     `json:"mode"`
	Strategy    string                     `json:"strategy"`
	SubStrategy string                     `json:"sub_strategy"`
	Timeframe   string                     `json:"timeframe"`
	CurrentStep int                        `json:"current_step"`
	TotalSteps  int                        `json:"total_steps"`
	Status      string                     `json:"status"`
	StepDetails []entrydecision.StepDetail `json:"step_details"`
	Direction   string                     `json:"direction,omitempty"`
	StartedAt   time.Time                  `json:"started_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
	ExpiresAt   time.Time                  `json:"expires_at,omitempty"`
}

// ScoreBreakdownResponse is the API response for GET /entry-decision/score/:symbol
type ScoreBreakdownResponse struct {
	Symbol     string         `json:"symbol"`
	Score      int            `json:"score"`
	Direction  string         `json:"direction"`
	Ready      bool           `json:"ready"`
	Threshold  int            `json:"threshold"`
	Components map[string]int `json:"components"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// ==================== ENTRY DECISION SERVICE CACHE ====================
// Caches strategy matchers and pattern matchers per user for performance

var (
	entryDecisionMatcherCache = make(map[string]*entrydecision.DefaultStrategyMatcher)
	patternMatcherCache       = make(map[string]*entrydecision.VolumeImbalancePatternMatcher)
	scoreCalculatorCache      = make(map[string]*entrydecision.TrendFollowingScoreCalculator)
	entryDecisionCacheMu      sync.RWMutex

	// Background broadcast service state
	entryDecisionBroadcastStop   chan struct{}
	entryDecisionBroadcastServer *Server
)

// ==================== ENTRY DECISION BROADCAST SERVICE ====================
// Broadcasts Entry Decision strategies to WebSocket clients at regular intervals.
// This enables real-time "Live" indicator in the UI instead of "Polling".

// StartEntryDecisionBroadcast starts the background broadcast service.
// Call this when the server starts.
func (s *Server) StartEntryDecisionBroadcast() {
	if entryDecisionBroadcastStop != nil {
		// Already running
		return
	}

	entryDecisionBroadcastStop = make(chan struct{})
	entryDecisionBroadcastServer = s

	go entryDecisionBroadcastLoop()
}

// StopEntryDecisionBroadcast stops the background broadcast service.
func StopEntryDecisionBroadcast() {
	if entryDecisionBroadcastStop != nil {
		close(entryDecisionBroadcastStop)
		entryDecisionBroadcastStop = nil
		entryDecisionBroadcastServer = nil
	}
}

// entryDecisionBroadcastLoop runs in the background and periodically broadcasts strategies.
func entryDecisionBroadcastLoop() {
	ticker := time.NewTicker(5 * time.Second) // Broadcast every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-entryDecisionBroadcastStop:
			return
		case <-ticker.C:
			broadcastEntryDecisionStrategies()
		}
	}
}

// broadcastEntryDecisionStrategies broadcasts current Entry Decision strategies to all clients.
// It collects users from both the pattern matcher cache AND users with running CoinProfilers
// in the UserAutopilotManager to ensure comprehensive coverage.
func broadcastEntryDecisionStrategies() {
	if entryDecisionBroadcastServer == nil || entryDecisionBroadcastServer.repo == nil {
		return
	}

	// Collect user IDs from multiple sources to ensure all active users get updates
	userIDSet := make(map[string]bool)

	// Source 1: Pattern matcher cache (users who have made API calls)
	entryDecisionCacheMu.RLock()
	for userID := range patternMatcherCache {
		userIDSet[userID] = true
	}
	entryDecisionCacheMu.RUnlock()

	// Source 2: Users with running CoinProfilers in UserAutopilotManager
	// This is CRITICAL for real-time updates - the CoinProfiler feeds the pattern matcher
	if entryDecisionBroadcastServer.userAutopilotManager != nil {
		runningUsers := entryDecisionBroadcastServer.userAutopilotManager.GetAllRunningUsers()
		for _, userID := range runningUsers {
			userIDSet[userID] = true
		}
	}

	// If no users, nothing to broadcast
	if len(userIDSet) == 0 {
		return
	}

	// Convert set to slice
	userIDs := make([]string, 0, len(userIDSet))
	for userID := range userIDSet {
		userIDs = append(userIDs, userID)
	}

	// For now, broadcast combined data for all users
	// In a multi-tenant system, you'd filter by connected users
	for _, userID := range userIDs {
		data := buildEntryDecisionBroadcastData(userID)
		if data != nil {
			BroadcastEntryDecisionUpdate(data)
		}
	}
}

// buildEntryDecisionBroadcastData builds the broadcast payload for a user.
func buildEntryDecisionBroadcastData(userID string) map[string]interface{} {
	if entryDecisionBroadcastServer == nil {
		return nil
	}

	// Get strategy reader
	strategyReader := entryDecisionBroadcastServer.getOrCreateStrategyReader(userID)
	if strategyReader == nil {
		return nil
	}

	// Get enabled strategies (using background context since this is async)
	enabledStrategies, err := strategyReader.GetEnabledStrategies(context.Background(), userID)
	if err != nil {
		return nil
	}

	// Get realtime matcher for countdown timer and direction info
	var globalNextCandleClose time.Time
	var globalLookingFor string
	var lastEvaluatedAt time.Time

	var realtimeMatcher *entrydecision.RealtimePatternMatcher
	if entryDecisionBroadcastServer.userAutopilotManager != nil {
		realtimeMatcher = entryDecisionBroadcastServer.userAutopilotManager.GetRealtimePatternMatcher(userID)
		if realtimeMatcher != nil {
			// Get any pattern update to extract timing info
			updates := realtimeMatcher.GetAllPatternUpdates()
			if len(updates) > 0 {
				// Use the first update's timing (all should be the same timeframe)
				globalNextCandleClose = updates[0].NextCandleClose
				globalLookingFor = updates[0].LookingFor
				lastEvaluatedAt = updates[0].LastEvaluatedAt
			}
		}
	}

	// Check if CoinProfiler is running - only populate coins when it is
	profilerRunning := entryDecisionBroadcastServer.userAutopilotManager != nil &&
		entryDecisionBroadcastServer.userAutopilotManager.IsCoinProfilerRunningForUser(userID)

	// Build response
	response := entrydecision.NewEntryDecisionResponse()

	// Get pattern matcher (shared across strategies for this user)
	patternMatcher := entryDecisionBroadcastServer.getOrCreatePatternMatcher(userID)

	// Get active positions for enrichment (same as REST endpoint)
	activePositions := entryDecisionBroadcastServer.getActivePositionsMap(userID)

	// Reset stale patterns (position_running status but no active position)
	if profilerRunning {
		entryDecisionBroadcastServer.resetStalePatterns(userID, patternMatcher, activePositions)
	}

	for _, strategy := range enabledStrategies {
		strategyType := entrydecision.GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		sm := entrydecision.NewStrategyMatch(
			strategy.Mode,
			strategy.StrategyGroup,
			strategy.SubStrategy,
			strategyType,
			strategy.Timeframe,
		)
		sm.Enabled = true

		// Set per-strategy countdown timer (based on its timeframe)
		sm.NextCandleClose = calculateNextCandleCloseForTimeframe(strategy.Timeframe)
		sm.LookingFor = globalLookingFor // Use global direction setting

		// Populate strategy config from pattern matcher for dynamic UI display
		if patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			pmConfig := patternMatcher.GetConfig()
			sm.Config = map[string]interface{}{
				"volume_spike_multiplier": pmConfig.MinVolumeSpikeMultiplier,
				"lookback_period":         pmConfig.LookbackPeriod,
				"breakout_volume_surge":   pmConfig.BreakoutVolumeSurge,
				"pattern_expiration_mins": pmConfig.PatternExpirationMinutes,
				"risk_reward_ratio":       4.0,
			}
		}

		// Only populate coins when CoinProfiler is running
		if profilerRunning && patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			// Use GetCoinMatchesForStrategy to filter by mode/timeframe - prevents duplicate coins
			// when the same coin is tracked across multiple strategies (e.g., scalp/3m and swing/1h)
			strategyMatches := patternMatcher.GetCoinMatchesForStrategy(strategy.Mode, strategy.Timeframe)
			for _, cm := range strategyMatches {
				if cm != nil && cm.Status != "" {
					// Enrich with real-time data from volume progress
					// This enables the volume progress bar and price context bar in the UI
					if realtimeMatcher != nil {
						volProgress := realtimeMatcher.GetVolumeProgress(cm.Symbol, strategy.Timeframe)
						if volProgress != nil {
							// Always update current price from real-time data
							cm.CurrentPrice = volProgress.CurrentPrice

							// Volume data for watching state coins
							if cm.Status == entrydecision.PatternStatusWatching {
								cm.CurrentVolume = volProgress.CurrentVolume
								cm.AvgVolume = volProgress.AverageVolume
								cm.VolumeMultiplier = volProgress.CurrentRatio
								cm.VolumeThreshold = volProgress.RequiredRatio
								// Calculate distance to threshold as percentage
								if volProgress.RequiredRatio > 0 {
									cm.VolumeDistancePercent = ((volProgress.CurrentRatio / volProgress.RequiredRatio) - 1) * 100
								}
							}
						}
					}

					// Enrich with active position data (same as REST endpoint)
					if posInfo, exists := activePositions[cm.Symbol]; exists {
						enrichCoinMatchWithPosition(cm, posInfo)
					}

					// Enrich with closed position PnL data (if recently closed)
					enrichCoinMatchWithClosedPnL(cm)

					sm.AddCoin(*cm)
				}
			}
		}

		// Fallback: Add synthetic CoinMatch for positions not found in pattern matcher
		// This ensures Step 4 always shows even if pattern state was lost (restart, edge case)
		if profilerRunning && strategyType == entrydecision.StrategyTypePattern {
			for posSymbol, posInfo := range activePositions {
				// Check if this position's symbol is already in the strategy's coins
				alreadyAdded := false
				for _, existingCoin := range sm.Coins {
					if existingCoin.Symbol == posSymbol {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					// Create a synthetic CoinMatch for this position
					syntheticCM := &entrydecision.CoinMatch{
						Symbol:     posSymbol,
						Status:     entrydecision.PatternStatusPositionRunning,
						Step:       4,
						TotalSteps: 4,
						UpdatedAt:  time.Now(),
					}
					enrichCoinMatchWithPosition(syntheticCM, posInfo)
					enrichCoinMatchWithClosedPnL(syntheticCM)
					sm.AddCoin(*syntheticCM)
				}
			}
		}

		response.AddStrategy(*sm)
	}

	response.GroupByMode()
	response.CalculateSummary()

	// Convert to map for broadcast, including countdown timer and direction info
	return map[string]interface{}{
		"strategies":           response.Strategies,
		"by_mode":              response.ByMode,
		"total_strategies":     response.TotalStrategies,
		"enabled_strategies":   response.EnabledStrategies,
		"total_coins_ready":    response.TotalCoinsReady,
		"total_coins_watching": response.TotalCoinsWatching,
		"updated_at":           response.UpdatedAt,
		"next_candle_close":    globalNextCandleClose,
		"looking_for":          globalLookingFor,
		"last_evaluated_at":    lastEvaluatedAt,
	}
}

// calculateNextCandleCloseForTimeframe calculates the next candle close time for a given timeframe.
// This is used to set per-strategy countdown timers.
func calculateNextCandleCloseForTimeframe(timeframe string) time.Time {
	now := time.Now().UTC()

	// Parse timeframe in minutes
	var durationMinutes int
	switch timeframe {
	case "1m":
		durationMinutes = 1
	case "3m":
		durationMinutes = 3
	case "5m":
		durationMinutes = 5
	case "15m":
		durationMinutes = 15
	case "30m":
		durationMinutes = 30
	case "1h":
		durationMinutes = 60
	case "4h":
		durationMinutes = 240
	case "1d":
		durationMinutes = 1440
	default:
		durationMinutes = 3 // Default to 3m
	}

	// Calculate based on current minute alignment (Binance candles align to clock)
	currentMinute := now.Minute()
	currentSecond := now.Second()

	// Find the next candle close minute
	currentCandleStart := currentMinute - (currentMinute % durationMinutes)
	nextCandleClose := currentCandleStart + durationMinutes

	// Handle hour rollover
	hoursToAdd := 0
	if nextCandleClose >= 60 {
		nextCandleClose = nextCandleClose % 60
		hoursToAdd = 1
	}

	// Build the next close time
	nextClose := time.Date(
		now.Year(), now.Month(), now.Day(),
		now.Hour()+hoursToAdd, nextCandleClose, 0, 0,
		time.UTC,
	)

	// If the calculated time is in the past (edge case at second 0), add duration
	if nextClose.Before(now) || (nextClose.Equal(now) && currentSecond == 0) {
		nextClose = nextClose.Add(time.Duration(durationMinutes) * time.Minute)
	}

	return nextClose
}

// ==================== POSITION ENRICHMENT ====================

// ActivePositionInfo holds relevant position data for enriching coin matches
type ActivePositionInfo struct {
	Symbol       string
	EntryPrice   float64
	Quantity     float64
	Side         string
	OpenedAt     time.Time
	UnrealizedPL float64
	ChainID      string
}

// getActivePositionsMap returns a map of symbol -> ActivePositionInfo for all active positions.
// It queries multiple sources to ensure positions are found regardless of how they were opened:
// 1. Ginie Autopilot (legacy system)
// 2. Binance directly (for chain-entry positions not tracked by Ginie)
// 3. Active order chains from DB (for chain ID and opened-at time)
func (s *Server) getActivePositionsMap(userID string) map[string]*ActivePositionInfo {
	positions := make(map[string]*ActivePositionInfo)

	// Source 1: Ginie Autopilot
	if controller := s.getFuturesAutopilot(); controller != nil {
		if autopilot := controller.GetGinieAutopilot(); autopilot != nil {
			giniePositions := autopilot.GetPositions()
			for _, gPos := range giniePositions {
				if gPos.RemainingQty > 0 {
					positions[gPos.Symbol] = &ActivePositionInfo{
						Symbol:       gPos.Symbol,
						EntryPrice:   gPos.EntryPrice,
						Quantity:     gPos.RemainingQty,
						Side:         gPos.Side,
						OpenedAt:     gPos.EntryTime,
						UnrealizedPL: 0,
					}
				}
			}
		}
	}

	// Source 2: Binance directly (for chain-entry positions not tracked by Ginie)
	if futuresClient := s.getFuturesClient(); futuresClient != nil {
		binancePositions, err := futuresClient.GetPositions()
		if err == nil {
			for _, pos := range binancePositions {
				if pos.PositionAmt != 0 {
					if _, exists := positions[pos.Symbol]; !exists {
						// Only add if not already from Ginie (avoid duplicates)
						side := "LONG"
						if pos.PositionAmt < 0 {
							side = "SHORT"
						}
						positions[pos.Symbol] = &ActivePositionInfo{
							Symbol:       pos.Symbol,
							EntryPrice:   pos.EntryPrice,
							Quantity:     math.Abs(pos.PositionAmt),
							Side:         side,
							UnrealizedPL: pos.UnrealizedProfit,
						}
					}
				}
			}
		}
	}

	// Source 3: Enrich with order chain data (chain ID, entry time)
	if s.repo != nil && len(positions) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		activeChains, err := s.repo.GetDB().GetActiveOrderChains(ctx, userID)
		if err == nil {
			for _, chain := range activeChains {
				if posInfo, exists := positions[chain.Symbol]; exists {
					posInfo.ChainID = chain.ChainID
					if chain.EntryFilledAt != nil {
						posInfo.OpenedAt = *chain.EntryFilledAt
					} else {
						posInfo.OpenedAt = chain.CreatedAt
					}
				}
			}
		}
	}

	return positions
}

// enrichCoinMatchWithPosition enriches a CoinMatch with active position data
// If a position exists, sets status to position_running and populates position fields
func enrichCoinMatchWithPosition(cm *entrydecision.CoinMatch, posInfo *ActivePositionInfo) {
	if cm == nil || posInfo == nil {
		return
	}

	cm.HasActivePosition = true
	cm.PositionEntryPrice = posInfo.EntryPrice
	cm.PositionQuantity = posInfo.Quantity
	cm.ChainID = posInfo.ChainID
	openedAt := posInfo.OpenedAt
	if !openedAt.IsZero() {
		cm.PositionOpenedAt = &openedAt
	}

	// Calculate SecondsRefToEntry if we have both reference detection time and position opened time
	if cm.ReferenceDetectedAt != nil && !openedAt.IsZero() {
		cm.SecondsRefToEntry = int(openedAt.Sub(*cm.ReferenceDetectedAt).Seconds())
		if cm.SecondsRefToEntry < 0 {
			cm.SecondsRefToEntry = 0
		}
	}

	// Set direction from position side (normalize casing)
	side := strings.ToUpper(posInfo.Side)
	if side == "LONG" {
		cm.Direction = "long"
	} else if side == "SHORT" {
		cm.Direction = "short"
	} else if cm.Direction == "" {
		cm.Direction = "long" // Default fallback
	}

	// Set status to position_running so UI knows a position is active
	// This prevents the UI from showing "watching" when a position exists
	cm.Status = entrydecision.PatternStatusPositionRunning

	// Preserve the entry candle info if present (reference candle becomes entry candle context)
	// The entry price from position takes precedence for display
	if cm.EntryCandle != nil {
		cm.EntryCandle.EntryPrice = posInfo.EntryPrice
	}
}

// closedPositionPnL stores recently closed position PnL data for display
// Key: symbol, Value: closed PNL info. Auto-expires after 60 seconds.
type closedPnLInfo struct {
	PnL         float64
	CloseReason string
	ExpiresAt   time.Time
}

var (
	closedPnLMap   = make(map[string]*closedPnLInfo)
	closedPnLMutex sync.Mutex
)

// getClosedPnL returns closed PNL info for a symbol if still valid
func getClosedPnL(symbol string) *closedPnLInfo {
	closedPnLMutex.Lock()
	defer closedPnLMutex.Unlock()
	if info, ok := closedPnLMap[symbol]; ok {
		if time.Now().Before(info.ExpiresAt) {
			return info
		}
		delete(closedPnLMap, symbol)
	}
	return nil
}

// enrichCoinMatchWithClosedPnL enriches a CoinMatch with closed position PnL data
func enrichCoinMatchWithClosedPnL(cm *entrydecision.CoinMatch) {
	if cm == nil {
		return
	}
	info := getClosedPnL(cm.Symbol)
	if info != nil {
		pnl := info.PnL
		cm.ClosedPnL = &pnl
		cm.ClosedReason = info.CloseReason
		cm.Status = entrydecision.PatternStatusPositionClosed
	}
}

// resetStalePatterns resets patterns for symbols that had position_running status
// but no longer have active positions (position was closed).
//
// SAFETY: If activePositions is empty, we verify against the DB (order_chains)
// before resetting. An empty activePositions map can result from Binance API
// failures (rate limit, IP ban, network error), NOT from actual position closure.
// Without this check, every API failure would destroy position_running state.
func (s *Server) resetStalePatterns(userID string, patternMatcher *entrydecision.VolumeImbalancePatternMatcher, activePositions map[string]*ActivePositionInfo) {
	if patternMatcher == nil {
		return
	}

	// Get all patterns and check for stale position_running status
	allPatterns := patternMatcher.GetAllPatterns()
	for _, pattern := range allPatterns {
		if pattern == nil {
			continue
		}

		// If pattern is position_running but no active position, position was closed
		if pattern.Status == entrydecision.PatternStatusPositionRunning {
			if _, hasPosition := activePositions[pattern.Symbol]; !hasPosition {
				// SAFETY CHECK: If activePositions is empty (possible API failure),
				// verify against DB order chains before destroying position_running state.
				// This prevents false resets when Binance API is rate-limited or down.
				if len(activePositions) == 0 && s.repo != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					activeChains, err := s.repo.GetDB().GetActiveOrderChains(ctx, userID)
					cancel()
					if err != nil {
						// DB error - don't reset, keep position_running to be safe
						log.Printf("[ENTRY-DECISION] Skipping stale pattern reset for %s: DB error %v (keeping position_running)", pattern.Symbol, err)
						continue
					}
					// Check if this symbol has an active chain in DB
					hasActiveChain := false
					for _, chain := range activeChains {
						if chain.Symbol == pattern.Symbol {
							hasActiveChain = true
							break
						}
					}
					if hasActiveChain {
						// DB confirms position is still active - API just failed
						log.Printf("[ENTRY-DECISION] Keeping position_running for %s: active chain confirmed in DB (API positions empty)", pattern.Symbol)
						continue
					}
				}

				// Try to get closed chain data for PNL display
				if s.repo != nil {
					ctx := context.Background()
					closedChains, err := s.repo.GetDB().GetOrderChainsWithFilter(ctx, database.OrderChainFilter{
						UserID: userID,
						Symbol: pattern.Symbol,
						Status: "closed",
						Limit:  1,
					})
					if err == nil && len(closedChains) > 0 {
						chain := closedChains[0]
						var pnl float64
						var reason string
						if chain.RealizedPnL != nil {
							pnl = *chain.RealizedPnL
						}
						if chain.CloseReason != nil {
							reason = *chain.CloseReason
						}

						// Store in temporary map for 60s display
						closedPnLMutex.Lock()
						closedPnLMap[pattern.Symbol] = &closedPnLInfo{
							PnL:         pnl,
							CloseReason: reason,
							ExpiresAt:   time.Now().Add(60 * time.Second),
						}
						closedPnLMutex.Unlock()

						log.Printf("[ENTRY-DECISION] Position closed for %s: PnL=%.2f reason=%s",
							pattern.Symbol, pnl, reason)
					}
				}
				// Reset pattern to watching state (closed PNL will be enriched from map during broadcast)
				log.Printf("[ENTRY-DECISION] Resetting stale pattern for %s (position closed, confirmed no active position)", pattern.Symbol)
				patternMatcher.ResetPattern(pattern.Symbol, pattern.Mode, pattern.Timeframe)
			}
		}
	}
}

// ==================== HANDLER FUNCTIONS ====================

// handleGetEntryDecisionStrategies handles GET /api/futures/entry-decision/strategies
// Returns all strategies with matching coins (strategy-first view)
func (s *Server) handleGetEntryDecisionStrategies(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Get or create strategy reader using the repository
	strategyReader := s.getOrCreateStrategyReader(userID)
	if strategyReader == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Database repository not initialized",
		})
		return
	}

	// Get enabled strategies from reader
	enabledStrategies, err := strategyReader.GetEnabledStrategies(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "FETCH_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Build response manually (simpler than full matcher since we don't have coin data yet)
	response := entrydecision.NewEntryDecisionResponse()

	// Get realtime matcher for volume progress data
	var realtimeMatcher *entrydecision.RealtimePatternMatcher
	if s.userAutopilotManager != nil {
		realtimeMatcher = s.userAutopilotManager.GetRealtimePatternMatcher(userID)
	}

	// Get active positions map for enrichment
	// This prevents coins with active positions from showing "watching" status
	activePositions := s.getActivePositionsMap(userID)

	// Check if CoinProfiler is running - only populate coins when it is
	// When profiler is not running, coins from restored pattern states would be stale
	profilerRunning := s.userAutopilotManager != nil &&
		s.userAutopilotManager.IsCoinProfilerRunningForUser(userID)

	// Reset stale patterns (position_running status but no active position)
	// This ensures patterns reset to watching when positions are closed
	patternMatcher := s.getOrCreatePatternMatcher(userID)
	if profilerRunning {
		s.resetStalePatterns(userID, patternMatcher, activePositions)
	}

	for _, strategy := range enabledStrategies {
		strategyType := entrydecision.GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		sm := entrydecision.NewStrategyMatch(
			strategy.Mode,
			strategy.StrategyGroup,
			strategy.SubStrategy,
			strategyType,
			strategy.Timeframe,
		)
		sm.Enabled = true

		// Populate strategy config from pattern matcher for dynamic UI display
		if patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			pmConfig := patternMatcher.GetConfig()
			sm.Config = map[string]interface{}{
				"volume_spike_multiplier": pmConfig.MinVolumeSpikeMultiplier,
				"lookback_period":         pmConfig.LookbackPeriod,
				"breakout_volume_surge":   pmConfig.BreakoutVolumeSurge,
				"pattern_expiration_mins": pmConfig.PatternExpirationMinutes,
				"risk_reward_ratio":       4.0, // Default R:R from pattern_matcher.go
			}
		}

		// Only populate coins when CoinProfiler is running
		// Exception: coins with active positions still appear
		if profilerRunning && patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			// Use GetCoinMatchesForStrategy to filter by mode/timeframe - prevents duplicate coins
			// when the same coin is tracked across multiple strategies (e.g., scalp/3m and swing/1h)
			strategyMatches := patternMatcher.GetCoinMatchesForStrategy(strategy.Mode, strategy.Timeframe)
			for _, cm := range strategyMatches {
				if cm != nil && cm.Status != "" {
					// Enrich with real-time data from volume progress
					// This enables the volume progress bar and price context bar in the UI
					if realtimeMatcher != nil {
						volProgress := realtimeMatcher.GetVolumeProgress(cm.Symbol, strategy.Timeframe)
						if volProgress != nil {
							// Always update current price from real-time data
							cm.CurrentPrice = volProgress.CurrentPrice

							// Volume data for watching state coins
							if cm.Status == entrydecision.PatternStatusWatching {
								cm.CurrentVolume = volProgress.CurrentVolume
								cm.AvgVolume = volProgress.AverageVolume
								cm.VolumeMultiplier = volProgress.CurrentRatio
								cm.VolumeThreshold = volProgress.RequiredRatio
								// Calculate distance to threshold as percentage
								if volProgress.RequiredRatio > 0 {
									cm.VolumeDistancePercent = ((volProgress.CurrentRatio / volProgress.RequiredRatio) - 1) * 100
								}
							}
						}
					}

					// Enrich with active position data if position exists
					// This sets status to "position_running" and populates position fields
					if posInfo, exists := activePositions[cm.Symbol]; exists {
						enrichCoinMatchWithPosition(cm, posInfo)
					}

					sm.AddCoin(*cm)
				}
			}
		}

		// Fallback: Add synthetic CoinMatch for positions not found in pattern matcher
		// This ensures Step 4 always shows even if pattern state was lost (restart, edge case)
		if profilerRunning && strategyType == entrydecision.StrategyTypePattern {
			for posSymbol, posInfo := range activePositions {
				// Check if this position's symbol is already in the strategy's coins
				alreadyAdded := false
				for _, existingCoin := range sm.Coins {
					if existingCoin.Symbol == posSymbol {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					// Create a synthetic CoinMatch for this position
					syntheticCM := &entrydecision.CoinMatch{
						Symbol:     posSymbol,
						Status:     entrydecision.PatternStatusPositionRunning,
						Step:       4,
						TotalSteps: 4,
						UpdatedAt:  time.Now(),
					}
					enrichCoinMatchWithPosition(syntheticCM, posInfo)
					sm.AddCoin(*syntheticCM)
				}
			}
		}

		response.AddStrategy(*sm)
	}

	// Group by mode and calculate summary
	response.GroupByMode()
	response.CalculateSummary()

	// Convert to API response
	apiResponse := EntryDecisionStrategiesResponse{
		Strategies:         response.Strategies,
		ByMode:             response.ByMode,
		TotalStrategies:    response.TotalStrategies,
		EnabledStrategies:  response.EnabledStrategies,
		TotalCoinsReady:    response.TotalCoinsReady,
		TotalCoinsWatching: response.TotalCoinsWatching,
		UpdatedAt:          response.UpdatedAt,
	}

	c.JSON(http.StatusOK, apiResponse)
}

// handleGetEntryDecisionStrategiesForMode handles GET /api/futures/entry-decision/strategies/:mode
// Returns strategies for a specific trading mode
func (s *Server) handleGetEntryDecisionStrategiesForMode(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	mode := strings.ToLower(c.Param("mode"))
	if !entrydecision.IsValidMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_MODE",
			"message": "Invalid trading mode. Valid modes: scalp, swing, position, ultra_fast",
		})
		return
	}

	// Get or create strategy reader using the repository
	strategyReader := s.getOrCreateStrategyReader(userID)
	if strategyReader == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Database repository not initialized",
		})
		return
	}

	// Get enabled strategies for this mode
	enabledStrategies, err := strategyReader.GetEnabledStrategiesForMode(c.Request.Context(), userID, mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "FETCH_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Build mode strategies response
	modeStrategies := entrydecision.NewModeStrategies(mode, true)

	patternMatcher := s.getOrCreatePatternMatcher(userID)

	// Get realtime matcher for volume progress data
	var realtimeMatcher *entrydecision.RealtimePatternMatcher
	if s.userAutopilotManager != nil {
		realtimeMatcher = s.userAutopilotManager.GetRealtimePatternMatcher(userID)
	}

	// Get active positions map for enrichment
	activePositions := s.getActivePositionsMap(userID)

	for _, strategy := range enabledStrategies {
		strategyType := entrydecision.GetStrategyType(strategy.StrategyGroup, strategy.SubStrategy)

		sm := entrydecision.NewStrategyMatch(
			strategy.Mode,
			strategy.StrategyGroup,
			strategy.SubStrategy,
			strategyType,
			strategy.Timeframe,
		)
		sm.Enabled = true

		// Get pattern matcher to check for tracked coins
		if patternMatcher != nil && strategyType == entrydecision.StrategyTypePattern {
			// Use GetCoinMatchesForStrategy to filter by mode/timeframe - prevents duplicate coins
			// when the same coin is tracked across multiple strategies (e.g., scalp/3m and swing/1h)
			strategyMatches := patternMatcher.GetCoinMatchesForStrategy(strategy.Mode, strategy.Timeframe)
			for _, cm := range strategyMatches {
				if cm != nil && cm.Status != "" {
					// Enrich with real-time data from volume progress
					if realtimeMatcher != nil {
						volProgress := realtimeMatcher.GetVolumeProgress(cm.Symbol, strategy.Timeframe)
						if volProgress != nil {
							// Always update current price from real-time data
							cm.CurrentPrice = volProgress.CurrentPrice

							// Volume data for watching state coins
							if cm.Status == entrydecision.PatternStatusWatching {
								cm.CurrentVolume = volProgress.CurrentVolume
								cm.AvgVolume = volProgress.AverageVolume
								cm.VolumeMultiplier = volProgress.CurrentRatio
								cm.VolumeThreshold = volProgress.RequiredRatio
								if volProgress.RequiredRatio > 0 {
									cm.VolumeDistancePercent = ((volProgress.CurrentRatio / volProgress.RequiredRatio) - 1) * 100
								}
							}
						}
					}

					// Enrich with active position data if position exists
					if posInfo, exists := activePositions[cm.Symbol]; exists {
						enrichCoinMatchWithPosition(cm, posInfo)
					}

					sm.AddCoin(*cm)
				}
			}
		}

		modeStrategies.AddStrategy(*sm)
	}

	c.JSON(http.StatusOK, modeStrategies)
}

// handleGetEntryCandidates handles GET /api/futures/entry-decision/candidates
// Returns all coins that are ready for entry across all strategies
func (s *Server) handleGetEntryCandidates(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Get pattern matcher to find ready patterns
	patternMatcher := s.getOrCreatePatternMatcher(userID)

	candidates := entrydecision.NewEntryCandidatesResponse()

	// Get ready patterns from pattern matcher
	if patternMatcher != nil {
		readyPatterns := patternMatcher.GetReadyPatterns()
		for _, progress := range readyPatterns {
			candidate := entrydecision.EntryCandidate{
				Symbol:      progress.Symbol,
				Mode:        progress.Mode,
				Strategy:    progress.Strategy,
				SubStrategy: progress.SubStrategy,
				Type:        entrydecision.StrategyTypePattern,
				Timeframe:   progress.Timeframe,
				Step:        progress.CurrentStep,
				Status:      progress.Status,
				Details:     formatStepDetails(progress),
				Priority:    calculatePatternPriority(progress),
				UpdatedAt:   progress.UpdatedAt,
			}
			candidates.AddCandidate(candidate)
		}
	}

	c.JSON(http.StatusOK, candidates)
}

// handleGetPatternProgress handles GET /api/futures/entry-decision/pattern/:symbol
// Returns pattern progress for a specific coin
func (s *Server) handleGetPatternProgress(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_SYMBOL",
			"message": "Symbol is required",
		})
		return
	}

	// Optional mode and timeframe query parameters
	mode := strings.ToLower(c.Query("mode"))
	if mode == "" {
		mode = "scalp" // Default mode
	}
	if !entrydecision.IsValidMode(mode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_MODE",
			"message": "Invalid trading mode. Valid modes: scalp, swing, position, ultra_fast",
		})
		return
	}

	timeframe := c.Query("timeframe")
	if timeframe == "" {
		timeframe = getDefaultTimeframeForMode(mode)
	}

	// Get the pattern matcher
	patternMatcher := s.getOrCreatePatternMatcher(userID)
	if patternMatcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Pattern matcher not initialized",
		})
		return
	}

	// Get pattern progress for the symbol
	progress := patternMatcher.GetPattern(symbol, mode, timeframe)
	if progress == nil {
		// No active pattern for this symbol
		c.JSON(http.StatusOK, gin.H{
			"symbol":       symbol,
			"mode":         mode,
			"timeframe":    timeframe,
			"status":       "not_tracking",
			"message":      "No active pattern being tracked for this symbol",
			"current_step": 0,
			"total_steps":  2, // 2-step pattern: Volume Spike → Breakout
		})
		return
	}

	// Convert to API response
	response := PatternProgressResponse{
		Symbol:      progress.Symbol,
		Mode:        progress.Mode,
		Strategy:    progress.Strategy,
		SubStrategy: progress.SubStrategy,
		Timeframe:   progress.Timeframe,
		CurrentStep: progress.CurrentStep,
		TotalSteps:  progress.TotalSteps,
		Status:      string(progress.Status),
		StepDetails: progress.StepDetails,
		StartedAt:   progress.StartedAt,
		UpdatedAt:   progress.UpdatedAt,
		ExpiresAt:   progress.ExpiresAt,
	}

	c.JSON(http.StatusOK, response)
}

// handleGetScoreBreakdown handles GET /api/futures/entry-decision/score/:symbol
// Returns score breakdown for a specific coin (for score-based strategies)
func (s *Server) handleGetScoreBreakdown(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_SYMBOL",
			"message": "Symbol is required",
		})
		return
	}

	// Get or create the score calculator
	scoreCalc := s.getOrCreateScoreCalculator(userID)
	if scoreCalc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "SERVICE_UNAVAILABLE",
			"message": "Score calculator not initialized",
		})
		return
	}

	// Get coin data for the symbol (from coin profiler if available)
	coinData := s.getCoinDataForSymbol(userID, symbol)

	// Calculate score
	result, err := scoreCalc.CalculateScore(symbol, coinData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SCORE_CALCULATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Convert to API response
	response := ScoreBreakdownResponse{
		Symbol:     result.Symbol,
		Score:      result.Score,
		Direction:  result.Direction,
		Ready:      result.Ready,
		Threshold:  result.Threshold,
		Components: result.Components,
		UpdatedAt:  result.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// ==================== HELPER FUNCTIONS ====================

// getOrCreateStrategyReader returns or creates a strategy reader for a user.
func (s *Server) getOrCreateStrategyReader(userID string) entrydecision.StrategyReader {
	if s.repo == nil {
		return nil
	}
	// Create a new reader each time - it's lightweight and uses the DB directly
	return entrydecision.NewDefaultStrategyReader(s.repo)
}

// getOrCreatePatternMatcher returns or creates a pattern matcher for a user.
// Priority: 1) Get from UserAutopilotInstance (connected to CoinProfiler)
//           2) Fall back to cache
//           3) Create new with USER SETTINGS from database/cache
//
// IMPORTANT: This now loads user-configured values from default-settings.json → database
// instead of using hard-coded defaults. This ensures the pattern matcher respects
// user's Volume Imbalance sub-strategy settings.
func (s *Server) getOrCreatePatternMatcher(userID string) *entrydecision.VolumeImbalancePatternMatcher {
	// First, try to get from UserAutopilotInstance (this is connected to CoinProfiler)
	if s.userAutopilotManager != nil {
		instance := s.userAutopilotManager.GetInstance(userID)
		if instance != nil && instance.RealtimePatternMatcher != nil {
			// Get the pattern matcher from the realtime matcher
			if patternMatcher := instance.RealtimePatternMatcher.GetPatternMatcher(); patternMatcher != nil {
				return patternMatcher
			}
		}
	}

	// Fall back to cache
	entryDecisionCacheMu.RLock()
	matcher, exists := patternMatcherCache[userID]
	entryDecisionCacheMu.RUnlock()

	if exists {
		return matcher
	}

	// Create a new pattern matcher with USER SETTINGS (not hard-coded defaults!)
	entryDecisionCacheMu.Lock()
	defer entryDecisionCacheMu.Unlock()

	// Double-check after acquiring write lock
	if matcher, exists = patternMatcherCache[userID]; exists {
		return matcher
	}

	// Load user's Volume Imbalance sub-strategy settings from database
	config := s.loadUserPatternMatcherConfig(userID)
	matcher = entrydecision.NewVolumeImbalancePatternMatcher(config)
	patternMatcherCache[userID] = matcher

	return matcher
}

// loadUserPatternMatcherConfig loads user's Volume Imbalance pattern_detection settings
// from the database and converts them to PatternMatcherConfig.
// Falls back to DefaultPatternMatcherConfig if user settings cannot be loaded.
func (s *Server) loadUserPatternMatcherConfig(userID string) *entrydecision.PatternMatcherConfig {
	if s.repo == nil {
		return entrydecision.DefaultPatternMatcherConfig()
	}

	// Load Volume Imbalance sub-strategy settings for scalp mode (primary mode)
	// The settings are the same structure across modes
	ctx := context.Background()
	subSettings, err := s.repo.GetSubStrategySettings(ctx, userID, "scalp", "breakout", "ravindra_volume_imbalance")
	if err != nil || subSettings == nil || len(subSettings.Settings) == 0 {
		// Fall back to defaults if user settings not found
		return entrydecision.DefaultPatternMatcherConfig()
	}

	// Parse settings JSON and create config
	settings := entrydecision.ParseSubStrategySettingsJSON(subSettings.Settings)
	if settings == nil {
		return entrydecision.DefaultPatternMatcherConfig()
	}

	return entrydecision.NewPatternMatcherConfigFromSettings(settings)
}

// getOrCreateScoreCalculator returns or creates a score calculator for a user.
func (s *Server) getOrCreateScoreCalculator(userID string) *entrydecision.TrendFollowingScoreCalculator {
	entryDecisionCacheMu.RLock()
	calc, exists := scoreCalculatorCache[userID]
	entryDecisionCacheMu.RUnlock()

	if exists {
		return calc
	}

	// Create a new score calculator with default config
	entryDecisionCacheMu.Lock()
	defer entryDecisionCacheMu.Unlock()

	// Double-check after acquiring write lock
	if calc, exists = scoreCalculatorCache[userID]; exists {
		return calc
	}

	calc = entrydecision.NewTrendFollowingScoreCalculator()
	scoreCalculatorCache[userID] = calc

	return calc
}

// getCoinDataForSymbol returns coin data for a symbol from the coin profiler.
// Returns nil if coin profiler is not available or symbol not found.
func (s *Server) getCoinDataForSymbol(userID string, symbol string) *coinprofiler.CoinData {
	// For now, return nil - the score calculator handles nil gracefully
	// In a future story, this will integrate with the Coin Profiler service
	return nil
}

// getDefaultTimeframeForMode returns the default timeframe for a trading mode.
// Updated: Volume Imbalance strategy uses 3m timeframe for scalp/swing modes
// based on Dec 2025 - Jan 2026 backtest results.
func getDefaultTimeframeForMode(mode string) string {
	switch mode {
	case "ultra_fast":
		return "1m"
	case "scalp":
		return "3m" // Volume Imbalance uses 3m for scalp
	case "swing":
		return "3m" // Volume Imbalance uses 3m for swing
	case "position":
		return "1h"
	default:
		return "3m"
	}
}

// formatStepDetails formats step details for display.
func formatStepDetails(progress *entrydecision.PatternProgress) string {
	if progress == nil {
		return ""
	}
	if progress.CurrentStep > 0 && progress.CurrentStep <= len(progress.StepDetails) {
		step := progress.StepDetails[progress.CurrentStep-1]
		if step.Progress != "" {
			return step.Progress
		}
	}
	return ""
}

// calculatePatternPriority calculates priority for a pattern candidate.
func calculatePatternPriority(progress *entrydecision.PatternProgress) int {
	if progress == nil {
		return 0
	}
	priority := 50 // Base priority

	// Ready patterns have highest priority
	if progress.Status == entrydecision.PatternStatusReady {
		priority += 30
	}

	// Add priority based on step progress
	priority += progress.CurrentStep * 5

	// Mode-based adjustments
	switch progress.Mode {
	case "ultra_fast":
		priority += 10
	case "scalp":
		priority += 5
	}

	return priority
}

// ==================== REAL-TIME PATTERN UPDATE BROADCASTING ====================

// PatternUpdateResponse is the WebSocket message format for pattern updates.
type PatternUpdateResponse struct {
	EventType string                       `json:"event_type"`
	Data      entrydecision.PatternUpdate  `json:"data"`
	Timestamp time.Time                    `json:"timestamp"`
}

// EntryLevelsResponse is used for displaying entry levels in the UI.
type EntryLevelsResponse struct {
	Symbol          string  `json:"symbol"`
	EntryPrice      float64 `json:"entry_price"`
	StopLoss        float64 `json:"stop_loss"`
	TakeProfit      float64 `json:"take_profit"`
	RiskPercent     float64 `json:"risk_percent"`
	RewardPercent   float64 `json:"reward_percent"`
	RiskRewardRatio float64 `json:"risk_reward_ratio"`
	CurrentPrice    float64 `json:"current_price,omitempty"`
}

// PatternUpdateBroadcaster provides WebSocket broadcasting for pattern updates.
// This implements the entrydecision.PatternUpdateBroadcaster interface.
type PatternUpdateBroadcaster struct {
	server *Server
}

// NewPatternUpdateBroadcaster creates a new pattern update broadcaster.
func NewPatternUpdateBroadcaster(server *Server) *PatternUpdateBroadcaster {
	return &PatternUpdateBroadcaster{server: server}
}

// BroadcastPatternUpdate broadcasts a pattern update to all connected WebSocket clients.
func (b *PatternUpdateBroadcaster) BroadcastPatternUpdate(update entrydecision.PatternUpdate) error {
	if userWSHub == nil {
		return nil
	}

	// Create event with the pattern update data
	event := events.Event{
		Type:      events.EventEntryDecisionPatternUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":       update.Symbol,
			"timeframe":    update.Timeframe,
			"mode":         update.Mode,
			"strategy":     update.Strategy,
			"sub_strategy": update.SubStrategy,
			"current_step": update.CurrentStep,
			"total_steps":  update.TotalSteps,
			"status":       string(update.Status),
			"step_details": update.StepDetails,
			"entry_levels": update.EntryLevels,
			"direction":    update.Direction,
			"updated_at":   update.UpdatedAt,
			// Step 3: Order filling fields
			"order_price":          update.OrderPrice,
			"order_quantity_usd":   update.OrderQuantityUSD,
			"fill_timeout_seconds": update.FillTimeoutSeconds,
			"fill_timeout_total":   update.FillTimeoutTotal,
		},
	}

	// Broadcast to all connected clients
	userWSHub.BroadcastToAll(event)

	return nil
}

// BroadcastEntryDecisionClear clears all cached entry decision data for a user and
// broadcasts a clear signal to the frontend. Called when the coin profiler stops.
func BroadcastEntryDecisionClear(userID string) {
	// 1. Remove user from all caches so broadcast loop stops sending stale data
	entryDecisionCacheMu.Lock()
	delete(patternMatcherCache, userID)
	delete(entryDecisionMatcherCache, userID)
	delete(scoreCalculatorCache, userID)
	entryDecisionCacheMu.Unlock()

	// 2. Clear lastBroadcastState entries for this user (they contain stale state keys)
	lastBroadcastStateMu.Lock()
	for key := range lastBroadcastState {
		delete(lastBroadcastState, key)
	}
	lastBroadcastStateMu.Unlock()

	// 3. Broadcast a clear event to the frontend
	clearData := map[string]interface{}{
		"strategies":          []interface{}{},
		"by_mode":             []interface{}{},
		"total_strategies":    0,
		"enabled_strategies":  0,
		"total_coins_ready":   0,
		"total_coins_watching": 0,
		"cleared":             true,
		"updated_at":          time.Now(),
	}
	BroadcastEntryDecisionUpdate(clearData)
	log.Printf("[ENTRY-DECISION-CLEAR] Cleared all entry decision data for user %s", userID)
}

// lastBroadcastState tracks the last broadcast state for each symbol to detect real changes
var (
	lastBroadcastStateMu sync.RWMutex
	lastBroadcastState   = make(map[string]struct {
		step   int
		status string
	})
)

// BroadcastPatternUpdateFunc is a standalone function that broadcasts pattern updates.
// This is used as the callback for UserAutopilotManager.SetPatternUpdateCallback.
// It wraps the event creation and broadcasting logic for real-time pattern state changes.
//
// IMPORTANT: This function broadcasts incremental pattern updates on every tick.
// Full ENTRY_DECISION_UPDATE is only sent when step/status changes to prevent flicker.
func BroadcastPatternUpdateFunc(update entrydecision.PatternUpdate) {
	if userWSHub == nil {
		return
	}

	// 1. ALWAYS broadcast the specific pattern update event (incremental - for real-time volume/price)
	// CRITICAL: Include reference_candle and volume_progress to prevent UI flicker
	patternEvent := events.Event{
		Type:      events.EventEntryDecisionPatternUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol":            update.Symbol,
			"timeframe":         update.Timeframe,
			"mode":              update.Mode,
			"strategy":          update.Strategy,
			"sub_strategy":      update.SubStrategy,
			"current_step":      update.CurrentStep,
			"total_steps":       update.TotalSteps,
			"status":            string(update.Status),
			"step_details":      update.StepDetails,
			"entry_levels":      update.EntryLevels,
			"reference_candle":  update.ReferenceCandle, // CRITICAL: Include for Step 2 preservation
			"volume_progress":   update.VolumeProgress,  // Include for real-time volume display
			"direction":         update.Direction,
			"looking_for":       update.LookingFor,
			"current_price":     update.CurrentPrice,
			"next_candle_close": update.NextCandleClose,
			"last_evaluated_at": update.LastEvaluatedAt,
			"updated_at":        update.UpdatedAt,
			// Step 3: Order filling fields
			"order_price":          update.OrderPrice,
			"order_quantity_usd":   update.OrderQuantityUSD,
			"fill_timeout_seconds": update.FillTimeoutSeconds,
			"fill_timeout_total":   update.FillTimeoutTotal,
		},
	}
	userWSHub.BroadcastToAll(patternEvent)

	// 2. Only broadcast full ENTRY_DECISION_UPDATE when there's a REAL state change
	// This prevents the full update from overwriting incremental updates on every tick
	stateKey := fmt.Sprintf("%s:%s:%s", update.Symbol, update.Mode, update.Timeframe)

	lastBroadcastStateMu.RLock()
	lastState, exists := lastBroadcastState[stateKey]
	lastBroadcastStateMu.RUnlock()

	isStateChange := !exists ||
		lastState.step != update.CurrentStep ||
		lastState.status != string(update.Status)

	if isStateChange {
		// Update tracked state
		lastBroadcastStateMu.Lock()
		lastBroadcastState[stateKey] = struct {
			step   int
			status string
		}{
			step:   update.CurrentStep,
			status: string(update.Status),
		}
		lastBroadcastStateMu.Unlock()

		// Broadcast full update on state change
		if update.UserID != "" && entryDecisionBroadcastServer != nil {
			data := buildEntryDecisionBroadcastData(update.UserID)
			if data != nil {
				BroadcastEntryDecisionUpdate(data)
				log.Printf("[ENTRY-DECISION-BROADCAST] State change for %s step=%d status=%s - sent full update",
					update.Symbol, update.CurrentStep, update.Status)
			}
		}
	}
}

// handleGetPatternUpdates handles GET /api/futures/entry-decision/pattern-updates
// Returns all current pattern updates (for initial page load).
func (s *Server) handleGetPatternUpdates(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	// Get realtime matcher
	realtimeMatcher := s.getOrCreateRealtimeMatcher(userID)
	if realtimeMatcher == nil {
		c.JSON(http.StatusOK, gin.H{
			"updates": []interface{}{},
			"count":   0,
		})
		return
	}

	// Get all pattern updates
	updates := realtimeMatcher.GetAllPatternUpdates()

	c.JSON(http.StatusOK, gin.H{
		"updates": updates,
		"count":   len(updates),
	})
}

// getOrCreateRealtimeMatcher returns or creates a realtime pattern matcher for a user.
func (s *Server) getOrCreateRealtimeMatcher(userID string) *entrydecision.RealtimePatternMatcher {
	entryDecisionCacheMu.RLock()
	// Check if we have a cached realtime matcher
	// For now, return nil - will be initialized when CoinProfiler integration is complete
	entryDecisionCacheMu.RUnlock()

	// Get pattern matcher
	patternMatcher := s.getOrCreatePatternMatcher(userID)
	if patternMatcher == nil {
		return nil
	}

	// Create realtime matcher
	realtimeMatcher := entrydecision.NewRealtimePatternMatcher(patternMatcher, nil)

	// Wire up broadcaster
	broadcaster := NewPatternUpdateBroadcaster(s)
	realtimeMatcher.WirePatternBroadcaster(broadcaster)

	return realtimeMatcher
}

// ==================== ENTRY LEVELS ENDPOINT ====================

// ==================== SETTINGS CHANGE BROADCASTING ====================

// SettingsChangeData represents the data sent in a SETTINGS_CHANGED WebSocket event.
type SettingsChangeData struct {
	UserID        string `json:"user_id"`
	ChangeType    string `json:"change_type"`     // "sub_strategy", "strategy_group", or "mode"
	Mode          string `json:"mode,omitempty"`  // Trading mode (scalp, swing, position)
	StrategyGroup string `json:"strategy_group,omitempty"`
	SubStrategy   string `json:"sub_strategy,omitempty"`
}

// BroadcastSettingsChanged broadcasts a settings change event to WebSocket clients.
// This enables the frontend to receive real-time notifications when settings are modified,
// allowing UI components to refresh their data without polling.
//
// Parameters:
//   - userID: The user whose settings changed
//   - changeType: Type of change - "sub_strategy", "strategy_group", or "mode"
//   - mode: Trading mode (scalp, swing, position) - may be empty
//   - strategyGroup: Strategy group name - may be empty
//   - subStrategy: Sub-strategy name - may be empty
func BroadcastSettingsChanged(userID, changeType, mode, strategyGroup, subStrategy string) {
	if userWSHub == nil {
		log.Printf("[SETTINGS-BROADCAST] WARNING: userWSHub is nil, cannot broadcast settings change")
		return
	}

	log.Printf("[SETTINGS-BROADCAST] Broadcasting settings change: user=%s type=%s mode=%s strategy=%s sub=%s",
		userID, changeType, mode, strategyGroup, subStrategy)

	event := events.Event{
		Type:      events.EventSettingsChanged,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"user_id":        userID,
			"change_type":    changeType,
			"mode":           mode,
			"strategy_group": strategyGroup,
			"sub_strategy":   subStrategy,
		},
	}

	// Broadcast to the specific user who made the change
	userWSHub.BroadcastToUser(userID, event)
	log.Printf("[SETTINGS-BROADCAST] Sent SETTINGS_CHANGED event to user %s", userID)
}

// handleGetEntryLevels handles GET /api/futures/entry-decision/entry-levels/:symbol
// Returns calculated entry, SL, and TP levels for a symbol.
func (s *Server) handleGetEntryLevels(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	symbol := strings.ToUpper(strings.TrimSpace(c.Param("symbol")))
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_SYMBOL",
			"message": "Symbol is required",
		})
		return
	}

	mode := strings.ToLower(c.Query("mode"))
	if mode == "" {
		mode = "scalp"
	}

	timeframe := c.Query("timeframe")
	if timeframe == "" {
		timeframe = getDefaultTimeframeForMode(mode)
	}

	// Get realtime matcher
	realtimeMatcher := s.getOrCreateRealtimeMatcher(userID)
	if realtimeMatcher == nil {
		c.JSON(http.StatusOK, gin.H{
			"symbol":  symbol,
			"message": "No active pattern for entry level calculation",
		})
		return
	}

	// Get pattern update with entry levels
	update := realtimeMatcher.GetPatternUpdate(symbol, mode, timeframe)
	if update == nil || update.EntryLevels == nil {
		c.JSON(http.StatusOK, gin.H{
			"symbol":  symbol,
			"message": "No entry levels available - pattern not at required stage",
		})
		return
	}

	response := EntryLevelsResponse{
		Symbol:          symbol,
		EntryPrice:      update.EntryLevels.EntryPrice,
		StopLoss:        update.EntryLevels.StopLoss,
		TakeProfit:      update.EntryLevels.TakeProfit,
		RiskPercent:     update.EntryLevels.RiskPercent,
		RewardPercent:   update.EntryLevels.RewardPercent,
		RiskRewardRatio: update.EntryLevels.RiskRewardRatio,
		CurrentPrice:    update.EntryLevels.CurrentPrice,
	}

	c.JSON(http.StatusOK, response)
}
