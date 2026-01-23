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
  Clock,
  Layers,
  AlertTriangle,
  Scale,
  PlusCircle,
  Timer,
  Zap,
  DollarSign,
  AlertCircle,
  GitBranch,
  Brain,
  Bell,
} from 'lucide-react';
import type {
  StrategyName,
  ModeStrategyConfig,
  SLTPConfig,
  ConfidenceConfig,
  ScoringConfig,
  ExitConditionsConfig,
  ExpandedSections,
  StrategyTimeframe,
  MTFConfig,
  StrategyCircuitBreakerConfig,
  HedgeConfig,
  AveragingConfig,
  StaleReleaseConfig,
  PositionOptimizationConfig,
  FundingRateConfig,
  RiskConfig,
  TrendDivergenceConfig,
  DynamicAIExitConfig,
  StrategyEarlyWarningConfig,
} from '../../types/modeStrategy';
import {
  DEFAULT_EXPANDED_SECTIONS,
  DEFAULT_MTF_CONFIG,
  DEFAULT_CIRCUIT_BREAKER_CONFIG,
  DEFAULT_HEDGE_CONFIG,
  DEFAULT_AVERAGING_CONFIG,
  DEFAULT_STALE_RELEASE_CONFIG,
  DEFAULT_POSITION_OPTIMIZATION_CONFIG,
  DEFAULT_FUNDING_RATE_CONFIG,
  DEFAULT_RISK_CONFIG,
  DEFAULT_TREND_DIVERGENCE_CONFIG,
  DEFAULT_DYNAMIC_AI_EXIT_CONFIG,
  DEFAULT_EARLY_WARNING_CONFIG,
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

// ==================== Select Input Component ====================

interface SelectInputProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  description?: string;
  disabled?: boolean;
}

function SelectInput({
  label,
  value,
  onChange,
  options,
  description,
  disabled = false,
}: SelectInputProps) {
  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium text-gray-300">{label}</label>
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          disabled={disabled}
          className="w-32 px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-sm focus:outline-none focus:ring-2 focus:ring-purple-500 disabled:opacity-50"
        >
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>
      {description && (
        <p className="text-xs text-gray-500">{description}</p>
      )}
    </div>
  );
}

// ==================== Timeframe Options ====================

const TIMEFRAME_OPTIONS = [
  { value: '1m', label: '1 min' },
  { value: '3m', label: '3 min' },
  { value: '5m', label: '5 min' },
  { value: '15m', label: '15 min' },
  { value: '30m', label: '30 min' },
  { value: '1h', label: '1 hour' },
  { value: '2h', label: '2 hours' },
  { value: '4h', label: '4 hours' },
  { value: '1d', label: '1 day' },
];

const RISK_LEVEL_OPTIONS = [
  { value: 'low', label: 'Low' },
  { value: 'medium', label: 'Medium' },
  { value: 'high', label: 'High' },
  { value: 'aggressive', label: 'Aggressive' },
];

const STALE_ACTION_OPTIONS = [
  { value: 'hold', label: 'Hold' },
  { value: 'reduce', label: 'Reduce' },
  { value: 'close', label: 'Close' },
  { value: 'llm_decide', label: 'LLM Decide' },
];

// ==================== Timeframe Section ====================

interface TimeframeSectionProps {
  timeframe: StrategyTimeframe;
  onChange: (updates: Partial<StrategyTimeframe>) => void;
  disabled?: boolean;
}

function TimeframeSection({ timeframe, onChange, disabled }: TimeframeSectionProps) {
  return (
    <>
      <SelectInput
        label="Trend Timeframe"
        value={timeframe.trend_timeframe}
        onChange={(v) => onChange({ trend_timeframe: v })}
        options={TIMEFRAME_OPTIONS}
        description="Timeframe for trend analysis"
        disabled={disabled}
      />
      <SelectInput
        label="Entry Timeframe"
        value={timeframe.entry_timeframe}
        onChange={(v) => onChange({ entry_timeframe: v })}
        options={TIMEFRAME_OPTIONS}
        description="Timeframe for entry signals"
        disabled={disabled}
      />
      <SelectInput
        label="Analysis Timeframe"
        value={timeframe.analysis_timeframe}
        onChange={(v) => onChange({ analysis_timeframe: v })}
        options={TIMEFRAME_OPTIONS}
        description="Timeframe for general analysis"
        disabled={disabled}
      />
    </>
  );
}

// ==================== MTF Section ====================

interface MTFSectionProps {
  mtf: MTFConfig;
  onChange: (updates: Partial<MTFConfig>) => void;
  disabled?: boolean;
}

function MTFSection({ mtf, onChange, disabled }: MTFSectionProps) {
  return (
    <>
      <ToggleInput
        label="Enable MTF Analysis"
        checked={mtf.enabled}
        onChange={(v) => onChange({ enabled: v })}
        description="Use multi-timeframe analysis for entry decisions"
        disabled={disabled}
      />
      {mtf.enabled && (
        <>
          <div className="grid grid-cols-2 gap-4">
            <SelectInput
              label="Primary TF"
              value={mtf.primary_timeframe}
              onChange={(v) => onChange({ primary_timeframe: v })}
              options={TIMEFRAME_OPTIONS}
              disabled={disabled}
            />
            <NumberInput
              label="Primary Weight"
              value={mtf.primary_weight}
              onChange={(v) => onChange({ primary_weight: v })}
              min={0}
              max={100}
              step={5}
              unit="%"
              disabled={disabled}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <SelectInput
              label="Secondary TF"
              value={mtf.secondary_timeframe}
              onChange={(v) => onChange({ secondary_timeframe: v })}
              options={TIMEFRAME_OPTIONS}
              disabled={disabled}
            />
            <NumberInput
              label="Secondary Weight"
              value={mtf.secondary_weight}
              onChange={(v) => onChange({ secondary_weight: v })}
              min={0}
              max={100}
              step={5}
              unit="%"
              disabled={disabled}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <SelectInput
              label="Tertiary TF"
              value={mtf.tertiary_timeframe}
              onChange={(v) => onChange({ tertiary_timeframe: v })}
              options={TIMEFRAME_OPTIONS}
              disabled={disabled}
            />
            <NumberInput
              label="Tertiary Weight"
              value={mtf.tertiary_weight}
              onChange={(v) => onChange({ tertiary_weight: v })}
              min={0}
              max={100}
              step={5}
              unit="%"
              disabled={disabled}
            />
          </div>
          <NumberInput
            label="Min Consensus"
            value={mtf.min_consensus}
            onChange={(v) => onChange({ min_consensus: v })}
            min={1}
            max={3}
            step={1}
            description="Minimum timeframes that must agree"
            disabled={disabled}
          />
          <SliderInput
            label="Min Weighted Strength"
            value={mtf.min_weighted_strength}
            onChange={(v) => onChange({ min_weighted_strength: v })}
            min={0}
            max={100}
            step={5}
            unit="%"
            description="Minimum combined weighted strength required"
            disabled={disabled}
          />
          <ToggleInput
            label="Trend Stability Check"
            checked={mtf.trend_stability_check}
            onChange={(v) => onChange({ trend_stability_check: v })}
            description="Check trend stability across timeframes"
            disabled={disabled}
          />
        </>
      )}
    </>
  );
}

// ==================== Circuit Breaker Section ====================

interface CircuitBreakerSectionProps {
  circuitBreaker: StrategyCircuitBreakerConfig;
  onChange: (updates: Partial<StrategyCircuitBreakerConfig>) => void;
  disabled?: boolean;
}

function CircuitBreakerSection({ circuitBreaker, onChange, disabled }: CircuitBreakerSectionProps) {
  return (
    <>
      <div className="grid grid-cols-2 gap-4">
        <NumberInput
          label="Max Loss/Hour"
          value={circuitBreaker.max_loss_per_hour_usd}
          onChange={(v) => onChange({ max_loss_per_hour_usd: v })}
          min={0}
          max={10000}
          step={50}
          unit="USD"
          disabled={disabled}
        />
        <NumberInput
          label="Max Loss/Day"
          value={circuitBreaker.max_loss_per_day_usd}
          onChange={(v) => onChange({ max_loss_per_day_usd: v })}
          min={0}
          max={50000}
          step={100}
          unit="USD"
          disabled={disabled}
        />
      </div>
      <NumberInput
        label="Max Consecutive Losses"
        value={circuitBreaker.max_consecutive_losses}
        onChange={(v) => onChange({ max_consecutive_losses: v })}
        min={1}
        max={20}
        description="Trigger cooldown after this many losses"
        disabled={disabled}
      />
      <NumberInput
        label="Cooldown Period"
        value={circuitBreaker.cooldown_minutes}
        onChange={(v) => onChange({ cooldown_minutes: v })}
        min={1}
        max={1440}
        unit="min"
        description="Pause trading after circuit breaker triggers"
        disabled={disabled}
      />
      <div className="grid grid-cols-2 gap-4">
        <NumberInput
          label="Max Trades/Hour"
          value={circuitBreaker.max_trades_per_hour}
          onChange={(v) => onChange({ max_trades_per_hour: v })}
          min={1}
          max={100}
          disabled={disabled}
        />
        <NumberInput
          label="Max Trades/Day"
          value={circuitBreaker.max_trades_per_day}
          onChange={(v) => onChange({ max_trades_per_day: v })}
          min={1}
          max={500}
          disabled={disabled}
        />
      </div>
      <div className="border-t border-gray-700 pt-4 mt-4">
        <h4 className="text-sm font-medium text-gray-400 mb-3">Win Rate Check</h4>
        <NumberInput
          label="Check After Trades"
          value={circuitBreaker.win_rate_check_after}
          onChange={(v) => onChange({ win_rate_check_after: v })}
          min={1}
          max={50}
          description="Start checking win rate after this many trades"
          disabled={disabled}
        />
        <SliderInput
          label="Min Win Rate"
          value={circuitBreaker.min_win_rate_pct}
          onChange={(v) => onChange({ min_win_rate_pct: v })}
          min={0}
          max={100}
          step={5}
          unit="%"
          description="Trigger circuit breaker if win rate falls below"
          disabled={disabled}
        />
      </div>
    </>
  );
}

// ==================== Hedge Section ====================

interface HedgeSectionProps {
  hedge: HedgeConfig;
  onChange: (updates: Partial<HedgeConfig>) => void;
  disabled?: boolean;
}

function HedgeSection({ hedge, onChange, disabled }: HedgeSectionProps) {
  return (
    <>
      <ToggleInput
        label="Allow Hedging"
        checked={hedge.allow_hedge}
        onChange={(v) => onChange({ allow_hedge: v })}
        description="Allow opening opposite positions to hedge"
        disabled={disabled}
      />
      {hedge.allow_hedge && (
        <>
          <SliderInput
            label="Min Confidence for Hedge"
            value={hedge.min_confidence_for_hedge}
            onChange={(v) => onChange({ min_confidence_for_hedge: v })}
            min={50}
            max={95}
            step={5}
            unit="%"
            description="Minimum confidence to open hedge position"
            disabled={disabled}
          />
          <NumberInput
            label="Existing Must Be In Profit"
            value={hedge.existing_must_be_in_profit_pct}
            onChange={(v) => onChange({ existing_must_be_in_profit_pct: v })}
            min={0}
            max={50}
            step={0.5}
            unit="%"
            description="Existing position must have this profit to hedge"
            disabled={disabled}
          />
          <SliderInput
            label="Max Hedge Size"
            value={hedge.max_hedge_size_percent}
            onChange={(v) => onChange({ max_hedge_size_percent: v })}
            min={10}
            max={100}
            step={10}
            unit="%"
            description="Maximum hedge size as % of original position"
            disabled={disabled}
          />
          <ToggleInput
            label="Allow Same Mode Hedge"
            checked={hedge.allow_same_mode_hedge}
            onChange={(v) => onChange({ allow_same_mode_hedge: v })}
            description="Allow hedging within the same trading mode"
            disabled={disabled}
          />
          <NumberInput
            label="Max Total Exposure"
            value={hedge.max_total_exposure_multiplier}
            onChange={(v) => onChange({ max_total_exposure_multiplier: v })}
            min={1}
            max={5}
            step={0.5}
            unit="x"
            description="Maximum combined exposure multiplier"
            disabled={disabled}
          />
        </>
      )}
    </>
  );
}

// ==================== Averaging Section ====================

interface AveragingSectionProps {
  averaging: AveragingConfig;
  onChange: (updates: Partial<AveragingConfig>) => void;
  disabled?: boolean;
}

function AveragingSection({ averaging, onChange, disabled }: AveragingSectionProps) {
  return (
    <>
      <ToggleInput
        label="Allow Averaging"
        checked={averaging.allow_averaging}
        onChange={(v) => onChange({ allow_averaging: v })}
        description="Allow adding to existing positions"
        disabled={disabled}
      />
      {averaging.allow_averaging && (
        <>
          <div className="grid grid-cols-2 gap-4">
            <NumberInput
              label="Average Up After"
              value={averaging.average_up_profit_percent}
              onChange={(v) => onChange({ average_up_profit_percent: v })}
              min={0.5}
              max={20}
              step={0.5}
              unit="%"
              description="Add when in profit"
              disabled={disabled}
            />
            <NumberInput
              label="Average Down After"
              value={averaging.average_down_loss_percent}
              onChange={(v) => onChange({ average_down_loss_percent: v })}
              min={0.5}
              max={20}
              step={0.5}
              unit="%"
              description="Add when at loss"
              disabled={disabled}
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <NumberInput
              label="Add Size"
              value={averaging.add_size_percent}
              onChange={(v) => onChange({ add_size_percent: v })}
              min={10}
              max={100}
              step={10}
              unit="%"
              description="Size of add as % of original"
              disabled={disabled}
            />
            <NumberInput
              label="Max Averages"
              value={averaging.max_averages}
              onChange={(v) => onChange({ max_averages: v })}
              min={1}
              max={10}
              description="Maximum number of adds"
              disabled={disabled}
            />
          </div>
          <SliderInput
            label="Min Confidence"
            value={averaging.min_confidence_for_average}
            onChange={(v) => onChange({ min_confidence_for_average: v })}
            min={50}
            max={95}
            step={5}
            unit="%"
            description="Minimum confidence for averaging"
            disabled={disabled}
          />
          <ToggleInput
            label="Use LLM for Averaging"
            checked={averaging.use_llm_for_averaging}
            onChange={(v) => onChange({ use_llm_for_averaging: v })}
            description="Let AI decide when to average"
            disabled={disabled}
          />
          <div className="border-t border-gray-700 pt-4 mt-4">
            <ToggleInput
              label="Staged Entry"
              checked={averaging.staged_entry_enabled}
              onChange={(v) => onChange({ staged_entry_enabled: v })}
              description="Split initial entry into stages"
              disabled={disabled}
            />
            {averaging.staged_entry_enabled && (
              <NumberInput
                label="Entry Levels"
                value={averaging.staged_entry_levels}
                onChange={(v) => onChange({ staged_entry_levels: v })}
                min={2}
                max={5}
                description="Number of staged entry levels"
                disabled={disabled}
              />
            )}
          </div>
        </>
      )}
    </>
  );
}

// ==================== Stale Release Section ====================

interface StaleReleaseSectionProps {
  staleRelease: StaleReleaseConfig;
  onChange: (updates: Partial<StaleReleaseConfig>) => void;
  disabled?: boolean;
}

function StaleReleaseSection({ staleRelease, onChange, disabled }: StaleReleaseSectionProps) {
  return (
    <>
      <ToggleInput
        label="Enable Stale Release"
        checked={staleRelease.enabled}
        onChange={(v) => onChange({ enabled: v })}
        description="Automatically release stale positions"
        disabled={disabled}
      />
      {staleRelease.enabled && (
        <>
          <NumberInput
            label="Max Hold Duration"
            value={staleRelease.max_hold_duration_minutes}
            onChange={(v) => onChange({ max_hold_duration_minutes: v })}
            min={1}
            max={10080}
            unit="min"
            description="Maximum time before forced action"
            disabled={disabled}
          />
          <NumberInput
            label="Min Profit to Keep"
            value={staleRelease.min_profit_to_keep_pct}
            onChange={(v) => onChange({ min_profit_to_keep_pct: v })}
            min={0}
            max={10}
            step={0.1}
            unit="%"
            description="Keep position if profit above this"
            disabled={disabled}
          />
          <NumberInput
            label="Max Loss to Force Close"
            value={staleRelease.max_loss_to_force_close_pct}
            onChange={(v) => onChange({ max_loss_to_force_close_pct: v })}
            min={0}
            max={20}
            step={0.1}
            unit="%"
            description="Force close if loss exceeds this"
            disabled={disabled}
          />
          <div className="border-t border-gray-700 pt-4 mt-4">
            <h4 className="text-sm font-medium text-gray-400 mb-3">Stale Zone</h4>
            <div className="grid grid-cols-2 gap-4">
              <NumberInput
                label="Zone Low"
                value={staleRelease.stale_zone_lo_pct}
                onChange={(v) => onChange({ stale_zone_lo_pct: v })}
                min={-10}
                max={0}
                step={0.1}
                unit="%"
                disabled={disabled}
              />
              <NumberInput
                label="Zone High"
                value={staleRelease.stale_zone_hi_pct}
                onChange={(v) => onChange({ stale_zone_hi_pct: v })}
                min={0}
                max={10}
                step={0.1}
                unit="%"
                disabled={disabled}
              />
            </div>
            <SelectInput
              label="Zone Action"
              value={staleRelease.stale_zone_action}
              onChange={(v) => onChange({ stale_zone_action: v })}
              options={STALE_ACTION_OPTIONS}
              description="Action when position is in stale zone"
              disabled={disabled}
            />
          </div>
        </>
      )}
    </>
  );
}

// ==================== Position Optimization Section ====================

interface PositionOptimizationSectionProps {
  optimization: PositionOptimizationConfig;
  onChange: (updates: Partial<PositionOptimizationConfig>) => void;
  disabled?: boolean;
}

function PositionOptimizationSection({ optimization, onChange, disabled }: PositionOptimizationSectionProps) {
  return (
    <>
      <div className="space-y-4">
        <h4 className="text-sm font-medium text-gray-400">Re-Entry Settings</h4>
        <ToggleInput
          label="Enable Re-Entry"
          checked={optimization.reentry_enabled}
          onChange={(v) => onChange({ reentry_enabled: v })}
          description="Allow re-entry after partial exit"
          disabled={disabled}
        />
        {optimization.reentry_enabled && (
          <>
            <ToggleInput
              label="Re-Entry After TP1"
              checked={optimization.reentry_after_tp1}
              onChange={(v) => onChange({ reentry_after_tp1: v })}
              description="Allow re-entry after hitting TP1"
              disabled={disabled}
            />
            <NumberInput
              label="Min Pullback"
              value={optimization.reentry_min_pullback_pct}
              onChange={(v) => onChange({ reentry_min_pullback_pct: v })}
              min={0.1}
              max={10}
              step={0.1}
              unit="%"
              description="Minimum pullback before re-entry"
              disabled={disabled}
            />
            <NumberInput
              label="Max Re-Entries"
              value={optimization.max_reentries_per_position}
              onChange={(v) => onChange({ max_reentries_per_position: v })}
              min={0}
              max={10}
              description="Maximum re-entries per position"
              disabled={disabled}
            />
          </>
        )}
      </div>

      <div className="border-t border-gray-700 pt-4 mt-4">
        <h4 className="text-sm font-medium text-gray-400 mb-3">Dynamic Stop Loss</h4>
        <ToggleInput
          label="Enable Dynamic SL"
          checked={optimization.dynamic_sl_enabled}
          onChange={(v) => onChange({ dynamic_sl_enabled: v })}
          description="Move SL to breakeven when in profit"
          disabled={disabled}
        />
        {optimization.dynamic_sl_enabled && (
          <NumberInput
            label="Move SL at Profit"
            value={optimization.dynamic_sl_at_breakeven_pct}
            onChange={(v) => onChange({ dynamic_sl_at_breakeven_pct: v })}
            min={0.5}
            max={10}
            step={0.1}
            unit="%"
            description="Move SL to breakeven at this profit"
            disabled={disabled}
          />
        )}
      </div>

      <div className="border-t border-gray-700 pt-4 mt-4">
        <h4 className="text-sm font-medium text-gray-400 mb-3">Profit Protection</h4>
        <ToggleInput
          label="Enable Profit Protection"
          checked={optimization.profit_protection_enabled}
          onChange={(v) => onChange({ profit_protection_enabled: v })}
          description="Lock in profits when threshold reached"
          disabled={disabled}
        />
        {optimization.profit_protection_enabled && (
          <div className="grid grid-cols-2 gap-4">
            <NumberInput
              label="Trigger At"
              value={optimization.profit_protection_trigger_pct}
              onChange={(v) => onChange({ profit_protection_trigger_pct: v })}
              min={1}
              max={20}
              step={0.5}
              unit="%"
              description="Activate at this profit"
              disabled={disabled}
            />
            <NumberInput
              label="Lock In"
              value={optimization.profit_protection_lock_pct}
              onChange={(v) => onChange({ profit_protection_lock_pct: v })}
              min={0.5}
              max={15}
              step={0.5}
              unit="%"
              description="Lock in this profit"
              disabled={disabled}
            />
          </div>
        )}
      </div>
    </>
  );
}

// ==================== Funding Rate Section ====================

interface FundingRateSectionProps {
  fundingRate: FundingRateConfig;
  onChange: (updates: Partial<FundingRateConfig>) => void;
  disabled?: boolean;
}

function FundingRateSection({ fundingRate, onChange, disabled }: FundingRateSectionProps) {
  return (
    <>
      <ToggleInput
        label="Enable Funding Rate Management"
        checked={fundingRate.enabled}
        onChange={(v) => onChange({ enabled: v })}
        description="Manage positions around funding rate events"
        disabled={disabled}
      />
      {fundingRate.enabled && (
        <>
          <NumberInput
            label="Max Funding Rate"
            value={fundingRate.max_funding_rate_pct}
            onChange={(v) => onChange({ max_funding_rate_pct: v })}
            min={0.01}
            max={1}
            step={0.01}
            unit="%"
            description="Maximum acceptable funding rate"
            disabled={disabled}
          />
          <NumberInput
            label="Exit Before Funding"
            value={fundingRate.exit_before_funding_minutes}
            onChange={(v) => onChange({ exit_before_funding_minutes: v })}
            min={0}
            max={60}
            unit="min"
            description="Exit this many minutes before funding"
            disabled={disabled}
          />
          <NumberInput
            label="Block Entry Above"
            value={fundingRate.block_entry_above_rate_pct}
            onChange={(v) => onChange({ block_entry_above_rate_pct: v })}
            min={0.01}
            max={1}
            step={0.01}
            unit="%"
            description="Block new entries above this rate"
            disabled={disabled}
          />
        </>
      )}
    </>
  );
}

// ==================== Risk Section ====================

interface RiskSectionProps {
  risk: RiskConfig;
  onChange: (updates: Partial<RiskConfig>) => void;
  disabled?: boolean;
}

function RiskSection({ risk, onChange, disabled }: RiskSectionProps) {
  return (
    <>
      <SelectInput
        label="Risk Level"
        value={risk.risk_level}
        onChange={(v) => onChange({ risk_level: v })}
        options={RISK_LEVEL_OPTIONS}
        description="Overall risk tolerance for this strategy"
        disabled={disabled}
      />
      <SliderInput
        label="Max Drawdown"
        value={risk.max_drawdown_percent}
        onChange={(v) => onChange({ max_drawdown_percent: v })}
        min={1}
        max={50}
        step={1}
        unit="%"
        description="Maximum allowable drawdown"
        disabled={disabled}
        accentColor="accent-red-500"
      />
      <SliderInput
        label="Max Daily Loss"
        value={risk.max_daily_loss_percent}
        onChange={(v) => onChange({ max_daily_loss_percent: v })}
        min={1}
        max={20}
        step={1}
        unit="%"
        description="Maximum loss per day"
        disabled={disabled}
        accentColor="accent-red-500"
      />
      <SliderInput
        label="Position Risk"
        value={risk.position_risk_percent}
        onChange={(v) => onChange({ position_risk_percent: v })}
        min={0.5}
        max={10}
        step={0.5}
        unit="%"
        description="Risk per position as % of capital"
        disabled={disabled}
      />
    </>
  );
}

// ==================== Trend Divergence Section ====================

interface TrendDivergenceSectionProps {
  divergence: TrendDivergenceConfig;
  onChange: (updates: Partial<TrendDivergenceConfig>) => void;
  disabled?: boolean;
}

function TrendDivergenceSection({ divergence, onChange, disabled }: TrendDivergenceSectionProps) {
  return (
    <>
      <ToggleInput
        label="Enable Divergence Detection"
        checked={divergence.enabled}
        onChange={(v) => onChange({ enabled: v })}
        description="Detect price/indicator divergence"
        disabled={disabled}
      />
      {divergence.enabled && (
        <>
          <NumberInput
            label="Min Divergence"
            value={divergence.min_divergence_percent}
            onChange={(v) => onChange({ min_divergence_percent: v })}
            min={0.5}
            max={20}
            step={0.5}
            unit="%"
            description="Minimum divergence to trigger"
            disabled={disabled}
          />
          <ToggleInput
            label="Block on Divergence"
            checked={divergence.block_on_divergence}
            onChange={(v) => onChange({ block_on_divergence: v })}
            description="Block entries when divergence detected"
            disabled={disabled}
          />
          <SliderInput
            label="Divergence Weight"
            value={divergence.divergence_weight}
            onChange={(v) => onChange({ divergence_weight: v })}
            min={0}
            max={100}
            step={5}
            unit="%"
            description="Weight in entry decision"
            disabled={disabled}
          />
        </>
      )}
    </>
  );
}

// ==================== Dynamic AI Exit Section ====================

interface DynamicAIExitSectionProps {
  aiExit: DynamicAIExitConfig;
  onChange: (updates: Partial<DynamicAIExitConfig>) => void;
  disabled?: boolean;
}

function DynamicAIExitSection({ aiExit, onChange, disabled }: DynamicAIExitSectionProps) {
  return (
    <>
      <ToggleInput
        label="Enable AI Exit"
        checked={aiExit.enabled}
        onChange={(v) => onChange({ enabled: v })}
        description="Use AI/LLM for exit decisions"
        disabled={disabled}
      />
      {aiExit.enabled && (
        <>
          <NumberInput
            label="Min Hold Before AI"
            value={Math.round(aiExit.min_hold_before_ai_ms / 1000)}
            onChange={(v) => onChange({ min_hold_before_ai_ms: v * 1000 })}
            min={0}
            max={3600}
            unit="sec"
            description="Minimum hold time before AI can exit"
            disabled={disabled}
          />
          <NumberInput
            label="AI Check Interval"
            value={Math.round(aiExit.ai_check_interval_ms / 1000)}
            onChange={(v) => onChange({ ai_check_interval_ms: v * 1000 })}
            min={5}
            max={300}
            unit="sec"
            description="How often AI evaluates position"
            disabled={disabled}
          />
          <ToggleInput
            label="Use LLM for Loss"
            checked={aiExit.use_llm_for_loss}
            onChange={(v) => onChange({ use_llm_for_loss: v })}
            description="Let AI decide when to exit losing positions"
            disabled={disabled}
          />
          <ToggleInput
            label="Use LLM for Profit"
            checked={aiExit.use_llm_for_profit}
            onChange={(v) => onChange({ use_llm_for_profit: v })}
            description="Let AI decide when to exit winning positions"
            disabled={disabled}
          />
          <NumberInput
            label="Max Hold Time"
            value={Math.round(aiExit.max_hold_time_ms / 60000)}
            onChange={(v) => onChange({ max_hold_time_ms: v * 60000 })}
            min={1}
            max={10080}
            unit="min"
            description="Maximum hold time (AI will force exit)"
            disabled={disabled}
          />
        </>
      )}
    </>
  );
}

// ==================== Early Warning Section ====================

interface EarlyWarningSectionProps {
  earlyWarning: StrategyEarlyWarningConfig;
  onChange: (updates: Partial<StrategyEarlyWarningConfig>) => void;
  disabled?: boolean;
}

function EarlyWarningSection({ earlyWarning, onChange, disabled }: EarlyWarningSectionProps) {
  return (
    <>
      <ToggleInput
        label="Enable Early Warning"
        checked={earlyWarning.enabled}
        onChange={(v) => onChange({ enabled: v })}
        description="Monitor underwater positions for early exit"
        disabled={disabled}
      />
      {earlyWarning.enabled && (
        <>
          <NumberInput
            label="Start After"
            value={earlyWarning.start_after_minutes}
            onChange={(v) => onChange({ start_after_minutes: v })}
            min={1}
            max={120}
            unit="min"
            description="Start monitoring after this time"
            disabled={disabled}
          />
          <NumberInput
            label="Min Loss Trigger"
            value={earlyWarning.min_loss_percent}
            onChange={(v) => onChange({ min_loss_percent: v })}
            min={0.1}
            max={10}
            step={0.1}
            unit="%"
            description="Minimum loss to trigger warning"
            disabled={disabled}
          />
          <NumberInput
            label="Check Interval"
            value={earlyWarning.check_interval_secs}
            onChange={(v) => onChange({ check_interval_secs: v })}
            min={5}
            max={300}
            unit="sec"
            description="How often to check position"
            disabled={disabled}
          />
          <NumberInput
            label="Close Min Hold"
            value={earlyWarning.close_min_hold_mins}
            onChange={(v) => onChange({ close_min_hold_mins: v })}
            min={1}
            max={60}
            unit="min"
            description="Minimum hold before early close allowed"
            disabled={disabled}
          />
        </>
      )}
    </>
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
        <h4 className="text-sm font-medium text-gray-400 mt-4">Position Cut % at Each TP</h4>
        <div className="grid grid-cols-3 gap-3">
          <NumberInput
            label="TP1 Sell"
            value={sltp.tp1_sell_percent ?? 33}
            onChange={(v) => onChange({ tp1_sell_percent: v })}
            min={0}
            max={100}
            step={5}
            unit="%"
            description="% to close at TP1"
            disabled={disabled}
          />
          <NumberInput
            label="TP2 Sell"
            value={sltp.tp2_sell_percent ?? 33}
            onChange={(v) => onChange({ tp2_sell_percent: v })}
            min={0}
            max={100}
            step={5}
            unit="%"
            description="% to close at TP2"
            disabled={disabled}
          />
          <NumberInput
            label="TP3 Sell"
            value={sltp.tp3_sell_percent ?? 34}
            onChange={(v) => onChange({ tp3_sell_percent: v })}
            min={0}
            max={100}
            step={5}
            unit="%"
            description="% to close at TP3"
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

  // Update timeframe config
  const updateTimeframe = useCallback((updates: Partial<StrategyTimeframe>) => {
    setLocalConfig((prev) => ({
      ...prev,
      timeframe: { ...prev.timeframe, ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update MTF config
  const updateMTF = useCallback((updates: Partial<MTFConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      mtf: { ...(prev.mtf || {} as MTFConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update circuit breaker config
  const updateCircuitBreaker = useCallback((updates: Partial<StrategyCircuitBreakerConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      circuit_breaker: { ...(prev.circuit_breaker || {} as StrategyCircuitBreakerConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update hedge config
  const updateHedge = useCallback((updates: Partial<HedgeConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      hedge: { ...(prev.hedge || {} as HedgeConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update averaging config
  const updateAveraging = useCallback((updates: Partial<AveragingConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      averaging: { ...(prev.averaging || {} as AveragingConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update stale release config
  const updateStaleRelease = useCallback((updates: Partial<StaleReleaseConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      stale_release: { ...(prev.stale_release || {} as StaleReleaseConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update position optimization config
  const updatePositionOptimization = useCallback((updates: Partial<PositionOptimizationConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      position_optimization: { ...(prev.position_optimization || {} as PositionOptimizationConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update funding rate config
  const updateFundingRate = useCallback((updates: Partial<FundingRateConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      funding_rate: { ...(prev.funding_rate || {} as FundingRateConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update risk config
  const updateRisk = useCallback((updates: Partial<RiskConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      risk: { ...(prev.risk || {} as RiskConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update trend divergence config
  const updateTrendDivergence = useCallback((updates: Partial<TrendDivergenceConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      trend_divergence: { ...(prev.trend_divergence || {} as TrendDivergenceConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update dynamic AI exit config
  const updateDynamicAIExit = useCallback((updates: Partial<DynamicAIExitConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      dynamic_ai_exit: { ...(prev.dynamic_ai_exit || {} as DynamicAIExitConfig), ...updates },
    }));
    setHasChanges(true);
  }, []);

  // Update early warning config
  const updateEarlyWarning = useCallback((updates: Partial<StrategyEarlyWarningConfig>) => {
    setLocalConfig((prev) => ({
      ...prev,
      early_warning: { ...(prev.early_warning || {} as StrategyEarlyWarningConfig), ...updates },
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
          expanded={expandedSections.position_sizing}
          onToggle={() => toggleSection('position_sizing')}
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
          expanded={expandedSections.entry_conditions}
          onToggle={() => toggleSection('entry_conditions')}
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
          expanded={expandedSections.exit_conditions}
          onToggle={() => toggleSection('exit_conditions')}
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

        {/* Story 11.41: Timeframe Settings */}
        <CollapsibleSection
          title="Timeframe"
          icon={<Clock className="w-4 h-4" />}
          iconColor="text-orange-400"
          expanded={expandedSections.timeframe}
          onToggle={() => toggleSection('timeframe')}
        >
          <TimeframeSection
            timeframe={localConfig.timeframe}
            onChange={updateTimeframe}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Multi-Timeframe Analysis */}
        <CollapsibleSection
          title="Multi-Timeframe Analysis"
          icon={<Layers className="w-4 h-4" />}
          iconColor="text-indigo-400"
          expanded={expandedSections.mtf}
          onToggle={() => toggleSection('mtf')}
        >
          <MTFSection
            mtf={localConfig.mtf || DEFAULT_MTF_CONFIG}
            onChange={updateMTF}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Circuit Breaker */}
        <CollapsibleSection
          title="Circuit Breaker"
          icon={<AlertTriangle className="w-4 h-4" />}
          iconColor="text-red-500"
          expanded={expandedSections.circuit_breaker}
          onToggle={() => toggleSection('circuit_breaker')}
        >
          <CircuitBreakerSection
            circuitBreaker={localConfig.circuit_breaker || DEFAULT_CIRCUIT_BREAKER_CONFIG}
            onChange={updateCircuitBreaker}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Hedging */}
        <CollapsibleSection
          title="Hedging"
          icon={<Scale className="w-4 h-4" />}
          iconColor="text-teal-400"
          expanded={expandedSections.hedge}
          onToggle={() => toggleSection('hedge')}
        >
          <HedgeSection
            hedge={localConfig.hedge || DEFAULT_HEDGE_CONFIG}
            onChange={updateHedge}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Position Averaging */}
        <CollapsibleSection
          title="Position Averaging"
          icon={<PlusCircle className="w-4 h-4" />}
          iconColor="text-lime-400"
          expanded={expandedSections.averaging}
          onToggle={() => toggleSection('averaging')}
        >
          <AveragingSection
            averaging={localConfig.averaging || DEFAULT_AVERAGING_CONFIG}
            onChange={updateAveraging}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Stale Position Release */}
        <CollapsibleSection
          title="Stale Position Release"
          icon={<Timer className="w-4 h-4" />}
          iconColor="text-amber-400"
          expanded={expandedSections.stale_release}
          onToggle={() => toggleSection('stale_release')}
        >
          <StaleReleaseSection
            staleRelease={localConfig.stale_release || DEFAULT_STALE_RELEASE_CONFIG}
            onChange={updateStaleRelease}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Position Optimization */}
        <CollapsibleSection
          title="Position Optimization"
          icon={<Zap className="w-4 h-4" />}
          iconColor="text-pink-400"
          expanded={expandedSections.position_optimization}
          onToggle={() => toggleSection('position_optimization')}
        >
          <PositionOptimizationSection
            optimization={localConfig.position_optimization || DEFAULT_POSITION_OPTIMIZATION_CONFIG}
            onChange={updatePositionOptimization}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Funding Rate */}
        <CollapsibleSection
          title="Funding Rate"
          icon={<DollarSign className="w-4 h-4" />}
          iconColor="text-emerald-400"
          expanded={expandedSections.funding_rate}
          onToggle={() => toggleSection('funding_rate')}
        >
          <FundingRateSection
            fundingRate={localConfig.funding_rate || DEFAULT_FUNDING_RATE_CONFIG}
            onChange={updateFundingRate}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Risk Management */}
        <CollapsibleSection
          title="Risk Management"
          icon={<AlertCircle className="w-4 h-4" />}
          iconColor="text-rose-400"
          expanded={expandedSections.risk}
          onToggle={() => toggleSection('risk')}
        >
          <RiskSection
            risk={localConfig.risk || DEFAULT_RISK_CONFIG}
            onChange={updateRisk}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Trend Divergence */}
        <CollapsibleSection
          title="Trend Divergence"
          icon={<GitBranch className="w-4 h-4" />}
          iconColor="text-violet-400"
          expanded={expandedSections.trend_divergence}
          onToggle={() => toggleSection('trend_divergence')}
        >
          <TrendDivergenceSection
            divergence={localConfig.trend_divergence || DEFAULT_TREND_DIVERGENCE_CONFIG}
            onChange={updateTrendDivergence}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Dynamic AI Exit */}
        <CollapsibleSection
          title="Dynamic AI Exit"
          icon={<Brain className="w-4 h-4" />}
          iconColor="text-fuchsia-400"
          expanded={expandedSections.dynamic_ai_exit}
          onToggle={() => toggleSection('dynamic_ai_exit')}
        >
          <DynamicAIExitSection
            aiExit={localConfig.dynamic_ai_exit || DEFAULT_DYNAMIC_AI_EXIT_CONFIG}
            onChange={updateDynamicAIExit}
            disabled={isDisabled}
          />
        </CollapsibleSection>

        {/* Story 11.41: Early Warning */}
        <CollapsibleSection
          title="Early Warning"
          icon={<Bell className="w-4 h-4" />}
          iconColor="text-sky-400"
          expanded={expandedSections.early_warning}
          onToggle={() => toggleSection('early_warning')}
        >
          <EarlyWarningSection
            earlyWarning={localConfig.early_warning || DEFAULT_EARLY_WARNING_CONFIG}
            onChange={updateEarlyWarning}
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
