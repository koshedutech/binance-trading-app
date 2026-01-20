# Story 9.12: Fully Deprecate autopilot_settings.json

## Story Information
- **Epic**: Epic 9 - Remove FuturesController and Consolidate to GinieAutopilot
- **Story ID**: 9.12
- **Priority**: Medium
- **Estimated Effort**: Large (3-5 days)
- **Created**: 2026-01-20
- **Status**: Ready for Development
- **Dependencies**: Story 9.10, Story 9.11

---

## Problem Statement

### Current State
Despite migrating specific features to DB+Redis (Stories 9.10, 9.11), there are still **110+ usages** of `GetSettingsManager()` across the codebase that read/write to `autopilot_settings.json`.

### Remaining Usages by File

| File | Count | Primary Usage |
|------|-------|---------------|
| `ginie_autopilot.go` | 45 | Symbol settings, mode configs, circuit breaker stats |
| `handlers_ginie.go` | 22 | Morning auto-block, mode configs, symbol settings |
| `futures_controller.go` | 11 | Legacy controller settings |
| `ginie_analyzer.go` | 10 | Confluence config, symbol settings |
| `handlers_futures_autopilot.go` | 9 | Symbol performance, settings |
| `handlers_settings_defaults.go` | 7 | Default settings operations |
| `spot_controller.go` | 3 | Spot settings |
| `ginie_patterns.go` | 2 | Pattern settings |
| `handlers_mode.go` | 1 | Mode allocation |
| `handlers_futures.go` | 1 | Settings manager |
| `user_autopilot_manager.go` | 1 | Settings loading |

### Issues with Current State
1. **File I/O on every call**: Settings read from disk repeatedly
2. **No multi-user support**: Single file shared across all users
3. **Race conditions**: Concurrent writes can corrupt the file
4. **Mixed concerns**: Config, runtime state, and cache all in one file
5. **No audit trail**: Changes not tracked in database

---

## Solution: Phased Migration to DB+Redis

### Phase 1: Symbol Settings Migration (HIGH PRIORITY)
Migrate symbol-specific settings (category, confidence, size multiplier) to database.

**Methods to migrate:**
- `GetSymbolSettings(symbol)` → `repo.GetUserSymbolSettings(ctx, userID, symbol)`
- `UpdateSymbolSettings(symbol, settings)` → `repo.UpsertUserSymbolSettings(ctx, settings)`
- `GetEffectiveConfidence(symbol, base)` → Use cached symbol settings
- `GetEffectivePositionSize(symbol, max)` → Use cached symbol settings
- `IsSymbolEnabled(symbol)` → Already partially migrated in 9.11

**Files to update:**
- `ginie_autopilot.go` (15+ calls)
- `handlers_futures_autopilot.go` (5+ calls)
- `ginie_analyzer.go` (5+ calls)

### Phase 2: Mode Circuit Breaker Stats Migration
Migrate per-mode circuit breaker statistics to database.

**Methods to migrate:**
- `GetAllModeCircuitBreakerStats()` → `repo.GetUserModeCircuitBreakerStats()`
- `SaveModeCircuitBreakerStats(mode, stats)` → `repo.SaveUserModeCircuitBreakerStats()`
- `CheckAndResetTimeBasedCounters()` → DB-based reset

**Files to update:**
- `ginie_autopilot.go` (5+ calls)

### Phase 3: Confluence Config Migration
Migrate coin-specific confluence configuration.

**Methods to migrate:**
- `GetCoinConfluenceConfig(symbol)` → `repo.GetUserCoinConfluenceConfig()`
- `UpdateCoinConfluenceConfig(symbol, config)` → `repo.UpsertUserCoinConfluenceConfig()`

**Files to update:**
- `ginie_analyzer.go` (2+ calls)

### Phase 4: Morning Auto-Block Config Migration
Migrate morning auto-block scheduler configuration.

**Methods to migrate:**
- `LoadSettingsFromDB()` → Already uses DB, but still references file for defaults
- Morning auto-block hour/minute settings

**Files to update:**
- `handlers_ginie.go` (4+ calls)
- `ginie_autopilot.go` (2+ calls)

### Phase 5: Legacy Settings Cleanup
Remove remaining file-based operations.

**Methods to deprecate:**
- `LoadSettings()` → Remove file loading entirely
- `SaveSettings()` → Remove file saving entirely
- `GetDefaultSettings()` → Use `default-settings.json` via cache

**Files to update:**
- `settings.go` - Remove file I/O
- `handlers_settings_defaults.go` - Use DB+cache
- `futures_controller.go` - Migrate or remove

### Phase 6: File Removal
1. Add `autopilot_settings.json` to `.gitignore`
2. Rename to `autopilot_settings.json.deprecated`
3. Remove all file references from code
4. Update documentation

---

## Technical Specification

### Database Tables (Already Exist)
Most tables already exist from previous migrations:
- `user_symbol_settings` - Symbol-specific settings
- `user_mode_configs` - Mode configurations
- `user_mode_circuit_breaker_stats` - Circuit breaker stats (migration 019)
- `user_ginie_settings` - Global Ginie settings (migration 017)
- `user_blocked_symbols` - Symbol blocking (migration 042)

### New Table Needed: Coin Confluence Config

```sql
CREATE TABLE user_coin_confluence_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(36) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    min_confluence_score DECIMAL(5,2) DEFAULT 0,
    weight_rsi DECIMAL(3,2) DEFAULT 1.0,
    weight_macd DECIMAL(3,2) DEFAULT 1.0,
    weight_ema DECIMAL(3,2) DEFAULT 1.0,
    weight_volume DECIMAL(3,2) DEFAULT 1.0,
    weight_trend DECIMAL(3,2) DEFAULT 1.0,
    custom_weights JSONB,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);
```

### Redis Cache Keys
Use existing cache patterns from `settings_cache_service.go`:
- `symbol_settings:{userID}:{symbol}` - Symbol settings
- `confluence_config:{userID}:{symbol}` - Confluence config
- `circuit_breaker_stats:{userID}:{mode}` - CB stats

---

## Acceptance Criteria

### AC9.12.1: Symbol Settings Migrated
- [x] `GetSymbolSettings()` uses DB+cache
- [x] `UpdateSymbolSettings()` uses DB with cache invalidation
- [x] `GetEffectiveConfidence()` uses cached data
- [x] `GetEffectivePositionSize()` uses cached data

### AC9.12.2: Circuit Breaker Stats Migrated
- [x] Stats loaded from DB on startup
- [x] Stats saved to DB on update
- [x] Time-based reset uses DB timestamps

### AC9.12.3: Confluence Config Migrated
- [ ] New migration creates table
- [ ] Repository methods implemented
- [ ] Cache layer added

### AC9.12.4: Morning Auto-Block Migrated
- [ ] Config stored in `user_ginie_settings`
- [ ] Handlers use DB directly

### AC9.12.5: File Removed
- [ ] No code references `autopilot_settings.json`
- [ ] File added to `.gitignore`
- [ ] File renamed to `.deprecated`

### AC9.12.6: Multi-User Support
- [ ] All settings isolated by userID
- [ ] Each user has independent configuration

---

## Implementation Plan

### Task 1: Symbol Settings (Phase 1)
1. Update `ginie_autopilot.go` symbol settings calls
2. Update `handlers_futures_autopilot.go` symbol handlers
3. Update `ginie_analyzer.go` symbol lookups
4. Add cache warming on startup

### Task 2: Circuit Breaker Stats (Phase 2)
1. Verify migration 019 is applied
2. Update `ginie_autopilot.go` CB stats calls
3. Add cache for CB stats

### Task 3: Confluence Config (Phase 3)
1. Create migration for `user_coin_confluence_config`
2. Add repository methods
3. Update `ginie_analyzer.go` confluence calls

### Task 4: Morning Auto-Block (Phase 4)
1. Move config to `user_ginie_settings` table
2. Update handlers to use DB
3. Update scheduler to use DB

### Task 5: Legacy Cleanup (Phase 5)
1. Remove `LoadSettings()` file operations
2. Remove `SaveSettings()` file operations
3. Update `handlers_settings_defaults.go`
4. Update `futures_controller.go`

### Task 6: File Removal (Phase 6)
1. Add to `.gitignore`
2. Rename file
3. Remove from `prod-release.sh` backup list
4. Update documentation

---

## Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| Data loss during migration | HIGH | Backup file before changes, migration script reads existing values |
| Performance regression | MEDIUM | Use Redis cache for hot paths |
| Breaking existing users | HIGH | Graceful fallback to defaults if DB empty |
| Large PR size | MEDIUM | Split into phases, merge incrementally |

---

## Estimated Effort by Phase

| Phase | Effort | Files | Priority |
|-------|--------|-------|----------|
| Phase 1: Symbol Settings | 1 day | 3 files | HIGH |
| Phase 2: Circuit Breaker | 0.5 day | 1 file | MEDIUM |
| Phase 3: Confluence Config | 0.5 day | 2 files | MEDIUM |
| Phase 4: Morning Auto-Block | 0.5 day | 2 files | LOW |
| Phase 5: Legacy Cleanup | 1 day | 4 files | LOW |
| Phase 6: File Removal | 0.5 day | Scripts | LOW |
| **Total** | **4 days** | **12+ files** | |

---

## Notes

- This story can be split into multiple smaller stories if needed
- Phase 1 (Symbol Settings) is the highest priority as it affects trading decisions
- `futures_controller.go` is being deprecated in Epic 9, so those usages may be removed naturally
- Some usages in `handlers_settings_defaults.go` may be for admin/debug endpoints that can be deprecated

---

## References

- Story 9.10: Migrate SettingsManager to DB+Redis (COMPLETE)
- Story 9.11: Migrate Symbol Blocking to Database (COMPLETE)
- Story 9.4: Settings Consolidation Single Source (COMPLETE)
- Epic 6: Database-First Architecture
