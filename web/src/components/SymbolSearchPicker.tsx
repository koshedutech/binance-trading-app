import React, { useState, useEffect, useRef } from 'react';
import { Search, X } from 'lucide-react';
import { apiService } from '../services/api';

interface SymbolSearchPickerProps {
  value: string;
  onChange: (symbol: string) => void;
  placeholder?: string;
}

export const SymbolSearchPicker: React.FC<SymbolSearchPickerProps> = ({
  value,
  onChange,
  placeholder = 'Search symbols...',
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [symbols, setSymbols] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [filteredSymbols, setFilteredSymbols] = useState<string[]>([]);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const fetchSymbols = async () => {
      setLoading(true);
      try {
        const symbolsList = await apiService.getBinanceSymbols();
        setSymbols(symbolsList);
        setFilteredSymbols(symbolsList.slice(0, 50)); // Show first 50 by default
      } catch (error) {
        console.error('Failed to fetch symbols:', error);
      } finally {
        setLoading(false);
      }
    };

    if (isOpen && symbols.length === 0) {
      fetchSymbols();
    }
  }, [isOpen, symbols.length]);

  useEffect(() => {
    if (search.trim() === '') {
      setFilteredSymbols(symbols.slice(0, 50));
    } else {
      const searchLower = search.toLowerCase();
      const filtered = symbols
        .filter((symbol) => symbol.toLowerCase().includes(searchLower))
        .slice(0, 50);
      setFilteredSymbols(filtered);
    }
  }, [search, symbols]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSelect = (symbol: string) => {
    onChange(symbol);
    setIsOpen(false);
    setSearch('');
  };

  const handleClear = (e: React.MouseEvent) => {
    e.stopPropagation();
    onChange('');
    setSearch('');
  };

  return (
    <div ref={containerRef} className="relative">
      <div
        onClick={() => setIsOpen(!isOpen)}
        className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white cursor-pointer flex items-center justify-between hover:border-gray-500 transition-colors"
      >
        <span className={value ? 'text-white' : 'text-gray-400'}>
          {value || placeholder}
        </span>
        <div className="flex items-center gap-2">
          {value && (
            <button
              onClick={handleClear}
              className="p-0.5 hover:bg-gray-600 rounded transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          )}
          <Search className="w-4 h-4 text-gray-400" />
        </div>
      </div>

      {isOpen && (
        <div className="absolute z-50 w-full mt-1 bg-gray-800 border border-gray-600 rounded-lg shadow-xl max-h-[300px] flex flex-col">
          {/* Search input */}
          <div className="p-2 border-b border-gray-700">
            <div className="relative">
              <Search className="absolute left-2 top-2.5 w-4 h-4 text-gray-400" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Type to search..."
                className="w-full pl-8 pr-3 py-2 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500"
                autoFocus
              />
            </div>
          </div>

          {/* Symbol list */}
          <div className="overflow-y-auto flex-1">
            {loading ? (
              <div className="p-4 text-center text-gray-400 text-sm">Loading symbols...</div>
            ) : filteredSymbols.length === 0 ? (
              <div className="p-4 text-center text-gray-400 text-sm">No symbols found</div>
            ) : (
              <div>
                {filteredSymbols.map((symbol) => (
                  <div
                    key={symbol}
                    onClick={() => handleSelect(symbol)}
                    className={`px-3 py-2 cursor-pointer hover:bg-gray-700 transition-colors ${
                      symbol === value ? 'bg-blue-600 text-white' : 'text-gray-300'
                    }`}
                  >
                    <div className="text-sm font-medium">{symbol}</div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Footer */}
          <div className="p-2 border-t border-gray-700 text-xs text-gray-500 text-center">
            Showing {filteredSymbols.length} of {symbols.length} symbols
          </div>
        </div>
      )}
    </div>
  );
};
