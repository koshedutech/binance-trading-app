// Story 7.15: Order Tree Node Component
// Story 7.19: Enhanced with timezone support via formatTime prop
// Story 7.21: Modifications displayed as tree child nodes under SL/TP orders
// Enhanced: Added duration counter, buy/sell side, order value display
// Individual node in the order chain tree structure
import { useState, useCallback, useEffect } from 'react';
import {
  ChevronDown,
  ChevronRight,
  TrendingUp,
  TrendingDown,
  Target,
  Shield,
  Layers,
  Activity,
  AlertTriangle,
  CheckCircle,
  Clock,
  Edit3,
  Timer,
} from 'lucide-react';
import { ChainOrder, PositionState, ORDER_TYPE_CONFIG, OrderTypeSuffix, PositionAnalyticsData, SLModification } from './types';
import { formatDollarImpact } from './ModificationHistory';
import type { ModificationEvent, ModifiableOrderType } from './ModificationHistory/types';

// Format duration from timestamp to human readable (e.g., "15m", "2h 30m", "1d 5h")
function formatDuration(startTime: number): string {
  const now = Date.now();
  const diffMs = now - startTime;
  if (diffMs < 0) return '0s';

  const seconds = Math.floor(diffMs / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    const remainingHours = hours % 24;
    return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
  }
  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
}

// Format countdown timer (e.g., "2:45" for 2 minutes 45 seconds remaining)
function formatCountdown(remainingMs: number): string {
  if (remainingMs <= 0) return '0:00';

  const totalSeconds = Math.floor(remainingMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;

  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

// Calculate remaining time before order timeout
// Entry orders have 180s timeout for LIMIT orders
const ENTRY_ORDER_TIMEOUT_MS = 180 * 1000; // 180 seconds

// Format a static duration between two timestamps (for closed positions)
function formatStaticDuration(startMs: number, endMs: number): string {
  const diffMs = endMs - startMs;
  if (diffMs <= 0) return '0s';

  const seconds = Math.floor(diffMs / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    const remainingHours = hours % 24;
    return remainingHours > 0 ? `${days}d ${remainingHours}h` : `${days}d`;
  }
  if (hours > 0) {
    const remainingMinutes = minutes % 60;
    return remainingMinutes > 0 ? `${hours}h ${remainingMinutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
}

// Format order value (price * quantity) - null-safe
function formatValue(price: number | null | undefined, quantity: number | null | undefined): string {
  const value = (price ?? 0) * (quantity ?? 0);
  if (value >= 1000) return `$${value.toFixed(2)}`;
  if (value >= 1) return `$${value.toFixed(4)}`;
  return `$${value.toFixed(6)}`;
}

// Tree node types
export type TreeNodeType = 'ENTRY' | 'POSITION' | 'TP' | 'TP1' | 'TP2' | 'TP3' | 'SL' | 'DCA1' | 'DCA2' | 'DCA3' | 'H' | 'HSL' | 'HTP' | 'RB';

interface OrderTreeNodeProps {
  type: TreeNodeType;
  order?: ChainOrder;
  positionState?: PositionState;
  positionAnalytics?: PositionAnalyticsData; // Story 11.40: Position analytics for stage badge
  modificationCount?: number;
  modifications?: ModificationEvent[];
  chainId: string;
  positionSide: 'LONG' | 'SHORT';
  isLast?: boolean;
  depth: number;
  onLoadModifications?: (orderType: ModifiableOrderType) => Promise<ModificationEvent[]>;
  // Story 7.19: Timezone-aware time formatter
  formatTime?: (timestamp: string | number) => string;
  // Expected P&L for TP/SL orders (calculated from entry price)
  entryPrice?: number;
  entryQuantity?: number;
  // Fee rate for P&L calculation (default 0.05% = 0.0005)
  feeRate?: number;
  // Live price for real-time position value updates
  livePrice?: number;
  // Whether this order was the exit order that closed the position
  isExitOrder?: boolean;
  // Reason for cancellation (for SL/TP that were cancelled when opposite hit)
  cancelReason?: string;
  // Story 14.19: Lifecycle status from composite event ('FILLED'|'CANCELED' etc.)
  lifecycleStatus?: string;
  // Story 14.19: Chain closed timestamp for freezing timers
  chainClosedAt?: string;
  // SL modification history from Ravindra monitor (breakeven, profit lock)
  slModifications?: SLModification[];
}

// Get icon for node type
function getNodeIcon(type: TreeNodeType) {
  switch (type) {
    case 'ENTRY':
      return TrendingUp;
    case 'POSITION':
      return Activity;
    case 'TP':
    case 'TP1':
    case 'TP2':
    case 'TP3':
      return Target;
    case 'SL':
      return Shield;
    case 'DCA1':
    case 'DCA2':
    case 'DCA3':
      return Layers;
    case 'H':
    case 'HSL':
    case 'HTP':
      return AlertTriangle;
    case 'RB':
      return Activity;
    default:
      return Activity;
  }
}

// Get status indicator
function getStatusIndicator(status: string) {
  const statusConfig: Record<string, { icon: typeof CheckCircle; color: string; label: string }> = {
    NEW: { icon: Clock, color: 'text-blue-400', label: 'Pending' },
    PARTIALLY_FILLED: { icon: Activity, color: 'text-yellow-400', label: 'Partial' },
    FILLED: { icon: CheckCircle, color: 'text-green-400', label: 'Filled' },
    CANCELED: { icon: AlertTriangle, color: 'text-gray-400', label: 'Cancelled' },
    EXPIRED: { icon: Clock, color: 'text-orange-400', label: 'Expired' },
    ACTIVE: { icon: Activity, color: 'text-green-400', label: 'Active' },
    PARTIAL: { icon: Activity, color: 'text-yellow-400', label: 'Partial' },
    CLOSED: { icon: CheckCircle, color: 'text-blue-400', label: 'Closed' },
  };
  return statusConfig[status] || statusConfig.NEW;
}

// Format price with 8 decimal precision (Binance standard, null-safe)
function formatPrice(price: number | null | undefined): string {
  const safePrice = price ?? 0;
  return safePrice.toFixed(8);
}

// Default time formatter (fallback to simple format)
const defaultFormatTime = (timestamp: string | number): string => {
  try {
    const date = typeof timestamp === 'number' ? new Date(timestamp) : new Date(timestamp);
    return date.toLocaleTimeString('en-GB', { hour12: false });
  } catch {
    return '--:--:--';
  }
};

// Helper to get stage display configuration
function getStageConfig(stage: PositionAnalyticsData['stage']) {
  switch (stage) {
    case 'RISK_ZONE':
      return { label: 'Risk Zone', color: 'text-red-400', bgColor: 'bg-red-500/20' };
    case 'BREAKEVEN':
      return { label: 'Breakeven', color: 'text-yellow-400', bgColor: 'bg-yellow-500/20' };
    case 'TP1':
      return { label: 'TP1+', color: 'text-green-400', bgColor: 'bg-green-500/20' };
    case 'EFFICIENCY':
      return { label: 'Efficiency', color: 'text-cyan-400', bgColor: 'bg-cyan-500/20' };
    default:
      return { label: 'Unknown', color: 'text-gray-400', bgColor: 'bg-gray-500/20' };
  }
}

// Calculate expected P&L for TP/SL orders
function calculateExpectedPnL(
  orderType: TreeNodeType,
  orderPrice: number,
  entryPrice: number,
  quantity: number,
  positionSide: 'LONG' | 'SHORT',
  feeRate: number = 0.0005 // 0.05% default taker fee
): { pnl: number; pnlPercent: number } | null {
  if (!orderPrice || !entryPrice || !quantity || orderPrice <= 0 || entryPrice <= 0) {
    return null;
  }

  // Only calculate for SL and TP orders
  if (!['SL', 'TP', 'TP1', 'TP2', 'TP3'].includes(orderType)) {
    return null;
  }

  // Calculate gross P&L based on position side
  let grossPnL: number;
  if (positionSide === 'LONG') {
    grossPnL = (orderPrice - entryPrice) * quantity;
  } else {
    grossPnL = (entryPrice - orderPrice) * quantity;
  }

  // Calculate fees (entry + exit)
  const entryValue = entryPrice * quantity;
  const exitValue = orderPrice * quantity;
  const totalFees = (entryValue + exitValue) * feeRate;

  // Net P&L after fees
  const netPnL = grossPnL - totalFees;
  const pnlPercent = entryValue > 0 ? (netPnL / entryValue) * 100 : 0;

  return { pnl: netPnL, pnlPercent };
}

export default function OrderTreeNode({
  type,
  order,
  positionState,
  positionAnalytics,
  modificationCount = 0,
  modifications,
  chainId,
  positionSide,
  isLast = false,
  depth,
  onLoadModifications,
  formatTime = defaultFormatTime, // Story 7.19: Use provided formatter or default
  entryPrice,
  entryQuantity,
  feeRate = 0.0005,
  livePrice,
  isExitOrder,
  cancelReason,
  lifecycleStatus,
  chainClosedAt,
  slModifications,
}: OrderTreeNodeProps) {
  // Show SL sub-tree when any SL events exist (including initial placement)
  const hasSLModifications = type === 'SL' && slModifications && slModifications.length > 0;
  const [expanded, setExpanded] = useState(!!hasSLModifications);
  const [localModifications, setLocalModifications] = useState<ModificationEvent[]>(modifications || []);
  const [loadingMods, setLoadingMods] = useState(false);
  const [duration, setDuration] = useState<string>('');
  const [countdown, setCountdown] = useState<string>('');

  // Derive effective lifecycle status: prefer explicit lifecycleStatus from WebSocket,
  // fall back to order's own status for API-sourced closed orders
  const effectiveLifecycleStatus = lifecycleStatus ||
    (order && (order.status === 'FILLED' || order.status === 'CANCELED' || order.status === 'EXPIRED')
      ? order.status : undefined);

  // Update duration counter every 10 seconds for pending orders (non-entry)
  // Update countdown every second for pending entry orders
  // Story 14.19: Show frozen duration for closed chain orders
  useEffect(() => {
    // For closed chain orders (SL/TP), show frozen duration
    if (order && (chainClosedAt || effectiveLifecycleStatus) && (effectiveLifecycleStatus === 'FILLED' || effectiveLifecycleStatus === 'CANCELED' || effectiveLifecycleStatus === 'EXPIRED')) {
      const endTime = effectiveLifecycleStatus === 'FILLED'
        ? (order.updateTime || (chainClosedAt ? new Date(chainClosedAt).getTime() : Date.now()))
        : (chainClosedAt ? new Date(chainClosedAt).getTime() : (order.updateTime || Date.now()));
      setDuration(formatStaticDuration(order.time, endTime));
      setCountdown('');
      return;
    }

    const isPending = order && ['NEW', 'PARTIALLY_FILLED'].includes(order.status);
    if (!isPending || !order) {
      setDuration('');
      setCountdown('');
      return;
    }

    const isEntry = type === 'ENTRY';

    if (isEntry) {
      // Entry orders: show reverse countdown (time remaining before timeout)
      const calculateCountdown = () => {
        const elapsedMs = Date.now() - order.time;
        const remainingMs = ENTRY_ORDER_TIMEOUT_MS - elapsedMs;
        setCountdown(remainingMs > 0 ? formatCountdown(remainingMs) : '0:00');
      };

      // Initial calculation
      calculateCountdown();

      // Update every second for countdown
      const interval = setInterval(calculateCountdown, 1000);
      return () => clearInterval(interval);
    } else {
      // Non-entry orders: show elapsed duration
      setDuration(formatDuration(order.time));
      const interval = setInterval(() => {
        setDuration(formatDuration(order.time));
      }, 10000);
      return () => clearInterval(interval);
    }
  }, [order, type, effectiveLifecycleStatus, chainClosedAt]);

  // Get config for this order type
  const typeKey = type === 'ENTRY' ? 'E' : type;
  const config = ORDER_TYPE_CONFIG[typeKey as OrderTypeSuffix] || {
    label: type,
    color: 'text-gray-400',
    bgColor: 'bg-gray-500/20',
    description: '',
  };

  const Icon = getNodeIcon(type);

  // Determine status - prefer lifecycleStatus, then effectiveLifecycleStatus, then order.status
  let status = 'NEW';
  let statusIndicator = getStatusIndicator('NEW');

  if (type === 'POSITION' && positionState) {
    status = positionState.status;
    statusIndicator = getStatusIndicator(status);
  } else if (effectiveLifecycleStatus) {
    // Story 14.19: Use lifecycle coordinator status or derived status from order
    status = effectiveLifecycleStatus;
    statusIndicator = getStatusIndicator(status);
  } else if (order) {
    status = order.status;
    statusIndicator = getStatusIndicator(status);
  }

  // Determine price to display
  // For filled SL/TP orders, show actual fill price (avgPrice) instead of trigger price
  let displayPrice = 0;
  let priceLabel = 'Price';

  if (type === 'POSITION' && positionState) {
    displayPrice = positionState.entryPrice;
    priceLabel = 'Entry';
  } else if (order) {
    const isFilled = status === 'FILLED' || effectiveLifecycleStatus === 'FILLED';
    const hasAvgPrice = (order.avgPrice ?? 0) > 0;
    if (isFilled && hasAvgPrice) {
      // Show actual fill price for filled orders
      displayPrice = order.avgPrice!;
      priceLabel = 'Fill';
    } else {
      displayPrice = order.stopPrice && order.stopPrice > 0 ? order.stopPrice : order.price;
      priceLabel = order.stopPrice && order.stopPrice > 0 ? 'Stop' : 'Price';
    }
  }

  // Check if this order type can have modifications
  // Note: TP4 is included for future compatibility with ModifiableOrderType
  const isModifiable = ['SL', 'TP', 'TP1', 'TP2', 'TP3'].includes(type);

  // Determine if this node has expandable content (legacy modifications OR SL modification history)
  const hasExpandableContent = (isModifiable && modificationCount > 0) || hasSLModifications;

  // Handle expansion and lazy load modifications
  const handleToggleExpand = useCallback(async () => {
    if (isModifiable && modificationCount > 0 && localModifications.length === 0 && onLoadModifications && !hasSLModifications) {
      setLoadingMods(true);
      try {
        const mods = await onLoadModifications(type as ModifiableOrderType);
        setLocalModifications(mods);
      } catch (err) {
        console.error('Failed to load modifications:', err);
      } finally {
        setLoadingMods(false);
      }
    }
    setExpanded(prev => !prev);
  }, [isModifiable, modificationCount, localModifications.length, onLoadModifications, type, hasSLModifications]);

  // Tree connector characters
  const getConnector = () => {
    if (depth === 0) return '';
    return isLast ? '\u2514\u2500\u2500 ' : '\u251C\u2500\u2500 '; // └── or ├──
  };

  return (
    <div className="tree-node">
      {/* Main node row */}
      <div className="flex items-start">
        {/* Tree connector */}
        {depth > 0 && (
          <span className="font-mono text-gray-600 select-none whitespace-pre" style={{ minWidth: `${depth * 24}px` }}>
            {getConnector()}
          </span>
        )}

        {/* Node content */}
        <div
          className={`flex-1 flex items-center gap-2 px-3 py-2 rounded-lg ${config.bgColor} border ${
            hasExpandableContent ? 'cursor-pointer hover:opacity-80' : ''
          } transition-opacity`}
          onClick={hasExpandableContent ? handleToggleExpand : undefined}
          onKeyDown={hasExpandableContent ? (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              handleToggleExpand();
            }
          } : undefined}
          role={hasExpandableContent ? 'button' : undefined}
          tabIndex={hasExpandableContent ? 0 : undefined}
          aria-expanded={hasExpandableContent ? expanded : undefined}
          aria-label={hasExpandableContent ? `${config.label} order with ${hasSLModifications ? (slModifications?.length || 0) - 1 : modificationCount} modifications. ${expanded ? 'Click to collapse' : 'Click to expand'}` : undefined}
        >
          {/* Expand/collapse icon for modifiable orders */}
          {hasExpandableContent && (
            <div className="flex-shrink-0">
              {expanded ? (
                <ChevronDown className="w-4 h-4 text-gray-400" />
              ) : (
                <ChevronRight className="w-4 h-4 text-gray-400" />
              )}
            </div>
          )}

          {/* POSITION type: Full details with entry price, current price, P&L */}
          {type === 'POSITION' ? (
            (() => {
              const isClosed = positionState?.status === 'CLOSED';
              return (
                <>
                  {/* Position icon and label */}
                  <Icon className={`w-4 h-4 flex-shrink-0 ${config.color}`} />
                  <span className={`font-medium ${config.color}`}>{config.label}</span>

                  {/* Side badge - use positionSide prop as authoritative source */}
                  <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
                    positionSide === 'LONG'
                      ? 'bg-green-500/20 text-green-400'
                      : 'bg-red-500/20 text-red-400'
                  }`}>
                    {positionSide === 'LONG'
                      ? <TrendingUp className="w-3 h-3" />
                      : <TrendingDown className="w-3 h-3" />
                    }
                    {positionSide}
                  </span>

                  {/* Stage badge (from position analytics) - only for open positions */}
                  {!isClosed && positionAnalytics && (
                    <span className={`flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${getStageConfig(positionAnalytics.stage).bgColor} ${getStageConfig(positionAnalytics.stage).color}`}>
                      {getStageConfig(positionAnalytics.stage).label}
                    </span>
                  )}

                  {/* Status indicator */}
                  <span className={`flex items-center gap-1 text-xs ${statusIndicator.color}`}>
                    <statusIndicator.icon className="w-3.5 h-3.5" />
                    {statusIndicator.label}
                  </span>

                  {/* Duration for closed positions */}
                  {isClosed && positionState.closedAt && (
                    <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-xs bg-gray-500/20 text-gray-400" title="Total position duration">
                      <Timer className="w-3 h-3" />
                      {formatStaticDuration(
                        new Date(positionState.entryFilledAt).getTime(),
                        new Date(positionState.closedAt).getTime()
                      )}
                    </span>
                  )}

                  {/* Spacer */}
                  <div className="flex-1" />

                  {/* Entry Price */}
                  {positionState && positionState.entryPrice > 0 && (
                    <div className="text-right">
                      <span className="text-gray-200 font-mono text-sm">${formatPrice(positionState.entryPrice)}</span>
                      <span className="text-xs text-gray-500 ml-1">Entry</span>
                    </div>
                  )}

                  {/* For CLOSED positions: show close price and realized PnL */}
                  {isClosed ? (
                    <>
                      {/* Close Price */}
                      {positionState.closePrice && positionState.closePrice > 0 && (
                        <div className="text-right ml-3">
                          <span className="text-orange-400 font-mono text-sm">${formatPrice(positionState.closePrice)}</span>
                          <span className="text-xs text-gray-500 ml-1">Close</span>
                        </div>
                      )}

                      {/* Quantity */}
                      {(positionState.entryQuantity ?? 0) > 0 && (
                        <div className="text-right ml-3">
                          <span className="text-gray-300 font-mono text-sm">{(positionState.entryQuantity ?? 0).toFixed(4)}</span>
                          <span className="text-xs text-gray-500 ml-1">Qty</span>
                        </div>
                      )}

                      {/* Realized PnL */}
                      {positionState.realizedPnl != null && (
                        <div className="text-right ml-3">
                          <span className={`font-mono text-sm font-medium ${(positionState.realizedPnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {(positionState.realizedPnl ?? 0) >= 0 ? '+' : ''}${(positionState.realizedPnl ?? 0).toFixed(2)}
                          </span>
                          {(() => {
                            const entryVal = (positionState.entryPrice || 0) * (positionState.entryQuantity || 0);
                            const pnlPct = entryVal > 0 ? ((positionState.realizedPnl ?? 0) / entryVal) * 100 : 0;
                            return entryVal > 0 ? (
                              <span className={`text-xs ml-1 ${(positionState.realizedPnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                ({pnlPct >= 0 ? '+' : ''}{pnlPct.toFixed(2)}%)
                              </span>
                            ) : null;
                          })()}
                          <span className="text-xs text-gray-500 ml-1">PnL</span>
                        </div>
                      )}
                    </>
                  ) : (
                    <>
                      {/* For OPEN positions: show live price, quantity, value, unrealized PnL */}
                      {/* Current Price (live price preferred, fallback to analytics) */}
                      {(() => {
                        const curPrice = livePrice || positionAnalytics?.current_price || 0;
                        return curPrice > 0 ? (
                          <div className="text-right ml-3">
                            <span className="text-blue-400 font-mono text-sm">${formatPrice(curPrice)}</span>
                            <span className="text-xs text-gray-500 ml-1">{livePrice ? 'Live' : 'Mark'}</span>
                          </div>
                        ) : null;
                      })()}

                      {/* Quantity */}
                      {positionState && positionState.entryQuantity > 0 && (
                        <div className="text-right ml-3">
                          <span className="text-gray-300 font-mono text-sm">{positionState.entryQuantity.toFixed(4)}</span>
                          <span className="text-xs text-gray-500 ml-1">Qty</span>
                        </div>
                      )}

                      {/* Notional Value (recalculated with live price if available) */}
                      {positionState && positionState.entryValue > 0 && (
                        <div className="text-right ml-3">
                          <span className="text-gray-300 font-mono text-sm">
                            ${livePrice && positionState.entryQuantity > 0
                              ? (livePrice * positionState.entryQuantity).toFixed(2)
                              : positionState.entryValue.toFixed(2)
                            }
                          </span>
                          <span className="text-xs text-gray-500 ml-1">Value</span>
                        </div>
                      )}

                      {/* Unrealized P&L (recalculated with live price if available) */}
                      {(() => {
                        const curPrice = livePrice || positionAnalytics?.current_price || 0;
                        const ePrice = positionState?.entryPrice || 0;
                        const eQty = positionState?.entryQuantity || 0;
                        const qty = positionState?.remainingQuantity || eQty;
                        const isLongPos = positionSide === 'LONG';
                        let pnl: number;
                        if (livePrice && ePrice > 0 && qty > 0) {
                          pnl = isLongPos ? (curPrice - ePrice) * qty : (ePrice - curPrice) * qty;
                        } else {
                          pnl = positionAnalytics?.unrealized_pnl ?? 0;
                        }
                        const eValue = ePrice * eQty;
                        const pnlPct = eValue > 0 ? (pnl / eValue) * 100 : 0;
                        const hasPnl = curPrice > 0 && ePrice > 0;
                        return hasPnl ? (
                          <div className="text-right ml-3">
                            <span className={`font-mono text-sm ${pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {pnl >= 0 ? '+' : ''}${pnl.toFixed(2)}
                            </span>
                            {eValue > 0 && (
                              <span className={`text-xs ml-1 ${pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                ({pnlPct >= 0 ? '+' : ''}{pnlPct.toFixed(2)}%)
                              </span>
                            )}
                            <span className="text-xs text-gray-500 ml-1">PnL</span>
                          </div>
                        ) : null;
                      })()}
                    </>
                  )}
                </>
              );
            })()
          ) : (
            <>
              {/* Non-POSITION types: Full detail rendering */}
              {/* Order type icon and label */}
              <Icon className={`w-4 h-4 flex-shrink-0 ${config.color}`} />
              <span className={`font-medium ${config.color}`}>{config.label}</span>

              {/* Buy/Sell side badge for entry orders - use positionSide prop as authoritative source */}
              {type === 'ENTRY' && (
                <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
                  positionSide === 'LONG'
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-red-500/20 text-red-400'
                }`}>
                  {positionSide === 'LONG'
                    ? <TrendingUp className="w-3 h-3" />
                    : <TrendingDown className="w-3 h-3" />
                  }
                  {positionSide}
                </span>
              )}

              {/* Modification count badge */}
              {isModifiable && modificationCount > 0 && (
                <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                  <Edit3 className="w-3 h-3" />
                  {modificationCount}
                </span>
              )}

              {/* Status indicator */}
              <span className={`flex items-center gap-1 text-xs ${statusIndicator.color}`}>
                <statusIndicator.icon className="w-3.5 h-3.5" />
                {statusIndicator.label}
              </span>

              {/* Exit badge for the order that closed the position */}
              {isExitOrder && order?.status === 'FILLED' && (
                <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-xs bg-orange-500/20 text-orange-400 font-medium">
                  ← EXIT
                </span>
              )}

              {/* Cancel reason for SL/TP that was cancelled when opposite hit */}
              {cancelReason && order?.status === 'CANCELED' && (
                <span className="text-xs text-gray-500 italic">
                  ({cancelReason})
                </span>
              )}

              {/* Countdown timer for pending entry orders */}
              {countdown && type === 'ENTRY' && (
                <span
                  className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
                    parseInt(countdown.split(':')[0]) < 1
                      ? 'bg-red-500/30 text-red-400 animate-pulse'
                      : 'bg-orange-500/20 text-orange-400'
                  }`}
                  title="Time remaining before order timeout"
                >
                  <Clock className="w-3 h-3" />
                  {countdown}
                </span>
              )}

              {/* Duration counter for pending non-entry orders, or frozen duration for closed orders */}
              {duration && type !== 'ENTRY' && (
                <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
                  effectiveLifecycleStatus ? 'bg-gray-500/20 text-gray-400' : 'bg-amber-500/20 text-amber-400'
                }`} title={effectiveLifecycleStatus ? 'Total order duration' : 'Time since order placed'}>
                  <Timer className="w-3 h-3" />
                  {duration}
                </span>
              )}

              {/* Spacer */}
              <div className="flex-1" />

              {/* Price display */}
              {displayPrice > 0 && (
                <div className="text-right">
                  <span className="text-gray-200 font-mono text-sm">${formatPrice(displayPrice)}</span>
                  <span className="text-xs text-gray-500 ml-1">{priceLabel}</span>
                </div>
              )}

              {/* Order-specific: quantity and value */}
              {order && (
                <>
                  <div className="text-right ml-3">
                    <span className="text-gray-300 font-mono text-sm">
                      {(order.executedQty ?? 0) > 0 ? `${(order.executedQty ?? 0).toFixed(4)}/` : ''}
                      {(order.origQty ?? 0).toFixed(4)}
                    </span>
                    <span className="text-xs text-gray-500 ml-1">Qty</span>
                  </div>
                  {/* Order value - use stopPrice for STOP orders, avgPrice for filled, or price as fallback */}
                  <div className="text-right ml-3">
                    <span className="text-gray-300 font-mono text-sm">
                      {formatValue(
                        order.avgPrice && order.avgPrice > 0
                          ? order.avgPrice
                          : (order.stopPrice && order.stopPrice > 0 ? order.stopPrice : order.price),
                        order.origQty
                      )}
                    </span>
                    <span className="text-xs text-gray-500 ml-1">Value</span>
                  </div>
                  {/* Filled value when partially/fully filled */}
                  {order.executedQty > 0 && order.avgPrice && order.avgPrice > 0 && (
                    <div className="text-right ml-3">
                      <span className="text-green-400 font-mono text-sm">
                        {formatValue(order.avgPrice, order.executedQty)}
                      </span>
                      <span className="text-xs text-gray-500 ml-1">Filled</span>
                    </div>
                  )}
                </>
              )}

              {/* Expected P&L for SL/TP orders */}
              {['SL', 'TP', 'TP1', 'TP2', 'TP3'].includes(type) && entryPrice && entryPrice > 0 && displayPrice > 0 && (
                (() => {
                  const qty = entryQuantity || order?.origQty || 0;
                  const expectedPnL = calculateExpectedPnL(type, displayPrice, entryPrice, qty, positionSide, feeRate);
                  if (!expectedPnL) return null;

                  const isSL = type === 'SL';
                  const label = isSL ? 'Exp. Loss' : 'Exp. Profit';
                  const colorClass = expectedPnL.pnl >= 0 ? 'text-green-400' : 'text-red-400';

                  return (
                    <div className="text-right ml-3">
                      <span className={`font-mono text-sm font-medium ${colorClass}`}>
                        {expectedPnL.pnl >= 0 ? '+' : ''}${expectedPnL.pnl.toFixed(2)}
                      </span>
                      <span className={`text-xs ml-1 ${colorClass}`}>
                        ({expectedPnL.pnlPercent >= 0 ? '+' : ''}{expectedPnL.pnlPercent.toFixed(2)}%)
                      </span>
                      <span className="text-xs text-gray-500 ml-1">{label}</span>
                    </div>
                  );
                })()
              )}

              {/* Timestamp - Story 7.19: Using timezone-aware formatter */}
              {order ? (
                <div className="text-xs text-gray-500 ml-3">
                  {formatTime(order.time)}
                </div>
              ) : null}
            </>
          )}
        </div>
      </div>

      {/* Expanded SL modification history - Tree structure from chain events */}
      {expanded && hasSLModifications && slModifications && (
        <div className="mt-1 space-y-1">
          {slModifications
            .sort((a, b) => a.sequence - b.sequence)
            .map((mod, idx) => (
              <SLModificationTreeNode
                key={mod.sequence}
                modification={mod}
                index={idx}
                total={slModifications.length}
                depth={depth + 1}
                isLast={idx === slModifications.length - 1}
                isCurrent={idx === slModifications.length - 1}
                formatTime={formatTime}
                entryPrice={entryPrice}
                entryQuantity={entryQuantity}
                positionSide={positionSide}
                feeRate={feeRate}
              />
            ))
          }
        </div>
      )}

      {/* Expanded legacy modification history - Rendered as child tree nodes */}
      {expanded && isModifiable && !hasSLModifications && (
        <div className="mt-1 space-y-1">
          {loadingMods ? (
            <div className="text-sm text-gray-500 animate-pulse py-2" style={{ marginLeft: `${(depth + 1) * 24}px` }}>
              Loading modification history...
            </div>
          ) : localModifications.length > 0 ? (
            <>
              {/* Render each modification as a child tree node */}
              {localModifications
                .sort((a, b) => a.version - b.version)
                .map((mod, idx) => (
                  <ModificationTreeNode
                    key={mod.id || idx}
                    modification={mod}
                    parentType={type}
                    depth={depth + 1}
                    isLast={idx === localModifications.length - 1}
                    formatTime={formatTime}
                  />
                ))
              }
            </>
          ) : (
            <div className="text-sm text-gray-500 py-2" style={{ marginLeft: `${(depth + 1) * 24}px` }}>
              No modification history available.
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Sub-component for rendering a single modification as a tree node
interface ModificationTreeNodeProps {
  modification: ModificationEvent;
  parentType: TreeNodeType;
  depth: number;
  isLast: boolean;
  formatTime?: (timestamp: string | number) => string;
}

function ModificationTreeNode({
  modification,
  parentType,
  depth,
  isLast,
  formatTime = defaultFormatTime,
}: ModificationTreeNodeProps) {
  // Tree connector characters
  const getConnector = () => {
    if (depth === 0) return '';
    return isLast ? '\u2514\u2500\u2500 ' : '\u251C\u2500\u2500 '; // └── or ├──
  };

  // Determine the display for this modification
  const isInitialPlacement = modification.eventType === 'PLACED' && modification.version === 1;
  const isPriceChange = modification.eventType === 'MODIFIED';

  // Get source display info
  const getSourceDisplay = () => {
    switch (modification.modificationSource) {
      case 'LLM_AUTO':
        return { label: 'AI', color: 'text-purple-400', bg: 'bg-purple-500/20', icon: '🤖' };
      case 'USER_MANUAL':
        return { label: 'Manual', color: 'text-blue-400', bg: 'bg-blue-500/20', icon: '👤' };
      case 'TRAILING_STOP':
        return { label: 'Trail', color: 'text-yellow-400', bg: 'bg-yellow-500/20', icon: '📈' };
      default:
        return { label: 'Unknown', color: 'text-gray-400', bg: 'bg-gray-500/20', icon: '•' };
    }
  };

  const sourceDisplay = getSourceDisplay();

  // Determine impact color
  const getImpactColor = () => {
    if (isInitialPlacement) return 'text-gray-400';
    if (modification.impactDirection === 'BETTER' || modification.impactDirection === 'TIGHTER') return 'text-green-400';
    if (modification.impactDirection === 'WORSE' || modification.impactDirection === 'WIDER') return 'text-red-400';
    return 'text-gray-400';
  };

  const bgColor = isInitialPlacement
    ? 'bg-gray-800/30'
    : modification.impactDirection === 'BETTER' || modification.impactDirection === 'TIGHTER'
    ? 'bg-green-900/20'
    : modification.impactDirection === 'WORSE' || modification.impactDirection === 'WIDER'
    ? 'bg-red-900/20'
    : 'bg-gray-800/30';

  return (
    <div className="flex items-start">
      {/* Tree connector */}
      <span className="font-mono text-gray-600 select-none whitespace-pre" style={{ minWidth: `${depth * 24}px` }}>
        {getConnector()}
      </span>

      {/* Modification content */}
      <div className={`flex-1 flex items-center gap-2 px-2 py-1.5 rounded-lg ${bgColor} border border-gray-700/30 text-xs`}>
        {/* Version badge */}
        <span className="flex items-center justify-center w-5 h-5 rounded-full bg-gray-700 text-gray-300 font-mono text-[10px]">
          {modification.version}
        </span>

        {/* Event type */}
        {isInitialPlacement ? (
          <span className="text-gray-400">Initial</span>
        ) : (
          <span className={getImpactColor()}>
            {modification.impactDirection === 'TIGHTER' ? '↑ Tightened' :
             modification.impactDirection === 'WIDER' ? '↓ Widened' :
             modification.impactDirection === 'BETTER' ? '↑ Improved' :
             modification.impactDirection === 'WORSE' ? '↓ Reduced' : 'Modified'}
          </span>
        )}

        {/* Source badge */}
        <span className={`flex items-center gap-0.5 px-1.5 py-0.5 rounded ${sourceDisplay.bg} ${sourceDisplay.color}`}>
          <span>{sourceDisplay.icon}</span>
          <span>{sourceDisplay.label}</span>
        </span>

        {/* Price change */}
        {isPriceChange && modification.oldPrice !== null && (
          <span className="text-gray-400 font-mono">
            ${(modification.oldPrice ?? 0).toFixed(2)} → ${(modification.newPrice ?? 0).toFixed(2)}
          </span>
        )}
        {isInitialPlacement && (
          <span className="text-gray-400 font-mono">
            ${(modification.newPrice ?? 0).toFixed(2)}
          </span>
        )}

        {/* Dollar impact */}
        {modification.dollarImpact !== 0 && !isInitialPlacement && (
          <span className={`font-medium ${(modification.dollarImpact ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {(modification.dollarImpact ?? 0) >= 0 ? '+' : ''}{formatDollarImpact(modification.dollarImpact ?? 0)}
          </span>
        )}

        {/* Spacer */}
        <div className="flex-1" />

        {/* Reason (truncated) */}
        {modification.modificationReason && !isInitialPlacement && (
          <span className="text-gray-500 truncate max-w-[200px]" title={modification.modificationReason}>
            {modification.modificationReason}
          </span>
        )}

        {/* Timestamp */}
        <span className="text-gray-500">
          {formatTime(modification.createdAt)}
        </span>
      </div>
    </div>
  );
}

// Sub-component for rendering a single SL modification from chain events
interface SLModificationTreeNodeProps {
  modification: SLModification;
  index: number;
  total: number;
  depth: number;
  isLast: boolean;
  isCurrent: boolean;
  formatTime?: (timestamp: string | number) => string;
  entryPrice?: number;
  entryQuantity?: number;
  positionSide: 'LONG' | 'SHORT';
  feeRate?: number;
}

function SLModificationTreeNode({
  modification,
  index,
  total,
  depth,
  isLast,
  isCurrent,
  formatTime = defaultFormatTime,
  entryPrice,
  entryQuantity,
  positionSide,
  feeRate = 0.0005,
}: SLModificationTreeNodeProps) {
  const getConnector = () => {
    if (depth === 0) return '';
    return isLast ? '\u2514\u2500\u2500 ' : '\u251C\u2500\u2500 '; // -- or ---
  };

  const isInitial = index === 0;
  const isModification = !isInitial && modification.oldPrice != null;

  // Determine if this is a favorable SL move (closer to entry = locking profit)
  // For LONG: SL moving up = good (locking profit), SL moving down = bad (more risk)
  // For SHORT: SL moving down = good (locking profit), SL moving up = bad (more risk)
  let isFavorable = false;
  if (isModification && modification.oldPrice != null) {
    const priceDiff = modification.newPrice - modification.oldPrice;
    isFavorable = positionSide === 'LONG' ? priceDiff > 0 : priceDiff < 0;
  }

  // Calculate P&L impact if SL hit at this level
  let pnlAtLevel: { pnl: number; pnlPercent: number } | null = null;
  if (entryPrice && entryPrice > 0 && entryQuantity && entryQuantity > 0) {
    let grossPnL: number;
    if (positionSide === 'LONG') {
      grossPnL = (modification.newPrice - entryPrice) * entryQuantity;
    } else {
      grossPnL = (entryPrice - modification.newPrice) * entryQuantity;
    }
    const entryValue = entryPrice * entryQuantity;
    const exitValue = modification.newPrice * entryQuantity;
    const totalFees = (entryValue + exitValue) * feeRate;
    const netPnL = grossPnL - totalFees;
    const pnlPercent = entryValue > 0 ? (netPnL / entryValue) * 100 : 0;
    pnlAtLevel = { pnl: netPnL, pnlPercent };
  }

  // Get source display
  const getSourceDisplay = () => {
    switch (modification.source) {
      case 'TRAILING_STOP':
        return { label: 'Ravindra', color: 'text-yellow-400', bg: 'bg-yellow-500/20' };
      case 'LLM_AUTO':
        return { label: 'AI', color: 'text-purple-400', bg: 'bg-purple-500/20' };
      case 'USER_MANUAL':
        return { label: 'Manual', color: 'text-blue-400', bg: 'bg-blue-500/20' };
      default:
        return { label: 'System', color: 'text-gray-400', bg: 'bg-gray-500/20' };
    }
  };

  const sourceDisplay = getSourceDisplay();

  // Background color based on favorability
  const bgColor = isInitial
    ? 'bg-gray-800/30'
    : isFavorable
    ? 'bg-green-900/20'
    : 'bg-red-900/20';

  // Border color for current active SL
  const borderClass = isCurrent
    ? 'border-yellow-500/50'
    : 'border-gray-700/30';

  // Format short reason for display
  const shortReason = modification.reason
    ? modification.reason
        .replace('Move to breakeven at ', 'BE @ ')
        .replace('Move to ', '')
        .replace(' level at ', ' @ ')
    : '';

  return (
    <div className="flex items-start">
      {/* Tree connector */}
      <span className="font-mono text-gray-600 select-none whitespace-pre" style={{ minWidth: `${depth * 24}px` }}>
        {getConnector()}
      </span>

      {/* Modification content */}
      <div className={`flex-1 flex items-center gap-2 px-2 py-1.5 rounded-lg ${bgColor} border ${borderClass} text-xs`}>
        {/* Version/sequence indicator */}
        <span className={`flex items-center justify-center w-5 h-5 rounded-full font-mono text-[10px] ${
          isInitial ? 'bg-gray-700 text-gray-300' : isFavorable ? 'bg-green-700 text-green-300' : 'bg-red-700 text-red-300'
        }`}>
          {isInitial ? '#' : index}
        </span>

        {/* Event label */}
        {isInitial ? (
          <span className="text-gray-400">Original</span>
        ) : (
          <span className={isFavorable ? 'text-green-400' : 'text-red-400'}>
            {isFavorable ? 'Tightened' : 'Widened'}
          </span>
        )}

        {/* Source badge */}
        {!isInitial && (
          <span className={`flex items-center gap-0.5 px-1.5 py-0.5 rounded ${sourceDisplay.bg} ${sourceDisplay.color}`}>
            <span>{sourceDisplay.label}</span>
          </span>
        )}

        {/* Price change display */}
        {isModification && modification.oldPrice != null ? (
          <span className="text-gray-400 font-mono">
            ${modification.oldPrice.toFixed(4)} <span className="text-gray-600">-&gt;</span> <span className={isFavorable ? 'text-green-400' : 'text-red-400'}>${modification.newPrice.toFixed(4)}</span>
          </span>
        ) : (
          <span className="text-gray-400 font-mono">
            ${modification.newPrice.toFixed(4)}
          </span>
        )}

        {/* P&L if SL hit at this level */}
        {pnlAtLevel && (
          <span className={`font-medium font-mono ${pnlAtLevel.pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {pnlAtLevel.pnl >= 0 ? '+' : ''}${pnlAtLevel.pnl.toFixed(2)}
            <span className="text-[10px] ml-0.5">({pnlAtLevel.pnlPercent >= 0 ? '+' : ''}{pnlAtLevel.pnlPercent.toFixed(1)}%)</span>
          </span>
        )}

        {/* Current indicator */}
        {isCurrent && !isInitial && (
          <span className="flex items-center gap-0.5 px-1.5 py-0.5 rounded bg-yellow-500/20 text-yellow-400 font-medium">
            Current
          </span>
        )}

        {/* Spacer */}
        <div className="flex-1" />

        {/* Reason */}
        {shortReason && !isInitial && (
          <span className="text-gray-500 truncate max-w-[180px]" title={modification.reason}>
            {shortReason}
          </span>
        )}

        {/* Timestamp */}
        <span className="text-gray-500">
          {formatTime(modification.timestamp)}
        </span>
      </div>
    </div>
  );
}

// Helper component to build entry order from position state
export function buildEntryFromPositionState(positionState: PositionState): ChainOrder {
  const parsed = {
    raw: positionState.entryClientOrderId,
    modeCode: null,
    dateStr: null,
    sequence: null,
    orderType: 'E' as OrderTypeSuffix,
    chainId: positionState.chainId,
    isFallback: false,
    isValid: true,
  };

  return {
    orderId: positionState.entryOrderId ?? 0,
    clientOrderId: positionState.entryClientOrderId ?? '',
    symbol: positionState.symbol ?? '',
    side: positionState.entrySide ?? 'BUY',
    positionSide: positionState.entrySide === 'BUY' ? 'LONG' : 'SHORT',
    type: 'LIMIT',
    status: 'FILLED',
    price: positionState.entryPrice ?? 0,
    avgPrice: positionState.entryPrice ?? 0,
    origQty: positionState.entryQuantity ?? 0,
    executedQty: positionState.entryQuantity ?? 0,
    stopPrice: 0,
    time: positionState.entryFilledAt ? new Date(positionState.entryFilledAt).getTime() : 0,
    updateTime: positionState.updatedAt ? new Date(positionState.updatedAt).getTime() : 0,
    orderType: 'E' as OrderTypeSuffix,
    parsed,
  };
}
