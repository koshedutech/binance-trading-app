import React, { useState, useEffect } from 'react';
import { History, TrendingUp, TrendingDown, Brain, RefreshCw, Filter, ChevronDown, ChevronRight, Zap, MessageSquare, BarChart3, Sparkles, Activity } from 'lucide-react';
import { apiService } from '../services/api';
import { formatDistanceToNow } from 'date-fns';

interface AIDecision {
  id: number;
  symbol: string;
  current_price: number;
  action: string;
  confidence: number;
  reasoning: string;
  ml_direction?: string;
  ml_confidence?: number;
  sentiment_direction?: string;
  sentiment_confidence?: number;
  llm_direction?: string;
  llm_confidence?: number;
  pattern_direction?: string;
  pattern_confidence?: number;
  bigcandle_direction?: string;
  bigcandle_confidence?: number;
  confluence_count: number;
  risk_level: string;
  executed: boolean;
  created_at: string;
}

interface Trade {
  id: number;
  symbol: string;
  side: string;
  entry_price: number;
  exit_price?: number;
  quantity: number;
  entry_time: string;
  exit_time?: string;
  stop_loss?: number;
  take_profit?: number;
  pnl?: number;
  pnl_percent?: number;
  strategy_name?: string;
  status: string;
  ai_decision_id?: number;
  ai_decision?: AIDecision;
  trailing_stop_enabled?: boolean;
}

// Signal confidence bar component
function SignalBar({ label, icon: Icon, direction, confidence }: {
  label: string;
  icon: React.ElementType;
  direction?: string;
  confidence?: number;
}) {
  if (!direction || confidence === undefined) return null;

  const isLong = direction.toLowerCase() === 'long' || direction.toLowerCase() === 'buy';
  const isShort = direction.toLowerCase() === 'short' || direction.toLowerCase() === 'sell';
  const confidencePercent = Math.round(confidence * 100);

  return (
    <div className="flex items-center gap-2 text-xs">
      <Icon className="w-3.5 h-3.5 text-gray-400" />
      <span className="w-20 text-gray-400">{label}</span>
      <span className={`w-12 font-medium ${isLong ? 'text-green-400' : isShort ? 'text-red-400' : 'text-gray-400'}`}>
        {direction}
      </span>
      <div className="flex-1 h-2 bg-gray-700 rounded-full overflow-hidden max-w-[100px]">
        <div
          className={`h-full rounded-full ${isLong ? 'bg-green-500' : isShort ? 'bg-red-500' : 'bg-gray-500'}`}
          style={{ width: `${confidencePercent}%` }}
        />
      </div>
      <span className="w-10 text-right text-gray-300">{confidencePercent}%</span>
    </div>
  );
}

// AI Decision detail component
function AIDecisionDetail({ decision }: { decision: AIDecision }) {
  return (
    <div className="bg-gray-900/70 rounded-lg p-4 space-y-4">
      {/* Reasoning */}
      <div>
        <h4 className="text-sm font-medium text-gray-300 mb-2 flex items-center gap-2">
          <MessageSquare className="w-4 h-4" />
          AI Reasoning
        </h4>
        <p className="text-sm text-gray-400 bg-gray-800/50 rounded p-3">
          {decision.reasoning || 'No reasoning provided'}
        </p>
      </div>

      {/* Signal Breakdown */}
      <div>
        <h4 className="text-sm font-medium text-gray-300 mb-3 flex items-center gap-2">
          <BarChart3 className="w-4 h-4" />
          Signal Breakdown
          <span className="ml-auto text-xs px-2 py-0.5 rounded bg-purple-500/20 text-purple-300">
            {decision.confluence_count} signals aligned
          </span>
        </h4>
        <div className="space-y-2">
          <SignalBar label="ML Model" icon={Zap} direction={decision.ml_direction} confidence={decision.ml_confidence} />
          <SignalBar label="LLM Analysis" icon={Brain} direction={decision.llm_direction} confidence={decision.llm_confidence} />
          <SignalBar label="Sentiment" icon={MessageSquare} direction={decision.sentiment_direction} confidence={decision.sentiment_confidence} />
          <SignalBar label="Pattern" icon={Sparkles} direction={decision.pattern_direction} confidence={decision.pattern_confidence} />
          <SignalBar label="Big Candle" icon={Activity} direction={decision.bigcandle_direction} confidence={decision.bigcandle_confidence} />
        </div>
      </div>

      {/* Meta info */}
      <div className="flex items-center gap-4 text-xs text-gray-500 pt-2 border-t border-gray-700">
        <span>Overall Confidence: <span className="text-gray-300 font-medium">{Math.round(decision.confidence * 100)}%</span></span>
        <span>Risk Level: <span className="text-gray-300 font-medium capitalize">{decision.risk_level}</span></span>
        <span>Decision ID: <span className="text-gray-300 font-mono">#{decision.id}</span></span>
      </div>
    </div>
  );
}

export default function TradeHistory() {
  const [trades, setTrades] = useState<Trade[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'autopilot' | 'strategy'>('all');
  const [limit, setLimit] = useState(20);
  const [expandedRows, setExpandedRows] = useState<Set<number>>(new Set());

  const toggleRow = (id: number) => {
    setExpandedRows(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const fetchTrades = async () => {
    setLoading(true);
    try {
      // Include AI decision data for autopilot trades
      const data = await apiService.getPositionHistory(limit, 0, true);
      setTrades(data);
    } catch (error) {
      console.error('Failed to fetch trade history:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTrades();
    // Refresh every 30 seconds (reduced from 10s to avoid rate limits)
    const interval = setInterval(fetchTrades, 30000);
    return () => clearInterval(interval);
  }, [limit]);

  const formatCurrency = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
    }).format(value);
  };

  const filteredTrades = trades.filter(trade => {
    if (filter === 'all') return true;
    if (filter === 'autopilot') return trade.strategy_name === 'autopilot';
    if (filter === 'strategy') return trade.strategy_name !== 'autopilot';
    return true;
  });

  const stats = {
    total: filteredTrades.length,
    wins: filteredTrades.filter(t => (t.pnl || 0) > 0).length,
    losses: filteredTrades.filter(t => (t.pnl || 0) < 0).length,
    totalPnl: filteredTrades.reduce((sum, t) => sum + (t.pnl || 0), 0),
    avgPnlPercent: filteredTrades.length > 0
      ? filteredTrades.reduce((sum, t) => sum + (t.pnl_percent || 0), 0) / filteredTrades.length
      : 0,
  };

  if (loading && trades.length === 0) {
    return (
      <div className="flex items-center justify-center p-8 text-gray-400">
        <RefreshCw className="w-5 h-5 animate-spin mr-2" />
        Loading trade history...
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header with filters */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div className="flex items-center gap-2">
          <History className="w-5 h-5 text-gray-400" />
          <span className="text-sm text-gray-400">
            {stats.total} trades |
            <span className="text-green-500 ml-1">{stats.wins}W</span> /
            <span className="text-red-500 ml-1">{stats.losses}L</span> |
            Total: <span className={stats.totalPnl >= 0 ? 'text-green-500' : 'text-red-500'}>
              {formatCurrency(stats.totalPnl)}
            </span>
          </span>
        </div>

        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-gray-500" />
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value as any)}
            className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
          >
            <option value="all">All Trades</option>
            <option value="autopilot">Autopilot Only</option>
            <option value="strategy">Strategies Only</option>
          </select>

          <select
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            className="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm"
          >
            <option value={10}>Last 10</option>
            <option value={20}>Last 20</option>
            <option value={50}>Last 50</option>
            <option value={100}>Last 100</option>
          </select>

          <button
            onClick={fetchTrades}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Refresh"
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Trade list */}
      {filteredTrades.length === 0 ? (
        <div className="text-center text-gray-400 py-8">
          No trades found
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="w-8 py-2 px-2"></th>
                <th className="text-left py-2 px-3 text-gray-400 font-medium">Time</th>
                <th className="text-left py-2 px-3 text-gray-400 font-medium">Symbol</th>
                <th className="text-left py-2 px-3 text-gray-400 font-medium">Side</th>
                <th className="text-right py-2 px-3 text-gray-400 font-medium">Entry</th>
                <th className="text-right py-2 px-3 text-gray-400 font-medium">Exit</th>
                <th className="text-right py-2 px-3 text-gray-400 font-medium">P&L</th>
                <th className="text-left py-2 px-3 text-gray-400 font-medium">Strategy</th>
                <th className="text-center py-2 px-3 text-gray-400 font-medium">Status</th>
              </tr>
            </thead>
            <tbody>
              {filteredTrades.map((trade) => {
                const hasAIDecision = trade.ai_decision !== undefined && trade.ai_decision !== null;
                const isExpanded = expandedRows.has(trade.id);

                return (
                  <React.Fragment key={trade.id}>
                    <tr
                      className={`border-b border-gray-800 hover:bg-gray-800/50 ${hasAIDecision ? 'cursor-pointer' : ''}`}
                      onClick={() => hasAIDecision && toggleRow(trade.id)}
                    >
                      <td className="py-2 px-2 text-center">
                        {hasAIDecision && (
                          isExpanded ? (
                            <ChevronDown className="w-4 h-4 text-purple-400" />
                          ) : (
                            <ChevronRight className="w-4 h-4 text-gray-500" />
                          )
                        )}
                      </td>
                      <td className="py-2 px-3 text-gray-300">
                        <div className="text-xs">
                          {trade.exit_time
                            ? formatDistanceToNow(new Date(trade.exit_time), { addSuffix: true })
                            : formatDistanceToNow(new Date(trade.entry_time), { addSuffix: true })
                          }
                        </div>
                      </td>
                      <td className="py-2 px-3 font-medium">{trade.symbol}</td>
                      <td className="py-2 px-3">
                        <div className="flex items-center gap-1">
                          {trade.side === 'BUY' ? (
                            <TrendingUp className="w-3 h-3 text-green-500" />
                          ) : (
                            <TrendingDown className="w-3 h-3 text-red-500" />
                          )}
                          <span className={trade.side === 'BUY' ? 'text-green-500' : 'text-red-500'}>
                            {trade.side}
                          </span>
                        </div>
                      </td>
                      <td className="py-2 px-3 text-right font-mono text-gray-300">
                        {formatCurrency(trade.entry_price)}
                      </td>
                      <td className="py-2 px-3 text-right font-mono text-gray-300">
                        {trade.exit_price ? formatCurrency(trade.exit_price) : '-'}
                      </td>
                      <td className="py-2 px-3 text-right">
                        {trade.pnl !== undefined && trade.pnl !== null ? (
                          <div className={trade.pnl >= 0 ? 'text-green-500' : 'text-red-500'}>
                            <div className="font-semibold">{formatCurrency(trade.pnl)}</div>
                            <div className="text-xs">
                              {trade.pnl_percent !== undefined
                                ? `${trade.pnl_percent >= 0 ? '+' : ''}${trade.pnl_percent.toFixed(2)}%`
                                : ''
                              }
                            </div>
                          </div>
                        ) : (
                          <span className="text-gray-500">-</span>
                        )}
                      </td>
                      <td className="py-2 px-3">
                        <div className="flex items-center gap-1">
                          {trade.strategy_name === 'autopilot' && (
                            <Brain className="w-3 h-3 text-purple-500" />
                          )}
                          <span className={`text-xs ${trade.strategy_name === 'autopilot' ? 'text-purple-400' : 'text-gray-400'}`}>
                            {trade.strategy_name === 'autopilot'
                              ? 'Autopilot'
                              : (trade.strategy_name?.substring(0, 20) || 'Manual')
                            }
                          </span>
                        </div>
                      </td>
                      <td className="py-2 px-3 text-center">
                        <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                          trade.status === 'OPEN'
                            ? 'bg-blue-500/20 text-blue-400'
                            : trade.status === 'CLOSED'
                            ? 'bg-gray-500/20 text-gray-400'
                            : 'bg-yellow-500/20 text-yellow-400'
                        }`}>
                          {trade.status}
                        </span>
                      </td>
                    </tr>
                    {/* Expanded AI Decision Details Row */}
                    {hasAIDecision && isExpanded && (
                      <tr className="bg-gray-800/30">
                        <td colSpan={9} className="py-3 px-4">
                          <AIDecisionDetail decision={trade.ai_decision!} />
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
