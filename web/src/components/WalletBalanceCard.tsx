import { useCallback, useEffect, useState } from 'react';
import { apiService } from '../services/api';
import { Wallet, RefreshCw, AlertCircle, TrendingUp, Lock, Wifi, WifiOff } from 'lucide-react';
import { useFuturesStore, selectTotalUnrealizedPnl } from '../store/futuresStore';
import { useWebSocketData } from '../hooks/useWebSocketData';
import type { WSEvent } from '../types';

interface WalletBalance {
  total_balance: number;
  available_balance: number;
  locked_balance: number;
  total_margin_balance: number;
  total_unrealized_pnl: number;
  currency: string;
  is_simulated: boolean;
  assets: Array<{ asset: string; free: number; locked: number }>;
}

export default function WalletBalanceCard() {
  // Get trading mode from futures store to trigger refresh when it changes
  const tradingMode = useFuturesStore((state) => state.tradingMode);

  // CRITICAL: Get unrealized PnL from positions (source of truth)
  const totalUnrealizedPnl = useFuturesStore(selectTotalUnrealizedPnl);

  // Track if refresh button was clicked
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Fetch function for REST API fallback
  const fetchBalance = useCallback(async (): Promise<WalletBalance> => {
    return apiService.getWalletBalance();
  }, []);

  // Transform WebSocket event data to WalletBalance format
  // The backend sends POSITION_UPDATE events which may contain balance updates
  const transformEvent = useCallback((event: WSEvent): WalletBalance | null => {
    // Check if the event has balance data
    // The backend sends account info in POSITION_UPDATE or a dedicated balance event
    if (event.data?.balances || event.data?.total_balance !== undefined) {
      return {
        total_balance: event.data.total_balance ?? 0,
        available_balance: event.data.available_balance ?? 0,
        locked_balance: event.data.locked_balance ?? 0,
        total_margin_balance: event.data.total_margin_balance ?? 0,
        total_unrealized_pnl: event.data.total_unrealized_pnl ?? 0,
        currency: event.data.currency ?? 'USD',
        is_simulated: event.data.is_simulated ?? false,
        assets: event.data.assets ?? [],
      };
    }
    return null;
  }, []);

  // Use WebSocket with REST fallback
  // Subscribe to POSITION_UPDATE as it may contain balance updates
  // Also subscribe to TRADE_UPDATE and ORDER_UPDATE which can affect balances
  const {
    data: balance,
    isConnected,
    isRealTime,
    lastUpdate,
    refresh,
    error,
    isLoading,
  } = useWebSocketData<WalletBalance>({
    messageType: 'POSITION_UPDATE',
    fallbackFetch: fetchBalance,
    fallbackInterval: 30000, // Poll every 30 seconds as fallback (reduced from 10s)
    transform: transformEvent as (event: WSEvent) => WalletBalance,
  });

  // Refetch when trading mode changes
  useEffect(() => {
    refresh();
  }, [tradingMode, refresh]);

  // Handle manual refresh
  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await refresh();
    } finally {
      setIsRefreshing(false);
    }
  };

  const formatUSD = (value: number) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: 'USD',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value);
  };

  // Format last update time
  const formatLastUpdate = (date: Date | null) => {
    if (!date) return '';
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffSec = Math.floor(diffMs / 1000);

    if (diffSec < 5) return 'just now';
    if (diffSec < 60) return `${diffSec}s ago`;
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return `${diffMin}m ago`;
    return date.toLocaleTimeString();
  };

  if (isLoading && !balance) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 border border-gray-700 animate-pulse">
        <div className="h-6 bg-gray-700 rounded w-24 mb-2"></div>
        <div className="h-8 bg-gray-700 rounded w-32"></div>
      </div>
    );
  }

  if (error && !balance) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 border border-red-500/30">
        <div className="flex items-center gap-2 text-red-500">
          <AlertCircle className="w-5 h-5" />
          <span className="text-sm">{error}</span>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <Wallet className="w-5 h-5 text-primary-500" />
          <span className="font-semibold text-white">Wallet Balance</span>
          {balance?.is_simulated && (
            <span className="px-2 py-0.5 bg-yellow-500/20 text-yellow-500 text-xs rounded">
              Simulated
            </span>
          )}
        </div>
        <div className="flex items-center gap-2">
          {/* Real-time indicator */}
          <div
            className="flex items-center gap-1"
            title={isRealTime ? 'Real-time via WebSocket' : 'Polling via REST API'}
          >
            {isConnected && isRealTime ? (
              <Wifi className="w-3.5 h-3.5 text-green-500" />
            ) : (
              <WifiOff className="w-3.5 h-3.5 text-gray-500" />
            )}
            {lastUpdate && (
              <span className="text-xs text-gray-500">
                {formatLastUpdate(lastUpdate)}
              </span>
            )}
          </div>
          <button
            onClick={handleRefresh}
            className="p-1.5 hover:bg-gray-700 rounded transition-colors"
            title="Refresh balance"
            disabled={isRefreshing}
          >
            <RefreshCw className={`w-4 h-4 text-gray-400 ${isRefreshing ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      {/* Balance Display */}
      <div className="p-4">
        {/* Wallet Balance & Margin Balance */}
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div>
            <span className="text-xs text-gray-500 uppercase tracking-wide">Wallet Balance</span>
            <div className="text-2xl font-bold text-white">
              {formatUSD(balance?.total_balance || 0)}
            </div>
          </div>
          <div>
            <span className="text-xs text-gray-500 uppercase tracking-wide">Margin Balance</span>
            <div className={`text-2xl font-bold ${
              totalUnrealizedPnl > 0
                ? 'text-green-500'
                : totalUnrealizedPnl < 0
                ? 'text-red-500'
                : 'text-white'
            }`}>
              {/* CRITICAL: Calculate from wallet + positions unrealized PnL (source of truth) */}
              {formatUSD((balance?.total_balance || 0) + totalUnrealizedPnl)}
            </div>
            {totalUnrealizedPnl !== 0 && (
              <div className={`text-xs ${totalUnrealizedPnl > 0 ? 'text-green-500' : 'text-red-500'}`}>
                {totalUnrealizedPnl > 0 ? '+' : ''}{formatUSD(totalUnrealizedPnl)}
              </div>
            )}
          </div>
        </div>

        {/* Available & Locked */}
        <div className="grid grid-cols-2 gap-4 mb-4">
          <div className="bg-gray-900 rounded-lg p-3">
            <div className="flex items-center gap-2 mb-1">
              <TrendingUp className="w-4 h-4 text-green-500" />
              <span className="text-xs text-gray-400">Available</span>
            </div>
            <div className="text-lg font-semibold text-green-500">
              {formatUSD(balance?.available_balance || 0)}
            </div>
          </div>
          <div className="bg-gray-900 rounded-lg p-3">
            <div className="flex items-center gap-2 mb-1">
              <Lock className="w-4 h-4 text-yellow-500" />
              <span className="text-xs text-gray-400">In Orders</span>
            </div>
            <div className="text-lg font-semibold text-yellow-500">
              {formatUSD(balance?.locked_balance || 0)}
            </div>
          </div>
        </div>

        {/* Asset List */}
        {balance?.assets && balance.assets.length > 0 && (
          <div className="border-t border-gray-700 pt-3">
            <span className="text-xs text-gray-500 uppercase tracking-wide mb-2 block">Assets</span>
            <div className="space-y-2 max-h-40 overflow-y-auto">
              {balance.assets.slice(0, 10).map((asset) => (
                <div
                  key={asset.asset}
                  className="flex items-center justify-between text-sm"
                >
                  <span className="text-gray-300 font-medium">{asset.asset}</span>
                  <div className="text-right">
                    <span className="text-white">{asset.free.toFixed(6)}</span>
                    {asset.locked > 0 && (
                      <span className="text-gray-500 text-xs ml-2">
                        (+{asset.locked.toFixed(6)} locked)
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
