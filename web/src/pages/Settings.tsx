import React, { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { User, Key, Brain, Shield, CheckCircle, AlertTriangle, Eye, EyeOff, Plus, Trash2, RefreshCw, Copy, ExternalLink, Globe, Activity, Database, Bot, Wifi, WifiOff } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { apiService } from '../services/api';

// API Key interface
interface APIKey {
  id: string;
  exchange: string;
  api_key_last_four: string;
  is_testnet: boolean;
  is_active: boolean;
  created_at: string;
}

// AI Key interface
interface AIKey {
  id: string;
  provider: string;
  key_last_four: string;
  is_active: boolean;
  created_at: string;
}

type TabType = 'profile' | 'binance' | 'ai';

const Settings: React.FC = () => {
  const { user, refreshUser } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeTab, setActiveTab] = useState<TabType>((searchParams.get('tab') as TabType) || 'profile');
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Profile state
  const [profileForm, setProfileForm] = useState({ name: user?.name || '' });
  const [passwordForm, setPasswordForm] = useState({ currentPassword: '', newPassword: '', confirmPassword: '' });
  const [isSubmittingProfile, setIsSubmittingProfile] = useState(false);
  const [isSubmittingPassword, setIsSubmittingPassword] = useState(false);

  // API Keys state
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [isLoadingAPIKeys, setIsLoadingAPIKeys] = useState(false);
  const [showAddAPIKeyModal, setShowAddAPIKeyModal] = useState(false);
  const [newAPIKey, setNewAPIKey] = useState({ apiKey: '', secretKey: '', isTestnet: true });
  const [showSecretKey, setShowSecretKey] = useState(false);
  const [isSubmittingAPIKey, setIsSubmittingAPIKey] = useState(false);

  // AI Keys state
  const [aiKeys, setAIKeys] = useState<AIKey[]>([]);
  const [isLoadingAIKeys, setIsLoadingAIKeys] = useState(false);
  const [showAddAIKeyModal, setShowAddAIKeyModal] = useState(false);
  const [newAIKey, setNewAIKey] = useState({ provider: 'claude', apiKey: '' });
  const [showAIAPIKey, setShowAIAPIKey] = useState(false);
  const [isSubmittingAIKey, setIsSubmittingAIKey] = useState(false);

  // User IP and API Status state
  const [userIPAddress, setUserIPAddress] = useState<string>('');
  const [isLoadingIP, setIsLoadingIP] = useState(false);
  const [userAPIStatus, setUserAPIStatus] = useState<{
    healthy: boolean;
    services: {
      binance_spot: { status: string; message: string };
      binance_futures: { status: string; message: string };
      ai_service: { status: string; message: string };
      database: { status: string; message: string };
    };
  } | null>(null);
  const [copiedIP, setCopiedIP] = useState(false);

  // Update URL when tab changes
  const changeTab = (tab: TabType) => {
    setActiveTab(tab);
    setSearchParams({ tab });
    setMessage(null);
  };

  // Load API keys, IP address, and status when Binance tab is active
  useEffect(() => {
    if (activeTab === 'binance') {
      fetchAPIKeys();
      fetchUserIPAddress();
      fetchUserAPIStatus();
    }
  }, [activeTab]);

  const fetchUserIPAddress = async () => {
    try {
      setIsLoadingIP(true);
      const response = await apiService.getUserIPAddress();
      setUserIPAddress(response.ip_address);
    } catch (error) {
      console.error('Failed to fetch IP address:', error);
    } finally {
      setIsLoadingIP(false);
    }
  };

  const fetchUserAPIStatus = async () => {
    try {
      const response = await apiService.getUserAPIStatus();
      setUserAPIStatus({
        healthy: response.healthy,
        services: response.services,
      });
    } catch (error) {
      console.error('Failed to fetch API status:', error);
    }
  };

  const copyIPToClipboard = async () => {
    try {
      await navigator.clipboard.writeText(userIPAddress);
      setCopiedIP(true);
      setTimeout(() => setCopiedIP(false), 2000);
    } catch (error) {
      console.error('Failed to copy IP:', error);
    }
  };

  // Load AI keys when AI tab is active
  useEffect(() => {
    if (activeTab === 'ai') {
      fetchAIKeys();
    }
  }, [activeTab]);

  // Update profile form when user changes
  useEffect(() => {
    if (user) {
      setProfileForm({ name: user.name });
    }
  }, [user]);

  const fetchAPIKeys = async () => {
    try {
      setIsLoadingAPIKeys(true);
      const keys = await apiService.getAPIKeys();
      setApiKeys(keys);
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to load API keys' });
    } finally {
      setIsLoadingAPIKeys(false);
    }
  };

  const fetchAIKeys = async () => {
    try {
      setIsLoadingAIKeys(true);
      const keys = await apiService.getAIKeys();
      setAIKeys(keys);
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to load AI keys' });
    } finally {
      setIsLoadingAIKeys(false);
    }
  };

  const handleUpdateProfile = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmittingProfile(true);
    setMessage(null);

    try {
      await apiService.updateProfile({ name: profileForm.name });
      await refreshUser();
      setMessage({ type: 'success', text: 'Profile updated successfully!' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to update profile' });
    } finally {
      setIsSubmittingProfile(false);
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmittingPassword(true);
    setMessage(null);

    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      setMessage({ type: 'error', text: 'New passwords do not match' });
      setIsSubmittingPassword(false);
      return;
    }

    if (passwordForm.newPassword.length < 6) {
      setMessage({ type: 'error', text: 'New password must be at least 6 characters long' });
      setIsSubmittingPassword(false);
      return;
    }

    try {
      await apiService.changePassword({
        current_password: passwordForm.currentPassword,
        new_password: passwordForm.newPassword,
      });
      setMessage({ type: 'success', text: 'Password changed successfully!' });
      setPasswordForm({ currentPassword: '', newPassword: '', confirmPassword: '' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to change password' });
    } finally {
      setIsSubmittingPassword(false);
    }
  };

  const handleAddAPIKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmittingAPIKey(true);
    setMessage(null);

    try {
      await apiService.addAPIKey({
        api_key: newAPIKey.apiKey,
        secret_key: newAPIKey.secretKey,
        is_testnet: newAPIKey.isTestnet,
      });
      setMessage({ type: 'success', text: 'API key added successfully!' });
      setShowAddAPIKeyModal(false);
      setNewAPIKey({ apiKey: '', secretKey: '', isTestnet: true });
      fetchAPIKeys();
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to add API key' });
    } finally {
      setIsSubmittingAPIKey(false);
    }
  };

  const handleDeleteAPIKey = async (keyId: string) => {
    if (!confirm('Are you sure you want to delete this API key? This action cannot be undone.')) {
      return;
    }

    try {
      await apiService.deleteAPIKey(keyId);
      setMessage({ type: 'success', text: 'API key deleted successfully' });
      fetchAPIKeys();
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to delete API key' });
    }
  };

  const handleTestAPIKey = async (keyId: string) => {
    try {
      await apiService.testAPIKey(keyId);
      setMessage({ type: 'success', text: 'Connection test successful!' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Connection test failed' });
    }
  };

  const handleAddAIKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmittingAIKey(true);
    setMessage(null);

    try {
      await apiService.addAIKey({
        provider: newAIKey.provider,
        api_key: newAIKey.apiKey,
      });
      setMessage({ type: 'success', text: 'AI key added successfully!' });
      setShowAddAIKeyModal(false);
      setNewAIKey({ provider: 'claude', apiKey: '' });
      fetchAIKeys();
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to add AI key' });
    } finally {
      setIsSubmittingAIKey(false);
    }
  };

  const handleDeleteAIKey = async (keyId: string) => {
    if (!confirm('Are you sure you want to delete this AI key? This action cannot be undone.')) {
      return;
    }

    try {
      await apiService.deleteAIKey(keyId);
      setMessage({ type: 'success', text: 'AI key deleted successfully' });
      fetchAIKeys();
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to delete AI key' });
    }
  };

  const handleTestAIKey = async (keyId: string) => {
    try {
      await apiService.testAIKey(keyId);
      setMessage({ type: 'success', text: 'AI key validation successful!' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'AI key validation failed' });
    }
  };

  const getProviderIcon = (provider: string) => {
    switch (provider.toLowerCase()) {
      case 'claude': return '🤖';
      case 'openai': return '🧠';
      case 'deepseek': return '🔍';
      default: return '🤖';
    }
  };

  const getProviderName = (provider: string) => {
    switch (provider.toLowerCase()) {
      case 'claude': return 'Anthropic Claude';
      case 'openai': return 'OpenAI';
      case 'deepseek': return 'DeepSeek';
      default: return provider;
    }
  };

  return (
    <div className="max-w-5xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <p className="text-gray-400">Manage your profile, API keys, and preferences</p>
      </div>

      {/* Tabs */}
      <div className="flex gap-2 mb-6 border-b border-dark-700">
        <button
          onClick={() => changeTab('profile')}
          className={`flex items-center gap-2 px-4 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'profile'
              ? 'text-primary-500 border-primary-500'
              : 'text-gray-400 border-transparent hover:text-gray-300'
          }`}
        >
          <User className="w-4 h-4" />
          Profile
        </button>
        <button
          onClick={() => changeTab('binance')}
          className={`flex items-center gap-2 px-4 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'binance'
              ? 'text-primary-500 border-primary-500'
              : 'text-gray-400 border-transparent hover:text-gray-300'
          }`}
        >
          <Key className="w-4 h-4" />
          Binance API Keys
        </button>
        <button
          onClick={() => changeTab('ai')}
          className={`flex items-center gap-2 px-4 py-3 font-medium transition-colors border-b-2 ${
            activeTab === 'ai'
              ? 'text-primary-500 border-primary-500'
              : 'text-gray-400 border-transparent hover:text-gray-300'
          }`}
        >
          <Brain className="w-4 h-4" />
          AI API Keys
        </button>
      </div>

      {/* Message Alert */}
      {message && (
        <div className={`mb-6 p-4 rounded-lg flex items-center gap-2 ${
          message.type === 'success'
            ? 'bg-green-900/50 border border-green-500 text-green-300'
            : 'bg-red-900/50 border border-red-500 text-red-300'
        }`}>
          {message.type === 'success' ? (
            <CheckCircle className="w-5 h-5" />
          ) : (
            <AlertTriangle className="w-5 h-5" />
          )}
          {message.text}
        </div>
      )}

      {/* Tab Content */}
      <div className="space-y-6">
        {/* Profile Tab */}
        {activeTab === 'profile' && (
          <div className="space-y-6">
            {/* User Info Card */}
            <div className="bg-dark-800 rounded-lg border border-dark-700 p-6">
              <h2 className="text-lg font-semibold text-white mb-4">Account Information</h2>
              <div className="grid grid-cols-2 gap-4 text-sm">
                <div>
                  <p className="text-gray-400">Email</p>
                  <p className="text-white font-medium">{user?.email}</p>
                </div>
                <div>
                  <p className="text-gray-400">Subscription Tier</p>
                  <p className="text-white font-medium capitalize">{user?.subscription_tier}</p>
                </div>
                <div>
                  <p className="text-gray-400">Profit Share</p>
                  <p className="text-white font-medium">{user?.profit_share_pct}%</p>
                </div>
                <div>
                  <p className="text-gray-400">Member Since</p>
                  <p className="text-white font-medium">{new Date(user?.created_at || '').toLocaleDateString()}</p>
                </div>
              </div>
            </div>

            {/* Update Profile */}
            <div className="bg-dark-800 rounded-lg border border-dark-700 p-6">
              <h2 className="text-lg font-semibold text-white mb-4">Update Profile</h2>
              <form onSubmit={handleUpdateProfile} className="space-y-4">
                <div>
                  <label htmlFor="name" className="block text-sm font-medium text-gray-300 mb-2">
                    Name
                  </label>
                  <input
                    id="name"
                    type="text"
                    value={profileForm.name}
                    onChange={(e) => setProfileForm({ ...profileForm, name: e.target.value })}
                    className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    required
                  />
                </div>
                <button
                  type="submit"
                  disabled={isSubmittingProfile}
                  className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isSubmittingProfile ? 'Updating...' : 'Update Profile'}
                </button>
              </form>
            </div>

            {/* Change Password */}
            <div className="bg-dark-800 rounded-lg border border-dark-700 p-6">
              <h2 className="text-lg font-semibold text-white mb-4">Change Password</h2>
              <form onSubmit={handleChangePassword} className="space-y-4">
                <div>
                  <label htmlFor="currentPassword" className="block text-sm font-medium text-gray-300 mb-2">
                    Current Password
                  </label>
                  <input
                    id="currentPassword"
                    type="password"
                    value={passwordForm.currentPassword}
                    onChange={(e) => setPasswordForm({ ...passwordForm, currentPassword: e.target.value })}
                    className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    required
                  />
                </div>
                <div>
                  <label htmlFor="newPassword" className="block text-sm font-medium text-gray-300 mb-2">
                    New Password
                  </label>
                  <input
                    id="newPassword"
                    type="password"
                    value={passwordForm.newPassword}
                    onChange={(e) => setPasswordForm({ ...passwordForm, newPassword: e.target.value })}
                    className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    required
                  />
                </div>
                <div>
                  <label htmlFor="confirmPassword" className="block text-sm font-medium text-gray-300 mb-2">
                    Confirm New Password
                  </label>
                  <input
                    id="confirmPassword"
                    type="password"
                    value={passwordForm.confirmPassword}
                    onChange={(e) => setPasswordForm({ ...passwordForm, confirmPassword: e.target.value })}
                    className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                    required
                  />
                </div>
                <button
                  type="submit"
                  disabled={isSubmittingPassword}
                  className="px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {isSubmittingPassword ? 'Changing...' : 'Change Password'}
                </button>
              </form>
            </div>
          </div>
        )}

        {/* Binance API Keys Tab */}
        {activeTab === 'binance' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-xl font-semibold text-white">Binance API Keys</h2>
                <p className="text-gray-400">Manage your Binance API keys for trading</p>
              </div>
              <button
                onClick={() => setShowAddAPIKeyModal(true)}
                className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
              >
                <Plus className="w-5 h-5" />
                Add API Key
              </button>
            </div>

            {/* User API Status */}
            {userAPIStatus && (
              <div className={`rounded-lg border p-4 ${
                userAPIStatus.healthy
                  ? 'bg-green-900/20 border-green-600/50'
                  : 'bg-yellow-900/20 border-yellow-600/50'
              }`}>
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2">
                    {userAPIStatus.healthy ? (
                      <Wifi className="w-5 h-5 text-green-400" />
                    ) : (
                      <WifiOff className="w-5 h-5 text-yellow-400" />
                    )}
                    <h3 className={`font-medium ${userAPIStatus.healthy ? 'text-green-400' : 'text-yellow-400'}`}>
                      Your API Status
                    </h3>
                  </div>
                  <span className={`text-xs px-2 py-1 rounded ${
                    userAPIStatus.healthy
                      ? 'bg-green-600/20 text-green-400'
                      : 'bg-yellow-600/20 text-yellow-400'
                  }`}>
                    {userAPIStatus.healthy ? 'All Configured' : 'Setup Required'}
                  </span>
                </div>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  {Object.entries(userAPIStatus.services).map(([key, service]) => (
                    <div
                      key={key}
                      className={`p-2 rounded text-xs ${
                        service.status === 'ok'
                          ? 'bg-green-500/10 border border-green-500/30'
                          : service.status === 'not_configured'
                          ? 'bg-gray-500/10 border border-gray-500/30'
                          : 'bg-red-500/10 border border-red-500/30'
                      }`}
                    >
                      <div className="flex items-center gap-1.5 mb-1">
                        {key === 'binance_spot' && <Activity className="w-3 h-3" />}
                        {key === 'binance_futures' && <Activity className="w-3 h-3" />}
                        {key === 'ai_service' && <Bot className="w-3 h-3" />}
                        {key === 'database' && <Database className="w-3 h-3" />}
                        <span className="text-gray-300">
                          {key === 'binance_spot' ? 'Spot' :
                           key === 'binance_futures' ? 'Futures' :
                           key === 'ai_service' ? 'AI' : 'DB'}
                        </span>
                      </div>
                      <div className={`flex items-center gap-1 ${
                        service.status === 'ok' ? 'text-green-400' :
                        service.status === 'not_configured' ? 'text-gray-400' : 'text-red-400'
                      }`}>
                        {service.status === 'ok' ? (
                          <CheckCircle className="w-3 h-3" />
                        ) : (
                          <AlertTriangle className="w-3 h-3" />
                        )}
                        <span className="truncate">{service.message}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Your IP Address - For Binance Whitelist */}
            <div className="bg-blue-900/20 border border-blue-600/50 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <Globe className="w-5 h-5 text-blue-400 mt-0.5" />
                <div className="flex-1">
                  <h3 className="text-blue-400 font-medium mb-2">Your IP Address for Binance Whitelist</h3>
                  <div className="flex items-center gap-3">
                    <div className="flex-1 bg-dark-700 rounded-lg px-4 py-2 font-mono text-white flex items-center justify-between">
                      {isLoadingIP ? (
                        <span className="text-gray-400">Loading...</span>
                      ) : (
                        <span>{userIPAddress || 'Unable to detect'}</span>
                      )}
                      {userIPAddress && (
                        <button
                          onClick={copyIPToClipboard}
                          className="ml-2 p-1 hover:bg-dark-600 rounded transition-colors"
                          title="Copy IP Address"
                        >
                          {copiedIP ? (
                            <CheckCircle className="w-4 h-4 text-green-400" />
                          ) : (
                            <Copy className="w-4 h-4 text-gray-400" />
                          )}
                        </button>
                      )}
                    </div>
                  </div>
                  <p className="text-blue-200/70 text-sm mt-2">
                    Add this IP address to your Binance API key whitelist for enhanced security.
                  </p>
                </div>
              </div>
            </div>

            {/* Security Notice */}
            <div className="bg-yellow-900/30 border border-yellow-600/50 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <Shield className="w-5 h-5 text-yellow-400 mt-0.5" />
                <div>
                  <h3 className="text-yellow-400 font-medium">Security Notice</h3>
                  <p className="text-yellow-200/70 text-sm mt-1">
                    Your API keys are encrypted and stored securely. Only enable the permissions listed below.
                    <strong className="text-yellow-300"> Never enable withdrawal or transfer permissions!</strong>
                  </p>
                </div>
              </div>
            </div>

            {/* API Keys List */}
            {isLoadingAPIKeys ? (
              <div className="bg-dark-800 rounded-lg border border-dark-700 p-8 text-center">
                <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin mx-auto" />
                <p className="text-gray-400 mt-4">Loading API keys...</p>
              </div>
            ) : apiKeys.length === 0 ? (
              <div className="bg-dark-800 rounded-lg border border-dark-700 p-8 text-center">
                <Key className="w-12 h-12 text-gray-600 mx-auto mb-4" />
                <h3 className="text-lg font-medium text-white mb-2">No API Keys</h3>
                <p className="text-gray-400 mb-4">You haven't added any API keys yet. Add one to start trading.</p>
                <button
                  onClick={() => setShowAddAPIKeyModal(true)}
                  className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
                >
                  Add Your First API Key
                </button>
              </div>
            ) : (
              <div className="space-y-4">
                {apiKeys.map((key) => (
                  <div key={key.id} className="bg-dark-800 rounded-lg border border-dark-700 p-6">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-4">
                        <div className={`w-12 h-12 rounded-lg flex items-center justify-center ${
                          key.is_testnet ? 'bg-yellow-600/20' : 'bg-green-600/20'
                        }`}>
                          <Key className={`w-6 h-6 ${key.is_testnet ? 'text-yellow-400' : 'text-green-400'}`} />
                        </div>
                        <div>
                          <div className="flex items-center gap-2">
                            <h3 className="text-lg font-medium text-white">
                              Binance {key.is_testnet ? 'Testnet' : 'Mainnet'}
                            </h3>
                            <span className={`px-2 py-0.5 rounded text-xs ${
                              key.is_active
                                ? 'bg-green-600/20 text-green-400'
                                : 'bg-red-600/20 text-red-400'
                            }`}>
                              {key.is_active ? 'Active' : 'Inactive'}
                            </span>
                          </div>
                          <p className="text-gray-400 text-sm">
                            API Key ending in <span className="font-mono">...{key.api_key_last_four}</span>
                          </p>
                          <p className="text-gray-500 text-xs mt-1">
                            Added {new Date(key.created_at).toLocaleDateString()}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => handleTestAPIKey(key.id)}
                          className="p-2 text-gray-400 hover:text-white hover:bg-dark-700 rounded-lg transition-colors"
                          title="Test Connection"
                        >
                          <RefreshCw className="w-5 h-5" />
                        </button>
                        <button
                          onClick={() => handleDeleteAPIKey(key.id)}
                          className="p-2 text-red-400 hover:text-red-300 hover:bg-red-900/30 rounded-lg transition-colors"
                          title="Delete API Key"
                        >
                          <Trash2 className="w-5 h-5" />
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Detailed Setup Instructions */}
            <div className="bg-dark-800 rounded-lg border border-dark-700 p-6">
              <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <Key className="w-5 h-5 text-primary-400" />
                How to Create Your Binance API Key
              </h3>

              {/* Step-by-step instructions */}
              <div className="space-y-4">
                <div className="border-l-2 border-primary-500 pl-4">
                  <h4 className="text-white font-medium mb-2">Step 1: Access API Management</h4>
                  <ol className="list-decimal list-inside text-gray-400 space-y-1 text-sm">
                    <li>Log in to your <a href="https://www.binance.com" target="_blank" rel="noopener noreferrer" className="text-primary-400 hover:underline inline-flex items-center gap-1">Binance account <ExternalLink className="w-3 h-3" /></a></li>
                    <li>Go to <strong className="text-white">Profile</strong> &gt; <strong className="text-white">API Management</strong></li>
                    <li>Click <strong className="text-white">"Create API"</strong> and choose <strong className="text-white">"System Generated"</strong></li>
                    <li>Complete 2FA verification</li>
                  </ol>
                </div>

                <div className="border-l-2 border-blue-500 pl-4">
                  <h4 className="text-white font-medium mb-2">Step 2: Whitelist Your IP Address</h4>
                  <ol className="list-decimal list-inside text-gray-400 space-y-1 text-sm">
                    <li>In the API key settings, find <strong className="text-white">"Restrict access to trusted IPs only"</strong></li>
                    <li>Click <strong className="text-white">"Edit"</strong> next to IP access restrictions</li>
                    <li>Add your IP address: <code className="bg-dark-600 px-2 py-0.5 rounded text-blue-400">{userIPAddress || 'Loading...'}</code></li>
                    <li>Save the changes</li>
                  </ol>
                </div>

                <div className="border-l-2 border-green-500 pl-4">
                  <h4 className="text-white font-medium mb-2">Step 3: Enable Required Permissions</h4>
                  <p className="text-gray-400 text-sm mb-2">In API Restrictions, enable <strong className="text-green-400">ONLY</strong> these options:</p>
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 bg-green-900/20 p-2 rounded">
                      <CheckCircle className="w-4 h-4 text-green-400" />
                      <span className="text-green-300 font-medium">Enable Reading</span>
                      <span className="text-gray-400 text-xs">(Required for account info)</span>
                    </div>
                    <div className="flex items-center gap-2 bg-green-900/20 p-2 rounded">
                      <CheckCircle className="w-4 h-4 text-green-400" />
                      <span className="text-green-300 font-medium">Enable Spot & Margin Trading</span>
                      <span className="text-gray-400 text-xs">(Required for spot trades)</span>
                    </div>
                    <div className="flex items-center gap-2 bg-green-900/20 p-2 rounded">
                      <CheckCircle className="w-4 h-4 text-green-400" />
                      <span className="text-green-300 font-medium">Enable Futures</span>
                      <span className="text-gray-400 text-xs">(Required for futures trades)</span>
                    </div>
                  </div>
                </div>

                <div className="border-l-2 border-red-500 pl-4">
                  <h4 className="text-white font-medium mb-2 flex items-center gap-2">
                    <AlertTriangle className="w-4 h-4 text-red-400" />
                    Do NOT Enable These Options
                  </h4>
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 bg-red-900/20 p-2 rounded">
                      <AlertTriangle className="w-4 h-4 text-red-400" />
                      <span className="text-red-300">Enable Withdrawals</span>
                      <span className="text-gray-400 text-xs">- Never enable this!</span>
                    </div>
                    <div className="flex items-center gap-2 bg-red-900/20 p-2 rounded">
                      <AlertTriangle className="w-4 h-4 text-red-400" />
                      <span className="text-red-300">Enable Internal Transfer</span>
                      <span className="text-gray-400 text-xs">- Not needed</span>
                    </div>
                    <div className="flex items-center gap-2 bg-red-900/20 p-2 rounded">
                      <AlertTriangle className="w-4 h-4 text-red-400" />
                      <span className="text-red-300">Enable Vanilla Options</span>
                      <span className="text-gray-400 text-xs">- Not needed</span>
                    </div>
                    <div className="flex items-center gap-2 bg-red-900/20 p-2 rounded">
                      <AlertTriangle className="w-4 h-4 text-red-400" />
                      <span className="text-red-300">Permits Universal Transfer</span>
                      <span className="text-gray-400 text-xs">- Not needed</span>
                    </div>
                  </div>
                </div>

                <div className="border-l-2 border-purple-500 pl-4">
                  <h4 className="text-white font-medium mb-2">Step 4: Copy Your Keys</h4>
                  <ol className="list-decimal list-inside text-gray-400 space-y-1 text-sm">
                    <li>Copy your <strong className="text-white">API Key</strong> (public key)</li>
                    <li>Copy your <strong className="text-white">Secret Key</strong> - <span className="text-red-400">This is shown only once!</span></li>
                    <li>Paste both keys in the form above</li>
                  </ol>
                </div>
              </div>

              {/* Testnet info */}
              <div className="mt-6 p-4 bg-dark-700 rounded-lg">
                <h4 className="text-white font-medium mb-2 flex items-center gap-2">
                  <Shield className="w-4 h-4 text-yellow-400" />
                  Want to Practice First?
                </h4>
                <p className="text-gray-400 text-sm">
                  Use Binance Testnet for risk-free practice with fake funds:
                </p>
                <div className="mt-2 flex flex-wrap gap-2">
                  <a
                    href="https://testnet.binancefuture.com"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 px-3 py-1 bg-yellow-600/20 text-yellow-400 rounded text-sm hover:bg-yellow-600/30 transition-colors"
                  >
                    <ExternalLink className="w-3 h-3" />
                    Futures Testnet
                  </a>
                  <a
                    href="https://testnet.binance.vision"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 px-3 py-1 bg-yellow-600/20 text-yellow-400 rounded text-sm hover:bg-yellow-600/30 transition-colors"
                  >
                    <ExternalLink className="w-3 h-3" />
                    Spot Testnet
                  </a>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* AI API Keys Tab */}
        {activeTab === 'ai' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-xl font-semibold text-white">AI API Keys</h2>
                <p className="text-gray-400">Manage your AI provider API keys for Ginie autopilot</p>
              </div>
              <button
                onClick={() => setShowAddAIKeyModal(true)}
                className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
              >
                <Plus className="w-4 h-4" />
                Add AI Key
              </button>
            </div>

            {/* Info Box */}
            <div className="p-4 bg-blue-900/20 border border-blue-500/50 rounded-lg">
              <div className="flex items-start gap-3">
                <Shield className="w-5 h-5 text-blue-400 mt-0.5" />
                <div className="flex-1">
                  <h3 className="text-blue-400 font-semibold mb-1">About AI API Keys</h3>
                  <p className="text-gray-300 text-sm">
                    Configure your personal AI provider API keys for Ginie autopilot. Your keys are encrypted and stored
                    securely. Supported providers: Anthropic Claude, OpenAI, and DeepSeek.
                  </p>
                </div>
              </div>
            </div>

            {/* AI Keys List */}
            <div className="bg-dark-800 rounded-lg border border-gray-700 overflow-hidden">
              {isLoadingAIKeys ? (
                <div className="p-8 text-center text-gray-400">
                  <RefreshCw className="w-8 h-8 animate-spin mx-auto mb-2" />
                  Loading AI keys...
                </div>
              ) : aiKeys.length === 0 ? (
                <div className="p-8 text-center text-gray-400">
                  <Brain className="w-12 h-12 mx-auto mb-3 opacity-50" />
                  <p className="text-lg mb-2">No AI keys configured</p>
                  <p className="text-sm">Add your AI provider API keys to enable Ginie autopilot</p>
                </div>
              ) : (
                <div className="divide-y divide-gray-700">
                  {aiKeys.map((key) => (
                    <div key={key.id} className="p-4 hover:bg-dark-700 transition-colors">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-4 flex-1">
                          <div className="text-3xl">{getProviderIcon(key.provider)}</div>
                          <div className="flex-1">
                            <div className="flex items-center gap-2">
                              <h3 className="text-white font-semibold">{getProviderName(key.provider)}</h3>
                              {key.is_active ? (
                                <span className="px-2 py-0.5 bg-green-900/30 text-green-400 text-xs rounded-full">
                                  Active
                                </span>
                              ) : (
                                <span className="px-2 py-0.5 bg-gray-700 text-gray-400 text-xs rounded-full">
                                  Inactive
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-4 mt-1 text-sm text-gray-400">
                              <span>Key: ****{key.key_last_four}</span>
                              <span>Added: {new Date(key.created_at).toLocaleDateString()}</span>
                            </div>
                          </div>
                        </div>

                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => handleTestAIKey(key.id)}
                            className="p-2 text-blue-400 hover:bg-blue-900/20 rounded-lg transition-colors"
                            title="Test connection"
                          >
                            <Shield className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleDeleteAIKey(key.id)}
                            className="p-2 text-red-400 hover:bg-red-900/20 rounded-lg transition-colors"
                            title="Delete key"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {/* Add API Key Modal */}
      {showAddAPIKeyModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-6 w-full max-w-md mx-4">
            <h2 className="text-xl font-bold text-white mb-4">Add API Key</h2>

            <form onSubmit={handleAddAPIKey} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Network Type
                </label>
                <div className="flex gap-4">
                  <label className="flex items-center">
                    <input
                      type="radio"
                      name="network"
                      checked={newAPIKey.isTestnet}
                      onChange={() => setNewAPIKey({ ...newAPIKey, isTestnet: true })}
                      className="w-4 h-4 text-primary-600 bg-dark-700 border-dark-600"
                    />
                    <span className="ml-2 text-gray-300">Testnet (Recommended)</span>
                  </label>
                  <label className="flex items-center">
                    <input
                      type="radio"
                      name="network"
                      checked={!newAPIKey.isTestnet}
                      onChange={() => setNewAPIKey({ ...newAPIKey, isTestnet: false })}
                      className="w-4 h-4 text-primary-600 bg-dark-700 border-dark-600"
                    />
                    <span className="ml-2 text-gray-300">Mainnet (Live)</span>
                  </label>
                </div>
              </div>

              <div>
                <label htmlFor="apiKey" className="block text-sm font-medium text-gray-300 mb-2">
                  API Key
                </label>
                <input
                  id="apiKey"
                  type="text"
                  value={newAPIKey.apiKey}
                  onChange={(e) => setNewAPIKey({ ...newAPIKey, apiKey: e.target.value })}
                  className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent font-mono text-sm"
                  placeholder="Enter your Binance API key"
                  required
                />
              </div>

              <div>
                <label htmlFor="secretKey" className="block text-sm font-medium text-gray-300 mb-2">
                  Secret Key
                </label>
                <div className="relative">
                  <input
                    id="secretKey"
                    type={showSecretKey ? 'text' : 'password'}
                    value={newAPIKey.secretKey}
                    onChange={(e) => setNewAPIKey({ ...newAPIKey, secretKey: e.target.value })}
                    className="w-full px-4 py-2 pr-10 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent font-mono text-sm"
                    placeholder="Enter your Binance secret key"
                    required
                  />
                  <button
                    type="button"
                    onClick={() => setShowSecretKey(!showSecretKey)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
                  >
                    {showSecretKey ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
                  </button>
                </div>
              </div>

              {!newAPIKey.isTestnet && (
                <div className="bg-red-900/30 border border-red-600/50 rounded-lg p-3">
                  <div className="flex items-start gap-2">
                    <AlertTriangle className="w-5 h-5 text-red-400 mt-0.5" />
                    <div>
                      <p className="text-red-300 text-sm font-medium">Live Trading Warning</p>
                      <p className="text-red-200/70 text-xs mt-1">
                        You are adding a mainnet API key. Real funds will be used for trading. Make sure you understand the risks.
                      </p>
                    </div>
                  </div>
                </div>
              )}

              <div className="flex gap-3 pt-4">
                <button
                  type="button"
                  onClick={() => {
                    setShowAddAPIKeyModal(false);
                    setNewAPIKey({ apiKey: '', secretKey: '', isTestnet: true });
                  }}
                  className="flex-1 px-4 py-2 bg-dark-700 text-gray-300 rounded-lg hover:bg-dark-600 transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isSubmittingAPIKey}
                  className="flex-1 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {isSubmittingAPIKey ? (
                    <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                  ) : (
                    <Plus className="w-5 h-5" />
                  )}
                  Add Key
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add AI Key Modal */}
      {showAddAIKeyModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-dark-800 rounded-lg border border-gray-700 max-w-md w-full">
            <div className="p-6">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-xl font-bold text-white">Add AI API Key</h2>
                <button
                  onClick={() => {
                    setShowAddAIKeyModal(false);
                    setNewAIKey({ provider: 'claude', apiKey: '' });
                  }}
                  className="text-gray-400 hover:text-white"
                >
                  ×
                </button>
              </div>

              <form onSubmit={handleAddAIKey} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">Provider</label>
                  <select
                    value={newAIKey.provider}
                    onChange={(e) => setNewAIKey({ ...newAIKey, provider: e.target.value })}
                    className="w-full px-4 py-2 bg-dark-700 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
                    required
                  >
                    <option value="claude">Anthropic Claude</option>
                    <option value="openai">OpenAI</option>
                    <option value="deepseek">DeepSeek</option>
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">API Key</label>
                  <div className="relative">
                    <input
                      type={showAIAPIKey ? 'text' : 'password'}
                      value={newAIKey.apiKey}
                      onChange={(e) => setNewAIKey({ ...newAIKey, apiKey: e.target.value })}
                      className="w-full px-4 py-2 bg-dark-700 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
                      placeholder="sk-ant-... or sk-..."
                      required
                    />
                    <button
                      type="button"
                      onClick={() => setShowAIAPIKey(!showAIAPIKey)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
                    >
                      {showAIAPIKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                  </div>
                  <p className="text-xs text-gray-500 mt-1">
                    Your API key will be encrypted and stored securely
                  </p>
                </div>

                <div className="flex gap-3 pt-4">
                  <button
                    type="button"
                    onClick={() => {
                      setShowAddAIKeyModal(false);
                      setNewAIKey({ provider: 'claude', apiKey: '' });
                    }}
                    className="flex-1 px-4 py-2 bg-dark-700 text-white rounded-lg hover:bg-dark-600 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmittingAIKey}
                    className="flex-1 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isSubmittingAIKey ? 'Adding...' : 'Add Key'}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default Settings;
