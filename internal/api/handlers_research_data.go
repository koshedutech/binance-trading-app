// Package api provides HTTP API handlers for the trading bot.
// Epic 15, Story 15.3: Data Download API Endpoints
// Provides endpoints for downloading historical candle data, checking download status,
// and querying data availability for the Pattern Discovery Agent.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"binance-trading-bot/internal/research"

	"github.com/gin-gonic/gin"
)

// DownloadDataRequest represents the request body for starting a download job.
// POST /api/research/download-data
type DownloadDataRequest struct {
	Symbol     string   `json:"symbol" binding:"required"`     // e.g., "MATICUSDT"
	Timeframes []string `json:"timeframes" binding:"required"` // e.g., ["5m", "15m", "1h"]
	From       string   `json:"from" binding:"required"`       // e.g., "2023-01-01"
	To         string   `json:"to" binding:"required"`         // e.g., "2026-01-27"
}

// DownloadDataResponse represents the response for a started download job.
type DownloadDataResponse struct {
	JobID            string `json:"job_id"`
	Status           string `json:"status"`
	EstimatedCandles int    `json:"estimated_candles"`
}

// DownloadStatusResponse represents the response for download status check.
type DownloadStatusResponse struct {
	JobID             string  `json:"job_id"`
	Status            string  `json:"status"`
	Progress          float64 `json:"progress"`
	CandlesDownloaded int     `json:"candles_downloaded"`
	CandlesTotal      int     `json:"candles_total"`
	Error             string  `json:"error,omitempty"`
}

// CoinDataInfo represents data availability for a single coin.
type CoinDataInfo struct {
	Symbol        string   `json:"symbol"`
	Timeframes    []string `json:"timeframes"`
	FromDate      string   `json:"from_date"`
	ToDate        string   `json:"to_date"`
	TotalCandles  int64    `json:"total_candles"`
	StorageSizeMB float64  `json:"storage_size_mb"`
}

// DataAvailabilityResponse represents the response for data availability query.
type DataAvailabilityResponse struct {
	Coins         []CoinDataInfo `json:"coins"`
	TotalCoins    int            `json:"total_coins"`
	TotalCandles  int64          `json:"total_candles"`
	StorageSizeMB float64        `json:"storage_size_mb"`
}

// handleStartDownload starts a new data download job.
// POST /api/research/download-data
func (s *Server) handleStartDownload(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	var req DownloadDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate symbol format (uppercase alphanumeric, 5-20 chars)
	if len(req.Symbol) < 5 || len(req.Symbol) > 20 {
		errorResponse(c, http.StatusBadRequest, "Invalid symbol length (expected 5-20 characters)")
		return
	}
	for _, char := range req.Symbol {
		if !((char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			errorResponse(c, http.StatusBadRequest, "Invalid symbol format (must be uppercase alphanumeric, e.g., BTCUSDT)")
			return
		}
	}

	// Validate timeframes
	if len(req.Timeframes) == 0 {
		errorResponse(c, http.StatusBadRequest, "At least one timeframe is required")
		return
	}

	// Parse dates
	fromDate, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid from date format (use YYYY-MM-DD)")
		return
	}

	toDate, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid to date format (use YYYY-MM-DD)")
		return
	}

	// Validate date range
	if toDate.Before(fromDate) {
		errorResponse(c, http.StatusBadRequest, "to date must be after from date")
		return
	}

	// Binance futures launched in September 2019; data before this is unavailable
	minDate := time.Date(2019, 9, 1, 0, 0, 0, 0, time.UTC)
	if fromDate.Before(minDate) {
		errorResponse(c, http.StatusBadRequest, "from date cannot be before 2019-09-01 (Binance futures launch date)")
		return
	}

	if toDate.After(time.Now()) {
		toDate = time.Now()
	}

	// Limit maximum date range to 3 years to prevent excessive downloads
	maxRange := 3 * 365 * 24 * time.Hour
	if toDate.Sub(fromDate) > maxRange {
		errorResponse(c, http.StatusBadRequest, "date range cannot exceed 3 years")
		return
	}

	// Check if DataDownloader is available
	if s.dataDownloader == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Data download service not available")
		return
	}

	// Use a timeout for the initial job creation (download runs in background)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Start downloads for each timeframe
	var totalEstimatedCandles int
	var lastJobID string
	var firstError error

	var invalidTimeframes []string
	for _, tfStr := range req.Timeframes {
		tf := research.Timeframe(tfStr)
		if !tf.IsValid() {
			invalidTimeframes = append(invalidTimeframes, tfStr)
			continue
		}

		downloadReq := research.DownloadRequest{
			Symbol:    req.Symbol,
			Timeframe: tf,
			StartTime: fromDate,
			EndTime:   toDate,
		}

		job, err := s.dataDownloader.StartDownload(ctx, downloadReq)
		if err != nil {
			if firstError == nil {
				firstError = err
			}
			continue
		}

		totalEstimatedCandles += job.Progress.TotalCandles
		lastJobID = job.ID
	}

	if lastJobID == "" {
		if firstError != nil {
			errorResponse(c, http.StatusBadRequest, "Failed to start download: "+firstError.Error())
		} else if len(invalidTimeframes) > 0 {
			errorResponse(c, http.StatusBadRequest, fmt.Sprintf("No valid timeframes provided. Invalid timeframes: %v. Valid options: 1m, 5m, 15m, 30m, 1h, 4h, 1d", invalidTimeframes))
		} else {
			errorResponse(c, http.StatusBadRequest, "No valid timeframes provided")
		}
		return
	}

	// For multi-timeframe downloads, create a composite job ID
	compositeJobID := lastJobID
	if len(req.Timeframes) > 1 {
		compositeJobID = req.Symbol + "_" + req.From + "_" + req.To
	}

	successResponse(c, DownloadDataResponse{
		JobID:            compositeJobID,
		Status:           "started",
		EstimatedCandles: totalEstimatedCandles,
	})
}

// handleGetDownloadStatus checks the status of a download job.
// GET /api/research/download-status/:job_id
func (s *Server) handleGetDownloadStatus(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	jobID := c.Param("job_id")
	if jobID == "" {
		errorResponse(c, http.StatusBadRequest, "Job ID is required")
		return
	}

	if s.dataDownloader == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Data download service not available")
		return
	}

	ctx := c.Request.Context()

	job, err := s.dataDownloader.GetJob(ctx, jobID)
	if err != nil {
		// Try to find by listing all jobs and matching prefix
		jobs, listErr := s.dataDownloader.ListJobs(ctx)
		if listErr != nil {
			errorResponse(c, http.StatusNotFound, "Job not found: "+err.Error())
			return
		}

		// Find matching jobs by symbol prefix (using strings.HasPrefix for safety)
		var matchingJobs []*research.DownloadJob
		for _, j := range jobs {
			if strings.HasPrefix(j.ID, jobID) {
				matchingJobs = append(matchingJobs, j)
			}
		}

		if len(matchingJobs) == 0 {
			errorResponse(c, http.StatusNotFound, "Job not found")
			return
		}

		// Aggregate status from all matching jobs
		var totalDownloaded, totalCandles int
		var overallStatus string = "completed"
		var lastError string

		for _, j := range matchingJobs {
			totalDownloaded += j.Progress.DownloadedCandles
			totalCandles += j.Progress.TotalCandles

			switch j.Status {
			case research.JobStatusRunning, research.JobStatusPending:
				overallStatus = "in_progress"
			case research.JobStatusFailed:
				if overallStatus != "in_progress" {
					overallStatus = "failed"
				}
				lastError = j.Error
			case research.JobStatusPaused:
				if overallStatus != "in_progress" && overallStatus != "failed" {
					overallStatus = "paused"
				}
			}
		}

		progress := float64(0)
		if totalCandles > 0 {
			progress = float64(totalDownloaded) / float64(totalCandles) * 100
		}

		successResponse(c, DownloadStatusResponse{
			JobID:             jobID,
			Status:            overallStatus,
			Progress:          progress,
			CandlesDownloaded: totalDownloaded,
			CandlesTotal:      totalCandles,
			Error:             lastError,
		})
		return
	}

	// Map internal status to API status
	apiStatus := string(job.Status)
	switch job.Status {
	case research.JobStatusRunning:
		apiStatus = "in_progress"
	case research.JobStatusPending:
		apiStatus = "pending"
	case research.JobStatusCompleted:
		apiStatus = "completed"
	case research.JobStatusFailed:
		apiStatus = "failed"
	case research.JobStatusPaused:
		apiStatus = "paused"
	case research.JobStatusCancelled:
		apiStatus = "cancelled"
	}

	successResponse(c, DownloadStatusResponse{
		JobID:             job.ID,
		Status:            apiStatus,
		Progress:          job.Progress.PercentComplete,
		CandlesDownloaded: job.Progress.DownloadedCandles,
		CandlesTotal:      job.Progress.TotalCandles,
		Error:             job.Error,
	})
}

// handleGetDataAvailability returns information about available data.
// GET /api/research/data-availability
func (s *Server) handleGetDataAvailability(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	if s.dataDownloader == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Data download service not available")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// Query all available data grouped by symbol
	availability, err := s.getDataAvailability(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to query data availability: "+err.Error())
		return
	}

	successResponse(c, availability)
}

// getDataAvailability queries the database for all available candle data.
func (s *Server) getDataAvailability(ctx context.Context) (*DataAvailabilityResponse, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("database repository not available")
	}

	// Query to get all available data grouped by symbol and timeframe
	query := `
		SELECT
			symbol,
			timeframe,
			MIN(open_time) as first_candle,
			MAX(open_time) as last_candle,
			COUNT(*) as candle_count
		FROM research_candles
		GROUP BY symbol, timeframe
		ORDER BY symbol, timeframe
	`

	rows, err := s.repo.GetDB().Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Aggregate data by symbol
	symbolData := make(map[string]*CoinDataInfo)
	var totalCandles int64

	for rows.Next() {
		var symbol, timeframe string
		var firstCandle, lastCandle time.Time
		var candleCount int64

		if err := rows.Scan(&symbol, &timeframe, &firstCandle, &lastCandle, &candleCount); err != nil {
			return nil, err
		}

		totalCandles += candleCount

		if existing, ok := symbolData[symbol]; ok {
			existing.Timeframes = append(existing.Timeframes, timeframe)
			existing.TotalCandles += candleCount

			// Update date range if this timeframe has wider range
			if firstCandle.Format("2006-01-02") < existing.FromDate {
				existing.FromDate = firstCandle.Format("2006-01-02")
			}
			if lastCandle.Format("2006-01-02") > existing.ToDate {
				existing.ToDate = lastCandle.Format("2006-01-02")
			}
		} else {
			symbolData[symbol] = &CoinDataInfo{
				Symbol:       symbol,
				Timeframes:   []string{timeframe},
				FromDate:     firstCandle.Format("2006-01-02"),
				ToDate:       lastCandle.Format("2006-01-02"),
				TotalCandles: candleCount,
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert map to slice
	coins := make([]CoinDataInfo, 0, len(symbolData))
	for _, coin := range symbolData {
		// Estimate storage size (approximately 100 bytes per candle)
		coin.StorageSizeMB = float64(coin.TotalCandles) * 100 / (1024 * 1024)
		coins = append(coins, *coin)
	}

	// Calculate total storage size
	totalStorageMB := float64(totalCandles) * 100 / (1024 * 1024)

	return &DataAvailabilityResponse{
		Coins:         coins,
		TotalCoins:    len(coins),
		TotalCandles:  totalCandles,
		StorageSizeMB: totalStorageMB,
	}, nil
}

// handleCancelDownload cancels a running download job.
// POST /api/research/download-cancel/:job_id
func (s *Server) handleCancelDownload(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	jobID := c.Param("job_id")
	if jobID == "" {
		errorResponse(c, http.StatusBadRequest, "Job ID is required")
		return
	}

	if s.dataDownloader == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Data download service not available")
		return
	}

	if err := s.dataDownloader.CancelJob(jobID); err != nil {
		errorResponse(c, http.StatusNotFound, "Failed to cancel job: "+err.Error())
		return
	}

	successResponse(c, gin.H{
		"job_id":  jobID,
		"status":  "cancelled",
		"message": "Download job cancelled successfully",
	})
}

// handleListDownloadJobs lists all download jobs.
// GET /api/research/download-jobs
func (s *Server) handleListDownloadJobs(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	if s.dataDownloader == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Data download service not available")
		return
	}

	ctx := c.Request.Context()

	jobs, err := s.dataDownloader.ListJobs(ctx)
	if err != nil {
		errorResponse(c, http.StatusInternalServerError, "Failed to list jobs: "+err.Error())
		return
	}

	// Convert to response format
	jobResponses := make([]DownloadStatusResponse, 0, len(jobs))
	for _, job := range jobs {
		apiStatus := string(job.Status)
		switch job.Status {
		case research.JobStatusRunning:
			apiStatus = "in_progress"
		case research.JobStatusPending:
			apiStatus = "pending"
		case research.JobStatusCompleted:
			apiStatus = "completed"
		case research.JobStatusFailed:
			apiStatus = "failed"
		case research.JobStatusPaused:
			apiStatus = "paused"
		case research.JobStatusCancelled:
			apiStatus = "cancelled"
		}

		jobResponses = append(jobResponses, DownloadStatusResponse{
			JobID:             job.ID,
			Status:            apiStatus,
			Progress:          job.Progress.PercentComplete,
			CandlesDownloaded: job.Progress.DownloadedCandles,
			CandlesTotal:      job.Progress.TotalCandles,
			Error:             job.Error,
		})
	}

	successResponse(c, gin.H{
		"jobs":  jobResponses,
		"total": len(jobResponses),
	})
}

// FeatureJobResponse represents the response for a feature calculation job.
type FeatureJobResponse struct {
	JobID            string  `json:"job_id"`
	Symbol           string  `json:"symbol"`
	Timeframe        string  `json:"timeframe"`
	Status           string  `json:"status"`
	Progress         float64 `json:"progress"`
	TotalCandles     int     `json:"total_candles"`
	ProcessedCandles int     `json:"processed_candles"`
	CachedFeatures   int     `json:"cached_features"`
	Error            string  `json:"error,omitempty"`
}

// handleTriggerFeatureCalculation triggers feature calculation for a symbol/timeframe.
// POST /api/research/calculate-features
func (s *Server) handleTriggerFeatureCalculation(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	var req struct {
		Symbol    string `json:"symbol" binding:"required"`
		Timeframe string `json:"timeframe" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "Invalid request: "+err.Error())
		return
	}

	// Validate timeframe
	tf := research.Timeframe(req.Timeframe)
	if !tf.IsValid() {
		errorResponse(c, http.StatusBadRequest, "Invalid timeframe: "+req.Timeframe)
		return
	}

	// Check if feature job service is available
	if s.featureJobService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Feature job service not available")
		return
	}

	ctx := c.Request.Context()

	job, err := s.featureJobService.TriggerFeatureCalculation(ctx, req.Symbol, tf)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "Failed to trigger feature calculation: "+err.Error())
		return
	}

	successResponse(c, FeatureJobResponse{
		JobID:            job.ID,
		Symbol:           job.Symbol,
		Timeframe:        string(job.Timeframe),
		Status:           string(job.Status),
		Progress:         job.Progress,
		TotalCandles:     job.TotalCandles,
		ProcessedCandles: job.ProcessedCandles,
		CachedFeatures:   job.CachedFeatures,
		Error:            job.Error,
	})
}

// handleGetFeatureJobStatus gets the status of a feature calculation job.
// GET /api/research/feature-job/:job_id
func (s *Server) handleGetFeatureJobStatus(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	jobID := c.Param("job_id")
	if jobID == "" {
		errorResponse(c, http.StatusBadRequest, "Job ID is required")
		return
	}

	if s.featureJobService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Feature job service not available")
		return
	}

	job := s.featureJobService.GetJob(jobID)
	if job == nil {
		errorResponse(c, http.StatusNotFound, "Job not found: "+jobID)
		return
	}

	successResponse(c, FeatureJobResponse{
		JobID:            job.ID,
		Symbol:           job.Symbol,
		Timeframe:        string(job.Timeframe),
		Status:           string(job.Status),
		Progress:         job.Progress,
		TotalCandles:     job.TotalCandles,
		ProcessedCandles: job.ProcessedCandles,
		CachedFeatures:   job.CachedFeatures,
		Error:            job.Error,
	})
}

// handleListFeatureJobs lists all feature calculation jobs.
// GET /api/research/feature-jobs
func (s *Server) handleListFeatureJobs(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	if s.featureJobService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Feature job service not available")
		return
	}

	jobs := s.featureJobService.ListJobs()

	responses := make([]FeatureJobResponse, 0, len(jobs))
	for _, job := range jobs {
		responses = append(responses, FeatureJobResponse{
			JobID:            job.ID,
			Symbol:           job.Symbol,
			Timeframe:        string(job.Timeframe),
			Status:           string(job.Status),
			Progress:         job.Progress,
			TotalCandles:     job.TotalCandles,
			ProcessedCandles: job.ProcessedCandles,
			CachedFeatures:   job.CachedFeatures,
			Error:            job.Error,
		})
	}

	successResponse(c, gin.H{
		"jobs":  responses,
		"total": len(responses),
	})
}

// handleGetFeatureJobStats gets statistics about the feature job service.
// GET /api/research/feature-job-stats
func (s *Server) handleGetFeatureJobStats(c *gin.Context) {
	// Validate user authentication
	if _, ok := s.getUserIDRequired(c); !ok {
		return
	}

	if s.featureJobService == nil {
		errorResponse(c, http.StatusServiceUnavailable, "Feature job service not available")
		return
	}

	stats := s.featureJobService.GetStats()
	successResponse(c, stats)
}
