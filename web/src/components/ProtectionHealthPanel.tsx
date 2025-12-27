import React, { useEffect, useState } from 'react';
import { getProtectionStatus, ProtectionStatusResponse, ProtectionPositionStatus } from '../services/futuresApi';

// Protection state colors and labels
const PROTECTION_STATES: Record<string, { label: string; color: string; icon: string }> = {
  PROTECTED: { label: 'Protected', color: 'text-green-400', icon: '✅' },
  SL_VERIFIED: { label: 'SL Only', color: 'text-yellow-400', icon: '⚠️' },
  OPENING: { label: 'Opening', color: 'text-blue-400', icon: '🔄' },
  PLACING_SL: { label: 'Placing SL', color: 'text-blue-400', icon: '🔄' },
  PLACING_TP: { label: 'Placing TP', color: 'text-blue-400', icon: '🔄' },
  HEALING: { label: 'Healing', color: 'text-orange-400', icon: '🔧' },
  UNPROTECTED: { label: 'UNPROTECTED', color: 'text-red-500', icon: '🔴' },
  EMERGENCY: { label: 'EMERGENCY', color: 'text-red-600 animate-pulse', icon: '🚨' },
  UNKNOWN: { label: 'Unknown', color: 'text-gray-400', icon: '❓' },
};

interface ProtectionHealthPanelProps {
  refreshInterval?: number; // in milliseconds, default 5000
  compact?: boolean; // compact mode for sidebar
}

export const ProtectionHealthPanel: React.FC<ProtectionHealthPanelProps> = ({
  refreshInterval = 5000,
  compact = false,
}) => {
  const [status, setStatus] = useState<ProtectionStatusResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);

  const fetchStatus = async () => {
    try {
      const data = await getProtectionStatus();
      setStatus(data);
      setLastUpdate(new Date());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch protection status');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStatus();
    const interval = setInterval(fetchStatus, refreshInterval);
    return () => clearInterval(interval);
  }, [refreshInterval]);

  const getStateInfo = (state: string) => {
    return PROTECTION_STATES[state] || PROTECTION_STATES.UNKNOWN;
  };

  const getHealthColor = (healthPct: number) => {
    if (healthPct >= 100) return 'text-green-400';
    if (healthPct >= 80) return 'text-yellow-400';
    if (healthPct >= 50) return 'text-orange-400';
    return 'text-red-500';
  };

  const getHealthBgColor = (healthPct: number) => {
    if (healthPct >= 100) return 'bg-green-500';
    if (healthPct >= 80) return 'bg-yellow-500';
    if (healthPct >= 50) return 'bg-orange-500';
    return 'bg-red-500';
  };

  if (loading && !status) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 animate-pulse">
        <div className="h-6 bg-gray-700 rounded w-1/2 mb-4"></div>
        <div className="h-20 bg-gray-700 rounded"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-gray-800 rounded-lg p-4 border border-red-500">
        <div className="flex items-center gap-2 text-red-400">
          <span>⚠️</span>
          <span>Protection Status Error: {error}</span>
        </div>
      </div>
    );
  }

  if (!status) return null;

  const { summary, positions } = status;
  const hasUnprotected = summary.unprotected > 0 || summary.healing > 0 || summary.emergency > 0;

  // Compact mode for sidebar
  if (compact) {
    return (
      <div className={`rounded-lg p-3 ${hasUnprotected ? 'bg-red-900/30 border border-red-500' : 'bg-gray-800'}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="text-lg">{hasUnprotected ? '🛡️' : '🛡️'}</span>
            <span className={`font-medium ${hasUnprotected ? 'text-red-400' : 'text-green-400'}`}>
              Protection
            </span>
          </div>
          <div className={`text-xl font-bold ${getHealthColor(summary.health_pct)}`}>
            {summary.health_pct.toFixed(0)}%
          </div>
        </div>
        {hasUnprotected && (
          <div className="mt-2 text-xs text-red-400 animate-pulse">
            ⚠️ {summary.unprotected} unprotected, {summary.healing} healing
          </div>
        )}
      </div>
    );
  }

  // Full panel mode
  return (
    <div className={`bg-gray-800 rounded-lg p-4 ${hasUnprotected ? 'border-2 border-red-500' : 'border border-gray-700'}`}>
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className="text-2xl">🛡️</span>
          <h3 className="text-lg font-semibold text-white">Position Protection Status</h3>
        </div>
        <div className="flex items-center gap-3">
          {/* Health Percentage Badge */}
          <div className={`px-3 py-1 rounded-full font-bold ${getHealthColor(summary.health_pct)} ${hasUnprotected ? 'animate-pulse' : ''}`}
               style={{ backgroundColor: hasUnprotected ? 'rgba(239, 68, 68, 0.2)' : 'rgba(34, 197, 94, 0.2)' }}>
            {summary.health_pct.toFixed(0)}% Health
          </div>
          {/* Last update */}
          <span className="text-xs text-gray-500">
            Updated: {lastUpdate?.toLocaleTimeString()}
          </span>
        </div>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-5 gap-3 mb-4">
        <div className="bg-gray-700/50 rounded-lg p-3 text-center">
          <div className="text-2xl font-bold text-white">{summary.total}</div>
          <div className="text-xs text-gray-400">Total</div>
        </div>
        <div className="bg-green-900/30 rounded-lg p-3 text-center">
          <div className="text-2xl font-bold text-green-400">{summary.protected}</div>
          <div className="text-xs text-gray-400">Protected</div>
        </div>
        <div className={`rounded-lg p-3 text-center ${summary.unprotected > 0 ? 'bg-red-900/50 animate-pulse' : 'bg-gray-700/50'}`}>
          <div className={`text-2xl font-bold ${summary.unprotected > 0 ? 'text-red-500' : 'text-gray-400'}`}>
            {summary.unprotected}
          </div>
          <div className="text-xs text-gray-400">Unprotected</div>
        </div>
        <div className={`rounded-lg p-3 text-center ${summary.healing > 0 ? 'bg-orange-900/30' : 'bg-gray-700/50'}`}>
          <div className={`text-2xl font-bold ${summary.healing > 0 ? 'text-orange-400' : 'text-gray-400'}`}>
            {summary.healing}
          </div>
          <div className="text-xs text-gray-400">Healing</div>
        </div>
        <div className={`rounded-lg p-3 text-center ${summary.emergency > 0 ? 'bg-red-900/50 animate-pulse' : 'bg-gray-700/50'}`}>
          <div className={`text-2xl font-bold ${summary.emergency > 0 ? 'text-red-600' : 'text-gray-400'}`}>
            {summary.emergency}
          </div>
          <div className="text-xs text-gray-400">Emergency</div>
        </div>
      </div>

      {/* Health Bar */}
      <div className="mb-4">
        <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
          <div
            className={`h-full ${getHealthBgColor(summary.health_pct)} transition-all duration-500`}
            style={{ width: `${summary.health_pct}%` }}
          ></div>
        </div>
      </div>

      {/* Position List */}
      {positions.length > 0 ? (
        <div className="space-y-2 max-h-60 overflow-y-auto">
          <div className="text-sm text-gray-400 mb-2">Active Positions:</div>
          {positions.map((pos: ProtectionPositionStatus) => {
            const stateInfo = getStateInfo(pos.protection_state);
            return (
              <div
                key={pos.symbol}
                className={`flex items-center justify-between p-2 rounded-lg ${
                  pos.is_protected ? 'bg-gray-700/50' : 'bg-red-900/30 border border-red-500'
                }`}
              >
                <div className="flex items-center gap-3">
                  <span className="text-lg">{stateInfo.icon}</span>
                  <div>
                    <span className="font-medium text-white">{pos.symbol}</span>
                    <span className={`ml-2 text-sm ${pos.side === 'LONG' ? 'text-green-400' : 'text-red-400'}`}>
                      {pos.side}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-4">
                  {/* SL/TP indicators */}
                  <div className="flex items-center gap-2 text-sm">
                    <span className={pos.sl_verified ? 'text-green-400' : 'text-red-400'}>
                      SL: {pos.sl_verified ? '✓' : '✗'}
                    </span>
                    <span className={pos.tp_verified ? 'text-green-400' : 'text-red-400'}>
                      TP: {pos.tp_verified ? '✓' : '✗'}
                    </span>
                  </div>
                  {/* State */}
                  <span className={`text-sm font-medium ${stateInfo.color}`}>
                    {stateInfo.label}
                  </span>
                  {/* Heal attempts if any */}
                  {pos.heal_attempts > 0 && (
                    <span className="text-xs text-orange-400">
                      (heal: {pos.heal_attempts})
                    </span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="text-center py-4 text-gray-500">
          No active positions
        </div>
      )}

      {/* Alert for unprotected positions */}
      {hasUnprotected && (
        <div className="mt-4 p-3 bg-red-900/30 border border-red-500 rounded-lg animate-pulse">
          <div className="flex items-center gap-2 text-red-400">
            <span className="text-xl">⚠️</span>
            <div>
              <div className="font-semibold">Protection Alert!</div>
              <div className="text-sm">
                {summary.unprotected > 0 && `${summary.unprotected} position(s) are UNPROTECTED. `}
                {summary.healing > 0 && `${summary.healing} position(s) are being healed. `}
                {summary.emergency > 0 && `${summary.emergency} position(s) in EMERGENCY mode!`}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default ProtectionHealthPanel;
