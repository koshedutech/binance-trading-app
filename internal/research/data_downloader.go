// Package research provides data structures and services for research infrastructure.
// Story 15.2: Data Download Service - Historical candle downloader for backtesting
package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/cache"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// DownloadJobStatus represents the status of a download job
type DownloadJobStatus string

const (
	JobStatusPending    DownloadJobStatus = "pending"
	JobStatusRunning    DownloadJobStatus = "running"
	JobStatusCompleted  DownloadJobStatus = "completed"
	JobStatusFailed     DownloadJobStatus = "failed"
	JobStatusCancelled  DownloadJobStatus = "cancelled"
	JobStatusPaused     DownloadJobStatus = "paused"
)

// DownloadJob represents a candle download job
type DownloadJob struct {
	ID           string            `json:"id"`
	Symbol       string            `json:"symbol"`
	Timeframe    Timeframe         `json:"timeframe"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	Status       DownloadJobStatus `json:"status"`
	Progress     DownloadProgress  `json:"progress"`
	Error        string            `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	CompletedAt  *time.Time        `json:"completed_at,omitempty"`
}

// DownloadProgress tracks the progress of a download job
type DownloadProgress struct {
	TotalCandles     int       `json:"total_candles"`      // Expected total candles
	DownloadedCandles int      `json:"downloaded_candles"` // Actually downloaded
	InsertedCandles  int       `json:"inserted_candles"`   // Successfully stored
	SkippedCandles   int       `json:"skipped_candles"`    // Duplicates skipped
	LastProcessedTime time.Time `json:"last_processed_time"` // For resume support
	PercentComplete  float64   `json:"percent_complete"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EstimatedETA     *time.Time `json:"estimated_eta,omitempty"`
}

// DownloadRequest represents a request to download candles
type DownloadRequest struct {
	Symbol    string    `json:"symbol"`
	Timeframe Timeframe `json:"timeframe"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Validate checks if the download request is valid
func (r *DownloadRequest) Validate() error {
	if r.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	// Basic symbol format validation for Binance futures
	// Symbols should be uppercase, alphanumeric, typically ending in USDT/BUSD
	if len(r.Symbol) < 5 || len(r.Symbol) > 20 {
		return fmt.Errorf("invalid symbol length: %s (expected 5-20 characters)", r.Symbol)
	}
	// Check for valid characters (alphanumeric only)
	for _, c := range r.Symbol {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return fmt.Errorf("invalid symbol format: %s (must be uppercase alphanumeric)", r.Symbol)
		}
	}
	if !r.Timeframe.IsValid() {
		return fmt.Errorf("invalid timeframe: %s", r.Timeframe)
	}
	if r.StartTime.IsZero() {
		return fmt.Errorf("start_time is required")
	}
	if r.EndTime.IsZero() {
		return fmt.Errorf("end_time is required")
	}
	if r.EndTime.Before(r.StartTime) {
		return fmt.Errorf("end_time must be after start_time")
	}
	// Binance API doesn't allow future dates
	if r.EndTime.After(time.Now()) {
		return fmt.Errorf("end_time cannot be in the future")
	}
	return nil
}

// EstimateCandleCount estimates the number of candles for the given request
func (r *DownloadRequest) EstimateCandleCount() int {
	duration := r.EndTime.Sub(r.StartTime)
	candleDuration := r.Timeframe.Duration()
	if candleDuration == 0 {
		return 0
	}
	return int(duration / candleDuration)
}

// DownloadCompleteCallback is called when a download job completes successfully
// It receives the symbol and timeframe of the completed download
type DownloadCompleteCallback func(ctx context.Context, symbol string, timeframe Timeframe)

// DataDownloader handles downloading historical candle data from Binance
type DataDownloader struct {
	pool         *pgxpool.Pool
	cacheService *cache.CacheService
	rateLimiter  *binance.RateLimiter

	// Active jobs tracking
	mu          sync.RWMutex
	activeJobs  map[string]context.CancelFunc

	// Configuration
	batchSize       int           // Max candles per API request (Binance max: 1000)
	requestDelay    time.Duration // Delay between API requests
	maxConcurrent   int           // Max concurrent downloads
	semaphore       chan struct{} // Concurrency limiter

	// Callback for download completion (triggers feature calculation)
	onComplete DownloadCompleteCallback
}

// DataDownloaderConfig holds configuration for the downloader
type DataDownloaderConfig struct {
	BatchSize      int           // Default: 1000 (Binance max)
	RequestDelay   time.Duration // Default: 100ms
	MaxConcurrent  int           // Default: 3
}

// DefaultDownloaderConfig returns the default configuration
func DefaultDownloaderConfig() DataDownloaderConfig {
	return DataDownloaderConfig{
		BatchSize:     1000,         // Binance API max
		RequestDelay:  100 * time.Millisecond, // ~600 req/min, well under 1200 limit
		MaxConcurrent: 3,
	}
}

// NewDataDownloader creates a new DataDownloader
func NewDataDownloader(pool *pgxpool.Pool, cacheService *cache.CacheService, config DataDownloaderConfig) *DataDownloader {
	if config.BatchSize <= 0 || config.BatchSize > 1000 {
		config.BatchSize = 1000
	}
	if config.RequestDelay <= 0 {
		config.RequestDelay = 100 * time.Millisecond
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 3
	}

	return &DataDownloader{
		pool:          pool,
		cacheService:  cacheService,
		rateLimiter:   binance.GetRateLimiter(),
		activeJobs:    make(map[string]context.CancelFunc),
		batchSize:     config.BatchSize,
		requestDelay:  config.RequestDelay,
		maxConcurrent: config.MaxConcurrent,
		semaphore:     make(chan struct{}, config.MaxConcurrent),
	}
}

// Redis key prefixes for job tracking
const (
	jobKeyPrefix     = "research:download:job:"
	jobListKey       = "research:download:jobs"
	jobTTL           = 7 * 24 * time.Hour // Keep job info for 7 days
)

// SetOnCompleteCallback sets the callback function to be called when a download completes.
// This is used to trigger feature calculation after new candles are downloaded.
func (d *DataDownloader) SetOnCompleteCallback(callback DownloadCompleteCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onComplete = callback
}

// generateJobID creates a unique job ID
func generateJobID(symbol string, timeframe Timeframe, startTime, endTime time.Time) string {
	return fmt.Sprintf("%s_%s_%d_%d", symbol, timeframe, startTime.Unix(), endTime.Unix())
}

// StartDownload initiates a new download job
// Returns the job ID and starts the download in the background
func (d *DataDownloader) StartDownload(ctx context.Context, req DownloadRequest) (*DownloadJob, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	jobID := generateJobID(req.Symbol, req.Timeframe, req.StartTime, req.EndTime)

	// Check if job already exists
	existingJob, err := d.GetJob(ctx, jobID)
	if err == nil && existingJob != nil {
		// Job exists - check if we should resume or reject
		switch existingJob.Status {
		case JobStatusRunning:
			return nil, fmt.Errorf("download job already running")
		case JobStatusCompleted:
			return existingJob, nil // Return existing completed job
		case JobStatusPaused, JobStatusFailed:
			// Resume from where we left off
			return d.resumeJob(ctx, existingJob)
		}
	}

	// Create new job
	now := time.Now()
	job := &DownloadJob{
		ID:        jobID,
		Symbol:    req.Symbol,
		Timeframe: req.Timeframe,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Status:    JobStatusPending,
		Progress: DownloadProgress{
			TotalCandles:      req.EstimateCandleCount(),
			LastProcessedTime: req.StartTime,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save job to Redis
	if err := d.saveJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to save job: %w", err)
	}

	// Start download in background
	go d.executeDownload(job)

	return job, nil
}

// resumeJob resumes a paused or failed job
func (d *DataDownloader) resumeJob(ctx context.Context, job *DownloadJob) (*DownloadJob, error) {
	job.Status = JobStatusPending
	job.Error = ""
	job.UpdatedAt = time.Now()

	if err := d.saveJob(ctx, job); err != nil {
		return nil, fmt.Errorf("failed to save resumed job: %w", err)
	}

	go d.executeDownload(job)

	return job, nil
}

// executeDownload performs the actual download
func (d *DataDownloader) executeDownload(job *DownloadJob) {
	// Acquire semaphore for concurrency limiting with timeout
	// This prevents blocking forever if all slots are taken during shutdown
	select {
	case d.semaphore <- struct{}{}:
		// Got the slot
	case <-time.After(30 * time.Second):
		// Timeout waiting for slot - mark job as failed
		job.Status = JobStatusFailed
		job.Error = "timeout waiting for download slot"
		job.UpdatedAt = time.Now()
		d.saveJob(context.Background(), job)
		log.Printf("[RESEARCH] Download failed (timeout waiting for slot): %s", job.ID)
		return
	}
	defer func() { <-d.semaphore }()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register active job
	d.mu.Lock()
	d.activeJobs[job.ID] = cancel
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.activeJobs, job.ID)
		d.mu.Unlock()
	}()

	// Update status to running
	now := time.Now()
	job.Status = JobStatusRunning
	job.Progress.StartedAt = &now
	job.UpdatedAt = now
	if err := d.saveJob(ctx, job); err != nil {
		log.Printf("[RESEARCH] Failed to update job status: %v", err)
	}

	log.Printf("[RESEARCH] Starting download: %s %s from %s to %s (est. %d candles)",
		job.Symbol, job.Timeframe, job.StartTime.Format(time.RFC3339),
		job.EndTime.Format(time.RFC3339), job.Progress.TotalCandles)

	// Create Binance futures client (public API, no auth needed for klines)
	binanceClient := binance.NewFuturesClient("", "", false)

	// Resume from last processed time
	currentTime := job.Progress.LastProcessedTime
	if currentTime.IsZero() {
		currentTime = job.StartTime
	}

	batchCount := 0
	for currentTime.Before(job.EndTime) {
		// Check for cancellation
		select {
		case <-ctx.Done():
			// Re-fetch job to check if it was paused (vs cancelled)
			// This prevents race condition where PauseJob sets status then cancels
			if latestJob, err := d.GetJob(context.Background(), job.ID); err == nil {
				if latestJob.Status == JobStatusPaused {
					log.Printf("[RESEARCH] Download paused: %s", job.ID)
					return
				}
			}
			job.Status = JobStatusCancelled
			job.UpdatedAt = time.Now()
			d.saveJob(context.Background(), job)
			log.Printf("[RESEARCH] Download cancelled: %s", job.ID)
			return
		default:
		}

		// Check rate limiter (read-only check - don't record weight here as publicGet does it)
		// Use CanMakeRequestWithPriority for read-only check to avoid double-counting weight
		if !d.rateLimiter.CanMakeRequestWithPriority("/fapi/v1/klines", binance.PriorityLow) {
			// Get wait time from rate limiter status
			_, _, _, waitTime := d.rateLimiter.GetCurrentUsage()
			if waitTime < 100*time.Millisecond {
				waitTime = 100 * time.Millisecond
			}
			if waitTime > 5*time.Second {
				waitTime = 5 * time.Second
			}
			log.Printf("[RESEARCH] Rate limited, waiting %v", waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Calculate end time for this batch
		batchEndTime := currentTime.Add(time.Duration(d.batchSize) * job.Timeframe.Duration())
		if batchEndTime.After(job.EndTime) {
			batchEndTime = job.EndTime
		}

		// Fetch candles from Binance
		klines, err := d.fetchKlines(ctx, binanceClient, job.Symbol, string(job.Timeframe),
			currentTime.UnixMilli(), batchEndTime.UnixMilli(), d.batchSize)
		if err != nil {
			log.Printf("[RESEARCH] Failed to fetch klines: %v", err)
			job.Status = JobStatusFailed
			job.Error = fmt.Sprintf("fetch failed: %v", err)
			job.UpdatedAt = time.Now()
			d.saveJob(context.Background(), job)
			return
		}

		if len(klines) == 0 {
			// No more data, move forward
			currentTime = batchEndTime
			continue
		}

		// Convert and store candles
		candles := d.convertKlinesToCandles(klines, job.Symbol, job.Timeframe)
		insertResult, err := d.bulkInsertCandles(ctx, candles)
		if err != nil {
			log.Printf("[RESEARCH] Failed to insert candles: %v", err)
			job.Status = JobStatusFailed
			job.Error = fmt.Sprintf("insert failed: %v", err)
			job.UpdatedAt = time.Now()
			d.saveJob(context.Background(), job)
			return
		}

		// Update progress
		job.Progress.DownloadedCandles += len(klines)
		job.Progress.InsertedCandles += insertResult.Inserted
		job.Progress.SkippedCandles += insertResult.Skipped

		// Update last processed time to the last candle's close time
		if len(klines) > 0 {
			lastKline := klines[len(klines)-1]
			job.Progress.LastProcessedTime = time.UnixMilli(lastKline.CloseTime)
			currentTime = job.Progress.LastProcessedTime.Add(time.Millisecond) // Move past last candle
		}

		// Calculate progress percentage
		if job.Progress.TotalCandles > 0 {
			job.Progress.PercentComplete = float64(job.Progress.DownloadedCandles) / float64(job.Progress.TotalCandles) * 100
			if job.Progress.PercentComplete > 100 {
				job.Progress.PercentComplete = 100
			}
		}

		// Estimate ETA
		if job.Progress.StartedAt != nil && job.Progress.PercentComplete > 0 {
			elapsed := time.Since(*job.Progress.StartedAt)
			totalEstimated := time.Duration(float64(elapsed) / job.Progress.PercentComplete * 100)
			eta := job.Progress.StartedAt.Add(totalEstimated)
			job.Progress.EstimatedETA = &eta
		}

		job.UpdatedAt = time.Now()

		// Save progress periodically (every 10 batches)
		batchCount++
		if batchCount%10 == 0 {
			if err := d.saveJob(ctx, job); err != nil {
				log.Printf("[RESEARCH] Failed to save progress: %v", err)
			}
			log.Printf("[RESEARCH] Download progress: %s %.1f%% (%d/%d candles)",
				job.ID, job.Progress.PercentComplete,
				job.Progress.DownloadedCandles, job.Progress.TotalCandles)
		}

		// Respect rate limits
		time.Sleep(d.requestDelay)
	}

	// Mark as completed
	now = time.Now()
	job.Status = JobStatusCompleted
	job.CompletedAt = &now
	job.UpdatedAt = now
	job.Progress.PercentComplete = 100

	if err := d.saveJob(context.Background(), job); err != nil {
		log.Printf("[RESEARCH] Failed to save completed job: %v", err)
	}

	log.Printf("[RESEARCH] Download completed: %s - %d candles inserted, %d skipped",
		job.ID, job.Progress.InsertedCandles, job.Progress.SkippedCandles)

	// Trigger feature calculation callback if set
	d.mu.RLock()
	callback := d.onComplete
	d.mu.RUnlock()

	if callback != nil {
		log.Printf("[RESEARCH] Triggering feature calculation for %s/%s", job.Symbol, job.Timeframe)
		// Run callback in a goroutine to not block the download completion
		// Wrap in recovery to prevent panics from crashing the goroutine silently
		go func(symbol string, tf Timeframe) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[RESEARCH] PANIC in feature calculation callback for %s/%s: %v", symbol, tf, r)
				}
			}()
			callback(context.Background(), symbol, tf)
		}(job.Symbol, job.Timeframe)
	}
}

// fetchKlines fetches klines from Binance API with time range
// Note: Context is accepted for future use but currently the Binance client
// doesn't support context cancellation. Cancel signals are checked in the
// main loop between API calls instead.
func (d *DataDownloader) fetchKlines(ctx context.Context, client *binance.FuturesClientImpl,
	symbol, interval string, startTime, endTime int64, limit int) ([]binance.Kline, error) {

	// Check context before making the call (since client doesn't support context)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use the FuturesClient method that supports time ranges
	klines, err := client.GetFuturesKlinesWithTimeRange(symbol, interval, startTime, endTime, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch klines: %w", err)
	}

	return klines, nil
}

// convertKlinesToCandles converts Binance Klines to ResearchCandles
func (d *DataDownloader) convertKlinesToCandles(klines []binance.Kline, symbol string, timeframe Timeframe) []*ResearchCandle {
	candles := make([]*ResearchCandle, 0, len(klines))

	for _, k := range klines {
		candle := &ResearchCandle{
			Symbol:         symbol,
			Timeframe:      timeframe,
			OpenTime:       time.UnixMilli(k.OpenTime).UTC(),
			CloseTime:      time.UnixMilli(k.CloseTime).UTC(),
			Open:           k.Open,
			High:           k.High,
			Low:            k.Low,
			Close:          k.Close,
			Volume:         k.Volume,
			QuoteVolume:    k.QuoteAssetVolume,
			TradeCount:     k.NumberOfTrades,
			TakerBuyVolume: k.TakerBuyBaseAssetVolume,
			TakerBuyQuote:  k.TakerBuyQuoteAssetVolume,
			CreatedAt:      time.Now(),
		}
		candles = append(candles, candle)
	}

	return candles
}

// bulkInsertCandles inserts candles into the database with upsert logic
func (d *DataDownloader) bulkInsertCandles(ctx context.Context, candles []*ResearchCandle) (*BulkInsertResult, error) {
	if len(candles) == 0 {
		return &BulkInsertResult{}, nil
	}

	startTime := time.Now()
	result := &BulkInsertResult{}

	// Use batch insert with ON CONFLICT for upsert
	batch := &pgx.Batch{}

	for _, c := range candles {
		batch.Queue(`
			INSERT INTO research_candles (
				symbol, timeframe, open_time, close_time,
				open, high, low, close,
				volume, quote_volume, trade_count,
				taker_buy_volume, taker_buy_quote, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			ON CONFLICT (symbol, timeframe, open_time) DO UPDATE SET
				open = EXCLUDED.open,
				high = EXCLUDED.high,
				low = EXCLUDED.low,
				close = EXCLUDED.close,
				volume = EXCLUDED.volume,
				quote_volume = EXCLUDED.quote_volume,
				trade_count = EXCLUDED.trade_count,
				taker_buy_volume = EXCLUDED.taker_buy_volume,
				taker_buy_quote = EXCLUDED.taker_buy_quote
		`,
			c.Symbol, string(c.Timeframe), c.OpenTime, c.CloseTime,
			c.Open, c.High, c.Low, c.Close,
			c.Volume, c.QuoteVolume, c.TradeCount,
			c.TakerBuyVolume, c.TakerBuyQuote, c.CreatedAt,
		)
	}

	br := d.pool.SendBatch(ctx, batch)

	for i := 0; i < len(candles); i++ {
		ct, err := br.Exec()
		if err != nil {
			result.Errors++
			log.Printf("[RESEARCH] Insert error for candle %d: %v", i, err)
			continue
		}

		// Note: With ON CONFLICT DO UPDATE, RowsAffected() returns 1 for both
		// new inserts and updates. We can't distinguish between them at the
		// batch level without additional queries. Count all successful ops as
		// "Inserted" which represents "processed successfully" in this context.
		if ct.RowsAffected() > 0 {
			result.Inserted++
		}
		// Skipped would only happen with ON CONFLICT DO NOTHING, which we don't use
	}

	// Close batch and check for errors
	if err := br.Close(); err != nil {
		return result, fmt.Errorf("batch close error: %w", err)
	}

	result.Duration = time.Since(startTime)

	// Return error if too many individual inserts failed (>10%)
	if result.Errors > 0 && float64(result.Errors)/float64(len(candles)) > 0.1 {
		return result, fmt.Errorf("batch insert had %d errors out of %d candles (%.1f%%)",
			result.Errors, len(candles), float64(result.Errors)/float64(len(candles))*100)
	}

	return result, nil
}

// saveJob saves job state to Redis
func (d *DataDownloader) saveJob(ctx context.Context, job *DownloadJob) error {
	if d.cacheService == nil {
		return nil // No Redis, skip persistence
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	client := d.cacheService.GetClient()
	if client == nil {
		return fmt.Errorf("redis client unavailable")
	}

	// Store in HASH for fast listing (HGETALL is O(n) where n = number of jobs)
	// The hash key is jobListKey, field is job ID, value is JSON
	if err := client.HSet(ctx, jobListKey, job.ID, string(data)).Err(); err != nil {
		return fmt.Errorf("failed to save job to hash: %w", err)
	}

	return nil
}

// GetJob retrieves a job by ID
func (d *DataDownloader) GetJob(ctx context.Context, jobID string) (*DownloadJob, error) {
	if d.cacheService == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	client := d.cacheService.GetClient()
	if client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	// Get from HASH
	data, err := client.HGet(ctx, jobListKey, jobID).Result()
	if err != nil {
		// Try legacy key format for backward compatibility
		legacyKey := jobKeyPrefix + jobID
		legacyData, legacyErr := d.cacheService.Get(ctx, legacyKey)
		if legacyErr != nil {
			return nil, fmt.Errorf("job not found: %s", jobID)
		}
		data = legacyData
	}

	var job DownloadJob
	if err := json.Unmarshal([]byte(data), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// CancelJob cancels a running download job
func (d *DataDownloader) CancelJob(jobID string) error {
	d.mu.Lock()
	cancel, exists := d.activeJobs[jobID]
	d.mu.Unlock()

	if !exists {
		return fmt.Errorf("job not found or not running: %s", jobID)
	}

	cancel()
	return nil
}

// Stop gracefully stops all active download jobs.
// This should be called during application shutdown to ensure clean termination.
func (d *DataDownloader) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	activeCount := len(d.activeJobs)
	if activeCount == 0 {
		log.Println("[RESEARCH] DataDownloader: No active jobs to stop")
		return
	}

	log.Printf("[RESEARCH] DataDownloader: Stopping %d active download jobs", activeCount)

	// Cancel all active jobs
	for jobID, cancel := range d.activeJobs {
		log.Printf("[RESEARCH] DataDownloader: Cancelling job %s", jobID)
		cancel()
	}

	// Note: Jobs will be marked as cancelled in their executeDownload goroutines
	// The activeJobs map will be cleared by the defer in executeDownload
}

// PauseJob pauses a running job (can be resumed later)
func (d *DataDownloader) PauseJob(ctx context.Context, jobID string) error {
	d.mu.Lock()
	cancel, exists := d.activeJobs[jobID]
	d.mu.Unlock()

	if !exists {
		return fmt.Errorf("job not found or not running: %s", jobID)
	}

	// Get current job state
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	// Update status to paused before cancelling
	// Note: executeDownload checks for JobStatusPaused before overwriting to cancelled
	job.Status = JobStatusPaused
	job.UpdatedAt = time.Now()
	if err := d.saveJob(ctx, job); err != nil {
		return fmt.Errorf("failed to save paused state: %w", err)
	}

	// Small delay to ensure the save completes before cancel signal is processed
	time.Sleep(10 * time.Millisecond)

	cancel()
	return nil
}

// ListJobs returns all tracked download jobs
func (d *DataDownloader) ListJobs(ctx context.Context) ([]*DownloadJob, error) {
	if d.cacheService == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	client := d.cacheService.GetClient()
	if client == nil {
		return nil, fmt.Errorf("redis client unavailable")
	}

	// Use HGETALL for O(n) performance where n = number of jobs (not total Redis keys)
	result, err := client.HGetAll(ctx, jobListKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs from hash: %w", err)
	}

	// If hash is empty, try to migrate from legacy individual keys
	if len(result) == 0 {
		return d.migrateAndListLegacyJobs(ctx, client)
	}

	jobs := make([]*DownloadJob, 0, len(result))
	for _, data := range result {
		var job DownloadJob
		if err := json.Unmarshal([]byte(data), &job); err != nil {
			continue
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

// migrateAndListLegacyJobs migrates jobs from legacy individual keys to the hash
func (d *DataDownloader) migrateAndListLegacyJobs(ctx context.Context, client *redis.Client) ([]*DownloadJob, error) {
	// Scan for legacy job keys (this is slow but only happens once during migration)
	keys, err := d.cacheService.ScanKeys(ctx, jobKeyPrefix+"*", 100)
	if err != nil {
		return nil, fmt.Errorf("failed to scan legacy job keys: %w", err)
	}

	if len(keys) == 0 {
		return []*DownloadJob{}, nil
	}

	log.Printf("[RESEARCH] Migrating %d legacy download jobs to hash storage", len(keys))

	jobs := make([]*DownloadJob, 0, len(keys))
	for _, key := range keys {
		data, err := d.cacheService.Get(ctx, key)
		if err != nil {
			continue
		}

		var job DownloadJob
		if err := json.Unmarshal([]byte(data), &job); err != nil {
			continue
		}
		jobs = append(jobs, &job)

		// Migrate to hash
		if err := client.HSet(ctx, jobListKey, job.ID, data).Err(); err != nil {
			log.Printf("[RESEARCH] Failed to migrate job %s to hash: %v", job.ID, err)
		}
	}

	log.Printf("[RESEARCH] Migration complete: %d jobs migrated to hash storage", len(jobs))
	return jobs, nil
}

// RecoverOrphanedJobs finds jobs that were "running" when the server stopped
// and marks them as "paused" so they can be resumed. This should be called on startup.
func (d *DataDownloader) RecoverOrphanedJobs(ctx context.Context) (int, error) {
	jobs, err := d.ListJobs(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	recovered := 0
	for _, job := range jobs {
		// Check if job was "running" but has no active goroutine
		if job.Status == JobStatusRunning || job.Status == JobStatusPending {
			d.mu.RLock()
			_, isActive := d.activeJobs[job.ID]
			d.mu.RUnlock()

			if !isActive {
				// This job was orphaned - mark as paused so it can be resumed
				job.Status = JobStatusPaused
				job.Error = "Interrupted by server restart. Click to resume."
				job.UpdatedAt = time.Now()

				if err := d.saveJob(ctx, job); err != nil {
					log.Printf("[RESEARCH] Failed to recover orphaned job %s: %v", job.ID, err)
					continue
				}

				log.Printf("[RESEARCH] Recovered orphaned job: %s (was at %.1f%%)",
					job.ID, job.Progress.PercentComplete)
				recovered++
			}
		}
	}

	if recovered > 0 {
		log.Printf("[RESEARCH] Recovered %d orphaned download jobs", recovered)
	}

	return recovered, nil
}

// ResumeJob manually resumes a paused job
func (d *DataDownloader) ResumeJob(ctx context.Context, jobID string) (*DownloadJob, error) {
	job, err := d.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}

	if job.Status != JobStatusPaused && job.Status != JobStatusFailed {
		return nil, fmt.Errorf("job cannot be resumed (status: %s)", job.Status)
	}

	return d.resumeJob(ctx, job)
}

// GetAvailableData returns information about what data is available in the database
func (d *DataDownloader) GetAvailableData(ctx context.Context, symbol string, timeframe Timeframe) (*AvailableData, error) {
	query := `
		SELECT
			symbol,
			timeframe,
			MIN(open_time) as first_candle,
			MAX(open_time) as last_candle,
			COUNT(*) as candle_count
		FROM research_candles
		WHERE symbol = $1 AND timeframe = $2
		GROUP BY symbol, timeframe
	`

	var data AvailableData
	var tf string
	err := d.pool.QueryRow(ctx, query, symbol, string(timeframe)).Scan(
		&data.Symbol, &tf, &data.FirstCandle, &data.LastCandle, &data.CandleCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &AvailableData{
				Symbol:    symbol,
				Timeframe: timeframe,
			}, nil
		}
		return nil, fmt.Errorf("failed to query available data: %w", err)
	}

	data.Timeframe = Timeframe(tf)
	return &data, nil
}

// DetectGaps finds gaps in the candle data for a symbol/timeframe
func (d *DataDownloader) DetectGaps(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time) ([]DataGap, error) {
	// Query candles ordered by time
	query := `
		SELECT open_time
		FROM research_candles
		WHERE symbol = $1 AND timeframe = $2 AND open_time >= $3 AND open_time <= $4
		ORDER BY open_time ASC
	`

	rows, err := d.pool.Query(ctx, query, symbol, string(timeframe), startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query candles: %w", err)
	}
	defer rows.Close()

	expectedInterval := timeframe.Duration()
	var gaps []DataGap
	var lastTime time.Time
	first := true

	for rows.Next() {
		var openTime time.Time
		if err := rows.Scan(&openTime); err != nil {
			return nil, fmt.Errorf("failed to scan open_time: %w", err)
		}

		if first {
			// Check gap from start
			if openTime.After(startTime.Add(expectedInterval)) {
				gap := DataGap{
					StartTime: startTime,
					EndTime:   openTime,
					Missing:   int(openTime.Sub(startTime) / expectedInterval),
				}
				gaps = append(gaps, gap)
			}
			first = false
		} else {
			// Check gap between candles
			expectedNext := lastTime.Add(expectedInterval)
			if openTime.After(expectedNext.Add(expectedInterval / 2)) { // Allow small tolerance
				gap := DataGap{
					StartTime: lastTime.Add(expectedInterval),
					EndTime:   openTime,
					Missing:   int(openTime.Sub(lastTime)/expectedInterval) - 1,
				}
				if gap.Missing > 0 {
					gaps = append(gaps, gap)
				}
			}
		}
		lastTime = openTime
	}

	// Check gap to end
	if !lastTime.IsZero() && lastTime.Before(endTime.Add(-expectedInterval)) {
		gap := DataGap{
			StartTime: lastTime.Add(expectedInterval),
			EndTime:   endTime,
			Missing:   int(endTime.Sub(lastTime) / expectedInterval),
		}
		gaps = append(gaps, gap)
	}

	return gaps, rows.Err()
}

// FillGaps downloads missing data to fill gaps
func (d *DataDownloader) FillGaps(ctx context.Context, symbol string, timeframe Timeframe, gaps []DataGap) ([]*DownloadJob, error) {
	var jobs []*DownloadJob

	for _, gap := range gaps {
		req := DownloadRequest{
			Symbol:    symbol,
			Timeframe: timeframe,
			StartTime: gap.StartTime,
			EndTime:   gap.EndTime,
		}

		job, err := d.StartDownload(ctx, req)
		if err != nil {
			log.Printf("[RESEARCH] Failed to start gap fill for %s: %v", gap.StartTime.Format(time.RFC3339), err)
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetCandles retrieves candles from the database for a given symbol, timeframe, and time range.
// Implements the CandleRepository interface used by BacktestEngine.
func (d *DataDownloader) GetCandles(ctx context.Context, symbol string, timeframe Timeframe, startTime, endTime time.Time) ([]*ResearchCandle, error) {
	query := `
		SELECT
			id, symbol, timeframe, open_time, close_time,
			open, high, low, close,
			volume, quote_volume, trade_count,
			taker_buy_volume, taker_buy_quote, created_at
		FROM research_candles
		WHERE symbol = $1 AND timeframe = $2 AND open_time >= $3 AND open_time <= $4
		ORDER BY open_time ASC
	`

	rows, err := d.pool.Query(ctx, query, symbol, string(timeframe), startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query candles: %w", err)
	}
	defer rows.Close()

	candles := make([]*ResearchCandle, 0)
	for rows.Next() {
		var c ResearchCandle
		var tf string
		err := rows.Scan(
			&c.ID, &c.Symbol, &tf, &c.OpenTime, &c.CloseTime,
			&c.Open, &c.High, &c.Low, &c.Close,
			&c.Volume, &c.QuoteVolume, &c.TradeCount,
			&c.TakerBuyVolume, &c.TakerBuyQuote, &c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan candle: %w", err)
		}
		c.Timeframe = Timeframe(tf)
		candles = append(candles, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating candles: %w", err)
	}

	return candles, nil
}
