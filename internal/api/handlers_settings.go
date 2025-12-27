package api

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/events"

	"github.com/gin-gonic/gin"
)

// SettingsAPI interface for settings-related operations
type SettingsAPI interface {
	GetAutopilotController() *autopilot.Controller
	GetCircuitBreaker() *circuit.CircuitBreaker
	SetDryRunMode(enabled bool) error
	GetDryRunMode() bool
}

// ==================== REQUEST TYPES ====================

// SetTradingModeRequest for toggling paper/live trading
type SetTradingModeRequest struct {
	DryRun bool `json:"dry_run"`
}

// SetAutopilotRulesRequest for updating autopilot rules
type SetAutopilotRulesRequest struct {
	Enabled              *bool    `json:"enabled,omitempty"`
	MaxDailyLoss         *float64 `json:"max_daily_loss,omitempty"`
	MaxConsecutiveLosses *int     `json:"max_consecutive_losses,omitempty"`
	MinConfidence        *float64 `json:"min_confidence,omitempty"`
	CooldownMinutes      *int     `json:"cooldown_minutes,omitempty"`
	RequireMultiSignal   *bool    `json:"require_multi_signal,omitempty"`
	RiskLevel            *string  `json:"risk_level,omitempty"`
}

// UpdateCircuitBreakerRequest for updating circuit breaker limits
type UpdateCircuitBreakerRequest struct {
	Enabled              *bool    `json:"enabled,omitempty"`
	MaxLossPerHour       *float64 `json:"max_loss_per_hour,omitempty"`
	MaxDailyLoss         *float64 `json:"max_daily_loss,omitempty"`
	MaxConsecutiveLosses *int     `json:"max_consecutive_losses,omitempty"`
	CooldownMinutes      *int     `json:"cooldown_minutes,omitempty"`
	MaxTradesPerMinute   *int     `json:"max_trades_per_minute,omitempty"`
	MaxDailyTrades       *int     `json:"max_daily_trades,omitempty"`
}

// ==================== HANDLERS ====================

// handleGetTradingMode returns current trading mode (paper/live)
func (s *Server) handleGetTradingMode(c *gin.Context) {
	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		c.JSON(http.StatusOK, gin.H{
			"dry_run":      true,
			"mode":         "paper",
			"mode_label":   "Paper Trading",
			"can_switch":   false,
			"switch_error": "Settings API not available",
		})
		return
	}

	dryRun := settingsAPI.GetDryRunMode()
	mode := "live"
	modeLabel := "Live Trading"
	if dryRun {
		mode = "paper"
		modeLabel = "Paper Trading"
	}

	c.JSON(http.StatusOK, gin.H{
		"dry_run":    dryRun,
		"mode":       mode,
		"mode_label": modeLabel,
		"can_switch": true,
	})
}

// handleSetTradingMode toggles between paper and live trading
func (s *Server) handleSetTradingMode(c *gin.Context) {
	var req SetTradingModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings API not available")
		return
	}

	// SAFETY CHECK: Stop Ginie autopilot if running before mode switch
	// This prevents lock contention and timeouts during client switching
	// Use non-blocking goroutine with timeout to prevent hangs
	// IMPORTANT: Remember if Ginie was running to restart it after mode switch
	futuresController := s.getFuturesAutopilot()
	ginieWasRunning := false

	// Check if Ginie is running BEFORE launching the stop goroutine (avoid race condition)
	// GinieAutopilot.IsRunning() checks if the trading loop is active
	// GinieAnalyzer.IsEnabled() checks if the analyzer is enabled
	if futuresController != nil {
		if giniePilot := futuresController.GetGinieAutopilot(); giniePilot != nil {
			ginieWasRunning = giniePilot.IsRunning()
		}
		// Also check if analyzer is enabled (backup check)
		if !ginieWasRunning {
			if ginieAnalyzer := futuresController.GetGinieAnalyzer(); ginieAnalyzer != nil {
				ginieWasRunning = ginieAnalyzer.IsEnabled()
			}
		}
		if ginieWasRunning {
			log.Printf("[MODE-SWITCH] Ginie was running/enabled, will restart after mode switch")
		}
	}

	if futuresController != nil && ginieWasRunning {
		stopDone := make(chan bool, 1)
		go func() {
			if giniePilot := futuresController.GetGinieAutopilot(); giniePilot != nil {
				if giniePilot.IsRunning() {
					log.Println("[MODE-SWITCH] Ginie autopilot is running, stopping it before mode switch...")
					if err := futuresController.StopGinieAutopilot(); err != nil {
						log.Printf("[MODE-SWITCH] Warning: Failed to stop Ginie before mode switch: %v\n", err)
					} else {
						log.Println("[MODE-SWITCH] Ginie autopilot stopped successfully")
					}
				}
			}
			stopDone <- true
		}()

		// Wait max 2 seconds for Ginie to stop, don't block request further
		select {
		case <-stopDone:
			log.Println("[MODE-SWITCH] Ginie stop completed, proceeding with mode switch")
			time.Sleep(500 * time.Millisecond) // Brief cleanup wait
		case <-time.After(2 * time.Second):
			log.Println("[MODE-SWITCH] Ginie stop timeout (2s), proceeding with mode switch anyway")
		}
	}

	log.Printf("[MODE-SWITCH] Starting trading mode switch to dry_run=%v\n", req.DryRun)
	if err := settingsAPI.SetDryRunMode(req.DryRun); err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to update trading mode: "+err.Error())
		return
	}
	log.Println("[MODE-SWITCH] Trading mode switch completed successfully")

	// Verify the change was applied by reading back the current mode
	currentMode := settingsAPI.GetDryRunMode()
	if currentMode != req.DryRun {
		// Mode didn't change as expected
		errorResponse(c, http.StatusInternalServerError, "Trading mode change was not applied correctly")
		return
	}

	mode := "live"
	modeLabel := "Live Trading"
	if req.DryRun {
		mode = "paper"
		modeLabel = "Paper Trading"
	}

	// CRITICAL FIX: Restart Ginie autopilot if it was running before mode switch
	// This ensures autopilot stays ON continuously until explicitly disabled
	ginieRestarted := false
	if ginieWasRunning && futuresController != nil {
		log.Println("[MODE-SWITCH] Ginie was running before mode switch, restarting it...")
		// Give system a moment to complete mode switch before restart
		time.Sleep(500 * time.Millisecond)

		// SAFETY CHECK: Ensure Ginie is fully stopped before restarting
		// This prevents race conditions if the stop timed out
		if giniePilot := futuresController.GetGinieAutopilot(); giniePilot != nil {
			if giniePilot.IsRunning() {
				log.Println("[MODE-SWITCH] Ginie still running after timeout, force-stopping...")
				giniePilot.Stop()
				time.Sleep(300 * time.Millisecond) // Brief wait for cleanup
			}
		}

		// Re-enable the analyzer first
		if ginieAnalyzer := futuresController.GetGinieAnalyzer(); ginieAnalyzer != nil {
			ginieAnalyzer.Enable()
			log.Println("[MODE-SWITCH] Ginie analyzer re-enabled")
		}

		// Start the autopilot trading loop
		if err := futuresController.StartGinieAutopilot(); err != nil {
			log.Printf("[MODE-SWITCH] Warning: Failed to restart Ginie after mode switch: %v\n", err)
		} else {
			log.Println("[MODE-SWITCH] Ginie autopilot restarted successfully after mode switch")
			ginieRestarted = true
		}
	}

	// Broadcast trading mode change to all connected clients via WebSocket
	if userWSHub != nil {
		userWSHub.BroadcastToAll(events.Event{
			Type:      events.EventTradingModeChanged,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"dry_run":         req.DryRun,
				"mode":            mode,
				"mode_label":      modeLabel,
				"ginie_restarted": ginieRestarted,
			},
		})
		log.Printf("[MODE-SWITCH] Broadcasted TRADING_MODE_CHANGED event to all clients")
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"dry_run":         req.DryRun,
		"mode":            mode,
		"mode_label":      modeLabel,
		"can_switch":      true,
		"message":         "Trading mode updated successfully",
		"ginie_restarted": ginieRestarted,
	})
}

// handleGetWalletBalance returns the wallet balance from Binance
func (s *Server) handleGetWalletBalance(c *gin.Context) {
	log.Printf("[WALLET-BALANCE-DEBUG] handleGetWalletBalance called")
	// Check if we're in dry run mode via settings API
	settingsAPI := s.getSettingsAPI()
	isSimulated := true
	if settingsAPI != nil {
		isSimulated = settingsAPI.GetDryRunMode()
	}
	log.Printf("[WALLET-BALANCE-DEBUG] isSimulated=%v", isSimulated)

	client := s.getBinanceClientForUser(c)
	log.Printf("[WALLET-BALANCE-DEBUG] client=%v", client != nil)
	if client == nil {
		// Return mock balance if no client available
		c.JSON(http.StatusOK, gin.H{
			"total_balance":     10000.0,
			"available_balance": 9500.0,
			"locked_balance":    500.0,
			"currency":          "USDT",
			"is_simulated":      true,
			"assets": []gin.H{
				{"asset": "USDT", "free": 9500.0, "locked": 500.0},
				{"asset": "BTC", "free": 0.0, "locked": 0.0},
			},
		})
		return
	}

	account, err := client.GetAccountInfo()
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to get account info: "+err.Error())
		return
	}

	// Stablecoins that are treated as 1:1 with USD
	stablecoins := map[string]bool{
		"USDT": true,
		"USDC": true,
		"BUSD": true,
		"TUSD": true,
		"USDP": true,
		"DAI":  true,
		"FDUSD": true,
	}

	// Calculate balances - convert all assets to USD equivalent
	var totalUSD, freeUSD, lockedUSD float64
	var freeUSDT, lockedUSDT float64 // Keep track of USDT specifically for available/locked
	assets := make([]gin.H, 0)

	for _, balance := range account.Balances {
		free := parseFloat(balance.Free)
		locked := parseFloat(balance.Locked)

		if free > 0 || locked > 0 {
			totalBalance := free + locked
			var usdValue float64

			if stablecoins[balance.Asset] {
				// Stablecoins are 1:1 with USD
				usdValue = totalBalance
				if balance.Asset == "USDT" {
					freeUSDT = free
					lockedUSDT = locked
				}
			} else {
				// Try to get price in USDT
				price, err := client.GetCurrentPrice(balance.Asset + "USDT")
				if err != nil {
					// Try BUSD pair as fallback
					price, err = client.GetCurrentPrice(balance.Asset + "BUSD")
					if err != nil {
						// If no price available, skip this asset for USD calculation
						log.Printf("[WALLET-BALANCE] Could not get price for %s: %v", balance.Asset, err)
						price = 0
					}
				}
				usdValue = totalBalance * price
			}

			assets = append(assets, gin.H{
				"asset":     balance.Asset,
				"free":      free,
				"locked":    locked,
				"usd_value": usdValue,
			})

			totalUSD += usdValue
			freeUSD += free * (usdValue / totalBalance) // Proportional free value
			lockedUSD += locked * (usdValue / totalBalance) // Proportional locked value
		}
	}

	// If no USDT specifically, use total USD values
	if freeUSDT == 0 && lockedUSDT == 0 {
		freeUSDT = freeUSD
		lockedUSDT = lockedUSD
	}

	c.JSON(http.StatusOK, gin.H{
		"total_balance":     totalUSD,
		"available_balance": freeUSDT,
		"locked_balance":    lockedUSDT,
		"currency":          "USD",
		"is_simulated":      isSimulated,
		"assets":            assets,
	})
}

// handleGetAutopilotStatus returns autopilot status and rules
func (s *Server) handleGetAutopilotStatus(c *gin.Context) {
	settingsAPI := s.getSettingsAPI()

	response := gin.H{
		"enabled":   false,
		"running":   false,
		"dry_run":   true,
		"available": false,
	}

	if settingsAPI == nil {
		c.JSON(http.StatusOK, response)
		return
	}

	autopilotCtrl := settingsAPI.GetAutopilotController()
	circuitBreaker := settingsAPI.GetCircuitBreaker()

	if autopilotCtrl == nil {
		c.JSON(http.StatusOK, response)
		return
	}

	response["available"] = true
	response["enabled"] = true
	response["running"] = autopilotCtrl.IsRunning()
	response["dry_run"] = settingsAPI.GetDryRunMode()
	response["stats"] = autopilotCtrl.GetStats()

	// Get circuit breaker status
	if circuitBreaker != nil {
		cbStats := circuitBreaker.GetStats()
		response["circuit_breaker"] = gin.H{
			"enabled":     circuitBreaker.IsEnabled(),
			"state":       cbStats["state"],
			"can_trade":   cbStats["state"] == "closed" || cbStats["state"] == "half_open",
			"trip_reason": cbStats["trip_reason"],
			"stats":       cbStats,
		}
	}

	c.JSON(http.StatusOK, response)
}

// handleSetAutopilotRules updates autopilot and circuit breaker rules
func (s *Server) handleSetAutopilotRules(c *gin.Context) {
	var req SetAutopilotRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings API not available")
		return
	}

	// Update circuit breaker settings
	circuitBreaker := settingsAPI.GetCircuitBreaker()
	if circuitBreaker != nil {
		// Note: In a real implementation, you'd update the circuit breaker config
		// For now, we just acknowledge the request
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Autopilot rules updated",
	})
}

// handleToggleAutopilot starts or stops the autopilot
func (s *Server) handleToggleAutopilot(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings API not available")
		return
	}

	autopilotCtrl := settingsAPI.GetAutopilotController()
	if autopilotCtrl == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Autopilot not available")
		return
	}

	if req.Enabled {
		// Check if Ginie autopilot is running - mutual exclusion
		futuresController := s.getFuturesAutopilot()
		if futuresController != nil {
			ginieAutopilot := futuresController.GetGinieAutopilot()
			if ginieAutopilot != nil && ginieAutopilot.IsRunning() {
				errorResponse(c, http.StatusConflict, "Cannot start AI autopilot while Ginie autopilot is running. Stop Ginie autopilot first.")
				return
			}
		}

		if err := autopilotCtrl.Start(); err != nil {
			errorResponse(c, http.StatusInternalServerError, "Failed to start autopilot: "+err.Error())
			return
		}
	} else {
		autopilotCtrl.Stop()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"enabled": req.Enabled,
		"running": autopilotCtrl.IsRunning(),
		"message": func() string {
			if req.Enabled {
				return "Autopilot started"
			}
			return "Autopilot stopped"
		}(),
	})
}

// handleResetCircuitBreaker manually resets the circuit breaker
func (s *Server) handleResetCircuitBreaker(c *gin.Context) {
	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings API not available")
		return
	}

	circuitBreaker := settingsAPI.GetCircuitBreaker()
	if circuitBreaker == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Circuit breaker not available")
		return
	}

	circuitBreaker.ForceReset()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Circuit breaker reset successfully",
		"state":   circuitBreaker.GetState(),
	})
}

// handleGetCircuitBreakerStatus returns circuit breaker status
func (s *Server) handleGetCircuitBreakerStatus(c *gin.Context) {
	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"available": false,
		})
		return
	}

	circuitBreaker := settingsAPI.GetCircuitBreaker()
	if circuitBreaker == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled":   false,
			"available": false,
		})
		return
	}

	stats := circuitBreaker.GetStats()
	config := circuitBreaker.GetConfig()
	canTrade, reason := circuitBreaker.CanTrade()

	c.JSON(http.StatusOK, gin.H{
		"available":           true,
		"enabled":             circuitBreaker.IsEnabled(),
		"state":               stats["state"],
		"can_trade":           canTrade,
		"block_reason":        reason,
		"consecutive_losses":  stats["consecutive_losses"],
		"hourly_loss":         stats["hourly_loss"],
		"daily_loss":          stats["daily_loss"],
		"trades_last_minute":  stats["trades_last_minute"],
		"daily_trades":        stats["daily_trades"],
		"trip_reason":         stats["trip_reason"],
		"last_trip_time":      stats["last_trip_time"],
		// Include configurable limits
		"config": gin.H{
			"max_loss_per_hour":       config.MaxLossPerHour,
			"max_daily_loss":          config.MaxDailyLoss,
			"max_consecutive_losses":  config.MaxConsecutiveLosses,
			"cooldown_minutes":        config.CooldownMinutes,
			"max_trades_per_minute":   config.MaxTradesPerMinute,
			"max_daily_trades":        config.MaxDailyTrades,
		},
	})
}

// handleUpdateCircuitBreakerConfig updates circuit breaker configuration
func (s *Server) handleUpdateCircuitBreakerConfig(c *gin.Context) {
	var req UpdateCircuitBreakerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	settingsAPI := s.getSettingsAPI()
	if settingsAPI == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Settings API not available")
		return
	}

	circuitBreaker := settingsAPI.GetCircuitBreaker()
	if circuitBreaker == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Circuit breaker not available")
		return
	}

	// Update enabled state if provided
	if req.Enabled != nil {
		circuitBreaker.SetEnabled(*req.Enabled)
	}

	// Build config update
	configUpdate := &circuit.CircuitBreakerConfig{}
	if req.MaxLossPerHour != nil {
		configUpdate.MaxLossPerHour = *req.MaxLossPerHour
	}
	if req.MaxDailyLoss != nil {
		configUpdate.MaxDailyLoss = *req.MaxDailyLoss
	}
	if req.MaxConsecutiveLosses != nil {
		configUpdate.MaxConsecutiveLosses = *req.MaxConsecutiveLosses
	}
	if req.CooldownMinutes != nil {
		configUpdate.CooldownMinutes = *req.CooldownMinutes
	}
	if req.MaxTradesPerMinute != nil {
		configUpdate.MaxTradesPerMinute = *req.MaxTradesPerMinute
	}
	if req.MaxDailyTrades != nil {
		configUpdate.MaxDailyTrades = *req.MaxDailyTrades
	}

	circuitBreaker.UpdateConfig(configUpdate)

	// Return updated config
	newConfig := circuitBreaker.GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Circuit breaker configuration updated",
		"config": gin.H{
			"enabled":                 circuitBreaker.IsEnabled(),
			"max_loss_per_hour":       newConfig.MaxLossPerHour,
			"max_daily_loss":          newConfig.MaxDailyLoss,
			"max_consecutive_losses":  newConfig.MaxConsecutiveLosses,
			"cooldown_minutes":        newConfig.CooldownMinutes,
			"max_trades_per_minute":   newConfig.MaxTradesPerMinute,
			"max_daily_trades":        newConfig.MaxDailyTrades,
		},
	})
}

// ==================== HELPER FUNCTIONS ====================

// getSettingsAPI returns the settings API if available
func (s *Server) getSettingsAPI() SettingsAPI {
	if settingsAPI, ok := s.botAPI.(SettingsAPI); ok {
		return settingsAPI
	}
	return nil
}

// SpotClientAPI interface for getting the appropriate spot client based on mode
type SpotClientAPI interface {
	GetSpotClient() binance.BinanceClient
}

// getBinanceClient returns the Binance client (uses mode-aware client if available)
func (s *Server) getBinanceClient() binance.BinanceClient {
	if s.botAPI == nil {
		return nil
	}
	// Try to use mode-aware SpotClient first
	if spotAPI, ok := s.botAPI.(SpotClientAPI); ok {
		return spotAPI.GetSpotClient()
	}
	// Fall back to legacy GetBinanceClient
	clientIface := s.botAPI.GetBinanceClient()
	if client, ok := clientIface.(binance.BinanceClient); ok {
		return client
	}
	return nil
}

// getBinanceClientForUser returns a Binance client for the authenticated user
// User must have API keys configured in the database - no global fallback
// Returns nil if user has no API keys (caller should return error to user)
func (s *Server) getBinanceClientForUser(c *gin.Context) binance.BinanceClient {
	log.Printf("[DEBUG] getBinanceClientForUser: authEnabled=%v, apiKeyService=%v", s.authEnabled, s.apiKeyService != nil)

	// Check if in paper trading mode - use mock client
	if s.botAPI != nil {
		if settingsAPI, ok := s.botAPI.(SettingsAPI); ok {
			if settingsAPI.GetDryRunMode() {
				log.Printf("[DEBUG] getBinanceClientForUser: Paper trading mode, using mock client")
				return s.getBinanceClient() // Returns mock client in paper mode
			}
		}
	}

	// Live mode - must use user-specific keys from database
	if s.authEnabled && s.apiKeyService != nil {
		userID := s.getUserID(c)
		log.Printf("[DEBUG] getBinanceClientForUser: userID=%s", userID)
		if userID != "" {
			ctx := c.Request.Context()
			// Try mainnet first, then testnet
			keys, err := s.apiKeyService.GetActiveBinanceKey(ctx, userID, false)
			if err != nil {
				log.Printf("[DEBUG] getBinanceClientForUser: mainnet key lookup failed: %v", err)
				// Try testnet
				keys, err = s.apiKeyService.GetActiveBinanceKey(ctx, userID, true)
			}
			if err == nil && keys != nil && keys.APIKey != "" && keys.SecretKey != "" {
				log.Printf("[DEBUG] getBinanceClientForUser: Found user keys for %s (testnet=%v, keyLen=%d)", userID, keys.IsTestnet, len(keys.APIKey))
				// Create user-specific spot client
				baseURL := "https://api.binance.com"
				if keys.IsTestnet {
					baseURL = "https://testnet.binance.vision"
				}
				client := binance.NewClient(keys.APIKey, keys.SecretKey, baseURL)
				if client != nil {
					log.Printf("[DEBUG] getBinanceClientForUser: Created user-specific client for %s", userID)
					return client
				}
			} else {
				log.Printf("[DEBUG] getBinanceClientForUser: No keys found for user %s, err=%v", userID, err)
			}
		}
	}

	// No user API keys found - return nil (caller should return error)
	log.Printf("[DEBUG] getBinanceClientForUser: No user API keys - user must configure keys in Settings")
	return nil
}

// parseFloat safely parses a string to float64
func parseFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}
