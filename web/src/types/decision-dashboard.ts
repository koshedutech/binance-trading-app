// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// TypeScript types for Decision Engine Dashboard

/**
 * Time range options for dashboard filtering
 */
export type TimeRange = '7d' | '30d' | '90d' | 'all';

/**
 * Market regime types matching backend
 */
export type MarketRegime = 'TRENDING' | 'RANGING' | 'VOLATILE' | 'CONSOLIDATING';

/**
 * Blocking reason categories
 */
export type BlockingCategory = 'HARD_BLOCK' | 'SOFT_BLOCK' | 'WARNING';

/**
 * Score distribution bucket data
 */
export interface ScoreBucket {
  bucket: string; // "0-25", "26-50", "51-75", "76-100"
  minScore: number;
  maxScore: number;
  tradeCount: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  avgPnL: number;
  totalPnL: number;
}

/**
 * Score distribution data for histogram
 */
export interface ScoreDistribution {
  buckets: ScoreBucket[];
  totalTrades: number;
  averageScore: number;
  medianScore: number;
  highScoreTrades: number; // Trades with score > 75
  lowScoreTrades: number; // Trades with score < 50
}

/**
 * Calibration accuracy comparison data
 */
export interface CalibrationAccuracy {
  strategy: string;
  scoreBucket: string;
  expectedWinRate: number;
  actualWinRate: number;
  delta: number; // actualWinRate - expectedWinRate
  totalTrades: number;
  confidence: string; // "high", "medium", "low" based on trade count
  isAccurate: boolean; // delta within acceptable range
}

/**
 * Overall calibration summary for a strategy
 */
export interface CalibrationSummary {
  strategy: string;
  overallExpected: number;
  overallActual: number;
  overallDelta: number;
  bucketAccuracies: CalibrationAccuracy[];
  totalCalibrationTrades: number;
  lastUpdated: string;
  confidenceLevel: string;
  isReliable: boolean; // Enough trades for reliable calibration
  minimumTradesRequired: number;
}

/**
 * Strategy performance metrics
 */
export interface StrategyPerformance {
  strategyName: string;
  totalTrades: number;
  winningTrades: number;
  losingTrades: number;
  winRate: number;
  totalPnL: number;
  avgPnL: number;
  avgWin: number;
  avgLoss: number;
  largestWin: number;
  largestLoss: number;
  profitFactor: number;
  maxDrawdown: number;
  sharpeRatio: number;
  expectancy: number;
  riskReward: number;
  consecutiveWins: number;
  consecutiveLosses: number;
  lastTradeTime?: string;
  status: string;
  trend: 'up' | 'down' | 'neutral';
  recentPnL: number[];
}

/**
 * Regime classification statistics
 */
export interface RegimeStats {
  regime: MarketRegime;
  count: number;
  percentage: number;
  avgDuration: number; // Average duration in the regime (candles)
  tradesInRegime: number;
  winRateInRegime: number;
  avgPnLInRegime: number;
}

/**
 * Regime distribution summary
 */
export interface RegimeDistribution {
  stats: RegimeStats[];
  totalClassifications: number;
  mostCommonRegime: MarketRegime;
  currentRegime?: MarketRegime;
  periodStart: string;
  periodEnd: string;
}

/**
 * Blocking reason frequency data
 */
export interface BlockingReasonFrequency {
  code: string;
  description: string;
  category: BlockingCategory;
  count: number;
  percentage: number;
  symbolsAffected: number;
}

/**
 * Blocking stats summary
 */
export interface BlockingStatsSummary {
  totalBlocks: number;
  hardBlockCount: number;
  softBlockCount: number;
  warningCount: number;
  byReason: BlockingReasonFrequency[];
  topReasons: BlockingReasonFrequency[];
  symbolsAffected: number;
  mostFrequentBlock: string;
  periodStart: string;
  periodEnd: string;
}

/**
 * Complete dashboard stats response
 */
export interface DashboardStats {
  scoreDistribution: ScoreDistribution;
  calibrationSummary: CalibrationSummary;
  strategyPerformances: StrategyPerformance[];
  regimeDistribution: RegimeDistribution;
  blockingStats: BlockingStatsSummary;
  timeRange: TimeRange;
  generatedAt: string;
}

/**
 * API response wrapper
 */
export interface DashboardResponse {
  success: boolean;
  data?: DashboardStats;
  error?: string;
}

// Display helper constants
export const REGIME_COLORS: Record<MarketRegime, string> = {
  TRENDING: '#22c55e', // green
  RANGING: '#3b82f6', // blue
  VOLATILE: '#f97316', // orange
  CONSOLIDATING: '#6b7280', // gray
};

export const REGIME_LABELS: Record<MarketRegime, string> = {
  TRENDING: 'Trending',
  RANGING: 'Ranging',
  VOLATILE: 'Volatile',
  CONSOLIDATING: 'Consolidating',
};

export const BLOCKING_CATEGORY_COLORS: Record<BlockingCategory, string> = {
  HARD_BLOCK: '#ef4444', // red
  SOFT_BLOCK: '#f97316', // orange
  WARNING: '#eab308', // yellow
};

export const BLOCKING_CATEGORY_LABELS: Record<BlockingCategory, string> = {
  HARD_BLOCK: 'Hard Block',
  SOFT_BLOCK: 'Soft Block',
  WARNING: 'Warning',
};

export const SCORE_BUCKET_COLORS: string[] = [
  '#ef4444', // 0-25: red
  '#f97316', // 26-50: orange
  '#eab308', // 51-75: yellow
  '#22c55e', // 76-100: green
];

export const TIME_RANGE_LABELS: Record<TimeRange, string> = {
  '7d': 'Last 7 Days',
  '30d': 'Last 30 Days',
  '90d': 'Last 90 Days',
  'all': 'All Time',
};

// Utility functions
export function getAccuracyStatus(delta: number): { label: string; color: string } {
  const absDelta = Math.abs(delta);
  if (absDelta <= 5) {
    return { label: 'Excellent', color: '#22c55e' };
  } else if (absDelta <= 10) {
    return { label: 'Good', color: '#84cc16' };
  } else if (absDelta <= 15) {
    return { label: 'Fair', color: '#eab308' };
  } else {
    return { label: 'Needs Calibration', color: '#ef4444' };
  }
}

export function formatPercentage(value: number, decimals: number = 1): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(decimals)}%`;
}

export function formatPnL(value: number): string {
  const formatted = Math.abs(value).toFixed(2);
  return value >= 0 ? `+$${formatted}` : `-$${formatted}`;
}

export function getStrategyTrend(recentPnL: number[]): 'up' | 'down' | 'neutral' {
  if (!recentPnL || recentPnL.length < 3) return 'neutral';
  const recent = recentPnL.slice(-5);
  const sum = recent.reduce((a, b) => a + b, 0);
  if (sum > 0) return 'up';
  if (sum < 0) return 'down';
  return 'neutral';
}
