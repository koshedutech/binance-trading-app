// Mode-Strategy Settings Types
// Epic 11: Position Decision Engine - Story 11.33: Mode-Strategy UI Update

// ==================== Strategy Names ====================

/**
 * Available trading strategy names.
 */
export type StrategyName =
  | 'trend_following'
  | 'mean_reversion'
  | 'breakout'
  | 'range_trading';

/**
 * Trading mode names.
 */
export type ModeName = 'scalp' | 'swing' | 'position' | 'ultra_fast';

// ==================== Strategy Configuration Types ====================

/**
 * Timeframe configuration for a strategy.
 */
export interface StrategyTimeframe {
  trend_timeframe: string;
  entry_timeframe: string;
  analysis_timeframe: string;
}

/**
 * SLTP (Stop Loss / Take Profit) configuration.
 */
export interface SLTPConfig {
  sl_percent: number;
  tp1_percent: number;
  tp2_percent: number;
  tp3_percent: number;
  trailing_enabled: boolean;
  trailing_activation_pct: number;
  trailing_stop_pct: number;
}

/**
 * Confidence thresholds configuration.
 */
export interface ConfidenceConfig {
  min_confidence: number;
  high_confidence: number;
  ultra_confidence: number;
}

/**
 * Entry conditions for Trend Following and general strategies.
 */
export interface TrendEntryConditions {
  adx_min: number;
  require_trend_align: boolean;
  min_volume_multiplier: number;
}

/**
 * Entry conditions for Mean Reversion strategy.
 */
export interface MeanReversionEntryConditions {
  rsi_oversold: number;
  rsi_overbought: number;
  bollinger_std: number;
  require_price_at_band: boolean;
}

/**
 * Entry conditions for Breakout strategy.
 */
export interface BreakoutEntryConditions {
  breakout_atr_multiplier: number;
  volume_spike_multiplier: number;
  require_consolidation: boolean;
  consolidation_bars: number;
}

/**
 * Entry conditions for Range Trading strategy.
 */
export interface RangeEntryConditions {
  range_high_touch: boolean;
  range_low_touch: boolean;
  range_width_atr: number;
  min_range_duration_bars: number;
}

/**
 * Union type for all entry conditions.
 */
export type EntryConditionsConfig =
  | TrendEntryConditions
  | MeanReversionEntryConditions
  | BreakoutEntryConditions
  | RangeEntryConditions
  | Record<string, unknown>;

/**
 * Exit conditions configuration.
 */
export interface ExitConditionsConfig {
  use_ai_exit?: boolean;
  exit_at_mean?: boolean;
  exit_at_range_boundary?: boolean;
  max_hold_minutes: number;
  early_warning_enabled: boolean;
}

/**
 * Scoring weights configuration.
 */
export interface ScoringConfig {
  technical_weight: number;
  momentum_weight: number;
  volume_weight: number;
  sentiment_weight: number;
}

// ==================== Complete Strategy Configuration ====================

/**
 * Complete configuration for a single strategy within a mode.
 */
export interface ModeStrategyConfig {
  enabled: boolean;
  priority: number;
  supported_regimes: string[];
  leverage: number;
  max_positions: number;
  base_size_usd: number;
  timeframe: StrategyTimeframe;
  sltp: SLTPConfig;
  confidence: ConfidenceConfig;
  entry_conditions: EntryConditionsConfig;
  exit_conditions: ExitConditionsConfig;
  scoring: ScoringConfig;
}

/**
 * Complete configuration for a trading mode.
 */
export interface ModeConfig {
  name: ModeName;
  enabled: boolean;
  default_strategy: StrategyName;
  auto_select_strategy: boolean;
  strategies: Record<StrategyName, ModeStrategyConfig>;
}

/**
 * All modes configuration.
 */
export type AllModesConfig = Record<ModeName, ModeConfig>;

// ==================== API Request/Response Types ====================

/**
 * Response for getting all mode strategies.
 * Backend returns: { mode: string, strategies: ModeStrategyResponse[] }
 */
export interface GetModeStrategiesResponse {
  mode: string;
  strategies: ModeStrategyResponse[];
}

/**
 * Response from backend for a single strategy.
 */
export interface ModeStrategyResponse {
  mode: string;
  strategy: string;
  enabled: boolean;
  priority: number;
  supported_regimes: string[];
  settings: {
    leverage: number;
    max_positions: number;
    base_size_usd: number;
    timeframe: StrategyTimeframe;
    sltp: SLTPConfig;
    confidence: ConfidenceConfig;
    exit_conditions: ExitConditionsConfig;
    scoring: ScoringConfig;
    entry_conditions?: EntryConditionsConfig;
  };
}

/**
 * Response for getting a specific strategy.
 * Backend returns the ModeStrategyResponse directly (no wrapper)
 */
export type GetModeStrategyResponse = ModeStrategyResponse;

/**
 * Request for updating a strategy.
 */
export interface UpdateModeStrategyRequest {
  enabled?: boolean;
  priority?: number;
  leverage?: number;
  max_positions?: number;
  base_size_usd?: number;
  timeframe?: Partial<StrategyTimeframe>;
  sltp?: Partial<SLTPConfig>;
  confidence?: Partial<ConfidenceConfig>;
  entry_conditions?: Partial<EntryConditionsConfig>;
  exit_conditions?: Partial<ExitConditionsConfig>;
  scoring?: Partial<ScoringConfig>;
}

/**
 * Response for updating a strategy.
 * Backend returns: { success: true, message: string, data: ModeStrategyResponse }
 */
export interface UpdateModeStrategyResponse {
  success: boolean;
  message: string;
  data: ModeStrategyResponse;
}

/**
 * Response for resetting a strategy to defaults.
 * Backend returns: { success: true, message: string, data: ModeStrategyResponse }
 */
export interface ResetModeStrategyResponse {
  success: boolean;
  message: string;
  data: ModeStrategyResponse;
}

/**
 * Comparison field between current and default values.
 */
export interface StrategyFieldComparison {
  path: string;
  current: unknown;
  default: unknown;
  match: boolean;
}

/**
 * Comparison response for a strategy.
 */
export interface StrategyComparisonResponse {
  success: boolean;
  mode: ModeName;
  strategy: StrategyName;
  enabled?: boolean;  // Strategy enabled status
  all_match: boolean;
  total_fields: number;
  matching_fields: number;
  differences: StrategyFieldComparison[];
  all_values?: StrategyFieldComparison[];  // All fields including matches
}

// ==================== UI State Types ====================

/**
 * Loading state for mode strategy settings.
 */
export interface ModeStrategyLoadingState {
  mode: boolean;
  strategy: boolean;
  update: boolean;
  reset: boolean;
  comparison: boolean;
}

/**
 * Error state for mode strategy settings.
 */
export interface ModeStrategyErrorState {
  mode: string | null;
  strategy: string | null;
  update: string | null;
  reset: string | null;
  comparison: string | null;
}

/**
 * Expanded sections state for collapsible UI.
 */
export interface ExpandedSections {
  position: boolean;
  sltp: boolean;
  confidence: boolean;
  entry: boolean;
  exit: boolean;
  scoring: boolean;
}

// ==================== Display Constants ====================

/**
 * Display names for strategy types.
 */
export const STRATEGY_DISPLAY_NAMES: Record<StrategyName, string> = {
  trend_following: 'Trend Following',
  mean_reversion: 'Mean Reversion',
  breakout: 'Breakout',
  range_trading: 'Range Trading',
};

/**
 * Display names for trading modes.
 */
export const MODE_DISPLAY_NAMES: Record<ModeName, string> = {
  scalp: 'Scalp',
  swing: 'Swing',
  position: 'Position',
  ultra_fast: 'Ultra Fast',
};

/**
 * Strategy descriptions.
 */
export const STRATEGY_DESCRIPTIONS: Record<StrategyName, string> = {
  trend_following: 'Follows established trends with momentum and volume confirmation. Best for trending markets.',
  mean_reversion: 'Trades reversals when price deviates from mean. Best for ranging markets with RSI extremes.',
  breakout: 'Trades breakouts from consolidation patterns with volume confirmation. Best for transitions.',
  range_trading: 'Trades between support and resistance levels. Best for sideways markets.',
};

/**
 * Mode descriptions.
 */
export const MODE_DESCRIPTIONS: Record<ModeName, string> = {
  scalp: 'Quick trades with small profit targets. High frequency, low hold time.',
  swing: 'Medium-term trades capturing larger moves. Moderate frequency and hold time.',
  position: 'Long-term trades following major trends. Low frequency, high hold time.',
  ultra_fast: 'Very quick in-and-out trades. Highest frequency, minimal hold time.',
};

/**
 * Supported market regimes for display.
 */
export const REGIME_DISPLAY_NAMES: Record<string, string> = {
  TRENDING: 'Trending',
  VOLATILE_TRENDING: 'Volatile Trending',
  RANGING: 'Ranging',
  MEAN_REVERTING: 'Mean Reverting',
  BREAKOUT: 'Breakout',
  LOW_VOLATILITY: 'Low Volatility',
};

// ==================== Default Values ====================

/**
 * Default SLTP configuration.
 */
export const DEFAULT_SLTP_CONFIG: SLTPConfig = {
  sl_percent: 2.0,
  tp1_percent: 0.5,
  tp2_percent: 1.0,
  tp3_percent: 1.5,
  trailing_enabled: false,
  trailing_activation_pct: 0,
  trailing_stop_pct: 0,
};

/**
 * Default confidence configuration.
 */
export const DEFAULT_CONFIDENCE_CONFIG: ConfidenceConfig = {
  min_confidence: 55,
  high_confidence: 75,
  ultra_confidence: 85,
};

/**
 * Default scoring configuration.
 */
export const DEFAULT_SCORING_CONFIG: ScoringConfig = {
  technical_weight: 40,
  momentum_weight: 30,
  volume_weight: 15,
  sentiment_weight: 15,
};

/**
 * Default expanded sections state.
 */
export const DEFAULT_EXPANDED_SECTIONS: ExpandedSections = {
  position: true,
  sltp: false,
  confidence: false,
  entry: false,
  exit: false,
  scoring: false,
};

// ==================== Utility Functions ====================

/**
 * Check if entry conditions are for trend following strategy.
 */
export function isTrendEntryConditions(
  conditions: EntryConditionsConfig
): conditions is TrendEntryConditions {
  return 'adx_min' in conditions;
}

/**
 * Check if entry conditions are for mean reversion strategy.
 */
export function isMeanReversionEntryConditions(
  conditions: EntryConditionsConfig
): conditions is MeanReversionEntryConditions {
  return 'rsi_oversold' in conditions && 'bollinger_std' in conditions;
}

/**
 * Check if entry conditions are for breakout strategy.
 */
export function isBreakoutEntryConditions(
  conditions: EntryConditionsConfig
): conditions is BreakoutEntryConditions {
  return 'breakout_atr_multiplier' in conditions;
}

/**
 * Check if entry conditions are for range trading strategy.
 */
export function isRangeEntryConditions(
  conditions: EntryConditionsConfig
): conditions is RangeEntryConditions {
  return 'range_high_touch' in conditions;
}

/**
 * Get the entry conditions type for a strategy.
 */
export function getEntryConditionsType(
  strategy: StrategyName
): 'trend' | 'mean_reversion' | 'breakout' | 'range' {
  switch (strategy) {
    case 'trend_following':
      return 'trend';
    case 'mean_reversion':
      return 'mean_reversion';
    case 'breakout':
      return 'breakout';
    case 'range_trading':
      return 'range';
  }
}

/**
 * Format percentage value for display.
 */
export function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`;
}

/**
 * Format timeframe for display.
 */
export function formatTimeframe(timeframe: string): string {
  const match = timeframe.match(/^(\d+)([mhd])$/);
  if (!match) return timeframe;

  const [, num, unit] = match;
  const unitNames: Record<string, string> = {
    m: 'min',
    h: 'hour',
    d: 'day',
  };

  return `${num} ${unitNames[unit]}${parseInt(num) !== 1 ? 's' : ''}`;
}
