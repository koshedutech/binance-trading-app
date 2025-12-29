import { useEffect, useState } from 'react';
import {
  futuresApi,
  formatUSD,
  SymbolPerformanceSettings,
  SymbolPerformanceReport
} from '../services/futuresApi';
import { TrendingUp, TrendingDown, AlertTriangle, Ban, Check, RefreshCw, ChevronDown, ChevronUp, Settings, Filter, Shield, Target, Clock, Lock, Unlock } from 'lucide-react';

interface SymbolPerformanceData {
  symbols: Record<string, SymbolPerformanceSettings>;
  category_config: {
    confidence_boost: Record<string, number>;
    size_multiplier: Record<string, number>;
  };
  global_min_confidence: number;
  global_max_usd: number;
}

const categoryColors: Record<string, string> = {
  best: 'bg-green-100 text-green-800 border-green-200',
  good: 'bg-blue-100 text-blue-800 border-blue-200',
  neutral: 'bg-gray-100 text-gray-800 border-gray-200',
  poor: 'bg-yellow-100 text-yellow-800 border-yellow-200',
  worst: 'bg-orange-100 text-orange-800 border-orange-200',
  blacklist: 'bg-red-100 text-red-800 border-red-200',
};

const categoryIcons: Record<string, JSX.Element> = {
  best: <TrendingUp className="w-4 h-4 text-green-600" />,
  good: <TrendingUp className="w-4 h-4 text-blue-600" />,
  neutral: <Settings className="w-4 h-4 text-gray-600" />,
  poor: <TrendingDown className="w-4 h-4 text-yellow-600" />,
  worst: <AlertTriangle className="w-4 h-4 text-orange-600" />,
  blacklist: <Ban className="w-4 h-4 text-red-600" />,
};

const categoryDescriptions: Record<string, string> = {
  best: 'Top performers: +50% size, -5% confidence',
  good: 'Above average: +20% size',
  neutral: 'Standard settings',
  poor: 'Below average: 50% size, +10% confidence',
  worst: 'Restricted: 25% size, +20% confidence',
  blacklist: 'Trading disabled',
};

interface BlockedSymbol {
  symbol: string;
  blocked_until: string;
  reason: string;
  remaining: string;
}

export default function SymbolPerformancePanel() {
  const [data, setData] = useState<SymbolPerformanceData | null>(null);
  const [report, setReport] = useState<SymbolPerformanceReport[]>([]);
  const [blockedSymbols, setBlockedSymbols] = useState<BlockedSymbol[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [autoBlocking, setAutoBlocking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [sortBy, setSortBy] = useState<'pnl' | 'winrate' | 'trades'>('pnl');
  const [sortAsc, setSortAsc] = useState(false);
  const [expandedSymbol, setExpandedSymbol] = useState<string | null>(null);
  const [updating, setUpdating] = useState<string | null>(null);

  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);
      const [settingsRes, reportRes, blockedRes] = await Promise.all([
        futuresApi.getSymbolPerformanceSettings(),
        futuresApi.getSymbolPerformanceReport(),
        futuresApi.getBlockedSymbols(),
      ]);
      setData(settingsRes);
      setReport(reportRes.report || []);
      setBlockedSymbols(blockedRes.blocked_symbols || []);
    } catch (err) {
      console.error('Failed to fetch symbol settings:', err);
      const errorMsg = err instanceof Error ? err.message : 'Failed to fetch symbol performance data';
      setError(errorMsg);
    } finally {
      setLoading(false);
    }
  };

  const isSymbolBlocked = (symbol: string) => {
    return blockedSymbols.some(b => b.symbol === symbol);
  };

  const getBlockedInfo = (symbol: string) => {
    return blockedSymbols.find(b => b.symbol === symbol);
  };

  const handleBlockForDay = async (symbol: string, reason?: string) => {
    try {
      setUpdating(symbol);
      await futuresApi.blockSymbolForDay(symbol, reason || 'manual_block');
      await fetchData();
    } catch (error) {
      console.error('Failed to block symbol:', error);
    } finally {
      setUpdating(null);
    }
  };

  const handleUnblock = async (symbol: string) => {
    try {
      setUpdating(symbol);
      await futuresApi.unblockSymbol(symbol);
      await fetchData();
    } catch (error) {
      console.error('Failed to unblock symbol:', error);
    } finally {
      setUpdating(null);
    }
  };

  const handleAutoBlockWorst = async () => {
    try {
      setAutoBlocking(true);
      const result = await futuresApi.autoBlockWorstPerformers();
      console.log('Auto-blocked:', result);
      await fetchData();
    } catch (error) {
      console.error('Failed to auto-block worst performers:', error);
    } finally {
      setAutoBlocking(false);
    }
  };

  const refreshFromDatabase = async () => {
    try {
      setRefreshing(true);
      setError(null);
      const result = await futuresApi.refreshSymbolPerformance();
      setReport(result.report || []);
      // Also refresh settings data
      const settingsRes = await futuresApi.getSymbolPerformanceSettings();
      setData(settingsRes);
    } catch (err) {
      console.error('Failed to refresh symbol performance:', err);
      const errorMsg = err instanceof Error ? err.message : 'Failed to refresh performance data';
      setError(errorMsg);
    } finally {
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 60000); // Refresh every minute
    return () => clearInterval(interval);
  }, []);

  const handleBlacklist = async (symbol: string) => {
    try {
      setUpdating(symbol);
      await futuresApi.blacklistSymbol(symbol, 'Poor performance');
      await fetchData();
    } catch (error) {
      console.error('Failed to blacklist symbol:', error);
    } finally {
      setUpdating(null);
    }
  };

  const handleUnblacklist = async (symbol: string) => {
    try {
      setUpdating(symbol);
      await futuresApi.unblacklistSymbol(symbol);
      await fetchData();
    } catch (error) {
      console.error('Failed to unblacklist symbol:', error);
    } finally {
      setUpdating(null);
    }
  };

  const handleCategoryChange = async (symbol: string, category: string) => {
    try {
      setUpdating(symbol);
      await futuresApi.updateSymbolSettings(symbol, {
        category,
        enabled: category !== 'blacklist',
        size_multiplier: 1.0,
      });
      await fetchData();
    } catch (error) {
      console.error('Failed to update symbol category:', error);
    } finally {
      setUpdating(null);
    }
  };

  // Filter and sort report
  const filteredReport = report
    .filter(r => selectedCategory === 'all' || r.category === selectedCategory)
    .sort((a, b) => {
      let comparison = 0;
      switch (sortBy) {
        case 'pnl':
          comparison = a.total_pnl - b.total_pnl;
          break;
        case 'winrate':
          comparison = a.win_rate - b.win_rate;
          break;
        case 'trades':
          comparison = a.total_trades - b.total_trades;
          break;
      }
      return sortAsc ? comparison : -comparison;
    });

  // Count by category
  const categoryCounts = report.reduce((acc, r) => {
    acc[r.category] = (acc[r.category] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  if (loading && !data) {
    return (
      <div className="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
        <div className="flex items-center justify-center">
          <RefreshCw className="w-6 h-6 animate-spin text-gray-400" />
          <span className="ml-2 text-gray-500">Loading symbol settings...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg shadow-sm border border-gray-200">
      {/* Error Banner */}
      {error && (
        <div className="p-4 bg-red-50 border-b border-red-200">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-5 h-5 text-red-500" />
            <span className="text-red-700">{error}</span>
            <button
              onClick={() => setError(null)}
              className="ml-auto text-red-500 hover:text-red-700"
            >
              ✕
            </button>
          </div>
        </div>
      )}
      {/* Header */}
      <div className="p-4 border-b border-gray-200">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Target className="w-5 h-5 text-indigo-600" />
            <h2 className="text-lg font-semibold text-gray-900">Symbol Performance Settings</h2>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={handleAutoBlockWorst}
              disabled={autoBlocking}
              className="px-3 py-1.5 text-sm bg-orange-600 text-white hover:bg-orange-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg flex items-center gap-1"
              title="Block all 'worst' category symbols for the day"
            >
              <Lock className={`w-4 h-4 ${autoBlocking ? 'animate-pulse' : ''}`} />
              {autoBlocking ? 'Blocking...' : 'Auto Block Worst'}
            </button>
            <button
              onClick={refreshFromDatabase}
              disabled={refreshing}
              className="px-3 py-1.5 text-sm bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg flex items-center gap-1"
              title="Recalculate categories from database trades"
            >
              <TrendingUp className={`w-4 h-4 ${refreshing ? 'animate-pulse' : ''}`} />
              {refreshing ? 'Recalculating...' : 'Recalculate'}
            </button>
            <button
              onClick={fetchData}
              className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg"
              title="Refresh view"
            >
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>
        <p className="text-sm text-gray-500 mt-1">
          Adjust confidence thresholds and position sizes based on symbol performance
        </p>
      </div>

      {/* Category Summary */}
      <div className="p-4 border-b border-gray-200 bg-gray-50">
        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => setSelectedCategory('all')}
            className={`px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
              selectedCategory === 'all'
                ? 'bg-indigo-100 text-indigo-800 border-2 border-indigo-300'
                : 'bg-white text-gray-600 border border-gray-300 hover:bg-gray-100'
            }`}
          >
            All ({report.length})
          </button>
          {['best', 'good', 'neutral', 'poor', 'worst', 'blacklist'].map(cat => (
            <button
              key={cat}
              onClick={() => setSelectedCategory(cat)}
              className={`px-3 py-1.5 rounded-full text-sm font-medium flex items-center gap-1 transition-colors ${
                selectedCategory === cat
                  ? `${categoryColors[cat]} border-2`
                  : 'bg-white text-gray-600 border border-gray-300 hover:bg-gray-100'
              }`}
            >
              {categoryIcons[cat]}
              <span className="capitalize">{cat}</span>
              <span className="text-xs opacity-75">({categoryCounts[cat] || 0})</span>
            </button>
          ))}
        </div>
      </div>

      {/* Global Settings */}
      {data && (
        <div className="p-4 border-b border-gray-200 bg-blue-50">
          <div className="flex items-center gap-4 text-sm flex-wrap">
            <div className="flex items-center gap-1">
              <Shield className="w-4 h-4 text-blue-600" />
              <span className="text-gray-600">Global Min Confidence:</span>
              <span className="font-semibold text-blue-800">{data.global_min_confidence}%</span>
            </div>
            <div className="flex items-center gap-1">
              <Target className="w-4 h-4 text-blue-600" />
              <span className="text-gray-600">Global Max Position:</span>
              <span className="font-semibold text-blue-800">{formatUSD(data.global_max_usd)}</span>
            </div>
            {blockedSymbols.length > 0 && (
              <div className="flex items-center gap-1 bg-orange-100 px-2 py-1 rounded-full">
                <Lock className="w-4 h-4 text-orange-600" />
                <span className="text-orange-700 font-semibold">{blockedSymbols.length} Blocked Today</span>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Sort Controls */}
      <div className="p-3 border-b border-gray-200 flex items-center gap-4 text-sm">
        <Filter className="w-4 h-4 text-gray-400" />
        <span className="text-gray-500">Sort by:</span>
        {[
          { key: 'pnl', label: 'PnL' },
          { key: 'winrate', label: 'Win Rate' },
          { key: 'trades', label: 'Trades' },
        ].map(({ key, label }) => (
          <button
            key={key}
            onClick={() => {
              if (sortBy === key) {
                setSortAsc(!sortAsc);
              } else {
                setSortBy(key as 'pnl' | 'winrate' | 'trades');
                setSortAsc(false);
              }
            }}
            className={`flex items-center gap-1 px-2 py-1 rounded ${
              sortBy === key ? 'bg-indigo-100 text-indigo-700' : 'text-gray-600 hover:bg-gray-100'
            }`}
          >
            {label}
            {sortBy === key && (sortAsc ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />)}
          </button>
        ))}
      </div>

      {/* Symbol List */}
      <div className="divide-y divide-gray-100 max-h-96 overflow-y-auto">
        {filteredReport.length === 0 ? (
          <div className="p-8 text-center text-gray-500">
            No symbols found{selectedCategory !== 'all' ? ` in ${selectedCategory} category` : ''}
          </div>
        ) : (
          filteredReport.map(sym => (
            <div key={sym.symbol} className="hover:bg-gray-50">
              {/* Main Row */}
              <div
                className="p-3 flex items-center justify-between cursor-pointer"
                onClick={() => setExpandedSymbol(expandedSymbol === sym.symbol ? null : sym.symbol)}
              >
                <div className="flex items-center gap-3">
                  <div className={`px-2 py-1 rounded-full text-xs font-medium flex items-center gap-1 ${categoryColors[sym.category]}`}>
                    {categoryIcons[sym.category]}
                    <span className="capitalize">{sym.category}</span>
                  </div>
                  <span className="font-semibold text-gray-900">{sym.symbol}</span>
                  {isSymbolBlocked(sym.symbol) && (
                    <span className="px-2 py-0.5 bg-orange-100 text-orange-700 text-xs rounded-full flex items-center gap-1" title={`Blocked until ${getBlockedInfo(sym.symbol)?.remaining || 'end of day'}`}>
                      <Clock className="w-3 h-3" />
                      Blocked
                    </span>
                  )}
                  {!sym.enabled && (
                    <span className="px-2 py-0.5 bg-red-100 text-red-600 text-xs rounded-full">Disabled</span>
                  )}
                </div>

                <div className="flex items-center gap-4 text-sm">
                  <div className="text-right">
                    <div className={`font-semibold ${sym.total_pnl >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                      {sym.total_pnl >= 0 ? '+' : ''}{formatUSD(sym.total_pnl)}
                    </div>
                    <div className="text-xs text-gray-500">{sym.total_trades} trades</div>
                  </div>
                  <div className="text-right w-16">
                    <div className={`font-semibold ${sym.win_rate >= 50 ? 'text-green-600' : 'text-red-600'}`}>
                      {sym.win_rate.toFixed(1)}%
                    </div>
                    <div className="text-xs text-gray-500">win rate</div>
                  </div>
                  <div className="text-right w-20">
                    <div className="font-medium text-gray-700">{sym.min_confidence.toFixed(0)}%</div>
                    <div className="text-xs text-gray-500">min conf</div>
                  </div>
                  <div className="text-right w-16">
                    <div className="font-medium text-gray-700">{formatUSD(sym.max_position_usd)}</div>
                    <div className="text-xs text-gray-500">max size</div>
                  </div>
                  {expandedSymbol === sym.symbol ? (
                    <ChevronUp className="w-4 h-4 text-gray-400" />
                  ) : (
                    <ChevronDown className="w-4 h-4 text-gray-400" />
                  )}
                </div>
              </div>

              {/* Expanded Details */}
              {expandedSymbol === sym.symbol && (
                <div className="px-4 pb-4 pt-2 bg-gray-50 border-t border-gray-100">
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-4">
                    <div className="bg-white p-3 rounded-lg border border-gray-200">
                      <div className="text-xs text-gray-500 mb-1">Winning Trades</div>
                      <div className="font-semibold text-green-600">{sym.winning_trades}</div>
                    </div>
                    <div className="bg-white p-3 rounded-lg border border-gray-200">
                      <div className="text-xs text-gray-500 mb-1">Losing Trades</div>
                      <div className="font-semibold text-red-600">{sym.losing_trades}</div>
                    </div>
                    <div className="bg-white p-3 rounded-lg border border-gray-200">
                      <div className="text-xs text-gray-500 mb-1">Avg PnL</div>
                      <div className={`font-semibold ${sym.avg_pnl >= 0 ? 'text-green-600' : 'text-red-600'}`}>
                        {formatUSD(sym.avg_pnl)}
                      </div>
                    </div>
                    <div className="bg-white p-3 rounded-lg border border-gray-200">
                      <div className="text-xs text-gray-500 mb-1">Size Multiplier</div>
                      <div className="font-semibold text-gray-700">{sym.size_multiplier.toFixed(2)}x</div>
                    </div>
                  </div>

                  {/* Category Description */}
                  <div className="text-sm text-gray-600 mb-3 p-2 bg-white rounded border border-gray-200">
                    <strong>Category Effect:</strong> {categoryDescriptions[sym.category]}
                  </div>

                  {/* Actions */}
                  <div className="flex items-center gap-2">
                    <select
                      value={sym.category}
                      onChange={(e) => handleCategoryChange(sym.symbol, e.target.value)}
                      disabled={updating === sym.symbol}
                      className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm focus:ring-2 focus:ring-indigo-500"
                    >
                      <option value="best">Best</option>
                      <option value="good">Good</option>
                      <option value="neutral">Neutral</option>
                      <option value="poor">Poor</option>
                      <option value="worst">Worst</option>
                      <option value="blacklist">Blacklist</option>
                    </select>

                    {/* Block/Unblock for Day */}
                    {isSymbolBlocked(sym.symbol) ? (
                      <button
                        onClick={() => handleUnblock(sym.symbol)}
                        disabled={updating === sym.symbol}
                        className="flex items-center gap-1 px-3 py-1.5 bg-green-100 text-green-700 rounded-lg text-sm hover:bg-green-200 disabled:opacity-50"
                        title={`Blocked until ${getBlockedInfo(sym.symbol)?.remaining}`}
                      >
                        <Unlock className="w-4 h-4" />
                        Unblock
                      </button>
                    ) : (
                      <button
                        onClick={() => handleBlockForDay(sym.symbol, 'worst_performer_manual')}
                        disabled={updating === sym.symbol}
                        className="flex items-center gap-1 px-3 py-1.5 bg-orange-100 text-orange-700 rounded-lg text-sm hover:bg-orange-200 disabled:opacity-50"
                        title="Block this symbol for the rest of the day"
                      >
                        <Lock className="w-4 h-4" />
                        Block Day
                      </button>
                    )}

                    {/* Blacklist (permanent) */}
                    {sym.category !== 'blacklist' ? (
                      <button
                        onClick={() => handleBlacklist(sym.symbol)}
                        disabled={updating === sym.symbol}
                        className="flex items-center gap-1 px-3 py-1.5 bg-red-100 text-red-700 rounded-lg text-sm hover:bg-red-200 disabled:opacity-50"
                      >
                        <Ban className="w-4 h-4" />
                        Blacklist
                      </button>
                    ) : (
                      <button
                        onClick={() => handleUnblacklist(sym.symbol)}
                        disabled={updating === sym.symbol}
                        className="flex items-center gap-1 px-3 py-1.5 bg-green-100 text-green-700 rounded-lg text-sm hover:bg-green-200 disabled:opacity-50"
                      >
                        <Check className="w-4 h-4" />
                        Enable
                      </button>
                    )}

                    {updating === sym.symbol && (
                      <RefreshCw className="w-4 h-4 animate-spin text-gray-400" />
                    )}
                  </div>
                </div>
              )}
            </div>
          ))
        )}
      </div>

      {/* Category Legend */}
      <div className="p-4 border-t border-gray-200 bg-gray-50">
        <div className="text-xs text-gray-500 mb-2">Category Effects:</div>
        <div className="grid grid-cols-2 md:grid-cols-3 gap-2 text-xs">
          {Object.entries(categoryDescriptions).map(([cat, desc]) => (
            <div key={cat} className="flex items-start gap-1">
              {categoryIcons[cat]}
              <span className="text-gray-600">{desc}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
