# Story 9.10: Migrate SettingsManager from File to DB+Redis

## Story Information
- **Epic**: Epic 9 - Remove FuturesController and Consolidate to GinieAutopilot
- **Story ID**: 9.10
- **Priority**: High
- **Estimated Effort**: Medium (8-12 hours)
- **Created**: 2026-01-20
- **Status**: Ready for Development

---

## Problem Statement

### Current State - Technical Debt
The codebase still has 50+ calls to the legacy `GetSettingsManager()` which reads/writes to `autopilot_settings.json` file:

```go
sm := autopilot.GetSettingsManager()
settings, err := sm.LoadSettings()  // Reads from JSON file
sm.SaveSettings(settings)           // Writes to JSON file
```

This is inconsistent with the established architecture:
- Epic 6 implemented **DB + Redis caching** for user settings
- `default-settings.json` is now the source of truth for defaults
- User settings are stored in PostgreSQL with Redis cache layer
- Mode-strategy configurations use write-through caching

### Files Still Using Legacy SettingsManager

| File | Calls | Purpose |
|------|-------|---------|
| `main.go` | 3 | Startup initialization |
| `handlers_futures_autopilot.go` | 16+ | Circuit breaker, mode switching |
| `handlers_spot_autopilot.go` | 4 | Spot autopilot settings |
| `spot_controller.go` | 3 | Spot trading settings |
| `ginie_patterns.go` | 2 | Pattern trading defaults |
| `ginie_adaptive.go` | 2 | Adaptive learning saves |

### The Problem
1. **Inconsistent Data Source**: Some settings from DB, some from file
2. **No Multi-User Support**: File-based settings are single-user
3. **No Cache Benefits**: File reads bypass Redis cache
4. **Runtime State Pollution**: `autopilot_settings.json` contains runtime state (PnL, trade counts) mixed with config

---

## Solution: Replace File-Based Calls with DB+Redis

### Architecture After Migration

```
┌─────────────────────────────────────────────────────────────┐
│  default-settings.json (Source of Truth for DEFAULTS)       │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼ (on first user login)
┌─────────────────────────────────────────────────────────────┐
│  PostgreSQL (User Settings Storage)                         │
│  - user_ginie_settings (global settings)                    │
│  - user_mode_configs (mode settings)                        │
│  - user_mode_strategy_configs (mode+strategy settings)      │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼ (write-through cache)
┌─────────────────────────────────────────────────────────────┐
│  Redis Cache Layer                                          │
│  - global:{userID}:trading (global settings)                │
│  - mode:{userID}:{mode}:{strategy} (mode-strategy)          │
│  - circuit_breaker:{userID} (circuit breaker state)         │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼ (API layer)
┌─────────────────────────────────────────────────────────────┐
│  GinieAutopilot / Controllers                               │
│  - Read from Cache (fast)                                   │
│  - Fallback to DB if cache miss                             │
│  - Write through to DB + Cache                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Technical Specification

### Task 1: Replace SettingsManager Calls in API Handlers

**File**: `internal/api/handlers_futures_autopilot.go`

Replace pattern:
```go
// BEFORE (legacy)
sm := autopilot.GetSettingsManager()
settings, err := sm.LoadSettings()

// AFTER (DB+Redis)
userID, _ := getUserID(r)
settings, err := s.cacheService.GetGlobalTradingSettings(r.Context(), userID)
```

For writes:
```go
// BEFORE
sm.SaveSettings(settings)

// AFTER
s.cacheService.SetGlobalTradingSettings(r.Context(), userID, settings)
```

### Task 2: Replace SettingsManager Calls in main.go

**File**: `main.go`

The initialization in main.go should:
1. Load defaults from `default-settings.json` (already exists)
2. Apply to new users via `user_initialization.go` (already exists)
3. Remove file-based settings loading

### Task 3: Replace SettingsManager Calls in Controllers

**Files**:
- `internal/autopilot/spot_controller.go`
- `internal/autopilot/ginie_patterns.go`
- `internal/autopilot/ginie_adaptive.go`

These need access to cache service. Options:
1. Pass cache service to constructors
2. Use dependency injection
3. Access via parent struct (UserAutopilotManager)

### Task 4: Remove autopilot_settings.json

After all calls migrated:
1. Add `autopilot_settings.json` to `.gitignore`
2. Remove from git tracking
3. Remove SettingsManager struct and related code

---

## Acceptance Criteria

### AC9.10.1: API Handlers Migrated
- [ ] All `GetSettingsManager()` calls removed from `handlers_futures_autopilot.go`
- [ ] Settings read from cache service with user context
- [ ] Settings write through cache to DB

### AC9.10.2: Controllers Migrated
- [ ] `spot_controller.go` uses cache service
- [ ] `ginie_patterns.go` uses cache for defaults
- [ ] `ginie_adaptive.go` saves via cache service

### AC9.10.3: main.go Updated
- [ ] No file-based settings loading on startup
- [ ] Defaults loaded from `default-settings.json`

### AC9.10.4: File Removed
- [ ] `autopilot_settings.json` removed from repo
- [ ] Added to `.gitignore`
- [ ] SettingsManager code deprecated or removed

### AC9.10.5: Multi-User Support
- [ ] Each user has isolated settings
- [ ] No shared state in file

### AC9.10.6: No Regressions
- [ ] All existing functionality works
- [ ] Settings persist across restarts
- [ ] Cache properly populated

---

## Implementation Plan

### Phase 1: API Handlers (handlers_futures_autopilot.go)
1. Add userID extraction to each handler
2. Replace LoadSettings → cache.GetGlobalTradingSettings
3. Replace SaveSettings → cache.SetGlobalTradingSettings
4. Test each endpoint

### Phase 2: Controllers
1. Add cacheService to controller constructors
2. Update spot_controller.go
3. Update ginie_patterns.go
4. Update ginie_adaptive.go

### Phase 3: Cleanup
1. Update main.go initialization
2. Remove autopilot_settings.json
3. Remove SettingsManager code
4. Update .gitignore

---

## Dependencies

- Epic 6 (Redis Caching Infrastructure) - COMPLETE
- Story 6.5 (Cache-First Read Pattern) - COMPLETE
- Story 6.6 (Ginie Engine Cache Reads) - COMPLETE

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Data loss during migration | High | Keep file as backup until verified |
| Missing settings | Medium | Map all file fields to DB equivalents |
| Performance regression | Low | Redis cache faster than file reads |
| Breaking existing flows | Medium | Incremental migration, test each endpoint |

---

## Testing Plan

1. **Unit Tests**: Mock cache service, verify settings CRUD
2. **Integration Tests**: Full flow with Redis + PostgreSQL
3. **Regression Tests**: All existing API endpoints work
4. **Multi-User Test**: Two users with different settings

---

## Notes

This story completes the migration from file-based settings to the DB+Redis architecture established in Epic 6. After completion, `autopilot_settings.json` can be fully removed and all settings will be properly persisted with multi-user support.
