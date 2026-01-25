// Strategy First View Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Main strategy-first view with mode grouping and collapsible sections

import React, { useState, useCallback } from 'react';
import {
  ChevronDown,
  ChevronRight,
  RefreshCw,
  AlertTriangle,
  Layers,
  Zap,
  Activity,
  TrendingUp,
  Clock,
  CheckCircle,
  AlertCircle,
  Wifi,
  WifiOff,
  LayoutGrid,
  List,
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

// ==================== Mode Section Component ====================

function ModeSection({
  modeData,
  defaultExpanded = true,
  onCoinSelect,
}: ModeSectionProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const colors = MODE_COLORS_MAP[modeData.mode] || MODE_COLORS_MAP.scalp;
  const displayName = MODE_DISPLAY_NAMES_MAP[modeData.mode] || modeData.mode;
  const icon = MODE_ICONS[modeData.mode] || <Layers className="w-4 h-4" />;

  const readyCoins = countModeReadyCoins(modeData);
  const totalCoins = countModeTotalCoins(modeData);
  const hasReady = readyCoins > 0;

  // Sort strategies: those with ready coins first
  const sortedStrategies = [...modeData.strategies].sort((a, b) => {
    const aReady = a.coins.filter(c => c.status === 'ready' || c.ready === true).length;
    const bReady = b.coins.filter(c => c.status === 'ready' || c.ready === true).length;
    return bReady - aReady;
  });

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
}: StrategyFirstViewProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [viewMode, setViewMode] = useState<'grouped' | 'flat'>('grouped');

  const {
    strategies,
    byMode,
    stats,
    isLoading,
    isRealTime,
    error,
    lastUpdated,
    refresh,
  } = useEntryDecisionStrategies();

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

  const formatTime = useCallback((date: Date | null) => {
    if (!date) return 'Never';
    return date.toLocaleTimeString();
  }, []);

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

          {/* Empty State */}
          {!isLoading && strategies.length === 0 && !error && (
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

          {/* Footer */}
          <div className="px-4 py-2 border-t border-gray-700 bg-gray-900/30 flex items-center justify-between">
            <div className="text-xs text-gray-500">
              {lastUpdated && `Last updated: ${formatTime(lastUpdated)}`}
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-500">
              <Clock className="w-3 h-3" />
              <span>Auto-refreshes every 30s</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
