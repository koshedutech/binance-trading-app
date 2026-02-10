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
 * 2-step pattern: Volume Spike → Breakout (NO consolidation requirement)
 * Based on Dec 2025 - Jan 2026 backtest: 51 trades, 47.1% WR, +1147% net return
 */
/** Trade direction type for pattern detection */
export type TradeDirection = 'long' | 'short' | 'both';

export interface PatternDetectionConfig {
  // ===== Direction Setting =====
  /**
   * Trade direction filter:
   * - "long": Only GREEN (bullish) candles qualify as reference, look for upward breakout
   * - "short": Only RED (bearish) candles qualify as reference, look for downward breakout
   * - "both": Accept any candle, direction determined by candle color
   */
  direction?: TradeDirection;

  // ===== Legacy fields (kept for backwards compatibility) =====
  /** @deprecated Use volume_spike_threshold instead */
  min_volume_ratio?: number;
  /** @deprecated Consolidation phase removed from strategy */
  consolidation_time_mins?: number;
  /** Breakout confirmation candles */
  breakout_confirmation_candles?: number;
  /** Maximum pattern age in minutes */
  max_pattern_age_mins?: number;
  /** Require higher timeframe confirmation */
  require_htf_confirmation?: boolean;
  /** Higher timeframe for confirmation */
  htf_timeframe?: string;
  /** @deprecated Consolidation phase removed from strategy */
  min_consolidation_candles?: number;
  /** @deprecated Consolidation phase removed from strategy */
  max_consolidation_candles?: number;
  /** @deprecated Consolidation phase removed from strategy */
  consolidation_range_tolerance?: number;

  // ===== Active parameters (3m timeframe, 2-step pattern) =====
  /** Number of candles to look back for reference candle selection */
  reference_lookback_candles?: number;
  /** Volume spike threshold multiplier (e.g., 3.0 = 3x average) */
  volume_spike_threshold?: number;
  /** Require pre-trend down before volume spike (BACKTESTED: false) */
  require_pre_trend_down?: boolean;
  /** Breakout volume surge multiplier (e.g., 1.0 = at least equal to reference) */
  breakout_volume_surge?: number;
  /** Entry volume vs reference requirement (e.g., 1.0 = at least equal) */
  entry_volume_vs_reference?: number;
  /** Maximum stop loss percent (caps risk) */
  max_sl_percent?: number;
}

// ==================== Budget Allocation Configuration ====================

/**
 * Per-strategy budget allocation settings
 * Allows independent capital management per strategy
 */
export interface BudgetAllocationConfig {
  /** Initial budget assigned to this strategy in USD */
  assigned_budget_usd: number;
  /** Maximum concurrent trades allowed */
  max_concurrent_trades: number;
  /** Position sizing method: 'all_in' | 'fixed_percent' | 'kelly' */
  position_sizing: string;
  /** Use incremental equity (profits compound into position sizing) */
  use_incremental_equity: boolean;
  /** Current equity value (running balance after trade P&L) - read-only, set by backend */
  current_equity?: number;
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
  /** Budget allocation for this strategy */
  budget_allocation?: BudgetAllocationConfig;
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
  /** Database record identifier (UUID) */
  id: string;
  /** Sub-strategy identifier (e.g., 'ravindra_volume_imbalance') - used for API calls */
  sub_strategy: string;
  /** Trading mode (e.g., 'scalp', 'swing') */
  mode: string;
  /** Strategy group (e.g., 'breakout') */
  strategy_group: string;
  /** Whether enabled */
  enabled: boolean;
  /** Strategy-specific settings (type varies) */
  settings: VolumeImbalanceSettings | Record<string, unknown>;
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
  strategy_group: string;
  /** Whether the parent strategy group is enabled */
  strategy_group_enabled: boolean;
  sub_strategies: SubStrategySettings[];
  count: number;
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
 * 2-step pattern: Volume Spike → Breakout (NO consolidation)
 * Based on Dec 2025 - Jan 2026 backtest: 51 trades, 47.1% WR, +1147% net return (after fees)
 */
export const DEFAULT_VOLUME_IMBALANCE_SETTINGS: VolumeImbalanceSettings = {
  enabled: true,
  risk_reward: {
    risk: 1,
    reward: 4,
    min_ratio: 3,
  },
  llm_validation_enabled: false,
  trailing_stop: {
    enabled: true,
    activation_profit_pct: 2.0,  // Activate at 2:1 R:R
    initial_trail_pct: 0.0,      // Move SL to breakeven (0R profit locked)
    milestones: [
      { trigger_profit_pct: 2.0, trail_distance_pct: 0.0, label: 'BE' },   // At 2:1 → breakeven
      { trigger_profit_pct: 3.0, trail_distance_pct: 1.0, label: '+1R' },  // At 3:1 → lock 1:1
    ],
  },
  pattern_detection: {
    // Direction setting: "long" (GREEN candles), "short" (RED candles), or "both"
    direction: 'long',
    // Legacy fields (kept for backwards compatibility)
    min_volume_ratio: 3.0,
    breakout_confirmation_candles: 1,
    max_pattern_age_mins: 60,
    require_htf_confirmation: false,
    htf_timeframe: '15m',
    // Active parameters (3m timeframe, 2-step: Volume Spike → Breakout)
    // BACKTESTED (Dec 2025 - Jan 2026): 51 trades, 47.1% WR, +1147% net return
    reference_lookback_candles: 5,
    volume_spike_threshold: 3.0,
    require_pre_trend_down: false, // BACKTESTED: false - original strategy did NOT use this filter
    breakout_volume_surge: 1.0,
    entry_volume_vs_reference: 1.0,
    max_sl_percent: 1.5,
  },
  max_concurrent_patterns: 5,
  priority: 1,
  budget_allocation: {
    assigned_budget_usd: 100,
    max_concurrent_trades: 1,
    position_sizing: 'all_in',
    use_incremental_equity: true,
  },
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
