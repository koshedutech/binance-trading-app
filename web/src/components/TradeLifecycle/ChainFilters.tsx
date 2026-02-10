import React from 'react';
import { Filter, X, Calendar } from 'lucide-react';
import { ChainFilters as FilterType, TradingModeCode, MODE_DISPLAY_NAMES } from './types';

interface ChainFiltersProps {
  filters: FilterType;
  onFilterChange: (filters: FilterType) => void;
  symbols: string[];
  onReset: () => void;
  activeOnly?: boolean;
  onActiveOnlyChange?: (value: boolean) => void;
}

// Get today's date in YYYY-MM-DD format
function getTodayDate(): string {
  const today = new Date();
  return today.toISOString().split('T')[0];
}

// Get yesterday's date in YYYY-MM-DD format
function getYesterdayDate(): string {
  const yesterday = new Date();
  yesterday.setDate(yesterday.getDate() - 1);
  return yesterday.toISOString().split('T')[0];
}

// Quick date presets
type DatePreset = 'today' | 'yesterday' | 'last7days' | 'last30days' | 'custom';

export default function ChainFilters({ filters, onFilterChange, symbols, onReset, activeOnly, onActiveOnlyChange }: ChainFiltersProps) {
  const hasActiveFilters =
    filters.mode !== 'all' ||
    filters.status !== 'all' ||
    filters.symbol !== 'all' ||
    filters.side !== 'all' ||
    filters.dateFrom ||
    filters.dateTo ||
    activeOnly;

  // Determine current date preset based on filter values
  const getActivePreset = (): DatePreset | null => {
    const today = getTodayDate();
    const yesterday = getYesterdayDate();

    if (filters.dateFrom === today && filters.dateTo === today) return 'today';
    if (filters.dateFrom === yesterday && filters.dateTo === yesterday) return 'yesterday';

    const last7Days = new Date();
    last7Days.setDate(last7Days.getDate() - 7);
    if (filters.dateFrom === last7Days.toISOString().split('T')[0] && filters.dateTo === today) return 'last7days';

    const last30Days = new Date();
    last30Days.setDate(last30Days.getDate() - 30);
    if (filters.dateFrom === last30Days.toISOString().split('T')[0] && filters.dateTo === today) return 'last30days';

    if (filters.dateFrom || filters.dateTo) return 'custom';
    return null;
  };

  // Handle date preset change
  const handlePresetChange = (preset: DatePreset) => {
    const today = getTodayDate();

    switch (preset) {
      case 'today':
        onFilterChange({ ...filters, dateFrom: today, dateTo: today });
        break;
      case 'yesterday':
        onFilterChange({ ...filters, dateFrom: getYesterdayDate(), dateTo: getYesterdayDate() });
        break;
      case 'last7days': {
        const last7Days = new Date();
        last7Days.setDate(last7Days.getDate() - 7);
        onFilterChange({ ...filters, dateFrom: last7Days.toISOString().split('T')[0], dateTo: today });
        break;
      }
      case 'last30days': {
        const last30Days = new Date();
        last30Days.setDate(last30Days.getDate() - 30);
        onFilterChange({ ...filters, dateFrom: last30Days.toISOString().split('T')[0], dateTo: today });
        break;
      }
      case 'custom':
        // Keep current values for custom
        break;
    }
  };

  const activePreset = getActivePreset();

  return (
    <div className="flex items-center gap-3 flex-wrap">
      <div className="flex items-center gap-1.5 text-gray-400">
        <Filter className="w-4 h-4" />
        <span className="text-sm">Filters:</span>
      </div>

      {/* Active Only toggle */}
      {onActiveOnlyChange && (
        <button
          onClick={() => onActiveOnlyChange(!activeOnly)}
          className={`flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-full transition-colors ${
            activeOnly
              ? 'bg-green-500/20 text-green-400 border border-green-500/50'
              : 'bg-gray-700 text-gray-400 hover:bg-gray-600 border border-transparent'
          }`}
          title="Show only active and partial orders"
        >
          <div className={`w-7 h-4 rounded-full relative transition-colors ${
            activeOnly ? 'bg-green-500/40' : 'bg-gray-600'
          }`}>
            <div className={`absolute top-0.5 w-3 h-3 rounded-full transition-all ${
              activeOnly ? 'right-0.5 bg-green-400' : 'left-0.5 bg-gray-400'
            }`} />
          </div>
          Active Only
        </button>
      )}

      {/* Separator before dropdowns */}
      <div className="w-px h-6 bg-gray-600" />

      {/* Mode filter */}
      <select
        value={filters.mode}
        onChange={(e) => onFilterChange({ ...filters, mode: e.target.value as TradingModeCode | 'all' })}
        className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm text-gray-300 min-w-[100px]"
      >
        <option value="all">All Modes</option>
        {(Object.keys(MODE_DISPLAY_NAMES) as TradingModeCode[]).map((mode) => (
          <option key={mode} value={mode}>
            {MODE_DISPLAY_NAMES[mode]}
          </option>
        ))}
      </select>

      {/* Status filter */}
      <select
        value={filters.status}
        onChange={(e) => onFilterChange({ ...filters, status: e.target.value as FilterType['status'] })}
        className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm text-gray-300 min-w-[100px]"
      >
        <option value="all">All Status</option>
        <option value="active">Active</option>
        <option value="partial">Partial</option>
        <option value="completed">Completed</option>
        <option value="cancelled">Cancelled</option>
      </select>

      {/* Symbol filter */}
      <select
        value={filters.symbol}
        onChange={(e) => onFilterChange({ ...filters, symbol: e.target.value })}
        className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm text-gray-300 min-w-[120px]"
      >
        <option value="all">All Symbols</option>
        {symbols.map((symbol) => (
          <option key={symbol} value={symbol}>
            {symbol}
          </option>
        ))}
      </select>

      {/* Side filter */}
      <select
        value={filters.side}
        onChange={(e) => onFilterChange({ ...filters, side: e.target.value as FilterType['side'] })}
        className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-sm text-gray-300 min-w-[90px]"
      >
        <option value="all">All Sides</option>
        <option value="LONG">Long</option>
        <option value="SHORT">Short</option>
      </select>

      {/* Separator */}
      <div className="w-px h-6 bg-gray-600" />

      {/* Date filter section */}
      <div className="flex items-center gap-1.5 text-gray-400">
        <Calendar className="w-4 h-4" />
        <span className="text-sm">Date:</span>
      </div>

      {/* Date preset buttons */}
      <div className="flex gap-1">
        <button
          onClick={() => handlePresetChange('today')}
          className={`px-2 py-1 text-xs rounded transition-colors ${
            activePreset === 'today'
              ? 'bg-purple-500/30 text-purple-400 border border-purple-500/50'
              : 'bg-gray-700 text-gray-400 hover:bg-gray-600 border border-transparent'
          }`}
        >
          Today
        </button>
        <button
          onClick={() => handlePresetChange('yesterday')}
          className={`px-2 py-1 text-xs rounded transition-colors ${
            activePreset === 'yesterday'
              ? 'bg-purple-500/30 text-purple-400 border border-purple-500/50'
              : 'bg-gray-700 text-gray-400 hover:bg-gray-600 border border-transparent'
          }`}
        >
          Yesterday
        </button>
        <button
          onClick={() => handlePresetChange('last7days')}
          className={`px-2 py-1 text-xs rounded transition-colors ${
            activePreset === 'last7days'
              ? 'bg-purple-500/30 text-purple-400 border border-purple-500/50'
              : 'bg-gray-700 text-gray-400 hover:bg-gray-600 border border-transparent'
          }`}
        >
          7 Days
        </button>
        <button
          onClick={() => handlePresetChange('last30days')}
          className={`px-2 py-1 text-xs rounded transition-colors ${
            activePreset === 'last30days'
              ? 'bg-purple-500/30 text-purple-400 border border-purple-500/50'
              : 'bg-gray-700 text-gray-400 hover:bg-gray-600 border border-transparent'
          }`}
        >
          30 Days
        </button>
      </div>

      {/* Custom date range inputs */}
      <div className="flex items-center gap-2">
        <input
          type="date"
          value={filters.dateFrom || ''}
          onChange={(e) => onFilterChange({ ...filters, dateFrom: e.target.value || undefined })}
          className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-xs text-gray-300"
          placeholder="From"
        />
        <span className="text-gray-500 text-xs">to</span>
        <input
          type="date"
          value={filters.dateTo || ''}
          onChange={(e) => onFilterChange({ ...filters, dateTo: e.target.value || undefined })}
          className="bg-gray-700 border border-gray-600 rounded px-2 py-1 text-xs text-gray-300"
          placeholder="To"
        />
      </div>

      {/* Clear date filter */}
      {(filters.dateFrom || filters.dateTo) && (
        <button
          onClick={() => onFilterChange({ ...filters, dateFrom: undefined, dateTo: undefined })}
          className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors"
          title="Clear date filter"
        >
          <X className="w-3 h-3" />
        </button>
      )}

      {/* Reset all filters button */}
      {hasActiveFilters && (
        <button
          onClick={onReset}
          className="flex items-center gap-1.5 px-2 py-1 text-sm text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors"
        >
          <X className="w-3.5 h-3.5" />
          Reset All
        </button>
      )}
    </div>
  );
}
