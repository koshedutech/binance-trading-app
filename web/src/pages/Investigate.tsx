import { useState, useEffect } from 'react';
import {
  Search,
  Activity,
  AlertTriangle,
  CheckCircle,
  XCircle,
  TrendingUp,
  TrendingDown,
  Shield,
  Zap,
  Brain,
  RefreshCw,
  ChevronDown,
  ChevronRight,
  RotateCcw,
  Crosshair,
  Calculator,
  Timer
} from 'lucide-react';
import {
  futuresApi,
  InvestigateStatus,
} from '../services/futuresApi';

export default function Investigate() {
  const [status, setStatus] = useState<InvestigateStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({
    rejections: true,
    modes: true,
    constraints: true,
    alerts: true,
    apiHealth: true,
  });
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const fetchStatus = async () => {
    try {
      const data = await futuresApi.getInvestigateStatus();
      setStatus(data);
      setError(null);
    } catch (err) {
      setError('Failed to fetch investigate status');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, []);

  const toggleSection = (section: string) => {
    setExpandedSections(prev => ({ ...prev, [section]: !prev[section] }));
  };

  const handleAction = async (action: string) => {
    setActionLoading(action);
    try {
      switch (action) {
        case 'resetCircuitBreaker':
          await futuresApi.resetCircuitBreaker();
          break;
        case 'recalculateAllocation':
          await futuresApi.recalculateAllocation();
          break;
        case 'forceSyncPositions':
          await futuresApi.forceSyncPositions();
          break;
        case 'clearCooldown':
          await futuresApi.clearFlipFlopCooldown();
          break;
      }
      await fetchStatus();
    } catch (err) {
      console.error(`Action ${action} failed:`, err);
    } finally {
      setActionLoading(null);
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active': return 'text-green-400';
      case 'blocked': return 'text-yellow-400';
      case 'stopped': return 'text-red-400';
      default: return 'text-gray-400';
    }
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active': return <CheckCircle className="w-6 h-6 text-green-400" />;
      case 'blocked': return <AlertTriangle className="w-6 h-6 text-yellow-400" />;
      case 'stopped': return <XCircle className="w-6 h-6 text-red-400" />;
      default: return <Activity className="w-6 h-6 text-gray-400" />;
    }
  };

  const getStatusBg = (status: string) => {
    switch (status) {
      case 'active': return 'bg-green-500/10 border-green-500/30';
      case 'blocked': return 'bg-yellow-500/10 border-yellow-500/30';
      case 'stopped': return 'bg-red-500/10 border-red-500/30';
      default: return 'bg-gray-500/10 border-gray-500/30';
    }
  };

  const getModeIcon = (modeName: string) => {
    switch (modeName.toLowerCase()) {
      case 'autopilot': return <Zap className="w-4 h-4" />;
      case 'circuit_breaker': return <Shield className="w-4 h-4" />;
      case 'dynamic_sltp': return <Crosshair className="w-4 h-4" />;
      case 'scalping': return <TrendingUp className="w-4 h-4" />;
      case 'averaging': return <TrendingDown className="w-4 h-4" />;
      default: return <Activity className="w-4 h-4" />;
    }
  };

  const formatModeName = (name: string) => {
    return name.split('_').map(word =>
      word.charAt(0).toUpperCase() + word.slice(1)
    ).join(' ');
  };

  const getConstraintColor = (status: string) => {
    switch (status) {
      case 'ok': return 'bg-green-500';
      case 'warning': return 'bg-yellow-500';
      case 'critical': return 'bg-red-500';
      default: return 'bg-gray-500';
    }
  };

  const formatTimeAgo = (timestamp: string) => {
    if (!timestamp || timestamp === 'N/A' || timestamp === '') return 'N/A';
    const date = new Date(timestamp);
    if (isNaN(date.getTime())) return 'N/A';
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return date.toLocaleDateString();
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-950 text-white p-6 flex items-center justify-center">
        <RefreshCw className="w-8 h-8 animate-spin text-purple-400" />
      </div>
    );
  }

  if (error || !status) {
    return (
      <div className="min-h-screen bg-gray-950 text-white p-6">
        <div className="max-w-6xl mx-auto">
          <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 text-red-400">
            {error || 'Failed to load status'}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-950 text-white p-6">
      <div className="max-w-6xl mx-auto space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Search className="w-8 h-8 text-purple-400" />
            <div>
              <h1 className="text-2xl font-bold">Investigate</h1>
              <p className="text-gray-400 text-sm">AI Autopilot Diagnostic Dashboard</p>
            </div>
          </div>
          <button
            onClick={fetchStatus}
            className="flex items-center gap-2 px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>

        {/* Trading Status - Hero Card */}
        <div className={`rounded-xl border p-6 ${getStatusBg(status.trading_status)}`}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              {getStatusIcon(status.trading_status)}
              <div>
                <h2 className={`text-2xl font-bold ${getStatusColor(status.trading_status)}`}>
                  {status.trading_status === 'active' && 'AI is Actively Trading'}
                  {status.trading_status === 'blocked' && 'AI is Blocked'}
                  {status.trading_status === 'stopped' && 'AI is Stopped'}
                </h2>
                <p className="text-gray-400">
                  Last decision: {formatTimeAgo(status.last_decision_time)} | {status.active_positions} active positions
                </p>
              </div>
            </div>
            {status.block_reasons.length > 0 && (
              <div className="text-right">
                <p className="text-yellow-400 text-sm font-medium">Block Reasons:</p>
                {status.block_reasons.map((reason, i) => (
                  <p key={i} className="text-yellow-300 text-sm">{reason}</p>
                ))}
              </div>
            )}
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* Left Column */}
          <div className="space-y-6">
            {/* Why Not Trading Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800">
              <button
                onClick={() => toggleSection('rejections')}
                className="w-full flex items-center justify-between p-4 hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <AlertTriangle className="w-5 h-5 text-yellow-400" />
                  <h3 className="font-semibold">Why Not Trading?</h3>
                  <span className="text-xs bg-gray-700 px-2 py-0.5 rounded-full text-gray-300">
                    {status.rejection_stats.rejection_rate.toFixed(0)}% rejection rate
                  </span>
                </div>
                {expandedSections.rejections ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>

              {expandedSections.rejections && (
                <div className="px-4 pb-4 space-y-3">
                  {/* Rejection Stats */}
                  <div className="grid grid-cols-3 gap-3 text-center">
                    <div className="bg-gray-800 rounded-lg p-3">
                      <p className="text-2xl font-bold text-white">{status.rejection_stats.total_decisions}</p>
                      <p className="text-xs text-gray-400">Total Decisions</p>
                    </div>
                    <div className="bg-gray-800 rounded-lg p-3">
                      <p className="text-2xl font-bold text-red-400">{status.rejection_stats.total_rejections}</p>
                      <p className="text-xs text-gray-400">Rejections</p>
                    </div>
                    <div className="bg-gray-800 rounded-lg p-3">
                      <p className="text-2xl font-bold text-blue-400">{(status.rejection_stats.avg_confidence * 100).toFixed(0)}%</p>
                      <p className="text-xs text-gray-400">Avg Confidence</p>
                    </div>
                  </div>

                  {/* Common Rejection Reasons */}
                  {Object.keys(status.rejection_stats.common_reasons).length > 0 && (
                    <div className="space-y-2">
                      <p className="text-sm text-gray-400 font-medium">Common Rejection Reasons:</p>
                      {Object.entries(status.rejection_stats.common_reasons)
                        .sort(([,a], [,b]) => b - a)
                        .slice(0, 5)
                        .map(([reason, count]) => (
                          <div key={reason} className="flex items-center justify-between bg-gray-800 rounded px-3 py-2">
                            <span className="text-sm text-gray-300">{reason}</span>
                            <span className="text-xs bg-yellow-500/20 text-yellow-400 px-2 py-0.5 rounded">{count}x</span>
                          </div>
                        ))}
                    </div>
                  )}

                  {/* Recent Rejections */}
                  {status.recent_rejections.length > 0 && (
                    <div className="space-y-2">
                      <p className="text-sm text-gray-400 font-medium">Recent Rejections:</p>
                      {status.recent_rejections.slice(0, 5).map((rejection, i) => (
                        <div key={i} className="bg-gray-800 rounded px-3 py-2 text-sm">
                          <div className="flex items-center justify-between">
                            <span className="text-white font-medium">{rejection.symbol}</span>
                            <span className="text-gray-500 text-xs">{formatTimeAgo(rejection.timestamp)}</span>
                          </div>
                          <p className="text-gray-400 text-xs mt-1">{rejection.reason}</p>
                        </div>
                      ))}
                    </div>
                  )}

                  {status.recent_rejections.length === 0 && Object.keys(status.rejection_stats.common_reasons).length === 0 && (
                    <div className="text-center py-4 text-gray-500">
                      <CheckCircle className="w-8 h-8 mx-auto mb-2 text-green-500" />
                      <p>No recent rejections</p>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Mode Status Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800">
              <button
                onClick={() => toggleSection('modes')}
                className="w-full flex items-center justify-between p-4 hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <Activity className="w-5 h-5 text-blue-400" />
                  <h3 className="font-semibold">Mode Status</h3>
                </div>
                {expandedSections.modes ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>

              {expandedSections.modes && (
                <div className="px-4 pb-4">
                  <div className="space-y-2">
                    {Object.entries(status.modes).map(([name, mode]) => (
                      <div key={name} className="flex items-center justify-between bg-gray-800 rounded-lg px-4 py-3">
                        <div className="flex items-center gap-3">
                          {getModeIcon(name)}
                          <span className="font-medium">{formatModeName(name)}</span>
                        </div>
                        <div className="flex items-center gap-3">
                          <span className="text-sm text-gray-400">{mode.details}</span>
                          <span className={`px-2 py-1 rounded text-xs font-medium ${
                            mode.enabled
                              ? 'bg-green-500/20 text-green-400'
                              : 'bg-red-500/20 text-red-400'
                          }`}>
                            {mode.status}
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Alerts Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800">
              <button
                onClick={() => toggleSection('alerts')}
                className="w-full flex items-center justify-between p-4 hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <AlertTriangle className="w-5 h-5 text-orange-400" />
                  <h3 className="font-semibold">Alerts</h3>
                  {status.alerts.length > 0 && (
                    <span className="bg-orange-500 text-white text-xs px-2 py-0.5 rounded-full">
                      {status.alerts.length}
                    </span>
                  )}
                </div>
                {expandedSections.alerts ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>

              {expandedSections.alerts && (
                <div className="px-4 pb-4">
                  {status.alerts.length > 0 ? (
                    <div className="space-y-2">
                      {status.alerts.map((alert, i) => (
                        <div key={i} className={`rounded-lg px-4 py-3 ${
                          alert.level === 'critical' ? 'bg-red-500/10 border border-red-500/30' :
                          alert.level === 'warning' ? 'bg-yellow-500/10 border border-yellow-500/30' :
                          'bg-blue-500/10 border border-blue-500/30'
                        }`}>
                          <div className="flex items-center justify-between">
                            <span className={`font-medium ${
                              alert.level === 'critical' ? 'text-red-400' :
                              alert.level === 'warning' ? 'text-yellow-400' :
                              'text-blue-400'
                            }`}>{alert.type}</span>
                          </div>
                          <p className="text-sm text-gray-300 mt-1">{alert.message}</p>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-center py-4 text-gray-500">
                      <CheckCircle className="w-8 h-8 mx-auto mb-2 text-green-500" />
                      <p>No active alerts</p>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* Right Column */}
          <div className="space-y-6">
            {/* Resource Constraints Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800">
              <button
                onClick={() => toggleSection('constraints')}
                className="w-full flex items-center justify-between p-4 hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <Calculator className="w-5 h-5 text-cyan-400" />
                  <h3 className="font-semibold">Resource Constraints</h3>
                </div>
                {expandedSections.constraints ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>

              {expandedSections.constraints && (
                <div className="px-4 pb-4 space-y-4">
                  {/* USD Allocation */}
                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-400">USD Allocation</span>
                      <span className="text-white">
                        ${status.constraints.usd_allocation.current.toFixed(2)} / ${status.constraints.usd_allocation.max.toFixed(2)}
                      </span>
                    </div>
                    <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${getConstraintColor(status.constraints.usd_allocation.status)} transition-all`}
                        style={{ width: `${Math.min(status.constraints.usd_allocation.percent, 100)}%` }}
                      />
                    </div>
                  </div>

                  {/* Daily Trades */}
                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-400">Daily Trades</span>
                      <span className="text-white">
                        {status.constraints.daily_trades.current} / {status.constraints.daily_trades.max}
                      </span>
                    </div>
                    <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${getConstraintColor(status.constraints.daily_trades.status)} transition-all`}
                        style={{ width: `${Math.min(status.constraints.daily_trades.percent, 100)}%` }}
                      />
                    </div>
                  </div>

                  {/* Daily PnL */}
                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-400">Daily PnL Limit</span>
                      <span className={status.constraints.daily_pnl.current < 0 ? 'text-red-400' : 'text-green-400'}>
                        ${status.constraints.daily_pnl.current.toFixed(2)} / -${Math.abs(status.constraints.daily_pnl.max).toFixed(2)}
                      </span>
                    </div>
                    <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${getConstraintColor(status.constraints.daily_pnl.status)} transition-all`}
                        style={{ width: `${Math.min(status.constraints.daily_pnl.percent, 100)}%` }}
                      />
                    </div>
                  </div>

                  {/* Hourly Loss */}
                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-400">Hourly Loss Limit</span>
                      <span className={status.constraints.hourly_loss.current < 0 ? 'text-red-400' : 'text-green-400'}>
                        ${status.constraints.hourly_loss.current.toFixed(2)} / -${Math.abs(status.constraints.hourly_loss.max).toFixed(2)}
                      </span>
                    </div>
                    <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${getConstraintColor(status.constraints.hourly_loss.status)} transition-all`}
                        style={{ width: `${Math.min(status.constraints.hourly_loss.percent, 100)}%` }}
                      />
                    </div>
                  </div>

                  {/* Consecutive Loss */}
                  <div>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="text-gray-400">Consecutive Losses</span>
                      <span className="text-white">
                        {status.constraints.consecutive_loss.current} / {status.constraints.consecutive_loss.max}
                      </span>
                    </div>
                    <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
                      <div
                        className={`h-full ${getConstraintColor(status.constraints.consecutive_loss.status)} transition-all`}
                        style={{ width: `${Math.min(status.constraints.consecutive_loss.percent, 100)}%` }}
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Signal Health Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800 p-4">
              <div className="flex items-center gap-2 mb-4">
                <Brain className="w-5 h-5 text-purple-400" />
                <h3 className="font-semibold">Signal Health</h3>
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div className={`rounded-lg p-3 text-center ${
                  status.signal_health.ml_predictor.available
                    ? 'bg-green-500/10 border border-green-500/30'
                    : 'bg-red-500/10 border border-red-500/30'
                }`}>
                  <p className={`text-sm font-medium ${status.signal_health.ml_predictor.available ? 'text-green-400' : 'text-red-400'}`}>
                    ML Predictor
                  </p>
                  <p className="text-xs text-gray-400 mt-1">
                    {status.signal_health.ml_predictor.last_used ? formatTimeAgo(status.signal_health.ml_predictor.last_used) : 'N/A'}
                  </p>
                </div>
                <div className={`rounded-lg p-3 text-center ${
                  status.signal_health.llm_analyzer.available
                    ? 'bg-green-500/10 border border-green-500/30'
                    : 'bg-red-500/10 border border-red-500/30'
                }`}>
                  <p className={`text-sm font-medium ${status.signal_health.llm_analyzer.available ? 'text-green-400' : 'text-red-400'}`}>
                    LLM Analyzer
                  </p>
                  <p className="text-xs text-gray-400 mt-1">
                    {status.signal_health.llm_analyzer.last_used ? formatTimeAgo(status.signal_health.llm_analyzer.last_used) : 'N/A'}
                  </p>
                </div>
                <div className={`rounded-lg p-3 text-center ${
                  status.signal_health.sentiment_analyzer.available
                    ? 'bg-green-500/10 border border-green-500/30'
                    : 'bg-red-500/10 border border-red-500/30'
                }`}>
                  <p className={`text-sm font-medium ${status.signal_health.sentiment_analyzer.available ? 'text-green-400' : 'text-red-400'}`}>
                    Sentiment
                  </p>
                  <p className="text-xs text-gray-400 mt-1">
                    {status.signal_health.sentiment_analyzer.last_used ? formatTimeAgo(status.signal_health.sentiment_analyzer.last_used) : 'N/A'}
                  </p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3 mt-3">
                <div className="bg-gray-800 rounded-lg p-3 text-center">
                  <p className="text-xl font-bold text-blue-400">{(status.signal_health.avg_confidence * 100).toFixed(0)}%</p>
                  <p className="text-xs text-gray-400">Avg Confidence</p>
                </div>
                <div className="bg-gray-800 rounded-lg p-3 text-center">
                  <p className="text-xl font-bold text-purple-400">{(status.signal_health.confluence_rate * 100).toFixed(0)}%</p>
                  <p className="text-xs text-gray-400">Confluence Rate</p>
                </div>
              </div>
            </div>

            {/* API Health Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800">
              <button
                onClick={() => toggleSection('apiHealth')}
                className="w-full flex items-center justify-between p-4 hover:bg-gray-800/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <Activity className="w-5 h-5 text-green-400" />
                  <h3 className="font-semibold">API Health</h3>
                </div>
                {expandedSections.apiHealth ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
              </button>

              {expandedSections.apiHealth && (
                <div className="px-4 pb-4">
                  <div className="grid grid-cols-2 gap-2">
                    {Object.entries(status.api_health).map(([name, health]) => (
                      <div key={name} className="flex items-center justify-between bg-gray-800 rounded px-3 py-2">
                        <span className="text-sm text-gray-300">{name}</span>
                        <span className={`text-xs px-2 py-0.5 rounded ${
                          health === 'ok' ? 'bg-green-500/20 text-green-400' :
                          health === 'slow' ? 'bg-yellow-500/20 text-yellow-400' :
                          'bg-red-500/20 text-red-400'
                        }`}>
                          {health.toUpperCase()}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Quick Actions Section */}
            <div className="bg-gray-900 rounded-xl border border-gray-800 p-4">
              <div className="flex items-center gap-2 mb-4">
                <Zap className="w-5 h-5 text-yellow-400" />
                <h3 className="font-semibold">Quick Actions</h3>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <button
                  onClick={() => handleAction('resetCircuitBreaker')}
                  disabled={actionLoading !== null}
                  className="flex items-center justify-center gap-2 px-4 py-3 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-50"
                >
                  <RotateCcw className={`w-4 h-4 ${actionLoading === 'resetCircuitBreaker' ? 'animate-spin' : ''}`} />
                  <span className="text-sm">Reset Circuit Breaker</span>
                </button>
                <button
                  onClick={() => handleAction('recalculateAllocation')}
                  disabled={actionLoading !== null}
                  className="flex items-center justify-center gap-2 px-4 py-3 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-50"
                >
                  <Calculator className={`w-4 h-4 ${actionLoading === 'recalculateAllocation' ? 'animate-spin' : ''}`} />
                  <span className="text-sm">Recalculate Allocation</span>
                </button>
                <button
                  onClick={() => handleAction('forceSyncPositions')}
                  disabled={actionLoading !== null}
                  className="flex items-center justify-center gap-2 px-4 py-3 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-50"
                >
                  <RefreshCw className={`w-4 h-4 ${actionLoading === 'forceSyncPositions' ? 'animate-spin' : ''}`} />
                  <span className="text-sm">Force Sync Positions</span>
                </button>
                <button
                  onClick={() => handleAction('clearCooldown')}
                  disabled={actionLoading !== null}
                  className="flex items-center justify-center gap-2 px-4 py-3 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-50"
                >
                  <Timer className={`w-4 h-4 ${actionLoading === 'clearCooldown' ? 'animate-spin' : ''}`} />
                  <span className="text-sm">Clear Flip-Flop Cooldown</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
