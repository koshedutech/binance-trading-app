import { useEffect, useState, useCallback, useRef } from 'react';
import { useFuturesStore, selectAvailableBalance, selectTotalMarginUsed, selectTotalUnrealizedPnl, selectActivePositions } from '../store/futuresStore';
import { formatUSD, getPositionColor, futuresApi } from '../services/futuresApi';
import { wsService } from '../services/websocket';
import CollapsibleCard from './CollapsibleCard';
import type { WSEvent, GinieStatusPayload } from '../types';
import {
  Wallet,
  TrendingUp,
  DollarSign,
  Activity,
  LayoutDashboard,
  Wifi,
  Users,
} from 'lucide-react';

interface AutopilotStats {
  active_positions: number;
  max_positions: number;
  running: boolean;
  unrealized_pnl: number;
}


export default function AccountStatsCard() {
  const accountInfo = useFuturesStore((state) => state.accountInfo);
  const availableBalance = useFuturesStore(selectAvailableBalance);
  const marginUsed = useFuturesStore(selectTotalMarginUsed);
  const totalUnrealizedPnl = useFuturesStore(selectTotalUnrealizedPnl);
  const activePositions = useFuturesStore(selectActivePositions);

  const [autopilotStats, setAutopilotStats] = useState<AutopilotStats | null>(null);
  const [wsConnected, setWsConnected] = useState(() => wsService.isConnected());

  // Safely parse values
  const safeNum = (val: number | string | null | undefined): number => {
    if (val === null || val === undefined) return 0;
    const num = typeof val === 'string' ? parseFloat(val) : val;
    return isNaN(num) ? 0 : num;
  };

  const walletBalance = safeNum(accountInfo?.total_wallet_balance);
  const marginBalance = walletBalance + totalUnrealizedPnl;

  const fetchAutopilotStatus = useCallback(async () => {
    try {
      const data = await futuresApi.getGinieAutopilotStatus();
      if (data?.stats) {
        setAutopilotStats({
          active_positions: data.stats.active_positions ?? 0,
          max_positions: data.stats.max_positions ?? 10,
          running: data.stats.running ?? false,
          unrealized_pnl: data.stats.unrealized_pnl ?? 0,
        });
      }
    } catch (err) {
      console.error('Failed to fetch autopilot status:', err);
    }
  }, []);


  // Ref for fallback interval
  const fallbackRef = useRef<NodeJS.Timeout | null>(null);

  // WebSocket subscription for real-time updates
  useEffect(() => {
    const handleGinieUpdate = (event: WSEvent) => {
      const status = event.data.status as GinieStatusPayload;
      if (status) {
        fetchAutopilotStatus();
      }
    };

    // Listen for mode config changes from GiniePanel
    const handleModeConfigUpdate = () => {
      fetchAutopilotStatus();
    };

    wsService.subscribe('GINIE_STATUS_UPDATE', handleGinieUpdate);
    window.addEventListener('mode-config-updated', handleModeConfigUpdate);

    // Fallback polling when WebSocket disconnected
    const startFallback = () => {
      if (!fallbackRef.current) {
        fallbackRef.current = setInterval(() => {
          fetchAutopilotStatus();
        }, 60000);
      }
    };

    const stopFallback = () => {
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
      fetchAutopilotStatus();
    };

    wsService.onDisconnect(startFallback);
    wsService.onConnect(stopFallback);

    if (!wsService.isConnected()) {
      startFallback();
    }

    // Initial fetch
    fetchAutopilotStatus();

    return () => {
      wsService.unsubscribe('GINIE_STATUS_UPDATE', handleGinieUpdate);
      window.removeEventListener('mode-config-updated', handleModeConfigUpdate);
      if (fallbackRef.current) {
        clearInterval(fallbackRef.current);
        fallbackRef.current = null;
      }
    };
  }, [fetchAutopilotStatus]);

  // Track WebSocket connection status
  useEffect(() => {
    const handleConnect = () => setWsConnected(true);
    const handleDisconnect = () => setWsConnected(false);
    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);
    setWsConnected(wsService.isConnected());
  }, []);

  // Position counts
  const currentPositions = autopilotStats?.active_positions ?? activePositions.length;
  const maxPositions = autopilotStats?.max_positions ?? 10;

  return (
    <CollapsibleCard
      title="Account Overview"
      icon={<LayoutDashboard className="w-4 h-4" />}
      defaultExpanded={false}
      badge="Stats"
      badgeColor="cyan"
    >
      <div className="space-y-4">
        {/* Account Balances + Positions (6 columns) */}
        <div className="grid grid-cols-6 gap-3">
          {/* Wallet Balance */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <Wallet className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs text-gray-500">Wallet</span>
            </div>
            <div className="text-lg font-bold text-white">{formatUSD(walletBalance)}</div>
          </div>

          {/* Margin Balance */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <TrendingUp className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs text-gray-500">Margin</span>
            </div>
            <div className={`text-lg font-bold ${
              marginBalance > walletBalance ? 'text-green-500' : marginBalance < walletBalance ? 'text-red-500' : 'text-white'
            }`}>
              {formatUSD(marginBalance)}
            </div>
          </div>

          {/* Available Balance */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <DollarSign className="w-3.5 h-3.5 text-green-400" />
              <span className="text-xs text-gray-500">Available</span>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>
            <div className="text-lg font-bold text-green-500">{formatUSD(availableBalance)}</div>
          </div>

          {/* Margin Used */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <Activity className="w-3.5 h-3.5 text-yellow-400" />
              <span className="text-xs text-gray-500">Margin Used</span>
            </div>
            <div className="text-lg font-bold text-yellow-500">{formatUSD(marginUsed)}</div>
          </div>

          {/* Unrealized PnL */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <TrendingUp className="w-3.5 h-3.5 text-gray-400" />
              <span className="text-xs text-gray-500">Unrealized</span>
              {wsConnected && <Wifi className="w-3 h-3 text-green-500" title="Real-time" />}
            </div>
            <div className={`text-lg font-bold ${getPositionColor(totalUnrealizedPnl)}`}>
              {formatUSD(totalUnrealizedPnl)}
            </div>
          </div>

          {/* Positions */}
          <div className="bg-gray-800 rounded-lg p-3 border border-gray-700">
            <div className="flex items-center gap-1.5 mb-1">
              <Users className="w-3.5 h-3.5 text-purple-400" />
              <span className="text-xs text-gray-500">Positions</span>
            </div>
            <div className="text-lg font-bold text-purple-400">
              {currentPositions}/{maxPositions}
            </div>
          </div>
        </div>
      </div>
    </CollapsibleCard>
  );
}
