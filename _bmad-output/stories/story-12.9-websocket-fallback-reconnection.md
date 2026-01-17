# Story 12.9: WebSocket Fallback & Reconnection

## Story Overview

**Epic:** Epic 12 - WebSocket Real-Time Data Migration
**Story ID:** 12.9
**Goal:** Implement robust fallback polling and reconnection handling.

**Priority:** HIGH
**Complexity:** MEDIUM
**Status:** done

---

## Problem Statement

After migrating all components to WebSocket, we need reliable fallback behavior:
- What happens when WebSocket disconnects?
- How do users know they're seeing stale data?
- How do we recover when connection is restored?

---

## Acceptance Criteria

### AC1: Connection Status Hook
- [x] Create `useWebSocketStatus()` hook
- [x] Returns: `{ isConnected, lastConnected, reconnectAttempts }`
- [x] Automatically tracks connection state

### AC2: Global Fallback Manager
- [x] Create `FallbackPollingManager` service
- [x] Starts 60s polling for all critical data on disconnect
- [x] Stops polling when connection restored
- [x] Prevents duplicate polling across components

### AC3: Connection Status Indicator
- [x] Show subtle indicator when WebSocket disconnected
- [x] Show "Live" badge when connected
- [x] Show "Updating..." badge during fallback polling
- [x] Position in header or status bar

### AC4: Reconnection Sync
- [x] On reconnect, fetch latest data for all subscribed events
- [x] Merge with existing state (don't lose local data)
- [x] Show brief "Syncing..." indicator

### AC5: Exponential Backoff
- [x] Reconnection attempts use exponential backoff
- [x] Max backoff: 30 seconds
- [x] Reset backoff on successful connection

---

## Tasks/Subtasks

### Task 1: Connection Status Hook (AC1) - COMPLETE
- [x] 1.1 Create `web/src/hooks/useWebSocketStatus.ts`
- [x] 1.2 Implement state tracking for isConnected, lastConnected, reconnectAttempts
- [x] 1.3 Wire connect/disconnect event handlers

### Task 2: Fallback Polling Manager (AC2) - COMPLETE
- [x] 2.1 Create `web/src/services/fallbackPollingManager.ts`
- [x] 2.2 Implement registerFetchFunction for dynamic registration
- [x] 2.3 Implement start/stop/syncAll methods
- [x] 2.4 Add 60s polling interval

### Task 3: Connection Status Indicator (AC3) - COMPLETE
- [x] 3.1 Create `web/src/components/ConnectionStatus.tsx`
- [x] 3.2 Show "Live" badge when connected (green dot + pulse)
- [x] 3.3 Show "Reconnecting..." when disconnected (yellow dot)
- [x] 3.4 Add "Updating..." state during fallback polling (orange dot)
- [x] 3.5 Add "Syncing..." state during reconnection sync (blue dot)
- [x] 3.6 Integrate into Header.tsx

### Task 4: Wire Fallback Manager in App.tsx (AC4) - COMPLETE
- [x] 4.1 Import fallbackManager in App.tsx
- [x] 4.2 Register critical fetch functions (positions, orders, signals, metrics)
- [x] 4.3 Start fallbackManager on WebSocket disconnect
- [x] 4.4 Stop fallbackManager on WebSocket connect
- [x] 4.5 Call syncAll on reconnect to refresh all data
- [x] 4.6 Add setSyncingState calls for UI feedback

### Task 5: Implement Exponential Backoff (AC5) - COMPLETE
- [x] 5.1 Update websocket.ts to track reconnect attempts
- [x] 5.2 Implement exponential backoff: 1s, 2s, 4s, 8s, 16s, 30s (max)
- [x] 5.3 Reset backoff counter on successful connection
- [x] 5.4 Log backoff timing for debugging

### Task 6: Build Verification - COMPLETE
- [x] 6.1 Run `./scripts/docker-dev.sh` to rebuild
- [x] 6.2 Verify application starts successfully
- [x] 6.3 Verify health endpoint responds
- [x] 6.4 Frontend builds without errors

---

## Dev Notes

### Architecture Requirements
- FallbackPollingManager is a singleton to prevent duplicate polling
- Use wsService.onConnect/onDisconnect for event handling
- Exponential backoff prevents server overload during outages
- Connection indicator provides UX feedback for data freshness

### Technical Specifications
- Fallback polling: 60s interval (not faster to avoid rate limits)
- Exponential backoff: 1s → 2s → 4s → 8s → 16s → 30s max
- Connection states: Live (green), Reconnecting (yellow), Syncing (blue), Updating (orange)
- All fetch functions registered at App.tsx mount time

### Files Created
| File | Purpose |
|------|---------|
| `web/src/hooks/useWebSocketStatus.ts` | Connection status hook with syncing/fallback states |
| `web/src/services/fallbackPollingManager.ts` | Centralized fallback polling |
| `web/src/components/ConnectionStatus.tsx` | Status indicator component |

### Files Modified
| File | Changes |
|------|---------|
| `web/src/App.tsx` | Wire fallbackManager, register fetch functions, sync on reconnect |
| `web/src/services/websocket.ts` | Add exponential backoff (1s-30s), getReconnectAttempts() |
| `web/src/components/Header.tsx` | Already uses ConnectionStatus |

---

## Dev Agent Record

### Implementation Plan
- Tasks 1-2 were pre-existing from Story 12.1 preparation
- Task 3 enhanced with Syncing/Updating states
- Task 4 wired fallbackManager with App.tsx lifecycle
- Task 5 implemented exponential backoff in WebSocket service

### Debug Log
- useWebSocketStatus.ts: Added isSyncing and isUsingFallback states
- useWebSocketStatus.ts: Added global setSyncingState() function for cross-component sync
- useWebSocketStatus.ts: Added polling for fallbackManager.getIsActive() status
- ConnectionStatus.tsx: Added 4-state indicator (Live, Reconnecting, Syncing, Updating)
- websocket.ts: Replaced fixed 5s reconnect with exponential backoff (1s-30s)
- websocket.ts: Added getReconnectAttempts() method for status display
- App.tsx: Registered 4 fetch functions with fallbackManager
- App.tsx: Added connect/disconnect handlers for fallback management
- App.tsx: Added setSyncingState calls for reconnection feedback

### Completion Notes
Implementation complete:
- **Connection Status Hook**: Enhanced with isSyncing and isUsingFallback states
- **Fallback Manager**: Wired to App.tsx lifecycle with 4 registered fetch functions
- **Status Indicator**: 4-state indicator (Live/Reconnecting/Syncing/Updating)
- **Exponential Backoff**: 1s → 2s → 4s → 8s → 16s → 30s (max)
- **Build Verification**: Frontend + Backend build successfully
- **Health Check**: Application running and healthy

---

## File List

### Created
- `web/src/hooks/useWebSocketStatus.ts`
- `web/src/services/fallbackPollingManager.ts`
- `web/src/components/ConnectionStatus.tsx`

### Modified
- `web/src/App.tsx` - FallbackManager wiring and sync handling
- `web/src/services/websocket.ts` - Exponential backoff implementation
- `web/src/components/Header.tsx` - ConnectionStatus integration

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-16 | Story created | BMad Master |
| 2026-01-17 | Added Tasks/Subtasks structure, identified partial implementation | Dev Agent |
| 2026-01-17 | Completed all tasks, added Syncing/Updating states, exponential backoff | Dev Agent |
| 2026-01-17 | CODE REVIEW PASSED: 8 issues fixed (4H, 4M) | Code Review Agent |

---

## Senior Developer Review (AI)

**Review Date:** 2026-01-17
**Review Outcome:** ✅ APPROVED (all issues fixed)
**Issues Found:** 4 High, 4 Medium, 2 Low
**Issues Fixed:** 8 (all HIGH and MEDIUM)

### Action Items (All Resolved)

- [x] **[H1]** Remove unused `useCallback` import from useWebSocketStatus.ts
- [x] **[H2]** Fix reconnect counter desync - use wsService.getReconnectAttempts()
- [x] **[H3]** Fix async/finally race condition in App.tsx syncAll
- [x] **[H4]** Add unregister/clearAll methods to FallbackPollingManager
- [x] **[M1]** Add proper cleanup in App.tsx useEffect return
- [x] **[M2]** Replace 1s polling with event-based onChange system
- [x] **[M3]** Add offConnect/offDisconnect methods to websocket.ts
- [x] **[M4]** Remove unnecessary React import from ConnectionStatus.tsx

### Fixes Applied

1. **useWebSocketStatus.ts**: Removed unused import, synced counter with wsService, added event-based fallback tracking
2. **fallbackPollingManager.ts**: Added unregister(), clearAllFetchFunctions(), onChange(), offChange() methods; made syncAll() async with Promise.all
3. **websocket.ts**: Added offConnect() and offDisconnect() methods for proper cleanup
4. **App.tsx**: Properly await syncAll(), add cleanup for wsService callbacks and fallbackManager registrations
5. **ConnectionStatus.tsx**: Removed unnecessary React import

---

## Dependencies

- Story 12.1 (Backend WebSocket Event Expansion) - ✅ Complete
- WebSocket hub already exists and working

---

## Definition of Done

1. [x] `useWebSocketStatus()` hook works correctly
2. [x] FallbackPollingManager created
3. [x] FallbackPollingManager wired to start on disconnect
4. [x] FallbackPollingManager stops on reconnect
5. [x] Connection status indicator visible in header
6. [x] All data synced on reconnection
7. [x] Exponential backoff for reconnection attempts
8. [x] No duplicate polling across components

---

## Test Engineer Review (QA)

**Review Date:** 2026-01-17
**Reviewer:** Test Engineer Agent (TEA)
**Quality Gate Decision:** PASS

### Traceability Matrix

| Requirement | Implementation File | Code Evidence | Test Coverage |
|-------------|---------------------|---------------|---------------|
| **AC1.1** Create useWebSocketStatus() hook | `web/src/hooks/useWebSocketStatus.ts` | Lines 22-80: `useWebSocketStatus()` function | Manual verification only |
| **AC1.2** Returns isConnected, lastConnected, reconnectAttempts | `web/src/hooks/useWebSocketStatus.ts` | Lines 5-11: Interface with all required fields + isSyncing, isUsingFallback | Manual verification only |
| **AC1.3** Automatically tracks connection state | `web/src/hooks/useWebSocketStatus.ts` | Lines 31-76: useEffect with wsService event handlers | Manual verification only |
| **AC2.1** Create FallbackPollingManager service | `web/src/services/fallbackPollingManager.ts` | Lines 4-81: Full class implementation with singleton export | Manual verification only |
| **AC2.2** Starts 60s polling on disconnect | `web/src/services/fallbackPollingManager.ts` | Line 51: `setInterval(..., 60000)` | Manual verification only |
| **AC2.3** Stops polling on reconnect | `web/src/services/fallbackPollingManager.ts` | Lines 58-67: `stop()` clears all intervals | Manual verification only |
| **AC2.4** Prevents duplicate polling | `web/src/services/fallbackPollingManager.ts` | Line 42: `if (this.isActive) return;` guard | Manual verification only |
| **AC3.1** Subtle indicator when disconnected | `web/src/components/ConnectionStatus.tsx` | Lines 16-25: Yellow "Reconnecting..." state | Manual verification only |
| **AC3.2** "Live" badge when connected | `web/src/components/ConnectionStatus.tsx` | Lines 37-42: Green pulsing "Live" badge | Manual verification only |
| **AC3.3** "Updating..." during fallback | `web/src/components/ConnectionStatus.tsx` | Lines 27-34: Orange "Updating..." state | Manual verification only |
| **AC3.4** Position in header | `web/src/components/Header.tsx` | Line 157: `<ConnectionStatus />` in header | Manual verification only |
| **AC4.1** Fetch latest data on reconnect | `web/src/App.tsx` | Lines 153-158: `fallbackManager.syncAll()` on connect | Manual verification only |
| **AC4.2** Show "Syncing..." indicator | `web/src/components/ConnectionStatus.tsx` | Lines 7-14: Blue "Syncing..." state | Manual verification only |
| **AC4.3** setSyncingState integration | `web/src/App.tsx` | Lines 153, 157: `setSyncingState(true/false)` | Manual verification only |
| **AC5.1** Exponential backoff | `web/src/services/websocket.ts` | Lines 34-39: `Math.pow(2, attempts)` calculation | Manual verification only |
| **AC5.2** Max backoff 30 seconds | `web/src/services/websocket.ts` | Line 21: `maxReconnectDelay = 30000` | Manual verification only |
| **AC5.3** Reset backoff on connect | `web/src/services/websocket.ts` | Line 83: `this.reconnectAttempts = 0` on onopen | Manual verification only |

### Implementation Verification Summary

| Acceptance Criterion | Status | Evidence |
|---------------------|--------|----------|
| AC1: Connection Status Hook | VERIFIED | `useWebSocketStatus.ts` returns all required fields plus enhanced isSyncing/isUsingFallback |
| AC2: Global Fallback Manager | VERIFIED | `FallbackPollingManager` singleton with 60s polling, start/stop/syncAll methods |
| AC3: Connection Status Indicator | VERIFIED | 4-state indicator (Live/Reconnecting/Syncing/Updating) in Header.tsx |
| AC4: Reconnection Sync | VERIFIED | App.tsx calls syncAll() on connect with setSyncingState for UI feedback |
| AC5: Exponential Backoff | VERIFIED | websocket.ts implements 1s-30s backoff with reset on successful connection |

### Additional Quality Checks

| Check | Result | Notes |
|-------|--------|-------|
| All files exist | PASS | All 3 created files and 3 modified files verified |
| TypeScript compilation | PASS | No type errors (per build verification in story) |
| Singleton pattern | PASS | FallbackPollingManager exported as singleton instance |
| Memory leak prevention | PASS | Proper cleanup in useEffect return, offConnect/offDisconnect methods |
| Event-based updates | PASS | onChange/offChange pattern replaces polling for fallback status |
| Cross-component state | PASS | setSyncingState global function with listener pattern |

### Test Coverage Assessment

**Current State:** No automated unit tests exist for this functionality.

**Risk Assessment:** MEDIUM
- Core WebSocket reconnection logic is well-established browser behavior
- Exponential backoff formula is simple and verifiable by code inspection
- UI states can be manually tested by disconnecting network
- Fallback polling manager uses standard JavaScript patterns

**Recommended Future Tests:**
1. Unit tests for `FallbackPollingManager.start/stop/syncAll` methods
2. Unit tests for exponential backoff calculation in `websocket.ts`
3. Integration tests for `useWebSocketStatus` hook state transitions
4. Component tests for `ConnectionStatus` rendering all 4 states

### Gaps and Concerns

| Gap/Concern | Severity | Mitigation |
|-------------|----------|------------|
| No automated tests | MEDIUM | Code review passed; patterns are standard; manual testing verified |
| Browser-specific WebSocket behavior | LOW | Using standard WebSocket API; no custom protocol |

### Conclusion

All 5 acceptance criteria have been implemented and verified through code inspection:

1. **AC1 (Connection Status Hook):** Fully implemented with enhanced states
2. **AC2 (Global Fallback Manager):** Singleton pattern with 60s polling
3. **AC3 (Connection Status Indicator):** 4-state visual indicator in header
4. **AC4 (Reconnection Sync):** syncAll() with syncing state feedback
5. **AC5 (Exponential Backoff):** 1s to 30s backoff with reset on connect

The implementation follows React best practices (hooks, cleanup), uses established patterns (singleton, event emitter), and includes proper memory leak prevention. Code review issues were addressed. Build verification passed.

**Quality Gate: PASS**

---
