// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// Score Distribution Chart Component

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
  Legend,
} from 'recharts';
import { ScoreDistribution, SCORE_BUCKET_COLORS, formatPercentage } from '../../types/decision-dashboard';

interface ScoreDistributionChartProps {
  data: ScoreDistribution | null;
  isLoading?: boolean;
}

interface ChartDataPoint {
  bucket: string;
  tradeCount: number;
  winRate: number;
  winCount: number;
  lossCount: number;
  avgPnL: number;
  fill: string;
}

export function ScoreDistributionChart({ data, isLoading }: ScoreDistributionChartProps) {
  const chartData = useMemo<ChartDataPoint[]>(() => {
    if (!data?.buckets) return [];

    return data.buckets.map((bucket, index) => ({
      bucket: bucket.bucket,
      tradeCount: bucket.tradeCount,
      winRate: bucket.winRate,
      winCount: bucket.winCount,
      lossCount: bucket.lossCount,
      avgPnL: bucket.avgPnL,
      fill: SCORE_BUCKET_COLORS[index] || '#6b7280',
    }));
  }, [data]);

  const CustomTooltip = ({ active, payload }: { active?: boolean; payload?: Array<{ payload: ChartDataPoint }> }) => {
    if (!active || !payload || !payload.length) return null;

    const item = payload[0].payload;
    return (
      <div className="bg-gray-800 border border-gray-600 rounded-lg p-3 shadow-lg">
        <p className="text-white font-semibold mb-2">Score: {item.bucket}</p>
        <div className="space-y-1 text-sm">
          <p className="text-gray-300">
            Trades: <span className="text-white font-medium">{item.tradeCount}</span>
          </p>
          <p className="text-gray-300">
            Wins: <span className="text-green-400 font-medium">{item.winCount}</span>
            {' / '}
            Losses: <span className="text-red-400 font-medium">{item.lossCount}</span>
          </p>
          <p className="text-gray-300">
            Win Rate: <span className="text-white font-medium">{item.winRate.toFixed(1)}%</span>
          </p>
          <p className="text-gray-300">
            Avg P&L: <span className={item.avgPnL >= 0 ? 'text-green-400' : 'text-red-400'}>
              {formatPercentage(item.avgPnL)}
            </span>
          </p>
        </div>
      </div>
    );
  };

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 h-80">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Score Distribution</h3>
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
          <h3 className="text-white font-semibold">Score Distribution</h3>
        </div>
        <div className="h-64 flex items-center justify-center">
          <p className="text-gray-400">No score data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-white font-semibold">Score Distribution</h3>
        <div className="text-sm text-gray-400">
          {data.totalTrades} total trades
        </div>
      </div>

      {/* Summary stats */}
      <div className="grid grid-cols-3 gap-4 mb-4 text-sm">
        <div className="bg-gray-700/50 rounded p-2">
          <p className="text-gray-400">Avg Score</p>
          <p className="text-white font-medium">{data.averageScore.toFixed(1)}</p>
        </div>
        <div className="bg-gray-700/50 rounded p-2">
          <p className="text-gray-400">High Score (&gt;75)</p>
          <p className="text-green-400 font-medium">{data.highScoreTrades}</p>
        </div>
        <div className="bg-gray-700/50 rounded p-2">
          <p className="text-gray-400">Low Score (&lt;50)</p>
          <p className="text-red-400 font-medium">{data.lowScoreTrades}</p>
        </div>
      </div>

      {/* Chart */}
      <div className="h-56">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={chartData}
            margin={{ top: 10, right: 10, left: -10, bottom: 0 }}
          >
            <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
            <XAxis
              dataKey="bucket"
              stroke="#9ca3af"
              tick={{ fill: '#9ca3af', fontSize: 12 }}
            />
            <YAxis
              stroke="#9ca3af"
              tick={{ fill: '#9ca3af', fontSize: 12 }}
            />
            <Tooltip content={<CustomTooltip />} />
            <Legend
              formatter={() => 'Trade Count'}
              wrapperStyle={{ color: '#9ca3af' }}
            />
            <Bar
              dataKey="tradeCount"
              name="Trade Count"
              radius={[4, 4, 0, 0]}
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.fill} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Legend */}
      <div className="flex justify-center gap-4 mt-2 text-xs">
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ backgroundColor: SCORE_BUCKET_COLORS[0] }} />
          <span className="text-gray-400">0-25 (Poor)</span>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ backgroundColor: SCORE_BUCKET_COLORS[1] }} />
          <span className="text-gray-400">26-50 (Fair)</span>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ backgroundColor: SCORE_BUCKET_COLORS[2] }} />
          <span className="text-gray-400">51-75 (Good)</span>
        </div>
        <div className="flex items-center gap-1">
          <div className="w-3 h-3 rounded" style={{ backgroundColor: SCORE_BUCKET_COLORS[3] }} />
          <span className="text-gray-400">76-100 (Excellent)</span>
        </div>
      </div>
    </div>
  );
}

export default ScoreDistributionChart;
