-- Add per-user trading mode (paper/live) to user_trading_configs
-- This allows each user to have their own trading mode independent of other users

-- Add dry_run_mode column with default true (paper trading - safe default)
ALTER TABLE user_trading_configs
ADD COLUMN IF NOT EXISTS dry_run_mode BOOLEAN NOT NULL DEFAULT true;

-- Add comment explaining the column
COMMENT ON COLUMN user_trading_configs.dry_run_mode IS 'Per-user paper/live trading mode. true=paper, false=live';

-- Create index for quick lookups when checking trading mode
CREATE INDEX IF NOT EXISTS idx_user_trading_configs_dry_run ON user_trading_configs(user_id, dry_run_mode);
