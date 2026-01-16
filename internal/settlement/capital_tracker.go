// Package settlement provides capital utilization tracking for Epic 8 Story 8.7.
// This service samples capital usage periodically and aggregates metrics at EOD.
package settlement

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/cache"
)

// CapitalSample represents a single capital sample taken during the day
type CapitalSample struct {
	Timestamp       time.Time `json:"timestamp"`
	TotalBalance    float64   `json:"total_balance"`    // Wallet balance
	UsedMargin      float64   `json:"used_margin"`      // In positions
	AvailableMargin float64   `json:"available_margin"`
	UnrealizedPnL   float64   `json:"unrealized_pnl"`
	Utilization     float64   `json:"utilization"` // Used/Total * 100
}

// CapitalMetrics represents aggregated capital metrics for a day
type CapitalMetrics struct {
	StartingBalance float64 `json:"starting_balance"` // First sample of day
	EndingBalance   float64 `json:"ending_balance"`   // Last sample of day
	MaxCapitalUsed  float64 `json:"max_capital_used"` // Highest used margin
	AvgCapitalUsed  float64 `json:"avg_capital_used"` // Average of samples
	MaxDrawdown     float64 `json:"max_drawdown"`     // Largest unrealized loss
	PeakBalance     float64 `json:"peak_balance"`     // Highest balance during day
	SampleCount     int     `json:"sample_count"`
}

// CapitalTracker handles capital sampling and metrics calculation
type CapitalTracker struct {
	clientFactory *binance.ClientFactory
	cacheService  *cache.CacheService
}

// NewCapitalTracker creates a new capital tracker
func NewCapitalTracker(clientFactory *binance.ClientFactory, cacheService *cache.CacheService) *CapitalTracker {
	return &CapitalTracker{
		clientFactory: clientFactory,
		cacheService:  cacheService,
	}
}

// SampleCapital takes a capital snapshot for a user and stores in Redis
func (t *CapitalTracker) SampleCapital(ctx context.Context, userID string) (*CapitalSample, error) {
	// Get Binance client for user
	client, err := t.clientFactory.GetFuturesClientForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Binance client: %w", err)
	}

	// Get account info which includes balance and positions
	accountInfo, err := client.GetFuturesAccountInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	// Extract balance information from account info
	totalBalance := accountInfo.TotalWalletBalance
	availableBalance := accountInfo.AvailableBalance
	usedMargin := accountInfo.TotalInitialMargin
	unrealizedPnL := accountInfo.TotalUnrealizedProfit

	// Create sample
	sample := &CapitalSample{
		Timestamp:       time.Now(),
		TotalBalance:    totalBalance,
		UsedMargin:      usedMargin,
		AvailableMargin: availableBalance,
		UnrealizedPnL:   unrealizedPnL,
	}

	// Calculate utilization
	if sample.TotalBalance > 0 {
		sample.Utilization = (sample.UsedMargin / sample.TotalBalance) * 100
	}

	// Store in Redis
	err = t.storeSample(ctx, userID, sample)
	if err != nil {
		log.Printf("[CAPITAL-TRACKER] Warning: failed to store sample: %v", err)
		// Don't fail the sample operation if storage fails
	}

	return sample, nil
}

// storeSample stores a capital sample in Redis sorted set
func (t *CapitalTracker) storeSample(ctx context.Context, userID string, sample *CapitalSample) error {
	if t.cacheService == nil {
		return fmt.Errorf("cache service not available")
	}

	// Key format: capital_samples:{user_id}:{YYYY-MM-DD}
	date := sample.Timestamp.Format("2006-01-02")
	key := fmt.Sprintf("capital_samples:%s:%s", userID, date)

	// Serialize sample
	data, err := json.Marshal(sample)
	if err != nil {
		return fmt.Errorf("failed to marshal sample: %w", err)
	}

	// Store as sorted set member with timestamp as score
	score := float64(sample.Timestamp.UnixMilli())
	err = t.cacheService.ZAdd(ctx, key, score, string(data))
	if err != nil {
		return fmt.Errorf("failed to store sample: %w", err)
	}

	// Set TTL of 48 hours (cleanup after EOD aggregation)
	err = t.cacheService.Expire(ctx, key, 48*time.Hour)
	if err != nil {
		log.Printf("[CAPITAL-TRACKER] Warning: failed to set TTL: %v", err)
	}

	return nil
}

// GetDaySamples retrieves all capital samples for a user on a specific date
func (t *CapitalTracker) GetDaySamples(ctx context.Context, userID string, date time.Time) ([]CapitalSample, error) {
	if t.cacheService == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	// Key format: capital_samples:{user_id}:{YYYY-MM-DD}
	dateStr := date.Format("2006-01-02")
	key := fmt.Sprintf("capital_samples:%s:%s", userID, dateStr)

	// Get all members from sorted set
	members, err := t.cacheService.ZRangeWithScores(ctx, key, 0, -1)
	if err != nil {
		return nil, fmt.Errorf("failed to get samples: %w", err)
	}

	var samples []CapitalSample
	for _, member := range members {
		var sample CapitalSample
		err := json.Unmarshal([]byte(member.Member), &sample)
		if err != nil {
			log.Printf("[CAPITAL-TRACKER] Warning: failed to unmarshal sample: %v", err)
			continue
		}
		samples = append(samples, sample)
	}

	return samples, nil
}

// AggregateMetrics calculates aggregated metrics from all samples for a day
func (t *CapitalTracker) AggregateMetrics(ctx context.Context, userID string, date time.Time) (*CapitalMetrics, error) {
	samples, err := t.GetDaySamples(ctx, userID, date)
	if err != nil {
		return nil, err
	}

	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples found for date %s", date.Format("2006-01-02"))
	}

	metrics := &CapitalMetrics{
		SampleCount: len(samples),
	}

	var totalUsedMargin float64
	var minUnrealized float64 = 0 // Track minimum (most negative) unrealized P&L

	for i, sample := range samples {
		// First sample = starting balance
		if i == 0 {
			metrics.StartingBalance = sample.TotalBalance
		}

		// Last sample = ending balance
		metrics.EndingBalance = sample.TotalBalance

		// Track max capital used
		if sample.UsedMargin > metrics.MaxCapitalUsed {
			metrics.MaxCapitalUsed = sample.UsedMargin
		}

		// Track peak balance
		if sample.TotalBalance > metrics.PeakBalance {
			metrics.PeakBalance = sample.TotalBalance
		}

		// Track max drawdown (most negative unrealized P&L)
		if sample.UnrealizedPnL < minUnrealized {
			minUnrealized = sample.UnrealizedPnL
		}

		totalUsedMargin += sample.UsedMargin
	}

	// Calculate average capital used
	metrics.AvgCapitalUsed = totalUsedMargin / float64(len(samples))

	// Max drawdown is the absolute value of the most negative unrealized P&L
	if minUnrealized < 0 {
		metrics.MaxDrawdown = -minUnrealized
	}

	return metrics, nil
}

// ClearDaySamples removes all capital samples for a day after aggregation
func (t *CapitalTracker) ClearDaySamples(ctx context.Context, userID string, date time.Time) error {
	if t.cacheService == nil {
		return fmt.Errorf("cache service not available")
	}

	dateStr := date.Format("2006-01-02")
	key := fmt.Sprintf("capital_samples:%s:%s", userID, dateStr)

	return t.cacheService.Delete(ctx, key)
}
