import { useEffect, useState } from 'react';
import { futuresApi, formatUSD, formatPercent } from '../services/futuresApi';
import { Bot, Power, PowerOff, Settings, AlertTriangle, CheckCircle, RefreshCw, TrendingUp, DollarSign, Shield, Percent, Target, Check, RotateCcw, Save, X, Edit2, Zap, Clock, Activity } from 'lucide-react';

interface AutopilotStatus {
  enabled: boolean;
  running: boolean;
  dry_run: boolean;
  risk_level?: string;
  daily_trades?: number;
  daily_pnl?: number;
  max_usd_allocation?: number;
  total_usd_allocated?: number;
  profit_reinvest_percent?: number;
  profit_reinvest_risk_level?: string;
  active_positions?: Array<{
    symbol: string;
    side: string;
    entry_price: number;
    quantity: number;
    leverage: number;
  }>;
  config?: {
    default_leverage: number;
    max_leverage: number;
    margin_type: string;
    take_profit: number;
    stop_loss: number;
    min_confidence: number;
    allow_shorts: boolean;
    trailing_stop: boolean;
  };
  message?: string;
}

interface ProfitStats {
  total_profit: number;
  profit_pool: number;
  total_usd_allocated: number;
  max_usd_allocation: number;
  profit_reinvest_percent: number;
  profit_reinvest_risk_level: string;
  daily_pnl: number;
}

interface CircuitBreakerStatus {
  available: boolean;
  enabled: boolean;
  state: string;
  can_trade: boolean;
  block_reason: string;
  consecutive_losses: number;
  hourly_loss: number;
  daily_loss: number;
  trades_last_minute: number;
  daily_trades: number;
  trip_reason: string;
  config: {
    enabled: boolean;
    max_loss_per_hour: number;
    max_daily_loss: number;
    max_consecutive_losses: number;
    cooldown_minutes: number;
    max_trades_per_minute: number;
    max_daily_trades: number;
  };
  message?: string;
}

interface CircuitBreakerConfig {
  max_loss_per_hour: number;
  max_daily_loss: number;
  max_consecutive_losses: number;
  cooldown_minutes: number;
  max_daily_trades: number;
}

interface DynamicSLTPConfig {
  enabled: boolean;
  atr_period: number;
  atr_multiplier_sl: number;
  atr_multiplier_tp: number;
  llm_weight: number;
  min_sl_percent: number;
  max_sl_percent: number;
  min_tp_percent: number;
  max_tp_percent: number;
}

interface ScalpingConfig {
  enabled: boolean;
  min_profit: number;
  quick_reentry: boolean;
  reentry_delay_sec: number;
  max_trades_per_day: number;
  trades_today: number;
}

export default function FuturesAutopilotPanel() {
  const [status, setStatus] = useState<AutopilotStatus | null>(null);
  const [profitStats, setProfitStats] = useState<ProfitStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  // Circuit Breaker state
  const [circuitStatus, setCircuitStatus] = useState<CircuitBreakerStatus | null>(null);
  const [showLossControl, setShowLossControl] = useState(false);
  const [isEditingCB, setIsEditingCB] = useState(false);
  const [isSavingCB, setIsSavingCB] = useState(false);
  const [isResettingCB, setIsResettingCB] = useState(false);
  const [cbConfig, setCbConfig] = useState<CircuitBreakerConfig>({
    max_loss_per_hour: 3,
    max_daily_loss: 5,
    max_consecutive_losses: 5,
    cooldown_minutes: 30,
    max_daily_trades: 100,
  });

  // Dynamic SL/TP state
  const [showDynamicSLTP, setShowDynamicSLTP] = useState(false);
  const [dynamicSLTPConfig, setDynamicSLTPConfig] = useState<DynamicSLTPConfig>({
    enabled: false,
    atr_period: 14,
    atr_multiplier_sl: 1.5,
    atr_multiplier_tp: 2.0,
    llm_weight: 0.3,
    min_sl_percent: 0.3,
    max_sl_percent: 3.0,
    min_tp_percent: 0.5,
    max_tp_percent: 5.0,
  });
  const [isSavingDynamicSLTP, setIsSavingDynamicSLTP] = useState(false);

  // Scalping mode state
  const [showScalping, setShowScalping] = useState(false);
  const [scalpingConfig, setScalpingConfig] = useState<ScalpingConfig>({
    enabled: false,
    min_profit: 0.2,
    quick_reentry: true,
    reentry_delay_sec: 5,
    max_trades_per_day: 0,
    trades_today: 0,
  });
  const [isSavingScalping, setIsSavingScalping] = useState(false);

  // Setting inputs
  const [riskLevel, setRiskLevel] = useState('moderate');
  const [maxAllocation, setMaxAllocation] = useState('2500');
  const [profitReinvestPercent, setProfitReinvestPercent] = useState('50');
  const [profitRiskLevel, setProfitRiskLevel] = useState('aggressive');
  const [takeProfitPercent, setTakeProfitPercent] = useState('2');
  const [stopLossPercent, setStopLossPercent] = useState('1');
  const [defaultLeverage, setDefaultLeverage] = useState('5');
  const [maxLeverage, setMaxLeverage] = useState(20);
  const [minConfidence, setMinConfidence] = useState('65');

  // Track if user is editing to prevent auto-refresh from overwriting
  const [isEditing, setIsEditing] = useState(false);
  const [initialLoadDone, setInitialLoadDone] = useState(false);

  const fetchStatus = async () => {
    try {
      const data = await futuresApi.getAutopilotStatus();
      setStatus(data);
      setRiskLevel(data.risk_level || 'moderate');
      // Only update inputs if user is not editing and this is initial load
      if (data.config && !isEditing) {
        if (!initialLoadDone) {
          setTakeProfitPercent(data.config.take_profit.toString());
          setStopLossPercent(data.config.stop_loss.toString());
          setDefaultLeverage(data.config.default_leverage.toString());
          setMinConfidence((data.config.min_confidence * 100).toString());
          setInitialLoadDone(true);
        }
        setMaxLeverage(data.config.max_leverage || 20);
      }
      setError(null);
    } catch (err) {
      setError('Failed to fetch autopilot status');
      console.error(err);
    }
  };

  const fetchProfitStats = async () => {
    try {
      const stats = await futuresApi.getAutopilotProfitStats();
      setProfitStats(stats);
      setMaxAllocation(stats.max_usd_allocation.toString());
      setProfitReinvestPercent(stats.profit_reinvest_percent.toString());
      setProfitRiskLevel(stats.profit_reinvest_risk_level || 'aggressive');
    } catch (err) {
      console.error('Failed to fetch profit stats:', err);
    }
  };

  const fetchCircuitBreakerStatus = async () => {
    try {
      const cbStatus = await futuresApi.getCircuitBreakerStatus();
      setCircuitStatus(cbStatus);
      if (cbStatus.config) {
        setCbConfig({
          max_loss_per_hour: cbStatus.config.max_loss_per_hour,
          max_daily_loss: cbStatus.config.max_daily_loss,
          max_consecutive_losses: cbStatus.config.max_consecutive_losses,
          cooldown_minutes: cbStatus.config.cooldown_minutes,
          max_daily_trades: cbStatus.config.max_daily_trades,
        });
      }
    } catch (err) {
      console.error('Failed to fetch circuit breaker status:', err);
    }
  };

  const fetchDynamicSLTPConfig = async () => {
    try {
      const config = await futuresApi.getDynamicSLTPConfig();
      setDynamicSLTPConfig(config);
    } catch (err) {
      console.error('Failed to fetch dynamic SL/TP config:', err);
    }
  };

  const fetchScalpingConfig = async () => {
    try {
      const config = await futuresApi.getScalpingConfig();
      setScalpingConfig(config);
    } catch (err) {
      console.error('Failed to fetch scalping config:', err);
    }
  };

  useEffect(() => {
    fetchStatus();
    fetchProfitStats();
    fetchCircuitBreakerStatus();
    fetchDynamicSLTPConfig();
    fetchScalpingConfig();
    const interval = setInterval(() => {
      fetchStatus();
      fetchProfitStats();
      fetchCircuitBreakerStatus();
      fetchDynamicSLTPConfig();
      fetchScalpingConfig();
    }, 15000); // Reduced from 5s to 15s to avoid rate limits
    return () => clearInterval(interval);
  }, []);

  const handleToggle = async () => {
    if (!status) return;
    setLoading(true);
    try {
      const result = await futuresApi.toggleAutopilot(!status.running);
      if (result.success) {
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to toggle autopilot');
    } finally {
      setLoading(false);
    }
  };

  const handleDryRunToggle = async () => {
    if (!status) return;
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotDryRun(!status.dry_run);
      if (result.success) {
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to update mode');
    } finally {
      setLoading(false);
    }
  };

  const handleRiskLevelChange = async (level: string) => {
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotRiskLevel(level);
      if (result.success) {
        setRiskLevel(level);
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to update risk level');
    } finally {
      setLoading(false);
    }
  };

  const handleAllocationChange = async () => {
    const value = parseFloat(maxAllocation);
    if (isNaN(value) || value <= 0) {
      setError('Invalid allocation amount');
      return;
    }
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotAllocation(value);
      if (result.success) {
        fetchProfitStats();
      }
    } catch (err) {
      setError('Failed to update allocation');
    } finally {
      setLoading(false);
    }
  };

  const handleProfitReinvestChange = async () => {
    const percent = parseFloat(profitReinvestPercent);
    if (isNaN(percent) || percent < 0 || percent > 100) {
      setError('Invalid reinvest percentage (0-100)');
      return;
    }
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotProfitReinvest(percent, profitRiskLevel);
      if (result.success) {
        fetchProfitStats();
      }
    } catch (err) {
      setError('Failed to update profit reinvestment');
    } finally {
      setLoading(false);
    }
  };

  const handleTPSLChange = async () => {
    const tp = parseFloat(takeProfitPercent);
    const sl = parseFloat(stopLossPercent);
    if (isNaN(tp) || tp <= 0 || tp > 100) {
      setError('Invalid take profit percentage (0.1-100)');
      return;
    }
    if (isNaN(sl) || sl <= 0 || sl > 100) {
      setError('Invalid stop loss percentage (0.1-100)');
      return;
    }
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotTPSL(tp, sl);
      if (result.success) {
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to update TP/SL percentages');
    } finally {
      setLoading(false);
    }
  };

  const handleLeverageChange = async () => {
    const leverage = parseInt(defaultLeverage);
    if (isNaN(leverage) || leverage < 1 || leverage > maxLeverage) {
      setError(`Invalid leverage (1-${maxLeverage})`);
      return;
    }
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotLeverage(leverage);
      if (result.success) {
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to update leverage');
    } finally {
      setLoading(false);
    }
  };

  const handleMinConfidenceChange = async () => {
    const confidence = parseFloat(minConfidence);
    if (isNaN(confidence) || confidence < 1 || confidence > 100) {
      setError('Invalid confidence percentage (1-100)');
      return;
    }
    setLoading(true);
    try {
      const result = await futuresApi.setAutopilotMinConfidence(confidence / 100);
      if (result.success) {
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to update min confidence');
    } finally {
      setLoading(false);
    }
  };

  // Circuit Breaker handlers
  const handleResetCircuitBreaker = async () => {
    setIsResettingCB(true);
    try {
      const result = await futuresApi.resetCircuitBreaker();
      if (result.success) {
        setSuccessMsg('Circuit breaker reset successfully');
        setTimeout(() => setSuccessMsg(null), 3000);
        fetchCircuitBreakerStatus();
      }
    } catch (err) {
      setError('Failed to reset circuit breaker');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsResettingCB(false);
    }
  };

  const handleSaveCBConfig = async () => {
    setIsSavingCB(true);
    try {
      const result = await futuresApi.updateCircuitBreakerConfig({
        max_loss_per_hour: cbConfig.max_loss_per_hour,
        max_daily_loss: cbConfig.max_daily_loss,
        max_consecutive_losses: cbConfig.max_consecutive_losses,
        cooldown_minutes: cbConfig.cooldown_minutes,
        max_daily_trades: cbConfig.max_daily_trades,
      });
      if (result.success) {
        setSuccessMsg('Loss limits updated successfully');
        setTimeout(() => setSuccessMsg(null), 3000);
        setIsEditingCB(false);
        fetchCircuitBreakerStatus();
      }
    } catch (err) {
      setError('Failed to update loss limits');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsSavingCB(false);
    }
  };

  const handleCancelCBEdit = () => {
    if (circuitStatus?.config) {
      setCbConfig({
        max_loss_per_hour: circuitStatus.config.max_loss_per_hour,
        max_daily_loss: circuitStatus.config.max_daily_loss,
        max_consecutive_losses: circuitStatus.config.max_consecutive_losses,
        cooldown_minutes: circuitStatus.config.cooldown_minutes,
        max_daily_trades: circuitStatus.config.max_daily_trades,
      });
    }
    setIsEditingCB(false);
  };

  const handleToggleCircuitBreaker = async () => {
    if (!circuitStatus) return;
    try {
      const result = await futuresApi.toggleCircuitBreaker(!circuitStatus.enabled);
      if (result.success) {
        setSuccessMsg(`Circuit breaker ${!circuitStatus.enabled ? 'enabled' : 'disabled'}`);
        setTimeout(() => setSuccessMsg(null), 3000);
        fetchCircuitBreakerStatus();
      }
    } catch (err) {
      setError('Failed to toggle circuit breaker');
      setTimeout(() => setError(null), 3000);
    }
  };

  // Dynamic SL/TP handlers
  const handleSaveDynamicSLTP = async () => {
    setIsSavingDynamicSLTP(true);
    try {
      const result = await futuresApi.setDynamicSLTPConfig(dynamicSLTPConfig);
      if (result.success) {
        setSuccessMsg('Dynamic SL/TP settings saved');
        setTimeout(() => setSuccessMsg(null), 3000);
        fetchDynamicSLTPConfig();
      }
    } catch (err) {
      setError('Failed to save Dynamic SL/TP settings');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsSavingDynamicSLTP(false);
    }
  };

  const handleToggleDynamicSLTP = async () => {
    const newConfig = { ...dynamicSLTPConfig, enabled: !dynamicSLTPConfig.enabled };
    setDynamicSLTPConfig(newConfig);
    try {
      const result = await futuresApi.setDynamicSLTPConfig(newConfig);
      if (result.success) {
        setSuccessMsg(`Dynamic SL/TP ${newConfig.enabled ? 'enabled' : 'disabled'}`);
        setTimeout(() => setSuccessMsg(null), 3000);
      }
    } catch (err) {
      setError('Failed to toggle Dynamic SL/TP');
      setDynamicSLTPConfig(dynamicSLTPConfig); // Revert
      setTimeout(() => setError(null), 3000);
    }
  };

  // Scalping mode handlers
  const handleSaveScalping = async () => {
    setIsSavingScalping(true);
    try {
      const result = await futuresApi.setScalpingConfig(scalpingConfig);
      if (result.success) {
        setSuccessMsg('Scalping mode settings saved');
        setTimeout(() => setSuccessMsg(null), 3000);
        fetchScalpingConfig();
      }
    } catch (err) {
      setError('Failed to save scalping settings');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsSavingScalping(false);
    }
  };

  const handleToggleScalping = async () => {
    const newConfig = { ...scalpingConfig, enabled: !scalpingConfig.enabled };
    setScalpingConfig(newConfig);
    try {
      const result = await futuresApi.setScalpingConfig(newConfig);
      if (result.success) {
        setSuccessMsg(`Scalping mode ${newConfig.enabled ? 'enabled' : 'disabled'}`);
        setTimeout(() => setSuccessMsg(null), 3000);
      }
    } catch (err) {
      setError('Failed to toggle scalping mode');
      setScalpingConfig(scalpingConfig); // Revert
      setTimeout(() => setError(null), 3000);
    }
  };

  const getRiskLevelColor = (level: string) => {
    switch (level) {
      case 'conservative': return 'text-blue-500 bg-blue-500/20';
      case 'moderate': return 'text-yellow-500 bg-yellow-500/20';
      case 'aggressive': return 'text-red-500 bg-red-500/20';
      default: return 'text-gray-500 bg-gray-500/20';
    }
  };

  // Helper functions for circuit breaker UI
  const getCBStateColor = (state: string) => {
    switch (state) {
      case 'closed': return 'text-green-500';
      case 'half_open': return 'text-yellow-500';
      case 'open': return 'text-red-500';
      default: return 'text-gray-500';
    }
  };

  const getCBStateLabel = (state: string) => {
    switch (state) {
      case 'closed': return 'Active';
      case 'half_open': return 'Testing';
      case 'open': return 'Tripped';
      default: return state;
    }
  };

  const getProgressColor = (current: number, max: number) => {
    const percent = (current / max) * 100;
    if (percent >= 80) return 'bg-red-500';
    if (percent >= 50) return 'bg-yellow-500';
    return 'bg-green-500';
  };

  if (!status) {
    return (
      <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
        <div className="flex items-center gap-2 text-gray-400">
          <Bot className="w-5 h-5" />
          <span>AI Autopilot</span>
        </div>
        <div className="mt-2 text-sm text-gray-500">Loading...</div>
      </div>
    );
  }

  if (status.message && !status.config) {
    return (
      <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
        <div className="flex items-center gap-2 text-gray-400">
          <Bot className="w-5 h-5" />
          <span>AI Autopilot</span>
        </div>
        <div className="mt-2 text-sm text-gray-500">{status.message}</div>
      </div>
    );
  }

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Bot className="w-5 h-5 text-purple-500" />
          <span className="font-semibold">AI Autopilot</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowScalping(!showScalping)}
            className={`p-1 hover:bg-gray-700 rounded ${showScalping ? 'bg-gray-700' : ''}`}
            title="Scalping Mode"
          >
            <Zap className={`w-4 h-4 ${scalpingConfig.enabled ? 'text-yellow-500' : 'text-gray-400'}`} />
          </button>
          <button
            onClick={() => setShowDynamicSLTP(!showDynamicSLTP)}
            className={`p-1 hover:bg-gray-700 rounded ${showDynamicSLTP ? 'bg-gray-700' : ''}`}
            title="Dynamic SL/TP"
          >
            <Activity className={`w-4 h-4 ${dynamicSLTPConfig.enabled ? 'text-cyan-500' : 'text-gray-400'}`} />
          </button>
          <button
            onClick={() => setShowLossControl(!showLossControl)}
            className={`p-1 hover:bg-gray-700 rounded ${showLossControl ? 'bg-gray-700' : ''}`}
            title="Loss Control"
          >
            <Shield className={`w-4 h-4 ${circuitStatus?.state === 'open' ? 'text-red-500' : 'text-gray-400'}`} />
          </button>
          <button
            onClick={() => setShowSettings(!showSettings)}
            className={`p-1 hover:bg-gray-700 rounded ${showSettings ? 'bg-gray-700' : ''}`}
            title="Settings"
          >
            <Settings className="w-4 h-4 text-gray-400" />
          </button>
          <button
            onClick={() => { fetchStatus(); fetchProfitStats(); fetchCircuitBreakerStatus(); fetchDynamicSLTPConfig(); fetchScalpingConfig(); }}
            className="p-1 hover:bg-gray-700 rounded"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4 text-gray-400" />
          </button>
        </div>
      </div>

      {/* Status */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          {status.running ? (
            <>
              <CheckCircle className="w-4 h-4 text-green-500" />
              <span className="text-green-500 text-sm">Running</span>
            </>
          ) : (
            <>
              <PowerOff className="w-4 h-4 text-gray-500" />
              <span className="text-gray-500 text-sm">Stopped</span>
            </>
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className={`px-2 py-0.5 rounded text-xs capitalize ${getRiskLevelColor(riskLevel)}`}>
            {riskLevel}
          </span>
          <span className={`px-2 py-0.5 rounded text-xs ${status.dry_run ? 'bg-yellow-500/20 text-yellow-500' : 'bg-red-500/20 text-red-500'}`}>
            {status.dry_run ? 'PAPER' : 'LIVE'}
          </span>
        </div>
      </div>

      {error && (
        <div className="mb-4 p-2 bg-red-500/10 border border-red-500/30 rounded text-red-500 text-xs flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="hover:text-red-400">×</button>
        </div>
      )}

      {successMsg && (
        <div className="mb-4 p-2 bg-green-500/10 border border-green-500/30 rounded text-green-500 text-xs flex items-center gap-2">
          <Check className="w-4 h-4" />
          <span>{successMsg}</span>
        </div>
      )}

      {/* Profit Stats */}
      {profitStats && (
        <div className="mb-4 p-3 bg-gray-800 rounded-lg">
          <div className="flex items-center gap-2 mb-2 text-sm text-gray-400">
            <TrendingUp className="w-4 h-4" />
            <span>Profit Stats</span>
          </div>
          <div className="grid grid-cols-2 gap-3 text-xs">
            <div>
              <div className="text-gray-500">Total Profit</div>
              <div className={`font-semibold ${profitStats.total_profit >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                {formatUSD(profitStats.total_profit)}
              </div>
            </div>
            <div>
              <div className="text-gray-500">Profit Pool</div>
              <div className="font-semibold text-purple-500">
                {formatUSD(profitStats.profit_pool)}
              </div>
            </div>
            <div>
              <div className="text-gray-500">USD Allocated</div>
              <div className="font-semibold text-yellow-500">
                {formatUSD(profitStats.total_usd_allocated)} / {formatUSD(profitStats.max_usd_allocation)}
              </div>
            </div>
            <div>
              <div className="text-gray-500">Daily PnL</div>
              <div className={`font-semibold ${profitStats.daily_pnl >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                {formatUSD(profitStats.daily_pnl)}
              </div>
            </div>
          </div>
          {profitStats.profit_pool > 0 && (
            <div className="mt-2 pt-2 border-t border-gray-700 text-xs text-gray-400">
              <span className="text-purple-400">{profitStats.profit_reinvest_percent}%</span> of profits ({formatUSD(profitStats.profit_pool)}) trading at{' '}
              <span className={getRiskLevelColor(profitStats.profit_reinvest_risk_level).replace('bg-', '').split(' ')[0]}>
                {profitStats.profit_reinvest_risk_level}
              </span> risk
            </div>
          )}
        </div>
      )}

      {/* Controls */}
      <div className="space-y-3">
        {/* Power Toggle */}
        <button
          onClick={handleToggle}
          disabled={loading}
          className={`w-full flex items-center justify-center gap-2 py-2 px-4 rounded-lg font-medium transition-colors ${
            status.running
              ? 'bg-red-500/20 text-red-500 hover:bg-red-500/30'
              : 'bg-green-500/20 text-green-500 hover:bg-green-500/30'
          } ${loading ? 'opacity-50 cursor-not-allowed' : ''}`}
        >
          {status.running ? (
            <>
              <PowerOff className="w-4 h-4" />
              Stop Autopilot
            </>
          ) : (
            <>
              <Power className="w-4 h-4" />
              Start Autopilot
            </>
          )}
        </button>

        {/* Mode Toggle */}
        <button
          onClick={handleDryRunToggle}
          disabled={loading || status.running}
          className={`w-full flex items-center justify-center gap-2 py-2 px-4 rounded-lg font-medium transition-colors ${
            status.dry_run
              ? 'bg-yellow-500/20 text-yellow-500 hover:bg-yellow-500/30'
              : 'bg-red-500/20 text-red-500 hover:bg-red-500/30'
          } ${loading || status.running ? 'opacity-50 cursor-not-allowed' : ''}`}
          title={status.running ? 'Stop autopilot to change mode' : ''}
        >
          {status.dry_run ? 'Switch to LIVE Mode' : 'Switch to PAPER Mode'}
        </button>

        {!status.dry_run && (
          <div className="flex items-center gap-2 text-xs text-yellow-500 bg-yellow-500/10 p-2 rounded">
            <AlertTriangle className="w-4 h-4" />
            <span>Live mode - Real trades will be executed!</span>
          </div>
        )}
      </div>

      {/* Settings Panel */}
      {showSettings && (
        <div className="mt-4 pt-4 border-t border-gray-700 space-y-4">
          <div className="text-sm font-medium text-gray-400 flex items-center gap-2">
            <Settings className="w-4 h-4" />
            Autopilot Settings
          </div>

          {/* Risk Level */}
          <div>
            <label className="text-xs text-gray-500 flex items-center gap-1 mb-2">
              <Shield className="w-3 h-3" />
              Risk Level
            </label>
            <div className="grid grid-cols-3 gap-2">
              {['conservative', 'moderate', 'aggressive'].map((level) => (
                <button
                  key={level}
                  onClick={() => handleRiskLevelChange(level)}
                  disabled={loading}
                  className={`py-2 px-2 rounded text-xs font-medium capitalize transition-colors ${
                    riskLevel === level
                      ? getRiskLevelColor(level)
                      : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
                  } ${loading ? 'opacity-50' : ''}`}
                >
                  {level}
                </button>
              ))}
            </div>
            <div className="mt-1 text-xs text-gray-500">
              {riskLevel === 'conservative' && 'Low risk: 80% confidence, 3x leverage, 1.5% TP, 0.5% SL'}
              {riskLevel === 'moderate' && 'Medium risk: 65% confidence, 5x leverage, 2% TP, 1% SL'}
              {riskLevel === 'aggressive' && 'High risk: 50% confidence, 10x leverage, 3% TP, 1.5% SL'}
            </div>
          </div>

          {/* Default Leverage */}
          <div>
            <label className="text-xs text-gray-500 flex items-center gap-1 mb-2">
              <TrendingUp className="w-3 h-3" />
              Default Leverage
            </label>
            <div className="flex gap-2">
              <input
                type="number"
                value={defaultLeverage}
                onChange={(e) => setDefaultLeverage(e.target.value)}
                className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                placeholder="5"
                min="1"
                max={maxLeverage}
                step="1"
              />
              <span className="flex items-center text-gray-500 text-sm">x</span>
              <button
                onClick={handleLeverageChange}
                disabled={loading}
                className="px-4 py-2 bg-purple-500/20 text-purple-500 rounded text-sm hover:bg-purple-500/30 disabled:opacity-50"
              >
                Set
              </button>
            </div>
            <div className="mt-1 text-xs text-gray-500">
              Leverage for new autopilot trades (1-{maxLeverage}x)
            </div>
          </div>

          {/* Max USD Allocation */}
          <div>
            <label className="text-xs text-gray-500 flex items-center gap-1 mb-2">
              <DollarSign className="w-3 h-3" />
              Max USD Allocation
            </label>
            <div className="flex gap-2">
              <input
                type="number"
                value={maxAllocation}
                onChange={(e) => setMaxAllocation(e.target.value)}
                className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                placeholder="2500"
                min="0"
                step="100"
              />
              <button
                onClick={handleAllocationChange}
                disabled={loading}
                className="px-4 py-2 bg-purple-500/20 text-purple-500 rounded text-sm hover:bg-purple-500/30 disabled:opacity-50"
              >
                Set
              </button>
            </div>
            <div className="mt-1 text-xs text-gray-500">
              Maximum USD the autopilot can allocate for trading
            </div>
          </div>

          {/* Profit Reinvestment */}
          <div>
            <label className="text-xs text-gray-500 flex items-center gap-1 mb-2">
              <Percent className="w-3 h-3" />
              Profit Reinvestment
            </label>
            <div className="flex gap-2 mb-2">
              <input
                type="number"
                value={profitReinvestPercent}
                onChange={(e) => setProfitReinvestPercent(e.target.value)}
                className="w-20 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                placeholder="50"
                min="0"
                max="100"
                step="5"
              />
              <span className="flex items-center text-gray-500 text-sm">%</span>
              <select
                value={profitRiskLevel}
                onChange={(e) => setProfitRiskLevel(e.target.value)}
                className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
              >
                <option value="conservative">Conservative</option>
                <option value="moderate">Moderate</option>
                <option value="aggressive">Aggressive</option>
              </select>
              <button
                onClick={handleProfitReinvestChange}
                disabled={loading}
                className="px-4 py-2 bg-purple-500/20 text-purple-500 rounded text-sm hover:bg-purple-500/30 disabled:opacity-50"
              >
                Set
              </button>
            </div>
            <div className="text-xs text-gray-500">
              Reinvest {profitReinvestPercent}% of earned profits with {profitRiskLevel} risk
            </div>
          </div>

          {/* Take Profit / Stop Loss */}
          <div>
            <label className="text-xs text-gray-500 flex items-center gap-1 mb-2">
              <Percent className="w-3 h-3" />
              Take Profit / Stop Loss
            </label>
            <div className="flex gap-2 items-center">
              <div className="flex-1">
                <div className="text-xs text-gray-500 mb-1">TP %</div>
                <input
                  type="number"
                  value={takeProfitPercent}
                  onChange={(e) => setTakeProfitPercent(e.target.value)}
                  onFocus={() => setIsEditing(true)}
                  onBlur={() => setIsEditing(false)}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-green-500"
                  placeholder="2"
                  min="0.1"
                  max="100"
                  step="0.1"
                />
              </div>
              <div className="flex-1">
                <div className="text-xs text-gray-500 mb-1">SL %</div>
                <input
                  type="number"
                  value={stopLossPercent}
                  onChange={(e) => setStopLossPercent(e.target.value)}
                  onFocus={() => setIsEditing(true)}
                  onBlur={() => setIsEditing(false)}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-red-500"
                  placeholder="1"
                  min="0.1"
                  max="100"
                  step="0.1"
                />
              </div>
              <button
                onClick={handleTPSLChange}
                disabled={loading}
                className="px-4 py-2 bg-purple-500/20 text-purple-500 rounded text-sm hover:bg-purple-500/30 disabled:opacity-50 self-end"
              >
                Set
              </button>
            </div>
            <div className="mt-1 text-xs text-gray-500">
              Custom take profit and stop loss percentages for new trades
            </div>
          </div>

          {/* Min Confidence */}
          <div>
            <label className="text-xs text-gray-500 flex items-center gap-1 mb-2">
              <Target className="w-3 h-3" />
              Min Confidence
            </label>
            <div className="flex gap-2">
              <input
                type="number"
                value={minConfidence}
                onChange={(e) => setMinConfidence(e.target.value)}
                className="flex-1 bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-purple-500"
                placeholder="65"
                min="1"
                max="100"
                step="1"
              />
              <span className="flex items-center text-gray-500 text-sm">%</span>
              <button
                onClick={handleMinConfidenceChange}
                disabled={loading}
                className="px-4 py-2 bg-purple-500/20 text-purple-500 rounded text-sm hover:bg-purple-500/30 disabled:opacity-50"
              >
                Set
              </button>
            </div>
            <div className="mt-1 text-xs text-gray-500">
              Minimum AI confidence required to open a trade (1-100%)
            </div>
          </div>
        </div>
      )}

      {/* Dynamic SL/TP Panel */}
      {showDynamicSLTP && (
        <div className="mt-4 pt-4 border-t border-gray-700 space-y-4">
          <div className="flex items-center justify-between">
            <div className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <Activity className="w-4 h-4" />
              Dynamic SL/TP (Volatility-Based)
            </div>
            <button
              onClick={handleToggleDynamicSLTP}
              className={`px-2 py-1 rounded text-xs ${
                dynamicSLTPConfig.enabled
                  ? 'bg-cyan-500/20 text-cyan-500'
                  : 'bg-gray-700 text-gray-400'
              }`}
            >
              {dynamicSLTPConfig.enabled ? 'ON' : 'OFF'}
            </button>
          </div>

          {dynamicSLTPConfig.enabled && (
            <div className="space-y-3">
              <div className="text-xs text-gray-500 bg-gray-800/50 p-2 rounded">
                Automatically calculates SL/TP based on ATR volatility + AI analysis for each coin
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">ATR Period</label>
                  <input
                    type="number"
                    value={dynamicSLTPConfig.atr_period}
                    onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, atr_period: parseInt(e.target.value) || 14})}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                    min="7"
                    max="21"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">AI Weight</label>
                  <div className="flex items-center gap-2">
                    <input
                      type="range"
                      value={dynamicSLTPConfig.llm_weight * 100}
                      onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, llm_weight: parseInt(e.target.value) / 100})}
                      className="flex-1"
                      min="0"
                      max="100"
                      step="10"
                    />
                    <span className="text-xs text-gray-400 w-10">{Math.round(dynamicSLTPConfig.llm_weight * 100)}%</span>
                  </div>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">SL Multiplier</label>
                  <input
                    type="number"
                    value={dynamicSLTPConfig.atr_multiplier_sl}
                    onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, atr_multiplier_sl: parseFloat(e.target.value) || 1.5})}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                    min="0.5"
                    max="3"
                    step="0.1"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">TP Multiplier</label>
                  <input
                    type="number"
                    value={dynamicSLTPConfig.atr_multiplier_tp}
                    onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, atr_multiplier_tp: parseFloat(e.target.value) || 2.0})}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                    min="0.5"
                    max="5"
                    step="0.1"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Min/Max SL %</label>
                  <div className="flex gap-1">
                    <input
                      type="number"
                      value={dynamicSLTPConfig.min_sl_percent}
                      onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, min_sl_percent: parseFloat(e.target.value) || 0.3})}
                      className="w-1/2 bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                      min="0.1"
                      max="1"
                      step="0.1"
                    />
                    <input
                      type="number"
                      value={dynamicSLTPConfig.max_sl_percent}
                      onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, max_sl_percent: parseFloat(e.target.value) || 3})}
                      className="w-1/2 bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                      min="1"
                      max="10"
                      step="0.5"
                    />
                  </div>
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Min/Max TP %</label>
                  <div className="flex gap-1">
                    <input
                      type="number"
                      value={dynamicSLTPConfig.min_tp_percent}
                      onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, min_tp_percent: parseFloat(e.target.value) || 0.5})}
                      className="w-1/2 bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                      min="0.1"
                      max="2"
                      step="0.1"
                    />
                    <input
                      type="number"
                      value={dynamicSLTPConfig.max_tp_percent}
                      onChange={(e) => setDynamicSLTPConfig({...dynamicSLTPConfig, max_tp_percent: parseFloat(e.target.value) || 5})}
                      className="w-1/2 bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                      min="1"
                      max="15"
                      step="0.5"
                    />
                  </div>
                </div>
              </div>

              <button
                onClick={handleSaveDynamicSLTP}
                disabled={isSavingDynamicSLTP}
                className="w-full py-2 bg-cyan-500/20 text-cyan-500 rounded text-sm hover:bg-cyan-500/30 disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {isSavingDynamicSLTP ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
                Save Settings
              </button>
            </div>
          )}
        </div>
      )}

      {/* Scalping Mode Panel */}
      {showScalping && (
        <div className="mt-4 pt-4 border-t border-gray-700 space-y-4">
          <div className="flex items-center justify-between">
            <div className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <Zap className="w-4 h-4" />
              Scalping Mode
              {scalpingConfig.enabled && scalpingConfig.trades_today > 0 && (
                <span className="text-xs bg-yellow-500/20 text-yellow-500 px-1.5 py-0.5 rounded">
                  {scalpingConfig.trades_today} trades today
                </span>
              )}
            </div>
            <button
              onClick={handleToggleScalping}
              className={`px-2 py-1 rounded text-xs ${
                scalpingConfig.enabled
                  ? 'bg-yellow-500/20 text-yellow-500'
                  : 'bg-gray-700 text-gray-400'
              }`}
            >
              {scalpingConfig.enabled ? 'ON' : 'OFF'}
            </button>
          </div>

          {scalpingConfig.enabled && (
            <div className="space-y-3">
              <div className="text-xs text-gray-500 bg-gray-800/50 p-2 rounded">
                Book quick profits and re-enter positions rapidly for high-frequency trading
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Min Profit to Book %</label>
                  <input
                    type="number"
                    value={scalpingConfig.min_profit}
                    onChange={(e) => setScalpingConfig({...scalpingConfig, min_profit: parseFloat(e.target.value) || 0.2})}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                    min="0.05"
                    max="1"
                    step="0.05"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Re-entry Delay (sec)</label>
                  <input
                    type="number"
                    value={scalpingConfig.reentry_delay_sec}
                    onChange={(e) => setScalpingConfig({...scalpingConfig, reentry_delay_sec: parseInt(e.target.value) || 5})}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                    min="1"
                    max="60"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    id="quick-reentry"
                    checked={scalpingConfig.quick_reentry}
                    onChange={(e) => setScalpingConfig({...scalpingConfig, quick_reentry: e.target.checked})}
                    className="rounded bg-gray-800 border-gray-700"
                  />
                  <label htmlFor="quick-reentry" className="text-xs text-gray-400">
                    Quick Re-entry
                  </label>
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Max Daily Trades (0=∞)</label>
                  <input
                    type="number"
                    value={scalpingConfig.max_trades_per_day}
                    onChange={(e) => setScalpingConfig({...scalpingConfig, max_trades_per_day: parseInt(e.target.value) || 0})}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-2 py-1.5 text-sm"
                    min="0"
                    max="1000"
                  />
                </div>
              </div>

              <button
                onClick={handleSaveScalping}
                disabled={isSavingScalping}
                className="w-full py-2 bg-yellow-500/20 text-yellow-500 rounded text-sm hover:bg-yellow-500/30 disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {isSavingScalping ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
                Save Settings
              </button>
            </div>
          )}
        </div>
      )}

      {/* Loss Control / Circuit Breaker Panel */}
      {showLossControl && circuitStatus && (
        <div className="mt-4 pt-4 border-t border-gray-700 space-y-4">
          <div className="flex items-center justify-between">
            <div className="text-sm font-medium text-gray-400 flex items-center gap-2">
              <Zap className="w-4 h-4" />
              Loss Control (Circuit Breaker)
            </div>
            <div className="flex items-center gap-2">
              <span className={`text-xs font-medium ${getCBStateColor(circuitStatus.state)}`}>
                {getCBStateLabel(circuitStatus.state)}
              </span>
              <button
                onClick={handleToggleCircuitBreaker}
                className={`px-2 py-1 rounded text-xs ${
                  circuitStatus.enabled
                    ? 'bg-green-500/20 text-green-500'
                    : 'bg-gray-700 text-gray-400'
                }`}
              >
                {circuitStatus.enabled ? 'ON' : 'OFF'}
              </button>
            </div>
          </div>

          {/* Circuit Breaker Status */}
          {circuitStatus.state === 'open' && (
            <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
              <div className="flex items-center gap-2 text-red-500 text-sm font-medium mb-2">
                <AlertTriangle className="w-4 h-4" />
                Trading Paused
              </div>
              <div className="text-xs text-red-400 mb-2">
                {circuitStatus.block_reason || circuitStatus.trip_reason || 'Loss limits exceeded'}
              </div>
              <button
                onClick={handleResetCircuitBreaker}
                disabled={isResettingCB}
                className="flex items-center gap-1 px-3 py-1.5 bg-red-500/20 text-red-500 rounded text-xs hover:bg-red-500/30 disabled:opacity-50"
              >
                <RotateCcw className={`w-3 h-3 ${isResettingCB ? 'animate-spin' : ''}`} />
                Reset & Resume Trading
              </button>
            </div>
          )}

          {/* Current Stats */}
          <div className="grid grid-cols-2 gap-3">
            <div className="p-2 bg-gray-800 rounded">
              <div className="text-xs text-gray-500 mb-1">Hourly Loss</div>
              <div className="flex items-center gap-2">
                <div className="flex-1 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                  <div
                    className={`h-full ${getProgressColor(Math.abs(circuitStatus.hourly_loss), cbConfig.max_loss_per_hour)}`}
                    style={{ width: `${Math.min(100, (Math.abs(circuitStatus.hourly_loss) / cbConfig.max_loss_per_hour) * 100)}%` }}
                  />
                </div>
                <span className="text-xs text-gray-400">
                  {Math.abs(circuitStatus.hourly_loss).toFixed(1)}%
                </span>
              </div>
            </div>
            <div className="p-2 bg-gray-800 rounded">
              <div className="text-xs text-gray-500 mb-1">Daily Loss</div>
              <div className="flex items-center gap-2">
                <div className="flex-1 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                  <div
                    className={`h-full ${getProgressColor(Math.abs(circuitStatus.daily_loss), cbConfig.max_daily_loss)}`}
                    style={{ width: `${Math.min(100, (Math.abs(circuitStatus.daily_loss) / cbConfig.max_daily_loss) * 100)}%` }}
                  />
                </div>
                <span className="text-xs text-gray-400">
                  {Math.abs(circuitStatus.daily_loss).toFixed(1)}%
                </span>
              </div>
            </div>
            <div className="p-2 bg-gray-800 rounded">
              <div className="text-xs text-gray-500 mb-1 flex items-center gap-1">
                <Activity className="w-3 h-3" />
                Consecutive Losses
              </div>
              <div className="text-sm font-medium">
                {circuitStatus.consecutive_losses} / {cbConfig.max_consecutive_losses}
              </div>
            </div>
            <div className="p-2 bg-gray-800 rounded">
              <div className="text-xs text-gray-500 mb-1 flex items-center gap-1">
                <Clock className="w-3 h-3" />
                Daily Trades
              </div>
              <div className="text-sm font-medium">
                {circuitStatus.daily_trades} / {cbConfig.max_daily_trades}
              </div>
            </div>
          </div>

          {/* Config Editor */}
          {isEditingCB ? (
            <div className="p-3 bg-gray-800 rounded-lg space-y-3">
              <div className="text-xs text-gray-400 font-medium">Edit Loss Limits</div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Max Hourly Loss %</label>
                  <input
                    type="number"
                    value={cbConfig.max_loss_per_hour}
                    onChange={(e) => setCbConfig({...cbConfig, max_loss_per_hour: parseFloat(e.target.value) || 0})}
                    className="w-full bg-gray-700 border border-gray-600 rounded px-2 py-1.5 text-sm focus:outline-none focus:border-purple-500"
                    min="0.1"
                    step="0.5"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Max Daily Loss %</label>
                  <input
                    type="number"
                    value={cbConfig.max_daily_loss}
                    onChange={(e) => setCbConfig({...cbConfig, max_daily_loss: parseFloat(e.target.value) || 0})}
                    className="w-full bg-gray-700 border border-gray-600 rounded px-2 py-1.5 text-sm focus:outline-none focus:border-purple-500"
                    min="0.1"
                    step="0.5"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Max Consecutive Losses</label>
                  <input
                    type="number"
                    value={cbConfig.max_consecutive_losses}
                    onChange={(e) => setCbConfig({...cbConfig, max_consecutive_losses: parseInt(e.target.value) || 0})}
                    className="w-full bg-gray-700 border border-gray-600 rounded px-2 py-1.5 text-sm focus:outline-none focus:border-purple-500"
                    min="1"
                    step="1"
                  />
                </div>
                <div>
                  <label className="text-xs text-gray-500 mb-1 block">Cooldown (minutes)</label>
                  <input
                    type="number"
                    value={cbConfig.cooldown_minutes}
                    onChange={(e) => setCbConfig({...cbConfig, cooldown_minutes: parseInt(e.target.value) || 0})}
                    className="w-full bg-gray-700 border border-gray-600 rounded px-2 py-1.5 text-sm focus:outline-none focus:border-purple-500"
                    min="1"
                    step="5"
                  />
                </div>
                <div className="col-span-2">
                  <label className="text-xs text-gray-500 mb-1 block">Max Daily Trades</label>
                  <input
                    type="number"
                    value={cbConfig.max_daily_trades}
                    onChange={(e) => setCbConfig({...cbConfig, max_daily_trades: parseInt(e.target.value) || 0})}
                    className="w-full bg-gray-700 border border-gray-600 rounded px-2 py-1.5 text-sm focus:outline-none focus:border-purple-500"
                    min="1"
                    step="10"
                  />
                </div>
              </div>
              <div className="flex gap-2">
                <button
                  onClick={handleSaveCBConfig}
                  disabled={isSavingCB}
                  className="flex-1 flex items-center justify-center gap-1 py-1.5 bg-green-500/20 text-green-500 rounded text-xs hover:bg-green-500/30 disabled:opacity-50"
                >
                  <Save className="w-3 h-3" />
                  {isSavingCB ? 'Saving...' : 'Save'}
                </button>
                <button
                  onClick={handleCancelCBEdit}
                  className="flex-1 flex items-center justify-center gap-1 py-1.5 bg-gray-700 text-gray-400 rounded text-xs hover:bg-gray-600"
                >
                  <X className="w-3 h-3" />
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <div className="flex items-center justify-between p-2 bg-gray-800 rounded">
              <div className="text-xs text-gray-500">
                Limits: {cbConfig.max_loss_per_hour}% hourly, {cbConfig.max_daily_loss}% daily, {cbConfig.max_consecutive_losses} losses
              </div>
              <button
                onClick={() => setIsEditingCB(true)}
                className="flex items-center gap-1 px-2 py-1 text-gray-400 hover:text-gray-300 text-xs"
              >
                <Edit2 className="w-3 h-3" />
                Edit
              </button>
            </div>
          )}
        </div>
      )}

      {/* Active Positions */}
      {status.running && status.active_positions && status.active_positions.length > 0 && (
        <div className="mt-4 pt-4 border-t border-gray-700">
          <div className="text-gray-500 text-xs mb-2">Active Positions ({status.active_positions.length})</div>
          <div className="space-y-1">
            {status.active_positions.map((pos, i) => (
              <div key={i} className="flex items-center justify-between text-sm py-1 px-2 bg-gray-800 rounded">
                <span className="font-medium">{pos.symbol}</span>
                <span className={pos.side === 'LONG' ? 'text-green-500' : 'text-red-500'}>
                  {pos.side} {pos.leverage}x
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Config Summary (collapsed when settings panel is open) */}
      {status.config && !showSettings && (
        <div className="mt-4 pt-4 border-t border-gray-700">
          <div className="flex items-center gap-1 text-gray-500 text-xs mb-2">
            <Settings className="w-3 h-3" />
            <span>Current Config</span>
          </div>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div>
              <span className="text-gray-500">Leverage: </span>
              <span>{status.config.default_leverage}x</span>
            </div>
            <div>
              <span className="text-gray-500">TP/SL: </span>
              <span>{status.config.take_profit}% / {status.config.stop_loss}%</span>
            </div>
            <div>
              <span className="text-gray-500">Confidence: </span>
              <span>{formatPercent(status.config.min_confidence * 100, false)}</span>
            </div>
            <div>
              <span className="text-gray-500">Shorts: </span>
              <span>{status.config.allow_shorts ? 'Yes' : 'No'}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
