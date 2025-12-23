import { useState, useEffect } from 'react';
import { createPortal } from 'react-dom';
import { apiService } from '../services/api';
import { AlertTriangle, Check, Loader2 } from 'lucide-react';

interface TradingModeState {
  dry_run: boolean;
  mode: 'paper' | 'live';
  mode_label: string;
  can_switch: boolean;
  switch_error?: string;
}

interface TradingModeToggleProps {
  onModeChange?: (mode: TradingModeState) => void;
}

export default function TradingModeToggle({ onModeChange }: TradingModeToggleProps = {}) {
  const [state, setState] = useState<TradingModeState | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSwitching, setIsSwitching] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchTradingMode();
  }, []);

  const fetchTradingMode = async () => {
    try {
      setIsLoading(true);
      const data = await apiService.getTradingMode();
      setState(data);
      onModeChange?.(data);
      setError(null);
    } catch (err) {
      setError('Failed to load trading mode');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleToggle = () => {
    if (!state) return;

    if (state.dry_run) {
      // Switching from PAPER to LIVE requires confirmation
      setShowConfirm(true);
    } else {
      // Switching from LIVE to PAPER is safe (no confirmation needed)
      switchMode(true);
    }
  };

  const switchMode = async (toDryRun: boolean) => {
    try {
      setIsSwitching(true);
      setShowConfirm(false);
      setError(null);
      const result = await apiService.setTradingMode(toDryRun);
      if (result.success) {
        const newState: TradingModeState = {
          dry_run: result.dry_run,
          mode: result.dry_run ? 'paper' : 'live',
          mode_label: result.dry_run ? 'Paper Trading' : 'Live Trading',
          can_switch: true,
        };
        setState(newState);
        onModeChange?.(newState);

        // Verify the switch after a brief delay to ensure backend persisted it
        setTimeout(() => {
          fetchTradingMode();
        }, 500);
      } else {
        setError('Failed to switch trading mode: ' + (result.message || 'Unknown error'));
      }
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Failed to switch trading mode';
      setError(errorMsg);
      console.error('Trading mode switch error:', err);
    } finally {
      setIsSwitching(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 px-4 py-2 bg-gray-800 rounded-lg">
        <Loader2 className="w-4 h-4 animate-spin text-gray-400" />
        <span className="text-sm text-gray-400">Loading...</span>
      </div>
    );
  }

  if (error && !state) {
    return (
      <div className="flex items-center gap-2 px-4 py-2 bg-red-500/10 border border-red-500/30 rounded-lg">
        <AlertTriangle className="w-4 h-4 text-red-500" />
        <span className="text-sm text-red-500">{error}</span>
      </div>
    );
  }

  const isPaper = state?.dry_run ?? true;
  const canSwitch = state?.can_switch ?? false;

  return (
    <>
      {error && state && (
        <div className="flex items-center gap-2 px-3 py-2 mb-2 bg-red-500/10 border border-red-500/30 rounded-lg">
          <AlertTriangle className="w-4 h-4 text-red-500 flex-shrink-0" />
          <span className="text-sm text-red-500">{error}</span>
          <button
            onClick={() => setError(null)}
            className="ml-auto text-red-500 hover:text-red-400 font-bold"
          >
            ×
          </button>
        </div>
      )}
      <button
        onClick={handleToggle}
        disabled={isSwitching || !canSwitch}
        title={!canSwitch && state?.switch_error ? state.switch_error : undefined}
        className={`
          w-full relative flex items-center gap-3 px-4 py-2 rounded-lg border transition-all
          ${isPaper
            ? 'bg-yellow-500/10 border-yellow-500/30 hover:bg-yellow-500/20'
            : 'bg-green-500/10 border-green-500/30 hover:bg-green-500/20'
          }
          ${isSwitching ? 'opacity-50 cursor-wait' : ''}
          ${!canSwitch ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
        `}
      >
        {isSwitching ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : (
          <div className={`w-3 h-3 rounded-full ${isPaper ? 'bg-yellow-500' : 'bg-green-500 animate-pulse'}`} />
        )}
        <div className="flex flex-col items-start flex-1">
          <span className={`text-sm font-semibold ${isPaper ? 'text-yellow-500' : 'text-green-500'}`}>
            {isSwitching ? 'Switching...' : (state?.mode_label || 'Paper Trading')}
          </span>
          <span className="text-xs text-gray-400">
            {!canSwitch && state?.switch_error
              ? state.switch_error
              : isSwitching
              ? 'Please wait...'
              : `Click to switch to ${isPaper ? 'Live' : 'Paper'}`
            }
          </span>
        </div>
      </button>

      {/* Confirmation Modal - rendered via Portal to avoid z-index issues */}
      {showConfirm && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="bg-gray-900 border border-gray-700 rounded-lg p-6 max-w-md mx-4">
            <div className="flex items-center gap-3 mb-4">
              <AlertTriangle className="w-8 h-8 text-red-500" />
              <h3 className="text-lg font-bold text-white">Switch to Live Trading?</h3>
            </div>
            <p className="text-gray-300 mb-4">
              You are about to switch from Paper Trading to <strong className="text-red-500">Live Trading</strong>.
              This means real money will be at risk.
            </p>
            <ul className="text-sm text-gray-400 mb-6 space-y-2">
              <li className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-yellow-500" />
                Real orders will be placed on Binance
              </li>
              <li className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-yellow-500" />
                Your actual balance will be used
              </li>
              <li className="flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 text-yellow-500" />
                Losses will be real
              </li>
            </ul>
            <div className="flex gap-3">
              <button
                onClick={() => setShowConfirm(false)}
                className="flex-1 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => switchMode(false)}
                className="flex-1 px-4 py-2 bg-red-600 hover:bg-red-500 text-white rounded-lg transition-colors flex items-center justify-center gap-2"
              >
                <Check className="w-4 h-4" />
                Enable Live Trading
              </button>
            </div>
          </div>
        </div>,
        document.body
      )}
    </>
  );
}
