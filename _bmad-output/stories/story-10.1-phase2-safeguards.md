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

---

## Acceptance Criteria

- [ ] AC-S1: No efficiency exit within configurable min hold time
- [ ] AC-S2: Requires 3 consecutive below-threshold signals to exit
- [ ] AC-S3: Efficiency exit blocked if below breakeven
- [ ] AC-S4: Trend data older than 30 seconds is not used for exit
- [ ] AC-S4b: Price data older than 5 seconds raises alert

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/autopilot/ginie_autopilot.go` | Add safeguard checks |
| `internal/autopilot/position_redis.go` | Add new fields to state |
| `internal/autopilot/ginie_types.go` | Add config for min hold times |

---

## Dependencies

- Phase 1 (Story 10.1 basic) must be deployed first
- Need monitoring to see if safeguards are triggering appropriately
