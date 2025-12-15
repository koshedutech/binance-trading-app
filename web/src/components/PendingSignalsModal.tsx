import { useState, useEffect } from 'react';
import { X, CheckCircle, XCircle, TrendingUp, TrendingDown } from 'lucide-react';
import { apiService } from '../services/api';

interface PendingSignal {
  id: number;
  strategy_name: string;
  symbol: string;
  signal_type: string;
  entry_price: number;
  current_price: number;
  stop_loss?: number;
  take_profit?: number;
  reason?: string;
  conditions_met: any;
  timestamp: string;
  status: string;
}

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export default function PendingSignalsModal({ isOpen, onClose }: Props) {
  const [pendingSignals, setPendingSignals] = useState<PendingSignal[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      fetchPendingSignals();
      // Poll for new signals every 2 seconds while modal is open
      const interval = setInterval(fetchPendingSignals, 2000);
      return () => clearInterval(interval);
    }
  }, [isOpen]);

  const fetchPendingSignals = async () => {
    try {
      const signals = await apiService.getPendingSignals();
      setPendingSignals(signals);
    } catch (err) {
      console.error('Failed to fetch pending signals:', err);
      setError('Failed to load pending signals');
    }
  };

  const handleConfirm = async (id: number) => {
    setLoading(true);
    setError(null);
    try {
      await apiService.confirmPendingSignal(id, 'CONFIRM');
      // Refresh signals after confirmation
      await fetchPendingSignals();
    } catch (err) {
      setError('Failed to confirm signal');
      console.error('Failed to confirm signal:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleReject = async (id: number) => {
    setLoading(true);
    setError(null);
    try {
      await apiService.confirmPendingSignal(id, 'REJECT');
      // Refresh signals after rejection
      await fetchPendingSignals();
    } catch (err) {
      setError('Failed to reject signal');
      console.error('Failed to reject signal:', err);
    } finally {
      setLoading(false);
    }
  };

  const renderConditions = (conditions: any) => {
    if (!conditions) return null;

    const conditionsMet = conditions.conditions_met || [];
    const conditionsFailed = conditions.conditions_failed || [];

    return (
      <div className="mt-3 space-y-2">
        <div className="text-sm font-semibold text-gray-300">Conditions Met:</div>
        {conditionsMet.length > 0 ? (
          <div className="space-y-1">
            {conditionsMet.map((condition: string, idx: number) => (
              <div key={idx} className="flex items-start text-sm text-gray-400">
                <CheckCircle className="w-4 h-4 text-green-500 mr-2 mt-0.5 flex-shrink-0" />
                <span>{condition}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-sm text-gray-500">No conditions met</div>
        )}

        {conditionsFailed.length > 0 && (
          <>
            <div className="text-sm font-semibold text-gray-300 mt-3">Conditions Not Met:</div>
            <div className="space-y-1">
              {conditionsFailed.map((condition: string, idx: number) => (
                <div key={idx} className="flex items-start text-sm text-gray-500">
                  <XCircle className="w-4 h-4 text-red-500 mr-2 mt-0.5 flex-shrink-0" />
                  <span>{condition}</span>
                </div>
              ))}
            </div>
          </>
        )}
      </div>
    );
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:p-0">
        {/* Backdrop */}
        <div
          className="fixed inset-0 transition-opacity bg-black bg-opacity-75"
          onClick={onClose}
        />

        {/* Modal */}
        <div className="relative inline-block w-full max-w-4xl p-6 my-8 overflow-hidden text-left align-middle transition-all transform bg-dark-800 shadow-xl rounded-xl">
          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-2xl font-bold text-white flex items-center">
              <span className="w-3 h-3 bg-yellow-500 rounded-full mr-3 animate-pulse"></span>
              Pending Trade Signals
            </h3>
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-white transition-colors"
            >
              <X className="w-6 h-6" />
            </button>
          </div>

          {/* Error Message */}
          {error && (
            <div className="mb-4 p-3 bg-red-500/10 border border-red-500 rounded-lg text-red-500 text-sm">
              {error}
            </div>
          )}

          {/* Signals List */}
          <div className="space-y-4 max-h-96 overflow-y-auto">
            {pendingSignals.length === 0 ? (
              <div className="text-center py-12 text-gray-400">
                <div className="text-lg mb-2">No pending signals</div>
                <div className="text-sm">All signals have been processed or autopilot is enabled</div>
              </div>
            ) : (
              pendingSignals.map((signal) => (
                <div
                  key={signal.id}
                  className="bg-dark-700 rounded-lg p-4 border border-dark-600 hover:border-primary-500 transition-colors"
                >
                  {/* Signal Header */}
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center">
                      {signal.signal_type === 'BUY' ? (
                        <TrendingUp className="w-5 h-5 text-green-500 mr-2" />
                      ) : (
                        <TrendingDown className="w-5 h-5 text-red-500 mr-2" />
                      )}
                      <span className="text-lg font-semibold text-white">{signal.symbol}</span>
                      <span
                        className={`ml-3 px-2 py-1 rounded text-xs font-semibold ${
                          signal.signal_type === 'BUY'
                            ? 'bg-green-500/20 text-green-500'
                            : 'bg-red-500/20 text-red-500'
                        }`}
                      >
                        {signal.signal_type}
                      </span>
                    </div>
                    <div className="text-sm text-gray-400">{signal.strategy_name}</div>
                  </div>

                  {/* Signal Details */}
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-3">
                    <div>
                      <div className="text-xs text-gray-400">Entry Price</div>
                      <div className="text-sm font-semibold text-white">
                        ${signal.entry_price.toFixed(2)}
                      </div>
                    </div>
                    <div>
                      <div className="text-xs text-gray-400">Current Price</div>
                      <div className="text-sm font-semibold text-white">
                        ${signal.current_price.toFixed(2)}
                      </div>
                    </div>
                    {signal.stop_loss && (
                      <div>
                        <div className="text-xs text-gray-400">Stop Loss</div>
                        <div className="text-sm font-semibold text-red-400">
                          ${signal.stop_loss.toFixed(2)}
                        </div>
                      </div>
                    )}
                    {signal.take_profit && (
                      <div>
                        <div className="text-xs text-gray-400">Take Profit</div>
                        <div className="text-sm font-semibold text-green-400">
                          ${signal.take_profit.toFixed(2)}
                        </div>
                      </div>
                    )}
                  </div>

                  {/* Reason */}
                  {signal.reason && (
                    <div className="mb-3 p-2 bg-dark-600 rounded text-sm text-gray-300">
                      <span className="font-semibold text-gray-400">Reason: </span>
                      {signal.reason}
                    </div>
                  )}

                  {/* Conditions */}
                  {renderConditions(signal.conditions_met)}

                  {/* Action Buttons */}
                  <div className="flex gap-3 mt-4">
                    <button
                      onClick={() => handleConfirm(signal.id)}
                      disabled={loading}
                      className="flex-1 bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded-lg font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center animate-pulse"
                    >
                      <CheckCircle className="w-5 h-5 mr-2" />
                      Confirm Trade
                    </button>
                    <button
                      onClick={() => handleReject(signal.id)}
                      disabled={loading}
                      className="flex-1 bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded-lg font-semibold transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center"
                    >
                      <XCircle className="w-5 h-5 mr-2" />
                      Reject
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
