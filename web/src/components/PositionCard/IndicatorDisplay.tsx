// Epic 11: Position Decision Engine - Story 11.21: Position Card UI
// IndicatorDisplay Component - Show key indicators (ADX, RSI, EMA, etc.)

import React from 'react';
import {
  TechnicalIndicators,
  TrendDirection,
  TREND_DISPLAY,
  formatIndicator,
} from '../../types/position-card';
import { TrendingUp, TrendingDown, Minus, Activity, Gauge, BarChart3 } from 'lucide-react';

interface IndicatorDisplayProps {
  /** Technical indicators data */
  indicators: TechnicalIndicators;
  /** 1-hour trend direction */
  trend1h: TrendDirection;
  /** 15-minute trend direction */
  trend15m: TrendDirection;
  /** Whether to show compact view */
  compact?: boolean;
  /** Additional CSS classes */
  className?: string;
}

/**
 * IndicatorDisplay shows key technical indicators with visual cues.
 * Includes ADX, RSI, EMAs, and trend directions.
 */
export default function IndicatorDisplay({
  indicators,
  trend1h,
  trend15m,
  compact = false,
  className = '',
}: IndicatorDisplayProps) {
  if (compact) {
    return (
      <div className={`flex items-center gap-4 text-sm ${className}`}>
        <IndicatorBadge label="ADX" value={indicators.adx} format="0" />
        <IndicatorBadge label="RSI" value={indicators.rsi} format="0" colorScale="rsi" />
        <TrendBadge label="1H" trend={trend1h} />
        <TrendBadge label="15M" trend={trend15m} />
      </div>
    );
  }

  // Full display with grid
  return (
    <div className={`space-y-3 ${className}`}>
      {/* Trend alignment */}
      <div className="flex items-center gap-4">
        <span className="text-sm text-gray-400">Trend:</span>
        <TrendBadge label="1H" trend={trend1h} showLabel />
        <TrendBadge label="15M" trend={trend15m} showLabel />
        <TrendAlignmentIndicator trend1h={trend1h} trend15m={trend15m} />
      </div>

      {/* Indicator grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <IndicatorCard
          icon={<Gauge className="w-4 h-4" />}
          label="ADX"
          value={indicators.adx}
          description={getADXDescription(indicators.adx)}
          colorScale="adx"
        />
        <IndicatorCard
          icon={<Activity className="w-4 h-4" />}
          label="RSI"
          value={indicators.rsi}
          description={getRSIDescription(indicators.rsi)}
          colorScale="rsi"
        />
        <IndicatorCard
          icon={<BarChart3 className="w-4 h-4" />}
          label="EMA 9"
          value={indicators.ema9}
          format="price"
        />
        <IndicatorCard
          icon={<BarChart3 className="w-4 h-4" />}
          label="EMA 21"
          value={indicators.ema21}
          format="price"
        />
      </div>

      {/* ATR if available */}
      {indicators.atr !== undefined && indicators.atr > 0 && (
        <div className="flex items-center gap-4 pt-2 border-t border-gray-700/50">
          <span className="text-sm text-gray-400">Volatility (ATR):</span>
          <span className="text-sm font-mono text-gray-300">
            {formatIndicator(indicators.atr, 4)}
          </span>
          {indicators.atrAverage !== undefined && indicators.atrAverage > 0 && (
            <span className={`text-xs ${indicators.atr > indicators.atrAverage ? 'text-orange-400' : 'text-green-400'}`}>
              ({indicators.atr > indicators.atrAverage ? 'Above' : 'Below'} avg: {formatIndicator(indicators.atrAverage, 4)})
            </span>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * IndicatorCard - Card display for a single indicator.
 */
interface IndicatorCardProps {
  icon: React.ReactNode;
  label: string;
  value: number;
  description?: string;
  format?: 'default' | 'price' | 'percent';
  colorScale?: 'adx' | 'rsi' | 'none';
}

function IndicatorCard({
  icon,
  label,
  value,
  description,
  format = 'default',
  colorScale = 'none',
}: IndicatorCardProps) {
  const formattedValue = formatValue(value, format);
  const color = getIndicatorColor(value, colorScale);

  return (
    <div className="p-2 rounded-lg bg-gray-800/50 border border-gray-700/50">
      <div className="flex items-center gap-1.5 text-gray-400 mb-1">
        {icon}
        <span className="text-xs">{label}</span>
      </div>
      <div className={`text-lg font-mono font-semibold ${color}`}>
        {formattedValue}
      </div>
      {description && (
        <div className="text-xs text-gray-500 mt-0.5">{description}</div>
      )}
    </div>
  );
}

/**
 * IndicatorBadge - Compact badge for a single indicator.
 */
interface IndicatorBadgeProps {
  label: string;
  value: number;
  format?: string;
  colorScale?: 'adx' | 'rsi' | 'none';
}

function IndicatorBadge({ label, value, format = '2', colorScale = 'none' }: IndicatorBadgeProps) {
  const color = getIndicatorColor(value, colorScale);
  const decimals = parseInt(format, 10) || 2;

  return (
    <span className="flex items-center gap-1">
      <span className="text-gray-500">{label}:</span>
      <span className={`font-mono ${color}`}>{value.toFixed(decimals)}</span>
    </span>
  );
}

/**
 * TrendBadge - Badge showing trend direction.
 */
interface TrendBadgeProps {
  label: string;
  trend: TrendDirection;
  showLabel?: boolean;
}

function TrendBadge({ label, trend, showLabel = false }: TrendBadgeProps) {
  const display = TREND_DISPLAY[trend];
  const Icon = trend === 'BULLISH' ? TrendingUp : trend === 'BEARISH' ? TrendingDown : Minus;

  return (
    <span className={`inline-flex items-center gap-1 text-sm ${display.color}`}>
      <span className="text-gray-500">{label}:</span>
      <Icon className="w-4 h-4" />
      {showLabel && <span>{display.name}</span>}
    </span>
  );
}

/**
 * TrendAlignmentIndicator - Shows whether trends are aligned.
 */
interface TrendAlignmentIndicatorProps {
  trend1h: TrendDirection;
  trend15m: TrendDirection;
}

function TrendAlignmentIndicator({ trend1h, trend15m }: TrendAlignmentIndicatorProps) {
  const aligned = trend1h === trend15m && trend1h !== 'NEUTRAL';
  const divergent = trend1h !== 'NEUTRAL' && trend15m !== 'NEUTRAL' && trend1h !== trend15m;

  if (aligned) {
    return (
      <span className="text-xs text-green-400 bg-green-500/10 px-2 py-0.5 rounded">
        Aligned
      </span>
    );
  }

  if (divergent) {
    return (
      <span className="text-xs text-red-400 bg-red-500/10 px-2 py-0.5 rounded">
        Divergent
      </span>
    );
  }

  return (
    <span className="text-xs text-gray-400 bg-gray-500/10 px-2 py-0.5 rounded">
      Mixed
    </span>
  );
}

/**
 * IndicatorsInline - Single line indicator summary.
 */
interface IndicatorsInlineProps {
  indicators: TechnicalIndicators;
  trend1h: TrendDirection;
  trend15m: TrendDirection;
  className?: string;
}

export function IndicatorsInline({ indicators, trend1h, trend15m, className = '' }: IndicatorsInlineProps) {
  const trend1hDisplay = TREND_DISPLAY[trend1h];
  const trend15mDisplay = TREND_DISPLAY[trend15m];

  return (
    <div className={`flex flex-wrap items-center gap-x-3 gap-y-1 text-xs ${className}`}>
      <span>
        <span className="text-gray-500">ADX:</span>{' '}
        <span className={getIndicatorColor(indicators.adx, 'adx')}>{indicators.adx.toFixed(1)}</span>
      </span>
      <span className="text-gray-600">|</span>
      <span>
        <span className="text-gray-500">RSI:</span>{' '}
        <span className={getIndicatorColor(indicators.rsi, 'rsi')}>{indicators.rsi.toFixed(1)}</span>
      </span>
      <span className="text-gray-600">|</span>
      <span>
        <span className="text-gray-500">Trend:</span>{' '}
        <span className={trend1hDisplay.color}>{trend1hDisplay.icon}</span>
        <span className={trend15mDisplay.color}>{trend15mDisplay.icon}</span>
      </span>
    </div>
  );
}

// Helper functions

function formatValue(value: number, format: string): string {
  switch (format) {
    case 'price':
      if (value >= 1000) return value.toFixed(2);
      if (value >= 1) return value.toFixed(4);
      return value.toFixed(8);
    case 'percent':
      return `${value.toFixed(2)}%`;
    default:
      return value.toFixed(2);
  }
}

function getIndicatorColor(value: number, scale: 'adx' | 'rsi' | 'none'): string {
  switch (scale) {
    case 'adx':
      // ADX: >25 strong trend (green), 20-25 moderate (yellow), <20 weak (gray)
      if (value >= 25) return 'text-green-400';
      if (value >= 20) return 'text-yellow-400';
      return 'text-gray-400';

    case 'rsi':
      // RSI: >70 overbought (red), 30-70 neutral (gray), <30 oversold (green for long)
      if (value >= 70) return 'text-red-400';
      if (value <= 30) return 'text-green-400';
      return 'text-gray-300';

    default:
      return 'text-gray-300';
  }
}

function getADXDescription(value: number): string {
  if (value >= 40) return 'Very Strong Trend';
  if (value >= 25) return 'Strong Trend';
  if (value >= 20) return 'Moderate Trend';
  return 'Weak/No Trend';
}

function getRSIDescription(value: number): string {
  if (value >= 80) return 'Extremely Overbought';
  if (value >= 70) return 'Overbought';
  if (value <= 20) return 'Extremely Oversold';
  if (value <= 30) return 'Oversold';
  return 'Neutral';
}
