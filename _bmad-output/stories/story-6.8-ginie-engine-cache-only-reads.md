# Story 6.8: Ginie Engine Reads from Cache Only
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 5
**Priority:** P0

## User Story
As the Ginie Autopilot engine, I want to read all settings exclusively from Redis cache so that my trading decision cycles are <1ms for settings access instead of 250ms+ DB queries.

## Acceptance Criteria
- [ ] `GinieAutopilot.GenerateDecision()` reads mode config from cache
- [ ] `FuturesController` reads mode settings from cache
- [ ] Circuit breaker checks use cached limits
- [ ] Scalp reentry logic uses cached config
- [ ] Capital allocation uses cached percentages
- [ ] Zero DB queries during active trading cycles
- [ ] Performance: Settings access <1ms (measured)

## Technical Approach

### Before: DB Queries Every Cycle
```go
func (g *GinieAutopilot) GenerateDecision(symbol string) (*Decision, error) {
    mode := g.settingsManager.GetCurrentMode()          // DB query: 50ms
    config := g.db.GetModeConfig(g.userID, mode)        // DB query: 50ms
    cb := g.db.GetCircuitBreaker(g.userID)              // DB query: 50ms
    scalpConfig := g.db.GetScalpReentry(g.userID)       // DB query: 50ms
    capitalAlloc := g.db.GetCapitalAllocation(g.userID) // DB query: 50ms
    // TOTAL: 250ms+ wasted on settings reads

    // Trading decision logic...
}
```

### After: Cache-Only Reads
```go
func (g *GinieAutopilot) GenerateDecision(symbol string) (*Decision, error) {
    mode := g.cache.GetCurrentMode(g.userID)            // Redis: <1ms
    config := g.cache.GetModeConfig(g.userID, mode)     // Redis: <1ms
    cb := g.cache.GetCircuitBreaker(g.userID)           // Redis: <1ms
    scalpConfig := g.cache.GetScalpReentry(g.userID)    // Redis: <1ms
    capitalAlloc := g.cache.GetCapitalAllocation(g.userID) // Redis: <1ms
    // TOTAL: <5ms for ALL settings reads (50x faster)

    // Trading decision logic...
}
```

### Affected Components

#### 1. GinieAutopilot Core
- `internal/autopilot/ginie_autopilot.go`
  - Inject CacheService instead of DB repository
  - Replace all `g.db.Get*()` with `g.cache.Get*()`
  - Remove direct DB dependencies

#### 2. FuturesController
- `internal/autopilot/futures_controller.go`
  - Read mode-specific settings from cache
  - Check circuit breaker from cache before executing trades
  - Load capital allocation percentages from cache

#### 3. Settings Manager
- `internal/autopilot/settings.go`
  - Refactor to use CacheService
  - Remove DB queries for current mode
  - Cache warm-up on Ginie startup

#### 4. Scalp Reentry Logic
- `internal/autopilot/scalp_reentry.go`
  - Read scalp reentry config from cache
  - Check reentry conditions using cached settings
  - No DB access during reentry decisions

### Cache Warm-Up on Startup
Ensure cache is populated before Ginie starts trading:
```go
func (g *GinieAutopilot) Start() error {
    // Warm up cache on startup
    if err := g.cache.LoadAllModes(g.userID); err != nil {
        return err
    }

    // Preload other configs
    g.cache.GetCircuitBreaker(g.userID)
    g.cache.GetScalpReentry(g.userID)
    g.cache.GetCapitalAllocation(g.userID)

    // Start trading cycles
    g.startTradingLoop()
    return nil
}
```

### Performance Monitoring
Add telemetry to measure cache performance:
```go
func (g *GinieAutopilot) GenerateDecision(symbol string) (*Decision, error) {
    start := time.Now()

    // Load all settings from cache
    settings := g.loadSettingsFromCache()

    settingsLoadTime := time.Since(start)
    if settingsLoadTime > 5*time.Millisecond {
        log.Warn("Settings load took %s (expected <5ms)", settingsLoadTime)
    }

    // Trading decision logic...
}
```

## Dependencies
- **Blocked By:** Stories 6.2, 6.3, 6.5, 6.7 (cache implementations and invalidation)
- **Blocks:** None (final optimization story)

## Files to Create/Modify
- `internal/autopilot/ginie_autopilot.go` - Replace DB calls with cache calls
- `internal/autopilot/futures_controller.go` - Use cache for mode settings
- `internal/autopilot/settings.go` - Refactor to use CacheService
- `internal/autopilot/scalp_reentry.go` - Use cached scalp config
- `internal/autopilot/capital_allocation.go` - Use cached capital percentages
- `internal/autopilot/circuit_breaker.go` - Use cached circuit breaker limits

## Testing Requirements

### Unit Tests
- Mock CacheService for all Ginie components
- Test GinieAutopilot.GenerateDecision() uses cache (not DB)
- Test FuturesController reads mode config from cache
- Test circuit breaker check reads from cache
- Test scalp reentry logic reads from cache
- Test cache warm-up on Ginie startup

### Integration Tests
- Test full Ginie cycle: Start → Load settings from cache → Generate decision → Execute
- Test cache miss handling: Empty cache → Ginie starts → Loads from DB → Populates cache → Continues with cache
- Test settings update during trading: Update mode config → Cache invalidated → Next Ginie cycle reads new config
- Test circuit breaker activation: Breached in cache → Ginie stops trading
- Test scalp reentry: Cached config enables reentry → Ginie executes reentry logic

### Performance Tests
- **Baseline**: Measure Ginie cycle time with DB queries (expect 250ms+ for settings)
- **Target**: Measure Ginie cycle time with cache (expect <5ms for settings)
- **Settings Load Time**: Measure time to load all settings from cache (<1ms per setting)
- **Full Cycle Time**: Measure total Ginie decision cycle time (should be significantly faster)
- **Load Test**: Run Ginie for 1000 cycles, verify consistent <5ms settings load time

### Regression Tests
- Verify trading decisions are identical with cache vs DB (same config = same decision)
- Test edge cases: Empty cache, Redis unavailable (should fall back to DB)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Zero DB queries during Ginie cycles verified
- [ ] Settings load time <1ms per setting verified
- [ ] Total settings load time <5ms verified
- [ ] Cache warm-up on startup tested
- [ ] Trading decisions consistent with DB-based approach
- [ ] Performance benchmarks documented (before/after)
- [ ] Telemetry added for monitoring
- [ ] Documentation updated
- [ ] PO acceptance received
