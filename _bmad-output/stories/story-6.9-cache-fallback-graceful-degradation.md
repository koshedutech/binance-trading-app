# Story 6.9: Cache Fallback & Graceful Degradation
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 5
**Priority:** P0

## User Story
As a trading bot operator, I want the system to gracefully degrade to DB-only mode when Redis is unavailable so that trading operations never fail due to cache issues.

## Acceptance Criteria
- [ ] **Read Path**: If Redis unavailable, fall back to DB read
- [ ] **Write Path**: If Redis unavailable during write, write to DB and skip cache update (log warning)
- [ ] Health check endpoint for Redis connection status
- [ ] Circuit breaker pattern for Redis operations
- [ ] If 3+ consecutive Redis failures, enter degraded mode (DB-only)
- [ ] Auto-recover when Redis becomes available again
- [ ] Log all Redis failures with appropriate severity
- [ ] Trading operations NEVER fail due to Redis issues

## Technical Approach

### Circuit Breaker Implementation
```go
type CacheService struct {
    redis              *redis.Client
    db                 *database.Repository
    redisHealthy       bool
    redisFailureCount  int
    circuitBreakerOpen bool
    mu                 sync.RWMutex
}

func (c *CacheService) recordRedisFailure() {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.redisFailureCount++
    if c.redisFailureCount >= 3 {
        c.redisHealthy = false
        c.circuitBreakerOpen = true
        log.Error("Redis circuit breaker OPEN - entering degraded mode (DB-only)")
    }
}

func (c *CacheService) healthCheck() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        if err := c.redis.Ping(ctx).Err(); err == nil {
            c.mu.Lock()
            if !c.redisHealthy {
                log.Info("Redis connection restored - circuit breaker CLOSED")
            }
            c.redisHealthy = true
            c.redisFailureCount = 0
            c.circuitBreakerOpen = false
            c.mu.Unlock()
        } else {
            c.recordRedisFailure()
        }
    }
}
```

### Read with Fallback
```go
func (c *CacheService) GetUserSettings(userID string) (*Settings, error) {
    key := fmt.Sprintf("user:%s:settings:all", userID)

    // Try cache first (if healthy)
    c.mu.RLock()
    healthy := c.redisHealthy
    c.mu.RUnlock()

    if healthy {
        if cached, err := c.redis.Get(ctx, key).Result(); err == nil {
            return parseSettings(cached), nil
        }

        // Check if Redis is down vs cache miss
        if err == redis.Nil {
            // Cache miss - expected, continue to DB
        } else {
            // Redis connection error
            c.recordRedisFailure()
            log.Warn("Redis unavailable for read, falling back to DB")
        }
    }

    // Fallback to DB (ALWAYS works)
    settings, err := c.db.GetUserSettings(userID)
    if err != nil {
        return nil, err
    }

    // Try to populate cache (if Redis healthy)
    c.mu.RLock()
    healthy = c.redisHealthy
    c.mu.RUnlock()

    if healthy {
        if err := c.redis.Set(ctx, key, settings.ToJSON(), 0).Err(); err != nil {
            c.recordRedisFailure()
        }
    }

    return settings, nil
}
```

### Write with Graceful Degradation
```go
func (c *CacheService) UpdateUserSettings(userID string, settings *Settings) error {
    // Write to DB first (ALWAYS - source of truth)
    if err := c.db.UpdateUserSettings(userID, settings); err != nil {
        return err
    }

    // Try to update cache (if healthy)
    c.mu.RLock()
    healthy := c.redisHealthy
    c.mu.RUnlock()

    if healthy {
        key := fmt.Sprintf("user:%s:settings:all", userID)
        if err := c.redis.Set(ctx, key, settings.ToJSON(), 0).Err(); err != nil {
            // Delete stale key to force cache miss
            c.redis.Del(ctx, key)
            c.recordRedisFailure()
            log.Warn("Redis update failed after DB write, deleted stale cache key: %s", key)
        }
    } else {
        log.Warn("Redis degraded, skipping cache update (DB write succeeded)")
    }

    return nil // DB write succeeded, that's what matters
}
```

### Health Check Endpoint
```go
// GET /api/health/redis
func RedisHealthHandler(w http.ResponseWriter, r *http.Request) {
    status := cache.GetRedisStatus()

    response := map[string]interface{}{
        "redis_healthy":        status.Healthy,
        "circuit_breaker_open": status.CircuitBreakerOpen,
        "failure_count":        status.FailureCount,
        "mode":                 status.Mode, // "cache" or "db-only"
    }

    if status.Healthy {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }

    json.NewEncoder(w).Encode(response)
}
```

### Fallback Strategy Matrix

| Scenario | Read Behavior | Write Behavior | Trading Impact |
|----------|--------------|----------------|----------------|
| **Redis Healthy** | Cache → DB fallback | DB → Cache update | Optimal (<1ms) |
| **Redis Unavailable** | DB only | DB only (skip cache) | Degraded (50ms) |
| **1-2 Redis Failures** | Try cache → DB fallback | DB → Try cache | Warning logged |
| **3+ Redis Failures** | DB only (circuit open) | DB only | Degraded mode |
| **Redis Recovered** | Resume cache | Resume cache | Auto-recovery |

### Monitoring & Logging

```go
// Log levels for Redis failures
func (c *CacheService) logRedisError(operation string, err error) {
    c.mu.RLock()
    failureCount := c.redisFailureCount
    c.mu.RUnlock()

    switch {
    case failureCount == 1:
        log.Warn("Redis %s failed (attempt 1/3): %v", operation, err)
    case failureCount == 2:
        log.Warn("Redis %s failed (attempt 2/3): %v", operation, err)
    case failureCount >= 3:
        log.Error("Redis %s failed (circuit breaker OPEN): %v", operation, err)
    }
}
```

### Auto-Recovery
- Health check runs every 10 seconds
- Ping Redis to verify connection
- If successful after failures:
  - Reset failure count
  - Close circuit breaker
  - Resume cache operations
  - Log recovery event

## Dependencies
- **Blocked By:** Stories 6.1-6.8 (all cache infrastructure must exist)
- **Blocks:** None (final resilience story)

## Files to Create/Modify
- `internal/cache/circuit_breaker.go` - Circuit breaker logic
- `internal/cache/health.go` - Redis health check and monitoring
- `internal/cache/service.go` - Add fallback logic to all methods
- `internal/api/handlers_health.go` - Redis health check endpoint
- `internal/monitoring/metrics.go` - Add Redis health metrics

## Testing Requirements

### Unit Tests
- Test circuit breaker opens after 3 failures
- Test circuit breaker closes on successful health check
- Test read fallback: Redis fails → DB read successful
- Test write fallback: Redis fails → DB write successful → Cache skipped
- Test failure count increment and reset
- Test health check ping logic

### Integration Tests
- **Test Redis Unavailable**:
  - Stop Redis container
  - Execute read operations → Should fall back to DB
  - Execute write operations → Should write to DB only
  - Verify no errors returned to clients
  - Verify degraded mode logged
- **Test Redis Recovery**:
  - Start with Redis down
  - Circuit breaker opens
  - Restart Redis container
  - Wait for health check
  - Verify circuit breaker closes
  - Verify cache operations resume
- **Test Partial Failure**:
  - Simulate 2 Redis failures
  - Verify warning logs
  - Execute 3rd operation successfully
  - Verify failure count resets

### Failure Scenario Tests
- **Redis connection timeout**: Verify falls back to DB within timeout
- **Redis out of memory**: Verify graceful degradation
- **Redis auth failure**: Verify circuit breaker opens
- **Network partition**: Verify DB fallback works
- **Redis restart**: Verify auto-recovery

### Performance Tests
- **Degraded Mode Performance**: Measure DB-only mode response time (should be <50ms)
- **Failure Detection Time**: Measure time to detect Redis failure and fall back (<100ms)
- **Recovery Time**: Measure time to detect Redis recovery and resume caching (<10s)

### End-to-End Tests
- Test Ginie trading continues during Redis outage (uses DB)
- Test API endpoints continue working during Redis outage
- Test settings updates persist to DB even when Redis is down
- Test no data loss during Redis unavailability

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] Circuit breaker opens after 3 failures verified
- [ ] DB fallback tested for all cache operations
- [ ] Auto-recovery tested when Redis comes back online
- [ ] Health check endpoint implemented and tested
- [ ] Trading operations never fail due to Redis verified
- [ ] Failure scenarios tested (Redis down, network issues, etc.)
- [ ] Monitoring and logging implemented
- [ ] Performance in degraded mode acceptable (<50ms)
- [ ] Documentation updated with fallback behavior
- [ ] PO acceptance received
