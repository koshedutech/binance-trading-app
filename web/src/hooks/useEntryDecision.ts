// Entry Decision System Hooks
// Epic 14: Chain Trading System - Story 14.13: Frontend UI Enhancement

import { useState, useEffect, useCallback, useRef } from 'react';
import { apiService } from '../services/api';
import { wsService } from '../services/websocket';
import type { WSEvent } from '../types';
import type {
  EntryDecisionStrategiesResponse,
  EntryCandidatesResponse,
  PatternProgressResponse,
  ScoreBreakdownResponse,
  StrategyMatch,
  ModeStrategies,
  EntryCandidate,
  PatternUpdate,
  PatternUpdatesResponse,
  EntryLevelsApiResponse,
} from '../types/entryDecision';

// ==================== Hook Interfaces ====================

interface UseEntryDecisionStrategiesResult {
  /** All strategies with their matching coins */
  strategies: StrategyMatch[];
  /** Strategies grouped by mode */
  byMode: ModeStrategies[];
  /** Summary statistics */
  stats: {
    totalStrategies: number;
    enabledStrategies: number;
    totalCoinsReady: number;
    totalCoinsWatching: number;
  };
  /** Loading state */
  isLoading: boolean;
  /** Whether using real-time WebSocket updates */
  isRealTime: boolean;
  /** Error message */
  error: string | null;
  /** Last update timestamp */
  lastUpdated: Date | null;
  /** Next candle close timestamp - for countdown timer display */
  nextCandleClose: Date | null;
  /** What direction we're looking for ("long", "short", or "both") */
  lookingFor: string | null;
  /** Refresh function */
  refresh: () => Promise<void>;
}

interface UseEntryCandidatesResult {
  /** Coins ready for entry */
  candidates: EntryCandidate[];
  /** Total count */
  totalCandidates: number;
  /** Loading state */
  isLoading: boolean;
  /** Error message */
  error: string | null;
  /** Refresh function */
  refresh: () => Promise<void>;
}

interface UsePatternProgressResult {
  /** Pattern progress data */
  progress: PatternProgressResponse | null;
  /** Loading state */
  isLoading: boolean;
  /** Error message */
  error: string | null;
  /** Refresh function */
  refresh: () => Promise<void>;
}

interface UseScoreBreakdownResult {
  /** Score breakdown data */
  breakdown: ScoreBreakdownResponse | null;
  /** Loading state */
  isLoading: boolean;
  /** Error message */
  error: string | null;
  /** Refresh function */
  refresh: () => Promise<void>;
}

// ==================== Type Guards ====================

/**
 * Type guard to validate WebSocket entry decision response
 */
function isEntryDecisionResponse(data: unknown): data is EntryDecisionStrategiesResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'strategies' in data &&
    Array.isArray((data as EntryDecisionStrategiesResponse).strategies)
  );
}

/**
 * Type guard to validate individual pattern update
 */
function isPatternUpdate(data: unknown): data is PatternUpdate {
  return (
    typeof data === 'object' &&
    data !== null &&
    'symbol' in data &&
    'status' in data &&
    typeof (data as PatternUpdate).symbol === 'string'
  );
}

// ==================== useEntryDecisionStrategies Hook ====================

/**
 * Hook to fetch and subscribe to Entry Decision strategies.
 * Uses WebSocket for real-time updates with REST fallback.
 *
 * @param mode Optional mode filter (scalp, swing, position, ultra_fast)
 */
export function useEntryDecisionStrategies(mode?: string): UseEntryDecisionStrategiesResult {
  const [strategies, setStrategies] = useState<StrategyMatch[]>([]);
  const [byMode, setByMode] = useState<ModeStrategies[]>([]);
  const [stats, setStats] = useState({
    totalStrategies: 0,
    enabledStrategies: 0,
    totalCoinsReady: 0,
    totalCoinsWatching: 0,
  });
  const [isLoading, setIsLoading] = useState(true);
  const [isRealTime, setIsRealTime] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [nextCandleClose, setNextCandleClose] = useState<Date | null>(null);
  const [lookingFor, setLookingFor] = useState<string | null>(null);

  const lastWsUpdateRef = useRef<number>(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);

  const fetchStrategies = useCallback(async () => {
    try {
      const endpoint = mode
        ? `/futures/entry-decision/strategies/${mode}`
        : '/futures/entry-decision/strategies';

      const response = await apiService.get<EntryDecisionStrategiesResponse>(endpoint);
      const data = response.data;

      // Always update from REST if WebSocket isn't active
      // (No artificial delay - update immediately)
      if (!isRealTime) {
        setStrategies(data.strategies || []);
        setByMode(data.by_mode || []);
        setStats({
          totalStrategies: data.total_strategies || 0,
          enabledStrategies: data.enabled_strategies || 0,
          totalCoinsReady: data.total_coins_ready || 0,
          totalCoinsWatching: data.total_coins_watching || 0,
        });
        setLastUpdated(new Date());
        // Extract countdown from REST response as fallback
        if (data.next_candle_close) {
          setNextCandleClose(new Date(data.next_candle_close));
        }
        if (data.looking_for) {
          setLookingFor(data.looking_for);
        }
      }
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch strategies';
      setError(message);
      console.error('Failed to fetch entry decision strategies:', err);
    } finally {
      setIsLoading(false);
    }
  }, [mode, isRealTime]);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    lastWsUpdateRef.current = 0; // Force update
    await fetchStrategies();
  }, [fetchStrategies]);

  useEffect(() => {
    // Force-clear stale state for a specific symbol.
    // Called when a position closes (CHAIN_LIFECYCLE_UPDATE / CHAIN_CLOSED) to ensure
    // the coin transitions cleanly back to "watching" without stale reference_candle
    // or position_running status being preserved by the merge logic.
    const forceClearSymbolState = (symbol: string) => {
      setStrategies(prev => prev.map(strategy => ({
        ...strategy,
        coins: strategy.coins.map(coin => {
          if (coin.symbol !== symbol) return coin;
          // Reset to clean watching state - clear all step 2+ fields
          return {
            ...coin,
            status: 'watching',
            step: 0,
            step_details: undefined,
            reference_candle: undefined,
            entry_candle: undefined,
            has_active_position: false,
            position_entry_price: undefined,
            position_opened_at: undefined,
            chain_id: undefined,
            closed_pnl: undefined,
            closed_reason: undefined,
            proximity_to_breakout: undefined,
            seconds_since_reference: undefined,
            seconds_until_expiry: undefined,
            seconds_ref_to_entry: undefined,
            potential_breakout: false,
            candles_since_reference: undefined,
          };
        }),
      })));

      setByMode(prev => prev.map(modeGroup => ({
        ...modeGroup,
        strategies: modeGroup.strategies.map(strategy => ({
          ...strategy,
          coins: strategy.coins.map(coin => {
            if (coin.symbol !== symbol) return coin;
            return {
              ...coin,
              status: 'watching',
              step: 0,
              step_details: undefined,
              reference_candle: undefined,
              entry_candle: undefined,
              has_active_position: false,
              position_entry_price: undefined,
              position_opened_at: undefined,
              chain_id: undefined,
              closed_pnl: undefined,
              closed_reason: undefined,
              proximity_to_breakout: undefined,
              seconds_since_reference: undefined,
              seconds_until_expiry: undefined,
              seconds_ref_to_entry: undefined,
              potential_breakout: false,
              candles_since_reference: undefined,
            };
          }),
        })),
      })));
    };

    // WebSocket message handler - IMMEDIATE updates, no throttling
    // CRITICAL: Merge strategy data to preserve reference_candle and other state that may not be in every update
    const handleStrategyUpdate = (event: WSEvent) => {
      if (isEntryDecisionResponse(event.data)) {
        // Handle clear signal: profiler stopped, reset all state
        if (event.data.cleared) {
          setStrategies([]);
          setByMode([]);
          setStats({
            totalStrategies: 0,
            enabledStrategies: 0,
            totalCoinsReady: 0,
            totalCoinsWatching: 0,
          });
          setNextCandleClose(null);
          setLookingFor(null);
          setLastUpdated(new Date());
          setIsRealTime(true);
          return;
        }

        const newStrategies = event.data.strategies;

        // If strategies are empty and all counts are 0, replace state entirely (don't merge)
        if (
          newStrategies.length === 0 &&
          (event.data.total_coins_ready || 0) === 0 &&
          (event.data.total_coins_watching || 0) === 0
        ) {
          setStrategies([]);
          setByMode(event.data.by_mode || []);
          setStats({
            totalStrategies: event.data.total_strategies || 0,
            enabledStrategies: event.data.enabled_strategies || 0,
            totalCoinsReady: 0,
            totalCoinsWatching: 0,
          });
          setLastUpdated(new Date());
          setIsRealTime(true);
          return;
        }

        // Merge with existing strategies to preserve fields that may not be in the update
        // This prevents UI flicker when reference_candle or other fields are temporarily missing
        setStrategies(prevStrategies => {
          return newStrategies.map(newStrategy => {
            // Find the previous strategy with the same identity
            const prevStrategy = prevStrategies.find(s =>
              s.strategy === newStrategy.strategy &&
              s.sub_strategy === newStrategy.sub_strategy &&
              s.timeframe === newStrategy.timeframe
            );

            if (!prevStrategy) return newStrategy;

            // Merge coins: preserve reference_candle and entry_candle from previous state
            const mergedCoins = newStrategy.coins.map(newCoin => {
              const prevCoin = prevStrategy.coins.find(c => c.symbol === newCoin.symbol);
              if (!prevCoin) return newCoin;

              // Merge: new data takes priority, but preserve important fields if missing
              // CRITICAL: Preserve step and status to prevent UI flicker when these are temporarily missing

              // Smart status preservation: If we were in Step 2 (have reference_candle) and the new status
              // would regress to 'watching', preserve the previous status to prevent flicker.
              // This handles partial WebSocket updates that don't include all fields.
              // EXCEPTION: If previous state was position_running/position_closed, ALWAYS allow
              // regression to watching (this is the legitimate lifecycle transition).
              const step2Statuses = ['accumulation', 'consolidating', 'ready'];
              const step4Statuses = ['position_running', 'position_closed'];
              const prevWasStep2 = step2Statuses.includes(prevCoin.status || '');
              const prevWasStep4 = step4Statuses.includes(prevCoin.status || '');
              const hasReferenceCandle = newCoin.reference_candle || prevCoin.reference_candle;
              const newStatusIsRegression = newCoin.status === 'watching' || !newCoin.status;

              // Only preserve Step 2 status when it would regress AND we're not coming from Step 4
              const shouldPreserve = prevWasStep2 && !prevWasStep4 && hasReferenceCandle && newStatusIsRegression;

              // Determine the status to use
              const mergedStatus = shouldPreserve
                ? prevCoin.status  // Preserve Step 2 status when it would regress
                : (newCoin.status || prevCoin.status);

              // When transitioning back to watching (from step4 or explicit reset),
              // do NOT preserve stale step 2+ data (reference_candle, entry_candle, etc.)
              const isResettingToWatching = newCoin.status === 'watching' && (prevWasStep4 || !newCoin.reference_candle);
              const cleanReset = prevWasStep4 && newStatusIsRegression;

              return {
                ...newCoin,
                // Preserve step if not in new data (prevents section from hiding) - unless resetting
                step: cleanReset ? (newCoin.step || 0)
                  : (newCoin.step !== undefined && newCoin.step > 0 ? newCoin.step : prevCoin.step),
                // Use smart status preservation (see above)
                status: mergedStatus,
                // Preserve step_details if not in new data - unless resetting
                step_details: cleanReset ? newCoin.step_details : (newCoin.step_details || prevCoin.step_details),
                // Preserve reference_candle if not in new data - unless resetting to watching
                reference_candle: isResettingToWatching ? newCoin.reference_candle : (newCoin.reference_candle || prevCoin.reference_candle),
                // Preserve entry_candle if not in new data - unless resetting
                entry_candle: isResettingToWatching ? newCoin.entry_candle : (newCoin.entry_candle || prevCoin.entry_candle),
                // Preserve current_price if not in new data (prevents price flickering)
                current_price: newCoin.current_price || prevCoin.current_price,
                // Preserve volume data if not in new data
                volume_multiplier: newCoin.volume_multiplier ?? prevCoin.volume_multiplier,
                volume_threshold: newCoin.volume_threshold ?? prevCoin.volume_threshold,
                // Preserve entry_levels (SL/TP/R:R) - critical for Step 4 display stability
                entry_levels: isResettingToWatching ? newCoin.entry_levels : (newCoin.entry_levels || prevCoin.entry_levels),
              };
            });

            return { ...newStrategy, coins: mergedCoins };
          });
        });

        if (event.data.by_mode) {
          // Also merge by_mode data to preserve reference_candle
          setByMode(prevByMode => {
            return event.data.by_mode!.map(newModeGroup => {
              const prevModeGroup = prevByMode.find(m => m.mode === newModeGroup.mode);
              if (!prevModeGroup) return newModeGroup;

              const mergedStrategies = newModeGroup.strategies.map(newStrategy => {
                const prevStrategy = prevModeGroup.strategies.find(s =>
                  s.strategy === newStrategy.strategy &&
                  s.sub_strategy === newStrategy.sub_strategy
                );
                if (!prevStrategy) return newStrategy;

                const mergedCoins = newStrategy.coins.map(newCoin => {
                  const prevCoin = prevStrategy.coins.find(c => c.symbol === newCoin.symbol);
                  if (!prevCoin) return newCoin;

                  // CRITICAL: Preserve step and status to prevent UI flicker
                  // Smart status preservation (same logic as strategies merge above)
                  // EXCEPTION: Allow regression from position_running/position_closed to watching
                  const step2Statuses = ['accumulation', 'consolidating', 'ready'];
                  const step4Statuses = ['position_running', 'position_closed'];
                  const prevWasStep2 = step2Statuses.includes(prevCoin.status || '');
                  const prevWasStep4 = step4Statuses.includes(prevCoin.status || '');
                  const hasReferenceCandle = newCoin.reference_candle || prevCoin.reference_candle;
                  const newStatusIsRegression = newCoin.status === 'watching' || !newCoin.status;

                  const shouldPreserve = prevWasStep2 && !prevWasStep4 && hasReferenceCandle && newStatusIsRegression;

                  const mergedStatus = shouldPreserve
                    ? prevCoin.status
                    : (newCoin.status || prevCoin.status);

                  const isResettingToWatching = newCoin.status === 'watching' && (prevWasStep4 || !newCoin.reference_candle);
                  const cleanReset = prevWasStep4 && newStatusIsRegression;

                  return {
                    ...newCoin,
                    step: cleanReset ? (newCoin.step || 0)
                      : (newCoin.step !== undefined && newCoin.step > 0 ? newCoin.step : prevCoin.step),
                    status: mergedStatus,
                    step_details: cleanReset ? newCoin.step_details : (newCoin.step_details || prevCoin.step_details),
                    reference_candle: isResettingToWatching ? newCoin.reference_candle : (newCoin.reference_candle || prevCoin.reference_candle),
                    entry_candle: isResettingToWatching ? newCoin.entry_candle : (newCoin.entry_candle || prevCoin.entry_candle),
                    current_price: newCoin.current_price || prevCoin.current_price,
                    volume_multiplier: newCoin.volume_multiplier ?? prevCoin.volume_multiplier,
                    volume_threshold: newCoin.volume_threshold ?? prevCoin.volume_threshold,
                    // Preserve entry_levels (SL/TP/R:R) - critical for Step 4 display stability
                    entry_levels: isResettingToWatching ? newCoin.entry_levels : (newCoin.entry_levels || prevCoin.entry_levels),
                  };
                });

                return { ...newStrategy, coins: mergedCoins };
              });

              return { ...newModeGroup, strategies: mergedStrategies };
            });
          });
        }

        setStats({
          totalStrategies: event.data.total_strategies || 0,
          enabledStrategies: event.data.enabled_strategies || 0,
          totalCoinsReady: event.data.total_coins_ready || 0,
          totalCoinsWatching: event.data.total_coins_watching || 0,
        });

        // Extract countdown timer data
        if (event.data.next_candle_close) {
          setNextCandleClose(new Date(event.data.next_candle_close));
        }
        if (event.data.looking_for) {
          setLookingFor(event.data.looking_for);
        }

        setIsRealTime(true);
        setError(null); // Clear any errors on successful WebSocket update
        setLastUpdated(new Date());
        lastWsUpdateRef.current = Date.now();
      }
    };

    // Real-time pattern update handler - merges individual coin updates for live proximity display
    // CRITICAL: Uses smart preservation logic to prevent UI flicker when step would regress
    const handlePatternUpdate = (event: WSEvent) => {
      const update = event.data as PatternUpdate;
      if (!isPatternUpdate(update)) return;

      // Merge update into strategies for real-time display
      setStrategies(prevStrategies => {
        return prevStrategies.map(strategy => {
          // Find the coin in this strategy
          const coinIndex = strategy.coins.findIndex(c => c.symbol === update.symbol);
          if (coinIndex === -1) return strategy;

          // Create updated coin with real-time data
          const updatedCoins = [...strategy.coins];
          const prevCoin = updatedCoins[coinIndex];
          const coin = { ...prevCoin };

          // CRITICAL: Smart status preservation to prevent UI flicker
          // If we were in Step 2 (have reference_candle) and the new update would regress
          // to 'watching', preserve the previous status unless it's a legitimate state change.
          // EXCEPTION: Allow regression from position_running/position_closed to watching.
          const step2Statuses = ['accumulation', 'consolidating', 'ready'];
          const step4Statuses = ['position_running', 'position_closed'];
          const prevWasStep2 = step2Statuses.includes(prevCoin.status || '');
          const prevWasStep4 = step4Statuses.includes(prevCoin.status || '');
          const hasReferenceCandle = update.reference_candle || prevCoin.reference_candle;
          const newStatusIsRegression = update.status === 'watching' || !update.status;

          // Only preserve Step 2 when NOT coming from Step 4
          const shouldPreserveStep2 = prevWasStep2 && !prevWasStep4 && hasReferenceCandle && newStatusIsRegression;
          // Clean reset: transitioning from position back to watching
          const cleanReset = prevWasStep4 && newStatusIsRegression;
          const isResettingToWatching = update.status === 'watching' && (prevWasStep4 || !update.reference_candle);

          // Update status with smart preservation
          coin.status = shouldPreserveStep2 ? prevCoin.status : (update.status || prevCoin.status);

          // Update step - never regress from Step 2+ to Step 1 if we have reference_candle
          // BUT allow regression from Step 4 (position) back to Step 1
          if (cleanReset) {
            coin.step = update.current_step || 0;
          } else if (update.current_step !== undefined && update.current_step > 0) {
            // If update has a valid step, use it unless it's a regression with reference_candle
            if (shouldPreserveStep2 && update.current_step < (prevCoin.step || 1)) {
              coin.step = prevCoin.step; // Preserve higher step
            } else {
              coin.step = update.current_step;
            }
          } else {
            coin.step = prevCoin.step; // Preserve existing step if not in update
          }

          // Preserve or update other fields
          coin.total_steps = update.total_steps || prevCoin.total_steps;
          coin.step_details = cleanReset ? update.step_details : (update.step_details || prevCoin.step_details);
          coin.direction = (update.direction as 'long' | 'short' | 'neutral' | undefined) || prevCoin.direction;
          coin.updated_at = update.updated_at;

          // Preserve reference_candle - critical for Step 2 display
          // BUT clear it when resetting to watching (position closed lifecycle)
          coin.reference_candle = isResettingToWatching ? update.reference_candle : (update.reference_candle || prevCoin.reference_candle);

          // Preserve entry_candle - critical for Step 3/4 display
          coin.entry_candle = isResettingToWatching ? update.entry_candle : (update.entry_candle || prevCoin.entry_candle);

          // Update entry levels with real-time price data
          if (update.entry_levels) {
            coin.current_price = update.entry_levels.current_price || prevCoin.current_price;
            // Calculate proximity to breakout from entry levels
            if (update.entry_levels.entry_price && update.entry_levels.current_price) {
              coin.proximity_to_breakout =
                ((update.entry_levels.current_price - update.entry_levels.entry_price) /
                  update.entry_levels.entry_price) * 100;
            }
          } else {
            // Preserve existing current_price if not in update
            coin.current_price = prevCoin.current_price;
          }

          // Preserve full entry_levels object
          coin.entry_levels = update.entry_levels || prevCoin.entry_levels;

          // Position tracking fields - critical for Step 4 display
          coin.has_active_position = update.has_active_position ?? prevCoin.has_active_position;
          coin.position_entry_price = update.position_entry_price || prevCoin.position_entry_price;
          coin.chain_id = update.chain_id || prevCoin.chain_id;
          coin.position_opened_at = update.position_opened_at || prevCoin.position_opened_at;
          coin.seconds_ref_to_entry = update.seconds_ref_to_entry ?? prevCoin.seconds_ref_to_entry;

          // Update volume progress if available
          if (update.volume_progress) {
            coin.volume_multiplier = update.volume_progress.current_ratio;
            coin.volume_threshold = update.volume_progress.required_ratio;
            coin.current_volume = update.volume_progress.current_volume;
            coin.avg_volume = update.volume_progress.average_volume;
          }

          updatedCoins[coinIndex] = coin;
          return { ...strategy, coins: updatedCoins };
        });
      });

      // Also update byMode for grouped view with same preservation logic
      setByMode(prevByMode => {
        return prevByMode.map(modeGroup => ({
          ...modeGroup,
          strategies: modeGroup.strategies.map(strategy => {
            const coinIndex = strategy.coins.findIndex(c => c.symbol === update.symbol);
            if (coinIndex === -1) return strategy;

            const updatedCoins = [...strategy.coins];
            const prevCoin = updatedCoins[coinIndex];
            const coin = { ...prevCoin };

            // CRITICAL: Same smart preservation logic as above
            // EXCEPTION: Allow regression from position_running/position_closed to watching
            const step2Statuses = ['accumulation', 'consolidating', 'ready'];
            const step4Statuses = ['position_running', 'position_closed'];
            const prevWasStep2 = step2Statuses.includes(prevCoin.status || '');
            const prevWasStep4 = step4Statuses.includes(prevCoin.status || '');
            const hasReferenceCandle = update.reference_candle || prevCoin.reference_candle;
            const newStatusIsRegression = update.status === 'watching' || !update.status;
            const shouldPreserveStep2 = prevWasStep2 && !prevWasStep4 && hasReferenceCandle && newStatusIsRegression;
            const cleanReset = prevWasStep4 && newStatusIsRegression;
            const isResettingToWatching = update.status === 'watching' && (prevWasStep4 || !update.reference_candle);

            coin.status = shouldPreserveStep2 ? prevCoin.status : (update.status || prevCoin.status);

            if (cleanReset) {
              coin.step = update.current_step || 0;
            } else if (update.current_step !== undefined && update.current_step > 0) {
              if (shouldPreserveStep2 && update.current_step < (prevCoin.step || 1)) {
                coin.step = prevCoin.step;
              } else {
                coin.step = update.current_step;
              }
            } else {
              coin.step = prevCoin.step;
            }

            coin.direction = (update.direction as 'long' | 'short' | 'neutral' | undefined) || prevCoin.direction;
            coin.reference_candle = isResettingToWatching ? update.reference_candle : (update.reference_candle || prevCoin.reference_candle);

            // Preserve entry_candle - critical for Step 3/4 display
            coin.entry_candle = isResettingToWatching ? update.entry_candle : (update.entry_candle || prevCoin.entry_candle);

            if (update.entry_levels) {
              coin.current_price = update.entry_levels.current_price || prevCoin.current_price;
              if (update.entry_levels.entry_price && update.entry_levels.current_price) {
                coin.proximity_to_breakout =
                  ((update.entry_levels.current_price - update.entry_levels.entry_price) /
                    update.entry_levels.entry_price) * 100;
              }
            }

            // Preserve full entry_levels object
            coin.entry_levels = update.entry_levels || prevCoin.entry_levels;

            // Position tracking fields - critical for Step 4 display
            coin.has_active_position = update.has_active_position ?? prevCoin.has_active_position;
            coin.position_entry_price = update.position_entry_price || prevCoin.position_entry_price;
            coin.chain_id = update.chain_id || prevCoin.chain_id;
            coin.position_opened_at = update.position_opened_at || prevCoin.position_opened_at;
            coin.seconds_ref_to_entry = update.seconds_ref_to_entry ?? prevCoin.seconds_ref_to_entry;

            if (update.volume_progress) {
              coin.volume_multiplier = update.volume_progress.current_ratio;
              coin.volume_threshold = update.volume_progress.required_ratio;
              coin.current_volume = update.volume_progress.current_volume;
            }

            updatedCoins[coinIndex] = coin;
            return { ...strategy, coins: updatedCoins };
          }),
        }));
      });

      setLastUpdated(new Date());
    };

    // CHAIN_CLOSED: When a position closes (SL/TP hit), the backend unsuppresses the symbol
    // for new pattern detection. Refresh strategies so the UI shows the symbol back in "watching" state.
    const handleChainClosed = (event: WSEvent) => {
      const data = event.data;
      console.log('[useEntryDecisionStrategies] CHAIN_CLOSED - refreshing strategies', {
        symbol: data?.symbol,
        closeReason: data?.close_reason,
      });
      // Force-clear stale state for this symbol immediately (before refetch)
      if (data?.symbol) {
        forceClearSymbolState(data.symbol);
      }
      // Refresh after a short delay to allow backend to unsuppress and update state
      setTimeout(() => fetchStrategies(), 1000);
    };

    // CHAIN_LIFECYCLE_UPDATE: Composite event from PositionLifecycleCoordinator.
    // Contains chain close + pattern reset + capacity update in one event.
    // This is the primary event for position close - force-clear stale coin state.
    const handleChainLifecycleUpdate = (event: WSEvent) => {
      const data = event.data;
      const symbol = data?.chain?.symbol || data?.pattern?.symbol;
      console.log('[useEntryDecisionStrategies] CHAIN_LIFECYCLE_UPDATE - clearing stale state', {
        symbol,
        patternStatus: data?.pattern?.status,
        closeReason: data?.chain?.close_reason,
      });
      if (symbol) {
        forceClearSymbolState(symbol);
      }
      // Refresh to get fresh data from backend
      setTimeout(() => fetchStrategies(), 500);
    };

    // POSITION_CREATED: When entry fills, symbol is suppressed. Refresh to reflect new state.
    const handlePositionCreated = () => {
      console.log('[useEntryDecisionStrategies] POSITION_CREATED - refreshing strategies');
      setTimeout(() => fetchStrategies(), 500);
    };

    // Track WebSocket connection status
    const handleConnect = () => {
      setIsRealTime(true);
      console.log('[useEntryDecisionStrategies] WebSocket connected');
    };

    const handleDisconnect = () => {
      setIsRealTime(false);
      console.log('[useEntryDecisionStrategies] WebSocket disconnected, will use REST fallback');
    };

    // Subscribe to WebSocket events
    wsService.subscribe('ENTRY_DECISION_UPDATE', handleStrategyUpdate);
    wsService.subscribe('ENTRY_DECISION_PATTERN_UPDATE', handlePatternUpdate);
    wsService.subscribe('CHAIN_CLOSED', handleChainClosed);
    wsService.subscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
    wsService.subscribe('POSITION_CREATED', handlePositionCreated);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    // Set initial connection state
    setIsRealTime(wsService.isConnected());

    // Initial fetch
    fetchStrategies();

    // REST polling fallback - only when WebSocket is not active
    // Poll every 5 seconds when disconnected (fast fallback)
    fallbackTimerRef.current = setInterval(() => {
      if (!wsService.isConnected()) {
        setIsRealTime(false);
        fetchStrategies();
      }
    }, 5000);

    return () => {
      wsService.unsubscribe('ENTRY_DECISION_UPDATE', handleStrategyUpdate);
      wsService.unsubscribe('ENTRY_DECISION_PATTERN_UPDATE', handlePatternUpdate);
      wsService.unsubscribe('CHAIN_CLOSED', handleChainClosed);
      wsService.unsubscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
      wsService.unsubscribe('POSITION_CREATED', handlePositionCreated);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [fetchStrategies]);

  return {
    strategies,
    byMode,
    stats,
    isLoading,
    isRealTime,
    error,
    lastUpdated,
    nextCandleClose,
    lookingFor,
    refresh,
  };
}

// ==================== useEntryCandidates Hook ====================

/**
 * Hook to fetch entry candidates (coins ready for entry).
 */
export function useEntryCandidates(): UseEntryCandidatesResult {
  const [candidates, setCandidates] = useState<EntryCandidate[]>([]);
  const [totalCandidates, setTotalCandidates] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);

  const fetchCandidates = useCallback(async () => {
    try {
      const response = await apiService.get<EntryCandidatesResponse>(
        '/futures/entry-decision/candidates'
      );
      setCandidates(response.data.candidates || []);
      setTotalCandidates(response.data.total_candidates || 0);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch candidates';
      setError(message);
      console.error('Failed to fetch entry candidates:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    await fetchCandidates();
  }, [fetchCandidates]);

  useEffect(() => {
    // Initial fetch
    fetchCandidates();

    // Set up polling (every 15 seconds for candidates)
    fallbackTimerRef.current = setInterval(fetchCandidates, 15000);

    return () => {
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [fetchCandidates]);

  return {
    candidates,
    totalCandidates,
    isLoading,
    error,
    refresh,
  };
}

// ==================== usePatternProgress Hook ====================

/**
 * Hook to fetch pattern progress for a specific symbol.
 *
 * @param symbol Coin symbol (e.g., "BTCUSDT")
 * @param mode Optional trading mode
 * @param timeframe Optional timeframe
 */
export function usePatternProgress(
  symbol: string,
  mode?: string,
  timeframe?: string
): UsePatternProgressResult {
  const [progress, setProgress] = useState<PatternProgressResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchProgress = useCallback(async () => {
    if (!symbol) {
      setProgress(null);
      setIsLoading(false);
      return;
    }

    try {
      let endpoint = `/futures/entry-decision/pattern/${symbol}`;
      const params: string[] = [];
      if (mode) params.push(`mode=${mode}`);
      if (timeframe) params.push(`timeframe=${timeframe}`);
      if (params.length > 0) {
        endpoint += `?${params.join('&')}`;
      }

      const response = await apiService.get<PatternProgressResponse>(endpoint);
      setProgress(response.data);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch pattern progress';
      setError(message);
      console.error('Failed to fetch pattern progress:', err);
    } finally {
      setIsLoading(false);
    }
  }, [symbol, mode, timeframe]);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    await fetchProgress();
  }, [fetchProgress]);

  useEffect(() => {
    fetchProgress();
  }, [fetchProgress]);

  return {
    progress,
    isLoading,
    error,
    refresh,
  };
}

// ==================== useScoreBreakdown Hook ====================

/**
 * Hook to fetch score breakdown for a specific symbol.
 *
 * @param symbol Coin symbol (e.g., "BTCUSDT")
 */
export function useScoreBreakdown(symbol: string): UseScoreBreakdownResult {
  const [breakdown, setBreakdown] = useState<ScoreBreakdownResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchBreakdown = useCallback(async () => {
    if (!symbol) {
      setBreakdown(null);
      setIsLoading(false);
      return;
    }

    try {
      const response = await apiService.get<ScoreBreakdownResponse>(
        `/futures/entry-decision/score/${symbol}`
      );
      setBreakdown(response.data);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch score breakdown';
      setError(message);
      console.error('Failed to fetch score breakdown:', err);
    } finally {
      setIsLoading(false);
    }
  }, [symbol]);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    await fetchBreakdown();
  }, [fetchBreakdown]);

  useEffect(() => {
    fetchBreakdown();
  }, [fetchBreakdown]);

  return {
    breakdown,
    isLoading,
    error,
    refresh,
  };
}

// ==================== usePatternUpdates Hook ====================

interface UsePatternUpdatesResult {
  /** All pattern updates */
  updates: PatternUpdate[];
  /** Loading state */
  isLoading: boolean;
  /** Error message */
  error: string | null;
  /** Refresh function */
  refresh: () => Promise<void>;
}

/**
 * Type guard to validate WebSocket pattern update
 */
function isPatternUpdateEvent(data: unknown): data is { event_type: string; data: PatternUpdate } {
  return (
    typeof data === 'object' &&
    data !== null &&
    'event_type' in data &&
    (data as { event_type: string }).event_type === 'ENTRY_DECISION_PATTERN_UPDATE' &&
    'data' in data
  );
}

/**
 * Hook to subscribe to real-time pattern updates via WebSocket.
 */
export function usePatternUpdates(): UsePatternUpdatesResult {
  const [updates, setUpdates] = useState<PatternUpdate[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchInitialUpdates = useCallback(async () => {
    try {
      const response = await apiService.get<PatternUpdatesResponse>(
        '/futures/entry-decision/pattern-updates'
      );
      setUpdates(response.data.updates || []);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch pattern updates';
      setError(message);
      console.error('Failed to fetch pattern updates:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    await fetchInitialUpdates();
  }, [fetchInitialUpdates]);

  useEffect(() => {
    // WebSocket message handler for pattern updates
    const handlePatternUpdate = (event: WSEvent) => {
      if (isPatternUpdateEvent(event.data)) {
        const update = event.data.data;

        setUpdates(prev => {
          // Find and update existing, or add new
          const key = `${update.symbol}:${update.mode}:${update.timeframe}`;
          const existingIndex = prev.findIndex(
            u => `${u.symbol}:${u.mode}:${u.timeframe}` === key
          );

          if (existingIndex >= 0) {
            // Update existing
            const newUpdates = [...prev];
            newUpdates[existingIndex] = update;
            return newUpdates;
          } else {
            // Add new
            return [...prev, update];
          }
        });
      }
    };

    // Subscribe to WebSocket events
    wsService.subscribe('ENTRY_DECISION_PATTERN_UPDATE', handlePatternUpdate);

    // Initial fetch
    fetchInitialUpdates();

    return () => {
      wsService.unsubscribe('ENTRY_DECISION_PATTERN_UPDATE', handlePatternUpdate);
    };
  }, [fetchInitialUpdates]);

  return {
    updates,
    isLoading,
    error,
    refresh,
  };
}

// ==================== useEntryLevels Hook ====================

interface UseEntryLevelsResult {
  /** Entry levels data */
  levels: EntryLevelsApiResponse | null;
  /** Loading state */
  isLoading: boolean;
  /** Error message */
  error: string | null;
  /** Refresh function */
  refresh: () => Promise<void>;
}

/**
 * Hook to fetch entry levels for a specific symbol.
 *
 * @param symbol Coin symbol (e.g., "BTCUSDT")
 * @param mode Optional trading mode
 * @param timeframe Optional timeframe
 */
export function useEntryLevels(
  symbol: string,
  mode?: string,
  timeframe?: string
): UseEntryLevelsResult {
  const [levels, setLevels] = useState<EntryLevelsApiResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchLevels = useCallback(async () => {
    if (!symbol) {
      setLevels(null);
      setIsLoading(false);
      return;
    }

    try {
      let endpoint = `/futures/entry-decision/entry-levels/${symbol}`;
      const params: string[] = [];
      if (mode) params.push(`mode=${mode}`);
      if (timeframe) params.push(`timeframe=${timeframe}`);
      if (params.length > 0) {
        endpoint += `?${params.join('&')}`;
      }

      const response = await apiService.get<EntryLevelsApiResponse>(endpoint);
      setLevels(response.data);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch entry levels';
      setError(message);
      console.error('Failed to fetch entry levels:', err);
    } finally {
      setIsLoading(false);
    }
  }, [symbol, mode, timeframe]);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    await fetchLevels();
  }, [fetchLevels]);

  useEffect(() => {
    fetchLevels();
  }, [fetchLevels]);

  return {
    levels,
    isLoading,
    error,
    refresh,
  };
}

// ==================== Combined Hook ====================

/**
 * Combined hook for Entry Decision System with all data
 */
export function useEntryDecision() {
  const strategiesResult = useEntryDecisionStrategies();
  const candidatesResult = useEntryCandidates();
  const patternUpdatesResult = usePatternUpdates();

  return {
    strategies: strategiesResult,
    candidates: candidatesResult,
    patternUpdates: patternUpdatesResult,
    isLoading: strategiesResult.isLoading || candidatesResult.isLoading || patternUpdatesResult.isLoading,
    hasError: !!strategiesResult.error || !!candidatesResult.error || !!patternUpdatesResult.error,
    refresh: async () => {
      await Promise.all([
        strategiesResult.refresh(),
        candidatesResult.refresh(),
        patternUpdatesResult.refresh(),
      ]);
    },
  };
}
