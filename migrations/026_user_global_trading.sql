-- Migration: 026_user_global_trading
-- Description: Create per-user global trading configuration table
-- Purpose: Store user-specific global trading settings (risk level, allocation, profit reinvestment)
-- Story: 6.2 - Complete User Settings Cache (Pre-Implementation Task)
-- Date: 2026-01-15

-- ============================================================
-- MIGRATION UP
-- ============================================================

-- ====================================================================================
-- TABLE: user_global_trading
-- ====================================================================================
-- Stores per-user global trading settings from default-settings.json → global_trading
-- Controls overall risk level and capital allocation preferences
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_global_trading (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Risk Level (conservative, moderate, aggressive)
    risk_level VARCHAR(20) NOT NULL DEFAULT 'moderate',

    -- Maximum USD allocation for trading
    max_usd_allocation DECIMAL(20,8) NOT NULL DEFAULT 2500.00000000,

    -- Profit reinvestment settings
    profit_reinvest_percent DECIMAL(5,2) NOT NULL DEFAULT 50.00,
    profit_reinvest_risk_level VARCHAR(20) NOT NULL DEFAULT 'aggressive',

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one global trading config per user
    CONSTRAINT unique_user_global_trading UNIQUE(user_id)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_global_trading_user_id ON user_global_trading(user_id);

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_global_trading_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_global_trading_updated_at ON user_global_trading;

CREATE TRIGGER trigger_user_global_trading_updated_at
    BEFORE UPDATE ON user_global_trading
    FOR EACH ROW
    EXECUTE FUNCTION update_user_global_trading_updated_at();

-- ====================================================================================
-- VALIDATION CONSTRAINTS
-- ====================================================================================

-- Validate risk_level values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_gt_risk_level_valid'
    ) THEN
        ALTER TABLE user_global_trading
        ADD CONSTRAINT check_gt_risk_level_valid
        CHECK (risk_level IN ('conservative', 'moderate', 'aggressive'));
    END IF;
END $$;

-- Validate max_usd_allocation ($10 - $1,000,000)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_gt_max_usd_allocation_range'
    ) THEN
        ALTER TABLE user_global_trading
        ADD CONSTRAINT check_gt_max_usd_allocation_range
        CHECK (max_usd_allocation >= 10.0 AND max_usd_allocation <= 1000000.0);
    END IF;
END $$;

-- Validate profit_reinvest_percent (0-100%)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_gt_profit_reinvest_percent_range'
    ) THEN
        ALTER TABLE user_global_trading
        ADD CONSTRAINT check_gt_profit_reinvest_percent_range
        CHECK (profit_reinvest_percent >= 0.0 AND profit_reinvest_percent <= 100.0);
    END IF;
END $$;

-- Validate profit_reinvest_risk_level values
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'check_gt_profit_reinvest_risk_level_valid'
    ) THEN
        ALTER TABLE user_global_trading
        ADD CONSTRAINT check_gt_profit_reinvest_risk_level_valid
        CHECK (profit_reinvest_risk_level IN ('conservative', 'moderate', 'aggressive'));
    END IF;
END $$;

-- ====================================================================================
-- COMMENTS FOR DOCUMENTATION
-- ====================================================================================

COMMENT ON TABLE user_global_trading IS 'Per-user global trading configuration. Controls overall risk level and capital allocation preferences.';
COMMENT ON COLUMN user_global_trading.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_global_trading.risk_level IS 'Overall trading risk level: conservative, moderate, aggressive (default: moderate)';
COMMENT ON COLUMN user_global_trading.max_usd_allocation IS 'Maximum USD allocation for trading ($10-$1M, default: $2500)';
COMMENT ON COLUMN user_global_trading.profit_reinvest_percent IS 'Percentage of profits to reinvest (0-100%, default: 50%)';
COMMENT ON COLUMN user_global_trading.profit_reinvest_risk_level IS 'Risk level for reinvested profits: conservative, moderate, aggressive (default: aggressive)';

-- ============================================================
-- MIGRATION DOWN (ROLLBACK)
-- ============================================================
-- Uncomment and execute the following to rollback this migration:

-- DROP TRIGGER IF EXISTS trigger_user_global_trading_updated_at ON user_global_trading;
-- DROP FUNCTION IF EXISTS update_user_global_trading_updated_at();
-- DROP INDEX IF EXISTS idx_user_global_trading_user_id;
-- DROP TABLE IF EXISTS user_global_trading CASCADE;
