package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"binance-trading-bot/internal/database"
)

// =====================================================
// STRATEGY HIERARCHY CACHE SERVICE
// Story 11.44: Volume Imbalance Database Schema & Repository
// Provides Redis caching for strategy hierarchy settings
// =====================================================

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

// Key patterns:
// - user:{userID}:strategy_group:{mode}:{group} - Strategy group settings
// - user:{userID}:sub_strategy:{mode}:{group}:{subStrategy} - Sub-strategy settings
// - user:{userID}:enabled_strategies - List of enabled sub-strategies (quick lookup)

func strategyGroupKey(userID, mode, group string) string {
	return fmt.Sprintf("user:%s:strategy_group:%s:%s", userID, mode, group)
}

func subStrategyKey(userID, mode, group, subStrategy string) string {
	return fmt.Sprintf("user:%s:sub_strategy:%s:%s:%s", userID, mode, group, subStrategy)
}

func enabledStrategiesKey(userID string) string {
	return fmt.Sprintf("user:%s:enabled_strategies", userID)
}

// =====================================================
// STRATEGY GROUP CACHE OPERATIONS
// =====================================================

// GetStrategyGroupFromCache retrieves a strategy group from cache
// Returns nil if not in cache (caller should fetch from DB)
func (s *StrategyHierarchyCacheService) GetStrategyGroupFromCache(ctx context.Context, userID, mode, group string) (*database.StrategyGroupSettings, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := strategyGroupKey(userID, mode, group)
	cached, err := s.cache.Get(ctx, key)
	if err != nil || cached == "" {
		return nil, nil // Cache miss
	}

	var settings database.StrategyGroupSettings
	if err := json.Unmarshal([]byte(cached), &settings); err != nil {
		s.logger.Debug("Failed to unmarshal strategy group from cache", "key", key, "error", err)
		return nil, nil // Treat as cache miss
	}

	return &settings, nil
}

// SetStrategyGroupCache stores a strategy group in cache
func (s *StrategyHierarchyCacheService) SetStrategyGroupCache(ctx context.Context, userID, mode, group string, settings *database.StrategyGroupSettings) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := strategyGroupKey(userID, mode, group)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy group: %w", err)
	}

	// TTL of 0 means no expiration (persistent until invalidated)
	if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
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
		// Invalidate enabled strategies cache since this may affect it
		s.InvalidateEnabledStrategiesCache(ctx, settings.UserID)
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
	if err != nil || cached == "" {
		return nil, nil // Cache miss
	}

	var settings database.SubStrategySettings
	if err := json.Unmarshal([]byte(cached), &settings); err != nil {
		s.logger.Debug("Failed to unmarshal sub-strategy from cache", "key", key, "error", err)
		return nil, nil // Treat as cache miss
	}

	return &settings, nil
}

// SetSubStrategyCache stores a sub-strategy in cache
func (s *StrategyHierarchyCacheService) SetSubStrategyCache(ctx context.Context, userID, mode, group, subStrategy string, settings *database.SubStrategySettings) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := subStrategyKey(userID, mode, group, subStrategy)
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal sub-strategy: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
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
		// Invalidate enabled strategies cache since this may affect it
		s.InvalidateEnabledStrategiesCache(ctx, settings.UserID)
	}

	return nil
}

// =====================================================
// ENABLED STRATEGIES CACHE
// =====================================================

// GetEnabledStrategiesFromCache retrieves the list of enabled strategies from cache
// Returns (nil, nil) for cache miss, ([]EnabledSubStrategy, nil) for cache hit (even if empty)
func (s *StrategyHierarchyCacheService) GetEnabledStrategiesFromCache(ctx context.Context, userID string) ([]database.EnabledSubStrategy, error) {
	if !s.cache.IsHealthy() {
		return nil, ErrCacheUnavailable
	}

	key := enabledStrategiesKey(userID)
	cached, err := s.cache.Get(ctx, key)
	if err != nil || cached == "" {
		return nil, nil // Cache miss
	}

	var strategies []database.EnabledSubStrategy
	if err := json.Unmarshal([]byte(cached), &strategies); err != nil {
		s.logger.Debug("Failed to unmarshal enabled strategies from cache", "key", key, "error", err)
		return nil, nil
	}

	// Story 11.44 Fix: Return empty slice (not nil) for cache hit with empty result
	// This distinguishes cache hit with empty data from cache miss
	if strategies == nil {
		strategies = []database.EnabledSubStrategy{}
	}

	return strategies, nil
}

// SetEnabledStrategiesCache stores the enabled strategies list in cache
func (s *StrategyHierarchyCacheService) SetEnabledStrategiesCache(ctx context.Context, userID string, strategies []database.EnabledSubStrategy) error {
	if !s.cache.IsHealthy() {
		return ErrCacheUnavailable
	}

	key := enabledStrategiesKey(userID)
	data, err := json.Marshal(strategies)
	if err != nil {
		return fmt.Errorf("failed to marshal enabled strategies: %w", err)
	}

	if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
		s.logger.Debug("Failed to cache enabled strategies", "key", key, "error", err)
		return err
	}

	return nil
}

// GetEnabledStrategies retrieves enabled strategies with cache-first pattern
func (s *StrategyHierarchyCacheService) GetEnabledStrategies(ctx context.Context, userID string) ([]database.EnabledSubStrategy, error) {
	// Try cache first
	cached, err := s.GetEnabledStrategiesFromCache(ctx, userID)
	if err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss - fetch from database
	strategies, err := s.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Populate cache (Story 11.44 Fix: Cache empty results too to prevent repeated DB queries)
	// Empty slice is valid and should be cached to avoid hitting DB every time
	if s.cache.IsHealthy() {
		// Use empty slice marker in cache for empty results
		if strategies == nil {
			strategies = []database.EnabledSubStrategy{}
		}
		_ = s.SetEnabledStrategiesCache(ctx, userID, strategies)
	}

	return strategies, nil
}

// InvalidateEnabledStrategiesCache removes the enabled strategies cache
// Called when any strategy's enabled status changes
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
	strategyGroupPattern := fmt.Sprintf("user:%s:strategy_group:*", userID)
	if err := s.cache.DeletePattern(ctx, strategyGroupPattern); err != nil {
		s.logger.Debug("Failed to delete strategy group cache pattern", "pattern", strategyGroupPattern, "error", err)
	}

	// Delete sub-strategy keys
	subStrategyPattern := fmt.Sprintf("user:%s:sub_strategy:*", userID)
	if err := s.cache.DeletePattern(ctx, subStrategyPattern); err != nil {
		s.logger.Debug("Failed to delete sub-strategy cache pattern", "pattern", subStrategyPattern, "error", err)
	}

	// Delete enabled strategies
	s.InvalidateEnabledStrategiesCache(ctx, userID)

	s.logger.Debug("Invalidated strategy hierarchy cache for user", "userID", userID)
	return nil
}

// InvalidateStrategyGroup removes a specific strategy group from cache
func (s *StrategyHierarchyCacheService) InvalidateStrategyGroup(ctx context.Context, userID, mode, group string) {
	if !s.cache.IsHealthy() {
		return
	}

	key := strategyGroupKey(userID, mode, group)
	s.cache.Delete(ctx, key)
	s.InvalidateEnabledStrategiesCache(ctx, userID)
}

// InvalidateSubStrategy removes a specific sub-strategy from cache
func (s *StrategyHierarchyCacheService) InvalidateSubStrategy(ctx context.Context, userID, mode, group, subStrategy string) {
	if !s.cache.IsHealthy() {
		return
	}

	key := subStrategyKey(userID, mode, group, subStrategy)
	s.cache.Delete(ctx, key)
	s.InvalidateEnabledStrategiesCache(ctx, userID)
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

	// Load enabled strategies
	strategies, err := s.repo.GetEnabledStrategies(ctx, userID)
	if err != nil {
		s.logger.Debug("Failed to load enabled strategies", "userID", userID, "error", err)
	} else if len(strategies) > 0 {
		_ = s.SetEnabledStrategiesCache(ctx, userID, strategies)
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
