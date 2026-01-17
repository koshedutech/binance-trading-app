// Story 7.13: Tree Structure UI for Modification History
// TypeScript interfaces for SL/TP modification tracking and display

// Modification event types
export type ModificationEventType = 'PLACED' | 'MODIFIED' | 'CANCELLED' | 'FILLED';

// Source of modification
export type ModificationSource = 'LLM_AUTO' | 'USER_MANUAL' | 'TRAILING_STOP';

// Impact direction for the change
export type ImpactDirection = 'BETTER' | 'WORSE' | 'TIGHTER' | 'WIDER' | 'INITIAL';

// Order types that can have modification history
export type ModifiableOrderType = 'SL' | 'TP1' | 'TP2' | 'TP3' | 'TP4';

// Market context at time of modification
export interface MarketContext {
  currentPrice: number;
  priceChange1h?: number;
  priceChange24h?: number;
  volatility?: number;
  trend?: 'BULLISH' | 'BEARISH' | 'NEUTRAL';
  atr?: number;
}

// Single modification event
export interface ModificationEvent {
  id: number;
  chainId: string;
  orderType: ModifiableOrderType;
  binanceOrderId?: number;

  // Event classification
  eventType: ModificationEventType;
  modificationSource: ModificationSource;
  version: number;

  // Price tracking
  oldPrice: number | null;
  newPrice: number;
  priceDelta: number | null;
  priceDeltaPercent: number | null;

  // Position context
  positionQuantity: number;
  positionEntryPrice: number;
  positionSide: 'LONG' | 'SHORT';

  // Dollar impact
  dollarImpact: number;
  impactDirection: ImpactDirection;

  // LLM decision tracking
  modificationReason: string;
  llmDecisionId?: string;
  llmConfidence?: number;

  // Market context at modification time
  marketContext?: MarketContext;

  // Timestamp
  createdAt: string; // ISO 8601
}

// Props for the main ModificationTree component
export interface ModificationTreeProps {
  chainId: string;
  orderType: ModifiableOrderType;
  currentPrice: number;
  events: ModificationEvent[];
  positionSide: 'LONG' | 'SHORT';
  isExpanded?: boolean;
  onToggle?: () => void;
  compact?: boolean;
}

// Summary statistics for modification history
export interface ModificationSummaryStats {
  totalModifications: number;
  netPriceChange: number;
  netPriceChangePercent: number;
  netDollarImpact: number;
  initialPrice: number;
  currentPrice: number;
  lastModifiedAt: string;
  sources: {
    llmAuto: number;
    userManual: number;
    trailingStop: number;
  };
}

// Props for individual modification node
export interface ModificationNodeProps {
  event: ModificationEvent;
  isFirst: boolean;
  isLast: boolean;
  previousEvent?: ModificationEvent;
  positionSide: 'LONG' | 'SHORT';
  orderType: ModifiableOrderType;
  onExpandReasoning?: () => void;
  isReasoningExpanded?: boolean;
}

// Props for impact badge component
export interface ImpactBadgeProps {
  amount: number;
  direction: ImpactDirection;
  showTrend?: boolean;
  size?: 'sm' | 'md' | 'lg';
}

// API response type for modification history
export interface ModificationHistoryResponse {
  success: boolean;
  chainId: string;
  orderType: ModifiableOrderType;
  events: ModificationEvent[];
  summary: ModificationSummaryStats;
}

// Helper functions

// Get display color for modification source
export function getSourceColor(source: ModificationSource): { color: string; bg: string } {
  switch (source) {
    case 'LLM_AUTO':
      return { color: 'text-purple-400', bg: 'bg-purple-500/20' };
    case 'USER_MANUAL':
      return { color: 'text-blue-400', bg: 'bg-blue-500/20' };
    case 'TRAILING_STOP':
      return { color: 'text-yellow-400', bg: 'bg-yellow-500/20' };
    default:
      return { color: 'text-gray-400', bg: 'bg-gray-500/20' };
  }
}

// Get display icon for modification source
export function getSourceIcon(source: ModificationSource): string {
  switch (source) {
    case 'LLM_AUTO':
      return '🤖';
    case 'USER_MANUAL':
      return '👤';
    case 'TRAILING_STOP':
      return '📈';
    default:
      return '•';
  }
}

// Get display label for modification source
export function getSourceLabel(source: ModificationSource): string {
  switch (source) {
    case 'LLM_AUTO':
      return 'AI Auto';
    case 'USER_MANUAL':
      return 'Manual';
    case 'TRAILING_STOP':
      return 'Trailing';
    default:
      return 'Unknown';
  }
}

// Get impact color based on direction and order type
export function getImpactColor(direction: ImpactDirection, orderType: ModifiableOrderType): string {
  // For initial placements
  if (direction === 'INITIAL') {
    return 'text-gray-400';
  }

  // For SL: TIGHTER = green (locked profit), WIDER = red (more risk)
  if (orderType === 'SL') {
    if (direction === 'TIGHTER') return 'text-green-400';
    if (direction === 'WIDER') return 'text-red-400';
  }

  // For TP: BETTER = green (more profit), WORSE = red (less profit)
  if (direction === 'BETTER') return 'text-green-400';
  if (direction === 'WORSE') return 'text-red-400';

  return 'text-gray-400';
}

// Get impact background color
export function getImpactBgColor(direction: ImpactDirection, orderType: ModifiableOrderType): string {
  if (direction === 'INITIAL') return 'bg-gray-500/10';

  if (orderType === 'SL') {
    if (direction === 'TIGHTER') return 'bg-green-500/10';
    if (direction === 'WIDER') return 'bg-red-500/10';
  }

  if (direction === 'BETTER') return 'bg-green-500/10';
  if (direction === 'WORSE') return 'bg-red-500/10';

  return 'bg-gray-500/10';
}

// Format dollar amount with sign
export function formatDollarImpact(amount: number): string {
  const sign = amount >= 0 ? '+' : '';
  return `${sign}$${Math.abs(amount).toFixed(2)}`;
}

// Format price delta with sign
export function formatPriceDelta(delta: number): string {
  const sign = delta >= 0 ? '+' : '';
  return `${sign}$${Math.abs(delta).toFixed(2)}`;
}

// Format percentage with sign
export function formatPercentChange(percent: number): string {
  const sign = percent >= 0 ? '+' : '';
  return `${sign}${percent.toFixed(2)}%`;
}

// Calculate summary stats from events
export function calculateSummaryStats(events: ModificationEvent[]): ModificationSummaryStats {
  if (events.length === 0) {
    return {
      totalModifications: 0,
      netPriceChange: 0,
      netPriceChangePercent: 0,
      netDollarImpact: 0,
      initialPrice: 0,
      currentPrice: 0,
      lastModifiedAt: new Date().toISOString(),
      sources: { llmAuto: 0, userManual: 0, trailingStop: 0 },
    };
  }

  // Sort by version to ensure correct order
  const sorted = [...events].sort((a, b) => a.version - b.version);
  const initial = sorted[0];
  const current = sorted[sorted.length - 1];

  const netPriceChange = current.newPrice - initial.newPrice;
  const netPriceChangePercent = initial.newPrice > 0
    ? (netPriceChange / initial.newPrice) * 100
    : 0;

  const netDollarImpact = events.reduce((sum, e) => sum + e.dollarImpact, 0);

  const sources = { llmAuto: 0, userManual: 0, trailingStop: 0 };
  events.forEach(e => {
    if (e.modificationSource === 'LLM_AUTO') sources.llmAuto++;
    if (e.modificationSource === 'USER_MANUAL') sources.userManual++;
    if (e.modificationSource === 'TRAILING_STOP') sources.trailingStop++;
  });

  return {
    totalModifications: events.length - 1, // Exclude initial placement
    netPriceChange,
    netPriceChangePercent,
    netDollarImpact,
    initialPrice: initial.newPrice,
    currentPrice: current.newPrice,
    lastModifiedAt: current.createdAt,
    sources,
  };
}

// Order type display config
export const ORDER_TYPE_LABELS: Record<ModifiableOrderType, { label: string; icon: string; color: string }> = {
  SL: { label: 'Stop Loss', icon: '🛡️', color: 'text-red-400' },
  TP1: { label: 'Take Profit 1', icon: '🎯', color: 'text-cyan-400' },
  TP2: { label: 'Take Profit 2', icon: '🎯', color: 'text-cyan-400' },
  TP3: { label: 'Take Profit 3', icon: '🎯', color: 'text-cyan-400' },
  TP4: { label: 'Take Profit 4', icon: '🎯', color: 'text-cyan-400' },
};
