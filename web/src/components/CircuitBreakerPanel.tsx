import { useState, useEffect } from 'react';
import { futuresApi } from '../services/futuresApi';
import { Shield, Settings, AlertTriangle, RefreshCw, TrendingDown, Activity, Clock, Target, Check, Save, X, RotateCcw } from 'lucide-react';
import { useFuturesStore } from '../store/futuresStore';

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

export default function CircuitBreakerPanel() {
  const [circuitStatus, setCircuitStatus] = useState<CircuitBreakerStatus | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isResetting, setIsResetting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  // CRITICAL: Subscribe to trading mode changes to refresh mode-specific data (paper vs live)
  const tradingMode = useFuturesStore((state) => state.tradingMode);
  // String inputs for proper text field behavior (allows clearing and typing)
  const [configInputs, setConfigInputs] = useState({
    max_loss_per_hour: '',
    max_daily_loss: '',
    max_consecutive_losses: '',
    cooldown_minutes: '',
    max_daily_trades: '',
  });

  const fetchStatus = async () => {
    try {
      const status = await futuresApi.getCircuitBreakerStatus();
      setCircuitStatus(status);
      if (status.config) {
        // Sync string inputs with server config
        setConfigInputs({
          max_loss_per_hour: status.config.max_loss_per_hour.toString(),
          max_daily_loss: status.config.max_daily_loss.toString(),
          max_consecutive_losses: status.config.max_consecutive_losses.toString(),
          cooldown_minutes: status.config.cooldown_minutes.toString(),
          max_daily_trades: status.config.max_daily_trades.toString(),
        });
      }
    } catch (err) {
      console.error('Failed to fetch circuit breaker status:', err);
    }
  };

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 15000);
    return () => clearInterval(interval);
  }, []);

  // CRITICAL: Refresh circuit breaker status when trading mode changes (paper <-> live)
  // This ensures daily_loss, hourly_loss display correct mode-specific values
  useEffect(() => {
    console.log('CircuitBreakerPanel: Trading mode changed to', tradingMode.mode, '- refreshing');
    fetchStatus();
  }, [tradingMode.dryRun]);

  const handleReset = async () => {
    setIsResetting(true);
    try {
      const result = await futuresApi.resetCircuitBreaker();
      if (result.success) {
        setSuccessMsg('Circuit breaker reset successfully');
        setTimeout(() => setSuccessMsg(null), 3000);
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to reset circuit breaker');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsResetting(false);
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      // Parse string inputs to numbers - use current config as fallback
      const configToSave = {
        max_loss_per_hour: parseFloat(configInputs.max_loss_per_hour) || circuitStatus?.config.max_loss_per_hour || 100,
        max_daily_loss: parseFloat(configInputs.max_daily_loss) || circuitStatus?.config.max_daily_loss || 500,
        max_consecutive_losses: parseInt(configInputs.max_consecutive_losses) || circuitStatus?.config.max_consecutive_losses || 5,
        cooldown_minutes: parseInt(configInputs.cooldown_minutes) || circuitStatus?.config.cooldown_minutes || 30,
        max_daily_trades: parseInt(configInputs.max_daily_trades) || circuitStatus?.config.max_daily_trades || 100,
      };
      const result = await futuresApi.updateCircuitBreakerConfig(configToSave);
      if (result.success) {
        setSuccessMsg('Circuit breaker limits updated successfully');
        setTimeout(() => setSuccessMsg(null), 3000);
        setIsEditing(false);
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to update circuit breaker limits');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsSaving(false);
    }
  };

  const handleResetDefaults = async () => {
    setIsSaving(true);
    try {
      // Reset to defaults - use reasonable defaults
      const defaults = {
        max_loss_per_hour: 100,
        max_daily_loss: 500,
        max_consecutive_losses: 5,
        cooldown_minutes: 30,
        max_daily_trades: 100,
      };
      setConfigInputs({
        max_loss_per_hour: defaults.max_loss_per_hour.toString(),
        max_daily_loss: defaults.max_daily_loss.toString(),
        max_consecutive_losses: defaults.max_consecutive_losses.toString(),
        cooldown_minutes: defaults.cooldown_minutes.toString(),
        max_daily_trades: defaults.max_daily_trades.toString(),
      });
      const result = await futuresApi.updateCircuitBreakerConfig(defaults);
      if (result.success) {
        setSuccessMsg('Circuit breaker reset to defaults');
        setTimeout(() => setSuccessMsg(null), 3000);
        setIsEditing(false);
        fetchStatus();
      }
    } catch (err) {
      setError('Failed to reset circuit breaker to defaults');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    if (circuitStatus?.config) {
      // Reset inputs to current server config
      setConfigInputs({
        max_loss_per_hour: circuitStatus.config.max_loss_per_hour.toString(),
        max_daily_loss: circuitStatus.config.max_daily_loss.toString(),
        max_consecutive_losses: circuitStatus.config.max_consecutive_losses.toString(),
        cooldown_minutes: circuitStatus.config.cooldown_minutes.toString(),
        max_daily_trades: circuitStatus.config.max_daily_trades.toString(),
      });
    }
    setIsEditing(false);
  };

  const getStateColor = (state: string) => {
    switch (state) {
      case 'closed': return 'text-green-500';
      case 'half_open': return 'text-yellow-500';
      case 'open': return 'text-red-500';
      default: return 'text-gray-500';
    }
  };

  const getStateLabel = (state: string) => {
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

  if (!circuitStatus) {
    return (
      <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
        <div className="flex items-center gap-2 text-gray-400">
          <Shield className="w-5 h-5" />
          <span className="font-semibold">Circuit Breaker</span>
          <span className="text-sm text-gray-500">(Loading...)</span>
        </div>
      </div>
    );
  }

  if (!circuitStatus.available) {
    return (
      <div className="bg-gray-800 rounded-lg border border-yellow-700 p-4">
        <div className="flex items-center gap-2 text-yellow-400">
          <Shield className="w-5 h-5" />
          <span className="font-semibold">Circuit Breaker</span>
          <span className="text-sm text-yellow-500">(Not Configured)</span>
        </div>
        <p className="text-sm text-gray-400 mt-2">
          {circuitStatus.message || 'Circuit breaker not available. Check server logs for details.'}
        </p>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Shield className={`w-5 h-5 ${getStateColor(circuitStatus.state)}`} />
          <span className="font-semibold">Circuit Breaker</span>
          <span className={`text-sm ${getStateColor(circuitStatus.state)}`}>
            ({getStateLabel(circuitStatus.state)})
          </span>
        </div>
        <div className="flex items-center gap-2">
          {!isEditing && (
            <button
              onClick={() => setIsEditing(true)}
              className="p-1.5 hover:bg-gray-700 rounded"
              title="Configure Limits"
            >
              <Settings className="w-4 h-4 text-gray-400" />
            </button>
          )}
          <button
            onClick={fetchStatus}
            className="p-1.5 hover:bg-gray-700 rounded"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4 text-gray-400" />
          </button>
          {circuitStatus.state === 'open' && (
            <button
              onClick={handleReset}
              disabled={isResetting}
              className="flex items-center gap-1 px-3 py-1.5 bg-yellow-500/20 text-yellow-500 rounded text-sm hover:bg-yellow-500/30"
            >
              {isResetting ? <RefreshCw className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
              Reset
            </button>
          )}
        </div>
      </div>

      {/* Alerts */}
      {error && (
        <div className="mb-3 p-2 bg-red-500/10 border border-red-500/30 rounded text-red-500 text-sm">
          {error}
        </div>
      )}
      {successMsg && (
        <div className="mb-3 p-2 bg-green-500/10 border border-green-500/30 rounded text-green-500 text-sm flex items-center gap-2">
          <Check className="w-4 h-4" />
          {successMsg}
        </div>
      )}

      {/* Trip Reason Alert */}
      {circuitStatus.state === 'open' && circuitStatus.trip_reason && (
        <div className="mb-3 p-3 bg-red-500/10 border border-red-500/30 rounded">
          <div className="flex items-center gap-2 text-red-500">
            <AlertTriangle className="w-4 h-4" />
            <span className="font-medium">Trading Halted</span>
          </div>
          <p className="text-sm text-gray-300 mt-1">{circuitStatus.trip_reason}</p>
        </div>
      )}

      {/* Editable Limits */}
      {isEditing ? (
        <div className="space-y-4 p-4 bg-gray-900 rounded border border-purple-500/30">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-purple-400">Configure Circuit Breaker</span>
            <div className="flex gap-2">
              <button onClick={handleCancel} className="p-1.5 hover:bg-gray-700 rounded" title="Cancel">
                <X className="w-4 h-4 text-gray-400" />
              </button>
              <button
                onClick={handleResetDefaults}
                disabled={isSaving}
                className="flex items-center gap-1 px-3 py-1.5 bg-gray-600 hover:bg-gray-700 text-white rounded text-sm"
                title="Reset to defaults"
              >
                <RotateCcw className="w-4 h-4" />
                Default
              </button>
              <button
                onClick={handleSave}
                disabled={isSaving}
                className="flex items-center gap-1 px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded text-sm"
              >
                {isSaving ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
                Save
              </button>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs text-gray-400 mb-1">Max Hourly Loss ($)</label>
              <input
                type="text"
                value={configInputs.max_loss_per_hour}
                onChange={(e) => setConfigInputs({ ...configInputs, max_loss_per_hour: e.target.value })}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm focus:border-purple-500 focus:outline-none"
                placeholder="100"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Max Daily Loss ($)</label>
              <input
                type="text"
                value={configInputs.max_daily_loss}
                onChange={(e) => setConfigInputs({ ...configInputs, max_daily_loss: e.target.value })}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm focus:border-purple-500 focus:outline-none"
                placeholder="500"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Max Consecutive Losses</label>
              <input
                type="text"
                value={configInputs.max_consecutive_losses}
                onChange={(e) => setConfigInputs({ ...configInputs, max_consecutive_losses: e.target.value })}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm focus:border-purple-500 focus:outline-none"
                placeholder="5"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-400 mb-1">Cooldown (minutes)</label>
              <input
                type="text"
                value={configInputs.cooldown_minutes}
                onChange={(e) => setConfigInputs({ ...configInputs, cooldown_minutes: e.target.value })}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm focus:border-purple-500 focus:outline-none"
                placeholder="30"
              />
            </div>
            <div className="col-span-2">
              <label className="block text-xs text-gray-400 mb-1">Max Daily Trades</label>
              <input
                type="text"
                value={configInputs.max_daily_trades}
                onChange={(e) => setConfigInputs({ ...configInputs, max_daily_trades: e.target.value })}
                className="w-full px-3 py-2 bg-gray-800 border border-gray-700 rounded text-sm focus:border-purple-500 focus:outline-none"
                placeholder="100"
              />
            </div>
          </div>
        </div>
      ) : (
        /* Progress bars display */
        <div className="space-y-3">
          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2 text-gray-400">
              <TrendingDown className="w-4 h-4" />
              Consecutive Losses
            </div>
            <div className="flex items-center gap-2">
              <div className="w-20 h-2 bg-gray-700 rounded-full overflow-hidden">
                <div
                  className={`h-full ${getProgressColor(circuitStatus.consecutive_losses, circuitStatus.config.max_consecutive_losses)}`}
                  style={{ width: `${Math.min((circuitStatus.consecutive_losses / circuitStatus.config.max_consecutive_losses) * 100, 100)}%` }}
                />
              </div>
              <span className={`font-medium min-w-[60px] text-right ${circuitStatus.consecutive_losses >= circuitStatus.config.max_consecutive_losses - 1 ? 'text-red-500' : 'text-white'}`}>
                {circuitStatus.consecutive_losses} / {circuitStatus.config.max_consecutive_losses}
              </span>
            </div>
          </div>

          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2 text-gray-400">
              <Activity className="w-4 h-4" />
              Hourly Loss
            </div>
            <div className="flex items-center gap-2">
              <div className="w-20 h-2 bg-gray-700 rounded-full overflow-hidden">
                <div
                  className={`h-full ${getProgressColor(circuitStatus.hourly_loss, circuitStatus.config.max_loss_per_hour)}`}
                  style={{ width: `${Math.min((circuitStatus.hourly_loss / circuitStatus.config.max_loss_per_hour) * 100, 100)}%` }}
                />
              </div>
              <span className={`font-medium min-w-[80px] text-right ${circuitStatus.hourly_loss >= circuitStatus.config.max_loss_per_hour * 0.8 ? 'text-red-500' : 'text-white'}`}>
                {circuitStatus.hourly_loss.toFixed(2)}% / {circuitStatus.config.max_loss_per_hour}%
              </span>
            </div>
          </div>

          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2 text-gray-400">
              <Target className="w-4 h-4" />
              Daily Loss
            </div>
            <div className="flex items-center gap-2">
              <div className="w-20 h-2 bg-gray-700 rounded-full overflow-hidden">
                <div
                  className={`h-full ${getProgressColor(circuitStatus.daily_loss, circuitStatus.config.max_daily_loss)}`}
                  style={{ width: `${Math.min((circuitStatus.daily_loss / circuitStatus.config.max_daily_loss) * 100, 100)}%` }}
                />
              </div>
              <span className={`font-medium min-w-[80px] text-right ${circuitStatus.daily_loss >= circuitStatus.config.max_daily_loss * 0.8 ? 'text-red-500' : 'text-white'}`}>
                {circuitStatus.daily_loss.toFixed(2)}% / {circuitStatus.config.max_daily_loss}%
              </span>
            </div>
          </div>

          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2 text-gray-400">
              <Clock className="w-4 h-4" />
              Daily Trades
            </div>
            <div className="flex items-center gap-2">
              <div className="w-20 h-2 bg-gray-700 rounded-full overflow-hidden">
                <div
                  className={`h-full ${getProgressColor(circuitStatus.daily_trades, circuitStatus.config.max_daily_trades)}`}
                  style={{ width: `${Math.min((circuitStatus.daily_trades / circuitStatus.config.max_daily_trades) * 100, 100)}%` }}
                />
              </div>
              <span className="font-medium text-white min-w-[70px] text-right">
                {circuitStatus.daily_trades} / {circuitStatus.config.max_daily_trades}
              </span>
            </div>
          </div>
        </div>
      )}

      {/* Can Trade Indicator */}
      <div className={`mt-4 p-3 rounded ${circuitStatus.can_trade ? 'bg-green-500/10 border border-green-500/30' : 'bg-red-500/10 border border-red-500/30'}`}>
        <div className="flex items-center gap-2">
          {circuitStatus.can_trade ? (
            <>
              <Check className="w-5 h-5 text-green-500" />
              <span className="text-green-500 font-medium">Trading Allowed</span>
            </>
          ) : (
            <>
              <AlertTriangle className="w-5 h-5 text-red-500" />
              <span className="text-red-500 font-medium">Trading Blocked</span>
            </>
          )}
        </div>
        {!circuitStatus.can_trade && circuitStatus.block_reason && (
          <p className="text-sm text-gray-400 mt-1">{circuitStatus.block_reason}</p>
        )}
      </div>
    </div>
  );
}
