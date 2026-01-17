// Story 7.13: Modification Node Component
// Individual modification entry in the tree view

import React, { useState } from 'react';
import { format, formatDistanceToNow } from 'date-fns';
import {
  ChevronDown,
  ChevronRight,
  Bot,
  User,
  TrendingUp,
  Info,
  Clock,
} from 'lucide-react';
import {
  ModificationNodeProps,
  ModificationSource,
  ModificationEvent,
  ModifiableOrderType,
  getSourceColor,
  getSourceIcon,
  getSourceLabel,
  getImpactColor,
  getImpactBgColor,
  formatDollarImpact,
  formatPriceDelta,
  formatPercentChange,
} from './types';
import ImpactBadge, { PriceDeltaBadge } from './ImpactBadge';

export default function ModificationNode({
  event,
  isFirst,
  isLast,
  previousEvent,
  positionSide,
  orderType,
  onExpandReasoning,
  isReasoningExpanded = false,
}: ModificationNodeProps) {
  const [showContext, setShowContext] = useState(false);

  // Get source styling
  const sourceStyle = getSourceColor(event.modificationSource);
  const sourceIcon = getSourceIcon(event.modificationSource);
  const sourceLabel = getSourceLabel(event.modificationSource);

  // Format price with appropriate precision
  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toFixed(2);
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(6);
  };

  // Format timestamp
  const formatTime = (isoDate: string) => {
    const date = new Date(isoDate);
    return format(date, 'HH:mm:ss');
  };

  // Get icon for modification source
  const getSourceIconComponent = (source: ModificationSource) => {
    switch (source) {
      case 'LLM_AUTO':
        return <Bot className="w-3.5 h-3.5 text-purple-400" />;
      case 'USER_MANUAL':
        return <User className="w-3.5 h-3.5 text-blue-400" />;
      case 'TRAILING_STOP':
        return <TrendingUp className="w-3.5 h-3.5 text-yellow-400" />;
      default:
        return null;
    }
  };

  // Is this the initial placement?
  const isInitial = event.eventType === 'PLACED' || event.impactDirection === 'INITIAL';

  return (
    <div className="relative pl-6">
      {/* Tree connector line */}
      {!isLast && (
        <div className="absolute left-2.5 top-6 bottom-0 w-0.5 bg-gray-700" />
      )}

      {/* Node indicator */}
      <div
        className={`absolute left-0 top-1.5 w-5 h-5 rounded-full flex items-center justify-center text-xs font-bold
          ${isInitial
            ? 'bg-gray-600 text-gray-300'
            : getImpactBgColor(event.impactDirection, orderType) + ' ' + getImpactColor(event.impactDirection, orderType)
          }
        `}
      >
        {isInitial ? '1' : `v${event.version}`}
      </div>

      {/* Main content */}
      <div className="pb-3">
        {/* Header row */}
        <div className="flex items-center justify-between gap-2 mb-1">
          <div className="flex items-center gap-2">
            {/* Version badge */}
            <span
              className={`text-sm font-medium ${
                isInitial ? 'text-gray-400' : getImpactColor(event.impactDirection, orderType)
              }`}
            >
              v{event.version}
            </span>

            {/* Source badge */}
            <span
              className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-xs ${sourceStyle.bg} ${sourceStyle.color}`}
            >
              {getSourceIconComponent(event.modificationSource)}
              <span>{sourceLabel}</span>
            </span>

            {/* Price display */}
            <span className="font-mono text-gray-200 font-medium">
              ${formatPrice(event.newPrice)}
            </span>

            {/* Price delta */}
            {!isInitial && event.priceDelta !== null && (
              <PriceDeltaBadge
                delta={event.priceDelta}
                percent={event.priceDeltaPercent}
                direction={event.impactDirection}
                orderType={orderType}
              />
            )}
          </div>

          {/* Right side: Impact + Time */}
          <div className="flex items-center gap-3">
            {/* Dollar impact */}
            {!isInitial && (
              <ImpactBadge
                amount={event.dollarImpact}
                direction={event.impactDirection}
                orderType={orderType}
                size="sm"
                showTrend={true}
              />
            )}

            {/* Timestamp */}
            <span className="text-xs text-gray-500 flex items-center gap-1">
              <Clock className="w-3 h-3" />
              {formatTime(event.createdAt)}
            </span>
          </div>
        </div>

        {/* Modification reason */}
        {event.modificationReason && (
          <div className="mt-1.5">
            <button
              type="button"
              onClick={() => onExpandReasoning?.()}
              aria-expanded={isReasoningExpanded}
              aria-label={`${isReasoningExpanded ? 'Hide' : 'Show'} modification reasoning`}
              className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-300 transition-colors"
            >
              {isReasoningExpanded ? (
                <ChevronDown className="w-3 h-3" />
              ) : (
                <ChevronRight className="w-3 h-3" />
              )}
              <span className="text-purple-400">Reasoning</span>
            </button>

            {isReasoningExpanded && (
              <div className="mt-2 pl-4 border-l-2 border-purple-500/30">
                <p className="text-sm text-gray-300 leading-relaxed">
                  {event.modificationReason}
                </p>

                {/* LLM confidence */}
                {event.llmConfidence !== undefined && (
                  <div className="mt-2 flex items-center gap-2">
                    <span className="text-xs text-gray-500">Confidence:</span>
                    <div className="flex items-center gap-1">
                      <div className="w-16 h-1.5 bg-gray-700 rounded-full overflow-hidden">
                        <div
                          className={`h-full rounded-full ${
                            event.llmConfidence >= 80
                              ? 'bg-green-500'
                              : event.llmConfidence >= 60
                              ? 'bg-yellow-500'
                              : 'bg-red-500'
                          }`}
                          style={{ width: `${event.llmConfidence}%` }}
                        />
                      </div>
                      <span className="text-xs text-gray-400">
                        {event.llmConfidence.toFixed(0)}%
                      </span>
                    </div>
                  </div>
                )}

                {/* Market context toggle */}
                {event.marketContext && (
                  <button
                    type="button"
                    onClick={() => setShowContext(!showContext)}
                    aria-expanded={showContext}
                    aria-label={`${showContext ? 'Hide' : 'Show'} market context details`}
                    className="mt-2 flex items-center gap-1 text-xs text-gray-500 hover:text-gray-400 transition-colors"
                  >
                    <Info className="w-3 h-3" />
                    <span>{showContext ? 'Hide' : 'Show'} market context</span>
                  </button>
                )}

                {/* Market context details */}
                {showContext && event.marketContext && (
                  <div className="mt-2 grid grid-cols-2 gap-2 text-xs">
                    <div className="bg-gray-800/50 rounded p-2">
                      <span className="text-gray-500">Price at change:</span>
                      <span className="ml-1 text-gray-300 font-mono">
                        ${formatPrice(event.marketContext.currentPrice)}
                      </span>
                    </div>
                    {event.marketContext.priceChange1h !== undefined && (
                      <div className="bg-gray-800/50 rounded p-2">
                        <span className="text-gray-500">1h change:</span>
                        <span
                          className={`ml-1 font-mono ${
                            event.marketContext.priceChange1h >= 0
                              ? 'text-green-400'
                              : 'text-red-400'
                          }`}
                        >
                          {formatPercentChange(event.marketContext.priceChange1h)}
                        </span>
                      </div>
                    )}
                    {event.marketContext.volatility !== undefined && (
                      <div className="bg-gray-800/50 rounded p-2">
                        <span className="text-gray-500">Volatility:</span>
                        <span className="ml-1 text-gray-300 font-mono">
                          {event.marketContext.volatility.toFixed(2)}%
                        </span>
                      </div>
                    )}
                    {event.marketContext.trend && (
                      <div className="bg-gray-800/50 rounded p-2">
                        <span className="text-gray-500">Trend:</span>
                        <span
                          className={`ml-1 ${
                            event.marketContext.trend === 'BULLISH'
                              ? 'text-green-400'
                              : event.marketContext.trend === 'BEARISH'
                              ? 'text-red-400'
                              : 'text-gray-400'
                          }`}
                        >
                          {event.marketContext.trend}
                        </span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// Compact node for collapsed view
export function ModificationNodeCompact({
  event,
  orderType,
}: {
  event: ModificationEvent;
  orderType: ModifiableOrderType;
}) {
  const sourceIcon = getSourceIcon(event.modificationSource);
  const formatPrice = (price: number) => {
    if (price >= 1000) return price.toFixed(2);
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(6);
  };

  return (
    <div className="flex items-center gap-2 text-xs text-gray-400">
      <span>{sourceIcon}</span>
      <span className="font-mono">${formatPrice(event.newPrice)}</span>
      {event.impactDirection !== 'INITIAL' && (
        <span className={getImpactColor(event.impactDirection, orderType)}>
          {formatDollarImpact(event.dollarImpact)}
        </span>
      )}
    </div>
  );
}
