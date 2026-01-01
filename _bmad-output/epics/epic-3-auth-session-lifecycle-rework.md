# Epic 3: Authentication Session Lifecycle & Multi-User Isolation

## Epic Overview

**Goal:** Fix critical authentication lifecycle issues where logout doesn't stop backend processes, remove all fallback mechanisms, and ensure complete multi-user isolation with proper session management.

**Business Value:** Enable secure multi-user deployment where each user's trading processes are completely isolated, logout actually stops all user resources, and no fallback mechanisms mask system failures.

**Priority:** CRITICAL - Security and resource management foundation

**Estimated Complexity:** HIGH

---

## Problem Statement

### Current Issues Discovered

| Issue | Severity | Impact |
|-------|----------|--------|
| **Logout doesn't stop user processes** | CRITICAL | Zombie trading processes, resource leaks |
| **Admin key fallback when userID empty** | CRITICAL | Wrong API keys used, security breach |
| **45+ fallback mechanisms** | HIGH | Masks failures, unpredictable behavior |
| **No pre-login API isolation** | HIGH | API calls before authentication |
| **WebSocket not reset on logout** | HIGH | Data leakage to next user |
| **Session revocation not propagated** | HIGH | Stale processes continue |
| **30-min cleanup interval too long** | MEDIUM | Resource waste |

### Root Cause Analysis

```
Current Architecture (BROKEN):
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Frontend   │ →   │  Auth API    │ →   │   Database   │
│  localStorage│     │  Revokes     │     │  Session     │
│  cleared     │     │  session     │     │  marked      │
└──────────────┘     └──────────────┘     └──────────────┘
                            ↓
                     ❌ NO CONNECTION TO:
                     - UserAutopilotManager
                     - GinieAutopilot goroutines
                     - WebSocket connections
                     - Binance client cache
```

---

## Target State

### Clean Architecture (TO BE)

```
Correct Architecture:
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Frontend   │ →   │  Auth API    │ →   │   EventBus   │
│  localStorage│     │  Publishes   │     │  user:logout │
│  cleared     │     │  event       │     │  event       │
└──────────────┘     └──────────────┘     └──────┬───────┘
                                                 │
        ┌────────────────────────────────────────┼────────────────────────────┐
        ▼                                        ▼                            ▼
┌───────────────┐                        ┌───────────────┐            ┌───────────────┐
│UserAutopilot  │                        │  WebSocket    │            │ Binance Client│
│Manager        │                        │  Hub          │            │ Factory       │
│               │                        │               │            │               │
│ Stops all     │                        │ Disconnects   │            │ Invalidates   │
│ user procs    │                        │ user conns    │            │ cached client │
└───────────────┘                        └───────────────┘            └───────────────┘
```

### Key Principles

1. **NO FALLBACK MECHANISMS** - Fail explicitly, never silently recover
2. **EVENT-DRIVEN LOGOUT** - All components subscribe to logout events
3. **COMPLETE USER ISOLATION** - Each user's resources completely separated
4. **PRE-LOGIN ISOLATION** - Zero API calls before authentication
5. **EXPLICIT ERROR HANDLING** - Every failure visible, not masked

---

## Requirements Traceability

### Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| FR-1 | Logout must stop ALL user background processes | 3.1, 3.2 |
| FR-2 | Remove ALL admin/default fallback mechanisms | 3.3 |
| FR-3 | Login page must make ZERO API calls | 3.4 |
| FR-4 | WebSocket must reset on logout | 3.1 |
| FR-5 | Multi-user: 2+ simultaneous users work correctly | 3.5 |
| FR-6 | Session revocation propagates to all components | 3.1, 3.2 |
| FR-7 | Admin user only for SMTP setup, not trading | 3.3 |
| FR-8 | Event bus publishes user:login and user:logout events | 3.2 |

### Non-Functional Requirements

| ID | Requirement | Stories |
|----|-------------|---------|
| NFR-1 | Process cleanup within 5 seconds of logout | 3.1 |
| NFR-2 | Zero resource leakage after logout | 3.1, 3.5 |
| NFR-3 | Explicit errors instead of silent fallbacks | 3.3 |
| NFR-4 | No API calls on login page render | 3.4 |
| NFR-5 | Complete user isolation in multi-user scenario | 3.5 |

---

## Story List

| Story | Title | Priority | Complexity | Dependencies |
|-------|-------|----------|------------|--------------|
| **3.1** | **Logout Stops All User Processes** | CRITICAL | HIGH | None |
| **3.2** | **Event-Driven Auth State Management** | CRITICAL | MEDIUM | 3.1 |
| **3.3** | **Remove All Fallback Mechanisms** | CRITICAL | HIGH | None |
| **3.4** | **Pre-Login API Isolation** | HIGH | LOW | None |
| **3.5** | **Multi-User Isolation Testing** | HIGH | MEDIUM | 3.1, 3.2, 3.3 |
| **3.6** | **Integration Testing & Validation** | HIGH | LOW | All |

---

## Story 3.1: Logout Stops All User Processes

**As a** user,
**I want** all my background processes to stop when I logout,
**So that** no orphan trading processes continue running.

### Acceptance Criteria

**AC-3.1.1: Autopilot Stops on Logout**
- **Given** a user with Ginie autopilot running
- **When** they click logout
- **Then** their GinieAutopilot.Stop() is called within 5 seconds
- **And** all 13+ goroutines for that user are terminated
- **And** no trading actions occur after logout

**AC-3.1.2: WebSocket Cleanup on Logout**
- **Given** a user with active WebSocket connections
- **When** they logout
- **Then** all their WebSocket connections are closed
- **And** frontend wsService.reset() is called
- **And** no data from previous session visible to next user

**AC-3.1.3: Client Cache Invalidation**
- **Given** a user with cached Binance client
- **When** they logout
- **Then** their client is removed from cache
- **And** next login creates fresh client

**AC-3.1.4: Session Database Cleanup**
- **Given** a user logging out
- **When** logout completes
- **Then** session is revoked in database
- **And** all related tokens are invalidated

### Tasks

- [ ] Task 3.1.1: Add userID parameter to logout handler
- [ ] Task 3.1.2: Call UserAutopilotManager.StopAutopilot(userID) in logout
- [ ] Task 3.1.3: Call ClientFactory.InvalidateUserClients(userID) in logout
- [ ] Task 3.1.4: Add wsService.reset() call in frontend logout
- [ ] Task 3.1.5: Add WebSocket disconnect for user in backend
- [ ] Task 3.1.6: Add timeout protection (5 second max)
- [ ] Task 3.1.7: Add logging for all cleanup steps

### Technical Notes

**Modified Logout Handler (handlers.go):**
```go
func (h *Handlers) Logout(c *gin.Context) {
    userID := auth.GetUserID(c)
    var req struct { RefreshToken string `json:"refresh_token"` }
    c.ShouldBindJSON(&req)

    // 1. Stop user's autopilot processes
    if h.userAutopilotManager != nil && userID != "" {
        h.userAutopilotManager.StopAutopilot(userID)
    }

    // 2. Invalidate cached Binance clients
    if h.clientFactory != nil && userID != "" {
        h.clientFactory.InvalidateUserClients(userID)
    }

    // 3. Disconnect user WebSocket connections
    if h.wsHub != nil && userID != "" {
        h.wsHub.DisconnectUser(userID)
    }

    // 4. Revoke session in database
    h.service.Logout(c.Request.Context(), req.RefreshToken)

    c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}
```

**Frontend Logout (AuthContext.tsx):**
```typescript
const logout = useCallback(async () => {
    const { refreshToken } = getStoredTokens();
    try {
        if (refreshToken) {
            await api.post('/auth/logout', { refresh_token: refreshToken });
        }
    } catch {
        // Ignore logout errors
    } finally {
        // CRITICAL: Reset WebSocket before clearing tokens
        wsService.reset();
        clearStoredTokens();
        setUser(null);
    }
}, []);
```

---

## Story 3.2: Event-Driven Auth State Management

**As a** system,
**I want** auth state changes published via EventBus,
**So that** all components can react to login/logout events.

### Acceptance Criteria

**AC-3.2.1: Logout Event Published**
- **Given** user logs out
- **When** logout handler executes
- **Then** `user:logout` event is published to EventBus
- **And** event contains userID

**AC-3.2.2: Components Subscribe to Events**
- **Given** UserAutopilotManager, WebSocketHub, ClientFactory
- **When** they initialize
- **Then** they subscribe to `user:logout` event
- **And** execute cleanup on event receipt

**AC-3.2.3: Login Event Published**
- **Given** user logs in successfully
- **When** login completes
- **Then** `user:login` event is published
- **And** components can initialize user resources

**AC-3.2.4: Event Timeout Protection**
- **Given** event handler takes too long
- **When** timeout exceeded (5s)
- **Then** processing continues
- **And** warning logged

### Tasks

- [ ] Task 3.2.1: Add `user:login` and `user:logout` events to EventBus
- [ ] Task 3.2.2: Publish events from auth handlers
- [ ] Task 3.2.3: Subscribe UserAutopilotManager to logout event
- [ ] Task 3.2.4: Subscribe WebSocketHub to logout event
- [ ] Task 3.2.5: Subscribe ClientFactory to logout event
- [ ] Task 3.2.6: Add timeout protection for event handlers
- [ ] Task 3.2.7: Add event logging and metrics

### Technical Notes

**EventBus Events (main.go or events.go):**
```go
const (
    EventUserLogin  = "user:login"
    EventUserLogout = "user:logout"
)

type UserAuthEvent struct {
    UserID    string
    Email     string
    Timestamp time.Time
}
```

**Subscription Setup (main.go):**
```go
// Subscribe components to auth events
eventBus.Subscribe(EventUserLogout, func(event interface{}) {
    if authEvent, ok := event.(UserAuthEvent); ok {
        // Stop user's autopilot
        userAutopilotManager.StopAutopilot(authEvent.UserID)
        // Invalidate client cache
        clientFactory.InvalidateUserClients(authEvent.UserID)
        // Disconnect WebSocket
        wsHub.DisconnectUser(authEvent.UserID)
    }
})
```

---

## Story 3.3: Remove All Fallback Mechanisms

**As a** developer,
**I want** all fallback mechanisms removed,
**So that** failures are explicit and not masked.

### Acceptance Criteria

**AC-3.3.1: Admin Key Fallback Removed**
- **Given** apikeys/service.go GetActiveBinanceKey() or GetActiveAIKey()
- **When** userID is empty
- **Then** return error "user ID required"
- **And** DO NOT fall back to admin user's keys

**AC-3.3.2: Default Encryption Key Removed**
- **Given** ENCRYPTION_KEY env variable not set
- **When** encryption is attempted
- **Then** return error "encryption key not configured"
- **And** DO NOT use hardcoded default key

**AC-3.3.3: Market Movers Fallback Removed**
- **Given** user coin sources lookup fails
- **When** error occurs
- **Then** return error to caller
- **And** DO NOT silently fall back to market movers

**AC-3.3.4: LIMIT to MARKET Fallback Removed**
- **Given** LIMIT order fails
- **When** error occurs
- **Then** return error to caller
- **And** DO NOT automatically retry as MARKET order

**AC-3.3.5: Global Mode Fallback Removed**
- **Given** trading mode lookup for user fails
- **When** error occurs
- **Then** return error
- **And** DO NOT fall back to global mode

### Tasks

- [ ] Task 3.3.1: Remove admin fallback from apikeys/service.go (lines 91-98, 132-139)
- [ ] Task 3.3.2: Remove default encryption key (line 32)
- [ ] Task 3.3.3: Remove market movers fallback in ginie_autopilot.go
- [ ] Task 3.3.4: Remove LIMIT→MARKET fallback
- [ ] Task 3.3.5: Remove global mode fallback in handlers_settings.go
- [ ] Task 3.3.6: Update error messages to be descriptive
- [ ] Task 3.3.7: Add validation at startup for required config
- [ ] Task 3.3.8: Document all removed fallbacks

### Technical Notes

**Before (apikeys/service.go):**
```go
func (s *Service) GetActiveBinanceKey(ctx context.Context, userID string) (*BinanceKeyResult, error) {
    if userID == "" {
        userID = s.adminUserID  // ❌ FALLBACK - REMOVE THIS
    }
    // ...
}
```

**After:**
```go
func (s *Service) GetActiveBinanceKey(ctx context.Context, userID string) (*BinanceKeyResult, error) {
    if userID == "" {
        return nil, errors.New("user ID required: cannot retrieve API keys without authenticated user")
    }
    // ...
}
```

---

## Story 3.4: Pre-Login API Isolation

**As a** user,
**I want** the login page to make zero API calls,
**So that** no backend connections exist before authentication.

### Acceptance Criteria

**AC-3.4.1: Login Page No API Calls**
- **Given** unauthenticated user on login page
- **When** page renders
- **Then** ZERO API calls are made
- **And** no "checking auth status" calls
- **And** no WebSocket connections attempted

**AC-3.4.2: Register Page No API Calls**
- **Given** unauthenticated user on register page
- **When** page renders
- **Then** ZERO API calls are made (until form submit)

**AC-3.4.3: API Status Check Removed**
- **Given** AuthContext initialization
- **When** no token in localStorage
- **Then** skip /auth/status check
- **And** immediately show login page

**AC-3.4.4: WebSocket Delayed Until Login**
- **Given** WebSocket service
- **When** user not authenticated
- **Then** no connection attempt made
- **When** user logs in
- **Then** WebSocket connects

### Tasks

- [ ] Task 3.4.1: Remove /auth/status call when no token present
- [ ] Task 3.4.2: Delay WebSocket connection until after login
- [ ] Task 3.4.3: Remove any pre-auth market data fetching
- [ ] Task 3.4.4: Verify network tab shows zero requests on login page
- [ ] Task 3.4.5: Add conditional auth check only when token exists

### Technical Notes

**Before (AuthContext.tsx):**
```typescript
useEffect(() => {
    // Called on every page load, even login page
    api.get('/auth/status').then(...)  // ❌ REMOVE
    const token = getStoredTokens().accessToken;
    if (token) {
        api.get('/auth/me').then(...)
    }
}, []);
```

**After:**
```typescript
useEffect(() => {
    const { accessToken } = getStoredTokens();

    // Only check auth if we have a token
    if (!accessToken) {
        setIsLoading(false);
        return; // No API calls for unauthenticated users
    }

    // Validate existing token
    api.get('/auth/me')
        .then(response => setUser(response.data))
        .catch(() => clearStoredTokens())
        .finally(() => setIsLoading(false));
}, []);
```

---

## Story 3.5: Multi-User Isolation Testing

**As a** system administrator,
**I want** verified multi-user isolation,
**So that** two users can trade simultaneously without interference.

### Acceptance Criteria

**AC-3.5.1: Simultaneous Login**
- **Given** two users (User A and User B)
- **When** both login simultaneously
- **Then** each gets separate session
- **And** each gets separate autopilot instance
- **And** each uses their own API keys

**AC-3.5.2: Isolated Trading**
- **Given** User A and User B both trading
- **When** User A places an order
- **Then** order uses User A's Binance keys
- **And** User B's positions are not affected

**AC-3.5.3: Isolated Logout**
- **Given** User A and User B both logged in
- **When** User A logs out
- **Then** User A's autopilot stops
- **And** User B's autopilot continues
- **And** User B's session unaffected

**AC-3.5.4: Session Isolation**
- **Given** User A on Device 1 and Device 2
- **When** User A logs out on Device 1
- **Then** Device 1 session revoked
- **And** Device 2 session still active

**AC-3.5.5: No Cross-User Data Leakage**
- **Given** User A logs out and User B logs in on same browser
- **When** User B accesses the app
- **Then** no data from User A visible
- **And** no cached clients from User A used

### Tasks

- [ ] Task 3.5.1: Create multi-user test scenario script
- [ ] Task 3.5.2: Test simultaneous login from different browsers
- [ ] Task 3.5.3: Test order placement with different users
- [ ] Task 3.5.4: Test logout isolation
- [ ] Task 3.5.5: Test session isolation across devices
- [ ] Task 3.5.6: Verify no cross-user data leakage
- [ ] Task 3.5.7: Document test results

### Technical Notes

**Test Scenario:**
```
1. Open Chrome Incognito - Login as User A
2. Open Firefox Private - Login as User B
3. User A: Start Ginie autopilot
4. User B: Start Ginie autopilot
5. Verify both autopilots running independently
6. User A: Logout
7. Verify User A autopilot stopped
8. Verify User B autopilot still running
9. User B: Place test order
10. Verify order uses User B's API keys
```

---

## Story 3.6: Integration Testing & Validation

**As a** developer,
**I want** comprehensive integration tests for auth lifecycle,
**So that** we can deploy with confidence.

### Acceptance Criteria

**AC-3.6.1: Logout Cleanup Verified**
- **Given** complete logout test
- **When** executed
- **Then** all processes stopped
- **And** all caches cleared
- **And** all connections closed

**AC-3.6.2: No Fallback Verification**
- **Given** missing userID in API calls
- **When** executed
- **Then** explicit error returned
- **And** no admin fallback used

**AC-3.6.3: Multi-User Stress Test**
- **Given** 5 simultaneous users
- **When** all trading
- **Then** complete isolation maintained
- **And** no resource contention

### Tasks

- [ ] Task 3.6.1: Create logout cleanup test
- [ ] Task 3.6.2: Create no-fallback verification test
- [ ] Task 3.6.3: Create multi-user stress test
- [ ] Task 3.6.4: Document all test scenarios
- [ ] Task 3.6.5: Create CI pipeline for auth tests

---

## Fallback Mechanisms to Remove

### Critical (Must Remove)

| Location | Line | Current Behavior | Action |
|----------|------|------------------|--------|
| `apikeys/service.go` | 91-98 | Falls back to admin AI key | Return error |
| `apikeys/service.go` | 132-139 | Falls back to admin Binance key | Return error |
| `apikeys/service.go` | 32 | Hardcoded default encryption key | Require config |
| `handlers_settings.go` | 60-86 | Falls back to global trading mode | Return error |

### High Priority (Should Remove)

| Location | Line | Current Behavior | Action |
|----------|------|------------------|--------|
| `ginie_autopilot.go` | 1473-1479 | Falls back to market movers | Return error |
| `ginie_autopilot.go` | 3501-3519 | LIMIT→MARKET fallback | Return error |
| `apikeys/service.go` | 222-227 | Admin fallback in HasActiveAIKey | Return false |
| `apikeys/service.go` | 245-250 | Admin fallback in HasActiveBinanceKey | Return false |

### Medium Priority (Consider)

| Location | Line | Current Behavior | Action |
|----------|------|------------------|--------|
| `ginie_autopilot.go` | Various | Mode config fallback values | Use strict config |
| `futuresStore.ts` | 5, 274 | Fallback symbols list | Show error instead |

---

## Implementation Order

```
Story 3.3 (Remove fallbacks) ← Do First - Foundation
    ↓
Story 3.4 (Pre-login isolation) ← Quick Win
    ↓
Story 3.1 (Logout stops processes) ← Core Fix
    ↓
Story 3.2 (Event-driven auth) ← Architecture
    ↓
Story 3.5 (Multi-user testing) ← Validation
    ↓
Story 3.6 (Integration testing) ← Final Verification
```

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing functionality | MEDIUM | HIGH | Careful testing, rollback plan |
| User confusion on new errors | HIGH | LOW | Clear error messages |
| Performance impact from events | LOW | MEDIUM | Async event handling |
| Incomplete fallback removal | MEDIUM | HIGH | Systematic code review |

---

## Definition of Done

- [ ] Logout stops all user processes within 5 seconds
- [ ] Zero fallback mechanisms in codebase
- [ ] Login page makes zero API calls
- [ ] WebSocket reset on logout
- [ ] Multi-user isolation verified with 2+ users
- [ ] All auth events published via EventBus
- [ ] Explicit errors for missing user context
- [ ] Integration tests passing
- [ ] Docker rebuild successful
- [ ] No TypeScript or Go compilation errors

---

## Architecture Summary

### Before (Current - Broken)

```
User Logout → Frontend clears tokens → Backend revokes session
                                              ↓
                                    ❌ Processes keep running
                                    ❌ WebSocket stays connected
                                    ❌ Cached clients remain
                                    ❌ 30-min until cleanup
```

### After (Target - Fixed)

```
User Logout → Frontend clears tokens + resets WebSocket
                    ↓
            Backend receives logout
                    ↓
            Publishes user:logout event
                    ↓
    ┌───────────────┼───────────────┐
    ↓               ↓               ↓
Autopilot       WebSocket       Client
Stops           Disconnects     Invalidated
    ↓               ↓               ↓
    └───────────────┴───────────────┘
                    ↓
            Session revoked in DB
                    ↓
            ✅ Complete cleanup < 5 seconds
```

---

## Notes

**Key Principle:** Fail Loudly, Never Silently

When something goes wrong:
- Return explicit error to caller
- Log the error with context
- Never substitute with default/fallback values
- Let the user/system know something is wrong

**Admin User Purpose:**
- ONLY for SMTP configuration
- ONLY for encryption key setup
- NOT for trading
- NOT for API key fallback
