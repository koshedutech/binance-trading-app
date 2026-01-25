// Story 7.15: Order Tree Node Component
// Story 7.19: Enhanced with timezone support via formatTime prop
// Story 7.21: Enhanced with inline modification history using ModificationRowList
// Enhanced: Added duration counter, buy/sell side, order value display
// Individual node in the order chain tree structure
import React, { useState, useCallback, useEffect } from 'react';
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
  DollarSign,
  History,
} from 'lucide-react';
import { ChainOrder, PositionState, ORDER_TYPE_CONFIG, OrderTypeSuffix } from './types';
import { ModificationRowList, calculateSummaryStats, formatDollarImpact } from './ModificationHistory';
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

// Format order value (price * quantity) - null-safe
function formatValue(price: number | null | undefined, quantity: number | null | undefined): string {
  const value = (price ?? 0) * (quantity ?? 0);
  if (value >= 1000) return `$${value.toFixed(2)}`;
  if (value >= 1) return `$${value.toFixed(4)}`;
  return `$${value.toFixed(6)}`;
}

// Tree node types
export type TreeNodeType = 'ENTRY' | 'POSITION' | 'TP1' | 'TP2' | 'TP3' | 'SL' | 'DCA1' | 'DCA2' | 'DCA3' | 'H' | 'HSL' | 'HTP' | 'RB';

interface OrderTreeNodeProps {
  type: TreeNodeType;
  order?: ChainOrder;
  positionState?: PositionState;
  modificationCount?: number;
  modifications?: ModificationEvent[];
  chainId: string;
  positionSide: 'LONG' | 'SHORT';
  isLast?: boolean;
  depth: number;
  onLoadModifications?: (orderType: ModifiableOrderType) => Promise<ModificationEvent[]>;
  // Story 7.19: Timezone-aware time formatter
  formatTime?: (timestamp: string | number) => string;
}

// Get icon for node type
function getNodeIcon(type: TreeNodeType) {
  switch (type) {
    case 'ENTRY':
      return TrendingUp;
    case 'POSITION':
      return Activity;
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

// Format price based on magnitude (null-safe)
function formatPrice(price: number | null | undefined): string {
  const safePrice = price ?? 0;
  if (safePrice >= 1000) return safePrice.toFixed(2);
  if (safePrice >= 1) return safePrice.toFixed(4);
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

export default function OrderTreeNode({
  type,
  order,
  positionState,
  modificationCount = 0,
  modifications,
  chainId,
  positionSide,
  isLast = false,
  depth,
  onLoadModifications,
  formatTime = defaultFormatTime, // Story 7.19: Use provided formatter or default
}: OrderTreeNodeProps) {
  const [expanded, setExpanded] = useState(false);
  const [localModifications, setLocalModifications] = useState<ModificationEvent[]>(modifications || []);
  const [loadingMods, setLoadingMods] = useState(false);
  const [duration, setDuration] = useState<string>('');
  const [countdown, setCountdown] = useState<string>('');

  // Update duration counter every 10 seconds for pending orders (non-entry)
  // Update countdown every second for pending entry orders
  useEffect(() => {
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
  }, [order, type]);

  // Get config for this order type
  const typeKey = type === 'ENTRY' ? 'E' : type;
  const config = ORDER_TYPE_CONFIG[typeKey as OrderTypeSuffix] || {
    label: type,
    color: 'text-gray-400',
    bgColor: 'bg-gray-500/20',
    description: '',
  };

  const Icon = getNodeIcon(type);

  // Determine status
  let status = 'NEW';
  let statusIndicator = getStatusIndicator('NEW');

  if (type === 'POSITION' && positionState) {
    status = positionState.status;
    statusIndicator = getStatusIndicator(status);
  } else if (order) {
    status = order.status;
    statusIndicator = getStatusIndicator(status);
  }

  // Determine price to display
  let displayPrice = 0;
  let priceLabel = 'Price';

  if (type === 'POSITION' && positionState) {
    displayPrice = positionState.entryPrice;
    priceLabel = 'Entry';
  } else if (order) {
    displayPrice = order.stopPrice && order.stopPrice > 0 ? order.stopPrice : order.price;
    priceLabel = order.stopPrice && order.stopPrice > 0 ? 'Stop' : 'Price';
  }

  // Check if this order type can have modifications
  // Note: TP4 is included for future compatibility with ModifiableOrderType
  const isModifiable = ['SL', 'TP1', 'TP2', 'TP3'].includes(type);

  // Handle expansion and lazy load modifications
  const handleToggleExpand = useCallback(async () => {
    if (isModifiable && modificationCount > 0 && localModifications.length === 0 && onLoadModifications) {
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
  }, [isModifiable, modificationCount, localModifications.length, onLoadModifications, type]);

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
            isModifiable && modificationCount > 0 ? 'cursor-pointer hover:opacity-80' : ''
          } transition-opacity`}
          onClick={isModifiable && modificationCount > 0 ? handleToggleExpand : undefined}
          onKeyDown={isModifiable && modificationCount > 0 ? (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              handleToggleExpand();
            }
          } : undefined}
          role={isModifiable && modificationCount > 0 ? 'button' : undefined}
          tabIndex={isModifiable && modificationCount > 0 ? 0 : undefined}
          aria-expanded={isModifiable && modificationCount > 0 ? expanded : undefined}
          aria-label={isModifiable && modificationCount > 0 ? `${config.label} order with ${modificationCount} modifications. ${expanded ? 'Click to collapse' : 'Click to expand'}` : undefined}
        >
          {/* Expand/collapse icon for modifiable orders */}
          {isModifiable && modificationCount > 0 && (
            <div className="flex-shrink-0">
              {expanded ? (
                <ChevronDown className="w-4 h-4 text-gray-400" />
              ) : (
                <ChevronRight className="w-4 h-4 text-gray-400" />
              )}
            </div>
          )}

          {/* Order type icon and label */}
          <Icon className={`w-4 h-4 flex-shrink-0 ${config.color}`} />
          <span className={`font-medium ${config.color}`}>{config.label}</span>

          {/* Buy/Sell side badge for entry orders and positions */}
          {(type === 'ENTRY' || type === 'POSITION') && (order || positionState) && (
            <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
              (order?.side || positionState?.entrySide) === 'BUY'
                ? 'bg-green-500/20 text-green-400'
                : 'bg-red-500/20 text-red-400'
            }`}>
              {(order?.side || positionState?.entrySide) === 'BUY'
                ? <TrendingUp className="w-3 h-3" />
                : <TrendingDown className="w-3 h-3" />
              }
              {(order?.side || positionState?.entrySide) === 'BUY' ? 'LONG' : 'SHORT'}
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

          {/* Duration counter for pending non-entry orders */}
          {duration && type !== 'ENTRY' && (
            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-xs bg-amber-500/20 text-amber-400" title="Time since order placed">
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

          {/* Position-specific: quantity, value, and P&L */}
          {type === 'POSITION' && positionState && (
            <>
              <div className="text-right ml-3">
                <span className="text-gray-300 font-mono text-sm">{(positionState.entryQuantity ?? 0).toFixed(4)}</span>
                <span className="text-xs text-gray-500 ml-1">Qty</span>
              </div>
              {/* Position value */}
              <div className="text-right ml-3">
                <span className="text-gray-300 font-mono text-sm">
                  {formatValue(positionState.entryPrice ?? 0, positionState.entryQuantity ?? 0)}
                </span>
                <span className="text-xs text-gray-500 ml-1">Value</span>
              </div>
              {(positionState.realizedPnl ?? 0) !== 0 && (
                <div className="text-right ml-3">
                  <span
                    className={`font-mono text-sm ${
                      (positionState.realizedPnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}
                  >
                    {(positionState.realizedPnl ?? 0) >= 0 ? '+' : ''}${(positionState.realizedPnl ?? 0).toFixed(2)}
                  </span>
                  <span className="text-xs text-gray-500 ml-1">P&L</span>
                </div>
              )}
            </>
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

          {/* Timestamp - Story 7.19: Using timezone-aware formatter */}
          {type === 'POSITION' && positionState ? (
            <div className="text-xs text-gray-500 ml-3">
              {formatTime(positionState.entryFilledAt)}
            </div>
          ) : order ? (
            <div className="text-xs text-gray-500 ml-3">
              {formatTime(order.time)}
            </div>
          ) : null}
        </div>
      </div>

      {/* Expanded modification history - Story 7.21: Inline row display */}
      {expanded && isModifiable && (
        <div className="mt-2" style={{ marginLeft: `${(depth + 1) * 24}px` }}>
          {loadingMods ? (
            <div className="text-sm text-gray-500 animate-pulse py-2">
              Loading modification history...
            </div>
          ) : localModifications.length > 0 ? (
            <div className="bg-gray-900/50 rounded-lg p-3 border border-gray-700/50">
              {/* Header with summary stats */}
              {(() => {
                const summary = calculateSummaryStats(localModifications);
                return (
                  <div className="flex items-center gap-3 mb-2 pb-2 border-b border-gray-700/50 text-xs">
                    <span className="flex items-center gap-1 text-gray-400">
                      <History className="w-3.5 h-3.5 text-purple-400" />
                      {summary.totalModifications} modification{summary.totalModifications !== 1 ? 's' : ''}
                    </span>
                    <span className="text-gray-600">|</span>
                    <span className="text-gray-400">
                      Initial: <span className="font-mono text-gray-300">${(summary.initialPrice ?? 0).toFixed(2)}</span>
                    </span>
                    <span className="text-gray-600">|</span>
                    <span className="text-gray-400">
                      Current: <span className="font-mono text-green-400">${(displayPrice ?? 0).toFixed(2)}</span>
                    </span>
                    <span className="text-gray-600">|</span>
                    <span className={`font-medium ${(summary.netDollarImpact ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      Net: {formatDollarImpact(summary.netDollarImpact ?? 0)}
                    </span>
                    {/* Source breakdown */}
                    <span className="ml-auto flex items-center gap-2 text-gray-500">
                      {summary.sources.llmAuto > 0 && (
                        <span className="text-purple-400">AI: {summary.sources.llmAuto}</span>
                      )}
                      {summary.sources.userManual > 0 && (
                        <span className="text-blue-400">Manual: {summary.sources.userManual}</span>
                      )}
                      {summary.sources.trailingStop > 0 && (
                        <span className="text-yellow-400">Trail: {summary.sources.trailingStop}</span>
                      )}
                    </span>
                  </div>
                );
              })()}
              {/* Modification row list */}
              <ModificationRowList
                events={localModifications}
                orderType={type as ModifiableOrderType}
                formatTime={formatTime}
              />
            </div>
          ) : (
            <div className="text-sm text-gray-500 py-2">
              No modification history available.
            </div>
          )}
        </div>
      )}
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
    orderId: positionState.entryOrderId,
    clientOrderId: positionState.entryClientOrderId,
    symbol: positionState.symbol,
    side: positionState.entrySide,
    positionSide: positionState.entrySide === 'BUY' ? 'LONG' : 'SHORT',
    type: 'MARKET',
    status: 'FILLED',
    price: positionState.entryPrice,
    avgPrice: positionState.entryPrice,
    origQty: positionState.entryQuantity,
    executedQty: positionState.entryQuantity,
    stopPrice: 0,
    time: new Date(positionState.entryFilledAt).getTime(),
    updateTime: new Date(positionState.updatedAt).getTime(),
    orderType: 'E' as OrderTypeSuffix,
    parsed,
  };
}
