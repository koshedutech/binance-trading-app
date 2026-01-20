// Package autopilot provides the baseline refresh service for efficiency-based exits.
// Epic 10 / Story 10.1: Position Management Optimization - Historical Baseline
//
// This service runs hourly to refresh the efficiency baseline threshold for each user/mode.
// The baseline is calculated from the average exit efficiency of closed trades in the last 4-8 hours.
//
// How it works:
// 1. Query closed trades from the last N hours (configurable, default 6)
// 2. Calculate average exit efficiency
// 3. Store in Redis for fast access during position monitoring
// 4. Use default threshold (50%) if insufficient historical data
//
// The baseline is used by the efficiency exit logic to determine when to exit:
//   EXIT when efficiency < baseline.AvgExitEfficiency
package autopilot

import (
	"context"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/types"
)

// EfficiencyBaselineCacheInterface defines the interface for baseline cache operations.
// This breaks the import cycle between autopilot and cache packages.
type EfficiencyBaselineCacheInterface interface {
	// GetBaseline retrieves the efficiency baseline for a user/mode
	GetBaseline(ctx context.Context, userID, mode string) (*types.HistoricalBaseline, error)
	// SaveBaseline saves a baseline to the cache
	SaveBaseline(ctx context.Context, userID, mode string, baseline *types.HistoricalBaseline) error
}

// BaselineRefreshConfig holds configuration for the baseline refresh service.
type BaselineRefreshConfig struct {
	// RefreshInterval is how often to refresh baselines (default: 1 hour)
	RefreshInterval time.Duration

	// WindowHours is the lookback window for trades (default: 6 hours)
	WindowHours int

	// MinTrades is minimum trades needed for a valid baseline
	MinTrades int

	// DefaultThreshold is used when insufficient data
	DefaultThreshold float64
}

// DefaultBaselineRefreshConfig returns the default configuration.
func DefaultBaselineRefreshConfig() *BaselineRefreshConfig {
	return &BaselineRefreshConfig{
		RefreshInterval:  1 * time.Hour,
		WindowHours:      types.DefaultBaselineWindowHours,
		MinTrades:        types.MinTradesForBaseline,
		DefaultThreshold: types.DefaultEfficiencyThreshold,
	}
}

// BaselineRefreshService manages hourly baseline refresh for all users/modes.
type BaselineRefreshService struct {
	db            *database.DB
	baselineCache EfficiencyBaselineCacheInterface
	config        *BaselineRefreshConfig

	// Running state
	mu        sync.RWMutex
	running   bool
	stopCh    chan struct{}
	lastRun   time.Time
	lastError error
}

// NewBaselineRefreshService creates a new baseline refresh service.
func NewBaselineRefreshService(
	db *database.DB,
	baselineCache EfficiencyBaselineCacheInterface,
	config *BaselineRefreshConfig,
) *BaselineRefreshService {
	if config == nil {
		config = DefaultBaselineRefreshConfig()
	}

	return &BaselineRefreshService{
		db:            db,
		baselineCache: baselineCache,
		config:        config,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the baseline refresh service.
// This runs in a goroutine and refreshes baselines periodically.
func (s *BaselineRefreshService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	log.Printf("[BASELINE-REFRESH] Starting service (interval=%v, window=%dh)",
		s.config.RefreshInterval, s.config.WindowHours)

	// Run initial refresh
	s.refreshAllBaselines(ctx)

	// Start periodic refresh
	go s.runLoop(ctx)

	return nil
}

// Stop halts the baseline refresh service.
func (s *BaselineRefreshService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false
	log.Printf("[BASELINE-REFRESH] Service stopped")
}

// IsRunning returns whether the service is currently running.
func (s *BaselineRefreshService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetLastRun returns the time of the last successful refresh.
func (s *BaselineRefreshService) GetLastRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRun
}

// GetLastError returns the last error encountered during refresh.
func (s *BaselineRefreshService) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// runLoop is the main loop that periodically refreshes baselines.
func (s *BaselineRefreshService) runLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Printf("[BASELINE-REFRESH] Context cancelled, stopping")
			return
		case <-s.stopCh:
			log.Printf("[BASELINE-REFRESH] Stop signal received")
			return
		case <-ticker.C:
			s.refreshAllBaselines(ctx)
		}
	}
}

// refreshAllBaselines refreshes baselines for all active users.
func (s *BaselineRefreshService) refreshAllBaselines(ctx context.Context) {
	if s.db == nil {
		log.Printf("[BASELINE-REFRESH] Database not available, skipping refresh")
		return
	}

	start := time.Now()
	log.Printf("[BASELINE-REFRESH] Starting baseline refresh")

	// Get list of active users (users with recent trades)
	userIDs, err := s.getActiveUsers(ctx)
	if err != nil {
		s.mu.Lock()
		s.lastError = err
		s.mu.Unlock()
		log.Printf("[BASELINE-REFRESH] Failed to get active users: %v", err)
		return
	}

	if len(userIDs) == 0 {
		log.Printf("[BASELINE-REFRESH] No active users found")
		s.mu.Lock()
		s.lastRun = time.Now()
		s.lastError = nil
		s.mu.Unlock()
		return
	}

	// Refresh baselines for each user
	modes := []string{"scalp", "swing", "position", "ultra_fast"}
	refreshed := 0
	errors := 0

	for _, userID := range userIDs {
		for _, mode := range modes {
			if err := s.refreshBaseline(ctx, userID, mode); err != nil {
				log.Printf("[BASELINE-REFRESH] Error refreshing %s/%s: %v", userID, mode, err)
				errors++
			} else {
				refreshed++
			}
		}
	}

	elapsed := time.Since(start)
	log.Printf("[BASELINE-REFRESH] Completed: %d baselines refreshed, %d errors (took %v)",
		refreshed, errors, elapsed.Round(time.Millisecond))

	s.mu.Lock()
	s.lastRun = time.Now()
	if errors > 0 {
		s.lastError = log.New(nil, "", 0).Output(0, "some baselines failed to refresh")
	} else {
		s.lastError = nil
	}
	s.mu.Unlock()
}

// getActiveUsers returns user IDs that have traded in the baseline window.
func (s *BaselineRefreshService) getActiveUsers(ctx context.Context) ([]string, error) {
	if s.db == nil || s.db.Pool == nil {
		return nil, nil
	}

	query := `
		SELECT DISTINCT user_id
		FROM trade_efficiency_metrics
		WHERE created_at >= NOW() - INTERVAL '1 hour' * $1`

	rows, err := s.db.Pool.Query(ctx, query, s.config.WindowHours*2) // 2x window to catch users
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			continue
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, rows.Err()
}

// refreshBaseline refreshes the baseline for a specific user/mode combination.
func (s *BaselineRefreshService) refreshBaseline(ctx context.Context, userID, mode string) error {
	// Query average exit efficiency from database
	avgEfficiency, tradeCount, err := s.db.GetAverageExitEfficiency(ctx, userID, mode, s.config.WindowHours)
	if err != nil {
		return err
	}

	// Calculate baseline
	baseline := s.calculateBaseline(avgEfficiency, tradeCount, mode)

	// Save to Redis
	if s.baselineCache != nil {
		return s.baselineCache.SaveBaseline(ctx, userID, mode, baseline)
	}

	return nil
}

// calculateBaseline creates a types.HistoricalBaseline from the calculated data.
func (s *BaselineRefreshService) calculateBaseline(avgEfficiency float64, tradeCount int, mode string) *types.HistoricalBaseline {
	// If insufficient trades, blend with default
	if tradeCount < s.config.MinTrades {
		// Weight based on how many trades we have
		weight := float64(tradeCount) / float64(s.config.MinTrades)
		if weight > 0 && avgEfficiency > 0 {
			avgEfficiency = (avgEfficiency * weight) + (s.config.DefaultThreshold * (1 - weight))
		} else {
			avgEfficiency = s.config.DefaultThreshold
		}
	}

	return &types.HistoricalBaseline{
		Mode:              mode,
		AvgExitEfficiency: avgEfficiency,
		TradeCount:        tradeCount,
		WindowHours:       s.config.WindowHours,
		LastUpdated:       time.Now().Unix(),
	}
}

// RefreshBaseline manually triggers a baseline refresh for a specific user/mode.
// Useful for testing or when immediate refresh is needed.
func (s *BaselineRefreshService) RefreshBaseline(ctx context.Context, userID, mode string) error {
	return s.refreshBaseline(ctx, userID, mode)
}

// RefreshUserBaselines refreshes all baselines for a specific user.
func (s *BaselineRefreshService) RefreshUserBaselines(ctx context.Context, userID string) error {
	modes := []string{"scalp", "swing", "position", "ultra_fast"}
	for _, mode := range modes {
		if err := s.refreshBaseline(ctx, userID, mode); err != nil {
			log.Printf("[BASELINE-REFRESH] Error refreshing %s/%s: %v", userID, mode, err)
			// Continue with other modes even if one fails
		}
	}
	return nil
}

// GetBaselineStats returns statistics about the baseline refresh service.
type BaselineRefreshStats struct {
	Running         bool      `json:"running"`
	LastRun         time.Time `json:"last_run"`
	LastError       string    `json:"last_error,omitempty"`
	RefreshInterval string    `json:"refresh_interval"`
	WindowHours     int       `json:"window_hours"`
	MinTrades       int       `json:"min_trades"`
	DefaultThreshold float64  `json:"default_threshold"`
}

// GetStats returns current service statistics.
func (s *BaselineRefreshService) GetStats() *BaselineRefreshStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &BaselineRefreshStats{
		Running:          s.running,
		LastRun:          s.lastRun,
		RefreshInterval:  s.config.RefreshInterval.String(),
		WindowHours:      s.config.WindowHours,
		MinTrades:        s.config.MinTrades,
		DefaultThreshold: s.config.DefaultThreshold,
	}

	if s.lastError != nil {
		stats.LastError = s.lastError.Error()
	}

	return stats
}

// CalculateBaselineFromMetrics calculates a baseline from a slice of efficiency metrics.
// This is a helper function for manual baseline calculation.
func CalculateBaselineFromMetrics(metrics []database.TradeEfficiencyMetric, mode string, windowHours int) *types.HistoricalBaseline {
	if len(metrics) == 0 {
		return &types.HistoricalBaseline{
			Mode:              mode,
			AvgExitEfficiency: types.DefaultEfficiencyThreshold,
			TradeCount:        0,
			WindowHours:       windowHours,
			LastUpdated:       time.Now().Unix(),
		}
	}

	// Calculate average exit efficiency
	sum := 0.0
	for _, m := range metrics {
		sum += m.ExitEfficiency
	}
	avg := sum / float64(len(metrics))

	return &types.HistoricalBaseline{
		Mode:              mode,
		AvgExitEfficiency: avg,
		TradeCount:        len(metrics),
		WindowHours:       windowHours,
		LastUpdated:       time.Now().Unix(),
	}
}
