import { useState, useEffect, useCallback, useRef } from 'react';
import { futuresApi } from '../services/futuresApi';

// ============================================================================
// Epic 14: Coin Profiler Hooks
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================

// TypeScript interfaces for Coin Profiler API responses

export interface CoinProfilerStatus {
  running: boolean;
  connected: boolean;
  symbol_count: number;
  subscription_count: number;
  updates_per_second: number;
  last_update_time: string;
  last_error: string;
  reconnect_count: number;
  started_at: string;
  uptime: string;
}

export interface TimeframeData {
  timeframe: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
  taker_buy_vol: number;
  taker_sell_vol: number;
  quote_volume: number;
  trade_count: number;
  is_closed_bar: boolean;
  open_time: string;
  close_time: string;
  updated_at: string;
}

export interface CoinData {
  symbol: string;
  price: number;
  volume_24h: number;
  volatility: number;
  timeframes: Record<string, TimeframeData>;
  source: 'strategy' | 'position' | 'both';
  strategies: string[];
  updated_at: string;
}

export interface CoinProfilerCoinsResponse {
  coins: CoinData[];
  total: number;
}

export interface CoinProfilerRequirements {
  all_timeframes: string[];
  all_data_fields: string[];
  total_strategies: number;
  subscriptions: Record<string, {
    timeframes: string[];
    source: string;
    strategy: string;
  }>;
}

export interface CoinProfilerActionResponse {
  success: boolean;
  message: string;
  status?: CoinProfilerStatus;
}

/**
 * Hook to poll Coin Profiler status
 * Polls every 5 seconds for real-time status updates
 */
export function useCoinProfilerStatus() {
  const [data, setData] = useState<CoinProfilerStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const response = await futuresApi.getCoinProfilerStatus();
      setData(response);
      setError(null);
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to fetch coin profiler status';
      setError(message);
      console.error('Failed to fetch coin profiler status:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    // Initial fetch
    fetchStatus();

    // Poll every 5 seconds
    intervalRef.current = window.setInterval(fetchStatus, 5000);

    return () => {
      if (intervalRef.current) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchStatus]);

  const refetch = useCallback(() => {
    setIsLoading(true);
    fetchStatus();
  }, [fetchStatus]);

  return { data, isLoading, error, refetch };
}

/**
 * Hook to fetch Coin Profiler tracked coins
 * Can be called on demand or with automatic polling
 */
export function useCoinProfilerCoins(autoPoll: boolean = false, pollInterval: number = 10000) {
  const [data, setData] = useState<CoinProfilerCoinsResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  const fetchCoins = useCallback(async () => {
    try {
      const response = await futuresApi.getCoinProfilerCoins();
      setData(response);
      setError(null);
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to fetch coin profiler coins';
      setError(message);
      console.error('Failed to fetch coin profiler coins:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    // Initial fetch
    fetchCoins();

    // Optional polling
    if (autoPoll) {
      intervalRef.current = window.setInterval(fetchCoins, pollInterval);
    }

    return () => {
      if (intervalRef.current) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchCoins, autoPoll, pollInterval]);

  const refetch = useCallback(() => {
    setIsLoading(true);
    fetchCoins();
  }, [fetchCoins]);

  return { data, isLoading, error, refetch };
}

/**
 * Hook to start/stop the Coin Profiler
 */
export function useCoinProfilerControl() {
  const [isStarting, setIsStarting] = useState(false);
  const [isStopping, setIsStopping] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [lastAction, setLastAction] = useState<CoinProfilerActionResponse | null>(null);

  const start = useCallback(async () => {
    setIsStarting(true);
    setError(null);
    setLastAction(null);

    try {
      const response = await futuresApi.startCoinProfiler();
      setLastAction(response);
      return response;
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to start coin profiler';
      setError(message);
      throw err;
    } finally {
      setIsStarting(false);
    }
  }, []);

  const stop = useCallback(async () => {
    setIsStopping(true);
    setError(null);
    setLastAction(null);

    try {
      const response = await futuresApi.stopCoinProfiler();
      setLastAction(response);
      return response;
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to stop coin profiler';
      setError(message);
      throw err;
    } finally {
      setIsStopping(false);
    }
  }, []);

  const reset = useCallback(() => {
    setIsStarting(false);
    setIsStopping(false);
    setError(null);
    setLastAction(null);
  }, []);

  return { start, stop, isStarting, isStopping, error, lastAction, reset };
}
