import { useEffect, useState, useCallback, useRef, useMemo } from 'react';
import { formatUSD, futuresApi } from '../services/futuresApi';
import { wsService } from '../services/websocket';
import CollapsibleCard from './CollapsibleCard';
import { DateRangeFilter, NavigationControls, DayDetailPanel } from './PnLSummary';
import type { DateRangeType } from './PnLSummary';
import type { PnLPayload, WSEvent } from '../types';
import {
  TrendingUp,
  Calendar,
  DollarSign,
  Activity,
  Wifi,
  Database,
  RefreshCw,
} from 'lucide-react';

interface DailyPnLBreakdown {
  date: string;
  day: number;
  day_name: string;
  pnl: number;
  commission: number;
  rebate?: number;
  funding: number;
  other?: number;
  net_pnl: number;
  trade_count: number;
  is_profit: boolean;
  is_today: boolean;
  is_cached?: boolean;
}

interface PnLSummaryData {
  // Daily breakdown
  daily_pnl: number;
  daily_commission: number;
  daily_funding: number;
  daily_trade_count: number;
  reset_countdown: string;
  seconds_to_reset: number;
  // Weekly breakdown
  weekly_pnl: number;
  weekly_commission: number;
  weekly_funding: number;
  weekly_trade_count: number;
  week_start_date: string;
  week_end_date: string;
  week_range: string;
  // 7-day calendar breakdown
  daily_breakdown: DailyPnLBreakdown[];
  // Timezone info
  timezone: string;
  timezone_offset: string;
  fetched_at: string;
}

export default function PnLSummaryCard() {
  const [pnlData, setPnlData] = useState<PnLSummaryData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [countdown, setCountdown] = useState<string>('');
  const [wsConnected, setWsConnected] = useState(() => wsService.isConnected());

  // Story 13.1: Extended navigation state
  const [selectedDay, setSelectedDay] = useState<DailyPnLBreakdown | null>(null);
  const [dateRange, setDateRange] = useState<DateRangeType>('week');
  const [customStartDate, setCustomStartDate] = useState<string>('');
  const [customEndDate, setCustomEndDate] = useState<string>('');
  const [extendedData, setExtendedData] = useState<DailyPnLBreakdown[] | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [isLoadingHistory, setIsLoadingHistory] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const DAYS_PER_PAGE = 7;

  const fetchPnlSummary = useCallback(async () => {
    try {
      const response = await futuresApi.getPnLSummary();
      setPnlData(response);
    } catch (err: any) {
      console.error('Failed to fetch PnL summary:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Story 13.1: Fetch extended history
  const fetchPnLHistory = useCallback(async (startDate?: string, endDate?: string) => {
    setIsLoadingHistory(true);
    setHistoryError(null);
    try {
      const response = await futuresApi.getPnLHistory(startDate, endDate);
      setExtendedData(response.daily_records);
    } catch (err: any) {
      console.error('Failed to fetch PnL history:', err);
      const message = err?.response?.data?.error || err?.message || 'Failed to load history';
      setHistoryError(message);
      setExtendedData(null);
    } finally {
      setIsLoadingHistory(false);
    }
  }, []);

  // Handle date range change
  const handleRangeChange = useCallback((range: DateRangeType, startDate?: string, endDate?: string) => {
    setDateRange(range);
    setCurrentPage(1);

    if (range === 'week') {
      // Use default 7-day data from pnlData
      setExtendedData(null);
    } else {
      // Calculate date range and fetch
      const today = new Date();
      let start: Date;
      let end: Date = today;

      switch (range) {
        case 'day':
          start = today;
          break;
        case 'month':
          start = new Date(today);
          start.setDate(start.getDate() - 29);
          break;
        case 'year':
          start = new Date(today);
          start.setFullYear(start.getFullYear() - 1);
          break;
        case 'custom':
          if (startDate && endDate) {
            setCustomStartDate(startDate);
            setCustomEndDate(endDate);
            fetchPnLHistory(startDate, endDate);
            return;
          }
          return;
        default:
          start = new Date(today);
          start.setDate(start.getDate() - 6);
      }

      const startStr = start.toISOString().split('T')[0];
      const endStr = end.toISOString().split('T')[0];
      fetchPnLHistory(startStr, endStr);
    }
  }, [fetchPnLHistory]);

  // Ref for fallback interval
  const fallbackRef = useRef<NodeJS.Timeout | null>(null);

  // WebSocket subscription for real-time updates
  useEffect(() => {
    const handlePnLUpdate = (event: WSEvent) => {
      const pnl = event.data.pnl as PnLPayload;
      if (pnl) {
        fetchPnlSummary();
      }
    };

    wsService.subscribe('PNL_UPDATE', handlePnLUpdate);

    // Also listen to CHAIN_LIFECYCLE_UPDATE (fires on every position close)
    const handleChainLifecycleUpdate = () => {
      fetchPnlSummary();
    };
    wsService.subscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);

    // Listen to PNL_CORRECTED (fires when real PnL arrives from Binance sub-orders)
    const handlePnLCorrected = () => {
      fetchPnlSummary();
    };
    wsService.subscribe('PNL_CORRECTED', handlePnLCorrected);

    // Fallback polling when WebSocket disconnected
    const startFallback = () => {
      if (!fallbackRef.current) {
        fallbackRef.current = setInterval(() => {
          fetchPnlSummary();
        }, 60000);
      }
    };

    const stopFallback = () => {
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
      fetchPnlSummary();
    };

    wsService.onDisconnect(startFallback);
    wsService.onConnect(stopFallback);

    if (!wsService.isConnected()) {
      startFallback();
    }

    // Initial fetch
    fetchPnlSummary();

    return () => {
      wsService.unsubscribe('PNL_UPDATE', handlePnLUpdate);
      wsService.unsubscribe('CHAIN_LIFECYCLE_UPDATE', handleChainLifecycleUpdate);
      wsService.unsubscribe('PNL_CORRECTED', handlePnLCorrected);
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
    };
  }, [fetchPnlSummary]);

  // Track WebSocket connection status
  useEffect(() => {
    const handleConnect = () => setWsConnected(true);
    const handleDisconnect = () => setWsConnected(false);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);
    setWsConnected(wsService.isConnected());
  }, []);

  // Current time in user's timezone (updates every second)
  const [currentTime, setCurrentTime] = useState<string>('');

  useEffect(() => {
    const offset = pnlData?.timezone_offset || '+00:00';

    const updateCurrentTime = () => {
      // Parse offset like "+07:00" or "-05:30"
      const match = offset.match(/([+-])(\d{2}):(\d{2})/);
      if (!match) {
        setCurrentTime(`-- GMT${offset}`);
        return;
      }

      const sign = match[1] === '+' ? 1 : -1;
      const offsetHours = parseInt(match[2], 10);
      const offsetMinutes = parseInt(match[3], 10);
      const totalOffsetMs = sign * (offsetHours * 60 + offsetMinutes) * 60 * 1000;

      // Get current UTC time and apply offset
      const now = new Date();
      const utcMs = now.getTime() + now.getTimezoneOffset() * 60 * 1000;
      const userTime = new Date(utcMs + totalOffsetMs);

      // Format as "6:27 PM"
      const hours = userTime.getHours();
      const minutes = userTime.getMinutes();
      const seconds = userTime.getSeconds();
      const ampm = hours >= 12 ? 'PM' : 'AM';
      const displayHours = hours % 12 || 12;
      const displayMinutes = minutes.toString().padStart(2, '0');
      const displaySeconds = seconds.toString().padStart(2, '0');

      setCurrentTime(`${displayHours}:${displayMinutes}:${displaySeconds} ${ampm} GMT${offset}`);
    };

    updateCurrentTime();
    const interval = setInterval(updateCurrentTime, 1000);
    return () => clearInterval(interval);
  }, [pnlData?.timezone_offset]);

  const getResetTimeFormatted = useCallback(() => {
    return currentTime;
  }, [currentTime]);

  const resetTimeFormatted = getResetTimeFormatted();

  // Countdown timer in "X Hours and Y Minutes" format
  useEffect(() => {
    if (!pnlData?.seconds_to_reset) return;

    let remainingSeconds = pnlData.seconds_to_reset;

    const updateCountdown = () => {
      const hours = Math.floor(remainingSeconds / 3600);
      const minutes = Math.floor((remainingSeconds % 3600) / 60);
      setCountdown(`${hours} Hours and ${minutes} Minutes`);
      remainingSeconds--;
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);

    return () => clearInterval(interval);
  }, [pnlData?.seconds_to_reset]);

  // Calculate P&L values
  const dailyGrossPnl = pnlData?.daily_pnl ?? 0;
  const weeklyGrossPnl = pnlData?.weekly_pnl ?? 0;
  const dailyTotalFees = pnlData ? pnlData.daily_commission + pnlData.daily_funding : 0;
  const weeklyTotalFees = pnlData ? pnlData.weekly_commission + pnlData.weekly_funding : 0;
  const dailyNetPnl = dailyGrossPnl - dailyTotalFees;
  const weeklyNetPnl = weeklyGrossPnl - weeklyTotalFees;

  // Get display data based on range selection
  const displayData = useMemo(() => {
    if (extendedData && dateRange !== 'week') {
      return extendedData;
    }
    return pnlData?.daily_breakdown || [];
  }, [extendedData, dateRange, pnlData?.daily_breakdown]);

  // Paginate display data
  const paginatedData = useMemo(() => {
    const startIndex = (currentPage - 1) * DAYS_PER_PAGE;
    return displayData.slice(startIndex, startIndex + DAYS_PER_PAGE);
  }, [displayData, currentPage]);

  const totalPages = Math.ceil(displayData.length / DAYS_PER_PAGE);

  // Page label
  const pageLabel = useMemo(() => {
    if (paginatedData.length === 0) return 'No data';
    const first = paginatedData[0];
    const last = paginatedData[paginatedData.length - 1];
    return `${last.date.slice(5)} - ${first.date.slice(5)}`;
  }, [paginatedData]);

  return (
    <CollapsibleCard
      title="P&L Summary"
      icon={<TrendingUp className="w-4 h-4" />}
      defaultExpanded={false}
      badge="Daily & Weekly"
      badgeColor="green"
    >
      {isLoading ? (
        <div className="grid grid-cols-2 gap-4">
          <div className="bg-gray-800 rounded-lg p-4 border border-gray-700 animate-pulse">
            <div className="h-6 bg-gray-700 rounded w-32 mb-3"></div>
            <div className="h-10 bg-gray-700 rounded w-24 mb-2"></div>
          </div>
          <div className="bg-gray-800 rounded-lg p-4 border border-gray-700 animate-pulse">
            <div className="h-6 bg-gray-700 rounded w-32 mb-3"></div>
            <div className="h-10 bg-gray-700 rounded w-24 mb-2"></div>
          </div>
        </div>
      ) : pnlData ? (
        <div className="grid grid-cols-2 gap-4">
          {/* Daily P&L Card */}
          <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
            {/* Header */}
            <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 bg-gradient-to-r from-green-900/30 to-gray-800">
              <div className="flex items-center gap-2">
                <DollarSign className="w-4 h-4 text-green-400" />
                <span className="font-semibold text-sm text-white">Daily P&L</span>
              </div>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>

            <div className="p-3">
              {/* Net PnL (BIG) with label */}
              <div className="text-center mb-2">
                <div className={`text-3xl font-extrabold ${dailyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {dailyNetPnl >= 0 ? '+' : ''}{formatUSD(dailyNetPnl)}
                </div>
                <div className="text-[10px] text-gray-500 uppercase">Net P&L</div>
              </div>

              {/* Gross - Fees (Commission + Funding) inline */}
              <div className="flex items-center justify-center flex-wrap gap-1 text-xs text-gray-400 mb-3">
                <span className="text-gray-500">Gross:</span>
                <span className={dailyGrossPnl >= 0 ? 'text-green-400' : 'text-red-400'}>{formatUSD(dailyGrossPnl)}</span>
                <span className="text-gray-600">-</span>
                <span className="text-gray-500">Fees:</span>
                <span className="text-yellow-400">{formatUSD(dailyTotalFees)}</span>
                <span className="text-gray-600 text-[10px]">(Comm:{formatUSD(pnlData.daily_commission)} + Fund:{formatUSD(Math.abs(pnlData.daily_funding))})</span>
              </div>

              {/* Trades & Reset Time */}
              <div className="flex items-center justify-between pt-2 border-t border-gray-700">
                <div className="flex items-center gap-1 text-xs text-gray-400">
                  <Activity className="w-3 h-3" />
                  <span>{pnlData.daily_trade_count} trades</span>
                </div>
                <div className="text-right">
                  <div className="text-[10px] text-blue-400">
                    {resetTimeFormatted}
                  </div>
                  <div className="text-[10px] text-gray-500">
                    Reset in <span className="font-mono font-bold text-blue-400">{countdown}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Weekly P&L Card */}
          <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
            {/* Header */}
            <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 bg-gradient-to-r from-blue-900/30 to-gray-800">
              <div className="flex items-center gap-2">
                <Calendar className="w-4 h-4 text-blue-400" />
                <span className="font-semibold text-sm text-white">Weekly P&L</span>
              </div>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>

            <div className="p-3">
              {/* Net PnL (BIG) with label */}
              <div className="text-center mb-2">
                <div className={`text-3xl font-extrabold ${weeklyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                  {weeklyNetPnl >= 0 ? '+' : ''}{formatUSD(weeklyNetPnl)}
                </div>
                <div className="text-[10px] text-gray-500 uppercase">Net P&L</div>
              </div>

              {/* Gross - Fees (Commission + Funding) inline */}
              <div className="flex items-center justify-center flex-wrap gap-1 text-xs text-gray-400 mb-3">
                <span className="text-gray-500">Gross:</span>
                <span className={weeklyGrossPnl >= 0 ? 'text-green-400' : 'text-red-400'}>{formatUSD(weeklyGrossPnl)}</span>
                <span className="text-gray-600">-</span>
                <span className="text-gray-500">Fees:</span>
                <span className="text-yellow-400">{formatUSD(weeklyTotalFees)}</span>
                <span className="text-gray-600 text-[10px]">(Comm:{formatUSD(pnlData.weekly_commission)} + Fund:{formatUSD(Math.abs(pnlData.weekly_funding))})</span>
              </div>

              {/* Date Range & Trades */}
              <div className="flex items-center justify-between pt-2 border-t border-gray-700">
                <div className="text-xs text-purple-400">
                  {pnlData.week_start_date} - {pnlData.week_end_date}
                </div>
                <div className="flex items-center gap-1 text-xs text-gray-400">
                  <Activity className="w-3 h-3" />
                  <span>{pnlData.weekly_trade_count} trades</span>
                </div>
              </div>
            </div>
          </div>

          {/* P&L Calendar with Navigation */}
          <div className="col-span-2 mt-2">
            <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
              {/* Header with filters and navigation */}
              <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 bg-gradient-to-r from-purple-900/30 to-gray-800">
                <div className="flex items-center gap-2">
                  <Calendar className="w-4 h-4 text-purple-400" />
                  <span className="font-semibold text-sm text-white">P&L Calendar</span>
                  {isLoadingHistory && <RefreshCw className="w-3 h-3 text-blue-400 animate-spin" />}
                </div>
                <div className="flex items-center gap-2">
                  <DateRangeFilter
                    selectedRange={dateRange}
                    onRangeChange={handleRangeChange}
                    customStartDate={customStartDate}
                    customEndDate={customEndDate}
                  />
                  {totalPages > 1 && (
                    <NavigationControls
                      currentPage={currentPage}
                      totalPages={totalPages}
                      pageLabel={pageLabel}
                      onFirst={() => setCurrentPage(1)}
                      onPrev={() => setCurrentPage(p => Math.max(1, p - 1))}
                      onNext={() => setCurrentPage(p => Math.min(totalPages, p + 1))}
                      onLast={() => setCurrentPage(totalPages)}
                    />
                  )}
                </div>
              </div>

              <div className="p-3">
                {/* Flex container for detail panel and calendar grid */}
                <div className="flex gap-3">
                  {/* Detail panel (left side when day selected) */}
                  {selectedDay && (
                    <DayDetailPanel
                      selectedDay={selectedDay}
                      onClose={() => setSelectedDay(null)}
                    />
                  )}

                  {/* Calendar grid */}
                  <div className="flex-1">
                    <div className="grid grid-cols-7 gap-2">
                      {paginatedData.length > 0 ? (
                        paginatedData.map((day) => {
                          const isProfit = day.net_pnl >= 0;
                          const hasActivity = day.trade_count > 0 || day.net_pnl !== 0;
                          const isSelected = selectedDay?.date === day.date;

                          return (
                            <div
                              key={day.date}
                              onClick={() => setSelectedDay(isSelected ? null : day)}
                              className={`
                                relative rounded-lg p-2 text-center transition-all cursor-pointer
                                ${day.is_today ? 'ring-2 ring-blue-500' : ''}
                                ${isSelected ? 'ring-2 ring-purple-500' : ''}
                                ${hasActivity
                                  ? isProfit
                                    ? 'bg-green-900/40 border border-green-700/50 hover:bg-green-900/60'
                                    : 'bg-red-900/40 border border-red-700/50 hover:bg-red-900/60'
                                  : 'bg-gray-700/30 border border-gray-600/30 hover:bg-gray-700/50'
                                }
                              `}
                            >
                              {/* TODAY label or Day name */}
                              <div className={`text-[10px] font-medium uppercase ${day.is_today ? 'text-blue-400' : 'text-gray-400'}`}>
                                {day.is_today ? 'TODAY' : day.day_name}
                              </div>

                              {/* Day number (prominent) */}
                              <div className={`text-2xl font-bold ${day.is_today ? 'text-blue-300' : 'text-white'}`}>
                                {day.day}
                              </div>

                              {/* Net P&L */}
                              <div className={`text-xs font-semibold ${
                                hasActivity
                                  ? isProfit ? 'text-green-400' : 'text-red-400'
                                  : 'text-gray-500'
                              }`}>
                                {hasActivity
                                  ? `${isProfit ? '+' : ''}${formatUSD(day.net_pnl)}`
                                  : '--'
                                }
                              </div>

                              {/* Trade count indicator */}
                              {day.trade_count > 0 && (
                                <div className="text-[9px] text-gray-400 mt-0.5">
                                  {day.trade_count} trades
                                </div>
                              )}

                              {/* Cached indicator */}
                              {day.is_cached && (
                                <div className="absolute top-1 left-1">
                                  <Database className="w-2.5 h-2.5 text-gray-500" title="Cached data" />
                                </div>
                              )}

                              {/* Today indicator */}
                              {day.is_today && (
                                <div className="absolute -top-1 -right-1 w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
                              )}
                            </div>
                          );
                        })
                      ) : historyError ? (
                        <div className="col-span-7 text-center text-red-400 py-4">
                          Error: {historyError}
                        </div>
                      ) : (
                        <div className="col-span-7 text-center text-gray-500 py-4">
                          No P&L data for this period
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <div className="text-center text-gray-500 py-4">
          No P&L data available
        </div>
      )}
    </CollapsibleCard>
  );
}
