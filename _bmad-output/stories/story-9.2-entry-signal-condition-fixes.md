# Story 9.2: Entry Signal Condition Fixes - Prevent Counter-Trend Entries

**Story ID:** SIGNAL-9.2
**Epic:** Epic 9 - Entry Signal Quality Improvements
**Priority:** P0 (Critical - Prevents Losing Trades)
**Estimated Effort:** 4 hours
**Author:** Claude Code Agent (Party Mode Analysis)
**Status:** In Development
**Created:** 2026-01-11

---

## Problem Statement

### Current State (BEFORE)

Analysis of trading logs revealed positions consistently entering LONG when market conditions were bearish, resulting in immediate losses. Three specific issues were identified:

#### Issue 1: EMA Trend Misalignment Allowed (Fix 1)

**Current Behavior:**
- EMA trend check is just 1 of 5 optional confluence filters
- Entry can pass with 2/5 filters even if EMA trend is OPPOSITE to entry direction
- Example from logs:
  ```
  LTCUSDT: ema_trend:"bearish" but direction:"long" → passed=true (3/5 filters)
  AVAXUSDT: EMA20=bearish, EMA50=bearish, direction=long → only penalized 1 point
  ```

**Code Location:** `internal/autopilot/ginie_analyzer.go:4126-4165`

**Current Code:**
```go
// EMA alignment is just 1 of 5 optional filters
if emaAligned {
    result.EMAValid = true
    confluenceCount++  // Just +1 to count
}
```

**Problem:** A trade can enter LONG with bearish EMA trend if other filters pass. This causes immediate losses as price continues in the existing trend direction.

---

#### Issue 2: Reversal Confidence Too Low (Fix 2)

**Current Behavior:**
- `scalp_reentry` mode allows reversal entries at 70% LLM confidence
- Reversal patterns triggering at 54-74% confidence via MTF alignment
- Low confidence reversals often fail, causing losses

**Code Location:** `autopilot_settings.json` - `mode_configs.scalp_reentry.reversal`

**Current Settings:**
```json
"reversal": {
    "reversal_enabled": true,
    "reversal_min_llm_confidence": 0.7,  // 70% - too low
    ...
}
```

**Problem:** 70% confidence is not high enough for reversal trades which are inherently riskier. Logs show:
```
REVERSAL 5m: LONG pattern, confidence=64.2%
REVERSAL 15m: LONG pattern, confidence=74.3%
MTF ALIGNED - LONG (2/3 TFs), Score=54.4
```

---

#### Issue 3: ADX Threshold Too Low (Fix 4)

**Current Behavior:**
- Base ADX threshold for scalp mode is 10.0
- Most coins show ADX of 11-17, passing the threshold
- Low ADX indicates NO CLEAR TREND, yet trades enter anyway

**Code Location:** `internal/autopilot/ginie_analyzer.go:4014-4019`

**Current Code:**
```go
default: // Scalp
    if coinConfig.ScalpADX > 0 {
        baseADXThreshold = coinConfig.ScalpADX
    } else {
        baseADXThreshold = 10.0  // Was 15.0, lowered because "blocking all trades"
    }
```

**Problem:** ADX of 10-17 means the market is ranging/choppy with no clear trend. Entering directional trades in this environment leads to whipsaws and losses.

---

## Changes Being Made

### Fix 1: Add EMA Trend Direction Blocking

**File:** `internal/autopilot/ginie_analyzer.go`

**BEFORE (line ~4160-4165):**
```go
// EMA alignment just adds to confluence count
if emaAligned {
    result.EMAValid = true
    confluenceCount++
    result.Details = append(result.Details, fmt.Sprintf("✓ EMA20=%.4f, EMA50=%.4f [%s]", ema20, ema50, emaTrend))
} else {
    result.Details = append(result.Details, fmt.Sprintf("✗ EMA20=%.4f, EMA50=%.4f [%s] - misaligned for %s", ema20, ema50, emaTrend, direction))
}
```

**AFTER:**
```go
// EMA alignment adds to confluence count
if emaAligned {
    result.EMAValid = true
    confluenceCount++
    result.Details = append(result.Details, fmt.Sprintf("✓ EMA20=%.4f, EMA50=%.4f [%s]", ema20, ema50, emaTrend))
} else {
    result.Details = append(result.Details, fmt.Sprintf("✗ EMA20=%.4f, EMA50=%.4f [%s] - misaligned for %s", ema20, ema50, emaTrend, direction))

    // FIX 1: BLOCK entry if EMA trend is OPPOSITE to entry direction
    // This prevents entering LONG when EMAs show bearish trend (and vice versa)
    if (direction == "long" && emaTrend == "bearish") || (direction == "short" && emaTrend == "bullish") {
        result.Passed = false
        result.Details = append(result.Details, "⛔ BLOCKED: EMA trend opposite to entry direction - high loss probability")
        return result
    }
}
```

**Rollback:** Remove the `if (direction == "long" && emaTrend == "bearish")` block to restore original behavior.

---

### Fix 2: Increase Reversal Confidence Threshold

**File:** `autopilot_settings.json`

**BEFORE (line ~874-879):**
```json
"reversal": {
    "reversal_enabled": true,
    "reversal_min_llm_confidence": 0.7,
    ...
}
```

**AFTER:**
```json
"reversal": {
    "reversal_enabled": true,
    "reversal_min_llm_confidence": 0.85,
    ...
}
```

**Rollback:** Change `0.85` back to `0.7`.

---

### Fix 4: Increase ADX Threshold for Scalp Mode

**File:** `internal/autopilot/ginie_analyzer.go`

**BEFORE (line ~4018):**
```go
default: // Scalp
    if coinConfig.ScalpADX > 0 {
        baseADXThreshold = coinConfig.ScalpADX
    } else {
        baseADXThreshold = 10.0  // Was 15.0
    }
```

**AFTER:**
```go
default: // Scalp
    if coinConfig.ScalpADX > 0 {
        baseADXThreshold = coinConfig.ScalpADX
    } else {
        baseADXThreshold = 15.0  // Restored: ADX < 15 indicates no clear trend
    }
```

**Rollback:** Change `15.0` back to `10.0`.

---

## Acceptance Criteria

### AC9.2.1: EMA Trend Blocking
- [ ] Entry is BLOCKED if direction=long and emaTrend=bearish
- [ ] Entry is BLOCKED if direction=short and emaTrend=bullish
- [ ] Log message clearly indicates "BLOCKED: EMA trend opposite to entry direction"
- [ ] Entry proceeds normally if EMA trend aligns with direction

### AC9.2.2: Reversal Confidence Increase
- [ ] `scalp_reentry.reversal.reversal_min_llm_confidence` changed from 0.7 to 0.85
- [ ] Reversal entries require 85%+ LLM confidence
- [ ] Lower confidence reversals are rejected

### AC9.2.3: ADX Threshold Increase
- [ ] Base ADX threshold for scalp mode changed from 10.0 to 15.0
- [ ] Entries in ranging markets (ADX < 15) are blocked
- [ ] Log shows new ADX threshold being applied

### AC9.2.4: Verification
- [ ] Container restarted and new settings loaded
- [ ] Logs show rejections with new criteria
- [ ] No LONG entries when EMA trend is bearish
- [ ] No entries when ADX < 15 for scalp mode

---

## Testing

### Test 1: Verify EMA Blocking
```bash
# Watch logs for EMA blocking
docker logs -f binance-trading-bot-dev 2>&1 | grep -E "BLOCKED.*EMA|emaTrend.*bearish.*long"

# Expected: See "BLOCKED: EMA trend opposite to entry direction" when EMA trend opposes entry
```

### Test 2: Verify Reversal Confidence
```bash
# Check settings loaded
docker logs binance-trading-bot-dev 2>&1 | grep -E "reversal_min_llm_confidence|REVERSAL.*confidence"

# Expected: Only reversals with 85%+ confidence should execute
```

### Test 3: Verify ADX Threshold
```bash
# Watch for ADX rejections
docker logs -f binance-trading-bot-dev 2>&1 | grep -E "ADX.*need.*15|ADX=1[0-4]"

# Expected: ADX values 10-14 should fail the threshold check
```

---

## Rollback Instructions

If these changes cause issues, revert each fix individually:

### Rollback Fix 1 (EMA Blocking):
```go
// Remove these lines from CheckEntryConfluence in ginie_analyzer.go:
// if (direction == "long" && emaTrend == "bearish") || (direction == "short" && emaTrend == "bullish") {
//     result.Passed = false
//     result.Details = append(result.Details, "⛔ BLOCKED: EMA trend opposite to entry direction - high loss probability")
//     return result
// }
```

### Rollback Fix 2 (Reversal Confidence):
```json
// In autopilot_settings.json, change:
"reversal_min_llm_confidence": 0.85
// Back to:
"reversal_min_llm_confidence": 0.7
```

### Rollback Fix 4 (ADX Threshold):
```go
// In ginie_analyzer.go line ~4018, change:
baseADXThreshold = 15.0
// Back to:
baseADXThreshold = 10.0
```

---

## Risk Assessment

| Fix | Risk Level | Potential Impact |
|-----|------------|------------------|
| Fix 1 (EMA Block) | LOW | May reduce trade frequency, but prevents counter-trend losses |
| Fix 2 (Reversal 85%) | LOW | Fewer reversal entries, but higher quality when they occur |
| Fix 4 (ADX 15) | MEDIUM | May block trades in low volatility markets - monitor for over-filtering |

---

## Definition of Done

- [ ] Fix 1: EMA trend blocking implemented and tested
- [ ] Fix 2: Reversal confidence threshold increased to 0.85
- [ ] Fix 4: ADX threshold increased to 15.0
- [ ] Container restarted with new code
- [ ] Logs verified showing new rejection criteria
- [ ] No counter-trend entries observed
- [ ] Story documented for rollback if needed

---

## Related

- **Analysis Source:** Party Mode multi-agent analysis (2026-01-11)
- **Agents Involved:** Murat (Risk), Winston (Architecture), John (PM), Amelia (Dev), Mary (Analyst)
- **Previous Story:** Story 9.1 - Configurable Trading Parameters
- **Epic:** Epic 9 - Entry Signal Quality Improvements

---

## Approval Sign-Off

- **Scrum Master (Bob)**: Pending
- **Developer (Amelia)**: Pending
- **Test Architect (Murat)**: Pending
