import { useState, useEffect, useRef, useCallback } from 'react';
import { X, Download, Calendar, Clock, AlertTriangle, Check, Search, Loader2, ChevronDown } from 'lucide-react';
import { apiService } from '../../services/api';

interface DownloadDataModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (request: DownloadRequest) => Promise<void>;
  existingSymbols: string[];
}

export interface DownloadRequest {
  symbol: string;
  timeframes: string[];
  from: string;
  to: string;
}

// API response wrapper
interface APIWrapper<T> {
  success: boolean;
  data: T;
}

interface SymbolsResponse {
  symbols: string[];
  count: number;
}

const AVAILABLE_TIMEFRAMES = [
  { value: '1m', label: '1 minute' },
  { value: '3m', label: '3 minutes' },
  { value: '5m', label: '5 minutes' },
  { value: '15m', label: '15 minutes' },
  { value: '1h', label: '1 hour' },
  { value: '4h', label: '4 hours' },
  { value: '1d', label: '1 day' },
];

// Popular trading pairs (shown as quick select)
const POPULAR_SYMBOLS = [
  'BTCUSDT',
  'ETHUSDT',
  'BNBUSDT',
  'SOLUSDT',
  'XRPUSDT',
  'DOGEUSDT',
  'ADAUSDT',
  'MATICUSDT',
  'LINKUSDT',
  'AVAXUSDT',
];

// Get default dates (from 1 year ago to today)
function getDefaultDates() {
  const to = new Date();
  const from = new Date();
  from.setFullYear(from.getFullYear() - 1);

  return {
    from: from.toISOString().split('T')[0],
    to: to.toISOString().split('T')[0],
  };
}

// Estimate number of candles
function estimateCandles(from: string, to: string, timeframes: string[]): number {
  const fromDate = new Date(from);
  const toDate = new Date(to);
  const diffMs = toDate.getTime() - fromDate.getTime();

  let total = 0;
  for (const tf of timeframes) {
    let candlesPerDay = 0;
    switch (tf) {
      case '1m': candlesPerDay = 1440; break;
      case '3m': candlesPerDay = 480; break;
      case '5m': candlesPerDay = 288; break;
      case '15m': candlesPerDay = 96; break;
      case '30m': candlesPerDay = 48; break;
      case '1h': candlesPerDay = 24; break;
      case '4h': candlesPerDay = 6; break;
      case '1d': candlesPerDay = 1; break;
      default: candlesPerDay = 96; // Default to 15m
    }
    const days = diffMs / (1000 * 60 * 60 * 24);
    total += Math.ceil(days * candlesPerDay);
  }

  return total;
}

export default function DownloadDataModal({
  isOpen,
  onClose,
  onSubmit,
  existingSymbols,
}: DownloadDataModalProps) {
  const [symbol, setSymbol] = useState('');
  const [timeframes, setTimeframes] = useState<string[]>(['15m', '1h']);
  const [from, setFrom] = useState(() => getDefaultDates().from);
  const [to, setTo] = useState(() => getDefaultDates().to);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Symbol search state
  const [availableSymbols, setAvailableSymbols] = useState<string[]>([]);
  const [isLoadingSymbols, setIsLoadingSymbols] = useState(false);
  const [showDropdown, setShowDropdown] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [highlightedIndex, setHighlightedIndex] = useState(0);

  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Fetch available symbols from Binance Futures
  const fetchSymbols = useCallback(async () => {
    setIsLoadingSymbols(true);
    try {
      // Use futures endpoint - returns array of symbols directly
      const response = await apiService.get<string[]>('/futures/symbols');
      if (Array.isArray(response.data)) {
        setAvailableSymbols(response.data);
      } else {
        // Fall back to wrapped response format
        const wrapped = response.data as unknown as APIWrapper<SymbolsResponse>;
        if (wrapped?.data?.symbols) {
          setAvailableSymbols(wrapped.data.symbols);
        }
      }
    } catch (err) {
      console.error('Failed to fetch symbols:', err);
      // Fall back to popular symbols if API fails
      setAvailableSymbols(POPULAR_SYMBOLS);
    } finally {
      setIsLoadingSymbols(false);
    }
  }, []);

  // Fetch symbols when modal opens
  useEffect(() => {
    if (isOpen && availableSymbols.length === 0) {
      fetchSymbols();
    }
  }, [isOpen, availableSymbols.length, fetchSymbols]);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      const defaultDates = getDefaultDates();
      setSymbol('');
      setSearchQuery('');
      setTimeframes(['15m', '1h']);
      setFrom(defaultDates.from);
      setTo(defaultDates.to);
      setError(null);
      setShowDropdown(false);
      setHighlightedIndex(0);
    }
  }, [isOpen]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(event.target as Node)
      ) {
        setShowDropdown(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Filter symbols based on search query
  const filteredSymbols = searchQuery
    ? availableSymbols.filter((s) =>
        s.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : availableSymbols;

  // Limit displayed results for performance
  const displayedSymbols = filteredSymbols.slice(0, 50);

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showDropdown) {
      if (e.key === 'ArrowDown' || e.key === 'Enter') {
        setShowDropdown(true);
        e.preventDefault();
      }
      return;
    }

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setHighlightedIndex((prev) =>
          prev < displayedSymbols.length - 1 ? prev + 1 : prev
        );
        break;
      case 'ArrowUp':
        e.preventDefault();
        setHighlightedIndex((prev) => (prev > 0 ? prev - 1 : 0));
        break;
      case 'Enter':
        e.preventDefault();
        if (displayedSymbols[highlightedIndex]) {
          selectSymbol(displayedSymbols[highlightedIndex]);
        }
        break;
      case 'Escape':
        setShowDropdown(false);
        break;
    }
  };

  // Select a symbol
  const selectSymbol = (s: string) => {
    setSymbol(s);
    setSearchQuery(s);
    setShowDropdown(false);
    setHighlightedIndex(0);
  };

  // Handle input change
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value.toUpperCase();
    setSearchQuery(value);
    setSymbol(value);
    setShowDropdown(true);
    setHighlightedIndex(0);
  };

  const toggleTimeframe = (tf: string) => {
    setTimeframes((prev) =>
      prev.includes(tf) ? prev.filter((t) => t !== tf) : [...prev, tf]
    );
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    // Validation
    if (!symbol.trim()) {
      setError('Please enter a symbol');
      return;
    }

    // Check if symbol is valid (exists in available symbols or is manually entered)
    const isValidSymbol = availableSymbols.length === 0 || availableSymbols.includes(symbol.toUpperCase());
    if (!isValidSymbol) {
      setError(`Symbol "${symbol}" not found on Binance. Please select a valid symbol.`);
      return;
    }

    if (timeframes.length === 0) {
      setError('Please select at least one timeframe');
      return;
    }
    if (new Date(to) < new Date(from)) {
      setError('End date must be after start date');
      return;
    }

    // Check date range (max 3 years)
    const fromDate = new Date(from);
    const toDate = new Date(to);
    const diffYears = (toDate.getTime() - fromDate.getTime()) / (1000 * 60 * 60 * 24 * 365);
    if (diffYears > 3) {
      setError('Date range cannot exceed 3 years');
      return;
    }

    setIsSubmitting(true);
    try {
      await onSubmit({
        symbol: symbol.toUpperCase(),
        timeframes,
        from,
        to,
      });
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start download');
    } finally {
      setIsSubmitting(false);
    }
  };

  const estimatedCandles = estimateCandles(from, to, timeframes);
  const symbolExists = existingSymbols.includes(symbol.toUpperCase());

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      role="dialog"
      aria-modal="true"
      aria-labelledby="download-modal-title"
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
        aria-label="Close modal"
      />

      {/* Modal */}
      <div className="relative bg-dark-800 rounded-lg border border-dark-700 w-full max-w-lg mx-4 shadow-2xl">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-dark-700">
          <div className="flex items-center gap-2">
            <Download className="w-5 h-5 text-primary-400" />
            <h2 id="download-modal-title" className="text-lg font-semibold text-white">Download Historical Data</h2>
          </div>
          <button
            onClick={onClose}
            className="p-1 hover:bg-dark-700 rounded transition-colors"
            aria-label="Close modal"
          >
            <X className="w-5 h-5 text-gray-400" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5">
          {/* Symbol input with autocomplete */}
          <div className="relative">
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Symbol
              {isLoadingSymbols && (
                <Loader2 className="w-3 h-3 inline ml-2 animate-spin text-gray-500" />
              )}
              {!isLoadingSymbols && availableSymbols.length > 0 && (
                <span className="text-xs text-gray-500 ml-2">
                  ({availableSymbols.length} available)
                </span>
              )}
            </label>

            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
              <input
                ref={inputRef}
                type="text"
                value={searchQuery}
                onChange={handleInputChange}
                onFocus={() => setShowDropdown(true)}
                onKeyDown={handleKeyDown}
                placeholder="Search symbols (e.g., BTC, ETH)"
                className="w-full pl-10 pr-10 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                autoComplete="off"
              />
              <button
                type="button"
                onClick={() => setShowDropdown(!showDropdown)}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1 hover:bg-dark-600 rounded"
              >
                <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${showDropdown ? 'rotate-180' : ''}`} />
              </button>
            </div>

            {/* Dropdown */}
            {showDropdown && (
              <div
                ref={dropdownRef}
                className="absolute z-10 w-full mt-1 bg-dark-700 border border-dark-600 rounded-lg shadow-xl max-h-60 overflow-y-auto"
              >
                {isLoadingSymbols ? (
                  <div className="flex items-center justify-center py-4 text-gray-400">
                    <Loader2 className="w-4 h-4 animate-spin mr-2" />
                    Loading symbols...
                  </div>
                ) : displayedSymbols.length > 0 ? (
                  <>
                    {displayedSymbols.map((s, index) => (
                      <button
                        key={s}
                        type="button"
                        onClick={() => selectSymbol(s)}
                        className={`w-full px-4 py-2 text-left text-sm flex items-center justify-between transition-colors ${
                          index === highlightedIndex
                            ? 'bg-primary-500/20 text-primary-400'
                            : 'text-gray-300 hover:bg-dark-600'
                        } ${existingSymbols.includes(s) ? 'border-l-2 border-yellow-500' : ''}`}
                      >
                        <span className="font-mono">{s}</span>
                        {existingSymbols.includes(s) && (
                          <span className="text-xs text-yellow-400">Has data</span>
                        )}
                      </button>
                    ))}
                    {filteredSymbols.length > 50 && (
                      <div className="px-4 py-2 text-xs text-gray-500 border-t border-dark-600">
                        Showing 50 of {filteredSymbols.length} results. Type more to narrow down.
                      </div>
                    )}
                  </>
                ) : searchQuery ? (
                  <div className="px-4 py-3 text-sm text-gray-400">
                    No symbols found matching "{searchQuery}"
                  </div>
                ) : (
                  <div className="px-4 py-3 text-sm text-gray-400">
                    Start typing to search symbols
                  </div>
                )}
              </div>
            )}

            {symbolExists && symbol && (
              <div className="flex items-center gap-1 mt-1 text-xs text-yellow-400">
                <AlertTriangle className="w-3 h-3" />
                Data exists for this symbol. New data will be merged.
              </div>
            )}

            {/* Quick select buttons */}
            <div className="flex flex-wrap gap-1 mt-2">
              {POPULAR_SYMBOLS.slice(0, 5).map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => selectSymbol(s)}
                  className={`px-2 py-1 text-xs rounded transition-colors ${
                    symbol === s
                      ? 'bg-primary-500/20 text-primary-400 border border-primary-500/30'
                      : 'bg-dark-700 text-gray-400 hover:text-white'
                  }`}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>

          {/* Timeframes */}
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">
              Timeframes
            </label>
            <div className="flex flex-wrap gap-2">
              {AVAILABLE_TIMEFRAMES.map((tf) => (
                <button
                  key={tf.value}
                  type="button"
                  onClick={() => toggleTimeframe(tf.value)}
                  className={`flex items-center gap-1.5 px-3 py-2 rounded-lg transition-colors ${
                    timeframes.includes(tf.value)
                      ? 'bg-primary-500/20 text-primary-400 border border-primary-500/30'
                      : 'bg-dark-700 text-gray-400 hover:text-white border border-dark-600'
                  }`}
                >
                  {timeframes.includes(tf.value) && <Check className="w-3 h-3" />}
                  <Clock className="w-3 h-3" />
                  {tf.label}
                </button>
              ))}
            </div>
          </div>

          {/* Date range */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                <Calendar className="w-3 h-3 inline mr-1" />
                From Date
              </label>
              <input
                type="date"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
                min="2019-09-01"
                max={to}
                className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-300 mb-2">
                <Calendar className="w-3 h-3 inline mr-1" />
                To Date
              </label>
              <input
                type="date"
                value={to}
                onChange={(e) => setTo(e.target.value)}
                min={from}
                max={new Date().toISOString().split('T')[0]}
                className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
              />
            </div>
          </div>

          {/* Estimate */}
          <div className="p-3 bg-dark-700/50 rounded-lg">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-400">Estimated candles:</span>
              <span className="text-white font-medium">
                {estimatedCandles.toLocaleString()}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm mt-1">
              <span className="text-gray-400">Estimated storage:</span>
              <span className="text-white font-medium">
                ~{((estimatedCandles * 100) / (1024 * 1024)).toFixed(2)} MB
              </span>
            </div>
          </div>

          {/* Error */}
          {error && (
            <div className="flex items-center gap-2 p-3 bg-red-500/10 border border-red-500/30 rounded-lg">
              <AlertTriangle className="w-4 h-4 text-red-400" />
              <span className="text-sm text-red-400">{error}</span>
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 bg-dark-700 text-gray-300 rounded-lg hover:bg-dark-600 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting || !symbol || timeframes.length === 0}
              className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isSubmitting ? (
                <>
                  <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  Starting...
                </>
              ) : (
                <>
                  <Download className="w-4 h-4" />
                  Download
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
