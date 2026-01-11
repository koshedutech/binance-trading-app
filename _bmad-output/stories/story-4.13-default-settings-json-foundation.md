# Story 4.13: Default Settings JSON Foundation

**Story ID:** SETTINGS-4.13
**Epic:** Epic 4 - Database-First Mode Configuration System
**Priority:** P0 (Critical - Foundation for all user settings)
**Estimated Effort:** 8 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** Ready for Development

---

## Problem Statement

### Current State (BROKEN)

The system has **THREE sources of truth** for configuration settings:

| Source | Location | Issue |
|--------|----------|-------|
| 1. JSON File | `autopilot_settings.json` | 1500+ lines, overwrites database on startup |
| 2. Hardcoded Defaults | `settings.go` | Scattered across 20+ functions |
| 3. Database | `user_mode_configs` table | Gets overwritten by JSON sync |

### Evidence of Problem

```
User disables ultra_fast mode → Saved to database
Server restarts → LoadSettings() reads JSON → syncModeConfigsToRootSettings()
Result: ultra_fast is ENABLED again (from JSON "enabled": true)
```

### Root Cause

`autopilot_settings.json` serves multiple conflicting purposes:
- Default values for new users
- Runtime configuration storage
- Admin template management
- Migration source

### Impact

- User settings don't persist across restarts
- No way to reset to "factory defaults"
- Admin cannot easily manage default templates
- Multi-user settings impossible

---

## User Story

> As a system administrator,
> I want a single `default-settings.json` file that contains ALL system defaults,
> So that new users get consistent starting configurations and any user can reset to defaults.

---

## Solution Overview

Create a **single, comprehensive `default-settings.json`** file that:
1. Contains ALL 500+ configuration settings
2. Is organized into logical groups
3. Includes risk information per setting
4. Is ONLY used for: (a) new user creation, (b) "Load Defaults" feature
5. Is NEVER read at runtime for active trading

```
┌─────────────────────────────────────────────────────────────┐
│                 default-settings.json                        │
│         (Template Only - Never Used at Runtime)              │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │ Mode Configs│ │ Position    │ │ Hedging     │            │
│  │ + risk_info │ │ Optimization│ │ Settings    │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐            │
│  │ Circuit     │ │ LLM/AI      │ │ Early       │            │
│  │ Breaker     │ │ Config      │ │ Warning     │            │
│  └─────────────┘ └─────────────┘ └─────────────┘            │
└─────────────────────────────────────────────────────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
   New User Creation    Load Defaults    Admin Sync
   (Copy to DB)         (Replace from)   (Story 4.15)
```

---

## Acceptance Criteria

### AC4.13.1: Comprehensive Settings File Created
- [ ] File created at `/default-settings.json` in project root
- [ ] Contains ALL 500+ configuration settings from audit
- [ ] Organized into 15+ logical groups (mode_configs, position_optimization, etc.)
- [ ] JSON is valid and properly formatted
- [ ] Includes metadata section with version, last_updated, updated_by

### AC4.13.2: Risk Information Included
- [ ] Each setting group has `_risk_info` object
- [ ] Risk info contains: `impact` (low/medium/high), `recommendation` (string)
- [ ] High-risk settings clearly marked (ultra_fast.enabled, hedging.enabled, etc.)
- [ ] Risk index at root level lists all high/medium/low risk settings

### AC4.13.3: All Setting Groups Present
- [ ] `global_trading` - Global trading mode, risk level, allocations
- [ ] `mode_configs` - All 5 modes with complete sub-configurations
- [ ] `position_optimization` - Averaging, SLTP, hedging, staged entry
- [ ] `circuit_breaker` - Global and per-mode circuit breakers
- [ ] `llm_config` - Global, per-mode, and adaptive AI settings
- [ ] `early_warning` - Position health monitor settings
- [ ] `breakout_detection` - Breakout detection configuration
- [ ] `coin_confluence` - Per-coin confluence defaults
- [ ] `capital_allocation` - Mode capital percentages
- [ ] `symbol_defaults` - Default per-symbol settings
- [ ] `spot_autopilot` - Spot trading configuration
- [ ] `safety_controls` - Per-mode safety controls
- [ ] `mtf_consensus` - Multi-timeframe consensus settings
- [ ] `scalp_reentry_config` - Scalp re-entry specific settings
- [ ] `stale_position_release` - Capital liberation settings

### AC4.13.4: Mode Configs Complete
Each of the 5 modes (ultra_fast, scalp, scalp_reentry, swing, position) must have:
- [ ] `enabled` - boolean (ultra_fast and scalp_reentry default FALSE)
- [ ] `timeframe` - trend, entry, analysis timeframes
- [ ] `confidence` - min, high, ultra thresholds
- [ ] `size` - base_usd, max_usd, leverage, max_positions
- [ ] `circuit_breaker` - mode-specific limits
- [ ] `sltp` - stop loss, take profit, trailing settings
- [ ] `hedging` - allow_hedge, min_confidence, etc.
- [ ] `averaging` - allow_averaging, staged_entry settings
- [ ] `stale_release` - max_hold_duration, stale_zone settings
- [ ] `assignment_criteria` - volatility, confidence bounds
- [ ] `mtf` - multi-timeframe weights and consensus
- [ ] `dynamic_ai_exit` - AI-driven exit settings
- [ ] `reversal_entry` - LIMIT order reversal settings
- [ ] `llm` - per-mode LLM settings
- [ ] `_risk_info` - risk information for this mode

### AC4.13.5: Old JSON File Deprecated
- [ ] `autopilot_settings.json` renamed to `autopilot_settings.json.deprecated`
- [ ] Comment added explaining it's replaced by `default-settings.json`
- [ ] No code references `autopilot_settings.json` for runtime settings

### AC4.13.6: Hardcoded Defaults Removed
- [ ] `DefaultSettings()` in settings.go returns empty/minimal struct
- [ ] `DefaultModeConfigs()` returns empty map
- [ ] `initializeModeConfigs()` does NOT set hardcoded values
- [ ] All defaults come from `default-settings.json` file

---

## Technical Implementation

### Task 1: Create default-settings.json Structure

Create the file with all groups:

```json
{
  "metadata": {
    "version": "1.0.0",
    "schema_version": 1,
    "last_updated": "2026-01-05T00:00:00Z",
    "updated_by": "system",
    "description": "Master default settings for Binance Trading Bot"
  },

  "global_trading": {
    "risk_level": "moderate",
    "max_usd_allocation": 2500,
    "profit_reinvest_percent": 50,
    "profit_reinvest_risk_level": "aggressive",
    "_risk_info": {
      "risk_level": {
        "impact": "high",
        "recommendation": "Use 'moderate' for beginners, 'conservative' for safety"
      },
      "max_usd_allocation": {
        "impact": "high",
        "recommendation": "Start with lower allocation until you understand the system"
      }
    }
  },

  "mode_configs": {
    "ultra_fast": {
      "mode_name": "ultra_fast",
      "enabled": false,
      "description": "1-3 second scalps with high frequency",
      "timeframe": {
        "trend_timeframe": "5m",
        "entry_timeframe": "1m",
        "analysis_timeframe": "5m"
      },
      "confidence": {
        "min_confidence": 40,
        "high_confidence": 80,
        "ultra_confidence": 90
      },
      "size": {
        "base_size_usd": 50,
        "max_size_usd": 200,
        "max_positions": 1,
        "leverage": 10
      },
      // ... complete configuration
      "_risk_info": {
        "enabled": {
          "impact": "high",
          "recommendation": "Keep disabled until experienced. High frequency = high risk."
        }
      }
    },
    "scalp": { /* complete config */ },
    "scalp_reentry": { /* complete config, enabled: false */ },
    "swing": { /* complete config */ },
    "position": { /* complete config */ }
  },

  "position_optimization": { /* complete config */ },
  "circuit_breaker": { /* complete config */ },
  "llm_config": { /* complete config */ },
  "early_warning": { /* complete config */ },
  "breakout_detection": { /* complete config */ },
  "coin_confluence": { /* complete config */ },
  "capital_allocation": { /* complete config */ },
  "symbol_defaults": { /* complete config */ },
  "spot_autopilot": { /* complete config */ },

  "_settings_risk_index": {
    "high_risk_settings": [
      "mode_configs.ultra_fast.enabled",
      "mode_configs.scalp_reentry.enabled",
      "position_optimization.hedging.enabled",
      "global_trading.risk_level=aggressive",
      "circuit_breaker.*.enabled=false"
    ],
    "medium_risk_settings": [
      "mode_configs.*.size.leverage>10",
      "mode_configs.*.confidence.min_confidence<40",
      "llm_config.global.enabled=false"
    ],
    "low_risk_settings": [
      "mode_configs.*.timeframe.*",
      "early_warning.enabled",
      "breakout_detection.enabled"
    ]
  }
}
```

### Task 2: Populate All Settings from Audit

Using the comprehensive audit (500+ settings), populate:
- All 5 mode configs with all 14 sub-sections each
- All global settings
- All per-mode safety controls
- All LLM configurations
- All circuit breaker settings

### Task 3: Add Helper Function to Load Defaults

```go
// internal/autopilot/default_settings.go

package autopilot

import (
    "encoding/json"
    "os"
    "sync"
)

var (
    defaultSettings     *DefaultSettingsFile
    defaultSettingsOnce sync.Once
)

type DefaultSettingsFile struct {
    Metadata             Metadata                       `json:"metadata"`
    GlobalTrading        GlobalTradingDefaults          `json:"global_trading"`
    ModeConfigs          map[string]*ModeFullConfig     `json:"mode_configs"`
    PositionOptimization PositionOptimizationDefaults   `json:"position_optimization"`
    CircuitBreaker       CircuitBreakerDefaults         `json:"circuit_breaker"`
    LLMConfig            LLMConfigDefaults              `json:"llm_config"`
    EarlyWarning         EarlyWarningDefaults           `json:"early_warning"`
    BreakoutDetection    BreakoutDetectionDefaults      `json:"breakout_detection"`
    CoinConfluence       CoinConfluenceDefaults         `json:"coin_confluence"`
    CapitalAllocation    CapitalAllocationDefaults      `json:"capital_allocation"`
    SymbolDefaults       SymbolDefaults                 `json:"symbol_defaults"`
    SpotAutopilot        SpotAutopilotDefaults          `json:"spot_autopilot"`
    SettingsRiskIndex    SettingsRiskIndex              `json:"_settings_risk_index"`
}

// LoadDefaultSettings loads the default-settings.json file (singleton)
func LoadDefaultSettings() (*DefaultSettingsFile, error) {
    var loadErr error
    defaultSettingsOnce.Do(func() {
        data, err := os.ReadFile("default-settings.json")
        if err != nil {
            loadErr = fmt.Errorf("failed to read default-settings.json: %w", err)
            return
        }

        defaultSettings = &DefaultSettingsFile{}
        if err := json.Unmarshal(data, defaultSettings); err != nil {
            loadErr = fmt.Errorf("failed to parse default-settings.json: %w", err)
            return
        }

        log.Printf("[DEFAULT-SETTINGS] Loaded version %s (updated: %s)",
            defaultSettings.Metadata.Version,
            defaultSettings.Metadata.LastUpdated)
    })

    return defaultSettings, loadErr
}

// GetDefaultModeConfig returns default config for a specific mode
func GetDefaultModeConfig(modeName string) (*ModeFullConfig, error) {
    defaults, err := LoadDefaultSettings()
    if err != nil {
        return nil, err
    }

    config, exists := defaults.ModeConfigs[modeName]
    if !exists {
        return nil, fmt.Errorf("mode %s not found in defaults", modeName)
    }

    // Return a copy to prevent mutation
    configCopy := *config
    return &configCopy, nil
}

// GetDefaultSettingsJSON returns the entire defaults as JSON bytes
func GetDefaultSettingsJSON() ([]byte, error) {
    defaults, err := LoadDefaultSettings()
    if err != nil {
        return nil, err
    }
    return json.Marshal(defaults)
}
```

### Task 4: Update LoadSettings() to NOT Read Mode Configs

```go
// In settings.go - LoadSettings()

func (sm *SettingsManager) LoadSettings() error {
    // ... existing code ...

    // REMOVED: Do NOT sync mode configs from JSON
    // OLD CODE (DELETE):
    // sm.syncModeConfigsToRootSettings()

    // Mode configs are now ONLY loaded from database per-user
    log.Printf("[SETTINGS] Mode configs are database-driven (not from JSON)")

    return nil
}
```

### Task 5: Rename Old JSON File

```bash
mv autopilot_settings.json autopilot_settings.json.deprecated
```

Add header comment to deprecated file:
```json
{
  "_DEPRECATED": "This file is deprecated as of Story 4.13. Use default-settings.json instead.",
  "_MIGRATION_DATE": "2026-01-05",
  // ... rest of old content for reference
}
```

---

## File Structure

### New Files
| File | Purpose |
|------|---------|
| `default-settings.json` | Master defaults file (500+ settings) |
| `internal/autopilot/default_settings.go` | Helper functions to load defaults |

### Modified Files
| File | Changes |
|------|---------|
| `internal/autopilot/settings.go` | Remove `syncModeConfigsToRootSettings()` call |

### Deprecated Files
| File | Action |
|------|--------|
| `autopilot_settings.json` | Rename to `.deprecated`, keep for reference |

---

## Testing Requirements

### Test 1: Validate JSON Structure
```bash
# Validate JSON is parseable
cat default-settings.json | jq '.' > /dev/null && echo "Valid JSON"

# Check all mode configs present
cat default-settings.json | jq '.mode_configs | keys'
# Expected: ["position", "scalp", "scalp_reentry", "swing", "ultra_fast"]

# Check risk info present
cat default-settings.json | jq '.mode_configs.ultra_fast._risk_info'
```

### Test 2: Verify Mode Configs Not Loaded from JSON
```bash
# Restart container
./scripts/docker-dev.sh

# Check logs - should NOT see JSON sync
docker logs binance-trading-bot-dev 2>&1 | grep -i "sync.*mode.*config"
# Expected: No output (sync removed)

# Should see database-driven message
docker logs binance-trading-bot-dev 2>&1 | grep "database-driven"
```

### Test 3: Verify Defaults Load Correctly
```go
// Unit test
func TestLoadDefaultSettings(t *testing.T) {
    defaults, err := LoadDefaultSettings()
    assert.NoError(t, err)
    assert.NotNil(t, defaults)
    assert.Equal(t, "1.0.0", defaults.Metadata.Version)
    assert.Len(t, defaults.ModeConfigs, 5)
    assert.False(t, defaults.ModeConfigs["ultra_fast"].Enabled)
    assert.True(t, defaults.ModeConfigs["scalp"].Enabled)
}
```

### Test 4: Verify Settings Persist Across Restart
```bash
# 1. Disable ultra_fast in database
curl -X POST http://localhost:8094/api/futures/ultrafast/toggle \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# 2. Restart container
./scripts/docker-dev.sh

# 3. Check ultra_fast is still disabled
curl http://localhost:8094/api/futures/ultrafast/config \
  -H "Authorization: Bearer $TOKEN" | jq '.enabled'
# Expected: false
```

---

## Dependencies

### Prerequisites
- Story 4.9: Data migration complete (mode configs in database)
- Story 4.11: DB-first mode enabled status

### Blocks
- Story 4.14: New User & Load Defaults (needs this file)
- Story 4.15: Admin Settings Sync (needs this file)
- Story 4.16: Settings Comparison (needs risk_info structure)

---

## Definition of Done

- [ ] `default-settings.json` created with ALL 500+ settings
- [ ] All 15+ setting groups present and complete
- [ ] All 5 mode configs with all 14 sub-sections each
- [ ] Risk info (`_risk_info`) present for each group
- [ ] Settings risk index at root level
- [ ] `LoadDefaultSettings()` helper function works
- [ ] `LoadSettings()` does NOT read mode configs from JSON
- [ ] `autopilot_settings.json` renamed to `.deprecated`
- [ ] All tests pass
- [ ] No regression in trading behavior
- [ ] Code review approved

---

## Approval Sign-Off

- **Scrum Master (Bob)**: Pending
- **Developer (Amelia)**: Pending
- **Test Architect (Murat)**: Pending
- **Architect (Winston)**: Pending
- **Product Manager (John)**: Pending

---

## Notes

### Why This Approach?

1. **Single Source of Truth**: One file for all defaults, not scattered across code
2. **Admin Control**: Admin can modify defaults without code changes
3. **User Reset**: Users can reset to "factory defaults" anytime
4. **Auditability**: Risk info helps users understand impact of changes
5. **Multi-User Ready**: Each user gets copy, not shared reference

### Migration Path

1. Story 4.13 (this): Create foundation file
2. Story 4.14: New user creation uses this file
3. Story 4.15: Admin changes sync to this file
4. Story 4.16: UI shows diff against this file

### File Size Consideration

The `default-settings.json` will be approximately 2000-3000 lines. This is acceptable because:
- It's read once at startup
- It's cached in memory
- It's only used for defaults, not runtime

---

## Related Stories

- **Story 4.11:** DB-first mode enabled (prerequisite)
- **Story 4.14:** New User & Load Defaults (depends on this)
- **Story 4.15:** Admin Settings Sync (depends on this)
- **Story 4.16:** Settings Comparison & Risk Display (depends on this)
