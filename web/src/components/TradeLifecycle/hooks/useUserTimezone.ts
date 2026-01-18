// Story 7.19: User Timezone Hook
// Provides timezone formatting for Trade Lifecycle timestamps
// Uses native Intl.DateTimeFormat for timezone conversion (no external dependency)
import { useState, useEffect, useCallback, useMemo } from 'react';
import { apiService } from '../../../services/api';

interface UseUserTimezoneReturn {
  timezone: string;
  loading: boolean;
  error: string | null;
  formatTime: (isoString: string | number) => string;
  formatDateTime: (isoString: string | number) => string;
  formatFullDateTime: (isoString: string | number) => string;
}

/**
 * Hook to fetch user's timezone setting and provide formatting utilities
 * Uses native Intl.DateTimeFormat for timezone conversions (no extra dependencies)
 */
export function useUserTimezone(): UseUserTimezoneReturn {
  const [timezone, setTimezone] = useState<string>('Asia/Kolkata'); // Default
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch user timezone on mount
  useEffect(() => {
    const fetchTimezone = async () => {
      try {
        setLoading(true);
        const response = await apiService.getUserTimezone();
        // Story 7.19 Fix: Check response.success before using timezone
        if (response.success === true && response.timezone) {
          setTimezone(response.timezone);
        }
        // Note: If success is false or timezone is empty, we keep the default 'Asia/Kolkata'
        // This is intentional fallback behavior (Issue #6 documentation)
        setError(null);
      } catch (err) {
        console.error('Failed to fetch user timezone:', err);
        setError('Failed to fetch timezone settings');
        // Keep default timezone on error
      } finally {
        setLoading(false);
      }
    };

    fetchTimezone();
  }, []);

  // Create formatters using useMemo for performance
  const timeFormatter = useMemo(() => {
    return new Intl.DateTimeFormat('en-GB', {
      timeZone: timezone,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }, [timezone]);

  const dateTimeFormatter = useMemo(() => {
    return new Intl.DateTimeFormat('en-GB', {
      timeZone: timezone,
      day: '2-digit',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }, [timezone]);

  const fullDateTimeFormatter = useMemo(() => {
    return new Intl.DateTimeFormat('en-GB', {
      timeZone: timezone,
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    });
  }, [timezone]);

  /**
   * Format ISO timestamp or epoch to time only (HH:mm:ss) in user's timezone
   * Used for modification history inline timestamps
   */
  const formatTime = useCallback((input: string | number): string => {
    if (!input) return '--:--:--';
    try {
      const date = typeof input === 'number' ? new Date(input) : new Date(input);
      if (isNaN(date.getTime())) return '--:--:--';
      return timeFormatter.format(date);
    } catch (err) {
      console.error('Failed to format time:', err);
      return '--:--:--';
    }
  }, [timeFormatter]);

  /**
   * Format ISO timestamp or epoch to short date-time (dd-MMM HH:mm:ss) in user's timezone
   * Used for chain headers and summaries
   */
  const formatDateTime = useCallback((input: string | number): string => {
    if (!input) return '--';
    try {
      const date = typeof input === 'number' ? new Date(input) : new Date(input);
      if (isNaN(date.getTime())) return '--';
      return dateTimeFormatter.format(date);
    } catch (err) {
      console.error('Failed to format datetime:', err);
      return '--';
    }
  }, [dateTimeFormatter]);

  /**
   * Format ISO timestamp or epoch to full date-time (dd MMM yyyy, HH:mm:ss) in user's timezone
   * Used for detailed views and tooltips
   */
  const formatFullDateTime = useCallback((input: string | number): string => {
    if (!input) return '--';
    try {
      const date = typeof input === 'number' ? new Date(input) : new Date(input);
      if (isNaN(date.getTime())) return '--';
      return fullDateTimeFormatter.format(date);
    } catch (err) {
      console.error('Failed to format full datetime:', err);
      return '--';
    }
  }, [fullDateTimeFormatter]);

  return {
    timezone,
    loading,
    error,
    formatTime,
    formatDateTime,
    formatFullDateTime,
  };
}

// Re-export for convenience
export default useUserTimezone;
