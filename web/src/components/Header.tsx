import { Activity, LayoutDashboard, TrendingUp, Sparkles } from 'lucide-react';
import { Link, useLocation } from 'react-router-dom';
import { useStore } from '../store';
import { PositionsHeader } from './PositionsHeader';

export default function Header() {
  const { botStatus } = useStore();
  const location = useLocation();

  const isActive = (path: string) => location.pathname === path;

  return (
    <header className="bg-dark-800 border-b border-dark-700 shadow-lg">
      <PositionsHeader />
      <div className="container mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-8">
            <Link to="/" className="flex items-center space-x-3">
              <Activity className="w-8 h-8 text-primary-500" />
              <div>
                <h1 className="text-2xl font-bold text-white">Trading Bot Dashboard</h1>
                <p className="text-sm text-gray-400">
                  {botStatus?.testnet ? 'Testnet' : 'Live Trading'} •{' '}
                  {botStatus?.dry_run ? 'Dry Run Mode' : 'Live Mode'}
                </p>
              </div>
            </Link>

            {/* Navigation Links */}
            <nav className="flex items-center space-x-4">
              <Link
                to="/"
                className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                  isActive('/')
                    ? 'bg-primary-600 text-white'
                    : 'text-gray-300 hover:bg-dark-700 hover:text-white'
                }`}
              >
                <LayoutDashboard className="w-4 h-4" />
                <span className="font-medium">Dashboard</span>
              </Link>
              <Link
                to="/visual-strategy-advanced"
                className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                  isActive('/visual-strategy-advanced')
                    ? 'bg-primary-600 text-white'
                    : 'text-gray-300 hover:bg-dark-700 hover:text-white'
                }`}
              >
                <TrendingUp className="w-4 h-4" />
                <span className="font-medium">Strategy Builder</span>
              </Link>
              <Link
                to="/pattern-scanner"
                className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                  isActive('/pattern-scanner')
                    ? 'bg-primary-600 text-white'
                    : 'text-gray-300 hover:bg-dark-700 hover:text-white'
                }`}
              >
                <Sparkles className="w-4 h-4" />
                <span className="font-medium">Pattern Scanner</span>
              </Link>
            </nav>
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
