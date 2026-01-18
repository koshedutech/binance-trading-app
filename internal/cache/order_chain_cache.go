// Package cache provides Redis-based caching for order chains.
// Epic 7: Client Order ID & Trade Lifecycle Tracking
// Story 7.20: Order Chain Redis Cache Layer
//
// This file implements the Redis cache layer for active order chains,
// providing fast reads for UI and reducing PostgreSQL load during active trading.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// OrderChainCache provides caching operations for order chains.
// It uses the underlying CacheService for Redis operations.
type OrderChainCache struct {
	cache  *CacheService
	logger Logger
}

// NewOrderChainCache creates a new OrderChainCache instance
func NewOrderChainCache(cache *CacheService, logger Logger) *OrderChainCache {
	return &OrderChainCache{
		cache:  cache,
		logger: logger,
	}
}

// === Chain State Operations ===

// SetChainState caches the current chain state in Redis.
// Key: order_chain:{user_id}:{chain_id}
// TTL: None (explicit delete on close)
func (c *OrderChainCache) SetChainState(ctx context.Context, chain *OrderChainState) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		c.logger.Warn("Cache unavailable, skipping SetChainState")
		return nil
	}

	key := OrderChainKey(chain.UserID, chain.ChainID)
	data, err := json.Marshal(chain)
	if err != nil {
		return fmt.Errorf("failed to marshal chain state: %w", err)
	}

	// No TTL for active chains - explicit delete on close
	if err := c.cache.Set(ctx, key, string(data), 0); err != nil {
		c.logger.Error("Failed to cache chain state", "chain_id", chain.ChainID, "error", err)
		return err
	}

	c.logger.Debug("Chain state cached", "chain_id", chain.ChainID, "status", chain.Status)

	return nil
}

// GetChainState retrieves cached chain state from Redis.
// Returns nil, nil if cache miss (not an error).
func (c *OrderChainCache) GetChainState(ctx context.Context, userID, chainID string) (*OrderChainState, error) {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil, nil // Graceful degradation
	}

	key := OrderChainKey(userID, chainID)
	data, err := c.cache.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		c.logger.Debug("Cache miss for chain state", "chain_id", chainID, "error", err)
		return nil, nil // Return nil on any error for graceful degradation
	}

	var chain OrderChainState
	if err := json.Unmarshal([]byte(data), &chain); err != nil {
		c.logger.Error("Failed to unmarshal cached chain state", "chain_id", chainID, "error", err)
		return nil, err
	}

	return &chain, nil
}

// DeleteChainState removes chain from cache (called on close).
// Also removes from active chains set.
func (c *OrderChainCache) DeleteChainState(ctx context.Context, userID, chainID string) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil
	}

	// Delete the chain state
	key := OrderChainKey(userID, chainID)
	if err := c.cache.Delete(ctx, key); err != nil {
		c.logger.Warn("Failed to delete chain state from cache", "chain_id", chainID, "error", err)
	}

	// Remove from active chains set
	if err := c.RemoveFromActiveChains(ctx, userID, chainID); err != nil {
		c.logger.Warn("Failed to remove from active chains set", "chain_id", chainID, "error", err)
	}

	c.logger.Debug("Chain state deleted from cache", "chain_id", chainID)

	return nil
}

// === Active Chains Index Operations ===

// AddToActiveChains adds a chain ID to the user's active chains set.
// Key: order_chains:active:{user_id}
// TTL: None
func (c *OrderChainCache) AddToActiveChains(ctx context.Context, userID, chainID string) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil
	}

	key := ActiveChainsKey(userID)
	client := c.cache.GetClient()
	if client == nil {
		return nil
	}

	if err := client.SAdd(ctx, key, chainID).Err(); err != nil {
		c.logger.Warn("Failed to add to active chains set", "chain_id", chainID, "error", err)
		return err
	}

	c.logger.Debug("Added to active chains set", "user_id", userID, "chain_id", chainID)

	return nil
}

// RemoveFromActiveChains removes a chain ID from the user's active chains set.
func (c *OrderChainCache) RemoveFromActiveChains(ctx context.Context, userID, chainID string) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil
	}

	key := ActiveChainsKey(userID)
	client := c.cache.GetClient()
	if client == nil {
		return nil
	}

	if err := client.SRem(ctx, key, chainID).Err(); err != nil {
		c.logger.Warn("Failed to remove from active chains set", "chain_id", chainID, "error", err)
		return err
	}

	c.logger.Debug("Removed from active chains set", "user_id", userID, "chain_id", chainID)

	return nil
}

// GetActiveChainIDs returns all active chain IDs for a user.
func (c *OrderChainCache) GetActiveChainIDs(ctx context.Context, userID string) ([]string, error) {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil, nil
	}

	key := ActiveChainsKey(userID)
	client := c.cache.GetClient()
	if client == nil {
		return nil, nil
	}

	chainIDs, err := client.SMembers(ctx, key).Result()
	if err != nil {
		c.logger.Warn("Failed to get active chain IDs", "user_id", userID, "error", err)
		return nil, nil // Graceful degradation
	}

	return chainIDs, nil
}

// === Batch Operations ===

// GetMultipleChains retrieves multiple chain states by their IDs.
// Uses Redis MGET for efficiency.
func (c *OrderChainCache) GetMultipleChains(ctx context.Context, userID string, chainIDs []string) ([]*OrderChainState, error) {
	if c.cache == nil || !c.cache.IsHealthy() || len(chainIDs) == 0 {
		return nil, nil
	}

	// Build keys
	keys := make([]string, len(chainIDs))
	for i, id := range chainIDs {
		keys[i] = OrderChainKey(userID, id)
	}

	values, err := c.cache.MGet(ctx, keys...)
	if err != nil {
		c.logger.Warn("Failed to get multiple chains from cache", "error", err)
		return nil, nil
	}

	chains := make([]*OrderChainState, 0, len(values))
	for i, v := range values {
		if v == nil {
			continue
		}
		str, ok := v.(string)
		if !ok {
			continue
		}
		var chain OrderChainState
		if err := json.Unmarshal([]byte(str), &chain); err != nil {
			c.logger.Warn("Failed to unmarshal cached chain", "chain_id", chainIDs[i], "error", err)
			continue
		}
		chains = append(chains, &chain)
	}

	return chains, nil
}

// GetAllActiveChainsForUser retrieves all active chains for a user.
// First gets the chain IDs from the active set, then fetches each chain.
func (c *OrderChainCache) GetAllActiveChainsForUser(ctx context.Context, userID string) ([]*OrderChainState, error) {
	chainIDs, err := c.GetActiveChainIDs(ctx, userID)
	if err != nil || len(chainIDs) == 0 {
		return nil, err
	}

	return c.GetMultipleChains(ctx, userID, chainIDs)
}

// === Recent Events Operations (for WebSocket) ===

// PushRecentEvent adds an event to the user's recent events list.
// Key: order_chain_events:recent:{user_id}
// TTL: 1 hour
// List is capped at RecentEventsMaxLength
func (c *OrderChainCache) PushRecentEvent(ctx context.Context, userID string, event *ChainEventSummary) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil
	}

	key := RecentEventsKey(userID)
	client := c.cache.GetClient()
	if client == nil {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event summary: %w", err)
	}

	// Use pipeline for atomic operations
	pipe := client.Pipeline()
	pipe.LPush(ctx, key, string(data))
	pipe.LTrim(ctx, key, 0, RecentEventsMaxLength-1) // Keep only the most recent
	pipe.Expire(ctx, key, RecentEventsTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		c.logger.Warn("Failed to push recent event", "user_id", userID, "error", err)
		return err
	}

	return nil
}

// GetRecentEvents returns recent events for a user (for WebSocket catch-up).
func (c *OrderChainCache) GetRecentEvents(ctx context.Context, userID string, limit int) ([]*ChainEventSummary, error) {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil, nil
	}

	if limit <= 0 || limit > RecentEventsMaxLength {
		limit = RecentEventsMaxLength
	}

	key := RecentEventsKey(userID)
	client := c.cache.GetClient()
	if client == nil {
		return nil, nil
	}

	items, err := client.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		c.logger.Warn("Failed to get recent events", "user_id", userID, "error", err)
		return nil, nil
	}

	events := make([]*ChainEventSummary, 0, len(items))
	for _, item := range items {
		var event ChainEventSummary
		if err := json.Unmarshal([]byte(item), &event); err != nil {
			c.logger.Warn("Failed to unmarshal recent event", "error", err)
			continue
		}
		events = append(events, &event)
	}

	return events, nil
}

// === Warm-up Operations ===

// WarmCacheForUser loads all active chains for a user into cache.
// Called during startup to pre-populate the cache.
func (c *OrderChainCache) WarmCacheForUser(ctx context.Context, userID string, chains []*OrderChainState) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil
	}

	client := c.cache.GetClient()
	if client == nil {
		return nil
	}

	// Use pipeline for efficiency
	pipe := client.Pipeline()

	// Clear old active set first
	activeKey := ActiveChainsKey(userID)
	pipe.Del(ctx, activeKey)

	chainCount := 0
	for _, chain := range chains {
		// Only cache active/partial chains
		if chain.Status != "ACTIVE" && chain.Status != "PARTIAL" {
			continue
		}

		// Cache the chain state
		key := OrderChainKey(userID, chain.ChainID)
		data, err := json.Marshal(chain)
		if err != nil {
			c.logger.Warn("Failed to marshal chain for warming", "chain_id", chain.ChainID, "error", err)
			continue
		}

		pipe.Set(ctx, key, string(data), 0) // No TTL
		pipe.SAdd(ctx, activeKey, chain.ChainID)
		chainCount++
	}

	if _, err := pipe.Exec(ctx); err != nil {
		c.logger.Error("Failed to warm cache for user", "user_id", userID, "error", err)
		return err
	}

	c.logger.Info("Cache warmed for user", "user_id", userID, "chains_cached", chainCount)

	return nil
}

// === Cleanup Operations ===

// InvalidateUserCache removes all cached data for a user (on logout).
// This is optional cleanup - data has no TTL so will persist otherwise.
func (c *OrderChainCache) InvalidateUserCache(ctx context.Context, userID string) error {
	if c.cache == nil || !c.cache.IsHealthy() {
		return nil
	}

	// Get all active chain IDs first
	chainIDs, _ := c.GetActiveChainIDs(ctx, userID)

	client := c.cache.GetClient()
	if client == nil {
		return nil
	}

	// Build list of keys to delete
	keysToDelete := []string{
		ActiveChainsKey(userID),
		RecentEventsKey(userID),
	}

	for _, chainID := range chainIDs {
		keysToDelete = append(keysToDelete, OrderChainKey(userID, chainID))
	}

	// Delete all keys
	if len(keysToDelete) > 0 {
		if err := client.Del(ctx, keysToDelete...).Err(); err != nil {
			c.logger.Warn("Failed to invalidate user cache", "user_id", userID, "error", err)
			return err
		}
	}

	c.logger.Info("User cache invalidated", "user_id", userID, "keys_deleted", len(keysToDelete))

	return nil
}

// === Utility Functions ===

// IsHealthy returns whether the underlying cache is healthy
func (c *OrderChainCache) IsHealthy() bool {
	return c.cache != nil && c.cache.IsHealthy()
}

// FormatTimestamp formats a time.Time for JSON storage
func FormatTimestamp(t time.Time) string {
	return t.Format(time.RFC3339)
}

// FormatTimestampPtr formats a time pointer for JSON storage
func FormatTimestampPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}
