# Story 9.6: Shared Redis Infrastructure - Active/Standby Container Control

**Story ID:** INFRA-9.6
**Epic:** Epic 9 - Entry Signal Quality Improvements & Infrastructure
**Priority:** P0 (Critical - Trade Protection)
**Estimated Effort:** 16-24 hours
**Author:** Claude Code Agent
**Status:** Ready for Implementation
**Created:** 2026-01-13
**Reviewed:** 2026-01-13 (Mary, Winston, Murat - All Approved after revisions)
**Depends On:** None (Infrastructure Story)

---

## Team Review Summary

| Reviewer | Role | Verdict | Key Feedback |
|----------|------|---------|--------------|
| Mary | Business Analyst | APPROVED | Added success metrics, user workflow |
| Winston | Architect | APPROVED | Added Redis fallback, race condition handling |
| Murat | Test Architect | APPROVED | Added edge case tests, test file locations |

---

## Problem Statement

### Current Pain Points

1. **Trade Unprotected During Dev Rebuild**: When dev container rebuilds (3-5 minutes), active trades lose protection:
   - Dynamic stop-loss stops working
   - Scalp reentry monitoring stops
   - No TP level management
   - Only Binance-side SL/TP orders remain (basic protection)

2. **Cannot Run Dev and Prod Simultaneously**:
   - If they share PostgreSQL, schema changes in dev break prod
   - If they have separate databases, state is not shared
   - Position state is in `ginie_position_state.json` file (conflict if shared)

3. **No Graceful Handover**:
   - Switching from dev to prod requires manual intervention
   - Risk of duplicate orders if both run simultaneously
   - No coordination mechanism between containers

### Why Not Shared PostgreSQL?

During development, we frequently:
- Add new database columns
- Create new tables
- Modify schemas

If dev and prod share the same PostgreSQL:
- Dev adds new column → Prod code doesn't know about it (may work)
- Dev removes column → Prod breaks immediately
- Dev changes column type → Both break

**Solution: Separate PostgreSQL, Shared Redis Only**

---

## Solution Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     SHARED REDIS (infra)                         │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │ Position State    │ Active Control  │ Pub/Sub Signals       ││
│  │ ginie:pos:BTCUSDT │ ginie:active    │ ginie:control         ││
│  │ ginie:pos:ETHUSDT │ ginie:heartbeat │ (DEACTIVATE, READY)   ││
│  └─────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
              ▲                                    ▲
              │                                    │
┌─────────────┴─────────────┐        ┌─────────────┴─────────────┐
│         DEV (8094)        │        │        PROD (8095)        │
│  ┌─────────────────────┐  │        │  ┌─────────────────────┐  │
│  │ Own PostgreSQL      │  │        │  │ Own PostgreSQL      │  │
│  │ (dev schema)        │  │        │  │ (prod schema)       │  │
│  └─────────────────────┘  │        │  └─────────────────────┘  │
│  Status: STANDBY          │        │  Status: ACTIVE (default) │
└───────────────────────────┘        └───────────────────────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Separate PostgreSQL per env | Schema changes in dev don't break prod |
| Shared Redis only | Schema-less, no migration issues |
| Position state in Redis | Both containers see same TP levels, scalp reentry state |
| Prod active by default | Production has stable, tested code |
| Manual takeover via UI | User controls when to switch |
| Redis Pub/Sub for signals | Instant communication (<1ms) |
| Graceful handover | Active finishes current work before deactivating |

---

## Goals

1. **Zero Downtime Protection**: When dev rebuilds, prod protects trades
2. **Shared Position State**: Both containers see same positions via Redis
3. **Manual Control**: UI toggle to take/release control
4. **Graceful Handover**: Current operations complete before switching
5. **No Duplicate Orders**: Only active container executes trades

---

## Success Metrics

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Duplicate Orders | **Zero** | Check order logs during 10 takeover cycles |
| Takeover Time | **< 5 seconds** | Measure from button click to ACTIVE status |
| Position Sync | **< 100ms** | Redis write/read latency monitoring |
| Graceful Handover | **100%** | Current operations complete before switch |
| Force Takeover | **< 2 seconds** | When other instance is dead |

---

## User Workflow

### Scenario: Dev Rebuild While Trades Active

```
1. Krishna has active BTCUSDT position (managed by Prod)
2. Krishna needs to test new code in Dev
3. Dev container restarts for rebuild (3-5 minutes)

   During rebuild:
   ├── Prod continues managing position ✓
   ├── Dynamic SL still working ✓
   ├── Scalp reentry still monitoring ✓
   └── TP levels still managed ✓

4. Dev rebuild completes
5. Krishna opens Dev Ginie page (8094)
6. Sees: "This Instance: DEV | Status: STANDBY"
7. Clicks "Take Control" → Confirmation dialog
8. Prod receives DEACTIVATE signal
9. Prod finishes current operation (if any)
10. Prod sends READY signal
11. Dev becomes ACTIVE
12. Krishna tests new functionality
13. When done: Clicks "Release Control"
14. Prod resumes as ACTIVE
```

### Scenario: Force Takeover (Prod Crashed)

```
1. Prod crashes unexpectedly
2. Krishna opens Dev Ginie page
3. Sees: "Other instance: NOT RESPONDING"
4. Clicks "Force Take Control"
5. Warning: "Other instance not responding. Force takeover?"
6. Confirms → Dev immediately becomes ACTIVE
7. Positions continue to be protected
```

---

## Implementation Phases

### Phase 1: Infrastructure Setup (LOW RISK)
*Estimated: 2 hours*
*Dependencies: None*

#### Task 1.1: Create docker-compose.infra.yml
Separate infrastructure (Redis only) that never restarts during dev/prod operations.

```yaml
# docker-compose.infra.yml
name: binance-infra

services:
  redis:
    image: redis:7-alpine
    container_name: binance-bot-redis
    restart: always
    command: redis-server --appendonly yes --maxmemory 512mb
    volumes:
      - redis-data:/data
    ports:
      - "6380:6379"
    networks:
      - infra-network

networks:
  infra-network:
    driver: bridge

volumes:
  redis-data:
```

#### Task 1.2: Update docker-compose.yml (dev)
- Keep own PostgreSQL
- Connect to shared Redis via external network
- Add INSTANCE_ID=dev
- Add ACTIVE_BY_DEFAULT=false

#### Task 1.3: Update docker-compose.prod.yml (prod)
- Keep own PostgreSQL
- Connect to shared Redis via external network
- Add INSTANCE_ID=prod
- Add ACTIVE_BY_DEFAULT=true

#### Task 1.4: Update startup scripts
```bash
# scripts/start-infra.sh
docker-compose -f docker-compose.infra.yml up -d

# scripts/docker-dev.sh (update)
# Check infra is running first
docker-compose -f docker-compose.infra.yml ps redis || {
  echo "Starting infrastructure..."
  docker-compose -f docker-compose.infra.yml up -d
}
```

---

### Phase 2: Position State in Redis (MEDIUM RISK)
*Estimated: 6 hours*
*Dependencies: Phase 1*

#### Task 2.1: Define Redis key structure
```
Redis Keys:
├── ginie:position:{userID}:{symbol}     # Position state JSON
│   └── {side, mode, tp_level, scalp_reentry: {...}, saved_at}
├── ginie:positions:{userID}:list        # List of active position symbols
├── ginie:active                          # Currently active instance ID ("prod" or "dev")
├── ginie:heartbeat:{instanceID}          # Heartbeat timestamp
└── ginie:control                         # Pub/Sub channel for control signals
```

#### Task 2.2: Create Redis position repository
File: `internal/database/redis_position_state.go`

```go
type RedisPositionStateRepository struct {
    client *redis.Client
}

// SavePositionState saves position to Redis (replaces JSON file)
func (r *RedisPositionStateRepository) SavePositionState(ctx context.Context, userID, symbol string, state *PersistedPositionState) error

// LoadPositionState loads position from Redis
func (r *RedisPositionStateRepository) LoadPositionState(ctx context.Context, userID, symbol string) (*PersistedPositionState, error)

// LoadAllPositions loads all positions for a user
func (r *RedisPositionStateRepository) LoadAllPositions(ctx context.Context, userID string) (map[string]*PersistedPositionState, error)

// DeletePosition removes position from Redis
func (r *RedisPositionStateRepository) DeletePosition(ctx context.Context, userID, symbol string) error
```

#### Task 2.3: Migrate SavePositionState in ginie_autopilot.go
Replace file writes with Redis writes:

```go
// Before (file-based)
func (ga *GinieAutopilot) SavePositionState() error {
    data, _ := json.MarshalIndent(store, "", "  ")
    os.WriteFile(positionStateFile, data, 0644)
}

// After (Redis-based)
func (ga *GinieAutopilot) SavePositionState() error {
    for symbol, pos := range ga.positions {
        state := PersistedPositionState{...}
        ga.redisRepo.SavePositionState(ctx, ga.userID, symbol, &state)
    }
}
```

#### Task 2.4: Migrate LoadPositionState in ginie_autopilot.go
Replace file reads with Redis reads:

```go
// Before
func LoadPositionState() (map[string]PersistedPositionState, error) {
    data, _ := os.ReadFile(positionStateFile)
    json.Unmarshal(data, &store)
}

// After
func (ga *GinieAutopilot) LoadPositionState() (map[string]PersistedPositionState, error) {
    return ga.redisRepo.LoadAllPositions(ctx, ga.userID)
}
```

#### Task 2.5: Remove ginie_position_state.json dependency
- Remove file mount from docker-compose files
- Remove positionStateFile constant
- Delete JSON file after migration verified

#### Task 2.6: Add Redis Fallback Logic
When Redis is unavailable, gracefully degrade to in-memory cache:

```go
type RedisPositionStateRepository struct {
    client          *redis.Client
    inMemoryCache   map[string]*PersistedPositionState  // Fallback cache
    cacheMu         sync.RWMutex
    redisAvailable  atomic.Bool
}

func (r *RedisPositionStateRepository) LoadPositionState(ctx context.Context, userID, symbol string) (*PersistedPositionState, error) {
    state, err := r.client.Get(ctx, r.key(userID, symbol)).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, nil // Position doesn't exist
        }
        // Redis unavailable - use in-memory fallback
        log.Warn("[REDIS] Unavailable, using in-memory cache", "error", err)
        r.redisAvailable.Store(false)
        return r.getFromCache(userID, symbol), nil
    }
    r.redisAvailable.Store(true)
    // Update in-memory cache
    r.updateCache(userID, symbol, &parsedState)
    return &parsedState, nil
}
```

---

### Phase 3: Active/Standby Control (MEDIUM RISK)
*Estimated: 4 hours*
*Dependencies: Phase 2*

#### Task 3.1: Create instance control manager
File: `internal/autopilot/instance_control.go`

```go
type InstanceControl struct {
    redis        *redis.Client
    instanceID   string              // "dev" or "prod"
    isActive     atomic.Bool         // Current state
    onDeactivate func()              // Callback when deactivated
    onActivate   func()              // Callback when activated
}

// IsActive returns whether this instance should execute trades
func (ic *InstanceControl) IsActive() bool

// TakeControl requests control transfer from other instance
func (ic *InstanceControl) TakeControl(ctx context.Context) error

// ReleaseControl voluntarily releases control
func (ic *InstanceControl) ReleaseControl(ctx context.Context) error

// StartHeartbeat begins heartbeat updates
func (ic *InstanceControl) StartHeartbeat(ctx context.Context)

// SubscribeToControl listens for control signals
func (ic *InstanceControl) SubscribeToControl(ctx context.Context)
```

#### Task 3.2: Implement Redis Pub/Sub signaling
```go
const controlChannel = "ginie:control"

type ControlSignal struct {
    Type       string    `json:"type"`       // "DEACTIVATE", "READY", "HEARTBEAT"
    FromInstance string  `json:"from"`       // Instance sending signal
    ToInstance   string  `json:"to"`         // Target instance (or "*" for all)
    Timestamp    int64   `json:"timestamp"`
}

// Signal flow for takeover:
// 1. Dev sends: {type: "DEACTIVATE", from: "dev", to: "prod"}
// 2. Prod receives, finishes current work
// 3. Prod sends: {type: "READY", from: "prod", to: "dev"}
// 4. Dev activates, sets ginie:active = "dev"
```

#### Task 3.3: Implement graceful deactivation
```go
func (ic *InstanceControl) handleDeactivateSignal(signal ControlSignal) {
    // 1. Stop accepting new trades
    ic.isActive.Store(false)

    // 2. Wait for current operations to complete (max 5 seconds)
    ic.waitForCurrentOperations(5 * time.Second)

    // 3. Save final state to Redis
    ic.onDeactivate()

    // 4. Send READY signal
    ic.publishSignal(ControlSignal{
        Type: "READY",
        FromInstance: ic.instanceID,
        ToInstance: signal.FromInstance,
    })
}
```

#### Task 3.4: Add heartbeat monitoring
```go
func (ic *InstanceControl) StartHeartbeat(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    for {
        select {
        case <-ticker.C:
            ic.redis.Set(ctx, "ginie:heartbeat:"+ic.instanceID, time.Now().Unix(), 30*time.Second)
        case <-ctx.Done():
            return
        }
    }
}

// Check if other instance is alive
func (ic *InstanceControl) isInstanceAlive(instanceID string) bool {
    _, err := ic.redis.Get(ctx, "ginie:heartbeat:"+instanceID).Result()
    return err == nil
}
```

---

### Phase 4: Autopilot Guards (MEDIUM RISK)
*Estimated: 4 hours*
*Dependencies: Phase 3*

#### Task 4.1: Add isActive check to all trade execution points
File: `internal/autopilot/ginie_autopilot.go`

```go
// Before executing ANY order
func (ga *GinieAutopilot) executeOrder(ctx context.Context, order *Order) error {
    // NEW: Check if this instance is active
    if !ga.instanceControl.IsActive() {
        log.Printf("[GINIE] Instance %s is STANDBY - skipping order execution", ga.instanceID)
        return ErrInstanceNotActive
    }

    // Proceed with order execution
    return ga.binanceClient.PlaceOrder(ctx, order)
}
```

#### Task 4.2: Guard all trading functions
Add isActive check to:
- `executeTPOrder()`
- `executeSLOrder()`
- `executeScalpReentry()`
- `executeDynamicSL()`
- `openNewPosition()`
- `closePosition()`

```go
// Helper function
func (ga *GinieAutopilot) requireActive() error {
    if !ga.instanceControl.IsActive() {
        return fmt.Errorf("instance %s is in STANDBY mode", ga.instanceID)
    }
    return nil
}
```

#### Task 4.3: Allow monitoring while standby
Standby instance should still:
- Monitor positions (read from Redis)
- Update UI with position data
- Log position changes
- Be ready to take over

```go
func (ga *GinieAutopilot) monitorPositions() {
    // Always monitor - whether active or standby
    positions := ga.redisRepo.LoadAllPositions(ctx, ga.userID)

    for _, pos := range positions {
        ga.updatePositionUI(pos)

        // Only execute trades if active
        if ga.instanceControl.IsActive() {
            ga.checkTPConditions(pos)
            ga.checkSLConditions(pos)
        }
    }
}
```

---

### Phase 5: API Endpoints (LOW RISK)
*Estimated: 2 hours*
*Dependencies: Phase 4*

#### Task 5.1: Create control status endpoint
File: `internal/api/handlers_instance_control.go`

```go
// GET /api/ginie/instance-status
type InstanceStatusResponse struct {
    InstanceID      string    `json:"instance_id"`      // "dev" or "prod"
    IsActive        bool      `json:"is_active"`        // This instance's status
    ActiveInstance  string    `json:"active_instance"`  // Which instance is active
    OtherAlive      bool      `json:"other_alive"`      // Is other instance running
    LastHeartbeat   time.Time `json:"last_heartbeat"`   // Last heartbeat time
    CanTakeControl  bool      `json:"can_take_control"` // Can this instance take over
}
```

#### Task 5.2: Create take control endpoint
```go
// POST /api/ginie/take-control
type TakeControlRequest struct {
    Force bool `json:"force"` // Force takeover even if other not responding
}

type TakeControlResponse struct {
    Success      bool   `json:"success"`
    Message      string `json:"message"`
    WaitSeconds  int    `json:"wait_seconds"`  // Estimated wait time
}
```

#### Task 5.3: Create release control endpoint
```go
// POST /api/ginie/release-control
type ReleaseControlResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}
```

---

### Phase 6: UI Toggle (LOW RISK)
*Estimated: 4 hours*
*Dependencies: Phase 5*

#### Task 6.1: Add control panel to Ginie page
File: `web/src/pages/Ginie.tsx`

```tsx
// Add to Ginie page header area
<InstanceControlPanel />
```

File: `web/src/components/InstanceControlPanel.tsx`

```tsx
interface InstanceControlPanelProps {}

export function InstanceControlPanel() {
    const { data: status } = useInstanceStatus();
    const takeControl = useTakeControl();
    const releaseControl = useReleaseControl();

    return (
        <Card className="mb-4">
            <CardHeader>
                <CardTitle className="flex items-center gap-2">
                    <Server className="h-5 w-5" />
                    Instance Control
                </CardTitle>
            </CardHeader>
            <CardContent>
                <div className="flex items-center justify-between">
                    <div>
                        <p className="text-sm text-muted-foreground">
                            This Instance: <strong>{status?.instanceId}</strong>
                        </p>
                        <p className="text-sm">
                            Status: {status?.isActive ? (
                                <Badge variant="success">ACTIVE - Trading</Badge>
                            ) : (
                                <Badge variant="secondary">STANDBY - Monitoring</Badge>
                            )}
                        </p>
                        {!status?.isActive && (
                            <p className="text-xs text-muted-foreground mt-1">
                                Active instance: {status?.activeInstance}
                            </p>
                        )}
                    </div>

                    <div>
                        {status?.isActive ? (
                            <Button
                                variant="outline"
                                onClick={() => releaseControl.mutate()}
                            >
                                Release Control
                            </Button>
                        ) : (
                            <Button
                                variant="default"
                                onClick={() => takeControl.mutate({})}
                                disabled={!status?.otherAlive}
                            >
                                Take Control
                            </Button>
                        )}
                    </div>
                </div>

                {/* Takeover progress indicator */}
                {takeControl.isLoading && (
                    <div className="mt-4">
                        <Progress value={50} />
                        <p className="text-sm text-center mt-1">
                            Waiting for other instance to release...
                        </p>
                    </div>
                )}
            </CardContent>
        </Card>
    );
}
```

#### Task 6.2: Add real-time status updates via WebSocket
```tsx
// Subscribe to instance status changes
useEffect(() => {
    const ws = new WebSocket(`ws://${window.location.host}/ws/instance-status`);
    ws.onmessage = (event) => {
        const status = JSON.parse(event.data);
        setInstanceStatus(status);
    };
    return () => ws.close();
}, []);
```

#### Task 6.3: Add confirmation dialog for takeover
```tsx
<AlertDialog>
    <AlertDialogTrigger asChild>
        <Button>Take Control</Button>
    </AlertDialogTrigger>
    <AlertDialogContent>
        <AlertDialogHeader>
            <AlertDialogTitle>Take Control?</AlertDialogTitle>
            <AlertDialogDescription>
                This will transfer trading control from {status?.activeInstance} to this instance.
                The other instance will complete any current operations before releasing.
            </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => takeControl.mutate({})}>
                Take Control
            </AlertDialogAction>
        </AlertDialogFooter>
    </AlertDialogContent>
</AlertDialog>
```

---

## Acceptance Criteria

### AC9.6.1: Infrastructure Separation
- [ ] `docker-compose.infra.yml` created with Redis only
- [ ] Dev has own PostgreSQL (unchanged schema freedom)
- [ ] Prod has own PostgreSQL (stable schema)
- [ ] Both connect to shared Redis
- [ ] Infrastructure survives dev/prod restarts

### AC9.6.2: Position State in Redis
- [ ] Position state saved to Redis instead of JSON file
- [ ] Scalp reentry state preserved in Redis
- [ ] TP levels preserved in Redis
- [ ] Both dev and prod see same position state
- [ ] `ginie_position_state.json` no longer used

### AC9.6.3: Active/Standby Control
- [ ] Only one instance is ACTIVE at a time
- [ ] Prod is ACTIVE by default on startup
- [ ] Dev is STANDBY by default on startup
- [ ] Active instance stored in Redis (`ginie:active`)
- [ ] Heartbeat updated every 5 seconds

### AC9.6.4: Graceful Handover
- [ ] Take control sends DEACTIVATE signal via Pub/Sub
- [ ] Active instance finishes current operation before releasing
- [ ] Active instance sends READY signal when done
- [ ] New active instance confirmed via Redis
- [ ] Max wait time: 10 seconds (then force takeover)

### AC9.6.5: Trade Execution Guards
- [ ] All trade functions check `isActive()` before executing
- [ ] Standby instance logs "skipping - STANDBY mode"
- [ ] No duplicate orders possible
- [ ] Standby instance still monitors positions

### AC9.6.6: API Endpoints
- [ ] `GET /api/ginie/instance-status` returns current state
- [ ] `POST /api/ginie/take-control` initiates takeover
- [ ] `POST /api/ginie/release-control` voluntary release
- [ ] Proper error handling and responses

### AC9.6.7: UI Toggle
- [ ] Instance Control panel visible on Ginie page
- [ ] Shows current instance ID (dev/prod)
- [ ] Shows ACTIVE/STANDBY status
- [ ] Take Control button for standby instance
- [ ] Release Control button for active instance
- [ ] Confirmation dialog before takeover
- [ ] Progress indicator during takeover

### AC9.6.8: Force Takeover
- [ ] If other instance not responding (no heartbeat > 30s)
- [ ] Force takeover available
- [ ] Warning shown before force takeover
- [ ] Force takeover succeeds even if other crashed

---

## Files to Modify/Create

### New Files
| File | Phase | Description |
|------|-------|-------------|
| `docker-compose.infra.yml` | 1 | Shared Redis infrastructure |
| `scripts/start-infra.sh` | 1 | Infrastructure startup script |
| `internal/database/redis_position_state.go` | 2 | Redis position repository |
| `internal/autopilot/instance_control.go` | 3 | Active/standby control logic |
| `internal/api/handlers_instance_control.go` | 5 | API endpoints |
| `web/src/components/InstanceControlPanel.tsx` | 6 | UI component |
| `web/src/hooks/useInstanceControl.ts` | 6 | React hooks |

### Modified Files
| File | Phase | Changes |
|------|-------|---------|
| `docker-compose.yml` | 1 | Remove Redis, add INSTANCE_ID, connect to infra |
| `docker-compose.prod.yml` | 1 | Remove Redis/Postgres, add INSTANCE_ID, connect to infra |
| `scripts/docker-dev.sh` | 1 | Check infra running first |
| `internal/autopilot/ginie_autopilot.go` | 2,4 | Redis position state, isActive guards |
| `internal/api/routes.go` | 5 | Add new endpoints |
| `web/src/pages/Ginie.tsx` | 6 | Add InstanceControlPanel |

---

## Testing Strategy

### Test File Locations

| Test Type | File |
|-----------|------|
| Unit: Redis Position Repo | `internal/database/redis_position_state_test.go` |
| Unit: Instance Control | `internal/autopilot/instance_control_test.go` |
| Integration: Takeover Flow | `internal/autopilot/instance_control_integration_test.go` |
| E2E: UI Toggle | `web/src/components/__tests__/InstanceControlPanel.test.tsx` |

### Phase 1 Tests
- [ ] Infra starts independently
- [ ] Dev connects to shared Redis
- [ ] Prod connects to shared Redis
- [ ] Infra survives dev restart
- [ ] Infra survives prod restart

### Phase 2 Tests
- [ ] Position saved to Redis
- [ ] Position loaded from Redis
- [ ] Scalp reentry state preserved
- [ ] Both instances see same data
- [ ] Redis failure handled gracefully (fallback to in-memory)

### Phase 3 Tests
- [ ] Only one instance active at startup
- [ ] Prod active by default
- [ ] Take control signal sent
- [ ] Handover completes within 10s
- [ ] Force takeover works when other dead

### Phase 4 Tests
- [ ] Standby instance doesn't execute trades
- [ ] Active instance executes trades
- [ ] Switch works mid-operation (waits for completion)
- [ ] No duplicate orders in logs

### Phase 5 Tests
- [ ] API returns correct status
- [ ] Take control API works
- [ ] Release control API works
- [ ] Proper error responses

### Phase 6 Tests
- [ ] UI shows correct status
- [ ] Take control button works
- [ ] Release control button works
- [ ] Real-time updates work

### Edge Case Tests (Critical)

#### EC1: Split-Brain Prevention
```
Scenario: Both instances think they're active
Given: Redis network partition occurs during takeover
When: Both instances receive conflicting state
Then: Only ONE executes trades (verify via order logs)
Test: Use Redis WATCH/MULTI for atomic active flag update
```

#### EC2: Redis Failure During Active Trading
```
Scenario: Redis dies while position is being updated
Given: Active instance has open BTCUSDT position
When: Redis becomes unavailable
Then:
  - Instance continues with in-memory state
  - Warning logged: "[REDIS] Unavailable, using in-memory cache"
  - UI shows "Redis Disconnected" warning
  - No duplicate orders placed
```

#### EC3: Rapid Takeover Cycling
```
Scenario: User clicks Take Control 5 times rapidly
Given: Prod is active
When: User spam-clicks "Take Control" on Dev
Then: Only ONE takeover process runs (debounce/lock)
Test: Add mutex lock in TakeControl(), return "already in progress"
```

#### EC4: Force Takeover During Critical Operation
```
Scenario: Force takeover during TP order execution
Given: Prod is placing a TP order (mid-API call)
When: Dev force-takes control (30s timeout)
Then:
  - Prod's TP order completes (API call already sent)
  - OR order is tracked and verified after takeover
  - No orphaned orders
  - Position state is consistent
```

#### EC5: Startup Race Condition
```
Scenario: Both containers start simultaneously
Given: Redis has no ginie:active key
When: Dev and Prod both start at same time
Then: Only Prod becomes active (ACTIVE_BY_DEFAULT=true wins)
Test: Use Redis SETNX (SET if Not eXists) for atomic claim
```

#### EC6: Handover Timeout (Prod Unresponsive)
```
Scenario: Prod receives DEACTIVATE but doesn't respond
Given: Prod is stuck in infinite loop
When: Dev sends DEACTIVATE, waits 10 seconds
Then: Dev force-takes control after timeout
Test: Configurable timeout (default 10s)
```

---

## Risk Assessment

| Phase | Risk | Impact | Mitigation |
|-------|------|--------|------------|
| 1 | Infra not started before dev/prod | Containers fail to start | Script checks infra first |
| 2 | Redis data loss | Position state lost | Redis persistence enabled |
| 3 | Both instances become active | Duplicate orders | Atomic Redis operations |
| 4 | Guards missed somewhere | Duplicate orders | Code review, grep for order functions |
| 5 | API security | Unauthorized takeover | Auth required on endpoints |
| 6 | UI shows stale status | User confused | WebSocket real-time updates |

---

## Rollback Plan

### Phase 1 Rollback
```bash
# Restore original docker-compose files
git checkout HEAD~1 -- docker-compose.yml docker-compose.prod.yml
# Stop infra
docker-compose -f docker-compose.infra.yml down
```

### Phase 2 Rollback
```bash
# Re-enable JSON file
# Restore ginie_position_state.json mount in docker-compose
git checkout HEAD~1 -- internal/autopilot/ginie_autopilot.go
```

### Full Rollback
```bash
git revert <all-phase-commits>
docker-compose down
docker-compose up -d
```

---

## Startup Sequence After Implementation

```bash
# 1. Start infrastructure (once, always running)
docker-compose -f docker-compose.infra.yml up -d

# 2. Start production (always running, ACTIVE by default)
docker-compose -f docker-compose.prod.yml up -d

# 3. Start development when needed (STANDBY by default)
./scripts/docker-dev.sh

# When dev needs to trade:
# - Open Ginie page on dev (8094)
# - Click "Take Control"
# - Dev becomes ACTIVE, prod becomes STANDBY

# When done testing:
# - Click "Release Control" on dev
# - Or just stop dev container
# - Prod automatically becomes ACTIVE
```

---

## Definition of Done

### Implementation
- [ ] All 6 phases implemented
- [ ] All acceptance criteria met
- [ ] Documentation updated (CLAUDE.md)

### Functional
- [ ] Dev and prod can run simultaneously
- [ ] Manual takeover works from UI
- [ ] Position state shared via Redis
- [ ] No duplicate orders possible
- [ ] Graceful handover within 10 seconds

### Testing
- [ ] Unit test coverage >= 80% for new files
- [ ] All edge case tests (EC1-EC6) pass
- [ ] Integration test for complete takeover cycle
- [ ] Manual test: Verify no duplicate orders during 5 takeover cycles
- [ ] Redis failure graceful degradation verified

### Success Metrics Validated
- [ ] Zero duplicate orders in 10 takeover test cycles
- [ ] Takeover completes in < 5 seconds average
- [ ] Position sync latency < 100ms

---

## Design Decisions (Clarified)

| Question | Decision | Rationale |
|----------|----------|-----------|
| Heartbeat interval | 5 seconds | Sufficient for detecting dead instance |
| Force takeover timeout | 30 seconds | Reasonable wait before assuming crash |
| WebSocket on standby | NO | Not needed - only active trades; standby is for dev/testing |
| Redis persistence | AOF every second | Sufficient for position state |
| Multi-user support | Global (single active) | This is for dev/test workflow; production will be standalone |

**Note:** This feature is primarily for development workflow protection. In production, the system runs standalone without active/standby switching.

---

## Related

- **Problem Origin**: Dev rebuilds leave trades unprotected
- **Architecture Decision**: Shared Redis, separate PostgreSQL
- **User Request**: Manual takeover via UI toggle
- **Future Enhancement**: Automatic failover when instance crashes
