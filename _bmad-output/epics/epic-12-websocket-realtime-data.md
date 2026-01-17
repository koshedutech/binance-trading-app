# Epic 12: WebSocket Real-Time Data Migration

## Epic Overview

**Goal:** Migrate all frontend components from API polling to WebSocket real-time updates, ensuring instant data synchronization for positions, orders, chains, and system status.

**Business Value:** Eliminate 30-second data staleness, reduce API rate limit pressure, improve user experience with instant updates, and provide consistent real-time behavior across all components.

**Priority:** HIGH - User experience and system efficiency

**Estimated Complexity:** MEDIUM

**Depends On:** Epic 7 (Client Order ID & Trade Lifecycle)

---

## Problem Statement

### Current Issues

| Issue | Severity | Impact |
|-------|----------|--------|
| **45 components still polling** | HIGH | Stale data up to 30 seconds |
| **TradeLifecycleTab polls every 30s** | HIGH | Order chains not real-time |
| **TradeLifecycleEvents polls every 30s** | HIGH | Lifecycle events delayed |
| **Ginie panels poll every 5-10s** | MEDIUM | Unnecessary API load |
| **Inconsistent update behavior** | MEDIUM | Some data real-time, some polling |

### Current State

```
CURRENT STATE:
┌─────────────────────────────────────────────────────────────────┐
│ WebSocket ALREADY IMPLEMENTED for:                              │
│  ✅ Positions (POSITION_UPDATE)                                 │
│  ✅ Orders (ORDER_UPDATE)                                       │
│  ✅ Balances (BALANCE_UPDATE)                                   │
│  ✅ Order Book (FUTURES_ORDERBOOK_UPDATE)                       │
│  ✅ Mark Prices (FUTURES_MARK_PRICE_UPDATE)                     │
│  ✅ Klines (FUTURES_KLINE_UPDATE)                               │
│                                                                 │
│ BUT many components IGNORE WebSocket and still POLL:            │
│  ❌ TradeLifecycleTab - polls /api/futures/orders/all (30s)     │
│  ❌ TradeLifecycleEvents - polls /api/futures/trade-events (30s)│
│  ❌ FuturesPositionsTable - polls positions (30s)               │
│  ❌ GiniePanel - polls status (10s)                             │
│  ❌ CircuitBreakerPanel - polls status (15s)                    │
│  ❌ 40+ other components with setInterval polling               │
└─────────────────────────────────────────────────────────────────┘
```

---

## Target State

```
TARGET STATE:
┌─────────────────────────────────────────────────────────────────┐
│ ALL components use WebSocket for real-time updates:             │
│                                                                 │
│  Backend WebSocket Events → Frontend Subscriptions → UI Update  │
│                                                                 │
│  New Events to Add:                                             │
│  ├── CHAIN_UPDATE (order chain state changes)                  │
│  ├── LIFECYCLE_EVENT (new trade lifecycle event logged)        │
│  ├── GINIE_STATUS_UPDATE (autopilot status changes)            │
│  ├── CIRCUIT_BREAKER_UPDATE (circuit breaker state)            │
│  ├── PNL_UPDATE (P&L calculations)                             │
│  └── SIGNAL_UPDATE (AI signals generated)                      │
│                                                                 │
│  Fallback Polling: 60s (only when WebSocket disconnected)       │
│                                                                 │
│  Result:                                                        │
│  - Instant updates (<100ms latency)                             │
│  - 90% reduction in API calls                                   │
│  - Consistent UX across all components                          │
└─────────────────────────────────────────────────────────────────┘
```

---

## Components to Migrate

### Phase 1: HIGH PRIORITY (Critical Trading Data)

| Component | File | Current Polling | Target Event |
|-----------|------|-----------------|--------------|
| TradeLifecycleTab | `TradeLifecycle/TradeLifecycleTab.tsx:129` | 30s | `ORDER_UPDATE`, `CHAIN_UPDATE` |
| TradeLifecycleEvents | `TradeLifecycleEvents.tsx:161` | 30s | `LIFECYCLE_EVENT` |
| FuturesPositionsTable | `FuturesPositionsTable.tsx:106,150,175` | 30s | `POSITION_UPDATE` (exists) |
| FuturesOrdersHistory | `FuturesOrdersHistory.tsx:162` | 30s | `ORDER_UPDATE` (exists) |
| TradeHistory | `TradeHistory.tsx:158` | 30s | `TRADE_UPDATE` (exists) |

### Phase 2: MEDIUM PRIORITY (Autopilot & Status)

| Component | File | Current Polling | Target Event |
|-----------|------|-----------------|--------------|
| AITradeStatusPanel | `AITradeStatusPanel.tsx:53` | 5s | `GINIE_STATUS_UPDATE` |
| PendingSignalsModal | `PendingSignalsModal.tsx:34` | 5s | `SIGNAL_UPDATE` |
| ModeSafetyPanel | `ModeSafetyPanel.tsx:65` | 5s | `MODE_SAFETY_UPDATE` |
| ModeAllocationPanel | `ModeAllocationPanel.tsx:161` | 5s | `MODE_ALLOCATION_UPDATE` |
| GinieDiagnosticsPanel | `GinieDiagnosticsPanel.tsx:154` | 10s | `GINIE_STATUS_UPDATE` |
| InvestigatePanel | `InvestigatePanel.tsx:53` | 10s | `SYSTEM_STATUS_UPDATE` |
| CircuitBreakerPanel | `CircuitBreakerPanel.tsx:70` | 15s | `CIRCUIT_BREAKER_UPDATE` |
| AutopilotRulesPanel | `AutopilotRulesPanel.tsx:101` | 15s | `AUTOPILOT_RULES_UPDATE` |
| EnhancedSignalsPanel | `EnhancedSignalsPanel.tsx:30` | 15s | `SIGNAL_UPDATE` |

### Phase 3: LOW PRIORITY (Analytics & Summaries)

| Component | File | Current Polling | Target Event |
|-----------|------|-----------------|--------------|
| PnLDashboard | `PnLDashboard.tsx:53` | 30s | `PNL_UPDATE` |
| AdvancedTradingPanel | `AdvancedTradingPanel.tsx:54` | 30s | Mixed events |
| APIHealthIndicator | `APIHealthIndicator.tsx:51` | 30s | `HEALTH_UPDATE` |
| GiniePanel | `GiniePanel.tsx:902` | 60s | `GINIE_STATUS_UPDATE` |
| PnLSummaryCards | `PnLSummaryCards.tsx:51` | 60s | `PNL_UPDATE` |
| TradeSourceStatsPanel | `TradeSourceStatsPanel.tsx:33` | 60s | `STATS_UPDATE` |
| SymbolPerformancePanel | `SymbolPerformancePanel.tsx:190` | 60s | `PERFORMANCE_UPDATE` |
| StrategyPerformanceDashboard | `StrategyPerformanceDashboard.tsx:260` | 30s | `PERFORMANCE_UPDATE` |

### Keep Polling (External/Static Data)

| Component | Reason |
|-----------|--------|
| NewsDashboard (180s) | External API, no WebSocket available |
| NewsFeedPanel (300s) | External API, no WebSocket available |
| StrategiesPanel (60s) | Static configuration data |

---

## Stories

### Story 12.1: Backend WebSocket Event Expansion

**Goal:** Add new WebSocket event types for missing data categories.

**New Events:**
- `CHAIN_UPDATE` - When order chain state changes
- `LIFECYCLE_EVENT` - When trade lifecycle event is logged
- `GINIE_STATUS_UPDATE` - When autopilot status changes
- `CIRCUIT_BREAKER_UPDATE` - When circuit breaker triggers/resets
- `PNL_UPDATE` - When P&L is recalculated
- `SIGNAL_UPDATE` - When AI signal is generated
- `MODE_SAFETY_UPDATE` - When mode safety status changes
- `SYSTEM_STATUS_UPDATE` - When system health changes

---

### Story 12.2: Trade Lifecycle Tab WebSocket Migration

**Goal:** Replace 30s polling in TradeLifecycleTab with WebSocket subscription.

**File:** `web/src/components/TradeLifecycle/TradeLifecycleTab.tsx:129`

**Changes:**
- Subscribe to `ORDER_UPDATE` and `CHAIN_UPDATE` events
- Remove `setInterval` polling
- Update chain display on WebSocket event
- Add fallback 60s polling only when WebSocket disconnected

---

### Story 12.3: Trade Lifecycle Events WebSocket Migration

**Goal:** Replace 30s polling in TradeLifecycleEvents with WebSocket subscription.

**File:** `web/src/components/TradeLifecycleEvents.tsx:161`

**Changes:**
- Subscribe to `LIFECYCLE_EVENT` WebSocket event
- Backend broadcasts event when `trade_lifecycle_events` table updated
- Append new events to list in real-time
- Remove `setInterval` polling

---

### Story 12.4: FuturesPositionsTable WebSocket Wiring

**Goal:** Wire existing `POSITION_UPDATE` event to FuturesPositionsTable.

**File:** `web/src/components/FuturesPositionsTable.tsx:106,150,175`

**Current:** Polls positions every 30s despite WebSocket existing
**Target:** Subscribe to `POSITION_UPDATE` and update table instantly

---

### Story 12.5: Ginie & Autopilot Status WebSocket

**Goal:** Replace polling in Ginie panels with WebSocket.

**Files:**
- `GiniePanel.tsx:854,902`
- `GinieDiagnosticsPanel.tsx:154`
- `AITradeStatusPanel.tsx:53`

**Changes:**
- Backend broadcasts `GINIE_STATUS_UPDATE` on autopilot state changes
- Frontend subscribes and updates panels instantly
- Remove 5-10s polling intervals

---

### Story 12.6: Circuit Breaker & Safety Panels WebSocket

**Goal:** Replace polling in safety panels with WebSocket.

**Files:**
- `CircuitBreakerPanel.tsx:70`
- `ModeSafetyPanel.tsx:65`
- `ModeAllocationPanel.tsx:161`

**Changes:**
- Backend broadcasts `CIRCUIT_BREAKER_UPDATE` on trigger/reset
- Backend broadcasts `MODE_SAFETY_UPDATE` on safety state changes
- Frontend subscribes and updates panels instantly

---

### Story 12.7: P&L Dashboard WebSocket

**Goal:** Replace polling in P&L components with WebSocket.

**Files:**
- `PnLDashboard.tsx:53`
- `PnLSummaryCards.tsx:51`

**Changes:**
- Backend broadcasts `PNL_UPDATE` after position/trade updates
- Frontend subscribes and updates P&L display instantly

---

### Story 12.8: Signals Panel WebSocket

**Goal:** Replace polling in signal panels with WebSocket.

**Files:**
- `PendingSignalsModal.tsx:34`
- `EnhancedSignalsPanel.tsx:30`
- `FuturesAISignals.tsx:98`
- `AISignalsPanel.tsx:141`

**Changes:**
- Backend broadcasts `SIGNAL_UPDATE` when AI generates signal
- Frontend subscribes and shows new signals instantly

---

### Story 12.9: WebSocket Fallback & Reconnection

**Goal:** Implement robust fallback polling when WebSocket disconnects.

**Changes:**
- Detect WebSocket disconnect in all components
- Start 60s fallback polling on disconnect
- Stop fallback polling when WebSocket reconnects
- Show connection status indicator to user

---

### Story 12.10: Remove All Deprecated Polling

**Goal:** Clean up all `setInterval` calls after WebSocket migration.

**Verification:**
- Grep for `setInterval` in frontend
- Verify only external API polling remains (News)
- Remove dead code and unused API endpoints

---

## Success Criteria

1. **TradeLifecycleTab updates instantly** when orders change
2. **TradeLifecycleEvents updates instantly** when events are logged
3. **All Ginie panels update instantly** on autopilot changes
4. **Circuit breaker changes appear instantly**
5. **P&L updates appear instantly** after trades
6. **Signals appear instantly** when generated
7. **No polling faster than 60s** (fallback only)
8. **WebSocket reconnection works** with automatic recovery
9. **API rate limit usage reduced by 80%+**
10. **User sees consistent real-time behavior** across all panels

---

## Technical Architecture

### Event Flow

```
┌─────────────────────────────────────────────────────────────────┐
│ BACKEND EVENT SOURCES                                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Binance User Data Stream                                        │
│   ├── Position updates  ──────► POSITION_UPDATE                 │
│   ├── Order updates     ──────► ORDER_UPDATE                    │
│   └── Balance updates   ──────► BALANCE_UPDATE                  │
│                                                                 │
│ Ginie Autopilot Engine                                          │
│   ├── Status changes    ──────► GINIE_STATUS_UPDATE             │
│   ├── Signal generated  ──────► SIGNAL_UPDATE                   │
│   └── Trade executed    ──────► CHAIN_UPDATE                    │
│                                                                 │
│ Trade Lifecycle Repository                                      │
│   └── Event logged      ──────► LIFECYCLE_EVENT                 │
│                                                                 │
│ Circuit Breaker Service                                         │
│   └── State changed     ──────► CIRCUIT_BREAKER_UPDATE          │
│                                                                 │
│ P&L Calculator                                                  │
│   └── Recalculated      ──────► PNL_UPDATE                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ WEBSOCKET HUB (User-Isolated)                                   │
│                                                                 │
│ BroadcastUserEvent(userID, eventType, payload)                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ FRONTEND COMPONENTS                                             │
│                                                                 │
│ webSocketService.subscribe('EVENT_TYPE', (data) => {            │
│   updateComponentState(data);                                   │
│ });                                                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### New Backend Broadcast Functions

```go
// internal/api/websocket_user.go

func (h *UserHub) BroadcastChainUpdate(userID string, chain *OrderChain) {
    h.BroadcastToUser(userID, "CHAIN_UPDATE", chain)
}

func (h *UserHub) BroadcastLifecycleEvent(userID string, event *TradeLifecycleEvent) {
    h.BroadcastToUser(userID, "LIFECYCLE_EVENT", event)
}

func (h *UserHub) BroadcastGinieStatus(userID string, status *GinieStatus) {
    h.BroadcastToUser(userID, "GINIE_STATUS_UPDATE", status)
}

func (h *UserHub) BroadcastCircuitBreaker(userID string, state *CircuitBreakerState) {
    h.BroadcastToUser(userID, "CIRCUIT_BREAKER_UPDATE", state)
}

func (h *UserHub) BroadcastPnL(userID string, pnl *PnLSummary) {
    h.BroadcastToUser(userID, "PNL_UPDATE", pnl)
}

func (h *UserHub) BroadcastSignal(userID string, signal *TradingSignal) {
    h.BroadcastToUser(userID, "SIGNAL_UPDATE", signal)
}
```

### Frontend Subscription Pattern

```typescript
// Component subscription pattern
useEffect(() => {
  const unsubscribe = webSocketService.subscribe('CHAIN_UPDATE', (data) => {
    setChains(prevChains => updateChain(prevChains, data));
  });

  // Fallback polling only when disconnected
  let fallbackInterval: NodeJS.Timeout | null = null;

  const checkConnection = () => {
    if (!webSocketService.isConnected()) {
      fallbackInterval = setInterval(fetchData, 60000);
    }
  };

  webSocketService.on('disconnect', () => {
    fallbackInterval = setInterval(fetchData, 60000);
  });

  webSocketService.on('connect', () => {
    if (fallbackInterval) clearInterval(fallbackInterval);
  });

  return () => {
    unsubscribe();
    if (fallbackInterval) clearInterval(fallbackInterval);
  };
}, []);
```

---

## Dependencies

| Dependency | Type | Status |
|------------|------|--------|
| Epic 7 - Client Order ID | Prerequisite | ✅ Complete |
| Existing WebSocket Hub | Infrastructure | ✅ Available |
| Binance User Data Stream | External | ✅ Working |

---

## Author

**Created By:** BMad Master
**Date:** 2026-01-16
**Version:** 1.0
**Depends On:** Epic 7 (Client Order ID)
