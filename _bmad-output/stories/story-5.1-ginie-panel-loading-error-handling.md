# Story 5.1: Fix Ginie Panel Loading & Error Handling (CRITICAL)

**Story ID:** GINIE-5.1
**Epic:** Epic 5 - Ginie Panel Error Handling & Reliability
**Priority:** P0 (CRITICAL - Blocks User Experience)
**Estimated Effort:** 8 hours
**Author:** BMAD Agent (Bob - Scrum Master)
**Status:** draft
**Depends On:** None (Critical fix for existing functionality)

---

## Problem Statement

### Current State

The Ginie Panel suffers from CRITICAL error handling issues that create a broken user experience:

1. **Infinite Loading State**: When API calls fail (401/403/503), the panel shows "Loading..." forever
2. **Silent Error Handling**: Errors are swallowed silently - users have no idea anything went wrong
3. **Token Expiry Silent Failure**: 401/403 errors return early without setting error state, leaving users confused
4. **503 Backend Errors Cascade**: When backend is down (503), entire panel breaks with no recovery option
5. **No Per-Section Error States**: One failed API call can make the entire panel unusable
6. **No Retry Mechanism**: Users must refresh the entire page to retry failed requests

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Problematic Code (Lines 161-174, 177-199, 259-268):**

```typescript
// Lines 161-174: Status fetch with silent 401/403 handling
const fetchStatus = async () => {
  try {
    const data = await futuresApi.getGinieStatus();
    setStatus(data);
    setStatusError(null);
    setError(null);
  } catch (err: any) {
    // PROBLEM: Returns early without setting error state
    if (err?.response?.status === 401 || err?.response?.status === 403) return;
    console.error('Failed to fetch Ginie status:', err);
    const errorMsg = err?.response?.data?.message || err?.message || 'Failed to load Ginie status';
    setStatusError(errorMsg);
    // Don't clear status - keep showing last known good data
  }
};

// Lines 177-199: Autopilot status with same issue
const fetchAutopilotStatus = async (initSettings = false) => {
  try {
    const data = await futuresApi.getGinieAutopilotStatus();
    setAutopilotStatus(data);
    setAutopilotError(null);
    // ... initialization logic
  } catch (err: any) {
    // PROBLEM: Returns early without setting error state
    if (err?.response?.status === 401 || err?.response?.status === 403) return;
    console.error('Failed to fetch Ginie autopilot status:', err);
    const errorMsg = err?.response?.data?.message || err?.message || 'Failed to load autopilot status';
    setAutopilotError(errorMsg);
  }
};

// Lines 259-268: Diagnostics with silent failure
const fetchDiagnostics = async () => {
  try {
    const data = await futuresApi.getGinieDiagnostics();
    setDiagnostics(data);
    setDiagnosticsError(null);
  } catch (err: any) {
    console.error('Failed to fetch diagnostics:', err);
    const errorMsg = err?.response?.data?.message || err?.message || 'Failed to load diagnostics';
    setDiagnosticsError(errorMsg);
  }
};
```

**Current Error Display (Lines 1053-1073):**

```typescript
// Only shown for statusError - but statusError is NOT SET for 401/403!
{statusError ? (
  <div className="text-red-400 text-sm">
    <p>Error: {statusError}</p>
    <button onClick={fetchStatus} className="mt-2 px-3 py-1 bg-purple-600 hover:bg-purple-700 rounded text-white text-xs">
      Retry
    </button>
  </div>
) : (
  <div className="text-gray-400 text-sm">Loading...</div>  // STUCK HERE FOREVER
)}
```

### Impact on Users

| Scenario | Current Behavior | User Experience |
|----------|------------------|-----------------|
| **Token Expires** | Panel shows "Loading..." forever | User thinks app is broken, refreshes page |
| **Backend 503** | Panel shows "Loading..." forever | User has no idea backend is down |
| **Network Timeout** | Panel shows "Loading..." forever | User waits indefinitely |
| **Auth Error** | Silent failure, no error shown | User confused why data isn't loading |
| **Any API Failure** | Must refresh entire page | Loses context, frustrating UX |

### Expected Behavior

1. **Never Infinite Loading**: Always show either data, error, or retry option
2. **Session Expiry Detection**: 401/403 shows "Session expired - Please log in again" with login link
3. **503 Backend Down**: Shows "Service temporarily unavailable" with retry button
4. **Per-Section Error States**: Each panel section handles errors independently
5. **One-Click Retry**: Users can retry failed requests without page refresh
6. **Clear Error Messages**: Users know exactly what went wrong and how to fix it

---

## User Story

> As a trader monitoring the Ginie Panel,
> I want clear error messages and retry options when API calls fail,
> So that I can quickly recover from errors without refreshing the page or losing my context.

---

## Design Mockup

### Error States Visual

```
┌─────────────────────────────────────────────────────────────────────┐
│  GINIE AI - Error State Examples                                   │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  SCENARIO 1: Session Expired (401/403)                             │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🔒 Session Expired                                          │   │
│  │                                                              │   │
│  │  Your session has expired. Please log in again to continue. │   │
│  │                                                              │   │
│  │  [Log In Again]                                              │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  SCENARIO 2: Backend Down (503)                                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ⚠️ Service Temporarily Unavailable                          │   │
│  │                                                              │   │
│  │  The Ginie service is temporarily unavailable. This is      │   │
│  │  usually due to maintenance or high load.                   │   │
│  │                                                              │   │
│  │  [Retry]  [Try Again in 30s]                                │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  SCENARIO 3: Network Error / Timeout                               │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🌐 Network Error                                            │   │
│  │                                                              │   │
│  │  Failed to connect to Ginie API. Check your internet        │   │
│  │  connection.                                                 │   │
│  │                                                              │   │
│  │  Error: Failed to load Ginie status                         │   │
│  │                                                              │   │
│  │  [Retry]                                                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  SCENARIO 4: Per-Section Independent Errors                        │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ✅ GINIE AI STATUS                                          │   │
│  │  Enabled: Yes | Scanned: 42 coins | Last scan: 2m ago       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ⚠️ AUTOPILOT STATUS - Error Loading                        │   │
│  │                                                              │   │
│  │  Failed to load autopilot status                            │   │
│  │  [Retry]                                                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ✅ DIAGNOSTICS                                              │   │
│  │  Can Trade: Yes | LLM Active: 3 symbols                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Loading States (Non-Infinite)

```
┌─────────────────────────────────────────────────────────────────────┐
│  INITIAL LOAD (First 10 seconds max)                               │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  🔄 Loading Ginie Status...                                  │   │
│  │  ████████░░░░░░░░░░░░░░░░ (60%)                             │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  AFTER 10 SECONDS - Show timeout warning                           │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  ⏳ Taking longer than usual...                              │   │
│  │                                                              │   │
│  │  The server might be slow or unreachable.                   │   │
│  │                                                              │   │
│  │  Still loading... [Cancel & Retry]                          │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Acceptance Criteria

### AC5.1.1: Session Expiry Detection (401/403)

**Must Handle:**
- [ ] 401 Unauthorized errors
- [ ] 403 Forbidden errors
- [ ] Token expiry errors from any API endpoint

**Display Requirements:**
- [ ] Clear message: "Session expired - Please log in again"
- [ ] Lock icon (🔒) for visual clarity
- [ ] "Log In Again" button redirects to `/login`
- [ ] Error persists until user takes action (no auto-retry)
- [ ] Does NOT show infinite loading state

**Technical:**
- [ ] Set `statusError`, `autopilotError`, or `diagnosticsError` with session expiry message
- [ ] Do NOT return early without setting error state
- [ ] Clear any loading spinners
- [ ] Keep last known good data visible if available

**Test:**
```typescript
// When token expires
fetchStatus() // Gets 401
// Expected: statusError = "Session expired - Please log in again"
// Expected: No loading spinner
// Expected: Shows error UI with login button
```

---

### AC5.1.2: Backend Unavailable (503)

**Must Handle:**
- [ ] 503 Service Unavailable errors
- [ ] 502 Bad Gateway errors
- [ ] 504 Gateway Timeout errors

**Display Requirements:**
- [ ] Clear message: "Service temporarily unavailable"
- [ ] Warning icon (⚠️)
- [ ] Explanation: "Usually due to maintenance or high load"
- [ ] "Retry" button to manually retry
- [ ] Optional: "Try Again in 30s" auto-retry countdown

**Technical:**
- [ ] Detect 503/502/504 status codes
- [ ] Set appropriate error state with user-friendly message
- [ ] Provide retry mechanism
- [ ] Log technical details to console for debugging

**Test:**
```typescript
// When backend is down
fetchStatus() // Gets 503
// Expected: statusError = "Service temporarily unavailable. This is usually due to maintenance or high load."
// Expected: Retry button visible
// Expected: Clicking retry calls fetchStatus() again
```

---

### AC5.1.3: Network Errors & Timeouts

**Must Handle:**
- [ ] Network timeout (no response)
- [ ] DNS resolution failures
- [ ] Connection refused errors
- [ ] Generic network errors

**Display Requirements:**
- [ ] Clear message: "Network Error"
- [ ] Globe icon (🌐)
- [ ] User-friendly explanation: "Check your internet connection"
- [ ] Technical error details in small text
- [ ] "Retry" button

**Technical:**
- [ ] Detect timeout errors (axios timeout, no response)
- [ ] Differentiate between network errors and API errors
- [ ] Show technical message from error object
- [ ] Allow immediate retry

---

### AC5.1.4: Per-Section Independent Error States

**Requirements:**
- [ ] Each panel section has its own error state:
  - `statusError` for Ginie AI status
  - `autopilotError` for Autopilot status
  - `diagnosticsError` for Diagnostics tab
- [ ] One section failing does NOT break other sections
- [ ] Each section shows its own error UI
- [ ] Each section has its own "Retry" button
- [ ] Successful sections display normally

**Visual:**
```typescript
// Section 1: Success
<GinieStatusPanel status={status} error={null} />

// Section 2: Error
<AutopilotStatusPanel
  status={null}
  error="Failed to load autopilot status"
  onRetry={fetchAutopilotStatus}
/>

// Section 3: Success
<DiagnosticsPanel diagnostics={diagnostics} error={null} />
```

---

### AC5.1.5: Retry Mechanism (No Page Refresh Required)

**Requirements:**
- [ ] Every error state shows a "Retry" button
- [ ] Clicking "Retry" calls the appropriate fetch function
- [ ] Loading state shown during retry
- [ ] Error cleared on successful retry
- [ ] New error shown if retry fails
- [ ] No page refresh required

**Implementation:**
```typescript
// Error UI with retry button
{statusError && (
  <div className="error-state">
    <p>{statusError}</p>
    <button onClick={fetchStatus} disabled={loading}>
      {loading ? 'Retrying...' : 'Retry'}
    </button>
  </div>
)}
```

---

### AC5.1.6: Loading State Timeout (Max 10 Seconds)

**Requirements:**
- [ ] Initial loading shows spinner with progress message
- [ ] After 10 seconds, show "Taking longer than usual..." warning
- [ ] After 30 seconds, show error state with retry option
- [ ] Never show infinite loading spinner

**Implementation:**
```typescript
// Loading timeout detection
useEffect(() => {
  if (loading) {
    const timeout = setTimeout(() => {
      setLoadingWarning(true); // After 10s
    }, 10000);

    const errorTimeout = setTimeout(() => {
      setStatusError('Request timed out after 30 seconds');
      setLoading(false);
    }, 30000);

    return () => {
      clearTimeout(timeout);
      clearTimeout(errorTimeout);
    };
  }
}, [loading]);
```

---

### AC5.1.7: Clear Error Messages

**Error Message Standards:**

| Error Type | User Message | Technical Details |
|------------|--------------|-------------------|
| **401/403** | "Session expired - Please log in again" | Console: Full error object |
| **503** | "Service temporarily unavailable. Usually due to maintenance or high load." | Console: Backend error |
| **Network** | "Network Error - Check your internet connection" | Console: Network details |
| **Timeout** | "Request timed out. The server is taking too long to respond." | Console: Timeout duration |
| **Generic** | "Failed to load [section name]" | Console: Original error |

**Display Requirements:**
- [ ] User-friendly message prominently displayed
- [ ] Technical details logged to console (not shown to user)
- [ ] Consistent error UI across all sections
- [ ] Icon indicating error type
- [ ] Action button (Retry or Login)

---

## Technical Implementation

### Task 1: Add Error Classification Helper

Create a helper function to classify errors and generate appropriate messages.

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Location:** Add after line 17 (after imports), before component definition

```typescript
// Error classification helper
interface ErrorClassification {
  type: 'session_expired' | 'service_unavailable' | 'network_error' | 'timeout' | 'generic';
  userMessage: string;
  technicalMessage: string;
  icon: string;
  actionType: 'login' | 'retry' | 'retry_with_delay';
  retryDelay?: number;
}

function classifyError(err: any, context: string): ErrorClassification {
  const status = err?.response?.status;
  const message = err?.response?.data?.message || err?.message || '';

  // Session expired (401/403)
  if (status === 401 || status === 403) {
    return {
      type: 'session_expired',
      userMessage: 'Session expired - Please log in again',
      technicalMessage: `${context}: ${message}`,
      icon: '🔒',
      actionType: 'login',
    };
  }

  // Service unavailable (503/502/504)
  if (status === 503 || status === 502 || status === 504) {
    return {
      type: 'service_unavailable',
      userMessage: 'Service temporarily unavailable. This is usually due to maintenance or high load.',
      technicalMessage: `${context}: Backend returned ${status}`,
      icon: '⚠️',
      actionType: 'retry_with_delay',
      retryDelay: 30,
    };
  }

  // Network timeout
  if (err?.code === 'ECONNABORTED' || message.includes('timeout')) {
    return {
      type: 'timeout',
      userMessage: 'Request timed out. The server is taking too long to respond.',
      technicalMessage: `${context}: ${message}`,
      icon: '⏳',
      actionType: 'retry',
    };
  }

  // Network error (no response)
  if (!err?.response) {
    return {
      type: 'network_error',
      userMessage: 'Network Error - Check your internet connection',
      technicalMessage: `${context}: ${message}`,
      icon: '🌐',
      actionType: 'retry',
    };
  }

  // Generic error
  return {
    type: 'generic',
    userMessage: `Failed to load ${context}`,
    technicalMessage: `${context}: ${message}`,
    icon: '❌',
    actionType: 'retry',
  };
}
```

---

### Task 2: Fix fetchStatus with Proper Error Handling

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Replace lines 161-175:**

```typescript
const fetchStatus = async () => {
  try {
    const data = await futuresApi.getGinieStatus();
    setStatus(data);
    setStatusError(null); // Clear error on success
    setError(null);
  } catch (err: any) {
    const classified = classifyError(err, 'Ginie status');
    console.error(`[GINIE-STATUS] ${classified.technicalMessage}`, err);

    // CRITICAL FIX: Always set error state, even for 401/403
    setStatusError(classified.userMessage);

    // Keep last known good data visible (don't clear status)
  }
};
```

---

### Task 3: Fix fetchAutopilotStatus with Proper Error Handling

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Replace lines 177-200:**

```typescript
const fetchAutopilotStatus = async (initSettings = false) => {
  try {
    const data = await futuresApi.getGinieAutopilotStatus();
    setAutopilotStatus(data);
    setAutopilotError(null); // Clear error on success

    // Only initialize settings from API on first load
    if (initSettings && !settingsInitialized && data.config) {
      if (data.config.min_confidence_to_trade !== undefined) {
        setConfidenceThreshold(data.config.min_confidence_to_trade);
      }
      if (data.config.risk_level) {
        setRiskLevel(data.config.risk_level);
      }
      setSettingsInitialized(true);
    }
  } catch (err: any) {
    const classified = classifyError(err, 'autopilot status');
    console.error(`[AUTOPILOT-STATUS] ${classified.technicalMessage}`, err);

    // CRITICAL FIX: Always set error state, even for 401/403
    setAutopilotError(classified.userMessage);

    // Keep last known good data (don't clear autopilotStatus)
  }
};
```

---

### Task 4: Fix fetchDiagnostics with Proper Error Handling

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Replace lines 259-269:**

```typescript
const fetchDiagnostics = async () => {
  try {
    const data = await futuresApi.getGinieDiagnostics();
    setDiagnostics(data);
    setDiagnosticsError(null); // Clear error on success
  } catch (err: any) {
    const classified = classifyError(err, 'diagnostics');
    console.error(`[DIAGNOSTICS] ${classified.technicalMessage}`, err);

    // Set error state with user-friendly message
    setDiagnosticsError(classified.userMessage);

    // Keep last known diagnostics data (don't clear)
  }
};
```

---

### Task 5: Fix Other Fetch Functions

**Apply same pattern to remaining fetch functions:**

1. `fetchCircuitBreaker` (lines 202-220)
2. `fetchModeCBStatus` (lines 223-231)
3. `fetchMarketMovers` (lines 249-257)

**Example for fetchCircuitBreaker:**

```typescript
const fetchCircuitBreaker = async () => {
  try {
    const data = await futuresApi.getGinieCircuitBreakerStatus();
    setCircuitBreaker(data);

    // ... initialization logic
  } catch (err: any) {
    const classified = classifyError(err, 'circuit breaker status');
    console.error(`[CIRCUIT-BREAKER] ${classified.technicalMessage}`, err);

    // Note: No dedicated error state for circuit breaker currently
    // Consider adding one if this section becomes critical
  }
};
```

---

### Task 6: Enhanced Error UI Component

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Add after error classification helper (before component definition):**

```typescript
// Reusable error display component
interface ErrorDisplayProps {
  error: string;
  onRetry: () => void;
  onLogin?: () => void;
  isRetrying?: boolean;
  section: string;
}

function ErrorDisplay({ error, onRetry, onLogin, isRetrying, section }: ErrorDisplayProps) {
  const isSessionExpired = error.includes('Session expired');
  const isServiceUnavailable = error.includes('Service temporarily unavailable');

  return (
    <div className="bg-red-900/20 border border-red-800 rounded-lg p-4">
      <div className="flex items-start gap-3">
        <div className="text-2xl mt-0.5">
          {isSessionExpired ? '🔒' : isServiceUnavailable ? '⚠️' : '❌'}
        </div>
        <div className="flex-1">
          <h4 className="text-red-400 font-semibold mb-1">
            {isSessionExpired ? 'Session Expired' :
             isServiceUnavailable ? 'Service Unavailable' :
             `Error Loading ${section}`}
          </h4>
          <p className="text-red-300 text-sm mb-3">{error}</p>

          <div className="flex gap-2">
            {isSessionExpired && onLogin ? (
              <button
                onClick={onLogin}
                className="px-4 py-2 bg-purple-600 hover:bg-purple-700 rounded text-white text-sm font-medium"
              >
                Log In Again
              </button>
            ) : (
              <button
                onClick={onRetry}
                disabled={isRetrying}
                className="px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:bg-gray-600 disabled:cursor-not-allowed rounded text-white text-sm font-medium"
              >
                {isRetrying ? (
                  <span className="flex items-center gap-2">
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    Retrying...
                  </span>
                ) : (
                  <span className="flex items-center gap-2">
                    <RefreshCw className="w-4 h-4" />
                    Retry
                  </span>
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
```

---

### Task 7: Update Status Error UI

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Replace lines 1053-1073 (current error display):**

```typescript
return (
  <div className="bg-gray-800 rounded-lg p-4 border border-gray-700">
    <div className="flex items-center gap-2 mb-3">
      <Sparkles className="w-5 h-5 text-purple-400" />
      <h3 className="text-lg font-semibold text-white">Ginie AI</h3>
    </div>

    {statusError ? (
      <ErrorDisplay
        error={statusError}
        onRetry={fetchStatus}
        onLogin={() => window.location.href = '/login'}
        isRetrying={loading}
        section="Ginie Status"
      />
    ) : !status ? (
      <div className="flex items-center gap-2 text-gray-400 text-sm">
        <RefreshCw className="w-4 h-4 animate-spin" />
        <span>Loading Ginie status...</span>
      </div>
    ) : (
      // ... existing status display
    )}
  </div>
);
```

---

### Task 8: Update Autopilot Error UI

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Find autopilot status display section and add error handling:**

```typescript
{/* Autopilot Status Section */}
<div className="bg-gray-800 rounded-lg p-4 border border-gray-700">
  <div className="flex items-center gap-2 mb-3">
    <Zap className="w-5 h-5 text-yellow-400" />
    <h3 className="text-lg font-semibold text-white">Autopilot</h3>
  </div>

  {autopilotError ? (
    <ErrorDisplay
      error={autopilotError}
      onRetry={() => fetchAutopilotStatus(false)}
      onLogin={() => window.location.href = '/login'}
      isRetrying={togglingAutopilot}
      section="Autopilot Status"
    />
  ) : !autopilotStatus ? (
    <div className="flex items-center gap-2 text-gray-400 text-sm">
      <RefreshCw className="w-4 h-4 animate-spin" />
      <span>Loading autopilot status...</span>
    </div>
  ) : (
    // ... existing autopilot display
  )}
</div>
```

---

### Task 9: Update Diagnostics Tab Error UI

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Replace lines 5083-5089 (diagnostics error display):**

```typescript
{activeTab === 'diagnostics' && (
  <div className="space-y-2 max-h-60 overflow-y-auto">
    {diagnosticsError ? (
      <ErrorDisplay
        error={diagnosticsError}
        onRetry={fetchDiagnostics}
        onLogin={() => window.location.href = '/login'}
        isRetrying={false}
        section="Diagnostics"
      />
    ) : !diagnostics ? (
      <div className="flex items-center gap-2 text-gray-400 text-sm p-4">
        <RefreshCw className="w-4 h-4 animate-spin" />
        <span>Loading diagnostics...</span>
      </div>
    ) : (
      // ... existing diagnostics display
    )}
  </div>
)}
```

---

### Task 10: Add Loading Timeout Detection

**File:** `/mnt/c/KOSH/binance-trading-bot/web/src/components/GiniePanel.tsx`

**Add new state for loading warnings (after line 151):**

```typescript
// Loading timeout tracking
const [loadingWarning, setLoadingWarning] = useState(false);
const [loadingTimeout, setLoadingTimeout] = useState(false);
```

**Add useEffect for timeout detection (after line 448):**

```typescript
// Detect loading timeouts
useEffect(() => {
  if (!status && !statusError) {
    // Show warning after 10 seconds
    const warningTimer = setTimeout(() => {
      setLoadingWarning(true);
    }, 10000);

    // Set error after 30 seconds
    const errorTimer = setTimeout(() => {
      if (!status && !statusError) {
        setStatusError('Request timed out after 30 seconds. The server may be slow or unreachable.');
        setLoadingTimeout(true);
      }
    }, 30000);

    return () => {
      clearTimeout(warningTimer);
      clearTimeout(errorTimer);
    };
  } else {
    setLoadingWarning(false);
    setLoadingTimeout(false);
  }
}, [status, statusError]);
```

**Update loading UI to show warning:**

```typescript
{!status && !statusError && (
  <div className="text-gray-400 text-sm">
    <div className="flex items-center gap-2 mb-2">
      <RefreshCw className="w-4 h-4 animate-spin" />
      <span>Loading Ginie status...</span>
    </div>

    {loadingWarning && (
      <div className="mt-2 p-2 bg-yellow-900/20 border border-yellow-800 rounded">
        <p className="text-yellow-400 text-xs">
          ⏳ Taking longer than usual... The server might be slow or unreachable.
        </p>
        <button
          onClick={() => {
            setStatusError('Loading cancelled by user');
            setLoadingWarning(false);
          }}
          className="mt-2 text-xs text-purple-400 hover:text-purple-300 underline"
        >
          Cancel & Retry
        </button>
      </div>
    )}
  </div>
)}
```

---

## Testing Strategy

### Test 1: Session Expiry (401/403)

**Setup:**
1. Log in to the application
2. Manually expire the token (delete from localStorage or wait for expiry)
3. Reload Ginie Panel

**Expected:**
- [ ] Panel shows "Session expired - Please log in again" message
- [ ] Lock icon (🔒) displayed
- [ ] "Log In Again" button visible
- [ ] NO infinite loading spinner
- [ ] Clicking "Log In Again" redirects to `/login`
- [ ] Error logged to console with technical details

**Test Script:**
```typescript
// In browser console
localStorage.removeItem('token'); // Expire token
window.location.reload(); // Reload page
// Expected: Session expired error shown
```

---

### Test 2: Backend Unavailable (503)

**Setup:**
1. Stop the backend server: `docker-compose down`
2. Keep frontend running
3. Load Ginie Panel

**Expected:**
- [ ] Panel shows "Service temporarily unavailable" message
- [ ] Warning icon (⚠️) displayed
- [ ] Explanation text shown
- [ ] "Retry" button visible
- [ ] NO infinite loading spinner
- [ ] Clicking "Retry" attempts to fetch again
- [ ] Error persists until backend is restored

**Test Script:**
```bash
# Terminal 1: Stop backend
cd /mnt/c/KOSH/binance-trading-bot
docker-compose down

# Terminal 2: Access frontend
# Open browser to http://localhost:3000
# Navigate to Ginie Panel
# Expected: Service unavailable error
```

---

### Test 3: Network Timeout

**Setup:**
1. Use browser DevTools to throttle network to "Slow 3G"
2. Set request timeout to 5 seconds in axios config
3. Load Ginie Panel

**Expected:**
- [ ] Loading spinner shown initially
- [ ] After 10 seconds, "Taking longer than usual" warning appears
- [ ] After 30 seconds, timeout error shown
- [ ] "Retry" button available
- [ ] Clicking retry attempts request again

---

### Test 4: Per-Section Independent Errors

**Setup:**
1. Mock API to return 503 for `/api/futures/ginie/status` only
2. Other endpoints return successfully

**Expected:**
- [ ] Ginie Status section shows error
- [ ] Autopilot section loads successfully
- [ ] Diagnostics section loads successfully
- [ ] Each section independent
- [ ] Retry button only in failed section

**Test with Mock:**
```typescript
// Mock API in futuresApi.ts for testing
export const futuresApi = {
  getGinieStatus: async () => {
    throw { response: { status: 503 } }; // Force error
  },
  getGinieAutopilotStatus: async () => {
    return { /* success data */ };
  },
  getGinieDiagnostics: async () => {
    return { /* success data */ };
  },
};
```

---

### Test 5: Retry Functionality

**Setup:**
1. Backend initially down (returns 503)
2. Load Ginie Panel - error shown
3. Start backend
4. Click "Retry"

**Expected:**
- [ ] Retry button shows "Retrying..." with spinner
- [ ] Request sent to backend
- [ ] On success, error cleared and data displayed
- [ ] No page refresh required

**Test Flow:**
```bash
# Step 1: Backend down
docker-compose down

# Step 2: Load panel - see error

# Step 3: Start backend
docker-compose up -d

# Step 4: Click Retry - should succeed
```

---

### Test 6: Loading State Timeout

**Setup:**
1. Backend responds very slowly (add delay in API)
2. Load Ginie Panel

**Expected:**
- [ ] 0-10s: Normal loading spinner with "Loading Ginie status..."
- [ ] 10-30s: Warning shown "Taking longer than usual..."
- [ ] 30s+: Error shown "Request timed out after 30 seconds"
- [ ] User can cancel and retry at any time

---

### Test 7: All Sections Error Recovery

**Setup:**
1. Backend completely down
2. All API calls fail with 503
3. Start backend
4. Retry each section

**Expected:**
- [ ] All sections show service unavailable error
- [ ] Each section has its own retry button
- [ ] After backend starts, each retry succeeds
- [ ] Sections recover independently
- [ ] No page refresh needed

---

## API Error Response Examples

### 401 Unauthorized

```json
{
  "error": "Unauthorized",
  "message": "Token expired"
}
```

### 503 Service Unavailable

```json
{
  "error": "Service Unavailable",
  "message": "Ginie service is temporarily down"
}
```

### Network Timeout (no response)

```javascript
Error: timeout of 10000ms exceeded
  code: "ECONNABORTED"
```

---

## Definition of Done

**Code Quality:**
- [ ] Error classification helper implemented
- [ ] All fetch functions use proper error handling
- [ ] No silent error returns (401/403 must set error state)
- [ ] ErrorDisplay component created and reusable

**Functionality:**
- [ ] 401/403 shows session expired error with login button
- [ ] 503 shows service unavailable error with retry button
- [ ] Network errors show appropriate message with retry
- [ ] Timeout after 30 seconds with clear error message
- [ ] Per-section independent error states
- [ ] Retry functionality works without page refresh

**UI/UX:**
- [ ] No infinite loading states
- [ ] Clear, user-friendly error messages
- [ ] Visual icons for error types (🔒, ⚠️, 🌐, ❌)
- [ ] Consistent error UI across all sections
- [ ] Loading timeout warnings after 10 seconds

**Testing:**
- [ ] All 7 test scenarios pass
- [ ] Session expiry tested and working
- [ ] Backend down scenario tested
- [ ] Network timeout tested
- [ ] Per-section errors tested
- [ ] Retry functionality verified
- [ ] Loading timeout warnings verified

**Documentation:**
- [ ] Code comments explain error handling strategy
- [ ] Console logs include context for debugging
- [ ] Error messages documented in story

---

## Approval Sign-Off

- **Scrum Master (Bob)**: Pending
- **Developer (Amelia)**: Pending
- **Test Architect (Murat)**: Pending
- **Architect (Winston)**: Pending
- **Product Manager (John)**: Pending

---

## Related Stories

- **Epic 5:** Ginie Panel Error Handling & Reliability (this story is first in the epic)
- **Story 2.7:** Mode Configuration UI (uses GiniePanel - benefits from this fix)
- **Story 2.8:** LLM & Adaptive AI Configuration (uses GiniePanel - benefits from this fix)

---

## Notes for Implementation

### Critical Issues This Fixes

1. **Infinite Loading Bug**: Lines 169, 195 return early without setting error state
2. **Silent 401/403 Handling**: Users have no idea their session expired
3. **No Retry Mechanism**: Users forced to refresh entire page
4. **503 Cascade Failure**: Backend down = entire panel broken
5. **No Timeout Protection**: Requests can hang forever

### Implementation Priority

**MUST HAVE (P0):**
- Task 1: Error classification helper
- Task 2-4: Fix fetch functions (status, autopilot, diagnostics)
- Task 6-9: Enhanced error UI for all sections

**SHOULD HAVE (P1):**
- Task 5: Fix remaining fetch functions
- Task 10: Loading timeout detection

**NICE TO HAVE (P2):**
- Auto-retry with exponential backoff
- Network status indicator
- Error rate tracking/alerting

### Testing Focus

**Critical Scenarios:**
1. Session expiry (401/403) - most common production issue
2. Backend restart/maintenance (503) - regular occurrence
3. Slow network/timeout - user with poor connection

**Edge Cases:**
- Multiple sections failing simultaneously
- Rapid retry attempts
- Token expiry during retry
- Backend partial outage (some endpoints work, others fail)

---

## Success Metrics

**Before This Story:**
- Users report "app is broken" when session expires
- Support tickets: "Ginie panel stuck loading"
- Users forced to refresh page frequently
- No visibility into backend issues

**After This Story:**
- Clear error messages for all failure scenarios
- Users can retry without page refresh
- Session expiry detected with login prompt
- Backend issues visible with service status
- Reduced support tickets related to "loading" issues

**Target Metrics:**
- 0% infinite loading states (down from ~15% of page loads)
- 100% error recovery success rate
- < 5 seconds average recovery time (retry to success)
- 90% user satisfaction with error handling
