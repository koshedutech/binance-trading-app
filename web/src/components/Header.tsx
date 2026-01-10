import { useState, useRef, useEffect } from 'react';
import { Activity, LayoutDashboard, TrendingUp, Sparkles, Zap, Layers, User, CreditCard, Key, LogOut, ChevronDown, LogIn, UserPlus, Search, Shield, Brain, RefreshCw } from 'lucide-react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useStore } from '../store';
import { useAuth, TIER_INFO } from '../contexts/AuthContext';
import { PositionsHeader } from './PositionsHeader';
import APIHealthIndicator from './APIHealthIndicator';

// Tier badge colors
const tierColors: Record<string, { bg: string; text: string }> = {
  free: { bg: 'bg-gray-600', text: 'text-gray-100' },
  trader: { bg: 'bg-blue-600', text: 'text-blue-100' },
  pro: { bg: 'bg-purple-600', text: 'text-purple-100' },
  whale: { bg: 'bg-yellow-600', text: 'text-yellow-100' },
};

export default function Header() {
  const { botStatus } = useStore();
  const { user, logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [subscriptionEnabled, setSubscriptionEnabled] = useState(true);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const isActive = (path: string) => location.pathname === path;

  // Check if subscription is enabled on mount
  useEffect(() => {
    const checkSubscriptionStatus = async () => {
      try {
        const response = await fetch('/api/auth/status');
        const data = await response.json();
        setSubscriptionEnabled(data.subscription_enabled ?? true);
      } catch (error) {
        // Default to enabled if check fails
        setSubscriptionEnabled(true);
      }
    };
    checkSubscriptionStatus();
  }, []);

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setDropdownOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleLogout = async () => {
    setDropdownOpen(false);
    await logout();
    // Navigate to login page after logout
    navigate('/login', { replace: true });
  };

  const tierColor = tierColors[user?.subscription_tier || 'free'] || tierColors.free;
  const tierInfo = TIER_INFO[user?.subscription_tier as keyof typeof TIER_INFO] || TIER_INFO.free;

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
                to="/strategy-builder"
                className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                  isActive('/strategy-builder')
                    ? 'bg-blue-600 text-white'
                    : 'text-gray-300 hover:bg-dark-700 hover:text-blue-400'
                }`}
              >
                <Layers className="w-4 h-4" />
                <span className="font-medium">Strategy Builder</span>
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
                <span className="font-medium">Visual Builder</span>
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
              <Link
                to="/futures"
                className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                  isActive('/futures')
                    ? 'bg-yellow-600 text-white'
                    : 'text-gray-300 hover:bg-dark-700 hover:text-yellow-400'
                }`}
              >
                <Zap className="w-4 h-4" />
                <span className="font-medium">Futures</span>
              </Link>
              <Link
                to="/investigate"
                className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                  isActive('/investigate')
                    ? 'bg-purple-600 text-white'
                    : 'text-gray-300 hover:bg-dark-700 hover:text-purple-400'
                }`}
              >
                <Search className="w-4 h-4" />
                <span className="font-medium">Investigate</span>
              </Link>
            </nav>
          </div>

          <div className="flex items-center space-x-4">
            {/* API Health Status - only show when logged in */}
            {user && <APIHealthIndicator />}

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

            {/* User Menu - when logged in */}
            {user ? (
              <div className="relative" ref={dropdownRef}>
                <button
                  onClick={() => setDropdownOpen(!dropdownOpen)}
                  className="flex items-center space-x-2 px-3 py-2 rounded-lg hover:bg-dark-700 transition-colors"
                >
                  <div className="w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center">
                    <span className="text-white font-medium text-sm">
                      {user.name?.charAt(0).toUpperCase() || user.email?.charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <div className="text-left hidden md:block">
                    <p className="text-sm font-medium text-white">{user.name || user.email}</p>
                    <span className={`text-xs px-2 py-0.5 rounded ${tierColor.bg} ${tierColor.text}`}>
                      {user.subscription_tier?.toUpperCase()} • {tierInfo.profitShare}% share
                    </span>
                  </div>
                  <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${dropdownOpen ? 'rotate-180' : ''}`} />
                </button>

                {/* Dropdown Menu */}
                {dropdownOpen && (
                  <div className="absolute right-0 mt-2 w-56 bg-dark-700 rounded-lg shadow-lg border border-dark-600 py-1 z-50">
                    <div className="px-4 py-3 border-b border-dark-600">
                      <p className="text-sm font-medium text-white">{user.name}</p>
                      <p className="text-xs text-gray-400">{user.email}</p>
                    </div>

                    <Link
                      to="/settings"
                      onClick={() => setDropdownOpen(false)}
                      className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-dark-600 hover:text-white transition-colors"
                    >
                      <User className="w-4 h-4" />
                      Settings
                    </Link>

                    <Link
                      to="/settings?tab=binance"
                      onClick={() => setDropdownOpen(false)}
                      className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-dark-600 hover:text-white transition-colors"
                    >
                      <Key className="w-4 h-4" />
                      API Keys
                    </Link>

                    <Link
                      to="/settings?tab=ai"
                      onClick={() => setDropdownOpen(false)}
                      className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-dark-600 hover:text-white transition-colors"
                    >
                      <Brain className="w-4 h-4" />
                      AI Keys
                    </Link>

                    <Link
                      to="/reset-settings"
                      onClick={() => setDropdownOpen(false)}
                      className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-dark-600 hover:text-white transition-colors"
                    >
                      <RefreshCw className="w-4 h-4" />
                      Reset Defaults
                    </Link>

                    {/* Only show Billing menu when subscription is enabled */}
                    {subscriptionEnabled && (
                      <Link
                        to="/billing"
                        onClick={() => setDropdownOpen(false)}
                        className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-dark-600 hover:text-white transition-colors"
                      >
                        <CreditCard className="w-4 h-4" />
                        Billing & Subscription
                      </Link>
                    )}

                    {user.is_admin && (
                      <Link
                        to="/admin"
                        onClick={() => setDropdownOpen(false)}
                        className="flex items-center gap-2 px-4 py-2 text-sm text-gray-300 hover:bg-dark-600 hover:text-white transition-colors"
                      >
                        <Shield className="w-4 h-4" />
                        Admin Panel
                      </Link>
                    )}

                    <div className="border-t border-dark-600 mt-1 pt-1">
                      <button
                        onClick={handleLogout}
                        className="flex items-center gap-2 px-4 py-2 text-sm text-red-400 hover:bg-dark-600 hover:text-red-300 transition-colors w-full text-left"
                      >
                        <LogOut className="w-4 h-4" />
                        Sign Out
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              /* Login/Register buttons - when not logged in */
              <div className="flex items-center space-x-3">
                <Link
                  to="/login"
                  className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-300 hover:text-white transition-colors"
                >
                  <LogIn className="w-4 h-4" />
                  Sign In
                </Link>
                <Link
                  to="/register"
                  className="flex items-center gap-2 px-4 py-2 text-sm font-medium bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                >
                  <UserPlus className="w-4 h-4" />
                  Register
                </Link>
              </div>
            )}
          </div>
        </div>
      </div>
    </header>
  );
}
