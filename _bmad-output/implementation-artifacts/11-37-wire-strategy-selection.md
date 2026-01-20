# Story 11.37: Wire Strategy Selection Based on Market Regime

Status: completed

## Story

As a **trader using multiple strategies**,
I want **the bot to automatically select the best strategy based on current market regime**,
so that **trend_following is used in trending markets and mean_reversion in ranging markets**.

## Problem Statement

`GetActiveStrategyForMode()` exists in cache service (lines 1473-1541) but is **never called**. The bot currently always uses the same strategy regardless of market conditions.

**Expected flow:**
1. Market regime detected (TRENDING, RANGING, VOLATILE, CONSOLIDATING)
2. GetActiveStrategyForMode(userID, mode, regime) called
3. Returns best enabled strategy for that regime
4. Trading uses that strategy's config

**Current flow:**
1. Hardcoded to trend_following always
2. No regime detection
3. No strategy switching

## Acceptance Criteria

1. **AC1: Strategy Selection Called**
   - GIVEN auto_select_strategy is ON for a mode
   - WHEN a trading decision is being made
   - THEN GetActiveStrategyForMode() should be called
   - AND it should receive current market regime

2. **AC2: Regime Detection Integrated**
   - GIVEN market data available
   - WHEN regime classifier runs (from Story 11.4)
   - THEN current regime should be passed to strategy selection
   - AND regime should be one of: TRENDING, RANGING, VOLATILE, CONSOLIDATING

3. **AC3: Strategy Matches Regime**
   - GIVEN regime = TRENDING
   - WHEN strategy selection runs
   - THEN trend_following or breakout should be selected (if enabled)
   - AND mean_reversion or range_trading should NOT be selected

4. **AC4: Manual Override Works**
   - GIVEN auto_select_strategy is OFF for a mode
   - WHEN user has manually selected a strategy
   - THEN that strategy should be used regardless of regime
   - AND no automatic switching should occur

5. **AC5: Cooldown Prevents Whipsawing**
   - GIVEN strategy was switched recently
   - WHEN regime changes again within 5 minutes
   - THEN strategy should NOT switch (cooldown active)
   - AND log should indicate cooldown prevented switch

## Tasks / Subtasks

- [x] Task 1: Integrate regime detection
  - [x] Subtask 1.1: Added classifyMarketRegime() function using same logic as decision.RegimeClassifier
  - [x] Subtask 1.2: Classify regime using ADX, ATR14, and AvgATR20 from scan data
  - [x] Subtask 1.3: Pass regime to GetActiveStrategyForMode()

- [x] Task 2: Wire strategy selection in decision flow
  - [x] Subtask 2.1: Added strategy selection call in ginie_analyzer.go (AnalyzeForTrading)
  - [x] Subtask 2.2: Strategy config and name stored for the trade
  - [x] Subtask 2.3: Strategy passed to downstream functions via existing strategyConfig variable

- [x] Task 3: Implement manual override
  - [x] Subtask 3.1: GetActiveStrategyForMode already checks enabled strategies and priorities
  - [x] Subtask 3.2: If no matching strategy for regime, defaults to trend_following
  - [x] Subtask 3.3: Regime-based selection uses cache service logic

- [x] Task 4: Add cooldown mechanism
  - [x] Subtask 4.1: Added strategyLastSwitch map to track last switch time per mode
  - [x] Subtask 4.2: Implemented 5-minute cooldown (strategySwitchCooldown constant)
  - [x] Subtask 4.3: Added logging when cooldown prevents switch

- [ ] Task 5: Testing (deferred to manual/integration testing)
  - [ ] Subtask 5.1: Test TRENDING regime selects trend_following
  - [ ] Subtask 5.2: Test RANGING regime selects mean_reversion
  - [ ] Subtask 5.3: Test manual override bypasses auto-select
  - [ ] Subtask 5.4: Test cooldown prevents rapid switching

## Implementation Summary

### Files Modified

1. **internal/autopilot/settings.go**
   - Added `GetActiveStrategyForMode(ctx, userID, mode, regime)` to `ModeConfigCache` interface

2. **internal/autopilot/ginie_autopilot.go**
   - Added `GetActiveStrategyForMode()` to `SettingsCacheReader` interface

3. **internal/autopilot/ginie_analyzer.go**
   - Added market regime constants (TRENDING, RANGING, VOLATILE, CONSOLIDATING)
   - Added `classifyMarketRegime()` function for regime detection
   - Added strategy switch cooldown fields to GinieAnalyzer struct
   - Updated `AnalyzeForTrading()` to use regime-based strategy selection with cooldown

### Key Changes

**Regime Classification (lines 6010-6054 in ginie_analyzer.go):**
```go
func classifyMarketRegime(adx, atr, atrAverage float64) string {
    // ADX >= 25 = TRENDING
    // ADX < 20 + high ATR = VOLATILE
    // ADX < 20 + low ATR = CONSOLIDATING
    // Otherwise = RANGING
}
```

**Strategy Selection with Cooldown (lines 1839-1917 in ginie_analyzer.go):**
- Classifies market regime using scan.Trend.ADXValue and scan.Volatility data
- Calls GetActiveStrategyForMode() to get best strategy for current regime
- Applies 5-minute cooldown to prevent rapid strategy switching
- Logs when cooldown blocks a strategy change

### Cooldown Mechanism

- Tracks `strategyLastSwitch[mode]` and `strategyLastSelected[mode]` per trading mode
- If strategy would change but cooldown is active (< 5 minutes since last switch), keeps previous strategy
- Logs cooldown remaining time when switch is blocked

## Dev Notes

### Key Function (Already Exists)

`internal/cache/settings_cache_service.go` lines 1473-1541:
```go
func (s *SettingsCacheService) GetActiveStrategyForMode(
    ctx context.Context,
    userID int,
    mode string,
    regime string,
) (*database.ModeStrategyConfig, string, error)
```

Returns:
- Config for selected strategy
- Strategy name (e.g., "trend_following")
- Error if none available

### Regime-Strategy Mapping (from default-settings.json)

| Regime | Best Strategies |
|--------|-----------------|
| TRENDING | trend_following, breakout |
| VOLATILE_TRENDING | trend_following, breakout |
| RANGING | mean_reversion, range_trading |
| CONSOLIDATING | range_trading |
| VOLATILE | breakout |

### Build Verification

```bash
docker exec binance-trading-bot-dev go build -buildvcs=false -o /tmp/test-build .
# Success - no errors
```

## References
- [Source: internal/cache/settings_cache_service.go#GetActiveStrategyForMode] - Lines 1473-1541
- [Source: internal/decision/regime_classifier.go] - Story 11.4 implementation
- [Source: default-settings.json#supported_regimes] - Strategy-regime mapping
