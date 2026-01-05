# Epic: Mode Configuration as Single Source of Truth

## Epic ID: EPIC-MODE-001
## Status: In Progress
## Priority: P0 (Critical)
## Created: 2026-01-01
## Owner: Winston (Architect) + Amelia (Dev)

---

## Problem Statement

The trading bot has configuration values scattered across multiple locations, causing:
1. **Duplicate defaults** - Same values defined in 5+ locations
2. **State divergence** - `pos.RemainingQty` vs `sr.RemainingQuantity`
3. **Precision errors** - Scalp reentry mode missing precision handling already solved elsewhere
4. **No user visibility** - Users can't see what values do or change them easily

## Goals

1. **Single Source of Truth**: Mode config is the ONLY place defaults are defined
2. **Precision Reuse**: All modes use the same precision utilities
3. **User Configurability**: Users can view, understand, and modify all mode values
4. **Scalp Reentry Fix**: Resolve all precision/calculation bugs

---

## Success Criteria

- [ ] All mode defaults come from `DefaultModeConfigs()` only
- [ ] All precision handling uses `roundPriceForTP()`, `roundPriceForSL()`, `roundQuantity()`
- [ ] UI shows all mode config values with descriptions
- [ ] Users can edit and save mode config values
- [ ] Scalp reentry mode passes all precision tests
- [ ] Breakeven calculation is mathematically correct
- [ ] Quantity tracking uses single source (`sr.RemainingQuantity`)

---

## Stories

### Story 1: Fix Scalp Reentry Precision Errors
**ID**: STORY-001
**Points**: 8
**Priority**: P0

**Description**: Fix all precision handling issues in `scalp_reentry_logic.go` that cause API rejections and incorrect calculations.

**Acceptance Criteria**:
- [ ] AC1: TP price rounded with `roundPriceForTP()` before comparison (lines 79, 82)
- [ ] AC2: ReentryTargetPrice rounded after breakeven calculation (line 169)
- [ ] AC3: DynamicSLPrice rounded with `roundPriceForSL()` before storage (line 430)
- [ ] AC4: All quantity calculations checked against minimum qty
- [ ] AC5: Floating point division errors prevented by pre-rounding percentages

**Files to Modify**:
- `internal/autopilot/scalp_reentry_logic.go`

---

### Story 2: Fix Breakeven Calculation Bug
**ID**: STORY-002
**Points**: 5
**Priority**: P0

**Description**: The `calculateNewBreakeven()` function incorrectly calculates breakeven by not accounting for sold quantities.

**Current Bug** (lines 517-539):
```go
totalQty = OriginalQty + ReentryQtys  // WRONG!
// Doesn't subtract sold quantities
```

**Correct Calculation**:
```
breakeven = (original_cost - sold_value + reentry_costs) / remaining_qty
```

**Acceptance Criteria**:
- [ ] AC1: Breakeven calculation subtracts sold quantities from total
- [ ] AC2: Breakeven correctly accounts for partial closes at each TP level
- [ ] AC3: Unit test validates breakeven across multiple cycles
- [ ] AC4: Edge case: zero remaining qty returns entry price

**Files to Modify**:
- `internal/autopilot/scalp_reentry_logic.go`

---

### Story 3: Consolidate Quantity Tracking
**ID**: STORY-003
**Points**: 5
**Priority**: P1

**Description**: Eliminate quantity tracking divergence between `pos.RemainingQty` and `sr.RemainingQuantity`.

**Current State**:
- `GiniePosition.RemainingQty` - updated by main position logic
- `ScalpReentryStatus.RemainingQuantity` - updated by scalp reentry logic
- These diverge, causing SL orders to protect wrong quantities

**Solution**:
- Make `sr.RemainingQuantity` the authoritative source for scalp_reentry mode
- Sync `pos.RemainingQty` FROM `sr.RemainingQuantity` after each operation

**Acceptance Criteria**:
- [ ] AC1: Single quantity source for scalp_reentry mode
- [ ] AC2: SL orders always use correct remaining quantity
- [ ] AC3: Position close uses correct final quantity
- [ ] AC4: No race conditions in quantity updates

**Files to Modify**:
- `internal/autopilot/scalp_reentry_logic.go`
- `internal/autopilot/ginie_autopilot.go` (position sync)

---

### Story 4: Mode Config API with Value Descriptions
**ID**: STORY-004
**Points**: 8
**Priority**: P1

**Description**: Create API endpoints that return mode config values with human-readable descriptions so users understand what each setting does.

**New Endpoints**:
```
GET  /api/futures/mode-configs/schema     - Get all fields with descriptions
GET  /api/futures/mode-configs/:mode      - Get specific mode config
POST /api/futures/mode-configs/:mode      - Update mode config
GET  /api/futures/mode-configs/defaults   - Get factory defaults
POST /api/futures/mode-configs/:mode/reset - Reset to defaults
```

**Response Format**:
```json
{
  "mode": "scalp_reentry",
  "fields": {
    "sltp.stop_loss_percent": {
      "value": 2.0,
      "default": 2.0,
      "min": 0.1,
      "max": 10.0,
      "description": "Stop loss percentage from entry price. Higher = wider stop, more room for volatility but larger potential loss.",
      "category": "Risk Management"
    }
  }
}
```

**Acceptance Criteria**:
- [ ] AC1: Schema endpoint returns all fields with descriptions
- [ ] AC2: Update endpoint validates values within min/max bounds
- [ ] AC3: Reset endpoint restores factory defaults
- [ ] AC4: Changes persist to autopilot_settings.json
- [ ] AC5: Changes take effect immediately (no restart)

**Files to Modify**:
- `internal/api/handlers_mode.go`
- `internal/api/server.go`
- `internal/autopilot/settings.go` (add field metadata)

---

### Story 5: Remove Duplicate Defaults
**ID**: STORY-005
**Points**: 5
**Priority**: P2

**Description**: Audit and remove all duplicate default value definitions, making `DefaultModeConfigs()` the single source.

**Duplicates Found**:
1. `DefaultGinieAutopilotConfig()` - has SL/TP, confidence, size limits
2. `DefaultGinieConfig()` - has ADX thresholds, position limits
3. `DefaultScalpReentryConfig()` - has mode-specific defaults
4. Individual handler fallbacks in `handlers_mode.go`
5. Hardcoded constants in `ginie_analyzer.go`

**Solution**:
- All defaults flow from `DefaultModeConfigs()`
- Other functions call `GetModeConfig("mode_name")` instead of hardcoding
- Remove/refactor duplicate definitions

**Acceptance Criteria**:
- [ ] AC1: `DefaultModeConfigs()` is the only default source
- [ ] AC2: No hardcoded SL/TP percentages outside mode configs
- [ ] AC3: No hardcoded confidence thresholds outside mode configs
- [ ] AC4: Audit log documents all removed duplicates

**Files to Modify**:
- `internal/autopilot/settings.go`
- `internal/autopilot/ginie_autopilot.go`
- `internal/autopilot/ginie_types.go`
- `internal/autopilot/ginie_analyzer.go`
- `internal/api/handlers_mode.go`

---

### Story 6: Mode Config UI Component
**ID**: STORY-006
**Points**: 13
**Priority**: P2

**Description**: Create a React component that displays all mode config values with descriptions, allows editing, and saves changes.

**UI Features**:
- Accordion/tab layout for each mode (ultra_fast, scalp, scalp_reentry, swing, position)
- Grouped by category (Risk, Size, Timing, AI, etc.)
- Each field shows: current value, description, min/max bounds
- Inline editing with validation
- Save/Reset buttons per mode
- Visual diff from defaults (highlight changed values)

**Acceptance Criteria**:
- [ ] AC1: All 5 modes displayed with all fields
- [ ] AC2: Field descriptions visible on hover/expand
- [ ] AC3: Validation prevents out-of-bounds values
- [ ] AC4: Save persists to backend
- [ ] AC5: Reset restores defaults with confirmation
- [ ] AC6: Loading states during API calls

**Files to Create**:
- `web/src/components/ModeConfigEditor.tsx`
- `web/src/services/modeConfigApi.ts`

**Files to Modify**:
- `web/src/pages/GiniePage.tsx` (add component)

---

### Story 7: Comprehensive Scalp Reentry Testing
**ID**: STORY-007
**Points**: 8
**Priority**: P1

**Description**: Create comprehensive tests for scalp_reentry mode covering all precision, calculation, and state management scenarios.

**Test Scenarios**:
1. TP1/TP2/TP3 price calculations with precision
2. Reentry quantity calculations with minimum validation
3. Breakeven calculation across multiple cycles
4. Dynamic SL updates with precision
5. Final trailing stop calculations
6. State machine transitions (waiting -> executing -> completed)
7. Edge cases: zero qty, minimum qty, max cycles

**Acceptance Criteria**:
- [ ] AC1: Unit tests for all precision-sensitive functions
- [ ] AC2: Integration test for full TP1->Reentry->TP2->Reentry->TP3 cycle
- [ ] AC3: Edge case tests (min qty, max cycles, timeout)
- [ ] AC4: All tests pass with 100% coverage on critical paths
- [ ] AC5: Test with real symbol precision data (BTCUSDT 3 decimals, etc.)

**Files to Create**:
- `internal/autopilot/scalp_reentry_logic_test.go`
- `internal/autopilot/scalp_reentry_precision_test.go`

---

## Technical Notes

### Precision Handling Pattern (Reuse This)

```go
// For TP prices - use directional rounding
tpPrice := roundPriceForTP(symbol, calculatedTP, side)

// For SL prices - use protective rounding
slPrice := roundPriceForSL(symbol, calculatedSL, side)

// For quantities - always floor and check minimum
qty := roundQuantity(symbol, calculatedQty)
if qty < getMinQuantity(symbol) {
    return ErrQuantityBelowMinimum
}
```

### Breakeven Calculation (Correct Formula)

```go
func calculateNewBreakeven(pos *GiniePosition, sr *ScalpReentryStatus) float64 {
    // Track net cost and net quantity
    netCost := pos.EntryPrice * pos.OriginalQty
    netQty := pos.OriginalQty

    for _, cycle := range sr.Cycles {
        // Subtract sold value
        netCost -= cycle.SellPrice * cycle.SellQuantity
        netQty -= cycle.SellQuantity

        // Add reentry cost
        if cycle.ReentryState == ReentryStateCompleted {
            netCost += cycle.ReentryFilledPrice * cycle.ReentryFilledQty
            netQty += cycle.ReentryFilledQty
        }
    }

    if netQty <= 0 {
        return pos.EntryPrice
    }

    return netCost / netQty
}
```

---

## Dependencies

- Story 2 depends on Story 1 (precision fixes first)
- Story 3 depends on Story 2 (correct calculation before consolidation)
- Story 4 is independent (can parallel with 1-3)
- Story 5 depends on Story 4 (API defines structure first)
- Story 6 depends on Story 4 (API needed for UI)
- Story 7 depends on Stories 1-3 (test after fixes)

## Suggested Sprint Plan

**Sprint 1 (P0 - Critical)**:
- Story 1: Fix Precision Errors
- Story 2: Fix Breakeven Calculation
- Story 3: Consolidate Quantity Tracking

**Sprint 2 (P1 - High)**:
- Story 4: Mode Config API
- Story 7: Comprehensive Testing

**Sprint 3 (P2 - Medium)**:
- Story 5: Remove Duplicate Defaults
- Story 6: Mode Config UI
