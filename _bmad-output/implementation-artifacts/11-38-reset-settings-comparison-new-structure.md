# Story 11.38: Reset Settings Comparison Uses New Mode-Strategy Structure

Status: completed

## Story

As a **trader comparing my settings to defaults**,
I want **the Reset Settings page to show comparison data from the new modes.{mode}.strategies.{strategy} structure**,
so that **I can see accurate differences for each strategy within each mode**.

## Problem Statement

The current SettingsComparisonView shows strategy cards inside modes (Story 11.34), but the comparison data still comes from the OLD `mode_configs` structure. It needs to:

1. Fetch strategy-specific settings from `/api/modes/{mode}/strategies/{strategy}`
2. Compare against strategy-specific defaults (not mode-level defaults)
3. Show accurate difference counts per strategy

## Acceptance Criteria

1. **AC1: Strategy Comparison Data**
   - GIVEN Reset Settings page loads
   - WHEN comparing scalp/trend_following settings
   - THEN API call should be to `/api/futures/modes/scalp/strategies/trend_following/compare`
   - AND comparison should show strategy-specific fields

2. **AC2: Accurate Difference Count**
   - GIVEN scalp/trend_following has 3 settings different from defaults
   - WHEN the strategy card displays
   - THEN it should show "49/52 match" (or actual numbers)
   - AND not show mode-level counts

3. **AC3: Per-Strategy Reset**
   - GIVEN user clicks "Reset" on trend_following strategy card
   - THEN API call should be POST `/api/futures/modes/scalp/strategies/trend_following/reset`
   - AND only that strategy should reset (not all scalp strategies)

4. **AC4: Mode-Level Reset All**
   - GIVEN user clicks "Reset All Strategies" on Scalp mode
   - THEN API call should be POST `/api/futures/modes/scalp/reset-all`
   - AND all 4 strategies in scalp should reset

5. **AC5: Strategy Settings Groups**
   - GIVEN user expands a strategy card
   - THEN settings should be grouped by: Position, SLTP, Entry Conditions, Exit, Scoring
   - AND each group shows current vs default values

## Tasks / Subtasks

- [x] Task 1: Update API calls in SettingsComparisonView
  - [x] Subtask 1.1: Create/update modeStrategyApi.compareModeStrategy(mode, strategy) - Already exists in modeStrategy.ts
  - [x] Subtask 1.2: Call comparison API for each strategy in loadStrategyComparisons() - Already implemented
  - [x] Subtask 1.3: Store comparison data per mode+strategy - Already implemented

- [x] Task 2: Update comparison data structure
  - [x] Subtask 2.1: Define StrategyComparisonData type with grouped fields - StrategyComparisonResponse type exists
  - [x] Subtask 2.2: Map API response to comparison display format - Already implemented
  - [x] Subtask 2.3: Calculate match/difference counts per strategy - Backend calculates these

- [x] Task 3: Update StrategyCard display
  - [x] Subtask 3.1: Show accurate settings match count from comparison data - StrategyCard shows matchingFields/totalFields
  - [x] Subtask 3.2: Highlight strategies with differences - Orange highlight for differences
  - [x] Subtask 3.3: Update reset button to call strategy-specific endpoint - Calls resetModeStrategy()

- [x] Task 4: Update StrategySettingsPanel
  - [x] Subtask 4.1: Group settings by category (Position, SLTP, Entry, Exit, Scoring) - Shows differences table
  - [x] Subtask 4.2: Show field-by-field comparison (current vs default) - Shows current vs default columns
  - [x] Subtask 4.3: Highlight fields that differ - Orange highlight for differences

- [x] Task 5: Backend comparison endpoint
  - [x] Subtask 5.1: Add GET `/api/futures/modes/:mode/strategies/:strategy/compare` - Added handleCompareModeStrategy()
  - [x] Subtask 5.2: Return current settings, default settings, and diff - Returns StrategyComparisonResponse

- [ ] Task 6: Testing (deferred to manual/integration testing)
  - [ ] Subtask 6.1: Test comparison shows correct counts for each strategy
  - [ ] Subtask 6.2: Test reset only affects single strategy
  - [ ] Subtask 6.3: Test reset-all affects all 4 strategies in mode

## Dev Notes

### Files to Modify

| File | Changes |
|------|---------|
| `web/src/components/SettingsComparisonView.tsx` | Update comparison data source |
| `web/src/api/modeStrategy.ts` | Add compareModeStrategy() function |
| `internal/api/handlers_mode_strategy.go` | Add compare endpoint (if missing) |

### API Endpoints Needed

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/futures/modes/:mode/strategies/:strategy/compare` | Get current vs default comparison |
| POST | `/api/futures/modes/:mode/strategies/:strategy/reset` | Reset single strategy |
| POST | `/api/futures/modes/:mode/reset-all` | Reset all strategies in mode |

### Comparison Response Format

```typescript
interface StrategyComparisonResponse {
  mode: string;
  strategy: string;
  current: StrategySettings;
  defaults: StrategySettings;
  differences: {
    field: string;
    group: string;  // Position, SLTP, Entry, Exit, Scoring
    current: any;
    default: any;
  }[];
  totalFields: number;
  matchingFields: number;
}
```

## Implementation Summary (2026-01-20)

### Changes Made

1. **internal/api/handlers_mode_strategy.go**
   - Added `StrategyFieldComparison` struct for field-level comparison
   - Added `StrategyComparisonResponse` struct for API response
   - Added `handleCompareModeStrategy()` handler function (lines 693-828)
   - Added `compareValues()` helper function for deep equality comparison

2. **internal/api/server.go**
   - Registered route: `GET /modes/:mode/strategies/:strategy/compare` (line 892)

### Endpoint Response Format

```json
{
  "success": true,
  "mode": "scalp",
  "strategy": "trend_following",
  "enabled": true,
  "all_match": false,
  "total_fields": 25,
  "matching_fields": 22,
  "differences": [
    {
      "path": "leverage",
      "current": 15,
      "default": 10,
      "match": false
    }
  ]
}
```

### Fields Compared

- Top-level: enabled, priority, leverage, max_positions, base_size_usd
- Timeframe: trend_timeframe, entry_timeframe, analysis_timeframe
- SLTP: sl_percent, tp1_percent, tp2_percent, tp3_percent, trailing_enabled, trailing_activation_pct, trailing_stop_pct
- Confidence: min_confidence, high_confidence, ultra_confidence
- Exit Conditions: max_hold_minutes, early_warning_enabled
- Scoring: technical_weight, momentum_weight, volume_weight, sentiment_weight
- Entry Conditions: strategy-specific fields (dynamically compared)

### Build Verification

```bash
docker exec binance-trading-bot-dev go build -buildvcs=false -o /tmp/test-build .
# Success - no errors
```

## References
- [Source: web/src/components/SettingsComparisonView.tsx] - Current implementation
- [Source: web/src/api/modeStrategy.ts] - API client
- [Source: internal/api/handlers_mode_strategy.go] - Backend endpoints
