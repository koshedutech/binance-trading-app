import React, { useState, useEffect } from 'react';
import { RefreshCw, AlertTriangle, Info, CheckCircle2, Loader2 } from 'lucide-react';
import {
  loadModeDefaults,
  loadCircuitBreakerDefaults,
  loadLLMConfigDefaults,
  loadCapitalAllocationDefaults,
  loadHedgeDefaults,
  loadScalpReentryDefaults,
  loadAllModesDefaults,
  ConfigResetPreview,
  ConfigResetResult,
} from '../services/futuresApi';
import ResetConfirmDialog from '../components/ResetConfirmDialog';

interface SettingGroup {
  id: string;
  name: string;
  description: string;
  riskLevel: 'high' | 'medium' | 'low';
  resetFn: (preview: boolean) => Promise<ConfigResetPreview | ConfigResetResult>;
  configType: string;
}

const settingGroups: SettingGroup[] = [
  {
    id: 'ultra-fast',
    name: 'Ultra Fast Mode',
    description: 'Reset Ultra Fast trading mode configuration to default values',
    riskLevel: 'high',
    resetFn: (preview) => loadModeDefaults('ultra_fast', preview),
    configType: 'Ultra Fast Mode',
  },
  {
    id: 'scalp',
    name: 'Scalp Mode',
    description: 'Reset Scalp trading mode configuration to default values',
    riskLevel: 'medium',
    resetFn: (preview) => loadModeDefaults('scalp', preview),
    configType: 'Scalp Mode',
  },
  {
    id: 'scalp-reentry',
    name: 'Scalp Re-entry Mode',
    description: 'Reset Scalp Re-entry mode configuration to default values',
    riskLevel: 'medium',
    resetFn: (preview) => loadScalpReentryDefaults(preview),
    configType: 'Scalp Re-entry Mode',
  },
  {
    id: 'swing',
    name: 'Swing Mode',
    description: 'Reset Swing trading mode configuration to default values',
    riskLevel: 'low',
    resetFn: (preview) => loadModeDefaults('swing', preview),
    configType: 'Swing Mode',
  },
  {
    id: 'position',
    name: 'Position Mode',
    description: 'Reset Position trading mode configuration to default values',
    riskLevel: 'low',
    resetFn: (preview) => loadModeDefaults('position', preview),
    configType: 'Position Mode',
  },
  {
    id: 'hedge',
    name: 'Hedge Mode Settings',
    description: 'Reset hedge mode settings to default values',
    riskLevel: 'high',
    resetFn: (preview) => loadHedgeDefaults(preview),
    configType: 'Hedge Mode',
  },
  {
    id: 'circuit-breaker',
    name: 'Circuit Breaker Settings',
    description: 'Reset circuit breaker protection settings to default values',
    riskLevel: 'medium',
    resetFn: (preview) => loadCircuitBreakerDefaults(preview),
    configType: 'Circuit Breaker',
  },
  {
    id: 'llm',
    name: 'LLM Configuration',
    description: 'Reset AI/LLM analysis configuration to default values',
    riskLevel: 'low',
    resetFn: (preview) => loadLLMConfigDefaults(preview),
    configType: 'LLM Config',
  },
  {
    id: 'capital-allocation',
    name: 'Capital Allocation',
    description: 'Reset capital allocation settings to default values',
    riskLevel: 'high',
    resetFn: (preview) => loadCapitalAllocationDefaults(preview),
    configType: 'Capital Allocation',
  },
];

export default function ResetSettings() {
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const [previewData, setPreviewData] = useState<ConfigResetPreview | null>(null);
  const [selectedGroup, setSelectedGroup] = useState<SettingGroup | null>(null);
  const [isLoadingPreview, setIsLoadingPreview] = useState(false);
  const [isResetting, setIsResetting] = useState(false);
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const [isResetAll, setIsResetAll] = useState(false);

  // Per-card preview data
  const [cardPreviews, setCardPreviews] = useState<Record<string, ConfigResetPreview>>({});
  const [cardLoadingStates, setCardLoadingStates] = useState<Record<string, boolean>>({});
  const [cardErrors, setCardErrors] = useState<Record<string, string>>({});

  // Auto-load preview data on mount
  useEffect(() => {
    const loadAllPreviews = async () => {
      for (const group of settingGroups) {
        setCardLoadingStates(prev => ({ ...prev, [group.id]: true }));
        try {
          const result = await group.resetFn(true);
          if ('preview' in result && result.preview) {
            setCardPreviews(prev => ({ ...prev, [group.id]: result }));
          }
        } catch (error: any) {
          setCardErrors(prev => ({
            ...prev,
            [group.id]: error.response?.data?.error || 'Failed to load preview'
          }));
        } finally {
          setCardLoadingStates(prev => ({ ...prev, [group.id]: false }));
        }
      }
    };

    loadAllPreviews();
  }, []);

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

  const getRiskIcon = (risk: 'high' | 'medium' | 'low') => {
    switch (risk) {
      case 'high':
        return <AlertTriangle className="w-4 h-4" />;
      case 'medium':
        return <Info className="w-4 h-4" />;
      case 'low':
        return <CheckCircle2 className="w-4 h-4" />;
    }
  };

  const handlePreviewChanges = async (group: SettingGroup) => {
    try {
      setIsLoadingPreview(true);
      setSelectedGroup(group);
      setIsResetAll(false);
      const result = await group.resetFn(true);
      if ('preview' in result && result.preview) {
        setPreviewData(result);
        setShowConfirmDialog(true);
      }
    } catch (error: any) {
      setMessage({
        type: 'error',
        text: error.response?.data?.error || 'Failed to load preview',
      });
    } finally {
      setIsLoadingPreview(false);
    }
  };

  const handleResetGroup = async () => {
    if (!selectedGroup) return;

    try {
      setIsResetting(true);
      const result = await selectedGroup.resetFn(false);
      if ('success' in result && result.success) {
        // Dispatch custom event for settings reset
        window.dispatchEvent(
          new CustomEvent('settings-reset', {
            detail: {
              configType: selectedGroup.configType,
              changesApplied: result.changes_applied,
              timestamp: new Date(),
            },
          })
        );

        setMessage({
          type: 'success',
          text: `${selectedGroup.name} has been reset to default values. Changes take effect within 1-2 minutes.`,
        });
        setShowConfirmDialog(false);

        // Reload page after 3 seconds to show fresh data
        setTimeout(() => window.location.reload(), 3000);
      }
    } catch (error: any) {
      setMessage({
        type: 'error',
        text: error.response?.data?.error || 'Failed to reset settings',
      });
    } finally {
      setIsResetting(false);
    }
  };

  const handlePreviewResetAll = async () => {
    try {
      setIsLoadingPreview(true);
      setSelectedGroup(null);
      setIsResetAll(true);
      const result = await loadAllModesDefaults(true);
      if ('preview' in result && result.preview) {
        setPreviewData(result);
        setShowConfirmDialog(true);
      }
    } catch (error: any) {
      setMessage({
        type: 'error',
        text: error.response?.data?.error || 'Failed to load preview',
      });
    } finally {
      setIsLoadingPreview(false);
    }
  };

  const handleResetAll = async () => {
    try {
      setIsResetting(true);
      const result = await loadAllModesDefaults(false);
      if ('success' in result && result.success) {
        // Dispatch custom event for settings reset
        window.dispatchEvent(
          new CustomEvent('settings-reset', {
            detail: {
              configType: 'all_settings',
              changesApplied: result.changes_applied,
              timestamp: new Date(),
            },
          })
        );

        setMessage({
          type: 'success',
          text: 'All settings have been reset to default values. Changes take effect within 1-2 minutes.',
        });
        setShowConfirmDialog(false);

        // Reload page after 3 seconds to show fresh data
        setTimeout(() => window.location.reload(), 3000);
      }
    } catch (error: any) {
      setMessage({
        type: 'error',
        text: error.response?.data?.error || 'Failed to reset all settings',
      });
    } finally {
      setIsResetting(false);
    }
  };

  const handleConfirmReset = async () => {
    if (isResetAll) {
      await handleResetAll();
    } else {
      await handleResetGroup();
    }
  };

  const formatValue = (value: any): string => {
    if (typeof value === 'boolean') return value ? 'true' : 'false';
    if (typeof value === 'number') return value.toString();
    if (typeof value === 'string') return value;
    if (value === null || value === undefined) return 'null';
    return JSON.stringify(value);
  };

  const retryLoadPreview = async (group: SettingGroup) => {
    setCardErrors(prev => {
      const next = { ...prev };
      delete next[group.id];
      return next;
    });
    setCardLoadingStates(prev => ({ ...prev, [group.id]: true }));
    try {
      const result = await group.resetFn(true);
      if ('preview' in result && result.preview) {
        setCardPreviews(prev => ({ ...prev, [group.id]: result }));
      }
    } catch (error: any) {
      setCardErrors(prev => ({
        ...prev,
        [group.id]: error.response?.data?.error || 'Failed to load preview'
      }));
    } finally {
      setCardLoadingStates(prev => ({ ...prev, [group.id]: false }));
    }
  };

  const renderCardPreview = (group: SettingGroup) => {
    const preview = cardPreviews[group.id];
    const isLoading = cardLoadingStates[group.id];
    const error = cardErrors[group.id];

    if (isLoading) {
      return (
        <div className="mb-4 space-y-2">
          <div className="flex items-center gap-2 text-gray-400">
            <Loader2 className="w-4 h-4 animate-spin" />
            <span className="text-sm">Loading preview...</span>
          </div>
        </div>
      );
    }

    if (error) {
      return (
        <div className="mb-4 p-3 bg-red-500/10 border border-red-500/30 rounded">
          <div className="flex items-start gap-2 mb-2">
            <AlertTriangle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
            <p className="text-sm text-red-400">{error}</p>
          </div>
          <button
            onClick={() => retryLoadPreview(group)}
            className="text-xs text-blue-400 hover:text-blue-300 underline"
          >
            Retry
          </button>
        </div>
      );
    }

    if (!preview) {
      return null;
    }

    const differences = preview.differences || [];
    const allValues = preview.all_values || [];
    const topDiffs = differences.slice(0, 5);
    const hasMore = differences.length > 5;

    return (
      <div className="mb-4 space-y-2">
        {/* Status Badge */}
        <div className="flex items-center gap-2 mb-2">
          {preview.all_match ? (
            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium bg-green-500/20 text-green-400 border border-green-500/30">
              <CheckCircle2 className="w-3 h-3" />
              All match defaults
            </span>
          ) : (
            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium bg-yellow-500/20 text-yellow-400 border border-yellow-500/30">
              <AlertTriangle className="w-3 h-3" />
              {preview.total_changes} change{preview.total_changes !== 1 ? 's' : ''} from default
            </span>
          )}
        </div>

        {/* Top Settings Preview */}
        {topDiffs.length > 0 && (
          <div className="space-y-1.5 text-xs">
            {topDiffs.map((diff, idx) => (
              <div key={idx} className="flex items-start gap-2 text-gray-300">
                <span className="text-gray-500 flex-shrink-0">•</span>
                <div className="flex-1 min-w-0">
                  <span className="font-medium text-white">{diff.path}:</span>{' '}
                  <span className="text-blue-400">{formatValue(diff.current)}</span>
                  {diff.current !== diff.default && (
                    <>
                      {' '}
                      <span className="text-gray-500">(default:</span>{' '}
                      <span className="text-gray-400">{formatValue(diff.default)}</span>
                      <span className="text-gray-500">)</span>
                    </>
                  )}
                </div>
              </div>
            ))}
            {hasMore && (
              <p className="text-gray-500 italic">
                +{differences.length - 5} more setting{differences.length - 5 !== 1 ? 's' : ''}
              </p>
            )}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="min-h-screen bg-dark-900 p-6">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-3 mb-2">
            <RefreshCw className="w-8 h-8 text-blue-500" />
            <h1 className="text-3xl font-bold text-white">Reset to Default Settings</h1>
          </div>
          <p className="text-gray-400 text-lg">
            Reset individual setting groups or all settings to their default values
          </p>
        </div>

        {/* Message Alert */}
        {message && (
          <div
            className={`mb-6 p-4 rounded-lg flex items-start gap-3 ${
              message.type === 'success'
                ? 'bg-green-500/10 border border-green-500/30'
                : 'bg-red-500/10 border border-red-500/30'
            }`}
          >
            {message.type === 'success' ? (
              <CheckCircle2 className="w-5 h-5 text-green-400 flex-shrink-0 mt-0.5" />
            ) : (
              <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
            )}
            <div className="flex-1">
              <p
                className={`text-sm ${
                  message.type === 'success' ? 'text-green-400' : 'text-red-400'
                }`}
              >
                {message.text}
              </p>
            </div>
            <button
              onClick={() => setMessage(null)}
              className="text-gray-400 hover:text-white transition-colors"
            >
              ×
            </button>
          </div>
        )}

        {/* Reset All Button */}
        <div className="mb-6">
          <button
            onClick={handlePreviewResetAll}
            disabled={isLoadingPreview}
            className="w-full py-4 px-6 bg-yellow-600 hover:bg-yellow-700 text-white font-semibold rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-3"
          >
            {isLoadingPreview && isResetAll ? (
              <>
                <Loader2 className="w-5 h-5 animate-spin" />
                Loading Preview...
              </>
            ) : (
              <>
                <RefreshCw className="w-5 h-5" />
                Reset All Settings to Defaults
              </>
            )}
          </button>
          <p className="text-xs text-gray-400 mt-2 text-center">
            This will reset all mode configurations, circuit breaker, hedge mode, and LLM settings
          </p>
        </div>

        {/* Warning Banner */}
        <div className="mb-6 bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" />
            <div>
              <p className="text-yellow-400 font-medium mb-1">Important Notice</p>
              <p className="text-sm text-gray-300">
                Resetting settings will replace your current configuration with default values. Always
                preview changes before applying. High-risk changes may affect active trading strategies.
              </p>
            </div>
          </div>
        </div>

        {/* Setting Groups Grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {settingGroups.map((group) => (
            <div
              key={group.id}
              className="bg-gray-800 rounded-lg border border-gray-700 p-5 hover:border-gray-600 transition-colors"
            >
              {/* Group Header */}
              <div className="flex items-start justify-between mb-3">
                <div className="flex-1">
                  <h3 className="text-lg font-semibold text-white mb-1">{group.name}</h3>
                  <p className="text-sm text-gray-400 mb-3">{group.description}</p>
                </div>
              </div>

              {/* Risk Level Badge */}
              <div className="flex items-center gap-2 mb-4">
                <span
                  className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium border ${getRiskColor(
                    group.riskLevel
                  )}`}
                >
                  {getRiskIcon(group.riskLevel)}
                  {group.riskLevel.toUpperCase()} RISK
                </span>
              </div>

              {/* Card Preview */}
              {renderCardPreview(group)}

              {/* Action Buttons */}
              <div className="flex gap-2">
                <button
                  onClick={() => handlePreviewChanges(group)}
                  disabled={isLoadingPreview || cardLoadingStates[group.id]}
                  className="flex-1 py-2 px-3 bg-blue-600 hover:bg-blue-700 text-white text-sm font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {isLoadingPreview && selectedGroup?.id === group.id && !isResetAll ? (
                    <>
                      <Loader2 className="w-4 h-4 animate-spin" />
                      Loading...
                    </>
                  ) : (
                    <>
                      <Info className="w-4 h-4" />
                      Preview All
                    </>
                  )}
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* Reset Confirm Dialog */}
        <ResetConfirmDialog
          open={showConfirmDialog}
          onClose={() => {
            setShowConfirmDialog(false);
            setPreviewData(null);
            setSelectedGroup(null);
            setIsResetAll(false);
          }}
          onConfirm={handleConfirmReset}
          title={isResetAll ? 'Reset All Settings' : `Reset ${selectedGroup?.name}`}
          configType={isResetAll ? 'All Settings' : previewData?.config_type || ''}
          loading={isResetting}
          allMatch={previewData?.all_match || false}
          differences={previewData?.differences || []}
          totalChanges={previewData?.total_changes || 0}
        />
      </div>
    </div>
  );
}
