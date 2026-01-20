// Epic 10: Exit Optimization Engine - Story 10.2: Position Analytics Dashboard
// Trade Distribution Charts Component (Pie Charts and Bar Charts)

import React from 'react';
import {
  PieChart,
  Pie,
  Cell,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts';
import { PieChart as PieChartIcon, BarChart3 } from 'lucide-react';
import {
  TradeDistributionResponse,
  MODE_COLORS,
  EXIT_REASON_COLORS,
  DECISION_MODE_COLORS,
  CHART_COLORS,
} from '../../types/positionAnalytics';

interface TradeDistributionChartsProps {
  data: TradeDistributionResponse | null;
  isLoading: boolean;
}

interface PieTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    payload: {
      label: string;
      count: number;
      percentage: number;
    };
  }>;
}

function PieTooltip({ active, payload }: PieTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const data = payload[0].payload;
  return (
    <div className="bg-gray-800 border border-gray-700 rounded-lg p-3 shadow-lg">
      <p className="text-sm font-medium text-white">{data.label}</p>
      <p className="text-sm text-gray-400">
        {data.count} trades ({data.percentage?.toFixed(1) ?? '-'}%)
      </p>
    </div>
  );
}

interface BarTooltipProps {
  active?: boolean;
  payload?: Array<{
    name: string;
    value: number;
    dataKey: string;
    color: string;
    payload: {
      label: string;
      wins: number;
      losses: number;
      winRate: number;
      totalTrades: number;
    };
  }>;
  label?: string;
}

function BarTooltip({ active, payload, label }: BarTooltipProps) {
  if (!active || !payload || payload.length === 0) return null;
  const data = payload[0].payload;
  return (
    <div className="bg-gray-800 border border-gray-700 rounded-lg p-3 shadow-lg">
      <p className="text-sm font-medium text-white mb-2">{label}</p>
      <div className="space-y-1">
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-green-400">Wins:</span>
          <span className="text-sm font-medium text-green-400">{data.wins}</span>
        </div>
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-red-400">Losses:</span>
          <span className="text-sm font-medium text-red-400">{data.losses}</span>
        </div>
        <div className="flex items-center justify-between gap-4">
          <span className="text-sm text-gray-300">Win Rate:</span>
          <span className="text-sm font-medium text-blue-400">
            {data.winRate?.toFixed(1) ?? '-'}%
          </span>
        </div>
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {[1, 2, 3, 4].map((i) => (
        <div key={i} className="bg-gray-800 rounded-lg p-6 animate-pulse">
          <div className="h-6 w-40 bg-gray-700 rounded mb-4"></div>
          <div className="h-48 bg-gray-700/50 rounded"></div>
        </div>
      ))}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center gap-3 mb-4">
        <PieChartIcon className="w-5 h-5 text-purple-400" />
        <h3 className="text-lg font-medium text-white">Trade Distribution</h3>
      </div>
      <div className="h-48 flex items-center justify-center">
        <p className="text-gray-400">No trade data available for this period</p>
      </div>
    </div>
  );
}

function ModeDistributionPie({
  data,
}: {
  data: TradeDistributionResponse['byMode'];
}) {
  const chartData = data.map((item) => ({
    ...item,
    name: item.mode,
    value: item.count,
    fill: MODE_COLORS[item.mode.toLowerCase()] || CHART_COLORS[0],
  }));

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center gap-3 mb-4">
        <PieChartIcon className="w-5 h-5 text-orange-400" />
        <h3 className="text-lg font-medium text-white">By Trading Mode</h3>
      </div>
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={40}
              outerRadius={70}
              dataKey="value"
              nameKey="label"
              label={({ label, percentage }) =>
                `${label} (${percentage?.toFixed(0) ?? '-'}%)`
              }
              labelLine={{ stroke: '#9CA3AF', strokeWidth: 1 }}
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.fill} />
              ))}
            </Pie>
            <Tooltip content={<PieTooltip />} />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="mt-4 flex flex-wrap gap-3 justify-center">
        {chartData.map((item, index) => (
          <div key={index} className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: item.fill }}
            />
            <span className="text-xs text-gray-400">{item.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function ExitReasonDistributionPie({
  data,
}: {
  data: TradeDistributionResponse['byExitReason'];
}) {
  const chartData = data.map((item, index) => ({
    ...item,
    name: item.reason,
    value: item.count,
    fill:
      EXIT_REASON_COLORS[item.reason.toUpperCase()] ||
      CHART_COLORS[index % CHART_COLORS.length],
  }));

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center gap-3 mb-4">
        <PieChartIcon className="w-5 h-5 text-green-400" />
        <h3 className="text-lg font-medium text-white">By Exit Reason</h3>
      </div>
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={40}
              outerRadius={70}
              dataKey="value"
              nameKey="label"
              label={({ label, percentage }) =>
                `${label} (${percentage?.toFixed(0) ?? '-'}%)`
              }
              labelLine={{ stroke: '#9CA3AF', strokeWidth: 1 }}
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.fill} />
              ))}
            </Pie>
            <Tooltip content={<PieTooltip />} />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="mt-4 flex flex-wrap gap-3 justify-center">
        {chartData.map((item, index) => (
          <div key={index} className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: item.fill }}
            />
            <span className="text-xs text-gray-400">{item.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function StrategyDistributionPie({
  data,
}: {
  data: TradeDistributionResponse['byStrategy'];
}) {
  const chartData = data.slice(0, 6).map((item, index) => ({
    ...item,
    name: item.strategy,
    label: item.strategy,
    value: item.count,
    fill: CHART_COLORS[index % CHART_COLORS.length],
  }));

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center gap-3 mb-4">
        <PieChartIcon className="w-5 h-5 text-blue-400" />
        <h3 className="text-lg font-medium text-white">By Entry Strategy</h3>
      </div>
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={40}
              outerRadius={70}
              dataKey="value"
              nameKey="strategy"
              label={({ strategy, percentage }) =>
                `${strategy} (${percentage?.toFixed(0) ?? '-'}%)`
              }
              labelLine={{ stroke: '#9CA3AF', strokeWidth: 1 }}
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.fill} />
              ))}
            </Pie>
            <Tooltip content={<PieTooltip />} />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="mt-4 flex flex-wrap gap-3 justify-center">
        {chartData.map((item, index) => (
          <div key={index} className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: item.fill }}
            />
            <span className="text-xs text-gray-400">{item.strategy}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function DecisionModeWinLossBar({
  data,
}: {
  data: TradeDistributionResponse['byDecisionMode'];
}) {
  const chartData = data.map((item) => ({
    ...item,
    name: item.label,
  }));

  return (
    <div className="bg-gray-800 rounded-lg p-6">
      <div className="flex items-center gap-3 mb-4">
        <BarChart3 className="w-5 h-5 text-purple-400" />
        <h3 className="text-lg font-medium text-white">
          Win/Loss by Decision Mode
        </h3>
      </div>
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={chartData} layout="vertical">
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
            <XAxis type="number" stroke="#9CA3AF" tick={{ fill: '#9CA3AF', fontSize: 12 }} />
            <YAxis
              type="category"
              dataKey="label"
              stroke="#9CA3AF"
              tick={{ fill: '#9CA3AF', fontSize: 12 }}
              width={100}
            />
            <Tooltip content={<BarTooltip />} />
            <Legend
              formatter={(value) => (
                <span className="text-gray-300 text-sm">{value}</span>
              )}
            />
            <Bar dataKey="wins" name="Wins" fill="#22c55e" stackId="a" />
            <Bar dataKey="losses" name="Losses" fill="#ef4444" stackId="a" />
          </BarChart>
        </ResponsiveContainer>
      </div>
      {/* Win Rate Summary */}
      <div className="mt-4 flex justify-center gap-6">
        {chartData.map((item, index) => (
          <div key={index} className="text-center">
            <p className="text-sm text-gray-400">{item.label}</p>
            <p
              className="text-lg font-semibold"
              style={{
                color:
                  DECISION_MODE_COLORS[item.mode] || CHART_COLORS[0],
              }}
            >
              {item.winRate?.toFixed(1) ?? '-'}%
            </p>
            <p className="text-xs text-gray-500">
              {item.totalTrades} trades
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}

export function TradeDistributionCharts({
  data,
  isLoading,
}: TradeDistributionChartsProps) {
  if (isLoading) return <LoadingSkeleton />;
  if (!data || data.totalTrades === 0) return <EmptyState />;

  return (
    <div className="space-y-6">
      {/* Summary */}
      <div className="bg-gray-800 rounded-lg p-4">
        <p className="text-sm text-gray-400">
          Total trades analyzed:{' '}
          <span className="text-white font-medium">{data.totalTrades}</span>
        </p>
      </div>

      {/* Charts Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {data.byMode && data.byMode.length > 0 && (
          <ModeDistributionPie data={data.byMode} />
        )}
        {data.byExitReason && data.byExitReason.length > 0 && (
          <ExitReasonDistributionPie data={data.byExitReason} />
        )}
        {data.byStrategy && data.byStrategy.length > 0 && (
          <StrategyDistributionPie data={data.byStrategy} />
        )}
        {data.byDecisionMode && data.byDecisionMode.length > 0 && (
          <DecisionModeWinLossBar data={data.byDecisionMode} />
        )}
      </div>
    </div>
  );
}

export default TradeDistributionCharts;
