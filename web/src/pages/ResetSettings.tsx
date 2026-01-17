import { useState, useCallback } from 'react';
import { RefreshCw, AlertTriangle, CheckCircle2, Loader2 } from 'lucide-react';
import {
  loadModeDefaults,
  loadCircuitBreakerDefaults,
  loadLLMConfigDefaults,
  loadCapitalAllocationDefaults,
  loadGlobalTradingDefaults,
  loadSafetySettingsDefaults,
  loadAllModesDefaults,
  resetModeGroup,
  adminSaveModeConfig,
  adminSaveOtherSetting,
} from '../services/futuresApi';
import SettingsComparisonView from '../components/SettingsComparisonView';
import { useAuth } from '../contexts/AuthContext';

// Toast notification types
interface ToastMessage {
  type: 'success' | 'error' | 'info';
  text: string;
}

export default function ResetSettings() {
  const { user } = useAuth();
  const [loading, setLoading] = useState(false);
  const [toast, setToast] = useState<ToastMessage | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Determine if user is admin
  const isAdmin = user?.is_admin || user?.email === 'admin@binance-bot.local';

  // Show toast notification
  const showToast = useCallback((type: ToastMessage['type'], text: string) => {
    setToast({ type, text });
    // Auto-dismiss after 5 seconds
    setTimeout(() => setToast(null), 5000);
  }, []);

  // Refresh the view
  const handleRefresh = useCallback(() => {
    setRefreshKey(prev => prev + 1);
  }, []);

  // Dispatch settings reset event for other components to react
  const dispatchResetEvent = (configType: string, changesApplied: number) => {
    window.dispatchEvent(
      new CustomEvent('settings-reset', {
        detail: {
          configType,
          changesApplied,
          timestamp: new Date(),
        },
      })
    );
  };

  // Reset all modes to defaults
  const handleResetAllModes = useCallback(async () => {
    try {
      setLoading(true);
      const result = await loadAllModesDefaults(false);
      if ('success' in result && result.success) {
        dispatchResetEvent('all_modes', result.changes_applied || 0);
        showToast('success', 'All modes reset to defaults successfully. Changes take effect within 1-2 minutes.');
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset all modes');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset a specific mode to defaults
  const handleResetMode = useCallback(async (mode: string) => {
    try {
      setLoading(true);
      const result = await loadModeDefaults(mode, false);
      if ('success' in result && result.success) {
        const modeName = mode.replace('_', ' ').replace(/\b\w/g, c => c.toUpperCase());
        dispatchResetEvent(mode, result.changes_applied || 0);
        showToast('success', `${modeName} mode reset to defaults. Changes take effect within 1-2 minutes.`);
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || `Failed to reset ${mode} mode`);
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset a specific group within a mode to defaults
  const handleResetModeGroup = useCallback(async (mode: string, group: string) => {
    try {
      setLoading(true);
      const result = await resetModeGroup(mode, group);
      if (result.success) {
        const modeName = mode.replace('_', ' ').replace(/\b\w/g, c => c.toUpperCase());
        const groupName = group.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
        dispatchResetEvent(`${mode}.${group}`, result.changes_applied || 0);
        showToast('success', `${modeName} → ${groupName} reset to defaults.`);
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || `Failed to reset ${group} group`);
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset all "other" settings (circuit breaker, LLM, capital allocation, global trading, safety)
  const handleResetAllOther = useCallback(async () => {
    try {
      setLoading(true);
      let totalChanges = 0;

      // Reset circuit breaker
      const cbResult = await loadCircuitBreakerDefaults(false);
      if ('success' in cbResult && cbResult.success) {
        totalChanges += cbResult.changes_applied || 0;
      }

      // Reset LLM config
      const llmResult = await loadLLMConfigDefaults(false);
      if ('success' in llmResult && llmResult.success) {
        totalChanges += llmResult.changes_applied || 0;
      }

      // Reset capital allocation
      const capResult = await loadCapitalAllocationDefaults(false);
      if ('success' in capResult && capResult.success) {
        totalChanges += capResult.changes_applied || 0;
      }

      // Reset global trading
      const gtResult = await loadGlobalTradingDefaults(false);
      if ('success' in gtResult && gtResult.success) {
        totalChanges += gtResult.changes_applied || 0;
      }

      // Reset safety settings
      const safetyResult = await loadSafetySettingsDefaults(false);
      if ('success' in safetyResult && safetyResult.success) {
        totalChanges += safetyResult.changes_applied || 0;
      }

      dispatchResetEvent('all_other', totalChanges);
      showToast('success', 'All other settings reset to defaults. Changes take effect within 1-2 minutes.');
      handleRefresh();
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset other settings');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset circuit breaker settings
  const handleResetCircuitBreaker = useCallback(async () => {
    try {
      setLoading(true);
      const result = await loadCircuitBreakerDefaults(false);
      if ('success' in result && result.success) {
        dispatchResetEvent('circuit_breaker', result.changes_applied || 0);
        showToast('success', 'Circuit breaker settings reset to defaults.');
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset circuit breaker settings');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset LLM config settings
  const handleResetLLMConfig = useCallback(async () => {
    try {
      setLoading(true);
      const result = await loadLLMConfigDefaults(false);
      if ('success' in result && result.success) {
        dispatchResetEvent('llm_config', result.changes_applied || 0);
        showToast('success', 'LLM configuration reset to defaults.');
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset LLM config');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset capital allocation settings
  const handleResetCapitalAllocation = useCallback(async () => {
    try {
      setLoading(true);
      const result = await loadCapitalAllocationDefaults(false);
      if ('success' in result && result.success) {
        dispatchResetEvent('capital_allocation', result.changes_applied || 0);
        showToast('success', 'Capital allocation settings reset to defaults.');
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset capital allocation');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset global trading settings
  const handleResetGlobalTrading = useCallback(async () => {
    try {
      setLoading(true);
      const result = await loadGlobalTradingDefaults(false);
      if ('success' in result && result.success) {
        dispatchResetEvent('global_trading', result.changes_applied || 0);
        showToast('success', 'Global trading settings reset to defaults.');
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset global trading settings');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Reset safety settings
  const handleResetSafetySettings = useCallback(async () => {
    try {
      setLoading(true);
      const result = await loadSafetySettingsDefaults(false);
      if ('success' in result && result.success) {
        dispatchResetEvent('safety_settings', result.changes_applied || 0);
        showToast('success', 'Safety settings reset to defaults.');
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || 'Failed to reset safety settings');
    } finally {
      setLoading(false);
    }
  }, [showToast, handleRefresh]);

  // Admin: Save mode config changes
  const handleSaveMode = useCallback(async (mode: string, editedValues: Record<string, any>) => {
    if (!isAdmin) return;
    try {
      setLoading(true);
      const result = await adminSaveModeConfig(mode, editedValues);
      if (result.success) {
        const modeName = mode.replace('_', ' ').replace(/\b\w/g, c => c.toUpperCase());
        showToast('success', `${modeName} mode defaults saved successfully.`);
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || `Failed to save ${mode} mode defaults`);
    } finally {
      setLoading(false);
    }
  }, [isAdmin, showToast, handleRefresh]);

  // Admin: Save other setting changes
  const handleSaveOtherSetting = useCallback(async (settingType: string, editedValues: Record<string, any>) => {
    if (!isAdmin) return;
    try {
      setLoading(true);
      const result = await adminSaveOtherSetting(settingType, editedValues);
      if (result.success) {
        const settingName = settingType.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
        showToast('success', `${settingName} defaults saved successfully.`);
        handleRefresh();
      }
    } catch (error: any) {
      showToast('error', error.response?.data?.error || `Failed to save ${settingType} defaults`);
    } finally {
      setLoading(false);
    }
  }, [isAdmin, showToast, handleRefresh]);

  return (
    <div className="min-h-screen bg-dark-900 p-6">
      <div className="max-w-7xl mx-auto">
        {/* Page Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold text-white">Reset & Compare Settings</h1>
            <p className="text-gray-400 mt-1">
              Compare your current settings with defaults and reset as needed
            </p>
          </div>
          <button
            onClick={handleRefresh}
            disabled={loading}
            className="flex items-center gap-2 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>

        {/* Toast Notification */}
        {toast && (
          <div
            className={`mb-6 p-4 rounded-lg flex items-start gap-3 ${
              toast.type === 'success'
                ? 'bg-green-500/10 border border-green-500/30'
                : toast.type === 'error'
                ? 'bg-red-500/10 border border-red-500/30'
                : 'bg-blue-500/10 border border-blue-500/30'
            }`}
          >
            {toast.type === 'success' ? (
              <CheckCircle2 className="w-5 h-5 text-green-400 flex-shrink-0 mt-0.5" />
            ) : toast.type === 'error' ? (
              <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
            ) : (
              <RefreshCw className="w-5 h-5 text-blue-400 flex-shrink-0 mt-0.5" />
            )}
            <div className="flex-1">
              <p
                className={`text-sm ${
                  toast.type === 'success'
                    ? 'text-green-400'
                    : toast.type === 'error'
                    ? 'text-red-400'
                    : 'text-blue-400'
                }`}
              >
                {toast.text}
              </p>
            </div>
            <button
              onClick={() => setToast(null)}
              className="text-gray-400 hover:text-white transition-colors"
            >
              x
            </button>
          </div>
        )}

        {/* Loading Overlay */}
        {loading && (
          <div className="mb-6 p-4 bg-blue-500/10 border border-blue-500/30 rounded-lg flex items-center gap-3">
            <Loader2 className="w-5 h-5 text-blue-400 animate-spin" />
            <span className="text-blue-400">Processing reset request...</span>
          </div>
        )}

        {/* Warning Banner */}
        <div className="mb-6 bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" />
            <div>
              <p className="text-yellow-400 font-medium mb-1">Important Notice</p>
              <p className="text-sm text-gray-300">
                Resetting settings will replace your current configuration with default values.
                Changes typically take effect within 1-2 minutes after reset.
              </p>
            </div>
          </div>
        </div>

        {/* Unified Settings Comparison View */}
        <SettingsComparisonView
          key={refreshKey}
          modes={['ultra_fast', 'scalp', 'swing', 'position']}
          isAdmin={isAdmin}
          // Mode resets
          onResetAllModes={handleResetAllModes}
          onResetMode={handleResetMode}
          onResetModeGroup={handleResetModeGroup}
          // Other settings resets
          onResetAllOther={handleResetAllOther}
          onResetCircuitBreaker={handleResetCircuitBreaker}
          onResetLLMConfig={handleResetLLMConfig}
          onResetCapitalAllocation={handleResetCapitalAllocation}
          onResetGlobalTrading={handleResetGlobalTrading}
          // Admin save handlers
          onSaveMode={isAdmin ? handleSaveMode : undefined}
          onSaveOtherSetting={isAdmin ? handleSaveOtherSetting : undefined}
        />
      </div>
    </div>
  );
}
