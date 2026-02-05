// Story 7.15: Order Chain Tree Structure UI
// Updated to use getOrderChainsWithState API for position state and modification counts
// Enhanced: Added date range filtering with historical orders API
// Story 14.15: Added Trading Toggle for ON/OFF control
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
  History,
  ChevronDown,
  ChevronRight,
  GitBranch,
  Briefcase,
  CloudCog,
} from 'lucide-react';
import { futuresApi, OrderChainWithState, PositionStateInfo, HistoricalOrderChain } from '../../services/futuresApi';
import { wsService } from '../../services/websocket';
import { fallbackManager } from '../../services/fallbackPollingManager';
import { ConnectionStatus } from '../ConnectionStatus';
import ChainCard from './ChainCard';
import ChainFilters from './ChainFilters';
import EntryDecisionEngineCard from './EntryDecisionEngineCard';
import { VolumeImbalanceCard } from '../EntryDecisionEngine';
import { StrategyFirstView } from '../EntryDecision';
import { useVolumeImbalancePatterns } from '../../hooks/useStrategyHierarchy';
import { TradingToggle } from '../TradingControl';
import { useTradingState } from '../../hooks/useTradingState';
import { CoinProfilerCard } from '../CoinProfiler';
import type { CoinDataUpdate, PositionCreatedEvent } from '../../hooks/useCoinProfiler';
import {
  OrderChain,
  ChainOrder,
  ChainFilters as FilterType,
  groupOrdersIntoChains,
  parseClientOrderId,
  TradingModeCode,
  PositionState,
  OrderTypeSuffix,
  ORDER_TYPE_CONFIG,
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

  // Section expansion states (must be declared before any early returns)
  // All collapsed by default - will auto-expand when data is received
  const [tradeCycleExpanded, setTradeCycleExpanded] = useState(true);
  const [ordersExpanded, setOrdersExpanded] = useState(false);
  const [positionsExpanded, setPositionsExpanded] = useState(false);

  // Track if auto-expansion has already occurred (to respect user's manual collapse)
  const ordersAutoExpandedRef = useRef(false);
  const positionsAutoExpandedRef = useRef(false);

  // State for sync operation
  const [syncing, setSyncing] = useState(false);
  const [syncResult, setSyncResult] = useState<{ success: boolean; message: string } | null>(null);

  // Volume Imbalance patterns for Entry Decision Engine
  const { patterns: volumeImbalancePatterns, isLoading: patternsLoading } = useVolumeImbalancePatterns();

  // Trading state - controls whether new entries are allowed
  const { data: tradingState } = useTradingState();
  const tradingEnabled = tradingState?.enabled ?? true;

  // Real-time price map from coin profiler subscription (symbol -> price)
  const [livePrices, setLivePrices] = useState<Map<string, number>>(new Map());

  // Subscribe to real-time price updates from coin profiler
  useEffect(() => {
    const handleCoinUpdate = (event: WSEvent) => {
      const update = event.data as CoinDataUpdate;
      if (update && update.symbol && update.price > 0) {
        setLivePrices(prev => {
          const newMap = new Map(prev);
          newMap.set(update.symbol, update.price);
          return newMap;
        });
      }
    };

    wsService.subscribe('COIN_DATA_UPDATE', handleCoinUpdate);

    return () => {
      wsService.unsubscribe('COIN_DATA_UPDATE', handleCoinUpdate);
    };
  }, []);

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

      // Determine orderType: prefer backend's order_type, fallback to parsed, then detect from Binance type
      let orderType: OrderTypeSuffix | null = null;
      if (order.order_type && ORDER_TYPE_CONFIG[order.order_type as OrderTypeSuffix]) {
        orderType = order.order_type as OrderTypeSuffix;
      } else if (parsed.orderType) {
        orderType = parsed.orderType;
      } else {
        // Fallback: detect from Binance order type for orders without proper client order ID
        const binanceType = order.type?.toUpperCase() || '';
        if (binanceType === 'STOP_MARKET' || binanceType === 'STOP') {
          orderType = 'SL';
        } else if (binanceType === 'TAKE_PROFIT_MARKET' || binanceType === 'TAKE_PROFIT') {
          orderType = 'TP1'; // Default to TP1 if we can't determine exact level
        } else if (binanceType === 'MARKET' || binanceType === 'LIMIT') {
          orderType = 'E'; // Entry order
        }
      }

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
        orderType,
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
      // Story 11.40: Position analytics for detailed position section
      positionAnalytics: apiChain.position_analytics ? {
        stage: apiChain.position_analytics.stage,
        stage_entry_time: apiChain.position_analytics.stage_entry_time,
        current_price: apiChain.position_analytics.current_price,
        breakeven_price: apiChain.position_analytics.breakeven_price,
        tp1_price: apiChain.position_analytics.tp1_price,
        tp2_price: apiChain.position_analytics.tp2_price,
        tp3_price: apiChain.position_analytics.tp3_price,
        stop_loss: apiChain.position_analytics.stop_loss,
        efficiency: apiChain.position_analytics.efficiency,
        decision_mode: apiChain.position_analytics.decision_mode,
        classic_scores: apiChain.position_analytics.classic_scores,
        new_engine_scores: apiChain.position_analytics.new_engine_scores,
        unrealized_pnl: apiChain.position_analytics.unrealized_pnl,
      } : undefined,
      // Entry decision context from pattern detection
      entryContext: apiChain.entry_context || undefined,
    };
  };

  // Helper: Convert historical API response to frontend OrderChain
  const mapHistoricalOrderChain = (histChain: HistoricalOrderChain): OrderChain => {
    // Historical chains from DB don't include individual orders
    // They have summary data about the chain
    const parsed = parseClientOrderId(histChain.chainId + '-E');

    return {
      chainId: histChain.chainId,
      modeCode: (histChain.modeCode as TradingModeCode) || null,
      dateStr: parsed?.dateStr || null,
      sequence: parsed?.sequence || null,
      symbol: histChain.symbol,
      side: histChain.side as 'BUY' | 'SELL' || null,
      positionSide: histChain.side === 'BUY' ? 'LONG' : 'SHORT' as 'LONG' | 'SHORT' | 'BOTH',
      orders: [], // Historical chains don't have order details from Binance
      entryOrder: null,
      tpOrders: [],
      slOrder: null,
      dcaOrders: [],
      rebuyOrder: null,
      hedgeOrder: null,
      hedgeSLOrder: null,
      hedgeTPOrder: null,
      // Story 7.21: Case-insensitive status mapping, default to 'completed' for historical chains
      status: (() => {
        const normalizedStatus = (histChain.status || '').toUpperCase();
        if (normalizedStatus === 'CLOSED') return 'completed';
        if (normalizedStatus === 'CANCELLED') return 'cancelled';
        if (normalizedStatus === 'PARTIAL') return 'partial';
        if (normalizedStatus === 'ACTIVE') return 'active';
        return 'completed'; // Default to completed for unknown historical statuses
      })() as 'active' | 'partial' | 'completed' | 'cancelled',
      totalValue: histChain.entryPrice * histChain.entryQuantity,
      filledValue: histChain.entryPrice * histChain.entryQuantity,
      pnl: histChain.realizedPnl,
      // Story 7.21 fix: Handle invalid/missing date strings to prevent NaN timestamps
      createdAt: histChain.createdAt ? new Date(histChain.createdAt).getTime() || Date.now() : Date.now(),
      updatedAt: histChain.updatedAt ? new Date(histChain.updatedAt).getTime() || Date.now() : Date.now(),
      isFallback: histChain.chainId.includes('FALLBACK'),
      positionState: {
        id: 0,
        chainId: histChain.chainId,
        symbol: histChain.symbol,
        entryOrderId: 0,
        entryClientOrderId: histChain.chainId + '-E',
        entrySide: histChain.side as 'BUY' | 'SELL',
        entryPrice: histChain.entryPrice,
        entryQuantity: histChain.entryQuantity,
        entryValue: histChain.entryPrice * histChain.entryQuantity,
        entryFees: histChain.totalFees,
        entryFilledAt: histChain.createdAt,
        // Story 7.21 fix: Map historical status to valid PositionState status
        status: (() => {
          const normalizedStatus = (histChain.status || '').toUpperCase();
          if (normalizedStatus === 'ACTIVE') return 'ACTIVE';
          if (normalizedStatus === 'PARTIAL') return 'PARTIAL';
          return 'CLOSED'; // CLOSED, CANCELLED, and unknown statuses map to CLOSED
        })() as 'ACTIVE' | 'PARTIAL' | 'CLOSED',
        remainingQuantity: histChain.remainingQuantity,
        realizedPnl: histChain.realizedPnl,
        createdAt: histChain.createdAt,
        updatedAt: histChain.updatedAt,
        closedAt: histChain.closedAt,
      },
      modificationCounts: {
        SL: histChain.slModificationCount || 0,
        TP1: histChain.tpModificationCount || 0,
      },
    };
  };

  // Track if we're in historical mode (date filter active)
  const isHistoricalMode = !!(filters.dateFrom || filters.dateTo);

  // Fetch order chains with state (Story 7.15: Use new API endpoint)
  // Uses fetchInFlightRef to prevent race conditions from concurrent calls
  // Enhanced: Uses historical API when date filters are active
  const fetchOrders = useCallback(async () => {
    // Prevent concurrent fetch calls (race condition protection)
    if (fetchInFlightRef.current) {
      return;
    }
    fetchInFlightRef.current = true;

    try {
      // If date filters are active, use historical API (database query)
      if (filters.dateFrom || filters.dateTo) {
        const histResponse = await futuresApi.getHistoricalOrderChains({
          symbol: filters.symbol !== 'all' ? filters.symbol : undefined,
          mode: filters.mode !== 'all' ? filters.mode : undefined,
          status: filters.status !== 'all' ? filters.status as 'active' | 'partial' | 'closed' | 'cancelled' : undefined,
          dateFrom: filters.dateFrom,
          dateTo: filters.dateTo,
          limit: 200,
        });

        if (histResponse && histResponse.chains) {
          const mappedChains = histResponse.chains.map(mapHistoricalOrderChain);
          setChains(mappedChains);
          setError(null);
          return;
        }
      }

      // Story 7.15: Use new getOrderChainsWithState API (for active orders from Binance)
      const response = await futuresApi.getOrderChainsWithState();

      // Story 7.21: Fetch and merge recent historical orders (last 7 days) with active orders
      // This ensures completed trades are always visible alongside active ones
      let historicalChains: OrderChain[] = [];
      try {
        const sevenDaysAgo = new Date();
        sevenDaysAgo.setDate(sevenDaysAgo.getDate() - 7);
        const histResponse = await futuresApi.getHistoricalOrderChains({
          symbol: filters.symbol !== 'all' ? filters.symbol : undefined,
          mode: filters.mode !== 'all' ? filters.mode : undefined,
          status: filters.status !== 'all' ? filters.status as 'active' | 'partial' | 'closed' | 'cancelled' : undefined,
          dateFrom: sevenDaysAgo.toISOString().split('T')[0],
          limit: 50,
        });
        if (histResponse && histResponse.chains) {
          historicalChains = histResponse.chains.map(mapHistoricalOrderChain);
        }
      } catch (histErr) {
        console.warn('Failed to fetch historical orders, continuing with active only:', histErr);
        // Continue with active orders only - historical fetch is non-blocking
      }

      if (!response || !response.chains) {
        // Fallback to old API if new endpoint fails or returns empty
        const fallbackResponse = await futuresApi.getAllOrders();
        if (!fallbackResponse) {
          // Story 7.21: Even if active orders fail, show historical orders if available
          if (historicalChains.length > 0) {
            setChains(historicalChains);
            setError(null);
            return;
          }
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
        // Story 7.21: Merge fallback active orders with historical orders
        const activeChainIds = new Set(grouped.map(c => c.chainId));
        const mergedChains = [
          ...grouped,
          ...historicalChains.filter(h => !activeChainIds.has(h.chainId))
        ];
        setChains(mergedChains);
        setError(null);
        return;
      }

      // Story 7.15: Map new API response to OrderChain format
      const activeChains = response.chains.map(mapOrderChainWithState);

      // Story 7.21: Merge active orders with historical orders (deduplicate by chainId)
      // Active orders take priority over historical (they have more detail)
      const activeChainIds = new Set(activeChains.map(c => c.chainId));
      const mergedChains = [
        ...activeChains,
        ...historicalChains.filter(h => !activeChainIds.has(h.chainId))
      ];

      setChains(mergedChains);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch orders:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch orders');
    } finally {
      setLoading(false);
      fetchInFlightRef.current = false;
    }
  }, [filters.dateFrom, filters.dateTo, filters.symbol, filters.mode, filters.status]); // Include filters for historical API

  // Fetch when filters change (including date filters)
  useEffect(() => {
    fetchOrders();
  }, [fetchOrders]);

  // Sync order state with Binance (reconciles stale orders)
  const handleSyncOrderState = useCallback(async () => {
    setSyncing(true);
    setSyncResult(null);
    try {
      const result = await futuresApi.syncOrderState();
      setSyncResult({ success: result.success, message: result.message });
      console.log('[TradeLifecycle] Sync complete:', result);
      // Refresh orders after sync
      await fetchOrders();
    } catch (err) {
      console.error('Failed to sync order state:', err);
      setSyncResult({ success: false, message: err instanceof Error ? err.message : 'Sync failed' });
    } finally {
      setSyncing(false);
      // Clear sync result after 5 seconds
      setTimeout(() => setSyncResult(null), 5000);
    }
  }, [fetchOrders]);

  // WebSocket subscription for real-time chain/order updates
  // Story 14.x: Direct state updates from WebSocket instead of API polling
  useEffect(() => {
    if (!autoRefresh) return;

    const handleChainUpdate = (event: WSEvent) => {
      // Direct chain update from WebSocket - update state immediately
      const chainData = event.data?.chain;
      if (!chainData) {
        // Fallback to API if data is missing
        fetchOrders();
        return;
      }

      setChains(prevChains => {
        // Find and update the specific chain
        const chainId = chainData.chain_id || chainData.chainId;
        const existingIdx = prevChains.findIndex(c => c.chainId === chainId);

        if (existingIdx >= 0) {
          // Update existing chain
          const updated = [...prevChains];
          const existingChain = updated[existingIdx];
          updated[existingIdx] = {
            ...existingChain,
            status: chainData.status || existingChain.status,
            updatedAt: chainData.updated_at || Date.now(),
            // Update position state if provided
            positionState: chainData.position_state
              ? mapPositionState(chainData.position_state)
              : existingChain.positionState,
          };
          return updated;
        } else {
          // New chain - do a full refresh to get all details
          fetchOrders();
          return prevChains;
        }
      });
    };

    const handleOrderUpdate = (event: WSEvent) => {
      // Direct order update from WebSocket
      const orderData = event.data;
      if (!orderData || !orderData.client_order_id) {
        // Fallback to API if data is missing
        fetchOrders();
        return;
      }

      // Parse chain ID from client order ID (format: MODE-YYMMDD-SEQ-TYPE)
      const parts = orderData.client_order_id?.split('-') || [];
      const chainId = parts.length >= 3 ? `${parts[0]}-${parts[1]}-${parts[2]}` : null;

      if (!chainId) {
        fetchOrders();
        return;
      }

      setChains(prevChains => {
        const chainIdx = prevChains.findIndex(c => c.chainId === chainId);
        if (chainIdx < 0) return prevChains;

        const updated = [...prevChains];
        const chain = { ...updated[chainIdx] };

        // Update order status in the chain
        const orderType = parts.length >= 4 ? parts[3] : null;
        const newStatus = orderData.status || orderData.order_status;

        if (orderType === 'E' && chain.entryOrder) {
          chain.entryOrder = { ...chain.entryOrder, status: newStatus };
        } else if (orderType === 'SL' && chain.slOrder) {
          chain.slOrder = { ...chain.slOrder, status: newStatus };
        } else if (orderType && orderType.startsWith('TP')) {
          const tpIdx = chain.tpOrders.findIndex(tp => tp.orderType === orderType);
          if (tpIdx >= 0) {
            chain.tpOrders = [...chain.tpOrders];
            chain.tpOrders[tpIdx] = { ...chain.tpOrders[tpIdx], status: newStatus };
          }
        }

        // Update chain status based on order fill
        if (newStatus === 'FILLED' && (orderType === 'SL' || orderType?.startsWith('TP'))) {
          // Exit order filled - position may be closed
          chain.status = 'completed';
        }

        chain.updatedAt = Date.now();
        updated[chainIdx] = chain;
        return updated;
      });
    };

    const handlePositionUpdate = (event: WSEvent) => {
      // Direct position update from WebSocket
      const positions = event.data?.positions || [];

      if (!Array.isArray(positions) || positions.length === 0) {
        return; // No data to update
      }

      setChains(prevChains => {
        let updated = [...prevChains];
        let hasChanges = false;

        for (const pos of positions) {
          const symbol = pos.symbol;
          const posAmt = parseFloat(pos.position_amount || pos.positionAmt || '0');

          // Find chains for this symbol
          for (let i = 0; i < updated.length; i++) {
            if (updated[i].symbol === symbol && updated[i].positionState) {
              hasChanges = true;

              // If position amount is 0, mark as closed
              if (posAmt === 0) {
                updated[i] = {
                  ...updated[i],
                  status: 'completed',
                  positionState: {
                    ...updated[i].positionState!,
                    status: 'CLOSED',
                    remainingQuantity: 0,
                    closedAt: new Date().toISOString(),
                  },
                  updatedAt: Date.now(),
                };
              } else {
                // Update unrealized PnL and remaining quantity
                updated[i] = {
                  ...updated[i],
                  positionState: {
                    ...updated[i].positionState!,
                    remainingQuantity: Math.abs(posAmt),
                    realizedPnl: parseFloat(pos.unrealized_pnl || pos.unRealizedProfit || '0'),
                  },
                  updatedAt: Date.now(),
                };
              }
            }
          }
        }

        return hasChanges ? updated : prevChains;
      });
    };

    const handlePnlUpdate = (event: WSEvent) => {
      // PnL update typically indicates position close
      const pnlData = event.data?.pnl || event.data;
      if (!pnlData) return;

      const chainId = pnlData.chain_id || pnlData.chainId;
      if (!chainId) {
        // If no chain ID, do a full refresh
        fetchOrders();
        return;
      }

      setChains(prevChains => {
        const chainIdx = prevChains.findIndex(c => c.chainId === chainId);
        if (chainIdx < 0) return prevChains;

        const updated = [...prevChains];
        updated[chainIdx] = {
          ...updated[chainIdx],
          status: 'completed',
          pnl: pnlData.realized_pnl || pnlData.realizedPnl || 0,
          positionState: updated[chainIdx].positionState ? {
            ...updated[chainIdx].positionState!,
            status: 'CLOSED',
            realizedPnl: pnlData.realized_pnl || pnlData.realizedPnl || 0,
            closedAt: new Date().toISOString(),
          } : undefined,
          updatedAt: Date.now(),
        };
        return updated;
      });
    };

    const handleTradeUpdate = (event: WSEvent) => {
      // TRADE_UPDATE event from backend indicates an order was filled
      // This is critical for real-time updates when SL/TP orders are triggered
      const tradeData = event.data;
      if (!tradeData) return;

      console.log('[TradeLifecycle] Received TRADE_UPDATE - refreshing state', {
        symbol: tradeData.symbol,
        orderId: tradeData.order_id || tradeData.orderId,
        executionType: tradeData.execution_type || tradeData.executionType,
      });

      // Refresh to get updated order states
      fetchOrders();
    };

    const handleConnect = () => {
      // Refresh data on reconnect to sync any missed events
      fetchOrders();
    };

    const handleOrderSync = (event: WSEvent) => {
      // ORDER_SYNC event from backend indicates state reconciliation
      // This happens when the backend reconnects to Binance and syncs state
      const data = event.data;
      if (!data || data.type !== 'ORDER_SYNC') return;

      console.log('[TradeLifecycle] Received ORDER_SYNC - refreshing state', {
        totalOrders: data.total_orders,
        syncReason: data.sync_reason,
      });

      // Full refresh to get reconciled state from backend
      fetchOrders();
    };

    const handleChainClosed = (event: WSEvent) => {
      // CHAIN_CLOSED event from backend indicates a stale chain was closed during reconciliation
      const data = event.data;
      if (!data) return;

      console.log('[TradeLifecycle] Received CHAIN_CLOSED - updating chain state', {
        chainId: data.chain_id,
        symbol: data.symbol,
        closeReason: data.close_reason,
        realizedPnl: data.realized_pnl,
      });

      // Update the specific chain to completed status
      setChains(prevChains => {
        const chainIdx = prevChains.findIndex(c => c.chainId === data.chain_id);
        if (chainIdx < 0) return prevChains;

        const updated = [...prevChains];
        updated[chainIdx] = {
          ...updated[chainIdx],
          status: 'completed',
          pnl: data.realized_pnl || updated[chainIdx].pnl,
          positionState: updated[chainIdx].positionState ? {
            ...updated[chainIdx].positionState!,
            status: 'CLOSED',
            realizedPnl: data.realized_pnl || updated[chainIdx].positionState!.realizedPnl,
            closedAt: new Date().toISOString(),
          } : undefined,
          updatedAt: Date.now(),
        };
        return updated;
      });
    };

    const handlePositionCreated = (event: WSEvent) => {
      // POSITION_CREATED event from backend indicates a new position was opened
      // This happens when an entry order is filled
      const positionData = event.data?.position as PositionCreatedEvent | undefined;
      if (!positionData) {
        console.log('[TradeLifecycle] POSITION_CREATED missing position data, refreshing');
        fetchOrders();
        return;
      }

      console.log('[TradeLifecycle] Received POSITION_CREATED - adding new position', {
        chainId: positionData.chain_id,
        symbol: positionData.symbol,
        side: positionData.side,
        entryPrice: positionData.entry_price,
        quantity: positionData.quantity,
      });

      // Full refresh to get the complete chain data from API
      // The position was just created, so we need all the order details
      fetchOrders();

      // Auto-expand positions section when a new position is created
      if (!positionsAutoExpandedRef.current) {
        setPositionsExpanded(true);
        positionsAutoExpandedRef.current = true;
      }
    };

    // Subscribe to WebSocket events
    wsService.subscribe('CHAIN_UPDATE', handleChainUpdate);
    wsService.subscribe('ORDER_UPDATE', handleOrderUpdate);
    wsService.subscribe('TRADE_UPDATE', handleTradeUpdate); // For filled orders (SL/TP triggered)
    wsService.subscribe('POSITION_UPDATE', handlePositionUpdate);
    wsService.subscribe('PNL_UPDATE', handlePnlUpdate);
    wsService.subscribe('ORDER_SYNC', handleOrderSync);
    wsService.subscribe('CHAIN_CLOSED', handleChainClosed);
    wsService.subscribe('POSITION_CREATED', handlePositionCreated); // For instant new position updates
    wsService.onConnect(handleConnect);

    // Register with fallbackManager for centralized fallback polling
    fallbackManager.registerFetchFunction(FALLBACK_KEY, fetchOrders);

    return () => {
      wsService.unsubscribe('CHAIN_UPDATE', handleChainUpdate);
      wsService.unsubscribe('ORDER_UPDATE', handleOrderUpdate);
      wsService.unsubscribe('TRADE_UPDATE', handleTradeUpdate);
      wsService.unsubscribe('POSITION_UPDATE', handlePositionUpdate);
      wsService.unsubscribe('PNL_UPDATE', handlePnlUpdate);
      wsService.unsubscribe('ORDER_SYNC', handleOrderSync);
      wsService.unsubscribe('CHAIN_CLOSED', handleChainClosed);
      wsService.unsubscribe('POSITION_CREATED', handlePositionCreated);
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

  // Reset filters (including date filters)
  const resetFilters = () => {
    setFilters({
      mode: 'all',
      status: 'all',
      symbol: 'all',
      side: 'all',
      dateFrom: undefined,
      dateTo: undefined,
    });
  };

  // Calculate position stats from chains that have active positions
  // Must be before any early returns to follow React hooks rules
  // Note: API may return lowercase (active/partial) or uppercase (ACTIVE/PARTIAL) status
  // A chain has an active position if:
  // 1. It has a positionState with status ACTIVE/PARTIAL, OR
  // 2. The chain status is active/partial AND the entry order is FILLED (position exists but no positionState record)
  const positionStats = useMemo(() => {
    const activePositions = chains.filter(c => {
      // Case 1: Has positionState with ACTIVE/PARTIAL status
      if (c.positionState) {
        const status = c.positionState.status?.toUpperCase();
        return status === 'ACTIVE' || status === 'PARTIAL';
      }

      // Case 2: Chain is active/partial with a filled entry order
      // This handles cases where positionState record is missing but position exists
      const chainStatus = c.status?.toLowerCase();
      if (chainStatus === 'active' || chainStatus === 'partial') {
        // Check if entry order exists and is filled
        const entryFilled = c.entryOrder?.status === 'FILLED';
        // Also check if entry order has executed quantity > 0 (order partially/fully filled)
        const hasFilledQty = (c.entryOrder?.executedQty || 0) > 0;
        return entryFilled || hasFilledQty;
      }

      return false;
    });
    const longPositions = activePositions.filter(c => c.positionSide === 'LONG');
    const shortPositions = activePositions.filter(c => c.positionSide === 'SHORT');
    const totalPnl = activePositions.reduce((sum, c) => sum + (c.positionState?.realizedPnl || 0), 0);

    return {
      total: activePositions.length,
      long: longPositions.length,
      short: shortPositions.length,
      totalPnl,
      positions: activePositions,
    };
  }, [chains]);

  // Auto-expand Orders section when active orders are received
  useEffect(() => {
    // Only auto-expand once, and only if there are active orders
    const hasActiveOrders = chains.some(c => c.status === 'active' || c.status === 'partial');
    if (hasActiveOrders && !ordersAutoExpandedRef.current) {
      setOrdersExpanded(true);
      ordersAutoExpandedRef.current = true;
    }
  }, [chains]);

  // Auto-expand Positions section when active positions are received
  useEffect(() => {
    // Only auto-expand once, and only if there are active positions
    if (positionStats.total > 0 && !positionsAutoExpandedRef.current) {
      setPositionsExpanded(true);
      positionsAutoExpandedRef.current = true;
    }
  }, [positionStats.total]);

  // Loading state
  if (loading && chains.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <div className="flex items-center justify-center text-gray-400">
          <RefreshCw className="w-5 h-5 animate-spin mr-2" />
          Loading trade cycles...
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
    <div className="space-y-4">
      {/* ==================== TRADE CYCLE MAIN CONTAINER ==================== */}
      <div className="bg-gray-800 rounded-lg border border-gray-700">
        {/* Trade Cycle Header - Always visible */}
        <div className="flex items-center justify-between px-4 py-3 rounded-t-lg">
          {/* Left side: Expandable title button */}
          <button
            type="button"
            onClick={() => setTradeCycleExpanded(!tradeCycleExpanded)}
            className="flex items-center gap-3 hover:bg-gray-700/30 transition-colors rounded-lg px-2 py-1 -ml-2"
          >
            {tradeCycleExpanded ? (
              <ChevronDown className="w-5 h-5 text-gray-400" />
            ) : (
              <ChevronRight className="w-5 h-5 text-gray-400" />
            )}
            <GitBranch className="w-5 h-5 text-cyan-400" />
            <span className="font-semibold text-white text-lg">Trade Cycle</span>
            <span className="text-xs bg-purple-500/20 text-purple-400 px-2 py-0.5 rounded font-medium">
              Lifecycle
            </span>
            <span className="text-xs text-gray-400 bg-gray-700 px-2 py-0.5 rounded">
              Entry → Orders → Positions
            </span>
          </button>

          {/* Right side: Trading Toggle + Stats */}
          <div className="flex items-center gap-4">
            {/* Story 14.15: Trading ON/OFF Toggle - Prominent control */}
            <TradingToggle compact />

            {/* Stats summary */}
            <div className="flex items-center gap-3 text-xs">
              <span className="text-purple-400">{stats.activeChains} active orders</span>
              <span className="text-gray-500">|</span>
              <span className="text-green-400">{positionStats.total} positions</span>
            </div>
          </div>
        </div>

        {/* Trade Cycle Content - Three expandable sections */}
        {tradeCycleExpanded && (
          <div className="border-t border-gray-700 p-4 space-y-4">

            {/* ==================== SECTION 0: COIN PROFILER ==================== */}
            {/* First expandable - Real-time data hub for Chain Trading System */}
            {/* CoinProfiler always runs to monitor positions, but strategies only when trading is ON */}
            <CoinProfilerCard />

            {/* ==================== SECTION 1: ENTRY DECISION ENGINE ==================== */}
            {/* Only show when trading is ON - this is for new entry analysis */}
            {tradingEnabled && (
              <EntryDecisionEngineCard defaultExpanded={false} />
            )}

            {/* ==================== SECTION 1.25: STRATEGY-FIRST VIEW (Story 14.13) ==================== */}
            {/* Shows strategies - when trading OFF, only shows coins with active positions */}
            <StrategyFirstView
              defaultExpanded={false}
              tradingEnabled={tradingEnabled}
              onCoinSelect={(symbol, strategy) => {
                console.log('Selected coin:', symbol, 'from strategy:', strategy.strategy);
                // TODO: Navigate to coin details or trigger entry flow
              }}
            />

            {/* ==================== SECTION 1.5: VOLUME IMBALANCE PATTERNS ==================== */}
            {/* Only show when trading is ON - these are new entry signals */}
            {tradingEnabled && volumeImbalancePatterns.length > 0 && (
              <div className="bg-gray-900/50 rounded-lg border border-purple-500/30 p-4">
                <h3 className="text-sm font-semibold text-purple-400 mb-3 flex items-center gap-2">
                  <Activity className="w-4 h-4" />
                  Volume Imbalance Patterns ({volumeImbalancePatterns.length})
                </h3>
                <div className="grid gap-3">
                  {volumeImbalancePatterns.map((pattern) => (
                    <VolumeImbalanceCard
                      key={pattern.id}
                      pattern={pattern}
                      onExecute={async (id) => {
                        console.log('Execute pattern:', id);
                        // TODO: Integrate with autopilot entry execution
                      }}
                      onSkip={async (id, reason) => {
                        console.log('Skip pattern:', id, reason);
                        // TODO: Integrate with pattern skip API
                      }}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* ==================== SECTION 2: ORDERS ==================== */}
            <div className="bg-gray-900/50 rounded-lg border border-gray-700">
              {/* Orders Header - Expandable */}
              <button
                type="button"
                onClick={() => setOrdersExpanded(!ordersExpanded)}
                className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-700/30 transition-colors"
              >
                <div className="flex items-center gap-3">
                  {ordersExpanded ? (
                    <ChevronDown className="w-5 h-5 text-gray-400" />
                  ) : (
                    <ChevronRight className="w-5 h-5 text-gray-400" />
                  )}
                  <Layers className="w-5 h-5 text-purple-400" />
                  <span className="font-semibold text-white">Orders</span>
                  {isHistoricalMode && (
                    <span className="flex items-center gap-1 px-2 py-0.5 rounded text-xs bg-amber-500/20 text-amber-400">
                      <History className="w-3 h-3" />
                      Historical
                    </span>
                  )}
                  <span className="text-xs text-gray-400 bg-gray-700 px-2 py-0.5 rounded">
                    {filteredChains.length} cycle{filteredChains.length !== 1 ? 's' : ''}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  <ConnectionStatus />
                  {!loading && stats.totalChains > 0 && (
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-green-400">{stats.activeChains} active</span>
                      <span className="text-gray-500">|</span>
                      <span className="text-yellow-400">{stats.partialChains} partial</span>
                      <span className="text-gray-500">|</span>
                      <span className="text-blue-400">{stats.completedChains} done</span>
                    </div>
                  )}
                  <button
                    onClick={(e) => { e.stopPropagation(); handleSyncOrderState(); }}
                    className="p-1.5 hover:bg-gray-700 rounded transition-colors"
                    title="Sync with Binance (reconcile stale orders)"
                    disabled={syncing}
                  >
                    <CloudCog className={`w-4 h-4 text-cyan-400 ${syncing ? 'animate-pulse' : ''}`} />
                  </button>
                  <button
                    onClick={(e) => { e.stopPropagation(); setLoading(true); fetchOrders(); }}
                    className="p-1.5 hover:bg-gray-700 rounded transition-colors"
                    title="Refresh"
                  >
                    <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
                  </button>
                </div>
              </button>

              {/* Sync result banner */}
              {syncResult && (
                <div className={`mx-4 mt-2 p-2 rounded-lg text-sm flex items-center gap-2 ${
                  syncResult.success
                    ? 'bg-green-500/10 border border-green-500/30 text-green-400'
                    : 'bg-red-500/10 border border-red-500/30 text-red-400'
                }`}>
                  {syncResult.success ? '✓' : '✕'} {syncResult.message}
                </div>
              )}

              {/* Orders Content */}
              {ordersExpanded && (
                <div className="border-t border-gray-700">
                  {/* Error banner */}
                  {error && chains.length > 0 && (
                    <div className="m-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center justify-between">
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
                  <div className="px-4 pt-4">
                    <div className="grid grid-cols-4 md:grid-cols-8 gap-3 mb-4">
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="text-lg font-semibold text-gray-200">{stats.totalChains}</div>
                        <div className="text-xs text-gray-500">Total</div>
                      </div>
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="text-lg font-semibold text-green-400">{stats.activeChains}</div>
                        <div className="text-xs text-gray-500">Active</div>
                      </div>
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="text-lg font-semibold text-yellow-400">{stats.partialChains}</div>
                        <div className="text-xs text-gray-500">Partial</div>
                      </div>
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="text-lg font-semibold text-blue-400">{stats.completedChains}</div>
                        <div className="text-xs text-gray-500">Complete</div>
                      </div>
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="text-lg font-semibold text-gray-200">{stats.totalOrders}</div>
                        <div className="text-xs text-gray-500">Orders</div>
                      </div>
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="flex items-center justify-center gap-1">
                          <TrendingUp className="w-3.5 h-3.5 text-green-400" />
                          <span className="text-lg font-semibold text-green-400">{stats.longChains}</span>
                        </div>
                        <div className="text-xs text-gray-500">Long</div>
                      </div>
                      <div className="bg-gray-800/50 rounded-lg p-2 text-center">
                        <div className="flex items-center justify-center gap-1">
                          <TrendingDown className="w-3.5 h-3.5 text-red-400" />
                          <span className="text-lg font-semibold text-red-400">{stats.shortChains}</span>
                        </div>
                        <div className="text-xs text-gray-500">Short</div>
                      </div>
                      {stats.fallbackChains > 0 && (
                        <div className="bg-gray-800/50 rounded-lg p-2 text-center">
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
                  <div className="p-4 space-y-3 max-h-[400px] overflow-y-auto">
                    {filteredChains.length === 0 ? (
                      <div className="text-center py-8">
                        <Layers className="w-12 h-12 mx-auto mb-3 text-gray-600" />
                        <p className="text-gray-400">No trade cycles found</p>
                        <p className="text-sm text-gray-500 mt-1">
                          {chains.length === 0
                            ? 'Trade cycles will appear when orders are placed'
                            : 'Try adjusting your filters'}
                        </p>
                      </div>
                    ) : (
                      filteredChains.map((chain) => (
                        <ChainCard
                          key={chain.chainId}
                          chain={chain}
                          livePrice={livePrices.get(chain.symbol)}
                        />
                      ))
                    )}
                  </div>

                  {/* Legend */}
                  <div className="px-4 py-3 border-t border-gray-700 bg-gray-800/30">
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
              )}
            </div>

            {/* ==================== SECTION 3: POSITIONS ==================== */}
            <div className="bg-gray-900/50 rounded-lg border border-gray-700">
              {/* Positions Header - Expandable */}
              <button
                type="button"
                onClick={() => setPositionsExpanded(!positionsExpanded)}
                className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-700/30 transition-colors"
              >
                <div className="flex items-center gap-3">
                  {positionsExpanded ? (
                    <ChevronDown className="w-5 h-5 text-gray-400" />
                  ) : (
                    <ChevronRight className="w-5 h-5 text-gray-400" />
                  )}
                  <Briefcase className="w-5 h-5 text-green-400" />
                  <span className="font-semibold text-white">Positions</span>
                  <span className="text-xs text-gray-400 bg-gray-700 px-2 py-0.5 rounded">
                    {positionStats.total} active position{positionStats.total !== 1 ? 's' : ''}
                  </span>
                </div>
                <div className="flex items-center gap-3">
                  {positionStats.total > 0 && (
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-green-400 flex items-center gap-1">
                        <TrendingUp className="w-3 h-3" />
                        {positionStats.long} long
                      </span>
                      <span className="text-gray-500">|</span>
                      <span className="text-red-400 flex items-center gap-1">
                        <TrendingDown className="w-3 h-3" />
                        {positionStats.short} short
                      </span>
                      {positionStats.totalPnl !== 0 && (
                        <>
                          <span className="text-gray-500">|</span>
                          <span className={positionStats.totalPnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                            PnL: {positionStats.totalPnl >= 0 ? '+' : ''}{positionStats.totalPnl.toFixed(2)} USDT
                          </span>
                        </>
                      )}
                    </div>
                  )}
                  {/* Manual refresh button for positions */}
                  <button
                    onClick={(e) => { e.stopPropagation(); setLoading(true); fetchOrders(); }}
                    className="p-1.5 hover:bg-gray-700 rounded transition-colors"
                    title="Refresh positions"
                  >
                    <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
                  </button>
                </div>
              </button>

              {/* Positions Content */}
              {positionsExpanded && (
                <div className="border-t border-gray-700 p-4">
                  {positionStats.total === 0 ? (
                    <div className="text-center py-8">
                      <Briefcase className="w-12 h-12 mx-auto mb-3 text-gray-600" />
                      <p className="text-gray-400">No active positions</p>
                      <p className="text-sm text-gray-500 mt-1">
                        Positions will appear here when orders are filled and positions are opened.
                      </p>
                    </div>
                  ) : (
                    <div className="space-y-3 max-h-[500px] overflow-y-auto">
                      {positionStats.positions.map((chain) => {
                        // Get price levels - support both positionState and fallback to entry order data
                        // When positionState is missing (no DB record), derive from entry order
                        const entryOrder = chain.entryOrder;
                        const hasPositionState = !!chain.positionState;

                        // Entry price: from positionState, or entry order's avgPrice (filled), or entry order's price
                        const entryPrice = chain.positionState?.entryPrice ||
                          (entryOrder?.avgPrice && entryOrder.avgPrice > 0 ? entryOrder.avgPrice : entryOrder?.price) || 0;

                        // Current price: use live price from coin profiler, fallback to analytics, then entry price
                        const currentPrice = livePrices.get(chain.symbol) || chain.positionAnalytics?.current_price || entryPrice;

                        // SL price: from order or analytics
                        const slPrice = chain.slOrder?.stopPrice || chain.slOrder?.price || chain.positionAnalytics?.stop_loss || 0;

                        // TP price: from order or analytics
                        const tp1Price = chain.tpOrders?.[0]?.stopPrice || chain.tpOrders?.[0]?.price || chain.positionAnalytics?.tp1_price || 0;

                        const isLong = chain.positionSide === 'LONG';

                        // Remaining quantity: from positionState, or entry order's executedQty (full position if no partial closes)
                        const remainingQty = chain.positionState?.remainingQuantity ||
                          (entryOrder?.executedQty || 0);

                        // Entry quantity: from positionState, or entry order's executedQty
                        const entryQty = chain.positionState?.entryQuantity ||
                          (entryOrder?.executedQty || 0);

                        // Calculate unrealized PnL
                        const unrealizedPnl = isLong
                          ? (currentPrice - entryPrice) * remainingQty
                          : (entryPrice - currentPrice) * remainingQty;
                        const pnlPercent = entryPrice > 0 ? ((currentPrice - entryPrice) / entryPrice) * 100 * (isLong ? 1 : -1) : 0;

                        // Calculate progress to SL and TP
                        const riskDistance = isLong ? entryPrice - slPrice : slPrice - entryPrice;
                        const rewardDistance = isLong ? tp1Price - entryPrice : entryPrice - tp1Price;
                        const currentFromEntry = isLong ? currentPrice - entryPrice : entryPrice - currentPrice;

                        // Derive status when positionState is missing
                        const positionStatus = chain.positionState?.status?.toUpperCase() ||
                          (chain.status === 'partial' ? 'PARTIAL' : 'ACTIVE');

                        // Progress: negative = towards SL, positive = towards TP
                        const progressToTP = rewardDistance > 0 ? (currentFromEntry / rewardDistance) * 100 : 0;
                        const progressToSL = riskDistance > 0 ? (-currentFromEntry / riskDistance) * 100 : 0;

                        // Determine stage
                        const stage = chain.positionAnalytics?.stage ||
                          (currentFromEntry < 0 ? 'RISK_ZONE' :
                           currentFromEntry >= rewardDistance ? 'TP1' :
                           currentFromEntry >= 0 ? 'BREAKEVEN' : 'RISK_ZONE');

                        // Stage display config
                        const stageConfig: Record<string, { label: string; color: string; bgColor: string; icon: string }> = {
                          'RISK_ZONE': { label: 'Risk Zone', color: 'text-red-400', bgColor: 'bg-red-500/20', icon: '⚠️' },
                          'BREAKEVEN': { label: 'Breakeven', color: 'text-blue-400', bgColor: 'bg-blue-500/20', icon: '🛡️' },
                          'TP1': { label: 'TP1 Zone', color: 'text-green-400', bgColor: 'bg-green-500/20', icon: '🎯' },
                          'EFFICIENCY': { label: 'Efficiency', color: 'text-cyan-400', bgColor: 'bg-cyan-500/20', icon: '📈' },
                        };
                        const currentStage = stageConfig[stage] || stageConfig['RISK_ZONE'];

                        return (
                          <div
                            key={chain.chainId}
                            className="bg-gray-800/50 rounded-lg p-4 border border-gray-700/50"
                          >
                            {/* Position Header */}
                            <div className="flex items-center justify-between mb-3">
                              <div className="flex items-center gap-2">
                                <span className="font-bold text-white text-lg">{chain.symbol}</span>
                                <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${
                                  isLong ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                                }`}>
                                  {isLong ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
                                  {chain.positionSide}
                                </span>
                                {chain.modeCode && (
                                  <span className="px-2 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                                    {chain.modeCode}
                                  </span>
                                )}
                                {/* Stage Badge */}
                                <span className={`px-2 py-0.5 rounded text-xs font-medium ${currentStage.bgColor} ${currentStage.color}`}>
                                  {currentStage.icon} {currentStage.label}
                                </span>
                              </div>
                              <div className="text-right">
                                <div className={`text-sm font-bold ${unrealizedPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  {unrealizedPnl >= 0 ? '+' : ''}{unrealizedPnl.toFixed(4)} USDT
                                </div>
                                <div className={`text-xs ${pnlPercent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                                  ({pnlPercent >= 0 ? '+' : ''}{pnlPercent.toFixed(2)}%)
                                </div>
                              </div>
                            </div>

                            {/* Price Levels Grid */}
                            <div className="grid grid-cols-4 gap-3 text-sm mb-3">
                              <div className="text-center p-2 bg-gray-700/30 rounded">
                                <div className="text-xs text-gray-500">Entry</div>
                                <div className="text-gray-200 font-mono">${entryPrice.toFixed(8)}</div>
                              </div>
                              <div className="text-center p-2 bg-gray-700/30 rounded">
                                <div className="text-xs text-gray-500">Current</div>
                                <div className={`font-mono font-bold ${
                                  // For LONG: higher price = profit (green), lower = loss (red)
                                  // For SHORT: lower price = profit (green), higher = loss (red)
                                  (isLong ? currentPrice >= entryPrice : currentPrice <= entryPrice)
                                    ? 'text-green-400' : 'text-red-400'
                                }`}>
                                  ${currentPrice.toFixed(8)}
                                </div>
                              </div>
                              <div className="text-center p-2 bg-red-900/20 rounded border border-red-800/30">
                                <div className="text-xs text-red-400">SL</div>
                                <div className="text-red-300 font-mono">{slPrice > 0 ? `$${slPrice.toFixed(8)}` : '-'}</div>
                              </div>
                              <div className="text-center p-2 bg-green-900/20 rounded border border-green-800/30">
                                <div className="text-xs text-green-400">TP</div>
                                <div className="text-green-300 font-mono">{tp1Price > 0 ? `$${tp1Price.toFixed(8)}` : '-'}</div>
                              </div>
                            </div>

                            {/* Price Progress Visualization - R:R based positioning */}
                            {slPrice > 0 && tp1Price > 0 && (
                              <div className="mb-3">
                                {(() => {
                                  // Calculate R:R ratio and entry position
                                  // riskDistance and rewardDistance already calculated above
                                  const totalRange = riskDistance + rewardDistance;
                                  const rrRatio = rewardDistance > 0 && riskDistance > 0 ? rewardDistance / riskDistance : 4;

                                  // Entry position on bar based on R:R
                                  // For 1:4 R:R: entry = 1/(1+4) * 100 = 20% for LONG
                                  // For SHORT: same calculation works because riskDistance/rewardDistance are already adjusted
                                  const entryPositionPercent = totalRange > 0
                                    ? (riskDistance / totalRange) * 100
                                    : 20; // Default to 1:4 (20%)

                                  // Current price position (0% = SL end, 100% = TP end)
                                  // For LONG: 0% = SL (lower), 100% = TP (higher)
                                  // For SHORT: 0% = SL end (higher price), 100% = TP end (lower price)
                                  let currentPositionPercent: number;
                                  if (isLong) {
                                    // LONG: SL is lower, TP is higher
                                    const priceRange = tp1Price - slPrice;
                                    currentPositionPercent = priceRange > 0
                                      ? ((currentPrice - slPrice) / priceRange) * 100
                                      : 50;
                                  } else {
                                    // SHORT: TP is lower, SL is higher
                                    // Normalize so bar always shows SL on left, TP on right
                                    const priceRange = slPrice - tp1Price;
                                    currentPositionPercent = priceRange > 0
                                      ? ((slPrice - currentPrice) / priceRange) * 100
                                      : 50;
                                  }
                                  currentPositionPercent = Math.max(-10, Math.min(110, currentPositionPercent));

                                  // Check if SL has moved to breakeven (SL = Entry)
                                  const breakevenPrice = chain.positionAnalytics?.breakeven_price || entryPrice;
                                  const isAtBreakeven = Math.abs(slPrice - entryPrice) < (entryPrice * 0.001); // Within 0.1%

                                  // Breakeven position on bar (if different from entry)
                                  let breakevenPositionPercent: number | null = null;
                                  if (chain.positionAnalytics?.breakeven_price && Math.abs(breakevenPrice - entryPrice) > (entryPrice * 0.001)) {
                                    if (isLong) {
                                      const priceRange = tp1Price - slPrice;
                                      breakevenPositionPercent = priceRange > 0
                                        ? ((breakevenPrice - slPrice) / priceRange) * 100
                                        : null;
                                    } else {
                                      const priceRange = slPrice - tp1Price;
                                      breakevenPositionPercent = priceRange > 0
                                        ? ((slPrice - breakevenPrice) / priceRange) * 100
                                        : null;
                                    }
                                  }

                                  // Format price with 8 decimal precision (Binance standard)
                                  const formatPrice = (price: number) => {
                                    return price.toFixed(8);
                                  };

                                  return (
                                    <>
                                      {/* Labels row - positioned based on R:R */}
                                      <div className="relative flex items-center text-xs text-gray-500 mb-1 h-4">
                                        {/* SL label at left (0%) */}
                                        <span className="absolute left-0 text-red-400" style={{ transform: 'translateX(0)' }}>
                                          SL ${formatPrice(slPrice)}
                                        </span>
                                        {/* Entry label at calculated position */}
                                        <span
                                          className="absolute text-blue-400"
                                          style={{
                                            left: `${entryPositionPercent}%`,
                                            transform: 'translateX(-50%)'
                                          }}
                                        >
                                          Entry ${formatPrice(entryPrice)}
                                        </span>
                                        {/* TP label at right (100%) */}
                                        <span className="absolute right-0 text-green-400" style={{ transform: 'translateX(0)' }}>
                                          TP ${formatPrice(tp1Price)}
                                        </span>
                                      </div>

                                      {/* Progress bar */}
                                      <div className="relative h-3 bg-gray-700 rounded-full overflow-hidden">
                                        {/* Risk zone (red) - from 0% to entry position */}
                                        <div
                                          className="absolute left-0 h-full bg-red-500/30"
                                          style={{ width: `${entryPositionPercent}%` }}
                                        />
                                        {/* Profit zone (green) - from entry position to 100% */}
                                        <div
                                          className="absolute h-full bg-green-500/30"
                                          style={{
                                            left: `${entryPositionPercent}%`,
                                            width: `${100 - entryPositionPercent}%`
                                          }}
                                        />
                                        {/* Entry marker */}
                                        <div
                                          className="absolute top-0 bottom-0 w-0.5 bg-blue-400 z-10"
                                          style={{ left: `${entryPositionPercent}%`, transform: 'translateX(-50%)' }}
                                        />
                                        {/* Breakeven marker (if different from entry) */}
                                        {breakevenPositionPercent !== null && (
                                          <div
                                            className="absolute top-0 bottom-0 w-0.5 bg-yellow-400 z-10 opacity-70"
                                            style={{ left: `${breakevenPositionPercent}%`, transform: 'translateX(-50%)' }}
                                            title={`Breakeven: $${formatPrice(breakevenPrice)}`}
                                          />
                                        )}
                                        {/* Current price marker */}
                                        <div
                                          className="absolute top-0 bottom-0 w-1.5 bg-white rounded shadow-lg shadow-white/50 transition-all duration-300 z-20"
                                          style={{ left: `${currentPositionPercent}%`, transform: 'translateX(-50%)' }}
                                          title={`Current: $${formatPrice(currentPrice)}`}
                                        />
                                      </div>

                                      {/* Progress text with R:R info */}
                                      <div className="flex justify-between mt-1 text-xs">
                                        <span className={progressToSL > 0 ? 'text-red-400' : 'text-gray-500'}>
                                          {progressToSL > 0 ? `${progressToSL.toFixed(0)}% to SL` : ''}
                                          {isAtBreakeven && <span className="text-yellow-400 ml-1">(Break Even)</span>}
                                        </span>
                                        <span className="text-gray-500">
                                          R:R 1:{rrRatio.toFixed(1)}
                                        </span>
                                        <span className={progressToTP > 0 ? 'text-green-400' : 'text-gray-500'}>
                                          {progressToTP > 0 ? `${progressToTP.toFixed(0)}% to TP` : ''}
                                        </span>
                                      </div>
                                    </>
                                  );
                                })()}
                              </div>
                            )}

                            {/* Expected TP Profit & SL Loss */}
                            {tp1Price > 0 && slPrice > 0 && entryPrice > 0 && remainingQty > 0 && (
                              <div className="flex items-center justify-between text-xs mb-2 px-1">
                                {(() => {
                                  const feeRate = 0.0004; // 0.04% taker fee
                                  const entryFee = entryPrice * remainingQty * feeRate;

                                  // Expected TP Profit
                                  const tpPriceChange = isLong ? tp1Price - entryPrice : entryPrice - tp1Price;
                                  const tpGrossPnl = tpPriceChange * remainingQty;
                                  const tpExitFee = tp1Price * remainingQty * feeRate;
                                  const expectedTpProfit = tpGrossPnl - entryFee - tpExitFee;

                                  // Expected SL Loss
                                  const slPriceChange = isLong ? slPrice - entryPrice : entryPrice - slPrice;
                                  const slGrossPnl = slPriceChange * remainingQty;
                                  const slExitFee = slPrice * remainingQty * feeRate;
                                  const expectedSlLoss = slGrossPnl - entryFee - slExitFee;

                                  return (
                                    <>
                                      <span className="text-green-400">
                                        TP: +${expectedTpProfit.toFixed(2)}
                                      </span>
                                      <span className="text-gray-600">|</span>
                                      <span className="text-red-400">
                                        SL: {expectedSlLoss >= 0 ? '+' : ''}${expectedSlLoss.toFixed(2)}
                                      </span>
                                    </>
                                  );
                                })()}
                              </div>
                            )}

                            {/* Bottom Stats Row */}
                            <div className="flex items-center justify-between text-xs text-gray-500 pt-2 border-t border-gray-700/50">
                              <span>Qty: {remainingQty.toFixed(4)} / {entryQty.toFixed(4)}</span>
                              <span>Chain: {chain.chainId}</span>
                              <span className={`${
                                positionStatus === 'ACTIVE' ? 'text-green-400' :
                                positionStatus === 'PARTIAL' ? 'text-yellow-400' : 'text-gray-400'
                              }`}>
                                {positionStatus}
                                {!hasPositionState && <span className="text-orange-400 ml-1">(derived)</span>}
                              </span>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}
            </div>

          </div>
        )}
      </div>
    </div>
  );
}
