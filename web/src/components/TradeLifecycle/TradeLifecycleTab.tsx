// Story 7.15: Order Chain Tree Structure UI
// Updated to use getOrderChainsWithState API for position state and modification counts
import { useState, useEffect, useMemo, useCallback, useRef } from 'react';
import {
  Layers,
  RefreshCw,
  AlertTriangle,
  BarChart3,
  Activity,
  TrendingUp,
  TrendingDown,
  Target,
  Shield,
} from 'lucide-react';
import { futuresApi, OrderChainWithState, PositionStateInfo } from '../../services/futuresApi';
import { wsService } from '../../services/websocket';
import { fallbackManager } from '../../services/fallbackPollingManager';
import { ConnectionStatus } from '../ConnectionStatus';
import ChainCard from './ChainCard';
import ChainFilters from './ChainFilters';
import {
  OrderChain,
  ChainOrder,
  ChainFilters as FilterType,
  groupOrdersIntoChains,
  parseClientOrderId,
  TradingModeCode,
  PositionState,
} from './types';
import type { WSEvent } from '../../types';

interface TradeLifecycleTabProps {
  autoRefresh?: boolean;
}

const FALLBACK_KEY = 'tradeLifecycleTab';

export default function TradeLifecycleTab({
  autoRefresh = true,
}: TradeLifecycleTabProps) {
  // Ref to prevent concurrent fetch calls (race condition protection)
  const fetchInFlightRef = useRef(false);
  const [chains, setChains] = useState<OrderChain[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<FilterType>({
    mode: 'all',
    status: 'all',
    symbol: 'all',
    side: 'all',
  });

  // Helper: Convert API PositionStateInfo to frontend PositionState
  const mapPositionState = (state: PositionStateInfo): PositionState => ({
    id: state.id,
    chainId: state.chain_id,
    symbol: state.symbol,
    entryOrderId: state.entry_order_id,
    entryClientOrderId: state.entry_client_order_id,
    entrySide: state.entry_side,
    entryPrice: state.entry_price,
    entryQuantity: state.entry_quantity,
    entryValue: state.entry_value,
    entryFees: state.entry_fees,
    entryFilledAt: state.entry_filled_at,
    status: state.status,
    remainingQuantity: state.remaining_quantity,
    realizedPnl: state.realized_pnl,
    createdAt: state.created_at,
    updatedAt: state.updated_at,
    closedAt: state.closed_at,
  });

  // Helper: Convert API OrderChainWithState to frontend OrderChain
  const mapOrderChainWithState = (apiChain: OrderChainWithState): OrderChain => {
    // Transform orders from API format to ChainOrder format
    const chainOrders: ChainOrder[] = apiChain.orders.map(order => {
      const parsed = parseClientOrderId(order.client_order_id);
      return {
        orderId: order.order_id,
        clientOrderId: order.client_order_id,
        symbol: order.symbol,
        side: order.side as 'BUY' | 'SELL',
        positionSide: apiChain.position_side as 'LONG' | 'SHORT' | 'BOTH',
        type: order.type,
        status: order.status,
        price: order.price,
        avgPrice: order.avg_price || 0,
        origQty: order.quantity,
        executedQty: order.executed_qty,
        stopPrice: order.stop_price || 0,
        time: order.time,
        updateTime: order.update_time,
        orderType: parsed.orderType,
        parsed,
      };
    });

    // Parse chain ID for mode code, date, etc.
    const firstParsed = chainOrders.length > 0 ? chainOrders[0].parsed : null;

    // Group orders by type
    const entryOrder = chainOrders.find(o => o.orderType === 'E') || null;
    const tpOrders = chainOrders.filter(o => o.orderType && ['TP1', 'TP2', 'TP3'].includes(o.orderType));
    const slOrder = chainOrders.find(o => o.orderType === 'SL') || null;
    const dcaOrders = chainOrders.filter(o => o.orderType && ['DCA1', 'DCA2', 'DCA3'].includes(o.orderType));
    const rebuyOrder = chainOrders.find(o => o.orderType === 'RB') || null;
    const hedgeOrder = chainOrders.find(o => o.orderType === 'H') || null;
    const hedgeSLOrder = chainOrders.find(o => o.orderType === 'HSL') || null;
    const hedgeTPOrder = chainOrders.find(o => o.orderType === 'HTP') || null;

    return {
      chainId: apiChain.chain_id,
      modeCode: (apiChain.mode_code as TradingModeCode) || null,
      dateStr: firstParsed?.dateStr || null,
      sequence: firstParsed?.sequence || null,
      symbol: apiChain.symbol,
      side: entryOrder?.side as 'BUY' | 'SELL' || null,
      positionSide: (apiChain.position_side as 'LONG' | 'SHORT' | 'BOTH') || null,
      orders: chainOrders,
      entryOrder,
      tpOrders,
      slOrder,
      dcaOrders,
      rebuyOrder,
      hedgeOrder,
      hedgeSLOrder,
      hedgeTPOrder,
      status: apiChain.status as 'active' | 'partial' | 'completed' | 'cancelled',
      totalValue: apiChain.total_value,
      filledValue: apiChain.filled_value,
      createdAt: apiChain.created_at,
      updatedAt: apiChain.updated_at,
      isFallback: false,
      // Story 7.15: Position state and modification counts from new API
      positionState: apiChain.position_state ? mapPositionState(apiChain.position_state) : undefined,
      modificationCounts: apiChain.modification_counts || undefined,
    };
  };

  // Fetch order chains with state (Story 7.15: Use new API endpoint)
  // Uses fetchInFlightRef to prevent race conditions from concurrent calls
  const fetchOrders = useCallback(async () => {
    // Prevent concurrent fetch calls (race condition protection)
    if (fetchInFlightRef.current) {
      return;
    }
    fetchInFlightRef.current = true;

    try {
      // Story 7.15: Use new getOrderChainsWithState API
      const response = await futuresApi.getOrderChainsWithState();

      if (!response || !response.chains) {
        // Fallback to old API if new endpoint fails or returns empty
        const fallbackResponse = await futuresApi.getAllOrders();
        if (!fallbackResponse) {
          setChains([]);
          setError(null);
          return;
        }

        // Original transformation logic for fallback
        const chainOrders: ChainOrder[] = [];

        // Define type for legacy API order format (regular orders)
        interface LegacyRegularOrder {
          orderId: number;
          clientOrderId?: string;
          symbol: string;
          side: 'BUY' | 'SELL';
          positionSide?: 'LONG' | 'SHORT' | 'BOTH';
          type: string;
          status: string;
          price?: string | number;
          avgPrice?: string | number;
          origQty?: string | number;
          executedQty?: string | number;
          stopPrice?: string | number;
          time?: number;
          updateTime?: number;
        }

        // Define type for legacy API algo order format
        interface LegacyAlgoOrder {
          algoId: number;
          clientAlgoId?: string;
          symbol: string;
          side: string;
          positionSide?: string;
          orderType?: string;
          algoType?: string;
          algoStatus?: string;
          price?: string | number;
          quantity?: string | number;
          executedQty?: string | number;
          triggerPrice?: string | number;
          createTime?: number;
          updateTime?: number;
        }

        if (fallbackResponse.regular_orders && Array.isArray(fallbackResponse.regular_orders)) {
          (fallbackResponse.regular_orders as LegacyRegularOrder[])
            .filter((order) => order.clientOrderId)
            .forEach((order) => {
              const parsed = parseClientOrderId(order.clientOrderId!);
              chainOrders.push({
                orderId: order.orderId,
                clientOrderId: order.clientOrderId!,
                symbol: order.symbol,
                side: order.side,
                positionSide: order.positionSide || 'BOTH',
                type: order.type,
                status: order.status,
                price: parseFloat(String(order.price || 0)) || 0,
                avgPrice: parseFloat(String(order.avgPrice || 0)) || 0,
                origQty: parseFloat(String(order.origQty || 0)) || 0,
                executedQty: parseFloat(String(order.executedQty || 0)) || 0,
                stopPrice: parseFloat(String(order.stopPrice || 0)) || 0,
                time: order.time || Date.now(),
                updateTime: order.updateTime || Date.now(),
                orderType: parsed.orderType,
                parsed,
              });
            });
        }

        if (fallbackResponse.algo_orders && Array.isArray(fallbackResponse.algo_orders)) {
          (fallbackResponse.algo_orders as LegacyAlgoOrder[])
            .filter((order) => order.clientAlgoId)
            .forEach((order) => {
              const parsed = parseClientOrderId(order.clientAlgoId!);
              chainOrders.push({
                orderId: order.algoId || 0,
                clientOrderId: order.clientAlgoId!,
                symbol: order.symbol,
                side: (order.side === 'BUY' || order.side === 'SELL' ? order.side : 'BUY') as 'BUY' | 'SELL',
                positionSide: ((order.positionSide === 'LONG' || order.positionSide === 'SHORT' || order.positionSide === 'BOTH') ? order.positionSide : 'BOTH') as 'LONG' | 'SHORT' | 'BOTH',
                type: order.orderType || order.algoType || 'UNKNOWN',
                status: order.algoStatus || 'NEW',
                price: parseFloat(String(order.price || 0)) || 0,
                avgPrice: 0,
                origQty: parseFloat(String(order.quantity || 0)) || 0,
                executedQty: parseFloat(String(order.executedQty || 0)) || 0,
                stopPrice: parseFloat(String(order.triggerPrice || 0)) || 0,
                time: order.createTime || Date.now(),
                updateTime: order.updateTime || Date.now(),
                orderType: parsed.orderType,
                parsed,
              });
            });
        }

        const grouped = groupOrdersIntoChains(chainOrders);
        setChains(grouped);
        setError(null);
        return;
      }

      // Story 7.15: Map new API response to OrderChain format
      const mappedChains = response.chains.map(mapOrderChainWithState);
      setChains(mappedChains);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch orders:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch orders');
    } finally {
      setLoading(false);
      fetchInFlightRef.current = false;
    }
  }, []); // No dependencies - uses only stable setters and external API

  // Initial fetch
  useEffect(() => {
    fetchOrders();
  }, [fetchOrders]);

  // WebSocket subscription for real-time chain/order updates
  // Uses centralized fallbackManager for disconnect recovery (Story 12.9 pattern)
  useEffect(() => {
    if (!autoRefresh) return;

    const handleChainUpdate = (event: WSEvent) => {
      // On chain update, refresh the full chains list
      // This ensures we have consistent state with all related orders
      fetchOrders();
    };

    const handleOrderUpdate = (event: WSEvent) => {
      // On order update, refresh chains as order state affects chain status
      fetchOrders();
    };

    const handleConnect = () => {
      // Refresh data on reconnect to sync any missed events
      fetchOrders();
    };

    // Subscribe to WebSocket events
    wsService.subscribe('CHAIN_UPDATE', handleChainUpdate);
    wsService.subscribe('ORDER_UPDATE', handleOrderUpdate);
    wsService.onConnect(handleConnect);

    // Register with fallbackManager for centralized fallback polling
    fallbackManager.registerFetchFunction(FALLBACK_KEY, fetchOrders);

    return () => {
      wsService.unsubscribe('CHAIN_UPDATE', handleChainUpdate);
      wsService.unsubscribe('ORDER_UPDATE', handleOrderUpdate);
      wsService.offConnect(handleConnect);
      fallbackManager.unregisterFetchFunction(FALLBACK_KEY);
    };
  }, [autoRefresh, fetchOrders]);

  // Get unique symbols for filter
  const symbols = useMemo(() => {
    const symbolSet = new Set(chains.map(c => c.symbol).filter(Boolean) as string[]);
    return Array.from(symbolSet).sort();
  }, [chains]);

  // Apply filters
  const filteredChains = useMemo(() => {
    return chains.filter((chain) => {
      if (filters.mode !== 'all' && chain.modeCode !== filters.mode) return false;
      if (filters.status !== 'all' && chain.status !== filters.status) return false;
      if (filters.symbol !== 'all' && chain.symbol !== filters.symbol) return false;
      if (filters.side !== 'all' && chain.positionSide !== filters.side) return false;
      return true;
    });
  }, [chains, filters]);

  // Calculate summary stats
  const stats = useMemo(() => {
    const totalChains = chains.length;
    const activeChains = chains.filter(c => c.status === 'active').length;
    const partialChains = chains.filter(c => c.status === 'partial').length;
    const completedChains = chains.filter(c => c.status === 'completed').length;
    const totalOrders = chains.reduce((sum, c) => sum + c.orders.length, 0);
    const longChains = chains.filter(c => c.positionSide === 'LONG').length;
    const shortChains = chains.filter(c => c.positionSide === 'SHORT').length;
    const fallbackChains = chains.filter(c => c.isFallback).length;

    // Count by mode
    const byMode: Record<TradingModeCode, number> = { ULT: 0, SCA: 0, SWI: 0, POS: 0 };
    chains.forEach((c) => {
      if (c.modeCode && byMode[c.modeCode] !== undefined) {
        byMode[c.modeCode]++;
      }
    });

    return {
      totalChains,
      activeChains,
      partialChains,
      completedChains,
      totalOrders,
      longChains,
      shortChains,
      fallbackChains,
      byMode,
    };
  }, [chains]);

  // Reset filters
  const resetFilters = () => {
    setFilters({
      mode: 'all',
      status: 'all',
      symbol: 'all',
      side: 'all',
    });
  };

  // Loading state
  if (loading && chains.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <div className="flex items-center justify-center text-gray-400">
          <RefreshCw className="w-5 h-5 animate-spin mr-2" />
          Loading order chains...
        </div>
      </div>
    );
  }

  // Error state
  if (error && chains.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-red-500/30">
        <div className="flex items-center gap-2 text-red-400">
          <AlertTriangle className="w-5 h-5" />
          <span>{error}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700">
      {/* Header */}
      <div className="p-4 border-b border-gray-700">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <Layers className="w-5 h-5 text-purple-400" />
            <h3 className="text-lg font-semibold text-gray-200">Order Chains</h3>
            <ConnectionStatus />
            <span className="text-sm text-gray-500">
              ({filteredChains.length} chain{filteredChains.length !== 1 ? 's' : ''})
            </span>
          </div>

          <button
            onClick={() => { setLoading(true); fetchOrders(); }}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>

        {/* Error banner - shows even when we have chains (refresh failure) */}
        {error && chains.length > 0 && (
          <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center justify-between">
            <div className="flex items-center gap-2 text-red-400">
              <AlertTriangle className="w-4 h-4" />
              <span className="text-sm">Refresh failed: {error}</span>
            </div>
            <button
              onClick={() => setError(null)}
              className="text-red-400 hover:text-red-300 text-sm"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Stats summary */}
        <div className="grid grid-cols-4 md:grid-cols-8 gap-3 mb-4">
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="text-lg font-semibold text-gray-200">{stats.totalChains}</div>
            <div className="text-xs text-gray-500">Total</div>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="text-lg font-semibold text-green-400">{stats.activeChains}</div>
            <div className="text-xs text-gray-500">Active</div>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="text-lg font-semibold text-yellow-400">{stats.partialChains}</div>
            <div className="text-xs text-gray-500">Partial</div>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="text-lg font-semibold text-blue-400">{stats.completedChains}</div>
            <div className="text-xs text-gray-500">Complete</div>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="text-lg font-semibold text-gray-200">{stats.totalOrders}</div>
            <div className="text-xs text-gray-500">Orders</div>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="flex items-center justify-center gap-1">
              <TrendingUp className="w-3.5 h-3.5 text-green-400" />
              <span className="text-lg font-semibold text-green-400">{stats.longChains}</span>
            </div>
            <div className="text-xs text-gray-500">Long</div>
          </div>
          <div className="bg-gray-900/50 rounded-lg p-2 text-center">
            <div className="flex items-center justify-center gap-1">
              <TrendingDown className="w-3.5 h-3.5 text-red-400" />
              <span className="text-lg font-semibold text-red-400">{stats.shortChains}</span>
            </div>
            <div className="text-xs text-gray-500">Short</div>
          </div>
          {stats.fallbackChains > 0 && (
            <div className="bg-gray-900/50 rounded-lg p-2 text-center">
              <div className="text-lg font-semibold text-orange-400">{stats.fallbackChains}</div>
              <div className="text-xs text-gray-500">Fallback</div>
            </div>
          )}
        </div>

        {/* Filters */}
        <ChainFilters
          filters={filters}
          onFilterChange={setFilters}
          symbols={symbols}
          onReset={resetFilters}
        />
      </div>

      {/* Chains list */}
      <div className="p-4 space-y-3 max-h-[600px] overflow-y-auto">
        {filteredChains.length === 0 ? (
          <div className="text-center py-8">
            <Layers className="w-12 h-12 mx-auto mb-3 text-gray-600" />
            <p className="text-gray-400">No order chains found</p>
            <p className="text-sm text-gray-500 mt-1">
              {chains.length === 0
                ? 'Order chains will appear when orders are placed with structured client order IDs'
                : 'Try adjusting your filters'}
            </p>
          </div>
        ) : (
          filteredChains.map((chain) => (
            <ChainCard key={chain.chainId} chain={chain} />
          ))
        )}
      </div>

      {/* Legend */}
      <div className="p-4 border-t border-gray-700 bg-gray-900/30">
        <div className="flex items-center gap-6 text-xs text-gray-500">
          <span className="font-medium">Order Types:</span>
          <div className="flex items-center gap-1">
            <TrendingUp className="w-3 h-3 text-green-400" />
            <span>Entry</span>
          </div>
          <div className="flex items-center gap-1">
            <Target className="w-3 h-3 text-cyan-400" />
            <span>Take Profit</span>
          </div>
          <div className="flex items-center gap-1">
            <Shield className="w-3 h-3 text-red-400" />
            <span>Stop Loss</span>
          </div>
          <div className="flex items-center gap-1">
            <Layers className="w-3 h-3 text-blue-400" />
            <span>DCA</span>
          </div>
          <div className="flex items-center gap-1">
            <Activity className="w-3 h-3 text-purple-400" />
            <span>Rebuy</span>
          </div>
          <div className="flex items-center gap-1">
            <BarChart3 className="w-3 h-3 text-yellow-400" />
            <span>Hedge</span>
          </div>
        </div>
      </div>
    </div>
  );
}
