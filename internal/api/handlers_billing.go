package api

import (
	"io"
	"net/http"

	"binance-trading-bot/internal/billing"
	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// handleGetProfitHistory returns the user's profit history
func (s *Server) handleGetProfitHistory(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	periods, err := s.repo.GetUserProfitPeriods(ctx, userID, 50)
	if err != nil {
		// Return empty array if table doesn't exist or other error
		// This is expected for new users or if billing hasn't been set up
		successResponse(c, []gin.H{})
		return
	}

	// Convert to response format
	response := make([]gin.H, len(periods))
	for i, period := range periods {
		response[i] = gin.H{
			"id":                period.ID,
			"period_start":      period.PeriodStart,
			"period_end":        period.PeriodEnd,
			"starting_balance":  period.StartingBalance,
			"ending_balance":    period.EndingBalance,
			"gross_profit":      period.GrossProfit,
			"net_profit":        period.NetProfit,
			"profit_share_rate": period.ProfitShareRate,
			"profit_share_due":  period.ProfitShareDue,
			"settlement_status": period.SettlementStatus,
			"created_at":        period.CreatedAt,
		}
	}

	successResponse(c, response)
}

// handleGetInvoices returns the user's invoices
func (s *Server) handleGetInvoices(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Get profit periods with invoice IDs
	periods, err := s.repo.GetUserProfitPeriods(ctx, userID, 50)
	if err != nil {
		// Return empty array if table doesn't exist or other error
		successResponse(c, []gin.H{})
		return
	}

	// Convert to invoice format
	invoices := make([]gin.H, 0)
	for _, period := range periods {
		if period.StripeInvoiceID != nil && *period.StripeInvoiceID != "" {
			invoices = append(invoices, gin.H{
				"id":         *period.StripeInvoiceID,
				"amount":     period.ProfitShareDue,
				"status":     period.SettlementStatus,
				"created_at": period.CreatedAt,
				"pdf_url":    nil, // Would need to fetch from Stripe
			})
		}
	}

	successResponse(c, invoices)
}

// handleCreateCheckoutSession creates a Stripe checkout session for subscription upgrade
func (s *Server) handleCreateCheckoutSession(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	var req struct {
		Tier string `json:"tier" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "tier is required")
		return
	}

	ctx := c.Request.Context()

	// Check if billing service is available
	if s.billingService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "billing service not available")
		return
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		errorResponse(c, http.StatusNotFound, "user not found")
		return
	}

	// Get or create Stripe customer
	customerID, err := s.billingService.GetOrCreateCustomer(ctx, user)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to get customer ID")
		return
	}

	// Map tier string to subscription tier
	tier := billing.SubscriptionTier(req.Tier)
	if !isValidTier(tier) {
		errorResponse(c, http.StatusBadRequest, "invalid subscription tier")
		return
	}

	// Build URLs based on request origin
	baseURL := c.Request.Header.Get("Origin")
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}
	successURL := baseURL + "/billing?success=true"
	cancelURL := baseURL + "/billing?canceled=true"

	// Create checkout session
	checkoutURL, err := s.billingService.CreateCheckoutSession(ctx, customerID, tier, successURL, cancelURL)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to create checkout session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"checkout_url": checkoutURL,
	})
}

// handleCreateCustomerPortal creates a Stripe customer portal session
func (s *Server) handleCreateCustomerPortal(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()

	// Check if billing service is available
	if s.billingService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "billing service not available")
		return
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		errorResponse(c, http.StatusNotFound, "user not found")
		return
	}

	// User must have a Stripe customer ID
	if user.StripeCustomerID == "" {
		errorResponse(c, http.StatusBadRequest, "no billing account found")
		return
	}

	// Build return URL based on request origin
	returnURL := c.Request.Header.Get("Origin")
	if returnURL == "" {
		returnURL = "http://localhost:5173"
	}
	returnURL += "/billing"

	// Create portal session
	portalURL, err := s.billingService.CreatePortalSession(ctx, user.StripeCustomerID, returnURL)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to create portal session")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"portal_url": portalURL,
	})
}

// handleStripeWebhook handles Stripe webhook events
func (s *Server) handleStripeWebhook(c *gin.Context) {
	// Check if billing service is available
	if s.billingService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "billing service not available")
		return
	}

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "failed to read request body")
		return
	}

	// Get the Stripe signature header
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		errorResponse(c, http.StatusBadRequest, "missing Stripe signature")
		return
	}

	ctx := c.Request.Context()

	// Handle the webhook
	if err := s.billingService.HandleWebhook(ctx, payload, signature); err != nil {
		errorResponse(c, http.StatusBadRequest, "webhook processing failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"received": true,
	})
}

// Helper function to validate subscription tier
func isValidTier(tier billing.SubscriptionTier) bool {
	switch tier {
	case billing.TierFree, billing.TierTrader, billing.TierPro, billing.TierWhale:
		return true
	default:
		return false
	}
}

// Helper to get user struct matching what billing service expects
func convertUser(user *database.User) *database.User {
	return user
}
