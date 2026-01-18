import { formatUSD } from '../../services/futuresApi';
import { TrendingUp, TrendingDown, DollarSign, Activity, Clock, Database } from 'lucide-react';

interface DayDetail {
  date: string;
  day: number;
  day_name: string;
  pnl: number;
  commission: number;
  rebate?: number;
  funding: number;
  other?: number;
  net_pnl: number;
  trade_count: number;
  is_profit: boolean;
  is_today: boolean;
  is_cached?: boolean;
}

interface DayDetailPanelProps {
  selectedDay: DayDetail | null;
  onClose: () => void;
}

export default function DayDetailPanel({ selectedDay, onClose }: DayDetailPanelProps) {
  if (!selectedDay) return null;

  const {
    date,
    // day not needed - we use day_name for display
    day_name,
    pnl,
    commission,
    rebate = 0,
    funding,
    other = 0,
    net_pnl,
    trade_count,
    is_profit,
    is_today,
    is_cached = false,
  } = selectedDay;

  const PnLIcon = is_profit ? TrendingUp : TrendingDown;
  const pnlColor = is_profit ? 'text-green-400' : 'text-red-400';
  const bgGradient = is_profit ? 'from-green-900/40' : 'from-red-900/40';

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden min-w-[220px]">
      {/* Header */}
      <div className={`px-3 py-2 border-b border-gray-700 bg-gradient-to-r ${bgGradient} to-gray-800`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <PnLIcon className={`w-4 h-4 ${pnlColor}`} />
            <div>
              <div className="text-sm font-semibold text-white">
                {is_today ? 'TODAY' : day_name}
              </div>
              <div className="text-xs text-gray-400">{date}</div>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-500 hover:text-white p-1"
          >
            x
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="p-3 space-y-3">
        {/* Net P&L (big) */}
        <div className="text-center pb-3 border-b border-gray-700">
          <div className={`text-2xl font-bold ${pnlColor}`}>
            {is_profit ? '+' : ''}{formatUSD(net_pnl)}
          </div>
          <div className="text-[10px] text-gray-500 uppercase">Net P&L</div>
        </div>

        {/* Breakdown */}
        <div className="space-y-2 text-xs">
          {/* Gross P&L */}
          <div className="flex items-center justify-between">
            <span className="text-gray-400">Gross P&L</span>
            <span className={pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
              {pnl >= 0 ? '+' : ''}{formatUSD(pnl)}
            </span>
          </div>

          {/* Commission */}
          <div className="flex items-center justify-between">
            <span className="text-gray-400">Commission</span>
            <span className="text-yellow-400">-{formatUSD(commission)}</span>
          </div>

          {/* Rebate (if any) */}
          {rebate !== 0 && (
            <div className="flex items-center justify-between">
              <span className="text-gray-400">Rebate</span>
              <span className="text-green-400">+{formatUSD(rebate)}</span>
            </div>
          )}

          {/* Funding */}
          <div className="flex items-center justify-between">
            <span className="text-gray-400">Funding Fee</span>
            <span className={funding <= 0 ? 'text-green-400' : 'text-yellow-400'}>
              {funding <= 0 ? '+' : '-'}{formatUSD(Math.abs(funding))}
            </span>
          </div>

          {/* Other (if any) */}
          {other !== 0 && (
            <div className="flex items-center justify-between">
              <span className="text-gray-400">Other</span>
              <span className={other >= 0 ? 'text-green-400' : 'text-red-400'}>
                {other >= 0 ? '+' : ''}{formatUSD(other)}
              </span>
            </div>
          )}
        </div>

        {/* Trade count & status */}
        <div className="pt-2 border-t border-gray-700 flex items-center justify-between text-xs">
          <div className="flex items-center gap-1 text-gray-400">
            <Activity className="w-3 h-3" />
            <span>{trade_count} trades</span>
          </div>
          <div className="flex items-center gap-1 text-gray-500">
            {is_cached ? (
              <>
                <Database className="w-3 h-3" />
                <span>Cached</span>
              </>
            ) : (
              <>
                <Clock className="w-3 h-3" />
                <span>Live</span>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
