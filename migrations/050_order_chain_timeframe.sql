-- Migration 050: Add strategy identification columns to order_chains
-- Epic: Trade Lifecycle Tracking
-- Story: Store actual timeframe from strategy when creating a position
--
-- Purpose: Store the timeframe (e.g., "3m", "5m", "15m") from the strategy
-- that detected the pattern, so the UI can display the correct timeframe
-- instead of hardcoded values.
--
-- Also adds mode, strategy_group, and sub_strategy columns for compounding
-- equity tracking (these fields exist in the Go struct but weren't persisted).
--
-- Date: 2026-02-04

-- ============================================================================
-- ADD STRATEGY IDENTIFICATION COLUMNS TO order_chains
-- ============================================================================

-- Add timeframe column (nullable for existing records)
ALTER TABLE order_chains
ADD COLUMN IF NOT EXISTS timeframe VARCHAR(10);

-- Add mode column (full mode name: "scalp", "swing", "position", "ultra_fast")
ALTER TABLE order_chains
ADD COLUMN IF NOT EXISTS mode VARCHAR(20);

-- Add strategy_group column (e.g., "breakout", "trending")
ALTER TABLE order_chains
ADD COLUMN IF NOT EXISTS strategy_group VARCHAR(50);

-- Add sub_strategy column (e.g., "ravindra_volume_imbalance")
ALTER TABLE order_chains
ADD COLUMN IF NOT EXISTS sub_strategy VARCHAR(100);

-- ============================================================================
-- ADD COMMENTS
-- ============================================================================

COMMENT ON COLUMN order_chains.timeframe IS 'Timeframe from strategy pattern detection (e.g., 3m, 5m, 15m). NULL for legacy chains.';
COMMENT ON COLUMN order_chains.mode IS 'Full mode name: scalp, swing, position, ultra_fast. NULL for legacy chains.';
COMMENT ON COLUMN order_chains.strategy_group IS 'Strategy group: breakout, trending, range, volatile. NULL for legacy chains.';
COMMENT ON COLUMN order_chains.sub_strategy IS 'Sub-strategy name: ravindra_volume_imbalance, etc. NULL for legacy chains.';

-- ============================================================================
-- INDEX for strategy-based queries
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_order_chains_strategy ON order_chains(user_id, mode, strategy_group, sub_strategy)
WHERE mode IS NOT NULL;

-- ============================================================================
-- ROLLBACK
-- ============================================================================
-- DROP INDEX IF EXISTS idx_order_chains_strategy;
-- ALTER TABLE order_chains DROP COLUMN IF EXISTS timeframe;
-- ALTER TABLE order_chains DROP COLUMN IF EXISTS mode;
-- ALTER TABLE order_chains DROP COLUMN IF EXISTS strategy_group;
-- ALTER TABLE order_chains DROP COLUMN IF EXISTS sub_strategy;
