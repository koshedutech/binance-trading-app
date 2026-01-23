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
 * Story 11.41: Expanded with ATR-based settings
 */
export interface SLTPConfig {
  sl_percent: number;
  tp1_percent: number;
  tp1_sell_percent?: number;
  tp2_percent: number;
  tp2_sell_percent?: number;
  tp3_percent: number;
  tp3_sell_percent?: number;
  trailing_enabled: boolean;
  trailing_activation_pct: number;
  trailing_stop_pct: number;
  use_atr_based?: boolean;
  atr_sl_multiplier?: number;
  atr_tp_multiplier?: number;
  min_sl_distance_pct?: number;
}

// ==================== Story 11.41: Additional Section Types ====================

/**
 * Position sizing configuration.
 */
export interface PositionSizingConfig {
  leverage: number;
  max_positions: number;
  base_size_usd: number;
  max_size_usd?: number;
  min_position_size_usd?: number;
  safety_margin?: number;
  auto_size_enabled?: boolean;
  auto_size_min_cover_fee?: number;
}

/**
 * Multi-timeframe analysis configuration.
 */
export interface MTFConfig {
  enabled: boolean;
  primary_timeframe: string;
  primary_weight: number;
  secondary_timeframe: string;
  secondary_weight: number;
  tertiary_timeframe: string;
  tertiary_weight: number;
  min_consensus: number;
  min_weighted_strength: number;
  trend_stability_check: boolean;
}

/**
 * Per-strategy circuit breaker configuration.
 */
export interface StrategyCircuitBreakerConfig {
  max_loss_per_hour_usd: number;
  max_loss_per_day_usd: number;
  max_consecutive_losses: number;
  cooldown_minutes: number;
  max_trades_per_hour: number;
  max_trades_per_day: number;
  win_rate_check_after: number;
  min_win_rate_pct: number;
}

/**
 * Hedging configuration.
 */
export interface HedgeConfig {
  allow_hedge: boolean;
  min_confidence_for_hedge: number;
  existing_must_be_in_profit_pct: number;
  max_hedge_size_percent: number;
  allow_same_mode_hedge: boolean;
  max_total_exposure_multiplier: number;
}

/**
 * Position averaging configuration.
 */
export interface AveragingConfig {
  allow_averaging: boolean;
  average_up_profit_percent: number;
  average_down_loss_percent: number;
  add_size_percent: number;
  max_averages: number;
  min_confidence_for_average: number;
  use_llm_for_averaging: boolean;
  staged_entry_enabled: boolean;
  staged_entry_levels: number;
  staged_entry_percent?: number[];
}

/**
 * Stale position release configuration.
 */
export interface StaleReleaseConfig {
  enabled: boolean;
  max_hold_duration_minutes: number;
  min_profit_to_keep_pct: number;
  max_loss_to_force_close_pct: number;
  stale_zone_lo_pct: number;
  stale_zone_hi_pct: number;
  stale_zone_action: string;
}

/**
 * Position optimization configuration.
 */
export interface PositionOptimizationConfig {
  reentry_enabled: boolean;
  reentry_after_tp1: boolean;
  reentry_min_pullback_pct: number;
  max_reentries_per_position: number;
  dynamic_sl_enabled: boolean;
  dynamic_sl_at_breakeven_pct: number;
  profit_protection_enabled: boolean;
  profit_protection_trigger_pct: number;
  profit_protection_lock_pct: number;
}

/**
 * Funding rate management configuration.
 */
export interface FundingRateConfig {
  enabled: boolean;
  max_funding_rate_pct: number;
  exit_before_funding_minutes: number;
  block_entry_above_rate_pct: number;
}

/**
 * Risk management configuration.
 */
export interface RiskConfig {
  risk_level: string;
  max_drawdown_percent: number;
  max_daily_loss_percent: number;
  position_risk_percent: number;
}

/**
 * Trend divergence detection configuration.
 */
export interface TrendDivergenceConfig {
  enabled: boolean;
  min_divergence_percent: number;
  block_on_divergence: boolean;
  divergence_weight: number;
}

/**
 * Dynamic AI exit configuration.
 */
export interface DynamicAIExitConfig {
  enabled: boolean;
  min_hold_before_ai_ms: number;
  ai_check_interval_ms: number;
  use_llm_for_loss: boolean;
  use_llm_for_profit: boolean;
  max_hold_time_ms: number;
}

/**
 * Per-strategy early warning configuration.
 */
export interface StrategyEarlyWarningConfig {
  enabled: boolean;
  start_after_minutes: number;
  min_loss_percent: number;
  check_interval_secs: number;
  close_min_hold_mins: number;
}

/**
 * All section names for mode-strategy config.
 */
export type SectionName =
  | 'position_sizing'
  | 'timeframe'
  | 'mtf'
  | 'sltp'
  | 'confidence'
  | 'entry_conditions'
  | 'exit_conditions'
  | 'scoring'
  | 'circuit_breaker'
  | 'hedge'
  | 'averaging'
  | 'stale_release'
  | 'position_optimization'
  | 'funding_rate'
  | 'risk'
  | 'trend_divergence'
  | 'dynamic_ai_exit'
  | 'early_warning';

/**
 * Display names for all sections.
 */
export const SECTION_DISPLAY_NAMES: Record<SectionName, string> = {
  position_sizing: 'Position Sizing',
  timeframe: 'Timeframe',
  mtf: 'Multi-Timeframe Analysis',
  sltp: 'Stop Loss / Take Profit',
  confidence: 'Confidence Thresholds',
  entry_conditions: 'Entry Conditions',
  exit_conditions: 'Exit Conditions',
  scoring: 'Scoring Weights',
  circuit_breaker: 'Circuit Breaker',
  hedge: 'Hedging',
  averaging: 'Position Averaging',
  stale_release: 'Stale Position Release',
  position_optimization: 'Position Optimization',
  funding_rate: 'Funding Rate',
  risk: 'Risk Management',
  trend_divergence: 'Trend Divergence',
  dynamic_ai_exit: 'Dynamic AI Exit',
  early_warning: 'Early Warning',
};

/**
 * Section descriptions for help text.
 */
export const SECTION_DESCRIPTIONS: Record<SectionName, string> = {
  position_sizing: 'Configure leverage, position limits, and sizing rules',
  timeframe: 'Set timeframes for trend, entry, and analysis',
  mtf: 'Configure multi-timeframe analysis with consensus requirements',
  sltp: 'Set stop loss, take profit levels, and trailing stop settings',
  confidence: 'Define minimum confidence thresholds for trade entry',
  entry_conditions: 'Set technical indicator requirements for entry',
  exit_conditions: 'Configure exit rules and hold time limits',
  scoring: 'Adjust weights for different scoring factors',
  circuit_breaker: 'Set per-strategy loss limits and cooldown periods',
  hedge: 'Configure hedging rules and exposure limits',
  averaging: 'Set rules for averaging into positions',
  stale_release: 'Configure automatic release of stale positions',
  position_optimization: 'Enable re-entry, dynamic SL, and profit protection',
  funding_rate: 'Manage positions around funding rate events',
  risk: 'Set overall risk level and drawdown limits',
  trend_divergence: 'Detect and handle divergence between price and trend',
  dynamic_ai_exit: 'Configure AI-driven exit decisions',
  early_warning: 'Set early warning system for underwater positions',
};

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
 * Story 11.41: Expanded to include all 18 sections
 */
export interface ModeStrategyConfig {
  enabled: boolean;
  priority: number;
  supported_regimes: string[];
  // Legacy position sizing fields (backward compatibility)
  leverage: number;
  max_positions: number;
  base_size_usd: number;
  // Required sections
  timeframe: StrategyTimeframe;
  sltp: SLTPConfig;
  confidence: ConfidenceConfig;
  entry_conditions: EntryConditionsConfig;
  exit_conditions: ExitConditionsConfig;
  scoring: ScoringConfig;
  // Story 11.41: New optional sections (18 total)
  position_sizing?: PositionSizingConfig;
  mtf?: MTFConfig;
  circuit_breaker?: StrategyCircuitBreakerConfig;
  hedge?: HedgeConfig;
  averaging?: AveragingConfig;
  stale_release?: StaleReleaseConfig;
  position_optimization?: PositionOptimizationConfig;
  funding_rate?: FundingRateConfig;
  risk?: RiskConfig;
  trend_divergence?: TrendDivergenceConfig;
  dynamic_ai_exit?: DynamicAIExitConfig;
  early_warning?: StrategyEarlyWarningConfig;
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
 * Story 11.41: Expanded to include all 18 sections
 */
export interface ModeStrategyResponse {
  mode: string;
  strategy: string;
  enabled: boolean;
  priority: number;
  supported_regimes: string[];
  settings: {
    // Legacy position sizing fields (backward compatibility)
    leverage: number;
    max_positions: number;
    base_size_usd: number;
    // Required sections
    timeframe: StrategyTimeframe;
    sltp: SLTPConfig;
    confidence: ConfidenceConfig;
    exit_conditions: ExitConditionsConfig;
    scoring: ScoringConfig;
    entry_conditions?: EntryConditionsConfig;
    // Story 11.41: All 18 sections
    position_sizing?: PositionSizingConfig;
    mtf?: MTFConfig;
    circuit_breaker?: StrategyCircuitBreakerConfig;
    hedge?: HedgeConfig;
    averaging?: AveragingConfig;
    stale_release?: StaleReleaseConfig;
    position_optimization?: PositionOptimizationConfig;
    funding_rate?: FundingRateConfig;
    risk?: RiskConfig;
    trend_divergence?: TrendDivergenceConfig;
    dynamic_ai_exit?: DynamicAIExitConfig;
    early_warning?: StrategyEarlyWarningConfig;
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

// ==================== Story 11.41: Section-Level API Types ====================

/**
 * Response for getting a specific section.
 */
export interface GetSectionResponse {
  success: boolean;
  mode: string;
  strategy: string;
  section: SectionName;
  data: unknown;
}

/**
 * Response for updating a specific section.
 */
export interface UpdateSectionResponse {
  success: boolean;
  message: string;
  mode: string;
  strategy: string;
  section: SectionName;
  data: unknown;
}

/**
 * Response for resetting a specific section.
 */
export interface ResetSectionResponse {
  success: boolean;
  message: string;
  mode: string;
  strategy: string;
  section: SectionName;
  data: unknown;
}

/**
 * Metadata for a single section.
 */
export interface SectionMeta {
  is_default: boolean;
  has_data: boolean;
}

/**
 * Response for listing all sections.
 */
export interface ListSectionsResponse {
  success: boolean;
  mode: string;
  strategy: string;
  sections: Record<SectionName, unknown>;
  section_meta: Record<SectionName, SectionMeta>;
  total_sections: number;
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
 * Story 11.41: Expanded to include all 18 sections
 */
export interface ExpandedSections {
  position_sizing: boolean;
  timeframe: boolean;
  mtf: boolean;
  sltp: boolean;
  confidence: boolean;
  entry_conditions: boolean;
  exit_conditions: boolean;
  scoring: boolean;
  circuit_breaker: boolean;
  hedge: boolean;
  averaging: boolean;
  stale_release: boolean;
  position_optimization: boolean;
  funding_rate: boolean;
  risk: boolean;
  trend_divergence: boolean;
  dynamic_ai_exit: boolean;
  early_warning: boolean;
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
 * Default MTF configuration.
 */
export const DEFAULT_MTF_CONFIG: MTFConfig = {
  enabled: false,
  primary_timeframe: '15m',
  primary_weight: 50,
  secondary_timeframe: '1h',
  secondary_weight: 30,
  tertiary_timeframe: '4h',
  tertiary_weight: 20,
  min_consensus: 2,
  min_weighted_strength: 60,
  trend_stability_check: true,
};

/**
 * Default circuit breaker configuration.
 */
export const DEFAULT_CIRCUIT_BREAKER_CONFIG: StrategyCircuitBreakerConfig = {
  max_loss_per_hour_usd: 500,
  max_loss_per_day_usd: 2000,
  max_consecutive_losses: 5,
  cooldown_minutes: 30,
  max_trades_per_hour: 20,
  max_trades_per_day: 100,
  win_rate_check_after: 10,
  min_win_rate_pct: 40,
};

/**
 * Default hedge configuration.
 */
export const DEFAULT_HEDGE_CONFIG: HedgeConfig = {
  allow_hedge: false,
  min_confidence_for_hedge: 75,
  existing_must_be_in_profit_pct: 2,
  max_hedge_size_percent: 50,
  allow_same_mode_hedge: false,
  max_total_exposure_multiplier: 2,
};

/**
 * Default averaging configuration.
 */
export const DEFAULT_AVERAGING_CONFIG: AveragingConfig = {
  allow_averaging: false,
  average_up_profit_percent: 3,
  average_down_loss_percent: 3,
  add_size_percent: 50,
  max_averages: 2,
  min_confidence_for_average: 70,
  use_llm_for_averaging: false,
  staged_entry_enabled: false,
  staged_entry_levels: 3,
};

/**
 * Default stale release configuration.
 */
export const DEFAULT_STALE_RELEASE_CONFIG: StaleReleaseConfig = {
  enabled: false,
  max_hold_duration_minutes: 1440,
  min_profit_to_keep_pct: 0.5,
  max_loss_to_force_close_pct: 5,
  stale_zone_lo_pct: -1,
  stale_zone_hi_pct: 1,
  stale_zone_action: 'hold',
};

/**
 * Default position optimization configuration.
 */
export const DEFAULT_POSITION_OPTIMIZATION_CONFIG: PositionOptimizationConfig = {
  reentry_enabled: false,
  reentry_after_tp1: false,
  reentry_min_pullback_pct: 1,
  max_reentries_per_position: 1,
  dynamic_sl_enabled: false,
  dynamic_sl_at_breakeven_pct: 2,
  profit_protection_enabled: false,
  profit_protection_trigger_pct: 5,
  profit_protection_lock_pct: 3,
};

/**
 * Default funding rate configuration.
 */
export const DEFAULT_FUNDING_RATE_CONFIG: FundingRateConfig = {
  enabled: false,
  max_funding_rate_pct: 0.1,
  exit_before_funding_minutes: 5,
  block_entry_above_rate_pct: 0.2,
};

/**
 * Default risk configuration.
 */
export const DEFAULT_RISK_CONFIG: RiskConfig = {
  risk_level: 'medium',
  max_drawdown_percent: 20,
  max_daily_loss_percent: 5,
  position_risk_percent: 2,
};

/**
 * Default trend divergence configuration.
 */
export const DEFAULT_TREND_DIVERGENCE_CONFIG: TrendDivergenceConfig = {
  enabled: false,
  min_divergence_percent: 5,
  block_on_divergence: false,
  divergence_weight: 20,
};

/**
 * Default dynamic AI exit configuration.
 */
export const DEFAULT_DYNAMIC_AI_EXIT_CONFIG: DynamicAIExitConfig = {
  enabled: false,
  min_hold_before_ai_ms: 60000,
  ai_check_interval_ms: 30000,
  use_llm_for_loss: true,
  use_llm_for_profit: true,
  max_hold_time_ms: 3600000,
};

/**
 * Default early warning configuration.
 */
export const DEFAULT_EARLY_WARNING_CONFIG: StrategyEarlyWarningConfig = {
  enabled: false,
  start_after_minutes: 5,
  min_loss_percent: 1,
  check_interval_secs: 30,
  close_min_hold_mins: 10,
};

/**
 * Default expanded sections state.
 * Story 11.41: Expanded to include all 18 sections
 */
export const DEFAULT_EXPANDED_SECTIONS: ExpandedSections = {
  position_sizing: true,
  timeframe: false,
  mtf: false,
  sltp: false,
  confidence: false,
  entry_conditions: false,
  exit_conditions: false,
  scoring: false,
  circuit_breaker: false,
  hedge: false,
  averaging: false,
  stale_release: false,
  position_optimization: false,
  funding_rate: false,
  risk: false,
  trend_divergence: false,
  dynamic_ai_exit: false,
  early_warning: false,
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
