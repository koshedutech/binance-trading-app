// Entry Decision System Types
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement

// ==================== Strategy Types ====================

/**
 * Strategy evaluation type
 */
export type StrategyType = 'pattern' | 'score';

/**
 * Pattern status for pattern-based strategies
 */
export type PatternStatus =
  | 'watching'
  | 'accumulation'
  | 'consolidating'
  | 'ready'
  | 'failed'
  | 'expired';

/**
 * Trading mode
 */
export type TradingMode = 'scalp' | 'swing' | 'position' | 'ultra_fast';

// ==================== Coin Match Types ====================

/**
 * Reference candle information for pattern tracking
 */
export interface ReferenceCandle {
  /** Reference candle open time */
  open_time: string;
  /** Reference candle close time */
  close_time: string;
  /** Open price */
  open: number;
  /** High price - breakout level for longs */
  high: number;
  /** Low price */
  low: number;
  /** Close price */
  close: number;
  /** Volume */
  volume: number;
  /** Volume multiplier vs average (e.g., 3.2x) */
  volume_multiplier: number;
}

/**
 * Entry candle information when pattern is ready
 */
export interface EntryCandle {
  /** Entry candle open time (UTC) */
  open_time: string;
  /** Entry candle close time (UTC) */
  close_time: string;
  /** Open price */
  open: number;
  /** High price */
  high: number;
  /** Low price */
  low: number;
  /** Close price at breakout detection */
  close: number;
  /** Volume at breakout */
  volume: number;
  /** Volume multiplier vs consolidation avg */
  volume_multiplier: number;
  /** Actual entry price sent for order */
  entry_price: number;
  /** When breakout was detected (UTC) */
  detected_at: string;
  /** "long" or "short" */
  direction: string;
}

/**
 * Represents a single coin's match status against a strategy
 */
export interface CoinMatch {
  /** Coin symbol (e.g., "BTCUSDT") */
  symbol: string;
  /** Last update timestamp */
  updated_at: string;

  // Pattern-based fields
  /** Current step (1, 2, 3, etc.) for pattern strategies */
  step?: number;
  /** Total steps in pattern (e.g., 2 for Volume Imbalance) */
  total_steps?: number;
  /** Current pattern status */
  status?: PatternStatus;
  /** Human-readable details (e.g., "3.2x avg") */
  details?: string;
  /** Detailed info for each step - used for tracking stage display */
  step_details?: StepDetail[];

  // Pattern tracking metrics
  /** Reference candle data (for step 1+) */
  reference_candle?: ReferenceCandle;
  /** Time when reference candle was detected */
  reference_detected_at?: string;
  /** Number of candles since reference */
  candles_since_reference?: number;
  /** Seconds elapsed since reference detection */
  seconds_since_reference?: number;
  /** Current price proximity to breakout (%) - negative = below, positive = above */
  proximity_to_breakout?: number;
  /** Whether current candle looks like potential breakout */
  potential_breakout?: boolean;

  // Entry/Breakout candle (when pattern is ready)
  /** Entry candle data when breakout detected */
  entry_candle?: EntryCandle;
  /** When pattern became ready (UTC) */
  ready_at?: string;
  /** Seconds until ready pattern expires */
  seconds_until_expiry?: number;

  // Score-based fields
  /** Score value (0-100) for score strategies */
  score?: number;
  /** Whether score meets threshold */
  ready?: boolean;

  // Additional context
  /** Trade direction */
  direction?: 'long' | 'short' | 'neutral';
  /** Current price for reference */
  current_price?: number;
  /** 24h volume for reference */
  volume_24h?: number;
  /** Current candle volume */
  current_volume?: number;
  /** Volume threshold to trigger (e.g., 3.0 for 3x avg) */
  volume_threshold?: number;
  /** Average volume used for comparison */
  avg_volume?: number;
  /** Current volume multiplier vs average */
  volume_multiplier?: number;
  /** Distance to volume threshold as percentage (negative = below threshold) */
  volume_distance_percent?: number;
  /** Distance to breakout price as percentage (negative = below entry) */
  price_distance_percent?: number;

  // Position tracking (when position is actually open on Binance)
  /** Whether there's an active position for this coin */
  has_active_position?: boolean;
  /** When position was opened (entry filled) - ISO timestamp */
  position_opened_at?: string;
  /** Position entry price (actual fill price from Binance) */
  position_entry_price?: number;
  /** Position quantity */
  position_quantity?: number;
  /** Chain ID for the position */
  chain_id?: string;
}

// ==================== Strategy Match Types ====================

/**
 * Requirements for a strategy
 */
export interface StrategyRequirements {
  /** Required timeframes */
  timeframes: string[];
  /** Required data fields */
  data_fields?: string[];
  /** Filter criteria */
  filters?: Record<string, unknown>;
  /** Human-readable description */
  description?: string;
}

/**
 * Represents a strategy with all matching coins
 */
export interface StrategyMatch {
  /** Trading mode: "scalp", "swing", "position" */
  mode: string;
  /** Strategy name: "volume_imbalance", "trend_following" */
  strategy: string;
  /** Sub-strategy name: "ravindra_volume_imbalance" */
  sub_strategy: string;

  /** Strategy type: "pattern" or "score" */
  type: StrategyType;
  /** Primary timeframe */
  timeframe: string;
  /** All required timeframes */
  timeframes?: string[];
  /** Score threshold (for score-based only) */
  threshold?: number;
  /** Whether this strategy is enabled */
  enabled: boolean;

  /** All coins being tracked by this strategy */
  coins: CoinMatch[];

  /** Strategy requirements for UI display */
  requirements?: StrategyRequirements;

  /** Next candle close timestamp - for countdown timer */
  next_candle_close?: string;
  /** What direction this strategy is looking for ("long", "short", or "both") */
  looking_for?: string;

  /** Last update timestamp */
  updated_at: string;
}

// ==================== Mode Grouping Types ====================

/**
 * Groups strategies by trading mode
 */
export interface ModeStrategies {
  /** Trading mode */
  mode: string;
  /** All strategies in this mode */
  strategies: StrategyMatch[];
  /** Whether this mode is enabled */
  enabled: boolean;
}

// ==================== API Response Types ====================

/**
 * Response from GET /api/futures/entry-decision/strategies
 */
export interface EntryDecisionStrategiesResponse {
  /** All enabled strategies with their matching coins */
  strategies: StrategyMatch[];
  /** Strategies grouped by mode */
  by_mode?: ModeStrategies[];
  /** Total number of strategies */
  total_strategies: number;
  /** Number of enabled strategies */
  enabled_strategies: number;
  /** Total coins ready for entry */
  total_coins_ready: number;
  /** Total coins being watched */
  total_coins_watching: number;
  /** Response timestamp */
  updated_at: string;

  // Real-time countdown fields (from WebSocket broadcasts)
  /** Next candle close timestamp - used for countdown timer display */
  next_candle_close?: string;
  /** What direction we're looking for ("long", "short", or "both") */
  looking_for?: string;
  /** When patterns were last evaluated */
  last_evaluated_at?: string;
}

/**
 * Step detail for pattern progress
 */
export interface StepDetail {
  /** Step number (1-based) */
  step_number: number;
  /** Step name (e.g., "Volume Spike") */
  name: string;
  /** Whether step is completed */
  completed: boolean;
  /** Completion timestamp */
  completed_at?: string;
  /** Progress display (e.g., "3/6 candles") */
  progress: string;
  /** Additional context */
  details: string;
}

/**
 * Response from GET /api/futures/entry-decision/pattern/:symbol
 */
export interface PatternProgressResponse {
  /** Coin symbol */
  symbol: string;
  /** Trading mode */
  mode: string;
  /** Strategy name */
  strategy: string;
  /** Sub-strategy name */
  sub_strategy: string;
  /** Timeframe */
  timeframe: string;
  /** Current step (1-based) */
  current_step: number;
  /** Total steps in pattern */
  total_steps: number;
  /** Pattern status */
  status: string;
  /** Details for each step */
  step_details: StepDetail[];
  /** Trade direction */
  direction?: string;
  /** When pattern tracking started */
  started_at: string;
  /** Last update */
  updated_at: string;
  /** When pattern expires */
  expires_at?: string;
}

/**
 * Response from GET /api/futures/entry-decision/score/:symbol
 */
export interface ScoreBreakdownResponse {
  /** Coin symbol */
  symbol: string;
  /** Total score (0-100) */
  score: number;
  /** Trade direction */
  direction: string;
  /** Whether score meets threshold */
  ready: boolean;
  /** Score threshold */
  threshold: number;
  /** Score component breakdown */
  components: Record<string, number>;
  /** Last update */
  updated_at: string;
}

/**
 * Entry candidate for ready coins
 */
export interface EntryCandidate {
  /** Coin symbol */
  symbol: string;
  /** Current price */
  current_price: number;
  /** Trading mode */
  mode: string;
  /** Strategy name */
  strategy: string;
  /** Sub-strategy name */
  sub_strategy?: string;
  /** Strategy type */
  type: StrategyType;
  /** Timeframe */
  timeframe: string;

  // Pattern-based
  /** Current step */
  step?: number;
  /** Pattern status */
  status?: PatternStatus;
  /** Progress details */
  details?: string;

  // Score-based
  /** Score value */
  score?: number;
  /** Score threshold */
  threshold?: number;

  /** Trade direction */
  direction: string;
  /** Priority (higher = more urgent) */
  priority: number;
  /** Confidence level */
  confidence?: number;
  /** Last update */
  updated_at: string;
}

/**
 * Response from GET /api/futures/entry-decision/candidates
 */
export interface EntryCandidatesResponse {
  /** All coins ready for entry */
  candidates: EntryCandidate[];
  /** Total number of candidates */
  total_candidates: number;
  /** Response timestamp */
  updated_at: string;
}

// ==================== Display Constants ====================

/**
 * Mode display names
 */
export const MODE_DISPLAY_NAMES: Record<string, string> = {
  scalp: 'Scalp Mode',
  swing: 'Swing Mode',
  position: 'Position Mode',
  ultra_fast: 'Ultra Fast Mode',
};

/**
 * Mode colors for UI
 */
export const MODE_COLORS: Record<string, { bg: string; text: string; border: string }> = {
  scalp: {
    bg: 'bg-blue-500/20',
    text: 'text-blue-400',
    border: 'border-blue-500/30',
  },
  swing: {
    bg: 'bg-green-500/20',
    text: 'text-green-400',
    border: 'border-green-500/30',
  },
  position: {
    bg: 'bg-purple-500/20',
    text: 'text-purple-400',
    border: 'border-purple-500/30',
  },
  ultra_fast: {
    bg: 'bg-orange-500/20',
    text: 'text-orange-400',
    border: 'border-orange-500/30',
  },
};

/**
 * Pattern status colors
 */
export const PATTERN_STATUS_COLORS: Record<PatternStatus, { bg: string; text: string }> = {
  watching: { bg: 'bg-gray-500/20', text: 'text-gray-400' },
  accumulation: { bg: 'bg-blue-500/20', text: 'text-blue-400' },
  consolidating: { bg: 'bg-yellow-500/20', text: 'text-yellow-400' },
  ready: { bg: 'bg-green-500/20', text: 'text-green-400' },
  failed: { bg: 'bg-red-500/20', text: 'text-red-400' },
  expired: { bg: 'bg-gray-500/20', text: 'text-gray-500' },
};

/**
 * Pattern status display labels
 */
export const PATTERN_STATUS_LABELS: Record<PatternStatus, string> = {
  watching: 'Watching',
  accumulation: 'Accumulation',
  consolidating: 'Consolidating',
  ready: 'Ready',
  failed: 'Failed',
  expired: 'Expired',
};

// ==================== Utility Functions ====================

/**
 * Check if a coin match is ready for entry
 */
export function isCoinReady(coin: CoinMatch): boolean {
  // Pattern-based check
  if (coin.status) {
    return coin.status === 'ready';
  }
  // Score-based check
  return coin.ready === true;
}

/**
 * Format strategy name for display
 */
export function formatStrategyName(strategy: string, subStrategy?: string): string {
  const formatted = strategy.split('_').map(word =>
    word.charAt(0).toUpperCase() + word.slice(1)
  ).join(' ');

  if (subStrategy) {
    const subFormatted = subStrategy.split('_').map(word =>
      word.charAt(0).toUpperCase() + word.slice(1)
    ).join(' ');
    return `${formatted} - ${subFormatted}`;
  }

  return formatted;
}

/**
 * Get count of ready coins in a strategy
 */
export function getReadyCount(strategy: StrategyMatch): number {
  return strategy.coins.filter(isCoinReady).length;
}

/**
 * Get count of watching coins in a strategy
 */
export function getWatchingCount(strategy: StrategyMatch): number {
  return strategy.coins.length - getReadyCount(strategy);
}

// ==================== Entry Levels Types ====================

/**
 * Calculated entry, stop-loss, and take-profit levels
 */
export interface EntryLevels {
  /** Entry price (typically reference high for longs) */
  entry_price: number;
  /** Stop loss price */
  stop_loss: number;
  /** Take profit price */
  take_profit: number;
  /** Risk as percentage of entry */
  risk_percent: number;
  /** Reward as percentage of entry */
  reward_percent: number;
  /** Risk/Reward ratio */
  risk_reward_ratio: number;
  /** Reference candle high */
  reference_high?: number;
  /** Reference candle low */
  reference_low?: number;
  /** Current market price */
  current_price?: number;
}

/**
 * Real-time volume progress for UI display
 */
export interface VolumeProgress {
  /** Current forming candle's volume */
  current_volume: number;
  /** Average volume of last N closed candles */
  average_volume: number;
  /** Current volume ratio (current_volume / average_volume) */
  current_ratio: number;
  /** Required ratio to trigger spike (e.g., 3.0) */
  required_ratio: number;
  /** Progress as percentage (current_ratio / required_ratio * 100) */
  progress_percent: number;
  /** Current candle direction: "bullish", "bearish", or "neutral" */
  candle_direction: string;
  /** True if current ratio > 50% of required */
  is_approaching_spike: boolean;
  /** Milliseconds until candle close */
  time_remaining_ms: number;
  /** Number of candles used for average calculation */
  lookback_candles: number;
}

/**
 * Pattern update from WebSocket (ENTRY_DECISION_PATTERN_UPDATE event)
 */
export interface PatternUpdate {
  /** Coin symbol */
  symbol: string;
  /** Timeframe */
  timeframe: string;
  /** Trading mode */
  mode: string;
  /** Strategy name */
  strategy: string;
  /** Sub-strategy name */
  sub_strategy: string;
  /** Current step (1-based) */
  current_step: number;
  /** Total steps in pattern */
  total_steps: number;
  /** Pattern status */
  status: PatternStatus;
  /** Step details */
  step_details: StepDetail[];
  /** Entry levels (when pattern is ready or near-ready) */
  entry_levels?: EntryLevels;
  /** Real-time volume progress (updated on every tick) */
  volume_progress?: VolumeProgress;
  /** Trade direction */
  direction?: string;
  /** What direction we're looking for ("long", "short", or "both") */
  looking_for?: string;
  /** Next candle close timestamp */
  next_candle_close?: string;
  /** When patterns were last evaluated */
  last_evaluated_at?: string;
  /** Last update */
  updated_at: string;
}

/**
 * Response from GET /api/futures/entry-decision/pattern-updates
 */
export interface PatternUpdatesResponse {
  /** All current pattern updates */
  updates: PatternUpdate[];
  /** Count of updates */
  count: number;
}

/**
 * Response from GET /api/futures/entry-decision/entry-levels/:symbol
 */
export interface EntryLevelsApiResponse {
  /** Coin symbol */
  symbol: string;
  /** Entry price */
  entry_price: number;
  /** Stop loss price */
  stop_loss: number;
  /** Take profit price */
  take_profit: number;
  /** Risk percentage */
  risk_percent: number;
  /** Reward percentage */
  reward_percent: number;
  /** Risk/Reward ratio */
  risk_reward_ratio: number;
  /** Current price */
  current_price?: number;
  /** Message if no levels available */
  message?: string;
}
