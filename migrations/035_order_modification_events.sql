-- Migration 035: Order Modification Events
-- Epic 7: Client Order ID & Trade Lifecycle Tracking
-- Story 7.12: Order Modification Event Log
--
-- Purpose: Capture every modification to SL/TP orders with full audit trail
-- including price changes, dollar impact, and LLM decision reasoning.

-- Order modification event log table
CREATE TABLE IF NOT EXISTS order_modification_events (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) NOT NULL,
    chain_id VARCHAR(30) NOT NULL,              -- "ULT-17JAN-00001"
    order_type VARCHAR(10) NOT NULL,            -- "SL", "TP1", "TP2", etc.
    binance_order_id BIGINT,                    -- Binance order ID (if known)

    -- Event classification
    event_type VARCHAR(20) NOT NULL,            -- "PLACED", "MODIFIED", "CANCELLED", "FILLED"
    modification_source VARCHAR(20),            -- "LLM_AUTO", "USER_MANUAL", "TRAILING_STOP"
    version INTEGER NOT NULL DEFAULT 1,         -- Incrementing version per order

    -- Price tracking
    old_price DECIMAL(18, 8),                   -- NULL for initial placement
    new_price DECIMAL(18, 8) NOT NULL,
    price_delta DECIMAL(18, 8),                 -- new_price - old_price (can be negative)
    price_delta_percent DECIMAL(8, 4),          -- Percentage change

    -- Position context (at time of modification)
    position_quantity DECIMAL(18, 8),           -- Current position size
    position_entry_price DECIMAL(18, 8),        -- Entry price for reference

    -- Dollar impact calculation
    dollar_impact DECIMAL(18, 2),               -- How much this change affects potential P&L
    impact_direction VARCHAR(10),               -- "BETTER", "WORSE", "TIGHTER", "WIDER", "INITIAL"

    -- LLM decision tracking
    modification_reason TEXT,                   -- Human-readable reason
    llm_decision_id VARCHAR(50),                -- Link to decision/event log
    llm_confidence DECIMAL(5, 2),               -- Confidence score (0-100)
    market_context JSONB,                       -- Price, trend, volatility at time of change

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for efficient queries
-- Index for looking up modifications by chain and order type
CREATE INDEX idx_mod_events_chain ON order_modification_events(chain_id, order_type);

-- Index for listing user's recent modifications
CREATE INDEX idx_mod_events_user_time ON order_modification_events(user_id, created_at DESC);

-- Index for filtering by modification source (LLM_AUTO, USER_MANUAL, TRAILING_STOP)
CREATE INDEX idx_mod_events_source ON order_modification_events(modification_source);

-- Index for filtering by event type
CREATE INDEX idx_mod_events_event_type ON order_modification_events(event_type);

-- Index for Binance order ID lookups
CREATE INDEX idx_mod_events_binance_order ON order_modification_events(binance_order_id) WHERE binance_order_id IS NOT NULL;

-- Comments for documentation
COMMENT ON TABLE order_modification_events IS 'Audit trail of all SL/TP order modifications with price changes, dollar impact, and LLM reasoning';
COMMENT ON COLUMN order_modification_events.chain_id IS 'Links to the order chain (e.g., ULT-17JAN-00001)';
COMMENT ON COLUMN order_modification_events.order_type IS 'Type of order: SL, TP1, TP2, TP3, TP4, HSL, HTP';
COMMENT ON COLUMN order_modification_events.event_type IS 'Event type: PLACED (initial), MODIFIED (price changed), CANCELLED, FILLED';
COMMENT ON COLUMN order_modification_events.modification_source IS 'Source: LLM_AUTO (Ginie), USER_MANUAL (user changed), TRAILING_STOP (trailing stop adjustment)';
COMMENT ON COLUMN order_modification_events.version IS 'Version number, increments with each modification';
COMMENT ON COLUMN order_modification_events.dollar_impact IS 'How this change affects potential P&L in USDT';
COMMENT ON COLUMN order_modification_events.impact_direction IS 'BETTER/WORSE for TP, TIGHTER/WIDER for SL, INITIAL for first placement';
COMMENT ON COLUMN order_modification_events.llm_decision_id IS 'Links to LLM decision log for traceability';
COMMENT ON COLUMN order_modification_events.market_context IS 'JSON snapshot of market conditions at time of modification';
