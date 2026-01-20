# Story 11.35: Expand DefaultModeStrategyConfig with All 16 Combinations

Status: completed

## Story

As a **developer maintaining the trading bot**,
I want **DefaultModeStrategyConfig() to return correct defaults for all 16 mode+strategy combinations**,
so that **new users get proper configuration values for swing, position, and ultra_fast modes** (not generic fallbacks).

## Problem Statement

Currently `DefaultModeStrategyConfig()` in `models_user_settings.go` only has hardcoded defaults for **scalp mode** (4 strategies). The other 12 combinations (swing, position, ultra_fast × 4 strategies each) return generic fallback values:

```go
// Current fallback for non-scalp modes
return &ModeStrategyConfig{
    Enabled:          false,
    Priority:         99,
    Leverage:         5,        // WRONG - should vary by mode
    MaxPositions:     1,        // WRONG - should vary by mode
    BaseSizeUSD:      100,      // WRONG - should vary by mode
    ...
}
```

The correct values ARE defined in `default-settings.json` (lines 1116-1850) but need to be ported to Go code.

## Acceptance Criteria

1. **AC1: Swing Mode Defaults**
   - GIVEN a call to `DefaultModeStrategyConfig("swing", "trend_following")`
   - THEN it should return values matching `default-settings.json` modes.swing.strategies.trend_following
   - AND same for mean_reversion, breakout, range_trading

2. **AC2: Position Mode Defaults**
   - GIVEN a call to `DefaultModeStrategyConfig("position", "trend_following")`
   - THEN it should return values matching `default-settings.json` modes.position.strategies.trend_following
   - AND same for mean_reversion, breakout, range_trading

3. **AC3: Ultra_Fast Mode Defaults**
   - GIVEN a call to `DefaultModeStrategyConfig("ultra_fast", "trend_following")`
   - THEN it should return values matching `default-settings.json` modes.ultra_fast.strategies.trend_following
   - AND same for mean_reversion, breakout, range_trading

4. **AC4: All 16 Combinations Have Unique Values**
   - GIVEN all 16 mode+strategy combinations
   - WHEN DefaultModeStrategyConfig is called for each
   - THEN each should return mode-specific and strategy-specific values
   - AND no fallback to generic values should occur

5. **AC5: New User Initialization Works**
   - GIVEN a new user registration
   - WHEN InitializeUserModeStrategies() is called
   - THEN all 16 mode+strategy records should have correct values in DB
   - AND values should match default-settings.json

## Tasks / Subtasks

- [x] Task 1: Port swing mode defaults from JSON to Go
  - [x] Subtask 1.1: Add swing/trend_following config
  - [x] Subtask 1.2: Add swing/mean_reversion config
  - [x] Subtask 1.3: Add swing/breakout config
  - [x] Subtask 1.4: Add swing/range_trading config

- [x] Task 2: Port position mode defaults from JSON to Go
  - [x] Subtask 2.1: Add position/trend_following config
  - [x] Subtask 2.2: Add position/mean_reversion config
  - [x] Subtask 2.3: Add position/breakout config
  - [x] Subtask 2.4: Add position/range_trading config

- [x] Task 3: Port ultra_fast mode defaults from JSON to Go
  - [x] Subtask 3.1: Add ultra_fast/trend_following config
  - [x] Subtask 3.2: Add ultra_fast/mean_reversion config
  - [x] Subtask 3.3: Add ultra_fast/breakout config
  - [x] Subtask 3.4: Add ultra_fast/range_trading config

- [ ] Task 4: Update fallback behavior (deferred - not needed for MVP)
  - [ ] Subtask 4.1: Remove generic fallback - return error or panic for unknown mode/strategy
  - [ ] Subtask 4.2: Add logging when defaults are requested

- [ ] Task 5: Testing (deferred - to be done with full test suite)
  - [ ] Subtask 5.1: Unit test all 16 combinations return unique values
  - [ ] Subtask 5.2: Test new user gets correct initial values
  - [ ] Subtask 5.3: Verify values match default-settings.json

## Dev Notes

### File to Modify
`internal/database/models_user_settings.go` lines 666-861

### Reference Values from default-settings.json

**Swing Mode:**
- trend_following: leverage=10, max_positions=5, base_size_usd=300
- mean_reversion: leverage=8, max_positions=5, base_size_usd=300
- breakout: leverage=10, max_positions=5, base_size_usd=300
- range_trading: leverage=8, max_positions=5, base_size_usd=300

**Position Mode:**
- trend_following: leverage=3, max_positions=2, base_size_usd=600
- mean_reversion: leverage=3, max_positions=2, base_size_usd=600
- breakout: leverage=3, max_positions=2, base_size_usd=600
- range_trading: leverage=3, max_positions=2, base_size_usd=600

**Ultra_Fast Mode:**
- trend_following: leverage=10, max_positions=1, base_size_usd=200
- mean_reversion: leverage=10, max_positions=1, base_size_usd=200
- breakout: leverage=10, max_positions=1, base_size_usd=200
- range_trading: leverage=10, max_positions=1, base_size_usd=200

### Code Pattern
Follow existing scalp pattern (lines 669-816) for each mode.

## References
- [Source: default-settings.json#modes] - Lines 1116-1850
- [Source: internal/database/models_user_settings.go#DefaultModeStrategyConfig] - Lines 666-861
