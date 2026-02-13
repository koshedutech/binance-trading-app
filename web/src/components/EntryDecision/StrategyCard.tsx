// Strategy Card Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Displays individual strategy with matching coins, expandable view

import React, { useState, useEffect, useRef, useMemo } from 'react';
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
import Step2ProgressBars from './Step2ProgressBars';
import type {
  StrategyMatch,
  CoinMatch,
  StrategyType,
  PatternStatus,
  PatternUpdate,
  formatStrategyName,
  isCoinReady,
  getReadyCount,
  getWatchingCount,
} from '../../types/entryDecision';

// ==================== Helper: Get UI Step Number (1-4) ====================

/**
 * Maps pattern status to 4-step UI step number
 */
function getUIStepNumber(status: PatternStatus | undefined): number {
  switch (status) {
    case 'watching':
    case 'accumulation':
      return 1; // Step 1: Reference Candle
    case 'consolidating':
      return 2; // Step 2: Consolidation
    case 'ready':
    case 'filling':
      return 3; // Step 3: Ready/Filling
    case 'position_running':
    case 'position_closed':
      return 4; // Step 4: Position
    default:
      return 1;
  }
}

// ==================== Helper: Convert CoinMatch to PatternUpdate ====================

/**
 * Converts CoinMatch data to PatternUpdate format for Step2ProgressBars
 * This allows Step2ProgressBars to be used in StrategyCard with CoinMatch data
 */
function coinMatchToPatternUpdate(coin: CoinMatch, strategyVolumeThreshold?: number): PatternUpdate {
  return {
    symbol: coin.symbol,
    timeframe: '15m', // Default, actual timeframe comes from strategy
    mode: 'scalp', // Default
    strategy: 'volume_imbalance', // Default
    sub_strategy: 'ravindra_volume_imbalance', // Default
    current_step: coin.step || 0,
    total_steps: coin.total_steps || 2,
    status: coin.status || 'watching',
    step_details: coin.step_details || [],
    reference_candle: coin.reference_candle,
    current_price: coin.current_price,
    day_high: coin.day_high,
    day_low: coin.day_low,
    // Use actual volume threshold: coin-level (real-time) > strategy config (DB) > undefined
    volume_threshold: coin.volume_threshold || strategyVolumeThreshold,
    direction: coin.direction,
    updated_at: coin.updated_at,
    // Step 2 specific fields - use current data with fallbacks
    current_candle_volume_multiplier: coin.volume_multiplier,
    // Consolidation average: midpoint between 1x (baseline) and reference volume
    consolidation_avg_volume_multiplier: coin.reference_candle
      ? (coin.reference_candle.volume_multiplier + 1) / 2
      : 1.5,
    consolidation_avg_price: coin.reference_candle
      ? (coin.reference_candle.high + coin.reference_candle.low) / 2
      : coin.current_price,
  };
}

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
  /** Volume spike multiplier from strategy config (database value) */
  strategyVolumeThreshold?: number;
  /** Strategy display name (e.g., "Ravindra Volume Imbalance") */
  strategyName?: string;
  /** Strategy timeframe (e.g., "3m", "15m") */
  timeframe?: string;
  /** Next candle close time for countdown timer */
  nextCandleClose?: Date | null;
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
 * Count coins with active positions (position_running status)
 */
function countPositionRunning(coins: CoinMatch[]): number {
  return coins.filter(c => c.status === 'position_running').length;
}

/**
 * Count watching coins (excludes ready and position_running)
 */
function countWatching(coins: CoinMatch[]): number {
  return coins.length - countReady(coins) - countPositionRunning(coins);
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

// ==================== Position Running Timer Component ====================

interface PositionRunningTimerProps {
  /** When position was opened (ISO timestamp) */
  positionOpenedAt: string;
  /** Position entry price */
  entryPrice?: number;
  /** Current price */
  currentPrice?: number;
  /** Chain ID for reference */
  chainId?: string;
  /** Trade direction (long/short) */
  direction?: string;
}

function PositionRunningTimer({ positionOpenedAt, entryPrice, currentPrice, chainId, direction }: PositionRunningTimerProps) {
  const [elapsedSeconds, setElapsedSeconds] = useState<number>(0);

  useEffect(() => {
    const openedAt = new Date(positionOpenedAt);

    const updateElapsed = () => {
      const now = new Date();
      const diff = Math.floor((now.getTime() - openedAt.getTime()) / 1000);
      setElapsedSeconds(Math.max(0, diff));
    };

    // Update immediately
    updateElapsed();

    // Update every second
    const timer = setInterval(updateElapsed, 1000);

    return () => clearInterval(timer);
  }, [positionOpenedAt]);

  // Calculate P&L if prices available (direction-aware)
  const isShort = direction === 'short';
  const pnlPercent = entryPrice && currentPrice
    ? isShort
      ? ((entryPrice - currentPrice) / entryPrice) * 100
      : ((currentPrice - entryPrice) / entryPrice) * 100
    : null;

  return (
    <div className="flex items-center gap-2">
      {/* Timer */}
      <div className="flex items-center gap-1 text-xs">
        <Clock className="w-3 h-3 text-orange-400" />
        <span className="text-gray-500">Running:</span>
        <span className="font-mono text-orange-400">{formatTimeElapsed(elapsedSeconds)}</span>
      </div>

      {/* Unrealized P&L */}
      {pnlPercent !== null && (
        <span className={`text-xs font-mono font-medium px-1 rounded ${
          pnlPercent >= 0 ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
        }`}>
          {pnlPercent >= 0 ? '+' : ''}{pnlPercent.toFixed(2)}%
        </span>
      )}
    </div>
  );
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
  strategyVolumeThreshold,
  strategyName,
  timeframe,
  nextCandleClose,
  onClick,
}: CoinRowProps) {
  // Volume threshold: prefer coin-level (from real-time data), then strategy config (from DB)
  const effectiveVolumeThreshold = coin.volume_threshold || strategyVolumeThreshold;
  const isReady = checkCoinReady(coin);
  const isActive = coin.status === 'accumulation' || coin.status === 'consolidating';
  // Position running means Chain Runner has an active position - detailed info shown in Trade Lifecycle
  const isPositionRunning = coin.status === 'position_running' || coin.has_active_position;
  const isPositionClosed = coin.status === 'position_closed';
  const stageInfo = STAGE_LABELS[coin.step || 0];
  const statusInfo = STATUS_STAGE_LABELS[coin.status || 'watching'];
  const isStageTwo = (coin.step || 0) >= 2 || !!coin.reference_candle;

  // Elapsed timer since reference candle detected (Stage 2+) - counts up continuously
  const [elapsedSinceRef, setElapsedSinceRef] = useState<number>(coin.seconds_since_reference || 0);

  useEffect(() => {
    if (!isStageTwo) return;

    const updateElapsed = () => {
      if (coin.reference_detected_at) {
        const detectedAt = new Date(coin.reference_detected_at).getTime();
        setElapsedSinceRef(Math.floor((Date.now() - detectedAt) / 1000));
      } else if (coin.seconds_since_reference !== undefined) {
        setElapsedSinceRef(prev => prev + 1);
      }
    };

    updateElapsed();
    const timer = setInterval(updateElapsed, 1000);
    return () => clearInterval(timer);
  }, [isStageTwo, coin.reference_detected_at, coin.seconds_since_reference]);

  // Candle countdown timer (per card) - counts down, resyncs from timeframe on wrap
  const calcCandleRemaining = (): number => {
    // Priority 1: Backend-provided next candle close time (if still in the future)
    if (nextCandleClose) {
      const diff = Math.floor((nextCandleClose.getTime() - Date.now()) / 1000);
      if (diff > 0) return diff;
    }
    // Priority 2: Calculate from timeframe (UTC-aligned candle intervals)
    if (timeframe) {
      const num = parseInt(timeframe);
      const unit = timeframe.replace(/[0-9]/g, '');
      let intervalMs = 0;
      if (unit === 'm') intervalMs = num * 60 * 1000;
      else if (unit === 'h') intervalMs = num * 60 * 60 * 1000;
      if (intervalMs > 0) {
        const elapsed = Date.now() % intervalMs;
        return Math.max(0, Math.floor((intervalMs - elapsed) / 1000));
      }
    }
    return 0;
  };

  const [candleCountdown, setCandleCountdown] = useState<number>(calcCandleRemaining);

  // Resync when backend sends new nextCandleClose
  useEffect(() => {
    setCandleCountdown(calcCandleRemaining());
  }, [nextCandleClose]);

  // Tick every second, resync from timeframe when hitting 0
  useEffect(() => {
    const timer = setInterval(() => {
      setCandleCountdown(prev => {
        if (prev <= 1) return calcCandleRemaining(); // Resync on wrap
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, [timeframe, nextCandleClose]);

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
      {/* Strategy Name + Candle Countdown Row */}
      <div className="flex items-center justify-between w-full mb-1.5">
        <div className="flex items-center gap-2 text-[10px]">
          {strategyName && (
            <span className="text-gray-500">{strategyName}</span>
          )}
          {timeframe && (
            <span className="px-1 py-0.5 bg-gray-700/50 rounded text-gray-400 font-mono">{timeframe}</span>
          )}
        </div>
        {/* Candle Countdown Timer - always visible */}
        {(nextCandleClose || timeframe) && candleCountdown >= 0 && (
          <div className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono ${
            candleCountdown <= 10
              ? 'bg-red-500/20 text-red-400 animate-pulse'
              : candleCountdown <= 30
                ? 'bg-orange-500/20 text-orange-400'
                : 'bg-gray-700/50 text-gray-300'
          }`}>
            <Clock className="w-2.5 h-2.5" />
            {formatTimeElapsed(candleCountdown)}
          </div>
        )}
      </div>

      {/* Top Row: Symbol, Direction, Elapsed Timer, Status */}
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

          {/* Elapsed timer since reference candle (Stage 2+) */}
          {isStageTwo && !isPositionRunning && elapsedSinceRef > 0 && (
            <span className="flex items-center gap-1 text-[10px] text-gray-400">
              <Clock className="w-2.5 h-2.5" />
              <span className="font-mono">{formatTimeElapsed(elapsedSinceRef)}</span>
              <span className="text-gray-500">elapsed</span>
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
          {coin.volume_multiplier && effectiveVolumeThreshold && (
            <span className={`font-mono ${
              coin.volume_multiplier >= effectiveVolumeThreshold ? 'text-green-400' :
              coin.volume_multiplier >= effectiveVolumeThreshold * 0.67 ? 'text-yellow-400' :
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

      {/* Bottom Row: Stage Details (only for pattern strategies with active tracking)
          When position is running, show simplified view - details are in Trade Lifecycle
          CRITICAL: Use reference_candle as primary indicator to prevent flicker during WebSocket updates
          reference_candle is preserved in merge logic, making this condition stable */}
      {strategyType === 'pattern' && (coin.reference_candle || (coin.step !== undefined && coin.step > 0)) && (
        <div className="mt-2 pt-2 border-t border-gray-700/30 space-y-2">
          {/* Position Running - Distinct Orange View */}
          {isPositionRunning ? (
            <div className="px-2 py-2 bg-orange-500/10 border border-orange-500/30 rounded space-y-1.5">
              {/* Row 1: Position badge + direction + entry price */}
              <div className="flex items-center gap-2 flex-wrap">
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-orange-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-orange-500"></span>
                </span>
                <span className="px-1.5 py-0.5 bg-orange-500/20 text-orange-400 text-xs font-bold rounded uppercase">POSITION</span>
                {coin.direction && (
                  <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-medium ${
                    coin.direction === 'long'
                      ? 'bg-green-500/20 text-green-400'
                      : 'bg-red-500/20 text-red-400'
                  }`}>
                    {coin.direction === 'long' ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
                    <span className="uppercase">{coin.direction}</span>
                  </span>
                )}
                {coin.position_entry_price && (
                  <span className="flex items-center gap-1 text-xs">
                    <span className="text-gray-500">Entry:</span>
                    <span className="text-white font-mono font-medium">
                      ${coin.position_entry_price.toFixed(coin.position_entry_price > 100 ? 2 : 4)}
                    </span>
                  </span>
                )}
                {coin.chain_id && (
                  <span className="text-orange-400 font-mono text-[10px]">{coin.chain_id}</span>
                )}
              </div>
              {/* Row 2: Reference candle details + SL/TP inline */}
              {coin.reference_candle && (
                <div className="flex items-center gap-2 text-[10px] flex-wrap">
                  <span className="text-green-400 font-medium">Ref:</span>
                  <span className="text-gray-400 font-mono">
                    {new Date(coin.reference_candle.open_time).toISOString().slice(5, 16).replace('T', ' ')} UTC
                  </span>
                  <span className="text-gray-500">H:</span>
                  <span className="text-white font-mono">{coin.reference_candle.high.toFixed(coin.reference_candle.high > 100 ? 2 : 4)}</span>
                  <span className="text-gray-500">L:</span>
                  <span className="text-white font-mono">{coin.reference_candle.low.toFixed(coin.reference_candle.low > 100 ? 2 : 4)}</span>
                  <span className="text-green-400 font-mono">{coin.reference_candle.volume_multiplier.toFixed(1)}x vol</span>
                  {coin.entry_levels && (
                    <>
                      <span className="text-gray-600">|</span>
                      <span className="text-gray-500">SL:</span>
                      <span className="text-red-400 font-mono">
                        {coin.entry_levels.stop_loss.toFixed(coin.entry_levels.stop_loss > 100 ? 2 : 4)}
                      </span>
                      <span className="text-gray-500">TP:</span>
                      <span className="text-green-400 font-mono">
                        {coin.entry_levels.take_profit.toFixed(coin.entry_levels.take_profit > 100 ? 2 : 4)}
                      </span>
                      <span className="text-gray-500">
                        R:R <span className="text-yellow-400 font-mono">{coin.entry_levels.risk_reward_ratio.toFixed(1)}</span>
                      </span>
                    </>
                  )}
                </div>
              )}
              {/* Row 3: Entry candle details */}
              {coin.entry_candle && (
                <div className="flex items-center gap-2 text-[10px]">
                  <span className="text-yellow-400 font-medium">Entry:</span>
                  <span className="text-gray-400 font-mono">
                    {new Date(coin.entry_candle.open_time).toISOString().slice(5, 16).replace('T', ' ')} UTC
                  </span>
                  <span className="text-gray-500">Price:</span>
                  <span className="text-white font-mono">{coin.entry_candle.entry_price?.toFixed(coin.entry_candle.entry_price > 100 ? 2 : 4)}</span>
                  {coin.entry_candle.direction && (
                    <span className={`ml-auto px-1 py-0.5 rounded text-[9px] font-medium ${
                      coin.entry_candle.direction === 'long' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                    }`}>{coin.entry_candle.direction.toUpperCase()}</span>
                  )}
                </div>
              )}
              {/* Row 5: Timers + PnL */}
              <div className="flex items-center justify-between text-xs">
                <div className="flex items-center gap-3">
                  {coin.seconds_ref_to_entry !== undefined && coin.seconds_ref_to_entry > 0 && (
                    <span className="flex items-center gap-1">
                      <span className="text-gray-500">Ref→Entry:</span>
                      <span className="text-orange-300 font-mono">{formatTimeElapsed(coin.seconds_ref_to_entry)}</span>
                    </span>
                  )}
                  {coin.position_opened_at && (
                    <PositionRunningTimer
                      positionOpenedAt={coin.position_opened_at}
                      entryPrice={coin.position_entry_price}
                      currentPrice={coin.current_price}
                      direction={coin.direction}
                    />
                  )}
                </div>
                {/* PnL with amount + percentage */}
                {coin.position_entry_price && coin.current_price ? (() => {
                  const isShort = coin.direction === 'short';
                  const pnlPercent = isShort
                    ? (coin.position_entry_price - coin.current_price) / coin.position_entry_price * 100
                    : (coin.current_price - coin.position_entry_price) / coin.position_entry_price * 100;
                  const qty = coin.position_quantity || 0;
                  const pnlAmount = isShort
                    ? (coin.position_entry_price - coin.current_price) * qty
                    : (coin.current_price - coin.position_entry_price) * qty;
                  const isPositive = pnlPercent >= 0;
                  return (
                    <div className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-xs font-mono font-medium ${
                      isPositive ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                    }`}>
                      <span>PnL:</span>
                      {qty > 0 && (
                        <span>{isPositive ? '+' : ''}{pnlAmount.toFixed(2)}</span>
                      )}
                      <span>({isPositive ? '+' : ''}{pnlPercent.toFixed(2)}%)</span>
                    </div>
                  );
                })() : null}
              </div>
            </div>
          ) : (
            <>
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

                {/* RIGHT: Entry Candle (when breakout detected OR status is ready) */}
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
                ) : coin.status === 'ready' ? (
                  /* Status is READY but entry_candle not populated - show ready state */
                  <div className="flex flex-col gap-1 border-l border-gray-700/50 pl-3">
                    <div className="flex items-center gap-1.5 text-xs">
                      <span className="text-green-400 font-medium animate-pulse">✅ Pattern Ready</span>
                      {coin.direction && (
                        <span className={`uppercase text-[10px] px-1 rounded ${
                          coin.direction === 'long'
                            ? 'bg-green-500/20 text-green-400'
                            : 'bg-red-500/20 text-red-400'
                        }`}>
                          {coin.direction}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2 text-xs text-green-400">
                      <span>Breakout confirmed - Entry signal active</span>
                    </div>
                    {/* Ready since time */}
                    {coin.ready_at && (
                      <div className="flex items-center gap-1 text-xs text-gray-400">
                        <Clock className="w-3 h-3" />
                        <span>Ready since {new Date(coin.ready_at).toISOString().slice(11, 19)} UTC</span>
                      </div>
                    )}
                    {/* Entry price from reference high if available */}
                    {coin.reference_candle && (
                      <div className="flex items-center gap-1 text-xs mt-0.5 bg-green-500/10 px-1.5 py-0.5 rounded">
                        <span className="text-green-300 font-medium">Entry Level:</span>
                        <span className="text-green-400 font-mono font-bold">
                          ${coin.reference_candle.high.toFixed(coin.reference_candle.high > 100 ? 2 : 4)}
                        </span>
                      </div>
                    )}
                  </div>
                ) : coin.reference_candle && (
                  /* Placeholder when no entry yet - show proximity info with actual prices */
                  <div className="flex flex-col gap-1 border-l border-gray-700/50 pl-3">
                    <div className="flex items-center gap-1.5 text-xs">
                      <span className="text-gray-500 font-medium">⏳ Awaiting Breakout</span>
                      {/* Candles since reference */}
                      {coin.candles_since_reference !== undefined && coin.candles_since_reference > 0 && (
                        <span className="text-gray-600 font-mono text-[10px]">
                          ({coin.candles_since_reference} candle{coin.candles_since_reference !== 1 ? 's' : ''})
                        </span>
                      )}
                    </div>
                    {/* Volume proximity to threshold */}
                    <div className="flex flex-col gap-0.5 text-xs">
                      {coin.volume_multiplier !== undefined && (
                        (() => {
                          const volThreshold = coin.volume_threshold || effectiveVolumeThreshold || 2.0;
                          const volRatio = coin.volume_multiplier / volThreshold;
                          return (
                            <div className="flex items-center gap-1.5">
                              <span className="text-gray-500">Vol:</span>
                              <span className={`font-mono font-medium ${
                                volRatio >= 1.0 ? 'text-green-400' :
                                volRatio >= 0.7 ? 'text-yellow-400' : 'text-gray-400'
                              }`}>
                                {coin.volume_multiplier.toFixed(2)}x
                              </span>
                              <span className="text-gray-600">/</span>
                              <span className="text-cyan-400 font-mono">{volThreshold.toFixed(1)}x needed</span>
                            </div>
                          );
                        })()
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
                {/* LEFT: Reference candle timing info */}
                <div className="flex items-center gap-3">
                  {/* Candles passed */}
                  {coin.candles_since_reference !== undefined && (
                    <span className="text-gray-400">
                      🕯 {coin.candles_since_reference} candle{coin.candles_since_reference !== 1 ? 's' : ''} passed
                    </span>
                  )}

                  {/* Time to reach ready (stops when ready) */}
                  {coin.seconds_since_reference !== undefined && coin.seconds_since_reference > 0 && (
                    <span className={`flex items-center gap-1 ${coin.status === 'ready' ? 'text-green-400' : 'text-gray-500'}`}>
                      <Clock className="w-3 h-3" />
                      {coin.status === 'ready' ? (
                        <span>Took {formatTimeElapsed(coin.seconds_since_reference)} to ready</span>
                      ) : (
                        <span>{formatTimeElapsed(coin.seconds_since_reference)} elapsed</span>
                      )}
                    </span>
                  )}
                </div>

                {/* RIGHT: Ready/Running status indicator */}
                {(coin.status === 'ready' || coin.status === 'position_running') && (
                  <div className="flex items-center gap-2">
                    {/* Expiry countdown for ready patterns (not for position_running) */}
                    {coin.status === 'ready' && coin.seconds_until_expiry !== undefined && coin.seconds_until_expiry > 0 && (
                      <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded font-mono ${
                        coin.seconds_until_expiry <= 10
                          ? 'bg-red-500/30 text-red-400 animate-pulse'
                          : 'bg-yellow-500/20 text-yellow-400'
                      }`}>
                        <Clock className="w-3 h-3" />
                        {coin.seconds_until_expiry}s
                      </span>
                    )}
                    {/* Ready badge or Position Running badge */}
                    {coin.status === 'position_running' ? (
                      <span className="flex items-center gap-1 px-1.5 py-0.5 bg-orange-500/20 text-orange-400 rounded font-bold">
                        <TrendingUp className="w-3 h-3" />
                        POSITION
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded">
                        <CheckCircle className="w-3 h-3" />
                        READY
                      </span>
                    )}
                  </div>
                )}
                </div>
              </div>
              )}

              {/* Row 3: Volume threshold reference */}
              {coin.reference_candle && coin.volume_multiplier !== undefined && (
                <div className="flex items-center justify-between text-xs text-gray-500">
                  <span>
                    Volume: <span className={`font-mono ${
                      coin.volume_multiplier >= (coin.volume_threshold || effectiveVolumeThreshold || 2.0) ? 'text-green-400' :
                      coin.volume_multiplier >= (coin.volume_threshold || effectiveVolumeThreshold || 2.0) * 0.7 ? 'text-yellow-400' : 'text-gray-400'
                    }`}>
                      {coin.volume_multiplier.toFixed(2)}x
                    </span>
                    <span className="text-gray-600"> / </span>
                    <span className="text-cyan-400 font-mono">
                      {(coin.volume_threshold || effectiveVolumeThreshold || 2.0).toFixed(1)}x
                    </span>
                  </span>
                  {coin.avg_volume && coin.avg_volume > 0 && (
                    <span>
                      Avg: <span className="text-white font-mono">
                        {coin.avg_volume >= 1000000 ? `${(coin.avg_volume / 1000000).toFixed(1)}M` :
                         coin.avg_volume >= 1000 ? `${(coin.avg_volume / 1000).toFixed(1)}K` :
                         coin.avg_volume.toFixed(0)}
                      </span>
                    </span>
                  )}
                </div>
              )}

              {/* Step 2 Progress Bars - Volume and Price progress toward entry */}
              {/* Only show in Step 2 (consolidating). Hide in Step 3 (ready/filling) and Step 4 (position). */}
              {coin.reference_candle && getUIStepNumber(coin.status) === 2 && (
                <Step2ProgressBars
                  update={coinMatchToPatternUpdate(coin, strategyVolumeThreshold)}
                  currentPrice={coin.current_price || 0}
                  candlesSinceReference={coin.candles_since_reference}
                />
              )}

              {/* Step 3: Order Filling UI - Show when ready/filling */}
              {getUIStepNumber(coin.status) === 3 && (
                <div className="mt-2 p-2 bg-cyan-500/10 border border-cyan-500/30 rounded-lg">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="relative flex h-2 w-2">
                        <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
                        <span className="relative inline-flex rounded-full h-2 w-2 bg-cyan-500"></span>
                      </span>
                      <span className="text-cyan-400 font-bold text-xs">⚡ ORDER PLACED</span>
                    </div>
                    {/* Fill timeout countdown */}
                    {coin.seconds_until_expiry !== undefined && coin.seconds_until_expiry > 0 && (
                      <span className={`text-xs font-mono px-2 py-0.5 rounded ${
                        coin.seconds_until_expiry > 30 ? 'bg-cyan-500/20 text-cyan-400' :
                        coin.seconds_until_expiry > 10 ? 'bg-yellow-500/20 text-yellow-400' :
                        'bg-red-500/20 text-red-400 animate-pulse'
                      }`}>
                        Timeout: {coin.seconds_until_expiry}s
                      </span>
                    )}
                  </div>

                  {/* Order details */}
                  <div className="mt-1.5 flex items-center gap-3 text-xs">
                    <span className="text-gray-400">
                      LIMIT {coin.direction === 'short' ? 'SELL' : 'BUY'} @
                      <span className="text-white font-mono ml-1">
                        ${(coin.entry_candle?.entry_price || coin.reference_candle?.high || 0).toFixed(
                          (coin.entry_candle?.entry_price || coin.reference_candle?.high || 0) > 100 ? 2 : 4
                        )}
                      </span>
                    </span>
                  </div>

                  <div className="mt-1 text-[10px] text-gray-500">
                    If not filled → Cancels and resets to Step 1
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* Position Closed PNL Display */}
      {isPositionClosed && coin.closed_pnl != null && (
        <div className={`mt-2 pt-2 border-t border-gray-700/30 px-2 py-2 rounded ${
          coin.closed_pnl >= 0
            ? 'bg-green-500/10 border border-green-500/30'
            : 'bg-red-500/10 border border-red-500/30'
        }`}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className={`px-1.5 py-0.5 rounded text-xs font-bold ${
                coin.closed_reason === 'TP_HIT'
                  ? 'bg-green-500/20 text-green-400'
                  : 'bg-red-500/20 text-red-400'
              }`}>
                {coin.closed_reason === 'TP_HIT' ? 'TP' : 'SL'}
              </span>
              <span className="text-gray-400 text-xs">Position Closed</span>
            </div>
            <span className={`font-mono font-bold ${
              coin.closed_pnl >= 0 ? 'text-green-400' : 'text-red-400'
            }`}>
              {coin.closed_pnl >= 0 ? '+' : ''}${(coin.closed_pnl ?? 0).toFixed(2)}
            </span>
          </div>
        </div>
      )}

      {/* Watching state info - Show live volume progress */}
      {/* Step 1 = Looking for volume spike (watching state in 2-step pattern) */}
      {strategyType === 'pattern' && (!coin.step || coin.step === 0 || coin.step === 1) && coin.status === 'watching' && (
        <div className="mt-2 pt-2 border-t border-gray-700/30 space-y-1.5">
          {/* Volume Progress Bar */}
          {coin.volume_multiplier !== undefined && coin.volume_threshold !== undefined ? (
            (() => {
              const distanceToThreshold = coin.volume_multiplier - coin.volume_threshold;
              const isAboveThreshold = distanceToThreshold >= 0;
              const progressPercent = (coin.volume_multiplier / coin.volume_threshold) * 100;
              // Calculate actual volume values
              const avgVol = coin.avg_volume || 0;
              const currentVol = coin.current_volume || (avgVol * coin.volume_multiplier);
              const thresholdVol = avgVol * coin.volume_threshold;

              // Calculate average position on the bar (average = 1x, threshold = Nx, so avg is at 1/N)
              const avgPositionPercent = (1 / coin.volume_threshold) * 100;
              const isAboveAverage = coin.volume_multiplier >= 1;

              return (
                <div className="space-y-1">
                  {/* Row 1: Current volume info - compact */}
                  <div className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-2">
                      <Volume2 className={`w-3.5 h-3.5 ${
                        isAboveThreshold ? 'text-green-400 animate-pulse' :
                        coin.volume_multiplier >= coin.volume_threshold * 0.7 ? 'text-yellow-400 animate-pulse' :
                        isAboveAverage ? 'text-blue-400' :
                        'text-gray-500'
                      }`} />
                      <span className="text-gray-500">Vol:</span>
                      <span className={`font-mono font-medium ${
                        isAboveThreshold ? 'text-green-400' :
                        coin.volume_multiplier >= coin.volume_threshold * 0.7 ? 'text-yellow-400' :
                        'text-white'
                      }`}>
                        {formatVolume(currentVol)}
                      </span>
                      <span className="text-gray-600">/</span>
                      <span className="text-yellow-400 font-mono">{formatVolume(thresholdVol)}</span>
                      <span className={`font-mono text-[10px] px-1 py-0.5 rounded ${
                        isAboveThreshold
                          ? 'bg-green-500/20 text-green-400'
                          : 'bg-gray-700/50 text-gray-400'
                      }`}>
                        {coin.volume_multiplier.toFixed(1)}x/{coin.volume_threshold.toFixed(1)}x
                      </span>
                    </div>
                    {/* Direction indicator */}
                    {coin.direction && (
                      <span className={`flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] uppercase ${
                        coin.direction === 'long' ? 'bg-green-500/20 text-green-400' :
                        coin.direction === 'short' ? 'bg-red-500/20 text-red-400' :
                        'bg-purple-500/20 text-purple-400'
                      }`}>
                        {coin.direction === 'long' && <TrendingUp className="w-3 h-3" />}
                        {coin.direction === 'short' && <TrendingDown className="w-3 h-3" />}
                        {coin.direction}
                      </span>
                    )}
                  </div>

                  {/* Three-layer Progress Bar with Average Marker */}
                  <div className="relative">
                    {/* Progress bar container */}
                    <div className="relative h-3 bg-gray-700/50 rounded-full overflow-hidden">
                      {/* Layer 1: Average zone (darker fill from 0 to average position) */}
                      <div
                        className="absolute inset-y-0 left-0 bg-gray-600/60 rounded-l-full"
                        style={{ width: `${avgPositionPercent}%` }}
                      />

                      {/* Layer 2: Current volume progress */}
                      <div
                        className={`absolute inset-y-0 left-0 rounded-full transition-all duration-500 ease-out ${
                          isAboveThreshold ? 'bg-green-500' :
                          coin.volume_multiplier >= coin.volume_threshold * 0.7 ? 'bg-yellow-500' :
                          isAboveAverage ? 'bg-blue-500' :
                          'bg-blue-400/70'
                        }`}
                        style={{ width: `${Math.min(progressPercent, 100)}%` }}
                      />

                      {/* Average marker line */}
                      <div
                        className="absolute inset-y-0 w-0.5 bg-white/60"
                        style={{ left: `${avgPositionPercent}%` }}
                      />
                    </div>

                    {/* Labels below progress bar */}
                    <div className="relative h-4 mt-0.5">
                      {/* Start label (0) */}
                      <span className="absolute left-0 text-[9px] text-gray-600 font-mono">0</span>

                      {/* Average marker label */}
                      <div
                        className="absolute transform -translate-x-1/2 text-[9px] text-gray-400 font-mono whitespace-nowrap"
                        style={{ left: `${avgPositionPercent}%` }}
                      >
                        <span className="text-gray-500">avg:</span>{formatVolume(avgVol)}
                      </div>

                      {/* Threshold label (end) */}
                      <span className="absolute right-0 text-[9px] text-yellow-500 font-mono">
                        {formatVolume(thresholdVol)}
                      </span>
                    </div>
                  </div>

                  {/* Status text */}
                  <div className="text-[10px] text-center">
                    <span className={isAboveThreshold ? 'text-green-400 font-medium' : 'text-gray-500'}>
                      {isAboveThreshold
                        ? `✓ Volume spike detected!`
                        : `${progressPercent.toFixed(0)}% to spike threshold`}
                    </span>
                  </div>

                  {/* Price Context Progress Bar */}
                  {coin.day_high && coin.day_low && coin.day_high > coin.day_low && (
                    (() => {
                      const dayLow = coin.day_low;
                      const dayHigh = coin.day_high;
                      const dayRange = dayHigh - dayLow;
                      const currentPrice = coin.current_price || 0;
                      const avgPrice = coin.avg_price_5 || currentPrice;

                      // Growth percentages from the lowest price
                      const totalRangePercent = dayLow > 0 ? ((dayHigh - dayLow) / dayLow) * 100 : 0;
                      const avgGrowthFromLow = dayLow > 0 ? ((avgPrice - dayLow) / dayLow) * 100 : 0;
                      const currentGrowthFromLow = dayLow > 0 ? ((currentPrice - dayLow) / dayLow) * 100 : 0;

                      // Position calculations (as percentage of day range, clamped 0-100)
                      const pricePositionPercent = dayRange > 0
                        ? Math.min(Math.max(((currentPrice - dayLow) / dayRange) * 100, 0), 100)
                        : 50;
                      const avgPositionPercent = dayRange > 0
                        ? Math.min(Math.max(((avgPrice - dayLow) / dayRange) * 100, 0), 100)
                        : 50;

                      const isAboveAvg = currentPrice >= avgPrice;

                      // Format price based on magnitude
                      const formatPrice = (price: number) => {
                        if (price >= 1000) return `$${price.toFixed(0)}`;
                        if (price >= 1) return `$${price.toFixed(2)}`;
                        return `$${price.toFixed(4)}`;
                      };

                      return (
                        <div className="mt-2 pt-2 border-t border-gray-700/20 space-y-1">
                          {/* Header with current price and total range */}
                          <div className="flex items-center justify-between text-xs">
                            <span className="text-gray-500">Price Range:</span>
                            <div className="flex items-center gap-2">
                              <span className={`font-mono font-medium ${isAboveAvg ? 'text-green-400' : 'text-red-400'}`}>
                                {formatPrice(currentPrice)}
                              </span>
                              <span className="text-gray-600">|</span>
                              <span className="text-yellow-400 font-mono text-[10px]">
                                Range: +{totalRangePercent.toFixed(2)}%
                              </span>
                            </div>
                          </div>

                          {/* Three-layer Progress Bar */}
                          <div className="relative">
                            <div className="relative h-3 bg-gray-700/50 rounded-full overflow-hidden">
                              {/* Layer 1: Average zone (darker fill from 0 to avg position) */}
                              <div
                                className="absolute inset-y-0 left-0 bg-gray-600/60 rounded-l-full"
                                style={{ width: `${avgPositionPercent}%` }}
                              />

                              {/* Layer 2: Current price progress */}
                              <div
                                className={`absolute inset-y-0 left-0 rounded-full transition-all duration-500 ease-out ${
                                  isAboveAvg ? 'bg-green-500' : 'bg-red-500'
                                }`}
                                style={{ width: `${pricePositionPercent}%` }}
                              />

                              {/* Average marker line */}
                              <div
                                className="absolute inset-y-0 w-0.5 bg-white/70"
                                style={{ left: `${avgPositionPercent}%` }}
                              />
                            </div>

                            {/* Labels below progress bar - two rows */}
                            <div className="relative mt-0.5">
                              {/* Row 1: Price values */}
                              <div className="relative h-3 flex justify-between items-center">
                                {/* Day Low */}
                                <span className="text-[9px] text-gray-500 font-mono">
                                  {formatPrice(dayLow)}
                                </span>

                                {/* Average price (positioned at marker) */}
                                <div
                                  className="absolute transform -translate-x-1/2 text-[9px] text-gray-400 font-mono"
                                  style={{ left: `${avgPositionPercent}%` }}
                                >
                                  {formatPrice(avgPrice)}
                                </div>

                                {/* Day High */}
                                <span className="text-[9px] text-gray-500 font-mono">
                                  {formatPrice(dayHigh)}
                                </span>
                              </div>

                              {/* Row 2: Growth percentages from low */}
                              <div className="relative h-3 flex justify-between items-center">
                                {/* Low baseline */}
                                <span className="text-[8px] text-gray-600 font-mono">
                                  0%
                                </span>

                                {/* Average growth from low */}
                                <div
                                  className="absolute transform -translate-x-1/2 text-[8px] text-blue-400 font-mono"
                                  style={{ left: `${avgPositionPercent}%` }}
                                >
                                  +{avgGrowthFromLow.toFixed(1)}%
                                </div>

                                {/* High growth (total range) */}
                                <span className="text-[8px] text-yellow-400 font-mono">
                                  +{totalRangePercent.toFixed(1)}%
                                </span>
                              </div>
                            </div>
                          </div>

                          {/* Status text - current position info */}
                          <div className="text-[10px] text-center">
                            <span className="text-gray-500">
                              Current: {pricePositionPercent.toFixed(0)}% into range
                            </span>
                            <span className="text-gray-600 mx-1">•</span>
                            <span className={currentGrowthFromLow >= 0 ? 'text-green-400' : 'text-red-400'}>
                              {currentGrowthFromLow >= 0 ? '+' : ''}{currentGrowthFromLow.toFixed(2)}% from low
                            </span>
                          </div>
                        </div>
                      );
                    })()
                  )}
                </div>
              );
            })()
          ) : (
            /* Fallback when no volume data */
            <span className="text-xs text-gray-500">
              🔍 Scanning for volume spike {effectiveVolumeThreshold ? `(${effectiveVolumeThreshold.toFixed(0)}x average)` : ''}...
            </span>
          )}
        </div>
      )}
    </button>
  );
}

// ==================== Main Component ====================

// Sort interval in milliseconds - only re-sort every 10 seconds to prevent rapid reordering
const SORT_THROTTLE_MS = 10000;

export default function StrategyCard({
  strategy,
  defaultExpanded = false,
  onCoinSelect,
  compact = false,
}: StrategyCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [showRequirements, setShowRequirements] = useState(false);
  const autoExpandedRef = useRef(false);

  // Throttled sort order - only updates every SORT_THROTTLE_MS
  const [sortedSymbolOrder, setSortedSymbolOrder] = useState<string[]>([]);
  const lastSortTimeRef = useRef<number>(0);

  // Note: readyCount and positionCount use full list (these are always meaningful)
  // watchingCount is derived after filtering phantom coins (see activeCoinsList below)
  const readyCount = countReady(strategy.coins);
  const positionCount = countPositionRunning(strategy.coins);

  // Parse countdown timer from strategy
  const nextCandleClose = strategy.next_candle_close
    ? new Date(strategy.next_candle_close)
    : null;
  const hasReadyCoins = readyCount > 0;

  // Auto-expand when coins have active data
  useEffect(() => {
    if (autoExpandedRef.current) return;

    const hasActiveCoins = strategy.coins.some(c =>
      (c.step && c.step > 0) ||
      c.status === 'accumulation' ||
      c.status === 'consolidating' ||
      c.status === 'ready' ||
      (c.volume_multiplier && c.volume_multiplier >= 1.5) // Approaching spike
    );

    if (hasActiveCoins) {
      setExpanded(true);
      autoExpandedRef.current = true;
    }
  }, [strategy.coins]);

  // Helper: derive step number from status string
  const getStepFromStatus = (status?: string): number => {
    switch (status) {
      case 'position_running':
      case 'position_closed':
        return 4;
      case 'ready':
      case 'filling':
        return 3;
      case 'consolidating':
        return 2;
      case 'watching':
      case 'accumulation':
      default:
        return 1;
    }
  };

  // Compute sorted order function (used by both initial and interval updates)
  // Priority: Step 4 (position) > Step 3 (filling/ready) > Step 2 (consolidating) > Step 1 (watching)
  // Within each step: by proximity/profit relevance
  const computeSortOrder = (coins: CoinMatch[]): string[] => {
    return [...coins].sort((a, b) => {
      // For score-based strategies, keep simple score sort
      if (strategy.type !== 'pattern') {
        return (b.score || 0) - (a.score || 0);
      }

      // Step 1: Sort by step number descending (4 -> 3 -> 2 -> 1)
      const stepA = a.step || getStepFromStatus(a.status);
      const stepB = b.step || getStepFromStatus(b.status);
      if (stepB !== stepA) return stepB - stepA;

      // Step 2: Within same step, sort by relevance
      if (stepA === 4) {
        // Position running: sort by entry price proximity (as proxy for profit)
        // Higher current_price relative to entry = more profit for longs
        const profitA = (a.current_price && a.position_entry_price)
          ? (a.current_price - a.position_entry_price) / a.position_entry_price
          : 0;
        const profitB = (b.current_price && b.position_entry_price)
          ? (b.current_price - b.position_entry_price) / b.position_entry_price
          : 0;
        return profitB - profitA;
      } else if (stepA === 3) {
        // Filling/ready: sort by proximity to breakout (closest first)
        const proxA = a.proximity_to_breakout || 100;
        const proxB = b.proximity_to_breakout || 100;
        return proxA - proxB;
      } else if (stepA === 2) {
        // Consolidating: sort by proximity to breakout (closest first)
        const proxA = a.proximity_to_breakout || 100;
        const proxB = b.proximity_to_breakout || 100;
        return proxA - proxB;
      } else {
        // Watching: sort by volume multiplier (higher = more interesting)
        const volA = a.volume_multiplier || 0;
        const volB = b.volume_multiplier || 0;
        return volB - volA;
      }
    }).map(c => c.symbol);
  };

  // Initialize sort order on first render and when coins list changes (new coins added/removed)
  useEffect(() => {
    const currentSymbols = new Set(strategy.coins.map(c => c.symbol));
    const storedSymbols = new Set(sortedSymbolOrder);

    // Check if coins were added or removed
    const symbolsChanged = currentSymbols.size !== storedSymbols.size ||
      [...currentSymbols].some(s => !storedSymbols.has(s));

    if (sortedSymbolOrder.length === 0 || symbolsChanged) {
      setSortedSymbolOrder(computeSortOrder(strategy.coins));
      lastSortTimeRef.current = Date.now();
    }
  }, [strategy.coins.map(c => c.symbol).join(',')]); // Only when symbol list changes

  // Throttled sort update - runs every SORT_THROTTLE_MS
  useEffect(() => {
    const interval = setInterval(() => {
      const now = Date.now();
      if (now - lastSortTimeRef.current >= SORT_THROTTLE_MS) {
        setSortedSymbolOrder(computeSortOrder(strategy.coins));
        lastSortTimeRef.current = now;
      }
    }, SORT_THROTTLE_MS);

    return () => clearInterval(interval);
  }, [strategy.coins, strategy.type]);

  // All coins from the CoinProfiler are real scanning coins - show them all
  // including Step 1 "watching" coins with their volume progress bars
  const activeCoinsList = useMemo(() => {
    return strategy.coins;
  }, [strategy.coins]);

  // Watching count from all coins
  const watchingCount = countWatching(activeCoinsList);

  // Get coins in the throttled sort order, but with fresh data
  // This ensures visual order stays stable while data updates in real-time
  const sortedCoins = useMemo(() => {
    if (sortedSymbolOrder.length === 0) {
      // Fallback: if no stored order yet, compute directly
      return computeSortOrder(activeCoinsList).map(symbol =>
        activeCoinsList.find(c => c.symbol === symbol)
      ).filter((c): c is CoinMatch => c !== undefined);
    }

    // Use stored order but get fresh coin data from filtered list
    const orderedCoins: CoinMatch[] = [];
    const usedSymbols = new Set<string>();
    const activeSymbols = new Set(activeCoinsList.map(c => c.symbol));

    // First, add coins in the stored order (only if they still exist)
    for (const symbol of sortedSymbolOrder) {
      if (!activeSymbols.has(symbol)) continue;
      const coin = activeCoinsList.find(c => c.symbol === symbol);
      if (coin) {
        orderedCoins.push(coin);
        usedSymbols.add(symbol);
      }
    }

    // Then, add any new coins that weren't in the stored order (at the end)
    for (const coin of activeCoinsList) {
      if (!usedSymbols.has(coin.symbol)) {
        orderedCoins.push(coin);
      }
    }

    return orderedCoins;
  }, [activeCoinsList, sortedSymbolOrder]);

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

          {/* Direction indicator at strategy level */}
          {strategy.looking_for && (
            <span className={`text-xs uppercase ${
              strategy.looking_for === 'long' ? 'text-green-400' :
              strategy.looking_for === 'short' ? 'text-red-400' :
              'text-purple-400'
            }`}>
              {strategy.looking_for === 'long' && <TrendingUp className="w-3 h-3 inline" />}
              {strategy.looking_for === 'short' && <TrendingDown className="w-3 h-3 inline" />}
              {strategy.looking_for === 'both' && <ArrowUpDown className="w-3 h-3 inline" />}
            </span>
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
          {positionCount > 0 && (
            <span className="flex items-center gap-1 px-2 py-1 bg-orange-500/20 text-orange-400 text-xs rounded-full">
              <Activity className="w-3 h-3" />
              {positionCount} position{positionCount !== 1 ? 's' : ''}
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
      {expanded && sortedCoins.length > 0 && (
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
                volumeImbalanceConfig={strategy.config ? {
                  volumeSpikeMultiplier: strategy.config.volume_spike_multiplier,
                  lookbackPeriod: strategy.config.lookback_period,
                  breakoutVolumeSurge: strategy.config.breakout_volume_surge,
                  patternExpirationMinutes: strategy.config.pattern_expiration_mins,
                } : undefined}
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
              strategyVolumeThreshold={strategy.config?.volume_spike_multiplier}
              strategyName={formatName(strategy.strategy, strategy.sub_strategy)}
              timeframe={strategy.timeframe}
              nextCandleClose={nextCandleClose}
              onClick={() => onCoinSelect?.(coin.symbol, strategy)}
            />
          ))}
        </div>
      )}

      {/* Empty state */}
      {expanded && sortedCoins.length === 0 && (
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
                volumeImbalanceConfig={strategy.config ? {
                  volumeSpikeMultiplier: strategy.config.volume_spike_multiplier,
                  lookbackPeriod: strategy.config.lookback_period,
                  breakoutVolumeSurge: strategy.config.breakout_volume_surge,
                  patternExpirationMinutes: strategy.config.pattern_expiration_mins,
                } : undefined}
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
