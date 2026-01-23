import { useEffect, useState, useCallback, useRef } from 'react';
import { useFuturesStore, selectActivePositions } from '../store/futuresStore';
import {
  futuresApi,
  OrderChainWithState,
  ChainOrderInfo,
  PositionStateInfo,
  formatUSD,
  formatPrice,
  formatQuantity,
  getPositionColor,
} from '../services/futuresApi';
import {
  TrendingUp,
  TrendingDown,
  X,
  RefreshCw,
  Target,
  Shield,
  Edit2,
  Check,
  XCircle,
  Zap,
  AlertCircle,
  Wifi,
  WifiOff,
  ChevronDown,
  ChevronUp,
  CheckCircle,
  Activity,
  Clock,
  Hash,
} from 'lucide-react';
import { apiService } from '../services/api';
import { wsService } from '../services/websocket';
import type { FuturesOrder, FuturesPosition } from '../types/futures';

interface PositionOrders {
  take_profit_orders: FuturesOrder[];
  stop_loss_orders: FuturesOrder[];
  trailing_stop_orders: FuturesOrder[];
}

interface FuturesPositionsTableProps {
  onSymbolClick?: (symbol: string) => void;
}

// Mode display names
const MODE_DISPLAY: Record<string, { label: string; color: string }> = {
  ULT: { label: 'ULT', color: 'text-orange-400 bg-orange-900/30' },
  SCA: { label: 'SCA', color: 'text-yellow-400 bg-yellow-900/30' },
  SWI: { label: 'SWI', color: 'text-blue-400 bg-blue-900/30' },
  POS: { label: 'POS', color: 'text-purple-400 bg-purple-900/30' },
};

// Extract specific order from chain by type
function getOrderByType(orders: ChainOrderInfo[], type: string): ChainOrderInfo | undefined {
  return orders.find(o => o.order_type === type);
}

// Get TP orders sorted by level
function getTPOrders(orders: ChainOrderInfo[]): ChainOrderInfo[] {
  return orders
    .filter(o => o.order_type.startsWith('TP'))
    .sort((a, b) => {
      const levelA = parseInt(a.order_type.replace('TP', '')) || 0;
      const levelB = parseInt(b.order_type.replace('TP', '')) || 0;
      return levelA - levelB;
    });
}

// Determine current TP level based on filled TPs
function getCurrentTPLevel(tpOrders: ChainOrderInfo[]): number {
  let level = 0;
  for (const tp of tpOrders) {
    if (tp.status === 'FILLED') {
      const tpLevel = parseInt(tp.order_type.replace('TP', '')) || 0;
      if (tpLevel > level) level = tpLevel;
    }
  }
  return level;
}

export default function FuturesPositionsTable({ onSymbolClick }: FuturesPositionsTableProps = {}) {
  const { fetchPositions, closePosition, isLoading } = useFuturesStore();

  // Binance positions for real-time mark prices
  const binancePositions = useFuturesStore(selectActivePositions);

  // Order chains from the NEW system (primary data source)
  const [orderChains, setOrderChains] = useState<OrderChainWithState[]>([]);
  const [loadingChains, setLoadingChains] = useState(false);
  const [lastRefreshTime, setLastRefreshTime] = useState<Date | null>(null);

  // Expanded position tracking
  const [expandedPosition, setExpandedPosition] = useState<string | null>(null);

  const [positionOrders, setPositionOrders] = useState<Record<string, PositionOrders>>({});
  const [editingTPSL, setEditingTPSL] = useState<string | null>(null);
  const [tpValue, setTpValue] = useState<string>('');
  const [slValue, setSlValue] = useState<string>('');
  const [savingTPSL, setSavingTPSL] = useState(false);
  const [tradingMode, setTradingMode] = useState<'live' | 'paper' | null>(null);
  const [wsConnected, setWsConnected] = useState(() => wsService.isConnected());

  // Ref for refresh interval
  const refreshIntervalRef = useRef<NodeJS.Timeout | null>(null);

  // Auto-select content on focus
  const handleInputFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    e.target.select();
  };

  // Fetch order chains from NEW system
  const fetchOrderChains = useCallback(async () => {
    try {
      setLoadingChains(true);
      const response = await futuresApi.getOrderChainsWithState({ status: 'active' });
      // Also include partial positions
      const partialResponse = await futuresApi.getOrderChainsWithState({ status: 'partial' });

      const allChains = [
        ...(response.chains || []),
        ...(partialResponse.chains || []),
      ];

      setOrderChains(allChains);
      setLastRefreshTime(new Date());
    } catch (err) {
      console.error('Error fetching order chains:', err);
    } finally {
      setLoadingChains(false);
    }
  }, []);

  // Fetch trading mode
  useEffect(() => {
    const fetchTradingMode = async () => {
      try {
        const modeData = await apiService.getTradingMode();
        setTradingMode(modeData.mode === 'live' ? 'live' : 'paper');
      } catch (error) {
        console.error('Error fetching trading mode:', error);
      }
    };
    fetchTradingMode();
  }, []);

  // WebSocket-driven updates with real-time refresh
  useEffect(() => {
    const handlePositionUpdate = () => {
      // Refresh both Binance positions (for mark price) and order chains
      fetchPositions();
      fetchOrderChains();
    };

    const handleOrderUpdate = () => {
      // ORDER_UPDATE triggers order chain refresh
      fetchOrderChains();

      // Also refresh position orders for TP/SL display
      if (orderChains.length > 0) {
        const fetchAllPositionOrders = async () => {
          const orders: Record<string, PositionOrders> = {};
          for (const chain of orderChains) {
            try {
              const data = await futuresApi.getPositionOrders(chain.symbol);
              orders[chain.symbol] = {
                take_profit_orders: data.take_profit_orders || [],
                stop_loss_orders: data.stop_loss_orders || [],
                trailing_stop_orders: data.trailing_stop_orders || [],
              };
            } catch (err) {
              console.error(`Error fetching orders for ${chain.symbol}:`, err);
            }
          }
          setPositionOrders(orders);
        };
        fetchAllPositionOrders();
      }
    };

    const handleConnect = () => {
      setWsConnected(true);
      // Full refresh on reconnect
      fetchOrderChains();
      fetchPositions();
    };

    const handleDisconnect = () => {
      setWsConnected(false);
    };

    // Subscribe to WebSocket events
    wsService.subscribe('POSITION_UPDATE', handlePositionUpdate);
    wsService.subscribe('PNL_UPDATE', handlePositionUpdate);
    wsService.subscribe('ORDER_UPDATE', handleOrderUpdate);
    wsService.subscribe('CHAIN_UPDATE', handlePositionUpdate);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    // Initialize
    setWsConnected(wsService.isConnected());
    fetchOrderChains();
    fetchPositions();

    // Set up 1-second refresh interval for real-time mark prices
    refreshIntervalRef.current = setInterval(() => {
      fetchPositions(); // Get latest mark prices from Binance
    }, 1000);

    return () => {
      wsService.unsubscribe('POSITION_UPDATE', handlePositionUpdate);
      wsService.unsubscribe('PNL_UPDATE', handlePositionUpdate);
      wsService.unsubscribe('ORDER_UPDATE', handleOrderUpdate);
      wsService.unsubscribe('CHAIN_UPDATE', handlePositionUpdate);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);

      if (refreshIntervalRef.current) {
        clearInterval(refreshIntervalRef.current);
      }
    };
  }, [fetchPositions, fetchOrderChains, orderChains.length]);

  // Fetch orders for each position
  useEffect(() => {
    const fetchOrders = async () => {
      const orders: Record<string, PositionOrders> = {};
      for (const chain of orderChains) {
        try {
          const data = await futuresApi.getPositionOrders(chain.symbol);
          orders[chain.symbol] = {
            take_profit_orders: data.take_profit_orders || [],
            stop_loss_orders: data.stop_loss_orders || [],
            trailing_stop_orders: data.trailing_stop_orders || [],
          };
        } catch (err) {
          console.error(`Error fetching orders for ${chain.symbol}:`, err);
        }
      }
      setPositionOrders(orders);
    };

    if (orderChains.length > 0) {
      fetchOrders();
    }
  }, [orderChains]);

  const handleClosePosition = async (symbol: string) => {
    if (window.confirm(`Are you sure you want to close your ${symbol} position?`)) {
      await closePosition(symbol);
      // Refresh after close
      setTimeout(() => {
        fetchOrderChains();
        fetchPositions();
      }, 500);
    }
  };

  const startEditTPSL = (symbol: string, currentTP?: number, currentSL?: number) => {
    setEditingTPSL(symbol);
    setTpValue(currentTP ? currentTP.toString() : '');
    setSlValue(currentSL ? currentSL.toString() : '');
  };

  const cancelEditTPSL = () => {
    setEditingTPSL(null);
    setTpValue('');
    setSlValue('');
  };

  const saveTPSL = async (symbol: string, positionSide: string) => {
    setSavingTPSL(true);
    try {
      await futuresApi.setPositionTPSL(
        symbol,
        positionSide,
        tpValue ? parseFloat(tpValue) : undefined,
        slValue ? parseFloat(slValue) : undefined
      );
      // Refresh orders
      const data = await futuresApi.getPositionOrders(symbol);
      setPositionOrders(prev => ({
        ...prev,
        [symbol]: {
          take_profit_orders: data.take_profit_orders || [],
          stop_loss_orders: data.stop_loss_orders || [],
          trailing_stop_orders: data.trailing_stop_orders || [],
        }
      }));
      setEditingTPSL(null);
    } catch (err) {
      console.error('Error saving TP/SL:', err);
      alert('Failed to save TP/SL orders');
    } finally {
      setSavingTPSL(false);
    }
  };

  // Get real-time price from Binance position
  const getRealTimePrice = (symbol: string): number => {
    const binancePos = binancePositions.find(bp => bp.symbol === symbol);
    return binancePos?.markPrice || 0;
  };

  // Calculate unrealized PnL
  const calculateUnrealizedPnL = (
    positionState: PositionStateInfo | undefined,
    positionSide: string,
    realTimePrice: number
  ): number => {
    if (!positionState || !realTimePrice || positionState.entry_price <= 0) {
      return 0;
    }

    const qty = positionState.remaining_quantity;
    if (qty <= 0) return 0;

    if (positionSide === 'LONG') {
      return (realTimePrice - positionState.entry_price) * qty;
    } else {
      return (positionState.entry_price - realTimePrice) * qty;
    }
  };

  // Calculate total PnL from all positions
  const calculateTotalPnL = (): number => {
    return orderChains.reduce((total, chain) => {
      const realTimePrice = getRealTimePrice(chain.symbol);
      const unrealizedPnl = calculateUnrealizedPnL(chain.position_state, chain.position_side, realTimePrice);
      const realizedPnl = chain.position_state?.realized_pnl || 0;
      return total + realizedPnl + unrealizedPnl;
    }, 0);
  };

  // Show empty state if no positions
  if (orderChains.length === 0 && binancePositions.length === 0) {
    return (
      <div className="bg-gray-900 rounded-lg border border-gray-700 p-6">
        <div className="text-center text-gray-400">
          <TrendingUp className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p>No open positions</p>
          <p className="text-sm mt-1">Your futures positions will appear here</p>
          {lastRefreshTime && (
            <p className="text-xs text-gray-500 mt-2">
              Last refresh: {lastRefreshTime.toLocaleTimeString()}
            </p>
          )}
        </div>
      </div>
    );
  }

  const totalPnL = calculateTotalPnL();

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-3">
          <span className="font-semibold">Positions</span>
          {wsConnected ? (
            <Wifi className="w-3 h-3 text-green-500" title="Real-time updates via WebSocket" />
          ) : (
            <WifiOff className="w-3 h-3 text-yellow-500" title="WebSocket disconnected - using polling" />
          )}
          <span className="text-sm text-gray-400">({orderChains.length})</span>
          {loadingChains && (
            <RefreshCw className="w-3 h-3 text-blue-400 animate-spin" />
          )}
        </div>
        <div className="flex items-center gap-4">
          <span className={`text-sm ${getPositionColor(totalPnL)}`}>
            Total PnL: {formatUSD(totalPnL)}
          </span>
          <button
            onClick={fetchOrderChains}
            disabled={loadingChains}
            className="p-1.5 hover:bg-gray-700 rounded"
            title="Refresh positions"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${loadingChains ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Positions List */}
      <div className="divide-y divide-gray-800">
        {orderChains.map((chain) => {
          const positionState = chain.position_state;
          const realTimePrice = getRealTimePrice(chain.symbol);
          const unrealizedPnl = calculateUnrealizedPnL(positionState, chain.position_side, realTimePrice);
          const realizedPnl = positionState?.realized_pnl || 0;
          const totalPnl = realizedPnl + unrealizedPnl;

          const entryPrice = positionState?.entry_price || 0;
          const entryQty = positionState?.entry_quantity || 0;
          const remainingQty = positionState?.remaining_quantity || 0;

          const pnlPercent = entryPrice > 0 && entryQty > 0
            ? (unrealizedPnl / (entryPrice * entryQty)) * 100
            : 0;

          const isExpanded = expandedPosition === chain.chain_id;
          const isLong = chain.position_side === 'LONG';

          // Get orders from chain
          const slOrder = getOrderByType(chain.orders, 'SL');
          const tpOrders = getTPOrders(chain.orders);
          const currentTPLevel = getCurrentTPLevel(tpOrders);
          const nextTP = tpOrders.find(tp => tp.status !== 'FILLED');

          // Calculate expected profit/loss
          const slPrice = slOrder?.stop_price || slOrder?.price || 0;
          const nextTPPrice = nextTP?.stop_price || nextTP?.price || 0;

          const expectedProfit = nextTPPrice > 0 && remainingQty > 0
            ? isLong
              ? (nextTPPrice - entryPrice) * remainingQty
              : (entryPrice - nextTPPrice) * remainingQty
            : 0;

          const expectedLoss = slPrice > 0 && remainingQty > 0
            ? isLong
              ? (entryPrice - slPrice) * remainingQty
              : (slPrice - entryPrice) * remainingQty
            : 0;

          const riskReward = expectedLoss > 0 ? expectedProfit / expectedLoss : 0;

          const orders = positionOrders[chain.symbol];
          const binanceTPOrder = orders?.take_profit_orders?.[0];
          const binanceSLOrder = orders?.stop_loss_orders?.[0];
          const trailingOrder = orders?.trailing_stop_orders?.[0];

          const isEditing = editingTPSL === chain.symbol;
          const modeInfo = MODE_DISPLAY[chain.mode_code] || { label: chain.mode_code || 'UNK', color: 'text-gray-400 bg-gray-900/30' };

          // Modification counts
          const slMods = chain.modification_counts?.SL || 0;
          const totalTPMods = (chain.modification_counts?.TP1 || 0) +
                             (chain.modification_counts?.TP2 || 0) +
                             (chain.modification_counts?.TP3 || 0);

          return (
            <div key={chain.chain_id} className="hover:bg-gray-800/30">
              {/* Main Row */}
              <div
                className="px-4 py-3 cursor-pointer"
                onClick={() => setExpandedPosition(isExpanded ? null : chain.chain_id)}
              >
                <div className="flex items-center justify-between">
                  {/* Left: Symbol and Info */}
                  <div className="flex items-center gap-3">
                    {isLong ? (
                      <TrendingUp className="w-5 h-5 text-green-500" />
                    ) : (
                      <TrendingDown className="w-5 h-5 text-red-500" />
                    )}

                    <div>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            onSymbolClick?.(chain.symbol);
                          }}
                          className="font-semibold text-white hover:text-yellow-400 transition-colors"
                        >
                          {chain.symbol.replace('USDT', '')}
                        </button>

                        <span className={`text-xs font-bold ${isLong ? 'text-green-400' : 'text-red-400'}`}>
                          {chain.position_side}
                        </span>

                        {/* Mode Badge */}
                        <span className={`text-xs uppercase font-bold px-1.5 py-0.5 rounded ${modeInfo.color}`}>
                          {modeInfo.label}
                        </span>

                        {/* Chain ID Badge */}
                        <span className="px-1.5 py-0.5 bg-gray-700/50 text-gray-400 rounded text-xs flex items-center gap-0.5">
                          <Hash className="w-3 h-3" />
                          {chain.chain_id.split('-').slice(-1)[0]}
                        </span>

                        {/* Trailing Badge */}
                        {trailingOrder && (
                          <span className="px-1.5 py-0.5 bg-cyan-900/50 text-cyan-400 rounded text-xs flex items-center gap-0.5">
                            <Activity className="w-3 h-3" />
                            TRAIL
                          </span>
                        )}

                        {/* TP Progress Badge */}
                        {currentTPLevel > 0 && (
                          <span className="px-1.5 py-0.5 bg-green-900/50 text-green-400 rounded text-xs font-medium flex items-center gap-1">
                            <Target className="w-3 h-3" />
                            TP{currentTPLevel}
                            {entryQty > 0 && remainingQty > 0 && (
                              <span className="text-green-300">
                                ({Math.round((1 - remainingQty / entryQty) * 100)}%)
                              </span>
                            )}
                          </span>
                        )}

                        {/* Status Badge */}
                        {chain.status === 'partial' && (
                          <span className="px-1.5 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-xs">
                            PARTIAL
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-2 text-xs text-gray-400 mt-0.5">
                        <span>{formatQuantity(remainingQty)}/{formatQuantity(entryQty)}</span>
                        <span>•</span>
                        <span>Entry: ${formatPrice(entryPrice)}</span>
                        {realTimePrice > 0 && (
                          <>
                            <span>•</span>
                            <span className="text-yellow-400">Now: ${formatPrice(realTimePrice)}</span>
                          </>
                        )}
                        {(slMods > 0 || totalTPMods > 0) && (
                          <>
                            <span>•</span>
                            <span className="text-gray-500">
                              Mods: {slMods > 0 && `SL×${slMods}`} {totalTPMods > 0 && `TP×${totalTPMods}`}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Right: PnL and Expand */}
                  <div className="flex items-center gap-4">
                    {/* Trading Mode */}
                    {tradingMode === 'live' ? (
                      <div className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-bold bg-green-500/20 border border-green-500 text-green-400">
                        <Zap className="w-3 h-3" />
                        LIVE
                      </div>
                    ) : tradingMode === 'paper' ? (
                      <div className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-bold bg-yellow-500/20 border border-yellow-500 text-yellow-400">
                        <AlertCircle className="w-3 h-3" />
                        PAPER
                      </div>
                    ) : null}

                    {/* PnL Display */}
                    <div className="text-right">
                      <div className={`font-bold ${getPositionColor(totalPnl)}`}>
                        {formatUSD(totalPnl)}
                      </div>
                      <div className={`text-xs ${getPositionColor(pnlPercent)}`}>
                        ({pnlPercent >= 0 ? '+' : ''}{pnlPercent.toFixed(2)}%)
                      </div>
                    </div>

                    {/* Expand Icon */}
                    {isExpanded ? (
                      <ChevronUp className="w-5 h-5 text-gray-400" />
                    ) : (
                      <ChevronDown className="w-5 h-5 text-gray-400" />
                    )}
                  </div>
                </div>
              </div>

              {/* Expanded Details */}
              {isExpanded && (
                <div className="px-4 pb-4 space-y-3 border-t border-gray-700/50 bg-gray-800/20">
                  {/* Price Info Grid */}
                  <div className="grid grid-cols-6 gap-2 pt-3">
                    <div className="bg-gray-700/50 p-2 rounded text-center">
                      <div className="text-gray-500 text-xs">Entry</div>
                      <div className="text-white font-mono">${formatPrice(entryPrice)}</div>
                    </div>
                    <div className="bg-gray-700/50 p-2 rounded text-center">
                      <div className="text-gray-500 text-xs">Current</div>
                      <div className="text-yellow-400 font-mono font-bold">${formatPrice(realTimePrice)}</div>
                    </div>
                    <div className="bg-gray-700/50 p-2 rounded text-center">
                      <div className="text-gray-500 text-xs">SL</div>
                      <div className="font-mono text-red-400">
                        ${formatPrice(slPrice)}
                        {slMods > 0 && <span className="text-xs text-gray-500 ml-1">(×{slMods})</span>}
                      </div>
                    </div>
                    <div className="bg-gray-700/50 p-2 rounded text-center">
                      <div className="text-gray-500 text-xs">Remaining Qty</div>
                      <div className="text-white">{formatQuantity(remainingQty)}</div>
                    </div>
                    <div className="bg-gray-700/50 p-2 rounded text-center">
                      <div className="text-gray-500 text-xs">Realized</div>
                      <div className={getPositionColor(realizedPnl)}>
                        {formatUSD(realizedPnl)}
                      </div>
                    </div>
                    <div className="bg-gray-700/50 p-2 rounded text-center">
                      <div className="text-gray-500 text-xs">Entry Fees</div>
                      <div className="text-gray-400">
                        {formatUSD(positionState?.entry_fees || 0)}
                      </div>
                    </div>
                  </div>

                  {/* Expected Profit/Loss Section */}
                  <div className="grid grid-cols-3 gap-2">
                    <div className="bg-green-900/30 p-2 rounded border border-green-800/50">
                      <div className="text-gray-400 text-xs">Expected Profit (Next TP)</div>
                      <div className="text-green-400 font-bold">+{formatUSD(expectedProfit)}</div>
                      <div className="text-green-300/70 text-xs">@ ${formatPrice(nextTPPrice)}</div>
                    </div>
                    <div className="bg-red-900/30 p-2 rounded border border-red-800/50">
                      <div className="text-gray-400 text-xs">Expected Loss (SL)</div>
                      <div className="text-red-400 font-bold">-{formatUSD(Math.abs(expectedLoss))}</div>
                      <div className="text-red-300/70 text-xs">@ ${formatPrice(slPrice)}</div>
                    </div>
                    <div className="bg-gray-700/50 p-2 rounded border border-gray-600/50">
                      <div className="text-gray-400 text-xs">Risk/Reward</div>
                      <div className={`font-bold ${riskReward >= 1 ? 'text-green-400' : 'text-yellow-400'}`}>
                        1:{riskReward.toFixed(2)}
                      </div>
                      <div className="text-gray-500 text-xs">
                        {riskReward >= 2 ? 'Excellent' : riskReward >= 1 ? 'Good' : 'Poor'}
                      </div>
                    </div>
                  </div>

                  {/* TP Progression */}
                  {tpOrders.length > 0 && (
                    <div className="space-y-2">
                      <div className="text-gray-500 text-xs font-medium">Take Profit Progression</div>
                      <div className="flex items-center gap-1 flex-wrap">
                        {tpOrders.map((tp, idx) => {
                          const isHit = tp.status === 'FILLED';
                          const tpLevel = parseInt(tp.order_type.replace('TP', '')) || idx + 1;
                          const isNext = currentTPLevel + 1 === tpLevel;
                          const tpMods = chain.modification_counts?.[tp.order_type] || 0;

                          return (
                            <div key={tp.order_type} className="flex items-center gap-1">
                              <div
                                className={`px-2 py-1 rounded text-xs font-bold flex items-center gap-1 ${
                                  isHit
                                    ? 'bg-green-900/60 text-green-300 ring-1 ring-green-600'
                                    : isNext
                                      ? 'bg-yellow-900/60 text-yellow-300 ring-1 ring-yellow-600 animate-pulse'
                                      : 'bg-gray-700/40 text-gray-400'
                                }`}
                              >
                                <span>{tp.order_type}</span>
                                {isHit && <CheckCircle className="w-3 h-3" />}
                                {isNext && !isHit && <AlertCircle className="w-3 h-3" />}
                                {tpMods > 0 && <span className="text-gray-400">(×{tpMods})</span>}
                              </div>
                              {idx < tpOrders.length - 1 && (
                                <span className={`text-xs font-bold ${
                                  isHit ? 'text-green-400' : 'text-gray-600'
                                }`}>→</span>
                              )}
                            </div>
                          );
                        })}
                      </div>

                      {/* TP Details */}
                      <div className="grid grid-cols-4 gap-2">
                        {tpOrders.map((tp) => {
                          const isHit = tp.status === 'FILLED';
                          const tpLevel = parseInt(tp.order_type.replace('TP', '')) || 1;
                          const isNext = currentTPLevel + 1 === tpLevel;
                          const tpPrice = tp.stop_price || tp.price || 0;

                          return (
                            <div
                              key={tp.order_type}
                              className={`text-xs p-1.5 rounded text-center ${
                                isHit
                                  ? 'bg-green-900/30 text-green-300'
                                  : isNext
                                    ? 'bg-yellow-900/30 text-yellow-300'
                                    : 'bg-gray-700/30 text-gray-400'
                              }`}
                            >
                              <div className="font-bold">{tp.order_type}</div>
                              <div>${formatPrice(tpPrice)}</div>
                              <div className="text-[10px]">Qty: {formatQuantity(tp.quantity)}</div>
                              {isHit && <div className="text-green-400">✓ HIT</div>}
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  )}

                  {/* Chain ID Info */}
                  <div className="text-xs text-gray-500 bg-gray-700/30 p-2 rounded">
                    <span className="font-medium">Chain ID:</span> {chain.chain_id}
                    <span className="mx-2">•</span>
                    <span className="font-medium">Created:</span> {new Date(chain.created_at).toLocaleString()}
                  </div>

                  {/* Actions */}
                  <div className="flex items-center justify-between pt-2 border-t border-gray-700/50">
                    <div className="flex items-center gap-2">
                      {/* Edit TP/SL */}
                      {isEditing ? (
                        <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
                          <input
                            type="number"
                            value={tpValue}
                            onChange={(e) => setTpValue(e.target.value)}
                            onFocus={handleInputFocus}
                            placeholder="TP"
                            className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs"
                          />
                          <input
                            type="number"
                            value={slValue}
                            onChange={(e) => setSlValue(e.target.value)}
                            onFocus={handleInputFocus}
                            placeholder="SL"
                            className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs"
                          />
                          <button
                            onClick={() => saveTPSL(chain.symbol, chain.position_side)}
                            disabled={savingTPSL}
                            className="px-2 py-1 bg-green-600 hover:bg-green-500 text-white rounded text-xs"
                          >
                            Save
                          </button>
                          <button
                            onClick={cancelEditTPSL}
                            className="px-2 py-1 bg-gray-600 hover:bg-gray-500 text-white rounded text-xs"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            startEditTPSL(chain.symbol, binanceTPOrder?.stopPrice, binanceSLOrder?.stopPrice);
                          }}
                          className="px-3 py-1.5 bg-blue-600/20 hover:bg-blue-600/30 text-blue-400 rounded text-xs flex items-center gap-1"
                        >
                          <Edit2 className="w-3 h-3" />
                          Edit TP/SL
                        </button>
                      )}

                      {/* Trailing Stop Info */}
                      {trailingOrder && (
                        <span className="text-xs text-purple-400">
                          TS: ${formatPrice(trailingOrder.stopPrice)}
                        </span>
                      )}
                    </div>

                    {/* Close Button */}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleClosePosition(chain.symbol);
                      }}
                      disabled={isLoading}
                      className="px-3 py-1.5 bg-red-600/20 hover:bg-red-600/30 text-red-400 rounded text-xs flex items-center gap-1"
                    >
                      <X className="w-3 h-3" />
                      Close Position
                    </button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Legend */}
      <div className="px-4 py-2 bg-gray-800/50 border-t border-gray-700 flex items-center flex-wrap gap-4 text-xs text-gray-500">
        <div className="flex items-center gap-1">
          <Target className="w-3 h-3 text-green-500" />
          <span>Take Profit</span>
        </div>
        <div className="flex items-center gap-1">
          <Shield className="w-3 h-3 text-red-500" />
          <span>Stop Loss</span>
        </div>
        <div className="flex items-center gap-1">
          <Activity className="w-3 h-3 text-cyan-400" />
          <span>Trailing</span>
        </div>
        <span className="text-gray-600">|</span>
        <div className="flex items-center gap-1">
          <Hash className="w-3 h-3 text-gray-500" />
          <span>Chain ID</span>
        </div>
        {lastRefreshTime && (
          <>
            <span className="text-gray-600">|</span>
            <div className="flex items-center gap-1">
              <Clock className="w-3 h-3 text-gray-500" />
              <span>Refreshed: {lastRefreshTime.toLocaleTimeString()}</span>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
