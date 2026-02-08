// Strategy First View Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Main strategy-first view with mode grouping and collapsible sections

import React, { useState, useEffect, useRef, useMemo } from 'react';
import {
  ChevronDown,
  ChevronRight,
  RefreshCw,
  AlertTriangle,
  Layers,
  Zap,
  Activity,
  TrendingUp,
  CheckCircle,
  AlertCircle,
  Wifi,
  WifiOff,
  LayoutGrid,
  List,
  Pause,
  Briefcase,
} from 'lucide-react';
import StrategyCard, { StrategyBadge } from './StrategyCard';
import { useEntryDecisionStrategies } from '../../hooks/useEntryDecision';
import type {
  StrategyMatch,
  ModeStrategies,
  MODE_DISPLAY_NAMES,
  MODE_COLORS,
} from '../../types/entryDecision';

// ==================== Interfaces ====================

interface StrategyFirstViewProps {
  /** Whether to start expanded */
  defaultExpanded?: boolean;
  /** Optional CSS class */
  className?: string;
  /** Callback when a coin is selected */
  onCoinSelect?: (symbol: string, strategy: StrategyMatch) => void;
  /** Callback when a strategy is selected */
  onStrategySelect?: (strategy: StrategyMatch) => void;
  /** Whether trading is enabled - when false, only shows position coins */
  tradingEnabled?: boolean;
}

interface ModeSectionProps {
  /** Mode strategies data */
  modeData: ModeStrategies;
  /** Whether initially expanded */
  defaultExpanded?: boolean;
  /** Callback when a coin is selected */
  onCoinSelect?: (symbol: string, strategy: StrategyMatch) => void;
}

// ==================== Constants ====================

const MODE_DISPLAY_NAMES_MAP: Record<string, string> = {
  scalp: 'Scalp Mode',
  swing: 'Swing Mode',
  position: 'Position Mode',
  ultra_fast: 'Ultra Fast Mode',
};

const MODE_COLORS_MAP: Record<string, { bg: string; text: string; border: string; icon: string }> = {
  scalp: {
    bg: 'bg-blue-500/20',
    text: 'text-blue-400',
    border: 'border-blue-500/30',
    icon: 'text-blue-400',
  },
  swing: {
    bg: 'bg-green-500/20',
    text: 'text-green-400',
    border: 'border-green-500/30',
    icon: 'text-green-400',
  },
  position: {
    bg: 'bg-purple-500/20',
    text: 'text-purple-400',
    border: 'border-purple-500/30',
    icon: 'text-purple-400',
  },
  ultra_fast: {
    bg: 'bg-orange-500/20',
    text: 'text-orange-400',
    border: 'border-orange-500/30',
    icon: 'text-orange-400',
  },
};

const MODE_ICONS: Record<string, React.ReactNode> = {
  scalp: <Zap className="w-4 h-4" />,
  swing: <Activity className="w-4 h-4" />,
  position: <TrendingUp className="w-4 h-4" />,
  ultra_fast: <Zap className="w-4 h-4" />,
};

// ==================== Helper Functions ====================

/**
 * Count ready coins across all strategies in a mode
 */
function countModeReadyCoins(modeData: ModeStrategies): number {
  return modeData.strategies.reduce((total, strategy) => {
    return total + strategy.coins.filter(c =>
      c.status === 'ready' || c.ready === true
    ).length;
  }, 0);
}

/**
 * Count total coins across all strategies in a mode
 */
function countModeTotalCoins(modeData: ModeStrategies): number {
  return modeData.strategies.reduce((total, strategy) => {
    return total + strategy.coins.length;
  }, 0);
}

// Sort throttle interval - only re-sort every 10 seconds to prevent rapid reordering
const STRATEGY_SORT_THROTTLE_MS = 10000;

// ==================== Mode Section Component ====================

function ModeSection({
  modeData,
  defaultExpanded = true,
  onCoinSelect,
}: ModeSectionProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const autoExpandedRef = useRef(false);

  // Throttled strategy sort order - only updates every STRATEGY_SORT_THROTTLE_MS
  const [sortedStrategyKeys, setSortedStrategyKeys] = useState<string[]>([]);
  const lastSortTimeRef = useRef<number>(0);

  // Auto-expand when strategies have coins with data (step > 0 or status != 'watching')
  useEffect(() => {
    if (autoExpandedRef.current) return;

    const hasActiveCoins = modeData.strategies.some(strategy =>
      strategy.coins.some(c =>
        (c.step && c.step > 0) ||
        c.status === 'accumulation' ||
        c.status === 'consolidating' ||
        c.status === 'ready' ||
        (c.volume_multiplier && c.volume_multiplier >= 1.5) // Coins with significant volume
      )
    );

    if (hasActiveCoins) {
      setExpanded(true);
      autoExpandedRef.current = true;
    }
  }, [modeData.strategies]);

  const colors = MODE_COLORS_MAP[modeData.mode] || MODE_COLORS_MAP.scalp;
  const displayName = MODE_DISPLAY_NAMES_MAP[modeData.mode] || modeData.mode;
  const icon = MODE_ICONS[modeData.mode] || <Layers className="w-4 h-4" />;

  const readyCoins = countModeReadyCoins(modeData);
  const totalCoins = countModeTotalCoins(modeData);
  const hasReady = readyCoins > 0;

  // Generate unique key for a strategy
  const getStrategyKey = (s: StrategyMatch): string =>
    `${s.mode}-${s.strategy}-${s.sub_strategy || ''}`;

  // Compute sorted order function
  const computeStrategyOrder = (strategies: StrategyMatch[]): string[] => {
    return [...strategies].sort((a, b) => {
      const aReady = a.coins.filter(c => c.status === 'ready' || c.ready === true).length;
      const bReady = b.coins.filter(c => c.status === 'ready' || c.ready === true).length;
      return bReady - aReady;
    }).map(getStrategyKey);
  };

  // Initialize sort order on first render and when strategy list changes
  useEffect(() => {
    const currentKeys = new Set(modeData.strategies.map(getStrategyKey));
    const storedKeys = new Set(sortedStrategyKeys);

    // Check if strategies were added or removed
    const keysChanged = currentKeys.size !== storedKeys.size ||
      [...currentKeys].some(k => !storedKeys.has(k));

    if (sortedStrategyKeys.length === 0 || keysChanged) {
      setSortedStrategyKeys(computeStrategyOrder(modeData.strategies));
      lastSortTimeRef.current = Date.now();
    }
  }, [modeData.strategies.map(getStrategyKey).join(',')]); // Only when strategy keys change

  // Throttled sort update - runs every STRATEGY_SORT_THROTTLE_MS
  useEffect(() => {
    const interval = setInterval(() => {
      const now = Date.now();
      if (now - lastSortTimeRef.current >= STRATEGY_SORT_THROTTLE_MS) {
        setSortedStrategyKeys(computeStrategyOrder(modeData.strategies));
        lastSortTimeRef.current = now;
      }
    }, STRATEGY_SORT_THROTTLE_MS);

    return () => clearInterval(interval);
  }, [modeData.strategies]);

  // Get strategies in throttled sort order with fresh data
  const sortedStrategies = useMemo(() => {
    if (sortedStrategyKeys.length === 0) {
      // Fallback: compute directly
      return computeStrategyOrder(modeData.strategies).map(key =>
        modeData.strategies.find(s => getStrategyKey(s) === key)
      ).filter((s): s is StrategyMatch => s !== undefined);
    }

    const orderedStrategies: StrategyMatch[] = [];
    const usedKeys = new Set<string>();

    // First, add strategies in stored order
    for (const key of sortedStrategyKeys) {
      const strategy = modeData.strategies.find(s => getStrategyKey(s) === key);
      if (strategy) {
        orderedStrategies.push(strategy);
        usedKeys.add(key);
      }
    }

    // Then, add any new strategies not in stored order
    for (const strategy of modeData.strategies) {
      if (!usedKeys.has(getStrategyKey(strategy))) {
        orderedStrategies.push(strategy);
      }
    }

    return orderedStrategies;
  }, [modeData.strategies, sortedStrategyKeys]);

  return (
    <div className={`rounded-lg border ${colors.border} overflow-hidden`}>
      {/* Mode Header */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className={`w-full flex items-center justify-between p-3 ${colors.bg} hover:opacity-90 transition-opacity`}
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}
          <span className={colors.icon}>{icon}</span>
          <span className={`font-semibold ${colors.text}`}>{displayName}</span>
          <span className="text-xs text-gray-400 bg-gray-700/50 px-2 py-0.5 rounded">
            {modeData.strategies.length} strateg{modeData.strategies.length !== 1 ? 'ies' : 'y'}
          </span>
        </div>

        <div className="flex items-center gap-3">
          {hasReady && (
            <span className="flex items-center gap-1 px-2 py-0.5 bg-green-500/20 text-green-400 text-xs rounded-full border border-green-500/30">
              <CheckCircle className="w-3 h-3" />
              {readyCoins} ready
            </span>
          )}
          {totalCoins > readyCoins && (
            <span className="text-xs text-gray-500">
              {totalCoins - readyCoins} watching
            </span>
          )}
        </div>
      </button>

      {/* Strategies list */}
      {expanded && (
        <div className="p-3 space-y-2 bg-gray-900/30">
          {sortedStrategies.length === 0 ? (
            <div className="text-center py-4 text-gray-500 text-sm">
              No strategies enabled for this mode
            </div>
          ) : (
            sortedStrategies.map((strategy) => (
              <StrategyCard
                key={`${strategy.mode}-${strategy.strategy}-${strategy.sub_strategy}`}
                strategy={strategy}
                defaultExpanded={strategy.coins.some(c => c.status === 'ready' || c.ready === true)}
                onCoinSelect={onCoinSelect}
              />
            ))
          )}
        </div>
      )}
    </div>
  );
}

// ==================== Main Component ====================

export default function StrategyFirstView({
  defaultExpanded = true,
  className = '',
  onCoinSelect,
  onStrategySelect,
  tradingEnabled = true,
}: StrategyFirstViewProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [viewMode, setViewMode] = useState<'grouped' | 'flat'>('grouped');
  // Track if auto-expansion has occurred (to respect user's manual collapse)
  const autoExpandedOnDataRef = useRef(false);

  const {
    strategies: rawStrategies,
    byMode: rawByMode,
    stats: rawStats,
    isLoading,
    isRealTime,
    error,
    refresh,
    // Note: nextCandleClose and lookingFor are now passed per-strategy to StrategyCard
  } = useEntryDecisionStrategies();

  // Always filter out position_running coins from strategy display
  const strategies = useMemo(() => {
    // Always filter out position_running coins from strategy display
    // When a position is open, the strategy is paused - don't show its coins here
    // Position data is shown in TradeLifecycle and CoinProfiler position sections
    const filtered = rawStrategies.map(strategy => ({
      ...strategy,
      coins: strategy.coins.filter(coin => coin.status !== 'position_running'),
    }));

    if (tradingEnabled) {
      // When trading is ON, show all non-position strategies (even if empty)
      return filtered;
    }

    // When trading is OFF, only show strategies that still have coins
    return filtered.filter(strategy => strategy.coins.length > 0);
  }, [rawStrategies, tradingEnabled]);

  const byMode = useMemo(() => {
    // Always filter out position_running coins from strategy display
    const filtered = rawByMode.map(modeGroup => ({
      ...modeGroup,
      strategies: modeGroup.strategies.map(strategy => ({
        ...strategy,
        coins: strategy.coins.filter(coin => coin.status !== 'position_running'),
      })),
    }));

    if (tradingEnabled) {
      return filtered;
    }

    // When trading is OFF, also filter out empty strategies and modes
    return filtered.map(modeGroup => ({
      ...modeGroup,
      strategies: modeGroup.strategies.filter(strategy => strategy.coins.length > 0),
    })).filter(modeGroup => modeGroup.strategies.length > 0);
  }, [rawByMode, tradingEnabled]);

  // Recalculate stats based on filtered data
  const stats = useMemo(() => {
    if (tradingEnabled) return rawStats;

    const totalCoinsReady = strategies.reduce((sum, s) =>
      sum + s.coins.filter(c => c.status === 'ready' || c.ready === true).length, 0);
    const totalCoinsWatching = strategies.reduce((sum, s) =>
      sum + s.coins.filter(c => c.status !== 'ready' && c.ready !== true).length, 0);

    return {
      ...rawStats,
      enabledStrategies: strategies.length,
      totalCoinsReady,
      totalCoinsWatching,
    };
  }, [rawStats, strategies, tradingEnabled]);

  // Group strategies by mode if not already grouped
  const groupedByMode = byMode.length > 0
    ? byMode
    : strategies.reduce((acc, strategy) => {
        const existingMode = acc.find(m => m.mode === strategy.mode);
        if (existingMode) {
          existingMode.strategies.push(strategy);
        } else {
          acc.push({
            mode: strategy.mode,
            strategies: [strategy],
            enabled: true,
          });
        }
        return acc;
      }, [] as ModeStrategies[]);

  // Sort modes: scalp, swing, position, ultra_fast
  const sortedModes = [...groupedByMode].sort((a, b) => {
    const order = ['scalp', 'swing', 'position', 'ultra_fast'];
    return order.indexOf(a.mode) - order.indexOf(b.mode);
  });

  // Auto-expand when strategies with coins are received for the first time
  useEffect(() => {
    const totalCoins = strategies.reduce((sum, s) => sum + s.coins.length, 0);
    if (totalCoins > 0 && !autoExpandedOnDataRef.current) {
      setExpanded(true);
      autoExpandedOnDataRef.current = true;
    }
  }, [strategies]);

  return (
    <div className={`bg-gray-800 rounded-lg border border-gray-700 ${className}`}>
      {/* Header - Always visible */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-700/30 transition-colors"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronRight className="w-5 h-5 text-gray-400" />
          )}
          <Layers className="w-5 h-5 text-cyan-400" />
          <span className="font-semibold text-white">Entry Decision Strategies</span>
          <span className="text-xs text-gray-400 bg-gray-700 px-2 py-0.5 rounded">
            {stats.enabledStrategies} strateg{stats.enabledStrategies !== 1 ? 'ies' : 'y'}
          </span>
          {/* Real-time indicator */}
          {isRealTime ? (
            <span className="flex items-center gap-1 text-xs text-green-400">
              <Wifi className="w-3 h-3" />
              Live
            </span>
          ) : (
            <span className="flex items-center gap-1 text-xs text-gray-500">
              <WifiOff className="w-3 h-3" />
              Polling
            </span>
          )}
          {/* NOTE: Countdown timer moved to StrategyCard for per-strategy display */}
        </div>

        <div className="flex items-center gap-3">
          {isLoading && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
          {!isLoading && (
            <div className="flex items-center gap-2 text-xs">
              <span className="text-green-400">{stats.totalCoinsReady} ready</span>
              <span className="text-gray-500">|</span>
              <span className="text-gray-400">{stats.totalCoinsWatching} watching</span>
            </div>
          )}
        </div>
      </button>

      {/* Content - Expanded view */}
      {expanded && (
        <div className="border-t border-gray-700">
          {/* Error State */}
          {error && (
            <div className="px-4 py-3 bg-yellow-500/10 border-b border-yellow-500/30">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-yellow-400">
                  <AlertTriangle className="w-4 h-4" />
                  <span className="text-sm">{error}</span>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    refresh();
                  }}
                  className="text-yellow-300 hover:text-yellow-200 text-sm"
                >
                  Retry
                </button>
              </div>
            </div>
          )}

          {/* Loading State */}
          {isLoading && strategies.length === 0 && (
            <div className="flex items-center justify-center py-8 text-gray-400">
              <RefreshCw className="w-5 h-5 animate-spin mr-2" />
              Loading strategies...
            </div>
          )}

          {/* Empty State - Trading Paused */}
          {!isLoading && strategies.length === 0 && !error && !tradingEnabled && (
            <div className="text-center py-8">
              <div className="w-16 h-16 mx-auto mb-3 rounded-full bg-yellow-500/20 flex items-center justify-center">
                <Pause className="w-8 h-8 text-yellow-400" />
              </div>
              <p className="text-yellow-400 font-medium">Trading Paused</p>
              <p className="text-sm text-gray-500 mt-1">
                New entry signals are paused. Only active positions are being monitored.
              </p>
              <p className="text-xs text-gray-600 mt-2">
                Click the ON button above to resume strategy scanning.
              </p>
            </div>
          )}

          {/* Empty State - No Strategies */}
          {!isLoading && strategies.length === 0 && !error && tradingEnabled && (
            <div className="text-center py-8">
              <Layers className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No strategies configured</p>
              <p className="text-sm text-gray-500 mt-1">
                Enable strategies in Settings to see entry opportunities.
              </p>
            </div>
          )}

          {/* View toggle and content */}
          {strategies.length > 0 && (
            <>
              {/* View toggle */}
              <div className="px-4 py-2 border-b border-gray-700/50 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setViewMode('grouped');
                    }}
                    className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${
                      viewMode === 'grouped'
                        ? 'bg-cyan-500/20 text-cyan-400'
                        : 'text-gray-400 hover:text-white'
                    }`}
                  >
                    <LayoutGrid className="w-3 h-3" />
                    By Mode
                  </button>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setViewMode('flat');
                    }}
                    className={`flex items-center gap-1 px-2 py-1 rounded text-xs transition-colors ${
                      viewMode === 'flat'
                        ? 'bg-cyan-500/20 text-cyan-400'
                        : 'text-gray-400 hover:text-white'
                    }`}
                  >
                    <List className="w-3 h-3" />
                    All
                  </button>
                </div>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    refresh();
                  }}
                  disabled={isLoading}
                  className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-white transition-colors disabled:opacity-50"
                >
                  <RefreshCw className={`w-3 h-3 ${isLoading ? 'animate-spin' : ''}`} />
                  Refresh
                </button>
              </div>

              {/* Grouped View */}
              {viewMode === 'grouped' && (
                <div className="p-4 space-y-4 max-h-[600px] overflow-y-auto">
                  {sortedModes.map((modeData) => (
                    <ModeSection
                      key={modeData.mode}
                      modeData={modeData}
                      defaultExpanded={modeData.strategies.some(s =>
                        s.coins.some(c => c.status === 'ready' || c.ready === true)
                      )}
                      onCoinSelect={onCoinSelect}
                    />
                  ))}
                </div>
              )}

              {/* Flat View */}
              {viewMode === 'flat' && (
                <div className="p-4 space-y-2 max-h-[600px] overflow-y-auto">
                  {strategies.map((strategy) => (
                    <StrategyCard
                      key={`${strategy.mode}-${strategy.strategy}-${strategy.sub_strategy}`}
                      strategy={strategy}
                      defaultExpanded={strategy.coins.some(c => c.status === 'ready' || c.ready === true)}
                      onCoinSelect={onCoinSelect}
                    />
                  ))}
                </div>
              )}
            </>
          )}

        </div>
      )}
    </div>
  );
}
