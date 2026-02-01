// Epic 11: Story 11.40 - Entry Decision Engine Gap Analysis UI
// Redesigned to show gap-based analysis with directional gaps, sub-component scores, and override controls
// Enhanced: Real-time WebSocket updates for COIN_STATE_UPDATE events
import { useState, useEffect, useCallback, useRef } from 'react';
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
  ArrowUp,
  ArrowDown,
} from 'lucide-react';
import { futuresApi, CoinStateWithGapsResponse, CoinStateResponse } from '../../services/futuresApi';
import { wsService } from '../../services/websocket';
import { fallbackManager } from '../../services/fallbackPollingManager';
import type { WSEvent } from '../../types';
import { useUserTimezone } from './hooks/useUserTimezone';
import ScoreBreakdownTree from './ScoreBreakdownTree';
import OverrideEntryButton from './OverrideEntryButton';
import { GapIndicatorCompact } from './GapIndicator';

interface EntryDecisionEngineCardProps {
  /** Whether the card is initially expanded */
  defaultExpanded?: boolean;
  /** Optional CSS class */
  className?: string;
}

// Status color mapping
const getStatusColorClass = (color: string): string => {
  switch (color) {
    case 'green':
      return 'bg-green-500/20 text-green-400 border-green-500/30';
    case 'light-green':
      return 'bg-emerald-500/20 text-emerald-400 border-emerald-500/30';
    case 'yellow':
      return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30';
    case 'orange':
      return 'bg-orange-500/20 text-orange-400 border-orange-500/30';
    case 'red':
      return 'bg-red-500/20 text-red-400 border-red-500/30';
    default:
      return 'bg-gray-500/20 text-gray-400 border-gray-500/30';
  }
};

// Gap display with direction arrow
const GapDisplay = ({ gap, threshold }: { gap: number; threshold: number }) => {
  if (gap <= 0) {
    return (
      <span className="flex items-center gap-1 text-green-400">
        <CheckCircle className="w-4 h-4" />
        <span className="font-medium">Above Threshold</span>
      </span>
    );
  }

  return (
    <span className="flex items-center gap-1 text-orange-400">
      <ArrowUp className="w-4 h-4" />
      <span className="font-medium">+{gap} to reach {threshold}</span>
    </span>
  );
};

// Fallback polling key for disconnect recovery
const FALLBACK_KEY = 'entryDecisionEngine';

export default function EntryDecisionEngineCard({
  defaultExpanded = false,
  className = '',
}: EntryDecisionEngineCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const [coinStates, setCoinStates] = useState<CoinStateWithGapsResponse[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [expandedCoins, setExpandedCoins] = useState<Set<string>>(new Set());
  const hasInitializedExpansion = useRef(false);
  // Track if auto-expansion has occurred (to respect user's manual collapse)
  const autoExpandedOnDataRef = useRef(false);

  // Use user's timezone for datetime display
  const { formatTime } = useUserTimezone();

  // Fetch all monitored coin states with gap analysis from the decision engine
  const fetchCoinStates = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      // Try the new gap analysis endpoint first
      let states: CoinStateWithGapsResponse[];
      try {
        states = await futuresApi.getAllCoinStatesWithGaps();
      } catch {
        // Fall back to regular endpoint if gaps endpoint not available
        const regularStates = await futuresApi.getAllCoinStates();
        states = regularStates.map((s: CoinStateResponse, index: number) => {
          // Convert percentage scores back to actual points
          const technicalPoints = Math.round(s.scores.technical * 40 / 100);
          const contextPoints = Math.round(s.scores.context * 30 / 100);
          const llmPoints = Math.round(s.scores.llm * 20 / 100);
          const historyPoints = Math.round(s.scores.history * 10 / 100);

          // Distribute sub-components proportionally so they SUM to the parent total
          // Technical (15 + 10 + 10 + 5 = 40)
          const trendAlign = Math.round(technicalPoints * 15 / 40);
          const momentum = Math.round(technicalPoints * 10 / 40);
          const volatility = Math.round(technicalPoints * 10 / 40);
          const volume = technicalPoints - trendAlign - momentum - volatility;

          // Context (10 + 10 + 10 = 30)
          const regimeMatch = Math.round(contextPoints * 10 / 30);
          const timeframeAlign = Math.round(contextPoints * 10 / 30);
          const btcTrend = contextPoints - regimeMatch - timeframeAlign;

          // History (5 + 5 = 10)
          const symbolWinrate = Math.round(historyPoints / 2);
          const strategyWinrate = historyPoints - symbolWinrate;

          return {
          ...s,
          score_breakdown: {
            technical: technicalPoints,
            technical_max: 40,
            trend_alignment: trendAlign,
            trend_alignment_max: 15,
            momentum: momentum,
            momentum_max: 10,
            volatility: volatility,
            volatility_max: 10,
            volume: volume,
            volume_max: 5,
            context: contextPoints,
            context_max: 30,
            regime_match: regimeMatch,
            regime_match_max: 10,
            timeframe_align: timeframeAlign,
            timeframe_align_max: 10,
            btc_trend: btcTrend,
            btc_trend_max: 10,
            llm: llmPoints,
            llm_max: 20,
            history: historyPoints,
            history_max: 10,
            symbol_winrate: symbolWinrate,
            symbol_winrate_max: 5,
            strategy_winrate: strategyWinrate,
            strategy_winrate_max: 5,
            final: s.scores.final,
            threshold: 55,
            gap_to_threshold: Math.max(0, 55 - s.scores.final),
          },
          blocking_with_gaps: s.blocking.all_reasons.map(r => ({
            code: r.code,
            category: r.category,
            description: r.description,
            current_value: typeof r.value === 'number' ? r.value : 0,
            target_value: typeof r.threshold === 'number' ? r.threshold : 0,
            gap: 0,
            gap_direction: 'up' as const,
            gap_display: '',
            overridable: r.overridable,
            timestamp: r.timestamp,
          })),
          score_history: { timestamps: [], scores: [], trend: 'stable' as const, change_8h: 0 },
          overall_gap: Math.max(0, 55 - s.scores.final),
          proximity_rank: index + 1,
          can_override: s.blocking.hard_block_count === 0 && s.blocking.soft_block_count > 0,
          status_label: s.decision === 'READY' ? 'Ready for Entry' : 'Blocked',
          status_color: (s.decision === 'READY' ? 'green' : 'red') as CoinStateWithGapsResponse['status_color'],
        };
        });
        // Sort by gap ascending
        states.sort((a, b) => a.overall_gap - b.overall_gap);
      }

      setCoinStates(states);
      setLastUpdated(new Date());

      // Auto-expand only the TOP coin on first load (single expandable mode)
      if (!hasInitializedExpansion.current && states.length > 0) {
        hasInitializedExpansion.current = true;
        const topCoin = states[0].symbol;
        setExpandedCoins(new Set([topCoin]));
      }
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
  }, []); // Note: expandedCoins.size removed - auto-expand is initialization only

  // Fetch on mount and when expanded
  useEffect(() => {
    if (expanded) {
      fetchCoinStates();
    }
  }, [expanded, fetchCoinStates]);

  // Auto-expand when coin states data is received for the first time
  useEffect(() => {
    if (coinStates.length > 0 && !autoExpandedOnDataRef.current) {
      setExpanded(true);
      autoExpandedOnDataRef.current = true;
    }
  }, [coinStates.length]);

  // WebSocket subscription for real-time COIN_STATE_UPDATE events
  // Updates state directly from WebSocket data - NO API calls
  useEffect(() => {
    if (!expanded) return;

    // Handler for coin state updates - updates state directly from WebSocket data
    const handleCoinStateUpdate = (event: WSEvent) => {
      const data = event.data?.coin_state;
      if (!data || !data.symbol) return;

      setCoinStates(prevStates => {
        // Find and update the coin in the existing state
        const existingIndex = prevStates.findIndex(s => s.symbol === data.symbol);

        if (existingIndex === -1) {
          // New coin - add it to the list (will get full data on next initial fetch)
          return prevStates;
        }

        // Update existing coin with WebSocket data
        const updatedStates = [...prevStates];
        const existingCoin = updatedStates[existingIndex];

        // Calculate score breakdown from raw scores
        const technicalPoints = data.score_technical || 0;
        const contextPoints = data.score_context || 0;
        const llmPoints = data.score_llm || 0;
        const historyPoints = data.score_history || 0;
        const finalScore = data.score_final || 0;
        const threshold = existingCoin.score_breakdown?.threshold || 55;

        // Distribute sub-components proportionally
        const trendAlign = Math.round(technicalPoints * 15 / 40);
        const momentum = Math.round(technicalPoints * 10 / 40);
        const volatility = Math.round(technicalPoints * 10 / 40);
        const volume = technicalPoints - trendAlign - momentum - volatility;
        const regimeMatch = Math.round(contextPoints * 10 / 30);
        const timeframeAlign = Math.round(contextPoints * 10 / 30);
        const btcTrend = contextPoints - regimeMatch - timeframeAlign;
        const symbolWinrate = Math.round(historyPoints / 2);
        const strategyWinrate = historyPoints - symbolWinrate;

        // Parse blocking reasons from WebSocket data
        const blockingReasons = data.blocking_reasons || [];
        const hardBlocks = blockingReasons.filter((r: string) =>
          r.includes('HARD') || r.includes('hard') ||
          r.includes('max_position') || r.includes('circuit_breaker')
        );
        const softBlocks = blockingReasons.filter((r: string) => !hardBlocks.includes(r));

        // Update the coin state with WebSocket data
        updatedStates[existingIndex] = {
          ...existingCoin,
          regime: data.regime || existingCoin.regime,
          decision: data.decision || existingCoin.decision,
          scores: {
            ...existingCoin.scores,
            technical: Math.round((technicalPoints / 40) * 100),
            context: Math.round((contextPoints / 30) * 100),
            llm: Math.round((llmPoints / 20) * 100),
            history: Math.round((historyPoints / 10) * 100),
            final: finalScore,
          },
          indicators: {
            ...existingCoin.indicators,
            rsi: data.rsi ?? existingCoin.indicators?.rsi,
            adx: data.adx ?? existingCoin.indicators?.adx,
            atr: data.atr ?? existingCoin.indicators?.atr,
            ema_9: data.ema_9 ?? existingCoin.indicators?.ema_9,
            ema_21: data.ema_21 ?? existingCoin.indicators?.ema_21,
          },
          blocking: {
            ...existingCoin.blocking,
            is_blocked: blockingReasons.length > 0,
            hard_block_count: hardBlocks.length,
            soft_block_count: softBlocks.length,
            all_reasons: blockingReasons.map((r: string) => ({
              code: r,
              category: hardBlocks.includes(r) ? 'HARD' : 'SOFT',
              description: r,
              overridable: !hardBlocks.includes(r),
            })),
          },
          score_breakdown: {
            ...existingCoin.score_breakdown,
            technical: technicalPoints,
            trend_alignment: trendAlign,
            momentum: momentum,
            volatility: volatility,
            volume: volume,
            context: contextPoints,
            regime_match: regimeMatch,
            timeframe_align: timeframeAlign,
            btc_trend: btcTrend,
            llm: llmPoints,
            history: historyPoints,
            symbol_winrate: symbolWinrate,
            strategy_winrate: strategyWinrate,
            final: finalScore,
            gap_to_threshold: Math.max(0, threshold - finalScore),
          },
          overall_gap: Math.max(0, threshold - finalScore),
          status_label: data.decision === 'READY' ? 'Ready for Entry' : 'Blocked',
          status_color: (data.decision === 'READY' ? 'green' : 'red') as CoinStateWithGapsResponse['status_color'],
          can_override: hardBlocks.length === 0 && softBlocks.length > 0,
        };

        // Re-sort by gap ascending
        updatedStates.sort((a, b) => a.overall_gap - b.overall_gap);

        return updatedStates;
      });

      // Update last updated timestamp
      setLastUpdated(new Date());
    };

    // Handler for reconnection - refresh full data on reconnect
    const handleConnect = () => {
      fetchCoinStates();
    };

    // Subscribe to WebSocket events
    wsService.subscribe('COIN_STATE_UPDATE', handleCoinStateUpdate);
    wsService.onConnect(handleConnect);

    // Register with fallbackManager for centralized fallback polling (backup only)
    fallbackManager.registerFetchFunction(FALLBACK_KEY, fetchCoinStates);

    return () => {
      wsService.unsubscribe('COIN_STATE_UPDATE', handleCoinStateUpdate);
      wsService.offConnect(handleConnect);
      fallbackManager.unregisterFetchFunction(FALLBACK_KEY);
    };
  }, [expanded, fetchCoinStates]);

  // Toggle coin expansion - only ONE coin can be expanded at a time
  const toggleCoinExpansion = (symbol: string) => {
    setExpandedCoins(prev => {
      // If clicking the already expanded coin, collapse it
      if (prev.has(symbol)) {
        return new Set();
      }
      // Otherwise, expand only this coin (collapse all others)
      return new Set([symbol]);
    });
  };

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
  const nearlyReadyCount = coinStates.filter(s => s.overall_gap > 0 && s.overall_gap <= 10).length;

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
              <span className="text-yellow-400">{nearlyReadyCount} nearly ready</span>
              <span className="text-gray-500">|</span>
              <span className="text-red-400">{blockedCount} blocked</span>
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

          {/* Coin States List - Sorted by proximity (gap ascending) */}
          {coinStates.length > 0 && (
            <div className="p-4 space-y-3 max-h-[600px] overflow-y-auto">
              {coinStates.map((state) => {
                const isExpanded = expandedCoins.has(state.symbol);

                return (
                  <div
                    key={state.symbol}
                    className="bg-gray-900/50 rounded-lg border border-gray-700/50 overflow-hidden"
                  >
                    {/* Coin Header - Always Visible */}
                    <button
                      type="button"
                      onClick={() => toggleCoinExpansion(state.symbol)}
                      className="w-full flex items-center justify-between p-4 hover:bg-gray-700/20 transition-colors"
                    >
                      <div className="flex items-center gap-3">
                        {isExpanded ? (
                          <ChevronDown className="w-4 h-4 text-gray-500" />
                        ) : (
                          <ChevronRight className="w-4 h-4 text-gray-500" />
                        )}

                        {/* Rank Badge */}
                        <span className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold ${
                          state.proximity_rank <= 3 ? 'bg-yellow-500/30 text-yellow-400' : 'bg-gray-700 text-gray-400'
                        }`}>
                          {state.proximity_rank}
                        </span>

                        {/* Symbol */}
                        <span className="font-bold text-white text-lg">{state.symbol}</span>

                        {/* Regime Badge */}
                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium ${getRegimeColor(state.regime)}`}>
                          {getRegimeIcon(state.regime)}
                          {state.regime}
                        </span>

                        {/* Status Badge */}
                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium border ${getStatusColorClass(state.status_color)}`}>
                          {state.decision === 'READY' && <CheckCircle className="w-3 h-3" />}
                          {state.decision === 'BLOCKED' && <XCircle className="w-3 h-3" />}
                          {state.decision === 'PENDING' && <AlertCircle className="w-3 h-3" />}
                          {state.status_label}
                        </span>
                      </div>

                      <div className="flex items-center gap-4">
                        {/* Score / Gap Display */}
                        <div className="text-right">
                          <div className="flex items-center gap-2">
                            <span className={`text-xl font-bold ${
                              state.scores.final >= 55 ? 'text-green-400' :
                              state.scores.final >= 45 ? 'text-yellow-400' :
                              'text-red-400'
                            }`}>
                              {state.scores.final}
                            </span>
                            <span className="text-gray-500 text-sm">/100</span>
                          </div>
                          {state.overall_gap > 0 && (
                            <span className="text-xs text-orange-400 flex items-center gap-1">
                              <ArrowUp className="w-3 h-3" />
                              +{state.overall_gap} needed
                            </span>
                          )}
                        </div>

                        {/* Override Button (if applicable) */}
                        {state.can_override && (
                          <div onClick={(e) => e.stopPropagation()}>
                            <OverrideEntryButton
                              symbol={state.symbol}
                              canOverride={state.can_override}
                              softBlockCount={state.blocking.soft_block_count}
                              hardBlockCount={state.blocking.hard_block_count}
                              onOverrideSuccess={fetchCoinStates}
                            />
                          </div>
                        )}
                      </div>
                    </button>

                    {/* Expanded Content */}
                    {isExpanded && (
                      <div className="border-t border-gray-700/50 p-4 space-y-4">
                        {/* Gap Overview */}
                        <div className="flex items-center justify-between p-3 bg-gray-800/50 rounded-lg">
                          <span className="text-sm text-gray-400">Entry Threshold Gap</span>
                          <GapDisplay gap={state.overall_gap} threshold={state.score_breakdown.threshold} />
                        </div>

                        {/* Score Breakdown Tree */}
                        <div>
                          <h4 className="text-sm font-medium text-gray-300 mb-2">Score Breakdown</h4>
                          <ScoreBreakdownTree breakdown={state.score_breakdown} />
                        </div>

                        {/* Blocking Conditions with Gaps */}
                        {state.blocking_with_gaps.length > 0 && (
                          <div>
                            <h4 className="text-sm font-medium text-gray-300 mb-2">
                              Blocking Conditions ({state.blocking.total_reasons})
                            </h4>
                            <div className="space-y-2">
                              {state.blocking_with_gaps.map((reason, idx) => (
                                <div
                                  key={idx}
                                  className={`flex items-center justify-between p-2 rounded-lg border ${
                                    reason.category === 'HARD_BLOCK' ? 'bg-red-500/10 border-red-500/30' :
                                    reason.category === 'SOFT_BLOCK' ? 'bg-yellow-500/10 border-yellow-500/30' :
                                    'bg-blue-500/10 border-blue-500/30'
                                  }`}
                                >
                                  <div className="flex items-center gap-2">
                                    {reason.category === 'HARD_BLOCK' && <XCircle className="w-4 h-4 text-red-400" />}
                                    {reason.category === 'SOFT_BLOCK' && <AlertCircle className="w-4 h-4 text-yellow-400" />}
                                    {reason.category === 'WARNING' && <AlertTriangle className="w-4 h-4 text-blue-400" />}
                                    <div>
                                      <span className={`text-sm font-medium ${
                                        reason.category === 'HARD_BLOCK' ? 'text-red-400' :
                                        reason.category === 'SOFT_BLOCK' ? 'text-yellow-400' :
                                        'text-blue-400'
                                      }`}>
                                        {reason.code.split('_').map(w => w.charAt(0) + w.slice(1).toLowerCase()).join(' ')}
                                      </span>
                                      <p className="text-xs text-gray-500">{reason.description}</p>
                                    </div>
                                  </div>
                                  {reason.gap_display && (
                                    <GapIndicatorCompact
                                      direction={reason.gap_direction}
                                      display={reason.gap_display}
                                    />
                                  )}
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Technical Indicators */}
                        <div className="flex items-center gap-6 text-sm border-t border-gray-700/50 pt-3">
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
                            <span className="text-gray-500">1H:</span>
                            <div className="flex items-center gap-1">
                              {getTrendIcon(state.trend_1h)}
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            <span className="text-gray-500">15M:</span>
                            <div className="flex items-center gap-1">
                              {getTrendIcon(state.trend_15m)}
                            </div>
                          </div>
                          <div className="flex items-center gap-2">
                            <Target className="w-3 h-3 text-gray-500" />
                            <span className="text-gray-400">{state.active_strategy || 'Default'}</span>
                          </div>
                        </div>

                        {/* Score History Trend */}
                        {state.score_history && state.score_history.scores.length > 0 && (
                          <div className="flex items-center gap-2 text-sm border-t border-gray-700/50 pt-3">
                            <span className="text-gray-500">8h Trend:</span>
                            <span className={`font-medium ${
                              state.score_history.trend === 'rising' ? 'text-green-400' :
                              state.score_history.trend === 'falling' ? 'text-red-400' :
                              'text-gray-400'
                            }`}>
                              {state.score_history.trend === 'rising' && <><ArrowUp className="w-3 h-3 inline" /> +{state.score_history.change_8h}</>}
                              {state.score_history.trend === 'falling' && <><ArrowDown className="w-3 h-3 inline" /> {state.score_history.change_8h}</>}
                              {state.score_history.trend === 'stable' && 'Stable'}
                            </span>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}

          {/* Footer with refresh */}
          <div className="px-4 py-3 border-t border-gray-700 bg-gray-900/30 flex items-center justify-between">
            <div className="text-xs text-gray-500">
              {lastUpdated && `Last updated: ${formatTime(lastUpdated.toISOString())}`}
              <span className="ml-2 text-gray-600">| Sorted by proximity to entry</span>
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
