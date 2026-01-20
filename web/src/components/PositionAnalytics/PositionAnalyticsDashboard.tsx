// Epic 10: Exit Optimization Engine - Story 10.2: Position Analytics Dashboard
// Main Dashboard Component

import React, { useState, useEffect, useCallback } from 'react';
import { RefreshCw, AlertCircle, BarChart3, TrendingUp, Award, Zap } from 'lucide-react';
import {
  TimeRange,
  TIME_RANGE_OPTIONS,
  PositionAnalyticsSummary,
  EfficiencyTimelineResponse,
  TradeDistributionResponse,
  formatPnL,
  formatEfficiency,
} from '../../types/positionAnalytics';
import { EfficiencyTimelineChart } from './EfficiencyTimelineChart';
import { TradeDistributionCharts } from './TradeDistributionCharts';
import { PerformanceMetricsTables } from './PerformanceMetricsTables';
import { ExportButton, downloadExport } from './ExportButton';
import { futuresApi } from '../../services/futuresApi';

interface PositionAnalyticsDashboardProps {
  className?: string;
}

export function PositionAnalyticsDashboard({ className = '' }: PositionAnalyticsDashboardProps) {
  // State
  const [timeRange, setTimeRange] = useState<TimeRange>('7d');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<string | null>(null);

  // Data states
  const [summary, setSummary] = useState<PositionAnalyticsSummary | null>(null);
  const [timeline, setTimeline] = useState<EfficiencyTimelineResponse | null>(null);
  const [distribution, setDistribution] = useState<TradeDistributionResponse | null>(null);

  // Fetch all dashboard data
  const fetchDashboardData = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      // Fetch all data in parallel
      const [summaryData, timelineData, distributionData] = await Promise.all([
        futuresApi.getPositionAnalyticsSummary(timeRange),
        futuresApi.getEfficiencyTimeline(timeRange),
        futuresApi.getTradeDistribution(timeRange),
      ]);

      setSummary(summaryData);
      setTimeline(timelineData);
      setDistribution(distributionData);
      setLastUpdated(new Date().toISOString());
    } catch (err) {
      console.error('[PositionAnalytics] Failed to fetch dashboard data:', err);
      setError('Failed to load analytics data. Please try again.');
    } finally {
      setIsLoading(false);
    }
  }, [timeRange]);

  useEffect(() => {
    fetchDashboardData();
  }, [fetchDashboardData]);

  const handleRefresh = () => {
    fetchDashboardData();
  };

  const handleExport = async (format: 'csv' | 'json', range: TimeRange, mode?: string) => {
    await downloadExport('/api/futures/position-analytics/export', format, range, mode);
  };

  return (
    <div className={`min-h-screen bg-gray-900 p-6 ${className}`}>
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <BarChart3 className="w-8 h-8 text-blue-400" />
          <div>
            <h1 className="text-2xl font-bold text-white">Position Analytics</h1>
            <p className="text-sm text-gray-400">
              Efficiency metrics, trade analysis, and performance tracking
            </p>
          </div>
        </div>

        <div className="flex items-center gap-4">
          {/* Time Range Selector */}
          <select
            value={timeRange}
            onChange={(e) => setTimeRange(e.target.value as TimeRange)}
            className="bg-gray-800 text-white border border-gray-600 rounded-lg px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            aria-label="Select time range"
          >
            {TIME_RANGE_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>

          {/* Export Button */}
          <ExportButton
            timeRange={timeRange}
            onExport={handleExport}
          />

          {/* Refresh Button */}
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

      {/* Error Message */}
      {error && (
        <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4 mb-6 flex items-center gap-3">
          <AlertCircle className="w-5 h-5 text-red-400" />
          <span className="text-red-400">{error}</span>
          <button
            onClick={handleRefresh}
            className="ml-auto text-sm text-red-400 hover:text-red-300 underline"
          >
            Retry
          </button>
        </div>
      )}

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <SummaryCard
          icon={<BarChart3 className="w-5 h-5 text-blue-400" />}
          title="Total Trades"
          value={summary?.totalTrades?.toString() || '-'}
          subtitle={summary?.overallWinRate != null ? `${summary.overallWinRate.toFixed(1)}% win rate` : ''}
          isLoading={isLoading}
        />
        <SummaryCard
          icon={<TrendingUp className="w-5 h-5 text-green-400" />}
          title="Overall Efficiency"
          value={summary ? formatEfficiency(summary.overallEfficiency) : '-'}
          subtitle={summary ? `Best: ${summary.bestMode?.replace('_', ' ') || '-'}` : ''}
          isLoading={isLoading}
        />
        <SummaryCard
          icon={<Zap className="w-5 h-5 text-yellow-400" />}
          title="Total PnL"
          value={summary ? formatPnL(summary.totalPnL) : '-'}
          valueColor={summary?.totalPnL && summary.totalPnL >= 0 ? 'text-green-400' : 'text-red-400'}
          subtitle={summary ? `Avg: ${formatPnL(summary.avgPnL)}` : ''}
          isLoading={isLoading}
        />
        <SummaryCard
          icon={<Award className="w-5 h-5 text-purple-400" />}
          title="Best Strategy"
          value={summary?.bestStrategy || '-'}
          subtitle="By efficiency"
          isLoading={isLoading}
        />
      </div>

      {/* Efficiency Timeline */}
      <div className="mb-6">
        <EfficiencyTimelineChart data={timeline} isLoading={isLoading} />
      </div>

      {/* Trade Distribution Charts */}
      <div className="mb-6">
        <h2 className="text-lg font-medium text-white mb-4">Trade Distribution</h2>
        <TradeDistributionCharts data={distribution} isLoading={isLoading} />
      </div>

      {/* Performance Tables */}
      <div className="mb-6">
        <h2 className="text-lg font-medium text-white mb-4">Performance Breakdown</h2>
        <PerformanceMetricsTables
          modeMetrics={summary?.modeMetrics || null}
          strategyMetrics={summary?.strategyMetrics || null}
          regimeMetrics={summary?.regimeMetrics || null}
          isLoading={isLoading}
        />
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

// ==================== Summary Card Component ====================

interface SummaryCardProps {
  icon: React.ReactNode;
  title: string;
  value: string;
  valueColor?: string;
  subtitle: string;
  isLoading: boolean;
}

function SummaryCard({
  icon,
  title,
  value,
  valueColor = 'text-white',
  subtitle,
  isLoading,
}: SummaryCardProps) {
  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 animate-pulse">
        <div className="flex items-center gap-2 mb-2">
          <div className="w-5 h-5 bg-gray-700 rounded"></div>
          <div className="h-4 w-24 bg-gray-700 rounded"></div>
        </div>
        <div className="h-8 w-32 bg-gray-700 rounded mb-1"></div>
        <div className="h-3 w-20 bg-gray-700 rounded"></div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="flex items-center gap-2 mb-2">
        {icon}
        <p className="text-sm text-gray-400">{title}</p>
      </div>
      <p className={`text-2xl font-bold ${valueColor}`}>{value}</p>
      {subtitle && <p className="text-xs text-gray-500 mt-1">{subtitle}</p>}
    </div>
  );
}

export default PositionAnalyticsDashboard;
