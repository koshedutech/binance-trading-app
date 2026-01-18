-- Migration 036: Order Chain Event Store
-- Epic 7: Client Order ID & Trade Lifecycle Tracking
-- Story 7.16: Order Chain Event Store
--
-- Purpose: Event sourcing pattern - single append-only event log capturing
-- every state change in an order chain's lifecycle. PostgreSQL is source of truth.
--
-- Architecture:
--   order_chains      = Master table with denormalized current state (fast reads)
--   order_chain_events = Append-only event log (complete history)
--
-- Date: 2026-01-18

-- ============================================================================
-- TABLE: order_chains (Master/Current State)
-- ============================================================================
CREATE TABLE IF NOT EXISTS order_chains (
    id SERIAL PRIMARY KEY,
    user_id UUID REFERENCES users(id) NOT NULL,
    chain_id VARCHAR(30) NOT NULL,              -- "ULT-18JAN-00001"
    symbol VARCHAR(20) NOT NULL,                -- "BTCUSDT"
    side VARCHAR(10) NOT NULL,                  -- "LONG" or "SHORT"
    mode_code VARCHAR(5) NOT NULL,              -- "ULT", "SCA", "SWI", "POS"

    -- Current state (denormalized for fast reads)
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    -- PENDING, ENTRY_PLACED, ACTIVE, PARTIAL, CLOSED, CANCELLED

    -- Entry details (populated on fill)
    entry_price DECIMAL(18,8),
    entry_quantity DECIMAL(18,8),
    entry_filled_at TIMESTAMP WITH TIME ZONE,
    entry_binance_order_id BIGINT,

    -- Current SL/TP prices (updated on each modification)
    current_sl_price DECIMAL(18,8),
    current_tp_price DECIMAL(18,8),             -- Single TP (no position opt)

    -- Position optimization TPs (when enabled)
    position_opt_enabled BOOLEAN DEFAULT FALSE,
    current_tp1_price DECIMAL(18,8),
    current_tp2_price DECIMAL(18,8),
    current_tp3_price DECIMAL(18,8),

    -- Remaining quantity tracking
    remaining_quantity DECIMAL(18,8),

    -- Hedging
    hedge_chain_id VARCHAR(30),                 -- Linked hedge chain if any
    is_hedge BOOLEAN DEFAULT FALSE,
    parent_chain_id VARCHAR(30),                -- If hedge, link to parent

    -- Counts for quick display
    sl_modification_count INTEGER DEFAULT 0,
    tp_modification_count INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,
    last_event_seq INTEGER DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,

    -- Close details
    close_reason VARCHAR(30),                   -- "SL_HIT", "TP_HIT", "MANUAL", "CANCELLED"

    -- P&L (updated on close)
    realized_pnl DECIMAL(18,2),
    total_fees DECIMAL(18,8),

    CONSTRAINT unique_user_chain UNIQUE (user_id, chain_id)
);

-- Indexes for order_chains
CREATE INDEX IF NOT EXISTS idx_order_chains_user_status ON order_chains(user_id, status);
CREATE INDEX IF NOT EXISTS idx_order_chains_user_symbol ON order_chains(user_id, symbol);
CREATE INDEX IF NOT EXISTS idx_order_chains_hedge ON order_chains(hedge_chain_id) WHERE hedge_chain_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_chains_parent ON order_chains(parent_chain_id) WHERE parent_chain_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_order_chains_created ON order_chains(user_id, created_at DESC);

-- ============================================================================
-- TABLE: order_chain_events (Event Log - Append Only)
-- ============================================================================
CREATE TABLE IF NOT EXISTS order_chain_events (
    id BIGSERIAL PRIMARY KEY,
    chain_id VARCHAR(30) NOT NULL,
    event_sequence INTEGER NOT NULL,            -- 1, 2, 3... per chain

    -- Event classification
    event_type VARCHAR(40) NOT NULL,            -- See Event Types below
    order_type VARCHAR(10),                     -- E, SL, TP, TP1, TP2, TP3, H, HSL, HTP

    -- Binance reference
    binance_order_id BIGINT,
    binance_client_order_id VARCHAR(40),

    -- Price data
    price DECIMAL(18,8),
    old_price DECIMAL(18,8),                    -- For modifications
    quantity DECIMAL(18,8),
    filled_quantity DECIMAL(18,8),

    -- Order details
    order_status VARCHAR(20),                   -- NEW, FILLED, CANCELLED, etc.
    order_side VARCHAR(10),                     -- BUY, SELL

    -- Modification context
    modification_source VARCHAR(20),            -- LLM_AUTO, USER_MANUAL, TRAILING_STOP, SYSTEM
    modification_reason TEXT,

    -- Market context at event time
    market_price DECIMAL(18,8),                 -- Current price when event occurred

    -- P&L for close events
    pnl DECIMAL(18,2),
    fees DECIMAL(18,8),

    -- Timestamps
    binance_timestamp BIGINT,                   -- Binance event time (ms)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT unique_chain_event_seq UNIQUE (chain_id, event_sequence)
);

-- Note: Foreign key added after backfill to avoid issues with existing data
-- CONSTRAINT fk_chain FOREIGN KEY (chain_id) REFERENCES order_chains(chain_id)

-- Indexes for order_chain_events
CREATE INDEX IF NOT EXISTS idx_chain_events_chain ON order_chain_events(chain_id);
CREATE INDEX IF NOT EXISTS idx_chain_events_type ON order_chain_events(event_type);
CREATE INDEX IF NOT EXISTS idx_chain_events_order_type ON order_chain_events(chain_id, order_type);
CREATE INDEX IF NOT EXISTS idx_chain_events_created ON order_chain_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chain_events_binance ON order_chain_events(binance_order_id) WHERE binance_order_id IS NOT NULL;

-- ============================================================================
-- EVENT TYPES DOCUMENTATION
-- ============================================================================
-- Entry Order Events:
--   ENTRY_PLACED, ENTRY_PARTIALLY_FILLED, ENTRY_FILLED, ENTRY_CANCELLED
--
-- Position Events:
--   POSITION_OPENED
--
-- Stop Loss Events:
--   SL_PLACED, SL_MODIFIED, SL_FILLED, SL_CANCELLED
--
-- Take Profit Events (Normal Mode):
--   TP_PLACED, TP_MODIFIED, TP_FILLED, TP_CANCELLED
--
-- Take Profit Events (Position Optimization):
--   TP1_PLACED, TP1_FILLED
--   TP2_PLACED, TP2_FILLED
--   TP3_PLACED, TP3_FILLED
--
-- Hedge Events:
--   HEDGE_LINKED, HEDGE_ENTRY_PLACED, HEDGE_ENTRY_FILLED
--   HEDGE_SL_PLACED, HEDGE_TP_PLACED, HEDGE_CLOSED
--
-- Chain Events:
--   CHAIN_CLOSED

-- ============================================================================
-- BACKFILL: Migrate existing data from position_states
-- ============================================================================
INSERT INTO order_chains (
    user_id, chain_id, symbol, side, mode_code, status,
    entry_price, entry_quantity, entry_filled_at, entry_binance_order_id,
    remaining_quantity, realized_pnl,
    created_at, updated_at, closed_at,
    close_reason
)
SELECT
    user_id,
    chain_id,
    symbol,
    CASE WHEN entry_side = 'BUY' THEN 'LONG' ELSE 'SHORT' END AS side,
    SUBSTRING(chain_id FROM 1 FOR 3) AS mode_code,
    CASE
        WHEN status = 'CLOSED' THEN 'CLOSED'
        WHEN status = 'PARTIAL' THEN 'PARTIAL'
        ELSE 'ACTIVE'
    END AS status,
    entry_price,
    entry_quantity,
    entry_filled_at,
    entry_order_id,
    remaining_quantity,
    realized_pnl,
    created_at,
    updated_at,
    closed_at,
    CASE WHEN status = 'CLOSED' THEN 'CLOSED' ELSE NULL END
FROM position_states
ON CONFLICT (user_id, chain_id) DO NOTHING;

-- ============================================================================
-- BACKFILL: Create POSITION_OPENED events from position_states
-- ============================================================================
INSERT INTO order_chain_events (
    chain_id, event_sequence, event_type, order_type,
    binance_order_id, price, quantity,
    order_side, created_at
)
SELECT
    chain_id,
    1 AS event_sequence,
    'POSITION_OPENED' AS event_type,
    'E' AS order_type,
    entry_order_id,
    entry_price,
    entry_quantity,
    entry_side,
    entry_filled_at
FROM position_states
ON CONFLICT (chain_id, event_sequence) DO NOTHING;

-- ============================================================================
-- BACKFILL: Migrate events from order_modification_events
-- ============================================================================
INSERT INTO order_chain_events (
    chain_id, event_sequence, event_type, order_type,
    binance_order_id, price, old_price, quantity,
    modification_source, modification_reason,
    market_price, created_at
)
SELECT
    chain_id,
    -- Start from 2 since POSITION_OPENED is 1, increment by version
    1 + version AS event_sequence,
    CASE
        WHEN event_type = 'PLACED' THEN order_type || '_PLACED'
        WHEN event_type = 'MODIFIED' THEN order_type || '_MODIFIED'
        WHEN event_type = 'CANCELLED' THEN order_type || '_CANCELLED'
        WHEN event_type = 'FILLED' THEN order_type || '_FILLED'
        ELSE order_type || '_' || event_type
    END AS event_type,
    order_type,
    binance_order_id,
    new_price,
    old_price,
    position_quantity,
    modification_source,
    modification_reason,
    (market_context->>'price')::DECIMAL(18,8) AS market_price,
    created_at
FROM order_modification_events
ON CONFLICT (chain_id, event_sequence) DO NOTHING;

-- ============================================================================
-- UPDATE: Set event_count and last_event_seq on order_chains
-- ============================================================================
UPDATE order_chains oc
SET
    event_count = (
        SELECT COUNT(*) FROM order_chain_events oce WHERE oce.chain_id = oc.chain_id
    ),
    last_event_seq = (
        SELECT COALESCE(MAX(event_sequence), 0) FROM order_chain_events oce WHERE oce.chain_id = oc.chain_id
    ),
    sl_modification_count = (
        SELECT COUNT(*) FROM order_chain_events oce
        WHERE oce.chain_id = oc.chain_id AND oce.event_type = 'SL_MODIFIED'
    ),
    tp_modification_count = (
        SELECT COUNT(*) FROM order_chain_events oce
        WHERE oce.chain_id = oc.chain_id AND oce.event_type LIKE 'TP%_MODIFIED'
    );

-- ============================================================================
-- ADD FOREIGN KEY (after backfill)
-- ============================================================================
-- Note: Only add FK after data is migrated to avoid constraint violations
-- Uncomment after verifying migration success:
-- ALTER TABLE order_chain_events
--     ADD CONSTRAINT fk_chain_events_chain
--     FOREIGN KEY (chain_id) REFERENCES order_chains(chain_id);

-- ============================================================================
-- COMMENTS
-- ============================================================================
COMMENT ON TABLE order_chains IS 'Master table for order chains with denormalized current state for fast reads';
COMMENT ON TABLE order_chain_events IS 'Append-only event log capturing every state change in order chain lifecycle';

COMMENT ON COLUMN order_chains.chain_id IS 'Order chain identifier (e.g., ULT-18JAN-00001)';
COMMENT ON COLUMN order_chains.status IS 'PENDING=waiting, ENTRY_PLACED=entry sent, ACTIVE=position open, PARTIAL=some TPs hit, CLOSED=done, CANCELLED=aborted';
COMMENT ON COLUMN order_chains.side IS 'LONG or SHORT';
COMMENT ON COLUMN order_chains.mode_code IS 'Trading mode: ULT(Ultra-Fast), SCA(Scalp), SWI(Swing), POS(Position)';
COMMENT ON COLUMN order_chains.position_opt_enabled IS 'TRUE if position optimization is on (TP1/TP2/TP3 split exits)';
COMMENT ON COLUMN order_chains.sl_modification_count IS 'Count of SL price modifications for quick UI display';
COMMENT ON COLUMN order_chains.last_event_seq IS 'Last event sequence number for this chain';

COMMENT ON COLUMN order_chain_events.event_sequence IS 'Monotonically increasing sequence per chain (1, 2, 3...)';
COMMENT ON COLUMN order_chain_events.event_type IS 'Event type: ENTRY_PLACED, SL_MODIFIED, TP_FILLED, etc.';
COMMENT ON COLUMN order_chain_events.order_type IS 'Order type: E(Entry), SL, TP, TP1, TP2, TP3, H(Hedge), HSL, HTP';
COMMENT ON COLUMN order_chain_events.modification_source IS 'Who modified: LLM_AUTO, USER_MANUAL, TRAILING_STOP, SYSTEM';
COMMENT ON COLUMN order_chain_events.binance_timestamp IS 'Binance server timestamp in milliseconds';
