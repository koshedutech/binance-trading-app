// Package api provides HTTP handlers for daily settlement and analytics.
// Epic 8 Stories 8.5 (Admin Dashboard), 8.6 (Historical Reports), 8.8 (Admin Retry)
package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"binance-trading-bot/internal/database"

	"github.com/gin-gonic/gin"
)

// contextKeyUserID is the context key for user ID
type contextKey string

const contextKeyUserID contextKey = "userID"

// --- USER ENDPOINTS ---

// HandleGetUserDailySummaries returns the authenticated user's daily summaries
// GET /api/user/daily-summaries?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD&mode=MODE
func (s *Server) HandleGetUserDailySummaries(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		s.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	// Parse query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	mode := r.URL.Query().Get("mode")

	// Default to last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	summaries, err := s.repo.GetDailyModeSummariesDateRange(r.Context(), userID, startDate, endDate, mode)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get summaries", err)
		return
	}

	// Calculate totals from ALL mode
	var totalPnL, totalVolume, totalFees float64
	var totalTrades int
	for _, summary := range summaries {
		if summary.Mode == "ALL" {
			totalPnL += summary.TotalPnL
			totalTrades += summary.TradeCount
			totalVolume += summary.TotalVolume
			totalFees += summary.TotalFees
		}
	}

	response := map[string]interface{}{
		"success":   true,
		"summaries": summaries,
		"totals": map[string]interface{}{
			"total_pnl":    totalPnL,
			"total_trades": totalTrades,
			"total_volume": totalVolume,
			"total_fees":   totalFees,
		},
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// HandleGetUserPerformanceSummary returns aggregated performance for a period
// GET /api/user/performance/summary?period=weekly|monthly|yearly&start_date=&end_date=
func (s *Server) HandleGetUserPerformanceSummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		s.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	period := r.URL.Query().Get("period")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Default to last 12 months
	endDate := time.Now()
	startDate := endDate.AddDate(-1, 0, 0)

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	var summaries []database.PeriodSummary
	var err error

	switch period {
	case "weekly":
		summaries, err = s.repo.GetWeeklySummary(r.Context(), userID, startDate, endDate)
	case "yearly":
		// For yearly, we use monthly and group in frontend
		summaries, err = s.repo.GetMonthlySummary(r.Context(), userID, startDate, endDate)
	default: // monthly
		summaries, err = s.repo.GetMonthlySummary(r.Context(), userID, startDate, endDate)
	}

	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get performance summary", err)
		return
	}

	response := map[string]interface{}{
		"success":   true,
		"period":    period,
		"summaries": summaries,
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// HandleGetUserModeComparison returns mode-by-mode performance comparison
// GET /api/user/performance/by-mode?start_date=&end_date=
func (s *Server) HandleGetUserModeComparison(w http.ResponseWriter, r *http.Request) {
	userID, ok := getUserIDFromContext(r.Context())
	if !ok {
		s.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Default to last 30 days
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -30)

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	comparison, err := s.repo.GetModeComparison(r.Context(), userID, startDate, endDate)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get mode comparison", err)
		return
	}

	response := map[string]interface{}{
		"success":    true,
		"comparison": comparison,
		"start_date": startDate.Format("2006-01-02"),
		"end_date":   endDate.Format("2006-01-02"),
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// --- ADMIN ENDPOINTS ---

// HandleGetAdminDailySummaries returns all users' daily summaries for admin dashboard
// GET /api/admin/daily-summaries/all
func (s *Server) HandleGetAdminDailySummaries(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	userID := r.URL.Query().Get("user_id")
	mode := r.URL.Query().Get("mode")
	status := r.URL.Query().Get("status")
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Default to current month
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := now

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			// FIX: Cap at max 500 to prevent resource exhaustion (Issue #13)
			if limit > 500 {
				limit = 500
			}
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	filter := database.AdminSummaryFilter{
		StartDate: startDate,
		EndDate:   endDate,
		UserID:    userID,
		Mode:      mode,
		Status:    status,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}

	result, err := s.repo.GetAdminDailySummaries(r.Context(), filter)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get admin summaries", err)
		return
	}

	response := map[string]interface{}{
		"success":   true,
		"summaries": result.Summaries,
		"totals": map[string]interface{}{
			"total_pnl":    result.TotalPnL,
			"total_trades": result.TotalTrades,
			"total_fees":   result.TotalFees,
			"avg_win_rate": result.AvgWinRate,
		},
		"pagination": map[string]interface{}{
			"page":        (offset / limit) + 1,
			"limit":       limit,
			"total_count": result.TotalCount,
			"total_pages": (result.TotalCount + limit - 1) / limit,
		},
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// HandleAdminExportCSV exports daily summaries to CSV
// GET /api/admin/daily-summaries/export
func (s *Server) HandleAdminExportCSV(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	// Default to current month
	now := time.Now()
	startDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := now

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err == nil {
			startDate = parsed
		}
	}
	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err == nil {
			endDate = parsed
		}
	}

	filter := database.AdminSummaryFilter{
		StartDate: startDate,
		EndDate:   endDate,
		Limit:     10000, // Export up to 10000 rows
	}

	result, err := s.repo.GetAdminDailySummaries(r.Context(), filter)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get data for export", err)
		return
	}

	// Set headers for CSV download
	filename := fmt.Sprintf("daily_summaries_%s_%s.csv", startDate.Format("20060102"), endDate.Format("20060102"))
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	writer := csv.NewWriter(w)

	// Write header
	header := []string{
		"User ID", "Date", "Mode", "Trades", "Wins", "Losses", "Win Rate",
		"Realized P&L", "Unrealized P&L", "Total P&L", "Fees", "Volume",
		"Status", "Timezone",
	}
	writer.Write(header)

	// Write data rows
	for _, summary := range result.Summaries {
		row := []string{
			summary.UserID,
			summary.SummaryDate.Format("2006-01-02"),
			summary.Mode,
			strconv.Itoa(summary.TradeCount),
			strconv.Itoa(summary.WinCount),
			strconv.Itoa(summary.LossCount),
			fmt.Sprintf("%.2f", summary.WinRate),
			fmt.Sprintf("%.2f", summary.RealizedPnL),
			fmt.Sprintf("%.2f", summary.UnrealizedPnL),
			fmt.Sprintf("%.2f", summary.TotalPnL),
			fmt.Sprintf("%.2f", summary.TotalFees),
			fmt.Sprintf("%.2f", summary.TotalVolume),
			summary.SettlementStatus,
			summary.UserTimezone,
		}
		writer.Write(row)
	}

	writer.Flush()
}

// HandleAdminSettlementRetry allows admin to retry a failed settlement
// POST /api/admin/settlements/retry/:user_id/:date
func (s *Server) HandleAdminSettlementRetry(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.PathValue("user_id")
	dateStr := r.PathValue("date")

	if userIDStr == "" || dateStr == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "user_id and date are required", nil)
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		s.writeErrorResponse(w, http.StatusBadRequest, "Invalid date format (use YYYY-MM-DD)", err)
		return
	}

	// Get user's timezone
	user, err := s.repo.GetUserByID(r.Context(), userIDStr)
	if err != nil {
		s.writeErrorResponse(w, http.StatusNotFound, "User not found", err)
		return
	}

	timezone := user.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Mark as retrying
	statusMsg := "Manual retry by admin"
	err = s.repo.UpdateSettlementStatus(r.Context(), userIDStr, date, "ALL", "retrying", &statusMsg)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update status", err)
		return
	}

	// Run settlement in background
	// FIX: Use context.Background() instead of r.Context() to prevent cancellation (Issue #8)
	go func() {
		if s.settlementService != nil {
			ctx := context.Background()
			_, err := s.settlementService.RunDailySettlement(ctx, userIDStr, date, timezone)
			if err != nil {
				errMsg := err.Error()
				s.repo.UpdateSettlementStatus(ctx, userIDStr, date, "ALL", "failed", &errMsg)
			}
		}
	}()

	response := map[string]interface{}{
		"success": true,
		"message": "Settlement retry initiated",
		"user_id": userIDStr,
		"date":    dateStr,
	}

	s.writeJSONResponse(w, http.StatusAccepted, response)
}

// HandleGetAdminSettlementStatus returns settlement status overview for monitoring
// GET /api/admin/settlements/status?status=all|failed|completed|retrying
func (s *Server) HandleGetAdminSettlementStatus(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	// Get recent summaries for status overview
	now := time.Now()
	startDate := now.AddDate(0, 0, -7) // Last 7 days

	filter := database.AdminSummaryFilter{
		StartDate: startDate,
		EndDate:   now,
		Limit:     1000,
	}

	if statusFilter != "" && statusFilter != "all" {
		filter.Status = statusFilter
	}

	result, err := s.repo.GetAdminDailySummaries(r.Context(), filter)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get settlement status", err)
		return
	}

	// Calculate summary stats
	completed := 0
	failed := 0
	retrying := 0
	for _, summary := range result.Summaries {
		switch summary.SettlementStatus {
		case "completed":
			completed++
		case "failed":
			failed++
		case "retrying":
			retrying++
		}
	}

	successRate := 0.0
	total := completed + failed + retrying
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	response := map[string]interface{}{
		"success":     true,
		"settlements": result.Summaries,
		"summary": map[string]interface{}{
			"total_settlements": total,
			"completed":         completed,
			"failed":            failed,
			"retrying":          retrying,
			"success_rate":      successRate,
		},
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// HandleGetReviewQueue returns data quality flagged settlements for admin review
// GET /api/admin/settlements/review-queue
func (s *Server) HandleGetReviewQueue(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			// FIX: Cap at max 500 to prevent resource exhaustion (Issue #13)
			if limit > 500 {
				limit = 500
			}
		}
	}

	offset := 0
	if offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	summaries, totalCount, err := s.repo.GetReviewQueue(r.Context(), limit, offset)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get review queue", err)
		return
	}

	response := map[string]interface{}{
		"success":   true,
		"summaries": summaries,
		"pagination": map[string]interface{}{
			"page":        (offset / limit) + 1,
			"limit":       limit,
			"total_count": totalCount,
			"total_pages": (totalCount + limit - 1) / limit,
		},
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// HandleApproveSummary approves a data quality flagged summary
// POST /api/admin/settlements/approve/:id
func (s *Server) HandleApproveSummary(w http.ResponseWriter, r *http.Request) {
	summaryID := r.PathValue("id")
	if summaryID == "" {
		s.writeErrorResponse(w, http.StatusBadRequest, "Summary ID is required", nil)
		return
	}

	adminID, ok := getUserIDFromContext(r.Context())
	if !ok {
		s.writeErrorResponse(w, http.StatusUnauthorized, "Authentication required", nil)
		return
	}

	err := s.repo.ApproveSummary(r.Context(), summaryID, adminID)
	if err != nil {
		s.writeErrorResponse(w, http.StatusInternalServerError, "Failed to approve summary", err)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Summary approved",
	}

	s.writeJSONResponse(w, http.StatusOK, response)
}

// --- HELPER METHODS ---

func (s *Server) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeErrorResponse(w http.ResponseWriter, statusCode int, message string, err error) {
	// FIX: Log error server-side but don't leak internal details to clients (Issue #2)
	if err != nil {
		// Log the actual error for debugging (server-side only)
		fmt.Printf("[SETTLEMENT-API] Error: %s - %v\n", message, err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := map[string]interface{}{
		"success": false,
		"error":   message,
		// Don't expose internal error details to clients for security
	}
	json.NewEncoder(w).Encode(response)
}

// getUserIDFromContext safely extracts user ID from context (Issue #3 fix)
func getUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(contextKeyUserID).(string)
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}

// --- GIN WRAPPER HANDLERS ---
// These handlers wrap the standard http.Handler functions for use with Gin router

// handleAdminDailySummariesGin wraps HandleGetAdminDailySummaries for Gin
func (s *Server) handleAdminDailySummariesGin(c *gin.Context) {
	// Create context with user ID
	userID := c.GetString("user_id")
	ctx := context.WithValue(c.Request.Context(), contextKeyUserID, userID)
	c.Request = c.Request.WithContext(ctx)

	s.HandleGetAdminDailySummaries(c.Writer, c.Request)
}

// handleAdminExportCSVGin wraps HandleAdminExportCSV for Gin
func (s *Server) handleAdminExportCSVGin(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := context.WithValue(c.Request.Context(), contextKeyUserID, userID)
	c.Request = c.Request.WithContext(ctx)

	s.HandleAdminExportCSV(c.Writer, c.Request)
}

// handleAdminSettlementStatusGin wraps HandleGetAdminSettlementStatus for Gin
func (s *Server) handleAdminSettlementStatusGin(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := context.WithValue(c.Request.Context(), contextKeyUserID, userID)
	c.Request = c.Request.WithContext(ctx)

	s.HandleGetAdminSettlementStatus(c.Writer, c.Request)
}

// handleAdminSettlementRetryGin wraps HandleAdminSettlementRetry for Gin
func (s *Server) handleAdminSettlementRetryGin(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := context.WithValue(c.Request.Context(), contextKeyUserID, userID)
	_ = ctx // Context with caller user ID (for audit logging if needed)

	// Extract params directly and pass to handler via context or query
	targetUserID := c.Param("user_id")
	dateStr := c.Param("date")

	if targetUserID == "" || dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and date are required"})
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format (use YYYY-MM-DD)"})
		return
	}

	// Get target user's timezone
	user, err := s.repo.GetUserByID(c.Request.Context(), targetUserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	timezone := user.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Mark as retrying
	statusMsg := "Manual retry by admin"
	err = s.repo.UpdateSettlementStatus(c.Request.Context(), targetUserID, date, "ALL", "retrying", &statusMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	// Run settlement in background
	go func() {
		if s.settlementService != nil {
			_, err := s.settlementService.RunDailySettlement(context.Background(), targetUserID, date, timezone)
			if err != nil {
				errMsg := err.Error()
				s.repo.UpdateSettlementStatus(context.Background(), targetUserID, date, "ALL", "failed", &errMsg)
			}
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"message": "Settlement retry initiated",
		"user_id": targetUserID,
		"date":    dateStr,
	})
}

// handleAdminReviewQueueGin wraps HandleGetReviewQueue for Gin
func (s *Server) handleAdminReviewQueueGin(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := context.WithValue(c.Request.Context(), contextKeyUserID, userID)
	c.Request = c.Request.WithContext(ctx)

	s.HandleGetReviewQueue(c.Writer, c.Request)
}

// handleAdminApproveSummaryGin wraps HandleApproveSummary for Gin
func (s *Server) handleAdminApproveSummaryGin(c *gin.Context) {
	userID := c.GetString("user_id")
	summaryID := c.Param("id")

	if summaryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Summary ID is required"})
		return
	}

	err := s.repo.ApproveSummary(c.Request.Context(), summaryID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Summary approved",
	})
}
