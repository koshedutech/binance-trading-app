import { useEffect, useState } from 'react';
import {
  futuresApi,
  GinieDiagnostics,
  GinieSignalLog,
  GinieSignalStats,
  DiagnosticIssue,
} from '../services/futuresApi';
import {
  Activity,
  AlertTriangle,
  CheckCircle,
  XCircle,
  Zap,
  Target,
  TrendingUp,
  TrendingDown,
  Shield,
  Eye,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  Info,
  AlertOctagon,
  Radio,
} from 'lucide-react';

export default function GinieDiagnosticsPanel() {
  const [diagnostics, setDiagnostics] = useState<GinieDiagnostics | null>(null);
  const [signals, setSignals] = useState<GinieSignalLog[]>([]);
  const [signalStats, setSignalStats] = useState<GinieSignalStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<'overview' | 'signals' | 'issues'>('overview');
  const [expandedSignal, setExpandedSignal] = useState<string | null>(null);
  const [signalFilter, setSignalFilter] = useState<'all' | 'executed' | 'rejected'>('all');

  const fetchDiagnostics = async () => {
    try {
      const data = await futuresApi.getGinieDiagnostics();
      setDiagnostics(data);
    } catch (err) {
      console.error('Failed to fetch diagnostics:', err);
    }
  };

  const fetchSignals = async () => {
    try {
      const statusFilter = signalFilter === 'all' ? undefined : signalFilter;
      const { signals: signalData } = await futuresApi.getGinieSignalLogs(100, statusFilter);
      setSignals(signalData || []);
      const stats = await futuresApi.getGinieSignalStats();
      setSignalStats(stats);
    } catch (err) {
      console.error('Failed to fetch signals:', err);
    }
  };

  const refreshAll = async () => {
    setLoading(true);
    await Promise.all([fetchDiagnostics(), fetchSignals()]);
    setLoading(false);
  };

  useEffect(() => {
    refreshAll();
    const interval = setInterval(refreshAll, 10000); // Refresh every 10s
    return () => clearInterval(interval);
  }, [signalFilter]);

  const formatTime = (timestamp: string) => {
    if (!timestamp || timestamp === '0001-01-01T00:00:00Z') return 'Never';
    const date = new Date(timestamp);
    return date.toLocaleTimeString();
  };

  const criticalIssues = diagnostics?.issues?.filter(i => i.severity === 'critical') || [];
  const warningIssues = diagnostics?.issues?.filter(i => i.severity === 'warning') || [];
  const infoIssues = diagnostics?.issues?.filter(i => i.severity === 'info') || [];

  return (
    <div className="bg-gray-800 rounded-lg p-4 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-2">
          <Activity className="w-5 h-5 text-purple-400" />
          <h2 className="text-lg font-semibold text-white">Ginie Diagnostics</h2>
          {diagnostics && (
            <span className={`px-2 py-0.5 rounded text-xs font-medium ${
              diagnostics.autopilot_running
                ? diagnostics.is_live_mode ? 'bg-green-500/20 text-green-400' : 'bg-yellow-500/20 text-yellow-400'
                : 'bg-gray-500/20 text-gray-400'
            }`}>
              {diagnostics.autopilot_running
                ? diagnostics.is_live_mode ? 'LIVE' : 'PAPER'
                : 'STOPPED'}
            </span>
          )}
        </div>
        <button
          onClick={refreshAll}
          disabled={loading}
          className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Quick Status Cards */}
      {diagnostics && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {/* Can Trade */}
          <div className={`p-3 rounded-lg ${diagnostics.can_trade ? 'bg-green-500/10' : 'bg-red-500/10'}`}>
            <div className="flex items-center space-x-2">
              {diagnostics.can_trade ? (
                <CheckCircle className="w-4 h-4 text-green-400" />
              ) : (
                <XCircle className="w-4 h-4 text-red-400" />
              )}
              <span className="text-sm text-gray-300">Can Trade</span>
            </div>
            <p className={`text-xs mt-1 ${diagnostics.can_trade ? 'text-green-400' : 'text-red-400'}`}>
              {diagnostics.can_trade_reason === 'ok' ? 'Ready' : diagnostics.can_trade_reason}
            </p>
          </div>

          {/* Position Slots */}
          <div className="p-3 rounded-lg bg-gray-700/50">
            <div className="flex items-center space-x-2">
              <Target className="w-4 h-4 text-blue-400" />
              <span className="text-sm text-gray-300">Positions</span>
            </div>
            <p className="text-lg font-semibold text-white">
              {diagnostics.positions.open_count}/{diagnostics.positions.max_allowed}
            </p>
            <div className="w-full bg-gray-600 rounded-full h-1.5 mt-1">
              <div
                className="bg-blue-500 h-1.5 rounded-full"
                style={{ width: `${(diagnostics.positions.open_count / diagnostics.positions.max_allowed) * 100}%` }}
              />
            </div>
          </div>

          {/* Circuit Breaker */}
          <div className={`p-3 rounded-lg ${
            diagnostics.circuit_breaker.state === 'open' ? 'bg-red-500/10' : 'bg-gray-700/50'
          }`}>
            <div className="flex items-center space-x-2">
              <Shield className="w-4 h-4 text-orange-400" />
              <span className="text-sm text-gray-300">Circuit Breaker</span>
            </div>
            <p className={`text-sm font-semibold ${
              diagnostics.circuit_breaker.state === 'open' ? 'text-red-400' : 'text-green-400'
            }`}>
              {diagnostics.circuit_breaker.state.toUpperCase()}
            </p>
            {diagnostics.circuit_breaker.cooldown_remaining && (
              <p className="text-xs text-gray-400">{diagnostics.circuit_breaker.cooldown_remaining}</p>
            )}
          </div>

          {/* Signal Execution */}
          <div className="p-3 rounded-lg bg-gray-700/50">
            <div className="flex items-center space-x-2">
              <Zap className="w-4 h-4 text-yellow-400" />
              <span className="text-sm text-gray-300">Signals (1h)</span>
            </div>
            <p className="text-lg font-semibold text-white">
              {diagnostics.signals.executed}/{diagnostics.signals.total_generated}
            </p>
            <p className="text-xs text-gray-400">
              {diagnostics.signals.execution_rate_pct.toFixed(1)}% execution
            </p>
          </div>
        </div>
      )}

      {/* Tabs */}
      <div className="flex space-x-2 border-b border-gray-700">
        {(['overview', 'signals', 'issues'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-sm font-medium transition-colors ${
              activeTab === tab
                ? 'text-purple-400 border-b-2 border-purple-400'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            {tab === 'overview' && 'Overview'}
            {tab === 'signals' && `Signals ${signalStats ? (signalFilter === 'all' ? `(${signalStats.total})` : `(${signals.length}/${signalStats.total})`) : ''}`}
            {tab === 'issues' && `Issues ${diagnostics?.issues?.length ? `(${diagnostics.issues.length})` : ''}`}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="min-h-[300px]">
        {/* Overview Tab */}
        {activeTab === 'overview' && diagnostics && (
          <div className="space-y-4">
            {/* Scanning Status */}
            <div className="bg-gray-700/30 rounded-lg p-3">
              <h3 className="text-sm font-medium text-gray-300 mb-2 flex items-center">
                <Eye className="w-4 h-4 mr-2" />
                Scanning Activity
              </h3>
              <div className="grid grid-cols-3 gap-4 text-sm">
                <div>
                  <p className="text-gray-400">Last Scan</p>
                  <p className="text-white">{formatTime(diagnostics.scanning.last_scan_time)}</p>
                </div>
                <div>
                  <p className="text-gray-400">Symbols</p>
                  <p className="text-white">{diagnostics.scanning.symbols_in_watchlist}</p>
                </div>
                <div>
                  <p className="text-gray-400">Modes</p>
                  <div className="flex space-x-1">
                    {diagnostics.scanning.scalp_enabled && <span className="text-xs bg-blue-500/20 text-blue-400 px-1 rounded">S</span>}
                    {diagnostics.scanning.swing_enabled && <span className="text-xs bg-green-500/20 text-green-400 px-1 rounded">W</span>}
                    {diagnostics.scanning.position_enabled && <span className="text-xs bg-purple-500/20 text-purple-400 px-1 rounded">P</span>}
                  </div>
                </div>
              </div>
            </div>

            {/* LLM Status */}
            <div className="bg-gray-700/30 rounded-lg p-3">
              <h3 className="text-sm font-medium text-gray-300 mb-2 flex items-center">
                <Radio className="w-4 h-4 mr-2" />
                LLM Status
              </h3>
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  {diagnostics.llm_status.connected ? (
                    <CheckCircle className="w-4 h-4 text-green-400" />
                  ) : (
                    <XCircle className="w-4 h-4 text-red-400" />
                  )}
                  <span className="text-sm text-white">
                    {diagnostics.llm_status.connected ? diagnostics.llm_status.provider : 'Not Connected'}
                  </span>
                </div>
                {diagnostics.llm_status.disabled_symbols.length > 0 && (
                  <span className="text-xs text-red-400">
                    {diagnostics.llm_status.disabled_symbols.length} symbols disabled
                  </span>
                )}
              </div>
            </div>

            {/* Profit Booking */}
            <div className="bg-gray-700/30 rounded-lg p-3">
              <h3 className="text-sm font-medium text-gray-300 mb-2 flex items-center">
                <TrendingUp className="w-4 h-4 mr-2" />
                Profit Booking (1h)
              </h3>
              <div className="grid grid-cols-4 gap-4 text-sm">
                <div>
                  <p className="text-gray-400">Pending TPs</p>
                  <p className="text-white">{diagnostics.profit_booking.positions_with_pending_tp}</p>
                </div>
                <div>
                  <p className="text-gray-400">TP Hits</p>
                  <p className="text-green-400">{diagnostics.profit_booking.tp_hits_last_hour}</p>
                </div>
                <div>
                  <p className="text-gray-400">Partials</p>
                  <p className="text-blue-400">{diagnostics.profit_booking.partial_closes_last_hour}</p>
                </div>
                <div>
                  <p className="text-gray-400">Failed</p>
                  <p className={diagnostics.profit_booking.failed_closes_last_hour > 0 ? 'text-red-400' : 'text-gray-400'}>
                    {diagnostics.profit_booking.failed_closes_last_hour}
                  </p>
                </div>
              </div>
            </div>

            {/* Circuit Breaker Details */}
            <div className="bg-gray-700/30 rounded-lg p-3">
              <h3 className="text-sm font-medium text-gray-300 mb-2 flex items-center">
                <Shield className="w-4 h-4 mr-2" />
                Circuit Breaker Details
              </h3>
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <p className="text-gray-400">Hourly Loss</p>
                  <p className="text-white">
                    ${diagnostics.circuit_breaker.hourly_loss.toFixed(2)} / ${diagnostics.circuit_breaker.hourly_loss_limit}
                  </p>
                  <div className="w-full bg-gray-600 rounded-full h-1 mt-1">
                    <div
                      className={`h-1 rounded-full ${
                        diagnostics.circuit_breaker.hourly_loss / diagnostics.circuit_breaker.hourly_loss_limit > 0.8
                          ? 'bg-red-500' : 'bg-green-500'
                      }`}
                      style={{ width: `${Math.min((diagnostics.circuit_breaker.hourly_loss / diagnostics.circuit_breaker.hourly_loss_limit) * 100, 100)}%` }}
                    />
                  </div>
                </div>
                <div>
                  <p className="text-gray-400">Daily Loss</p>
                  <p className="text-white">
                    ${diagnostics.circuit_breaker.daily_loss.toFixed(2)} / ${diagnostics.circuit_breaker.daily_loss_limit}
                  </p>
                  <div className="w-full bg-gray-600 rounded-full h-1 mt-1">
                    <div
                      className={`h-1 rounded-full ${
                        diagnostics.circuit_breaker.daily_loss / diagnostics.circuit_breaker.daily_loss_limit > 0.8
                          ? 'bg-red-500' : 'bg-green-500'
                      }`}
                      style={{ width: `${Math.min((diagnostics.circuit_breaker.daily_loss / diagnostics.circuit_breaker.daily_loss_limit) * 100, 100)}%` }}
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Signals Tab */}
        {activeTab === 'signals' && (
          <div className="space-y-3">
            {/* Signal Filter */}
            <div className="flex space-x-2">
              {(['all', 'executed', 'rejected'] as const).map((filter) => (
                <button
                  key={filter}
                  onClick={() => setSignalFilter(filter)}
                  className={`px-3 py-1 text-xs rounded-full transition-colors ${
                    signalFilter === filter
                      ? filter === 'executed' ? 'bg-green-500/20 text-green-400'
                        : filter === 'rejected' ? 'bg-red-500/20 text-red-400'
                        : 'bg-purple-500/20 text-purple-400'
                      : 'bg-gray-700 text-gray-400 hover:text-white'
                  }`}
                >
                  {filter.charAt(0).toUpperCase() + filter.slice(1)}
                  {signalStats && filter === 'all' && ` (${signalStats.total})`}
                  {signalStats && filter === 'executed' && ` (${signalStats.executed})`}
                  {signalStats && filter === 'rejected' && ` (${signalStats.rejected})`}
                </button>
              ))}
            </div>

            {/* Signal Stats Bar */}
            {signalStats && signalStats.total > 0 && (
              <div className="bg-gray-700/30 rounded-lg p-2">
                <div className="flex items-center space-x-4 text-xs">
                  <span className="text-gray-400">Execution Rate:</span>
                  <div className="flex-1 bg-gray-600 rounded-full h-2">
                    <div
                      className="bg-green-500 h-2 rounded-full"
                      style={{ width: `${signalStats.execution_rate}%` }}
                    />
                  </div>
                  <span className="text-white font-medium">{signalStats.execution_rate.toFixed(1)}%</span>
                </div>
              </div>
            )}

            {/* Signal List */}
            <div className="space-y-2 max-h-[400px] overflow-y-auto">
              {signals.length === 0 ? (
                <p className="text-center text-gray-400 py-8">No signals generated yet</p>
              ) : (
                signals.map((signal) => (
                  <div
                    key={signal.id || `${signal.symbol}-${signal.timestamp}`}
                    className="bg-gray-700/30 rounded-lg p-3"
                  >
                    <div
                      className="flex items-center justify-between cursor-pointer"
                      onClick={() => setExpandedSignal(expandedSignal === signal.id ? null : signal.id)}
                    >
                      <div className="flex items-center space-x-3">
                        <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                          signal.status === 'executed' ? 'bg-green-500/20 text-green-400' :
                          signal.status === 'rejected' ? 'bg-red-500/20 text-red-400' :
                          'bg-yellow-500/20 text-yellow-400'
                        }`}>
                          {signal.status.toUpperCase()}
                        </span>
                        <span className="text-white font-medium">{signal.symbol}</span>
                        <span className={`text-sm ${signal.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
                          {signal.direction === 'LONG' ? <TrendingUp className="w-4 h-4 inline" /> : <TrendingDown className="w-4 h-4 inline" />}
                          {signal.direction}
                        </span>
                        <span className="text-xs text-gray-400">{signal.mode}</span>
                      </div>
                      <div className="flex items-center space-x-3">
                        <span className="text-sm text-gray-300">{signal.confidence.toFixed(0)}%</span>
                        <span className="text-xs text-gray-400">{formatTime(signal.timestamp)}</span>
                        {expandedSignal === signal.id ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                      </div>
                    </div>

                    {/* Expanded Details */}
                    {expandedSignal === signal.id && (
                      <div className="mt-3 pt-3 border-t border-gray-600 grid grid-cols-2 md:grid-cols-4 gap-3 text-xs">
                        <div>
                          <p className="text-gray-400">Entry Price</p>
                          <p className="text-white">${signal.entry_price.toFixed(4)}</p>
                        </div>
                        <div>
                          <p className="text-gray-400">Stop Loss</p>
                          <p className="text-red-400">${signal.stop_loss.toFixed(4)}</p>
                        </div>
                        <div>
                          <p className="text-gray-400">Take Profit</p>
                          <p className="text-green-400">${signal.take_profit_1.toFixed(4)}</p>
                        </div>
                        <div>
                          <p className="text-gray-400">R:R Ratio</p>
                          <p className="text-white">{signal.risk_reward.toFixed(2)}</p>
                        </div>
                        <div>
                          <p className="text-gray-400">Leverage</p>
                          <p className="text-white">{signal.leverage}x</p>
                        </div>
                        <div>
                          <p className="text-gray-400">Trend</p>
                          <p className="text-white">{signal.trend}</p>
                        </div>
                        <div>
                          <p className="text-gray-400">Volatility</p>
                          <p className="text-white">{signal.volatility}</p>
                        </div>
                        <div>
                          <p className="text-gray-400">Signals Met</p>
                          <p className="text-white">{signal.primary_met}/{signal.primary_required}</p>
                        </div>
                        {signal.rejection_reason && (
                          <div className="col-span-full">
                            <p className="text-gray-400">Rejection Reason</p>
                            <p className="text-red-400">{signal.rejection_reason}</p>
                          </div>
                        )}
                        {signal.signal_names && signal.signal_names.length > 0 && (
                          <div className="col-span-full">
                            <p className="text-gray-400 mb-1">Active Signals</p>
                            <div className="flex flex-wrap gap-1">
                              {signal.signal_names.map((name, idx) => (
                                <span key={idx} className="px-2 py-0.5 bg-gray-600 rounded text-gray-300">{name}</span>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        )}

        {/* Issues Tab */}
        {activeTab === 'issues' && diagnostics && (
          <div className="space-y-3">
            {diagnostics.issues?.length === 0 ? (
              <div className="text-center py-8">
                <CheckCircle className="w-12 h-12 text-green-400 mx-auto mb-2" />
                <p className="text-green-400">All systems operational</p>
              </div>
            ) : (
              <>
                {/* Critical Issues */}
                {criticalIssues.length > 0 && (
                  <div className="space-y-2">
                    <h3 className="text-sm font-medium text-red-400">Critical Issues ({criticalIssues.length})</h3>
                    {criticalIssues.map((issue, idx) => (
                      <IssueCard key={`critical-${idx}`} issue={issue} />
                    ))}
                  </div>
                )}

                {/* Warning Issues */}
                {warningIssues.length > 0 && (
                  <div className="space-y-2">
                    <h3 className="text-sm font-medium text-yellow-400">Warnings ({warningIssues.length})</h3>
                    {warningIssues.map((issue, idx) => (
                      <IssueCard key={`warning-${idx}`} issue={issue} />
                    ))}
                  </div>
                )}

                {/* Info Issues */}
                {infoIssues.length > 0 && (
                  <div className="space-y-2">
                    <h3 className="text-sm font-medium text-blue-400">Info ({infoIssues.length})</h3>
                    {infoIssues.map((issue, idx) => (
                      <IssueCard key={`info-${idx}`} issue={issue} />
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// Issue Card Component
function IssueCard({ issue }: { issue: DiagnosticIssue }) {
  const getSeverityStyles = (severity: string) => {
    switch (severity) {
      case 'critical': return 'border-red-500/50 bg-red-500/10';
      case 'warning': return 'border-yellow-500/50 bg-yellow-500/10';
      case 'info': return 'border-blue-500/50 bg-blue-500/10';
      default: return 'border-gray-500/50 bg-gray-500/10';
    }
  };

  const getSeverityIcon = (severity: string) => {
    switch (severity) {
      case 'critical': return <AlertOctagon className="w-4 h-4 text-red-400" />;
      case 'warning': return <AlertTriangle className="w-4 h-4 text-yellow-400" />;
      case 'info': return <Info className="w-4 h-4 text-blue-400" />;
      default: return <Info className="w-4 h-4 text-gray-400" />;
    }
  };

  return (
    <div className={`rounded-lg border p-3 ${getSeverityStyles(issue.severity)}`}>
      <div className="flex items-start space-x-2">
        {getSeverityIcon(issue.severity)}
        <div className="flex-1">
          <p className="text-sm text-white">{issue.message}</p>
          <p className="text-xs text-gray-400 mt-1">
            <span className="text-gray-500">Suggestion:</span> {issue.suggestion}
          </p>
          <span className="inline-block mt-1 px-2 py-0.5 bg-gray-700 rounded text-xs text-gray-400">
            {issue.category}
          </span>
        </div>
      </div>
    </div>
  );
}
