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
import PositionCard from './PositionCard';
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
  const [activeOnly, setActiveOnly] = useState(false);

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
    closePrice: state.close_price,
    closeReason: state.close_reason,
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
    const tpOrders = chainOrders.filter(o => o.orderType && ['TP', 'TP1', 'TP2', 'TP3'].includes(o.orderType));
    const slOrder = chainOrders.find(o => o.orderType === 'SL') || null;
    const dcaOrders = chainOrders.filter(o => o.orderType && ['DCA1', 'DCA2', 'DCA3'].includes(o.orderType));
    const rebuyOrder = chainOrders.find(o => o.orderType === 'RB') || null;
    const hedgeOrder = chainOrders.find(o => o.orderType === 'H') || null;
    const hedgeSLOrder = chainOrders.find(o => o.orderType === 'HSL') || null;
    const hedgeTPOrder = chainOrders.find(o => o.orderType === 'HTP') || null;

    // Extract SL/TP lifecycle status from order data (for API-sourced chains)
    const slStatusFromOrder = slOrder?.status;
    const tpStatusFromOrder = tpOrders.length > 0 ? tpOrders[0].status : undefined;
    const slFillPriceFromOrder = slOrder && (slOrder.avgPrice ?? 0) > 0 ? slOrder.avgPrice : undefined;
    const tpFillPriceFromOrder = tpOrders.length > 0 && (tpOrders[0].avgPrice ?? 0) > 0 ? tpOrders[0].avgPrice : undefined;

    // Extract close price and reason from position state
    const posState = apiChain.position_state;
    const closePriceFromState = posState && posState.close_price > 0 ? posState.close_price : undefined;
    const closedAtFromState = posState?.closed_at || undefined;
    const closeReasonFromState = posState?.close_reason || undefined;

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
      // Trailing stop status from RavindraPositionMonitor
      trailingStopStatus: apiChain.trailing_stop_status || undefined,
      // SL modification history from chain events
      slModifications: apiChain.sl_modifications?.map(mod => ({
        sequence: mod.sequence,
        oldPrice: mod.old_price,
        newPrice: mod.new_price,
        reason: mod.reason,
        source: mod.source,
        binanceOrderId: mod.binance_order_id,
        timestamp: mod.timestamp,
      })) || undefined,
      // SL/TP lifecycle status extracted from order data (for closed chains loaded from API)
      slStatus: slStatusFromOrder,
      tpStatus: tpStatusFromOrder,
      slFillPrice: slFillPriceFromOrder,
      tpFillPrice: tpFillPriceFromOrder,
      // Close price/reason from position state
      closePrice: closePriceFromState,
      closedAt: closedAtFromState,
      closeReason: closeReasonFromState,
    };
  };

  // Helper: Convert historical API response to frontend OrderChain
  const mapHistoricalOrderChain = (histChain: HistoricalOrderChain): OrderChain => {
    // Historical chains from DB don't include individual orders from Binance,
    // but we create synthetic order objects from summary data so ChainCard/OrderTreeNode
    // can display entry, SL, TP information.
    const parsedEntry = parseClientOrderId(histChain.chainId + '-E');
    const parsedSL = parseClientOrderId(histChain.chainId + '-SL');
    const parsedTP = parseClientOrderId(histChain.chainId + '-TP');

    const createdAtMs = histChain.createdAt ? new Date(histChain.createdAt).getTime() || Date.now() : Date.now();
    const updatedAtMs = histChain.updatedAt ? new Date(histChain.updatedAt).getTime() || Date.now() : Date.now();
    const closedAtMs = histChain.closedAt ? new Date(histChain.closedAt).getTime() || undefined : undefined;
    const positionSide = (histChain.side === 'BUY' ? 'LONG' : 'SHORT') as 'LONG' | 'SHORT' | 'BOTH';
    const closeSide = (histChain.side === 'BUY' ? 'SELL' : 'BUY') as 'BUY' | 'SELL';

    const isSLFilled = histChain.closeReason === 'SL_HIT' || histChain.closeReason === 'PROTECTION_CLOSE';
    const isTPFilled = histChain.closeReason === 'TP_HIT';

    // Synthetic entry order
    const entryOrder: ChainOrder = {
      orderId: 0,
      clientOrderId: histChain.chainId + '-E',
      symbol: histChain.symbol,
      side: histChain.side as 'BUY' | 'SELL',
      positionSide,
      type: 'LIMIT',
      status: 'FILLED',
      price: histChain.entryPrice,
      avgPrice: histChain.entryPrice,
      origQty: histChain.entryQuantity,
      executedQty: histChain.entryQuantity,
      stopPrice: 0,
      time: createdAtMs,
      updateTime: updatedAtMs,
      orderType: 'E',
      parsed: parsedEntry,
    };

    // Synthetic SL order (only if SL price exists)
    const slOrder: ChainOrder | null = histChain.currentSlPrice > 0 ? {
      orderId: 0,
      clientOrderId: histChain.chainId + '-SL',
      symbol: histChain.symbol,
      side: closeSide,
      positionSide,
      type: 'STOP_MARKET',
      status: isSLFilled ? 'FILLED' : (histChain.status === 'ACTIVE' ? 'NEW' : 'CANCELED'),
      price: histChain.currentSlPrice,
      avgPrice: isSLFilled ? histChain.currentSlPrice : 0,
      origQty: histChain.entryQuantity,
      executedQty: isSLFilled ? histChain.entryQuantity : 0,
      stopPrice: histChain.currentSlPrice,
      time: createdAtMs,
      updateTime: closedAtMs || updatedAtMs,
      orderType: 'SL',
      parsed: parsedSL,
    } : null;

    // Synthetic TP order (only if TP price exists)
    const tpOrder: ChainOrder | null = histChain.currentTpPrice > 0 ? {
      orderId: 0,
      clientOrderId: histChain.chainId + '-TP',
      symbol: histChain.symbol,
      side: closeSide,
      positionSide,
      type: 'TAKE_PROFIT_MARKET',
      status: isTPFilled ? 'FILLED' : (histChain.status === 'ACTIVE' ? 'NEW' : 'CANCELED'),
      price: histChain.currentTpPrice,
      avgPrice: isTPFilled ? histChain.currentTpPrice : 0,
      origQty: histChain.entryQuantity,
      executedQty: isTPFilled ? histChain.entryQuantity : 0,
      stopPrice: histChain.currentTpPrice,
      time: createdAtMs,
      updateTime: closedAtMs || updatedAtMs,
      orderType: 'TP',
      parsed: parsedTP,
    } : null;

    // Build orders array
    const orders: ChainOrder[] = [entryOrder];
    if (slOrder) orders.push(slOrder);
    if (tpOrder) orders.push(tpOrder);

    return {
      chainId: histChain.chainId,
      modeCode: (histChain.modeCode as TradingModeCode) || null,
      dateStr: parsedEntry?.dateStr || null,
      sequence: parsedEntry?.sequence || null,
      symbol: histChain.symbol,
      side: histChain.side as 'BUY' | 'SELL' || null,
      positionSide,
      orders,
      entryOrder,
      tpOrders: tpOrder ? [tpOrder] : [],
      slOrder,
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
      createdAt: createdAtMs,
      updatedAt: updatedAtMs,
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
        closePrice: isSLFilled ? histChain.currentSlPrice : (isTPFilled ? histChain.currentTpPrice : undefined),
        closeReason: histChain.closeReason,
      },
      modificationCounts: {
        SL: histChain.slModificationCount || 0,
        TP1: histChain.tpModificationCount || 0,
      },
      // Close-related fields for ChainCard display
      closeReason: histChain.closeReason,
      closePrice: isSLFilled ? histChain.currentSlPrice : (isTPFilled ? histChain.currentTpPrice : undefined),
      closedAt: histChain.closedAt,
      realizedPnl: histChain.realizedPnl,
      totalFees: histChain.totalFees,
      slStatus: isSLFilled ? 'FILLED' : (histChain.currentSlPrice > 0 ? 'CANCELED' : undefined),
      tpStatus: isTPFilled ? 'FILLED' : (histChain.currentTpPrice > 0 ? 'CANCELED' : undefined),
      slFillPrice: isSLFilled ? histChain.currentSlPrice : undefined,
      tpFillPrice: isTPFilled ? histChain.currentTpPrice : undefined,
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

      // Merge protection: preserve WebSocket-derived close state that API cache might overwrite.
      // When CHAIN_LIFECYCLE_UPDATE has already updated a chain to 'completed' with SL/TP status,
      // the API's stale cache (up to 5s) can return the chain as still 'active' with SL/TP 'NEW'.
      // Preserve the authoritative close state from WebSocket.
      setChains(prev => {
        // Build map of chains that have WebSocket-derived close data
        const closedChainState = new Map<string, OrderChain>();
        for (const chain of prev) {
          if (chain.status === 'completed' && (chain.slStatus || chain.tpStatus || chain.closedAt)) {
            closedChainState.set(chain.chainId, chain);
          }
        }

        // If no closed chains to protect, just use merged chains as-is
        if (closedChainState.size === 0) return mergedChains;

        // Merge: for chains that we know are closed (from WebSocket), preserve close state
        return mergedChains.map(chain => {
          const closedData = closedChainState.get(chain.chainId);
          if (!closedData) return chain;

          // API returned potentially stale data for a chain we know is closed
          // Preserve the authoritative close state
          return {
            ...chain,
            status: 'completed' as const,
            slStatus: closedData.slStatus || chain.slStatus,
            tpStatus: closedData.tpStatus || chain.tpStatus,
            slFillPrice: closedData.slFillPrice || chain.slFillPrice,
            tpFillPrice: closedData.tpFillPrice || chain.tpFillPrice,
            closedAt: closedData.closedAt || chain.closedAt,
            closeReason: closedData.closeReason || chain.closeReason,
            closePrice: closedData.closePrice || chain.closePrice,
            realizedPnl: closedData.realizedPnl ?? chain.realizedPnl,
            pnl: closedData.pnl ?? chain.pnl,
            totalFees: closedData.totalFees ?? chain.totalFees,
            slOrder: closedData.slOrder || chain.slOrder,
            tpOrders: closedData.tpOrders?.length ? closedData.tpOrders : chain.tpOrders,
            positionState: closedData.positionState || chain.positionState,
          };
        });
      });
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

      if (!Array.isArray(positions)) {
        return; // No data to update
      }

      // Empty positions array is ignored - it does NOT mean all positions are closed.
      // Position closes are handled authoritatively by CHAIN_LIFECYCLE_UPDATE events
      // from the PositionLifecycleCoordinator. The backend's periodic broadcastPositionStatus()
      // can send empty arrays when the internal position map hasn't been populated yet.
      if (positions.length === 0) {
        return;
      }

      setChains(prevChains => {
        let updated = [...prevChains];
        let hasChanges = false;

        for (const pos of positions) {
          const symbol = pos.symbol;
          const posAmt = parseFloat(pos.position_amount || pos.position_amt || pos.positionAmt || '0');
          const posSide = (pos.position_side || pos.positionSide || '').toUpperCase();

          // Find chains for this symbol
          for (let i = 0; i < updated.length; i++) {
            if (updated[i].symbol === symbol && updated[i].positionState) {
              // In hedge mode, skip if position side doesn't match chain side
              const chainSide = (updated[i].positionSide || '').toUpperCase();
              if (posSide && chainSide && posSide !== chainSide) {
                continue;
              }
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
      const tradeData = event.data;
      if (!tradeData) return;

      console.log('[TradeLifecycle] Received TRADE_UPDATE', {
        symbol: tradeData.symbol,
        orderId: tradeData.order_id || tradeData.orderId,
        executionType: tradeData.execution_type || tradeData.executionType,
      });

      // TRADE_UPDATE is informational only - position close state is handled
      // authoritatively by CHAIN_LIFECYCLE_UPDATE from PositionLifecycleCoordinator.
      // DO NOT call fetchOrders() here - it races with WebSocket state updates
      // and the 5s API cache can serve stale data that overwrites correct close status.
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

    // Story 14.19: CHAIN_LIFECYCLE_UPDATE - composite event from PositionLifecycleCoordinator
    const handleChainLifecycleUpdate = (event: WSEvent) => {
      const data = event.data;
      if (!data?.chain?.chain_id) return;

      console.log('[TradeLifecycle] Received CHAIN_LIFECYCLE_UPDATE', {
        chainId: data.chain.chain_id,
        closeReason: data.chain.close_reason,
        realizedPnl: data.chain.realized_pnl,
      });

      setChains(prev => prev.map(chain => {
        if (chain.chainId !== data.chain.chain_id) return chain;

        // Update individual SL order object status
        const updatedSlOrder = chain.slOrder ? {
          ...chain.slOrder,
          status: data.orders?.sl_status || chain.slOrder.status,
        } : undefined;

        // Update individual TP order objects status
        const updatedTpOrders = chain.tpOrders?.map(tp => ({
          ...tp,
          status: data.orders?.tp_status || tp.status,
        }));

        return {
          ...chain,
          status: 'completed',
          realizedPnl: data.chain.realized_pnl,
          totalFees: data.chain.total_fees,
          closePrice: data.chain.close_price,
          closedAt: data.chain.closed_at,
          closeReason: data.chain.close_reason,
          pnl: data.chain.realized_pnl,
          positionState: chain.positionState ? {
            ...chain.positionState,
            status: 'CLOSED' as const,
            realizedPnl: data.chain.realized_pnl,
            closedAt: data.chain.closed_at,
            closePrice: data.chain.close_price,
            closeReason: data.chain.close_reason,
          } : undefined,
          slOrder: updatedSlOrder,
          tpOrders: updatedTpOrders,
          slStatus: data.orders?.sl_status,
          tpStatus: data.orders?.tp_status,
          slFillPrice: data.orders?.sl_fill_price,
          tpFillPrice: data.orders?.tp_fill_price,
          updatedAt: Date.now(),
        };
      }));
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

      // Initial fetch for entry order data (SL/TP will arrive via SL_TP_PLACED event)
      fetchOrders();

      // Auto-expand positions section when a new position is created
      if (!positionsAutoExpandedRef.current) {
        setPositionsExpanded(true);
        positionsAutoExpandedRef.current = true;
      }
    };

    // Handler for SL/TP orders placed after position creation
    const handleSLTPPlaced = (event: any) => {
      const data = event.data;
      if (!data) return;

      console.log('[TradeLifecycle] SL/TP orders placed', {
        chainId: data.chain_id,
        slPrice: data.sl_price,
        tpPrice: data.tp_price,
      });

      // Update chain state directly from event data to avoid 5s API cache delay.
      // This ensures SL/TP orders appear immediately in the order tree.
      setChains(prev => prev.map(chain => {
        if (chain.chainId !== data.chain_id) return chain;

        // Determine the close side from chain's position side
        const closeSide: 'BUY' | 'SELL' = chain.positionSide === 'LONG' ? 'SELL' : 'BUY';
        const now = Date.now();

        // Build synthetic SL order from event data
        const newOrders: ChainOrder[] = [...chain.orders];
        let newSlOrder = chain.slOrder;
        let newTpOrders = [...chain.tpOrders];

        if (data.sl_order_id && data.sl_price) {
          const slClientOrderId = chain.chainId + '-SL';
          const slOrder: ChainOrder = {
            orderId: data.sl_order_id,
            clientOrderId: slClientOrderId,
            symbol: data.symbol || chain.symbol || '',
            side: closeSide,
            positionSide: chain.positionSide || 'BOTH',
            type: 'STOP_MARKET',
            status: data.sl_status || 'NEW',
            price: 0,
            origQty: chain.positionState?.entryQuantity || 0,
            executedQty: 0,
            stopPrice: data.sl_price,
            time: now,
            updateTime: now,
            orderType: 'SL' as const,
            parsed: parseClientOrderId(slClientOrderId),
          };
          // Only add if not already present (deduplicate by orderId)
          if (!newOrders.some(o => o.orderId === data.sl_order_id)) {
            newOrders.push(slOrder);
          }
          newSlOrder = slOrder;
        }

        if (data.tp_order_id && data.tp_price) {
          const tpClientOrderId = chain.chainId + '-TP1';
          const tpOrder: ChainOrder = {
            orderId: data.tp_order_id,
            clientOrderId: tpClientOrderId,
            symbol: data.symbol || chain.symbol || '',
            side: closeSide,
            positionSide: chain.positionSide || 'BOTH',
            type: 'TAKE_PROFIT_MARKET',
            status: data.tp_status || 'NEW',
            price: 0,
            origQty: chain.positionState?.entryQuantity || 0,
            executedQty: 0,
            stopPrice: data.tp_price,
            time: now,
            updateTime: now,
            orderType: 'TP1' as const,
            parsed: parseClientOrderId(tpClientOrderId),
          };
          // Only add if not already present (deduplicate by orderId)
          if (!newOrders.some(o => o.orderId === data.tp_order_id)) {
            newOrders.push(tpOrder);
          }
          newTpOrders = [tpOrder];
        }

        return {
          ...chain,
          orders: newOrders,
          slOrder: newSlOrder,
          tpOrders: newTpOrders.length > 0 ? newTpOrders : chain.tpOrders,
          updatedAt: now,
        };
      }));

      // Also fetch after cache expires to get full accurate data from API
      setTimeout(() => fetchOrders(), 6000);
    };

    // Handler for trailing SL updates from Ravindra Position Monitor
    // This fires when the monitor moves the SL at a R:R milestone (1:2 breakeven, 1:3 profit lock)
    const handleTrailingSLUpdate = (event: WSEvent) => {
      const data = event.data?.trailing_sl;
      if (!data?.chain_id) return;

      console.log('[TradeLifecycle] Received TRAILING_SL_UPDATE', {
        chainId: data.chain_id,
        action: data.action,
        oldSL: data.old_sl,
        newSL: data.new_sl,
        currentRR: data.current_rr,
      });

      setChains(prev => prev.map(chain => {
        if (chain.chainId !== data.chain_id) return chain;

        // Update SL order price to reflect the new SL
        const updatedSlOrder = chain.slOrder ? {
          ...chain.slOrder,
          stopPrice: data.new_sl || chain.slOrder.stopPrice,
          price: data.new_sl || chain.slOrder.price,
        } : chain.slOrder;

        // Update trailing stop status with the new data from backend
        const updatedTrailingStopStatus = chain.trailingStopStatus ? {
          ...chain.trailingStopStatus,
          current_stop_loss: data.new_sl ?? chain.trailingStopStatus.current_stop_loss,
          current_rr: data.current_rr ?? chain.trailingStopStatus.current_rr,
          at_breakeven: data.at_breakeven ?? chain.trailingStopStatus.at_breakeven,
          at_1r: data.at_1r ?? chain.trailingStopStatus.at_1r,
          highest_price: data.trailing_stop_status?.highest_price ?? chain.trailingStopStatus.highest_price,
        } : data.trailing_stop_status ? {
          entry_price: data.entry_price || 0,
          current_stop_loss: data.new_sl || 0,
          take_profit: data.trailing_stop_status?.take_profit || 0,
          risk_amount: data.trailing_stop_status?.risk_amount || 0,
          current_rr: data.current_rr || 0,
          highest_price: data.trailing_stop_status?.highest_price || 0,
          at_breakeven: data.at_breakeven || false,
          at_1r: data.at_1r || false,
          breakeven_level: data.trailing_stop_status?.breakeven_level || 0,
          one_r_level: data.trailing_stop_status?.one_r_level || 0,
          breakeven_trigger: data.trailing_stop_status?.breakeven_trigger || 0,
          one_r_trigger: data.trailing_stop_status?.one_r_trigger || 0,
          side: data.trailing_stop_status?.side || '',
        } : chain.trailingStopStatus;

        return {
          ...chain,
          slOrder: updatedSlOrder,
          trailingStopStatus: updatedTrailingStopStatus,
          updatedAt: Date.now(),
        };
      }));

      // Debounced refetch to pick up the new sl_modifications from the API
      // (the SL modification tree is populated from API data, not WebSocket)
      setTimeout(() => fetchOrders(), 3000);
    };

    // Handler for PnL correction from real trade data
    const handlePnLCorrected = (event: any) => {
      const data = event.data;
      if (!data?.chain_id) return;

      console.log('[TradeLifecycle] PnL corrected from real trade data', {
        chainId: data.chain_id,
        realizedPnl: data.realized_pnl,
        commission: data.commission,
      });

      // Direct state update - don't use fetchOrders which can overwrite close status
      setChains(prev => prev.map(chain => {
        if (chain.chainId !== data.chain_id) return chain;
        return {
          ...chain,
          pnl: data.realized_pnl ?? chain.pnl,
          totalFees: data.commission ?? chain.totalFees,
          realizedPnl: data.realized_pnl ?? chain.realizedPnl,
          positionState: chain.positionState ? {
            ...chain.positionState,
            realizedPnl: data.realized_pnl ?? chain.positionState.realizedPnl,
          } : undefined,
          updatedAt: Date.now(),
        };
      }));
    };

    // Subscribe to WebSocket events
    wsService.subscribe('CHAIN_UPDATE', handleChainUpdate);
    wsService.subscribe('ORDER_UPDATE', handleOrderUpdate);
    wsService.subscribe('TRADE_UPDATE', handleTradeUpdate); // For filled orders (SL/TP triggered)
    wsService.subscribe('POSITION_UPDATE', handlePositionUpdate);
    wsService.subscribe('PNL_UPDATE', handlePnlUpdate);
    wsService.subscribe('ORDER_SYNC', handleOrderSync);
    wsService.subscribe('CHAIN_CLOSED', handleChainClosed);
    wsService.subscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
    wsService.subscribe('POSITION_CREATED', handlePositionCreated); // For instant new position updates
    wsService.subscribe('SL_TP_PLACED', handleSLTPPlaced); // For SL/TP placed after position creation
    wsService.subscribe('PNL_CORRECTED', handlePnLCorrected); // For real PnL from trade data
    wsService.subscribe('TRAILING_SL_UPDATE', handleTrailingSLUpdate); // For R:R-based SL moves
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
      wsService.unsubscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
      wsService.unsubscribe('POSITION_CREATED', handlePositionCreated);
      wsService.unsubscribe('SL_TP_PLACED', handleSLTPPlaced);
      wsService.unsubscribe('PNL_CORRECTED', handlePnLCorrected);
      wsService.unsubscribe('TRAILING_SL_UPDATE', handleTrailingSLUpdate);
      wsService.offConnect(handleConnect);
      fallbackManager.unregisterFetchFunction(FALLBACK_KEY);
    };
  }, [autoRefresh, fetchOrders]);

  // Get unique symbols for filter
  const symbols = useMemo(() => {
    const symbolSet = new Set(chains.map(c => c.symbol).filter(Boolean) as string[]);
    return Array.from(symbolSet).sort();
  }, [chains]);

  // Apply filters and sort: active/partial first (newest first), then completed/cancelled (newest first)
  const filteredChains = useMemo(() => {
    const filtered = chains.filter((chain) => {
      // Active-only toggle filter
      if (activeOnly && chain.status !== 'active' && chain.status !== 'partial') return false;
      if (filters.mode !== 'all' && chain.modeCode !== filters.mode) return false;
      if (filters.status !== 'all' && chain.status !== filters.status) return false;
      if (filters.symbol !== 'all' && chain.symbol !== filters.symbol) return false;
      if (filters.side !== 'all' && chain.positionSide !== filters.side) return false;
      return true;
    });

    return filtered.sort((a, b) => {
      const isActiveA = a.status === 'active' || a.status === 'partial';
      const isActiveB = b.status === 'active' || b.status === 'partial';

      // Active/partial chains come first
      if (isActiveA && !isActiveB) return -1;
      if (!isActiveA && isActiveB) return 1;

      // Within same group, sort by creation date (stable - doesn't change on updates)
      const timeA = a.createdAt || 0;
      const timeB = b.createdAt || 0;
      return timeB - timeA;
    });
  }, [chains, filters, activeOnly]);

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

  // Reset filters (including date filters and active-only toggle)
  const resetFilters = () => {
    setFilters({
      mode: 'all',
      status: 'all',
      symbol: 'all',
      side: 'all',
      dateFrom: undefined,
      dateTo: undefined,
    });
    setActiveOnly(false);
  };

  // Calculate position stats from chains that have active positions
  // Must be before any early returns to follow React hooks rules
  // Note: API may return lowercase (active/partial) or uppercase (ACTIVE/PARTIAL) status
  // A chain has an active position if:
  // 1. It has a positionState with status ACTIVE/PARTIAL, OR
  // 2. The chain status is active/partial AND the entry order is FILLED (position exists but no positionState record)
  const positionStats = useMemo(() => {
    const activePositions = chains.filter(c => {
      // Entry must be filled for a position to exist
      // Skip chains where entry order is still pending (NEW status)
      const entryPending = c.entryOrder && c.entryOrder.status !== 'FILLED' && (c.entryOrder.executedQty || 0) === 0;
      if (entryPending) return false;

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
                      activeOnly={activeOnly}
                      onActiveOnlyChange={setActiveOnly}
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
                      {positionStats.positions.map((chain) => (
                        <PositionCard
                          key={chain.chainId}
                          chain={chain}
                          livePrice={livePrices.get(chain.symbol) || 0}
                          onChainRefresh={fetchOrders}
                        />
                      ))}
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
