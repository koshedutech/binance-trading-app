// Epic 11: Story 11.40 - Gap Analysis UI
// OverrideEntryButton component allows users to override soft blocks
import { useState } from 'react';
import { AlertTriangle, TrendingUp, TrendingDown, Loader2 } from 'lucide-react';
import { futuresApi } from '../../services/futuresApi';

interface OverrideEntryButtonProps {
  symbol: string;
  canOverride: boolean;
  softBlockCount: number;
  hardBlockCount: number;
  onOverrideSuccess?: () => void;
}

export default function OverrideEntryButton({
  symbol,
  canOverride,
  softBlockCount,
  hardBlockCount,
  onOverrideSuccess,
}: OverrideEntryButtonProps) {
  const [showModal, setShowModal] = useState(false);
  const [direction, setDirection] = useState<'LONG' | 'SHORT'>('LONG');
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleOverride = async () => {
    setLoading(true);
    setError(null);

    try {
      await futuresApi.overrideEntry(symbol, direction, reason || undefined);
      setShowModal(false);
      setReason('');
      onOverrideSuccess?.();
    } catch (err: unknown) {
      const errorMessage = err instanceof Error ? err.message :
        (err as { response?: { data?: { error?: string; message?: string } } })?.response?.data?.error ||
        (err as { response?: { data?: { error?: string; message?: string } } })?.response?.data?.message ||
        'Failed to override entry';
      setError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  // Don't render if can't override
  if (!canOverride || hardBlockCount > 0) {
    return null;
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setShowModal(true)}
        className="flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-lg bg-yellow-500/20 text-yellow-400 hover:bg-yellow-500/30 transition-colors border border-yellow-500/30"
      >
        <AlertTriangle className="w-4 h-4" />
        Override ({softBlockCount} soft block{softBlockCount !== 1 ? 's' : ''})
      </button>

      {/* Override Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50">
          <div className="bg-gray-800 rounded-xl border border-gray-700 shadow-xl max-w-md w-full">
            {/* Header */}
            <div className="px-6 py-4 border-b border-gray-700">
              <h3 className="text-lg font-semibold text-white">Override Entry for {symbol}</h3>
              <p className="text-sm text-gray-400 mt-1">
                This will bypass {softBlockCount} soft block{softBlockCount !== 1 ? 's' : ''} and trigger an entry
              </p>
            </div>

            {/* Content */}
            <div className="px-6 py-4 space-y-4">
              {/* Warning */}
              <div className="flex items-start gap-3 p-3 rounded-lg bg-yellow-500/10 border border-yellow-500/30">
                <AlertTriangle className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
                <div className="text-sm text-yellow-200">
                  <p className="font-medium">Warning: Manual Override</p>
                  <p className="text-yellow-300/80 mt-1">
                    You are about to override the decision engine's soft blocks. This action will be logged.
                  </p>
                </div>
              </div>

              {/* Direction Selection */}
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Position Direction
                </label>
                <div className="flex gap-3">
                  <button
                    type="button"
                    onClick={() => setDirection('LONG')}
                    className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-lg border transition-colors ${
                      direction === 'LONG'
                        ? 'bg-green-500/20 border-green-500 text-green-400'
                        : 'bg-gray-700 border-gray-600 text-gray-400 hover:border-gray-500'
                    }`}
                  >
                    <TrendingUp className="w-5 h-5" />
                    <span className="font-medium">LONG</span>
                  </button>
                  <button
                    type="button"
                    onClick={() => setDirection('SHORT')}
                    className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-lg border transition-colors ${
                      direction === 'SHORT'
                        ? 'bg-red-500/20 border-red-500 text-red-400'
                        : 'bg-gray-700 border-gray-600 text-gray-400 hover:border-gray-500'
                    }`}
                  >
                    <TrendingDown className="w-5 h-5" />
                    <span className="font-medium">SHORT</span>
                  </button>
                </div>
              </div>

              {/* Reason (Optional) */}
              <div>
                <label htmlFor="override-reason" className="block text-sm font-medium text-gray-300 mb-2">
                  Reason (Optional)
                </label>
                <textarea
                  id="override-reason"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                  placeholder="Why are you overriding? (helps with later analysis)"
                  rows={2}
                  className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-yellow-500/50 focus:border-yellow-500"
                />
              </div>

              {/* Error Message */}
              {error && (
                <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/30 text-sm text-red-400">
                  {error}
                </div>
              )}
            </div>

            {/* Footer */}
            <div className="px-6 py-4 border-t border-gray-700 flex justify-end gap-3">
              <button
                type="button"
                onClick={() => {
                  setShowModal(false);
                  setError(null);
                }}
                disabled={loading}
                className="px-4 py-2 text-sm font-medium text-gray-300 hover:text-white transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleOverride}
                disabled={loading}
                className={`flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-lg transition-colors disabled:opacity-50 ${
                  direction === 'LONG'
                    ? 'bg-green-600 hover:bg-green-500 text-white'
                    : 'bg-red-600 hover:bg-red-500 text-white'
                }`}
              >
                {loading && <Loader2 className="w-4 h-4 animate-spin" />}
                {loading ? 'Processing...' : `Enter ${direction}`}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
