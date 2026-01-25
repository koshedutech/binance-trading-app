// Simple Strategy Group Card
// Story 11.43-11.46: Display strategy group from API response

import React, { useState } from 'react';
import {
  ChevronDown,
  ChevronUp,
  Settings,
  ToggleLeft,
  ToggleRight,
  Clock,
  DollarSign,
  TrendingUp,
  Layers,
  Zap,
} from 'lucide-react';

// Match the actual API response structure
interface StrategyGroupFromAPI {
  id?: string;
  mode: string;
  strategy_group: string;
  enabled: boolean;
  timeframe: string;
  position_size_percent: number;
  max_leverage: number;
  max_positions: number;
  min_volume_usdt: number;
}

interface StrategyGroupCardProps {
  mode: string;
  group: StrategyGroupFromAPI;
  onUpdate?: (updates: Partial<StrategyGroupFromAPI>) => Promise<void>;
}

// Strategy group display names and descriptions
const STRATEGY_GROUP_INFO: Record<string, { name: string; description: string; icon: React.ReactNode }> = {
  breakout: {
    name: 'Breakout',
    description: 'Breakout trading strategies including Ravindra Volume Imbalance',
    icon: <Zap className="w-5 h-5" />,
  },
  trending: {
    name: 'Trending',
    description: 'Trend-following strategies for sustained market moves',
    icon: <TrendingUp className="w-5 h-5" />,
  },
  range: {
    name: 'Range',
    description: 'Range-bound trading strategies for sideways markets',
    icon: <Layers className="w-5 h-5" />,
  },
  volatile: {
    name: 'Volatile',
    description: 'High volatility strategies for extreme market conditions',
    icon: <Settings className="w-5 h-5" />,
  },
};

export function StrategyGroupCard({ mode, group, onUpdate }: StrategyGroupCardProps) {
  const [isExpanded, setIsExpanded] = useState(false);
  const [isUpdating, setIsUpdating] = useState(false);

  const info = STRATEGY_GROUP_INFO[group.strategy_group] || {
    name: group.strategy_group,
    description: 'Strategy group',
    icon: <Settings className="w-5 h-5" />,
  };

  const handleToggle = async () => {
    if (!onUpdate) return;
    setIsUpdating(true);
    try {
      await onUpdate({ enabled: !group.enabled });
    } finally {
      setIsUpdating(false);
    }
  };

  return (
    <div
      className={`
        rounded-lg border transition-all
        ${group.enabled
          ? 'bg-gray-800/50 border-purple-500/30'
          : 'bg-gray-900/50 border-gray-700/50 opacity-60'}
      `}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between p-4 cursor-pointer"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-3">
          <div className={`${group.enabled ? 'text-purple-400' : 'text-gray-500'}`}>
            {info.icon}
          </div>
          <div>
            <h3 className="font-medium text-white flex items-center gap-2">
              {info.name}
              <span
                className={`
                  px-2 py-0.5 text-xs rounded
                  ${group.enabled
                    ? 'bg-green-500/20 text-green-400'
                    : 'bg-gray-600/20 text-gray-500'}
                `}
              >
                {group.enabled ? 'ON' : 'OFF'}
              </span>
            </h3>
            <p className="text-sm text-gray-500">{info.description}</p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          {/* Toggle Button */}
          <button
            onClick={(e) => {
              e.stopPropagation();
              handleToggle();
            }}
            disabled={isUpdating || !onUpdate}
            className={`
              p-1 rounded transition-colors
              ${group.enabled ? 'text-green-400 hover:text-green-300' : 'text-gray-500 hover:text-gray-400'}
              ${isUpdating ? 'opacity-50 cursor-wait' : ''}
            `}
          >
            {group.enabled ? (
              <ToggleRight className="w-6 h-6" />
            ) : (
              <ToggleLeft className="w-6 h-6" />
            )}
          </button>

          {/* Expand Arrow */}
          {isExpanded ? (
            <ChevronUp className="w-5 h-5 text-gray-400" />
          ) : (
            <ChevronDown className="w-5 h-5 text-gray-400" />
          )}
        </div>
      </div>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t border-gray-700/50 p-4 space-y-4">
          {/* Base Settings */}
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
            <div className="flex items-center gap-2 bg-gray-700/30 rounded-lg p-3">
              <Clock className="w-4 h-4 text-gray-400" />
              <div>
                <p className="text-xs text-gray-500">Timeframe</p>
                <p className="text-sm font-medium text-white">{group.timeframe}</p>
              </div>
            </div>

            <div className="flex items-center gap-2 bg-gray-700/30 rounded-lg p-3">
              <DollarSign className="w-4 h-4 text-gray-400" />
              <div>
                <p className="text-xs text-gray-500">Position Size</p>
                <p className="text-sm font-medium text-white">{group.position_size_percent}%</p>
              </div>
            </div>

            <div className="flex items-center gap-2 bg-gray-700/30 rounded-lg p-3">
              <TrendingUp className="w-4 h-4 text-gray-400" />
              <div>
                <p className="text-xs text-gray-500">Max Leverage</p>
                <p className="text-sm font-medium text-white">{group.max_leverage}x</p>
              </div>
            </div>

            <div className="flex items-center gap-2 bg-gray-700/30 rounded-lg p-3">
              <Layers className="w-4 h-4 text-gray-400" />
              <div>
                <p className="text-xs text-gray-500">Max Positions</p>
                <p className="text-sm font-medium text-white">{group.max_positions}</p>
              </div>
            </div>

            <div className="flex items-center gap-2 bg-gray-700/30 rounded-lg p-3">
              <DollarSign className="w-4 h-4 text-gray-400" />
              <div>
                <p className="text-xs text-gray-500">Min Volume</p>
                <p className="text-sm font-medium text-white">
                  ${(group.min_volume_usdt / 1000000).toFixed(1)}M
                </p>
              </div>
            </div>
          </div>

          {/* Sub-strategies info */}
          {group.strategy_group === 'breakout' && (
            <div className="bg-purple-900/20 border border-purple-500/30 rounded-lg p-4">
              <div className="flex items-center gap-2 mb-2">
                <Zap className="w-4 h-4 text-purple-400" />
                <span className="text-sm font-medium text-purple-300">Sub-Strategies</span>
              </div>
              <div className="space-y-2">
                <div className="flex items-center justify-between p-2 bg-gray-800/50 rounded">
                  <div>
                    <span className="text-sm text-white">Ravindra Volume Imbalance</span>
                    <p className="text-xs text-gray-500">3-step pattern: Accumulation → Consolidation → Breakout</p>
                  </div>
                  <span className="px-2 py-0.5 text-xs bg-green-500/20 text-green-400 rounded">
                    Active
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default StrategyGroupCard;
