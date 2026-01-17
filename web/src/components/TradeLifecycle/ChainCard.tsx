// Story 7.15: Order Chain Tree Structure UI
// Hierarchical display: Entry -> Position -> [TP/SL]
import React, { useState, useEffect, useCallback } from 'react';
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
} from 'lucide-react';
import { formatDistanceToNow, format } from 'date-fns';
import { OrderChain, ChainOrder, ORDER_TYPE_CONFIG, MODE_DISPLAY_NAMES, OrderTypeSuffix, PositionState } from './types';
import OrderTreeNode, { buildEntryFromPositionState, TreeNodeType } from './OrderTreeNode';
import { ModificationTree } from './ModificationHistory';
import type { ModificationEvent, ModifiableOrderType } from './ModificationHistory/types';
import { futuresApi } from '../../services/futuresApi';

interface ChainCardProps {
  chain: OrderChain;
  compact?: boolean;
  useTreeView?: boolean; // New prop to toggle tree view (default true)
}

export default function ChainCard({ chain, compact = false, useTreeView = true }: ChainCardProps) {
  const [expanded, setExpanded] = useState(false);
  const [showLegacyView, setShowLegacyView] = useState(false); // Toggle to show old horizontal view
  const [modificationData, setModificationData] = useState<Record<ModifiableOrderType, ModificationEvent[]>>({
    SL: [],
    TP1: [],
    TP2: [],
    TP3: [],
    TP4: [],
  });
  const [modificationLoading, setModificationLoading] = useState(false);
  const [modificationsLoaded, setModificationsLoaded] = useState(false);

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

  // Format price
  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toFixed(2);
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(8);
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

          {/* Position state indicator */}
          {chain.positionState && (
            <span className={`px-1.5 py-0.5 rounded text-xs ${
              chain.positionState.status === 'ACTIVE' ? 'bg-green-500/20 text-green-400' :
              chain.positionState.status === 'PARTIAL' ? 'bg-yellow-500/20 text-yellow-400' :
              'bg-blue-500/20 text-blue-400'
            }`}>
              {chain.positionState.status}
            </span>
          )}
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

          {/* Time */}
          <div className="flex items-center gap-1.5 text-gray-500 text-xs">
            <Clock className="w-3 h-3" />
            <span>{formatDistanceToNow(chain.createdAt, { addSuffix: true })}</span>
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

            {/* Position State - Child of entry (depth 1) */}
            {chain.positionState && (
              <div className="mt-1">
                <OrderTreeNode
                  type="POSITION"
                  positionState={chain.positionState}
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

          {/* Chain info footer */}
          <div className="pt-3 border-t border-gray-700 grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-gray-500">Created:</span>
              <span className="ml-2 text-gray-300">
                {format(chain.createdAt, 'MMM dd, HH:mm:ss')}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Updated:</span>
              <span className="ml-2 text-gray-300">
                {format(chain.updatedAt, 'MMM dd, HH:mm:ss')}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Total Value:</span>
              <span className="ml-2 text-gray-300 font-mono">
                ${chain.totalValue.toFixed(2)}
              </span>
            </div>
            <div>
              <span className="text-gray-500">Filled Value:</span>
              <span className="ml-2 text-gray-300 font-mono">
                ${chain.filledValue.toFixed(2)}
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
  onToggleToTree: () => void;
  showToggle: boolean;
}

function LegacyChainView({
  chain,
  modificationData,
  modificationLoading,
  formatPrice,
  onToggleToTree,
  showToggle,
}: LegacyChainViewProps) {
  const [showModificationHistory, setShowModificationHistory] = useState(false);

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
            {order.executedQty.toFixed(4)}/{order.origQty.toFixed(4)}
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
                  {chain.slOrder && (
                    <ModificationTree
                      chainId={chain.chainId}
                      orderType="SL"
                      currentPrice={chain.slOrder.stopPrice || chain.slOrder.price}
                      events={modificationData.SL}
                      positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                      compact={true}
                    />
                  )}

                  {/* Take Profit Modifications */}
                  {chain.tpOrders.map((tp) => {
                    const tpType = tp.orderType as ModifiableOrderType;
                    if (!tpType || !['TP1', 'TP2', 'TP3', 'TP4'].includes(tpType)) return null;
                    return (
                      <ModificationTree
                        key={tp.orderId}
                        chainId={chain.chainId}
                        orderType={tpType}
                        currentPrice={tp.price}
                        events={modificationData[tpType] || []}
                        positionSide={chain.positionSide === 'LONG' ? 'LONG' : 'SHORT'}
                        compact={true}
                      />
                    );
                  })}

                  {/* Empty state if no SL/TP orders */}
                  {!chain.slOrder && chain.tpOrders.length === 0 && (
                    <div className="text-sm text-gray-500 py-4 text-center">
                      No SL/TP orders to show modification history for.
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
            {format(chain.createdAt, 'MMM dd, HH:mm:ss')}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Updated:</span>
          <span className="ml-2 text-gray-300">
            {format(chain.updatedAt, 'MMM dd, HH:mm:ss')}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Total Value:</span>
          <span className="ml-2 text-gray-300 font-mono">
            ${chain.totalValue.toFixed(2)}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Filled Value:</span>
          <span className="ml-2 text-gray-300 font-mono">
            ${chain.filledValue.toFixed(2)}
          </span>
        </div>
      </div>
    </div>
  );
}
