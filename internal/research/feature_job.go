// Package research provides data structures and services for research infrastructure.
// Story 15.6: Feature Calculation Background Job - Triggers feature calculation after download
package research

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FeatureJobStatus represents the status of a feature calculation job
type FeatureJobStatus string

const (
	FeatureJobPending    FeatureJobStatus = "pending"
	FeatureJobRunning    FeatureJobStatus = "running"
	FeatureJobCompleted  FeatureJobStatus = "completed"
	FeatureJobFailed     FeatureJobStatus = "failed"
	FeatureJobCancelled  FeatureJobStatus = "cancelled"
)

// FeatureJob represents a feature calculation job for a symbol/timeframe
type FeatureJob struct {
	ID              string           `json:"id"`
	Symbol          string           `json:"symbol"`
	Timeframe       Timeframe        `json:"timeframe"`
	Status          FeatureJobStatus `json:"status"`
	TotalCandles    int              `json:"total_candles"`
	ProcessedCandles int             `json:"processed_candles"`
	CachedFeatures  int              `json:"cached_features"`
	Progress        float64          `json:"progress"`
	Error           string           `json:"error,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	StartedAt       *time.Time       `json:"started_at,omitempty"`
	CompletedAt     *time.Time       `json:"completed_at,omitempty"`
}

// FeatureJobConfig holds configuration for the feature job service
type FeatureJobConfig struct {
	// RecalculationInterval is how often to recalculate features for active coins
	RecalculationInterval time.Duration

	// BatchSize is the number of candles to process in each batch
	BatchSize int

	// MaxConcurrentJobs limits parallel feature calculation jobs
	MaxConcurrentJobs int

	// LookbackCandles is the number of candles needed for feature calculation
	LookbackCandles int

	// MaxJobHistory is the maximum number of completed jobs to retain (0 = unlimited)
	MaxJobHistory int

	// JobRetentionDuration is how long to keep completed jobs (0 = keep forever)
	JobRetentionDuration time.Duration
}

// DefaultFeatureJobConfig returns the default configuration
func DefaultFeatureJobConfig() FeatureJobConfig {
	return FeatureJobConfig{
		RecalculationInterval: 30 * time.Minute,  // Recalculate every 30 minutes
		BatchSize:             500,               // Process 500 candles at a time
		MaxConcurrentJobs:     2,                 // Max 2 parallel jobs
		LookbackCandles:       RecommendedLookback, // 100 candles for feature calculation
		MaxJobHistory:         100,               // Keep last 100 completed jobs
		JobRetentionDuration:  24 * time.Hour,    // Auto-cleanup jobs older than 24h
	}
}

// FeatureJobService manages background feature calculation jobs
type FeatureJobService struct {
	pool             *pgxpool.Pool
	featureCalc      *FeatureCalculator
	config           FeatureJobConfig

	// Running state
	mu               sync.RWMutex
	running          bool
	stopCh           chan struct{}
	lastRun          time.Time
	lastError        error

	// Job tracking
	jobs             map[string]*FeatureJob
	activeJobs       map[string]context.CancelFunc
	jobIDCounter     int

	// Semaphore for concurrency control
	semaphore        chan struct{}

	// Tracked symbol/timeframe combinations for periodic recalculation
	trackedPairs     map[string]bool // key: "SYMBOL:TIMEFRAME"
	trackedPairsMu   sync.RWMutex
}

// NewFeatureJobService creates a new feature job service
func NewFeatureJobService(
	pool *pgxpool.Pool,
	featureCalc *FeatureCalculator,
	config FeatureJobConfig,
) *FeatureJobService {
	if config.BatchSize <= 0 {
		config.BatchSize = 500
	}
	if config.MaxConcurrentJobs <= 0 {
		config.MaxConcurrentJobs = 2
	}
	if config.LookbackCandles <= 0 {
		config.LookbackCandles = RecommendedLookback
	}
	if config.RecalculationInterval <= 0 {
		config.RecalculationInterval = 30 * time.Minute
	}

	return &FeatureJobService{
		pool:         pool,
		featureCalc:  featureCalc,
		config:       config,
		stopCh:       make(chan struct{}),
		jobs:         make(map[string]*FeatureJob),
		activeJobs:   make(map[string]context.CancelFunc),
		semaphore:    make(chan struct{}, config.MaxConcurrentJobs),
		trackedPairs: make(map[string]bool),
	}
}

// Start begins the feature job service background loop
func (s *FeatureJobService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	log.Printf("[FEATURE_JOB] Starting service (recalc_interval=%v, batch_size=%d, max_concurrent=%d)",
		s.config.RecalculationInterval, s.config.BatchSize, s.config.MaxConcurrentJobs)

	// Start periodic recalculation loop
	go s.runRecalculationLoop(ctx)

	return nil
}

// Stop halts the feature job service
func (s *FeatureJobService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	// Cancel all active jobs
	for jobID, cancel := range s.activeJobs {
		log.Printf("[FEATURE_JOB] Cancelling job %s", jobID)
		cancel()
	}

	close(s.stopCh)
	s.running = false
	log.Printf("[FEATURE_JOB] Service stopped")
}

// IsRunning returns whether the service is currently running
func (s *FeatureJobService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// TrackPair adds a symbol/timeframe pair to the periodic recalculation list
func (s *FeatureJobService) TrackPair(symbol string, timeframe Timeframe) {
	key := fmt.Sprintf("%s:%s", symbol, timeframe)
	s.trackedPairsMu.Lock()
	s.trackedPairs[key] = true
	s.trackedPairsMu.Unlock()
	log.Printf("[FEATURE_JOB] Tracking pair: %s", key)
}

// UntrackPair removes a symbol/timeframe pair from periodic recalculation
func (s *FeatureJobService) UntrackPair(symbol string, timeframe Timeframe) {
	key := fmt.Sprintf("%s:%s", symbol, timeframe)
	s.trackedPairsMu.Lock()
	delete(s.trackedPairs, key)
	s.trackedPairsMu.Unlock()
	log.Printf("[FEATURE_JOB] Untracked pair: %s", key)
}

// GetTrackedPairs returns all tracked symbol/timeframe pairs
func (s *FeatureJobService) GetTrackedPairs() []string {
	s.trackedPairsMu.RLock()
	defer s.trackedPairsMu.RUnlock()

	pairs := make([]string, 0, len(s.trackedPairs))
	for pair := range s.trackedPairs {
		pairs = append(pairs, pair)
	}
	return pairs
}

// TriggerFeatureCalculation starts a feature calculation job for a symbol/timeframe
// This is called by DataDownloader when a download completes
func (s *FeatureJobService) TriggerFeatureCalculation(ctx context.Context, symbol string, timeframe Timeframe) (*FeatureJob, error) {
	jobID := s.generateJobID(symbol, timeframe)

	// Create new job
	now := time.Now()
	job := &FeatureJob{
		ID:        jobID,
		Symbol:    symbol,
		Timeframe: timeframe,
		Status:    FeatureJobPending,
		CreatedAt: now,
	}

	// Atomically check for existing job and register new one
	s.mu.Lock()
	if _, exists := s.activeJobs[jobID]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("feature calculation already running for %s/%s", symbol, timeframe)
	}
	// Register job while still holding lock to prevent race condition
	s.jobs[jobID] = job
	s.mu.Unlock()

	// Track this pair for future recalculations
	s.TrackPair(symbol, timeframe)

	// Start calculation in background
	go s.executeFeatureCalculation(job)

	// Return a copy to prevent race conditions with background updates
	jobCopy := *job
	return &jobCopy, nil
}

// GetJob retrieves a job by ID
// Returns a copy of the job to prevent data races with background updates
func (s *FeatureJobService) GetJob(jobID string) *FeatureJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.jobs[jobID]
	if !exists {
		return nil
	}
	// Return a copy to prevent data races
	jobCopy := *job
	return &jobCopy
}

// ListJobs returns all tracked jobs
// Returns copies of jobs to prevent data races with background updates
func (s *FeatureJobService) ListJobs() []*FeatureJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*FeatureJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		// Return copies to prevent data races
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	return jobs
}

// CancelJob cancels a running feature calculation job
func (s *FeatureJobService) CancelJob(jobID string) error {
	s.mu.Lock()
	cancel, exists := s.activeJobs[jobID]
	s.mu.Unlock()

	if !exists {
		return fmt.Errorf("job not found or not running: %s", jobID)
	}

	cancel()
	return nil
}

// runRecalculationLoop periodically recalculates features for tracked pairs
func (s *FeatureJobService) runRecalculationLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.RecalculationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[FEATURE_JOB] Context cancelled, stopping recalculation loop")
			return
		case <-s.stopCh:
			log.Printf("[FEATURE_JOB] Stop signal received")
			return
		case <-ticker.C:
			s.recalculateTrackedPairs(ctx)
		}
	}
}

// recalculateTrackedPairs triggers feature calculation for all tracked pairs
func (s *FeatureJobService) recalculateTrackedPairs(ctx context.Context) {
	s.trackedPairsMu.RLock()
	pairs := make([]string, 0, len(s.trackedPairs))
	for pair := range s.trackedPairs {
		pairs = append(pairs, pair)
	}
	s.trackedPairsMu.RUnlock()

	if len(pairs) == 0 {
		return
	}

	log.Printf("[FEATURE_JOB] Starting periodic recalculation for %d pairs", len(pairs))

	for _, pair := range pairs {
		// Parse symbol and timeframe from pair key
		symbol, timeframe := parsePairKey(pair)
		if symbol == "" || !timeframe.IsValid() {
			continue
		}

		// Check for cancellation
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}

		// Trigger calculation (non-blocking, will queue if at capacity)
		if _, err := s.TriggerFeatureCalculation(ctx, symbol, timeframe); err != nil {
			// Log but continue - might already be running
			log.Printf("[FEATURE_JOB] Could not trigger recalculation for %s: %v", pair, err)
		}
	}

	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()
}

// executeFeatureCalculation performs the actual feature calculation
func (s *FeatureJobService) executeFeatureCalculation(job *FeatureJob) {
	// Acquire semaphore
	select {
	case s.semaphore <- struct{}{}:
		// Got the slot
	case <-time.After(30 * time.Second):
		job.Status = FeatureJobFailed
		job.Error = "timeout waiting for calculation slot"
		log.Printf("[FEATURE_JOB] Job %s failed: timeout waiting for slot", job.ID)
		return
	}
	defer func() { <-s.semaphore }()

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register active job
	s.mu.Lock()
	s.activeJobs[job.ID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeJobs, job.ID)
		s.mu.Unlock()
	}()

	// Update status to running
	now := time.Now()
	job.Status = FeatureJobRunning
	job.StartedAt = &now

	log.Printf("[FEATURE_JOB] Starting feature calculation for %s/%s", job.Symbol, job.Timeframe)

	// Get total candle count
	totalCandles, err := s.getCandleCount(ctx, job.Symbol, job.Timeframe)
	if err != nil {
		job.Status = FeatureJobFailed
		job.Error = fmt.Sprintf("failed to get candle count: %v", err)
		log.Printf("[FEATURE_JOB] Job %s failed: %v", job.ID, err)
		return
	}

	if totalCandles < MinimumLookback {
		job.Status = FeatureJobFailed
		job.Error = fmt.Sprintf("insufficient candles: have %d, need at least %d", totalCandles, MinimumLookback)
		log.Printf("[FEATURE_JOB] Job %s failed: %s", job.ID, job.Error)
		return
	}

	job.TotalCandles = totalCandles
	log.Printf("[FEATURE_JOB] Processing %d candles for %s/%s", totalCandles, job.Symbol, job.Timeframe)

	// Process candles in batches
	offset := 0
	processedTotal := 0
	cachedTotal := 0

	for offset < totalCandles {
		// Check for cancellation
		select {
		case <-ctx.Done():
			job.Status = FeatureJobCancelled
			log.Printf("[FEATURE_JOB] Job %s cancelled", job.ID)
			return
		default:
		}

		// Calculate batch size including lookback
		batchSize := s.config.BatchSize
		if offset+batchSize > totalCandles {
			batchSize = totalCandles - offset
		}

		// We need extra candles for lookback at the start
		fetchStart := offset
		if offset > 0 {
			fetchStart = offset - s.config.LookbackCandles
			if fetchStart < 0 {
				fetchStart = 0
			}
		}
		fetchLimit := batchSize + (offset - fetchStart)

		// Fetch candles
		candles, err := s.getCandles(ctx, job.Symbol, job.Timeframe, fetchStart, fetchLimit)
		if err != nil {
			job.Status = FeatureJobFailed
			job.Error = fmt.Sprintf("failed to fetch candles: %v", err)
			log.Printf("[FEATURE_JOB] Job %s failed: %v", job.ID, err)
			return
		}

		if len(candles) == 0 {
			break
		}

		// Calculate target count (candles to process, excluding lookback)
		targetCount := len(candles)
		if offset == 0 {
			// First batch: process all candles that have enough lookback
			targetCount = len(candles) - MinimumLookback + 1
			if targetCount <= 0 {
				offset += batchSize
				continue
			}
		} else {
			// Subsequent batches: process new candles (batchSize)
			targetCount = batchSize
		}

		// Calculate and cache features
		features, err := s.featureCalc.CalculateBatchAndCache(ctx, candles, targetCount)
		if err != nil {
			log.Printf("[FEATURE_JOB] Warning: batch calculation error at offset %d: %v", offset, err)
			// Continue with next batch instead of failing completely
		} else {
			cachedCount := 0
			for _, f := range features {
				if f != nil {
					cachedCount++
				}
			}
			cachedTotal += cachedCount
		}

		processedTotal += targetCount
		offset += batchSize

		// Update progress
		job.ProcessedCandles = processedTotal
		job.CachedFeatures = cachedTotal
		if job.TotalCandles > 0 {
			job.Progress = float64(processedTotal) / float64(job.TotalCandles-MinimumLookback+1) * 100
			if job.Progress > 100 {
				job.Progress = 100
			}
		}

		// Log progress every 5000 candles
		if processedTotal%5000 < batchSize {
			log.Printf("[FEATURE_JOB] %s: %.1f%% complete (%d/%d candles, %d features cached)",
				job.ID, job.Progress, processedTotal, totalCandles, cachedTotal)
		}
	}

	// Mark as completed
	completedAt := time.Now()
	job.Status = FeatureJobCompleted
	job.CompletedAt = &completedAt
	job.Progress = 100

	elapsed := completedAt.Sub(*job.StartedAt)
	log.Printf("[FEATURE_JOB] Completed %s: %d candles processed, %d features cached (took %v)",
		job.ID, processedTotal, cachedTotal, elapsed.Round(time.Millisecond))

	// Trigger automatic cleanup to prevent memory leaks
	s.autoCleanupJobs()
}

// autoCleanupJobs removes old completed jobs to prevent memory leaks
func (s *FeatureJobService) autoCleanupJobs() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Skip if no limits configured
	if s.config.MaxJobHistory <= 0 && s.config.JobRetentionDuration <= 0 {
		return
	}

	// Collect completed jobs with their completion times
	type jobInfo struct {
		id          string
		completedAt time.Time
	}
	var completedJobs []jobInfo

	now := time.Now()
	for id, job := range s.jobs {
		if job.Status == FeatureJobCompleted || job.Status == FeatureJobFailed || job.Status == FeatureJobCancelled {
			if job.CompletedAt != nil {
				completedJobs = append(completedJobs, jobInfo{id: id, completedAt: *job.CompletedAt})
			}
		}
	}

	// Remove jobs older than retention duration
	if s.config.JobRetentionDuration > 0 {
		cutoff := now.Add(-s.config.JobRetentionDuration)
		for _, j := range completedJobs {
			if j.completedAt.Before(cutoff) {
				delete(s.jobs, j.id)
			}
		}
	}

	// Enforce max job history limit
	if s.config.MaxJobHistory > 0 && len(s.jobs) > s.config.MaxJobHistory {
		// Re-collect completed jobs after time-based cleanup
		completedJobs = completedJobs[:0]
		for id, job := range s.jobs {
			if job.Status == FeatureJobCompleted || job.Status == FeatureJobFailed || job.Status == FeatureJobCancelled {
				if job.CompletedAt != nil {
					completedJobs = append(completedJobs, jobInfo{id: id, completedAt: *job.CompletedAt})
				}
			}
		}

		// Sort by completion time (oldest first) and remove excess
		if len(completedJobs) > 0 {
			// Simple bubble sort for small lists - could use sort.Slice for larger lists
			for i := 0; i < len(completedJobs)-1; i++ {
				for j := i + 1; j < len(completedJobs); j++ {
					if completedJobs[i].completedAt.After(completedJobs[j].completedAt) {
						completedJobs[i], completedJobs[j] = completedJobs[j], completedJobs[i]
					}
				}
			}

			// Remove oldest jobs to get under the limit
			toRemove := len(s.jobs) - s.config.MaxJobHistory
			for i := 0; i < toRemove && i < len(completedJobs); i++ {
				delete(s.jobs, completedJobs[i].id)
			}
		}
	}
}

// getCandleCount returns the total number of candles for a symbol/timeframe
func (s *FeatureJobService) getCandleCount(ctx context.Context, symbol string, timeframe Timeframe) (int, error) {
	if s.pool == nil {
		return 0, fmt.Errorf("database pool not available")
	}

	query := `SELECT COUNT(*) FROM research_candles WHERE symbol = $1 AND timeframe = $2`
	var count int
	if err := s.pool.QueryRow(ctx, query, symbol, string(timeframe)).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// getCandles fetches candles from the database with pagination
func (s *FeatureJobService) getCandles(ctx context.Context, symbol string, timeframe Timeframe, offset, limit int) ([]*ResearchCandle, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("database pool not available")
	}

	query := `
		SELECT
			id, symbol, timeframe, open_time, close_time,
			open, high, low, close,
			volume, quote_volume, trade_count,
			taker_buy_volume, taker_buy_quote, created_at
		FROM research_candles
		WHERE symbol = $1 AND timeframe = $2
		ORDER BY open_time ASC
		LIMIT $3 OFFSET $4
	`

	rows, err := s.pool.Query(ctx, query, symbol, string(timeframe), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candles []*ResearchCandle
	for rows.Next() {
		c := &ResearchCandle{}
		var tf string
		err := rows.Scan(
			&c.ID, &c.Symbol, &tf, &c.OpenTime, &c.CloseTime,
			&c.Open, &c.High, &c.Low, &c.Close,
			&c.Volume, &c.QuoteVolume, &c.TradeCount,
			&c.TakerBuyVolume, &c.TakerBuyQuote, &c.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		c.Timeframe = Timeframe(tf)
		candles = append(candles, c)
	}

	return candles, rows.Err()
}

// generateJobID creates a unique job ID
func (s *FeatureJobService) generateJobID(symbol string, timeframe Timeframe) string {
	s.mu.Lock()
	s.jobIDCounter++
	counter := s.jobIDCounter
	s.mu.Unlock()
	return fmt.Sprintf("feat_%s_%s_%d_%d", symbol, timeframe, time.Now().Unix(), counter)
}

// parsePairKey parses a "SYMBOL:TIMEFRAME" key into components
func parsePairKey(key string) (string, Timeframe) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return key[:i], Timeframe(key[i+1:])
		}
	}
	return "", ""
}

// FeatureJobStats provides statistics about the feature job service
type FeatureJobStats struct {
	Running               bool      `json:"running"`
	LastRun               time.Time `json:"last_run"`
	LastError             string    `json:"last_error,omitempty"`
	RecalculationInterval string    `json:"recalculation_interval"`
	BatchSize             int       `json:"batch_size"`
	MaxConcurrentJobs     int       `json:"max_concurrent_jobs"`
	TrackedPairsCount     int       `json:"tracked_pairs_count"`
	ActiveJobsCount       int       `json:"active_jobs_count"`
	TotalJobsCount        int       `json:"total_jobs_count"`
}

// GetStats returns current service statistics
func (s *FeatureJobService) GetStats() *FeatureJobStats {
	s.mu.RLock()
	running := s.running
	lastRun := s.lastRun
	lastError := s.lastError
	activeCount := len(s.activeJobs)
	totalCount := len(s.jobs)
	s.mu.RUnlock()

	s.trackedPairsMu.RLock()
	trackedCount := len(s.trackedPairs)
	s.trackedPairsMu.RUnlock()

	stats := &FeatureJobStats{
		Running:               running,
		LastRun:               lastRun,
		RecalculationInterval: s.config.RecalculationInterval.String(),
		BatchSize:             s.config.BatchSize,
		MaxConcurrentJobs:     s.config.MaxConcurrentJobs,
		TrackedPairsCount:     trackedCount,
		ActiveJobsCount:       activeCount,
		TotalJobsCount:        totalCount,
	}

	if lastError != nil {
		stats.LastError = lastError.Error()
	}

	return stats
}

// GetLastRun returns the time of the last recalculation run
func (s *FeatureJobService) GetLastRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRun
}

// GetLastError returns the last error encountered
func (s *FeatureJobService) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// CleanupOldJobs removes completed jobs older than the specified duration
func (s *FeatureJobService) CleanupOldJobs(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for id, job := range s.jobs {
		// Don't remove active jobs
		if job.Status == FeatureJobRunning || job.Status == FeatureJobPending {
			continue
		}
		if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
			delete(s.jobs, id)
			removed++
		}
	}

	if removed > 0 {
		log.Printf("[FEATURE_JOB] Cleaned up %d old jobs", removed)
	}

	return removed
}
