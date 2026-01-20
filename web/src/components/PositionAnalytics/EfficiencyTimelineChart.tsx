// Epic 10: Exit Optimization Engine - Story 10.2: Position Analytics Dashboard
// Efficiency Timeline Chart Component

import React from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
  Area,
  ComposedChart,
} from 'recharts';
import { TrendingUp, TrendingDown, Minus, Activity } from 'lucide-react';
import {
  EfficiencyTimelineResponse,
  EfficiencyTimelineDataPoint,
  formatPercentage,
  getTrendIndicator,
} from '../../types/positionAnalytics';

interface EfficiencyTimelineChartProps {
  data: EfficiencyTimelineResponse | null;
  isLoading: boolean;
}

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    color: string;
    dataKey: string;
  }>;
  label?: string;
}

function CustomTooltip({ active, payload, label }: CustomTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;

  const dataPoint = payload[0]?.payload as EfficiencyTimelineDataPoint;
  if (!dataPoint) return null;

  const formatDate = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <div className="bg-gray-800 border border-gray-700 rounded-lg p-3 shadow-lg">
      <p className="text-sm text-gray-400 mb-2">{formatDate(label || '')}</p>
      <div className="space-y-1">
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-gray-300">Efficiency:</span>
          <span className="text-sm font-medium text-blue-400">
            {dataPoint.avgEfficiency?.toFixed(1) ?? '-'}%
          </span>
        </div>
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-gray-300">Win Rate:</span>
          <span className="text-sm font-medium text-green-400">
            {dataPoint.winRate?.toFixed(1) ?? '-'}%
          </span>
        </div>
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-gray-300">Trades:</span>
          <span className="text-sm font-medium text-white">
            {dataPoint.tradeCount}
          </span>
        </div>
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-gray-300">Avg PnL:</span>
          <span
            className={`text-sm font-medium ${
              dataPoint.avgPnL >= 0 ? 'text-green-400' : 'text-red-400'
            }`}
          >
            {formatPercentage(dataPoint.avgPnL)}
          </span>
        </div>
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="bg-gray-800 rounded-lg p-6 animate-pulse">
      <div className="flex items-center justify-between mb-4">
        <div className="h-6 w-48 bg-gray-700 rounded"></div>
        <div className="h-8 w-24 bg-gray-700 rounded"></div>
      </div>
      <div className="h-64 bg-gray-700/50 rounded"></div>
    </div>
  );
}

function EmptyState() {
  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center gap-3 mb-4">
        <Activity className="w-5 h-5 text-blue-400" />
        <h3 className="text-lg font-medium text-white">Efficiency Timeline</h3>
      </div>
      <div className="h-64 flex items-center justify-center">
        <p className="text-gray-400">No trade data available for this period</p>
      </div>
    </div>
  );
}

export function EfficiencyTimelineChart({
  data,
  isLoading,
}: EfficiencyTimelineChartProps) {
  if (isLoading) return <LoadingSkeleton />;
  if (!data || !data.dataPoints || data.dataPoints.length === 0) return <EmptyState />;

  const trend = getTrendIndicator(data.trendSlope);

  // Format x-axis label based on aggregation
  const formatXAxisLabel = (timestamp: string) => {
    const date = new Date(timestamp);
    if (data.aggregation === 'hourly') {
      return date.toLocaleTimeString('en-US', {
        hour: '2-digit',
        minute: '2-digit',
      });
    }
    return date.toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
    });
  };

  // Calculate statistics with null safety
  const avgEfficiency =
    data.dataPoints.reduce((sum, p) => sum + (p.avgEfficiency ?? 0), 0) /
    data.dataPoints.length;
  const totalTrades = data.dataPoints.reduce((sum, p) => sum + p.tradeCount, 0);

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <Activity className="w-5 h-5 text-blue-400" />
          <h3 className="text-lg font-medium text-white">Efficiency Timeline</h3>
        </div>
        <div className="flex items-center gap-4">
          <div className="text-sm text-gray-400">
            <span className="text-white font-medium">{totalTrades}</span> trades
          </div>
          <div className="flex items-center gap-2 px-3 py-1 bg-gray-700 rounded-lg">
            {trend.icon === 'up' && (
              <TrendingUp className="w-4 h-4" style={{ color: trend.color }} />
            )}
            {trend.icon === 'down' && (
              <TrendingDown className="w-4 h-4" style={{ color: trend.color }} />
            )}
            {trend.icon === 'neutral' && (
              <Minus className="w-4 h-4" style={{ color: trend.color }} />
            )}
            <span className="text-sm" style={{ color: trend.color }}>
              {trend.label}
            </span>
          </div>
        </div>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-3 gap-4 mb-4">
        <div className="bg-gray-700/50 rounded-lg p-3">
          <p className="text-xs text-gray-400 mb-1">Avg Efficiency</p>
          <p className="text-xl font-semibold text-blue-400">
            {isNaN(avgEfficiency) ? '-' : avgEfficiency.toFixed(1)}%
          </p>
        </div>
        <div className="bg-gray-700/50 rounded-lg p-3">
          <p className="text-xs text-gray-400 mb-1">Trend Slope</p>
          <p
            className="text-xl font-semibold"
            style={{ color: trend.color }}
          >
            {data.trendSlope != null ? (
              <>
                {data.trendSlope >= 0 ? '+' : ''}
                {data.trendSlope.toFixed(2)}
              </>
            ) : '-'}
          </p>
        </div>
        <div className="bg-gray-700/50 rounded-lg p-3">
          <p className="text-xs text-gray-400 mb-1">Aggregation</p>
          <p className="text-xl font-semibold text-white capitalize">
            {data.aggregation}
          </p>
        </div>
      </div>

      {/* Chart */}
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <ComposedChart data={data.dataPoints}>
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
            <XAxis
              dataKey="timestamp"
              tickFormatter={formatXAxisLabel}
              stroke="#9CA3AF"
              tick={{ fill: '#9CA3AF', fontSize: 12 }}
              interval="preserveStartEnd"
            />
            <YAxis
              yAxisId="left"
              stroke="#9CA3AF"
              tick={{ fill: '#9CA3AF', fontSize: 12 }}
              domain={[0, 100]}
              tickFormatter={(v) => `${v}%`}
            />
            <YAxis
              yAxisId="right"
              orientation="right"
              stroke="#9CA3AF"
              tick={{ fill: '#9CA3AF', fontSize: 12 }}
            />
            <Tooltip content={<CustomTooltip />} />
            <Legend
              wrapperStyle={{ paddingTop: '10px' }}
              formatter={(value) => (
                <span className="text-gray-300 text-sm">{value}</span>
              )}
            />
            <Area
              yAxisId="right"
              type="monotone"
              dataKey="tradeCount"
              name="Trades"
              fill="#6366f1"
              fillOpacity={0.2}
              stroke="#6366f1"
              strokeWidth={0}
            />
            <Line
              yAxisId="left"
              type="monotone"
              dataKey="avgEfficiency"
              name="Efficiency %"
              stroke="#3b82f6"
              strokeWidth={2}
              dot={{ fill: '#3b82f6', r: 3 }}
              activeDot={{ fill: '#3b82f6', r: 5, stroke: '#fff', strokeWidth: 2 }}
            />
            <Line
              yAxisId="left"
              type="monotone"
              dataKey="winRate"
              name="Win Rate %"
              stroke="#22c55e"
              strokeWidth={2}
              dot={{ fill: '#22c55e', r: 3 }}
              activeDot={{ fill: '#22c55e', r: 5, stroke: '#fff', strokeWidth: 2 }}
            />
          </ComposedChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

export default EfficiencyTimelineChart;
