// Package api provides HTTP handlers for the Position Analytics Dashboard.
// Epic 10: Exit Optimization Engine
// Story 10.2: Position Analytics Dashboard
package api

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ==================== Response Types ====================

// EfficiencyTimelineResponse represents the efficiency timeline API response.
type EfficiencyTimelineResponse struct {
	DataPoints  []TimelineDataPoint `json:"data_points"`
	Aggregation string              `json:"aggregation"`
	TrendSlope  float64             `json:"trend_slope"`
	PeriodStart string              `json:"period_start"`
	PeriodEnd   string              `json:"period_end"`
}

// TimelineDataPoint represents a single point in the timeline.
type TimelineDataPoint struct {
	Timestamp     string  `json:"timestamp"`
	AvgEfficiency float64 `json:"avg_efficiency"`
	TradeCount    int     `json:"trade_count"`
	WinRate       float64 `json:"win_rate"`
	AvgPnL        float64 `json:"avg_pnl"`
}

// TradeDistributionResponse represents the trade distribution API response.
type TradeDistributionResponse struct {
	ByMode         []ModeDistribution         `json:"by_mode"`
	ByExitReason   []ExitReasonDistribution   `json:"by_exit_reason"`
	ByStrategy     []StrategyDistribution     `json:"by_strategy"`
	ByDecisionMode []DecisionModeDistribution `json:"by_decision_mode"`
	TotalTrades    int                        `json:"total_trades"`
	PeriodStart    string                     `json:"period_start"`
	PeriodEnd      string                     `json:"period_end"`
}

// ModeDistribution represents trade distribution by mode.
type ModeDistribution struct {
	Mode       string  `json:"mode"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// ExitReasonDistribution represents trade distribution by exit reason.
type ExitReasonDistribution struct {
	Reason     string  `json:"reason"`
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// StrategyDistribution represents trade distribution by strategy.
type StrategyDistribution struct {
	Strategy   string  `json:"strategy"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// DecisionModeDistribution represents win/loss by decision mode.
type DecisionModeDistribution struct {
	Mode        string  `json:"mode"`
	Label       string  `json:"label"`
	Wins        int     `json:"wins"`
	Losses      int     `json:"losses"`
	WinRate     float64 `json:"win_rate"`
	TotalTrades int     `json:"total_trades"`
}

// PositionAnalyticsSummaryResponse represents the summary API response.
type PositionAnalyticsSummaryResponse struct {
	TotalTrades       int                           `json:"total_trades"`
	OverallWinRate    float64                       `json:"overall_win_rate"`
	OverallEfficiency float64                       `json:"overall_efficiency"`
	TotalPnL          float64                       `json:"total_pnl"`
	AvgPnL            float64                       `json:"avg_pnl"`
	BestMode          string                        `json:"best_mode"`
	BestStrategy      string                        `json:"best_strategy"`
	ModeMetrics       []ModePerformanceResponse     `json:"mode_metrics"`
	StrategyMetrics   []StrategyPerformanceResponse `json:"strategy_metrics"`
	RegimeMetrics     []RegimePerformanceResponse   `json:"regime_metrics"`
	PeriodStart       string                        `json:"period_start"`
	PeriodEnd         string                        `json:"period_end"`
}

// ModePerformanceResponse represents mode performance metrics.
type ModePerformanceResponse struct {
	Mode           string  `json:"mode"`
	Label          string  `json:"label"`
	TotalTrades    int     `json:"total_trades"`
	WinCount       int     `json:"win_count"`
	LossCount      int     `json:"loss_count"`
	WinRate        float64 `json:"win_rate"`
	AvgEfficiency  float64 `json:"avg_efficiency"`
	AvgPeakProfit  float64 `json:"avg_peak_profit"`
	AvgExitProfit  float64 `json:"avg_exit_profit"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnL         float64 `json:"avg_pnl"`
	ProfitFactor   float64 `json:"profit_factor"`
	BreakevenRate  float64 `json:"breakeven_rate"`
	TP1HitRate     float64 `json:"tp1_hit_rate"`
	AvgHoldMinutes float64 `json:"avg_hold_minutes"`
}

// StrategyPerformanceResponse represents strategy performance metrics.
type StrategyPerformanceResponse struct {
	Strategy       string  `json:"strategy"`
	TotalTrades    int     `json:"total_trades"`
	WinCount       int     `json:"win_count"`
	LossCount      int     `json:"loss_count"`
	WinRate        float64 `json:"win_rate"`
	AvgEfficiency  float64 `json:"avg_efficiency"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnL         float64 `json:"avg_pnl"`
	AvgHoldMinutes float64 `json:"avg_hold_minutes"`
	BreakevenRate  float64 `json:"breakeven_rate"`
	TP1HitRate     float64 `json:"tp1_hit_rate"`
}

// RegimePerformanceResponse represents regime performance metrics.
type RegimePerformanceResponse struct {
	Regime        string  `json:"regime"`
	Label         string  `json:"label"`
	TotalTrades   int     `json:"total_trades"`
	WinCount      int     `json:"win_count"`
	LossCount     int     `json:"loss_count"`
	WinRate       float64 `json:"win_rate"`
	AvgEfficiency float64 `json:"avg_efficiency"`
	TotalPnL      float64 `json:"total_pnl"`
	AvgPnL        float64 `json:"avg_pnl"`
}

// ==================== Helper Functions ====================

// parseTimeRange parses the range query parameter and returns start/end times.
func parseTimeRange(rangeParam string) (time.Time, time.Time, string) {
	now := time.Now()
	endTime := now
	var startTime time.Time
	aggregation := "daily"

	switch rangeParam {
	case "7d":
		startTime = now.AddDate(0, 0, -7)
		aggregation = "hourly"
	case "30d":
		startTime = now.AddDate(0, 0, -30)
	case "90d":
		startTime = now.AddDate(0, 0, -90)
	case "all":
		startTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		startTime = now.AddDate(0, 0, -7)
		aggregation = "hourly"
	}

	return startTime, endTime, aggregation
}

// getModeLabel returns human-readable label for a trading mode.
func getModeLabel(mode string) string {
	labels := map[string]string{
		"ultra_fast": "Ultra Fast",
		"scalp":      "Scalp",
		"swing":      "Swing",
		"position":   "Position",
	}
	if label, ok := labels[mode]; ok {
		return label
	}
	return mode
}

// getExitReasonLabel returns human-readable label for an exit reason.
func getExitReasonLabel(reason string) string {
	labels := map[string]string{
		"TP_HIT":          "Take Profit",
		"SL_HIT":          "Stop Loss",
		"TRAILING_STOP":   "Trailing Stop",
		"MANUAL":          "Manual",
		"EFFICIENCY_EXIT": "Efficiency Exit",
		"LIQUIDATION":     "Liquidation",
		"UNKNOWN":         "Unknown",
	}
	if label, ok := labels[reason]; ok {
		return label
	}
	return reason
}

// getDecisionModeLabel returns human-readable label for a decision mode.
func getDecisionModeLabel(mode string) string {
	labels := map[string]string{
		"classic":    "Classic",
		"new_engine": "New Engine",
	}
	if label, ok := labels[mode]; ok {
		return label
	}
	return mode
}

// getRegimeLabel returns human-readable label for a market regime.
func getRegimeLabel(regime string) string {
	labels := map[string]string{
		"TRENDING":      "Trending",
		"RANGING":       "Ranging",
		"VOLATILE":      "Volatile",
		"CONSOLIDATING": "Consolidating",
		"UNKNOWN":       "Unknown",
	}
	if label, ok := labels[regime]; ok {
		return label
	}
	return regime
}

// calculateTrendSlope calculates a simple trend slope from timeline data.
func calculateTrendSlope(dataPoints []TimelineDataPoint) float64 {
	if len(dataPoints) < 2 {
		return 0
	}

	n := float64(len(dataPoints))
	var sumX, sumY, sumXY, sumX2 float64

	for i, point := range dataPoints {
		x := float64(i)
		y := point.AvgEfficiency
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Linear regression slope
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denominator
}

// ==================== Handler Functions ====================

// handleGetPositionAnalyticsSummary returns aggregated analytics summary.
// GET /api/futures/position-analytics/summary
func (s *Server) handleGetPositionAnalyticsSummary(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	rangeParam := c.DefaultQuery("range", "7d")
	startTime, endTime, _ := parseTimeRange(rangeParam)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get summary
	summary, err := s.repo.GetDB().GetPositionAnalyticsSummary(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get analytics summary: %v", err),
		})
		return
	}

	// Get mode metrics
	modeMetrics, err := s.repo.GetDB().GetModePerformanceMetrics(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get mode metrics: %v", err),
		})
		return
	}

	// Get strategy metrics
	strategyMetrics, err := s.repo.GetDB().GetStrategyPerformanceMetrics(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get strategy metrics: %v", err),
		})
		return
	}

	// Get regime metrics
	regimeMetrics, err := s.repo.GetDB().GetRegimePerformanceMetrics(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get regime metrics: %v", err),
		})
		return
	}

	// Build response
	response := PositionAnalyticsSummaryResponse{
		TotalTrades:       summary.TotalTrades,
		OverallWinRate:    summary.OverallWinRate,
		OverallEfficiency: summary.OverallEfficiency,
		TotalPnL:          summary.TotalPnL,
		AvgPnL:            summary.AvgPnL,
		BestMode:          summary.BestMode,
		BestStrategy:      summary.BestStrategy,
		PeriodStart:       startTime.Format(time.RFC3339),
		PeriodEnd:         endTime.Format(time.RFC3339),
	}

	// Convert mode metrics
	for _, m := range modeMetrics {
		response.ModeMetrics = append(response.ModeMetrics, ModePerformanceResponse{
			Mode:           m.Mode,
			Label:          getModeLabel(m.Mode),
			TotalTrades:    m.TotalTrades,
			WinCount:       m.WinCount,
			LossCount:      m.LossCount,
			WinRate:        m.WinRate,
			AvgEfficiency:  m.AvgEfficiency,
			AvgPeakProfit:  m.AvgPeakProfit,
			AvgExitProfit:  m.AvgExitProfit,
			TotalPnL:       m.TotalPnL,
			AvgPnL:         m.AvgPnL,
			ProfitFactor:   m.ProfitFactor,
			BreakevenRate:  m.BreakevenRate,
			TP1HitRate:     m.TP1HitRate,
			AvgHoldMinutes: m.AvgHoldMinutes,
		})
	}

	// Convert strategy metrics
	for _, s := range strategyMetrics {
		response.StrategyMetrics = append(response.StrategyMetrics, StrategyPerformanceResponse{
			Strategy:       s.Strategy,
			TotalTrades:    s.TotalTrades,
			WinCount:       s.WinCount,
			LossCount:      s.LossCount,
			WinRate:        s.WinRate,
			AvgEfficiency:  s.AvgEfficiency,
			TotalPnL:       s.TotalPnL,
			AvgPnL:         s.AvgPnL,
			AvgHoldMinutes: s.AvgHoldMinutes,
			BreakevenRate:  s.BreakevenRate,
			TP1HitRate:     s.TP1HitRate,
		})
	}

	// Convert regime metrics
	for _, r := range regimeMetrics {
		response.RegimeMetrics = append(response.RegimeMetrics, RegimePerformanceResponse{
			Regime:        r.Regime,
			Label:         getRegimeLabel(r.Regime),
			TotalTrades:   r.TotalTrades,
			WinCount:      r.WinCount,
			LossCount:     r.LossCount,
			WinRate:       r.WinRate,
			AvgEfficiency: r.AvgEfficiency,
			TotalPnL:      r.TotalPnL,
			AvgPnL:        r.AvgPnL,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// handleGetEfficiencyTimeline returns efficiency metrics over time.
// GET /api/futures/position-analytics/efficiency-timeline
func (s *Server) handleGetEfficiencyTimeline(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	rangeParam := c.DefaultQuery("range", "7d")
	startTime, endTime, aggregation := parseTimeRange(rangeParam)
	hourly := aggregation == "hourly"

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get timeline data
	timeline, err := s.repo.GetDB().GetEfficiencyTimeline(ctx, userID, startTime, endTime, hourly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get efficiency timeline: %v", err),
		})
		return
	}

	// Convert to response format - initialize as empty slice to avoid null in JSON
	dataPoints := make([]TimelineDataPoint, 0)
	for _, t := range timeline {
		dataPoints = append(dataPoints, TimelineDataPoint{
			Timestamp:     t.Timestamp.Format(time.RFC3339),
			AvgEfficiency: t.AvgEfficiency,
			TradeCount:    t.TradeCount,
			WinRate:       t.WinRate,
			AvgPnL:        t.AvgPnL,
		})
	}

	response := EfficiencyTimelineResponse{
		DataPoints:  dataPoints,
		Aggregation: aggregation,
		TrendSlope:  calculateTrendSlope(dataPoints),
		PeriodStart: startTime.Format(time.RFC3339),
		PeriodEnd:   endTime.Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// handleGetTradeDistribution returns trade distribution by various categories.
// GET /api/futures/position-analytics/distribution
func (s *Server) handleGetTradeDistribution(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	rangeParam := c.DefaultQuery("range", "7d")
	startTime, endTime, _ := parseTimeRange(rangeParam)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Get mode distribution
	modeDistrib, totalTrades, err := s.repo.GetDB().GetTradeDistributionByMode(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get mode distribution: %v", err),
		})
		return
	}

	// Get exit reason distribution
	exitReasonDistrib, err := s.repo.GetDB().GetTradeDistributionByExitReason(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get exit reason distribution: %v", err),
		})
		return
	}

	// Get strategy distribution
	strategyDistrib, err := s.repo.GetDB().GetTradeDistributionByStrategy(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get strategy distribution: %v", err),
		})
		return
	}

	// Get decision mode win/loss
	decisionModeDistrib, err := s.repo.GetDB().GetWinLossByDecisionMode(ctx, userID, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get decision mode distribution: %v", err),
		})
		return
	}

	// Build response - initialize slices to avoid null in JSON
	response := TradeDistributionResponse{
		TotalTrades:   totalTrades,
		PeriodStart:   startTime.Format(time.RFC3339),
		PeriodEnd:     endTime.Format(time.RFC3339),
		ByMode:        make([]ModeDistribution, 0),
		ByExitReason:  make([]ExitReasonDistribution, 0),
		ByStrategy:    make([]StrategyDistribution, 0),
		ByDecisionMode: make([]DecisionModeDistribution, 0),
	}

	for _, m := range modeDistrib {
		response.ByMode = append(response.ByMode, ModeDistribution{
			Mode:       m.Mode,
			Label:      getModeLabel(m.Mode),
			Count:      m.Count,
			Percentage: m.Percentage,
		})
	}

	for _, e := range exitReasonDistrib {
		response.ByExitReason = append(response.ByExitReason, ExitReasonDistribution{
			Reason:     e.ExitReason,
			Label:      getExitReasonLabel(e.ExitReason),
			Count:      e.Count,
			Percentage: e.Percentage,
		})
	}

	for _, s := range strategyDistrib {
		response.ByStrategy = append(response.ByStrategy, StrategyDistribution{
			Strategy:   s.Strategy,
			Count:      s.Count,
			Percentage: s.Percentage,
		})
	}

	for _, d := range decisionModeDistrib {
		response.ByDecisionMode = append(response.ByDecisionMode, DecisionModeDistribution{
			Mode:        d.DecisionMode,
			Label:       getDecisionModeLabel(d.DecisionMode),
			Wins:        d.Wins,
			Losses:      d.Losses,
			WinRate:     d.WinRate,
			TotalTrades: d.TotalTrades,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// handleExportPositionAnalytics exports analytics data as CSV or JSON.
// GET /api/futures/position-analytics/export
func (s *Server) handleExportPositionAnalytics(c *gin.Context) {
	userID, ok := s.getUserIDRequired(c)
	if !ok {
		return
	}

	rangeParam := c.DefaultQuery("range", "7d")
	format := c.DefaultQuery("format", "csv")
	modeFilter := c.Query("mode")

	// Validate mode filter if provided
	validModes := map[string]bool{
		"ultra_fast": true,
		"scalp":      true,
		"swing":      true,
		"position":   true,
	}
	if modeFilter != "" && !validModes[modeFilter] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Invalid mode filter: %s. Valid modes: ultra_fast, scalp, swing, position", modeFilter),
		})
		return
	}

	startTime, endTime, _ := parseTimeRange(rangeParam)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Get all metrics for export
	metrics, err := s.repo.GetDB().GetAllEfficiencyMetricsForExport(ctx, userID, startTime, endTime, modeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Failed to get metrics for export: %v", err),
		})
		return
	}

	// Generate filename
	filename := fmt.Sprintf("position-analytics-%s-%s.%s",
		startTime.Format("2006-01-02"),
		endTime.Format("2006-01-02"),
		format,
	)

	if format == "json" {
		// JSON export
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Header("Content-Type", "application/json")

		exportData := gin.H{
			"export_date":  time.Now().Format(time.RFC3339),
			"period_start": startTime.Format(time.RFC3339),
			"period_end":   endTime.Format(time.RFC3339),
			"total_trades": len(metrics),
			"mode_filter":  modeFilter,
			"trades":       metrics,
		}

		jsonBytes, err := json.MarshalIndent(exportData, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to generate JSON export",
			})
			return
		}

		c.Data(http.StatusOK, "application/json", jsonBytes)
		return
	}

	// CSV export (default)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "text/csv")

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"ID", "Symbol", "Mode", "Entry Price", "Exit Price",
		"Entry Time", "Exit Time", "Quantity",
		"Peak Profit %", "Exit Profit %", "Exit Efficiency %",
		"Exit Reason", "Decision Mode", "Entry Strategy", "Entry Regime",
		"Breakeven Achieved", "TP1 Hit", "Trade Category",
	}
	if err := writer.Write(header); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to write CSV header",
		})
		return
	}

	// Write data rows
	for _, m := range metrics {
		row := []string{
			fmt.Sprintf("%d", m.ID),
			m.Symbol,
			m.Mode,
			fmt.Sprintf("%.8f", m.EntryPrice),
			fmt.Sprintf("%.8f", m.ExitPrice),
			m.EntryTime.Format(time.RFC3339),
			m.ExitTime.Format(time.RFC3339),
			fmt.Sprintf("%.8f", m.ExitQty),
			fmt.Sprintf("%.4f", m.PeakProfitPct),
			fmt.Sprintf("%.4f", m.ExitProfitPct),
			fmt.Sprintf("%.2f", m.ExitEfficiency),
			m.ExitReason,
			m.DecisionMode,
			safeString(m.EntryStrategy),
			safeString(m.EntryRegime),
			fmt.Sprintf("%t", m.BreakevenAchieved),
			fmt.Sprintf("%t", m.TP1Hit),
			fmt.Sprintf("%d", m.TradeCategory),
		}
		if err := writer.Write(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to write CSV row",
			})
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Failed to flush CSV data",
		})
		return
	}
	c.Data(http.StatusOK, "text/csv", buf.Bytes())
}

// safeString returns the string value or empty string if nil.
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
