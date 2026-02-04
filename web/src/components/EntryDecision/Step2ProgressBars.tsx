// Step 2 Progress Bars Component
// Epic 14: Shows real-time volume and price progress in Step 2 (Breakout Detection)
// Displays progress toward reference candle values for entry trigger

import React from 'react';
import { Activity, Target, TrendingUp, TrendingDown } from 'lucide-react';
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
}

function VolumeProgressBar({
  currentVolumeMultiplier,
  avgVolumeMultiplier,
  referenceVolumeMultiplier,
  entryThreshold,
}: VolumeProgressBarProps) {
  // Bar range: 1x to reference volume (e.g., 3.5x)
  const minValue = 1;
  const maxValue = referenceVolumeMultiplier;
  const range = maxValue - minValue;

  // Calculate positions as percentages
  const avgPosition = range > 0 ? ((avgVolumeMultiplier - minValue) / range) * 100 : 50;
  const currentPosition = range > 0 ? Math.min(((currentVolumeMultiplier - minValue) / range) * 100, 100) : 0;
  const entryPosition = range > 0 ? ((entryThreshold - minValue) / range) * 100 : 66;

  // Progress percentage (current toward reference)
  const progressPercent = range > 0 ? Math.min((currentVolumeMultiplier / referenceVolumeMultiplier) * 100, 100) : 0;

  // Determine color based on progress
  const getProgressColor = () => {
    if (currentVolumeMultiplier >= referenceVolumeMultiplier) return 'bg-green-500';
    if (currentVolumeMultiplier >= entryThreshold) return 'bg-yellow-500';
    if (currentVolumeMultiplier >= avgVolumeMultiplier) return 'bg-blue-500';
    return 'bg-blue-400/50';
  };

  const isConditionMet = currentVolumeMultiplier >= entryThreshold;

  return (
    <div className="mb-3">
      {/* Header */}
      <div className="flex items-center justify-between mb-1.5">
        <div className="flex items-center gap-2 text-xs">
          <Activity className="w-3.5 h-3.5 text-blue-400" />
          <span className="text-gray-300 font-medium">Volume Progress</span>
          {isConditionMet && (
            <span className="px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded text-[10px]">
              MET
            </span>
          )}
        </div>
        <div className="text-xs text-gray-400">
          <span className="text-white font-mono">{currentVolumeMultiplier.toFixed(2)}x</span>
          <span className="mx-1">/</span>
          <span className="text-green-400 font-mono">{referenceVolumeMultiplier.toFixed(2)}x</span>
          <span className="ml-2 text-gray-500">({progressPercent.toFixed(0)}%)</span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="relative h-5 bg-gray-700/50 rounded-lg overflow-visible">
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
            Ref→Last {avgVolumeMultiplier.toFixed(1)}x
          </div>
        </div>

        {/* Volume Threshold Marker (required for entry trigger) */}
        {Math.abs(entryThreshold - referenceVolumeMultiplier) > 0.1 && (
          <div
            className="absolute top-0 bottom-0 flex flex-col items-center"
            style={{ left: `${Math.min(Math.max(entryPosition, 5), 95)}%`, transform: 'translateX(-50%)' }}
          >
            <div className="w-1 h-full bg-yellow-500/70" />
            <div className="absolute -top-4 text-[9px] text-yellow-400 whitespace-nowrap">
              Threshold {entryThreshold.toFixed(1)}x
            </div>
          </div>
        )}

        {/* Current Volume Indicator (floating circle) */}
        <div
          className={`absolute top-1/2 -translate-y-1/2 w-4 h-4 rounded-full border-2 border-white shadow-lg transition-all duration-200 ${
            isConditionMet ? 'bg-green-500' : 'bg-blue-500'
          }`}
          style={{ left: `${Math.min(Math.max(currentPosition, 2), 98)}%`, transform: 'translateX(-50%) translateY(-50%)' }}
        />

        {/* Scale Labels */}
        <div className="absolute -bottom-4 left-0 text-[9px] text-gray-500">1x</div>
        <div className="absolute -bottom-4 right-0 text-[9px] text-green-400 font-medium">
          {referenceVolumeMultiplier.toFixed(1)}x
        </div>
      </div>

      {/* Spacer for labels */}
      <div className="h-5" />
    </div>
  );
}

// ==================== Price Progress Bar ====================

interface PriceProgressBarProps {
  /** Current market price */
  currentPrice: number;
  /** Day low price */
  dayLow: number;
  /** Day high price */
  dayHigh: number;
  /** Reference candle high (entry level) */
  entryPrice: number;
  /** Average price from reference to last candle */
  avgPrice: number;
  /** Trade direction */
  direction: string;
}

function PriceProgressBar({
  currentPrice,
  dayLow,
  dayHigh,
  entryPrice,
  avgPrice,
  direction,
}: PriceProgressBarProps) {
  const isLong = direction === 'long';

  // Add buffer to range
  const buffer = (dayHigh - dayLow) * 0.05;
  const minValue = dayLow - buffer;
  const maxValue = dayHigh + buffer;
  const range = maxValue - minValue;

  // Calculate positions as percentages
  const entryPosition = range > 0 ? ((entryPrice - minValue) / range) * 100 : 50;
  const avgPosition = range > 0 ? ((avgPrice - minValue) / range) * 100 : 40;
  const currentPosition = range > 0 ? ((currentPrice - minValue) / range) * 100 : 50;
  const dayLowPosition = range > 0 ? ((dayLow - minValue) / range) * 100 : 0;
  const dayHighPosition = range > 0 ? ((dayHigh - minValue) / range) * 100 : 100;

  // Distance to entry
  const distanceToEntry = ((entryPrice - currentPrice) / currentPrice) * 100;
  const isAtOrAboveEntry = isLong ? currentPrice >= entryPrice : currentPrice <= entryPrice;

  // Determine current price color
  const getCurrentColor = () => {
    if (isAtOrAboveEntry) return 'bg-green-500';
    if (Math.abs(distanceToEntry) < 0.5) return 'bg-yellow-500';
    return 'bg-blue-500';
  };

  // Format price
  const formatPrice = (price: number) => {
    if (price >= 1000) return `$${price.toFixed(0)}`;
    if (price >= 1) return `$${price.toFixed(2)}`;
    return `$${price.toFixed(4)}`;
  };

  return (
    <div className="mb-2">
      {/* Header */}
      <div className="flex items-center justify-between mb-1.5">
        <div className="flex items-center gap-2 text-xs">
          <Target className="w-3.5 h-3.5 text-purple-400" />
          <span className="text-gray-300 font-medium">Price Progress</span>
          <span className={`px-1.5 py-0.5 rounded text-[10px] ${
            isLong ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
          }`}>
            {direction.toUpperCase()}
          </span>
          {isAtOrAboveEntry && (
            <span className="px-1.5 py-0.5 bg-green-500/20 text-green-400 rounded text-[10px]">
              MET
            </span>
          )}
        </div>
        <div className="text-xs text-gray-400 flex items-center gap-1">
          <span className="text-white font-mono">{formatPrice(currentPrice)}</span>
          <span className="mx-1">→</span>
          <span className="text-purple-400 font-mono">{formatPrice(entryPrice)}</span>
          <span className={`ml-1 ${isAtOrAboveEntry ? 'text-green-400' : 'text-gray-500'}`}>
            ({distanceToEntry > 0 ? '+' : ''}{distanceToEntry.toFixed(2)}%)
          </span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="relative h-5 bg-gray-700/50 rounded-lg overflow-visible">
        {/* Day Low Marker */}
        <div
          className="absolute top-0 bottom-0"
          style={{ left: `${dayLowPosition}%` }}
        >
          <div className="w-0.5 h-full bg-red-500/50" />
        </div>

        {/* Day High Marker */}
        <div
          className="absolute top-0 bottom-0"
          style={{ left: `${dayHighPosition}%` }}
        >
          <div className="w-0.5 h-full bg-green-500/50" />
        </div>

        {/* Average Price Marker (Ref→Last candle average) */}
        <div
          className="absolute top-0 bottom-0 flex flex-col items-center"
          style={{ left: `${Math.min(Math.max(avgPosition, 5), 95)}%`, transform: 'translateX(-50%)' }}
        >
          <div className="w-0.5 h-full bg-gray-400/70" />
          <div className="absolute -bottom-4 text-[9px] text-gray-400 whitespace-nowrap">
            Ref→Last
          </div>
        </div>

        {/* Reference Candle High Price Marker (Breakout Entry Level) */}
        <div
          className="absolute top-0 bottom-0 flex flex-col items-center z-10"
          style={{ left: `${Math.min(Math.max(entryPosition, 5), 95)}%`, transform: 'translateX(-50%)' }}
        >
          <div className="w-1.5 h-full bg-purple-500" />
          <div className="absolute -top-4 text-[9px] text-purple-400 font-medium whitespace-nowrap">
            Ref High
          </div>
        </div>

        {/* Current Price Indicator (floating circle) */}
        <div
          className={`absolute top-1/2 -translate-y-1/2 w-4 h-4 rounded-full border-2 border-white shadow-lg transition-all duration-200 z-20 ${getCurrentColor()}`}
          style={{ left: `${Math.min(Math.max(currentPosition, 2), 98)}%`, transform: 'translateX(-50%) translateY(-50%)' }}
        />

        {/* Scale Labels - Day Low and Day High */}
        <div className="absolute -bottom-4 left-0 text-[9px] text-red-400 whitespace-nowrap">
          Day Low {formatPrice(dayLow)}
        </div>
        <div className="absolute -bottom-4 right-0 text-[9px] text-green-400 whitespace-nowrap">
          Day High {formatPrice(dayHigh)}
        </div>
      </div>

      {/* Spacer for labels */}
      <div className="h-5" />
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
        <div className="h-8 bg-gray-700/30 rounded" />
      </div>
    );
  }

  // Use reference candle mid price as fallback if currentPrice is missing/0
  const effectiveCurrentPrice = currentPrice > 0
    ? currentPrice
    : (referenceCandle.high + referenceCandle.low) / 2;

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

  // Price data
  const dayLow = update.day_low || referenceCandle.low * 0.98;
  const dayHigh = update.day_high || referenceCandle.high * 1.02;
  const entryPrice = referenceCandle.high; // Entry at reference high for longs

  // Average price from reference to last candle
  const avgPrice = update.consolidation_avg_price
    || (referenceCandle.open + referenceCandle.close) / 2; // Fallback: ref candle mid

  const direction = update.direction || 'long';

  // Check if both conditions are met
  const volumeConditionMet = currentVolumeMultiplier >= entryThreshold;
  const priceConditionMet = direction === 'long'
    ? effectiveCurrentPrice >= entryPrice
    : effectiveCurrentPrice <= entryPrice;
  const bothConditionsMet = volumeConditionMet && priceConditionMet;

  return (
    <div className="mt-4 pt-4 border-t border-gray-700/50">
      {/* Section Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2 text-xs text-gray-400">
          <TrendingUp className="w-3.5 h-3.5" />
          <span>Step 2 Entry Progress</span>
          {/* Candle count since reference */}
          {candlesSinceReference !== undefined && candlesSinceReference > 0 && (
            <span className="px-1.5 py-0.5 bg-gray-700/50 text-gray-300 rounded text-[10px] font-mono">
              🕯 {candlesSinceReference} candle{candlesSinceReference !== 1 ? 's' : ''}
            </span>
          )}
        </div>
        {bothConditionsMet ? (
          <div className="flex items-center gap-1 px-2 py-1 bg-green-500/20 border border-green-500/30 rounded text-xs text-green-400 animate-pulse">
            <TrendingUp className="w-3 h-3" />
            <span>ENTRY TRIGGERED</span>
          </div>
        ) : (
          <div className="text-[10px] text-gray-500">
            {volumeConditionMet ? 'Vol ' : ''}{priceConditionMet ? 'Price ' : ''}
            {!volumeConditionMet && !priceConditionMet ? 'Waiting for conditions...' : 'waiting...'}
          </div>
        )}
      </div>

      {/* Volume Progress Bar */}
      <VolumeProgressBar
        currentVolumeMultiplier={currentVolumeMultiplier}
        avgVolumeMultiplier={avgVolumeMultiplier}
        referenceVolumeMultiplier={referenceVolumeMultiplier}
        entryThreshold={entryThreshold}
      />

      {/* Price Progress Bar */}
      <PriceProgressBar
        currentPrice={effectiveCurrentPrice}
        dayLow={dayLow}
        dayHigh={dayHigh}
        entryPrice={entryPrice}
        avgPrice={avgPrice}
        direction={direction}
      />

      {/* Summary Status */}
      <div className="flex items-center justify-between text-[10px] mt-2 pt-2 border-t border-gray-700/30">
        <div className="flex items-center gap-3">
          <span className={`flex items-center gap-1 ${volumeConditionMet ? 'text-green-400' : 'text-gray-500'}`}>
            <span className={`w-2 h-2 rounded-full ${volumeConditionMet ? 'bg-green-500' : 'bg-gray-600'}`} />
            Volume {volumeConditionMet ? '100%' : `${((currentVolumeMultiplier / referenceVolumeMultiplier) * 100).toFixed(0)}%`}
          </span>
          <span className={`flex items-center gap-1 ${priceConditionMet ? 'text-green-400' : 'text-gray-500'}`}>
            <span className={`w-2 h-2 rounded-full ${priceConditionMet ? 'bg-green-500' : 'bg-gray-600'}`} />
            Price {priceConditionMet ? 'Breakout!' : `${Math.abs(((entryPrice - effectiveCurrentPrice) / effectiveCurrentPrice) * 100).toFixed(2)}% away`}
          </span>
        </div>
        <span className="text-gray-600">
          {bothConditionsMet ? 'Entry conditions MET' : 'Both required for entry'}
        </span>
      </div>
    </div>
  );
}
