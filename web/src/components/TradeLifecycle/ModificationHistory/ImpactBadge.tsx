// Story 7.13: Impact Badge Component
// Displays dollar impact with color coding and trend icons

import React from 'react';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';
import {
  ImpactBadgeProps,
  ImpactDirection,
  ModifiableOrderType,
  getImpactColor,
  getImpactBgColor,
  formatDollarImpact,
} from './types';

interface ImpactBadgeComponentProps extends ImpactBadgeProps {
  orderType?: ModifiableOrderType;
}

export default function ImpactBadge({
  amount,
  direction,
  showTrend = true,
  size = 'md',
  orderType = 'SL',
}: ImpactBadgeComponentProps) {
  // Get colors based on impact direction and order type
  const textColor = getImpactColor(direction, orderType);
  const bgColor = getImpactBgColor(direction, orderType);

  // Size variants
  const sizeClasses = {
    sm: 'px-1.5 py-0.5 text-xs',
    md: 'px-2 py-1 text-sm',
    lg: 'px-3 py-1.5 text-base',
  };

  const iconSizes = {
    sm: 'w-3 h-3',
    md: 'w-3.5 h-3.5',
    lg: 'w-4 h-4',
  };

  // Determine trend icon
  const getTrendIcon = () => {
    if (!showTrend || direction === 'INITIAL') {
      return null;
    }

    // Positive impact (favorable)
    if (direction === 'BETTER' || direction === 'TIGHTER') {
      return <TrendingUp className={`${iconSizes[size]} text-green-400`} />;
    }

    // Negative impact (unfavorable)
    if (direction === 'WORSE' || direction === 'WIDER') {
      return <TrendingDown className={`${iconSizes[size]} text-red-400`} />;
    }

    return <Minus className={`${iconSizes[size]} text-gray-400`} />;
  };

  // Format amount for display
  const displayAmount = formatDollarImpact(amount);

  return (
    <span
      className={`inline-flex items-center gap-1 font-mono font-medium rounded ${sizeClasses[size]} ${bgColor} ${textColor}`}
    >
      {getTrendIcon()}
      <span>{displayAmount}</span>
    </span>
  );
}

// Compact version for inline display
export function ImpactBadgeCompact({
  amount,
  direction,
  orderType = 'SL',
}: {
  amount: number;
  direction: ImpactDirection;
  orderType?: ModifiableOrderType;
}) {
  const textColor = getImpactColor(direction, orderType);
  const displayAmount = formatDollarImpact(amount);

  return (
    <span className={`font-mono text-xs ${textColor}`}>
      {displayAmount}
    </span>
  );
}

// Price delta badge (shows price change, not dollar impact)
export function PriceDeltaBadge({
  delta,
  percent,
  direction,
  orderType = 'SL',
  size = 'sm',
}: {
  delta: number | null;
  percent: number | null;
  direction: ImpactDirection;
  orderType?: ModifiableOrderType;
  size?: 'sm' | 'md';
}) {
  if (delta === null || direction === 'INITIAL') {
    return (
      <span className="text-gray-500 text-xs font-mono">
        (initial)
      </span>
    );
  }

  const textColor = getImpactColor(direction, orderType);
  const sign = delta >= 0 ? '+' : '';

  const sizeClass = size === 'sm' ? 'text-xs' : 'text-sm';

  return (
    <span className={`font-mono ${sizeClass} ${textColor}`}>
      {sign}${Math.abs(delta).toFixed(2)}
      {percent !== null && (
        <span className="text-gray-500 ml-1">
          ({sign}{percent.toFixed(2)}%)
        </span>
      )}
    </span>
  );
}
