// Coin Stage Card Component
// Epic 14: Chain Trading System - Entry Decision Real-Time Monitoring
// Displays live coin progress with entry levels and pattern stage tracking

import React, { useState, useEffect, useCallback } from 'react';
import {
  TrendingUp,
  TrendingDown,
  Target,
  AlertTriangle,
  CheckCircle,
  Clock,
  Activity,
  DollarSign,
  Zap,
  Loader2,
} from 'lucide-react';
import PatternProgress from './PatternProgress';
import ReferenceCandleContext from './ReferenceCandleContext';
import Step2ProgressBars from './Step2ProgressBars';
import { wsService } from '../../services/websocket';
import type {
  PatternUpdate,
  PatternStatus,
  EntryLevels,
  EntryCandle,
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
  filling: { bg: 'bg-cyan-500/20', text: 'text-cyan-400', border: 'border-cyan-500/30' },
  failed: { bg: 'bg-red-500/20', text: 'text-red-400', border: 'border-red-500/30' },
  expired: { bg: 'bg-gray-500/20', text: 'text-gray-500', border: 'border-gray-500/30' },
  position_running: { bg: 'bg-orange-500/20', text: 'text-orange-400', border: 'border-orange-500/30' },
  position_closed: { bg: 'bg-blue-500/20', text: 'text-blue-400', border: 'border-blue-500/30' },
};

const STATUS_LABELS: Record<PatternStatus, string> = {
  watching: 'Step 1',
  accumulation: 'Step 1',
  consolidating: 'Step 2',
  ready: 'Step 3',
  filling: 'Filling',
  failed: 'Failed',
  expired: 'Expired',
  position_running: 'Position',
  position_closed: 'Closed',
};

// 4-Step UI mapping: status -> step number (1-4)
const getStepNumber = (status: PatternStatus): number => {
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
};

// Step names for 4-step progress indicator
const STEP_NAMES = ['Ref', 'Consol', 'Ready', 'Position'];

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

// ==================== 4-Step Progress Display ====================

interface FourStepProgressProps {
  status: PatternStatus;
}

function FourStepProgressDisplay({ status }: FourStepProgressProps) {
  const currentStep = getStepNumber(status);
  const totalSteps = 4;

  return (
    <div className="flex items-center gap-1 mb-3">
      {Array.from({ length: totalSteps }, (_, i) => {
        const stepNum = i + 1;
        const isCompleted = stepNum < currentStep;
        const isCurrent = stepNum === currentStep;

        return (
          <div key={i} className="flex items-center">
            <div className="flex flex-col items-center">
              <div
                className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold
                  ${isCompleted
                    ? 'bg-green-500/30 text-green-400 border-2 border-green-500/60'
                    : isCurrent
                      ? 'bg-cyan-500/30 text-cyan-400 border-2 border-cyan-500/60 animate-pulse'
                      : 'bg-gray-700/50 text-gray-500 border-2 border-gray-600/50'
                  }`}
              >
                {isCompleted ? <CheckCircle className="w-4 h-4" /> : stepNum}
              </div>
              <span className={`text-[9px] mt-0.5 ${
                isCompleted ? 'text-green-400' : isCurrent ? 'text-cyan-400' : 'text-gray-500'
              }`}>
                {STEP_NAMES[i]}
              </span>
            </div>
            {i < totalSteps - 1 && (
              <div className={`w-6 h-0.5 mb-3 mx-0.5 ${isCompleted ? 'bg-green-500/50' : 'bg-gray-700/50'}`} />
            )}
          </div>
        );
      })}
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
  // Story 14.19: When lifecycle reset is active, show as Step 1 (watching)
  const effectiveStatus: PatternStatus = lifecycleReset ? 'watching' : update.status;
  const colors = STATUS_COLORS[effectiveStatus] || STATUS_COLORS.watching;
  const isReady = effectiveStatus === 'ready';
  const isFilling = effectiveStatus === 'filling';
  const isActive = effectiveStatus === 'accumulation' || effectiveStatus === 'consolidating';
  // Position running means Chain Runner has an active position
  const isPositionRunning = effectiveStatus === 'position_running';
  // Step 3 = Ready or Filling (order placed, waiting for fill)
  const isStepThree = isReady || isFilling;
  // Step 4 = Position active
  const isStepFour = isPositionRunning;
  // Stage 2+ means we're past the initial volume spike detection (steps 2, 3, or 4)
  const isStageTwo = update.current_step >= 2 || effectiveStatus === 'consolidating' || isStepThree || isStepFour;
  // Get the UI step number (1-4) based on status
  const uiStep = getStepNumber(effectiveStatus);
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

  // Fill timeout countdown (Step 3: filling)
  const [fillTimeoutRemaining, setFillTimeoutRemaining] = useState<number>(
    update.fill_timeout_seconds || 0
  );

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

      // Timer 4: Fill timeout countdown (Step 3: filling)
      if (isFilling && update.fill_timeout_seconds !== undefined) {
        setFillTimeoutRemaining((prev) => Math.max(0, prev - 1));
      }
    }, 1000);

    return () => clearInterval(interval);
  }, [isStageTwo, isReady, isFilling, isPositionRunning, update.reference_detected_at, update.seconds_until_expiry, update.position_opened_at, update.seconds_since_reference, update.fill_timeout_seconds]);

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
    if (update.fill_timeout_seconds !== undefined) {
      setFillTimeoutRemaining(update.fill_timeout_seconds);
    }
  }, [update.seconds_since_reference, update.seconds_until_expiry, update.fill_timeout_seconds]);

  // Story 14.19: Local override status when CHAIN_LIFECYCLE_UPDATE matches this card's symbol
  const [lifecycleReset, setLifecycleReset] = useState(false);

  useEffect(() => {
    const handleChainLifecycleUpdate = (event: any) => {
      const data = event?.data;
      const eventSymbol = data?.pattern?.symbol || data?.chain?.symbol;
      if (eventSymbol && eventSymbol === update.symbol) {
        // Position closed - this card should show Step 1 (watching) briefly
        // The backend will send a PATTERN_UPDATE shortly after to set the real state
        setLifecycleReset(true);
      }
    };

    wsService.subscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
    return () => {
      wsService.unsubscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
    };
  }, [update.symbol]);

  // Clear the lifecycle reset when we receive a legitimate non-position status from props.
  // This handles: watching (reset complete), accumulation/consolidating (new pattern started).
  // We keep lifecycleReset=true as long as the status is still position_running (stale data).
  useEffect(() => {
    if (lifecycleReset && update.status !== 'position_running' && update.status !== 'position_closed') {
      setLifecycleReset(false);
    }
  }, [lifecycleReset, update.status]);

  return (
    <div
      className={`
        rounded-lg border cursor-pointer
        transition-colors duration-200 ease-in-out
        ${isStepThree
          ? 'bg-cyan-500/5 border-cyan-500/30 shadow-md shadow-cyan-500/5'
          : isStepFour
            ? 'bg-orange-500/5 border-orange-500/30 shadow-md shadow-orange-500/5'
            : isActive
              ? 'bg-gray-900/50 border-yellow-500/30'
              : 'bg-gray-900/50 border-gray-700/50'
        }
        hover:bg-gray-800/30
      `}
      onClick={onClick}
    >
      {/* Content wrapper with min-height to prevent flickering on state transitions. */}
      <div className={`p-3 ${isStepFour ? 'min-h-[320px]' : isStageTwo ? 'min-h-[340px]' : 'min-h-[160px]'}`}>
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
            {STATUS_LABELS[effectiveStatus] || effectiveStatus}
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

        {/* 4-Step Progress Indicator */}
        {!compact && (
          <FourStepProgressDisplay status={effectiveStatus} />
        )}

        {/* Requirements Section - Stage 1 only */}
        {!compact && (update.current_step === 1 || effectiveStatus === 'watching') && (
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

        {/* Real-time Volume Progress - Only in Step 1 */}
        {uiStep === 1 && update.volume_progress && (
          <VolumeProgressBar progress={update.volume_progress} detailed={!compact} />
        )}

        {/* Compact Step Display */}
        {compact && (
          <div className="flex items-center justify-between text-xs mb-2">
            <span className="text-gray-500">
              Step {uiStep}/4
            </span>
            <span className="text-gray-400">
              {STEP_NAMES[uiStep - 1]}
            </span>
          </div>
        )}

        {/* Reference Candle Context - Always show in Steps 2, 3, and 4
            Preserves reference candle info across all stages for context */}
        {!compact && isStageTwo && update.reference_candle && price && (
          <ReferenceCandleContext
            referenceCandle={update.reference_candle}
            currentPrice={price}
            dayHigh={update.day_high}
            dayLow={update.day_low}
            volumeThreshold={update.volume_threshold}
            direction={update.direction || 'long'}
          />
        )}

        {/* Entry Levels - Show in Steps 2, 3, and 4 for reference */}
        {!compact && update.entry_levels && isStageTwo && (
          <EntryLevelsPanel
            levels={update.entry_levels}
            currentPrice={price}
            direction={update.direction}
          />
        )}

        {/* Step 2 Progress Bars - Only show in Step 2 (consolidating)
            NOT shown in Steps 3 (ready/filling) or 4 (position) */}
        {!compact && uiStep === 2 && update.reference_candle && price && (
          <Step2ProgressBars
            update={update}
            currentPrice={price}
          />
        )}

        {/* Step 3a: Breakout Detected - Order being placed */}
        {!compact && isReady && (
          <div className="mt-3 p-3 bg-yellow-500/10 border border-yellow-500/30 rounded-lg space-y-2">
            <div className="flex items-center gap-2">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-yellow-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-yellow-500"></span>
              </span>
              <span className="text-yellow-400 font-bold text-xs">BREAKOUT DETECTED</span>
              {update.breakout_detected_at && (
                <span className="text-[10px] text-gray-500 ml-auto flex items-center gap-1">
                  <Clock className="w-2.5 h-2.5" />
                  {new Date(update.breakout_detected_at).toISOString().slice(11, 19)} UTC
                </span>
              )}
            </div>

            <div className="flex items-center gap-2 text-[10px] text-gray-400">
              <Loader2 className="w-3 h-3 animate-spin text-yellow-400" />
              <span>Placing order...</span>
            </div>

            {/* Entry Candle Info (breakout candle) - if available */}
            {update.entry_candle && (
              <div className="p-2 bg-gray-800/30 rounded border border-gray-700/30">
                <div className="flex items-center gap-2 mb-1.5 text-[10px] text-gray-500">
                  <Zap className="w-3 h-3 text-yellow-400" />
                  <span>Breakout Candle</span>
                </div>
                <div className="grid grid-cols-4 gap-2 text-[10px]">
                  <div>
                    <span className="text-gray-500 block">High</span>
                    <span className="text-white font-mono">
                      ${update.entry_candle.high.toFixed(update.entry_candle.high > 100 ? 2 : 4)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-500 block">Low</span>
                    <span className="text-white font-mono">
                      ${update.entry_candle.low.toFixed(update.entry_candle.low > 100 ? 2 : 4)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-500 block">Entry</span>
                    <span className="text-yellow-400 font-mono">
                      ${update.entry_candle.entry_price.toFixed(update.entry_candle.entry_price > 100 ? 2 : 4)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-500 block">Volume</span>
                    <span className="text-green-400 font-mono">
                      {update.entry_candle.volume_multiplier
                        ? `${update.entry_candle.volume_multiplier.toFixed(1)}x`
                        : '-'}
                    </span>
                  </div>
                </div>
              </div>
            )}

            {/* Ready Expiration Countdown */}
            {timeUntilExpiry > 0 && (
              <div className="flex items-center justify-between text-[10px]">
                <span className="text-gray-500">Order placement timeout</span>
                <span className={`font-mono font-medium ${
                  timeUntilExpiry > 30 ? 'text-yellow-400' : 'text-red-400 animate-pulse'
                }`}>
                  {formatTimer(timeUntilExpiry)}
                </span>
              </div>
            )}
          </div>
        )}

        {/* Step 3b: Order Placed - Waiting for fill */}
        {!compact && isFilling && (
          <div className="mt-3 p-3 bg-cyan-500/10 border border-cyan-500/30 rounded-lg space-y-2">
            <div className="flex items-center gap-2">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-cyan-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-cyan-500"></span>
              </span>
              <span className="text-cyan-400 font-bold text-xs">ORDER PLACED</span>
              {/* Time elapsed from Step 2 (consolidation start) to Step 3 (breakout) */}
              {update.reference_detected_at && update.breakout_detected_at && (
                <span className="text-[10px] text-gray-500 ml-auto flex items-center gap-1">
                  <Clock className="w-2.5 h-2.5" />
                  Took {formatTimer(Math.floor(
                    (new Date(update.breakout_detected_at).getTime() - new Date(update.reference_detected_at).getTime()) / 1000
                  ))}
                </span>
              )}
            </div>

            {/* Order Details */}
            <div className="flex items-center justify-between text-xs">
              <span className="text-gray-400">
                LIMIT {update.direction === 'short' ? 'SELL' : 'BUY'} @
                <span className="text-white font-mono ml-1">
                  ${(update.order_price || update.entry_levels?.entry_price || 0).toFixed(
                    (update.order_price || update.entry_levels?.entry_price || 0) > 100 ? 2 : 4
                  )}
                </span>
              </span>
              {update.order_quantity_usd && (
                <span className="text-gray-500">
                  ${update.order_quantity_usd.toFixed(0)}
                </span>
              )}
            </div>

            {/* Entry Candle Info - Similar to Reference Candle display */}
            {update.entry_candle && (
              <div className="p-2 bg-gray-800/30 rounded border border-gray-700/30">
                <div className="flex items-center gap-2 mb-1.5 text-[10px] text-gray-500">
                  <Zap className="w-3 h-3 text-cyan-400" />
                  <span>Entry Candle</span>
                  {update.entry_candle.open_time && (
                    <span className="ml-auto text-gray-600">
                      {new Date(update.entry_candle.open_time).toISOString().slice(5, 16).replace('T', ' ')} UTC
                    </span>
                  )}
                </div>
                <div className="grid grid-cols-4 gap-2 text-[10px]">
                  <div>
                    <span className="text-gray-500 block">High</span>
                    <span className="text-white font-mono">
                      ${update.entry_candle.high.toFixed(update.entry_candle.high > 100 ? 2 : 4)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-500 block">Low</span>
                    <span className="text-white font-mono">
                      ${update.entry_candle.low.toFixed(update.entry_candle.low > 100 ? 2 : 4)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-500 block">Entry</span>
                    <span className="text-cyan-400 font-mono">
                      ${update.entry_candle.entry_price.toFixed(update.entry_candle.entry_price > 100 ? 2 : 4)}
                    </span>
                  </div>
                  <div>
                    <span className="text-gray-500 block">Volume</span>
                    <span className="text-green-400 font-mono">
                      {update.entry_candle.volume_multiplier
                        ? `${update.entry_candle.volume_multiplier.toFixed(1)}x`
                        : '-'}
                    </span>
                  </div>
                </div>
              </div>
            )}

            {/* Fill Timeout Countdown */}
            <div className="space-y-1">
              <div className="flex items-center justify-between text-[10px]">
                <span className="text-gray-500">Waiting to fill...</span>
                <span className={`font-mono font-medium ${
                  fillTimeoutRemaining > 60 ? 'text-cyan-400' :
                  fillTimeoutRemaining > 30 ? 'text-yellow-400' :
                  'text-red-400 animate-pulse'
                }`}>
                  {formatTimer(fillTimeoutRemaining)}
                </span>
              </div>
              {update.fill_timeout_total && update.fill_timeout_total > 0 && (
                <div className="h-2 bg-gray-700/50 rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-all duration-1000 ${
                      fillTimeoutRemaining > 60 ? 'bg-cyan-500' :
                      fillTimeoutRemaining > 30 ? 'bg-yellow-500' :
                      'bg-red-500'
                    }`}
                    style={{
                      width: `${Math.max(0, (fillTimeoutRemaining / update.fill_timeout_total) * 100)}%`
                    }}
                  />
                </div>
              )}
            </div>

            <div className="text-[10px] text-gray-500 pt-1 border-t border-cyan-500/20">
              If not filled → Cancels and resets to Step 1
            </div>
          </div>
        )}

        {/* Step 4: Position Running - shows position with entry filled info, P&L, R:R progress */}
        {isPositionRunning && (
          <div className="mt-3 p-3 bg-orange-500/10 border border-orange-500/30 rounded-lg space-y-2">
            {/* Entry Filled Badge */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <CheckCircle className="w-4 h-4 text-green-400" />
                <span className="text-green-400 font-bold text-xs">✓ ENTRY FILLED</span>
                {update.position_entry_price && (
                  <span className="text-white font-mono text-xs">
                    @ ${update.position_entry_price.toFixed(update.position_entry_price > 100 ? 2 : 4)}
                  </span>
                )}
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

            {/* Reference & Entry Candle Context (compact) */}
            {(update.reference_candle || update.entry_candle) && (
              <div className="p-2 bg-gray-800/40 rounded border border-gray-700/30 space-y-1.5">
                {/* Reference Candle Row */}
                {update.reference_candle && (
                  <div className="flex items-center gap-2 text-[10px]">
                    <Activity className="w-3 h-3 text-green-400 flex-shrink-0" />
                    <span className="text-gray-500">Ref:</span>
                    <span className="text-gray-400 font-mono">
                      {new Date(update.reference_candle.open_time).toISOString().slice(11, 16)} UTC
                    </span>
                    <span className="text-gray-600">|</span>
                    <span className="text-gray-400">H:</span>
                    <span className="text-white font-mono">
                      {update.reference_candle.high.toFixed(update.reference_candle.high > 100 ? 2 : 4)}
                    </span>
                    <span className="text-gray-400">L:</span>
                    <span className="text-white font-mono">
                      {update.reference_candle.low.toFixed(update.reference_candle.low > 100 ? 2 : 4)}
                    </span>
                    <span className="text-green-400 font-mono ml-auto">
                      {update.reference_candle.volume_multiplier.toFixed(1)}x
                    </span>
                  </div>
                )}
                {/* Entry Candle Row */}
                {update.entry_candle && (
                  <div className="flex items-center gap-2 text-[10px]">
                    <Zap className="w-3 h-3 text-yellow-400 flex-shrink-0" />
                    <span className="text-gray-500">Entry:</span>
                    <span className="text-gray-400 font-mono">
                      {new Date(update.entry_candle.open_time).toISOString().slice(11, 16)} UTC
                    </span>
                    <span className="text-gray-600">|</span>
                    <span className="text-gray-400">Price:</span>
                    <span className="text-white font-mono">
                      {update.entry_candle.entry_price.toFixed(update.entry_candle.entry_price > 100 ? 2 : 4)}
                    </span>
                    <span className={`ml-auto px-1 py-0.5 rounded text-[9px] font-medium ${
                      update.entry_candle.direction === 'long'
                        ? 'bg-green-500/20 text-green-400'
                        : 'bg-red-500/20 text-red-400'
                    }`}>
                      {update.entry_candle.direction?.toUpperCase()}
                    </span>
                  </div>
                )}
                {/* Entry Levels Summary */}
                {update.entry_levels && (
                  <div className="flex items-center gap-3 text-[10px] pt-1 border-t border-gray-700/20">
                    <div className="flex items-center gap-1">
                      <Target className="w-3 h-3 text-blue-400" />
                      <span className="text-gray-500">SL:</span>
                      <span className="text-red-400 font-mono">
                        {update.entry_levels.stop_loss.toFixed(update.entry_levels.stop_loss > 100 ? 2 : 4)}
                      </span>
                    </div>
                    <div className="flex items-center gap-1">
                      <span className="text-gray-500">TP:</span>
                      <span className="text-green-400 font-mono">
                        {update.entry_levels.take_profit.toFixed(update.entry_levels.take_profit > 100 ? 2 : 4)}
                      </span>
                    </div>
                    <span className="text-gray-500 ml-auto">
                      R:R <span className="text-yellow-400 font-mono">{update.entry_levels.risk_reward_ratio.toFixed(1)}</span>
                    </span>
                  </div>
                )}
              </div>
            )}

            {/* Unrealized P&L - calculated from entry price and current price */}
            {(() => {
              const entryPrice = update.position_entry_price || update.entry_levels?.entry_price || 0;
              const curPrice = update.current_price || 0;
              const isLong = update.direction === 'long';
              if (entryPrice > 0 && curPrice > 0) {
                const pnlPercent = isLong
                  ? ((curPrice - entryPrice) / entryPrice) * 100
                  : ((entryPrice - curPrice) / entryPrice) * 100;
                return (
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-gray-400">PnL:</span>
                    <span className={`font-mono font-bold ${pnlPercent >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {pnlPercent >= 0 ? '+' : ''}{pnlPercent.toFixed(2)}%
                    </span>
                  </div>
                );
              }
              return null;
            })()}

            {/* Position Timer */}
            {update.position_opened_at && (
              <div className="flex items-center gap-1 text-xs pt-1 border-t border-orange-500/20">
                <Clock className="w-3 h-3 text-orange-500" />
                <span className="text-gray-500">Position Running:</span>
                <span className="text-orange-400 font-mono">{formatTimer(positionRunningTime)}</span>
              </div>
            )}
          </div>
        )}

        {/* Position Closed PNL Display */}
        {effectiveStatus === 'position_closed' && update.closed_pnl != null && (
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

        {/* Last Update */}
        <div className="flex items-center gap-1 mt-2 text-[10px] text-gray-600">
          <Clock className="w-3 h-3" />
          <span>Updated {new Date(update.updated_at).toISOString().slice(11, 19)} UTC</span>
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
  const uiStep = getStepNumber(update.status);
  const isStepThree = update.status === 'ready' || update.status === 'filling';
  const isStepFour = update.status === 'position_running';

  return (
    <button
      type="button"
      onClick={onClick}
      className={`
        w-full flex items-center justify-between p-2 rounded-lg border transition-colors
        ${isStepThree
          ? 'bg-cyan-500/10 border-cyan-500/30 hover:bg-cyan-500/20'
          : isStepFour
            ? 'bg-orange-500/10 border-orange-500/30 hover:bg-orange-500/20'
            : 'bg-gray-800/50 border-gray-700/50 hover:bg-gray-700/50'
        }
      `}
    >
      <div className="flex items-center gap-2">
        <span className={`font-medium ${isStepThree ? 'text-cyan-400' : isStepFour ? 'text-orange-400' : 'text-white'}`}>
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
          Step {uiStep}/4
        </span>
        <span className={`px-1.5 py-0.5 rounded text-[10px] ${colors.bg} ${colors.text}`}>
          {STEP_NAMES[uiStep - 1]}
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
  // Filter out position_running and position_closed entries from the grid.
  // Step 4 (position) info is displayed separately in the StrategyCard coin row,
  // so showing it here would be redundant/stale.
  const gridUpdates = updates.filter(u =>
    u.status !== 'position_running' && u.status !== 'position_closed'
  );

  if (gridUpdates.length === 0) {
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

  // Helper: derive UI step from status
  const getStepFromStatus = (status: string): number => {
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

  // Sort: Step 4 first, then 3, 2, 1. Within same step, by relevance.
  const sortedUpdates = [...gridUpdates].sort((a, b) => {
    // Primary: step number descending (4 -> 3 -> 2 -> 1)
    const stepA = getStepFromStatus(a.status);
    const stepB = getStepFromStatus(b.status);
    if (stepB !== stepA) return stepB - stepA;

    // Secondary: within same step, sort by relevance
    if (stepA === 3) {
      // Ready/filling: sort by current_step descending, then by fill timeout
      if (b.current_step !== a.current_step) return b.current_step - a.current_step;
      return (a.fill_timeout_seconds || 999) - (b.fill_timeout_seconds || 999);
    } else if (stepA === 2) {
      // Consolidating: sort by current_step descending (progress within step 2)
      if (b.current_step !== a.current_step) return b.current_step - a.current_step;
      // Then by volume progress (higher = closer to breakout)
      const volA = a.volume_progress?.progress_percent || 0;
      const volB = b.volume_progress?.progress_percent || 0;
      return volB - volA;
    } else {
      // Watching: sort by volume progress (higher = more likely to spike)
      const volA = a.volume_progress?.progress_percent || 0;
      const volB = b.volume_progress?.progress_percent || 0;
      return volB - volA;
    }
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
