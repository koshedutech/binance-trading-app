import { useState, useEffect, useCallback, useRef } from 'react';
import { wsService } from '../services/websocket';
import type { WSEvent } from '../types';

interface WebSocketDataOptions<T> {
  /** WebSocket event type to subscribe to (e.g., 'POSITION_UPDATE', 'TRADE_UPDATE') */
  messageType: string;
  /** Optional REST API fallback function */
  fallbackFetch?: () => Promise<T>;
  /** Fallback polling interval in ms (default 30000) */
  fallbackInterval?: number;
  /** Transform WebSocket event data before setting state */
  transform?: (event: WSEvent) => T;
  /** Initial data */
  initialData?: T;
}

interface WebSocketDataResult<T> {
  /** Current data from WebSocket or REST fallback */
  data: T | null;
  /** Whether WebSocket is connected */
  isConnected: boolean;
  /** Whether data came from WebSocket (real-time) vs REST (polling) */
  isRealTime: boolean;
  /** Timestamp of last data update */
  lastUpdate: Date | null;
  /** Manual refresh trigger (calls fallbackFetch) */
  refresh: () => Promise<void>;
  /** Any error from fallback fetch */
  error: string | null;
  /** Loading state for initial fetch */
  isLoading: boolean;
}

/**
 * Hook to subscribe to WebSocket events with REST API fallback.
 * Primary data source is WebSocket; REST polling is used as fallback
 * when WebSocket is unavailable or for initial data.
 */
export function useWebSocketData<T>(options: WebSocketDataOptions<T>): WebSocketDataResult<T> {
  const {
    messageType,
    fallbackFetch,
    fallbackInterval = 30000,
    transform,
    initialData = null,
  } = options;

  const [data, setData] = useState<T | null>(initialData);
  const [isConnected, setIsConnected] = useState(false);
  const [isRealTime, setIsRealTime] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(!!fallbackFetch);

  // Track if we've received WebSocket data recently
  const lastWsUpdateRef = useRef<number>(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);

  // Fetch data from REST API
  const fetchData = useCallback(async () => {
    if (!fallbackFetch) return;

    try {
      const result = await fallbackFetch();

      // Only update if we haven't received WebSocket data recently (within 5 seconds)
      const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
      if (timeSinceWsUpdate > 5000) {
        setData(result);
        setLastUpdate(new Date());
        setIsRealTime(false);
      }
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch data';
      setError(message);
      console.error(`Fallback fetch failed for ${messageType}:`, err);
    } finally {
      setIsLoading(false);
    }
  }, [fallbackFetch, messageType]);

  // Manual refresh function
  const refresh = useCallback(async () => {
    if (fallbackFetch) {
      setIsLoading(true);
      // Force update even if we have recent WS data
      lastWsUpdateRef.current = 0;
      await fetchData();
    }
  }, [fallbackFetch, fetchData]);

  useEffect(() => {
    // WebSocket message handler
    const handleMessage = (event: WSEvent) => {
      const transformedData = transform ? transform(event) : (event.data as unknown as T);
      setData(transformedData);
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
    wsService.subscribe(messageType, handleMessage);
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
        // Only poll if we haven't received WebSocket data recently
        const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
        if (timeSinceWsUpdate > 5000) {
          fetchData();
        }
      }, fallbackInterval);
    }

    // Cleanup
    return () => {
      wsService.unsubscribe(messageType, handleMessage);
      // Note: Cannot unsubscribe from onConnect/onDisconnect as wsService
      // doesn't provide that API - they persist until reset()

      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [messageType, fallbackFetch, fallbackInterval, transform, fetchData]);

  return {
    data,
    isConnected,
    isRealTime,
    lastUpdate,
    refresh,
    error,
    isLoading,
  };
}

/**
 * Hook to subscribe to multiple WebSocket event types.
 * Useful when data comes from different event sources.
 */
export function useMultiWebSocketData<T>(options: {
  messageTypes: string[];
  fallbackFetch?: () => Promise<T>;
  fallbackInterval?: number;
  transform?: (event: WSEvent) => T;
  merge?: (current: T | null, event: WSEvent) => T;
  initialData?: T;
}): WebSocketDataResult<T> {
  const {
    messageTypes,
    fallbackFetch,
    fallbackInterval = 30000,
    transform,
    merge,
    initialData = null,
  } = options;

  const [data, setData] = useState<T | null>(initialData);
  const [isConnected, setIsConnected] = useState(false);
  const [isRealTime, setIsRealTime] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(!!fallbackFetch);

  const lastWsUpdateRef = useRef<number>(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);
  const dataRef = useRef<T | null>(initialData);

  const fetchData = useCallback(async () => {
    if (!fallbackFetch) return;

    try {
      const result = await fallbackFetch();
      const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
      if (timeSinceWsUpdate > 5000) {
        setData(result);
        dataRef.current = result;
        setLastUpdate(new Date());
        setIsRealTime(false);
      }
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch data';
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

  useEffect(() => {
    const handleMessage = (event: WSEvent) => {
      let newData: T;
      if (merge) {
        newData = merge(dataRef.current, event);
      } else if (transform) {
        newData = transform(event);
      } else {
        newData = event.data as unknown as T;
      }
      setData(newData);
      dataRef.current = newData;
      setLastUpdate(new Date());
      setIsRealTime(true);
      setIsConnected(true);
      lastWsUpdateRef.current = Date.now();
    };

    const handleConnect = () => setIsConnected(true);
    const handleDisconnect = () => {
      setIsConnected(false);
      setIsRealTime(false);
    };

    // Subscribe to all message types
    messageTypes.forEach(type => {
      wsService.subscribe(type, handleMessage);
    });
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

    return () => {
      messageTypes.forEach(type => {
        wsService.unsubscribe(type, handleMessage);
      });
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [messageTypes.join(','), fallbackFetch, fallbackInterval, transform, merge, fetchData]);

  return {
    data,
    isConnected,
    isRealTime,
    lastUpdate,
    refresh,
    error,
    isLoading,
  };
}
