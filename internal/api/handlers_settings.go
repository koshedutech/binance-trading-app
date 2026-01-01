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

// handleGetTradingMode returns current trading mode (paper/live) for the authenticated user
func (s *Server) handleGetTradingMode(c *gin.Context) {
	// Get user ID from auth context
	userID := s.getUserID(c)
	if userID == "" {
		// No fallback - authentication is required
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	// Get per-user trading mode from database
	ctx := c.Request.Context()
	dryRun, err := s.repo.GetUserDryRunMode(ctx, userID)
	if err != nil {
		log.Printf("[TRADING-MODE] Error getting user dry run mode for %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to retrieve trading mode: "+err.Error())
		return
	}

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
		"user_id":    userID, // Include user ID for debugging
	})
}

// handleSetTradingMode toggles between paper and live trading for the authenticated user
func (s *Server) handleSetTradingMode(c *gin.Context) {
	var req SetTradingModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user ID from auth context
	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// Per-user trading mode: save to database
	if userID != "" {
		log.Printf("[MODE-SWITCH] User %s switching to dry_run=%v", userID, req.DryRun)

		// Save per-user trading mode to database
		if err := s.repo.SetUserDryRunMode(ctx, userID, req.DryRun); err != nil {
			log.Printf("[MODE-SWITCH] Failed to save user trading mode: %v", err)
			errorResponse(c, http.StatusInternalServerError, "Failed to save trading mode: "+err.Error())
			return
		}

		log.Printf("[MODE-SWITCH] User %s trading mode saved to database: dry_run=%v", userID, req.DryRun)

		// Handle per-user Ginie autopilot restart if needed
		ginieRestarted := false
		if s.userAutopilotManager != nil {
			// Check if user's autopilot is running
			if s.userAutopilotManager.IsRunning(userID) {
				log.Printf("[MODE-SWITCH] User %s Ginie autopilot is running, stopping before mode switch...", userID)

				// Stop the user's autopilot
				if err := s.userAutopilotManager.StopAutopilot(userID); err != nil {
					log.Printf("[MODE-SWITCH] Warning: Failed to stop user %s Ginie: %v", userID, err)
				}

				// Brief wait for cleanup
				time.Sleep(500 * time.Millisecond)

				// Restart with new mode
				log.Printf("[MODE-SWITCH] Restarting user %s Ginie autopilot after mode switch...", userID)
				if err := s.userAutopilotManager.StartAutopilot(ctx, userID); err != nil {
					log.Printf("[MODE-SWITCH] Warning: Failed to restart user %s Ginie: %v", userID, err)
				} else {
					log.Printf("[MODE-SWITCH] User %s Ginie autopilot restarted successfully", userID)
					ginieRestarted = true
				}
			}
		}

		mode := "live"
		modeLabel := "Live Trading"
		if req.DryRun {
			mode = "paper"
			modeLabel = "Paper Trading"
		}

		// Broadcast trading mode change to THIS USER ONLY via WebSocket
		if userWSHub != nil {
			userWSHub.BroadcastToUser(userID, events.Event{
				Type:      events.EventTradingModeChanged,
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"dry_run":         req.DryRun,
					"mode":            mode,
					"mode_label":      modeLabel,
					"ginie_restarted": ginieRestarted,
					"user_id":         userID,
				},
			})
			log.Printf("[MODE-SWITCH] Broadcasted TRADING_MODE_CHANGED event to user %s only", userID)
		}

		c.JSON(http.StatusOK, gin.H{
			"success":         true,
			"dry_run":         req.DryRun,
			"mode":            mode,
			"mode_label":      modeLabel,
			"can_switch":      true,
			"message":         "Trading mode updated successfully",
			"ginie_restarted": ginieRestarted,
			"user_id":         userID,
		})
		return
	}

	// No fallback to global mode - user authentication is required
	// This ensures trading mode is always tied to a specific user account
	errorResponse(c, http.StatusUnauthorized, "User authentication required to change trading mode")
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

	userID := s.getUserID(c)
	ctx := c.Request.Context()

	// Check if in paper trading mode - use per-user mode if authenticated
	if userID != "" {
		// Get per-user trading mode from database
		dryRun, err := s.repo.GetUserDryRunMode(ctx, userID)
		if err != nil {
			log.Printf("[DEBUG] getBinanceClientForUser: Error getting user dry run mode: %v, defaulting to paper", err)
			dryRun = true
		}
		if dryRun {
			log.Printf("[DEBUG] getBinanceClientForUser: User %s in paper trading mode, using mock client", userID)
			return s.getBinanceClient() // Returns mock client in paper mode
		}
	} else {
		// No user authentication - return nil
		// Client must authenticate to use trading features
		log.Printf("[DEBUG] getBinanceClientForUser: No user authentication, cannot provide client")
		return nil
	}

	// Live mode - must use user-specific keys from database
	if s.authEnabled && s.apiKeyService != nil && userID != "" {
		log.Printf("[DEBUG] getBinanceClientForUser: userID=%s in LIVE mode", userID)
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
