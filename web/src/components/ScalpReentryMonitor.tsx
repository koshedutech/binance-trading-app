import { useState, useEffect, useCallback } from 'react';
import { RefreshCw, TrendingUp, TrendingDown, Clock, CheckCircle, XCircle, AlertCircle, ChevronDown, ChevronUp, Target, ArrowRight } from 'lucide-react';
import { futuresApi, ScalpReentryPositionStatus, ScalpReentryCycleInfo, ScalpReentryPositionsSummary } from '../services/futuresApi';

interface ScalpReentryMonitorProps {
  autoRefresh?: boolean;
  refreshInterval?: number; // ms
}

const ScalpReentryMonitor = ({ autoRefresh = true, refreshInterval = 5000 }: ScalpReentryMonitorProps) => {
  const [positions, setPositions] = useState<ScalpReentryPositionStatus[]>([]);
  const [summary, setSummary] = useState<ScalpReentryPositionsSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedPosition, setExpandedPosition] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  const fetchData = useCallback(async () => {
    try {
      const data = await futuresApi.getScalpReentryPositions();
      setPositions(data.positions || []);
      setSummary(data.summary);
      setLastUpdate(new Date());
      setError(null);
    } catch (err: any) {
      setError(err?.response?.data?.error || 'Failed to fetch scalp reentry positions');
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

  const getStateIcon = (state: string) => {
    switch (state) {
      case 'WAITING':
        return <Clock className="w-3.5 h-3.5 text-yellow-400" />;
      case 'EXECUTING':
        return <RefreshCw className="w-3.5 h-3.5 text-blue-400 animate-spin" />;
      case 'COMPLETED':
        return <CheckCircle className="w-3.5 h-3.5 text-green-400" />;
      case 'FAILED':
        return <XCircle className="w-3.5 h-3.5 text-red-400" />;
      case 'SKIPPED':
        return <AlertCircle className="w-3.5 h-3.5 text-orange-400" />;
      default:
        return <Target className="w-3.5 h-3.5 text-gray-400" />;
    }
  };

  const getStateColor = (state: string) => {
    switch (state) {
      case 'WAITING':
        return 'text-yellow-400 bg-yellow-900/30';
      case 'EXECUTING':
        return 'text-blue-400 bg-blue-900/30';
      case 'COMPLETED':
        return 'text-green-400 bg-green-900/30';
      case 'FAILED':
        return 'text-red-400 bg-red-900/30';
      case 'SKIPPED':
        return 'text-orange-400 bg-orange-900/30';
      default:
        return 'text-gray-400 bg-gray-700/30';
    }
  };

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

  const renderTPProgress = (pos: ScalpReentryPositionStatus) => {
    const tpLevels = [
      { level: 1, label: 'TP1', percent: pos.next_tp_level === 1 ? pos.next_tp_percent : 0.3 },
      { level: 2, label: 'TP2', percent: pos.next_tp_level === 2 ? pos.next_tp_percent : 0.6 },
      { level: 3, label: 'TP3', percent: pos.next_tp_level === 3 ? pos.next_tp_percent : 1.0 },
    ];

    return (
      <div className="flex items-center gap-1 mt-1">
        {tpLevels.map((tp, idx) => {
          const isHit = pos.tp_level_unlocked >= tp.level;
          const isNext = pos.tp_level_unlocked + 1 === tp.level;
          const isBlocked = isNext && pos.next_tp_blocked;

          return (
            <div key={tp.level} className="flex items-center">
              <div
                className={`px-1.5 py-0.5 rounded text-[9px] font-medium ${
                  isHit
                    ? 'bg-green-900/50 text-green-400 border border-green-700'
                    : isBlocked
                    ? 'bg-yellow-900/50 text-yellow-400 border border-yellow-700 animate-pulse'
                    : isNext
                    ? 'bg-blue-900/50 text-blue-400 border border-blue-700'
                    : 'bg-gray-700/50 text-gray-500 border border-gray-600'
                }`}
                title={isHit ? `${tp.label} hit at ${tp.percent}%` : isBlocked ? `Waiting for rebuy before ${tp.label}` : `Target: ${tp.percent}%`}
              >
                {isHit ? '✓' : isBlocked ? '⏳' : ''} {tp.label}
              </div>
              {idx < 2 && <ArrowRight className="w-2.5 h-2.5 text-gray-600 mx-0.5" />}
            </div>
          );
        })}
      </div>
    );
  };

  const renderCycleHistory = (cycles: ScalpReentryCycleInfo[]) => {
    if (!cycles || cycles.length === 0) {
      return <div className="text-[10px] text-gray-500 italic">No cycles yet</div>;
    }

    return (
      <div className="space-y-1.5 mt-2">
        <div className="text-[10px] font-medium text-gray-400">Cycle History</div>
        {cycles.map((cycle) => (
          <div
            key={cycle.cycle_number}
            className="flex items-start gap-2 p-1.5 bg-gray-800/50 rounded border border-gray-700"
          >
            <div className="flex-shrink-0">{getStateIcon(cycle.state)}</div>
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-[10px] font-medium text-gray-300">
                  Cycle {cycle.cycle_number} - TP{cycle.tp_level}
                </span>
                <span className={`text-[9px] px-1 py-0.5 rounded ${getStateColor(cycle.state)}`}>
                  {cycle.state}
                </span>
              </div>
              <div className="flex items-center gap-3 mt-0.5 text-[9px] text-gray-400">
                <span>Sold @ {formatPrice(cycle.sell_price)}</span>
                <span className={cycle.sell_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                  {formatPnL(cycle.sell_pnl)}
                </span>
                {cycle.state === 'WAITING' && (
                  <span className="text-yellow-400">→ Target: {formatPrice(cycle.reentry_target)}</span>
                )}
                {cycle.state === 'COMPLETED' && (
                  <span className="text-green-400">→ Rebought @ {formatPrice(cycle.reentry_price)}</span>
                )}
                {cycle.state === 'SKIPPED' && (
                  <span className="text-orange-400">→ {cycle.outcome_reason}</span>
                )}
              </div>
              {cycle.ai_reasoning && (
                <div className="text-[9px] text-gray-500 mt-0.5 truncate" title={cycle.ai_reasoning}>
                  AI: {cycle.ai_reasoning}
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-4">
        <RefreshCw className="w-4 h-4 text-gray-400 animate-spin" />
        <span className="ml-2 text-xs text-gray-400">Loading scalp reentry positions...</span>
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
      {/* Summary Header */}
      {summary && (
        <div className="flex items-center justify-between p-2 bg-gray-800/50 rounded border border-gray-700">
          <div className="flex items-center gap-4">
            <div className="text-xs text-gray-400">
              <span className="font-medium text-gray-300">{summary.total_positions}</span> positions
            </div>
            <div className="text-xs text-gray-400">
              <span className="font-medium text-gray-300">{summary.total_cycles}</span> cycles
            </div>
            <div className="text-xs text-gray-400">
              <span className="font-medium text-gray-300">{summary.total_reentries}</span> reentries
            </div>
            <div className={`text-xs ${summary.total_accumulated_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {formatPnL(summary.total_accumulated_pnl)} accumulated
            </div>
          </div>
          <div className="flex items-center gap-2">
            <span className={`text-[9px] px-1.5 py-0.5 rounded ${summary.config_enabled ? 'bg-green-900/50 text-green-400' : 'bg-gray-700/50 text-gray-500'}`}>
              {summary.config_enabled ? 'ENABLED' : 'DISABLED'}
            </span>
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
      )}

      {/* Positions List */}
      {positions.length === 0 ? (
        <div className="p-4 text-center text-xs text-gray-500">
          No positions in scalp_reentry mode
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
                    {pos.side === 'LONG' ? (
                      <TrendingUp className="w-4 h-4 text-green-400" />
                    ) : (
                      <TrendingDown className="w-4 h-4 text-red-400" />
                    )}
                    <span className="font-medium text-sm text-gray-200">{pos.symbol}</span>
                    <span className={`text-[9px] px-1.5 py-0.5 rounded ${pos.side === 'LONG' ? 'bg-green-900/50 text-green-400' : 'bg-red-900/50 text-red-400'}`}>
                      {pos.side}
                    </span>
                    {pos.scalp_reentry_active && (
                      <span className="text-[9px] px-1.5 py-0.5 rounded bg-purple-900/50 text-purple-400 border border-purple-700">
                        SCALP-REENTRY
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="text-right">
                      <div className={`text-xs font-medium ${pos.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                        {formatPnL(pos.unrealized_pnl)} ({formatPercent(pos.unrealized_pnl_pct)})
                      </div>
                      {pos.accumulated_profit > 0 && (
                        <div className="text-[9px] text-gray-400">
                          Accumulated: <span className="text-green-400">{formatPnL(pos.accumulated_profit)}</span>
                        </div>
                      )}
                    </div>
                    {expandedPosition === pos.symbol ? (
                      <ChevronUp className="w-4 h-4 text-gray-400" />
                    ) : (
                      <ChevronDown className="w-4 h-4 text-gray-400" />
                    )}
                  </div>
                </div>

                {/* TP Progress Bar */}
                {renderTPProgress(pos)}

                {/* Current State Indicator */}
                {pos.current_cycle_state && pos.current_cycle_state !== 'NONE' && (
                  <div className="flex items-center gap-2 mt-1.5">
                    {getStateIcon(pos.current_cycle_state)}
                    <span className={`text-[10px] ${getStateColor(pos.current_cycle_state)} px-1.5 py-0.5 rounded`}>
                      {pos.current_cycle_state === 'WAITING' && (
                        <>Waiting for rebuy @ {formatPrice(pos.reentry_target_price)} ({formatPercent(pos.distance_to_reentry)} away)</>
                      )}
                      {pos.current_cycle_state === 'EXECUTING' && 'Executing rebuy order...'}
                      {pos.current_cycle_state === 'COMPLETED' && 'Rebuy complete - ready for next TP'}
                      {pos.current_cycle_state !== 'WAITING' && pos.current_cycle_state !== 'EXECUTING' && pos.current_cycle_state !== 'COMPLETED' && pos.current_cycle_state}
                    </span>
                  </div>
                )}
              </div>

              {/* Expanded Details */}
              {expandedPosition === pos.symbol && (
                <div className="px-2 pb-2 border-t border-gray-700">
                  {/* Stats Grid */}
                  <div className="grid grid-cols-4 gap-2 mt-2 text-[10px]">
                    <div className="p-1.5 bg-gray-700/30 rounded">
                      <div className="text-gray-500">Entry</div>
                      <div className="text-gray-300">{formatPrice(pos.entry_price)}</div>
                    </div>
                    <div className="p-1.5 bg-gray-700/30 rounded">
                      <div className="text-gray-500">Current</div>
                      <div className="text-gray-300">{formatPrice(pos.current_price)}</div>
                    </div>
                    <div className="p-1.5 bg-gray-700/30 rounded">
                      <div className="text-gray-500">Reentries</div>
                      <div className="text-gray-300">{pos.successful_reentries}/{pos.total_cycles}</div>
                    </div>
                    <div className="p-1.5 bg-gray-700/30 rounded">
                      <div className="text-gray-500">Skipped</div>
                      <div className="text-orange-400">{pos.skipped_reentries}</div>
                    </div>
                  </div>

                  {/* Final Portion Tracking */}
                  {pos.final_portion_active && (
                    <div className="mt-2 p-1.5 bg-purple-900/20 border border-purple-700/50 rounded">
                      <div className="text-[10px] font-medium text-purple-400">Final Portion Mode</div>
                      <div className="text-[9px] text-gray-400 mt-0.5">
                        Qty: {pos.final_portion_qty.toFixed(4)} | Peak: {formatPrice(pos.final_trailing_peak)}
                        {pos.dynamic_sl_active && <span className="ml-2 text-yellow-400">Dynamic SL: {formatPrice(pos.dynamic_sl_price)}</span>}
                      </div>
                    </div>
                  )}

                  {/* Cycle History */}
                  {renderCycleHistory(pos.cycles)}

                  {/* Last Update */}
                  <div className="text-[9px] text-gray-500 mt-2">
                    Last update: {pos.last_update || 'N/A'}
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

export default ScalpReentryMonitor;
