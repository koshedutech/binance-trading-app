// Strategy Card Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Displays individual strategy with matching coins, expandable view

import React, { useState } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Activity,
  BarChart2,
  Clock,
  Target,
  Layers,
  TrendingUp,
  TrendingDown,
  CheckCircle,
  AlertCircle,
} from 'lucide-react';
import PatternProgress, { PatternProgressCompact } from './PatternProgress';
import ScoreDisplay, { ScoreDisplayCompact } from './ScoreDisplay';
import type {
  StrategyMatch,
  CoinMatch,
  StrategyType,
  PatternStatus,
  formatStrategyName,
  isCoinReady,
  getReadyCount,
  getWatchingCount,
} from '../../types/entryDecision';

// ==================== Interfaces ====================

interface StrategyCardProps {
  /** Strategy data */
  strategy: StrategyMatch;
  /** Whether initially expanded */
  defaultExpanded?: boolean;
  /** Callback when a coin is selected */
  onCoinSelect?: (symbol: string, strategy: StrategyMatch) => void;
  /** Show compact view */
  compact?: boolean;
}

interface CoinRowProps {
  /** Coin match data */
  coin: CoinMatch;
  /** Strategy type */
  strategyType: StrategyType;
  /** Score threshold (for score-based) */
  threshold?: number;
  /** Click handler */
  onClick?: () => void;
}

// ==================== Helper Functions ====================

/**
 * Get strategy type icon
 */
function getStrategyTypeIcon(type: StrategyType): React.ReactNode {
  switch (type) {
    case 'pattern':
      return <Activity className="w-4 h-4" />;
    case 'score':
      return <BarChart2 className="w-4 h-4" />;
    default:
      return <Target className="w-4 h-4" />;
  }
}

/**
 * Format strategy name for display
 */
function formatName(strategy: string, subStrategy?: string): string {
  const formatted = strategy.split('_').map(word =>
    word.charAt(0).toUpperCase() + word.slice(1)
  ).join(' ');

  if (subStrategy) {
    const subFormatted = subStrategy.split('_').map(word =>
      word.charAt(0).toUpperCase() + word.slice(1)
    ).join(' ');
    return `${formatted} - ${subFormatted}`;
  }

  return formatted;
}

/**
 * Check if coin is ready
 */
function checkCoinReady(coin: CoinMatch): boolean {
  if (coin.status) {
    return coin.status === 'ready';
  }
  return coin.ready === true;
}

/**
 * Count ready coins
 */
function countReady(coins: CoinMatch[]): number {
  return coins.filter(checkCoinReady).length;
}

/**
 * Count watching coins
 */
function countWatching(coins: CoinMatch[]): number {
  return coins.length - countReady(coins);
}

// ==================== Coin Row Component ====================

function CoinRow({
  coin,
  strategyType,
  threshold = 55,
  onClick,
}: CoinRowProps) {
  const isReady = checkCoinReady(coin);

  return (
    <button
      type="button"
      onClick={onClick}
      className={`
        w-full flex items-center justify-between p-3 rounded-lg transition-colors
        ${isReady
          ? 'bg-green-500/10 hover:bg-green-500/20 border border-green-500/30'
          : 'bg-gray-800/50 hover:bg-gray-700/50 border border-gray-700/50'
        }
      `}
    >
      {/* Left side: Symbol and direction */}
      <div className="flex items-center gap-3">
        <span className={`font-bold ${isReady ? 'text-green-400' : 'text-white'}`}>
          {coin.symbol}
        </span>

        {/* Direction indicator */}
        {coin.direction && (
          <span className={`flex items-center gap-1 ${
            coin.direction === 'long' ? 'text-green-400' :
            coin.direction === 'short' ? 'text-red-400' :
            'text-gray-400'
          }`}>
            {coin.direction === 'long' && <TrendingUp className="w-3 h-3" />}
            {coin.direction === 'short' && <TrendingDown className="w-3 h-3" />}
            <span className="text-xs uppercase">{coin.direction}</span>
          </span>
        )}

        {/* Ready badge */}
        {isReady && (
          <span className="flex items-center gap-1 px-2 py-0.5 bg-green-500/20 text-green-400 text-xs rounded-full">
            <CheckCircle className="w-3 h-3" />
            Ready
          </span>
        )}
      </div>

      {/* Right side: Progress/Score display */}
      <div className="flex items-center gap-4">
        {strategyType === 'pattern' && coin.step !== undefined && coin.status && (
          <PatternProgressCompact
            currentStep={coin.step}
            totalSteps={3}
            status={coin.status}
          />
        )}

        {strategyType === 'score' && coin.score !== undefined && (
          <ScoreDisplayCompact
            score={coin.score}
            threshold={threshold}
            ready={coin.ready}
          />
        )}

        {/* Price */}
        {coin.current_price && (
          <span className="text-xs text-gray-500">
            ${coin.current_price.toFixed(4)}
          </span>
        )}
      </div>
    </button>
  );
}

// ==================== Main Component ====================

export default function StrategyCard({
  strategy,
  defaultExpanded = false,
  onCoinSelect,
  compact = false,
}: StrategyCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const readyCount = countReady(strategy.coins);
  const watchingCount = countWatching(strategy.coins);
  const hasReadyCoins = readyCount > 0;

  // Sort coins: ready first, then by step/score
  const sortedCoins = [...strategy.coins].sort((a, b) => {
    const aReady = checkCoinReady(a);
    const bReady = checkCoinReady(b);
    if (aReady && !bReady) return -1;
    if (!aReady && bReady) return 1;

    // For pattern-based, sort by step (higher = closer to ready)
    if (strategy.type === 'pattern') {
      return (b.step || 0) - (a.step || 0);
    }
    // For score-based, sort by score (higher = closer to ready)
    return (b.score || 0) - (a.score || 0);
  });

  return (
    <div className={`
      bg-gray-900/50 rounded-lg border overflow-hidden
      ${hasReadyCoins
        ? 'border-green-500/30 shadow-md shadow-green-500/5'
        : 'border-gray-700/50'
      }
    `}>
      {/* Header - Always visible */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between p-4 hover:bg-gray-800/30 transition-colors"
      >
        <div className="flex items-center gap-3">
          {/* Expand icon */}
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}

          {/* Strategy type icon */}
          <span className={`${hasReadyCoins ? 'text-green-400' : 'text-purple-400'}`}>
            {getStrategyTypeIcon(strategy.type)}
          </span>

          {/* Strategy name */}
          <div className="text-left">
            <span className="font-medium text-white">
              {formatName(strategy.strategy, strategy.sub_strategy)}
            </span>
            {!compact && (
              <div className="flex items-center gap-2 text-xs text-gray-500 mt-0.5">
                <span className="flex items-center gap-1">
                  <Clock className="w-3 h-3" />
                  {strategy.timeframe}
                </span>
                <span className="flex items-center gap-1">
                  <Layers className="w-3 h-3" />
                  {strategy.type === 'pattern' ? 'Pattern' : 'Score'}
                </span>
                {strategy.threshold && (
                  <span className="flex items-center gap-1">
                    <Target className="w-3 h-3" />
                    Threshold: {strategy.threshold}
                  </span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Right side: Coin counts */}
        <div className="flex items-center gap-3">
          {hasReadyCoins && (
            <span className="flex items-center gap-1 px-2 py-1 bg-green-500/20 text-green-400 text-xs rounded-full">
              <CheckCircle className="w-3 h-3" />
              {readyCount} ready
            </span>
          )}
          {watchingCount > 0 && (
            <span className="flex items-center gap-1 text-xs text-gray-400">
              <AlertCircle className="w-3 h-3" />
              {watchingCount} watching
            </span>
          )}
        </div>
      </button>

      {/* Expanded content: Coin list */}
      {expanded && strategy.coins.length > 0 && (
        <div className="border-t border-gray-700/50 p-4 space-y-2">
          {sortedCoins.map((coin) => (
            <CoinRow
              key={coin.symbol}
              coin={coin}
              strategyType={strategy.type}
              threshold={strategy.threshold}
              onClick={() => onCoinSelect?.(coin.symbol, strategy)}
            />
          ))}
        </div>
      )}

      {/* Empty state */}
      {expanded && strategy.coins.length === 0 && (
        <div className="border-t border-gray-700/50 p-4 text-center">
          <AlertCircle className="w-8 h-8 mx-auto mb-2 text-gray-600" />
          <p className="text-sm text-gray-500">No coins being tracked</p>
          <p className="text-xs text-gray-600 mt-1">
            Coins will appear here when they match this strategy's criteria
          </p>
        </div>
      )}
    </div>
  );
}

// ==================== Compact Strategy Badge ====================

interface StrategyBadgeProps {
  /** Strategy data */
  strategy: StrategyMatch;
  /** Click handler */
  onClick?: () => void;
}

/**
 * Compact badge showing strategy summary
 */
export function StrategyBadge({ strategy, onClick }: StrategyBadgeProps) {
  const readyCount = countReady(strategy.coins);
  const hasReady = readyCount > 0;

  return (
    <button
      type="button"
      onClick={onClick}
      className={`
        inline-flex items-center gap-2 px-3 py-1.5 rounded-lg border transition-colors
        ${hasReady
          ? 'bg-green-500/10 border-green-500/30 hover:bg-green-500/20'
          : 'bg-gray-800/50 border-gray-700/50 hover:bg-gray-700/50'
        }
      `}
    >
      <span className={hasReady ? 'text-green-400' : 'text-purple-400'}>
        {getStrategyTypeIcon(strategy.type)}
      </span>
      <span className="text-sm text-white">
        {formatName(strategy.strategy)}
      </span>
      <span className={`text-xs ${hasReady ? 'text-green-400' : 'text-gray-500'}`}>
        ({strategy.coins.length})
      </span>
    </button>
  );
}
