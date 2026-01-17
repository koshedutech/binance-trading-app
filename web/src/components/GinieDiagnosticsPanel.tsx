import { useEffect, useState, useRef, useCallback } from 'react';
import {
  futuresApi,
  GinieDiagnostics,
  GinieSignalLog,
  GinieSignalStats,
  DiagnosticIssue,
  TradeConditionsResponse,
  PendingOrdersResponse,
} from '../services/futuresApi';
import { useFuturesStore } from '../store/futuresStore';
import { wsService } from '../services/websocket';
import type { WSEvent, GinieStatusPayload } from '../types';
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
  Lock,
  Unlock,
  Clock,
  Check,
  X,
} from 'lucide-react';

export default function GinieDiagnosticsPanel() {
  const [diagnostics, setDiagnostics] = useState<GinieDiagnostics | null>(null);
  const [signals, setSignals] = useState<GinieSignalLog[]>([]);
  const [signalStats, setSignalStats] = useState<GinieSignalStats | null>(null);
  const [tradeConditions, setTradeConditions] = useState<TradeConditionsResponse | null>(null);
  const [pendingOrders, setPendingOrders] = useState<PendingOrdersResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<'overview' | 'signals' | 'issues'>('overview');
  const [expandedSignal, setExpandedSignal] = useState<string | null>(null);
  const [signalFilter, setSignalFilter] = useState<'all' | 'executed' | 'rejected'>('all');
  const [showKillSwitchDetails, setShowKillSwitchDetails] = useState(false);
  const [resettingSymbol, setResettingSymbol] = useState<string | null>(null);
  const [holdSignals, setHoldSignals] = useState(false);
  const [showConditionsDetail, setShowConditionsDetail] = useState(false);

  // CRITICAL: Subscribe to trading mode changes to refresh mode-specific data (paper vs live)
  const tradingMode = useFuturesStore((state) => state.tradingMode);

  // Expandable section states - all collapsed by default
  const [showTradeConditions, setShowTradeConditions] = useState(false);
  const [showPendingOrders, setShowPendingOrders] = useState(false);
  const [showScanningActivity, setShowScanningActivity] = useState(false);
  const [showLLMStatus, setShowLLMStatus] = useState(false);
  const [showProfitBooking, setShowProfitBooking] = useState(false);
  const [showCircuitBreaker, setShowCircuitBreaker] = useState(false);

  // Ref to track current filter for interval callback
  const signalFilterRef = useRef(signalFilter);
  signalFilterRef.current = signalFilter;

  // Ref to track hold state for interval callback
  const holdSignalsRef = useRef(holdSignals);
  holdSignalsRef.current = holdSignals;

  // Ref for WebSocket fallback polling interval
  const fallbackRef = useRef<NodeJS.Timeout | null>(null);

  const fetchDiagnostics = async () => {
    try {
      const data = await futuresApi.getGinieDiagnostics();
      setDiagnostics(data);
    } catch (err) {
      console.error('Failed to fetch diagnostics:', err);
    }
  };

  const fetchTradeConditions = async () => {
    try {
      const data = await futuresApi.getGinieTradeConditions();
      setTradeConditions(data);
    } catch (err) {
      console.error('Failed to fetch trade conditions:', err);
    }
  };

  const fetchPendingOrders = async () => {
    try {
      const data = await futuresApi.getGiniePendingOrders();
      setPendingOrders(data);
    } catch (err) {
      console.error('Failed to fetch pending orders:', err);
    }
  };

  const fetchSignals = useCallback(async (forceRefresh = false) => {
    // Don't refresh signals if hold is active (unless forced)
    if (holdSignalsRef.current && !forceRefresh) {
      return;
    }
    try {
      const statusFilter = signalFilterRef.current === 'all' ? undefined : signalFilterRef.current;
      const { signals: signalData } = await futuresApi.getGinieSignalLogs(200, statusFilter);
      setSignals(signalData || []);
      const stats = await futuresApi.getGinieSignalStats();
      setSignalStats(stats);
    } catch (err) {
      console.error('Failed to fetch signals:', err);
    }
  }, []);

  const refreshAll = useCallback(async (forceRefresh = false) => {
    setLoading(true);
    await Promise.all([
      fetchDiagnostics(),
      fetchSignals(forceRefresh),
      fetchTradeConditions(),
      fetchPendingOrders(),
    ]);
    setLoading(false);
  }, [fetchSignals]);

  const resetLLMKillSwitch = async (symbol: string) => {
    try {
      setResettingSymbol(symbol);
      await futuresApi.resetGinieLLMSL(symbol);
      await fetchDiagnostics(); // Refresh to show updated state
    } catch (err) {
      console.error('Failed to reset LLM kill switch:', err);
    } finally {
      setResettingSymbol(null);
    }
  };

  const resetAllLLMKillSwitches = async () => {
    const symbols = diagnostics?.llm_status?.disabled_symbols || [];
    setResettingSymbol('all');
    try {
      for (const symbol of symbols) {
        await futuresApi.resetGinieLLMSL(symbol);
      }
      await fetchDiagnostics();
    } catch (err) {
      console.error('Failed to reset all LLM kill switches:', err);
    } finally {
      setResettingSymbol(null);
    }
  };

  // Initial fetch and WebSocket subscription (Story 12.5 - replace polling with WebSocket)
  useEffect(() => {
    // Initial data fetch
    refreshAll(true);

    // WebSocket handler for Ginie status updates
    const handleGinieStatusUpdate = (event: WSEvent) => {
      const status = event.data.status as GinieStatusPayload;
      if (status) {
        // Refresh diagnostics data when status changes
        refreshAll(false);
      }
    };

    // Subscribe to Ginie status WebSocket events
    wsService.subscribe('GINIE_STATUS_UPDATE', handleGinieStatusUpdate);

    // Fallback: 60s polling only when WebSocket disconnected
    const startFallback = () => {
      if (!fallbackRef.current) {
        fallbackRef.current = setInterval(() => refreshAll(false), 60000);
      }
    };

    const stopFallback = () => {
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
      // Refresh on reconnect to get latest data
      refreshAll(false);
    };

    wsService.onDisconnect(startFallback);
    wsService.onConnect(stopFallback);

    // Start fallback if WebSocket not connected initially
    if (!wsService.isConnected()) {
      startFallback();
    }

    return () => {
      wsService.unsubscribe('GINIE_STATUS_UPDATE', handleGinieStatusUpdate);
      wsService.offConnect(stopFallback);
      wsService.offDisconnect(startFallback);
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
    };
  }, []); // Empty dependency array - only runs once on mount

  // CRITICAL: Refresh all diagnostics when trading mode changes (paper <-> live)
  // This ensures daily loss, trade counts, and other mode-specific stats update immediately
  useEffect(() => {
    console.log('GinieDiagnosticsPanel: Trading mode changed to', tradingMode.mode, '- refreshing all data');
    refreshAll(true);
  }, [tradingMode.dryRun]); // Watch specifically dryRun to detect paper/live switch

  // Fetch signals when filter changes (separate from interval)
  useEffect(() => {
    // Force refresh when filter changes
    fetchSignals(true);
  }, [signalFilter, fetchSignals]);

  const formatTime = (timestamp: string) => {
    if (!timestamp || timestamp === '0001-01-01T00:00:00Z') return 'Never';
    const date = new Date(timestamp);
    return date.toLocaleTimeString();
  };

  const formatTimeAgo = (timestamp: string) => {
    if (!timestamp || timestamp === '0001-01-01T00:00:00Z') return 'Unknown';
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);

    if (diffSecs < 60) return `${diffSecs}s ago`;
    if (diffMins < 60) return `${diffMins}m ago`;
    return `${diffHours}h ${diffMins % 60}m ago`;
  };

  const criticalIssues = diagnostics?.issues?.filter(i => i.severity === 'critical') || [];
  const warningIssues = diagnostics?.issues?.filter(i => i.severity === 'warning') || [];
  const infoIssues = diagnostics?.issues?.filter(i => i.severity === 'info') || [];

  // Calculate conditions summary
  const conditionsMet = tradeConditions?.conditions?.filter(c => c.passed).length || 0;
  const totalConditions = tradeConditions?.conditions?.length || 0;
  const blockingConditions = tradeConditions?.conditions?.filter(c => !c.passed) || [];

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
          onClick={() => refreshAll(true)}
          disabled={loading}
          className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {/* Circuit Status Summary - New clickable summary bar */}
      {tradeConditions && (
        <div
          onClick={() => setShowConditionsDetail(!showConditionsDetail)}
          className={`p-2 rounded-lg cursor-pointer transition-colors ${
            tradeConditions.all_passed
              ? 'bg-green-500/10 hover:bg-green-500/20 border border-green-500/30'
              : 'bg-red-500/10 hover:bg-red-500/20 border border-red-500/30'
          }`}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              {tradeConditions.all_passed ? (
                <CheckCircle className="w-4 h-4 text-green-400" />
              ) : (
                <XCircle className="w-4 h-4 text-red-400" />
              )}
              <span className={`text-sm font-medium ${
                tradeConditions.all_passed ? 'text-green-400' : 'text-red-400'
              }`}>
                {tradeConditions.all_passed
                  ? `All ${totalConditions} conditions met - Ready to trade`
                  : `${tradeConditions.blocking_count} condition${tradeConditions.blocking_count !== 1 ? 's' : ''} blocking - Cannot trade`
                }
              </span>
            </div>
            <div className="flex items-center space-x-2">
              <span className="text-xs text-gray-400">{conditionsMet}/{totalConditions}</span>
              {showConditionsDetail ? (
                <ChevronUp className="w-4 h-4 text-gray-400" />
              ) : (
                <ChevronDown className="w-4 h-4 text-gray-400" />
              )}
            </div>
          </div>

          {/* Expandable blocking conditions detail */}
          {showConditionsDetail && blockingConditions.length > 0 && (
            <div className="mt-2 pt-2 border-t border-red-500/20 space-y-1">
              {blockingConditions.map((condition, idx) => (
                <div key={idx} className="flex items-center space-x-2 text-xs">
                  <X className="w-3 h-3 text-red-400 flex-shrink-0" />
                  <span className="text-red-300">{condition.name}:</span>
                  <span className="text-gray-400">{condition.detail}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

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
            {/* Trading Conditions Checklist - Collapsible Section */}
            <div className="bg-gray-700/30 rounded-lg overflow-hidden">
              <div
                onClick={() => setShowTradeConditions(!showTradeConditions)}
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
              >
                <h3 className="text-sm font-medium text-gray-300 flex items-center">
                  <Check className="w-4 h-4 mr-2" />
                  Trading Conditions Checklist
                  {tradeConditions && (
                    <span className="ml-2 text-xs text-gray-400">
                      ({conditionsMet}/{totalConditions} passed)
                    </span>
                  )}
                </h3>
                {showTradeConditions ? (
                  <ChevronUp className="w-4 h-4 text-gray-400" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                )}
              </div>
              {showTradeConditions && (
                <div className="px-3 pb-3">
                  {tradeConditions ? (
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                      {tradeConditions.conditions.map((condition, idx) => (
                        <div
                          key={idx}
                          className={`flex items-center space-x-2 p-2 rounded ${
                            condition.passed ? 'bg-green-500/10' : 'bg-red-500/10'
                          }`}
                        >
                          {condition.passed ? (
                            <Check className="w-4 h-4 text-green-400 flex-shrink-0" />
                          ) : (
                            <X className="w-4 h-4 text-red-400 flex-shrink-0" />
                          )}
                          <div className="min-w-0">
                            <p className={`text-xs font-medium truncate ${
                              condition.passed ? 'text-green-400' : 'text-red-400'
                            }`}>
                              {condition.name}
                            </p>
                            <p className="text-xs text-gray-400 truncate" title={condition.detail}>
                              {condition.detail}
                            </p>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-xs text-gray-400">Loading conditions...</p>
                  )}
                </div>
              )}
            </div>

            {/* Pending Limit Orders - Collapsible Section */}
            {pendingOrders && pendingOrders.count > 0 && (
              <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg overflow-hidden">
                <div
                  onClick={() => setShowPendingOrders(!showPendingOrders)}
                  className="flex items-center justify-between p-3 cursor-pointer hover:bg-yellow-500/20 transition-colors"
                >
                  <h3 className="text-sm font-medium text-yellow-400 flex items-center">
                    <Clock className="w-4 h-4 mr-2" />
                    Pending Limit Orders ({pendingOrders.count})
                  </h3>
                  {showPendingOrders ? (
                    <ChevronUp className="w-4 h-4 text-yellow-400" />
                  ) : (
                    <ChevronDown className="w-4 h-4 text-yellow-400" />
                  )}
                </div>
                {showPendingOrders && (
                  <div className="px-3 pb-3">
                    <p className="text-xs text-gray-400 mb-2">
                      These signals were executed but are waiting for LIMIT orders to fill. They will appear in positions once filled.
                    </p>
                    <div className="space-y-2">
                      {pendingOrders.pending_orders.map((order, idx) => (
                        <div key={idx} className="flex items-center justify-between bg-gray-800/50 p-2 rounded">
                          <div className="flex items-center space-x-3">
                            <span className="text-white font-medium">{order.symbol}</span>
                            <span className={`text-xs px-2 py-0.5 rounded ${
                              order.direction === 'LONG'
                                ? 'bg-green-500/20 text-green-400'
                                : 'bg-red-500/20 text-red-400'
                            }`}>
                              {order.direction}
                            </span>
                            <span className="text-xs text-gray-400">{order.mode}</span>
                          </div>
                          <div className="flex items-center space-x-4 text-xs">
                            <div>
                              <span className="text-gray-400">Entry: </span>
                              <span className="text-white">${order.entry_price.toFixed(4)}</span>
                            </div>
                            <div>
                              <span className="text-gray-400">Waiting: </span>
                              <span className="text-yellow-400">{formatTimeAgo(order.placed_at)}</span>
                            </div>
                            <div>
                              <span className="text-gray-400">Timeout: </span>
                              <span className={order.seconds_left < 60 ? 'text-red-400' : 'text-gray-300'}>
                                {order.seconds_left}s
                              </span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Scanning Activity - Collapsible Section */}
            <div className="bg-gray-700/30 rounded-lg overflow-hidden">
              <div
                onClick={() => setShowScanningActivity(!showScanningActivity)}
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
              >
                <h3 className="text-sm font-medium text-gray-300 flex items-center">
                  <Eye className="w-4 h-4 mr-2" />
                  Scanning Activity
                  <span className="ml-2 text-xs text-gray-400">
                    ({diagnostics.scanning?.symbols_in_watchlist ?? 0} symbols)
                  </span>
                </h3>
                {showScanningActivity ? (
                  <ChevronUp className="w-4 h-4 text-gray-400" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                )}
              </div>
              {showScanningActivity && (
                <div className="px-3 pb-3">
                  <div className="grid grid-cols-3 gap-4 text-sm">
                    <div>
                      <p className="text-gray-400">Last Scan</p>
                      <p className="text-white">{formatTime(diagnostics.scanning?.last_scan_time ?? '')}</p>
                    </div>
                    <div>
                      <p className="text-gray-400">Symbols</p>
                      <p className="text-white">{diagnostics.scanning?.symbols_in_watchlist ?? 0}</p>
                    </div>
                    <div>
                      <p className="text-gray-400">Modes</p>
                      <div className="flex space-x-1">
                        {diagnostics.scanning?.ultra_fast_enabled && <span className="text-xs bg-yellow-500/20 text-yellow-400 px-1 rounded">UF</span>}
                        {diagnostics.scanning?.scalp_enabled && <span className="text-xs bg-blue-500/20 text-blue-400 px-1 rounded">S</span>}
                        {diagnostics.scanning?.swing_enabled && <span className="text-xs bg-green-500/20 text-green-400 px-1 rounded">W</span>}
                        {diagnostics.scanning?.position_enabled && <span className="text-xs bg-purple-500/20 text-purple-400 px-1 rounded">P</span>}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* LLM Status - Collapsible Section */}
            <div className="bg-gray-700/30 rounded-lg overflow-hidden">
              <div
                onClick={() => setShowLLMStatus(!showLLMStatus)}
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
              >
                <h3 className="text-sm font-medium text-gray-300 flex items-center">
                  <Radio className="w-4 h-4 mr-2" />
                  LLM Status
                  <span className="ml-2 text-xs">
                    {diagnostics.llm_status?.connected ? (
                      <span className="text-green-400">({diagnostics.llm_status?.provider})</span>
                    ) : (
                      <span className="text-red-400">(Not Connected)</span>
                    )}
                  </span>
                  {(diagnostics.llm_status?.disabled_symbols?.length ?? 0) > 0 && (
                    <span className="ml-2 text-xs text-red-400">
                      ({diagnostics.llm_status?.disabled_symbols?.length} kill switches)
                    </span>
                  )}
                </h3>
                {showLLMStatus ? (
                  <ChevronUp className="w-4 h-4 text-gray-400" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                )}
              </div>
              {showLLMStatus && (
                <div className="px-3 pb-3">
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center space-x-2">
                      {diagnostics.llm_status?.connected ? (
                        <CheckCircle className="w-4 h-4 text-green-400" />
                      ) : (
                        <XCircle className="w-4 h-4 text-red-400" />
                      )}
                      <span className="text-sm text-white">
                        {diagnostics.llm_status?.connected ? diagnostics.llm_status?.provider : 'Not Connected'}
                      </span>
                    </div>
                    {(diagnostics.llm_status?.disabled_symbols?.length ?? 0) > 0 && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setShowKillSwitchDetails(!showKillSwitchDetails);
                        }}
                        className="flex items-center space-x-1 text-xs text-red-400 hover:text-red-300 transition-colors"
                      >
                        <AlertOctagon className="w-3 h-3" />
                        <span>{diagnostics.llm_status?.disabled_symbols?.length ?? 0} kill switches active</span>
                        {showKillSwitchDetails ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                      </button>
                    )}
                  </div>

                  {/* Expandable Kill Switch Details */}
                  {showKillSwitchDetails && (diagnostics.llm_status?.disabled_symbols?.length ?? 0) > 0 && (
                    <div className="mt-3 pt-3 border-t border-gray-600">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-xs text-gray-400">Symbols with LLM SL updates disabled:</span>
                        <button
                          onClick={resetAllLLMKillSwitches}
                          disabled={resettingSymbol === 'all'}
                          className="text-xs bg-red-500/20 text-red-400 hover:bg-red-500/30 px-2 py-1 rounded flex items-center space-x-1 disabled:opacity-50"
                        >
                          <RefreshCw className={`w-3 h-3 ${resettingSymbol === 'all' ? 'animate-spin' : ''}`} />
                          <span>Reset All</span>
                        </button>
                      </div>
                      <div className="space-y-1">
                        {diagnostics.llm_status?.disabled_symbols?.map((symbol) => (
                          <div key={symbol} className="flex items-center justify-between bg-gray-800/50 px-2 py-1 rounded">
                            <span className="text-sm text-white font-mono">{symbol}</span>
                            <button
                              onClick={() => resetLLMKillSwitch(symbol)}
                              disabled={resettingSymbol === symbol}
                              className="text-xs bg-blue-500/20 text-blue-400 hover:bg-blue-500/30 px-2 py-0.5 rounded flex items-center space-x-1 disabled:opacity-50"
                            >
                              <RefreshCw className={`w-3 h-3 ${resettingSymbol === symbol ? 'animate-spin' : ''}`} />
                              <span>Reset</span>
                            </button>
                          </div>
                        ))}
                      </div>
                      <p className="text-xs text-gray-500 mt-2">
                        Kill switch activates after 3 consecutive bad LLM SL calls. Manual reset required.
                      </p>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* Profit Booking - Collapsible Section */}
            <div className="bg-gray-700/30 rounded-lg overflow-hidden">
              <div
                onClick={() => setShowProfitBooking(!showProfitBooking)}
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
              >
                <h3 className="text-sm font-medium text-gray-300 flex items-center">
                  <TrendingUp className="w-4 h-4 mr-2" />
                  Profit Booking (1h)
                  <span className="ml-2 text-xs text-gray-400">
                    ({diagnostics.profit_booking?.tp_hits_last_hour ?? 0} TPs hit)
                  </span>
                </h3>
                {showProfitBooking ? (
                  <ChevronUp className="w-4 h-4 text-gray-400" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                )}
              </div>
              {showProfitBooking && (
                <div className="px-3 pb-3">
                  <div className="grid grid-cols-4 gap-4 text-sm">
                    <div>
                      <p className="text-gray-400">Pending TPs</p>
                      <p className="text-white">{diagnostics.profit_booking?.positions_with_pending_tp ?? 0}</p>
                    </div>
                    <div>
                      <p className="text-gray-400">TP Hits</p>
                      <p className="text-green-400">{diagnostics.profit_booking?.tp_hits_last_hour ?? 0}</p>
                    </div>
                    <div>
                      <p className="text-gray-400">Partials</p>
                      <p className="text-blue-400">{diagnostics.profit_booking?.partial_closes_last_hour ?? 0}</p>
                    </div>
                    <div>
                      <p className="text-gray-400">Failed</p>
                      <p className={(diagnostics.profit_booking?.failed_closes_last_hour ?? 0) > 0 ? 'text-red-400' : 'text-gray-400'}>
                        {diagnostics.profit_booking?.failed_closes_last_hour ?? 0}
                      </p>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Circuit Breaker Details - Collapsible Section */}
            <div className="bg-gray-700/30 rounded-lg overflow-hidden">
              <div
                onClick={() => setShowCircuitBreaker(!showCircuitBreaker)}
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
              >
                <h3 className="text-sm font-medium text-gray-300 flex items-center">
                  <Shield className="w-4 h-4 mr-2" />
                  Circuit Breaker Details
                  <span className={`ml-2 text-xs ${
                    diagnostics.circuit_breaker.state === 'open' ? 'text-red-400' : 'text-green-400'
                  }`}>
                    ({diagnostics.circuit_breaker.state.toUpperCase()})
                  </span>
                </h3>
                {showCircuitBreaker ? (
                  <ChevronUp className="w-4 h-4 text-gray-400" />
                ) : (
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                )}
              </div>
              {showCircuitBreaker && (
                <div className="px-3 pb-3">
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
              )}
            </div>
          </div>
        )}

        {/* Signals Tab */}
        {activeTab === 'signals' && (
          <div className="space-y-3">
            {/* Signal Filter with Hold Toggle */}
            <div className="flex items-center justify-between">
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

              {/* Hold/Pin Toggle */}
              <div className="flex items-center space-x-2">
                <span className="text-xs text-gray-400">
                  {signals.length} signal{signals.length !== 1 ? 's' : ''} visible
                </span>
                <button
                  onClick={() => setHoldSignals(!holdSignals)}
                  className={`flex items-center space-x-1 px-2 py-1 text-xs rounded transition-colors ${
                    holdSignals
                      ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30'
                      : 'bg-gray-700 text-gray-400 hover:text-white'
                  }`}
                  title={holdSignals ? 'Auto-refresh paused - click to resume' : 'Click to hold signals and pause auto-refresh'}
                >
                  {holdSignals ? (
                    <>
                      <Lock className="w-3 h-3" />
                      <span>Held</span>
                    </>
                  ) : (
                    <>
                      <Unlock className="w-3 h-3" />
                      <span>Hold</span>
                    </>
                  )}
                </button>
                {holdSignals && (
                  <button
                    onClick={() => fetchSignals(true)}
                    className="p-1 text-gray-400 hover:text-white transition-colors"
                    title="Manual refresh"
                  >
                    <RefreshCw className="w-3 h-3" />
                  </button>
                )}
              </div>
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
            <div className="space-y-2 max-h-[600px] overflow-y-auto">
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
