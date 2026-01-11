# Story 6.2: User Settings Cache with Write-Through Pattern
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 5
**Priority:** P0

## User Story
As a developer, I want user settings cached with write-through pattern so that settings reads are sub-millisecond and always consistent with the database.

## Acceptance Criteria
- [ ] Cache key: `user:{user_id}:settings:all`
- [ ] Full user settings JSON stored in Redis
- [ ] Read path: Redis first → DB fallback → populate cache
- [ ] Write path: API → DB → Redis update (write-through)
- [ ] Cache populated on first access after container start
- [ ] No TTL (settings persist until updated)

## Technical Approach

### Cache Key Schema
- Key format: `user:{user_id}:settings:all`
- Value: JSON serialized user settings
- No expiration (TTL = 0)

### Read Pattern Implementation
```go
func (c *CacheService) GetUserSettings(userID string) (*Settings, error) {
    key := fmt.Sprintf("user:%s:settings:all", userID)

    // Try cache first
    if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
        return parseSettings(cached), nil
    }

    // Cache miss - load from DB
    settings, err := c.db.GetUserSettings(userID)
    if err != nil {
        return nil, err
    }

    // Populate cache (no TTL)
    c.redis.Set(ctx, key, settings.ToJSON(), 0)
    return settings, nil
}
```

### Write Pattern Implementation
```go
func (c *CacheService) UpdateUserSettings(userID string, settings *Settings) error {
    // Write to DB first
    if err := c.db.UpdateUserSettings(userID, settings); err != nil {
        return err
    }

    // Update cache immediately
    key := fmt.Sprintf("user:%s:settings:all", userID)
    return c.redis.Set(ctx, key, settings.ToJSON(), 0).Err()
}
```

### Integration Points
- Correlates with Epic 4.14 (Load Defaults): When user loads defaults, update DB then update cache
- Correlates with Epic 4.16 (Settings Comparison): Compare user's cached settings vs admin defaults cache
- All settings API endpoints use CacheService instead of direct DB access

## Dependencies
- **Blocked By:** Story 6.1 (Redis Infrastructure Setup)
- **Blocks:** Story 6.6 (Cache-First Read Pattern for All Settings APIs)

## Files to Create/Modify
- `internal/cache/settings_cache.go` - User settings caching logic
- `internal/cache/keys.go` - Key naming convention helpers
- `internal/cache/service.go` - Main CacheService struct
- `internal/api/handlers_settings.go` - Refactor to use CacheService
- `internal/database/user_settings.go` - Ensure proper JSON serialization

## Testing Requirements

### Unit Tests
- Test cache key generation for different user IDs
- Test JSON serialization/deserialization of settings
- Test read path: cache hit returns cached data
- Test read path: cache miss loads from DB and populates cache
- Test write path: DB write then cache update
- Test settings persistence (no TTL)

### Integration Tests
- Test full read-write cycle: Write settings → Read settings (from cache)
- Test cache miss handling: Delete cache key → Read settings (loads from DB)
- Test cache population: First read after restart loads from DB
- Test cache consistency: Update settings → Immediately read → Verify updated
- Test multiple users: Ensure user1 cache doesn't affect user2

### Performance Tests
- Measure read latency from cache (<1ms target)
- Measure read latency from DB fallback (<50ms acceptable)
- Test 1000 concurrent reads from cache
- Verify cache hit ratio >95% after warm-up

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Cache read latency <1ms verified
- [ ] Write-through pattern tested and verified
- [ ] Cache persistence across container restart tested
- [ ] Documentation updated
- [ ] PO acceptance received
