// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// BlockingReasons Component - List of blocking reasons with explanations

import React, { useState } from 'react';
import {
  BlockingSummary,
  BlockingReason,
  BLOCKING_CATEGORY_DISPLAY,
  BLOCKING_CODE_DESCRIPTIONS,
} from '../../types/position-card';
import {
  AlertTriangle,
  XCircle,
  AlertCircle,
  Info,
  ChevronDown,
  ChevronRight,
  Shield,
  CheckCircle,
} from 'lucide-react';

interface BlockingReasonsProps {
  /** Blocking summary data */
  blocking: BlockingSummary;
  /** Whether to show expanded view by default */
  defaultExpanded?: boolean;
  /** Maximum number of reasons to show before collapsing */
  maxVisible?: number;
  /** Additional CSS classes */
  className?: string;
}

/**
 * BlockingReasons displays all blocking conditions preventing trade entry.
 * Shows severity badges, explanations, and override capability.
 */
export default function BlockingReasons({
  blocking,
  defaultExpanded = false,
  maxVisible = 3,
  className = '',
}: BlockingReasonsProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  // No blocking reasons - show success state
  if (blocking.totalReasons === 0) {
    return (
      <div className={`flex items-center gap-2 text-green-400 ${className}`}>
        <CheckCircle className="w-4 h-4" />
        <span className="text-sm">No blocking conditions - Ready to trade</span>
      </div>
    );
  }

  const visibleReasons = expanded
    ? blocking.allReasons
    : blocking.allReasons.slice(0, maxVisible);
  const hiddenCount = blocking.allReasons.length - maxVisible;

  return (
    <div className={`space-y-2 ${className}`}>
      {/* Summary header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <BlockingSummaryBadge blocking={blocking} />
          {blocking.canOverride && blocking.softBlockCount > 0 && (
            <span className="text-xs text-yellow-400/70 flex items-center gap-1">
              <Shield className="w-3 h-3" />
              Can override
            </span>
          )}
        </div>
        {blocking.totalReasons > maxVisible && (
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="text-xs text-gray-400 hover:text-gray-300 flex items-center gap-1"
            aria-expanded={expanded}
            aria-label={expanded ? 'Show fewer blocking reasons' : `Show all ${blocking.totalReasons} blocking reasons`}
          >
            {expanded ? (
              <>
                <ChevronDown className="w-3 h-3" />
                Show less
              </>
            ) : (
              <>
                <ChevronRight className="w-3 h-3" />
                Show all ({blocking.totalReasons})
              </>
            )}
          </button>
        )}
      </div>

      {/* Reason list */}
      <div className="space-y-1.5">
        {visibleReasons.map((reason, index) => (
          <BlockingReasonItem key={`${reason.code}-${index}`} reason={reason} />
        ))}
        {!expanded && hiddenCount > 0 && (
          <button
            onClick={() => setExpanded(true)}
            className="w-full py-1.5 text-xs text-gray-400 hover:text-gray-300 bg-gray-800/50 rounded border border-gray-700/50 hover:border-gray-600/50 transition-colors"
          >
            +{hiddenCount} more blocking reason{hiddenCount > 1 ? 's' : ''}
          </button>
        )}
      </div>
    </div>
  );
}

/**
 * BlockingSummaryBadge - Compact summary of blocking state.
 */
interface BlockingSummaryBadgeProps {
  blocking: BlockingSummary;
}

function BlockingSummaryBadge({ blocking }: BlockingSummaryBadgeProps) {
  if (blocking.hardBlockCount > 0) {
    return (
      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium bg-red-500/20 text-red-400">
        <XCircle className="w-3.5 h-3.5" />
        {blocking.hardBlockCount} Hard Block{blocking.hardBlockCount > 1 ? 's' : ''}
      </span>
    );
  }

  if (blocking.softBlockCount > 0) {
    return (
      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium bg-yellow-500/20 text-yellow-400">
        <AlertTriangle className="w-3.5 h-3.5" />
        {blocking.softBlockCount} Soft Block{blocking.softBlockCount > 1 ? 's' : ''}
      </span>
    );
  }

  if (blocking.warningCount > 0) {
    return (
      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-xs font-medium bg-blue-500/20 text-blue-400">
        <AlertCircle className="w-3.5 h-3.5" />
        {blocking.warningCount} Warning{blocking.warningCount > 1 ? 's' : ''}
      </span>
    );
  }

  return null;
}

/**
 * BlockingReasonItem - Single blocking reason with details.
 */
interface BlockingReasonItemProps {
  reason: BlockingReason;
}

function BlockingReasonItem({ reason }: BlockingReasonItemProps) {
  const categoryDisplay = BLOCKING_CATEGORY_DISPLAY[reason.category];
  const description = BLOCKING_CODE_DESCRIPTIONS[reason.code] || reason.description;

  const Icon = reason.category === 'HARD_BLOCK' ? XCircle
    : reason.category === 'SOFT_BLOCK' ? AlertTriangle
    : Info;

  return (
    <div
      className={`
        flex items-start gap-2 p-2 rounded-lg border
        ${reason.category === 'HARD_BLOCK' ? 'bg-red-500/10 border-red-500/20' :
          reason.category === 'SOFT_BLOCK' ? 'bg-yellow-500/10 border-yellow-500/20' :
          'bg-blue-500/10 border-blue-500/20'}
      `}
    >
      <Icon className={`w-4 h-4 mt-0.5 flex-shrink-0 ${categoryDisplay.color}`} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className={`text-sm font-medium ${categoryDisplay.color}`}>
            {formatReasonCode(reason.code)}
          </span>
          {reason.overridable && (
            <span className="text-xs text-gray-500">(overridable)</span>
          )}
        </div>
        <p className="text-xs text-gray-400 mt-0.5">{description}</p>
        {(reason.value !== undefined || reason.threshold !== undefined) && (
          <div className="flex items-center gap-3 mt-1 text-xs">
            {reason.value !== undefined && (
              <span className="text-gray-300">
                Value: <span className="font-mono">{formatValue(reason.value)}</span>
              </span>
            )}
            {reason.threshold !== undefined && (
              <span className="text-gray-500">
                Threshold: <span className="font-mono">{formatValue(reason.threshold)}</span>
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * BlockingReasonsCompact - Minimal view for card headers.
 */
interface BlockingReasonsCompactProps {
  blocking: BlockingSummary;
  className?: string;
}

export function BlockingReasonsCompact({ blocking, className = '' }: BlockingReasonsCompactProps) {
  if (blocking.totalReasons === 0) {
    return (
      <span className={`text-xs text-green-400 flex items-center gap-1 ${className}`}>
        <CheckCircle className="w-3 h-3" />
        Clear
      </span>
    );
  }

  return (
    <div className={`flex items-center gap-1 ${className}`}>
      {blocking.hardBlockCount > 0 && (
        <span className="inline-flex items-center gap-0.5 text-xs text-red-400">
          <XCircle className="w-3 h-3" />
          {blocking.hardBlockCount}
        </span>
      )}
      {blocking.softBlockCount > 0 && (
        <span className="inline-flex items-center gap-0.5 text-xs text-yellow-400">
          <AlertTriangle className="w-3 h-3" />
          {blocking.softBlockCount}
        </span>
      )}
      {blocking.warningCount > 0 && (
        <span className="inline-flex items-center gap-0.5 text-xs text-blue-400">
          <AlertCircle className="w-3 h-3" />
          {blocking.warningCount}
        </span>
      )}
    </div>
  );
}

// Helper functions

function formatReasonCode(code: string): string {
  return code
    .split('_')
    .map(word => word.charAt(0) + word.slice(1).toLowerCase())
    .join(' ');
}

function formatValue(value: number | string | unknown): string {
  if (typeof value === 'number') {
    return value.toFixed(2);
  }
  return String(value);
}
