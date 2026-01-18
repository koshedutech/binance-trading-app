// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// Regime Statistics Card Component

import { useMemo } from 'react';
import {
  PieChart,
  Pie,
  Cell,
  ResponsiveContainer,
  Tooltip,
  Legend,
} from 'recharts';
import {
  RegimeDistribution,
  REGIME_COLORS,
  REGIME_LABELS,
  MarketRegime,
} from '../../types/decision-dashboard';
import { TrendingUp, Activity, Zap, Layers } from 'lucide-react';

interface RegimeStatsCardProps {
  data: RegimeDistribution | null;
  isLoading?: boolean;
}

const REGIME_ICONS: Record<MarketRegime, React.ComponentType<{ className?: string; style?: React.CSSProperties }>> = {
  TRENDING: TrendingUp,
  RANGING: Activity,
  VOLATILE: Zap,
  CONSOLIDATING: Layers,
};

// Type for chart data points
interface RegimeChartDataPoint {
  name: string;
  value: number;
  percentage: number;
  regime: MarketRegime;
  avgDuration: number;
  tradesInRegime: number;
  winRateInRegime: number;
  avgPnLInRegime: number;
  color: string;
}

export function RegimeStatsCard({ data, isLoading }: RegimeStatsCardProps) {
  const chartData = useMemo<RegimeChartDataPoint[]>(() => {
    if (!data?.stats) return [];

    return data.stats.map((stat) => ({
      name: REGIME_LABELS[stat.regime],
      value: stat.count,
      percentage: stat.percentage,
      regime: stat.regime,
      avgDuration: stat.avgDuration,
      tradesInRegime: stat.tradesInRegime,
      winRateInRegime: stat.winRateInRegime,
      avgPnLInRegime: stat.avgPnLInRegime,
      color: REGIME_COLORS[stat.regime],
    }));
  }, [data]);

  const CustomTooltip = ({ active, payload }: { active?: boolean; payload?: Array<{ payload: RegimeChartDataPoint }> }) => {
    if (!active || !payload || !payload.length) return null;

    const item = payload[0].payload;
    return (
      <div className="bg-gray-800 border border-gray-600 rounded-lg p-3 shadow-lg">
        <div className="flex items-center gap-2 mb-2">
          <div
            className="w-3 h-3 rounded"
            style={{ backgroundColor: item.color }}
          />
          <span className="text-white font-semibold">{item.name}</span>
        </div>
        <div className="space-y-1 text-sm">
          <p className="text-gray-300">
            Count: <span className="text-white font-medium">{item.value}</span>
          </p>
          <p className="text-gray-300">
            Percentage: <span className="text-white font-medium">{item.percentage.toFixed(1)}%</span>
          </p>
          <p className="text-gray-300">
            Avg Duration: <span className="text-white font-medium">{item.avgDuration} candles</span>
          </p>
          <p className="text-gray-300">
            Trades: <span className="text-white font-medium">{item.tradesInRegime}</span>
          </p>
          <p className="text-gray-300">
            Win Rate:{' '}
            <span
              className={`font-medium ${
                item.winRateInRegime >= 50 ? 'text-green-400' : 'text-red-400'
              }`}
            >
              {item.winRateInRegime.toFixed(1)}%
            </span>
          </p>
        </div>
      </div>
    );
  };

  const renderCustomLegend = () => {
    return (
      <div className="flex flex-wrap justify-center gap-3 mt-2">
        {chartData.map((entry) => {
          const Icon = REGIME_ICONS[entry.regime];
          return (
            <div key={entry.name} className="flex items-center gap-1.5 text-xs">
              <div
                className="w-2.5 h-2.5 rounded"
                style={{ backgroundColor: entry.color }}
              />
              <Icon className="w-3 h-3" style={{ color: entry.color }} />
              <span className="text-gray-300">{entry.name}</span>
            </div>
          );
        })}
      </div>
    );
  };

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 h-80">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Regime Classification</h3>
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
          <h3 className="text-white font-semibold">Regime Classification</h3>
        </div>
        <div className="h-64 flex items-center justify-center">
          <p className="text-gray-400">No regime data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-white font-semibold">Regime Classification</h3>
        {data.currentRegime && (
          <div className="flex items-center gap-2">
            <span className="text-xs text-gray-400">Current:</span>
            <span
              className="text-xs font-medium px-2 py-1 rounded"
              style={{
                backgroundColor: `${REGIME_COLORS[data.currentRegime]}20`,
                color: REGIME_COLORS[data.currentRegime],
              }}
            >
              {REGIME_LABELS[data.currentRegime]}
            </span>
          </div>
        )}
      </div>

      {/* Pie Chart */}
      <div className="h-48">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              innerRadius={40}
              outerRadius={70}
              paddingAngle={2}
              dataKey="value"
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip content={<CustomTooltip />} />
          </PieChart>
        </ResponsiveContainer>
      </div>

      {/* Custom Legend */}
      {renderCustomLegend()}

      {/* Stats breakdown */}
      <div className="mt-4 space-y-2">
        {data.stats.map((stat) => {
          const Icon = REGIME_ICONS[stat.regime];
          return (
            <div
              key={stat.regime}
              className="flex items-center justify-between bg-gray-700/30 rounded px-3 py-2"
            >
              <div className="flex items-center gap-2">
                <Icon
                  className="w-4 h-4"
                  style={{ color: REGIME_COLORS[stat.regime] }}
                />
                <span className="text-white text-sm">{REGIME_LABELS[stat.regime]}</span>
              </div>
              <div className="flex items-center gap-4 text-sm">
                <span className="text-gray-400">{stat.percentage.toFixed(1)}%</span>
                <span
                  className={`font-medium ${
                    stat.winRateInRegime >= 50 ? 'text-green-400' : 'text-red-400'
                  }`}
                >
                  {stat.winRateInRegime.toFixed(0)}% WR
                </span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Footer */}
      <div className="mt-4 pt-4 border-t border-gray-700 flex justify-between text-xs text-gray-500">
        <span>Total: {data.totalClassifications} classifications</span>
        <span>Most common: {REGIME_LABELS[data.mostCommonRegime]}</span>
      </div>
    </div>
  );
}

export default RegimeStatsCard;
