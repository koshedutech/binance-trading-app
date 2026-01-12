# Story 9.4: Settings Consolidation - Single Source of Truth

**Story ID:** SETTINGS-9.4
**Epic:** Epic 9 - Entry Signal Quality Improvements & Settings Cleanup
**Priority:** P1 (High - Architectural Cleanup)
**Estimated Effort:** 20-28 hours
**Author:** Claude Code Agent (Party Mode Analysis)
**Status:** Ready for Implementation
**Created:** 2026-01-12
**Reviewed:** 2026-01-12 (Mary, Winston, Murat - All Approved after revisions)
**Depends On:** Story 9.3 (ADX Blocking)

---

## Team Review Summary

| Reviewer | Role | Initial Verdict | Final Verdict |
|----------|------|-----------------|---------------|
| Mary | Business Analyst | NEEDS_CHANGES | APPROVED |
| Winston | Architect | NEEDS_CHANGES | APPROVED |
| Murat | Test Architect | NEEDS_CHANGES | APPROVED |

**Key feedback incorporated:**
- Added explicit migration rules for existing user customizations
- Added database migration SQL
- Defined runtime state file schema
- Added missing file references
- Added test file locations
- Added rollback verification commands
- Expanded acceptance criteria

---

## Problem Statement

### Current State (PROBLEMATIC)

The codebase has **TWO settings files** with overlapping but inconsistent configurations:

| File | Lines | Purpose | Issues |
|------|-------|---------|--------|
| `default-settings.json` | 1024 | Master defaults for UI & new users | Missing 50+ settings |
| `autopilot_settings.json` | 1600+ | Runtime extended settings | Has settings NOT in default-settings |

**Specific Problems:**

1. **Duplicate scalp_reentry**: Exists as BOTH a mode (lines 370-527) AND optimization config (lines 950-1004)
2. **Version mismatch**: `early_warning.start_after_minutes` is 10 in default, 1 in autopilot
3. **Missing settings in default-settings.json**:
   - Symbol-specific settings (category boosts)
   - Extended early warning parameters (8 extra fields)
   - Morning auto-block settings
   - Breakout detection settings
   - Trading fees configuration
   - Mode LLM settings
   - Adaptive AI config
   - Safety settings per mode
   - Rebuy trend confirmation (Option C - new)
4. **Early Warning is GLOBAL**: Should be mode-specific
5. **Trading fees HARDCODED**: Should be configurable

### Target State

**SINGLE SOURCE OF TRUTH**: `default-settings.json` contains ALL settings, properly organized.
**Runtime State Separated**: `ginie_runtime_state.json` for dynamic/runtime-only data.

---

## Goals

1. Remove duplicate `mode_configs.scalp_reentry` (it's NOT a mode)
2. Add missing settings to `default-settings.json`
3. Make early_warning mode-specific
4. Add Option C: trend-based rebuy confirmation
5. Reduce rebuy exposure (50% instead of 80%, 1 attempt instead of 3)
6. Add configurable trading fees
7. Define runtime state file for dynamic data
8. Update code, database, API, and UI accordingly

---

## Phased Implementation

### Phase 1: Settings-Only Changes (LOW RISK)
*Estimated: 4 hours*
*Dependencies: None*

No code changes. Only `default-settings.json` modifications.

#### Task 1.1: Update scalp_reentry_config
```json
"scalp_reentry_config": {
  "reentry_percent": 50,                          // Was: 80
  "max_reentry_attempts": 1,                      // Was: 3
  "reentry_require_trend_confirmation": true,     // NEW
  "reentry_min_adx": 20,                          // NEW (valid range: 15-50)
  "tp1_percent": 0.5,                             // Was: 0.35 (increase for fee coverage)
  ...
}
```

#### Task 1.2: Add missing extended early_warning fields
```json
"early_warning": {
  // Existing 6 fields...
  "tighten_sl_on_warning": true,                  // NEW
  "min_confidence": 0.7,                          // NEW
  "max_llm_calls_per_pos": 3,                     // NEW
  "close_min_hold_mins": 15,                      // NEW
  "close_min_confidence": 0.85,                   // NEW
  "close_require_consecutive": 2,                 // NEW
  "close_sl_proximity_pct": 50                    // NEW
}
```

#### Task 1.3: Add trading fees configuration (NEW GROUP - Group 10)
```json
"trading_fees": {
  "taker_fee_percent": 0.04,
  "maker_fee_percent": 0.02,
  "vip_level": 0
}
```
**Note:** `auto_detect_from_binance` deferred to future enhancement.

#### Task 1.4: Define runtime state file schema
Create `ginie_runtime_state.json` for dynamic data:
```json
{
  "ginie_total_pnl": 0,
  "ginie_daily_pnl": 0,
  "ginie_total_trades": 0,
  "ginie_winning_trades": 0,
  "mode_circuit_breaker_stats": {},
  "symbol_runtime_state": {
    "BTCUSDT": {
      "blocked_until": null,
      "total_trades": 0,
      "winning_trades": 0,
      "total_pnl": 0
    }
  }
}
```

---

### Phase 2: Remove Duplicate scalp_reentry Mode (MEDIUM RISK)
*Estimated: 6 hours*
*Dependencies: Phase 1*

#### Task 2.1: Remove mode_configs.scalp_reentry (lines 370-527)
- Delete entire `"scalp_reentry": {...}` section from `mode_configs`
- Keep `scalp_reentry_config` as the ONLY source

#### Task 2.2: Update _settings_risk_index
- Remove `"mode_configs.scalp_reentry.enabled"` from high_risk_settings

#### Task 2.3: Code changes
Files to modify:
- `internal/api/handlers_ginie.go` (line 1826 - has ModeConfigs["scalp_reentry"] reference)
- `internal/autopilot/settings.go` - Update GetModeConfig() to return nil for scalp_reentry
- `internal/database/db_user_mode_config_migration.go` (line 18 - CHECK constraint)

#### Task 2.4: Database migration
```sql
-- Migration UP: Handle scalp_reentry mode rows
-- Option 1: Delete deprecated rows (if using scalp_reentry_config table)
DELETE FROM user_mode_configs WHERE mode_name = 'scalp_reentry';

-- Option 2: Or mark as deprecated (safer)
UPDATE user_mode_configs
SET config_json = jsonb_set(config_json, '{deprecated}', 'true')
WHERE mode_name = 'scalp_reentry';

-- Update CHECK constraint (if removing scalp_reentry completely)
ALTER TABLE user_mode_configs
DROP CONSTRAINT IF EXISTS user_mode_configs_mode_name_check;

ALTER TABLE user_mode_configs
ADD CONSTRAINT user_mode_configs_mode_name_check
CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position'));

-- Migration DOWN: Restore scalp_reentry
ALTER TABLE user_mode_configs
DROP CONSTRAINT IF EXISTS user_mode_configs_mode_name_check;

ALTER TABLE user_mode_configs
ADD CONSTRAINT user_mode_configs_mode_name_check
CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position', 'scalp_reentry'));
```

---

### Phase 3: Implement Option C - Trend-Based Rebuy (MEDIUM RISK)
*Estimated: 6 hours*
*Dependencies: Phase 1 (can run parallel with Phase 2)*

#### Task 3.1: Add settings (already done in Phase 1)
- `reentry_require_trend_confirmation`
- `reentry_min_adx`

#### Task 3.2: Code changes in scalp_reentry_logic.go
```go
func (sr *ScalpReentryManager) shouldExecuteReentry(symbol string, currentPrice float64) bool {
    // Existing checks...

    // NEW: Option C - Trend confirmation
    if sr.config.ReentryRequireTrendConfirmation {
        adx := sr.getADXValue(symbol)
        if adx < sr.config.ReentryMinADX {
            sr.logger.Info("Rebuy blocked - ADX below threshold",
                "symbol", symbol,
                "adx", adx,
                "threshold", sr.config.ReentryMinADX)
            return false
        }
    }

    return true
}
```

#### Task 3.3: Update settings.go struct
```go
type ScalpReentryConfig struct {
    // Existing fields...
    ReentryRequireTrendConfirmation bool    `json:"reentry_require_trend_confirmation"`
    ReentryMinADX                   float64 `json:"reentry_min_adx"`
}

// Validation
func (c *ScalpReentryConfig) Validate() error {
    if c.ReentryMinADX < 15 || c.ReentryMinADX > 50 {
        return fmt.Errorf("reentry_min_adx must be between 15 and 50, got: %.1f", c.ReentryMinADX)
    }
    return nil
}
```

#### Task 3.4: Update DefaultScalpReentryConfig() function
Location: `internal/autopilot/scalp_reentry_types.go` (lines 450-499)
```go
func DefaultScalpReentryConfig() *ScalpReentryConfig {
    return &ScalpReentryConfig{
        // ... existing defaults ...
        ReentryRequireTrendConfirmation: true,
        ReentryMinADX:                   20.0,
    }
}
```

---

### Phase 4: Make Early Warning Mode-Specific (HIGHER RISK)
*Estimated: 8 hours*
*Dependencies: Phase 2 (must complete first)*

#### Task 4.1: Add early_warning to each mode config
```json
"mode_configs": {
  "scalp": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 10,
      "min_loss_percent": 1.0,
      "check_interval_secs": 30
    }
  },
  "swing": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 15,
      "min_loss_percent": 2.0,
      "check_interval_secs": 60
    }
  },
  "position": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 30,
      "min_loss_percent": 3.0,
      "check_interval_secs": 120
    }
  },
  "ultra_fast": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 2,
      "min_loss_percent": 0.5,
      "check_interval_secs": 15
    }
  }
}
```

#### Task 4.2: Keep global early_warning as fallback
- Global config used if mode-specific not defined
- Hierarchy: Mode-specific > Global > Code defaults

#### Task 4.3: Code changes in ginie_autopilot.go
```go
func (ga *GinieAutopilot) getEarlyWarningConfig(mode GinieTradingMode) *EarlyWarningConfig {
    // 1. Try mode-specific config
    if modeConfig, ok := ga.modeConfigs[mode]; ok {
        if modeConfig.EarlyWarning != nil {
            return modeConfig.EarlyWarning
        }
    }
    // 2. Fall back to global
    return ga.globalEarlyWarning
}
```

#### Task 4.4: Database migration - Add early_warning JSONB column
```sql
-- Migration UP
ALTER TABLE user_mode_configs
ADD COLUMN IF NOT EXISTS early_warning JSONB DEFAULT NULL;

-- Add to user_early_warning table: 7 new columns
ALTER TABLE user_early_warning
ADD COLUMN IF NOT EXISTS tighten_sl_on_warning BOOLEAN DEFAULT true,
ADD COLUMN IF NOT EXISTS min_confidence NUMERIC(5,4) DEFAULT 0.7,
ADD COLUMN IF NOT EXISTS max_llm_calls_per_pos INTEGER DEFAULT 3,
ADD COLUMN IF NOT EXISTS close_min_hold_mins INTEGER DEFAULT 15,
ADD COLUMN IF NOT EXISTS close_min_confidence NUMERIC(5,4) DEFAULT 0.85,
ADD COLUMN IF NOT EXISTS close_require_consecutive INTEGER DEFAULT 2,
ADD COLUMN IF NOT EXISTS close_sl_proximity_pct INTEGER DEFAULT 50;

-- Migration DOWN
ALTER TABLE user_mode_configs DROP COLUMN IF EXISTS early_warning;

ALTER TABLE user_early_warning
DROP COLUMN IF EXISTS tighten_sl_on_warning,
DROP COLUMN IF EXISTS min_confidence,
DROP COLUMN IF EXISTS max_llm_calls_per_pos,
DROP COLUMN IF EXISTS close_min_hold_mins,
DROP COLUMN IF EXISTS close_min_confidence,
DROP COLUMN IF EXISTS close_require_consecutive,
DROP COLUMN IF EXISTS close_sl_proximity_pct;
```

#### Task 4.5: API changes
- Update mode config GET/POST to include early_warning
- Update "Load Defaults" to copy mode-specific early_warning

#### Task 4.6: UI changes
- Display early_warning settings under each mode in ResetSettings
- Admin can edit per-mode early_warning
- Users can view and restore

---

### Phase 5: Full Settings Sync & Cleanup (MEDIUM RISK)
*Estimated: 4 hours*
*Dependencies: Phases 1-4*

#### Task 5.1: Audit autopilot_settings.json - Complete List
Settings to migrate (enumerated):
- [ ] `symbol_settings` (category, custom_roi, blocked_until → runtime)
- [ ] `category_confidence_boost` (best, good, neutral, poor, worst)
- [ ] `category_size_multiplier`
- [ ] `morning_auto_block_enabled`, `morning_auto_block_hour`
- [ ] `breakout_config` (all 11 settings)
- [ ] `safety_ultra_fast`, `safety_scalp`, `safety_swing`, `safety_position`
- [ ] `mode_allocation` (max_positions per mode)
- [ ] `mode_llm_settings` (per-mode LLM weights)
- [ ] `adaptive_ai_config`
- [ ] `coin_confluence_configs`

#### Task 5.2: Add missing settings to default-settings.json
Group these appropriately in the 9+ groups structure.

#### Task 5.3: Update LoadDefaultSettings()
- Ensure all new fields are loaded
- Update struct definitions

#### Task 5.4: Update UI components
- ResetSettings page displays all new settings
- Proper grouping and labels

#### Task 5.5: Separate runtime state
- Move dynamic data to `ginie_runtime_state.json`
- Update code to read/write runtime state separately

#### Task 5.6: Deprecate autopilot_settings.json
- Remove settings that now live in default-settings.json
- Keep only until migration verified complete

---

## Acceptance Criteria

### AC9.4.1: Rebuy Exposure Reduced
- [ ] `reentry_percent` changed from 80 to 50
- [ ] `max_reentry_attempts` changed from 3 to 1
- [ ] **Migration rule:** Changes apply to users with default values (80/3); custom values preserved
- [ ] Changes reflected in database for existing users

### AC9.4.2: Option C - Trend-Based Rebuy
- [ ] `reentry_require_trend_confirmation` setting added (default: true)
- [ ] `reentry_min_adx` setting added (default: 20, valid range: 15-50)
- [ ] Code checks ADX before allowing rebuy
- [ ] Rebuy blocked if ADX < threshold (logged)
- [ ] **Boundary test:** ADX exactly at 20.0 allows rebuy (>= operator)
- [ ] **Error handling:** If ADX retrieval fails, rebuy proceeds (fail-open)

### AC9.4.3: Duplicate scalp_reentry Removed
- [ ] `mode_configs.scalp_reentry` deleted from default-settings.json
- [ ] Only `scalp_reentry_config` exists (optimization, not mode)
- [ ] `handlers_ginie.go` line 1826 updated (no ModeConfigs["scalp_reentry"])
- [ ] Code handles scalp_reentry as optimization, not mode
- [ ] Database CHECK constraint updated
- [ ] Database migration handles existing rows

### AC9.4.4: Early Warning Mode-Specific
- [ ] Each mode has `early_warning` section in config
- [ ] Scalp: min_loss_percent = 1.0
- [ ] Swing: min_loss_percent = 2.0
- [ ] Position: min_loss_percent = 3.0
- [ ] Ultra-fast: min_loss_percent = 0.5
- [ ] Code reads mode-specific config first, then global fallback
- [ ] **Backward compatibility:** Existing positions continue using their original config
- [ ] **7 new columns** added to user_early_warning table

### AC9.4.5: Trading Fees Configurable
- [ ] `trading_fees` group added to default-settings.json
- [ ] Code uses configurable fees instead of hardcoded constants
- [ ] UI displays trading fees in ResetSettings

### AC9.4.6: UI Updated
- [ ] Admin: Can edit all new settings
- [ ] User: Can view all settings, restore to defaults
- [ ] All 10 groups displayed properly

### AC9.4.7: Full Sync Complete - Enumerated Settings
- [ ] `symbol_settings` structure defined (static in defaults, dynamic in runtime)
- [ ] `category_confidence_boost` added
- [ ] `category_size_multiplier` added
- [ ] `morning_auto_block_*` settings added
- [ ] `breakout_config` added (11 settings)
- [ ] `safety_*` per mode added
- [ ] `mode_allocation` added
- [ ] `mode_llm_settings` added
- [ ] `adaptive_ai_config` added
- [ ] `ginie_runtime_state.json` created for dynamic data
- [ ] default-settings.json is SINGLE SOURCE OF TRUTH
- [ ] autopilot_settings.json deprecated

---

## Impact Analysis

### Files to Modify

| File | Phase | Changes |
|------|-------|---------|
| `default-settings.json` | 1,2,4,5 | Add settings, remove duplicate scalp_reentry, add mode early_warning |
| `internal/autopilot/settings.go` | 3,4 | Update structs, add validation |
| `internal/autopilot/scalp_reentry_logic.go` | 3 | Add Option C logic |
| `internal/autopilot/scalp_reentry_types.go` | 3 | Update DefaultScalpReentryConfig() |
| `internal/autopilot/ginie_autopilot.go` | 4 | Mode-specific early warning |
| `internal/api/handlers_ginie.go` | 2 | Remove ModeConfigs["scalp_reentry"] reference (line 1826) |
| `internal/database/models_user_settings.go` | 4 | New fields for early_warning |
| `internal/database/repository_user_early_warning.go` | 4 | Handle new columns |
| `internal/database/db_user_mode_config_migration.go` | 2 | Update CHECK constraint |
| `internal/api/handlers_settings_defaults.go` | 4,5 | Update handlers |
| `web/src/pages/ResetSettings.tsx` | 5 | Display new settings |

### Database Tables Affected

| Table | Phase | Change |
|-------|-------|--------|
| `user_mode_configs` | 2,4 | Update CHECK constraint, add early_warning JSONB |
| `user_early_warning` | 4 | Add 7 new columns |
| `user_scalp_reentry_config` | 3 | Add 2 new fields |

---

## Testing Strategy

### Test Files to Update

| File | Phase | Tests to Add |
|------|-------|--------------|
| `internal/autopilot/scalp_reentry_test.go` | 3 | Option C ADX tests |
| `internal/autopilot/settings_test.go` | 1,3,4 | New field validation tests |
| `internal/database/repository_*_test.go` | 2,4 | Migration tests |

### Phase 1 Testing (Settings-only)
- [ ] Verify default-settings.json is valid JSON
- [ ] Verify LoadDefaultSettings() loads all new fields
- [ ] Check new user creation uses new defaults
- [ ] Schema validation test

### Phase 2 Testing (Remove duplicate)
- [ ] Verify no code references mode_configs.scalp_reentry
- [ ] Verify scalp_reentry_config is used correctly
- [ ] Test scalp mode with reentry optimization
- [ ] **DB Migration test:** UP migration on empty DB
- [ ] **DB Migration test:** UP migration with existing user data
- [ ] **DB Migration test:** DOWN migration verified
- [ ] **Idempotency test:** Run migration twice without error

### Phase 3 Testing (Option C)
- [ ] Test rebuy when ADX >= 20 (should work)
- [ ] Test rebuy when ADX < 20 (should be blocked)
- [ ] **Boundary test:** ADX exactly 20.0 (should work)
- [ ] **Error test:** ADX retrieval timeout (fail-open)
- [ ] Verify logs show "Rebuy blocked - ADX below threshold"

### Phase 4 Testing (Mode-specific early warning)
- [ ] Test scalp position: early warning at -1.0%
- [ ] Test swing position: early warning at -2.0%
- [ ] Test position trade: early warning at -3.0%
- [ ] **Fallback test:** Mode-specific missing → uses global
- [ ] **Fallback test:** Global missing → uses code defaults
- [ ] **DB Migration test:** UP and DOWN verified
- [ ] **Concurrent access test:** Multiple goroutines reading settings

### Phase 5 Testing (Full sync)
- [ ] Verify all settings display in UI
- [ ] Verify admin can edit
- [ ] Verify user can view and restore
- [ ] Test with new user creation
- [ ] Verify runtime state file created and populated

### Definition of Done - Test Coverage
- [ ] Unit test coverage >= 80% for modified files
- [ ] All existing `scalp_reentry_test.go` tests pass
- [ ] Database migration tested (UP and DOWN)
- [ ] All new settings appear in `/api/settings/defaults` response
- [ ] LoadDefaultSettings() benchmark shows no performance regression

---

## Rollback Instructions

### Phase 1 Rollback
```bash
git checkout HEAD~1 -- default-settings.json
docker restart binance-trading-bot-dev
```

### Phase 2 Rollback
```bash
# 1. Revert code changes
git revert <phase-2-commit-hash>

# 2. Rollback database
docker exec binance-bot-postgres-dev psql -U trading_bot -d trading_bot -c "
ALTER TABLE user_mode_configs
DROP CONSTRAINT IF EXISTS user_mode_configs_mode_name_check;

ALTER TABLE user_mode_configs
ADD CONSTRAINT user_mode_configs_mode_name_check
CHECK (mode_name IN ('ultra_fast', 'scalp', 'swing', 'position', 'scalp_reentry'));
"

# 3. Restart container
docker restart binance-trading-bot-dev

# 4. Verify rollback
curl http://localhost:8094/health
curl http://localhost:8094/api/settings/defaults | jq '.mode_configs.scalp_reentry'
```

### Phase 3 Rollback
```bash
git revert <phase-3-commit-hash>
docker restart binance-trading-bot-dev

# Verify: Option C should no longer block rebuys
docker logs binance-trading-bot-dev 2>&1 | grep -v "Rebuy blocked - ADX"
```

### Phase 4 Rollback
```bash
# 1. Revert code changes
git revert <phase-4-commit-hash>

# 2. Rollback database
docker exec binance-bot-postgres-dev psql -U trading_bot -d trading_bot -c "
ALTER TABLE user_mode_configs DROP COLUMN IF EXISTS early_warning;

ALTER TABLE user_early_warning
DROP COLUMN IF EXISTS tighten_sl_on_warning,
DROP COLUMN IF EXISTS min_confidence,
DROP COLUMN IF EXISTS max_llm_calls_per_pos,
DROP COLUMN IF EXISTS close_min_hold_mins,
DROP COLUMN IF EXISTS close_min_confidence,
DROP COLUMN IF EXISTS close_require_consecutive,
DROP COLUMN IF EXISTS close_sl_proximity_pct;
"

# 3. Restart container
docker restart binance-trading-bot-dev

# 4. Verify rollback
curl http://localhost:8094/health
```

### Rollback Verification Checklist
- [ ] `curl http://localhost:8094/health` returns 200
- [ ] No orphaned database records
- [ ] Existing positions continue trading
- [ ] UI displays correctly
- [ ] No error logs related to settings

---

## Risk Assessment

| Phase | Risk Level | Risks | Mitigation |
|-------|------------|-------|------------|
| Phase 1 | LOW | Invalid JSON | Schema validation before commit |
| Phase 2 | MEDIUM | Code references missed; DB migration issues | Grep for all references; test migration UP/DOWN |
| Phase 3 | MEDIUM | ADX API failures; boundary conditions | Fail-open on errors; boundary tests |
| Phase 4 | HIGHER | Existing positions affected; API breaking | Backward compatibility; keep global fallback |
| Phase 5 | MEDIUM | Data loss during deprecation | Verify runtime state file works before deprecating |

### Additional Risks Identified by Review
| Risk | Impact | Mitigation |
|------|--------|------------|
| Data loss during migration | HIGH | Verify runtime state file before deprecating autopilot_settings.json |
| API breaking changes | MEDIUM | Backward-compatible field handling; test all endpoints |
| Concurrent settings access | MEDIUM | Lock mechanism for settings writes if needed |

---

## Coexistence Strategy During Migration

During Phases 2-5, both files exist:

| Setting Type | Read From | Write To |
|--------------|-----------|----------|
| Default configs | `default-settings.json` | `default-settings.json` |
| User overrides | Database | Database |
| Runtime state (PnL, stats) | `autopilot_settings.json` → `ginie_runtime_state.json` | New runtime file |

**Migration Flag:** Add `settings_version: 2` to indicate new format.

---

## Definition of Done

- [ ] Phase 1: Settings changes committed and tested
- [ ] Phase 2: Duplicate scalp_reentry removed, code updated, DB migrated
- [ ] Phase 3: Option C implemented and working (with tests)
- [ ] Phase 4: Early warning mode-specific working (with DB migration)
- [ ] Phase 5: Full sync complete, single source of truth established
- [ ] All acceptance criteria met
- [ ] All test files updated with new tests
- [ ] Database migrations tested (UP and DOWN)
- [ ] UI displays all settings correctly
- [ ] Admin can edit, users can view/restore
- [ ] Container restarted and tested
- [ ] No regressions in trading functionality
- [ ] All CI checks pass (lint, test, build)

---

## Related

- **Previous Story:** Story 9.3 - ADX Blocking Condition
- **Analysis Source:** Party Mode multi-agent analysis (2026-01-12)
- **Discussion:** Loophole #3 - Rebuy at Losing Trade (Option C selected)
- **Settings Architecture:** 10 groups in default-settings.json (was 9, adding trading_fees)
- **Review:** Mary (Analyst), Winston (Architect), Murat (Test Architect) - All Approved
