// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// Blocking Reason Frequency Chart Component

import { useMemo } from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import {
  BlockingStatsSummary,
  BLOCKING_CATEGORY_COLORS,
  BLOCKING_CATEGORY_LABELS,
  BlockingCategory,
} from '../../types/decision-dashboard';
import { ShieldX, ShieldAlert, AlertTriangle } from 'lucide-react';

interface BlockingReasonChartProps {
  data: BlockingStatsSummary | null;
  isLoading?: boolean;
}

const CATEGORY_ICONS: Record<BlockingCategory, React.ComponentType<{ className?: string; style?: React.CSSProperties }>> = {
  HARD_BLOCK: ShieldX,
  SOFT_BLOCK: ShieldAlert,
  WARNING: AlertTriangle,
};

// Type for chart data points
interface BlockingChartDataPoint {
  code: string;
  description: string;
  count: number;
  percentage: number;
  category: BlockingCategory;
  color: string;
  symbolsAffected: number;
}

export function BlockingReasonChart({ data, isLoading }: BlockingReasonChartProps) {
  const chartData = useMemo<BlockingChartDataPoint[]>(() => {
    if (!data?.topReasons) return [];

    return data.topReasons.slice(0, 8).map((reason) => ({
      code: reason.code,
      description: reason.description,
      count: reason.count,
      percentage: reason.percentage,
      category: reason.category,
      color: BLOCKING_CATEGORY_COLORS[reason.category],
      symbolsAffected: reason.symbolsAffected,
    }));
  }, [data]);

  const truncateLabel = (label: string, maxLength: number = 15) => {
    if (label.length <= maxLength) return label;
    return label.substring(0, maxLength - 3) + '...';
  };

  const CustomTooltip = ({ active, payload }: { active?: boolean; payload?: Array<{ payload: BlockingChartDataPoint }> }) => {
    if (!active || !payload || !payload.length) return null;

    const item = payload[0].payload;
    const Icon = CATEGORY_ICONS[item.category];

    return (
      <div className="bg-gray-800 border border-gray-600 rounded-lg p-3 shadow-lg max-w-xs">
        <div className="flex items-center gap-2 mb-2">
          <Icon className="w-4 h-4" style={{ color: item.color }} />
          <span className="text-white font-semibold text-sm">{item.code}</span>
        </div>
        <p className="text-gray-300 text-xs mb-2">{item.description}</p>
        <div className="space-y-1 text-xs">
          <p className="text-gray-400">
            Count: <span className="text-white font-medium">{item.count}</span>
          </p>
          <p className="text-gray-400">
            Percentage: <span className="text-white font-medium">{item.percentage.toFixed(1)}%</span>
          </p>
          <p className="text-gray-400">
            Category:{' '}
            <span style={{ color: item.color }} className="font-medium">
              {BLOCKING_CATEGORY_LABELS[item.category]}
            </span>
          </p>
          <p className="text-gray-400">
            Symbols Affected: <span className="text-white font-medium">{item.symbolsAffected}</span>
          </p>
        </div>
      </div>
    );
  };

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 h-80">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Blocking Reasons</h3>
        </div>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse text-gray-400">Loading...</div>
        </div>
      </div>
    );
  }

  if (!data || chartData.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 h-80">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Blocking Reasons</h3>
        </div>
        <div className="h-64 flex items-center justify-center">
          <p className="text-gray-400">No blocking data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-white font-semibold">Blocking Reasons</h3>
        <span className="text-xs text-gray-400">
          {data.totalBlocks} total blocks
        </span>
      </div>

      {/* Category summary */}
      <div className="grid grid-cols-3 gap-3 mb-4">
        <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-2 text-center">
          <div className="flex items-center justify-center gap-1 mb-1">
            <ShieldX className="w-4 h-4 text-red-400" />
            <span className="text-red-400 text-xs font-medium">Hard</span>
          </div>
          <p className="text-white font-bold">{data.hardBlockCount}</p>
        </div>
        <div className="bg-orange-500/10 border border-orange-500/30 rounded-lg p-2 text-center">
          <div className="flex items-center justify-center gap-1 mb-1">
            <ShieldAlert className="w-4 h-4 text-orange-400" />
            <span className="text-orange-400 text-xs font-medium">Soft</span>
          </div>
          <p className="text-white font-bold">{data.softBlockCount}</p>
        </div>
        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-2 text-center">
          <div className="flex items-center justify-center gap-1 mb-1">
            <AlertTriangle className="w-4 h-4 text-yellow-400" />
            <span className="text-yellow-400 text-xs font-medium">Warning</span>
          </div>
          <p className="text-white font-bold">{data.warningCount}</p>
        </div>
      </div>

      {/* Bar chart */}
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={chartData}
            layout="vertical"
            margin={{ top: 5, right: 10, left: 5, bottom: 5 }}
          >
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" horizontal={false} />
            <XAxis type="number" stroke="#9ca3af" tick={{ fill: '#9ca3af', fontSize: 10 }} />
            <YAxis
              type="category"
              dataKey="code"
              stroke="#9ca3af"
              tick={{ fill: '#9ca3af', fontSize: 10 }}
              width={100}
              tickFormatter={(value) => truncateLabel(value)}
            />
            <Tooltip content={<CustomTooltip />} />
            <Bar dataKey="count" radius={[0, 4, 4, 0]}>
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Top reason highlight */}
      {data.mostFrequentBlock && (
        <div className="mt-4 bg-gray-700/30 rounded-lg p-3">
          <p className="text-xs text-gray-400 mb-1">Most Frequent Block:</p>
          <div className="flex items-center gap-2">
            <span className="text-white font-medium text-sm">{data.mostFrequentBlock}</span>
          </div>
        </div>
      )}

      {/* Legend */}
      <div className="flex justify-center gap-4 mt-4 text-xs">
        {Object.entries(BLOCKING_CATEGORY_LABELS).map(([category, label]) => (
          <div key={category} className="flex items-center gap-1">
            <div
              className="w-3 h-3 rounded"
              style={{ backgroundColor: BLOCKING_CATEGORY_COLORS[category as BlockingCategory] }}
            />
            <span className="text-gray-400">{label}</span>
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className="mt-4 pt-4 border-t border-gray-700 flex justify-between text-xs text-gray-500">
        <span>Symbols affected: {data.symbolsAffected}</span>
        <span>
          {data.periodStart} - {data.periodEnd}
        </span>
      </div>
    </div>
  );
}

export default BlockingReasonChart;
