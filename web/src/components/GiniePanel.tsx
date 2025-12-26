import { useEffect, useState } from 'react';
import { futuresApi, formatUSD, GinieStatus, GinieCoinScan, GinieAutopilotStatus, GiniePosition, GinieTradeResult, GinieCircuitBreakerStatus, MarketMoversResponse, GinieDiagnostics, GinieSignalLog, GinieSignalStats, ModeFullConfig, LLMConfig, ModeLLMSettings, AdaptiveAIConfig, AdaptiveRecommendation, ModeStatistics, LLMCallDiagnostics } from '../services/futuresApi';
import {
  Sparkles, Power, PowerOff, RefreshCw, Shield, CheckCircle, XCircle,
  ChevronDown, ChevronUp, Zap, Clock, BarChart3, Play, Square, Target,
  Trash2, AlertOctagon, ToggleLeft, ToggleRight, Settings, Activity, Download,
  TrendingUp, TrendingDown, BarChart2, Flame, Stethoscope, AlertTriangle, Info, Eye, Radio,
  ListChecks, AlertCircle, Brain, Lightbulb, Check, X, Gauge
} from 'lucide-react';
import SymbolPerformancePanel from './SymbolPerformancePanel';

export default function GiniePanel() {
  const [status, setStatus] = useState<GinieStatus | null>(null);
  const [autopilotStatus, setAutopilotStatus] = useState<GinieAutopilotStatus | null>(null);
  const [circuitBreaker, setCircuitBreaker] = useState<GinieCircuitBreakerStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [togglingAutopilot, setTogglingAutopilot] = useState(false);
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
  // Trend Timeframes state
  const [trendTimeframes, setTrendTimeframes] = useState({
    ultra_fast: '5m',
    scalp: '15m',
    swing: '1h',
    position: '4h',
    block_on_divergence: true,
  });
  const [savingTimeframes, setSavingTimeframes] = useState(false);
  const [editingTimeframes, setEditingTimeframes] = useState(false);
  // SL/TP Configuration state
  const [sltpConfig, setSltpConfig] = useState({
    ultra_fast: { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 0.1, trailing_activation: 0.2 },
    scalp: { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 0.3, trailing_activation: 0.5 },
    swing: { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 1.5, trailing_activation: 1.0 },
    position: { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 3.0, trailing_activation: 2.0 },
  });
  const [tpMode, setTpMode] = useState({
    use_single_tp: true,
    single_tp_percent: 5.0,
    tp1_percent: 25.0,
    tp2_percent: 25.0,
    tp3_percent: 25.0,
    tp4_percent: 25.0,
  });
  const [savingSLTP, setSavingSLTP] = useState(false);
  const [editingSLTP, setEditingSLTP] = useState(false);
  const [selectedMode, setSelectedMode] = useState<'ultra_fast' | 'scalp' | 'swing' | 'position'>('swing');
  // Trade history with full decision details
  const [tradeHistory, setTradeHistory] = useState<any[]>([]);
  const [expandedTrade, setExpandedTrade] = useState<string | null>(null);
  const [selectedDateRange, setSelectedDateRange] = useState({ start: '', end: '' });
  // LLM diagnostics tracking
  const [llmSwitches, setLlmSwitches] = useState<any[]>([]);
  // Performance metrics with live data
  const [performanceMetrics, setPerformanceMetrics] = useState<any>(null);
  const [loadingPerformance, setLoadingPerformance] = useState(false);
  // Mode Configuration state (Story 2.7 Task 2.7.9)
  const [modeConfigs, setModeConfigs] = useState<Record<string, ModeFullConfig>>({});
  const [showModeConfig, setShowModeConfig] = useState(false);
  const [selectedModeConfig, setSelectedModeConfig] = useState<'ultra_fast' | 'scalp' | 'swing' | 'position'>('ultra_fast');
  const [expandedModeSection, setExpandedModeSection] = useState<string | null>(null);
  const [savingModeConfig, setSavingModeConfig] = useState(false);
  const [resettingModes, setResettingModes] = useState(false);
  const [modeConfigErrors, setModeConfigErrors] = useState<Record<string, string>>({});

  // LLM & Adaptive AI state (Story 2.8)
  const [llmConfig, setLlmConfig] = useState<LLMConfig | null>(null);
  const [modeLLMSettings, setModeLLMSettings] = useState<Record<string, ModeLLMSettings>>({});
  const [adaptiveConfig, setAdaptiveConfig] = useState<AdaptiveAIConfig | null>(null);
  const [recommendations, setRecommendations] = useState<AdaptiveRecommendation[]>([]);
  const [modeStatistics, setModeStatistics] = useState<Record<string, ModeStatistics>>({});
  const [llmCallDiagnostics, setLlmCallDiagnostics] = useState<LLMCallDiagnostics | null>(null);
  const [showLLMSettings, setShowLLMSettings] = useState(false);
  const [savingLLMConfig, setSavingLLMConfig] = useState(false);
  const [applyingRecommendation, setApplyingRecommendation] = useState<string | null>(null);
  const [selectedLLMMode, setSelectedLLMMode] = useState<'ultra_fast' | 'scalp' | 'swing' | 'position'>('swing');

  const validTimeframes = ['1m', '3m', '5m', '15m', '30m', '1h', '2h', '4h', '6h', '8h', '12h', '1d', '3d', '1w', '1M'];
  const timeframeOptions = ['1m', '5m', '15m', '1h', '4h', '1d'];

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
    fetchTrendTimeframes(); // Fetch trend timeframe configuration
    fetchSLTPConfig(); // Fetch SL/TP configuration
    fetchTradeHistory(); // Fetch trade history
    fetchPerformanceMetrics(); // Fetch performance metrics
    fetchLLMSwitches(); // Fetch LLM diagnostics
    fetchModeConfigs(); // Fetch mode configurations (Story 2.7)
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

  const handleToggleFuturesAutopilot = async () => {
    if (!autopilotStatus) return;
    setTogglingAutopilot(true);
    try {
      const newRunning = !autopilotStatus.stats.running;
      const result = await futuresApi.toggleAutopilot(newRunning);
      setSuccessMsg(result.message || `Futures Autopilot ${newRunning ? 'enabled' : 'disabled'}`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAutopilotStatus();
    } catch (err) {
      setError('Failed to toggle Futures Autopilot');
    } finally {
      setTogglingAutopilot(false);
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

  const fetchTrendTimeframes = async () => {
    try {
      const result = await futuresApi.getGinieTrendTimeframes();
      if (result.success && result.timeframes) {
        // Merge API response with defaults to ensure all modes are present
        const apiData = result.timeframes as any;
        setTrendTimeframes({
          ...trendTimeframes,
          ultra_fast: apiData?.ultrafast || '5m',
          scalp: apiData?.scalp || '15m',
          swing: apiData?.swing || '1h',
          position: apiData?.position || '4h',
          block_on_divergence: apiData?.block_on_divergence !== undefined ? apiData.block_on_divergence : true,
        });
      }
    } catch (err) {
      console.error('Failed to fetch trend timeframes:', err);
    }
  };

  const handleSaveTimeframes = async () => {
    setSavingTimeframes(true);
    try {
      const result = await futuresApi.updateGinieTrendTimeframes({
        scalp_timeframe: trendTimeframes.scalp,
        swing_timeframe: trendTimeframes.swing,
        position_timeframe: trendTimeframes.position,
        ultrafast_timeframe: trendTimeframes.ultra_fast,
        block_on_divergence: trendTimeframes.block_on_divergence,
      });
      if (result.success) {
        setSuccessMsg('Trend timeframes updated successfully');
        setEditingTimeframes(false);
        setTimeout(() => setSuccessMsg(null), 3000);
      }
    } catch (err) {
      setError('Failed to update trend timeframes');
    } finally {
      setSavingTimeframes(false);
    }
  };

  const fetchSLTPConfig = async () => {
    try {
      const result = await futuresApi.getGinieSLTPConfig();
      if (result.success && result.sltp_config) {
        // Merge with defaults to ensure all modes are present
        const apiConfig = result.sltp_config as any;
        const mergedConfig = {
          ultra_fast: apiConfig?.ultrafast || { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 0.1, trailing_activation: 0.2 },
          scalp: apiConfig?.scalp || { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 0.3, trailing_activation: 0.5 },
          swing: apiConfig?.swing || { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 1.5, trailing_activation: 1.0 },
          position: apiConfig?.position || { sl_percent: 0, tp_percent: 0, trailing_enabled: true, trailing_percent: 3.0, trailing_activation: 2.0 },
        };
        setSltpConfig(mergedConfig);
        setTpMode(result.tp_mode);
      }
    } catch (err) {
      console.error('Failed to fetch SL/TP config:', err);
    }
  };

  const handleSaveSLTP = async () => {
    setSavingSLTP(true);
    try {
      const config = sltpConfig[selectedMode];
      await futuresApi.updateGinieSLTP(selectedMode, {
        sl_percent: config.sl_percent,
        tp_percent: config.tp_percent,
        trailing_enabled: config.trailing_enabled,
        trailing_percent: config.trailing_percent,
        trailing_activation: config.trailing_activation,
      });
      setSuccessMsg(`${selectedMode} mode SL/TP updated`);
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to update SL/TP configuration');
    } finally {
      setSavingSLTP(false);
    }
  };

  const handleSaveTPMode = async () => {
    setSavingSLTP(true);
    try {
      await futuresApi.updateGinieTPMode(tpMode);
      setSuccessMsg('TP mode updated successfully');
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to update TP mode');
    } finally {
      setSavingSLTP(false);
    }
  };

  const fetchTradeHistory = async () => {
    try {
      const result = await futuresApi.getTradeHistoryWithDateRange(selectedDateRange.start, selectedDateRange.end);
      setTradeHistory(result.trades || []);
    } catch (err) {
      console.error('Failed to fetch trade history:', err);
    }
  };

  const fetchPerformanceMetrics = async () => {
    setLoadingPerformance(true);
    try {
      const result = await futuresApi.getPerformanceMetrics(selectedDateRange.start, selectedDateRange.end);
      setPerformanceMetrics(result);
    } catch (err) {
      console.error('Failed to fetch performance metrics:', err);
    } finally {
      setLoadingPerformance(false);
    }
  };

  const fetchLLMSwitches = async () => {
    try {
      const result = await futuresApi.getLLMDiagnostics();
      setLlmSwitches(result.switches || []);
    } catch (err) {
      console.error('Failed to fetch LLM diagnostics:', err);
    }
  };

  const handleResetLLMDiagnostics = async () => {
    if (!window.confirm('Reset all LLM diagnostic data? This cannot be undone.')) return;
    try {
      await futuresApi.resetLLMDiagnostics();
      setSuccessMsg('LLM diagnostics reset');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchLLMSwitches();
    } catch (err) {
      setError('Failed to reset LLM diagnostics');
    }
  };

  // Mode Configuration functions (Story 2.7 Task 2.7.9)
  const fetchModeConfigs = async () => {
    try {
      const result = await futuresApi.getModeConfigs();
      if (result.success && result.mode_configs) {
        setModeConfigs(result.mode_configs);
      }
    } catch (err) {
      console.error('Failed to fetch mode configurations:', err);
    }
  };

  const validateModeConfig = (_mode: string, config: ModeFullConfig): string | null => {
    if (config.confidence) {
      if (config.confidence.min_confidence < 0 || config.confidence.min_confidence > 100) {
        return 'Min Confidence must be between 0 and 100';
      }
      if (config.confidence.high_confidence < config.confidence.min_confidence) {
        return 'High Confidence must be >= Min Confidence';
      }
      if (config.confidence.ultra_confidence < config.confidence.high_confidence) {
        return 'Ultra Confidence must be >= High Confidence';
      }
    }
    if (config.size) {
      if (config.size.base_size_usd < 0) return 'Base Size must be positive';
      if (config.size.max_size_usd < config.size.base_size_usd) {
        return 'Max Size must be >= Base Size';
      }
      if (config.size.leverage < 1 || config.size.leverage > 125) {
        return 'Leverage must be between 1 and 125';
      }
    }
    if (config.sltp) {
      if (config.sltp.stop_loss_percent < 0 || config.sltp.stop_loss_percent > 50) {
        return 'Stop Loss must be between 0 and 50%';
      }
      if (config.sltp.take_profit_percent < 0 || config.sltp.take_profit_percent > 100) {
        return 'Take Profit must be between 0 and 100%';
      }
    }
    return null;
  };

  const handleSaveModeConfig = async (mode: string) => {
    const config = modeConfigs[mode];
    if (!config) return;

    const validationError = validateModeConfig(mode, config);
    if (validationError) {
      setModeConfigErrors({ ...modeConfigErrors, [mode]: validationError });
      return;
    }
    setModeConfigErrors({ ...modeConfigErrors, [mode]: '' });

    setSavingModeConfig(true);
    try {
      await futuresApi.updateModeConfig(mode, config);
      setSuccessMsg(`${mode} configuration saved`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchModeConfigs();
    } catch (err) {
      setError(`Failed to save ${mode} configuration`);
    } finally {
      setSavingModeConfig(false);
    }
  };

  const handleResetModeConfigs = async () => {
    if (!window.confirm('Reset all mode configurations to defaults? This cannot be undone.')) return;
    setResettingModes(true);
    try {
      const result = await futuresApi.resetModeConfigs();
      if (result.success && result.mode_configs) {
        setModeConfigs(result.mode_configs);
      }
      setSuccessMsg('All mode configurations reset to defaults');
      setTimeout(() => setSuccessMsg(null), 3000);
    } catch (err) {
      setError('Failed to reset mode configurations');
    } finally {
      setResettingModes(false);
    }
  };

  const updateModeConfig = (mode: string, path: string, value: any) => {
    setModeConfigs(prev => {
      const updated = { ...prev };
      const config = { ...updated[mode] };
      const parts = path.split('.');

      if (parts.length === 1) {
        (config as any)[parts[0]] = value;
      } else if (parts.length === 2) {
        const section = parts[0] as keyof ModeFullConfig;
        if (!config[section]) {
          (config as any)[section] = {};
        }
        (config[section] as any)[parts[1]] = value;
      }

      updated[mode] = config;
      return updated;
    });
    // Clear any validation error when user makes changes
    setModeConfigErrors({ ...modeConfigErrors, [mode]: '' });
  };

  // LLM & Adaptive AI functions (Story 2.8)
  const fetchLLMConfig = async () => {
    try {
      const result = await futuresApi.getLLMConfig();
      if (result.success) {
        setLlmConfig(result.llm_config);
        setModeLLMSettings(result.mode_settings || {});
        setAdaptiveConfig(result.adaptive_config);
      }
    } catch (err) {
      console.error('Failed to fetch LLM config:', err);
    }
  };

  const fetchAdaptiveRecommendations = async () => {
    try {
      const result = await futuresApi.getAdaptiveRecommendations();
      if (result.success) {
        setRecommendations(result.recommendations || []);
        setModeStatistics(result.statistics || {});
      }
    } catch (err) {
      console.error('Failed to fetch adaptive recommendations:', err);
    }
  };

  const fetchLLMCallDiagnostics = async () => {
    try {
      const result = await futuresApi.getLLMCallDiagnostics();
      setLlmCallDiagnostics(result);
    } catch (err) {
      console.error('Failed to fetch LLM diagnostics:', err);
    }
  };

  const handleUpdateLLMConfig = async (updates: Partial<LLMConfig>) => {
    setSavingLLMConfig(true);
    try {
      await futuresApi.updateLLMConfig(updates);
      setSuccessMsg('LLM configuration updated');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchLLMConfig();
    } catch (err) {
      setError('Failed to update LLM configuration');
    } finally {
      setSavingLLMConfig(false);
    }
  };

  const handleUpdateModeLLMSettings = async (mode: string, settings: Partial<ModeLLMSettings>) => {
    setSavingLLMConfig(true);
    try {
      await futuresApi.updateModeLLMSettings(mode, settings);
      setSuccessMsg(`${mode} LLM settings updated`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchLLMConfig();
    } catch (err) {
      setError('Failed to update mode LLM settings');
    } finally {
      setSavingLLMConfig(false);
    }
  };

  const handleApplyRecommendation = async (id: string) => {
    setApplyingRecommendation(id);
    try {
      await futuresApi.applyRecommendation(id);
      setSuccessMsg('Recommendation applied');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAdaptiveRecommendations();
    } catch (err) {
      setError('Failed to apply recommendation');
    } finally {
      setApplyingRecommendation(null);
    }
  };

  const handleDismissRecommendation = async (id: string) => {
    setApplyingRecommendation(id);
    try {
      await futuresApi.dismissRecommendation(id);
      setSuccessMsg('Recommendation dismissed');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAdaptiveRecommendations();
    } catch (err) {
      setError('Failed to dismiss recommendation');
    } finally {
      setApplyingRecommendation(null);
    }
  };

  const handleApplyAllRecommendations = async () => {
    if (!window.confirm('Apply all pending recommendations?')) return;
    setSavingLLMConfig(true);
    try {
      const result = await futuresApi.applyAllRecommendations();
      setSuccessMsg(`Applied ${result.applied.length} recommendations`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchAdaptiveRecommendations();
    } catch (err) {
      setError('Failed to apply all recommendations');
    } finally {
      setSavingLLMConfig(false);
    }
  };

  const handleResetLLMCallDiagnostics = async () => {
    if (!window.confirm('Reset LLM diagnostics?')) return;
    try {
      await futuresApi.resetLLMCallDiagnostics();
      setSuccessMsg('LLM diagnostics reset');
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchLLMCallDiagnostics();
    } catch (err) {
      setError('Failed to reset LLM diagnostics');
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
      case 'ULTRAFAST-READY':
        return <span className="px-2 py-0.5 bg-orange-900/50 text-orange-400 rounded text-xs">UF</span>;
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

        {/* CONDITIONAL: Show Futures Autopilot toggle only when Genie is DISABLED */}
        {!status.enabled && (
          <>
            <button
              onClick={handleToggleFuturesAutopilot}
              disabled={togglingAutopilot}
              className={`flex items-center justify-center w-7 h-7 rounded transition-colors ${
                autopilotStatus?.stats?.running
                  ? 'bg-orange-900/30 hover:bg-orange-900/50 text-orange-400'
                  : 'bg-gray-900/30 hover:bg-gray-900/50 text-gray-400'
              }`}
              title={autopilotStatus?.stats?.running ? 'Stop Futures Autopilot' : 'Start Futures Autopilot'}
            >
              {autopilotStatus?.stats?.running ? <Power className="w-3.5 h-3.5" /> : <PowerOff className="w-3.5 h-3.5" />}
            </button>
            <div className="w-px h-5 bg-gray-600 mx-0.5" />
          </>
        )}

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

      {/* Trend Timeframes Setting */}
      <div className="space-y-2 mb-3">
        <div className="flex items-center gap-2 px-2 py-1.5 bg-gray-700/30 rounded border border-gray-600">
          <BarChart3 className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" />
          <span className="text-xs text-gray-300 whitespace-nowrap">Timeframes:</span>
          {!editingTimeframes ? (
            <>
              <span className="text-[10px] text-gray-500">UF:</span>
              <span className="text-xs text-red-400 font-medium">{(trendTimeframes as any).ultra_fast}</span>
              <span className="text-[10px] text-gray-500">Scalp:</span>
              <span className="text-xs text-yellow-400 font-medium">{trendTimeframes.scalp}</span>
              <span className="text-[10px] text-gray-500">Swing:</span>
              <span className="text-xs text-blue-400 font-medium">{trendTimeframes.swing}</span>
              <span className="text-[10px] text-gray-500">Pos:</span>
              <span className="text-xs text-purple-400 font-medium">{trendTimeframes.position}</span>
              <span className="ml-auto text-[10px]">
                {trendTimeframes.block_on_divergence ? '🚫' : '✓'}
              </span>
            </>
          ) : (
            <>
              <div className="flex-1 flex items-center gap-2">
                <div className="flex items-center gap-1">
                  <span className="text-[10px] text-gray-500">UF:</span>
                  <select
                    value={(trendTimeframes as any).ultra_fast}
                    onChange={(e) => setTrendTimeframes({...trendTimeframes, ultra_fast: e.target.value} as any)}
                    className="w-16 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  >
                    {validTimeframes.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                  </select>
                </div>
                <div className="flex items-center gap-1">
                  <span className="text-[10px] text-gray-500">Scalp:</span>
                  <select
                    value={trendTimeframes.scalp}
                    onChange={(e) => setTrendTimeframes({...trendTimeframes, scalp: e.target.value})}
                    className="w-16 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  >
                    {validTimeframes.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                  </select>
                </div>
                <div className="flex items-center gap-1">
                  <span className="text-[10px] text-gray-500">Swing:</span>
                  <select
                    value={trendTimeframes.swing}
                    onChange={(e) => setTrendTimeframes({...trendTimeframes, swing: e.target.value})}
                    className="w-16 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  >
                    {validTimeframes.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                  </select>
                </div>
                <div className="flex items-center gap-1">
                  <span className="text-[10px] text-gray-500">Pos:</span>
                  <select
                    value={trendTimeframes.position}
                    onChange={(e) => setTrendTimeframes({...trendTimeframes, position: e.target.value})}
                    className="w-16 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  >
                    {validTimeframes.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                  </select>
                </div>
                <button
                  onClick={() => setTrendTimeframes({...trendTimeframes, block_on_divergence: !trendTimeframes.block_on_divergence})}
                  className={`px-1.5 py-0.5 rounded text-[10px] transition-colors ${
                    trendTimeframes.block_on_divergence
                      ? 'bg-red-900/50 text-red-400 border border-red-700'
                      : 'bg-green-900/50 text-green-400 border border-green-700'
                  }`}
                  title="Toggle divergence blocking"
                >
                  {trendTimeframes.block_on_divergence ? 'BLOCK' : 'ALLOW'}
                </button>
              </div>
            </>
          )}
          <button
            onClick={() => setEditingTimeframes(!editingTimeframes)}
            disabled={savingTimeframes}
            className="px-1.5 py-0.5 bg-blue-900/50 hover:bg-blue-900/70 text-blue-400 rounded text-[10px] transition-colors disabled:opacity-50"
          >
            {editingTimeframes ? 'Done' : 'Edit'}
          </button>
          {editingTimeframes && (
            <button
              onClick={handleSaveTimeframes}
              disabled={savingTimeframes}
              className="px-1.5 py-0.5 bg-green-900/50 hover:bg-green-900/70 text-green-400 rounded text-[10px] transition-colors disabled:opacity-50"
            >
              {savingTimeframes ? '...' : 'Save'}
            </button>
          )}
        </div>
        {editingTimeframes && (
          <div className="px-2 py-2 bg-blue-900/20 border border-blue-700/30 rounded text-[10px] text-blue-400">
            💡 Set per-mode timeframes for trend detection. Block option prevents trades when severe divergence detected between timeframes.
          </div>
        )}
      </div>

      {/* SL/TP Configuration Section */}
      <div className="space-y-2 mb-3">
        <div className="flex items-center gap-2 px-2 py-1.5 bg-gray-700/30 rounded border border-gray-600">
          <Target className="w-3.5 h-3.5 text-red-400 flex-shrink-0" />
          <span className="text-xs text-gray-300 whitespace-nowrap">SL/TP:</span>

          {/* Mode Tabs - Always Visible */}
          <div className="flex gap-1 mr-2">
            {(['ultra_fast', 'scalp', 'swing', 'position'] as const).map(mode => (
              <button
                key={mode}
                onClick={() => setSelectedMode(mode)}
                className={`px-2 py-0.5 rounded text-[10px] transition-colors ${
                  selectedMode === mode
                    ? 'bg-white/20 text-white font-bold'
                    : 'bg-gray-600/30 text-gray-400 hover:text-gray-300'
                }`}
              >
                {mode.charAt(0).toUpperCase() + mode.slice(1)}
              </button>
            ))}
          </div>

          {!editingSLTP ? (
            <>
              <span className="text-[10px] text-gray-500">Manual:</span>
              <span className="text-xs text-yellow-400">{sltpConfig[selectedMode]?.sl_percent > 0 ? '✓' : '✗'}</span>
              <span className="text-[10px] text-gray-500">Trailing:</span>
              <span className="text-xs text-blue-400">{sltpConfig[selectedMode]?.trailing_enabled ? '✓' : '✗'}</span>
              <span className="text-[10px] text-gray-500">TP Mode:</span>
              <span className="text-xs text-purple-400">{tpMode?.use_single_tp ? 'Single' : 'Multi'}</span>
            </>
          ) : (
            <></>
          )}
          <button
            onClick={() => setEditingSLTP(!editingSLTP)}
            disabled={savingSLTP}
            className="px-1.5 py-0.5 bg-blue-900/50 hover:bg-blue-900/70 text-blue-400 rounded text-[10px] transition-colors disabled:opacity-50"
          >
            {editingSLTP ? 'Done' : 'Edit'}
          </button>
        </div>

        {editingSLTP && (
          <div className="space-y-3 px-3 py-2 bg-gray-800/50 border border-gray-600 rounded">
            {/* SL/TP Percentages */}
            <div>
              <h4 className="text-[11px] font-semibold text-gray-400 mb-2 flex items-center gap-1">
                <span>Manual SL/TP % (0 = use ATR/LLM)</span>
              </h4>
              <div className="grid grid-cols-2 gap-2">
                <div>
                  <label className="block text-[10px] text-gray-400 mb-1">Stop Loss %</label>
                  <input
                    type="number"
                    step="0.1"
                    min="0"
                    max="20"
                    value={sltpConfig[selectedMode]?.sl_percent || 0}
                    onChange={(e) => setSltpConfig({
                      ...sltpConfig,
                      [selectedMode]: {...(sltpConfig[selectedMode] || {}), sl_percent: parseFloat(e.target.value) || 0}
                    })}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
                <div>
                  <label className="block text-[10px] text-gray-400 mb-1">Take Profit %</label>
                  <input
                    type="number"
                    step="0.1"
                    min="0"
                    max="50"
                    value={sltpConfig[selectedMode]?.tp_percent || 0}
                    onChange={(e) => setSltpConfig({
                      ...sltpConfig,
                      [selectedMode]: {...(sltpConfig[selectedMode] || {}), tp_percent: parseFloat(e.target.value) || 0}
                    })}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
              </div>
            </div>

            {/* Trailing Stop */}
            <div>
              <label className="flex items-center gap-2 text-[11px] font-semibold text-gray-400 mb-2">
                <input
                  type="checkbox"
                  checked={sltpConfig[selectedMode]?.trailing_enabled || false}
                  onChange={(e) => setSltpConfig({
                    ...sltpConfig,
                    [selectedMode]: {...(sltpConfig[selectedMode] || {}), trailing_enabled: e.target.checked}
                  })}
                  className="w-3 h-3"
                />
                Trailing Stop
              </label>
              {sltpConfig[selectedMode]?.trailing_enabled && (
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Trailing %</label>
                    <input
                      type="number"
                      step="0.1"
                      min="0"
                      max="10"
                      value={sltpConfig[selectedMode]?.trailing_percent || 0}
                      onChange={(e) => setSltpConfig({
                        ...sltpConfig,
                        [selectedMode]: {...(sltpConfig[selectedMode] || {}), trailing_percent: parseFloat(e.target.value) || 0}
                      })}
                      className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Activation %</label>
                    <input
                      type="number"
                      step="0.1"
                      min="0"
                      max="20"
                      value={sltpConfig[selectedMode]?.trailing_activation || 0}
                      onChange={(e) => setSltpConfig({
                        ...sltpConfig,
                        [selectedMode]: {...(sltpConfig[selectedMode] || {}), trailing_activation: parseFloat(e.target.value) || 0}
                      })}
                      className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                    />
                  </div>
                </div>
              )}
            </div>

            {/* TP Mode Selection */}
            <div>
              <label className="block text-[11px] font-semibold text-gray-400 mb-2">Take Profit Mode</label>
              <div className="grid grid-cols-2 gap-2">
                <button
                  onClick={() => setTpMode({...tpMode, use_single_tp: true})}
                  className={`px-2 py-1 rounded text-[10px] transition-colors ${
                    tpMode.use_single_tp
                      ? 'bg-green-900/50 text-green-400 border border-green-700'
                      : 'bg-gray-700/50 text-gray-400 border border-gray-600'
                  }`}
                >
                  Single TP
                </button>
                <button
                  onClick={() => setTpMode({...tpMode, use_single_tp: false})}
                  className={`px-2 py-1 rounded text-[10px] transition-colors ${
                    !tpMode.use_single_tp
                      ? 'bg-blue-900/50 text-blue-400 border border-blue-700'
                      : 'bg-gray-700/50 text-gray-400 border border-gray-600'
                  }`}
                >
                  Multi TP
                </button>
              </div>

              {tpMode.use_single_tp ? (
                <div className="mt-2">
                  <label className="block text-[10px] text-gray-400 mb-1">Close at % gain</label>
                  <input
                    type="number"
                    step="0.1"
                    min="0"
                    max="50"
                    value={tpMode.single_tp_percent}
                    onChange={(e) => setTpMode({...tpMode, single_tp_percent: parseFloat(e.target.value) || 0})}
                    className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  />
                </div>
              ) : (
                <div className="mt-2 grid grid-cols-4 gap-1">
                  {(['tp1', 'tp2', 'tp3', 'tp4'] as const).map((level) => (
                    <div key={level}>
                      <label className="block text-[10px] text-gray-400 mb-1">{level.toUpperCase()}</label>
                      <input
                        type="number"
                        step="0.1"
                        min="0"
                        max="100"
                        value={tpMode[`${level}_percent`]}
                        onChange={(e) => setTpMode({...tpMode, [`${level}_percent`]: parseFloat(e.target.value) || 0})}
                        className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs text-center"
                      />
                      <div className="text-[9px] text-gray-500 text-center mt-0.5">
                        {(tpMode.tp1_percent + tpMode.tp2_percent + tpMode.tp3_percent + tpMode.tp4_percent).toFixed(1)}% total
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Save Buttons */}
            <div className="flex gap-2 pt-2">
              <button
                onClick={handleSaveSLTP}
                disabled={savingSLTP}
                className="flex-1 px-2 py-1 bg-green-900/50 hover:bg-green-900/70 text-green-400 rounded text-[10px] transition-colors disabled:opacity-50"
              >
                {savingSLTP ? 'Saving...' : `Save ${selectedMode}`}
              </button>
              <button
                onClick={handleSaveTPMode}
                disabled={savingSLTP}
                className="flex-1 px-2 py-1 bg-purple-900/50 hover:bg-purple-900/70 text-purple-400 rounded text-[10px] transition-colors disabled:opacity-50"
              >
                {savingSLTP ? 'Saving...' : 'Save TP Mode'}
              </button>
            </div>

            <div className="px-2 py-1.5 bg-blue-900/20 border border-blue-700/30 rounded text-[10px] text-blue-400">
              💡 Manual override: Set % &gt; 0 to override ATR/LLM. Single TP closes at one level, Multi-TP distributes across 4 levels.
            </div>
          </div>
        )}
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

      {/* Mode Configuration Section (Story 2.7 Task 2.7.9) */}
      <div className="space-y-2 mb-3">
        <div
          className="flex items-center justify-between gap-2 px-2 py-1.5 bg-gray-700/30 rounded border border-gray-600 cursor-pointer hover:bg-gray-700/50 transition-colors"
          onClick={() => setShowModeConfig(!showModeConfig)}
        >
          <div className="flex items-center gap-2">
            <Settings className="w-3.5 h-3.5 text-indigo-400 flex-shrink-0" />
            <span className="text-xs text-gray-300 font-medium">Mode Configuration</span>
            <span className="text-[10px] text-gray-500">
              ({Object.values(modeConfigs).filter(c => c.enabled).length}/4 enabled)
            </span>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={(e) => { e.stopPropagation(); handleResetModeConfigs(); }}
              disabled={resettingModes}
              className="px-1.5 py-0.5 bg-orange-900/50 hover:bg-orange-900/70 text-orange-400 rounded text-[10px] transition-colors disabled:opacity-50"
              title="Reset All Modes to Defaults"
            >
              {resettingModes ? '...' : 'Reset All'}
            </button>
            {showModeConfig ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </div>
        </div>

        {showModeConfig && (
          <div className="px-2 py-2 bg-gray-800/50 border border-gray-600 rounded space-y-3">
            {/* Mode Tabs */}
            <div className="flex gap-1 border-b border-gray-700 pb-2">
              {(['ultra_fast', 'scalp', 'swing', 'position'] as const).map(mode => {
                const config = modeConfigs[mode];
                const modeColors: Record<string, string> = {
                  ultra_fast: 'text-red-400 bg-red-900/30 border-red-700',
                  scalp: 'text-yellow-400 bg-yellow-900/30 border-yellow-700',
                  swing: 'text-blue-400 bg-blue-900/30 border-blue-700',
                  position: 'text-purple-400 bg-purple-900/30 border-purple-700',
                };
                const inactiveColors = 'text-gray-400 bg-gray-700/30 border-gray-600';
                return (
                  <button
                    key={mode}
                    onClick={() => setSelectedModeConfig(mode)}
                    className={`flex-1 px-2 py-1 rounded text-[10px] font-medium border transition-colors ${
                      selectedModeConfig === mode ? modeColors[mode] : inactiveColors
                    }`}
                  >
                    <div className="flex items-center justify-center gap-1">
                      {mode === 'ultra_fast' ? 'Ultra-Fast' : mode.charAt(0).toUpperCase() + mode.slice(1)}
                      {config?.enabled ? (
                        <CheckCircle className="w-2.5 h-2.5" />
                      ) : (
                        <XCircle className="w-2.5 h-2.5 opacity-50" />
                      )}
                    </div>
                  </button>
                );
              })}
            </div>

            {/* Selected Mode Configuration */}
            {modeConfigs[selectedModeConfig] && (
              <div className="space-y-3">
                {/* Enable/Disable Toggle */}
                <div className="flex items-center justify-between">
                  <label className="text-xs text-gray-300 font-medium">Enable {selectedModeConfig === 'ultra_fast' ? 'Ultra-Fast' : selectedModeConfig.charAt(0).toUpperCase() + selectedModeConfig.slice(1)} Mode</label>
                  <button
                    onClick={() => updateModeConfig(selectedModeConfig, 'enabled', !modeConfigs[selectedModeConfig]?.enabled)}
                    className={`px-2 py-1 rounded text-[10px] transition-colors ${
                      modeConfigs[selectedModeConfig]?.enabled
                        ? 'bg-green-900/50 text-green-400 border border-green-700'
                        : 'bg-gray-700/50 text-gray-400 border border-gray-600'
                    }`}
                  >
                    {modeConfigs[selectedModeConfig]?.enabled ? 'Enabled' : 'Disabled'}
                  </button>
                </div>

                {/* Collapsible Sections */}
                {/* Timeframe Settings */}
                <div className="border border-gray-700 rounded">
                  <button
                    onClick={() => setExpandedModeSection(expandedModeSection === 'timeframe' ? null : 'timeframe')}
                    className="w-full flex items-center justify-between px-2 py-1.5 text-xs text-gray-300 hover:bg-gray-700/30"
                  >
                    <span className="font-medium">Timeframe Settings</span>
                    {expandedModeSection === 'timeframe' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                  </button>
                  {expandedModeSection === 'timeframe' && modeConfigs[selectedModeConfig]?.timeframe && (
                    <div className="px-2 py-2 border-t border-gray-700 grid grid-cols-3 gap-2">
                      <div>
                        <label className="block text-[10px] text-gray-400 mb-1">Trend TF</label>
                        <select
                          value={modeConfigs[selectedModeConfig]?.timeframe?.trend_timeframe || '1h'}
                          onChange={(e) => updateModeConfig(selectedModeConfig, 'timeframe.trend_timeframe', e.target.value)}
                          className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                        >
                          {timeframeOptions.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                        </select>
                      </div>
                      <div>
                        <label className="block text-[10px] text-gray-400 mb-1">Entry TF</label>
                        <select
                          value={modeConfigs[selectedModeConfig]?.timeframe?.entry_timeframe || '15m'}
                          onChange={(e) => updateModeConfig(selectedModeConfig, 'timeframe.entry_timeframe', e.target.value)}
                          className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                        >
                          {timeframeOptions.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                        </select>
                      </div>
                      <div>
                        <label className="block text-[10px] text-gray-400 mb-1">Analysis TF</label>
                        <select
                          value={modeConfigs[selectedModeConfig]?.timeframe?.analysis_timeframe || '4h'}
                          onChange={(e) => updateModeConfig(selectedModeConfig, 'timeframe.analysis_timeframe', e.target.value)}
                          className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                        >
                          {timeframeOptions.map(tf => <option key={tf} value={tf}>{tf}</option>)}
                        </select>
                      </div>
                    </div>
                  )}
                </div>

                {/* Confidence Thresholds */}
                <div className="border border-gray-700 rounded">
                  <button
                    onClick={() => setExpandedModeSection(expandedModeSection === 'confidence' ? null : 'confidence')}
                    className="w-full flex items-center justify-between px-2 py-1.5 text-xs text-gray-300 hover:bg-gray-700/30"
                  >
                    <span className="font-medium">Confidence Thresholds</span>
                    {expandedModeSection === 'confidence' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                  </button>
                  {expandedModeSection === 'confidence' && modeConfigs[selectedModeConfig]?.confidence && (
                    <div className="px-2 py-2 border-t border-gray-700 grid grid-cols-3 gap-2">
                      <div>
                        <label className="block text-[10px] text-gray-400 mb-1">Min Confidence</label>
                        <div className="flex items-center gap-1">
                          <input
                            type="number"
                            min="0"
                            max="100"
                            step="5"
                            value={modeConfigs[selectedModeConfig]?.confidence?.min_confidence || 50}
                            onChange={(e) => updateModeConfig(selectedModeConfig, 'confidence.min_confidence', Number(e.target.value))}
                            className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                          />
                          <span className="text-[10px] text-gray-500">%</span>
                        </div>
                      </div>
                      <div>
                        <label className="block text-[10px] text-gray-400 mb-1">High Confidence</label>
                        <div className="flex items-center gap-1">
                          <input
                            type="number"
                            min="0"
                            max="100"
                            step="5"
                            value={modeConfigs[selectedModeConfig]?.confidence?.high_confidence || 75}
                            onChange={(e) => updateModeConfig(selectedModeConfig, 'confidence.high_confidence', Number(e.target.value))}
                            className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                          />
                          <span className="text-[10px] text-gray-500">%</span>
                        </div>
                      </div>
                      <div>
                        <label className="block text-[10px] text-gray-400 mb-1">Ultra Confidence</label>
                        <div className="flex items-center gap-1">
                          <input
                            type="number"
                            min="0"
                            max="100"
                            step="5"
                            value={modeConfigs[selectedModeConfig]?.confidence?.ultra_confidence || 90}
                            onChange={(e) => updateModeConfig(selectedModeConfig, 'confidence.ultra_confidence', Number(e.target.value))}
                            className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                          />
                          <span className="text-[10px] text-gray-500">%</span>
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                {/* Position Sizing */}
                <div className="border border-gray-700 rounded">
                  <button
                    onClick={() => setExpandedModeSection(expandedModeSection === 'size' ? null : 'size')}
                    className="w-full flex items-center justify-between px-2 py-1.5 text-xs text-gray-300 hover:bg-gray-700/30"
                  >
                    <span className="font-medium">Position Sizing</span>
                    {expandedModeSection === 'size' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                  </button>
                  {expandedModeSection === 'size' && modeConfigs[selectedModeConfig]?.size && (
                    <div className="px-2 py-2 border-t border-gray-700 space-y-2">
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Base Size</label>
                          <div className="flex items-center gap-1">
                            <span className="text-[10px] text-gray-500">$</span>
                            <input
                              type="number"
                              min="10"
                              max="10000"
                              step="50"
                              value={modeConfigs[selectedModeConfig]?.size?.base_size_usd || 100}
                              onChange={(e) => updateModeConfig(selectedModeConfig, 'size.base_size_usd', Number(e.target.value))}
                              className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                            />
                          </div>
                        </div>
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Max Size</label>
                          <div className="flex items-center gap-1">
                            <span className="text-[10px] text-gray-500">$</span>
                            <input
                              type="number"
                              min="10"
                              max="50000"
                              step="50"
                              value={modeConfigs[selectedModeConfig]?.size?.max_size_usd || 500}
                              onChange={(e) => updateModeConfig(selectedModeConfig, 'size.max_size_usd', Number(e.target.value))}
                              className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                            />
                          </div>
                        </div>
                      </div>
                      <div className="grid grid-cols-3 gap-2">
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Max Positions</label>
                          <input
                            type="number"
                            min="1"
                            max="20"
                            value={modeConfigs[selectedModeConfig]?.size?.max_positions || 3}
                            onChange={(e) => updateModeConfig(selectedModeConfig, 'size.max_positions', Number(e.target.value))}
                            className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                          />
                        </div>
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Leverage</label>
                          <div className="flex items-center gap-1">
                            <input
                              type="number"
                              min="1"
                              max="125"
                              value={modeConfigs[selectedModeConfig]?.size?.leverage || 5}
                              onChange={(e) => updateModeConfig(selectedModeConfig, 'size.leverage', Number(e.target.value))}
                              className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                            />
                            <span className="text-[10px] text-gray-500">x</span>
                          </div>
                        </div>
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Size Multiplier</label>
                          <div className="flex items-center gap-1">
                            <input
                              type="number"
                              min="1"
                              max="5"
                              step="0.1"
                              value={modeConfigs[selectedModeConfig]?.size?.size_multiplier_hi || 1.5}
                              onChange={(e) => updateModeConfig(selectedModeConfig, 'size.size_multiplier_hi', Number(e.target.value))}
                              className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                            />
                            <span className="text-[10px] text-gray-500">x</span>
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                {/* SL/TP Settings */}
                <div className="border border-gray-700 rounded">
                  <button
                    onClick={() => setExpandedModeSection(expandedModeSection === 'sltp' ? null : 'sltp')}
                    className="w-full flex items-center justify-between px-2 py-1.5 text-xs text-gray-300 hover:bg-gray-700/30"
                  >
                    <span className="font-medium">SL/TP Settings</span>
                    {expandedModeSection === 'sltp' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                  </button>
                  {expandedModeSection === 'sltp' && modeConfigs[selectedModeConfig]?.sltp && (
                    <div className="px-2 py-2 border-t border-gray-700 space-y-2">
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Stop Loss %</label>
                          <div className="flex items-center gap-1">
                            <input
                              type="number"
                              min="0"
                              max="50"
                              step="0.1"
                              value={modeConfigs[selectedModeConfig]?.sltp?.stop_loss_percent || 1.5}
                              onChange={(e) => updateModeConfig(selectedModeConfig, 'sltp.stop_loss_percent', Number(e.target.value))}
                              className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                            />
                            <span className="text-[10px] text-gray-500">%</span>
                          </div>
                        </div>
                        <div>
                          <label className="block text-[10px] text-gray-400 mb-1">Take Profit %</label>
                          <div className="flex items-center gap-1">
                            <input
                              type="number"
                              min="0"
                              max="100"
                              step="0.1"
                              value={modeConfigs[selectedModeConfig]?.sltp?.take_profit_percent || 3.0}
                              onChange={(e) => updateModeConfig(selectedModeConfig, 'sltp.take_profit_percent', Number(e.target.value))}
                              className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                            />
                            <span className="text-[10px] text-gray-500">%</span>
                          </div>
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        <label className="flex items-center gap-1 text-[10px] text-gray-400">
                          <input
                            type="checkbox"
                            checked={modeConfigs[selectedModeConfig]?.sltp?.trailing_stop_enabled || false}
                            onChange={(e) => updateModeConfig(selectedModeConfig, 'sltp.trailing_stop_enabled', e.target.checked)}
                            className="w-3 h-3"
                          />
                          Trailing Stop
                        </label>
                        {modeConfigs[selectedModeConfig]?.sltp?.trailing_stop_enabled && (
                          <div className="flex items-center gap-2">
                            <div className="flex items-center gap-1">
                              <span className="text-[10px] text-gray-500">Trail:</span>
                              <input
                                type="number"
                                min="0"
                                max="20"
                                step="0.1"
                                value={modeConfigs[selectedModeConfig]?.sltp?.trailing_stop_percent || 1.0}
                                onChange={(e) => updateModeConfig(selectedModeConfig, 'sltp.trailing_stop_percent', Number(e.target.value))}
                                className="w-12 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                              />
                              <span className="text-[10px] text-gray-500">%</span>
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>

                {/* Circuit Breaker Preview */}
                <div className="border border-gray-700 rounded">
                  <button
                    onClick={() => setExpandedModeSection(expandedModeSection === 'circuit' ? null : 'circuit')}
                    className="w-full flex items-center justify-between px-2 py-1.5 text-xs text-gray-300 hover:bg-gray-700/30"
                  >
                    <span className="font-medium">Circuit Breaker Preview</span>
                    {expandedModeSection === 'circuit' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
                  </button>
                  {expandedModeSection === 'circuit' && modeConfigs[selectedModeConfig]?.circuit_breaker && (
                    <div className="px-2 py-2 border-t border-gray-700">
                      <div className="grid grid-cols-3 gap-2 text-[10px]">
                        <div className="bg-gray-700/30 rounded p-1.5">
                          <div className="text-gray-500">Max Loss/Day</div>
                          <div className="text-red-400 font-medium">${modeConfigs[selectedModeConfig]?.circuit_breaker?.max_loss_per_day || 100}</div>
                        </div>
                        <div className="bg-gray-700/30 rounded p-1.5">
                          <div className="text-gray-500">Max Consec. Loss</div>
                          <div className="text-yellow-400 font-medium">{modeConfigs[selectedModeConfig]?.circuit_breaker?.max_consecutive_losses || 5}</div>
                        </div>
                        <div className="bg-gray-700/30 rounded p-1.5">
                          <div className="text-gray-500">Cooldown</div>
                          <div className="text-blue-400 font-medium">{modeConfigs[selectedModeConfig]?.circuit_breaker?.cooldown_minutes || 30} min</div>
                        </div>
                      </div>
                      <div className="text-[9px] text-gray-500 mt-1.5 text-center">
                        Circuit breaker settings are read-only in this view
                      </div>
                    </div>
                  )}
                </div>

                {/* Validation Errors */}
                {modeConfigErrors[selectedModeConfig] && (
                  <div className="px-2 py-1.5 bg-red-900/30 border border-red-700 rounded text-[10px] text-red-400 flex items-center gap-1">
                    <AlertCircle className="w-3 h-3" />
                    {modeConfigErrors[selectedModeConfig]}
                  </div>
                )}

                {/* Save Button */}
                <div className="flex justify-end pt-1">
                  <button
                    onClick={() => handleSaveModeConfig(selectedModeConfig)}
                    disabled={savingModeConfig}
                    className="px-3 py-1 bg-indigo-900/50 hover:bg-indigo-900/70 text-indigo-400 rounded text-xs transition-colors disabled:opacity-50"
                  >
                    {savingModeConfig ? 'Saving...' : `Save ${selectedModeConfig === 'ultra_fast' ? 'Ultra-Fast' : selectedModeConfig.charAt(0).toUpperCase() + selectedModeConfig.slice(1)} Config`}
                  </button>
                </div>
              </div>
            )}

            {/* Help Text */}
            <div className="px-2 py-1.5 bg-indigo-900/20 border border-indigo-700/30 rounded text-[10px] text-indigo-400">
              Configure each trading mode independently. Ultra-Fast for quick scalps, Scalp for short-term, Swing for medium-term, Position for long-term trades.
            </div>
          </div>
        )}
      </div>

      {/* LLM & Adaptive AI Section (Story 2.8) */}
      <div className="space-y-2 mb-3">
        <div
          className="flex items-center justify-between gap-2 px-2 py-1.5 bg-gray-700/30 rounded border border-gray-600 cursor-pointer hover:bg-gray-700/50 transition-colors"
          onClick={() => {
            setShowLLMSettings(!showLLMSettings);
            if (!showLLMSettings) {
              fetchLLMConfig();
              fetchAdaptiveRecommendations();
              fetchLLMCallDiagnostics();
            }
          }}
        >
          <div className="flex items-center gap-2">
            <Brain className="w-3.5 h-3.5 text-cyan-400 flex-shrink-0" />
            <span className="text-xs text-gray-300 font-medium">LLM & Adaptive AI</span>
            {llmConfig?.enabled && (
              <span className="px-1 py-0.5 bg-cyan-900/50 text-cyan-400 rounded text-[10px]">ON</span>
            )}
            {adaptiveConfig?.enabled && (
              <span className="px-1 py-0.5 bg-green-900/50 text-green-400 rounded text-[10px]">LEARN</span>
            )}
            {recommendations.filter(r => !r.dismissed && !r.applied_at).length > 0 && (
              <span className="px-1.5 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-[10px] font-medium">
                {recommendations.filter(r => !r.dismissed && !r.applied_at).length} recs
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            {showLLMSettings ? <ChevronUp className="w-3.5 h-3.5 text-gray-400" /> : <ChevronDown className="w-3.5 h-3.5 text-gray-400" />}
          </div>
        </div>

        {showLLMSettings && (
          <div className="px-2 py-2 bg-gray-800/50 border border-gray-600 rounded space-y-4">
            {/* LLM Provider Configuration */}
            <div className="border border-gray-700 rounded p-2 space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Gauge className="w-3.5 h-3.5 text-cyan-400" />
                  <span className="text-xs text-gray-300 font-medium">LLM Provider</span>
                </div>
                <button
                  onClick={() => handleUpdateLLMConfig({ enabled: !llmConfig?.enabled })}
                  disabled={savingLLMConfig}
                  className={`px-2 py-0.5 rounded text-[10px] transition-colors ${
                    llmConfig?.enabled
                      ? 'bg-cyan-900/50 text-cyan-400 border border-cyan-700'
                      : 'bg-gray-700/50 text-gray-400 border border-gray-600'
                  }`}
                >
                  {llmConfig?.enabled ? 'Enabled' : 'Disabled'}
                </button>
              </div>

              {llmConfig?.enabled && (
                <div className="grid grid-cols-2 gap-2 mt-2">
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Provider</label>
                    <select
                      value={llmConfig?.provider || 'deepseek'}
                      onChange={(e) => handleUpdateLLMConfig({ provider: e.target.value })}
                      className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                    >
                      <option value="deepseek">DeepSeek</option>
                      <option value="claude">Claude</option>
                      <option value="openai">OpenAI</option>
                      <option value="local">Local</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Model</label>
                    <input
                      type="text"
                      value={llmConfig?.model || ''}
                      onChange={(e) => handleUpdateLLMConfig({ model: e.target.value })}
                      className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                      placeholder="e.g. deepseek-chat"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Timeout (ms)</label>
                    <input
                      type="number"
                      min="1000"
                      max="30000"
                      step="1000"
                      value={llmConfig?.timeout_ms || 10000}
                      onChange={(e) => handleUpdateLLMConfig({ timeout_ms: Number(e.target.value) })}
                      className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] text-gray-400 mb-1">Cache Duration (sec)</label>
                    <input
                      type="number"
                      min="0"
                      max="3600"
                      step="60"
                      value={llmConfig?.cache_duration_sec || 300}
                      onChange={(e) => handleUpdateLLMConfig({ cache_duration_sec: Number(e.target.value) })}
                      className="w-full px-1 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                    />
                  </div>
                </div>
              )}
            </div>

            {/* Mode LLM Weight Sliders */}
            {llmConfig?.enabled && (
              <div className="border border-gray-700 rounded p-2 space-y-2">
                <div className="flex items-center gap-2 mb-2">
                  <Settings className="w-3.5 h-3.5 text-purple-400" />
                  <span className="text-xs text-gray-300 font-medium">Mode LLM Settings</span>
                </div>

                {/* Mode Tabs */}
                <div className="flex gap-1 mb-2">
                  {(['ultra_fast', 'scalp', 'swing', 'position'] as const).map(mode => (
                    <button
                      key={mode}
                      onClick={() => setSelectedLLMMode(mode)}
                      className={`flex-1 px-2 py-0.5 rounded text-[10px] transition-colors ${
                        selectedLLMMode === mode
                          ? 'bg-purple-900/50 text-purple-400 border border-purple-700'
                          : 'bg-gray-700/30 text-gray-400 border border-gray-600'
                      }`}
                    >
                      {mode === 'ultra_fast' ? 'UF' : mode.charAt(0).toUpperCase() + mode.slice(1)}
                    </button>
                  ))}
                </div>

                {modeLLMSettings[selectedLLMMode] && (
                  <div className="space-y-2">
                    {/* LLM Enable for mode */}
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] text-gray-400">Enable LLM for {selectedLLMMode}</span>
                      <button
                        onClick={() => handleUpdateModeLLMSettings(selectedLLMMode, {
                          llm_enabled: !modeLLMSettings[selectedLLMMode]?.llm_enabled
                        })}
                        disabled={savingLLMConfig}
                        className={`px-1.5 py-0.5 rounded text-[10px] ${
                          modeLLMSettings[selectedLLMMode]?.llm_enabled
                            ? 'bg-green-900/50 text-green-400'
                            : 'bg-gray-700 text-gray-400'
                        }`}
                      >
                        {modeLLMSettings[selectedLLMMode]?.llm_enabled ? 'ON' : 'OFF'}
                      </button>
                    </div>

                    {/* LLM Weight Slider */}
                    <div>
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-[10px] text-gray-400">LLM Weight</span>
                        <span className="text-[10px] text-purple-400 font-medium">
                          {Math.round((modeLLMSettings[selectedLLMMode]?.llm_weight || 0) * 100)}%
                        </span>
                      </div>
                      <input
                        type="range"
                        min="0"
                        max="100"
                        value={Math.round((modeLLMSettings[selectedLLMMode]?.llm_weight || 0) * 100)}
                        onChange={(e) => handleUpdateModeLLMSettings(selectedLLMMode, {
                          llm_weight: Number(e.target.value) / 100
                        })}
                        className="w-full h-1.5 bg-gray-600 rounded-lg appearance-none cursor-pointer accent-purple-500"
                      />
                    </div>

                    {/* Min LLM Confidence */}
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] text-gray-400">Min LLM Confidence</span>
                      <input
                        type="number"
                        min="0"
                        max="100"
                        step="5"
                        value={Math.round((modeLLMSettings[selectedLLMMode]?.min_llm_confidence || 0) * 100)}
                        onChange={(e) => handleUpdateModeLLMSettings(selectedLLMMode, {
                          min_llm_confidence: Number(e.target.value) / 100
                        })}
                        className="w-14 px-1 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-[10px] text-center"
                      />
                    </div>

                    {/* Toggles */}
                    <div className="flex flex-wrap gap-2">
                      <label className="flex items-center gap-1 text-[10px] text-gray-400">
                        <input
                          type="checkbox"
                          checked={modeLLMSettings[selectedLLMMode]?.skip_on_timeout || false}
                          onChange={(e) => handleUpdateModeLLMSettings(selectedLLMMode, {
                            skip_on_timeout: e.target.checked
                          })}
                          className="w-3 h-3"
                        />
                        Skip on timeout
                      </label>
                      <label className="flex items-center gap-1 text-[10px] text-gray-400">
                        <input
                          type="checkbox"
                          checked={modeLLMSettings[selectedLLMMode]?.block_on_disagreement || false}
                          onChange={(e) => handleUpdateModeLLMSettings(selectedLLMMode, {
                            block_on_disagreement: e.target.checked
                          })}
                          className="w-3 h-3"
                        />
                        Block on disagreement
                      </label>
                      <label className="flex items-center gap-1 text-[10px] text-gray-400">
                        <input
                          type="checkbox"
                          checked={modeLLMSettings[selectedLLMMode]?.cache_enabled || false}
                          onChange={(e) => handleUpdateModeLLMSettings(selectedLLMMode, {
                            cache_enabled: e.target.checked
                          })}
                          className="w-3 h-3"
                        />
                        Cache enabled
                      </label>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Adaptive AI Recommendations */}
            <div className="border border-gray-700 rounded p-2 space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Lightbulb className="w-3.5 h-3.5 text-yellow-400" />
                  <span className="text-xs text-gray-300 font-medium">Adaptive Recommendations</span>
                  {recommendations.filter(r => !r.dismissed && !r.applied_at).length > 0 && (
                    <span className="px-1.5 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-[10px]">
                      {recommendations.filter(r => !r.dismissed && !r.applied_at).length}
                    </span>
                  )}
                </div>
                {recommendations.filter(r => !r.dismissed && !r.applied_at).length > 1 && (
                  <button
                    onClick={handleApplyAllRecommendations}
                    disabled={savingLLMConfig}
                    className="px-1.5 py-0.5 bg-green-900/50 hover:bg-green-900/70 text-green-400 rounded text-[10px] disabled:opacity-50"
                  >
                    Apply All
                  </button>
                )}
              </div>

              {recommendations.filter(r => !r.dismissed && !r.applied_at).length === 0 ? (
                <div className="text-[10px] text-gray-500 py-2 text-center">
                  No pending recommendations
                </div>
              ) : (
                <div className="space-y-1 max-h-40 overflow-y-auto">
                  {recommendations.filter(r => !r.dismissed && !r.applied_at).map(rec => (
                    <div key={rec.id} className="bg-gray-700/30 rounded p-1.5 space-y-1">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-1.5">
                          <span className="text-[10px] text-yellow-400 font-medium">{rec.type}</span>
                          <span className="text-[9px] text-gray-500">({rec.mode})</span>
                        </div>
                        <div className="flex items-center gap-1">
                          <button
                            onClick={() => handleApplyRecommendation(rec.id)}
                            disabled={applyingRecommendation === rec.id}
                            className="p-0.5 bg-green-900/50 hover:bg-green-900/70 text-green-400 rounded disabled:opacity-50"
                            title="Apply"
                          >
                            <Check className="w-3 h-3" />
                          </button>
                          <button
                            onClick={() => handleDismissRecommendation(rec.id)}
                            disabled={applyingRecommendation === rec.id}
                            className="p-0.5 bg-red-900/50 hover:bg-red-900/70 text-red-400 rounded disabled:opacity-50"
                            title="Dismiss"
                          >
                            <X className="w-3 h-3" />
                          </button>
                        </div>
                      </div>
                      <div className="flex items-center gap-2 text-[9px]">
                        <span className="text-gray-500">Current:</span>
                        <span className="text-red-400">{JSON.stringify(rec.current_value)}</span>
                        <span className="text-gray-500">-&gt;</span>
                        <span className="text-green-400">{JSON.stringify(rec.suggested_value)}</span>
                      </div>
                      <div className="text-[9px] text-gray-400">{rec.reason}</div>
                      {rec.expected_improvement && (
                        <div className="text-[9px] text-cyan-400">Expected: {rec.expected_improvement}</div>
                      )}
                    </div>
                  ))}
                </div>
              )}

              {/* Mode Statistics */}
              {Object.keys(modeStatistics).length > 0 && (
                <div className="mt-2 pt-2 border-t border-gray-700">
                  <div className="text-[10px] text-gray-400 mb-1">Mode Statistics</div>
                  <div className="grid grid-cols-4 gap-1">
                    {Object.entries(modeStatistics).map(([mode, stats]) => (
                      <div key={mode} className="bg-gray-700/30 rounded p-1 text-center">
                        <div className="text-[9px] text-gray-500 capitalize">{mode}</div>
                        <div className={`text-[10px] font-medium ${stats.win_rate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                          {stats.win_rate.toFixed(0)}% WR
                        </div>
                        <div className="text-[9px] text-gray-500">{stats.total_trades} trades</div>
                        {stats.agreement_win_rate > 0 && (
                          <div className="text-[9px] text-cyan-400" title="Agreement vs Disagreement win rate">
                            {stats.agreement_win_rate.toFixed(0)}% vs {stats.disagreement_win_rate.toFixed(0)}%
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* LLM Diagnostics */}
            <div className="border border-gray-700 rounded p-2 space-y-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <Stethoscope className="w-3.5 h-3.5 text-blue-400" />
                  <span className="text-xs text-gray-300 font-medium">LLM Diagnostics</span>
                </div>
                <button
                  onClick={handleResetLLMCallDiagnostics}
                  className="px-1.5 py-0.5 bg-orange-900/50 hover:bg-orange-900/70 text-orange-400 rounded text-[10px]"
                >
                  Reset
                </button>
              </div>

              {llmCallDiagnostics && (
                <div className="grid grid-cols-4 gap-2 text-center">
                  <div className="bg-gray-700/30 rounded p-1.5">
                    <div className="text-[9px] text-gray-500">Total Calls</div>
                    <div className="text-xs text-white font-medium">{llmCallDiagnostics.total_calls}</div>
                  </div>
                  <div className="bg-gray-700/30 rounded p-1.5">
                    <div className="text-[9px] text-gray-500">Cache Hits</div>
                    <div className="text-xs text-green-400 font-medium">
                      {llmCallDiagnostics.cache_hits > 0
                        ? Math.round((llmCallDiagnostics.cache_hits / (llmCallDiagnostics.cache_hits + llmCallDiagnostics.cache_misses)) * 100)
                        : 0}%
                    </div>
                  </div>
                  <div className="bg-gray-700/30 rounded p-1.5">
                    <div className="text-[9px] text-gray-500">Avg Latency</div>
                    <div className={`text-xs font-medium ${llmCallDiagnostics.avg_latency_ms > 3000 ? 'text-yellow-400' : 'text-white'}`}>
                      {llmCallDiagnostics.avg_latency_ms.toFixed(0)}ms
                    </div>
                  </div>
                  <div className="bg-gray-700/30 rounded p-1.5">
                    <div className="text-[9px] text-gray-500">Error Rate</div>
                    <div className={`text-xs font-medium ${llmCallDiagnostics.error_rate > 0.1 ? 'text-red-400' : 'text-white'}`}>
                      {(llmCallDiagnostics.error_rate * 100).toFixed(1)}%
                    </div>
                  </div>
                </div>
              )}

              {/* Recent Errors */}
              {llmCallDiagnostics?.recent_errors && llmCallDiagnostics.recent_errors.length > 0 && (
                <div className="mt-2">
                  <div className="text-[10px] text-gray-400 mb-1">Recent Errors</div>
                  <div className="space-y-0.5 max-h-20 overflow-y-auto">
                    {llmCallDiagnostics.recent_errors.slice(0, 5).map((err, idx) => (
                      <div key={idx} className="text-[9px] text-red-400 truncate" title={err}>
                        {err}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            {/* Help Text */}
            <div className="px-2 py-1.5 bg-cyan-900/20 border border-cyan-700/30 rounded text-[10px] text-cyan-400">
              LLM enhances trading decisions with AI analysis. Adaptive AI learns from outcomes to improve recommendations.
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
      {activeTab === 'history' && (
        <div className="space-y-2">
          {/* Date Range Filter */}
          <div className="flex items-center gap-2 mb-2 text-xs">
            <span className="text-gray-400">From:</span>
            <input
              type="date"
              value={selectedDateRange.start}
              onChange={(e) => {
                setSelectedDateRange({...selectedDateRange, start: e.target.value});
                fetchTradeHistory();
              }}
              className="px-2 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
            />
            <span className="text-gray-400">To:</span>
            <input
              type="date"
              value={selectedDateRange.end}
              onChange={(e) => {
                setSelectedDateRange({...selectedDateRange, end: e.target.value});
                fetchTradeHistory();
              }}
              className="px-2 py-0.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
            />
            <button
              onClick={() => {
                setSelectedDateRange({start: '', end: ''});
                fetchTradeHistory();
              }}
              className="px-2 py-0.5 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300"
            >
              Clear
            </button>
          </div>

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
            {tradeHistory.length === 0 && (!autopilotStatus?.trade_history || autopilotStatus.trade_history.length === 0) ? (
              <div className="text-center text-gray-500 py-4">
                No {sourceFilter === 'all' ? '' : sourceFilter + ' '}trade history yet
              </div>
            ) : (
              (tradeHistory.length > 0 ? tradeHistory : autopilotStatus?.trade_history || [])
                .filter(trade => sourceFilter === 'all' || trade.source === sourceFilter)
                .slice().reverse().map((trade, idx) => (
                  <TradeHistoryRow
                    key={`${trade.symbol}-${idx}`}
                    trade={trade}
                    expanded={expandedTrade === `${trade.symbol}-${idx}`}
                    onToggle={() => setExpandedTrade(expandedTrade === `${trade.symbol}-${idx}` ? null : `${trade.symbol}-${idx}`)}
                  />
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

          {/* LLM Switches Tracking */}
          <div className="mt-4 pt-4 border-t border-gray-700">
            <div className="flex items-center justify-between mb-2">
              <div className="text-xs text-gray-400 flex items-center gap-1">
                <Sparkles className="w-3.5 h-3.5" /> LLM Switches
              </div>
              <button
                onClick={handleResetLLMDiagnostics}
                className="px-2 py-0.5 bg-red-900/30 hover:bg-red-900/50 text-red-400 rounded text-[10px] transition-colors"
              >
                Reset
              </button>
            </div>

            {llmSwitches.length === 0 ? (
              <p className="text-center text-gray-500 py-2 text-xs">No LLM switches recorded</p>
            ) : (
              <div className="space-y-1 max-h-24 overflow-y-auto">
                {llmSwitches.slice(-20).reverse().map((sw, idx) => (
                  <div key={idx} className="bg-gray-700/30 rounded p-1.5 text-xs">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-1">
                        <span className="text-gray-500">{new Date(sw.timestamp).toLocaleTimeString()}</span>
                        <span className="text-white font-medium">{sw.symbol}</span>
                        <span className={`px-1 py-0.5 rounded text-[9px] ${
                          sw.action === 'enable' ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400'
                        }`}>
                          {sw.action.toUpperCase()}
                        </span>
                      </div>
                      <span className="text-gray-400 text-[9px]">{sw.reason}</span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Performance Tab */}
      {activeTab === 'performance' && (
        <div className="space-y-2">
          {/* Reload Button */}
          <button
            onClick={() => {
              fetchPerformanceMetrics();
            }}
            disabled={loadingPerformance}
            className="w-full px-2 py-1 bg-green-900/30 hover:bg-green-900/50 disabled:bg-gray-700 text-green-400 disabled:text-gray-500 rounded text-xs transition-colors"
          >
            {loadingPerformance ? 'Loading...' : 'Refresh Performance Data'}
          </button>

          {/* Performance Summary */}
          {performanceMetrics && (
            <div className="max-h-96 overflow-y-auto space-y-2">
              {/* Overall Stats */}
              <div className="grid grid-cols-4 gap-2">
                <div className="bg-gray-700/30 rounded p-2">
                  <div className="text-[10px] text-gray-400">Total PnL</div>
                  <div className={`text-sm font-bold ${(performanceMetrics.total_pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatUSD(performanceMetrics.total_pnl || 0)}
                  </div>
                </div>
                <div className="bg-gray-700/30 rounded p-2">
                  <div className="text-[10px] text-gray-400">Total Trades</div>
                  <div className="text-sm font-bold text-white">{performanceMetrics.total_trades || 0}</div>
                </div>
                <div className="bg-gray-700/30 rounded p-2">
                  <div className="text-[10px] text-gray-400">Win Rate</div>
                  <div className="text-sm font-bold text-green-400">
                    {performanceMetrics.total_trades > 0
                      ? ((performanceMetrics.winning_trades || 0) / performanceMetrics.total_trades * 100).toFixed(1)
                      : 0}%
                  </div>
                </div>
                <div className="bg-gray-700/30 rounded p-2">
                  <div className="text-[10px] text-gray-400">Avg Win/Loss</div>
                  <div className="text-sm font-bold text-blue-400">
                    {((performanceMetrics.total_pnl || 0) / Math.max(performanceMetrics.total_trades || 1, 1)).toFixed(2)}
                  </div>
                </div>
              </div>

              {/* Per-Coin Performance */}
              {performanceMetrics.coin_metrics && Object.entries(performanceMetrics.coin_metrics).length > 0 && (
                <div className="border-t border-gray-700 pt-2">
                  <div className="text-xs font-medium text-gray-400 mb-2">Per-Coin Performance</div>
                  <div className="space-y-1">
                    {Object.entries(performanceMetrics.coin_metrics)
                      .sort((a: any, b: any) => (b[1].total_pnl || 0) - (a[1].total_pnl || 0))
                      .slice(0, 10)
                      .map(([coin, metrics]: [string, any]) => (
                        <div key={coin} className="bg-gray-700/30 rounded p-2 text-xs">
                          <div className="flex items-center justify-between mb-1">
                            <span className="text-white font-medium">{coin}</span>
                            <span className={`font-bold ${(metrics.total_pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                              {formatUSD(metrics.total_pnl || 0)}
                            </span>
                          </div>
                          <div className="grid grid-cols-4 gap-2 text-[10px] text-gray-400">
                            <div>
                              <span className="text-gray-500">Trades:</span> {metrics.total_trades || 0}
                            </div>
                            <div>
                              <span className="text-gray-500">Wins:</span> {metrics.winning_trades || 0}
                            </div>
                            <div>
                              <span className="text-gray-500">Rate:</span> {metrics.total_trades > 0 ? ((metrics.winning_trades || 0) / metrics.total_trades * 100).toFixed(0) : 0}%
                            </div>
                            <div>
                              <span className="text-gray-500">Avg:</span> {((metrics.total_pnl || 0) / Math.max(metrics.total_trades || 1, 1)).toFixed(2)}
                            </div>
                          </div>
                        </div>
                      ))}
                  </div>
                </div>
              )}

              {/* SymbolPerformancePanel as fallback */}
              <div className="border-t border-gray-700 pt-2">
                <SymbolPerformancePanel />
              </div>
            </div>
          )}

          {!performanceMetrics && (
            <div className="text-center text-gray-500 py-4 text-xs">
              Click "Refresh Performance Data" to load metrics
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Position Card Component
function PositionCard({ position, expanded, onToggle }: { position: GiniePosition; expanded: boolean; onToggle: () => void }) {
  const pnlTotal = position.realized_pnl + position.unrealized_pnl;
  const pnlPercent = ((position.remaining_qty > 0 ? position.unrealized_pnl : 0) / (position.entry_price * position.original_qty)) * 100;
  const [editingROI, setEditingROI] = useState(false);
  const [roiValue, setRoiValue] = useState((position as any).custom_roi_percent?.toString() || '');
  const [savingROI, setSavingROI] = useState(false);

  const handleSaveROI = async () => {
    if (!roiValue) {
      setEditingROI(false);
      return;
    }

    const roiPercent = parseFloat(roiValue);
    if (isNaN(roiPercent) || roiPercent < 0 || roiPercent > 1000) {
      alert('ROI % must be between 0-1000');
      return;
    }

    setSavingROI(true);
    try {
      await futuresApi.setPositionROITarget(position.symbol, roiPercent, false);
      setEditingROI(false);
      setRoiValue('');
    } catch (err) {
      console.error('Error saving ROI target:', err);
      alert('Failed to save ROI target');
    } finally {
      setSavingROI(false);
    }
  };

  return (
    <div className="bg-gray-700/30 rounded p-2">
      <div className="flex items-center justify-between cursor-pointer" onClick={onToggle}>
        <div className="flex items-center gap-2">
          <span className="text-white font-medium">{position.symbol.replace('USDT', '')}</span>
          <span className={`text-xs font-bold ${position.side === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
            {position.side}
          </span>
          <span className={`text-xs uppercase font-bold ${
            position.mode === 'scalp' ? 'text-yellow-400' :
            position.mode === 'swing' ? 'text-blue-400' :
            'text-purple-400'
          }`}>{position.mode.slice(0, 3).toUpperCase()}</span>
          {/* Source Badge */}
          <span className={`px-1 py-0.5 rounded text-xs ${
            position.source === 'strategy' ? 'bg-purple-900/50 text-purple-400' : 'bg-blue-900/50 text-blue-400'
          }`}>
            {position.source === 'strategy' ? position.strategy_name || 'Strategy' : 'AI'}
          </span>
          {position.trailing_active && (
            <span className="px-1 py-0.5 bg-blue-900/50 text-blue-400 rounded text-xs">TRAIL</span>
          )}
          {/* ROI Target Badge */}
          {(position as any).custom_roi_percent && (
            <span className="px-1 py-0.5 bg-yellow-900/50 text-yellow-400 rounded text-xs font-bold">
              ROI: {((position as any).custom_roi_percent).toFixed(2)}%
            </span>
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
          <div className="grid grid-cols-5 gap-2">
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
            <div className="bg-gray-700/50 p-1.5 rounded">
              <div className="text-gray-500">🎯 ROI Target</div>
              {editingROI ? (
                <div className="space-y-1">
                  <input
                    type="number"
                    value={roiValue}
                    onChange={(e) => setRoiValue(e.target.value)}
                    placeholder="ROI %"
                    step="0.1"
                    min="0"
                    max="1000"
                    className="w-full bg-gray-600 text-white rounded px-1 py-0.5 text-xs border border-gray-500"
                    autoFocus
                  />
                  <div className="flex gap-1">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleSaveROI();
                      }}
                      disabled={savingROI}
                      className="flex-1 bg-green-600 hover:bg-green-700 text-white px-1 py-0.5 rounded text-xs disabled:opacity-50"
                    >
                      {savingROI ? 'Saving...' : 'Save'}
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setEditingROI(false);
                        setRoiValue('');
                      }}
                      className="flex-1 bg-gray-600 hover:bg-gray-700 text-white px-1 py-0.5 rounded text-xs"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              ) : (
                <div
                  onClick={(e) => {
                    e.stopPropagation();
                    setRoiValue((position as any).custom_roi_percent?.toString() || '');
                    setEditingROI(true);
                  }}
                  className="cursor-pointer hover:text-yellow-300 transition"
                >
                  {(position as any).custom_roi_percent ? (
                    <span className="text-yellow-400 font-bold">{((position as any).custom_roi_percent).toFixed(2)}%</span>
                  ) : (
                    <span className="text-gray-400">-</span>
                  )}
                </div>
              )}
            </div>
          </div>

          {/* TP Levels with Progression */}
          <div className="space-y-2">
            <div className="text-gray-500 text-xs font-medium">Take Profit Progression</div>
            <div className="flex items-center gap-1 flex-wrap">
              {position.take_profits.map((tp, idx) => {
                const isHit = tp.status === 'hit';
                const isActive = position.current_tp_level === tp.level;
                const isNext = position.current_tp_level + 1 === tp.level;

                return (
                  <div key={tp.level} className="flex items-center gap-1">
                    {/* TP Box */}
                    <div
                      className={`px-2 py-1.5 rounded text-xs font-bold flex items-center gap-1 transition-colors ${
                        isHit
                          ? 'bg-green-900/60 text-green-300 ring-1 ring-green-600'
                          : isNext
                            ? 'bg-yellow-900/60 text-yellow-300 ring-1 ring-yellow-600'
                            : 'bg-gray-700/40 text-gray-400'
                      }`}
                    >
                      <span>TP{tp.level}</span>
                      {isHit && <CheckCircle className="w-3 h-3" />}
                      {isNext && !isHit && <AlertCircle className="w-3 h-3 animate-pulse" />}
                    </div>

                    {/* Arrow between TPs */}
                    {idx < position.take_profits.length - 1 && (
                      <div
                        className={`text-xs font-bold ${
                          isHit ? 'text-green-400' : isActive || isNext ? 'text-yellow-400' : 'text-gray-600'
                        }`}
                      >
                        →
                      </div>
                    )}
                  </div>
                );
              })}
            </div>

            {/* TP Details Row */}
            <div className="grid grid-cols-4 gap-2 mt-2">
              {position.take_profits.map((tp) => (
                <div
                  key={tp.level}
                  className={`text-xs p-1.5 rounded text-center ${
                    tp.status === 'hit'
                      ? 'bg-green-900/30 text-green-300'
                      : position.current_tp_level + 1 === tp.level
                        ? 'bg-yellow-900/30 text-yellow-300'
                        : 'bg-gray-700/30 text-gray-400'
                  }`}
                >
                  <div className="font-bold">TP{tp.level}</div>
                  <div className="text-[10px] text-gray-400">${Number(tp.price || 0).toFixed(2)}</div>
                  <div className="text-[10px] text-gray-500">{tp.percent}%</div>
                </div>
              ))}
            </div>
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
function TradeHistoryRow({ trade, expanded = false, onToggle }: { trade: GinieTradeResult; expanded?: boolean; onToggle?: () => void }) {
  const time = new Date(trade.timestamp).toLocaleTimeString();
  const date = new Date(trade.timestamp).toLocaleDateString();

  return (
    <div className="bg-gray-700/30 rounded">
      <div
        className="flex items-center justify-between p-2 cursor-pointer hover:bg-gray-700/50 transition-colors"
        onClick={onToggle}
      >
        <div className="flex items-center gap-2">
          <span className="text-gray-500 text-xs">{date} {time}</span>
          <span className="text-white font-medium">{trade.symbol.replace('USDT', '')}</span>
          <span className={`font-bold text-xs ${trade.side === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
            {trade.side}
          </span>
          <span className="text-gray-400 text-xs">{trade.action}</span>
          {trade.source && (
            <span className={`px-1 py-0.5 rounded text-xs ${
              trade.source === 'strategy' ? 'bg-purple-900/30 text-purple-400' : 'bg-blue-900/30 text-blue-400'
            }`}>
              {trade.source === 'strategy' ? trade.strategy_name || 'Strategy' : 'AI'}
            </span>
          )}
          {(trade as any).mode && (
            <span className={`px-1 py-0.5 rounded text-xs font-bold ${
              (trade as any).mode === 'scalp' ? 'text-yellow-400' :
              (trade as any).mode === 'swing' ? 'text-blue-400' :
              'text-purple-400'
            }`}>
              {((trade as any).mode || '').slice(0, 3).toUpperCase()}
            </span>
          )}
          {trade.tp_level && trade.tp_level > 0 && (
            <span className="px-1 py-0.5 bg-green-900/30 text-green-400 rounded text-xs">TP{trade.tp_level}</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="text-gray-400 text-xs">{Number(trade.quantity || 0).toFixed(4)}</span>
          <span className={`font-bold text-xs ${(trade.pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {formatUSD(trade.pnl || 0)}
          </span>
          {onToggle && (expanded ? <ChevronUp className="w-3 h-3 text-gray-400" /> : <ChevronDown className="w-3 h-3 text-gray-400" />)}
        </div>
      </div>

      {expanded && (
        <div className="border-t border-gray-600 p-2 bg-gray-800/50 space-y-2 text-xs">
          <div className="grid grid-cols-2 gap-2">
            <div>
              <span className="text-gray-500">Entry Price:</span>
              <div className="text-white font-mono">{Number((trade as any).entry_price || 0).toFixed(8)}</div>
            </div>
            <div>
              <span className="text-gray-500">Exit Price:</span>
              <div className="text-white font-mono">{Number((trade as any).exit_price || 0).toFixed(8)}</div>
            </div>
            <div>
              <span className="text-gray-500">Entry Time:</span>
              <div className="text-white">{new Date((trade as any).entry_time || 0).toLocaleString()}</div>
            </div>
            <div>
              <span className="text-gray-500">Exit Time:</span>
              <div className="text-white">{new Date(trade.timestamp).toLocaleString()}</div>
            </div>
          </div>
          {(trade as any).decision_details && (
            <div className="mt-2 p-2 bg-gray-900/50 rounded border border-gray-700">
              <span className="text-gray-400">Decision Details:</span>
              <div className="text-gray-300 mt-1 whitespace-pre-wrap text-xs max-h-32 overflow-y-auto">
                {typeof (trade as any).decision_details === 'string'
                  ? (trade as any).decision_details
                  : JSON.stringify((trade as any).decision_details, null, 2)}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
