import { useState, useEffect, useRef } from 'react';
import { Wifi, WifiOff, AlertTriangle, CheckCircle, Database, Bot, Activity } from 'lucide-react';
import { apiService } from '../services/api';
import { useAuth } from '../contexts/AuthContext';

interface ServiceStatus {
  status: string;
  message: string;
}

interface HealthStatus {
  healthy: boolean;
  services: {
    binance_spot: ServiceStatus;
    binance_futures: ServiceStatus;
    ai_service: ServiceStatus;
    database: ServiceStatus;
  };
}

export default function APIHealthIndicator() {
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [showDetails, setShowDetails] = useState(false);
  const { isAuthenticated } = useAuth();
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Reset state immediately when auth changes to prevent showing stale status
    setHealth(null);
    setLoading(true);
    setShowDetails(false);

    const fetchHealth = async () => {
      try {
        // Use user-specific API status when authenticated to show correct status
        // based on user's configured API keys, not global system status
        const data = isAuthenticated
          ? await apiService.getUserAPIStatus()
          : await apiService.getAPIHealthStatus();
        setHealth(data);
      } catch (error) {
        console.error('Failed to fetch API health:', error);
        setHealth(null);
      } finally {
        setLoading(false);
      }
    };

    fetchHealth();
    const interval = setInterval(fetchHealth, 30000); // Check every 30 seconds

    // Listen for API key changes to refresh immediately
    const handleAPIKeyChange = () => {
      fetchHealth();
    };
    window.addEventListener('api-key-changed', handleAPIKeyChange);

    return () => {
      clearInterval(interval);
      window.removeEventListener('api-key-changed', handleAPIKeyChange);
    };
  }, [isAuthenticated]);

  // Close popover on click outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowDetails(false);
      }
    };

    if (showDetails) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [showDetails]);

  // Close popover on Escape key
  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setShowDetails(false);
      }
    };

    if (showDetails) {
      document.addEventListener('keydown', handleEscape);
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
    };
  }, [showDetails]);

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'ok':
        return <CheckCircle className="w-3 h-3 text-green-500" />;
      case 'error':
        return <AlertTriangle className="w-3 h-3 text-red-500" />;
      case 'stopped':
        return <AlertTriangle className="w-3 h-3 text-yellow-500" />;
      case 'not_configured':
        return <AlertTriangle className="w-3 h-3 text-orange-500" />;
      default:
        return <AlertTriangle className="w-3 h-3 text-gray-500" />;
    }
  };

  const getServiceIcon = (service: string) => {
    switch (service) {
      case 'binance_spot':
      case 'binance_futures':
        return <Activity className="w-3 h-3" />;
      case 'ai_service':
        return <Bot className="w-3 h-3" />;
      case 'database':
        return <Database className="w-3 h-3" />;
      default:
        return <Wifi className="w-3 h-3" />;
    }
  };

  const getServiceLabel = (service: string) => {
    switch (service) {
      case 'binance_spot':
        return 'Spot API';
      case 'binance_futures':
        return 'Futures API';
      case 'ai_service':
        return 'AI Service';
      case 'database':
        return 'Database';
      default:
        return service;
    }
  };

  if (loading) {
    return (
      <div className="flex items-center gap-1 text-gray-400">
        <Wifi className="w-4 h-4 animate-pulse" />
      </div>
    );
  }

  if (!health) {
    return (
      <div className="flex items-center gap-1 text-red-500" title="API Unreachable">
        <WifiOff className="w-4 h-4" />
      </div>
    );
  }

  const errorCount = Object.values(health.services).filter(s => s.status === 'error').length;
  const notConfiguredCount = Object.values(health.services).filter(s => s.status === 'not_configured').length;
  const issueCount = errorCount + notConfiguredCount;

  // Determine overall status color
  const getStatusStyle = () => {
    if (health.healthy && notConfiguredCount === 0) {
      return 'bg-green-500/20 text-green-400 hover:bg-green-500/30';
    }
    if (errorCount > 0) {
      return 'bg-red-500/20 text-red-400 hover:bg-red-500/30';
    }
    if (notConfiguredCount > 0) {
      return 'bg-orange-500/20 text-orange-400 hover:bg-orange-500/30';
    }
    return 'bg-yellow-500/20 text-yellow-400 hover:bg-yellow-500/30';
  };

  const getStatusText = () => {
    if (health.healthy && notConfiguredCount === 0) return 'APIs OK';
    if (errorCount > 0) return `${errorCount} Error${errorCount > 1 ? 's' : ''}`;
    if (notConfiguredCount > 0) return `${notConfiguredCount} Not Set`;
    return 'Issues';
  };

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setShowDetails(!showDetails)}
        className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium transition-colors ${getStatusStyle()}`}
        title={health.healthy && notConfiguredCount === 0 ? 'All APIs Connected' : `${issueCount} API Issue(s)`}
      >
        {health.healthy && notConfiguredCount === 0 ? (
          <Wifi className="w-3.5 h-3.5" />
        ) : (
          <WifiOff className="w-3.5 h-3.5" />
        )}
        <span className="hidden sm:inline">
          {getStatusText()}
        </span>
      </button>

      {showDetails && (
        <div className="absolute right-0 top-full mt-2 w-64 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-50">
          <div className="p-3 border-b border-gray-700">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium text-white">API Status</span>
              {health.healthy && notConfiguredCount === 0 ? (
                <span className="text-xs text-green-400">All Connected</span>
              ) : notConfiguredCount > 0 && errorCount === 0 ? (
                <span className="text-xs text-orange-400">Keys Not Set</span>
              ) : (
                <span className="text-xs text-red-400">Issues Detected</span>
              )}
            </div>
          </div>
          <div className="p-2 space-y-1">
            {Object.entries(health.services).map(([key, service]) => (
              <div
                key={key}
                className={`flex items-center justify-between p-2 rounded text-xs ${
                  service.status === 'ok'
                    ? 'bg-green-500/10'
                    : service.status === 'error'
                    ? 'bg-red-500/10'
                    : service.status === 'not_configured'
                    ? 'bg-orange-500/10'
                    : 'bg-yellow-500/10'
                }`}
              >
                <div className="flex items-center gap-2">
                  {getServiceIcon(key)}
                  <span className="text-gray-300">{getServiceLabel(key)}</span>
                </div>
                <div className="flex items-center gap-1.5">
                  {getStatusIcon(service.status)}
                  <span
                    className={
                      service.status === 'ok'
                        ? 'text-green-400'
                        : service.status === 'error'
                        ? 'text-red-400'
                        : service.status === 'not_configured'
                        ? 'text-orange-400'
                        : 'text-yellow-400'
                    }
                  >
                    {service.status === 'ok' ? 'OK' : service.status === 'not_configured' ? 'NOT SET' : service.status.toUpperCase()}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
