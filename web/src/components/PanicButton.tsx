import { useState } from 'react';
import { AlertTriangle, X, Loader2 } from 'lucide-react';
import { futuresApi } from '../services/futuresApi';
import { apiService } from '../services/api';

interface PanicButtonProps {
  type: 'futures' | 'spot' | 'all';
  onComplete?: () => void;
  className?: string;
}

export default function PanicButton({ type, onComplete, className = '' }: PanicButtonProps) {
  const [showConfirm, setShowConfirm] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const [result, setResult] = useState<{
    success: boolean;
    message: string;
    closed: number;
    total: number;
    errors: string[];
  } | null>(null);

  const handlePanicClose = async () => {
    setIsClosing(true);
    setResult(null);

    try {
      let futuresResult = null;
      let spotResult = null;

      if (type === 'futures' || type === 'all') {
        futuresResult = await futuresApi.closeAllPositions();
      }

      if (type === 'spot' || type === 'all') {
        spotResult = await apiService.closeAllPositions();
      }

      // Combine results
      const totalClosed = (futuresResult?.closed || 0) + (spotResult?.closed || 0);
      const totalPositions = (futuresResult?.total || 0) + (spotResult?.total || 0);
      const allErrors = [
        ...(futuresResult?.errors || []).map(e => `[Futures] ${e}`),
        ...(spotResult?.errors || []).map(e => `[Spot] ${e}`),
      ];

      setResult({
        success: allErrors.length === 0,
        message: totalPositions === 0
          ? 'No open positions to close'
          : `Closed ${totalClosed}/${totalPositions} positions`,
        closed: totalClosed,
        total: totalPositions,
        errors: allErrors,
      });

      if (onComplete) {
        onComplete();
      }
    } catch (error: any) {
      setResult({
        success: false,
        message: error?.response?.data?.error || error.message || 'Failed to close positions',
        closed: 0,
        total: 0,
        errors: [error?.response?.data?.error || error.message],
      });
    } finally {
      setIsClosing(false);
    }
  };

  const getButtonLabel = () => {
    switch (type) {
      case 'futures':
        return 'Close All Futures';
      case 'spot':
        return 'Close All Spot';
      case 'all':
        return 'PANIC: Close All';
    }
  };

  return (
    <>
      <button
        onClick={() => setShowConfirm(true)}
        className={`
          bg-red-600 hover:bg-red-700 text-white font-bold py-2 px-4 rounded-lg
          transition-all duration-200 flex items-center gap-2
          hover:scale-105 hover:shadow-lg hover:shadow-red-500/25
          ${className}
        `}
      >
        <AlertTriangle className="w-5 h-5" />
        {getButtonLabel()}
      </button>

      {/* Confirmation Modal */}
      {showConfirm && (
        <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
          <div className="bg-gray-800 rounded-xl max-w-md w-full shadow-2xl border border-red-500/50">
            {/* Header */}
            <div className="flex items-center justify-between p-4 border-b border-gray-700">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-red-500/20 rounded-lg">
                  <AlertTriangle className="w-6 h-6 text-red-500" />
                </div>
                <h3 className="text-lg font-bold text-white">Emergency Close</h3>
              </div>
              <button
                onClick={() => {
                  setShowConfirm(false);
                  setResult(null);
                }}
                className="text-gray-400 hover:text-white transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Body */}
            <div className="p-4 space-y-4">
              {!result ? (
                <>
                  <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4">
                    <p className="text-red-400 font-medium mb-2">
                      This will immediately close ALL {type === 'all' ? '' : type} positions at market price!
                    </p>
                    <ul className="text-sm text-gray-400 space-y-1">
                      <li>- All open positions will be closed</li>
                      <li>- Market orders will be used (instant execution)</li>
                      <li>- This action cannot be undone</li>
                      <li>- Slippage may occur during volatile markets</li>
                    </ul>
                  </div>

                  <div className="flex gap-3">
                    <button
                      onClick={() => setShowConfirm(false)}
                      className="flex-1 py-2 px-4 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={handlePanicClose}
                      disabled={isClosing}
                      className="flex-1 py-2 px-4 bg-red-600 hover:bg-red-700 text-white font-bold rounded-lg transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
                    >
                      {isClosing ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          Closing...
                        </>
                      ) : (
                        <>
                          <AlertTriangle className="w-4 h-4" />
                          Confirm Close All
                        </>
                      )}
                    </button>
                  </div>
                </>
              ) : (
                <>
                  {/* Result Display */}
                  <div className={`rounded-lg p-4 ${result.success ? 'bg-green-500/10 border border-green-500/30' : 'bg-yellow-500/10 border border-yellow-500/30'}`}>
                    <p className={`font-medium mb-2 ${result.success ? 'text-green-400' : 'text-yellow-400'}`}>
                      {result.message}
                    </p>
                    {result.total > 0 && (
                      <div className="text-sm text-gray-400">
                        <p>Positions closed: {result.closed}/{result.total}</p>
                      </div>
                    )}
                    {result.errors.length > 0 && (
                      <div className="mt-2">
                        <p className="text-sm text-red-400 mb-1">Errors:</p>
                        <ul className="text-xs text-red-300 space-y-1">
                          {result.errors.map((err, i) => (
                            <li key={i}>- {err}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>

                  <button
                    onClick={() => {
                      setShowConfirm(false);
                      setResult(null);
                    }}
                    className="w-full py-2 px-4 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
                  >
                    Close
                  </button>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
