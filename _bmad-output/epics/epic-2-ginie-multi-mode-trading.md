# Epic 2: Ginie Multi-Mode Trading System

## Epic Overview

**Goal:** Complete implementation and verification of all four Ginie trading modes (Ultra-Fast, Scalp, Swing, Position) with proper entry/exit logic, mode-specific configurations, and integrated orchestration.

**Business Value:** Enable diversified trading strategies across different timeframes and risk profiles, maximizing profit opportunities while managing risk through mode-specific parameters.

**Priority:** HIGH - Core trading functionality

**Estimated Complexity:** HIGH

---

## Current State Analysis

| Mode | Entry Scanning | Exit Monitoring | Status | Critical Issue |
|------|---------------|-----------------|--------|----------------|
| **Ultra-Fast** | ❌ NOT WIRED | ✅ 500ms polling | **PARTIAL (60%)** | `GenerateUltraFastSignal()` never called |
| **Scalp** | ✅ Working | ✅ Working | **WORKING** | Needs verification logging |
| **Swing** | ✅ Working | ✅ Working | **WORKING (Baseline)** | Default mode, well-tested |
| **Position** | ✅ Working | ✅ Working | **WORKING** | Needs verification logging |

### Key Files

| File | Purpose | Lines of Interest |
|------|---------|-------------------|
| `internal/autopilot/ginie_autopilot.go` | Main scanning loop, execution | 1094-1116 (scan loop), 7000-7323 (ultra-fast) |
| `internal/autopilot/ginie_analyzer.go` | Decision generation, signals | 1070-1525 (mode signals), 2452+ (ultra-fast signal) |
| `internal/autopilot/settings.go` | Mode configurations | 307-650 (mode settings) |
| `internal/autopilot/ginie_types.go` | Type definitions | Mode constants, signal structs |

---

## Target State

| Mode | Entry | Exit | Configuration | Verification |
|------|-------|------|---------------|--------------|
| **Ultra-Fast** | ✅ 5-second scan loop integrated | ✅ 500ms exit monitoring | ✅ All settings applied | ✅ Logged & tested |
| **Scalp** | ✅ 15-minute scan | ✅ Multi-level TP | ✅ All settings applied | ✅ Logged & tested |
| **Swing** | ✅ 4-hour scan | ✅ Trailing stops | ✅ All settings applied | ✅ Logged & tested |
| **Position** | ✅ Daily scan | ✅ Wide trailing | ✅ All settings applied | ✅ Logged & tested |

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-2.1 | Ultra-fast mode scans every 5 seconds | 2.1 |
| FR-2.2 | Ultra-fast uses 4-layer signal generation | 2.1 |
| FR-2.3 | Ultra-fast exits within 3 seconds max hold | 2.1 |
| FR-2.4 | Scalp mode uses RSI/Stochastic/EMA signals | 2.2 |
| FR-2.5 | Scalp mode respects 15-minute trend timeframe | 2.2 |
| FR-2.6 | Swing mode uses MACD/ADX/Bollinger signals | 2.3 |
| FR-2.7 | Swing mode enables trailing after TP1+breakeven | 2.3 |
| FR-2.8 | Position mode uses EMA200/weekly trend | 2.4 |
| FR-2.9 | Position mode uses conservative 2x leverage | 2.4 |
| FR-2.10 | Mode allocation respects capital percentages | 2.5 |
| FR-2.11 | Trend divergence blocks trades when enabled | 2.5 |
| FR-2.12 | Each mode has independent safety controls | 2.5 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-2.1 | Ultra-fast 500ms monitoring doesn't block main loop | 2.1 |
| NFR-2.2 | All modes log decisions for debugging | All |
| NFR-2.3 | Mode switching doesn't cause race conditions | 2.5 |
| NFR-2.4 | Settings changes apply without restart | All |

---

## Story List

| Story | Title | Priority | Complexity | Dependencies | Status |
|-------|-------|----------|------------|--------------|--------|
| **2.1** | **Ultra-Fast Mode: Complete Entry Integration** | **CRITICAL** | **HIGH** | None | 🔴 Not Started |
| 2.2 | Scalp Mode: Verification & Enhanced Logging | HIGH | MEDIUM | None | 🔴 Not Started |
| 2.3 | Swing Mode: Baseline Verification | MEDIUM | LOW | None | 🔴 Not Started |
| 2.4 | Position Mode: Verification & Enhanced Logging | HIGH | MEDIUM | None | 🔴 Not Started |
| 2.5 | Mode Orchestration & Integration Testing | HIGH | MEDIUM | 2.1-2.4 | 🔴 Not Started |
| **2.6** | **ROI-Based SL/TP Selection & UI Bug Fix** | **HIGH** | **MEDIUM** | None | 🔴 Not Started |
| **2.7** | **Mode-Specific Circuit Breaker, Confidence, Timeframe & Size** | **CRITICAL** | **HIGH** | 2.5 | 🔴 Not Started |
| **2.8** | **LLM & Adaptive AI Decision Engine** | **CRITICAL** | **HIGH** | 2.7 | 🔴 Not Started |

---

## Story 2.1: Ultra-Fast Mode - Complete Entry Integration

### User Story

**As a** Ginie autopilot user,
**I want** ultra-fast mode to automatically scan for and execute rapid trades,
**So that** I can capture quick profit opportunities in volatile market conditions.

### Current State

Ultra-fast mode has sophisticated components that are NOT connected:

```
CURRENT FLOW (BROKEN):
[UltraFastEnabled=true] → [Nothing happens for entries]
                        → monitorUltraFastPositions() [exits only]

EXPECTED FLOW (TO IMPLEMENT):
[UltraFastEnabled=true] → [5-second scan loop]
                        → GenerateUltraFastSignal(symbol)
                        → [Confidence check]
                        → executeUltraFastEntry(symbol, signal)
                        → monitorUltraFastPositions() [exits]
```

### Existing Components (Already Coded)

| Component | Location | Status |
|-----------|----------|--------|
| `GenerateUltraFastSignal()` | ginie_analyzer.go:2452 | ✅ Complete but unused |
| `executeUltraFastEntry()` | ginie_autopilot.go:7323 | ✅ Complete but never called |
| `monitorUltraFastPositions()` | ginie_autopilot.go:7000 | ✅ Working |
| `checkUltraFastExits()` | ginie_autopilot.go:7036 | ✅ Working |
| `executeUltraFastExit()` | ginie_autopilot.go:7201 | ✅ Working |
| Configuration settings | settings.go:512-526 | ✅ Defined |

### Missing Component

**Ultra-Fast Scan Loop** - Must be added to main autopilot loop:

```go
// In main loop (ginie_autopilot.go around line 1116)
if currentSettings.UltraFastEnabled && now.Sub(lastUltraFastScan) >= time.Duration(currentSettings.UltraFastScanInterval)*time.Millisecond {
    ga.scanForUltraFast()
    lastUltraFastScan = now
}
```

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| AC-2.1.1 | Ultra-fast scan loop runs every 5 seconds when enabled | Log shows `[ULTRA-FAST-SCAN]` entries every 5s |
| AC-2.1.2 | `GenerateUltraFastSignal()` is called for each watched symbol | Log shows signal generation per symbol |
| AC-2.1.3 | Signals with confidence >= 50% trigger `executeUltraFastEntry()` | Log shows entry attempts |
| AC-2.1.4 | Positions are tracked with `UltraFastSignal` data | Position shows signal metadata |
| AC-2.1.5 | Exit monitoring continues working (500ms polling) | Positions close within max hold time |
| AC-2.1.6 | Safety controls (max trades/minute, daily limit) are enforced | Trades blocked when limits hit |
| AC-2.1.7 | Ultra-fast positions respect mode allocation (20% capital) | Position size calculated correctly |

### Technical Tasks

| Task | Description | File | Estimated Lines |
|------|-------------|------|-----------------|
| 2.1.1 | Add `lastUltraFastScan` timestamp variable | ginie_autopilot.go | 5 |
| 2.1.2 | Add ultra-fast scan condition to main loop | ginie_autopilot.go:1116 | 10 |
| 2.1.3 | Implement `scanForUltraFast()` function | ginie_autopilot.go | 50-80 |
| 2.1.4 | Wire `GenerateUltraFastSignal()` to entry logic | ginie_autopilot.go | 30 |
| 2.1.5 | Add comprehensive logging throughout | ginie_autopilot.go | 20 |
| 2.1.6 | Add unit tests for ultra-fast scanning | New test file | 100 |
| 2.1.7 | Integration test with paper trading | Manual | - |

### Configuration Reference

```json
// From autopilot_settings.json
{
  "ultra_fast_enabled": true,
  "ultra_fast_scan_interval": 5000,        // 5 seconds
  "ultra_fast_monitor_interval": 500,       // 500ms exit check
  "ultra_fast_max_positions": 5,
  "ultra_fast_max_usd_per_pos": 500,
  "ultra_fast_min_confidence": 50,
  "ultra_fast_max_hold_ms": 3000,           // 3 second max
  "ultra_fast_max_daily_trades": 100,
  "ginie_sl_percent_ultrafast": 1,
  "ginie_tp_percent_ultrafast": 2,
  "ginie_trailing_stop_enabled_ultrafast": false
}
```

### Signal Generation Logic (Already Implemented)

```
Layer 1: Trend Filter (1h candles)
  - Detects bias: LONG if close > prev*1.005, SHORT if close < prev*0.995
  - Trend strength: 70% for directional, 40% for neutral

Layer 2: Volatility Regime Classification
  - Categories: extreme, high, medium, low
  - Provides re-entry delays and max trades per hour

Layer 3: Entry Trigger (1m candles)
  - Counts bullish/bearish candles in last 5
  - Confidence: 75% if 3+/5 align with trend

Layer 4: Dynamic Profit Target
  - Fee-aware TP calculation using ATR
  - Minimum 1% profit target after fees
```

### Exit Logic (Already Working)

5-tier priority system:
1. Stop Loss Hit → 100% close
2. Profit Target Hit → 100% close
3. Trailing Stop Triggered → 100% close
4. Time Limit + Profitable → 100% close (after 1s if profitable)
5. Force Exit Timeout → 100% close (after 3s)

### Definition of Done

- [ ] Ultra-fast scan loop integrated into main autopilot loop
- [ ] `GenerateUltraFastSignal()` called for each symbol
- [ ] `executeUltraFastEntry()` triggered when confidence >= threshold
- [ ] All logs showing correct flow: scan → signal → entry → monitor → exit
- [ ] Paper trading verified for at least 10 trades
- [ ] No interference with other mode scans
- [ ] Safety controls verified (rate limits, daily limits)

---

## Story 2.2: Scalp Mode - Verification & Enhanced Logging

### User Story

**As a** Ginie autopilot user,
**I want** scalp mode to execute quick trades with verified logic,
**So that** I can capture short-term opportunities with proper entry/exit management.

### Current State

Scalp mode is integrated but needs verification logging:

| Component | Status | Notes |
|-----------|--------|-------|
| Entry Scanning | ✅ Working | Every 15 minutes via `scanForMode(GinieModeScalp)` |
| Signal Generation | ✅ Working | RSI, Stochastic, EMA, Volume signals |
| Entry Execution | ✅ Working | Via `executeTrade()` |
| Exit Management | ✅ Working | Multi-level TP, trailing (if enabled) |
| Logging | ⚠️ Partial | Need detailed flow logging |

### Signal Generation (4 signals, need 3/4)

| Signal | Weight | Long Condition | Short Condition |
|--------|--------|----------------|-----------------|
| RSI(7) | 30% | RSI < 30 | RSI > 70 |
| Stochastic RSI | 25% | StochRSI < 20 | StochRSI > 80 |
| EMA 9/21 | 25% | EMA9 > EMA21, price > EMA9 | EMA9 < EMA21, price < EMA9 |
| Volume | 20% | Volume > 1.0x avg | Volume > 1.0x avg |

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| AC-2.2.1 | Scalp scan runs every 15 minutes when enabled | Log shows `[SCALP-SCAN]` entries |
| AC-2.2.2 | All 4 signals are evaluated and logged | Log shows individual signal results |
| AC-2.2.3 | Trades execute only when 3/4 signals met | Log shows signal count before trade |
| AC-2.2.4 | SL/TP placed correctly per configuration | Orders verified on Binance |
| AC-2.2.5 | Counter-trend protection blocks opposing trades | Log shows blocking reason |
| AC-2.2.6 | ADX penalty applied for weak trends | Log shows ADX value and penalty |

### Technical Tasks

| Task | Description | File |
|------|-------------|------|
| 2.2.1 | Add `[SCALP-SCAN]` logging at scan start | ginie_autopilot.go |
| 2.2.2 | Add individual signal result logging | ginie_analyzer.go |
| 2.2.3 | Add signal count summary before trade decision | ginie_analyzer.go |
| 2.2.4 | Add SL/TP placement verification logging | ginie_autopilot.go |
| 2.2.5 | Document signal thresholds in comments | ginie_analyzer.go |
| 2.2.6 | Create test script for scalp mode verification | New script |

### Configuration Reference

```json
{
  "ginie_trend_timeframe_scalp": "15m",
  "ginie_sl_percent_scalp": 1.5,
  "ginie_tp_percent_scalp": 3,
  "ginie_trailing_stop_enabled_scalp": false,
  "ginie_use_single_tp_scalp": true
}
```

### Definition of Done

- [ ] All scalp scans logged with timestamp and symbol count
- [ ] Each signal evaluation logged (RSI, Stoch, EMA, Volume)
- [ ] Trade decisions logged with full reasoning
- [ ] SL/TP orders verified on Binance
- [ ] At least 5 scalp trades executed and verified in paper mode

---

## Story 2.3: Swing Mode - Baseline Verification

### User Story

**As a** Ginie autopilot user,
**I want** swing mode to be verified as the working baseline,
**So that** I can confidently use it as the primary trading mode.

### Current State

Swing mode is the **default mode** and currently working:

| Component | Status | Notes |
|-----------|--------|-------|
| Entry Scanning | ✅ Working | Every 4 hours |
| Signal Generation | ✅ Working | MACD, RSI, EMA50, ADX, BB |
| Entry Execution | ✅ Working | Via `executeTrade()` |
| Exit Management | ✅ Working | Trailing stops enabled |
| Logging | ⚠️ Partial | Baseline for comparison |

### Signal Generation (5 signals, need 4/5)

| Signal | Weight | Long Condition | Short Condition |
|--------|--------|----------------|-----------------|
| Price vs EMA50 | 25% | Price > EMA50 | Price < EMA50 |
| RSI(14) | 20% | RSI 50-70 | RSI 30-50 |
| MACD | 20% | MACD > Signal | MACD < Signal |
| ADX/DMI | 20% | ADX > 25, +DI > -DI | ADX > 25, -DI > +DI |
| Bollinger | 15% | Near lower band | Near upper band |

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| AC-2.3.1 | Swing scan runs every 4 hours when enabled | Log shows `[SWING-SCAN]` entries |
| AC-2.3.2 | All 5 signals are evaluated | Log shows individual results |
| AC-2.3.3 | Trend divergence detection works | Log shows divergence severity |
| AC-2.3.4 | Trailing stops activate after TP1+breakeven | Log shows trailing activation |
| AC-2.3.5 | Swing is default when mode not specified | Verify `SelectMode()` defaults |

### Configuration Reference

```json
{
  "ginie_trend_timeframe_swing": "1h",
  "ginie_sl_percent_swing": 2.5,
  "ginie_tp_percent_swing": 5,
  "ginie_trailing_stop_enabled_swing": true,
  "ginie_trailing_stop_percent_swing": 1.5,
  "ginie_trailing_activation_mode": "after_tp1_and_breakeven"
}
```

### Definition of Done

- [ ] All swing scans logged with detailed signal breakdown
- [ ] Divergence detection verified with examples
- [ ] Trailing stop activation logged and verified
- [ ] Documentation updated with swing mode as baseline

---

## Story 2.4: Position Mode - Verification & Enhanced Logging

### User Story

**As a** Ginie autopilot user,
**I want** position mode to execute longer-term trades,
**So that** I can capture larger moves with conservative risk management.

### Current State

Position mode is integrated but needs verification:

| Component | Status | Notes |
|-----------|--------|-------|
| Entry Scanning | ✅ Working | Daily scan |
| Signal Generation | ✅ Working | EMA200, ADX, DMI, S/R, Volume |
| Entry Execution | ✅ Working | Conservative 2x leverage |
| Exit Management | ✅ Working | Wide trailing (3%) |
| Logging | ⚠️ Partial | Need verification |

### Signal Generation (5 signals, need 4/5)

| Signal | Weight | Long Condition | Short Condition |
|--------|--------|----------------|-----------------|
| EMA200 | 20% | Price > EMA200 | Price < EMA200 |
| ADX Strength | 30% | ADX > 35 | ADX > 35 |
| Trend Align | 25% | +DI > -DI | -DI > +DI |
| Support/Resist | 15% | Near support | Near resistance |
| Volume Profile | 10% | High vol at level | High vol at level |

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| AC-2.4.1 | Position scan runs daily when enabled | Log shows `[POSITION-SCAN]` |
| AC-2.4.2 | Conservative 2x leverage applied | Order shows correct leverage |
| AC-2.4.3 | Wide SL/TP (3%/8%) applied | Orders verified |
| AC-2.4.4 | Trailing activates after 2% profit | Log shows activation |
| AC-2.4.5 | Only 2 max positions allowed | Position count verified |

### Configuration Reference

```json
{
  "ginie_trend_timeframe_position": "4h",
  "ginie_sl_percent_position": 3,
  "ginie_tp_percent_position": 8,
  "ginie_trailing_stop_enabled_position": true,
  "ginie_trailing_stop_percent_position": 2,
  "ginie_trailing_stop_activation_position": 0
}
```

### Definition of Done

- [ ] All position scans logged with signal breakdown
- [ ] Conservative leverage verified
- [ ] Wide SL/TP verified on Binance orders
- [ ] At least 2 position trades tracked over time

---

## Story 2.5: Mode Orchestration & Integration Testing

### User Story

**As a** Ginie autopilot user,
**I want** all four modes to work together seamlessly,
**So that** I can run diversified trading strategies simultaneously.

### Current State

Mode orchestration partially working:

| Component | Status | Notes |
|-----------|--------|-------|
| Mode Selection | ✅ Working | `SelectMode()` function |
| Independent Scanning | ✅ Fixed | Each mode scans independently |
| Capital Allocation | ⚠️ Needs verification | 20/20/35/15 split |
| Safety Controls | ✅ Working | Per-mode limits |
| Integration | ⚠️ Needs testing | All modes running together |

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| AC-2.5.1 | All 4 modes can run simultaneously | Log shows all mode scans |
| AC-2.5.2 | Capital allocation respected (20/20/35/15) | Position sizes verified |
| AC-2.5.3 | Per-mode position limits enforced | Max positions per mode |
| AC-2.5.4 | Per-mode safety controls independent | Rate limits per mode |
| AC-2.5.5 | No race conditions between modes | Concurrent execution stable |
| AC-2.5.6 | Trend divergence blocking works cross-mode | Blocks applied correctly |

### Technical Tasks

| Task | Description |
|------|-------------|
| 2.5.1 | Add integration test running all modes |
| 2.5.2 | Verify capital allocation math |
| 2.5.3 | Test concurrent mode execution |
| 2.5.4 | Document mode interaction patterns |
| 2.5.5 | Create monitoring dashboard for multi-mode |

### Definition of Done

- [ ] All 4 modes running simultaneously in paper mode
- [ ] Capital allocation verified with math proof
- [ ] 24-hour test run with no errors
- [ ] Documentation complete for mode orchestration

---

## Dependencies Graph

```
Story 2.1 (Ultra-Fast) ──┐
Story 2.2 (Scalp) ───────┼──→ Story 2.5 (Integration)
Story 2.3 (Swing) ───────┤
Story 2.4 (Position) ────┘
```

Stories 2.1-2.4 can be worked in parallel.
Story 2.5 requires all others complete.

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| Ultra-fast entry causes rapid losses | HIGH | HIGH | Start with paper mode, strict limits |
| Mode interference (race conditions) | MEDIUM | MEDIUM | Proper locking, sequential execution |
| Capital allocation exceeds balance | LOW | HIGH | Pre-trade balance checks |
| API rate limits hit | MEDIUM | MEDIUM | Implement backoff, caching |

---

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| All modes executing | 4/4 modes active | Log verification |
| Trade execution rate | > 90% success | Orders placed vs attempted |
| Win rate (paper mode) | > 50% | Post-trade analysis |
| System stability | Zero crashes in 24h | Monitoring |

---

## Timeline Suggestion

| Phase | Stories | Parallel Work |
|-------|---------|---------------|
| Phase 1 | 2.1, 2.2, 2.3, 2.4, 2.6 | All can run parallel |
| Phase 2 | 2.5 | After Phase 1 complete |
| Phase 3 | Monitoring & Tuning | Ongoing |

---

## Story 2.6: ROI-Based SL/TP Selection & UI Bug Fix

### User Story

**As a** Ginie autopilot user,
**I want** to choose between price-based and ROI-based SL/TP calculation,
**So that** I can set stop-loss and take-profit based on my desired return percentage.

### Current State

| Component | Status | Issue |
|-----------|--------|-------|
| Price-based SL/TP | ✅ Working | Current implementation |
| ROI-based SL/TP | ❌ Not implemented | Need to add selection |
| UI Settings Save | ❌ **BUG** | Settings not persisting |
| Trailing Stop Selection | ✅ Working | Per-mode configuration |

### Bug Analysis: UI Settings Not Saving

**Root Cause:** Key mismatch in `GiniePanel.tsx` lines 572-580

```typescript
// BUG: API returns 'ultrafast' but code looks for 'ultra_fast'
const mergedConfig = {
  ultrafast: apiConfig?.ultra_fast || {...},  // ❌ WRONG KEY
  scalp: apiConfig?.scalp || {...},
  swing: apiConfig?.swing || {...},
  position: apiConfig?.position || {...},
};
```

**Fix Required:**
```typescript
const mergedConfig = {
  ultrafast: apiConfig?.ultrafast || {...},  // ✅ Correct key
  scalp: apiConfig?.scalp || {...},
  swing: apiConfig?.swing || {...},
  position: apiConfig?.position || {...},
};
```

### ROI-Based SL/TP Formulas (Binance Standard)

**For LONG positions:**
```
SL Price = Entry × (1 - SL_ROI% / (Leverage × 100))
TP Price = Entry × (1 + TP_ROI% / (Leverage × 100))
```

**For SHORT positions:**
```
SL Price = Entry × (1 + SL_ROI% / (Leverage × 100))
TP Price = Entry × (1 - TP_ROI% / (Leverage × 100))
```

**Example Calculation:**
- Entry Price: $100
- Leverage: 10x
- Desired TP ROI: 5%
- TP Price (LONG) = $100 × (1 + 5/(10×100)) = $100 × 1.005 = **$100.50**

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| AC-2.6.1 | **BUG FIX**: UI SL/TP edits persist after save and page refresh | Save, refresh, verify values |
| AC-2.6.2 | User can select "Price-Based" or "ROI-Based" SL/TP mode | UI dropdown/toggle |
| AC-2.6.3 | Price-based uses existing percentage calculation | Current behavior preserved |
| AC-2.6.4 | ROI-based calculates price from entry × ROI% / leverage | Formula verified |
| AC-2.6.5 | ROI-based applies to all modes (ultra-fast, scalp, swing, position) | Per-mode testing |
| AC-2.6.6 | Selection persists to settings file | Check autopilot_settings.json |
| AC-2.6.7 | Binance orders placed with correct calculated prices | Verify on Binance |
| AC-2.6.8 | Trailing stop selection works for both modes | Toggle test |
| **AC-2.6.9** | **Progressive SL Movement enabled when multi-TP selected** | Toggle visible only for multi-TP |
| **AC-2.6.10** | **When TP1 hit, SL moves to entry price (breakeven)** | Log + Binance order update |
| **AC-2.6.11** | **When TP2 hit, SL moves to TP1 price** | Log + Binance order update |
| **AC-2.6.12** | **When TP3 hit, SL moves to TP2 price** | Log + Binance order update |
| **AC-2.6.13** | **UI clearly shows SL movement rules per TP level** | Visual table in UI |
| **AC-2.6.14** | **LLM can analyze market and suggest optimal SL/TP** | API returns LLM suggestions |
| **AC-2.6.15** | **User can enable/disable LLM adaptive mode** | Toggle in UI |
| **AC-2.6.16** | **LLM can adjust SL/TP dynamically based on volatility** | Log shows LLM adjustments |
| **AC-2.6.17** | **LLM weight configurable (0-100% blend with ATR)** | Settings slider |

### Progressive SL Movement Feature (Lock-in Profits)

**Concept:** As each take-profit level is hit, the stop-loss automatically moves up to lock in profits.

**Logic Flow:**
```
┌──────────────────────────────────────────────────────────────────┐
│ PROGRESSIVE SL MOVEMENT - "Lock-in Profits"                      │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  INITIAL STATE:                                                   │
│  ├── Entry Price: $100                                           │
│  ├── Stop Loss: $98 (-2%)                                        │
│  ├── TP1: $102 (+2%)  → Close 25%                                │
│  ├── TP2: $104 (+4%)  → Close 25%                                │
│  ├── TP3: $106 (+6%)  → Close 25%                                │
│  └── TP4: $108 (+8%)  → Close 25%                                │
│                                                                   │
│  WHEN TP1 HIT ($102):                                            │
│  ├── ✅ Close 25% of position at $102                            │
│  ├── 🔄 Move SL from $98 → $100 (Entry = Breakeven)              │
│  └── 🔒 Remaining 75% now risk-free!                             │
│                                                                   │
│  WHEN TP2 HIT ($104):                                            │
│  ├── ✅ Close 25% of position at $104                            │
│  ├── 🔄 Move SL from $100 → $102 (TP1 Price)                     │
│  └── 🔒 Remaining 50% has locked +2% gain minimum                │
│                                                                   │
│  WHEN TP3 HIT ($106):                                            │
│  ├── ✅ Close 25% of position at $106                            │
│  ├── 🔄 Move SL from $102 → $104 (TP2 Price)                     │
│  └── 🔒 Remaining 25% has locked +4% gain minimum                │
│                                                                   │
│  WHEN TP4 HIT ($108):                                            │
│  ├── ✅ Close final 25% of position at $108                      │
│  └── ✅ Position fully closed with full profit                   │
│                                                                   │
│  IF PRICE REVERSES (after TP2 hit, falls to $102):               │
│  ├── SL at $102 triggers                                         │
│  ├── Remaining 50% closed at $102                                │
│  └── 🔒 Still profitable! (TP1 + TP2 gains + breakeven on rest)  │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

**User Benefit:** Even if the price reverses after hitting some TP levels, the trader locks in profits progressively instead of losing all gains.

### UI Design for Progressive SL Movement

```
┌─────────────────────────────────────────────────────────────────────┐
│ 📊 Take Profit Configuration                                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  TP Mode:  ○ Single TP (Close 100% at one level)                    │
│            ● Multi-Level TP (Close portions at each level)          │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ Level │ Close % │ Target │ After Hit → Move SL To              ││
│  ├───────┼─────────┼────────┼─────────────────────────────────────┤│
│  │ TP1   │  [25%]  │ [+2%]  │ 🔒 Entry Price (Breakeven)          ││
│  │ TP2   │  [25%]  │ [+4%]  │ 🔒 TP1 Price (Lock +2%)             ││
│  │ TP3   │  [25%]  │ [+6%]  │ 🔒 TP2 Price (Lock +4%)             ││
│  │ TP4   │  [25%]  │ [+8%]  │ ✅ Position Closed                  ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐│
│  │ ☑ Enable Progressive SL Movement                                ││
│  │   ───────────────────────────────────────────                   ││
│  │   ℹ️ "Lock in profits by moving stop-loss as each TP is hit"   ││
│  │                                                                  ││
│  │   How it works:                                                  ││
│  │   • TP1 hit → SL moves to entry (no loss possible)              ││
│  │   • TP2 hit → SL moves to TP1 (minimum gain locked)             ││
│  │   • TP3 hit → SL moves to TP2 (more gain locked)                ││
│  │                                                                  ││
│  │   💡 Protects profits if price reverses after partial closes    ││
│  └─────────────────────────────────────────────────────────────────┘│
│                                                                      │
│                                        [Cancel]  [Save Settings]    │
└─────────────────────────────────────────────────────────────────────┘
```

### Technical Tasks

| Task | Description | File | Priority |
|------|-------------|------|----------|
| **2.6.1** | **Fix key mismatch in fetchSLTPConfig()** | GiniePanel.tsx:572-580 | **CRITICAL** |
| 2.6.2 | Add `sltp_calculation_mode` field to settings | settings.go | HIGH |
| 2.6.3 | Add `roi_sl_percent` and `roi_tp_percent` per mode | settings.go | HIGH |
| 2.6.4 | Implement ROI-to-price conversion function | ginie_analyzer.go | HIGH |
| 2.6.5 | Update `RecalculateAdaptiveSLTP` to use selected mode | ginie_autopilot.go | HIGH |
| 2.6.6 | Add UI toggle for Price-Based vs ROI-Based | GiniePanel.tsx | HIGH |
| 2.6.7 | Add API endpoint for SLTP mode selection | handlers_ginie.go | MEDIUM |
| 2.6.8 | Update futuresApi.ts with new API calls | futuresApi.ts | MEDIUM |
| 2.6.9 | Add validation for ROI percentages | settings.go | LOW |
| 2.6.10 | Add unit tests for ROI calculation | New test file | LOW |
| **2.6.11** | **Add `progressive_sl_enabled` setting** | settings.go | **HIGH** |
| **2.6.12** | **Implement `MoveSLToPrice()` function** | ginie_autopilot.go | **HIGH** |
| **2.6.13** | **Add TP hit detection with SL movement logic** | ginie_autopilot.go | **HIGH** |
| **2.6.14** | **Update Binance SL order on TP hit** | ginie_autopilot.go | **HIGH** |
| **2.6.15** | **Add UI table showing SL movement per TP level** | GiniePanel.tsx | **HIGH** |
| **2.6.16** | **Add toggle for Progressive SL (only visible for multi-TP)** | GiniePanel.tsx | **MEDIUM** |
| 2.6.17 | Log all SL movements with before/after prices | ginie_autopilot.go | MEDIUM |
| 2.6.18 | Handle edge cases (order cancellation, partial fills) | ginie_autopilot.go | MEDIUM |
| **2.6.19** | **Add `llm_adaptive_sltp_enabled` setting** | settings.go | **HIGH** |
| **2.6.20** | **Implement `GetLLMAdaptiveSLTP()` function** | ginie_analyzer.go | **HIGH** |
| **2.6.21** | **Add LLM weight slider (0-100% blend with ATR)** | GiniePanel.tsx | **HIGH** |
| **2.6.22** | **Create LLM prompt template for market analysis** | ginie_analyzer.go | **HIGH** |
| 2.6.23 | Add LLM suggestion logging with reasoning | ginie_autopilot.go | MEDIUM |
| 2.6.24 | Implement LLM volatility detection for dynamic SL adjustment | ginie_analyzer.go | MEDIUM |

### New Settings Structure

```go
// In settings.go - Add to AutopilotSettings struct

// SL/TP Calculation Mode Selection
SLTPCalculationMode      string  `json:"sltp_calculation_mode"`      // "price" or "roi"

// Per-mode ROI settings (when sltp_calculation_mode = "roi")
GinieROISLPercentUltrafast  float64 `json:"ginie_roi_sl_percent_ultrafast"`  // e.g., 2 = -2% ROI
GinieROITPPercentUltrafast  float64 `json:"ginie_roi_tp_percent_ultrafast"`  // e.g., 5 = +5% ROI
GinieROISLPercentScalp      float64 `json:"ginie_roi_sl_percent_scalp"`
GinieROITPPercentScalp      float64 `json:"ginie_roi_tp_percent_scalp"`
GinieROISLPercentSwing      float64 `json:"ginie_roi_sl_percent_swing"`
GinieROITPPercentSwing      float64 `json:"ginie_roi_tp_percent_swing"`
GinieROISLPercentPosition   float64 `json:"ginie_roi_sl_percent_position"`
GinieROITPPercentPosition   float64 `json:"ginie_roi_tp_percent_position"`

// Progressive SL Movement (Lock-in Profits)
GinieProgressiveSLEnabled   bool    `json:"ginie_progressive_sl_enabled"`   // Enable SL movement on TP hits
// When enabled and multi-TP selected:
// - TP1 hit → SL moves to entry price (breakeven)
// - TP2 hit → SL moves to TP1 price
// - TP3 hit → SL moves to TP2 price

// LLM Adaptive SL/TP Settings
LLMAdaptiveSLTPEnabled      bool    `json:"llm_adaptive_sltp_enabled"`      // Enable LLM suggestions
LLMAdaptiveWeight           int     `json:"llm_adaptive_weight"`            // 0-100% blend with ATR (default: 50)
LLMVolatilityAdjustment     bool    `json:"llm_volatility_adjustment"`      // Allow LLM to adjust based on volatility
LLMMinConfidenceForAdjust   int     `json:"llm_min_confidence_adjust"`      // Min LLM confidence to apply suggestions (default: 70)
// When enabled:
// - LLM analyzes market conditions (trend, volatility, support/resistance)
// - Suggests optimal SL/TP levels based on analysis
// - Blends with ATR calculation: final = (ATR * (100-weight) + LLM * weight) / 100
// - Can dynamically widen/tighten SL/TP based on volatility regime
```

### ROI Calculation Function (To Implement)

```go
// In ginie_analyzer.go or new file
func CalculatePriceFromROI(entryPrice, roiPercent, leverage float64, isLong bool) float64 {
    adjustment := roiPercent / (leverage * 100)
    if isLong {
        return entryPrice * (1 + adjustment)
    }
    return entryPrice * (1 - adjustment)
}

func CalculateSLTPFromROI(entryPrice, slROI, tpROI, leverage float64, isLong bool) (slPrice, tpPrice float64) {
    // SL is always a loss, so negate the direction
    if isLong {
        slPrice = entryPrice * (1 - slROI/(leverage*100))
        tpPrice = entryPrice * (1 + tpROI/(leverage*100))
    } else {
        slPrice = entryPrice * (1 + slROI/(leverage*100))
        tpPrice = entryPrice * (1 - tpROI/(leverage*100))
    }
    return slPrice, tpPrice
}
```

### Progressive SL Movement Function (To Implement)

```go
// In ginie_autopilot.go

// GetNewSLPriceOnTPHit returns the new SL price when a TP level is hit
// Returns 0 if progressive SL is disabled or no movement needed
func (ga *GinieAutopilot) GetNewSLPriceOnTPHit(pos *GiniePosition, tpLevelHit int) float64 {
    settings := ga.settingsManager.GetCurrentSettings()

    // Check if progressive SL is enabled
    if !settings.GinieProgressiveSLEnabled {
        return 0
    }

    // Only applies to multi-TP mode
    if settings.GinieUseSingleTP {
        return 0
    }

    entryPrice := pos.EntryPrice

    switch tpLevelHit {
    case 1:
        // TP1 hit → Move SL to entry price (breakeven)
        return entryPrice
    case 2:
        // TP2 hit → Move SL to TP1 price
        return pos.TakeProfits[0].Price
    case 3:
        // TP3 hit → Move SL to TP2 price
        return pos.TakeProfits[1].Price
    default:
        return 0
    }
}

// MoveSLToNewPrice cancels existing SL order and places new one at newSLPrice
func (ga *GinieAutopilot) MoveSLToNewPrice(pos *GiniePosition, newSLPrice float64, reason string) error {
    ga.logger.Info("Progressive SL Movement",
        "symbol", pos.Symbol,
        "old_sl", pos.StopLoss,
        "new_sl", newSLPrice,
        "reason", reason)

    // 1. Cancel existing SL order
    if pos.SLOrderID != 0 {
        err := ga.futuresClient.CancelOrder(pos.Symbol, pos.SLOrderID)
        if err != nil {
            ga.logger.Error("Failed to cancel old SL order", "error", err)
            return err
        }
    }

    // 2. Place new SL order at new price
    side := "SELL"
    if pos.Direction == "SHORT" {
        side = "BUY"
    }

    order, err := ga.futuresClient.PlaceStopMarketOrder(
        pos.Symbol,
        side,
        pos.RemainingQty,
        newSLPrice,
    )
    if err != nil {
        ga.logger.Error("Failed to place new SL order", "error", err)
        return err
    }

    // 3. Update position tracking
    pos.StopLoss = newSLPrice
    pos.SLOrderID = order.OrderID

    ga.logger.Info("SL moved successfully",
        "symbol", pos.Symbol,
        "new_sl", newSLPrice,
        "new_order_id", order.OrderID)

    return nil
}

// OnTPHit is called when a TP level is hit, handles progressive SL movement
func (ga *GinieAutopilot) OnTPHit(pos *GiniePosition, tpLevelHit int) {
    newSLPrice := ga.GetNewSLPriceOnTPHit(pos, tpLevelHit)

    if newSLPrice > 0 && newSLPrice != pos.StopLoss {
        reason := fmt.Sprintf("TP%d hit - locking profits", tpLevelHit)
        err := ga.MoveSLToNewPrice(pos, newSLPrice, reason)
        if err != nil {
            ga.logger.Error("Progressive SL movement failed",
                "symbol", pos.Symbol,
                "tp_level", tpLevelHit,
                "error", err)
        }
    }
}
```

### LLM Adaptive SL/TP Function (To Implement)

```go
// In ginie_analyzer.go

// LLMSLTPSuggestion represents LLM's recommended SL/TP values
type LLMSLTPSuggestion struct {
    SLPercent       float64 `json:"sl_percent"`
    TPPercent       float64 `json:"tp_percent"`
    Confidence      int     `json:"confidence"`
    Reasoning       string  `json:"reasoning"`
    VolatilityLevel string  `json:"volatility_level"` // "low", "medium", "high", "extreme"
    TrendStrength   float64 `json:"trend_strength"`   // 0-100
    AdjustmentType  string  `json:"adjustment_type"`  // "widen", "tighten", "normal"
}

// GetLLMAdaptiveSLTP asks the LLM for optimal SL/TP based on market conditions
func (ga *GinieAnalyzer) GetLLMAdaptiveSLTP(symbol string, mode GinieMode, entryPrice float64, isLong bool, marketData *MarketAnalysis) (*LLMSLTPSuggestion, error) {
    settings := ga.settingsManager.GetCurrentSettings()

    if !settings.LLMAdaptiveSLTPEnabled {
        return nil, nil
    }

    // Build prompt for LLM
    prompt := ga.BuildLLMSLTPPrompt(symbol, mode, entryPrice, isLong, marketData)

    // Call LLM API
    response, err := ga.llmClient.Analyze(prompt)
    if err != nil {
        ga.logger.Error("LLM SLTP analysis failed", "error", err)
        return nil, err
    }

    // Parse LLM response
    suggestion := &LLMSLTPSuggestion{}
    if err := json.Unmarshal([]byte(response.JSON), suggestion); err != nil {
        ga.logger.Error("Failed to parse LLM SLTP response", "error", err)
        return nil, err
    }

    ga.logger.Info("LLM SLTP Suggestion",
        "symbol", symbol,
        "mode", mode,
        "sl_percent", suggestion.SLPercent,
        "tp_percent", suggestion.TPPercent,
        "confidence", suggestion.Confidence,
        "volatility", suggestion.VolatilityLevel,
        "adjustment", suggestion.AdjustmentType,
        "reasoning", suggestion.Reasoning)

    return suggestion, nil
}

// BuildLLMSLTPPrompt creates the prompt for LLM market analysis
func (ga *GinieAnalyzer) BuildLLMSLTPPrompt(symbol string, mode GinieMode, entryPrice float64, isLong bool, data *MarketAnalysis) string {
    direction := "LONG"
    if !isLong {
        direction = "SHORT"
    }

    modeDefaults := ga.GetModeDefaultSLTP(mode)

    return fmt.Sprintf(`Analyze the following market conditions for %s and suggest optimal Stop Loss and Take Profit percentages.

## Position Details
- Symbol: %s
- Direction: %s
- Entry Price: %.8f
- Trading Mode: %s (default SL: %.2f%%, TP: %.2f%%)

## Current Market Data
- Current Price: %.8f
- 24h High: %.8f
- 24h Low: %.8f
- 24h Volume: %.2f
- ATR (14): %.8f
- RSI (14): %.2f
- ADX: %.2f
- Trend Direction: %s
- Volatility Regime: %s

## Support/Resistance Levels
- Nearest Support: %.8f
- Nearest Resistance: %.8f

## Task
Based on this data, suggest:
1. Optimal Stop Loss percentage (considering volatility and support/resistance)
2. Optimal Take Profit percentage (considering trend strength and resistance)
3. Whether to widen (high volatility), tighten (low volatility), or keep normal SL/TP
4. Your confidence level (0-100) in these suggestions
5. Brief reasoning (1-2 sentences)

Return ONLY a JSON object with these fields:
{
  "sl_percent": <number>,
  "tp_percent": <number>,
  "confidence": <0-100>,
  "reasoning": "<string>",
  "volatility_level": "<low|medium|high|extreme>",
  "trend_strength": <0-100>,
  "adjustment_type": "<widen|tighten|normal>"
}`,
        symbol, symbol, direction, entryPrice, mode, modeDefaults.SL, modeDefaults.TP,
        data.CurrentPrice, data.High24h, data.Low24h, data.Volume24h,
        data.ATR, data.RSI, data.ADX, data.TrendDirection, data.VolatilityRegime,
        data.NearestSupport, data.NearestResistance)
}

// BlendATRWithLLM combines ATR-based and LLM-suggested SL/TP values
func (ga *GinieAnalyzer) BlendATRWithLLM(atrSL, atrTP float64, llmSuggestion *LLMSLTPSuggestion, weight int) (finalSL, finalTP float64) {
    if llmSuggestion == nil || weight == 0 {
        return atrSL, atrTP
    }

    // Check minimum confidence
    settings := ga.settingsManager.GetCurrentSettings()
    if llmSuggestion.Confidence < settings.LLMMinConfidenceForAdjust {
        ga.logger.Info("LLM confidence too low, using ATR only",
            "llm_confidence", llmSuggestion.Confidence,
            "min_required", settings.LLMMinConfidenceForAdjust)
        return atrSL, atrTP
    }

    // Blend: final = ATR * (100-weight)/100 + LLM * weight/100
    atrWeight := float64(100 - weight) / 100.0
    llmWeight := float64(weight) / 100.0

    finalSL = (atrSL * atrWeight) + (llmSuggestion.SLPercent * llmWeight)
    finalTP = (atrTP * atrWeight) + (llmSuggestion.TPPercent * llmWeight)

    ga.logger.Info("Blended ATR + LLM SL/TP",
        "atr_sl", atrSL, "atr_tp", atrTP,
        "llm_sl", llmSuggestion.SLPercent, "llm_tp", llmSuggestion.TPPercent,
        "weight", weight,
        "final_sl", finalSL, "final_tp", finalTP)

    return finalSL, finalTP
}

// AdjustSLTPForVolatility dynamically adjusts SL/TP based on LLM volatility assessment
func (ga *GinieAnalyzer) AdjustSLTPForVolatility(baseSL, baseTP float64, suggestion *LLMSLTPSuggestion) (adjustedSL, adjustedTP float64) {
    if suggestion == nil {
        return baseSL, baseTP
    }

    switch suggestion.AdjustmentType {
    case "widen":
        // High volatility - widen SL to avoid early stops, widen TP for bigger moves
        adjustedSL = baseSL * 1.5
        adjustedTP = baseTP * 1.3
    case "tighten":
        // Low volatility - tighten SL for safety, tighten TP for quicker exits
        adjustedSL = baseSL * 0.7
        adjustedTP = baseTP * 0.8
    default:
        // Normal - no adjustment
        adjustedSL = baseSL
        adjustedTP = baseTP
    }

    ga.logger.Info("Volatility-adjusted SL/TP",
        "adjustment", suggestion.AdjustmentType,
        "volatility", suggestion.VolatilityLevel,
        "base_sl", baseSL, "adjusted_sl", adjustedSL,
        "base_tp", baseTP, "adjusted_tp", adjustedTP)

    return adjustedSL, adjustedTP
}
```

### UI Design for LLM Adaptive SL/TP

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 🤖 AI-Powered SL/TP Optimization                                         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ☑ Enable LLM Adaptive SL/TP                                            │
│    ──────────────────────────────                                        │
│    ℹ️ "AI analyzes market conditions to suggest optimal levels"          │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │ LLM Weight (Blend with ATR calculation)                            │ │
│  │                                                                     │ │
│  │ ATR Only ─────────────●───────────── LLM Only                      │ │
│  │    0%                50%                 100%                       │ │
│  │                                                                     │ │
│  │ Current: [50%] ← Use slider                                        │ │
│  │                                                                     │ │
│  │ Formula: Final = (ATR × 50%) + (LLM × 50%)                         │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │ ☑ Enable Volatility Adjustment                                     │ │
│  │   ℹ️ "AI can widen/tighten SL/TP based on market volatility"       │ │
│  │                                                                     │ │
│  │   • High Volatility: Widen SL +50%, TP +30%                        │ │
│  │   • Low Volatility:  Tighten SL -30%, TP -20%                      │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │ Minimum LLM Confidence:  [70] %                                    │ │
│  │   ℹ️ "Only apply LLM suggestions if confidence >= this value"      │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │ 📊 Recent LLM Suggestions                                          │ │
│  ├────────────┬───────┬───────┬───────┬────────────────────────────────┤ │
│  │ Symbol     │ SL%   │ TP%   │ Conf. │ Reasoning                      │ │
│  ├────────────┼───────┼───────┼───────┼────────────────────────────────┤ │
│  │ BTCUSDT    │ 1.8%  │ 4.2%  │ 85%   │ High volatility, widen SL     │ │
│  │ ETHUSDT    │ 2.1%  │ 5.0%  │ 72%   │ Strong trend, normal TP       │ │
│  │ SOLUSDT    │ 1.2%  │ 2.5%  │ 68%   │ Low volatility, tighter SL    │ │
│  └────────────┴───────┴───────┴───────┴────────────────────────────────┘ │
│                                                                          │
│                                           [Cancel]  [Save Settings]     │
└─────────────────────────────────────────────────────────────────────────┘
```

### LLM Adaptive Flow Diagram

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    LLM ADAPTIVE SL/TP DECISION FLOW                       │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                           │
│  1. ENTRY SIGNAL DETECTED                                                 │
│     │                                                                     │
│     ▼                                                                     │
│  2. GATHER MARKET DATA                                                    │
│     ├── Current price, 24h range                                          │
│     ├── ATR, RSI, ADX, MACD                                               │
│     ├── Support/Resistance levels                                         │
│     └── Volume profile                                                    │
│     │                                                                     │
│     ▼                                                                     │
│  3. CALCULATE ATR-BASED SL/TP (Baseline)                                  │
│     ├── SL = ATR × multiplier × mode factor                               │
│     └── TP = ATR × TP ratio × mode factor                                 │
│     │                                                                     │
│     ▼                                                                     │
│  ┌───────────────────────────────────────┐                                │
│  │ LLM Adaptive Enabled?                  │                               │
│  │    NO ──────────────────────────────▶ Use ATR values                   │
│  │    YES                                 │                               │
│  └───────────────────────────────────────┘                                │
│     │                                                                     │
│     ▼                                                                     │
│  4. CALL LLM FOR ANALYSIS                                                 │
│     ├── Send market data + context                                        │
│     ├── Receive: SL%, TP%, confidence, reasoning                          │
│     └── Receive: volatility level, adjustment type                        │
│     │                                                                     │
│     ▼                                                                     │
│  ┌───────────────────────────────────────┐                                │
│  │ LLM Confidence >= Min Threshold?       │                               │
│  │    NO ──────────────────────────────▶ Use ATR values only              │
│  │    YES                                 │                               │
│  └───────────────────────────────────────┘                                │
│     │                                                                     │
│     ▼                                                                     │
│  5. BLEND ATR + LLM                                                       │
│     ├── Final SL = ATR_SL × (1-weight) + LLM_SL × weight                  │
│     └── Final TP = ATR_TP × (1-weight) + LLM_TP × weight                  │
│     │                                                                     │
│     ▼                                                                     │
│  ┌───────────────────────────────────────┐                                │
│  │ Volatility Adjustment Enabled?         │                               │
│  │    NO ──────────────────────────────▶ Use blended values               │
│  │    YES                                 │                               │
│  └───────────────────────────────────────┘                                │
│     │                                                                     │
│     ▼                                                                     │
│  6. APPLY VOLATILITY ADJUSTMENT                                           │
│     ├── "widen"  → SL × 1.5, TP × 1.3                                     │
│     ├── "tighten" → SL × 0.7, TP × 0.8                                    │
│     └── "normal" → No change                                              │
│     │                                                                     │
│     ▼                                                                     │
│  7. PLACE SL/TP ORDERS ON BINANCE                                         │
│     ├── Log LLM reasoning for debugging                                   │
│     └── Track suggestion accuracy over time                               │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### UI Changes Required

**New Toggle in SL/TP Config Section:**

```
┌─────────────────────────────────────────────┐
│ SL/TP Calculation Mode                      │
│ ┌─────────────┐ ┌─────────────┐            │
│ │ Price-Based │ │ ROI-Based   │ ← Toggle   │
│ └─────────────┘ └─────────────┘            │
│                                             │
│ When ROI-Based selected:                    │
│ ┌─────────────────────────────────────────┐│
│ │ Stop Loss ROI: [ -2.0 ] %               ││
│ │ Take Profit ROI: [ 5.0 ] %              ││
│ │ ☑ Apply to all modes                    ││
│ └─────────────────────────────────────────┘│
└─────────────────────────────────────────────┘
```

### API Endpoints

| Method | Endpoint | Purpose |
|--------|----------|---------|
| GET | `/api/futures/ginie/sltp-mode` | Get current SLTP calculation mode |
| POST | `/api/futures/ginie/sltp-mode` | Set Price-Based or ROI-Based |
| POST | `/api/futures/ginie/sltp-roi/:mode` | Set ROI percentages per mode |

### Definition of Done

- [ ] **BUG FIXED**: UI settings save and persist correctly
- [ ] SLTP mode selection (Price vs ROI) implemented
- [ ] ROI calculation function tested with edge cases
- [ ] All 4 modes support ROI-based SL/TP
- [ ] UI shows selected mode and appropriate inputs
- [ ] Binance orders verified with correct prices
- [ ] Settings persist across restarts
- [ ] Trailing stop works with both modes
- [ ] **Progressive SL Movement**: SL moves to entry on TP1, to TP1 on TP2, etc.
- [ ] **LLM Adaptive**: Toggle to enable/disable LLM suggestions
- [ ] **LLM Weight Slider**: 0-100% blend with ATR calculation working
- [ ] **LLM Volatility**: Dynamic SL/TP adjustment based on market volatility
- [ ] **LLM Logging**: All suggestions logged with reasoning for debugging
- [ ] **Min Confidence**: LLM suggestions only applied when confidence >= threshold

---

## Story 2.6 Feature Summary

| Feature | Description | Key Setting |
|---------|-------------|-------------|
| **Bug Fix** | UI SL/TP settings persist after save | Fix key mismatch |
| **Price-Based SL/TP** | Existing percentage-based calculation | `sltp_calculation_mode: "price"` |
| **ROI-Based SL/TP** | Calculate from desired ROI% × leverage | `sltp_calculation_mode: "roi"` |
| **Progressive SL** | Lock profits by moving SL on each TP hit | `ginie_progressive_sl_enabled` |
| **LLM Adaptive** | AI suggests optimal SL/TP levels | `llm_adaptive_sltp_enabled` |
| **LLM Weight** | Blend ATR + LLM (0-100%) | `llm_adaptive_weight` |
| **Volatility Adjust** | Widen/tighten SL/TP based on market | `llm_volatility_adjustment` |
| **Min Confidence** | Only apply LLM if confidence >= value | `llm_min_confidence_adjust` |

---

## Story 2.7: Mode-Specific Circuit Breaker, Confidence, Timeframe & Size

### User Story

**As a** Ginie autopilot user,
**I want** each trading mode to have its own Circuit Breaker, Confidence Level, Timeframe, Position Size, Hedge Mode, Position Averaging, and Stale Position Release settings,
**So that** trades are executed with mode-appropriate risk management, capital is utilized efficiently, and the system adapts to different market conditions per mode.

### Business Value

Different trading modes require different risk profiles:
- **Ultra-Fast**: High frequency, tight controls, small sizes, quick exits
- **Scalp**: Medium frequency, moderate controls, standard sizes
- **Swing**: Lower frequency, relaxed controls, larger sizes, longer holds
- **Position**: Lowest frequency, widest controls, largest sizes, long-term holds

### Mode-Specific Configuration Matrix

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│                           GINIE MODE-SPECIFIC CONFIGURATION MATRIX                                        │
├────────────────┬─────────────────┬─────────────────┬─────────────────┬─────────────────┬─────────────────┤
│ Parameter      │ Ultra-Fast      │ Scalp           │ Swing           │ Position        │ Notes           │
├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┤
│ **TIMEFRAME**  │                 │                 │                 │                 │                 │
│ Trend TF       │ 5m              │ 15m             │ 1h              │ 4h              │ Higher TF trend │
│ Entry TF       │ 1m              │ 5m              │ 15m             │ 1h              │ Signal timing   │
│ Analysis TF    │ 1m              │ 15m             │ 4h              │ 1d              │ Pattern detect  │
├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┤
│ **CONFIDENCE** │                 │                 │                 │                 │                 │
│ Min Confidence │ 50%             │ 60%             │ 65%             │ 75%             │ Entry threshold │
│ High Conf.     │ 70%             │ 75%             │ 80%             │ 85%             │ Size multiplier │
│ Ultra Conf.    │ 85%             │ 88%             │ 90%             │ 92%             │ Max size        │
├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┤
│ **SIZE**       │                 │                 │                 │                 │                 │
│ Base Size USD  │ $100            │ $200            │ $400            │ $600            │ Per position    │
│ Max Size USD   │ $200            │ $400            │ $750            │ $1000           │ With multiplier │
│ Max Positions  │ 5               │ 4               │ 3               │ 2               │ Concurrent      │
│ Leverage       │ 10x             │ 8x              │ 5x              │ 3x              │ Risk factor     │
│ Size Multiplier│ 1.0-1.5x        │ 1.0-1.8x        │ 1.0-2.0x        │ 1.0-2.5x        │ On high conf.   │
├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┤
│ **CIRCUIT**    │                 │                 │                 │                 │                 │
│ **BREAKER**    │                 │                 │                 │                 │                 │
│ Max Loss/Hour  │ $20             │ $40             │ $80             │ $150            │ Hourly limit    │
│ Max Loss/Day   │ $50             │ $100            │ $200            │ $400            │ Daily limit     │
│ Max Consec.Loss│ 3               │ 5               │ 7               │ 10              │ Before pause    │
│ Cooldown (min) │ 15              │ 30              │ 60              │ 120             │ After trigger   │
│ Max Trades/Min │ 5               │ 3               │ 2               │ 1               │ Rate limit      │
│ Max Trades/Hr  │ 30              │ 20              │ 10              │ 5               │ Hourly limit    │
│ Max Trades/Day │ 100             │ 50              │ 20              │ 10              │ Daily limit     │
│ Win Rate Check │ After 10 trades │ After 15 trades │ After 20 trades │ After 25 trades │ Evaluation      │
│ Min Win Rate   │ 45%             │ 50%             │ 55%             │ 60%             │ Threshold       │
├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────────┤
│ **SL/TP**      │                 │                 │                 │                 │                 │
│ Stop Loss %    │ 1.0%            │ 1.5%            │ 2.5%            │ 3.5%            │ Default SL      │
│ Take Profit %  │ 2.0%            │ 3.0%            │ 5.0%            │ 8.0%            │ Default TP      │
│ Trailing Stop  │ Disabled        │ Optional        │ Enabled         │ Enabled         │ After TP1       │
│ Trail Percent  │ N/A             │ 0.5%            │ 1.5%            │ 2.5%            │ Trail distance  │
│ Max Hold Time  │ 3 seconds       │ 4 hours         │ 3 days          │ 14 days         │ Force exit      │
└────────────────┴─────────────────┴─────────────────┴─────────────────┴─────────────────┴─────────────────┘
```

---

### Unified Scanner → Analyzer → Mode Assignment Flow

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                        GINIE UNIFIED TRADING FLOW                                         │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ STEP 1: UNIFIED SCANNER                                                              │ │
│  │ ─────────────────────────                                                            │ │
│  │ • Scans ALL watchlist coins continuously (every 5 seconds)                          │ │
│  │ • Detects: Price action, Volume spikes, Pattern formations                          │ │
│  │ • Output: List of symbols with potential opportunities                               │ │
│  │ • Does NOT decide direction or mode yet                                             │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                               │                                                           │
│                               ▼                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ STEP 2: DEEP ANALYZER                                                                │ │
│  │ ─────────────────────                                                                │ │
│  │ For each potential opportunity:                                                      │ │
│  │ • Multi-timeframe analysis (1m, 5m, 15m, 1h, 4h)                                    │ │
│  │ • Calculate: RSI, MACD, ADX, Bollinger, EMA crossovers                              │ │
│  │ • Detect: Support/Resistance, Trend direction, Volatility regime                    │ │
│  │ • LLM Analysis (if enabled): Market sentiment, News impact                          │ │
│  │                                                                                       │ │
│  │ Output per symbol:                                                                   │ │
│  │ {                                                                                    │ │
│  │   symbol: "BTCUSDT",                                                                │ │
│  │   direction: "LONG",                                                                │ │
│  │   confidence: 72,                                                                   │ │
│  │   risk_score: 35,        // 0-100 (lower = safer)                                   │ │
│  │   volatility: "medium",                                                             │ │
│  │   expected_hold: "2h",                                                              │ │
│  │   profit_potential: 2.5, // percentage                                              │ │
│  │   trend_strength: 68                                                                │ │
│  │ }                                                                                    │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                               │                                                           │
│                               ▼                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ STEP 3: MODE ASSIGNMENT ENGINE                                                       │ │
│  │ ──────────────────────────────                                                       │ │
│  │                                                                                       │ │
│  │ Assignment Rules:                                                                    │ │
│  │ ┌───────────────────────────────────────────────────────────────────────────────┐   │ │
│  │ │ ULTRA-FAST Assignment:                                                         │   │ │
│  │ │ • Volatility: HIGH or EXTREME                                                  │   │ │
│  │ │ • Expected Hold: < 5 minutes                                                   │   │ │
│  │ │ • Confidence: 50-70%                                                           │   │ │
│  │ │ • Risk Score: < 50                                                             │   │ │
│  │ │ • Quick profit potential: 0.5-2%                                               │   │ │
│  │ └───────────────────────────────────────────────────────────────────────────────┘   │ │
│  │ ┌───────────────────────────────────────────────────────────────────────────────┐   │ │
│  │ │ SCALP Assignment:                                                              │   │ │
│  │ │ • Volatility: MEDIUM to HIGH                                                   │   │ │
│  │ │ • Expected Hold: 15 min - 4 hours                                              │   │ │
│  │ │ • Confidence: 60-75%                                                           │   │ │
│  │ │ • Risk Score: < 45                                                             │   │ │
│  │ │ • Profit potential: 1-3%                                                       │   │ │
│  │ └───────────────────────────────────────────────────────────────────────────────┘   │ │
│  │ ┌───────────────────────────────────────────────────────────────────────────────┐   │ │
│  │ │ SWING Assignment:                                                              │   │ │
│  │ │ • Volatility: LOW to MEDIUM                                                    │   │ │
│  │ │ • Expected Hold: 4 hours - 3 days                                              │   │ │
│  │ │ • Confidence: 65-85%                                                           │   │ │
│  │ │ • Risk Score: < 40                                                             │   │ │
│  │ │ • Profit potential: 3-8%                                                       │   │ │
│  │ │ • Trend alignment required                                                     │   │ │
│  │ └───────────────────────────────────────────────────────────────────────────────┘   │ │
│  │ ┌───────────────────────────────────────────────────────────────────────────────┐   │ │
│  │ │ POSITION Assignment:                                                           │   │ │
│  │ │ • Volatility: LOW                                                              │   │ │
│  │ │ • Expected Hold: 3+ days                                                       │   │ │
│  │ │ • Confidence: 75%+                                                             │   │ │
│  │ │ • Risk Score: < 30                                                             │   │ │
│  │ │ • Profit potential: 5-15%                                                      │   │ │
│  │ │ • Strong trend + High timeframe confirmation                                   │   │ │
│  │ └───────────────────────────────────────────────────────────────────────────────┘   │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                               │                                                           │
│                               ▼                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ STEP 4: CONFLICT RESOLUTION (Hybrid Approach)                                        │ │
│  │ ─────────────────────────────────────────────                                        │ │
│  │ Check: Does symbol already have an active position?                                 │ │
│  │                                                                                       │ │
│  │ → See "Hybrid Conflict Resolution" section below                                     │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                               │                                                           │
│                               ▼                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ STEP 5: MODE-SPECIFIC EXECUTION                                                      │ │
│  │ ─────────────────────────────────                                                    │ │
│  │ • Apply assigned mode's settings (locked until close)                               │ │
│  │ • Set leverage, position size per mode config                                       │ │
│  │ • Place SL/TP orders per mode config                                                │ │
│  │ • Enable trailing if mode allows                                                    │ │
│  │ • Track under mode's circuit breaker                                                │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Hedge Mode Configuration (LONG + SHORT Simultaneously)

#### How Hedge Mode Permission Works

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                       HEDGE MODE: USER PERMISSION & ACTIVATION FLOW                       │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  STEP 1: BINANCE ACCOUNT SETUP (One-Time)                                                │
│  ─────────────────────────────────────────                                                │
│  Before hedging can work, user MUST enable Hedge Mode on Binance:                         │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Binance Futures → Settings → Position Mode → Select "Hedge Mode"                   │  │
│  │                                                                                     │  │
│  │ ⚠️ IMPORTANT: This is a Binance account setting, NOT our bot setting              │  │
│  │ • One-Way Mode (default): Only LONG OR SHORT per symbol                            │  │
│  │ • Hedge Mode: BOTH LONG AND SHORT allowed per symbol                               │  │
│  │                                                                                     │  │
│  │ API Check: GET /fapi/v1/positionSide/dual → {"dualSidePosition": true}             │  │
│  └────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
│  STEP 2: GINIE HEDGE SETTINGS (Per Mode - User Configurable)                             │
│  ────────────────────────────────────────────────────────────                             │
│  Each mode has independent hedge settings that user can customize:                        │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │                         GINIE PANEL → MODE CONFIGURATION                           │  │
│  │                                                                                     │  │
│  │  Mode: [Ultra-Fast ▼]                                                              │  │
│  │                                                                                     │  │
│  │  🔀 HEDGE MODE SETTINGS                                                            │  │
│  │  ┌─────────────────────────────────────────────────────────────────────────────┐   │  │
│  │  │ ☑ Allow Hedge Mode                    ← Master toggle for this mode         │   │  │
│  │  │                                                                              │   │  │
│  │  │ Min Confidence for Hedge:    [70 ]%   ← Higher than normal entry            │   │  │
│  │  │ Existing Position Must Be:   [Any ▼]  ← Options: Any, >0%, >1%, >2% profit  │   │  │
│  │  │ Max Hedge Size:              [100]%   ← Percentage of base position size    │   │  │
│  │  │                                                                              │   │  │
│  │  │ ☐ Require Manual Confirmation         ← If checked, popup before hedge      │   │  │
│  │  │ ☐ Allow Same-Mode Hedge               ← Usually disabled (risky)            │   │  │
│  │  │                                                                              │   │  │
│  │  │ Max Total Exposure:          [2.0 ]x  ← Cap: 2x normal allocation           │   │  │
│  │  └─────────────────────────────────────────────────────────────────────────────┘   │  │
│  │                                                                                     │  │
│  │                                  [Save Settings]                                    │  │
│  └────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
│  STEP 3: AUTOMATIC HEDGE TRIGGER (When Scanner Detects Opportunity)                       │
│  ───────────────────────────────────────────────────────────────────                      │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │                           HEDGE DECISION FLOW                                       │  │
│  │                                                                                     │  │
│  │                        ┌──────────────────────┐                                    │  │
│  │                        │   Scanner detects    │                                    │  │
│  │                        │   opportunity for    │                                    │  │
│  │                        │   BTCUSDT SHORT      │                                    │  │
│  │                        └──────────┬───────────┘                                    │  │
│  │                                   │                                                │  │
│  │                                   ▼                                                │  │
│  │                 ┌─────────────────────────────────────┐                           │  │
│  │                 │ Check: Does BTCUSDT have existing   │                           │  │
│  │                 │ LONG position in any mode?          │                           │  │
│  │                 └─────────────────┬───────────────────┘                           │  │
│  │                       ┌───────────┴───────────┐                                   │  │
│  │                       │                       │                                   │  │
│  │                      YES                      NO                                  │  │
│  │                       │                       │                                   │  │
│  │                       ▼                       ▼                                   │  │
│  │    ┌──────────────────────────────┐   ┌──────────────────────┐                   │  │
│  │    │ THIS IS A HEDGE SCENARIO     │   │ Normal trade flow    │                   │  │
│  │    │ (opposite direction exists)  │   │ (no conflict)        │                   │  │
│  │    └──────────────┬───────────────┘   └──────────────────────┘                   │  │
│  │                   │                                                               │  │
│  │                   ▼                                                               │  │
│  │    ┌──────────────────────────────────────────────────────────┐                  │  │
│  │    │ HEDGE PERMISSION CHECKS (All must pass)                   │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 1. Binance Hedge Mode enabled?                           │                  │  │
│  │    │    └─ API: dualSidePosition == true                      │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 2. New mode allows hedging?                               │                  │  │
│  │    │    └─ Check: newModeConfig.Hedge.AllowHedge == true      │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 3. Signal confidence meets hedge threshold?               │                  │  │
│  │    │    └─ Check: signal.Confidence >= MinConfidenceForHedge  │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 4. Existing position meets profit requirement?            │                  │  │
│  │    │    └─ Check: existingPnL% >= ExistingMustBeInProfit      │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 5. Not same mode hedge? (unless explicitly allowed)       │                  │  │
│  │    │    └─ Check: existingMode != newMode OR AllowSameModeHedge│                  │  │
│  │    │                                                           │                  │  │
│  │    │ 6. Total exposure within limit?                           │                  │  │
│  │    │    └─ Check: (existing + new) <= MaxTotalExposure        │                  │  │
│  │    └──────────────────────────────────────────────────────────┘                  │  │
│  │                   │                                                               │  │
│  │         ┌─────────┴─────────┐                                                    │  │
│  │         │                   │                                                    │  │
│  │    ALL PASS             ANY FAIL                                                 │  │
│  │         │                   │                                                    │  │
│  │         ▼                   ▼                                                    │  │
│  │    ┌────────────┐    ┌─────────────────────────┐                                │  │
│  │    │ Proceed to │    │ BLOCK TRADE             │                                │  │
│  │    │ Confirmation│    │ Log reason:             │                                │  │
│  │    │ Check      │    │ "Hedge blocked: [reason]"│                                │  │
│  │    └─────┬──────┘    └─────────────────────────┘                                │  │
│  │          │                                                                        │  │
│  │          ▼                                                                        │  │
│  │    ┌──────────────────────────────────────────────────────────┐                  │  │
│  │    │ MANUAL CONFIRMATION CHECK                                 │                  │  │
│  │    │                                                           │                  │  │
│  │    │ If RequireManualConfirmation == true:                     │                  │  │
│  │    │ ┌─────────────────────────────────────────────────────┐   │                  │  │
│  │    │ │           🔀 HEDGE CONFIRMATION                      │   │                  │  │
│  │    │ │                                                      │   │                  │  │
│  │    │ │  Existing: BTCUSDT LONG (Swing) @ $95,000           │   │                  │  │
│  │    │ │            Size: $400, PnL: +2.5% (+$10.00)         │   │                  │  │
│  │    │ │                                                      │   │                  │  │
│  │    │ │  Proposed: BTCUSDT SHORT (Ultra-Fast)               │   │                  │  │
│  │    │ │            Size: $100, Confidence: 72%               │   │                  │  │
│  │    │ │            Entry: $97,375                            │   │                  │  │
│  │    │ │                                                      │   │                  │  │
│  │    │ │  ⚠️ This will create opposing positions              │   │                  │  │
│  │    │ │                                                      │   │                  │  │
│  │    │ │  [Cancel]  [Confirm Hedge]  [Auto-approve 5 min]    │   │                  │  │
│  │    │ └─────────────────────────────────────────────────────┘   │                  │  │
│  │    │                                                           │                  │  │
│  │    │ If RequireManualConfirmation == false:                    │                  │  │
│  │    │ └─ Skip confirmation, execute immediately                 │                  │  │
│  │    │                                                           │                  │  │
│  │    └──────────────────────────────────────────────────────────┘                  │  │
│  │          │                                                                        │  │
│  │          ▼                                                                        │  │
│  │    ┌──────────────────────────────────────────────────────────┐                  │  │
│  │    │ EXECUTE HEDGE TRADE                                       │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 1. Calculate hedge position size:                         │                  │  │
│  │    │    hedgeSize = min(baseSizeUSD, existingSize × maxHedge%) │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 2. Place order with positionSide:                         │                  │  │
│  │    │    {                                                      │                  │  │
│  │    │      "symbol": "BTCUSDT",                                 │                  │  │
│  │    │      "side": "SELL",                                      │                  │  │
│  │    │      "positionSide": "SHORT",  ← KEY: Hedge Mode param   │                  │  │
│  │    │      "type": "MARKET",                                    │                  │  │
│  │    │      "quantity": 0.001                                    │                  │  │
│  │    │    }                                                      │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 3. Set SL/TP per NEW mode's configuration                │                  │  │
│  │    │    (Ultra-Fast settings, NOT Swing settings)             │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 4. Track under NEW mode's circuit breaker                 │                  │  │
│  │    │                                                           │                  │  │
│  │    │ 5. Log hedge creation for monitoring                      │                  │  │
│  │    └──────────────────────────────────────────────────────────┘                  │  │
│  └────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
│  API ENDPOINTS FOR HEDGE CONTROL:                                                         │
│  ─────────────────────────────────                                                        │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Endpoint                                      │ Method │ Description               │  │
│  ├───────────────────────────────────────────────┼────────┼──────────────────────────┤  │
│  │ /api/futures/ginie/hedge/status               │ GET    │ Check if hedge mode enabled│  │
│  │ /api/futures/ginie/hedge/binance-check        │ GET    │ Check Binance hedge status │  │
│  │ /api/futures/ginie/mode-config/:mode/hedge    │ PUT    │ Update hedge settings     │  │
│  │ /api/futures/ginie/hedge/pending              │ GET    │ Get pending confirmations │  │
│  │ /api/futures/ginie/hedge/confirm/:id          │ POST   │ Confirm pending hedge     │  │
│  │ /api/futures/ginie/hedge/reject/:id           │ POST   │ Reject pending hedge      │  │
│  │ /api/futures/ginie/hedge/history              │ GET    │ Get hedge trade history   │  │
│  └───────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

#### Hedge Mode Settings Reference

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                              HEDGE MODE CONFIGURATION                                     │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  Binance Hedge Mode allows BOTH LONG and SHORT positions on the same symbol              │
│  simultaneously. Each position is managed independently under its assigned mode.          │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ HEDGE MODE SETTINGS (Per Mode)                                                      │  │
│  ├────────────────┬─────────────────┬─────────────────┬─────────────────┬─────────────┤  │
│  │ Setting        │ Ultra-Fast      │ Scalp           │ Swing           │ Position    │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Allow Hedge    │ ✅ Yes          │ ✅ Yes          │ ✅ Yes          │ ⚠️ Cautious │  │
│  │ Min Confidence │ 70%             │ 75%             │ 80%             │ 85%         │  │
│  │ for Hedge      │                 │                 │                 │             │  │
│  │ Existing Must  │ Any             │ > 0%            │ > 1%            │ > 2%        │  │
│  │ Be In Profit   │                 │                 │                 │             │  │
│  │ Max Hedge Size │ 100% of orig    │ 75% of orig     │ 50% of orig     │ 50% of orig │  │
│  │ Same Mode Hedge│ ❌ No           │ ❌ No           │ ❌ No           │ ❌ No       │  │
│  └────────────────┴─────────────────┴─────────────────┴─────────────────┴─────────────┘  │
│                                                                                           │
│  HEDGE RULES:                                                                             │
│  1. Cannot hedge within SAME mode (no Ultra-Fast LONG + Ultra-Fast SHORT)                │
│  2. New hedge position must meet its own mode's confidence threshold                     │
│  3. Existing position should be in profit before allowing hedge (per mode setting)       │
│  4. Each position tracked under its own mode's circuit breaker                           │
│  5. Total exposure per symbol capped at 2x normal allocation                             │
│                                                                                           │
│  EXAMPLE:                                                                                 │
│  ├── BTCUSDT LONG in Swing mode ($400, +2.5% profit)                                    │
│  ├── Scanner detects Ultra-Fast SHORT opportunity (confidence: 72%)                      │
│  ├── Check: Swing profit > 1%? ✅ Yes (2.5%)                                            │
│  ├── Check: Ultra-Fast confidence > 70%? ✅ Yes (72%)                                   │
│  └── Result: ✅ OPEN HEDGE - Ultra-Fast SHORT ($100, max 100% of UF base)               │
│                                                                                           │
│  NOW ACTIVE:                                                                              │
│  ├── Position 1: BTCUSDT LONG (Swing) - Swing SL/TP/Trailing applied                    │
│  └── Position 2: BTCUSDT SHORT (Ultra-Fast) - Ultra-Fast SL/TP applied                  │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Position Averaging Configuration (Add to Existing Position)

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                           POSITION AVERAGING CONFIGURATION                                │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  Position averaging allows ADDING to an existing position in the SAME direction          │
│  when a new opportunity is detected. Each mode has specific rules.                        │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ AVERAGING SETTINGS (Per Mode)                                                       │  │
│  ├────────────────┬─────────────────┬─────────────────┬─────────────────┬─────────────┤  │
│  │ Setting        │ Ultra-Fast      │ Scalp           │ Swing           │ Position    │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Allow Average  │ ❌ No           │ ✅ Yes          │ ✅ Yes          │ ✅ Yes      │  │
│  │ (too fast)     │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Avg When In    │ N/A             │ Profit > 0.5%   │ Profit > 1%     │ Profit > 2% │  │
│  │ Profit (%)     │                 │ OR Loss < -1%   │ OR Loss < -1.5% │ OR Loss <-2%│  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Add Size (%)   │ N/A             │ 50% of original │ 50% of original │ 30% of orig │  │
│  │ of Original    │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Max Averages   │ 0               │ 2               │ 3               │ 2           │  │
│  │ Per Position   │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Min Confidence │ N/A             │ 70%             │ 75%             │ 80%         │  │
│  │ for Average    │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Recalc SL/TP   │ N/A             │ ✅ Yes          │ ✅ Yes          │ ✅ Yes      │  │
│  │ After Average  │                 │ (new avg entry) │ (new avg entry) │(new avg ent)│  │
│  └────────────────┴─────────────────┴─────────────────┴─────────────────┴─────────────┘  │
│                                                                                           │
│  AVERAGING LOGIC:                                                                         │
│                                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ AVERAGE UP (Position in Profit)                                                      │ │
│  │ ─────────────────────────────────                                                    │ │
│  │ • New opportunity SAME direction + position profitable                               │ │
│  │ • Strengthens winning position                                                       │ │
│  │ • New avg entry = (old_entry × old_qty + new_entry × new_qty) / total_qty           │ │
│  │ • SL/TP recalculated from new average entry                                         │ │
│  │                                                                                       │ │
│  │ Example:                                                                             │ │
│  │ ├── Existing: BTCUSDT LONG @ $100, qty: 0.1, now +1.5%                              │ │
│  │ ├── New signal: LONG @ $101.50, confidence: 75%                                     │ │
│  │ ├── Add: 50% of original = 0.05 BTC @ $101.50                                       │ │
│  │ ├── New avg entry: ($100×0.1 + $101.50×0.05) / 0.15 = $100.50                       │ │
│  │ └── Recalculate SL/TP from $100.50                                                  │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ AVERAGE DOWN (Position in Loss - DCA Style)                                          │ │
│  │ ───────────────────────────────────────────                                          │ │
│  │ • New opportunity SAME direction + position in acceptable loss range                 │ │
│  │ • Lowers average entry to recover faster                                             │ │
│  │ • ⚠️ RISKY - Only if new signal has HIGH confidence                                 │ │
│  │                                                                                       │ │
│  │ Example:                                                                             │ │
│  │ ├── Existing: BTCUSDT LONG @ $100, qty: 0.1, now -1.2%                              │ │
│  │ ├── New signal: LONG @ $98.80, confidence: 78%                                      │ │
│  │ ├── Check: Loss (-1.2%) within limit (-1.5%)? ✅                                    │ │
│  │ ├── Add: 50% of original = 0.05 BTC @ $98.80                                        │ │
│  │ ├── New avg entry: ($100×0.1 + $98.80×0.05) / 0.15 = $99.60                         │ │
│  │ └── Break-even now at $99.60 instead of $100 ✅                                     │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Stale Position Release (Capital Liberation)

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                        STALE POSITION RELEASE (Capital Liberation)                        │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  Positions that occupy capital for too long with minimal P&L should be released          │
│  to free up capital for better opportunities. Each mode has different tolerances.        │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ STALE POSITION SETTINGS (Per Mode)                                                  │  │
│  ├────────────────┬─────────────────┬─────────────────┬─────────────────┬─────────────┤  │
│  │ Setting        │ Ultra-Fast      │ Scalp           │ Swing           │ Position    │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Enable Stale   │ ✅ Yes          │ ✅ Yes          │ ✅ Yes          │ ✅ Yes      │  │
│  │ Release        │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Max Hold Time  │ 10 seconds      │ 6 hours         │ 5 days          │ 21 days     │  │
│  │ Before Review  │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Min Profit to  │ 0.3%            │ 0.5%            │ 1.0%            │ 2.0%        │  │
│  │ Keep Position  │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Max Loss to    │ -0.5%           │ -1.0%           │ -1.5%           │ -2.0%       │  │
│  │ Force Close    │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Stale Zone     │ -0.3% to +0.3%  │ -0.5% to +0.5%  │ -1% to +1%      │ -1.5%to+1.5%│  │
│  │ (Auto-Close)   │                 │                 │                 │             │  │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Extend Time If │ N/A             │ Trend still     │ Trend still     │ Trend still │  │
│  │ Conditions Met │                 │ aligned         │ aligned + ADX>25│ aligned+ADX>30│ │
│  ├────────────────┼─────────────────┼─────────────────┼─────────────────┼─────────────┤  │
│  │ Extension Time │ N/A             │ +2 hours        │ +1 day          │ +3 days     │  │
│  └────────────────┴─────────────────┴─────────────────┴─────────────────┴─────────────┘  │
│                                                                                           │
│  STALE POSITION DECISION FLOW:                                                            │
│                                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │                                                                                       │ │
│  │  Position exceeds Max Hold Time                                                      │ │
│  │       │                                                                               │ │
│  │       ▼                                                                               │ │
│  │  Check Current P&L                                                                   │ │
│  │       │                                                                               │ │
│  │       ├── P&L >= Min Profit to Keep (e.g., +1%)                                     │ │
│  │       │       └── ✅ KEEP - Position is performing well                             │ │
│  │       │                                                                               │ │
│  │       ├── P&L <= Max Loss to Force Close (e.g., -1.5%)                              │ │
│  │       │       └── 🛑 CLOSE - Cut losses, free capital                               │ │
│  │       │                                                                               │ │
│  │       └── P&L in Stale Zone (e.g., -1% to +1%)                                      │ │
│  │               │                                                                       │ │
│  │               ▼                                                                       │ │
│  │           Check Extension Conditions                                                 │ │
│  │               │                                                                       │ │
│  │               ├── Trend still aligned + ADX strong?                                 │ │
│  │               │       └── ⏰ EXTEND - Give more time                                │ │
│  │               │                                                                       │ │
│  │               └── Conditions NOT met?                                               │ │
│  │                       └── 🔄 CLOSE - Release capital for better use                 │ │
│  │                                                                                       │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                           │
│  EXAMPLE - Swing Mode Stale Position:                                                     │
│  ├── ETHUSDT LONG opened 5 days ago @ $3,400                                            │
│  ├── Current price: $3,420 (+0.6% profit)                                               │
│  ├── Max Hold Time: 5 days ✅ Exceeded                                                  │
│  ├── Min Profit to Keep: 1.0% ❌ Only 0.6%                                              │
│  ├── Stale Zone: -1% to +1% ✅ In stale zone                                            │
│  ├── Check trend: Still bullish? ✅ Yes                                                 │
│  ├── Check ADX: > 25? ❌ ADX = 22                                                       │
│  └── DECISION: 🔄 CLOSE at +0.6% - Trend weak, release capital                         │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Hybrid Conflict Resolution (Smart Decision Engine)

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                      HYBRID CONFLICT RESOLUTION - DECISION ENGINE                         │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  When a new opportunity is detected for a symbol that already has an active position,    │
│  the system uses this decision tree to determine the best action.                        │
│                                                                                           │
│  ┌─────────────────────────────────────────────────────────────────────────────────────┐ │
│  │                                                                                       │ │
│  │  NEW OPPORTUNITY DETECTED                                                            │ │
│  │       │                                                                               │ │
│  │       ▼                                                                               │ │
│  │  ┌───────────────────────────────────────────────────────────────────────────────┐   │ │
│  │  │ Does symbol have an EXISTING position?                                         │   │ │
│  │  │     │                                                                           │   │ │
│  │  │     ├── NO ────────────────────────────────────▶ EXECUTE NEW TRADE            │   │ │
│  │  │     │                                             (normal flow)                │   │ │
│  │  │     │                                                                           │   │ │
│  │  │     └── YES                                                                     │   │ │
│  │  │           │                                                                     │   │ │
│  │  │           ▼                                                                     │   │ │
│  │  │     ┌─────────────────────────────────────────────────────────────────────┐    │   │ │
│  │  │     │ Is new direction SAME as existing?                                   │    │   │ │
│  │  │     │     │                                                                 │    │   │ │
│  │  │     │     ├── YES (LONG + LONG or SHORT + SHORT)                           │    │   │ │
│  │  │     │     │     │                                                           │    │   │ │
│  │  │     │     │     ▼                                                           │    │   │ │
│  │  │     │     │   ┌───────────────────────────────────────────────────────┐    │    │   │ │
│  │  │     │     │   │ CHECK AVERAGING CONDITIONS                             │    │    │   │ │
│  │  │     │     │   │                                                         │    │    │   │ │
│  │  │     │     │   │ Mode allows averaging?                                  │    │    │   │ │
│  │  │     │     │   │   NO ──────▶ BLOCK (log: "averaging disabled")         │    │    │   │ │
│  │  │     │     │   │   YES                                                   │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     ▼                                                   │    │    │   │ │
│  │  │     │     │   │ Max averages reached?                                   │    │    │   │ │
│  │  │     │     │   │   YES ─────▶ BLOCK (log: "max averages reached")       │    │    │   │ │
│  │  │     │     │   │   NO                                                    │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     ▼                                                   │    │    │   │ │
│  │  │     │     │   │ Position in acceptable P&L range for averaging?         │    │    │   │ │
│  │  │     │     │   │   NO ──────▶ BLOCK (log: "P&L outside avg range")      │    │    │   │ │
│  │  │     │     │   │   YES                                                   │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     ▼                                                   │    │    │   │ │
│  │  │     │     │   │ New confidence >= mode's avg threshold?                 │    │    │   │ │
│  │  │     │     │   │   NO ──────▶ BLOCK (log: "confidence too low")         │    │    │   │ │
│  │  │     │     │   │   YES                                                   │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     ▼                                                   │    │    │   │ │
│  │  │     │     │   │ ✅ AVERAGE: Add to position                            │    │    │   │ │
│  │  │     │     │   │    • Add configured % of original size                 │    │    │   │ │
│  │  │     │     │   │    • Recalculate average entry                         │    │    │   │ │
│  │  │     │     │   │    • Update SL/TP from new average                     │    │    │   │ │
│  │  │     │     │   └───────────────────────────────────────────────────────┘    │    │   │ │
│  │  │     │     │                                                                 │    │   │ │
│  │  │     │     └── NO (OPPOSITE: LONG + SHORT or SHORT + LONG)                  │    │   │ │
│  │  │     │           │                                                           │    │   │ │
│  │  │     │           ▼                                                           │    │   │ │
│  │  │     │     ┌───────────────────────────────────────────────────────────┐    │    │   │ │
│  │  │     │     │ CHECK HEDGE OR OVERRIDE CONDITIONS                         │    │    │   │ │
│  │  │     │     │                                                             │    │    │   │ │
│  │  │     │     │ Is existing position in PROFIT?                             │    │    │   │ │
│  │  │     │     │   │                                                         │    │    │   │ │
│  │  │     │     │   ├── YES (Profitable)                                      │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     ▼                                                   │    │    │   │ │
│  │  │     │     │   │   Profit >= mode's hedge threshold?                     │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     ├── YES + New confidence >= hedge min               │    │    │   │ │
│  │  │     │     │   │     │     │                                             │    │    │   │ │
│  │  │     │     │   │     │     ▼                                             │    │    │   │ │
│  │  │     │     │   │     │   ✅ HEDGE: Open opposite direction               │    │    │   │ │
│  │  │     │     │   │     │      • Use new mode's settings                    │    │    │   │ │
│  │  │     │     │   │     │      • Both positions active                      │    │    │   │ │
│  │  │     │     │   │     │      • Each managed by its mode                   │    │    │   │ │
│  │  │     │     │   │     │                                                   │    │    │   │ │
│  │  │     │     │   │     └── NO                                              │    │    │   │ │
│  │  │     │     │   │           └── BLOCK (log: "profit/conf too low")       │    │    │   │ │
│  │  │     │     │   │                                                         │    │    │   │ │
│  │  │     │     │   └── NO (Break-even or Loss)                               │    │    │   │ │
│  │  │     │     │         │                                                   │    │    │   │ │
│  │  │     │     │         ▼                                                   │    │    │   │ │
│  │  │     │     │       Calculate Priority Scores                             │    │    │   │ │
│  │  │     │     │       • Existing: Confidence × Mode Weight × (1 + P&L%)    │    │    │   │ │
│  │  │     │     │       • New: Confidence × Mode Weight × Profit Potential   │    │    │   │ │
│  │  │     │     │         │                                                   │    │    │   │ │
│  │  │     │     │         ▼                                                   │    │    │   │ │
│  │  │     │     │       New Score > Existing Score × 1.5? (50% better)       │    │    │   │ │
│  │  │     │     │         │                                                   │    │    │   │ │
│  │  │     │     │         ├── YES                                             │    │    │   │ │
│  │  │     │     │         │     │                                             │    │    │   │ │
│  │  │     │     │         │     ▼                                             │    │    │   │ │
│  │  │     │     │         │   🔄 OVERRIDE: Close existing, Open new          │    │    │   │ │
│  │  │     │     │         │      • Close existing at market                   │    │    │   │ │
│  │  │     │     │         │      • Open new in opposite direction            │    │    │   │ │
│  │  │     │     │         │      • Log: "Override - better opportunity"      │    │    │   │ │
│  │  │     │     │         │                                                   │    │    │   │ │
│  │  │     │     │         └── NO                                              │    │    │   │ │
│  │  │     │     │               └── BLOCK (log: "not enough improvement")    │    │    │   │ │
│  │  │     │     │                                                             │    │    │   │ │
│  │  │     │     └───────────────────────────────────────────────────────────┘    │    │   │ │
│  │  │     │                                                                       │    │   │ │
│  │  │     └─────────────────────────────────────────────────────────────────────┘    │   │ │
│  │  │                                                                                 │   │ │
│  │  └───────────────────────────────────────────────────────────────────────────────┘   │ │
│  │                                                                                       │ │
│  └─────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                           │
│  PRIORITY SCORE CALCULATION:                                                              │
│  ─────────────────────────────                                                            │
│  Mode Weights: Ultra-Fast=0.8, Scalp=1.0, Swing=1.2, Position=1.5                        │
│                                                                                           │
│  Existing Position Score = Confidence × ModeWeight × (1 + CurrentP&L/100)                │
│  New Opportunity Score = Confidence × ModeWeight × (1 + ExpectedProfit/100)              │
│                                                                                           │
│  Example:                                                                                 │
│  • Existing: Scalp LONG, 65% conf, -0.5% loss → 65 × 1.0 × 0.995 = 64.7                 │
│  • New: Swing SHORT, 78% conf, 4% potential → 78 × 1.2 × 1.04 = 97.3                    │
│  • Ratio: 97.3 / 64.7 = 1.50 (exactly 50% better)                                       │
│  • Decision: OVERRIDE ✅                                                                 │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

### Circuit Breaker Risk Assessment Per Mode

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                        GINIE CIRCUIT BREAKER - MODE RISK ASSESSMENT                       │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  ULTRA-FAST MODE - "AGGRESSIVE PROTECTION"                                                │
│  ─────────────────────────────────────────                                                │
│  Risk Level: HIGH (many trades, small size, tight limits)                                 │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Trigger Condition              │ Action                    │ Recovery              │  │
│  ├────────────────────────────────┼───────────────────────────┼───────────────────────┤  │
│  │ 3 consecutive losses           │ PAUSE 15 min              │ Auto-resume           │  │
│  │ $20 loss in 1 hour             │ STOP ultra-fast for hour  │ Reset next hour       │  │
│  │ $50 loss in 1 day              │ DISABLE ultra-fast today  │ Reset at midnight     │  │
│  │ Win rate < 45% (10+ trades)    │ PAUSE + Alert             │ Manual review needed  │  │
│  │ 5 trades in 1 minute           │ Rate limit triggered      │ Wait 1 min            │  │
│  └────────────────────────────────┴───────────────────────────┴───────────────────────┘  │
│                                                                                           │
│  SCALP MODE - "BALANCED PROTECTION"                                                       │
│  ───────────────────────────────────                                                      │
│  Risk Level: MEDIUM (moderate trades, standard limits)                                    │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Trigger Condition              │ Action                    │ Recovery              │  │
│  ├────────────────────────────────┼───────────────────────────┼───────────────────────┤  │
│  │ 5 consecutive losses           │ PAUSE 30 min              │ Auto-resume           │  │
│  │ $40 loss in 1 hour             │ PAUSE scalp for hour      │ Reset next hour       │  │
│  │ $100 loss in 1 day             │ DISABLE scalp today       │ Reset at midnight     │  │
│  │ Win rate < 50% (15+ trades)    │ PAUSE + reduce size 50%   │ Review after 10 wins  │  │
│  │ 3 trades in 1 minute           │ Rate limit triggered      │ Wait 1 min            │  │
│  └────────────────────────────────┴───────────────────────────┴───────────────────────┘  │
│                                                                                           │
│  SWING MODE - "RELAXED PROTECTION"                                                        │
│  ─────────────────────────────────                                                        │
│  Risk Level: LOWER (fewer trades, larger sizes, wider limits)                             │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Trigger Condition              │ Action                    │ Recovery              │  │
│  ├────────────────────────────────┼───────────────────────────┼───────────────────────┤  │
│  │ 7 consecutive losses           │ PAUSE 60 min              │ Auto-resume           │  │
│  │ $80 loss in 1 hour             │ PAUSE swing for 2 hours   │ Auto-resume           │  │
│  │ $200 loss in 1 day             │ DISABLE swing today       │ Reset at midnight     │  │
│  │ Win rate < 55% (20+ trades)    │ PAUSE + LLM re-evaluation │ After LLM approval    │  │
│  │ 2 trades in 1 minute           │ Rate limit triggered      │ Wait 1 min            │  │
│  └────────────────────────────────┴───────────────────────────┴───────────────────────┘  │
│                                                                                           │
│  POSITION MODE - "CONSERVATIVE PROTECTION"                                                │
│  ──────────────────────────────────────────                                               │
│  Risk Level: LOWEST (few trades, largest sizes, widest limits)                            │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Trigger Condition              │ Action                    │ Recovery              │  │
│  ├────────────────────────────────┼───────────────────────────┼───────────────────────┤  │
│  │ 10 consecutive losses          │ PAUSE 120 min             │ Manual resume only    │  │
│  │ $150 loss in 1 hour            │ PAUSE position for day    │ Reset at midnight     │  │
│  │ $400 loss in 1 day             │ DISABLE position 48 hours │ Manual override       │  │
│  │ Win rate < 60% (25+ trades)    │ FULL STOP + Alert         │ Manual review only    │  │
│  │ 1 trade in 1 minute            │ Rate limit (expected)     │ Normal behavior       │  │
│  └────────────────────────────────┴───────────────────────────┴───────────────────────┘  │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

### Confidence Level Decision Flow

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                        CONFIDENCE-BASED TRADE EXECUTION FLOW                              │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  SIGNAL GENERATED                                                                         │
│       │                                                                                   │
│       ▼                                                                                   │
│  ┌─────────────────────────────────────────────┐                                          │
│  │ Get Mode-Specific Min Confidence Threshold   │                                          │
│  │ ├── Ultra-Fast: 50%                          │                                          │
│  │ ├── Scalp: 60%                               │                                          │
│  │ ├── Swing: 65%                               │                                          │
│  │ └── Position: 75%                            │                                          │
│  └─────────────────────────────────────────────┘                                          │
│       │                                                                                   │
│       ▼                                                                                   │
│  ┌─────────────────────────────────────────────┐                                          │
│  │ Signal Confidence >= Min Threshold?          │                                          │
│  │    NO ───────────────────────────▶ REJECT TRADE                                        │
│  │    YES                                       │                                          │
│  └─────────────────────────────────────────────┘                                          │
│       │                                                                                   │
│       ▼                                                                                   │
│  ┌─────────────────────────────────────────────┐                                          │
│  │ Calculate Position Size Based on Confidence  │                                          │
│  │                                               │                                          │
│  │ Base Size = Mode Base USD                     │                                          │
│  │                                               │                                          │
│  │ If Confidence >= High Threshold:              │                                          │
│  │   Size = Base × 1.5                           │                                          │
│  │                                               │                                          │
│  │ If Confidence >= Ultra Threshold:             │                                          │
│  │   Size = Base × Mode Max Multiplier           │                                          │
│  │   (capped at Max Size USD)                    │                                          │
│  └─────────────────────────────────────────────┘                                          │
│       │                                                                                   │
│       ▼                                                                                   │
│  ┌─────────────────────────────────────────────┐                                          │
│  │ Apply Mode-Specific SL/TP                     │                                          │
│  │ ├── Get SL% and TP% for mode                  │                                          │
│  │ ├── Apply LLM adjustment (if enabled)         │                                          │
│  │ └── Calculate prices based on entry           │                                          │
│  └─────────────────────────────────────────────┘                                          │
│       │                                                                                   │
│       ▼                                                                                   │
│  ┌─────────────────────────────────────────────┐                                          │
│  │ Check Circuit Breaker for Mode                │                                          │
│  │ ├── Loss limits OK?                           │                                          │
│  │ ├── Rate limits OK?                           │                                          │
│  │ ├── Win rate OK?                              │                                          │
│  │ └── Consecutive loss OK?                      │                                          │
│  │    NO ───────────────────────────▶ BLOCK TRADE (log reason)                            │
│  │    YES                                        │                                          │
│  └─────────────────────────────────────────────┘                                          │
│       │                                                                                   │
│       ▼                                                                                   │
│  EXECUTE TRADE WITH:                                                                      │
│  ├── Mode-specific size                                                                   │
│  ├── Mode-specific leverage                                                               │
│  ├── Mode-specific SL/TP                                                                  │
│  └── Mode-specific trailing (if enabled)                                                  │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| **AC-2.7.1** | **Each mode has independent circuit breaker settings** | Config shows 4 different circuit breakers |
| **AC-2.7.2** | **Circuit breaker triggers are mode-specific** | Ultra-fast triggers at 3 losses, Position at 10 |
| **AC-2.7.3** | **Min confidence varies by mode** | Ultra-fast: 50%, Position: 75% |
| **AC-2.7.4** | **Position size varies by mode** | Ultra-fast: $100, Position: $600 base |
| **AC-2.7.5** | **Timeframes are mode-specific** | Ultra-fast: 5m trend, Position: 4h trend |
| **AC-2.7.6** | **High confidence increases position size** | Size multiplier applied correctly |
| **AC-2.7.7** | **Circuit breaker pauses only affected mode** | Other modes continue trading |
| **AC-2.7.8** | **Win rate tracking per mode** | Separate win rate stats for each |
| **AC-2.7.9** | **Recovery actions differ by mode** | Ultra-fast auto-recovers, Position requires manual |
| **AC-2.7.10** | **SL/TP placed according to mode settings** | Binance orders match mode config |
| **AC-2.7.11** | **Max positions respected per mode** | Ultra-fast: 5, Position: 2 |
| **AC-2.7.12** | **Leverage applied per mode** | Ultra-fast: 10x, Position: 3x |
| **AC-2.7.13** | **All settings have Story 2.7 defaults** | Default values match documentation |
| **AC-2.7.14** | **User can customize any setting via UI/API** | Settings editable in Ginie panel |
| **AC-2.7.15** | **User settings are persisted to file** | `autopilot_settings.json` stores custom values |
| **AC-2.7.16** | **User settings override defaults on load** | After restart, custom settings applied |
| **AC-2.7.17** | **Reset to defaults option available** | User can restore Story 2.7 defaults |
| **AC-2.7.18** | **Hedge mode settings customizable per mode** | User can enable/disable hedge per mode |
| **AC-2.7.19** | **Averaging settings customizable per mode** | User can adjust averaging thresholds |
| **AC-2.7.20** | **Stale release settings customizable per mode** | User can adjust max hold times |

### Technical Tasks

| Task | Description | File | Priority |
|------|-------------|------|----------|
| **2.7.1** | **Create ModeConfig struct with all parameters** | ginie_types.go | **CRITICAL** |
| **2.7.2** | **Implement GetModeConfig(mode) function** | settings.go | **CRITICAL** |
| **2.7.3** | **Add mode-specific circuit breaker struct** | settings.go | **HIGH** |
| **2.7.4** | **Implement CheckModeCircuitBreaker(mode)** | ginie_autopilot.go | **HIGH** |
| **2.7.5** | **Add mode-specific confidence thresholds** | settings.go | **HIGH** |
| **2.7.6** | **Implement CalculateModePositionSize(mode, confidence)** | ginie_autopilot.go | **HIGH** |
| **2.7.7** | **Add mode-specific timeframe selection** | ginie_analyzer.go | **HIGH** |
| **2.7.8** | **Track win rate per mode separately** | ginie_autopilot.go | **HIGH** |
| **2.7.9** | **Add UI panel for per-mode configuration** | GiniePanel.tsx | **MEDIUM** |
| **2.7.10** | **Add API endpoints for mode config CRUD** | handlers_ginie.go | **MEDIUM** |
| **2.7.11** | **Implement mode-specific recovery logic** | ginie_autopilot.go | **MEDIUM** |
| **2.7.12** | **Add logging for circuit breaker triggers** | ginie_autopilot.go | **MEDIUM** |
| **2.7.13** | **Add unit tests for mode configurations** | New test file | **LOW** |
| **2.7.14** | **Integration test all 4 modes simultaneously** | New test file | **LOW** |
| **2.7.15** | **Load ModeConfigs from autopilot_settings.json** | settings.go | **HIGH** |
| **2.7.16** | **Save user-modified ModeConfigs to file** | settings.go | **HIGH** |
| **2.7.17** | **Add GET /api/futures/ginie/mode-config endpoint** | handlers_ginie.go | **HIGH** |
| **2.7.18** | **Add PUT /api/futures/ginie/mode-config/:mode endpoint** | handlers_ginie.go | **HIGH** |
| **2.7.19** | **Add POST /api/futures/ginie/mode-config/reset endpoint** | handlers_ginie.go | **MEDIUM** |
| **2.7.20** | **Add Mode Configuration panel to Ginie UI** | GiniePanel.tsx | **MEDIUM** |
| **2.7.21** | **Validate user inputs against min/max bounds** | handlers_ginie.go | **MEDIUM** |
| **2.7.22** | **Merge user settings over defaults on load** | settings.go | **HIGH** |

### New Settings Structure

```go
// In ginie_types.go - New struct for mode-specific configuration

// GinieModeConfig holds all settings specific to a trading mode
type GinieModeConfig struct {
    // Mode Identity
    Mode            GinieMode `json:"mode"`
    Enabled         bool      `json:"enabled"`

    // Timeframe Configuration
    TrendTimeframe  string    `json:"trend_timeframe"`   // "5m", "15m", "1h", "4h"
    EntryTimeframe  string    `json:"entry_timeframe"`   // Signal detection TF
    AnalysisTimeframe string  `json:"analysis_timeframe"` // Pattern detection TF

    // Confidence Thresholds
    MinConfidence   int       `json:"min_confidence"`    // Entry threshold (50-75)
    HighConfidence  int       `json:"high_confidence"`   // Size boost threshold
    UltraConfidence int       `json:"ultra_confidence"`  // Max size threshold

    // Position Sizing
    BaseSizeUSD     float64   `json:"base_size_usd"`     // Default position size
    MaxSizeUSD      float64   `json:"max_size_usd"`      // Cap after multiplier
    MaxPositions    int       `json:"max_positions"`     // Concurrent positions
    Leverage        int       `json:"leverage"`          // Mode leverage
    SizeMultiplier  float64   `json:"size_multiplier"`   // High conf multiplier

    // SL/TP Configuration
    StopLossPercent   float64 `json:"stop_loss_percent"`
    TakeProfitPercent float64 `json:"take_profit_percent"`
    TrailingEnabled   bool    `json:"trailing_enabled"`
    TrailingPercent   float64 `json:"trailing_percent"`
    MaxHoldDuration   string  `json:"max_hold_duration"` // "3s", "4h", "3d", "14d"

    // Circuit Breaker (Mode-Specific)
    CircuitBreaker  ModeCircuitBreaker `json:"circuit_breaker"`
}

// ModeCircuitBreaker holds risk controls for a specific mode
type ModeCircuitBreaker struct {
    // Loss Limits
    MaxLossPerHour    float64 `json:"max_loss_per_hour"`
    MaxLossPerDay     float64 `json:"max_loss_per_day"`
    MaxConsecutiveLoss int    `json:"max_consecutive_loss"`

    // Rate Limits
    MaxTradesPerMinute int    `json:"max_trades_per_minute"`
    MaxTradesPerHour   int    `json:"max_trades_per_hour"`
    MaxTradesPerDay    int    `json:"max_trades_per_day"`

    // Win Rate Monitoring
    WinRateCheckAfter  int    `json:"win_rate_check_after"`  // Min trades before check
    MinWinRatePercent  int    `json:"min_win_rate_percent"`

    // Cooldown & Recovery
    CooldownMinutes    int    `json:"cooldown_minutes"`
    AutoRecovery       bool   `json:"auto_recovery"`         // false = manual only

    // Current State (tracked)
    CurrentHourLoss    float64   `json:"current_hour_loss"`
    CurrentDayLoss     float64   `json:"current_day_loss"`
    ConsecutiveLosses  int       `json:"consecutive_losses"`
    TradesThisMinute   int       `json:"trades_this_minute"`
    TradesThisHour     int       `json:"trades_this_hour"`
    TradesThisDay      int       `json:"trades_this_day"`
    TotalWins          int       `json:"total_wins"`
    TotalTrades        int       `json:"total_trades"`
    IsPaused           bool      `json:"is_paused"`
    PausedUntil        time.Time `json:"paused_until"`
    PauseReason        string    `json:"pause_reason"`
}
```

### Implementation Functions

```go
// In settings.go or ginie_autopilot.go

// GetDefaultModeConfig returns the default configuration for a mode
func GetDefaultModeConfig(mode GinieMode) GinieModeConfig {
    configs := map[GinieMode]GinieModeConfig{
        GinieModeUltraFast: {
            Mode:              GinieModeUltraFast,
            Enabled:           true,
            TrendTimeframe:    "5m",
            EntryTimeframe:    "1m",
            AnalysisTimeframe: "1m",
            MinConfidence:     50,
            HighConfidence:    70,
            UltraConfidence:   85,
            BaseSizeUSD:       100,
            MaxSizeUSD:        200,
            MaxPositions:      5,
            Leverage:          10,
            SizeMultiplier:    1.5,
            StopLossPercent:   1.0,
            TakeProfitPercent: 2.0,
            TrailingEnabled:   false,
            TrailingPercent:   0,
            MaxHoldDuration:   "3s",
            CircuitBreaker: ModeCircuitBreaker{
                MaxLossPerHour:     20,
                MaxLossPerDay:      50,
                MaxConsecutiveLoss: 3,
                MaxTradesPerMinute: 5,
                MaxTradesPerHour:   30,
                MaxTradesPerDay:    100,
                WinRateCheckAfter:  10,
                MinWinRatePercent:  45,
                CooldownMinutes:    15,
                AutoRecovery:       true,
            },
        },
        GinieModeScalp: {
            Mode:              GinieModeScalp,
            Enabled:           true,
            TrendTimeframe:    "15m",
            EntryTimeframe:    "5m",
            AnalysisTimeframe: "15m",
            MinConfidence:     60,
            HighConfidence:    75,
            UltraConfidence:   88,
            BaseSizeUSD:       200,
            MaxSizeUSD:        400,
            MaxPositions:      4,
            Leverage:          8,
            SizeMultiplier:    1.8,
            StopLossPercent:   1.5,
            TakeProfitPercent: 3.0,
            TrailingEnabled:   false, // Optional
            TrailingPercent:   0.5,
            MaxHoldDuration:   "4h",
            CircuitBreaker: ModeCircuitBreaker{
                MaxLossPerHour:     40,
                MaxLossPerDay:      100,
                MaxConsecutiveLoss: 5,
                MaxTradesPerMinute: 3,
                MaxTradesPerHour:   20,
                MaxTradesPerDay:    50,
                WinRateCheckAfter:  15,
                MinWinRatePercent:  50,
                CooldownMinutes:    30,
                AutoRecovery:       true,
            },
        },
        GinieModeSwing: {
            Mode:              GinieModeSwing,
            Enabled:           true,
            TrendTimeframe:    "1h",
            EntryTimeframe:    "15m",
            AnalysisTimeframe: "4h",
            MinConfidence:     65,
            HighConfidence:    80,
            UltraConfidence:   90,
            BaseSizeUSD:       400,
            MaxSizeUSD:        750,
            MaxPositions:      3,
            Leverage:          5,
            SizeMultiplier:    2.0,
            StopLossPercent:   2.5,
            TakeProfitPercent: 5.0,
            TrailingEnabled:   true,
            TrailingPercent:   1.5,
            MaxHoldDuration:   "72h", // 3 days
            CircuitBreaker: ModeCircuitBreaker{
                MaxLossPerHour:     80,
                MaxLossPerDay:      200,
                MaxConsecutiveLoss: 7,
                MaxTradesPerMinute: 2,
                MaxTradesPerHour:   10,
                MaxTradesPerDay:    20,
                WinRateCheckAfter:  20,
                MinWinRatePercent:  55,
                CooldownMinutes:    60,
                AutoRecovery:       true,
            },
        },
        GinieModePosition: {
            Mode:              GinieModePosition,
            Enabled:           true,
            TrendTimeframe:    "4h",
            EntryTimeframe:    "1h",
            AnalysisTimeframe: "1d",
            MinConfidence:     75,
            HighConfidence:    85,
            UltraConfidence:   92,
            BaseSizeUSD:       600,
            MaxSizeUSD:        1000,
            MaxPositions:      2,
            Leverage:          3,
            SizeMultiplier:    2.5,
            StopLossPercent:   3.5,
            TakeProfitPercent: 8.0,
            TrailingEnabled:   true,
            TrailingPercent:   2.5,
            MaxHoldDuration:   "336h", // 14 days
            CircuitBreaker: ModeCircuitBreaker{
                MaxLossPerHour:     150,
                MaxLossPerDay:      400,
                MaxConsecutiveLoss: 10,
                MaxTradesPerMinute: 1,
                MaxTradesPerHour:   5,
                MaxTradesPerDay:    10,
                WinRateCheckAfter:  25,
                MinWinRatePercent:  60,
                CooldownMinutes:    120,
                AutoRecovery:       false, // Manual only for position mode
            },
        },
    }
    return configs[mode]
}

// CheckModeCircuitBreaker validates if trading is allowed for a mode
func (ga *GinieAutopilot) CheckModeCircuitBreaker(mode GinieMode) (allowed bool, reason string) {
    config := ga.GetModeConfig(mode)
    cb := &config.CircuitBreaker

    // Check if paused
    if cb.IsPaused {
        if time.Now().Before(cb.PausedUntil) {
            return false, fmt.Sprintf("Mode %s paused until %s: %s", mode, cb.PausedUntil.Format(time.RFC3339), cb.PauseReason)
        }
        // Auto-recovery if enabled
        if cb.AutoRecovery {
            cb.IsPaused = false
            cb.PauseReason = ""
            ga.logger.Info("Mode auto-recovered", "mode", mode)
        } else {
            return false, fmt.Sprintf("Mode %s requires manual recovery: %s", mode, cb.PauseReason)
        }
    }

    // Check consecutive losses
    if cb.ConsecutiveLosses >= cb.MaxConsecutiveLoss {
        ga.TriggerModeCircuitBreaker(mode, fmt.Sprintf("%d consecutive losses", cb.ConsecutiveLosses))
        return false, fmt.Sprintf("Max consecutive losses reached: %d", cb.ConsecutiveLosses)
    }

    // Check hourly loss
    if cb.CurrentHourLoss >= cb.MaxLossPerHour {
        ga.TriggerModeCircuitBreaker(mode, fmt.Sprintf("Hourly loss $%.2f exceeded limit $%.2f", cb.CurrentHourLoss, cb.MaxLossPerHour))
        return false, fmt.Sprintf("Hourly loss limit reached: $%.2f", cb.MaxLossPerHour)
    }

    // Check daily loss
    if cb.CurrentDayLoss >= cb.MaxLossPerDay {
        ga.TriggerModeCircuitBreaker(mode, fmt.Sprintf("Daily loss $%.2f exceeded limit $%.2f", cb.CurrentDayLoss, cb.MaxLossPerDay))
        return false, fmt.Sprintf("Daily loss limit reached: $%.2f", cb.MaxLossPerDay)
    }

    // Check trade rate limits
    if cb.TradesThisMinute >= cb.MaxTradesPerMinute {
        return false, fmt.Sprintf("Rate limit: %d trades/min", cb.MaxTradesPerMinute)
    }
    if cb.TradesThisHour >= cb.MaxTradesPerHour {
        return false, fmt.Sprintf("Hourly trade limit: %d trades", cb.MaxTradesPerHour)
    }
    if cb.TradesThisDay >= cb.MaxTradesPerDay {
        return false, fmt.Sprintf("Daily trade limit: %d trades", cb.MaxTradesPerDay)
    }

    // Check win rate (if enough trades)
    if cb.TotalTrades >= cb.WinRateCheckAfter {
        winRate := float64(cb.TotalWins) / float64(cb.TotalTrades) * 100
        if winRate < float64(cb.MinWinRatePercent) {
            ga.TriggerModeCircuitBreaker(mode, fmt.Sprintf("Win rate %.1f%% below minimum %d%%", winRate, cb.MinWinRatePercent))
            return false, fmt.Sprintf("Win rate too low: %.1f%% < %d%%", winRate, cb.MinWinRatePercent)
        }
    }

    return true, "OK"
}

// CalculateModePositionSize determines position size based on mode and confidence
func (ga *GinieAutopilot) CalculateModePositionSize(mode GinieMode, confidence int) float64 {
    config := ga.GetModeConfig(mode)

    size := config.BaseSizeUSD

    // Apply confidence-based multiplier
    if confidence >= config.UltraConfidence {
        size = config.BaseSizeUSD * config.SizeMultiplier
    } else if confidence >= config.HighConfidence {
        size = config.BaseSizeUSD * 1.5
    }

    // Cap at max size
    if size > config.MaxSizeUSD {
        size = config.MaxSizeUSD
    }

    ga.logger.Info("Calculated position size",
        "mode", mode,
        "confidence", confidence,
        "base_size", config.BaseSizeUSD,
        "calculated_size", size,
        "max_size", config.MaxSizeUSD)

    return size
}

// ExecuteTradeWithModeConfig executes a trade with all mode-specific settings
func (ga *GinieAutopilot) ExecuteTradeWithModeConfig(signal *GinieSignal) error {
    mode := signal.Mode
    config := ga.GetModeConfig(mode)

    // 1. Check circuit breaker
    allowed, reason := ga.CheckModeCircuitBreaker(mode)
    if !allowed {
        ga.logger.Warn("Trade blocked by circuit breaker", "mode", mode, "reason", reason)
        return fmt.Errorf("circuit breaker: %s", reason)
    }

    // 2. Check confidence threshold
    if signal.Confidence < config.MinConfidence {
        ga.logger.Info("Signal below confidence threshold",
            "mode", mode,
            "signal_confidence", signal.Confidence,
            "min_required", config.MinConfidence)
        return fmt.Errorf("confidence %d below threshold %d", signal.Confidence, config.MinConfidence)
    }

    // 3. Calculate position size
    positionSize := ga.CalculateModePositionSize(mode, signal.Confidence)

    // 4. Apply mode leverage
    leverage := config.Leverage

    // 5. Calculate SL/TP prices
    slPercent := config.StopLossPercent
    tpPercent := config.TakeProfitPercent

    // Apply LLM adjustment if enabled
    if ga.settingsManager.GetCurrentSettings().LLMAdaptiveSLTPEnabled {
        llmSuggestion, _ := ga.analyzer.GetLLMAdaptiveSLTP(signal.Symbol, mode, signal.EntryPrice, signal.IsLong, signal.MarketData)
        if llmSuggestion != nil {
            weight := ga.settingsManager.GetCurrentSettings().LLMAdaptiveWeight
            slPercent, tpPercent = ga.analyzer.BlendATRWithLLM(slPercent, tpPercent, llmSuggestion, weight)
        }
    }

    // Calculate actual prices
    slPrice, tpPrice := ga.CalculateSLTPPrices(signal.EntryPrice, slPercent, tpPercent, signal.IsLong)

    // 6. Execute the trade
    ga.logger.Info("Executing mode-specific trade",
        "mode", mode,
        "symbol", signal.Symbol,
        "direction", signal.Direction,
        "confidence", signal.Confidence,
        "size_usd", positionSize,
        "leverage", leverage,
        "sl_price", slPrice,
        "tp_price", tpPrice,
        "trailing", config.TrailingEnabled)

    // Place order with mode config
    order, err := ga.PlaceEntryOrder(signal.Symbol, signal.Direction, positionSize, leverage)
    if err != nil {
        return err
    }

    // Place SL/TP orders
    err = ga.PlaceSLTPOrders(signal.Symbol, order.OrderID, slPrice, tpPrice, config.TrailingEnabled, config.TrailingPercent)
    if err != nil {
        return err
    }

    // 7. Update circuit breaker tracking
    ga.UpdateModeTradeCount(mode)

    return nil
}
```

### UI Design for Mode Configuration

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│ ⚙️ Mode-Specific Configuration                                                           │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                          │
│  Select Mode: [Ultra-Fast ▼] [Scalp] [Swing] [Position]                                 │
│                                                                                          │
│  ═══════════════════════════════════════════════════════════════════════════════════    │
│                                                                                          │
│  📊 TIMEFRAME SETTINGS                                                                   │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ Trend Timeframe:    [5m  ▼]     ← Higher TF for trend direction                    │ │
│  │ Entry Timeframe:    [1m  ▼]     ← Signal detection timeframe                       │ │
│  │ Analysis Timeframe: [1m  ▼]     ← Pattern recognition timeframe                    │ │
│  └────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                          │
│  🎯 CONFIDENCE THRESHOLDS                                                                │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ Minimum Confidence: [50 ]%  ← Trades rejected below this                           │ │
│  │ High Confidence:    [70 ]%  ← Size × 1.5 above this                                │ │
│  │ Ultra Confidence:   [85 ]%  ← Max size multiplier above this                       │ │
│  └────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                          │
│  💰 POSITION SIZING                                                                      │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ Base Size:      $[100    ]    Max Size:     $[200    ]                             │ │
│  │ Max Positions:   [5      ]    Leverage:      [10    ]x                             │ │
│  │ Size Multiplier: [1.5    ]x   (for ultra confidence)                               │ │
│  └────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                          │
│  🛑 CIRCUIT BREAKER                                                                      │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ ┌─────────────────────────────┬─────────────────────────────────────────────────┐  │ │
│  │ │ Loss Limits                 │ Rate Limits                                     │  │ │
│  │ ├─────────────────────────────┼─────────────────────────────────────────────────┤  │ │
│  │ │ Max Loss/Hour:    $[20   ] │ Max Trades/Min:  [5  ]                          │  │ │
│  │ │ Max Loss/Day:     $[50   ] │ Max Trades/Hour: [30 ]                          │  │ │
│  │ │ Max Consec. Loss:  [3    ] │ Max Trades/Day:  [100]                          │  │ │
│  │ └─────────────────────────────┴─────────────────────────────────────────────────┘  │ │
│  │                                                                                     │ │
│  │ ┌─────────────────────────────┬─────────────────────────────────────────────────┐  │ │
│  │ │ Win Rate Monitoring         │ Recovery Settings                               │  │ │
│  │ ├─────────────────────────────┼─────────────────────────────────────────────────┤  │ │
│  │ │ Check After:      [10   ]  │ Cooldown:        [15  ] minutes                 │  │ │
│  │ │ Min Win Rate:     [45   ]% │ ☑ Auto Recovery                                 │  │ │
│  │ └─────────────────────────────┴─────────────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                          │
│  📈 SL/TP SETTINGS                                                                       │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐ │
│  │ Stop Loss:     [1.0 ]%        Take Profit: [2.0 ]%                                │ │
│  │ ☐ Trailing Stop Enabled       Trail %:     [N/A ]                                 │ │
│  │ Max Hold Time: [3 seconds  ▼]                                                     │ │
│  └────────────────────────────────────────────────────────────────────────────────────┘ │
│                                                                                          │
│                             [Reset to Defaults]  [Cancel]  [Save Configuration]         │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Settings Persistence & User Customization

**Core Principle:** All mode configuration values in Story 2.7 are **defaults**. Users can customize any setting via UI or API, and their customizations are **persisted and prioritized** over defaults.

```
┌──────────────────────────────────────────────────────────────────────────────────────────┐
│                    SETTINGS FLOW: DEFAULTS → USER CUSTOMIZATION                           │
├──────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                           │
│  1. INITIAL STATE (First Run)                                                             │
│  ────────────────────────────                                                             │
│  • Story 2.7 defaults are written to autopilot_settings.json                             │
│  • All 4 modes use documented default values                                             │
│  • ModeConfigs map contains: ultra_fast, scalp, swing, position                          │
│                                                                                           │
│  2. USER CUSTOMIZATION FLOW                                                               │
│  ──────────────────────────                                                               │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ User Action           │ Result                                                      │  │
│  ├───────────────────────┼─────────────────────────────────────────────────────────────┤  │
│  │ View Mode Config      │ GET /api/futures/ginie/mode-config returns current values   │  │
│  │ Edit Mode Config      │ UI fields populated with current (default or custom) values │  │
│  │ Save Mode Config      │ PUT /api/futures/ginie/mode-config/:mode saves to JSON file │  │
│  │ Reset to Defaults     │ POST /api/futures/ginie/mode-config/reset restores Story 2.7│  │
│  └────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
│  3. LOAD PRIORITY (On Server Start)                                                       │
│  ────────────────────────────────                                                         │
│  Priority: User Settings > Defaults                                                       │
│                                                                                           │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ func LoadSettings() *AutopilotSettings {                                            │  │
│  │     // 1. Start with Story 2.7 defaults                                             │  │
│  │     settings := DefaultSettings()                                                   │  │
│  │                                                                                      │  │
│  │     // 2. Read user's custom settings from file                                      │  │
│  │     userSettings := readFromJSON("autopilot_settings.json")                         │  │
│  │                                                                                      │  │
│  │     // 3. Merge: user values override defaults                                       │  │
│  │     mergeSettings(settings, userSettings)                                            │  │
│  │                                                                                      │  │
│  │     return settings  // User's customizations + defaults for untouched fields       │  │
│  │ }                                                                                    │  │
│  └────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
│  4. SETTINGS FILE STRUCTURE (autopilot_settings.json)                                    │
│  ───────────────────────────────────────────────────                                     │
│  {                                                                                        │
│    "mode_configs": {                                                                     │
│      "ultra_fast": {                                                                     │
│        "enabled": true,                                                                  │
│        "timeframe": { "trend_timeframe": "5m", "entry_timeframe": "1m", ... },          │
│        "confidence": { "min_confidence": 50, "high_confidence": 70, ... },              │
│        "size": { "base_size_usd": 100, "max_size_usd": 200, "leverage": 10, ... },      │
│        "circuit_breaker": { "max_loss_per_hour": 20, "max_consecutive_losses": 3, ... },│
│        "sltp": { "stop_loss_percent": 1.0, "take_profit_percent": 2.0, ... },           │
│        "hedge": { "allow_hedge": true, "min_confidence_for_hedge": 70, ... },            │
│        "averaging": { "allow_averaging": false, "max_averages": 0, ... },               │
│        "stale_release": { "enabled": true, "max_hold_duration": "10s", ... },           │
│        "assignment": { "volatility_min": "high", "priority_weight": 0.8, ... }          │
│      },                                                                                  │
│      "scalp": { ... },                                                                   │
│      "swing": { ... },                                                                   │
│      "position": { ... }                                                                 │
│    }                                                                                     │
│  }                                                                                        │
│                                                                                           │
│  5. API ENDPOINTS                                                                         │
│  ─────────────────                                                                        │
│  ┌────────────────────────────────────────────────────────────────────────────────────┐  │
│  │ Endpoint                                   │ Method │ Description                   │  │
│  ├────────────────────────────────────────────┼────────┼──────────────────────────────┤  │
│  │ /api/futures/ginie/mode-config             │ GET    │ Get all 4 mode configurations │  │
│  │ /api/futures/ginie/mode-config/:mode       │ GET    │ Get single mode configuration │  │
│  │ /api/futures/ginie/mode-config/:mode       │ PUT    │ Update single mode config     │  │
│  │ /api/futures/ginie/mode-config/reset       │ POST   │ Reset all to Story 2.7 defaults│  │
│  │ /api/futures/ginie/mode-config/:mode/reset │ POST   │ Reset single mode to defaults │  │
│  └────────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                           │
│  6. VALIDATION RULES                                                                      │
│  ───────────────────                                                                      │
│  • min_confidence: 30-100                                                                 │
│  • leverage: 1-20                                                                         │
│  • base_size_usd: 10-10000                                                               │
│  • stop_loss_percent: 0.1-10.0                                                           │
│  • take_profit_percent: 0.1-20.0                                                         │
│  • max_consecutive_losses: 1-50                                                          │
│  • cooldown_minutes: 1-1440 (24 hours)                                                   │
│  • Timeframes must be valid: "1m", "5m", "15m", "30m", "1h", "4h", "1d"                  │
│                                                                                           │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

### Definition of Done

- [ ] **ModeConfig struct** with all parameters implemented
- [ ] **Per-mode circuit breakers** with independent tracking
- [ ] **Confidence-based trade gating** per mode
- [ ] **Position sizing varies by confidence** level
- [ ] **Mode timeframes configurable** and applied
- [ ] **SL/TP placed per mode** settings
- [ ] **Circuit breaker pauses single mode** only
- [ ] **Win rate tracked per mode** independently
- [ ] **Auto/manual recovery** based on mode
- [ ] **UI panel for mode configuration** complete
- [ ] **API endpoints for mode CRUD** working
- [ ] **All 4 modes tested simultaneously** without conflict
- [ ] **Story 2.7 defaults** written to settings on first run
- [ ] **User customizations persisted** to autopilot_settings.json
- [ ] **User settings loaded on restart** and override defaults
- [ ] **Reset to defaults** restores Story 2.7 values
- [ ] **Validation rules enforced** on all user inputs
- [ ] **Hedge mode settings customizable** per mode
- [ ] **Averaging settings customizable** per mode
- [ ] **Stale release settings customizable** per mode

---

## Story 2.8: LLM & Adaptive AI Decision Engine

### User Story

**As a** Ginie autopilot user,
**I want** an intelligent AI system that analyzes market conditions and adapts to changing patterns,
**So that** my trading decisions are based on advanced pattern recognition, sentiment analysis, and continuous learning from outcomes.

---

### Overview

The LLM & Adaptive AI Decision Engine integrates Large Language Models (LLM) with traditional technical analysis to create a hybrid decision-making system that:

1. **Understands Market Context** - LLM analyzes news, sentiment, and market narratives
2. **Enhances Signal Confidence** - AI validates technical signals with broader context
3. **Adapts Over Time** - Learns from trade outcomes to improve future decisions
4. **Explains Decisions** - Provides human-readable reasoning for every trade

---

### Architecture: AI Decision Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         GINIE AI DECISION PIPELINE                               │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │   SCANNER    │───▶│   ANALYZER   │───▶│  LLM ENGINE  │───▶│   DECISION   │   │
│  │  (Technical) │    │  (Signals)   │    │  (Context)   │    │   FUSION     │   │
│  └──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘   │
│         │                   │                   │                   │            │
│         ▼                   ▼                   ▼                   ▼            │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Price Data   │    │ RSI, MACD    │    │ Market News  │    │ Final Score  │   │
│  │ Volume Data  │    │ EMA, BB      │    │ Sentiment    │    │ Confidence   │   │
│  │ Order Book   │    │ ADX, ATR     │    │ Trend Reason │    │ Trade/Skip   │   │
│  └──────────────┘    └──────────────┘    └──────────────┘    └──────────────┘   │
│                                                                                  │
│                              ┌──────────────────┐                                │
│                              │  ADAPTIVE LAYER  │                                │
│                              │  (Learn & Tune)  │                                │
│                              └──────────────────┘                                │
│                                       │                                          │
│         ┌─────────────────────────────┴─────────────────────────────┐            │
│         ▼                             ▼                             ▼            │
│  ┌──────────────┐           ┌──────────────┐           ┌──────────────┐          │
│  │ Trade History│           │  Win/Loss    │           │  Parameter   │          │
│  │   Analysis   │           │  Patterns    │           │   Tuning     │          │
│  └──────────────┘           └──────────────┘           └──────────────┘          │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

### Component 1: LLM Integration

#### 1.1 LLM Provider Configuration

| Provider | Model | Use Case | Cost | Speed |
|----------|-------|----------|------|-------|
| **DeepSeek** | `deepseek-chat` | Primary analysis | Low | Fast |
| **Claude** | `claude-3-haiku` | Fallback / complex | Medium | Medium |
| **OpenAI** | `gpt-4o-mini` | Secondary fallback | Medium | Fast |
| **Local** | `llama3.2` | Offline / privacy | Free | Variable |

#### 1.2 LLM Analysis Prompt Template

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         LLM ANALYSIS PROMPT STRUCTURE                            │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  SYSTEM PROMPT:                                                                  │
│  ─────────────                                                                   │
│  You are a professional crypto trading analyst. Analyze the following data      │
│  and provide a trading recommendation with confidence score (0-100).            │
│                                                                                  │
│  CONTEXT DATA INJECTION:                                                         │
│  ───────────────────────                                                         │
│  {                                                                               │
│    "symbol": "BTCUSDT",                                                          │
│    "current_price": 98500.50,                                                    │
│    "price_change_1h": -0.5%,                                                     │
│    "price_change_24h": +2.3%,                                                    │
│    "volume_24h": 45000000000,                                                    │
│    "volume_change": +15%,                                                        │
│    "technical_signals": {                                                        │
│      "rsi_14": 62,                                                               │
│      "macd_signal": "bullish_crossover",                                         │
│      "ema_trend": "above_50_100",                                                │
│      "bb_position": "upper_half",                                                │
│      "adx_strength": 28                                                          │
│    },                                                                            │
│    "market_context": {                                                           │
│      "btc_dominance": 52.3,                                                      │
│      "total_market_cap_change": +1.2%,                                           │
│      "fear_greed_index": 65                                                      │
│    },                                                                            │
│    "recent_news": [                                                              │
│      "Bitcoin ETF sees record inflows",                                          │
│      "Fed signals rate pause",                                                   │
│      "Whale accumulation detected"                                               │
│    ]                                                                             │
│  }                                                                               │
│                                                                                  │
│  REQUIRED OUTPUT FORMAT:                                                         │
│  ───────────────────────                                                         │
│  {                                                                               │
│    "recommendation": "LONG" | "SHORT" | "HOLD",                                  │
│    "confidence": 0-100,                                                          │
│    "reasoning": "Brief explanation",                                             │
│    "key_factors": ["factor1", "factor2", "factor3"],                             │
│    "risk_level": "low" | "moderate" | "high",                                    │
│    "suggested_sl_percent": 1.0-5.0,                                              │
│    "suggested_tp_percent": 2.0-10.0,                                             │
│    "time_horizon": "ultra_fast" | "scalp" | "swing" | "position"                 │
│  }                                                                               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 1.3 LLM Response Processing

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         LLM RESPONSE PROCESSING FLOW                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌───────────────┐     ┌───────────────┐     ┌───────────────┐                  │
│  │ Raw LLM       │────▶│ JSON Parser   │────▶│ Validation    │                  │
│  │ Response      │     │ + Cleanup     │     │ Layer         │                  │
│  └───────────────┘     └───────────────┘     └───────────────┘                  │
│                                                    │                             │
│                              ┌─────────────────────┴─────────────────────┐       │
│                              ▼                                           ▼       │
│                    ┌───────────────┐                          ┌───────────────┐  │
│                    │   VALID       │                          │   INVALID     │  │
│                    │   Response    │                          │   Response    │  │
│                    └───────────────┘                          └───────────────┘  │
│                              │                                           │       │
│                              ▼                                           ▼       │
│                    ┌───────────────┐                          ┌───────────────┐  │
│                    │ Merge with    │                          │ Use Technical │  │
│                    │ Technical     │                          │ Signal Only   │  │
│                    │ Signals       │                          │ (Fallback)    │  │
│                    └───────────────┘                          └───────────────┘  │
│                                                                                  │
│  VALIDATION RULES:                                                               │
│  ─────────────────                                                               │
│  • recommendation must be LONG, SHORT, or HOLD                                   │
│  • confidence must be 0-100 integer                                              │
│  • reasoning must be non-empty string                                            │
│  • time_horizon must match valid mode                                            │
│  • sl/tp percentages must be within bounds                                       │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

### Component 2: Decision Fusion (Blending Technical + LLM)

#### 2.1 Confidence Fusion Formula

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         CONFIDENCE FUSION ALGORITHM                              │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  INPUTS:                                                                         │
│  ───────                                                                         │
│  • technical_confidence: 0-100 (from Scanner + Analyzer)                         │
│  • llm_confidence: 0-100 (from LLM response)                                     │
│  • llm_weight: 0.0-1.0 (configurable per mode, default 0.3)                      │
│  • agreement_bonus: +10 if both agree on direction                               │
│  • disagreement_penalty: -15 if directions conflict                              │
│                                                                                  │
│  FORMULA:                                                                        │
│  ────────                                                                        │
│                                                                                  │
│  base_fusion = (technical × (1 - llm_weight)) + (llm × llm_weight)               │
│                                                                                  │
│  IF technical_direction == llm_direction:                                        │
│      final_confidence = base_fusion + agreement_bonus                            │
│  ELSE IF technical_direction != llm_direction:                                   │
│      final_confidence = base_fusion + disagreement_penalty                       │
│      // Log conflict for adaptive learning                                       │
│  ELSE (one is HOLD):                                                             │
│      final_confidence = base_fusion                                              │
│                                                                                  │
│  final_confidence = clamp(final_confidence, 0, 100)                              │
│                                                                                  │
│  EXAMPLE CALCULATION:                                                            │
│  ────────────────────                                                            │
│  technical_confidence = 75 (LONG)                                                │
│  llm_confidence = 80 (LONG)                                                      │
│  llm_weight = 0.3                                                                │
│                                                                                  │
│  base_fusion = (75 × 0.7) + (80 × 0.3) = 52.5 + 24 = 76.5                        │
│  agreement_bonus = +10 (both say LONG)                                           │
│  final_confidence = 76.5 + 10 = 86.5 → 87%                                       │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 2.2 Mode-Specific LLM Weight Defaults

| Mode | LLM Weight | Reasoning |
|------|------------|-----------|
| **Ultra-Fast** | 0.1 (10%) | Speed critical, rely more on technical |
| **Scalp** | 0.2 (20%) | Quick decisions, moderate LLM input |
| **Swing** | 0.4 (40%) | More time for analysis, higher LLM weight |
| **Position** | 0.5 (50%) | Long-term, narrative matters most |

---

### Component 3: Adaptive AI (Learning from Outcomes)

#### 3.1 Trade Outcome Tracking

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         TRADE OUTCOME TRACKING SCHEMA                            │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  {                                                                               │
│    "trade_id": "ginie_btcusdt_1703520000",                                       │
│    "symbol": "BTCUSDT",                                                          │
│    "mode": "swing",                                                              │
│    "entry_time": "2025-12-26T10:00:00Z",                                         │
│    "exit_time": "2025-12-26T14:30:00Z",                                          │
│    "direction": "LONG",                                                          │
│    "entry_price": 98500.00,                                                      │
│    "exit_price": 99200.00,                                                       │
│    "pnl_percent": +0.71,                                                         │
│    "pnl_usd": +3.55,                                                             │
│    "outcome": "WIN",  // WIN, LOSS, BREAKEVEN                                    │
│                                                                                  │
│    "decision_context": {                                                         │
│      "technical_confidence": 72,                                                 │
│      "llm_confidence": 85,                                                       │
│      "final_confidence": 81,                                                     │
│      "technical_direction": "LONG",                                              │
│      "llm_direction": "LONG",                                                    │
│      "agreement": true,                                                          │
│      "llm_reasoning": "Bullish momentum with ETF inflows",                       │
│      "llm_key_factors": ["etf_inflows", "rsi_oversold", "volume_spike"]          │
│    },                                                                            │
│                                                                                  │
│    "market_snapshot": {                                                          │
│      "rsi_at_entry": 58,                                                         │
│      "macd_at_entry": "bullish",                                                 │
│      "volume_ratio_at_entry": 1.3,                                               │
│      "btc_dominance_at_entry": 52.1,                                             │
│      "fear_greed_at_entry": 62                                                   │
│    }                                                                             │
│  }                                                                               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 3.2 Adaptive Learning Algorithm

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         ADAPTIVE LEARNING PROCESS                                │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  TRIGGER: Every 50 trades OR every 24 hours                                      │
│  ─────────────────────────────────────────────                                   │
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │ STEP 1: Aggregate Trade Outcomes                                         │   │
│  ├──────────────────────────────────────────────────────────────────────────┤   │
│  │                                                                           │   │
│  │  Per Mode Statistics (last 50 trades):                                    │   │
│  │  ┌────────────────────────────────────────────────────────────────────┐   │   │
│  │  │ Mode        │ Wins │ Losses │ Win% │ Avg Win │ Avg Loss │ Profit │   │   │
│  │  ├────────────────────────────────────────────────────────────────────┤   │   │
│  │  │ Ultra-Fast  │  12  │   8    │ 60%  │  +0.8%  │  -0.5%   │ +5.6%  │   │   │
│  │  │ Scalp       │   8  │   7    │ 53%  │  +1.2%  │  -0.8%   │ +4.0%  │   │   │
│  │  │ Swing       │   6  │   4    │ 60%  │  +2.5%  │  -1.5%   │ +9.0%  │   │   │
│  │  │ Position    │   3  │   2    │ 60%  │  +4.0%  │  -2.0%   │ +8.0%  │   │   │
│  │  └────────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                           │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │ STEP 2: Analyze Signal Accuracy                                          │   │
│  ├──────────────────────────────────────────────────────────────────────────┤   │
│  │                                                                           │   │
│  │  Technical vs LLM Agreement Analysis:                                     │   │
│  │  ┌────────────────────────────────────────────────────────────────────┐   │   │
│  │  │ Scenario              │ Trades │ Win% │ Profit │ Insight          │   │   │
│  │  ├────────────────────────────────────────────────────────────────────┤   │   │
│  │  │ Both Agree (LONG)     │   18   │ 72%  │ +15.2% │ High confidence  │   │   │
│  │  │ Both Agree (SHORT)    │   12   │ 67%  │ +8.1%  │ High confidence  │   │   │
│  │  │ Technical wins, LLM   │    8   │ 50%  │ +2.0%  │ Trust technical  │   │   │
│  │  │ LLM wins, Technical   │    6   │ 33%  │ -3.5%  │ Lower LLM weight │   │   │
│  │  │ Disagreement executed │    6   │ 33%  │ -4.8%  │ Skip conflicts   │   │   │
│  │  └────────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                           │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │ STEP 3: Generate Adjustment Recommendations                              │   │
│  ├──────────────────────────────────────────────────────────────────────────┤   │
│  │                                                                           │   │
│  │  Based on analysis, AI suggests:                                          │   │
│  │                                                                           │   │
│  │  ┌────────────────────────────────────────────────────────────────────┐   │   │
│  │  │ RECOMMENDATION 1: Reduce LLM weight for Scalp mode                 │   │   │
│  │  │   Current: 0.20 → Suggested: 0.15                                  │   │   │
│  │  │   Reason: LLM disagreements losing more often in scalp timeframe   │   │   │
│  │  ├────────────────────────────────────────────────────────────────────┤   │   │
│  │  │ RECOMMENDATION 2: Increase min confidence for Ultra-Fast           │   │   │
│  │  │   Current: 50 → Suggested: 60                                      │   │   │
│  │  │   Reason: Lower confidence trades have 45% win rate vs 68% overall │   │   │
│  │  ├────────────────────────────────────────────────────────────────────┤   │   │
│  │  │ RECOMMENDATION 3: Enable disagreement blocking for Position mode   │   │   │
│  │  │   Current: false → Suggested: true                                 │   │   │
│  │  │   Reason: 0% win rate when technical and LLM disagree              │   │   │
│  │  └────────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                           │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
│  ┌──────────────────────────────────────────────────────────────────────────┐   │
│  │ STEP 4: Apply Adjustments (User Approval Required)                       │   │
│  ├──────────────────────────────────────────────────────────────────────────┤   │
│  │                                                                           │   │
│  │  IF auto_adapt_enabled AND adjustment < max_auto_adjustment:             │   │
│  │      Apply automatically with notification                                │   │
│  │  ELSE:                                                                    │   │
│  │      Show recommendation in UI, wait for user approval                    │   │
│  │                                                                           │   │
│  │  ┌───────────────────────────────────────────────────────────┐           │   │
│  │  │  🤖 Adaptive AI Recommendation                           │           │   │
│  │  │                                                           │           │   │
│  │  │  Based on 50 recent trades, I recommend:                  │           │   │
│  │  │                                                           │           │   │
│  │  │  • Reduce Ultra-Fast LLM weight: 0.10 → 0.05              │           │   │
│  │  │  • Increase Swing min confidence: 55 → 65                 │           │   │
│  │  │                                                           │           │   │
│  │  │  Expected improvement: +3.2% win rate                     │           │   │
│  │  │                                                           │           │   │
│  │  │  [Apply All] [Review Each] [Dismiss]                      │           │   │
│  │  └───────────────────────────────────────────────────────────┘           │   │
│  │                                                                           │   │
│  └──────────────────────────────────────────────────────────────────────────┘   │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

### Component 4: LLM Configuration Settings (Per Mode)

#### 4.1 Configuration Structure

```json
{
  "llm_config": {
    "enabled": true,
    "provider": "deepseek",
    "model": "deepseek-chat",
    "fallback_provider": "claude",
    "fallback_model": "claude-3-haiku",
    "timeout_ms": 5000,
    "retry_count": 2,
    "cache_duration_sec": 300
  },

  "mode_llm_settings": {
    "ultra_fast": {
      "llm_enabled": true,
      "llm_weight": 0.1,
      "skip_on_timeout": true,
      "min_llm_confidence": 40,
      "block_on_disagreement": false,
      "cache_enabled": true
    },
    "scalp": {
      "llm_enabled": true,
      "llm_weight": 0.2,
      "skip_on_timeout": true,
      "min_llm_confidence": 50,
      "block_on_disagreement": false,
      "cache_enabled": true
    },
    "swing": {
      "llm_enabled": true,
      "llm_weight": 0.4,
      "skip_on_timeout": false,
      "min_llm_confidence": 60,
      "block_on_disagreement": true,
      "cache_enabled": false
    },
    "position": {
      "llm_enabled": true,
      "llm_weight": 0.5,
      "skip_on_timeout": false,
      "min_llm_confidence": 65,
      "block_on_disagreement": true,
      "cache_enabled": false
    }
  },

  "adaptive_ai_config": {
    "enabled": true,
    "learning_window_trades": 50,
    "learning_window_hours": 24,
    "auto_adjust_enabled": false,
    "max_auto_adjustment_percent": 10,
    "require_approval": true,
    "min_trades_for_learning": 20,
    "store_decision_context": true
  }
}
```

#### 4.2 Mode-Specific LLM Defaults

| Setting | Ultra-Fast | Scalp | Swing | Position |
|---------|------------|-------|-------|----------|
| **llm_enabled** | true | true | true | true |
| **llm_weight** | 0.10 | 0.20 | 0.40 | 0.50 |
| **skip_on_timeout** | true | true | false | false |
| **min_llm_confidence** | 40 | 50 | 60 | 65 |
| **block_on_disagreement** | false | false | true | true |
| **cache_enabled** | true | true | false | false |

---

### Component 6: News & Sentiment Data Sources

#### 6.1 Data Source Architecture

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    NEWS & SENTIMENT DATA AGGREGATION PIPELINE                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │                         PRIMARY DATA SOURCES                                ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         ││
│  │  │  CryptoNews │  │  CoinGecko  │  │  LunarCrush │  │   Santiment │         ││
│  │  │     API     │  │  Sentiment  │  │   Social    │  │  On-Chain   │         ││
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         ││
│  │         │                │                │                │                 ││
│  │         ▼                ▼                ▼                ▼                 ││
│  │  ┌───────────────────────────────────────────────────────────────────────┐  ││
│  │  │                    DATA NORMALIZER & AGGREGATOR                       │  ││
│  │  │  • Standardize sentiment scores to -100 to +100 scale                 │  ││
│  │  │  • Merge duplicate news from multiple sources                         │  ││
│  │  │  • Weight by source reliability                                       │  ││
│  │  │  • Cache with TTL per source                                          │  ││
│  │  └───────────────────────────────────────────────────────────────────────┘  ││
│  │                                      │                                       ││
│  └──────────────────────────────────────┼───────────────────────────────────────┘│
│                                         ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │                         SECONDARY DATA SOURCES                              ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         ││
│  │  │  Twitter/X  │  │   Reddit    │  │  Telegram   │  │  YouTube    │         ││
│  │  │  Mentions   │  │  r/crypto   │  │  Channels   │  │  Sentiment  │         ││
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         ││
│  │         │                │                │                │                 ││
│  │         ▼                ▼                ▼                ▼                 ││
│  │  ┌───────────────────────────────────────────────────────────────────────┐  ││
│  │  │                     SOCIAL SENTIMENT ANALYZER                         │  ││
│  │  │  • Volume of mentions (buzz score)                                    │  ││
│  │  │  • Sentiment polarity (positive/negative/neutral)                     │  ││
│  │  │  • Influencer impact weighting                                        │  ││
│  │  │  • Viral content detection                                            │  ││
│  │  └───────────────────────────────────────────────────────────────────────┘  ││
│  │                                      │                                       ││
│  └──────────────────────────────────────┼───────────────────────────────────────┘│
│                                         ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │                         ON-CHAIN DATA SOURCES                               ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         ││
│  │  │  Glassnode  │  │  Whale      │  │  Exchange   │  │  Funding    │         ││
│  │  │  Metrics    │  │  Alert      │  │  Flows      │  │   Rates     │         ││
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         ││
│  │         │                │                │                │                 ││
│  │         ▼                ▼                ▼                ▼                 ││
│  │  ┌───────────────────────────────────────────────────────────────────────┐  ││
│  │  │                      ON-CHAIN SIGNAL PROCESSOR                        │  ││
│  │  │  • Whale accumulation/distribution                                    │  ││
│  │  │  • Exchange inflow/outflow (selling/buying pressure)                  │  ││
│  │  │  • Funding rate extremes (overleveraged market)                       │  ││
│  │  │  • Active addresses trend                                             │  ││
│  │  └───────────────────────────────────────────────────────────────────────┘  ││
│  │                                      │                                       ││
│  └──────────────────────────────────────┼───────────────────────────────────────┘│
│                                         ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │                      UNIFIED SENTIMENT CONTEXT                              ││
│  │                                                                              ││
│  │  {                                                                           ││
│  │    "symbol": "BTCUSDT",                                                      ││
│  │    "timestamp": "2025-12-26T10:00:00Z",                                      ││
│  │    "aggregated_sentiment": 72,        // -100 to +100                        ││
│  │    "news_headlines": [...],                                                  ││
│  │    "social_buzz_score": 85,           // 0-100 (volume)                      ││
│  │    "whale_activity": "accumulating",  // accumulating/distributing/neutral   ││
│  │    "exchange_flow": "outflow",        // inflow/outflow/neutral              ││
│  │    "fear_greed_index": 65,            // 0-100                               ││
│  │    "funding_rate_signal": "neutral"   // overleveraged_long/short/neutral    ││
│  │  }                                                                           ││
│  │                           │                                                  ││
│  │                           ▼                                                  ││
│  │              ┌─────────────────────────┐                                     ││
│  │              │   INJECT INTO LLM       │                                     ││
│  │              │   ANALYSIS PROMPT       │                                     ││
│  │              └─────────────────────────┘                                     ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 6.2 Data Source Providers

##### 6.2.1 News Sources

| Provider | Type | Data Provided | Cost | Rate Limit | Priority |
|----------|------|---------------|------|------------|----------|
| **CryptoCompare News** | REST API | Crypto news headlines, categories | Free tier | 100K/month | PRIMARY |
| **CryptoPanic** | REST API | Aggregated news, voting sentiment | Free tier | 5 req/min | PRIMARY |
| **Messari** | REST API | Research, news, asset profiles | Free tier | 20 req/min | SECONDARY |
| **The Block** | RSS Feed | Institutional news | Free | N/A | SECONDARY |
| **CoinDesk** | RSS Feed | Industry news | Free | N/A | SECONDARY |
| **Decrypt** | RSS Feed | News with sentiment tags | Free | N/A | FALLBACK |

##### 6.2.2 Sentiment & Social Sources

| Provider | Type | Data Provided | Cost | Rate Limit | Priority |
|----------|------|---------------|------|------------|----------|
| **LunarCrush** | REST API | Social volume, sentiment, influencers | Free tier | 10 req/min | PRIMARY |
| **Santiment** | GraphQL | Social trends, dev activity | Free tier | 300 req/day | PRIMARY |
| **Alternative.me** | REST API | Fear & Greed Index | Free | 60 req/hour | PRIMARY |
| **CoinGecko** | REST API | Community score, social stats | Free tier | 10 req/min | SECONDARY |
| **Twitter/X API** | REST API | Mentions, trending, sentiment | $100/mo | 100 req/15min | OPTIONAL |

##### 6.2.3 On-Chain Data Sources

| Provider | Type | Data Provided | Cost | Rate Limit | Priority |
|----------|------|---------------|------|------------|----------|
| **Glassnode** | REST API | On-chain metrics, whale alerts | Free tier | 10 req/min | PRIMARY |
| **CryptoQuant** | REST API | Exchange flows, funding rates | Free tier | 60 req/hour | PRIMARY |
| **WhaleAlert** | REST API | Large transaction notifications | Free tier | 10 req/min | SECONDARY |
| **Coinglass** | REST API | Funding rates, liquidations | Free tier | 60 req/min | SECONDARY |
| **DefiLlama** | REST API | TVL changes, protocol flows | Free | Generous | FALLBACK |

#### 6.3 Sentiment Score Normalization

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    SENTIMENT NORMALIZATION ALGORITHM                             │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  INPUT: Raw sentiment from multiple sources (different scales)                   │
│  OUTPUT: Unified score from -100 (extreme fear/bearish) to +100 (extreme greed) │
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ SOURCE NORMALIZATION FORMULAS                                               ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │ Fear & Greed Index (0-100):                                                  ││
│  │   normalized = (raw - 50) * 2                                                ││
│  │   Example: 75 → (75-50)*2 = +50                                              ││
│  │                                                                              ││
│  │ LunarCrush Galaxy Score (0-100):                                             ││
│  │   normalized = (raw - 50) * 2                                                ││
│  │   Example: 80 → (80-50)*2 = +60                                              ││
│  │                                                                              ││
│  │ CryptoPanic Votes (positive/negative count):                                 ││
│  │   ratio = positive / (positive + negative)                                   ││
│  │   normalized = (ratio - 0.5) * 200                                           ││
│  │   Example: 70% positive → (0.7-0.5)*200 = +40                                ││
│  │                                                                              ││
│  │ Santiment Social Volume (relative to 30d avg):                               ││
│  │   IF volume > 2x avg: buzz_boost = +20                                       ││
│  │   IF volume < 0.5x avg: buzz_penalty = -10                                   ││
│  │   ELSE: neutral = 0                                                          ││
│  │                                                                              ││
│  │ Funding Rate (-0.1% to +0.1%):                                               ││
│  │   IF rate > +0.05%: overleveraged_long = -30 (contrarian bearish)            ││
│  │   IF rate < -0.05%: overleveraged_short = +30 (contrarian bullish)           ││
│  │   ELSE: neutral = 0                                                          ││
│  │                                                                              ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ WEIGHTED AGGREGATION                                                        ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │ Source Weights (configurable):                                               ││
│  │ ┌────────────────────────────────────────────────────────────────────────┐  ││
│  │ │ Source              │ Weight │ Reasoning                              │  ││
│  │ ├────────────────────────────────────────────────────────────────────────┤  ││
│  │ │ Fear & Greed Index  │ 0.25   │ Well-established, widely followed      │  ││
│  │ │ LunarCrush          │ 0.20   │ Real-time social sentiment             │  ││
│  │ │ On-Chain (Whale)    │ 0.20   │ Smart money movements                  │  ││
│  │ │ News Sentiment      │ 0.15   │ Headline-driven moves                  │  ││
│  │ │ Funding Rate        │ 0.10   │ Contrarian signal                      │  ││
│  │ │ Social Buzz         │ 0.10   │ Retail attention                       │  ││
│  │ └────────────────────────────────────────────────────────────────────────┘  ││
│  │                                                                              ││
│  │ FORMULA:                                                                     ││
│  │ aggregated_sentiment = Σ (source_normalized × source_weight)                 ││
│  │                                                                              ││
│  │ EXAMPLE:                                                                     ││
│  │   Fear & Greed: +50 × 0.25 = +12.5                                           ││
│  │   LunarCrush: +60 × 0.20 = +12.0                                             ││
│  │   On-Chain: +40 × 0.20 = +8.0                                                ││
│  │   News: +30 × 0.15 = +4.5                                                    ││
│  │   Funding: 0 × 0.10 = 0                                                      ││
│  │   Social Buzz: +20 × 0.10 = +2.0                                             ││
│  │   ─────────────────────────────                                              ││
│  │   TOTAL: +39.0 (Moderately Bullish)                                          ││
│  │                                                                              ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 6.4 News Categorization & Impact Scoring

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    NEWS IMPACT CLASSIFICATION                                    │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ CATEGORY DEFINITIONS                                                        ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │ 🔴 HIGH IMPACT (immediate price action expected)                             ││
│  │    • Regulatory: SEC decisions, country bans/approvals                       ││
│  │    • ETF: Approval/rejection, major inflows/outflows                         ││
│  │    • Hack/Exploit: Exchange hacks, protocol exploits                         ││
│  │    • Macro: Fed rate decisions, inflation data                               ││
│  │    • Whale: $100M+ transactions                                              ││
│  │    Impact Score: 80-100                                                      ││
│  │                                                                              ││
│  │ 🟠 MEDIUM IMPACT (gradual price influence)                                   ││
│  │    • Partnership: Major company integrations                                 ││
│  │    • Listing: New exchange listings                                          ││
│  │    • Upgrade: Protocol upgrades, hard forks                                  ││
│  │    • Funding: VC rounds, treasury movements                                  ││
│  │    • Legal: Lawsuits, settlements                                            ││
│  │    Impact Score: 40-79                                                       ││
│  │                                                                              ││
│  │ 🟢 LOW IMPACT (background sentiment)                                         ││
│  │    • Opinion: Analyst predictions, influencer takes                          ││
│  │    • Development: GitHub updates, roadmap progress                           ││
│  │    • Community: AMAs, conferences, meetups                                   ││
│  │    • Education: How-to articles, explainers                                  ││
│  │    Impact Score: 0-39                                                        ││
│  │                                                                              ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ KEYWORD-BASED CLASSIFICATION                                                ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │ HIGH_IMPACT_BULLISH = [                                                      ││
│  │   "ETF approved", "SEC approval", "institutional adoption",                  ││
│  │   "record inflows", "legal victory", "country adopts",                       ││
│  │   "major partnership", "rate cut", "whale accumulation"                      ││
│  │ ]                                                                            ││
│  │                                                                              ││
│  │ HIGH_IMPACT_BEARISH = [                                                      ││
│  │   "hack", "exploit", "rug pull", "SEC lawsuit", "banned",                    ││
│  │   "exchange insolvent", "massive outflows", "rate hike",                     ││
│  │   "whale dump", "delisting", "shutdown"                                      ││
│  │ ]                                                                            ││
│  │                                                                              ││
│  │ MEDIUM_IMPACT_BULLISH = [                                                    ││
│  │   "partnership", "integration", "listed on", "upgrade",                      ││
│  │   "funding round", "expansion", "all-time high"                              ││
│  │ ]                                                                            ││
│  │                                                                              ││
│  │ MEDIUM_IMPACT_BEARISH = [                                                    ││
│  │   "investigation", "lawsuit filed", "delay", "postponed",                    ││
│  │   "security concern", "vulnerability", "layoffs"                             ││
│  │ ]                                                                            ││
│  │                                                                              ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                                  │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │ NEWS PROCESSING FLOW                                                        ││
│  ├─────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                              ││
│  │  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐││
│  │  │ Fetch News  │────▶│  Dedupe &   │────▶│  Classify   │────▶│   Score &   │││
│  │  │ from APIs   │     │   Filter    │     │  Category   │     │   Weight    │││
│  │  └─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘││
│  │                                                                              ││
│  │  For each news item:                                                         ││
│  │  1. Extract symbol mentions (BTCUSDT, ETHUSDT, etc.)                         ││
│  │  2. Match keywords to category                                               ││
│  │  3. Assign impact score (0-100)                                              ││
│  │  4. Determine sentiment polarity (bullish/bearish/neutral)                   ││
│  │  5. Apply recency decay (older news = less weight)                           ││
│  │                                                                              ││
│  │  Recency Decay Formula:                                                      ││
│  │    weight = base_weight × e^(-hours_old / decay_rate)                        ││
│  │    decay_rate = 6 hours (news loses 63% weight after 6 hours)                ││
│  │                                                                              ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 6.5 Data Refresh & Caching Strategy

| Data Type | Refresh Interval | Cache TTL | Reason |
|-----------|------------------|-----------|--------|
| **Fear & Greed Index** | 1 hour | 30 min | Updated hourly by provider |
| **News Headlines** | 5 minutes | 2 min | Breaking news matters |
| **Social Sentiment** | 15 minutes | 10 min | Social trends change slowly |
| **Whale Alerts** | 1 minute | 30 sec | Real-time critical |
| **Funding Rates** | 1 minute | 30 sec | Changes frequently |
| **Exchange Flows** | 15 minutes | 10 min | Aggregated data |

#### 6.6 Configuration Structure

```json
{
  "sentiment_config": {
    "enabled": true,
    "refresh_interval_sec": 300,
    "cache_enabled": true,

    "sources": {
      "fear_greed": {
        "enabled": true,
        "provider": "alternative_me",
        "weight": 0.25,
        "api_key": ""
      },
      "lunar_crush": {
        "enabled": true,
        "provider": "lunarcrush",
        "weight": 0.20,
        "api_key": "${LUNARCRUSH_API_KEY}"
      },
      "on_chain": {
        "enabled": true,
        "providers": ["glassnode", "cryptoquant"],
        "weight": 0.20,
        "glassnode_api_key": "${GLASSNODE_API_KEY}",
        "cryptoquant_api_key": ""
      },
      "news": {
        "enabled": true,
        "providers": ["cryptopanic", "cryptocompare"],
        "weight": 0.15,
        "cryptopanic_api_key": "${CRYPTOPANIC_API_KEY}",
        "max_headlines": 10,
        "recency_hours": 24
      },
      "funding_rate": {
        "enabled": true,
        "provider": "coinglass",
        "weight": 0.10,
        "contrarian_mode": true
      },
      "social_buzz": {
        "enabled": true,
        "provider": "santiment",
        "weight": 0.10,
        "api_key": "${SANTIMENT_API_KEY}"
      }
    },

    "thresholds": {
      "extreme_fear": -70,
      "fear": -30,
      "neutral_low": -10,
      "neutral_high": 10,
      "greed": 30,
      "extreme_greed": 70
    },

    "mode_sentiment_weight": {
      "ultra_fast": 0.05,
      "scalp": 0.10,
      "swing": 0.20,
      "position": 0.30
    }
  }
}
```

#### 6.7 Sentiment-Enhanced LLM Prompt

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                    SENTIMENT DATA IN LLM PROMPT                                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  The LLM prompt includes a dedicated sentiment section:                          │
│                                                                                  │
│  {                                                                               │
│    "symbol": "BTCUSDT",                                                          │
│    "price_data": { ... },                                                        │
│    "technical_signals": { ... },                                                 │
│                                                                                  │
│    "sentiment_data": {                                                           │
│      "aggregated_score": 45,                                                     │
│      "interpretation": "Moderately Bullish",                                     │
│                                                                                  │
│      "fear_greed": {                                                             │
│        "value": 65,                                                              │
│        "label": "Greed",                                                         │
│        "change_24h": +5                                                          │
│      },                                                                          │
│                                                                                  │
│      "social": {                                                                 │
│        "buzz_score": 78,                                                         │
│        "sentiment_polarity": "positive",                                         │
│        "trending_hashtags": ["#Bitcoin", "#BTCto100k"],                          │
│        "influencer_sentiment": "bullish"                                         │
│      },                                                                          │
│                                                                                  │
│      "on_chain": {                                                               │
│        "whale_activity": "accumulating",                                         │
│        "whale_transactions_24h": 45,                                             │
│        "exchange_netflow": "outflow",                                            │
│        "exchange_netflow_btc": -2500,                                            │
│        "active_addresses_change": "+3.2%"                                        │
│      },                                                                          │
│                                                                                  │
│      "funding_rate": {                                                           │
│        "current": 0.012,                                                         │
│        "signal": "slightly_long_heavy",                                          │
│        "interpretation": "Mild contrarian bearish pressure"                      │
│      },                                                                          │
│                                                                                  │
│      "recent_news": [                                                            │
│        {                                                                         │
│          "headline": "Bitcoin ETF sees $500M inflows in single day",             │
│          "source": "CoinDesk",                                                   │
│          "category": "ETF",                                                      │
│          "impact": "HIGH",                                                       │
│          "sentiment": "bullish",                                                 │
│          "age_hours": 2                                                          │
│        },                                                                        │
│        {                                                                         │
│          "headline": "Fed signals potential rate pause in Q1",                   │
│          "source": "Reuters",                                                    │
│          "category": "Macro",                                                    │
│          "impact": "HIGH",                                                       │
│          "sentiment": "bullish",                                                 │
│          "age_hours": 5                                                          │
│        },                                                                        │
│        {                                                                         │
│          "headline": "Whale moves 5000 BTC to cold storage",                     │
│          "source": "WhaleAlert",                                                 │
│          "category": "OnChain",                                                  │
│          "impact": "MEDIUM",                                                     │
│          "sentiment": "bullish",                                                 │
│          "age_hours": 1                                                          │
│        }                                                                         │
│      ]                                                                           │
│    }                                                                             │
│  }                                                                               │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 6.8 Sentiment Dashboard UI

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         SENTIMENT DASHBOARD UI                                   │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌───────────────────────────────────────────────────────────────────────────┐  │
│  │  📊 MARKET SENTIMENT OVERVIEW                        Last Update: 10:05   │  │
│  ├───────────────────────────────────────────────────────────────────────────┤  │
│  │                                                                           │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │  │
│  │  │      AGGREGATED SENTIMENT                                           │ │  │
│  │  │                                                                      │ │  │
│  │  │  ◀──────────────────────●──────────────────────▶                    │ │  │
│  │  │  -100        -50        0    +45    +50       +100                   │ │  │
│  │  │  Extreme Fear      Neutral      GREED      Extreme Greed             │ │  │
│  │  │                                                                      │ │  │
│  │  │  Current: +45 (Moderately Bullish) ↑ +8 from 24h ago                │ │  │
│  │  └─────────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                           │  │
│  │  ┌───────────────┬───────────────┬───────────────┬───────────────┐       │  │
│  │  │ Fear & Greed  │ Social Buzz   │ Whale Signal  │ Funding Rate  │       │  │
│  │  │     65 🟢     │    78 🟢      │  Accumulate   │   +0.012%     │       │  │
│  │  │    Greed      │   High Vol    │    🐋 ↑      │   Neutral     │       │  │
│  │  └───────────────┴───────────────┴───────────────┴───────────────┘       │  │
│  │                                                                           │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │  │
│  │  │  📰 LATEST NEWS                                           [View All] │ │  │
│  │  ├─────────────────────────────────────────────────────────────────────┤ │  │
│  │  │                                                                      │ │  │
│  │  │  🔴 HIGH | Bitcoin ETF sees $500M inflows          2h ago  [Bullish]│ │  │
│  │  │  🔴 HIGH | Fed signals potential rate pause        5h ago  [Bullish]│ │  │
│  │  │  🟠 MED  | Whale moves 5000 BTC to storage         1h ago  [Bullish]│ │  │
│  │  │  🟠 MED  | Ethereum upgrade scheduled for Q1       8h ago  [Neutral]│ │  │
│  │  │  🟢 LOW  | Analyst predicts $150k by 2026         12h ago  [Bullish]│ │  │
│  │  │                                                                      │ │  │
│  │  └─────────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                           │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │  │
│  │  │  🐋 ON-CHAIN SIGNALS                                                │ │  │
│  │  ├─────────────────────────────────────────────────────────────────────┤ │  │
│  │  │                                                                      │ │  │
│  │  │  Exchange Flow (24h):  -2,500 BTC  🟢 Outflow = Bullish             │ │  │
│  │  │  Whale Transactions:   45 large moves (>$1M)                         │ │  │
│  │  │  Active Addresses:     +3.2% vs 7-day avg                            │ │  │
│  │  │  Stablecoin Inflow:    +$150M USDT to exchanges                      │ │  │
│  │  │                                                                      │ │  │
│  │  └─────────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                           │  │
│  │  [Configure Sources] [Refresh Now] [View Historical]                     │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 6.9 Sentiment API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/futures/ginie/sentiment` | GET | Get aggregated sentiment data |
| `/api/futures/ginie/sentiment/config` | GET | Get sentiment source configuration |
| `/api/futures/ginie/sentiment/config` | PUT | Update sentiment source settings |
| `/api/futures/ginie/sentiment/news` | GET | Get recent news headlines |
| `/api/futures/ginie/sentiment/onchain` | GET | Get on-chain signals |
| `/api/futures/ginie/sentiment/refresh` | POST | Force refresh sentiment data |
| `/api/futures/ginie/sentiment/history` | GET | Get historical sentiment |

#### 6.10 Additional Acceptance Criteria for Sentiment

| ID | Criteria | Verification |
|----|----------|--------------|
| **AC-2.8.17** | Fear & Greed Index fetched and displayed | Dashboard shows current value |
| **AC-2.8.18** | News headlines fetched from multiple sources | At least 3 sources providing data |
| **AC-2.8.19** | News categorized by impact level | HIGH/MEDIUM/LOW labels assigned |
| **AC-2.8.20** | On-chain whale activity tracked | Accumulation/distribution detected |
| **AC-2.8.21** | Funding rate signal interpreted | Contrarian mode working |
| **AC-2.8.22** | Sentiment aggregated with configurable weights | Weights sum to 1.0 |
| **AC-2.8.23** | Sentiment data injected into LLM prompt | Prompt includes sentiment section |
| **AC-2.8.24** | Caching prevents excessive API calls | Rate limits respected |
| **AC-2.8.25** | Source failure doesn't break aggregation | Graceful degradation |
| **AC-2.8.26** | Sentiment dashboard visible in UI | All components rendered |

#### 6.11 Additional Technical Tasks for Sentiment

| Task | Description | File | Priority |
|------|-------------|------|----------|
| **2.8.19** | Create SentimentConfig struct | settings.go | **HIGH** |
| **2.8.20** | Implement Fear & Greed API client | sentiment/fear_greed.go (new) | **HIGH** |
| **2.8.21** | Implement CryptoPanic news client | sentiment/news.go (new) | **HIGH** |
| **2.8.22** | Implement LunarCrush social client | sentiment/social.go (new) | **MEDIUM** |
| **2.8.23** | Implement on-chain data aggregator | sentiment/onchain.go (new) | **MEDIUM** |
| **2.8.24** | Implement funding rate client | sentiment/funding.go (new) | **MEDIUM** |
| **2.8.25** | Create sentiment normalizer service | sentiment/normalizer.go (new) | **HIGH** |
| **2.8.26** | Build sentiment aggregator with caching | sentiment/aggregator.go (new) | **HIGH** |
| **2.8.27** | Add sentiment to LLM prompt builder | ginie_analyzer.go | **HIGH** |
| **2.8.28** | Create sentiment API handlers | handlers_sentiment.go (new) | **MEDIUM** |
| **2.8.29** | Build Sentiment Dashboard component | SentimentDashboard.tsx (new) | **MEDIUM** |
| **2.8.30** | Add news feed component | NewsFeed.tsx (new) | **LOW** |

---

### Component 7: AI Decision Logging & Transparency

#### 7.1 Decision Explanation in UI

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                         TRADE DECISION EXPLANATION UI                            │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                  │
│  ┌───────────────────────────────────────────────────────────────────────────┐  │
│  │  📊 BTCUSDT Trade Decision                              [EXECUTED: LONG]  │  │
│  ├───────────────────────────────────────────────────────────────────────────┤  │
│  │                                                                           │  │
│  │  FINAL CONFIDENCE: 87%                                     Mode: Swing   │  │
│  │  ═══════════════════════════════════════════════════                     │  │
│  │                                                                           │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │  │
│  │  │ TECHNICAL ANALYSIS (60% weight)                    Confidence: 75%  │ │  │
│  │  ├─────────────────────────────────────────────────────────────────────┤ │  │
│  │  │ ✅ RSI (14): 58 - Neutral, room to run                              │ │  │
│  │  │ ✅ MACD: Bullish crossover 2 hours ago                              │ │  │
│  │  │ ✅ EMA: Price above 50 & 100 EMA                                    │ │  │
│  │  │ ⚠️ ADX: 24 - Trend strength moderate                               │ │  │
│  │  │ ✅ Volume: 1.3x average - Confirming                                │ │  │
│  │  └─────────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                           │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │  │
│  │  │ LLM ANALYSIS (40% weight)                          Confidence: 85%  │ │  │
│  │  ├─────────────────────────────────────────────────────────────────────┤ │  │
│  │  │ 🤖 Provider: DeepSeek (deepseek-chat)                               │ │  │
│  │  │                                                                      │ │  │
│  │  │ Reasoning:                                                           │ │  │
│  │  │ "Strong bullish setup with institutional interest. ETF inflows      │ │  │
│  │  │  continue at record pace. Fear & Greed at 65 suggests room for      │ │  │
│  │  │  further upside before overheating. Technical breakout above        │ │  │
│  │  │  99k resistance would target 102k."                                  │ │  │
│  │  │                                                                      │ │  │
│  │  │ Key Factors:                                                         │ │  │
│  │  │ • ETF inflows at record levels                                       │ │  │
│  │  │ • Whale accumulation detected                                        │ │  │
│  │  │ • Fed pause favorable for risk assets                                │ │  │
│  │  │                                                                      │ │  │
│  │  │ Suggested: SL: 2.0% | TP: 4.0% | Horizon: Swing                      │ │  │
│  │  └─────────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                           │  │
│  │  ┌─────────────────────────────────────────────────────────────────────┐ │  │
│  │  │ FUSION RESULT                                                       │ │  │
│  │  ├─────────────────────────────────────────────────────────────────────┤ │  │
│  │  │ Base: (75 × 0.6) + (85 × 0.4) = 45 + 34 = 79                        │ │  │
│  │  │ Agreement Bonus: +10 (both LONG)                                     │ │  │
│  │  │ Final: 79 + 10 = 89 → Rounded: 87%                                   │ │  │
│  │  │                                                                      │ │  │
│  │  │ ✅ PASSED: Min confidence 55% for Swing mode                        │ │  │
│  │  │ ✅ PASSED: No disagreement blocking                                  │ │  │
│  │  │ ✅ PASSED: Circuit breaker not triggered                            │ │  │
│  │  └─────────────────────────────────────────────────────────────────────┘ │  │
│  │                                                                           │  │
│  │  [View Full LLM Response] [View Market Snapshot] [Report Issue]          │  │
│  └───────────────────────────────────────────────────────────────────────────┘  │
│                                                                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

#### 7.2 AI Decision History Log

| Time | Symbol | Mode | Tech% | LLM% | Final% | Direction | Outcome | P&L |
|------|--------|------|-------|------|--------|-----------|---------|-----|
| 10:00 | BTCUSDT | Swing | 75 | 85 | 87 | LONG | WIN | +2.1% |
| 09:45 | ETHUSDT | Scalp | 68 | 72 | 73 | LONG | WIN | +0.8% |
| 09:30 | SOLUSDT | Ultra | 82 | 45 | 71 | LONG | LOSS | -0.5% |
| 09:15 | BTCUSDT | Position | 60 | 40 | 48 | SKIP | - | - |

---

### Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|--------------|
| **AC-2.8.1** | LLM provider is configurable (DeepSeek, Claude, OpenAI, Local) | Settings show provider selection |
| **AC-2.8.2** | LLM analysis is requested for each trade decision | Logs show LLM calls per symbol |
| **AC-2.8.3** | LLM response is validated and fallback used on failure | Invalid responses trigger fallback |
| **AC-2.8.4** | Confidence fusion formula applies per mode weights | Final confidence matches formula |
| **AC-2.8.5** | Agreement/disagreement bonus/penalty applied | Logs show fusion calculation |
| **AC-2.8.6** | LLM reasoning stored with trade history | Trade history shows AI context |
| **AC-2.8.7** | Adaptive learning runs every 50 trades or 24h | Learning job executes on schedule |
| **AC-2.8.8** | Adaptive recommendations shown in UI | Dashboard displays suggestions |
| **AC-2.8.9** | User can approve/dismiss adaptive adjustments | UI has approve/dismiss buttons |
| **AC-2.8.10** | LLM weight customizable per mode | Settings editable via UI/API |
| **AC-2.8.11** | Skip on timeout works for fast modes | Ultra-fast doesn't wait for LLM |
| **AC-2.8.12** | Block on disagreement works for slow modes | Swing/Position skip conflicts |
| **AC-2.8.13** | LLM cache reduces duplicate calls | Same symbol within 5min uses cache |
| **AC-2.8.14** | Decision explanation visible in trade detail | UI shows full AI breakdown |
| **AC-2.8.15** | All settings have Story 2.8 defaults | Default values match documentation |
| **AC-2.8.16** | User settings override defaults | Customizations persist |

---

### Technical Tasks

| Task | Description | File | Priority |
|------|-------------|------|----------|
| **2.8.1** | Add LLMConfig and ModeLLMSettings structs | settings.go | **HIGH** |
| **2.8.2** | Add AdaptiveAIConfig struct | settings.go | **HIGH** |
| **2.8.3** | Implement LLM prompt builder with context injection | ginie_analyzer.go | **HIGH** |
| **2.8.4** | Implement LLM response parser with validation | ginie_analyzer.go | **HIGH** |
| **2.8.5** | Implement confidence fusion algorithm | ginie_analyzer.go | **HIGH** |
| **2.8.6** | Add LLM call to GenerateDecision flow | ginie_analyzer.go | **HIGH** |
| **2.8.7** | Implement LLM response caching | ginie_analyzer.go | **MEDIUM** |
| **2.8.8** | Store decision context with trade history | ginie_autopilot.go | **MEDIUM** |
| **2.8.9** | Implement adaptive learning job | ginie_adaptive.go (new) | **HIGH** |
| **2.8.10** | Generate adjustment recommendations | ginie_adaptive.go | **MEDIUM** |
| **2.8.11** | Add GET /api/futures/ginie/llm-config endpoint | handlers_ginie.go | **HIGH** |
| **2.8.12** | Add PUT /api/futures/ginie/llm-config/:mode endpoint | handlers_ginie.go | **HIGH** |
| **2.8.13** | Add GET /api/futures/ginie/adaptive-recommendations | handlers_ginie.go | **MEDIUM** |
| **2.8.14** | Add POST /api/futures/ginie/adaptive-apply | handlers_ginie.go | **MEDIUM** |
| **2.8.15** | Add AI Decision panel to trade detail UI | GiniePanel.tsx | **MEDIUM** |
| **2.8.16** | Add Adaptive AI recommendations UI | GiniePanel.tsx | **MEDIUM** |
| **2.8.17** | Add LLM settings configuration UI | GiniePanel.tsx | **MEDIUM** |
| **2.8.18** | Add decision history with AI context | TradeHistory.tsx | **LOW** |

---

### API Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/futures/ginie/llm-config` | GET | Get LLM configuration |
| `/api/futures/ginie/llm-config/:mode` | PUT | Update mode LLM settings |
| `/api/futures/ginie/adaptive-config` | GET | Get adaptive AI config |
| `/api/futures/ginie/adaptive-config` | PUT | Update adaptive AI config |
| `/api/futures/ginie/adaptive-recommendations` | GET | Get pending recommendations |
| `/api/futures/ginie/adaptive-apply` | POST | Apply recommendations |
| `/api/futures/ginie/adaptive-dismiss` | POST | Dismiss recommendations |
| `/api/futures/ginie/decision-history` | GET | Get decisions with AI context |

---

### Dependencies

- **Story 2.7**: Mode-specific configuration structure must be in place
- **LLM Provider**: DeepSeek/Claude/OpenAI API access configured
- **Trade History**: Position history must store decision context

---

### Definition of Done Checklist

- [ ] **LLM integration** calls provider for each decision
- [ ] **Fallback logic** switches provider on failure
- [ ] **Confidence fusion** applies weights correctly
- [ ] **Agreement/disagreement** modifiers work
- [ ] **Mode-specific LLM settings** configurable
- [ ] **Adaptive learning** analyzes outcomes
- [ ] **Recommendations** generated and displayed
- [ ] **User approval** required for auto-adjustments
- [ ] **Decision context** stored with trades
- [ ] **UI shows** full AI reasoning
- [ ] **All defaults** match Story 2.8 documentation
- [ ] **User settings** persist and override defaults

---

## Story 2.8 UI Wireframes

### Wireframe 1: Ginie Main Dashboard

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  🤖 GINIE AUTOPILOT                                           [LIVE] 🟢    ⚙️ Settings    👤 User  │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  STATUS BAR                                                                                     ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐          ││
│  │  │ 🟢 RUNNING   │ │ Mode: Multi  │ │ Positions: 7 │ │ Today P&L:   │ │ Win Rate:    │          ││
│  │  │              │ │              │ │ /10 max      │ │ +$124.50     │ │ 67% (8/12)   │          ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘          ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  📊 MODE ALLOCATION                        │ │  📈 MARKET SENTIMENT                            ││
│  │                                            │ │                                                  ││
│  │  ┌──────────────────────────────────────┐  │ │  ◀────────────────●────────────────────▶        ││
│  │  │ ⚡ Ultra-Fast  20%  ███░░░░░  2 pos  │  │ │  -100     Fear    0   +45  Greed     +100       ││
│  │  │ 🏃 Scalp       30%  █████░░░  3 pos  │  │ │                                                  ││
│  │  │ 🌊 Swing       35%  ██████░░  1 pos  │  │ │  Fear & Greed: 65 🟢  |  Whale: Accumulating 🐋 ││
│  │  │ 🏔️ Position    15%  ██░░░░░░  1 pos  │  │ │  Funding: +0.01%     |  News: Bullish (3 HIGH) ││
│  │  └──────────────────────────────────────┘  │ │                                                  ││
│  │                                            │ │  [View Details]                                  ││
│  │  Capital: $2,500 | Used: $1,875 (75%)      │ │                                                  ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  📋 ACTIVE POSITIONS                                                            [Expand All]    ││
│  ├─────────────────────────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                                                  ││
│  │  ┌─────────┬────────┬────────┬──────────┬──────────┬──────────┬──────────┬─────────┬───────────┐││
│  │  │ Symbol  │ Mode   │ Side   │ Entry    │ Current  │ P&L      │ SL/TP    │ROI Tgt %│ Action    │││
│  │  ├─────────┼────────┼────────┼──────────┼──────────┼──────────┼──────────┼─────────┼───────────┤││
│  │  │ BTCUSDT │ 🌊Swing│ LONG   │ 98,500   │ 99,150   │ +$32.50  │ 96k/102k │ 5.0% 🎯 │[Close][✏️]│││
│  │  │         │        │        │          │ +0.66%   │ ROI:3.3% │          │ custom  │ [▼ View]  │││
│  │  ├─────────┼────────┼────────┼──────────┼──────────┼──────────┼──────────┼─────────┼───────────┤││
│  │  │ ETHUSDT │ 🏃Scalp│ LONG   │ 3,450    │ 3,478    │ +$14.00  │ 3.4k/3.5k│ —       │[Close][✏️]│││
│  │  │         │        │        │          │ +0.81%   │ ROI:4.1% │          │ default │ [▼ View]  │││
│  │  ├─────────┼────────┼────────┼──────────┼──────────┼──────────┼──────────┼─────────┼───────────┤││
│  │  │ SOLUSDT │ ⚡Ultra │ SHORT  │ 185.20   │ 184.50   │ +$3.80   │ 187/183  │ 4.3% 🎯 │[Close][✏️]│││
│  │  │         │        │        │          │ +0.38%   │ ROI:1.9% │          │ custom  │ [▼ View]  │││
│  │  └─────────┴────────┴────────┴──────────┴──────────┴──────────┴──────────┴─────────┴───────────┘││
│  │                                                                                                  ││
│  │  Legend: 🎯 = Custom ROI target set | — = Using mode defaults | ROI = Current ROI after fees   ││
│  │  [+ Show 4 more positions...]                                                                    ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  🤖 RECENT AI DECISIONS                    │ │  📰 LATEST NEWS                                 ││
│  │                                            │ │                                                  ││
│  │  10:05 BTCUSDT → LONG (87%)               │ │  🔴 Bitcoin ETF $500M inflows        2h  [Bull] ││
│  │    Tech: 75% | LLM: 85% | Agree ✓         │ │  🔴 Fed signals rate pause           5h  [Bull] ││
│  │    [View Full Analysis]                    │ │  🟠 Whale moves 5000 BTC             1h  [Bull] ││
│  │                                            │ │  🟠 ETH upgrade Q1 2026              8h  [Neut] ││
│  │  09:45 ETHUSDT → LONG (72%)               │ │                                                  ││
│  │    Tech: 68% | LLM: 72% | Agree ✓         │ │  [View All News →]                              ││
│  │    [View Full Analysis]                    │ │                                                  ││
│  │                                            │ │                                                  ││
│  │  09:30 AVAXUSDT → SKIP (48%)              │ │                                                  ││
│  │    Tech: 60% | LLM: 40% | Disagree ✗      │ │                                                  ││
│  │    [View Full Analysis]                    │ │                                                  ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  QUICK CONTROLS                                                                                 ││
│  │                                                                                                  ││
│  │  [🟢 Start Ginie]  [⏸️ Pause]  [🔴 Stop All]  [⚙️ Mode Config]  [📊 Analytics]  [📜 History]    ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 2: Mode Configuration Panel

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  ⚙️ MODE CONFIGURATION                                                              [✕ Close]      │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  SELECT MODE:   [⚡ Ultra-Fast]  [🏃 Scalp]  [🌊 Swing ✓]  [🏔️ Position]                        ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  🌊 SWING MODE SETTINGS                    │ │  📊 SWING MODE PERFORMANCE                      ││
│  │                                            │ │                                                  ││
│  │  ☑️ Enabled                                │ │  Total Trades: 156                              ││
│  │                                            │ │  Win Rate: 62.3%                                ││
│  │  ─────────────────────────────────────     │ │  Total P&L: +$487.50                            ││
│  │  TIMEFRAME & CONFIDENCE                    │ │  Avg Hold: 4.2 hours                            ││
│  │  ─────────────────────────────────────     │ │                                                  ││
│  │                                            │ │  Best Symbol: BTCUSDT (+$156)                   ││
│  │  Trend Timeframe:    [1h     ▼]            │ │  Worst Symbol: XRPUSDT (-$45)                   ││
│  │  Min Confidence:     [55     ] %           │ │                                                  ││
│  │  LLM Weight:         [0.40   ]             │ │  ┌──────────────────────────────────────────┐   ││
│  │  Block Disagreement: [✓]                   │ │  │  P&L CHART (7 DAYS)                      │   ││
│  │                                            │ │  │   $60 ┤      ╭─╮                          │   ││
│  │  ─────────────────────────────────────     │ │  │   $40 ┤  ╭──╯  ╰──╮    ╭─╮               │   ││
│  │  POSITION SIZING                           │ │  │   $20 ┤ ╱         ╰───╯  ╰──╮            │   ││
│  │  ─────────────────────────────────────     │ │  │    $0 ┤╱                     ╰──         │   ││
│  │                                            │ │  │       └──┬──┬──┬──┬──┬──┬──┬──          │   ││
│  │  Capital Allocation: [35    ] %            │ │  │         M  T  W  T  F  S  S              │   ││
│  │  Max Positions:      [3     ]              │ │  └──────────────────────────────────────────┘   ││
│  │  Max USD/Position:   [$500  ]              │ │                                                  ││
│  │  Leverage:           [5     ] x            │ └──────────────────────────────────────────────────┘│
│  │                                            │                                                      │
│  │  ─────────────────────────────────────     │ ┌──────────────────────────────────────────────────┐│
│  │  STOP LOSS / TAKE PROFIT                   │ │  🛡️ CIRCUIT BREAKER                             ││
│  │  ─────────────────────────────────────     │ │                                                  ││
│  │                                            │ │  ☑️ Enabled                                      ││
│  │  ☑️ Use Manual SL/TP (override ATR/LLM)    │ │                                                  ││
│  │                                            │ │  Max Trades/Hour:     [30    ]                   ││
│  │  Stop Loss:          [2.5   ] %            │ │  Max Trades/Day:      [80    ]                   ││
│  │  Take Profit:        [5.0   ] %            │ │  Max Loss/Window:     [-3.0  ] %                 ││
│  │                                            │ │  Window Duration:     [60    ] min               ││
│  │  ☑️ Trailing Stop Enabled                  │ │  Cooldown on Trigger: [60    ] min               ││
│  │  Trailing %:         [1.5   ] %            │ │                                                  ││
│  │  Activation:         [1.0   ] % profit     │ │  ☑️ Win Rate Monitor                             ││
│  │                                            │ │  Sample Size:         [25    ]                   ││
│  │  ─────────────────────────────────────     │ │  Min Win Rate:        [55    ] %                 ││
│  │  TAKE PROFIT MODE                          │ │                                                  ││
│  │  ─────────────────────────────────────     │ └──────────────────────────────────────────────────┘│
│  │                                            │                                                      │
│  │  ○ Single TP (100% at target)              │ ┌──────────────────────────────────────────────────┐│
│  │  ● Multi-Level TP                          │ │  🔀 CONFLICT RESOLUTION                          ││
│  │                                            │ │                                                  ││
│  │  TP1 (25%): [1.5] %   TP2 (25%): [2.5] %   │ │  ☑️ Allow Hedge Mode                             ││
│  │  TP3 (25%): [4.0] %   TP4 (25%): [5.0] %   │ │  Opposite Size:       [50    ] %                 ││
│  │                                            │ │  Require Confirm:     [✓]                        ││
│  │  ─────────────────────────────────────     │ │                                                  ││
│  │  ORDER EXECUTION                           │ │  ☑️ Allow Position Averaging                     ││
│  │  ─────────────────────────────────────     │ │  Max Entries:         [3     ]                   ││
│  │                                            │ │  Avg Down Threshold:  [-1.5  ] %                 ││
│  │  Entry Order Type:                         │ │                                                  ││
│  │  ● Market (instant)  ○ Limit (at price)    │ │  ☑️ Stale Position Release                       ││
│  │  Limit Offset:       [0.05  ] %            │ │  Max Hold Time:       [8     ] hours             ││
│  │                                            │ │  Release at P&L:      [-0.5, +0.3] %             ││
│  │  Close Order Type:                         │ │                                                  ││
│  │  ● Market (instant)  ○ Limit (at target)   │ │  ─────────────────────────────────────────────   ││
│  │  Limit Offset:       [0.05  ] %            │ │  MODE DEFAULTS NOTE                              ││
│  │                                            │ │  ─────────────────────────────────────────────   ││
│  │  ☑️ Reduce Only (safety - prevents flip)   │ │  These settings apply as defaults when Swing    ││
│  │  ☑️ Post-Only (maker orders only)          │ │  mode opens positions. Override per-position.   ││
│  │                                            │ │                                                  ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [Reset to Defaults]                                      [Cancel]  [💾 Save Swing Settings]   ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 3: AI Decision Detail Modal

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  🤖 AI DECISION ANALYSIS                                                            [✕ Close]      │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  BTCUSDT  |  LONG  |  🌊 Swing Mode  |  Executed: 2025-12-26 10:05:32                           ││
│  │                                                                                                  ││
│  │  ════════════════════════════════════════════════════════════════════════════════════════════   ││
│  │  FINAL CONFIDENCE: 87%                                                     RESULT: EXECUTED    ││
│  │  ════════════════════════════════════════════════════════════════════════════════════════════   ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  📊 TECHNICAL ANALYSIS                     │ │  🤖 LLM ANALYSIS                                ││
│  │  Weight: 60%  |  Confidence: 75%           │ │  Weight: 40%  |  Confidence: 85%                ││
│  ├────────────────────────────────────────────┤ ├──────────────────────────────────────────────────┤│
│  │                                            │ │                                                  ││
│  │  INDICATORS:                               │ │  Provider: DeepSeek (deepseek-chat)             ││
│  │  ────────────────────────────────────────  │ │  Response Time: 1.2s                            ││
│  │                                            │ │                                                  ││
│  │  ✅ RSI (14):        58                    │ │  REASONING:                                      ││
│  │     Status: Neutral, room to run upward    │ │  ──────────────────────────────────────────────  ││
│  │                                            │ │                                                  ││
│  │  ✅ MACD:            Bullish Crossover     │ │  "Strong bullish setup with institutional       ││
│  │     Signal crossed above MACD 2h ago       │ │   interest. Bitcoin ETF inflows continue at     ││
│  │                                            │ │   record pace with $500M in single day. Fear    ││
│  │  ✅ EMA Trend:       Bullish               │ │   & Greed at 65 suggests room for further       ││
│  │     Price above 50 & 100 EMA               │ │   upside before market overheats. Technical     ││
│  │                                            │ │   breakout above 99k resistance would target    ││
│  │  ⚠️ ADX:             24                    │ │   102k-105k range. On-chain data shows whale    ││
│  │     Trend strength: Moderate               │ │   accumulation continuing."                     ││
│  │                                            │ │                                                  ││
│  │  ✅ Bollinger:       Upper Half            │ │  KEY FACTORS:                                    ││
│  │     Not overbought yet                     │ │  ──────────────────────────────────────────────  ││
│  │                                            │ │                                                  ││
│  │  ✅ Volume:          1.3x Average          │ │  • ETF inflows at record levels                  ││
│  │     Confirming bullish momentum            │ │  • Whale accumulation detected                   ││
│  │                                            │ │  • Fed pause favorable for risk assets           ││
│  │  ✅ ATR (14):        $1,250                │ │  • Technical breakout imminent                   ││
│  │     Volatility: Normal                     │ │                                                  ││
│  │                                            │ │  LLM SUGGESTIONS:                                ││
│  │  TREND ALIGNMENT:                          │ │  ──────────────────────────────────────────────  ││
│  │  ────────────────────────────────────────  │ │                                                  ││
│  │  15m: ✅ Bullish                           │ │  Stop Loss:    2.0%                              ││
│  │  1h:  ✅ Bullish                           │ │  Take Profit:  4.0%                              ││
│  │  4h:  ✅ Bullish                           │ │  Time Horizon: Swing (4-24h)                     ││
│  │  1D:  ✅ Bullish                           │ │  Risk Level:   Moderate                          ││
│  │                                            │ │                                                  ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  📈 SENTIMENT DATA                                                                              ││
│  ├─────────────────────────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                                                  ││
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐           ││
│  │  │ Aggregated   │ │ Fear & Greed │ │ Social Buzz  │ │ Whale Signal │ │ Funding Rate │           ││
│  │  │    +45 🟢    │ │    65 🟢     │ │   78 High    │ │ Accumulating │ │  +0.012%     │           ││
│  │  │  Mod.Bullish │ │    Greed     │ │              │ │    🐋 ↑      │ │   Neutral    │           ││
│  │  └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘           ││
│  │                                                                                                  ││
│  │  Recent News Impact:                                                                             ││
│  │  🔴 HIGH | "Bitcoin ETF sees $500M inflows in single day" - CoinDesk (2h ago) [Bullish]         ││
│  │  🔴 HIGH | "Fed signals potential rate pause in Q1" - Reuters (5h ago) [Bullish]                ││
│  │  🟠 MED  | "Whale moves 5000 BTC to cold storage" - WhaleAlert (1h ago) [Bullish]               ││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  🧮 CONFIDENCE FUSION CALCULATION                                                               ││
│  ├─────────────────────────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                                                  ││
│  │  Base Fusion = (Technical × Tech_Weight) + (LLM × LLM_Weight)                                   ││
│  │              = (75 × 0.60) + (85 × 0.40)                                                        ││
│  │              = 45 + 34                                                                          ││
│  │              = 79                                                                               ││
│  │                                                                                                  ││
│  │  Direction Check: Technical=LONG, LLM=LONG → ✅ AGREEMENT                                       ││
│  │  Agreement Bonus: +10                                                                           ││
│  │                                                                                                  ││
│  │  Final = 79 + 10 = 89 → Clamped: 87%                                                            ││
│  │                                                                                                  ││
│  │  ────────────────────────────────────────────────────────────────────────────────────────────   ││
│  │  VALIDATION CHECKS:                                                                             ││
│  │  ✅ Final Confidence (87%) >= Min Confidence (55%) for Swing mode                              ││
│  │  ✅ No disagreement blocking (both agree on LONG)                                               ││
│  │  ✅ Circuit breaker not triggered (5 trades today, limit 80)                                    ││
│  │  ✅ Capital available ($625 of $875 allocated to Swing)                                         ││
│  │  ✅ Position limit not reached (1 of 3 max Swing positions)                                     ││
│  │                                                                                                  ││
│  │  RESULT: ✅ TRADE EXECUTED                                                                      ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [View Raw LLM Response]  [View Market Snapshot]  [Export JSON]           [Report Issue]        ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 4: LLM & Adaptive AI Settings

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  🤖 LLM & ADAPTIVE AI SETTINGS                                                      [✕ Close]      │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  TABS:  [🤖 LLM Provider]  [⚖️ Mode Weights]  [🧠 Adaptive AI]  [📊 Performance]                ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│  🤖 LLM PROVIDER CONFIGURATION                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  PRIMARY PROVIDER                          │ │  FALLBACK PROVIDER                              ││
│  │                                            │ │                                                  ││
│  │  Provider:  [DeepSeek        ▼]            │ │  Provider:  [Claude          ▼]                 ││
│  │  Model:     [deepseek-chat   ▼]            │ │  Model:     [claude-3-haiku  ▼]                 ││
│  │  API Key:   [••••••••••••••••]  [Test]     │ │  API Key:   [••••••••••••••••]  [Test]          ││
│  │                                            │ │                                                  ││
│  │  Status:    🟢 Connected                   │ │  Status:    🟢 Connected                        ││
│  │  Avg Time:  1.2s                           │ │  Avg Time:  2.1s                                ││
│  │  Success:   98.5%                          │ │  Success:   99.2%                               ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  REQUEST SETTINGS                                                                               ││
│  │                                                                                                  ││
│  │  Timeout:          [5000   ] ms          Retry Count:      [2     ]                             ││
│  │  Cache Duration:   [300    ] sec         Max Tokens:       [500   ]                             ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│  ⚖️ MODE-SPECIFIC LLM WEIGHTS                                                                       │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │                                                                                                  ││
│  │  ┌────────────────────────────────────────────────────────────────────────────────────────────┐ ││
│  │  │ MODE         │ LLM ENABLED │ LLM WEIGHT │ SKIP TIMEOUT │ BLOCK DISAGREE │ MIN CONF │ CACHE │ ││
│  │  ├────────────────────────────────────────────────────────────────────────────────────────────┤ ││
│  │  │ ⚡ Ultra-Fast │    [✓]      │   [0.10]   │     [✓]      │      [ ]       │  [40]    │  [✓]  │ ││
│  │  │ 🏃 Scalp      │    [✓]      │   [0.20]   │     [✓]      │      [ ]       │  [50]    │  [✓]  │ ││
│  │  │ 🌊 Swing      │    [✓]      │   [0.40]   │     [ ]      │      [✓]       │  [60]    │  [ ]  │ ││
│  │  │ 🏔️ Position   │    [✓]      │   [0.50]   │     [ ]      │      [✓]       │  [65]    │  [ ]  │ ││
│  │  └────────────────────────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                                  ││
│  │  Legend:                                                                                         ││
│  │  • LLM Weight: 0.0 (ignore LLM) to 1.0 (full LLM, ignore technical)                             ││
│  │  • Skip Timeout: If LLM times out, proceed with technical only                                  ││
│  │  • Block Disagree: Skip trade if technical and LLM directions conflict                          ││
│  │  • Cache: Use cached LLM response for same symbol within cache duration                         ││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│  🧠 ADAPTIVE AI (SELF-LEARNING)                                                                     │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  LEARNING CONFIGURATION                    │ │  PENDING RECOMMENDATIONS                        ││
│  │                                            │ │                                                  ││
│  │  ☑️ Adaptive Learning Enabled              │ │  ┌────────────────────────────────────────────┐ ││
│  │                                            │ │  │  🤖 Adaptive AI has 2 recommendations      │ ││
│  │  Learning Trigger:                         │ │  │                                            │ ││
│  │  ○ Every [50   ] trades                    │ │  │  1. Reduce Ultra-Fast LLM weight           │ ││
│  │  ○ Every [24   ] hours                     │ │  │     Current: 0.10 → Suggested: 0.05        │ ││
│  │  ● Whichever comes first                   │ │  │     Reason: 45% win rate when LLM used     │ ││
│  │                                            │ │  │                                            │ ││
│  │  Min Trades for Learning: [20   ]          │ │  │  2. Increase Swing min confidence          │ ││
│  │                                            │ │  │     Current: 55 → Suggested: 65            │ ││
│  │  ─────────────────────────────────────     │ │  │     Reason: Low conf trades: 40% win rate  │ ││
│  │  AUTO-ADJUSTMENT                           │ │  │                                            │ ││
│  │  ─────────────────────────────────────     │ │  │  Expected Improvement: +4.2% win rate      │ ││
│  │                                            │ │  │                                            │ ││
│  │  ☐ Auto-apply adjustments                  │ │  │  [Apply All]  [Review Each]  [Dismiss]    │ ││
│  │  Max Auto Adjustment: [10   ] %            │ │  └────────────────────────────────────────────┘ ││
│  │  ☑️ Require user approval                  │ │                                                  ││
│  │                                            │ │  Last Analysis: 2h ago (42/50 trades)           ││
│  │  ─────────────────────────────────────     │ │  Next Analysis: ~8 trades or 22h               ││
│  │  DECISION STORAGE                          │ │                                                  ││
│  │  ─────────────────────────────────────     │ │  [Run Analysis Now]  [View History]            ││
│  │                                            │ │                                                  ││
│  │  ☑️ Store full decision context            │ │                                                  ││
│  │  ☑️ Store LLM reasoning                    │ │                                                  ││
│  │  ☑️ Store market snapshots                 │ │                                                  ││
│  │                                            │ │                                                  ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [Reset All to Defaults]                                              [Cancel]  [💾 Save All]  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 5: Sentiment Data Sources Configuration

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  📊 SENTIMENT DATA SOURCES                                                          [✕ Close]      │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  TABS:  [📈 Overview]  [📰 News Sources]  [💬 Social]  [🐋 On-Chain]  [⚙️ Weights]              ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│  📈 SENTIMENT OVERVIEW                                                                              │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │                                  AGGREGATED SENTIMENT                                            ││
│  │                                                                                                  ││
│  │     -100        -50         0         +45        +50        +100                                 ││
│  │       │          │          │          ●          │          │                                   ││
│  │  ◀────┼──────────┼──────────┼──────────┼──────────┼──────────┼────▶                             ││
│  │       │          │          │          │          │          │                                   ││
│  │    EXTREME    FEAR     NEUTRAL              GREED      EXTREME                                   ││
│  │     FEAR                                              GREED                                      ││
│  │                                                                                                  ││
│  │                        Current: +45 (Moderately Bullish)                                         ││
│  │                        Change 24h: ↑ +8 points                                                   ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  SOURCE BREAKDOWN                          │ │  SOURCE STATUS                                  ││
│  │                                            │ │                                                  ││
│  │  ┌──────────────────────────────────────┐  │ │  ┌────────────────────────────────────────────┐ ││
│  │  │ Fear & Greed (25%)   +50  ████████░░│  │ │  │ Source          │ Status │ Last Update     │ ││
│  │  │ LunarCrush (20%)     +60  █████████░│  │ │  ├────────────────────────────────────────────┤ ││
│  │  │ On-Chain (20%)       +40  ███████░░░│  │ │  │ Alternative.me  │  🟢    │ 5 min ago       │ ││
│  │  │ News (15%)           +30  █████░░░░░│  │ │  │ LunarCrush      │  🟢    │ 12 min ago      │ ││
│  │  │ Funding Rate (10%)    0   ░░░░░░░░░░│  │ │  │ CryptoPanic     │  🟢    │ 2 min ago       │ ││
│  │  │ Social Buzz (10%)    +20  ███░░░░░░░│  │ │  │ Glassnode       │  🟢    │ 8 min ago       │ ││
│  │  └──────────────────────────────────────┘  │ │  │ CryptoQuant     │  🟡    │ 45 min ago      │ ││
│  │                                            │ │  │ Santiment       │  🔴    │ API Error       │ ││
│  │  Weighted Sum: +39.0                       │ │  │ Coinglass       │  🟢    │ 1 min ago       │ ││
│  │  (rounded to +45 with momentum)            │ │  └────────────────────────────────────────────┘ ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│  📰 NEWS SOURCES                                                                                    │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │                                                                                                  ││
│  │  ┌────────────────────────────────────────────────────────────────────────────────────────────┐ ││
│  │  │ SOURCE           │ ENABLED │ API KEY           │ PRIORITY  │ STATUS │ HEADLINES/DAY       │ ││
│  │  ├────────────────────────────────────────────────────────────────────────────────────────────┤ ││
│  │  │ CryptoCompare    │  [✓]    │ [••••••••] [Edit] │ PRIMARY   │  🟢    │ ~150                │ ││
│  │  │ CryptoPanic      │  [✓]    │ [••••••••] [Edit] │ PRIMARY   │  🟢    │ ~200                │ ││
│  │  │ Messari          │  [✓]    │ [••••••••] [Edit] │ SECONDARY │  🟢    │ ~50                 │ ││
│  │  │ The Block (RSS)  │  [✓]    │ N/A               │ SECONDARY │  🟢    │ ~30                 │ ││
│  │  │ CoinDesk (RSS)   │  [✓]    │ N/A               │ SECONDARY │  🟢    │ ~80                 │ ││
│  │  │ Decrypt (RSS)    │  [ ]    │ N/A               │ FALLBACK  │  ⚪    │ Disabled            │ ││
│  │  └────────────────────────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                                  ││
│  │  News Settings:                                                                                  ││
│  │  Max Headlines per Symbol: [10   ]     Recency Window: [24   ] hours     Dedup Enabled: [✓]    ││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│  🐋 ON-CHAIN SOURCES                                                                                │
│  ══════════════════════════════════════════════════════════════════════════════════════════════════ │
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │                                                                                                  ││
│  │  ┌────────────────────────────────────────────────────────────────────────────────────────────┐ ││
│  │  │ SOURCE        │ ENABLED │ API KEY           │ DATA PROVIDED              │ STATUS         │ ││
│  │  ├────────────────────────────────────────────────────────────────────────────────────────────┤ ││
│  │  │ Glassnode     │  [✓]    │ [••••••••] [Edit] │ Whale alerts, metrics      │  🟢 Active    │ ││
│  │  │ CryptoQuant   │  [✓]    │ [••••••••] [Edit] │ Exchange flows, funding    │  🟡 Delayed   │ ││
│  │  │ WhaleAlert    │  [✓]    │ [••••••••] [Edit] │ Large transactions         │  🟢 Active    │ ││
│  │  │ Coinglass     │  [✓]    │ Free tier         │ Funding rates, liquidations│  🟢 Active    │ ││
│  │  │ DefiLlama     │  [ ]    │ Free              │ TVL changes                │  ⚪ Disabled  │ ││
│  │  └────────────────────────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                                  ││
│  │  On-Chain Settings:                                                                              ││
│  │  Whale Threshold: [$1,000,000]     Funding Contrarian Mode: [✓]     Exchange Flow Alert: [✓]   ││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [Test All Connections]  [Refresh Now]                            [Cancel]  [💾 Save Settings] ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 6: Trade History with AI Context

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  📜 TRADE HISTORY                                                      [Export CSV]  [✕ Close]     │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  FILTERS:  Date: [Last 7 Days ▼]  Mode: [All ▼]  Symbol: [All ▼]  Outcome: [All ▼]  [🔍 Search]││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  SUMMARY:  Total: 156 trades  |  Wins: 97 (62.2%)  |  P&L: +$487.50  |  Avg Hold: 2.4h         ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │                                                                                                  ││
│  │  ┌─────────────────────────────────────────────────────────────────────────────────────────────┐││
│  │  │ TIME       │ SYMBOL  │ MODE  │ SIDE │ ENTRY    │ EXIT     │ P&L      │ TECH │ LLM │ FINAL │ │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │ 10:05      │ BTCUSDT │ 🌊    │ LONG │ 98,500   │ 99,150   │ +$32.50  │ 75%  │ 85% │ 87%   │ │││
│  │  │ Today      │         │ Swing │      │          │          │ +0.66%   │ ✓    │ ✓   │ AGREE │ │││
│  │  │            │ [▼ Expand to see AI decision details]                                          │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │ 09:45      │ ETHUSDT │ 🏃    │ LONG │ 3,450    │ 3,478    │ +$14.00  │ 68%  │ 72% │ 73%   │ │││
│  │  │ Today      │         │ Scalp │      │          │          │ +0.81%   │ ✓    │ ✓   │ AGREE │ │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │ 09:30      │ SOLUSDT │ ⚡    │ SHRT │ 185.20   │ 186.10   │ -$4.50   │ 82%  │ 45% │ 71%   │ │││
│  │  │ Today      │         │ Ultra │      │          │          │ -0.49%   │ ✓    │ ✗   │DISAG. │ │││
│  │  │            │ [▼ Expand to see AI decision details]                                          │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │ 08:15      │ BTCUSDT │ 🏔️    │ LONG │ 97,800   │ 98,450   │ +$65.00  │ 70%  │ 80% │ 78%   │ │││
│  │  │ Today      │         │ Pos.  │      │          │          │ +0.66%   │ ✓    │ ✓   │ AGREE │ │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │ Yesterday  │ XRPUSDT │ 🏃    │ SHRT │ 2.35     │ 2.38     │ -$15.00  │ 55%  │ 48% │ 52%   │ │││
│  │  │ 23:45      │         │ Scalp │      │          │          │ -1.28%   │ ✗    │ ✗   │DISAG. │ │││
│  │  └─────────────────────────────────────────────────────────────────────────────────────────────┘││
│  │                                                                                                  ││
│  │  ┌─────────────────────────────────────────────────────────────────────────────────────────────┐││
│  │  │  ▼ EXPANDED: SOLUSDT Trade (09:30 Today)                                                    │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │                                                                                              │││
│  │  │  AI DECISION CONTEXT:                                                                        │││
│  │  │  ─────────────────────────────────────────────────────────────────────────────────────────   │││
│  │  │                                                                                              │││
│  │  │  Technical (82%): RSI oversold bounce, MACD bearish, EMA bearish, Volume spike              │││
│  │  │  LLM (45%): "Conflicting signals. Short-term bearish but news of Solana upgrade            │││
│  │  │             suggests medium-term bullish. Recommend caution on short positions."            │││
│  │  │                                                                                              │││
│  │  │  DISAGREEMENT: Technical=SHORT, LLM=HOLD                                                     │││
│  │  │  Block on Disagreement was DISABLED for Ultra-Fast mode → Trade executed anyway             │││
│  │  │                                                                                              │││
│  │  │  Sentiment at Entry:                                                                         │││
│  │  │  Fear & Greed: 62 | Social Buzz: 85 (Solana trending) | Whale: Accumulating                 │││
│  │  │                                                                                              │││
│  │  │  OUTCOME: LOSS (-0.49%)                                                                      │││
│  │  │  LESSON: Consider enabling "Block on Disagreement" for Ultra-Fast mode                       │││
│  │  │                                                                                              │││
│  │  │  [View Full Analysis]  [Report Issue]                                                        │││
│  │  └─────────────────────────────────────────────────────────────────────────────────────────────┘││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  PAGINATION:  [< Prev]  Page 1 of 16  (showing 1-10 of 156)  [Next >]    [Show: 10 ▼ per page] ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 7: Adaptive AI Recommendations Modal

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  🧠 ADAPTIVE AI RECOMMENDATIONS                                                     [✕ Close]      │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  ANALYSIS SUMMARY                                                                               ││
│  │                                                                                                  ││
│  │  Based on: 50 recent trades  |  Period: Last 24 hours  |  Analysis Time: 2 min ago             ││
│  │                                                                                                  ││
│  │  Current Performance:  Win Rate: 58%  |  P&L: +$124.50  |  Avg Trade: +$2.49                   ││
│  │  Projected After Changes:  Win Rate: 63% (+5%)  |  Est. P&L Improvement: +18%                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  RECOMMENDATIONS (3)                                                           [Select All ☐]  ││
│  ├─────────────────────────────────────────────────────────────────────────────────────────────────┤│
│  │                                                                                                  ││
│  │  ┌─────────────────────────────────────────────────────────────────────────────────────────────┐││
│  │  │  ☐  RECOMMENDATION 1: Reduce Ultra-Fast LLM Weight                         IMPACT: HIGH    │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │                                                                                              │││
│  │  │  Change:  LLM Weight for Ultra-Fast mode                                                     │││
│  │  │  Current: 0.10 (10%)                                                                         │││
│  │  │  Suggested: 0.05 (5%)                                                                        │││
│  │  │                                                                                              │││
│  │  │  Reasoning:                                                                                  │││
│  │  │  "Ultra-Fast trades with LLM disagreement show 45% win rate vs 68% when technical-only.     │││
│  │  │   LLM responses often arrive too late for ultra-fast timeframes. Reducing weight will       │││
│  │  │   prioritize faster technical signals while still considering LLM when available."          │││
│  │  │                                                                                              │││
│  │  │  Evidence:                                                                                   │││
│  │  │  • 12 Ultra-Fast trades with LLM: 5 wins (42%)                                               │││
│  │  │  • 8 Ultra-Fast trades technical-only: 6 wins (75%)                                          │││
│  │  │  • Average LLM response time: 1.8s (too slow for 3s max hold)                                │││
│  │  │                                                                                              │││
│  │  │  [Preview Impact]  [View Trades]                                   [Apply ✓]  [Dismiss ✗]   │││
│  │  └─────────────────────────────────────────────────────────────────────────────────────────────┘││
│  │                                                                                                  ││
│  │  ┌─────────────────────────────────────────────────────────────────────────────────────────────┐││
│  │  │  ☐  RECOMMENDATION 2: Increase Swing Mode Minimum Confidence               IMPACT: MEDIUM  │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │                                                                                              │││
│  │  │  Change:  Minimum Confidence for Swing mode                                                  │││
│  │  │  Current: 55%                                                                                │││
│  │  │  Suggested: 65%                                                                              │││
│  │  │                                                                                              │││
│  │  │  Reasoning:                                                                                  │││
│  │  │  "Swing trades with confidence 55-64% show only 40% win rate. Trades with 65%+ confidence   │││
│  │  │   show 72% win rate. Raising the threshold will filter out lower-quality setups."           │││
│  │  │                                                                                              │││
│  │  │  Evidence:                                                                                   │││
│  │  │  • Confidence 55-64%: 10 trades, 4 wins (40%), P&L: -$28                                     │││
│  │  │  • Confidence 65-74%: 8 trades, 5 wins (63%), P&L: +$45                                      │││
│  │  │  • Confidence 75%+: 6 trades, 5 wins (83%), P&L: +$89                                        │││
│  │  │                                                                                              │││
│  │  │  [Preview Impact]  [View Trades]                                   [Apply ✓]  [Dismiss ✗]   │││
│  │  └─────────────────────────────────────────────────────────────────────────────────────────────┘││
│  │                                                                                                  ││
│  │  ┌─────────────────────────────────────────────────────────────────────────────────────────────┐││
│  │  │  ☐  RECOMMENDATION 3: Enable Block on Disagreement for Scalp               IMPACT: MEDIUM  │││
│  │  ├─────────────────────────────────────────────────────────────────────────────────────────────┤││
│  │  │                                                                                              │││
│  │  │  Change:  Block on Disagreement for Scalp mode                                               │││
│  │  │  Current: Disabled (false)                                                                   │││
│  │  │  Suggested: Enabled (true)                                                                   │││
│  │  │                                                                                              │││
│  │  │  Reasoning:                                                                                  │││
│  │  │  "Scalp trades executed despite Technical/LLM disagreement show 33% win rate. These trades  │││
│  │  │   should be skipped. Enabling this filter would have avoided 4 losing trades."              │││
│  │  │                                                                                              │││
│  │  │  [Preview Impact]  [View Trades]                                   [Apply ✓]  [Dismiss ✗]   │││
│  │  └─────────────────────────────────────────────────────────────────────────────────────────────┘││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [Dismiss All]                               [Apply Selected (0)]  [Apply All Recommendations] ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 8: Full Sentiment Dashboard

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  📊 MARKET SENTIMENT DASHBOARD                                [Auto-Refresh: ON]   [✕ Close]       │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │                                 MARKET SENTIMENT GAUGE                                          ││
│  │                                                                                                  ││
│  │                              ╭─────────────────────╮                                             ││
│  │                           ╭──╯                     ╰──╮                                          ││
│  │                        ╭──╯                           ╰──╮                                       ││
│  │                     ╭──╯         GREED: +45              ╰──╮                                    ││
│  │                   ╭─╯               ▲                       ╰─╮                                  ││
│  │                  ╱                  │                          ╲                                 ││
│  │                ╱   FEAR           ──┼──           GREED          ╲                               ││
│  │               ╱                     │                              ╲                             ││
│  │              ╱  -100       -50      0       +50       +100          ╲                            ││
│  │              ────────────────────────────────────────────────────────                            ││
│  │                                                                                                  ││
│  │                        Status: MODERATELY BULLISH   ↑ +8 from yesterday                         ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌──────────────────────┐ ┌──────────────────────┐ ┌──────────────────────┐ ┌──────────────────────┐│
│  │  😨 FEAR & GREED     │ │  💬 SOCIAL BUZZ      │ │  🐋 WHALE ACTIVITY   │ │  📈 FUNDING RATE     ││
│  │                      │ │                      │ │                      │ │                      ││
│  │       ╭───╮          │ │       ╭───╮          │ │                      │ │       ╭───╮          ││
│  │      ╱ 65 ╲          │ │      ╱ 78 ╲          │ │    ACCUMULATING      │ │      ╱0.01╲          ││
│  │     ╱     ╲          │ │     ╱     ╲          │ │        🐋 ↑          │ │     ╱  %  ╲          ││
│  │     ╲ GREED ╱        │ │     ╲ HIGH ╱         │ │                      │ │     ╲NEUTRAL╱        ││
│  │      ╲     ╱         │ │      ╲    ╱          │ │   45 large moves     │ │      ╲     ╱         ││
│  │       ╰───╯          │ │       ╰───╯          │ │      (24h)           │ │       ╰───╯          ││
│  │                      │ │                      │ │                      │ │                      ││
│  │  ↑ +5 (24h)          │ │  🔥 BTC trending     │ │  Net: +2,500 BTC     │ │  Slightly long-heavy ││
│  └──────────────────────┘ └──────────────────────┘ └──────────────────────┘ └──────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────┐ ┌────────────────────────────────────────────────┐ │
│  │  📰 LATEST NEWS                   [View All]│ │  🐋 ON-CHAIN SIGNALS                          │ │
│  ├─────────────────────────────────────────────┤ ├────────────────────────────────────────────────┤ │
│  │                                             │ │                                                │ │
│  │  🔴 HIGH IMPACT                             │ │  EXCHANGE FLOWS (24h)                          │ │
│  │  ───────────────────────────────────────    │ │  ┌────────────────────────────────────────┐   │ │
│  │  • Bitcoin ETF sees $500M inflows   2h ago  │ │  │ BTC:  -2,500 ████████░░░░  OUTFLOW 🟢  │   │ │
│  │    Source: CoinDesk         Sentiment: 🟢   │ │  │ ETH:  -1,200 ██████░░░░░░  OUTFLOW 🟢  │   │ │
│  │                                             │ │  │ SOL:  +500   ░░░░░░███░░░  INFLOW  🟡  │   │ │
│  │  • Fed signals potential rate pause 5h ago  │ │  └────────────────────────────────────────┘   │ │
│  │    Source: Reuters          Sentiment: 🟢   │ │                                                │ │
│  │                                             │ │  WHALE TRANSACTIONS                            │ │
│  │  🟠 MEDIUM IMPACT                           │ │  ┌────────────────────────────────────────┐   │ │
│  │  ───────────────────────────────────────    │ │  │ • 1,000 BTC moved to cold wallet  1h   │   │ │
│  │  • Whale moves 5000 BTC to storage  1h ago  │ │  │ • 500 BTC to Binance (sell?)      2h   │   │ │
│  │    Source: WhaleAlert       Sentiment: 🟢   │ │  │ • 2,000 ETH to cold wallet        3h   │   │ │
│  │                                             │ │  │ • 10,000 SOL from exchange        4h   │   │ │
│  │  • Ethereum upgrade scheduled Q1    8h ago  │ │  └────────────────────────────────────────┘   │ │
│  │    Source: CryptoCompare    Sentiment: 🟡   │ │                                                │ │
│  │                                             │ │  STABLECOIN FLOWS                              │ │
│  │  🟢 LOW IMPACT                              │ │  USDT to exchanges: +$150M (buying power) 🟢  │ │
│  │  ───────────────────────────────────────    │ │  USDC to exchanges: +$50M                  🟢  │ │
│  │  • Analyst predicts $150k by 2026  12h ago  │ │                                                │ │
│  │    Source: Twitter          Sentiment: 🟢   │ │  ACTIVE ADDRESSES                              │ │
│  │                                             │ │  BTC: +3.2% vs 7d avg 🟢                       │ │
│  │                                             │ │  ETH: +1.8% vs 7d avg 🟢                       │ │
│  └─────────────────────────────────────────────┘ └────────────────────────────────────────────────┘ │
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  SENTIMENT HISTORY (7 DAYS)                                                                     ││
│  │                                                                                                  ││
│  │   +100 ┤                                                                                         ││
│  │    +50 ┤                    ╭──╮        ╭────────────╮                                           ││
│  │      0 ┤────────╮  ╭───────╯  ╰───╮╭───╯            ╰───●  Current: +45                         ││
│  │    -50 ┤        ╰──╯              ╰╯                                                             ││
│  │   -100 ┤                                                                                         ││
│  │        └──────┬──────┬──────┬──────┬──────┬──────┬──────┬                                        ││
│  │              Mon    Tue    Wed    Thu    Fri    Sat    Sun                                       ││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [⚙️ Configure Sources]  [🔄 Refresh Now]  [📊 View Analytics]            Last Update: 2 min ago││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 9: ROI Target Editor (Early Profit Booking)

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  🎯 SET ROI TARGET - BTCUSDT                                                       [✕ Close]       │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  POSITION DETAILS                                                                               ││
│  │                                                                                                  ││
│  │  Symbol: BTCUSDT        Mode: 🌊 Swing         Side: LONG         Leverage: 10x                ││
│  │  Entry:  $98,500        Current: $99,150       P&L: +$32.50 (+0.66%)                            ││
│  │                                                                                                  ││
│  │  ════════════════════════════════════════════════════════════════════════════════════════════   ││
│  │  CURRENT ROI (after fees): 3.3%                                            Progress: ████░░░░   ││
│  │  ════════════════════════════════════════════════════════════════════════════════════════════   ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌────────────────────────────────────────────┐ ┌──────────────────────────────────────────────────┐│
│  │  🎯 ROI TARGET SETTINGS                    │ │  📊 ROI CALCULATION PREVIEW                     ││
│  │                                            │ │                                                  ││
│  │  ─────────────────────────────────────     │ │  Entry Price:     $98,500                       ││
│  │  TARGET TYPE                               │ │  Target ROI:      5.0%                          ││
│  │  ─────────────────────────────────────     │ │  Leverage:        10x                           ││
│  │                                            │ │                                                  ││
│  │  ○ Use Mode Defaults                       │ │  ─────────────────────────────────────────────  ││
│  │    (Swing: TP% × leverage = 5% × 10 = 50%) │ │  Price Move Needed: +0.50% ($98,992.50)         ││
│  │                                            │ │  Est. Exit Price:  $98,992.50                   ││
│  │  ● Custom ROI Target                       │ │  Est. Gross P&L:   +$49.25                      ││
│  │    ┌────────────────────────────────────┐  │ │  Est. Fees:        -$1.98                       ││
│  │    │  [5.0    ] %                       │  │ │  Est. Net P&L:     +$47.27                      ││
│  │    └────────────────────────────────────┘  │ │                                                  ││
│  │                                            │ │  ─────────────────────────────────────────────  ││
│  │  ─────────────────────────────────────     │ │  Current ROI:      3.3%                         ││
│  │  QUICK PRESETS                             │ │  Target ROI:       5.0%                         ││
│  │  ─────────────────────────────────────     │ │  Remaining:        1.7%                         ││
│  │                                            │ │                                                  ││
│  │  [2%] [3%] [5%] [8%] [10%] [15%] [20%]     │ │  ◀──────────────●────────────────────▶          ││
│  │                                            │ │  0%     3.3%    5%              20%             ││
│  │  ─────────────────────────────────────     │ │                  ↑                              ││
│  │  CLOSE ORDER TYPE                          │ │               TARGET                            ││
│  │  ─────────────────────────────────────     │ │                                                  ││
│  │                                            │ │  ─────────────────────────────────────────────  ││
│  │  ● Market Order (instant close)            │ │  EXECUTION PREVIEW                              ││
│  │  ○ Limit Order  (at target price)          │ │  ─────────────────────────────────────────────  ││
│  │                                            │ │                                                  ││
│  │  If Limit:                                 │ │  Status: 🟡 1.7% away from target               ││
│  │  Limit Price: [$99,000]  or Offset: [0.1]% │ │                                                  ││
│  │                                            │ │  When ROI reaches 5.0%:                         ││
│  │  ☑️ Reduce Only (safety - prevents flip)   │ │  → Close via: MARKET ORDER                     ││
│  │                                            │ │  → Quantity: 100% of position                   ││
│  │  ─────────────────────────────────────     │ │  → Reduce Only: ✓ Enabled                       ││
│  │  PERSISTENCE OPTIONS                       │ │                                                  ││
│  │  ─────────────────────────────────────     │ │                                                  ││
│  │                                            │ │                                                  ││
│  │  ☐ Save for this position only             │ │                                                  ││
│  │  ☑️ Save for future BTCUSDT trades          │ │                                                  ││
│  │                                            │ │                                                  ││
│  └────────────────────────────────────────────┘ └──────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  ⚠️ EARLY PROFIT BOOKING INFO                                                                   ││
│  │                                                                                                  ││
│  │  • ROI is calculated AFTER trading fees (entry + exit)                                          ││
│  │  • For leveraged positions: ROI = (Price Move % × Leverage) - Fees                              ││
│  │  • Example: 0.5% price move × 10x leverage = 5% ROI (before fees)                               ││
│  │  • Position will close via MARKET ORDER when target ROI is reached                              ││
│  │  • This overrides multi-level TP orders for immediate profit capture                            ││
│  │                                                                                                  ││
│  │  Priority Order:                                                                                 ││
│  │  1. Position-level custom ROI (this setting)                                                    ││
│  │  2. Symbol-level saved ROI (if "Save for future" enabled)                                       ││
│  │  3. Mode-based defaults (TP% × leverage)                                                        ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [Clear ROI Target]                                             [Cancel]  [💾 Set ROI Target]  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 10: Position Card with ROI Target (Ginie Panel)

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  📋 GINIE ACTIVE POSITIONS                                              [Collapse All] [Refresh]   │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  ▼ BTCUSDT                                                                                      ││
│  │  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━   ││
│  │                                                                                                  ││
│  │  ┌──────────────────────────────────────────────────────────────────────────────────────────┐   ││
│  │  │  🌊 SWING  │  LONG  │  10x  │  Entry: $98,500  │  Current: $99,150  │  +$32.50 (+0.66%)  │   ││
│  │  └──────────────────────────────────────────────────────────────────────────────────────────┘   ││
│  │                                                                                                  ││
│  │  ┌────────────────────────┐ ┌────────────────────────┐ ┌────────────────────────────────────┐   ││
│  │  │  💰 P&L               │ │  📊 CURRENT ROI        │ │  🎯 ROI TARGET                     │   ││
│  │  │                        │ │                        │ │                                    │   ││
│  │  │  Unrealized: +$32.50   │ │  After Fees: 3.3%      │ │  Target: 5.0%  🎯                  │   ││
│  │  │  ROI %: +3.3%          │ │                        │ │  Source: Custom                    │   ││
│  │  │                        │ │  ████████░░░░░░ 66%    │ │  Remaining: 1.7%                   │   ││
│  │  │  Realized: $0          │ │  (of 5% target)        │ │                                    │   ││
│  │  │                        │ │                        │ │  [✏️ Edit]  [✕ Clear]              │   ││
│  │  └────────────────────────┘ └────────────────────────┘ └────────────────────────────────────┘   ││
│  │                                                                                                  ││
│  │  ┌────────────────────────┐ ┌────────────────────────┐ ┌────────────────────────────────────┐   ││
│  │  │  🛡️ STOP LOSS         │ │  🎯 TAKE PROFITS       │ │  📈 AI ANALYSIS                    │   ││
│  │  │                        │ │                        │ │                                    │   ││
│  │  │  Price: $96,000        │ │  TP1: $99,500 (25%)    │ │  Tech: 75%  LLM: 85%               │   ││
│  │  │  Distance: -2.5%       │ │  TP2: $100,500 (25%)   │ │  Fusion: 87%                       │   ││
│  │  │  Status: 🟢 Active     │ │  TP3: $101,500 (25%)   │ │  Direction: AGREE ✓                │   ││
│  │  │                        │ │  TP4: $102,500 (25%)   │ │                                    │   ││
│  │  │  [✏️ Edit SL]          │ │  [✏️ Edit TPs]         │ │  [View Full Analysis]              │   ││
│  │  └────────────────────────┘ └────────────────────────┘ └────────────────────────────────────┘   ││
│  │                                                                                                  ││
│  │  ┌──────────────────────────────────────────────────────────────────────────────────────────┐   ││
│  │  │  ⚡ EARLY PROFIT STATUS                                                                  │   ││
│  │  │                                                                                          │   ││
│  │  │  ROI Target: 5.0%  |  Current ROI: 3.3%  |  Remaining: 1.7%  |  Status: 🟡 MONITORING   │   ││
│  │  │                                                                                          │   ││
│  │  │  Progress: ◀═══════════════════════════════●══════════════▶                              │   ││
│  │  │            0%                              3.3%          5%                              │   ││
│  │  │                                                                                          │   ││
│  │  │  When ROI reaches 5.0% → Auto-close position via market order                            │   ││
│  │  └──────────────────────────────────────────────────────────────────────────────────────────┘   ││
│  │                                                                                                  ││
│  │  ┌──────────────────────────────────────────────────────────────────────────────────────────┐   ││
│  │  │  [🔴 Close Position]  [⏸️ Pause Monitoring]  [📊 View Chart]  [📜 Trade History]          │   ││
│  │  └──────────────────────────────────────────────────────────────────────────────────────────┘   ││
│  │                                                                                                  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  ▶ ETHUSDT   🏃 SCALP  LONG  +$14.00 (+4.1% ROI)   Target: — (using mode default: 30%)        ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  ▶ SOLUSDT   ⚡ ULTRA  SHORT  +$3.80 (+1.9% ROI)   Target: 4.3% 🎯 (custom)  [2.4% remaining]  ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Wireframe 11: Mode-Based ROI Defaults Configuration

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  ⚙️ EARLY PROFIT BOOKING SETTINGS                                                  [✕ Close]       │
├─────────────────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  GLOBAL SETTINGS                                                                                ││
│  │                                                                                                  ││
│  │  ☑️ Enable Early Profit Booking                                                                 ││
│  │                                                                                                  ││
│  │  When enabled, positions will be closed automatically when ROI (after fees) reaches the        ││
│  │  target threshold. This provides an alternative to waiting for multi-level TP orders.          ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  MODE-SPECIFIC DEFAULTS                                                                         ││
│  │                                                                                                  ││
│  │  These TP% values are multiplied by leverage to calculate the ROI threshold.                   ││
│  │  Example: 2% TP × 10x leverage = 20% ROI target                                                ││
│  │                                                                                                  ││
│  │  ┌────────────────────────────────────────────────────────────────────────────────────────────┐ ││
│  │  │ MODE           │ TP %   │ × LEVERAGE │ = ROI THRESHOLD │ DESCRIPTION                       │ ││
│  │  ├────────────────────────────────────────────────────────────────────────────────────────────┤ ││
│  │  │ ⚡ Ultra-Fast   │ [2.0 ]%│ × 10x      │ = 20% ROI       │ Quick scalps, fast exits          │ ││
│  │  │ 🏃 Scalp        │ [3.0 ]%│ × 10x      │ = 30% ROI       │ Short-term momentum               │ ││
│  │  │ 🌊 Swing        │ [5.0 ]%│ × 10x      │ = 50% ROI       │ Multi-hour trends                 │ ││
│  │  │ 🏔️ Position     │ [8.0 ]%│ × 10x      │ = 80% ROI       │ Multi-day positions               │ ││
│  │  └────────────────────────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                                  ││
│  │  Note: Leverage shown is example (10x). Actual ROI threshold = TP% × position leverage         ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  SYMBOL-SPECIFIC OVERRIDES                                                                      ││
│  │                                                                                                  ││
│  │  These override mode defaults for specific symbols (saved via "Save for future" option)        ││
│  │                                                                                                  ││
│  │  ┌────────────────────────────────────────────────────────────────────────────────────────────┐ ││
│  │  │ SYMBOL    │ CUSTOM ROI % │ OVERRIDES MODE │ ACTIONS                                        │ ││
│  │  ├────────────────────────────────────────────────────────────────────────────────────────────┤ ││
│  │  │ BTCUSDT   │ 5.0%         │ All modes      │ [✏️ Edit]  [🗑️ Remove]                         │ ││
│  │  │ SOLUSDT   │ 4.3%         │ All modes      │ [✏️ Edit]  [🗑️ Remove]                         │ ││
│  │  │ AVAXUSDT  │ 6.0%         │ All modes      │ [✏️ Edit]  [🗑️ Remove]                         │ ││
│  │  └────────────────────────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                                                  ││
│  │  [+ Add Symbol Override]                                                                        ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  ROI CALCULATION FORMULA                                                                        ││
│  │                                                                                                  ││
│  │  ┌──────────────────────────────────────────────────────────────────────────────────────────┐   ││
│  │  │                                                                                          │   ││
│  │  │  ROI% = ((Net P&L × Leverage) / Notional Value) × 100                                    │   ││
│  │  │                                                                                          │   ││
│  │  │  Where:                                                                                  │   ││
│  │  │  • Net P&L = Gross P&L - Entry Fee - Exit Fee                                            │   ││
│  │  │  • Entry Fee = 0.02% of notional (maker) or 0.05% (taker)                                │   ││
│  │  │  • Exit Fee = 0.05% of notional (taker, market order)                                    │   ││
│  │  │  • Notional = Quantity × Entry Price                                                     │   ││
│  │  │                                                                                          │   ││
│  │  │  Example (10x Long BTCUSDT):                                                             │   ││
│  │  │  Entry: $100,000 | Exit: $100,500 | Qty: 0.1 BTC | Notional: $10,000                     │   ││
│  │  │  Gross P&L: ($100,500 - $100,000) × 0.1 = $50                                            │   ││
│  │  │  Fees: (0.02% + 0.05%) × $10,000 = $7                                                    │   ││
│  │  │  Net P&L: $50 - $7 = $43                                                                 │   ││
│  │  │  ROI: ($43 × 10 / $10,000) × 100 = 4.3%                                                  │   ││
│  │  │                                                                                          │   ││
│  │  └──────────────────────────────────────────────────────────────────────────────────────────┘   ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────────────────────┐│
│  │  [Reset to Defaults]                                                    [Cancel]  [💾 Save]    ││
│  └─────────────────────────────────────────────────────────────────────────────────────────────────┘│
│                                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

*Last Updated: 2025-12-26 - Added ROI Target wireframes (Wireframes 9, 10, 11)*

---

*Epic created by BMAD Party Mode - Bob (SM), Mary (Analyst), Winston (Architect), John (PM)*
*Date: 2025-12-26*
*Last Updated: 2025-12-26 - Added UI Wireframes for Story 2.8*
