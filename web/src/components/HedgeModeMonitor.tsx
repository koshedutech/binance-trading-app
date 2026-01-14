import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, TrendingUp, TrendingDown, ChevronDown, ChevronUp, Shield, Layers, ArrowLeftRight, DollarSign, Activity, Target } from 'lucide-react';
import { futuresApi, HedgeModePositionData } from '../services/futuresApi';

interface HedgeModeMonitorProps {
  autoRefresh?: boolean;
  refreshInterval?: number;
}

const HedgeModeMonitor = ({ autoRefresh = true, refreshInterval = 5000 }: HedgeModeMonitorProps) => {
  const [positions, setPositions] = useState<HedgeModePositionData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedPosition, setExpandedPosition] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [count, setCount] = useState(0);

  const fetchData = useCallback(async () => {
    try {
      const posData = await futuresApi.getHedgeModePositions();
      setPositions(posData.positions || []);
      setCount(posData.count);
      setLastUpdate(new Date());
      setError(null);
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to fetch hedge mode data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    if (autoRefresh) {
      const interval = setInterval(fetchData, refreshInterval);
      return () => clearInterval(interval);
    }
  }, [fetchData, autoRefresh, refreshInterval]);

  const formatPrice = (price: number) => {
    if (price === 0) return '-';
    if (price < 0.01) return price.toFixed(8);
    if (price < 1) return price.toFixed(6);
    if (price < 100) return price.toFixed(4);
    return price.toFixed(2);
  };

  const formatPnL = (pnl: number) => {
    const sign = pnl >= 0 ? '+' : '';
    return `${sign}$${pnl.toFixed(2)}`;
  };

  const formatPercent = (pct: number) => {
    const sign = pct >= 0 ? '+' : '';
    return `${sign}${pct.toFixed(2)}%`;
  };

  const getTriggerTypeColor = (type: string) => {
    switch (type) {
      case 'profit':
        return 'text-green-400 bg-green-900/30';
      case 'loss':
        return 'text-red-400 bg-red-900/30';
      default:
        return 'text-gray-400 bg-gray-700/30';
    }
  };

  const getModeColor = (mode: string) => {
    switch (mode?.toLowerCase()) {
      case 'scalp':
        return 'bg-yellow-900/50 text-yellow-400';
      case 'swing':
        return 'bg-blue-900/50 text-blue-400';
      case 'position':
        return 'bg-green-900/50 text-green-400';
      case 'ultra_fast':
        return 'bg-red-900/50 text-red-400';
      default:
        return 'bg-gray-700/50 text-gray-400';
    }
  };

  const renderProgressBar = (current: number, target: number, color: string) => {
    const percentage = Math.min((current / target) * 100, 100);
    return (
      <div className="w-full bg-gray-700 rounded-full h-1.5">
        <div
          className={`h-1.5 rounded-full ${color}`}
          style={{ width: `${percentage}%` }}
        />
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-4">
        <RefreshCw className="w-4 h-4 text-gray-400 animate-spin" />
        <span className="ml-2 text-xs text-gray-400">Loading hedge mode positions...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-2 bg-red-900/20 border border-red-700 rounded">
        <span className="text-xs text-red-400">{error}</span>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {/* Header - Monitor Only (No Configuration) */}
      <div className="flex items-center justify-between p-2 bg-gray-800/50 rounded border border-gray-700">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-1.5">
            <Layers className="w-4 h-4 text-purple-400" />
            <span className="text-sm font-medium text-gray-200">Hedge Monitor</span>
          </div>
          <div className="text-xs text-gray-400">
            <span className="font-medium text-gray-300">{count}</span> active hedges
          </div>
        </div>
        <div className="flex items-center gap-2">
          {lastUpdate && (
            <span className="text-[9px] text-gray-500">
              Updated {lastUpdate.toLocaleTimeString()}
            </span>
          )}
          <button
            onClick={fetchData}
            className="p-1 hover:bg-gray-700 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw className="w-3 h-3 text-gray-400" />
          </button>
        </div>
      </div>

      {/* Info Banner */}
      <div className="px-2 py-1.5 bg-purple-900/20 border border-purple-700/30 rounded text-[10px] text-purple-400">
        Hedge settings are configured in each mode's Position Optimization section. This view shows active hedge positions.
      </div>

      {/* Positions List */}
      {positions.length === 0 ? (
        <div className="p-4 text-center text-xs text-gray-500">
          No positions with active hedge mode
        </div>
      ) : (
        <div className="space-y-2">
          {positions.map((pos) => (
            <div
              key={pos.symbol}
              className="bg-gray-800/50 rounded border border-gray-700 overflow-hidden"
            >
              {/* Position Header */}
              <div
                className="p-2 cursor-pointer hover:bg-gray-700/30 transition-colors"
                onClick={() => setExpandedPosition(expandedPosition === pos.symbol ? null : pos.symbol)}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {/* Mode tag first - shows which mode owns this position */}
                    <span className={`text-[9px] px-1.5 py-0.5 rounded uppercase font-medium ${getModeColor(pos.mode)}`}>
                      {pos.mode || 'unknown'}
                    </span>
                    {/* Symbol */}
                    <span className="font-medium text-sm text-gray-200">{pos.symbol}</span>
                    {/* Direction */}
                    {pos.original_side === 'LONG' ? (
                      <TrendingUp className="w-4 h-4 text-green-400" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-400" />
                    )}
                    {/* Hedge status */}
                    {pos.hedge.active && (
                      <span className="text-[9px] px-1.5 py-0.5 rounded bg-purple-900/50 text-purple-400 border border-purple-700 flex items-center gap-1">
                        <ArrowLeftRight className="w-3 h-3" /> HEDGED
                      </span>
                    )}
                    {pos.hedge.trigger_type && (
                      <span className={`text-[9px] px-1.5 py-0.5 rounded ${getTriggerTypeColor(pos.hedge.trigger_type)}`}>
                        {pos.hedge.trigger_type.toUpperCase()} trigger
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="text-right">
                      <div className={`text-sm font-bold ${pos.combined.roi_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {formatPercent(pos.combined.roi_percent)} ROI
                      </div>
                      <div className="text-[9px] text-gray-400">
                        Combined: <span className={pos.combined.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>{formatPnL(pos.combined.total_pnl)}</span>
                      </div>
                    </div>
                    {expandedPosition === pos.symbol ? (
                      <ChevronUp className="w-4 h-4 text-gray-400" />
                    ) : (
                      <ChevronDown className="w-4 h-4 text-gray-400" />
                    )}
                  </div>
                </div>

                {/* ROI Progress Bar */}
                {pos.hedge.active && (
                  <div className="mt-2">
                    <div className="flex items-center justify-between text-[9px] text-gray-500 mb-1">
                      <span>Combined ROI Progress</span>
                      <span>{formatPercent(pos.combined.roi_percent)}</span>
                    </div>
                    {renderProgressBar(
                      Math.max(0, pos.combined.roi_percent),
                      5, // Default target - backend determines actual exit
                      pos.combined.roi_percent >= 0 ? 'bg-green-500' : 'bg-red-500'
                    )}
                  </div>
                )}
              </div>

              {/* Expanded Details */}
              {expandedPosition === pos.symbol && (
                <div className="px-2 pb-2 border-t border-gray-700 space-y-3">
                  {/* Mode Info Banner */}
                  <div className="mt-2 px-2 py-1.5 bg-gray-700/30 rounded border border-gray-600 text-[10px] text-gray-400">
                    <span className="text-gray-500">Using hedge settings from:</span>{' '}
                    <span className={`font-medium uppercase ${getModeColor(pos.mode).replace('bg-', 'text-').split(' ')[1]}`}>
                      {pos.mode || 'unknown'}
                    </span>{' '}
                    <span className="text-gray-500">mode</span>
                  </div>

                  {/* Two-Column Layout: Original vs Hedge */}
                  <div className="grid grid-cols-2 gap-3 mt-2">
                    {/* Original Position */}
                    <div className="p-2 bg-gray-700/30 rounded border border-gray-600">
                      <div className="flex items-center gap-1.5 mb-2">
                        <Target className="w-3.5 h-3.5 text-blue-400" />
                        <span className="text-[10px] font-medium text-blue-400">ORIGINAL ({pos.original_side})</span>
                      </div>
                      <div className="space-y-1 text-[10px]">
                        <div className="flex justify-between">
                          <span className="text-gray-500">Entry</span>
                          <span className="text-gray-300">{formatPrice(pos.entry_price)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Avg (BE)</span>
                          <span className="text-gray-300">{formatPrice(pos.original.current_be)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Qty</span>
                          <span className="text-gray-300">{pos.original.remaining_qty.toFixed(4)}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">TP Level</span>
                          <span className="text-gray-300">TP{pos.original.tp_level}</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Realized</span>
                          <span className={pos.original.accum_profit >= 0 ? 'text-green-400' : 'text-red-400'}>
                            {formatPnL(pos.original.accum_profit)}
                          </span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-gray-500">Unrealized</span>
                          <span className={pos.original.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                            {formatPnL(pos.original.unrealized_pnl)}
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Hedge Position */}
                    <div className={`p-2 rounded border ${pos.hedge.active ? 'bg-purple-900/20 border-purple-700' : 'bg-gray-700/30 border-gray-600'}`}>
                      <div className="flex items-center gap-1.5 mb-2">
                        <ArrowLeftRight className="w-3.5 h-3.5 text-purple-400" />
                        <span className="text-[10px] font-medium text-purple-400">
                          HEDGE ({pos.hedge.side || 'N/A'})
                        </span>
                        {!pos.hedge.active && (
                          <span className="text-[8px] px-1 py-0.5 bg-gray-700 rounded text-gray-500">INACTIVE</span>
                        )}
                      </div>
                      {pos.hedge.active ? (
                        <div className="space-y-1 text-[10px]">
                          <div className="flex justify-between">
                            <span className="text-gray-500">Entry</span>
                            <span className="text-gray-300">{formatPrice(pos.hedge.entry_price)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Avg (BE)</span>
                            <span className="text-gray-300">{formatPrice(pos.hedge.current_be)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Qty</span>
                            <span className="text-gray-300">{pos.hedge.remaining_qty.toFixed(4)}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">TP Level</span>
                            <span className="text-gray-300">TP{pos.hedge.tp_level}</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Realized</span>
                            <span className={pos.hedge.accum_profit >= 0 ? 'text-green-400' : 'text-red-400'}>
                              {formatPnL(pos.hedge.accum_profit)}
                            </span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-gray-500">Unrealized</span>
                            <span className={pos.hedge.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                              {formatPnL(pos.hedge.unrealized_pnl)}
                            </span>
                          </div>
                        </div>
                      ) : (
                        <div className="text-[10px] text-gray-500 italic">
                          Hedge not yet triggered
                        </div>
                      )}
                    </div>
                  </div>

                  {/* DCA Info */}
                  {pos.dca.enabled && (
                    <div className="p-2 bg-yellow-900/20 border border-yellow-700/50 rounded">
                      <div className="flex items-center gap-1.5 mb-1">
                        <Layers className="w-3.5 h-3.5 text-yellow-400" />
                        <span className="text-[10px] font-medium text-yellow-400">DCA (Dollar Cost Averaging)</span>
                      </div>
                      <div className="grid grid-cols-3 gap-2 text-[10px]">
                        <div>
                          <span className="text-gray-500">Additions: </span>
                          <span className="text-gray-300">{pos.dca.additions_count}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">Total Qty: </span>
                          <span className="text-gray-300">{pos.dca.total_qty.toFixed(4)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">Neg TP Triggered: </span>
                          <span className="text-red-400">{pos.dca.neg_tp_triggered}</span>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Wide SL Info */}
                  {pos.wide_sl.price > 0 && (
                    <div className="p-2 bg-orange-900/20 border border-orange-700/50 rounded">
                      <div className="flex items-center gap-1.5 mb-1">
                        <Shield className="w-3.5 h-3.5 text-orange-400" />
                        <span className="text-[10px] font-medium text-orange-400">Wide Stop Loss (ATR-based)</span>
                      </div>
                      <div className="grid grid-cols-3 gap-2 text-[10px]">
                        <div>
                          <span className="text-gray-500">SL Price: </span>
                          <span className="text-gray-300">{formatPrice(pos.wide_sl.price)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">ATR Mult: </span>
                          <span className="text-gray-300">{pos.wide_sl.atr_multiplier}x</span>
                        </div>
                        <div>
                          <span className="text-gray-500">AI Blocked: </span>
                          <span className={pos.wide_sl.ai_blocked ? 'text-green-400' : 'text-red-400'}>
                            {pos.wide_sl.ai_blocked ? 'YES' : 'NO'}
                          </span>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Combined PnL Summary */}
                  <div className="p-2 bg-gray-700/30 rounded border border-gray-600">
                    <div className="flex items-center gap-1.5 mb-1">
                      <DollarSign className="w-3.5 h-3.5 text-gray-400" />
                      <span className="text-[10px] font-medium text-gray-400">Combined P&L</span>
                    </div>
                    <div className="grid grid-cols-4 gap-2 text-[10px]">
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">ROI</div>
                        <div className={`font-medium ${pos.combined.roi_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPercent(pos.combined.roi_percent)}
                        </div>
                      </div>
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">Realized</div>
                        <div className={`font-medium ${pos.combined.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPnL(pos.combined.realized_pnl)}
                        </div>
                      </div>
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">Unrealized</div>
                        <div className={`font-medium ${pos.combined.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPnL(pos.combined.unrealized_pnl)}
                        </div>
                      </div>
                      <div className="text-center p-1.5 bg-gray-800 rounded">
                        <div className="text-gray-500">Total</div>
                        <div className={`font-medium ${pos.combined.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {formatPnL(pos.combined.total_pnl)}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Debug Log */}
                  {pos.debug_log && pos.debug_log.length > 0 && (
                    <div className="p-2 bg-gray-900/50 rounded border border-gray-600">
                      <div className="flex items-center gap-1.5 mb-1">
                        <Activity className="w-3.5 h-3.5 text-gray-400" />
                        <span className="text-[10px] font-medium text-gray-400">Debug Log (Last 5)</span>
                      </div>
                      <div className="space-y-0.5 max-h-24 overflow-y-auto">
                        {pos.debug_log.slice(-5).map((log, idx) => (
                          <div key={idx} className="text-[9px] text-gray-500 font-mono truncate" title={log}>
                            {log}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {/* Current Price */}
                  <div className="text-[9px] text-gray-500 text-right">
                    Current Price: <span className="text-gray-400">{formatPrice(pos.current_price)}</span>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default HedgeModeMonitor;
