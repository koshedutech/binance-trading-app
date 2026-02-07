import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import {
  Database,
  ChevronDown,
  ChevronUp,
  Play,
  Square,
  RefreshCw,
  AlertCircle,
  Zap,
  Activity,
  AlertTriangle,
  Lock,
} from 'lucide-react';
import {
  useCoinProfilerStatus,
  useCoinProfilerControl,
  useCoinProfilerRequirements,
  useCoinProfilerRealtime,
  type CoinDataUpdate,
} from '../../hooks/useCoinProfiler';
import { useTradingState } from '../../hooks/useTradingState';
import { wsService } from '../../services/websocket';
import CoinProfilerStatus, { CoinProfilerStatusDetailed } from './CoinProfilerStatus';
import CoinList from './CoinList';
import RequirementsBreakdown from './RequirementsBreakdown';

// ============================================================================
// Epic 14: Coin Profiler Card Component
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================

/**
 * Main Coin Profiler Card - Expandable card showing profiler status and tracked coins
 *
 * This component serves as the central data hub visualization for the Chain Trading System.
 * It displays real-time WebSocket connection status, tracked symbols, and data collection metrics.
 *
 * REAL-TIME UPDATES:
 * - Subscribes to COIN_DATA_UPDATE WebSocket events
 * - Updates individual coin data immediately when received (millisecond updates)
 * - No polling - direct 1:1 push from backend to frontend
 */
export default function CoinProfilerCard() {
  const [isExpanded, setIsExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState<'sources' | 'status' | 'coins'>('coins');
  const [wsConnected, setWsConnected] = useState(wsService.isConnected());

  // Hooks for data fetching
  const { data: status, isLoading: statusLoading, refetch: refetchStatus } = useCoinProfilerStatus();
  const { data: requirements, isLoading: reqsLoading, refetch: refetchReqs } = useCoinProfilerRequirements(
    isExpanded && activeTab === 'sources', // Only auto-poll when visible
    10000 // Poll every 10 seconds
  );
  const { start, stop, isStarting, isStopping, error: controlError, reset: resetControl } = useCoinProfilerControl();

  // Real-time coin data with WebSocket updates
  const { coins, total, lastUpdate, updateCount, handleCoinUpdate, updateCoinSource, refetch: refetchCoins } = useCoinProfilerRealtime(wsConnected);

  // Trading state - controls what can be started/stopped
  const { data: tradingState } = useTradingState();
  const tradingEnabled = tradingState?.enabled ?? true;

  // Count active positions (coins with source "position" or "both")
  const positionCoinsCount = useMemo(() => {
    if (!coins) return 0;
    return coins.filter(c => c.source === 'position' || c.source === 'both').length;
  }, [coins]);

  // WebSocket-driven position adjustment for instant Stop button locking
  // This bridges the gap between position creation and the next poll cycle
  const [wsPositionAdjustment, setWsPositionAdjustment] = useState(0);
  const prevPollCountRef = useRef(positionCoinsCount);

  // Reset adjustment when poll data arrives (poll is source of truth)
  useEffect(() => {
    if (positionCoinsCount !== prevPollCountRef.current) {
      setWsPositionAdjustment(0);
      prevPollCountRef.current = positionCoinsCount;
    }
  }, [positionCoinsCount]);

  // Subscribe to position lifecycle WebSocket events
  const handlePositionCreated = useCallback((event: any) => {
    setWsPositionAdjustment(prev => prev + 1);
    // POSITION_CREATED event nests data under event.data.position (not event.data directly)
    const data = event?.data?.position || event?.data;
    if (data?.symbol) {
      // Check if coin already exists as "strategy" source -> becomes "both"
      const existingCoin = coins.find(c => c.symbol === data.symbol);
      const newSource = existingCoin?.source === 'strategy' ? 'both' : 'position';
      updateCoinSource(data.symbol, newSource, data.timeframe);
    }
    // Delay refetch slightly to ensure backend DB has committed all updates
    // (chain status ACTIVE, coin profiler source updated)
    setTimeout(() => {
      refetchCoins();
      refetchReqs();
    }, 500);
  }, [coins, updateCoinSource, refetchCoins, refetchReqs]);

  const handleChainClosed = useCallback((event: any) => {
    setWsPositionAdjustment(prev => Math.max(0, prev - 1));
    // CHAIN_CLOSED event puts data directly in event.data (not nested)
    const data = event?.data;
    if (data?.symbol) {
      const existingCoin = coins.find(c => c.symbol === data.symbol);
      if (existingCoin?.source === 'both') {
        updateCoinSource(data.symbol, 'strategy');
      } else if (existingCoin?.source === 'position') {
        updateCoinSource(data.symbol, 'strategy');
      }
    }
    // Delay refetch slightly to ensure backend has completed all updates
    setTimeout(() => {
      refetchCoins();
      refetchReqs();
    }, 500);
  }, [coins, updateCoinSource, refetchCoins, refetchReqs]);

  // Handle POSITION_UPDATE with status "CLOSED" (catches positions closing without a chain)
  const handlePositionUpdate = useCallback((event: any) => {
    const data = event?.data;
    if (Array.isArray(data) && data.some((p: any) => p.status === 'CLOSED')) {
      refetchCoins();
      refetchReqs();
    }
  }, [refetchCoins, refetchReqs]);

  useEffect(() => {
    wsService.subscribe('POSITION_CREATED', handlePositionCreated);
    wsService.subscribe('CHAIN_CLOSED', handleChainClosed);
    wsService.subscribe('POSITION_UPDATE', handlePositionUpdate);

    return () => {
      wsService.unsubscribe('POSITION_CREATED', handlePositionCreated);
      wsService.unsubscribe('CHAIN_CLOSED', handleChainClosed);
      wsService.unsubscribe('POSITION_UPDATE', handlePositionUpdate);
    };
  }, [handlePositionCreated, handleChainClosed, handlePositionUpdate]);

  const effectivePositionCount = positionCoinsCount + wsPositionAdjustment;

  // Control restrictions
  const canStopProfiler = effectivePositionCount === 0; // Can only stop if no positions
  const canStartProfiler = tradingEnabled; // Can only start if trading is ON

  // Subscribe to WebSocket events for real-time coin updates
  useEffect(() => {
    const handleWsEvent = (event: { type: string; data?: CoinDataUpdate }) => {
      if (event.type === 'COIN_DATA_UPDATE' && event.data) {
        handleCoinUpdate(event.data);
      }
    };

    const handleConnect = () => setWsConnected(true);
    const handleDisconnect = () => setWsConnected(false);

    // Subscribe to coin data updates
    wsService.subscribe('COIN_DATA_UPDATE', handleWsEvent);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    // Set initial state
    setWsConnected(wsService.isConnected());

    return () => {
      wsService.unsubscribe('COIN_DATA_UPDATE', handleWsEvent);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);
    };
  }, [handleCoinUpdate]);

  // Warning states for control restrictions
  const [showStopWarning, setShowStopWarning] = useState(false);
  const [showStartWarning, setShowStartWarning] = useState(false);

  // Handle start/stop actions with restrictions
  const handleStart = async () => {
    if (!canStartProfiler) {
      setShowStartWarning(true);
      setTimeout(() => setShowStartWarning(false), 5000);
      return;
    }
    try {
      await start();
      refetchStatus();
    } catch {
      // Error is handled by the hook
    }
  };

  const handleStop = async () => {
    if (!canStopProfiler) {
      setShowStopWarning(true);
      setTimeout(() => setShowStopWarning(false), 5000);
      return;
    }
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
    return `${total || status.symbol_count} coins`;
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

  // Format update rate
  const formatUpdateRate = () => {
    if (!status?.updates_per_second) return null;
    return `${status.updates_per_second}/s`;
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
          {/* Real-time indicator */}
          {status?.running && wsConnected && (
            <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-cyan-500/20 text-cyan-400">
              <Zap className="w-3 h-3" />
              LIVE
            </span>
          )}
          {/* Update rate */}
          {formatUpdateRate() && (
            <span className="text-[10px] text-gray-500">
              {formatUpdateRate()}
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
                disabled={isStopping || !canStopProfiler}
                className={`px-2 py-1 rounded text-xs font-medium flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed ${
                  !canStopProfiler
                    ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30'
                    : 'bg-red-500/20 hover:bg-red-500/30 text-red-400'
                }`}
                title={!canStopProfiler ? `Cannot stop: ${effectivePositionCount} position(s) active. Pause Trade Cycle instead.` : 'Stop Profiler'}
              >
                {isStopping ? (
                  <RefreshCw className="w-3 h-3 animate-spin" />
                ) : !canStopProfiler ? (
                  <Lock className="w-3 h-3" />
                ) : (
                  <Square className="w-3 h-3" />
                )}
                {!canStopProfiler ? 'Locked' : 'Stop'}
              </button>
            ) : (
              <button
                onClick={handleStart}
                disabled={isStarting || !canStartProfiler}
                className={`px-2 py-1 rounded text-xs font-medium flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed ${
                  !canStartProfiler
                    ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30'
                    : 'bg-green-500/20 hover:bg-green-500/30 text-green-400'
                }`}
                title={!canStartProfiler ? 'Enable Trade Cycle first to start Coin Profiler' : 'Start Profiler'}
              >
                {isStarting ? (
                  <RefreshCw className="w-3 h-3 animate-spin" />
                ) : !canStartProfiler ? (
                  <Lock className="w-3 h-3" />
                ) : (
                  <Play className="w-3 h-3" />
                )}
                {!canStartProfiler ? 'Locked' : 'Start'}
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

      {/* Warning: Cannot stop due to active positions */}
      {showStopWarning && (
        <div className="px-3 py-2 bg-yellow-900/20 border-t border-yellow-800">
          <div className="flex items-center gap-2 text-yellow-400 text-xs">
            <AlertTriangle className="w-4 h-4" />
            <span>
              Cannot stop Coin Profiler: <strong>{effectivePositionCount} position(s)</strong> are active.
              Use <strong>Trade Cycle OFF</strong> to pause new entries while monitoring positions.
            </span>
            <button
              onClick={() => setShowStopWarning(false)}
              className="ml-auto text-yellow-400 hover:text-yellow-300"
            >
              ×
            </button>
          </div>
        </div>
      )}

      {/* Warning: Cannot start due to Trade Cycle OFF */}
      {showStartWarning && (
        <div className="px-3 py-2 bg-yellow-900/20 border-t border-yellow-800">
          <div className="flex items-center gap-2 text-yellow-400 text-xs">
            <AlertTriangle className="w-4 h-4" />
            <span>
              Cannot start Coin Profiler: <strong>Trade Cycle is OFF</strong>.
              Enable Trade Cycle first to start data collection.
            </span>
            <button
              onClick={() => setShowStartWarning(false)}
              className="ml-auto text-yellow-400 hover:text-yellow-300"
            >
              ×
            </button>
          </div>
        </div>
      )}

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
              onClick={() => {
                setActiveTab('sources');
                refetchReqs(); // Refresh requirements when switching to tab
              }}
              className={`flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                activeTab === 'sources'
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-gray-300'
              }`}
            >
              Sources
            </button>
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
                if (!coins.length) refetchCoins(); // Only fetch if no coins yet
              }}
              className={`flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors ${
                activeTab === 'coins'
                  ? 'bg-gray-700 text-white'
                  : 'text-gray-400 hover:text-gray-300'
              }`}
            >
              Coins ({total || 0})
            </button>
          </div>

          {/* Tab Content */}
          {activeTab === 'sources' && (
            <div>
              <RequirementsBreakdown requirements={requirements} isLoading={reqsLoading} />

              {/* Refresh Button */}
              <div className="mt-3 flex justify-end">
                <button
                  onClick={refetchReqs}
                  className="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 flex items-center gap-1"
                >
                  <RefreshCw className={`w-3 h-3 ${reqsLoading ? 'animate-spin' : ''}`} />
                  Refresh
                </button>
              </div>
            </div>
          )}

          {activeTab === 'status' && (
            <div>
              <CoinProfilerStatusDetailed status={status} isLoading={statusLoading} />

              {/* Real-time stats */}
              {status?.running && (
                <div className="mt-3 p-2 bg-gray-800/50 rounded">
                  <div className="flex items-center gap-2 text-xs">
                    <Activity className="w-3 h-3 text-cyan-400" />
                    <span className="text-gray-400">Real-time Updates:</span>
                    <span className="text-cyan-400 font-medium">{updateCount.toLocaleString()}</span>
                    {lastUpdate && (
                      <span className="text-gray-500 ml-2">
                        Last: {lastUpdate.toLocaleTimeString()}
                      </span>
                    )}
                  </div>
                </div>
              )}

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
          )}

          {activeTab === 'coins' && (
            <div>
              <CoinList
                data={{ coins, total }}
                isLoading={false} // Real-time data, never "loading"
              />

              {/* Real-time indicator */}
              <div className="mt-3 flex items-center justify-between">
                <div className="flex items-center gap-2 text-[10px] text-gray-500">
                  {wsConnected ? (
                    <>
                      <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
                      <span>Live updates active</span>
                    </>
                  ) : (
                    <>
                      <span className="w-2 h-2 rounded-full bg-yellow-500" />
                      <span>Connecting...</span>
                    </>
                  )}
                  {lastUpdate && (
                    <span className="ml-2">
                      Updated: {lastUpdate.toLocaleTimeString()}
                    </span>
                  )}
                </div>
                <button
                  onClick={refetchCoins}
                  className="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs text-gray-300 flex items-center gap-1"
                >
                  <RefreshCw className="w-3 h-3" />
                  Reload
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
