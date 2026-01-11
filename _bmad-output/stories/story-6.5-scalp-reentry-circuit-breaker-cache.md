# Story 6.5: Scalp Reentry & Circuit Breaker Cache
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 3
**Priority:** P1

## User Story
As the Ginie Autopilot engine, I want scalp reentry and circuit breaker configurations cached so that I can check these critical limits in <1ms during trading cycles.

## Acceptance Criteria
- [ ] Cache key: `user:{user_id}:scalp_reentry`
- [ ] Cache key: `user:{user_id}:circuit_breaker`
- [ ] All 36+ scalp reentry settings cached
- [ ] Global circuit breaker limits cached
- [ ] Update via API triggers immediate cache update
- [ ] Ginie checks circuit breaker from cache (not DB)

## Technical Approach

### Cache Key Schema
- `user:{user_id}:scalp_reentry` - All scalp reentry configuration (36+ settings)
- `user:{user_id}:circuit_breaker` - Global circuit breaker limits
- No expiration (TTL = 0)

### Scalp Reentry Cache Structure
Contains all settings from Epic 5.2:
- Enable scalp reentry (boolean)
- Number of scalp attempts
- Ultra DCA config (6 settings)
- Scalp DCA config (6 settings)
- Swing DCA config (6 settings)
- Position DCA config (6 settings)
- Quantity adjustment ratios
- Price adjustment offsets
- Reentry conditions and triggers

### Circuit Breaker Cache Structure
Contains settings from Epic 5.3:
- Max daily loss limit
- Max drawdown percentage
- Max concurrent positions
- Pause duration on breach
- Auto-resume settings
- Per-symbol limits
- Global kill switch

### Write-Through Pattern
```go
func (c *CacheService) UpdateScalpReentry(userID string, config *ScalpReentryConfig) error {
    // Write to DB first
    if err := c.db.UpdateScalpReentry(userID, config); err != nil {
        return err
    }

    // Update cache immediately
    key := fmt.Sprintf("user:%s:scalp_reentry", userID)
    return c.redis.Set(ctx, key, config.ToJSON(), 0).Err()
}

func (c *CacheService) UpdateCircuitBreaker(userID string, config *CircuitBreakerConfig) error {
    // Write to DB first
    if err := c.db.UpdateCircuitBreaker(userID, config); err != nil {
        return err
    }

    // Update cache immediately
    key := fmt.Sprintf("user:%s:circuit_breaker", userID)
    return c.redis.Set(ctx, key, config.ToJSON(), 0).Err()
}
```

### Ginie Integration
Ginie checks circuit breaker before every trading decision:
```go
func (g *GinieAutopilot) GenerateDecision(symbol string) (*Decision, error) {
    // Check circuit breaker from cache (not DB)
    cb := g.cache.GetCircuitBreaker(g.userID)
    if cb.IsBreached() {
        return nil, ErrCircuitBreakerActive
    }

    // Get scalp reentry config from cache
    scalpConfig := g.cache.GetScalpReentry(g.userID)
    // Use config in decision logic
}
```

## Dependencies
- **Blocked By:** Story 6.1 (Redis Infrastructure Setup)
- **Blocks:** Story 6.8 (Ginie Engine Reads from Cache Only)

## Files to Create/Modify
- `internal/cache/scalp_cache.go` - Scalp reentry caching logic
- `internal/cache/circuit_breaker_cache.go` - Circuit breaker caching logic
- `internal/autopilot/ginie_autopilot.go` - Read circuit breaker from cache
- `internal/autopilot/scalp_reentry.go` - Read scalp config from cache
- `internal/api/handlers_ginie.go` - Update cache on config changes

## Testing Requirements

### Unit Tests
- Test scalp reentry cache key generation
- Test circuit breaker cache key generation
- Test JSON serialization of scalp reentry config (36+ fields)
- Test JSON serialization of circuit breaker config
- Test write-through pattern for both configs
- Test cache read with hit and miss scenarios

### Integration Tests
- Test scalp reentry update workflow: API update → DB write → Cache update → Ginie reads new config
- Test circuit breaker update workflow: API update → DB write → Cache update → Ginie checks new limits
- Test circuit breaker breach: Ginie checks cache → Breached → Trading paused
- Test scalp reentry disabled: Update config → Ginie reads from cache → Scalp reentry skipped
- Test cache persistence: Restart container → Circuit breaker limits retained

### Performance Tests
- Measure circuit breaker check latency from cache (<1ms)
- Measure scalp reentry config read latency from cache (<1ms)
- Test 100 concurrent circuit breaker checks
- Verify Ginie cycle time improvement with cached configs

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Scalp reentry config cached and accessible
- [ ] Circuit breaker config cached and accessible
- [ ] Ginie reads from cache verified
- [ ] Cache latency <1ms verified
- [ ] Write-through pattern tested
- [ ] Documentation updated
- [ ] PO acceptance received
