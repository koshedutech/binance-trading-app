import { useState } from 'react';
import { TrendingUp, TrendingDown, ChevronDown, ChevronUp, Play, Copy, Archive, Trash2 } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';
import PatternDataDisplay from './PatternDataDisplay';
import { parsePatternData } from '../utils/patternParser';
import type { EnhancedPendingSignal } from '../types';

interface Props {
  signal: EnhancedPendingSignal;
  onExecute: (signal: EnhancedPendingSignal) => void;
  onDuplicate: (signal: EnhancedPendingSignal) => void;
  onArchive: (signal: EnhancedPendingSignal) => void;
  onDelete: (signal: EnhancedPendingSignal) => void;
}

export default function SignalCard({ signal, onExecute, onDuplicate, onArchive, onDelete }: Props) {
  const [isExpanded, setIsExpanded] = useState(false);
  const patternData = signal.patternData || parsePatternData(signal.reason || '');
  const isConfirmed = signal.status === 'CONFIRMED';

  const priceChange = ((signal.current_price - signal.entry_price) / signal.entry_price) * 100;
  const priceChangeColor = priceChange >= 0 ? 'text-green-500' : 'text-red-500';

  return (
    <div className={`border rounded-lg p-4 hover:bg-dark-750 transition-colors ${
      isConfirmed ? 'border-green-800' : 'border-red-800'
    }`}>
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <div className="flex items-center gap-2">
            <span className="font-bold text-lg">{signal.symbol}</span>
            <span className={`badge ${signal.signal_type === 'BUY' ? 'badge-success' : 'badge-danger'}`}>
              {signal.signal_type === 'BUY' ? (
                <><TrendingUp className="w-3 h-3 inline mr-1" />{signal.signal_type}</>
              ) : (
                <><TrendingDown className="w-3 h-3 inline mr-1" />{signal.signal_type}</>
              )}
            </span>
          </div>

          {/* Price Info */}
          <div className="text-sm text-gray-400 mt-1">
            Entry: ${signal.entry_price.toFixed(2)} → Current: ${signal.current_price.toFixed(2)}
            <span className={`ml-2 ${priceChangeColor}`}>
              ({priceChange >= 0 ? '+' : ''}{priceChange.toFixed(2)}%)
            </span>
          </div>

          {/* Pattern Data */}
          {patternData && (
            <div className="mt-2">
              <PatternDataDisplay patternData={patternData} />
            </div>
          )}

          {/* Strategy & Time */}
          <div className="text-xs text-gray-500 mt-2">
            {signal.strategy_name} • {formatDistanceToNow(new Date(signal.timestamp), { addSuffix: true })}
          </div>
        </div>
      </div>

      {/* Expanded Details */}
      {isExpanded && (
        <div className="mt-4 pt-4 border-t border-dark-700">
          <div className="space-y-3">
            {/* Trade Parameters */}
            <div>
              <h4 className="text-xs font-semibold text-gray-400 mb-1">TRADE PARAMETERS</h4>
              <div className="grid grid-cols-2 gap-2 text-sm">
                {signal.stop_loss && (
                  <div>
                    <span className="text-gray-500">Stop Loss:</span>
                    <span className="ml-2 text-red-400">${signal.stop_loss.toFixed(2)}</span>
                  </div>
                )}
                {signal.take_profit && (
                  <div>
                    <span className="text-gray-500">Take Profit:</span>
                    <span className="ml-2 text-green-400">${signal.take_profit.toFixed(2)}</span>
                  </div>
                )}
              </div>
            </div>

            {/* Reason */}
            {signal.reason && (
              <div>
                <h4 className="text-xs font-semibold text-gray-400 mb-1">ANALYSIS</h4>
                <p className="text-sm text-gray-300">{signal.reason}</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Action Buttons */}
      <div className="flex gap-2 mt-4 flex-wrap">
        {isConfirmed && !signal.archived && (
          <button
            onClick={() => onExecute(signal)}
            className="btn btn-sm btn-primary flex items-center gap-1"
          >
            <Play className="w-3 h-3" />
            Execute
          </button>
        )}

        <button
          onClick={() => setIsExpanded(!isExpanded)}
          className="btn btn-sm flex items-center gap-1"
        >
          {isExpanded ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
          {isExpanded ? 'Collapse' : 'Details'}
        </button>

        <button
          onClick={() => onDuplicate(signal)}
          className="btn btn-sm flex items-center gap-1"
        >
          <Copy className="w-3 h-3" />
          Copy
        </button>

        <button
          onClick={() => onArchive(signal)}
          className="btn btn-sm flex items-center gap-1 text-gray-400 hover:text-orange-400"
        >
          <Archive className="w-3 h-3" />
          Archive
        </button>

        <button
          onClick={() => onDelete(signal)}
          className="btn btn-sm flex items-center gap-1 text-gray-400 hover:text-red-400"
        >
          <Trash2 className="w-3 h-3" />
          Delete
        </button>
      </div>
    </div>
  );
}
