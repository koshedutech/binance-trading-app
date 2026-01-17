import { useWebSocketStatus } from '../hooks/useWebSocketStatus';

export function ConnectionStatus() {
  const { isConnected, reconnectAttempts, isSyncing, isUsingFallback } = useWebSocketStatus();

  // Priority: Syncing > Reconnecting > Fallback Polling > Live
  if (isSyncing) {
    return (
      <div className="flex items-center gap-1 text-blue-400">
        <div className="w-2 h-2 bg-blue-400 rounded-full animate-pulse" />
        <span className="text-xs">Syncing...</span>
      </div>
    );
  }

  if (!isConnected) {
    return (
      <div className="flex items-center gap-1 text-yellow-500">
        <div className="w-2 h-2 bg-yellow-500 rounded-full animate-pulse" />
        <span className="text-xs">
          Reconnecting{reconnectAttempts > 1 ? ` (${reconnectAttempts})` : '...'}
        </span>
      </div>
    );
  }

  if (isUsingFallback) {
    return (
      <div className="flex items-center gap-1 text-orange-400">
        <div className="w-2 h-2 bg-orange-400 rounded-full" />
        <span className="text-xs">Updating...</span>
      </div>
    );
  }

  // Connected and not using fallback = Live
  return (
    <div className="flex items-center gap-1 text-green-500">
      <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
      <span className="text-xs">Live</span>
    </div>
  );
}
