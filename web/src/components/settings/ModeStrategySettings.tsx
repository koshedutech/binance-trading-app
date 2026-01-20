// Epic 11: Position Decision Engine - Story 11.33: Mode-Strategy UI Update
// Main component for Mode->Strategy hierarchy settings

import React, { useState, useEffect, useCallback } from 'react';
import {
  Settings2,
  TrendingUp,
  RotateCcw,
  Zap,
  Target,
  BarChart3,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  ChevronDown,
  Power,
  PowerOff,
} from 'lucide-react';
import type {
  ModeName,
  StrategyName,
  ModeConfig,
  ModeStrategyConfig,
  StrategyComparisonResponse,
} from '../../types/modeStrategy';
import {
  MODE_DISPLAY_NAMES,
  MODE_DESCRIPTIONS,
  STRATEGY_DISPLAY_NAMES,
  STRATEGY_DESCRIPTIONS,
} from '../../types/modeStrategy';
import modeStrategyApi from '../../api/modeStrategy';
import StrategySettingsForm from './StrategySettingsForm';

// ==================== Interfaces ====================

interface ModeStrategySettingsProps {
  initialMode?: ModeName;
  onModeChange?: (mode: ModeName) => void;
  disabled?: boolean;
}

// ==================== Strategy Tab Component ====================

interface StrategyTabProps {
  strategyName: StrategyName;
  config: ModeStrategyConfig;
  isActive: boolean;
  onClick: () => void;
  onToggle: (enabled: boolean) => void;
  disabled?: boolean;
}

function StrategyTab({
  strategyName,
  config,
  isActive,
  onClick,
  onToggle,
  disabled = false,
}: StrategyTabProps) {
  const getStrategyIcon = (name: StrategyName) => {
    switch (name) {
      case 'trend_following':
        return <TrendingUp className="w-4 h-4" />;
      case 'mean_reversion':
        return <RotateCcw className="w-4 h-4" />;
      case 'breakout':
        return <Zap className="w-4 h-4" />;
      case 'range_trading':
        return <Target className="w-4 h-4" />;
    }
  };

  const handleToggleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!disabled) {
      onToggle(!config.enabled);
    }
  };

  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`
        relative flex flex-col items-center p-3 rounded-lg border-2 transition-all min-w-[110px]
        ${isActive
          ? 'border-purple-500 bg-purple-500/10'
          : 'border-gray-700 bg-gray-800 hover:border-gray-600'}
        ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
      `}
    >
      {/* Strategy Icon */}
      <div className={`mb-1 ${isActive ? 'text-purple-400' : 'text-gray-400'}`}>
        {getStrategyIcon(strategyName)}
      </div>

      {/* Strategy Name */}
      <span className={`text-xs font-medium text-center ${isActive ? 'text-purple-300' : 'text-gray-300'}`}>
        {STRATEGY_DISPLAY_NAMES[strategyName]}
      </span>

      {/* Enable/Disable Toggle */}
      <div
        onClick={handleToggleClick}
        className={`
          mt-2 flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium transition-colors
          ${config.enabled
            ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
            : 'bg-gray-600/20 text-gray-500 hover:bg-gray-600/30'}
          ${disabled ? '' : 'cursor-pointer'}
        `}
      >
        {config.enabled ? (
          <>
            <Power className="w-3 h-3" />
            ON
          </>
        ) : (
          <>
            <PowerOff className="w-3 h-3" />
            OFF
          </>
        )}
      </div>
    </button>
  );
}

// ==================== Mode Selector Component ====================

interface ModeSelectorProps {
  currentMode: ModeName;
  onChange: (mode: ModeName) => void;
  disabled?: boolean;
}

function ModeSelector({ currentMode, onChange, disabled = false }: ModeSelectorProps) {
  const modes: ModeName[] = ['scalp', 'swing', 'position', 'ultra_fast'];
  const [isOpen, setIsOpen] = useState(false);

  const getModeIcon = (mode: ModeName) => {
    switch (mode) {
      case 'scalp':
        return <Zap className="w-4 h-4" />;
      case 'swing':
        return <TrendingUp className="w-4 h-4" />;
      case 'position':
        return <Target className="w-4 h-4" />;
      case 'ultra_fast':
        return <BarChart3 className="w-4 h-4" />;
    }
  };

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className={`
          flex items-center gap-2 px-4 py-2 bg-gray-700 rounded-lg border border-gray-600
          transition-colors
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'hover:bg-gray-600 cursor-pointer'}
        `}
      >
        {getModeIcon(currentMode)}
        <span className="text-white font-medium">{MODE_DISPLAY_NAMES[currentMode]}</span>
        <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-10"
            onClick={() => setIsOpen(false)}
          />

          {/* Dropdown */}
          <div className="absolute right-0 mt-2 w-64 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-20 py-2">
            {modes.map((mode) => (
              <button
                key={mode}
                type="button"
                onClick={() => {
                  onChange(mode);
                  setIsOpen(false);
                }}
                className={`
                  w-full flex items-center gap-3 px-4 py-3 text-left transition-colors
                  ${mode === currentMode
                    ? 'bg-purple-500/10 text-purple-400'
                    : 'text-gray-300 hover:bg-gray-700'}
                `}
              >
                <div className={mode === currentMode ? 'text-purple-400' : 'text-gray-500'}>
                  {getModeIcon(mode)}
                </div>
                <div className="flex-1">
                  <div className="font-medium">{MODE_DISPLAY_NAMES[mode]}</div>
                  <div className="text-xs text-gray-500">{MODE_DESCRIPTIONS[mode]}</div>
                </div>
                {mode === currentMode && (
                  <CheckCircle2 className="w-4 h-4 text-purple-400" />
                )}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

// ==================== Main Component ====================

export default function ModeStrategySettings({
  initialMode = 'scalp',
  onModeChange,
  disabled = false,
}: ModeStrategySettingsProps) {
  // State
  const [currentMode, setCurrentMode] = useState<ModeName>(initialMode);
  const [modeConfig, setModeConfig] = useState<ModeConfig | null>(null);
  const [activeStrategy, setActiveStrategy] = useState<StrategyName>('trend_following');

  // Loading states
  const [isLoadingMode, setIsLoadingMode] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isResetting, setIsResetting] = useState(false);
  const [isTogglingStrategy, setIsTogglingStrategy] = useState(false);

  // Messages
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Comparison state
  const [comparison, setComparison] = useState<StrategyComparisonResponse | null>(null);
  const [isComparing, setIsComparing] = useState(false);

  // Load mode config
  const loadModeConfig = useCallback(async (mode: ModeName) => {
    setIsLoadingMode(true);
    setError(null);

    try {
      const config = await modeStrategyApi.getModeStrategies(mode);
      setModeConfig(config);

      // Set active strategy to the first enabled one, or the default
      const strategies = Object.entries(config.strategies) as [StrategyName, ModeStrategyConfig][];
      const enabledStrategy = strategies.find(([, s]) => s.enabled);
      if (enabledStrategy) {
        setActiveStrategy(enabledStrategy[0]);
      } else {
        setActiveStrategy(config.default_strategy || 'trend_following');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load mode configuration');
    } finally {
      setIsLoadingMode(false);
    }
  }, []);

  // Load initial config
  useEffect(() => {
    loadModeConfig(currentMode);
  }, [currentMode, loadModeConfig]);

  // Handle mode change
  const handleModeChange = (mode: ModeName) => {
    setCurrentMode(mode);
    onModeChange?.(mode);
  };

  // Handle strategy toggle
  const handleStrategyToggle = async (strategy: StrategyName, enabled: boolean) => {
    if (!modeConfig || isTogglingStrategy) return;

    setIsTogglingStrategy(true);
    setError(null);

    try {
      await modeStrategyApi.toggleModeStrategy(currentMode, strategy, enabled);

      // Update local state
      setModeConfig({
        ...modeConfig,
        strategies: {
          ...modeConfig.strategies,
          [strategy]: {
            ...modeConfig.strategies[strategy],
            enabled,
          },
        },
      });

      setSuccessMessage(`Strategy ${enabled ? 'enabled' : 'disabled'}`);
      setTimeout(() => setSuccessMessage(null), 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to toggle strategy');
    } finally {
      setIsTogglingStrategy(false);
    }
  };

  // Handle strategy settings update
  const handleStrategyUpdate = async (config: ModeStrategyConfig) => {
    if (!modeConfig || isSaving) return;

    setIsSaving(true);
    setError(null);

    try {
      const response = await modeStrategyApi.updateModeStrategy(currentMode, activeStrategy, config);

      // Transform response.data (ModeStrategyResponse) to ModeStrategyConfig
      const updatedConfig: ModeStrategyConfig = {
        enabled: response.data.enabled,
        priority: response.data.priority,
        supported_regimes: response.data.supported_regimes,
        leverage: response.data.settings.leverage,
        max_positions: response.data.settings.max_positions,
        base_size_usd: response.data.settings.base_size_usd,
        timeframe: response.data.settings.timeframe,
        sltp: response.data.settings.sltp,
        confidence: response.data.settings.confidence,
        entry_conditions: response.data.settings.entry_conditions || {},
        exit_conditions: response.data.settings.exit_conditions,
        scoring: response.data.settings.scoring,
      };

      // Update local state
      setModeConfig({
        ...modeConfig,
        strategies: {
          ...modeConfig.strategies,
          [activeStrategy]: updatedConfig,
        },
      });

      setSuccessMessage('Settings saved successfully');
      setTimeout(() => setSuccessMessage(null), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  // Handle reset to defaults
  const handleReset = async () => {
    if (!modeConfig || isResetting) return;

    setIsResetting(true);
    setError(null);

    try {
      const response = await modeStrategyApi.resetModeStrategy(currentMode, activeStrategy);

      // Transform response.data (ModeStrategyResponse) to ModeStrategyConfig
      const resetConfig: ModeStrategyConfig = {
        enabled: response.data.enabled,
        priority: response.data.priority,
        supported_regimes: response.data.supported_regimes,
        leverage: response.data.settings.leverage,
        max_positions: response.data.settings.max_positions,
        base_size_usd: response.data.settings.base_size_usd,
        timeframe: response.data.settings.timeframe,
        sltp: response.data.settings.sltp,
        confidence: response.data.settings.confidence,
        entry_conditions: response.data.settings.entry_conditions || {},
        exit_conditions: response.data.settings.exit_conditions,
        scoring: response.data.settings.scoring,
      };

      // Update local state
      setModeConfig({
        ...modeConfig,
        strategies: {
          ...modeConfig.strategies,
          [activeStrategy]: resetConfig,
        },
      });

      setSuccessMessage('Settings reset to defaults');
      setTimeout(() => setSuccessMessage(null), 3000);
      // Clear comparison after reset
      setComparison(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to reset settings');
    } finally {
      setIsResetting(false);
    }
  };

  // Handle compare with defaults
  const handleCompare = async () => {
    if (!modeConfig || isComparing) return;

    setIsComparing(true);
    setError(null);

    try {
      const result = await modeStrategyApi.compareModeStrategy(currentMode, activeStrategy);
      setComparison(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to compare settings');
      setComparison(null);
    } finally {
      setIsComparing(false);
    }
  };

  // Clear comparison when changing strategy
  useEffect(() => {
    setComparison(null);
  }, [activeStrategy, currentMode]);

  const isDisabled = disabled || isLoadingMode || isSaving || isResetting;
  const strategyEntries = modeConfig
    ? (Object.entries(modeConfig.strategies) as [StrategyName, ModeStrategyConfig][])
    : [];

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-gray-700 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Settings2 className="w-5 h-5 text-purple-400" />
          <h3 className="font-semibold text-white">Mode Strategy Settings</h3>
        </div>
        <ModeSelector
          currentMode={currentMode}
          onChange={handleModeChange}
          disabled={isDisabled}
        />
      </div>

      {/* Content */}
      <div className="p-4 space-y-4">
        {/* Loading state */}
        {isLoadingMode && (
          <div className="flex items-center justify-center py-12">
            <Loader2 className="w-8 h-8 animate-spin text-purple-400" />
            <span className="ml-3 text-gray-400">Loading mode configuration...</span>
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

        {!isLoadingMode && modeConfig && (
          <>
            {/* Strategy Tabs */}
            <div>
              <label className="block text-sm font-medium text-gray-400 mb-2">Strategies</label>
              <div className="flex flex-wrap gap-2">
                {strategyEntries.map(([strategyName, config]) => (
                  <StrategyTab
                    key={strategyName}
                    strategyName={strategyName}
                    config={config}
                    isActive={activeStrategy === strategyName}
                    onClick={() => setActiveStrategy(strategyName)}
                    onToggle={(enabled) => handleStrategyToggle(strategyName, enabled)}
                    disabled={isDisabled || isTogglingStrategy}
                  />
                ))}
              </div>
            </div>

            {/* Strategy Description */}
            <div className="p-3 bg-gray-700/30 rounded-lg">
              <p className="text-sm text-gray-400">
                {STRATEGY_DESCRIPTIONS[activeStrategy]}
              </p>
            </div>

            {/* Compare with Defaults */}
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={handleCompare}
                disabled={isComparing || isDisabled}
                className={`
                  flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg transition-colors
                  ${isComparing ? 'bg-gray-700 text-gray-500' : 'bg-gray-700 hover:bg-gray-600 text-gray-300'}
                  disabled:opacity-50 disabled:cursor-not-allowed
                `}
              >
                {isComparing ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Comparing...
                  </>
                ) : (
                  <>
                    <BarChart3 className="w-4 h-4" />
                    Compare with Defaults
                  </>
                )}
              </button>

              {/* Comparison Result Badge */}
              {comparison && (
                <div className={`
                  flex items-center gap-2 px-3 py-1.5 text-sm rounded-lg
                  ${comparison.all_match
                    ? 'bg-green-900/30 border border-green-500/30 text-green-400'
                    : 'bg-amber-900/30 border border-amber-500/30 text-amber-400'}
                `}>
                  {comparison.all_match ? (
                    <>
                      <CheckCircle2 className="w-4 h-4" />
                      Settings match defaults
                    </>
                  ) : (
                    <>
                      <AlertTriangle className="w-4 h-4" />
                      {comparison.total_fields - comparison.matching_fields} of {comparison.total_fields} settings differ from defaults
                    </>
                  )}
                </div>
              )}
            </div>

            {/* Detailed Differences */}
            {comparison && !comparison.all_match && comparison.differences.length > 0 && (
              <div className="p-3 bg-gray-700/30 rounded-lg space-y-2">
                <h4 className="text-sm font-medium text-gray-300 mb-2">Modified Settings:</h4>
                <div className="max-h-48 overflow-y-auto space-y-1">
                  {comparison.differences.slice(0, 10).map((diff, idx) => (
                    <div key={idx} className="flex items-center justify-between text-xs py-1 px-2 bg-gray-800/50 rounded">
                      <span className="text-gray-400 font-mono">{diff.path}</span>
                      <div className="flex items-center gap-2">
                        <span className="text-amber-400">{JSON.stringify(diff.current)}</span>
                        <span className="text-gray-500">→</span>
                        <span className="text-green-400">{JSON.stringify(diff.default)}</span>
                      </div>
                    </div>
                  ))}
                  {comparison.differences.length > 10 && (
                    <p className="text-xs text-gray-500 text-center py-1">
                      ... and {comparison.differences.length - 10} more
                    </p>
                  )}
                </div>
              </div>
            )}

            {/* Strategy Settings Form */}
            <StrategySettingsForm
              strategyName={activeStrategy}
              config={modeConfig.strategies[activeStrategy]}
              onSave={handleStrategyUpdate}
              onReset={handleReset}
              isSaving={isSaving}
              isResetting={isResetting}
              disabled={isDisabled}
            />
          </>
        )}
      </div>
    </div>
  );
}
