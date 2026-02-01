// Coin Stage Card Component
// Epic 14: Chain Trading System - Entry Decision Real-Time Monitoring
// Displays live coin progress with entry levels and pattern stage tracking

import React from 'react';
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
import type {
  PatternUpdate,
  PatternStatus,
  EntryLevels,
  VolumeProgress,
  PATTERN_STATUS_COLORS,
  PATTERN_STATUS_LABELS,
} from '../../types/entryDecision';

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
};

const STATUS_LABELS: Record<PatternStatus, string> = {
  watching: 'Watching',
  accumulation: 'Accumulating',
  consolidating: 'Consolidating',
  ready: 'Ready',
  failed: 'Failed',
  expired: 'Expired',
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
  const price = currentPrice || update.entry_levels?.current_price;

  return (
    <div
      className={`
        rounded-lg border transition-all cursor-pointer
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
      <div className="p-3">
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
          </div>

          {/* Status Badge */}
          <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${colors.bg} ${colors.text}`}>
            {STATUS_LABELS[update.status] || update.status}
          </span>
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

        {/* Entry Levels (when available and not compact) */}
        {!compact && update.entry_levels && (update.status === 'consolidating' || update.status === 'ready') && (
          <EntryLevelsPanel
            levels={update.entry_levels}
            currentPrice={price}
            direction={update.direction}
          />
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
