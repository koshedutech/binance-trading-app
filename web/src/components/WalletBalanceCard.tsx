import { useState, useEffect } from 'react';
import { apiService } from '../services/api';
import { Wallet, RefreshCw, AlertCircle, TrendingUp, Lock } from 'lucide-react';
import { useFuturesStore, selectTotalUnrealizedPnl } from '../store/futuresStore';

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
  const [balance, setBalance] = useState<WalletBalance | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Get trading mode from futures store to trigger refresh when it changes
  const tradingMode = useFuturesStore((state) => state.tradingMode);

  // CRITICAL: Get unrealized PnL from positions (source of truth)
  const totalUnrealizedPnl = useFuturesStore(selectTotalUnrealizedPnl);

  useEffect(() => {
    fetchBalance();
    const interval = setInterval(fetchBalance, 10000); // Refresh every 10 seconds for near real-time updates
    return () => clearInterval(interval);
  }, [tradingMode]); // Re-fetch when trading mode changes

  const fetchBalance = async () => {
    try {
      const data = await apiService.getWalletBalance();
      setBalance(data);
      setError(null);
    } catch (err) {
      setError('Failed to load balance');
      console.error(err);
    } finally {
      setIsLoading(false);
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

  if (isLoading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 border border-gray-700 animate-pulse">
        <div className="h-6 bg-gray-700 rounded w-24 mb-2"></div>
        <div className="h-8 bg-gray-700 rounded w-32"></div>
      </div>
    );
  }

  if (error) {
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
        <button
          onClick={fetchBalance}
          className="p-1.5 hover:bg-gray-700 rounded transition-colors"
          title="Refresh balance"
        >
          <RefreshCw className={`w-4 h-4 text-gray-400 ${isLoading ? 'animate-spin' : ''}`} />
        </button>
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
