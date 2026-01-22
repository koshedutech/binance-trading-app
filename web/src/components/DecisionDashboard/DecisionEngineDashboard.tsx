// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// Main Dashboard Component

import { useState, useEffect, useCallback } from 'react';
import axios from 'axios';
import {
  DashboardStats,
  TimeRange,
  TIME_RANGE_LABELS,
  ScoreDistribution,
  CalibrationSummary,
  StrategyPerformance,
  RegimeDistribution,
  BlockingStatsSummary,
} from '../../types/decision-dashboard';
import { ScoreDistributionChart } from './ScoreDistributionChart';
import { CalibrationAccuracyCard } from './CalibrationAccuracyCard';
import { StrategyComparisonTable } from './StrategyComparisonTable';
import { RegimeStatsCard } from './RegimeStatsCard';
import { BlockingReasonChart } from './BlockingReasonChart';
import { RefreshCw, AlertCircle, BarChart3 } from 'lucide-react';

// Token storage key (must match auth system)
const ACCESS_TOKEN_KEY = 'access_token';

// Helper to get auth headers for API calls
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem(ACCESS_TOKEN_KEY);
  return token ? { Authorization: `Bearer ${token}` } : {};
}

interface DecisionEngineDashboardProps {
  className?: string;
}

export function DecisionEngineDashboard({ className = '' }: DecisionEngineDashboardProps) {
  const [timeRange, setTimeRange] = useState<TimeRange>('7d');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);

  // Individual data states for flexibility
  const [scoreDistribution, setScoreDistribution] = useState<ScoreDistribution | null>(null);
  const [calibrationSummary, setCalibrationSummary] = useState<CalibrationSummary | null>(null);
  const [strategyPerformances, setStrategyPerformances] = useState<StrategyPerformance[] | null>(null);
  const [regimeDistribution, setRegimeDistribution] = useState<RegimeDistribution | null>(null);
  const [blockingStats, setBlockingStats] = useState<BlockingStatsSummary | null>(null);

  const fetchDashboardData = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      // Try the aggregate endpoint first
      const response = await axios.get<{ success: boolean; data: DashboardStats; error?: string }>(
        `/api/futures/decision/dashboard/stats?range=${timeRange}`,
        { headers: getAuthHeaders() }
      );

      if (response.data.success && response.data.data) {
        const data = response.data.data;
        setScoreDistribution(data.scoreDistribution);
        setCalibrationSummary(data.calibrationSummary);
        setStrategyPerformances(data.strategyPerformances);
        setRegimeDistribution(data.regimeDistribution);
        setBlockingStats(data.blockingStats);
        setLastUpdated(data.generatedAt);
      } else {
        // Aggregate endpoint not available, try individual endpoints
        await fetchIndividualData();
      }
    } catch (err) {
      // If aggregate endpoint fails, try individual endpoints
      console.log('[Dashboard] Aggregate endpoint not available, trying individual endpoints');
      await fetchIndividualData();
    } finally {
      setIsLoading(false);
    }
  }, [timeRange]);

  const fetchIndividualData = async () => {
    interface FetchResult {
      source: string;
      success: boolean;
    }
    const results: FetchResult[] = [];

    // Fetch strategy performance
    try {
      const perfResponse = await axios.get<{ success: boolean; performances: StrategyPerformance[] }>(
        `/api/strategy-performance?range=${timeRange}`,
        { headers: getAuthHeaders() }
      );
      if (perfResponse.data.performances) {
        setStrategyPerformances(perfResponse.data.performances);
        results.push({ source: 'strategy performance', success: true });
      } else {
        results.push({ source: 'strategy performance', success: false });
      }
    } catch {
      results.push({ source: 'strategy performance', success: false });
    }

    // Fetch calibration data for default strategy
    try {
      const calibResponse = await axios.get<CalibrationSummary>(
        '/api/futures/calibration/data/trend_following',
        { headers: getAuthHeaders() }
      );
      if (calibResponse.data) {
        setCalibrationSummary(calibResponse.data);
        results.push({ source: 'calibration', success: true });
      } else {
        results.push({ source: 'calibration', success: false });
      }
    } catch {
      results.push({ source: 'calibration', success: false });
    }

    // Fetch indicator performance for score distribution
    try {
      const indicatorResponse = await axios.get(
        '/api/futures/indicators/performance/trend_following',
        { headers: getAuthHeaders() }
      );
      if (indicatorResponse.data) {
        // Map indicator performance to score distribution format
        const indicatorData = indicatorResponse.data;
        const scoreData: ScoreDistribution = {
          buckets: [
            { bucket: '0-25', minScore: 0, maxScore: 25, tradeCount: indicatorData.bucket_0_25?.total_trades || 0, winCount: indicatorData.bucket_0_25?.winning_trades || 0, lossCount: (indicatorData.bucket_0_25?.total_trades || 0) - (indicatorData.bucket_0_25?.winning_trades || 0), winRate: indicatorData.bucket_0_25?.win_rate || 0, avgPnL: indicatorData.bucket_0_25?.avg_pnl_percent || 0, totalPnL: 0 },
            { bucket: '26-50', minScore: 26, maxScore: 50, tradeCount: indicatorData.bucket_26_50?.total_trades || 0, winCount: indicatorData.bucket_26_50?.winning_trades || 0, lossCount: (indicatorData.bucket_26_50?.total_trades || 0) - (indicatorData.bucket_26_50?.winning_trades || 0), winRate: indicatorData.bucket_26_50?.win_rate || 0, avgPnL: indicatorData.bucket_26_50?.avg_pnl_percent || 0, totalPnL: 0 },
            { bucket: '51-75', minScore: 51, maxScore: 75, tradeCount: indicatorData.bucket_51_75?.total_trades || 0, winCount: indicatorData.bucket_51_75?.winning_trades || 0, lossCount: (indicatorData.bucket_51_75?.total_trades || 0) - (indicatorData.bucket_51_75?.winning_trades || 0), winRate: indicatorData.bucket_51_75?.win_rate || 0, avgPnL: indicatorData.bucket_51_75?.avg_pnl_percent || 0, totalPnL: 0 },
            { bucket: '76-100', minScore: 76, maxScore: 100, tradeCount: indicatorData.bucket_76_100?.total_trades || 0, winCount: indicatorData.bucket_76_100?.winning_trades || 0, lossCount: (indicatorData.bucket_76_100?.total_trades || 0) - (indicatorData.bucket_76_100?.winning_trades || 0), winRate: indicatorData.bucket_76_100?.win_rate || 0, avgPnL: indicatorData.bucket_76_100?.avg_pnl_percent || 0, totalPnL: 0 },
          ],
          totalTrades: indicatorData.total_trades || 0,
          averageScore: indicatorData.average_score || 50,
          medianScore: 50,
          highScoreTrades: indicatorData.bucket_76_100?.total_trades || 0,
          lowScoreTrades: (indicatorData.bucket_0_25?.total_trades || 0) + (indicatorData.bucket_26_50?.total_trades || 0),
        };
        setScoreDistribution(scoreData);
        results.push({ source: 'indicator performance', success: true });
      } else {
        results.push({ source: 'indicator performance', success: false });
      }
    } catch {
      results.push({ source: 'indicator performance', success: false });
    }

    // Evaluate results - show error only if ALL endpoints failed
    const failedSources = results.filter(r => !r.success);
    const allFailed = failedSources.length === results.length && results.length > 0;

    if (allFailed) {
      setError('Unable to load dashboard data. Please try again later.');
    } else if (failedSources.length > 0) {
      console.warn(`[Dashboard] Some data sources unavailable: ${failedSources.map(f => f.source).join(', ')}`);
    }

    setLastUpdated(new Date().toISOString());
  };

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  const handleRefresh = () => {
    fetchDashboardData();
  };

  return (
    <div className={`min-h-screen bg-gray-900 p-6 ${className}`}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <BarChart3 className="w-8 h-8 text-blue-400" />
          <div>
            <h1 className="text-2xl font-bold text-white">Decision Engine Dashboard</h1>
            <p className="text-sm text-gray-400">Analytics and performance metrics</p>
          </div>
        </div>

        <div className="flex items-center gap-4">
          {/* Time range selector */}
          <label htmlFor="dashboard-time-range" className="sr-only">
            Select time range
          </label>
          <select
            id="dashboard-time-range"
            value={timeRange}
            onChange={(e) => setTimeRange(e.target.value as TimeRange)}
            className="bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            aria-label="Time range filter"
          >
            {Object.entries(TIME_RANGE_LABELS).map(([value, label]) => (
              <option key={value} value={value}>
                {label}
              </option>
            ))}
          </select>

          {/* Refresh button */}
          <button
            onClick={handleRefresh}
            disabled={isLoading}
            className="flex items-center gap-2 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-600/50 text-white px-4 py-2 rounded-lg transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {/* Error message */}
      {error && (
        <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 mb-6 flex items-center gap-3">
          <AlertCircle className="w-5 h-5 text-red-400" />
          <span className="text-red-400">{error}</span>
        </div>
      )}

      {/* Dashboard Grid */}
      <div className="space-y-6">
        {/* Row 1: Score Distribution & Calibration Accuracy */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <ScoreDistributionChart data={scoreDistribution} isLoading={isLoading} />
          <CalibrationAccuracyCard data={calibrationSummary} isLoading={isLoading} />
        </div>

        {/* Row 2: Strategy Performance Table */}
        <StrategyComparisonTable data={strategyPerformances} isLoading={isLoading} />

        {/* Row 3: Regime Stats & Blocking Reasons */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <RegimeStatsCard data={regimeDistribution} isLoading={isLoading} />
          <BlockingReasonChart data={blockingStats} isLoading={isLoading} />
        </div>
      </div>

      {/* Footer */}
      {lastUpdated && (
        <div className="mt-6 text-center text-xs text-gray-500">
          Last updated: {new Date(lastUpdated).toLocaleString()}
        </div>
      )}
    </div>
  );
}

export default DecisionEngineDashboard;
