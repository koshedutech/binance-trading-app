-- Migration: 042_user_blocked_symbols
-- Description: Create table for per-user symbol blocking with expiration support
-- Story: 9.11 - Migrate Symbol Blocking to Database

-- Up Migration
CREATE TABLE IF NOT EXISTS user_blocked_symbols (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(36) NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    reason VARCHAR(255),
    blocked_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,  -- NULL = permanent block
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, symbol)
);

-- Index for fast user lookups
CREATE INDEX IF NOT EXISTS idx_blocked_symbols_user_id ON user_blocked_symbols(user_id);

-- Index for cleanup of expired blocks
CREATE INDEX IF NOT EXISTS idx_blocked_symbols_expires_at ON user_blocked_symbols(expires_at)
    WHERE expires_at IS NOT NULL;

-- Index for checking if a specific symbol is blocked for a user
CREATE INDEX IF NOT EXISTS idx_blocked_symbols_user_symbol ON user_blocked_symbols(user_id, symbol);

COMMENT ON TABLE user_blocked_symbols IS 'Per-user symbol blocking with optional expiration. Replaces file-based blocking in autopilot_settings.json';
COMMENT ON COLUMN user_blocked_symbols.expires_at IS 'When the block expires. NULL means permanent until manually unblocked';
COMMENT ON COLUMN user_blocked_symbols.reason IS 'Why the symbol was blocked (e.g., "poor performance", "manual block", "auto-blocked")';
