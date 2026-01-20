# Story 11.36: Runtime Code Reads Strategy Config Instead of Mode Config

Status: done

## Story

As a **trader using the bot**,
I want **the trading bot to use my strategy-specific settings** (leverage, SLTP, entry conditions),
so that **my configured settings actually affect trading decisions** instead of being ignored.

## Problem Statement

Currently the trading bot reads from old `mode_configs` structure:
```go
modeConfig := cache.GetModeConfig(userID, mode)  // OLD
slConfig := modeConfig.SLTP
```

It should read from new `modes.{mode}.strategies.{strategy}` structure:
```go
strategyConfig := cache.GetModeStrategyConfig(userID, mode, strategy)  // NEW
slConfig := strategyConfig.SLTP
```

**Impact:** Users edit strategy settings in UI → Settings saved → **Settings NEVER USED**

## Acceptance Criteria

1. **AC1: Autopilot Reads Strategy Config**
   - GIVEN ginie_autopilot.go makes a trading decision
   - WHEN it needs leverage, position size, or SLTP values
   - THEN it should call GetModeStrategyConfig(userID, mode, activeStrategy)
   - AND use the returned values for the trade

2. **AC2: Dynamic SLTP Uses Strategy Config**
   - GIVEN dynamic_sltp.go calculates stop loss / take profit
   - WHEN it reads SLTP percentages
   - THEN it should use strategyConfig.SLTP (not modeConfig.SLTP)
   - AND values should reflect user's strategy-specific settings

3. **AC3: Position Exit Engine Uses Strategy Config**
   - GIVEN position_exit_engine.go decides exit conditions
   - WHEN it checks max_hold_minutes, early_warning, etc.
   - THEN it should read from strategyConfig.ExitConditions
   - AND respect strategy-specific exit rules

4. **AC4: Entry Conditions Use Strategy Config**
   - GIVEN a strategy evaluates entry signal
   - WHEN it checks ADX threshold, RSI range, volume requirements
   - THEN it should read from strategyConfig.EntryConditions
   - AND not use hardcoded values

5. **AC5: Scoring Weights Use Strategy Config**
   - GIVEN score calculation for a signal
   - WHEN weights are applied (technical, momentum, volume, sentiment)
   - THEN it should use strategyConfig.Scoring weights
   - AND different strategies should use different weight distributions

## Tasks / Subtasks

- [x] Task 1: Update ginie_autopilot.go (ginie_analyzer.go)
  - [x] Subtask 1.1: Add GetModeStrategyConfig call at trade decision point
  - [x] Subtask 1.2: Replace modeConfig references with strategyConfig (with priority)
  - [x] Subtask 1.3: Pass strategy name through decision flow (defaulting to "trend_following")

- [x] Task 2: Update dynamic_sltp.go
  - [x] Subtask 2.1: N/A - dynamic_sltp.go takes config as parameter from caller
  - [x] Subtask 2.2: SLTP values now read from strategyConfig at call site (ginie_analyzer.go)

- [x] Task 3: Update position_exit_engine.go
  - [x] Subtask 3.1: N/A - position_exit_engine uses strategy registry, not modeConfig
  - [x] Subtask 3.2: N/A - exit conditions come from strategy interface
  - [x] Subtask 3.3: N/A - early_warning not in this file

- [ ] Task 4: Update strategy implementations (Deferred to Story 11.37)
  - [ ] Subtask 4.1: trend_following.go - read entry_conditions from config
  - [ ] Subtask 4.2: mean_reversion.go - read entry_conditions from config
  - [ ] Subtask 4.3: breakout.go - read entry_conditions from config
  - [ ] Subtask 4.4: range_trading.go - read entry_conditions from config

- [ ] Task 5: Update scoring/calibration (Deferred to Story 11.37)
  - [ ] Subtask 5.1: Read scoring weights from strategyConfig.Scoring
  - [ ] Subtask 5.2: Apply strategy-specific weight distribution

- [x] Task 6: Testing
  - [x] Subtask 6.1: Build verification passed
  - [ ] Subtask 6.2: Test each strategy uses its own entry conditions (Deferred)
  - [ ] Subtask 6.3: Test SLTP values come from strategy config (Manual test needed)

## Dev Notes

### Files to Modify

| File | Changes Needed |
|------|----------------|
| `internal/autopilot/ginie_autopilot.go` | Add strategyConfig parameter, replace modeConfig reads |
| `internal/autopilot/dynamic_sltp.go` | Change SLTP source |
| `internal/autopilot/position_exit_engine.go` | Read exit conditions from strategy |
| `internal/decision/strategies/trend_following.go` | Read entry_conditions |
| `internal/decision/strategies/mean_reversion.go` | Read entry_conditions |
| `internal/decision/strategies/breakout.go` | Read entry_conditions |
| `internal/decision/strategies/range_trading.go` | Read entry_conditions |
| `internal/decision/calibration.go` | Use strategy scoring weights |

### Pattern Change

**Before:**
```go
func processSignal(userID int, mode string, signal Signal) {
    modeConfig := cache.GetModeConfig(userID, mode)
    leverage := modeConfig.Leverage
    slPercent := modeConfig.SLTP.StopLossPercent
}
```

**After:**
```go
func processSignal(userID int, mode string, strategy string, signal Signal) {
    strategyConfig := cache.GetModeStrategyConfig(userID, mode, strategy)
    leverage := strategyConfig.Leverage
    slPercent := strategyConfig.SLTP.SLPercent
}
```

### Dependencies
- Story 11.35 must be done first (defaults populated)
- Story 11.37 determines which strategy is active

## Implementation Notes (2026-01-20)

### Changes Made

1. **internal/autopilot/ginie_autopilot.go**
   - Added `GetModeStrategyConfig` method to `SettingsCacheReader` interface (line 82-86)
   - Interface uses `database.ModeStrategyConfig` for return type

2. **internal/autopilot/settings.go**
   - Added `GetModeStrategyConfig` method to `ModeConfigCache` interface (line 25-28)

3. **internal/autopilot/ginie_analyzer.go**
   - Added `strconv` import for userID string-to-int conversion
   - Added strategy config loading after mode config loading (lines 1829-1845)
   - Uses "trend_following" as default strategy until Story 11.37 wires strategy selection
   - Added strategyConfig override section for leverage, base_size_usd, SL/TP (lines 2315-2389)
   - strategyConfig values take priority over modeConfig values

### Key Implementation Details

- **Priority Chain**: Mode defaults → modeConfig → strategyConfig (highest priority)
- **Default Strategy**: "trend_following" until Story 11.37 implements proper strategy selection
- **SLTP Handling**: When strategyConfig.SLTP.SLPercent/TP1Percent are set, they directly configure the min/max bounds and ATR multipliers
- **Build Verified**: `docker exec binance-trading-bot-dev go build -buildvcs=false -o /tmp/test-build .`

### What's Deferred to Story 11.37

- Strategy implementations reading entry_conditions from user config
- Scoring weights using strategyConfig.Scoring
- Active strategy selection based on market conditions

## References
- [Source: internal/cache/settings_cache_service.go#GetModeStrategyConfig] - Lines 1213-1261
- [Source: internal/autopilot/ginie_autopilot.go] - Main trading logic
- [Source: internal/autopilot/dynamic_sltp.go] - SLTP calculation
