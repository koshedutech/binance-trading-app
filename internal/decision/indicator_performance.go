// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.14: Indicator Performance Tracker
package decision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"binance-trading-bot/internal/cache"
)

// Redis key prefixes for indicator performance tracking
const (
	// PrefixTradeSnapshot stores trade entry snapshots
	// Format: decision:trade_snapshot:{userID}:{tradeID}
	PrefixTradeSnapshot = "decision:trade_snapshot:%s:%s"

	// PrefixIndicatorPerfStrategy stores strategy-level performance
	// Format: decision:indicator_perf_strategy:{userID}:{strategy}:{hash}
	PrefixIndicatorPerfStrategy = "decision:indicator_perf_strategy:%s:%s:%s"

	// PrefixIndicatorCorrelation stores correlation data
	// Format: decision:indicator_corr:{userID}:{strategy}
	PrefixIndicatorCorrelation = "decision:indicator_corr:%s:%s"

	// PrefixCompletedTrades stores completed trade records for analysis
	// Format: decision:completed_trades:{userID}:{strategy}
	PrefixCompletedTrades = "decision:completed_trades:%s:%s"

	// DefaultPerformanceTTL is the default TTL for performance data (30 days)
	DefaultPerformanceTTL = 30 * 24 * time.Hour

	// MinTradesForRecommendation is the minimum trades needed before generating recommendations
	MinTradesForRecommendation = 20

	// MinTradesForCorrelation is the minimum data points for meaningful correlation
	MinTradesForCorrelation = 10
)

// TradeIndicatorSnapshot captures indicator values at trade entry time.
type TradeIndicatorSnapshot struct {
	// TradeID is the unique identifier for this trade (e.g., clientOrderId or chain base ID)
	TradeID string `json:"trade_id"`
	// UserID is the user who made this trade
	UserID int `json:"user_id"`
	// Symbol is the trading pair (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`
	// Strategy is the trading strategy used (e.g., "trend_following")
	Strategy string `json:"strategy"`
	// Indicators maps indicator names to their values at entry time
	Indicators map[string]float64 `json:"indicators"`
	// EntryScore is the decision score at entry (0-100)
	EntryScore int `json:"entry_score"`
	// EntryTime is when the trade was entered
	EntryTime time.Time `json:"entry_time"`
	// MarketRegime is the detected market regime at entry
	MarketRegime string `json:"market_regime,omitempty"`
	// Side is LONG or SHORT
	Side string `json:"side,omitempty"`
}

// TradeOutcome records the result of a trade.
type TradeOutcome struct {
	// TradeID links this outcome to the entry snapshot
	TradeID string `json:"trade_id"`
	// IsWin indicates if the trade was profitable
	IsWin bool `json:"is_win"`
	// PnLPercent is the realized P&L percentage
	PnLPercent float64 `json:"pnl_percent"`
	// ExitTime is when the trade was closed
	ExitTime time.Time `json:"exit_time"`
	// ExitReason describes why the trade was closed (e.g., "TP_HIT", "SL_HIT", "MANUAL")
	ExitReason string `json:"exit_reason"`
}

// CompletedTrade combines entry snapshot with outcome for analysis.
type CompletedTrade struct {
	Snapshot *TradeIndicatorSnapshot `json:"snapshot"`
	Outcome  *TradeOutcome           `json:"outcome"`
}

// IndicatorCombinationStats tracks performance for a specific indicator combination.
type IndicatorCombinationStats struct {
	// CombinationHash is a unique identifier for this indicator set
	CombinationHash string `json:"combination_hash"`
	// Indicators lists the indicators in this combination
	Indicators []string `json:"indicators"`
	// Strategy is the trading strategy
	Strategy string `json:"strategy"`
	// TradeCount is the total number of trades
	TradeCount int `json:"trade_count"`
	// WinCount is the number of winning trades
	WinCount int `json:"win_count"`
	// WinRate is the win percentage (0-100)
	WinRate float64 `json:"win_rate"`
	// AvgPnLPercent is the average P&L percentage per trade
	AvgPnLPercent float64 `json:"avg_pnl_percent"`
	// TotalPnLPercent is the sum of all P&L percentages
	TotalPnLPercent float64 `json:"total_pnl_percent"`
	// AvgEntryScore is the average entry score
	AvgEntryScore float64 `json:"avg_entry_score"`
	// TotalEntryScore is the sum of all entry scores
	TotalEntryScore float64 `json:"total_entry_score"`
	// LastUpdated is when this record was last updated
	LastUpdated time.Time `json:"last_updated"`
	// CreatedAt is when this record was created
	CreatedAt time.Time `json:"created_at"`
}

// RecordTrade updates the stats with a new trade result.
func (ics *IndicatorCombinationStats) RecordTrade(won bool, entryScore int, pnlPercent float64) {
	ics.TradeCount++
	if won {
		ics.WinCount++
	}
	ics.WinRate = float64(ics.WinCount) / float64(ics.TradeCount) * 100

	ics.TotalEntryScore += float64(entryScore)
	ics.AvgEntryScore = ics.TotalEntryScore / float64(ics.TradeCount)

	ics.TotalPnLPercent += pnlPercent
	ics.AvgPnLPercent = ics.TotalPnLPercent / float64(ics.TradeCount)

	ics.LastUpdated = time.Now()
}

// IndicatorCorrelation tracks the correlation between an individual indicator and trade success.
type IndicatorCorrelation struct {
	// IndicatorName is the name of the indicator
	IndicatorName string `json:"indicator_name"`
	// WinCorrelation is the Pearson correlation coefficient with win/loss (-1 to 1)
	WinCorrelation float64 `json:"win_correlation"`
	// PnLCorrelation is the Pearson correlation coefficient with PnL% (-1 to 1)
	PnLCorrelation float64 `json:"pnl_correlation"`
	// SampleSize is the number of trades used for correlation
	SampleSize int `json:"sample_size"`
	// AvgValueOnWin is the average indicator value on winning trades
	AvgValueOnWin float64 `json:"avg_value_on_win"`
	// AvgValueOnLoss is the average indicator value on losing trades
	AvgValueOnLoss float64 `json:"avg_value_on_loss"`
	// LastCalculated is when this correlation was last computed
	LastCalculated time.Time `json:"last_calculated"`
}

// Recommendation represents an actionable suggestion based on performance data.
type Recommendation struct {
	// Type is the kind of recommendation
	Type RecommendationType `json:"type"`
	// Priority indicates importance (HIGH, MEDIUM, LOW)
	Priority string `json:"priority"`
	// Indicator is the indicator involved (if applicable)
	Indicator string `json:"indicator,omitempty"`
	// Segment is the indicator segment (if applicable)
	Segment string `json:"segment,omitempty"`
	// CurrentValue is the current setting value (if applicable)
	CurrentValue float64 `json:"current_value,omitempty"`
	// SuggestedValue is the recommended value (if applicable)
	SuggestedValue float64 `json:"suggested_value,omitempty"`
	// Reason explains why this recommendation is made
	Reason string `json:"reason"`
	// ConfidenceLevel indicates how confident we are (0-1)
	ConfidenceLevel float64 `json:"confidence_level"`
	// BasedOnTrades is the number of trades this is based on
	BasedOnTrades int `json:"based_on_trades"`
}

// RecommendationType defines the types of recommendations.
type RecommendationType string

const (
	// RecommendationAddIndicator suggests adding an indicator
	RecommendationAddIndicator RecommendationType = "ADD_INDICATOR"
	// RecommendationRemoveIndicator suggests removing an indicator
	RecommendationRemoveIndicator RecommendationType = "REMOVE_INDICATOR"
	// RecommendationAdjustWeight suggests changing indicator weight
	RecommendationAdjustWeight RecommendationType = "ADJUST_WEIGHT"
	// RecommendationChangeStrategy suggests trying a different strategy
	RecommendationChangeStrategy RecommendationType = "CHANGE_STRATEGY"
	// RecommendationKeepCurrent suggests no changes needed
	RecommendationKeepCurrent RecommendationType = "KEEP_CURRENT"
)

// IndicatorPerformanceTracker tracks indicator performance across trades.
// It integrates with the trade lifecycle to record entry indicators and correlate with outcomes.
type IndicatorPerformanceTracker struct {
	cache *cache.CacheService
	mu    sync.RWMutex
	ttl   time.Duration

	// pendingTrades stores snapshots for trades that haven't closed yet
	pendingTrades map[string]*TradeIndicatorSnapshot

	// Auto-optimization flag (stored but implementation deferred)
	autoOptimizeEnabled bool
}

// NewIndicatorPerformanceTracker creates a new tracker with the given cache service.
func NewIndicatorPerformanceTracker(cacheService *cache.CacheService) *IndicatorPerformanceTracker {
	return &IndicatorPerformanceTracker{
		cache:         cacheService,
		ttl:           DefaultPerformanceTTL,
		pendingTrades: make(map[string]*TradeIndicatorSnapshot),
	}
}

// SetAutoOptimize enables or disables auto-optimization mode.
// Note: Actual auto-optimization implementation is deferred; this just stores the flag.
func (t *IndicatorPerformanceTracker) SetAutoOptimize(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.autoOptimizeEnabled = enabled
}

// IsAutoOptimizeEnabled returns whether auto-optimization is enabled.
func (t *IndicatorPerformanceTracker) IsAutoOptimizeEnabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.autoOptimizeEnabled
}

// RecordTradeEntry logs indicator values when a trade is opened.
// This creates a pending trade snapshot that will be correlated with the outcome on exit.
func (t *IndicatorPerformanceTracker) RecordTradeEntry(ctx context.Context, snapshot *TradeIndicatorSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot is required")
	}
	if snapshot.TradeID == "" {
		return fmt.Errorf("trade_id is required")
	}
	if snapshot.UserID <= 0 {
		return fmt.Errorf("user_id must be positive")
	}
	if snapshot.Strategy == "" {
		snapshot.Strategy = "trend_following" // Default strategy
	}

	t.mu.Lock()
	t.pendingTrades[snapshot.TradeID] = snapshot
	t.mu.Unlock()

	// Also persist to Redis for recovery after restart
	if t.cache != nil {
		userIDStr := fmt.Sprintf("%d", snapshot.UserID)
		key := fmt.Sprintf(PrefixTradeSnapshot, userIDStr, snapshot.TradeID)
		data, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("failed to marshal snapshot: %w", err)
		}
		if err := t.cache.Set(ctx, key, string(data), t.ttl); err != nil {
			log.Printf("[INDICATOR_PERF] Failed to persist trade snapshot to Redis: %v", err)
			// Don't fail - memory tracking is still active
		}
	}

	log.Printf("[INDICATOR_PERF] Recorded trade entry: tradeID=%s, symbol=%s, strategy=%s, indicators=%d",
		snapshot.TradeID, snapshot.Symbol, snapshot.Strategy, len(snapshot.Indicators))

	return nil
}

// RecordTradeExit records the trade outcome and updates performance statistics.
// It correlates the outcome with the entry snapshot and updates all relevant metrics.
func (t *IndicatorPerformanceTracker) RecordTradeExit(ctx context.Context, outcome *TradeOutcome) error {
	if outcome == nil {
		return fmt.Errorf("outcome is required")
	}
	if outcome.TradeID == "" {
		return fmt.Errorf("trade_id is required")
	}

	// Find the corresponding entry snapshot
	t.mu.Lock()
	snapshot, exists := t.pendingTrades[outcome.TradeID]
	if exists {
		delete(t.pendingTrades, outcome.TradeID)
	}
	t.mu.Unlock()

	// If not in memory, try to load from Redis by scanning for the tradeID
	if !exists && t.cache != nil {
		snapshot = t.findSnapshotInRedis(ctx, outcome.TradeID)
		if snapshot != nil {
			log.Printf("[INDICATOR_PERF] Recovered snapshot from Redis for tradeID=%s", outcome.TradeID)
		}
	}

	if snapshot == nil {
		log.Printf("[INDICATOR_PERF] No snapshot found for tradeID=%s, skipping", outcome.TradeID)
		return nil
	}

	// Create completed trade record
	completedTrade := &CompletedTrade{
		Snapshot: snapshot,
		Outcome:  outcome,
	}

	// Update combination stats
	if err := t.updateCombinationStats(ctx, snapshot, outcome); err != nil {
		log.Printf("[INDICATOR_PERF] Failed to update combination stats: %v", err)
	}

	// Store completed trade for correlation analysis
	if err := t.storeCompletedTrade(ctx, completedTrade); err != nil {
		log.Printf("[INDICATOR_PERF] Failed to store completed trade: %v", err)
	}

	// Clean up Redis snapshot
	if t.cache != nil {
		userIDStr := fmt.Sprintf("%d", snapshot.UserID)
		key := fmt.Sprintf(PrefixTradeSnapshot, userIDStr, outcome.TradeID)
		_ = t.cache.Delete(ctx, key)
	}

	log.Printf("[INDICATOR_PERF] Recorded trade exit: tradeID=%s, symbol=%s, isWin=%v, pnl=%.2f%%",
		outcome.TradeID, snapshot.Symbol, outcome.IsWin, outcome.PnLPercent)

	return nil
}

// updateCombinationStats updates the performance stats for the indicator combination used.
// Uses a per-combination lock key approach to prevent race conditions while allowing
// concurrent updates to different combinations.
func (t *IndicatorPerformanceTracker) updateCombinationStats(ctx context.Context, snapshot *TradeIndicatorSnapshot, outcome *TradeOutcome) error {
	if t.cache == nil {
		return fmt.Errorf("cache service not available")
	}

	// Extract indicator names and create hash
	indicators := make([]string, 0, len(snapshot.Indicators))
	for name := range snapshot.Indicators {
		indicators = append(indicators, name)
	}
	sort.Strings(indicators)
	hash := hashIndicatorList(indicators)

	userIDStr := fmt.Sprintf("%d", snapshot.UserID)
	key := fmt.Sprintf(PrefixIndicatorPerfStrategy, userIDStr, snapshot.Strategy, hash)

	// Lock for read-modify-write to prevent lost updates on concurrent trade closures
	// Note: For better scalability, consider using Redis transactions (WATCH/MULTI/EXEC)
	// or distributed locks for high-concurrency scenarios
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get or create stats
	var stats *IndicatorCombinationStats
	data, err := t.cache.Get(ctx, key)
	if err == nil && data != "" {
		stats = &IndicatorCombinationStats{}
		if err := json.Unmarshal([]byte(data), stats); err != nil {
			stats = nil
		}
	}

	if stats == nil {
		stats = &IndicatorCombinationStats{
			CombinationHash: hash,
			Indicators:      indicators,
			Strategy:        snapshot.Strategy,
			CreatedAt:       time.Now(),
		}
	}

	// Update stats
	stats.RecordTrade(outcome.IsWin, snapshot.EntryScore, outcome.PnLPercent)

	// Save back to Redis
	updatedData, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	return t.cache.Set(ctx, key, string(updatedData), t.ttl)
}

// storeCompletedTrade stores a completed trade record for later analysis.
// Uses mutex locking to prevent race conditions on concurrent read-modify-write operations.
func (t *IndicatorPerformanceTracker) storeCompletedTrade(ctx context.Context, trade *CompletedTrade) error {
	if t.cache == nil {
		return fmt.Errorf("cache service not available")
	}

	userIDStr := fmt.Sprintf("%d", trade.Snapshot.UserID)
	key := fmt.Sprintf(PrefixCompletedTrades, userIDStr, trade.Snapshot.Strategy)

	// Lock for read-modify-write operation to prevent race conditions
	// when multiple trades close concurrently for the same user/strategy
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get existing trades list
	var trades []*CompletedTrade
	data, err := t.cache.Get(ctx, key)
	if err == nil && data != "" {
		_ = json.Unmarshal([]byte(data), &trades)
	}

	// Append new trade (keep last 500 for analysis)
	trades = append(trades, trade)
	if len(trades) > 500 {
		trades = trades[len(trades)-500:]
	}

	// Save back
	updatedData, err := json.Marshal(trades)
	if err != nil {
		return fmt.Errorf("failed to marshal trades: %w", err)
	}

	return t.cache.Set(ctx, key, string(updatedData), t.ttl)
}

// GetIndicatorCorrelations calculates correlations between indicator values and trade outcomes.
func (t *IndicatorPerformanceTracker) GetIndicatorCorrelations(ctx context.Context, userID int, strategy string) (map[string]*IndicatorCorrelation, error) {
	if t.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	// Load completed trades
	trades, err := t.getCompletedTrades(ctx, userID, strategy, 500)
	if err != nil {
		return nil, fmt.Errorf("failed to load completed trades: %w", err)
	}

	if len(trades) < MinTradesForCorrelation {
		return nil, fmt.Errorf("insufficient trades for correlation: have %d, need %d", len(trades), MinTradesForCorrelation)
	}

	// Filter out invalid trades first to avoid sparse array issues
	validTrades := make([]*CompletedTrade, 0, len(trades))
	for _, trade := range trades {
		if trade.Snapshot != nil && trade.Outcome != nil {
			validTrades = append(validTrades, trade)
		}
	}

	if len(validTrades) < MinTradesForCorrelation {
		return nil, fmt.Errorf("insufficient valid trades for correlation: have %d, need %d", len(validTrades), MinTradesForCorrelation)
	}

	// Build indicator value arrays and outcome arrays using only valid trades
	indicatorValues := make(map[string][]float64)
	winValues := make([]float64, len(validTrades))
	pnlValues := make([]float64, len(validTrades))

	for i, trade := range validTrades {
		if trade.Outcome.IsWin {
			winValues[i] = 1.0
		} else {
			winValues[i] = 0.0
		}
		pnlValues[i] = trade.Outcome.PnLPercent

		for name, value := range trade.Snapshot.Indicators {
			if _, exists := indicatorValues[name]; !exists {
				indicatorValues[name] = make([]float64, len(validTrades))
			}
			indicatorValues[name][i] = value
		}
	}

	// Calculate correlations for each indicator
	correlations := make(map[string]*IndicatorCorrelation)
	for name, values := range indicatorValues {
		// Skip if we don't have enough non-zero values
		nonZeroCount := 0
		for _, v := range values {
			if v != 0 {
				nonZeroCount++
			}
		}
		if nonZeroCount < MinTradesForCorrelation {
			continue
		}

		winCorr := calculatePearsonCorrelation(values, winValues)
		pnlCorr := calculatePearsonCorrelation(values, pnlValues)

		// Calculate average values on win vs loss
		var sumOnWin, sumOnLoss float64
		var countOnWin, countOnLoss int
		for i, value := range values {
			if winValues[i] == 1.0 {
				sumOnWin += value
				countOnWin++
			} else {
				sumOnLoss += value
				countOnLoss++
			}
		}

		avgOnWin := 0.0
		if countOnWin > 0 {
			avgOnWin = sumOnWin / float64(countOnWin)
		}
		avgOnLoss := 0.0
		if countOnLoss > 0 {
			avgOnLoss = sumOnLoss / float64(countOnLoss)
		}

		correlations[name] = &IndicatorCorrelation{
			IndicatorName:  name,
			WinCorrelation: winCorr,
			PnLCorrelation: pnlCorr,
			SampleSize:     len(validTrades),
			AvgValueOnWin:  avgOnWin,
			AvgValueOnLoss: avgOnLoss,
			LastCalculated: time.Now(),
		}
	}

	return correlations, nil
}

// GetTopPerformingCombinations returns the best indicator combinations by win rate and PnL.
func (t *IndicatorPerformanceTracker) GetTopPerformingCombinations(ctx context.Context, userID int, strategy string, limit int) ([]IndicatorCombinationStats, error) {
	if t.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	if limit <= 0 {
		limit = 10
	}

	// Get all combination stats for this user and strategy
	// We need to scan for keys matching the pattern
	userIDStr := fmt.Sprintf("%d", userID)
	pattern := fmt.Sprintf(PrefixIndicatorPerfStrategy, userIDStr, strategy, "*")

	keys, err := t.cache.ScanKeys(ctx, pattern, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	var combinations []IndicatorCombinationStats
	for _, key := range keys {
		data, err := t.cache.Get(ctx, key)
		if err != nil || data == "" {
			continue
		}

		var stats IndicatorCombinationStats
		if err := json.Unmarshal([]byte(data), &stats); err != nil {
			continue
		}

		// Only include combinations with enough trades
		if stats.TradeCount >= 5 {
			combinations = append(combinations, stats)
		}
	}

	// Sort by composite score (win rate * 0.6 + avg PnL rank * 0.4)
	sort.Slice(combinations, func(i, j int) bool {
		// Prefer higher win rate and higher avg PnL
		scoreI := combinations[i].WinRate*0.6 + math.Min(combinations[i].AvgPnLPercent*10, 40)
		scoreJ := combinations[j].WinRate*0.6 + math.Min(combinations[j].AvgPnLPercent*10, 40)
		return scoreI > scoreJ
	})

	if len(combinations) > limit {
		combinations = combinations[:limit]
	}

	return combinations, nil
}

// GetRecommendations generates actionable recommendations based on performance data.
func (t *IndicatorPerformanceTracker) GetRecommendations(ctx context.Context, userID int, strategy string) ([]Recommendation, error) {
	if t.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	// Get correlations
	correlations, err := t.GetIndicatorCorrelations(ctx, userID, strategy)
	if err != nil {
		// Not enough data for correlations - return basic recommendation
		return []Recommendation{{
			Type:            RecommendationKeepCurrent,
			Priority:        "LOW",
			Reason:          fmt.Sprintf("Insufficient trade data for analysis: %v", err),
			ConfidenceLevel: 0.1,
			BasedOnTrades:   0,
		}}, nil
	}

	// Get top combinations
	topCombos, err := t.GetTopPerformingCombinations(ctx, userID, strategy, 5)
	if err != nil {
		topCombos = nil // Continue without combinations
	}

	// Generate recommendations
	recommendations := t.analyzeIndicatorPerformance(correlations, topCombos, MinTradesForRecommendation)

	return recommendations, nil
}

// analyzeIndicatorPerformance generates recommendations based on correlation and combination data.
func (t *IndicatorPerformanceTracker) analyzeIndicatorPerformance(
	correlations map[string]*IndicatorCorrelation,
	combinations []IndicatorCombinationStats,
	minTrades int,
) []Recommendation {
	var recommendations []Recommendation

	// Analyze correlations for poorly performing indicators
	for name, corr := range correlations {
		if corr.SampleSize < minTrades {
			continue
		}

		// Strong negative correlation with win rate suggests removing this indicator
		if corr.WinCorrelation < -0.3 {
			recommendations = append(recommendations, Recommendation{
				Type:            RecommendationRemoveIndicator,
				Priority:        t.priorityFromCorrelation(corr.WinCorrelation),
				Indicator:       name,
				Reason:          fmt.Sprintf("Indicator '%s' has negative correlation (%.2f) with winning trades. Average value on wins: %.2f, on losses: %.2f", name, corr.WinCorrelation, corr.AvgValueOnWin, corr.AvgValueOnLoss),
				ConfidenceLevel: t.confidenceFromSampleSize(corr.SampleSize),
				BasedOnTrades:   corr.SampleSize,
			})
		}

		// Strong positive correlation suggests this indicator is working well
		if corr.WinCorrelation > 0.4 {
			recommendations = append(recommendations, Recommendation{
				Type:            RecommendationAdjustWeight,
				Priority:        "MEDIUM",
				Indicator:       name,
				SuggestedValue:  0.7, // Suggest higher weight
				Reason:          fmt.Sprintf("Indicator '%s' has strong positive correlation (%.2f) with winning trades. Consider increasing its weight.", name, corr.WinCorrelation),
				ConfidenceLevel: t.confidenceFromSampleSize(corr.SampleSize),
				BasedOnTrades:   corr.SampleSize,
			})
		}
	}

	// Analyze top combinations for suggestions
	if len(combinations) > 0 {
		bestCombo := combinations[0]
		if bestCombo.WinRate > 60 && bestCombo.TradeCount >= minTrades {
			recommendations = append(recommendations, Recommendation{
				Type:            RecommendationKeepCurrent,
				Priority:        "LOW",
				Reason:          fmt.Sprintf("Top performing combination (%v) has %.1f%% win rate over %d trades. Consider maintaining this setup.", bestCombo.Indicators, bestCombo.WinRate, bestCombo.TradeCount),
				ConfidenceLevel: t.confidenceFromSampleSize(bestCombo.TradeCount),
				BasedOnTrades:   bestCombo.TradeCount,
			})
		}

		// Find indicators common in top combinations but not in current setup
		indicatorFrequency := make(map[string]int)
		for _, combo := range combinations {
			if combo.WinRate > 55 {
				for _, ind := range combo.Indicators {
					indicatorFrequency[ind]++
				}
			}
		}

		for ind, freq := range indicatorFrequency {
			if freq >= 3 && freq == len(combinations) {
				recommendations = append(recommendations, Recommendation{
					Type:            RecommendationAddIndicator,
					Priority:        "MEDIUM",
					Indicator:       ind,
					Reason:          fmt.Sprintf("Indicator '%s' appears in all top %d performing combinations", ind, len(combinations)),
					ConfidenceLevel: 0.7,
					BasedOnTrades:   combinations[0].TradeCount,
				})
			}
		}
	}

	// If no specific recommendations, give general advice
	if len(recommendations) == 0 {
		recommendations = append(recommendations, Recommendation{
			Type:            RecommendationKeepCurrent,
			Priority:        "LOW",
			Reason:          "Current indicator configuration appears balanced. Continue trading to gather more data.",
			ConfidenceLevel: 0.5,
			BasedOnTrades:   0,
		})
	}

	// Sort by priority (HIGH > MEDIUM > LOW)
	priorityOrder := map[string]int{"HIGH": 0, "MEDIUM": 1, "LOW": 2}
	sort.Slice(recommendations, func(i, j int) bool {
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})

	return recommendations
}

// priorityFromCorrelation determines priority based on correlation strength.
func (t *IndicatorPerformanceTracker) priorityFromCorrelation(corr float64) string {
	absCorr := math.Abs(corr)
	if absCorr > 0.6 {
		return "HIGH"
	}
	if absCorr > 0.4 {
		return "MEDIUM"
	}
	return "LOW"
}

// confidenceFromSampleSize calculates confidence based on sample size.
func (t *IndicatorPerformanceTracker) confidenceFromSampleSize(sampleSize int) float64 {
	if sampleSize >= 100 {
		return 0.9
	}
	if sampleSize >= 50 {
		return 0.7
	}
	if sampleSize >= 20 {
		return 0.5
	}
	return 0.3
}

// getCompletedTrades retrieves completed trade records for analysis.
func (t *IndicatorPerformanceTracker) getCompletedTrades(ctx context.Context, userID int, strategy string, limit int) ([]*CompletedTrade, error) {
	if t.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	userIDStr := fmt.Sprintf("%d", userID)
	key := fmt.Sprintf(PrefixCompletedTrades, userIDStr, strategy)

	data, err := t.cache.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get completed trades: %w", err)
	}

	var trades []*CompletedTrade
	if data != "" {
		if err := json.Unmarshal([]byte(data), &trades); err != nil {
			return nil, fmt.Errorf("failed to unmarshal trades: %w", err)
		}
	}

	if limit > 0 && len(trades) > limit {
		trades = trades[len(trades)-limit:]
	}

	return trades, nil
}

// GetPerformanceStats returns overall performance statistics for a user and strategy.
func (t *IndicatorPerformanceTracker) GetPerformanceStats(ctx context.Context, userID int, strategy string) (*PerformanceStatsResponse, error) {
	if t.cache == nil {
		return nil, fmt.Errorf("cache service not available")
	}

	trades, err := t.getCompletedTrades(ctx, userID, strategy, 500)
	if err != nil || len(trades) == 0 {
		return &PerformanceStatsResponse{
			UserID:          userID,
			Strategy:        strategy,
			TotalTrades:     0,
			Message:         "No completed trades found",
		}, nil
	}

	// Calculate overall stats
	totalTrades := len(trades)
	winCount := 0
	totalPnL := 0.0
	totalScore := 0.0

	for _, trade := range trades {
		if trade.Outcome != nil && trade.Outcome.IsWin {
			winCount++
		}
		if trade.Outcome != nil {
			totalPnL += trade.Outcome.PnLPercent
		}
		if trade.Snapshot != nil {
			totalScore += float64(trade.Snapshot.EntryScore)
		}
	}

	winRate := 0.0
	avgPnL := 0.0
	avgScore := 0.0
	if totalTrades > 0 {
		winRate = float64(winCount) / float64(totalTrades) * 100
		avgPnL = totalPnL / float64(totalTrades)
		avgScore = totalScore / float64(totalTrades)
	}

	// Get top combinations
	topCombos, _ := t.GetTopPerformingCombinations(ctx, userID, strategy, 3)

	return &PerformanceStatsResponse{
		UserID:          userID,
		Strategy:        strategy,
		TotalTrades:     totalTrades,
		WinCount:        winCount,
		WinRate:         winRate,
		AvgPnLPercent:   avgPnL,
		TotalPnLPercent: totalPnL,
		AvgEntryScore:   avgScore,
		TopCombinations: topCombos,
		LastUpdated:     time.Now(),
	}, nil
}

// PerformanceStatsResponse is the API response for performance statistics.
type PerformanceStatsResponse struct {
	UserID          int                         `json:"user_id"`
	Strategy        string                      `json:"strategy"`
	TotalTrades     int                         `json:"total_trades"`
	WinCount        int                         `json:"win_count"`
	WinRate         float64                     `json:"win_rate"`
	AvgPnLPercent   float64                     `json:"avg_pnl_percent"`
	TotalPnLPercent float64                     `json:"total_pnl_percent"`
	AvgEntryScore   float64                     `json:"avg_entry_score"`
	TopCombinations []IndicatorCombinationStats `json:"top_combinations,omitempty"`
	LastUpdated     time.Time                   `json:"last_updated"`
	Message         string                      `json:"message,omitempty"`
}

// hashIndicatorList creates a SHA256 hash (first 8 chars) from a sorted list of indicators.
func hashIndicatorList(indicators []string) string {
	if len(indicators) == 0 {
		return "00000000"
	}

	sorted := make([]string, len(indicators))
	copy(sorted, indicators)
	sort.Strings(sorted)

	h := sha256.New()
	for _, ind := range sorted {
		h.Write([]byte(ind))
		h.Write([]byte(":"))
	}

	return hex.EncodeToString(h.Sum(nil))[:8]
}

// calculatePearsonCorrelation computes the Pearson correlation coefficient between two series.
func calculatePearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) < 2 {
		return 0
	}

	n := float64(len(x))

	// Calculate means
	sumX, sumY := 0.0, 0.0
	for i := range x {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate correlation
	var sumXY, sumX2, sumY2 float64
	for i := range x {
		dx := x[i] - meanX
		dy := y[i] - meanY
		sumXY += dx * dy
		sumX2 += dx * dx
		sumY2 += dy * dy
	}

	if sumX2 == 0 || sumY2 == 0 {
		return 0
	}

	return sumXY / (math.Sqrt(sumX2) * math.Sqrt(sumY2))
}

// findSnapshotInRedis scans Redis for a trade snapshot by tradeID.
// This is used when the snapshot is not in memory (e.g., after service restart).
// Returns nil if not found.
func (t *IndicatorPerformanceTracker) findSnapshotInRedis(ctx context.Context, tradeID string) *TradeIndicatorSnapshot {
	if t.cache == nil {
		return nil
	}

	// Scan for keys matching the trade snapshot pattern with this tradeID
	// Pattern: decision:trade_snapshot:*:{tradeID}
	pattern := fmt.Sprintf("decision:trade_snapshot:*:%s", tradeID)

	keys, err := t.cache.ScanKeys(ctx, pattern, 10)
	if err != nil || len(keys) == 0 {
		return nil
	}

	// Try to load the first matching key
	data, err := t.cache.Get(ctx, keys[0])
	if err != nil || data == "" {
		return nil
	}

	var snapshot TradeIndicatorSnapshot
	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		return nil
	}

	// Clean up the Redis key since we found it
	_ = t.cache.Delete(ctx, keys[0])

	return &snapshot
}

// ClearPendingTrades clears all pending trade snapshots (for testing/reset).
func (t *IndicatorPerformanceTracker) ClearPendingTrades() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingTrades = make(map[string]*TradeIndicatorSnapshot)
}

// GetPendingTradeCount returns the number of pending trades.
func (t *IndicatorPerformanceTracker) GetPendingTradeCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.pendingTrades)
}
