import React, { useState, useEffect, useCallback } from 'react';
import {
  Search,
  ChevronDown,
  ChevronUp,
  AlertTriangle,
  CheckCircle,
  XCircle,
  RefreshCw,
  Activity,
  TrendingUp,
  TrendingDown,
  Clock,
  Zap,
  Shield,
  BarChart2,
  AlertCircle,
  Info,
} from 'lucide-react';
import { futuresApi, InvestigateStatus, AlertItem } from '../services/futuresApi';

export const InvestigatePanel: React.FC = () => {
  const [isExpanded, setIsExpanded] = useState(true);
  const [status, setStatus] = useState<InvestigateStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    rejections: true,
    modes: true,
    constraints: true,
    alerts: true,
    apiHealth: false,
    actions: false,
  });

  const fetchStatus = useCallback(async () => {
    try {
      setLoading(true);
      const data = await futuresApi.getInvestigateStatus();
      setStatus(data);
      setError(null);
    } catch (err) {
      setError('Failed to fetch investigate status');
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, [fetchStatus]);

  const toggleSection = (section: string) => {
    setExpandedSections(prev => ({ ...prev, [section]: !prev[section] }));
  };

  const handleResetCircuitBreaker = async () => {
    try {
      await futuresApi.resetCircuitBreaker();
      setSuccessMsg('Circuit breaker reset successfully');
      fetchStatus();
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to reset circuit breaker');
    }
  };

  const handleRecalculateAllocation = async () => {
    try {
      await futuresApi.recalculateAllocation();
      setSuccessMsg('Allocation recalculated');
      fetchStatus();
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to recalculate allocation');
    }
  };

  const handleClearCooldown = async () => {
    try {
      await futuresApi.clearFlipFlopCooldown();
      setSuccessMsg('Cooldown cleared');
      fetchStatus();
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to clear cooldown');
    }
  };

  const handleForceSync = async () => {
    try {
      await futuresApi.forceSyncPositions();
      setSuccessMsg('Position sync completed');
      fetchStatus();
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to sync positions');
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'text-green-500';
      case 'blocked':
        return 'text-yellow-500';
      case 'stopped':
        return 'text-red-500';
      default:
        return 'text-gray-400';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active':
        return <CheckCircle className="w-5 h-5 text-green-500" />;
      case 'blocked':
        return <AlertTriangle className="w-5 h-5 text-yellow-500" />;
      case 'stopped':
        return <XCircle className="w-5 h-5 text-red-500" />;
      default:
        return <Activity className="w-5 h-5 text-gray-400" />;
    }
  };

  const getConstraintColor = (status: string) => {
    switch (status) {
      case 'ok':
        return 'bg-green-500';
      case 'warning':
        return 'bg-yellow-500';
      case 'critical':
        return 'bg-red-500';
      default:
        return 'bg-gray-500';
    }
  };

  const getAlertIcon = (level: string) => {
    switch (level) {
      case 'critical':
        return <XCircle className="w-4 h-4 text-red-500" />;
      case 'warning':
        return <AlertTriangle className="w-4 h-4 text-yellow-500" />;
      default:
        return <Info className="w-4 h-4 text-blue-500" />;
    }
  };

  const formatTime = (timeStr: string) => {
    if (!timeStr) return 'Never';
    const date = new Date(timeStr);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    return date.toLocaleDateString();
  };

  const hasAlerts = status && status.alerts && status.alerts.length > 0;

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700">
      {/* Header */}
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-800/50"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-3">
          <Search className="w-5 h-5 text-purple-500" />
          <span className="font-semibold">Investigate</span>
          {hasAlerts && (
            <span className="flex items-center gap-1 px-2 py-0.5 bg-yellow-500/20 text-yellow-500 text-xs rounded-full animate-pulse">
              <AlertCircle className="w-3 h-3" />
              {status?.alerts.length}
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={(e) => {
              e.stopPropagation();
              fetchStatus();
            }}
            className="p-1 hover:bg-gray-700 rounded"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          {isExpanded ? <ChevronUp className="w-5 h-5" /> : <ChevronDown className="w-5 h-5" />}
        </div>
      </div>

      {isExpanded && status && (
        <div className="border-t border-gray-700 p-4 space-y-4">
          {/* Success/Error Messages */}
          {successMsg && (
            <div className="p-3 bg-green-500/10 border border-green-500/30 rounded text-green-500 text-sm">
              {successMsg}
            </div>
          )}
          {error && (
            <div className="p-3 bg-red-500/10 border border-red-500/30 rounded text-red-500 text-sm">
              {error}
            </div>
          )}

          {/* Trading Status */}
          <div className="p-4 bg-gray-800 rounded-lg">
            <div className="flex items-center gap-3 mb-2">
              {getStatusIcon(status.trading_status)}
              <span className={`text-lg font-semibold ${getStatusColor(status.trading_status)}`}>
                {status.trading_status === 'active' && 'AI is actively trading'}
                {status.trading_status === 'blocked' && 'AI is running but blocked'}
                {status.trading_status === 'stopped' && 'AI autopilot is stopped'}
              </span>
            </div>
            <div className="flex items-center gap-4 text-sm text-gray-400">
              <span className="flex items-center gap-1">
                <Clock className="w-4 h-4" />
                Last decision: {formatTime(status.last_decision_time)}
              </span>
              <span className="flex items-center gap-1">
                <TrendingUp className="w-4 h-4" />
                {status.active_positions} active positions
              </span>
            </div>
            {status.block_reasons.length > 0 && (
              <div className="mt-3 space-y-1">
                {status.block_reasons.map((reason, i) => (
                  <div key={i} className="flex items-center gap-2 text-sm text-yellow-500">
                    <AlertTriangle className="w-4 h-4" />
                    {reason}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Why Not Trading / Rejections */}
          <div className="border border-gray-700 rounded-lg">
            <div
              className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-800/50"
              onClick={() => toggleSection('rejections')}
            >
              <span className="font-medium flex items-center gap-2">
                <TrendingDown className="w-4 h-4 text-red-500" />
                Why Not Trading?
                <span className="text-xs text-gray-500">
                  ({status.rejection_stats.rejection_rate.toFixed(0)}% rejection rate)
                </span>
              </span>
              {expandedSections.rejections ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </div>
            {expandedSections.rejections && (
              <div className="border-t border-gray-700 p-3 space-y-3">
                {/* Common Reasons */}
                {Object.keys(status.rejection_stats.common_reasons).length > 0 && (
                  <div className="space-y-2">
                    {Object.entries(status.rejection_stats.common_reasons)
                      .sort(([, a], [, b]) => b - a)
                      .map(([reason, count]) => (
                        <div key={reason} className="flex items-center justify-between text-sm">
                          <span className="text-gray-400">{reason}</span>
                          <span className="px-2 py-0.5 bg-gray-700 rounded text-xs">{count}x</span>
                        </div>
                      ))}
                  </div>
                )}
                {/* Recent Rejections */}
                {status.recent_rejections.length > 0 && (
                  <div className="mt-3 pt-3 border-t border-gray-700">
                    <div className="text-xs text-gray-500 mb-2">Recent Rejections:</div>
                    <div className="space-y-2 max-h-40 overflow-y-auto">
                      {status.recent_rejections.slice(0, 5).map((rej, i) => (
                        <div key={i} className="text-xs p-2 bg-gray-800 rounded">
                          <div className="flex items-center justify-between mb-1">
                            <span className="font-medium">{rej.symbol}</span>
                            <span className="text-gray-500">{formatTime(rej.timestamp)}</span>
                          </div>
                          <div className="text-red-400">{rej.reason}</div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {status.recent_rejections.length === 0 && Object.keys(status.rejection_stats.common_reasons).length === 0 && (
                  <div className="text-sm text-gray-500 flex items-center gap-2">
                    <CheckCircle className="w-4 h-4 text-green-500" />
                    No recent rejections
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Mode Status */}
          <div className="border border-gray-700 rounded-lg">
            <div
              className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-800/50"
              onClick={() => toggleSection('modes')}
            >
              <span className="font-medium flex items-center gap-2">
                <Zap className="w-4 h-4 text-purple-500" />
                Mode Status
              </span>
              {expandedSections.modes ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </div>
            {expandedSections.modes && (
              <div className="border-t border-gray-700 p-3">
                <div className="grid grid-cols-1 gap-2">
                  {Object.entries(status.modes).map(([key, mode]) => (
                    <div key={key} className="flex items-center justify-between p-2 bg-gray-800 rounded text-sm">
                      <span className="capitalize">{key.replace('_', ' ')}</span>
                      <div className="flex items-center gap-2">
                        <span className={mode.enabled ? 'text-green-500' : 'text-gray-500'}>
                          {mode.status}
                        </span>
                        <span className="text-xs text-gray-500">{mode.details}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Resource Constraints */}
          <div className="border border-gray-700 rounded-lg">
            <div
              className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-800/50"
              onClick={() => toggleSection('constraints')}
            >
              <span className="font-medium flex items-center gap-2">
                <BarChart2 className="w-4 h-4 text-blue-500" />
                Resource Constraints
              </span>
              {expandedSections.constraints ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </div>
            {expandedSections.constraints && (
              <div className="border-t border-gray-700 p-3 space-y-3">
                {[
                  { label: 'USD Allocation', data: status.constraints.usd_allocation, prefix: '$' },
                  { label: 'Daily Trades', data: status.constraints.daily_trades },
                  { label: 'Daily PnL', data: status.constraints.daily_pnl, prefix: '$' },
                  { label: 'Hourly Loss', data: status.constraints.hourly_loss, suffix: '%' },
                  { label: 'Consecutive Losses', data: status.constraints.consecutive_loss },
                ].map(({ label, data, prefix = '', suffix = '' }) => (
                  <div key={label} className="space-y-1">
                    <div className="flex items-center justify-between text-sm">
                      <span className="text-gray-400">{label}</span>
                      <span>
                        {prefix}{data.current.toFixed(data.current % 1 === 0 ? 0 : 2)}{suffix} / {prefix}{Math.abs(data.max).toFixed(data.max % 1 === 0 ? 0 : 2)}{suffix}
                      </span>
                    </div>
                    <div className="w-full h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${getConstraintColor(data.status)} transition-all`}
                        style={{ width: `${Math.min(data.percent, 100)}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Alerts */}
          {status.alerts.length > 0 && (
            <div className="border border-gray-700 rounded-lg">
              <div
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-800/50"
                onClick={() => toggleSection('alerts')}
              >
                <span className="font-medium flex items-center gap-2">
                  <AlertCircle className="w-4 h-4 text-yellow-500" />
                  Alerts ({status.alerts.length})
                </span>
                {expandedSections.alerts ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
              </div>
              {expandedSections.alerts && (
                <div className="border-t border-gray-700 p-3 space-y-2">
                  {status.alerts.map((alert: AlertItem, i: number) => (
                    <div
                      key={i}
                      className={`flex items-start gap-2 p-2 rounded text-sm ${
                        alert.level === 'critical'
                          ? 'bg-red-500/10 border border-red-500/30'
                          : alert.level === 'warning'
                          ? 'bg-yellow-500/10 border border-yellow-500/30'
                          : 'bg-blue-500/10 border border-blue-500/30'
                      }`}
                    >
                      {getAlertIcon(alert.level)}
                      <span>{alert.message}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* API Health */}
          <div className="border border-gray-700 rounded-lg">
            <div
              className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-800/50"
              onClick={() => toggleSection('apiHealth')}
            >
              <span className="font-medium flex items-center gap-2">
                <Shield className="w-4 h-4 text-green-500" />
                API Health
              </span>
              {expandedSections.apiHealth ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </div>
            {expandedSections.apiHealth && (
              <div className="border-t border-gray-700 p-3">
                <div className="grid grid-cols-2 gap-2">
                  {Object.entries(status.api_health).map(([key, value]) => (
                    <div key={key} className="flex items-center justify-between p-2 bg-gray-800 rounded text-sm">
                      <span className="capitalize">{key.replace('_', ' ')}</span>
                      <span className={value === 'ok' ? 'text-green-500' : 'text-red-500'}>
                        {value === 'ok' ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Quick Actions */}
          <div className="border border-gray-700 rounded-lg">
            <div
              className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-800/50"
              onClick={() => toggleSection('actions')}
            >
              <span className="font-medium flex items-center gap-2">
                <Zap className="w-4 h-4 text-orange-500" />
                Quick Actions
              </span>
              {expandedSections.actions ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            </div>
            {expandedSections.actions && (
              <div className="border-t border-gray-700 p-3">
                <div className="grid grid-cols-2 gap-2">
                  <button
                    onClick={handleResetCircuitBreaker}
                    className="px-3 py-2 bg-purple-500/20 text-purple-500 hover:bg-purple-500/30 rounded text-sm"
                  >
                    Reset Circuit Breaker
                  </button>
                  <button
                    onClick={handleRecalculateAllocation}
                    className="px-3 py-2 bg-blue-500/20 text-blue-500 hover:bg-blue-500/30 rounded text-sm"
                  >
                    Recalculate Allocation
                  </button>
                  <button
                    onClick={handleForceSync}
                    className="px-3 py-2 bg-green-500/20 text-green-500 hover:bg-green-500/30 rounded text-sm"
                  >
                    Force Sync Positions
                  </button>
                  <button
                    onClick={handleClearCooldown}
                    className="px-3 py-2 bg-yellow-500/20 text-yellow-500 hover:bg-yellow-500/30 rounded text-sm"
                  >
                    Clear Flip-Flop Cooldown
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {isExpanded && !status && loading && (
        <div className="border-t border-gray-700 p-8 text-center">
          <RefreshCw className="w-6 h-6 animate-spin mx-auto text-gray-500" />
          <p className="text-gray-500 mt-2">Loading...</p>
        </div>
      )}
    </div>
  );
};

export default InvestigatePanel;
