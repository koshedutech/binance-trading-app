// Strategy Hierarchy Types
// Epic 11: Position Decision Engine - Story 11.46: Volume Imbalance UI Components

// ==================== Pattern States ====================

/**
 * Volume Imbalance pattern states
 */
export type VolumeImbalanceState = 'WATCHING' | 'CONSOLIDATING' | 'READY' | 'EXPIRED';

/**
 * Pattern step status
 */
export type PatternStepStatus = 'pending' | 'in_progress' | 'completed' | 'failed';

// ==================== Trailing Stop Configuration ====================

/**
 * A single trailing stop milestone
 */
export interface TrailingStopMilestone {
  /** Profit percentage to trigger this milestone */
  trigger_profit_pct: number;
  /** Trail distance percentage at this milestone */
  trail_distance_pct: number;
  /** Optional label for display */
  label?: string;
}

/**
 * Complete trailing stop configuration
 */
export interface TrailingStopConfig {
  /** Whether trailing stop is enabled */
  enabled: boolean;
  /** Activation profit percentage */
  activation_profit_pct: number;
  /** Initial trail distance percentage */
  initial_trail_pct: number;
  /** Milestones for dynamic trailing */
  milestones: TrailingStopMilestone[];
}

// ==================== Pattern Detection Configuration ====================

/**
 * Volume imbalance pattern detection parameters
 */
export interface PatternDetectionConfig {
  /** Minimum volume ratio for imbalance detection */
  min_volume_ratio: number;
  /** Consolidation time in minutes */
  consolidation_time_mins: number;
  /** Breakout confirmation candles */
  breakout_confirmation_candles: number;
  /** Maximum pattern age in minutes */
  max_pattern_age_mins: number;
  /** Require higher timeframe confirmation */
  require_htf_confirmation: boolean;
  /** Higher timeframe for confirmation */
  htf_timeframe: string;
}

// ==================== Risk/Reward Configuration ====================

/**
 * Risk/Reward ratio configuration
 */
export interface RiskRewardConfig {
  /** Risk portion of ratio (e.g., 1 for 1:4) */
  risk: number;
  /** Reward portion of ratio (e.g., 4 for 1:4) */
  reward: number;
  /** Minimum R:R to accept trade */
  min_ratio: number;
}

// ==================== Volume Imbalance Settings ====================

/**
 * Complete Volume Imbalance strategy settings
 */
export interface VolumeImbalanceSettings {
  /** Whether this sub-strategy is enabled */
  enabled: boolean;
  /** Risk/Reward configuration */
  risk_reward: RiskRewardConfig;
  /** Whether LLM validation is required */
  llm_validation_enabled: boolean;
  /** LLM provider to use */
  llm_provider?: string;
  /** Trailing stop configuration */
  trailing_stop: TrailingStopConfig;
  /** Pattern detection configuration */
  pattern_detection: PatternDetectionConfig;
  /** Maximum concurrent patterns to track */
  max_concurrent_patterns: number;
  /** Priority relative to other sub-strategies */
  priority: number;
}

// ==================== Volume Imbalance Pattern ====================

/**
 * A single pattern step
 */
export interface PatternStep {
  /** Step name */
  name: string;
  /** Step description */
  description: string;
  /** Current status */
  status: PatternStepStatus;
  /** Timestamp when completed (if applicable) */
  completed_at?: number;
  /** Additional details */
  details?: Record<string, unknown>;
}

/**
 * Trade setup calculated from pattern
 */
export interface TradeSetup {
  /** Entry price */
  entry_price: number;
  /** Stop loss price */
  stop_loss: number;
  /** Take profit price */
  take_profit: number;
  /** Position side */
  side: 'LONG' | 'SHORT';
  /** Calculated R:R ratio */
  risk_reward_ratio: number;
  /** Suggested position size in USDT */
  position_size_usdt?: number;
  /** Suggested leverage */
  leverage?: number;
}

/**
 * Volume Imbalance pattern state
 */
export interface VolumeImbalancePattern {
  /** Unique pattern ID */
  id: string;
  /** Trading symbol */
  symbol: string;
  /** Pattern timeframe */
  timeframe: string;
  /** Current pattern state */
  state: VolumeImbalanceState;
  /** Pattern detection timestamp */
  detected_at: number;
  /** Last update timestamp */
  updated_at: number;
  /** Expiry timestamp */
  expires_at: number;
  /** Pattern progress (0-100) */
  progress_percent: number;
  /** 3-step pattern progress */
  steps: PatternStep[];
  /** Trade setup (available when READY) */
  trade_setup?: TradeSetup;
  /** Volume ratio that triggered detection */
  volume_ratio: number;
  /** Current price */
  current_price: number;
  /** LLM validation result (if validated) */
  llm_validated?: boolean;
  /** LLM validation reason */
  llm_validation_reason?: string;
  /** Consolidation progress (when CONSOLIDATING) */
  consolidation_progress?: number;
  /** Higher timeframe confirmation */
  htf_confirmed?: boolean;
}

// ==================== Strategy Group Settings ====================

/**
 * Base settings for a strategy group
 */
export interface StrategyGroupBaseSettings {
  /** Primary timeframe for analysis */
  timeframe: string;
  /** Position size as percentage of available balance */
  position_size_percent: number;
  /** Maximum leverage allowed */
  max_leverage: number;
  /** Maximum concurrent positions */
  max_positions: number;
  /** Minimum volume in USDT for coin selection */
  min_volume_usdt: number;
}

/**
 * Sub-strategy basic info
 */
export interface SubStrategyInfo {
  /** Sub-strategy identifier */
  id: string;
  /** Display name */
  name: string;
  /** Description */
  description: string;
  /** Whether enabled */
  enabled: boolean;
  /** Priority order */
  priority: number;
  /** Icon name (lucide-react) */
  icon?: string;
}

/**
 * Complete strategy group settings
 */
export interface StrategyGroupSettings {
  /** Group identifier */
  id: string;
  /** Display name (e.g., "Scalp Mode") */
  name: string;
  /** Description */
  description: string;
  /** Whether the entire group is enabled */
  enabled: boolean;
  /** Base settings for all sub-strategies */
  base_settings: StrategyGroupBaseSettings;
  /** List of sub-strategies in this group */
  sub_strategies: SubStrategyInfo[];
  /** Supported market regimes */
  supported_regimes: string[];
  /** Last updated timestamp */
  updated_at: number;
}

// ==================== Sub-Strategy Settings ====================

/**
 * Generic sub-strategy settings (base interface)
 */
export interface SubStrategySettings {
  /** Sub-strategy identifier */
  id: string;
  /** Parent group identifier */
  group_id: string;
  /** Whether enabled */
  enabled: boolean;
  /** Priority */
  priority: number;
  /** Strategy-specific settings (type varies) */
  settings: VolumeImbalanceSettings | Record<string, unknown>;
  /** Last updated timestamp */
  updated_at: number;
}

// ==================== API Response Types ====================

/**
 * Response for getting strategy groups
 */
export interface GetStrategyGroupsResponse {
  success: boolean;
  mode: string;
  groups: StrategyGroupSettings[];
}

/**
 * Response for getting sub-strategies
 */
export interface GetSubStrategiesResponse {
  success: boolean;
  mode: string;
  group: string;
  sub_strategies: SubStrategySettings[];
}

/**
 * Response for getting volume imbalance patterns
 */
export interface GetVolumeImbalancePatternsResponse {
  success: boolean;
  patterns: VolumeImbalancePattern[];
  stats: {
    watching: number;
    consolidating: number;
    ready: number;
    expired_today: number;
  };
}

/**
 * Request for updating strategy group
 */
export interface UpdateStrategyGroupRequest {
  enabled?: boolean;
  base_settings?: Partial<StrategyGroupBaseSettings>;
}

/**
 * Request for updating sub-strategy
 */
export interface UpdateSubStrategyRequest {
  enabled?: boolean;
  priority?: number;
  settings?: Partial<VolumeImbalanceSettings> | Record<string, unknown>;
}

/**
 * Response for update operations
 */
export interface UpdateResponse {
  success: boolean;
  message: string;
}

// ==================== Display Constants ====================

/**
 * State badge colors for Volume Imbalance
 */
export const VOLUME_IMBALANCE_STATE_COLORS: Record<VolumeImbalanceState, {
  bg: string;
  text: string;
  border: string;
}> = {
  WATCHING: {
    bg: 'bg-gray-500/20',
    text: 'text-gray-400',
    border: 'border-gray-500/30',
  },
  CONSOLIDATING: {
    bg: 'bg-yellow-500/20',
    text: 'text-yellow-400',
    border: 'border-yellow-500/30',
  },
  READY: {
    bg: 'bg-green-500/20',
    text: 'text-green-400',
    border: 'border-green-500/30',
  },
  EXPIRED: {
    bg: 'bg-red-500/20',
    text: 'text-red-400',
    border: 'border-red-500/30',
  },
};

/**
 * State display names
 */
export const VOLUME_IMBALANCE_STATE_LABELS: Record<VolumeImbalanceState, string> = {
  WATCHING: 'Watching',
  CONSOLIDATING: 'Consolidating',
  READY: 'Ready to Execute',
  EXPIRED: 'Expired',
};

/**
 * Default Volume Imbalance settings
 */
export const DEFAULT_VOLUME_IMBALANCE_SETTINGS: VolumeImbalanceSettings = {
  enabled: false,
  risk_reward: {
    risk: 1,
    reward: 4,
    min_ratio: 3,
  },
  llm_validation_enabled: true,
  trailing_stop: {
    enabled: true,
    activation_profit_pct: 1.0,
    initial_trail_pct: 0.5,
    milestones: [
      { trigger_profit_pct: 2.0, trail_distance_pct: 0.4, label: 'TP1' },
      { trigger_profit_pct: 3.0, trail_distance_pct: 0.3, label: 'TP2' },
      { trigger_profit_pct: 4.0, trail_distance_pct: 0.2, label: 'Final' },
    ],
  },
  pattern_detection: {
    min_volume_ratio: 2.0,
    consolidation_time_mins: 15,
    breakout_confirmation_candles: 2,
    max_pattern_age_mins: 60,
    require_htf_confirmation: true,
    htf_timeframe: '1h',
  },
  max_concurrent_patterns: 5,
  priority: 1,
};

// ==================== Utility Functions ====================

/**
 * Format R:R ratio for display
 */
export function formatRiskRewardRatio(rr: RiskRewardConfig): string {
  return `${rr.risk}:${rr.reward}`;
}

/**
 * Calculate R:R ratio value
 */
export function calculateRiskRewardValue(rr: RiskRewardConfig): number {
  return rr.reward / rr.risk;
}

/**
 * Format trailing stop milestones for display
 */
export function formatTrailingMilestones(config: TrailingStopConfig): string[] {
  return config.milestones.map((m) =>
    `${m.label || `+${m.trigger_profit_pct}%`}: Trail ${m.trail_distance_pct}%`
  );
}

/**
 * Get step completion count
 */
export function getCompletedSteps(steps: PatternStep[]): number {
  return steps.filter((s) => s.status === 'completed').length;
}

/**
 * Check if pattern is actionable
 */
export function isPatternActionable(pattern: VolumeImbalancePattern): boolean {
  return pattern.state === 'READY' && !!pattern.trade_setup;
}
