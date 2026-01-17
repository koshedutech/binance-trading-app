import { useState, useEffect } from 'react';
import { wsService } from '../services/websocket';
import { fallbackManager } from '../services/fallbackPollingManager';

interface WebSocketStatus {
  isConnected: boolean;
  lastConnected: Date | null;
  reconnectAttempts: number;
  isSyncing: boolean;
  isUsingFallback: boolean;
}

// Global syncing state that can be set from App.tsx
let globalSyncingState = false;
const syncingListeners: Set<(syncing: boolean) => void> = new Set();

export function setSyncingState(syncing: boolean): void {
  globalSyncingState = syncing;
  syncingListeners.forEach(listener => listener(syncing));
}

export function useWebSocketStatus(): WebSocketStatus {
  const [status, setStatus] = useState<WebSocketStatus>({
    isConnected: wsService.isConnected(),
    lastConnected: wsService.isConnected() ? new Date() : null,
    reconnectAttempts: wsService.getReconnectAttempts(),
    isSyncing: globalSyncingState,
    isUsingFallback: fallbackManager.getIsActive(),
  });

  useEffect(() => {
    const handleConnect = () => {
      setStatus(prev => ({
        ...prev,
        isConnected: true,
        lastConnected: new Date(),
        reconnectAttempts: 0, // wsService resets on connect
        isUsingFallback: fallbackManager.getIsActive(),
      }));
    };

    const handleDisconnect = () => {
      setStatus(prev => ({
        ...prev,
        isConnected: false,
        // Use wsService's counter as source of truth
        reconnectAttempts: wsService.getReconnectAttempts(),
        isUsingFallback: fallbackManager.getIsActive(),
      }));
    };

    const handleSyncingChange = (syncing: boolean) => {
      setStatus(prev => ({
        ...prev,
        isSyncing: syncing,
      }));
    };

    const handleFallbackChange = (isActive: boolean) => {
      setStatus(prev => ({
        ...prev,
        isUsingFallback: isActive,
      }));
    };

    wsService.onConnect(handleConnect);
    wsService.onDisconnect(handleDisconnect);
    syncingListeners.add(handleSyncingChange);
    fallbackManager.onChange(handleFallbackChange);

    return () => {
      wsService.offConnect(handleConnect);
      wsService.offDisconnect(handleDisconnect);
      syncingListeners.delete(handleSyncingChange);
      fallbackManager.offChange(handleFallbackChange);
    };
  }, []);

  return status;
}
