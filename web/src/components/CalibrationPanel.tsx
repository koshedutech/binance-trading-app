// Calibration Panel Component
// Epic 11: Position Decision Engine
// Story 11.26: Calibration Data Lifecycle UI
// Story 11.27: Calibration Display & UI (Enhanced)

import { useState, useEffect, useCallback, useRef, useMemo, useId } from 'react';
import {
  RefreshCw,
  AlertTriangle,
  Loader2,
  BarChart3,
  Target,
  TrendingUp,
  TrendingDown,
  History,
  CheckCircle2,
  XCircle,
  Download,
  Info,
  ChevronDown,
  ChevronUp,
  Eye,
  CheckCircle,
} from 'lucide-react';
import {
  getCalibrationConfidence,
  getCalibrationHistory,
  getCalibrationData,
  resetCalibration,
  type CalibrationConfidence,
  type CalibrationHistoryRecord,
  type ScoreBucket,
} from '../api/decisionEngine';

// ==================== INTERFACES ====================

interface CalibrationPanelProps {
  strategy?: string;
  indicatorHash?: string;
  indicators?: string[];
  onReset?: () => void;
  onIndicatorChange?: () => void;
  isNewCalibration?: boolean; // Flag for indicator change notice
}

interface CalibrationBucketData {
  bucket: string;
  scoreRange: string;
  expectedWinRate: number;
  actualWinRate: number;
  delta: number;
  totalTrades: number;
  winningTrades: number;
  avgPnL: number;
  totalPnL: number;
  confidence: string;
  isAccurate: boolean;
}

interface ExpandedHistoryRecord extends CalibrationHistoryRecord {
  isExpanded?: boolean;
}

// ==================== CONSTANTS ====================

const CONFIDENCE_LEVELS = {
  insufficient: {
    label: 'Insufficient Data',
    color: 'text-gray-400',
    bgColor: 'bg-gray-500/20',
    borderColor: 'border-gray-500/30',
    icon: <AlertTriangle className="w-5 h-5" />,
  },
  low: {
    label: 'Low Confidence',
    color: 'text-yellow-400',
    bgColor: 'bg-yellow-500/20',
    borderColor: 'border-yellow-500/30',
    icon: <Target className="w-5 h-5" />,
  },
  medium: {
    label: 'Medium Confidence',
    color: 'text-blue-400',
    bgColor: 'bg-blue-500/20',
    borderColor: 'border-blue-500/30',
    icon: <TrendingUp className="w-5 h-5" />,
  },
  high: {
    label: 'High Confidence',
    color: 'text-green-400',
    bgColor: 'bg-green-500/20',
    borderColor: 'border-green-500/30',
    icon: <CheckCircle2 className="w-5 h-5" />,
  },
};

const STRATEGY_NAMES: Record<string, string> = {
  trend_following: 'Trend Following',
  mean_reversion: 'Mean Reversion',
  breakout: 'Breakout',
  range_trading: 'Range Trading',
};

// Expected win rates for each score bucket (based on midpoint scoring)
const EXPECTED_WIN_RATES: Record<string, number> = {
  '0_24': 12,   // 12% expected for lowest scores
  '25_49': 37,  // 37% expected for below-average scores
  '50_74': 62,  // 62% expected for above-average scores
  '75_100': 87, // 87% expected for highest scores
};

const BUCKET_LABELS: Record<string, string> = {
  '0_24': '0-24',
  '25_49': '25-49',
  '50_74': '50-74',
  '75_100': '75-100',
  'score_bucket_0_24': '0-24',
  'score_bucket_25_49': '25-49',
  'score_bucket_50_74': '50-74',
  'score_bucket_75_100': '75-100',
};

// ==================== UTILITY FUNCTIONS ====================

function getAccuracyStatus(delta: number): { label: string; color: string; bgColor: string } {
  const absDelta = Math.abs(delta);
  if (absDelta <= 5) {
    return { label: 'Excellent', color: 'text-green-400', bgColor: 'bg-green-500/20' };
  } else if (absDelta <= 10) {
    return { label: 'Good', color: 'text-lime-400', bgColor: 'bg-lime-500/20' };
  } else if (absDelta <= 15) {
    return { label: 'Fair', color: 'text-yellow-400', bgColor: 'bg-yellow-500/20' };
  } else {
    return { label: 'Needs Calibration', color: 'text-red-400', bgColor: 'bg-red-500/20' };
  }
}

function formatPercentage(value: number, decimals: number = 1): string {
  return `${value >= 0 ? '+' : ''}${value.toFixed(decimals)}%`;
}

// CSV escape function to prevent injection attacks (Issue #2 fix)
function escapeCSV(value: string | number): string {
  const str = String(value);
  // Escape fields containing commas, quotes, or newlines
  if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
    return `"${str.replace(/"/g, '""')}"`;
  }
  // Prevent formula injection by prefixing with single quote
  if (/^[=+\-@]/.test(str)) {
    return `'${str}`;
  }
  return str;
}

// Format relative time for "Last Updated" display (Issue #3 fix)
function formatRelativeTime(date: Date | string | undefined): string {
  if (!date) return 'Unknown';
  const now = new Date();
  const then = typeof date === 'string' ? new Date(date) : date;
  const diffMs = now.getTime() - then.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins} minute${diffMins !== 1 ? 's' : ''} ago`;
  if (diffHours < 24) return `${diffHours} hour${diffHours !== 1 ? 's' : ''} ago`;
  return `${diffDays} day${diffDays !== 1 ? 's' : ''} ago`;
}

function normalizeBucketKey(key: string): string {
  // Convert score_bucket_0_24 -> 0_24 for consistent lookup
  return key.replace('score_bucket_', '');
}

// ==================== COMPONENT ====================

export default function CalibrationPanel({
  strategy = 'trend_following',
  indicatorHash,
  indicators,
  onReset,
  onIndicatorChange,
  isNewCalibration = false,
}: CalibrationPanelProps) {
  const [confidence, setConfidence] = useState<CalibrationConfidence | null>(null);
  const [history, setHistory] = useState<ExpandedHistoryRecord[]>([]);
  const [bucketData, setBucketData] = useState<CalibrationBucketData[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [resetting, setResetting] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [showResetConfirm, setShowResetConfirm] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [showAccuracyTable, setShowAccuracyTable] = useState(true);
  const [exportingData, setExportingData] = useState(false);
  const [dismissedIndicatorNotice, setDismissedIndicatorNotice] = useState(false);
  const [showExportMenu, setShowExportMenu] = useState(false);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const refreshTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const resetConfirmTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const exportMenuId = useId();

  // Calculate overall accuracy metrics
  const overallAccuracy = useMemo(() => {
    if (bucketData.length === 0) return null;

    const totalTrades = bucketData.reduce((sum, b) => sum + b.totalTrades, 0);
    const totalWins = bucketData.reduce((sum, b) => sum + b.winningTrades, 0);
    const weightedExpected = bucketData.reduce((sum, b) => sum + (b.expectedWinRate * b.totalTrades), 0);
    const weightedActual = bucketData.reduce((sum, b) => sum + (b.actualWinRate * b.totalTrades), 0);

    const overallExpected = totalTrades > 0 ? weightedExpected / totalTrades : 0;
    const overallActual = totalTrades > 0 ? weightedActual / totalTrades : 0;
    const overallDelta = overallActual - overallExpected;

    return {
      totalTrades,
      totalWins,
      overallExpected,
      overallActual,
      overallDelta,
      status: getAccuracyStatus(overallDelta),
    };
  }, [bucketData]);

  // Load calibration data
  const loadCalibrationData = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const [confidenceRes, historyRes, dataRes] = await Promise.all([
        getCalibrationConfidence(strategy, indicatorHash),
        getCalibrationHistory(strategy),
        getCalibrationData(strategy, indicatorHash),
      ]);

      setConfidence(confidenceRes.confidence);
      setHistory((historyRes.history || []).map(h => ({ ...h, isExpanded: false })));

      // Process bucket data for accuracy table
      const processedBuckets: CalibrationBucketData[] = [];
      const buckets = dataRes.buckets || confidenceRes.confidence.bucket_breakdown || {};

      Object.entries(buckets).forEach(([key, value]) => {
        const normalizedKey = normalizeBucketKey(key);
        const expectedWinRate = EXPECTED_WIN_RATES[normalizedKey] || 50;
        const label = BUCKET_LABELS[key] || BUCKET_LABELS[normalizedKey] || normalizedKey;

        // Handle both numeric (trade count) and object (detailed bucket) formats
        let totalTrades = 0;
        let winningTrades = 0;
        let actualWinRate = 0;
        let avgPnL = 0;
        let totalPnL = 0;

        if (typeof value === 'number') {
          totalTrades = value;
          // Estimate from expected rate when only count available
          actualWinRate = expectedWinRate;
          winningTrades = Math.round(totalTrades * (actualWinRate / 100));
        } else if (typeof value === 'object' && value !== null) {
          const bucket = value as ScoreBucket;
          totalTrades = bucket.total_trades || 0;
          winningTrades = bucket.winning_trades || 0;
          actualWinRate = bucket.win_rate || (totalTrades > 0 ? (winningTrades / totalTrades) * 100 : 0);
          avgPnL = bucket.avg_pnl_percent || 0;
          totalPnL = bucket.total_pnl_percent || 0;
        }

        const delta = actualWinRate - expectedWinRate;
        const absDelta = Math.abs(delta);

        processedBuckets.push({
          bucket: key,
          scoreRange: label,
          expectedWinRate,
          actualWinRate,
          delta,
          totalTrades,
          winningTrades,
          avgPnL,
          totalPnL,
          confidence: totalTrades >= 50 ? 'high' : totalTrades >= 20 ? 'medium' : 'low',
          isAccurate: absDelta <= 10,
        });
      });

      // Sort by score range
      processedBuckets.sort((a, b) => {
        const aMin = parseInt(a.scoreRange.split('-')[0]) || 0;
        const bMin = parseInt(b.scoreRange.split('-')[0]) || 0;
        return aMin - bMin;
      });

      setBucketData(processedBuckets);
      setLastUpdated(new Date()); // Track when data was last loaded
    } catch (err) {
      console.error('Failed to load calibration data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load calibration data');
    } finally {
      setLoading(false);
    }
  }, [strategy, indicatorHash]);

  useEffect(() => {
    loadCalibrationData();
  }, [loadCalibrationData]);

  // Cleanup timeouts on unmount
  useEffect(() => {
    return () => {
      if (refreshTimeoutRef.current) {
        clearTimeout(refreshTimeoutRef.current);
      }
      if (resetConfirmTimeoutRef.current) {
        clearTimeout(resetConfirmTimeoutRef.current);
      }
    };
  }, []);

  // Debounced refresh handler (prevents rapid clicking)
  const handleRefresh = useCallback(() => {
    if (refreshing) return;
    setRefreshing(true);
    loadCalibrationData().finally(() => {
      refreshTimeoutRef.current = setTimeout(() => {
        setRefreshing(false);
      }, 1000);
    });
  }, [loadCalibrationData, refreshing]);

  // Handle reset with auto-timeout for confirmation dialog
  const handleReset = async () => {
    if (!showResetConfirm) {
      setShowResetConfirm(true);
      resetConfirmTimeoutRef.current = setTimeout(() => {
        setShowResetConfirm(false);
      }, 10000);
      return;
    }
    if (resetConfirmTimeoutRef.current) {
      clearTimeout(resetConfirmTimeoutRef.current);
    }

    setResetting(true);
    setShowResetConfirm(false);

    try {
      await resetCalibration({ strategy, indicator_hash: indicatorHash });
      await loadCalibrationData();
      onReset?.();
    } catch (err) {
      console.error('Failed to reset calibration:', err);
      setError(err instanceof Error ? err.message : 'Failed to reset calibration');
    } finally {
      setResetting(false);
    }
  };

  // Cancel reset confirmation
  const cancelReset = () => {
    if (resetConfirmTimeoutRef.current) {
      clearTimeout(resetConfirmTimeoutRef.current);
    }
    setShowResetConfirm(false);
  };

  // Export calibration data
  const handleExport = useCallback(async (format: 'json' | 'csv') => {
    setExportingData(true);
    try {
      const exportData = {
        strategy,
        indicatorHash: indicatorHash || 'default',
        indicators: indicators || [],
        exportedAt: new Date().toISOString(),
        confidence: confidence,
        buckets: bucketData.map(b => ({
          scoreRange: b.scoreRange,
          expectedWinRate: b.expectedWinRate,
          actualWinRate: b.actualWinRate,
          delta: b.delta,
          totalTrades: b.totalTrades,
          winningTrades: b.winningTrades,
          avgPnL: b.avgPnL,
          totalPnL: b.totalPnL,
          isAccurate: b.isAccurate,
        })),
        overall: overallAccuracy,
      };

      let content: string;
      let filename: string;
      let mimeType: string;

      if (format === 'json') {
        content = JSON.stringify(exportData, null, 2);
        filename = `calibration-${strategy}-${new Date().toISOString().split('T')[0]}.json`;
        mimeType = 'application/json';
      } else {
        // CSV format with proper escaping (Issue #2 fix - XSS/CSV injection prevention)
        const headers = ['Score Range', 'Expected Win %', 'Actual Win %', 'Delta', 'Trades', 'Wins', 'Avg PnL %', 'Accurate'];
        const rows = bucketData.map(b => [
          escapeCSV(b.scoreRange),
          escapeCSV(b.expectedWinRate.toFixed(1)),
          escapeCSV(b.actualWinRate.toFixed(1)),
          escapeCSV(formatPercentage(b.delta)),
          escapeCSV(b.totalTrades),
          escapeCSV(b.winningTrades),
          escapeCSV(b.avgPnL.toFixed(2)),
          escapeCSV(b.isAccurate ? 'Yes' : 'No'),
        ]);
        content = [headers.join(','), ...rows.map(r => r.join(','))].join('\n');
        filename = `calibration-${strategy}-${new Date().toISOString().split('T')[0]}.csv`;
        mimeType = 'text/csv';
      }
      setShowExportMenu(false); // Close menu after export

      // Create download
      const blob = new Blob([content], { type: mimeType });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error('Failed to export calibration data:', err);
    } finally {
      setExportingData(false);
    }
  }, [strategy, indicatorHash, indicators, confidence, bucketData, overallAccuracy]);

  // Toggle history record expansion
  const toggleHistoryExpand = (index: number) => {
    setHistory(prev => prev.map((h, i) =>
      i === index ? { ...h, isExpanded: !h.isExpanded } : h
    ));
  };

  // Loading state
  if (loading) {
    return (
      <div className="bg-gray-800/50 rounded-lg border border-gray-700/50 p-4">
        <div className="flex items-center justify-center py-8">
          <Loader2 className="w-6 h-6 animate-spin text-blue-400" />
          <span className="ml-2 text-gray-400">Loading calibration data...</span>
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-4">
        <div className="flex items-center gap-2 text-red-400">
          <AlertTriangle className="w-5 h-5" />
          <span>Error: {error}</span>
        </div>
        <button
          onClick={loadCalibrationData}
          className="mt-3 px-3 py-1 text-sm bg-red-600 hover:bg-red-700 text-white rounded"
        >
          Retry
        </button>
      </div>
    );
  }

  const levelConfig = confidence ? CONFIDENCE_LEVELS[confidence.level] : CONFIDENCE_LEVELS.insufficient;

  return (
    <div className="space-y-4">
      {/* Indicator Change Notice Banner (Story 11.27) */}
      {isNewCalibration && !dismissedIndicatorNotice && (
        <div className="bg-blue-900/30 border border-blue-500/40 rounded-lg p-4">
          <div className="flex items-start justify-between">
            <div className="flex items-start gap-3">
              <Info className="w-5 h-5 text-blue-400 mt-0.5" />
              <div>
                <h4 className="text-sm font-medium text-blue-300">New Calibration Started</h4>
                <p className="text-sm text-blue-200/70 mt-1">
                  You changed your indicator configuration. A new calibration has been started.
                  Previous calibration data has been archived and can be viewed in the history.
                </p>
                {indicators && indicators.length > 0 && (
                  <p className="text-xs text-blue-200/50 mt-2">
                    Current indicators: {indicators.join(', ')}
                  </p>
                )}
              </div>
            </div>
            <button
              onClick={() => setDismissedIndicatorNotice(true)}
              className="text-blue-400 hover:text-blue-300 p-1"
              title="Dismiss"
            >
              <XCircle className="w-4 h-4" />
            </button>
          </div>
          {onIndicatorChange && (
            <button
              onClick={onIndicatorChange}
              className="mt-3 text-xs text-blue-400 hover:text-blue-300 underline"
            >
              View previous calibration →
            </button>
          )}
        </div>
      )}

      {/* Reliability Warning (when calibration needs more trades) */}
      {confidence && !confidence.is_reliable && confidence.trades_needed > 0 && (
        <div className="bg-yellow-900/20 border border-yellow-500/30 rounded-lg p-3">
          <div className="flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-yellow-400" />
            <span className="text-sm text-yellow-300">
              Calibration needs <strong>{confidence.trades_needed}</strong> more trades to be reliable
            </span>
          </div>
        </div>
      )}

      {/* Main Calibration Card */}
      <div className={`rounded-lg border p-4 ${levelConfig.bgColor} ${levelConfig.borderColor}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className={levelConfig.color}>{levelConfig.icon}</div>
            <div>
              <h3 className="text-lg font-semibold text-white">Calibration: {STRATEGY_NAMES[strategy] || strategy}</h3>
              {indicators && indicators.length > 0 && (
                <p className="text-sm text-gray-400">
                  Indicators: {indicators.join(' + ')}
                </p>
              )}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {/* Export Button with accessible dropdown (Issue #1 fix) */}
            <div className="relative">
              <button
                disabled={exportingData || bucketData.length === 0}
                onClick={() => !exportingData && bucketData.length > 0 && setShowExportMenu(!showExportMenu)}
                aria-haspopup="menu"
                aria-expanded={showExportMenu}
                aria-controls={exportMenuId}
                aria-label="Export calibration data"
                className={`p-2 transition-colors ${exportingData || bucketData.length === 0 ? 'text-gray-600 cursor-not-allowed' : 'text-gray-400 hover:text-white'}`}
                title="Export Data"
              >
                <Download className={`w-4 h-4 ${exportingData ? 'animate-pulse' : ''}`} />
              </button>
              {showExportMenu && (
                <div
                  id={exportMenuId}
                  role="menu"
                  className="absolute right-0 top-full mt-1 z-10"
                  onBlur={(e) => {
                    // Close menu when focus leaves the dropdown
                    if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                      setShowExportMenu(false);
                    }
                  }}
                >
                  <div className="bg-gray-800 border border-gray-700 rounded-lg shadow-lg py-1 min-w-[100px]">
                    <button
                      role="menuitem"
                      onClick={() => handleExport('json')}
                      className="w-full px-3 py-1.5 text-left text-sm text-gray-300 hover:bg-gray-700 hover:text-white focus:bg-gray-700 focus:outline-none"
                    >
                      Export JSON
                    </button>
                    <button
                      role="menuitem"
                      onClick={() => handleExport('csv')}
                      className="w-full px-3 py-1.5 text-left text-sm text-gray-300 hover:bg-gray-700 hover:text-white focus:bg-gray-700 focus:outline-none"
                    >
                      Export CSV
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* Refresh Button */}
            <button
              onClick={handleRefresh}
              disabled={refreshing}
              className={`p-2 transition-colors ${refreshing ? 'text-gray-600 cursor-not-allowed' : 'text-gray-400 hover:text-white'}`}
              title={refreshing ? 'Refreshing...' : 'Refresh'}
            >
              <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            </button>

            {/* History Toggle */}
            <button
              onClick={() => setShowHistory(!showHistory)}
              className={`p-2 transition-colors ${showHistory ? 'text-blue-400' : 'text-gray-400 hover:text-white'}`}
              title="View History"
            >
              <History className="w-4 h-4" />
            </button>

            {/* Confidence Badge */}
            <span className={`px-3 py-1.5 rounded-full text-sm font-medium ${levelConfig.bgColor} ${levelConfig.color}`}>
              {levelConfig.label}
            </span>
          </div>
        </div>

        {/* Overall Accuracy Summary (Story 11.27) */}
        {overallAccuracy && (
          <div className="mt-4 bg-gray-800/50 rounded-lg p-4">
            <div className="flex items-center justify-between mb-3">
              <span className="text-gray-400 text-sm">Overall Accuracy</span>
              <span className={`px-2 py-0.5 rounded text-xs font-medium ${overallAccuracy.status.bgColor} ${overallAccuracy.status.color}`}>
                {overallAccuracy.status.label}
              </span>
            </div>
            <div className="grid grid-cols-3 gap-4 text-center">
              <div>
                <p className="text-gray-400 text-xs mb-1">Expected</p>
                <p className="text-xl font-bold text-white">{overallAccuracy.overallExpected.toFixed(1)}%</p>
              </div>
              <div>
                <p className="text-gray-400 text-xs mb-1">Actual</p>
                <p className="text-xl font-bold text-white">{overallAccuracy.overallActual.toFixed(1)}%</p>
              </div>
              <div>
                <p className="text-gray-400 text-xs mb-1">Delta</p>
                <div className="flex items-center justify-center gap-1">
                  {overallAccuracy.overallDelta > 0 ? (
                    <TrendingUp className="w-4 h-4 text-green-400" />
                  ) : overallAccuracy.overallDelta < 0 ? (
                    <TrendingDown className="w-4 h-4 text-red-400" />
                  ) : null}
                  <p className={`text-xl font-bold ${overallAccuracy.overallDelta >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                    {formatPercentage(overallAccuracy.overallDelta)}
                  </p>
                </div>
              </div>
            </div>
            <div className="mt-3 pt-3 border-t border-gray-700/50 text-center">
              <span className="text-xs text-gray-500">
                Total: {overallAccuracy.totalTrades} trades | {overallAccuracy.totalWins} wins
                {lastUpdated && ` | Last updated: ${formatRelativeTime(lastUpdated)}`}
              </span>
            </div>
          </div>
        )}

        {/* Accuracy Table Toggle (Issue #7 fix - aria-expanded) */}
        <button
          onClick={() => setShowAccuracyTable(!showAccuracyTable)}
          aria-expanded={showAccuracyTable}
          className="mt-4 flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors"
        >
          {showAccuracyTable ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          <BarChart3 className="w-4 h-4" />
          Score Bucket Accuracy
        </button>

        {/* Accuracy Table (Story 11.27 - Main Feature) */}
        {showAccuracyTable && bucketData.length > 0 && (
          <div className="mt-3 overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-gray-400 text-xs border-b border-gray-700/50">
                  <th className="text-left py-2 px-2">Score Range</th>
                  <th className="text-center py-2 px-2">Trades</th>
                  <th className="text-center py-2 px-2">Win Rate</th>
                  <th className="text-center py-2 px-2">Expected</th>
                  <th className="text-center py-2 px-2">Accuracy</th>
                </tr>
              </thead>
              <tbody>
                {bucketData.map((bucket) => (
                    <tr key={bucket.bucket} className="border-b border-gray-700/30">
                      <td className="py-2 px-2">
                        <span className="text-white font-medium">{bucket.scoreRange}</span>
                      </td>
                      <td className="text-center py-2 px-2">
                        <span className="text-gray-300">{bucket.totalTrades}</span>
                        <span className="text-gray-500 text-xs ml-1">({bucket.winningTrades}W)</span>
                      </td>
                      <td className="text-center py-2 px-2">
                        <span className="text-white font-medium">{bucket.actualWinRate.toFixed(0)}%</span>
                      </td>
                      <td className="text-center py-2 px-2">
                        <span className="text-gray-400">{bucket.expectedWinRate}%</span>
                      </td>
                      <td className="text-center py-2 px-2">
                        <div className="flex items-center justify-center gap-1">
                          {bucket.isAccurate ? (
                            <CheckCircle className="w-3 h-3 text-green-400" />
                          ) : (
                            <AlertTriangle className="w-3 h-3 text-yellow-400" />
                          )}
                          <span className={`font-medium ${bucket.delta >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                            {formatPercentage(bucket.delta)}
                          </span>
                        </div>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Stats Grid */}
        {confidence && (
          <div className="grid grid-cols-3 gap-4 mt-4">
            <div className="bg-gray-800/50 rounded-lg p-3">
              <div className="text-2xl font-bold text-white">{confidence.total_trades}</div>
              <div className="text-xs text-gray-400">Total Trades</div>
            </div>
            <div className="bg-gray-800/50 rounded-lg p-3">
              <div className="text-2xl font-bold text-white">
                {confidence.trades_needed > 0 ? confidence.trades_needed : '-'}
              </div>
              <div className="text-xs text-gray-400">Trades Needed</div>
            </div>
            <div className="bg-gray-800/50 rounded-lg p-3">
              <div className={`text-2xl font-bold ${confidence.is_reliable ? 'text-green-400' : 'text-yellow-400'}`}>
                {confidence.reliability_pct.toFixed(0)}%
              </div>
              <div className="text-xs text-gray-400">Reliability</div>
            </div>
          </div>
        )}

        {/* Reset Section */}
        <div className="mt-4 pt-4 border-t border-gray-700/50">
          {showResetConfirm ? (
            <div className="flex items-center justify-between bg-red-900/20 rounded-lg p-3">
              <span className="text-sm text-red-300">
                Reset all calibration data for {STRATEGY_NAMES[strategy]}?
              </span>
              <div className="flex gap-2">
                <button
                  onClick={cancelReset}
                  className="px-3 py-1 text-sm bg-gray-600 hover:bg-gray-700 text-white rounded"
                >
                  Cancel
                </button>
                <button
                  onClick={handleReset}
                  disabled={resetting}
                  className="px-3 py-1 text-sm bg-red-600 hover:bg-red-700 disabled:bg-gray-600 text-white rounded flex items-center gap-1"
                >
                  {resetting && <Loader2 className="w-3 h-3 animate-spin" />}
                  Confirm Reset
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={handleReset}
              className="flex items-center gap-2 px-3 py-2 text-sm bg-gray-700 hover:bg-gray-600 text-gray-300 hover:text-white rounded transition-colors"
            >
              <RefreshCw className="w-4 h-4" />
              Reset Calibration
            </button>
          )}
        </div>
      </div>

      {/* History Section with View Details (Story 11.27) */}
      {showHistory && (
        <div className="bg-gray-800/50 rounded-lg border border-gray-700/50 p-4">
          <h4 className="text-sm font-medium text-gray-300 mb-3 flex items-center gap-2">
            <History className="w-4 h-4" />
            Calibration History
          </h4>
          {history.length === 0 ? (
            <p className="text-sm text-gray-500 py-4 text-center">No archived calibrations yet</p>
          ) : (
            <div className="space-y-2 max-h-96 overflow-y-auto">
              {history.map((record, idx) => (
                <div
                  key={record.id || idx}
                  className="bg-gray-900/50 rounded overflow-hidden"
                >
                  <button
                    type="button"
                    aria-expanded={record.isExpanded}
                    className="w-full p-3 flex justify-between items-center cursor-pointer hover:bg-gray-800/50 text-left"
                    onClick={() => toggleHistoryExpand(idx)}
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <span className="text-gray-300 text-sm">Hash: </span>
                        <span className="text-gray-400 font-mono text-xs">{record.indicator_hash}</span>
                      </div>
                      <div className="mt-1 text-xs text-gray-500">
                        <span>{record.total_trades} trades</span>
                        <span className="mx-2">|</span>
                        <span>{record.archive_reason}</span>
                        <span className="mx-2">|</span>
                        <span>{new Date(record.archived_at).toLocaleDateString()}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Eye className="w-4 h-4 text-gray-400" />
                      {record.isExpanded ? (
                        <ChevronUp className="w-4 h-4 text-gray-400" />
                      ) : (
                        <ChevronDown className="w-4 h-4 text-gray-400" />
                      )}
                    </div>
                  </button>

                  {/* Expanded Details (Story 11.27 - View Previous Calibration) */}
                  {record.isExpanded && (
                    <div className="p-3 pt-0 border-t border-gray-700/30">
                      {record.indicators && record.indicators.length > 0 && (
                        <div className="mb-3">
                          <span className="text-xs text-gray-500">Indicators: </span>
                          <span className="text-xs text-gray-400">{record.indicators.join(', ')}</span>
                        </div>
                      )}
                      {record.buckets && Object.keys(record.buckets).length > 0 && (
                        <div className="grid grid-cols-4 gap-2 mt-2">
                          {Object.entries(record.buckets).map(([bucketKey, bucketData]) => {
                            const label = BUCKET_LABELS[bucketKey] || bucketKey;
                            const bucket = bucketData as ScoreBucket;
                            return (
                              <div key={bucketKey} className="bg-gray-800/50 rounded p-2 text-center">
                                <div className="text-xs text-gray-500 mb-1">{label}</div>
                                <div className="text-sm font-medium text-white">{bucket.total_trades || 0}</div>
                                <div className="text-xs text-gray-400">
                                  {(bucket.win_rate || 0).toFixed(0)}% WR
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
