// Score Display Component
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement
// Displays score bar visualization with threshold line and gradient coloring

import React from 'react';
import {
  TrendingUp,
  TrendingDown,
  Minus,
  CheckCircle,
  XCircle,
} from 'lucide-react';
import type { CoinMatch } from '../../types/entryDecision';

// ==================== Interfaces ====================

interface ScoreDisplayProps {
  /** Score value (0-100) */
  score: number;
  /** Threshold for "ready" state */
  threshold?: number;
  /** Whether the score meets threshold */
  ready?: boolean;
  /** Trade direction */
  direction?: 'long' | 'short' | 'neutral';
  /** Size variant */
  size?: 'sm' | 'md' | 'lg';
  /** Show threshold line */
  showThreshold?: boolean;
  /** Show direction indicator */
  showDirection?: boolean;
  /** Show ready indicator */
  showReadyIndicator?: boolean;
}

interface ScoreDisplayFromCoinProps {
  /** Coin match data */
  coin: CoinMatch;
  /** Threshold for "ready" state */
  threshold?: number;
  /** Size variant */
  size?: 'sm' | 'md' | 'lg';
}

interface ScoreBreakdownBarProps {
  /** Component name */
  name: string;
  /** Component score */
  score: number;
  /** Maximum possible score */
  maxScore: number;
  /** Color for the bar */
  color?: string;
}

// ==================== Size Configurations ====================

const sizeConfig = {
  sm: {
    barHeight: 'h-1.5',
    text: 'text-xs',
    scoreText: 'text-sm',
    padding: 'p-1.5',
    gap: 'gap-1',
  },
  md: {
    barHeight: 'h-2',
    text: 'text-sm',
    scoreText: 'text-lg',
    padding: 'p-2',
    gap: 'gap-2',
  },
  lg: {
    barHeight: 'h-3',
    text: 'text-base',
    scoreText: 'text-2xl',
    padding: 'p-3',
    gap: 'gap-3',
  },
};

// ==================== Helper Functions ====================

/**
 * Get color class based on score and threshold
 */
function getScoreColor(score: number, threshold: number = 55): string {
  if (score >= threshold) return 'text-green-400';
  if (score >= threshold - 15) return 'text-yellow-400';
  if (score >= threshold - 30) return 'text-orange-400';
  return 'text-red-400';
}

/**
 * Get bar gradient based on score
 */
function getBarGradient(score: number, threshold: number = 55): string {
  if (score >= threshold) {
    return 'bg-gradient-to-r from-green-500 to-green-400';
  }
  if (score >= threshold - 15) {
    return 'bg-gradient-to-r from-yellow-500 to-yellow-400';
  }
  if (score >= threshold - 30) {
    return 'bg-gradient-to-r from-orange-500 to-orange-400';
  }
  return 'bg-gradient-to-r from-red-500 to-red-400';
}

/**
 * Get direction icon and color
 */
function getDirectionInfo(direction?: 'long' | 'short' | 'neutral'): {
  icon: React.ReactNode;
  color: string;
  label: string;
} {
  switch (direction) {
    case 'long':
      return {
        icon: <TrendingUp className="w-4 h-4" />,
        color: 'text-green-400',
        label: 'Long',
      };
    case 'short':
      return {
        icon: <TrendingDown className="w-4 h-4" />,
        color: 'text-red-400',
        label: 'Short',
      };
    default:
      return {
        icon: <Minus className="w-4 h-4" />,
        color: 'text-gray-400',
        label: 'Neutral',
      };
  }
}

// ==================== Main Component ====================

export default function ScoreDisplay({
  score,
  threshold = 55,
  ready,
  direction,
  size = 'md',
  showThreshold = true,
  showDirection = false,
  showReadyIndicator = true,
}: ScoreDisplayProps) {
  const config = sizeConfig[size];
  const scoreColor = getScoreColor(score, threshold);
  const barGradient = getBarGradient(score, threshold);
  const directionInfo = getDirectionInfo(direction);
  const isReady = ready !== undefined ? ready : score >= threshold;

  // Calculate threshold position (percentage)
  const thresholdPosition = Math.min(100, Math.max(0, threshold));

  return (
    <div className={`flex flex-col ${config.gap}`}>
      {/* Score value and indicators row */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {/* Score value */}
          <span className={`font-bold ${scoreColor} ${config.scoreText}`}>
            {score}
          </span>
          <span className={`text-gray-500 ${config.text}`}>
            / 100
          </span>

          {/* Direction indicator */}
          {showDirection && direction && (
            <span className={`flex items-center gap-1 ${directionInfo.color}`}>
              {directionInfo.icon}
              <span className={config.text}>{directionInfo.label}</span>
            </span>
          )}
        </div>

        {/* Ready indicator */}
        {showReadyIndicator && (
          <span className={`flex items-center gap-1 ${config.text} ${isReady ? 'text-green-400' : 'text-gray-500'}`}>
            {isReady ? (
              <>
                <CheckCircle className="w-4 h-4" />
                <span>Ready</span>
              </>
            ) : (
              <>
                <XCircle className="w-4 h-4" />
                <span>Not Ready</span>
              </>
            )}
          </span>
        )}
      </div>

      {/* Score bar */}
      <div className="relative">
        {/* Background bar */}
        <div className={`w-full ${config.barHeight} bg-gray-700 rounded-full overflow-hidden`}>
          {/* Score fill */}
          <div
            className={`${config.barHeight} ${barGradient} rounded-full transition-all duration-300`}
            style={{ width: `${Math.min(100, Math.max(0, score))}%` }}
          />
        </div>

        {/* Threshold line */}
        {showThreshold && (
          <div
            className="absolute top-0 w-0.5 bg-white/60 rounded-full"
            style={{
              left: `${thresholdPosition}%`,
              height: size === 'sm' ? '0.375rem' : size === 'md' ? '0.5rem' : '0.75rem',
              transform: 'translateX(-50%)',
            }}
            title={`Threshold: ${threshold}`}
          />
        )}
      </div>

      {/* Threshold label */}
      {showThreshold && (
        <div className="flex justify-between text-[10px] text-gray-500">
          <span>0</span>
          <span
            style={{ marginLeft: `${thresholdPosition - 5}%` }}
            className="text-gray-400"
          >
            Threshold: {threshold}
          </span>
          <span>100</span>
        </div>
      )}
    </div>
  );
}

// ==================== From Coin Match Variant ====================

/**
 * ScoreDisplay component that accepts a CoinMatch object
 */
export function ScoreDisplayFromCoin({
  coin,
  threshold = 55,
  size = 'md',
}: ScoreDisplayFromCoinProps) {
  // If no score data, return placeholder
  if (coin.score === undefined) {
    return (
      <div className="text-xs text-gray-500">
        No score data
      </div>
    );
  }

  return (
    <ScoreDisplay
      score={coin.score}
      threshold={threshold}
      ready={coin.ready}
      direction={coin.direction as 'long' | 'short' | 'neutral' | undefined}
      size={size}
    />
  );
}

// ==================== Compact Inline Variant ====================

interface ScoreDisplayCompactProps {
  /** Score value (0-100) */
  score: number;
  /** Threshold for "ready" state */
  threshold?: number;
  /** Whether the score meets threshold */
  ready?: boolean;
}

/**
 * Compact inline score display
 */
export function ScoreDisplayCompact({
  score,
  threshold = 55,
  ready,
}: ScoreDisplayCompactProps) {
  const isReady = ready !== undefined ? ready : score >= threshold;
  const scoreColor = getScoreColor(score, threshold);

  return (
    <div className="flex items-center gap-2">
      <span className={`text-sm font-bold ${scoreColor}`}>
        {score}
      </span>
      <span className="text-xs text-gray-500">/ {threshold}</span>
      {isReady && (
        <CheckCircle className="w-3.5 h-3.5 text-green-400" />
      )}
    </div>
  );
}

// ==================== Score Breakdown Bar ====================

/**
 * Individual score component breakdown bar
 */
export function ScoreBreakdownBar({
  name,
  score,
  maxScore,
  color = 'bg-blue-500',
}: ScoreBreakdownBarProps) {
  const percentage = maxScore > 0 ? (score / maxScore) * 100 : 0;

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-gray-400 w-20 truncate" title={name}>
        {name}
      </span>
      <div className="flex-1 h-1.5 bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full ${color} rounded-full transition-all duration-300`}
          style={{ width: `${Math.min(100, percentage)}%` }}
        />
      </div>
      <span className="text-xs text-gray-500 w-10 text-right">
        {score}/{maxScore}
      </span>
    </div>
  );
}

// ==================== Full Score Breakdown ====================

interface ScoreBreakdownFullProps {
  /** Score components */
  components: Record<string, number>;
  /** Maximum scores for each component */
  maxScores?: Record<string, number>;
  /** Total score */
  totalScore: number;
  /** Threshold */
  threshold?: number;
}

/**
 * Full score breakdown with all components
 */
export function ScoreBreakdownFull({
  components,
  maxScores = {},
  totalScore,
  threshold = 55,
}: ScoreBreakdownFullProps) {
  // Default max scores if not provided
  const defaultMaxScores: Record<string, number> = {
    technical: 40,
    context: 30,
    llm: 20,
    history: 10,
  };

  const componentColors: Record<string, string> = {
    technical: 'bg-blue-500',
    context: 'bg-purple-500',
    llm: 'bg-cyan-500',
    history: 'bg-yellow-500',
  };

  return (
    <div className="space-y-3">
      {/* Total score display */}
      <ScoreDisplay
        score={totalScore}
        threshold={threshold}
        size="sm"
        showDirection={false}
      />

      {/* Component breakdown */}
      <div className="space-y-1.5">
        {Object.entries(components).map(([name, value]) => (
          <ScoreBreakdownBar
            key={name}
            name={name.charAt(0).toUpperCase() + name.slice(1)}
            score={value}
            maxScore={maxScores[name] || defaultMaxScores[name] || 100}
            color={componentColors[name] || 'bg-gray-500'}
          />
        ))}
      </div>
    </div>
  );
}
