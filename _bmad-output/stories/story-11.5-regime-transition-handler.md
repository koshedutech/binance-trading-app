# Story 11.5: Regime Transition Handler

**Epic:** 11 - Position Decision Engine
**Priority:** P1
**Status:** done
**Created:** 2026-01-18

## Goal

Handle transitions between market regimes gracefully. The handler confirms regime changes over multiple candles to prevent whipsaw trading, notifies active positions of regime changes, and logs transitions for analysis.

## Acceptance Criteria

- [x] Detect regime changes with confirmation (avoid whipsaws)
- [x] Minimum time in regime before allowing transition
- [x] Notify active positions of regime changes
- [x] Log regime transitions for analysis
- [x] Optional alerts for significant regime changes

## Implementation Tasks

### Task 1: Create Transition Handler (regime_transition.go)
- Create TransitionHandler struct with configuration
- Implement TransitionConfig for confirmation candles, min duration, whipsaw window
- Implement PendingTransition struct for tracking pending changes
- Implement TransitionListener interface for notifications

### Task 2: Implement Whipsaw Prevention
- Track pending transitions per symbol
- Require N consecutive candles confirming new regime
- Cancel pending if regime reverts during confirmation

### Task 3: Implement Notification System
- Simple TransitionListener interface
- Callback on confirmed transitions
- Optional logging of transitions

### Task 4: Write Unit Tests (regime_transition_test.go)
- TestTransitionHandler_ConfirmedTransition
- TestTransitionHandler_WhipsawPrevention
- TestTransitionHandler_MinDuration
- TestTransitionHandler_Notifications
- TestTransitionHandler_ConcurrentAccess

## Technical Design

### TransitionHandler Structure

```go
type TransitionHandler struct {
    classifier     *RegimeClassifier
    config         *TransitionConfig
    pendingChanges map[string]*PendingTransition  // symbol -> pending
    pendingMu      sync.RWMutex
    listeners      []TransitionListener
    listenerMu     sync.RWMutex
}
```

### TransitionConfig

```go
type TransitionConfig struct {
    ConfirmationCandles  int           // Default: 3 candles to confirm
    MinRegimeDuration    time.Duration // Min time before transition allowed
    WhipsawWindow        time.Duration // Window to detect whipsaw
    EnableNotifications  bool          // Alert on transitions
    EnableLogging        bool          // Log transitions
}
```

### Confirmation Process

1. Classifier detects regime change
2. TransitionHandler creates PendingTransition
3. Subsequent candles must confirm the new regime
4. After N candles of confirmation, transition is confirmed
5. If any candle reverts to original regime, pending is cancelled

### Default Configuration

- ConfirmationCandles: 3
- MinRegimeDuration: 15 minutes
- WhipsawWindow: 30 minutes
- EnableNotifications: true
- EnableLogging: true

## Files to Create

1. `internal/decision/regime_transition.go`
2. `internal/decision/regime_transition_test.go`

## Dependencies

- `internal/decision/regime_classifier.go` - RegimeClassifier and RegimeChange
- `internal/decision/coin_state.go` - CoinState and MarketRegime

## Notes

- Thread-safe with sync.RWMutex for concurrent access
- Pending transitions stored in memory (not Redis)
- Listeners are notified asynchronously to avoid blocking
- Whipsaw detection uses both candle count and time window

---

## Change Log

| Date | Status | Notes |
|------|--------|-------|
| 2026-01-18 | in-progress | Story created and implementation started |
| 2026-01-18 | done | Implementation complete - all 24 tests pass |
