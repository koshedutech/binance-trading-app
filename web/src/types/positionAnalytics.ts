// Epic 10: Exit Optimization Engine - Story 10.2: Position Analytics Dashboard
// TypeScript types for Position Analytics

/**
 * Time range options for dashboard filtering
 */
export type TimeRange = '7d' | '30d' | '90d' | 'all';

/**
 * Aggregation type for timeline data
 */
export type AggregationType = 'hourly' | 'daily';

/**
 * Market regime types matching backend
 */
export type MarketRegime = 'TRENDING' | 'RANGING' | 'VOLATILE' | 'CONSOLIDATING';

/**
 * Trading modes
 */
export type TradingMode = 'ultra_fast' | 'scalp' | 'swing' | 'position';

/**
 * Exit reasons
 */
export type ExitReason = 'TP_HIT' | 'SL_HIT' | 'TRAILING_STOP' | 'MANUAL' | 'EFFICIENCY_EXIT' | 'LIQUIDATION';

/**
 * Decision modes
 */
export type DecisionMode = 'classic' | 'new_engine';

// ==================== Summary Types ====================

/**
 * Overall position analytics summary
 */
export interface PositionAnalyticsSummary {
  totalTrades: number;
  overallWinRate: number;
  overallEfficiency: number;
  totalPnL: number;
  avgPnL: number;
  bestMode: string;
  bestStrategy: string;
  modeMetrics: ModePerformanceMetrics[];
  strategyMetrics: StrategyPerformanceMetrics[];
  regimeMetrics: RegimePerformanceMetrics[];
  periodStart: string;
  periodEnd: string;
}

// ==================== Timeline Types ====================

/**
 * Single data point in efficiency timeline
 */
export interface EfficiencyTimelineDataPoint {
  timestamp: string; // ISO date string
  avgEfficiency: number;
  tradeCount: number;
  winRate: number;
  avgPnL: number;
}

/**
 * Efficiency timeline response
 */
export interface EfficiencyTimelineResponse {
  dataPoints: EfficiencyTimelineDataPoint[];
  aggregation: AggregationType;
  trendSlope: number; // positive = improving
  periodStart: string;
  periodEnd: string;
}

// ==================== Distribution Types ====================

/**
 * Distribution item for pie charts
 */
export interface DistributionItem {
  label: string;
  count: number;
  percentage: number;
}

/**
 * Distribution by mode
 */
export interface ModeDistribution extends DistributionItem {
  mode: TradingMode | string;
}

/**
 * Distribution by exit reason
 */
export interface ExitReasonDistribution extends DistributionItem {
  reason: ExitReason | string;
}

/**
 * Distribution by strategy
 */
export interface StrategyDistribution extends DistributionItem {
  strategy: string;
}

/**
 * Win/Loss by decision mode
 */
export interface DecisionModeWinLoss {
  mode: DecisionMode | string;
  wins: number;
  losses: number;
  winRate: number;
  totalTrades: number;
}

/**
 * Complete trade distribution response
 */
export interface TradeDistributionResponse {
  byMode: ModeDistribution[];
  byExitReason: ExitReasonDistribution[];
  byStrategy: StrategyDistribution[];
  byDecisionMode: DecisionModeWinLoss[];
  totalTrades: number;
  periodStart: string;
  periodEnd: string;
}

// ==================== Performance Metrics Types ====================

/**
 * Performance metrics grouped by trading mode
 */
export interface ModePerformanceMetrics {
  mode: string;
  totalTrades: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  avgEfficiency: number;
  avgPeakProfit: number;
  avgExitProfit: number;
  totalPnL: number;
  avgPnL: number;
  profitFactor: number;
  breakevenRate: number;
  tp1HitRate: number;
  avgHoldMinutes: number;
}

/**
 * Performance metrics grouped by entry strategy
 */
export interface StrategyPerformanceMetrics {
  strategy: string;
  totalTrades: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  avgEfficiency: number;
  totalPnL: number;
  avgPnL: number;
  avgHoldMinutes: number;
  breakevenRate: number;
  tp1HitRate: number;
}

/**
 * Performance metrics grouped by market regime
 */
export interface RegimePerformanceMetrics {
  regime: MarketRegime | string;
  totalTrades: number;
  winCount: number;
  lossCount: number;
  winRate: number;
  avgEfficiency: number;
  totalPnL: number;
  avgPnL: number;
}

// ==================== Export Types ====================

/**
 * Export format options
 */
export type ExportFormat = 'csv' | 'json';

/**
 * Export request parameters
 */
export interface ExportRequest {
  format: ExportFormat;
  range: TimeRange;
  mode?: string;
}

/**
 * Export response containing file data
 */
export interface ExportResponse {
  filename: string;
  contentType: string;
  data: string; // base64 encoded or raw string
}

// ==================== API Response Types ====================

/**
 * Generic API response wrapper
 */
export interface PositionAnalyticsResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

/**
 * Summary endpoint response
 */
export type SummaryResponse = PositionAnalyticsResponse<PositionAnalyticsSummary>;

/**
 * Timeline endpoint response
 */
export type TimelineResponse = PositionAnalyticsResponse<EfficiencyTimelineResponse>;

/**
 * Distribution endpoint response
 */
export type DistributionResponse = PositionAnalyticsResponse<TradeDistributionResponse>;

// ==================== Filter/Query Types ====================

/**
 * Query parameters for analytics endpoints
 */
export interface AnalyticsQueryParams {
  range: TimeRange;
  mode?: string;
}

// ==================== Display Constants ====================

export const TIME_RANGE_LABELS: Record<TimeRange, string> = {
  '7d': 'Last 7 Days',
  '30d': 'Last 30 Days',
  '90d': 'Last 90 Days',
  'all': 'All Time',
};

export const TIME_RANGE_OPTIONS: { value: TimeRange; label: string }[] = [
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days' },
  { value: '90d', label: 'Last 90 Days' },
  { value: 'all', label: 'All Time' },
];

export const MODE_COLORS: Record<string, string> = {
  ultra_fast: '#f97316', // orange
  scalp: '#3b82f6',     // blue
  swing: '#22c55e',     // green
  position: '#8b5cf6',  // purple
  unknown: '#6b7280',   // gray
};

export const MODE_LABELS: Record<string, string> = {
  ultra_fast: 'Ultra Fast',
  scalp: 'Scalp',
  swing: 'Swing',
  position: 'Position',
};

export const EXIT_REASON_COLORS: Record<string, string> = {
  TP_HIT: '#22c55e',        // green
  SL_HIT: '#ef4444',        // red
  TRAILING_STOP: '#3b82f6', // blue
  MANUAL: '#8b5cf6',        // purple
  EFFICIENCY_EXIT: '#f97316', // orange
  LIQUIDATION: '#dc2626',   // dark red
  unknown: '#6b7280',       // gray
};

export const EXIT_REASON_LABELS: Record<string, string> = {
  TP_HIT: 'Take Profit',
  SL_HIT: 'Stop Loss',
  TRAILING_STOP: 'Trailing Stop',
  MANUAL: 'Manual',
  EFFICIENCY_EXIT: 'Efficiency Exit',
  LIQUIDATION: 'Liquidation',
};

export const DECISION_MODE_COLORS: Record<string, string> = {
  classic: '#3b82f6',     // blue
  new_engine: '#22c55e',  // green
};

export const DECISION_MODE_LABELS: Record<string, string> = {
  classic: 'Classic',
  new_engine: 'New Engine',
};

export const REGIME_COLORS: Record<string, string> = {
  TRENDING: '#22c55e',      // green
  RANGING: '#3b82f6',       // blue
  VOLATILE: '#f97316',      // orange
  CONSOLIDATING: '#6b7280', // gray
};

export const REGIME_LABELS: Record<string, string> = {
  TRENDING: 'Trending',
  RANGING: 'Ranging',
  VOLATILE: 'Volatile',
  CONSOLIDATING: 'Consolidating',
};

// Chart color palettes
export const CHART_COLORS = [
  '#3b82f6', // blue
  '#22c55e', // green
  '#f97316', // orange
  '#8b5cf6', // purple
  '#ef4444', // red
  '#eab308', // yellow
  '#06b6d4', // cyan
  '#ec4899', // pink
];

// ==================== Utility Functions ====================

/**
 * Format percentage with sign
 */
export function formatPercentage(value: number | null | undefined, decimals: number = 1): string {
  if (value == null || isNaN(value)) return '-';
  const formatted = value.toFixed(decimals);
  return value >= 0 ? `+${formatted}%` : `${formatted}%`;
}

/**
 * Format PnL value with sign and currency
 */
export function formatPnL(value: number | null | undefined, currency: string = '$'): string {
  if (value == null || isNaN(value)) return '-';
  const abs = Math.abs(value).toFixed(2);
  return value >= 0 ? `+${currency}${abs}` : `-${currency}${abs}`;
}

/**
 * Format efficiency as percentage
 */
export function formatEfficiency(value: number | null | undefined): string {
  if (value == null || isNaN(value)) return '-';
  return `${value.toFixed(1)}%`;
}

/**
 * Format duration in minutes to human readable
 */
export function formatDuration(minutes: number): string {
  if (minutes < 60) {
    return `${Math.round(minutes)}m`;
  }
  const hours = Math.floor(minutes / 60);
  const mins = Math.round(minutes % 60);
  if (hours < 24) {
    return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
  }
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
}

/**
 * Get color for a mode
 */
export function getModeColor(mode: string): string {
  return MODE_COLORS[mode.toLowerCase()] || MODE_COLORS.unknown;
}

/**
 * Get color for an exit reason
 */
export function getExitReasonColor(reason: string): string {
  return EXIT_REASON_COLORS[reason.toUpperCase()] || EXIT_REASON_COLORS.unknown;
}

/**
 * Get trend indicator based on slope value
 */
export function getTrendIndicator(slope: number): { label: string; color: string; icon: 'up' | 'down' | 'neutral' } {
  if (slope > 0.5) {
    return { label: 'Improving', color: '#22c55e', icon: 'up' };
  } else if (slope < -0.5) {
    return { label: 'Declining', color: '#ef4444', icon: 'down' };
  }
  return { label: 'Stable', color: '#6b7280', icon: 'neutral' };
}

/**
 * Calculate date range from TimeRange
 */
export function getDateRange(range: TimeRange): { start: Date; end: Date } {
  const end = new Date();
  const start = new Date();

  switch (range) {
    case '7d':
      start.setDate(end.getDate() - 7);
      break;
    case '30d':
      start.setDate(end.getDate() - 30);
      break;
    case '90d':
      start.setDate(end.getDate() - 90);
      break;
    case 'all':
      start.setFullYear(2020); // Reasonable start date
      break;
  }

  return { start, end };
}

/**
 * Format date for export filename
 */
export function formatDateForFilename(date: Date): string {
  return date.toISOString().split('T')[0];
}

/**
 * Generate export filename
 */
export function generateExportFilename(range: TimeRange, format: ExportFormat): string {
  const { start, end } = getDateRange(range);
  return `position-analytics-${formatDateForFilename(start)}-${formatDateForFilename(end)}.${format}`;
}
