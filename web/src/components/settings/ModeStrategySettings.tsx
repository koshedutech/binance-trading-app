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
  XCircle,
  Loader2,
  ChevronDown,
  ChevronUp,
  ChevronRight,
  Power,
  PowerOff,
  RefreshCw,
  Wallet,
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
import { apiService } from '../../services/api';
import { wsService } from '../../services/websocket';
import StrategySettingsForm from './StrategySettingsForm';
// Story 11.43-11.46: Import sub-strategies hook and types for Ravindra Volume Imbalance
import { useSubStrategies, useUpdateSubStrategy } from '../../hooks/useStrategyHierarchy';
import type { SubStrategySettings, VolumeImbalanceSettings, BudgetAllocationConfig } from '../../types/strategyHierarchy';

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
  // Story 11.41: Expanded to include all 18 sections
  const handleStrategyUpdate = async (config: ModeStrategyConfig) => {
    if (!modeConfig || isSaving) return;

    setIsSaving(true);
    setError(null);

    try {
      const response = await modeStrategyApi.updateModeStrategy(currentMode, activeStrategy, config);

      // Transform response.data (ModeStrategyResponse) to ModeStrategyConfig
      // Story 11.41: Include all 18 sections
      const updatedConfig: ModeStrategyConfig = {
        enabled: response.data.enabled,
        priority: response.data.priority,
        supported_regimes: response.data.supported_regimes,
        // Legacy position sizing fields
        leverage: response.data.settings.leverage,
        max_positions: response.data.settings.max_positions,
        base_size_usd: response.data.settings.base_size_usd,
        // Required sections
        timeframe: response.data.settings.timeframe,
        sltp: response.data.settings.sltp,
        confidence: response.data.settings.confidence,
        entry_conditions: response.data.settings.entry_conditions || {},
        exit_conditions: response.data.settings.exit_conditions,
        scoring: response.data.settings.scoring,
        // Story 11.41: All 18 sections (optional)
        position_sizing: response.data.settings.position_sizing,
        mtf: response.data.settings.mtf,
        circuit_breaker: response.data.settings.circuit_breaker,
        hedge: response.data.settings.hedge,
        averaging: response.data.settings.averaging,
        stale_release: response.data.settings.stale_release,
        position_optimization: response.data.settings.position_optimization,
        funding_rate: response.data.settings.funding_rate,
        risk: response.data.settings.risk,
        trend_divergence: response.data.settings.trend_divergence,
        dynamic_ai_exit: response.data.settings.dynamic_ai_exit,
        early_warning: response.data.settings.early_warning,
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
  // Story 11.41: Expanded to include all 18 sections
  const handleReset = async () => {
    if (!modeConfig || isResetting) return;

    setIsResetting(true);
    setError(null);

    try {
      const response = await modeStrategyApi.resetModeStrategy(currentMode, activeStrategy);

      // Transform response.data (ModeStrategyResponse) to ModeStrategyConfig
      // Story 11.41: Include all 18 sections
      const resetConfig: ModeStrategyConfig = {
        enabled: response.data.enabled,
        priority: response.data.priority,
        supported_regimes: response.data.supported_regimes,
        // Legacy position sizing fields
        leverage: response.data.settings.leverage,
        max_positions: response.data.settings.max_positions,
        base_size_usd: response.data.settings.base_size_usd,
        // Required sections
        timeframe: response.data.settings.timeframe,
        sltp: response.data.settings.sltp,
        confidence: response.data.settings.confidence,
        entry_conditions: response.data.settings.entry_conditions || {},
        exit_conditions: response.data.settings.exit_conditions,
        scoring: response.data.settings.scoring,
        // Story 11.41: All 18 sections (optional)
        position_sizing: response.data.settings.position_sizing,
        mtf: response.data.settings.mtf,
        circuit_breaker: response.data.settings.circuit_breaker,
        hedge: response.data.settings.hedge,
        averaging: response.data.settings.averaging,
        stale_release: response.data.settings.stale_release,
        position_optimization: response.data.settings.position_optimization,
        funding_rate: response.data.settings.funding_rate,
        risk: response.data.settings.risk,
        trend_divergence: response.data.settings.trend_divergence,
        dynamic_ai_exit: response.data.settings.dynamic_ai_exit,
        early_warning: response.data.settings.early_warning,
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

  // Handle sub-strategy toggle
  const handleSubStrategyToggle = useCallback((subStrategyId: string, enabled: boolean) => {
    // Log the toggle for now - can be enhanced with success message
    console.log(`Sub-strategy ${subStrategyId} ${enabled ? 'enabled' : 'disabled'}`);
    setSuccessMessage(`Sub-strategy ${enabled ? 'enabled' : 'disabled'}`);
    setTimeout(() => setSuccessMessage(null), 2000);
  }, []);

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

            {/* Story 11.43-11.46: Sub-Strategies Section for Breakout - BEFORE Strategy Settings Form */}
            {activeStrategy === 'breakout' && (
              <SubStrategiesSection mode={currentMode} onToggle={handleSubStrategyToggle} />
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

// Helper function for formatting values in comparison tables
function formatValue(value: any): string {
  if (value === null || value === undefined) return 'N/A';
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'number') {
    if (Number.isInteger(value)) return value.toLocaleString();
    return value.toFixed(4).replace(/\.?0+$/, '');
  }
  if (Array.isArray(value)) return value.length > 0 ? value.join(', ') : '[]';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

// Story 11.43-11.46: Sub-Strategies Section Component
// Displays sub-strategies (e.g., Ravindra Volume Imbalance) inside the Breakout strategy
// Enhanced to follow CollapsibleStrategySection pattern with field comparisons and functional toggle
function SubStrategiesSection({
  mode,
  onToggle,
}: {
  mode: string;
  onToggle?: (subStrategyId: string, enabled: boolean) => void;
}) {
  const { subStrategies, strategyGroupEnabled, isLoading, error, refresh } = useSubStrategies(mode, 'breakout');
  const [expandedSections, setExpandedSections] = useState<Set<string>>(new Set());

  const toggleSection = (key: string) => {
    setExpandedSections(prev => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  if (isLoading) {
    return (
      <div className="border border-gray-700 rounded-lg overflow-hidden mb-4">
        <div className="p-3 bg-purple-900/20 flex items-center gap-2">
          <Loader2 className="w-4 h-4 text-purple-400 animate-spin" />
          <span className="text-sm text-purple-300">Loading sub-strategies...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="border border-red-500/30 rounded-lg overflow-hidden mb-4">
        <div className="p-3 bg-red-900/20 flex items-center gap-2">
          <AlertTriangle className="w-4 h-4 text-red-400" />
          <span className="text-sm text-red-400">Failed to load sub-strategies: {error}</span>
        </div>
      </div>
    );
  }

  if (subStrategies.length === 0) {
    return null; // No sub-strategies to show
  }

  return (
    <div className="space-y-4">
      {/* Sub-Strategies Section with clear header */}
      <div className={`border rounded-lg overflow-hidden ${
        strategyGroupEnabled
          ? 'border-purple-500/30 bg-purple-900/10'
          : 'border-gray-600/50 bg-gray-800/30 opacity-60'
      }`}>
        <div className={`flex items-center gap-2 px-4 py-3 border-b ${
          strategyGroupEnabled
            ? 'bg-purple-900/20 border-purple-500/30'
            : 'bg-gray-700/30 border-gray-600/50'
        }`}>
          <Zap className={`w-5 h-5 ${strategyGroupEnabled ? 'text-purple-400' : 'text-gray-500'}`} />
          <span className={`text-sm font-semibold ${strategyGroupEnabled ? 'text-purple-300' : 'text-gray-400'}`}>
            Sub-Strategies
          </span>
          <span className={`px-2 py-0.5 text-xs rounded ${
            strategyGroupEnabled
              ? 'bg-purple-500/20 text-purple-300'
              : 'bg-gray-600/30 text-gray-500'
          }`}>
            {subStrategies.length}
          </span>
          {/* Disabled indicator when parent strategy group is OFF */}
          {!strategyGroupEnabled && (
            <span className="ml-auto flex items-center gap-1.5 px-2 py-0.5 bg-amber-500/20 text-amber-400 text-xs rounded">
              <AlertTriangle className="w-3 h-3" />
              Breakout strategy is OFF
            </span>
          )}
        </div>

        {/* Warning banner when parent is disabled */}
        {!strategyGroupEnabled && (
          <div className="px-4 py-2 bg-amber-900/20 border-b border-amber-500/20">
            <p className="text-xs text-amber-400/90">
              Enable the Breakout strategy above to use these sub-strategies.
              Your settings will be preserved.
            </p>
          </div>
        )}

        <div className="p-3 space-y-2">
          {subStrategies.map((subStrategy) => (
            <SubStrategyCollapsibleSection
              key={subStrategy.sub_strategy}
              subStrategy={subStrategy}
              mode={mode}
              expanded={expandedSections.has(`sub_strategy_${subStrategy.sub_strategy}`)}
              onToggle={() => toggleSection(`sub_strategy_${subStrategy.sub_strategy}`)}
              onEnabledChange={onToggle}
              onRefresh={refresh}
              parentDisabled={!strategyGroupEnabled}
            />
          ))}
        </div>
      </div>

      {/* Divider between Sub-Strategies and Main Strategy Settings */}
      <div className="flex items-center gap-3 py-2">
        <div className="flex-1 border-t border-gray-600"></div>
        <span className="text-xs text-gray-500 font-medium uppercase tracking-wider">Breakout Main Settings</span>
        <div className="flex-1 border-t border-gray-600"></div>
      </div>
    </div>
  );
}

// Story 11.43-11.46: Sub-Strategy Collapsible Section Component
// Updated to use proper form inputs instead of comparison table
function SubStrategyCollapsibleSection({
  subStrategy,
  mode,
  expanded,
  onToggle,
  onEnabledChange,
  onRefresh,
  parentDisabled = false,
}: {
  subStrategy: SubStrategySettings;
  mode: string;
  expanded: boolean;
  onToggle: () => void;
  onEnabledChange?: (subStrategyId: string, enabled: boolean) => void;
  onRefresh?: () => Promise<void>;
  /** Whether the parent strategy group is disabled */
  parentDisabled?: boolean;
}) {
  // Type guard for Volume Imbalance settings
  // Use sub_strategy field for identification (not id which is the DB UUID)
  const subStrategyName = subStrategy.sub_strategy;
  const isVolumeImbalance = subStrategyName === 'ravindra_volume_imbalance';
  const settings = subStrategy.settings as VolumeImbalanceSettings | undefined;
  const [isToggling, setIsToggling] = useState(false);
  const [isResetting, setIsResetting] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [toggleError, setToggleError] = useState<string | null>(null);
  const [hasChanges, setHasChanges] = useState(false);

  // Available capital state - fetched from backend
  const [equityData, setEquityData] = useState<{
    effective_budget: number;
    capital_in_use: number;
    available: number;
    active_chains: number;
  } | null>(null);

  // Reusable equity fetch function
  const fetchEquity = useCallback(async () => {
    try {
      const response = await apiService.get<{
        success: boolean;
        effective_budget: number;
        capital_in_use: number;
        available: number;
        active_chains: number;
      }>(`/futures/sub-strategies/${mode}/breakout/${subStrategyName}/equity`);
      setEquityData(response.data);
    } catch {
      // Non-fatal - just don't show available
    }
  }, [mode, subStrategyName]);

  // Fetch available capital when section is expanded
  useEffect(() => {
    if (!expanded || !isVolumeImbalance) return;
    fetchEquity();
  }, [expanded, isVolumeImbalance, fetchEquity]);

  // Real-time equity updates via WebSocket
  useEffect(() => {
    if (!expanded || !isVolumeImbalance) return;

    const handleEquityUpdate = () => {
      fetchEquity();
    };

    // Listen to events that change equity
    wsService.subscribe('EQUITY_UPDATE', handleEquityUpdate);
    wsService.subscribe('CHAIN_LIFECYCLE_UPDATE', handleEquityUpdate);
    wsService.subscribe('POSITION_CREATED', handleEquityUpdate);

    return () => {
      wsService.unsubscribe('EQUITY_UPDATE', handleEquityUpdate);
      wsService.unsubscribe('CHAIN_LIFECYCLE_UPDATE', handleEquityUpdate);
      wsService.unsubscribe('POSITION_CREATED', handleEquityUpdate);
    };
  }, [expanded, isVolumeImbalance, fetchEquity]);

  // Default settings based on Dec 2025 - Jan 2026 backtest: 51 trades, 47.1% WR, +1147% net return
  const defaultSettings: VolumeImbalanceSettings = {
    enabled: true,
    risk_reward: { risk: 1, reward: 4, min_ratio: 3 },
    llm_validation_enabled: false,
    trailing_stop: {
      enabled: true,
      activation_profit_pct: 2.0,  // Activate at 2:1 R:R
      initial_trail_pct: 0.0,      // Move SL to breakeven (0R profit locked)
      milestones: [
        { trigger_profit_pct: 2.0, trail_distance_pct: 0.0, label: 'BE' },   // At 2:1 → breakeven
        { trigger_profit_pct: 3.0, trail_distance_pct: 1.0, label: '+1R' },  // At 3:1 → lock 1:1
      ],
    },
    pattern_detection: {
      // Legacy fields (kept for backwards compatibility)
      min_volume_ratio: 3.0,
      breakout_confirmation_candles: 1,
      max_pattern_age_mins: 60,
      require_htf_confirmation: false,
      htf_timeframe: '15m',
      // Direction setting: "long" (GREEN candles), "short" (RED candles), or "both"
      direction: 'long' as const,
      // Backtested parameters (3m timeframe) - 2 step pattern: Volume Spike → Breakout
      reference_lookback_candles: 5,
      volume_spike_threshold: 3.0,
      breakout_volume_surge: 1.0,
      entry_volume_vs_reference: 1.0,
      max_sl_percent: 1.5,
    },
    max_concurrent_patterns: 5,
    priority: 1,
    budget_allocation: {
      assigned_budget_usd: 100,
      max_concurrent_trades: 1,
      position_sizing: 'all_in',
      use_incremental_equity: true,
    },
  };

  // Local state for form editing - initialize with defaults merged with settings
  const [localSettings, setLocalSettings] = useState<VolumeImbalanceSettings>(() => {
    if (settings) {
      return {
        ...defaultSettings,
        ...settings,
        risk_reward: { ...defaultSettings.risk_reward, ...(settings.risk_reward || {}) },
        trailing_stop: { ...defaultSettings.trailing_stop, ...(settings.trailing_stop || {}) },
        pattern_detection: { ...defaultSettings.pattern_detection, ...(settings.pattern_detection || {}) },
        budget_allocation: { ...defaultSettings.budget_allocation, ...(settings.budget_allocation || {}) },
      };
    }
    return defaultSettings;
  });

  // Update local settings when settings change
  React.useEffect(() => {
    if (settings) {
      setLocalSettings({
        ...defaultSettings,
        ...settings,
        risk_reward: { ...defaultSettings.risk_reward, ...(settings.risk_reward || {}) },
        trailing_stop: { ...defaultSettings.trailing_stop, ...(settings.trailing_stop || {}) },
        pattern_detection: { ...defaultSettings.pattern_detection, ...(settings.pattern_detection || {}) },
        budget_allocation: { ...defaultSettings.budget_allocation, ...(settings.budget_allocation || {}) },
      });
      setHasChanges(false);
    }
  }, [settings]);

  // Use the update hook for toggling enabled state
  // IMPORTANT: Pass sub_strategy name (not id) to the hook - API expects strategy name
  const { mutate: updateSubStrategy, isLoading: isUpdating } = useUpdateSubStrategy(
    mode,
    'breakout',
    subStrategyName
  );

  // Get display name from sub_strategy field (not id)
  const displayName = subStrategyName === 'ravindra_volume_imbalance'
    ? 'Ravindra Volume Imbalance'
    : subStrategyName.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());

  // Handle enable/disable toggle
  const handleToggleEnabled = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isToggling || isUpdating || isResetting || isSaving) return;

    const newEnabled = !subStrategy.enabled;
    setIsToggling(true);
    setToggleError(null);

    try {
      await updateSubStrategy({ enabled: newEnabled });
      // Notify parent and refresh data - use sub_strategy name
      onEnabledChange?.(subStrategyName, newEnabled);
      await onRefresh?.();
    } catch (err) {
      setToggleError(err instanceof Error ? err.message : 'Failed to toggle sub-strategy');
    } finally {
      setIsToggling(false);
    }
  };

  // Handle reset to defaults
  const handleReset = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isResetting || isUpdating || isToggling || isSaving) return;

    setIsResetting(true);
    setToggleError(null);

    try {
      // Reset all settings to defaults - send the complete settings object
      await updateSubStrategy({
        enabled: defaultSettings.enabled,
        priority: defaultSettings.priority,
        settings: defaultSettings,
      });
      // Update local state to defaults
      setLocalSettings(defaultSettings);
      // Refresh data after reset
      await onRefresh?.();
      setHasChanges(false);
    } catch (err) {
      setToggleError(err instanceof Error ? err.message : 'Failed to reset sub-strategy');
    } finally {
      setIsResetting(false);
    }
  };

  // Handle save
  const handleSave = async () => {
    if (!localSettings || isSaving || isUpdating || isToggling || isResetting) return;

    setIsSaving(true);
    setToggleError(null);

    try {
      await updateSubStrategy({
        enabled: subStrategy.enabled,
        priority: localSettings.priority,
        settings: {
          priority: localSettings.priority,
          llm_validation_enabled: localSettings.llm_validation_enabled,
          max_concurrent_patterns: localSettings.max_concurrent_patterns,
          risk_reward: localSettings.risk_reward,
          trailing_stop: localSettings.trailing_stop,
          pattern_detection: localSettings.pattern_detection,
          budget_allocation: localSettings.budget_allocation,
        },
      });
      await onRefresh?.();
      setHasChanges(false);
    } catch (err) {
      setToggleError(err instanceof Error ? err.message : 'Failed to save sub-strategy settings');
    } finally {
      setIsSaving(false);
    }
  };

  // Update local settings helper
  const updateLocalSettings = (updates: Partial<VolumeImbalanceSettings>) => {
    if (!localSettings) return;
    setLocalSettings({ ...localSettings, ...updates });
    setHasChanges(true);
  };

  // Create field comparison list for header status
  const fieldComparisons: { path: string; current: any; default: any; match: boolean }[] = [];

  if (settings) {
    fieldComparisons.push({
      path: 'enabled',
      current: subStrategy.enabled,
      default: defaultSettings.enabled,
      match: subStrategy.enabled === defaultSettings.enabled,
    });
    fieldComparisons.push({
      path: 'priority',
      current: settings.priority,
      default: defaultSettings.priority,
      match: settings.priority === defaultSettings.priority,
    });
    if (settings.risk_reward) {
      fieldComparisons.push({
        path: 'risk_reward.risk',
        current: settings.risk_reward.risk,
        default: defaultSettings.risk_reward.risk,
        match: settings.risk_reward.risk === defaultSettings.risk_reward.risk,
      });
    }
    if (settings.pattern_detection) {
      fieldComparisons.push({
        path: 'pattern_detection.min_volume_ratio',
        current: settings.pattern_detection.min_volume_ratio,
        default: defaultSettings.pattern_detection.min_volume_ratio,
        match: settings.pattern_detection.min_volume_ratio === defaultSettings.pattern_detection.min_volume_ratio,
      });
    }
    if (settings.trailing_stop) {
      fieldComparisons.push({
        path: 'trailing_stop.enabled',
        current: settings.trailing_stop.enabled,
        default: defaultSettings.trailing_stop.enabled,
        match: settings.trailing_stop.enabled === defaultSettings.trailing_stop.enabled,
      });
    }
  }

  const matchingFields = fieldComparisons.filter(f => f.match).length;
  const totalFields = fieldComparisons.length;
  const differentFields = totalFields - matchingFields;
  const allMatch = differentFields === 0;

  const isDisabled = isToggling || isUpdating || isResetting || isSaving || parentDisabled;

  // HTF Timeframe options
  const htfTimeframeOptions = [
    { value: '15m', label: '15 min' },
    { value: '1h', label: '1 hour' },
    { value: '4h', label: '4 hours' },
  ];

  return (
    <div className={`border rounded-lg overflow-hidden ${parentDisabled ? 'border-gray-600/50' : 'border-gray-700'}`}>
      {/* Section Header - follows CollapsibleStrategySection pattern */}
      <button
        type="button"
        onClick={onToggle}
        className={`w-full flex items-center justify-between px-3 py-2 transition-colors ${
          parentDisabled
            ? 'bg-gray-700/20 hover:bg-gray-700/30'
            : allMatch
              ? 'bg-green-900/20 hover:bg-green-900/30'
              : 'bg-orange-900/20 hover:bg-orange-900/30'
        }`}
      >
        <div className="flex items-center gap-2">
          <span className={parentDisabled ? 'text-gray-500' : 'text-purple-400'}>
            <Zap className="w-4 h-4" />
          </span>
          <span className={`font-medium text-sm ${parentDisabled ? 'text-gray-400' : 'text-gray-200'}`}>{displayName}</span>
          {/* Enable/Disable Toggle - Shows actual state but disabled when parent is OFF */}
          <div
            onClick={parentDisabled ? undefined : handleToggleEnabled}
            className={`
              flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium transition-colors
              ${parentDisabled
                ? 'bg-gray-600/30 text-gray-500 cursor-not-allowed'
                : subStrategy.enabled
                  ? 'bg-green-500/20 text-green-400 hover:bg-green-500/30 cursor-pointer'
                  : 'bg-gray-600/20 text-gray-500 hover:bg-gray-600/30 cursor-pointer'}
              ${(isToggling || isUpdating) ? 'opacity-50 cursor-wait' : ''}
            `}
            title={parentDisabled ? 'Enable Breakout strategy first' : undefined}
          >
            {(isToggling || isUpdating) ? (
              <Loader2 className="w-3 h-3 animate-spin" />
            ) : subStrategy.enabled ? (
              <Power className="w-3 h-3" />
            ) : (
              <PowerOff className="w-3 h-3" />
            )}
            {subStrategy.enabled ? 'ON' : 'OFF'}
          </div>
          {/* Show "Inactive" badge when parent is disabled but sub-strategy is ON */}
          {parentDisabled && subStrategy.enabled && (
            <span className="text-xs text-amber-500/80 italic">(inactive)</span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {/* Reset Button - only show when there are differences */}
          {!allMatch && (
            <div
              onClick={handleReset}
              className={`
                flex items-center gap-1 px-2 py-0.5 rounded text-xs font-medium transition-colors cursor-pointer
                bg-blue-600/20 text-blue-400 hover:bg-blue-600/30
                ${isResetting ? 'opacity-50 cursor-wait' : ''}
              `}
              title="Reset to defaults"
            >
              {isResetting ? (
                <Loader2 className="w-3 h-3 animate-spin" />
              ) : (
                <RefreshCw className="w-3 h-3" />
              )}
              Reset
            </div>
          )}
          {expanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}
        </div>
      </button>

      {/* Toggle Error */}
      {toggleError && (
        <div className="px-3 py-2 bg-red-900/20 border-t border-red-500/30 flex items-center gap-2">
          <AlertTriangle className="w-3 h-3 text-red-400" />
          <span className="text-xs text-red-400">{toggleError}</span>
        </div>
      )}

      {/* Section Content - Editable Form */}
      {expanded && localSettings && (
        <div className="bg-gray-800/50 border-t border-gray-700 p-4 space-y-6">
          {isVolumeImbalance && (
            <div className="text-xs text-gray-400 italic border-b border-gray-700 pb-3">
              2-step pattern: Volume Spike → Breakout
            </div>
          )}

          {/* Basic Settings */}
          <div className="space-y-4">
            <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
              <Settings2 className="w-4 h-4 text-blue-400" />
              Basic Settings
            </h4>
            <div className="grid grid-cols-2 gap-4 pl-6">
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Priority</label>
                  <div className="flex items-center gap-1">
                    <input
                      type="number"
                      value={localSettings.priority}
                      onChange={(e) => updateLocalSettings({ priority: Number(e.target.value) })}
                      min={1}
                      max={10}
                      step={1}
                      disabled={isDisabled}
                      className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                    />
                  </div>
                </div>
                <p className="text-xs text-gray-500">Lower = higher priority</p>
              </div>
            </div>
          </div>

          {/* Budget Allocation - Per-strategy capital management */}
          <div className="space-y-4">
            <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
              <Wallet className="w-4 h-4 text-green-400" />
              Budget Allocation
            </h4>
            <p className="text-xs text-gray-500 pl-6 -mt-2">Per-strategy capital management for independent equity tracking</p>
            <div className="grid grid-cols-2 gap-4 pl-6">
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Assigned Budget</label>
                  <div className="flex items-center gap-1">
                    <input
                      type="number"
                      value={localSettings.budget_allocation?.assigned_budget_usd || 100}
                      onChange={(e) => updateLocalSettings({
                        budget_allocation: {
                          ...(localSettings.budget_allocation || defaultSettings.budget_allocation!),
                          assigned_budget_usd: Number(e.target.value)
                        }
                      })}
                      min={10}
                      max={100000}
                      step={10}
                      disabled={isDisabled}
                      className="w-24 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                    />
                    <span className="text-xs text-gray-500 w-8">USD</span>
                  </div>
                </div>
                <p className="text-xs text-gray-500">Initial budget for this strategy</p>
              </div>
              {/* Current Equity - read-only display (only when incremental equity is on) */}
              {(localSettings.budget_allocation?.use_incremental_equity ?? true) && (
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Current Value</label>
                    <div className="flex items-center gap-1">
                      <span className={`text-sm font-mono font-semibold ${
                        (localSettings.budget_allocation?.current_equity ?? localSettings.budget_allocation?.assigned_budget_usd ?? 100) >=
                        (localSettings.budget_allocation?.assigned_budget_usd ?? 100)
                          ? 'text-green-400' : 'text-red-400'
                      }`}>
                        ${(localSettings.budget_allocation?.current_equity ?? localSettings.budget_allocation?.assigned_budget_usd ?? 100).toFixed(2)}
                      </span>
                      <span className="text-xs text-gray-500 w-8">USD</span>
                    </div>
                  </div>
                  <p className="text-xs text-gray-500">Running balance after trade P&L</p>
                </div>
              )}
              {/* Available Capital - shows effective budget minus capital in active trades */}
              {equityData && (
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Available</label>
                    <div className="flex items-center gap-1">
                      <span className={`text-sm font-mono font-semibold ${
                        equityData.available > 0 ? 'text-blue-400' : 'text-orange-400'
                      }`}>
                        ${equityData.available.toFixed(2)}
                      </span>
                      <span className="text-xs text-gray-500 w-8">USD</span>
                    </div>
                  </div>
                  <p className="text-xs text-gray-500">
                    {equityData.capital_in_use > 0
                      ? `$${equityData.capital_in_use.toFixed(2)} in ${equityData.active_chains} active trade${equityData.active_chains !== 1 ? 's' : ''}`
                      : 'No active trades'}
                  </p>
                </div>
              )}
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Max Concurrent Trades</label>
                  <input
                    type="number"
                    value={localSettings.budget_allocation?.max_concurrent_trades || 1}
                    onChange={(e) => updateLocalSettings({
                      budget_allocation: {
                        ...(localSettings.budget_allocation || defaultSettings.budget_allocation!),
                        max_concurrent_trades: Number(e.target.value)
                      }
                    })}
                    min={1}
                    max={10}
                    step={1}
                    disabled={isDisabled}
                    className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  />
                </div>
              </div>
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Position Sizing</label>
                  <select
                    value={localSettings.budget_allocation?.position_sizing || 'all_in'}
                    onChange={(e) => updateLocalSettings({
                      budget_allocation: {
                        ...(localSettings.budget_allocation || defaultSettings.budget_allocation!),
                        position_sizing: e.target.value
                      }
                    })}
                    disabled={isDisabled}
                    className="w-32 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  >
                    <option value="all_in">All In (100%)</option>
                    <option value="fixed_percent">Fixed Percent</option>
                    <option value="kelly">Kelly Criterion</option>
                  </select>
                </div>
              </div>
              <div className="flex items-start justify-between py-2">
                <div className="flex-1">
                  <label className="text-sm font-medium text-gray-300">Incremental Equity</label>
                  <p className="text-xs text-gray-500 mt-1">Profits compound into position sizing</p>
                </div>
                <button
                  type="button"
                  onClick={() => !isDisabled && updateLocalSettings({
                    budget_allocation: {
                      ...(localSettings.budget_allocation || defaultSettings.budget_allocation!),
                      use_incremental_equity: !(localSettings.budget_allocation?.use_incremental_equity ?? true)
                    }
                  })}
                  disabled={isDisabled}
                  className={`
                    relative inline-flex h-6 w-11 items-center rounded-full transition-colors
                    ${(localSettings.budget_allocation?.use_incremental_equity ?? true) ? 'bg-purple-600' : 'bg-gray-600'}
                    ${isDisabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
                  `}
                >
                  <span
                    className={`
                      inline-block h-4 w-4 transform rounded-full bg-white transition-transform
                      ${(localSettings.budget_allocation?.use_incremental_equity ?? true) ? 'translate-x-6' : 'translate-x-1'}
                    `}
                  />
                </button>
              </div>
            </div>
          </div>

          {/* Risk/Reward */}
          <div className="space-y-4">
            <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
              <Target className="w-4 h-4 text-green-400" />
              Risk/Reward
            </h4>
            <div className="grid grid-cols-3 gap-4 pl-6">
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Risk</label>
                  <input
                    type="number"
                    value={localSettings.risk_reward.risk}
                    onChange={(e) => updateLocalSettings({
                      risk_reward: { ...localSettings.risk_reward, risk: Number(e.target.value) }
                    })}
                    min={0.1}
                    max={10}
                    step={0.1}
                    disabled={isDisabled}
                    className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  />
                </div>
              </div>
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Reward</label>
                  <input
                    type="number"
                    value={localSettings.risk_reward.reward}
                    onChange={(e) => updateLocalSettings({
                      risk_reward: { ...localSettings.risk_reward, reward: Number(e.target.value) }
                    })}
                    min={0.1}
                    max={20}
                    step={0.1}
                    disabled={isDisabled}
                    className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  />
                </div>
              </div>
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Min Ratio</label>
                  <input
                    type="number"
                    value={localSettings.risk_reward.min_ratio}
                    onChange={(e) => updateLocalSettings({
                      risk_reward: { ...localSettings.risk_reward, min_ratio: Number(e.target.value) }
                    })}
                    min={1}
                    max={10}
                    step={0.5}
                    disabled={isDisabled}
                    className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Pattern Detection - Backtested Parameters (3m timeframe) */}
          <div className="space-y-4">
            <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
              <BarChart3 className="w-4 h-4 text-cyan-400" />
              Pattern Detection
            </h4>
            <p className="text-xs text-gray-500 pl-6 -mt-2">Backtested Dec 2025 - Jan 2026: 51 trades, 47.1% WR, +1147% return</p>

            {/* Trade Direction Selection */}
            <div className="pl-6">
              <span className="text-xs text-purple-400 font-medium">Trade Direction</span>
              <div className="mt-2">
                <div className="flex items-center justify-between">
                  <div>
                    <label className="text-sm font-medium text-gray-300">Direction</label>
                    <p className="text-xs text-gray-500 mt-0.5">
                      {localSettings.pattern_detection.direction === 'long' && 'Only GREEN (bullish) candles → Look for upward breakout'}
                      {localSettings.pattern_detection.direction === 'short' && 'Only RED (bearish) candles → Look for downward breakout'}
                      {localSettings.pattern_detection.direction === 'both' && 'Any candle color → Direction determined by candle'}
                      {!localSettings.pattern_detection.direction && 'Only GREEN (bullish) candles → Look for upward breakout'}
                    </p>
                  </div>
                  <select
                    value={localSettings.pattern_detection.direction || 'long'}
                    onChange={(e) => updateLocalSettings({
                      pattern_detection: { ...localSettings.pattern_detection, direction: e.target.value as 'long' | 'short' | 'both' }
                    })}
                    disabled={isDisabled}
                    className="w-32 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  >
                    <option value="long">Long Only</option>
                    <option value="short">Short Only</option>
                    <option value="both">Both</option>
                  </select>
                </div>
              </div>
            </div>

            {/* Reference Candle Selection */}
            <div className="pl-6 pt-3 border-t border-gray-700">
              <span className="text-xs text-purple-400 font-medium">Reference Candle Selection</span>
              <div className="grid grid-cols-2 gap-4 mt-2">
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Lookback Candles</label>
                    <input
                      type="number"
                      value={localSettings.pattern_detection.reference_lookback_candles || 5}
                      onChange={(e) => updateLocalSettings({
                        pattern_detection: { ...localSettings.pattern_detection, reference_lookback_candles: Number(e.target.value) }
                      })}
                      min={1}
                      max={50}
                      step={1}
                      disabled={isDisabled}
                      className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                    />
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Volume Spike Threshold</label>
                    <div className="flex items-center gap-1">
                      <input
                        type="number"
                        value={localSettings.pattern_detection.volume_spike_threshold || 3.0}
                        onChange={(e) => updateLocalSettings({
                          pattern_detection: { ...localSettings.pattern_detection, volume_spike_threshold: Number(e.target.value) }
                        })}
                        min={1}
                        max={10}
                        step={0.1}
                        disabled={isDisabled}
                        className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                      />
                      <span className="text-xs text-gray-500 w-4">x</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* Breakout Parameters */}
            <div className="pl-6 pt-3 border-t border-gray-700">
              <span className="text-xs text-purple-400 font-medium">Breakout Confirmation</span>
              <div className="grid grid-cols-2 gap-4 mt-2">
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Volume Surge</label>
                    <div className="flex items-center gap-1">
                      <input
                        type="number"
                        value={localSettings.pattern_detection.breakout_volume_surge || 1.0}
                        onChange={(e) => updateLocalSettings({
                          pattern_detection: { ...localSettings.pattern_detection, breakout_volume_surge: Number(e.target.value) }
                        })}
                        min={0.5}
                        max={5}
                        step={0.1}
                        disabled={isDisabled}
                        className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                      />
                      <span className="text-xs text-gray-500 w-4">x</span>
                    </div>
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Entry Vol vs Ref</label>
                    <div className="flex items-center gap-1">
                      <input
                        type="number"
                        value={localSettings.pattern_detection.entry_volume_vs_reference || 1.0}
                        onChange={(e) => updateLocalSettings({
                          pattern_detection: { ...localSettings.pattern_detection, entry_volume_vs_reference: Number(e.target.value) }
                        })}
                        min={0.5}
                        max={5}
                        step={0.1}
                        disabled={isDisabled}
                        className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                      />
                      <span className="text-xs text-gray-500 w-4">x</span>
                    </div>
                  </div>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Confirm Candles</label>
                    <input
                      type="number"
                      value={localSettings.pattern_detection.breakout_confirmation_candles}
                      onChange={(e) => updateLocalSettings({
                        pattern_detection: { ...localSettings.pattern_detection, breakout_confirmation_candles: Number(e.target.value) }
                      })}
                      min={1}
                      max={10}
                      step={1}
                      disabled={isDisabled}
                      className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                    />
                  </div>
                </div>
              </div>
            </div>

            {/* Risk Management */}
            <div className="pl-6 pt-3 border-t border-gray-700">
              <span className="text-xs text-purple-400 font-medium">Risk Management</span>
              <div className="grid grid-cols-2 gap-4 mt-2">
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Max Stop Loss</label>
                    <div className="flex items-center gap-1">
                      <input
                        type="number"
                        value={localSettings.pattern_detection.max_sl_percent || 1.5}
                        onChange={(e) => updateLocalSettings({
                          pattern_detection: { ...localSettings.pattern_detection, max_sl_percent: Number(e.target.value) }
                        })}
                        min={0.5}
                        max={5}
                        step={0.1}
                        disabled={isDisabled}
                        className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                      />
                      <span className="text-xs text-gray-500 w-4">%</span>
                    </div>
                  </div>
                  <p className="text-xs text-gray-500">Caps risk per trade</p>
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">Max Pattern Age</label>
                    <div className="flex items-center gap-1">
                      <input
                        type="number"
                        value={localSettings.pattern_detection.max_pattern_age_mins}
                        onChange={(e) => updateLocalSettings({
                          pattern_detection: { ...localSettings.pattern_detection, max_pattern_age_mins: Number(e.target.value) }
                        })}
                        min={5}
                        max={480}
                        step={5}
                        disabled={isDisabled}
                        className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                      />
                      <span className="text-xs text-gray-500 w-4">m</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            {/* HTF Confirmation */}
            <div className="pl-6 pt-3 border-t border-gray-700">
              <div className="flex items-start justify-between py-2">
                <div className="flex-1">
                  <label className="text-sm font-medium text-gray-300">Require HTF Confirmation</label>
                  <p className="text-xs text-gray-500 mt-1">Validate pattern with higher timeframe</p>
                </div>
                <button
                  type="button"
                  onClick={() => !isDisabled && updateLocalSettings({
                    pattern_detection: { ...localSettings.pattern_detection, require_htf_confirmation: !localSettings.pattern_detection.require_htf_confirmation }
                  })}
                  disabled={isDisabled}
                  className={`
                    relative inline-flex h-6 w-11 items-center rounded-full transition-colors
                    ${localSettings.pattern_detection.require_htf_confirmation ? 'bg-purple-600' : 'bg-gray-600'}
                    ${isDisabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
                  `}
                >
                  <span
                    className={`
                      inline-block h-4 w-4 transform rounded-full bg-white transition-transform
                      ${localSettings.pattern_detection.require_htf_confirmation ? 'translate-x-6' : 'translate-x-1'}
                    `}
                  />
                </button>
              </div>
              {localSettings.pattern_detection.require_htf_confirmation && (
                <div className="space-y-1 mt-2">
                  <div className="flex items-center justify-between">
                    <label className="text-sm font-medium text-gray-300">HTF Timeframe</label>
                    <select
                      value={localSettings.pattern_detection.htf_timeframe}
                      onChange={(e) => updateLocalSettings({
                        pattern_detection: { ...localSettings.pattern_detection, htf_timeframe: e.target.value }
                      })}
                      disabled={isDisabled}
                      className="w-24 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                    >
                      {htfTimeframeOptions.map((opt) => (
                        <option key={opt.value} value={opt.value}>
                          {opt.label}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Trailing Stop */}
          <div className="space-y-4">
            <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
              <TrendingUp className="w-4 h-4 text-yellow-400" />
              Trailing Stop
            </h4>
            <div className="pl-6 space-y-4">
              <div className="flex items-start justify-between py-2">
                <div className="flex-1">
                  <label className="text-sm font-medium text-gray-300">Enable Trailing Stop</label>
                  <p className="text-xs text-gray-500 mt-1">Lock in profits as price moves favorably</p>
                </div>
                <button
                  type="button"
                  onClick={() => !isDisabled && updateLocalSettings({
                    trailing_stop: { ...localSettings.trailing_stop, enabled: !localSettings.trailing_stop.enabled }
                  })}
                  disabled={isDisabled}
                  className={`
                    relative inline-flex h-6 w-11 items-center rounded-full transition-colors
                    ${localSettings.trailing_stop.enabled ? 'bg-purple-600' : 'bg-gray-600'}
                    ${isDisabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
                  `}
                >
                  <span
                    className={`
                      inline-block h-4 w-4 transform rounded-full bg-white transition-transform
                      ${localSettings.trailing_stop.enabled ? 'translate-x-6' : 'translate-x-1'}
                    `}
                  />
                </button>
              </div>
              {localSettings.trailing_stop.enabled && (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <div className="flex items-center justify-between">
                        <label className="text-sm font-medium text-gray-300">Activate at R:R</label>
                        <div className="flex items-center gap-1">
                          <input
                            type="number"
                            value={localSettings.trailing_stop.activation_profit_pct}
                            onChange={(e) => updateLocalSettings({
                              trailing_stop: { ...localSettings.trailing_stop, activation_profit_pct: Number(e.target.value) }
                            })}
                            min={1}
                            max={10}
                            step={0.5}
                            disabled={isDisabled}
                            className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                          />
                          <span className="text-xs text-gray-500 w-4">:1</span>
                        </div>
                      </div>
                      <p className="text-xs text-gray-500">Start trailing when profit = {localSettings.trailing_stop.activation_profit_pct}x risk</p>
                    </div>
                    <div className="space-y-1">
                      <div className="flex items-center justify-between">
                        <label className="text-sm font-medium text-gray-300">Initial Lock</label>
                        <div className="flex items-center gap-1">
                          <input
                            type="number"
                            value={localSettings.trailing_stop.initial_trail_pct}
                            onChange={(e) => updateLocalSettings({
                              trailing_stop: { ...localSettings.trailing_stop, initial_trail_pct: Number(e.target.value) }
                            })}
                            min={0}
                            max={5}
                            step={0.5}
                            disabled={isDisabled}
                            className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                          />
                          <span className="text-xs text-gray-500 w-4">R</span>
                        </div>
                      </div>
                      <p className="text-xs text-gray-500">{localSettings.trailing_stop.initial_trail_pct === 0 ? 'Move SL to breakeven' : `Lock in ${localSettings.trailing_stop.initial_trail_pct}:1 profit`}</p>
                    </div>
                  </div>
                  {/* Editable Milestones */}
                  <div className="bg-gray-700/30 rounded p-3 space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-purple-400 font-medium">Trailing Milestones</span>
                      <button
                        type="button"
                        onClick={() => {
                          const milestones = [...(localSettings.trailing_stop.milestones || [])];
                          const lastTrigger = milestones.length > 0 ? milestones[milestones.length - 1].trigger_profit_pct : 1;
                          const lastTrail = milestones.length > 0 ? milestones[milestones.length - 1].trail_distance_pct : 0;
                          milestones.push({
                            trigger_profit_pct: lastTrigger + 1,
                            trail_distance_pct: lastTrail + 1,
                            label: `+${Math.round(lastTrail + 1)}R`,
                          });
                          updateLocalSettings({
                            trailing_stop: { ...localSettings.trailing_stop, milestones }
                          });
                        }}
                        disabled={isDisabled}
                        className="text-xs text-purple-400 hover:text-purple-300 disabled:opacity-50"
                      >
                        + Add Milestone
                      </button>
                    </div>
                    <div className="space-y-2">
                      {localSettings.trailing_stop.milestones?.map((m, i) => (
                        <div key={i} className="flex items-center gap-2 bg-gray-800/50 rounded p-2">
                          {/* Label */}
                          <input
                            type="text"
                            value={m.label || ''}
                            onChange={(e) => {
                              const milestones = [...(localSettings.trailing_stop.milestones || [])];
                              milestones[i] = { ...milestones[i], label: e.target.value };
                              updateLocalSettings({
                                trailing_stop: { ...localSettings.trailing_stop, milestones }
                              });
                            }}
                            placeholder="Label"
                            disabled={isDisabled}
                            className="w-14 px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs text-center focus:outline-none focus:ring-1 focus:ring-purple-500 disabled:opacity-50"
                          />
                          <span className="text-xs text-gray-500">At</span>
                          {/* Trigger R:R */}
                          <input
                            type="number"
                            value={m.trigger_profit_pct}
                            onChange={(e) => {
                              const milestones = [...(localSettings.trailing_stop.milestones || [])];
                              milestones[i] = { ...milestones[i], trigger_profit_pct: Number(e.target.value) };
                              updateLocalSettings({
                                trailing_stop: { ...localSettings.trailing_stop, milestones }
                              });
                            }}
                            min={0.5}
                            max={10}
                            step={0.5}
                            disabled={isDisabled}
                            className="w-14 px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs text-right focus:outline-none focus:ring-1 focus:ring-purple-500 disabled:opacity-50"
                          />
                          <span className="text-xs text-gray-500">:1 →</span>
                          <span className="text-xs text-gray-500">Lock</span>
                          {/* Trail Distance */}
                          <input
                            type="number"
                            value={m.trail_distance_pct}
                            onChange={(e) => {
                              const milestones = [...(localSettings.trailing_stop.milestones || [])];
                              milestones[i] = { ...milestones[i], trail_distance_pct: Number(e.target.value) };
                              updateLocalSettings({
                                trailing_stop: { ...localSettings.trailing_stop, milestones }
                              });
                            }}
                            min={0}
                            max={10}
                            step={0.5}
                            disabled={isDisabled}
                            className="w-14 px-1.5 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs text-right focus:outline-none focus:ring-1 focus:ring-purple-500 disabled:opacity-50"
                          />
                          <span className="text-xs text-gray-500">R</span>
                          {/* Remove button */}
                          <button
                            type="button"
                            onClick={() => {
                              const milestones = [...(localSettings.trailing_stop.milestones || [])];
                              milestones.splice(i, 1);
                              updateLocalSettings({
                                trailing_stop: { ...localSettings.trailing_stop, milestones }
                              });
                            }}
                            disabled={isDisabled || (localSettings.trailing_stop.milestones?.length || 0) <= 1}
                            className="text-red-400 hover:text-red-300 disabled:opacity-30 disabled:cursor-not-allowed ml-auto"
                            title="Remove milestone"
                          >
                            <XCircle className="w-4 h-4" />
                          </button>
                        </div>
                      ))}
                    </div>
                    <p className="text-xs text-gray-500 mt-2">
                      Example: "BE" at 2:1 → Lock 0R means move SL to breakeven when profit reaches 2x risk
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Other Settings */}
          <div className="space-y-4">
            <h4 className="text-sm font-medium text-gray-300 flex items-center gap-2">
              <Settings2 className="w-4 h-4 text-purple-400" />
              Other
            </h4>
            <div className="pl-6 space-y-4">
              <div className="flex items-start justify-between py-2">
                <div className="flex-1">
                  <label className="text-sm font-medium text-gray-300">LLM Validation</label>
                  <p className="text-xs text-gray-500 mt-1">Use AI to validate pattern before execution</p>
                </div>
                <button
                  type="button"
                  onClick={() => !isDisabled && updateLocalSettings({ llm_validation_enabled: !localSettings.llm_validation_enabled })}
                  disabled={isDisabled}
                  className={`
                    relative inline-flex h-6 w-11 items-center rounded-full transition-colors
                    ${localSettings.llm_validation_enabled ? 'bg-purple-600' : 'bg-gray-600'}
                    ${isDisabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
                  `}
                >
                  <span
                    className={`
                      inline-block h-4 w-4 transform rounded-full bg-white transition-transform
                      ${localSettings.llm_validation_enabled ? 'translate-x-6' : 'translate-x-1'}
                    `}
                  />
                </button>
              </div>
              <div className="space-y-1">
                <div className="flex items-center justify-between">
                  <label className="text-sm font-medium text-gray-300">Max Concurrent Patterns</label>
                  <input
                    type="number"
                    value={localSettings.max_concurrent_patterns}
                    onChange={(e) => updateLocalSettings({ max_concurrent_patterns: Number(e.target.value) })}
                    min={1}
                    max={20}
                    step={1}
                    disabled={isDisabled}
                    className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
                  />
                </div>
                <p className="text-xs text-gray-500">Maximum patterns to track simultaneously</p>
              </div>
            </div>
          </div>

          {/* Save Button */}
          {hasChanges && (
            <div className="flex justify-end pt-4 border-t border-gray-700">
              <button
                type="button"
                onClick={handleSave}
                disabled={isDisabled}
                className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Saving...
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="w-4 h-4" />
                    Save Changes
                  </>
                )}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
