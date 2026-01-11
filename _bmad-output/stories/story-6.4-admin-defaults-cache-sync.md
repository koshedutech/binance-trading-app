# Story 6.4: Admin Defaults Cache with Sync Invalidation
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 3
**Priority:** P1

## User Story
As an admin, I want default settings cached with sync invalidation so that settings comparisons and new user creation are instant and always use the latest defaults.

## Acceptance Criteria
- [ ] Cache key: `admin:defaults:all`
- [ ] Full defaults JSON from default-settings.json
- [ ] Hash key: `admin:defaults:hash` for change detection
- [ ] Admin Sync (Epic 4.15) triggers cache invalidation
- [ ] New user creation copies from admin defaults cache
- [ ] Settings comparison uses cached defaults (not file read)

## Technical Approach

### Cache Key Schema
- `admin:defaults:all` - Full default settings JSON
- `admin:defaults:hash` - MD5 hash of defaults for change detection
- No expiration (TTL = 0)

### Change Detection Strategy
```go
func (c *CacheService) GetAdminDefaults() (*DefaultSettings, error) {
    key := "admin:defaults:all"
    hashKey := "admin:defaults:hash"

    // Load file and calculate hash
    defaults := loadDefaultSettingsJSON()
    currentHash := md5Hash(defaults.ToJSON())

    // Check cached hash
    cachedHash, err := c.redis.Get(ctx, hashKey).Result()
    if err == nil && cachedHash == currentHash {
        // Hash matches, return cached defaults
        cached, _ := c.redis.Get(ctx, key).Result()
        return parseDefaults(cached), nil
    }

    // Hash mismatch or cache miss - update cache
    c.redis.Set(ctx, key, defaults.ToJSON(), 0)
    c.redis.Set(ctx, hashKey, currentHash, 0)
    return defaults, nil
}
```

### Admin Sync Integration (Epic 4.15)
When admin updates default-settings.json:
```go
func (c *CacheService) InvalidateAdminDefaults() error {
    return c.redis.Del(ctx, "admin:defaults:all", "admin:defaults:hash").Err()
}
```

### Settings Comparison Integration (Epic 4.16)
Compare user settings to defaults using cache:
```go
func CompareToDefaults(userID string) (*Diff, error) {
    userSettings := cache.GetUserSettings(userID)      // From cache
    adminDefaults := cache.GetAdminDefaults()          // From cache
    return calculateDiff(userSettings, adminDefaults), nil
}
```

### New User Creation
When creating new user, copy from cached defaults:
```go
func CreateNewUser(email string) error {
    defaults := cache.GetAdminDefaults() // From cache, not file
    // Copy defaults to new user's settings
    return db.CreateUserWithSettings(email, defaults)
}
```

## Dependencies
- **Blocked By:** Story 6.1 (Redis Infrastructure Setup)
- **Blocks:** None (parallel with other caching stories)

## Files to Create/Modify
- `internal/cache/defaults_cache.go` - Admin defaults caching logic
- `internal/cache/keys.go` - Add admin defaults key constants
- `internal/api/handlers_admin.go` - Invalidate cache on admin sync
- `internal/auth/user_creation.go` - Use cached defaults for new users
- `internal/api/handlers_settings.go` - Use cached defaults for comparison

## Testing Requirements

### Unit Tests
- Test admin defaults cache key generation
- Test MD5 hash calculation for defaults
- Test change detection logic (hash match vs mismatch)
- Test cache invalidation on admin sync
- Test defaults loading from file on cache miss

### Integration Tests
- Test admin sync workflow: Update default-settings.json → Trigger sync → Cache invalidated → Next read loads new defaults
- Test new user creation: Create user → Verify settings match cached defaults
- Test settings comparison: Compare user settings to cached defaults
- Test hash-based change detection: Modify file → Hash changes → Cache refreshed
- Test file unchanged: Read twice → Second read uses cached hash

### Performance Tests
- Measure admin defaults read latency from cache (<1ms)
- Test new user creation time with cached defaults (<10ms)
- Test settings comparison time with cached defaults (<5ms)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Admin sync invalidates cache verified
- [ ] New user creation uses cache verified
- [ ] Settings comparison uses cache verified
- [ ] Hash-based change detection tested
- [ ] Documentation updated
- [ ] PO acceptance received
