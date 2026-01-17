// Story 7.15: Order Tree Node Component
// Individual node in the order chain tree structure
import React, { useState, useCallback } from 'react';
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
} from 'lucide-react';
import { format } from 'date-fns';
import { ChainOrder, PositionState, ORDER_TYPE_CONFIG, OrderTypeSuffix } from './types';
import { ModificationTree } from './ModificationHistory';
import type { ModificationEvent, ModifiableOrderType } from './ModificationHistory/types';

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

// Format price based on magnitude
function formatPrice(price: number): string {
  if (price >= 1000) return price.toFixed(2);
  if (price >= 1) return price.toFixed(4);
  return price.toFixed(8);
}

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
}: OrderTreeNodeProps) {
  const [expanded, setExpanded] = useState(false);
  const [localModifications, setLocalModifications] = useState<ModificationEvent[]>(modifications || []);
  const [loadingMods, setLoadingMods] = useState(false);

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

          {/* Spacer */}
          <div className="flex-1" />

          {/* Price display */}
          {displayPrice > 0 && (
            <div className="text-right">
              <span className="text-gray-200 font-mono text-sm">${formatPrice(displayPrice)}</span>
              <span className="text-xs text-gray-500 ml-1">{priceLabel}</span>
            </div>
          )}

          {/* Position-specific: quantity and P&L */}
          {type === 'POSITION' && positionState && (
            <>
              <div className="text-right ml-3">
                <span className="text-gray-300 font-mono text-sm">{positionState.entryQuantity.toFixed(4)}</span>
                <span className="text-xs text-gray-500 ml-1">Qty</span>
              </div>
              {positionState.realizedPnl !== 0 && (
                <div className="text-right ml-3">
                  <span
                    className={`font-mono text-sm ${
                      positionState.realizedPnl >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}
                  >
                    {positionState.realizedPnl >= 0 ? '+' : ''}${positionState.realizedPnl.toFixed(2)}
                  </span>
                  <span className="text-xs text-gray-500 ml-1">P&L</span>
                </div>
              )}
            </>
          )}

          {/* Order-specific: quantity */}
          {order && (
            <div className="text-right ml-3">
              <span className="text-gray-300 font-mono text-sm">
                {order.executedQty > 0 ? `${order.executedQty.toFixed(4)}/` : ''}
                {order.origQty.toFixed(4)}
              </span>
              <span className="text-xs text-gray-500 ml-1">Qty</span>
            </div>
          )}

          {/* Timestamp */}
          {type === 'POSITION' && positionState ? (
            <div className="text-xs text-gray-500 ml-3">
              {format(new Date(positionState.entryFilledAt), 'HH:mm:ss')}
            </div>
          ) : order ? (
            <div className="text-xs text-gray-500 ml-3">
              {format(order.time, 'HH:mm:ss')}
            </div>
          ) : null}
        </div>
      </div>

      {/* Expanded modification history */}
      {expanded && isModifiable && (
        <div className="mt-2" style={{ marginLeft: `${(depth + 1) * 24}px` }}>
          {loadingMods ? (
            <div className="text-sm text-gray-500 animate-pulse py-2">
              Loading modification history...
            </div>
          ) : localModifications.length > 0 ? (
            <ModificationTree
              chainId={chainId}
              orderType={type as ModifiableOrderType}
              currentPrice={displayPrice}
              events={localModifications}
              positionSide={positionSide}
              compact={true}
            />
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
