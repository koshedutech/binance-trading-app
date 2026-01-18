// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// RegimeBadge Component - Color-coded market regime indicator

import React from 'react';
import {
  MarketRegime,
  REGIME_DISPLAY_NAMES,
  REGIME_COLORS,
} from '../../types/position-card';

interface RegimeBadgeProps {
  /** Market regime to display */
  regime: MarketRegime;
  /** Whether to show full text or abbreviated */
  compact?: boolean;
  /** Additional CSS classes */
  className?: string;
}

/**
 * RegimeBadge displays the current market regime with appropriate color coding.
 * Regimes indicate the market condition and help determine strategy selection.
 */
export default function RegimeBadge({ regime, compact = false, className = '' }: RegimeBadgeProps) {
  const colors = REGIME_COLORS[regime];
  const displayName = REGIME_DISPLAY_NAMES[regime];

  // Icon based on regime type
  const getRegimeIcon = () => {
    switch (regime) {
      case 'TRENDING':
        return (
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
          </svg>
        );
      case 'RANGING':
        return (
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M8 7h12M8 12h12M8 17h12" />
          </svg>
        );
      case 'VOLATILE':
        return (
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        );
      case 'CONSOLIDATING':
        return (
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
        );
      default:
        return null;
    }
  };

  return (
    <span
      className={`
        inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-semibold
        ${colors.bg} ${colors.text} ${colors.border} border
        transition-all duration-200
        ${className}
      `}
      title={`Market Regime: ${displayName}`}
    >
      {getRegimeIcon()}
      {compact ? regime.substring(0, 3) : displayName}
    </span>
  );
}

/**
 * RegimeBadgeSmall - A more compact version for tight spaces.
 */
export function RegimeBadgeSmall({ regime }: { regime: MarketRegime }) {
  const colors = REGIME_COLORS[regime];

  return (
    <span
      className={`inline-flex items-center justify-center w-6 h-6 rounded-full ${colors.bg} ${colors.text}`}
      title={REGIME_DISPLAY_NAMES[regime]}
    >
      {regime.charAt(0)}
    </span>
  );
}
