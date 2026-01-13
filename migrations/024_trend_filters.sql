-- Migration 024: Refactor MTF and add trend_filters
-- UP

-- 1. Add trend_filters column
ALTER TABLE user_mode_configs
ADD COLUMN IF NOT EXISTS trend_filters JSONB DEFAULT NULL;

COMMENT ON COLUMN user_mode_configs.trend_filters IS
'Trend validation filters: BTC check, Price/EMA, VWAP alignment';

-- 2. Migrate old MTF structure to new format (for existing users)
UPDATE user_mode_configs
SET mtf = jsonb_build_object(
    'enabled', COALESCE((mtf->>'mtf_enabled')::boolean, true),
    'higher_tf', jsonb_build_object(
        'enabled', true,
        'timeframe', CASE
            WHEN mode_name = 'position' THEN '1d'
            WHEN mode_name = 'swing' THEN '4h'
            WHEN mode_name = 'scalp' THEN '1h'
            WHEN mode_name = 'ultra_fast' THEN '15m'
            ELSE '1h'
        END,
        'block_on_disagreement', true,
        'check_ema_trend', true,
        'ema_fast', 20,
        'ema_slow', 50
    ),
    'trading_tf', jsonb_build_object(
        'timeframe', COALESCE(mtf->>'primary_timeframe', '15m'),
        'require_alignment', true
    )
)
WHERE mtf IS NOT NULL AND mtf ? 'mtf_enabled';

-- 3. Populate default trend_filters for existing users
UPDATE user_mode_configs
SET trend_filters = jsonb_build_object(
    'btc_trend_check', jsonb_build_object(
        'enabled', CASE WHEN mode_name = 'ultra_fast' THEN false ELSE true END,
        'btc_symbol', 'BTCUSDT',
        'block_alt_long_when_btc_bearish', true,
        'block_alt_short_when_btc_bullish', true,
        'btc_trend_timeframe', CASE
            WHEN mode_name = 'position' THEN '4h'
            WHEN mode_name = 'swing' THEN '1h'
            WHEN mode_name = 'scalp' THEN '15m'
            WHEN mode_name = 'ultra_fast' THEN '5m'
            ELSE '15m'
        END
    ),
    'price_vs_ema', jsonb_build_object(
        'enabled', true,
        'require_price_above_ema_for_long', true,
        'require_price_below_ema_for_short', true,
        'ema_period', CASE
            WHEN mode_name = 'position' THEN 100
            WHEN mode_name = 'swing' THEN 50
            WHEN mode_name = 'scalp' THEN 20
            WHEN mode_name = 'ultra_fast' THEN 9
            ELSE 20
        END
    ),
    'vwap_filter', jsonb_build_object(
        'enabled', CASE WHEN mode_name = 'position' THEN false ELSE true END,
        'require_price_above_vwap_for_long', true,
        'require_price_below_vwap_for_short', true,
        'near_vwap_tolerance_percent', CASE
            WHEN mode_name = 'position' THEN 0.0
            WHEN mode_name = 'swing' THEN 0.2
            WHEN mode_name = 'scalp' THEN 0.1
            WHEN mode_name = 'ultra_fast' THEN 0.05
            ELSE 0.1
        END
    )
)
WHERE trend_filters IS NULL;

-- DOWN
ALTER TABLE user_mode_configs
DROP COLUMN IF EXISTS trend_filters;
