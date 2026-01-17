import { useEffect, useState, useCallback, useRef } from 'react';
import { useFuturesStore, selectAvailableBalance, selectTotalMarginUsed, selectTotalUnrealizedPnl, selectActivePositions } from '../store/futuresStore';
import { apiService } from '../services/api';
import { formatUSD, getPositionColor, futuresApi } from '../services/futuresApi';
import { wsService } from '../services/websocket';
import CollapsibleCard from './CollapsibleCard';
import type { PnLPayload, WSEvent, GinieStatusPayload } from '../types';
import {
  Wallet,
  TrendingUp,
  Clock,
  Calendar,
  DollarSign,
  Activity,
  ArrowRight,
  LayoutDashboard,
  Wifi,
  Users,
} from 'lucide-react';

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
  // Timezone info
  timezone: string;
  timezone_offset: string;
  fetched_at: string;
}

interface AutopilotStats {
  active_positions: number;
  max_positions: number;
  running: boolean;
  unrealized_pnl: number;
}


export default function AccountStatsCard() {
  const accountInfo = useFuturesStore((state) => state.accountInfo);
  const availableBalance = useFuturesStore(selectAvailableBalance);
  const marginUsed = useFuturesStore(selectTotalMarginUsed);
  const totalUnrealizedPnl = useFuturesStore(selectTotalUnrealizedPnl);
  const activePositions = useFuturesStore(selectActivePositions);

  const [pnlData, setPnlData] = useState<PnLSummaryData | null>(null);
  const [autopilotStats, setAutopilotStats] = useState<AutopilotStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [countdown, setCountdown] = useState<string>('');
  const [wsConnected, setWsConnected] = useState(() => wsService.isConnected());

  // Safely parse values
  const safeNum = (val: number | string | null | undefined): number => {
    if (val === null || val === undefined) return 0;
    const num = typeof val === 'string' ? parseFloat(val) : val;
    return isNaN(num) ? 0 : num;
  };

  const walletBalance = safeNum(accountInfo?.total_wallet_balance);
  const marginBalance = walletBalance + totalUnrealizedPnl;

  const fetchPnlSummary = useCallback(async () => {
    try {
      const response = await apiService.request<PnLSummaryData>('/futures/pnl-summary');
      setPnlData(response);
    } catch (err: any) {
      console.error('Failed to fetch PnL summary:', err);
    } finally {
      setIsLoading(false);
    }
  }, []);

  const fetchAutopilotStatus = useCallback(async () => {
    try {
      const data = await futuresApi.getGinieAutopilotStatus();
      if (data?.stats) {
        setAutopilotStats({
          active_positions: data.stats.active_positions ?? 0,
          max_positions: data.stats.max_positions ?? 10,
          running: data.stats.running ?? false,
          unrealized_pnl: data.stats.unrealized_pnl ?? 0,
        });
      }
    } catch (err) {
      console.error('Failed to fetch autopilot status:', err);
    }
  }, []);


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

    const handleGinieUpdate = (event: WSEvent) => {
      const status = event.data.status as GinieStatusPayload;
      if (status) {
        fetchAutopilotStatus();
      }
    };

    wsService.subscribe('PNL_UPDATE', handlePnLUpdate);
    wsService.subscribe('GINIE_STATUS_UPDATE', handleGinieUpdate);

    // Fallback polling when WebSocket disconnected
    const startFallback = () => {
      if (!fallbackRef.current) {
        fallbackRef.current = setInterval(() => {
          fetchPnlSummary();
          fetchAutopilotStatus();
        }, 60000);
      }
    };

    const stopFallback = () => {
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
      fetchPnlSummary();
      fetchAutopilotStatus();
    };

    wsService.onDisconnect(startFallback);
    wsService.onConnect(stopFallback);

    if (!wsService.isConnected()) {
      startFallback();
    }

    // Initial fetch
    fetchPnlSummary();
    fetchAutopilotStatus();

    return () => {
      wsService.unsubscribe('PNL_UPDATE', handlePnLUpdate);
      wsService.unsubscribe('GINIE_STATUS_UPDATE', handleGinieUpdate);
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
    };
  }, [fetchPnlSummary, fetchAutopilotStatus]);

  // Track WebSocket connection status
  useEffect(() => {
    const handleConnect = () => setWsConnected(true);
    const handleDisconnect = () => setWsConnected(false);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);
    setWsConnected(wsService.isConnected());
  }, []);

  // Countdown timer
  useEffect(() => {
    if (!pnlData?.seconds_to_reset) return;

    let remainingSeconds = pnlData.seconds_to_reset;

    const updateCountdown = () => {
      const hours = Math.floor(remainingSeconds / 3600);
      const minutes = Math.floor((remainingSeconds % 3600) / 60);
      const seconds = remainingSeconds % 60;
      setCountdown(`${hours}h ${minutes}m ${seconds}s`);
      remainingSeconds--;
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);

    return () => clearInterval(interval);
  }, [pnlData?.seconds_to_reset]);

  // Calculate net PnL
  const dailyNetPnl = pnlData ? pnlData.daily_pnl - pnlData.daily_commission - pnlData.daily_funding : 0;
  const weeklyNetPnl = pnlData ? pnlData.weekly_pnl - pnlData.weekly_commission - pnlData.weekly_funding : 0;
  const dailyTotalFees = pnlData ? pnlData.daily_commission + pnlData.daily_funding : 0;
  const weeklyTotalFees = pnlData ? pnlData.weekly_commission + pnlData.weekly_funding : 0;

  // Position counts
  const currentPositions = autopilotStats?.active_positions ?? activePositions.length;
  const maxPositions = autopilotStats?.max_positions ?? 10;

  return (
    <CollapsibleCard
      title="Account Overview"
      icon={<LayoutDashboard className="w-4 h-4" />}
      defaultExpanded={false}
      badge="Stats"
      badgeColor="cyan"
    >
      <div className="space-y-4">
        {/* Row 1: Account Balances + Positions (6 columns) */}
        <div className="grid grid-cols-6 gap-3">
          {/* Wallet Balance */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <Wallet className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs text-gray-500">Wallet</span>
            </div>
            <div className="text-lg font-bold text-white">{formatUSD(walletBalance)}</div>
          </div>

          {/* Margin Balance */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <TrendingUp className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs text-gray-500">Margin</span>
            </div>
            <div className={`text-lg font-bold ${
              marginBalance > walletBalance ? 'text-green-500' : marginBalance < walletBalance ? 'text-red-500' : 'text-white'
            }`}>
              {formatUSD(marginBalance)}
            </div>
          </div>

          {/* Available Balance */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <DollarSign className="w-3.5 h-3.5 text-green-400" />
              <span className="text-xs text-gray-500">Available</span>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>
            <div className="text-lg font-bold text-green-500">{formatUSD(availableBalance)}</div>
          </div>

          {/* Margin Used */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <Activity className="w-3.5 h-3.5 text-yellow-400" />
              <span className="text-xs text-gray-500">Margin Used</span>
            </div>
            <div className="text-lg font-bold text-yellow-500">{formatUSD(marginUsed)}</div>
          </div>

          {/* Unrealized PnL */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <TrendingUp className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs text-gray-500">Unrealized</span>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>
            <div className={`text-lg font-bold ${getPositionColor(totalUnrealizedPnl)}`}>
              {formatUSD(totalUnrealizedPnl)}
            </div>
          </div>

          {/* Positions */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <Users className="w-3.5 h-3.5 text-purple-400" />
              <span className="text-xs text-gray-500">Positions</span>
            </div>
            <div className="text-lg font-bold text-purple-400">
              {currentPositions}/{maxPositions}
            </div>
          </div>
        </div>

        {/* Row 2: P&L Summary */}
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
            {/* Daily Net PNL Card */}
            <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
              {/* Header */}
              <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 bg-gradient-to-r from-green-900/30 to-gray-800">
                <div className="flex items-center gap-2">
                  <DollarSign className="w-4 h-4 text-green-400" />
                  <span className="font-semibold text-sm text-white">Daily Net PNL</span>
                </div>
                <div className="flex items-center gap-1 text-xs text-gray-400 bg-gray-900/50 px-2 py-0.5 rounded">
                  <Clock className="w-3 h-3" />
                  <span>{pnlData.timezone}</span>
                </div>
              </div>

              <div className="p-3">
                {/* Net PnL */}
                <div className="text-center mb-2">
                  <div className={`text-3xl font-extrabold ${dailyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {dailyNetPnl >= 0 ? '+' : ''}{formatUSD(dailyNetPnl)}
                  </div>
                </div>

                {/* Calculation */}
                <div className="flex items-center justify-center gap-2 text-xs mb-3 bg-gray-900/50 rounded py-1.5 px-2">
                  <span className={`font-semibold ${pnlData.daily_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatUSD(pnlData.daily_pnl)}
                  </span>
                  <span className="text-gray-500">−</span>
                  <span className="font-semibold text-yellow-400">{formatUSD(dailyTotalFees)}</span>
                  <span className="text-gray-500 text-[10px]">(profit − fees)</span>
                </div>

                {/* Fee Breakdown */}
                <div className="grid grid-cols-2 gap-2 mb-2">
                  <div className="bg-gray-900 rounded p-2">
                    <div className="text-[9px] text-gray-500 uppercase">Trading Fees</div>
                    <div className="text-sm font-bold text-yellow-400">-{formatUSD(pnlData.daily_commission)}</div>
                  </div>
                  <div className="bg-gray-900 rounded p-2">
                    <div className="text-[9px] text-gray-500 uppercase">Funding Fees</div>
                    <div className={`text-sm font-bold ${pnlData.daily_funding > 0 ? 'text-red-400' : pnlData.daily_funding < 0 ? 'text-green-400' : 'text-gray-400'}`}>
                      {pnlData.daily_funding > 0 ? '-' : pnlData.daily_funding < 0 ? '+' : ''}{formatUSD(Math.abs(pnlData.daily_funding))}
                    </div>
                  </div>
                </div>

                {/* Reset Countdown & Trades */}
                <div className="flex items-center justify-between pt-2 border-t border-gray-700">
                  <div className="flex items-center gap-1 text-xs text-gray-400">
                    <Activity className="w-3 h-3" />
                    <span>{pnlData.daily_trade_count} trades</span>
                  </div>
                  <div className="flex items-center gap-1">
                    <span className="text-[10px] text-gray-500">Resets in</span>
                    <span className="font-mono font-bold text-blue-400 bg-blue-900/30 px-1.5 py-0.5 rounded text-xs">
                      {countdown}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            {/* Weekly Net PNL Card */}
            <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
              {/* Header */}
              <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700 bg-gradient-to-r from-blue-900/30 to-gray-800">
                <div className="flex items-center gap-2">
                  <Calendar className="w-4 h-4 text-blue-400" />
                  <span className="font-semibold text-sm text-white">Weekly Net PNL</span>
                </div>
                <div className="flex items-center gap-1 text-xs text-gray-400 bg-gray-900/50 px-2 py-0.5 rounded">
                  <span>7 Days</span>
                </div>
              </div>

              <div className="p-3">
                {/* Net PnL */}
                <div className="text-center mb-2">
                  <div className={`text-3xl font-extrabold ${weeklyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {weeklyNetPnl >= 0 ? '+' : ''}{formatUSD(weeklyNetPnl)}
                  </div>
                </div>

                {/* Calculation */}
                <div className="flex items-center justify-center gap-2 text-xs mb-3 bg-gray-900/50 rounded py-1.5 px-2">
                  <span className={`font-semibold ${pnlData.weekly_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatUSD(pnlData.weekly_pnl)}
                  </span>
                  <span className="text-gray-500">−</span>
                  <span className="font-semibold text-yellow-400">{formatUSD(weeklyTotalFees)}</span>
                  <span className="text-gray-500 text-[10px]">(profit − fees)</span>
                </div>

                {/* Fee Breakdown */}
                <div className="grid grid-cols-2 gap-2 mb-2">
                  <div className="bg-gray-900 rounded p-2">
                    <div className="text-[9px] text-gray-500 uppercase">Trading Fees</div>
                    <div className="text-sm font-bold text-yellow-400">-{formatUSD(pnlData.weekly_commission)}</div>
                  </div>
                  <div className="bg-gray-900 rounded p-2">
                    <div className="text-[9px] text-gray-500 uppercase">Funding Fees</div>
                    <div className={`text-sm font-bold ${pnlData.weekly_funding > 0 ? 'text-red-400' : pnlData.weekly_funding < 0 ? 'text-green-400' : 'text-gray-400'}`}>
                      {pnlData.weekly_funding > 0 ? '-' : pnlData.weekly_funding < 0 ? '+' : ''}{formatUSD(Math.abs(pnlData.weekly_funding))}
                    </div>
                  </div>
                </div>

                {/* Date Range & Trades */}
                <div className="flex items-center justify-between pt-2 border-t border-gray-700">
                  <div className="flex items-center gap-1 text-xs text-gray-400">
                    <Activity className="w-3 h-3" />
                    <span>{pnlData.weekly_trade_count} trades</span>
                  </div>
                  <div className="flex items-center gap-1 text-xs text-purple-400">
                    <span>{pnlData.week_start_date}</span>
                    <ArrowRight className="w-3 h-3 text-gray-500" />
                    <span>{pnlData.week_end_date}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ) : null}
      </div>
    </CollapsibleCard>
  );
}
