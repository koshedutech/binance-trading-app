# Story 6.3: Mode Configuration Cache (All 4 Modes)
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 5
**Priority:** P0

## User Story
As the Ginie Autopilot engine, I want individual mode configurations cached so that I can access mode-specific settings in <1ms during trading decisions.

## Acceptance Criteria
- [ ] Cache keys for all 4 modes:
  - `user:{user_id}:mode:ultra`
  - `user:{user_id}:mode:scalp`
  - `user:{user_id}:mode:swing`
  - `user:{user_id}:mode:position`
- [ ] Mode config JSON with all mode-specific settings
- [ ] Individual mode update invalidates only that mode's cache
- [ ] Bulk load all modes on first Ginie startup
- [ ] Include: SL/TP settings, confidence thresholds, circuit breaker per mode

## Technical Approach

### Cache Key Schema
- Key format: `user:{user_id}:mode:{mode_name}`
- Mode names: ultra, scalp, swing, position
- Value: JSON serialized mode configuration
- No expiration (TTL = 0)

### Mode Configuration Structure
Each mode cache contains:
- SL/TP percentages
- Confidence threshold
- Circuit breaker limits (specific to mode)
- Risk management settings
- Position sizing rules
- Entry/exit criteria

### Granular Invalidation
When user updates a specific mode:
- Only invalidate that mode's cache key
- Do not invalidate other modes or full settings cache
- Allows independent mode configuration changes

### Bulk Loading Strategy
On Ginie startup or first access:
```go
func (c *CacheService) LoadAllModes(userID string) error {
    modes := []string{"ultra", "scalp", "swing", "position"}
    for _, mode := range modes {
        key := fmt.Sprintf("user:%s:mode:%s", userID, mode)

        // Check if already cached
        if exists, _ := c.redis.Exists(ctx, key).Result(); exists > 0 {
            continue
        }

        // Load from DB and cache
        config, err := c.db.GetModeConfig(userID, mode)
        if err != nil {
            return err
        }
        c.redis.Set(ctx, key, config.ToJSON(), 0)
    }
    return nil
}
```

### Integration Points
- Correlates with Epic 4.13 (Default Settings): Mode configs loaded from default-settings.json
- Correlates with Epic 5 (Mode Config DB Wiring): DB schema already exists for mode configs
- Ginie engine reads mode config from cache before each trading decision

## Dependencies
- **Blocked By:** Story 6.1 (Redis Infrastructure Setup)
- **Blocks:** Story 6.8 (Ginie Engine Reads from Cache Only)

## Files to Create/Modify
- `internal/cache/mode_cache.go` - Mode configuration caching logic
- `internal/cache/keys.go` - Add mode key generation functions
- `internal/autopilot/ginie_autopilot.go` - Use cached mode configs
- `internal/autopilot/futures_controller.go` - Use cached mode configs
- `internal/api/handlers_modes.go` - Refactor to use CacheService

## Testing Requirements

### Unit Tests
- Test mode cache key generation for all 4 modes
- Test JSON serialization/deserialization of mode configs
- Test individual mode invalidation (only one key deleted)
- Test bulk mode loading
- Test mode config read with cache hit
- Test mode config read with cache miss

### Integration Tests
- Test full mode update cycle: Update ultra mode → Cache invalidated → Read ultra mode (from DB) → Other modes still cached
- Test bulk load on first access: Fresh cache → Load all modes → All 4 modes cached
- Test mode-specific updates don't affect other modes
- Test Ginie engine reads correct mode config from cache

### Performance Tests
- Measure mode config read latency from cache (<1ms)
- Test 100 concurrent mode reads
- Test bulk load performance (all 4 modes in <10ms)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] All 4 mode configs cacheable
- [ ] Granular invalidation verified
- [ ] Bulk loading tested
- [ ] Mode read latency <1ms verified
- [ ] Documentation updated
- [ ] PO acceptance received
