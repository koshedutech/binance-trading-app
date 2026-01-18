// Epic 11: Position Decision Engine - Coin State Panel
// Expandable panel showing coin state for monitored symbols
import { useState, useEffect, useCallback } from 'react';
import {
  Activity,
  TrendingUp,
  TrendingDown,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  AlertCircle,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Zap,
  BarChart2,
} from 'lucide-react';
import { futuresApi, CoinStateResponse } from '../../services/futuresApi';

interface CoinStatePanelProps {
  /** Symbols to show (from active order chains) */
  activeSymbols: string[];
  /** Whether the panel is initially expanded */
  defaultExpanded?: boolean;
  /** Optional CSS class */
  className?: string;
}

export default function CoinStatePanel({
  activeSymbols,
  defaultExpanded = false,
  className = '',
}: CoinStatePanelProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [coinStates, setCoinStates] = useState<CoinStateResponse[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchCoinStates = useCallback(async () => {
    if (activeSymbols.length === 0) {
      setCoinStates([]);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const states = await futuresApi.getAllCoinStates(activeSymbols);
      setCoinStates(states);
    } catch (err) {
      console.log('[CoinStatePanel] Failed to fetch coin states:', err);
      setError('Unable to load coin states');
    } finally {
      setLoading(false);
    }
  }, [activeSymbols]);

  useEffect(() => {
    if (expanded && activeSymbols.length > 0) {
      fetchCoinStates();
    }
  }, [expanded, activeSymbols, fetchCoinStates]);

  const getRegimeIcon = (regime: string) => {
    switch (regime) {
      case 'TRENDING':
        return <TrendingUp className="w-4 h-4" />;
      case 'RANGING':
        return <Activity className="w-4 h-4" />;
      case 'VOLATILE':
        return <Zap className="w-4 h-4" />;
      case 'CONSOLIDATING':
        return <BarChart2 className="w-4 h-4" />;
      default:
        return <Activity className="w-4 h-4" />;
    }
  };

  const getRegimeColor = (regime: string) => {
    switch (regime) {
      case 'TRENDING':
        return 'text-green-400 bg-green-500/20';
      case 'RANGING':
        return 'text-blue-400 bg-blue-500/20';
      case 'VOLATILE':
        return 'text-orange-400 bg-orange-500/20';
      case 'CONSOLIDATING':
        return 'text-purple-400 bg-purple-500/20';
      default:
        return 'text-gray-400 bg-gray-500/20';
    }
  };

  const getDecisionBadge = (decision: string, blocking: CoinStateResponse['blocking']) => {
    if (decision === 'READY') {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-green-500/20 text-green-400">
          <CheckCircle className="w-3 h-3" />
          Ready
        </span>
      );
    }
    if (decision === 'BLOCKED') {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-red-500/20 text-red-400">
          <XCircle className="w-3 h-3" />
          {blocking.hard_block_count} Block{blocking.hard_block_count !== 1 ? 's' : ''}
        </span>
      );
    }
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-yellow-500/20 text-yellow-400">
        <AlertCircle className="w-3 h-3" />
        Pending
      </span>
    );
  };

  const getScoreColor = (score: number, max: number) => {
    const percentage = (score / max) * 100;
    if (percentage >= 75) return 'text-green-400';
    if (percentage >= 50) return 'text-yellow-400';
    if (percentage >= 25) return 'text-orange-400';
    return 'text-red-400';
  };

  const getTrendIcon = (trend: string) => {
    switch (trend) {
      case 'BULLISH':
        return <TrendingUp className="w-3 h-3 text-green-400" />;
      case 'BEARISH':
        return <TrendingDown className="w-3 h-3 text-red-400" />;
      default:
        return <Activity className="w-3 h-3 text-gray-400" />;
    }
  };

  if (activeSymbols.length === 0) {
    return null;
  }

  return (
    <div className={`bg-gray-800/50 rounded-lg border border-gray-700/50 ${className}`}>
      {/* Header */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-700/30 transition-colors rounded-t-lg"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronRight className="w-5 h-5 text-gray-400" />
          )}
          <BarChart2 className="w-5 h-5 text-blue-400" />
          <span className="font-medium text-white">Decision Engine States</span>
          <span className="text-xs text-gray-400 bg-gray-700 px-2 py-0.5 rounded">
            {activeSymbols.length} coin{activeSymbols.length !== 1 ? 's' : ''}
          </span>
        </div>

        <div className="flex items-center gap-2">
          {loading && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
          {!loading && coinStates.length > 0 && (
            <span className="text-xs text-gray-400">
              {coinStates.filter(s => s.decision === 'READY').length} ready
            </span>
          )}
        </div>
      </button>

      {/* Content */}
      {expanded && (
        <div className="px-4 pb-4 space-y-3">
          {error && (
            <div className="flex items-center gap-2 text-sm text-yellow-400 bg-yellow-500/10 px-3 py-2 rounded">
              <AlertTriangle className="w-4 h-4" />
              {error}
              <button
                onClick={fetchCoinStates}
                className="ml-auto text-yellow-300 hover:text-yellow-200"
              >
                Retry
              </button>
            </div>
          )}

          {loading && coinStates.length === 0 && (
            <div className="flex items-center justify-center py-4 text-gray-400">
              <RefreshCw className="w-5 h-5 animate-spin mr-2" />
              Loading coin states...
            </div>
          )}

          {!loading && coinStates.length === 0 && !error && (
            <div className="text-center py-4 text-gray-400 text-sm">
              No coin state data available yet.
              <br />
              <span className="text-xs text-gray-500">
                States will appear when the decision engine scans active symbols.
              </span>
            </div>
          )}

          {coinStates.length > 0 && (
            <div className="grid gap-2">
              {coinStates.map((state) => (
                <div
                  key={state.symbol}
                  className="bg-gray-900/50 rounded-lg p-3 border border-gray-700/30"
                >
                  {/* Symbol Row */}
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-white">{state.symbol}</span>
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${getRegimeColor(state.regime)}`}>
                        {getRegimeIcon(state.regime)}
                        {state.regime}
                      </span>
                    </div>
                    {getDecisionBadge(state.decision, state.blocking)}
                  </div>

                  {/* Scores Row */}
                  <div className="flex items-center gap-4 text-xs mb-2">
                    <div className="flex items-center gap-1">
                      <span className="text-gray-400">Score:</span>
                      <span className={`font-bold ${getScoreColor(state.scores.final, 100)}`}>
                        {state.scores.final}/100
                      </span>
                    </div>
                    <div className="flex items-center gap-2 text-gray-500">
                      <span>T:{state.scores.technical}/40</span>
                      <span>C:{state.scores.context}/30</span>
                      <span>L:{state.scores.llm}/20</span>
                      <span>H:{state.scores.history}/10</span>
                    </div>
                  </div>

                  {/* Indicators Row */}
                  <div className="flex items-center gap-4 text-xs text-gray-400">
                    <span>ADX: <span className="text-gray-300">{state.adx.toFixed(1)}</span></span>
                    <span>RSI: <span className="text-gray-300">{state.rsi.toFixed(1)}</span></span>
                    <div className="flex items-center gap-1">
                      <span>1H:</span>
                      {getTrendIcon(state.trend_1h)}
                    </div>
                    <div className="flex items-center gap-1">
                      <span>15M:</span>
                      {getTrendIcon(state.trend_15m)}
                    </div>
                    <span className="text-gray-500">{state.active_strategy}</span>
                  </div>

                  {/* Blocking Reasons (if any) */}
                  {state.blocking.total_reasons > 0 && (
                    <div className="mt-2 pt-2 border-t border-gray-700/30">
                      <div className="flex flex-wrap gap-1">
                        {state.blocking.all_reasons.slice(0, 3).map((reason, idx) => (
                          <span
                            key={idx}
                            className={`inline-flex items-center px-2 py-0.5 rounded text-xs ${
                              reason.category === 'HARD_BLOCK' ? 'bg-red-500/20 text-red-400' :
                              reason.category === 'SOFT_BLOCK' ? 'bg-yellow-500/20 text-yellow-400' :
                              'bg-blue-500/20 text-blue-400'
                            }`}
                            title={reason.description}
                          >
                            {reason.code.split('_').map(w => w.charAt(0) + w.slice(1).toLowerCase()).join(' ')}
                          </span>
                        ))}
                        {state.blocking.total_reasons > 3 && (
                          <span className="text-xs text-gray-500">
                            +{state.blocking.total_reasons - 3} more
                          </span>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Refresh Button */}
          {coinStates.length > 0 && (
            <button
              onClick={fetchCoinStates}
              disabled={loading}
              className="w-full py-2 text-sm text-gray-400 hover:text-white hover:bg-gray-700/30 rounded transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-4 h-4 inline mr-2 ${loading ? 'animate-spin' : ''}`} />
              Refresh States
            </button>
          )}
        </div>
      )}
    </div>
  );
}
