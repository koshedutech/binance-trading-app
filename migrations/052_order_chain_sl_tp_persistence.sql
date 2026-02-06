-- Migration 052: Add SL/TP persistence columns to order_chains
-- Purpose: Store SL/TP order details so closed chains can reconstruct
-- full order tree without relying on Binance API (which purges historical data).
--
-- Date: 2026-02-06

-- ============================================================================
-- ADD SL/TP PERSISTENCE COLUMNS
-- ============================================================================

ALTER TABLE order_chains
ADD COLUMN IF NOT EXISTS sl_binance_order_id BIGINT,
ADD COLUMN IF NOT EXISTS sl_limit_price DECIMAL(18,8),
ADD COLUMN IF NOT EXISTS sl_fill_price DECIMAL(18,8),
ADD COLUMN IF NOT EXISTS sl_fill_time TIMESTAMP,
ADD COLUMN IF NOT EXISTS sl_status VARCHAR(20),
ADD COLUMN IF NOT EXISTS sl_quantity DECIMAL(18,8),
ADD COLUMN IF NOT EXISTS tp_binance_order_id BIGINT,
ADD COLUMN IF NOT EXISTS tp_limit_price DECIMAL(18,8),
ADD COLUMN IF NOT EXISTS tp_fill_price DECIMAL(18,8),
ADD COLUMN IF NOT EXISTS tp_fill_time TIMESTAMP,
ADD COLUMN IF NOT EXISTS tp_status VARCHAR(20),
ADD COLUMN IF NOT EXISTS tp_quantity DECIMAL(18,8),
ADD COLUMN IF NOT EXISTS close_price DECIMAL(18,8);

COMMENT ON COLUMN order_chains.sl_binance_order_id IS 'Binance order ID for the SL order';
COMMENT ON COLUMN order_chains.sl_limit_price IS 'SL limit/trigger price when placed';
COMMENT ON COLUMN order_chains.sl_fill_price IS 'Actual fill price when SL was hit';
COMMENT ON COLUMN order_chains.sl_fill_time IS 'Timestamp when SL was filled';
COMMENT ON COLUMN order_chains.sl_status IS 'SL order status: NEW, FILLED, CANCELED';
COMMENT ON COLUMN order_chains.sl_quantity IS 'SL order quantity';
COMMENT ON COLUMN order_chains.tp_binance_order_id IS 'Binance order ID for the TP order';
COMMENT ON COLUMN order_chains.tp_limit_price IS 'TP limit/trigger price when placed';
COMMENT ON COLUMN order_chains.tp_fill_price IS 'Actual fill price when TP was hit';
COMMENT ON COLUMN order_chains.tp_fill_time IS 'Timestamp when TP was filled';
COMMENT ON COLUMN order_chains.tp_status IS 'TP order status: NEW, FILLED, CANCELED';
COMMENT ON COLUMN order_chains.tp_quantity IS 'TP order quantity';
COMMENT ON COLUMN order_chains.close_price IS 'Close price: fill price of the SL or TP that closed the position';

-- ============================================================================
-- ROLLBACK
-- ============================================================================
-- ALTER TABLE order_chains
-- DROP COLUMN IF EXISTS sl_binance_order_id,
-- DROP COLUMN IF EXISTS sl_limit_price,
-- DROP COLUMN IF EXISTS sl_fill_price,
-- DROP COLUMN IF EXISTS sl_fill_time,
-- DROP COLUMN IF EXISTS sl_status,
-- DROP COLUMN IF EXISTS sl_quantity,
-- DROP COLUMN IF EXISTS tp_binance_order_id,
-- DROP COLUMN IF EXISTS tp_limit_price,
-- DROP COLUMN IF EXISTS tp_fill_price,
-- DROP COLUMN IF EXISTS tp_fill_time,
-- DROP COLUMN IF EXISTS tp_status,
-- DROP COLUMN IF EXISTS tp_quantity,
-- DROP COLUMN IF EXISTS close_price;
