# Story 6.7: Cache Invalidation on Settings Update
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 5
**Priority:** P0

## User Story
As a developer, I want cache automatically invalidated on any settings update so that there is zero cache staleness and users always see the latest configuration.

## Acceptance Criteria
- [ ] All PUT/POST settings endpoints trigger cache update
- [ ] Invalidation is synchronous (before API response)
- [ ] Pattern: DB write success → cache update → return success
- [ ] **Cache-DB Consistency**: If Redis update fails after DB write, DELETE the stale cache key
- [ ] This forces cache miss on next read, ensuring fresh data from DB
- [ ] Reset to Defaults (Epic 4.17) triggers full cache refresh
- [ ] Admin Sync (Epic 4.15) invalidates admin defaults cache

## Technical Approach

### Write-Through with Consistency Handling
```go
func (c *CacheService) UpdateUserSettings(userID string, settings *Settings) error {
    // Write to DB first (ALWAYS)
    if err := c.db.UpdateUserSettings(userID, settings); err != nil {
        return err
    }

    // Try to update cache
    key := fmt.Sprintf("user:%s:settings:all", userID)
    if err := c.redis.Set(ctx, key, settings.ToJSON(), 0).Err(); err != nil {
        // If cache update fails, DELETE the key to force cache miss
        c.redis.Del(ctx, key)
        log.Warn("Redis update failed after DB write, deleted stale cache key: %s", key)
    }

    return nil
}
```

### Invalidation Matrix

| API Endpoint | Cache Keys Invalidated | Invalidation Method |
|--------------|----------------------|---------------------|
| `PUT /api/user/settings` | `user:{id}:settings:all` | Write-through update |
| `PUT /api/futures/ginie/modes/ultra` | `user:{id}:mode:ultra` | Write-through update |
| `PUT /api/futures/ginie/modes/scalp` | `user:{id}:mode:scalp` | Write-through update |
| `PUT /api/futures/ginie/modes/swing` | `user:{id}:mode:swing` | Write-through update |
| `PUT /api/futures/ginie/modes/position` | `user:{id}:mode:position` | Write-through update |
| `POST /api/user/settings/load-defaults` | All user cache keys | Bulk update (all keys) |
| `POST /api/admin/sync-defaults` | `admin:defaults:*` | DELETE keys |
| `PUT /api/futures/ginie/scalp-reentry-config` | `user:{id}:scalp_reentry` | Write-through update |
| `PUT /api/user/global-circuit-breaker` | `user:{id}:circuit_breaker` | Write-through update |
| `PUT /api/futures/ginie/capital-allocation` | `user:{id}:settings:all` | Write-through update |

### Epic 4.17 Integration (Reset to Defaults)
When user resets all settings to defaults:
```go
func ResetToDefaults(userID string) error {
    // Load defaults
    defaults := cache.GetAdminDefaults()

    // Write to DB
    if err := db.ResetUserSettings(userID, defaults); err != nil {
        return err
    }

    // Invalidate ALL user cache keys
    keys := []string{
        fmt.Sprintf("user:%s:settings:all", userID),
        fmt.Sprintf("user:%s:mode:ultra", userID),
        fmt.Sprintf("user:%s:mode:scalp", userID),
        fmt.Sprintf("user:%s:mode:swing", userID),
        fmt.Sprintf("user:%s:mode:position", userID),
        fmt.Sprintf("user:%s:scalp_reentry", userID),
        fmt.Sprintf("user:%s:circuit_breaker", userID),
    }

    // Update all keys with new defaults
    for _, key := range keys {
        // Set new values, or DELETE if update fails
        if err := redis.Set(ctx, key, ...).Err(); err != nil {
            redis.Del(ctx, key)
        }
    }

    return nil
}
```

### Epic 4.15 Integration (Admin Sync)
When admin updates default-settings.json:
```go
func SyncAdminDefaults() error {
    // Delete admin defaults cache
    cache.InvalidateAdminDefaults() // Deletes admin:defaults:*

    // Optionally: Invalidate all user caches if defaults changed significantly
    // This ensures users get new defaults on next settings comparison

    return nil
}
```

### Consistency Guarantees
1. **DB is source of truth**: Always write to DB first
2. **Cache update failure handling**: DELETE stale key to force cache miss
3. **Synchronous invalidation**: Cache updated before API returns success
4. **Atomic operations**: Use Redis transactions where needed

## Dependencies
- **Blocked By:** Stories 6.2-6.6 (all cache implementations)
- **Blocks:** Story 6.8 (Ginie Engine Reads from Cache Only)

## Files to Create/Modify
- `internal/cache/invalidation.go` - Cache invalidation logic
- `internal/cache/service.go` - Add consistency handling methods
- `internal/api/handlers_settings.go` - Add cache invalidation to PUT handlers
- `internal/api/handlers_ginie.go` - Add cache invalidation to PUT handlers
- `internal/api/handlers_modes.go` - Add cache invalidation to PUT handlers
- `internal/api/handlers_admin.go` - Add admin defaults invalidation

## Testing Requirements

### Unit Tests
- Test write-through pattern: DB write → Cache update
- Test cache update failure: DB write success → Cache fails → Key deleted
- Test individual mode invalidation (only one key updated)
- Test bulk invalidation (Reset to Defaults updates all keys)
- Test admin sync invalidation (admin:defaults:* deleted)

### Integration Tests
- Test settings update cycle: Update settings → Cache invalidated → Read settings → New data returned
- Test mode update: Update ultra mode → Only ultra cache updated → Other modes unchanged
- Test Reset to Defaults: Trigger reset → All user caches updated → Reads show defaults
- Test Admin Sync: Update default-settings.json → Sync → Admin cache invalidated → Next read loads new file
- Test consistency: Cache update fails → Key deleted → Next read loads from DB

### Concurrency Tests
- Test concurrent updates to same settings (ensure consistency)
- Test race condition: Update settings while Ginie reading (should get old or new, not corrupted)
- Test multiple users updating different settings simultaneously

### Performance Tests
- Measure invalidation overhead: Write to DB + update cache (<10ms total)
- Test bulk invalidation performance (Reset to Defaults updates all keys <20ms)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] All PUT/POST endpoints invalidate cache
- [ ] Cache-DB consistency guaranteed (DELETE on update failure)
- [ ] Reset to Defaults updates all caches
- [ ] Admin Sync invalidates admin defaults
- [ ] Zero cache staleness verified
- [ ] Concurrency tests passing
- [ ] Documentation updated
- [ ] PO acceptance received
