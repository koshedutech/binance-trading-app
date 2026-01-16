import React, { useState } from 'react';
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
} from 'lucide-react';
import { formatDistanceToNow, format } from 'date-fns';
import { OrderChain, ChainOrder, ORDER_TYPE_CONFIG, MODE_DISPLAY_NAMES, OrderTypeSuffix } from './types';

interface ChainCardProps {
  chain: OrderChain;
  compact?: boolean;
}

export default function ChainCard({ chain, compact = false }: ChainCardProps) {
  const [expanded, setExpanded] = useState(false);

  // Get status badge styling
  const getStatusBadge = (status: string) => {
    const configs: Record<string, { color: string; bg: string; label: string }> = {
      active: { color: 'text-green-400', bg: 'bg-green-500/20', label: 'Active' },
      partial: { color: 'text-yellow-400', bg: 'bg-yellow-500/20', label: 'Partial' },
      completed: { color: 'text-blue-400', bg: 'bg-blue-500/20', label: 'Completed' },
      cancelled: { color: 'text-gray-400', bg: 'bg-gray-500/20', label: 'Cancelled' },
    };
    const config = configs[status] || configs.active;
    return (
      <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${config.bg} ${config.color}`}>
        {config.label}
      </span>
    );
  };

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

  // Format price
  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toFixed(2);
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(8);
  };

  // Format quantity
  const formatQty = (qty: number) => {
    return qty.toFixed(4);
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
            {formatQty(order.executedQty)}/{formatQty(order.origQty)}
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

  // Direction icon
  const DirectionIcon = chain.positionSide === 'LONG' ? TrendingUp : TrendingDown;
  const directionColor = chain.positionSide === 'LONG' ? 'text-green-400' : 'text-red-400';

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

      {/* Expanded content */}
      {expanded && (
        <div className="border-t border-gray-700 p-4 space-y-4">
          {/* Chain structure visualization */}
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

            {/* DCA Orders */}
            {chain.dcaOrders.length > 0 && (
              <div className="flex items-center gap-1 ml-4 border-l border-gray-600 pl-4">
                {chain.dcaOrders.map((dca, idx) => {
                  const dcaConfig = dca.orderType ? ORDER_TYPE_CONFIG[dca.orderType] : null;
                  return (
                    <div key={dca.orderId} className="flex items-center gap-1">
                      <div className={`px-3 py-1.5 rounded-lg ${dcaConfig?.bgColor || 'bg-blue-500/20'} border border-blue-500/30`}>
                        <div className="flex items-center gap-2">
                          <Layers className="w-4 h-4 text-blue-400" />
                          <span className="text-blue-400 font-medium">{dca.orderType || 'DCA'}</span>
                        </div>
                        <div className="text-xs text-gray-400 mt-1">
                          {formatPrice(dca.price)}
                        </div>
                      </div>
                      {idx < chain.dcaOrders.length - 1 && <div className="w-2 h-0.5 bg-gray-600" />}
                    </div>
                  );
                })}
              </div>
            )}

            {/* Rebuy */}
            {chain.rebuyOrder && (
              <div className="flex items-center gap-1 ml-4 border-l border-gray-600 pl-4">
                <div className={`px-3 py-1.5 rounded-lg ${ORDER_TYPE_CONFIG.RB.bgColor} border border-purple-500/30`}>
                  <div className="flex items-center gap-2">
                    <Activity className="w-4 h-4 text-purple-400" />
                    <span className="text-purple-400 font-medium">Rebuy</span>
                  </div>
                  <div className="text-xs text-gray-400 mt-1">
                    {formatPrice(chain.rebuyOrder.price)}
                  </div>
                </div>
              </div>
            )}

            {/* Hedge */}
            {chain.hedgeOrder && (
              <div className="flex items-center gap-1 ml-4 border-l border-gray-600 pl-4">
                <div className={`px-3 py-1.5 rounded-lg ${ORDER_TYPE_CONFIG.H.bgColor} border border-yellow-500/30`}>
                  <div className="flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-yellow-400" />
                    <span className="text-yellow-400 font-medium">Hedge</span>
                  </div>
                  <div className="text-xs text-gray-400 mt-1">
                    {formatPrice(chain.hedgeOrder.price)}
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Order details */}
          <div className="space-y-2">
            <h4 className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <Activity className="w-4 h-4" />
              Order Details
            </h4>
            <div className="space-y-1.5">
              {chain.orders.map(renderOrderRow)}
            </div>
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
    </div>
  );
}
