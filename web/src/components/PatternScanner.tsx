import React, { useState, useEffect } from 'react';
import { Search, Sparkles, TrendingUp, TrendingDown, AlertCircle, Loader, ShoppingCart, LogOut, BarChart3, Star, DollarSign } from 'lucide-react';
import { apiService } from '../services/api';
import { ChartModal } from './ChartModal';

interface CandlestickPattern {
  Name: string;
  Type: string;
  Reliability: string;
  Description: string;
}

interface TimeframePatternResult {
  interval: string;
  patterns: CandlestickPattern[];
}

interface SymbolPatternResult {
  symbol: string;
  timeframes: TimeframePatternResult[];
}

const ALL_TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d'];

export const PatternScanner: React.FC = () => {
  const [selectedSymbols, setSelectedSymbols] = useState<string[]>([]);
  const [selectedTimeframes, setSelectedTimeframes] = useState<string[]>(['5m', '15m', '1h', '4h']);
  const [symbolInput, setSymbolInput] = useState('');
  const [results, setResults] = useState<SymbolPatternResult[]>([]);
  const [scanning, setScanning] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [chartModal, setChartModal] = useState<{ isOpen: boolean; symbol: string; interval: string }>({
    isOpen: false,
    symbol: '',
    interval: '1h',
  });
  const [buyingSymbol, setBuyingSymbol] = useState<string | null>(null);
  const [sellingSymbol, setSellingSymbol] = useState<string | null>(null);
  const [closingAll, setClosingAll] = useState(false);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [watchlist, setWatchlist] = useState<string[]>([]);

  useEffect(() => {
    // Load watchlist on component mount
    const loadWatchlist = async () => {
      try {
        const items = await apiService.getWatchlist();
        setWatchlist(items.map((item: any) => item.symbol));
      } catch (err) {
        console.error('Failed to load watchlist:', err);
      }
    };
    loadWatchlist();
  }, []);

  const addSymbol = () => {
    const symbol = symbolInput.trim().toUpperCase();
    if (symbol && !selectedSymbols.includes(symbol)) {
      // Add USDT suffix if not present
      const finalSymbol = symbol.endsWith('USDT') ? symbol : symbol + 'USDT';
      setSelectedSymbols([...selectedSymbols, finalSymbol]);
      setSymbolInput('');
    }
  };

  const removeSymbol = (symbol: string) => {
    setSelectedSymbols(selectedSymbols.filter((s) => s !== symbol));
  };

  const toggleTimeframe = (timeframe: string) => {
    if (selectedTimeframes.includes(timeframe)) {
      setSelectedTimeframes(selectedTimeframes.filter((t) => t !== timeframe));
    } else {
      setSelectedTimeframes([...selectedTimeframes, timeframe]);
    }
  };

  const handleScan = async () => {
    if (selectedSymbols.length === 0) {
      setError('Please add at least one symbol');
      return;
    }

    if (selectedTimeframes.length === 0) {
      setError('Please select at least one timeframe');
      return;
    }

    setScanning(true);
    setError(null);

    try {
      const response = await apiService.scanPatterns({
        symbols: selectedSymbols,
        intervals: selectedTimeframes,
      });

      setResults(response);
    } catch (err: any) {
      setError(err.message || 'Failed to scan patterns');
    } finally {
      setScanning(false);
    }
  };

  const getPatternIcon = (type: string) => {
    if (type === 'BULLISH') return <TrendingUp className="w-4 h-4 text-green-500" />;
    if (type === 'BEARISH') return <TrendingDown className="w-4 h-4 text-red-500" />;
    return <AlertCircle className="w-4 h-4 text-yellow-500" />;
  };

  const getPatternColor = (type: string) => {
    if (type === 'BULLISH') return 'bg-green-900/30 border-green-500/50 text-green-400';
    if (type === 'BEARISH') return 'bg-red-900/30 border-red-500/50 text-red-400';
    return 'bg-yellow-900/30 border-yellow-500/50 text-yellow-400';
  };

  const getReliabilityBadge = (reliability: string) => {
    const colors = {
      HIGH: 'bg-green-600 text-white',
      MEDIUM: 'bg-yellow-600 text-white',
      LOW: 'bg-gray-600 text-white',
    };
    return colors[reliability as keyof typeof colors] || 'bg-gray-600 text-white';
  };

  const openChart = (symbol: string, interval: string) => {
    setChartModal({ isOpen: true, symbol, interval });
  };

  const closeChart = () => {
    setChartModal({ isOpen: false, symbol: '', interval: '1h' });
  };

  const handleBuy = async (symbol: string) => {
    setBuyingSymbol(symbol);
    setError(null);
    setSuccessMessage(null);

    try {
      // Place a market buy order with a small quantity (adjust as needed)
      await apiService.placeOrder({
        symbol,
        side: 'BUY',
        order_type: 'MARKET',
        quantity: 0.001, // Small test quantity - adjust based on your needs
        price: 0,
      });

      setSuccessMessage(`Successfully placed buy order for ${symbol}`);
      setTimeout(() => setSuccessMessage(null), 5000);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to place order');
    } finally {
      setBuyingSymbol(null);
    }
  };

  const handleSell = async (symbol: string) => {
    setSellingSymbol(symbol);
    setError(null);
    setSuccessMessage(null);

    try {
      // Place a market sell order with a small quantity
      await apiService.placeOrder({
        symbol,
        side: 'SELL',
        order_type: 'MARKET',
        quantity: 0.001, // Small test quantity - adjust based on your needs
        price: 0,
      });

      setSuccessMessage(`Successfully placed sell order for ${symbol}`);
      setTimeout(() => setSuccessMessage(null), 5000);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to place order');
    } finally {
      setSellingSymbol(null);
    }
  };

  const handleExitAll = async () => {
    if (!confirm('Are you sure you want to close ALL open positions?')) {
      return;
    }

    setClosingAll(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const result = await apiService.closeAllPositions();
      setSuccessMessage(result.message);

      if (result.errors && result.errors.length > 0) {
        setError(`Some positions failed to close: ${result.errors.join(', ')}`);
      }

      setTimeout(() => setSuccessMessage(null), 5000);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to close positions');
    } finally {
      setClosingAll(false);
    }
  };

  const toggleWatchlist = async (symbol: string) => {
    try {
      if (watchlist.includes(symbol)) {
        await apiService.removeFromWatchlist(symbol);
        setWatchlist(watchlist.filter((s) => s !== symbol));
        setSuccessMessage(`Removed ${symbol} from watchlist`);
      } else {
        await apiService.addToWatchlist(symbol);
        setWatchlist([...watchlist, symbol]);
        setSuccessMessage(`Added ${symbol} to watchlist`);
      }
      setTimeout(() => setSuccessMessage(null), 3000);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to update watchlist');
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-white flex items-center gap-2">
            <Sparkles className="w-6 h-6 text-purple-500" />
            Multi-Timeframe Pattern Scanner
          </h2>
          <p className="text-gray-400 text-sm mt-1">
            Scan multiple symbols across different timeframes for candlestick patterns
          </p>
        </div>
        <button
          onClick={handleExitAll}
          disabled={closingAll}
          className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded-lg font-semibold flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-lg hover:shadow-xl"
        >
          {closingAll ? (
            <>
              <Loader className="w-4 h-4 animate-spin" />
              Closing...
            </>
          ) : (
            <>
              <LogOut className="w-4 h-4" />
              Exit All Positions
            </>
          )}
        </button>
      </div>

      {/* Configuration */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Symbol Selection */}
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <h3 className="text-lg font-semibold text-white mb-4">Select Symbols</h3>

          <div className="flex gap-2 mb-4">
            <input
              type="text"
              value={symbolInput}
              onChange={(e) => setSymbolInput(e.target.value.toUpperCase())}
              onKeyPress={(e) => e.key === 'Enter' && addSymbol()}
              placeholder="Enter symbol (e.g., BTC, ETH)"
              className="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
            />
            <button
              onClick={addSymbol}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors"
            >
              Add
            </button>
          </div>

          <div className="flex flex-wrap gap-2">
            {selectedSymbols.length === 0 ? (
              <div className="text-gray-500 text-sm">No symbols selected</div>
            ) : (
              selectedSymbols.map((symbol) => (
                <div
                  key={symbol}
                  className="flex items-center gap-2 px-3 py-1 bg-blue-900/30 border border-blue-500/50 rounded text-blue-400"
                >
                  <span>{symbol}</span>
                  <button onClick={() => removeSymbol(symbol)} className="hover:text-red-400">
                    ×
                  </button>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Timeframe Selection */}
        <div className="bg-gray-800 rounded-lg p-6 border border-gray-700">
          <h3 className="text-lg font-semibold text-white mb-4">Select Timeframes</h3>

          <div className="grid grid-cols-4 gap-2">
            {ALL_TIMEFRAMES.map((timeframe) => (
              <button
                key={timeframe}
                onClick={() => toggleTimeframe(timeframe)}
                className={`px-3 py-2 rounded transition-colors ${
                  selectedTimeframes.includes(timeframe)
                    ? 'bg-purple-600 text-white'
                    : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
                }`}
              >
                {timeframe}
              </button>
            ))}
          </div>

          <div className="mt-4 text-sm text-gray-400">
            {selectedTimeframes.length} timeframe{selectedTimeframes.length !== 1 ? 's' : ''} selected
          </div>
        </div>
      </div>

      {/* Scan Button */}
      <div className="flex justify-center">
        <button
          onClick={handleScan}
          disabled={scanning || selectedSymbols.length === 0 || selectedTimeframes.length === 0}
          className="px-8 py-3 bg-gradient-to-r from-purple-600 to-blue-600 hover:from-purple-700 hover:to-blue-700 text-white rounded-lg font-semibold flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed transition-all shadow-lg hover:shadow-xl"
        >
          {scanning ? (
            <>
              <Loader className="w-5 h-5 animate-spin" />
              Scanning...
            </>
          ) : (
            <>
              <Search className="w-5 h-5" />
              Scan Patterns
            </>
          )}
        </button>
      </div>

      {/* Success Message */}
      {successMessage && (
        <div className="bg-green-900/30 border border-green-500/50 rounded-lg p-4 text-green-400">
          {successMessage}
        </div>
      )}

      {/* Error Message */}
      {error && (
        <div className="bg-red-900/30 border border-red-500/50 rounded-lg p-4 text-red-400">
          {error}
        </div>
      )}

      {/* Results */}
      {results.length > 0 && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-xl font-semibold text-white">
              Scan Results ({results.length} symbol{results.length !== 1 ? 's' : ''} with patterns)
            </h3>
            <div className="text-sm text-gray-400">
              Total patterns found:{' '}
              {results.reduce((sum, r) => sum + r.timeframes.reduce((s, t) => s + t.patterns.length, 0), 0)}
            </div>
          </div>

          {results.map((symbolResult) => (
            <div key={symbolResult.symbol} className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
              <div className="bg-gradient-to-r from-gray-700 to-gray-800 px-6 py-4 border-b border-gray-600">
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="text-lg font-bold text-white">{symbolResult.symbol}</h4>
                    <div className="text-sm text-gray-400">
                      Patterns found in {symbolResult.timeframes.length} timeframe
                      {symbolResult.timeframes.length !== 1 ? 's' : ''}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => toggleWatchlist(symbolResult.symbol)}
                      className={`p-2 rounded transition-colors ${
                        watchlist.includes(symbolResult.symbol)
                          ? 'bg-yellow-600 hover:bg-yellow-700 text-white'
                          : 'bg-gray-700 hover:bg-gray-600 text-gray-400'
                      }`}
                      title={watchlist.includes(symbolResult.symbol) ? 'Remove from watchlist' : 'Add to watchlist'}
                    >
                      <Star className={`w-4 h-4 ${watchlist.includes(symbolResult.symbol) ? 'fill-current' : ''}`} />
                    </button>
                    <button
                      onClick={() => openChart(symbolResult.symbol, symbolResult.timeframes[0]?.interval || '1h')}
                      className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold flex items-center gap-2 transition-colors"
                    >
                      <BarChart3 className="w-4 h-4" />
                      Chart
                    </button>
                    <button
                      onClick={() => handleBuy(symbolResult.symbol)}
                      disabled={buyingSymbol === symbolResult.symbol}
                      className="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded font-semibold flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      {buyingSymbol === symbolResult.symbol ? (
                        <>
                          <Loader className="w-4 h-4 animate-spin" />
                          Buying...
                        </>
                      ) : (
                        <>
                          <ShoppingCart className="w-4 h-4" />
                          Buy
                        </>
                      )}
                    </button>
                    <button
                      onClick={() => handleSell(symbolResult.symbol)}
                      disabled={sellingSymbol === symbolResult.symbol}
                      className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white rounded font-semibold flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                      {sellingSymbol === symbolResult.symbol ? (
                        <>
                          <Loader className="w-4 h-4 animate-spin" />
                          Selling...
                        </>
                      ) : (
                        <>
                          <DollarSign className="w-4 h-4" />
                          Sell
                        </>
                      )}
                    </button>
                  </div>
                </div>
              </div>

              <div className="p-6 space-y-6">
                {symbolResult.timeframes.map((timeframe) => (
                  <div key={timeframe.interval}>
                    <div className="flex items-center gap-2 mb-3">
                      <div className="px-3 py-1 bg-purple-600 text-white rounded font-semibold text-sm">
                        {timeframe.interval}
                      </div>
                      <div className="text-sm text-gray-400">
                        {timeframe.patterns.length} pattern{timeframe.patterns.length !== 1 ? 's' : ''}
                      </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                      {timeframe.patterns.map((pattern, idx) => (
                        <div
                          key={idx}
                          className={`border rounded-lg p-4 ${getPatternColor(pattern.Type)}`}
                        >
                          <div className="flex items-start justify-between mb-2">
                            <div className="flex items-center gap-2">
                              {getPatternIcon(pattern.Type)}
                              <h5 className="font-semibold">{pattern.Name}</h5>
                            </div>
                            <span className={`px-2 py-0.5 rounded text-xs font-semibold ${getReliabilityBadge(pattern.Reliability)}`}>
                              {pattern.Reliability}
                            </span>
                          </div>
                          <p className="text-xs opacity-90">{pattern.Description}</p>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* No Results */}
      {!scanning && results.length === 0 && selectedSymbols.length > 0 && error === null && (
        <div className="text-center py-12 text-gray-500">
          <Sparkles className="w-16 h-16 mx-auto mb-4 opacity-50" />
          <p>Click "Scan Patterns" to start scanning for candlestick patterns</p>
        </div>
      )}

      {/* Chart Modal */}
      <ChartModal
        isOpen={chartModal.isOpen}
        onClose={closeChart}
        symbol={chartModal.symbol}
        interval={chartModal.interval}
      />
    </div>
  );
};
