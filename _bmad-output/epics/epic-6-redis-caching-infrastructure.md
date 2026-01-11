# Epic 6: Redis Caching Infrastructure

## Epic Overview

**Goal:** Implement high-performance Redis caching layer to eliminate database round-trips for settings and runtime state, correlating with Epic 4 (Default Settings System) and Epic 5 (Database Wiring) to provide instant access to user configurations for all trading decisions.

**Business Value:** Dramatically reduce latency for Ginie Autopilot decision-making, ensure settings persistence across container restarts, and provide foundation for order sequence tracking and Binance API response caching.

**Priority:** HIGH - Infrastructure foundation for Epic 7 & 8

**Estimated Complexity:** MEDIUM-HIGH

**Correlates With:**
- Epic 4 (Stories 4.13-4.17): Default Settings Loading System
- Epic 5: Database Wiring & Stability
- Epic 7 (Story 7.2): Sequence counter for clientOrderId generation

---

## Problem Statement

### Current Issues

| Issue | Severity | Impact |
|-------|----------|--------|
| **Database round-trip for every settings read** | HIGH | 50-200ms latency per decision |
| **Mode config checked on every Ginie cycle** | HIGH | Unnecessary DB load |
| **Settings lost on container restart** | MEDIUM | Must re-query DB after restart |
| **No caching between Binance API calls** | MEDIUM | Rate limit pressure |
| **Order sequence requires persistent counter** | HIGH | Cannot implement Epic 7 without this |

### Current Architecture (Inefficient)

```
CURRENT STATE:
┌─────────────────────────────────────────────────────────────────┐
│ EVERY GINIE DECISION CYCLE (~5 seconds):                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Ginie Engine                                                   │
│      │                                                          │
│      ├──→ Query mode config         → PostgreSQL (50ms)         │
│      ├──→ Query user settings       → PostgreSQL (50ms)         │
│      ├──→ Query circuit breaker     → PostgreSQL (50ms)         │
│      ├──→ Query scalp reentry       → PostgreSQL (50ms)         │
│      └──→ Query capital allocation  → PostgreSQL (50ms)         │
│                                                                 │
│  TOTAL: 250ms+ wasted on settings reads PER CYCLE               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Target State

### Redis-First Architecture

```
TARGET STATE:
┌─────────────────────────────────────────────────────────────────┐
│ GINIE DECISION CYCLE WITH REDIS:                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Ginie Engine                                                   │
│      │                                                          │
│      └──→ All settings from Redis   → Redis (<1ms)              │
│                                                                 │
│  TOTAL: <1ms for ALL settings reads                             │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  SETTINGS UPDATE PATH (Write-Through):                          │
│                                                                 │
│  UI Change → API → PostgreSQL → Redis (invalidate/update)       │
│                                                                 │
│  Ginie reads ONLY from Redis (never DB during trading)          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Key Principles

1. **CACHE-FIRST READS** - All settings reads hit Redis first, DB only on cache miss
2. **WRITE-THROUGH UPDATES** - Settings changes go to DB then immediately update Redis
3. **PERSISTENT CACHE** - Redis AOF ensures data survives container restarts
4. **NO TTL FOR SETTINGS** - User settings never expire, only invalidate on update
5. **CORRELATE WITH EPIC 4** - Load Defaults (4.14) populates Redis, Admin Sync (4.15) invalidates cache

---

## Correlation with Epic 4 & Epic 5

### Epic 4 Correlation (Stories 4.13-4.17)

| Epic 4 Story | How Redis Integrates |
|--------------|---------------------|
| **4.13** Default Settings JSON | JSON loaded to DB → then to Redis on first read |
| **4.14** New User & Load Defaults | On "Load Defaults": update DB → update Redis cache |
| **4.15** Admin Settings Sync | Admin change triggers: JSON → DB → **invalidate Redis** |
| **4.16** Settings Comparison | Compare user's Redis cache vs admin defaults cache |
| **4.17** Comprehensive Reset | Reset action: update DB → **update Redis immediately** |

### Epic 5 Correlation

| Epic 5 Component | How Redis Integrates |
|------------------|---------------------|
| **5.1** Ginie Panel Loading | Panel loads settings from Redis → instant load |
| **5.2** Scalp Reentry Config | Scalp reentry in Redis → Ginie reads instantly |
| **5.3** Global Circuit Breaker | CB config in Redis → checked every cycle |

### Epic 7 Correlation

| Epic 7 Story | How Redis Integrates |
|--------------|---------------------|
| **7.2** ClientOrderId Generation | Uses Redis sequence counter: `user:{user_id}:sequence:{YYYYMMDD}` |
| **7.2** Daily Reset | Sequence counter key changes daily (YYYYMMDD format) |
| **7.2** Atomic Increment | Redis INCR ensures unique sequence numbers per user per day |

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-1 | Redis infrastructure with AOF persistence | 6.1 |
| FR-2 | User settings cached with write-through pattern | 6.2 |
| FR-3 | All 4 mode configs cached (Ultra/Scalp/Swing/Position) | 6.3 |
| FR-4 | Admin defaults cached with sync invalidation | 6.4 |
| FR-5 | Scalp reentry & circuit breaker cached | 6.5 |
| FR-6 | All settings APIs use cache-first pattern | 6.6 |
| FR-7 | Cache invalidated on any settings update | 6.7 |
| FR-8 | Ginie engine reads exclusively from cache | 6.8 |
| FR-9 | Graceful degradation if Redis unavailable | 6.9 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-1 | Settings read latency < 1ms (vs 50ms DB) | All |
| NFR-2 | Cache survives container restart (AOF persistence) | 6.1 |
| NFR-3 | Cache miss auto-populates from DB | 6.2 |
| NFR-4 | Zero cache staleness (immediate invalidation) | 6.7 |
| NFR-5 | Redis connection pool with health checks | 6.1 |
| NFR-6 | Graceful fallback to DB-only if Redis fails | 6.9 |
| NFR-7 | Circuit breaker for Redis health monitoring | 6.9 |

---

## Stories

### Story 6.1: Redis Infrastructure Setup

**Goal:** Deploy Redis with persistence and integrate with trading bot infrastructure.

**Acceptance Criteria:**
- [ ] Redis 7 Alpine container in docker-compose.yml
- [ ] AOF (Append Only File) persistence enabled
- [ ] Named volume for Redis data (redis_data)
- [ ] Health check endpoint
- [ ] Go Redis client (go-redis/redis) integrated
- [ ] Connection pool with configurable size
- [ ] Graceful reconnection on connection loss
- [ ] Environment variables for Redis config (host, port, password)

**Technical Notes:**
```yaml
# docker-compose.yml addition
services:
  redis:
    image: redis:7-alpine
    container_name: binance-bot-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --appendfsync everysec --maxmemory 512mb --maxmemory-policy noeviction
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 3

volumes:
  redis_data:
```

---

### Story 6.2: User Settings Cache with Write-Through Pattern

**Goal:** Cache all user settings with automatic DB sync on updates.

**Acceptance Criteria:**
- [ ] Cache key: `user:{user_id}:settings:all`
- [ ] Full user settings JSON stored in Redis
- [ ] Read path: Redis first → DB fallback → populate cache
- [ ] Write path: API → DB → Redis update (write-through)
- [ ] Cache populated on first access after container start
- [ ] No TTL (settings persist until updated)

**Correlates With:** Epic 4.14 (Load Defaults), Epic 4.16 (Settings Comparison)

**Technical Notes:**
```go
// Read pattern
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

// Write pattern
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

---

### Story 6.3: Mode Configuration Cache (All 4 Modes)

**Goal:** Cache individual mode configs for instant access during trading decisions.

**Acceptance Criteria:**
- [ ] Cache keys:
  - `user:{user_id}:mode:ultra`
  - `user:{user_id}:mode:scalp`
  - `user:{user_id}:mode:swing`
  - `user:{user_id}:mode:position`
- [ ] Mode config JSON with all mode-specific settings
- [ ] Individual mode update invalidates only that mode's cache
- [ ] Bulk load all modes on first Ginie startup
- [ ] Include: SL/TP settings, confidence thresholds, circuit breaker per mode

**Correlates With:** Epic 4.13 (Default Settings), Epic 5 (Mode Config DB Wiring)

---

### Story 6.4: Admin Defaults Cache with Sync Invalidation

**Goal:** Cache admin defaults template for fast comparison and new user creation.

**Acceptance Criteria:**
- [ ] Cache key: `admin:defaults:all`
- [ ] Full defaults JSON from default-settings.json
- [ ] Hash key: `admin:defaults:hash` for change detection
- [ ] Admin Sync (Epic 4.15) triggers cache invalidation
- [ ] New user creation copies from admin defaults cache
- [ ] Settings comparison uses cached defaults (not file read)

**Correlates With:** Epic 4.15 (Admin Settings Sync)

**Technical Notes:**
```go
// When admin updates settings (correlates with 4.15)
func (c *CacheService) InvalidateAdminDefaults() error {
    return c.redis.Del(ctx, "admin:defaults:all", "admin:defaults:hash").Err()
}

// When comparing user settings to defaults (correlates with 4.16)
func (c *CacheService) GetAdminDefaults() (*DefaultSettings, error) {
    key := "admin:defaults:all"
    if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
        return parseDefaults(cached), nil
    }

    // Load from file, cache it
    defaults := loadDefaultSettingsJSON()
    c.redis.Set(ctx, key, defaults.ToJSON(), 0)
    return defaults, nil
}
```

---

### Story 6.5: Scalp Reentry & Circuit Breaker Cache

**Goal:** Cache Epic 5 database-wired configs for instant access.

**Acceptance Criteria:**
- [ ] Cache key: `user:{user_id}:scalp_reentry`
- [ ] Cache key: `user:{user_id}:circuit_breaker`
- [ ] All 36+ scalp reentry settings cached
- [ ] Global circuit breaker limits cached
- [ ] Update via API triggers immediate cache update
- [ ] Ginie checks circuit breaker from cache (not DB)

**Correlates With:** Epic 5.2 (Scalp Reentry DB), Epic 5.3 (Circuit Breaker DB)

---

### Story 6.6: Cache-First Read Pattern for All Settings APIs

**Goal:** Refactor all settings API handlers to use cache-first pattern.

**Acceptance Criteria:**
- [ ] `GET /api/user/settings` reads from cache
- [ ] `GET /api/futures/ginie/modes/:mode` reads from cache
- [ ] `GET /api/futures/ginie/scalp-reentry-config` reads from cache
- [ ] `GET /api/user/global-circuit-breaker` reads from cache
- [ ] `GET /api/futures/ginie/capital-allocation` reads from cache
- [ ] All GET handlers: Redis first → DB fallback → populate cache
- [ ] API response time reduced from 50-200ms to <5ms

**Affected Files:**
- internal/api/handlers_settings.go
- internal/api/handlers_ginie.go
- internal/api/handlers_modes.go

---

### Story 6.7: Cache Invalidation on Settings Update

**Goal:** Ensure zero cache staleness with immediate invalidation on any update.

**Acceptance Criteria:**
- [ ] All PUT/POST settings endpoints trigger cache update
- [ ] Invalidation is synchronous (before API response)
- [ ] Pattern: DB write success → cache update → return success
- [ ] **Cache-DB Consistency**: If Redis update fails after DB write, DELETE the stale cache key
- [ ] This forces cache miss on next read, ensuring fresh data from DB
- [ ] Reset to Defaults (Epic 4.17) triggers full cache refresh
- [ ] Admin Sync (Epic 4.15) invalidates admin defaults cache

**Correlates With:** Epic 4.17 (Comprehensive Reset), Epic 4.15 (Admin Sync)

**Technical Notes:**
```go
// Write-through with consistency handling
func (c *CacheService) UpdateUserSettings(userID string, settings *Settings) error {
    // Write to DB first
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

**Invalidation Matrix:**

| API Endpoint | Cache Keys Invalidated |
|--------------|----------------------|
| `PUT /api/user/settings` | `user:{id}:settings:all` |
| `PUT /api/futures/ginie/modes/:mode` | `user:{id}:mode:{mode}` |
| `POST /api/user/settings/load-defaults` | All user cache keys |
| `POST /api/admin/sync-defaults` | `admin:defaults:*` |
| `PUT /api/futures/ginie/scalp-reentry-config` | `user:{id}:scalp_reentry` |
| `PUT /api/user/global-circuit-breaker` | `user:{id}:circuit_breaker` |

---

### Story 6.8: Ginie Engine Reads from Cache Only

**Goal:** Modify Ginie Autopilot to exclusively read settings from Redis cache.

**Acceptance Criteria:**
- [ ] `GinieAutopilot.GenerateDecision()` reads mode config from cache
- [ ] `FuturesController` reads mode settings from cache
- [ ] Circuit breaker checks use cached limits
- [ ] Scalp reentry logic uses cached config
- [ ] Capital allocation uses cached percentages
- [ ] Zero DB queries during active trading cycles
- [ ] Performance: Settings access <1ms (measured)

**Affected Files:**
- internal/autopilot/ginie_autopilot.go
- internal/autopilot/futures_controller.go
- internal/autopilot/settings.go
- internal/autopilot/scalp_reentry.go

**Technical Notes:**
```go
// Before (DB query every cycle)
func (g *GinieAutopilot) GenerateDecision(symbol string) (*Decision, error) {
    mode := g.settingsManager.GetCurrentMode()  // DB query
    config := g.db.GetModeConfig(g.userID, mode) // DB query
    // ...
}

// After (Cache only)
func (g *GinieAutopilot) GenerateDecision(symbol string) (*Decision, error) {
    mode := g.cache.GetCurrentMode(g.userID)     // Redis <1ms
    config := g.cache.GetModeConfig(g.userID, mode) // Redis <1ms
    // ...
}
```

---

### Story 6.9: Cache Fallback & Graceful Degradation

**Goal:** Ensure the trading bot operates even if Redis is unavailable.

**Acceptance Criteria:**
- [ ] **Read Path**: If Redis unavailable, fall back to DB read
- [ ] **Write Path**: If Redis unavailable during write, write to DB and skip cache update (log warning)
- [ ] Health check endpoint for Redis connection status
- [ ] Circuit breaker pattern for Redis operations
- [ ] If 3+ consecutive Redis failures, enter degraded mode (DB-only)
- [ ] Auto-recover when Redis becomes available again
- [ ] Log all Redis failures with appropriate severity
- [ ] Trading operations NEVER fail due to Redis issues

**Technical Notes:**
```go
// Read with fallback
func (c *CacheService) GetUserSettings(userID string) (*Settings, error) {
    key := fmt.Sprintf("user:%s:settings:all", userID)

    // Try cache first
    if c.redisHealthy {
        if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
            return parseSettings(cached), nil
        }

        // Check if Redis is down
        if err == redis.Nil {
            // Cache miss - expected, continue to DB
        } else {
            // Redis connection error
            c.recordRedisFailure()
            log.Warn("Redis unavailable for read, falling back to DB")
        }
    }

    // Fallback to DB
    settings, err := c.db.GetUserSettings(userID)
    if err != nil {
        return nil, err
    }

    // Try to populate cache (if Redis healthy)
    if c.redisHealthy {
        if err := c.redis.Set(ctx, key, settings.ToJSON(), 0).Err(); err != nil {
            c.recordRedisFailure()
        }
    }

    return settings, nil
}

// Circuit breaker
func (c *CacheService) recordRedisFailure() {
    c.redisFailureCount++
    if c.redisFailureCount >= 3 {
        c.redisHealthy = false
        log.Error("Redis circuit breaker OPEN - entering degraded mode (DB-only)")
    }
}

func (c *CacheService) healthCheck() {
    if err := c.redis.Ping(ctx).Err(); err == nil {
        if !c.redisHealthy {
            log.Info("Redis connection restored - circuit breaker CLOSED")
        }
        c.redisHealthy = true
        c.redisFailureCount = 0
    }
}
```

**Fallback Strategy:**
- Redis healthy: Cache-first reads, write-through updates
- Redis degraded: DB-only mode, log warnings
- Redis recovered: Resume cache operations, repopulate cache

---

## Redis Key Schema

### Complete Key Naming Convention

```
# User Settings (correlates Epic 4)
user:{user_id}:settings:all              → Full user settings JSON
user:{user_id}:settings:trading          → Trading-specific settings
user:{user_id}:settings:notifications    → Notification preferences

# Mode Configurations (correlates Epic 4 & 5)
user:{user_id}:mode:ultra                → Ultra mode config
user:{user_id}:mode:scalp                → Scalp mode config
user:{user_id}:mode:swing                → Swing mode config
user:{user_id}:mode:position             → Position mode config

# Epic 5 Configs
user:{user_id}:scalp_reentry             → Scalp reentry config
user:{user_id}:circuit_breaker           → Global circuit breaker

# Admin Defaults (correlates Epic 4.15)
admin:defaults:all                       → Master defaults JSON
admin:defaults:hash                      → MD5 hash for change detection

# Runtime State (for Epic 7)
user:{user_id}:sequence:{YYYYMMDD}       → Daily order sequence counter

# Future: Binance API Cache (Epic 8 prep)
binance:{user_id}:positions              → Position cache (TTL: 5s)
binance:{user_id}:orders                 → Open orders cache (TTL: 5s)
```

---

## Dependencies

| Dependency | Type | Status |
|------------|------|--------|
| Epic 4 - Default Settings System | Prerequisite | Complete |
| Epic 5 - Database Wiring | Prerequisite | In Progress |
| PostgreSQL database | Infrastructure | Exists |
| Docker Compose | Infrastructure | Exists |
| go-redis/redis v9 | Library | To Add |

---

## Success Criteria

1. **Redis Running**: Container healthy, AOF persistence working
2. **Settings Cached**: All user settings accessible via Redis
3. **Write-Through Working**: Any settings change reflects in cache immediately
4. **Ginie Performance**: Settings access <1ms (vs 50ms+ previously)
5. **Persistence**: Cache survives container restart
6. **Epic 4 Integration**: Load Defaults updates cache, Admin Sync invalidates cache
7. **Epic 5 Integration**: Scalp reentry and circuit breaker cached
8. **Zero Staleness**: No scenario where cache has outdated settings

---

## Technical Considerations

### Redis Fallback Strategy

**Goal:** Ensure trading operations continue even if Redis is unavailable.

**Strategy:**
1. **Health Monitoring**: Continuous Redis health checks via `PING` command
2. **Circuit Breaker**: After 3 consecutive failures, enter degraded mode (DB-only)
3. **Graceful Degradation**:
   - Read operations fall back to DB
   - Write operations skip cache update (log warning)
   - No trading operation fails due to Redis unavailability
4. **Auto-Recovery**: When Redis becomes healthy, automatically resume caching

**Implementation:**
```go
type CacheService struct {
    redis            *redis.Client
    db               *database.Repository
    redisHealthy     bool
    redisFailureCount int
    circuitBreakerOpen bool
}

// Read with fallback
func (c *CacheService) GetUserSettings(userID string) (*Settings, error) {
    // Try Redis first (if healthy)
    if c.redisHealthy {
        if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
            return parseSettings(cached), nil
        }
        // Redis error - fall back to DB
        if err != redis.Nil {
            c.recordRedisFailure()
            log.Warn("Redis unavailable, falling back to DB")
        }
    }

    // Always have DB fallback
    return c.db.GetUserSettings(userID)
}

// Write with graceful degradation
func (c *CacheService) UpdateUserSettings(userID string, settings *Settings) error {
    // Write to DB first (ALWAYS)
    if err := c.db.UpdateUserSettings(userID, settings); err != nil {
        return err
    }

    // Try to update cache (if healthy)
    if c.redisHealthy {
        key := fmt.Sprintf("user:%s:settings:all", userID)
        if err := c.redis.Set(ctx, key, settings.ToJSON(), 0).Err(); err != nil {
            // Delete stale key to force cache miss
            c.redis.Del(ctx, key)
            c.recordRedisFailure()
            log.Warn("Redis update failed, deleted stale cache key")
        }
    }

    return nil // DB write succeeded, that's what matters
}
```

**Why This Matters:**
- Trading operations NEVER fail due to Redis issues
- Graceful performance degradation (slower, but still functional)
- Automatic recovery when Redis becomes available

### Infrastructure Changes

```yaml
# docker-compose.yml additions
services:
  redis:
    image: redis:7-alpine
    container_name: binance-bot-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes --appendfsync everysec --maxmemory 512mb --maxmemory-policy noeviction
    restart: unless-stopped
    networks:
      - trading-network

  trading-bot:
    depends_on:
      - postgres
      - redis  # Add dependency
    environment:
      - REDIS_HOST=redis
      - REDIS_PORT=6379

volumes:
  redis_data:
```

### New Files

```
internal/cache/
├── redis.go              # Redis client, connection pool
├── settings_cache.go     # User settings caching
├── mode_cache.go         # Mode configuration cache
├── defaults_cache.go     # Admin defaults cache
├── invalidation.go       # Cache invalidation logic
└── keys.go               # Key naming conventions
```

### Modified Files

| File | Changes |
|------|---------|
| docker-compose.yml | Add Redis service |
| docker-compose.prod.yml | Add Redis service |
| main.go | Initialize Redis client |
| internal/autopilot/ginie_autopilot.go | Use cache service |
| internal/autopilot/futures_controller.go | Use cache service |
| internal/autopilot/settings.go | Add cache layer |
| internal/api/handlers_settings.go | Cache-first reads |
| internal/api/handlers_ginie.go | Cache-first reads |
| go.mod | Add go-redis/redis v9 |

---

## Testing Strategy

### Unit Tests
- Cache key generation
- JSON serialization/deserialization
- Invalidation logic

### Integration Tests
- Write-through pattern (DB + cache consistency)
- Cache miss → DB load → cache populate
- Container restart → cache recovery

### Performance Tests
- Settings read latency (<1ms target)
- Ginie cycle time improvement measurement
- Load test: 1000 settings reads/second

### Edge Cases
- **Redis connection failure**: Gracefully degrade to DB-only mode (Story 6.9)
- **Redis unavailable during read**: Fall back to DB, log warning
- **Redis unavailable during write**: Write to DB, skip cache, log warning
- **Redis recovery**: Auto-resume caching when connection restored
- **Circuit breaker activation**: After 3 failures, enter DB-only mode
- Concurrent writes to same key
- Large settings JSON (>1MB)
- Redis memory pressure (maxmemory policy: noeviction)

---

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| Redis connection failure | HIGH | Story 6.9: Circuit breaker + graceful degradation to DB-only mode |
| Cache-DB inconsistency | HIGH | Story 6.7: Write-through with DELETE on cache update failure |
| Memory pressure | MEDIUM | maxmemory 512mb + noeviction policy + monitoring |
| Cold start latency | LOW | Lazy loading, warm-up on startup |
| Stale cache after failed write | HIGH | Story 6.7: DELETE stale key to force cache miss |
| Redis unavailable blocks trading | CRITICAL | Story 6.9: All operations fall back to DB, never fail |

---

## Author

**Created By:** BMAD Party Mode (Architect: Winston, Analyst: Mary, PM: John)
**Date:** 2026-01-06
**Version:** 1.0
**Correlates With:** Epic 4 (4.13-4.17), Epic 5
