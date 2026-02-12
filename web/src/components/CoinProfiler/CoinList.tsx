import { useState, useEffect, useRef } from 'react';
import {
  TrendingUp,
  TrendingDown,
  Activity,
  ChevronDown,
  ChevronUp,
  Clock,
  BarChart2,
  ArrowUp,
  ArrowDown,
  Minus,
} from 'lucide-react';
import type { CoinData, CoinProfilerCoinsResponse, TimeframeData } from '../../hooks/useCoinProfiler';
import { TradingViewChart } from './TradingViewChart';

// ============================================================================
// Epic 14: Coin List Display
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================

interface CoinListProps {
  data: CoinProfilerCoinsResponse | null;
  isLoading: boolean;
}

/**
 * Candle countdown timer - shows remaining time until candle closes
 */
function CandleCountdown({ closeTime }: { closeTime: string }) {
  const [remaining, setRemaining] = useState<string>('');

  useEffect(() => {
    const updateCountdown = () => {
      try {
        const closeDate = new Date(closeTime);
        const now = new Date();
        const diffMs = closeDate.getTime() - now.getTime();

        if (diffMs <= 0) {
          setRemaining('Closing...');
          return;
        }

        const totalSeconds = Math.floor(diffMs / 1000);
        const minutes = Math.floor(totalSeconds / 60);
        const seconds = totalSeconds % 60;

        if (minutes > 0) {
          setRemaining(`${minutes}m ${seconds.toString().padStart(2, '0')}s`);
        } else {
          setRemaining(`${seconds}s`);
        }
      } catch {
        setRemaining('');
      }
    };

    updateCountdown();
    const interval = setInterval(updateCountdown, 1000);
    return () => clearInterval(interval);
  }, [closeTime]);

  if (!remaining) return null;

  return (
    <span className="text-cyan-400 font-mono">
      ⏱ {remaining}
    </span>
  );
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
 * Convert Binance timeframe notation to TradingView interval
 * Binance: 1m, 3m, 5m, 15m, 30m, 1h, 2h, 4h, 6h, 8h, 12h, 1d, 3d, 1w, 1M
 * TradingView: 1, 3, 5, 15, 30, 60, 120, 240, 360, 480, 720, D, 3D, W, M
 */
function timeframeToTradingViewInterval(tf: string): string {
  // Handle minute timeframes
  if (tf.endsWith('m')) {
    return tf.replace('m', '');
  }
  // Handle hour timeframes - convert to minutes
  if (tf.endsWith('h')) {
    const hours = parseInt(tf.replace('h', ''), 10);
    return String(hours * 60);
  }
  // Handle day timeframes
  if (tf === '1d' || tf === '1D') return 'D';
  if (tf === '3d' || tf === '3D') return '3D';
  // Handle week/month
  if (tf === '1w' || tf === '1W') return 'W';
  if (tf === '1M') return 'M';
  // Default to 15 minutes if unknown
  return '15';
}

/**
 * Get the primary timeframe data (first available)
 */
function getPrimaryTimeframe(coin: CoinData): { name: string; data: TimeframeData } | null {
  const timeframes = coin.timeframes || {};
  const entries = Object.entries(timeframes);
  if (entries.length === 0) return null;
  // Prefer common timeframes in order
  const preferredOrder = ['1m', '3m', '5m', '15m', '30m', '1h', '4h', '1d'];
  for (const tf of preferredOrder) {
    if (timeframes[tf]) return { name: tf, data: timeframes[tf] };
  }
  return { name: entries[0][0], data: entries[0][1] };
}

/**
 * Calculate price change percentage from open to close
 */
function calculatePriceChange(tfData: TimeframeData | null): number {
  if (!tfData || tfData.open === 0) return 0;
  return ((tfData.close - tfData.open) / tfData.open) * 100;
}

/**
 * Calculate volatility from high-low range
 */
function calculateVolatility(tfData: TimeframeData | null): number {
  if (!tfData || tfData.open === 0) return 0;
  return ((tfData.high - tfData.low) / tfData.open) * 100;
}

/**
 * Determine source label based on available data
 */
function getSourceLabel(coin: CoinData): string {
  if (coin.source && coin.source !== '') return coin.source;
  if (coin.strategies && coin.strategies.length > 0) return 'strategy';
  return 'scan';
}

/**
 * Get source badge styling
 */
function getSourceStyle(source: string): { bg: string; text: string; label: string } {
  switch (source) {
    case 'strategy':
      return { bg: 'bg-purple-500/20', text: 'text-purple-400', label: 'Strategy' };
    case 'position':
      return { bg: 'bg-blue-500/20', text: 'text-blue-400', label: 'Position' };
    case 'scan':
      return { bg: 'bg-gray-500/20', text: 'text-gray-400', label: 'Scan' };
    default:
      return { bg: 'bg-gray-500/20', text: 'text-gray-400', label: source || 'Unknown' };
  }
}

/**
 * Price indicator with color coding
 */
function PriceIndicator({ current, previous, showDiff = false }: {
  current: number;
  previous?: number;
  showDiff?: boolean;
}) {
  const diff = previous ? current - previous : 0;
  const isUp = diff > 0;
  const isDown = diff < 0;

  return (
    <span className={`font-medium transition-colors duration-300 ${
      isUp ? 'text-green-400' : isDown ? 'text-red-400' : 'text-gray-300'
    }`}>
      {formatPrice(current)}
      {showDiff && diff !== 0 && (
        <span className="ml-1 text-[10px]">
          {isUp ? '+' : ''}{diff.toFixed(diff < 1 ? 6 : 2)}
        </span>
      )}
    </span>
  );
}

/**
 * Change indicator with arrow and percentage
 */
function ChangeIndicator({ change }: { change: number }) {
  if (change === 0) {
    return (
      <div className="flex items-center justify-end gap-1 text-gray-500">
        <Minus className="w-3 h-3" />
        <span className="text-xs">0.00%</span>
      </div>
    );
  }

  const isPositive = change > 0;
  return (
    <div className={`flex items-center justify-end gap-1 ${isPositive ? 'text-green-400' : 'text-red-400'}`}>
      {isPositive ? (
        <TrendingUp className="w-3 h-3" />
      ) : (
        <TrendingDown className="w-3 h-3" />
      )}
      <span className="text-xs font-medium">
        {isPositive ? '+' : ''}{change.toFixed(2)}%
      </span>
    </div>
  );
}

/**
 * Volume indicator with direction
 */
function VolumeIndicator({ volume, takerBuyRatio }: { volume: number; takerBuyRatio: number }) {
  // takerBuyRatio > 0.5 means more buying pressure
  const isBullish = takerBuyRatio > 0.55;
  const isBearish = takerBuyRatio < 0.45;

  return (
    <div className="flex items-center justify-end gap-1">
      {isBullish && <ArrowUp className="w-3 h-3 text-green-500" />}
      {isBearish && <ArrowDown className="w-3 h-3 text-red-500" />}
      <span className={`text-xs ${isBullish ? 'text-green-400' : isBearish ? 'text-red-400' : 'text-gray-500'}`}>
        {formatVolume(volume)}
      </span>
    </div>
  );
}

/**
 * Individual coin row component with real-time updates
 * Single expandable mode: only one coin can be expanded at a time
 */
interface CoinRowProps {
  coin: CoinData;
  prevCoin?: CoinData;
  isExpanded: boolean;
  onToggleExpand: () => void;
}

function CoinRow({ coin, prevCoin, isExpanded, onToggleExpand }: CoinRowProps) {
  const expanded = isExpanded; // Use controlled state from parent
  const [flash, setFlash] = useState<'up' | 'down' | null>(null);
  const prevPriceRef = useRef(coin.price);

  const primaryTf = getPrimaryTimeframe(coin);
  const tfData = primaryTf?.data || null;
  const priceChange = calculatePriceChange(tfData);
  const volatility = calculateVolatility(tfData);
  const source = getSourceLabel(coin);
  const sourceStyle = getSourceStyle(source);

  // Calculate taker buy ratio for volume direction
  const takerBuyRatio = tfData && tfData.volume > 0
    ? tfData.taker_buy_vol / tfData.volume
    : 0.5;

  // Flash effect when price changes
  useEffect(() => {
    if (coin.price !== prevPriceRef.current) {
      const direction = coin.price > prevPriceRef.current ? 'up' : 'down';
      setFlash(direction);
      prevPriceRef.current = coin.price;

      const timer = setTimeout(() => setFlash(null), 500);
      return () => clearTimeout(timer);
    }
  }, [coin.price]);

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

  // Get all timeframe names, including position timeframe if available
  const tfKeys = Object.keys(coin.timeframes || {});
  const positionTf = coin.position_timeframe;
  // Show position timeframe prominently if source is position
  const timeframeNames = positionTf && source === 'position'
    ? (tfKeys.length > 0 && !tfKeys.includes(positionTf)
        ? `${positionTf}, ${tfKeys.join(', ')}`
        : tfKeys.length > 0 ? tfKeys.join(', ') : positionTf)
    : (tfKeys.join(', ') || '-');

  return (
    <div className={`border-b border-gray-700/50 last:border-b-0 transition-colors duration-300 ${
      flash === 'up' ? 'bg-green-900/20' : flash === 'down' ? 'bg-red-900/20' : ''
    }`}>
      {/* Main Row */}
      <button
        onClick={onToggleExpand}
        className="w-full flex items-center gap-3 py-2 px-2 hover:bg-gray-700/30 transition-colors"
      >
        {/* Symbol */}
        <div className="flex-1 text-left">
          <span className="font-medium text-white">{coin.symbol}</span>
        </div>

        {/* Price with color coding */}
        <div className="w-24 text-right">
          <span className={`text-sm font-medium transition-colors duration-300 ${
            priceChange > 0 ? 'text-green-400' : priceChange < 0 ? 'text-red-400' : 'text-gray-300'
          }`}>
            {formatPrice(coin.price)}
          </span>
        </div>

        {/* Change % */}
        <div className="w-20">
          <ChangeIndicator change={priceChange} />
        </div>

        {/* Volume with direction */}
        <div className="w-20">
          <VolumeIndicator
            volume={tfData?.volume || 0}
            takerBuyRatio={takerBuyRatio}
          />
        </div>

        {/* Source Badge */}
        <div className="w-16">
          <span className={`px-1.5 py-0.5 rounded text-[10px] ${sourceStyle.bg} ${sourceStyle.text}`}>
            {sourceStyle.label}
          </span>
        </div>

        {/* Timeframes - show actual names */}
        <div className="w-14 text-center">
          <span className="text-xs text-cyan-400 font-medium">{timeframeNames}</span>
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
              <span className={volatility > 1 ? 'text-yellow-400' : 'text-gray-500'}>
                Volatility: {volatility.toFixed(2)}%
              </span>
            </div>
            {coin.strategies && coin.strategies.length > 0 && (
              <div className="flex items-center gap-1">
                <BarChart2 className="w-3 h-3" />
                <span>Strategies: {coin.strategies.join(', ')}</span>
              </div>
            )}
          </div>

          {/* Chart + Data Layout */}
          <div className="flex gap-3">
            {/* TradingView Chart - Left Side */}
            <div className="flex-1 min-w-0">
              <TradingViewChart
                symbol={coin.symbol}
                timeframe={primaryTf?.name ? timeframeToTradingViewInterval(primaryTf.name) : '15'}
                height={280}
              />
            </div>

            {/* Timeframe Data Cards - Right Side */}
            <div className="w-72 flex-shrink-0">
              {Object.keys(coin.timeframes || {}).length > 0 && (
                <div className="space-y-2 max-h-[280px] overflow-y-auto pr-1">
              {Object.entries(coin.timeframes || {}).map(([tf, data]) => {
                const tfChange = data.open > 0 ? ((data.close - data.open) / data.open) * 100 : 0;
                const tfVolatility = data.open > 0 ? ((data.high - data.low) / data.open) * 100 : 0;
                const buyRatio = data.volume > 0 ? data.taker_buy_vol / data.volume : 0.5;
                const isBullish = buyRatio > 0.55;
                const isBearish = buyRatio < 0.45;

                return (
                  <div key={tf} className="bg-gray-900 rounded p-2">
                    {/* Timeframe Header */}
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-xs text-cyan-400 font-medium uppercase">{tf}</span>
                      <span className={`text-xs font-medium ${
                        tfChange > 0 ? 'text-green-400' : tfChange < 0 ? 'text-red-400' : 'text-gray-400'
                      }`}>
                        {tfChange > 0 ? '+' : ''}{tfChange.toFixed(2)}%
                      </span>
                    </div>

                    {/* OHLC Grid */}
                    <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[11px]">
                      <div className="flex justify-between">
                        <span className="text-gray-500">Open:</span>
                        <span className="text-gray-300 font-mono">{formatPrice(data.open)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-500">High:</span>
                        <span className="text-green-400 font-mono">{formatPrice(data.high)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-500">Close:</span>
                        <span className={`font-mono ${
                          data.close > data.open ? 'text-green-400' : data.close < data.open ? 'text-red-400' : 'text-gray-300'
                        }`}>{formatPrice(data.close)}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-gray-500">Low:</span>
                        <span className="text-red-400 font-mono">{formatPrice(data.low)}</span>
                      </div>
                    </div>

                    {/* Volume & Stats */}
                    <div className="mt-2 pt-2 border-t border-gray-800 grid grid-cols-2 gap-2 text-[10px]">
                      <div>
                        <div className="text-gray-500">Volume</div>
                        <div className="font-medium text-gray-400">
                          {formatVolume(data.volume)}
                        </div>
                      </div>
                      <div>
                        <div className="text-gray-500">Pressure</div>
                        <div className="flex items-center gap-1">
                          {isBullish ? (
                            <>
                              <ArrowUp className="w-3 h-3 text-green-500" />
                              <span className="text-green-400">Buying</span>
                            </>
                          ) : isBearish ? (
                            <>
                              <ArrowDown className="w-3 h-3 text-red-500" />
                              <span className="text-red-400">Selling</span>
                            </>
                          ) : (
                            <span className="text-gray-500">Neutral</span>
                          )}
                        </div>
                      </div>
                      <div>
                        <div className="text-gray-500">Buy/Sell Ratio</div>
                        <div className="text-gray-400">
                          <span className="text-green-400">{(buyRatio * 100).toFixed(0)}%</span>
                          <span className="text-gray-600"> / </span>
                          <span className="text-red-400">{((1 - buyRatio) * 100).toFixed(0)}%</span>
                        </div>
                      </div>
                      <div>
                        <div className="text-gray-500">Volatility</div>
                        <div className={tfVolatility > 1 ? 'text-yellow-400' : 'text-gray-400'}>
                          {tfVolatility.toFixed(2)}%
                        </div>
                      </div>
                      <div>
                        <div className="text-gray-500">Trades</div>
                        <div className="text-gray-400">{data.trade_count.toLocaleString()}</div>
                      </div>
                      <div>
                        <div className="text-gray-500">Quote Vol</div>
                        <div className="text-gray-400">${formatVolume(data.quote_volume)}</div>
                      </div>
                    </div>

                    {/* Bar Status & Countdown */}
                    <div className="mt-1 flex items-center justify-between text-[9px]">
                      <span className="text-gray-600">
                        {data.is_closed_bar ? 'Closed bar' : 'Live bar'}
                      </span>
                      {!data.is_closed_bar && data.close_time && (
                        <CandleCountdown closeTime={data.close_time} />
                      )}
                    </div>
                  </div>
                );
              })}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Coin List component showing all tracked coins
 */
export default function CoinList({ data, isLoading }: CoinListProps) {
  const [sortBy, setSortBy] = useState<'symbol' | 'price' | 'change' | 'volume'>('symbol');
  const [sortAsc, setSortAsc] = useState(true);
  const prevDataRef = useRef<Map<string, CoinData>>(new Map());
  // Single expandable mode: only one coin can be expanded at a time
  const [expandedSymbol, setExpandedSymbol] = useState<string | null>(null);

  // Toggle expansion - only one coin at a time
  const handleToggleExpand = (symbol: string) => {
    setExpandedSymbol(prev => prev === symbol ? null : symbol);
  };

  // Track previous data for comparison
  useEffect(() => {
    if (data?.coins) {
      const newMap = new Map<string, CoinData>();
      data.coins.forEach(coin => newMap.set(coin.symbol, coin));
      prevDataRef.current = newMap;
    }
  }, [data]);

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
    const tfA = getPrimaryTimeframe(a)?.data;
    const tfB = getPrimaryTimeframe(b)?.data;

    switch (sortBy) {
      case 'symbol':
        comparison = a.symbol.localeCompare(b.symbol);
        break;
      case 'price':
        comparison = a.price - b.price;
        break;
      case 'change':
        comparison = calculatePriceChange(tfA) - calculatePriceChange(tfB);
        break;
      case 'volume':
        comparison = (tfA?.volume || 0) - (tfB?.volume || 0);
        break;
    }
    return sortAsc ? comparison : -comparison;
  });

  const handleSort = (column: typeof sortBy) => {
    if (sortBy === column) {
      setSortAsc(!sortAsc);
    } else {
      setSortBy(column);
      setSortAsc(column === 'symbol'); // Default asc for symbol, desc for others
    }
  };

  const SortHeader = ({ column, label, width, align = 'left' }: {
    column: typeof sortBy;
    label: string;
    width: string;
    align?: 'left' | 'right' | 'center';
  }) => (
    <button
      onClick={() => handleSort(column)}
      className={`${width} text-${align} text-[10px] uppercase flex items-center gap-1 hover:text-gray-300 transition-colors ${
        align === 'right' ? 'justify-end' : align === 'center' ? 'justify-center' : ''
      }`}
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
        <SortHeader column="price" label="Price" width="w-24" align="right" />
        <SortHeader column="change" label="Change" width="w-20" align="right" />
        <SortHeader column="volume" label="Volume" width="w-20" align="right" />
        <div className="w-16 text-center text-[10px] uppercase">Source</div>
        <div className="w-14 text-center text-[10px] uppercase">TF</div>
        <div className="w-6" />
      </div>

      {/* Coin Rows */}
      <div className="max-h-80 overflow-y-auto">
        {sortedCoins.map((coin) => (
          <CoinRow
            key={coin.symbol}
            coin={coin}
            prevCoin={prevDataRef.current.get(coin.symbol)}
            isExpanded={expandedSymbol === coin.symbol}
            onToggleExpand={() => handleToggleExpand(coin.symbol)}
          />
        ))}
      </div>

      {/* Footer with Legend */}
      <div className="px-2 py-1.5 bg-gray-800/30 border-t border-gray-700 flex items-center justify-between">
        <span className="text-xs text-gray-500">
          Total: {data.total} coin{data.total !== 1 ? 's' : ''}
        </span>
        <div className="flex items-center gap-3 text-[10px]">
          <div className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-purple-500" />
            <span className="text-gray-500">Strategy</span>
          </div>
          <div className="flex items-center gap-1">
            <span className="w-2 h-2 rounded-full bg-blue-500" />
            <span className="text-gray-500">Position</span>
          </div>
          <div className="flex items-center gap-1">
            <ArrowUp className="w-3 h-3 text-green-500" />
            <span className="text-gray-500">Buy pressure</span>
          </div>
          <div className="flex items-center gap-1">
            <ArrowDown className="w-3 h-3 text-red-500" />
            <span className="text-gray-500">Sell pressure</span>
          </div>
        </div>
      </div>
    </div>
  );
}
