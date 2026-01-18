// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// Strategy Comparison Table Component

import { useState, useMemo } from 'react';
import { StrategyPerformance, formatPnL } from '../../types/decision-dashboard';
import { TrendingUp, TrendingDown, Minus, ChevronUp, ChevronDown } from 'lucide-react';

interface StrategyComparisonTableProps {
  data: StrategyPerformance[] | null;
  isLoading?: boolean;
}

type SortField = 'strategyName' | 'totalTrades' | 'winRate' | 'avgPnL' | 'totalPnL' | 'profitFactor' | 'sharpeRatio';
type SortDirection = 'asc' | 'desc';

export function StrategyComparisonTable({ data, isLoading }: StrategyComparisonTableProps) {
  const [sortField, setSortField] = useState<SortField>('totalPnL');
  const [sortDirection, setSortDirection] = useState<SortDirection>('desc');

  const sortedData = useMemo(() => {
    if (!data) return [];

    return [...data].sort((a, b) => {
      let aVal = a[sortField];
      let bVal = b[sortField];

      // Handle string sorting
      if (typeof aVal === 'string' && typeof bVal === 'string') {
        return sortDirection === 'asc'
          ? aVal.localeCompare(bVal)
          : bVal.localeCompare(aVal);
      }

      // Handle numeric sorting
      const aNum = Number(aVal) || 0;
      const bNum = Number(bVal) || 0;

      return sortDirection === 'asc' ? aNum - bNum : bNum - aNum;
    });
  }, [data, sortField, sortDirection]);

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('desc');
    }
  };

  const SortHeader = ({ field, label }: { field: SortField; label: string }) => {
    const isSorted = sortField === field;
    const ariaSort = isSorted ? (sortDirection === 'asc' ? 'ascending' : 'descending') : undefined;

    const handleKeyDown = (e: React.KeyboardEvent) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        handleSort(field);
      }
    };

    return (
      <th
        className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-white transition-colors"
        onClick={() => handleSort(field)}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        role="columnheader"
        aria-sort={ariaSort}
      >
        <div className="flex items-center gap-1">
          {label}
          {isSorted ? (
            sortDirection === 'asc' ? (
              <ChevronUp className="w-4 h-4" aria-hidden="true" />
            ) : (
              <ChevronDown className="w-4 h-4" aria-hidden="true" />
            )
          ) : (
            <div className="w-4 h-4" aria-hidden="true" />
          )}
        </div>
      </th>
    );
  };

  const TrendIcon = ({ trend }: { trend: 'up' | 'down' | 'neutral' }) => {
    if (trend === 'up') return <TrendingUp className="w-4 h-4 text-green-400" />;
    if (trend === 'down') return <TrendingDown className="w-4 h-4 text-red-400" />;
    return <Minus className="w-4 h-4 text-gray-400" />;
  };

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Strategy Performance</h3>
        </div>
        <div className="h-48 flex items-center justify-center">
          <div className="animate-pulse text-gray-400">Loading...</div>
        </div>
      </div>
    );
  }

  if (!data || data.length === 0) {
    return (
      <div className="bg-gray-800 rounded-lg p-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Strategy Performance</h3>
        </div>
        <div className="h-48 flex items-center justify-center">
          <p className="text-gray-400">No strategy data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-white font-semibold">Strategy Performance</h3>
        <span className="text-xs text-gray-400">
          {data.length} strategies
        </span>
      </div>

      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-gray-700">
          <thead>
            <tr>
              <SortHeader field="strategyName" label="Strategy" />
              <SortHeader field="totalTrades" label="Trades" />
              <SortHeader field="winRate" label="Win Rate" />
              <SortHeader field="avgPnL" label="Avg P&L" />
              <SortHeader field="totalPnL" label="Total P&L" />
              <SortHeader field="profitFactor" label="Profit Factor" />
              <SortHeader field="sharpeRatio" label="Sharpe" />
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Trend
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-700">
            {sortedData.map((strategy) => (
              <tr key={strategy.strategyName} className="hover:bg-gray-700/50 transition-colors">
                <td className="px-4 py-3 whitespace-nowrap">
                  <div className="flex items-center gap-2">
                    <span className="text-white font-medium">{strategy.strategyName}</span>
                    <span
                      className={`text-xs px-2 py-0.5 rounded ${
                        strategy.status === 'active'
                          ? 'bg-green-500/20 text-green-400'
                          : 'bg-gray-500/20 text-gray-400'
                      }`}
                    >
                      {strategy.status}
                    </span>
                  </div>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-gray-300">
                  {strategy.totalTrades}
                  <span className="text-xs text-gray-500 ml-1">
                    ({strategy.winningTrades}W/{strategy.losingTrades}L)
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span
                    className={`font-medium ${
                      strategy.winRate >= 60
                        ? 'text-green-400'
                        : strategy.winRate >= 50
                        ? 'text-yellow-400'
                        : 'text-red-400'
                    }`}
                  >
                    {strategy.winRate.toFixed(1)}%
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span
                    className={`font-medium ${
                      strategy.avgPnL >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}
                  >
                    {formatPnL(strategy.avgPnL)}
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span
                    className={`font-semibold ${
                      strategy.totalPnL >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}
                  >
                    {formatPnL(strategy.totalPnL)}
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span
                    className={`${
                      strategy.profitFactor >= 1.5
                        ? 'text-green-400'
                        : strategy.profitFactor >= 1
                        ? 'text-yellow-400'
                        : 'text-red-400'
                    }`}
                  >
                    {strategy.profitFactor.toFixed(2)}
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <span
                    className={`${
                      strategy.sharpeRatio >= 1.5
                        ? 'text-green-400'
                        : strategy.sharpeRatio >= 1
                        ? 'text-yellow-400'
                        : strategy.sharpeRatio >= 0
                        ? 'text-gray-400'
                        : 'text-red-400'
                    }`}
                  >
                    {strategy.sharpeRatio.toFixed(2)}
                  </span>
                </td>
                <td className="px-4 py-3 whitespace-nowrap">
                  <TrendIcon trend={strategy.trend} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Summary footer */}
      <div className="mt-4 pt-4 border-t border-gray-700 grid grid-cols-4 gap-4 text-sm">
        <div>
          <span className="text-gray-400">Best Strategy:</span>
          <span className="text-green-400 ml-2 font-medium">
            {sortedData.length > 0
              ? sortedData.reduce((best, curr) =>
                  curr.totalPnL > best.totalPnL ? curr : best
                ).strategyName
              : '-'}
          </span>
        </div>
        <div>
          <span className="text-gray-400">Total Trades:</span>
          <span className="text-white ml-2 font-medium">
            {sortedData.reduce((sum, s) => sum + s.totalTrades, 0)}
          </span>
        </div>
        <div>
          <span className="text-gray-400">Avg Win Rate:</span>
          <span className="text-white ml-2 font-medium">
            {sortedData.length > 0
              ? (
                  sortedData.reduce((sum, s) => sum + s.winRate, 0) / sortedData.length
                ).toFixed(1)
              : 0}
            %
          </span>
        </div>
        <div>
          <span className="text-gray-400">Total P&L:</span>
          <span
            className={`ml-2 font-medium ${
              sortedData.reduce((sum, s) => sum + s.totalPnL, 0) >= 0
                ? 'text-green-400'
                : 'text-red-400'
            }`}
          >
            {formatPnL(sortedData.reduce((sum, s) => sum + s.totalPnL, 0))}
          </span>
        </div>
      </div>
    </div>
  );
}

export default StrategyComparisonTable;
