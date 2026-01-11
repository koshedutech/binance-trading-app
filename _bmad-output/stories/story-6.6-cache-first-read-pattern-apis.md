# Story 6.6: Cache-First Read Pattern for All Settings APIs
**Epic:** Epic 6: Redis Caching Infrastructure
**Sprint:** Sprint 6
**Story Points:** 5
**Priority:** P0

## User Story
As an API consumer, I want all settings API endpoints to use cache-first pattern so that API response times are <5ms instead of 50-200ms.

## Acceptance Criteria
- [ ] `GET /api/user/settings` reads from cache
- [ ] `GET /api/futures/ginie/modes/:mode` reads from cache
- [ ] `GET /api/futures/ginie/scalp-reentry-config` reads from cache
- [ ] `GET /api/user/global-circuit-breaker` reads from cache
- [ ] `GET /api/futures/ginie/capital-allocation` reads from cache
- [ ] All GET handlers: Redis first → DB fallback → populate cache
- [ ] API response time reduced from 50-200ms to <5ms

## Technical Approach

### API Handler Refactoring Pattern
Before (DB-only):
```go
func GetUserSettings(w http.ResponseWriter, r *http.Request) {
    userID := getUserID(r)
    settings, err := db.GetUserSettings(userID) // 50ms DB query
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    json.NewEncoder(w).Encode(settings)
}
```

After (Cache-first):
```go
func GetUserSettings(w http.ResponseWriter, r *http.Request) {
    userID := getUserID(r)
    settings, err := cache.GetUserSettings(userID) // <1ms cache read
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }
    json.NewEncoder(w).Encode(settings)
}
```

### Endpoints to Refactor

| Endpoint | Current | Target | Cache Key |
|----------|---------|--------|-----------|
| `GET /api/user/settings` | 50-100ms | <5ms | `user:{id}:settings:all` |
| `GET /api/futures/ginie/modes/ultra` | 50ms | <5ms | `user:{id}:mode:ultra` |
| `GET /api/futures/ginie/modes/scalp` | 50ms | <5ms | `user:{id}:mode:scalp` |
| `GET /api/futures/ginie/modes/swing` | 50ms | <5ms | `user:{id}:mode:swing` |
| `GET /api/futures/ginie/modes/position` | 50ms | <5ms | `user:{id}:mode:position` |
| `GET /api/futures/ginie/scalp-reentry-config` | 50ms | <5ms | `user:{id}:scalp_reentry` |
| `GET /api/user/global-circuit-breaker` | 50ms | <5ms | `user:{id}:circuit_breaker` |
| `GET /api/futures/ginie/capital-allocation` | 50ms | <5ms | Part of `user:{id}:settings:all` |

### CacheService Integration
All handlers will inject CacheService:
```go
type Handler struct {
    cache *cache.CacheService
    db    *database.Repository
}

func NewHandler(cache *cache.CacheService, db *database.Repository) *Handler {
    return &Handler{cache: cache, db: db}
}
```

### Graceful Degradation
All handlers maintain DB fallback via CacheService:
- CacheService handles Redis → DB fallback internally
- Handlers don't need to know if Redis is available
- Consistent API response even if Redis is down

## Dependencies
- **Blocked By:** Stories 6.2, 6.3, 6.4, 6.5 (cache implementations for each config type)
- **Blocks:** None (optimization story)

## Files to Create/Modify
- `internal/api/handlers_settings.go` - Refactor all settings GET handlers
- `internal/api/handlers_ginie.go` - Refactor Ginie GET handlers
- `internal/api/handlers_modes.go` - Refactor mode GET handlers
- `internal/api/middleware.go` - Inject CacheService into request context
- `internal/api/server.go` - Initialize handlers with CacheService

## Testing Requirements

### Unit Tests
- Mock CacheService for each handler
- Test GET /api/user/settings returns cached data
- Test GET /api/futures/ginie/modes/:mode returns cached mode config
- Test GET endpoints with cache hit scenario
- Test GET endpoints with cache miss scenario (DB fallback)

### Integration Tests
- Test full request cycle: Client → API → Cache → Response
- Test response time improvement: Measure before/after refactoring
- Test cache miss handling: Delete cache → API call → Loads from DB → Populates cache
- Test all 7 affected endpoints with real Redis and PostgreSQL
- Test concurrent API calls (100 requests/second)

### Performance Tests
- **Baseline**: Measure current API response times (expect 50-200ms)
- **Target**: Measure post-refactoring response times (expect <5ms)
- **Load Test**: 1000 requests/second to /api/user/settings
- **Cache Hit Ratio**: Verify >95% cache hit rate after warm-up
- **p50, p95, p99 Latencies**: Track latency percentiles

### End-to-End Tests
- Test UI → API → Cache flow for Ginie panel load
- Test settings update → Cache invalidation → Next read shows new data
- Test Redis unavailable → API still works (DB fallback)

## Definition of Done
- [ ] All acceptance criteria met
- [ ] Code reviewed
- [ ] Unit tests passing (>80% coverage)
- [ ] Integration tests passing
- [ ] All 7 GET endpoints refactored to cache-first
- [ ] API response time <5ms verified via load testing
- [ ] Cache hit ratio >95% verified
- [ ] DB fallback tested when Redis unavailable
- [ ] Performance benchmarks documented (before/after)
- [ ] Documentation updated
- [ ] PO acceptance received
