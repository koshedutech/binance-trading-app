// Epic 10: Position Stage Management - Story 10.1: Position Decision Settings Panel
// Settings panel for configuring position exit decision mode and parameters

import React, { useState, useEffect, useCallback } from 'react';
import {
  BarChart2,
  Brain,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  RefreshCw,
  Info,
} from 'lucide-react';
import type {
  DecisionMode,
  ClassicDecisionSettings,
  NewEngineDecisionSettings,
  PositionDecisionSettings,
} from '../../types/positionManagement';
import {
  DEFAULT_CLASSIC_SETTINGS,
  DEFAULT_NEW_ENGINE_SETTINGS,
} from '../../types/positionManagement';

// ==================== Interfaces ====================

interface PositionDecisionSettingsProps {
  settings?: PositionDecisionSettings;
  onSettingsChange?: (settings: PositionDecisionSettings) => void;
  onSave?: (settings: PositionDecisionSettings) => Promise<void>;
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
}: SliderInputProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium text-gray-300">{label}</label>
        <span className="text-sm text-gray-400">
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

// ==================== Toggle Component ====================

interface ToggleInputProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  description?: string;
}

function ToggleInput({
  label,
  checked,
  onChange,
  disabled = false,
  description,
}: ToggleInputProps) {
  return (
    <div className="flex items-start justify-between py-2">
      <div className="flex-1">
        <label className="text-sm font-medium text-gray-300">{label}</label>
        {description && (
          <p className="text-xs text-gray-500 mt-1">{description}</p>
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

// ==================== Mode Selection Component ====================

interface ModeSelectionProps {
  mode: DecisionMode;
  onChange: (mode: DecisionMode) => void;
  disabled?: boolean;
}

function ModeSelection({ mode, onChange, disabled = false }: ModeSelectionProps) {
  const modes: { value: DecisionMode; label: string; icon: React.ReactNode; description: string }[] = [
    {
      value: 'classic',
      label: 'Classic Mode',
      icon: <BarChart2 className="w-5 h-5" />,
      description: 'Traditional indicators: ADX, RSI, reversal signals',
    },
    {
      value: 'new_engine',
      label: 'New Decision Engine',
      icon: <Brain className="w-5 h-5" />,
      description: 'AI-powered: strategy-aware, regime-adaptive scoring',
    },
  ];

  return (
    <div className="space-y-3">
      <label className="text-sm font-medium text-gray-400">Decision Mode</label>
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {modes.map((m) => (
          <button
            key={m.value}
            type="button"
            onClick={() => !disabled && onChange(m.value)}
            disabled={disabled}
            className={`
              p-4 rounded-lg border-2 text-left transition-all
              ${mode === m.value
                ? 'border-purple-500 bg-purple-500/10'
                : 'border-gray-700 bg-gray-800 hover:border-gray-600'}
              ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
            `}
          >
            <div className="flex items-center gap-3 mb-2">
              <div className={mode === m.value ? 'text-purple-400' : 'text-gray-500'}>
                {m.icon}
              </div>
              <span className={`font-medium ${mode === m.value ? 'text-purple-400' : 'text-gray-300'}`}>
                {m.label}
              </span>
              {mode === m.value && (
                <CheckCircle2 className="w-4 h-4 text-purple-400 ml-auto" />
              )}
            </div>
            <p className="text-xs text-gray-500">{m.description}</p>
          </button>
        ))}
      </div>
    </div>
  );
}

// ==================== Classic Settings Panel ====================

interface ClassicSettingsPanelProps {
  settings: ClassicDecisionSettings;
  onChange: (settings: ClassicDecisionSettings) => void;
  disabled?: boolean;
}

function ClassicSettingsPanel({ settings, onChange, disabled = false }: ClassicSettingsPanelProps) {
  const updateSetting = <K extends keyof ClassicDecisionSettings>(
    key: K,
    value: ClassicDecisionSettings[K]
  ) => {
    onChange({ ...settings, [key]: value });
  };

  return (
    <div className="space-y-4 p-4 bg-blue-900/10 border border-blue-500/20 rounded-lg">
      <div className="flex items-center gap-2 mb-2">
        <BarChart2 className="w-4 h-4 text-blue-400" />
        <h4 className="text-sm font-medium text-blue-400">Classic Mode Settings</h4>
      </div>

      <SliderInput
        label="ADX Reversal Threshold"
        value={settings.adx_reversal_threshold}
        min={15}
        max={30}
        onChange={(v) => updateSetting('adx_reversal_threshold', v)}
        disabled={disabled}
        description="Minimum ADX value to confirm trend strength for reversal detection"
      />

      <SliderInput
        label="Reversal Confirmations"
        value={settings.reversal_confirmations}
        min={1}
        max={4}
        onChange={(v) => updateSetting('reversal_confirmations', v)}
        disabled={disabled}
        description="Number of consecutive reversal signals required to exit"
      />

      <div className="grid grid-cols-2 gap-4">
        <SliderInput
          label="RSI Oversold"
          value={settings.rsi_oversold}
          min={20}
          max={40}
          onChange={(v) => updateSetting('rsi_oversold', v)}
          disabled={disabled}
          description="RSI level indicating oversold conditions"
        />

        <SliderInput
          label="RSI Overbought"
          value={settings.rsi_overbought}
          min={60}
          max={80}
          onChange={(v) => updateSetting('rsi_overbought', v)}
          disabled={disabled}
          description="RSI level indicating overbought conditions"
        />
      </div>

      <div className="mt-3 p-3 bg-blue-900/20 rounded-lg">
        <div className="flex items-start gap-2">
          <Info className="w-4 h-4 text-blue-400 mt-0.5 flex-shrink-0" />
          <p className="text-xs text-blue-300">
            Classic mode uses traditional technical indicators to detect trend reversals.
            Exit signals are generated when ADX confirms trend weakness and reversal
            confirmations are met.
          </p>
        </div>
      </div>
    </div>
  );
}

// ==================== New Engine Settings Panel ====================

interface NewEngineSettingsPanelProps {
  settings: NewEngineDecisionSettings;
  onChange: (settings: NewEngineDecisionSettings) => void;
  disabled?: boolean;
}

function NewEngineSettingsPanel({ settings, onChange, disabled = false }: NewEngineSettingsPanelProps) {
  const updateSetting = <K extends keyof NewEngineDecisionSettings>(
    key: K,
    value: NewEngineDecisionSettings[K]
  ) => {
    onChange({ ...settings, [key]: value });
  };

  return (
    <div className="space-y-4 p-4 bg-purple-900/10 border border-purple-500/20 rounded-lg">
      <div className="flex items-center gap-2 mb-2">
        <Brain className="w-4 h-4 text-purple-400" />
        <h4 className="text-sm font-medium text-purple-400">New Engine Settings</h4>
      </div>

      <ToggleInput
        label="Use Active Strategy"
        checked={settings.use_active_strategy}
        onChange={(v) => updateSetting('use_active_strategy', v)}
        disabled={disabled}
        description="Use the currently active strategy's exit rules based on market regime"
      />

      <ToggleInput
        label="Exit on Regime Change"
        checked={settings.exit_on_regime_change}
        onChange={(v) => updateSetting('exit_on_regime_change', v)}
        disabled={disabled}
        description="Automatically exit when market regime changes significantly"
      />

      <ToggleInput
        label="Use Strategy Exit Rules"
        checked={settings.use_strategy_exit_rules}
        onChange={(v) => updateSetting('use_strategy_exit_rules', v)}
        disabled={disabled}
        description="Apply strategy-specific exit conditions (trailing stops, time limits)"
      />

      <div className="mt-3 p-3 bg-purple-900/20 rounded-lg">
        <div className="flex items-start gap-2">
          <Info className="w-4 h-4 text-purple-400 mt-0.5 flex-shrink-0" />
          <p className="text-xs text-purple-300">
            New Decision Engine uses AI-powered scoring and regime detection.
            It adapts exit decisions based on market conditions and the active
            trading strategy for optimal position management.
          </p>
        </div>
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function PositionDecisionSettingsPanel({
  settings: initialSettings,
  onSettingsChange,
  onSave,
  isLoading = false,
  disabled = false,
}: PositionDecisionSettingsProps) {
  // Local state for form
  const [settings, setSettings] = useState<PositionDecisionSettings>({
    mode: 'new_engine',
    classic: DEFAULT_CLASSIC_SETTINGS,
    new_engine: DEFAULT_NEW_ENGINE_SETTINGS,
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Initialize from props
  useEffect(() => {
    if (initialSettings) {
      setSettings(initialSettings);
    }
  }, [initialSettings]);

  // Handle mode change
  const handleModeChange = useCallback((mode: DecisionMode) => {
    const newSettings = { ...settings, mode };
    setSettings(newSettings);
    onSettingsChange?.(newSettings);
  }, [settings, onSettingsChange]);

  // Handle classic settings change
  const handleClassicChange = useCallback((classic: ClassicDecisionSettings) => {
    const newSettings = { ...settings, classic };
    setSettings(newSettings);
    onSettingsChange?.(newSettings);
  }, [settings, onSettingsChange]);

  // Handle new engine settings change
  const handleNewEngineChange = useCallback((new_engine: NewEngineDecisionSettings) => {
    const newSettings = { ...settings, new_engine };
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
    const defaultSettings: PositionDecisionSettings = {
      mode: 'new_engine',
      classic: DEFAULT_CLASSIC_SETTINGS,
      new_engine: DEFAULT_NEW_ENGINE_SETTINGS,
    };
    setSettings(defaultSettings);
    onSettingsChange?.(defaultSettings);
  };

  const isDisabled = disabled || isLoading || saving;

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Brain className="w-5 h-5 text-purple-400" />
          <h3 className="font-semibold text-white">Position Decision Settings</h3>
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
            {/* Mode Selection */}
            <ModeSelection
              mode={settings.mode}
              onChange={handleModeChange}
              disabled={isDisabled}
            />

            {/* Mode-specific settings */}
            {settings.mode === 'classic' ? (
              <ClassicSettingsPanel
                settings={settings.classic}
                onChange={handleClassicChange}
                disabled={isDisabled}
              />
            ) : (
              <NewEngineSettingsPanel
                settings={settings.new_engine}
                onChange={handleNewEngineChange}
                disabled={isDisabled}
              />
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
