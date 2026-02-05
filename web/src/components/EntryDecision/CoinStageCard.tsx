// Coin Stage Card Component
// Epic 14: Chain Trading System - Entry Decision Real-Time Monitoring
// Displays live coin progress with entry levels and pattern stage tracking

import React, { useState, useEffect } from 'react';
import {
  TrendingUp,
  TrendingDown,
  Target,
  AlertTriangle,
  CheckCircle,
  Clock,
  Activity,
  DollarSign,
} from 'lucide-react';
import PatternProgress from './PatternProgress';
import ReferenceCandleContext from './ReferenceCandleContext';
import Step2ProgressBars from './Step2ProgressBars';
import type {
  PatternUpdate,
  PatternStatus,
  EntryLevels,
  VolumeProgress,
  ReferenceCandle,
  PATTERN_STATUS_COLORS,
  PATTERN_STATUS_LABELS,
} from '../../types/entryDecision';

// ==================== Helper Functions ====================

/**
 * Format seconds into a compact timer display (e.g., "1m 30s" or "45s")
 */
const formatTimer = (seconds: number): string => {
  if (seconds < 0) return '0s';
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  if (mins > 0) return `${mins}m ${secs}s`;
  return `${secs}s`;
};

// ==================== Interfaces ====================

interface CoinStageCardProps {
  /** Pattern update data */
  update: PatternUpdate;
  /** Current price (from live feed) */
  currentPrice?: number;
  /** Click handler */
  onClick?: () => void;
  /** Compact mode */
  compact?: boolean;
}

interface EntryLevelsPanelProps {
  /** Entry levels data */
  levels: EntryLevels;
  /** Current price */
  currentPrice?: number;
  /** Trade direction */
  direction?: string;
}

// ==================== Status Colors ====================

const STATUS_COLORS: Record<PatternStatus, { bg: string; text: string; border: string }> = {
  watching: { bg: 'bg-gray-500/20', text: 'text-gray-400', border: 'border-gray-500/30' },
  accumulation: { bg: 'bg-blue-500/20', text: 'text-blue-400', border: 'border-blue-500/30' },
  consolidating: { bg: 'bg-yellow-500/20', text: 'text-yellow-400', border: 'border-yellow-500/30' },
  ready: { bg: 'bg-green-500/20', text: 'text-green-400', border: 'border-green-500/30' },
  failed: { bg: 'bg-red-500/20', text: 'text-red-400', border: 'border-red-500/30' },
  expired: { bg: 'bg-gray-500/20', text: 'text-gray-500', border: 'border-gray-500/30' },
  position_running: { bg: 'bg-orange-500/20', text: 'text-orange-400', border: 'border-orange-500/30' },
  position_closed: { bg: 'bg-blue-500/20', text: 'text-blue-400', border: 'border-blue-500/30' },
};

const STATUS_LABELS: Record<PatternStatus, string> = {
  watching: 'Watching',
  accumulation: 'Accumulating',
  consolidating: 'Consolidating',
  ready: 'Ready',
  failed: 'Failed',
  expired: 'Expired',
  position_running: 'In Position',
  position_closed: 'Closed',
};

// ==================== Entry Levels Panel ====================

function EntryLevelsPanel({ levels, currentPrice, direction }: EntryLevelsPanelProps) {
  const price = currentPrice || levels.current_price || levels.entry_price;
  const isLong = direction === 'long' || !direction;

  // Calculate distance to entry
  const distanceToEntry = ((levels.entry_price - price) / price) * 100;
  const isNearEntry = Math.abs(distanceToEntry) < 1; // Within 1%

  return (
    <div className="mt-3 p-3 bg-gray-800/50 rounded-lg border border-gray-700/50">
      <div className="flex items-center gap-2 mb-2 text-xs text-gray-500">
        <Target className="w-3 h-3" />
        <span>Entry Levels {direction && `(${direction.toUpperCase()})`}</span>
        {isNearEntry && (
          <span className="ml-auto px-1.5 py-0.5 bg-yellow-500/20 text-yellow-400 rounded text-[10px]">
            NEAR ENTRY
          </span>
        )}
      </div>

      {/* Main Levels */}
      <div className="grid grid-cols-3 gap-2 text-xs">
        {/* Entry */}
        <div className="p-2 bg-blue-500/10 rounded border border-blue-500/20">
          <div className="flex items-center gap-1 text-blue-400 mb-1">
            {isLong ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
            <span>Entry</span>
          </div>
          <div className="font-mono text-white">
            ${levels.entry_price.toFixed(levels.entry_price > 100 ? 2 : 4)}
          </div>
          <div className="text-[10px] text-gray-500 mt-0.5">
            {distanceToEntry > 0 ? '+' : ''}{distanceToEntry.toFixed(2)}%
          </div>
        </div>

        {/* Stop Loss */}
        <div className="p-2 bg-red-500/10 rounded border border-red-500/20">
          <div className="flex items-center gap-1 text-red-400 mb-1">
            <AlertTriangle className="w-3 h-3" />
            <span>Stop Loss</span>
          </div>
          <div className="font-mono text-white">
            ${levels.stop_loss.toFixed(levels.stop_loss > 100 ? 2 : 4)}
          </div>
          <div className="text-[10px] text-red-400 mt-0.5">
            -{levels.risk_percent.toFixed(2)}%
          </div>
        </div>

        {/* Take Profit */}
        <div className="p-2 bg-green-500/10 rounded border border-green-500/20">
          <div className="flex items-center gap-1 text-green-400 mb-1">
            <DollarSign className="w-3 h-3" />
            <span>Take Profit</span>
          </div>
          <div className="font-mono text-white">
            ${levels.take_profit.toFixed(levels.take_profit > 100 ? 2 : 4)}
          </div>
          <div className="text-[10px] text-green-400 mt-0.5">
            +{levels.reward_percent.toFixed(2)}%
          </div>
        </div>
      </div>

      {/* Risk/Reward Summary */}
      <div className="flex items-center justify-between mt-2 pt-2 border-t border-gray-700/50 text-xs">
        <div className="flex items-center gap-4">
          <span className="text-gray-500">
            Risk: <span className="text-red-400">{levels.risk_percent.toFixed(2)}%</span>
          </span>
          <span className="text-gray-500">
            Reward: <span className="text-green-400">{levels.reward_percent.toFixed(2)}%</span>
          </span>
        </div>
        <span className="text-yellow-400 font-medium">
          R:R 1:{levels.risk_reward_ratio.toFixed(1)}
        </span>
      </div>
    </div>
  );
}

// ==================== Volume Progress Bar ====================

interface VolumeProgressBarProps {
  /** Volume progress data */
  progress: VolumeProgress;
  /** Show detailed mode */
  detailed?: boolean;
}

function VolumeProgressBar({ progress, detailed = false }: VolumeProgressBarProps) {
  const {
    current_ratio,
    required_ratio,
    progress_percent,
    candle_direction,
    is_approaching_spike,
    time_remaining_ms,
  } = progress;

  // Color based on progress and candle direction
  const isBullish = candle_direction === 'bullish';
  const getBarColor = () => {
    if (progress_percent >= 100) return 'bg-green-500'; // Spike triggered!
    if (is_approaching_spike) return isBullish ? 'bg-yellow-500' : 'bg-orange-500';
    return isBullish ? 'bg-blue-500' : 'bg-red-500';
  };

  const getGlowColor = () => {
    if (progress_percent >= 100) return 'shadow-green-500/50';
    if (is_approaching_spike) return isBullish ? 'shadow-yellow-500/30' : 'shadow-orange-500/30';
    return '';
  };

  // Format time remaining
  const formatTime = (ms: number) => {
    const seconds = Math.floor(ms / 1000);
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
  };

  return (
    <div className="mb-2">
      {/* Label Row */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2 text-xs">
          <Activity className={`w-3 h-3 ${is_approaching_spike ? 'text-yellow-400 animate-pulse' : 'text-gray-500'}`} />
          <span className="text-gray-400">Volume</span>
          <span className={`font-mono font-medium ${
            progress_percent >= 100 ? 'text-green-400' :
            is_approaching_spike ? 'text-yellow-400' :
            isBullish ? 'text-blue-400' : 'text-red-400'
          }`}>
            {current_ratio.toFixed(2)}x
          </span>
          <span className="text-gray-500">/</span>
          <span className="text-gray-400 font-mono">{required_ratio.toFixed(1)}x</span>
        </div>
        <div className="flex items-center gap-2 text-[10px] text-gray-500">
          <Clock className="w-3 h-3" />
          <span>{formatTime(time_remaining_ms)}</span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="relative h-2 bg-gray-700/50 rounded-full overflow-hidden">
        <div
          className={`absolute inset-y-0 left-0 rounded-full transition-all duration-300 ${getBarColor()} ${getGlowColor()}`}
          style={{ width: `${Math.min(progress_percent, 100)}%` }}
        />
        {/* Threshold marker */}
        <div
          className="absolute inset-y-0 w-0.5 bg-white/30"
          style={{ left: '100%' }}
          title={`Threshold: ${required_ratio}x`}
        />
      </div>

      {/* Detailed Mode - Shows percentages */}
      {detailed && (
        <div className="flex items-center justify-between mt-1 text-[10px]">
          <span className="text-gray-500">
            {progress_percent.toFixed(0)}% to spike
          </span>
          <span className={`${isBullish ? 'text-green-400' : 'text-red-400'}`}>
            {isBullish ? '▲' : '▼'} {candle_direction}
          </span>
        </div>
      )}

      {/* Spike Alert */}
      {progress_percent >= 100 && (
        <div className="flex items-center gap-1 mt-1 text-xs text-green-400 animate-pulse">
          <CheckCircle className="w-3 h-3" />
          <span>Volume spike detected!</span>
        </div>
      )}
    </div>
  );
}

// ==================== Step Progress Display ====================

interface StepProgressProps {
  currentStep: number;
  totalSteps: number;
  status: PatternStatus;
  stepDetails: Array<{
    step_number: number;
    name: string;
    completed: boolean;
    progress: string;
    details: string;
  }>;
}

function StepProgressDisplay({ currentStep, totalSteps, status, stepDetails }: StepProgressProps) {
  return (
    <div className="flex items-center gap-2">
      {/* Step indicators */}
      <div className="flex items-center gap-1">
        {Array.from({ length: totalSteps }, (_, i) => {
          const stepNum = i + 1;
          const isCompleted = stepNum < currentStep || status === 'ready';
          const isCurrent = stepNum === currentStep && status !== 'ready';

          return (
            <div key={i} className="flex items-center">
              <div
                className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium
                  ${isCompleted
                    ? 'bg-green-500/20 text-green-400 border border-green-500/50'
                    : isCurrent
                      ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/50 animate-pulse'
                      : 'bg-gray-700/50 text-gray-500 border border-gray-600/50'
                  }`}
              >
                {isCompleted ? <CheckCircle className="w-3 h-3" /> : stepNum}
              </div>
              {i < totalSteps - 1 && (
                <div className={`w-4 h-0.5 ${isCompleted ? 'bg-green-500/50' : 'bg-gray-700/50'}`} />
              )}
            </div>
          );
        })}
      </div>

      {/* Current step details */}
      {stepDetails && stepDetails[currentStep - 1] && (
        <span className="text-xs text-gray-400 ml-2">
          {stepDetails[currentStep - 1].progress || stepDetails[currentStep - 1].name}
        </span>
      )}
    </div>
  );
}

// ==================== Main Component ====================

export default function CoinStageCard({
  update,
  currentPrice,
  onClick,
  compact = false,
}: CoinStageCardProps) {
  const colors = STATUS_COLORS[update.status] || STATUS_COLORS.watching;
  const isReady = update.status === 'ready';
  const isActive = update.status === 'accumulation' || update.status === 'consolidating';
  // Position running means Chain Runner has an active position - detailed info shown in Trade Lifecycle
  const isPositionRunning = update.status === 'position_running';
  // Stage 2+ means we're past the initial volume spike detection
  const isStageTwo = update.current_step >= 2;
  const price = currentPrice || update.entry_levels?.current_price;

  // ==================== Timer States ====================

  // Time since reference candle found (Stage 2+)
  const [timeSinceReference, setTimeSinceReference] = useState<number>(
    update.seconds_since_reference || 0
  );

  // Order expiry countdown (Ready state)
  const [timeUntilExpiry, setTimeUntilExpiry] = useState<number>(
    update.seconds_until_expiry || 0
  );

  // Position running timer
  const [positionRunningTime, setPositionRunningTime] = useState<number>(0);

  // Update timers every second
  useEffect(() => {
    const interval = setInterval(() => {
      // Timer 1: Time since reference candle (Stage 2+)
      if (isStageTwo && update.reference_detected_at) {
        const now = Date.now();
        const detectedAt = new Date(update.reference_detected_at).getTime();
        const elapsed = Math.floor((now - detectedAt) / 1000);
        setTimeSinceReference(elapsed);
      } else if (update.seconds_since_reference !== undefined) {
        setTimeSinceReference((prev) => prev + 1);
      }

      // Timer 2: Order expiry countdown (Ready state)
      if (isReady && update.seconds_until_expiry !== undefined) {
        setTimeUntilExpiry((prev) => Math.max(0, prev - 1));
      }

      // Timer 3: Position running timer
      if (isPositionRunning && update.position_opened_at) {
        const now = Date.now();
        const openedAt = new Date(update.position_opened_at).getTime();
        const elapsed = Math.floor((now - openedAt) / 1000);
        setPositionRunningTime(elapsed);
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [isStageTwo, isReady, isPositionRunning, update.reference_detected_at, update.seconds_until_expiry, update.position_opened_at, update.seconds_since_reference]);

  // Candle countdown timer (always visible on every card)
  // Uses next_candle_close from backend, or calculates from timeframe as fallback
  const calcCandleRemaining = (): number => {
    // Priority 1: Backend-provided next candle close time
    if (update.next_candle_close) {
      const target = new Date(update.next_candle_close).getTime();
      return Math.max(0, Math.floor((target - Date.now()) / 1000));
    }
    // Priority 2: Volume progress time remaining
    if (update.volume_progress?.time_remaining_ms) {
      return Math.max(0, Math.floor(update.volume_progress.time_remaining_ms / 1000));
    }
    // Priority 3: Calculate from timeframe (UTC-aligned candle intervals)
    const tf = update.timeframe;
    if (tf) {
      const num = parseInt(tf);
      const unit = tf.replace(/[0-9]/g, '');
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

  // Recalculate when backend data arrives (keeps timer accurate)
  useEffect(() => {
    setCandleCountdown(calcCandleRemaining());
  }, [update.next_candle_close, update.volume_progress?.time_remaining_ms]);

  // Tick every second, resync from timeframe when hitting 0
  useEffect(() => {
    const id = setInterval(() => {
      setCandleCountdown(prev => {
        if (prev <= 1) return calcCandleRemaining(); // Resync on wrap
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(id);
  }, [update.timeframe]);

  // Update timers when props change
  useEffect(() => {
    if (update.seconds_since_reference !== undefined) {
      setTimeSinceReference(update.seconds_since_reference);
    }
    if (update.seconds_until_expiry !== undefined) {
      setTimeUntilExpiry(update.seconds_until_expiry);
    }
  }, [update.seconds_since_reference, update.seconds_until_expiry]);

  return (
    <div
      className={`
        rounded-lg border cursor-pointer
        transition-colors duration-200 ease-in-out
        ${isReady
          ? 'bg-green-500/5 border-green-500/30 shadow-md shadow-green-500/5'
          : isActive
            ? 'bg-gray-900/50 border-yellow-500/30'
            : 'bg-gray-900/50 border-gray-700/50'
        }
        hover:bg-gray-800/30
      `}
      onClick={onClick}
    >
      {/* Content wrapper with min-height to prevent flickering on state transitions.
          Stage 2 cards are taller due to reference candle and entry levels.
          Position running cards are shorter since details are in Trade Lifecycle. */}
      <div className={`p-3 ${isPositionRunning ? 'min-h-[140px]' : isStageTwo ? 'min-h-[280px]' : 'min-h-[140px]'}`}>
        {/* Header Row */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            {/* Symbol */}
            <span className={`font-bold ${isReady ? 'text-green-400' : 'text-white'}`}>
              {update.symbol}
            </span>

            {/* Direction indicator */}
            {update.direction && (
              <span className={`flex items-center gap-1 text-xs ${
                update.direction === 'long' ? 'text-green-400' : 'text-red-400'
              }`}>
                {update.direction === 'long'
                  ? <TrendingUp className="w-3 h-3" />
                  : <TrendingDown className="w-3 h-3" />
                }
                <span className="uppercase">{update.direction}</span>
              </span>
            )}

            {/* Timeframe */}
            <span className="text-xs text-gray-500 px-1.5 py-0.5 bg-gray-700/50 rounded">
              {update.timeframe}
            </span>

            {/* Candle Countdown Timer - Always visible next to symbol */}
            <span className={`flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded font-mono ${
              candleCountdown <= 10
                ? 'bg-red-500/20 text-red-400 animate-pulse'
                : candleCountdown <= 30
                  ? 'bg-orange-500/20 text-orange-400'
                  : 'bg-gray-700/50 text-gray-300'
            }`}>
              <Clock className="w-2.5 h-2.5" />
              {formatTimer(candleCountdown)}
            </span>
          </div>

          {/* Status Badge */}
          <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${colors.bg} ${colors.text}`}>
            {STATUS_LABELS[update.status] || update.status}
          </span>
        </div>

        {/* Timer Displays Row */}
        <div className="flex items-center gap-3 mb-2">
          {/* Timer 1: Time since reference candle (Stage 2+) */}
          {isStageTwo && !isPositionRunning && (update.reference_detected_at || update.seconds_since_reference !== undefined) && (
            <div className="flex items-center gap-1 text-[10px]">
              <Clock className="w-3 h-3 text-gray-500" />
              <span className="text-gray-500">Since ref:</span>
              <span className="text-white font-mono">{formatTimer(timeSinceReference)}</span>
            </div>
          )}

          {/* Timer 2: Order expiry countdown (Ready state) */}
          {isReady && update.seconds_until_expiry !== undefined && (
            <div className="flex items-center gap-1 text-[10px]">
              <Clock className="w-3 h-3 text-yellow-500" />
              <span className="text-gray-500">Expires in:</span>
              <span className={`font-mono font-medium ${
                timeUntilExpiry > 30 ? 'text-yellow-400' :
                timeUntilExpiry > 10 ? 'text-orange-400' :
                'text-red-400 animate-pulse'
              }`}>
                {formatTimer(timeUntilExpiry)}
              </span>
            </div>
          )}

          {/* Timer 3: Position running timer */}
          {isPositionRunning && update.position_opened_at && (
            <div className="flex items-center gap-1 text-[10px]">
              <Clock className="w-3 h-3 text-orange-500" />
              <span className="text-gray-500">Position:</span>
              <span className="text-orange-400 font-mono">{formatTimer(positionRunningTime)}</span>
            </div>
          )}
        </div>

        {/* Price */}
        {price && (
          <div className="flex items-center gap-2 mb-2">
            <span className="text-sm text-gray-400">Current:</span>
            <span className="font-mono text-white">
              ${price.toFixed(price > 100 ? 2 : 4)}
            </span>
          </div>
        )}

        {/* Requirements Section - Stage 1 only */}
        {!compact && (update.current_step === 1 || update.status === 'watching') && (
          <div className="mb-3 p-2 bg-gray-800/30 rounded border border-gray-700/50">
            <div className="flex items-center gap-3 text-[10px]">
              <span className="text-gray-500 font-medium">Requirements:</span>
              <div className="flex items-center gap-3 flex-wrap">
                {/* Volume Requirement */}
                {(update.volume_threshold || update.volume_progress?.required_ratio) && (
                  <span className="text-gray-400">
                    Volume ≥ <span className="text-yellow-400 font-mono">
                      {(update.volume_threshold || update.volume_progress?.required_ratio || 0).toFixed(1)}x
                    </span> avg
                  </span>
                )}
                {/* Direction Requirement */}
                {(update.direction || update.looking_for) && (
                  <span className="text-gray-400">
                    Direction: <span className={`font-medium ${
                      (update.direction || update.looking_for)?.toLowerCase() === 'long'
                        ? 'text-green-400'
                        : (update.direction || update.looking_for)?.toLowerCase() === 'short'
                          ? 'text-red-400'
                          : 'text-gray-400'
                    }`}>
                      {(update.direction || update.looking_for)?.toUpperCase()}
                    </span>
                  </span>
                )}
                {/* Timeframe */}
                <span className="text-gray-400">
                  Timeframe: <span className="text-blue-400 font-mono">{update.timeframe}</span>
                </span>
              </div>
            </div>
          </div>
        )}

        {/* Real-time Volume Progress */}
        {update.volume_progress && (
          <VolumeProgressBar progress={update.volume_progress} detailed={!compact} />
        )}

        {/* Step Progress */}
        {!compact && (
          <div className="mb-2">
            <StepProgressDisplay
              currentStep={update.current_step}
              totalSteps={update.total_steps}
              status={update.status}
              stepDetails={update.step_details}
            />
          </div>
        )}

        {/* Compact Step Display */}
        {compact && (
          <div className="flex items-center justify-between text-xs mb-2">
            <span className="text-gray-500">
              Step {update.current_step}/{update.total_steps}
            </span>
            {update.step_details && update.step_details[update.current_step - 1] && (
              <span className="text-gray-400">
                {update.step_details[update.current_step - 1].progress}
              </span>
            )}
          </div>
        )}

        {/* Reference Candle Context (Stage 2+) - Shows Stage 1 spike info
            IMPORTANT: Always show this in Stage 2+ to prevent UI flickering when
            status transitions between watching/consolidating/ready. The Stage 2
            context provides important reference information for monitoring.
            NOT shown when position is running - that info is in Trade Lifecycle. */}
        {!compact && isStageTwo && !isPositionRunning && update.reference_candle && price && (
          <ReferenceCandleContext
            referenceCandle={update.reference_candle}
            currentPrice={price}
            dayHigh={update.day_high}
            dayLow={update.day_low}
            volumeThreshold={update.volume_threshold}
            direction={update.direction || 'long'}
          />
        )}

        {/* Entry Levels - Show in Stage 2+ (not just consolidating/ready) to prevent
            UI flickering. When transitioning between states, the card maintains
            consistent height and shows the calculated entry levels for reference.
            NOT shown when position is running - that info is in Trade Lifecycle. */}
        {!compact && update.entry_levels && isStageTwo && !isPositionRunning && (
          <EntryLevelsPanel
            levels={update.entry_levels}
            currentPrice={price}
            direction={update.direction}
          />
        )}

        {/* Step 2 Progress Bars - Shows volume and price progress toward reference values
            Displays real-time progress for entry conditions in Stage 2 */}
        {!compact && isStageTwo && !isPositionRunning && update.reference_candle && price && (
          <Step2ProgressBars
            update={update}
            currentPrice={price}
          />
        )}

        {/* Position Running indicator - shows position mode instead of strategy tracking.
            When a position is active, we display entry price, chain ID, direction, and timers.
            Full P&L and stage details are shown in Trade Lifecycle. */}
        {isPositionRunning && (
          <div className="mt-3 p-3 bg-orange-500/10 border border-orange-500/30 rounded-lg space-y-2">
            {/* Position Mode Badge */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-orange-400 opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-orange-500"></span>
                </span>
                <span className="px-2 py-0.5 bg-orange-500/20 text-orange-400 rounded text-xs font-bold uppercase">
                  POSITION
                </span>
              </div>
              {/* Direction Badge */}
              {update.direction && (
                <span className={`flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${
                  update.direction === 'long'
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-red-500/20 text-red-400'
                }`}>
                  {update.direction === 'long' ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
                  <span className="uppercase">{update.direction}</span>
                </span>
              )}
            </div>

            {/* Position Details */}
            <div className="grid grid-cols-2 gap-2 text-xs">
              {/* Entry Price */}
              {update.position_entry_price && (
                <div className="flex flex-col">
                  <span className="text-gray-500">Entry Price</span>
                  <span className="text-white font-mono font-medium">
                    ${update.position_entry_price.toFixed(update.position_entry_price > 100 ? 2 : 4)}
                  </span>
                </div>
              )}
              {/* Chain ID */}
              {update.chain_id && (
                <div className="flex flex-col">
                  <span className="text-gray-500">Chain ID</span>
                  <span className="text-orange-400 font-mono text-[10px]">{update.chain_id}</span>
                </div>
              )}
            </div>

            {/* Timers Row */}
            <div className="flex items-center gap-4 text-xs">
              {/* Ref to Entry time (frozen) */}
              {update.seconds_ref_to_entry !== undefined && update.seconds_ref_to_entry > 0 && (
                <div className="flex items-center gap-1">
                  <span className="text-gray-500">Ref→Entry:</span>
                  <span className="text-orange-300 font-mono">{formatTimer(update.seconds_ref_to_entry)}</span>
                </div>
              )}
              {/* Position running time (live) */}
              {update.position_opened_at && (
                <div className="flex items-center gap-1">
                  <Clock className="w-3 h-3 text-orange-500" />
                  <span className="text-gray-500">Running:</span>
                  <span className="text-orange-400 font-mono">{formatTimer(positionRunningTime)}</span>
                </div>
              )}
            </div>

            {/* Trade Lifecycle Link */}
            <div className="text-gray-500 text-[10px] pt-1 border-t border-orange-500/20">
              See Trade Lifecycle for full P&L and stage details
            </div>
          </div>
        )}

        {/* Position Closed PNL Display */}
        {update.status === 'position_closed' && update.closed_pnl != null && (
          <div className={`mt-3 p-3 rounded-lg border ${
            update.closed_pnl >= 0
              ? 'bg-green-500/10 border-green-500/30'
              : 'bg-red-500/10 border-red-500/30'
          }`}>
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`px-2 py-0.5 rounded text-xs font-bold ${
                  update.closed_reason === 'TP_HIT'
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-red-500/20 text-red-400'
                }`}>
                  {update.closed_reason === 'TP_HIT' ? 'TP' : 'SL'}
                </span>
                <span className="text-gray-400 text-xs">Position Closed</span>
              </div>
              <span className={`font-mono font-bold text-lg ${
                update.closed_pnl >= 0 ? 'text-green-400' : 'text-red-400'
              }`}>
                {update.closed_pnl >= 0 ? '+' : ''}${(update.closed_pnl ?? 0).toFixed(2)}
              </span>
            </div>
          </div>
        )}

        {/* Ready Action Hint */}
        {isReady && (
          <div className="mt-2 flex items-center gap-2 text-xs text-green-400">
            <CheckCircle className="w-4 h-4" />
            <span>Pattern complete - Ready for entry</span>
          </div>
        )}

        {/* Last Update */}
        <div className="flex items-center gap-1 mt-2 text-[10px] text-gray-600">
          <Clock className="w-3 h-3" />
          <span>Updated {new Date(update.updated_at).toLocaleTimeString()}</span>
        </div>
      </div>
    </div>
  );
}

// ==================== Compact List Item ====================

interface CoinStageListItemProps {
  update: PatternUpdate;
  onClick?: () => void;
}

export function CoinStageListItem({ update, onClick }: CoinStageListItemProps) {
  const colors = STATUS_COLORS[update.status] || STATUS_COLORS.watching;
  const isReady = update.status === 'ready';

  return (
    <button
      type="button"
      onClick={onClick}
      className={`
        w-full flex items-center justify-between p-2 rounded-lg border transition-colors
        ${isReady
          ? 'bg-green-500/10 border-green-500/30 hover:bg-green-500/20'
          : 'bg-gray-800/50 border-gray-700/50 hover:bg-gray-700/50'
        }
      `}
    >
      <div className="flex items-center gap-2">
        <span className={`font-medium ${isReady ? 'text-green-400' : 'text-white'}`}>
          {update.symbol}
        </span>
        <span className="text-xs text-gray-500">{update.timeframe}</span>
        {update.direction && (
          <span className={`text-xs ${update.direction === 'long' ? 'text-green-400' : 'text-red-400'}`}>
            {update.direction === 'long' ? <TrendingUp className="w-3 h-3" /> : <TrendingDown className="w-3 h-3" />}
          </span>
        )}
      </div>

      <div className="flex items-center gap-2">
        <span className="text-xs text-gray-400">
          Step {update.current_step}/{update.total_steps}
        </span>
        <span className={`px-1.5 py-0.5 rounded text-[10px] ${colors.bg} ${colors.text}`}>
          {STATUS_LABELS[update.status]}
        </span>
      </div>
    </button>
  );
}

// ==================== Grid Container ====================

interface CoinStageGridProps {
  updates: PatternUpdate[];
  onSelect?: (update: PatternUpdate) => void;
}

export function CoinStageGrid({ updates, onSelect }: CoinStageGridProps) {
  if (updates.length === 0) {
    return (
      <div className="text-center py-8">
        <Activity className="w-8 h-8 mx-auto mb-2 text-gray-600" />
        <p className="text-sm text-gray-500">No active patterns</p>
        <p className="text-xs text-gray-600 mt-1">
          Patterns will appear here when coins match strategy criteria
        </p>
      </div>
    );
  }

  // Sort: ready first, then by step progress
  const sortedUpdates = [...updates].sort((a, b) => {
    if (a.status === 'ready' && b.status !== 'ready') return -1;
    if (b.status === 'ready' && a.status !== 'ready') return 1;
    return b.current_step - a.current_step;
  });

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
      {sortedUpdates.map((update) => (
        <CoinStageCard
          key={`${update.symbol}-${update.mode}-${update.timeframe}`}
          update={update}
          onClick={() => onSelect?.(update)}
        />
      ))}
    </div>
  );
}
