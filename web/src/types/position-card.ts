// Position Card Types
// Epic 11: Position Decision Engine - Story 11.21: Position Card UI

// ==================== Market Regime Types ====================

/**
 * Market regime classification for a coin.
 * Determines which trading strategy is most appropriate.
 */
export type MarketRegime = 'TRENDING' | 'RANGING' | 'VOLATILE' | 'CONSOLIDATING';

/**
 * Trend direction on a specific timeframe.
 */
export type TrendDirection = 'BULLISH' | 'BEARISH' | 'NEUTRAL';

/**
 * Decision state for a coin - whether it can be traded.
 */
export type DecisionState = 'READY' | 'BLOCKED' | 'PENDING';

// ==================== Blocking Reason Types ====================

/**
 * Category of blocking reason - determines severity and override capability.
 */
export type BlockingCategory = 'HARD_BLOCK' | 'SOFT_BLOCK' | 'WARNING';

/**
 * Codes for specific blocking conditions.
 */
export type BlockingReasonCode =
  // Hard blocks
  | 'TREND_DIVERGENCE'
  | 'ADX_TOO_LOW'
  | 'CIRCUIT_BREAKER_ACTIVE'
  | 'REGIME_MISMATCH'
  | 'TIMEFRAME_MISALIGN'
  // Soft blocks
  | 'SCORE_BELOW_THRESHOLD'
  | 'LOW_CONFIDENCE'
  | 'WEAK_MOMENTUM'
  // Warnings
  | 'LOW_VOLUME'
  | 'WIDE_SPREAD'
  | 'HIGH_RSI'
  | 'LOW_RSI'
  | 'LOW_WIN_RATE';

/**
 * A single blocking reason with full details.
 */
export interface BlockingReason {
  /** Unique code identifying this blocking reason */
  code: BlockingReasonCode;
  /** Severity category */
  category: BlockingCategory;
  /** Human-readable explanation */
  description: string;
  /** Actual value that caused the block */
  value?: number | string;
  /** Threshold the value was compared against */
  threshold?: number | string;
  /** When this reason was recorded (Unix milliseconds) */
  timestamp: number;
  /** Whether user can override this block */
  overridable: boolean;
}

/**
 * Summary of all blocking reasons for a coin.
 */
export interface BlockingSummary {
  /** Total number of blocking reasons */
  totalReasons: number;
  /** Count of hard blocks (cannot override) */
  hardBlockCount: number;
  /** Count of soft blocks (can override) */
  softBlockCount: number;
  /** Count of warnings (informational) */
  warningCount: number;
  /** Whether any blocking condition prevents trading */
  isBlocked: boolean;
  /** Whether all blocks can be overridden */
  canOverride: boolean;
  /** Most severe blocking reason */
  primaryReason?: BlockingReason;
  /** All blocking reasons */
  allReasons: BlockingReason[];
}

// ==================== Score Types ====================

/**
 * Breakdown of the additive scoring components.
 * Total score = technical + context + llm + history
 */
export interface ScoreBreakdown {
  /** Technical indicator score (0-40) */
  technical: number;
  /** Market context score (0-30) */
  context: number;
  /** LLM analysis score (0-20) */
  llm: number;
  /** Historical performance score (0-10) */
  history: number;
  /** Final combined score (0-100) */
  final: number;
}

/**
 * Maximum values for each score component.
 */
export const SCORE_MAX_VALUES = {
  technical: 40,
  context: 30,
  llm: 20,
  history: 10,
  final: 100,
} as const;

// ==================== Indicator Types ====================

/**
 * Key technical indicators displayed on the position card.
 */
export interface TechnicalIndicators {
  /** Average Directional Index - trend strength (0-100) */
  adx: number;
  /** Relative Strength Index - momentum (0-100) */
  rsi: number;
  /** 9-period Exponential Moving Average */
  ema9: number;
  /** 21-period Exponential Moving Average */
  ema21: number;
  /** Average True Range - volatility */
  atr?: number;
  /** ATR average for comparison */
  atrAverage?: number;
}

// ==================== Coin State Types ====================

/**
 * Complete state for a monitored coin.
 * This is the primary data structure for the Position Card.
 */
export interface CoinState {
  /** Trading pair symbol (e.g., BTCUSDT) */
  symbol: string;
  /** Current price */
  price: number;
  /** Market regime classification */
  regime: MarketRegime;
  /** Currently active strategy */
  activeStrategy: string;
  /** Technical indicators */
  indicators: TechnicalIndicators;
  /** Trend direction on 1-hour timeframe */
  trend1h: TrendDirection;
  /** Trend direction on 15-minute timeframe */
  trend15m: TrendDirection;
  /** Score breakdown */
  scores: ScoreBreakdown;
  /** Blocking reasons summary */
  blocking: BlockingSummary;
  /** Decision state (READY, BLOCKED, PENDING) */
  decision: DecisionState;
  /** Last update timestamp (Unix milliseconds) */
  lastUpdated: number;
}

// ==================== Position Card Props ====================

/**
 * Props for the PositionCard component.
 */
export interface PositionCardProps {
  /** Coin state data */
  state: CoinState;
  /** Whether card starts expanded */
  defaultExpanded?: boolean;
  /** Whether to show trading action buttons */
  showActions?: boolean;
  /** Callback when Enter Long is clicked */
  onEnterLong?: (symbol: string) => void;
  /** Callback when Enter Short is clicked */
  onEnterShort?: (symbol: string) => void;
  /** Callback when Close Position is clicked */
  onClosePosition?: (symbol: string) => void;
  /** Whether there's an active position for this symbol */
  hasActivePosition?: boolean;
  /** Position side if active */
  positionSide?: 'LONG' | 'SHORT';
  /** Compact display mode */
  compact?: boolean;
}

// ==================== WebSocket Event Types ====================

/**
 * WebSocket event type for coin state updates.
 */
export const COIN_STATE_UPDATE_EVENT = 'COIN_STATE_UPDATE' as const;

/**
 * Payload for coin state WebSocket updates.
 */
export interface CoinStateUpdatePayload {
  symbol: string;
  price?: number;
  regime?: MarketRegime;
  activeStrategy?: string;
  adx?: number;
  rsi?: number;
  ema9?: number;
  ema21?: number;
  atr?: number;
  atrAverage?: number;
  trend1h?: TrendDirection;
  trend15m?: TrendDirection;
  scoreTechnical?: number;
  scoreContext?: number;
  scoreLlm?: number;
  scoreHistory?: number;
  scoreFinal?: number;
  blockingReasons?: BlockingReason[];
  decision?: DecisionState;
  lastUpdated: number;
}

// ==================== Display Helpers ====================

/**
 * Display names for market regimes.
 */
export const REGIME_DISPLAY_NAMES: Record<MarketRegime, string> = {
  TRENDING: 'Trending',
  RANGING: 'Ranging',
  VOLATILE: 'Volatile',
  CONSOLIDATING: 'Consolidating',
};

/**
 * Color classes for market regimes.
 */
export const REGIME_COLORS: Record<MarketRegime, { text: string; bg: string; border: string }> = {
  TRENDING: { text: 'text-green-400', bg: 'bg-green-500/20', border: 'border-green-500/30' },
  RANGING: { text: 'text-blue-400', bg: 'bg-blue-500/20', border: 'border-blue-500/30' },
  VOLATILE: { text: 'text-orange-400', bg: 'bg-orange-500/20', border: 'border-orange-500/30' },
  CONSOLIDATING: { text: 'text-purple-400', bg: 'bg-purple-500/20', border: 'border-purple-500/30' },
};

/**
 * Icons/emojis for market regimes.
 */
export const REGIME_ICONS: Record<MarketRegime, string> = {
  TRENDING: '\u2191', // Up arrow
  RANGING: '\u2194', // Left-right arrow
  VOLATILE: '\u26A1', // Lightning bolt
  CONSOLIDATING: '\u25CF', // Circle
};

/**
 * Display names for blocking categories.
 */
export const BLOCKING_CATEGORY_DISPLAY: Record<BlockingCategory, { name: string; color: string; bg: string }> = {
  HARD_BLOCK: { name: 'Hard Block', color: 'text-red-400', bg: 'bg-red-500/20' },
  SOFT_BLOCK: { name: 'Soft Block', color: 'text-yellow-400', bg: 'bg-yellow-500/20' },
  WARNING: { name: 'Warning', color: 'text-blue-400', bg: 'bg-blue-500/20' },
};

/**
 * Human-readable descriptions for blocking reason codes.
 */
export const BLOCKING_CODE_DESCRIPTIONS: Record<BlockingReasonCode, string> = {
  TREND_DIVERGENCE: 'Trend divergence between timeframes',
  ADX_TOO_LOW: 'ADX below minimum threshold',
  CIRCUIT_BREAKER_ACTIVE: 'Circuit breaker is active',
  REGIME_MISMATCH: 'Strategy not suited for current regime',
  TIMEFRAME_MISALIGN: 'Timeframes not aligned',
  SCORE_BELOW_THRESHOLD: 'Score below entry threshold',
  LOW_CONFIDENCE: 'Low confidence in signal',
  WEAK_MOMENTUM: 'Weak momentum indicators',
  LOW_VOLUME: 'Volume below average',
  WIDE_SPREAD: 'Bid-ask spread too wide',
  HIGH_RSI: 'RSI indicates overbought',
  LOW_RSI: 'RSI indicates oversold',
  LOW_WIN_RATE: 'Historical win rate below threshold',
};

/**
 * Display names for trend directions.
 */
export const TREND_DISPLAY: Record<TrendDirection, { name: string; color: string; icon: string }> = {
  BULLISH: { name: 'Bullish', color: 'text-green-400', icon: '\u25B2' },
  BEARISH: { name: 'Bearish', color: 'text-red-400', icon: '\u25BC' },
  NEUTRAL: { name: 'Neutral', color: 'text-gray-400', icon: '\u25C6' },
};

/**
 * Display names for decision states.
 */
export const DECISION_DISPLAY: Record<DecisionState, { name: string; color: string; bg: string }> = {
  READY: { name: 'Ready', color: 'text-green-400', bg: 'bg-green-500/20' },
  BLOCKED: { name: 'Blocked', color: 'text-red-400', bg: 'bg-red-500/20' },
  PENDING: { name: 'Pending', color: 'text-yellow-400', bg: 'bg-yellow-500/20' },
};

// ==================== Utility Functions ====================

/**
 * Calculate score percentage for visual display.
 */
export function getScorePercentage(score: number, max: number): number {
  return Math.min(100, Math.max(0, (score / max) * 100));
}

/**
 * Get color class based on score value.
 */
export function getScoreColor(score: number, max: number): string {
  const percentage = getScorePercentage(score, max);
  if (percentage >= 75) return 'text-green-400';
  if (percentage >= 50) return 'text-yellow-400';
  if (percentage >= 25) return 'text-orange-400';
  return 'text-red-400';
}

/**
 * Get background color class based on score value.
 */
export function getScoreBgColor(score: number, max: number): string {
  const percentage = getScorePercentage(score, max);
  if (percentage >= 75) return 'bg-green-500/30';
  if (percentage >= 50) return 'bg-yellow-500/30';
  if (percentage >= 25) return 'bg-orange-500/30';
  return 'bg-red-500/30';
}

/**
 * Format indicator value for display.
 */
export function formatIndicator(value: number, decimals = 2): string {
  return value.toFixed(decimals);
}

/**
 * Format price with appropriate precision.
 */
export function formatPrice(price: number): string {
  if (price >= 1000) return price.toFixed(2);
  if (price >= 1) return price.toFixed(4);
  return price.toFixed(8);
}

/**
 * Create an empty coin state for placeholder/loading states.
 */
export function createEmptyCoinState(symbol: string): CoinState {
  return {
    symbol,
    price: 0,
    regime: 'CONSOLIDATING',
    activeStrategy: '',
    indicators: {
      adx: 0,
      rsi: 50,
      ema9: 0,
      ema21: 0,
    },
    trend1h: 'NEUTRAL',
    trend15m: 'NEUTRAL',
    scores: {
      technical: 0,
      context: 0,
      llm: 0,
      history: 0,
      final: 0,
    },
    blocking: {
      totalReasons: 0,
      hardBlockCount: 0,
      softBlockCount: 0,
      warningCount: 0,
      isBlocked: false,
      canOverride: true,
      allReasons: [],
    },
    decision: 'PENDING',
    lastUpdated: Date.now(),
  };
}

/**
 * Parse coin state from API response or WebSocket payload.
 */
export function parseCoinStateFromPayload(payload: CoinStateUpdatePayload): Partial<CoinState> {
  const result: Partial<CoinState> = {
    symbol: payload.symbol,
    lastUpdated: payload.lastUpdated,
  };

  if (payload.price !== undefined) result.price = payload.price;
  if (payload.regime !== undefined) result.regime = payload.regime;
  if (payload.activeStrategy !== undefined) result.activeStrategy = payload.activeStrategy;
  if (payload.trend1h !== undefined) result.trend1h = payload.trend1h;
  if (payload.trend15m !== undefined) result.trend15m = payload.trend15m;
  if (payload.decision !== undefined) result.decision = payload.decision;

  // Build indicators object
  const indicators: Partial<TechnicalIndicators> = {};
  if (payload.adx !== undefined) indicators.adx = payload.adx;
  if (payload.rsi !== undefined) indicators.rsi = payload.rsi;
  if (payload.ema9 !== undefined) indicators.ema9 = payload.ema9;
  if (payload.ema21 !== undefined) indicators.ema21 = payload.ema21;
  if (payload.atr !== undefined) indicators.atr = payload.atr;
  if (payload.atrAverage !== undefined) indicators.atrAverage = payload.atrAverage;
  if (Object.keys(indicators).length > 0) {
    result.indicators = indicators as TechnicalIndicators;
  }

  // Build scores object
  const scores: Partial<ScoreBreakdown> = {};
  if (payload.scoreTechnical !== undefined) scores.technical = payload.scoreTechnical;
  if (payload.scoreContext !== undefined) scores.context = payload.scoreContext;
  if (payload.scoreLlm !== undefined) scores.llm = payload.scoreLlm;
  if (payload.scoreHistory !== undefined) scores.history = payload.scoreHistory;
  if (payload.scoreFinal !== undefined) scores.final = payload.scoreFinal;
  if (Object.keys(scores).length > 0) {
    result.scores = scores as ScoreBreakdown;
  }

  // Build blocking summary if blocking reasons provided
  if (payload.blockingReasons !== undefined) {
    const reasons = payload.blockingReasons;
    result.blocking = {
      totalReasons: reasons.length,
      hardBlockCount: reasons.filter(r => r.category === 'HARD_BLOCK').length,
      softBlockCount: reasons.filter(r => r.category === 'SOFT_BLOCK').length,
      warningCount: reasons.filter(r => r.category === 'WARNING').length,
      isBlocked: reasons.some(r => r.category === 'HARD_BLOCK' || r.category === 'SOFT_BLOCK'),
      canOverride: reasons.every(r => r.overridable),
      primaryReason: reasons.find(r => r.category === 'HARD_BLOCK') ||
                     reasons.find(r => r.category === 'SOFT_BLOCK') ||
                     reasons[0],
      allReasons: reasons,
    };
  }

  return result;
}

/**
 * Merge partial coin state update into existing state.
 */
export function mergeCoinState(current: CoinState, update: Partial<CoinState>): CoinState {
  return {
    ...current,
    ...update,
    indicators: {
      ...current.indicators,
      ...(update.indicators || {}),
    },
    scores: {
      ...current.scores,
      ...(update.scores || {}),
    },
    blocking: update.blocking || current.blocking,
    lastUpdated: update.lastUpdated || Date.now(),
  };
}
