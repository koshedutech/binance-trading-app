# Story 7.17: Chain Event Writer Service

## Story Overview

**Story ID:** 7-17
**Epic:** 7 - Client Order ID & Trade Lifecycle Tracking
**Priority:** P0 (Critical)
**Status:** done
**Created:** 2026-01-18
**Completed:** 2026-01-18
**Complexity:** Large
**Depends On:** Story 7.16 (Order Chain Event Store)

---

## Goal

Create a centralized service that writes events to the `order_chain_events` table and updates the `order_chains` master table. This service is the ONLY writer to these tables, ensuring consistency.

---

## Acceptance Criteria

- [x] AC1: ChainEventWriter service with methods to write each event type
- [x] AC2: Atomic operations - event + master table update in single transaction
- [x] AC3: Event sequence auto-incremented per chain
- [x] AC4: Master table `order_chains` denormalized state updated on each event
- [ ] AC5: Redis cache updated after successful PostgreSQL write (deferred to Story 7.20)
- [x] AC6: Handles all event types (Entry, Position, SL, TP, Hedge, Chain)
- [x] AC7: Binance confirmation required before writing (no speculative events)
- [x] AC8: Error handling with retry for transient failures

---

## Service Interface

```go
// internal/orders/chain_event_writer.go

type ChainEventWriter struct {
    db    *database.Repository
    cache *cache.CacheService
    log   zerolog.Logger
}

// NewChainEventWriter creates a new event writer
func NewChainEventWriter(db *database.Repository, cache *cache.CacheService) *ChainEventWriter

// --- Chain Lifecycle ---

// CreateChain creates a new order chain (called when entry order is placed)
func (w *ChainEventWriter) CreateChain(ctx context.Context, req CreateChainRequest) (*OrderChain, error)

// CloseChain marks chain as closed (called when position fully exits)
func (w *ChainEventWriter) CloseChain(ctx context.Context, chainID string, reason string, totalPnL float64, totalFees float64) error

// --- Entry Events ---

// RecordEntryPlaced records entry order placement (after Binance confirms)
func (w *ChainEventWriter) RecordEntryPlaced(ctx context.Context, chainID string, req EntryPlacedEvent) error

// RecordEntryFilled records entry order fill (position now active)
func (w *ChainEventWriter) RecordEntryFilled(ctx context.Context, chainID string, req EntryFilledEvent) error

// RecordEntryCancelled records entry cancellation
func (w *ChainEventWriter) RecordEntryCancelled(ctx context.Context, chainID string, reason string) error

// --- Stop Loss Events ---

// RecordSLPlaced records initial SL placement
func (w *ChainEventWriter) RecordSLPlaced(ctx context.Context, chainID string, req SLPlacedEvent) error

// RecordSLModified records SL price modification (the key event for tracking)
func (w *ChainEventWriter) RecordSLModified(ctx context.Context, chainID string, req SLModifiedEvent) error

// RecordSLFilled records SL hit (position closed by SL)
func (w *ChainEventWriter) RecordSLFilled(ctx context.Context, chainID string, req SLFilledEvent) error

// --- Take Profit Events (Normal Mode) ---

// RecordTPPlaced records initial TP placement
func (w *ChainEventWriter) RecordTPPlaced(ctx context.Context, chainID string, req TPPlacedEvent) error

// RecordTPModified records TP price modification
func (w *ChainEventWriter) RecordTPModified(ctx context.Context, chainID string, req TPModifiedEvent) error

// RecordTPFilled records TP hit
func (w *ChainEventWriter) RecordTPFilled(ctx context.Context, chainID string, req TPFilledEvent) error

// --- Take Profit Events (Position Optimization) ---

// RecordTPLevelPlaced records TP1/TP2/TP3 placement
func (w *ChainEventWriter) RecordTPLevelPlaced(ctx context.Context, chainID string, level int, req TPLevelPlacedEvent) error

// RecordTPLevelFilled records TP1/TP2/TP3 hit
func (w *ChainEventWriter) RecordTPLevelFilled(ctx context.Context, chainID string, level int, req TPLevelFilledEvent) error

// --- Hedge Events ---

// LinkHedgeChain links a hedge chain to primary chain
func (w *ChainEventWriter) LinkHedgeChain(ctx context.Context, primaryChainID, hedgeChainID string) error

// RecordHedgeClosed records hedge position closure
func (w *ChainEventWriter) RecordHedgeClosed(ctx context.Context, chainID string, pnl float64) error

// --- Query Methods ---

// GetChain retrieves current chain state
func (w *ChainEventWriter) GetChain(ctx context.Context, chainID string) (*OrderChain, error)

// GetChainEvents retrieves all events for a chain
func (w *ChainEventWriter) GetChainEvents(ctx context.Context, chainID string) ([]*ChainEvent, error)

// GetActiveChains retrieves all active chains for a user
func (w *ChainEventWriter) GetActiveChains(ctx context.Context, userID string) ([]*OrderChain, error)
```

---

## Event Request Structures

```go
// internal/orders/chain_event_types.go

type CreateChainRequest struct {
    UserID    string
    ChainID   string
    Symbol    string
    Side      string  // "LONG" or "SHORT"
    ModeCode  string  // "ULT", "SCA", etc.
    IsHedge   bool
    ParentChainID string  // If hedge, link to parent
}

type EntryPlacedEvent struct {
    BinanceOrderID       int64
    BinanceClientOrderID string
    Price                float64
    Quantity             float64
    BinanceTimestamp     int64
}

type EntryFilledEvent struct {
    FilledPrice    float64
    FilledQuantity float64
    Fees           float64
    BinanceTimestamp int64
}

type SLPlacedEvent struct {
    BinanceOrderID       int64
    BinanceClientOrderID string
    Price                float64
    BinanceTimestamp     int64
}

type SLModifiedEvent struct {
    BinanceOrderID       int64
    OldPrice             float64
    NewPrice             float64
    ModificationSource   string  // "LLM_AUTO", "USER_MANUAL", "TRAILING_STOP"
    ModificationReason   string
    MarketPrice          float64
    BinanceTimestamp     int64
}

type SLFilledEvent struct {
    FilledPrice      float64
    PnL              float64
    Fees             float64
    BinanceTimestamp int64
}

type TPPlacedEvent struct {
    BinanceOrderID       int64
    BinanceClientOrderID string
    Price                float64
    BinanceTimestamp     int64
}

type TPModifiedEvent struct {
    BinanceOrderID       int64
    OldPrice             float64
    NewPrice             float64
    ModificationSource   string
    ModificationReason   string
    MarketPrice          float64
    BinanceTimestamp     int64
}

type TPFilledEvent struct {
    FilledPrice      float64
    PnL              float64
    Fees             float64
    BinanceTimestamp int64
}

type TPLevelPlacedEvent struct {
    Level                int     // 1, 2, or 3
    BinanceOrderID       int64
    BinanceClientOrderID string
    Price                float64
    Quantity             float64  // Percentage-based quantity
    BinanceTimestamp     int64
}

type TPLevelFilledEvent struct {
    Level            int
    FilledPrice      float64
    FilledQuantity   float64
    PnL              float64
    Fees             float64
    BinanceTimestamp int64
}
```

---

## Write Pattern (Transaction)

```go
func (w *ChainEventWriter) RecordSLModified(ctx context.Context, chainID string, req SLModifiedEvent) error {
    return w.db.Transaction(ctx, func(tx *sql.Tx) error {
        // 1. Get next sequence number
        seq, err := w.getNextSequence(tx, chainID)
        if err != nil {
            return err
        }

        // 2. Insert event
        event := &ChainEvent{
            ChainID:            chainID,
            EventSequence:      seq,
            EventType:          "SL_MODIFIED",
            OrderType:          "SL",
            BinanceOrderID:     req.BinanceOrderID,
            Price:              req.NewPrice,
            OldPrice:           &req.OldPrice,
            ModificationSource: req.ModificationSource,
            ModificationReason: req.ModificationReason,
            MarketPrice:        req.MarketPrice,
            BinanceTimestamp:   req.BinanceTimestamp,
        }
        if err := w.insertEvent(tx, event); err != nil {
            return err
        }

        // 3. Update master table
        if err := w.updateChainSLPrice(tx, chainID, req.NewPrice); err != nil {
            return err
        }

        // 4. Increment modification count
        if err := w.incrementSLModCount(tx, chainID); err != nil {
            return err
        }

        return nil
    })

    // 5. Update Redis cache (outside transaction, best-effort)
    w.updateRedisCache(ctx, chainID)

    return nil
}
```

---

## Redis Cache Update

After each successful PostgreSQL write, update Redis:

```go
func (w *ChainEventWriter) updateRedisCache(ctx context.Context, chainID string) {
    chain, err := w.GetChain(ctx, chainID)
    if err != nil {
        w.log.Warn().Err(err).Str("chain_id", chainID).Msg("Failed to update Redis cache")
        return
    }

    // Serialize chain to JSON
    data, _ := json.Marshal(chain)

    // Set in Redis (no TTL for active chains)
    key := fmt.Sprintf("order_chain:%s:%s", chain.UserID, chainID)
    w.cache.Set(ctx, key, string(data), 0)
}
```

---

## Files to Create

| File | Description |
|------|-------------|
| `internal/orders/chain_event_writer.go` | Main service |
| `internal/orders/chain_event_types.go` | Request/response types |
| `internal/orders/chain_event_writer_test.go` | Unit tests |

---

## Integration Points

| Component | Integration |
|-----------|-------------|
| Ginie Autopilot | Replace direct DB calls with ChainEventWriter |
| API Handlers | Use ChainEventWriter for queries |
| WebSocket | Push events after successful write |

---

## Test Scenarios

1. **Create chain** - New chain created with PENDING status
2. **Entry flow** - PLACED → FILLED → POSITION_OPENED events
3. **SL modification** - Multiple SL_MODIFIED events, sequence correct
4. **Transaction rollback** - Event insert fails, master not updated
5. **Redis sync** - Cache updated after write
6. **Concurrent writes** - Two modifications same chain, both succeed with unique sequences

---

## Definition of Done

- [ ] ChainEventWriter service implemented
- [ ] All event types supported
- [ ] Transaction safety verified
- [ ] Redis cache updated on writes
- [ ] Unit tests passing
- [ ] Build passes

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story created | System |
