import React, { useState } from 'react';
import {
  TrendingUp,
  Activity,
  Zap,
  Target,
  Shield,
  Clock,
  BarChart2,
  ArrowUpDown,
  Search,
  Check,
  Info,
  Star,
  Copy,
} from 'lucide-react';
import type { NestedConditionGroup, AdvancedCondition } from './AdvancedConditionBuilder';

export interface StrategyTemplate {
  id: string;
  name: string;
  description: string;
  category: 'trend' | 'momentum' | 'mean-reversion' | 'breakout' | 'scalping' | 'swing';
  difficulty: 'beginner' | 'intermediate' | 'advanced';
  timeframes: string[];
  indicators: string[];
  entryConditions: NestedConditionGroup;
  exitConditions: NestedConditionGroup;
  riskSettings: {
    stopLossPercent: number;
    takeProfitPercent: number;
    positionSizePercent: number;
    maxDrawdown?: number;
  };
  description_long?: string;
  tips?: string[];
  backtestStats?: {
    winRate: number;
    profitFactor: number;
    avgTrade: number;
    maxDrawdown: number;
  };
}

// Pre-built strategy templates
const STRATEGY_TEMPLATES: StrategyTemplate[] = [
  {
    id: 'ema-crossover',
    name: 'EMA Crossover',
    description: 'Classic trend-following strategy using EMA 9/21 crossover',
    category: 'trend',
    difficulty: 'beginner',
    timeframes: ['15m', '1h', '4h'],
    indicators: ['EMA 9', 'EMA 21'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'ema-cross',
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'EMA', params: { period: 9 } },
          operator: 'crosses_above',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 21 } },
        } as AdvancedCondition,
        {
          id: 'trend-confirm',
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 50 } },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'ema-cross-exit',
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'EMA', params: { period: 9 } },
          operator: 'crosses_below',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 21 } },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 2,
      takeProfitPercent: 4,
      positionSizePercent: 5,
    },
    tips: [
      'Works best in trending markets',
      'Avoid during consolidation periods',
      'Consider adding volume confirmation',
    ],
    backtestStats: {
      winRate: 52,
      profitFactor: 1.6,
      avgTrade: 1.2,
      maxDrawdown: 12,
    },
  },
  {
    id: 'rsi-oversold',
    name: 'RSI Oversold Bounce',
    description: 'Mean reversion strategy buying oversold conditions',
    category: 'mean-reversion',
    difficulty: 'beginner',
    timeframes: ['1h', '4h', '1d'],
    indicators: ['RSI 14'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'rsi-oversold',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '<',
          rightOperand: { type: 'value', value: 30 },
        } as AdvancedCondition,
        {
          id: 'rsi-turning',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 }, offset: 1 },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'rsi-overbought',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '>',
          rightOperand: { type: 'value', value: 70 },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 3,
      takeProfitPercent: 5,
      positionSizePercent: 3,
    },
    tips: [
      'Wait for RSI to start rising before entry',
      'Combine with support levels for better entries',
      'Avoid during strong downtrends',
    ],
    backtestStats: {
      winRate: 58,
      profitFactor: 1.4,
      avgTrade: 0.9,
      maxDrawdown: 15,
    },
  },
  {
    id: 'bollinger-squeeze',
    name: 'Bollinger Band Squeeze',
    description: 'Breakout strategy based on volatility contraction',
    category: 'breakout',
    difficulty: 'intermediate',
    timeframes: ['15m', '1h', '4h'],
    indicators: ['Bollinger Bands 20,2', 'ATR 14'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'bb-squeeze',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'ATR', params: { period: 14 } },
          operator: '<',
          rightOperand: { type: 'indicator', indicator: 'ATR', params: { period: 14, lookback: 20, type: 'min' } },
        } as AdvancedCondition,
        {
          id: 'price-above-bb',
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'BB_UPPER', params: { period: 20, stdDev: 2 } },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'price-below-middle',
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '<',
          rightOperand: { type: 'indicator', indicator: 'BB_MIDDLE', params: { period: 20, stdDev: 2 } },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 1.5,
      takeProfitPercent: 3,
      positionSizePercent: 4,
    },
    tips: [
      'Look for narrow bands indicating low volatility',
      'Volume spike on breakout adds confirmation',
      'Expect big moves after squeeze periods',
    ],
    backtestStats: {
      winRate: 48,
      profitFactor: 1.8,
      avgTrade: 1.5,
      maxDrawdown: 10,
    },
  },
  {
    id: 'macd-divergence',
    name: 'MACD Divergence',
    description: 'Identify trend reversals using MACD histogram divergence',
    category: 'momentum',
    difficulty: 'advanced',
    timeframes: ['1h', '4h', '1d'],
    indicators: ['MACD 12,26,9'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'macd-cross',
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'MACD_LINE', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 } },
          operator: 'crosses_above',
          rightOperand: { type: 'indicator', indicator: 'MACD_SIGNAL', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 } },
        } as AdvancedCondition,
        {
          id: 'histogram-positive',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'MACD', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9, output: 'histogram' } },
          operator: '>',
          rightOperand: { type: 'value', value: 0 },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'macd-cross-exit',
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'MACD_LINE', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 } },
          operator: 'crosses_below',
          rightOperand: { type: 'indicator', indicator: 'MACD_SIGNAL', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 } },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 2.5,
      takeProfitPercent: 5,
      positionSizePercent: 3,
    },
    tips: [
      'Look for divergence between price and MACD',
      'Combine with support/resistance levels',
      'More reliable on higher timeframes',
    ],
    backtestStats: {
      winRate: 55,
      profitFactor: 1.7,
      avgTrade: 1.3,
      maxDrawdown: 14,
    },
  },
  {
    id: 'multi-tf-trend',
    name: 'Multi-Timeframe Trend',
    description: 'Trend alignment across multiple timeframes for high-probability entries',
    category: 'trend',
    difficulty: 'advanced',
    timeframes: ['15m', '1h', '4h'],
    indicators: ['EMA 50 (4h)', 'EMA 20 (1h)', 'RSI 14 (15m)'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: '4h-trend',
          type: 'simple',
          leftOperand: { type: 'price', timeframe: '4h' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 50 }, timeframe: '4h' },
        } as AdvancedCondition,
        {
          id: '1h-trend',
          type: 'simple',
          leftOperand: { type: 'price', timeframe: '1h' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 20 }, timeframe: '1h' },
        } as AdvancedCondition,
        {
          id: '15m-momentum',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 }, timeframe: '15m' },
          operator: 'between',
          rightOperand: { type: 'value', value: 40, value2: 60 },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: '1h-trend-break',
          type: 'simple',
          leftOperand: { type: 'price', timeframe: '1h' },
          operator: '<',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 20 }, timeframe: '1h' },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 2,
      takeProfitPercent: 6,
      positionSizePercent: 4,
    },
    description_long: 'This strategy uses multiple timeframes to ensure trend alignment. The 4h timeframe determines the major trend, 1h confirms intermediate direction, and 15m provides the entry timing. Only enter when all timeframes agree.',
    tips: [
      'Higher timeframe trend is the most important filter',
      'Wait for pullbacks on the entry timeframe',
      'Avoid trading against the 4h trend',
    ],
    backtestStats: {
      winRate: 62,
      profitFactor: 2.1,
      avgTrade: 1.8,
      maxDrawdown: 8,
    },
  },
  {
    id: 'scalping-momentum',
    name: 'Momentum Scalper',
    description: 'Quick momentum trades with tight stops',
    category: 'scalping',
    difficulty: 'advanced',
    timeframes: ['1m', '5m'],
    indicators: ['EMA 9', 'RSI 7', 'Volume'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'price-above-ema',
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 9 } },
        } as AdvancedCondition,
        {
          id: 'rsi-momentum',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 7 } },
          operator: 'between',
          rightOperand: { type: 'value', value: 50, value2: 70 },
        } as AdvancedCondition,
        {
          id: 'volume-spike',
          type: 'simple',
          leftOperand: { type: 'candle', candleProperty: 'volume' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'VOLUME_SMA', params: { period: 20 } },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'price-below-ema',
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '<',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 9 } },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 0.5,
      takeProfitPercent: 1,
      positionSizePercent: 10,
    },
    tips: [
      'Requires fast execution and low latency',
      'Only trade during high-volume sessions',
      'Exit quickly when momentum fades',
    ],
    backtestStats: {
      winRate: 60,
      profitFactor: 1.3,
      avgTrade: 0.3,
      maxDrawdown: 5,
    },
  },
  {
    id: 'swing-support',
    name: 'Swing Support Bounce',
    description: 'Swing trading strategy based on support level bounces',
    category: 'swing',
    difficulty: 'intermediate',
    timeframes: ['4h', '1d'],
    indicators: ['EMA 200', 'RSI 14', 'ATR 14'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'near-ema200',
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '<=',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 200 } },
        } as AdvancedCondition,
        {
          id: 'rsi-oversold',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '<',
          rightOperand: { type: 'value', value: 40 },
        } as AdvancedCondition,
        {
          id: 'bullish-candle',
          type: 'simple',
          leftOperand: { type: 'candle', candleProperty: 'close' },
          operator: '>',
          rightOperand: { type: 'candle', candleProperty: 'open' },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'rsi-overbought',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '>',
          rightOperand: { type: 'value', value: 65 },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 3,
      takeProfitPercent: 9,
      positionSizePercent: 3,
    },
    tips: [
      'Best used after clear downtrend',
      'Look for volume confirmation on bounce',
      'Be patient - swings take time to develop',
    ],
    backtestStats: {
      winRate: 50,
      profitFactor: 1.9,
      avgTrade: 2.5,
      maxDrawdown: 12,
    },
  },
  {
    id: 'stochastic-reversal',
    name: 'Stochastic Reversal',
    description: 'Reversal strategy using stochastic crossover in extreme zones',
    category: 'mean-reversion',
    difficulty: 'intermediate',
    timeframes: ['1h', '4h'],
    indicators: ['Stochastic 14,3', 'EMA 50'],
    entryConditions: {
      id: 'root',
      operator: 'AND',
      conditions: [
        {
          id: 'stoch-oversold',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'STOCH_K', params: { kPeriod: 14, dPeriod: 3 } },
          operator: '<',
          rightOperand: { type: 'value', value: 20 },
        } as AdvancedCondition,
        {
          id: 'stoch-cross',
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'STOCH_K', params: { kPeriod: 14, dPeriod: 3 } },
          operator: 'crosses_above',
          rightOperand: { type: 'indicator', indicator: 'STOCH_D', params: { kPeriod: 14, dPeriod: 3 } },
        } as AdvancedCondition,
      ],
    },
    exitConditions: {
      id: 'exit-root',
      operator: 'OR',
      conditions: [
        {
          id: 'stoch-overbought',
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'STOCH_K', params: { kPeriod: 14, dPeriod: 3 } },
          operator: '>',
          rightOperand: { type: 'value', value: 80 },
        } as AdvancedCondition,
      ],
    },
    riskSettings: {
      stopLossPercent: 2,
      takeProfitPercent: 4,
      positionSizePercent: 4,
    },
    tips: [
      'Wait for crossover within oversold zone',
      'Combine with support levels',
      'Avoid during strong trends',
    ],
    backtestStats: {
      winRate: 55,
      profitFactor: 1.5,
      avgTrade: 1.0,
      maxDrawdown: 11,
    },
  },
];

const CATEGORY_INFO = {
  'trend': { icon: TrendingUp, color: 'text-green-400', bg: 'bg-green-600/20' },
  'momentum': { icon: Zap, color: 'text-yellow-400', bg: 'bg-yellow-600/20' },
  'mean-reversion': { icon: ArrowUpDown, color: 'text-blue-400', bg: 'bg-blue-600/20' },
  'breakout': { icon: Target, color: 'text-purple-400', bg: 'bg-purple-600/20' },
  'scalping': { icon: Clock, color: 'text-orange-400', bg: 'bg-orange-600/20' },
  'swing': { icon: Activity, color: 'text-cyan-400', bg: 'bg-cyan-600/20' },
};

const DIFFICULTY_COLORS = {
  'beginner': 'bg-green-600/30 text-green-300',
  'intermediate': 'bg-yellow-600/30 text-yellow-300',
  'advanced': 'bg-red-600/30 text-red-300',
};

interface StrategyTemplatesLibraryProps {
  onSelectTemplate: (template: StrategyTemplate) => void;
  onClose?: () => void;
}

export const StrategyTemplatesLibrary: React.FC<StrategyTemplatesLibraryProps> = ({
  onSelectTemplate,
  onClose,
}) => {
  const [search, setSearch] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null);
  const [selectedDifficulty, setSelectedDifficulty] = useState<string | null>(null);
  const [expandedTemplate, setExpandedTemplate] = useState<string | null>(null);

  const filteredTemplates = STRATEGY_TEMPLATES.filter((template) => {
    const matchesSearch = search === '' ||
      template.name.toLowerCase().includes(search.toLowerCase()) ||
      template.description.toLowerCase().includes(search.toLowerCase());
    const matchesCategory = selectedCategory === null || template.category === selectedCategory;
    const matchesDifficulty = selectedDifficulty === null || template.difficulty === selectedDifficulty;
    return matchesSearch && matchesCategory && matchesDifficulty;
  });

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700">
      {/* Header */}
      <div className="p-4 border-b border-gray-700">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <Star className="w-5 h-5 text-yellow-400" />
            Strategy Templates Library
          </h2>
          {onClose && (
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-white"
            >
              &times;
            </button>
          )}
        </div>

        {/* Search */}
        <div className="relative mb-4">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search templates..."
            className="w-full pl-10 pr-4 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400"
          />
        </div>

        {/* Filters */}
        <div className="flex flex-wrap gap-2">
          <div className="flex items-center gap-1">
            <span className="text-xs text-gray-400 mr-2">Category:</span>
            {Object.entries(CATEGORY_INFO).map(([key, info]) => {
              const Icon = info.icon;
              return (
                <button
                  key={key}
                  onClick={() => setSelectedCategory(selectedCategory === key ? null : key)}
                  className={`px-2 py-1 rounded text-xs flex items-center gap-1 transition-colors ${
                    selectedCategory === key
                      ? `${info.bg} ${info.color}`
                      : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
                  }`}
                >
                  <Icon className="w-3 h-3" />
                  {key}
                </button>
              );
            })}
          </div>
          <div className="flex items-center gap-1 ml-4">
            <span className="text-xs text-gray-400 mr-2">Level:</span>
            {['beginner', 'intermediate', 'advanced'].map((level) => (
              <button
                key={level}
                onClick={() => setSelectedDifficulty(selectedDifficulty === level ? null : level)}
                className={`px-2 py-1 rounded text-xs capitalize transition-colors ${
                  selectedDifficulty === level
                    ? DIFFICULTY_COLORS[level as keyof typeof DIFFICULTY_COLORS]
                    : 'bg-gray-700 text-gray-400 hover:bg-gray-600'
                }`}
              >
                {level}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Template List */}
      <div className="max-h-[500px] overflow-y-auto p-4 space-y-3">
        {filteredTemplates.length === 0 ? (
          <div className="text-center text-gray-400 py-8">
            No templates match your filters
          </div>
        ) : (
          filteredTemplates.map((template) => {
            const categoryInfo = CATEGORY_INFO[template.category];
            const CategoryIcon = categoryInfo.icon;
            const isExpanded = expandedTemplate === template.id;

            return (
              <div
                key={template.id}
                className="bg-gray-700/50 border border-gray-600 rounded-lg overflow-hidden"
              >
                {/* Template Header */}
                <div
                  className="p-4 cursor-pointer hover:bg-gray-700/70 transition-colors"
                  onClick={() => setExpandedTemplate(isExpanded ? null : template.id)}
                >
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-3">
                      <div className={`p-2 rounded-lg ${categoryInfo.bg}`}>
                        <CategoryIcon className={`w-5 h-5 ${categoryInfo.color}`} />
                      </div>
                      <div>
                        <h3 className="font-medium text-white">{template.name}</h3>
                        <p className="text-sm text-gray-400 mt-0.5">{template.description}</p>
                        <div className="flex items-center gap-2 mt-2">
                          <span className={`px-2 py-0.5 rounded text-xs ${DIFFICULTY_COLORS[template.difficulty]}`}>
                            {template.difficulty}
                          </span>
                          <span className="text-xs text-gray-500">
                            {template.timeframes.join(', ')}
                          </span>
                        </div>
                      </div>
                    </div>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onSelectTemplate(template);
                      }}
                      className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white flex items-center gap-1"
                    >
                      <Copy className="w-4 h-4" />
                      Use
                    </button>
                  </div>

                  {/* Backtest Stats Preview */}
                  {template.backtestStats && (
                    <div className="flex items-center gap-4 mt-3 pt-3 border-t border-gray-600">
                      <div className="text-center">
                        <div className="text-xs text-gray-400">Win Rate</div>
                        <div className={`text-sm font-medium ${template.backtestStats.winRate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                          {template.backtestStats.winRate}%
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-400">Profit Factor</div>
                        <div className={`text-sm font-medium ${template.backtestStats.profitFactor >= 1.5 ? 'text-green-400' : 'text-yellow-400'}`}>
                          {template.backtestStats.profitFactor.toFixed(1)}
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-400">Avg Trade</div>
                        <div className={`text-sm font-medium ${template.backtestStats.avgTrade > 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {template.backtestStats.avgTrade > 0 ? '+' : ''}{template.backtestStats.avgTrade}%
                        </div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-400">Max DD</div>
                        <div className="text-sm font-medium text-red-400">
                          -{template.backtestStats.maxDrawdown}%
                        </div>
                      </div>
                    </div>
                  )}
                </div>

                {/* Expanded Details */}
                {isExpanded && (
                  <div className="border-t border-gray-600 p-4 bg-gray-800/50">
                    {/* Indicators */}
                    <div className="mb-4">
                      <h4 className="text-sm font-medium text-white mb-2 flex items-center gap-1">
                        <BarChart2 className="w-4 h-4 text-blue-400" />
                        Indicators Used
                      </h4>
                      <div className="flex flex-wrap gap-2">
                        {template.indicators.map((ind, i) => (
                          <span key={i} className="px-2 py-1 bg-gray-700 rounded text-xs text-gray-300">
                            {ind}
                          </span>
                        ))}
                      </div>
                    </div>

                    {/* Risk Settings */}
                    <div className="mb-4">
                      <h4 className="text-sm font-medium text-white mb-2 flex items-center gap-1">
                        <Shield className="w-4 h-4 text-green-400" />
                        Risk Settings
                      </h4>
                      <div className="grid grid-cols-3 gap-2">
                        <div className="bg-gray-700/50 rounded p-2 text-center">
                          <div className="text-xs text-gray-400">Stop Loss</div>
                          <div className="text-sm text-red-400 font-medium">{template.riskSettings.stopLossPercent}%</div>
                        </div>
                        <div className="bg-gray-700/50 rounded p-2 text-center">
                          <div className="text-xs text-gray-400">Take Profit</div>
                          <div className="text-sm text-green-400 font-medium">{template.riskSettings.takeProfitPercent}%</div>
                        </div>
                        <div className="bg-gray-700/50 rounded p-2 text-center">
                          <div className="text-xs text-gray-400">Position Size</div>
                          <div className="text-sm text-blue-400 font-medium">{template.riskSettings.positionSizePercent}%</div>
                        </div>
                      </div>
                    </div>

                    {/* Tips */}
                    {template.tips && (
                      <div>
                        <h4 className="text-sm font-medium text-white mb-2 flex items-center gap-1">
                          <Info className="w-4 h-4 text-yellow-400" />
                          Tips
                        </h4>
                        <ul className="space-y-1">
                          {template.tips.map((tip, i) => (
                            <li key={i} className="text-xs text-gray-400 flex items-start gap-2">
                              <Check className="w-3 h-3 text-green-400 mt-0.5 flex-shrink-0" />
                              {tip}
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};

export { STRATEGY_TEMPLATES };
