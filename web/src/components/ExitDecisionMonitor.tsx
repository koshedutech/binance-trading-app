/**
 * Exit Decision Monitor Component
 * Story 10.3: Exit Decision Monitoring UI
 *
 * Displays real-time exit decision state for a position including:
 * - Hold safeguards (min hold time, breakeven, consecutive signals)
 * - Exit checks (trend reversal, efficiency exit, trailing SL, dynamic TP)
 * - Classic mode indicators (ADX, RSI, EMA, reversal signals)
 * - New Engine mode details (strategy, regime, exit signal)
 */

import React, { useState, useEffect, useCallback } from 'react';
import { apiService } from '../services/api';
import { wsService } from '../services/websocket';
import {
  ExitDecisionState,
  ExitCheck,
  HoldSafeguards,
  ExitCheckStatus,
  formatDuration,
  getStatusColor,
  getStatusBgColor,
  getExitCheckDisplayName,
  getPriorityBadge,
} from '../types/exitDecision';

interface ExitDecisionMonitorProps {
  symbol: string;
  isExpanded: boolean;
  onRefresh?: () => void;
}

// Status badge component
const StatusBadge: React.FC<{ status: ExitCheckStatus }> = ({ status }) => {
  return (
    <span
      className={`px-2 py-0.5 text-xs font-medium rounded ${getStatusColor(status)} ${getStatusBgColor(status)}`}
    >
      {status}
    </span>
  );
};

// Priority badge component
const PriorityBadge: React.FC<{ priority: number }> = ({ priority }) => {
  const colors = {
    1: 'bg-red-500/20 text-red-400 border-red-500/30',
    2: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
    3: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
    4: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
  };
  const colorClass = colors[priority as keyof typeof colors] || colors[4];

  return (
    <span className={`px-1.5 py-0.5 text-xs font-bold rounded border ${colorClass}`}>
      {getPriorityBadge(priority)}
    </span>
  );
};

// Progress bar component
const ProgressBar: React.FC<{ percent: number; label: string; color?: string }> = ({
  percent,
  label,
  color = 'blue',
}) => {
  const clampedPercent = Math.min(100, Math.max(0, percent));
  const colorClasses = {
    blue: 'bg-blue-500',
    green: 'bg-green-500',
    yellow: 'bg-yellow-500',
    red: 'bg-red-500',
  };
  const bgClass = colorClasses[color as keyof typeof colorClasses] || colorClasses.blue;

  return (
    <div className="w-full">
      <div className="flex justify-between text-xs mb-1">
        <span className="text-gray-400">{label}</span>
        <span className="text-gray-300">{clampedPercent.toFixed(1)}%</span>
      </div>
      <div className="w-full bg-gray-700 rounded-full h-1.5">
        <div
          className={`h-1.5 rounded-full transition-all duration-300 ${bgClass}`}
          style={{ width: `${clampedPercent}%` }}
        />
      </div>
    </div>
  );
};

// Hold Safeguards Section
const HoldSafeguardsSection: React.FC<{ safeguards: HoldSafeguards }> = ({ safeguards }) => {
  const { min_hold_time, breakeven, consecutive_signals } = safeguards;

  return (
    <div className="bg-gray-800/50 rounded-lg p-3 space-y-3">
      <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
        <svg className="w-4 h-4 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
        Hold Safeguards
      </h4>

      {/* Min Hold Time */}
      <div className="flex items-center justify-between text-sm">
        <span className="text-gray-400">Min Hold Time</span>
        <div className="flex items-center gap-2">
          <span className="text-gray-300">
            {formatDuration(min_hold_time.elapsed_seconds)} / {formatDuration(min_hold_time.required_seconds)}
          </span>
          {min_hold_time.met ? (
            <svg className="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
            </svg>
          ) : (
            <svg className="w-4 h-4 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          )}
        </div>
      </div>

      {/* Breakeven */}
      <div className="flex items-center justify-between text-sm">
        <span className="text-gray-400">Breakeven</span>
        <div className="flex items-center gap-2">
          {breakeven.achieved ? (
            <>
              <span className="text-green-400">Achieved @ ${breakeven.be_price?.toFixed(4)}</span>
              <svg className="w-4 h-4 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            </>
          ) : (
            <>
              <span className="text-yellow-400">Pending</span>
              <svg className="w-4 h-4 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </>
          )}
        </div>
      </div>

      {/* Consecutive Signals */}
      <div className="flex items-center justify-between text-sm">
        <span className="text-gray-400">Consecutive Signals</span>
        <span className="text-gray-300">
          {consecutive_signals.current} / {consecutive_signals.required} required
        </span>
      </div>
    </div>
  );
};

// Classic Mode Indicators (Task 7)
const ClassicModeIndicators: React.FC<{ details: ExitCheck['trend_reversal_details'] }> = ({ details }) => {
  if (!details) return null;

  const { adx, rsi, ema_cross, reversal_signals, macd } = details;

  return (
    <div className="grid grid-cols-2 gap-3 mt-2">
      {/* ADX */}
      <div className="bg-gray-700/50 rounded p-2">
        <div className="text-xs text-gray-400 mb-1">ADX</div>
        <div className="flex items-baseline gap-1">
          <span className="text-lg font-semibold text-white">{adx.value.toFixed(1)}</span>
          <span className="text-xs text-gray-500">/ {adx.threshold}</span>
        </div>
        <div className={`text-xs mt-1 ${
          adx.status === 'strong_trend' ? 'text-green-400' :
          adx.status === 'weak_trend' ? 'text-yellow-400' : 'text-gray-400'
        }`}>
          {adx.status.replace('_', ' ')}
        </div>
      </div>

      {/* RSI */}
      <div className="bg-gray-700/50 rounded p-2">
        <div className="text-xs text-gray-400 mb-1">RSI</div>
        <div className="flex items-baseline gap-1">
          <span className="text-lg font-semibold text-white">{rsi.value.toFixed(1)}</span>
        </div>
        <div className={`text-xs mt-1 ${
          rsi.state === 'overbought' ? 'text-red-400' :
          rsi.state === 'oversold' ? 'text-green-400' : 'text-gray-400'
        }`}>
          {rsi.state} ({rsi.oversold}-{rsi.overbought})
        </div>
      </div>

      {/* EMA Cross */}
      <div className="bg-gray-700/50 rounded p-2">
        <div className="text-xs text-gray-400 mb-1">EMA Cross</div>
        <div className="flex items-center gap-2">
          {ema_cross.detected ? (
            <>
              <span className={`text-sm font-medium ${
                ema_cross.type === 'bullish' ? 'text-green-400' : 'text-red-400'
              }`}>
                {ema_cross.type?.toUpperCase()}
              </span>
              <span className="text-xs text-yellow-400">DETECTED</span>
            </>
          ) : (
            <span className="text-sm text-gray-500">No cross</span>
          )}
        </div>
      </div>

      {/* Reversal Signals */}
      <div className="bg-gray-700/50 rounded p-2">
        <div className="text-xs text-gray-400 mb-1">Reversal Signals</div>
        <div className="flex items-baseline gap-1">
          <span className={`text-lg font-semibold ${
            reversal_signals.count >= reversal_signals.required ? 'text-red-400' : 'text-white'
          }`}>
            {reversal_signals.count}
          </span>
          <span className="text-xs text-gray-500">/ {reversal_signals.required}</span>
        </div>
      </div>

      {/* MACD (if available) */}
      {macd && (
        <div className="bg-gray-700/50 rounded p-2 col-span-2">
          <div className="text-xs text-gray-400 mb-1">MACD Histogram</div>
          <div className="flex items-center gap-2">
            <span className={`text-sm font-medium ${
              macd.histogram > 0 ? 'text-green-400' : 'text-red-400'
            }`}>
              {macd.histogram.toFixed(4)}
            </span>
            <span className={`text-xs ${
              macd.direction === 'bullish' ? 'text-green-400' :
              macd.direction === 'bearish' ? 'text-red-400' : 'text-gray-400'
            }`}>
              {macd.direction}
            </span>
          </div>
        </div>
      )}
    </div>
  );
};

// New Engine Mode Display (Task 8)
const NewEngineModeDisplay: React.FC<{ details: ExitCheck['new_engine_details'] }> = ({ details }) => {
  if (!details) return null;

  const { strategy, entry_regime, current_regime, regime_changed, technical_score, exit_signal, strategy_rules } = details;

  return (
    <div className="mt-2 space-y-3">
      <div className="grid grid-cols-2 gap-3">
        {/* Strategy */}
        <div className="bg-gray-700/50 rounded p-2">
          <div className="text-xs text-gray-400 mb-1">Strategy</div>
          <div className="text-sm font-medium text-white capitalize">{strategy}</div>
        </div>

        {/* Regime */}
        <div className="bg-gray-700/50 rounded p-2">
          <div className="text-xs text-gray-400 mb-1">Regime</div>
          <div className="flex items-center gap-1">
            <span className="text-sm font-medium text-white capitalize">{current_regime}</span>
            {regime_changed && (
              <span className="text-xs text-yellow-400">(changed from {entry_regime})</span>
            )}
          </div>
        </div>
      </div>

      {/* Technical Score */}
      <ProgressBar
        percent={technical_score}
        label="Technical Score"
        color={technical_score >= 70 ? 'green' : technical_score >= 40 ? 'yellow' : 'red'}
      />

      {/* Exit Signal Strength */}
      <ProgressBar
        percent={exit_signal}
        label="Exit Signal Strength"
        color={exit_signal >= 70 ? 'red' : exit_signal >= 40 ? 'yellow' : 'green'}
      />

      {/* Strategy Rules */}
      {strategy_rules && strategy_rules.length > 0 && (
        <div className="bg-gray-700/50 rounded p-2">
          <div className="text-xs text-gray-400 mb-2">Strategy Rules</div>
          <ul className="space-y-1">
            {strategy_rules.map((rule, idx) => (
              <li key={idx} className="text-xs text-gray-300 flex items-start gap-1">
                <span className="text-blue-400 mt-0.5">-</span>
                {rule}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};

// Single Exit Check Card
const ExitCheckCard: React.FC<{ check: ExitCheck; decisionMode: string }> = ({ check, decisionMode }) => {
  const [isExpanded, setIsExpanded] = useState(check.would_exit);

  return (
    <div className={`border rounded-lg overflow-hidden ${
      check.would_exit ? 'border-red-500/50 bg-red-500/5' : 'border-gray-700 bg-gray-800/30'
    }`}>
      {/* Header */}
      <button
        className="w-full px-3 py-2 flex items-center justify-between hover:bg-gray-700/30 transition-colors"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-2">
          <PriorityBadge priority={check.priority} />
          <span className="text-sm font-medium text-white">{getExitCheckDisplayName(check.name)}</span>
        </div>
        <div className="flex items-center gap-2">
          <StatusBadge status={check.status} />
          <svg
            className={`w-4 h-4 text-gray-400 transition-transform ${isExpanded ? 'rotate-180' : ''}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </button>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="px-3 pb-3 border-t border-gray-700/50">
          {/* Trend Reversal Details */}
          {check.name === 'TREND_REVERSAL' && check.trend_reversal_details && (
            <ClassicModeIndicators details={check.trend_reversal_details} />
          )}

          {/* New Engine Details */}
          {check.name === 'TREND_REVERSAL' && decisionMode === 'new_engine' && check.new_engine_details && (
            <NewEngineModeDisplay details={check.new_engine_details} />
          )}

          {/* Efficiency Exit Details */}
          {check.name === 'EFFICIENCY_EXIT' && check.efficiency_details && (
            <div className="mt-2 space-y-2">
              <div className="grid grid-cols-2 gap-2 text-sm">
                <div>
                  <span className="text-gray-400">Peak Profit:</span>
                  <span className={`ml-1 ${check.efficiency_details.peak_profit_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {check.efficiency_details.peak_profit_percent.toFixed(2)}%
                  </span>
                </div>
                <div>
                  <span className="text-gray-400">Current:</span>
                  <span className={`ml-1 ${check.efficiency_details.current_profit_percent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {check.efficiency_details.current_profit_percent.toFixed(2)}%
                  </span>
                </div>
              </div>
              <ProgressBar
                percent={check.efficiency_details.efficiency_percent}
                label={`Efficiency (threshold: ${check.efficiency_details.threshold_percent}%)`}
                color={check.efficiency_details.efficiency_percent >= check.efficiency_details.threshold_percent ? 'green' : 'red'}
              />
            </div>
          )}

          {/* Trailing SL Details */}
          {check.name === 'TRAILING_SL' && check.trailing_sl_details && (
            <div className="mt-2 space-y-2">
              <div className="grid grid-cols-2 gap-2 text-sm">
                <div>
                  <span className="text-gray-400">Stop Loss:</span>
                  <span className="ml-1 text-white">${check.trailing_sl_details.sl_price.toFixed(4)}</span>
                </div>
                <div>
                  <span className="text-gray-400">Current:</span>
                  <span className="ml-1 text-white">${check.trailing_sl_details.current_price.toFixed(4)}</span>
                </div>
              </div>
              {check.trailing_sl_details.active && check.trailing_sl_details.distance_percent !== undefined && (
                <div className="text-sm">
                  <span className="text-gray-400">Distance:</span>
                  <span className={`ml-1 ${check.trailing_sl_details.distance_percent > 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {check.trailing_sl_details.distance_percent.toFixed(2)}%
                  </span>
                </div>
              )}
            </div>
          )}

          {/* Dynamic TP Details */}
          {check.name === 'DYNAMIC_TP' && check.dynamic_tp_details && (
            <div className="mt-2 space-y-2">
              <div className="grid grid-cols-2 gap-2 text-sm">
                {check.dynamic_tp_details.tp_price && (
                  <div>
                    <span className="text-gray-400">Take Profit:</span>
                    <span className="ml-1 text-white">${check.dynamic_tp_details.tp_price.toFixed(4)}</span>
                  </div>
                )}
                <div>
                  <span className="text-gray-400">Current:</span>
                  <span className="ml-1 text-white">${check.dynamic_tp_details.current_price.toFixed(4)}</span>
                </div>
              </div>
              {check.dynamic_tp_details.active && check.dynamic_tp_details.progress_percent !== undefined && (
                <ProgressBar
                  percent={check.dynamic_tp_details.progress_percent}
                  label="Progress to TP"
                  color="green"
                />
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

// Main Component
export const ExitDecisionMonitor: React.FC<ExitDecisionMonitorProps> = ({
  symbol,
  isExpanded,
  onRefresh,
}) => {
  const [state, setState] = useState<ExitDecisionState | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [wsConnected, setWsConnected] = useState(true); // AC8: WebSocket connection status

  const fetchData = useCallback(async () => {
    if (!isExpanded) return;

    setLoading(true);
    setError(null);

    try {
      const response = await apiService.getExitDecisionState(symbol);
      if (response.success && response.exit_decision) {
        setState(response.exit_decision as ExitDecisionState);
      } else {
        setError(response.error || 'Failed to fetch exit decision state');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }, [symbol, isExpanded]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Auto-refresh every 5 seconds when expanded
  useEffect(() => {
    if (!isExpanded) return;

    const interval = setInterval(fetchData, 5000);
    return () => clearInterval(interval);
  }, [isExpanded, fetchData]);

  // WebSocket subscription for real-time updates (Story 10.3: AC8)
  useEffect(() => {
    if (!isExpanded) return;

    const handleExitDecisionUpdate = (event: { type: string; data?: { exit_decision?: ExitDecisionState } }) => {
      // Check if update is for our symbol
      if (event.data?.exit_decision?.symbol === symbol) {
        setState(event.data.exit_decision);
        setWsConnected(true); // Got update, connection is good
      }
    };

    // Handle WebSocket connection/disconnection (AC8)
    const handleWsConnect = () => setWsConnected(true);
    const handleWsDisconnect = () => setWsConnected(false);

    // Subscribe to exit decision updates and connection events
    wsService.subscribe('EXIT_DECISION_UPDATE', handleExitDecisionUpdate);
    wsService.subscribe('CONNECTED', handleWsConnect);
    wsService.subscribe('DISCONNECTED', handleWsDisconnect);

    return () => {
      wsService.unsubscribe('EXIT_DECISION_UPDATE', handleExitDecisionUpdate);
      wsService.unsubscribe('CONNECTED', handleWsConnect);
      wsService.unsubscribe('DISCONNECTED', handleWsDisconnect);
    };
  }, [isExpanded, symbol]);

  const handleRefresh = () => {
    fetchData();
    onRefresh?.();
  };

  if (!isExpanded) return null;

  if (loading && !state) {
    return (
      <div className="p-4 text-center">
        <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full mx-auto"></div>
        <p className="text-sm text-gray-400 mt-2">Loading exit decision state...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-4 text-center">
        <p className="text-sm text-red-400">{error}</p>
        <button
          onClick={handleRefresh}
          className="mt-2 px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 rounded transition-colors"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!state) return null;

  return (
    <div className="space-y-4">
      {/* Header with Overall Decision */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h3 className="text-sm font-medium text-gray-300">Exit Decision Monitor</h3>
          <span className={`px-2 py-0.5 text-xs font-medium rounded ${
            state.overall_decision === 'EXIT'
              ? 'bg-red-500/20 text-red-400'
              : 'bg-green-500/20 text-green-400'
          }`}>
            {state.overall_decision}
          </span>
          <span className="text-xs text-gray-500 capitalize">({state.decision_mode} mode)</span>
        </div>
        <button
          onClick={handleRefresh}
          disabled={loading}
          className="p-1.5 hover:bg-gray-700 rounded transition-colors disabled:opacity-50"
          title="Refresh"
        >
          <svg
            className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
        </button>
      </div>

      {/* Decision Reason */}
      <div className="text-xs text-gray-400 bg-gray-800/50 rounded px-3 py-2">
        {state.decision_reason}
      </div>

      {/* Hold Safeguards */}
      <HoldSafeguardsSection safeguards={state.hold_safeguards} />

      {/* Exit Checks */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
          <svg className="w-4 h-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Exit Checks
        </h4>
        <div className="space-y-2">
          {state.exit_checks.map((check, idx) => (
            <ExitCheckCard key={idx} check={check} decisionMode={state.decision_mode} />
          ))}
        </div>
      </div>

      {/* Last Updated with relative time (AC6) and connection status (AC8) */}
      <div className="text-xs text-gray-500 text-right flex items-center justify-end gap-2">
        {wsConnected ? (
          <>
            <span className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse" title="Connected - Auto-refreshing"></span>
            <span>
              Last checked: {Math.round((Date.now() - state.last_check_time) / 1000)}s ago
            </span>
          </>
        ) : (
          <>
            <span className="w-1.5 h-1.5 bg-yellow-500 rounded-full animate-pulse" title="Reconnecting..."></span>
            <span className="text-yellow-500">Reconnecting...</span>
          </>
        )}
      </div>
    </div>
  );
};

export default ExitDecisionMonitor;
