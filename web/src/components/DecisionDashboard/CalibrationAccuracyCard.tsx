// Epic 11: Position Decision Engine - Story 11.22: Performance Dashboard
// Calibration Accuracy Card Component

import { useMemo } from 'react';
import { CalibrationSummary, getAccuracyStatus, formatPercentage } from '../../types/decision-dashboard';
import { CheckCircle, AlertTriangle, XCircle, TrendingUp, TrendingDown } from 'lucide-react';

interface CalibrationAccuracyCardProps {
  data: CalibrationSummary | null;
  isLoading?: boolean;
}

export function CalibrationAccuracyCard({ data, isLoading }: CalibrationAccuracyCardProps) {
  const overallStatus = useMemo(() => {
    if (!data) return null;
    return getAccuracyStatus(data.overallDelta);
  }, [data]);

  const DeltaIcon = ({ delta }: { delta: number }) => {
    if (Math.abs(delta) <= 5) {
      return <CheckCircle className="w-4 h-4 text-green-400" />;
    } else if (Math.abs(delta) <= 15) {
      return <AlertTriangle className="w-4 h-4 text-yellow-400" />;
    }
    return <XCircle className="w-4 h-4 text-red-400" />;
  };

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 h-80">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Calibration Accuracy</h3>
        </div>
        <div className="h-64 flex items-center justify-center">
          <div className="animate-pulse text-gray-400">Loading...</div>
        </div>
      </div>
    );
  }

  if (!data) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 h-80">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-white font-semibold">Calibration Accuracy</h3>
        </div>
        <div className="h-64 flex items-center justify-center">
          <p className="text-gray-400">No calibration data available</p>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-white font-semibold">Calibration Accuracy</h3>
        <span className="text-xs text-gray-400 bg-gray-700 px-2 py-1 rounded">
          {data.strategy}
        </span>
      </div>

      {/* Overall accuracy */}
      <div className="bg-gray-700/50 rounded-lg p-4 mb-4">
        <div className="flex items-center justify-between mb-2">
          <span className="text-gray-400 text-sm">Overall Accuracy</span>
          <div
            className="px-2 py-1 rounded text-xs font-medium"
            style={{ backgroundColor: `${overallStatus?.color}20`, color: overallStatus?.color }}
          >
            {overallStatus?.label}
          </div>
        </div>

        <div className="grid grid-cols-3 gap-4 text-center">
          <div>
            <p className="text-gray-400 text-xs mb-1">Expected</p>
            <p className="text-xl font-bold text-white">{data.overallExpected.toFixed(1)}%</p>
          </div>
          <div>
            <p className="text-gray-400 text-xs mb-1">Actual</p>
            <p className="text-xl font-bold text-white">{data.overallActual.toFixed(1)}%</p>
          </div>
          <div>
            <p className="text-gray-400 text-xs mb-1">Delta</p>
            <div className="flex items-center justify-center gap-1">
              {data.overallDelta > 0 ? (
                <TrendingUp className="w-4 h-4 text-green-400" />
              ) : data.overallDelta < 0 ? (
                <TrendingDown className="w-4 h-4 text-red-400" />
              ) : null}
              <p
                className={`text-xl font-bold ${
                  data.overallDelta >= 0 ? 'text-green-400' : 'text-red-400'
                }`}
              >
                {formatPercentage(data.overallDelta)}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Reliability warning */}
      {!data.isReliable && (
        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-3 mb-4">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-yellow-400" />
            <span className="text-yellow-400 text-sm">
              Need {data.minimumTradesRequired - data.totalCalibrationTrades} more trades for reliable calibration
            </span>
          </div>
        </div>
      )}

      {/* Bucket breakdown */}
      <div className="space-y-2">
        <p className="text-gray-400 text-xs font-medium mb-2">Score Bucket Breakdown</p>
        {data.bucketAccuracies.map((bucket) => (
          <div
            key={bucket.scoreBucket}
            className="flex items-center justify-between bg-gray-700/30 rounded p-2"
          >
            <div className="flex items-center gap-2">
              <DeltaIcon delta={bucket.delta} />
              <span className="text-white text-sm font-medium">{bucket.scoreBucket}</span>
            </div>
            <div className="flex items-center gap-4 text-sm">
              <span className="text-gray-400">
                {bucket.expectedWinRate.toFixed(0)}% expected
              </span>
              <span className="text-white font-medium">
                {bucket.actualWinRate.toFixed(0)}% actual
              </span>
              <span
                className={`font-medium ${
                  bucket.delta >= 0 ? 'text-green-400' : 'text-red-400'
                }`}
              >
                {formatPercentage(bucket.delta)}
              </span>
              <span className="text-gray-500 text-xs">
                ({bucket.totalTrades} trades)
              </span>
            </div>
          </div>
        ))}
      </div>

      {/* Footer info */}
      <div className="mt-4 pt-4 border-t border-gray-700 flex justify-between text-xs text-gray-500">
        <span>Total trades: {data.totalCalibrationTrades}</span>
        <span>Last updated: {new Date(data.lastUpdated).toLocaleString()}</span>
      </div>
    </div>
  );
}

export default CalibrationAccuracyCard;
