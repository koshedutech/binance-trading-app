# Story 11.41: Comprehensive Mode-Strategy Configuration Amendment

**Epic:** 11 - Position Decision Engine
**Priority:** P0
**Status:** done
**Created:** 2026-01-23

---

## Goal

Comprehensively amend the mode-strategy configuration to include ALL essential trading fields from the legacy `mode_configs` system (184 fields) into the new `modes` structure. This story ensures the trading system has complete configuration capability following the **Settings Lifecycle Rule (SETTINGS-LIFECYCLE-001)**.

---

## Problem Statement

The current `modes` structure in `default-settings.json` has only **~30 fields per strategy**, while the legacy `mode_configs` has **184 fields per mode**. Critical trading functionality is missing:

### Gap Analysis: Missing Fields

| Category | Old System Fields | New System Fields | Gap |
|----------|-------------------|-------------------|-----|
| **Timeframe** | 3 | 3 | None |
| **TP Cut Percentages** | 6 (tp1/2/3_percent + tp1/2/3_sell_percent) | 3 (only price levels) | **Missing: tp1/2/3_sell_percent** |
| **MTF (Multi-Timeframe)** | 7 | 0 | **Entire section missing** |
| **Circuit Breaker** | 9 | 0 | **Entire section missing** |
| **Hedge Settings** | 6 | 0 | **Entire section missing** |
| **Averaging/DCA** | 8 | 0 | **Entire section missing** |
| **Stale Release** | 7 | 0 | **Entire section missing** |
| **Position Optimization** | 48 | 0 | **Entire section missing** |
| **Funding Rate** | 8 | 0 | **Entire section missing** |
| **Risk Settings** | 6 | 0 | **Entire section missing** |
| **Trend Divergence** | 4 | 0 | **Entire section missing** |
| **Dynamic AI Exit** | 5 | 4 (partial) | **Missing: detailed settings** |
| **Early Warning** | 5 | 1 (boolean only) | **Missing: detailed settings** |
| **Assignment Criteria** | 8 | 0 | **Entire section missing** |

**Total Gap:** ~120 essential fields per mode-strategy combination are missing.

---

## Acceptance Criteria

### 1. default-settings.json Amendment
- [ ] Add all missing configuration sections to each strategy under `modes`
- [ ] Include TP cut percentages (tp1/2/3_sell_percent)
- [ ] Include MTF settings per strategy
- [ ] Include circuit breaker per strategy
- [ ] Include hedge settings per strategy
- [ ] Include averaging/DCA per strategy
- [ ] Include stale release per strategy
- [ ] Include position optimization per strategy
- [ ] Include funding rate per strategy
- [ ] Include risk settings per strategy
- [ ] Include trend divergence per strategy
- [ ] Include detailed dynamic AI exit per strategy
- [ ] Include detailed early warning per strategy
- [ ] Total: 16 mode-strategy combinations fully configured

### 2. Database Migration
- [ ] Create migration `XXX_expand_user_mode_strategy_settings.sql`
- [ ] JSONB column already exists - ensure schema supports new fields
- [ ] Migration script to copy relevant settings from `user_mode_configs` to `user_mode_strategy_settings`
- [ ] No data loss - existing strategy settings preserved

### 3. Go Struct Updates
- [ ] Update `ModeStrategyConfig` struct in `models_user_settings.go`
- [ ] Add all new configuration sub-structs
- [ ] Ensure JSON tags match `default-settings.json`

### 4. Redis Cache Layer
- [ ] Update `settings_cache_service.go` for new fields
- [ ] Ensure cache key structure supports full config
- [ ] Update extract/merge functions
- [ ] Test cache read/write with new structure

### 5. User Initialization
- [ ] Update `InitializeUserModeStrategies()` to include all new fields
- [ ] Ensure new users get complete defaults

### 6. API Endpoints
- [ ] GET/PUT `/api/modes/{mode}/strategies/{strategy}` returns/accepts all fields
- [ ] Validation for all new fields
- [ ] Write-through cache pattern maintained

### 7. Reset Settings Page UI
- [ ] Add new grouped sections for each configuration category
- [ ] Collapsible sections: MTF, Circuit Breaker, Hedge, Averaging, etc.
- [ ] Show current vs default comparison for all fields
- [ ] Reset per section and per strategy

### 8. Futures Page Mode Strategy Settings UI
- [ ] Display all configuration sections with proper grouping
- [ ] Editable fields with appropriate input types
- [ ] Save functionality with validation
- [ ] Real-time preview where applicable

---

## Technical Design

### New Mode-Strategy Configuration Structure

```json
{
  "modes": {
    "scalp": {
      "name": "scalp",
      "enabled": true,
      "default_strategy": "trend_following",
      "auto_select_strategy": false,
      "strategies": {
        "trend_following": {
          "enabled": true,
          "priority": 1,
          "supported_regimes": ["TRENDING", "VOLATILE_TRENDING"],

          "position_sizing": {
            "leverage": 10,
            "max_positions": 10,
            "base_size_usd": 200,
            "max_size_usd": 500,
            "min_position_size_usd": 50,
            "safety_margin": 0.9,
            "auto_size_enabled": false,
            "auto_size_min_cover_fee": 0.1
          },

          "timeframe": {
            "trend_timeframe": "15m",
            "entry_timeframe": "5m",
            "analysis_timeframe": "15m"
          },

          "mtf": {
            "enabled": true,
            "primary_timeframe": "15m",
            "primary_weight": 0.5,
            "secondary_timeframe": "5m",
            "secondary_weight": 0.3,
            "tertiary_timeframe": "1m",
            "tertiary_weight": 0.2,
            "min_consensus": 2,
            "min_weighted_strength": 55,
            "trend_stability_check": true
          },

          "sltp": {
            "sl_percent": 2.0,
            "tp1_percent": 0.5,
            "tp1_sell_percent": 50,
            "tp2_percent": 1.0,
            "tp2_sell_percent": 30,
            "tp3_percent": 1.5,
            "tp3_sell_percent": 20,
            "trailing_enabled": true,
            "trailing_activation_pct": 0.5,
            "trailing_stop_pct": 0.3,
            "use_atr_based": false,
            "atr_sl_multiplier": 1.5,
            "atr_tp_multiplier": 2.0,
            "min_sl_distance_pct": 0.3
          },

          "confidence": {
            "min_confidence": 55,
            "high_confidence": 75,
            "ultra_confidence": 85
          },

          "entry_conditions": {
            "adx_min": 15,
            "adx_max": 100,
            "rsi_min": 30,
            "rsi_max": 70,
            "require_trend_align": true,
            "min_volume_multiplier": 1.0,
            "use_limit_entry": true,
            "limit_order_gap_percent": 0.1,
            "max_limit_gap_percent": 0.5
          },

          "exit_conditions": {
            "use_ai_exit": true,
            "max_hold_minutes": 240,
            "early_warning_enabled": true,
            "exit_on_trend_reversal": true,
            "adx_exit_threshold": 15
          },

          "scoring": {
            "technical_weight": 40,
            "momentum_weight": 30,
            "volume_weight": 15,
            "sentiment_weight": 15,
            "min_score": 55,
            "high_score": 75
          },

          "circuit_breaker": {
            "max_loss_per_hour_usd": 100,
            "max_loss_per_day_usd": 300,
            "max_consecutive_losses": 3,
            "cooldown_minutes": 30,
            "max_trades_per_hour": 20,
            "max_trades_per_day": 100,
            "win_rate_check_after": 10,
            "min_win_rate_pct": 40
          },

          "hedge": {
            "allow_hedge": false,
            "min_confidence_for_hedge": 75,
            "existing_must_be_in_profit_pct": 1.0,
            "max_hedge_size_percent": 50,
            "allow_same_mode_hedge": false,
            "max_total_exposure_multiplier": 2.0
          },

          "averaging": {
            "allow_averaging": false,
            "average_up_profit_percent": 1.0,
            "average_down_loss_percent": -1.0,
            "add_size_percent": 50,
            "max_averages": 2,
            "min_confidence_for_average": 70,
            "use_llm_for_averaging": true,
            "staged_entry_enabled": false,
            "staged_entry_levels": 3,
            "staged_entry_percent": [50, 30, 20]
          },

          "stale_release": {
            "enabled": true,
            "max_hold_duration_minutes": 240,
            "min_profit_to_keep_pct": 0.3,
            "max_loss_to_force_close_pct": -1.5,
            "stale_zone_lo_pct": -0.2,
            "stale_zone_hi_pct": 0.2,
            "stale_zone_action": "close"
          },

          "position_optimization": {
            "reentry_enabled": false,
            "reentry_after_tp1": true,
            "reentry_min_pullback_pct": 0.3,
            "max_reentries_per_position": 2,
            "dynamic_sl_enabled": false,
            "dynamic_sl_at_breakeven_pct": 0.5,
            "profit_protection_enabled": false,
            "profit_protection_trigger_pct": 1.0,
            "profit_protection_lock_pct": 50
          },

          "funding_rate": {
            "enabled": false,
            "max_funding_rate_pct": 0.1,
            "exit_before_funding_minutes": 15,
            "block_entry_above_rate_pct": 0.15
          },

          "risk": {
            "risk_level": "moderate",
            "max_drawdown_percent": 5.0,
            "max_daily_loss_percent": 3.0,
            "position_risk_percent": 2.0
          },

          "trend_divergence": {
            "enabled": true,
            "min_divergence_percent": 10,
            "block_on_divergence": true,
            "divergence_weight": 0.3
          },

          "dynamic_ai_exit": {
            "enabled": true,
            "min_hold_before_ai_ms": 60000,
            "ai_check_interval_ms": 30000,
            "use_llm_for_loss": true,
            "use_llm_for_profit": false,
            "max_hold_time_ms": 14400000
          },

          "early_warning": {
            "enabled": true,
            "start_after_minutes": 30,
            "min_loss_percent": -0.5,
            "check_interval_secs": 60,
            "close_min_hold_mins": 15
          }
        },
        "mean_reversion": { /* ... similar full config ... */ },
        "breakout": { /* ... similar full config ... */ },
        "range_trading": { /* ... similar full config ... */ }
      }
    },
    "swing": { /* ... 4 strategies with full config ... */ },
    "position": { /* ... 4 strategies with full config ... */ },
    "ultra_fast": { /* ... 4 strategies with full config ... */ }
  }
}
```

### Configuration Sections Summary

| Section | Fields | Description |
|---------|--------|-------------|
| `position_sizing` | 8 | Leverage, position limits, sizing rules |
| `timeframe` | 3 | Trend/entry/analysis timeframes |
| `mtf` | 9 | Multi-timeframe alignment settings |
| `sltp` | 14 | Stop loss, take profit, trailing, TP cut percentages |
| `confidence` | 3 | Min/high/ultra confidence thresholds |
| `entry_conditions` | 10 | ADX, RSI, trend alignment, limit orders |
| `exit_conditions` | 5 | AI exit, max hold, trend reversal |
| `scoring` | 6 | Weight distribution and thresholds |
| `circuit_breaker` | 8 | Loss limits, cooldowns, trade limits |
| `hedge` | 6 | Hedging rules |
| `averaging` | 10 | DCA/averaging configuration |
| `stale_release` | 7 | Stale position management |
| `position_optimization` | 9 | Reentry, dynamic SL, profit protection |
| `funding_rate` | 4 | Funding rate management |
| `risk` | 4 | Risk level and limits |
| `trend_divergence` | 4 | Trend divergence blocking |
| `dynamic_ai_exit` | 6 | AI-powered exit decisions |
| `early_warning` | 5 | Early warning system |

**Total: ~107 fields per strategy × 4 strategies × 4 modes = 1,712 configurable parameters**

---

## Implementation Tasks

### Phase 1: default-settings.json Amendment

#### Task 1.1: Create Full Strategy Template
- Define complete strategy configuration with all 18 sections
- Ensure sensible defaults for each mode (scalp vs swing vs position vs ultra_fast differ)
- Document each field with `_info` comments

#### Task 1.2: Apply Template to All 16 Mode-Strategy Combinations
- Scalp: trend_following, mean_reversion, breakout, range_trading
- Swing: trend_following, mean_reversion, breakout, range_trading
- Position: trend_following, mean_reversion, breakout, range_trading
- Ultra_fast: trend_following, mean_reversion, breakout, range_trading

#### Task 1.3: Validate JSON Structure
- Schema validation
- Field type validation
- Default value sanity checks

### Phase 2: Go Struct Updates

#### Task 2.1: Update ModeStrategyConfig Struct
```go
// internal/database/models_user_settings.go

type ModeStrategyConfig struct {
    Enabled          bool                        `json:"enabled"`
    Priority         int                         `json:"priority"`
    SupportedRegimes []string                    `json:"supported_regimes"`
    PositionSizing   StrategyPositionSizing      `json:"position_sizing"`
    Timeframe        StrategyTimeframe           `json:"timeframe"`
    MTF              StrategyMTF                 `json:"mtf"`
    SLTP             StrategySLTP                `json:"sltp"`
    Confidence       StrategyConfidence          `json:"confidence"`
    EntryConditions  StrategyEntryConditions     `json:"entry_conditions"`
    ExitConditions   StrategyExitConditions      `json:"exit_conditions"`
    Scoring          StrategyScoring             `json:"scoring"`
    CircuitBreaker   StrategyCircuitBreaker      `json:"circuit_breaker"`
    Hedge            StrategyHedge               `json:"hedge"`
    Averaging        StrategyAveraging           `json:"averaging"`
    StaleRelease     StrategyStaleRelease        `json:"stale_release"`
    PositionOptimization StrategyPositionOptimization `json:"position_optimization"`
    FundingRate      StrategyFundingRate         `json:"funding_rate"`
    Risk             StrategyRisk                `json:"risk"`
    TrendDivergence  StrategyTrendDivergence     `json:"trend_divergence"`
    DynamicAIExit    StrategyDynamicAIExit       `json:"dynamic_ai_exit"`
    EarlyWarning     StrategyEarlyWarning        `json:"early_warning"`
}

type StrategySLTP struct {
    SLPercent             float64 `json:"sl_percent"`
    TP1Percent            float64 `json:"tp1_percent"`
    TP1SellPercent        int     `json:"tp1_sell_percent"`  // NEW: Position cut at TP1
    TP2Percent            float64 `json:"tp2_percent"`
    TP2SellPercent        int     `json:"tp2_sell_percent"`  // NEW: Position cut at TP2
    TP3Percent            float64 `json:"tp3_percent"`
    TP3SellPercent        int     `json:"tp3_sell_percent"`  // NEW: Position cut at TP3
    TrailingEnabled       bool    `json:"trailing_enabled"`
    TrailingActivationPct float64 `json:"trailing_activation_pct"`
    TrailingStopPct       float64 `json:"trailing_stop_pct"`
    UseATRBased           bool    `json:"use_atr_based"`
    ATRSLMultiplier       float64 `json:"atr_sl_multiplier"`
    ATRTPMultiplier       float64 `json:"atr_tp_multiplier"`
    MinSLDistancePct      float64 `json:"min_sl_distance_pct"`
}

// ... Add all other sub-structs
```

#### Task 2.2: Create All Sub-Structs
- StrategyPositionSizing
- StrategyMTF
- StrategyCircuitBreaker
- StrategyHedge
- StrategyAveraging
- StrategyStaleRelease
- StrategyPositionOptimization
- StrategyFundingRate
- StrategyRisk
- StrategyTrendDivergence
- StrategyDynamicAIExit
- StrategyEarlyWarning

### Phase 3: Database Migration

#### Task 3.1: Create Migration File
```sql
-- migrations/XXX_expand_mode_strategy_settings.sql

-- The JSONB column already supports arbitrary structures
-- This migration ensures indexes and adds validation

-- Add GIN index for JSONB queries
CREATE INDEX IF NOT EXISTS idx_user_mode_strategy_settings_jsonb
ON user_mode_strategy_settings USING GIN (settings);

-- Migration: Copy existing mode_configs to trend_following strategies
-- (Only for users who don't already have strategy settings)
INSERT INTO user_mode_strategy_settings (user_id, mode, strategy, enabled, priority, settings)
SELECT
    umc.user_id,
    umc.mode_name as mode,
    'trend_following' as strategy,
    (umc.config_json->>'enabled')::boolean as enabled,
    1 as priority,
    umc.config_json as settings
FROM user_mode_configs umc
WHERE NOT EXISTS (
    SELECT 1 FROM user_mode_strategy_settings umss
    WHERE umss.user_id = umc.user_id
    AND umss.mode = umc.mode_name
    AND umss.strategy = 'trend_following'
)
ON CONFLICT (user_id, mode, strategy) DO NOTHING;
```

### Phase 4: Redis Cache Updates

#### Task 4.1: Update Cache Key Structure
```
user:{userID}:mode_strategy:{mode}:{strategy}
```

#### Task 4.2: Update Cache Functions
- `GetModeStrategyConfig()` - Load full config
- `SetModeStrategyConfig()` - Store full config
- `GetModeStrategySection()` - Load specific section (e.g., `sltp`)
- `SetModeStrategySection()` - Update specific section

### Phase 5: API Updates

#### Task 5.1: Expand API Response
```go
// GET /api/modes/{mode}/strategies/{strategy}
type ModeStrategyResponse struct {
    Mode             string                 `json:"mode"`
    Strategy         string                 `json:"strategy"`
    Enabled          bool                   `json:"enabled"`
    Priority         int                    `json:"priority"`
    SupportedRegimes []string               `json:"supported_regimes"`

    // All sections included
    PositionSizing   StrategyPositionSizing `json:"position_sizing"`
    Timeframe        StrategyTimeframe      `json:"timeframe"`
    MTF              StrategyMTF            `json:"mtf"`
    SLTP             StrategySLTP           `json:"sltp"`
    // ... all other sections

    // Comparison metadata
    IsDefault              bool   `json:"is_default"`
    DifferencesFromDefault int    `json:"differences_from_default"`
}
```

#### Task 5.2: Add Section-Level Endpoints
```
PUT /api/modes/{mode}/strategies/{strategy}/sections/{section}
POST /api/modes/{mode}/strategies/{strategy}/sections/{section}/reset
```

### Phase 6: Frontend Updates

#### Task 6.1: Update TypeScript Types
```typescript
// web/src/types/modeStrategy.ts

interface ModeStrategyConfig {
  enabled: boolean;
  priority: number;
  supported_regimes: string[];

  position_sizing: PositionSizingConfig;
  timeframe: TimeframeConfig;
  mtf: MTFConfig;
  sltp: SLTPConfig;
  confidence: ConfidenceConfig;
  entry_conditions: EntryConditionsConfig;
  exit_conditions: ExitConditionsConfig;
  scoring: ScoringConfig;
  circuit_breaker: CircuitBreakerConfig;
  hedge: HedgeConfig;
  averaging: AveragingConfig;
  stale_release: StaleReleaseConfig;
  position_optimization: PositionOptimizationConfig;
  funding_rate: FundingRateConfig;
  risk: RiskConfig;
  trend_divergence: TrendDivergenceConfig;
  dynamic_ai_exit: DynamicAIExitConfig;
  early_warning: EarlyWarningConfig;
}

interface SLTPConfig {
  sl_percent: number;
  tp1_percent: number;
  tp1_sell_percent: number;  // NEW
  tp2_percent: number;
  tp2_sell_percent: number;  // NEW
  tp3_percent: number;
  tp3_sell_percent: number;  // NEW
  trailing_enabled: boolean;
  trailing_activation_pct: number;
  trailing_stop_pct: number;
  use_atr_based: boolean;
  atr_sl_multiplier: number;
  atr_tp_multiplier: number;
  min_sl_distance_pct: number;
}
```

#### Task 6.2: Reset Settings Page Enhancement
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Mode-Strategy Settings                                    [Reset All]        │
├─────────────────────────────────────────────────────────────────────────────┤
│ ▼ Scalp Mode                                                                 │
│   ├─ ▼ Trend Following (85/107 match defaults)               [Reset]        │
│   │     ├─ ▶ Position Sizing (8 fields)         All Match                   │
│   │     ├─ ▶ Timeframe (3 fields)               All Match                   │
│   │     ├─ ▶ MTF Settings (9 fields)            2 Differences               │
│   │     ├─ ▶ SLTP (14 fields)                   5 Differences  ⚠️           │
│   │     ├─ ▶ Confidence (3 fields)              All Match                   │
│   │     ├─ ▶ Entry Conditions (10 fields)       3 Differences               │
│   │     ├─ ▶ Exit Conditions (5 fields)         All Match                   │
│   │     ├─ ▶ Scoring (6 fields)                 1 Difference                │
│   │     ├─ ▶ Circuit Breaker (8 fields)         2 Differences               │
│   │     ├─ ▶ Hedge (6 fields)                   All Match                   │
│   │     ├─ ▶ Averaging (10 fields)              All Match                   │
│   │     ├─ ▶ Stale Release (7 fields)           1 Difference                │
│   │     ├─ ▶ Position Optimization (9 fields)   3 Differences               │
│   │     ├─ ▶ Funding Rate (4 fields)            All Match                   │
│   │     ├─ ▶ Risk (4 fields)                    2 Differences               │
│   │     ├─ ▶ Trend Divergence (4 fields)        1 Difference                │
│   │     ├─ ▶ Dynamic AI Exit (6 fields)         2 Differences               │
│   │     └─ ▶ Early Warning (5 fields)           All Match                   │
│   │                                                                          │
│   ├─ ▶ Mean Reversion (107/107 match defaults)               [Reset]        │
│   ├─ ▶ Breakout (102/107 match defaults)                     [Reset]        │
│   └─ ▶ Range Trading (disabled)                              [Reset]        │
│                                                                              │
│ ▼ Swing Mode                                                                 │
│   └─ ...                                                                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### Task 6.3: Futures Page Mode Strategy Settings
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ Mode: [Scalp ▼]    Strategy: [Trend Following ▼]    [Auto-Select: OFF]      │
├─────────────────────────────────────────────────────────────────────────────┤
│ ┌─ Position Sizing ──────────────────────────────────────────────────────┐  │
│ │  Leverage: [10]        Max Positions: [10]      Base Size: [$200]      │  │
│ │  Max Size: [$500]      Min Size: [$50]          Safety Margin: [0.9]   │  │
│ └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ ┌─ Timeframe ────────────────────────────────────────────────────────────┐  │
│ │  Trend TF: [15m ▼]     Entry TF: [5m ▼]         Analysis TF: [15m ▼]   │  │
│ └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ ┌─ Multi-Timeframe (MTF) ────────────────────────────────────────────────┐  │
│ │  [✓] Enabled           Min Consensus: [2]       Min Strength: [55]     │  │
│ │  Primary: [15m] Weight: [0.5]                                          │  │
│ │  Secondary: [5m] Weight: [0.3]                                         │  │
│ │  Tertiary: [1m] Weight: [0.2]                                          │  │
│ └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ ┌─ Stop Loss / Take Profit ──────────────────────────────────────────────┐  │
│ │  Stop Loss: [2.0%]                                                     │  │
│ │  ┌────────────────────────────────────────────────────────────────┐    │  │
│ │  │ TP Level │ Price % │ Position Cut % │ Remaining │              │    │  │
│ │  │ TP1      │ [0.5%]  │ [50%]          │ 50%       │              │    │  │
│ │  │ TP2      │ [1.0%]  │ [30%]          │ 20%       │              │    │  │
│ │  │ TP3      │ [1.5%]  │ [20%]          │ 0%        │              │    │  │
│ │  └────────────────────────────────────────────────────────────────┘    │  │
│ │  [✓] Trailing Stop   Activation: [0.5%]   Trail: [0.3%]               │  │
│ └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ ┌─ Circuit Breaker ──────────────────────────────────────────────────────┐  │
│ │  Max Loss/Hour: [$100]   Max Loss/Day: [$300]   Cooldown: [30min]      │  │
│ │  Max Consecutive Losses: [3]   Max Trades/Hour: [20]                   │  │
│ └────────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│ [More sections collapsed by default...]                                      │
│                                                                              │
│              [Reset to Defaults]    [Cancel]    [Save Changes]              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Settings Lifecycle Compliance

This story follows **SETTINGS-LIFECYCLE-001**:

```
default-settings.json → Database → Redis Cache → API → Frontend
```

### Checklist

- [ ] 1. Added to `default-settings.json` in correct section
- [ ] 2. Added to Go struct with matching JSON tag
- [ ] 3. Database migration created
- [ ] 4. Repository methods handle new field
- [ ] 5. Cache `extractGroupFromConfig()` extracts field
- [ ] 6. Cache `mergeGroupIntoConfig()` merges field
- [ ] 7. Admin defaults cache loads new field
- [ ] 8. New user initialization includes field
- [ ] 9. API handler reads/writes new field
- [ ] 10. Frontend component displays/edits field
- [ ] 11. Settings comparison shows field with risk level
- [ ] 12. Reset-to-defaults includes new field

---

## Files to Modify

### Backend (Go)

| File | Changes |
|------|---------|
| `default-settings.json` | Complete restructure of `modes` section |
| `internal/database/models_user_settings.go` | Add all new sub-structs |
| `internal/database/repository_mode_strategy.go` | Update CRUD for new fields |
| `internal/database/user_initialization.go` | Initialize all 16 mode-strategies |
| `internal/cache/settings_cache_service.go` | Update cache functions |
| `internal/api/handlers_mode_strategy.go` | Expand endpoints |
| `migrations/XXX_expand_mode_strategy_settings.sql` | Schema and data migration |

### Frontend (React/TypeScript)

| File | Changes |
|------|---------|
| `web/src/types/modeStrategy.ts` | Add all new type definitions |
| `web/src/services/modeStrategyApi.ts` | Update API calls |
| `web/src/components/ModeStrategySettings.tsx` | Full settings UI |
| `web/src/components/ModeStrategySLTPCard.tsx` | SLTP with TP cut % |
| `web/src/components/ModeStrategyMTFCard.tsx` | MTF settings |
| `web/src/components/ModeStrategyCircuitBreakerCard.tsx` | Circuit breaker |
| `web/src/pages/ResetSettings.tsx` | Add mode-strategy sections |
| `web/src/pages/FuturesDashboard.tsx` | Integrate settings UI |

---

## Dependencies

- Story 11.28-11.33 (Mode-Strategy Architecture) - Foundation in place
- Story 11.35 (Expand Default Mode-Strategy Configs) - Partial completion
- Story 11.39 (Futures Mode-Strategy Data Wiring) - API basics done

---

## Testing Requirements

### Unit Tests
- [ ] All new Go structs have JSON marshal/unmarshal tests
- [ ] Repository CRUD tests for full config
- [ ] Cache service tests for section extraction/merge
- [ ] Migration script test (idempotent, no data loss)

### Integration Tests
- [ ] API returns complete config
- [ ] Settings save and reload correctly
- [ ] Reset to defaults works for all sections
- [ ] New user gets all 16 mode-strategy configs

### Frontend Tests
- [ ] Form validation for all field types
- [ ] Save functionality with optimistic updates
- [ ] Section collapse/expand
- [ ] Comparison view accuracy

---

## Story Points: 21

**Breakdown:**
- default-settings.json restructure: 3 points
- Go struct updates: 3 points
- Database migration: 2 points
- Cache updates: 2 points
- User initialization: 2 points
- API expansion: 3 points
- Reset Settings UI: 3 points
- Futures Settings UI: 3 points

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-23 | ready | Story created with comprehensive gap analysis |
| 2026-01-23 | done | Story completed - all 8 ACs implemented. CODE REVIEW PASSED + QA TRACE PASSED |
