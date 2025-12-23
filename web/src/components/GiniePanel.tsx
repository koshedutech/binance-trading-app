import { useEffect, useState } from 'react';
import { futuresApi, formatUSD, GinieStatus, GinieCoinScan, GinieAutopilotStatus, GiniePosition, GinieTradeResult, GinieCircuitBreakerStatus, MarketMoversResponse, GinieDiagnostics, GinieSignalLog, GinieSignalStats } from '../services/futuresApi';
import {
  Sparkles, Power, PowerOff, RefreshCw, Shield, CheckCircle, XCircle,
  ChevronDown, ChevronUp, Zap, Clock, BarChart3, Play, Square, Target,
  Trash2, AlertOctagon, ToggleLeft, ToggleRight, Settings, Activity, Download,
  TrendingUp, TrendingDown, BarChart2, Flame, Stethoscope, AlertTriangle, Info, Eye, Radio,
  ListChecks
} from 'lucide-react';
import SymbolPerformancePanel from './SymbolPerformancePanel';

export default function GiniePanel() {
  const [status, setStatus] = useState<GinieStatus | null>(null);
  const [autopilotStatus, setAutopilotStatus] = useState<GinieAutopilotStatus | null>(null);
  const [circuitBreaker, setCircuitBreaker] = useState<GinieCircuitBreakerStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [scanning, setScanning] = useState(false);
  const [analyzing, setAnalyzing] = useState(false);
  const [togglingMode, setTogglingMode] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [panicking, setPanicking] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [expandedDecision, setExpandedDecision] = useState<string | null>(null);
  const [expandedPosition, setExpandedPosition] = useState<string | null>(null);
  const [coinScans, setCoinScans] = useState<GinieCoinScan[]>([]);
  const [showScans, setShowScans] = useState(false);
  const [activeTab, setActiveTab] = useState<'decisions' | 'positions' | 'history' | 'movers' | 'diagnostics' | 'performance'>('decisions');
  // Diagnostics state
  const [diagnostics, setDiagnostics] = useState<GinieDiagnostics | null>(null);
  const [signalLogs, setSignalLogs] = useState<GinieSignalLog[]>([]);
  const [signalStats, setSignalStats] = useState<GinieSignalStats | null>(null);
  const [signalFilter, setSignalFilter] = useState<'all' | 'executed' | 'rejected'>('all');
  const [expandedSignal, setExpandedSignal] = useState<string | null>(null);
  const [confidenceThreshold, setConfidenceThreshold] = useState<number>(65);
  const [savingConfig, setSavingConfig] = useState(false);
  // Circuit Breaker UI state
  const [showCBConfig, setShowCBConfig] = useState(false);
  const [cbConfig, setCBConfig] = useState({
    max_loss_per_hour: 100,
    max_daily_loss: 300,
    max_consecutive_losses: 3,
    cooldown_minutes: 30,
  });
  const [savingCB, setSavingCB] = useState(false);
  const [togglingCB, setTogglingCB] = useState(false);
  // Risk Level state
  const [riskLevel, setRiskLevel] = useState<string>('moderate');
  const [savingRiskLevel, setSavingRiskLevel] = useState(false);
  // Market Movers state
  const [marketMovers, setMarketMovers] = useState<MarketMoversResponse | null>(null);
  const [loadingMovers, setLoadingMovers] = useState(false);
  const [refreshingSymbols, setRefreshingSymbols] = useState(false);
  // Position sizing state
  const [maxUsdPerPosition, setMaxUsdPerPosition] = useState<number>(500);
  const [maxPositions, setMaxPositions] = useState<number>(5);
  const [leverage, setLeverage] = useState<number>(5);
  const [savingPositionSize, setSavingPositionSize] = useState(false);
  const [settingsInitialized, setSettingsInitialized] = useState(false);
  // Auto Size Mode state
  const [autoSizeMode, setAutoSizeMode] = useState(false);
  const [categoryMultipliers, setCategoryMultipliers] = useState({
    best: 1.5,
    good: 1.2,
    neutral: 1.0,
    poor: 0.5,
    worst: 0.25,
  });
  // Editing mode flags - prevent API overwriting user input
  const [isEditingPositionSize, setIsEditingPositionSize] = useState(false);
  const [isEditingCBConfig, setIsEditingCBConfig] = useState(false);
  // Source filter for positions/history (AI vs Strategy)
  const [sourceFilter, setSourceFilter] = useState<'all' | 'ai' | 'strategy'>('all');

  const isRunning = autopilotStatus?.stats?.running ?? false;
  const isDryRun = autopilotStatus?.config?.dry_run ?? true;

  const fetchStatus = async () => {
    try {
      const data = await futuresApi.getGinieStatus();
      setStatus(data);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch Ginie status:', err);
    }
  };

  const fetchAutopilotStatus = async (initSettings = false) => {
    try {
      const data = await futuresApi.getGinieAutopilotStatus();
      setAutopilotStatus(data);

      // Only initialize settings from API on first load AND when not actively editing
      // This prevents overwriting user input while they're typing
      if (initSettings && !settingsInitialized && !isEditingPositionSize && data.config) {
        if (data.config.min_confidence_to_trade !== undefined) {
          setConfidenceThreshold(data.config.min_confidence_to_trade);
        }
        if (data.config.risk_level) {
          setRiskLevel(data.config.risk_level);
        }
        if (data.config.max_usd_per_position !== undefined) {
          setMaxUsdPerPosition(data.config.max_usd_per_position);
        }
        if (data.config.max_positions !== undefined) {
          setMaxPositions(data.config.max_positions);
        }
        if (data.config.default_leverage !== undefined) {
          setLeverage(data.config.default_leverage);
        }
        setSettingsInitialized(true);
      }
    } catch (err) {
      console.error('Failed to fetch Ginie autopilot status:', err);
    }
  };

  const fetchCircuitBreaker = async () => {
    try {
      const data = await futuresApi.getGinieCircuitBreakerStatus();
      setCircuitBreaker(data);
      // Update cbConfig with current values ONLY if not actively editing
      if (data && !isEditingCBConfig) {
        setCBConfig({
          max_loss_per_hour: data.max_loss_per_hour,
          max_daily_loss: data.max_daily_loss,
          max_consecutive_losses: data.max_consecutive,
          cooldown_minutes: data.cooldown_minutes,
        });
      }
    } catch (err) {
      console.error('Failed to fetch Ginie circuit breaker status:', err);
    }
  };

  const fetchMarketMovers = async () => {
    try {
      const data = await futuresApi.getMarketMovers(15);
      setMarketMovers(data);
    } catch (err) {
      console.error('Failed to fetch market movers:', err);
    }
  };

  const fetchDiagnostics = async () => {
    try {
      const data = await futuresApi.getGinieDiagnostics();
      setDiagnostics(data);
    } catch (err) {
      console.error('Failed to fetch diagnostics:', err);
    }
  };

  const fetchSignalLogs = async () => {
    try {
      const statusFilter = signalFilter === 'all' ? undefined : signalFilter;
      const { signals } = await futuresApi.getGinieSignalLogs(50, statusFilter);
      setSignalLogs(signals || []);
      // Fetch stats separately
      const stats = await futuresApi.getGinieSignalStats();
      setSignalStats(stats);
    } catch (err) {
      console.error('Failed to fetch signal logs:', err);
    }
  };

  const syncPositionsOnLoad = async () => {
    try {
      await futuresApi.syncGiniePositions();
      await fetchAutopilotStatus();
    } catch (err) {
      console.error('Failed to sync positions on load:', err);
    }
  };

  useEffect(() => {
    fetchStatus();
    fetchAutopilotStatus(true); // Initialize settings on first load
    fetchCircuitBreaker();
    fetchMarketMovers();
    fetchDiagnostics();
    fetchSignalLogs();
    syncPositionsOnLoad(); // Auto-sync positions on mount
    const interval = setInterval(() => {
      fetchStatus();
      fetchAutopilotStatus(false); // Don't overwrite user input on subsequent fetches
      fetchCircuitBreaker();
      if (activeTab === 'diagnostics') {
        fetchDiagnostics();
        fetchSignalLogs();
      }
    }, 10000);
    return () => clearInterval(interval);
  }, [activeTab]);

  // Refetch signals when filter changes
  useEffect(() => {
    fetchSignalLogs();
  }, [signalFilter]);

  const handleToggle = async () => {
    if (!status) return;
    setLoading(true);
    try {
      const result = await futuresApi.toggleGinie(!status.enabled);
      setSuccessMsg(result.message);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchStatus();
    } catch (err) {
      setError('Failed to toggle Ginie');
    } finally {
      setLoading(false);
    }
  };

  const handleStartAutopilot = async () => {
    setLoading(true);
    try {
      const result = await futuresApi.startGinieAutopilot();
      setSuccessMsg(result.message);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to start autopilot');
    } finally {
      setLoading(false);
    }
  };

  const handleStopAutopilot = async () => {
    setLoading(true);
    try {
      const result = await futuresApi.stopGinieAutopilot();
      setSuccessMsg(result.message);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to stop autopilot');
    } finally {
      setLoading(false);
    }
  };

  const handleScanAll = async () => {
    setScanning(true);
    try {
      const result = await futuresApi.ginieScanAll();
      setCoinScans(result.scans);
      setShowScans(true);
      setSuccessMsg(`Scanned ${result.count} coins`);
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to scan coins');
    } finally {
      setScanning(false);
    }
  };

  const handleAnalyzeAll = async () => {
    setAnalyzing(true);
    try {
      const result = await futuresApi.ginieAnalyzeAll();
      await fetchStatus();
      let msg = `Analyzed ${result.count} coins`;
      if (result.best_long) {
        msg += ` | Best Long: ${result.best_long.symbol} (${Number(result.best_long.confidence_score || 0).toFixed(0)}%)`;
      }
      if (result.best_short) {
        msg += ` | Best Short: ${result.best_short.symbol} (${Number(result.best_short.confidence_score || 0).toFixed(0)}%)`;
      }
      setSuccessMsg(msg);
      setTimeout(() => setSuccessMsg(null), 5000);
    } catch (err) {
      setError('Failed to analyze coins');
    } finally {
      setAnalyzing(false);
    }
  };

  const handleToggleMode = async () => {
    setTogglingMode(true);
    try {
      const newDryRun = !isDryRun;
      await futuresApi.setGinieDryRun(newDryRun);
      setSuccessMsg(newDryRun ? 'Switched to PAPER mode' : 'Switched to LIVE mode');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to toggle mode');
    } finally {
      setTogglingMode(false);
    }
  };

  const handleClearPositions = async () => {
    if (!confirm('Clear all Ginie tracked positions and stats?')) return;
    setClearing(true);
    try {
      const result = await futuresApi.clearGiniePositions();
      setSuccessMsg(result.message);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to clear positions');
    } finally {
      setClearing(false);
    }
  };

  const handlePanicExit = async () => {
    if (!confirm('GINIE PANIC EXIT: Close all Ginie-managed positions?')) return;
    setPanicking(true);
    try {
      const result = await futuresApi.closeAllGiniePositions();
      setSuccessMsg(`Ginie panic exit: Closed ${result.positions_closed} positions, PnL: ${formatUSD(result.total_pnl)}`);
      setTimeout(() => setSuccessMsg(null), 5000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Ginie panic exit failed!');
    } finally {
      setPanicking(false);
    }
  };

  const handleSyncPositions = async () => {
    setSyncing(true);
    try {
      const result = await futuresApi.syncGiniePositions();
      if (result.synced_count > 0) {
        setSuccessMsg(`Synced ${result.synced_count} positions from exchange`);
      } else {
        setSuccessMsg('Positions are in sync');
      }
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to sync positions');
    } finally {
      setSyncing(false);
    }
  };

  const handleSetRiskLevel = async (level: string) => {
    setSavingRiskLevel(true);
    try {
      const result = await futuresApi.setGinieRiskLevel(level);
      setRiskLevel(level);
      setSuccessMsg(`Risk level set to ${level.toUpperCase()}: Confidence ${result.min_confidence}%, Max $${result.max_usd}, ${result.leverage}x leverage`);
      setTimeout(() => setSuccessMsg(null), 4000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to set risk level');
    } finally {
      setSavingRiskLevel(false);
    }
  };

  const handleFetchMarketMovers = async () => {
    setLoadingMovers(true);
    try {
      const data = await futuresApi.getMarketMovers(15);
      setMarketMovers(data);
      setSuccessMsg('Market movers updated');
      setTimeout(() => setSuccessMsg(null), 2000);
    } catch (err) {
      setError('Failed to fetch market movers');
    } finally {
      setLoadingMovers(false);
    }
  };

  const handleRefreshDynamicSymbols = async () => {
    setRefreshingSymbols(true);
    try {
      const result = await futuresApi.refreshDynamicSymbols(15);
      setSuccessMsg(`Watch list updated: ${result.symbol_count} symbols`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchMarketMovers();
    } catch (err) {
      setError('Failed to refresh dynamic symbols');
    } finally {
      setRefreshingSymbols(false);
    }
  };

  const handleSavePositionSize = async () => {
    setSavingPositionSize(true);
    try {
      await futuresApi.updateGinieAutopilotConfig({
        max_usd_per_position: maxUsdPerPosition,
        max_positions: maxPositions,
        default_leverage: leverage,
      });
      setSuccessMsg(`$${maxUsdPerPosition} x ${maxPositions} pos @ ${leverage}x leverage`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus(false);
    } catch (err) {
      setError('Failed to update position size');
    } finally {
      setSavingPositionSize(false);
    }
  };

  const handleResetCircuitBreaker = async () => {
    try {
      await futuresApi.resetGinieCircuitBreaker();
      setSuccessMsg('Ginie circuit breaker reset');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchCircuitBreaker();
    } catch (err) {
      setError('Failed to reset Ginie circuit breaker');
    }
  };

  const handleToggleCircuitBreaker = async () => {
    if (!circuitBreaker) return;
    setTogglingCB(true);
    try {
      const result = await futuresApi.toggleGinieCircuitBreaker(!circuitBreaker.enabled);
      setSuccessMsg(result.message);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchCircuitBreaker();
    } catch (err) {
      setError('Failed to toggle circuit breaker');
    } finally {
      setTogglingCB(false);
    }
  };

  const handleSaveCBConfig = async () => {
    setSavingCB(true);
    try {
      const result = await futuresApi.updateGinieCircuitBreakerConfig(cbConfig);
      setSuccessMsg(result.message);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchCircuitBreaker();
      setShowCBConfig(false);
    } catch (err) {
      setError('Failed to update circuit breaker config');
    } finally {
      setSavingCB(false);
    }
  };

  const handleConfidenceChange = async (value: number) => {
    setConfidenceThreshold(value);
  };

  const handleSaveConfidence = async () => {
    setSavingConfig(true);
    try {
      await futuresApi.updateGinieAutopilotConfig({
        min_confidence_to_trade: confidenceThreshold,
      });
      setSuccessMsg(`Confidence threshold set to ${confidenceThreshold}%`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to update confidence threshold');
    } finally {
      setSavingConfig(false);
    }
  };

  const getModeColor = (mode: string) => {
    switch (mode) {
      case 'scalp': return 'text-yellow-400';
      case 'swing': return 'text-blue-400';
      case 'position': return 'text-purple-400';
      default: return 'text-gray-400';
    }
  };

  const getStatusBadge = (scanStatus: string) => {
    switch (scanStatus) {
      case 'SCALP-READY':
        return <span className="px-2 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-xs">SCALP</span>;
      case 'SWING-READY':
        return <span className="px-2 py-0.5 bg-blue-900/50 text-blue-400 rounded text-xs">SWING</span>;
      case 'POSITION-READY':
        return <span className="px-2 py-0.5 bg-purple-900/50 text-purple-400 rounded text-xs">POSITION</span>;
      case 'HEDGE-REQUIRED':
        return <span className="px-2 py-0.5 bg-orange-900/50 text-orange-400 rounded text-xs">HEDGE</span>;
      case 'AVOID':
        return <span className="px-2 py-0.5 bg-red-900/50 text-red-400 rounded text-xs">AVOID</span>;
      default:
        return <span className="px-2 py-0.5 bg-gray-700 text-gray-400 rounded text-xs">{scanStatus}</span>;
    }
  };

  const getRecommendationBadge = (rec: string) => {
    switch (rec) {
      case 'EXECUTE':
        return <span className="px-2 py-0.5 bg-green-900/50 text-green-400 rounded text-xs font-bold">EXECUTE</span>;
      case 'WAIT':
        return <span className="px-2 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-xs font-bold">WAIT</span>;
      case 'SKIP':
        return <span className="px-2 py-0.5 bg-red-900/50 text-red-400 rounded text-xs font-bold">SKIP</span>;
      default:
        return <span className="px-2 py-0.5 bg-gray-700 text-gray-400 rounded text-xs">{rec}</span>;
    }
  };

  if (!status) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 border border-gray-700">
        <div className="flex items-center gap-2 mb-3">
          <Sparkles className="w-5 h-5 text-purple-400" />
          <h3 className="text-lg font-semibold text-white">Ginie AI</h3>
        </div>
        <div className="text-gray-400 text-sm">Loading...</div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-3 border border-gray-700 h-full">
      {/* Header Row 1 - Title and Badges */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Sparkles className="w-4 h-4 text-purple-400" />
          <h3 className="text-sm font-semibold text-white">Ginie</h3>
        </div>
        <div className="flex items-center gap-1">
          <span className={`px-1.5 py-0.5 rounded text-[10px] uppercase font-bold ${getModeColor(status.active_mode)} bg-gray-700/50`}>
            {status.active_mode}
          </span>
          {isDryRun && (
            <span className="px-1 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-[10px] font-medium">PAPER</span>
          )}
          {isRunning && (
            <span className="px-1 py-0.5 bg-green-900/50 text-green-400 rounded text-[10px] font-medium animate-pulse">AUTO</span>
          )}
        </div>
      </div>
      {/* Header Row 2 - Buttons */}
      <div className="flex items-center justify-center gap-1 mb-2">
        {/* Scan & Analyze */}
        <button
          onClick={handleScanAll}
          disabled={scanning}
          className="flex items-center justify-center w-7 h-7 bg-blue-900/30 hover:bg-blue-900/50 rounded text-blue-400 disabled:opacity-50 transition-colors"
          title="Scan All Coins"
        >
          {scanning ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <BarChart3 className="w-3.5 h-3.5" />}
        </button>
        <button
          onClick={handleAnalyzeAll}
          disabled={analyzing}
          className="flex items-center justify-center w-7 h-7 bg-purple-900/30 hover:bg-purple-900/50 rounded text-purple-400 disabled:opacity-50 transition-colors"
          title="Analyze All Coins"
        >
          {analyzing ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Zap className="w-3.5 h-3.5" />}
        </button>
        <div className="w-px h-5 bg-gray-600 mx-0.5" />
        {/* Start/Stop & Enable/Disable */}
        <button
          onClick={isRunning ? handleStopAutopilot : handleStartAutopilot}
          disabled={loading}
          className={`flex items-center justify-center w-7 h-7 rounded transition-colors ${
            isRunning
              ? 'bg-red-900/30 hover:bg-red-900/50 text-red-400'
              : 'bg-green-900/30 hover:bg-green-900/50 text-green-400'
          }`}
          title={isRunning ? 'Stop Autopilot' : 'Start Autopilot'}
        >
          {isRunning ? <Square className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
        </button>
        <button
          onClick={handleToggle}
          disabled={loading}
          className={`flex items-center justify-center w-7 h-7 rounded transition-colors ${
            status.enabled
              ? 'bg-green-900/30 hover:bg-green-900/50 text-green-400'
              : 'bg-red-900/30 hover:bg-red-900/50 text-red-400'
          }`}
          title={status.enabled ? 'Disable Ginie' : 'Enable Ginie'}
        >
          {status.enabled ? <Power className="w-3.5 h-3.5" /> : <PowerOff className="w-3.5 h-3.5" />}
        </button>
        <div className="w-px h-5 bg-gray-600 mx-0.5" />
        {/* Paper/Live Toggle */}
        <button
          onClick={handleToggleMode}
          disabled={togglingMode}
          className={`flex items-center justify-center w-7 h-7 rounded transition-colors ${
            isDryRun
              ? 'bg-yellow-900/30 hover:bg-yellow-900/50 text-yellow-400'
              : 'bg-green-900/30 hover:bg-green-900/50 text-green-400'
          }`}
          title={isDryRun ? 'Switch to LIVE mode' : 'Switch to PAPER mode'}
        >
          {togglingMode ? (
            <RefreshCw className="w-3.5 h-3.5 animate-spin" />
          ) : isDryRun ? (
            <ToggleLeft className="w-3.5 h-3.5" />
          ) : (
            <ToggleRight className="w-3.5 h-3.5" />
          )}
        </button>
        {/* Sync Positions */}
        <button
          onClick={handleSyncPositions}
          disabled={syncing}
          className="flex items-center justify-center w-7 h-7 bg-blue-900/30 hover:bg-blue-900/50 rounded text-blue-400 disabled:opacity-50 transition-colors"
          title="Sync positions from exchange"
        >
          {syncing ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Download className="w-3.5 h-3.5" />}
        </button>
        {/* Clear Positions */}
        <button
          onClick={handleClearPositions}
          disabled={clearing}
          className="flex items-center justify-center w-7 h-7 bg-orange-900/30 hover:bg-orange-900/50 rounded text-orange-400 disabled:opacity-50 transition-colors"
          title="Clear Ginie Positions"
        >
          {clearing ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Trash2 className="w-3.5 h-3.5" />}
        </button>
        {/* Panic Exit */}
        <button
          onClick={handlePanicExit}
          disabled={panicking}
          className="flex items-center justify-center w-7 h-7 bg-red-900/50 hover:bg-red-700/50 rounded text-red-400 disabled:opacity-50 transition-colors animate-pulse"
          title="PANIC EXIT - Close All Positions"
        >
          {panicking ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <AlertOctagon className="w-3.5 h-3.5" />}
        </button>
      </div>

      {/* Circuit Breaker Status - Enhanced UI */}
      {circuitBreaker && (
        <div className={`rounded text-xs mb-3 border ${
          circuitBreaker.state === 'open'
            ? 'bg-red-900/30 border-red-800'
            : !circuitBreaker.can_trade
              ? 'bg-yellow-900/20 border-yellow-800'
              : circuitBreaker.enabled
                ? 'bg-gray-700/30 border-gray-600'
                : 'bg-gray-700/20 border-gray-700'
        }`}>
          {/* Header Row */}
          <div className="flex items-center justify-between px-2 py-1.5">
            <div className="flex items-center gap-1.5">
              <Shield className={`w-3.5 h-3.5 ${
                circuitBreaker.state === 'open' ? 'text-red-400' :
                !circuitBreaker.can_trade ? 'text-yellow-400' :
                circuitBreaker.enabled ? 'text-blue-400' : 'text-gray-500'
              }`} />
              <span className={`font-medium ${
                circuitBreaker.state === 'open' ? 'text-red-400' : 'text-gray-300'
              }`}>
                Ginie Circuit Breaker
              </span>
              <span className={`px-1 py-0.5 rounded text-[10px] uppercase font-bold ${
                circuitBreaker.state === 'open'
                  ? 'bg-red-900/50 text-red-400'
                  : !circuitBreaker.can_trade
                    ? 'bg-yellow-900/50 text-yellow-400'
                    : 'bg-green-900/50 text-green-400'
              }`}>
                {circuitBreaker.state === 'open' ? 'TRIPPED' : circuitBreaker.can_trade ? 'OK' : 'BLOCKED'}
              </span>
            </div>
            <div className="flex items-center gap-1">
              {/* Toggle Button */}
              <button
                onClick={handleToggleCircuitBreaker}
                disabled={togglingCB}
                className={`p-1 rounded transition-colors ${
                  circuitBreaker.enabled
                    ? 'bg-blue-900/50 hover:bg-blue-900/70 text-blue-400'
                    : 'bg-gray-700 hover:bg-gray-600 text-gray-400'
                }`}
                title={circuitBreaker.enabled ? 'Disable Circuit Breaker' : 'Enable Circuit Breaker'}
              >
                {togglingCB ? (
                  <RefreshCw className="w-3 h-3 animate-spin" />
                ) : circuitBreaker.enabled ? (
                  <ToggleRight className="w-3.5 h-3.5" />
                ) : (
                  <ToggleLeft className="w-3.5 h-3.5" />
                )}
              </button>
              {/* Config Button */}
              <button
                onClick={() => setShowCBConfig(!showCBConfig)}
                className="p-1 bg-gray-700 hover:bg-gray-600 text-gray-400 rounded transition-colors"
                title="Configure Circuit Breaker"
              >
                <Settings className="w-3 h-3" />
              </button>
              {/* Reset Button */}
              {(circuitBreaker.state === 'open' || !circuitBreaker.can_trade) && (
                <button
                  onClick={handleResetCircuitBreaker}
                  className="px-1.5 py-0.5 bg-blue-900/50 hover:bg-blue-900/70 text-blue-400 rounded text-[10px] transition-colors"
                >
                  Reset
                </button>
              )}
            </div>
          </div>

          {/* Block Reason */}
          {circuitBreaker.block_reason && (
            <div className="px-2 pb-1.5 text-[10px] text-yellow-400">
              ⚠ {circuitBreaker.block_reason}
            </div>
          )}

          {/* Stats Row */}
          {circuitBreaker.enabled && (
            <div className="flex items-center gap-3 px-2 pb-1.5 text-[10px] text-gray-400 border-t border-gray-700/50 pt-1.5">
              <span className="flex items-center gap-1">
                <Activity className="w-3 h-3" />
                Hourly: <span className={circuitBreaker.hourly_loss > 0 ? 'text-red-400' : 'text-gray-300'}>
                  -${Number(circuitBreaker.hourly_loss ?? 0).toFixed(0)}
                </span>
                /{circuitBreaker.max_loss_per_hour}
              </span>
              <span>
                Daily: <span className={circuitBreaker.daily_loss > 0 ? 'text-red-400' : 'text-gray-300'}>
                  -${Number(circuitBreaker.daily_loss ?? 0).toFixed(0)}
                </span>
                /{circuitBreaker.max_daily_loss}
              </span>
              <span>
                Losses: <span className={circuitBreaker.consecutive_losses > 0 ? 'text-yellow-400' : 'text-gray-300'}>
                  {circuitBreaker.consecutive_losses}
                </span>
                /{circuitBreaker.max_consecutive}
              </span>
            </div>
          )}

          {/* Config Panel */}
          {showCBConfig && (
            <div className="border-t border-gray-700/50 p-2 space-y-2">
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="text-[10px] text-gray-400">Max Loss/Hour ($)</label>
                  <input
                    type="number"
                    value={cbConfig.max_loss_per_hour}
                    onChange={(e) => setCBConfig({ ...cbConfig, max_loss_per_hour: Number(e.target.value) })}
                    onFocus={() => setIsEditingCBConfig(true)}
                    onBlur={() => setIsEditingCBConfig(false)}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
                <div>
                  <label className="text-[10px] text-gray-400">Max Daily Loss ($)</label>
                  <input
                    type="number"
                    value={cbConfig.max_daily_loss}
                    onChange={(e) => setCBConfig({ ...cbConfig, max_daily_loss: Number(e.target.value) })}
                    onFocus={() => setIsEditingCBConfig(true)}
                    onBlur={() => setIsEditingCBConfig(false)}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
                <div>
                  <label className="text-[10px] text-gray-400">Max Consecutive Losses</label>
                  <input
                    type="number"
                    value={cbConfig.max_consecutive_losses}
                    onChange={(e) => setCBConfig({ ...cbConfig, max_consecutive_losses: Number(e.target.value) })}
                    onFocus={() => setIsEditingCBConfig(true)}
                    onBlur={() => setIsEditingCBConfig(false)}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
                <div>
                  <label className="text-[10px] text-gray-400">Cooldown (minutes)</label>
                  <input
                    type="number"
                    value={cbConfig.cooldown_minutes}
                    onChange={(e) => setCBConfig({ ...cbConfig, cooldown_minutes: Number(e.target.value) })}
                    onFocus={() => setIsEditingCBConfig(true)}
                    onBlur={() => setIsEditingCBConfig(false)}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
              </div>
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setShowCBConfig(false)}
                  className="px-2 py-1 text-gray-400 hover:text-white text-xs"
                >
                  Cancel
                </button>
                <button
                  onClick={handleSaveCBConfig}
                  disabled={savingCB}
                  className="px-2 py-1 bg-blue-900/50 hover:bg-blue-900/70 text-blue-400 rounded text-xs disabled:opacity-50"
                >
                  {savingCB ? 'Saving...' : 'Save'}
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Risk Level Selector */}
      <div className="flex items-center gap-2 px-2 py-1.5 mb-3 bg-gray-700/30 rounded border border-gray-600">
        <Shield className="w-3.5 h-3.5 text-orange-400 flex-shrink-0" />
        <span className="text-xs text-gray-300 whitespace-nowrap">Risk:</span>
        <div className="flex-1 flex gap-1">
          {['conservative', 'moderate', 'aggressive'].map((level) => (
            <button
              key={level}
              onClick={() => handleSetRiskLevel(level)}
              disabled={savingRiskLevel}
              className={`flex-1 px-2 py-1 rounded text-[10px] font-medium transition-colors ${
                riskLevel === level
                  ? level === 'conservative'
                    ? 'bg-blue-900/50 text-blue-400 border border-blue-700'
                    : level === 'moderate'
                    ? 'bg-yellow-900/50 text-yellow-400 border border-yellow-700'
                    : 'bg-red-900/50 text-red-400 border border-red-700'
                  : 'bg-gray-700 text-gray-400 hover:bg-gray-600 border border-gray-600'
              } disabled:opacity-50`}
            >
              {savingRiskLevel && riskLevel === level ? '...' : level.charAt(0).toUpperCase() + level.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Confidence Threshold Setting */}
      <div className="flex items-center gap-2 px-2 py-1.5 mb-3 bg-gray-700/30 rounded border border-gray-600">
        <Target className="w-3.5 h-3.5 text-purple-400 flex-shrink-0" />
        <span className="text-xs text-gray-300 whitespace-nowrap">Confidence:</span>
        <input
          type="range"
          min="20"
          max="95"
          step="5"
          value={confidenceThreshold}
          onChange={(e) => handleConfidenceChange(Number(e.target.value))}
          className="flex-1 h-1.5 bg-gray-600 rounded-lg appearance-none cursor-pointer accent-purple-500"
        />
        <span className="text-xs font-bold text-purple-400 w-8 text-right">{confidenceThreshold}%</span>
        <button
          onClick={handleSaveConfidence}
          disabled={savingConfig || confidenceThreshold === (autopilotStatus?.config?.min_confidence_to_trade ?? 65)}
          className="px-1.5 py-0.5 bg-purple-900/50 hover:bg-purple-900/70 text-purple-400 rounded text-[10px] transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {savingConfig ? '...' : 'Save'}
        </button>
      </div>

      {/* Position Size Setting */}
      <div className="space-y-2 mb-3">
        {/* Main Size Controls */}
        <div className="flex items-center gap-2 px-2 py-1.5 bg-gray-700/30 rounded border border-gray-600">
          <BarChart3 className="w-3.5 h-3.5 text-cyan-400 flex-shrink-0" />
          <span className="text-xs text-gray-300 whitespace-nowrap">Size:</span>
          {!autoSizeMode ? (
            <>
              <span className="text-[10px] text-gray-500">$</span>
              <input
                type="number"
                min="50"
                max="5000"
                step="50"
                value={maxUsdPerPosition}
                onChange={(e) => setMaxUsdPerPosition(Number(e.target.value))}
                onFocus={() => setIsEditingPositionSize(true)}
                onBlur={() => setIsEditingPositionSize(false)}
                className="w-14 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs text-center"
              />
              <span className="text-[10px] text-gray-500">×</span>
              <input
                type="number"
                min="1"
                max="20"
                value={maxPositions}
                onChange={(e) => setMaxPositions(Number(e.target.value))}
                onFocus={() => setIsEditingPositionSize(true)}
                onBlur={() => setIsEditingPositionSize(false)}
                className="w-9 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs text-center"
              />
              <span className="text-[10px] text-gray-500">@</span>
              <input
                type="number"
                min="1"
                max="125"
                value={leverage}
                onChange={(e) => setLeverage(Number(e.target.value))}
                onFocus={() => setIsEditingPositionSize(true)}
                onBlur={() => setIsEditingPositionSize(false)}
                className="w-9 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs text-center"
              />
              <span className="text-[10px] text-gray-500">x</span>
              <span className="text-xs text-cyan-400 font-medium">${maxUsdPerPosition * maxPositions}</span>
            </>
          ) : (
            <span className="text-xs text-purple-400 font-medium ml-auto">AUTO MODE (By Category)</span>
          )}
          <button
            onClick={handleSavePositionSize}
            disabled={savingPositionSize}
            className="px-1.5 py-0.5 bg-cyan-900/50 hover:bg-cyan-900/70 text-cyan-400 rounded text-[10px] transition-colors disabled:opacity-50"
          >
            {savingPositionSize ? '...' : 'Save'}
          </button>
          <button
            onClick={() => setAutoSizeMode(!autoSizeMode)}
            className={`px-2 py-0.5 rounded text-[10px] transition-colors ${
              autoSizeMode
                ? 'bg-purple-900/50 text-purple-400 border border-purple-500/30'
                : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
            }`}
            title={autoSizeMode ? 'Disable auto sizing' : 'Enable auto sizing by category'}
          >
            {autoSizeMode ? 'ON' : 'AUTO'}
          </button>
        </div>

        {/* Category Multipliers (Auto Mode) */}
        {autoSizeMode && (
          <div className="px-2 py-2 bg-purple-900/20 border border-purple-700/30 rounded space-y-2">
            <div className="text-xs text-purple-400 font-medium mb-2">Position Size Multipliers by Category:</div>
            <div className="grid grid-cols-5 gap-2">
              {Object.entries(categoryMultipliers).map(([category, value]) => (
                <div key={category}>
                  <label className="text-[10px] text-gray-400 block mb-1 capitalize">{category}</label>
                  <div className="flex items-center gap-1">
                    <input
                      type="number"
                      min="0.1"
                      max="2"
                      step="0.1"
                      value={value}
                      onChange={(e) => setCategoryMultipliers({...categoryMultipliers, [category]: Number(e.target.value)})}
                      className="w-12 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs text-center"
                    />
                    <span className="text-[10px] text-gray-500">x</span>
                  </div>
                </div>
              ))}
            </div>
            <div className="text-[10px] text-gray-400 mt-2">
              💡 Best performers get larger positions (e.g. 1.5x), worst performers get smaller positions (e.g. 0.25x)
            </div>
          </div>
        )}
      </div>

      {/* Messages */}
      {error && (
        <div className="mb-3 p-2 bg-red-900/30 border border-red-800 rounded text-red-400 text-sm">
          {error}
        </div>
      )}
      {successMsg && (
        <div className="mb-3 p-2 bg-green-900/30 border border-green-800 rounded text-green-400 text-sm">
          {successMsg}
        </div>
      )}

      {/* Stats Grid */}
      <div className="grid grid-cols-4 gap-3 mb-4">
        <div className="bg-gray-700/50 rounded p-2 text-center">
          <div className="text-xs text-gray-400">Positions</div>
          <div className="text-lg font-bold text-white">
            {autopilotStatus?.stats?.active_positions ?? 0}/{autopilotStatus?.stats?.max_positions ?? status.max_positions}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2 text-center">
          <div className="text-xs text-gray-400">Available</div>
          <div className="text-lg font-bold text-blue-400">
            {formatUSD(autopilotStatus?.available_balance ?? 0)}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2 text-center">
          <div className="text-xs text-gray-400">Unrealized</div>
          <div className={`text-lg font-bold ${(autopilotStatus?.stats?.unrealized_pnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatUSD(autopilotStatus?.stats?.unrealized_pnl ?? 0)}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2 text-center">
          <div className="text-xs text-gray-400">Daily PnL</div>
          <div className={`text-lg font-bold ${(autopilotStatus?.stats?.daily_pnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatUSD(autopilotStatus?.stats?.daily_pnl ?? 0)}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2 text-center">
          <div className="text-xs text-gray-400">Total PnL</div>
          <div className={`text-lg font-bold ${(autopilotStatus?.stats?.total_pnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatUSD(autopilotStatus?.stats?.total_pnl ?? 0)}
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-2 border-b border-gray-700 pb-1">
        <button
          onClick={() => setActiveTab('decisions')}
          className={`px-2 py-0.5 rounded text-xs ${activeTab === 'decisions' ? 'bg-purple-900/50 text-purple-400' : 'text-gray-400 hover:text-white'}`}
        >
          Decisions
        </button>
        <button
          onClick={() => setActiveTab('positions')}
          className={`px-2 py-0.5 rounded text-xs ${activeTab === 'positions' ? 'bg-purple-900/50 text-purple-400' : 'text-gray-400 hover:text-white'}`}
        >
          Pos ({autopilotStatus?.positions?.length ?? 0})
        </button>
        <button
          onClick={() => setActiveTab('history')}
          className={`px-2 py-0.5 rounded text-xs ${activeTab === 'history' ? 'bg-purple-900/50 text-purple-400' : 'text-gray-400 hover:text-white'}`}
        >
          History
        </button>
        <button
          onClick={() => setActiveTab('movers')}
          className={`px-2 py-0.5 rounded text-xs ${activeTab === 'movers' ? 'bg-orange-900/50 text-orange-400' : 'text-gray-400 hover:text-white'}`}
        >
          Movers
        </button>
        <button
          onClick={() => setActiveTab('diagnostics')}
          className={`px-2 py-0.5 rounded text-xs flex items-center gap-0.5 ${activeTab === 'diagnostics' ? 'bg-cyan-900/50 text-cyan-400' : 'text-gray-400 hover:text-white'}`}
        >
          <Stethoscope className="w-3 h-3" />
          Diag
          {diagnostics?.issues && diagnostics.issues.filter(i => i.severity === 'critical').length > 0 && (
            <span className="w-1.5 h-1.5 bg-red-500 rounded-full animate-pulse" />
          )}
        </button>
        <button
          onClick={() => setActiveTab('performance')}
          className={`px-2 py-0.5 rounded text-xs flex items-center gap-0.5 ${activeTab === 'performance' ? 'bg-green-900/50 text-green-400' : 'text-gray-400 hover:text-white'}`}
        >
          <ListChecks className="w-3 h-3" />
          Perf
        </button>
      </div>

      {/* Positions Tab */}
      {activeTab === 'positions' && autopilotStatus?.positions && (
        <div className="space-y-2">
          {/* Source Filter */}
          <div className="flex items-center gap-1 mb-2">
            <span className="text-xs text-gray-400 mr-1">Source:</span>
            {(['all', 'ai', 'strategy'] as const).map((filter) => (
              <button
                key={filter}
                onClick={() => setSourceFilter(filter)}
                className={`px-2 py-0.5 rounded text-xs ${
                  sourceFilter === filter
                    ? filter === 'ai' ? 'bg-blue-600 text-white'
                      : filter === 'strategy' ? 'bg-purple-600 text-white'
                      : 'bg-gray-600 text-white'
                    : 'bg-gray-700 text-gray-400 hover:text-white'
                }`}
              >
                {filter === 'all' ? 'All' : filter === 'ai' ? 'AI' : 'Strategy'}
              </button>
            ))}
          </div>
          <div className="max-h-60 overflow-y-auto space-y-2">
            {autopilotStatus.positions
              .filter(pos => sourceFilter === 'all' || pos.source === sourceFilter)
              .length === 0 ? (
              <div className="text-center text-gray-500 py-4">
                No {sourceFilter === 'all' ? 'active' : sourceFilter} positions
              </div>
            ) : (
              autopilotStatus.positions
                .filter(pos => sourceFilter === 'all' || pos.source === sourceFilter)
                .map((pos) => (
                  <PositionCard
                    key={pos.symbol}
                    position={pos}
                    expanded={expandedPosition === pos.symbol}
                    onToggle={() => setExpandedPosition(expandedPosition === pos.symbol ? null : pos.symbol)}
                  />
                ))
            )}
          </div>
        </div>
      )}

      {/* History Tab */}
      {activeTab === 'history' && autopilotStatus?.trade_history && (
        <div className="space-y-2">
          {/* Source Filter for History */}
          <div className="flex items-center gap-1 mb-2">
            <span className="text-xs text-gray-400 mr-1">Source:</span>
            {(['all', 'ai', 'strategy'] as const).map((filter) => (
              <button
                key={filter}
                onClick={() => setSourceFilter(filter)}
                className={`px-2 py-0.5 rounded text-xs ${
                  sourceFilter === filter
                    ? filter === 'ai' ? 'bg-blue-600 text-white'
                      : filter === 'strategy' ? 'bg-purple-600 text-white'
                      : 'bg-gray-600 text-white'
                    : 'bg-gray-700 text-gray-400 hover:text-white'
                }`}
              >
                {filter === 'all' ? 'All' : filter === 'ai' ? 'AI' : 'Strategy'}
              </button>
            ))}
          </div>
          <div className="space-y-1 max-h-60 overflow-y-auto">
            {autopilotStatus.trade_history
              .filter(trade => sourceFilter === 'all' || trade.source === sourceFilter)
              .length === 0 ? (
              <div className="text-center text-gray-500 py-4">
                No {sourceFilter === 'all' ? '' : sourceFilter + ' '}trade history yet
              </div>
            ) : (
              autopilotStatus.trade_history
                .filter(trade => sourceFilter === 'all' || trade.source === sourceFilter)
                .slice().reverse().map((trade, idx) => (
                  <TradeHistoryRow key={`${trade.symbol}-${idx}`} trade={trade} />
                ))
            )}
          </div>
        </div>
      )}

      {/* Decisions Tab */}
      {activeTab === 'decisions' && (
        <>
          {/* Coin Scans (collapsible) */}
          {coinScans.length > 0 && (
            <div className="mb-4">
              <button
                onClick={() => setShowScans(!showScans)}
                className="flex items-center justify-between w-full text-left mb-2"
              >
                <span className="text-sm font-medium text-gray-300">Coin Scans ({coinScans.length})</span>
                {showScans ? <ChevronUp className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
              </button>
              {showScans && (
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {coinScans.map((scan) => (
                    <div
                      key={scan.symbol}
                      className="flex items-center justify-between p-2 bg-gray-700/30 rounded text-sm"
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-white font-medium">{scan.symbol.replace('USDT', '')}</span>
                        {getStatusBadge(scan.status)}
                      </div>
                      <div className="flex items-center gap-3 text-xs">
                        <span className="text-gray-400">
                          Score: <span className="text-white">{Number(scan.score || 0).toFixed(0)}</span>
                        </span>
                        <span className={scan.trade_ready ? 'text-green-400' : 'text-red-400'}>
                          {scan.trade_ready ? 'Ready' : 'Not Ready'}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Recent Decisions */}
          {status.recent_decisions && status.recent_decisions.length > 0 ? (
            <div>
              <div className="text-xs text-gray-400 mb-2">Recent Decisions</div>
              <div className="space-y-2 max-h-60 overflow-y-auto">
                {status.recent_decisions.slice(-5).reverse().map((decision, idx) => (
                  <div
                    key={`${decision.symbol}-${idx}`}
                    className="bg-gray-700/30 rounded p-2"
                  >
                    <div
                      className="flex items-center justify-between cursor-pointer"
                      onClick={() => setExpandedDecision(
                        expandedDecision === `${decision.symbol}-${idx}` ? null : `${decision.symbol}-${idx}`
                      )}
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-white font-medium">{decision.symbol.replace('USDT', '')}</span>
                        {getStatusBadge(decision.scan_status)}
                        {getRecommendationBadge(decision.recommendation)}
                      </div>
                      <div className="flex items-center gap-2">
                        <span className={`text-sm font-bold ${
                          decision.trade_execution.action === 'LONG' ? 'text-green-400' :
                          decision.trade_execution.action === 'SHORT' ? 'text-red-400' :
                          'text-gray-400'
                        }`}>
                          {decision.trade_execution.action}
                        </span>
                        <span className="text-xs text-gray-400">
                          {Number(decision.confidence_score || 0).toFixed(0)}%
                        </span>
                        {expandedDecision === `${decision.symbol}-${idx}` ?
                          <ChevronUp className="w-4 h-4 text-gray-400" /> :
                          <ChevronDown className="w-4 h-4 text-gray-400" />
                        }
                      </div>
                    </div>

                    {/* Expanded Decision Details */}
                    {expandedDecision === `${decision.symbol}-${idx}` && (
                      <div className="mt-3 pt-3 border-t border-gray-600 space-y-3 text-xs">
                        {/* Market Conditions */}
                        <div>
                          <div className="text-gray-400 mb-1">Market Conditions</div>
                          <div className="grid grid-cols-4 gap-2">
                            <div className="bg-gray-700/50 p-1.5 rounded">
                              <div className="text-gray-500">Trend</div>
                              <div className={`capitalize ${
                                decision.market_conditions.trend === 'bullish' ? 'text-green-400' :
                                decision.market_conditions.trend === 'bearish' ? 'text-red-400' :
                                'text-gray-400'
                              }`}>{decision.market_conditions.trend}</div>
                            </div>
                            <div className="bg-gray-700/50 p-1.5 rounded">
                              <div className="text-gray-500">ADX</div>
                              <div className="text-white">{Number(decision.market_conditions?.adx || 0).toFixed(1)}</div>
                            </div>
                            <div className="bg-gray-700/50 p-1.5 rounded">
                              <div className="text-gray-500">Volatility</div>
                              <div className="text-white">{decision.market_conditions.volatility}</div>
                            </div>
                            <div className="bg-gray-700/50 p-1.5 rounded">
                              <div className="text-gray-500">Volume</div>
                              <div className="text-white">{decision.market_conditions.volume}</div>
                            </div>
                          </div>
                        </div>

                        {/* Signal Analysis */}
                        <div>
                          <div className="text-gray-400 mb-1">
                            Signals ({decision.signal_analysis.primary_met}/{decision.signal_analysis.primary_required})
                            <span className={`ml-2 ${decision.signal_analysis.primary_passed ? 'text-green-400' : 'text-red-400'}`}>
                              {decision.signal_analysis.signal_strength}
                            </span>
                          </div>
                          <div className="flex flex-wrap gap-1">
                            {decision.signal_analysis.primary_signals.map((sig, sigIdx) => (
                              <span
                                key={sigIdx}
                                className={`px-1.5 py-0.5 rounded ${
                                  sig.met ? 'bg-green-900/30 text-green-400' : 'bg-gray-700 text-gray-500'
                                }`}
                                title={sig.description}
                              >
                                {sig.met ? <CheckCircle className="w-3 h-3 inline mr-1" /> : <XCircle className="w-3 h-3 inline mr-1" />}
                                {sig.name}
                              </span>
                            ))}
                          </div>
                        </div>

                        {/* Trade Execution */}
                        {decision.trade_execution.action !== 'WAIT' && (
                          <div>
                            <div className="text-gray-400 mb-1">Trade Setup</div>
                            <div className="grid grid-cols-4 gap-2">
                              <div className="bg-gray-700/50 p-1.5 rounded">
                                <div className="text-gray-500">Entry</div>
                                <div className="text-white text-[10px]">
                                  ${Number(decision.trade_execution?.entry_low || 0).toFixed(2)} - ${Number(decision.trade_execution?.entry_high || 0).toFixed(2)}
                                </div>
                              </div>
                              <div className="bg-gray-700/50 p-1.5 rounded">
                                <div className="text-gray-500">SL</div>
                                <div className="text-red-400">
                                  {Number(decision.trade_execution?.stop_loss_pct || 0).toFixed(2)}%
                                </div>
                              </div>
                              <div className="bg-gray-700/50 p-1.5 rounded">
                                <div className="text-gray-500">R:R</div>
                                <div className="text-white">
                                  {Number(decision.trade_execution?.risk_reward || 0).toFixed(2)}
                                </div>
                              </div>
                              <div className="bg-gray-700/50 p-1.5 rounded">
                                <div className="text-gray-500">Leverage</div>
                                <div className="text-white">
                                  {decision.trade_execution.leverage}x
                                </div>
                              </div>
                            </div>
                            {/* Take Profit Levels */}
                            {decision.trade_execution.take_profits.length > 0 && (
                              <div className="mt-2 flex gap-2 flex-wrap">
                                {decision.trade_execution.take_profits.map((tp) => (
                                  <span
                                    key={tp.level}
                                    className="px-2 py-0.5 bg-green-900/30 text-green-400 rounded"
                                  >
                                    TP{tp.level}: +{tp.gain_pct}% ({tp.percent}%)
                                  </span>
                                ))}
                              </div>
                            )}
                          </div>
                        )}

                        {/* Hedge Recommendation */}
                        {decision.hedge.required && (
                          <div className="flex items-center gap-2 p-2 bg-orange-900/20 border border-orange-800/50 rounded">
                            <Shield className="w-4 h-4 text-orange-400" />
                            <span className="text-orange-400">
                              Hedge Recommended: {decision.hedge.hedge_type} ({decision.hedge.hedge_size}%)
                            </span>
                          </div>
                        )}

                        {/* Recommendation Note */}
                        <div className="text-gray-400 italic">
                          "{decision.recommendation_note}"
                        </div>

                        {/* Next Review */}
                        <div className="flex items-center gap-2 text-gray-500">
                          <Clock className="w-3 h-3" />
                          Next review: {decision.next_review}
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-center text-gray-500 py-4">
              <Sparkles className="w-8 h-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">No decisions yet. Click Analyze to scan coins.</p>
            </div>
          )}
        </>
      )}

      {/* Market Movers Tab */}
      {activeTab === 'movers' && (
        <div className="space-y-3">
          {/* Action Buttons */}
          <div className="flex items-center gap-2 mb-3">
            <button
              onClick={handleFetchMarketMovers}
              disabled={loadingMovers}
              className="flex items-center gap-1 px-3 py-1.5 bg-orange-900/30 hover:bg-orange-900/50 rounded text-orange-400 text-sm disabled:opacity-50 transition-colors"
            >
              {loadingMovers ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
              Refresh
            </button>
            <button
              onClick={handleRefreshDynamicSymbols}
              disabled={refreshingSymbols}
              className="flex items-center gap-1 px-3 py-1.5 bg-blue-900/30 hover:bg-blue-900/50 rounded text-blue-400 text-sm disabled:opacity-50 transition-colors"
            >
              {refreshingSymbols ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Zap className="w-3.5 h-3.5" />}
              Apply to Watch List
            </button>
          </div>

          {marketMovers ? (
            <div className="grid grid-cols-2 gap-3">
              {/* Top Gainers */}
              <div className="bg-gray-700/30 rounded p-2">
                <div className="flex items-center gap-1.5 mb-2 text-green-400">
                  <TrendingUp className="w-4 h-4" />
                  <span className="text-xs font-medium">Top Gainers</span>
                </div>
                <div className="flex flex-wrap gap-1">
                  {marketMovers.top_gainers.slice(0, 8).map((symbol) => (
                    <span key={symbol} className="px-1.5 py-0.5 bg-green-900/30 text-green-400 rounded text-[10px]">
                      {symbol.replace('USDT', '')}
                    </span>
                  ))}
                </div>
              </div>

              {/* Top Losers */}
              <div className="bg-gray-700/30 rounded p-2">
                <div className="flex items-center gap-1.5 mb-2 text-red-400">
                  <TrendingDown className="w-4 h-4" />
                  <span className="text-xs font-medium">Top Losers</span>
                </div>
                <div className="flex flex-wrap gap-1">
                  {marketMovers.top_losers.slice(0, 8).map((symbol) => (
                    <span key={symbol} className="px-1.5 py-0.5 bg-red-900/30 text-red-400 rounded text-[10px]">
                      {symbol.replace('USDT', '')}
                    </span>
                  ))}
                </div>
              </div>

              {/* Top Volume */}
              <div className="bg-gray-700/30 rounded p-2">
                <div className="flex items-center gap-1.5 mb-2 text-blue-400">
                  <BarChart2 className="w-4 h-4" />
                  <span className="text-xs font-medium">Top Volume</span>
                </div>
                <div className="flex flex-wrap gap-1">
                  {marketMovers.top_volume.slice(0, 8).map((symbol) => (
                    <span key={symbol} className="px-1.5 py-0.5 bg-blue-900/30 text-blue-400 rounded text-[10px]">
                      {symbol.replace('USDT', '')}
                    </span>
                  ))}
                </div>
              </div>

              {/* High Volatility */}
              <div className="bg-gray-700/30 rounded p-2">
                <div className="flex items-center gap-1.5 mb-2 text-orange-400">
                  <Flame className="w-4 h-4" />
                  <span className="text-xs font-medium">High Volatility</span>
                </div>
                <div className="flex flex-wrap gap-1">
                  {marketMovers.high_volatility.slice(0, 8).map((symbol) => (
                    <span key={symbol} className="px-1.5 py-0.5 bg-orange-900/30 text-orange-400 rounded text-[10px]">
                      {symbol.replace('USDT', '')}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <div className="text-center text-gray-500 py-4">
              <BarChart2 className="w-8 h-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">Loading market movers...</p>
            </div>
          )}
        </div>
      )}

      {/* Diagnostics Tab */}
      {activeTab === 'diagnostics' && (
        <div className="space-y-2 max-h-60 overflow-y-auto">
          {/* Quick Status Cards */}
          <div className="grid grid-cols-4 gap-2">
            <div className={`p-2 rounded text-center ${diagnostics?.can_trade ? 'bg-green-900/30 border border-green-800' : 'bg-red-900/30 border border-red-800'}`}>
              <div className="text-[10px] text-gray-400">Can Trade</div>
              <div className={`text-sm font-bold ${diagnostics?.can_trade ? 'text-green-400' : 'text-red-400'}`}>
                {diagnostics?.can_trade ? 'YES' : 'NO'}
              </div>
            </div>
            <div className="bg-gray-700/50 p-2 rounded text-center">
              <div className="text-[10px] text-gray-400">Positions</div>
              <div className="text-sm font-bold text-white">
                {diagnostics?.positions?.open_count ?? 0}/{diagnostics?.positions?.max_allowed ?? 0}
              </div>
            </div>
            <div className={`p-2 rounded text-center ${diagnostics?.circuit_breaker?.state === 'open' ? 'bg-red-900/30 border border-red-800' : 'bg-gray-700/50'}`}>
              <div className="text-[10px] text-gray-400">Circuit</div>
              <div className={`text-sm font-bold ${diagnostics?.circuit_breaker?.state === 'open' ? 'text-red-400' : 'text-green-400'}`}>
                {diagnostics?.circuit_breaker?.state?.toUpperCase() || 'OK'}
              </div>
            </div>
            <div className="bg-gray-700/50 p-2 rounded text-center">
              <div className="text-[10px] text-gray-400">Signals</div>
              <div className="text-sm font-bold text-cyan-400">
                {signalStats?.executed ?? 0}/{signalStats?.total ?? 0}
              </div>
            </div>
          </div>

          {/* Can't Trade Reason */}
          {!diagnostics?.can_trade && diagnostics?.can_trade_reason && (
            <div className="p-2 bg-red-900/20 border border-red-800/50 rounded text-xs text-red-400 flex items-center gap-2">
              <AlertOctagon className="w-4 h-4 flex-shrink-0" />
              <span>{diagnostics.can_trade_reason}</span>
            </div>
          )}

          {/* Issues Summary */}
          {diagnostics?.issues && diagnostics.issues.length > 0 && (
            <div className="space-y-1.5">
              <div className="text-xs text-gray-400 flex items-center gap-1">
                <AlertTriangle className="w-3.5 h-3.5" />
                Issues ({diagnostics.issues.length})
              </div>
              <div className="space-y-1 max-h-20 overflow-y-auto">
                {diagnostics.issues.map((issue, idx) => (
                  <div
                    key={idx}
                    className={`p-2 rounded text-xs flex items-start gap-2 ${
                      issue.severity === 'critical' ? 'bg-red-900/20 border border-red-800/50' :
                      issue.severity === 'warning' ? 'bg-yellow-900/20 border border-yellow-800/50' :
                      'bg-blue-900/20 border border-blue-800/50'
                    }`}
                  >
                    {issue.severity === 'critical' ? <AlertOctagon className="w-3.5 h-3.5 text-red-400 flex-shrink-0 mt-0.5" /> :
                     issue.severity === 'warning' ? <AlertTriangle className="w-3.5 h-3.5 text-yellow-400 flex-shrink-0 mt-0.5" /> :
                     <Info className="w-3.5 h-3.5 text-blue-400 flex-shrink-0 mt-0.5" />}
                    <div className="flex-1">
                      <p className={`${
                        issue.severity === 'critical' ? 'text-red-400' :
                        issue.severity === 'warning' ? 'text-yellow-400' : 'text-blue-400'
                      }`}>{issue.message}</p>
                      <p className="text-gray-500 mt-0.5">{issue.suggestion}</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Scanning & LLM Status */}
          <div className="grid grid-cols-2 gap-2">
            <div className="bg-gray-700/30 rounded p-2">
              <div className="flex items-center gap-1 mb-1 text-xs text-gray-400">
                <Eye className="w-3.5 h-3.5" /> Scanning
              </div>
              <div className="text-[10px] space-y-0.5">
                <div className="flex justify-between">
                  <span className="text-gray-500">Last Scan</span>
                  <span className="text-white">
                    {diagnostics?.scanning?.last_scan_time && diagnostics.scanning.last_scan_time !== '0001-01-01T00:00:00Z'
                      ? new Date(diagnostics.scanning.last_scan_time).toLocaleTimeString()
                      : 'Never'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Watchlist</span>
                  <span className="text-white">{diagnostics?.scanning?.symbols_in_watchlist ?? 0} symbols</span>
                </div>
                <div className="flex gap-1 mt-1">
                  {diagnostics?.scanning?.scalp_enabled && <span className="px-1 bg-yellow-900/30 text-yellow-400 rounded text-[9px]">SCALP</span>}
                  {diagnostics?.scanning?.swing_enabled && <span className="px-1 bg-blue-900/30 text-blue-400 rounded text-[9px]">SWING</span>}
                  {diagnostics?.scanning?.position_enabled && <span className="px-1 bg-purple-900/30 text-purple-400 rounded text-[9px]">POS</span>}
                </div>
              </div>
            </div>
            <div className="bg-gray-700/30 rounded p-2">
              <div className="flex items-center gap-1 mb-1 text-xs text-gray-400">
                <Radio className="w-3.5 h-3.5" /> LLM
              </div>
              <div className="text-[10px] space-y-0.5">
                <div className="flex justify-between items-center">
                  <span className="text-gray-500">Status</span>
                  {diagnostics?.llm_status?.connected ? (
                    <span className="flex items-center gap-1 text-green-400">
                      <CheckCircle className="w-3 h-3" /> Connected
                    </span>
                  ) : (
                    <span className="flex items-center gap-1 text-red-400">
                      <XCircle className="w-3 h-3" /> Disconnected
                    </span>
                  )}
                </div>
                <div className="flex justify-between">
                  <span className="text-gray-500">Provider</span>
                  <span className="text-white">{diagnostics?.llm_status?.provider || 'N/A'}</span>
                </div>
              </div>
            </div>
          </div>

          {/* Signal Logs Section */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <div className="text-xs text-gray-400 flex items-center gap-1">
                <Activity className="w-3.5 h-3.5" /> Signal Logs
              </div>
              <div className="flex gap-1">
                {(['all', 'executed', 'rejected'] as const).map((filter) => (
                  <button
                    key={filter}
                    onClick={() => setSignalFilter(filter)}
                    className={`px-2 py-0.5 text-[10px] rounded transition-colors ${
                      signalFilter === filter
                        ? filter === 'executed' ? 'bg-green-900/50 text-green-400'
                          : filter === 'rejected' ? 'bg-red-900/50 text-red-400'
                          : 'bg-cyan-900/50 text-cyan-400'
                        : 'bg-gray-700 text-gray-400 hover:text-white'
                    }`}
                  >
                    {filter.charAt(0).toUpperCase() + filter.slice(1)}
                  </button>
                ))}
              </div>
            </div>

            {/* Execution Rate Bar */}
            {signalStats && signalStats.total > 0 && (
              <div className="flex items-center gap-2 mb-2 text-[10px]">
                <span className="text-gray-400">Rate:</span>
                <div className="flex-1 bg-gray-600 rounded-full h-1.5">
                  <div className="bg-green-500 h-1.5 rounded-full" style={{ width: `${signalStats.execution_rate}%` }} />
                </div>
                <span className="text-white font-medium">{signalStats.execution_rate.toFixed(0)}%</span>
              </div>
            )}

            {/* Signal List */}
            <div className="space-y-1 max-h-24 overflow-y-auto">
              {signalLogs.length === 0 ? (
                <p className="text-center text-gray-500 py-4 text-xs">No signals yet</p>
              ) : (
                signalLogs.slice(0, 20).map((signal, idx) => (
                  <div key={signal.id || idx} className="bg-gray-700/30 rounded p-1.5">
                    <div
                      className="flex items-center justify-between cursor-pointer"
                      onClick={() => setExpandedSignal(expandedSignal === (signal.id || `${idx}`) ? null : (signal.id || `${idx}`))}
                    >
                      <div className="flex items-center gap-1.5">
                        <span className={`px-1 py-0.5 rounded text-[9px] font-medium ${
                          signal.status === 'executed' ? 'bg-green-900/50 text-green-400' :
                          signal.status === 'rejected' ? 'bg-red-900/50 text-red-400' :
                          'bg-yellow-900/50 text-yellow-400'
                        }`}>
                          {signal.status.toUpperCase()}
                        </span>
                        <span className="text-white text-xs font-medium">{signal.symbol?.replace('USDT', '')}</span>
                        <span className={`text-[10px] ${signal.direction === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
                          {signal.direction}
                        </span>
                      </div>
                      <div className="flex items-center gap-2 text-[10px]">
                        <span className="text-gray-400">{signal.confidence?.toFixed(0)}%</span>
                        {expandedSignal === (signal.id || `${idx}`) ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                      </div>
                    </div>

                    {expandedSignal === (signal.id || `${idx}`) && (
                      <div className="mt-2 pt-2 border-t border-gray-600 grid grid-cols-3 gap-2 text-[10px]">
                        <div>
                          <span className="text-gray-500">Entry:</span>
                          <span className="text-white ml-1">${signal.entry_price?.toFixed(4)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">SL:</span>
                          <span className="text-red-400 ml-1">${signal.stop_loss?.toFixed(4)}</span>
                        </div>
                        <div>
                          <span className="text-gray-500">R:R:</span>
                          <span className="text-white ml-1">{signal.risk_reward?.toFixed(2)}</span>
                        </div>
                        {signal.rejection_reason && (
                          <div className="col-span-3 text-red-400">
                            Reason: {signal.rejection_reason}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      )}

      {/* Performance Tab */}
      {activeTab === 'performance' && (
        <div className="max-h-96 overflow-y-auto">
          <SymbolPerformancePanel />
        </div>
      )}
    </div>
  );
}

// Position Card Component
function PositionCard({ position, expanded, onToggle }: { position: GiniePosition; expanded: boolean; onToggle: () => void }) {
  const pnlTotal = position.realized_pnl + position.unrealized_pnl;
  const pnlPercent = ((position.remaining_qty > 0 ? position.unrealized_pnl : 0) / (position.entry_price * position.original_qty)) * 100;

  return (
    <div className="bg-gray-700/30 rounded p-2">
      <div className="flex items-center justify-between cursor-pointer" onClick={onToggle}>
        <div className="flex items-center gap-2">
          <span className="text-white font-medium">{position.symbol.replace('USDT', '')}</span>
          <span className={`text-xs font-bold ${position.side === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
            {position.side}
          </span>
          <span className={`text-xs uppercase ${
            position.mode === 'scalp' ? 'text-yellow-400' :
            position.mode === 'swing' ? 'text-blue-400' :
            'text-purple-400'
          }`}>{position.mode}</span>
          {/* Source Badge */}
          <span className={`px-1 py-0.5 rounded text-xs ${
            position.source === 'strategy' ? 'bg-purple-900/50 text-purple-400' : 'bg-blue-900/50 text-blue-400'
          }`}>
            {position.source === 'strategy' ? position.strategy_name || 'Strategy' : 'AI'}
          </span>
          {position.trailing_active && (
            <span className="px-1 py-0.5 bg-blue-900/50 text-blue-400 rounded text-xs">TRAIL</span>
          )}
        </div>
        <div className="flex items-center gap-3">
          <span className={`font-bold ${pnlTotal >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatUSD(pnlTotal)} ({Number(pnlPercent || 0).toFixed(2)}%)
          </span>
          {expanded ? <ChevronUp className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
        </div>
      </div>

      {expanded && (
        <div className="mt-3 pt-3 border-t border-gray-600 space-y-2 text-xs">
          <div className="grid grid-cols-4 gap-2">
            <div className="bg-gray-700/50 p-1.5 rounded">
              <div className="text-gray-500">Entry</div>
              <div className="text-white">${Number(position.entry_price || 0).toFixed(2)}</div>
            </div>
            <div className="bg-gray-700/50 p-1.5 rounded">
              <div className="text-gray-500">Qty</div>
              <div className="text-white">{Number(position.remaining_qty || 0).toFixed(4)} / {Number(position.original_qty || 0).toFixed(4)}</div>
            </div>
            <div className="bg-gray-700/50 p-1.5 rounded">
              <div className="text-gray-500">SL</div>
              <div className={position.moved_to_breakeven ? 'text-blue-400' : 'text-red-400'}>
                ${Number(position.stop_loss || 0).toFixed(2)}
                {position.moved_to_breakeven && ' (BE)'}
              </div>
            </div>
            <div className="bg-gray-700/50 p-1.5 rounded">
              <div className="text-gray-500">Leverage</div>
              <div className="text-white">{position.leverage}x</div>
            </div>
          </div>

          {/* TP Levels */}
          <div className="flex gap-2 flex-wrap">
            {position.take_profits.map((tp) => (
              <div
                key={tp.level}
                className={`px-2 py-1 rounded flex items-center gap-1 ${
                  tp.status === 'hit' ? 'bg-green-900/50 text-green-400' : 'bg-gray-700 text-gray-400'
                }`}
              >
                <Target className="w-3 h-3" />
                TP{tp.level}: ${Number(tp.price || 0).toFixed(2)} ({tp.percent}%)
                {tp.status === 'hit' && <CheckCircle className="w-3 h-3 ml-1" />}
              </div>
            ))}
          </div>

          <div className="text-gray-500">
            Realized: <span className={position.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
              {formatUSD(position.realized_pnl)}
            </span>
          </div>
        </div>
      )}
    </div>
  );
}

// Trade History Row Component
function TradeHistoryRow({ trade }: { trade: GinieTradeResult }) {
  const time = new Date(trade.timestamp).toLocaleTimeString();

  return (
    <div className="flex items-center justify-between p-2 bg-gray-700/30 rounded text-xs">
      <div className="flex items-center gap-2">
        <span className="text-gray-400">{time}</span>
        <span className="text-white font-medium">{trade.symbol.replace('USDT', '')}</span>
        <span className={`font-bold ${trade.side === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
          {trade.side}
        </span>
        <span className="text-gray-400">{trade.action}</span>
        {trade.source && (
          <span className={`px-1 py-0.5 rounded ${
            trade.source === 'strategy' ? 'bg-purple-900/30 text-purple-400' : 'bg-blue-900/30 text-blue-400'
          }`}>
            {trade.source === 'strategy' ? trade.strategy_name || 'Strategy' : 'AI'}
          </span>
        )}
        {trade.tp_level && trade.tp_level > 0 && (
          <span className="px-1 py-0.5 bg-green-900/30 text-green-400 rounded">TP{trade.tp_level}</span>
        )}
      </div>
      <div className="flex items-center gap-2">
        <span className="text-gray-400">{Number(trade.quantity || 0).toFixed(4)}</span>
        <span className={`font-bold ${(trade.pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
          {formatUSD(trade.pnl || 0)}
        </span>
      </div>
    </div>
  );
}
