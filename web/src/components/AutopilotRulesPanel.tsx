import { useState, useEffect } from 'react';
import { apiService } from '../services/api';
import {
  Bot,
  Shield,
  AlertTriangle,
  Power,
  PowerOff,
  RefreshCw,
  TrendingDown,
  Activity,
  Clock,
  Target,
  AlertCircle,
  Check,
  Settings,
  Save,
  X,
} from 'lucide-react';

interface AutopilotStatus {
  available: boolean;
  enabled: boolean;
  running: boolean;
  dry_run: boolean;
  stats?: {
    total_decisions: number;
    approved_decisions: number;
    rejected_decisions: number;
    total_trades: number;
    winning_trades: number;
    losing_trades: number;
    total_pnl: number;
    daily_pnl: number;
    win_rate: number;
  };
  circuit_breaker?: {
    enabled: boolean;
    state: string;
    can_trade: boolean;
    trip_reason: string;
    stats: any;
  };
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
    max_loss_per_hour: number;
    max_daily_loss: number;
    max_consecutive_losses: number;
    cooldown_minutes: number;
    max_trades_per_minute: number;
    max_daily_trades: number;
  };
}

interface EditableConfig {
  max_loss_per_hour: number;
  max_daily_loss: number;
  max_consecutive_losses: number;
  cooldown_minutes: number;
  max_daily_trades: number;
}

export default function AutopilotRulesPanel() {
  const [autopilotStatus, setAutopilotStatus] = useState<AutopilotStatus | null>(null);
  const [circuitStatus, setCircuitStatus] = useState<CircuitBreakerStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isToggling, setIsToggling] = useState(false);
  const [isResetting, setIsResetting] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [editConfig, setEditConfig] = useState<EditableConfig>({
    max_loss_per_hour: 3,
    max_daily_loss: 5,
    max_consecutive_losses: 5,
    cooldown_minutes: 30,
    max_daily_trades: 100,
  });

  // Auto-select content on focus for easier value replacement
  const handleInputFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    e.target.select();
  };

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, 15000); // Reduced from 5s to 15s to avoid rate limits
    return () => clearInterval(interval);
  }, []);

  const fetchStatus = async () => {
    try {
      const [autopilot, circuit] = await Promise.all([
        apiService.getAutopilotStatus(),
        apiService.getCircuitBreakerStatus(),
      ]);
      setAutopilotStatus(autopilot);
      setCircuitStatus(circuit);

      // Update edit config with current values
      if (circuit.config) {
        setEditConfig({
          max_loss_per_hour: circuit.config.max_loss_per_hour,
          max_daily_loss: circuit.config.max_daily_loss,
          max_consecutive_losses: circuit.config.max_consecutive_losses,
          cooldown_minutes: circuit.config.cooldown_minutes,
          max_daily_trades: circuit.config.max_daily_trades,
        });
      }
      setError(null);
    } catch (err) {
      console.error('Failed to fetch status:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleToggleAutopilot = async () => {
    if (!autopilotStatus) return;

    try {
      setIsToggling(true);
      const result = await apiService.toggleAutopilot(!autopilotStatus.running);
      if (result.success) {
        setSuccessMsg(result.message);
        setTimeout(() => setSuccessMsg(null), 3000);
        await fetchStatus();
      }
    } catch (err) {
      setError('Failed to toggle autopilot');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsToggling(false);
    }
  };

  const handleResetCircuitBreaker = async () => {
    try {
      setIsResetting(true);
      const result = await apiService.resetCircuitBreaker();
      if (result.success) {
        setSuccessMsg('Circuit breaker reset successfully');
        setTimeout(() => setSuccessMsg(null), 3000);
        await fetchStatus();
      }
    } catch (err) {
      setError('Failed to reset circuit breaker');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsResetting(false);
    }
  };

  const handleSaveConfig = async () => {
    try {
      setIsSaving(true);
      const result = await apiService.updateCircuitBreakerConfig({
        max_loss_per_hour: editConfig.max_loss_per_hour,
        max_daily_loss: editConfig.max_daily_loss,
        max_consecutive_losses: editConfig.max_consecutive_losses,
        cooldown_minutes: editConfig.cooldown_minutes,
        max_daily_trades: editConfig.max_daily_trades,
      });
      if (result.success) {
        setSuccessMsg('Loss limits updated successfully');
        setTimeout(() => setSuccessMsg(null), 3000);
        setIsEditing(false);
        await fetchStatus();
      }
    } catch (err) {
      setError('Failed to update loss limits');
      setTimeout(() => setError(null), 3000);
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancelEdit = () => {
    // Reset to current values
    if (circuitStatus?.config) {
      setEditConfig({
        max_loss_per_hour: circuitStatus.config.max_loss_per_hour,
        max_daily_loss: circuitStatus.config.max_daily_loss,
        max_consecutive_losses: circuitStatus.config.max_consecutive_losses,
        cooldown_minutes: circuitStatus.config.cooldown_minutes,
        max_daily_trades: circuitStatus.config.max_daily_trades,
      });
    }
    setIsEditing(false);
  };

  const getStateColor = (state: string) => {
    switch (state) {
      case 'closed':
        return 'text-green-500';
      case 'half_open':
        return 'text-yellow-500';
      case 'open':
        return 'text-red-500';
      default:
        return 'text-gray-500';
    }
  };

  const getStateLabel = (state: string) => {
    switch (state) {
      case 'closed':
        return 'Active';
      case 'half_open':
        return 'Testing';
      case 'open':
        return 'Tripped';
      default:
        return state;
    }
  };

  const getProgressColor = (current: number, max: number) => {
    const percent = (current / max) * 100;
    if (percent >= 80) return 'bg-red-500';
    if (percent >= 50) return 'bg-yellow-500';
    return 'bg-green-500';
  };

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700 animate-pulse">
        <div className="h-6 bg-gray-700 rounded w-48 mb-4"></div>
        <div className="space-y-3">
          <div className="h-4 bg-gray-700 rounded w-full"></div>
          <div className="h-4 bg-gray-700 rounded w-3/4"></div>
          <div className="h-4 bg-gray-700 rounded w-1/2"></div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <Bot className="w-5 h-5 text-primary-500" />
          <span className="font-semibold text-white">AI Autopilot Control</span>
        </div>
        <div className="flex items-center gap-2">
          {!isEditing && (
            <button
              onClick={() => setIsEditing(true)}
              className="p-1.5 hover:bg-gray-700 rounded transition-colors"
              title="Configure Limits"
            >
              <Settings className="w-4 h-4 text-gray-400" />
            </button>
          )}
          <button
            onClick={fetchStatus}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw className="w-4 h-4 text-gray-400" />
          </button>
        </div>
      </div>

      {/* Alerts */}
      {error && (
        <div className="mx-4 mt-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-red-500" />
          <span className="text-sm text-red-500">{error}</span>
        </div>
      )}
      {successMsg && (
        <div className="mx-4 mt-4 p-3 bg-green-500/10 border border-green-500/30 rounded-lg flex items-center gap-2">
          <Check className="w-4 h-4 text-green-500" />
          <span className="text-sm text-green-500">{successMsg}</span>
        </div>
      )}

      <div className="p-4 space-y-4">
        {/* Autopilot Status */}
        <div className="bg-gray-900 rounded-lg p-4">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <div className={`w-3 h-3 rounded-full ${autopilotStatus?.running ? 'bg-green-500 animate-pulse' : 'bg-gray-500'}`} />
              <div>
                <div className="font-medium text-white">
                  Autopilot {autopilotStatus?.running ? 'Running' : 'Stopped'}
                </div>
                <div className="text-xs text-gray-400">
                  {autopilotStatus?.dry_run ? 'Paper Trading Mode' : 'Live Trading Mode'}
                </div>
              </div>
            </div>
            <button
              onClick={handleToggleAutopilot}
              disabled={isToggling || !autopilotStatus?.available}
              className={`
                flex items-center gap-2 px-4 py-2 rounded-lg font-medium transition-colors
                ${autopilotStatus?.running
                  ? 'bg-red-500/20 hover:bg-red-500/30 text-red-500 border border-red-500/30'
                  : 'bg-green-500/20 hover:bg-green-500/30 text-green-500 border border-green-500/30'
                }
                ${isToggling ? 'opacity-50 cursor-wait' : ''}
                ${!autopilotStatus?.available ? 'opacity-50 cursor-not-allowed' : ''}
              `}
            >
              {isToggling ? (
                <RefreshCw className="w-4 h-4 animate-spin" />
              ) : autopilotStatus?.running ? (
                <PowerOff className="w-4 h-4" />
              ) : (
                <Power className="w-4 h-4" />
              )}
              {autopilotStatus?.running ? 'Stop' : 'Start'}
            </button>
          </div>

          {/* Stats */}
          {autopilotStatus?.stats && (
            <div className="grid grid-cols-4 gap-3 pt-3 border-t border-gray-700">
              <div className="text-center">
                <div className="text-lg font-bold text-white">{autopilotStatus.stats.total_trades}</div>
                <div className="text-xs text-gray-400">Trades</div>
              </div>
              <div className="text-center">
                <div className={`text-lg font-bold ${autopilotStatus.stats.win_rate >= 0.5 ? 'text-green-500' : 'text-red-500'}`}>
                  {(autopilotStatus.stats.win_rate * 100).toFixed(1)}%
                </div>
                <div className="text-xs text-gray-400">Win Rate</div>
              </div>
              <div className="text-center">
                <div className={`text-lg font-bold ${autopilotStatus.stats.total_pnl >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                  ${autopilotStatus.stats.total_pnl.toFixed(2)}
                </div>
                <div className="text-xs text-gray-400">Total PnL</div>
              </div>
              <div className="text-center">
                <div className={`text-lg font-bold ${autopilotStatus.stats.daily_pnl >= 0 ? 'text-green-500' : 'text-red-500'}`}>
                  ${autopilotStatus.stats.daily_pnl.toFixed(2)}
                </div>
                <div className="text-xs text-gray-400">Daily PnL</div>
              </div>
            </div>
          )}
        </div>

        {/* Circuit Breaker Status */}
        <div className="bg-gray-900 rounded-lg p-4">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-3">
              <Shield className={`w-5 h-5 ${getStateColor(circuitStatus?.state || 'closed')}`} />
              <div>
                <div className="font-medium text-white">Circuit Breaker</div>
                <div className={`text-sm ${getStateColor(circuitStatus?.state || 'closed')}`}>
                  {getStateLabel(circuitStatus?.state || 'closed')}
                </div>
              </div>
            </div>
            {circuitStatus?.state === 'open' && (
              <button
                onClick={handleResetCircuitBreaker}
                disabled={isResetting}
                className="flex items-center gap-2 px-4 py-2 bg-yellow-500/20 hover:bg-yellow-500/30 text-yellow-500 border border-yellow-500/30 rounded-lg font-medium transition-colors"
              >
                {isResetting ? (
                  <RefreshCw className="w-4 h-4 animate-spin" />
                ) : (
                  <RefreshCw className="w-4 h-4" />
                )}
                Reset & Resume
              </button>
            )}
          </div>

          {/* Trip Reason Alert */}
          {circuitStatus?.state === 'open' && circuitStatus.trip_reason && (
            <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
              <div className="flex items-center gap-2 text-red-500">
                <AlertTriangle className="w-4 h-4" />
                <span className="font-medium">Trading Halted</span>
              </div>
              <p className="text-sm text-gray-300 mt-1">{circuitStatus.trip_reason}</p>
            </div>
          )}

          {/* Editable Limits */}
          {isEditing ? (
            <div className="space-y-4 mb-4 p-4 bg-gray-800 rounded-lg border border-primary-500/30">
              <div className="flex items-center justify-between mb-2">
                <span className="text-sm font-medium text-primary-400">Configure Loss Limits</span>
                <div className="flex gap-2">
                  <button
                    onClick={handleCancelEdit}
                    className="p-1.5 hover:bg-gray-700 rounded text-gray-400"
                    title="Cancel"
                  >
                    <X className="w-4 h-4" />
                  </button>
                  <button
                    onClick={handleSaveConfig}
                    disabled={isSaving}
                    className="flex items-center gap-1 px-3 py-1 bg-primary-600 hover:bg-primary-700 text-white rounded text-sm font-medium"
                  >
                    {isSaving ? <RefreshCw className="w-3 h-3 animate-spin" /> : <Save className="w-3 h-3" />}
                    Save
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Max Hourly Loss (%)</label>
                  <input
                    type="number"
                    min="0"
                    max="20"
                    step="0.5"
                    value={editConfig.max_loss_per_hour}
                    onChange={(e) => setEditConfig({ ...editConfig, max_loss_per_hour: parseFloat(e.target.value) || 0 })}
                    onFocus={handleInputFocus}
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded text-white text-sm focus:border-primary-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Max Daily Loss (%)</label>
                  <input
                    type="number"
                    min="0"
                    max="50"
                    step="0.5"
                    value={editConfig.max_daily_loss}
                    onChange={(e) => setEditConfig({ ...editConfig, max_daily_loss: parseFloat(e.target.value) || 0 })}
                    onFocus={handleInputFocus}
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded text-white text-sm focus:border-primary-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Max Consecutive Losses</label>
                  <input
                    type="number"
                    min="0"
                    max="20"
                    step="1"
                    value={editConfig.max_consecutive_losses}
                    onChange={(e) => setEditConfig({ ...editConfig, max_consecutive_losses: parseInt(e.target.value) || 0 })}
                    onFocus={handleInputFocus}
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded text-white text-sm focus:border-primary-500 focus:outline-none"
                  />
                </div>
                <div>
                  <label className="block text-xs text-gray-400 mb-1">Cooldown (minutes)</label>
                  <input
                    type="number"
                    min="0"
                    max="120"
                    step="5"
                    value={editConfig.cooldown_minutes}
                    onChange={(e) => setEditConfig({ ...editConfig, cooldown_minutes: parseInt(e.target.value) || 0 })}
                    onFocus={handleInputFocus}
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded text-white text-sm focus:border-primary-500 focus:outline-none"
                  />
                </div>
                <div className="col-span-2">
                  <label className="block text-xs text-gray-400 mb-1">Max Daily Trades</label>
                  <input
                    type="number"
                    min="0"
                    max="500"
                    step="10"
                    value={editConfig.max_daily_trades}
                    onChange={(e) => setEditConfig({ ...editConfig, max_daily_trades: parseInt(e.target.value) || 0 })}
                    onFocus={handleInputFocus}
                    className="w-full px-3 py-2 bg-gray-900 border border-gray-700 rounded text-white text-sm focus:border-primary-500 focus:outline-none"
                  />
                </div>
              </div>
            </div>
          ) : (
            /* Safety Rules Display */
            <div className="space-y-3">
              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2 text-gray-400">
                  <TrendingDown className="w-4 h-4" />
                  Consecutive Losses
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
                    <div
                      className={`h-full ${getProgressColor(circuitStatus?.consecutive_losses || 0, circuitStatus?.config?.max_consecutive_losses || 5)}`}
                      style={{ width: `${Math.min(((circuitStatus?.consecutive_losses || 0) / (circuitStatus?.config?.max_consecutive_losses || 5)) * 100, 100)}%` }}
                    />
                  </div>
                  <span className={`font-medium min-w-[80px] text-right ${(circuitStatus?.consecutive_losses || 0) >= (circuitStatus?.config?.max_consecutive_losses || 5) - 1 ? 'text-red-500' : 'text-white'}`}>
                    {circuitStatus?.consecutive_losses || 0} / {circuitStatus?.config?.max_consecutive_losses || 5}
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2 text-gray-400">
                  <Activity className="w-4 h-4" />
                  Hourly Loss
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
                    <div
                      className={`h-full ${getProgressColor(circuitStatus?.hourly_loss || 0, circuitStatus?.config?.max_loss_per_hour || 3)}`}
                      style={{ width: `${Math.min(((circuitStatus?.hourly_loss || 0) / (circuitStatus?.config?.max_loss_per_hour || 3)) * 100, 100)}%` }}
                    />
                  </div>
                  <span className={`font-medium min-w-[80px] text-right ${(circuitStatus?.hourly_loss || 0) >= (circuitStatus?.config?.max_loss_per_hour || 3) * 0.8 ? 'text-red-500' : 'text-white'}`}>
                    {(circuitStatus?.hourly_loss || 0).toFixed(2)}% / {circuitStatus?.config?.max_loss_per_hour || 3}%
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2 text-gray-400">
                  <Target className="w-4 h-4" />
                  Daily Loss
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
                    <div
                      className={`h-full ${getProgressColor(circuitStatus?.daily_loss || 0, circuitStatus?.config?.max_daily_loss || 5)}`}
                      style={{ width: `${Math.min(((circuitStatus?.daily_loss || 0) / (circuitStatus?.config?.max_daily_loss || 5)) * 100, 100)}%` }}
                    />
                  </div>
                  <span className={`font-medium min-w-[80px] text-right ${(circuitStatus?.daily_loss || 0) >= (circuitStatus?.config?.max_daily_loss || 5) * 0.8 ? 'text-red-500' : 'text-white'}`}>
                    {(circuitStatus?.daily_loss || 0).toFixed(2)}% / {circuitStatus?.config?.max_daily_loss || 5}%
                  </span>
                </div>
              </div>

              <div className="flex items-center justify-between text-sm">
                <div className="flex items-center gap-2 text-gray-400">
                  <Clock className="w-4 h-4" />
                  Daily Trades
                </div>
                <div className="flex items-center gap-2">
                  <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
                    <div
                      className={`h-full ${getProgressColor(circuitStatus?.daily_trades || 0, circuitStatus?.config?.max_daily_trades || 100)}`}
                      style={{ width: `${Math.min(((circuitStatus?.daily_trades || 0) / (circuitStatus?.config?.max_daily_trades || 100)) * 100, 100)}%` }}
                    />
                  </div>
                  <span className="font-medium text-white min-w-[80px] text-right">
                    {circuitStatus?.daily_trades || 0} / {circuitStatus?.config?.max_daily_trades || 100}
                  </span>
                </div>
              </div>
            </div>
          )}

          {/* Can Trade Indicator */}
          <div className={`mt-4 p-3 rounded-lg ${circuitStatus?.can_trade ? 'bg-green-500/10 border border-green-500/30' : 'bg-red-500/10 border border-red-500/30'}`}>
            <div className="flex items-center gap-2">
              {circuitStatus?.can_trade ? (
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
            {!circuitStatus?.can_trade && circuitStatus?.block_reason && (
              <p className="text-sm text-gray-400 mt-1">{circuitStatus.block_reason}</p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
