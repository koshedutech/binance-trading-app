import { useState } from 'react';
import {
  Database,
  ChevronDown,
  ChevronUp,
  Play,
  Square,
  RefreshCw,
  AlertCircle,
} from 'lucide-react';
import {
  useCoinProfilerStatus,
  useCoinProfilerCoins,
  useCoinProfilerControl,
} from '../../hooks/useCoinProfiler';
import CoinProfilerStatus, { CoinProfilerStatusDetailed } from './CoinProfilerStatus';
import CoinList from './CoinList';

// ============================================================================
// Epic 14: Coin Profiler Card Component
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================

/**
 * Main Coin Profiler Card - Expandable card showing profiler status and tracked coins
 *
 * This component serves as the central data hub visualization for the Chain Trading System.
 * It displays real-time WebSocket connection status, tracked symbols, and data collection metrics.
 */
export default function CoinProfilerCard() {
  const [isExpanded, setIsExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState<'status' | 'coins'>('status');

  // Hooks for data fetching
  const { data: status, isLoading: statusLoading, refetch: refetchStatus } = useCoinProfilerStatus();
  const { data: coinsData, isLoading: coinsLoading, refetch: refetchCoins } = useCoinProfilerCoins(
    isExpanded && activeTab === 'coins', // Only auto-poll when visible
    10000 // Poll every 10 seconds
  );
  const { start, stop, isStarting, isStopping, error: controlError, reset: resetControl } = useCoinProfilerControl();

  // Handle start/stop actions
  const handleStart = async () => {
    try {
      await start();
      refetchStatus();
    } catch {
      // Error is handled by the hook
    }
  };

  const handleStop = async () => {
    try {
      await stop();
      refetchStatus();
    } catch {
      // Error is handled by the hook
    }
  };

  // Determine badge content
  const getBadgeContent = () => {
    if (!status) return null;
    if (!status.running) return 'Stopped';
    if (!status.connected) return 'Reconnecting';
    return `${status.symbol_count} coins`;
  };

  const getBadgeColor = (): 'green' | 'yellow' | 'red' | 'gray' => {
    if (!status) return 'gray';
    if (!status.running) return 'gray';
    if (!status.connected) return 'yellow';
    return 'green';
  };

  const badgeColors = {
    green: 'bg-green-500/20 text-green-400',
    yellow: 'bg-yellow-500/20 text-yellow-400',
    red: 'bg-red-500/20 text-red-400',
    gray: 'bg-gray-500/20 text-gray-400',
  };

  return (
    <div className="bg-gray-700/30 rounded-lg overflow-hidden">
      {/* Header - Clickable to expand */}
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <Database className="w-4 h-4 text-cyan-400" />
          <h3 className="text-sm font-medium text-gray-300">Coin Profiler</h3>
          {getBadgeContent() && (
            <span className={`px-2 py-0.5 rounded text-xs font-medium ${badgeColors[getBadgeColor()]}`}>
              {getBadgeContent()}
            </span>
          )}
        </div>

        <div className="flex items-center gap-3">
          {/* Compact Status (when collapsed) */}
          {!isExpanded && (
            <CoinProfilerStatus status={status} isLoading={statusLoading} />
          )}

          {/* Control Buttons (visible in both states) */}
          <div className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
            {status?.running ? (
              <button
                onClick={handleStop}
                disabled={isStopping}
                className="px-2 py-1 bg-red-500/20 hover:bg-red-500/30 text-red-400 rounded text-xs font-medium flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed"
                title="Stop Profiler"
              >
                {isStopping ? (
                  <RefreshCw className="w-3 h-3 animate-spin" />
                ) : (
                  <Square className="w-3 h-3" />
                )}
                Stop
              </button>
            ) : (
              <button
                onClick={handleStart}
                disabled={isStarting}
                className="px-2 py-1 bg-green-500/20 hover:bg-green-500/30 text-green-400 rounded text-xs font-medium flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed"
                title="Start Profiler"
              >
                {isStarting ? (
                  <RefreshCw className="w-3 h-3 animate-spin" />
                ) : (
                  <Play className="w-3 h-3" />
                )}
                Start
              </button>
            )}
          </div>

          {/* Expand/Collapse Toggle */}
          {isExpanded ? (
            <ChevronUp className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          )}
        </div>
      </button>

      {/* Control Error */}
      {controlError && (
        <div className="px-3 py-2 bg-red-900/20 border-t border-red-800">
          <div className="flex items-center gap-2 text-red-400 text-xs">
            <AlertCircle className="w-4 h-4" />
            <span>{controlError}</span>
            <button
              onClick={resetControl}
              className="ml-auto text-red-400 hover:text-red-300"
            >
              Dismiss
            </button>
          </div>
        </div>
      )}

      {/* Expanded Content */}
      <div
        className={`transition-all duration-200 ease-in-out overflow-hidden ${
          isExpanded ? 'max-h-[600px] opacity-100' : 'max-h-0 opacity-0'
        }`}
      >
        <div className="px-3 pb-3">
          {/* Tab Navigation */}
          <div className="flex items-center gap-1 mb-3 bg-gray-800 rounded-lg p-1">
            <button
              onClick={() => setActiveTab('status')}
              className={`flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                activeTab === 'status'
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-gray-300'
              }`}
            >
              Status
            </button>
            <button
              onClick={() => {
                setActiveTab('coins');
                refetchCoins(); // Refresh coins when switching to tab
              }}
              className={`flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                activeTab === 'coins'
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-gray-300'
              }`}
            >
              Coins ({coinsData?.total || 0})
            </button>
          </div>

          {/* Tab Content */}
          {activeTab === 'status' ? (
            <div>
              <CoinProfilerStatusDetailed status={status} isLoading={statusLoading} />

              {/* Refresh Button */}
              <div className="mt-3 flex justify-end">
                <button
                  onClick={refetchStatus}
                  className="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 flex items-center gap-1"
                >
                  <RefreshCw className={`w-3 h-3 ${statusLoading ? 'animate-spin' : ''}`} />
                  Refresh Status
                </button>
              </div>
            </div>
          ) : (
            <div>
              <CoinList data={coinsData} isLoading={coinsLoading} />

              {/* Refresh Button */}
              <div className="mt-3 flex justify-end">
                <button
                  onClick={refetchCoins}
                  className="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 flex items-center gap-1"
                >
                  <RefreshCw className={`w-3 h-3 ${coinsLoading ? 'animate-spin' : ''}`} />
                  Refresh Coins
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
