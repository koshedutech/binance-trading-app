// Package api provides HTTP handlers for the Binance Trading Bot.
// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
package api

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"binance-trading-bot/internal/decision"

	"github.com/gin-gonic/gin"
)

// DashboardStats is the aggregate response for the dashboard endpoint.
type DashboardStats struct {
	ScoreDistribution     *ScoreDistributionData     `json:"scoreDistribution"`
	CalibrationSummary    *CalibrationSummaryData    `json:"calibrationSummary"`
	StrategyPerformances  []StrategyPerformanceData  `json:"strategyPerformances"`
	RegimeDistribution    *RegimeDistributionData    `json:"regimeDistribution"`
	BlockingStats         *BlockingStatsData         `json:"blockingStats"`
	TimeRange             string                     `json:"timeRange"`
	GeneratedAt           string                     `json:"generatedAt"`
}

// ScoreDistributionData represents score bucket analysis.
type ScoreDistributionData struct {
	Buckets         []ScoreBucketData `json:"buckets"`
	TotalTrades     int               `json:"totalTrades"`
	AverageScore    float64           `json:"averageScore"`
	MedianScore     float64           `json:"medianScore"`
	HighScoreTrades int               `json:"highScoreTrades"`
	LowScoreTrades  int               `json:"lowScoreTrades"`
}

// ScoreBucketData represents a single score bucket.
type ScoreBucketData struct {
	Bucket     string  `json:"bucket"`
	MinScore   int     `json:"minScore"`
	MaxScore   int     `json:"maxScore"`
	TradeCount int     `json:"tradeCount"`
	WinCount   int     `json:"winCount"`
	LossCount  int     `json:"lossCount"`
	WinRate    float64 `json:"winRate"`
	AvgPnL     float64 `json:"avgPnL"`
	TotalPnL   float64 `json:"totalPnL"`
}

// CalibrationSummaryData represents calibration accuracy metrics.
type CalibrationSummaryData struct {
	Strategy                 string                    `json:"strategy"`
	OverallExpected          float64                   `json:"overallExpected"`
	OverallActual            float64                   `json:"overallActual"`
	OverallDelta             float64                   `json:"overallDelta"`
	BucketAccuracies         []CalibrationAccuracyData `json:"bucketAccuracies"`
	TotalCalibrationTrades   int                       `json:"totalCalibrationTrades"`
	LastUpdated              string                    `json:"lastUpdated"`
	ConfidenceLevel          string                    `json:"confidenceLevel"`
	IsReliable               bool                      `json:"isReliable"`
	MinimumTradesRequired    int                       `json:"minimumTradesRequired"`
}

// CalibrationAccuracyData represents accuracy for a single bucket.
type CalibrationAccuracyData struct {
	ScoreBucket     string  `json:"scoreBucket"`
	ExpectedWinRate float64 `json:"expectedWinRate"`
	ActualWinRate   float64 `json:"actualWinRate"`
	Delta           float64 `json:"delta"`
	TotalTrades     int     `json:"totalTrades"`
	Confidence      string  `json:"confidence"`
	IsAccurate      bool    `json:"isAccurate"`
}

// StrategyPerformanceData matches the frontend interface.
type StrategyPerformanceData struct {
	StrategyName      string    `json:"strategyName"`
	TotalTrades       int       `json:"totalTrades"`
	WinningTrades     int       `json:"winningTrades"`
	LosingTrades      int       `json:"losingTrades"`
	WinRate           float64   `json:"winRate"`
	TotalPnL          float64   `json:"totalPnL"`
	AvgPnL            float64   `json:"avgPnL"`
	AvgWin            float64   `json:"avgWin"`
	AvgLoss           float64   `json:"avgLoss"`
	LargestWin        float64   `json:"largestWin"`
	LargestLoss       float64   `json:"largestLoss"`
	ProfitFactor      float64   `json:"profitFactor"`
	MaxDrawdown       float64   `json:"maxDrawdown"`
	SharpeRatio       float64   `json:"sharpeRatio"`
	Expectancy        float64   `json:"expectancy"`
	RiskReward        float64   `json:"riskReward"`
	ConsecutiveWins   int       `json:"consecutiveWins"`
	ConsecutiveLosses int       `json:"consecutiveLosses"`
	LastTradeTime     *string   `json:"lastTradeTime,omitempty"`
	Status            string    `json:"status"`
	Trend             string    `json:"trend"`
	RecentPnL         []float64 `json:"recentPnL"`
}

// RegimeDistributionData represents regime classification statistics.
type RegimeDistributionData struct {
	Stats                []RegimeStatsData `json:"stats"`
	TotalClassifications int               `json:"totalClassifications"`
	MostCommonRegime     string            `json:"mostCommonRegime"`
	CurrentRegime        *string           `json:"currentRegime,omitempty"`
	PeriodStart          string            `json:"periodStart"`
	PeriodEnd            string            `json:"periodEnd"`
}

// RegimeStatsData represents stats for a single regime.
type RegimeStatsData struct {
	Regime          string  `json:"regime"`
	Count           int     `json:"count"`
	Percentage      float64 `json:"percentage"`
	AvgDuration     float64 `json:"avgDuration"`
	TradesInRegime  int     `json:"tradesInRegime"`
	WinRateInRegime float64 `json:"winRateInRegime"`
	AvgPnLInRegime  float64 `json:"avgPnLInRegime"`
}

// BlockingStatsData represents blocking reason statistics.
type BlockingStatsData struct {
	TotalBlocks       int                         `json:"totalBlocks"`
	HardBlockCount    int                         `json:"hardBlockCount"`
	SoftBlockCount    int                         `json:"softBlockCount"`
	WarningCount      int                         `json:"warningCount"`
	ByReason          []BlockingReasonFrequency   `json:"byReason"`
	TopReasons        []BlockingReasonFrequency   `json:"topReasons"`
	SymbolsAffected   int                         `json:"symbolsAffected"`
	MostFrequentBlock string                      `json:"mostFrequentBlock"`
	PeriodStart       string                      `json:"periodStart"`
	PeriodEnd         string                      `json:"periodEnd"`
}

// BlockingReasonFrequency represents frequency data for a blocking reason.
type BlockingReasonFrequency struct {
	Code            string  `json:"code"`
	Description     string  `json:"description"`
	Category        string  `json:"category"`
	Count           int     `json:"count"`
	Percentage      float64 `json:"percentage"`
	SymbolsAffected int     `json:"symbolsAffected"`
}

// handleGetDecisionDashboardStats returns aggregated dashboard statistics.
// GET /api/futures/decision/dashboard/stats
func (s *Server) handleGetDecisionDashboardStats(c *gin.Context) {
	// Parse time range
	timeRange := c.DefaultQuery("range", "7d")

	// Calculate date range
	now := time.Now()
	var startTime time.Time

	switch timeRange {
	case "7d":
		startTime = now.AddDate(0, 0, -7)
	case "30d":
		startTime = now.AddDate(0, 0, -30)
	case "90d":
		startTime = now.AddDate(0, 0, -90)
	default:
		startTime = time.Time{} // All time
	}

	// Get user ID (if available for per-user stats)
	userIDStr := c.GetString("user_id")
	userID := 0
	if userIDStr != "" {
		userID, _ = strconv.Atoi(userIDStr)
	}

	// Build response
	stats := &DashboardStats{
		TimeRange:   timeRange,
		GeneratedAt: now.Format(time.RFC3339),
	}

	// 1. Get Strategy Performance from trade history
	stats.StrategyPerformances = s.getStrategyPerformancesForDashboard(c, startTime)

	// 2. Get Score Distribution from calibration data
	stats.ScoreDistribution = s.getScoreDistributionFromCalibration(c, userID)

	// 3. Get Calibration Summary
	stats.CalibrationSummary = s.getCalibrationSummaryForDashboard(c, userID)

	// 4. Get Regime Distribution from regime classifier
	stats.RegimeDistribution = s.getRegimeDistributionForDashboard(startTime, now)

	// 5. Get Blocking Stats from blocking tracker
	stats.BlockingStats = s.getBlockingStatsForDashboard(userID, startTime, now)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// getStrategyPerformancesForDashboard aggregates strategy performance data.
func (s *Server) getStrategyPerformancesForDashboard(c *gin.Context, startTime time.Time) []StrategyPerformanceData {
	ctx := c.Request.Context()

	// Get trades from database
	tradesPtr, err := s.repo.GetTradeHistory(ctx, 1000, 0)
	if err != nil {
		return []StrategyPerformanceData{}
	}

	// Convert and filter by time
	strategyMap := make(map[string]*StrategyPerformanceData)

	for _, trade := range tradesPtr {
		// Filter by time
		if !startTime.IsZero() && trade.EntryTime.Before(startTime) {
			continue
		}

		strategyName := "Manual"
		if trade.StrategyName != nil && *trade.StrategyName != "" {
			strategyName = *trade.StrategyName
		}

		if _, exists := strategyMap[strategyName]; !exists {
			strategyMap[strategyName] = &StrategyPerformanceData{
				StrategyName: strategyName,
				Status:       "active",
				Trend:        "neutral",
				RecentPnL:    make([]float64, 0),
			}
		}

		perf := strategyMap[strategyName]
		perf.TotalTrades++

		pnl := float64(0)
		if trade.PnL != nil {
			pnl = *trade.PnL
		}

		perf.TotalPnL += pnl
		perf.RecentPnL = append(perf.RecentPnL, pnl)

		if pnl > 0 {
			perf.WinningTrades++
			if pnl > perf.LargestWin {
				perf.LargestWin = pnl
			}
		} else if pnl < 0 {
			perf.LosingTrades++
			if pnl < perf.LargestLoss {
				perf.LargestLoss = pnl
			}
		}

		// Track last trade time
		if trade.ExitTime != nil {
			timeStr := trade.ExitTime.Format(time.RFC3339)
			perf.LastTradeTime = &timeStr
		}
	}

	// Calculate derived metrics
	performances := make([]StrategyPerformanceData, 0)
	for _, perf := range strategyMap {
		if perf.TotalTrades > 0 {
			perf.WinRate = float64(perf.WinningTrades) / float64(perf.TotalTrades) * 100
			perf.AvgPnL = perf.TotalPnL / float64(perf.TotalTrades)
		}

		// Calculate average win and loss
		if perf.WinningTrades > 0 {
			totalWins := float64(0)
			for _, pnl := range perf.RecentPnL {
				if pnl > 0 {
					totalWins += pnl
				}
			}
			perf.AvgWin = totalWins / float64(perf.WinningTrades)
		}

		if perf.LosingTrades > 0 {
			totalLosses := float64(0)
			for _, pnl := range perf.RecentPnL {
				if pnl < 0 {
					totalLosses += -pnl
				}
			}
			perf.AvgLoss = totalLosses / float64(perf.LosingTrades)
		}

		// Profit factor
		if perf.AvgLoss > 0 && perf.LosingTrades > 0 {
			totalWins := perf.AvgWin * float64(perf.WinningTrades)
			totalLosses := perf.AvgLoss * float64(perf.LosingTrades)
			if totalLosses > 0 {
				perf.ProfitFactor = totalWins / totalLosses
			}
		}

		// Expectancy
		if perf.TotalTrades > 0 {
			winProb := float64(perf.WinningTrades) / float64(perf.TotalTrades)
			lossProb := float64(perf.LosingTrades) / float64(perf.TotalTrades)
			perf.Expectancy = (winProb * perf.AvgWin) - (lossProb * perf.AvgLoss)
		}

		// Risk/Reward ratio
		if perf.AvgLoss > 0 {
			perf.RiskReward = perf.AvgWin / perf.AvgLoss
		}

		// Calculate max drawdown
		perf.MaxDrawdown = calculateMaxDrawdown(perf.RecentPnL)

		// Determine trend from recent trades
		if len(perf.RecentPnL) >= 5 {
			recent := perf.RecentPnL[len(perf.RecentPnL)-5:]
			sum := float64(0)
			for _, p := range recent {
				sum += p
			}
			if sum > 0 {
				perf.Trend = "up"
			} else if sum < 0 {
				perf.Trend = "down"
			}
		}

		// Limit recent PnL to last 10
		if len(perf.RecentPnL) > 10 {
			perf.RecentPnL = perf.RecentPnL[len(perf.RecentPnL)-10:]
		}

		// Calculate consecutive wins/losses
		perf.ConsecutiveWins, perf.ConsecutiveLosses = calculateConsecutive(perf.RecentPnL)

		performances = append(performances, *perf)
	}

	// Sort by total PnL descending
	sort.Slice(performances, func(i, j int) bool {
		return performances[i].TotalPnL > performances[j].TotalPnL
	})

	return performances
}

// getScoreDistributionFromCalibration builds score distribution from calibration data.
func (s *Server) getScoreDistributionFromCalibration(c *gin.Context, userID int) *ScoreDistributionData {
	// Try to get calibration service
	calibSvc := s.getCalibrationService()
	if calibSvc == nil {
		// Return mock data if service not available
		return &ScoreDistributionData{
			Buckets: []ScoreBucketData{
				{Bucket: "0-25", MinScore: 0, MaxScore: 25, TradeCount: 0, WinRate: 0},
				{Bucket: "26-50", MinScore: 26, MaxScore: 50, TradeCount: 0, WinRate: 0},
				{Bucket: "51-75", MinScore: 51, MaxScore: 75, TradeCount: 0, WinRate: 0},
				{Bucket: "76-100", MinScore: 76, MaxScore: 100, TradeCount: 0, WinRate: 0},
			},
			TotalTrades:  0,
			AverageScore: 50,
			MedianScore:  50,
		}
	}

	ctx := c.Request.Context()

	// Get calibration data for default strategy
	calibData, err := calibSvc.GetCalibrationData(ctx, userID, "trend_following", "")
	if err != nil || calibData == nil {
		return &ScoreDistributionData{
			Buckets: []ScoreBucketData{
				{Bucket: "0-25", MinScore: 0, MaxScore: 25, TradeCount: 0, WinRate: 0},
				{Bucket: "26-50", MinScore: 26, MaxScore: 50, TradeCount: 0, WinRate: 0},
				{Bucket: "51-75", MinScore: 51, MaxScore: 75, TradeCount: 0, WinRate: 0},
				{Bucket: "76-100", MinScore: 76, MaxScore: 100, TradeCount: 0, WinRate: 0},
			},
			TotalTrades:  0,
			AverageScore: 50,
			MedianScore:  50,
		}
	}

	buckets := []ScoreBucketData{
		buildScoreBucket("0-24", 0, 24, calibData.Buckets[decision.BucketLow]),
		buildScoreBucket("25-49", 25, 49, calibData.Buckets[decision.BucketMediumLow]),
		buildScoreBucket("50-74", 50, 74, calibData.Buckets[decision.BucketMediumHigh]),
		buildScoreBucket("75-100", 75, 100, calibData.Buckets[decision.BucketHigh]),
	}

	totalTrades := 0
	highScoreTrades := 0
	lowScoreTrades := 0

	for _, b := range buckets {
		totalTrades += b.TradeCount
		if b.MinScore >= 76 {
			highScoreTrades += b.TradeCount
		}
		if b.MaxScore <= 50 {
			lowScoreTrades += b.TradeCount
		}
	}

	return &ScoreDistributionData{
		Buckets:         buckets,
		TotalTrades:     totalTrades,
		AverageScore:    calculateWeightedAverageScore(buckets),
		MedianScore:     50, // Simplified
		HighScoreTrades: highScoreTrades,
		LowScoreTrades:  lowScoreTrades,
	}
}

// buildScoreBucket creates a ScoreBucketData from calibration bucket.
func buildScoreBucket(label string, minScore, maxScore int, bucket *decision.ScoreBucket) ScoreBucketData {
	if bucket == nil {
		return ScoreBucketData{
			Bucket:   label,
			MinScore: minScore,
			MaxScore: maxScore,
		}
	}

	winCount := int(bucket.WinningTrades)
	lossCount := int(bucket.TotalTrades) - winCount

	return ScoreBucketData{
		Bucket:     label,
		MinScore:   minScore,
		MaxScore:   maxScore,
		TradeCount: int(bucket.TotalTrades),
		WinCount:   winCount,
		LossCount:  lossCount,
		WinRate:    bucket.WinRate * 100, // Convert to percentage
		AvgPnL:     bucket.AvgPnLPercent,
		TotalPnL:   bucket.AvgPnLPercent * float64(bucket.TotalTrades),
	}
}

// calculateWeightedAverageScore calculates weighted average from buckets.
func calculateWeightedAverageScore(buckets []ScoreBucketData) float64 {
	totalTrades := 0
	weightedSum := float64(0)

	for _, b := range buckets {
		midpoint := (b.MinScore + b.MaxScore) / 2
		weightedSum += float64(midpoint) * float64(b.TradeCount)
		totalTrades += b.TradeCount
	}

	if totalTrades == 0 {
		return 50
	}

	return weightedSum / float64(totalTrades)
}

// getCalibrationSummaryForDashboard builds calibration summary.
func (s *Server) getCalibrationSummaryForDashboard(c *gin.Context, userID int) *CalibrationSummaryData {
	calibSvc := s.getCalibrationService()
	if calibSvc == nil {
		return nil
	}

	ctx := c.Request.Context()
	strategy := "trend_following"

	// Get calibration data first
	calibData, err := calibSvc.GetCalibrationData(ctx, userID, strategy, "")
	if err != nil || calibData == nil {
		return nil
	}

	// Get confidence level from lifecycle service
	lifecycleSvc := s.getCalibrationLifecycleService()
	var confidenceLevel string
	var isReliable bool
	if lifecycleSvc != nil {
		confidence, err := lifecycleSvc.GetCalibrationConfidence(ctx, userID, strategy, "")
		if err == nil && confidence != nil {
			confidenceLevel = string(confidence.Level)
			isReliable = confidence.IsReliable
		}
	}
	if confidenceLevel == "" {
		confidenceLevel = "insufficient"
		isReliable = false
	}

	// Build bucket accuracies
	bucketAccuracies := []CalibrationAccuracyData{
		buildBucketAccuracy("0-24", 12.0, calibData.Buckets[decision.BucketLow]),       // Expected 12%
		buildBucketAccuracy("25-49", 37.0, calibData.Buckets[decision.BucketMediumLow]),  // Expected 37%
		buildBucketAccuracy("50-74", 62.0, calibData.Buckets[decision.BucketMediumHigh]), // Expected 62%
		buildBucketAccuracy("75-100", 87.0, calibData.Buckets[decision.BucketHigh]),      // Expected 87%
	}

	// Calculate overall metrics
	totalTrades := 0
	totalActual := float64(0)
	totalExpected := float64(0)

	for _, acc := range bucketAccuracies {
		totalTrades += acc.TotalTrades
		totalActual += acc.ActualWinRate * float64(acc.TotalTrades)
		totalExpected += acc.ExpectedWinRate * float64(acc.TotalTrades)
	}

	overallActual := float64(0)
	overallExpected := float64(0)
	if totalTrades > 0 {
		overallActual = totalActual / float64(totalTrades)
		overallExpected = totalExpected / float64(totalTrades)
	}

	return &CalibrationSummaryData{
		Strategy:               strategy,
		OverallExpected:        overallExpected,
		OverallActual:          overallActual,
		OverallDelta:           overallActual - overallExpected,
		BucketAccuracies:       bucketAccuracies,
		TotalCalibrationTrades: totalTrades,
		LastUpdated:            calibData.LastUpdated.Format("2006-01-02T15:04:05Z07:00"),
		ConfidenceLevel:        confidenceLevel,
		IsReliable:             isReliable,
		MinimumTradesRequired:  50,
	}
}

// buildBucketAccuracy creates calibration accuracy data for a bucket.
func buildBucketAccuracy(bucket string, expectedWinRate float64, calibBucket *decision.ScoreBucket) CalibrationAccuracyData {
	if calibBucket == nil {
		return CalibrationAccuracyData{
			ScoreBucket:     bucket,
			ExpectedWinRate: expectedWinRate,
			ActualWinRate:   0,
			Delta:           -expectedWinRate,
			TotalTrades:     0,
			Confidence:      "low",
			IsAccurate:      false,
		}
	}

	// WinRate is stored as 0-1, convert to percentage for display
	actualWinRatePct := calibBucket.WinRate * 100
	delta := actualWinRatePct - expectedWinRate
	confidence := "low"
	if calibBucket.TotalTrades >= 50 {
		confidence = "high"
	} else if calibBucket.TotalTrades >= 20 {
		confidence = "medium"
	}

	isAccurate := delta >= -10 && delta <= 10

	return CalibrationAccuracyData{
		ScoreBucket:     bucket,
		ExpectedWinRate: expectedWinRate,
		ActualWinRate:   actualWinRatePct,
		Delta:           delta,
		TotalTrades:     int(calibBucket.TotalTrades),
		Confidence:      confidence,
		IsAccurate:      isAccurate,
	}
}

// getRegimeDistributionForDashboard builds regime distribution stats.
// TODO(Story-11.4/11.5): Integrate with MarketRegimeClassifier once implemented.
// Currently returns placeholder data indicating no real regime data is available.
func (s *Server) getRegimeDistributionForDashboard(startTime, endTime time.Time) *RegimeDistributionData {
	// Check if regime classifier service is available
	regimeClassifier := s.getRegimeClassifier()
	if regimeClassifier != nil {
		// Attempt to get real regime data
		regimeData, err := regimeClassifier.GetRegimeStats(startTime, endTime)
		if err == nil && regimeData != nil {
			return convertRegimeDataToDistribution(regimeData, startTime, endTime)
		}
	}

	// Return empty/placeholder data indicating no regime tracking yet
	// This clearly communicates to the frontend that data is unavailable
	stats := []RegimeStatsData{
		{
			Regime:          "TRENDING",
			Count:           0,
			Percentage:      0.0,
			AvgDuration:     0,
			TradesInRegime:  0,
			WinRateInRegime: 0,
			AvgPnLInRegime:  0,
		},
		{
			Regime:          "RANGING",
			Count:           0,
			Percentage:      0.0,
			AvgDuration:     0,
			TradesInRegime:  0,
			WinRateInRegime: 0,
			AvgPnLInRegime:  0,
		},
		{
			Regime:          "VOLATILE",
			Count:           0,
			Percentage:      0.0,
			AvgDuration:     0,
			TradesInRegime:  0,
			WinRateInRegime: 0,
			AvgPnLInRegime:  0,
		},
		{
			Regime:          "CONSOLIDATING",
			Count:           0,
			Percentage:      0.0,
			AvgDuration:     0,
			TradesInRegime:  0,
			WinRateInRegime: 0,
			AvgPnLInRegime:  0,
		},
	}

	return &RegimeDistributionData{
		Stats:                stats,
		TotalClassifications: 0,
		MostCommonRegime:     "TRENDING", // Default when no data
		PeriodStart:          startTime.Format("2006-01-02"),
		PeriodEnd:            endTime.Format("2006-01-02"),
	}
}

// getRegimeClassifier returns the regime classifier service if available.
// TODO(Story-11.4): Implement actual service connection once MarketRegimeClassifier is complete.
func (s *Server) getRegimeClassifier() interface{ GetRegimeStats(start, end time.Time) (interface{}, error) } {
	// Placeholder - return nil until regime classifier is implemented
	return nil
}

// convertRegimeDataToDistribution converts regime classifier output to dashboard format.
// TODO(Story-11.4): Implement actual conversion once regime data structure is defined.
func convertRegimeDataToDistribution(data interface{}, startTime, endTime time.Time) *RegimeDistributionData {
	// Placeholder for future implementation
	return nil
}

// getBlockingStatsForDashboard builds blocking reason statistics.
// TODO(Story-11.18): Integrate with BlockingReasonTracker once implemented.
// Currently returns empty data indicating no blocking history available yet.
func (s *Server) getBlockingStatsForDashboard(userID int, startTime, endTime time.Time) *BlockingStatsData {
	// Try to get real blocking data from the blocking tracker service
	blockingTracker := s.getBlockingTracker()
	if blockingTracker != nil {
		blockingData, err := blockingTracker.GetBlockingStats(userID, startTime, endTime)
		if err == nil && blockingData != nil {
			return convertBlockingDataToStats(blockingData, startTime, endTime)
		}
	}

	// Return empty data indicating no blocking tracking yet
	// This clearly communicates to the frontend that data is unavailable
	return &BlockingStatsData{
		TotalBlocks:       0,
		HardBlockCount:    0,
		SoftBlockCount:    0,
		WarningCount:      0,
		ByReason:          []BlockingReasonFrequency{},
		TopReasons:        []BlockingReasonFrequency{},
		SymbolsAffected:   0,
		MostFrequentBlock: "",
		PeriodStart:       startTime.Format("2006-01-02"),
		PeriodEnd:         endTime.Format("2006-01-02"),
	}
}

// getBlockingTracker returns the blocking tracker service if available.
// TODO(Story-11.18): Implement actual service connection once BlockingReasonTracker is complete.
func (s *Server) getBlockingTracker() interface {
	GetBlockingStats(userID int, start, end time.Time) (interface{}, error)
} {
	// Placeholder - return nil until blocking tracker is implemented
	return nil
}

// convertBlockingDataToStats converts blocking tracker output to dashboard format.
// TODO(Story-11.18): Implement actual conversion once blocking data structure is defined.
func convertBlockingDataToStats(data interface{}, startTime, endTime time.Time) *BlockingStatsData {
	// Placeholder for future implementation
	return nil
}
