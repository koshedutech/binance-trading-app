-- Epic 7: Client Order ID & Trade Lifecycle Tracking
-- Story 7.11: Position State Tracking
--
-- Purpose: Track the transition from Entry Order to Active Position as an explicit
-- lifecycle stage, ensuring the entry order remains visible in the chain even after it fills.
--
-- Migration: 034_position_states.sql
-- Date: 2026-01-17

-- Position state tracking table
CREATE TABLE IF NOT EXISTS position_states (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) NOT NULL,
    chain_id VARCHAR(30) NOT NULL,              -- "ULT-17JAN-00001"
    symbol VARCHAR(20) NOT NULL,                -- "BTCUSDT"

    -- Entry order reference
    entry_order_id BIGINT NOT NULL,             -- Binance order ID
    entry_client_order_id VARCHAR(40),          -- "ULT-17JAN-00001-E"

    -- Position entry details
    entry_side VARCHAR(10) NOT NULL,            -- "BUY" (LONG) or "SELL" (SHORT)
    entry_price DECIMAL(18, 8) NOT NULL,        -- Avg fill price
    entry_quantity DECIMAL(18, 8) NOT NULL,     -- Total filled quantity
    entry_value DECIMAL(18, 2) NOT NULL,        -- entry_price * entry_quantity
    entry_fees DECIMAL(18, 8) DEFAULT 0,        -- Commission paid
    entry_filled_at TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Current position state
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, PARTIAL, CLOSED
    remaining_quantity DECIMAL(18, 8) NOT NULL,
    realized_pnl DECIMAL(18, 2) DEFAULT 0,         -- P&L from partial closes

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    closed_at TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT unique_chain_position UNIQUE (user_id, chain_id)
);

-- Index for efficient queries
CREATE INDEX IF NOT EXISTS idx_position_states_user_status ON position_states(user_id, status);
CREATE INDEX IF NOT EXISTS idx_position_states_chain ON position_states(chain_id);
CREATE INDEX IF NOT EXISTS idx_position_states_symbol ON position_states(user_id, symbol, status);
CREATE INDEX IF NOT EXISTS idx_position_states_entry_order ON position_states(entry_order_id);

-- Add comment for documentation
COMMENT ON TABLE position_states IS 'Tracks the lifecycle of positions from entry order fill to close, preserving entry order details for Trade Lifecycle display';
COMMENT ON COLUMN position_states.chain_id IS 'Order chain identifier (e.g., ULT-17JAN-00001) linking all related orders';
COMMENT ON COLUMN position_states.status IS 'ACTIVE = full position open, PARTIAL = some TPs hit, CLOSED = position fully closed';
COMMENT ON COLUMN position_states.remaining_quantity IS 'Current position size after partial closes';
COMMENT ON COLUMN position_states.realized_pnl IS 'Accumulated P&L from partial closes (TP hits)';
