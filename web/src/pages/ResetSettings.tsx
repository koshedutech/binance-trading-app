import React, { useState, useEffect } from 'react';
import { RefreshCw, AlertTriangle, Info, CheckCircle2, Loader2, Eye } from 'lucide-react';
import {
  loadModeDefaults,
  loadCircuitBreakerDefaults,
  loadLLMConfigDefaults,
  loadCapitalAllocationDefaults,
  loadHedgeDefaults,
  loadScalpReentryDefaults,
  loadSafetySettingsDefaults,
  loadAllModesDefaults,
  getAllDefaultSettings,
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
    id: 'scalp-reentry-config',
    name: 'Scalp Re-entry Optimization',
    description: 'Reset Scalp Re-entry optimization config (not a mode - enhances scalp positions)',
    riskLevel: 'medium',
    resetFn: (preview) => loadScalpReentryDefaults(preview),
    configType: 'Scalp Re-entry Config',
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
  {
    id: 'safety-settings',
    name: 'Safety Settings',
    description: 'Reset per-mode safety controls (rate limits, profit monitoring, win-rate monitoring)',
    riskLevel: 'medium',
    resetFn: (preview) => loadSafetySettingsDefaults(preview),
    configType: 'Safety Settings',
  },
];

// View-only settings groups (no database backing - display defaults only)
// These are settings from default-settings.json that exist and can be displayed
interface ViewOnlyGroup {
  id: string;
  name: string;
  description: string;
  jsonKey: string; // Key in default-settings.json
}

const viewOnlyGroups: ViewOnlyGroup[] = [
  {
    id: 'global-trading',
    name: 'Global Trading',
    description: 'Global trading settings like risk level and max allocation',
    jsonKey: 'global_trading',
  },
  {
    id: 'position-optimization',
    name: 'Position Optimization',
    description: 'Averaging and hedging optimization settings',
    jsonKey: 'position_optimization',
  },
  {
    id: 'early-warning',
    name: 'Early Warning',
    description: 'Early warning monitoring settings for loss detection',
    jsonKey: 'early_warning',
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

  // View-only defaults from default-settings.json
  const [allDefaults, setAllDefaults] = useState<Record<string, unknown> | null>(null);
  const [defaultsLoading, setDefaultsLoading] = useState(true);
  const [defaultsError, setDefaultsError] = useState<string | null>(null);

  // View-only dialog state
  const [selectedViewOnlyGroup, setSelectedViewOnlyGroup] = useState<ViewOnlyGroup | null>(null);
  const [showViewOnlyDialog, setShowViewOnlyDialog] = useState(false);

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

  // Load all default settings for view-only display
  useEffect(() => {
    const loadDefaults = async () => {
      try {
        setDefaultsLoading(true);
        const result = await getAllDefaultSettings();
        if (result.success && result.defaults) {
          setAllDefaults(result.defaults);
        }
      } catch (error: any) {
        setDefaultsError(error.response?.data?.error || 'Failed to load default settings');
      } finally {
        setDefaultsLoading(false);
      }
    };

    loadDefaults();
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

  // Refresh a specific card's preview data after admin saves
  const refreshCardPreview = async (groupId: string) => {
    const group = settingGroups.find(g => g.id === groupId);
    if (!group) return;

    setCardLoadingStates(prev => ({ ...prev, [groupId]: true }));
    try {
      const result = await group.resetFn(true);
      if ('preview' in result && result.preview) {
        setCardPreviews(prev => ({ ...prev, [groupId]: result }));
      }
    } catch (error: any) {
      console.error('Failed to refresh card preview:', error);
    } finally {
      setCardLoadingStates(prev => ({ ...prev, [groupId]: false }));
    }
  };

  // Map configType back to group.id for refreshing
  const getGroupIdFromConfigType = (configType: string): string | null => {
    const mapping: Record<string, string> = {
      'ultra_fast': 'ultra-fast',
      'scalp': 'scalp',
      'scalp_reentry': 'scalp-reentry-config',
      'swing': 'swing',
      'position': 'position',
      'hedge_mode': 'hedge',
      'circuit_breaker': 'circuit-breaker',
      'llm_config': 'llm',
      'capital_allocation': 'capital-allocation',
      'safety_settings': 'safety-settings',
    };
    return mapping[configType] || null;
  };

  // Helper to flatten nested objects for display
  const flattenObject = (obj: any, prefix = ''): { key: string; value: any }[] => {
    const result: { key: string; value: any }[] = [];
    for (const key in obj) {
      const newKey = prefix ? `${prefix}.${key}` : key;
      const value = obj[key];
      if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
        result.push(...flattenObject(value, newKey));
      } else {
        result.push({ key: newKey, value });
      }
    }
    return result;
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

    // Admin preview - show all default values
    if (preview.is_admin && preview.default_value) {
      const flattened = flattenObject(preview.default_value);
      const topValues = flattened.slice(0, 8);
      const hasMore = flattened.length > 8;

      return (
        <div className="mb-4 space-y-2">
          {/* Admin Badge */}
          <div className="flex items-center gap-2 mb-2">
            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded text-xs font-medium bg-purple-500/20 text-purple-400 border border-purple-500/30">
              <Info className="w-3 h-3" />
              Admin View - Default Values
            </span>
          </div>

          {/* Default Values Preview */}
          <div className="space-y-1.5 text-xs max-h-48 overflow-y-auto">
            {topValues.map((item, idx) => (
              <div key={idx} className="flex items-start gap-2 text-gray-300">
                <span className="text-gray-500 flex-shrink-0">•</span>
                <div className="flex-1 min-w-0">
                  <span className="font-medium text-white">{item.key}:</span>{' '}
                  <span className="text-green-400">{formatValue(item.value)}</span>
                </div>
              </div>
            ))}
            {hasMore && (
              <p className="text-gray-500 italic">
                +{flattened.length - 8} more settings (click Preview All to see full list)
              </p>
            )}
          </div>
        </div>
      );
    }

    // Regular user preview - show differences
    const differences = preview.differences || [];
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

        {/* View-Only Settings Section */}
        <div className="mt-10">
          <div className="mb-6">
            <div className="flex items-center gap-3 mb-2">
              <Eye className="w-6 h-6 text-purple-500" />
              <h2 className="text-2xl font-bold text-white">Additional Settings (View Only)</h2>
            </div>
            <p className="text-gray-400">
              These settings are from default-settings.json. Contact admin to modify defaults.
            </p>
          </div>

          {defaultsLoading && (
            <div className="flex items-center gap-2 text-gray-400 p-4">
              <Loader2 className="w-5 h-5 animate-spin" />
              <span>Loading default settings...</span>
            </div>
          )}

          {defaultsError && (
            <div className="p-4 bg-red-500/10 border border-red-500/30 rounded-lg text-red-400">
              {defaultsError}
            </div>
          )}

          {allDefaults && !defaultsLoading && (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {viewOnlyGroups.map((group) => {
                const sectionData = allDefaults[group.jsonKey];
                return (
                  <div
                    key={group.id}
                    className="bg-gray-800/50 rounded-lg border border-purple-500/20 p-5 hover:border-purple-500/40 transition-colors"
                  >
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex-1">
                        <h3 className="text-lg font-semibold text-white mb-1">{group.name}</h3>
                        <p className="text-sm text-gray-400 mb-3">{group.description}</p>
                      </div>
                    </div>

                    {/* View Only Badge */}
                    <div className="flex items-center gap-2 mb-4">
                      <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium border bg-purple-500/20 text-purple-400 border-purple-500/30">
                        <Eye className="w-3 h-3" />
                        VIEW ONLY
                      </span>
                    </div>

                    {/* Settings Preview */}
                    <div className="space-y-1.5 text-xs max-h-48 overflow-y-auto mb-4">
                      {sectionData && typeof sectionData === 'object' ? (
                        Object.entries(sectionData as Record<string, unknown>)
                          .filter(([key]) => !key.startsWith('_')) // Skip _risk_info etc
                          .slice(0, 8)
                          .map(([key, value]) => (
                            <div key={key} className="flex items-start gap-2 text-gray-300">
                              <span className="text-purple-400 flex-shrink-0">•</span>
                              <div className="flex-1 min-w-0">
                                <span className="font-medium text-white">{key}:</span>{' '}
                                <span className="text-green-400">
                                  {typeof value === 'object' ? JSON.stringify(value).slice(0, 50) + '...' : String(value)}
                                </span>
                              </div>
                            </div>
                          ))
                      ) : (
                        <p className="text-gray-500 italic">No data available</p>
                      )}
                      {sectionData && typeof sectionData === 'object' &&
                        Object.keys(sectionData as Record<string, unknown>).filter(k => !k.startsWith('_')).length > 8 && (
                        <p className="text-gray-500 italic">
                          +{Object.keys(sectionData as Record<string, unknown>).filter(k => !k.startsWith('_')).length - 8} more fields
                        </p>
                      )}
                    </div>

                    {/* View All Button */}
                    {sectionData && typeof sectionData === 'object' && (
                      <button
                        onClick={() => {
                          setSelectedViewOnlyGroup(group);
                          setShowViewOnlyDialog(true);
                        }}
                        className="w-full py-2 px-3 bg-purple-600/50 hover:bg-purple-600 text-white text-sm font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
                      >
                        <Eye className="w-4 h-4" />
                        View All Settings
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
          )}
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
          isAdmin={previewData?.is_admin}
          defaultValue={previewData?.default_value}
          onSaveSuccess={async () => {
            // Refresh the specific card's preview data after admin saves
            const configType = previewData?.config_type || '';
            const groupId = getGroupIdFromConfigType(configType);
            if (groupId) {
              refreshCardPreview(groupId);
            } else if (isResetAll) {
              // Refresh all cards if "Reset All" was used
              settingGroups.forEach(group => refreshCardPreview(group.id));
            }
            // Also refresh the allDefaults for view-only cards
            try {
              const result = await getAllDefaultSettings();
              if (result.success && result.defaults) {
                setAllDefaults(result.defaults);
              }
            } catch (error) {
              console.error('Failed to refresh all defaults:', error);
            }
          }}
        />

        {/* View-Only Settings Dialog */}
        {showViewOnlyDialog && selectedViewOnlyGroup && allDefaults && (
          <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
            <div className="bg-gray-800 rounded-xl max-w-3xl w-full max-h-[90vh] shadow-2xl border border-gray-700 flex flex-col">
              {/* Header */}
              <div className="flex items-center justify-between p-4 border-b border-gray-700">
                <div className="flex items-center gap-3">
                  <h3 className="text-lg font-bold text-white">{selectedViewOnlyGroup.name}</h3>
                  <span className="px-2 py-1 text-xs bg-purple-500/20 text-purple-400 rounded border border-purple-500/30 flex items-center gap-1">
                    <Eye className="w-3 h-3" /> View Only
                  </span>
                </div>
                <button
                  onClick={() => {
                    setShowViewOnlyDialog(false);
                    setSelectedViewOnlyGroup(null);
                  }}
                  className="text-gray-400 hover:text-white transition-colors"
                >
                  <span className="text-2xl">&times;</span>
                </button>
              </div>

              {/* Body */}
              <div className="p-4 overflow-y-auto flex-1">
                <div className="bg-purple-500/10 border border-purple-500/30 rounded-lg p-4 mb-4">
                  <p className="text-purple-400 font-medium">Default Settings from JSON</p>
                  <p className="text-sm text-gray-400 mt-1">
                    {selectedViewOnlyGroup.description}. These values are from <code className="px-1 bg-gray-700 rounded text-xs">default-settings.json</code>.
                  </p>
                </div>

                {/* All Settings Table */}
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="border-b border-gray-700">
                        <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300">Setting</th>
                        <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300">Value</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(() => {
                        const sectionData = allDefaults[selectedViewOnlyGroup.jsonKey];
                        if (!sectionData || typeof sectionData !== 'object') {
                          return (
                            <tr>
                              <td colSpan={2} className="py-4 text-center text-gray-500">No data available</td>
                            </tr>
                          );
                        }

                        // Flatten nested objects for display
                        const flattenObject = (obj: any, prefix = ''): { key: string; value: any }[] => {
                          const result: { key: string; value: any }[] = [];
                          for (const key in obj) {
                            if (key.startsWith('_')) continue; // Skip internal keys
                            const newKey = prefix ? `${prefix}.${key}` : key;
                            const value = obj[key];
                            if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
                              result.push(...flattenObject(value, newKey));
                            } else {
                              result.push({ key: newKey, value });
                            }
                          }
                          return result;
                        };

                        const flattenedData = flattenObject(sectionData);

                        return flattenedData.map((item, index) => (
                          <tr key={index} className="border-b border-gray-700/50 hover:bg-gray-700/20">
                            <td className="py-2 px-3">
                              <span className="text-sm text-white font-medium">{item.key}</span>
                            </td>
                            <td className="py-2 px-3 text-sm font-mono">
                              <span className="text-green-400">
                                {typeof item.value === 'boolean'
                                  ? item.value ? 'Yes' : 'No'
                                  : typeof item.value === 'object'
                                    ? JSON.stringify(item.value)
                                    : String(item.value)}
                              </span>
                            </td>
                          </tr>
                        ));
                      })()}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Footer */}
              <div className="p-4 border-t border-gray-700">
                <button
                  onClick={() => {
                    setShowViewOnlyDialog(false);
                    setSelectedViewOnlyGroup(null);
                  }}
                  className="w-full py-2 px-4 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
