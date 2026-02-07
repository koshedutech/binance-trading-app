import { RefreshCw, TrendingUp, Briefcase, Clock, Target, ArrowUpRight, ArrowDownRight, Lock, CheckCircle, AlertTriangle } from 'lucide-react';
import type { CoinProfilerRequirements, StrategyRef, StrategyCapacityInfo } from '../../hooks/useCoinProfiler';

// ============================================================================
// Epic 14: Coin Profiler Requirements Breakdown
// Story 14.7: Frontend UI Component for Coin Profiler
// ============================================================================
// This component displays the subscription sources as per the Epic 14 wireframe:
// - FROM ENABLED STRATEGIES: Shows strategies that are driving data collection
// - FROM OPEN POSITIONS: Shows positions that need exit monitoring
// ============================================================================

interface RequirementsBreakdownProps {
  requirements: CoinProfilerRequirements | null;
  isLoading: boolean;
}

/**
 * Formats a strategy sub_strategy name for display
 */
function formatStrategyName(subStrategy: string): string {
  return subStrategy
    .split('_')
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(' ');
}

/**
 * Formats mode name for display
 */
function formatMode(mode: string): string {
  const modeMap: Record<string, string> = {
    'scalp': 'Scalp',
    'swing': 'Swing',
    'position': 'Position',
    'ultra_fast': 'Ultra Fast',
  };
  return modeMap[mode] || mode;
}

/**
 * Position limit badge component
 * Shows current/max positions with status indicator
 */
function PositionLimitBadge({ strat }: { strat: StrategyRef }) {
  // If capacity info is not available, don't show badge
  if (strat.current_positions === undefined || strat.max_positions === undefined) {
    return null;
  }

  const current = strat.current_positions;
  const max = strat.max_positions;
  const status = strat.capacity_status || 'available';

  // Determine colors and icon based on status
  const getStatusStyles = () => {
    switch (status) {
      case 'at_limit':
        return {
          bgColor: 'bg-red-500/20',
          textColor: 'text-red-400',
          borderColor: 'border-red-500/30',
          Icon: Lock,
          label: 'AT LIMIT',
        };
      case 'limited':
        return {
          bgColor: 'bg-yellow-500/20',
          textColor: 'text-yellow-400',
          borderColor: 'border-yellow-500/30',
          Icon: AlertTriangle,
          label: 'LIMITED',
        };
      default:
        return {
          bgColor: 'bg-green-500/20',
          textColor: 'text-green-400',
          borderColor: 'border-green-500/30',
          Icon: CheckCircle,
          label: 'Available',
        };
    }
  };

  const styles = getStatusStyles();
  const Icon = styles.Icon;

  return (
    <div className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded ${styles.bgColor} ${styles.textColor} text-[10px] border ${styles.borderColor}`}>
      <Icon className="w-3 h-3" />
      <span>{current}/{max}</span>
      {status === 'at_limit' && <span className="font-medium">[PAUSED]</span>}
    </div>
  );
}

/**
 * Capacity summary section showing all strategies' position limits
 */
function CapacitySummary({ capacitySummary }: { capacitySummary?: StrategyCapacityInfo[] }) {
  if (!capacitySummary || capacitySummary.length === 0) {
    return null;
  }

  // Check if any strategy is at limit
  const atLimitCount = capacitySummary.filter(c => c.capacity_status === 'at_limit').length;
  const limitedCount = capacitySummary.filter(c => c.capacity_status === 'limited').length;

  return (
    <div className="p-3 bg-gray-800/30 rounded-lg">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Target className="w-4 h-4 text-cyan-400" />
          <span className="text-sm font-medium text-gray-300">POSITION CAPACITY</span>
        </div>
        {atLimitCount > 0 && (
          <span className="px-1.5 py-0.5 rounded text-xs bg-red-500/20 text-red-400">
            {atLimitCount} at limit
          </span>
        )}
        {limitedCount > 0 && atLimitCount === 0 && (
          <span className="px-1.5 py-0.5 rounded text-xs bg-yellow-500/20 text-yellow-400">
            {limitedCount} limited
          </span>
        )}
      </div>

      <div className="space-y-1.5">
        {capacitySummary.map((cap, idx) => {
          const statusStyles = cap.capacity_status === 'at_limit'
            ? { bg: 'bg-red-500/10', border: 'border-red-500/30', text: 'text-red-400', icon: Lock }
            : cap.capacity_status === 'limited'
            ? { bg: 'bg-yellow-500/10', border: 'border-yellow-500/30', text: 'text-yellow-400', icon: AlertTriangle }
            : { bg: 'bg-green-500/10', border: 'border-green-500/30', text: 'text-green-400', icon: CheckCircle };

          const StatusIcon = statusStyles.icon;

          return (
            <div
              key={idx}
              className={`flex items-center justify-between text-xs rounded px-2 py-1.5 border ${statusStyles.bg} ${statusStyles.border}`}
            >
              <div className="flex items-center gap-2">
                <StatusIcon className={`w-3 h-3 ${statusStyles.text}`} />
                <span className="text-gray-300">{formatStrategyName(cap.sub_strategy)}</span>
                <span className="text-gray-500">({formatMode(cap.mode)})</span>
              </div>
              <div className="flex items-center gap-2">
                <span className={`font-medium ${statusStyles.text}`}>
                  {cap.current_positions}/{cap.max_positions}
                </span>
                {cap.capacity_status === 'at_limit' && (
                  <span className="text-red-400 font-medium">PAUSED</span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Section header component
 */
function SectionHeader({ icon: Icon, title, count, color }: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  count: number;
  color: string;
}) {
  return (
    <div className="flex items-center gap-2 mb-2">
      <Icon className={`w-4 h-4 ${color}`} />
      <span className="text-sm font-medium text-gray-300">{title}</span>
      <span className={`px-1.5 py-0.5 rounded text-xs ${color.includes('purple') ? 'bg-purple-500/20 text-purple-400' : 'bg-green-500/20 text-green-400'}`}>
        {count}
      </span>
    </div>
  );
}

/**
 * Get color for trading mode
 */
function getModeColor(mode: string): { bg: string; text: string } {
  const colors: Record<string, { bg: string; text: string }> = {
    'scalp': { bg: 'bg-orange-500/20', text: 'text-orange-400' },
    'swing': { bg: 'bg-blue-500/20', text: 'text-blue-400' },
    'position': { bg: 'bg-purple-500/20', text: 'text-purple-400' },
    'ultra_fast': { bg: 'bg-pink-500/20', text: 'text-pink-400' },
  };
  return colors[mode] || { bg: 'bg-gray-500/20', text: 'text-gray-400' };
}

/**
 * Shows the aggregated timeframes (with mode info) and symbols
 */
function AggregatedInfo({ requirements }: { requirements: CoinProfilerRequirements }) {
  if (!requirements.all_timeframes?.length && !requirements.all_symbols?.length) {
    return null;
  }

  // Use timeframes_by_mode if available for clearer display
  const timeframesByMode = requirements.timeframes_by_mode;
  const hasTimeframesByMode = timeframesByMode && Object.keys(timeframesByMode).length > 0;

  return (
    <div className="mb-4 p-3 bg-gray-800/50 rounded-lg">
      <div className="text-xs text-gray-500 uppercase mb-2">Aggregated Requirements</div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <div className="text-[10px] text-gray-500 mb-1">Timeframes by Mode</div>
          <div className="flex flex-wrap gap-1">
            {hasTimeframesByMode ? (
              // Display timeframes grouped by mode with color coding
              Object.entries(timeframesByMode).flatMap(([mode, timeframes]) => {
                const colors = getModeColor(mode);
                return timeframes.map(tf => (
                  <span
                    key={`${mode}-${tf}`}
                    className={`px-1.5 py-0.5 ${colors.bg} ${colors.text} rounded text-xs`}
                    title={`${formatMode(mode)} mode requires ${tf} timeframe`}
                  >
                    {formatMode(mode)}: {tf}
                  </span>
                ));
              })
            ) : requirements.all_timeframes?.length ? (
              // Fallback to simple list if timeframes_by_mode not available
              requirements.all_timeframes.map(tf => (
                <span key={tf} className="px-1.5 py-0.5 bg-blue-500/20 text-blue-400 rounded text-xs">
                  {tf}
                </span>
              ))
            ) : (
              <span className="text-xs text-gray-500">None</span>
            )}
          </div>
        </div>
        <div>
          <div className="text-[10px] text-gray-500 mb-1">Symbols</div>
          <div className="flex flex-wrap gap-1 max-h-20 overflow-y-auto">
            {requirements.all_symbols?.length ? (
              requirements.all_symbols.slice(0, 10).map(symbol => (
                <span key={symbol} className="px-1.5 py-0.5 bg-cyan-500/20 text-cyan-400 rounded text-xs">
                  {symbol}
                </span>
              ))
            ) : (
              <span className="text-xs text-gray-500">None</span>
            )}
            {requirements.all_symbols?.length > 10 && (
              <span className="text-xs text-gray-500">+{requirements.all_symbols.length - 10} more</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Displays strategy sources section
 */
function StrategySources({ requirements }: { requirements: CoinProfilerRequirements }) {
  const strategies = requirements.from_strategies || [];
  const timeframesByMode = requirements.timeframes_by_mode || {};

  if (strategies.length === 0 && requirements.strategy_count === 0) {
    return (
      <div className="p-3 bg-gray-800/30 rounded-lg">
        <SectionHeader icon={TrendingUp} title="FROM ENABLED STRATEGIES" count={0} color="text-purple-400" />
        <div className="text-xs text-gray-500 italic">
          No strategies enabled. Enable strategies in Mode Strategy Settings to start scanning for entries.
        </div>
      </div>
    );
  }

  // Group strategies by mode, collecting unique strategies with their capacity info
  const byMode: Record<string, StrategyRef[]> = {};
  strategies.forEach(s => {
    s.strategies?.forEach(strat => {
      const mode = strat.mode || 'unknown';
      if (!byMode[mode]) byMode[mode] = [];
      // Check if this strategy is already added for this mode
      const exists = byMode[mode].some(es => es.sub_strategy === strat.sub_strategy);
      if (!exists) {
        byMode[mode].push(strat);
      }
    });
  });

  return (
    <div className="p-3 bg-gray-800/30 rounded-lg">
      <SectionHeader icon={TrendingUp} title="FROM ENABLED STRATEGIES" count={requirements.strategy_count || 0} color="text-purple-400" />

      {Object.keys(byMode).length > 0 ? (
        <div className="space-y-2">
          {Object.entries(byMode).map(([mode, modeStrategies]) => {
            const modeColors = getModeColor(mode);
            const modeTimeframes = timeframesByMode[mode] || [];

            return (
              <div key={mode} className={`pl-2 border-l-2 ${modeColors.text.replace('text-', 'border-')}/30`}>
                <div className="flex items-center gap-2 mb-1">
                  <span className={`text-xs font-medium ${modeColors.text}`}>
                    {formatMode(mode)} Mode
                  </span>
                  {modeTimeframes.length > 0 && (
                    <span className={`px-1.5 py-0.5 rounded text-[10px] ${modeColors.bg} ${modeColors.text}`}>
                      {modeTimeframes.join(', ')}
                    </span>
                  )}
                </div>
                {modeStrategies.map((strat, sidx) => (
                  <div key={sidx} className="flex items-center justify-between text-xs mb-1.5 bg-gray-800/30 rounded px-2 py-1">
                    <div className="flex items-center gap-2">
                      <span className="text-gray-300">{formatStrategyName(strat.sub_strategy)}</span>
                    </div>
                    <PositionLimitBadge strat={strat} />
                  </div>
                ))}
              </div>
            );
          })}
        </div>
      ) : requirements.strategy_count > 0 ? (
        <div className="text-xs text-gray-400">
          {requirements.strategy_count} strategies enabled - collecting data for all watchlist symbols
        </div>
      ) : null}

      {/* Show warning when all strategies are at capacity */}
      {Object.keys(byMode).length > 0 && (() => {
        const allStrats = Object.values(byMode).flat();
        const allAtLimit = allStrats.length > 0 && allStrats.every(s => s.capacity_status === 'at_limit');
        if (!allAtLimit) return null;
        return (
          <div className="mt-2 p-2 bg-red-900/20 border border-red-500/30 rounded text-xs text-red-400 flex items-center gap-2">
            <Lock className="w-4 h-4 flex-shrink-0" />
            <span>All strategies at capacity. No new entries will be taken until a position closes.</span>
          </div>
        );
      })()}

      {/* Show timeframes by mode summary */}
      {Object.keys(timeframesByMode).length > 0 && (
        <div className="mt-2 pt-2 border-t border-gray-700/50">
          <div className="flex items-center gap-1 text-[10px] text-gray-500 mb-1">
            <Clock className="w-3 h-3" />
            <span>Timeframe demand by mode:</span>
          </div>
          <div className="flex flex-wrap gap-1">
            {Object.entries(timeframesByMode).map(([mode, tfs]) => {
              const colors = getModeColor(mode);
              return tfs.map(tf => (
                <span
                  key={`${mode}-${tf}`}
                  className={`px-1.5 py-0.5 rounded text-[10px] ${colors.bg} ${colors.text}`}
                >
                  {formatMode(mode)}: {tf}
                </span>
              ));
            })}
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Displays position sources section
 */
function PositionSources({ requirements }: { requirements: CoinProfilerRequirements }) {
  const positions = requirements.from_positions || [];

  if (positions.length === 0) {
    return (
      <div className="p-3 bg-gray-800/30 rounded-lg">
        <SectionHeader icon={Briefcase} title="FROM OPEN POSITIONS" count={0} color="text-green-400" />
        <div className="text-xs text-gray-500 italic">
          No open positions. Position data will be collected when you have active trades.
        </div>
      </div>
    );
  }

  return (
    <div className="p-3 bg-gray-800/30 rounded-lg">
      <SectionHeader icon={Briefcase} title="FROM OPEN POSITIONS" count={positions.length} color="text-green-400" />

      <div className="space-y-1.5">
        {positions.map((pos, idx) => (
          <div key={idx} className="flex items-center justify-between text-xs bg-gray-800/50 rounded px-2 py-1.5">
            <div className="flex items-center gap-2">
              {pos.side === 'LONG' ? (
                <ArrowUpRight className="w-3 h-3 text-green-400" />
              ) : (
                <ArrowDownRight className="w-3 h-3 text-red-400" />
              )}
              <span className="font-medium text-gray-200">{pos.symbol}</span>
              <span className="text-gray-500">{formatMode(pos.mode)}</span>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-gray-500">
                {pos.timeframes?.join(', ') || 'default'}
              </span>
              {pos.exit_mode && (
                <span className={`px-1.5 py-0.5 rounded text-[10px] ${
                  pos.exit_mode === 'trailing' ? 'bg-yellow-500/20 text-yellow-400' :
                  pos.exit_mode === 'both' ? 'bg-purple-500/20 text-purple-400' :
                  'bg-blue-500/20 text-blue-400'
                }`}>
                  {pos.exit_mode === 'tp_sl' ? 'TP/SL' :
                   pos.exit_mode === 'trailing' ? 'Trailing' :
                   pos.exit_mode === 'both' ? 'TP+Trail' : pos.exit_mode}
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

/**
 * Main Requirements Breakdown component
 * Shows the subscription sources per the Epic 14 wireframe
 */
export default function RequirementsBreakdown({ requirements, isLoading }: RequirementsBreakdownProps) {
  if (isLoading && !requirements) {
    return (
      <div className="p-4 text-center text-gray-500">
        <RefreshCw className="w-5 h-5 animate-spin mx-auto mb-2" />
        <span className="text-sm">Loading requirements...</span>
      </div>
    );
  }

  if (!requirements) {
    return (
      <div className="p-4 text-center text-gray-500">
        <Target className="w-5 h-5 mx-auto mb-2" />
        <span className="text-sm">No requirements data available</span>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Aggregated Info */}
      <AggregatedInfo requirements={requirements} />

      {/* Capacity Summary - Show position limits for all strategies */}
      <CapacitySummary capacitySummary={requirements.capacity_summary} />

      {/* Strategy Sources */}
      <StrategySources requirements={requirements} />

      {/* Position Sources */}
      <PositionSources requirements={requirements} />

      {/* Summary */}
      {(requirements.strategy_count > 0 || requirements.position_count > 0) && (
        <div className="text-[10px] text-gray-500 text-center pt-2 border-t border-gray-700">
          Collecting data for {requirements.all_symbols?.length || 0} symbols across {requirements.all_timeframes?.length || 0} timeframes
        </div>
      )}
    </div>
  );
}
