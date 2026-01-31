// Strategy Card Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Displays individual strategy with matching coins, expandable view

import React, { useState, useEffect } from 'react';
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
  Info,
  ArrowUpDown,
  Volume2,
} from 'lucide-react';
import PatternProgress, { PatternProgressCompact } from './PatternProgress';
import ScoreDisplay, { ScoreDisplayCompact } from './ScoreDisplay';
import RequirementsPanel from './RequirementsPanel';
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

/**
 * Format elapsed time in human-readable format
 */
function formatTimeElapsed(seconds: number): string {
  if (seconds < 60) {
    return `${Math.floor(seconds)}s`;
  }
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return secs > 0 ? `${mins}m ${secs}s` : `${mins}m`;
  }
  const hours = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  return mins > 0 ? `${hours}h ${mins}m` : `${hours}h`;
}

// ==================== Countdown Timer Component ====================

interface CountdownTimerProps {
  /** Target timestamp for countdown */
  targetTime: Date | null;
  /** What direction we're looking for */
  lookingFor: string | null;
  /** Whether this is compact mode */
  compact?: boolean;
}

function CountdownTimer({ targetTime, lookingFor, compact = false }: CountdownTimerProps) {
  const [secondsRemaining, setSecondsRemaining] = useState<number>(0);

  useEffect(() => {
    if (!targetTime) {
      setSecondsRemaining(0);
      return;
    }

    const updateCountdown = () => {
      const now = new Date();
      const diff = Math.max(0, Math.floor((targetTime.getTime() - now.getTime()) / 1000));
      setSecondsRemaining(diff);
    };

    // Update immediately
    updateCountdown();

    // Update every second
    const timer = setInterval(updateCountdown, 1000);

    return () => clearInterval(timer);
  }, [targetTime]);

  if (!targetTime) {
    return null;
  }

  const mins = Math.floor(secondsRemaining / 60);
  const secs = Math.floor(secondsRemaining % 60);
  const isExpiring = secondsRemaining > 0 && secondsRemaining <= 30;

  if (compact) {
    return (
      <div className="flex items-center gap-2">
        <div className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${
          isExpiring
            ? 'bg-yellow-500/20 text-yellow-400 animate-pulse'
            : 'bg-gray-700/50 text-gray-400'
        }`}>
          <Clock className="w-3 h-3" />
          <span className="font-mono">{mins}:{secs.toString().padStart(2, '0')}</span>
        </div>
        {lookingFor && (
          <span className={`text-xs uppercase ${
            lookingFor === 'long' ? 'text-green-400' :
            lookingFor === 'short' ? 'text-red-400' :
            'text-purple-400'
          }`}>
            {lookingFor === 'long' && <TrendingUp className="w-3 h-3 inline" />}
            {lookingFor === 'short' && <TrendingDown className="w-3 h-3 inline" />}
            {lookingFor === 'both' && <ArrowUpDown className="w-3 h-3 inline" />}
          </span>
        )}
      </div>
    );
  }

  return (
    <div className="flex items-center gap-3">
      {/* Countdown Timer */}
      <div className={`flex items-center gap-1.5 px-2 py-1 rounded ${
        isExpiring
          ? 'bg-yellow-500/20 text-yellow-400 animate-pulse'
          : 'bg-gray-700/50 text-gray-300'
      }`}>
        <Clock className="w-3.5 h-3.5" />
        <span className="font-mono text-sm font-medium">
          {mins}:{secs.toString().padStart(2, '0')}
        </span>
      </div>

      {/* Looking For Direction */}
      {lookingFor && (
        <div className={`flex items-center gap-1 px-2 py-1 rounded text-xs ${
          lookingFor === 'long'
            ? 'bg-green-500/20 text-green-400'
            : lookingFor === 'short'
              ? 'bg-red-500/20 text-red-400'
              : 'bg-purple-500/20 text-purple-400'
        }`}>
          {lookingFor === 'long' && <TrendingUp className="w-3 h-3" />}
          {lookingFor === 'short' && <TrendingDown className="w-3 h-3" />}
          {lookingFor === 'both' && <ArrowUpDown className="w-3 h-3" />}
          <span className="uppercase font-medium">{lookingFor}</span>
        </div>
      )}
    </div>
  );
}

// ==================== Volume Display Helper ====================

function formatVolume(volume: number): string {
  if (volume >= 1000000) {
    return `${(volume / 1000000).toFixed(2)}M`;
  }
  if (volume >= 1000) {
    return `${(volume / 1000).toFixed(1)}K`;
  }
  return volume.toFixed(2);
}

// ==================== Stage Labels ====================

const STAGE_LABELS: Record<number, { name: string; description: string; color: string }> = {
  1: {
    name: 'Volume Spike',
    description: 'Reference candle detected',
    color: 'text-blue-400',
  },
  2: {
    name: 'Breakout',
    description: 'Watching for breakout entry',
    color: 'text-yellow-400',
  },
};

const STATUS_STAGE_LABELS: Record<string, { label: string; description: string }> = {
  watching: { label: 'Scanning', description: 'Looking for volume spike' },
  accumulation: { label: 'Step 1 Complete', description: 'Reference candle found, watching for breakout' },
  consolidating: { label: 'Consolidating', description: 'Price consolidating near reference' },
  ready: { label: 'Ready', description: 'Pattern complete - entry signal' },
  failed: { label: 'Failed', description: 'Pattern invalidated' },
  expired: { label: 'Expired', description: 'Pattern timed out' },
};

// ==================== Coin Row Component ====================

function CoinRow({
  coin,
  strategyType,
  threshold = 55,
  onClick,
}: CoinRowProps) {
  const isReady = checkCoinReady(coin);
  const isActive = coin.status === 'accumulation' || coin.status === 'consolidating';
  const stageInfo = STAGE_LABELS[coin.step || 0];
  const statusInfo = STATUS_STAGE_LABELS[coin.status || 'watching'];

  return (
    <button
      type="button"
      onClick={onClick}
      className={`
        w-full flex flex-col p-3 rounded-lg transition-colors text-left
        ${isReady
          ? 'bg-green-500/10 hover:bg-green-500/20 border border-green-500/30'
          : isActive
            ? 'bg-yellow-500/5 hover:bg-yellow-500/10 border border-yellow-500/20'
            : 'bg-gray-800/50 hover:bg-gray-700/50 border border-gray-700/50'
        }
      `}
    >
      {/* Top Row: Symbol, Direction, Status */}
      <div className="flex items-center justify-between w-full">
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

        {/* Progress/Score display */}
        <div className="flex items-center gap-4">
          {strategyType === 'pattern' && coin.step !== undefined && coin.status && (
            <PatternProgressCompact
              currentStep={coin.step}
              totalSteps={coin.total_steps || 2} // Use from coin, fallback to 2-step
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
              ${coin.current_price.toFixed(coin.current_price > 100 ? 2 : 4)}
            </span>
          )}
        </div>
      </div>

      {/* Volume Info Row - shown for pattern strategies */}
      {strategyType === 'pattern' && (coin.current_volume || coin.volume_multiplier) && (
        <div className="mt-1.5 flex items-center gap-3 text-xs">
          {/* Current Volume */}
          {coin.current_volume && (
            <div className="flex items-center gap-1 text-gray-400">
              <Volume2 className="w-3 h-3" />
              <span>Vol: {formatVolume(coin.current_volume)}</span>
            </div>
          )}

          {/* Volume Multiplier */}
          {coin.volume_multiplier && (
            <span className={`font-mono ${
              coin.volume_multiplier >= 3.0 ? 'text-green-400' :
              coin.volume_multiplier >= 2.0 ? 'text-yellow-400' :
              'text-gray-500'
            }`}>
              {coin.volume_multiplier.toFixed(1)}x avg
            </span>
          )}

          {/* Volume Distance from Threshold */}
          {coin.volume_distance_percent !== undefined && (
            <span className={`font-mono ${
              coin.volume_distance_percent >= 0 ? 'text-green-400' : 'text-gray-500'
            }`}>
              {coin.volume_distance_percent >= 0 ? '+' : ''}{coin.volume_distance_percent.toFixed(1)}% threshold
            </span>
          )}

          {/* Price Distance from Entry */}
          {coin.price_distance_percent !== undefined && (
            <span className={`font-mono ${
              coin.price_distance_percent >= 0 ? 'text-green-400' :
              coin.price_distance_percent > -0.5 ? 'text-yellow-400' :
              'text-gray-500'
            }`}>
              {coin.price_distance_percent >= 0 ? '↑' : '↓'} {Math.abs(coin.price_distance_percent).toFixed(2)}% entry
            </span>
          )}
        </div>
      )}

      {/* Bottom Row: Stage Details (only for pattern strategies with active tracking) */}
      {strategyType === 'pattern' && coin.step !== undefined && coin.step > 0 && (
        <div className="mt-2 pt-2 border-t border-gray-700/30 space-y-2">
          {/* Row 1: Step info and progress */}
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              {coin.step_details && coin.step_details[coin.step - 1] ? (
                <>
                  <span className={`text-xs font-medium ${stageInfo?.color || 'text-gray-400'}`}>
                    Step {coin.step}: {coin.step_details[coin.step - 1].name}
                  </span>
                  <span className="text-xs text-gray-500">
                    {coin.step_details[coin.step - 1].completed
                      ? `✓ ${coin.step_details[coin.step - 1].progress}`
                      : coin.step_details[coin.step - 1].progress || statusInfo?.description
                    }
                  </span>
                </>
              ) : (
                <>
                  <span className={`text-xs font-medium ${stageInfo?.color || 'text-gray-400'}`}>
                    Step {coin.step}: {stageInfo?.name || 'Unknown'}
                  </span>
                  <span className="text-xs text-gray-500">
                    {statusInfo?.description || coin.details || ''}
                  </span>
                </>
              )}
            </div>

            {/* Additional details from step_details */}
            {coin.step_details && coin.step_details[coin.step - 1]?.details && (
              <span className="text-xs text-gray-400 bg-gray-700/50 px-2 py-0.5 rounded">
                {coin.step_details[coin.step - 1].details}
              </span>
            )}
          </div>

          {/* Row 2: Reference candle and Entry candle side by side */}
          {(coin.reference_candle || coin.entry_candle) && (
            <div className="flex flex-col gap-1.5 bg-gray-900/50 rounded px-2 py-1.5">
              {/* Two-column layout: Reference (left) | Entry (right) */}
              <div className="grid grid-cols-2 gap-3 border-b border-gray-700/50 pb-1.5">
                {/* LEFT: Reference Candle */}
                {coin.reference_candle && coin.reference_candle.open_time && (
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center gap-1.5 text-xs">
                      <span className="text-purple-400 font-medium">📌 Reference Candle</span>
                    </div>
                    <div className="flex flex-wrap items-center gap-1.5 text-xs">
                      <span className="text-yellow-400 font-mono">
                        {new Date(coin.reference_candle.open_time).toISOString().slice(5, 16).replace('T', ' ')} UTC
                      </span>
                      <span className="text-gray-500">|</span>
                      <span className="text-cyan-400 font-medium">
                        {coin.reference_candle.volume_multiplier?.toFixed(1) || '?'}x vol
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-gray-400">
                        H: <span className="text-green-400 font-mono">${coin.reference_candle.high.toFixed(coin.reference_candle.high > 100 ? 2 : 4)}</span>
                      </span>
                      <span className="text-gray-400">
                        L: <span className="text-red-400 font-mono">${coin.reference_candle.low.toFixed(coin.reference_candle.low > 100 ? 2 : 4)}</span>
                      </span>
                    </div>
                  </div>
                )}

                {/* RIGHT: Entry Candle (when breakout detected) */}
                {coin.entry_candle ? (
                  <div className="flex flex-col gap-1 border-l border-gray-700/50 pl-3">
                    <div className="flex items-center gap-1.5 text-xs">
                      <span className="text-green-400 font-medium">🎯 Entry Candle</span>
                      {coin.entry_candle.direction && (
                        <span className={`uppercase text-[10px] px-1 rounded ${
                          coin.entry_candle.direction === 'long'
                            ? 'bg-green-500/20 text-green-400'
                            : 'bg-red-500/20 text-red-400'
                        }`}>
                          {coin.entry_candle.direction}
                        </span>
                      )}
                      {/* Expiry countdown */}
                      {coin.seconds_until_expiry !== undefined && coin.seconds_until_expiry > 0 && (
                        <span className={`text-[10px] px-1 rounded font-mono ${
                          coin.seconds_until_expiry <= 10
                            ? 'bg-red-500/30 text-red-400 animate-pulse'
                            : 'bg-yellow-500/20 text-yellow-400'
                        }`}>
                          Expires: {coin.seconds_until_expiry}s
                        </span>
                      )}
                    </div>
                    <div className="flex flex-wrap items-center gap-1.5 text-xs">
                      <span className="text-yellow-400 font-mono">
                        {new Date(coin.entry_candle.detected_at).toISOString().slice(5, 16).replace('T', ' ')} UTC
                      </span>
                      <span className="text-gray-500">|</span>
                      <span className="text-cyan-400 font-medium">
                        {coin.entry_candle.volume_multiplier?.toFixed(1) || '?'}x vol
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="text-gray-400">
                        H: <span className="text-green-400 font-mono">${coin.entry_candle.high.toFixed(coin.entry_candle.high > 100 ? 2 : 4)}</span>
                      </span>
                      <span className="text-gray-400">
                        L: <span className="text-red-400 font-mono">${coin.entry_candle.low.toFixed(coin.entry_candle.low > 100 ? 2 : 4)}</span>
                      </span>
                    </div>
                    {/* CRITICAL: Entry Price sent for order */}
                    <div className="flex items-center gap-1 text-xs mt-0.5 bg-green-500/10 px-1.5 py-0.5 rounded">
                      <span className="text-green-300 font-medium">Entry Price:</span>
                      <span className="text-green-400 font-mono font-bold">
                        ${coin.entry_candle.entry_price.toFixed(coin.entry_candle.entry_price > 100 ? 2 : 4)}
                      </span>
                    </div>
                  </div>
                ) : coin.reference_candle && (
                  /* Placeholder when no entry yet - show proximity info */
                  <div className="flex flex-col gap-1 border-l border-gray-700/50 pl-3">
                    <div className="flex items-center gap-1.5 text-xs">
                      <span className="text-gray-500 font-medium">⏳ Awaiting Breakout</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs text-gray-500">
                      {coin.proximity_to_breakout !== undefined && (
                        <span className={`font-mono ${
                          coin.proximity_to_breakout >= 0 ? 'text-green-400' :
                          coin.proximity_to_breakout > -0.5 ? 'text-yellow-400' : 'text-gray-400'
                        }`}>
                          {coin.proximity_to_breakout >= 0 ? '+' : ''}{coin.proximity_to_breakout.toFixed(2)}%
                          {coin.proximity_to_breakout >= 0 ? ' ABOVE' : ' to entry'}
                        </span>
                      )}
                    </div>
                    {/* Potential breakout indicator - blinking */}
                    {coin.potential_breakout && (
                      <span className="flex items-center gap-1 px-1.5 py-0.5 bg-green-500/30 text-green-400 text-xs rounded animate-pulse w-fit">
                        ⚡ Potential Breakout
                      </span>
                    )}
                  </div>
                )}
              </div>

              {/* Tracking metrics row */}
              <div className="flex items-center justify-between text-xs">
                {/* Reference candle timing info */}
                <div className="flex items-center gap-3">
                  {/* Candles passed */}
                  {coin.candles_since_reference !== undefined && (
                    <span className="text-gray-400">
                      🕯 {coin.candles_since_reference} candle{coin.candles_since_reference !== 1 ? 's' : ''} passed
                    </span>
                  )}

                  {/* Time elapsed */}
                  {coin.seconds_since_reference !== undefined && coin.seconds_since_reference > 0 && (
                    <span className="text-gray-500">
                      ⏱ {formatTimeElapsed(coin.seconds_since_reference)} ago
                    </span>
                  )}
                </div>

                {/* Ready status indicator */}
                {coin.status === 'ready' && coin.ready_at && (
                  <span className="text-green-400 text-xs">
                    Ready since {new Date(coin.ready_at).toISOString().slice(11, 19)} UTC
                  </span>
                )}
              </div>
            </div>
          )}

          {/* Row 3: Breakout level reference */}
          {coin.reference_candle && coin.current_price && (
            <div className="flex items-center justify-between text-xs text-gray-500">
              <span>
                Breakout @ <span className="text-yellow-400 font-mono">
                  ${coin.reference_candle.high.toFixed(coin.reference_candle.high > 100 ? 2 : 4)}
                </span>
              </span>
              <span>
                Current: <span className="text-white font-mono">
                  ${coin.current_price.toFixed(coin.current_price > 100 ? 2 : 4)}
                </span>
              </span>
            </div>
          )}
        </div>
      )}

      {/* Watching state info */}
      {strategyType === 'pattern' && (!coin.step || coin.step === 0) && coin.status === 'watching' && (
        <div className="mt-2 pt-2 border-t border-gray-700/30">
          <span className="text-xs text-gray-500">
            🔍 Scanning for volume spike (3x average)...
          </span>
        </div>
      )}
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
  const [showRequirements, setShowRequirements] = useState(false);

  const readyCount = countReady(strategy.coins);
  const watchingCount = countWatching(strategy.coins);

  // Parse countdown timer from strategy
  const nextCandleClose = strategy.next_candle_close
    ? new Date(strategy.next_candle_close)
    : null;
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

          {/* Per-Strategy Countdown Timer */}
          {strategy.type === 'pattern' && nextCandleClose && (
            <CountdownTimer
              targetTime={nextCandleClose}
              lookingFor={strategy.looking_for || null}
              compact={true}
            />
          )}
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
          {/* Requirements toggle button */}
          <div className="flex justify-end mb-2">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setShowRequirements(!showRequirements);
              }}
              className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-white bg-gray-800/50 hover:bg-gray-700/50 rounded transition-colors"
            >
              <Info className="w-3 h-3" />
              {showRequirements ? 'Hide' : 'Show'} Requirements
            </button>
          </div>

          {/* Requirements panel */}
          {showRequirements && (
            <div className="mb-3">
              <RequirementsPanel
                strategy={strategy.strategy}
                subStrategy={strategy.sub_strategy}
                timeframe={strategy.timeframe}
                defaultExpanded={true}
              />
            </div>
          )}

          {/* Coin list */}
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
        <div className="border-t border-gray-700/50 p-4">
          {/* Requirements toggle for empty state too */}
          <div className="flex justify-end mb-2">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                setShowRequirements(!showRequirements);
              }}
              className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-white bg-gray-800/50 hover:bg-gray-700/50 rounded transition-colors"
            >
              <Info className="w-3 h-3" />
              {showRequirements ? 'Hide' : 'Show'} Requirements
            </button>
          </div>

          {/* Requirements panel */}
          {showRequirements && (
            <div className="mb-3">
              <RequirementsPanel
                strategy={strategy.strategy}
                subStrategy={strategy.sub_strategy}
                timeframe={strategy.timeframe}
                defaultExpanded={true}
              />
            </div>
          )}

          <div className="text-center">
            <AlertCircle className="w-8 h-8 mx-auto mb-2 text-gray-600" />
            <p className="text-sm text-gray-500">No coins being tracked</p>
            <p className="text-xs text-gray-600 mt-1">
              Coins will appear here when they match this strategy's criteria
            </p>
          </div>
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
