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
  position_timeframe?: string; // Entry timeframe from active position's order chain
  updated_at: string;
}

export interface CoinProfilerCoinsResponse {
  coins: CoinData[];
  total: number;
}

export interface StrategyRef {
  mode: string;
  strategy: string;
  sub_strategy: string;
  // Capacity fields (added by backend when available)
  current_positions?: number;
  max_positions?: number;
  capacity_status?: 'available' | 'limited' | 'at_limit';
}

export interface StrategyCapacityInfo {
  mode: string;
  strategy_group: string;
  sub_strategy: string;
  current_positions: number;
  max_positions: number;
  capacity_status: 'available' | 'limited' | 'at_limit';
}

export interface PositionRequirement {
  symbol: string;
  mode: string;
  side: string;
  exit_mode: string;
  timeframes: string[];
}

export interface StrategySource {
  symbol: string;
  timeframes: string[];
  strategies: StrategyRef[];
}

export interface CoinProfilerRequirements {
  all_timeframes: string[];
  all_data_fields: string[];
  all_symbols: string[];
  strategy_count: number;
  position_count: number;
  from_strategies: StrategySource[];
  from_positions: PositionRequirement[];
  subscriptions: Record<string, {
    timeframes: string[];
    data_fields: string[];
    source: string;
    strategies: StrategyRef[];
    positions: PositionRequirement[];
  }>;
  // Capacity summary for all enabled sub-strategies
  capacity_summary?: StrategyCapacityInfo[];
  // Maps mode to its required timeframes (e.g., {"scalp": ["3m"], "swing": ["1h"]})
  timeframes_by_mode?: Record<string, string[]>;
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

/**
 * Hook to fetch Coin Profiler requirements (strategy and position breakdown)
 * Shows what data is being collected and why
 */
export function useCoinProfilerRequirements(autoPoll: boolean = false, pollInterval: number = 10000) {
  const [data, setData] = useState<CoinProfilerRequirements | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  const fetchRequirements = useCallback(async () => {
    try {
      const response = await futuresApi.getCoinProfilerRequirements();
      setData(response);
      setError(null);
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to fetch requirements';
      setError(message);
      console.error('Failed to fetch coin profiler requirements:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    // Initial fetch
    fetchRequirements();

    // Optional polling
    if (autoPoll) {
      intervalRef.current = window.setInterval(fetchRequirements, pollInterval);
    }

    return () => {
      if (intervalRef.current) {
        window.clearInterval(intervalRef.current);
      }
    };
  }, [fetchRequirements, autoPoll, pollInterval]);

  const refetch = useCallback(() => {
    setIsLoading(true);
    fetchRequirements();
  }, [fetchRequirements]);

  return { data, isLoading, error, refetch };
}

/**
 * Real-time coin data update from WebSocket
 */
export interface CoinDataUpdate {
  symbol: string;
  price: number;
  timeframe: string;
  data: {
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
  };
}

/**
 * Position created event from WebSocket
 * Broadcast when a new position is opened (entry order filled)
 */
export interface PositionCreatedEvent {
  chain_id: string;
  symbol: string;
  side: 'LONG' | 'SHORT';
  entry_price: number;
  quantity: number;
  mode: string;
  mode_code: string;
  strategy_group: string;
  sub_strategy: string;
  timeframe: string;
  order_id: number;
  created_at: string;
}

/**
 * Hook that maintains real-time coin data using WebSocket updates.
 * Receives COIN_DATA_UPDATE events and updates local state immediately.
 * Falls back to API polling if WebSocket is not connected.
 */
export function useCoinProfilerRealtime(wsConnected: boolean = false) {
  const [coins, setCoins] = useState<Map<string, CoinData>>(new Map());
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [updateCount, setUpdateCount] = useState(0);

  // Initial load from API
  const { data: initialData, refetch } = useCoinProfilerCoins(false);

  // Initialize from API data
  useEffect(() => {
    if (initialData?.coins) {
      const coinMap = new Map<string, CoinData>();
      initialData.coins.forEach(coin => coinMap.set(coin.symbol, coin));
      setCoins(coinMap);
    }
  }, [initialData]);

  // Handle real-time WebSocket updates
  const handleCoinUpdate = useCallback((update: CoinDataUpdate) => {
    setCoins(prevCoins => {
      const newCoins = new Map(prevCoins);
      const existing = newCoins.get(update.symbol);

      // Create or update coin data
      const updatedCoin: CoinData = existing ? {
        ...existing,
        price: update.price,
        timeframes: {
          ...existing.timeframes,
          [update.timeframe]: {
            timeframe: update.timeframe,
            open: update.data.open,
            high: update.data.high,
            low: update.data.low,
            close: update.data.close,
            volume: update.data.volume,
            taker_buy_vol: update.data.taker_buy_vol,
            taker_sell_vol: update.data.taker_sell_vol,
            quote_volume: update.data.quote_volume,
            trade_count: update.data.trade_count,
            is_closed_bar: update.data.is_closed_bar,
            open_time: update.data.open_time,
            close_time: update.data.close_time,
            updated_at: update.data.updated_at,
          }
        },
        updated_at: new Date().toISOString(),
      } : {
        symbol: update.symbol,
        price: update.price,
        volume_24h: 0,
        volatility: 0,
        timeframes: {
          [update.timeframe]: {
            timeframe: update.timeframe,
            open: update.data.open,
            high: update.data.high,
            low: update.data.low,
            close: update.data.close,
            volume: update.data.volume,
            taker_buy_vol: update.data.taker_buy_vol,
            taker_sell_vol: update.data.taker_sell_vol,
            quote_volume: update.data.quote_volume,
            trade_count: update.data.trade_count,
            is_closed_bar: update.data.is_closed_bar,
            open_time: update.data.open_time,
            close_time: update.data.close_time,
            updated_at: update.data.updated_at,
          }
        },
        source: 'strategy',
        strategies: [],
        updated_at: new Date().toISOString(),
      };

      newCoins.set(update.symbol, updatedCoin);
      return newCoins;
    });

    setLastUpdate(new Date());
    setUpdateCount(prev => prev + 1);
  }, []);

  // Optimistically update a coin's source (e.g., when a position is created/closed)
  const updateCoinSource = useCallback((symbol: string, source: 'strategy' | 'position' | 'both', timeframe?: string) => {
    setCoins(prevCoins => {
      const newCoins = new Map(prevCoins);
      const existing = newCoins.get(symbol);
      if (existing) {
        const updatedCoin: CoinData = {
          ...existing,
          source,
          updated_at: new Date().toISOString(),
        };
        // If a timeframe is provided and not already tracked, add a placeholder
        if (timeframe && !existing.timeframes[timeframe]) {
          updatedCoin.timeframes = {
            ...existing.timeframes,
            [timeframe]: {
              timeframe,
              open: 0, high: 0, low: 0, close: 0,
              volume: 0, taker_buy_vol: 0, taker_sell_vol: 0,
              quote_volume: 0, trade_count: 0,
              is_closed_bar: false, open_time: '', close_time: '',
              updated_at: new Date().toISOString(),
            },
          };
        }
        newCoins.set(symbol, updatedCoin);
      } else {
        // Coin not tracked yet - add it with position source
        newCoins.set(symbol, {
          symbol,
          price: 0,
          volume_24h: 0,
          volatility: 0,
          timeframes: timeframe ? {
            [timeframe]: {
              timeframe,
              open: 0, high: 0, low: 0, close: 0,
              volume: 0, taker_buy_vol: 0, taker_sell_vol: 0,
              quote_volume: 0, trade_count: 0,
              is_closed_bar: false, open_time: '', close_time: '',
              updated_at: new Date().toISOString(),
            },
          } : {},
          source,
          strategies: [],
          updated_at: new Date().toISOString(),
        });
      }
      return newCoins;
    });
  }, []);

  // Convert Map to array for component consumption
  const coinsArray = Array.from(coins.values());

  return {
    coins: coinsArray,
    coinsMap: coins,
    total: coins.size,
    lastUpdate,
    updateCount,
    handleCoinUpdate,
    updateCoinSource,
    refetch,
  };
}
