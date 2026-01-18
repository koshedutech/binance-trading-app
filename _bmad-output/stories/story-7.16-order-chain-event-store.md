# Story 7.16: Order Chain Event Store

## Story Overview

**Story ID:** 7-16
**Epic:** 7 - Client Order ID & Trade Lifecycle Tracking
**Priority:** P0 (Critical)
**Status:** done
**Created:** 2026-01-18
**Completed:** 2026-01-18
**Complexity:** Large

---

## Problem Statement

Current architecture has fragmented data storage:
- Entry orders fetched from Binance API (disappear after fill)
- Position states in `position_states` table
- Modifications in `order_modification_events` table
- No unified event log for the complete order lifecycle

**Issues:**
1. Entry order data lost when order fills (Binance removes from open orders)
2. Multiple tables to query for complete chain history
3. No crash recovery - if system crashes mid-trade, state is unclear
4. UI cannot display complete tree without fetching from multiple sources

---

## Solution: Event Sourcing Pattern

Create a single **append-only event log** that captures every state change in an order chain's lifecycle. PostgreSQL is the source of truth, Redis caches active chains.

---

## Acceptance Criteria

- [x] AC1: Create `order_chain_events` table with append-only event log
- [x] AC2: Create `order_chains` master table with denormalized current state
- [x] AC3: Each event type captures required data (see Event Types below)
- [x] AC4: Events are never updated or deleted (append-only)
- [x] AC5: Event sequence is monotonically increasing per chain
- [x] AC6: Timestamps stored in UTC, converted to user timezone on display
- [x] AC7: Indexes support efficient queries by chain_id, user_id, event_type
- [x] AC8: Migration script to backfill from existing tables

---

## Database Schema

### Table: `order_chains` (Master/Current State)

```sql
CREATE TABLE order_chains (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,           -- UUID
    chain_id VARCHAR(30) NOT NULL,          -- "ULT-18JAN-00001"
    symbol VARCHAR(20) NOT NULL,            -- "BTCUSDT"
    side VARCHAR(10) NOT NULL,              -- "LONG" or "SHORT"
    mode_code VARCHAR(5) NOT NULL,          -- "ULT", "SCA", "SWI", "POS"

    -- Current state (denormalized for fast reads)
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    -- PENDING, ENTRY_PLACED, ACTIVE, PARTIAL, CLOSED, CANCELLED

    entry_price DECIMAL(18,8),
    entry_quantity DECIMAL(18,8),
    entry_filled_at TIMESTAMP WITH TIME ZONE,

    current_sl_price DECIMAL(18,8),
    current_tp_price DECIMAL(18,8),         -- Single TP (no position opt)

    -- Position optimization TPs (when enabled)
    position_opt_enabled BOOLEAN DEFAULT FALSE,
    current_tp1_price DECIMAL(18,8),
    current_tp2_price DECIMAL(18,8),
    current_tp3_price DECIMAL(18,8),

    -- Hedging
    hedge_chain_id VARCHAR(30),             -- Linked hedge chain if any
    is_hedge BOOLEAN DEFAULT FALSE,

    -- Counts for quick display
    sl_modification_count INTEGER DEFAULT 0,
    tp_modification_count INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,

    -- P&L (updated on close)
    realized_pnl DECIMAL(18,2),
    total_fees DECIMAL(18,8),

    CONSTRAINT unique_user_chain UNIQUE (user_id, chain_id)
);

CREATE INDEX idx_order_chains_user_status ON order_chains(user_id, status);
CREATE INDEX idx_order_chains_symbol ON order_chains(user_id, symbol);
CREATE INDEX idx_order_chains_hedge ON order_chains(hedge_chain_id);
```

### Table: `order_chain_events` (Event Log - Append Only)

```sql
CREATE TABLE order_chain_events (
    id BIGSERIAL PRIMARY KEY,
    chain_id VARCHAR(30) NOT NULL,
    event_sequence INTEGER NOT NULL,        -- 1, 2, 3... per chain

    -- Event classification
    event_type VARCHAR(40) NOT NULL,        -- See Event Types below
    order_type VARCHAR(10),                 -- E, SL, TP, TP1, TP2, TP3, H, HSL, HTP

    -- Binance reference
    binance_order_id BIGINT,
    binance_client_order_id VARCHAR(40),

    -- Price data
    price DECIMAL(18,8),
    old_price DECIMAL(18,8),                -- For modifications
    quantity DECIMAL(18,8),
    filled_quantity DECIMAL(18,8),

    -- Order details
    order_status VARCHAR(20),               -- NEW, FILLED, CANCELLED, etc.
    order_side VARCHAR(10),                 -- BUY, SELL

    -- Modification context
    modification_source VARCHAR(20),        -- LLM_AUTO, USER_MANUAL, TRAILING_STOP, SYSTEM
    modification_reason TEXT,

    -- Market context at event time
    market_price DECIMAL(18,8),             -- Current price when event occurred

    -- P&L for close events
    pnl DECIMAL(18,2),
    fees DECIMAL(18,8),

    -- Timestamps
    binance_timestamp BIGINT,               -- Binance event time (ms)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_chain_event_seq UNIQUE (chain_id, event_sequence),
    CONSTRAINT fk_chain FOREIGN KEY (chain_id) REFERENCES order_chains(chain_id)
);

CREATE INDEX idx_events_chain ON order_chain_events(chain_id);
CREATE INDEX idx_events_type ON order_chain_events(event_type);
CREATE INDEX idx_events_created ON order_chain_events(created_at DESC);
```

---

## Event Types

### Entry Order Events
| Event Type | When | Key Data |
|------------|------|----------|
| `ENTRY_PLACED` | Entry order sent to Binance | price, quantity, binance_order_id |
| `ENTRY_PARTIALLY_FILLED` | Entry partially filled | filled_quantity, market_price |
| `ENTRY_FILLED` | Entry fully filled | filled_quantity, fees |
| `ENTRY_CANCELLED` | Entry cancelled | reason |

### Position Events
| Event Type | When | Key Data |
|------------|------|----------|
| `POSITION_OPENED` | Entry filled, position active | entry_price, quantity |

### Stop Loss Events
| Event Type | When | Key Data |
|------------|------|----------|
| `SL_PLACED` | Initial SL order | price, binance_order_id |
| `SL_MODIFIED` | SL price changed | old_price, price, modification_source, reason |
| `SL_FILLED` | SL hit, position closed | price, pnl, fees |
| `SL_CANCELLED` | SL cancelled (e.g., TP hit first) | reason |

### Take Profit Events (Normal Mode)
| Event Type | When | Key Data |
|------------|------|----------|
| `TP_PLACED` | Initial TP order | price, binance_order_id |
| `TP_MODIFIED` | TP price changed | old_price, price, modification_source, reason |
| `TP_FILLED` | TP hit, position closed | price, pnl, fees |
| `TP_CANCELLED` | TP cancelled | reason |

### Take Profit Events (Position Optimization)
| Event Type | When | Key Data |
|------------|------|----------|
| `TP1_PLACED` | TP1 order placed | price, quantity (25%) |
| `TP1_FILLED` | TP1 hit | price, pnl, fees |
| `TP2_PLACED` | TP2 order placed after TP1 | price, quantity (25%) |
| `TP2_FILLED` | TP2 hit | price, pnl, fees |
| `TP3_PLACED` | TP3 order placed after TP2 | price, quantity (50%) |
| `TP3_FILLED` | TP3 hit | price, pnl, fees |

### Hedge Events
| Event Type | When | Key Data |
|------------|------|----------|
| `HEDGE_LINKED` | Hedge chain created | hedge_chain_id |
| `HEDGE_ENTRY_PLACED` | Hedge entry sent | price, side (opposite) |
| `HEDGE_ENTRY_FILLED` | Hedge entry filled | price, quantity |
| `HEDGE_SL_PLACED` | Hedge SL placed | price |
| `HEDGE_TP_PLACED` | Hedge TP placed | price |
| `HEDGE_CLOSED` | Hedge position closed | pnl |

### Chain Events
| Event Type | When | Key Data |
|------------|------|----------|
| `CHAIN_CLOSED` | All positions closed | total_pnl, total_fees, close_reason |

---

## Migration Script

```sql
-- Migration: 036_order_chain_events.sql

-- 1. Create order_chains table
CREATE TABLE order_chains (...);

-- 2. Create order_chain_events table
CREATE TABLE order_chain_events (...);

-- 3. Backfill from position_states
INSERT INTO order_chains (user_id, chain_id, symbol, side, mode_code, status, ...)
SELECT
    user_id::varchar,
    chain_id,
    symbol,
    CASE WHEN entry_side = 'BUY' THEN 'LONG' ELSE 'SHORT' END,
    SUBSTRING(chain_id FROM 1 FOR 3),  -- Extract mode from chain_id
    CASE
        WHEN status = 'CLOSED' THEN 'CLOSED'
        WHEN status = 'ACTIVE' THEN 'ACTIVE'
        ELSE 'ACTIVE'
    END,
    entry_price,
    entry_quantity,
    entry_filled_at,
    -- ... etc
FROM position_states;

-- 4. Backfill events from order_modification_events
INSERT INTO order_chain_events (chain_id, event_sequence, event_type, order_type, ...)
SELECT
    chain_id,
    version,
    CASE
        WHEN version = 1 THEN order_type || '_PLACED'
        ELSE order_type || '_MODIFIED'
    END,
    order_type,
    new_price,
    old_price,
    modification_source,
    modification_reason,
    created_at
FROM order_modification_events
ORDER BY chain_id, version;

-- 5. Create POSITION_OPENED events from position_states
INSERT INTO order_chain_events (chain_id, event_sequence, event_type, price, quantity, created_at)
SELECT
    chain_id,
    0,  -- Sequence 0 for position opened (before any SL/TP)
    'POSITION_OPENED',
    entry_price,
    entry_quantity,
    entry_filled_at
FROM position_states;
```

---

## Files to Create

| File | Description |
|------|-------------|
| `migrations/036_order_chain_events.sql` | Schema + migration |
| `internal/database/repository_order_chains.go` | CRUD for order_chains |
| `internal/database/repository_chain_events.go` | Append events, query by chain |

---

## Test Scenarios

1. **Create chain** - Insert order_chains row, verify
2. **Append event** - Add event, sequence increments
3. **Query events** - Get all events for chain in order
4. **Migration** - Existing position_states and modification_events migrated correctly
5. **Concurrent events** - Two events same chain, sequences unique

---

## Definition of Done

- [ ] Migration creates tables with correct schema
- [ ] Migration backfills from existing tables
- [ ] Repository methods for CRUD operations
- [ ] Unit tests passing
- [ ] Build passes

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2026-01-18 | Story created for event sourcing architecture | System |
