// Story 14.15: Trading Status Display Component
// Shows current trading state with visual indicators

import { Clock, Activity, LogIn, LogOut } from 'lucide-react';
import type { TradingState } from '../../hooks/useTradingState';

interface TradingStatusProps {
  state: TradingState;
  compact?: boolean;
}

/**
 * Formats ISO timestamp to relative time or readable format
 */
function formatTimestamp(isoString: string): string {
  try {
    const date = new Date(isoString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString();
  } catch {
    return 'Unknown';
  }
}

export default function TradingStatus({ state, compact = false }: TradingStatusProps) {
  const isEnabled = state.enabled;

  if (compact) {
    // Compact mode - just shows status text and icons
    return (
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-1.5">
          {state.entry_active && (
            <span className="flex items-center gap-0.5 text-xs text-green-400">
              <LogIn className="w-3 h-3" />
              Entry
            </span>
          )}
          {state.exit_active && (
            <span className="flex items-center gap-0.5 text-xs text-blue-400">
              <LogOut className="w-3 h-3" />
              Exit
            </span>
          )}
          {!state.entry_active && !state.exit_active && (
            <span className="text-xs text-gray-500">Inactive</span>
          )}
        </div>
      </div>
    );
  }

  // Full status display
  return (
    <div className="flex flex-col gap-1">
      {/* Main status */}
      <div className="flex items-center gap-2">
        <span
          className={`font-medium ${isEnabled ? 'text-green-400' : 'text-gray-400'}`}
        >
          {isEnabled ? 'Trading Active' : 'Trading Paused'}
        </span>
      </div>

      {/* Sub-status: Entry/Exit state */}
      <div className="flex items-center gap-3 text-xs">
        <div className="flex items-center gap-1.5">
          <Activity className="w-3 h-3 text-gray-500" />
          {isEnabled ? (
            <span className="text-gray-400">
              Entry + Exit monitoring
            </span>
          ) : (
            <span className="text-gray-500">
              Exit monitoring only - No new entries
            </span>
          )}
        </div>
      </div>

      {/* Last changed timestamp */}
      {state.last_changed && (
        <div className="flex items-center gap-1.5 text-xs text-gray-500">
          <Clock className="w-3 h-3" />
          <span>Changed {formatTimestamp(state.last_changed)}</span>
          {state.changed_by && (
            <span className="text-gray-600">by {state.changed_by}</span>
          )}
        </div>
      )}

      {/* Reason if provided */}
      {state.reason && (
        <div className="text-xs text-gray-500 italic">
          "{state.reason}"
        </div>
      )}
    </div>
  );
}
