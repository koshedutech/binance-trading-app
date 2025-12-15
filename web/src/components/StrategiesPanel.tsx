import { useEffect } from 'react';
import { useStore } from '../store';
import { apiService } from '../services/api';
import { Power, PowerOff } from 'lucide-react';

export default function StrategiesPanel() {
  const { strategies, setStrategies } = useStore();

  useEffect(() => {
    const fetchStrategies = async () => {
      try {
        const data = await apiService.getStrategies();
        setStrategies(data);
      } catch (error) {
        console.error('Failed to fetch strategies:', error);
      }
    };

    fetchStrategies();
    const interval = setInterval(fetchStrategies, 60000);
    return () => clearInterval(interval);
  }, [setStrategies]);

  const handleToggle = async (name: string, enabled: boolean) => {
    try {
      await apiService.toggleStrategy(name, !enabled);
      const updated = await apiService.getStrategies();
      setStrategies(updated);
    } catch (error) {
      alert('Failed to toggle strategy');
      console.error(error);
    }
  };

  if (strategies.length === 0) {
    return (
      <div className="p-8 text-center text-gray-400">
        No strategies registered
      </div>
    );
  }

  return (
    <div className="divide-y divide-dark-700">
      {strategies.map((strategy) => (
        <div key={strategy.name} className="p-4 hover:bg-dark-750 transition-colors">
          <div className="flex items-center justify-between">
            <div className="flex-1">
              <div className="flex items-center space-x-2">
                <h3 className="font-semibold">{strategy.name}</h3>
                <span
                  className={`badge ${
                    strategy.enabled ? 'badge-success' : 'badge-danger'
                  }`}
                >
                  {strategy.enabled ? 'Active' : 'Disabled'}
                </span>
              </div>
              <div className="mt-1 text-sm text-gray-400">
                {strategy.symbol} • {strategy.interval}
              </div>
              {strategy.last_signal && (
                <div className="mt-1 text-xs text-gray-500">
                  Last signal: {strategy.last_signal}
                </div>
              )}
            </div>
            <button
              onClick={() => handleToggle(strategy.name, strategy.enabled)}
              className={`btn text-xs py-1 px-3 flex items-center space-x-1 ${
                strategy.enabled ? 'btn-danger' : 'btn-success'
              }`}
            >
              {strategy.enabled ? (
                <>
                  <PowerOff className="w-3 h-3" />
                  <span>Disable</span>
                </>
              ) : (
                <>
                  <Power className="w-3 h-3" />
                  <span>Enable</span>
                </>
              )}
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
