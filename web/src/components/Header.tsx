import { Activity } from 'lucide-react';
import { useStore } from '../store';

export default function Header() {
  const { botStatus } = useStore();

  return (
    <header className="bg-dark-800 border-b border-dark-700 shadow-lg">
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <Activity className="w-8 h-8 text-primary-500" />
            <div>
              <h1 className="text-2xl font-bold text-white">Trading Bot Dashboard</h1>
              <p className="text-sm text-gray-400">
                {botStatus?.testnet ? 'Testnet' : 'Live Trading'} •{' '}
                {botStatus?.dry_run ? 'Dry Run Mode' : 'Live Mode'}
              </p>
            </div>
          </div>

          <div className="flex items-center space-x-4">
            {botStatus && (
              <div className="flex items-center space-x-2">
                <div
                  className={`w-3 h-3 rounded-full ${
                    botStatus.running ? 'bg-success animate-pulse' : 'bg-gray-500'
                  }`}
                />
                <span className="text-sm text-gray-300">
                  {botStatus.running ? 'Running' : 'Stopped'}
                </span>
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
