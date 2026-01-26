import { RefreshCw, TrendingUp, Briefcase, Clock, Target, ArrowUpRight, ArrowDownRight } from 'lucide-react';
import type { CoinProfilerRequirements } from '../../hooks/useCoinProfiler';

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
 * Shows the aggregated timeframes and data fields
 */
function AggregatedInfo({ requirements }: { requirements: CoinProfilerRequirements }) {
  if (!requirements.all_timeframes?.length && !requirements.all_symbols?.length) {
    return null;
  }

  return (
    <div className="mb-4 p-3 bg-gray-800/50 rounded-lg">
      <div className="text-xs text-gray-500 uppercase mb-2">Aggregated Requirements</div>
      <div className="grid grid-cols-2 gap-3">
        <div>
          <div className="text-[10px] text-gray-500 mb-1">Timeframes</div>
          <div className="flex flex-wrap gap-1">
            {requirements.all_timeframes?.length ? (
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

  // Group strategies by mode
  const byMode: Record<string, typeof strategies> = {};
  strategies.forEach(s => {
    s.strategies?.forEach(strat => {
      const mode = strat.mode || 'unknown';
      if (!byMode[mode]) byMode[mode] = [];
      // Check if this strategy is already added for this mode
      const exists = byMode[mode].some(existing =>
        existing.strategies?.some(es => es.sub_strategy === strat.sub_strategy)
      );
      if (!exists) {
        byMode[mode].push(s);
      }
    });
  });

  return (
    <div className="p-3 bg-gray-800/30 rounded-lg">
      <SectionHeader icon={TrendingUp} title="FROM ENABLED STRATEGIES" count={requirements.strategy_count || 0} color="text-purple-400" />

      {Object.keys(byMode).length > 0 ? (
        <div className="space-y-2">
          {Object.entries(byMode).map(([mode, sources]) => (
            <div key={mode} className="pl-2 border-l-2 border-purple-500/30">
              <div className="text-xs text-purple-300 font-medium mb-1">{formatMode(mode)} Mode</div>
              {sources.map((source, idx) => (
                <div key={idx} className="flex items-start gap-2 text-xs mb-1">
                  {source.strategies?.map((strat, sidx) => (
                    <div key={sidx} className="flex items-center gap-1">
                      <span className="text-gray-400">{formatStrategyName(strat.sub_strategy)}</span>
                      {source.timeframes?.length > 0 && (
                        <span className="text-gray-600">
                          ({source.timeframes.join(', ')})
                        </span>
                      )}
                    </div>
                  ))}
                </div>
              ))}
            </div>
          ))}
        </div>
      ) : requirements.strategy_count > 0 ? (
        <div className="text-xs text-gray-400">
          {requirements.strategy_count} strategies enabled - collecting data for all watchlist symbols
        </div>
      ) : null}

      {requirements.all_timeframes?.length > 0 && (
        <div className="mt-2 flex items-center gap-1 text-[10px] text-gray-500">
          <Clock className="w-3 h-3" />
          <span>Timeframe needs: {requirements.all_timeframes.join(', ')}</span>
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
