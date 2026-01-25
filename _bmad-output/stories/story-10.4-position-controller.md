# Story 10.4: Position Controller - Exit Signal Executor

## Story Overview

**Epic**: Epic 10 - Position Management & Optimization
**Story ID:** POS-10.4
**Story Type**: Feature Implementation
**Priority**: P1 (High)
**Complexity**: Medium
**Status**: Done
**Created:** 2026-01-25
**Last Updated:** 2026-01-25
**Implemented:** 2026-01-25

---

## Executive Summary

This story implements a **Position Controller** - a lightweight component that:
1. **Subscribes to Exit Decision Service signals** (from Story 10.1/10.3)
2. **Executes SL/TP order updates on Binance** when signals arrive
3. **Provides protection heal** - ensures SL/TP orders exist on Binance
4. **Uses Chain system settings** - not old Ginie mode settings
5. **Enables complete disabling of Ginie autopilot** for position management

This is the **missing piece** that connects Exit Decision (brain) to Binance (execution).

---

## Problem Statement

### Current Architecture Gap

```
Exit Decision Service (Epic 10)
        │
        │ Generates signals:
        │  - "Trail SL to $X"
        │  - "Move to breakeven"
        │  - "Efficiency declining"
        │  - "Trend reversal"
        │
        ▼
    [NOTHING] ← Gap - signals are stored but not acted upon
        │
        ▼
    Binance (SL/TP orders unchanged)
```

### Current Workaround (Problem)

Ginie's position management currently handles SL/TP updates, but:
- Uses **old mode settings** (not chain system settings)
- Has **complex AI agents** (not needed for simple updates)
- Has **learning engine** (overhead for basic operations)
- Cannot be disabled without losing position protection

### Solution

A simple **Position Controller** that:
1. Consumes Exit Decision signals
2. Calls Binance API to update orders
3. No AI, no learning - just execution

---

## Architecture

### New Data Flow

```
Exit Decision Service (Epic 10.1)
        │
        │ Generates ExitSignal:
        │  - Symbol, UserID
        │  - ExitType (TRAIL_SL, BREAKEVEN, EFFICIENCY, TREND_REVERSAL)
        │  - NewSLPrice, NewTPPrice
        │  - Urgency (immediate, normal, monitor)
        │
        ▼
Position Controller (This Story)
        │
        │ Subscribes via SignalHandler
        │ Receives signal → Validates → Executes
        │
        ├── TRAIL_SL → Cancel old SL → Place new SL at signal.NewSLPrice
        ├── UPDATE_TP → Cancel old TP → Place new TP at signal.NewTPPrice
        ├── BREAKEVEN → Move SL to entry price + fees
        ├── PROTECTION_HEAL → Ensure SL/TP orders exist
        │
        ▼
    Binance Futures API
        │
        │ Orders updated on exchange
        │
        ▼
    Position Protected
```

### Component Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                    Chain Trading System                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Chain Entry Runner (Epic 14)                                   │
│       │                                                         │
│       │ Places entry + initial SL/TP                           │
│       ▼                                                         │
│  Order Chain Created                                            │
│       │                                                         │
│       │ Position opened on Binance                             │
│       ▼                                                         │
│  Exit Decision Service (Epic 10.1)                             │
│       │                                                         │
│       │ Monitors position, generates signals                   │
│       ▼                                                         │
│  Position Controller (This Story) ◄─────── NEW                 │
│       │                                                         │
│       │ Executes SL/TP updates on Binance                      │
│       ▼                                                         │
│  Position Closed (SL or TP hit on Binance)                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Design

### Core Interface

```go
// PositionController executes exit signals on Binance
type PositionController struct {
    // Dependencies
    futuresClient    binance.FuturesClient
    exitDecisionSvc  *exitdecision.Service
    chainEventWriter orders.ChainEventWriter

    // Configuration
    config           *PositionControllerConfig
    userID           string

    // State
    running          bool
    mu               sync.RWMutex

    // Control
    ctx              context.Context
    cancelFunc       context.CancelFunc
    wg               sync.WaitGroup
}

type PositionControllerConfig struct {
    // Execution settings
    MaxRetries           int           `json:"max_retries"`          // Default: 3
    RetryDelay           time.Duration `json:"retry_delay"`          // Default: 1s
    OrderTimeout         time.Duration `json:"order_timeout"`        // Default: 10s

    // Protection heal settings
    HealCheckInterval    time.Duration `json:"heal_interval"`        // Default: 30s
    EnableProtectionHeal bool          `json:"enable_heal"`          // Default: true

    // Signal processing
    SignalBatchSize      int           `json:"batch_size"`           // Default: 10
    ProcessingInterval   time.Duration `json:"process_interval"`     // Default: 100ms
}
```

### Signal Handler

```go
// handleExitSignal processes an exit signal from Exit Decision Service
func (pc *PositionController) handleExitSignal(signal exitdecision.ExitSignal) {
    log.Printf("[POS-CTRL] Received signal: %s %s (urgency=%s)",
        signal.Symbol, signal.ExitType, signal.Urgency)

    switch signal.ExitType {
    case exitdecision.ExitTypeTrailSL:
        pc.executeTrailingSL(signal)

    case exitdecision.ExitTypeBreakeven:
        pc.executeMoveToBreakeven(signal)

    case exitdecision.ExitTypeUpdateTP:
        pc.executeUpdateTP(signal)

    case exitdecision.ExitTypeTrendReversal:
        pc.executeTrendReversalExit(signal)

    case exitdecision.ExitTypeEfficiency:
        pc.executeEfficiencyExit(signal)
    }
}
```

### SL/TP Update Logic

```go
// executeTrailingSL updates the stop loss order on Binance
func (pc *PositionController) executeTrailingSL(signal exitdecision.ExitSignal) error {
    ctx, cancel := context.WithTimeout(pc.ctx, pc.config.OrderTimeout)
    defer cancel()

    // Step 1: Get current position info
    position, err := pc.getPosition(ctx, signal.Symbol)
    if err != nil {
        return fmt.Errorf("failed to get position: %w", err)
    }

    // Step 2: Validate new SL is an improvement
    if !pc.isValidSLUpdate(position, signal.NewSLPrice) {
        log.Printf("[POS-CTRL] SL update rejected: not an improvement")
        return nil
    }

    // Step 3: Cancel existing SL order
    if position.SLOrderID != "" {
        err = pc.cancelOrder(ctx, signal.Symbol, position.SLOrderID)
        if err != nil {
            log.Printf("[POS-CTRL] Warning: failed to cancel old SL: %v", err)
            // Continue anyway - will place new order
        }
    }

    // Step 4: Place new SL order
    newOrderID, err := pc.placeStopLossOrder(ctx, signal.Symbol, position.Side,
        position.Quantity, signal.NewSLPrice)
    if err != nil {
        return fmt.Errorf("failed to place new SL: %w", err)
    }

    // Step 5: Update chain event (for tracking)
    if pc.chainEventWriter != nil {
        pc.chainEventWriter.RecordSLUpdate(ctx, orders.SLUpdateEvent{
            ChainID:    position.ChainID,
            Symbol:     signal.Symbol,
            OldSL:      position.SLPrice,
            NewSL:      signal.NewSLPrice,
            Reason:     string(signal.ExitType),
            OrderID:    newOrderID,
        })
    }

    log.Printf("[POS-CTRL] SL updated: %s %.4f → %.4f (order=%s)",
        signal.Symbol, position.SLPrice, signal.NewSLPrice, newOrderID)

    return nil
}

// isValidSLUpdate checks if the new SL price is an improvement
func (pc *PositionController) isValidSLUpdate(pos *PositionInfo, newSL float64) bool {
    if pos.Side == "LONG" {
        // For LONG, new SL must be higher (trailing up)
        return newSL > pos.SLPrice
    }
    // For SHORT, new SL must be lower (trailing down)
    return newSL < pos.SLPrice
}
```

### Protection Heal

```go
// runProtectionHealLoop ensures SL/TP orders exist for all positions
func (pc *PositionController) runProtectionHealLoop() {
    ticker := time.NewTicker(pc.config.HealCheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-pc.ctx.Done():
            return
        case <-ticker.C:
            pc.healAllPositions()
        }
    }
}

// healAllPositions checks and fixes missing SL/TP orders
func (pc *PositionController) healAllPositions() {
    ctx, cancel := context.WithTimeout(pc.ctx, 30*time.Second)
    defer cancel()

    // Get all chain positions for this user
    positions, err := pc.getChainPositions(ctx)
    if err != nil {
        log.Printf("[POS-CTRL] Heal: failed to get positions: %v", err)
        return
    }

    for _, pos := range positions {
        // Check SL exists
        if pos.SLOrderID == "" || !pc.orderExists(ctx, pos.Symbol, pos.SLOrderID) {
            log.Printf("[POS-CTRL] Heal: Missing SL for %s, placing...", pos.Symbol)
            pc.placeProtectionSL(ctx, pos)
        }

        // Check TP exists
        if pos.TPOrderID == "" || !pc.orderExists(ctx, pos.Symbol, pos.TPOrderID) {
            log.Printf("[POS-CTRL] Heal: Missing TP for %s, placing...", pos.Symbol)
            pc.placeProtectionTP(ctx, pos)
        }
    }
}

// placeProtectionSL places a fallback SL if missing
func (pc *PositionController) placeProtectionSL(ctx context.Context, pos *PositionInfo) error {
    // Get SL price from chain settings (not old Ginie settings)
    slPercent := pc.getChainSLPercent(pos.ChainID)

    var slPrice float64
    if pos.Side == "LONG" {
        slPrice = pos.EntryPrice * (1 - slPercent/100)
    } else {
        slPrice = pos.EntryPrice * (1 + slPercent/100)
    }

    orderID, err := pc.placeStopLossOrder(ctx, pos.Symbol, pos.Side, pos.Quantity, slPrice)
    if err != nil {
        return err
    }

    log.Printf("[POS-CTRL] Heal: Placed protection SL for %s at %.4f (order=%s)",
        pos.Symbol, slPrice, orderID)

    return nil
}
```

---

## Integration with Exit Decision Service

### Subscription Setup

```go
// Start starts the position controller
func (pc *PositionController) Start(ctx context.Context) error {
    pc.mu.Lock()
    if pc.running {
        pc.mu.Unlock()
        return fmt.Errorf("position controller already running")
    }
    pc.running = true
    pc.ctx, pc.cancelFunc = context.WithCancel(ctx)
    pc.mu.Unlock()

    // Subscribe to exit decision signals
    if pc.exitDecisionSvc != nil {
        pc.exitDecisionSvc.SubscribeToSignals(pc.handleExitSignal)
        log.Printf("[POS-CTRL] Subscribed to Exit Decision signals")
    }

    // Start protection heal loop
    if pc.config.EnableProtectionHeal {
        pc.wg.Add(1)
        go func() {
            defer pc.wg.Done()
            pc.runProtectionHealLoop()
        }()
    }

    log.Printf("[POS-CTRL] Position Controller started for user %s", pc.userID)
    return nil
}
```

### Signal Types from Exit Decision Service

| Signal Type | Action | Priority |
|-------------|--------|----------|
| `TRAIL_SL` | Update SL to new price (trailing) | Normal |
| `BREAKEVEN` | Move SL to entry + fees | Normal |
| `UPDATE_TP` | Update TP to new price | Normal |
| `TREND_REVERSAL` | Tighten SL immediately | Immediate |
| `EFFICIENCY` | Tighten SL based on efficiency | Normal |
| `PROTECTION_HEAL` | Ensure orders exist | Background |

---

## Settings (Chain System Based)

### Not Using Old Ginie Settings

The Position Controller uses **chain system settings**, not old mode settings:

```go
// getChainSLPercent gets SL% from chain settings, not Ginie mode settings
func (pc *PositionController) getChainSLPercent(chainID string) float64 {
    // Get from order_chains table or strategy configuration
    chain, err := pc.chainEventWriter.GetChain(pc.ctx, chainID)
    if err != nil {
        return 1.5 // Default fallback
    }

    // Use strategy-based SL from chain
    return chain.SLPercent
}

// getChainTPPercent gets TP% from chain settings
func (pc *PositionController) getChainTPPercent(chainID string) float64 {
    chain, err := pc.chainEventWriter.GetChain(pc.ctx, chainID)
    if err != nil {
        return 2.0 // Default fallback
    }

    return chain.TPPercent
}
```

### Why This Matters

| Aspect | Old Ginie | New Position Controller |
|--------|-----------|------------------------|
| SL % | From mode config (scalp: 1.5%, swing: 3%) | From chain/strategy config |
| TP % | From mode config | From chain/strategy config |
| Settings source | User mode settings table | Order chain record |
| Problem | XRPUSDT used wrong mode | Uses exact settings from entry |

---

## Integration into UserAutopilotManager

### Adding Position Controller

```go
// UserAutopilotInstance includes Position Controller
type UserAutopilotInstance struct {
    UserID              string
    FuturesClient       binance.FuturesClient
    LLMAnalyzer         *llm.Analyzer
    Autopilot           *GinieAutopilot              // OLD - will be disabled
    ChainEntryRunner    *ChainEntryRunner
    CoinProfiler        *coinprofiler.CoinProfiler
    ExitDecisionService *exitdecision.Service
    PositionController  *PositionController          // NEW - This story
    CreatedAt           time.Time
    LastActive          time.Time
}

// CreateInstance now includes Position Controller
func (m *UserAutopilotManager) createInstance(ctx context.Context, userID string) (*UserAutopilotInstance, error) {
    // ... existing code ...

    // Create Position Controller
    posController := NewPositionController(
        futuresClient,
        exitDecisionSvc,
        chainEventWriter,
        &PositionControllerConfig{
            MaxRetries:           3,
            RetryDelay:           time.Second,
            OrderTimeout:         10 * time.Second,
            HealCheckInterval:    30 * time.Second,
            EnableProtectionHeal: true,
        },
        userID,
    )

    instance := &UserAutopilotInstance{
        // ... existing fields ...
        PositionController: posController,
    }

    return instance, nil
}
```

### Startup Order

```
1. CoinProfiler.Start()       - Price data collection
2. ExitDecisionService.Start() - Signal generation
3. PositionController.Start()  - Signal execution ← NEW
4. ChainEntryRunner.Start()    - Entry placement
5. GinieAutopilot.Start()      - DISABLED when chain system active
```

---

## Disabling Ginie Position Management

### When entry_decision_system = "chain"

```go
// startUserServices starts appropriate services based on entry_decision_system
func (m *UserAutopilotManager) startUserServices(ctx context.Context, userID string) error {
    settings, _ := m.getUserSettings(ctx, userID)

    if settings.EntryDecisionSystem == "chain" {
        // NEW CHAIN SYSTEM
        m.StartCoinProfiler(ctx, userID)
        m.StartExitDecisionService(ctx, userID)
        m.StartPositionController(ctx, userID)  // NEW
        m.StartChainEntryRunner(ctx, userID)

        // DO NOT start Ginie autopilot for position management
        // Ginie's monitoring loop is completely bypassed
        log.Printf("[USER-AUTOPILOT] Chain system active - Ginie position management DISABLED")

    } else {
        // LEGACY SYSTEM
        m.StartGinieAutopilot(ctx, userID)
    }

    return nil
}
```

### What Gets Disabled

| Ginie Feature | Status When Chain Active |
|---------------|-------------------------|
| Entry analysis (GinieAnalyzer) | DISABLED - Chain Entry Runner handles |
| Entry placement | DISABLED - Chain Entry Runner handles |
| Position monitoring | DISABLED - Exit Decision Service handles |
| SL/TP updates | DISABLED - Position Controller handles |
| Protection heal | DISABLED - Position Controller handles |
| AI agents | DISABLED - Not needed |
| Learning engine | DISABLED - Not needed |

---

## API Endpoints

### Get Position Controller Status

```go
// GET /api/futures/position-controller/status
type PositionControllerStatus struct {
    Running              bool      `json:"running"`
    StartedAt            time.Time `json:"started_at"`
    SignalsProcessed     int64     `json:"signals_processed"`
    SLUpdatesExecuted    int64     `json:"sl_updates"`
    TPUpdatesExecuted    int64     `json:"tp_updates"`
    HealActionsExecuted  int64     `json:"heal_actions"`
    LastSignalTime       time.Time `json:"last_signal_time"`
    LastError            string    `json:"last_error,omitempty"`
}
```

### Manual Heal Trigger

```go
// POST /api/futures/position-controller/heal
// Triggers immediate protection heal check
```

---

## Database Changes

### None Required

The Position Controller uses existing tables:
- `order_chains` - For chain/strategy settings
- `order_chain_events` - For recording SL/TP updates

No new migrations needed.

---

## Implementation Tasks

### Task 1: Core Position Controller
- [x] Create `internal/autopilot/position_controller.go`
- [x] Implement PositionController struct
- [x] Implement Start/Stop methods
- [x] Implement signal subscription

### Task 2: Signal Handlers
- [x] Implement handleExitSignal dispatcher
- [x] Implement executeTrailingSL
- [x] Implement executeMoveToBreakeven
- [x] Implement executeUpdateTP
- [x] Implement executeTrendReversalExit
- [x] Implement executeEfficiencyExit

### Task 3: Binance Order Operations
- [x] Implement cancelOrder (cancelOrderWithRetry)
- [x] Implement placeStopLossOrder
- [x] Implement placeTakeProfitOrder
- [x] Implement orderExists check
- [x] Add retry logic with exponential backoff

### Task 4: Protection Heal
- [x] Implement runProtectionHealLoop
- [x] Implement healAllPositions
- [x] Implement placeProtectionSL
- [x] Implement placeProtectionTP
- [x] Get settings from chain (not Ginie mode)

### Task 5: Integration
- [x] Add to UserAutopilotInstance
- [x] Wire in UserAutopilotManager
- [x] Add startup/shutdown ordering
- [ ] Add condition to disable Ginie when chain active (Future - when chain system fully tested)

### Task 6: API
- [x] Add GET /api/futures/position-controller/status
- [x] Add POST /api/futures/position-controller/heal
- [x] Add POST /api/futures/position-controller/start
- [x] Add POST /api/futures/position-controller/stop

### Task 7: Testing
- [x] Unit tests for signal handlers
- [x] Unit tests for SL validation logic
- [x] Unit tests for protection heal
- [ ] Integration test with mock Binance client (Future)
- [ ] End-to-end test with Exit Decision Service (Future)

---

## Acceptance Criteria

### AC10.4.1: Signal Subscription
- [x] Position Controller subscribes to Exit Decision signals
- [x] All signal types are handled (TRAIL_SL, BREAKEVEN, UPDATE_TP, etc.)
- [x] Signals are processed within 100ms of receipt

### AC10.4.2: SL/TP Updates
- [x] SL orders updated on Binance when TRAIL_SL signal received
- [x] TP orders updated on Binance when UPDATE_TP signal received
- [x] Only "improvement" updates are executed (SL trails up for LONG, down for SHORT)
- [x] Old orders cancelled before new orders placed

### AC10.4.3: Protection Heal
- [x] Missing SL orders detected and placed automatically
- [x] Missing TP orders detected and placed automatically
- [x] Heal check runs every 30 seconds (configurable)
- [x] Fallback prices from chain settings, not Ginie mode settings

### AC10.4.4: Chain Settings Usage
- [x] SL% retrieved from order_chains table (via ChainEventWriter)
- [x] TP% retrieved from order_chains table (via ChainEventWriter)
- [x] NO reference to old Ginie mode settings
- [x] Settings match what was used at entry time

### AC10.4.5: Ginie Disablement
- [ ] When entry_decision_system="chain", Ginie position management is disabled (Future - requires full chain system testing)
- [x] Position Controller handles all SL/TP updates when enabled
- [x] Architecture supports running alongside or instead of Ginie

### AC10.4.6: Error Handling
- [x] Retries on Binance API failures (max 3 attempts with exponential backoff)
- [x] Graceful degradation if Exit Decision Service unavailable
- [x] Errors logged with context for debugging

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Signal-to-execution latency | < 500ms |
| SL/TP update success rate | > 99% |
| Protection heal coverage | 100% of chain positions |
| Ginie position management calls | 0 when chain active |

---

## Dependencies

| Dependency | Status | Required For |
|------------|--------|--------------|
| Exit Decision Service (10.1) | Done | Signal source |
| Chain Entry Runner (14.x) | Done | Creates positions with chain settings |
| order_chains table | Done | Settings storage |
| Binance Futures Client | Existing | Order execution |

---

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `internal/autopilot/position_controller.go` | Create | Core controller implementation |
| `internal/autopilot/position_controller_test.go` | Create | Unit tests |
| `internal/autopilot/user_autopilot_manager.go` | Modify | Add Position Controller integration |
| `internal/api/handlers_position_controller.go` | Create | API endpoints |
| `internal/api/server.go` | Modify | Register routes |

---

## Summary

### What This Story Delivers

```
BEFORE (Current):
  Exit Decision Service → [signals stored but not used]
  Ginie → [monitors + updates SL/TP with wrong settings]

AFTER (This Story):
  Exit Decision Service → Position Controller → Binance
  Ginie → [DISABLED for position management]
```

### Key Points

1. **Simple execution layer** - No AI, no learning, just execute signals
2. **Uses chain settings** - Not old Ginie mode settings
3. **Protection heal** - Ensures orders always exist
4. **Enables Ginie disablement** - Chain system becomes complete
5. **Separation of concerns** - Exit Decision decides, Position Controller executes

---

**This story is ready for implementation.**
