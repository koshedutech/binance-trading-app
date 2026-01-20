# Story 10.3: Exit Decision Monitoring UI

**Epic:** 10 - Position Management & Optimization
**Priority:** P1 (High)
**Status:** done
**Created:** 2026-01-20
**Completed:** 2026-01-20

---

## User Story

**As a** trader with open positions managed by Ginie Autopilot,
**I want** a real-time Exit Decision Monitoring panel that shows what exit conditions are being checked, their current values vs thresholds, and the system's decision,
**So that** I can understand why the system is holding or exiting a position, monitor the exit logic in real-time, and have confidence in the automated exit decisions.

---

## Context & Background

### Current State (Story 10.1 Delivered):
- Position stage tracking (RISK_ZONE → BREAKEVEN → TP1 → EFFICIENCY)
- `PositionCardExpanded` component exists but shows mostly static/entry data
- Backend has exit decision logic in `position_decision.go` but doesn't expose real-time state
- Classic and New Engine exit modes are implemented but not visible to user

### Gap Identified:
- No UI showing **why** the system would exit or hold
- No real-time display of indicator values being checked
- No visibility into exit condition status (passed/failed/pending)
- Users cannot monitor the exit decision flow

### This Story Delivers:
- Real-time exit decision state exposed via API
- New Exit Decision Monitor UI component
- Live updates via WebSocket
- Integration into PositionCardExpanded or Positions tab

---

## Acceptance Criteria

### AC1: Exit Decision State API Endpoint
- [x] New endpoint `GET /api/futures/positions/:symbol/exit-decision` returns current exit decision state
- [x] Response includes:
  - `decision_mode`: "classic" | "new_engine"
  - `hold_safeguards`: min_hold_met, breakeven_achieved, consecutive_signals_count
  - `exit_checks`: array of exit conditions with name, status, current_value, threshold, details
  - `overall_decision`: "HOLD" | "EXIT"
  - `decision_reason`: human-readable explanation
  - `last_check_time`: timestamp of last evaluation
- [x] Endpoint returns 404 if position not found
- [x] Endpoint returns data even when position is in RISK_ZONE stage
- **Test:** Call API with valid position symbol, verify all fields present and accurate

### AC2: Exit Checks Data Structure
- [x] Each exit check includes:
  - `name`: "TREND_REVERSAL" | "EFFICIENCY_EXIT" | "TRAILING_SL" | "DYNAMIC_TP"
  - `priority`: 1-4 (P1 highest)
  - `status`: "PASSED" | "FAILED" | "PENDING" | "DISABLED"
  - `current_value`: current indicator/metric value
  - `threshold`: configured threshold
  - `details`: sub-checks with individual status
- [x] Trend Reversal check includes: ADX, RSI, EMA cross, reversal signal count
- [x] Efficiency check includes: peak_profit, current_profit, efficiency_percent, threshold_percent
- [x] Trailing SL shows: active status, current SL price, trigger distance
- [x] Dynamic TP shows: active status, current TP price, progress to target
- **Test:** Verify exit checks array contains all 4 priority levels with correct structure

### AC3: Classic Mode Indicator Display
- [x] When decision_mode is "classic", show:
  - ADX value with threshold (e.g., "24.5 / 20")
  - ADX trend direction (increasing/decreasing)
  - RSI value with state (oversold/normal/overbought)
  - EMA cross status (bullish/bearish/none)
  - Reversal signals count (e.g., "1/2 required")
  - MACD histogram direction
- [x] Visual indicators: green checkmark for passing, yellow warning for approaching, red for triggered
- **Test:** Open position with Classic mode, verify all indicator values update in real-time

### AC4: New Engine Mode Display
- [x] When decision_mode is "new_engine", show:
  - Strategy-based exit status
  - Regime change detection (current vs entry regime)
  - Technical score trend
  - Exit signal strength (0-100)
- [x] Show which strategy rules are being evaluated
- [x] Display regime with color coding (TRENDING=blue, RANGING=yellow, VOLATILE=red)
- **Test:** Open position with New Engine mode, verify strategy/regime info displays correctly

### AC5: Hold Safeguards Section
- [x] Display minimum hold time: elapsed vs required (e.g., "3:42 / 5:00")
- [x] Progress bar for hold time
- [x] Breakeven achievement status with timestamp
- [x] Consecutive exit signals counter (e.g., "1/2 required before exit")
- [x] Visual indication when safeguard is satisfied (checkmark) or pending (clock)
- **Test:** New position shows safeguards as pending, verify they update as conditions are met

### AC6: Exit Decision Monitor UI Component
- [x] New component `ExitDecisionMonitor.tsx` in `web/src/components/`
- [x] Displays all exit checks in priority order (P1 at top)
- [x] Color-coded status: green=safe, yellow=warning, red=would exit
- [x] Collapsible sections for detailed view
- [x] Shows "Last checked: Xs ago" with auto-refresh indicator
- [x] Responsive design fits in GiniePanel width
- **Test:** Component renders without errors, all sections expandable

### AC7: Integration into PositionCardExpanded
- [x] ExitDecisionMonitor appears as a new section in PositionCardExpanded
- [x] Section is collapsible with header "Exit Decision Monitor"
- [x] Loads data when position card is expanded
- [x] Shows loading skeleton while fetching
- [x] Refresh button to manually reload data
- **Test:** Expand position card, verify Exit Decision Monitor section appears and loads

### AC8: WebSocket Real-Time Updates
- [x] New WebSocket event type: `exit_decision_update`
- [x] Backend publishes updates when exit decision state changes significantly
- [x] Frontend subscribes when ExitDecisionMonitor is visible
- [x] Updates without full page refresh
- [x] Handles WebSocket disconnection gracefully (shows "Reconnecting...")
- **Test:** Change position price, verify UI updates within 2-3 seconds via WebSocket

### AC9: Overall Decision Display
- [x] Large, clear display of current decision: "HOLD" or "EXIT TRIGGERED"
- [x] Decision reason in plain text (e.g., "Holding: All safeguards passed, no exit signals")
- [x] If EXIT, show which condition triggered it
- [x] Color coding: green for HOLD (safe), red for EXIT (action needed)
- [x] Timestamp of decision
- **Test:** Verify decision display updates correctly based on exit checks

### AC10: Backend Exit Decision State Exposure
- [x] Modify `GinieAutopilot` to track and expose current exit decision state
- [x] State updated on each monitoring loop iteration
- [x] State includes all fields needed by API endpoint
- [x] Thread-safe access to exit decision state
- [x] Minimal performance impact on monitoring loop
- **Test:** Verify monitoring loop performance not degraded (< 5% overhead)

---

## Technical Design

### API Response Structure

```json
{
  "success": true,
  "exit_decision": {
    "symbol": "BTCUSDT",
    "decision_mode": "classic",
    "overall_decision": "HOLD",
    "decision_reason": "All safeguards passed, no exit signals detected",
    "last_check_time": 1705729200000,

    "hold_safeguards": {
      "min_hold_time": {
        "required_seconds": 300,
        "elapsed_seconds": 342,
        "met": true
      },
      "breakeven": {
        "achieved": true,
        "achieved_at": 1705728900000,
        "be_price": 94250.50
      },
      "consecutive_signals": {
        "required": 2,
        "current": 0
      }
    },

    "exit_checks": [
      {
        "name": "TREND_REVERSAL",
        "priority": 1,
        "status": "PASSED",
        "would_exit": false,
        "details": {
          "adx": {
            "value": 24.5,
            "threshold": 20,
            "status": "strong_trend",
            "direction": "increasing"
          },
          "rsi": {
            "value": 58.2,
            "state": "normal",
            "overbought": 70,
            "oversold": 30
          },
          "ema_cross": {
            "detected": false,
            "type": null
          },
          "reversal_signals": {
            "count": 0,
            "required": 2
          }
        }
      },
      {
        "name": "EFFICIENCY_EXIT",
        "priority": 2,
        "status": "PASSED",
        "would_exit": false,
        "details": {
          "peak_profit_percent": 2.45,
          "current_profit_percent": 1.82,
          "efficiency_percent": 74.3,
          "threshold_percent": 50.0
        }
      },
      {
        "name": "TRAILING_SL",
        "priority": 3,
        "status": "ACTIVE",
        "would_exit": false,
        "details": {
          "active": true,
          "sl_price": 94100.00,
          "current_price": 94850.00,
          "distance_percent": 0.79
        }
      },
      {
        "name": "DYNAMIC_TP",
        "priority": 4,
        "status": "PENDING",
        "would_exit": false,
        "details": {
          "active": true,
          "tp_price": 96500.00,
          "current_price": 94850.00,
          "progress_percent": 45.2
        }
      }
    ]
  }
}
```

### Files to Create

| File | Purpose |
|------|---------|
| `web/src/components/ExitDecisionMonitor.tsx` | Main UI component |
| `web/src/types/exitDecision.ts` | TypeScript type definitions |
| `internal/api/handlers_exit_decision.go` | API handler for exit decision state |
| `internal/autopilot/exit_decision_state.go` | Exit decision state tracking |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/api/server.go` | Add new route |
| `internal/autopilot/ginie_autopilot.go` | Expose exit decision state |
| `internal/autopilot/position_decision.go` | Track state during evaluation |
| `web/src/components/PositionCardExpanded.tsx` | Integrate ExitDecisionMonitor |
| `web/src/services/api.ts` | Add API client method |
| `web/src/services/websocket.ts` | Handle new event type |

### WebSocket Event Structure

```json
{
  "type": "exit_decision_update",
  "payload": {
    "symbol": "BTCUSDT",
    "user_id": "uuid",
    "overall_decision": "HOLD",
    "decision_reason": "...",
    "last_check_time": 1705729200000,
    "changed_checks": ["EFFICIENCY_EXIT"]
  }
}
```

---

## Implementation Tasks

### Task 1: Exit Decision State Types (Backend)
**Estimate:** 1 hour

- [x] 1.1 Create `internal/autopilot/exit_decision_state.go`
- [x] 1.2 Define `ExitDecisionState` struct with all fields
- [x] 1.3 Define `ExitCheck` struct for individual checks
- [x] 1.4 Define `HoldSafeguards` struct
- [x] 1.5 Add constructor and helper methods

### Task 2: Track Exit Decision State in Monitoring Loop
**Estimate:** 3 hours

- [x] 2.1 Add `currentExitState` field to `GiniePosition`
- [x] 2.2 Modify `EvaluateExitPriorities()` to populate state
- [x] 2.3 Track each check's result in state
- [x] 2.4 Update state on each monitoring iteration
- [x] 2.5 Ensure thread-safe access with mutex
- [x] 2.6 Add method `GetExitDecisionState(symbol)` to autopilot

### Task 3: Exit Decision API Endpoint
**Estimate:** 2 hours

- [x] 3.1 Create `internal/api/handlers_exit_decision.go`
- [x] 3.2 Implement `handleGetExitDecision()` handler
- [x] 3.3 Register route in `server.go`
- [x] 3.4 Map internal state to API response format
- [x] 3.5 Handle position not found case
- [x] 3.6 Add proper error responses

### Task 4: TypeScript Types for Exit Decision
**Estimate:** 1 hour

- [x] 4.1 Create `web/src/types/exitDecision.ts`
- [x] 4.2 Define `ExitDecisionResponse` interface
- [x] 4.3 Define `ExitCheck` interface with all detail types
- [x] 4.4 Define `HoldSafeguards` interface
- [x] 4.5 Add display helper constants (colors, labels)

### Task 5: API Client Method
**Estimate:** 30 minutes

- [x] 5.1 Add `getExitDecisionState(symbol)` to `api.ts`
- [x] 5.2 Add proper TypeScript return type
- [x] 5.3 Handle error cases

### Task 6: Exit Decision Monitor Component
**Estimate:** 4 hours

- [x] 6.1 Create `ExitDecisionMonitor.tsx`
- [x] 6.2 Implement hold safeguards section with progress bars
- [x] 6.3 Implement exit checks list with priority ordering
- [x] 6.4 Create collapsible detail sections for each check
- [x] 6.5 Add overall decision display with color coding
- [x] 6.6 Implement loading and error states
- [x] 6.7 Add refresh button functionality
- [x] 6.8 Style with Tailwind for dark theme

### Task 7: Classic Mode Indicators Display
**Estimate:** 2 hours

- [x] 7.1 Create `ClassicIndicatorsPanel.tsx` sub-component (embedded in ExitDecisionMonitor)
- [x] 7.2 Display ADX with threshold bar
- [x] 7.3 Display RSI with zone indicator
- [x] 7.4 Display EMA cross status
- [x] 7.5 Display reversal signals counter
- [x] 7.6 Add tooltips explaining each indicator

### Task 8: New Engine Mode Display
**Estimate:** 2 hours

- [x] 8.1 Create `NewEnginePanel.tsx` sub-component (embedded in ExitDecisionMonitor)
- [x] 8.2 Display strategy exit rules status
- [x] 8.3 Display regime change detection
- [x] 8.4 Display technical score trend
- [x] 8.5 Display exit signal strength gauge

### Task 9: Integration into PositionCardExpanded
**Estimate:** 2 hours

- [x] 9.1 Import ExitDecisionMonitor component
- [x] 9.2 Add as collapsible section after efficiency metrics
- [x] 9.3 Fetch exit decision data on expand
- [x] 9.4 Pass data to ExitDecisionMonitor
- [x] 9.5 Handle loading state
- [x] 9.6 Add section header with toggle

### Task 10: WebSocket Integration
**Estimate:** 3 hours

- [x] 10.1 Add `exit_decision_update` event type to backend
- [x] 10.2 Publish event when exit decision state changes significantly
- [x] 10.3 Add debouncing to prevent event flood (max 1 per second)
- [x] 10.4 Handle event in frontend WebSocket service
- [x] 10.5 Update ExitDecisionMonitor state on event
- [x] 10.6 Show connection status indicator

### Task 11: Testing & Verification
**Estimate:** 2 hours

- [x] 11.1 Test API endpoint with various position states
- [x] 11.2 Test UI renders correctly for Classic mode
- [x] 11.3 Test UI renders correctly for New Engine mode
- [x] 11.4 Test WebSocket updates work
- [x] 11.5 Test edge cases (no position, position closing)
- [x] 11.6 Verify performance impact on monitoring loop

---

## Dependencies

- **Story 10.1** (completed): Position stage management, PositionCardExpanded
- **Epic 11** (completed): Entry Decision Engine (for New Engine mode data)
- **Epic 12** (completed): WebSocket infrastructure

---

## Estimation Summary

| Task | Estimate |
|------|----------|
| Task 1: Exit Decision State Types | 1 hour |
| Task 2: Track State in Monitoring Loop | 3 hours |
| Task 3: Exit Decision API Endpoint | 2 hours |
| Task 4: TypeScript Types | 1 hour |
| Task 5: API Client Method | 0.5 hours |
| Task 6: Exit Decision Monitor Component | 4 hours |
| Task 7: Classic Mode Indicators Display | 2 hours |
| Task 8: New Engine Mode Display | 2 hours |
| Task 9: Integration into PositionCardExpanded | 2 hours |
| Task 10: WebSocket Integration | 3 hours |
| Task 11: Testing & Verification | 2 hours |
| **Total** | **22.5 hours** |

---

## UI Mockup

```
┌─────────────────────────────────────────────────────────────────┐
│ ▼ Exit Decision Monitor                        [↻] Last: 2s ago│
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  DECISION: HOLD                              ✅ Safe     │   │
│  │  All safeguards passed, no exit signals detected         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ── Hold Safeguards ──────────────────────────────────────────  │
│  ✅ Min Hold Time    5:32 / 5:00 ████████████████░░  PASSED    │
│  ✅ Breakeven        Achieved at 03:24:15              PASSED   │
│  ⏳ Exit Signals     0 / 2 required                    WAITING  │
│                                                                 │
│  ── Exit Checks (Priority Order) ─────────────────────────────  │
│                                                                 │
│  ▸ P1: Trend Reversal                               ✅ PASSED   │
│    └─ No reversal signals detected                              │
│                                                                 │
│  ▾ P2: Efficiency Exit                              ✅ PASSED   │
│    ├─ Peak Profit:     +2.45%                                   │
│    ├─ Current Profit:  +1.82%                                   │
│    ├─ Efficiency:      74.3%  ████████████████░░░░  OK         │
│    └─ Threshold:       50.0%  (would exit below this)           │
│                                                                 │
│  ▸ P3: Trailing SL                                  🔵 ACTIVE   │
│    └─ SL at $94,100 (0.79% away)                               │
│                                                                 │
│  ▸ P4: Dynamic TP                                   ⏳ PENDING  │
│    └─ TP at $96,500 (45.2% progress)                           │
│                                                                 │
│  ── Classic Indicators ───────────────────────────────────────  │
│  ADX:  24.5 / 20    ████████████░░░░░░  Strong Trend ↑         │
│  RSI:  58.2         ░░░░████████░░░░░░  Normal                  │
│  EMA:  No cross detected                                        │
│  Signals: 0/2       ░░░░░░░░░░░░░░░░░░  None                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-20 | ready-for-dev | Story created based on gap analysis |
| 2026-01-20 | in-progress | Implementation started |
| 2026-01-20 | done | Code review completed, all issues fixed |

## Dev Agent Record

### File List

**Files Created:**
- `internal/autopilot/exit_decision_state.go` - Exit decision state types and manager
- `internal/api/handlers_exit_decision.go` - API handler for exit decision endpoint
- `web/src/types/exitDecision.ts` - TypeScript type definitions
- `web/src/components/ExitDecisionMonitor.tsx` - Main UI component with ClassicModeIndicators and NewEngineModeDisplay embedded

**Files Modified:**
- `internal/api/server.go:476` - Added route for exit decision endpoint
- `internal/autopilot/ginie_autopilot.go` - Added exitDecisionStateManager, GetExitDecisionState(), updateExitDecisionStateForPosition()
- `internal/events/bus.go` - Added EventExitDecisionUpdate event type and broadcast function
- `internal/api/websocket_user.go` - Added BroadcastExitDecisionUpdate function and callback wiring
- `web/src/services/api.ts` - Added getExitDecisionState() method
- `web/src/components/PositionCardExpanded.tsx` - Integrated ExitDecisionMonitor component

### Code Review Notes

- **Fixed H3:** TypeScript type field mismatch - changed `data` to `exit_decision` in api.ts and exitDecision.ts
- **Fixed H2:** Added relative time display "Xs ago" with auto-refresh indicator
- **Fixed M1:** Added WebSocket connection status indicator with "Reconnecting..." display
- **Note M2:** ClassicIndicatorsPanel and NewEnginePanel embedded in ExitDecisionMonitor instead of separate files (acceptable design)

