// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.7: Strategy Selector
package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// SelectorConfig holds configuration for the StrategySelector.
type SelectorConfig struct {
	// CooldownDuration is the minimum time between strategy switches for a symbol.
	// Default: 5 minutes
	CooldownDuration time.Duration

	// DefaultStrategy is used when no strategy matches the current regime.
	// Default: "trend_following"
	DefaultStrategy string

	// SelectionHistoryTTL is how long to keep selection history entries.
	// Default: 24 hours
	SelectionHistoryTTL time.Duration

	// MaxHistoryEntries is the maximum selection history entries per symbol.
	// Default: 100
	MaxHistoryEntries int

	// EnableLogging controls whether selections are logged.
	// Default: true
	EnableLogging bool
}

// DefaultSelectorConfig returns a SelectorConfig with sensible defaults.
func DefaultSelectorConfig() *SelectorConfig {
	return &SelectorConfig{
		CooldownDuration:    5 * time.Minute,
		DefaultStrategy:     "trend_following",
		SelectionHistoryTTL: 24 * time.Hour,
		MaxHistoryEntries:   100,
		EnableLogging:       true,
	}
}

// SelectionReason describes why a strategy was selected.
type SelectionReason string

const (
	// ReasonRegimeMatch indicates strategy was selected based on regime match.
	ReasonRegimeMatch SelectionReason = "REGIME_MATCH"
	// ReasonUserPreference indicates user preference override was applied.
	ReasonUserPreference SelectionReason = "USER_PREFERENCE"
	// ReasonFallback indicates the default/fallback strategy was used.
	ReasonFallback SelectionReason = "FALLBACK"
	// ReasonCooldown indicates the previous strategy was retained due to cooldown.
	ReasonCooldown SelectionReason = "COOLDOWN"
	// ReasonBestScore indicates strategy was selected as best scoring for regime.
	ReasonBestScore SelectionReason = "BEST_SCORE"
)

// StrategySelection represents a strategy selection event.
type StrategySelection struct {
	// Symbol is the trading pair this selection applies to.
	Symbol string `json:"symbol"`
	// UserID is the user this selection belongs to.
	UserID string `json:"user_id"`
	// Regime is the market regime at selection time.
	Regime MarketRegime `json:"regime"`
	// SelectedStrategy is the name of the selected strategy.
	SelectedStrategy string `json:"selected_strategy"`
	// PreviousStrategy is the strategy that was active before (if any).
	PreviousStrategy string `json:"previous_strategy,omitempty"`
	// Reason explains why this strategy was selected.
	Reason SelectionReason `json:"reason"`
	// Score is the strategy's score at selection time (if scored).
	Score float64 `json:"score,omitempty"`
	// Timestamp is when the selection was made (Unix milliseconds).
	Timestamp int64 `json:"timestamp"`
	// CooldownActive indicates if cooldown prevented a switch.
	CooldownActive bool `json:"cooldown_active,omitempty"`
	// AvailableStrategies lists strategies that were considered.
	AvailableStrategies []string `json:"available_strategies,omitempty"`
}

// NewStrategySelection creates a new StrategySelection with current timestamp.
func NewStrategySelection(symbol, userID string, regime MarketRegime, strategy string, reason SelectionReason) *StrategySelection {
	return &StrategySelection{
		Symbol:           symbol,
		UserID:           userID,
		Regime:           regime,
		SelectedStrategy: strategy,
		Reason:           reason,
		Timestamp:        time.Now().UnixMilli(),
	}
}

// ToJSON returns the JSON representation of the selection.
// Returns an empty JSON object "{}" if marshaling fails.
func (ss *StrategySelection) ToJSON() string {
	data, err := json.Marshal(ss)
	if err != nil {
		log.Printf("[SELECTOR] Warning: Failed to marshal selection to JSON: %v", err)
		return "{}"
	}
	return string(data)
}

// CooldownState tracks the strategy switch cooldown for a symbol.
type CooldownState struct {
	// Symbol is the trading pair.
	Symbol string `json:"symbol"`
	// LastStrategy is the strategy that was last active.
	LastStrategy string `json:"last_strategy"`
	// LastSwitchTime is when the strategy was last changed (Unix milliseconds).
	LastSwitchTime int64 `json:"last_switch_time"`
}

// Redis key patterns for strategy selector
const (
	// PrefixStrategyCooldown is the Redis key pattern for cooldown state.
	// Format: decision:user:{userID}:cooldown:{symbol}
	PrefixStrategyCooldown = "decision:user:%s:cooldown:%s"

	// PrefixStrategyHistory is the Redis key pattern for selection history (sorted set).
	// Format: decision:user:{userID}:selection_history:{symbol}
	PrefixStrategyHistory = "decision:user:%s:selection_history:%s"

	// PrefixUserPreference is the Redis key pattern for user strategy preference.
	// Format: decision:user:{userID}:preference:{symbol}
	PrefixUserPreference = "decision:user:%s:preference:%s"
)

// strategyCooldownKey generates the Redis key for cooldown state.
func strategyCooldownKey(userID, symbol string) string {
	return fmt.Sprintf(PrefixStrategyCooldown, sanitizeKeyComponent(userID), sanitizeKeyComponent(symbol))
}

// strategyHistoryKey generates the Redis key for selection history.
func strategyHistoryKey(userID, symbol string) string {
	return fmt.Sprintf(PrefixStrategyHistory, sanitizeKeyComponent(userID), sanitizeKeyComponent(symbol))
}

// userPreferenceKey generates the Redis key for user preference.
func userPreferenceKey(userID, symbol string) string {
	return fmt.Sprintf(PrefixUserPreference, sanitizeKeyComponent(userID), sanitizeKeyComponent(symbol))
}

// StrategySelector handles automatic strategy selection based on market regime.
// It integrates with the StrategyRegistry and provides:
// - Regime-based strategy matching
// - User preference overrides
// - Switch cooldown to prevent rapid switching
// - Selection history logging for analysis
type StrategySelector struct {
	registry *StrategyRegistry
	client   *redis.Client
	config   *SelectorConfig

	// In-memory cache for cooldown states (for fast access)
	cooldownCache   map[string]*CooldownState // key: "userID:symbol"
	cooldownCacheMu sync.RWMutex

	// User preferences cache
	userPreferences   map[string]string // key: "userID:symbol", value: strategy name
	userPreferencesMu sync.RWMutex
}

// NewStrategySelector creates a new StrategySelector with the given registry and Redis client.
// If config is nil, default configuration is used.
// Returns nil if registry is nil.
func NewStrategySelector(registry *StrategyRegistry, client *redis.Client, config *SelectorConfig) *StrategySelector {
	if registry == nil {
		log.Printf("[SELECTOR] Warning: NewStrategySelector called with nil registry")
		return nil
	}
	if config == nil {
		config = DefaultSelectorConfig()
	}

	return &StrategySelector{
		registry:        registry,
		client:          client,
		config:          config,
		cooldownCache:   make(map[string]*CooldownState),
		userPreferences: make(map[string]string),
	}
}

// GetConfig returns the current configuration (read-only copy).
func (ss *StrategySelector) GetConfig() SelectorConfig {
	return *ss.config
}

// GetRegistry returns the underlying StrategyRegistry.
func (ss *StrategySelector) GetRegistry() *StrategyRegistry {
	return ss.registry
}

// selectorCacheKey generates the in-memory cache key for a user/symbol pair.
func selectorCacheKey(userID, symbol string) string {
	return userID + ":" + symbol
}

// SelectStrategy selects the best strategy for the given coin state and user.
// It considers:
// 1. User preference override (if set)
// 2. Switch cooldown (if active, returns previous strategy)
// 3. Regime-based matching from registry
// 4. Fallback to default strategy
//
// Returns the selection details and any error.
func (ss *StrategySelector) SelectStrategy(ctx context.Context, state *CoinState, userID string) (*StrategySelection, error) {
	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	symbol := state.Symbol
	regime := state.Regime
	currentStrategy := state.ActiveStrategy

	// Check for user preference override
	if preference := ss.getUserPreference(ctx, userID, symbol); preference != "" {
		// Verify the preferred strategy exists and is enabled
		if ss.registry.HasStrategy(preference) && ss.registry.IsEnabled(preference) {
			selection := NewStrategySelection(symbol, userID, regime, preference, ReasonUserPreference)
			selection.PreviousStrategy = currentStrategy
			ss.logSelection(ctx, selection)
			return selection, nil
		}
		// Preference is invalid, clear it
		_ = ss.ClearUserPreference(ctx, userID, symbol)
	}

	// Check switch cooldown
	if ss.isCooldownActive(ctx, userID, symbol) {
		// Return the current strategy, cooldown is active
		if currentStrategy != "" {
			selection := NewStrategySelection(symbol, userID, regime, currentStrategy, ReasonCooldown)
			selection.CooldownActive = true
			ss.logSelection(ctx, selection)
			return selection, nil
		}
	}

	// Get enabled strategies for the current regime
	strategies := ss.registry.GetEnabledStrategiesForRegime(regime)
	availableNames := make([]string, len(strategies))
	for i, s := range strategies {
		availableNames[i] = s.Name()
	}

	var selectedStrategy string
	var selectedScore float64
	var reason SelectionReason

	if len(strategies) == 0 {
		// No strategies for this regime, use fallback
		selectedStrategy = ss.config.DefaultStrategy
		reason = ReasonFallback

		// Verify fallback exists
		if !ss.registry.HasStrategy(selectedStrategy) {
			// Try to get any default strategy from registry
			if defaultStrat, err := ss.registry.GetDefaultStrategy(); err == nil {
				selectedStrategy = defaultStrat.Name()
			} else {
				return nil, fmt.Errorf("no strategies available for regime %s and no fallback available", regime)
			}
		}
	} else if len(strategies) == 1 {
		// Only one strategy available for this regime
		selectedStrategy = strategies[0].Name()
		reason = ReasonRegimeMatch
	} else {
		// Multiple strategies available - select the best scoring one
		bestName, bestScore, err := ss.registry.SelectBestStrategy(state, regime)
		if err != nil {
			// Fallback to first available
			selectedStrategy = strategies[0].Name()
			reason = ReasonRegimeMatch
		} else {
			selectedStrategy = bestName
			selectedScore = bestScore.FinalScore
			reason = ReasonBestScore
		}
	}

	// Check if strategy actually changed
	strategyChanged := currentStrategy != "" && currentStrategy != selectedStrategy

	// Create selection record
	selection := NewStrategySelection(symbol, userID, regime, selectedStrategy, reason)
	selection.PreviousStrategy = currentStrategy
	selection.Score = selectedScore
	selection.AvailableStrategies = availableNames

	// Update cooldown if strategy changed
	if strategyChanged {
		ss.updateCooldown(ctx, userID, symbol, selectedStrategy)
	}

	// Log the selection
	ss.logSelection(ctx, selection)

	return selection, nil
}

// getUserPreference retrieves the user's strategy preference for a symbol.
// Returns empty string if no preference is set.
func (ss *StrategySelector) getUserPreference(ctx context.Context, userID, symbol string) string {
	key := selectorCacheKey(userID, symbol)

	// Check in-memory cache first
	ss.userPreferencesMu.RLock()
	if pref, ok := ss.userPreferences[key]; ok {
		ss.userPreferencesMu.RUnlock()
		return pref
	}
	ss.userPreferencesMu.RUnlock()

	// Check Redis if client is available
	if ss.client != nil {
		redisKey := userPreferenceKey(userID, symbol)
		result, err := ss.client.Get(ctx, redisKey).Result()
		if err == nil && result != "" {
			// Cache it
			ss.userPreferencesMu.Lock()
			ss.userPreferences[key] = result
			ss.userPreferencesMu.Unlock()
			return result
		}
	}

	return ""
}

// SetUserPreference sets the user's preferred strategy for a symbol.
// The preference will override regime-based selection.
func (ss *StrategySelector) SetUserPreference(ctx context.Context, userID, symbol, strategyName string) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}
	if strategyName == "" {
		return ss.ClearUserPreference(ctx, userID, symbol)
	}

	// Verify strategy exists
	if !ss.registry.HasStrategy(strategyName) {
		return fmt.Errorf("strategy '%s' not found in registry", strategyName)
	}

	key := selectorCacheKey(userID, symbol)

	// Update in-memory cache
	ss.userPreferencesMu.Lock()
	ss.userPreferences[key] = strategyName
	ss.userPreferencesMu.Unlock()

	// Update Redis if client is available
	if ss.client != nil {
		redisKey := userPreferenceKey(userID, symbol)
		// Use a long TTL for preferences (30 days)
		err := ss.client.Set(ctx, redisKey, strategyName, 30*24*time.Hour).Err()
		if err != nil {
			return fmt.Errorf("failed to save preference to Redis: %w", err)
		}
	}

	if ss.config.EnableLogging {
		log.Printf("[SELECTOR] %s: User %s set preference to %s", symbol, userID, strategyName)
	}

	return nil
}

// ClearUserPreference removes the user's strategy preference for a symbol.
func (ss *StrategySelector) ClearUserPreference(ctx context.Context, userID, symbol string) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}

	key := selectorCacheKey(userID, symbol)

	// Clear from in-memory cache
	ss.userPreferencesMu.Lock()
	delete(ss.userPreferences, key)
	ss.userPreferencesMu.Unlock()

	// Clear from Redis if client is available
	if ss.client != nil {
		redisKey := userPreferenceKey(userID, symbol)
		err := ss.client.Del(ctx, redisKey).Err()
		if err != nil && err != redis.Nil {
			return fmt.Errorf("failed to clear preference from Redis: %w", err)
		}
	}

	if ss.config.EnableLogging {
		log.Printf("[SELECTOR] %s: User %s cleared preference", symbol, userID)
	}

	return nil
}

// HasUserPreference checks if a user has a strategy preference for a symbol.
func (ss *StrategySelector) HasUserPreference(ctx context.Context, userID, symbol string) bool {
	return ss.getUserPreference(ctx, userID, symbol) != ""
}

// isCooldownActive checks if the strategy switch cooldown is active for a symbol.
func (ss *StrategySelector) isCooldownActive(ctx context.Context, userID, symbol string) bool {
	key := selectorCacheKey(userID, symbol)

	// Check in-memory cache first
	ss.cooldownCacheMu.RLock()
	cooldown, ok := ss.cooldownCache[key]
	ss.cooldownCacheMu.RUnlock()

	if ok {
		elapsed := time.Since(time.UnixMilli(cooldown.LastSwitchTime))
		return elapsed < ss.config.CooldownDuration
	}

	// Check Redis if client is available
	if ss.client != nil {
		redisKey := strategyCooldownKey(userID, symbol)
		result, err := ss.client.Get(ctx, redisKey).Result()
		if err == nil && result != "" {
			var state CooldownState
			if err := json.Unmarshal([]byte(result), &state); err == nil {
				// Cache it
				ss.cooldownCacheMu.Lock()
				ss.cooldownCache[key] = &state
				ss.cooldownCacheMu.Unlock()

				elapsed := time.Since(time.UnixMilli(state.LastSwitchTime))
				return elapsed < ss.config.CooldownDuration
			}
		}
	}

	return false
}

// updateCooldown records a strategy switch and starts the cooldown.
func (ss *StrategySelector) updateCooldown(ctx context.Context, userID, symbol, newStrategy string) {
	key := selectorCacheKey(userID, symbol)
	now := time.Now().UnixMilli()

	cooldown := &CooldownState{
		Symbol:         symbol,
		LastStrategy:   newStrategy,
		LastSwitchTime: now,
	}

	// Update in-memory cache
	ss.cooldownCacheMu.Lock()
	ss.cooldownCache[key] = cooldown
	ss.cooldownCacheMu.Unlock()

	// Update Redis if client is available
	if ss.client != nil {
		data, err := json.Marshal(cooldown)
		if err != nil {
			log.Printf("[SELECTOR] Warning: Failed to marshal cooldown state: %v", err)
		} else {
			redisKey := strategyCooldownKey(userID, symbol)
			// TTL should be slightly longer than cooldown duration
			ttl := ss.config.CooldownDuration + time.Minute
			if err := ss.client.Set(ctx, redisKey, string(data), ttl).Err(); err != nil {
				log.Printf("[SELECTOR] Warning: Failed to save cooldown to Redis: %v", err)
			}
		}
	}

	if ss.config.EnableLogging {
		log.Printf("[SELECTOR] %s: Strategy switched to %s, cooldown started for %v",
			symbol, newStrategy, ss.config.CooldownDuration)
	}
}

// GetCooldownRemaining returns the remaining cooldown time for a symbol.
// Returns 0 if no cooldown is active.
func (ss *StrategySelector) GetCooldownRemaining(ctx context.Context, userID, symbol string) time.Duration {
	key := selectorCacheKey(userID, symbol)

	ss.cooldownCacheMu.RLock()
	cooldown, ok := ss.cooldownCache[key]
	ss.cooldownCacheMu.RUnlock()

	if !ok {
		// Try Redis
		if ss.client != nil {
			redisKey := strategyCooldownKey(userID, symbol)
			result, err := ss.client.Get(ctx, redisKey).Result()
			if err == nil && result != "" {
				var state CooldownState
				if err := json.Unmarshal([]byte(result), &state); err == nil {
					cooldown = &state
					// Cache the result from Redis for future calls
					ss.cooldownCacheMu.Lock()
					ss.cooldownCache[key] = cooldown
					ss.cooldownCacheMu.Unlock()
				}
			}
		}
	}

	if cooldown == nil {
		return 0
	}

	elapsed := time.Since(time.UnixMilli(cooldown.LastSwitchTime))
	if elapsed >= ss.config.CooldownDuration {
		return 0
	}

	return ss.config.CooldownDuration - elapsed
}

// ClearCooldown manually clears the cooldown for a symbol.
// This allows immediate strategy switches.
func (ss *StrategySelector) ClearCooldown(ctx context.Context, userID, symbol string) error {
	key := selectorCacheKey(userID, symbol)

	// Clear from in-memory cache
	ss.cooldownCacheMu.Lock()
	delete(ss.cooldownCache, key)
	ss.cooldownCacheMu.Unlock()

	// Clear from Redis if client is available
	if ss.client != nil {
		redisKey := strategyCooldownKey(userID, symbol)
		err := ss.client.Del(ctx, redisKey).Err()
		if err != nil && err != redis.Nil {
			return fmt.Errorf("failed to clear cooldown from Redis: %w", err)
		}
	}

	if ss.config.EnableLogging {
		log.Printf("[SELECTOR] %s: Cooldown cleared for user %s", symbol, userID)
	}

	return nil
}

// logSelection logs a strategy selection to Redis for analysis.
func (ss *StrategySelector) logSelection(ctx context.Context, selection *StrategySelection) {
	if selection == nil {
		return
	}

	// Log to console if enabled
	if ss.config.EnableLogging {
		log.Printf("[SELECTOR] %s: Selected %s for regime %s (reason: %s)",
			selection.Symbol, selection.SelectedStrategy, selection.Regime, selection.Reason)
	}

	// Log to Redis sorted set if client is available
	if ss.client != nil {
		redisKey := strategyHistoryKey(selection.UserID, selection.Symbol)
		data := selection.ToJSON()

		// Add to sorted set with timestamp as score
		err := ss.client.ZAdd(ctx, redisKey, redis.Z{
			Score:  float64(selection.Timestamp),
			Member: data,
		}).Err()

		if err != nil {
			log.Printf("[SELECTOR] Warning: Failed to log selection to Redis: %v", err)
			return
		}

		// Set TTL on the sorted set
		if err := ss.client.Expire(ctx, redisKey, ss.config.SelectionHistoryTTL).Err(); err != nil {
			log.Printf("[SELECTOR] Warning: Failed to set TTL on selection history: %v", err)
		}

		// Trim to max entries (keep most recent)
		if err := ss.client.ZRemRangeByRank(ctx, redisKey, 0, int64(-ss.config.MaxHistoryEntries-1)).Err(); err != nil {
			log.Printf("[SELECTOR] Warning: Failed to trim selection history: %v", err)
		}
	}
}

// GetSelectionHistory retrieves the selection history for a symbol.
// Returns up to 'limit' most recent selections.
func (ss *StrategySelector) GetSelectionHistory(ctx context.Context, userID, symbol string, limit int) ([]StrategySelection, error) {
	if userID == "" || symbol == "" {
		return nil, fmt.Errorf("userID and symbol are required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > ss.config.MaxHistoryEntries {
		limit = ss.config.MaxHistoryEntries
	}

	if ss.client == nil {
		return []StrategySelection{}, nil
	}

	redisKey := strategyHistoryKey(userID, symbol)

	// Get most recent entries (highest scores = most recent timestamps)
	results, err := ss.client.ZRevRangeWithScores(ctx, redisKey, 0, int64(limit-1)).Result()
	if err != nil {
		if err == redis.Nil {
			return []StrategySelection{}, nil
		}
		return nil, fmt.Errorf("failed to get selection history: %w", err)
	}

	selections := make([]StrategySelection, 0, len(results))
	for _, z := range results {
		var selection StrategySelection
		if err := json.Unmarshal([]byte(z.Member.(string)), &selection); err == nil {
			selections = append(selections, selection)
		}
	}

	return selections, nil
}

// GetSelectionHistoryCount returns the number of selection history entries for a symbol.
func (ss *StrategySelector) GetSelectionHistoryCount(ctx context.Context, userID, symbol string) (int64, error) {
	if userID == "" || symbol == "" {
		return 0, fmt.Errorf("userID and symbol are required")
	}
	if ss.client == nil {
		return 0, nil
	}

	redisKey := strategyHistoryKey(userID, symbol)
	count, err := ss.client.ZCard(ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get history count: %w", err)
	}

	return count, nil
}

// ClearSelectionHistory removes all selection history for a symbol.
func (ss *StrategySelector) ClearSelectionHistory(ctx context.Context, userID, symbol string) error {
	if ss.client == nil {
		return nil
	}

	redisKey := strategyHistoryKey(userID, symbol)
	err := ss.client.Del(ctx, redisKey).Err()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to clear selection history: %w", err)
	}

	if ss.config.EnableLogging {
		log.Printf("[SELECTOR] %s: Selection history cleared for user %s", symbol, userID)
	}

	return nil
}

// GetSelectionStats returns statistics about strategy selections for a user.
func (ss *StrategySelector) GetSelectionStats(ctx context.Context, userID string, symbols []string) (*SelectionStats, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	stats := &SelectionStats{
		UserID:          userID,
		SelectionsByReason:  make(map[SelectionReason]int),
		SelectionsByRegime:  make(map[MarketRegime]int),
		SelectionsByStrategy: make(map[string]int),
		LastUpdated:     time.Now().UnixMilli(),
	}

	for _, symbol := range symbols {
		history, err := ss.GetSelectionHistory(ctx, userID, symbol, ss.config.MaxHistoryEntries)
		if err != nil {
			continue
		}

		for _, selection := range history {
			stats.TotalSelections++
			stats.SelectionsByReason[selection.Reason]++
			stats.SelectionsByRegime[selection.Regime]++
			stats.SelectionsByStrategy[selection.SelectedStrategy]++

			if selection.CooldownActive {
				stats.CooldownTriggers++
			}
		}
	}

	// Find most common strategy
	maxCount := 0
	for strategy, count := range stats.SelectionsByStrategy {
		if count > maxCount {
			maxCount = count
			stats.MostUsedStrategy = strategy
		}
	}

	return stats, nil
}

// SelectionStats provides aggregate statistics about strategy selections.
type SelectionStats struct {
	// UserID is the user these stats belong to.
	UserID string `json:"user_id"`
	// TotalSelections is the total number of selections recorded.
	TotalSelections int `json:"total_selections"`
	// SelectionsByReason maps selection reasons to counts.
	SelectionsByReason map[SelectionReason]int `json:"selections_by_reason"`
	// SelectionsByRegime maps market regimes to selection counts.
	SelectionsByRegime map[MarketRegime]int `json:"selections_by_regime"`
	// SelectionsByStrategy maps strategy names to selection counts.
	SelectionsByStrategy map[string]int `json:"selections_by_strategy"`
	// CooldownTriggers is how many times cooldown prevented a switch.
	CooldownTriggers int `json:"cooldown_triggers"`
	// MostUsedStrategy is the most frequently selected strategy.
	MostUsedStrategy string `json:"most_used_strategy"`
	// LastUpdated is when these stats were computed.
	LastUpdated int64 `json:"last_updated"`
}

// ClearAllUserData removes all selector data for a user (preferences, cooldowns, history).
func (ss *StrategySelector) ClearAllUserData(ctx context.Context, userID string, symbols []string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	for _, symbol := range symbols {
		_ = ss.ClearUserPreference(ctx, userID, symbol)
		_ = ss.ClearCooldown(ctx, userID, symbol)
		_ = ss.ClearSelectionHistory(ctx, userID, symbol)
	}

	return nil
}
