// Story 7.21: Inline Modification Row Component
// Compact, single-line display for modification history entries
// Format: [Source] HH:MM:SS | $OLD -> $NEW | arrow $delta | reason | Conf: XX%

import React from 'react';
import { Bot, User, TrendingUp as TrailingIcon, ArrowUp, ArrowDown, Minus, Clock } from 'lucide-react';
import {
  ModificationEvent,
  ModifiableOrderType,
  ModificationSource,
  ImpactDirection,
  getImpactColor,
  formatDollarImpact,
} from './types';

interface ModificationRowProps {
  event: ModificationEvent;
  orderType: ModifiableOrderType;
  isLast: boolean;
  formatTime?: (isoDate: string) => string;
}

// Get source badge configuration
function getSourceConfig(source: ModificationSource): { icon: React.ReactNode; label: string; color: string; bg: string } {
  switch (source) {
    case 'LLM_AUTO':
      return {
        icon: <Bot className="w-3 h-3" />,
        label: 'AI',
        color: 'text-purple-400',
        bg: 'bg-purple-500/20',
      };
    case 'USER_MANUAL':
      return {
        icon: <User className="w-3 h-3" />,
        label: 'Manual',
        color: 'text-blue-400',
        bg: 'bg-blue-500/20',
      };
    case 'TRAILING_STOP':
      return {
        icon: <TrailingIcon className="w-3 h-3" />,
        label: 'Trailing',
        color: 'text-yellow-400',
        bg: 'bg-yellow-500/20',
      };
    default:
      return {
        icon: null,
        label: 'Unknown',
        color: 'text-gray-400',
        bg: 'bg-gray-500/20',
      };
  }
}

// Get direction arrow and label
function getDirectionDisplay(direction: ImpactDirection, orderType: ModifiableOrderType): {
  icon: React.ReactNode;
  label: string;
  color: string;
} {
  if (direction === 'INITIAL') {
    return {
      icon: <Minus className="w-3 h-3 text-gray-500" />,
      label: '',
      color: 'text-gray-500',
    };
  }

  // For SL orders
  if (orderType === 'SL') {
    if (direction === 'TIGHTER') {
      return {
        icon: <ArrowUp className="w-3 h-3 text-green-400" />,
        label: 'TIGHTER',
        color: 'text-green-400',
      };
    }
    if (direction === 'WIDER') {
      return {
        icon: <ArrowDown className="w-3 h-3 text-red-400" />,
        label: 'WIDER',
        color: 'text-red-400',
      };
    }
  }

  // For TP orders
  if (direction === 'BETTER') {
    return {
      icon: <ArrowUp className="w-3 h-3 text-green-400" />,
      label: 'BETTER',
      color: 'text-green-400',
    };
  }
  if (direction === 'WORSE') {
    return {
      icon: <ArrowDown className="w-3 h-3 text-red-400" />,
      label: 'WORSE',
      color: 'text-red-400',
    };
  }

  return {
    icon: <Minus className="w-3 h-3 text-gray-500" />,
    label: '',
    color: 'text-gray-500',
  };
}

// Format price with appropriate precision
function formatPrice(price: number): string {
  if (price >= 1000) return price.toFixed(2);
  if (price >= 1) return price.toFixed(4);
  return price.toFixed(6);
}

// Default time formatter
const defaultFormatTime = (isoDate: string): string => {
  try {
    const date = new Date(isoDate);
    return new Intl.DateTimeFormat('en-GB', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    }).format(date);
  } catch {
    return '--:--:--';
  }
};

export default function ModificationRow({
  event,
  orderType,
  isLast,
  formatTime = defaultFormatTime,
}: ModificationRowProps) {
  const sourceConfig = getSourceConfig(event.modificationSource);
  const directionDisplay = getDirectionDisplay(event.impactDirection, orderType);
  const impactColor = getImpactColor(event.impactDirection, orderType);
  const isInitial = event.impactDirection === 'INITIAL' || event.eventType === 'PLACED';

  // Truncate reason if too long
  const truncateReason = (reason: string, maxLen: number = 35): string => {
    if (!reason) return '';
    if (reason.length <= maxLen) return reason;
    return reason.substring(0, maxLen - 3) + '...';
  };

  return (
    <div className="flex items-center text-xs">
      {/* Tree connector */}
      <span className="text-gray-600 font-mono mr-2 select-none">
        {isLast ? '└─' : '├─'}
      </span>

      {/* Source badge */}
      <span
        className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded ${sourceConfig.bg} ${sourceConfig.color} min-w-[60px]`}
        title={`Source: ${sourceConfig.label}`}
      >
        {sourceConfig.icon}
        <span className="font-medium">{sourceConfig.label}</span>
      </span>

      {/* Separator */}
      <span className="mx-2 text-gray-600">|</span>

      {/* Timestamp */}
      <span className="text-gray-400 flex items-center gap-1 min-w-[65px]">
        <Clock className="w-3 h-3" />
        {formatTime(event.createdAt)}
      </span>

      {/* Separator */}
      <span className="mx-2 text-gray-600">|</span>

      {/* Price change */}
      <span className="font-mono min-w-[160px]">
        {isInitial ? (
          <span className="text-gray-300">${formatPrice(event.newPrice)}</span>
        ) : (
          <>
            <span className="text-gray-500">${formatPrice(event.oldPrice || 0)}</span>
            <span className="text-gray-600 mx-1">→</span>
            <span className="text-gray-200">${formatPrice(event.newPrice)}</span>
          </>
        )}
      </span>

      {/* Separator */}
      <span className="mx-2 text-gray-600">|</span>

      {/* Direction arrow + dollar impact */}
      {isInitial ? (
        <span className="text-gray-500 min-w-[85px]">— Initial</span>
      ) : (
        <span className={`flex items-center gap-1 min-w-[85px] ${impactColor}`}>
          {directionDisplay.icon}
          <span className="font-mono font-medium">{formatDollarImpact(event.dollarImpact)}</span>
        </span>
      )}

      {/* Direction label */}
      {!isInitial && directionDisplay.label && (
        <>
          <span className="mx-2 text-gray-600">|</span>
          <span className={`min-w-[50px] font-medium ${directionDisplay.color}`}>
            {directionDisplay.label}
          </span>
        </>
      )}

      {/* Reason (if present) */}
      {event.modificationReason && (
        <>
          <span className="mx-2 text-gray-600">|</span>
          <span
            className="text-gray-400 italic truncate max-w-[200px]"
            title={event.modificationReason}
          >
            "{truncateReason(event.modificationReason)}"
          </span>
        </>
      )}

      {/* LLM Confidence (if present) */}
      {event.llmConfidence !== undefined && event.modificationSource === 'LLM_AUTO' && (
        <>
          <span className="mx-2 text-gray-600">|</span>
          <span
            className={`font-medium ${
              event.llmConfidence >= 80
                ? 'text-green-400'
                : event.llmConfidence >= 60
                ? 'text-yellow-400'
                : 'text-red-400'
            }`}
          >
            Conf: {event.llmConfidence.toFixed(0)}%
          </span>
        </>
      )}
    </div>
  );
}

// Compact list view for modification history
interface ModificationRowListProps {
  events: ModificationEvent[];
  orderType: ModifiableOrderType;
  formatTime?: (isoDate: string) => string;
}

export function ModificationRowList({
  events,
  orderType,
  formatTime,
}: ModificationRowListProps) {
  // Sort events by version descending (newest first)
  const sortedEvents = [...events].sort((a, b) => b.version - a.version);

  if (sortedEvents.length === 0) {
    return (
      <div className="text-xs text-gray-500 py-2">
        No modification history available.
      </div>
    );
  }

  return (
    <div className="space-y-1 py-2">
      {sortedEvents.map((event, idx) => (
        <ModificationRow
          key={event.id}
          event={event}
          orderType={orderType}
          isLast={idx === sortedEvents.length - 1}
          formatTime={formatTime}
        />
      ))}
    </div>
  );
}
