// Epic 11: Entry Decision Engine - Standalone Expandable Card
// Shows all monitored coins with their decision states, scores, and blocking reasons
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
  Brain,
  Target,
} from 'lucide-react';
import { futuresApi, CoinStateResponse } from '../../services/futuresApi';

interface EntryDecisionEngineCardProps {
  /** Whether the card is initially expanded */
  defaultExpanded?: boolean;
  /** Optional CSS class */
  className?: string;
}

export default function EntryDecisionEngineCard({
  defaultExpanded = true,
  className = '',
}: EntryDecisionEngineCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [coinStates, setCoinStates] = useState<CoinStateResponse[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  // Fetch all monitored coin states from the decision engine
  const fetchCoinStates = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      // Get count first to know how many coins are monitored
      const count = await futuresApi.getCoinStateCount();

      if (count === 0) {
        setCoinStates([]);
        setLastUpdated(new Date());
        return;
      }

      // Fetch all coin states (the API returns all when no symbols specified)
      const states = await futuresApi.getAllCoinStates();
      setCoinStates(states);
      setLastUpdated(new Date());
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message :
        (err as { response?: { data?: { error?: string; message?: string } } })?.response?.data?.error ||
        (err as { response?: { data?: { error?: string; message?: string } } })?.response?.data?.message ||
        'Unable to load decision engine data';
      console.error('[EntryDecisionEngine] Failed to fetch coin states:', err);
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch on mount and when expanded
  useEffect(() => {
    if (expanded) {
      fetchCoinStates();
    }
  }, [expanded, fetchCoinStates]);

  // Auto-refresh every 30 seconds when expanded
  useEffect(() => {
    if (!expanded) return;

    const interval = setInterval(fetchCoinStates, 30000);
    return () => clearInterval(interval);
  }, [expanded, fetchCoinStates]);

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
          Ready to Enter
        </span>
      );
    }
    if (decision === 'BLOCKED') {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-red-500/20 text-red-400">
          <XCircle className="w-3 h-3" />
          {blocking.hard_block_count} Hard Block{blocking.hard_block_count !== 1 ? 's' : ''}
        </span>
      );
    }
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium bg-yellow-500/20 text-yellow-400">
        <AlertCircle className="w-3 h-3" />
        Evaluating
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

  const getScoreBarWidth = (score: number, max: number) => {
    return `${(score / max) * 100}%`;
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

  // Summary stats
  const readyCount = coinStates.filter(s => s.decision === 'READY').length;
  const blockedCount = coinStates.filter(s => s.decision === 'BLOCKED').length;
  const pendingCount = coinStates.filter(s => s.decision !== 'READY' && s.decision !== 'BLOCKED').length;

  return (
    <div className={`bg-gray-800 rounded-lg border border-gray-700 ${className}`}>
      {/* Header - Always visible */}
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center justify-between px-4 py-3 hover:bg-gray-700/30 transition-colors"
      >
        <div className="flex items-center gap-3">
          {expanded ? (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronRight className="w-5 h-5 text-gray-400" />
          )}
          <Brain className="w-5 h-5 text-purple-400" />
          <span className="font-semibold text-white">Entry Decision Engine</span>
          <span className="text-xs text-gray-400 bg-gray-700 px-2 py-0.5 rounded">
            {coinStates.length} coin{coinStates.length !== 1 ? 's' : ''} monitored
          </span>
        </div>

        <div className="flex items-center gap-3">
          {loading && <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />}
          {!loading && coinStates.length > 0 && (
            <div className="flex items-center gap-2 text-xs">
              <span className="text-green-400">{readyCount} ready</span>
              <span className="text-gray-500">|</span>
              <span className="text-red-400">{blockedCount} blocked</span>
              {pendingCount > 0 && (
                <>
                  <span className="text-gray-500">|</span>
                  <span className="text-yellow-400">{pendingCount} pending</span>
                </>
              )}
            </div>
          )}
        </div>
      </button>

      {/* Content - Expanded view */}
      {expanded && (
        <div className="border-t border-gray-700">
          {/* Error State */}
          {error && (
            <div className="px-4 py-3 bg-yellow-500/10 border-b border-yellow-500/30">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-yellow-400">
                  <AlertTriangle className="w-4 h-4" />
                  <span className="text-sm">{error}</span>
                </div>
                <button
                  onClick={fetchCoinStates}
                  className="text-yellow-300 hover:text-yellow-200 text-sm"
                >
                  Retry
                </button>
              </div>
            </div>
          )}

          {/* Loading State */}
          {loading && coinStates.length === 0 && (
            <div className="flex items-center justify-center py-8 text-gray-400">
              <RefreshCw className="w-5 h-5 animate-spin mr-2" />
              Loading decision engine data...
            </div>
          )}

          {/* Empty State */}
          {!loading && coinStates.length === 0 && !error && (
            <div className="text-center py-8">
              <Brain className="w-12 h-12 mx-auto mb-3 text-gray-600" />
              <p className="text-gray-400">No coins being monitored</p>
              <p className="text-sm text-gray-500 mt-1">
                The decision engine will show coin states when autopilot is scanning for entry opportunities.
              </p>
            </div>
          )}

          {/* Coin States Grid */}
          {coinStates.length > 0 && (
            <div className="p-4 space-y-3 max-h-[500px] overflow-y-auto">
              {coinStates.map((state) => (
                <div
                  key={state.symbol}
                  className="bg-gray-900/50 rounded-lg p-4 border border-gray-700/50"
                >
                  {/* Row 1: Symbol, Regime, Decision */}
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-3">
                      <span className="font-bold text-white text-lg">{state.symbol}</span>
                      <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${getRegimeColor(state.regime)}`}>
                        {getRegimeIcon(state.regime)}
                        {state.regime}
                      </span>
                    </div>
                    {getDecisionBadge(state.decision, state.blocking)}
                  </div>

                  {/* Row 2: Score Breakdown with Visual Bars */}
                  <div className="grid grid-cols-5 gap-3 mb-3">
                    {/* Final Score */}
                    <div className="col-span-1">
                      <div className="text-xs text-gray-500 mb-1">Final Score</div>
                      <div className={`text-2xl font-bold ${getScoreColor(state.scores.final, 100)}`}>
                        {state.scores.final}
                      </div>
                      <div className="text-xs text-gray-500">/100</div>
                    </div>

                    {/* Score Components */}
                    <div className="col-span-4 space-y-2">
                      {/* Technical Score */}
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-400 w-16">Technical</span>
                        <div className="flex-1 bg-gray-700 rounded-full h-2">
                          <div
                            className="bg-blue-500 h-2 rounded-full transition-all"
                            style={{ width: getScoreBarWidth(state.scores.technical, 40) }}
                          />
                        </div>
                        <span className="text-xs text-gray-300 w-10 text-right">{state.scores.technical}/40</span>
                      </div>

                      {/* Context Score */}
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-400 w-16">Context</span>
                        <div className="flex-1 bg-gray-700 rounded-full h-2">
                          <div
                            className="bg-green-500 h-2 rounded-full transition-all"
                            style={{ width: getScoreBarWidth(state.scores.context, 30) }}
                          />
                        </div>
                        <span className="text-xs text-gray-300 w-10 text-right">{state.scores.context}/30</span>
                      </div>

                      {/* LLM Score */}
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-400 w-16">LLM</span>
                        <div className="flex-1 bg-gray-700 rounded-full h-2">
                          <div
                            className="bg-purple-500 h-2 rounded-full transition-all"
                            style={{ width: getScoreBarWidth(state.scores.llm, 20) }}
                          />
                        </div>
                        <span className="text-xs text-gray-300 w-10 text-right">{state.scores.llm}/20</span>
                      </div>

                      {/* History Score */}
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-400 w-16">History</span>
                        <div className="flex-1 bg-gray-700 rounded-full h-2">
                          <div
                            className="bg-yellow-500 h-2 rounded-full transition-all"
                            style={{ width: getScoreBarWidth(state.scores.history, 10) }}
                          />
                        </div>
                        <span className="text-xs text-gray-300 w-10 text-right">{state.scores.history}/10</span>
                      </div>
                    </div>
                  </div>

                  {/* Row 3: Technical Indicators */}
                  <div className="flex items-center gap-6 text-sm border-t border-gray-700/50 pt-3 mb-3">
                    <div className="flex items-center gap-2">
                      <span className="text-gray-500">ADX:</span>
                      <span className={`font-medium ${state.adx >= 25 ? 'text-green-400' : state.adx >= 20 ? 'text-yellow-400' : 'text-gray-400'}`}>
                        {state.adx.toFixed(1)}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-gray-500">RSI:</span>
                      <span className={`font-medium ${state.rsi > 70 ? 'text-red-400' : state.rsi < 30 ? 'text-green-400' : 'text-gray-300'}`}>
                        {state.rsi.toFixed(1)}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-gray-500">1H Trend:</span>
                      <div className="flex items-center gap-1">
                        {getTrendIcon(state.trend_1h)}
                        <span className={`font-medium ${state.trend_1h === 'BULLISH' ? 'text-green-400' : state.trend_1h === 'BEARISH' ? 'text-red-400' : 'text-gray-400'}`}>
                          {state.trend_1h}
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-gray-500">15M Trend:</span>
                      <div className="flex items-center gap-1">
                        {getTrendIcon(state.trend_15m)}
                        <span className={`font-medium ${state.trend_15m === 'BULLISH' ? 'text-green-400' : state.trend_15m === 'BEARISH' ? 'text-red-400' : 'text-gray-400'}`}>
                          {state.trend_15m}
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Target className="w-3 h-3 text-gray-500" />
                      <span className="text-gray-400">{state.active_strategy}</span>
                    </div>
                  </div>

                  {/* Row 4: Blocking Reasons (if any) */}
                  {state.blocking.total_reasons > 0 && (
                    <div className="border-t border-gray-700/50 pt-3">
                      <div className="text-xs text-gray-500 mb-2">
                        Blocking Reasons ({state.blocking.total_reasons})
                      </div>
                      <div className="flex flex-wrap gap-2">
                        {state.blocking.all_reasons.map((reason, idx) => (
                          <span
                            key={idx}
                            className={`inline-flex items-center px-2 py-1 rounded text-xs ${
                              reason.category === 'HARD_BLOCK' ? 'bg-red-500/20 text-red-400 border border-red-500/30' :
                              reason.category === 'SOFT_BLOCK' ? 'bg-yellow-500/20 text-yellow-400 border border-yellow-500/30' :
                              'bg-blue-500/20 text-blue-400 border border-blue-500/30'
                            }`}
                            title={reason.description}
                          >
                            {reason.category === 'HARD_BLOCK' && <XCircle className="w-3 h-3 mr-1" />}
                            {reason.category === 'SOFT_BLOCK' && <AlertCircle className="w-3 h-3 mr-1" />}
                            {reason.category === 'WARNING' && <AlertTriangle className="w-3 h-3 mr-1" />}
                            {reason.code.split('_').map(w => w.charAt(0) + w.slice(1).toLowerCase()).join(' ')}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Footer with refresh */}
          <div className="px-4 py-3 border-t border-gray-700 bg-gray-900/30 flex items-center justify-between">
            <div className="text-xs text-gray-500">
              {lastUpdated && `Last updated: ${lastUpdated.toLocaleTimeString()}`}
            </div>
            <button
              onClick={fetchCoinStates}
              disabled={loading}
              className="flex items-center gap-2 px-3 py-1.5 text-sm text-gray-400 hover:text-white hover:bg-gray-700/50 rounded transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
              Refresh
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
