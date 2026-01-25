// Story 14.15: Trading State Hook
// Provides hooks for fetching and updating trading state (ON/OFF toggle)

import { useState, useEffect, useCallback, useRef } from 'react';
import axios from 'axios';

// Token storage keys
const ACCESS_TOKEN_KEY = 'access_token';

// Types based on API response from handlers_trading_state.go
export interface TradingState {
  enabled: boolean;
  entry_active: boolean;
  exit_active: boolean;
  last_changed: string;
  changed_by: string;
  reason?: string;
}

export interface SetTradingEnabledRequest {
  enabled?: boolean;
  reason?: string;
}

// Create axios instance for trading API
const tradingClient = axios.create({
  baseURL: '/api/trading',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth interceptor
tradingClient.interceptors.request.use((config) => {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  if (token) {
    config.headers = config.headers || {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

/**
 * Hook for fetching trading state with polling
 * Polls every 5 seconds to keep state in sync
 */
export function useTradingState(pollInterval = 5000) {
  const [data, setData] = useState<TradingState | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  const fetchState = useCallback(async () => {
    try {
      const response = await tradingClient.get<TradingState>('/state');
      setData(response.data);
      setError(null);
    } catch (err: unknown) {
      const message =
        axios.isAxiosError(err) && err.response?.data?.message
          ? err.response.data.message
          : err instanceof Error
            ? err.message
            : 'Failed to fetch trading state';
      setError(message);
      console.error('Failed to fetch trading state:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    // Initial fetch
    fetchState();

    // Poll at specified interval
    if (pollInterval > 0) {
      intervalRef.current = window.setInterval(fetchState, pollInterval);
    }

    return () => {
      if (intervalRef.current) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchState, pollInterval]);

  const refetch = useCallback(() => {
    setIsLoading(true);
    fetchState();
  }, [fetchState]);

  return { data, isLoading, error, refetch };
}

/**
 * Hook for enabling trading
 */
export function useEnableTrading() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TradingState | null>(null);

  const mutate = useCallback(async (reason?: string) => {
    setIsLoading(true);
    setError(null);
    setData(null);

    try {
      const response = await tradingClient.post<TradingState>('/enable', { reason });
      setData(response.data);
      return response.data;
    } catch (err: unknown) {
      const message =
        axios.isAxiosError(err) && err.response?.data?.message
          ? err.response.data.message
          : err instanceof Error
            ? err.message
            : 'Failed to enable trading';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const reset = useCallback(() => {
    setIsLoading(false);
    setError(null);
    setData(null);
  }, []);

  return { mutate, isLoading, error, data, reset };
}

/**
 * Hook for disabling trading
 */
export function useDisableTrading() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TradingState | null>(null);

  const mutate = useCallback(async (reason?: string) => {
    setIsLoading(true);
    setError(null);
    setData(null);

    try {
      const response = await tradingClient.post<TradingState>('/disable', { reason });
      setData(response.data);
      return response.data;
    } catch (err: unknown) {
      const message =
        axios.isAxiosError(err) && err.response?.data?.message
          ? err.response.data.message
          : err instanceof Error
            ? err.message
            : 'Failed to disable trading';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const reset = useCallback(() => {
    setIsLoading(false);
    setError(null);
    setData(null);
  }, []);

  return { mutate, isLoading, error, data, reset };
}

/**
 * Hook for setting trading state (PUT)
 */
export function useSetTradingState() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TradingState | null>(null);

  const mutate = useCallback(async (enabled: boolean, reason?: string) => {
    setIsLoading(true);
    setError(null);
    setData(null);

    try {
      const response = await tradingClient.put<TradingState>('/state', { enabled, reason });
      setData(response.data);
      return response.data;
    } catch (err: unknown) {
      const message =
        axios.isAxiosError(err) && err.response?.data?.message
          ? err.response.data.message
          : err instanceof Error
            ? err.message
            : 'Failed to set trading state';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const reset = useCallback(() => {
    setIsLoading(false);
    setError(null);
    setData(null);
  }, []);

  return { mutate, isLoading, error, data, reset };
}
