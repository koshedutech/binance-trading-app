package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"binance-trading-bot/internal/database"

	"github.com/redis/go-redis/v9"
)

// =====================================================
// STRATEGY HIERARCHY CACHE SERVICE
// Story 11.44: Volume Imbalance Database Schema & Repository
// Provides Redis caching for strategy hierarchy settings
// =====================================================

// TTL for strategy hierarchy cache entries
const (
	StrategyHierarchyTTL = 1 * time.Hour
)

// StrategyHierarchyCacheService provides cache access to strategy hierarchy settings
// Uses cache-first read pattern with database fallback
type StrategyHierarchyCacheService struct {
	cache  *CacheService
	repo   *database.Repository
	logger Logger
}

// NewStrategyHierarchyCacheService creates a new strategy hierarchy cache service
func NewStrategyHierarchyCacheService(cache *CacheService, repo *database.Repository, logger Logger) *StrategyHierarchyCacheService {
	return &StrategyHierarchyCacheService{
		cache:  cache,
		repo:   repo,
		logger: logger,
	}
}

// =====================================================
// REDIS KEY PATTERNS
// =====================================================

// Key patterns (Story 11.44):
// - strat:group:{user_id}:{mode}:{group} - Strategy group settings (JSON)
// - strat:sub:{user_id}:{mode}:{group}:{sub} - Sub-strategy settings (JSON)
// - strat:enabled:{user_id} - Set of enabled strategy keys (Redis SET)

func strategyGroupKey(userID, mode, group string) string {
	return fmt.Sprintf("strat:group:%s:%s:%s", userID, mode, group)
}

func subStrategyKey(userID, mode, group, subStrategy string) string {
	return fmt.Sprintf("strat:sub:%s:%s:%s:%s", userID, mode, group, subStrategy)
}

func enabledStrategiesKey(userID string) string {
	return fmt.Sprintf("strat:enabled:%s", userID)
}

// enabledStrategyMember creates a string key for a single enabled strategy
// Format: {mode}:{group}:{subStrategy}
func enabledStrategyMember(mode, group, subStrategy string) string {
	return fmt.Sprintf("%s:%s:%s", mode, group, subStrategy)
}

// =====================================================
// STRATEGY GROUP CACHE OPERATIONS
// =====================================================

// GetStrategyGroupFromCache retrieves a strategy group from cache
// Returns nil, nil if not in cache (caller should fetch from DB)
func (s *StrategyHierarchyCacheService) GetStrategyGroupFromCache(ctx context.Context, userID, mode, group string) (*database.StrategyGroupSettings, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := strategyGroupKey(userID, mode, group)
	cached, err := s.cache.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, nil // Treat errors as cache miss for graceful degradation
	}
	if cached == "" {
		return nil, nil // Cache miss
	}

	var settings database.StrategyGroupSettings
	if err := json.Unmarshal([]byte(cached), &settings); err != nil {
		s.logger.Debug("Failed to unmarshal strategy group from cache", "key", key, "error", err)
		return nil, nil // Treat as cache miss
	}

	return &settings, nil
}

// SetStrategyGroupInCache stores a strategy group in cache with TTL
func (s *StrategyHierarchyCacheService) SetStrategyGroupInCache(settings *database.StrategyGroupSettings) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	ctx := context.Background()
	key := strategyGroupKey(settings.UserID, settings.Mode, settings.StrategyGroup)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy group: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), StrategyHierarchyTTL); err != nil {
		s.logger.Debug("Failed to cache strategy group", "key", key, "error", err)
		return err
	}

	return nil
}

// SetStrategyGroupCache stores a strategy group in cache (with context and explicit params)
// Deprecated: Use SetStrategyGroupInCache instead
func (s *StrategyHierarchyCacheService) SetStrategyGroupCache(ctx context.Context, userID, mode, group string, settings *database.StrategyGroupSettings) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := strategyGroupKey(userID, mode, group)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy group: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), StrategyHierarchyTTL); err != nil {
		s.logger.Debug("Failed to cache strategy group", "key", key, "error", err)
		return err
	}

	return nil
}

// GetStrategyGroup retrieves a strategy group with cache-first pattern
// Falls back to database if not in cache, then populates cache
func (s *StrategyHierarchyCacheService) GetStrategyGroup(ctx context.Context, userID, mode, group string) (*database.StrategyGroupSettings, error) {
	// Try cache first
	cached, err := s.GetStrategyGroupFromCache(ctx, userID, mode, group)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss - fetch from database
	settings, err := s.repo.GetStrategyGroupSettings(ctx, userID, mode, group)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil // Not found in DB either
	}

	// Populate cache for next time
	if s.cache.IsHealthy() {
		_ = s.SetStrategyGroupCache(ctx, userID, mode, group, settings)
	}

	return settings, nil
}

// UpdateStrategyGroup updates a strategy group in both DB and cache (write-through)
func (s *StrategyHierarchyCacheService) UpdateStrategyGroup(ctx context.Context, settings *database.StrategyGroupSettings) error {
	// Write to database first
	if err := s.repo.UpsertStrategyGroupSettings(ctx, settings); err != nil {
		return err
	}

	// Update cache
	if s.cache.IsHealthy() {
		if err := s.SetStrategyGroupCache(ctx, settings.UserID, settings.Mode, settings.StrategyGroup, settings); err != nil {
			s.logger.Debug("Failed to update strategy group cache", "error", err)
		}
		// Refresh enabled strategies cache since this may affect it
		_ = s.RefreshEnabledStrategiesCache(settings.UserID)
	}

	return nil
}

// =====================================================
// SUB-STRATEGY CACHE OPERATIONS
// =====================================================

// GetSubStrategyFromCache retrieves a sub-strategy from cache
func (s *StrategyHierarchyCacheService) GetSubStrategyFromCache(ctx context.Context, userID, mode, group, subStrategy string) (*database.SubStrategySettings, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := subStrategyKey(userID, mode, group, subStrategy)
	cached, err := s.cache.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, nil // Treat errors as cache miss for graceful degradation
	}
	if cached == "" {
		return nil, nil // Cache miss
	}

	var settings database.SubStrategySettings
	if err := json.Unmarshal([]byte(cached), &settings); err != nil {
		s.logger.Debug("Failed to unmarshal sub-strategy from cache", "key", key, "error", err)
		return nil, nil // Treat as cache miss
	}

	return &settings, nil
}

// SetSubStrategyInCache stores a sub-strategy in cache with TTL
func (s *StrategyHierarchyCacheService) SetSubStrategyInCache(settings *database.SubStrategySettings) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	ctx := context.Background()
	key := subStrategyKey(settings.UserID, settings.Mode, settings.StrategyGroup, settings.SubStrategy)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal sub-strategy: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), StrategyHierarchyTTL); err != nil {
		s.logger.Debug("Failed to cache sub-strategy", "key", key, "error", err)
		return err
	}

	return nil
}

// SetSubStrategyCache stores a sub-strategy in cache (with context and explicit params)
// Deprecated: Use SetSubStrategyInCache instead
func (s *StrategyHierarchyCacheService) SetSubStrategyCache(ctx context.Context, userID, mode, group, subStrategy string, settings *database.SubStrategySettings) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := subStrategyKey(userID, mode, group, subStrategy)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal sub-strategy: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), StrategyHierarchyTTL); err != nil {
		s.logger.Debug("Failed to cache sub-strategy", "key", key, "error", err)
		return err
	}

	return nil
}

// GetSubStrategy retrieves a sub-strategy with cache-first pattern
func (s *StrategyHierarchyCacheService) GetSubStrategy(ctx context.Context, userID, mode, group, subStrategy string) (*database.SubStrategySettings, error) {
	// Try cache first
	cached, err := s.GetSubStrategyFromCache(ctx, userID, mode, group, subStrategy)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss - fetch from database
	settings, err := s.repo.GetSubStrategySettings(ctx, userID, mode, group, subStrategy)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}

	// Populate cache
	if s.cache.IsHealthy() {
		_ = s.SetSubStrategyCache(ctx, userID, mode, group, subStrategy, settings)
	}

	return settings, nil
}

// UpdateSubStrategy updates a sub-strategy in both DB and cache (write-through)
func (s *StrategyHierarchyCacheService) UpdateSubStrategy(ctx context.Context, settings *database.SubStrategySettings) error {
	// Write to database first
	if err := s.repo.UpsertSubStrategySettings(ctx, settings); err != nil {
		return err
	}

	// Update cache
	if s.cache.IsHealthy() {
		if err := s.SetSubStrategyCache(ctx, settings.UserID, settings.Mode, settings.StrategyGroup, settings.SubStrategy, settings); err != nil {
			s.logger.Debug("Failed to update sub-strategy cache", "error", err)
		}
		// Refresh enabled strategies cache since this may affect it
		_ = s.RefreshEnabledStrategiesCache(settings.UserID)
	}

	return nil
}

// =====================================================
// ENABLED STRATEGIES CACHE (Redis SET)
// =====================================================

// GetEnabledStrategiesFromCache retrieves the list of enabled strategies from cache
// Uses Redis SET for O(1) membership checks
// Returns (nil, nil) for cache miss, ([]string, nil) for cache hit (even if empty)
func (s *StrategyHierarchyCacheService) GetEnabledStrategiesFromCache(userID string) ([]string, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	ctx := context.Background()
	key := enabledStrategiesKey(userID)
	client := s.cache.GetClient()
	if client == nil {
		return nil, ErrCacheUnavailable
	}

	// Check if key exists first (to distinguish empty set from cache miss)
	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		s.logger.Debug("Failed to check enabled strategies key existence", "key", key, "error", err)
		return nil, nil // Treat as cache miss
	}
	if exists == 0 {
		return nil, nil // Cache miss
	}

	// Get all members from the SET
	members, err := client.SMembers(ctx, key).Result()
	if err != nil {
		s.logger.Debug("Failed to get enabled strategies from cache", "key", key, "error", err)
		return nil, nil // Treat as cache miss
	}

	return members, nil
}

// setEnabledStrategiesInCache stores enabled strategies as a Redis SET
// Internal method - use RefreshEnabledStrategiesCache for external use
func (s *StrategyHierarchyCacheService) setEnabledStrategiesInCache(ctx context.Context, userID string, strategies []database.EnabledSubStrategy) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := enabledStrategiesKey(userID)
	client := s.cache.GetClient()
	if client == nil {
		return ErrCacheUnavailable
	}

	// Use pipeline for atomic operation: delete old set and add new members
	pipe := client.Pipeline()
	pipe.Del(ctx, key)

	// Add each enabled strategy as a member
	if len(strategies) > 0 {
		members := make([]interface{}, len(strategies))
		for i, strat := range strategies {
			members[i] = enabledStrategyMember(strat.Mode, strat.StrategyGroup, strat.SubStrategy)
		}
		pipe.SAdd(ctx, key, members...)
	} else {
		// For empty set, we still need to mark the cache as valid
		// Use a special marker that we can detect
		pipe.SAdd(ctx, key, "__empty__")
	}

	// Set TTL
	pipe.Expire(ctx, key, StrategyHierarchyTTL)

	_, err := pipe.Exec(ctx)
	if err != nil {
		s.logger.Debug("Failed to set enabled strategies cache", "key", key, "error", err)
		return err
	}

	return nil
}

// RefreshEnabledStrategiesCache fetches enabled strategies from DB and updates cache
func (s *StrategyHierarchyCacheService) RefreshEnabledStrategiesCache(userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	ctx := context.Background()

	// Fetch from database
	strategies, err := s.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		s.logger.Debug("Failed to fetch enabled strategies from DB", "userID", userID, "error", err)
		return err
	}

	// Update cache
	return s.setEnabledStrategiesInCache(ctx, userID, strategies)
}

// IsStrategyEnabled checks if a specific strategy is enabled (fast O(1) lookup)
func (s *StrategyHierarchyCacheService) IsStrategyEnabled(ctx context.Context, userID, mode, group, subStrategy string) (bool, error) {
	if !s.cache.IsHealthy() {
		return false, ErrCacheUnavailable
	}

	key := enabledStrategiesKey(userID)
	member := enabledStrategyMember(mode, group, subStrategy)
	client := s.cache.GetClient()
	if client == nil {
		return false, ErrCacheUnavailable
	}

	isMember, err := client.SIsMember(ctx, key, member).Result()
	if err != nil {
		s.logger.Debug("Failed to check strategy membership", "key", key, "member", member, "error", err)
		return false, nil // Graceful degradation - assume not cached
	}

	return isMember, nil
}

// GetEnabledStrategies retrieves enabled strategies with cache-first pattern
// Returns database.EnabledSubStrategy slice for compatibility
func (s *StrategyHierarchyCacheService) GetEnabledStrategies(ctx context.Context, userID string) ([]database.EnabledSubStrategy, error) {
	// Try cache first
	cachedMembers, err := s.GetEnabledStrategiesFromCache(userID)
	if err == nil && cachedMembers != nil {
		// Parse members back to EnabledSubStrategy
		strategies := make([]database.EnabledSubStrategy, 0, len(cachedMembers))
		for _, member := range cachedMembers {
			// Skip empty marker
			if member == "__empty__" {
				continue
			}
			// Parse "mode:group:subStrategy" format
			var mode, group, sub string
			n, _ := fmt.Sscanf(member, "%s", &mode) // This won't work for colons, use split
			parts := splitStrategyMember(member)
			if len(parts) == 3 {
				mode, group, sub = parts[0], parts[1], parts[2]
				strategies = append(strategies, database.EnabledSubStrategy{
					Mode:          mode,
					StrategyGroup: group,
					SubStrategy:   sub,
				})
			} else {
				s.logger.Debug("Invalid strategy member format", "member", member, "parsed", n)
			}
		}
		return strategies, nil
	}

	// Cache miss - fetch from database
	strategies, err := s.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Populate cache
	if s.cache.IsHealthy() {
		_ = s.setEnabledStrategiesInCache(ctx, userID, strategies)
	}

	return strategies, nil
}

// splitStrategyMember splits a strategy member string "mode:group:sub" into parts
func splitStrategyMember(member string) []string {
	parts := make([]string, 0, 3)
	start := 0
	for i, c := range member {
		if c == ':' {
			parts = append(parts, member[start:i])
			start = i + 1
		}
	}
	if start < len(member) {
		parts = append(parts, member[start:])
	}
	return parts
}

// InvalidateEnabledStrategiesCache removes the enabled strategies cache
// Called when any strategy's enabled status changes
// Deprecated: Use RefreshEnabledStrategiesCache instead for write-through pattern
func (s *StrategyHierarchyCacheService) InvalidateEnabledStrategiesCache(ctx context.Context, userID string) {
	if !s.cache.IsHealthy() {
		return
	}

	key := enabledStrategiesKey(userID)
	if err := s.cache.Delete(ctx, key); err != nil {
		s.logger.Debug("Failed to invalidate enabled strategies cache", "key", key, "error", err)
	}
}

// =====================================================
// CACHE INVALIDATION
// =====================================================

// InvalidateStrategyHierarchyCache removes all strategy hierarchy cache for a user
// Called on full refresh or user deletion
func (s *StrategyHierarchyCacheService) InvalidateStrategyHierarchyCache(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Delete strategy group keys
	strategyGroupPattern := fmt.Sprintf("strat:group:%s:*", userID)
	if err := s.cache.DeletePattern(ctx, strategyGroupPattern); err != nil {
		s.logger.Debug("Failed to delete strategy group cache pattern", "pattern", strategyGroupPattern, "error", err)
	}

	// Delete sub-strategy keys
	subStrategyPattern := fmt.Sprintf("strat:sub:%s:*", userID)
	if err := s.cache.DeletePattern(ctx, subStrategyPattern); err != nil {
		s.logger.Debug("Failed to delete sub-strategy cache pattern", "pattern", subStrategyPattern, "error", err)
	}

	// Delete enabled strategies
	s.InvalidateEnabledStrategiesCache(ctx, userID)

	s.logger.Debug("Invalidated strategy hierarchy cache for user", "userID", userID)
	return nil
}

// InvalidateStrategyGroup removes a specific strategy group from cache
func (s *StrategyHierarchyCacheService) InvalidateStrategyGroup(ctx context.Context, userID, mode, group string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := strategyGroupKey(userID, mode, group)
	if err := s.cache.Delete(ctx, key); err != nil {
		s.logger.Debug("Failed to invalidate strategy group", "key", key, "error", err)
		return err
	}

	// Refresh enabled strategies to ensure consistency
	_ = s.RefreshEnabledStrategiesCache(userID)
	return nil
}

// InvalidateSubStrategy removes a specific sub-strategy from cache
func (s *StrategyHierarchyCacheService) InvalidateSubStrategy(ctx context.Context, userID, mode, group, subStrategy string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := subStrategyKey(userID, mode, group, subStrategy)
	if err := s.cache.Delete(ctx, key); err != nil {
		s.logger.Debug("Failed to invalidate sub-strategy", "key", key, "error", err)
		return err
	}

	// Refresh enabled strategies to ensure consistency
	_ = s.RefreshEnabledStrategiesCache(userID)
	return nil
}

// =====================================================
// CACHE LOADING (On User Login)
// =====================================================

// LoadStrategyHierarchyToCache loads all strategy hierarchy settings to cache for a user
// Called during user login or session init
func (s *StrategyHierarchyCacheService) LoadStrategyHierarchyToCache(ctx context.Context, userID string) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	// Load all strategy groups
	for _, mode := range []string{"scalp", "swing", "position", "ultra_fast"} {
		groups, err := s.repo.GetAllStrategyGroups(ctx, userID, mode)
		if err != nil {
			s.logger.Debug("Failed to load strategy groups for mode", "mode", mode, "error", err)
			continue
		}

		for _, group := range groups {
			_ = s.SetStrategyGroupCache(ctx, userID, mode, group.StrategyGroup, group)

			// Load sub-strategies for this group
			subStrategies, err := s.repo.GetAllSubStrategies(ctx, userID, mode, group.StrategyGroup)
			if err != nil {
				s.logger.Debug("Failed to load sub-strategies", "mode", mode, "group", group.StrategyGroup, "error", err)
				continue
			}

			for _, sub := range subStrategies {
				_ = s.SetSubStrategyCache(ctx, userID, mode, group.StrategyGroup, sub.SubStrategy, sub)
			}
		}
	}

	// Load enabled strategies using refresh (populates Redis SET)
	if err := s.RefreshEnabledStrategiesCache(userID); err != nil {
		s.logger.Debug("Failed to refresh enabled strategies cache", "userID", userID, "error", err)
	}

	s.logger.Debug("Loaded strategy hierarchy to cache", "userID", userID)
	return nil
}

// =====================================================
// UTILITY METHODS
// =====================================================

// IsHealthy returns whether the underlying cache is healthy
func (s *StrategyHierarchyCacheService) IsHealthy() bool {
	return s.cache.IsHealthy()
}

// GetAllStrategyGroupsForMode retrieves all strategy groups for a mode with caching
func (s *StrategyHierarchyCacheService) GetAllStrategyGroupsForMode(ctx context.Context, userID, mode string) ([]*database.StrategyGroupSettings, error) {
	// For bulk operations, we go directly to DB and update cache
	groups, err := s.repo.GetAllStrategyGroups(ctx, userID, mode)
	if err != nil {
		return nil, err
	}

	// Update cache for each group
	if s.cache.IsHealthy() {
		for _, group := range groups {
			_ = s.SetStrategyGroupCache(ctx, userID, mode, group.StrategyGroup, group)
		}
	}

	return groups, nil
}

// GetAllSubStrategiesForGroup retrieves all sub-strategies for a group with caching
func (s *StrategyHierarchyCacheService) GetAllSubStrategiesForGroup(ctx context.Context, userID, mode, group string) ([]*database.SubStrategySettings, error) {
	// For bulk operations, we go directly to DB and update cache
	subStrategies, err := s.repo.GetAllSubStrategies(ctx, userID, mode, group)
	if err != nil {
		return nil, err
	}

	// Update cache for each sub-strategy
	if s.cache.IsHealthy() {
		for _, sub := range subStrategies {
			_ = s.SetSubStrategyCache(ctx, userID, mode, group, sub.SubStrategy, sub)
		}
	}

	return subStrategies, nil
}

// =====================================================
// LEGACY COMPATIBILITY
// =====================================================

// SetEnabledStrategiesCache stores the enabled strategies list in cache
// Deprecated: Uses JSON array storage. Use RefreshEnabledStrategiesCache for SET-based storage
func (s *StrategyHierarchyCacheService) SetEnabledStrategiesCache(ctx context.Context, userID string, strategies []database.EnabledSubStrategy) error {
	return s.setEnabledStrategiesInCache(ctx, userID, strategies)
}
