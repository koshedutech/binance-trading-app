import { useState, useEffect, useCallback, useRef } from 'react';
import {
  Brain,
  CheckCircle,
  XCircle,
  AlertTriangle,
  TrendingUp,
  TrendingDown,
  Clock,
  RefreshCw,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { futuresApi } from '../services/futuresApi';
import { wsService } from '../services/websocket';
import { fallbackManager } from '../services/fallbackPollingManager';
import type { WSEvent, GinieStatusPayload } from '../types';

interface Decision {
  timestamp: string;
  symbol: string;
  action: string;
  confidence: number;
  approved: boolean;
  executed: boolean;
  rejection_reason?: string;
  quantity?: number;
  leverage?: number;
  entry_price?: number;
}

export default function AITradeStatusPanel() {
  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(true);

  // Generate unique key for fallback manager registration
  const fallbackKey = useRef(`ai-trade-status-panel-${Date.now()}`).current;

  const fetchDecisions = useCallback(async () => {
    try {
      const data = await futuresApi.getRecentDecisions();
      setDecisions(data.decisions || []);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch decisions:', err);
      setError('Failed to load decisions');
    } finally {
      setLoading(false);
    }
  }, []);

  // Story 12.8: WebSocket subscription for real-time updates + centralized fallback
  useEffect(() => {
    // Handle signal updates via WebSocket
    const handleSignalUpdate = (_event: WSEvent) => {
      // Refresh the full list when any signal changes
      if (autoRefresh) {
        fetchDecisions();
      }
    };

    // Handle Ginie status updates (for decision execution feedback)
    const handleGinieStatusUpdate = (event: WSEvent) => {
      const status = event.data.status as GinieStatusPayload;
      if (status && autoRefresh) {
        fetchDecisions();
      }
    };

    // Handle WebSocket connection changes
    const handleConnect = () => {
      // Refresh data when reconnecting to ensure consistency
      if (autoRefresh) {
        fetchDecisions();
      }
    };

    const handleDisconnect = () => {
      // Fallback polling is handled by fallbackManager
    };

    // Subscribe to signal events (Story 12.8 pattern)
    wsService.subscribe('SIGNAL_UPDATE', handleSignalUpdate);
    wsService.subscribe('SIGNAL_GENERATED', handleSignalUpdate);
    wsService.subscribe('GINIE_STATUS_UPDATE', handleGinieStatusUpdate);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    // Register with fallback manager for centralized 60s polling when disconnected
    fallbackManager.registerFetchFunction(fallbackKey, fetchDecisions);

    // Initial fetch
    fetchDecisions();

    // Cleanup: remove all listeners and unregister from fallback manager
    return () => {
      wsService.unsubscribe('SIGNAL_UPDATE', handleSignalUpdate);
      wsService.unsubscribe('SIGNAL_GENERATED', handleSignalUpdate);
      wsService.unsubscribe('GINIE_STATUS_UPDATE', handleGinieStatusUpdate);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);
      fallbackManager.unregisterFetchFunction(fallbackKey);
    };
  }, [autoRefresh, fetchDecisions, fallbackKey]);

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  const getActionIcon = (action: string) => {
    if (action.includes('long')) {
      return <TrendingUp className="w-4 h-4 text-green-500" />;
    } else if (action.includes('short')) {
      return <TrendingDown className="w-4 h-4 text-red-500" />;
    }
    return <AlertTriangle className="w-4 h-4 text-yellow-500" />;
  };

  const getStatusBadge = (decision: Decision) => {
    if (decision.executed) {
      return (
        <span className="flex items-center gap-1 px-2 py-0.5 text-xs rounded-full bg-green-500/20 text-green-400">
          <CheckCircle className="w-3 h-3" />
          Executed
        </span>
      );
    } else if (decision.approved) {
      return (
        <span className="flex items-center gap-1 px-2 py-0.5 text-xs rounded-full bg-yellow-500/20 text-yellow-400">
          <Clock className="w-3 h-3" />
          Pending
        </span>
      );
    } else {
      return (
        <span className="flex items-center gap-1 px-2 py-0.5 text-xs rounded-full bg-red-500/20 text-red-400">
          <XCircle className="w-3 h-3" />
          Rejected
        </span>
      );
    }
  };

  const stats = {
    total: decisions.length,
    executed: decisions.filter(d => d.executed).length,
    rejected: decisions.filter(d => !d.approved).length,
    pending: decisions.filter(d => d.approved && !d.executed).length,
  };

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700">
      {/* Header */}
      <div
        className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-800/50 transition-colors"
        onClick={() => setExpanded(!expanded)}
      >
        <div className="flex items-center gap-3">
          <Brain className="w-5 h-5 text-purple-400" />
          <h3 className="font-semibold text-white">AI Trade Decisions</h3>
          <span className="text-xs text-gray-400">
            ({stats.total} recent)
          </span>
        </div>
        <div className="flex items-center gap-3">
          {/* Quick Stats */}
          <div className="hidden sm:flex items-center gap-2 text-xs">
            <span className="text-green-400">{stats.executed} executed</span>
            <span className="text-gray-500">|</span>
            <span className="text-red-400">{stats.rejected} rejected</span>
          </div>

          {/* Auto-refresh toggle */}
          <button
            onClick={(e) => {
              e.stopPropagation();
              setAutoRefresh(!autoRefresh);
            }}
            className={`p-1.5 rounded transition-colors ${
              autoRefresh ? 'text-green-400 hover:bg-green-500/20' : 'text-gray-500 hover:bg-gray-700'
            }`}
            title={autoRefresh ? 'Auto-refresh ON' : 'Auto-refresh OFF'}
          >
            <RefreshCw className={`w-4 h-4 ${autoRefresh && !loading ? 'animate-spin' : ''}`} />
          </button>

          {expanded ? (
            <ChevronUp className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          )}
        </div>
      </div>

      {/* Content */}
      {expanded && (
        <div className="border-t border-gray-700">
          {loading && decisions.length === 0 ? (
            <div className="p-4 text-center text-gray-400">
              <RefreshCw className="w-5 h-5 animate-spin mx-auto mb-2" />
              Loading decisions...
            </div>
          ) : error ? (
            <div className="p-4 text-center text-red-400">
              <AlertTriangle className="w-5 h-5 mx-auto mb-2" />
              {error}
            </div>
          ) : decisions.length === 0 ? (
            <div className="p-4 text-center text-gray-400">
              <Brain className="w-8 h-8 mx-auto mb-2 opacity-50" />
              <p className="text-sm">No recent decisions</p>
              <p className="text-xs mt-1">Autopilot is evaluating markets...</p>
            </div>
          ) : (
            <div className="max-h-80 overflow-y-auto">
              {decisions.map((decision, index) => (
                <div
                  key={`${decision.timestamp}-${index}`}
                  className={`p-3 border-b border-gray-800 last:border-b-0 hover:bg-gray-800/50 transition-colors ${
                    decision.executed ? 'bg-green-500/5' : !decision.approved ? 'bg-red-500/5' : ''
                  }`}
                >
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      {getActionIcon(decision.action)}
                      <span className="font-medium text-white">{decision.symbol}</span>
                      <span className="text-xs text-gray-400 capitalize">
                        {decision.action.replace('_', ' ')}
                      </span>
                    </div>
                    {getStatusBadge(decision)}
                  </div>

                  <div className="flex items-center justify-between text-xs">
                    <div className="flex items-center gap-3 text-gray-400">
                      <span>
                        Confidence: <span className={decision.confidence >= 0.5 ? 'text-green-400' : 'text-yellow-400'}>
                          {(decision.confidence * 100).toFixed(1)}%
                        </span>
                      </span>
                      {decision.leverage && (
                        <span>
                          Leverage: <span className="text-white">{decision.leverage}x</span>
                        </span>
                      )}
                      {decision.quantity && (
                        <span>
                          Qty: <span className="text-white">{decision.quantity}</span>
                        </span>
                      )}
                    </div>
                    <span className="text-gray-500 flex items-center gap-1">
                      <Clock className="w-3 h-3" />
                      {formatTime(decision.timestamp)}
                    </span>
                  </div>

                  {/* Rejection Reason */}
                  {decision.rejection_reason && (
                    <div className="mt-2 p-2 bg-red-500/10 rounded text-xs text-red-300 flex items-start gap-2">
                      <AlertTriangle className="w-3.5 h-3.5 mt-0.5 flex-shrink-0" />
                      <span>{decision.rejection_reason}</span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
