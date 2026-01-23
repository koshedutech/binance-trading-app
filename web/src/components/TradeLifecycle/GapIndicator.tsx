// Epic 11: Story 11.40 - Gap Analysis UI
// GapIndicator component shows directional gap with visual styling
import { ArrowUp, ArrowDown, Check } from 'lucide-react';
import { GapDirection } from '../../services/futuresApi';

interface GapIndicatorProps {
  gap: number;
  direction: GapDirection;
  display: string;
  size?: 'sm' | 'md' | 'lg';
  showValue?: boolean;
}

/**
 * GapIndicator displays a directional gap with styling
 * @param gap - The numeric gap value (used only if display is not provided)
 * @param direction - Direction of the gap: 'up', 'down', or 'in_range'
 * @param display - Pre-formatted display string from server (e.g., "↑10.0", "✓ Met")
 * @param size - Size variant: 'sm', 'md', or 'lg'
 * @param showValue - Whether to show the value/display text (default true)
 */
export default function GapIndicator({
  gap,
  direction,
  display,
  size = 'md',
  showValue = true,
}: GapIndicatorProps) {
  // Size classes
  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };

  const iconSizes = {
    sm: 'w-3 h-3',
    md: 'w-4 h-4',
    lg: 'w-5 h-5',
  };

  // Direction-based styling
  const getDirectionStyles = () => {
    switch (direction) {
      case 'up':
        return {
          bg: 'bg-red-500/20',
          text: 'text-red-400',
          border: 'border-red-500/30',
          icon: <ArrowUp className={iconSizes[size]} />,
        };
      case 'down':
        return {
          bg: 'bg-blue-500/20',
          text: 'text-blue-400',
          border: 'border-blue-500/30',
          icon: <ArrowDown className={iconSizes[size]} />,
        };
      case 'in_range':
        return {
          bg: 'bg-green-500/20',
          text: 'text-green-400',
          border: 'border-green-500/30',
          icon: <Check className={iconSizes[size]} />,
        };
    }
  };

  const styles = getDirectionStyles();

  // Determine the display value - prefer server-provided display string
  const displayValue = display || (direction === 'in_range' ? 'Met' : gap.toFixed(1));

  return (
    <span
      className={`inline-flex items-center gap-1 rounded border font-medium ${sizeClasses[size]} ${styles.bg} ${styles.text} ${styles.border}`}
    >
      {styles.icon}
      {showValue && <span>{displayValue}</span>}
    </span>
  );
}

// Compact version for use in tables/lists
export function GapIndicatorCompact({
  direction,
  display,
}: {
  direction: GapDirection;
  display: string;
}) {
  const getColor = () => {
    switch (direction) {
      case 'up':
        return 'text-red-400';
      case 'down':
        return 'text-blue-400';
      case 'in_range':
        return 'text-green-400';
    }
  };

  return (
    <span className={`font-mono font-medium ${getColor()}`}>
      {display}
    </span>
  );
}
