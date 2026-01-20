// Epic 11: Position Decision Engine - Story 11.33: Mode-Strategy UI Update
// Strategy Settings Form component with collapsible sections

import React, { useState, useCallback } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Shield,
  Target,
  BarChart3,
  Sliders,
  Activity,
  TrendingUp,
  RefreshCw,
  CheckCircle2,
  Loader2,
  Info,
} from 'lucide-react';
import type {
  StrategyName,
  ModeStrategyConfig,
  SLTPConfig,
  ConfidenceConfig,
  ScoringConfig,
  ExitConditionsConfig,
  ExpandedSections,
} from '../../types/modeStrategy';
import {
  DEFAULT_EXPANDED_SECTIONS,
  isTrendEntryConditions,
  isMeanReversionEntryConditions,
  isBreakoutEntryConditions,
  isRangeEntryConditions,
  REGIME_DISPLAY_NAMES,
} from '../../types/modeStrategy';

// ==================== Interfaces ====================

interface StrategySettingsFormProps {
  strategyName: StrategyName;
  config: ModeStrategyConfig;
  onSave: (config: ModeStrategyConfig) => Promise<void>;
  onReset: () => Promise<void>;
  isSaving?: boolean;
  isResetting?: boolean;
  disabled?: boolean;
}

// ==================== Collapsible Section Component ====================

interface CollapsibleSectionProps {
  title: string;
  icon: React.ReactNode;
  iconColor: string;
  expanded: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}

function CollapsibleSection({
  title,
  icon,
  iconColor,
  expanded,
  onToggle,
  children,
}: CollapsibleSectionProps) {
  return (
    <div className="border border-gray-700 rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        className="w-full flex items-center justify-between px-4 py-3 bg-gray-700/30 hover:bg-gray-700/50 transition-colors"
      >
        <div className="flex items-center gap-2">
          <span className={iconColor}>{icon}</span>
          <span className="font-medium text-gray-200">{title}</span>
        </div>
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-gray-400" />
        ) : (
          <ChevronRight className="w-4 h-4 text-gray-400" />
        )}
      </button>
      {expanded && (
        <div className="p-4 bg-gray-800/50 space-y-4">
          {children}
        </div>
      )}
    </div>
  );
}

// ==================== Number Input Component ====================

interface NumberInputProps {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  description?: string;
  disabled?: boolean;
}

function NumberInput({
  label,
  value,
  onChange,
  min = 0,
  max = 100,
  step = 1,
  unit = '',
  description,
  disabled = false,
}: NumberInputProps) {
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium text-gray-300">{label}</label>
        <div className="flex items-center gap-1">
          <input
            type="number"
            value={value}
            onChange={(e) => onChange(Number(e.target.value))}
            min={min}
            max={max}
            step={step}
            disabled={disabled}
            className="w-20 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-right text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
          />
          {unit && <span className="text-xs text-gray-500 w-6">{unit}</span>}
        </div>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
    </div>
  );
}

// ==================== Toggle Input Component ====================

interface ToggleInputProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  description?: string;
  disabled?: boolean;
}

function ToggleInput({
  label,
  checked,
  onChange,
  description,
  disabled = false,
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

// ==================== Slider Input Component ====================

interface SliderInputProps {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min: number;
  max: number;
  step?: number;
  unit?: string;
  description?: string;
  disabled?: boolean;
  accentColor?: string;
}

function SliderInput({
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
  unit = '',
  description,
  disabled = false,
  accentColor = 'accent-purple-500',
}: SliderInputProps) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium text-gray-300">{label}</label>
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
        className={`w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed ${accentColor}`}
      />
      <div className="flex justify-between text-xs text-gray-600">
        <span>{min}{unit}</span>
        <span>{max}{unit}</span>
      </div>
    </div>
  );
}

// ==================== Position Settings Section ====================

interface PositionSettingsSectionProps {
  config: ModeStrategyConfig;
  onChange: (updates: Partial<ModeStrategyConfig>) => void;
  disabled?: boolean;
}

function PositionSettingsSection({ config, onChange, disabled }: PositionSettingsSectionProps) {
  return (
    <>
      <div className="grid grid-cols-2 gap-4">
        <NumberInput
          label="Leverage"
          value={config.leverage}
          onChange={(v) => onChange({ leverage: v })}
          min={1}
          max={125}
          step={1}
          unit="x"
          disabled={disabled}
        />
        <NumberInput
          label="Max Positions"
          value={config.max_positions}
          onChange={(v) => onChange({ max_positions: v })}
          min={1}
          max={50}
          disabled={disabled}
        />
      </div>
      <NumberInput
        label="Base Size"
        value={config.base_size_usd}
        onChange={(v) => onChange({ base_size_usd: v })}
        min={10}
        max={10000}
        step={10}
        unit="USD"
        description="Base position size in USD"
        disabled={disabled}
      />
      <NumberInput
        label="Priority"
        value={config.priority}
        onChange={(v) => onChange({ priority: v })}
        min={1}
        max={10}
        description="Strategy selection priority (lower = higher priority)"
        disabled={disabled}
      />

      {/* Supported Regimes Display */}
      <div className="pt-2">
        <label className="text-sm font-medium text-gray-400 block mb-2">Supported Market Regimes</label>
        <div className="flex flex-wrap gap-2">
          {config.supported_regimes.map((regime) => (
            <span
              key={regime}
              className="px-2 py-1 bg-purple-500/20 text-purple-300 text-xs rounded"
            >
              {REGIME_DISPLAY_NAMES[regime] || regime}
            </span>
          ))}
        </div>
      </div>
    </>
  );
}

// ==================== SLTP Settings Section ====================

interface SLTPSettingsSectionProps {
  sltp: SLTPConfig;
  onChange: (updates: Partial<SLTPConfig>) => void;
  disabled?: boolean;
}

function SLTPSettingsSection({ sltp, onChange, disabled }: SLTPSettingsSectionProps) {
  return (
    <>
      <SliderInput
        label="Stop Loss"
        value={sltp.sl_percent}
        onChange={(v) => onChange({ sl_percent: v })}
        min={0.1}
        max={10}
        step={0.1}
        unit="%"
        description="Maximum loss percentage before position is closed"
        disabled={disabled}
        accentColor="accent-red-500"
      />

      <div className="space-y-3 pt-2">
        <h4 className="text-sm font-medium text-gray-400">Take Profit Levels</h4>
        <div className="grid grid-cols-3 gap-3">
          <NumberInput
            label="TP1"
            value={sltp.tp1_percent}
            onChange={(v) => onChange({ tp1_percent: v })}
            min={0.1}
            max={20}
            step={0.1}
            unit="%"
            disabled={disabled}
          />
          <NumberInput
            label="TP2"
            value={sltp.tp2_percent}
            onChange={(v) => onChange({ tp2_percent: v })}
            min={0.1}
            max={30}
            step={0.1}
            unit="%"
            disabled={disabled}
          />
          <NumberInput
            label="TP3"
            value={sltp.tp3_percent}
            onChange={(v) => onChange({ tp3_percent: v })}
            min={0.1}
            max={50}
            step={0.1}
            unit="%"
            disabled={disabled}
          />
        </div>
      </div>

      <div className="border-t border-gray-700 pt-4 mt-4">
        <ToggleInput
          label="Trailing Stop"
          checked={sltp.trailing_enabled}
          onChange={(v) => onChange({ trailing_enabled: v })}
          description="Enable trailing stop to lock in profits"
          disabled={disabled}
        />

        {sltp.trailing_enabled && (
          <div className="mt-3 space-y-3 pl-4 border-l-2 border-purple-500/30">
            <NumberInput
              label="Activation"
              value={sltp.trailing_activation_pct}
              onChange={(v) => onChange({ trailing_activation_pct: v })}
              min={0.1}
              max={10}
              step={0.1}
              unit="%"
              description="Profit % required to activate trailing"
              disabled={disabled}
            />
            <NumberInput
              label="Trail Distance"
              value={sltp.trailing_stop_pct}
              onChange={(v) => onChange({ trailing_stop_pct: v })}
              min={0.1}
              max={5}
              step={0.1}
              unit="%"
              description="Distance from peak profit"
              disabled={disabled}
            />
          </div>
        )}
      </div>
    </>
  );
}

// ==================== Confidence Settings Section ====================

interface ConfidenceSettingsSectionProps {
  confidence: ConfidenceConfig;
  onChange: (updates: Partial<ConfidenceConfig>) => void;
  disabled?: boolean;
}

function ConfidenceSettingsSection({ confidence, onChange, disabled }: ConfidenceSettingsSectionProps) {
  return (
    <>
      <SliderInput
        label="Minimum Confidence"
        value={confidence.min_confidence}
        onChange={(v) => onChange({ min_confidence: v })}
        min={30}
        max={90}
        step={5}
        unit="%"
        description="Minimum confidence required for entry"
        disabled={disabled}
      />
      <SliderInput
        label="High Confidence"
        value={confidence.high_confidence}
        onChange={(v) => onChange({ high_confidence: v })}
        min={50}
        max={95}
        step={5}
        unit="%"
        description="Confidence level for increased position size"
        disabled={disabled}
      />
      <SliderInput
        label="Ultra Confidence"
        value={confidence.ultra_confidence}
        onChange={(v) => onChange({ ultra_confidence: v })}
        min={70}
        max={100}
        step={5}
        unit="%"
        description="Confidence level for maximum position size"
        disabled={disabled}
      />
    </>
  );
}

// ==================== Entry Conditions Section ====================

interface EntryConditionsSectionProps {
  strategyName: StrategyName;
  conditions: ModeStrategyConfig['entry_conditions'];
  onChange: (updates: Partial<ModeStrategyConfig['entry_conditions']>) => void;
  disabled?: boolean;
}

function EntryConditionsSection({ strategyName, conditions, onChange, disabled }: EntryConditionsSectionProps) {
  // Trend Following Entry Conditions
  if (isTrendEntryConditions(conditions)) {
    return (
      <>
        <NumberInput
          label="Minimum ADX"
          value={conditions.adx_min}
          onChange={(v) => onChange({ adx_min: v })}
          min={10}
          max={50}
          description="Minimum ADX for trend strength"
          disabled={disabled}
        />
        <ToggleInput
          label="Require Trend Alignment"
          checked={conditions.require_trend_align}
          onChange={(v) => onChange({ require_trend_align: v })}
          description="All timeframes must agree on trend direction"
          disabled={disabled}
        />
        <NumberInput
          label="Volume Multiplier"
          value={conditions.min_volume_multiplier}
          onChange={(v) => onChange({ min_volume_multiplier: v })}
          min={0.5}
          max={5}
          step={0.1}
          unit="x"
          description="Minimum volume relative to average"
          disabled={disabled}
        />
      </>
    );
  }

  // Mean Reversion Entry Conditions
  if (isMeanReversionEntryConditions(conditions)) {
    return (
      <>
        <div className="grid grid-cols-2 gap-4">
          <NumberInput
            label="RSI Oversold"
            value={conditions.rsi_oversold}
            onChange={(v) => onChange({ rsi_oversold: v })}
            min={10}
            max={40}
            description="RSI level for oversold"
            disabled={disabled}
          />
          <NumberInput
            label="RSI Overbought"
            value={conditions.rsi_overbought}
            onChange={(v) => onChange({ rsi_overbought: v })}
            min={60}
            max={90}
            description="RSI level for overbought"
            disabled={disabled}
          />
        </div>
        <NumberInput
          label="Bollinger Std Dev"
          value={conditions.bollinger_std}
          onChange={(v) => onChange({ bollinger_std: v })}
          min={1}
          max={4}
          step={0.5}
          description="Standard deviations for Bollinger Bands"
          disabled={disabled}
        />
        <ToggleInput
          label="Require Price at Band"
          checked={conditions.require_price_at_band}
          onChange={(v) => onChange({ require_price_at_band: v })}
          description="Price must touch Bollinger Band for entry"
          disabled={disabled}
        />
      </>
    );
  }

  // Breakout Entry Conditions
  if (isBreakoutEntryConditions(conditions)) {
    return (
      <>
        <NumberInput
          label="Breakout ATR Multiplier"
          value={conditions.breakout_atr_multiplier}
          onChange={(v) => onChange({ breakout_atr_multiplier: v })}
          min={0.5}
          max={5}
          step={0.1}
          unit="x"
          description="ATR multiple for breakout detection"
          disabled={disabled}
        />
        <NumberInput
          label="Volume Spike Multiplier"
          value={conditions.volume_spike_multiplier}
          onChange={(v) => onChange({ volume_spike_multiplier: v })}
          min={1}
          max={5}
          step={0.5}
          unit="x"
          description="Required volume spike for confirmation"
          disabled={disabled}
        />
        <ToggleInput
          label="Require Consolidation"
          checked={conditions.require_consolidation}
          onChange={(v) => onChange({ require_consolidation: v })}
          description="Price must consolidate before breakout"
          disabled={disabled}
        />
        {conditions.require_consolidation && (
          <NumberInput
            label="Consolidation Bars"
            value={conditions.consolidation_bars}
            onChange={(v) => onChange({ consolidation_bars: v })}
            min={5}
            max={100}
            description="Number of bars in consolidation"
            disabled={disabled}
          />
        )}
      </>
    );
  }

  // Range Trading Entry Conditions
  if (isRangeEntryConditions(conditions)) {
    return (
      <>
        <ToggleInput
          label="Range High Touch"
          checked={conditions.range_high_touch}
          onChange={(v) => onChange({ range_high_touch: v })}
          description="Enter short when price touches range high"
          disabled={disabled}
        />
        <ToggleInput
          label="Range Low Touch"
          checked={conditions.range_low_touch}
          onChange={(v) => onChange({ range_low_touch: v })}
          description="Enter long when price touches range low"
          disabled={disabled}
        />
        <NumberInput
          label="Range Width (ATR)"
          value={conditions.range_width_atr}
          onChange={(v) => onChange({ range_width_atr: v })}
          min={1}
          max={10}
          step={0.5}
          unit="x"
          description="Minimum range width in ATR"
          disabled={disabled}
        />
        <NumberInput
          label="Min Range Duration"
          value={conditions.min_range_duration_bars}
          onChange={(v) => onChange({ min_range_duration_bars: v })}
          min={5}
          max={100}
          description="Minimum bars in range"
          disabled={disabled}
        />
      </>
    );
  }

  // Default fallback
  return (
    <div className="text-sm text-gray-500">
      Entry conditions for {strategyName} strategy
    </div>
  );
}

// ==================== Exit Conditions Section ====================

interface ExitConditionsSectionProps {
  exit: ExitConditionsConfig;
  onChange: (updates: Partial<ExitConditionsConfig>) => void;
  disabled?: boolean;
}

function ExitConditionsSection({ exit, onChange, disabled }: ExitConditionsSectionProps) {
  return (
    <>
      {exit.use_ai_exit !== undefined && (
        <ToggleInput
          label="Use AI Exit"
          checked={exit.use_ai_exit}
          onChange={(v) => onChange({ use_ai_exit: v })}
          description="Use AI to determine optimal exit timing"
          disabled={disabled}
        />
      )}
      {exit.exit_at_mean !== undefined && (
        <ToggleInput
          label="Exit at Mean"
          checked={exit.exit_at_mean}
          onChange={(v) => onChange({ exit_at_mean: v })}
          description="Close position when price returns to mean"
          disabled={disabled}
        />
      )}
      {exit.exit_at_range_boundary !== undefined && (
        <ToggleInput
          label="Exit at Range Boundary"
          checked={exit.exit_at_range_boundary}
          onChange={(v) => onChange({ exit_at_range_boundary: v })}
          description="Close position at opposite range boundary"
          disabled={disabled}
        />
      )}
      <NumberInput
        label="Max Hold Time"
        value={exit.max_hold_minutes}
        onChange={(v) => onChange({ max_hold_minutes: v })}
        min={1}
        max={50000}
        unit="min"
        description="Maximum time to hold a position"
        disabled={disabled}
      />
      <ToggleInput
        label="Early Warning"
        checked={exit.early_warning_enabled}
        onChange={(v) => onChange({ early_warning_enabled: v })}
        description="Enable early exit warnings"
        disabled={disabled}
      />
    </>
  );
}

// ==================== Scoring Section ====================

interface ScoringSectionProps {
  scoring: ScoringConfig;
  onChange: (updates: Partial<ScoringConfig>) => void;
  disabled?: boolean;
}

function ScoringSection({ scoring, onChange, disabled }: ScoringSectionProps) {
  const total = scoring.technical_weight + scoring.momentum_weight +
    scoring.volume_weight + scoring.sentiment_weight;

  return (
    <>
      <div className="p-3 bg-gray-700/30 rounded-lg mb-4">
        <div className="flex items-center gap-2 mb-2">
          <Info className="w-4 h-4 text-blue-400" />
          <span className="text-sm text-gray-300">
            Total Weight: <span className={total === 100 ? 'text-green-400' : 'text-yellow-400'}>{total}%</span>
          </span>
        </div>
        {total !== 100 && (
          <p className="text-xs text-yellow-400">Weights should sum to 100%</p>
        )}
      </div>

      <SliderInput
        label="Technical Weight"
        value={scoring.technical_weight}
        onChange={(v) => onChange({ technical_weight: v })}
        min={0}
        max={100}
        step={5}
        unit="%"
        description="Weight for technical indicators"
        disabled={disabled}
      />
      <SliderInput
        label="Momentum Weight"
        value={scoring.momentum_weight}
        onChange={(v) => onChange({ momentum_weight: v })}
        min={0}
        max={100}
        step={5}
        unit="%"
        description="Weight for momentum signals"
        disabled={disabled}
      />
      <SliderInput
        label="Volume Weight"
        value={scoring.volume_weight}
        onChange={(v) => onChange({ volume_weight: v })}
        min={0}
        max={100}
        step={5}
        unit="%"
        description="Weight for volume analysis"
        disabled={disabled}
      />
      <SliderInput
        label="Sentiment Weight"
        value={scoring.sentiment_weight}
        onChange={(v) => onChange({ sentiment_weight: v })}
        min={0}
        max={100}
        step={5}
        unit="%"
        description="Weight for market sentiment"
        disabled={disabled}
      />
    </>
  );
}

// ==================== Main Component ====================

export default function StrategySettingsForm({
  strategyName,
  config,
  onSave,
  onReset,
  isSaving = false,
  isResetting = false,
  disabled = false,
}: StrategySettingsFormProps) {
  // Local state for editing
  const [localConfig, setLocalConfig] = useState<ModeStrategyConfig>(config);
  const [expandedSections, setExpandedSections] = useState<ExpandedSections>(DEFAULT_EXPANDED_SECTIONS);
  const [hasChanges, setHasChanges] = useState(false);

  // Reset local config when strategy changes
  React.useEffect(() => {
    setLocalConfig(config);
    setHasChanges(false);
  }, [config, strategyName]);

  // Toggle section expansion
  const toggleSection = (section: keyof ExpandedSections) => {
    setExpandedSections((prev) => ({
      ...prev,
      [section]: !prev[section],
    }));
  };

  // Update local config
  const updateConfig = useCallback((updates: Partial<ModeStrategyConfig>) => {
    setLocalConfig((prev) => ({ ...prev, ...updates }));
    setHasChanges(true);
  }, []);

  // Update SLTP config
  const updateSLTP = useCallback((updates: Partial<SLTPConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      sltp: { ...prev.sltp, ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update confidence config
  const updateConfidence = useCallback((updates: Partial<ConfidenceConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      confidence: { ...prev.confidence, ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update entry conditions
  const updateEntryConditions = useCallback((updates: Partial<ModeStrategyConfig['entry_conditions']>) => {
    setLocalConfig((prev) => ({
      ...prev,
      entry_conditions: { ...prev.entry_conditions, ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update exit conditions
  const updateExitConditions = useCallback((updates: Partial<ExitConditionsConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      exit_conditions: { ...prev.exit_conditions, ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update scoring config
  const updateScoring = useCallback((updates: Partial<ScoringConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      scoring: { ...prev.scoring, ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Handle save
  const handleSave = async () => {
    await onSave(localConfig);
    setHasChanges(false);
  };

  // Handle reset
  const handleReset = async () => {
    await onReset();
    setHasChanges(false);
  };

  const isDisabled = disabled || isSaving || isResetting;

  return (
    <div className="space-y-4">
      {/* Section Title */}
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium text-gray-400">
          Strategy Settings: <span className="text-purple-400">{strategyName.replace('_', ' ').replace(/\b\w/g, (l) => l.toUpperCase())}</span>
        </h4>
        {hasChanges && (
          <span className="text-xs text-yellow-400">Unsaved changes</span>
        )}
      </div>

      {/* Collapsible Sections */}
      <div className="space-y-3">
        {/* Position Settings */}
        <CollapsibleSection
          title="Position Settings"
          icon={<Sliders className="w-4 h-4" />}
          iconColor="text-blue-400"
          expanded={expandedSections.position}
          onToggle={() => toggleSection('position')}
        >
          <PositionSettingsSection
            config={localConfig}
            onChange={updateConfig}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* SLTP Settings */}
        <CollapsibleSection
          title="Stop Loss / Take Profit"
          icon={<Shield className="w-4 h-4" />}
          iconColor="text-red-400"
          expanded={expandedSections.sltp}
          onToggle={() => toggleSection('sltp')}
        >
          <SLTPSettingsSection
            sltp={localConfig.sltp}
            onChange={updateSLTP}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Confidence Settings */}
        <CollapsibleSection
          title="Confidence Thresholds"
          icon={<Target className="w-4 h-4" />}
          iconColor="text-green-400"
          expanded={expandedSections.confidence}
          onToggle={() => toggleSection('confidence')}
        >
          <ConfidenceSettingsSection
            confidence={localConfig.confidence}
            onChange={updateConfidence}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Entry Conditions */}
        <CollapsibleSection
          title="Entry Conditions"
          icon={<TrendingUp className="w-4 h-4" />}
          iconColor="text-purple-400"
          expanded={expandedSections.entry}
          onToggle={() => toggleSection('entry')}
        >
          <EntryConditionsSection
            strategyName={strategyName}
            conditions={localConfig.entry_conditions}
            onChange={updateEntryConditions}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Exit Conditions */}
        <CollapsibleSection
          title="Exit Conditions"
          icon={<Activity className="w-4 h-4" />}
          iconColor="text-yellow-400"
          expanded={expandedSections.exit}
          onToggle={() => toggleSection('exit')}
        >
          <ExitConditionsSection
            exit={localConfig.exit_conditions}
            onChange={updateExitConditions}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Scoring Weights */}
        <CollapsibleSection
          title="Scoring Weights"
          icon={<BarChart3 className="w-4 h-4" />}
          iconColor="text-cyan-400"
          expanded={expandedSections.scoring}
          onToggle={() => toggleSection('scoring')}
        >
          <ScoringSection
            scoring={localConfig.scoring}
            onChange={updateScoring}
            disabled={isDisabled}
          />
        </CollapsibleSection>
      </div>

      {/* Action Buttons */}
      <div className="flex gap-3 pt-4 border-t border-gray-700">
        <button
          type="button"
          onClick={handleReset}
          disabled={isDisabled}
          className="flex items-center justify-center gap-2 px-4 py-2 bg-gray-700 hover:bg-gray-600 text-gray-300 rounded-lg font-medium transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {isResetting ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <RefreshCw className="w-4 h-4" />
          )}
          Reset to Defaults
        </button>
        <button
          type="button"
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
              Save Changes
            </>
          )}
        </button>
      </div>
    </div>
  );
}
