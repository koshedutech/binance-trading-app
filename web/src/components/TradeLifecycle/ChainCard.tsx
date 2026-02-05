// Story 7.15: Order Chain Tree Structure UI
// Hierarchical display: Entry -> Position -> [TP/SL]
import React, { useState, useEffect, useCallback, useMemo } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Activity,
  Target,
  Shield,
  TrendingUp,
  TrendingDown,
  Layers,
  Clock,
  AlertTriangle,
  History,
  GitBranch,
  Edit3,
  Check,
} from 'lucide-react';
import { OrderChain, ChainOrder, ORDER_TYPE_CONFIG, MODE_DISPLAY_NAMES, OrderTypeSuffix, PositionState, PositionAnalyticsData } from './types';
import {
  StageBadge,
  StageTimeline,
  PriceLevelsDisplay,
  EfficiencyMetricsDisplay,
  ClassicIndicatorDisplay,
  NewEngineIndicatorDisplay,
  DecisionModeBadge,
} from '../PositionCardExpanded';
import type { ExpandedPositionData } from '../../types/positionManagement';
import { useUserTimezone } from './hooks/useUserTimezone';
import OrderTreeNode, { buildEntryFromPositionState, TreeNodeType } from './OrderTreeNode';
import { ModificationTree, ModificationRowList, calculateSummaryStats, formatDollarImpact } from './ModificationHistory';
import type { ModificationEvent, ModifiableOrderType } from './ModificationHistory/types';
import { futuresApi } from '../../services/futuresApi';

interface ChainCardProps {
  chain: OrderChain;
  compact?: boolean;
  useTreeView?: boolean; // New prop to toggle tree view (default true)
  // User fee rates for economics calculations
  makerFeeRate?: number; // Default 0.02% = 0.0002
  takerFeeRate?: number; // Default 0.04% = 0.0004
  // Real-time price from coin profiler subscription
  livePrice?: number;
}

// Fee calculation helpers (similar to OrderEconomics.tsx)
interface TPSLEconomics {
  price: number;
  expectedPnL: number;
  expectedPnLPercent: number;
  entryFee: number;
  exitFee: number;
  grossPnL: number;
  status: string;
  orderId: number;
  orderType: string;
}

function calculateTPEconomics(
  tpOrder: ChainOrder,
  positionState: PositionState | undefined,
  entryOrder: ChainOrder | null,
  positionSide: 'LONG' | 'SHORT' | 'BOTH' | null,
  takerFeeRate: number
): TPSLEconomics | null {
  if (!positionState && !entryOrder) return null;

  const entryPrice = positionState?.entryPrice || entryOrder?.avgPrice || entryOrder?.price || 0;
  const quantity = positionState?.entryQuantity || entryOrder?.executedQty || entryOrder?.origQty || 0;
  const entryValue = positionState?.entryValue || entryPrice * quantity;
  const entryFees = positionState?.entryFees || entryValue * takerFeeRate; // Assume taker if unknown

  const tpPrice = tpOrder.price || tpOrder.stopPrice || 0;
  const exitValue = tpPrice * quantity;
  const exitFee = exitValue * takerFeeRate;

  // Calculate gross and net P&L
  const priceChange = positionSide === 'LONG'
    ? tpPrice - entryPrice
    : entryPrice - tpPrice;
  const grossPnL = priceChange * quantity;
  const expectedPnL = grossPnL - entryFees - exitFee;
  const expectedPnLPercent = entryValue > 0 ? (expectedPnL / entryValue) * 100 : 0;

  return {
    price: tpPrice,
    expectedPnL,
    expectedPnLPercent,
    entryFee: entryFees,
    exitFee,
    grossPnL,
    status: tpOrder.status,
    orderId: tpOrder.orderId,
    orderType: tpOrder.orderType || 'TP',
  };
}

function calculateSLEconomics(
  slOrder: ChainOrder,
  positionState: PositionState | undefined,
  entryOrder: ChainOrder | null,
  positionSide: 'LONG' | 'SHORT' | 'BOTH' | null,
  takerFeeRate: number
): TPSLEconomics | null {
  if (!positionState && !entryOrder) return null;

  const entryPrice = positionState?.entryPrice || entryOrder?.avgPrice || entryOrder?.price || 0;
  const quantity = positionState?.entryQuantity || entryOrder?.executedQty || entryOrder?.origQty || 0;
  const entryValue = positionState?.entryValue || entryPrice * quantity;
  const entryFees = positionState?.entryFees || entryValue * takerFeeRate;

  const slPrice = slOrder.stopPrice || slOrder.price || 0;
  const exitValue = slPrice * quantity;
  const exitFee = exitValue * takerFeeRate;

  // Calculate gross and net P&L (negative for SL)
  const priceChange = positionSide === 'LONG'
    ? slPrice - entryPrice
    : entryPrice - slPrice;
  const grossPnL = priceChange * quantity;
  const expectedPnL = grossPnL - entryFees - exitFee;
  const expectedPnLPercent = entryValue > 0 ? (expectedPnL / entryValue) * 100 : 0;

  return {
    price: slPrice,
    expectedPnL,
    expectedPnLPercent,
    entryFee: entryFees,
    exitFee,
    grossPnL,
    status: slOrder.status,
    orderId: slOrder.orderId,
    orderType: 'SL',
  };
}

// Format currency with appropriate precision (null-safe)
function formatCurrency(value: number | null | undefined): string {
  const safeValue = value ?? 0;
  const absValue = Math.abs(safeValue);
  if (absValue >= 100) return `$${safeValue.toFixed(2)}`;
  if (absValue >= 1) return `$${safeValue.toFixed(4)}`;
  return `$${safeValue.toFixed(6)}`;
}

// Format percentage with sign (null-safe)
function formatPnLPercent(value: number | null | undefined): string {
  const safeValue = value ?? 0;
  return `${safeValue >= 0 ? '+' : ''}${safeValue.toFixed(2)}%`;
}

// Convert chain data to ExpandedPositionData for position detail display
function convertToExpandedPositionData(
  chain: OrderChain,
  positionState: PositionState,
  analytics?: PositionAnalyticsData,
  livePrice?: number  // Real-time price from coin profiler
): ExpandedPositionData {
  // Get SL price from order or analytics
  const slPrice = chain.slOrder?.stopPrice || chain.slOrder?.price || analytics?.stop_loss;

  // Calculate unrealized PnL using live price if available, otherwise analytics or entry price
  const currentPrice = livePrice ?? analytics?.current_price ?? positionState.entryPrice;
  const priceChange = chain.positionSide === 'LONG'
    ? currentPrice - positionState.entryPrice
    : positionState.entryPrice - currentPrice;
  const unrealizedPnl = priceChange * positionState.remainingQuantity;
  const entryValue = positionState.entryValue || positionState.entryPrice * positionState.entryQuantity;
  const unrealizedPnlPercent = entryValue > 0 ? (unrealizedPnl / entryValue) * 100 : 0;

  return {
    symbol: chain.symbol || positionState.symbol,
    side: chain.positionSide === 'LONG' ? 'LONG' : 'SHORT',
    entry_price: positionState.entryPrice,
    current_price: currentPrice,
    quantity: positionState.entryQuantity,
    leverage: 1, // Default leverage (not tracked in position state)
    // Use calculated unrealized P&L based on live price (if available)
    unrealized_pnl: livePrice ? unrealizedPnl : (analytics?.unrealized_pnl ?? unrealizedPnl),
    unrealized_pnl_percent: unrealizedPnlPercent,
    roe: analytics?.roe ?? unrealizedPnlPercent,
    stage: analytics?.stage ?? 'RISK_ZONE',
    stage_entry_time: analytics?.stage_entry_time ?? new Date(positionState.entryFilledAt).getTime(),
    stage_progress_percent: 0,
    stop_loss: slPrice,
    take_profit: chain.tpOrders[0]?.price,
    breakeven_price: analytics?.breakeven_price,
    tp1_price: analytics?.tp1_price ?? chain.tpOrders[0]?.price,
    tp2_price: analytics?.tp2_price ?? chain.tpOrders[1]?.price,
    tp3_price: analytics?.tp3_price ?? chain.tpOrders[2]?.price,
    decision_mode: analytics?.decision_mode ?? 'classic',
    efficiency: analytics?.efficiency,
    classic_scores: analytics?.classic_scores,
    new_engine_scores: analytics?.new_engine_scores,
    entry_time: new Date(positionState.entryFilledAt).getTime(),
    last_updated: new Date(positionState.updatedAt).getTime(),
  };
}

export default function ChainCard({
  chain,
  compact = false,
  useTreeView = true,
  makerFeeRate = 0.0002,  // 0.02% VIP0 maker
  takerFeeRate = 0.0004,  // 0.04% VIP0 taker
  livePrice,             // Real-time price from coin profiler
}: ChainCardProps) {
  // Use live price if available, otherwise fall back to analytics/position state
  const currentPrice = livePrice ?? chain.positionAnalytics?.current_price ?? chain.positionState?.entryPrice ?? 0;

  // Calculate live unrealized P&L based on current price
  const liveUnrealizedPnl = useMemo(() => {
    if (!chain.positionState || currentPrice <= 0) {
      return chain.positionAnalytics?.unrealized_pnl ?? 0;
    }
    const entryPrice = chain.positionState.entryPrice;
    const qty = chain.positionState.remainingQuantity || chain.positionState.entryQuantity;
    const isLong = chain.positionSide === 'LONG';
    const priceChange = isLong ? currentPrice - entryPrice : entryPrice - currentPrice;
    return priceChange * qty;
  }, [chain.positionState, chain.positionSide, chain.positionAnalytics?.unrealized_pnl, currentPrice]);

  const [expanded, setExpanded] = useState(false);
  const [showLegacyView, setShowLegacyView] = useState(false); // Toggle to show old horizontal view
  const [tpSectionExpanded, setTpSectionExpanded] = useState(false);
  const [slSectionExpanded, setSlSectionExpanded] = useState(false);
  const [slModHistoryExpanded, setSlModHistoryExpanded] = useState(false);
  const [analyticsExpanded, setAnalyticsExpanded] = useState(false); // Story 11.40: Position Analytics expansion
  const [modificationData, setModificationData] = useState<Record<ModifiableOrderType, ModificationEvent[]>>({
    SL: [],
    TP1: [],
    TP2: [],
    TP3: [],
    TP4: [],
  });
  const [modificationLoading, setModificationLoading] = useState(false);
  const [modificationsLoaded, setModificationsLoaded] = useState(false);

  // Use user's timezone for all datetime displays
  const { formatDateTime } = useUserTimezone();

  // Fetch modification history when expanded (lazy load)
  useEffect(() => {
    if (!expanded || modificationsLoaded) return;

    const fetchModificationHistory = async () => {
      setModificationLoading(true);
      try {
        const response = await futuresApi.getChainModificationHistory(chain.chainId);
        if (response.modifications) {
          setModificationData(prev => ({
            ...prev,
            ...Object.fromEntries(
              Object.entries(response.modifications).map(([key, value]) => [
                key as ModifiableOrderType,
                value.events || [],
              ])
            ),
          }));
        }
        setModificationsLoaded(true);
      } catch (err) {
        console.error('Failed to fetch modification history:', err);
        // Silently fail - component will show empty state
      } finally {
        setModificationLoading(false);
      }
    };

    fetchModificationHistory();
  }, [expanded, chain.chainId, modificationsLoaded]);

  // Callback to load modifications for a specific order type
  // Uses ref to avoid stale closure issues with modificationData dependency
  const loadModifications = useCallback(async (orderType: ModifiableOrderType): Promise<ModificationEvent[]> => {
    // Check current state via setter function to avoid stale closure
    return new Promise((resolve) => {
      setModificationData(prev => {
        if (prev[orderType].length > 0) {
          resolve(prev[orderType]);
          return prev;
        }

        // Fetch asynchronously
        futuresApi.getChainModificationHistory(chain.chainId)
          .then(response => {
            if (response.modifications && response.modifications[orderType]) {
              const events = response.modifications[orderType].events || [];
              setModificationData(current => ({
                ...current,
                [orderType]: events,
              }));
              resolve(events);
            } else {
              resolve([]);
            }
          })
          .catch(err => {
            console.error(`Failed to fetch modifications for ${orderType}:`, err);
            resolve([]);
          });

        return prev; // Don't modify state here, let the async handler do it
      });
    });
  }, [chain.chainId]);

  // Get status badge styling
  const getStatusBadge = (status: string) => {
    const configs: Record<string, { color: string; bg: string; label: string }> = {
      active: { color: 'text-green-400', bg: 'bg-green-500/20', label: 'Active' },
      partial: { color: 'text-yellow-400', bg: 'bg-yellow-500/20', label: 'Partial' },
      completed: { color: 'text-blue-400', bg: 'bg-blue-500/20', label: 'Completed' },
      cancelled: { color: 'text-gray-400', bg: 'bg-gray-500/20', label: 'Cancelled' },
      closed: { color: 'text-blue-400', bg: 'bg-blue-500/20', label: 'Closed' },
    };
    const config = configs[status] || configs.active;
    return (
      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${config.bg} ${config.color}`}>
        {config.label}
      </span>
    );
  };

  // Format price with 8 decimal precision (Binance standard, null-safe)
  const formatPrice = (price: number | null | undefined) => {
    const safePrice = price ?? 0;
    return safePrice.toFixed(8);
  };

  // Direction icon
  const DirectionIcon = chain.positionSide === 'LONG' ? TrendingUp : TrendingDown;
  const directionColor = chain.positionSide === 'LONG' ? 'text-green-400' : 'text-red-400';

  // Build entry order from position state if entry order is null (entry already filled)
  const entryOrder = chain.entryOrder || (chain.positionState ? buildEntryFromPositionState(chain.positionState) : null);

  // Get total modification count for display in header
  const totalModifications = chain.modificationCounts
    ? Object.values(chain.modificationCounts).reduce((sum, count) => sum + count, 0)
    : 0;

  // Get exit orders (TP and SL) for tree view
  const exitOrders = [
    ...chain.tpOrders,
    ...(chain.slOrder ? [chain.slOrder] : []),
  ];

  // Calculate TP economics for all TP orders
  const tpEconomics = useMemo(() => {
    return chain.tpOrders.map(tp =>
      calculateTPEconomics(tp, chain.positionState, entryOrder, chain.positionSide, takerFeeRate)
    ).filter((e): e is TPSLEconomics => e !== null);
  }, [chain.tpOrders, chain.positionState, entryOrder, chain.positionSide, takerFeeRate]);

  // Calculate SL economics
  const slEconomics = useMemo(() => {
    if (!chain.slOrder) return null;
    return calculateSLEconomics(chain.slOrder, chain.positionState, entryOrder, chain.positionSide, takerFeeRate);
  }, [chain.slOrder, chain.positionState, entryOrder, chain.positionSide, takerFeeRate]);

  // Get TP modification counts
  const tpModCount = useMemo(() => {
    return chain.tpOrders.reduce((sum, tp) => {
      const tpType = tp.orderType || 'TP1';
      return sum + (chain.modificationCounts?.[tpType] || 0);
    }, 0);
  }, [chain.tpOrders, chain.modificationCounts]);

  // Get SL modification count
  const slModCount = chain.modificationCounts?.SL || 0;

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div
        className="p-4 flex items-center justify-between cursor-pointer hover:bg-gray-800/80 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-purple-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-500" />
          )}

          {/* Chain ID and Mode */}
          <div className="flex items-center gap-2">
            <Layers className="w-4 h-4 text-purple-400" />
            <span className="font-mono text-sm text-gray-200">{chain.chainId}</span>
            {chain.isFallback && (
              <span className="px-1.5 py-0.5 rounded text-xs bg-orange-500/20 text-orange-400">
                Fallback
              </span>
            )}
          </div>

          {/* Mode badge */}
          {chain.modeCode && (
            <span className="px-2 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400 font-medium">
              {MODE_DISPLAY_NAMES[chain.modeCode]}
            </span>
          )}

          {/* Status */}
          {getStatusBadge(chain.status)}

          {/* Modification count badge */}
          {totalModifications > 0 && (
            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
              <Edit3 className="w-3 h-3" />
              {totalModifications} mods
            </span>
          )}

          {/* Story 7.21: Position state indicator - only show if different from chain status */}
          {/* Map chain status to position state status for comparison:
              chain 'active' = position 'ACTIVE'
              chain 'partial' = position 'PARTIAL'
              chain 'completed' = position 'CLOSED'
              chain 'cancelled' = (no equivalent, always show)
          */}
          {chain.positionState && (() => {
            // Determine if position state status would duplicate chain status badge
            const positionStatus = chain.positionState.status;
            // Map chain status to position state equivalent
            const statusEquivalent =
              (chain.status === 'active' && positionStatus === 'ACTIVE') ||
              (chain.status === 'partial' && positionStatus === 'PARTIAL') ||
              (chain.status === 'completed' && positionStatus === 'CLOSED');
            // Don't show duplicate badge
            if (statusEquivalent) return null;
            // Don't show CLOSED position status when chain is still active/partial
            // This prevents confusing display of "CLOSED" on open orders
            if (positionStatus === 'CLOSED' && (chain.status === 'active' || chain.status === 'partial')) {
              return null;
            }
            return (
              <span className={`px-1.5 py-0.5 rounded text-xs ${
                positionStatus === 'ACTIVE' ? 'bg-green-500/20 text-green-400' :
                positionStatus === 'PARTIAL' ? 'bg-yellow-500/20 text-yellow-400' :
                'bg-blue-500/20 text-blue-400'
              }`}>
                {positionStatus}
              </span>
            );
          })()}
        </div>

        <div className="flex items-center gap-4">
          {/* Symbol and direction */}
          <div className="flex items-center gap-2">
            <span className="text-gray-200 font-semibold">{chain.symbol}</span>
            <DirectionIcon className={`w-4 h-4 ${directionColor}`} />
            <span className={`text-sm ${directionColor}`}>{chain.positionSide}</span>
          </div>

          {/* Order count */}
          <div className="flex items-center gap-1.5 text-gray-400 text-sm">
            <Activity className="w-3.5 h-3.5" />
            <span>{chain.orders.length} orders</span>
          </div>

          {/* Time - User's timezone */}
          <div className="flex items-center gap-1.5 text-gray-500 text-xs">
            <Clock className="w-3 h-3" />
            <span>{formatDateTime(chain.createdAt)}</span>
          </div>
        </div>
      </div>

      {/* Expanded content - Tree View */}
      {expanded && useTreeView && !showLegacyView && (
        <div className="border-t border-gray-700 p-4 space-y-4">
          {/* Tree View Toggle */}
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <GitBranch className="w-4 h-4 text-purple-400" />
              Order Tree
            </h4>
            <button
              onClick={(e) => { e.stopPropagation(); setShowLegacyView(true); }}
              className="text-xs text-gray-500 hover:text-gray-400 transition-colors"
            >
              Switch to List View
            </button>
          </div>

          {/* Tree Structure */}
          <div className="space-y-2 bg-gray-900/50 rounded-lg p-3">
            {/* Entry Order - Always at root (depth 0) */}
            {entryOrder && (
              <OrderTreeNode
                type="ENTRY"
                order={entryOrder}
                chainId={chain.chainId}
                positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                depth={0}
                isLast={!chain.positionState && exitOrders.length === 0}
              />
            )}

            {/* Position State - Child of entry (depth 1) - Minimal card with stage badge */}
            {chain.positionState && (
              <div className="mt-1">
                <OrderTreeNode
                  type="POSITION"
                  positionState={chain.positionState}
                  positionAnalytics={chain.positionAnalytics}
                  chainId={chain.chainId}
                  positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                  depth={1}
                  isLast={exitOrders.length === 0}
                />

                {/* Exit Orders (TP/SL) - Children of position (depth 2, parallel) */}
                {exitOrders.length > 0 && (
                  <div className="space-y-1 mt-1">
                    {chain.tpOrders.map((tp, idx) => (
                      <OrderTreeNode
                        key={tp.orderId}
                        type={(tp.orderType || 'TP1') as TreeNodeType}
                        order={tp}
                        chainId={chain.chainId}
                        positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                        depth={2}
                        isLast={!chain.slOrder && idx === chain.tpOrders.length - 1}
                        modificationCount={chain.modificationCounts?.[tp.orderType || 'TP1'] || 0}
                        modifications={modificationData[(tp.orderType || 'TP1') as ModifiableOrderType]}
                        onLoadModifications={loadModifications}
                        entryPrice={chain.positionState?.entryPrice || entryOrder?.avgPrice || entryOrder?.price}
                        entryQuantity={chain.positionState?.entryQuantity || entryOrder?.executedQty || entryOrder?.origQty}
                        feeRate={takerFeeRate}
                      />
                    ))}
                    {chain.slOrder && (
                      <OrderTreeNode
                        type="SL"
                        order={chain.slOrder}
                        chainId={chain.chainId}
                        positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                        depth={2}
                        isLast={true}
                        modificationCount={chain.modificationCounts?.SL || 0}
                        modifications={modificationData.SL}
                        onLoadModifications={loadModifications}
                        entryPrice={chain.positionState?.entryPrice || entryOrder?.avgPrice || entryOrder?.price}
                        entryQuantity={chain.positionState?.entryQuantity || entryOrder?.executedQty || entryOrder?.origQty}
                        feeRate={takerFeeRate}
                      />
                    )}
                  </div>
                )}
              </div>
            )}

            {/* If no position state but we have exit orders, show them as children of entry */}
            {!chain.positionState && exitOrders.length > 0 && (
              <div className="space-y-1 mt-1">
                {chain.tpOrders.map((tp, idx) => (
                  <OrderTreeNode
                    key={tp.orderId}
                    type={(tp.orderType || 'TP1') as TreeNodeType}
                    order={tp}
                    chainId={chain.chainId}
                    positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                    depth={1}
                    isLast={!chain.slOrder && idx === chain.tpOrders.length - 1}
                    modificationCount={chain.modificationCounts?.[tp.orderType || 'TP1'] || 0}
                    modifications={modificationData[(tp.orderType || 'TP1') as ModifiableOrderType]}
                    onLoadModifications={loadModifications}
                    entryPrice={entryOrder?.avgPrice || entryOrder?.price}
                    entryQuantity={entryOrder?.executedQty || entryOrder?.origQty}
                    feeRate={takerFeeRate}
                  />
                ))}
                {chain.slOrder && (
                  <OrderTreeNode
                    type="SL"
                    order={chain.slOrder}
                    chainId={chain.chainId}
                    positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                    depth={1}
                    isLast={true}
                    modificationCount={chain.modificationCounts?.SL || 0}
                    modifications={modificationData.SL}
                    onLoadModifications={loadModifications}
                    entryPrice={entryOrder?.avgPrice || entryOrder?.price}
                    entryQuantity={entryOrder?.executedQty || entryOrder?.origQty}
                    feeRate={takerFeeRate}
                  />
                )}
              </div>
            )}

            {/* DCA Orders - Separate branch */}
            {chain.dcaOrders.length > 0 && (
              <div className="mt-3 pt-3 border-t border-gray-700/50">
                <span className="text-xs text-gray-500 mb-2 block">DCA Orders</span>
                <div className="space-y-1">
                  {chain.dcaOrders.map((dca, idx) => (
                    <OrderTreeNode
                      key={dca.orderId}
                      type={(dca.orderType || 'DCA1') as TreeNodeType}
                      order={dca}
                      chainId={chain.chainId}
                      positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                      depth={0}
                      isLast={idx === chain.dcaOrders.length - 1}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Hedge Orders - Separate branch */}
            {(chain.hedgeOrder || chain.hedgeSLOrder || chain.hedgeTPOrder) && (
              <div className="mt-3 pt-3 border-t border-gray-700/50">
                <span className="text-xs text-gray-500 mb-2 block">Hedge Orders</span>
                <div className="space-y-1">
                  {chain.hedgeOrder && (
                    <OrderTreeNode
                      type="H"
                      order={chain.hedgeOrder}
                      chainId={chain.chainId}
                      positionSide={chain.positionSide === 'LONG' ? 'SHORT' : 'LONG'} // Hedge is opposite
                      depth={0}
                      isLast={!chain.hedgeSLOrder && !chain.hedgeTPOrder}
                    />
                  )}
                  {chain.hedgeSLOrder && (
                    <OrderTreeNode
                      type="HSL"
                      order={chain.hedgeSLOrder}
                      chainId={chain.chainId}
                      positionSide={chain.positionSide === 'LONG' ? 'SHORT' : 'LONG'}
                      depth={1}
                      isLast={!chain.hedgeTPOrder}
                    />
                  )}
                  {chain.hedgeTPOrder && (
                    <OrderTreeNode
                      type="HTP"
                      order={chain.hedgeTPOrder}
                      chainId={chain.chainId}
                      positionSide={chain.positionSide === 'LONG' ? 'SHORT' : 'LONG'}
                      depth={1}
                      isLast={true}
                    />
                  )}
                </div>
              </div>
            )}

            {/* Rebuy Order - Separate branch */}
            {chain.rebuyOrder && (
              <div className="mt-3 pt-3 border-t border-gray-700/50">
                <span className="text-xs text-gray-500 mb-2 block">Rebuy</span>
                <OrderTreeNode
                  type="RB"
                  order={chain.rebuyOrder}
                  chainId={chain.chainId}
                  positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                  depth={0}
                  isLast={true}
                />
              </div>
            )}
          </div>

          {/* Detailed Position Section - Shown below Order Tree for active/partial chains with position state */}
          {chain.positionState && (chain.status === 'active' || chain.status === 'partial') && (
            <div className="border border-gray-700 rounded-lg overflow-hidden bg-gray-900/30">
              <div className="px-4 py-3 border-b border-gray-700 bg-gray-800/50">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Target className="w-4 h-4 text-cyan-400" />
                    <span className="text-sm font-medium text-gray-300">Position Details</span>
                    {chain.positionAnalytics && (
                      <StageBadge stage={chain.positionAnalytics.stage} compact />
                    )}
                  </div>
                  <div className="flex items-center gap-3">
                    {/* Realized P&L if any */}
                    {chain.positionState.realizedPnl !== 0 && (
                      <span className={`text-xs ${chain.positionState.realizedPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        Realized: {chain.positionState.realizedPnl >= 0 ? '+' : ''}{chain.positionState.realizedPnl.toFixed(4)} USDT
                      </span>
                    )}
                    {/* Unrealized P&L - calculated live */}
                    <span className={`text-sm font-semibold ${liveUnrealizedPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {liveUnrealizedPnl >= 0 ? '+' : ''}{liveUnrealizedPnl.toFixed(2)} USDT
                    </span>
                  </div>
                </div>
              </div>

              <div className="p-4 space-y-4">
                {/* Position Stage Progress Bar */}
                {chain.positionAnalytics && (
                  <div>
                    <h4 className="text-sm font-medium text-gray-400 mb-3 flex items-center gap-2">
                      <Activity className="w-4 h-4 text-purple-400" />
                      Stage Progress
                    </h4>
                    <StageTimeline
                      currentStage={chain.positionAnalytics.stage}
                      stageEntryTime={chain.positionAnalytics.stage_entry_time}
                    />
                  </div>
                )}

                {/* Price Levels Visualization */}
                <div>
                  <h4 className="text-sm font-medium text-gray-400 mb-3 flex items-center gap-2">
                    <Layers className="w-4 h-4 text-yellow-400" />
                    Price Levels
                  </h4>
                  <PriceLevelsDisplay
                    position={convertToExpandedPositionData(chain, chain.positionState, chain.positionAnalytics, currentPrice)}
                  />
                </div>

                {/* Risk Metrics Grid */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  <div className="bg-gray-800/50 rounded-lg p-3">
                    <span className="text-xs text-gray-500 block mb-1">Entry Price</span>
                    <span className="text-sm font-mono text-blue-400">${chain.positionState.entryPrice.toFixed(8)}</span>
                  </div>
                  <div className="bg-gray-800/50 rounded-lg p-3">
                    <span className="text-xs text-gray-500 block mb-1">Entry Value</span>
                    <span className="text-sm font-mono text-gray-300">${chain.positionState.entryValue.toFixed(8)}</span>
                  </div>
                  <div className="bg-gray-800/50 rounded-lg p-3">
                    <span className="text-xs text-gray-500 block mb-1">Quantity</span>
                    <span className="text-sm font-mono text-gray-300">
                      {chain.positionState.remainingQuantity.toFixed(4)}
                      {chain.positionState.remainingQuantity !== chain.positionState.entryQuantity && (
                        <span className="text-gray-500 text-xs ml-1">
                          / {chain.positionState.entryQuantity.toFixed(4)}
                        </span>
                      )}
                    </span>
                  </div>
                  <div className="bg-gray-800/50 rounded-lg p-3">
                    <span className="text-xs text-gray-500 block mb-1">Entry Fees</span>
                    <span className="text-sm font-mono text-orange-400">${chain.positionState.entryFees.toFixed(8)}</span>
                  </div>
                </div>

                {/* Current Price & SL/TP Levels */}
                {(chain.positionAnalytics || currentPrice > 0) && (
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                    <div className="bg-gray-800/50 rounded-lg p-3">
                      <span className="text-xs text-gray-500 block mb-1">Current Price</span>
                      <span className="text-sm font-mono text-white">${currentPrice.toFixed(8)}</span>
                    </div>
                    {chain.positionAnalytics?.breakeven_price && (
                      <div className="bg-gray-800/50 rounded-lg p-3">
                        <span className="text-xs text-gray-500 block mb-1">Breakeven</span>
                        <span className="text-sm font-mono text-yellow-400">${chain.positionAnalytics.breakeven_price.toFixed(8)}</span>
                      </div>
                    )}
                    {chain.slOrder && (
                      <div className="bg-gray-800/50 rounded-lg p-3">
                        <span className="text-xs text-gray-500 block mb-1">Stop Loss</span>
                        <span className="text-sm font-mono text-red-400">${(chain.slOrder.stopPrice || chain.slOrder.price || 0).toFixed(8)}</span>
                      </div>
                    )}
                    {chain.tpOrders.length > 0 && (
                      <div className="bg-gray-800/50 rounded-lg p-3">
                        <span className="text-xs text-gray-500 block mb-1">TP1</span>
                        <span className="text-sm font-mono text-green-400">${(chain.tpOrders[0].price || 0).toFixed(8)}</span>
                      </div>
                    )}
                  </div>
                )}

                {/* ROE Display */}
                {chain.positionAnalytics && (
                  <div className="flex items-center justify-between pt-3 border-t border-gray-700">
                    <span className="text-sm text-gray-400">Return on Equity (ROE)</span>
                    <span className={`text-sm font-semibold ${(chain.positionAnalytics.roe ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {(chain.positionAnalytics.roe ?? 0) >= 0 ? '+' : ''}{(chain.positionAnalytics.roe ?? 0).toFixed(2)}%
                    </span>
                  </div>
                )}
              </div>
            </div>
          )}

          {/* Story 11.40: Position Analytics Section */}
          {chain.positionAnalytics && (
            <div className="border border-gray-700 rounded-lg overflow-hidden">
              <button
                type="button"
                onClick={(e) => { e.stopPropagation(); setAnalyticsExpanded(!analyticsExpanded); }}
                className="w-full px-4 py-3 flex items-center justify-between bg-gray-800/50 hover:bg-gray-700/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <Activity className="w-4 h-4 text-purple-400" />
                  <span className="text-sm font-medium text-gray-300">Position Analytics</span>
                  <StageBadge stage={chain.positionAnalytics.stage} compact />
                  <DecisionModeBadge mode={chain.positionAnalytics.decision_mode} />
                </div>
                <div className="flex items-center gap-3">
                  {/* P&L - calculated live */}
                  <span className={`text-sm font-semibold ${liveUnrealizedPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {liveUnrealizedPnl >= 0 ? '+' : ''}{liveUnrealizedPnl.toFixed(2)} USDT
                  </span>
                  {analyticsExpanded ? <ChevronDown className="w-4 h-4 text-gray-400" /> : <ChevronRight className="w-4 h-4 text-gray-400" />}
                </div>
              </button>

              {analyticsExpanded && (
                <div className="p-4 border-t border-gray-700 space-y-4">
                  {/* Stage Timeline */}
                  <div>
                    <h4 className="text-sm font-medium text-gray-400 mb-3">Position Stage</h4>
                    <StageTimeline
                      currentStage={chain.positionAnalytics.stage}
                      stageEntryTime={chain.positionAnalytics.stage_entry_time}
                    />
                  </div>

                  {/* Price Levels Overview */}
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                    <div className="bg-gray-800/50 rounded-lg p-3">
                      <span className="text-xs text-gray-500 block mb-1">Current Price</span>
                      <span className="text-sm font-mono text-white">${currentPrice.toFixed(8)}</span>
                    </div>
                    {chain.positionAnalytics.breakeven_price && (
                      <div className="bg-gray-800/50 rounded-lg p-3">
                        <span className="text-xs text-gray-500 block mb-1">Breakeven</span>
                        <span className="text-sm font-mono text-yellow-400">${chain.positionAnalytics.breakeven_price.toFixed(8)}</span>
                      </div>
                    )}
                    {chain.positionAnalytics.tp1_price && (
                      <div className="bg-gray-800/50 rounded-lg p-3">
                        <span className="text-xs text-gray-500 block mb-1">TP1</span>
                        <span className="text-sm font-mono text-green-400">${chain.positionAnalytics.tp1_price.toFixed(8)}</span>
                      </div>
                    )}
                    {chain.positionAnalytics.stop_loss && (
                      <div className="bg-gray-800/50 rounded-lg p-3">
                        <span className="text-xs text-gray-500 block mb-1">Stop Loss</span>
                        <span className="text-sm font-mono text-red-400">${chain.positionAnalytics.stop_loss.toFixed(8)}</span>
                      </div>
                    )}
                  </div>

                  {/* Efficiency Metrics (when in EFFICIENCY stage) */}
                  {chain.positionAnalytics.stage === 'EFFICIENCY' && chain.positionAnalytics.efficiency && (
                    <EfficiencyMetricsDisplay metrics={chain.positionAnalytics.efficiency} />
                  )}

                  {/* Indicator Scores based on decision mode */}
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {chain.positionAnalytics.decision_mode === 'classic' && chain.positionAnalytics.classic_scores && (
                      <ClassicIndicatorDisplay scores={chain.positionAnalytics.classic_scores} />
                    )}
                    {chain.positionAnalytics.decision_mode === 'new_engine' && chain.positionAnalytics.new_engine_scores && (
                      <NewEngineIndicatorDisplay scores={chain.positionAnalytics.new_engine_scores} />
                    )}
                  </div>

                  {/* ROE Display */}
                  <div className="flex items-center justify-between pt-3 border-t border-gray-700">
                    <span className="text-sm text-gray-400">Return on Equity (ROE)</span>
                    <span className={`text-sm font-semibold ${(chain.positionAnalytics.roe ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {(chain.positionAnalytics.roe ?? 0) >= 0 ? '+' : ''}{(chain.positionAnalytics.roe ?? 0).toFixed(2)}%
                    </span>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Collapsible Take Profit Section */}
          {tpEconomics.length > 0 && (
            <div className="bg-gray-900/30 rounded-lg border border-gray-700/50 overflow-hidden">
              <button
                onClick={(e) => { e.stopPropagation(); setTpSectionExpanded(!tpSectionExpanded); }}
                className="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  {tpSectionExpanded ? (
                    <ChevronDown className="w-4 h-4 text-cyan-400" />
                  ) : (
                    <ChevronRight className="w-4 h-4 text-gray-500" />
                  )}
                  <TrendingUp className="w-4 h-4 text-cyan-400" />
                  <span className="text-sm font-medium text-cyan-400">Take Profit</span>
                  <span className="text-xs text-gray-500">({tpEconomics.length})</span>
                  {tpModCount > 0 && (
                    <span className="px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                      {tpModCount} mods
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  {/* Show total expected gain */}
                  <span className="text-sm font-mono text-green-400">
                    +{formatCurrency(tpEconomics.reduce((sum, tp) => sum + Math.max(0, tp.expectedPnL), 0))}
                  </span>
                </div>
              </button>

              {tpSectionExpanded && (
                <div className="px-4 pb-3 space-y-2">
                  {tpEconomics.map((tp) => (
                    <div
                      key={tp.orderId}
                      className="flex items-center justify-between py-2 px-3 bg-gray-800/50 rounded-lg text-sm"
                    >
                      <div className="flex items-center gap-3">
                        <Target className="w-4 h-4 text-cyan-400" />
                        <span className="font-medium text-cyan-400">{tp.orderType}</span>
                        <span className="font-mono text-gray-300">${formatPrice(tp.price)}</span>
                        {chain.modificationCounts?.[tp.orderType] && chain.modificationCounts[tp.orderType] > 0 && (
                          <span className="px-1 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                            {chain.modificationCounts[tp.orderType]} mods
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-4">
                        <div className="text-right">
                          <span className={`font-mono ${tp.expectedPnL >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {tp.expectedPnL >= 0 ? '+' : ''}{formatCurrency(tp.expectedPnL)}
                          </span>
                          <span className={`ml-2 text-xs ${tp.expectedPnL >= 0 ? 'text-green-400/70' : 'text-red-400/70'}`}>
                            ({formatPnLPercent(tp.expectedPnLPercent)})
                          </span>
                        </div>
                        <div className="flex items-center gap-1.5">
                          {tp.status === 'FILLED' ? (
                            <>
                              <Check className="w-3.5 h-3.5 text-green-400" />
                              <span className="text-xs text-green-400">Hit</span>
                            </>
                          ) : tp.status === 'CANCELED' ? (
                            <span className="text-xs text-gray-500">Cancelled</span>
                          ) : (
                            <>
                              <Clock className="w-3.5 h-3.5 text-yellow-400" />
                              <span className="text-xs text-yellow-400">Pending</span>
                            </>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Collapsible Stop Loss Section */}
          {slEconomics && (
            <div className="bg-gray-900/30 rounded-lg border border-gray-700/50 overflow-hidden">
              <button
                onClick={(e) => { e.stopPropagation(); setSlSectionExpanded(!slSectionExpanded); }}
                className="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  {slSectionExpanded ? (
                    <ChevronDown className="w-4 h-4 text-red-400" />
                  ) : (
                    <ChevronRight className="w-4 h-4 text-gray-500" />
                  )}
                  <TrendingDown className="w-4 h-4 text-red-400" />
                  <span className="text-sm font-medium text-red-400">Stop Loss</span>
                  {slModCount > 0 && (
                    <span className="px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                      {slModCount} mods
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  {/* Show max loss */}
                  <span className="text-sm font-mono text-red-400">
                    {formatCurrency(slEconomics.expectedPnL)}
                  </span>
                </div>
              </button>

              {slSectionExpanded && (
                <div className="px-4 pb-3 space-y-2">
                  {/* SL Main Row */}
                  <div className="flex items-center justify-between py-2 px-3 bg-gray-800/50 rounded-lg text-sm">
                    <div className="flex items-center gap-3">
                      <Shield className="w-4 h-4 text-red-400" />
                      <span className="font-medium text-red-400">SL</span>
                      <span className="font-mono text-gray-300">${formatPrice(slEconomics.price)}</span>
                    </div>
                    <div className="flex items-center gap-4">
                      <div className="text-right">
                        <span className="text-gray-400 text-xs mr-2">Max Loss:</span>
                        <span className="font-mono text-red-400">
                          {formatCurrency(slEconomics.expectedPnL)}
                        </span>
                        <span className="ml-2 text-xs text-red-400/70">
                          ({formatPnLPercent(slEconomics.expectedPnLPercent)})
                        </span>
                      </div>
                      {slModCount > 0 && (
                        <span className="text-xs text-gray-400">
                          Mods: {slModCount}
                        </span>
                      )}
                      <div className="flex items-center gap-1.5">
                        {slEconomics.status === 'FILLED' ? (
                          <>
                            <Check className="w-3.5 h-3.5 text-red-400" />
                            <span className="text-xs text-red-400">Hit</span>
                          </>
                        ) : slEconomics.status === 'CANCELED' ? (
                          <span className="text-xs text-gray-500">Cancelled</span>
                        ) : (
                          <>
                            <Shield className="w-3.5 h-3.5 text-green-400" />
                            <span className="text-xs text-green-400">Active</span>
                          </>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* SL Modification History (expandable sub-section) */}
                  {slModCount > 0 && (
                    <div className="ml-6">
                      <button
                        onClick={(e) => { e.stopPropagation(); setSlModHistoryExpanded(!slModHistoryExpanded); }}
                        className="flex items-center gap-2 text-xs text-gray-500 hover:text-gray-400 transition-colors py-1"
                      >
                        {slModHistoryExpanded ? (
                          <ChevronDown className="w-3 h-3" />
                        ) : (
                          <ChevronRight className="w-3 h-3" />
                        )}
                        <History className="w-3 h-3" />
                        <span>Modification History ({slModCount})</span>
                      </button>

                      {slModHistoryExpanded && modificationData.SL.length > 0 && (
                        <div className="mt-2 ml-4 bg-gray-900/50 rounded-lg p-3 border border-gray-700/50">
                          {/* Summary header */}
                          {(() => {
                            const summary = calculateSummaryStats(modificationData.SL);
                            return (
                              <div className="flex items-center gap-3 mb-2 pb-2 border-b border-gray-700/50 text-xs">
                                <span className="flex items-center gap-1 text-gray-400">
                                  <History className="w-3.5 h-3.5 text-purple-400" />
                                  {summary.totalModifications} modification{summary.totalModifications !== 1 ? 's' : ''}
                                </span>
                                <span className="text-gray-600">|</span>
                                <span className="text-gray-400">
                                  Initial: <span className="font-mono text-gray-300">${(summary.initialPrice ?? 0).toFixed(8)}</span>
                                </span>
                                <span className="text-gray-600">|</span>
                                <span className="text-gray-400">
                                  Current: <span className="font-mono text-green-400">${(slEconomics.price ?? 0).toFixed(8)}</span>
                                </span>
                                <span className="text-gray-600">|</span>
                                <span className={`font-medium ${(summary.netDollarImpact ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  Net: {formatDollarImpact(summary.netDollarImpact ?? 0)}
                                </span>
                              </div>
                            );
                          })()}
                          <ModificationRowList
                            events={modificationData.SL}
                            orderType="SL"
                          />
                        </div>
                      )}

                      {slModHistoryExpanded && modificationData.SL.length === 0 && !modificationLoading && (
                        <div className="mt-2 ml-4 text-xs text-gray-500">
                          Loading modification history...
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Chain info footer - Show ENTRY values only, not sum of all orders */}
          <div className="pt-3 border-t border-gray-700 grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-gray-500">Created:</span>
              <span className="ml-2 text-gray-300">
                {formatDateTime(chain.createdAt)}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Updated:</span>
              <span className="ml-2 text-gray-300">
                {formatDateTime(chain.updatedAt)}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Entry Value:</span>
              <span className="ml-2 text-gray-300 font-mono">
                {chain.positionState ? (
                  `$${(chain.positionState.entryValue ?? 0).toFixed(2)}`
                ) : entryOrder ? (
                  `$${(((entryOrder.stopPrice || entryOrder.price) ?? 0) * (entryOrder.origQty ?? 0)).toFixed(2)}`
                ) : (
                  <span className="text-gray-500">N/A</span>
                )}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Filled Value:</span>
              <span className="ml-2 text-gray-300 font-mono">
                {chain.positionState ? (
                  `$${(chain.positionState.entryValue ?? 0).toFixed(2)}`
                ) : entryOrder && entryOrder.status === 'FILLED' ? (
                  `$${(((entryOrder.avgPrice || entryOrder.price) ?? 0) * (entryOrder.executedQty ?? 0)).toFixed(2)}`
                ) : (
                  <span className="text-gray-500">$0.00</span>
                )}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Legacy List View (for fallback or preference) */}
      {expanded && (!useTreeView || showLegacyView) && (
        <LegacyChainView
          chain={chain}
          modificationData={modificationData}
          modificationLoading={modificationLoading}
          formatPrice={formatPrice}
          formatDateTime={formatDateTime}
          onToggleToTree={() => setShowLegacyView(false)}
          showToggle={useTreeView}
        />
      )}
    </div>
  );
}

// Legacy horizontal view component (kept for fallback/comparison)
interface LegacyChainViewProps {
  chain: OrderChain;
  modificationData: Record<ModifiableOrderType, ModificationEvent[]>;
  modificationLoading: boolean;
  formatPrice: (price: number) => string;
  formatDateTime: (input: string | number) => string; // Story 7.21 fix: Add formatDateTime prop
  onToggleToTree: () => void;
  showToggle: boolean;
}

function LegacyChainView({
  chain,
  modificationData,
  modificationLoading,
  formatPrice,
  formatDateTime,
  onToggleToTree,
  showToggle,
}: LegacyChainViewProps) {
  const [showModificationHistory, setShowModificationHistory] = useState(false);

  // Story 7.21: Handle historical chains with no order details
  // Historical chains from DB don't have individual order records from Binance
  if (chain.orders.length === 0) {
    return (
      <div className="border-t border-gray-700 p-4">
        {/* View Toggle */}
        {showToggle && (
          <div className="flex items-center justify-between mb-4">
            <h4 className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <Activity className="w-4 h-4" />
              Order Details (List View)
            </h4>
            <button
              onClick={(e) => { e.stopPropagation(); onToggleToTree(); }}
              className="text-xs text-purple-400 hover:text-purple-300 transition-colors"
            >
              Switch to Tree View
            </button>
          </div>
        )}

        {/* Fallback UI for historical chains without order details */}
        <div className="text-center py-6 text-gray-500">
          <History className="w-8 h-8 mx-auto mb-2" />
          <p className="font-medium">Historical Chain</p>
          <p className="text-sm mt-1">Order details not available from Binance</p>

          {/* Show what data we do have from the position state */}
          {chain.positionState && (
            <div className="mt-4 grid grid-cols-2 md:grid-cols-4 gap-3 text-left max-w-md mx-auto">
              <div className="bg-gray-800/50 rounded-lg p-2">
                <span className="text-xs text-gray-500 block">Entry Price</span>
                <span className="text-sm font-mono text-gray-300">
                  ${chain.positionState.entryPrice?.toFixed(8) || 'N/A'}
                </span>
              </div>
              <div className="bg-gray-800/50 rounded-lg p-2">
                <span className="text-xs text-gray-500 block">Quantity</span>
                <span className="text-sm font-mono text-gray-300">
                  {chain.positionState.entryQuantity || 'N/A'}
                </span>
              </div>
              <div className="bg-gray-800/50 rounded-lg p-2">
                <span className="text-xs text-gray-500 block">Status</span>
                <span className={`text-sm ${
                  chain.positionState.status === 'ACTIVE' ? 'text-green-400' :
                  chain.positionState.status === 'PARTIAL' ? 'text-yellow-400' :
                  chain.positionState.status === 'CLOSED' ? 'text-blue-400' :
                  'text-gray-400'
                }`}>
                  {chain.positionState.status}
                </span>
              </div>
              <div className="bg-gray-800/50 rounded-lg p-2">
                <span className="text-xs text-gray-500 block">Realized PnL</span>
                <span className={`text-sm font-mono ${
                  (chain.positionState.realizedPnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'
                }`}>
                  {(chain.positionState.realizedPnl || 0) >= 0 ? '+' : ''}
                  ${(chain.positionState.realizedPnl || 0).toFixed(4)}
                </span>
              </div>
            </div>
          )}

          {/* If no position state, show basic chain info */}
          {!chain.positionState && (
            <div className="mt-4 text-xs text-gray-600">
              Chain ID: {chain.chainId} | Status: {chain.status}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Get order type badge
  const getOrderTypeBadge = (orderType: OrderTypeSuffix | null) => {
    if (!orderType) return null;
    const config = ORDER_TYPE_CONFIG[orderType];
    if (!config) return null;
    return (
      <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium ${config.bgColor} ${config.color}`}>
        {config.label}
      </span>
    );
  };

  // Get order status badge
  const getOrderStatusBadge = (status: string) => {
    const configs: Record<string, { color: string; bg: string }> = {
      NEW: { color: 'text-blue-400', bg: 'bg-blue-500/10' },
      PARTIALLY_FILLED: { color: 'text-yellow-400', bg: 'bg-yellow-500/10' },
      FILLED: { color: 'text-green-400', bg: 'bg-green-500/10' },
      CANCELED: { color: 'text-gray-400', bg: 'bg-gray-500/10' },
      EXPIRED: { color: 'text-orange-400', bg: 'bg-orange-500/10' },
      REJECTED: { color: 'text-red-400', bg: 'bg-red-500/10' },
    };
    const config = configs[status] || { color: 'text-gray-400', bg: 'bg-gray-500/10' };
    return (
      <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs ${config.bg} ${config.color}`}>
        {status.replace('_', ' ')}
      </span>
    );
  };

  // Render order row
  const renderOrderRow = (order: ChainOrder) => (
    <div
      key={order.orderId}
      className="flex items-center justify-between py-2 px-3 bg-gray-900/50 rounded-lg text-sm"
    >
      <div className="flex items-center gap-3">
        {getOrderTypeBadge(order.orderType)}
        <span className="font-mono text-gray-400 text-xs">{order.type}</span>
        {getOrderStatusBadge(order.status)}
      </div>
      <div className="flex items-center gap-4 text-right">
        <div>
          <div className="text-gray-300 font-mono">{formatPrice(order.price)}</div>
          <div className="text-xs text-gray-500">Price</div>
        </div>
        <div>
          <div className="text-gray-300 font-mono">
            {(order.executedQty ?? 0).toFixed(4)}/{(order.origQty ?? 0).toFixed(4)}
          </div>
          <div className="text-xs text-gray-500">Filled/Total</div>
        </div>
        {order.stopPrice && order.stopPrice > 0 && (
          <div>
            <div className="text-yellow-400 font-mono">{formatPrice(order.stopPrice)}</div>
            <div className="text-xs text-gray-500">Stop</div>
          </div>
        )}
      </div>
    </div>
  );

  return (
    <div className="border-t border-gray-700 p-4 space-y-4">
      {/* View Toggle */}
      {showToggle && (
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-medium text-gray-400 flex items-center gap-2">
            <Activity className="w-4 h-4" />
            Order Details (List View)
          </h4>
          <button
            onClick={(e) => { e.stopPropagation(); onToggleToTree(); }}
            className="text-xs text-purple-400 hover:text-purple-300 transition-colors"
          >
            Switch to Tree View
          </button>
        </div>
      )}

      {/* Chain structure visualization - horizontal */}
      <div className="flex items-center gap-2 overflow-x-auto pb-2">
        {/* Entry */}
        {chain.entryOrder && (
          <div className="flex items-center gap-1">
            <div className={`px-3 py-1.5 rounded-lg ${ORDER_TYPE_CONFIG.E.bgColor} border border-green-500/30`}>
              <div className="flex items-center gap-2">
                <TrendingUp className="w-4 h-4 text-green-400" />
                <span className="text-green-400 font-medium">Entry</span>
              </div>
              <div className="text-xs text-gray-400 mt-1">
                {formatPrice(chain.entryOrder.price)}
              </div>
            </div>
            <div className="w-4 h-0.5 bg-gray-600" />
          </div>
        )}

        {/* Take Profits */}
        {chain.tpOrders.map((tp, idx) => {
          const tpConfig = tp.orderType ? ORDER_TYPE_CONFIG[tp.orderType] : null;
          return (
            <div key={tp.orderId} className="flex items-center gap-1">
              <div className={`px-3 py-1.5 rounded-lg ${tpConfig?.bgColor || 'bg-cyan-500/20'} border border-cyan-500/30`}>
                <div className="flex items-center gap-2">
                  <Target className="w-4 h-4 text-cyan-400" />
                  <span className="text-cyan-400 font-medium">{tp.orderType || 'TP'}</span>
                </div>
                <div className="text-xs text-gray-400 mt-1">
                  {formatPrice(tp.price)}
                </div>
              </div>
              {idx < chain.tpOrders.length - 1 && <div className="w-2 h-0.5 bg-gray-600" />}
            </div>
          );
        })}

        {/* Stop Loss */}
        {chain.slOrder && (
          <div className="flex items-center gap-1">
            {(chain.tpOrders.length > 0 || chain.entryOrder) && (
              <div className="w-4 h-0.5 bg-gray-600" />
            )}
            <div className={`px-3 py-1.5 rounded-lg ${ORDER_TYPE_CONFIG.SL.bgColor} border border-red-500/30`}>
              <div className="flex items-center gap-2">
                <Shield className="w-4 h-4 text-red-400" />
                <span className="text-red-400 font-medium">SL</span>
              </div>
              <div className="text-xs text-gray-400 mt-1">
                {formatPrice(chain.slOrder.stopPrice || chain.slOrder.price)}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* Order details */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium text-gray-400 flex items-center gap-2">
          <Activity className="w-4 h-4" />
          All Orders
        </h4>
        <div className="space-y-1.5">
          {chain.orders.map(renderOrderRow)}
        </div>
      </div>

      {/* Modification History Section */}
      {(chain.slOrder || chain.tpOrders.length > 0) && (
        <div className="space-y-3">
          <button
            onClick={() => setShowModificationHistory(!showModificationHistory)}
            className="flex items-center gap-2 text-sm font-medium text-gray-400 hover:text-gray-300 transition-colors"
          >
            {showModificationHistory ? (
              <ChevronDown className="w-4 h-4 text-purple-400" />
            ) : (
              <ChevronRight className="w-4 h-4" />
            )}
            <History className="w-4 h-4 text-purple-400" />
            <span>Modification History</span>
            <span className="text-xs text-gray-500">
              (SL/TP price changes)
            </span>
          </button>

          {showModificationHistory && (
            <div className="space-y-3 pl-6">
              {modificationLoading ? (
                <div className="text-sm text-gray-500 animate-pulse">
                  Loading modification history...
                </div>
              ) : (
                <>
                  {/* Stop Loss Modifications */}
                  {chain.slOrder && modificationData.SL.length > 0 && (
                    <div className="bg-gray-900/50 rounded-lg p-3 border border-gray-700/50">
                      <div className="flex items-center gap-2 mb-2 pb-2 border-b border-gray-700/50">
                        <Shield className="w-4 h-4 text-red-400" />
                        <span className="text-sm font-medium text-red-400">Stop Loss</span>
                        <span className="text-xs text-gray-500">
                          (${(chain.slOrder.stopPrice || chain.slOrder.price || 0).toFixed(8)})
                        </span>
                        {(() => {
                          const summary = calculateSummaryStats(modificationData.SL);
                          return (
                            <>
                              <span className="px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                                {summary.totalModifications} mod{summary.totalModifications !== 1 ? 's' : ''}
                              </span>
                              <span className={`ml-auto text-xs font-medium ${summary.netDollarImpact >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                Net: {formatDollarImpact(summary.netDollarImpact)}
                              </span>
                            </>
                          );
                        })()}
                      </div>
                      <ModificationRowList
                        events={modificationData.SL}
                        orderType="SL"
                      />
                    </div>
                  )}

                  {/* Take Profit Modifications */}
                  {chain.tpOrders.map((tp) => {
                    const tpType = tp.orderType as ModifiableOrderType;
                    if (!tpType || !['TP1', 'TP2', 'TP3', 'TP4'].includes(tpType)) return null;
                    const events = modificationData[tpType] || [];
                    if (events.length === 0) return null;
                    const summary = calculateSummaryStats(events);
                    return (
                      <div key={tp.orderId} className="bg-gray-900/50 rounded-lg p-3 border border-gray-700/50">
                        <div className="flex items-center gap-2 mb-2 pb-2 border-b border-gray-700/50">
                          <Target className="w-4 h-4 text-cyan-400" />
                          <span className="text-sm font-medium text-cyan-400">{tpType}</span>
                          <span className="text-xs text-gray-500">
                            (${(tp.price || 0).toFixed(8)})
                          </span>
                          <span className="px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                            {summary.totalModifications} mod{summary.totalModifications !== 1 ? 's' : ''}
                          </span>
                          <span className={`ml-auto text-xs font-medium ${summary.netDollarImpact >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            Net: {formatDollarImpact(summary.netDollarImpact)}
                          </span>
                        </div>
                        <ModificationRowList
                          events={events}
                          orderType={tpType}
                        />
                      </div>
                    );
                  })}

                  {/* Empty state if no SL/TP orders or no modifications */}
                  {!chain.slOrder && chain.tpOrders.length === 0 && (
                    <div className="text-sm text-gray-500 py-4 text-center">
                      No SL/TP orders to show modification history for.
                    </div>
                  )}
                  {chain.slOrder && modificationData.SL.length === 0 && chain.tpOrders.every(tp => {
                    const tpType = tp.orderType as ModifiableOrderType;
                    return !tpType || (modificationData[tpType] || []).length === 0;
                  }) && (
                    <div className="text-sm text-gray-500 py-4 text-center">
                      No modifications recorded for this chain.
                    </div>
                  )}
                </>
              )}
            </div>
          )}
        </div>
      )}

      {/* Chain info footer */}
      <div className="pt-3 border-t border-gray-700 grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
        <div>
          <span className="text-gray-500">Created:</span>
          <span className="ml-2 text-gray-300">
            {formatDateTime(chain.createdAt)}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Updated:</span>
          <span className="ml-2 text-gray-300">
            {formatDateTime(chain.updatedAt)}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Total Value:</span>
          <span className="ml-2 text-gray-300 font-mono">
            ${(chain.totalValue ?? 0).toFixed(2)}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Filled Value:</span>
          <span className="ml-2 text-gray-300 font-mono">
            ${(chain.filledValue ?? 0).toFixed(2)}
          </span>
        </div>
      </div>
    </div>
  );
}
