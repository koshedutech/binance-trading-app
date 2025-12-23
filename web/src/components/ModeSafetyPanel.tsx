import { useEffect, useState } from 'react';
import { Shield, AlertTriangle, Pause, Play, TrendingUp, Activity, BarChart3, RefreshCw, AlertCircle } from 'lucide-react';
import { futuresApi } from '../services/futuresApi';

interface ModeStatus {
  mode: string;
  paused: boolean;
  pause_reason: string;
  pause_until: string | null;
  current_win_rate: number;
  min_win_rate: number;
  recent_trades_pct: number;
  max_loss_window: number;
}

interface ModeSafetyStatus {
  success: boolean;
  modes: {
    [key: string]: ModeStatus;
  };
  timestamp: string;
}

export default function ModeSafetyPanel() {
  const [safetyStatus, setSafetyStatus] = useState<{ [key: string]: ModeStatus }>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  const modeNames: { [key: string]: string } = {
    ultra_fast: 'Ultra-Fast Scalping',
    scalp: 'Scalp',
    swing: 'Swing',
    position: 'Position',
  };

  const modeColors: { [key: string]: string } = {
    ultra_fast: 'from-red-500 to-orange-500',
    scalp: 'from-orange-500 to-yellow-500',
    swing: 'from-blue-500 to-cyan-500',
    position: 'from-green-500 to-emerald-500',
  };

  const fetchSafetyStatus = async () => {
    setLoading(true);
    try {
      const data: ModeSafetyStatus = await futuresApi.getModeSafetyStatus();
      setSafetyStatus(data.modes || {});
      setError(null);
      setLastUpdate(new Date());
    } catch (err) {
      setError('Failed to fetch safety status');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSafetyStatus();
    const interval = setInterval(fetchSafetyStatus, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleResumeMode = async (mode: string) => {
    try {
      await futuresApi.resumeMode(mode);
      await fetchSafetyStatus();
    } catch (err) {
      setError(`Failed to resume ${mode} mode`);
      console.error(err);
    }
  };

  const getWinRateColor = (current: number, min: number): string => {
    if (current >= min) return 'text-green-400';
    if (current >= min - 5) return 'text-yellow-400';
    return 'text-red-400';
  };

  const getTimeRemaining = (pauseUntil: string | null): string => {
    if (!pauseUntil) return '';
    const resumeTime = new Date(pauseUntil);
    const now = new Date();
    const diffMs = resumeTime.getTime() - now.getTime();

    if (diffMs <= 0) return 'Resuming...';

    const minutes = Math.floor(diffMs / 60000);
    const seconds = Math.floor((diffMs % 60000) / 1000);

    if (minutes > 0) {
      return `${minutes}m ${seconds}s remaining`;
    }
    return `${seconds}s remaining`;
  };

  const renderModeCard = (mode: string, status: ModeStatus) => {
    const isPaused = status.paused;
    const timeRemaining = getTimeRemaining(status.pause_until);

    return (
      <div key={mode} className="bg-gray-800 rounded-lg p-4 border border-gray-700">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2">
            <div className={`p-2 bg-gradient-to-br ${modeColors[mode]} rounded text-white`}>
              <Shield className="w-4 h-4" />
            </div>
            <h3 className="font-semibold text-white">{modeNames[mode]}</h3>
          </div>
          {isPaused ? (
            <div className="flex items-center gap-2 px-3 py-1 bg-red-900 border border-red-700 rounded">
              <Pause className="w-4 h-4 text-red-400" />
              <span className="text-sm text-red-200">Paused</span>
            </div>
          ) : (
            <div className="flex items-center gap-2 px-3 py-1 bg-green-900 border border-green-700 rounded">
              <Play className="w-4 h-4 text-green-400" />
              <span className="text-sm text-green-200">Active</span>
            </div>
          )}
        </div>

        {isPaused && status.pause_reason && (
          <div className="mb-4 p-2 bg-yellow-900 border border-yellow-700 rounded">
            <p className="text-sm text-yellow-200 flex items-start gap-2">
              <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
              <span>
                <strong>Paused:</strong> {status.pause_reason}
                {timeRemaining && <div className="text-xs mt-1">{timeRemaining}</div>}
              </span>
            </p>
          </div>
        )}

        <div className="space-y-3">
          {/* Win Rate */}
          <div>
            <div className="flex justify-between items-center mb-1">
              <span className="text-sm text-gray-300 flex items-center gap-2">
                <TrendingUp className="w-4 h-4" />
                Win Rate
              </span>
              <span className={`text-sm font-semibold ${getWinRateColor(status.current_win_rate, status.min_win_rate)}`}>
                {status.current_win_rate.toFixed(1)}% / {status.min_win_rate.toFixed(1)}%
              </span>
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2">
              <div
                className={`h-2 rounded-full ${status.current_win_rate >= status.min_win_rate ? 'bg-green-500' : 'bg-red-500'}`}
                style={{ width: `${Math.min((status.current_win_rate / status.min_win_rate) * 100, 100)}%` }}
              />
            </div>
          </div>

          {/* Trade Activity */}
          <div>
            <div className="flex justify-between items-center mb-1">
              <span className="text-sm text-gray-300 flex items-center gap-2">
                <Activity className="w-4 h-4" />
                Recent Activity
              </span>
              <span className="text-sm text-gray-400">{status.recent_trades_pct.toFixed(1)}%</span>
            </div>
            <div className="w-full bg-gray-700 rounded-full h-2">
              <div
                className="h-2 rounded-full bg-blue-500"
                style={{ width: `${Math.min(status.recent_trades_pct, 100)}%` }}
              />
            </div>
          </div>

          {/* Loss Threshold */}
          <div>
            <div className="flex justify-between items-center mb-1">
              <span className="text-sm text-gray-300 flex items-center gap-2">
                <BarChart3 className="w-4 h-4" />
                Max Loss Window
              </span>
              <span className="text-sm text-gray-400">{status.max_loss_window.toFixed(2)}%</span>
            </div>
          </div>
        </div>

        {isPaused && (
          <button
            onClick={() => handleResumeMode(mode)}
            className="w-full mt-4 px-3 py-2 bg-green-600 hover:bg-green-700 text-white rounded text-sm font-semibold transition flex items-center justify-center gap-2"
          >
            <Play className="w-4 h-4" />
            Resume Mode
          </button>
        )}
      </div>
    );
  };

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 p-6">
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-gradient-to-br from-blue-500 to-purple-500 rounded-lg text-white">
            <Shield className="w-6 h-6" />
          </div>
          <div>
            <h2 className="text-2xl font-bold text-white">Mode Safety Control</h2>
            <p className="text-gray-400 text-sm">Monitor and manage trading safety across modes</p>
          </div>
        </div>
        <button
          onClick={fetchSafetyStatus}
          disabled={loading}
          className="p-2 hover:bg-gray-800 rounded text-gray-400 hover:text-gray-200 transition"
        >
          <RefreshCw className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-900 border border-red-700 rounded text-red-200 text-sm flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {['ultra_fast', 'scalp', 'swing', 'position'].map(mode => {
          const status = safetyStatus[mode];
          if (!status) return null;
          return renderModeCard(mode, status);
        })}
      </div>

      {lastUpdate && (
        <p className="text-xs text-gray-500 mt-4 text-right">
          Last updated: {lastUpdate.toLocaleTimeString()}
        </p>
      )}

      {/* Safety Information */}
      <div className="mt-6 p-4 bg-gray-800 border border-gray-700 rounded">
        <h3 className="font-semibold text-white mb-2 flex items-center gap-2">
          <AlertCircle className="w-4 h-4" />
          How Safety Controls Work
        </h3>
        <ul className="text-sm text-gray-300 space-y-1">
          <li>• <strong>Win Rate:</strong> Modes pause if win rate drops below minimum threshold</li>
          <li>• <strong>Recent Activity:</strong> Tracks trading frequency and adjusts rate limits</li>
          <li>• <strong>Max Loss Window:</strong> Prevents cumulative losses within rolling time window</li>
          <li>• <strong>Auto-Resume:</strong> Modes automatically resume after cooldown period expires</li>
        </ul>
      </div>
    </div>
  );
}
