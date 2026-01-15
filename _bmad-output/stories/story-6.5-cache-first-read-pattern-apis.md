# Story 6.5: Cache-First Read Pattern for ALL Settings APIs

**Story ID:** CACHE-6.5
**Epic:** Epic 6 - Redis Caching Infrastructure
**Priority:** P0 (Critical - API performance)
**Story Points:** 13
**Estimated Effort:** 18 hours
**Author:** BMAD Party Mode (Bob - Scrum Master)
**Status:** Done
**Version:** 7.1 (Implementation Complete - January 2026)

---

## Description

Refactor **ALL settings-related API endpoints** to use cache-first pattern. This includes:
- **User Mode Settings APIs** → `SettingsCacheService`
- **Admin Defaults APIs** → `AdminDefaultsCacheService`
- **Ginie Configuration APIs** → Various cache keys
- **Circuit Breaker APIs** → Cache-first reads
- **All Other Settings APIs** → Comprehensive coverage

**Scope**: 51 endpoints across 14 categories (verified against server.go)

APIs will read from Redis cache and return HTTP 503 if Redis is unavailable (no silent DB fallback).

---

## Current State Analysis (CRITICAL)

### Cache Infrastructure: READY ✅
- `SettingsCacheService` - Fully implemented (21KB, internal/cache/settings_cache_service.go)
- `AdminDefaultsCacheService` - Fully implemented (18KB, internal/cache/admin_defaults_cache.go)
- Cache is hydrated at user login (Story 6.2)
- Admin cache refreshes after sync operations (Story 6.4)

### API Handlers: CACHE-FIRST WIRED ✅
**All settings API handlers now use cache-first pattern.**

| Handler File | Current Pattern | Cache Usage |
|--------------|-----------------|-------------|
| `handlers_ginie.go` | **CACHE-FIRST** | Mode configs, circuit breaker, LLM config |
| `handlers_mode.go` | **CACHE-FIRST** | Capital allocation |
| `handlers_settings.go` | **CACHE-FIRST** | User settings |
| `handlers_settings_defaults.go` | **CACHE-FIRST** | Global trading, safety settings |
| `handlers_admin.go` | **CACHE-FIRST** | Admin defaults (mode, global) |
| `handlers_user_settings.go` | **CACHE-FIRST** | Settings comparison, reset |

**Implementation Complete**: All GET endpoints read from cache, return HTTP 503 if unavailable.

### This Story's Purpose
Wire all API handlers to:
1. Read from cache instead of DB
2. Return HTTP 503 if cache unavailable
3. Use write-through for updates (DB first → cache update)

---

## User Story

> As an API consumer,
> I want ALL settings GET endpoints to read from Redis cache,
> So that API response times are <5ms and the system behaves consistently with the "Redis is the brain" architecture.

---

## Critical Architecture Principle

```
┌─────────────────────────────────────────────────────────────────────────┐
│                    API LAYER (Story 6.5) - FULL SCOPE                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐       │
│  │ Mode Config APIs │  │ Circuit Breaker  │  │ Admin Defaults   │       │
│  │ (4 modes × 20)   │  │ APIs (3 types)   │  │ APIs             │       │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘       │
│           │                     │                     │                  │
│  ┌────────┴─────────┐  ┌────────┴─────────┐  ┌────────┴─────────┐       │
│  │ LLM Config APIs  │  │ Capital Alloc    │  │ Safety Settings  │       │
│  │                  │  │ APIs             │  │ APIs             │       │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘       │
│           │                     │                     │                  │
│  ┌────────┴─────────┐  ┌────────┴─────────┐  ┌────────┴─────────┐       │
│  │ Hedge Config     │  │ Global Trading   │  │ Autopilot        │       │
│  │ APIs             │  │ APIs (NEW)       │  │ Config APIs      │       │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘       │
│           │                     │                     │                  │
│           └─────────────────────┼─────────────────────┘                  │
│                                 │                                        │
│                                 v                                        │
│                    ┌────────────────────────┐                           │
│                    │    CACHE SERVICES      │                           │
│                    │  SettingsCacheService  │                           │
│                    │  AdminDefaultsCache    │                           │
│                    └───────────┬────────────┘                           │
│                                │                                        │
│                                v                                        │
│                             REDIS                                       │
│                                │                                        │
│                         ┌──────┴──────┐                                 │
│                         │             │                                 │
│                         v             v                                 │
│                     Redis OK     Redis DOWN                             │
│                         │             │                                 │
│                         v             v                                 │
│                    Return data   HTTP 503                               │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Complete Endpoint Inventory (51 Endpoints - Verified)

### Category 1: Mode Configuration (5 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/mode-configs` | EXISTS | MGET all modes |
| GET | `/api/futures/ginie/mode-config/:mode` | EXISTS | `user:{id}:mode:{mode}:*` |
| PUT | `/api/futures/ginie/mode-config/:mode` | EXISTS | Write-through |
| POST | `/api/futures/ginie/mode-config/:mode/toggle` | EXISTS | `user:{id}:mode:{mode}:enabled` |
| POST | `/api/futures/ginie/mode-config/reset` | EXISTS | Write-through |

### Category 2: SLTP Config (2 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/sltp-config` | EXISTS | `user:{id}:mode:{mode}:sltp` |
| POST | `/api/futures/ginie/sltp-config/:mode` | EXISTS | Write-through |

### Category 3: LLM Configuration (3 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/llm-config` | EXISTS | `user:{id}:llm_config` |
| PUT | `/api/futures/ginie/llm-config` | EXISTS | Write-through |
| PUT | `/api/futures/ginie/llm-config/:mode` | EXISTS | `user:{id}:mode:{mode}:llm` |

### Category 4: Circuit Breaker (6 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/user/global-circuit-breaker` | EXISTS | `user:{id}:circuit_breaker` |
| PUT | `/api/user/global-circuit-breaker` | EXISTS | Write-through |
| GET | `/api/futures/ginie/circuit-breaker/status` | EXISTS | Same as above |
| GET | `/api/futures/autopilot/circuit-breaker/status` | EXISTS | Same as above |
| GET | `/api/futures/ginie/mode-circuit-breaker-status` | EXISTS | `user:{id}:mode:*:circuit_breaker` |
| POST | `/api/futures/ginie/mode-circuit-breaker/:mode/reset` | EXISTS | Write-through |

### Category 5: Capital Allocation (3 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/modes/allocations` | EXISTS | `user:{id}:capital_allocation` |
| GET | `/api/futures/modes/allocations/:mode` | EXISTS | Extract from above |
| POST | `/api/futures/modes/allocations` | EXISTS | Write-through |

**Note**: No direct `GET /api/futures/ginie/capital-allocation` endpoint (uses `/api/futures/modes/allocations`)

### Category 6: Safety Settings (3 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/safety-settings` | EXISTS | `user:{id}:safety:{mode}` (all 4 modes) |
| PUT | `/api/futures/ginie/safety-settings/:mode` | EXISTS | Write-through |
| POST | `/api/futures/ginie/safety-settings/load-defaults` | EXISTS | Copy admin → user |

### Category 7: Hedge Mode Config (4 endpoints) ✅ ALL EXIST
**Note**: Hedge endpoints are GLOBAL, not per-mode (no `:mode` parameter)
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/hedge-config` | EXISTS | `user:{id}:mode:{active}:hedge` |
| POST | `/api/futures/ginie/hedge-config` | EXISTS | Write-through |
| POST | `/api/futures/ginie/hedge-mode/toggle` | EXISTS | Write-through |
| POST | `/api/futures/ginie/hedge-mode/load-defaults` | EXISTS | Copy admin → user |

### Category 8: Autopilot Config (3 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/autopilot/config` | EXISTS | Aggregate from mode settings |
| GET | `/api/futures/autopilot/config` | EXISTS | Same (alias) |
| POST | `/api/futures/ginie/autopilot/config` | EXISTS | Write-through |

### Category 9: Global Trading (2 endpoints) 🆕 TO BE CREATED
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/global-trading` | **CREATE** | `user:{id}:global_trading` |
| PUT | `/api/futures/ginie/global-trading` | **CREATE** | Write-through |

**Note**: DB table exists (Story 6.2), endpoints need to be added.

### Category 10: Admin Defaults (3 endpoints) ⚠️ PARTIAL
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/futures/ginie/default-settings` | EXISTS | All admin defaults |
| GET | `/api/admin/defaults/:mode` | **CREATE** | `admin:defaults:mode:{mode}:*` |
| GET | `/api/admin/defaults/global/:setting` | **CREATE** | `admin:defaults:global:{setting}` |

### Category 11: Settings Comparison & Reset (4 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| GET | `/api/user/settings/comparison` | EXISTS | Both caches |
| POST | `/api/user/settings/reset` | EXISTS | Copy admin → user |
| GET | `/api/settings/diff/modes/:mode` | EXISTS | Compare mode keys |
| POST | `/api/settings/load-defaults` | EXISTS | Copy all defaults |

### Category 12: Load Defaults APIs (9 endpoints) ✅ ALL EXIST
| Method | Endpoint | Status | Cache Key Pattern |
|--------|----------|--------|-------------------|
| POST | `/api/futures/ginie/modes/:mode/load-defaults` | EXISTS | Copy admin → user |
| POST | `/api/futures/ginie/modes/:mode/groups/:group/reset` | EXISTS | Single group |
| POST | `/api/futures/ginie/modes/load-defaults` | EXISTS | All modes |
| POST | `/api/futures/ginie/modes/reset-all` | EXISTS | Reset all |
| POST | `/api/futures/ginie/circuit-breaker/load-defaults` | EXISTS | CB defaults |
| POST | `/api/futures/ginie/llm-config/load-defaults` | EXISTS | LLM defaults |
| POST | `/api/futures/ginie/capital-allocation/load-defaults` | EXISTS | Allocation defaults |
| POST | `/api/futures/ginie/safety-settings/load-defaults` | EXISTS | Safety defaults |
| POST | `/api/futures/ginie/position-optimization/load-defaults` | EXISTS | Position opt defaults |

### Category 13: Position Optimization (via Mode Groups)
**Note**: Position optimization is accessed through mode config endpoints (one of 20 groups per mode).
No dedicated position optimization GET/POST endpoints exist - use `GET /api/futures/ginie/mode-config/:mode` and extract `position_optimization` group.

### Category 14: Individual Mode Group Endpoints (Implicit via Mode Config)
Mode groups (confidence, sltp, position_optimization, hedge, etc.) are accessed via:
- `GET /api/futures/ginie/mode-config/:mode` → Returns all 20 groups
- Individual group updates via `POST /api/futures/ginie/modes/:mode/groups/:group/reset`

---

## Endpoint Summary

| Status | Count | Description |
|--------|-------|-------------|
| ✅ EXISTS (Refactor) | 47 | Wire to cache-first pattern |
| 🆕 CREATE | 4 | New endpoints needed |
| **TOTAL** | **51** | |

### Endpoints to Create (4)
1. `GET /api/futures/ginie/global-trading`
2. `PUT /api/futures/ginie/global-trading`
3. `GET /api/admin/defaults/:mode`
4. `GET /api/admin/defaults/global/:setting`

---

## Acceptance Criteria

### AC6.5.1: Mode Configuration APIs
- [ ] `GET /api/futures/ginie/mode-configs` → Cache-first (MGET all modes)
- [ ] `GET /api/futures/ginie/mode-config/:mode` → Cache-first (MGET 20 keys)
- [ ] `PUT /api/futures/ginie/mode-config/:mode` → Write-through
- [ ] `POST /api/futures/ginie/mode-config/:mode/toggle` → Write-through
- [ ] `POST /api/futures/ginie/mode-config/reset` → Write-through
- [ ] Response time <5ms for GET endpoints

### AC6.5.2: SLTP & LLM Configuration APIs
- [ ] `GET /api/futures/ginie/sltp-config` → Cache-first
- [ ] `POST /api/futures/ginie/sltp-config/:mode` → Write-through
- [ ] `GET /api/futures/ginie/llm-config` → Cache-first (`user:{id}:llm_config`)
- [ ] `PUT /api/futures/ginie/llm-config` → Write-through
- [ ] `PUT /api/futures/ginie/llm-config/:mode` → Write-through

### AC6.5.3: Circuit Breaker APIs (All Types)
- [ ] `GET /api/user/global-circuit-breaker` → Cache-first
- [ ] `PUT /api/user/global-circuit-breaker` → Write-through
- [ ] `GET /api/futures/ginie/circuit-breaker/status` → Cache-first
- [ ] `GET /api/futures/autopilot/circuit-breaker/status` → Cache-first
- [ ] `GET /api/futures/ginie/mode-circuit-breaker-status` → Cache-first (per-mode)
- [ ] `POST /api/futures/ginie/mode-circuit-breaker/:mode/reset` → Write-through

### AC6.5.4: Capital Allocation & Safety APIs
- [ ] `GET /api/futures/modes/allocations` → Cache-first (`user:{id}:capital_allocation`)
- [ ] `GET /api/futures/modes/allocations/:mode` → Cache-first (extract)
- [ ] `POST /api/futures/modes/allocations` → Write-through
- [ ] `GET /api/futures/ginie/safety-settings` → Cache-first (all 4 modes)
- [ ] `PUT /api/futures/ginie/safety-settings/:mode` → Write-through

### AC6.5.5: Hedge Mode & Autopilot APIs
- [ ] `GET /api/futures/ginie/hedge-config` → Cache-first (global, not per-mode)
- [ ] `POST /api/futures/ginie/hedge-config` → Write-through
- [ ] `POST /api/futures/ginie/hedge-mode/toggle` → Write-through
- [ ] `GET /api/futures/ginie/autopilot/config` → Cache-first (MGET)
- [ ] `GET /api/futures/autopilot/config` → Cache-first (alias)
- [ ] `POST /api/futures/ginie/autopilot/config` → Write-through

### AC6.5.6: Global Trading APIs (NEW - Create Endpoints)
- [ ] **CREATE** `GET /api/futures/ginie/global-trading` → Cache-first (`user:{id}:global_trading`)
- [ ] **CREATE** `PUT /api/futures/ginie/global-trading` → Write-through
- [ ] Add route in server.go
- [ ] Add handler in handlers_ginie.go

### AC6.5.7: Admin Defaults APIs
- [ ] `GET /api/futures/ginie/default-settings` → Cache-first (AdminDefaultsCacheService)
- [ ] **CREATE** `GET /api/admin/defaults/:mode` → Cache-first
- [ ] **CREATE** `GET /api/admin/defaults/global/:setting` → Cache-first

### AC6.5.8: Settings Comparison & Reset APIs
- [ ] `GET /api/user/settings/comparison` → Both caches (user + admin)
- [ ] `POST /api/user/settings/reset` → Write-through (copy admin → user)
- [ ] `GET /api/settings/diff/modes/:mode` → Both caches
- [ ] `POST /api/settings/load-defaults` → Write-through

### AC6.5.9: Load Defaults APIs (Write Operations)
- [ ] All 9 load-defaults endpoints → Write-through pattern
- [ ] Read from AdminDefaultsCacheService
- [ ] Write to DB first, then update user cache

### AC6.5.10: Error Handling (Redis Down = 503)
- [ ] ALL GET endpoints return HTTP 503 when `ErrCacheUnavailable`
- [ ] Error response: `{"error": "Cache unavailable", "code": "CACHE_UNAVAILABLE", "retry_after": 5}`
- [ ] NO silent fallback to database
- [ ] Logging: Warn level for all 503 responses

### AC6.5.11: Write-Through Pattern
- [ ] All PUT/POST endpoints use write-through: DB first → Cache update
- [ ] Partial updates: Only changed groups are updated
- [ ] Cache invalidation on write failure

---

## Sprint Tasks

### Phase 1: Infrastructure & Error Handling (2 points)
| Task | Description | Points |
|------|-------------|--------|
| **6.5.1** | Create `RespondCacheUnavailable()` helper in errors.go | 0.5 |
| **6.5.2** | Create cache-first wrapper/pattern for handlers | 1.0 |
| **6.5.3** | Create write-through wrapper/pattern for handlers | 0.5 |

### Phase 2: Core Mode Configuration (3 points)
| Task | Description | Points |
|------|-------------|--------|
| **6.5.4** | Refactor `handleGetModeConfigs` → Cache-first | 0.5 |
| **6.5.5** | Refactor `handleGetModeConfig` → Cache-first | 0.5 |
| **6.5.6** | Refactor `handleUpdateModeConfig` → Write-through | 0.5 |
| **6.5.7** | Refactor `handleToggleModeConfig` → Write-through | 0.5 |
| **6.5.8** | Refactor SLTP config handlers | 0.5 |
| **6.5.9** | Refactor LLM config handlers (3 endpoints) | 0.5 |

### Phase 3: Circuit Breaker & Capital Allocation (2.5 points)
| Task | Description | Points |
|------|-------------|--------|
| **6.5.10** | Refactor all Circuit Breaker handlers (6 endpoints) | 1.5 |
| **6.5.11** | Refactor Capital Allocation handlers (3 endpoints) | 0.5 |
| **6.5.12** | Refactor Safety Settings handlers (3 endpoints) | 0.5 |

### Phase 4: Hedge, Autopilot, Global Trading (2.5 points)
| Task | Description | Points |
|------|-------------|--------|
| **6.5.13** | Refactor Hedge Mode handlers (4 endpoints, global) | 0.5 |
| **6.5.14** | Refactor Autopilot Config handlers (3 endpoints) | 0.5 |
| **6.5.15** | **CREATE** Global Trading endpoints + handlers | 1.0 |
| **6.5.16** | **CREATE** Admin Defaults endpoints (2 new) | 0.5 |

### Phase 5: Settings Comparison & Load Defaults (1.5 points)
| Task | Description | Points |
|------|-------------|--------|
| **6.5.17** | Refactor Settings Comparison handlers (4 endpoints) | 0.5 |
| **6.5.18** | Refactor Load Defaults handlers (9 endpoints) | 0.5 |
| **6.5.19** | Refactor Admin Defaults handler | 0.5 |

### Phase 6: Testing (1.5 points)
| Task | Description | Points |
|------|-------------|--------|
| **6.5.20** | Unit tests: Cache-first pattern for all categories | 0.5 |
| **6.5.21** | Unit tests: HTTP 503 on cache unavailable | 0.5 |
| **6.5.22** | Integration tests: API → Cache → Response cycle | 0.25 |
| **6.5.23** | Performance tests: <5ms verification | 0.25 |

**Total: 13 Story Points**

---

## Cache Key Alignment with Story 6.2 v7.1 (88 Keys Architecture)

Story 6.5 uses the cache keys defined in Story 6.2 v7.1:

| Cache Category | Keys | Pattern |
|----------------|------|---------|
| Mode Settings | 80 | `user:{id}:mode:{mode}:{group}` (4 modes × 20 groups) |
| Global Circuit Breaker | 1 | `user:{id}:circuit_breaker` |
| Global LLM Config | 1 | `user:{id}:llm_config` |
| Capital Allocation | 1 | `user:{id}:capital_allocation` |
| Global Trading | 1 | `user:{id}:global_trading` |
| Safety Settings | 4 | `user:{id}:safety:{mode}` |
| **TOTAL** | **88** | |

**Important Notes:**
- `position_optimization` is within mode groups (one of the 20 groups per mode)
- `hedge_config` is within mode groups as `hedge`
- Hedge endpoints are GLOBAL (no `:mode` parameter) - use active mode
- Safety settings use per-mode keys: `user:{id}:safety:scalp`, etc.

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/api/handlers_ginie.go` | Refactor all Ginie settings handlers to cache-first |
| `internal/api/handlers_mode.go` | Refactor trading mode handlers |
| `internal/api/handlers_settings.go` | Refactor user settings handlers |
| `internal/api/handlers_settings_defaults.go` | Refactor load defaults handlers |
| `internal/api/handlers_admin.go` | Refactor admin handlers, add new endpoints |
| `internal/api/handlers_admin_settings.go` | Refactor admin settings handlers |
| `internal/api/server.go` | Add new routes for global-trading, admin defaults |
| `internal/api/errors.go` | Add `RespondCacheUnavailable()` helper |
| `internal/cache/settings_cache_service.go` | Verify all getter methods exist |

---

## Testing Requirements

### Unit Tests (P0)
```go
// Cache-First Pattern
func TestGetModeConfig_CacheHit_ReturnsData(t *testing.T)
func TestGetModeConfig_CacheUnavailable_Returns503(t *testing.T)
func TestGetAllModeConfigs_CacheHit_ReturnsAllModes(t *testing.T)

// Write-Through Pattern
func TestUpdateModeConfig_WritesDBFirst_ThenCache(t *testing.T)
func TestUpdateModeConfig_DBFails_NoCache Update(t *testing.T)

// LLM Config
func TestGetLLMConfig_CacheHit_ReturnsData(t *testing.T)
func TestUpdateLLMConfig_WriteThrough(t *testing.T)

// Circuit Breaker (all types)
func TestGetGlobalCircuitBreaker_CacheHit(t *testing.T)
func TestGetGinieCircuitBreaker_CacheHit(t *testing.T)
func TestGetModeCircuitBreaker_CacheHit(t *testing.T)

// Capital Allocation
func TestGetModeAllocations_CacheHit(t *testing.T)
func TestGetModeAllocationByMode_CacheHit(t *testing.T)

// Safety Settings (per-mode)
func TestGetSafetySettings_CacheHit(t *testing.T)
func TestUpdateSafetySettings_WriteThrough(t *testing.T)

// Hedge Config (global)
func TestGetHedgeConfig_CacheHit_UsesActiveMode(t *testing.T)

// Global Trading (NEW)
func TestGetGlobalTrading_CacheHit(t *testing.T)
func TestUpdateGlobalTrading_WriteThrough(t *testing.T)

// Admin Defaults
func TestGetAdminDefaults_CacheHit(t *testing.T)
func TestGetAdminModeDefaults_CacheHit(t *testing.T)
func TestCompareSettings_BothCaches(t *testing.T)

// Error Handling
func TestAllGETEndpoints_CacheUnavailable_Returns503(t *testing.T)
func TestWriteThrough_CacheUpdateFails_LogsWarning(t *testing.T)
```

### Integration Tests
- Full request cycle for each category
- Response time verification: <5ms for cache hits
- 503 response when Redis is stopped
- Write-through verification (DB + Cache consistency)

### Performance Tests
- **Baseline**: Current API response times (50-200ms due to DB queries)
- **Target**: <5ms for ALL GET endpoints
- **Load**: 1000 req/sec across all endpoint categories

---

## Definition of Done

### Implementation
- [ ] All 47 existing endpoints refactored to cache-first pattern
- [ ] 4 new endpoints created (global-trading, admin defaults)
- [ ] All GET handlers use appropriate cache service
- [ ] All PUT/POST handlers use write-through pattern
- [ ] HTTP 503 returned for ALL endpoints when cache unavailable
- [ ] No direct DB queries in handlers for settings data (except writes)

### Testing
- [ ] Unit tests: All categories covered (cache-first + write-through)
- [ ] Unit tests: Cache unavailable returns 503 for all GET endpoints
- [ ] Integration: Full request cycle works
- [ ] Performance: <5ms response time verified for all GET endpoints

### Monitoring
- [ ] Logging for 503 responses (all endpoints)
- [ ] Log warnings when cache update fails in write-through

---

## Dependencies

| Dependency | Status |
|------------|--------|
| Story 6.2 v7.1 - User Settings Cache (88 keys) | ✅ COMPLETE |
| Story 6.4 - Admin Defaults Cache (89 keys with hash) | ✅ COMPLETE |
| SettingsCacheService | ✅ EXISTS |
| AdminDefaultsCacheService | ✅ EXISTS |
| DB table: `user_global_trading` | ✅ EXISTS (from 6.2) |
| Repository: `GetUserGlobalTrading` | ✅ EXISTS (from 6.2) |

**All dependencies satisfied - Ready for development.**

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Large scope (51 endpoints) | Phase-based implementation, pattern-based refactoring |
| Redis unavailable | Return 503, frontend shows retry message |
| Cache miss | Not applicable - cache is always hydrated at login |
| Breaking existing functionality | Extensive unit tests before refactoring |
| Performance regression | Measure before/after for each category |

---

## Author

**Created By:** BMAD Party Mode (Bob, Winston, Mary, John, Murat, Amelia)
**Date:** 2026-01-15
**Version:** 7.0 (Verified against actual codebase)
**Sprint Planned:** 2026-01-15
**Status:** Ready for Development

---

## Changelog

| Version | Date | Changes |
|---------|------|---------|
| 1.0 | 2026-01-14 | Initial draft |
| 2.0 | 2026-01-15 | Aligned with 6.2 granular architecture |
| 3.0 | 2026-01-15 | Added Admin Defaults APIs |
| 4.0 | 2026-01-15 | **FULL SCOPE**: Added 27 missing endpoints from API audit |
| 5.0 | 2026-01-15 | Aligned with Story 6.2 v6.0 (89 keys) |
| 6.0 | 2026-01-15 | Aligned with Story 6.2 v7.0 (88 keys) |
| 7.0 | 2026-01-15 | **VERIFIED AGAINST CODEBASE**: Fixed endpoint inventory (51 total), identified 4 endpoints to create, corrected hedge config (global not per-mode), added current state analysis showing handlers are DB-first, updated dependencies to show 6.2 complete |
