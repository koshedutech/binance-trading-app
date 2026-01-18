// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// useCoinState Hook - WebSocket integration for real-time coin state updates

import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { wsService } from '../../services/websocket';
import {
  CoinState,
  CoinStateUpdatePayload,
  createEmptyCoinState,
  parseCoinStateFromPayload,
  mergeCoinState,
  COIN_STATE_UPDATE_EVENT,
} from '../../types/position-card';
import type { WSEvent } from '../../types';

interface UseCoinStateOptions {
  /** Symbol to subscribe to */
  symbol: string;
  /** Initial state (if already available) */
  initialState?: CoinState;
  /** Optional REST API fallback function */
  fallbackFetch?: (symbol: string) => Promise<CoinState>;
  /** Fallback polling interval in ms (default 30000) */
  fallbackInterval?: number;
}

interface UseCoinStateResult {
  /** Current coin state */
  state: CoinState;
  /** Whether WebSocket is connected */
  isConnected: boolean;
  /** Whether data came from WebSocket (real-time) vs REST */
  isRealTime: boolean;
  /** Timestamp of last update */
  lastUpdate: Date | null;
  /** Manual refresh trigger */
  refresh: () => Promise<void>;
  /** Any error from fallback fetch */
  error: string | null;
  /** Loading state */
  isLoading: boolean;
}

/**
 * Hook to subscribe to coin state updates via WebSocket.
 * Falls back to REST API polling when WebSocket is unavailable.
 */
export function useCoinState({
  symbol,
  initialState,
  fallbackFetch,
  fallbackInterval = 30000,
}: UseCoinStateOptions): UseCoinStateResult {
  const [state, setState] = useState<CoinState>(
    initialState || createEmptyCoinState(symbol)
  );
  const [isConnected, setIsConnected] = useState(false);
  const [isRealTime, setIsRealTime] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(!!fallbackFetch);

  // Track last WebSocket update time
  const lastWsUpdateRef = useRef<number>(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);

  // Fetch data from REST API
  const fetchData = useCallback(async () => {
    if (!fallbackFetch) return;

    try {
      const result = await fallbackFetch(symbol);

      // Only update if no recent WebSocket data (within 5 seconds)
      const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
      if (timeSinceWsUpdate > 5000) {
        setState(result);
        setLastUpdate(new Date());
        setIsRealTime(false);
      }
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch coin state';
      setError(message);
      console.error(`Fallback fetch failed for ${symbol}:`, err);
    } finally {
      setIsLoading(false);
    }
  }, [fallbackFetch, symbol]);

  // Manual refresh function
  const refresh = useCallback(async () => {
    if (fallbackFetch) {
      setIsLoading(true);
      lastWsUpdateRef.current = 0; // Force update
      await fetchData();
    }
  }, [fallbackFetch, fetchData]);

  useEffect(() => {
    // WebSocket message handler
    const handleMessage = (event: WSEvent) => {
      // Check if this event is for our symbol
      const payload = event.data as CoinStateUpdatePayload;
      if (payload.symbol !== symbol) return;

      // Parse and merge the update
      const partialState = parseCoinStateFromPayload(payload);
      setState(current => mergeCoinState(current, partialState));

      setLastUpdate(new Date());
      setIsRealTime(true);
      setIsConnected(true);
      lastWsUpdateRef.current = Date.now();
    };

    // Connection status handlers
    const handleConnect = () => {
      setIsConnected(true);
    };

    const handleDisconnect = () => {
      setIsConnected(false);
      setIsRealTime(false);
    };

    // Subscribe to WebSocket events
    wsService.subscribe(COIN_STATE_UPDATE_EVENT, handleMessage);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    // Set initial connection state
    setIsConnected(wsService.isConnected());

    // Initial fetch via REST
    if (fallbackFetch) {
      fetchData();
    }

    // Set up REST polling as fallback
    if (fallbackFetch && fallbackInterval > 0) {
      fallbackTimerRef.current = setInterval(() => {
        const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
        if (timeSinceWsUpdate > 5000) {
          fetchData();
        }
      }, fallbackInterval);
    }

    // Cleanup - must remove all registered callbacks to prevent memory leaks
    return () => {
      wsService.unsubscribe(COIN_STATE_UPDATE_EVENT, handleMessage);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);

      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [symbol, fallbackFetch, fallbackInterval, fetchData]);

  return {
    state,
    isConnected,
    isRealTime,
    lastUpdate,
    refresh,
    error,
    isLoading,
  };
}

/**
 * Hook to subscribe to multiple coin states.
 */
interface UseMultipleCoinStatesOptions {
  /** Symbols to subscribe to */
  symbols: string[];
  /** Initial states (if already available) */
  initialStates?: Record<string, CoinState>;
  /** Optional REST API fallback function */
  fallbackFetch?: () => Promise<Record<string, CoinState>>;
  /** Fallback polling interval in ms */
  fallbackInterval?: number;
}

interface UseMultipleCoinStatesResult {
  /** Map of symbol to coin state */
  states: Record<string, CoinState>;
  /** Whether WebSocket is connected */
  isConnected: boolean;
  /** Whether any data came from WebSocket */
  isRealTime: boolean;
  /** Manual refresh trigger */
  refresh: () => Promise<void>;
  /** Any error */
  error: string | null;
  /** Loading state */
  isLoading: boolean;
}

export function useMultipleCoinStates({
  symbols,
  initialStates,
  fallbackFetch,
  fallbackInterval = 30000,
}: UseMultipleCoinStatesOptions): UseMultipleCoinStatesResult {
  const [states, setStates] = useState<Record<string, CoinState>>(() => {
    const initial: Record<string, CoinState> = {};
    for (const symbol of symbols) {
      initial[symbol] = initialStates?.[symbol] || createEmptyCoinState(symbol);
    }
    return initial;
  });
  const [isConnected, setIsConnected] = useState(false);
  const [isRealTime, setIsRealTime] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(!!fallbackFetch);

  const lastWsUpdateRef = useRef<number>(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);

  const fetchData = useCallback(async () => {
    if (!fallbackFetch) return;

    try {
      const result = await fallbackFetch();
      const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
      if (timeSinceWsUpdate > 5000) {
        setStates(result);
        setIsRealTime(false);
      }
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch coin states';
      setError(message);
    } finally {
      setIsLoading(false);
    }
  }, [fallbackFetch]);

  const refresh = useCallback(async () => {
    if (fallbackFetch) {
      setIsLoading(true);
      lastWsUpdateRef.current = 0;
      await fetchData();
    }
  }, [fallbackFetch, fetchData]);

  // Memoize the sorted symbols string to avoid unnecessary effect re-runs
  // when array order changes but contents are the same
  const symbolsKey = useMemo(() => [...symbols].sort().join(','), [symbols]);

  useEffect(() => {
    const handleMessage = (event: WSEvent) => {
      const payload = event.data as CoinStateUpdatePayload;

      // Check if this symbol is in our list
      if (!symbols.includes(payload.symbol)) return;

      const partialState = parseCoinStateFromPayload(payload);
      setStates(current => ({
        ...current,
        [payload.symbol]: mergeCoinState(
          current[payload.symbol] || createEmptyCoinState(payload.symbol),
          partialState
        ),
      }));

      setIsRealTime(true);
      setIsConnected(true);
      lastWsUpdateRef.current = Date.now();
    };

    const handleConnect = () => setIsConnected(true);
    const handleDisconnect = () => {
      setIsConnected(false);
      setIsRealTime(false);
    };

    wsService.subscribe(COIN_STATE_UPDATE_EVENT, handleMessage);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);

    setIsConnected(wsService.isConnected());

    if (fallbackFetch) {
      fetchData();
    }

    if (fallbackFetch && fallbackInterval > 0) {
      fallbackTimerRef.current = setInterval(() => {
        const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
        if (timeSinceWsUpdate > 5000) {
          fetchData();
        }
      }, fallbackInterval);
    }

    // Cleanup - must remove all registered callbacks to prevent memory leaks
    return () => {
      wsService.unsubscribe(COIN_STATE_UPDATE_EVENT, handleMessage);
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [symbolsKey, symbols, fallbackFetch, fallbackInterval, fetchData]);

  return {
    states,
    isConnected,
    isRealTime,
    refresh,
    error,
    isLoading,
  };
}

export default useCoinState;
