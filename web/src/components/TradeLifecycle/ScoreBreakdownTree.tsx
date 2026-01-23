// Epic 11: Story 11.40 - Gap Analysis UI
// ScoreBreakdownTree component shows hierarchical score breakdown with visual progress
import { useState } from 'react';
import { ChevronDown, ChevronRight, BarChart2, Brain, History, Layers } from 'lucide-react';
import { ScoreBreakdownDetailed } from '../../services/futuresApi';

interface ScoreBreakdownTreeProps {
  breakdown: ScoreBreakdownDetailed;
  compact?: boolean;
}

interface ScoreRowProps {
  label: string;
  value: number;
  max: number;
  color: string;
  indent?: boolean;
}

function ScoreRow({ label, value, max, color, indent = false }: ScoreRowProps) {
  const percentage = max > 0 ? (value / max) * 100 : 0;

  return (
    <div className={`flex items-center gap-2 ${indent ? 'ml-4' : ''}`}>
      <span className="text-xs text-gray-400 w-24 truncate">{label}</span>
      <div className="flex-1 bg-gray-700 rounded-full h-1.5 min-w-[40px]">
        <div
          className={`h-1.5 rounded-full transition-all ${color}`}
          style={{ width: `${Math.min(100, percentage)}%` }}
        />
      </div>
      <span className="text-xs text-gray-300 w-12 text-right font-mono">
        {value}/{max}
      </span>
    </div>
  );
}

interface ScoreSectionProps {
  title: string;
  icon: React.ReactNode;
  value: number;
  max: number;
  color: string;
  children: React.ReactNode;
  defaultExpanded?: boolean;
}

function ScoreSection({ title, icon, value, max, color, children, defaultExpanded = false }: ScoreSectionProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const percentage = max > 0 ? (value / max) * 100 : 0;

  return (
    <div className="border-b border-gray-700/50 last:border-b-0">
      <button
        type="button"
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 py-2 hover:bg-gray-700/20 transition-colors"
      >
        {expanded ? (
          <ChevronDown className="w-4 h-4 text-gray-500" />
        ) : (
          <ChevronRight className="w-4 h-4 text-gray-500" />
        )}
        <span className={`${color.replace('bg-', 'text-').replace('-500', '-400')}`}>
          {icon}
        </span>
        <span className="text-sm text-gray-200 flex-1 text-left">{title}</span>
        <div className="w-20 bg-gray-700 rounded-full h-2">
          <div
            className={`h-2 rounded-full transition-all ${color}`}
            style={{ width: `${Math.min(100, percentage)}%` }}
          />
        </div>
        <span className="text-sm text-gray-300 w-12 text-right font-mono">
          {value}/{max}
        </span>
      </button>
      {expanded && (
        <div className="pb-2 space-y-1.5">
          {children}
        </div>
      )}
    </div>
  );
}

export default function ScoreBreakdownTree({ breakdown, compact = false }: ScoreBreakdownTreeProps) {
  if (compact) {
    return (
      <div className="space-y-2">
        <ScoreRow label="Technical" value={breakdown.technical} max={breakdown.technical_max} color="bg-blue-500" />
        <ScoreRow label="Context" value={breakdown.context} max={breakdown.context_max} color="bg-green-500" />
        <ScoreRow label="LLM" value={breakdown.llm} max={breakdown.llm_max} color="bg-purple-500" />
        <ScoreRow label="History" value={breakdown.history} max={breakdown.history_max} color="bg-yellow-500" />
      </div>
    );
  }

  return (
    <div className="bg-gray-900/50 rounded-lg border border-gray-700/50">
      {/* Technical Section */}
      <ScoreSection
        title="Technical"
        icon={<BarChart2 className="w-4 h-4" />}
        value={breakdown.technical}
        max={breakdown.technical_max}
        color="bg-blue-500"
        defaultExpanded
      >
        <ScoreRow
          label="Trend Align"
          value={breakdown.trend_alignment}
          max={breakdown.trend_alignment_max}
          color="bg-blue-400"
          indent
        />
        <ScoreRow
          label="Momentum"
          value={breakdown.momentum}
          max={breakdown.momentum_max}
          color="bg-blue-400"
          indent
        />
        <ScoreRow
          label="Volatility"
          value={breakdown.volatility}
          max={breakdown.volatility_max}
          color="bg-blue-400"
          indent
        />
        <ScoreRow
          label="Volume"
          value={breakdown.volume}
          max={breakdown.volume_max}
          color="bg-blue-400"
          indent
        />
      </ScoreSection>

      {/* Context Section */}
      <ScoreSection
        title="Context"
        icon={<Layers className="w-4 h-4" />}
        value={breakdown.context}
        max={breakdown.context_max}
        color="bg-green-500"
      >
        <ScoreRow
          label="Regime Match"
          value={breakdown.regime_match}
          max={breakdown.regime_match_max}
          color="bg-green-400"
          indent
        />
        <ScoreRow
          label="TF Align"
          value={breakdown.timeframe_align}
          max={breakdown.timeframe_align_max}
          color="bg-green-400"
          indent
        />
        <ScoreRow
          label="BTC Trend"
          value={breakdown.btc_trend}
          max={breakdown.btc_trend_max}
          color="bg-green-400"
          indent
        />
      </ScoreSection>

      {/* LLM Section */}
      <ScoreSection
        title="LLM Confidence"
        icon={<Brain className="w-4 h-4" />}
        value={breakdown.llm}
        max={breakdown.llm_max}
        color="bg-purple-500"
      >
        <div className="ml-4 text-xs text-gray-500 py-1">
          AI confidence score from analysis
        </div>
      </ScoreSection>

      {/* History Section */}
      <ScoreSection
        title="History"
        icon={<History className="w-4 h-4" />}
        value={breakdown.history}
        max={breakdown.history_max}
        color="bg-yellow-500"
      >
        <ScoreRow
          label="Symbol WR"
          value={breakdown.symbol_winrate}
          max={breakdown.symbol_winrate_max}
          color="bg-yellow-400"
          indent
        />
        <ScoreRow
          label="Strategy WR"
          value={breakdown.strategy_winrate}
          max={breakdown.strategy_winrate_max}
          color="bg-yellow-400"
          indent
        />
      </ScoreSection>

      {/* Final Score Summary */}
      <div className="p-3 bg-gray-800/50 rounded-b-lg">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-sm text-gray-300">Final Score</span>
            {breakdown.gap_to_threshold > 0 ? (
              <span className="text-xs px-2 py-0.5 rounded bg-red-500/20 text-red-400">
                Gap: {breakdown.gap_to_threshold}
              </span>
            ) : (
              <span className="text-xs px-2 py-0.5 rounded bg-green-500/20 text-green-400">
                Above Threshold
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <span className={`text-2xl font-bold ${
              breakdown.final >= breakdown.threshold ? 'text-green-400' :
              breakdown.final >= breakdown.threshold - 10 ? 'text-yellow-400' :
              'text-red-400'
            }`}>
              {breakdown.final}
            </span>
            <span className="text-gray-500">/ {breakdown.threshold}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
