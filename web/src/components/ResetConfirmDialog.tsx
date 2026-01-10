import { AlertTriangle, X, Loader2, CheckCircle2 } from 'lucide-react';

interface SettingDiff {
  path: string;
  current: any;
  default: any;
  risk_level: 'high' | 'medium' | 'low';
  impact?: string;
  recommendation?: string;
}

interface ResetConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  configType: string;
  loading: boolean;
  allMatch: boolean;
  differences: SettingDiff[];
  totalChanges: number;
}

export default function ResetConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  configType,
  loading,
  allMatch,
  differences,
  totalChanges,
}: ResetConfirmDialogProps) {
  if (!open) return null;

  // Format value for display
  const formatValue = (value: any): string => {
    if (value === null || value === undefined) return 'N/A';
    if (typeof value === 'boolean') return value ? 'Yes' : 'No';
    if (Array.isArray(value)) return value.join(', ');
    if (typeof value === 'object') return JSON.stringify(value);
    if (typeof value === 'number') {
      // Format numbers nicely
      return value.toLocaleString();
    }
    return String(value);
  };

  // Get chip color based on risk level
  const getRiskColor = (risk: 'high' | 'medium' | 'low') => {
    switch (risk) {
      case 'high':
        return 'bg-red-500/20 text-red-400 border-red-500/30';
      case 'medium':
        return 'bg-orange-500/20 text-orange-400 border-orange-500/30';
      case 'low':
        return 'bg-green-500/20 text-green-400 border-green-500/30';
      default:
        return 'bg-gray-500/20 text-gray-400 border-gray-500/30';
    }
  };

  // Separate changed and unchanged items
  const changedItems = differences.filter(
    (diff) => JSON.stringify(diff.current) !== JSON.stringify(diff.default)
  );
  const unchangedItems = differences.filter(
    (diff) => JSON.stringify(diff.current) === JSON.stringify(diff.default)
  );

  // Count risk levels (only for changed items)
  const riskCounts = changedItems.reduce(
    (acc, diff) => {
      acc[diff.risk_level] = (acc[diff.risk_level] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>
  );

  const hasHighRisk = (riskCounts.high || 0) > 0;

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-xl max-w-4xl w-full max-h-[90vh] shadow-2xl border border-gray-700 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-700">
          <div className="flex items-center gap-3">
            {hasHighRisk && !allMatch && (
              <div className="p-2 bg-yellow-500/20 rounded-lg">
                <AlertTriangle className="w-6 h-6 text-yellow-500" />
              </div>
            )}
            <h3 className="text-lg font-bold text-white">{title}</h3>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {loading ? (
            <div className="flex flex-col items-center justify-center py-12 space-y-3">
              <Loader2 className="w-8 h-8 text-blue-500 animate-spin" />
              <p className="text-gray-400">Loading preview...</p>
            </div>
          ) : (
            <>
              {/* All Match Info Banner */}
              {allMatch && (
                <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-4 flex items-start gap-3">
                  <CheckCircle2 className="w-6 h-6 text-green-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-green-400 font-medium">
                      All settings already match defaults
                    </p>
                    <p className="text-sm text-gray-400 mt-1">
                      No changes needed for {configType} configuration.
                    </p>
                  </div>
                </div>
              )}

              {/* Risk Summary */}
              {hasHighRisk && !allMatch && (
                <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4">
                  <div className="flex items-start gap-3">
                    <AlertTriangle className="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" />
                    <div className="flex-1">
                      <p className="text-yellow-400 font-medium mb-2">
                        Warning: High-risk changes detected
                      </p>
                      <div className="flex flex-wrap gap-2 text-sm">
                        {riskCounts.high > 0 && (
                          <span className="px-2 py-1 rounded bg-red-500/20 text-red-400 border border-red-500/30">
                            {riskCounts.high} High Risk
                          </span>
                        )}
                        {riskCounts.medium > 0 && (
                          <span className="px-2 py-1 rounded bg-orange-500/20 text-orange-400 border border-orange-500/30">
                            {riskCounts.medium} Medium Risk
                          </span>
                        )}
                        {riskCounts.low > 0 && (
                          <span className="px-2 py-1 rounded bg-green-500/20 text-green-400 border border-green-500/30">
                            {riskCounts.low} Low Risk
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Changes Summary */}
              {!allMatch && (
                <div className="bg-blue-500/10 border border-blue-500/30 rounded-lg p-4">
                  <p className="text-blue-400 font-medium">
                    {changedItems.length} setting{changedItems.length !== 1 ? 's' : ''} will be reset to default values
                  </p>
                  {unchangedItems.length > 0 && (
                    <p className="text-sm text-gray-400 mt-1">
                      {unchangedItems.length} setting{unchangedItems.length !== 1 ? 's are' : ' is'} already at default
                    </p>
                  )}
                </div>
              )}

              {/* Differences Table */}
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-gray-700">
                      <th className="text-left py-2 px-3 text-sm font-semibold text-gray-300">
                        Setting
                      </th>
                      <th className="text-left py-2 px-3 text-sm font-semibold text-gray-300">
                        Current Value
                      </th>
                      <th className="text-left py-2 px-3 text-sm font-semibold text-gray-300">
                        Default Value
                      </th>
                      <th className="text-left py-2 px-3 text-sm font-semibold text-gray-300">
                        Risk Level
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {/* Changed Items Section */}
                    {changedItems.length > 0 && (
                      <>
                        <tr>
                          <td colSpan={4} className="py-2 px-3">
                            <div className="flex items-center gap-2">
                              <div className="h-px flex-1 bg-amber-500/30"></div>
                              <span className="text-xs font-semibold text-amber-400 uppercase tracking-wider">
                                Changes to Apply ({changedItems.length})
                              </span>
                              <div className="h-px flex-1 bg-amber-500/30"></div>
                            </div>
                          </td>
                        </tr>
                        {changedItems.map((diff, index) => (
                          <tr
                            key={`changed-${index}`}
                            className="border-b border-gray-700/50 bg-amber-500/5 hover:bg-amber-500/10 transition-colors"
                          >
                            <td className="py-3 px-3">
                              <div className="flex flex-col">
                                <span className="text-sm text-white font-medium">
                                  {diff.path}
                                </span>
                                {diff.impact && (
                                  <span className="text-xs text-gray-400 mt-1">
                                    {diff.impact}
                                  </span>
                                )}
                                {diff.recommendation && (
                                  <span className="text-xs text-blue-400 mt-1">
                                    💡 {diff.recommendation}
                                  </span>
                                )}
                              </div>
                            </td>
                            <td className="py-3 px-3 text-sm font-mono">
                              <span className="text-orange-400 font-semibold">
                                {formatValue(diff.current)}
                              </span>
                            </td>
                            <td className="py-3 px-3 text-sm font-mono">
                              <span className="text-green-400 font-semibold">
                                {formatValue(diff.default)}
                              </span>
                            </td>
                            <td className="py-3 px-3">
                              <span
                                className={`inline-block px-2 py-1 rounded text-xs font-medium border ${getRiskColor(
                                  diff.risk_level
                                )}`}
                              >
                                {diff.risk_level.toUpperCase()}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </>
                    )}

                    {/* Unchanged Items Section */}
                    {unchangedItems.length > 0 && (
                      <>
                        <tr>
                          <td colSpan={4} className="py-2 px-3">
                            <div className="flex items-center gap-2">
                              <div className="h-px flex-1 bg-gray-600/30"></div>
                              <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">
                                Already at Default ({unchangedItems.length})
                              </span>
                              <div className="h-px flex-1 bg-gray-600/30"></div>
                            </div>
                          </td>
                        </tr>
                        {unchangedItems.map((diff, index) => (
                          <tr
                            key={`unchanged-${index}`}
                            className="border-b border-gray-700/50 hover:bg-gray-700/20 transition-colors opacity-60"
                          >
                            <td className="py-3 px-3">
                              <div className="flex flex-col">
                                <span className="text-sm text-gray-400 font-medium">
                                  {diff.path}
                                </span>
                                {diff.impact && (
                                  <span className="text-xs text-gray-500 mt-1">
                                    {diff.impact}
                                  </span>
                                )}
                              </div>
                            </td>
                            <td className="py-3 px-3 text-sm text-gray-500 font-mono">
                              {formatValue(diff.current)}
                            </td>
                            <td className="py-3 px-3 text-sm text-gray-500 font-mono">
                              {formatValue(diff.default)}
                            </td>
                            <td className="py-3 px-3">
                              <span className="inline-block px-2 py-1 rounded text-xs font-medium border border-gray-600/30 bg-gray-600/10 text-gray-500">
                                OK
                              </span>
                            </td>
                          </tr>
                        ))}
                      </>
                    )}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-700 flex gap-3">
          {allMatch ? (
            <button
              onClick={onClose}
              className="flex-1 py-2 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors"
            >
              Close
            </button>
          ) : (
            <>
              <button
                onClick={onClose}
                className="flex-1 py-2 px-4 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={onConfirm}
                disabled={loading}
                className="flex-1 py-2 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              >
                {loading ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Loading...
                  </>
                ) : (
                  <>
                    Reset to Defaults
                  </>
                )}
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
