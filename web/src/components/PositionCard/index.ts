// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// Component exports

export { default as PositionCard, PositionCardSkeleton, PositionCardList } from './PositionCard';
export { default as RegimeBadge, RegimeBadgeSmall } from './RegimeBadge';
export { default as ScoreBreakdown, FinalScoreBadge, ScoreRing } from './ScoreBreakdown';
export { default as BlockingReasons, BlockingReasonsCompact } from './BlockingReasons';
export { default as IndicatorDisplay, IndicatorsInline } from './IndicatorDisplay';

// Hooks
export { useCoinState, useMultipleCoinStates } from './useCoinState';

// Re-export types for convenience
export type {
  MarketRegime,
  TrendDirection,
  DecisionState,
  BlockingCategory,
  BlockingReasonCode,
  BlockingReason,
  BlockingSummary,
  ScoreBreakdown as ScoreBreakdownType,
  TechnicalIndicators,
  CoinState,
  PositionCardProps,
  CoinStateUpdatePayload,
} from '../../types/position-card';

export {
  REGIME_DISPLAY_NAMES,
  REGIME_COLORS,
  REGIME_ICONS,
  BLOCKING_CATEGORY_DISPLAY,
  BLOCKING_CODE_DESCRIPTIONS,
  TREND_DISPLAY,
  DECISION_DISPLAY,
  SCORE_MAX_VALUES,
  getScorePercentage,
  getScoreColor,
  getScoreBgColor,
  formatIndicator,
  formatPrice,
  createEmptyCoinState,
  parseCoinStateFromPayload,
  mergeCoinState,
  COIN_STATE_UPDATE_EVENT,
} from '../../types/position-card';
