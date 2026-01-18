import { useState } from 'react';
import { Calendar, ChevronDown } from 'lucide-react';

export type DateRangeType = 'day' | 'week' | 'month' | 'year' | 'custom';

interface DateRangeFilterProps {
  selectedRange: DateRangeType;
  onRangeChange: (range: DateRangeType, startDate?: string, endDate?: string) => void;
  customStartDate?: string;
  customEndDate?: string;
}

export default function DateRangeFilter({
  selectedRange,
  onRangeChange,
  customStartDate,
  customEndDate,
}: DateRangeFilterProps) {
  const [showCustomPicker, setShowCustomPicker] = useState(false);
  const [tempStartDate, setTempStartDate] = useState(customStartDate || '');
  const [tempEndDate, setTempEndDate] = useState(customEndDate || '');

  const rangeOptions: { value: DateRangeType; label: string }[] = [
    { value: 'day', label: '1 Day' },
    { value: 'week', label: '7 Days' },
    { value: 'month', label: '30 Days' },
    { value: 'year', label: '1 Year' },
    { value: 'custom', label: 'Custom' },
  ];

  const handleRangeSelect = (range: DateRangeType) => {
    if (range === 'custom') {
      setShowCustomPicker(true);
    } else {
      setShowCustomPicker(false);
      onRangeChange(range);
    }
  };

  const handleCustomApply = () => {
    if (tempStartDate && tempEndDate) {
      // Validate that start date is not after end date
      if (new Date(tempStartDate) > new Date(tempEndDate)) {
        alert('Start date cannot be after end date');
        return;
      }
      onRangeChange('custom', tempStartDate, tempEndDate);
      setShowCustomPicker(false);
    }
  };

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {/* Range selector buttons */}
      <div className="flex items-center gap-1 bg-gray-800 rounded-lg p-1">
        {rangeOptions.map((option) => (
          <button
            key={option.value}
            onClick={() => handleRangeSelect(option.value)}
            className={`
              px-3 py-1.5 text-xs font-medium rounded transition-all
              ${selectedRange === option.value
                ? 'bg-blue-600 text-white'
                : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }
            `}
          >
            {option.label}
          </button>
        ))}
      </div>

      {/* Custom date picker popup */}
      {showCustomPicker && (
        <div className="flex items-center gap-2 bg-gray-800 rounded-lg p-2 border border-gray-700">
          <div className="flex items-center gap-1">
            <Calendar className="w-4 h-4 text-gray-400" />
            <input
              type="date"
              value={tempStartDate}
              onChange={(e) => setTempStartDate(e.target.value)}
              className="bg-gray-700 text-white text-xs px-2 py-1 rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
            />
          </div>
          <span className="text-gray-500">to</span>
          <input
            type="date"
            value={tempEndDate}
            onChange={(e) => setTempEndDate(e.target.value)}
            className="bg-gray-700 text-white text-xs px-2 py-1 rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
          />
          <button
            onClick={handleCustomApply}
            disabled={!tempStartDate || !tempEndDate}
            className="px-3 py-1 text-xs font-medium bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Apply
          </button>
          <button
            onClick={() => setShowCustomPicker(false)}
            className="px-2 py-1 text-xs text-gray-400 hover:text-white"
          >
            Cancel
          </button>
        </div>
      )}

      {/* Show current custom range if selected */}
      {selectedRange === 'custom' && customStartDate && customEndDate && !showCustomPicker && (
        <div className="flex items-center gap-1 text-xs text-gray-400">
          <Calendar className="w-3 h-3" />
          <span>{customStartDate} to {customEndDate}</span>
          <button
            onClick={() => setShowCustomPicker(true)}
            className="ml-1 text-blue-400 hover:text-blue-300"
          >
            <ChevronDown className="w-3 h-3" />
          </button>
        </div>
      )}
    </div>
  );
}
