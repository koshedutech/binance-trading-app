import { useState, useEffect, useMemo, useRef } from 'react';
import { AlertTriangle, X, Loader2, CheckCircle2, Eye, EyeOff, Save, RotateCcw, Edit3 } from 'lucide-react';
import { saveAdminDefaults } from '../services/futuresApi';

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
  // Admin-specific props
  isAdmin?: boolean;
  defaultValue?: any;
  onSaveDefaults?: (values: Record<string, any>) => Promise<void>;
  // Callback after successful save - parent can use this to refresh data
  onSaveSuccess?: () => void;
}

// Settings that are displayed in the main UI panels
const UI_VISIBLE_SETTINGS: Record<string, string[]> = {
  mode_config: [
    'enabled', 'confidence.min_confidence', 'confidence.high_confidence', 'confidence.ultra_confidence',
    'size.base_size_usd', 'size.max_size_usd', 'size.leverage', 'size.max_positions', 'size.auto_size_enabled',
    'sltp.stop_loss_percent', 'sltp.take_profit_percent', 'sltp.trailing_stop_enabled',
    'sltp.trailing_stop_percent', 'sltp.trailing_stop_activation', 'sltp.use_roi_based_sltp',
    'sltp.auto_sltp_enabled', 'sltp.auto_trailing_enabled',
    'hedge.allow_hedge', 'hedge.min_confidence_for_hedge',
    'trend_divergence.enabled', 'trend_divergence.block_on_divergence',
    'funding_rate.enabled', 'funding_rate.max_funding_rate',
  ],
  circuit_breaker: [
    'enabled', 'max_loss_per_hour', 'max_daily_loss', 'max_consecutive_losses',
    'cooldown_minutes', 'max_trades_per_minute', 'max_daily_trades',
  ],
  llm_config: ['enabled', 'provider', 'model', 'timeout_ms'],
  capital_allocation: ['ultra_fast_percent', 'scalp_percent', 'swing_percent', 'position_percent', 'allow_dynamic_rebalance', 'rebalance_threshold_pct'],
  scalp_reentry: [
    'enabled', 'tp1_percent', 'tp1_sell_percent', 'tp2_percent', 'tp2_sell_percent',
    'tp3_percent', 'tp3_sell_percent', 'reentry_enabled', 'reentry_dip_percent', 'reentry_max_entries',
  ],
  // Story 9.4: Safety settings per mode
  safety_settings: [
    // Per-mode prefixes - all settings under these modes are UI visible
    'safety_settings.ultra_fast', 'safety_settings.scalp', 'safety_settings.swing', 'safety_settings.position',
    // Individual settings (in case not using prefix matching)
    'max_trades_per_minute', 'max_trades_per_hour', 'max_trades_per_day',
    'enable_profit_monitor', 'profit_window_minutes', 'max_loss_percent_in_window', 'pause_cooldown_minutes',
    'enable_win_rate_monitor', 'win_rate_sample_size', 'min_win_rate_threshold', 'win_rate_cooldown_minutes',
  ],
};

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
  isAdmin,
  defaultValue,
  onSaveDefaults,
  onSaveSuccess,
}: ResetConfirmDialogProps) {
  // State for admin editing default values
  const [editedDefaults, setEditedDefaults] = useState<Record<string, any>>({});
  const [showHiddenSettings, setShowHiddenSettings] = useState(false);
  const [saving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  // Flatten nested object to path-value pairs
  const flattenObject = (obj: any, prefix = ''): { key: string; value: any }[] => {
    const result: { key: string; value: any }[] = [];
    if (!obj) return result;
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

  // Get flattened defaults for admin editing
  const flattenedDefaults = useMemo(() => {
    if (isAdmin && defaultValue) {
      return flattenObject(defaultValue);
    }
    return [];
  }, [isAdmin, defaultValue]);

  // Categorize for user view
  const uiVisiblePaths = UI_VISIBLE_SETTINGS[configType] || [];
  const { uiVisibleSettings, hiddenSettings } = useMemo(() => {
    if (isAdmin) return { uiVisibleSettings: [], hiddenSettings: [] };

    const visible: SettingDiff[] = [];
    const hidden: SettingDiff[] = [];
    differences.forEach((diff) => {
      const isVisible = uiVisiblePaths.some((pattern) =>
        diff.path === pattern || diff.path.startsWith(pattern + '.')
      );
      if (isVisible) visible.push(diff);
      else hidden.push(diff);
    });
    return { uiVisibleSettings: visible, hiddenSettings: hidden };
  }, [isAdmin, differences, uiVisiblePaths]);

  // Categorize admin defaults into UI visible and hidden
  // Sort visible items by the order they appear in UI_VISIBLE_SETTINGS
  const { adminVisibleDefaults, adminHiddenDefaults } = useMemo(() => {
    if (!isAdmin) return { adminVisibleDefaults: [], adminHiddenDefaults: [] };

    const visible: { key: string; value: any }[] = [];
    const hidden: { key: string; value: any }[] = [];
    flattenedDefaults.forEach((item) => {
      const isVisible = uiVisiblePaths.some((pattern) =>
        item.key === pattern || item.key.startsWith(pattern + '.')
      );
      if (isVisible) visible.push(item);
      else hidden.push(item);
    });

    // Sort visible items by the order defined in UI_VISIBLE_SETTINGS
    visible.sort((a, b) => {
      const indexA = uiVisiblePaths.findIndex(p => a.key === p || a.key.startsWith(p + '.'));
      const indexB = uiVisiblePaths.findIndex(p => b.key === p || b.key.startsWith(p + '.'));
      return indexA - indexB;
    });

    return { adminVisibleDefaults: visible, adminHiddenDefaults: hidden };
  }, [isAdmin, flattenedDefaults, uiVisiblePaths]);

  // Initialize admin edited values from defaults
  useEffect(() => {
    if (open && isAdmin && flattenedDefaults.length > 0) {
      const initial: Record<string, any> = {};
      flattenedDefaults.forEach((item) => {
        initial[item.key] = item.value;
      });
      setEditedDefaults(initial);
      setHasChanges(false);
    }
  }, [open, isAdmin, flattenedDefaults]);

  // Early return AFTER all hooks
  if (!open) return null;

  // Format value for display
  const formatValue = (value: any): string => {
    if (value === null || value === undefined) return 'N/A';
    if (typeof value === 'boolean') return value ? 'Yes' : 'No';
    if (Array.isArray(value)) return value.join(', ');
    if (typeof value === 'object') return JSON.stringify(value);
    if (typeof value === 'number') return value.toLocaleString();
    return String(value);
  };

  // Parse value based on type
  const parseValue = (value: string, originalValue: any): any => {
    if (typeof originalValue === 'boolean') {
      return value.toLowerCase() === 'true' || value.toLowerCase() === 'yes';
    }
    if (typeof originalValue === 'number') {
      const parsed = parseFloat(value);
      return isNaN(parsed) ? originalValue : parsed;
    }
    if (Array.isArray(originalValue)) {
      return value.split(',').map((s) => s.trim());
    }
    return value;
  };

  // Validate capital allocation totals to 100%
  const validateCapitalAllocation = (): string | null => {
    if (configType !== 'capital_allocation') return null;

    const ultraFast = Number(editedDefaults['ultra_fast_percent']) || 0;
    const scalp = Number(editedDefaults['scalp_percent']) || 0;
    const swing = Number(editedDefaults['swing_percent']) || 0;
    const position = Number(editedDefaults['position_percent']) || 0;
    const total = ultraFast + scalp + swing + position;

    if (total < 99 || total > 101) {
      return `Total allocation must be 100%. Current total: ${total.toFixed(1)}%`;
    }
    return null;
  };

  // Handle admin editing a default value
  const handleDefaultValueChange = (path: string, value: string, originalValue: any) => {
    const parsed = parseValue(value, originalValue);
    setEditedDefaults((prev) => ({ ...prev, [path]: parsed }));
    setHasChanges(true);
    // Clear validation error when user makes changes
    setValidationError(null);
  };

  // Save admin changes to default-settings.json
  // Uses saveAdminDefaults directly with configType - no longer relies on parent callback
  const handleSaveDefaults = async () => {
    console.log('[ADMIN-SAVE] handleSaveDefaults called');
    console.log('[ADMIN-SAVE] configType:', configType);
    console.log('[ADMIN-SAVE] editedDefaults:', editedDefaults);
    console.log('[ADMIN-SAVE] hasChanges:', hasChanges);

    // Validate capital allocation before saving
    const error = validateCapitalAllocation();
    if (error) {
      setValidationError(error);
      return;
    }

    setSaving(true);
    try {
      console.log('[ADMIN-SAVE] Calling saveAdminDefaults directly...');
      console.log('[ADMIN-SAVE] Request payload:', JSON.stringify({ configType, editedDefaults }, null, 2));

      const response = await saveAdminDefaults(configType, editedDefaults);
      console.log('[ADMIN-SAVE] Save response:', response);

      if (!response.success) {
        throw new Error(response.message || 'Save failed');
      }

      console.log('[ADMIN-SAVE] Save successful, changes_count:', response.changes_count);
      setHasChanges(false);
      setValidationError(null);

      // Call the success callback to trigger parent refresh - AWAIT it to ensure refresh completes
      if (onSaveSuccess) {
        console.log('[ADMIN-SAVE] Calling onSaveSuccess callback...');
        await onSaveSuccess();
        console.log('[ADMIN-SAVE] onSaveSuccess callback completed');
      }

      // Show visible success alert so user knows save worked
      alert(`✅ SUCCESS: Saved ${response.changes_count} changes to default-settings.json for ${response.config_type}`);

      // Close the dialog
      console.log('[ADMIN-SAVE] Closing dialog');
      onClose();
    } catch (error: any) {
      console.error('[ADMIN-SAVE] Failed to save defaults:', error);
      const errorMsg = error?.response?.data?.message || error?.response?.data?.error || error?.message || 'Unknown error';
      alert(`Failed to save: ${errorMsg}`);
    } finally {
      setSaving(false);
    }
  };

  // Get chip color based on risk level
  const getRiskColor = (risk: 'high' | 'medium' | 'low') => {
    switch (risk) {
      case 'high': return 'bg-red-500/20 text-red-400 border-red-500/30';
      case 'medium': return 'bg-orange-500/20 text-orange-400 border-orange-500/30';
      case 'low': return 'bg-green-500/20 text-green-400 border-green-500/30';
      default: return 'bg-gray-500/20 text-gray-400 border-gray-500/30';
    }
  };

  // Render admin editable input
  const renderAdminInput = (path: string, value: any) => {
    const currentValue = editedDefaults[path] ?? value;
    const isModified = JSON.stringify(currentValue) !== JSON.stringify(value);
    const inputType = typeof value === 'boolean' ? 'checkbox' : typeof value === 'number' ? 'number' : 'text';

    if (inputType === 'checkbox') {
      return (
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={currentValue === true}
            onChange={(e) => handleDefaultValueChange(path, e.target.checked ? 'true' : 'false', value)}
            className="w-4 h-4 rounded bg-gray-700 border-gray-600"
          />
          <span className={`text-xs ${currentValue ? 'text-green-400' : 'text-gray-400'}`}>
            {currentValue ? 'Enabled' : 'Disabled'}
          </span>
          {isModified && <span className="text-xs text-orange-400">(modified)</span>}
        </div>
      );
    }

    // Auto-select content on focus for easier value replacement
    const handleFocus = (e: React.FocusEvent<HTMLInputElement>) => {
      e.target.select();
    };

    return (
      <input
        type={inputType}
        value={typeof currentValue === 'object' ? JSON.stringify(currentValue) : currentValue}
        onChange={(e) => handleDefaultValueChange(path, e.target.value, value)}
        onFocus={handleFocus}
        className={`w-full px-2 py-1 text-sm bg-gray-700 border rounded focus:ring-1 focus:ring-blue-500 ${
          isModified ? 'border-orange-500' : 'border-gray-600'
        }`}
        step={inputType === 'number' ? 'any' : undefined}
      />
    );
  };

  // Render admin defaults table
  const renderAdminDefaultsTable = (items: { key: string; value: any }[], sectionTitle: string, sectionColor: string) => {
    if (items.length === 0) return null;

    const modifiedCount = items.filter(item =>
      JSON.stringify(editedDefaults[item.key]) !== JSON.stringify(item.value)
    ).length;

    return (
      <div className="mb-4">
        <div className="flex items-center gap-2 mb-2">
          <div className={`h-px flex-1 ${sectionColor}`}></div>
          <span className={`text-xs font-semibold uppercase tracking-wider ${sectionColor.replace('bg-', 'text-').replace('/30', '')}`}>
            {sectionTitle} ({items.length} settings{modifiedCount > 0 ? `, ${modifiedCount} modified` : ''})
          </span>
          <div className={`h-px flex-1 ${sectionColor}`}></div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300 w-1/2">Setting</th>
                <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300 w-1/2">Default Value (Editable)</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item, index) => {
                const isModified = JSON.stringify(editedDefaults[item.key]) !== JSON.stringify(item.value);
                return (
                  <tr key={index} className={`border-b border-gray-700/50 hover:bg-gray-700/20 ${isModified ? 'bg-orange-500/5' : ''}`}>
                    <td className="py-2 px-3">
                      <span className="text-sm text-white font-medium">{item.key}</span>
                    </td>
                    <td className="py-2 px-3">
                      {renderAdminInput(item.key, item.value)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  // Render user differences table
  const renderUserDiffSection = (settings: SettingDiff[], sectionTitle: string, sectionColor: string) => {
    if (settings.length === 0) return null;

    const changedCount = settings.filter(s => JSON.stringify(s.current) !== JSON.stringify(s.default)).length;

    return (
      <div className="mb-4">
        <div className="flex items-center gap-2 mb-2">
          <div className={`h-px flex-1 ${sectionColor}`}></div>
          <span className={`text-xs font-semibold uppercase tracking-wider ${sectionColor.replace('bg-', 'text-').replace('/30', '')}`}>
            {sectionTitle} ({settings.length} settings, {changedCount} differ)
          </span>
          <div className={`h-px flex-1 ${sectionColor}`}></div>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-gray-700">
                <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300">Setting</th>
                <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300">Your Value</th>
                <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300">Default</th>
                <th className="text-left py-2 px-3 text-xs font-semibold text-gray-300">Status</th>
              </tr>
            </thead>
            <tbody>
              {settings.map((diff, index) => {
                const isAtDefault = JSON.stringify(diff.current) === JSON.stringify(diff.default);
                return (
                  <tr key={index} className={`border-b border-gray-700/50 hover:bg-gray-700/20 ${!isAtDefault ? 'bg-amber-500/5' : ''}`}>
                    <td className="py-2 px-3">
                      <div className="flex flex-col">
                        <span className="text-sm text-white font-medium">{diff.path}</span>
                        {diff.impact && <span className="text-xs text-gray-500 mt-0.5">{diff.impact}</span>}
                      </div>
                    </td>
                    <td className="py-2 px-3 text-sm font-mono">
                      <span className={!isAtDefault ? 'text-orange-400' : 'text-gray-400'}>{formatValue(diff.current)}</span>
                    </td>
                    <td className="py-2 px-3 text-sm text-green-400 font-mono">{formatValue(diff.default)}</td>
                    <td className="py-2 px-3">
                      {isAtDefault ? (
                        <span className="inline-block px-2 py-1 rounded text-xs font-medium border border-green-500/30 bg-green-500/10 text-green-400">OK</span>
                      ) : (
                        <span className={`inline-block px-2 py-1 rounded text-xs font-medium border ${getRiskColor(diff.risk_level)}`}>
                          {diff.risk_level.toUpperCase()}
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  // Count changes for user view
  const totalDifferent = differences.filter(d => JSON.stringify(d.current) !== JSON.stringify(d.default)).length;

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 rounded-xl max-w-5xl w-full max-h-[90vh] shadow-2xl border border-gray-700 flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-700">
          <div className="flex items-center gap-3">
            {/* Admin on individual modes: Show "Edit" instead of "Reset" in title */}
            <h3 className="text-lg font-bold text-white">
              {isAdmin && defaultValue && configType !== 'All Settings'
                ? title.replace('Reset', 'Edit')
                : title}
            </h3>
            {isAdmin && defaultValue && configType !== 'All Settings' && (
              <span className="px-2 py-1 text-xs bg-purple-500/20 text-purple-400 rounded border border-purple-500/30 flex items-center gap-1">
                <Edit3 className="w-3 h-3" /> Admin Edit Mode
              </span>
            )}
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-white transition-colors">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Body */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {loading ? (
            <div className="flex flex-col items-center justify-center py-12 space-y-3">
              <Loader2 className="w-8 h-8 text-blue-500 animate-spin" />
              <p className="text-gray-400">Loading...</p>
            </div>
          ) : configType === 'All Settings' ? (
            /* ===== RESET ALL SETTINGS: Same message for admin and normal user ===== */
            <div className="bg-blue-500/10 border border-blue-500/30 rounded-lg p-4 flex items-start gap-3">
              <RotateCcw className="w-6 h-6 text-blue-400 flex-shrink-0 mt-0.5" />
              <div>
                <p className="text-blue-400 font-medium">Restore All Settings to Default</p>
                <p className="text-sm text-gray-400 mt-1">
                  This will reset all mode configurations (Scalp, Swing, Ultra Fast, Position),
                  circuit breaker settings, LLM config, capital allocation, hedge mode, and
                  scalp re-entry settings to their default values from <code className="px-1 bg-gray-700 rounded text-xs">default-settings.json</code>.
                </p>
                <p className="text-sm text-yellow-400 mt-2">
                  <AlertTriangle className="w-4 h-4 inline mr-1" />
                  This action will overwrite your current database settings.
                </p>
              </div>
            </div>
          ) : isAdmin && defaultValue ? (
            /* ===== ADMIN VIEW: Edit default-settings.json (individual modes only) ===== */
            <>
              <div className="bg-purple-500/10 border border-purple-500/30 rounded-lg p-4 flex items-start gap-3">
                <Edit3 className="w-6 h-6 text-purple-400 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-purple-400 font-medium">Admin: Edit Default Settings</p>
                  <p className="text-sm text-gray-400 mt-1">
                    Edit the default values for <strong>{configType}</strong>. These values are saved to
                    <code className="mx-1 px-1 bg-gray-700 rounded text-xs">default-settings.json</code>
                    and used for new user initialization.
                  </p>
                </div>
              </div>

              {/* UI Visible Defaults */}
              {renderAdminDefaultsTable(adminVisibleDefaults, 'UI Visible Settings', 'bg-blue-500/30')}

              {/* Capital Allocation Total Display */}
              {configType === 'capital_allocation' && (
                <div className={`p-3 rounded-lg border ${
                  (() => {
                    const total = (Number(editedDefaults['ultra_fast_percent']) || 0) +
                                  (Number(editedDefaults['scalp_percent']) || 0) +
                                  (Number(editedDefaults['swing_percent']) || 0) +
                                  (Number(editedDefaults['position_percent']) || 0);
                    return total >= 99 && total <= 101;
                  })()
                    ? 'bg-green-500/10 border-green-500/30'
                    : 'bg-red-500/10 border-red-500/30'
                }`}>
                  <div className="flex items-center justify-between">
                    <span className="text-sm text-gray-300">Total Allocation:</span>
                    <span className={`text-lg font-bold ${
                      (() => {
                        const total = (Number(editedDefaults['ultra_fast_percent']) || 0) +
                                      (Number(editedDefaults['scalp_percent']) || 0) +
                                      (Number(editedDefaults['swing_percent']) || 0) +
                                      (Number(editedDefaults['position_percent']) || 0);
                        return total >= 99 && total <= 101 ? 'text-green-400' : 'text-red-400';
                      })()
                    }`}>
                      {((Number(editedDefaults['ultra_fast_percent']) || 0) +
                        (Number(editedDefaults['scalp_percent']) || 0) +
                        (Number(editedDefaults['swing_percent']) || 0) +
                        (Number(editedDefaults['position_percent']) || 0)).toFixed(1)}%
                    </span>
                  </div>
                  {validationError && (
                    <p className="text-sm text-red-400 mt-2 flex items-center gap-2">
                      <AlertTriangle className="w-4 h-4" />
                      {validationError}
                    </p>
                  )}
                </div>
              )}

              {/* Hidden Defaults Toggle */}
              {adminHiddenDefaults.length > 0 && (
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setShowHiddenSettings(!showHiddenSettings)}
                    className="flex items-center gap-2 px-3 py-2 bg-gray-700/50 hover:bg-gray-700 rounded-lg text-sm text-gray-300 transition-colors"
                  >
                    {showHiddenSettings ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    {showHiddenSettings ? 'Hide' : 'Show'} Advanced Settings ({adminHiddenDefaults.length})
                  </button>
                  <span className="text-xs text-gray-500">Settings not shown in UI but stored in config</span>
                </div>
              )}

              {/* Hidden Defaults */}
              {showHiddenSettings && renderAdminDefaultsTable(adminHiddenDefaults, 'Advanced Settings', 'bg-gray-600/30')}
            </>
          ) : (
            /* ===== USER VIEW: Show differences, option to reset ===== */
            <>
              {allMatch || totalDifferent === 0 ? (
                <div className="bg-green-500/10 border border-green-500/30 rounded-lg p-4 flex items-start gap-3">
                  <CheckCircle2 className="w-6 h-6 text-green-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-green-400 font-medium">All settings match defaults</p>
                    <p className="text-sm text-gray-400 mt-1">No changes needed for {configType} configuration.</p>
                  </div>
                </div>
              ) : (
                <div className="bg-blue-500/10 border border-blue-500/30 rounded-lg p-4 flex items-start gap-3">
                  <AlertTriangle className="w-6 h-6 text-blue-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-blue-400 font-medium">
                      {totalDifferent} setting{totalDifferent !== 1 ? 's' : ''} differ from defaults
                    </p>
                    <p className="text-sm text-gray-400 mt-1">
                      Click "Reset to Defaults" to restore your settings to match the default values.
                    </p>
                  </div>
                </div>
              )}

              {/* User differences */}
              {renderUserDiffSection(uiVisibleSettings, 'UI Visible Settings', 'bg-blue-500/30')}

              {hiddenSettings.length > 0 && (
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setShowHiddenSettings(!showHiddenSettings)}
                    className="flex items-center gap-2 px-3 py-2 bg-gray-700/50 hover:bg-gray-700 rounded-lg text-sm text-gray-300 transition-colors"
                  >
                    {showHiddenSettings ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    {showHiddenSettings ? 'Hide' : 'Show'} Advanced Settings ({hiddenSettings.length})
                  </button>
                </div>
              )}

              {showHiddenSettings && renderUserDiffSection(hiddenSettings, 'Advanced Settings', 'bg-gray-600/30')}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-700 flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 py-2 px-4 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors"
          >
            Cancel
          </button>

          {/*
            Button Logic:
            1. "Reset All Settings" (configType === 'All Settings'): Only "Restore to Database" for everyone
            2. Individual mode (admin): Only "Save to JSON" button, fields are editable
            3. Individual mode (normal user): Only "Restore to Database" button
          */}
          {configType === 'All Settings' ? (
            /* Reset All Settings - same for admin and normal user: only Restore to Database */
            <button
              onClick={onConfirm}
              disabled={loading}
              className="flex-1 py-2 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {loading ? (
                <><Loader2 className="w-4 h-4 animate-spin" /> Restoring...</>
              ) : (
                <><RotateCcw className="w-4 h-4" /> Restore to Database</>
              )}
            </button>
          ) : isAdmin && defaultValue ? (
            /* Individual mode for Admin: Only Save to JSON (fields are editable) */
            <button
              onClick={handleSaveDefaults}
              disabled={saving}
              className="flex-1 py-2 px-4 bg-purple-600 hover:bg-purple-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {saving ? (
                <><Loader2 className="w-4 h-4 animate-spin" /> Saving...</>
              ) : (
                <><Save className="w-4 h-4" /> Save to JSON</>
              )}
            </button>
          ) : (
            /* Normal user: Restore to Database */
            <button
              onClick={onConfirm}
              disabled={loading}
              className="flex-1 py-2 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
            >
              {loading ? (
                <><Loader2 className="w-4 h-4 animate-spin" /> Restoring...</>
              ) : (
                <><RotateCcw className="w-4 h-4" /> Restore to Database</>
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
