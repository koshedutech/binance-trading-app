# Story 10.1 Phase 2: Critical Safeguards

## Overview

**Parent Story:** Story 10.1 - Position Management & Efficiency Exit System
**Phase:** 2 (Post basic implementation)
**Priority:** HIGH
**Status:** Planned

---

## Context

Phase 1 implements the basic efficiency exit system. Phase 2 adds critical safeguards identified during risk analysis to prevent common failure scenarios.

---

## Safeguards to Implement

### S1: Minimum Hold Time Before Efficiency Exit

**Problem:** System could exit within seconds of entry due to early price fluctuations.

**Solution:**
```go
const MinHoldBeforeEfficiencyExit = 2 * time.Minute  // Per mode configurable

func (ga *GinieAutopilot) shouldExitOnEfficiency(state *PositionRuntimeState) bool {
    // Don't use efficiency exit too early
    holdDuration := time.Since(time.Unix(state.EntryTime, 0))
    if holdDuration < MinHoldBeforeEfficiencyExit {
        return false
    }

    // ... rest of efficiency check
}
```

**Configuration:**
```json
{
  "ultra_fast": { "min_hold_before_efficiency_mins": 1 },
  "scalp": { "min_hold_before_efficiency_mins": 2 },
  "swing": { "min_hold_before_efficiency_mins": 5 },
  "position": { "min_hold_before_efficiency_mins": 15 }
}
```

---

### S2: Consecutive Signal Requirement (Whipsaw Prevention)

**Problem:** Single tick below threshold triggers exit, causing false exits during normal price oscillation.

**Solution:**
```go
type PositionRuntimeState struct {
    // ... existing fields

    // Whipsaw prevention
    ConsecutiveBelowThreshold int   `json:"consec_below"`
    LastBelowThresholdTime    int64 `json:"below_ts"`
}

const RequiredConsecutiveSignals = 3  // Or 3 seconds

func (ga *GinieAutopilot) checkEfficiencyWithDebounce(state *PositionRuntimeState) bool {
    baseline, _ := ga.redis.GetBaseline(state.UserID, state.Mode)

    if state.Efficiency < baseline.AvgExitEfficiency {
        state.ConsecutiveBelowThreshold++
        if state.ConsecutiveBelowThreshold == 1 {
            state.LastBelowThresholdTime = time.Now().Unix()
        }
    } else {
        // Reset counter if efficiency recovers
        state.ConsecutiveBelowThreshold = 0
        state.LastBelowThresholdTime = 0
    }

    // Require 3 consecutive signals
    return state.ConsecutiveBelowThreshold >= RequiredConsecutiveSignals
}
```

---

### S3: Breakeven Verification Before Efficiency Exit

**Problem:** If price drops below breakeven after efficiency tracking started, efficiency calculation becomes negative/invalid.

**Solution:**
```go
func (ga *GinieAutopilot) shouldExitOnEfficiency(state *PositionRuntimeState) bool {
    // MUST be above breakeven for efficiency exit
    if state.Side == "LONG" && state.CurrentPrice <= state.BEPrice {
        return false  // Use normal SL logic instead
    }
    if state.Side == "SHORT" && state.CurrentPrice >= state.BEPrice {
        return false
    }

    // Also verify profit is positive
    if state.CurrentProfit <= 0 {
        return false
    }

    // ... rest of efficiency check
}
```

---

### S4: Stale Data Detection

**Problem:** Trend analysis data could be old, leading to decisions based on outdated information.

**Solution:**
```go
const MaxTrendDataAge = 30 * time.Second

func (ga *GinieAutopilot) isTrendDataFresh(state *PositionRuntimeState) bool {
    trendAge := time.Now().Unix() - state.TrendTime
    return trendAge <= int64(MaxTrendDataAge.Seconds())
}

func (ga *GinieAutopilot) shouldExitOnTrendReversal(state *PositionRuntimeState) bool {
    // Don't trust stale trend data for exit decisions
    if !ga.isTrendDataFresh(state) {
        // Request fresh analysis instead of exiting
        ga.requestTrendAnalysis(state.Symbol)
        return false
    }

    return state.Reversal && state.TrendStrength > 0.75
}
```

**Additional: Stale price detection:**
```go
const MaxPriceDataAge = 5 * time.Second

func (ga *GinieAutopilot) isPriceDataFresh(state *PositionRuntimeState) bool {
    priceAge := time.Now().Unix() - state.LastUpdate
    if priceAge > int64(MaxPriceDataAge.Seconds()) {
        // Alert! Price data is stale
        ga.alertStaleData(state.Symbol, priceAge)
        return false
    }
    return true
}
```

---

### S5: Epic 11 Integration Safeguards (New Engine Mode)

**Problem:** If New Engine mode is selected but Epic 11 components are unavailable or return errors, exit decisions could fail.

**Solution:**
```go
func (ga *GinieAutopilot) detectTrendReversalNewEngine(state *PositionRuntimeState) bool {
    // SAFEGUARD: Verify DecisionEngine is available
    if ga.decisionEngine == nil {
        ga.logWarning("DecisionEngine unavailable, falling back to Classic mode")
        return ga.detectTrendReversalClassic(state)
    }

    // SAFEGUARD: Verify strategy is available
    strategy := ga.decisionEngine.GetActiveStrategy(state.Symbol)
    if strategy == nil {
        ga.logWarning("No active strategy for %s, falling back to Classic mode", state.Symbol)
        return ga.detectTrendReversalClassic(state)
    }

    // SAFEGUARD: Verify indicators can be calculated
    trendIndicators := ga.decisionEngine.GetIndicators("trend")
    if len(trendIndicators) == 0 {
        ga.logWarning("No trend indicators configured, falling back to Classic mode")
        return ga.detectTrendReversalClassic(state)
    }

    // SAFEGUARD: Handle indicator calculation errors
    trendScore, err := ga.calculateTrendScore(state.Symbol, trendIndicators)
    if err != nil {
        ga.logError("Indicator calculation failed: %v, falling back to Classic mode", err)
        return ga.detectTrendReversalClassic(state)
    }

    // Proceed with New Engine logic...
    return ga.evaluateNewEngineExit(state, strategy, trendScore)
}
```

**Regime Change Safeguard:**
```go
func (ga *GinieAutopilot) shouldExitOnRegimeChange(state *PositionRuntimeState) bool {
    if !ga.config.NewEngine.ExitOnRegimeChange {
        return false
    }

    // SAFEGUARD: Verify regime detection is available
    if ga.decisionEngine == nil {
        return false  // Can't detect regime, don't exit
    }

    currentRegime, err := ga.decisionEngine.GetCurrentRegime(state.Symbol)
    if err != nil {
        ga.logWarning("Regime detection failed: %v", err)
        return false  // Can't detect regime, don't exit
    }

    // SAFEGUARD: Require regime to be stable before acting
    if currentRegime != state.EntryRegime {
        state.RegimeChangeDetectedAt = time.Now().Unix()
        regimeChangeAge := time.Now().Unix() - state.RegimeChangeDetectedAt

        // Require regime change to persist for 60 seconds
        if regimeChangeAge < 60 {
            return false
        }

        ga.logInfo("Regime changed from %s to %s for %s, triggering exit",
            state.EntryRegime, currentRegime, state.Symbol)
        return true
    }

    return false
}
```

**Configuration:**
```json
{
  "new_engine": {
    "fallback_to_classic_on_error": true,
    "regime_change_confirmation_seconds": 60,
    "min_indicators_required": 1
  }
}
```

---

### S6: Decision Mode Consistency

**Problem:** Changing decision mode while positions are open could cause inconsistent behavior.

**Solution:**
```go
func (ga *GinieAutopilot) canChangeDecisionMode(userID string, newMode string) (bool, string) {
    // Check for open positions
    activePositions := ga.redis.GetActivePositions(userID)

    if len(activePositions) > 0 {
        return false, fmt.Sprintf(
            "Cannot change decision mode while %d positions are open. "+
            "Close all positions first or wait for them to exit.",
            len(activePositions))
    }

    return true, ""
}
```

**UI Warning:**
```
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️  Cannot Change Decision Mode                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ You have 2 open positions. Changing the decision mode while    │
│ positions are open could cause inconsistent exit behavior.      │
│                                                                 │
│ Options:                                                        │
│ • Wait for positions to close naturally                         │
│ • Manually close all positions first                           │
│ • Change will apply to NEW positions only (if forced)          │
│                                                                 │
│ [Cancel]  [Apply to New Positions Only]                        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Testing Requirements

### Test S1: Minimum Hold Time
```
1. Open position
2. Price immediately shows efficiency decline (e.g., 40%)
3. Verify NO exit within first 2 minutes
4. After 2 minutes, verify exit triggers if still below threshold
```

### Test S2: Whipsaw Prevention
```
1. Open position, achieve breakeven
2. Efficiency oscillates: 55% → 45% → 52% → 48% → 53%
3. Verify NO exit (not 3 consecutive below)
4. Efficiency drops: 45% → 44% → 43%
5. Verify EXIT after 3rd consecutive signal
```

### Test S3: Breakeven Verification
```
1. Open position at $100
2. Price rises to $101 (peak = 1%)
3. Price drops to $99.50 (below breakeven)
4. Efficiency = negative
5. Verify NO efficiency exit (use SL instead)
```

### Test S4: Stale Data Detection
```
1. Disconnect trend analysis feed
2. Wait 35 seconds
3. Verify trend-based exit is BLOCKED
4. Verify alert is raised
5. Reconnect, verify fresh data allows decisions
```

### Test S5: Epic 11 Integration Fallback
```
1. Set decision mode to "new_engine"
2. Open position with strategy "trend_following"
3. Simulate DecisionEngine unavailable (nil)
4. Verify system falls back to Classic mode detection
5. Verify warning is logged
6. Restore DecisionEngine
7. Verify New Engine mode resumes
```

### Test S6: Decision Mode Change Prevention
```
1. Set decision mode to "classic"
2. Open a position
3. Try to change decision mode to "new_engine"
4. Verify change is blocked with appropriate message
5. Close the position
6. Retry changing decision mode
7. Verify change succeeds
```

### Test S5b: Regime Change Confirmation
```
1. Set decision mode to "new_engine" with exit_on_regime_change=true
2. Open position in TRENDING regime
3. Simulate brief regime change to RANGING (< 60s)
4. Verify NO exit (not yet confirmed)
5. Keep regime at RANGING for 60+ seconds
6. Verify EXIT triggers after confirmation period
```

---

## Acceptance Criteria

- [ ] AC-S1: No efficiency exit within configurable min hold time
- [ ] AC-S2: Requires 3 consecutive below-threshold signals to exit
- [ ] AC-S3: Efficiency exit blocked if below breakeven
- [ ] AC-S4: Trend data older than 30 seconds is not used for exit
- [ ] AC-S4b: Price data older than 5 seconds raises alert
- [ ] AC-S5: New Engine mode falls back to Classic on error
- [ ] AC-S5b: Regime change requires 60s confirmation
- [ ] AC-S6: Decision mode change blocked while positions open

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/autopilot/ginie_autopilot.go` | Add safeguard checks |
| `internal/autopilot/position_redis.go` | Add new fields to state (ConsecutiveBelowThreshold, RegimeChangeDetectedAt) |
| `internal/autopilot/ginie_types.go` | Add config for min hold times, safeguard settings |
| `internal/autopilot/position_exit_engine.go` | Add fallback logic to Classic mode |
| `internal/api/handlers_settings.go` | Add decision mode change validation |
| `web/src/components/PositionDecisionSettings.tsx` | Add warning modal for mode change |

---

## Dependencies

- Phase 1 (Story 10.1 basic) must be deployed first
- Need monitoring to see if safeguards are triggering appropriately
- Epic 11 components (optional) for New Engine mode safeguards

---

## Implementation Priority

| Safeguard | Priority | Complexity | Impact |
|-----------|----------|------------|--------|
| S1: Min Hold Time | HIGH | Low | Prevents early exits |
| S2: Whipsaw Prevention | HIGH | Medium | Prevents false exits |
| S3: Breakeven Check | HIGH | Low | Prevents invalid exits |
| S4: Stale Data | HIGH | Low | Prevents bad decisions |
| S5: Epic 11 Fallback | MEDIUM | Medium | Ensures reliability |
| S6: Mode Consistency | MEDIUM | Low | Prevents confusion |
