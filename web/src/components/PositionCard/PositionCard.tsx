// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// PositionCard Component - Unified UI for position management

import React, { useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  ChevronDown,
  ChevronRight,
  Clock,
  TrendingUp,
  TrendingDown,
  Zap,
  Activity,
  RefreshCw,
} from 'lucide-react';
import {
  PositionCardProps,
  formatPrice,
  DECISION_DISPLAY,
} from '../../types/position-card';
import RegimeBadge from './RegimeBadge';
import ScoreBreakdown, { FinalScoreBadge, ScoreRing } from './ScoreBreakdown';
import BlockingReasons, { BlockingReasonsCompact } from './BlockingReasons';
import IndicatorDisplay, { IndicatorsInline } from './IndicatorDisplay';

/**
 * PositionCard displays complete coin state with collapsible details.
 * Features:
 * - Collapsible cards per coin
 * - Show current state (regime, score, indicators)
 * - Display blocking reasons with explanations
 * - Manual + Auto trading through same interface
 * - Real-time updates via WebSocket
 */
export default function PositionCard({
  state,
  defaultExpanded = false,
  showActions = true,
  onEnterLong,
  onEnterShort,
  onClosePosition,
  hasActivePosition = false,
  positionSide,
  compact = false,
}: PositionCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  // Decision state styling
  const decisionDisplay = DECISION_DISPLAY[state.decision];

  // Last updated formatting
  const lastUpdatedText = state.lastUpdated > 0
    ? formatDistanceToNow(state.lastUpdated, { addSuffix: true })
    : 'Unknown';

  // Position indicator for active positions
  const PositionIcon = positionSide === 'LONG' ? TrendingUp : positionSide === 'SHORT' ? TrendingDown : null;
  const positionColor = positionSide === 'LONG' ? 'text-green-400' : positionSide === 'SHORT' ? 'text-red-400' : '';

  if (compact) {
    return (
      <CompactPositionCard
        state={state}
        onExpand={() => setExpanded(true)}
        hasActivePosition={hasActivePosition}
        positionSide={positionSide}
      />
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden transition-all duration-200 hover:border-gray-600">
      {/* Header - Always visible */}
      <button
        type="button"
        className="w-full p-4 flex items-center justify-between cursor-pointer hover:bg-gray-800/80 transition-colors text-left"
        onClick={() => setExpanded(!expanded)}
        aria-expanded={expanded}
        aria-controls={`position-card-details-${state.symbol}`}
      >
        <div className="flex items-center gap-3">
          {/* Expand/collapse chevron */}
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-purple-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-500" />
          )}

          {/* Symbol and active position indicator */}
          <div className="flex items-center gap-2">
            <span className="text-lg font-semibold text-gray-200">{state.symbol}</span>
            {hasActivePosition && PositionIcon && (
              <span className={`flex items-center gap-1 text-xs ${positionColor}`}>
                <PositionIcon className="w-3.5 h-3.5" />
                {positionSide}
              </span>
            )}
          </div>

          {/* Regime badge */}
          <RegimeBadge regime={state.regime} />

          {/* Decision state badge */}
          <span className={`px-2 py-0.5 rounded text-xs font-medium ${decisionDisplay.bg} ${decisionDisplay.color}`}>
            {decisionDisplay.name}
          </span>
        </div>

        <div className="flex items-center gap-4">
          {/* Score badge */}
          <FinalScoreBadge score={state.scores.final} />

          {/* Blocking status compact */}
          <BlockingReasonsCompact blocking={state.blocking} />

          {/* Price */}
          <div className="text-right">
            <div className="text-sm font-mono text-gray-200">${formatPrice(state.price)}</div>
          </div>

          {/* Last updated */}
          <div className="flex items-center gap-1 text-gray-500 text-xs">
            <Clock className="w-3 h-3" />
            <span>{lastUpdatedText}</span>
          </div>
        </div>
      </button>

      {/* Expanded content */}
      {expanded && (
        <div id={`position-card-details-${state.symbol}`} className="border-t border-gray-700 p-4 space-y-4">
          {/* Top row: Score ring + Active strategy */}
          <div className="flex items-start justify-between">
            <div className="flex items-center gap-4">
              <ScoreRing score={state.scores.final} />
              <div>
                <div className="text-sm text-gray-400">Active Strategy</div>
                <div className="text-lg font-medium text-gray-200">
                  {state.activeStrategy || 'None Selected'}
                </div>
              </div>
            </div>
            {/* Quick actions */}
            {showActions && (
              <div className="flex items-center gap-2">
                <button
                  className="p-2 text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors"
                  title="Refresh data"
                >
                  <RefreshCw className="w-4 h-4" />
                </button>
              </div>
            )}
          </div>

          {/* Score breakdown */}
          <div>
            <h4 className="text-sm font-medium text-gray-400 mb-2 flex items-center gap-2">
              <Activity className="w-4 h-4 text-purple-400" />
              Score Breakdown
            </h4>
            <ScoreBreakdown scores={state.scores} />
          </div>

          {/* Indicators */}
          <div>
            <h4 className="text-sm font-medium text-gray-400 mb-2 flex items-center gap-2">
              <Zap className="w-4 h-4 text-yellow-400" />
              Indicators
            </h4>
            <IndicatorDisplay
              indicators={state.indicators}
              trend1h={state.trend1h}
              trend15m={state.trend15m}
            />
          </div>

          {/* Blocking reasons */}
          <div>
            <h4 className="text-sm font-medium text-gray-400 mb-2">Blocking Reasons</h4>
            <BlockingReasons blocking={state.blocking} />
          </div>

          {/* Trading actions */}
          {showActions && (
            <div className="pt-3 border-t border-gray-700">
              <div className="flex flex-wrap gap-2">
                {!hasActivePosition ? (
                  <>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onEnterLong?.(state.symbol);
                      }}
                      disabled={state.decision === 'BLOCKED' && state.blocking.hardBlockCount > 0}
                      className={`
                        flex items-center gap-2 px-4 py-2 rounded-lg font-medium text-sm
                        transition-colors
                        ${state.decision === 'BLOCKED' && state.blocking.hardBlockCount > 0
                          ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
                          : 'bg-green-600 hover:bg-green-500 text-white'}
                      `}
                    >
                      <TrendingUp className="w-4 h-4" />
                      Enter Long
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onEnterShort?.(state.symbol);
                      }}
                      disabled={state.decision === 'BLOCKED' && state.blocking.hardBlockCount > 0}
                      className={`
                        flex items-center gap-2 px-4 py-2 rounded-lg font-medium text-sm
                        transition-colors
                        ${state.decision === 'BLOCKED' && state.blocking.hardBlockCount > 0
                          ? 'bg-gray-700 text-gray-500 cursor-not-allowed'
                          : 'bg-red-600 hover:bg-red-500 text-white'}
                      `}
                    >
                      <TrendingDown className="w-4 h-4" />
                      Enter Short
                    </button>
                  </>
                ) : (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onClosePosition?.(state.symbol);
                    }}
                    className="flex items-center gap-2 px-4 py-2 rounded-lg font-medium text-sm bg-yellow-600 hover:bg-yellow-500 text-white transition-colors"
                  >
                    Close Position
                  </button>
                )}

                {/* Override option for soft blocks */}
                {state.decision === 'BLOCKED' && state.blocking.canOverride && state.blocking.softBlockCount > 0 && !hasActivePosition && (
                  <span className="text-xs text-yellow-400 flex items-center ml-2">
                    * Soft blocks can be overridden
                  </span>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * CompactPositionCard - Minimal card view for list displays.
 */
interface CompactPositionCardProps {
  state: PositionCardProps['state'];
  onExpand: () => void;
  hasActivePosition: boolean;
  positionSide?: 'LONG' | 'SHORT';
}

function CompactPositionCard({
  state,
  onExpand,
  hasActivePosition,
  positionSide,
}: CompactPositionCardProps) {
  const decisionDisplay = DECISION_DISPLAY[state.decision];
  const PositionIcon = positionSide === 'LONG' ? TrendingUp : positionSide === 'SHORT' ? TrendingDown : null;
  const positionColor = positionSide === 'LONG' ? 'text-green-400' : positionSide === 'SHORT' ? 'text-red-400' : '';

  return (
    <button
      type="button"
      className="w-full p-3 bg-gray-800 rounded-lg border border-gray-700 hover:border-gray-600 cursor-pointer transition-all text-left"
      onClick={onExpand}
      aria-label={`Expand ${state.symbol} position card`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-gray-200">{state.symbol}</span>
          {hasActivePosition && PositionIcon && (
            <PositionIcon className={`w-3.5 h-3.5 ${positionColor}`} />
          )}
          <RegimeBadge regime={state.regime} compact />
        </div>
        <div className="flex items-center gap-3">
          <span className={`text-xs ${decisionDisplay.color}`}>{state.scores.final}/100</span>
          <BlockingReasonsCompact blocking={state.blocking} />
        </div>
      </div>
      <div className="mt-2">
        <IndicatorsInline
          indicators={state.indicators}
          trend1h={state.trend1h}
          trend15m={state.trend15m}
        />
      </div>
    </button>
  );
}

/**
 * PositionCardSkeleton - Loading placeholder for position cards.
 */
export function PositionCardSkeleton() {
  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 p-4 animate-pulse">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-4 h-4 bg-gray-700 rounded" />
          <div className="w-24 h-6 bg-gray-700 rounded" />
          <div className="w-20 h-5 bg-gray-700 rounded-full" />
        </div>
        <div className="flex items-center gap-4">
          <div className="w-16 h-5 bg-gray-700 rounded" />
          <div className="w-20 h-5 bg-gray-700 rounded" />
        </div>
      </div>
    </div>
  );
}

/**
 * PositionCardList - Container for multiple position cards.
 */
interface PositionCardListProps {
  children: React.ReactNode;
  className?: string;
}

export function PositionCardList({ children, className = '' }: PositionCardListProps) {
  return (
    <div className={`space-y-3 ${className}`}>
      {children}
    </div>
  );
}
