// Epic 10: Position Stage Management - Story 10.1: Efficiency Exit Settings Panel
// Settings panel for configuring efficiency-based exit parameters

import React, { useState, useEffect, useCallback } from 'react';
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  RefreshCw,
  Info,
  Clock,
  TrendingUp,
  BarChart,
} from 'lucide-react';
import type { EfficiencyExitSettings } from '../../types/positionManagement';
import { DEFAULT_EFFICIENCY_SETTINGS } from '../../types/positionManagement';

// ==================== Interfaces ====================

interface EfficiencyExitSettingsProps {
  settings?: EfficiencyExitSettings;
  onSettingsChange?: (settings: EfficiencyExitSettings) => void;
  onSave?: (settings: EfficiencyExitSettings) => Promise<void>;
  isLoading?: boolean;
  disabled?: boolean;
}

// ==================== Slider Component ====================

interface SliderInputProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (value: number) => void;
  disabled?: boolean;
  description?: string;
  unit?: string;
  icon?: React.ReactNode;
}

function SliderInput({
  label,
  value,
  min,
  max,
  step = 1,
  onChange,
  disabled = false,
  description,
  unit = '',
  icon,
}: SliderInputProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {icon}
          <label className="text-sm font-medium text-gray-300">{label}</label>
        </div>
        <span className="text-sm font-mono text-purple-400">
          {value}{unit}
        </span>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        disabled={disabled}
        className="w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed accent-purple-500"
      />
      <div className="flex justify-between text-xs text-gray-600">
        <span>{min}{unit}</span>
        <span>{max}{unit}</span>
      </div>
    </div>
  );
}

// ==================== Number Input Component ====================

interface NumberInputProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (value: number) => void;
  disabled?: boolean;
  description?: string;
  unit?: string;
  icon?: React.ReactNode;
}

function NumberInput({
  label,
  value,
  min,
  max,
  step = 1,
  onChange,
  disabled = false,
  description,
  unit = '',
  icon,
}: NumberInputProps) {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = Number(e.target.value);
    if (newValue >= min && newValue <= max) {
      onChange(newValue);
    }
  };

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {icon}
          <label className="text-sm font-medium text-gray-300">{label}</label>
        </div>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
      <div className="flex items-center gap-2">
        <input
          type="number"
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={handleChange}
          disabled={disabled}
          className="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 focus:border-transparent disabled:opacity-50 disabled:cursor-not-allowed"
        />
        {unit && <span className="text-sm text-gray-500">{unit}</span>}
      </div>
    </div>
  );
}

// ==================== Toggle Component ====================

interface ToggleInputProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  description?: string;
  icon?: React.ReactNode;
}

function ToggleInput({
  label,
  checked,
  onChange,
  disabled = false,
  description,
  icon,
}: ToggleInputProps) {
  return (
    <div className="flex items-start justify-between py-3 border-b border-gray-700/50 last:border-0">
      <div className="flex-1">
        <div className="flex items-center gap-2">
          {icon}
          <label className="text-sm font-medium text-gray-300">{label}</label>
        </div>
        {description && (
          <p className="text-xs text-gray-500 mt-1 ml-6">{description}</p>
        )}
      </div>
      <button
        type="button"
        onClick={() => !disabled && onChange(!checked)}
        disabled={disabled}
        className={`
          relative inline-flex h-6 w-11 items-center rounded-full transition-colors
          ${checked ? 'bg-purple-600' : 'bg-gray-600'}
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
        `}
      >
        <span
          className={`
            inline-block h-4 w-4 transform rounded-full bg-white transition-transform
            ${checked ? 'translate-x-6' : 'translate-x-1'}
          `}
        />
      </button>
    </div>
  );
}

// ==================== Threshold Progress Bar ====================

interface ThresholdDisplayProps {
  currentThreshold: number;
  label?: string;
}

function ThresholdDisplay({ currentThreshold, label = 'Current Efficiency Threshold' }: ThresholdDisplayProps) {
  const progressColor = currentThreshold >= 80 ? 'bg-green-500' :
    currentThreshold >= 50 ? 'bg-yellow-500' : 'bg-red-500';

  return (
    <div className="p-4 bg-gray-700/30 rounded-lg">
      <div className="flex items-center justify-between mb-2">
        <span className="text-sm text-gray-400">{label}</span>
        <span className="text-lg font-bold text-purple-400">
          {currentThreshold.toFixed(1)}%
        </span>
      </div>
      <div className="h-3 bg-gray-700 rounded-full overflow-hidden">
        <div
          className={`h-full ${progressColor} transition-all duration-500`}
          style={{ width: `${Math.min(100, currentThreshold)}%` }}
        />
      </div>
      <p className="mt-2 text-xs text-gray-500">
        This threshold is dynamically calculated based on historical performance and market conditions.
        Positions exit when efficiency drops below this value.
      </p>
    </div>
  );
}

// ==================== Main Component ====================

export default function EfficiencyExitSettingsPanel({
  settings: initialSettings,
  onSettingsChange,
  onSave,
  isLoading = false,
  disabled = false,
}: EfficiencyExitSettingsProps) {
  // Local state for form
  const [settings, setSettings] = useState<EfficiencyExitSettings>(DEFAULT_EFFICIENCY_SETTINGS);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Initialize from props
  useEffect(() => {
    if (initialSettings) {
      setSettings(initialSettings);
    }
  }, [initialSettings]);

  // Update setting helper
  const updateSetting = useCallback(<K extends keyof EfficiencyExitSettings>(
    key: K,
    value: EfficiencyExitSettings[K]
  ) => {
    const newSettings = { ...settings, [key]: value };
    setSettings(newSettings);
    onSettingsChange?.(newSettings);
  }, [settings, onSettingsChange]);

  // Handle save
  const handleSave = async () => {
    if (!onSave) return;

    setSaving(true);
    setError(null);
    setSuccessMessage(null);

    try {
      await onSave(settings);
      setSuccessMessage('Settings saved successfully');
      setTimeout(() => setSuccessMessage(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  // Handle reset to defaults
  const handleReset = () => {
    setSettings(DEFAULT_EFFICIENCY_SETTINGS);
    onSettingsChange?.(DEFAULT_EFFICIENCY_SETTINGS);
  };

  const isDisabled = disabled || isLoading || saving;

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Activity className="w-5 h-5 text-purple-400" />
          <h3 className="font-semibold text-white">Efficiency Exit Settings</h3>
        </div>
        <button
          onClick={handleReset}
          disabled={isDisabled}
          className="flex items-center gap-1 px-2 py-1 text-xs text-gray-400 hover:text-gray-200 hover:bg-gray-700 rounded transition-colors disabled:opacity-50"
        >
          <RefreshCw className="w-3 h-3" />
          Reset
        </button>
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
            <AlertTriangle className="w-4 h-4 text-red-400" />
            <span className="text-sm text-red-400">{error}</span>
          </div>
        )}

        {/* Success message */}
        {successMessage && (
          <div className="p-3 bg-green-900/20 border border-green-500/30 rounded-lg flex items-center gap-2">
            <CheckCircle2 className="w-4 h-4 text-green-400" />
            <span className="text-sm text-green-400">{successMessage}</span>
          </div>
        )}

        {!isLoading && (
          <>
            {/* Enable Toggle */}
            <ToggleInput
              label="Enable Efficiency Exit"
              checked={settings.enabled}
              onChange={(v) => updateSetting('enabled', v)}
              disabled={isDisabled}
              description="Automatically exit positions when profit efficiency drops below threshold"
              icon={<TrendingUp className="w-4 h-4 text-purple-400" />}
            />

            {/* Settings (only shown when enabled) */}
            {settings.enabled && (
              <div className="space-y-5 pt-2">
                {/* Current Threshold Display */}
                <ThresholdDisplay currentThreshold={settings.current_threshold_percent} />

                {/* Historical Window */}
                <SliderInput
                  label="Historical Window"
                  value={settings.historical_window_hours}
                  min={1}
                  max={168}
                  step={1}
                  onChange={(v) => updateSetting('historical_window_hours', v)}
                  disabled={isDisabled}
                  description="Hours of trading history to analyze for efficiency calculation"
                  unit=" hours"
                  icon={<Clock className="w-4 h-4 text-blue-400" />}
                />

                {/* Minimum Hold Time */}
                <NumberInput
                  label="Minimum Hold Time"
                  value={settings.min_hold_time_minutes}
                  min={1}
                  max={1440}
                  step={1}
                  onChange={(v) => updateSetting('min_hold_time_minutes', v)}
                  disabled={isDisabled}
                  description="Minimum time to hold position before efficiency exit can trigger"
                  unit="minutes"
                  icon={<Clock className="w-4 h-4 text-yellow-400" />}
                />

                {/* Consecutive Signals Required */}
                <NumberInput
                  label="Consecutive Signals Required"
                  value={settings.consecutive_signals_required}
                  min={1}
                  max={5}
                  step={1}
                  onChange={(v) => updateSetting('consecutive_signals_required', v)}
                  disabled={isDisabled}
                  description="Number of consecutive efficiency drop signals required to exit"
                  unit="signals"
                  icon={<BarChart className="w-4 h-4 text-green-400" />}
                />

                {/* Info box */}
                <div className="p-3 bg-purple-900/20 border border-purple-500/30 rounded-lg">
                  <div className="flex items-start gap-2">
                    <Info className="w-4 h-4 text-purple-400 mt-0.5 flex-shrink-0" />
                    <div className="text-xs text-purple-300">
                      <p className="font-medium mb-1">How Efficiency Exit Works:</p>
                      <ul className="space-y-1 list-disc list-inside text-purple-300/80">
                        <li>Monitors peak profit vs current profit ratio</li>
                        <li>Calculates dynamic threshold from historical trades</li>
                        <li>Exits when efficiency drops below threshold</li>
                        <li>Helps lock in profits during pullbacks</li>
                      </ul>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Disabled state info */}
            {!settings.enabled && (
              <div className="p-4 bg-gray-700/30 rounded-lg text-center">
                <Activity className="w-8 h-8 text-gray-600 mx-auto mb-2" />
                <p className="text-sm text-gray-500">
                  Efficiency exit is disabled. Enable it to automatically exit positions
                  when profit efficiency drops below dynamically calculated thresholds.
                </p>
              </div>
            )}

            {/* Save button */}
            {onSave && (
              <div className="pt-4 border-t border-gray-700">
                <button
                  onClick={handleSave}
                  disabled={isDisabled}
                  className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {saving ? (
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
            )}
          </>
        )}
      </div>
    </div>
  );
}
