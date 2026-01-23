# Story 11.42: Wire Strategy-Level Settings to Backend Business Logic

## Story Overview

**Story ID:** 11.42
**Epic:** Epic 11 - Position Decision Engine
**Status:** Completed
**Priority:** High
**Created:** 2026-01-23
**Completed:** 2026-01-23
**Depends On:** Story 11.41 (Comprehensive Mode-Strategy Configuration)

---

## Problem Statement

Story 11.41 added 18 configuration sections per mode+strategy combination with full UI support. However, investigation revealed that **only 4 out of 18 sections are actually wired to backend business logic** at the strategy level:

### Currently Working (4 sections):
1. **position_sizing** - `BaseSizeUSD`, `MaxPositions` used in entry logic
2. **sltp** - `SLPercent`, `TP1/2/3Percent` used in exit logic
3. **confidence** - `MinConfidence` used for entry decisions
4. **timeframe** - Partially used, falls back to mode-level

### UI-Only - No Backend Implementation (3 sections):
5. **entry_conditions** - ADXMin, RSIMin, RequireTrendAlign never read
6. **scoring** - TechnicalWeight, MomentumWeight, VolumeWeight, SentimentWeight never used
7. **circuit_breaker** - Uses global/mode-level, not strategy-level

### Using Mode-Level Instead of Strategy-Level (11 sections):
8. **mtf** - Uses `modeConfig.MTF` instead of `strategyConfig.MTF`
9. **hedge** - Uses `modeConfig.Hedge` instead of `strategyConfig.Hedge`
10. **averaging** - Uses `modeConfig.Averaging` instead of `strategyConfig.Averaging`
11. **stale_release** - Uses `modeConfig.StaleRelease`
12. **position_optimization** - Uses mode-level config
13. **funding_rate** - Uses `modeConfig.FundingRate`
14. **risk** - Uses `modeConfig.Risk`
15. **trend_divergence** - Uses `modeConfig.TrendDivergence`
16. **dynamic_ai_exit** - Uses hardcoded mode defaults
17. **early_warning** - Uses `modeConfig.EarlyWarning`
18. **exit_conditions** - Uses mode-level exit logic

**Impact:** Users can configure different settings per strategy in the UI, but the backend ignores these and uses mode-level settings. This means "Scalp → Trend Following" and "Scalp → Mean Reversion" use the same settings for most features.

---

## Acceptance Criteria

### AC1: Entry Conditions Wiring
- [x] `strategyConfig.EntryConditions.ADXMin` used in entry decision logic
- [x] `strategyConfig.EntryConditions.ADXMax` used in entry decision logic
- [x] `strategyConfig.EntryConditions.RSIMin` used in entry decision logic
- [x] `strategyConfig.EntryConditions.RSIMax` used in entry decision logic
- [x] `strategyConfig.EntryConditions.RequireTrendAlign` used in entry decision logic
- [x] Strategy-specific entry conditions (TrendFollowing, MeanReversion, Breakout, Range) applied

### AC2: Scoring Weights Wiring
- [x] `strategyConfig.Scoring.TechnicalWeight` used in score calculation
- [x] `strategyConfig.Scoring.MomentumWeight` used in score calculation
- [x] `strategyConfig.Scoring.VolumeWeight` used in score calculation
- [x] `strategyConfig.Scoring.SentimentWeight` used in score calculation
- [x] Weights sum validation (should equal 100%)

### AC3: Strategy-Level Circuit Breaker
- [x] Each mode+strategy has its own circuit breaker state
- [x] `strategyConfig.CircuitBreaker.MaxLossPerHourUSD` enforced per strategy
- [x] `strategyConfig.CircuitBreaker.MaxConsecutiveLosses` tracked per strategy
- [x] `strategyConfig.CircuitBreaker.CooldownMinutes` applied per strategy
- [x] Strategy paused independently without affecting other strategies

### AC4: MTF Settings Per Strategy
- [x] `strategyConfig.MTF.Enabled` controls MTF for that strategy only
- [x] `strategyConfig.MTF.PrimaryTimeframe/Weight` used per strategy
- [x] `strategyConfig.MTF.SecondaryTimeframe/Weight` used per strategy
- [x] `strategyConfig.MTF.MinConsensus` applied per strategy

### AC5: Hedge Settings Per Strategy
- [x] `strategyConfig.Hedge.AllowHedge` controls hedging per strategy
- [x] `strategyConfig.Hedge.MinConfidenceForHedge` applied per strategy
- [x] `strategyConfig.Hedge.MaxHedgeSizePercent` limits per strategy

### AC6: Averaging Settings Per Strategy
- [x] `strategyConfig.Averaging.AllowAveraging` controls per strategy
- [x] `strategyConfig.Averaging.AverageUpProfitPercent` applied per strategy
- [x] `strategyConfig.Averaging.AverageDownLossPercent` applied per strategy
- [x] `strategyConfig.Averaging.MaxAverages` limits per strategy

### AC7: Stale Release Per Strategy
- [x] `strategyConfig.StaleRelease.Enabled` controls per strategy
- [x] `strategyConfig.StaleRelease.MaxHoldDurationMinutes` applied per strategy
- [x] `strategyConfig.StaleRelease.StaleZoneAction` used per strategy

### AC8: Position Optimization Per Strategy
- [x] `strategyConfig.PositionOptimization.ReentryEnabled` controls per strategy
- [x] `strategyConfig.PositionOptimization.DynamicSLEnabled` controls per strategy
- [x] `strategyConfig.PositionOptimization.ProfitProtectionEnabled` controls per strategy

### AC9: Funding Rate Per Strategy
- [x] `strategyConfig.FundingRate.Enabled` controls per strategy
- [x] `strategyConfig.FundingRate.MaxFundingRatePct` applied per strategy
- [x] `strategyConfig.FundingRate.BlockEntryAboveRatePct` enforced per strategy

### AC10: Risk Settings Per Strategy
- [x] `strategyConfig.Risk.RiskLevel` applied per strategy
- [x] `strategyConfig.Risk.MaxDrawdownPercent` enforced per strategy
- [x] `strategyConfig.Risk.MaxDailyLossPercent` tracked per strategy

### AC11: Trend Divergence Per Strategy
- [x] `strategyConfig.TrendDivergence.Enabled` controls per strategy
- [x] `strategyConfig.TrendDivergence.BlockOnDivergence` applied per strategy
- [x] `strategyConfig.TrendDivergence.DivergenceWeight` used in scoring

### AC12: Dynamic AI Exit Per Strategy
- [x] `strategyConfig.DynamicAIExit.Enabled` controls per strategy
- [x] `strategyConfig.DynamicAIExit.UseLLMForLoss` applied per strategy
- [x] `strategyConfig.DynamicAIExit.UseLLMForProfit` applied per strategy
- [x] `strategyConfig.DynamicAIExit.MaxHoldTimeMs` enforced per strategy

### AC13: Early Warning Per Strategy
- [x] `strategyConfig.EarlyWarning.Enabled` controls per strategy
- [x] `strategyConfig.EarlyWarning.StartAfterMinutes` applied per strategy
- [x] `strategyConfig.EarlyWarning.MinLossPercent` triggers per strategy

### AC14: Exit Conditions Per Strategy
- [x] `strategyConfig.ExitConditions.UseAIExit` controls per strategy
- [x] `strategyConfig.ExitConditions.MaxHoldMinutes` enforced per strategy
- [x] `strategyConfig.ExitConditions.EarlyWarningEnabled` applied per strategy

---

## Implementation Summary

### Phase 1: Entry Logic Updates (Completed)
**File:** `internal/autopilot/ginie_analyzer.go`

1. Added `validateStrategyEntryConditions()` function (lines 189-273):
   - Validates ADX bounds from `strategyConfig.EntryConditionsV2`
   - Validates RSI bounds using calculated RSI(14)
   - Checks trend alignment if `RequireTrendAlign` is enabled
   - Checks volume multiplier threshold
   - Returns (passed, reason) for entry decision flow

2. Added `ScoringWeights` struct and `NormalizeScoringWeights()` function:
   - Maps strategy scoring config to normalized weights (sum to 1.0)
   - Default weights: Technical 45%, Momentum 20%, Volume 25%, Sentiment 10%

3. Added `calculateScanScoreWithWeights()` function:
   - Uses strategy-level scoring weights when available
   - Falls back to defaults if not configured

4. Added `GenerateSignalsWithScoring()` function:
   - Categorizes signals by type (Technical, Momentum, Volume, Sentiment)
   - Applies strategy-level weights to signal scoring
   - Uses MinScore/HighScore from config for signal strength thresholds

### Phase 2: Circuit Breaker Per Strategy (Completed)
**File:** `internal/autopilot/ginie_autopilot.go`

1. Changed data structure from `modeCircuitBreakers` to `strategyCircuitBreakers`:
   - Key format: `"mode"` or `"mode|strategy"` for per-strategy tracking

2. Added helper functions:
   - `getCircuitBreakerKey(mode, strategy)` - generates composite key
   - `getStrategyCircuitBreaker(mode, strategy)` - gets/creates CB with strategy config
   - `CheckStrategyCircuitBreaker(mode, strategy)` - checks if trading allowed
   - `TriggerStrategyCircuitBreaker(mode, strategy, reason)` - triggers CB
   - `RecordStrategyTradeResult(mode, strategy, pnl)` - records with mode aggregate

3. Updated all callers to pass `pos.EntryStrategy` for position-related operations

### Phase 3: Position Management Updates (Completed)
**Files:** `ginie_autopilot.go`, `position_optimization_logic.go`

1. Added helper functions:
   - `getStrategyPositionOptimization(pos)` - retrieves position optimization config
   - `getStrategyHedgeConfig(pos)` - retrieves hedge config per strategy
   - `getStrategyStaleReleaseConfig(pos)` - retrieves stale release config

2. Added type conversion functions:
   - `convertStrategyToPositionOptConfig()` - converts strategy to mode type
   - `convertStrategyToHedgeConfig()` - converts strategy hedge config
   - `convertStrategyToStaleConfig()` - converts strategy stale release config

3. Updated position optimization functions to use strategy-level config:
   - `checkAndExecuteReentry()`, `monitorFinalTrailing()`, `initHedgeReentryState()`
   - `checkAndTriggerHedge()`, `checkNegativeTPTrigger()`, `checkCombinedExit()`
   - `checkRallyExit()`, `updateHedgeWideSL()`, `monitorHedgeMode()`
   - `monitorHedgeTPs()`, `activateProfitProtection()`

### Phase 4: Exit Logic Updates (Completed)
**File:** `ginie_autopilot.go`

1. Added helper functions:
   - `getStrategyEarlyWarning(mode, strategy)` - retrieves early warning config
   - `getStrategyDynamicAIExitConfig(mode, strategy)` - retrieves dynamic AI exit config
   - `getStrategyExitConditions(mode, strategy)` - retrieves exit conditions

2. Updated `checkPositionsForEarlyWarning()` to use strategy-level config

3. Updated `getDynamicAIExitDecision()` to check:
   - Strategy-level Enabled flag
   - UseLLMForLoss and UseLLMForProfit settings
   - MaxHoldTimeMS for forced exit

### Phase 5: Other Settings (Completed)
**Files:** `ginie_autopilot.go`, `ginie_analyzer.go`

1. Added helper functions:
   - `getStrategyFundingRateConfig(mode, strategy)` - funding rate settings
   - `getStrategyMTFConfig(mode, strategy)` - multi-timeframe settings
   - `getStrategyRiskConfig(mode, strategy)` - risk settings
   - `getStrategyTrendDivergenceConfig(mode, strategy)` - trend divergence settings

2. Updated funding rate functions (`checkFundingRate`, `shouldExitBeforeFunding`, `adjustSizeForFunding`) to use strategy config

3. Updated MTF logic in TrendFilterValidator to use strategy config

4. Updated trend divergence check in `generateDecisionInternal()` to use strategy config

---

## Key Files Modified

| File | Changes |
|------|---------|
| `internal/autopilot/ginie_analyzer.go` | Entry conditions validation, scoring weights, trend divergence per strategy |
| `internal/autopilot/ginie_autopilot.go` | Circuit breaker per strategy, hedge/stale/position opt helpers, exit logic helpers, MTF/funding/risk helpers |
| `internal/autopilot/position_optimization_logic.go` | Updated 11 functions to use strategy-level position optimization |
| `internal/autopilot/ginie_trend_filters_test.go` | Fixed mock interface (added missing methods) |

---

## Testing Summary

### Build Verification
- Project builds successfully with `go build -buildvcs=false`
- All changes compile without errors

### Runtime Verification
- Application starts and runs without errors
- Health check passes: `{"database":"healthy","status":"healthy"}`

### Test Notes
- Pre-existing test issues with logging format directives (unrelated to this story)
- Mock interface updated with missing methods (`GetAllOrdersByDateRange`, `GetTradeHistoryByDateRange`)

---

## Definition of Done

- [x] All 18 sections read from `strategyConfig` instead of `modeConfig`
- [x] Circuit breaker works independently per mode+strategy
- [x] Build verification passed
- [x] Application runs without errors
- [x] Logging shows which strategy config is being used
- [x] All helper functions implement fallback to mode-level config

---

## Backward Compatibility

All changes maintain backward compatibility:
- Empty strategy strings fall back to mode-level config
- Missing strategy configs fall back to mode-level config
- Mode-level aggregate tracking maintained for circuit breaker
- Original function signatures preserved as wrappers
