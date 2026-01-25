import { Wifi, WifiOff, RefreshCw, Activity, Clock, AlertTriangle } from 'lucide-react';
import type { CoinProfilerStatus as CoinProfilerStatusType } from '../../hooks/useCoinProfiler';

// ============================================================================
// Epic 14: Coin Profiler Status Display
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================

interface CoinProfilerStatusProps {
  status: CoinProfilerStatusType | null;
  isLoading: boolean;
}

/**
 * Connection status indicator with color coding
 */
function ConnectionIndicator({ connected, running }: { connected: boolean; running: boolean }) {
  if (!running) {
    return (
      <div className="flex items-center gap-1.5">
        <div className="w-2 h-2 rounded-full bg-gray-500" />
        <span className="text-xs text-gray-500">Stopped</span>
      </div>
    );
  }

  if (connected) {
    return (
      <div className="flex items-center gap-1.5">
        <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
        <Wifi className="w-3 h-3 text-green-500" />
        <span className="text-xs text-green-500">Connected</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-1.5">
      <div className="w-2 h-2 rounded-full bg-yellow-500 animate-pulse" />
      <RefreshCw className="w-3 h-3 text-yellow-500 animate-spin" />
      <span className="text-xs text-yellow-500">Reconnecting</span>
    </div>
  );
}

/**
 * Compact status display showing key metrics
 */
export default function CoinProfilerStatus({ status, isLoading }: CoinProfilerStatusProps) {
  if (isLoading && !status) {
    return (
      <div className="flex items-center gap-3 text-gray-500">
        <RefreshCw className="w-4 h-4 animate-spin" />
        <span className="text-xs">Loading status...</span>
      </div>
    );
  }

  if (!status) {
    return (
      <div className="flex items-center gap-2 text-gray-500">
        <WifiOff className="w-4 h-4" />
        <span className="text-xs">Status unavailable</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-4 flex-wrap">
      {/* Connection Status */}
      <ConnectionIndicator connected={status.connected} running={status.running} />

      {/* Symbol Count */}
      <div className="flex items-center gap-1.5 text-xs">
        <Activity className="w-3 h-3 text-purple-400" />
        <span className="text-gray-400">{status.symbol_count}</span>
        <span className="text-gray-600">symbols</span>
      </div>

      {/* Subscriptions */}
      <div className="flex items-center gap-1.5 text-xs">
        <Wifi className="w-3 h-3 text-blue-400" />
        <span className="text-gray-400">{status.subscription_count}</span>
        <span className="text-gray-600">subs</span>
      </div>

      {/* Updates per second */}
      {status.running && status.updates_per_second > 0 && (
        <div className="flex items-center gap-1.5 text-xs">
          <RefreshCw className="w-3 h-3 text-cyan-400" />
          <span className="text-gray-400">{status.updates_per_second.toFixed(1)}</span>
          <span className="text-gray-600">upd/s</span>
        </div>
      )}

      {/* Uptime */}
      {status.running && status.uptime && (
        <div className="flex items-center gap-1.5 text-xs">
          <Clock className="w-3 h-3 text-gray-500" />
          <span className="text-gray-500">{status.uptime}</span>
        </div>
      )}

      {/* Reconnect Count (if > 0) */}
      {status.reconnect_count > 0 && (
        <div className="flex items-center gap-1.5 text-xs text-yellow-500">
          <AlertTriangle className="w-3 h-3" />
          <span>{status.reconnect_count} reconnects</span>
        </div>
      )}

      {/* Last Error (if any) */}
      {status.last_error && (
        <div className="flex items-center gap-1.5 text-xs text-red-400" title={status.last_error}>
          <AlertTriangle className="w-3 h-3" />
          <span className="truncate max-w-32">Error</span>
        </div>
      )}
    </div>
  );
}

/**
 * Detailed status display for expanded view
 */
export function CoinProfilerStatusDetailed({ status, isLoading }: CoinProfilerStatusProps) {
  if (isLoading && !status) {
    return (
      <div className="p-4 text-center text-gray-500">
        <RefreshCw className="w-5 h-5 animate-spin mx-auto mb-2" />
        <span className="text-sm">Loading profiler status...</span>
      </div>
    );
  }

  if (!status) {
    return (
      <div className="p-4 text-center text-gray-500">
        <WifiOff className="w-5 h-5 mx-auto mb-2" />
        <span className="text-sm">Profiler status unavailable</span>
      </div>
    );
  }

  const formatTime = (timeStr: string) => {
    if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return 'N/A';
    try {
      return new Date(timeStr).toLocaleString();
    } catch {
      return timeStr;
    }
  };

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
      {/* Status */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Status</div>
        <div className="flex items-center gap-2">
          <div className={`w-2.5 h-2.5 rounded-full ${
            status.running
              ? status.connected
                ? 'bg-green-500 animate-pulse'
                : 'bg-yellow-500 animate-pulse'
              : 'bg-gray-500'
          }`} />
          <span className={`text-sm font-medium ${
            status.running
              ? status.connected
                ? 'text-green-400'
                : 'text-yellow-400'
              : 'text-gray-400'
          }`}>
            {status.running
              ? status.connected
                ? 'Connected'
                : 'Reconnecting'
              : 'Stopped'}
          </span>
        </div>
      </div>

      {/* Symbols */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Symbols</div>
        <div className="text-lg font-bold text-purple-400">{status.symbol_count}</div>
      </div>

      {/* Subscriptions */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Subscriptions</div>
        <div className="text-lg font-bold text-blue-400">{status.subscription_count}</div>
      </div>

      {/* Update Rate */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Update Rate</div>
        <div className="text-lg font-bold text-cyan-400">
          {status.updates_per_second.toFixed(1)}/s
        </div>
      </div>

      {/* Last Update */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Last Update</div>
        <div className="text-xs text-gray-300">{formatTime(status.last_update_time)}</div>
      </div>

      {/* Started At */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Started</div>
        <div className="text-xs text-gray-300">{formatTime(status.started_at)}</div>
      </div>

      {/* Uptime */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Uptime</div>
        <div className="text-sm text-gray-300">{status.uptime || 'N/A'}</div>
      </div>

      {/* Reconnects */}
      <div className="bg-gray-800 rounded-lg p-3">
        <div className="text-[10px] text-gray-500 uppercase mb-1">Reconnects</div>
        <div className={`text-lg font-bold ${status.reconnect_count > 0 ? 'text-yellow-400' : 'text-gray-400'}`}>
          {status.reconnect_count}
        </div>
      </div>

      {/* Last Error (full width) */}
      {status.last_error && (
        <div className="col-span-2 md:col-span-4 bg-red-900/20 border border-red-800 rounded-lg p-3">
          <div className="text-[10px] text-red-500 uppercase mb-1">Last Error</div>
          <div className="text-xs text-red-400 break-words">{status.last_error}</div>
        </div>
      )}
    </div>
  );
}
