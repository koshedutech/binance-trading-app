# Story 9.9: Position Optimization Refactoring

## Story Information
- **Epic**: Epic 9 - System Refinement and Optimization
- **Story ID**: 9.9
- **Priority**: Critical
- **Estimated Effort**: Large (Multi-phase)
- **Created**: 2026-01-14
- **Status**: Ready for Development
- **Reviews Completed**: 2026-01-14

---

## Team Review Summary

| Reviewer | Status | Key Findings |
|----------|--------|--------------|
| **PM** | APPROVED | Add user benefits section, clarify original mode strategy |
| **Architect** | APPROVED WITH CHANGES | Complete struct with all fields, resolve naming conflict |
| **Dev** | NEEDS CHANGES | Add missing files, fix migration scripts |
| **QA** | NEEDS CHANGES | Add original mode recovery, clarify early warning interaction |

### Critical Issues Resolved

1. **Original Mode Recovery** - Add `OriginalMode` field to `GiniePosition` before migration
2. **Complete Struct** - Include ALL 40+ fields from existing `ScalpReentryConfig`
3. **File Coverage** - Added 8 additional files requiring modification
4. **Migration Safety** - Added transaction wrapper, backup, and rollback scripts

### User Benefits
- **Risk Protection**: Positions will be protected by early warning and trend reversal regardless of optimization status
- **Consistent Behavior**: Trading mode stays predictable throughout position lifecycle
- **Reduced Losses**: Direct fix for the bug that caused JUPUSDT, JTOUSDT, SANDUSDT losses

---

## Problem Statement

### Current Architecture (Problematic)

The current system treats `scalp_reentry` as a **separate 5th trading mode**. When a position is opened in any mode (scalp, swing, position, ultra_fast), it gets "upgraded" to `scalp_reentry` mode for position optimization features.

**This causes critical issues:**

1. **Mode Identity Loss**: Position loses its original mode (scalp → scalp_reentry)
2. **Rule Bypass**: Original mode rules stop applying:
   - Early warning system explicitly skips `scalp_reentry` positions
   - Trend reversal detection doesn't close these positions
   - Mode-specific validations are bypassed
3. **Settings Confusion**: Which settings apply? Original mode or scalp_reentry?
4. **Trade Losses**: Positions weren't closed on trend reversal because the code checks `if mode == scalp_reentry { continue }` in early warning

### Evidence of Problem

```go
// ginie_autopilot.go:11011-11014
// This SKIPS all scalp_reentry positions from early warning!
if pos.Mode == GinieModeScalpReentry {
    continue  // Skip early warning for scalp_reentry
}
```

Result: Three positions (JUPUSDT, JTOUSDT, SANDUSDT) all in `scalp_reentry` mode were NOT closed when trend reversed, leading to significant losses.

---

## Solution: Position Optimization as a Feature, Not a Mode

### New Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  4 TRADING MODES (position mode NEVER changes)                 │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐       │
│  │  SCALP   │  │  SWING   │  │ POSITION │  │ULTRA FAST│       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘       │
│       │             │             │             │               │
│       ▼             ▼             ▼             ▼               │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  POSITION OPTIMIZATION (Feature - NOT a mode)           │   │
│  │                                                         │   │
│  │  ┌─────────────────────────────────────────────────┐   │   │
│  │  │  Section 1: Progressive Profit Taking           │   │   │
│  │  │  • TP1, TP2, TP3, TP4 percentages & sell %      │   │   │
│  │  │  • Re-buy settings (when TP hit, trend reverses)│   │   │
│  │  │  • AI-assisted TP optimization                   │   │   │
│  │  └─────────────────────────────────────────────────┘   │   │
│  │                                                         │   │
│  │  ┌─────────────────────────────────────────────────┐   │   │
│  │  │  Section 2: Hedging                             │   │   │
│  │  │  • Hedge mode enable/disable                    │   │   │
│  │  │  • Hedge ratio and chain settings               │   │   │
│  │  └─────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Key Principle

**Position mode NEVER changes.** If a trade opens as `scalp`, it stays `scalp` forever. Position optimization is a **feature flag** within each mode that enables:
- Progressive profit taking (TP1 → TP2 → TP3 → TP4)
- Re-buy on trend reversal after TP hit
- Optional hedging capabilities

All original mode rules (early warning, trend reversal, MTF validation, etc.) continue to apply regardless of position optimization status.

---

## Technical Specification

### Phase 1: Settings Structure Changes

#### 1.1 Remove Global `scalp_reentry_config`

**DELETE** from `default-settings.json`:
```json
"scalp_reentry_config": { ... }  // REMOVE ENTIRELY
```

#### 1.2 Add `position_optimization` Section to Each Mode

**ADD** to each mode in `mode_configs`:

```json
"mode_configs": {
  "scalp": {
    "enabled": true,
    "base_size_usd": 500,
    // ... existing settings ...

    "position_optimization": {
      "enabled": true,

      "progressive_profit": {
        "tp1_percent": 0.35,
        "tp1_sell_percent": 30,
        "tp2_percent": 0.70,
        "tp2_sell_percent": 50,
        "tp3_percent": 1.00,
        "tp3_sell_percent": 80,
        "tp4_percent": 1.50,
        "tp4_sell_percent": 100,

        "rebuy_enabled": true,
        "rebuy_percent": 50,
        "rebuy_price_buffer": 0.05,
        "max_rebuy_attempts": 1,
        "rebuy_timeout_sec": 300,
        "rebuy_require_trend_confirmation": true,
        "rebuy_min_adx": 25,

        "final_trailing_percent": 5,
        "final_hold_min_percent": 20,

        "use_ai_decisions": true,
        "ai_min_confidence": 0.65,
        "ai_tp_optimization": true,
        "ai_dynamic_sl": true,

        "max_cycles_per_position": 10,
        "profit_protection_enabled": true,
        "profit_protection_percent": 50
      },

      "hedging": {
        "enabled": false,
        "min_confidence_for_hedge": 75,
        "existing_must_be_in_profit": 1,
        "max_hedge_size_percent": 50,
        "allow_hedge_chains": false,
        "max_hedge_chain_depth": 2
      }
    }
  },

  "swing": {
    // ... existing settings ...
    "position_optimization": {
      // Same structure, possibly different default values
    }
  },

  "position": {
    // ... existing settings ...
    "position_optimization": {
      // Same structure, possibly different default values
    }
  },

  "ultra_fast": {
    // ... existing settings ...
    "position_optimization": {
      // Same structure, possibly different default values
    }
  }
}
```

#### 1.3 Go Struct Changes

**IMPORTANT**: Use existing `ScalpReentryConfig` struct from `scalp_reentry_types.go` - rename it to `PositionOptimizationConfig`. Do NOT create a new simplified struct.

**File**: `internal/autopilot/scalp_reentry_types.go` → rename to `position_optimization_types.go`

```go
// RENAME ScalpReentryConfig to PositionOptimizationConfig
// Keep ALL 40+ existing fields - do NOT simplify
type PositionOptimizationConfig struct {
    // Master toggle
    Enabled bool `json:"enabled"`

    // TP Levels configuration (positive - profit taking)
    TP1Percent     float64 `json:"tp1_percent"`
    TP1SellPercent float64 `json:"tp1_sell_percent"`
    TP2Percent     float64 `json:"tp2_percent"`
    TP2SellPercent float64 `json:"tp2_sell_percent"`
    TP3Percent     float64 `json:"tp3_percent"`
    TP3SellPercent float64 `json:"tp3_sell_percent"`

    // Re-entry configuration
    ReentryPercent                  float64 `json:"reentry_percent"`
    ReentryPriceBuffer              float64 `json:"reentry_price_buffer"`
    MaxReentryAttempts              int     `json:"max_reentry_attempts"`
    ReentryTimeoutSec               int     `json:"reentry_timeout_sec"`
    ReentryRequireTrendConfirmation bool    `json:"reentry_require_trend_confirmation"`
    ReentryMinADX                   float64 `json:"reentry_min_adx"`

    // Final portion trailing
    FinalTrailingPercent float64 `json:"final_trailing_percent"`
    FinalHoldMinPercent  float64 `json:"final_hold_min_percent"`

    // Dynamic SL
    DynamicSLMaxLossPct   float64 `json:"dynamic_sl_max_loss_pct"`
    DynamicSLProtectPct   float64 `json:"dynamic_sl_protect_pct"`
    DynamicSLUpdateIntSec int     `json:"dynamic_sl_update_int"`

    // AI Configuration
    UseAIDecisions   bool    `json:"use_ai_decisions"`
    AIMinConfidence  float64 `json:"ai_min_confidence"`
    AITPOptimization bool    `json:"ai_tp_optimization"`
    AIDynamicSL      bool    `json:"ai_dynamic_sl"`

    // Multi-agent configuration
    UseMultiAgent        bool `json:"use_multi_agent"`
    EnableSentimentAgent bool `json:"enable_sentiment_agent"`
    EnableRiskAgent      bool `json:"enable_risk_agent"`
    EnableTPAgent        bool `json:"enable_tp_agent"`

    // Adaptive learning
    EnableAdaptiveLearning   bool    `json:"enable_adaptive_learning"`
    AdaptiveWindowTrades     int     `json:"adaptive_window_trades"`
    AdaptiveMinTrades        int     `json:"adaptive_min_trades"`
    AdaptiveMaxReentryPctAdj float64 `json:"adaptive_max_reentry_adjust"`

    // Risk limits
    MaxCyclesPerPosition int     `json:"max_cycles_per_position"`
    MaxDailyReentries    int     `json:"max_daily_reentries"`
    MinPositionSizeUSD   float64 `json:"min_position_size_usd"`
    StopLossPercent      float64 `json:"stop_loss_percent"`

    // Hedge Mode Configuration
    HedgeModeEnabled    bool    `json:"hedge_mode_enabled"`
    TriggerOnProfitTP   bool    `json:"trigger_on_profit_tp"`
    TriggerOnLossTP     bool    `json:"trigger_on_loss_tp"`
    DCAOnLoss           bool    `json:"dca_on_loss"`
    MaxPositionMultiple float64 `json:"max_position_multiple"`
    CombinedROIExitPct  float64 `json:"combined_roi_exit_pct"`
    WideSLATRMultiplier float64 `json:"wide_sl_atr_multiplier"`
    DisableAISL         bool    `json:"disable_ai_sl"`

    // Rally Exit
    RallyExitEnabled      bool    `json:"rally_exit_enabled"`
    RallyADXThreshold     float64 `json:"rally_adx_threshold"`
    RallySustainedMovePct float64 `json:"rally_sustained_move_pct"`

    // Negative TP Levels (DCA on loss)
    NegTP1Percent    float64 `json:"neg_tp1_percent"`
    NegTP1AddPercent float64 `json:"neg_tp1_add_percent"`
    NegTP2Percent    float64 `json:"neg_tp2_percent"`
    NegTP2AddPercent float64 `json:"neg_tp2_add_percent"`
    NegTP3Percent    float64 `json:"neg_tp3_percent"`
    NegTP3AddPercent float64 `json:"neg_tp3_add_percent"`

    // Profit Protection
    ProfitProtectionEnabled bool    `json:"profit_protection_enabled"`
    ProfitProtectionPercent float64 `json:"profit_protection_percent"`
    MaxLossOfEarnedProfit   float64 `json:"max_loss_of_earned_profit"`

    // Chain Control
    AllowHedgeChains   bool `json:"allow_hedge_chains"`
    MaxHedgeChainDepth int  `json:"max_hedge_chain_depth"`
}

// RENAME ScalpReentryStatus to PositionOptimizationStatus
// Keep ALL existing fields from scalp_reentry_types.go

// Update ModeFullConfig to include position_optimization
type ModeFullConfig struct {
    // ... existing fields ...
    PositionOptimization *PositionOptimizationConfig `json:"position_optimization"`
}
```

---

### Phase 2: Remove scalp_reentry as a Mode

#### 2.1 Remove Mode Enum Value

**File**: `internal/autopilot/ginie_autopilot.go`

```go
// BEFORE
const (
    GinieModeUltraFast    GinieTradingMode = "ultra_fast"
    GinieModeScalp        GinieTradingMode = "scalp"
    GinieModeSwing        GinieTradingMode = "swing"
    GinieModePosition     GinieTradingMode = "position"
    GinieModeScalpReentry GinieTradingMode = "scalp_reentry"  // REMOVE THIS
)

// AFTER
const (
    GinieModeUltraFast GinieTradingMode = "ultra_fast"
    GinieModeScalp     GinieTradingMode = "scalp"
    GinieModeSwing     GinieTradingMode = "swing"
    GinieModePosition  GinieTradingMode = "position"
    // scalp_reentry REMOVED - it's now a feature, not a mode
)
```

#### 2.2 Remove Mode Upgrade Logic

**DELETE** all code that upgrades mode to scalp_reentry:

```go
// REMOVE ALL instances of:
pos.Mode = GinieModeScalpReentry
// or
mode = GinieModeScalpReentry
// or
[SCALP-UPGRADE] ... upgraded to scalp_reentry
```

#### 2.3 Add OriginalMode Field to GiniePosition (CRITICAL - from Review)

**BLOCKING ISSUE**: Currently when position upgrades to scalp_reentry, the original mode is LOST. We must add an `OriginalMode` field BEFORE migration.

```go
type GiniePosition struct {
    // ... existing fields ...

    // CRITICAL: Add this field to preserve original mode
    OriginalMode GinieTradingMode `json:"original_mode"`  // Never changes, set at position open

    // Position Optimization State (uses existing ScalpReentry field, renamed)
    // Note: CurrentTPLevel already exists - do NOT add again
    // PositionOptimizationActive is inferred from PositionOptimization != nil
}
```

**Pre-Migration Step**: Before removing scalp_reentry mode, run:
```sql
-- Add original_mode column to track original mode
ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS original_mode VARCHAR(20);

-- For existing scalp_reentry positions, we must default to 'scalp' since we cannot recover original
-- This is a data limitation acknowledged by the team
UPDATE futures_trades
SET original_mode = 'scalp'
WHERE trading_mode = 'scalp_reentry' AND original_mode IS NULL;

-- For non-scalp_reentry positions, original_mode = trading_mode
UPDATE futures_trades
SET original_mode = trading_mode
WHERE trading_mode != 'scalp_reentry' AND original_mode IS NULL;
```

---

### Phase 3: Update Backend Logic

#### 3.1 Position Optimization Check

Replace mode-based checks with feature flag checks:

```go
// BEFORE (problematic)
if pos.Mode == GinieModeScalpReentry {
    // Do scalp_reentry logic
}

// AFTER (correct)
modeConfig := ga.getModeConfig(pos.Mode)
if modeConfig.PositionOptimization != nil && modeConfig.PositionOptimization.Enabled {
    // Do position optimization logic
    // But pos.Mode is still "scalp" or "swing" etc.
}
```

#### 3.2 Remove Early Warning Mode Exclusion

**File**: `internal/autopilot/ginie_autopilot.go` (around line 11011)

```go
// BEFORE (problematic)
for _, pos := range positionsToCheck {
    // Skip scalp_reentry mode - let it manage its own exits via progressive TP
    if pos.Mode == GinieModeScalpReentry {
        continue  // THIS CAUSES POSITIONS TO NOT BE CLOSED ON REVERSAL!
    }
    // ... early warning logic
}

// AFTER (correct)
for _, pos := range positionsToCheck {
    // All modes now go through early warning
    // Position optimization is handled separately
    // ... early warning logic
}
```

#### 3.3 Progressive TP Logic

Update to read from mode's position_optimization settings:

```go
func (ga *GinieAutopilot) getProgressiveProfitConfig(mode GinieTradingMode) *ProgressiveProfitConfig {
    modeConfig := ga.getModeConfig(mode)
    if modeConfig == nil || modeConfig.PositionOptimization == nil {
        return nil
    }
    if !modeConfig.PositionOptimization.Enabled {
        return nil
    }
    return modeConfig.PositionOptimization.ProgressiveProfit
}
```

---

### Phase 4: Update UI

#### 4.1 Mode Settings Tab Structure

Each mode tab (Scalp, Swing, Position, Ultra Fast) will have:

```
┌─────────────────────────────────────────────────────────────────┐
│  SCALP MODE SETTINGS                                           │
│                                                                 │
│  [Existing Settings Sections]                                  │
│  • Base Size & Confidence                                      │
│  • SLTP Settings                                               │
│  • MTF Settings                                                │
│  • Trend Filters                                               │
│  • etc.                                                        │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  POSITION OPTIMIZATION                          [Toggle] │   │
│  │                                                         │   │
│  │  ┌───────────────────────────────────────────────────┐ │   │
│  │  │  Progressive Profit Taking                        │ │   │
│  │  │  ┌──────────┬──────────┬──────────┬──────────┐   │ │   │
│  │  │  │   TP1    │   TP2    │   TP3    │   TP4    │   │ │   │
│  │  │  │  0.35%   │  0.70%   │  1.00%   │  1.50%   │   │ │   │
│  │  │  │ Sell 30% │ Sell 50% │ Sell 80% │ Sell 100%│   │ │   │
│  │  │  └──────────┴──────────┴──────────┴──────────┘   │ │   │
│  │  │                                                   │ │   │
│  │  │  Re-buy Settings                                 │ │   │
│  │  │  [✓] Enable Re-buy   Re-buy %: [50]             │ │   │
│  │  │  Max Attempts: [1]   ADX Min: [25]              │ │   │
│  │  └───────────────────────────────────────────────────┘ │   │
│  │                                                         │   │
│  │  ┌───────────────────────────────────────────────────┐ │   │
│  │  │  Hedging                              [Toggle]    │ │   │
│  │  │  Max Hedge %: [50]   Chain Depth: [2]            │ │   │
│  │  └───────────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

#### 4.2 Remove Scalp Re-entry Tab/Section

Remove any separate UI for `scalp_reentry` configuration. All settings are now within each mode's Position Optimization section.

---

### Phase 5: Database Migration

#### 5.1 User Mode Configs Migration

```sql
-- For each mode, add position_optimization with current scalp_reentry values
UPDATE user_mode_configs
SET config_json = jsonb_set(
    config_json,
    '{position_optimization}',
    '{
        "enabled": true,
        "progressive_profit": {
            "tp1_percent": 0.35,
            "tp1_sell_percent": 30,
            "tp2_percent": 0.70,
            "tp2_sell_percent": 50,
            "tp3_percent": 1.00,
            "tp3_sell_percent": 80,
            "tp4_percent": 1.50,
            "tp4_sell_percent": 100,
            "rebuy_enabled": true,
            "rebuy_percent": 50,
            "rebuy_price_buffer": 0.05,
            "max_rebuy_attempts": 1,
            "rebuy_timeout_sec": 300,
            "rebuy_require_trend_confirmation": true,
            "rebuy_min_adx": 25,
            "final_trailing_percent": 5,
            "final_hold_min_percent": 20,
            "use_ai_decisions": true,
            "ai_min_confidence": 0.65,
            "ai_tp_optimization": true,
            "ai_dynamic_sl": true,
            "max_cycles_per_position": 10,
            "profit_protection_enabled": true,
            "profit_protection_percent": 50
        },
        "hedging": {
            "enabled": false,
            "min_confidence_for_hedge": 75,
            "existing_must_be_in_profit": 1,
            "max_hedge_size_percent": 50,
            "allow_hedge_chains": false,
            "max_hedge_chain_depth": 2
        }
    }'::jsonb
)
WHERE mode_name IN ('scalp', 'swing', 'position', 'ultra_fast');
```

#### 5.2 Remove scalp_reentry Entries

```sql
-- Delete scalp_reentry mode configs (data already migrated to each mode)
DELETE FROM user_mode_configs WHERE mode_name = 'scalp_reentry';
```

#### 5.3 Update Existing Positions

```sql
-- Convert any scalp_reentry positions back to their original mode
-- Note: We may need to store original_mode somewhere or infer from context
UPDATE futures_trades
SET trading_mode = 'scalp'  -- or appropriate original mode
WHERE trading_mode = 'scalp_reentry'
AND status = 'OPEN';
```

---

### Phase 6: Code Cleanup

#### 6.1 Files to Modify (Complete List from Reviews)

**Backend - Core (RENAME files):**
| File | Changes |
|------|---------|
| `internal/autopilot/scalp_reentry_types.go` | RENAME to `position_optimization_types.go`, rename types |
| `internal/autopilot/scalp_reentry_logic.go` | RENAME to `position_optimization_logic.go`, update function signatures |
| `internal/autopilot/scalp_reentry_agents.go` | RENAME to `position_optimization_agents.go` |
| `internal/autopilot/scalp_reentry_learning.go` | RENAME to `position_optimization_learning.go` |
| `internal/autopilot/scalp_reentry_test.go` | RENAME to `position_optimization_test.go` |

**Backend - Updates:**
| File | Changes |
|------|---------|
| `internal/autopilot/ginie_autopilot.go` | Remove mode upgrade, remove early warning skip, update monitoring |
| `internal/autopilot/ginie_types.go` | Remove `GinieModeScalpReentry`, `ScanStatusScalpReentryReady` |
| `internal/autopilot/ginie_analyzer.go` | Update mode analysis |
| `internal/autopilot/settings.go` | Add PositionOptimization to ModeFullConfig |
| `internal/autopilot/default_settings.go` | Update default config structure |
| `internal/autopilot/admin_sync.go` | Update settings sync |
| `internal/api/handlers_settings_defaults.go` | Update comparison logic |
| `internal/api/handlers_ginie.go` | Update API endpoints |

**Database/Redis:**
| File | Changes |
|------|---------|
| `internal/database/redis_position_state.go` | Update state persistence |
| `internal/database/redis_order_tracker.go` | Update references |

**Frontend:**
| File | Changes |
|------|---------|
| `web/src/components/ScalpReentryMonitor.tsx` | RENAME to `PositionOptimizationMonitor.tsx` |
| `web/src/components/TradingModeBadge.tsx` | Remove scalp_reentry badge |
| `web/src/components/GiniePanel.tsx` | Update references |
| `web/src/components/settings/*` | Add position_optimization to each mode |

**Config:**
| File | Changes |
|------|---------|
| `default-settings.json` | Move scalp_reentry_config into each mode's position_optimization |

#### 6.2 Search and Replace

```
scalp_reentry       → position_optimization (where referring to feature)
scalp-reentry       → position-optimization
ScalpReentry        → PositionOptimization
SCALP_REENTRY       → (remove - no longer a mode)
GinieModeScalpReentry → (remove entirely)
```

---

## Acceptance Criteria

### AC9.9.1: Mode Persistence
- [ ] Position mode (scalp/swing/position/ultra_fast) NEVER changes during position lifecycle
- [ ] No code exists that sets mode to `scalp_reentry`
- [ ] Position logs show original mode throughout

### AC9.9.2: Settings Structure
- [ ] Each mode has `position_optimization` section in settings
- [ ] `scalp_reentry_config` removed from global settings
- [ ] `progressive_profit` subsection contains TP1-TP4 and rebuy settings
- [ ] `hedging` subsection contains all hedge settings

### AC9.9.3: Early Warning Works for All Modes
- [ ] Early warning no longer skips any positions based on mode
- [ ] Positions with position_optimization enabled still get early warning checks
- [ ] Trend reversal detection works for all positions

### AC9.9.4: Progressive TP Logic
- [ ] TP logic reads from mode's `position_optimization.progressive_profit`
- [ ] Re-buy logic reads from same section
- [ ] Each mode can have different TP percentages

### AC9.9.5: UI Updates
- [ ] Position Optimization section appears in each mode's settings tab
- [ ] Two subsections: Progressive Profit Taking and Hedging
- [ ] No separate scalp_reentry tab/section exists

### AC9.9.6: Database Migration
- [ ] All 4 modes have position_optimization config in database
- [ ] scalp_reentry mode entries removed from user_mode_configs
- [ ] Existing open positions updated to original mode

### AC9.9.7: Code Cleanup
- [ ] No references to `scalp_reentry` as a mode in codebase
- [ ] `GinieModeScalpReentry` constant removed
- [ ] All upgrade logic removed

---

## Risk Assessment

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking existing positions | High | Careful migration, backup before changes |
| Settings loss during migration | Medium | Copy existing settings to all modes |
| UI confusion during transition | Low | Clear UI with same display as before |
| Performance impact | Low | No significant changes to runtime behavior |

---

## Testing Plan

1. **Unit Tests**: Settings loading, struct marshaling
2. **Integration Tests**: Position lifecycle without mode change
3. **Migration Tests**: Database migration scripts on test data
4. **UI Tests**: Settings display and save functionality
5. **End-to-End Tests**: Open position → TP hits → Rebuy → Early warning still works

---

## Implementation Order

1. **Phase 1**: Settings structure changes (Go structs + default-settings.json)
2. **Phase 2**: Remove scalp_reentry mode constant and enum
3. **Phase 3**: Update backend logic to use position_optimization feature flag
4. **Phase 4**: Database migration scripts
5. **Phase 5**: UI updates
6. **Phase 6**: Code cleanup and testing
7. **Phase 7**: Documentation update

---

## Dependencies

- Story 9.5 (Enhanced Trend Validation Filters) - Completed
- No external dependencies

---

## Notes

This refactoring addresses the root cause of why positions weren't being closed on trend reversal. The architectural change from "mode upgrade" to "feature flag" ensures that all mode-specific protections (early warning, trend reversal, etc.) continue to apply throughout the position's lifecycle.
