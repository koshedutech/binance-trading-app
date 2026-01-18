// Decision Engine Types
// Epic 11: Position Decision Engine - Story 11.24: Decision Engine Settings UI

// Strategy names matching the backend constants
export type StrategyName =
  | 'trend_following'
  | 'mean_reversion'
  | 'breakout'
  | 'range_trading';

// Scoring configuration for additive scoring system
export interface ScoringConfig {
  technical_weight: number;    // Weight for technical indicators (0-2)
  context_weight: number;      // Weight for market context (0-2)
  llm_weight: number;          // Weight for LLM analysis (0-2)
  history_weight: number;      // Weight for historical performance (0-2)
  min_threshold: number;       // Minimum score to consider entry (0-100)
}

// Blocking configuration for trade entry conditions
export interface BlockingConfig {
  adx_minimum: number;           // Minimum ADX for trend strength (0-100)
  score_minimum: number;         // Minimum score for entry (0-100)
  rsi_upper_limit: number;       // RSI upper limit for overbought (0-100)
  rsi_lower_limit: number;       // RSI lower limit for oversold (0-100)
  win_rate_minimum: number;      // Minimum win rate threshold (0-100)
  max_blocking_reasons: number;  // Maximum blocking reasons before hard block
  enable_hard_blocks: boolean;   // Enable hard blocking conditions
  enable_soft_blocks: boolean;   // Enable soft blocking conditions
  enable_warnings: boolean;      // Enable warning conditions
}

// Entry conditions for opening positions
export interface EntryConditions {
  min_score: number;           // Minimum final score required (0-100)
  min_adx: number;             // Minimum ADX value (0-100)
  require_trend_align: boolean; // Require timeframes to agree
  volume_multiplier: number;   // Required volume multiple of average
  min_confidence: number;      // Minimum LLM confidence (0-100)
  max_spread: number;          // Maximum bid-ask spread (percentage)
}

// Exit conditions for closing positions
export interface ExitConditions {
  take_profit_percent: number;     // Profit target as percentage
  stop_loss_percent: number;       // Maximum loss as percentage
  trailing_stop: boolean;          // Enable trailing stop
  trailing_percent: number;        // Trailing stop distance as percentage
  max_hold_time: string;           // Maximum hold duration (e.g., "4h", "1d")
  exit_on_trend_reversal: boolean; // Close on trend reversal
  min_hold_time: string;           // Minimum hold time (e.g., "5m", "1h")
}

// Complete settings for a single trading strategy
export interface StrategySettings {
  name: StrategyName;
  enabled: boolean;
  description: string;

  // Configuration groups
  scoring: ScoringConfig;
  blocking: BlockingConfig;
  entry: EntryConditions;
  exit: ExitConditions;

  // Timeframe settings
  primary_timeframe: string;   // Main analysis timeframe (e.g., "1h", "4h")
  secondary_timeframe: string; // Confirmation timeframe (e.g., "15m", "5m")
}

// Complete decision engine settings for a user
export interface DecisionEngineSettings {
  strategies: Record<StrategyName, StrategySettings>;
  active_strategy: StrategyName;
  last_updated: number;  // Unix milliseconds
  version: number;
}

// Strategy info for listing available strategies
export interface StrategyInfo {
  name: StrategyName;
  display_name: string;
  description: string;
  primary_timeframe: string;
  secondary_timeframe: string;
}

// ==================== Comparison Types ====================

// Single field comparison for settings diff
export interface FieldComparison {
  path: string;
  current: unknown;
  default: unknown;
  match: boolean;
}

// Group comparison (scoring, blocking, entry, exit, etc.)
export interface GroupComparison {
  group_name: string;
  total_fields: number;
  matching_fields: number;
  all_match: boolean;
  fields: FieldComparison[];
}

// Strategy comparison result
export interface StrategyComparison {
  strategy_name: string;
  enabled: boolean;
  total_fields: number;
  matching_fields: number;
  groups: Record<string, GroupComparison>;
}

// Complete comparison response
export interface DecisionEngineComparisonResponse {
  success: boolean;
  preview: boolean;
  config_type: string;
  all_match: boolean;
  total_strategies: number;
  total_fields: number;
  matching_fields: number;
  total_differences: number;
  strategies: Record<StrategyName, StrategyComparison>;
  active_strategy: {
    current: StrategyName;
    default: StrategyName;
    match: boolean;
  };
}

// ==================== API Request/Response Types ====================

// Get settings response
export interface GetDecisionSettingsResponse {
  success: boolean;
  settings: DecisionEngineSettings;
}

// Update settings request
export interface UpdateDecisionSettingsRequest {
  active_strategy?: StrategyName;
  strategies?: Record<string, Partial<StrategySettings>>;
}

// Update settings response
export interface UpdateDecisionSettingsResponse {
  success: boolean;
  message: string;
  settings: DecisionEngineSettings;
}

// Reset settings request
export interface ResetDecisionSettingsRequest {
  strategy?: StrategyName;  // Optional: reset only specific strategy
  // Note: group-level reset is not yet implemented on the backend
  // group?: string;
}

// Reset settings response
export interface ResetDecisionSettingsResponse {
  success: boolean;
  message: string;
}

// List strategies response
export interface ListStrategiesResponse {
  success: boolean;
  strategies: StrategyInfo[];
}

// Set active strategy request
export interface SetActiveStrategyRequest {
  strategy: StrategyName;
}

// Set active strategy response
export interface SetActiveStrategyResponse {
  success: boolean;
  message: string;
  active_strategy: StrategyName;
}

// ==================== UI State Types ====================

// Expanded state for collapsible sections
export interface ExpandedState {
  strategies: Record<StrategyName, boolean>;
  groups: Record<string, Record<string, boolean>>;
}

// Loading state for async operations
export interface DecisionEngineLoadingState {
  settings: boolean;
  comparison: boolean;
  update: boolean;
  reset: boolean;
}

// Error state
export interface DecisionEngineErrorState {
  settings: string | null;
  comparison: string | null;
  update: string | null;
  reset: string | null;
}

// ==================== Helper Types ====================

// Valid timeframe values
export type Timeframe =
  | '1m' | '3m' | '5m' | '15m' | '30m'
  | '1h' | '2h' | '4h' | '6h' | '8h' | '12h'
  | '1d' | '3d' | '1w' | '1M';

// Valid hold time formats
export type HoldTime = `${number}${'m' | 'h' | 'd'}`;

// Group names for settings comparison
export type SettingsGroupName =
  | 'scoring'
  | 'blocking'
  | 'entry'
  | 'exit'
  | 'timeframes'
  | 'general';

// Display names for groups
export const GROUP_DISPLAY_NAMES: Record<SettingsGroupName, string> = {
  scoring: 'Scoring',
  blocking: 'Blocking Rules',
  entry: 'Entry Conditions',
  exit: 'Exit Conditions',
  timeframes: 'Timeframes',
  general: 'General',
};

// Display names for strategies
export const STRATEGY_DISPLAY_NAMES: Record<StrategyName, string> = {
  trend_following: 'Trend Following',
  mean_reversion: 'Mean Reversion',
  breakout: 'Breakout',
  range_trading: 'Range Trading',
};

// Strategy descriptions
export const STRATEGY_DESCRIPTIONS: Record<StrategyName, string> = {
  trend_following: 'Follows established trends with momentum and volume confirmation. Best for trending markets with ADX > 25.',
  mean_reversion: 'Trades reversals when price deviates significantly from mean. Best for ranging markets with RSI extremes.',
  breakout: 'Trades breakouts from consolidation patterns with volume confirmation. Best for transitions from ranging to trending.',
  range_trading: 'Trades between identified support and resistance levels. Best for sideways markets with clear boundaries.',
};
