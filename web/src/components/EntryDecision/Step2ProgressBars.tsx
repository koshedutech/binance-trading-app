// Step 2 Progress Bars Component
// Epic 14: Shows real-time volume progress in Step 2 (Breakout Detection)
// Entry criteria is VOLUME-based: current candle volume must match reference candle energy

import React from 'react';
import { Activity, TrendingUp, TrendingDown } from 'lucide-react';
import type { ReferenceCandle, PatternUpdate } from '../../types/entryDecision';

// ==================== Interfaces ====================

interface Step2ProgressBarsProps {
  /** Pattern update data */
  update: PatternUpdate;
  /** Current market price */
  currentPrice: number;
  /** Number of candles since reference (optional, for display) */
  candlesSinceReference?: number;
}

// ==================== Volume Progress Bar ====================

interface VolumeProgressBarProps {
  /** Current candle's volume multiplier */
  currentVolumeMultiplier: number;
  /** Average volume multiplier from reference to last candle */
  avgVolumeMultiplier: number;
  /** Reference candle's achieved volume multiplier (the target) */
  referenceVolumeMultiplier: number;
  /** Entry threshold (e.g., 3x) */
  entryThreshold: number;
  /** Direction for display context */
  direction: string;
}

function VolumeProgressBar({
  currentVolumeMultiplier,
  avgVolumeMultiplier,
  referenceVolumeMultiplier,
  entryThreshold,
  direction,
}: VolumeProgressBarProps) {
  // Bar range: 1x to reference volume (e.g., 3.5x)
  const minValue = 1;
  const maxValue = referenceVolumeMultiplier;
  const range = maxValue - minValue;

  // Calculate positions as percentages
  const avgPosition = range > 0 ? ((avgVolumeMultiplier - minValue) / range) * 100 : 50;
  const currentPosition = range > 0 ? Math.min(((currentVolumeMultiplier - minValue) / range) * 100, 100) : 0;
  const entryPosition = range > 0 ? ((entryThreshold - minValue) / range) * 100 : 66;

  // Progress percentage (current toward entry threshold)
  const progressPercent = entryThreshold > 0 ? Math.min((currentVolumeMultiplier / entryThreshold) * 100, 100) : 0;

  // Shortfall: how much more volume is needed
  const shortfall = entryThreshold - currentVolumeMultiplier;
  const isConditionMet = currentVolumeMultiplier >= entryThreshold;

  // Determine color based on progress
  const getProgressColor = () => {
    if (isConditionMet) return 'bg-green-500';
    if (currentVolumeMultiplier >= entryThreshold * 0.75) return 'bg-yellow-500';
    if (currentVolumeMultiplier >= avgVolumeMultiplier) return 'bg-blue-500';
    return 'bg-blue-400/50';
  };

  const isLong = direction === 'long';

  return (
    <div className="mb-3">
      {/* Header */}
      <div className="flex items-center justify-between mb-1.5">
        <div className="flex items-center gap-2 text-xs">
          <Activity className="w-3.5 h-3.5 text-blue-400" />
          <span className="text-gray-300 font-medium">Volume Breakout</span>
          <span className={`px-1.5 py-0.5 rounded text-[10px] ${
            isLong ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
          }`}>
            {direction.toUpperCase()}
          </span>
          {isConditionMet && (
            <span className="px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded text-[10px] animate-pulse">
              TRIGGERED
            </span>
          )}
        </div>
        <div className="text-xs text-gray-400">
          <span className="text-white font-mono">{currentVolumeMultiplier.toFixed(2)}x</span>
          <span className="mx-1">/</span>
          <span className="text-yellow-400 font-mono">{entryThreshold.toFixed(1)}x</span>
          <span className="ml-1 text-gray-500">needed</span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="relative h-6 bg-gray-700/50 rounded-lg overflow-visible">
        {/* Fill - current progress */}
        <div
          className={`absolute inset-y-0 left-0 rounded-l-lg transition-all duration-300 ${getProgressColor()}`}
          style={{ width: `${Math.max(currentPosition, 0)}%` }}
        />

        {/* Average Volume Marker (Ref→Last candle average) */}
        <div
          className="absolute top-0 bottom-0 flex flex-col items-center"
          style={{ left: `${Math.min(Math.max(avgPosition, 5), 95)}%`, transform: 'translateX(-50%)' }}
        >
          <div className="w-0.5 h-full bg-gray-400/70" />
          <div className="absolute -bottom-4 text-[9px] text-gray-400 whitespace-nowrap">
            Avg {avgVolumeMultiplier.toFixed(1)}x
          </div>
        </div>

        {/* Volume Threshold Marker (required for entry trigger) */}
        <div
          className="absolute top-0 bottom-0 flex flex-col items-center z-10"
          style={{ left: `${Math.min(Math.max(entryPosition, 5), 95)}%`, transform: 'translateX(-50%)' }}
        >
          <div className="w-1.5 h-full bg-yellow-500/70" />
          <div className="absolute -top-4 text-[9px] text-yellow-400 font-medium whitespace-nowrap">
            Entry {entryThreshold.toFixed(1)}x
          </div>
        </div>

        {/* Reference Volume Marker (right edge = reference candle's actual volume) */}
        {referenceVolumeMultiplier > entryThreshold + 0.1 && (
          <div className="absolute top-0 bottom-0 right-0 flex flex-col items-center">
            <div className="w-0.5 h-full bg-green-500/50" />
          </div>
        )}

        {/* Current Volume Indicator (floating circle) */}
        <div
          className={`absolute top-1/2 -translate-y-1/2 w-5 h-5 rounded-full border-2 border-white shadow-lg transition-all duration-200 z-20 ${
            isConditionMet ? 'bg-green-500' : 'bg-blue-500'
          }`}
          style={{ left: `${Math.min(Math.max(currentPosition, 2), 98)}%`, transform: 'translateX(-50%) translateY(-50%)' }}
        />

        {/* Progress text inside bar */}
        <div className="absolute inset-0 flex items-center justify-center text-[10px] font-mono text-white/70 z-10">
          {isConditionMet ? 'Volume matched!' : `${progressPercent.toFixed(0)}%`}
        </div>

        {/* Scale Labels */}
        <div className="absolute -bottom-4 left-0 text-[9px] text-gray-500">1x</div>
        <div className="absolute -bottom-4 right-0 text-[9px] text-green-400 font-medium">
          Ref {referenceVolumeMultiplier.toFixed(1)}x
        </div>
      </div>

      {/* Spacer for labels */}
      <div className="h-5" />

      {/* Volume Detail Row */}
      <div className="flex items-center justify-between text-[10px] mt-1">
        <div className="flex items-center gap-3">
          <span className={`flex items-center gap-1 ${isConditionMet ? 'text-green-400' : 'text-gray-400'}`}>
            <span className={`w-2 h-2 rounded-full ${isConditionMet ? 'bg-green-500' : 'bg-gray-600'}`} />
            {isConditionMet
              ? `Volume matched at ${currentVolumeMultiplier.toFixed(2)}x`
              : `Need ${shortfall.toFixed(2)}x more volume`
            }
          </span>
        </div>
        <span className="text-gray-500 font-mono">
          {progressPercent.toFixed(0)}% of threshold
        </span>
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function Step2ProgressBars({ update, currentPrice, candlesSinceReference }: Step2ProgressBarsProps) {
  // Extract required data - CRITICAL: Don't return null, render placeholder instead to prevent flicker
  const referenceCandle = update.reference_candle;

  // If no reference candle data yet, render a loading placeholder instead of nothing
  // This prevents the component from unmounting and causing layout shift/flicker
  if (!referenceCandle) {
    return (
      <div className="mt-4 pt-4 border-t border-gray-700/50 animate-pulse">
        <div className="h-4 bg-gray-700/30 rounded w-1/3 mb-3" />
        <div className="h-8 bg-gray-700/30 rounded mb-3" />
      </div>
    );
  }

  // Volume data
  const referenceVolumeMultiplier = referenceCandle.volume_multiplier;
  // Use actual threshold from settings, fallback to 2.0 (common default)
  const entryThreshold = update.volume_threshold || 2.0;

  // Current candle volume - use from update or calculate from volume_progress
  const currentVolumeMultiplier = update.current_candle_volume_multiplier
    || update.volume_progress?.current_ratio
    || 1.0;

  // Average volume from reference to last candle (use consolidation avg or fallback)
  const avgVolumeMultiplier = update.consolidation_avg_volume_multiplier
    || (referenceVolumeMultiplier + 1) / 2; // Fallback: midpoint

  const direction = update.direction || 'long';

  // Volume is the ONLY entry criteria
  const volumeConditionMet = currentVolumeMultiplier >= entryThreshold;

  return (
    <div className="mt-4 pt-4 border-t border-gray-700/50">
      {/* Section Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2 text-xs text-gray-400">
          {direction === 'long'
            ? <TrendingUp className="w-3.5 h-3.5 text-green-400" />
            : <TrendingDown className="w-3.5 h-3.5 text-red-400" />
          }
          <span>Awaiting Volume Breakout</span>
          {/* Candle count since reference */}
          {candlesSinceReference !== undefined && candlesSinceReference > 0 && (
            <span className="px-1.5 py-0.5 bg-gray-700/50 text-gray-300 rounded text-[10px] font-mono">
              {candlesSinceReference} candle{candlesSinceReference !== 1 ? 's' : ''}
            </span>
          )}
        </div>
        {volumeConditionMet ? (
          <div className="flex items-center gap-1 px-2 py-1 bg-green-500/20 border border-green-500/30 rounded text-xs text-green-400 animate-pulse">
            <TrendingUp className="w-3 h-3" />
            <span>ENTRY TRIGGERED</span>
          </div>
        ) : (
          <div className="text-[10px] text-gray-500">
            Waiting for volume match...
          </div>
        )}
      </div>

      {/* Volume Progress Bar - Primary entry criteria */}
      <VolumeProgressBar
        currentVolumeMultiplier={currentVolumeMultiplier}
        avgVolumeMultiplier={avgVolumeMultiplier}
        referenceVolumeMultiplier={referenceVolumeMultiplier}
        entryThreshold={entryThreshold}
        direction={direction}
      />
    </div>
  );
}
