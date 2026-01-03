package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ==================== REQUEST/RESPONSE TYPES ====================

// UpdatePaperBalanceRequest for manual balance update
type UpdatePaperBalanceRequest struct {
	Balance float64 `json:"balance" binding:"required"`
}

// ==================== HANDLERS ====================

// handleGetPaperBalance returns the current paper balance for the authenticated user
// GET /api/settings/paper-balance
func (s *Server) handleGetPaperBalance(c *gin.Context) {
	// Get user ID from auth context
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()

	// Get paper balance and dry run mode from database
	balance, dryRunMode, err := s.repo.GetUserPaperBalance(ctx, userID)
	if err != nil {
		log.Printf("[PAPER-BALANCE] Error getting paper balance for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to retrieve paper balance")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"paper_balance_usdt": fmt.Sprintf("%.8f", balance),
		"dry_run_mode":       dryRunMode,
	})
}

// handleUpdatePaperBalance updates the paper balance for the authenticated user
// PUT /api/settings/paper-balance
func (s *Server) handleUpdatePaperBalance(c *gin.Context) {
	var req UpdatePaperBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user ID from auth context
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	// Validate balance range
	if req.Balance < 10.0 || req.Balance > 1000000.0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Balance must be between $10 and $1,000,000",
		})
		return
	}

	ctx := c.Request.Context()

	// Update paper balance in database
	if err := s.repo.SetUserPaperBalance(ctx, userID, req.Balance); err != nil {
		log.Printf("[PAPER-BALANCE] Failed to update paper balance for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to update paper balance")
		return
	}

	log.Printf("[PAPER-BALANCE] User %s updated paper balance to %.2f", userID, req.Balance)

	c.JSON(http.StatusOK, gin.H{
		"paper_balance_usdt": fmt.Sprintf("%.8f", req.Balance),
		"message":            "Paper balance updated successfully",
	})
}

// handleSyncPaperBalance syncs paper balance from real Binance account balance
// POST /api/settings/sync-paper-balance
func (s *Server) handleSyncPaperBalance(c *gin.Context) {
	// Get user ID from auth context
	userID := s.getUserID(c)
	if userID == "" {
		errorResponse(c, http.StatusUnauthorized, "User authentication required")
		return
	}

	ctx := c.Request.Context()

	// Get user's Binance client (requires API keys configured)
	client := s.getBinanceClientForUser(c)
	if client == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":           "Binance API credentials not configured",
			"action_required": "Please add your API keys in Settings",
		})
		return
	}

	// Fetch USDT balance from Binance Spot account
	balance, err := client.GetUSDTBalance()
	if err != nil {
		log.Printf("[PAPER-BALANCE] Failed to fetch USDT balance from Binance for user %s: %v", userID, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error":           "Failed to fetch balance from Binance",
			"details":         err.Error(),
			"retry_suggested": true,
		})
		return
	}

	// Validate balance is within allowed range
	if balance < 10.0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Synced balance is below minimum ($10)",
			"balance": balance,
		})
		return
	}

	if balance > 1000000.0 {
		balance = 1000000.0 // Cap at max
		log.Printf("[PAPER-BALANCE] User %s synced balance exceeds max, capping at $1M", userID)
	}

	// Update paper balance in database
	if err := s.repo.SetUserPaperBalance(ctx, userID, balance); err != nil {
		log.Printf("[PAPER-BALANCE] Failed to save synced balance for user %s: %v", userID, err)
		errorResponse(c, http.StatusInternalServerError, "Failed to save synced balance")
		return
	}

	log.Printf("[PAPER-BALANCE] User %s synced paper balance from Binance: %.2f USDT", userID, balance)

	c.JSON(http.StatusOK, gin.H{
		"paper_balance_usdt": fmt.Sprintf("%.8f", balance),
		"synced_from":        "binance_spot_account",
		"message":            "Paper balance synced successfully",
	})
}
