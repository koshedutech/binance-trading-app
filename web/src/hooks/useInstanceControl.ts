import { useState, useEffect, useCallback, useRef } from 'react';
import { futuresApi } from '../services/futuresApi';

// Types based on API specification from Story 9.6
export interface InstanceStatus {
  instance_id: string;        // "dev" or "prod"
  is_active: boolean;         // This instance's status
  active_instance: string;    // Which instance is active
  other_alive: boolean;       // Is other instance running
  last_heartbeat: string;     // Last heartbeat time (ISO string)
  can_take_control: boolean;  // Can this instance take over
}

export interface TakeControlRequest {
  force?: boolean; // Force takeover even if other not responding
}

export interface TakeControlResponse {
  success: boolean;
  message: string;
  wait_seconds?: number; // Estimated wait time
}

export interface ReleaseControlResponse {
  success: boolean;
  message: string;
}

// Query hook for GET /api/ginie/instance-status (polls every 5 seconds)
export function useInstanceStatus() {
  const [data, setData] = useState<InstanceStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const intervalRef = useRef<number | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const response = await futuresApi.getInstanceStatus();
      setData(response);
      setError(null);
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to fetch instance status';
      setError(message);
      console.error('Failed to fetch instance status:', err);
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

// Mutation hook for POST /api/ginie/take-control
export function useTakeControl() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<TakeControlResponse | null>(null);

  const mutate = useCallback(async (request: TakeControlRequest = {}) => {
    setIsLoading(true);
    setError(null);
    setData(null);

    try {
      const response = await futuresApi.takeControl(request);
      setData(response);
      return response;
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to take control';
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

// Mutation hook for POST /api/ginie/release-control
export function useReleaseControl() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<ReleaseControlResponse | null>(null);

  const mutate = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    setData(null);

    try {
      const response = await futuresApi.releaseControl();
      setData(response);
      return response;
    } catch (err: any) {
      const message = err?.response?.data?.message || err?.message || 'Failed to release control';
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
