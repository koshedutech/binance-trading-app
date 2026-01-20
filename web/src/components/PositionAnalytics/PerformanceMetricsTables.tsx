// Epic 10: Exit Optimization Engine - Story 10.2: Position Analytics Dashboard
// Performance Metrics Tables Component (Sortable Tables)

import React, { useState, useMemo } from 'react';
import { Table, TrendingUp, TrendingDown, ArrowUpDown, ChevronUp, ChevronDown } from 'lucide-react';
import {
  ModePerformanceMetrics,
  StrategyPerformanceMetrics,
  RegimePerformanceMetrics,
  formatPercentage,
  formatPnL,
  formatEfficiency,
  formatDuration,
  getModeColor,
  REGIME_COLORS,
} from '../../types/positionAnalytics';

// ==================== Types ====================

type SortDirection = 'asc' | 'desc';

interface SortConfig<T> {
  key: keyof T;
  direction: SortDirection;
}

// ==================== Props ====================

interface PerformanceMetricsTablesProps {
  modeMetrics: ModePerformanceMetrics[] | null;
  strategyMetrics: StrategyPerformanceMetrics[] | null;
  regimeMetrics: RegimePerformanceMetrics[] | null;
  isLoading: boolean;
}

// ==================== Helper Components ====================

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      {[1, 2, 3].map((i) => (
        <div key={i} className="bg-gray-800 rounded-lg p-6 animate-pulse">
          <div className="h-6 w-48 bg-gray-700 rounded mb-4"></div>
          <div className="space-y-2">
            {[1, 2, 3].map((j) => (
              <div key={j} className="h-10 bg-gray-700/50 rounded"></div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

interface SortableHeaderProps {
  label: string;
  sortKey: string;
  currentKey: string | null;
  direction: SortDirection;
  onSort: (key: string) => void;
  className?: string;
}

function SortableHeader({
  label,
  sortKey,
  currentKey,
  direction,
  onSort,
  className = '',
}: SortableHeaderProps) {
  const isActive = currentKey === sortKey;

  return (
    <th
      className={`px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-white transition-colors ${className}`}
      onClick={() => onSort(sortKey)}
    >
      <div className="flex items-center gap-1">
        <span>{label}</span>
        {isActive ? (
          direction === 'asc' ? (
            <ChevronUp className="w-3 h-3" />
          ) : (
            <ChevronDown className="w-3 h-3" />
          )
        ) : (
          <ArrowUpDown className="w-3 h-3 opacity-50" />
        )}
      </div>
    </th>
  );
}

function MetricCell({
  value,
  format = 'number',
}: {
  value: number;
  format?: 'number' | 'percent' | 'pnl' | 'efficiency' | 'duration';
}) {
  // Handle null/undefined values
  if (value == null || isNaN(value)) {
    return <span className="text-gray-500">-</span>;
  }

  let formattedValue: string;
  let color = 'text-white';

  switch (format) {
    case 'percent':
      formattedValue = `${value.toFixed(1)}%`;
      break;
    case 'pnl':
      formattedValue = formatPnL(value);
      color = value >= 0 ? 'text-green-400' : 'text-red-400';
      break;
    case 'efficiency':
      formattedValue = formatEfficiency(value);
      color = value >= 70 ? 'text-green-400' : value >= 50 ? 'text-yellow-400' : 'text-red-400';
      break;
    case 'duration':
      formattedValue = formatDuration(value);
      break;
    default:
      formattedValue = value.toFixed(2);
  }

  return <span className={color}>{formattedValue}</span>;
}

// ==================== Mode Performance Table ====================

interface ModeTableProps {
  data: ModePerformanceMetrics[];
}

function ModePerformanceTable({ data }: ModeTableProps) {
  const [sortConfig, setSortConfig] = useState<SortConfig<ModePerformanceMetrics>>({
    key: 'totalTrades',
    direction: 'desc',
  });

  const handleSort = (key: string) => {
    setSortConfig((prev) => ({
      key: key as keyof ModePerformanceMetrics,
      direction: prev.key === key && prev.direction === 'desc' ? 'asc' : 'desc',
    }));
  };

  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => {
      const aVal = a[sortConfig.key];
      const bVal = b[sortConfig.key];
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        return sortConfig.direction === 'asc' ? aVal - bVal : bVal - aVal;
      }
      return 0;
    });
  }, [data, sortConfig]);

  return (
    <div className="bg-gray-800 rounded-lg overflow-hidden">
      <div className="flex items-center gap-3 p-4 border-b border-gray-700">
        <Table className="w-5 h-5 text-orange-400" />
        <h3 className="text-lg font-medium text-white">Performance by Trading Mode</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-700">
          <thead className="bg-gray-900/50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Mode
              </th>
              <SortableHeader
                label="Trades"
                sortKey="totalTrades"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Win Rate"
                sortKey="winRate"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Efficiency"
                sortKey="avgEfficiency"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Total PnL"
                sortKey="totalPnL"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Profit Factor"
                sortKey="profitFactor"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Avg Hold"
                sortKey="avgHoldMinutes"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="BE Rate"
                sortKey="breakevenRate"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="TP1 Rate"
                sortKey="tp1HitRate"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-700">
            {sortedData.map((row, index) => (
              <tr key={row.mode} className={index % 2 === 0 ? 'bg-gray-800' : 'bg-gray-800/50'}>
                <td className="px-4 py-3 whitespace-nowrap">
                  <div className="flex items-center gap-2">
                    <div
                      className="w-3 h-3 rounded-full"
                      style={{ backgroundColor: getModeColor(row.mode) }}
                    />
                    <span className="text-sm font-medium text-white capitalize">
                      {row.mode.replace('_', ' ')}
                    </span>
                  </div>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-white">
                  {row.totalTrades}
                  <span className="text-gray-500 text-xs ml-1">
                    ({row.winCount}W / {row.lossCount}L)
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.winRate} format="percent" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgEfficiency} format="efficiency" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.totalPnL} format="pnl" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <span className={row.profitFactor != null && row.profitFactor >= 1 ? 'text-green-400' : 'text-red-400'}>
                    {row.profitFactor != null ? row.profitFactor.toFixed(2) : '-'}
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgHoldMinutes} format="duration" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.breakevenRate} format="percent" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.tp1HitRate} format="percent" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ==================== Strategy Performance Table ====================

interface StrategyTableProps {
  data: StrategyPerformanceMetrics[];
}

function StrategyPerformanceTable({ data }: StrategyTableProps) {
  const [sortConfig, setSortConfig] = useState<SortConfig<StrategyPerformanceMetrics>>({
    key: 'totalTrades',
    direction: 'desc',
  });

  const handleSort = (key: string) => {
    setSortConfig((prev) => ({
      key: key as keyof StrategyPerformanceMetrics,
      direction: prev.key === key && prev.direction === 'desc' ? 'asc' : 'desc',
    }));
  };

  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => {
      const aVal = a[sortConfig.key];
      const bVal = b[sortConfig.key];
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        return sortConfig.direction === 'asc' ? aVal - bVal : bVal - aVal;
      }
      return 0;
    });
  }, [data, sortConfig]);

  return (
    <div className="bg-gray-800 rounded-lg overflow-hidden">
      <div className="flex items-center gap-3 p-4 border-b border-gray-700">
        <Table className="w-5 h-5 text-blue-400" />
        <h3 className="text-lg font-medium text-white">Performance by Entry Strategy</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-700">
          <thead className="bg-gray-900/50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Strategy
              </th>
              <SortableHeader
                label="Trades"
                sortKey="totalTrades"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Win Rate"
                sortKey="winRate"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Efficiency"
                sortKey="avgEfficiency"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Total PnL"
                sortKey="totalPnL"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Avg PnL"
                sortKey="avgPnL"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Avg Hold"
                sortKey="avgHoldMinutes"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-700">
            {sortedData.map((row, index) => (
              <tr key={row.strategy} className={index % 2 === 0 ? 'bg-gray-800' : 'bg-gray-800/50'}>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span className="text-sm font-medium text-white">{row.strategy}</span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-white">
                  {row.totalTrades}
                  <span className="text-gray-500 text-xs ml-1">
                    ({row.winCount}W / {row.lossCount}L)
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.winRate} format="percent" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgEfficiency} format="efficiency" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.totalPnL} format="pnl" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgPnL} format="pnl" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgHoldMinutes} format="duration" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ==================== Regime Performance Table ====================

interface RegimeTableProps {
  data: RegimePerformanceMetrics[];
}

function RegimePerformanceTable({ data }: RegimeTableProps) {
  const [sortConfig, setSortConfig] = useState<SortConfig<RegimePerformanceMetrics>>({
    key: 'totalTrades',
    direction: 'desc',
  });

  const handleSort = (key: string) => {
    setSortConfig((prev) => ({
      key: key as keyof RegimePerformanceMetrics,
      direction: prev.key === key && prev.direction === 'desc' ? 'asc' : 'desc',
    }));
  };

  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => {
      const aVal = a[sortConfig.key];
      const bVal = b[sortConfig.key];
      if (typeof aVal === 'number' && typeof bVal === 'number') {
        return sortConfig.direction === 'asc' ? aVal - bVal : bVal - aVal;
      }
      return 0;
    });
  }, [data, sortConfig]);

  return (
    <div className="bg-gray-800 rounded-lg overflow-hidden">
      <div className="flex items-center gap-3 p-4 border-b border-gray-700">
        <Table className="w-5 h-5 text-green-400" />
        <h3 className="text-lg font-medium text-white">Performance by Market Regime</h3>
      </div>
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-700">
          <thead className="bg-gray-900/50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Regime
              </th>
              <SortableHeader
                label="Trades"
                sortKey="totalTrades"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Win Rate"
                sortKey="winRate"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Efficiency"
                sortKey="avgEfficiency"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Total PnL"
                sortKey="totalPnL"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
              <SortableHeader
                label="Avg PnL"
                sortKey="avgPnL"
                currentKey={sortConfig.key}
                direction={sortConfig.direction}
                onSort={handleSort}
              />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-700">
            {sortedData.map((row, index) => (
              <tr key={row.regime} className={index % 2 === 0 ? 'bg-gray-800' : 'bg-gray-800/50'}>
                <td className="px-4 py-3 whitespace-nowrap">
                  <div className="flex items-center gap-2">
                    <div
                      className="w-3 h-3 rounded-full"
                      style={{ backgroundColor: REGIME_COLORS[row.regime] || '#6b7280' }}
                    />
                    <span className="text-sm font-medium text-white">{row.regime}</span>
                  </div>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-white">
                  {row.totalTrades}
                  <span className="text-gray-500 text-xs ml-1">
                    ({row.winCount}W / {row.lossCount}L)
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.winRate} format="percent" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgEfficiency} format="efficiency" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.totalPnL} format="pnl" />
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <MetricCell value={row.avgPnL} format="pnl" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export function PerformanceMetricsTables({
  modeMetrics,
  strategyMetrics,
  regimeMetrics,
  isLoading,
}: PerformanceMetricsTablesProps) {
  if (isLoading) return <LoadingSkeleton />;

  const hasData =
    (modeMetrics && modeMetrics.length > 0) ||
    (strategyMetrics && strategyMetrics.length > 0) ||
    (regimeMetrics && regimeMetrics.length > 0);

  if (!hasData) {
    return (
      <div className="bg-gray-800 rounded-lg p-6">
        <div className="flex items-center gap-3 mb-4">
          <Table className="w-5 h-5 text-blue-400" />
          <h3 className="text-lg font-medium text-white">Performance Metrics</h3>
        </div>
        <div className="h-32 flex items-center justify-center">
          <p className="text-gray-400">No performance data available for this period</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {modeMetrics && modeMetrics.length > 0 && (
        <ModePerformanceTable data={modeMetrics} />
      )}
      {strategyMetrics && strategyMetrics.length > 0 && (
        <StrategyPerformanceTable data={strategyMetrics} />
      )}
      {regimeMetrics && regimeMetrics.length > 0 && (
        <RegimePerformanceTable data={regimeMetrics} />
      )}
    </div>
  );
}

export default PerformanceMetricsTables;
