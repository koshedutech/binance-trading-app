// System Control Settings Component
// UI for configuring order tracking system and position management system

import React, { useState, useEffect, useCallback } from 'react';
import {
  Settings,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  RefreshCw,
  Info,
  Link2,
  Database,
  Layers,
  GitBranch,
} from 'lucide-react';
import systemControlApi, {
  type SystemControlSettings,
  type OrderTrackingSystem,
  type PositionManagementSystem,
  type EntryDecisionSystem,
} from '../../api/systemControl';

// ==================== Descriptions ====================

const ORDER_TRACKING_DESCRIPTIONS: Record<OrderTrackingSystem, string> = {
  chain: 'New chain-based event tracking (recommended)',
  legacy: 'Legacy event logging system',
  both: 'Log to both systems (for migration/testing)',
};

const POSITION_MANAGEMENT_DESCRIPTIONS: Record<PositionManagementSystem, string> = {
  legacy: 'Legacy position management system',
  chain: 'Chain-based position management (recommended)',
};

const ORDER_TRACKING_ICONS: Record<OrderTrackingSystem, React.ReactNode> = {
  chain: <Link2 className="w-5 h-5" />,
  legacy: <Database className="w-5 h-5" />,
  both: <Layers className="w-5 h-5" />,
};

const POSITION_MANAGEMENT_ICONS: Record<PositionManagementSystem, React.ReactNode> = {
  legacy: <Database className="w-5 h-5" />,
  chain: <GitBranch className="w-5 h-5" />,
};

const ENTRY_DECISION_DESCRIPTIONS: Record<EntryDecisionSystem, string> = {
  legacy: 'Legacy Ginie confluence system',
  chain: 'New chain-based entry decision system',
};

const ENTRY_DECISION_ICONS: Record<EntryDecisionSystem, React.ReactNode> = {
  legacy: <Database className="w-5 h-5" />,
  chain: <GitBranch className="w-5 h-5" />,
};

// ==================== Interfaces ====================

interface SystemControlSettingsProps {
  disabled?: boolean;
}

// ==================== Radio Selection Component ====================

interface RadioSelectionProps<T extends string> {
  label: string;
  description?: string;
  value: T;
  options: T[];
  onChange: (value: T) => void;
  disabled?: boolean;
  descriptions: Record<T, string>;
  icons: Record<T, React.ReactNode>;
}

function RadioSelection<T extends string>({
  label,
  description,
  value,
  options,
  onChange,
  disabled = false,
  descriptions,
  icons,
}: RadioSelectionProps<T>) {
  return (
    <div className="space-y-3">
      <div>
        <label className="text-sm font-medium text-gray-300">{label}</label>
        {description && (
          <p className="text-xs text-gray-500 mt-1">{description}</p>
        )}
      </div>
      <div className="space-y-2">
        {options.map((option) => (
          <button
            key={option}
            type="button"
            onClick={() => !disabled && onChange(option)}
            disabled={disabled}
            className={`
              w-full p-3 rounded-lg border-2 text-left transition-all flex items-center gap-3
              ${value === option
                ? 'border-purple-500 bg-purple-500/10'
                : 'border-gray-700 bg-gray-800 hover:border-gray-600'}
              ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
            `}
          >
            <div className={value === option ? 'text-purple-400' : 'text-gray-500'}>
              {icons[option]}
            </div>
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <span className={`font-medium capitalize ${value === option ? 'text-purple-400' : 'text-gray-300'}`}>
                  {option.replace('_', ' ')}
                </span>
                {value === option && (
                  <CheckCircle2 className="w-4 h-4 text-purple-400" />
                )}
              </div>
              <p className="text-xs text-gray-500 mt-0.5">{descriptions[option]}</p>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function SystemControlSettingsPanel({
  disabled = false,
}: SystemControlSettingsProps) {
  // State
  const [settings, setSettings] = useState<SystemControlSettings | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Pending changes (local state for form)
  const [pendingOrderTracking, setPendingOrderTracking] = useState<OrderTrackingSystem | null>(null);
  const [pendingPositionManagement, setPendingPositionManagement] = useState<PositionManagementSystem | null>(null);
  const [pendingEntryDecision, setPendingEntryDecision] = useState<EntryDecisionSystem | null>(null);

  // Load settings
  const loadSettings = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const data = await systemControlApi.getSystemControl();
      setSettings(data);
      setPendingOrderTracking(data.order_tracking_system);
      setPendingPositionManagement(data.position_management_system);
      setPendingEntryDecision(data.entry_decision_system);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load system control settings');
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Load on mount
  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  // Check if there are unsaved changes
  const hasChanges =
    settings &&
    (pendingOrderTracking !== settings.order_tracking_system ||
      pendingPositionManagement !== settings.position_management_system ||
      pendingEntryDecision !== settings.entry_decision_system);

  // Handle save
  const handleSave = async () => {
    if (!hasChanges || !pendingOrderTracking || !pendingPositionManagement || !pendingEntryDecision) return;

    setIsSaving(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const updated = await systemControlApi.updateSystemControl({
        order_tracking_system: pendingOrderTracking,
        position_management_system: pendingPositionManagement,
        entry_decision_system: pendingEntryDecision,
      });
      setSettings(updated);
      setSuccessMessage('System control settings saved successfully');
      setTimeout(() => setSuccessMessage(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  // Handle reset
  const handleReset = () => {
    if (settings) {
      setPendingOrderTracking(settings.order_tracking_system);
      setPendingPositionManagement(settings.position_management_system);
      setPendingEntryDecision(settings.entry_decision_system);
    }
  };

  const isDisabled = disabled || isLoading || isSaving;

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Settings className="w-5 h-5 text-purple-400" />
          <h3 className="font-semibold text-white">System Control Settings</h3>
        </div>
        <div className="flex items-center gap-2">
          {hasChanges && (
            <span className="text-xs text-yellow-400 bg-yellow-400/10 px-2 py-1 rounded">
              Unsaved changes
            </span>
          )}
          <button
            onClick={loadSettings}
            disabled={isDisabled}
            className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors disabled:opacity-50"
            title="Refresh settings"
          >
            <RefreshCw className={`w-3 h-3 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="p-4 space-y-6">
        {/* Loading state */}
        {isLoading && (
          <div className="flex items-center justify-center py-8">
            <Loader2 className="w-6 h-6 animate-spin text-purple-400" />
            <span className="ml-2 text-gray-400">Loading settings...</span>
          </div>
        )}

        {/* Error message */}
        {error && (
          <div className="p-3 bg-red-900/20 border border-red-500/30 rounded-lg flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-red-400 flex-shrink-0" />
            <span className="text-sm text-red-400">{error}</span>
          </div>
        )}

        {/* Success message */}
        {successMessage && (
          <div className="p-3 bg-green-900/20 border border-green-500/30 rounded-lg flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 text-green-400 flex-shrink-0" />
            <span className="text-sm text-green-400">{successMessage}</span>
          </div>
        )}

        {!isLoading && settings && (
          <>
            {/* Info box */}
            <div className="p-3 bg-blue-900/20 border border-blue-500/20 rounded-lg">
              <div className="flex items-start gap-2">
                <Info className="w-4 h-4 text-blue-400 mt-0.5 flex-shrink-0" />
                <p className="text-xs text-blue-300">
                  These settings control which system handles order tracking and position management.
                  Changes take effect immediately and affect how trades are logged and managed.
                </p>
              </div>
            </div>

            {/* Order Tracking System */}
            <RadioSelection
              label="Order Tracking System"
              description="Select which system tracks order events and lifecycle"
              value={pendingOrderTracking || settings.order_tracking_system}
              options={settings.valid_order_tracking_systems}
              onChange={setPendingOrderTracking}
              disabled={isDisabled}
              descriptions={ORDER_TRACKING_DESCRIPTIONS}
              icons={ORDER_TRACKING_ICONS}
            />

            {/* Position Management System */}
            <RadioSelection
              label="Position Management System"
              description="Select which system manages position state and events"
              value={pendingPositionManagement || settings.position_management_system}
              options={settings.valid_position_management_systems}
              onChange={setPendingPositionManagement}
              disabled={isDisabled}
              descriptions={POSITION_MANAGEMENT_DESCRIPTIONS}
              icons={POSITION_MANAGEMENT_ICONS}
            />

            {/* Entry Decision System */}
            <RadioSelection
              label="Entry Decision System"
              description="Select which system determines trade entry decisions"
              value={pendingEntryDecision || settings.entry_decision_system}
              options={settings.valid_entry_decision_systems}
              onChange={setPendingEntryDecision}
              disabled={isDisabled}
              descriptions={ENTRY_DECISION_DESCRIPTIONS}
              icons={ENTRY_DECISION_ICONS}
            />

            {/* Action buttons */}
            <div className="pt-4 border-t border-gray-700 flex gap-3">
              <button
                onClick={handleReset}
                disabled={isDisabled || !hasChanges}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <RefreshCw className="w-4 h-4" />
                Reset
              </button>
              <button
                onClick={handleSave}
                disabled={isDisabled || !hasChanges}
                className="flex-1 flex items-center justify-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Saving...
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="w-4 h-4" />
                    Save Settings
                  </>
                )}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
