// Story 7.13: Modification Tree Component
// Main container for displaying SL/TP modification history

import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { format } from 'date-fns';
import {
  ChevronDown,
  ChevronRight,
  Shield,
  Target,
  AlertCircle,
  History,
  RefreshCw,
} from 'lucide-react';
import {
  ModificationTreeProps,
  ModificationEvent,
  ModifiableOrderType,
  calculateSummaryStats,
  ORDER_TYPE_LABELS,
  formatDollarImpact,
  formatPriceDelta,
  formatPercentChange,
  getImpactColor,
} from './types';
import ModificationNode from './ModificationNode';
import ImpactBadge from './ImpactBadge';
import { futuresApi } from '../../../services/futuresApi';

export default function ModificationTree({
  chainId,
  orderType,
  currentPrice,
  events: initialEvents,
  positionSide,
  isExpanded: controlledExpanded,
  onToggle,
  compact = false,
}: ModificationTreeProps) {
  // State management
  const [isExpanded, setIsExpanded] = useState(controlledExpanded ?? false);
  const [expandedReasoning, setExpandedReasoning] = useState<number | null>(null);
  const [events, setEvents] = useState<ModificationEvent[]>(initialEvents || []);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Sync with controlled expansion state
  useEffect(() => {
    if (controlledExpanded !== undefined) {
      setIsExpanded(controlledExpanded);
    }
  }, [controlledExpanded]);

  // Update events when props change
  useEffect(() => {
    if (initialEvents && initialEvents.length > 0) {
      setEvents(initialEvents);
    }
  }, [initialEvents]);

  // Calculate summary stats
  const summary = useMemo(() => calculateSummaryStats(events), [events]);

  // Sort events by version (newest first for display)
  const sortedEvents = useMemo(() => {
    return [...events].sort((a, b) => b.version - a.version);
  }, [events]);

  // Get order type config
  const orderConfig = ORDER_TYPE_LABELS[orderType];

  // Handle toggle
  const handleToggle = useCallback(() => {
    const newExpanded = !isExpanded;
    setIsExpanded(newExpanded);
    onToggle?.();
  }, [isExpanded, onToggle]);

  // Fetch modification history if not provided
  const fetchHistory = useCallback(async () => {
    if (events.length > 0) return; // Already have data

    setLoading(true);
    setError(null);

    try {
      const response = await futuresApi.getModificationHistory(chainId, orderType);
      if (response.success && response.events) {
        setEvents(response.events);
      }
    } catch (err) {
      console.error('Failed to fetch modification history:', err);
      setError('Failed to load modification history');
    } finally {
      setLoading(false);
    }
  }, [chainId, orderType, events.length]);

  // Fetch on expand if no events
  useEffect(() => {
    if (isExpanded && events.length === 0) {
      fetchHistory();
    }
  }, [isExpanded, events.length, fetchHistory]);

  // Toggle reasoning expansion
  const toggleReasoning = useCallback((eventId: number) => {
    setExpandedReasoning(prev => prev === eventId ? null : eventId);
  }, []);

  // No modifications - show minimal state
  if (events.length === 0 && !loading) {
    return (
      <div className="bg-gray-800/50 rounded-lg p-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {orderType === 'SL' ? (
              <Shield className="w-4 h-4 text-red-400" />
            ) : (
              <Target className="w-4 h-4 text-cyan-400" />
            )}
            <span className={`text-sm font-medium ${orderConfig.color}`}>
              {orderConfig.label}
            </span>
          </div>
          <span className="text-xs text-gray-500">No modifications</span>
        </div>
      </div>
    );
  }

  // Compact mode - just show header with summary
  if (compact) {
    return (
      <div
        role="button"
        tabIndex={0}
        aria-expanded={isExpanded}
        aria-label={`${orderConfig.label} modification history, ${summary.totalModifications} changes`}
        className="bg-gray-800/50 rounded-lg p-3 cursor-pointer hover:bg-gray-800/70 transition-colors"
        onClick={handleToggle}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleToggle(); } }}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {isExpanded ? (
              <ChevronDown className="w-4 h-4 text-gray-400" />
            ) : (
              <ChevronRight className="w-4 h-4 text-gray-400" />
            )}
            {orderType === 'SL' ? (
              <Shield className="w-4 h-4 text-red-400" />
            ) : (
              <Target className="w-4 h-4 text-cyan-400" />
            )}
            <span className={`text-sm font-medium ${orderConfig.color}`}>
              {orderConfig.label}
            </span>
            {summary.totalModifications > 0 && (
              <span className="px-1.5 py-0.5 rounded text-xs bg-purple-500/20 text-purple-400">
                {summary.totalModifications} change{summary.totalModifications !== 1 ? 's' : ''}
              </span>
            )}
          </div>
          <div className="flex items-center gap-3">
            <span className="font-mono text-gray-200">
              ${currentPrice.toFixed(2)}
            </span>
            {summary.totalModifications > 0 && (
              <ImpactBadge
                amount={summary.netDollarImpact}
                direction={summary.netDollarImpact >= 0 ? 'BETTER' : 'WORSE'}
                orderType={orderType}
                size="sm"
                showTrend={false}
              />
            )}
          </div>
        </div>

        {isExpanded && (
          <div className="mt-3 pt-3 border-t border-gray-700">
            {loading ? (
              <div className="flex items-center justify-center py-4">
                <RefreshCw className="w-4 h-4 animate-spin text-gray-400" />
                <span className="ml-2 text-sm text-gray-400">Loading...</span>
              </div>
            ) : error ? (
              <div className="flex items-center gap-2 text-red-400 text-sm">
                <AlertCircle className="w-4 h-4" />
                {error}
              </div>
            ) : (
              <div className="space-y-1">
                {sortedEvents.map((event, idx) => (
                  <ModificationNode
                    key={event.id}
                    event={event}
                    isFirst={idx === 0}
                    isLast={idx === sortedEvents.length - 1}
                    previousEvent={idx < sortedEvents.length - 1 ? sortedEvents[idx + 1] : undefined}
                    positionSide={positionSide}
                    orderType={orderType}
                    onExpandReasoning={() => toggleReasoning(event.id)}
                    isReasoningExpanded={expandedReasoning === event.id}
                  />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    );
  }

  // Full mode with header and tree
  return (
    <div className="bg-gray-800/50 rounded-lg overflow-hidden border border-gray-700">
      {/* Header */}
      <div
        role="button"
        tabIndex={0}
        aria-expanded={isExpanded}
        aria-label={`${orderConfig.label} modification history, ${summary.totalModifications} changes`}
        className="p-4 cursor-pointer hover:bg-gray-800/70 transition-colors"
        onClick={handleToggle}
        onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); handleToggle(); } }}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {isExpanded ? (
              <ChevronDown className="w-5 h-5 text-purple-400" />
            ) : (
              <ChevronRight className="w-5 h-5 text-gray-500" />
            )}

            {/* Order type icon */}
            <div className="flex items-center gap-2">
              <span className="text-xl">{orderConfig.icon}</span>
              <span className={`font-semibold ${orderConfig.color}`}>
                {orderConfig.label}
              </span>
            </div>

            {/* Modification count badge */}
            {summary.totalModifications > 0 && (
              <span className="px-2 py-0.5 rounded-full text-xs bg-purple-500/20 text-purple-400 font-medium">
                {summary.totalModifications} change{summary.totalModifications !== 1 ? 's' : ''}
              </span>
            )}
          </div>

          {/* Current price and impact */}
          <div className="flex items-center gap-4">
            <div className="text-right">
              <div className="font-mono text-lg text-gray-200">
                ${currentPrice.toFixed(2)}
              </div>
              {summary.totalModifications > 0 && (
                <div className="text-xs text-gray-500">
                  from ${summary.initialPrice.toFixed(2)}
                </div>
              )}
            </div>

            {summary.totalModifications > 0 && (
              <ImpactBadge
                amount={summary.netDollarImpact}
                direction={summary.netDollarImpact >= 0 ? 'BETTER' : 'WORSE'}
                orderType={orderType}
                size="md"
                showTrend={true}
              />
            )}
          </div>
        </div>

        {/* Quick summary when collapsed */}
        {!isExpanded && summary.totalModifications > 0 && (
          <div className="mt-2 flex items-center gap-4 text-xs text-gray-500">
            <span>
              Initial: <span className="text-gray-400 font-mono">${summary.initialPrice.toFixed(2)}</span>
            </span>
            <span>
              Current: <span className="text-gray-400 font-mono">${summary.currentPrice.toFixed(2)}</span>
            </span>
            <span className={getImpactColor(summary.netDollarImpact >= 0 ? 'BETTER' : 'WORSE', orderType)}>
              Net: {formatDollarImpact(summary.netDollarImpact)}
            </span>
          </div>
        )}
      </div>

      {/* Expanded content */}
      {isExpanded && (
        <div className="border-t border-gray-700">
          {/* Summary bar */}
          <div className="px-4 py-3 bg-gray-900/50 flex items-center justify-between text-sm">
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2">
                <History className="w-4 h-4 text-gray-500" />
                <span className="text-gray-400">
                  {summary.totalModifications} modification{summary.totalModifications !== 1 ? 's' : ''}
                </span>
              </div>
              <span className="text-gray-600">|</span>
              <span className="text-gray-400">
                Net change: <span className={getImpactColor(summary.netPriceChange >= 0 ? 'BETTER' : 'WORSE', orderType)}>
                  {formatPriceDelta(summary.netPriceChange)}
                </span>
              </span>
              <span className="text-gray-600">|</span>
              <span className="text-gray-400">
                Impact: <span className={getImpactColor(summary.netDollarImpact >= 0 ? 'BETTER' : 'WORSE', orderType)}>
                  {formatDollarImpact(summary.netDollarImpact)}
                </span>
              </span>
            </div>

            {/* Source breakdown */}
            <div className="flex items-center gap-3 text-xs">
              {summary.sources.llmAuto > 0 && (
                <span className="text-purple-400">
                  AI: {summary.sources.llmAuto}
                </span>
              )}
              {summary.sources.userManual > 0 && (
                <span className="text-blue-400">
                  Manual: {summary.sources.userManual}
                </span>
              )}
              {summary.sources.trailingStop > 0 && (
                <span className="text-yellow-400">
                  Trailing: {summary.sources.trailingStop}
                </span>
              )}
            </div>
          </div>

          {/* Tree content */}
          <div className="p-4">
            {loading ? (
              <div className="flex items-center justify-center py-8">
                <RefreshCw className="w-5 h-5 animate-spin text-gray-400" />
                <span className="ml-2 text-gray-400">Loading modification history...</span>
              </div>
            ) : error ? (
              <div className="flex items-center justify-center gap-2 py-8 text-red-400">
                <AlertCircle className="w-5 h-5" />
                <span>{error}</span>
              </div>
            ) : (
              <div className="space-y-0">
                {/* Current value indicator */}
                <div className="flex items-center gap-2 mb-4 pb-3 border-b border-gray-700">
                  <div className="w-3 h-3 rounded-full bg-green-500" />
                  <span className="text-sm text-gray-400">Current:</span>
                  <span className="font-mono text-lg text-green-400">
                    ${currentPrice.toFixed(2)}
                  </span>
                </div>

                {/* Modification nodes */}
                {sortedEvents.map((event, idx) => (
                  <ModificationNode
                    key={event.id}
                    event={event}
                    isFirst={idx === 0}
                    isLast={idx === sortedEvents.length - 1}
                    previousEvent={idx < sortedEvents.length - 1 ? sortedEvents[idx + 1] : undefined}
                    positionSide={positionSide}
                    orderType={orderType}
                    onExpandReasoning={() => toggleReasoning(event.id)}
                    isReasoningExpanded={expandedReasoning === event.id}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// Helper component for when there's no data to display
export function EmptyModificationTree({ orderType }: { orderType: ModifiableOrderType }) {
  const orderConfig = ORDER_TYPE_LABELS[orderType];

  return (
    <div className="bg-gray-800/50 rounded-lg p-4 flex items-center justify-between">
      <div className="flex items-center gap-2">
        <span className="text-lg">{orderConfig.icon}</span>
        <span className={`font-medium ${orderConfig.color}`}>
          {orderConfig.label}
        </span>
      </div>
      <span className="text-sm text-gray-500">No modifications recorded</span>
    </div>
  );
}
