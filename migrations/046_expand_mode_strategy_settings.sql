-- Migration 046: Expand mode-strategy settings for comprehensive 18-section configuration
-- Story 11.41: Comprehensive Mode-Strategy Configuration Amendment
--
-- This migration ensures the existing JSONB column supports the expanded configuration structure.
-- The user_mode_strategy_settings table already uses JSONB which natively supports schema evolution.
-- No data migration needed - new fields will be added when settings are updated.

-- Add GIN index for efficient JSONB queries if not exists
-- This enables fast lookups by mode, strategy, or any nested field
CREATE INDEX IF NOT EXISTS idx_user_mode_strategy_settings_settings_gin
ON user_mode_strategy_settings USING GIN (settings);

-- Add index for querying by mode name within JSONB
CREATE INDEX IF NOT EXISTS idx_user_mode_strategy_settings_mode_name
ON user_mode_strategy_settings USING btree ((settings->>'name'));

-- Add index for querying enabled strategies
CREATE INDEX IF NOT EXISTS idx_user_mode_strategy_settings_enabled
ON user_mode_strategy_settings USING btree ((settings->>'enabled'));

-- Document the expanded schema structure (18 sections per strategy)
-- This is a reference comment for the expected JSONB structure:
--
-- modes.{scalp|swing|position|ultra_fast}.strategies.{trend_following|mean_reversion|breakout|range_trading}:
--   1. position_sizing: { leverage, max_positions, base_size_usd, max_size_usd, min_position_size_usd, safety_margin, auto_size_enabled, auto_size_min_cover_fee }
--   2. timeframe: { trend_timeframe, entry_timeframe, analysis_timeframe }
--   3. mtf: { enabled, primary_timeframe, primary_weight, secondary_timeframe, secondary_weight, tertiary_timeframe, tertiary_weight, min_consensus, min_weighted_strength, trend_stability_check }
--   4. sltp: { sl_percent, tp1_percent, tp1_sell_percent, tp2_percent, tp2_sell_percent, tp3_percent, tp3_sell_percent, trailing_enabled, trailing_activation_pct, trailing_stop_pct, use_atr_based, atr_sl_multiplier, atr_tp_multiplier, min_sl_distance_pct }
--   5. confidence: { min_confidence, high_confidence, ultra_confidence }
--   6. entry_conditions: { adx_min, adx_max, rsi_min, rsi_max, require_trend_align, min_volume_multiplier, use_limit_entry, limit_order_gap_percent, max_limit_gap_percent }
--   7. exit_conditions: { use_ai_exit, max_hold_minutes, early_warning_enabled, exit_on_trend_reversal, adx_exit_threshold }
--   8. scoring: { technical_weight, momentum_weight, volume_weight, sentiment_weight, min_score, high_score }
--   9. circuit_breaker: { max_loss_per_hour_usd, max_loss_per_day_usd, max_consecutive_losses, cooldown_minutes, max_trades_per_hour, max_trades_per_day, win_rate_check_after, min_win_rate_pct }
--  10. hedge: { allow_hedge, min_confidence_for_hedge, existing_must_be_in_profit_pct, max_hedge_size_percent, allow_same_mode_hedge, max_total_exposure_multiplier }
--  11. averaging: { allow_averaging, average_up_profit_percent, average_down_loss_percent, add_size_percent, max_averages, min_confidence_for_average, use_llm_for_averaging, staged_entry_enabled, staged_entry_levels, staged_entry_percent }
--  12. stale_release: { enabled, max_hold_duration_minutes, min_profit_to_keep_pct, max_loss_to_force_close_pct, stale_zone_lo_pct, stale_zone_hi_pct, stale_zone_action }
--  13. position_optimization: { reentry_enabled, reentry_after_tp1, reentry_min_pullback_pct, max_reentries_per_position, dynamic_sl_enabled, dynamic_sl_at_breakeven_pct, profit_protection_enabled, profit_protection_trigger_pct, profit_protection_lock_pct }
--  14. funding_rate: { enabled, max_funding_rate_pct, exit_before_funding_minutes, block_entry_above_rate_pct }
--  15. risk: { risk_level, max_drawdown_percent, max_daily_loss_percent, position_risk_percent }
--  16. trend_divergence: { enabled, min_divergence_percent, block_on_divergence, divergence_weight }
--  17. dynamic_ai_exit: { enabled, min_hold_before_ai_ms, ai_check_interval_ms, use_llm_for_loss, use_llm_for_profit, max_hold_time_ms }
--  18. early_warning: { enabled, start_after_minutes, min_loss_percent, check_interval_secs, close_min_hold_mins }
--
-- BACKWARD COMPATIBILITY:
-- - Existing settings with legacy structure (leverage, max_positions, base_size_usd at root level) remain valid
-- - New code should prefer position_sizing sub-object when present
-- - JSON merging during updates will automatically add new fields from default-settings.json

-- Create a function to validate mode-strategy settings structure (optional enforcement)
CREATE OR REPLACE FUNCTION validate_mode_strategy_settings(settings JSONB)
RETURNS BOOLEAN AS $$
BEGIN
    -- Basic validation: must have enabled and priority
    IF settings ? 'enabled' AND settings ? 'priority' THEN
        RETURN TRUE;
    END IF;
    RETURN FALSE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Add a comment to the table documenting the expanded schema
COMMENT ON TABLE user_mode_strategy_settings IS 'Per-user mode-strategy configurations with 18 comprehensive settings sections per strategy. Story 11.41: Comprehensive Mode-Strategy Configuration Amendment.';
COMMENT ON COLUMN user_mode_strategy_settings.settings IS 'JSONB containing complete strategy configuration including position_sizing, timeframe, mtf, sltp, confidence, entry_conditions, exit_conditions, scoring, circuit_breaker, hedge, averaging, stale_release, position_optimization, funding_rate, risk, trend_divergence, dynamic_ai_exit, and early_warning sections.';
