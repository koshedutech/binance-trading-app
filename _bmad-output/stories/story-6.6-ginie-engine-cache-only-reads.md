# Story 6.6: Ginie Engine Cache-Only Reads

**Story ID:** CACHE-6.6
**Epic:** Epic 6 - Redis Caching Infrastructure
**Priority:** P0 (Critical - Trading performance)
**Story Points:** 8
**Estimated Effort:** 10 hours
**Author:** BMAD Party Mode (Bob - Scrum Master)
**Status:** Done
**Version:** 4.0 (Implementation Complete - January 2026)
**Sprint Planned:** 2026-01-15

---

## Current State Analysis (January 2026)

### SettingsCacheService: EXISTS AND READY ✅

The `SettingsCacheService` in `/internal/cache/settings_cache_service.go` already has ALL the methods needed:

| Method | Purpose | Status |
|--------|---------|--------|
| `GetModeConfig()` | Full mode config from cache | ✅ Ready |
| `GetModeConfidence()` | Confidence settings | ✅ Ready |
| `GetModeSLTP()` | SLTP settings | ✅ Ready |
| `GetPositionOptimization()` | Position optimization | ✅ Ready |
| `GetCircuitBreaker()` | Global circuit breaker | ✅ Ready |
| `GetCapitalAllocation()` | Capital allocation | ✅ Ready |
| `GetLLMConfig()` | LLM configuration | ✅ Ready |
| `GetSafetySettings()` | Safety settings per mode | ✅ Ready |
| `GetModeEnabled()` | Mode enabled status | ✅ Ready |
| `GetEnabledModes()` | List of enabled modes | ✅ Ready |
| `IsHealthy()` | Cache health check | ✅ Ready |
| `LoadUserSettings()` | Load all 83 keys on login | ✅ Ready |

### Injection Chain: NOT WIRED ❌

**Current injection path is BROKEN:**
```
main.go:257           → settingsCacheService CREATED ✅
main.go:946           → NewUserAutopilotManager() MISSING settingsCacheService ❌
UserAutopilotManager  → NewGinieAutopilot() MISSING settingsCacheService ❌
GinieAutopilot        → Uses repo.Get*() DIRECT DB CALLS ❌
```

**Required injection path:**
```
main.go               → Pass settingsCacheService to NewUserAutopilotManager()
UserAutopilotManager  → Store and pass to NewGinieAutopilot()
GinieAutopilot        → Use settingsCache.Get*() for ALL reads
FuturesController     → Also needs settingsCache injection
GinieAnalyzer         → Also needs settingsCache injection
```

### Direct DB Calls in Autopilot: 25+ LOCATIONS ❌

**File: `ginie_autopilot.go`**
| Line | Call | Fix |
|------|------|-----|
| 1159 | `repo.GetUserGlobalCircuitBreaker()` | `settingsCache.GetCircuitBreaker()` |
| 1237 | `GetUserModeConfigFromDB(ultra_fast)` | `settingsCache.GetModeConfig()` |
| 1248 | `GetUserModeConfigFromDB(scalp)` | `settingsCache.GetModeConfig()` |
| 1259 | `GetUserModeConfigFromDB(swing)` | `settingsCache.GetModeConfig()` |
| 1270 | `GetUserModeConfigFromDB(position)` | `settingsCache.GetModeConfig()` |
| 1380 | `GetUserModeConfigFromDB()` in loop | `settingsCache.GetModeConfig()` |
| 1690 | `repo.GetUserCapitalAllocation()` | `settingsCache.GetCapitalAllocation()` |
| 2339 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |
| 7209 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |

**File: `futures_controller.go`**
| Line | Call | Fix |
|------|------|-----|
| 1432 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |
| 2736 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |
| 2949 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |
| 3100 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |
| 4320 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |
| 4347 | `GetUserModeConfigFromDB(conservative)` | `settingsCache.GetModeConfig()` |
| 4353 | `GetUserModeConfigFromDB(aggressive)` | `settingsCache.GetModeConfig()` |
| 4565 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |

**File: `ginie_analyzer.go`**
| Line | Call | Fix |
|------|------|-----|
| 1794 | `GetUserModeConfigFromDB()` | `settingsCache.GetModeConfig()` |

**File: `admin_sync.go`**
| Line | Call | Fix |
|------|------|-----|
| 208 | `repo.GetUserCapitalAllocation()` | `settingsCache.GetCapitalAllocation()` |
| 225 | `repo.GetUserGlobalCircuitBreaker()` | `settingsCache.GetCircuitBreaker()` |
| 244 | `repo.GetUserLLMConfig()` | `settingsCache.GetLLMConfig()` |

**File: `settings.go`**
| Line | Call | Fix |
|------|------|-----|
| 3864 | `repo.GetUserCapitalAllocation()` | `settingsCache.GetCapitalAllocation()` |

---

## Description

Modify the Ginie Autopilot engine to read ALL settings exclusively from **SettingsCacheService** using granular Redis keys. If Redis is unavailable, trading **STOPS** (no silent DB fallback). This ensures:

1. **Sub-millisecond settings access** during trading cycles
2. **Consistent behavior** with "Redis is the brain" architecture
3. **Zero DB queries** during active trading

---

## User Story

> As the Ginie Autopilot engine,
> I want to read all settings from Redis cache using granular keys,
> So that my trading decision cycles have <5ms total settings access instead of 250ms+ DB queries.

---

## Critical Architecture Principle

```
┌─────────────────────────────────────────────────────────────────────────┐
│              GINIE ENGINE - CACHE ONLY (NO DB DURING TRADING)            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Trading Cycle                                                           │
│       │                                                                  │
│       v                                                                  │
│  ┌─────────────────────┐                                                │
│  │ SettingsCacheService│ ← Granular reads via GetModeGroup()            │
│  └──────────┬──────────┘                                                │
│             │                                                            │
│      ┌──────┴──────┐                                                    │
│      │             │                                                     │
│      v             v                                                     │
│  Redis OK     Redis DOWN                                                │
│      │             │                                                     │
│      v             v                                                     │
│  Continue     STOP TRADING                                              │
│  Trading      (ErrCacheUnavailable)                                     │
│                                                                          │
│  RATIONALE: Trading with stale/inconsistent settings is DANGEROUS.       │
│  Better to stop than trade with wrong parameters.                        │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Acceptance Criteria

### AC6.6.1: Ginie Autopilot Uses SettingsCacheService
- [ ] Inject `SettingsCacheService` into `GinieAutopilot`
- [ ] Remove all direct `repository.Get*()` calls for settings
- [ ] Use granular methods: `GetModeGroup()`, `GetModeEnabled()`, `GetModeConfidence()`, etc.

### AC6.6.2: Trading Cycle Settings Reads
- [ ] `GenerateDecision()` reads confidence from `GetModeConfidence(ctx, userID, mode)`
- [ ] SLTP settings from `GetModeSLTP(ctx, userID, mode)`
- [ ] Position optimization from `GetPositionOptimization(ctx, userID, mode)`
- [ ] Circuit breaker from `GetCircuitBreaker(ctx, userID)`
- [ ] Capital allocation from `GetCapitalAllocation(ctx, userID)`

### AC6.6.3: Redis Down = Trading Stops
- [ ] If `ErrCacheUnavailable` returned, trading loop pauses
- [ ] Log error: "Trading paused - cache unavailable"
- [ ] Retry connection every 5 seconds
- [ ] Resume trading when Redis reconnects
- [ ] **NO silent fallback to DB**

### AC6.6.4: Cache Warm-Up on Startup
- [ ] Call `SettingsCacheService.LoadUserSettings(ctx, userID)` on Ginie startup
- [ ] If warm-up fails (Redis down), Ginie does NOT start trading
- [ ] Log: "Ginie startup blocked - cache unavailable"

### AC6.6.5: Performance Target
- [ ] Total settings load time per cycle: <5ms
- [ ] Individual setting read: <1ms
- [ ] Zero DB queries during trading cycles (verified via logging)

### AC6.6.6: Settings Change Detection
- [ ] When user updates settings via API, cache is updated (per 6.2 write-through)
- [ ] Next Ginie cycle automatically reads new values from cache
- [ ] No restart required for settings changes

---

## Sprint Tasks (Updated January 2026)

| Task | Description | Files | Points |
|------|-------------|-------|--------|
| **6.6.1** | Wire SettingsCacheService injection chain | `main.go`, `user_autopilot_manager.go` | 1 |
| **6.6.2** | Add settingsCache to GinieAutopilot struct and constructor | `ginie_autopilot.go` | 1 |
| **6.6.3** | Replace DB calls in GinieAutopilot initialization | `ginie_autopilot.go:1159-1270` | 1 |
| **6.6.4** | Replace DB calls in GinieAutopilot trading loops | `ginie_autopilot.go:1380,1690,2339,7209` | 2 |
| **6.6.5** | Add settingsCache to FuturesController and replace DB calls | `futures_controller.go` (8 locations) | 2 |
| **6.6.6** | Add settingsCache to GinieAnalyzer and replace DB call | `ginie_analyzer.go:1794` | 1 |
| **6.6.7** | Add cache unavailable handling (trading stops) | All trading loops | 1 |
| **6.6.8** | Verify build and test cache-first behavior | Compile + runtime test | 1 |

**Total: 10 Story Points** (updated from 8 due to larger scope)

---

## Technical Specification

### Before: DB Queries Every Cycle (250ms+)

```go
func (g *GinieAutopilot) GenerateDecision(ctx context.Context, symbol string) (*Decision, error) {
    // OLD: Multiple DB queries per cycle
    mode := g.getCurrentMode()                                    // 50ms
    configJSON, _ := g.repo.GetUserModeConfig(ctx, g.userID, mode) // 50ms
    cbConfig, _ := g.repo.GetUserGlobalCircuitBreaker(ctx, g.userID) // 50ms
    capitalAlloc, _ := g.repo.GetUserCapitalAllocation(ctx, g.userID) // 50ms
    // Parse JSON, extract fields...
    // TOTAL: 200-300ms wasted on settings reads

    // Trading logic...
}
```

### After: Cache-Only via SettingsCacheService (<5ms)

```go
func (g *GinieAutopilot) GenerateDecision(ctx context.Context, symbol string) (*Decision, error) {
    // NEW: Granular cache reads via SettingsCacheService

    // Check if mode is enabled (<1ms)
    enabled, err := g.settingsCache.GetModeEnabled(ctx, g.userID, g.currentMode)
    if err != nil {
        if errors.Is(err, cache.ErrCacheUnavailable) {
            return nil, g.handleCacheUnavailable(err)
        }
        return nil, err
    }
    if !enabled {
        return nil, nil // Mode disabled, skip
    }

    // Get confidence settings (<1ms)
    confidence, err := g.settingsCache.GetModeConfidence(ctx, g.userID, g.currentMode)
    if err != nil {
        return nil, g.handleCacheUnavailable(err)
    }

    // Get SLTP settings (<1ms)
    sltp, err := g.settingsCache.GetModeSLTP(ctx, g.userID, g.currentMode)
    if err != nil {
        return nil, g.handleCacheUnavailable(err)
    }

    // Get position optimization (<1ms)
    posOpt, err := g.settingsCache.GetPositionOptimization(ctx, g.userID, g.currentMode)
    if err != nil {
        return nil, g.handleCacheUnavailable(err)
    }

    // Get circuit breaker (<1ms)
    circuitBreaker, err := g.settingsCache.GetCircuitBreaker(ctx, g.userID)
    if err != nil {
        return nil, g.handleCacheUnavailable(err)
    }

    // TOTAL: <5ms for ALL settings reads (50x faster)

    // Trading decision logic with typed configs...
    return g.makeDecision(confidence, sltp, posOpt, circuitBreaker)
}

func (g *GinieAutopilot) handleCacheUnavailable(err error) error {
    if errors.Is(err, cache.ErrCacheUnavailable) {
        g.logger.Error("Trading paused - cache unavailable", "userID", g.userID)
        g.pauseTrading()
        return fmt.Errorf("trading paused: %w", err)
    }
    return err
}
```

### Ginie Startup with Cache Warm-Up

```go
func (g *GinieAutopilot) Start(ctx context.Context) error {
    // CRITICAL: Warm up cache before trading starts
    g.logger.Info("Warming up settings cache", "userID", g.userID)

    if err := g.settingsCache.LoadUserSettings(ctx, g.userID); err != nil {
        if errors.Is(err, cache.ErrCacheUnavailable) {
            g.logger.Error("Ginie startup blocked - cache unavailable", "userID", g.userID)
            return fmt.Errorf("cannot start trading without cache: %w", err)
        }
        return err
    }

    g.logger.Info("Cache warm-up complete, starting trading loop", "userID", g.userID)

    // Start the trading loop
    go g.tradingLoop(ctx)

    return nil
}
```

### Trading Loop with Cache Health Check

```go
func (g *GinieAutopilot) tradingLoop(ctx context.Context) {
    ticker := time.NewTicker(g.cycleInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Check cache health before each cycle
            if !g.settingsCache.IsHealthy() {
                g.logger.Warn("Trading cycle skipped - cache unhealthy", "userID", g.userID)
                continue
            }

            // Execute trading cycle
            start := time.Now()
            if err := g.executeCycle(ctx); err != nil {
                if errors.Is(err, cache.ErrCacheUnavailable) {
                    g.logger.Error("Trading paused due to cache failure", "error", err)
                    continue
                }
                g.logger.Error("Trading cycle failed", "error", err)
            }

            // Performance monitoring
            elapsed := time.Since(start)
            if elapsed > 100*time.Millisecond {
                g.logger.Warn("Slow trading cycle", "elapsed", elapsed)
            }
        }
    }
}
```

### GinieAutopilot Structure Update

```go
type GinieAutopilot struct {
    userID        string
    currentMode   string
    settingsCache *cache.SettingsCacheService  // NEW: Cache service
    // Remove: repo *database.Repository (no direct DB access)
    logger        Logger
    cycleInterval time.Duration
    // ... other fields
}

func NewGinieAutopilot(
    userID string,
    settingsCache *cache.SettingsCacheService,
    logger Logger,
) *GinieAutopilot {
    return &GinieAutopilot{
        userID:        userID,
        settingsCache: settingsCache,
        logger:        logger,
        cycleInterval: 1 * time.Second,
    }
}
```

---

## Files to Modify (Updated January 2026)

| File | Changes | Priority |
|------|---------|----------|
| `main.go` | Pass `settingsCacheService` to `NewUserAutopilotManager()` | P0 |
| `internal/autopilot/user_autopilot_manager.go` | Add `settingsCache` field, pass to `NewGinieAutopilot()` | P0 |
| `internal/autopilot/ginie_autopilot.go` | Add `settingsCache` field, replace 9+ DB calls with cache calls | P0 |
| `internal/autopilot/futures_controller.go` | Add `settingsCache` field, replace 8+ DB calls with cache calls | P0 |
| `internal/autopilot/ginie_analyzer.go` | Add `settingsCache` field, replace DB call with cache call | P1 |
| `internal/autopilot/admin_sync.go` | Replace 3 DB calls with cache calls | P1 |
| `internal/autopilot/settings.go` | Replace capital allocation DB call with cache call | P1 |

**Note:** `settings_manager.go` doesn't exist - the `GetUserModeConfigFromDB()` function is in `settings.go` and will remain for admin operations. Trading operations will use cache.

---

## Testing Requirements

### Unit Tests (P0)
```go
// Cache unavailable stops trading (no DB fallback)
func TestGenerateDecision_CacheUnavailable_StopsTrading(t *testing.T)

// Successful cycle reads from cache
func TestGenerateDecision_CacheHit_ReturnsDecision(t *testing.T)

// Startup fails if cache unavailable
func TestStart_CacheUnavailable_ReturnsError(t *testing.T)

// Settings change reflected in next cycle
func TestSettingsChange_NextCycleReadsNewValues(t *testing.T)

// Zero DB queries during trading
func TestTradingCycle_NoDBQueries(t *testing.T)
```

### Integration Tests
- Full trading cycle with Redis
- Startup with cache warm-up
- Settings update → Cache update → Next cycle uses new settings
- Redis failure → Trading pauses → Redis recovers → Trading resumes

### Performance Tests
- **Baseline**: Current cycle time with DB (250ms+ for settings)
- **Target**: <5ms for all settings reads
- **Load**: 1000 cycles, verify consistent <5ms settings load
- **Measurement**: Add timing logs for each cache read

---

## Definition of Done

### Implementation
- [ ] GinieAutopilot uses SettingsCacheService exclusively
- [ ] Zero direct DB queries for settings during trading
- [ ] ErrCacheUnavailable pauses trading (no fallback)
- [ ] Cache warm-up on startup
- [ ] Performance monitoring added

### Testing
- [ ] Unit tests: Cache unavailable stops trading
- [ ] Unit tests: Settings reads from cache
- [ ] Integration: Full trading cycle with cache
- [ ] Performance: <5ms total settings load verified

### Observability
- [ ] Logging: Cache unavailable events
- [ ] Logging: Slow cycle warnings (>100ms)
- [ ] Metrics: Settings load time per cycle

---

## Dependencies

| Dependency | Status |
|------------|--------|
| Story 6.2 - User Settings Cache | COMPLETE |
| Story 6.4 - Admin Defaults Cache | Required (for resets) |
| Story 6.5 - Cache-First APIs | Required (for consistency) |
| SettingsCacheService | EXISTS |
| GetModeConfidence(), GetModeSLTP(), etc. | EXISTS |

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Redis goes down during trading | Trading pauses, logs error, waits for reconnect |
| Stale settings in cache | Write-through pattern ensures cache always updated |
| Cache miss during trading | Auto-populate on miss (6.2 pattern) |
| Settings change not reflected | Write-through updates cache immediately |

---

## Author

**Created By:** BMAD Party Mode (Bob, Winston, Mary, John, Murat, Amelia)
**Date:** 2026-01-15
**Version:** 2.1 (Sprint planned)
**Sprint Planned:** 2026-01-15
**Status:** Ready for Development
