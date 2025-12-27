import React, { useState, useEffect } from 'react';
import {
  History,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  TrendingUp,
  Target,
  Shield,
  AlertTriangle,
  Activity,
  Clock,
  ArrowRight,
  CheckCircle,
  XCircle,
  BarChart3,
  Zap,
  Filter,
} from 'lucide-react';
import { futuresApi, TradeLifecycleEvent, TradeLifecycleSummary } from '../services/futuresApi';
import { formatDistanceToNow, format } from 'date-fns';

// Event type configuration with icons, colors, and labels
const EVENT_CONFIG: Record<string, { icon: React.ElementType; color: string; bgColor: string; label: string }> = {
  position_opened: {
    icon: TrendingUp,
    color: 'text-green-400',
    bgColor: 'bg-green-500/20',
    label: 'Position Opened',
  },
  sltp_placed: {
    icon: Shield,
    color: 'text-blue-400',
    bgColor: 'bg-blue-500/20',
    label: 'SL/TP Placed',
  },
  sl_revised: {
    icon: Shield,
    color: 'text-yellow-400',
    bgColor: 'bg-yellow-500/20',
    label: 'SL Revised',
  },
  moved_to_breakeven: {
    icon: Target,
    color: 'text-purple-400',
    bgColor: 'bg-purple-500/20',
    label: 'Moved to Breakeven',
  },
  tp_hit: {
    icon: CheckCircle,
    color: 'text-green-400',
    bgColor: 'bg-green-500/20',
    label: 'Take Profit Hit',
  },
  trailing_activated: {
    icon: Activity,
    color: 'text-cyan-400',
    bgColor: 'bg-cyan-500/20',
    label: 'Trailing Activated',
  },
  trailing_updated: {
    icon: ArrowRight,
    color: 'text-cyan-400',
    bgColor: 'bg-cyan-500/20',
    label: 'Trailing Updated',
  },
  position_closed: {
    icon: XCircle,
    color: 'text-gray-400',
    bgColor: 'bg-gray-500/20',
    label: 'Position Closed',
  },
  external_close: {
    icon: AlertTriangle,
    color: 'text-orange-400',
    bgColor: 'bg-orange-500/20',
    label: 'External Close',
  },
  sl_hit: {
    icon: XCircle,
    color: 'text-red-400',
    bgColor: 'bg-red-500/20',
    label: 'Stop Loss Hit',
  },
};

// Source badge configuration
const SOURCE_CONFIG: Record<string, { color: string; label: string }> = {
  ginie: { color: 'bg-purple-500/20 text-purple-400 border-purple-500/50', label: 'Ginie' },
  trailing: { color: 'bg-cyan-500/20 text-cyan-400 border-cyan-500/50', label: 'Trailing' },
  manual: { color: 'bg-blue-500/20 text-blue-400 border-blue-500/50', label: 'Manual' },
  external: { color: 'bg-orange-500/20 text-orange-400 border-orange-500/50', label: 'External' },
  system: { color: 'bg-gray-500/20 text-gray-400 border-gray-500/50', label: 'System' },
};

interface Props {
  tradeId?: number; // If provided, shows events for specific trade
  limit?: number;
  showSummary?: boolean;
  compact?: boolean;
  autoRefresh?: boolean;
  refreshInterval?: number;
}

export default function TradeLifecycleEvents({
  tradeId,
  limit = 50,
  showSummary = true,
  compact = false,
  autoRefresh = true,
  refreshInterval = 30000,
}: Props) {
  const [events, setEvents] = useState<TradeLifecycleEvent[]>([]);
  const [summary, setSummary] = useState<TradeLifecycleSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedEvents, setExpandedEvents] = useState<Set<number>>(new Set());
  const [filterType, setFilterType] = useState<string>('all');

  const toggleEvent = (id: number) => {
    setExpandedEvents((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const fetchData = async () => {
    try {
      if (tradeId) {
        // Fetch events for specific trade
        const [eventsRes, summaryRes] = await Promise.all([
          futuresApi.getTradeLifecycleEvents(tradeId),
          showSummary ? futuresApi.getTradeLifecycleSummary(tradeId).catch(() => null) : Promise.resolve(null),
        ]);
        setEvents(eventsRes.events || []);
        if (summaryRes?.summary) {
          setSummary(summaryRes.summary);
        }
      } else {
        // Fetch recent events across all trades
        const res = await futuresApi.getRecentTradeLifecycleEvents(limit);
        setEvents(res.events || []);
      }
      setError(null);
    } catch (err) {
      console.error('Failed to fetch lifecycle events:', err);
      setError(err instanceof Error ? err.message : 'Failed to fetch events');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    if (autoRefresh) {
      const interval = setInterval(fetchData, refreshInterval);
      return () => clearInterval(interval);
    }
  }, [tradeId, limit, autoRefresh, refreshInterval]);

  // Filter events by type
  const filteredEvents = filterType === 'all'
    ? events
    : events.filter(e => e.event_type === filterType);

  // Get unique event types for filter
  const eventTypes = [...new Set(events.map(e => e.event_type))];

  // Render event badge
  const renderEventBadge = (eventType: string) => {
    const config = EVENT_CONFIG[eventType] || {
      icon: Activity,
      color: 'text-gray-400',
      bgColor: 'bg-gray-500/20',
      label: eventType,
    };
    const Icon = config.icon;

    return (
      <span className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium ${config.bgColor} ${config.color}`}>
        <Icon className="w-3.5 h-3.5" />
        {config.label}
      </span>
    );
  };

  // Render source badge
  const renderSourceBadge = (source: string) => {
    const config = SOURCE_CONFIG[source] || SOURCE_CONFIG.system;
    return (
      <span className={`inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium border ${config.color}`}>
        {config.label}
      </span>
    );
  };

  // Format price with appropriate precision
  const formatPrice = (price?: number) => {
    if (price === undefined || price === null) return '-';
    if (price >= 1000) return price.toFixed(2);
    if (price >= 1) return price.toFixed(4);
    return price.toFixed(8);
  };

  // Format PnL with color
  const formatPnL = (pnl?: number, pnlPercent?: number) => {
    if (pnl === undefined || pnl === null) return null;
    const isPositive = pnl >= 0;
    return (
      <span className={isPositive ? 'text-green-400' : 'text-red-400'}>
        {isPositive ? '+' : ''}{pnl.toFixed(2)} USDT
        {pnlPercent !== undefined && (
          <span className="text-xs ml-1">
            ({isPositive ? '+' : ''}{pnlPercent.toFixed(2)}%)
          </span>
        )}
      </span>
    );
  };

  // Render event details
  const renderEventDetails = (event: TradeLifecycleEvent) => {
    const details = event.details || {};

    return (
      <div className="bg-gray-900/70 rounded-lg p-4 space-y-3 mt-2">
        {/* Reason if present */}
        {event.reason && (
          <div>
            <h4 className="text-xs font-medium text-gray-400 mb-1">Reason</h4>
            <p className="text-sm text-gray-300 bg-gray-800/50 rounded p-2">
              {event.reason}
            </p>
          </div>
        )}

        {/* Price changes */}
        {(event.old_value !== undefined || event.new_value !== undefined) && (
          <div className="flex items-center gap-4">
            {event.old_value !== undefined && (
              <div>
                <span className="text-xs text-gray-500">Previous:</span>
                <span className="ml-2 text-sm font-mono text-gray-400">{formatPrice(event.old_value)}</span>
              </div>
            )}
            {event.old_value !== undefined && event.new_value !== undefined && (
              <ArrowRight className="w-4 h-4 text-gray-500" />
            )}
            {event.new_value !== undefined && (
              <div>
                <span className="text-xs text-gray-500">New:</span>
                <span className="ml-2 text-sm font-mono text-gray-300">{formatPrice(event.new_value)}</span>
              </div>
            )}
          </div>
        )}

        {/* TP Level info */}
        {event.tp_level !== undefined && (
          <div className="flex items-center gap-2">
            <Target className="w-4 h-4 text-green-400" />
            <span className="text-sm text-gray-300">Take Profit Level {event.tp_level}</span>
          </div>
        )}

        {/* Quantity closed */}
        {event.quantity_closed !== undefined && (
          <div className="flex items-center gap-2">
            <BarChart3 className="w-4 h-4 text-gray-400" />
            <span className="text-sm text-gray-300">
              Quantity: {event.quantity_closed.toFixed(4)}
            </span>
          </div>
        )}

        {/* PnL info */}
        {event.pnl_realized !== undefined && (
          <div className="flex items-center gap-2">
            <Zap className="w-4 h-4 text-yellow-400" />
            <span className="text-sm">
              Realized PnL: {formatPnL(event.pnl_realized, event.pnl_percent)}
            </span>
          </div>
        )}

        {/* SL revision count */}
        {event.sl_revision_count !== undefined && event.sl_revision_count > 0 && (
          <div className="flex items-center gap-2">
            <Shield className="w-4 h-4 text-yellow-400" />
            <span className="text-sm text-gray-300">
              SL Revision #{event.sl_revision_count}
            </span>
          </div>
        )}

        {/* Mode */}
        {event.mode && (
          <div className="flex items-center gap-2">
            <Activity className="w-4 h-4 text-purple-400" />
            <span className="text-sm text-gray-300 capitalize">Mode: {event.mode}</span>
          </div>
        )}

        {/* Additional details */}
        {Object.keys(details).length > 0 && (
          <div className="border-t border-gray-700 pt-3 mt-3">
            <h4 className="text-xs font-medium text-gray-400 mb-2">Additional Details</h4>
            <div className="grid grid-cols-2 gap-2 text-xs">
              {Object.entries(details).map(([key, value]) => (
                <div key={key} className="flex justify-between">
                  <span className="text-gray-500 capitalize">{key.replace(/_/g, ' ')}:</span>
                  <span className="text-gray-300 font-mono">
                    {typeof value === 'number' ? formatPrice(value) : String(value)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Conditions met */}
        {event.conditions_met && Object.keys(event.conditions_met).length > 0 && (
          <div className="border-t border-gray-700 pt-3 mt-3">
            <h4 className="text-xs font-medium text-gray-400 mb-2">Conditions Met</h4>
            <div className="flex flex-wrap gap-2">
              {Object.entries(event.conditions_met).map(([key, value]) => (
                <span
                  key={key}
                  className="inline-flex items-center px-2 py-1 rounded text-xs bg-green-500/10 text-green-400 border border-green-500/30"
                >
                  {key}: {typeof value === 'number' ? value.toFixed(2) : String(value)}
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Timestamp */}
        <div className="text-xs text-gray-500 pt-2 border-t border-gray-700 flex items-center gap-1">
          <Clock className="w-3 h-3" />
          {format(new Date(event.timestamp), 'yyyy-MM-dd HH:mm:ss')}
        </div>
      </div>
    );
  };

  // Loading state
  if (loading && events.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <div className="flex items-center justify-center text-gray-400">
          <RefreshCw className="w-5 h-5 animate-spin mr-2" />
          Loading lifecycle events...
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-red-500/30">
        <div className="flex items-center gap-2 text-red-400">
          <AlertTriangle className="w-5 h-5" />
          <span>{error}</span>
        </div>
      </div>
    );
  }

  // Empty state
  if (events.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
        <div className="text-center text-gray-400">
          <History className="w-12 h-12 mx-auto mb-3 opacity-30" />
          <p>No lifecycle events found</p>
          <p className="text-sm mt-1">Events will appear as trades are executed</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700">
      {/* Header */}
      <div className="p-4 border-b border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <History className="w-5 h-5 text-purple-400" />
          <h3 className="text-lg font-semibold text-gray-200">
            {tradeId ? 'Trade Lifecycle' : 'Recent Trade Events'}
          </h3>
          <span className="text-sm text-gray-500">
            ({filteredEvents.length} event{filteredEvents.length !== 1 ? 's' : ''})
          </span>
        </div>

        <div className="flex items-center gap-3">
          {/* Filter dropdown */}
          {eventTypes.length > 1 && (
            <div className="flex items-center gap-2">
              <Filter className="w-4 h-4 text-gray-500" />
              <select
                value={filterType}
                onChange={(e) => setFilterType(e.target.value)}
                className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm text-gray-300"
              >
                <option value="all">All Events</option>
                {eventTypes.map((type) => (
                  <option key={type} value={type}>
                    {EVENT_CONFIG[type]?.label || type}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Refresh button */}
          <button
            onClick={() => { setLoading(true); fetchData(); }}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Summary section (for single trade view) */}
      {showSummary && summary && (
        <div className="p-4 bg-gray-900/50 border-b border-gray-700">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <span className="text-gray-500">Symbol:</span>
              <span className="ml-2 font-semibold text-gray-200">{summary.symbol}</span>
            </div>
            <div>
              <span className="text-gray-500">Mode:</span>
              <span className="ml-2 font-medium text-purple-400 capitalize">{summary.mode}</span>
            </div>
            <div>
              <span className="text-gray-500">SL Revisions:</span>
              <span className="ml-2 font-medium text-yellow-400">{summary.sl_revisions}</span>
            </div>
            <div>
              <span className="text-gray-500">TP Hits:</span>
              <span className="ml-2 font-medium text-green-400">{summary.tp_hits}</span>
            </div>
            {summary.final_pnl !== undefined && (
              <div className="col-span-2">
                <span className="text-gray-500">Final PnL:</span>
                <span className="ml-2">{formatPnL(summary.final_pnl, summary.final_pnl_percent)}</span>
              </div>
            )}
            {summary.close_reason && (
              <div className="col-span-2">
                <span className="text-gray-500">Close Reason:</span>
                <span className="ml-2 text-gray-300">{summary.close_reason}</span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Events timeline */}
      <div className={`divide-y divide-gray-700 ${compact ? 'max-h-96 overflow-y-auto' : ''}`}>
        {filteredEvents.map((event) => {
          const isExpanded = expandedEvents.has(event.id);
          const hasDetails = event.reason || event.old_value !== undefined ||
                            event.new_value !== undefined || event.details;

          return (
            <div key={event.id} className="p-4 hover:bg-gray-800/50 transition-colors">
              {/* Event row */}
              <div
                className={`flex items-start gap-3 ${hasDetails ? 'cursor-pointer' : ''}`}
                onClick={() => hasDetails && toggleEvent(event.id)}
              >
                {/* Expand indicator */}
                <div className="pt-0.5">
                  {hasDetails ? (
                    isExpanded ? (
                      <ChevronDown className="w-4 h-4 text-purple-400" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-gray-500" />
                    )
                  ) : (
                    <div className="w-4" />
                  )}
                </div>

                {/* Event content */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-3 flex-wrap">
                    {/* Event type badge */}
                    {renderEventBadge(event.event_type)}

                    {/* Source badge */}
                    {renderSourceBadge(event.source)}

                    {/* Trigger price if present */}
                    {event.trigger_price !== undefined && (
                      <span className="text-sm font-mono text-gray-400">
                        @ {formatPrice(event.trigger_price)}
                      </span>
                    )}

                    {/* PnL if present (inline for compact view) */}
                    {event.pnl_realized !== undefined && (
                      <span className="text-sm">
                        {formatPnL(event.pnl_realized, event.pnl_percent)}
                      </span>
                    )}
                  </div>

                  {/* Timestamp */}
                  <div className="mt-1 text-xs text-gray-500 flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {formatDistanceToNow(new Date(event.timestamp), { addSuffix: true })}
                    {event.futures_trade_id && !tradeId && (
                      <span className="ml-2 text-gray-600">
                        Trade #{event.futures_trade_id}
                      </span>
                    )}
                  </div>
                </div>
              </div>

              {/* Expanded details */}
              {isExpanded && hasDetails && renderEventDetails(event)}
            </div>
          );
        })}
      </div>
    </div>
  );
}
