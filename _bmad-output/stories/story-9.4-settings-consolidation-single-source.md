# Story 9.4: Settings Consolidation - Single Source of Truth

**Story ID:** SETTINGS-9.4
**Epic:** Epic 9 - Entry Signal Quality Improvements & Settings Cleanup
**Priority:** P1 (High - Architectural Cleanup)
**Estimated Effort:** 16-24 hours
**Author:** Claude Code Agent (Party Mode Analysis)
**Status:** Draft
**Created:** 2026-01-12
**Depends On:** Story 9.3 (ADX Blocking)

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
   - Rebuy trend confirmation (Option C - new)
4. **Early Warning is GLOBAL**: Should be mode-specific
5. **Trading fees HARDCODED**: Should be configurable

### Target State

**SINGLE SOURCE OF TRUTH**: `default-settings.json` contains ALL settings, properly organized.

---

## Goals

1. Remove duplicate `mode_configs.scalp_reentry` (it's NOT a mode)
2. Add missing settings to `default-settings.json`
3. Make early_warning mode-specific
4. Add Option C: trend-based rebuy confirmation
5. Reduce rebuy exposure (50% instead of 80%, 1 attempt instead of 3)
6. Add configurable trading fees
7. Update code, database, API, and UI accordingly

---

## Phased Implementation

### Phase 1: Settings-Only Changes (LOW RISK)
*Estimated: 4 hours*

No code changes. Only `default-settings.json` modifications.

#### Task 1.1: Update scalp_reentry_config
```json
"scalp_reentry_config": {
  "reentry_percent": 50,                          // Was: 80
  "max_reentry_attempts": 1,                      // Was: 3
  "reentry_require_trend_confirmation": true,     // NEW
  "reentry_min_adx": 20,                          // NEW
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

#### Task 1.3: Add trading fees configuration (NEW GROUP)
```json
"trading_fees": {
  "taker_fee_percent": 0.04,
  "maker_fee_percent": 0.02,
  "vip_level": 0,
  "auto_detect_from_binance": false
}
```

---

### Phase 2: Remove Duplicate scalp_reentry Mode (MEDIUM RISK)
*Estimated: 4 hours*

#### Task 2.1: Remove mode_configs.scalp_reentry (lines 370-527)
- Delete entire `"scalp_reentry": {...}` section from `mode_configs`
- Keep `scalp_reentry_config` as the ONLY source

#### Task 2.2: Update _settings_risk_index
- Remove `"mode_configs.scalp_reentry.enabled"` from high_risk_settings

#### Task 2.3: Code changes
- Update any code that references `ModeConfigs["scalp_reentry"]`
- Ensure scalp_reentry uses `ScalpReentryConfig` only
- Update `GetModeConfig()` to return nil for scalp_reentry (it's not a mode)

#### Task 2.4: Database changes
- If `user_mode_configs` has scalp_reentry rows, migrate to `user_scalp_reentry_config`
- Or mark scalp_reentry mode rows as deprecated

---

### Phase 3: Implement Option C - Trend-Based Rebuy (MEDIUM RISK)
*Estimated: 6 hours*

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
```

---

### Phase 4: Make Early Warning Mode-Specific (HIGHER RISK)
*Estimated: 6 hours*

#### Task 4.1: Add early_warning to each mode config
```json
"mode_configs": {
  "scalp": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 10,
      "min_loss_percent": 1.0
    }
  },
  "swing": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 15,
      "min_loss_percent": 2.0
    }
  },
  "position": {
    "early_warning": {
      "enabled": true,
      "start_after_minutes": 30,
      "min_loss_percent": 3.0
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

#### Task 4.4: Database migration
- Add `early_warning` JSONB column to `user_mode_configs` table
- Or create new `user_mode_early_warning` table

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

#### Task 5.1: Audit autopilot_settings.json
- List ALL settings not in default-settings.json
- Categorize: Must add / Deprecated / Runtime-only

#### Task 5.2: Add missing settings to default-settings.json
- Symbol settings (category boosts, size multipliers)
- Morning auto-block settings
- Breakout detection settings
- Safety settings per mode
- Any other missing

#### Task 5.3: Update LoadDefaultSettings()
- Ensure all new fields are loaded
- Update struct definitions

#### Task 5.4: Update UI components
- ResetSettings page displays all new settings
- Proper grouping and labels

#### Task 5.5: Deprecate autopilot_settings.json
- Move any runtime-only state to separate state file
- Remove settings that now live in default-settings.json
- Eventually delete file when fully migrated

---

## Acceptance Criteria

### AC9.4.1: Rebuy Exposure Reduced
- [ ] `reentry_percent` changed from 80 to 50
- [ ] `max_reentry_attempts` changed from 3 to 1
- [ ] Changes reflected in database for existing users

### AC9.4.2: Option C - Trend-Based Rebuy
- [ ] `reentry_require_trend_confirmation` setting added
- [ ] `reentry_min_adx` setting added (default: 20)
- [ ] Code checks ADX before allowing rebuy
- [ ] Rebuy blocked if ADX < threshold (logged)

### AC9.4.3: Duplicate scalp_reentry Removed
- [ ] `mode_configs.scalp_reentry` deleted from default-settings.json
- [ ] Only `scalp_reentry_config` exists (optimization, not mode)
- [ ] Code handles scalp_reentry as optimization, not mode
- [ ] Database migration handles existing rows

### AC9.4.4: Early Warning Mode-Specific
- [ ] Each mode has `early_warning` section in config
- [ ] Scalp: min_loss_percent = 1.0
- [ ] Swing: min_loss_percent = 2.0
- [ ] Position: min_loss_percent = 3.0
- [ ] Code reads mode-specific config first, then global fallback

### AC9.4.5: Trading Fees Configurable
- [ ] `trading_fees` group added to default-settings.json
- [ ] Code uses configurable fees instead of hardcoded constants
- [ ] UI displays trading fees in ResetSettings

### AC9.4.6: UI Updated
- [ ] Admin: Can edit all new settings
- [ ] User: Can view all settings, restore to defaults
- [ ] All 9+ groups displayed properly

### AC9.4.7: Full Sync Complete
- [ ] All missing settings from autopilot_settings.json added
- [ ] default-settings.json is SINGLE SOURCE OF TRUTH
- [ ] autopilot_settings.json deprecated or removed

---

## Impact Analysis

### Files to Modify

| File | Changes |
|------|---------|
| `default-settings.json` | Add settings, remove duplicate scalp_reentry |
| `internal/autopilot/settings.go` | Update structs, add new fields |
| `internal/autopilot/scalp_reentry_logic.go` | Add Option C logic |
| `internal/autopilot/ginie_autopilot.go` | Mode-specific early warning |
| `internal/database/models_user_settings.go` | New fields |
| `internal/database/repository_*.go` | Update queries |
| `internal/api/handlers_settings_defaults.go` | Update handlers |
| `web/src/pages/ResetSettings.tsx` | Display new settings |

### Database Tables Affected

| Table | Change |
|-------|--------|
| `user_mode_configs` | Add early_warning JSONB or handle scalp_reentry deprecation |
| `user_scalp_reentry_config` | Add new fields (reentry_require_trend_confirmation, reentry_min_adx) |
| `user_trading_fees` | NEW table (or add to user_settings) |

---

## Testing Strategy

### Phase 1 Testing (Settings-only)
- Verify default-settings.json is valid JSON
- Verify LoadDefaultSettings() loads all new fields
- Check new user creation uses new defaults

### Phase 2 Testing (Remove duplicate)
- Verify no code references mode_configs.scalp_reentry
- Verify scalp_reentry_config is used correctly
- Test scalp mode with reentry optimization

### Phase 3 Testing (Option C)
- Test rebuy when ADX >= 20 (should work)
- Test rebuy when ADX < 20 (should be blocked)
- Verify logs show "Rebuy blocked - ADX below threshold"

### Phase 4 Testing (Mode-specific early warning)
- Test scalp position: early warning at -1.0%
- Test swing position: early warning at -2.0%
- Test position trade: early warning at -3.0%
- Verify mode fallback to global if not configured

### Phase 5 Testing (Full sync)
- Verify all settings display in UI
- Verify admin can edit
- Verify user can view and restore
- Test with new user creation

---

## Rollback Instructions

### Phase 1 Rollback
```bash
git checkout HEAD~1 -- default-settings.json
docker restart binance-trading-bot-dev
```

### Phase 2-5 Rollback
Each phase should be committed separately. Rollback with:
```bash
git revert <phase-commit-hash>
docker restart binance-trading-bot-dev
# Run database rollback migration if needed
```

---

## Risk Assessment

| Phase | Risk Level | Mitigation |
|-------|------------|------------|
| Phase 1 | LOW | Settings only, no code changes |
| Phase 2 | MEDIUM | Careful code audit for scalp_reentry references |
| Phase 3 | MEDIUM | Feature flag (reentry_require_trend_confirmation) allows gradual rollout |
| Phase 4 | HIGHER | Keep global fallback, test each mode separately |
| Phase 5 | MEDIUM | Audit before removing autopilot_settings.json |

---

## Definition of Done

- [ ] Phase 1: Settings changes committed and tested
- [ ] Phase 2: Duplicate scalp_reentry removed, code updated
- [ ] Phase 3: Option C implemented and working
- [ ] Phase 4: Early warning mode-specific working
- [ ] Phase 5: Full sync complete, single source of truth established
- [ ] All acceptance criteria met
- [ ] UI displays all settings correctly
- [ ] Admin can edit, users can view/restore
- [ ] Container restarted and tested
- [ ] No regressions in trading functionality

---

## Related

- **Previous Story:** Story 9.3 - ADX Blocking Condition
- **Analysis Source:** Party Mode multi-agent analysis (2026-01-12)
- **Discussion:** Loophole #3 - Rebuy at Losing Trade (Option C selected)
- **Settings Architecture:** 9 groups in default-settings.json
