import React from 'react';
import { Filter, X } from 'lucide-react';
import { ChainFilters as FilterType, TradingModeCode, MODE_DISPLAY_NAMES } from './types';

interface ChainFiltersProps {
  filters: FilterType;
  onFilterChange: (filters: FilterType) => void;
  symbols: string[];
  onReset: () => void;
}

export default function ChainFilters({ filters, onFilterChange, symbols, onReset }: ChainFiltersProps) {
  const hasActiveFilters =
    filters.mode !== 'all' ||
    filters.status !== 'all' ||
    filters.symbol !== 'all' ||
    filters.side !== 'all';

  return (
    <div className="flex items-center gap-3 flex-wrap">
      <div className="flex items-center gap-1.5 text-gray-400">
        <Filter className="w-4 h-4" />
        <span className="text-sm">Filters:</span>
      </div>

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

      {/* Reset button */}
      {hasActiveFilters && (
        <button
          onClick={onReset}
          className="flex items-center gap-1.5 px-2 py-1 text-sm text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors"
        >
          <X className="w-3.5 h-3.5" />
          Reset
        </button>
      )}
    </div>
  );
}
