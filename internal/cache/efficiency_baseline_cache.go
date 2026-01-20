// Package cache provides Redis-based caching for efficiency baselines.
// Epic 10 / Story 10.1: Position Management Optimization - Historical Baseline
//
// This file implements Redis storage for historical efficiency baselines.
// The baseline is the average exit efficiency from recent closed trades,
// used as the threshold for efficiency-based exits.
//
// Key pattern: baseline:{user_id}:{mode}
// Example: baseline:user123:scalp
//
// The baseline is refreshed hourly from closed trades in the last 4-8 hours.
// Default threshold is 50% if no historical data exists.
package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/types"
)

// Redis key prefix for efficiency baselines
const (
	// PrefixEfficiencyBaseline is the prefix for baseline cache keys
	// Format: baseline:{user_id}:{mode}
	PrefixEfficiencyBaseline = "baseline:%s:%s"

	// BaselineTTL is the TTL for baseline cache entries
	// Set to 2 hours - baselines are refreshed hourly, so this provides buffer
	BaselineTTL = 2 * time.Hour

	// Re-export constants from types package for backwards compatibility
	DefaultEfficiencyThreshold = types.DefaultEfficiencyThreshold
	DefaultBaselineWindowHours = types.DefaultBaselineWindowHours
	MinTradesForBaseline       = types.MinTradesForBaseline
)

// HistoricalBaseline is an alias to types.HistoricalBaseline for backwards compatibility.
type HistoricalBaseline = types.HistoricalBaseline

// EfficiencyBaselineCache provides Redis-based storage for efficiency baselines.
type EfficiencyBaselineCache struct {
	cache *CacheService
}

// NewEfficiencyBaselineCache creates a new EfficiencyBaselineCache.
func NewEfficiencyBaselineCache(cache *CacheService) *EfficiencyBaselineCache {
	return &EfficiencyBaselineCache{
		cache: cache,
	}
}

// baselineKey generates the Redis key for a baseline.
func (c *EfficiencyBaselineCache) baselineKey(userID, mode string) string {
	return fmt.Sprintf(PrefixEfficiencyBaseline, userID, mode)
}

// GetBaseline retrieves the efficiency baseline for a user/mode.
// Returns default baseline if not found or on error.
func (c *EfficiencyBaselineCache) GetBaseline(ctx context.Context, userID, mode string) (*HistoricalBaseline, error) {
	if c.cache == nil {
		return c.getDefaultBaseline(mode), nil
	}

	key := c.baselineKey(userID, mode)

	var baseline HistoricalBaseline
	err := c.cache.GetJSON(ctx, key, &baseline)
	if err != nil {
		// Cache miss or error - return default
		log.Printf("[EFFICIENCY-BASELINE] Cache miss for %s/%s, using default threshold %.2f",
			userID, mode, DefaultEfficiencyThreshold)
		return c.getDefaultBaseline(mode), nil
	}

	// Validate the baseline is still fresh (within 2x window)
	age := time.Since(time.Unix(baseline.LastUpdated, 0))
	maxAge := time.Duration(baseline.WindowHours*2) * time.Hour
	if age > maxAge {
		log.Printf("[EFFICIENCY-BASELINE] Stale baseline for %s/%s (age=%v), using default",
			userID, mode, age.Round(time.Minute))
		return c.getDefaultBaseline(mode), nil
	}

	log.Printf("[EFFICIENCY-BASELINE] Retrieved baseline for %s/%s: threshold=%.2f%%, trades=%d",
		userID, mode, baseline.AvgExitEfficiency*100, baseline.TradeCount)

	return &baseline, nil
}

// SaveBaseline saves an efficiency baseline to Redis.
func (c *EfficiencyBaselineCache) SaveBaseline(ctx context.Context, userID, mode string, baseline *HistoricalBaseline) error {
	if c.cache == nil {
		return fmt.Errorf("cache service not available")
	}

	if baseline == nil {
		return fmt.Errorf("cannot save nil baseline")
	}

	// Ensure LastUpdated is set
	if baseline.LastUpdated == 0 {
		baseline.LastUpdated = time.Now().Unix()
	}

	key := c.baselineKey(userID, mode)

	err := c.cache.SetJSON(ctx, key, baseline, BaselineTTL)
	if err != nil {
		log.Printf("[EFFICIENCY-BASELINE] Failed to save baseline for %s/%s: %v", userID, mode, err)
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	log.Printf("[EFFICIENCY-BASELINE] Saved baseline for %s/%s: threshold=%.2f%%, trades=%d, window=%dh",
		userID, mode, baseline.AvgExitEfficiency*100, baseline.TradeCount, baseline.WindowHours)

	return nil
}

// DeleteBaseline removes a baseline from Redis.
func (c *EfficiencyBaselineCache) DeleteBaseline(ctx context.Context, userID, mode string) error {
	if c.cache == nil {
		return nil
	}

	key := c.baselineKey(userID, mode)
	return c.cache.Delete(ctx, key)
}

// GetOrCreateBaseline retrieves a baseline or creates a default if none exists.
func (c *EfficiencyBaselineCache) GetOrCreateBaseline(ctx context.Context, userID, mode string) *HistoricalBaseline {
	baseline, _ := c.GetBaseline(ctx, userID, mode)
	return baseline
}

// getDefaultBaseline returns the default baseline for a mode.
func (c *EfficiencyBaselineCache) getDefaultBaseline(mode string) *HistoricalBaseline {
	return &HistoricalBaseline{
		Mode:              mode,
		AvgExitEfficiency: DefaultEfficiencyThreshold,
		TradeCount:        0,
		WindowHours:       DefaultBaselineWindowHours,
		LastUpdated:       time.Now().Unix(),
	}
}

// GetAllBaselines retrieves all baselines for a user across all modes.
func (c *EfficiencyBaselineCache) GetAllBaselines(ctx context.Context, userID string) (map[string]*HistoricalBaseline, error) {
	modes := []string{"scalp", "swing", "position", "ultra_fast"}
	baselines := make(map[string]*HistoricalBaseline)

	for _, mode := range modes {
		baseline, _ := c.GetBaseline(ctx, userID, mode)
		baselines[mode] = baseline
	}

	return baselines, nil
}

// BaselineCalculationInput holds data needed to calculate a new baseline.
type BaselineCalculationInput struct {
	// ExitEfficiencies is a slice of exit efficiency values from closed trades
	ExitEfficiencies []float64

	// Mode is the trading mode
	Mode string

	// WindowHours is the lookback window used
	WindowHours int
}

// CalculateBaseline calculates a new baseline from trade data.
// This is a pure calculation function - it doesn't interact with Redis.
func CalculateBaseline(input *BaselineCalculationInput) *HistoricalBaseline {
	if input == nil || len(input.ExitEfficiencies) == 0 {
		return &HistoricalBaseline{
			Mode:              input.Mode,
			AvgExitEfficiency: DefaultEfficiencyThreshold,
			TradeCount:        0,
			WindowHours:       input.WindowHours,
			LastUpdated:       time.Now().Unix(),
		}
	}

	efficiencies := input.ExitEfficiencies
	n := len(efficiencies)

	// If too few trades, use default with a blend
	if n < MinTradesForBaseline {
		// Blend: weight default more heavily when few trades
		weight := float64(n) / float64(MinTradesForBaseline)
		sum := 0.0
		for _, e := range efficiencies {
			sum += e
		}
		tradeAvg := sum / float64(n)
		blended := (tradeAvg * weight) + (DefaultEfficiencyThreshold * (1 - weight))

		return &HistoricalBaseline{
			Mode:              input.Mode,
			AvgExitEfficiency: blended,
			TradeCount:        n,
			WindowHours:       input.WindowHours,
			LastUpdated:       time.Now().Unix(),
		}
	}

	// Calculate statistics
	sum := 0.0
	minEff := efficiencies[0]
	maxEff := efficiencies[0]

	for _, e := range efficiencies {
		sum += e
		if e < minEff {
			minEff = e
		}
		if e > maxEff {
			maxEff = e
		}
	}

	avg := sum / float64(n)

	// Calculate standard deviation
	varianceSum := 0.0
	for _, e := range efficiencies {
		diff := e - avg
		varianceSum += diff * diff
	}
	stdDev := 0.0
	if n > 1 {
		stdDev = varianceSum / float64(n-1)
		if stdDev > 0 {
			stdDev = sqrt(stdDev)
		}
	}

	return &HistoricalBaseline{
		Mode:              input.Mode,
		AvgExitEfficiency: avg,
		TradeCount:        n,
		WindowHours:       input.WindowHours,
		LastUpdated:       time.Now().Unix(),
		MinEfficiency:     minEff,
		MaxEfficiency:     maxEff,
		StdDeviation:      stdDev,
	}
}

// sqrt calculates square root using Newton's method.
// Avoiding math import for a simple function.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2
	for i := 0; i < 10; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}

// BaselineStats returns statistics about the baseline cache.
type BaselineStats struct {
	UserID      string                      `json:"user_id"`
	Baselines   map[string]*HistoricalBaseline `json:"baselines"`
	TotalTrades int                         `json:"total_trades"`
	AvgThreshold float64                    `json:"avg_threshold"`
}

// GetBaselineStats retrieves statistics about all baselines for a user.
func (c *EfficiencyBaselineCache) GetBaselineStats(ctx context.Context, userID string) (*BaselineStats, error) {
	baselines, err := c.GetAllBaselines(ctx, userID)
	if err != nil {
		return nil, err
	}

	totalTrades := 0
	sumThreshold := 0.0
	count := 0

	for _, b := range baselines {
		totalTrades += b.TradeCount
		if b.TradeCount > 0 {
			sumThreshold += b.AvgExitEfficiency
			count++
		}
	}

	avgThreshold := DefaultEfficiencyThreshold
	if count > 0 {
		avgThreshold = sumThreshold / float64(count)
	}

	return &BaselineStats{
		UserID:       userID,
		Baselines:    baselines,
		TotalTrades:  totalTrades,
		AvgThreshold: avgThreshold,
	}, nil
}

// Note: MarshalJSON was removed because HistoricalBaseline is now a type alias
// to types.HistoricalBaseline, and Go doesn't allow adding methods to type aliases.
// The avg_eff_percent can be calculated by the frontend as avg_eff * 100.

// EfficiencyBaselineKey generates the cache key for a baseline.
// Exported for use in other packages.
func EfficiencyBaselineKey(userID, mode string) string {
	return fmt.Sprintf(PrefixEfficiencyBaseline, userID, mode)
}
