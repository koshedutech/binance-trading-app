import React, { useState, useEffect } from 'react';
import { apiService } from '../services/api';
import type { ScanResult, ProximityResult } from '../types';

const StrategyScanner: React.FC = () => {
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [showWatchlistOnly, setShowWatchlistOnly] = useState(false);
  const [watchlist, setWatchlist] = useState<Set<string>>(new Set());
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());
  const [error, setError] = useState<string | null>(null);

  // Fetch scan results
  const fetchScanResults = async () => {
    try {
      const result = await apiService.getScanResults();
      setScanResult(result);
      setError(null);
    } catch (err) {
      console.error('Failed to fetch scan results:', err);
      setError('Failed to load scan results');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  // Fetch watchlist
  const fetchWatchlist = async () => {
    try {
      const items = await apiService.getWatchlist();
      setWatchlist(new Set(items.map(item => item.symbol)));
    } catch (err) {
      console.error('Failed to fetch watchlist:', err);
    }
  };

  // Initial load
  useEffect(() => {
    fetchScanResults();
    fetchWatchlist();
  }, []);

  // Auto-refresh every 30 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      fetchScanResults();
    }, 30000);

    return () => clearInterval(interval);
  }, []);

  // Manual refresh
  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await apiService.refreshScan();
      // Wait a bit for scan to complete
      setTimeout(() => {
        fetchScanResults();
      }, 2000);
    } catch (err) {
      console.error('Failed to trigger scan:', err);
      setRefreshing(false);
    }
  };

  // Toggle watchlist
  const toggleWatchlist = async (symbol: string) => {
    const isInWatchlist = watchlist.has(symbol);

    try {
      if (isInWatchlist) {
        await apiService.removeFromWatchlist(symbol);
        setWatchlist(prev => {
          const next = new Set(prev);
          next.delete(symbol);
          return next;
        });
      } else {
        await apiService.addToWatchlist(symbol);
        setWatchlist(prev => new Set(prev).add(symbol));
      }
    } catch (err) {
      console.error('Failed to update watchlist:', err);
    }
  };

  // Toggle row expansion
  const toggleRowExpansion = (key: string) => {
    setExpandedRows(prev => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  // Get readiness color
  const getReadinessColor = (score: number): string => {
    if (score >= 80) return 'bg-green-500/10 text-green-500 border-green-500/20';
    if (score >= 50) return 'bg-yellow-500/10 text-yellow-500 border-yellow-500/20';
    return 'bg-red-500/10 text-red-500 border-red-500/20';
  };

  // Format time prediction
  const formatTimePrediction = (result: ProximityResult): string => {
    if (!result.time_prediction) return '-';
    const { min_minutes, max_minutes } = result.time_prediction;
    if (min_minutes < 60) {
      return `${min_minutes}-${max_minutes}m`;
    }
    const minHours = Math.floor(min_minutes / 60);
    const maxHours = Math.floor(max_minutes / 60);
    return `${minHours}-${maxHours}h`;
  };

  // Filter results
  const filteredResults = scanResult?.results.filter(result => {
    if (showWatchlistOnly) {
      return watchlist.has(result.symbol);
    }
    return true;
  }) || [];

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-gray-400">Loading scanner results...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="text-red-400">{error}</div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Header Controls */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div className="text-sm text-gray-400">
            Last scan: {scanResult?.symbols_scanned || 0} symbols in{' '}
            {scanResult?.duration ? (scanResult.duration / 1e9).toFixed(1) : 0}s
          </div>
          <label className="flex items-center gap-2 text-sm cursor-pointer">
            <input
              type="checkbox"
              checked={showWatchlistOnly}
              onChange={e => setShowWatchlistOnly(e.target.checked)}
              className="rounded bg-gray-700 border-gray-600 text-blue-500 focus:ring-blue-500"
            />
            <span className="text-gray-300">Watchlist Only</span>
          </label>
        </div>

        <button
          onClick={handleRefresh}
          disabled={refreshing}
          className="px-4 py-2 bg-blue-500 hover:bg-blue-600 disabled:bg-gray-600 disabled:cursor-not-allowed rounded text-white text-sm transition-colors"
        >
          {refreshing ? 'Refreshing...' : 'Refresh Now'}
        </button>
      </div>

      {/* Results Table */}
      {filteredResults.length === 0 ? (
        <div className="text-center py-12 text-gray-400">
          {showWatchlistOnly
            ? 'No watchlist symbols found. Add symbols to your watchlist to track them.'
            : 'No opportunities found. Scanner will continue monitoring all symbols.'}
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left p-3 font-medium text-gray-300">Symbol</th>
                <th className="text-left p-3 font-medium text-gray-300">Strategy</th>
                <th className="text-right p-3 font-medium text-gray-300">Distance</th>
                <th className="text-center p-3 font-medium text-gray-300">Conditions</th>
                <th className="text-center p-3 font-medium text-gray-300">Readiness</th>
                <th className="text-center p-3 font-medium text-gray-300">Est. Time</th>
                <th className="text-center p-3 font-medium text-gray-300">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredResults.map(result => {
                const rowKey = `${result.symbol}_${result.strategy_name}`;
                const isExpanded = expandedRows.has(rowKey);
                const isInWatchlist = watchlist.has(result.symbol);

                return (
                  <React.Fragment key={rowKey}>
                    <tr className="border-b border-gray-800 hover:bg-gray-800/50 transition-colors">
                      {/* Symbol */}
                      <td className="p-3">
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => toggleWatchlist(result.symbol)}
                            className={`text-lg ${
                              isInWatchlist ? 'text-yellow-400' : 'text-gray-600 hover:text-gray-400'
                            }`}
                            title={isInWatchlist ? 'Remove from watchlist' : 'Add to watchlist'}
                          >
                            {isInWatchlist ? '★' : '☆'}
                          </button>
                          <span className="font-medium text-white">{result.symbol}</span>
                        </div>
                      </td>

                      {/* Strategy */}
                      <td className="p-3 text-gray-300">{result.strategy_name}</td>

                      {/* Distance */}
                      <td className="p-3 text-right">
                        <div className="text-white">{result.distance_percent.toFixed(2)}%</div>
                        <div className="text-xs text-gray-400">
                          ${Math.abs(result.distance_absolute).toFixed(2)}
                        </div>
                      </td>

                      {/* Conditions */}
                      <td className="p-3">
                        <div className="flex flex-col items-center gap-1">
                          <div className="text-white">
                            {result.conditions.met_conditions}/{result.conditions.total_conditions}
                          </div>
                          <div className="w-full bg-gray-700 rounded-full h-1.5">
                            <div
                              className="bg-blue-500 h-1.5 rounded-full transition-all"
                              style={{
                                width: `${
                                  (result.conditions.met_conditions /
                                    result.conditions.total_conditions) *
                                  100
                                }%`,
                              }}
                            />
                          </div>
                        </div>
                      </td>

                      {/* Readiness Score */}
                      <td className="p-3">
                        <div className="flex justify-center">
                          <span
                            className={`px-3 py-1 rounded-full border text-sm font-medium ${getReadinessColor(
                              result.readiness_score
                            )}`}
                          >
                            {result.readiness_score.toFixed(0)}%
                          </span>
                        </div>
                      </td>

                      {/* Time Prediction */}
                      <td className="p-3 text-center text-gray-300">
                        {formatTimePrediction(result)}
                      </td>

                      {/* Actions */}
                      <td className="p-3 text-center">
                        <button
                          onClick={() => toggleRowExpansion(rowKey)}
                          className="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded text-xs transition-colors"
                        >
                          {isExpanded ? 'Hide' : 'Details'}
                        </button>
                      </td>
                    </tr>

                    {/* Expanded Row - Conditions Details */}
                    {isExpanded && (
                      <tr className="bg-gray-800/30">
                        <td colSpan={7} className="p-4">
                          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            {/* Left Column - Price Info */}
                            <div className="space-y-2">
                              <h4 className="font-medium text-white mb-2">Price Information</h4>
                              <div className="grid grid-cols-2 gap-2 text-sm">
                                <div className="text-gray-400">Current Price:</div>
                                <div className="text-white">${result.current_price.toFixed(2)}</div>

                                <div className="text-gray-400">Target Price:</div>
                                <div className="text-white">${result.target_price.toFixed(2)}</div>

                                <div className="text-gray-400">Trend:</div>
                                <div
                                  className={`${
                                    result.trend_direction === 'BULLISH'
                                      ? 'text-green-400'
                                      : result.trend_direction === 'BEARISH'
                                      ? 'text-red-400'
                                      : 'text-gray-400'
                                  }`}
                                >
                                  {result.trend_direction}
                                </div>

                                {result.time_prediction && (
                                  <>
                                    <div className="text-gray-400">Confidence:</div>
                                    <div className="text-white">
                                      {(result.time_prediction.confidence * 100).toFixed(0)}%
                                    </div>
                                  </>
                                )}
                              </div>
                            </div>

                            {/* Right Column - Conditions Breakdown */}
                            <div className="space-y-2">
                              <h4 className="font-medium text-white mb-2">Conditions Breakdown</h4>
                              <div className="space-y-1">
                                {result.conditions.details.map((condition, idx) => (
                                  <div key={idx} className="flex items-start gap-2 text-sm">
                                    <span
                                      className={`mt-0.5 ${
                                        condition.met ? 'text-green-400' : 'text-red-400'
                                      }`}
                                    >
                                      {condition.met ? '✓' : '✗'}
                                    </span>
                                    <div className="flex-1">
                                      <div
                                        className={`${
                                          condition.met ? 'text-white' : 'text-gray-400'
                                        }`}
                                      >
                                        {condition.name}
                                      </div>
                                      <div className="text-xs text-gray-500">
                                        {condition.description}
                                      </div>
                                    </div>
                                  </div>
                                ))}
                              </div>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default StrategyScanner;
