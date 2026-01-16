import { useEffect, useState, useCallback } from 'react';
import { apiService } from '../services/api';
import { formatUSD } from '../services/futuresApi';
import { RefreshCw, Clock, Calendar, TrendingUp, TrendingDown, DollarSign, Activity, Coins } from 'lucide-react';

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

export default function PnLSummaryCards() {
  const [pnlData, setPnlData] = useState<PnLSummaryData | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [countdown, setCountdown] = useState<string>('');

  const fetchPnlSummary = useCallback(async () => {
    try {
      const response = await apiService.request<PnLSummaryData>('/futures/pnl-summary');
      setPnlData(response);
      setError(null);
    } catch (err: any) {
      console.error('Failed to fetch PnL summary:', err);
      setError(err?.message || 'Failed to load PnL data');
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Initial fetch
  useEffect(() => {
    fetchPnlSummary();
    // Refresh every 60 seconds
    const interval = setInterval(fetchPnlSummary, 60000);
    return () => clearInterval(interval);
  }, [fetchPnlSummary]);

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

  if (isLoading) {
    return (
      <div className="grid grid-cols-2 gap-4 mb-4">
        <div className="bg-gray-800 rounded-lg p-4 border border-gray-700 animate-pulse">
          <div className="h-6 bg-gray-700 rounded w-32 mb-3"></div>
          <div className="h-10 bg-gray-700 rounded w-24 mb-2"></div>
          <div className="h-4 bg-gray-700 rounded w-full"></div>
        </div>
        <div className="bg-gray-800 rounded-lg p-4 border border-gray-700 animate-pulse">
          <div className="h-6 bg-gray-700 rounded w-32 mb-3"></div>
          <div className="h-10 bg-gray-700 rounded w-24 mb-2"></div>
          <div className="h-4 bg-gray-700 rounded w-full"></div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mb-4 p-4 bg-red-900/20 border border-red-800 rounded-lg text-red-400 text-sm">
        {error}
        <button onClick={fetchPnlSummary} className="ml-2 text-red-300 hover:text-white underline">
          Retry
        </button>
      </div>
    );
  }

  if (!pnlData) return null;

  // Calculate net PnL
  const dailyNetPnl = pnlData.daily_pnl - pnlData.daily_commission - pnlData.daily_funding;
  const weeklyNetPnl = pnlData.weekly_pnl - pnlData.weekly_commission - pnlData.weekly_funding;
  const dailyTotalFees = pnlData.daily_commission + pnlData.daily_funding;
  const weeklyTotalFees = pnlData.weekly_commission + pnlData.weekly_funding;

  return (
    <div className="grid grid-cols-2 gap-4 mb-4">
      {/* Daily PnL Card */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700 bg-gray-900/50">
          <div className="flex items-center gap-2">
            <DollarSign className="w-5 h-5 text-green-400" />
            <span className="font-semibold text-white">Daily PnL</span>
          </div>
          <div className="flex items-center gap-2 text-xs text-gray-400">
            <Clock className="w-3.5 h-3.5" />
            <span>Resets in {countdown}</span>
          </div>
        </div>
        <div className="p-4">
          {/* Net PnL - Large Display */}
          <div className="text-center mb-4">
            <div className="text-xs text-gray-500 uppercase tracking-wide mb-1">Net PnL</div>
            <div className={`text-3xl font-bold ${dailyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {dailyNetPnl >= 0 ? '+' : ''}{formatUSD(dailyNetPnl)}
            </div>
          </div>

          {/* Breakdown Grid */}
          <div className="grid grid-cols-2 gap-3 text-sm">
            {/* Gross PnL */}
            <div className="bg-gray-900 rounded p-2">
              <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
                <TrendingUp className="w-3 h-3" />
                Gross PnL
              </div>
              <div className={`font-semibold ${pnlData.daily_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {formatUSD(pnlData.daily_pnl)}
              </div>
            </div>

            {/* Total Fees */}
            <div className="bg-gray-900 rounded p-2">
              <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
                <Coins className="w-3 h-3" />
                Total Fees
              </div>
              <div className="font-semibold text-yellow-400">
                -{formatUSD(dailyTotalFees)}
              </div>
            </div>

            {/* Commission */}
            <div className="bg-gray-900 rounded p-2">
              <div className="text-xs text-gray-500 mb-1">Commission</div>
              <div className="font-semibold text-orange-400">
                -{formatUSD(pnlData.daily_commission)}
              </div>
            </div>

            {/* Funding Fees */}
            <div className="bg-gray-900 rounded p-2">
              <div className="text-xs text-gray-500 mb-1">Funding Fees</div>
              <div className={`font-semibold ${pnlData.daily_funding >= 0 ? 'text-orange-400' : 'text-green-400'}`}>
                {pnlData.daily_funding >= 0 ? '-' : '+'}{formatUSD(Math.abs(pnlData.daily_funding))}
              </div>
            </div>
          </div>

          {/* Trade Count & Timezone */}
          <div className="mt-3 pt-3 border-t border-gray-700 flex items-center justify-between text-xs">
            <div className="flex items-center gap-1 text-gray-400">
              <Activity className="w-3 h-3" />
              <span>{pnlData.daily_trade_count} trades</span>
            </div>
            <div className="text-gray-500">
              {pnlData.timezone}
            </div>
          </div>
        </div>
      </div>

      {/* Weekly PnL Card */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700 bg-gray-900/50">
          <div className="flex items-center gap-2">
            <Calendar className="w-5 h-5 text-blue-400" />
            <span className="font-semibold text-white">Weekly PnL</span>
          </div>
          <div className="flex items-center gap-1 text-xs text-gray-400">
            <span>{pnlData.week_range}</span>
          </div>
        </div>
        <div className="p-4">
          {/* Net PnL - Large Display */}
          <div className="text-center mb-4">
            <div className="text-xs text-gray-500 uppercase tracking-wide mb-1">Net PnL</div>
            <div className={`text-3xl font-bold ${weeklyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {weeklyNetPnl >= 0 ? '+' : ''}{formatUSD(weeklyNetPnl)}
            </div>
          </div>

          {/* Breakdown Grid */}
          <div className="grid grid-cols-2 gap-3 text-sm">
            {/* Gross PnL */}
            <div className="bg-gray-900 rounded p-2">
              <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
                <TrendingUp className="w-3 h-3" />
                Gross PnL
              </div>
              <div className={`font-semibold ${pnlData.weekly_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {formatUSD(pnlData.weekly_pnl)}
              </div>
            </div>

            {/* Total Fees */}
            <div className="bg-gray-900 rounded p-2">
              <div className="flex items-center gap-1 text-xs text-gray-500 mb-1">
                <Coins className="w-3 h-3" />
                Total Fees
              </div>
              <div className="font-semibold text-yellow-400">
                -{formatUSD(weeklyTotalFees)}
              </div>
            </div>

            {/* Commission */}
            <div className="bg-gray-900 rounded p-2">
              <div className="text-xs text-gray-500 mb-1">Commission</div>
              <div className="font-semibold text-orange-400">
                -{formatUSD(pnlData.weekly_commission)}
              </div>
            </div>

            {/* Funding Fees */}
            <div className="bg-gray-900 rounded p-2">
              <div className="text-xs text-gray-500 mb-1">Funding Fees</div>
              <div className={`font-semibold ${pnlData.weekly_funding >= 0 ? 'text-orange-400' : 'text-green-400'}`}>
                {pnlData.weekly_funding >= 0 ? '-' : '+'}{formatUSD(Math.abs(pnlData.weekly_funding))}
              </div>
            </div>
          </div>

          {/* Trade Count & Date Range */}
          <div className="mt-3 pt-3 border-t border-gray-700 flex items-center justify-between text-xs">
            <div className="flex items-center gap-1 text-gray-400">
              <Activity className="w-3 h-3" />
              <span>{pnlData.weekly_trade_count} trades</span>
            </div>
            <div className="text-gray-500">
              {pnlData.week_start_date} - {pnlData.week_end_date}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
