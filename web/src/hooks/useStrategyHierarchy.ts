// Strategy Hierarchy Hooks
// Epic 11: Position Decision Engine - Story 11.46: Volume Imbalance UI Components

import { useState, useEffect, useCallback, useRef } from 'react';
import { apiService } from '../services/api';
import { wsService } from '../services/websocket';
import type { WSEvent } from '../types';
import type {
  StrategyGroupSettings,
  SubStrategySettings,
  VolumeImbalancePattern,
  GetStrategyGroupsResponse,
  GetSubStrategiesResponse,
  GetVolumeImbalancePatternsResponse,
  UpdateStrategyGroupRequest,
  UpdateSubStrategyRequest,
  UpdateResponse,
} from '../types/strategyHierarchy';

// ==================== Hook Interfaces ====================

interface UseStrategyGroupsResult {
  groups: StrategyGroupSettings[];
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

interface UseSubStrategiesResult {
  subStrategies: SubStrategySettings[];
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

interface UseVolumeImbalancePatternsResult {
  patterns: VolumeImbalancePattern[];
  stats: {
    watching: number;
    consolidating: number;
    ready: number;
    expired_today: number;
  };
  isLoading: boolean;
  isRealTime: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

interface UseMutationResult<T> {
  mutate: (data: T) => Promise<UpdateResponse>;
  isLoading: boolean;
  error: string | null;
  reset: () => void;
}

// ==================== Strategy Groups Hook ====================

/**
 * Hook to fetch strategy groups for a trading mode
 */
export function useStrategyGroups(mode: string): UseStrategyGroupsResult {
  const [groups, setGroups] = useState<StrategyGroupSettings[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchGroups = useCallback(async () => {
    if (!mode) return;

    setIsLoading(true);
    setError(null);

    try {
      const response = await apiService.get<GetStrategyGroupsResponse>(
        `/futures/strategy-groups/${mode}`
      );
      setGroups(response.data.groups || []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch strategy groups';
      setError(message);
      console.error('Failed to fetch strategy groups:', err);
    } finally {
      setIsLoading(false);
    }
  }, [mode]);

  useEffect(() => {
    fetchGroups();
  }, [fetchGroups]);

  return {
    groups,
    isLoading,
    error,
    refresh: fetchGroups,
  };
}

// ==================== Sub-Strategies Hook ====================

/**
 * Hook to fetch sub-strategies for a strategy group
 */
export function useSubStrategies(mode: string, group: string): UseSubStrategiesResult {
  const [subStrategies, setSubStrategies] = useState<SubStrategySettings[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchSubStrategies = useCallback(async () => {
    if (!mode || !group) return;

    setIsLoading(true);
    setError(null);

    try {
      const response = await apiService.get<GetSubStrategiesResponse>(
        `/futures/sub-strategies/${mode}/${group}`
      );
      setSubStrategies(response.data.sub_strategies || []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch sub-strategies';
      setError(message);
      console.error('Failed to fetch sub-strategies:', err);
    } finally {
      setIsLoading(false);
    }
  }, [mode, group]);

  useEffect(() => {
    fetchSubStrategies();
  }, [fetchSubStrategies]);

  return {
    subStrategies,
    isLoading,
    error,
    refresh: fetchSubStrategies,
  };
}

// ==================== Type Guards ====================

/**
 * Type guard to validate WebSocket message structure
 */
function isVolumeImbalanceResponse(data: unknown): data is GetVolumeImbalancePatternsResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'patterns' in data &&
    Array.isArray((data as GetVolumeImbalancePatternsResponse).patterns)
  );
}

// ==================== Volume Imbalance Patterns Hook ====================

/**
 * Hook to fetch and subscribe to Volume Imbalance patterns
 * Uses WebSocket for real-time updates with REST fallback
 */
export function useVolumeImbalancePatterns(): UseVolumeImbalancePatternsResult {
  const [patterns, setPatterns] = useState<VolumeImbalancePattern[]>([]);
  const [stats, setStats] = useState({
    watching: 0,
    consolidating: 0,
    ready: 0,
    expired_today: 0,
  });
  const [isLoading, setIsLoading] = useState(true);
  const [isRealTime, setIsRealTime] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const lastWsUpdateRef = useRef<number>(0);
  const fallbackTimerRef = useRef<NodeJS.Timeout | null>(null);

  const fetchPatterns = useCallback(async () => {
    try {
      const response = await apiService.get<GetVolumeImbalancePatternsResponse>(
        '/futures/patterns/volume-imbalance'
      );

      // Only update if we haven't received WebSocket data recently
      const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
      if (timeSinceWsUpdate > 5000) {
        setPatterns(response.data.patterns || []);
        setStats(response.data.stats || stats);
        setIsRealTime(false);
      }
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch patterns';
      setError(message);
      console.error('Failed to fetch volume imbalance patterns:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const refresh = useCallback(async () => {
    setIsLoading(true);
    lastWsUpdateRef.current = 0; // Force update
    await fetchPatterns();
  }, [fetchPatterns]);

  useEffect(() => {
    // WebSocket message handler
    const handlePatternUpdate = (event: WSEvent) => {
      if (isVolumeImbalanceResponse(event.data)) {
        setPatterns(event.data.patterns);
        if (event.data.stats) {
          setStats(event.data.stats);
        }
        setIsRealTime(true);
        lastWsUpdateRef.current = Date.now();
      } else {
        console.warn('Received invalid volume imbalance WebSocket message:', event.data);
      }
    };

    // Subscribe to WebSocket events
    wsService.subscribe('VOLUME_IMBALANCE_UPDATE', handlePatternUpdate);

    // Initial fetch
    fetchPatterns();

    // Set up REST polling as fallback (every 30 seconds)
    fallbackTimerRef.current = setInterval(() => {
      const timeSinceWsUpdate = Date.now() - lastWsUpdateRef.current;
      if (timeSinceWsUpdate > 5000) {
        fetchPatterns();
      }
    }, 30000);

    return () => {
      wsService.unsubscribe('VOLUME_IMBALANCE_UPDATE', handlePatternUpdate);
      if (fallbackTimerRef.current) {
        clearInterval(fallbackTimerRef.current);
        fallbackTimerRef.current = null;
      }
    };
  }, [fetchPatterns]);

  return {
    patterns,
    stats,
    isLoading,
    isRealTime,
    error,
    refresh,
  };
}

// ==================== Update Strategy Group Hook ====================

/**
 * Request type for updating strategy group with mode and group in the call
 */
interface UpdateStrategyGroupMutationRequest {
  mode: string;
  group: string;
  settings: Partial<UpdateStrategyGroupRequest>;
}

/**
 * Hook to update a strategy group (dynamic mode/group in mutation call)
 */
export function useUpdateStrategyGroup(): {
  mutateAsync: (data: UpdateStrategyGroupMutationRequest) => Promise<UpdateResponse>;
  isLoading: boolean;
  error: string | null;
  reset: () => void;
} {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mutateAsync = useCallback(
    async (data: UpdateStrategyGroupMutationRequest): Promise<UpdateResponse> => {
      setIsLoading(true);
      setError(null);

      try {
        const response = await apiService.put<UpdateResponse>(
          `/futures/strategy-groups/${data.mode}/${data.group}`,
          data.settings
        );
        return response.data;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to update strategy group';
        setError(message);
        throw err;
      } finally {
        setIsLoading(false);
      }
    },
    []
  );

  const reset = useCallback(() => {
    setError(null);
  }, []);

  return {
    mutateAsync,
    isLoading,
    error,
    reset,
  };
}

// ==================== Update Sub-Strategy Hook ====================

/**
 * Hook to update a sub-strategy
 */
export function useUpdateSubStrategy(
  mode: string,
  groupId: string,
  subStrategyId: string
): UseMutationResult<UpdateSubStrategyRequest> {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const mutate = useCallback(
    async (data: UpdateSubStrategyRequest): Promise<UpdateResponse> => {
      setIsLoading(true);
      setError(null);

      try {
        const response = await apiService.put<UpdateResponse>(
          `/futures/sub-strategies/${mode}/${groupId}/${subStrategyId}`,
          data
        );
        return response.data;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to update sub-strategy';
        setError(message);
        throw err;
      } finally {
        setIsLoading(false);
      }
    },
    [mode, groupId, subStrategyId]
  );

  const reset = useCallback(() => {
    setError(null);
  }, []);

  return {
    mutate,
    isLoading,
    error,
    reset,
  };
}

// ==================== Execute Pattern Hook ====================

/**
 * Hook to execute or skip a Volume Imbalance pattern
 */
export function usePatternAction(): {
  execute: (patternId: string) => Promise<UpdateResponse>;
  skip: (patternId: string, reason?: string) => Promise<UpdateResponse>;
  isLoading: boolean;
  error: string | null;
} {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const execute = useCallback(async (patternId: string): Promise<UpdateResponse> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await apiService.post<UpdateResponse>(
        `/futures/patterns/volume-imbalance/${patternId}/execute`
      );
      return response.data;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to execute pattern';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const skip = useCallback(async (patternId: string, reason?: string): Promise<UpdateResponse> => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await apiService.post<UpdateResponse>(
        `/futures/patterns/volume-imbalance/${patternId}/skip`,
        { reason }
      );
      return response.data;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to skip pattern';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  return {
    execute,
    skip,
    isLoading,
    error,
  };
}

// ==================== Combined Hook for Full Data ====================

/**
 * Combined hook for strategy hierarchy with all data
 */
export function useStrategyHierarchy(mode: string) {
  const groupsResult = useStrategyGroups(mode);
  const patternsResult = useVolumeImbalancePatterns();

  return {
    groups: groupsResult,
    patterns: patternsResult,
    isLoading: groupsResult.isLoading || patternsResult.isLoading,
    hasError: !!groupsResult.error || !!patternsResult.error,
  };
}
