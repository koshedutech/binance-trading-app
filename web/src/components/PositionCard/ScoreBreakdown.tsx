// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// ScoreBreakdown Component - Visual breakdown of score components

import React, { useMemo } from 'react';
import {
  ScoreBreakdown as ScoreBreakdownType,
  SCORE_MAX_VALUES,
  getScorePercentage,
  getScoreColor,
  getScoreBgColor,
} from '../../types/position-card';

interface ScoreBreakdownProps {
  /** Score breakdown data */
  scores: ScoreBreakdownType;
  /** Whether to show detailed view */
  detailed?: boolean;
  /** Additional CSS classes */
  className?: string;
}

/**
 * ScoreBreakdown displays the additive scoring components.
 * Shows Technical (40) + Context (30) + LLM (20) + History (10) = Final (100)
 */
export default function ScoreBreakdown({ scores, detailed = false, className = '' }: ScoreBreakdownProps) {
  const components = [
    { key: 'technical', label: 'Technical', score: scores.technical, max: SCORE_MAX_VALUES.technical },
    { key: 'context', label: 'Context', score: scores.context, max: SCORE_MAX_VALUES.context },
    { key: 'llm', label: 'LLM', score: scores.llm, max: SCORE_MAX_VALUES.llm },
    { key: 'history', label: 'History', score: scores.history, max: SCORE_MAX_VALUES.history },
  ];

  if (!detailed) {
    // Compact view - just show cards
    return (
      <div className={`flex flex-wrap gap-2 ${className}`}>
        {components.map(({ key, label, score, max }) => (
          <ScoreCard key={key} label={label} score={score} max={max} />
        ))}
      </div>
    );
  }

  // Detailed view with progress bars
  return (
    <div className={`space-y-3 ${className}`}>
      {components.map(({ key, label, score, max }) => (
        <ScoreBar key={key} label={label} score={score} max={max} />
      ))}
      <div className="pt-2 border-t border-gray-700">
        <ScoreBar label="Final Score" score={scores.final} max={SCORE_MAX_VALUES.final} showTotal />
      </div>
    </div>
  );
}

/**
 * ScoreCard - Compact card showing a single score component.
 */
interface ScoreCardProps {
  label: string;
  score: number;
  max: number;
}

function ScoreCard({ label, score, max }: ScoreCardProps) {
  const color = getScoreColor(score, max);
  const bgColor = getScoreBgColor(score, max);

  return (
    <div className={`flex flex-col items-center p-2 rounded-lg bg-gray-800/50 border border-gray-700/50 min-w-[70px]`}>
      <span className="text-xs text-gray-400 mb-1">{label}</span>
      <div className={`flex items-baseline gap-0.5 ${color}`}>
        <span className="text-lg font-bold">{score}</span>
        <span className="text-xs text-gray-500">/{max}</span>
      </div>
      <div className="w-full mt-1.5 h-1 bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${bgColor}`}
          style={{ width: `${getScorePercentage(score, max)}%` }}
        />
      </div>
    </div>
  );
}

/**
 * ScoreBar - Progress bar showing a single score component.
 */
interface ScoreBarProps {
  label: string;
  score: number;
  max: number;
  showTotal?: boolean;
}

function ScoreBar({ label, score, max, showTotal = false }: ScoreBarProps) {
  const percentage = getScorePercentage(score, max);
  const color = getScoreColor(score, max);

  // Determine bar color based on percentage
  const barColor = percentage >= 75 ? 'bg-green-500'
    : percentage >= 50 ? 'bg-yellow-500'
    : percentage >= 25 ? 'bg-orange-500'
    : 'bg-red-500';

  return (
    <div className="space-y-1">
      <div className="flex justify-between items-center">
        <span className={`text-sm ${showTotal ? 'font-semibold text-gray-200' : 'text-gray-400'}`}>
          {label}
        </span>
        <span className={`text-sm font-mono ${color}`}>
          {score}<span className="text-gray-500">/{max}</span>
        </span>
      </div>
      <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full rounded-full transition-all duration-500 ${barColor}`}
          style={{ width: `${percentage}%` }}
        />
      </div>
    </div>
  );
}

/**
 * FinalScoreBadge - Compact badge showing just the final score.
 */
interface FinalScoreBadgeProps {
  score: number;
  className?: string;
}

export function FinalScoreBadge({ score, className = '' }: FinalScoreBadgeProps) {
  const color = getScoreColor(score, SCORE_MAX_VALUES.final);
  const bgColor = getScoreBgColor(score, SCORE_MAX_VALUES.final);

  return (
    <span
      className={`
        inline-flex items-center gap-1 px-2 py-0.5 rounded text-sm font-medium
        ${bgColor} ${color} ${className}
      `}
    >
      Score: {score}/100
    </span>
  );
}

/**
 * ScoreRing - Circular progress indicator for the final score.
 */
interface ScoreRingProps {
  score: number;
  size?: number;
  strokeWidth?: number;
  className?: string;
}

export function ScoreRing({ score, size = 60, strokeWidth = 4, className = '' }: ScoreRingProps) {
  // Memoize SVG calculations to avoid recalculating on every render
  const { percentage, radius, circumference, offset, strokeColor } = useMemo(() => {
    const pct = getScorePercentage(score, SCORE_MAX_VALUES.final);
    const r = (size - strokeWidth) / 2;
    const circ = r * 2 * Math.PI;
    const off = circ - (pct / 100) * circ;

    // Color based on score
    const color = pct >= 75 ? '#22c55e' // green
      : pct >= 50 ? '#eab308' // yellow
      : pct >= 25 ? '#f97316' // orange
      : '#ef4444'; // red

    return { percentage: pct, radius: r, circumference: circ, offset: off, strokeColor: color };
  }, [score, size, strokeWidth]);

  return (
    <div className={`relative inline-flex items-center justify-center ${className}`}>
      <svg width={size} height={size} className="transform -rotate-90">
        {/* Background circle */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke="#374151"
          strokeWidth={strokeWidth}
          fill="none"
        />
        {/* Progress circle */}
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke={strokeColor}
          strokeWidth={strokeWidth}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          className="transition-all duration-500"
        />
      </svg>
      <span className="absolute text-sm font-bold text-gray-200">{score}</span>
    </div>
  );
}
