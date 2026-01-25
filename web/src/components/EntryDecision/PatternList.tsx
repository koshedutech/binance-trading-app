// Pattern State List Container
// Epic 11: Position Decision Engine - Story 11.46: Volume Imbalance UI Components
// Displays all active Volume Imbalance patterns in a filterable list view

import React, { useState, useMemo, useCallback } from 'react';
import {
  RefreshCw,
  AlertTriangle,
  Circle,
  Clock,
  Zap,
  Filter,
  Layers,
  Wifi,
  WifiOff,
  CheckCircle,
  XCircle,
} from 'lucide-react';
import { useVolumeImbalancePatterns, usePatternAction } from '../../hooks/useStrategyHierarchy';
import VolumeImbalanceCard from '../EntryDecisionEngine/VolumeImbalanceCard';
import type { VolumeImbalanceState, VolumeImbalancePattern } from '../../types/strategyHierarchy';

// ==================== Interfaces ====================

interface PatternListProps {
  /** Optional CSS class */
  className?: string;
  /** Whether to show only ready patterns */
  readyOnly?: boolean;
  /** Maximum patterns to display */
  maxPatterns?: number;
  /** Callback when a pattern is executed */
  onPatternExecute?: (patternId: string) => void;
  /** Callback when a pattern is skipped */
  onPatternSkip?: (patternId: string, reason?: string) => void;
}

type FilterState = 'ALL' | VolumeImbalanceState;

// ==================== Filter Button Component ====================

interface FilterButtonProps {
  state: FilterState;
  activeFilter: FilterState;
  count: number;
  onClick: (state: FilterState) => void;
  icon: React.ReactNode;
  label: string;
  colorClass: string;
}

function FilterButton({
  state,
  activeFilter,
  count,
  onClick,
  icon,
  label,
  colorClass,
}: FilterButtonProps) {
  const isActive = activeFilter === state;

  return (
    <button
      type="button"
      onClick={() => onClick(state)}
      className={`
        flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all
        ${isActive
          ? `${colorClass} ring-1 ring-current`
          : 'text-gray-400 hover:text-gray-200 bg-gray-700/30 hover:bg-gray-700/50'
        }
      `}
    >
      {icon}
      <span>{label}</span>
      <span className={`
        ml-1 px-1.5 py-0.5 rounded text-[10px]
        ${isActive ? 'bg-white/20' : 'bg-gray-600/50'}
      `}>
        {count}
      </span>
    </button>
  );
}

// ==================== Summary Header Component ====================

interface SummaryHeaderProps {
  stats: {
    watching: number;
    consolidating: number;
    ready: number;
    expired_today: number;
  };
  isRealTime: boolean;
  isLoading: boolean;
  onRefresh: () => void;
}

function SummaryHeader({ stats, isRealTime, isLoading, onRefresh }: SummaryHeaderProps) {
  const total = stats.watching + stats.consolidating + stats.ready;

  return (
    <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <Layers className="w-5 h-5 text-cyan-400" />
          <span className="font-semibold text-white">Volume Imbalance Patterns</span>
        </div>

        {/* Summary counts */}
        <div className="flex items-center gap-3 text-xs">
          <span className="flex items-center gap-1 text-gray-400">
            <Circle className="w-3 h-3" />
            {stats.watching} Watching
          </span>
          <span className="text-gray-600">|</span>
          <span className="flex items-center gap-1 text-yellow-400">
            <Clock className="w-3 h-3" />
            {stats.consolidating} Consolidating
          </span>
          <span className="text-gray-600">|</span>
          <span className="flex items-center gap-1 text-green-400">
            <Zap className="w-3 h-3" />
            {stats.ready} Ready
          </span>
        </div>
      </div>

      <div className="flex items-center gap-3">
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

        {/* Refresh button */}
        <button
          type="button"
          onClick={onRefresh}
          disabled={isLoading}
          className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-white transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-3 h-3 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function PatternList({
  className = '',
  readyOnly = false,
  maxPatterns,
  onPatternExecute,
  onPatternSkip,
}: PatternListProps) {
  const [activeFilter, setActiveFilter] = useState<FilterState>(readyOnly ? 'READY' : 'ALL');

  const {
    patterns,
    stats,
    isLoading,
    isRealTime,
    error,
    refresh,
  } = useVolumeImbalancePatterns();

  const { execute, skip, isLoading: isActionLoading } = usePatternAction();

  // Filter patterns based on active filter
  const filteredPatterns = useMemo(() => {
    let result = patterns;

    // Apply state filter
    if (activeFilter !== 'ALL') {
      result = result.filter(p => p.state === activeFilter);
    }

    // Sort: Ready first, then by detected_at (newest first)
    result = [...result].sort((a, b) => {
      // Ready patterns first
      if (a.state === 'READY' && b.state !== 'READY') return -1;
      if (b.state === 'READY' && a.state !== 'READY') return 1;
      // Then by detection time (newest first)
      return b.detected_at - a.detected_at;
    });

    // Apply max limit
    if (maxPatterns && maxPatterns > 0) {
      result = result.slice(0, maxPatterns);
    }

    return result;
  }, [patterns, activeFilter, maxPatterns]);

  // Handle execute action
  const handleExecute = useCallback(async (patternId: string) => {
    await execute(patternId);
    onPatternExecute?.(patternId);
  }, [execute, onPatternExecute]);

  // Handle skip action
  const handleSkip = useCallback(async (patternId: string, reason?: string) => {
    await skip(patternId, reason);
    onPatternSkip?.(patternId, reason);
  }, [skip, onPatternSkip]);

  // Calculate filter counts
  const filterCounts = useMemo(() => ({
    ALL: patterns.length,
    WATCHING: stats.watching,
    CONSOLIDATING: stats.consolidating,
    READY: stats.ready,
    EXPIRED: stats.expired_today,
  }), [patterns.length, stats]);

  return (
    <div className={`bg-gray-800 rounded-lg border border-gray-700 ${className}`}>
      {/* Summary Header */}
      <SummaryHeader
        stats={stats}
        isRealTime={isRealTime}
        isLoading={isLoading}
        onRefresh={refresh}
      />

      {/* Filter Bar */}
      {!readyOnly && (
        <div className="flex items-center gap-2 px-4 py-3 border-b border-gray-700/50 bg-gray-900/30">
          <Filter className="w-4 h-4 text-gray-500" />
          <FilterButton
            state="ALL"
            activeFilter={activeFilter}
            count={filterCounts.ALL}
            onClick={setActiveFilter}
            icon={<Layers className="w-3 h-3" />}
            label="All"
            colorClass="text-cyan-400 bg-cyan-500/20"
          />
          <FilterButton
            state="WATCHING"
            activeFilter={activeFilter}
            count={filterCounts.WATCHING}
            onClick={setActiveFilter}
            icon={<Circle className="w-3 h-3" />}
            label="Watching"
            colorClass="text-gray-300 bg-gray-500/20"
          />
          <FilterButton
            state="CONSOLIDATING"
            activeFilter={activeFilter}
            count={filterCounts.CONSOLIDATING}
            onClick={setActiveFilter}
            icon={<Clock className="w-3 h-3" />}
            label="Consolidating"
            colorClass="text-yellow-400 bg-yellow-500/20"
          />
          <FilterButton
            state="READY"
            activeFilter={activeFilter}
            count={filterCounts.READY}
            onClick={setActiveFilter}
            icon={<Zap className="w-3 h-3" />}
            label="Ready"
            colorClass="text-green-400 bg-green-500/20"
          />
        </div>
      )}

      {/* Error State */}
      {error && (
        <div className="px-4 py-3 bg-yellow-500/10 border-b border-yellow-500/30">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 text-yellow-400">
              <AlertTriangle className="w-4 h-4" />
              <span className="text-sm">{error}</span>
            </div>
            <button
              type="button"
              onClick={refresh}
              className="text-yellow-300 hover:text-yellow-200 text-sm"
            >
              Retry
            </button>
          </div>
        </div>
      )}

      {/* Loading State */}
      {isLoading && patterns.length === 0 && (
        <div className="flex items-center justify-center py-12 text-gray-400">
          <RefreshCw className="w-5 h-5 animate-spin mr-2" />
          Loading patterns...
        </div>
      )}

      {/* Empty State */}
      {!isLoading && filteredPatterns.length === 0 && !error && (
        <div className="text-center py-12">
          {activeFilter === 'ALL' ? (
            <>
              <Layers className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No active patterns detected</p>
              <p className="text-sm text-gray-500 mt-1">
                Volume Imbalance patterns will appear here when detected.
              </p>
            </>
          ) : activeFilter === 'READY' ? (
            <>
              <CheckCircle className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No patterns ready for execution</p>
              <p className="text-sm text-gray-500 mt-1">
                {stats.consolidating > 0
                  ? `${stats.consolidating} pattern${stats.consolidating !== 1 ? 's' : ''} consolidating`
                  : 'Patterns will appear when they reach ready state.'
                }
              </p>
            </>
          ) : activeFilter === 'CONSOLIDATING' ? (
            <>
              <Clock className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No patterns consolidating</p>
              <p className="text-sm text-gray-500 mt-1">
                Patterns move to consolidating after initial detection.
              </p>
            </>
          ) : activeFilter === 'WATCHING' ? (
            <>
              <Circle className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No patterns being watched</p>
              <p className="text-sm text-gray-500 mt-1">
                New patterns appear here first when detected.
              </p>
            </>
          ) : (
            <>
              <XCircle className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No expired patterns today</p>
            </>
          )}
        </div>
      )}

      {/* Pattern List */}
      {filteredPatterns.length > 0 && (
        <div className="p-4 space-y-3 max-h-[600px] overflow-y-auto">
          {filteredPatterns.map((pattern) => (
            <VolumeImbalanceCard
              key={pattern.id}
              pattern={pattern}
              onExecute={handleExecute}
              onSkip={handleSkip}
              isLoading={isActionLoading}
              disabled={isActionLoading}
            />
          ))}
        </div>
      )}

      {/* Footer with stats */}
      {filteredPatterns.length > 0 && (
        <div className="px-4 py-2 border-t border-gray-700 bg-gray-900/30 flex items-center justify-between">
          <div className="text-xs text-gray-500">
            Showing {filteredPatterns.length} of {patterns.length} patterns
            {maxPatterns && patterns.length > maxPatterns && (
              <span className="ml-1">(limited to {maxPatterns})</span>
            )}
          </div>
          {stats.expired_today > 0 && (
            <div className="text-xs text-gray-500">
              {stats.expired_today} expired today
            </div>
          )}
        </div>
      )}
    </div>
  );
}
