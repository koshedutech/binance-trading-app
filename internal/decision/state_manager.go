// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.1: Redis State Management
package decision

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// StateManager handles Redis operations for coin state management.
// It provides delta updates, TTL management, and atomic operations.
type StateManager struct {
	client *redis.Client
	ttl    time.Duration
}

// NewStateManager creates a new StateManager with the given Redis client.
// Returns nil if client is nil - caller must check.
func NewStateManager(client *redis.Client) *StateManager {
	if client == nil {
		log.Printf("[DECISION] Warning: NewStateManager called with nil client")
		return nil
	}
	return &StateManager{
		client: client,
		ttl:    DefaultCoinStateTTL,
	}
}

// NewStateManagerWithTTL creates a new StateManager with a custom TTL.
// Returns nil if client is nil - caller must check.
func NewStateManagerWithTTL(client *redis.Client, ttl time.Duration) *StateManager {
	if client == nil {
		log.Printf("[DECISION] Warning: NewStateManagerWithTTL called with nil client")
		return nil
	}
	if ttl <= 0 {
		ttl = DefaultCoinStateTTL
	}
	return &StateManager{
		client: client,
		ttl:    ttl,
	}
}

// GetCoinState retrieves the full coin state for a user and symbol.
// Returns nil, nil if the key doesn't exist.
func (sm *StateManager) GetCoinState(ctx context.Context, userID, symbol string) (*CoinState, error) {
	if userID == "" || symbol == "" {
		return nil, fmt.Errorf("userID and symbol are required")
	}

	key := CoinStateKey(userID, symbol)

	// Use HGetAll to get all fields
	result, err := sm.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get coin state from Redis: %w", err)
	}

	// Check if key exists (HGetAll returns empty map for non-existent keys)
	if len(result) == 0 {
		return nil, nil
	}

	// Parse the result into CoinState
	coinState, err := FromMap(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse coin state: %w", err)
	}

	return coinState, nil
}

// SetCoinState stores the complete coin state (overwrites all fields).
// Use UpdateCoinState for delta updates instead.
func (sm *StateManager) SetCoinState(ctx context.Context, userID, symbol string, state *CoinState) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}
	if state == nil {
		return fmt.Errorf("coin state cannot be nil")
	}

	// Validate state before storing
	if err := state.Validate(); err != nil {
		return fmt.Errorf("invalid coin state: %w", err)
	}

	key := CoinStateKey(userID, symbol)

	// Update timestamp
	state.UpdateTimestamp()

	// Convert to map for HSet
	data := state.ToMap()

	// Use pipeline for atomic set + expire
	pipe := sm.client.Pipeline()

	// HSet all fields
	pipe.HSet(ctx, key, data)

	// Set TTL
	pipe.Expire(ctx, key, sm.ttl)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set coin state in Redis: %w", err)
	}

	return nil
}

// UpdateCoinState performs a delta update on specific fields.
// Only the provided fields are updated; other fields remain unchanged.
// This is the preferred method for updates to minimize network traffic.
func (sm *StateManager) UpdateCoinState(ctx context.Context, userID, symbol string, updates map[string]interface{}) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}
	if len(updates) == 0 {
		return nil // Nothing to update
	}

	key := CoinStateKey(userID, symbol)

	// Always update the timestamp on any update
	updates["last_updated"] = fmt.Sprintf("%d", time.Now().UnixMilli())

	// Use pipeline for atomic HSet + Expire
	pipe := sm.client.Pipeline()

	// HSet updates only the specified fields
	pipe.HSet(ctx, key, updates)

	// Reset TTL on any update
	pipe.Expire(ctx, key, sm.ttl)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update coin state in Redis: %w", err)
	}

	return nil
}

// SetCoinDecision updates only the decision field.
// This is a convenience method for quick decision updates.
func (sm *StateManager) SetCoinDecision(ctx context.Context, userID, symbol string, decision Decision) error {
	return sm.UpdateCoinState(ctx, userID, symbol, map[string]interface{}{
		"decision": string(decision),
	})
}

// SetBlockingReasons updates the blocking_reasons field and sets decision to BLOCKED if reasons exist.
func (sm *StateManager) SetBlockingReasons(ctx context.Context, userID, symbol string, reasons []string) error {
	reasonsJSON, err := json.Marshal(reasons)
	if err != nil {
		return fmt.Errorf("failed to marshal blocking reasons: %w", err)
	}

	updates := map[string]interface{}{
		"blocking_reasons": string(reasonsJSON),
	}

	// Automatically set decision based on blocking reasons
	if len(reasons) > 0 {
		updates["decision"] = string(DecisionBlocked)
	} else {
		updates["decision"] = string(DecisionReady)
	}

	return sm.UpdateCoinState(ctx, userID, symbol, updates)
}

// DeleteCoinState removes the coin state completely from Redis.
func (sm *StateManager) DeleteCoinState(ctx context.Context, userID, symbol string) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}

	key := CoinStateKey(userID, symbol)

	err := sm.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete coin state from Redis: %w", err)
	}

	return nil
}

// GetAllCoinStates retrieves all coin states for a user.
// Uses SCAN to find all matching keys, then fetches each state.
func (sm *StateManager) GetAllCoinStates(ctx context.Context, userID string) ([]*CoinState, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	pattern := CoinStatePattern(userID)
	var states []*CoinState

	// Use SCAN to find all matching keys
	iter := sm.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()

		// Get the state for this key
		result, err := sm.client.HGetAll(ctx, key).Result()
		if err != nil {
			log.Printf("[DECISION] Failed to get state for key %s: %v", key, err)
			continue
		}

		if len(result) == 0 {
			continue // Key was deleted between scan and get
		}

		state, err := FromMap(result)
		if err != nil {
			log.Printf("[DECISION] Failed to parse state for key %s: %v", key, err)
			continue
		}

		states = append(states, state)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan coin states: %w", err)
	}

	return states, nil
}

// GetCoinStates retrieves multiple coin states by symbols in a single operation.
// Uses pipelining for efficiency.
func (sm *StateManager) GetCoinStates(ctx context.Context, userID string, symbols []string) ([]*CoinState, error) {
	if len(symbols) == 0 {
		return []*CoinState{}, nil
	}

	// Use pipeline to fetch all states in one round trip
	pipe := sm.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(symbols))

	for i, symbol := range symbols {
		key := CoinStateKey(userID, symbol)
		cmds[i] = pipe.HGetAll(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get coin states: %w", err)
	}

	states := make([]*CoinState, 0, len(symbols))
	for i, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil {
			log.Printf("[DECISION] Failed to get state for %s: %v", symbols[i], err)
			continue
		}

		if len(result) == 0 {
			continue // Key doesn't exist
		}

		state, err := FromMap(result)
		if err != nil {
			log.Printf("[DECISION] Failed to parse state for %s: %v", symbols[i], err)
			continue
		}

		states = append(states, state)
	}

	return states, nil
}

// UpdatePrice updates only the price field with TTL refresh.
// This is a high-frequency operation optimized for speed.
func (sm *StateManager) UpdatePrice(ctx context.Context, userID, symbol string, price float64) error {
	return sm.UpdateCoinState(ctx, userID, symbol, map[string]interface{}{
		"price": formatFloat(price),
	})
}

// UpdateIndicators updates technical indicator fields.
func (sm *StateManager) UpdateIndicators(ctx context.Context, userID, symbol string, adx, rsi, ema9, ema21 float64) error {
	return sm.UpdateCoinState(ctx, userID, symbol, map[string]interface{}{
		"adx":    formatFloat(adx),
		"rsi":    formatFloat(rsi),
		"ema_9":  formatFloat(ema9),
		"ema_21": formatFloat(ema21),
	})
}

// UpdateScores updates all scoring components.
func (sm *StateManager) UpdateScores(ctx context.Context, userID, symbol string, technical, context_, llm, history, final int) error {
	return sm.UpdateCoinState(ctx, userID, symbol, map[string]interface{}{
		"score_technical": fmt.Sprintf("%d", technical),
		"score_context":   fmt.Sprintf("%d", context_),
		"score_llm":       fmt.Sprintf("%d", llm),
		"score_history":   fmt.Sprintf("%d", history),
		"score_final":     fmt.Sprintf("%d", final),
	})
}

// UpdateTrends updates trend direction fields.
func (sm *StateManager) UpdateTrends(ctx context.Context, userID, symbol string, trend1h, trend15m TrendDirection) error {
	return sm.UpdateCoinState(ctx, userID, symbol, map[string]interface{}{
		"trend_1h":  string(trend1h),
		"trend_15m": string(trend15m),
	})
}

// UpdateRegime updates the market regime and active strategy.
func (sm *StateManager) UpdateRegime(ctx context.Context, userID, symbol string, regime MarketRegime, strategy string) error {
	return sm.UpdateCoinState(ctx, userID, symbol, map[string]interface{}{
		"regime":          string(regime),
		"active_strategy": strategy,
	})
}

// Exists checks if a coin state exists for the given user and symbol.
func (sm *StateManager) Exists(ctx context.Context, userID, symbol string) (bool, error) {
	key := CoinStateKey(userID, symbol)

	count, err := sm.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return count > 0, nil
}

// RefreshTTL resets the TTL for a coin state without modifying any data.
func (sm *StateManager) RefreshTTL(ctx context.Context, userID, symbol string) error {
	key := CoinStateKey(userID, symbol)

	err := sm.client.Expire(ctx, key, sm.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to refresh TTL: %w", err)
	}

	return nil
}

// GetTTL returns the remaining TTL for a coin state.
// Returns -2 if the key doesn't exist, -1 if no TTL is set.
func (sm *StateManager) GetTTL(ctx context.Context, userID, symbol string) (time.Duration, error) {
	key := CoinStateKey(userID, symbol)

	ttl, err := sm.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL: %w", err)
	}

	return ttl, nil
}

// DeleteAllUserStates removes all coin states for a user.
// Use with caution - this is typically called on user logout or cleanup.
func (sm *StateManager) DeleteAllUserStates(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID is required")
	}

	pattern := CoinStatePattern(userID)

	// Use SCAN to find all matching keys
	iter := sm.client.Scan(ctx, 0, pattern, 100).Iterator()
	var keysToDelete []string

	for iter.Next(ctx) {
		keysToDelete = append(keysToDelete, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys for deletion: %w", err)
	}

	if len(keysToDelete) == 0 {
		return nil // Nothing to delete
	}

	// Delete all found keys
	err := sm.client.Del(ctx, keysToDelete...).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user states: %w", err)
	}

	log.Printf("[DECISION] Deleted %d coin states for user %s", len(keysToDelete), userID)
	return nil
}

// CountUserStates returns the number of coin states for a user.
func (sm *StateManager) CountUserStates(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("userID is required")
	}

	pattern := CoinStatePattern(userID)
	var count int

	iter := sm.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		count++
	}

	if err := iter.Err(); err != nil {
		return 0, fmt.Errorf("failed to count states: %w", err)
	}

	return count, nil
}

// Score History Management for Gap Analysis UI (Story 11.40)
// Stores 8 hours of 5-minute score samples (96 data points max)

const (
	// PrefixScoreHistory is the Redis key pattern for score history lists
	// Format: decision:history:{userID}:{symbol}
	PrefixScoreHistory = "decision:history:%s:%s"
	// ScoreHistoryTTL is the TTL for score history data (8 hours)
	ScoreHistoryTTL = 8 * time.Hour
	// MaxScoreHistoryEntries is the max number of entries to keep (8 hours * 12 per hour = 96)
	MaxScoreHistoryEntries = 96
)

// ScoreHistoryKey generates the Redis key for a symbol's score history.
func ScoreHistoryKey(userID, symbol string) string {
	userID = sanitizeKeyComponent(userID)
	symbol = sanitizeKeyComponent(symbol)
	return fmt.Sprintf(PrefixScoreHistory, userID, symbol)
}

// ScoreHistoryEntry represents a single score entry in history
type ScoreHistoryEntry struct {
	Timestamp int64 `json:"t"`
	Score     int   `json:"s"`
}

// AddScoreToHistory appends a score to the symbol's history list.
// Keeps only the most recent MaxScoreHistoryEntries entries.
func (sm *StateManager) AddScoreToHistory(ctx context.Context, userID, symbol string, score int) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}

	key := ScoreHistoryKey(userID, symbol)
	timestamp := time.Now().UnixMilli()

	// Create entry as JSON
	entry, err := json.Marshal(ScoreHistoryEntry{
		Timestamp: timestamp,
		Score:     score,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal score entry: %w", err)
	}

	// Use pipeline for atomic operations
	pipe := sm.client.Pipeline()

	// Push to list (right side = newest)
	pipe.RPush(ctx, key, string(entry))

	// Trim to keep only the most recent entries
	pipe.LTrim(ctx, key, -MaxScoreHistoryEntries, -1)

	// Set/refresh TTL
	pipe.Expire(ctx, key, ScoreHistoryTTL)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add score to history: %w", err)
	}

	return nil
}

// GetScoreHistory retrieves the score history for a symbol.
// Returns a ScoreHistoryForUI optimized for frontend display.
func (sm *StateManager) GetScoreHistory(ctx context.Context, userID, symbol string) (*ScoreHistoryForUI, error) {
	if userID == "" || symbol == "" {
		return nil, fmt.Errorf("userID and symbol are required")
	}

	key := ScoreHistoryKey(userID, symbol)

	// Get all entries from the list
	entries, err := sm.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get score history: %w", err)
	}

	history := &ScoreHistoryForUI{
		Timestamps: make([]int64, 0, len(entries)),
		Scores:     make([]int, 0, len(entries)),
	}

	for _, entry := range entries {
		var e ScoreHistoryEntry
		if err := json.Unmarshal([]byte(entry), &e); err != nil {
			log.Printf("[DECISION] Failed to parse score history entry: %v", err)
			continue
		}
		history.Timestamps = append(history.Timestamps, e.Timestamp)
		history.Scores = append(history.Scores, e.Score)
	}

	// Calculate trend
	history.CalculateTrend()

	return history, nil
}

// ClearScoreHistory removes all score history for a symbol.
func (sm *StateManager) ClearScoreHistory(ctx context.Context, userID, symbol string) error {
	if userID == "" || symbol == "" {
		return fmt.Errorf("userID and symbol are required")
	}

	key := ScoreHistoryKey(userID, symbol)
	return sm.client.Del(ctx, key).Err()
}
