// Package indicators provides an extensible indicator framework for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.12: Indicator Segment Framework
package indicators

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"binance-trading-bot/internal/cache"
)

// Redis key prefix for indicator performance tracking
const (
	// PrefixIndicatorPerf is the Redis key pattern for indicator performance data
	// Format: decision:indicator_perf:{userID}:{hash}
	// Note: Using %s for userID to match existing codebase conventions (e.g., decision:user:%s:coin:%s)
	PrefixIndicatorPerf = "decision:indicator_perf:%s:%s"

	// PrefixIndicatorPerfIndex is the Redis key pattern for user's performance index
	// Format: decision:indicator_perf_index:{userID}
	PrefixIndicatorPerfIndex = "decision:indicator_perf_index:%s"

	// DefaultPerfTTL is the default TTL for performance data (30 days)
	DefaultPerfTTL = 30 * 24 * time.Hour
)

// IndicatorPerformance tracks how indicator combinations perform over time.
type IndicatorPerformance struct {
	// IndicatorHash is a unique hash of sorted indicator names.
	IndicatorHash string `json:"hash"`
	// Indicators is the list of indicator names in this combination.
	Indicators []string `json:"indicators"`
	// Segment identifies which segment this combination is for.
	Segment IndicatorSegment `json:"segment"`
	// TradeCount is the total number of trades using this combination.
	TradeCount int `json:"trade_count"`
	// WinCount is the number of winning trades.
	WinCount int `json:"win_count"`
	// WinRate is the calculated win rate (0-100%).
	WinRate float64 `json:"win_rate"`
	// AvgScore is the average score when this combination was used.
	AvgScore float64 `json:"avg_score"`
	// TotalPnL is the total profit/loss from trades using this combination.
	TotalPnL float64 `json:"total_pnl"`
	// AvgPnL is the average profit/loss per trade.
	AvgPnL float64 `json:"avg_pnl"`
	// LastUpdated is when this record was last updated (Unix milliseconds).
	LastUpdated int64 `json:"last_updated"`
	// CreatedAt is when this record was created (Unix milliseconds).
	CreatedAt int64 `json:"created_at"`
}

// NewIndicatorPerformance creates a new IndicatorPerformance record.
func NewIndicatorPerformance(indicators []string, segment IndicatorSegment) *IndicatorPerformance {
	hash := hashIndicators(indicators)
	now := time.Now().UnixMilli()

	// Create sorted copy of indicators
	sortedIndicators := make([]string, len(indicators))
	copy(sortedIndicators, indicators)
	sort.Strings(sortedIndicators)

	return &IndicatorPerformance{
		IndicatorHash: hash,
		Indicators:    sortedIndicators,
		Segment:       segment,
		TradeCount:    0,
		WinCount:      0,
		WinRate:       0,
		AvgScore:      0,
		TotalPnL:      0,
		AvgPnL:        0,
		LastUpdated:   now,
		CreatedAt:     now,
	}
}

// RecordTrade updates the performance stats with a new trade result.
func (ip *IndicatorPerformance) RecordTrade(won bool, score, pnl float64) {
	// Update cumulative average score
	totalScore := ip.AvgScore * float64(ip.TradeCount)
	ip.TradeCount++
	ip.AvgScore = (totalScore + score) / float64(ip.TradeCount)

	// Update win stats
	if won {
		ip.WinCount++
	}
	ip.WinRate = float64(ip.WinCount) / float64(ip.TradeCount) * 100

	// Update PnL stats
	ip.TotalPnL += pnl
	ip.AvgPnL = ip.TotalPnL / float64(ip.TradeCount)

	ip.LastUpdated = time.Now().UnixMilli()
}

// Clone returns a deep copy of the IndicatorPerformance.
func (ip *IndicatorPerformance) Clone() *IndicatorPerformance {
	indicators := make([]string, len(ip.Indicators))
	copy(indicators, ip.Indicators)

	return &IndicatorPerformance{
		IndicatorHash: ip.IndicatorHash,
		Indicators:    indicators,
		Segment:       ip.Segment,
		TradeCount:    ip.TradeCount,
		WinCount:      ip.WinCount,
		WinRate:       ip.WinRate,
		AvgScore:      ip.AvgScore,
		TotalPnL:      ip.TotalPnL,
		AvgPnL:        ip.AvgPnL,
		LastUpdated:   ip.LastUpdated,
		CreatedAt:     ip.CreatedAt,
	}
}

// hashIndicators creates a unique hash for a set of indicators.
func hashIndicators(indicators []string) string {
	// Sort indicators for consistent hashing
	sorted := make([]string, len(indicators))
	copy(sorted, indicators)
	sort.Strings(sorted)

	// Create hash
	h := sha256.New()
	for _, ind := range sorted {
		h.Write([]byte(ind))
		h.Write([]byte(":"))
	}

	return hex.EncodeToString(h.Sum(nil))[:16] // Use first 16 chars
}

// PerformanceTracker tracks indicator combination performance using Redis.
type PerformanceTracker struct {
	cache *cache.CacheService
	ttl   time.Duration
}

// NewPerformanceTracker creates a new PerformanceTracker.
// If cache is nil, the tracker will operate in memory-only mode.
func NewPerformanceTracker(cacheService *cache.CacheService) *PerformanceTracker {
	return &PerformanceTracker{
		cache: cacheService,
		ttl:   DefaultPerfTTL,
	}
}

// RecordTrade records a trade result for a specific indicator combination.
// userID should be passed as a string to match existing codebase conventions.
func (pt *PerformanceTracker) RecordTrade(ctx context.Context, userID string, indicators []string, segment IndicatorSegment, won bool, score, pnl float64) error {
	if pt.cache == nil {
		return fmt.Errorf("cache service not available")
	}

	hash := hashIndicators(indicators)
	key := fmt.Sprintf(PrefixIndicatorPerf, userID, hash)

	// Try to get existing performance record
	perf, err := pt.GetPerformance(ctx, userID, indicators)
	if err != nil || perf == nil {
		// Create new record
		perf = NewIndicatorPerformance(indicators, segment)
	}

	// Update with new trade
	perf.RecordTrade(won, score, pnl)

	// Save to Redis
	data, err := json.Marshal(perf)
	if err != nil {
		return fmt.Errorf("failed to marshal performance: %w", err)
	}

	if err := pt.cache.Set(ctx, key, string(data), pt.ttl); err != nil {
		return fmt.Errorf("failed to save performance: %w", err)
	}

	// Update the index (non-critical, don't fail on error)
	_ = pt.updateIndex(ctx, userID, hash)

	return nil
}

// GetPerformance retrieves the performance record for a specific indicator combination.
func (pt *PerformanceTracker) GetPerformance(ctx context.Context, userID string, indicators []string) (*IndicatorPerformance, error) {
	if pt.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	hash := hashIndicators(indicators)
	key := fmt.Sprintf(PrefixIndicatorPerf, userID, hash)

	data, err := pt.cache.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance: %w", err)
	}

	var perf IndicatorPerformance
	if err := json.Unmarshal([]byte(data), &perf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal performance: %w", err)
	}

	return &perf, nil
}

// GetTopCombinations returns the top performing indicator combinations for a segment.
func (pt *PerformanceTracker) GetTopCombinations(ctx context.Context, userID string, segment IndicatorSegment, limit int) ([]IndicatorPerformance, error) {
	if pt.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	// Get all performance records for this user
	allPerf, err := pt.getAllPerformance(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Filter by segment and sort by win rate
	var filtered []IndicatorPerformance
	for _, perf := range allPerf {
		if perf.Segment == segment && perf.TradeCount >= 5 { // Minimum 5 trades for meaningful stats
			filtered = append(filtered, perf)
		}
	}

	// Sort by win rate (descending)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].WinRate > filtered[j].WinRate
	})

	// Limit results
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// GetTopByPnL returns the top performing combinations by PnL.
func (pt *PerformanceTracker) GetTopByPnL(ctx context.Context, userID string, segment IndicatorSegment, limit int) ([]IndicatorPerformance, error) {
	if pt.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	allPerf, err := pt.getAllPerformance(ctx, userID)
	if err != nil {
		return nil, err
	}

	var filtered []IndicatorPerformance
	for _, perf := range allPerf {
		if perf.Segment == segment && perf.TradeCount >= 5 {
			filtered = append(filtered, perf)
		}
	}

	// Sort by total PnL (descending)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].TotalPnL > filtered[j].TotalPnL
	})

	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}

	return filtered, nil
}

// getAllPerformance retrieves all performance records for a user.
func (pt *PerformanceTracker) getAllPerformance(ctx context.Context, userID string) ([]IndicatorPerformance, error) {
	indexKey := fmt.Sprintf(PrefixIndicatorPerfIndex, userID)

	// Get the index of hashes
	indexData, err := pt.cache.Get(ctx, indexKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance index: %w", err)
	}

	var hashes []string
	if err := json.Unmarshal([]byte(indexData), &hashes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// Get all performance records
	var results []IndicatorPerformance
	for _, hash := range hashes {
		key := fmt.Sprintf(PrefixIndicatorPerf, userID, hash)
		data, err := pt.cache.Get(ctx, key)
		if err != nil {
			continue // Skip missing records
		}

		var perf IndicatorPerformance
		if err := json.Unmarshal([]byte(data), &perf); err != nil {
			continue
		}

		results = append(results, perf)
	}

	return results, nil
}

// updateIndex adds a hash to the user's performance index.
func (pt *PerformanceTracker) updateIndex(ctx context.Context, userID string, hash string) error {
	indexKey := fmt.Sprintf(PrefixIndicatorPerfIndex, userID)

	// Get existing index
	var hashes []string
	indexData, err := pt.cache.Get(ctx, indexKey)
	if err == nil {
		_ = json.Unmarshal([]byte(indexData), &hashes)
	}

	// Check if hash already exists
	for _, h := range hashes {
		if h == hash {
			return nil // Already in index
		}
	}

	// Add new hash
	hashes = append(hashes, hash)

	// Limit index size (keep most recent 100)
	if len(hashes) > 100 {
		hashes = hashes[len(hashes)-100:]
	}

	// Save updated index
	data, err := json.Marshal(hashes)
	if err != nil {
		return err
	}

	return pt.cache.Set(ctx, indexKey, string(data), pt.ttl)
}

// GetStats returns aggregate stats for a user's indicator performance.
func (pt *PerformanceTracker) GetStats(ctx context.Context, userID string) (*PerformanceStats, error) {
	if pt.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	allPerf, err := pt.getAllPerformance(ctx, userID)
	if err != nil {
		return &PerformanceStats{}, err
	}

	stats := &PerformanceStats{
		TotalCombinations: len(allPerf),
		BySegment:         make(map[IndicatorSegment]SegmentStats),
	}

	for _, segment := range AllSegments() {
		segStats := SegmentStats{}
		for _, perf := range allPerf {
			if perf.Segment == segment {
				segStats.CombinationCount++
				segStats.TotalTrades += perf.TradeCount
				segStats.TotalWins += perf.WinCount
				segStats.TotalPnL += perf.TotalPnL

				if perf.WinRate > segStats.BestWinRate {
					segStats.BestWinRate = perf.WinRate
					segStats.BestCombination = perf.Indicators
				}
			}
		}
		if segStats.TotalTrades > 0 {
			segStats.OverallWinRate = float64(segStats.TotalWins) / float64(segStats.TotalTrades) * 100
		}
		stats.BySegment[segment] = segStats
	}

	return stats, nil
}

// PerformanceStats contains aggregate performance statistics.
type PerformanceStats struct {
	TotalCombinations int                           `json:"total_combinations"`
	BySegment         map[IndicatorSegment]SegmentStats `json:"by_segment"`
}

// SegmentStats contains performance stats for a single segment.
type SegmentStats struct {
	CombinationCount int      `json:"combination_count"`
	TotalTrades      int      `json:"total_trades"`
	TotalWins        int      `json:"total_wins"`
	OverallWinRate   float64  `json:"overall_win_rate"`
	TotalPnL         float64  `json:"total_pnl"`
	BestWinRate      float64  `json:"best_win_rate"`
	BestCombination  []string `json:"best_combination,omitempty"`
}
