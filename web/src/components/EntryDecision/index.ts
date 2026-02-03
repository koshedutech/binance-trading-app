// Entry Decision Components Index
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement

// Main view component
export { default as StrategyFirstView } from './StrategyFirstView';

// Strategy card component
export { default as StrategyCard, StrategyBadge } from './StrategyCard';

// Pattern progress components
export {
  default as PatternProgress,
  PatternProgressFromCoin,
  PatternProgressCompact,
} from './PatternProgress';

// Score display components
export {
  default as ScoreDisplay,
  ScoreDisplayFromCoin,
  ScoreDisplayCompact,
  ScoreBreakdownBar,
  ScoreBreakdownFull,
} from './ScoreDisplay';

// Pattern list container (Story 11.46)
export { default as PatternList } from './PatternList';

// Requirements panel (displays strategy requirements)
export { default as RequirementsPanel, RequirementsBadge } from './RequirementsPanel';

// Coin stage card (real-time coin monitoring with entry levels)
export {
  default as CoinStageCard,
  CoinStageListItem,
  CoinStageGrid,
} from './CoinStageCard';

// Reference candle context (Stage 1 info displayed in Stage 2)
export { default as ReferenceCandleContext } from './ReferenceCandleContext';

// Step 2 Progress Bars (volume and price progress toward entry)
export { default as Step2ProgressBars } from './Step2ProgressBars';
