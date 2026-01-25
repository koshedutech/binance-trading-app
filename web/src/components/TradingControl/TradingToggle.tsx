// Story 14.15: Trading Toggle Component
// Main ON/OFF toggle for trading control - Epic 14 Chain Trading System

import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import {
  Power,
  AlertTriangle,
  Check,
  Loader2,
  Activity,
  Pause,
} from 'lucide-react';
import {
  useTradingState,
  useEnableTrading,
  useDisableTrading,
} from '../../hooks/useTradingState';
import TradingStatus from './TradingStatus';

interface TradingToggleProps {
  /** Callback when trading state changes */
  onStateChange?: (enabled: boolean) => void;
  /** Show compact version (just toggle, no status details) */
  compact?: boolean;
  /** Custom class for container */
  className?: string;
}

export default function TradingToggle({
  onStateChange,
  compact = false,
  className = '',
}: TradingToggleProps) {
  const { data: state, isLoading, error, refetch } = useTradingState();
  const enableTrading = useEnableTrading();
  const disableTrading = useDisableTrading();

  const [showConfirm, setShowConfirm] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);

  // Clear local error after 5 seconds
  useEffect(() => {
    if (localError) {
      const timer = setTimeout(() => setLocalError(null), 5000);
      return () => clearTimeout(timer);
    }
  }, [localError]);

  const handleToggle = () => {
    if (!state) return;

    if (state.enabled) {
      // Turning OFF - show confirmation modal
      setShowConfirm(true);
    } else {
      // Turning ON - safe, no confirmation needed
      handleEnable();
    }
  };

  const handleEnable = async () => {
    try {
      setLocalError(null);
      await enableTrading.mutate('User enabled trading');
      refetch();
      onStateChange?.(true);
    } catch (err) {
      setLocalError('Failed to enable trading');
      console.error('Enable trading error:', err);
    }
  };

  const handleDisable = async () => {
    try {
      setLocalError(null);
      setShowConfirm(false);
      await disableTrading.mutate('User disabled trading');
      refetch();
      onStateChange?.(false);
    } catch (err) {
      setLocalError('Failed to disable trading');
      console.error('Disable trading error:', err);
    }
  };

  const isSwitching = enableTrading.isLoading || disableTrading.isLoading;
  const isEnabled = state?.enabled ?? false;

  // Loading state
  if (isLoading && !state) {
    return (
      <div className={`flex items-center gap-2 px-3 py-2 bg-gray-800 rounded-lg ${className}`}>
        <Loader2 className="w-4 h-4 animate-spin text-gray-400" />
        <span className="text-sm text-gray-400">Loading...</span>
      </div>
    );
  }

  // Error state (no data)
  if (error && !state) {
    return (
      <div className={`flex items-center gap-2 px-3 py-2 bg-red-500/10 border border-red-500/30 rounded-lg ${className}`}>
        <AlertTriangle className="w-4 h-4 text-red-500" />
        <span className="text-sm text-red-500">Failed to load</span>
        <button
          onClick={refetch}
          className="text-red-400 hover:text-red-300 text-xs underline"
        >
          Retry
        </button>
      </div>
    );
  }

  // Compact mode - just the toggle button
  if (compact) {
    return (
      <>
        <button
          onClick={handleToggle}
          disabled={isSwitching}
          title={isEnabled ? 'Click to pause trading' : 'Click to enable trading'}
          className={`
            relative flex items-center gap-2 px-3 py-1.5 rounded-lg border transition-all
            ${isEnabled
              ? 'bg-green-500/20 border-green-500/50 hover:bg-green-500/30 text-green-400'
              : 'bg-gray-700/50 border-gray-600 hover:bg-gray-700 text-gray-400'
            }
            ${isSwitching ? 'opacity-60 cursor-wait' : 'cursor-pointer'}
            ${className}
          `}
        >
          {isSwitching ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : isEnabled ? (
            <Activity className="w-4 h-4" />
          ) : (
            <Pause className="w-4 h-4" />
          )}
          <span className="text-sm font-medium">
            {isSwitching ? 'Switching...' : isEnabled ? 'ON' : 'OFF'}
          </span>
        </button>

        {/* Confirmation Modal */}
        {showConfirm && createPortal(
          <ConfirmModal
            onConfirm={handleDisable}
            onCancel={() => setShowConfirm(false)}
            isLoading={disableTrading.isLoading}
          />,
          document.body
        )}
      </>
    );
  }

  // Full mode - toggle with status
  return (
    <>
      {/* Error banner */}
      {(localError || (error && state)) && (
        <div className="flex items-center gap-2 px-3 py-2 mb-2 bg-red-500/10 border border-red-500/30 rounded-lg">
          <AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0" />
          <span className="text-sm text-red-500">{localError || error}</span>
          <button
            onClick={() => setLocalError(null)}
            className="ml-auto text-red-500 hover:text-red-400 font-bold"
          >
            x
          </button>
        </div>
      )}

      {/* Main toggle control */}
      <div
        className={`
          flex items-center gap-4 px-4 py-3 rounded-lg border transition-all
          ${isEnabled
            ? 'bg-green-500/10 border-green-500/30'
            : 'bg-gray-800/50 border-gray-700'
          }
          ${className}
        `}
      >
        {/* Large toggle button */}
        <button
          onClick={handleToggle}
          disabled={isSwitching}
          title={isEnabled ? 'Click to pause trading' : 'Click to enable trading'}
          className={`
            relative w-16 h-16 rounded-full flex items-center justify-center
            transition-all duration-200 transform hover:scale-105
            ${isEnabled
              ? 'bg-green-500 shadow-lg shadow-green-500/30 hover:bg-green-400'
              : 'bg-gray-600 hover:bg-gray-500'
            }
            ${isSwitching ? 'opacity-60 cursor-wait' : 'cursor-pointer'}
          `}
          aria-label={isEnabled ? 'Disable trading' : 'Enable trading'}
        >
          {isSwitching ? (
            <Loader2 className="w-8 h-8 text-white animate-spin" />
          ) : (
            <Power className="w-8 h-8 text-white" />
          )}
        </button>

        {/* Status text */}
        <div className="flex-1">
          {state && <TradingStatus state={state} />}
        </div>
      </div>

      {/* Confirmation Modal */}
      {showConfirm && createPortal(
        <ConfirmModal
          onConfirm={handleDisable}
          onCancel={() => setShowConfirm(false)}
          isLoading={disableTrading.isLoading}
        />,
        document.body
      )}
    </>
  );
}

// Confirmation Modal Component
interface ConfirmModalProps {
  onConfirm: () => void;
  onCancel: () => void;
  isLoading: boolean;
}

function ConfirmModal({ onConfirm, onCancel, isLoading }: ConfirmModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
      <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-md mx-4 shadow-xl">
        <div className="flex items-center gap-3 mb-4">
          <div className="p-2 bg-yellow-500/20 rounded-full">
            <AlertTriangle className="w-6 h-6 text-yellow-500" />
          </div>
          <h3 className="text-lg font-bold text-white">Pause Trading?</h3>
        </div>

        <p className="text-gray-300 mb-4">
          This will pause all <strong className="text-yellow-400">new entry signals</strong>.
        </p>

        <ul className="text-sm text-gray-400 mb-6 space-y-2">
          <li className="flex items-start gap-2">
            <Check className="w-4 h-4 text-green-400 mt-0.5 flex-shrink-0" />
            <span>Existing positions will continue to be monitored</span>
          </li>
          <li className="flex items-start gap-2">
            <Check className="w-4 h-4 text-green-400 mt-0.5 flex-shrink-0" />
            <span>Take Profit and Stop Loss orders will still execute</span>
          </li>
          <li className="flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-yellow-500 mt-0.5 flex-shrink-0" />
            <span>No new trades will be entered until you re-enable</span>
          </li>
        </ul>

        <div className="flex gap-3">
          <button
            onClick={onCancel}
            disabled={isLoading}
            className="flex-1 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            onClick={onConfirm}
            disabled={isLoading}
            className="flex-1 px-4 py-2 bg-yellow-600 hover:bg-yellow-500 text-white rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
          >
            {isLoading ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Pausing...
              </>
            ) : (
              <>
                <Pause className="w-4 h-4" />
                Pause Trading
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
