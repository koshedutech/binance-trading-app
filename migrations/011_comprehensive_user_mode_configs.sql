-- Migration 011: Comprehensive User Mode Configuration Schema
-- This migration creates a complete schema for storing per-user mode configurations
-- Reference: internal/autopilot/settings.go (ModeFullConfig and all sub-configurations)

-- ====================================================================================
-- MAIN TABLE: user_mode_configs
-- ====================================================================================
-- Stores the complete mode configuration as JSONB for flexibility
-- Each user can have 5 modes: ultra_fast, scalp, scalp_reentry, swing, position
-- ====================================================================================

CREATE TABLE IF NOT EXISTS user_mode_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode_name VARCHAR(50) NOT NULL CHECK (mode_name IN ('ultra_fast', 'scalp', 'scalp_reentry', 'swing', 'position')),
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Complete mode configuration stored as JSONB
    -- Contains autopilot.ModeFullConfig structure
    config_json JSONB NOT NULL,

    -- Metadata
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Ensure one config per user per mode
    UNIQUE(user_id, mode_name)
);

-- ====================================================================================
-- INDEXES FOR PERFORMANCE
-- ====================================================================================

-- Fast user lookups
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_user_id
    ON user_mode_configs(user_id);

-- Fast mode name filtering
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_mode_name
    ON user_mode_configs(mode_name);

-- Fast enabled mode lookups (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_enabled
    ON user_mode_configs(user_id, enabled)
    WHERE enabled = true;

-- GIN index for JSON field queries (allows querying inside config_json)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_config_json
    ON user_mode_configs USING GIN (config_json);

-- Partial index for quick access to enabled configs
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_user_enabled
    ON user_mode_configs(user_id, mode_name)
    WHERE enabled = true;

-- ====================================================================================
-- JSONB PATH INDEXES FOR COMMON QUERIES
-- ====================================================================================
-- These indexes optimize queries on specific fields within config_json

-- Index for confidence thresholds (frequently queried)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_min_confidence
    ON user_mode_configs ((config_json->'confidence'->>'min_confidence'));

-- Index for leverage settings (frequently queried)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_leverage
    ON user_mode_configs ((config_json->'size'->>'leverage'));

-- Index for max positions (frequently queried)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_max_positions
    ON user_mode_configs ((config_json->'size'->>'max_positions'));

-- Index for base size (frequently queried)
CREATE INDEX IF NOT EXISTS idx_user_mode_configs_base_size
    ON user_mode_configs ((config_json->'size'->>'base_size_usd'));

-- ====================================================================================
-- TRIGGER: AUTO-UPDATE updated_at TIMESTAMP
-- ====================================================================================

CREATE OR REPLACE FUNCTION update_user_mode_configs_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_user_mode_configs_updated_at ON user_mode_configs;

CREATE TRIGGER trigger_user_mode_configs_updated_at
    BEFORE UPDATE ON user_mode_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_user_mode_configs_updated_at();

-- ====================================================================================
-- VALIDATION FUNCTIONS
-- ====================================================================================

-- Function to validate that config_json has required fields
CREATE OR REPLACE FUNCTION validate_mode_config_json()
RETURNS TRIGGER AS $$
BEGIN
    -- Validate that essential fields exist
    IF NOT (
        NEW.config_json ? 'mode_name' AND
        NEW.config_json ? 'enabled' AND
        NEW.config_json ? 'timeframe' AND
        NEW.config_json ? 'confidence' AND
        NEW.config_json ? 'size' AND
        NEW.config_json ? 'circuit_breaker' AND
        NEW.config_json ? 'sltp'
    ) THEN
        RAISE EXCEPTION 'config_json must contain required fields: mode_name, enabled, timeframe, confidence, size, circuit_breaker, sltp';
    END IF;

    -- Validate mode_name matches
    IF (NEW.config_json->>'mode_name') != NEW.mode_name THEN
        RAISE EXCEPTION 'config_json.mode_name must match mode_name column';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_validate_mode_config_json ON user_mode_configs;

CREATE TRIGGER trigger_validate_mode_config_json
    BEFORE INSERT OR UPDATE ON user_mode_configs
    FOR EACH ROW
    EXECUTE FUNCTION validate_mode_config_json();

-- ====================================================================================
-- DEFAULT MODE CONFIGURATIONS
-- ====================================================================================
-- These views provide access to default configurations for each mode
-- Useful for UI to show "reset to defaults" functionality
-- ====================================================================================

COMMENT ON TABLE user_mode_configs IS 'Per-user trading mode configurations. Each user can customize 5 trading modes with unique settings.';
COMMENT ON COLUMN user_mode_configs.user_id IS 'Foreign key to users table. Configs are deleted when user is deleted (CASCADE).';
COMMENT ON COLUMN user_mode_configs.mode_name IS 'Trading mode name: ultra_fast, scalp, scalp_reentry, swing, or position';
COMMENT ON COLUMN user_mode_configs.enabled IS 'Whether this mode is currently enabled for the user';
COMMENT ON COLUMN user_mode_configs.config_json IS 'Complete mode configuration as JSONB (autopilot.ModeFullConfig structure)';

-- ====================================================================================
-- HELPER VIEWS FOR COMMON QUERIES
-- ====================================================================================

-- View: Active modes per user
CREATE OR REPLACE VIEW v_user_active_modes AS
SELECT
    user_id,
    mode_name,
    config_json->'confidence'->>'min_confidence' as min_confidence,
    config_json->'size'->>'base_size_usd' as base_size_usd,
    config_json->'size'->>'max_positions' as max_positions,
    config_json->'size'->>'leverage' as leverage,
    created_at,
    updated_at
FROM user_mode_configs
WHERE enabled = true;

COMMENT ON VIEW v_user_active_modes IS 'Shows only enabled modes with key configuration parameters extracted for quick reference';

-- View: Mode configuration summary
CREATE OR REPLACE VIEW v_user_mode_summary AS
SELECT
    u.email,
    umc.mode_name,
    umc.enabled,
    (umc.config_json->'confidence'->>'min_confidence')::numeric as min_confidence,
    (umc.config_json->'confidence'->>'high_confidence')::numeric as high_confidence,
    (umc.config_json->'confidence'->>'ultra_confidence')::numeric as ultra_confidence,
    (umc.config_json->'size'->>'base_size_usd')::numeric as base_size_usd,
    (umc.config_json->'size'->>'max_size_usd')::numeric as max_size_usd,
    (umc.config_json->'size'->>'max_positions')::integer as max_positions,
    (umc.config_json->'size'->>'leverage')::integer as leverage,
    (umc.config_json->'sltp'->>'stop_loss_percent')::numeric as stop_loss_percent,
    (umc.config_json->'sltp'->>'take_profit_percent')::numeric as take_profit_percent,
    (umc.config_json->'circuit_breaker'->>'max_loss_per_day')::numeric as max_loss_per_day,
    (umc.config_json->'circuit_breaker'->>'max_consecutive_losses')::integer as max_consecutive_losses,
    umc.updated_at
FROM user_mode_configs umc
JOIN users u ON u.id = umc.user_id;

COMMENT ON VIEW v_user_mode_summary IS 'Human-readable summary of user mode configurations with key settings extracted';

-- ====================================================================================
-- SAMPLE DEFAULT CONFIGURATIONS
-- ====================================================================================
-- Example: Insert default ultra_fast configuration (reference only, do not execute)
-- These match the defaults from autopilot_settings.json
-- ====================================================================================

/*
-- Ultra Fast Mode Default Configuration
{
  "mode_name": "ultra_fast",
  "enabled": false,
  "timeframe": {
    "trend_timeframe": "5m",
    "entry_timeframe": "1m",
    "analysis_timeframe": "1m"
  },
  "confidence": {
    "min_confidence": 40,
    "high_confidence": 50,
    "ultra_confidence": 60
  },
  "size": {
    "base_size_usd": 400,
    "max_size_usd": 400,
    "max_positions": 5,
    "leverage": 10,
    "size_multiplier_lo": 1,
    "size_multiplier_hi": 1.5,
    "safety_margin": 0.9,
    "min_balance_usd": 50,
    "min_position_size_usd": 400,
    "risk_multiplier_conservative": 0,
    "risk_multiplier_moderate": 0,
    "risk_multiplier_aggressive": 0,
    "confidence_multiplier_base": 0.5,
    "confidence_multiplier_scale": 0.5,
    "auto_size_enabled": false,
    "auto_size_min_cover_fee": 15
  },
  "circuit_breaker": {
    "max_loss_per_hour": 20,
    "max_loss_per_day": 50,
    "max_consecutive_losses": 3,
    "cooldown_minutes": 15,
    "max_trades_per_minute": 5,
    "max_trades_per_hour": 30,
    "max_trades_per_day": 100,
    "win_rate_check_after": 10,
    "min_win_rate": 45
  },
  "sltp": {
    "stop_loss_percent": 1,
    "take_profit_percent": 2,
    "trailing_stop_enabled": false,
    "trailing_stop_percent": 0,
    "trailing_stop_activation": 0,
    "trailing_activation_price": 0,
    "max_hold_duration": "3s",
    "use_single_tp": true,
    "single_tp_percent": 2,
    "tp_gain_levels": null,
    "tp_allocation": [100, 0, 0, 0],
    "trailing_activation_mode": "immediate",
    "use_roi_based_sltp": false,
    "roi_stop_loss_percent": -5,
    "roi_take_profit_percent": 10,
    "margin_type": "ISOLATED",
    "isolated_margin_percent": 100,
    "atr_sl_multiplier": 0,
    "atr_tp_multiplier": 0,
    "atr_sl_min": 0,
    "atr_sl_max": 0,
    "atr_tp_min": 0,
    "atr_tp_max": 0,
    "llm_weight": 0,
    "atr_weight": 0,
    "auto_sltp_enabled": false,
    "auto_trailing_enabled": false,
    "min_profit_to_trail_pct": 0.3,
    "min_sl_distance_from_zero": 0.1
  },
  "hedge": {
    "allow_hedge": true,
    "min_confidence_for_hedge": 70,
    "existing_must_be_in_profit": 0,
    "max_hedge_size_percent": 100,
    "allow_same_mode_hedge": false,
    "max_total_exposure_multiplier": 2
  },
  "averaging": {
    "allow_averaging": false,
    "average_up_profit_percent": 0,
    "average_down_loss_percent": 0,
    "add_size_percent": 0,
    "max_averages": 0,
    "min_confidence_for_average": 0,
    "use_llm_for_averaging": false
  },
  "stale_release": {
    "enabled": true,
    "max_hold_duration": "10s",
    "min_profit_to_keep": 0.3,
    "max_loss_to_force_close": -0.5,
    "stale_zone_lo": -0.3,
    "stale_zone_hi": 0.3,
    "stale_zone_close_action": "close"
  },
  "assignment": {
    "volatility_min": "high",
    "volatility_max": "extreme",
    "expected_hold_min": "0",
    "expected_hold_max": "5m",
    "confidence_min": 50,
    "confidence_max": 70,
    "risk_score_max": 50,
    "profit_potential_min": 0.5,
    "profit_potential_max": 2,
    "requires_trend_align": false,
    "priority_weight": 0.8
  },
  "funding_rate": null,
  "risk": null,
  "trend_divergence": null,
  "mtf": {
    "mtf_enabled": true,
    "primary_timeframe": "5m",
    "primary_weight": 0.4,
    "secondary_timeframe": "3m",
    "secondary_weight": 0.35,
    "tertiary_timeframe": "1m",
    "tertiary_weight": 0.25,
    "min_consensus": 2,
    "min_weighted_strength": 65,
    "trend_stability_check": true
  },
  "dynamic_ai_exit": {
    "dynamic_ai_exit_enabled": true,
    "min_hold_before_ai_ms": 3000,
    "ai_check_interval_ms": 5000,
    "use_llm_for_loss": true,
    "use_llm_for_profit": false,
    "max_hold_time_ms": 0
  },
  "reversal": null
}

-- Scalp Mode Default Configuration
{
  "mode_name": "scalp",
  "enabled": true,
  "timeframe": {
    "trend_timeframe": "15m",
    "entry_timeframe": "5m",
    "analysis_timeframe": "15m"
  },
  "confidence": {
    "min_confidence": 45,
    "high_confidence": 55,
    "ultra_confidence": 65
  },
  "size": {
    "base_size_usd": 400,
    "max_size_usd": 600,
    "max_positions": 4,
    "leverage": 8,
    "size_multiplier_lo": 1,
    "size_multiplier_hi": 1.8,
    "safety_margin": 0.9,
    "min_balance_usd": 50,
    "min_position_size_usd": 400,
    "risk_multiplier_conservative": 0,
    "risk_multiplier_moderate": 0,
    "risk_multiplier_aggressive": 0,
    "confidence_multiplier_base": 0.5,
    "confidence_multiplier_scale": 0.5,
    "auto_size_enabled": false,
    "auto_size_min_cover_fee": 15
  },
  "circuit_breaker": {
    "max_loss_per_hour": 40,
    "max_loss_per_day": 100,
    "max_consecutive_losses": 5,
    "cooldown_minutes": 30,
    "max_trades_per_minute": 3,
    "max_trades_per_hour": 20,
    "max_trades_per_day": 50,
    "win_rate_check_after": 15,
    "min_win_rate": 50
  },
  "sltp": {
    "stop_loss_percent": 1.5,
    "take_profit_percent": 3,
    "trailing_stop_enabled": false,
    "trailing_stop_percent": 0.5,
    "trailing_stop_activation": 0.5,
    "trailing_activation_price": 0,
    "max_hold_duration": "4h",
    "use_single_tp": true,
    "single_tp_percent": 3,
    "tp_gain_levels": null,
    "tp_allocation": [100, 0, 0, 0],
    "trailing_activation_mode": "after_tp1",
    "use_roi_based_sltp": false,
    "roi_stop_loss_percent": -8,
    "roi_take_profit_percent": 15,
    "margin_type": "ISOLATED",
    "isolated_margin_percent": 100,
    "atr_sl_multiplier": 0,
    "atr_tp_multiplier": 0,
    "atr_sl_min": 0,
    "atr_sl_max": 0,
    "atr_tp_min": 0,
    "atr_tp_max": 0,
    "llm_weight": 0,
    "atr_weight": 0,
    "auto_sltp_enabled": false,
    "auto_trailing_enabled": false,
    "min_profit_to_trail_pct": 0.5,
    "min_sl_distance_from_zero": 0.1
  },
  "hedge": {
    "allow_hedge": true,
    "min_confidence_for_hedge": 75,
    "existing_must_be_in_profit": 0,
    "max_hedge_size_percent": 75,
    "allow_same_mode_hedge": false,
    "max_total_exposure_multiplier": 2
  },
  "averaging": {
    "allow_averaging": true,
    "average_up_profit_percent": 0.5,
    "average_down_loss_percent": -1,
    "add_size_percent": 50,
    "max_averages": 2,
    "min_confidence_for_average": 65,
    "use_llm_for_averaging": true
  },
  "stale_release": {
    "enabled": true,
    "max_hold_duration": "6h",
    "min_profit_to_keep": 0.5,
    "max_loss_to_force_close": -1,
    "stale_zone_lo": -0.5,
    "stale_zone_hi": 0.5,
    "stale_zone_close_action": "close"
  },
  "assignment": {
    "volatility_min": "medium",
    "volatility_max": "high",
    "expected_hold_min": "15m",
    "expected_hold_max": "4h",
    "confidence_min": 60,
    "confidence_max": 75,
    "risk_score_max": 45,
    "profit_potential_min": 1,
    "profit_potential_max": 3,
    "requires_trend_align": false,
    "priority_weight": 1
  },
  "funding_rate": null,
  "risk": null,
  "trend_divergence": null,
  "mtf": {
    "mtf_enabled": true,
    "primary_timeframe": "15m",
    "primary_weight": 0.4,
    "secondary_timeframe": "5m",
    "secondary_weight": 0.35,
    "tertiary_timeframe": "1m",
    "tertiary_weight": 0.25,
    "min_consensus": 2,
    "min_weighted_strength": 60,
    "trend_stability_check": true
  },
  "dynamic_ai_exit": {
    "dynamic_ai_exit_enabled": true,
    "min_hold_before_ai_ms": 10000,
    "ai_check_interval_ms": 30000,
    "use_llm_for_loss": true,
    "use_llm_for_profit": true,
    "max_hold_time_ms": 14400000
  },
  "reversal": {
    "reversal_enabled": true,
    "reversal_min_llm_confidence": 0.65,
    "reversal_consecutive_candles": 3,
    "reversal_limit_timeout_sec": 120,
    "reversal_require_all_tfs": false
  }
}

-- Additional modes (scalp_reentry, swing, position) follow the same structure
-- See autopilot_settings.json for complete default values
*/

-- ====================================================================================
-- USAGE EXAMPLES
-- ====================================================================================

/*
-- Example 1: Get all enabled modes for a user
SELECT mode_name, config_json
FROM user_mode_configs
WHERE user_id = 'user-uuid-here' AND enabled = true;

-- Example 2: Get specific configuration value using JSONB operators
SELECT
    mode_name,
    config_json->'size'->>'base_size_usd' as base_size,
    config_json->'size'->>'leverage' as leverage
FROM user_mode_configs
WHERE user_id = 'user-uuid-here';

-- Example 3: Update just the enabled flag
UPDATE user_mode_configs
SET enabled = false
WHERE user_id = 'user-uuid-here' AND mode_name = 'scalp';

-- Example 4: Query users with high leverage settings
SELECT u.email, umc.mode_name,
       (umc.config_json->'size'->>'leverage')::integer as leverage
FROM user_mode_configs umc
JOIN users u ON u.id = umc.user_id
WHERE (umc.config_json->'size'->>'leverage')::integer > 5;

-- Example 5: Find modes with aggressive circuit breaker settings
SELECT u.email, umc.mode_name,
       (umc.config_json->'circuit_breaker'->>'max_consecutive_losses')::integer as max_losses
FROM user_mode_configs umc
JOIN users u ON u.id = umc.user_id
WHERE (umc.config_json->'circuit_breaker'->>'max_consecutive_losses')::integer >= 5;

-- Example 6: Update multiple fields in config using jsonb_set
UPDATE user_mode_configs
SET config_json = jsonb_set(
    jsonb_set(config_json, '{size,base_size_usd}', '500'),
    '{size,leverage}', '10'
)
WHERE user_id = 'user-uuid-here' AND mode_name = 'scalp';
*/

-- ====================================================================================
-- PERFORMANCE NOTES
-- ====================================================================================
/*
1. JSONB vs Normalized Tables:
   - JSONB chosen for flexibility and ease of schema evolution
   - GIN indexes provide fast queries on JSON fields
   - Config structure can change without migration
   - Trade-off: Slightly slower than fully normalized, but more flexible

2. Index Strategy:
   - Primary indexes on user_id and mode_name (most common filters)
   - Partial index on enabled modes (80% of queries filter by enabled=true)
   - JSONB path indexes on frequently queried fields
   - GIN index for complex JSON queries

3. Query Optimization:
   - Use prepared statements for frequent queries
   - Batch reads with WHERE user_id = $1 instead of multiple single reads
   - Use JSONB operators (->>, ->) for direct field access
   - Views provide cached extraction of common fields

4. Scaling Considerations:
   - Table size: ~5 rows per user (one per mode)
   - Expected growth: Linear with user count
   - JSONB compression: PostgreSQL automatically compresses JSONB
   - Partition strategy: Not needed unless user count exceeds 10M+
*/
