import { Wifi, WifiOff } from 'lucide-react';
import { useStore } from '../store';

export default function ConnectionIndicator() {
  const { isConnected, isWSConnected } = useStore();

  if (isConnected && isWSConnected) {
    return null; // Don't show if everything is connected
  }

  return (
    <div className="fixed top-20 right-4 z-50">
      <div className="bg-dark-800 border border-dark-700 rounded-lg shadow-lg p-3 flex items-center space-x-2">
        {!isWSConnected ? (
          <>
            <WifiOff className="w-4 h-4 text-danger" />
            <span className="text-sm text-gray-300">WebSocket disconnected</span>
          </>
        ) : (
          <>
            <Wifi className="w-4 h-4 text-warning" />
            <span className="text-sm text-gray-300">Reconnecting...</span>
          </>
        )}
      </div>
    </div>
  );
}
