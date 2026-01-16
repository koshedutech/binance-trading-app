import { useEffect, useState, useCallback } from 'react';
import { apiService } from '../services/api';
import { formatUSD } from '../services/futuresApi';
import { Clock, Calendar, TrendingUp, DollarSign, Activity, Coins, ArrowRight } from 'lucide-react';

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

  // Calculate net PnL (Gross PnL minus all fees)
  // Note: daily_pnl is realized PnL from trades, commission and funding are costs
  const dailyNetPnl = pnlData.daily_pnl - pnlData.daily_commission - pnlData.daily_funding;
  const weeklyNetPnl = pnlData.weekly_pnl - pnlData.weekly_commission - pnlData.weekly_funding;
  const dailyTotalFees = pnlData.daily_commission + pnlData.daily_funding;
  const weeklyTotalFees = pnlData.weekly_commission + pnlData.weekly_funding;

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
      {/* Daily Net PNL Card */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700 bg-gradient-to-r from-green-900/30 to-gray-800">
          <div className="flex items-center gap-2">
            <DollarSign className="w-5 h-5 text-green-400" />
            <span className="font-bold text-white">Daily Net PNL</span>
          </div>
          <div className="flex items-center gap-1 text-xs text-gray-400 bg-gray-900/50 px-2 py-1 rounded">
            <Clock className="w-3 h-3" />
            <span>{pnlData.timezone}</span>
          </div>
        </div>

        <div className="p-4">
          {/* Net PnL - Large Bold Display */}
          <div className="text-center mb-3">
            <div className={`text-4xl font-extrabold ${dailyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {dailyNetPnl >= 0 ? '+' : ''}{formatUSD(dailyNetPnl)}
            </div>
          </div>

          {/* Calculation Formula: Profit - Fees */}
          <div className="flex items-center justify-center gap-2 text-sm mb-4 bg-gray-900/50 rounded-lg py-2 px-3">
            <span className={`font-semibold ${pnlData.daily_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {formatUSD(pnlData.daily_pnl)}
            </span>
            <span className="text-gray-500">−</span>
            <span className="font-semibold text-yellow-400">{formatUSD(dailyTotalFees)}</span>
            <span className="text-gray-500 text-xs">(profit − fees)</span>
          </div>

          {/* Fee Breakdown Row */}
          <div className="grid grid-cols-2 gap-2 mb-3">
            {/* Trading Fees (Commission) */}
            <div className="bg-gray-900 rounded-lg p-3">
              <div className="text-[10px] text-gray-500 uppercase tracking-wide mb-1">Trading Fees</div>
              <div className="text-lg font-bold text-yellow-400">
                -{formatUSD(pnlData.daily_commission)}
              </div>
            </div>
            {/* Funding Fees */}
            <div className="bg-gray-900 rounded-lg p-3">
              <div className="text-[10px] text-gray-500 uppercase tracking-wide mb-1">Funding Fees</div>
              <div className={`text-lg font-bold ${pnlData.daily_funding > 0 ? 'text-red-400' : pnlData.daily_funding < 0 ? 'text-green-400' : 'text-gray-400'}`}>
                {pnlData.daily_funding > 0 ? '-' : pnlData.daily_funding < 0 ? '+' : ''}{formatUSD(Math.abs(pnlData.daily_funding))}
              </div>
            </div>
          </div>

          {/* Reset Countdown & Trades */}
          <div className="flex items-center justify-between pt-3 border-t border-gray-700">
            <div className="flex items-center gap-1 text-xs text-gray-400">
              <Activity className="w-3 h-3" />
              <span>{pnlData.daily_trade_count} trades today</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500">Resets in</span>
              <span className="font-mono font-bold text-blue-400 bg-blue-900/30 px-2 py-0.5 rounded text-sm">
                {countdown}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Weekly Net PNL Card */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700 bg-gradient-to-r from-blue-900/30 to-gray-800">
          <div className="flex items-center gap-2">
            <Calendar className="w-5 h-5 text-blue-400" />
            <span className="font-bold text-white">Weekly Net PNL</span>
          </div>
          <div className="flex items-center gap-1 text-xs text-gray-400 bg-gray-900/50 px-2 py-1 rounded">
            <span>7 Days</span>
          </div>
        </div>

        <div className="p-4">
          {/* Net PnL - Large Bold Display */}
          <div className="text-center mb-3">
            <div className={`text-4xl font-extrabold ${weeklyNetPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {weeklyNetPnl >= 0 ? '+' : ''}{formatUSD(weeklyNetPnl)}
            </div>
          </div>

          {/* Calculation Formula: Profit - Fees */}
          <div className="flex items-center justify-center gap-2 text-sm mb-4 bg-gray-900/50 rounded-lg py-2 px-3">
            <span className={`font-semibold ${pnlData.weekly_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
              {formatUSD(pnlData.weekly_pnl)}
            </span>
            <span className="text-gray-500">−</span>
            <span className="font-semibold text-yellow-400">{formatUSD(weeklyTotalFees)}</span>
            <span className="text-gray-500 text-xs">(profit − fees)</span>
          </div>

          {/* Fee Breakdown Row */}
          <div className="grid grid-cols-2 gap-2 mb-3">
            {/* Trading Fees (Commission) */}
            <div className="bg-gray-900 rounded-lg p-3">
              <div className="text-[10px] text-gray-500 uppercase tracking-wide mb-1">Trading Fees</div>
              <div className="text-lg font-bold text-yellow-400">
                -{formatUSD(pnlData.weekly_commission)}
              </div>
            </div>
            {/* Funding Fees */}
            <div className="bg-gray-900 rounded-lg p-3">
              <div className="text-[10px] text-gray-500 uppercase tracking-wide mb-1">Funding Fees</div>
              <div className={`text-lg font-bold ${pnlData.weekly_funding > 0 ? 'text-red-400' : pnlData.weekly_funding < 0 ? 'text-green-400' : 'text-gray-400'}`}>
                {pnlData.weekly_funding > 0 ? '-' : pnlData.weekly_funding < 0 ? '+' : ''}{formatUSD(Math.abs(pnlData.weekly_funding))}
              </div>
            </div>
          </div>

          {/* Date Range & Trades */}
          <div className="flex items-center justify-between pt-3 border-t border-gray-700">
            <div className="flex items-center gap-1 text-xs text-gray-400">
              <Activity className="w-3 h-3" />
              <span>{pnlData.weekly_trade_count} trades</span>
            </div>
            <div className="flex items-center gap-1 text-sm text-purple-400">
              <span>{pnlData.week_start_date}</span>
              <ArrowRight className="w-3 h-3 text-gray-500" />
              <span>{pnlData.week_end_date}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
