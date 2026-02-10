// Reference Candle Context Component
// Epic 14: Chain Trading System - Stage 2 Context Display
// Shows Stage 1 reference candle information during Stage 2 (Breakout detection)
// Helps visualize where the entry point is relative to current price

import React from 'react';
import {
  Activity,
  TrendingUp,
  Target,
  ArrowUp,
  ArrowDown,
  CheckCircle,
} from 'lucide-react';
import type { ReferenceCandle } from '../../types/entryDecision';

// ==================== Interfaces ====================

interface ReferenceCandleContextProps {
  /** Reference candle data from Stage 1 */
  referenceCandle: ReferenceCandle;
  /** Current market price */
  currentPrice: number;
  /** Day high price for range display */
  dayHigh?: number;
  /** Day low price for range display */
  dayLow?: number;
  /** Volume threshold required (e.g., 3.0 for 3x) */
  volumeThreshold?: number;
  /** Trade direction */
  direction?: string;
}

// ==================== Volume Reference Bar ====================

interface VolumeReferenceBarProps {
  /** Actual volume multiplier achieved (e.g., 3.5x) */
  actualMultiplier: number;
  /** Required threshold (e.g., 3.0x) */
  requiredThreshold: number;
}

function VolumeReferenceBar({ actualMultiplier, requiredThreshold }: VolumeReferenceBarProps) {
  // Calculate position of the actual multiplier on the bar
  // Bar shows 0x to (threshold + 1x) range
  const maxDisplay = requiredThreshold + 1.5;
  const thresholdPercent = (requiredThreshold / maxDisplay) * 100;
  const actualPercent = Math.min((actualMultiplier / maxDisplay) * 100, 100);

  // Determine bar color based on how much over threshold
  const overageRatio = actualMultiplier / requiredThreshold;
  const getBarColor = () => {
    if (overageRatio >= 1.2) return 'bg-green-500'; // 20%+ over threshold
    if (overageRatio >= 1.0) return 'bg-yellow-500'; // At or above threshold
    return 'bg-orange-500'; // Below threshold (shouldn't happen in Stage 2)
  };

  return (
    <div className="mb-3">
      {/* Label Row */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2 text-xs">
          <Activity className="w-3 h-3 text-green-400" />
          <span className="text-gray-400">Stage 1 Volume Spike</span>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <span className="text-green-400 font-mono font-bold">
            {actualMultiplier.toFixed(2)}x
          </span>
          <span className="text-gray-500">achieved</span>
          <span className="text-gray-600">|</span>
          <span className="text-gray-400 font-mono">{requiredThreshold.toFixed(1)}x</span>
          <span className="text-gray-500">required</span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="relative h-3 bg-gray-700/50 rounded-full overflow-hidden">
        {/* Filled portion up to actual */}
        <div
          className={`absolute inset-y-0 left-0 rounded-full transition-all duration-300 ${getBarColor()}`}
          style={{ width: `${actualPercent}%` }}
        />

        {/* Threshold marker line */}
        <div
          className="absolute inset-y-0 w-0.5 bg-white/50"
          style={{ left: `${thresholdPercent}%` }}
          title={`Required: ${requiredThreshold}x`}
        />

        {/* Actual value marker - diamond shape */}
        <div
          className="absolute top-1/2 -translate-y-1/2 w-2 h-2 bg-green-400 rotate-45 border border-green-300"
          style={{ left: `${actualPercent}%`, transform: 'translateX(-50%) translateY(-50%) rotate(45deg)' }}
          title={`Achieved: ${actualMultiplier.toFixed(2)}x`}
        />
      </div>

      {/* Scale markers */}
      <div className="flex items-center justify-between mt-0.5 text-[9px] text-gray-600">
        <span>0x</span>
        <span style={{ marginLeft: `${thresholdPercent - 5}%` }}>{requiredThreshold}x</span>
        <span>{maxDisplay.toFixed(1)}x</span>
      </div>

      {/* Spike confirmed badge */}
      <div className="flex items-center gap-1 mt-1 text-[10px] text-green-400">
        <CheckCircle className="w-3 h-3" />
        <span>Volume spike confirmed @ {actualMultiplier.toFixed(2)}x avg</span>
      </div>
    </div>
  );
}

// ==================== Price Reference Bar ====================

interface PriceReferenceBarProps {
  /** Reference candle high (entry level for longs) */
  referenceHigh: number;
  /** Reference candle low */
  referenceLow: number;
  /** Current market price */
  currentPrice: number;
  /** Day high for range */
  dayHigh: number;
  /** Day low for range */
  dayLow: number;
  /** Trade direction */
  direction: string;
}

function PriceReferenceBar({
  referenceHigh,
  referenceLow,
  currentPrice,
  dayHigh,
  dayLow,
  direction,
}: PriceReferenceBarProps) {
  const isLong = direction === 'long';
  const entryLevel = isLong ? referenceHigh : referenceLow;

  // Calculate positions on the bar
  const range = dayHigh - dayLow;
  const buffer = range * 0.1; // 10% buffer on each side
  const displayLow = dayLow - buffer;
  const displayHigh = dayHigh + buffer;
  const displayRange = displayHigh - displayLow;

  const entryPercent = ((entryLevel - displayLow) / displayRange) * 100;
  const currentPercent = ((currentPrice - displayLow) / displayRange) * 100;
  const refHighPercent = ((referenceHigh - displayLow) / displayRange) * 100;
  const refLowPercent = ((referenceLow - displayLow) / displayRange) * 100;

  // Distance from current price to entry level
  const distanceToEntry = ((entryLevel - currentPrice) / currentPrice) * 100;
  const isAboveEntry = currentPrice > entryLevel;
  const isNearEntry = Math.abs(distanceToEntry) < 0.5; // Within 0.5%

  // Determine current price indicator color
  const getCurrentColor = () => {
    if (isNearEntry) return 'bg-yellow-400'; // Near entry level
    if (isLong) {
      return isAboveEntry ? 'bg-green-400' : 'bg-blue-400'; // Above entry = good for long
    }
    return isAboveEntry ? 'bg-blue-400' : 'bg-green-400'; // Below entry = good for short
  };

  return (
    <div className="mb-3">
      {/* Label Row */}
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center gap-2 text-xs">
          <Target className="w-3 h-3 text-blue-400" />
          <span className="text-gray-400">Price Context</span>
          <span className={`px-1.5 py-0.5 rounded text-[10px] ${
            isLong ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
          }`}>
            {direction.toUpperCase()}
          </span>
        </div>
        <div className="flex items-center gap-2 text-xs">
          <span className={`font-mono ${isNearEntry ? 'text-yellow-400' : isAboveEntry ? 'text-green-400' : 'text-blue-400'}`}>
            ${currentPrice.toFixed(currentPrice > 100 ? 2 : 4)}
          </span>
          <span className="text-gray-600">|</span>
          <span className={`text-gray-400 flex items-center gap-1`}>
            {distanceToEntry > 0 ? <ArrowUp className="w-3 h-3" /> : <ArrowDown className="w-3 h-3" />}
            {Math.abs(distanceToEntry).toFixed(2)}% to entry
          </span>
        </div>
      </div>

      {/* Price Bar */}
      <div className="relative h-4 bg-gray-700/50 rounded-full overflow-hidden">
        {/* Reference candle range (high to low) - shaded region */}
        <div
          className="absolute inset-y-0 bg-blue-500/20 border-l border-r border-blue-500/30"
          style={{
            left: `${Math.min(refLowPercent, refHighPercent)}%`,
            width: `${Math.abs(refHighPercent - refLowPercent)}%`,
          }}
          title={`Ref candle: $${referenceLow.toFixed(2)} - $${referenceHigh.toFixed(2)}`}
        />

        {/* Entry level marker - vertical line */}
        <div
          className={`absolute inset-y-0 w-1 ${isLong ? 'bg-green-500' : 'bg-red-500'}`}
          style={{ left: `${entryPercent}%`, transform: 'translateX(-50%)' }}
          title={`Entry: $${entryLevel.toFixed(currentPrice > 100 ? 2 : 4)}`}
        />

        {/* Current price marker - triangle/arrow */}
        <div
          className={`absolute top-1/2 -translate-y-1/2 w-3 h-3 ${getCurrentColor()} rounded-full border-2 border-white/50`}
          style={{ left: `${currentPercent}%`, transform: 'translateX(-50%) translateY(-50%)' }}
          title={`Current: $${currentPrice.toFixed(currentPrice > 100 ? 2 : 4)}`}
        />
      </div>

      {/* Price scale */}
      <div className="flex items-center justify-between mt-0.5 text-[9px] text-gray-600">
        <span>${displayLow.toFixed(displayLow > 100 ? 0 : 2)}</span>
        <span className="text-blue-400">${entryLevel.toFixed(entryLevel > 100 ? 2 : 4)} (entry)</span>
        <span>${displayHigh.toFixed(displayHigh > 100 ? 0 : 2)}</span>
      </div>

      {/* Status message */}
      <div className={`flex items-center gap-1 mt-1 text-[10px] ${
        isNearEntry ? 'text-yellow-400' :
        (isLong ? isAboveEntry : !isAboveEntry) ? 'text-green-400' : 'text-blue-400'
      }`}>
        {isNearEntry ? (
          <>
            <Activity className="w-3 h-3 animate-pulse" />
            <span>Near entry level - monitoring for breakout</span>
          </>
        ) : isLong ? (
          isAboveEntry ? (
            <>
              <TrendingUp className="w-3 h-3" />
              <span>Price above entry - potential breakout</span>
            </>
          ) : (
            <>
              <ArrowUp className="w-3 h-3" />
              <span>Price {Math.abs(distanceToEntry).toFixed(2)}% below entry - consolidating</span>
            </>
          )
        ) : (
          !isAboveEntry ? (
            <>
              <ArrowDown className="w-3 h-3" />
              <span>Price below entry - potential breakout</span>
            </>
          ) : (
            <>
              <ArrowUp className="w-3 h-3" />
              <span>Price {Math.abs(distanceToEntry).toFixed(2)}% above entry - consolidating</span>
            </>
          )
        )}
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function ReferenceCandleContext({
  referenceCandle,
  currentPrice,
  dayHigh,
  dayLow,
  volumeThreshold = 3.0,
  direction = 'long',
}: ReferenceCandleContextProps) {
  // Use reference candle prices for range if day high/low not provided
  const effectiveDayHigh = dayHigh || Math.max(referenceCandle.high, currentPrice) * 1.01;
  const effectiveDayLow = dayLow || Math.min(referenceCandle.low, currentPrice) * 0.99;

  return (
    <div className="mt-3 p-3 bg-gray-800/30 rounded-lg border border-gray-700/30">
      <div className="flex items-center gap-2 mb-3 text-xs text-gray-500 border-b border-gray-700/30 pb-2">
        <CheckCircle className="w-3.5 h-3.5 text-green-400" />
        <span>Stage 1 Reference (Completed)</span>
        <span className="ml-auto text-gray-600">
          Candle: {new Date(referenceCandle.open_time).toISOString().slice(11, 19)} UTC
        </span>
      </div>

      {/* Volume Reference Bar */}
      <VolumeReferenceBar
        actualMultiplier={referenceCandle.volume_multiplier}
        requiredThreshold={volumeThreshold}
      />

      {/* Price Reference Bar */}
      <PriceReferenceBar
        referenceHigh={referenceCandle.high}
        referenceLow={referenceCandle.low}
        currentPrice={currentPrice}
        dayHigh={effectiveDayHigh}
        dayLow={effectiveDayLow}
        direction={direction}
      />

      {/* Reference candle summary */}
      <div className="grid grid-cols-4 gap-2 mt-2 pt-2 border-t border-gray-700/30 text-[10px]">
        <div>
          <span className="text-gray-500 block">High</span>
          <span className="text-white font-mono">${referenceCandle.high.toFixed(referenceCandle.high > 100 ? 2 : 4)}</span>
        </div>
        <div>
          <span className="text-gray-500 block">Low</span>
          <span className="text-white font-mono">${referenceCandle.low.toFixed(referenceCandle.low > 100 ? 2 : 4)}</span>
        </div>
        <div>
          <span className="text-gray-500 block">Close</span>
          <span className="text-white font-mono">${referenceCandle.close.toFixed(referenceCandle.close > 100 ? 2 : 4)}</span>
        </div>
        <div>
          <span className="text-gray-500 block">Volume</span>
          <span className="text-green-400 font-mono">{referenceCandle.volume_multiplier.toFixed(2)}x</span>
        </div>
      </div>
    </div>
  );
}

// ==================== Compact Version ====================

interface ReferenceCandleContextCompactProps {
  /** Reference candle data */
  referenceCandle: ReferenceCandle;
  /** Current price */
  currentPrice: number;
  /** Trade direction */
  direction?: string;
}

export function ReferenceCandleContextCompact({
  referenceCandle,
  currentPrice,
  direction = 'long',
}: ReferenceCandleContextCompactProps) {
  const isLong = direction === 'long';
  const entryLevel = isLong ? referenceCandle.high : referenceCandle.low;
  const distanceToEntry = ((entryLevel - currentPrice) / currentPrice) * 100;

  return (
    <div className="flex items-center gap-3 p-2 bg-gray-800/30 rounded border border-gray-700/30 text-xs">
      <div className="flex items-center gap-1 text-green-400">
        <Activity className="w-3 h-3" />
        <span className="font-mono">{referenceCandle.volume_multiplier.toFixed(2)}x</span>
      </div>
      <span className="text-gray-600">|</span>
      <div className="flex items-center gap-1">
        <Target className="w-3 h-3 text-blue-400" />
        <span className="text-gray-400">Entry:</span>
        <span className="text-white font-mono">${entryLevel.toFixed(entryLevel > 100 ? 2 : 4)}</span>
      </div>
      <span className={`ml-auto ${distanceToEntry > 0 ? 'text-blue-400' : 'text-green-400'}`}>
        {distanceToEntry > 0 ? <ArrowUp className="w-3 h-3 inline" /> : <ArrowDown className="w-3 h-3 inline" />}
        {Math.abs(distanceToEntry).toFixed(2)}%
      </span>
    </div>
  );
}
