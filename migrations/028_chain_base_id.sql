-- Migration 028: Order Chain Tracking (Epic 7 Story 7.3)
-- Adds chain_base_id column to futures_trades and futures_orders tables
-- The chain_base_id links related orders (Entry, SL, TP, DCA, Hedge) together
-- Format: "SCA-15JAN-00001" (max 36 chars)

-- Add chain_base_id column to futures_trades table (positions/trades)
ALTER TABLE futures_trades ADD COLUMN IF NOT EXISTS chain_base_id VARCHAR(36);

-- Create index for efficient chain queries on futures_trades
CREATE INDEX IF NOT EXISTS idx_futures_trades_chain_base_id ON futures_trades(chain_base_id);

-- Add chain_base_id column to futures_orders table
ALTER TABLE futures_orders ADD COLUMN IF NOT EXISTS chain_base_id VARCHAR(36);

-- Create index for efficient chain queries on futures_orders
CREATE INDEX IF NOT EXISTS idx_futures_orders_chain_base_id ON futures_orders(chain_base_id);

-- Add comment for documentation
COMMENT ON COLUMN futures_trades.chain_base_id IS 'Links related positions in a chain (Entry, DCA, Hedge). Format: SCA-15JAN-00001';
COMMENT ON COLUMN futures_orders.chain_base_id IS 'Links related orders in a chain (Entry, SL, TP, DCA, Hedge). Format: SCA-15JAN-00001';
