import { useState, Fragment } from 'react';
import { Check, X, Database, Calendar, BarChart2, RefreshCw } from 'lucide-react';

// Interfaces
export interface CoinDataInfo {
  symbol: string;
  timeframes: string[];
  from_date: string;
  to_date: string;
  total_candles: number;
  storage_size_mb: number;
  feature_status?: Record<string, boolean>;  // Timeframe -> hasFeatures
  feature_counts?: Record<string, number>;   // Timeframe -> featureCount
}

interface DataAvailabilityTableProps {
  coins: CoinDataInfo[];
  onRefresh: () => void;
  onCalculateFeatures: (symbol: string, timeframe: string) => void;
  isLoading: boolean;
}

// Standard timeframes for the matrix
const STANDARD_TIMEFRAMES = ['1m', '3m', '5m', '15m', '1h', '4h', '1d'];

// Format date range display
function formatDateRange(fromDate: string, toDate: string): string {
  const from = new Date(fromDate);
  const to = new Date(toDate);
  const diffYears = (to.getTime() - from.getTime()) / (1000 * 60 * 60 * 24 * 365);

  if (diffYears >= 1) {
    return `${diffYears.toFixed(1)}y`;
  }
  const diffMonths = (to.getTime() - from.getTime()) / (1000 * 60 * 60 * 24 * 30);
  return `${Math.round(diffMonths)}m`;
}

// Format candle count
function formatCandleCount(count: number): string {
  if (count >= 1000000) {
    return `${(count / 1000000).toFixed(1)}M`;
  }
  if (count >= 1000) {
    return `${(count / 1000).toFixed(1)}K`;
  }
  return count.toString();
}

export default function DataAvailabilityTable({
  coins,
  onRefresh,
  onCalculateFeatures,
  isLoading,
}: DataAvailabilityTableProps) {
  const [sortBy, setSortBy] = useState<'symbol' | 'candles'>('symbol');
  const [sortAsc, setSortAsc] = useState(true);
  const [selectedCoin, setSelectedCoin] = useState<string | null>(null);

  // Sort coins
  const sortedCoins = [...coins].sort((a, b) => {
    if (sortBy === 'symbol') {
      return sortAsc ? a.symbol.localeCompare(b.symbol) : b.symbol.localeCompare(a.symbol);
    }
    return sortAsc ? a.total_candles - b.total_candles : b.total_candles - a.total_candles;
  });

  const handleSort = (column: 'symbol' | 'candles') => {
    if (sortBy === column) {
      setSortAsc(!sortAsc);
    } else {
      setSortBy(column);
      setSortAsc(true);
    }
  };

  return (
    <div className="bg-dark-800 rounded-lg border border-dark-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-dark-700">
        <div className="flex items-center gap-2">
          <Database className="w-5 h-5 text-primary-400" />
          <h3 className="text-lg font-semibold text-white">Data Availability by Coin</h3>
          <span className="px-2 py-0.5 bg-primary-500/20 text-primary-400 text-xs rounded">
            {coins.length} coins
          </span>
        </div>
        <button
          onClick={onRefresh}
          disabled={isLoading}
          className="flex items-center gap-2 px-3 py-1.5 bg-dark-700 hover:bg-dark-600 text-gray-300 rounded-lg transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full">
          <thead>
            <tr className="bg-dark-700/50">
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-white"
                onClick={() => handleSort('symbol')}
              >
                <div className="flex items-center gap-1">
                  Coin
                  {sortBy === 'symbol' && (
                    <span className="text-primary-400">{sortAsc ? '▲' : '▼'}</span>
                  )}
                </div>
              </th>
              {STANDARD_TIMEFRAMES.map((tf) => (
                <th
                  key={tf}
                  className="px-3 py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wider"
                >
                  {tf}
                </th>
              ))}
              <th
                className="px-4 py-3 text-right text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-white"
                onClick={() => handleSort('candles')}
              >
                <div className="flex items-center justify-end gap-1">
                  Total
                  {sortBy === 'candles' && (
                    <span className="text-primary-400">{sortAsc ? '▲' : '▼'}</span>
                  )}
                </div>
              </th>
              <th className="px-4 py-3 text-center text-xs font-medium text-gray-400 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-dark-700">
            {isLoading && coins.length === 0 ? (
              <tr>
                <td colSpan={STANDARD_TIMEFRAMES.length + 3} className="px-4 py-8 text-center">
                  <div className="flex flex-col items-center gap-2">
                    <RefreshCw className="w-6 h-6 text-gray-400 animate-spin" />
                    <span className="text-gray-400">Loading data availability...</span>
                  </div>
                </td>
              </tr>
            ) : sortedCoins.length === 0 ? (
              <tr>
                <td colSpan={STANDARD_TIMEFRAMES.length + 3} className="px-4 py-8 text-center">
                  <div className="flex flex-col items-center gap-2">
                    <Database className="w-8 h-8 text-gray-600" />
                    <span className="text-gray-400">No data available</span>
                    <span className="text-gray-500 text-sm">Download some data to get started</span>
                  </div>
                </td>
              </tr>
            ) : (
              sortedCoins.map((coin) => {
                const isExpanded = selectedCoin === coin.symbol;
                return (
                  <Fragment key={coin.symbol}>
                    <tr
                      className="hover:bg-dark-700/50 cursor-pointer transition-colors"
                      onClick={() => setSelectedCoin(isExpanded ? null : coin.symbol)}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-white">{coin.symbol}</span>
                          <span className="text-xs text-gray-500">
                            {formatDateRange(coin.from_date, coin.to_date)}
                          </span>
                        </div>
                      </td>
                      {STANDARD_TIMEFRAMES.map((tf) => {
                        const hasTimeframe = coin.timeframes.includes(tf);
                        const hasFeatures = coin.feature_status?.[tf] || false;
                        return (
                          <td key={tf} className="px-3 py-3 text-center">
                            {hasTimeframe ? (
                              <div className="flex items-center justify-center gap-1">
                                <Check className="w-4 h-4 text-green-400" />
                                {hasFeatures && (
                                  <BarChart2 className="w-3 h-3 text-purple-400" title="Features computed" />
                                )}
                              </div>
                            ) : (
                              <div className="flex items-center justify-center">
                                <X className="w-4 h-4 text-gray-600" />
                              </div>
                            )}
                          </td>
                        );
                      })}
                      <td className="px-4 py-3 text-right">
                        <span className="text-gray-300">{formatCandleCount(coin.total_candles)}</span>
                      </td>
                      <td className="px-4 py-3 text-center">
                        {(() => {
                          // Check if all timeframes have features computed
                          const allFeaturesComputed = coin.timeframes.every(
                            (tf) => coin.feature_status?.[tf]
                          );
                          // Find first timeframe without features
                          const tfWithoutFeatures = coin.timeframes.find(
                            (tf) => !coin.feature_status?.[tf]
                          );

                          if (allFeaturesComputed) {
                            return (
                              <span className="text-xs text-green-400" title="All features computed">
                                <Check className="w-4 h-4 inline" />
                              </span>
                            );
                          }

                          return (
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                // Calculate features for the first timeframe without features
                                const tf = tfWithoutFeatures || coin.timeframes[0] || '15m';
                                onCalculateFeatures(coin.symbol, tf);
                              }}
                              className="p-1.5 hover:bg-dark-600 rounded transition-colors"
                              title={`Calculate features for ${tfWithoutFeatures || 'all timeframes'}`}
                            >
                              <BarChart2 className="w-4 h-4 text-primary-400" />
                            </button>
                          );
                        })()}
                      </td>
                    </tr>
                    {/* Expanded row with details */}
                    {isExpanded && (
                      <tr>
                        <td colSpan={STANDARD_TIMEFRAMES.length + 3} className="px-4 py-3 bg-dark-700/30">
                          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div className="flex items-center gap-2">
                              <Calendar className="w-4 h-4 text-gray-400" />
                              <div>
                                <div className="text-xs text-gray-500">Date Range</div>
                                <div className="text-sm text-gray-300">
                                  {coin.from_date} to {coin.to_date}
                                </div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <Database className="w-4 h-4 text-gray-400" />
                              <div>
                                <div className="text-xs text-gray-500">Total Candles</div>
                                <div className="text-sm text-gray-300">
                                  {coin.total_candles.toLocaleString()}
                                </div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <BarChart2 className="w-4 h-4 text-gray-400" />
                              <div>
                                <div className="text-xs text-gray-500">Timeframes</div>
                                <div className="text-sm text-gray-300">
                                  {coin.timeframes.join(', ')}
                                </div>
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              <Database className="w-4 h-4 text-gray-400" />
                              <div>
                                <div className="text-xs text-gray-500">Storage</div>
                                <div className="text-sm text-gray-300">
                                  {coin.storage_size_mb.toFixed(2)} MB
                                </div>
                              </div>
                            </div>
                          </div>

                          {/* Feature status per timeframe */}
                          {coin.feature_status && Object.keys(coin.feature_status).length > 0 && (
                            <div className="mt-3 pt-3 border-t border-dark-600">
                              <div className="text-xs text-gray-500 mb-2">Feature Status by Timeframe</div>
                              <div className="flex flex-wrap gap-2">
                                {coin.timeframes.map((tf) => {
                                  const hasFeatures = coin.feature_status?.[tf];
                                  const featureCount = coin.feature_counts?.[tf] || 0;
                                  return (
                                    <div
                                      key={tf}
                                      className={`px-2 py-1 rounded text-xs flex items-center gap-1 ${
                                        hasFeatures
                                          ? 'bg-purple-500/20 text-purple-400'
                                          : 'bg-dark-600 text-gray-400'
                                      }`}
                                    >
                                      <span className="font-medium">{tf}</span>
                                      {hasFeatures ? (
                                        <span>({featureCount.toLocaleString()} cached)</span>
                                      ) : (
                                        <button
                                          onClick={(e) => {
                                            e.stopPropagation();
                                            onCalculateFeatures(coin.symbol, tf);
                                          }}
                                          className="ml-1 text-primary-400 hover:text-primary-300"
                                        >
                                          Calculate
                                        </button>
                                      )}
                                    </div>
                                  );
                                })}
                              </div>
                            </div>
                          )}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
