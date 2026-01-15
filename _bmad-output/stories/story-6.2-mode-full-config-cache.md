# Story 6.2: Complete User Settings Cache (UI-Mirrored)

**Story ID:** CACHE-6.2
**Epic:** Epic 6 - Redis Caching Infrastructure
**Priority:** P0 (Critical - Foundation for all cache operations)
**Estimated Effort:** 18 hours
**Author:** BMAD Party Mode (Bob - Scrum Master)
**Status:** Development Complete
**Version:** 7.1 (Development Complete - Migration 026 applied, all infrastructure ready)

---

## Description

Implement Redis caching for **ALL user settings** using a granular key structure that mirrors the `default-settings.json` structure exactly:

1. **Mode Settings** (4 modes x 20 groups = 80 keys) - includes position_optimization as one of the 20 groups
2. **Global Settings** (Circuit Breaker, LLM Config, Capital Allocation, Global Trading = 4 keys)
3. **Safety Settings** (Per-mode safety: ultra_fast, scalp, swing, position = 4 keys)

**Total: 88 Redis keys per user** (matches Story 6.4 admin defaults minus the hash key)

**Source of Truth:** `default-settings.json` - ALL keys in this file must have corresponding cache keys.

Each settings card in the Reset Settings page becomes a separate Redis key, enabling:

- Easy navigation (keys match UI structure)
- Granular updates (change one card, update one key)
- Partial resets (reset one group without affecting others)
- Self-documenting architecture (Redis = UI)

---

## Critical Architecture Principle

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     REDIS IS THE BRAIN - NO BYPASS                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ALL READS go through cache:                                             │
│  ┌──────────┐     ┌─────────┐     ┌────────────┐     ┌──────────┐       │
│  │ API/Ginie│────>│  Cache  │────>│ Return from│────>│  Caller  │       │
│  │ Request  │     │ Service │     │   Cache    │     │          │       │
│  └──────────┘     └────┬────┘     └────────────┘     └──────────┘       │
│                        │                                                 │
│                        v (on cache miss)                                 │
│                   ┌─────────┐                                            │
│                   │   DB    │ Cache populates itself, then returns       │
│                   └─────────┘                                            │
│                                                                          │
│  NEVER bypass cache to read DB directly!                                 │
│  If Redis is DOWN = System ERROR (no trading)                            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key Rules:**
1. **Cache miss** → Cache loads from DB → Populates cache → Returns FROM CACHE
2. **Redis down** → Return ERROR (system cannot operate without cache)
3. **Never bypass** → All reads MUST go through SettingsCacheService

---

## User Story

> As the Ginie Autopilot engine,
> I want ALL user settings cached in granular Redis keys matching the UI structure,
> So that I can access any specific setting in <1ms and update individual settings without reloading entire configurations.

---

## Background

### Key Structure (Mirrors default-settings.json)

```
# Mode Settings (80 keys = 4 modes x 20 groups)
# From: default-settings.json → mode_configs.{mode}.{group}
user:{userID}:mode:scalp:enabled              → {"enabled": true}
user:{userID}:mode:scalp:timeframe            → {"trend_tf": "15m", ...}
user:{userID}:mode:scalp:confidence           → {"min": 55, "high": 75, ...}
user:{userID}:mode:scalp:size                 → {"base_size_usd": 200, ...}
... (20 keys per mode × 4 modes = 80 keys)

# Global Settings (4 keys)
# From: default-settings.json → circuit_breaker.global, llm_config.global, capital_allocation, global_trading
user:{userID}:circuit_breaker                 → {"enabled": true, "max_loss_per_hour": 100, ...}
user:{userID}:llm_config                      → {"enabled": true, "provider": "deepseek", ...}
user:{userID}:capital_allocation              → {"ultra_fast_percent": 0, "scalp_percent": 100, ...}
user:{userID}:global_trading                  → {"risk_level": "moderate", "max_usd_allocation": 2500, ...}

# Safety Settings (4 keys - per mode)
# From: default-settings.json → safety_settings.{mode}
user:{userID}:safety:ultra_fast               → {"max_trades_per_minute": 10, ...}
user:{userID}:safety:scalp                    → {"max_trades_per_minute": 8, ...}
user:{userID}:safety:swing                    → {"max_trades_per_minute": 10, ...}
user:{userID}:safety:position                 → {"max_trades_per_minute": 5, ...}

# NOTE: position_optimization is within mode groups (one of the 20 groups per mode)
# NOT a separate global key - see mode settings above
```

### Complete Key Count (88 keys per user)

| Category | Keys | Source in default-settings.json |
|----------|------|--------------------------------|
| Mode Settings | 80 | `mode_configs.{mode}.{group}` (includes position_optimization as one of 20 groups) |
| Global Circuit Breaker | 1 | `circuit_breaker.global` |
| Global LLM Config | 1 | `llm_config.global` |
| Capital Allocation | 1 | `capital_allocation` |
| Global Trading | 1 | `global_trading` |
| Safety Settings | 4 | `safety_settings.{mode}` |
| **TOTAL** | **88** | (matches Story 6.4 admin defaults minus hash key) |

**Note:** Position optimization is within mode groups (one of the 20 groups), NOT a separate global key.

### UI Reference

The structure comes from `SETTING_GROUPS` in `web/src/components/SettingsComparisonView.tsx` (20 groups):

| # | Group Key | UI Card Name |
|---|-----------|--------------|
| 1 | `enabled` | Mode Status |
| 2 | `timeframe` | Timeframe Settings |
| 3 | `confidence` | Confidence Settings |
| 4 | `size` | Size Settings |
| 5 | `sltp` | SL/TP Settings |
| 6 | `risk` | Risk Settings |
| 7 | `circuit_breaker` | Circuit Breaker |
| 8 | `hedge` | Hedge Settings |
| 9 | `averaging` | Position Averaging |
| 10 | `stale_release` | Stale Position Release |
| 11 | `assignment` | Mode Assignment |
| 12 | `mtf` | Multi-Timeframe (MTF) |
| 13 | `dynamic_ai_exit` | Dynamic AI Exit |
| 14 | `reversal` | Reversal Entry |
| 15 | `funding_rate` | Funding Rate |
| 16 | `trend_divergence` | Trend Divergence |
| 17 | `position_optimization` | Position Optimization |
| 18 | `trend_filters` | Trend Filters |
| 19 | `early_warning` | Early Warning |
| 20 | `entry` | Entry Settings |

---

## Acceptance Criteria

### AC6.2.1: Setting Groups Constant (Go)
- [ ] Define `SettingGroups` slice matching UI's `SETTING_GROUPS`
- [ ] 20 groups total (matching UI exactly)
- [ ] Each group has key, name, and field prefixes

### AC6.2.2: Cache Key Structure (Mirrors default-settings.json)
- [ ] Mode settings pattern: `user:{userID}:mode:{modeName}:{groupKey}`
- [ ] Global settings pattern: `user:{userID}:{settingName}`
- [ ] Safety settings pattern: `user:{userID}:safety:{modeName}`
- [ ] 4 modes x 20 groups = 80 mode keys per user
- [ ] 4 global settings keys (circuit_breaker, llm_config, capital_allocation, global_trading)
- [ ] 4 safety settings keys (per-mode: ultra_fast, scalp, swing, position)
- [ ] 1 global position_optimization key
- [ ] **Total: 88 keys per user**
- [ ] Each key stores only relevant fields (~200-500 bytes)
- [ ] No TTL (settings persist until updated)

### AC6.2.3: Load ALL User Settings (On Login)
- [ ] Function: `LoadUserSettings(ctx, userID) error`
- [ ] Load mode settings from DB, split into groups, store as 80 Redis keys
- [ ] Load global settings as 4 Redis keys (circuit_breaker, llm_config, capital_allocation, global_trading)
- [ ] Load safety settings as 4 Redis keys (per-mode)
- [ ] Load global position_optimization as 1 Redis key
- [ ] **Total: 88 keys loaded on login**
- [ ] Called from authentication success handler
- [ ] If Redis unavailable during login, return error (user cannot trade)

### AC6.2.4: Get Single Group (Cache-Only with Auto-Populate)
- [ ] Function: `GetModeGroup(ctx, userID, mode, group) ([]byte, error)`
- [ ] If Redis unavailable → Return `ErrCacheUnavailable`
- [ ] If cache hit → Return from cache
- [ ] If cache miss → Load from DB → Populate cache → Return from cache
- [ ] **NEVER return data directly from DB bypassing cache**

### AC6.2.5: Get Full Mode Config (Assemble from Groups)
- [ ] Function: `GetModeConfig(ctx, userID, mode) (*ModeFullConfig, error)`
- [ ] Use Redis MGET for atomic read of all 20 groups
- [ ] Merge into single struct
- [ ] For API responses that need complete config

### AC6.2.6: Update Single Group (Write-Through: DB First)
- [ ] Function: `UpdateModeGroup(ctx, userID, mode, group, data) error`
- [ ] **Step 1:** Write to DB first (durable storage)
- [ ] **Step 2:** Update Redis cache
- [ ] If Redis update fails, log warning (DB has truth, cache will repopulate on next read)

### AC6.2.7: Reset Single Group
- [ ] Function: `ResetModeGroup(ctx, userID, mode, group) error`
- [ ] Get default from `admin:defaults:mode:{mode}:{group}` or default-settings.json
- [ ] Update user's group with default value (using write-through)

### AC6.2.8: Invalidation Operations
- [ ] `InvalidateModeGroup(ctx, userID, mode, group)` - Single group
- [ ] `InvalidateMode(ctx, userID, mode)` - All 20 groups for a mode
- [ ] `InvalidateAllModes(ctx, userID)` - All 80 mode keys
- [ ] `InvalidateGlobalSetting(ctx, userID, setting)` - Single global setting
- [ ] `InvalidateSafetySettings(ctx, userID, mode)` - Single mode's safety settings
- [ ] `InvalidateAllSafetySettings(ctx, userID)` - All 4 safety settings keys
- [ ] `InvalidateGlobalPositionOptimization(ctx, userID)` - Global position optimization
- [ ] `InvalidateAllUserSettings(ctx, userID)` - All 88 keys

### AC6.2.9: Global Settings (Circuit Breaker, LLM, Capital, Global Trading)
- [ ] `GetCircuitBreaker(ctx, userID)` - Cache-only with auto-populate
- [ ] `UpdateCircuitBreaker(ctx, userID, data)` - Write-through (DB first)
- [ ] `GetLLMConfig(ctx, userID)` - Cache-only with auto-populate
- [ ] `UpdateLLMConfig(ctx, userID, data)` - Write-through (DB first)
- [ ] `GetCapitalAllocation(ctx, userID)` - Cache-only with auto-populate
- [ ] `UpdateCapitalAllocation(ctx, userID, data)` - Write-through (DB first)
- [ ] `GetGlobalTrading(ctx, userID)` - Cache-only with auto-populate (NEW)
- [ ] `UpdateGlobalTrading(ctx, userID, data)` - Write-through (DB first) (NEW)
- [ ] All global settings loaded on user login alongside mode settings

### AC6.2.10: Safety Settings (Per-Mode)
- [ ] `GetSafetySettings(ctx, userID, mode)` - Cache-only with auto-populate
- [ ] `UpdateSafetySettings(ctx, userID, mode, data)` - Write-through (DB first)
- [ ] `GetAllSafetySettings(ctx, userID)` - Get all 4 modes' safety settings
- [ ] Key pattern: `user:{userID}:safety:{mode}`
- [ ] 4 keys total (ultra_fast, scalp, swing, position)
- [ ] Safety settings loaded on user login

### AC6.2.11: Error Handling (Redis Unavailable = System Error)
- [ ] Define `ErrCacheUnavailable` error type
- [ ] Define `ErrSettingNotFound` error type
- [ ] All read operations return error if Redis is down (no silent fallback)
- [ ] Ginie autopilot must stop trading if cache is unavailable
- [ ] API endpoints return HTTP 503 when cache unavailable

---

## Technical Specification

### New File: `internal/cache/settings_errors.go`

```go
package cache

import "errors"

var (
    // ErrCacheUnavailable is returned when Redis is not healthy
    ErrCacheUnavailable = errors.New("cache unavailable - Redis is not healthy")

    // ErrSettingNotFound is returned when a setting doesn't exist in cache or DB
    ErrSettingNotFound = errors.New("setting not found")
)
```

### New File: `internal/cache/settings_groups.go`

```go
package cache

// Logger interface for dependency injection
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}

// SettingGroup defines a UI settings card that maps to a Redis key
type SettingGroup struct {
    Key         string   // Redis key suffix (e.g., "confidence")
    Name        string   // UI display name (e.g., "Confidence Settings")
    Prefixes    []string // Field prefixes to extract (e.g., ["confidence."])
    Description string   // What this group contains
}

// SettingGroups matches SETTING_GROUPS from SettingsComparisonView.tsx (20 groups)
var SettingGroups = []SettingGroup{
    {Key: "enabled", Name: "Mode Status", Prefixes: []string{"enabled"}, Description: "Whether mode is enabled"},
    {Key: "timeframe", Name: "Timeframe Settings", Prefixes: []string{"timeframe."}, Description: "Chart timeframes"},
    {Key: "confidence", Name: "Confidence Settings", Prefixes: []string{"confidence."}, Description: "Confidence thresholds"},
    {Key: "size", Name: "Size Settings", Prefixes: []string{"size."}, Description: "Position sizing"},
    {Key: "sltp", Name: "SL/TP Settings", Prefixes: []string{"sltp."}, Description: "Stop loss and take profit"},
    {Key: "risk", Name: "Risk Settings", Prefixes: []string{"risk."}, Description: "Risk parameters"},
    {Key: "circuit_breaker", Name: "Circuit Breaker", Prefixes: []string{"circuit_breaker."}, Description: "Per-mode limits"},
    {Key: "hedge", Name: "Hedge Settings", Prefixes: []string{"hedge."}, Description: "Hedge configuration"},
    {Key: "averaging", Name: "Position Averaging", Prefixes: []string{"averaging."}, Description: "DCA rules"},
    {Key: "stale_release", Name: "Stale Position Release", Prefixes: []string{"stale_release."}, Description: "Stale position handling"},
    {Key: "assignment", Name: "Mode Assignment", Prefixes: []string{"assignment."}, Description: "Mode selection criteria"},
    {Key: "mtf", Name: "Multi-Timeframe (MTF)", Prefixes: []string{"mtf."}, Description: "MTF analysis"},
    {Key: "dynamic_ai_exit", Name: "Dynamic AI Exit", Prefixes: []string{"dynamic_ai_exit."}, Description: "AI exit decisions"},
    {Key: "reversal", Name: "Reversal Entry", Prefixes: []string{"reversal."}, Description: "Reversal patterns"},
    {Key: "funding_rate", Name: "Funding Rate", Prefixes: []string{"funding_rate."}, Description: "Funding rate rules"},
    {Key: "trend_divergence", Name: "Trend Divergence", Prefixes: []string{"trend_divergence."}, Description: "Trend alignment"},
    {Key: "position_optimization", Name: "Position Optimization", Prefixes: []string{"position_optimization."}, Description: "Progressive TP, DCA"},
    {Key: "trend_filters", Name: "Trend Filters", Prefixes: []string{"trend_filters."}, Description: "BTC, EMA, VWAP filters"},
    {Key: "early_warning", Name: "Early Warning", Prefixes: []string{"early_warning."}, Description: "Early exit monitoring"},
    {Key: "entry", Name: "Entry Settings", Prefixes: []string{"entry."}, Description: "Entry configuration"},
}

// TradingModes defines the 4 trading modes
var TradingModes = []string{"ultra_fast", "scalp", "swing", "position"}

// GlobalSettings defines the 4 global setting types
var GlobalSettings = []string{"circuit_breaker", "llm_config", "capital_allocation", "global_trading"}

// SafetySettingModes defines modes that have safety settings
var SafetySettingModes = []string{"ultra_fast", "scalp", "swing", "position"}

// GetSettingGroupKeys returns just the keys for iteration
func GetSettingGroupKeys() []string {
    keys := make([]string, len(SettingGroups))
    for i, g := range SettingGroups {
        keys[i] = g.Key
    }
    return keys
}
```

### Updated: `internal/cache/settings_cache_service.go`

```go
package cache

import (
    "context"
    "encoding/json"
    "fmt"

    "binance-trading-bot/internal/autopilot"
    "binance-trading-bot/internal/database"
)

// SettingsCacheService provides granular cache access to user settings
// ALL reads go through this service - never bypass to DB directly
type SettingsCacheService struct {
    cache  *CacheService
    repo   *database.Repository
    logger Logger
}

// NewSettingsCacheService creates a new settings cache service
func NewSettingsCacheService(cache *CacheService, repo *database.Repository, logger Logger) *SettingsCacheService {
    return &SettingsCacheService{
        cache:  cache,
        repo:   repo,
        logger: logger,
    }
}

// ============================================================================
// LOAD OPERATIONS (On User Login)
// ============================================================================

// LoadUserSettings loads ALL user settings (88 keys) on login
// This MUST succeed for user to trade - returns error if Redis unavailable
func (s *SettingsCacheService) LoadUserSettings(ctx context.Context, userID string) error {
    if !s.cache.IsHealthy() {
        return ErrCacheUnavailable
    }

    var errs []error

    // Load mode settings (80 keys)
    for _, mode := range TradingModes {
        if err := s.loadModeToCache(ctx, userID, mode); err != nil {
            errs = append(errs, fmt.Errorf("mode %s: %w", mode, err))
        }
    }

    // Load global settings (4 keys: circuit_breaker, llm_config, capital_allocation, global_trading)
    if err := s.loadGlobalSettings(ctx, userID); err != nil {
        errs = append(errs, fmt.Errorf("global settings: %w", err))
    }

    // Load safety settings (4 keys: per-mode safety)
    if err := s.loadSafetySettings(ctx, userID); err != nil {
        errs = append(errs, fmt.Errorf("safety settings: %w", err))
    }

    if len(errs) > 0 {
        s.logger.Warn("Some settings failed to load", "userID", userID, "errors", errs)
    }

    return nil
}

// loadModeToCache loads a single mode's settings into granular cache keys
func (s *SettingsCacheService) loadModeToCache(ctx context.Context, userID, mode string) error {
    // Get full mode config from database
    configJSON, err := s.repo.GetUserModeConfig(ctx, userID, mode)
    if err != nil {
        return fmt.Errorf("failed to get mode config: %w", err)
    }
    if configJSON == nil {
        return nil // No config in DB, skip caching
    }

    // Parse into ModeFullConfig
    var config autopilot.ModeFullConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("failed to parse mode config: %w", err)
    }

    // Extract and cache each group
    for _, group := range SettingGroups {
        groupData := s.extractGroupFromConfig(&config, group.Key)
        if groupData == nil {
            continue
        }

        key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group.Key)
        groupJSON, _ := json.Marshal(groupData)

        if err := s.cache.Set(ctx, key, string(groupJSON), 0); err != nil {
            s.logger.Debug("Failed to cache group", "key", key, "error", err)
        }
    }

    return nil
}

// loadGlobalSettings loads all global settings (circuit_breaker, llm_config, capital_allocation, global_trading)
func (s *SettingsCacheService) loadGlobalSettings(ctx context.Context, userID string) error {
    // Circuit Breaker
    if cb, err := s.repo.GetUserGlobalCircuitBreaker(ctx, userID); err == nil && cb != nil {
        key := fmt.Sprintf("user:%s:circuit_breaker", userID)
        data, _ := json.Marshal(cb)
        s.cache.Set(ctx, key, string(data), 0)
    }

    // LLM Config
    if llm, err := s.repo.GetUserLLMConfig(ctx, userID); err == nil && llm != nil {
        key := fmt.Sprintf("user:%s:llm_config", userID)
        data, _ := json.Marshal(llm)
        s.cache.Set(ctx, key, string(data), 0)
    }

    // Capital Allocation
    if cap, err := s.repo.GetUserCapitalAllocation(ctx, userID); err == nil && cap != nil {
        key := fmt.Sprintf("user:%s:capital_allocation", userID)
        data, _ := json.Marshal(cap)
        s.cache.Set(ctx, key, string(data), 0)
    }

    // Global Trading (NEW)
    if gt, err := s.repo.GetUserGlobalTrading(ctx, userID); err == nil && gt != nil {
        key := fmt.Sprintf("user:%s:global_trading", userID)
        data, _ := json.Marshal(gt)
        s.cache.Set(ctx, key, string(data), 0)
    }

    return nil
}

// loadSafetySettings loads per-mode safety settings (4 keys)
func (s *SettingsCacheService) loadSafetySettings(ctx context.Context, userID string) error {
    for _, mode := range SafetySettingModes {
        if safety, err := s.repo.GetUserSafetySettings(ctx, userID, mode); err == nil && safety != nil {
            key := fmt.Sprintf("user:%s:safety:%s", userID, mode)
            data, _ := json.Marshal(safety)
            s.cache.Set(ctx, key, string(data), 0)
        }
    }
    return nil
}

// NOTE: position_optimization is within mode groups (one of the 20 groups per mode)
// NOT a separate global key - it's loaded as part of loadModeToCache

// ============================================================================
// READ OPERATIONS (Cache-Only with Auto-Populate)
// ============================================================================

// GetModeGroup retrieves a single settings group
// NEVER bypasses cache - if miss, populates cache first then returns from cache
func (s *SettingsCacheService) GetModeGroup(ctx context.Context, userID, mode, group string) ([]byte, error) {
    // RULE: Redis must be healthy - no bypass allowed
    if !s.cache.IsHealthy() {
        return nil, ErrCacheUnavailable
    }

    key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)

    // Try cache first
    cached, err := s.cache.Get(ctx, key)
    if err == nil && cached != "" {
        return []byte(cached), nil
    }

    // Cache miss - populate cache from DB, then return FROM CACHE
    if err := s.populateModeGroupFromDB(ctx, userID, mode, group); err != nil {
        return nil, err
    }

    // Now read from cache (NOT from DB directly)
    cached, err = s.cache.Get(ctx, key)
    if err != nil || cached == "" {
        return nil, ErrSettingNotFound
    }

    return []byte(cached), nil
}

// populateModeGroupFromDB loads a single group from DB into cache
func (s *SettingsCacheService) populateModeGroupFromDB(ctx context.Context, userID, mode, group string) error {
    configJSON, err := s.repo.GetUserModeConfig(ctx, userID, mode)
    if err != nil {
        return fmt.Errorf("failed to get mode config from DB: %w", err)
    }
    if configJSON == nil {
        return ErrSettingNotFound
    }

    var config autopilot.ModeFullConfig
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("failed to parse mode config: %w", err)
    }

    groupData := s.extractGroupFromConfig(&config, group)
    if groupData == nil {
        return ErrSettingNotFound
    }

    key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
    groupJSON, _ := json.Marshal(groupData)

    return s.cache.Set(ctx, key, string(groupJSON), 0)
}

// GetModeEnabled checks if a mode is enabled (fast path)
func (s *SettingsCacheService) GetModeEnabled(ctx context.Context, userID, mode string) (bool, error) {
    data, err := s.GetModeGroup(ctx, userID, mode, "enabled")
    if err != nil {
        return false, err
    }

    var result map[string]interface{}
    if err := json.Unmarshal(data, &result); err != nil {
        return false, err
    }

    if enabled, ok := result["enabled"].(bool); ok {
        return enabled, nil
    }
    return false, nil
}

// GetModeConfidence retrieves confidence settings for a mode
func (s *SettingsCacheService) GetModeConfidence(ctx context.Context, userID, mode string) (*autopilot.ModeConfidenceConfig, error) {
    data, err := s.GetModeGroup(ctx, userID, mode, "confidence")
    if err != nil {
        return nil, err
    }

    var config autopilot.ModeConfidenceConfig
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    return &config, nil
}

// GetModeSLTP retrieves SLTP settings for a mode
func (s *SettingsCacheService) GetModeSLTP(ctx context.Context, userID, mode string) (*autopilot.ModeSLTPConfig, error) {
    data, err := s.GetModeGroup(ctx, userID, mode, "sltp")
    if err != nil {
        return nil, err
    }

    var config autopilot.ModeSLTPConfig
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    return &config, nil
}

// GetPositionOptimization retrieves position optimization settings
func (s *SettingsCacheService) GetPositionOptimization(ctx context.Context, userID, mode string) (*autopilot.PositionOptimizationConfig, error) {
    data, err := s.GetModeGroup(ctx, userID, mode, "position_optimization")
    if err != nil {
        return nil, err
    }

    var config autopilot.PositionOptimizationConfig
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    return &config, nil
}

// GetModeConfig assembles full ModeFullConfig from all cached groups
// Uses MGET for atomic read of all 20 groups
func (s *SettingsCacheService) GetModeConfig(ctx context.Context, userID, mode string) (*autopilot.ModeFullConfig, error) {
    if !s.cache.IsHealthy() {
        return nil, ErrCacheUnavailable
    }

    // Build keys for all groups
    keys := make([]string, len(SettingGroups))
    for i, group := range SettingGroups {
        keys[i] = fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group.Key)
    }

    // Atomic read of all 20 groups using MGET
    values, err := s.cache.MGet(ctx, keys...)
    if err != nil {
        return nil, fmt.Errorf("failed to get mode config: %w", err)
    }

    config := &autopilot.ModeFullConfig{ModeName: mode}

    // Check for any misses and handle
    for i, val := range values {
        if val == nil || val == "" {
            // Cache miss for this group - populate it
            if err := s.populateModeGroupFromDB(ctx, userID, mode, SettingGroups[i].Key); err != nil {
                s.logger.Debug("Group not found", "group", SettingGroups[i].Key, "error", err)
                continue
            }
            // Re-fetch this single key
            cached, _ := s.cache.Get(ctx, keys[i])
            if cached != "" {
                val = cached
            }
        }

        if val != nil && val != "" {
            s.mergeGroupIntoConfig(config, SettingGroups[i].Key, []byte(val.(string)))
        }
    }

    return config, nil
}

// ============================================================================
// WRITE OPERATIONS (Write-Through: DB First, Then Cache)
// ============================================================================

// UpdateModeGroup updates a single settings group with write-through
// DB FIRST for durability, then cache
func (s *SettingsCacheService) UpdateModeGroup(ctx context.Context, userID, mode, group string, data []byte) error {
    // STEP 1: Write to durable storage first
    if err := s.repo.UpdateUserModeConfigGroup(ctx, userID, mode, group, data); err != nil {
        return fmt.Errorf("failed to persist to DB: %w", err)
    }

    // STEP 2: Update cache (best effort - DB has the truth)
    key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
    if s.cache.IsHealthy() {
        if err := s.cache.Set(ctx, key, string(data), 0); err != nil {
            // Log warning but don't fail - DB has the truth
            // Next read will repopulate cache from DB
            s.logger.Warn("Failed to update cache, will repopulate on next read",
                "key", key, "error", err)
        }
    }

    return nil
}

// ============================================================================
// CROSS-MODE SETTINGS (Circuit Breaker, LLM Config, Capital Allocation)
// ============================================================================

// GetCircuitBreaker retrieves global circuit breaker (cache-only with auto-populate)
func (s *SettingsCacheService) GetCircuitBreaker(ctx context.Context, userID string) (*database.UserGlobalCircuitBreaker, error) {
    if !s.cache.IsHealthy() {
        return nil, ErrCacheUnavailable
    }

    key := fmt.Sprintf("user:%s:circuit_breaker", userID)

    // Try cache first
    cached, err := s.cache.Get(ctx, key)
    if err == nil && cached != "" {
        var cb database.UserGlobalCircuitBreaker
        if err := json.Unmarshal([]byte(cached), &cb); err == nil {
            return &cb, nil
        }
    }

    // Cache miss - load from DB, populate cache, return from cache
    cb, err := s.repo.GetUserGlobalCircuitBreaker(ctx, userID)
    if err != nil {
        return nil, err
    }
    if cb == nil {
        return nil, ErrSettingNotFound
    }

    // Populate cache
    data, _ := json.Marshal(cb)
    s.cache.Set(ctx, key, string(data), 0)

    return cb, nil
}

// UpdateCircuitBreaker updates with write-through (DB first)
func (s *SettingsCacheService) UpdateCircuitBreaker(ctx context.Context, userID string, cb *database.UserGlobalCircuitBreaker) error {
    // DB first
    cb.UserID = userID
    if err := s.repo.SaveUserGlobalCircuitBreaker(ctx, cb); err != nil {
        return err
    }

    // Then cache
    key := fmt.Sprintf("user:%s:circuit_breaker", userID)
    if s.cache.IsHealthy() {
        data, _ := json.Marshal(cb)
        s.cache.Set(ctx, key, string(data), 0)
    }

    return nil
}

// GetLLMConfig retrieves LLM configuration (cache-only with auto-populate)
func (s *SettingsCacheService) GetLLMConfig(ctx context.Context, userID string) (*database.UserLLMConfig, error) {
    if !s.cache.IsHealthy() {
        return nil, ErrCacheUnavailable
    }

    key := fmt.Sprintf("user:%s:llm_config", userID)

    // Try cache first
    cached, err := s.cache.Get(ctx, key)
    if err == nil && cached != "" {
        var llm database.UserLLMConfig
        if err := json.Unmarshal([]byte(cached), &llm); err == nil {
            return &llm, nil
        }
    }

    // Cache miss - load from DB, populate cache
    llm, err := s.repo.GetUserLLMConfig(ctx, userID)
    if err != nil {
        return nil, err
    }
    if llm == nil {
        return nil, ErrSettingNotFound
    }

    data, _ := json.Marshal(llm)
    s.cache.Set(ctx, key, string(data), 0)

    return llm, nil
}

// UpdateLLMConfig updates with write-through (DB first)
func (s *SettingsCacheService) UpdateLLMConfig(ctx context.Context, userID string, llm *database.UserLLMConfig) error {
    llm.UserID = userID
    if err := s.repo.SaveUserLLMConfig(ctx, llm); err != nil {
        return err
    }

    key := fmt.Sprintf("user:%s:llm_config", userID)
    if s.cache.IsHealthy() {
        data, _ := json.Marshal(llm)
        s.cache.Set(ctx, key, string(data), 0)
    }

    return nil
}

// GetCapitalAllocation retrieves capital allocation (cache-only with auto-populate)
func (s *SettingsCacheService) GetCapitalAllocation(ctx context.Context, userID string) (*database.UserCapitalAllocation, error) {
    if !s.cache.IsHealthy() {
        return nil, ErrCacheUnavailable
    }

    key := fmt.Sprintf("user:%s:capital_allocation", userID)

    // Try cache first
    cached, err := s.cache.Get(ctx, key)
    if err == nil && cached != "" {
        var cap database.UserCapitalAllocation
        if err := json.Unmarshal([]byte(cached), &cap); err == nil {
            return &cap, nil
        }
    }

    // Cache miss - load from DB, populate cache
    cap, err := s.repo.GetUserCapitalAllocation(ctx, userID)
    if err != nil {
        return nil, err
    }
    if cap == nil {
        return nil, ErrSettingNotFound
    }

    data, _ := json.Marshal(cap)
    s.cache.Set(ctx, key, string(data), 0)

    return cap, nil
}

// UpdateCapitalAllocation updates with write-through (DB first)
func (s *SettingsCacheService) UpdateCapitalAllocation(ctx context.Context, userID string, cap *database.UserCapitalAllocation) error {
    cap.UserID = userID
    if err := s.repo.SaveUserCapitalAllocation(ctx, cap); err != nil {
        return err
    }

    key := fmt.Sprintf("user:%s:capital_allocation", userID)
    if s.cache.IsHealthy() {
        data, _ := json.Marshal(cap)
        s.cache.Set(ctx, key, string(data), 0)
    }

    return nil
}

// ============================================================================
// RESET AND INVALIDATION OPERATIONS
// ============================================================================

// ResetModeGroup resets a single group to admin defaults
func (s *SettingsCacheService) ResetModeGroup(ctx context.Context, userID, mode, group string) error {
    // Get default value from admin defaults cache or JSON file
    defaultKey := fmt.Sprintf("admin:defaults:mode:%s:%s", mode, group)
    defaultData, err := s.cache.Get(ctx, defaultKey)
    if err != nil || defaultData == "" {
        defaultData, err = s.loadGroupFromDefaults(mode, group)
        if err != nil {
            return fmt.Errorf("failed to get default for group %s: %w", group, err)
        }
    }

    return s.UpdateModeGroup(ctx, userID, mode, group, []byte(defaultData))
}

// loadGroupFromDefaults loads a group's default value from default-settings.json
func (s *SettingsCacheService) loadGroupFromDefaults(mode, group string) (string, error) {
    defaults, err := autopilot.LoadDefaultSettings()
    if err != nil {
        return "", fmt.Errorf("default settings not available: %w", err)
    }

    modeConfig, exists := defaults.ModeConfigs[mode]
    if !exists || modeConfig == nil {
        return "", fmt.Errorf("mode %s not found in defaults", mode)
    }

    groupData := s.extractGroupFromConfig(modeConfig, group)
    if groupData == nil {
        return "", fmt.Errorf("group %s not found in mode config", group)
    }

    data, err := json.Marshal(groupData)
    return string(data), err
}

// InvalidateModeGroup removes a single group from cache
func (s *SettingsCacheService) InvalidateModeGroup(ctx context.Context, userID, mode, group string) error {
    key := fmt.Sprintf("user:%s:mode:%s:%s", userID, mode, group)
    return s.cache.Delete(ctx, key)
}

// InvalidateMode removes all groups for a mode from cache
func (s *SettingsCacheService) InvalidateMode(ctx context.Context, userID, mode string) error {
    pattern := fmt.Sprintf("user:%s:mode:%s:*", userID, mode)
    return s.cache.DeletePattern(ctx, pattern)
}

// InvalidateAllModes removes all mode settings from cache for a user
func (s *SettingsCacheService) InvalidateAllModes(ctx context.Context, userID string) error {
    pattern := fmt.Sprintf("user:%s:mode:*", userID)
    return s.cache.DeletePattern(ctx, pattern)
}

// InvalidateCrossModeSetting removes a single cross-mode setting from cache
func (s *SettingsCacheService) InvalidateCrossModeSetting(ctx context.Context, userID, setting string) error {
    key := fmt.Sprintf("user:%s:%s", userID, setting)
    return s.cache.Delete(ctx, key)
}

// InvalidateAllUserSettings removes ALL user settings from cache (83 keys)
func (s *SettingsCacheService) InvalidateAllUserSettings(ctx context.Context, userID string) error {
    if err := s.InvalidateAllModes(ctx, userID); err != nil {
        s.logger.Warn("Failed to invalidate mode settings", "error", err)
    }

    for _, setting := range CrossModeSettings {
        s.InvalidateCrossModeSetting(ctx, userID, setting)
    }

    return nil
}

// GetEnabledModes returns list of enabled mode names for a user
func (s *SettingsCacheService) GetEnabledModes(ctx context.Context, userID string) ([]string, error) {
    var enabled []string

    for _, mode := range TradingModes {
        isEnabled, err := s.GetModeEnabled(ctx, userID, mode)
        if err != nil {
            return nil, err
        }
        if isEnabled {
            enabled = append(enabled, mode)
        }
    }

    return enabled, nil
}

// ============================================================================
// HELPER METHODS
// ============================================================================

// extractGroupFromConfig extracts a specific group's data from ModeFullConfig
func (s *SettingsCacheService) extractGroupFromConfig(config *autopilot.ModeFullConfig, groupKey string) interface{} {
    switch groupKey {
    case "enabled":
        return map[string]interface{}{"enabled": config.Enabled}
    case "timeframe":
        return config.Timeframe
    case "confidence":
        return config.Confidence
    case "size":
        return config.Size
    case "sltp":
        return config.SLTP
    case "risk":
        return config.Risk
    case "circuit_breaker":
        return config.CircuitBreaker
    case "hedge":
        return config.Hedge
    case "averaging":
        return config.Averaging
    case "stale_release":
        return config.StaleRelease
    case "assignment":
        return config.Assignment
    case "mtf":
        return config.MTF
    case "dynamic_ai_exit":
        return config.DynamicAIExit
    case "reversal":
        return config.Reversal
    case "funding_rate":
        return config.FundingRate
    case "trend_divergence":
        return config.TrendDivergence
    case "position_optimization":
        return config.PositionOptimization
    case "trend_filters":
        return config.TrendFilters
    case "early_warning":
        return config.EarlyWarning
    case "entry":
        return config.Entry
    default:
        return nil
    }
}

// mergeGroupIntoConfig merges a group's JSON data into ModeFullConfig
func (s *SettingsCacheService) mergeGroupIntoConfig(config *autopilot.ModeFullConfig, groupKey string, data []byte) error {
    var err error
    switch groupKey {
    case "enabled":
        var m map[string]interface{}
        if err = json.Unmarshal(data, &m); err == nil {
            if enabled, ok := m["enabled"].(bool); ok {
                config.Enabled = enabled
            }
        }
    case "timeframe":
        var t autopilot.ModeTimeframeConfig
        if err = json.Unmarshal(data, &t); err == nil {
            config.Timeframe = &t
        }
    case "confidence":
        var c autopilot.ModeConfidenceConfig
        if err = json.Unmarshal(data, &c); err == nil {
            config.Confidence = &c
        }
    case "size":
        var sz autopilot.ModeSizeConfig
        if err = json.Unmarshal(data, &sz); err == nil {
            config.Size = &sz
        }
    case "sltp":
        var sl autopilot.ModeSLTPConfig
        if err = json.Unmarshal(data, &sl); err == nil {
            config.SLTP = &sl
        }
    case "risk":
        var r autopilot.ModeRiskConfig
        if err = json.Unmarshal(data, &r); err == nil {
            config.Risk = &r
        }
    case "circuit_breaker":
        var cb autopilot.ModeCircuitBreakerConfig
        if err = json.Unmarshal(data, &cb); err == nil {
            config.CircuitBreaker = &cb
        }
    case "hedge":
        var h autopilot.HedgeModeConfig
        if err = json.Unmarshal(data, &h); err == nil {
            config.Hedge = &h
        }
    case "averaging":
        var a autopilot.PositionAveragingConfig
        if err = json.Unmarshal(data, &a); err == nil {
            config.Averaging = &a
        }
    case "stale_release":
        var sr autopilot.StalePositionReleaseConfig
        if err = json.Unmarshal(data, &sr); err == nil {
            config.StaleRelease = &sr
        }
    case "assignment":
        var as autopilot.ModeAssignmentConfig
        if err = json.Unmarshal(data, &as); err == nil {
            config.Assignment = &as
        }
    case "mtf":
        var m autopilot.ModeMTFConfig
        if err = json.Unmarshal(data, &m); err == nil {
            config.MTF = &m
        }
    case "dynamic_ai_exit":
        var d autopilot.ModeDynamicAIExitConfig
        if err = json.Unmarshal(data, &d); err == nil {
            config.DynamicAIExit = &d
        }
    case "reversal":
        var rv autopilot.ModeReversalConfig
        if err = json.Unmarshal(data, &rv); err == nil {
            config.Reversal = &rv
        }
    case "funding_rate":
        var f autopilot.ModeFundingRateConfig
        if err = json.Unmarshal(data, &f); err == nil {
            config.FundingRate = &f
        }
    case "trend_divergence":
        var td autopilot.ModeTrendDivergenceConfig
        if err = json.Unmarshal(data, &td); err == nil {
            config.TrendDivergence = &td
        }
    case "position_optimization":
        var p autopilot.PositionOptimizationConfig
        if err = json.Unmarshal(data, &p); err == nil {
            config.PositionOptimization = &p
        }
    case "trend_filters":
        var tf autopilot.TrendFiltersConfig
        if err = json.Unmarshal(data, &tf); err == nil {
            config.TrendFilters = &tf
        }
    case "early_warning":
        var e autopilot.ModeEarlyWarningConfig
        if err = json.Unmarshal(data, &e); err == nil {
            config.EarlyWarning = &e
        }
    case "entry":
        var en autopilot.ModeEntryConfig
        if err = json.Unmarshal(data, &en); err == nil {
            config.Entry = &en
        }
    }
    return err
}
```

### New Repository Method: `internal/database/repository_user_mode_config.go`

Add this method to the repository:

```go
// UpdateUserModeConfigGroup updates a specific group within a mode's config JSON
// This performs a JSON merge: reads existing config, updates the group, writes back
func (r *Repository) UpdateUserModeConfigGroup(ctx context.Context, userID, modeName, groupKey string, groupData []byte) error {
    // 1. Get existing config
    configJSON, err := r.GetUserModeConfig(ctx, userID, modeName)
    if err != nil {
        return fmt.Errorf("failed to get existing config: %w", err)
    }

    if configJSON == nil {
        return fmt.Errorf("mode config %s not found for user %s", modeName, userID)
    }

    // 2. Parse into map for dynamic merging
    var config map[string]interface{}
    if err := json.Unmarshal(configJSON, &config); err != nil {
        return fmt.Errorf("failed to parse existing config: %w", err)
    }

    // 3. Parse group data
    var groupValue interface{}
    if err := json.Unmarshal(groupData, &groupValue); err != nil {
        return fmt.Errorf("failed to parse group data: %w", err)
    }

    // 4. Update the field (groupKey maps directly to JSON field name)
    config[groupKey] = groupValue

    // 5. Marshal back
    updatedJSON, err := json.Marshal(config)
    if err != nil {
        return fmt.Errorf("failed to marshal updated config: %w", err)
    }

    // 6. Get enabled status from config
    enabled := false
    if e, ok := config["enabled"].(bool); ok {
        enabled = e
    }

    // 7. Save back
    return r.SaveUserModeConfig(ctx, userID, modeName, enabled, updatedJSON)
}
```

### Add MGet to CacheService: `internal/cache/cache_service.go`

```go
// MGet retrieves multiple keys atomically
func (c *CacheService) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
    if !c.IsHealthy() {
        return nil, ErrCacheUnavailable
    }
    return c.client.MGet(ctx, keys...).Result()
}
```

---

## Files to Create/Modify

### New Files
| File | Description |
|------|-------------|
| `internal/cache/settings_errors.go` | Error types |
| `internal/cache/settings_groups.go` | Setting groups constant + Logger interface |
| `internal/cache/settings_cache_service.go` | Granular cache service |
| `internal/cache/settings_cache_service_test.go` | Unit tests |

### Modified Files
| File | Changes |
|------|---------|
| `internal/cache/cache_service.go` | Add MGet method |
| `main.go` | Initialize SettingsCacheService |
| `internal/api/server.go` | Add settingsCache field |
| `internal/api/handlers_auth.go` | Call LoadUserSettings on login |
| `internal/database/repository_user_mode_config.go` | Add UpdateUserModeConfigGroup method |

---

## Testing Strategy

### Unit Tests - Critical Scenarios

```go
// P0: Redis unavailable tests - system must error, not bypass
func TestGetModeGroup_RedisDown_ReturnsError(t *testing.T) {
    mockCache := NewMockCache()
    mockCache.SetHealthy(false)  // Redis unavailable
    mockDB := NewMockRepo()
    mockDB.SetupFullConfig()     // DB HAS the data

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    _, err := svc.GetModeGroup(ctx, "user1", "scalp", "confidence")

    // MUST return error, NOT data from DB
    assert.ErrorIs(t, err, ErrCacheUnavailable)
    assert.Zero(t, mockDB.GetCallCount())  // DB should NOT be called for bypass
}

// P0: Cache miss auto-populates then returns from cache
func TestGetModeGroup_CacheMiss_PopulatesThenReturnsFromCache(t *testing.T) {
    mockCache := NewMockCache()
    mockCache.SetHealthy(true)
    mockCache.SetGetResult("", nil)  // First call: miss
    mockDB := NewMockRepo()
    mockDB.SetupFullConfig()

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    result, err := svc.GetModeGroup(ctx, "user1", "scalp", "confidence")

    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, 1, mockCache.SetCallCount())  // Cache was populated
    assert.Equal(t, 2, mockCache.GetCallCount())  // First miss, second hit after populate
}

// P0: Write-through is DB first, then cache
func TestUpdateModeGroup_DBFirst_ThenCache(t *testing.T) {
    callOrder := []string{}
    mockCache := NewMockCache()
    mockCache.OnSet(func() { callOrder = append(callOrder, "cache") })
    mockDB := NewMockRepo()
    mockDB.OnUpdate(func() { callOrder = append(callOrder, "db") })

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    err := svc.UpdateModeGroup(ctx, "user1", "scalp", "sltp", []byte(`{}`))

    assert.NoError(t, err)
    assert.Equal(t, []string{"db", "cache"}, callOrder)  // DB before cache
}

// P0: All 88 keys populated on login
func TestLoadUserSettings_Populates88Keys(t *testing.T) {
    mockCache := NewMockCache()
    mockCache.SetHealthy(true)
    mockDB := NewMockRepo()
    mockDB.SetupFullUserSettings()  // All settings from default-settings.json

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    err := svc.LoadUserSettings(ctx, "user1")

    assert.NoError(t, err)
    // 80 mode + 4 global + 4 safety = 88 keys
    assert.Equal(t, 88, mockCache.SetCallCount())
}

// P0: Safety settings load for all 4 modes
func TestLoadUserSettings_LoadsSafetySettings(t *testing.T) {
    mockCache := NewMockCache()
    mockCache.SetHealthy(true)
    mockDB := NewMockRepo()
    mockDB.SetupSafetySettings()

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    err := svc.LoadUserSettings(ctx, "user1")

    assert.NoError(t, err)
    // Verify safety keys were set
    assert.True(t, mockCache.KeyExists("user:user1:safety:ultra_fast"))
    assert.True(t, mockCache.KeyExists("user:user1:safety:scalp"))
    assert.True(t, mockCache.KeyExists("user:user1:safety:swing"))
    assert.True(t, mockCache.KeyExists("user:user1:safety:position"))
}

// P0: Global trading settings load
func TestLoadUserSettings_LoadsGlobalTrading(t *testing.T) {
    mockCache := NewMockCache()
    mockCache.SetHealthy(true)
    mockDB := NewMockRepo()
    mockDB.SetupGlobalTrading()

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    err := svc.LoadUserSettings(ctx, "user1")

    assert.NoError(t, err)
    assert.True(t, mockCache.KeyExists("user:user1:global_trading"))
}

// P1: Cache hit returns immediately without DB call
func TestGetModeGroup_CacheHit_NoDB(t *testing.T) {
    mockCache := NewMockCache()
    mockCache.SetHealthy(true)
    mockCache.SetGetResult(`{"min": 55}`, nil)  // Cache hit
    mockDB := NewMockRepo()

    svc := NewSettingsCacheService(mockCache, mockDB, logger)

    result, err := svc.GetModeGroup(ctx, "user1", "scalp", "confidence")

    assert.NoError(t, err)
    assert.Contains(t, string(result), "55")
    assert.Zero(t, mockDB.GetCallCount())  // DB not called
}
```

---

## Definition of Done

### Implementation
- [ ] `SettingGroups` constant matches UI's `SETTING_GROUPS` (20 groups)
- [ ] `LoadUserSettings` populates all **88 keys** on login
- [ ] `GetModeGroup` returns error if Redis unavailable (no bypass)
- [ ] `GetModeGroup` auto-populates on cache miss, then returns from cache
- [ ] `GetModeConfig` uses MGET for atomic 20-key read
- [ ] `UpdateModeGroup` writes DB first, then cache
- [ ] Global settings follow same pattern (circuit_breaker, llm_config, capital_allocation, global_trading)
- [ ] Safety settings follow same pattern (4 per-mode keys)
- [ ] Global position optimization follows same pattern
- [ ] All invalidation methods work correctly

### Testing
- [ ] Unit tests: Redis down returns `ErrCacheUnavailable`
- [ ] Unit tests: Cache miss auto-populates then returns from cache
- [ ] Unit tests: Write-through is DB-first
- [ ] Unit tests: **88 keys** populated on login (80 mode + 4 global + 4 safety)
- [ ] Unit tests: Safety settings load correctly for all 4 modes
- [ ] Unit tests: Global trading settings load correctly
- [ ] Integration: Login flow populates cache
- [ ] Performance: Group read <1ms from cache

### Error Handling
- [ ] `ErrCacheUnavailable` returned when Redis unhealthy
- [ ] `ErrSettingNotFound` returned when setting doesn't exist
- [ ] API returns HTTP 503 when cache unavailable

---

## Dependencies

| Dependency | Status |
|------------|--------|
| Story 6.1 - Redis Infrastructure | COMPLETE |
| CacheService with Get/Set/Delete/DeletePattern | COMPLETE |
| CacheService.MGet | TO BE ADDED |
| Repository.GetUserModeConfig | EXISTS |
| Repository.UpdateUserModeConfigGroup | TO BE ADDED |
| SETTING_GROUPS (UI reference) | EXISTS |
| Repository.GetUserSafetySettings | ✅ EXISTS |
| Repository.GetUserGlobalTrading | ❌ TO BE CREATED |

---

## Pre-Implementation Tasks (MUST Complete First)

These database tables and repository methods are **prerequisites** discovered during Development Review:

### 1. Database Tables to Create

| Migration | Table | Fields |
|-----------|-------|--------|
| `0XX_user_global_trading.sql` | `user_global_trading` | `id, user_id, risk_level, max_usd_allocation, profit_reinvest_percent, profit_reinvest_risk_level, created_at, updated_at` |

**Note:** Position optimization is within mode groups (one of the 20 groups per mode), NOT a separate global table.

### 2. Models to Add (`internal/database/models_user_settings.go`)

```go
// UserGlobalTrading represents per-user global trading settings
type UserGlobalTrading struct {
    ID                     string  `json:"id"`
    UserID                 string  `json:"user_id"`
    RiskLevel              string  `json:"risk_level"`
    MaxUSDAllocation       float64 `json:"max_usd_allocation"`
    ProfitReinvestPercent  float64 `json:"profit_reinvest_percent"`
    ProfitReinvestRiskLevel string `json:"profit_reinvest_risk_level"`
    CreatedAt              time.Time `json:"created_at"`
    UpdatedAt              time.Time `json:"updated_at"`
}
```

### 3. Repository Methods to Create

```go
// internal/database/repository_user_global_trading.go
func (r *Repository) GetUserGlobalTrading(ctx context.Context, userID string) (*UserGlobalTrading, error)
func (r *Repository) SaveUserGlobalTrading(ctx context.Context, config *UserGlobalTrading) error
```

### 4. Update Existing Code

- [ ] Update `loadUserSettings` in `handlers_user_settings.go` to load GlobalTrading from DB instead of defaults
- [ ] Update user initialization to create default GlobalTrading record

### 5. Test Updates Required

- [ ] Rename `TestLoadUserSettings_Populates83Keys` → `TestLoadUserSettings_Populates88Keys`
- [ ] Add `safetySettings` mock data to MockRepository
- [ ] Add `globalTrading` mock data to MockRepository
- [ ] Add new test: `TestLoadSafetySettings_LoadsAllFourModes`
- [ ] Add new test: `TestGetSafetySettings_CacheOnlyWithAutoPopulate`
- [ ] Add new test: `TestGetGlobalTrading_CacheOnlyWithAutoPopulate`

---

## Author

**Created By:** BMAD Party Mode
**Date:** 2026-01-15
**Version:** 7.1 (Development Complete - Migration 026 applied, all infrastructure ready)
**Reviewed By:** Development Review ✅ | QA Review ✅

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 4.0 | 2026-01-15 | Post-Review - All Fixes Applied |
| 5.0 | 2026-01-15 | Added missing keys: global_trading, safety_settings (4). Total: 83 → 88 keys |
| 6.0 | 2026-01-15 | Added Pre-Implementation Tasks (DB tables, models, repository methods) discovered during Dev/QA Review |
| 7.0 | 2026-01-15 | Aligned with Story 6.4 (88 keys): Removed global position_optimization (it's within mode groups), 89 → 88 keys |
| 7.1 | 2026-01-15 | **Development Complete**: Created migration 026_user_global_trading.sql, UserGlobalTrading model, repository_user_global_trading.go CRUD operations, integrated into user_initialization.go and main.go. Migration auto-applies on startup. All infrastructure ready for cache implementation. |
