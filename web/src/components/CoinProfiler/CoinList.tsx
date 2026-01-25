import { useState } from 'react';
import {
  TrendingUp,
  TrendingDown,
  Activity,
  ChevronDown,
  ChevronUp,
  Layers,
  Clock,
  BarChart2,
} from 'lucide-react';
import type { CoinData, CoinProfilerCoinsResponse } from '../../hooks/useCoinProfiler';

// ============================================================================
// Epic 14: Coin List Display
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================

interface CoinListProps {
  data: CoinProfilerCoinsResponse | null;
  isLoading: boolean;
}

/**
 * Format price with appropriate decimal places
 */
function formatPrice(price: number): string {
  if (price === 0) return '$0.00';
  if (price >= 1000) return `$${price.toLocaleString(undefined, { maximumFractionDigits: 2 })}`;
  if (price >= 1) return `$${price.toFixed(4)}`;
  return `$${price.toFixed(6)}`;
}

/**
 * Format volume in human readable format
 */
function formatVolume(volume: number): string {
  if (volume >= 1e9) return `${(volume / 1e9).toFixed(2)}B`;
  if (volume >= 1e6) return `${(volume / 1e6).toFixed(2)}M`;
  if (volume >= 1e3) return `${(volume / 1e3).toFixed(2)}K`;
  return volume.toFixed(2);
}

/**
 * Get source badge color
 */
function getSourceColor(source: string): string {
  switch (source) {
    case 'strategy':
      return 'bg-purple-500/20 text-purple-400 border-purple-500/30';
    case 'position':
      return 'bg-blue-500/20 text-blue-400 border-blue-500/30';
    case 'both':
      return 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30';
    default:
      return 'bg-gray-500/20 text-gray-400 border-gray-500/30';
  }
}

/**
 * Individual coin row component
 */
function CoinRow({ coin }: { coin: CoinData }) {
  const [expanded, setExpanded] = useState(false);
  const timeframeCount = Object.keys(coin.timeframes || {}).length;

  // Calculate price change from timeframe data if available
  const priceChange = coin.timeframes?.['5m']
    ? ((coin.price - coin.timeframes['5m'].open) / coin.timeframes['5m'].open) * 100
    : 0;

  const formatTimeAgo = (timeStr: string) => {
    if (!timeStr || timeStr === '0001-01-01T00:00:00Z') return 'N/A';
    try {
      const date = new Date(timeStr);
      const now = new Date();
      const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
      if (seconds < 60) return `${seconds}s ago`;
      if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
      return `${Math.floor(seconds / 3600)}h ago`;
    } catch {
      return 'N/A';
    }
  };

  return (
    <div className="border-b border-gray-700/50 last:border-b-0">
      {/* Main Row */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 py-2 px-2 hover:bg-gray-700/30 transition-colors"
      >
        {/* Symbol */}
        <div className="flex-1 text-left">
          <span className="font-medium text-white">{coin.symbol}</span>
        </div>

        {/* Price */}
        <div className="w-24 text-right">
          <span className="text-sm text-gray-300">{formatPrice(coin.price)}</span>
        </div>

        {/* Change */}
        <div className="w-16 text-right flex items-center justify-end gap-1">
          {priceChange !== 0 && (
            <>
              {priceChange > 0 ? (
                <TrendingUp className="w-3 h-3 text-green-500" />
              ) : (
                <TrendingDown className="w-3 h-3 text-red-500" />
              )}
              <span className={`text-xs ${priceChange > 0 ? 'text-green-500' : 'text-red-500'}`}>
                {priceChange > 0 ? '+' : ''}{priceChange.toFixed(2)}%
              </span>
            </>
          )}
        </div>

        {/* Volume */}
        <div className="w-20 text-right">
          <span className="text-xs text-gray-500">{formatVolume(coin.volume_24h)}</span>
        </div>

        {/* Source Badge */}
        <div className="w-20">
          <span className={`px-1.5 py-0.5 rounded text-[10px] border ${getSourceColor(coin.source)}`}>
            {coin.source}
          </span>
        </div>

        {/* Timeframes */}
        <div className="w-12 flex items-center justify-center gap-1">
          <Layers className="w-3 h-3 text-gray-500" />
          <span className="text-xs text-gray-500">{timeframeCount}</span>
        </div>

        {/* Expand Toggle */}
        <div className="w-6">
          {expanded ? (
            <ChevronUp className="w-4 h-4 text-gray-500" />
          ) : (
            <ChevronDown className="w-4 h-4 text-gray-500" />
          )}
        </div>
      </button>

      {/* Expanded Details */}
      {expanded && (
        <div className="bg-gray-800/50 px-3 py-2">
          {/* Meta info */}
          <div className="flex items-center gap-4 mb-2 text-xs text-gray-500">
            <div className="flex items-center gap-1">
              <Clock className="w-3 h-3" />
              <span>Updated: {formatTimeAgo(coin.updated_at)}</span>
            </div>
            <div className="flex items-center gap-1">
              <Activity className="w-3 h-3" />
              <span>Volatility: {(coin.volatility * 100).toFixed(2)}%</span>
            </div>
            {coin.strategies && coin.strategies.length > 0 && (
              <div className="flex items-center gap-1">
                <BarChart2 className="w-3 h-3" />
                <span>Strategies: {coin.strategies.join(', ')}</span>
              </div>
            )}
          </div>

          {/* Timeframe Data */}
          {timeframeCount > 0 && (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {Object.entries(coin.timeframes || {}).map(([tf, data]) => (
                <div key={tf} className="bg-gray-900 rounded p-2">
                  <div className="text-[10px] text-gray-500 uppercase mb-1">{tf}</div>
                  <div className="grid grid-cols-2 gap-1 text-[10px]">
                    <div>
                      <span className="text-gray-500">O:</span>{' '}
                      <span className="text-gray-300">{formatPrice(data.open)}</span>
                    </div>
                    <div>
                      <span className="text-gray-500">H:</span>{' '}
                      <span className="text-green-400">{formatPrice(data.high)}</span>
                    </div>
                    <div>
                      <span className="text-gray-500">L:</span>{' '}
                      <span className="text-red-400">{formatPrice(data.low)}</span>
                    </div>
                    <div>
                      <span className="text-gray-500">C:</span>{' '}
                      <span className="text-gray-300">{formatPrice(data.close)}</span>
                    </div>
                    <div className="col-span-2">
                      <span className="text-gray-500">Vol:</span>{' '}
                      <span className="text-gray-400">{formatVolume(data.volume)}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Coin List component showing all tracked coins
 */
export default function CoinList({ data, isLoading }: CoinListProps) {
  const [sortBy, setSortBy] = useState<'symbol' | 'price' | 'volume' | 'source'>('symbol');
  const [sortAsc, setSortAsc] = useState(true);

  if (isLoading && !data) {
    return (
      <div className="p-4 text-center text-gray-500">
        <Activity className="w-5 h-5 animate-pulse mx-auto mb-2" />
        <span className="text-sm">Loading coins...</span>
      </div>
    );
  }

  if (!data || data.coins.length === 0) {
    return (
      <div className="p-4 text-center text-gray-500">
        <Activity className="w-5 h-5 mx-auto mb-2 opacity-50" />
        <span className="text-sm">No coins being tracked</span>
        <p className="text-xs mt-1 text-gray-600">
          Start the profiler to begin tracking coins
        </p>
      </div>
    );
  }

  // Sort coins
  const sortedCoins = [...data.coins].sort((a, b) => {
    let comparison = 0;
    switch (sortBy) {
      case 'symbol':
        comparison = a.symbol.localeCompare(b.symbol);
        break;
      case 'price':
        comparison = a.price - b.price;
        break;
      case 'volume':
        comparison = a.volume_24h - b.volume_24h;
        break;
      case 'source':
        comparison = a.source.localeCompare(b.source);
        break;
    }
    return sortAsc ? comparison : -comparison;
  });

  const handleSort = (column: typeof sortBy) => {
    if (sortBy === column) {
      setSortAsc(!sortAsc);
    } else {
      setSortBy(column);
      setSortAsc(true);
    }
  };

  const SortHeader = ({ column, label, width }: { column: typeof sortBy; label: string; width: string }) => (
    <button
      onClick={() => handleSort(column)}
      className={`${width} text-left text-[10px] uppercase flex items-center gap-1 hover:text-gray-300 transition-colors`}
    >
      {label}
      {sortBy === column && (
        sortAsc ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />
      )}
    </button>
  );

  return (
    <div>
      {/* Header */}
      <div className="flex items-center gap-3 px-2 py-1.5 bg-gray-800/50 border-b border-gray-700 text-gray-500">
        <SortHeader column="symbol" label="Symbol" width="flex-1" />
        <SortHeader column="price" label="Price" width="w-24 text-right" />
        <div className="w-16 text-right text-[10px] uppercase">Change</div>
        <SortHeader column="volume" label="Volume" width="w-20 text-right" />
        <SortHeader column="source" label="Source" width="w-20" />
        <div className="w-12 text-center text-[10px] uppercase">TFs</div>
        <div className="w-6" />
      </div>

      {/* Coin Rows */}
      <div className="max-h-80 overflow-y-auto">
        {sortedCoins.map((coin) => (
          <CoinRow key={coin.symbol} coin={coin} />
        ))}
      </div>

      {/* Footer */}
      <div className="px-2 py-1.5 bg-gray-800/30 border-t border-gray-700 flex items-center justify-between">
        <span className="text-xs text-gray-500">
          Total: {data.total} coin{data.total !== 1 ? 's' : ''}
        </span>
        <div className="flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-purple-500" />
            <span className="text-gray-500">Strategy</span>
          </div>
          <div className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-blue-500" />
            <span className="text-gray-500">Position</span>
          </div>
          <div className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-cyan-500" />
            <span className="text-gray-500">Both</span>
          </div>
        </div>
      </div>
    </div>
  );
}
