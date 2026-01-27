import { useState } from 'react';
import { ChevronDown, ChevronRight, Sparkles, TrendingUp, BarChart2, Activity, Clock, Waves } from 'lucide-react';

// Feature descriptions for tooltips/display
const FEATURE_DESCRIPTIONS: Record<string, string> = {
  // Price features
  return_1c: 'Return over 1 candle (percentage change)',
  return_3c: 'Return over 3 candles',
  return_5c: 'Return over 5 candles',
  return_10c: 'Return over 10 candles',
  return_20c: 'Return over 20 candles',
  body_ratio: 'Candle body size relative to total range',
  upper_wick_ratio: 'Upper wick size relative to body',
  lower_wick_ratio: 'Lower wick size relative to body',
  position_in_range: 'Close position within high-low range',
  gap_percent: 'Gap from previous close (percentage)',
  range_percent: 'High-low range as percentage',
  consecutive_up: 'Count of consecutive bullish candles',
  consecutive_down: 'Count of consecutive bearish candles',
  higher_high: 'Made a higher high than previous',
  lower_low: 'Made a lower low than previous',

  // Volume features
  volume_ratio_5: 'Volume relative to 5-period average',
  volume_ratio_10: 'Volume relative to 10-period average',
  volume_ratio_20: 'Volume relative to 20-period average',
  volume_trend_5: 'Volume trend over 5 periods',
  taker_buy_ratio: 'Taker buy volume / total volume',
  trade_count_value: 'Number of trades in candle',
  avg_trade_size: 'Average trade size',
  obv: 'On-Balance Volume indicator',
  obv_trend: 'OBV trend direction',
  volume_price_trend: 'Volume Price Trend indicator',
  accum_dist: 'Accumulation/Distribution indicator',
  money_flow: 'Money Flow indicator',

  // Volatility features
  atr_7: 'Average True Range (7 period)',
  atr_14: 'Average True Range (14 period)',
  atr_21: 'Average True Range (21 period)',
  atr_percent: 'ATR as percentage of price',
  bollinger_upper: 'Bollinger Band upper value',
  bollinger_lower: 'Bollinger Band lower value',
  bollinger_width: 'Bollinger Band width',
  bollinger_position: 'Price position within Bollinger Bands',
  range_expansion: 'Range expansion indicator',
  volatility_ratio: 'Volatility ratio',

  // Momentum features
  rsi_7: 'RSI (7 period)',
  rsi_14: 'RSI (14 period)',
  rsi_21: 'RSI (21 period)',
  stoch_k: 'Stochastic %K',
  stoch_d: 'Stochastic %D',
  macd_line: 'MACD line value',
  macd_signal: 'MACD signal line value',
  macd_histogram: 'MACD histogram value',
  roc_5: 'Rate of Change (5 period)',
  roc_10: 'Rate of Change (10 period)',
  roc_20: 'Rate of Change (20 period)',
  momentum_5: 'Momentum (5 period)',
  momentum_10: 'Momentum (10 period)',
  williams_r: 'Williams %R indicator',

  // Trend features
  sma_10: 'Simple Moving Average (10 period)',
  sma_20: 'Simple Moving Average (20 period)',
  sma_50: 'Simple Moving Average (50 period)',
  ema_9: 'Exponential Moving Average (9 period)',
  ema_21: 'Exponential Moving Average (21 period)',
  price_vs_sma_20: 'Price relative to SMA(20)',
  sma_cross: 'SMA crossover signal',
  adx: 'Average Directional Index',
  plus_di: 'Positive Directional Indicator',
  minus_di: 'Negative Directional Indicator',
  trend_strength: 'Trend strength indicator',
  trend_direction: 'Trend direction (-1, 0, +1)',

  // Time features
  hour_of_day: 'Hour of day (0-23)',
  day_of_week: 'Day of week (0-6)',
  is_weekend: 'Is weekend (0 or 1)',
  day_of_month: 'Day of month (1-31)',
  is_month_end: 'Is month end (0 or 1)',
};

// Category icons
const CATEGORY_ICONS: Record<string, React.ReactNode> = {
  price: <TrendingUp className="w-4 h-4" />,
  volume: <BarChart2 className="w-4 h-4" />,
  volatility: <Waves className="w-4 h-4" />,
  momentum: <Activity className="w-4 h-4" />,
  trend: <TrendingUp className="w-4 h-4" />,
  time: <Clock className="w-4 h-4" />,
};

export interface FeatureCategories {
  price: string[];
  volume: string[];
  volatility: string[];
  momentum: string[];
  trend: string[];
  time: string[];
}

interface FeatureListProps {
  categories: FeatureCategories;
  totalCount: number;
}

export default function FeatureList({ categories, totalCount }: FeatureListProps) {
  const [expandedCategories, setExpandedCategories] = useState<Set<string>>(new Set(['price']));

  // Guard against undefined categories
  if (!categories) {
    return null;
  }

  const toggleCategory = (category: string) => {
    setExpandedCategories((prev) => {
      const next = new Set(prev);
      if (next.has(category)) {
        next.delete(category);
      } else {
        next.add(category);
      }
      return next;
    });
  };

  const categoryOrder: (keyof FeatureCategories)[] = [
    'price',
    'volume',
    'volatility',
    'momentum',
    'trend',
    'time',
  ];

  return (
    <div className="bg-dark-800 rounded-lg border border-dark-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-dark-700">
        <div className="flex items-center gap-2">
          <Sparkles className="w-5 h-5 text-purple-400" />
          <h3 className="text-lg font-semibold text-white">Available Features</h3>
          <span className="px-2 py-0.5 bg-purple-500/20 text-purple-400 text-xs rounded">
            {totalCount} features
          </span>
        </div>
      </div>

      {/* Feature categories */}
      <div className="divide-y divide-dark-700">
        {categoryOrder.map((category) => {
          const features = categories[category] || [];
          const isExpanded = expandedCategories.has(category);

          return (
            <div key={category}>
              {/* Category header */}
              <button
                onClick={() => toggleCategory(category)}
                className="w-full flex items-center justify-between px-4 py-3 hover:bg-dark-700/50 transition-colors"
              >
                <div className="flex items-center gap-2">
                  <span className="text-gray-400">{CATEGORY_ICONS[category]}</span>
                  <span className="font-medium text-gray-200 capitalize">{category} Features</span>
                  <span className="px-1.5 py-0.5 bg-dark-600 text-gray-400 text-xs rounded">
                    {features.length}
                  </span>
                </div>
                {isExpanded ? (
                  <ChevronDown className="w-4 h-4 text-gray-400" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-gray-400" />
                )}
              </button>

              {/* Feature list */}
              {isExpanded && (
                <div className="px-4 pb-3">
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2">
                    {features.map((feature) => (
                      <div
                        key={feature}
                        className="flex flex-col p-2 bg-dark-700/30 rounded hover:bg-dark-700/50 transition-colors"
                        title={FEATURE_DESCRIPTIONS[feature] || feature}
                      >
                        <code className="text-sm text-primary-400 font-mono">{feature}</code>
                        <span className="text-xs text-gray-500 mt-0.5 line-clamp-1">
                          {FEATURE_DESCRIPTIONS[feature] || 'No description available'}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
