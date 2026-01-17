import { useEffect, useState, useCallback, useMemo } from 'react';
import { useFuturesStore, selectActivePositions } from '../store/futuresStore';
import { futuresApi } from '../services/futuresApi';
import {
  formatUSD,
  formatPrice,
  formatPercent,
  formatQuantity,
  calculateROE,
  getPositionSideLabel,
  getPositionColor,
} from '../services/futuresApi';
import {
  TrendingUp,
  TrendingDown,
  X,
  RefreshCw,
  AlertTriangle,
  Target,
  Shield,
  Edit2,
  Check,
  XCircle,
  Zap,
  AlertCircle,
  Brain,
  User,
  Wifi,
  WifiOff,
} from 'lucide-react';
import { apiService } from '../services/api';
import { wsService } from '../services/websocket';
import type { FuturesOrder, FuturesPosition } from '../types/futures';
import type { WSEvent } from '../types';

interface PositionOrders {
  take_profit_orders: FuturesOrder[];
  stop_loss_orders: FuturesOrder[];
  trailing_stop_orders: FuturesOrder[];
}

interface FuturesPositionsTableProps {
  onSymbolClick?: (symbol: string) => void;
}

export default function FuturesPositionsTable({ onSymbolClick }: FuturesPositionsTableProps = {}) {
  const {
    fetchPositions,
    closePosition,
    isLoading,
    updatePositions,
  } = useFuturesStore();

  const activePositions = useFuturesStore(selectActivePositions);

  const [positionOrders, setPositionOrders] = useState<Record<string, PositionOrders>>({});
  const [editingTPSL, setEditingTPSL] = useState<string | null>(null);
  const [tpValue, setTpValue] = useState<string>('');
  const [slValue, setSlValue] = useState<string>('');
  const [savingTPSL, setSavingTPSL] = useState(false);
  const [editingROI, setEditingROI] = useState<string | null>(null);
  const [roiValue, setRoiValue] = useState<string>('');
  const [saveForFuture, setSaveForFuture] = useState<boolean>(false);
  const [savingROI, setSavingROI] = useState(false);
  const [tradingMode, setTradingMode] = useState<'live' | 'paper' | null>(null);
  const [tradeSources, setTradeSources] = useState<Record<string, string>>({});
  const [wsConnected, setWsConnected] = useState(() => wsService.isConnected());

  // Real-time mark prices from WebSocket for instant PnL updates (like Binance app)
  const [liveMarkPrices, setLiveMarkPrices] = useState<Record<string, number>>({});

  // Calculate real-time PnL using Binance's exact formula:
  // unrealizedPnL = (markPrice - entryPrice) * positionAmt
  // For SHORT positions, positionAmt is negative, so formula works for both
  const calculateLivePnL = useCallback((position: FuturesPosition): { markPrice: number; pnl: number; roe: number } => {
    // Use live mark price if available, otherwise fall back to position's stored markPrice
    const currentMarkPrice = liveMarkPrices[position.symbol] ?? position.markPrice;

    // Binance PnL formula: (markPrice - entryPrice) * positionAmt
    const pnl = (currentMarkPrice - position.entryPrice) * position.positionAmt;

    // ROE formula: unrealizedPnL / initialMargin * 100
    // initialMargin = |positionAmt| * entryPrice / leverage
    const initialMargin = Math.abs(position.positionAmt) * position.entryPrice / position.leverage;
    const roe = initialMargin > 0 ? (pnl / initialMargin) * 100 : 0;

    return { markPrice: currentMarkPrice, pnl, roe };
  }, [liveMarkPrices]);

  // Calculate total real-time PnL across all positions
  const totalUnrealizedPnl = useMemo(() => {
    return activePositions.reduce((total, position) => {
      const { pnl } = calculateLivePnL(position);
      return total + pnl;
    }, 0);
  }, [activePositions, calculateLivePnL]);

  // Auto-select content on focus for easier value replacement
  const handleInputFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    e.target.select();
  };

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

  // Fetch trade sources for positions
  useEffect(() => {
    const fetchTradeSources = async () => {
      try {
        const data = await futuresApi.getPositionTradeSources();
        setTradeSources(data.sources || {});
      } catch (error) {
        console.error('Error fetching trade sources:', error);
      }
    };
    if (activePositions.length > 0) {
      fetchTradeSources();
    }
  }, [activePositions]);

  // State for custom ROI from Ginie positions
  const [roiFromGinie, setRoiFromGinie] = useState<Record<string, number | null>>({});

  // Real-time mark price updates from WebSocket (updates every ~1 second like Binance app)
  useEffect(() => {
    const handleMarkPriceUpdate = (event: WSEvent) => {
      const symbol = event.data?.symbol as string;
      const markPriceStr = event.data?.markPrice as string;

      if (symbol && markPriceStr) {
        const markPrice = parseFloat(markPriceStr);
        if (!isNaN(markPrice)) {
          setLiveMarkPrices(prev => ({
            ...prev,
            [symbol]: markPrice,
          }));
        }
      }
    };

    wsService.subscribe('FUTURES_MARK_PRICE_UPDATE', handleMarkPriceUpdate);

    return () => {
      wsService.unsubscribe('FUTURES_MARK_PRICE_UPDATE', handleMarkPriceUpdate);
    };
  }, []);

  // WebSocket-driven position updates
  // Note: Fallback polling is handled centrally by FallbackPollingManager in App.tsx (Story 12.9)
  useEffect(() => {
    const handlePositionUpdate = () => {
      // Always refresh positions from API on any position update
      // This ensures we have complete and accurate state including closures
      fetchPositions();
    };

    const handleOrderUpdate = () => {
      // ORDER_UPDATE triggers position orders refresh to update TP/SL display
      if (activePositions.length > 0) {
        const fetchAllPositionOrders = async () => {
          const orders: Record<string, PositionOrders> = {};
          for (const position of activePositions) {
            try {
              const data = await futuresApi.getPositionOrders(position.symbol);
              orders[position.symbol] = {
                take_profit_orders: data.take_profit_orders || [],
                stop_loss_orders: data.stop_loss_orders || [],
                trailing_stop_orders: data.trailing_stop_orders || [],
              };
            } catch (err) {
              console.error(`Error fetching orders for ${position.symbol}:`, err);
            }
          }
          setPositionOrders(orders);
        };
        fetchAllPositionOrders();
      }
    };

    const handleConnect = () => {
      setWsConnected(true);
      // Data sync on reconnect is handled by FallbackPollingManager.syncAll() in App.tsx
      fetchPositions();
    };

    const handleDisconnect = () => {
      setWsConnected(false);
      // Fallback polling is handled by FallbackPollingManager in App.tsx
    };

    // Subscribe to WebSocket events
    wsService.subscribe('POSITION_UPDATE', handlePositionUpdate);
    wsService.subscribe('ORDER_UPDATE', handleOrderUpdate);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    // Initialize connection state
    setWsConnected(wsService.isConnected());

    // Initial fetch on mount
    fetchPositions();

    return () => {
      wsService.unsubscribe('POSITION_UPDATE', handlePositionUpdate);
      wsService.unsubscribe('ORDER_UPDATE', handleOrderUpdate);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);
    };
  }, [fetchPositions, activePositions]);

  // Fetch Ginie positions to get custom ROI values - triggered by position updates
  const fetchGinieROI = useCallback(async () => {
    try {
      const data = await futuresApi.getGinieAutopilotPositions();
      const roiMap: Record<string, number | null> = {};
      if (data.positions && Array.isArray(data.positions)) {
        for (const pos of data.positions) {
          if ((pos as any).custom_roi_percent !== undefined) {
            roiMap[pos.symbol] = (pos as any).custom_roi_percent;
          }
        }
      }
      setRoiFromGinie(roiMap);
    } catch (err) {
      console.error('Error fetching Ginie ROI:', err);
    }
  }, []);

  // Fetch Ginie ROI when positions change (triggered by WebSocket updates)
  useEffect(() => {
    if (activePositions.length > 0) {
      fetchGinieROI();
    }
  }, [activePositions, fetchGinieROI]);

  // Fetch orders for each position - triggered by position updates (via WebSocket)
  useEffect(() => {
    const fetchOrders = async () => {
      const orders: Record<string, PositionOrders> = {};
      for (const position of activePositions) {
        try {
          const data = await futuresApi.getPositionOrders(position.symbol);
          orders[position.symbol] = {
            take_profit_orders: data.take_profit_orders || [],
            stop_loss_orders: data.stop_loss_orders || [],
            trailing_stop_orders: data.trailing_stop_orders || [],
          };
        } catch (err) {
          console.error(`Error fetching orders for ${position.symbol}:`, err);
        }
      }
      setPositionOrders(orders);
    };

    if (activePositions.length > 0) {
      fetchOrders();
    }
  }, [activePositions]);

  const handleClosePosition = async (symbol: string) => {
    if (window.confirm(`Are you sure you want to close your ${symbol} position?`)) {
      await closePosition(symbol);
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

  const startEditROI = (symbol: string, currentROI?: number) => {
    setEditingROI(symbol);
    setRoiValue(currentROI ? currentROI.toString() : '');
    setSaveForFuture(false);
  };

  const cancelEditROI = () => {
    setEditingROI(null);
    setRoiValue('');
    setSaveForFuture(false);
  };

  const saveROI = async (symbol: string) => {
    setSavingROI(true);
    try {
      const roiPercent = roiValue ? parseFloat(roiValue) : 0;

      // Validation
      if (roiPercent < 0 || roiPercent > 1000) {
        alert('ROI % must be between 0-1000');
        return;
      }

      await futuresApi.setPositionROITarget(symbol, roiPercent, saveForFuture);

      // Refresh positions to get updated data
      await fetchPositions();

      setEditingROI(null);
      setRoiValue('');
      setSaveForFuture(false);
    } catch (err) {
      console.error('Error saving ROI target:', err);
      alert('Failed to save ROI target');
    } finally {
      setSavingROI(false);
    }
  };

  if (activePositions.length === 0) {
    return (
      <div className="bg-gray-900 rounded-lg border border-gray-700 p-6">
        <div className="text-center text-gray-400">
          <TrendingUp className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p>No open positions</p>
          <p className="text-sm mt-1">Your futures positions will appear here</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-3">
          <span className="font-semibold">Positions</span>
          {wsConnected ? (
            <Wifi className="w-3 h-3 text-green-500" title="Real-time updates via WebSocket" />
          ) : (
            <WifiOff className="w-3 h-3 text-yellow-500" title="WebSocket disconnected - using 60s fallback polling" />
          )}
          <span className="text-sm text-gray-400">({activePositions.length})</span>
        </div>
        <div className="flex items-center gap-4">
          <span className={`text-sm ${getPositionColor(totalUnrealizedPnl)}`}>
            Total PnL: {formatUSD(totalUnrealizedPnl)}
          </span>
          <button
            onClick={() => fetchPositions()}
            className="p-1.5 hover:bg-gray-700 rounded"
            title="Refresh positions"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 border-b border-gray-700">
              <th className="text-left py-3 px-4 font-medium">Symbol</th>
              <th className="text-center py-3 px-2 font-medium">Mode</th>
              <th className="text-left py-3 px-4 font-medium">Size</th>
              <th className="text-right py-3 px-4 font-medium">Entry</th>
              <th className="text-right py-3 px-4 font-medium">Mark</th>
              <th className="text-right py-3 px-4 font-medium">Liq.</th>
              <th className="text-center py-3 px-4 font-medium">
                <div className="flex items-center justify-center gap-1">
                  <Target className="w-3 h-3 text-green-500" />
                  TP
                </div>
              </th>
              <th className="text-center py-3 px-4 font-medium">
                <div className="flex items-center justify-center gap-1">
                  <Shield className="w-3 h-3 text-red-500" />
                  SL
                </div>
              </th>
              <th className="text-right py-3 px-4 font-medium">PnL (ROE%)</th>
              <th className="text-center py-3 px-4 font-medium">
                <div className="flex items-center justify-center gap-1">
                  <TrendingUp className="w-3 h-3 text-yellow-500" />
                  ROI Target %
                </div>
              </th>
              <th className="text-center py-3 px-4 font-medium">Actions</th>
            </tr>
          </thead>
          <tbody>
            {activePositions.map((position) => {
              const { label: sideLabel, color: sideColor } = getPositionSideLabel(
                position.positionAmt,
                position.positionSide
              );

              // Use real-time calculated PnL from WebSocket mark prices (like Binance app)
              const liveData = calculateLivePnL(position);
              const liveMarkPrice = liveData.markPrice;
              const livePnL = liveData.pnl;
              const roe = liveData.roe;

              const isNearLiquidation = position.liquidationPrice > 0 && (
                (position.positionAmt > 0 && liveMarkPrice < position.liquidationPrice * 1.1) ||
                (position.positionAmt < 0 && liveMarkPrice > position.liquidationPrice * 0.9)
              );

              const orders = positionOrders[position.symbol];
              const tpOrder = orders?.take_profit_orders?.[0];
              const slOrder = orders?.stop_loss_orders?.[0];
              const trailingOrder = orders?.trailing_stop_orders?.[0];

              const isEditing = editingTPSL === position.symbol;

              // Calculate TP/SL distance from current price (using live mark price)
              const tpPrice = tpOrder?.stopPrice;
              const slPrice = slOrder?.stopPrice;
              const tpDistance = tpPrice ? ((tpPrice - liveMarkPrice) / liveMarkPrice * 100) : null;
              const slDistance = slPrice ? ((slPrice - liveMarkPrice) / liveMarkPrice * 100) : null;

              return (
                <tr key={`${position.symbol}-${position.positionSide}`} className="border-b border-gray-800 hover:bg-gray-800/50">
                  {/* Symbol */}
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-2">
                      {position.positionAmt > 0 ? (
                        <TrendingUp className="w-4 h-4 text-green-500" />
                      ) : (
                        <TrendingDown className="w-4 h-4 text-red-500" />
                      )}
                      <div>
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => onSymbolClick?.(position.symbol)}
                            className="font-semibold hover:text-yellow-400 transition-colors cursor-pointer text-left"
                            title="Click to view chart"
                          >
                            {position.symbol}
                          </button>
                          {/* Trade Source Badge */}
                          {tradeSources[position.symbol] === 'ai' && (
                            <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-xs font-medium bg-purple-500/20 text-purple-400 border border-purple-500/50" title="AI Autopilot">
                              <Brain className="w-3 h-3" />
                              AI
                            </span>
                          )}
                          {tradeSources[position.symbol] === 'strategy' && (
                            <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-xs font-medium bg-yellow-500/20 text-yellow-400 border border-yellow-500/50" title="Strategy">
                              <Zap className="w-3 h-3" />
                              STR
                            </span>
                          )}
                          {tradeSources[position.symbol] === 'manual' && (
                            <span className="inline-flex items-center gap-0.5 px-1.5 py-0.5 rounded text-xs font-medium bg-blue-500/20 text-blue-400 border border-blue-500/50" title="Manual">
                              <User className="w-3 h-3" />
                              MAN
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-2 text-xs">
                          <span className={sideColor}>{sideLabel}</span>
                          <span className="text-yellow-500">{position.leverage}x</span>
                          <span className="text-gray-500">{position.marginType.toLowerCase()}</span>
                        </div>
                      </div>
                    </div>
                  </td>

                  {/* Mode */}
                  <td className="py-3 px-2 text-center">
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
                  </td>

                  {/* Size */}
                  <td className="py-3 px-4">
                    <div className={sideColor}>
                      {formatQuantity(Math.abs(position.positionAmt))}
                    </div>
                    <div className="text-xs text-gray-500">
                      {formatUSD(Math.abs(position.notional))}
                    </div>
                  </td>

                  {/* Entry Price */}
                  <td className="py-3 px-4 text-right font-mono text-xs">
                    {formatPrice(position.entryPrice)}
                  </td>

                  {/* Mark Price (real-time from WebSocket) */}
                  <td className="py-3 px-4 text-right font-mono text-xs">
                    {formatPrice(liveMarkPrice)}
                  </td>

                  {/* Liquidation Price */}
                  <td className="py-3 px-4 text-right">
                    <div className={`font-mono text-xs ${isNearLiquidation ? 'text-red-500' : 'text-gray-400'}`}>
                      {position.liquidationPrice > 0 ? formatPrice(position.liquidationPrice) : '-'}
                    </div>
                    {isNearLiquidation && (
                      <div className="flex items-center justify-end gap-1 text-xs text-red-500">
                        <AlertTriangle className="w-3 h-3" />
                      </div>
                    )}
                  </td>

                  {/* Take Profit */}
                  <td className="py-3 px-4 text-center">
                    {isEditing ? (
                      <input
                        type="number"
                        value={tpValue}
                        onChange={(e) => setTpValue(e.target.value)}
                        onFocus={handleInputFocus}
                        placeholder="TP price"
                        className="w-20 px-2 py-1 bg-gray-800 border border-gray-600 rounded text-xs text-center"
                        step="0.01"
                      />
                    ) : tpOrder ? (
                      <div>
                        <div className="font-mono text-xs text-green-400">
                          {formatPrice(tpOrder.stopPrice)}
                        </div>
                        <div className="text-xs text-gray-500">
                          {tpDistance !== null && !isNaN(Number(tpDistance)) && (
                            <span className={tpDistance >= 0 ? 'text-green-500' : 'text-red-500'}>
                              {tpDistance >= 0 ? '+' : ''}{Number(tpDistance).toFixed(2)}%
                            </span>
                          )}
                        </div>
                      </div>
                    ) : (
                      <span className="text-gray-500 text-xs">-</span>
                    )}
                  </td>

                  {/* Stop Loss */}
                  <td className="py-3 px-4 text-center">
                    {isEditing ? (
                      <input
                        type="number"
                        value={slValue}
                        onChange={(e) => setSlValue(e.target.value)}
                        onFocus={handleInputFocus}
                        placeholder="SL price"
                        className="w-20 px-2 py-1 bg-gray-800 border border-gray-600 rounded text-xs text-center"
                        step="0.01"
                      />
                    ) : slOrder ? (
                      <div>
                        <div className="font-mono text-xs text-red-400">
                          {formatPrice(slOrder.stopPrice)}
                        </div>
                        <div className="text-xs text-gray-500">
                          {slDistance !== null && !isNaN(Number(slDistance)) && (
                            <span className={slDistance >= 0 ? 'text-green-500' : 'text-red-500'}>
                              {slDistance >= 0 ? '+' : ''}{Number(slDistance).toFixed(2)}%
                            </span>
                          )}
                        </div>
                      </div>
                    ) : (
                      <div className="flex items-center justify-center gap-1">
                        <AlertTriangle className="w-3 h-3 text-yellow-500" />
                        <span className="text-yellow-500 text-xs">No SL!</span>
                      </div>
                    )}
                  </td>

                  {/* PnL (real-time calculated from WebSocket mark price) */}
                  <td className="py-3 px-4 text-right">
                    <div className={`font-semibold ${getPositionColor(livePnL)}`}>
                      {formatUSD(livePnL)}
                    </div>
                    <div className={`text-xs ${getPositionColor(roe)}`}>
                      ({formatPercent(roe)})
                    </div>
                    {trailingOrder && (
                      <div className="text-xs text-purple-400">
                        TS: {formatPrice(trailingOrder.stopPrice)}
                      </div>
                    )}
                  </td>

                  {/* ROI Target */}
                  <td className="py-3 px-4 text-center">
                    {editingROI === position.symbol ? (
                      <div className="flex flex-col gap-1">
                        <input
                          type="number"
                          value={roiValue}
                          onChange={(e) => setRoiValue(e.target.value)}
                          onFocus={handleInputFocus}
                          placeholder="ROI %"
                          className="w-20 px-2 py-1 bg-gray-800 border border-gray-600 rounded text-xs text-center"
                          step="0.1"
                          min="0"
                          max="1000"
                        />
                        <label className="flex items-center gap-1 text-xs text-gray-400">
                          <input
                            type="checkbox"
                            checked={saveForFuture}
                            onChange={(e) => setSaveForFuture(e.target.checked)}
                            className="w-3 h-3"
                          />
                          Save
                        </label>
                      </div>
                    ) : roiFromGinie[position.symbol] !== null && roiFromGinie[position.symbol] !== undefined ? (
                      <div>
                        <div className="font-mono text-xs text-yellow-400">
                          {(roiFromGinie[position.symbol]!).toFixed(2)}%
                        </div>
                        <div className="text-xs text-gray-500">
                          (custom)
                        </div>
                      </div>
                    ) : (
                      <div className="text-xs text-gray-500">
                        <div>-</div>
                        <div className="text-xs text-gray-600">(auto)</div>
                      </div>
                    )}
                  </td>

                  {/* Actions */}
                  <td className="py-3 px-4">
                    <div className="flex items-center justify-center gap-1">
                      {editingROI === position.symbol ? (
                        <>
                          <button
                            onClick={() => saveROI(position.symbol)}
                            disabled={savingROI}
                            className="p-1.5 bg-green-500/20 hover:bg-green-500/30 text-green-500 rounded"
                            title="Save ROI Target"
                          >
                            <Check className="w-3 h-3" />
                          </button>
                          <button
                            onClick={cancelEditROI}
                            className="p-1.5 bg-gray-500/20 hover:bg-gray-500/30 text-gray-400 rounded"
                            title="Cancel"
                          >
                            <XCircle className="w-3 h-3" />
                          </button>
                        </>
                      ) : isEditing ? (
                        <>
                          <button
                            onClick={() => saveTPSL(position.symbol, position.positionSide)}
                            disabled={savingTPSL}
                            className="p-1.5 bg-green-500/20 hover:bg-green-500/30 text-green-500 rounded"
                            title="Save TP/SL"
                          >
                            <Check className="w-3 h-3" />
                          </button>
                          <button
                            onClick={cancelEditTPSL}
                            className="p-1.5 bg-gray-500/20 hover:bg-gray-500/30 text-gray-400 rounded"
                            title="Cancel"
                          >
                            <XCircle className="w-3 h-3" />
                          </button>
                        </>
                      ) : (
                        <>
                          <button
                            onClick={() => startEditTPSL(
                              position.symbol,
                              tpOrder?.stopPrice,
                              slOrder?.stopPrice
                            )}
                            className="p-1.5 bg-blue-500/20 hover:bg-blue-500/30 text-blue-400 rounded"
                            title="Edit TP/SL/ROI"
                          >
                            <Edit2 className="w-3 h-3" />
                          </button>
                          <button
                            onClick={() => startEditROI(position.symbol, roiFromGinie[position.symbol] || undefined)}
                            className="p-1.5 bg-yellow-500/20 hover:bg-yellow-500/30 text-yellow-400 rounded"
                            title="Set ROI Target"
                          >
                            <TrendingUp className="w-3 h-3" />
                          </button>
                          <button
                            onClick={() => handleClosePosition(position.symbol)}
                            disabled={isLoading}
                            className="p-1.5 bg-red-500/20 hover:bg-red-500/30 text-red-500 rounded"
                            title="Close Position"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
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
          <span className="text-purple-400">TS</span>
          <span>Trailing Stop</span>
        </div>
        <div className="flex items-center gap-1">
          <AlertTriangle className="w-3 h-3 text-yellow-500" />
          <span>No protection</span>
        </div>
        <span className="text-gray-600">|</span>
        <div className="flex items-center gap-1">
          <Brain className="w-3 h-3 text-purple-400" />
          <span>AI Autopilot</span>
        </div>
        <div className="flex items-center gap-1">
          <Zap className="w-3 h-3 text-yellow-400" />
          <span>Strategy</span>
        </div>
        <div className="flex items-center gap-1">
          <User className="w-3 h-3 text-blue-400" />
          <span>Manual</span>
        </div>
      </div>
    </div>
  );
}
