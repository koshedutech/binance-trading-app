import { useEffect, useRef } from 'react';
import { WifiOff } from 'lucide-react';
import { useStore } from '../store';

export default function ConnectionIndicator() {
  const { isConnected, isWSConnected } = useStore();
  const wasConnected = useRef(false);

  // Track if we've ever been connected
  useEffect(() => {
    if (isWSConnected) {
      wasConnected.current = true;
    }
  }, [isWSConnected]);

  // Don't show anything if connected
  if (isConnected && isWSConnected) {
    return null;
  }

  // Only show disconnected message if we were previously connected
  if (!wasConnected.current) {
    return null; // Don't show on initial load
  }

  return (
    <div className="fixed top-20 right-4 z-50">
      <div className="bg-dark-800 border border-dark-700 rounded-lg shadow-lg p-3 flex items-center space-x-2">
        <WifiOff className="w-4 h-4 text-danger" />
        <span className="text-sm text-gray-300">WebSocket disconnected</span>
      </div>
    </div>
  );
}
