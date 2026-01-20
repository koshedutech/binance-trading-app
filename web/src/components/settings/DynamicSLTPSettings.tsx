// Epic 10: Position Stage Management - Story 10.1: Dynamic SL/TP Settings Panel
// Settings panel for configuring dynamic stop loss and take profit parameters

import React, { useState, useEffect, useCallback } from 'react';
import {
  Shield,
  Target,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  RefreshCw,
  Info,
  Zap,
  Upload,
} from 'lucide-react';
import type { DynamicSLTPSettings } from '../../types/positionManagement';
import { DEFAULT_DYNAMIC_SLTP_SETTINGS } from '../../types/positionManagement';

// ==================== Interfaces ====================

interface DynamicSLTPSettingsProps {
  settings?: DynamicSLTPSettings;
  onSettingsChange?: (settings: DynamicSLTPSettings) => void;
  onSave?: (settings: DynamicSLTPSettings) => Promise<void>;
  isLoading?: boolean;
  disabled?: boolean;
}

// ==================== Toggle Component ====================

interface ToggleInputProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  description?: string;
  icon?: React.ReactNode;
  iconColor?: string;
}

function ToggleInput({
  label,
  checked,
  onChange,
  disabled = false,
  description,
  icon,
  iconColor = 'text-purple-400',
}: ToggleInputProps) {
  return (
    <div className="flex items-start justify-between py-3 border-b border-gray-700/50 last:border-0">
      <div className="flex-1">
        <div className="flex items-center gap-2">
          {icon && <span className={iconColor}>{icon}</span>}
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
  iconColor?: string;
  accentColor?: string;
}

function SliderInput({
  label,
  value,
  min,
  max,
  step = 0.1,
  onChange,
  disabled = false,
  description,
  unit = 'x',
  icon,
  iconColor = 'text-purple-400',
  accentColor = 'accent-purple-500',
}: SliderInputProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {icon && <span className={iconColor}>{icon}</span>}
          <label className="text-sm font-medium text-gray-300">{label}</label>
        </div>
        <span className={`text-sm font-mono ${iconColor}`}>
          {value.toFixed(1)}{unit}
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
        className={`w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed ${accentColor}`}
      />
      <div className="flex justify-between text-xs text-gray-600">
        <span>{min}{unit}</span>
        <span>{max}{unit}</span>
      </div>
    </div>
  );
}

// ==================== ATR Preview Component ====================

interface ATRPreviewProps {
  slMultiplier: number;
  tpMultiplier: number;
}

function ATRPreview({ slMultiplier, tpMultiplier }: ATRPreviewProps) {
  // Example ATR value for demonstration
  const exampleATR = 0.50; // 0.50 USDT per ATR
  const examplePrice = 100; // $100 example price

  const slDistance = exampleATR * slMultiplier;
  const tpDistance = exampleATR * tpMultiplier;
  const slPercent = (slDistance / examplePrice) * 100;
  const tpPercent = (tpDistance / examplePrice) * 100;

  return (
    <div className="p-4 bg-gray-700/30 rounded-lg">
      <h4 className="text-sm font-medium text-gray-400 mb-3">Example Calculation</h4>
      <div className="text-xs text-gray-500 mb-3">
        Assuming ATR = ${exampleATR.toFixed(2)} at price ${examplePrice}
      </div>

      <div className="grid grid-cols-2 gap-4">
        {/* Stop Loss */}
        <div className="p-3 bg-red-900/20 border border-red-500/30 rounded-lg">
          <div className="flex items-center gap-2 mb-2">
            <Shield className="w-4 h-4 text-red-400" />
            <span className="text-sm font-medium text-red-400">Stop Loss</span>
          </div>
          <div className="text-lg font-bold text-red-400">
            ${slDistance.toFixed(2)}
          </div>
          <div className="text-xs text-red-400/70">
            {slPercent.toFixed(2)}% from entry
          </div>
        </div>

        {/* Take Profit */}
        <div className="p-3 bg-green-900/20 border border-green-500/30 rounded-lg">
          <div className="flex items-center gap-2 mb-2">
            <Target className="w-4 h-4 text-green-400" />
            <span className="text-sm font-medium text-green-400">Take Profit</span>
          </div>
          <div className="text-lg font-bold text-green-400">
            ${tpDistance.toFixed(2)}
          </div>
          <div className="text-xs text-green-400/70">
            {tpPercent.toFixed(2)}% from entry
          </div>
        </div>
      </div>

      <div className="mt-3 text-xs text-gray-500">
        Risk/Reward Ratio: 1:{(tpMultiplier / slMultiplier).toFixed(1)}
      </div>
    </div>
  );
}

// ==================== Main Component ====================

export default function DynamicSLTPSettingsPanel({
  settings: initialSettings,
  onSettingsChange,
  onSave,
  isLoading = false,
  disabled = false,
}: DynamicSLTPSettingsProps) {
  // Local state for form
  const [settings, setSettings] = useState<DynamicSLTPSettings>(DEFAULT_DYNAMIC_SLTP_SETTINGS);
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
  const updateSetting = useCallback(<K extends keyof DynamicSLTPSettings>(
    key: K,
    value: DynamicSLTPSettings[K]
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
    setSettings(DEFAULT_DYNAMIC_SLTP_SETTINGS);
    onSettingsChange?.(DEFAULT_DYNAMIC_SLTP_SETTINGS);
  };

  const isDisabled = disabled || isLoading || saving;
  const anyEnabled = settings.dynamic_sl_enabled || settings.dynamic_tp_enabled;

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Zap className="w-5 h-5 text-yellow-400" />
          <h3 className="font-semibold text-white">Dynamic SL/TP Settings</h3>
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
            <Loader2 className="w-6 h-6 animate-spin text-yellow-400" />
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
            {/* Enable Toggles */}
            <div className="space-y-1">
              <ToggleInput
                label="Enable Dynamic Stop Loss"
                checked={settings.dynamic_sl_enabled}
                onChange={(v) => updateSetting('dynamic_sl_enabled', v)}
                disabled={isDisabled}
                description="Automatically adjust stop loss based on ATR and market volatility"
                icon={<Shield className="w-4 h-4" />}
                iconColor="text-red-400"
              />

              <ToggleInput
                label="Enable Dynamic Take Profit"
                checked={settings.dynamic_tp_enabled}
                onChange={(v) => updateSetting('dynamic_tp_enabled', v)}
                disabled={isDisabled}
                description="Automatically adjust take profit based on ATR and market conditions"
                icon={<Target className="w-4 h-4" />}
                iconColor="text-green-400"
              />

              <ToggleInput
                label="Update on Binance"
                checked={settings.update_on_binance}
                onChange={(v) => updateSetting('update_on_binance', v)}
                disabled={isDisabled}
                description="Sync dynamic SL/TP orders to Binance exchange"
                icon={<Upload className="w-4 h-4" />}
                iconColor="text-yellow-400"
              />
            </div>

            {/* ATR Multiplier Settings (only shown when enabled) */}
            {anyEnabled && (
              <div className="space-y-5 pt-2">
                {/* Stop Loss ATR Multiplier */}
                {settings.dynamic_sl_enabled && (
                  <SliderInput
                    label="ATR Multiplier for SL"
                    value={settings.atr_multiplier_sl}
                    min={1.0}
                    max={5.0}
                    step={0.1}
                    onChange={(v) => updateSetting('atr_multiplier_sl', v)}
                    disabled={isDisabled}
                    description="Stop loss distance as a multiple of ATR"
                    unit="x"
                    icon={<Shield className="w-4 h-4" />}
                    iconColor="text-red-400"
                    accentColor="accent-red-500"
                  />
                )}

                {/* Take Profit ATR Multiplier */}
                {settings.dynamic_tp_enabled && (
                  <SliderInput
                    label="ATR Multiplier for TP"
                    value={settings.atr_multiplier_tp}
                    min={1.5}
                    max={10.0}
                    step={0.1}
                    onChange={(v) => updateSetting('atr_multiplier_tp', v)}
                    disabled={isDisabled}
                    description="Take profit distance as a multiple of ATR"
                    unit="x"
                    icon={<Target className="w-4 h-4" />}
                    iconColor="text-green-400"
                    accentColor="accent-green-500"
                  />
                )}

                {/* ATR Preview */}
                <ATRPreview
                  slMultiplier={settings.atr_multiplier_sl}
                  tpMultiplier={settings.atr_multiplier_tp}
                />

                {/* Info box */}
                <div className="p-3 bg-yellow-900/20 border border-yellow-500/30 rounded-lg">
                  <div className="flex items-start gap-2">
                    <Info className="w-4 h-4 text-yellow-400 mt-0.5 flex-shrink-0" />
                    <div className="text-xs text-yellow-300">
                      <p className="font-medium mb-1">How Dynamic SL/TP Works:</p>
                      <ul className="space-y-1 list-disc list-inside text-yellow-300/80">
                        <li>Uses Average True Range (ATR) to measure volatility</li>
                        <li>SL/TP distances adapt to current market conditions</li>
                        <li>Higher ATR = wider stops, lower ATR = tighter stops</li>
                        <li>Helps avoid premature stop-outs in volatile markets</li>
                      </ul>
                    </div>
                  </div>
                </div>

                {/* Update on Binance warning */}
                {settings.update_on_binance && (
                  <div className="p-3 bg-orange-900/20 border border-orange-500/30 rounded-lg">
                    <div className="flex items-start gap-2">
                      <AlertTriangle className="w-4 h-4 text-orange-400 mt-0.5 flex-shrink-0" />
                      <div className="text-xs text-orange-300">
                        <p className="font-medium">Binance Sync Enabled</p>
                        <p className="mt-1 text-orange-300/80">
                          Dynamic SL/TP changes will be synced to Binance orders.
                          This may incur additional API calls and order modifications.
                        </p>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Disabled state info */}
            {!anyEnabled && (
              <div className="p-4 bg-gray-700/30 rounded-lg text-center">
                <Zap className="w-8 h-8 text-gray-600 mx-auto mb-2" />
                <p className="text-sm text-gray-500">
                  Dynamic SL/TP is disabled. Enable either dynamic stop loss or
                  take profit to automatically adjust levels based on market volatility.
                </p>
              </div>
            )}

            {/* Save button */}
            {onSave && (
              <div className="pt-4 border-t border-gray-700">
                <button
                  onClick={handleSave}
                  disabled={isDisabled}
                  className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-yellow-600 hover:bg-yellow-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
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
