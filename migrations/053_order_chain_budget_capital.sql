-- Migration 053: Add budget_capital_used column to order_chains
-- Used for incremental equity position sizing - tracks how much budget capital each chain uses

ALTER TABLE order_chains ADD COLUMN IF NOT EXISTS budget_capital_used DECIMAL(18,8);
