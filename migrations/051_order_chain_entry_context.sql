-- Migration 051: Add entry_context JSONB column to order_chains
-- Purpose: Store entry decision context (reference candle, volume multiplier,
-- consolidation duration, breakout time) so the UI can display how the
-- entry was made even after pattern state is cleared.
--
-- Date: 2026-02-05

-- ============================================================================
-- ADD ENTRY CONTEXT COLUMN
-- ============================================================================

ALTER TABLE order_chains
ADD COLUMN IF NOT EXISTS entry_context JSONB;

COMMENT ON COLUMN order_chains.entry_context IS 'JSON entry decision context: reference candle data, volume multiplier, consolidation duration, breakout time, direction';

-- ============================================================================
-- ROLLBACK
-- ============================================================================
-- ALTER TABLE order_chains DROP COLUMN IF EXISTS entry_context;
