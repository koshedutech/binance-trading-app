import { useEffect, useState } from 'react';
import { spotAutopilotApi, SpotAutopilotStatus, SpotCircuitBreakerStatus, SpotProfitStats } from '../services/spotAutopilotApi';
import { Bot, Power, PowerOff, AlertTriangle, RefreshCw, TrendingUp, DollarSign, Shield, Percent, Target, Check, RotateCcw, X } from 'lucide-react';

const formatUSD = (value: number): string => {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2 }).format(value);
};

const formatPercent = (value: number): string => {
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`;
};

export default function SpotAutopilotPanel() {
  const [status, setStatus] = useState<SpotAutopilotStatus | null>(null);
  const [profitStats, setProfitStats] = useState<SpotProfitStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  // Circuit Breaker state
  const [circuitStatus, setCircuitStatus] = useState<SpotCircuitBreakerStatus | null>(null);
  const [showCircuitBreaker, setShowCircuitBreaker] = useState(false);
  const [isResettingCB, setIsResettingCB] = useState(false);

  const [initialLoadDone, setInitialLoadDone] = useState(false);

  const fetchStatus = async () => {
    try {
      const data = await spotAutopilotApi.getStatus();
      setStatus(data);
      if (!initialLoadDone) {
        setInitialLoadDone(true);
      }
      setError(null);
    } catch (err) {
      setError('Failed to fetch spot autopilot status');
      console.error(err);
    }
  };

  const fetchProfitStats = async () => {
    try {
      const stats = await spotAutopilotApi.getProfitStats();
      setProfitStats(stats);
    } catch (err) {
      console.error('Failed to fetch profit stats:', err);
    }
  };

  const fetchCircuitBreakerStatus = async () => {
    try {
      const cbStatus = await spotAutopilotApi.getCircuitBreakerStatus();
      setCircuitStatus(cbStatus);
    } catch (err) {
      console.error('Failed to fetch circuit breaker status:', err);
    }
  };

  useEffect(() => {
    fetchStatus();
    fetchProfitStats();
    fetchCircuitBreakerStatus();
    // Increased from 15s to 30s to prevent timeout race conditions
    // (10s timeout + 20s buffer before next poll)
    const interval = setInterval(() => {
      fetchStatus();
      fetchProfitStats();
      fetchCircuitBreakerStatus();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleToggle = async () => {
    if (!status) return;
    setLoading(true);
    try {
      const result = await spotAutopilotApi.toggle(!status.running);
      if (result.success) {
        fetchStatus();
        showSuccess(result.message);
      }
    } catch (err) {
      setError('Failed to toggle spot autopilot');
    } finally {
      setLoading(false);
    }
  };

  const handleDryRunToggle = async () => {
    if (!status) return;
    setLoading(true);
    try {
      const result = await spotAutopilotApi.setDryRun(!status.dry_run);
      if (result.success) {
        fetchStatus();
        showSuccess(result.message);
      }
    } catch (err) {
      setError('Failed to toggle dry run mode');
    } finally {
      setLoading(false);
    }
  };

  const handleResetCircuitBreaker = async () => {
    setIsResettingCB(true);
    try {
      const result = await spotAutopilotApi.resetCircuitBreaker();
      if (result.success) {
        fetchCircuitBreakerStatus();
        showSuccess('Circuit breaker reset');
      }
    } catch (err) {
      setError('Failed to reset circuit breaker');
    } finally {
      setIsResettingCB(false);
    }
  };

  const showSuccess = (msg: string) => {
    setSuccessMsg(msg);
    setTimeout(() => setSuccessMsg(null), 3000);
  };

  if (!status) {
    return (
      <div className="bg-gray-800 rounded-lg p-4">
        <div className="flex items-center gap-2 mb-4">
          <Bot className="w-5 h-5 text-blue-400" />
          <h3 className="text-lg font-semibold text-white">Spot AI Trader</h3>
        </div>
        <div className="text-gray-400 text-sm">Loading...</div>
      </div>
    );
  }

  const isRunning = status.running;
  const isDryRun = status.dry_run;

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Bot className="w-5 h-5 text-blue-400" />
          <h3 className="text-lg font-semibold text-white">Spot AI Trader</h3>
          {isDryRun && (
            <span className="px-2 py-0.5 text-xs bg-yellow-500/20 text-yellow-400 rounded">
              PAPER
            </span>
          )}
          {!isDryRun && isRunning && (
            <span className="px-2 py-0.5 text-xs bg-green-500/20 text-green-400 rounded">
              LIVE
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowCircuitBreaker(!showCircuitBreaker)}
            className={`p-1.5 rounded transition-colors ${
              circuitStatus?.tripped
                ? 'bg-red-500/20 text-red-400'
                : 'hover:bg-gray-700 text-gray-400'
            }`}
            title="Circuit Breaker"
          >
            <Shield className="w-4 h-4" />
          </button>
          <button
            onClick={fetchStatus}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors text-gray-400"
          >
            <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Success/Error Messages */}
      {successMsg && (
        <div className="mb-4 p-2 bg-green-500/20 text-green-400 rounded text-sm flex items-center gap-2">
          <Check className="w-4 h-4" />
          {successMsg}
        </div>
      )}
      {error && (
        <div className="mb-4 p-2 bg-red-500/20 text-red-400 rounded text-sm flex items-center gap-2">
          <AlertTriangle className="w-4 h-4" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Circuit Breaker Panel */}
      {showCircuitBreaker && circuitStatus && (
        <div className="mb-4 p-3 bg-gray-700/50 rounded-lg">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-white">Circuit Breaker</span>
            <span className={`text-xs px-2 py-0.5 rounded ${
              circuitStatus.tripped ? 'bg-red-500/20 text-red-400' : 'bg-green-500/20 text-green-400'
            }`}>
              {circuitStatus.tripped ? 'TRIPPED' : 'OK'}
            </span>
          </div>
          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="text-gray-400">Hourly Loss:</div>
            <div className={circuitStatus.hourly_loss > 0 ? 'text-red-400' : 'text-gray-300'}>
              {formatUSD(circuitStatus.hourly_loss)}
            </div>
            <div className="text-gray-400">Daily Loss:</div>
            <div className={circuitStatus.daily_loss > 0 ? 'text-red-400' : 'text-gray-300'}>
              {formatUSD(circuitStatus.daily_loss)}
            </div>
            <div className="text-gray-400">Consecutive Losses:</div>
            <div className="text-gray-300">{circuitStatus.consecutive_losses}</div>
            <div className="text-gray-400">Trades Today:</div>
            <div className="text-gray-300">{circuitStatus.trades_today}</div>
          </div>
          {circuitStatus.tripped && (
            <button
              onClick={handleResetCircuitBreaker}
              disabled={isResettingCB}
              className="mt-2 w-full py-1.5 bg-yellow-600 hover:bg-yellow-500 text-white text-xs rounded flex items-center justify-center gap-1"
            >
              <RotateCcw className={`w-3 h-3 ${isResettingCB ? 'animate-spin' : ''}`} />
              Reset Circuit Breaker
            </button>
          )}
        </div>
      )}

      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3 mb-4">
        <div className="bg-gray-700/50 rounded p-2">
          <div className="flex items-center gap-1 text-gray-400 text-xs mb-1">
            <TrendingUp className="w-3 h-3" />
            Daily PnL
          </div>
          <div className={`text-lg font-semibold ${
            (status.daily_pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'
          }`}>
            {formatUSD(status.daily_pnl || 0)}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2">
          <div className="flex items-center gap-1 text-gray-400 text-xs mb-1">
            <DollarSign className="w-3 h-3" />
            Total PnL
          </div>
          <div className={`text-lg font-semibold ${
            (status.total_pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'
          }`}>
            {formatUSD(status.total_pnl || 0)}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2">
          <div className="flex items-center gap-1 text-gray-400 text-xs mb-1">
            <Target className="w-3 h-3" />
            Win Rate
          </div>
          <div className="text-lg font-semibold text-white">
            {profitStats?.win_rate ? formatPercent(profitStats.win_rate) : '0%'}
          </div>
        </div>
        <div className="bg-gray-700/50 rounded p-2">
          <div className="flex items-center gap-1 text-gray-400 text-xs mb-1">
            <Percent className="w-3 h-3" />
            Positions
          </div>
          <div className="text-lg font-semibold text-white">
            {status.active_positions || 0} / {status.max_positions || 5}
          </div>
        </div>
      </div>

      {/* Control Buttons */}
      <div className="flex gap-2">
        <button
          onClick={handleToggle}
          disabled={loading}
          className={`flex-1 py-2.5 rounded-lg font-medium flex items-center justify-center gap-2 transition-colors ${
            isRunning
              ? 'bg-red-600 hover:bg-red-500 text-white'
              : 'bg-green-600 hover:bg-green-500 text-white'
          }`}
        >
          {isRunning ? (
            <>
              <PowerOff className="w-4 h-4" />
              Stop
            </>
          ) : (
            <>
              <Power className="w-4 h-4" />
              Start
            </>
          )}
        </button>
        <button
          onClick={handleDryRunToggle}
          disabled={loading}
          className={`px-4 py-2.5 rounded-lg font-medium flex items-center gap-2 transition-colors ${
            isDryRun
              ? 'bg-yellow-600 hover:bg-yellow-500 text-white'
              : 'bg-gray-600 hover:bg-gray-500 text-white'
          }`}
          title={isDryRun ? 'Switch to LIVE trading' : 'Switch to PAPER trading'}
        >
          {isDryRun ? 'PAPER' : 'LIVE'}
        </button>
      </div>

    </div>
  );
}
