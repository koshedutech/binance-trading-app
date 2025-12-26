import React, { useState, useEffect } from 'react';
import { Users, Settings, Mail, Save, AlertCircle, CheckCircle, Eye, EyeOff, Shield } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext';
import { apiService } from '../services/api';
import { Navigate } from 'react-router-dom';

interface User {
  id: string;
  email: string;
  name: string;
  subscription_tier: string;
  is_admin: boolean;
  email_verified: boolean;
  created_at: string;
  last_login_at?: string;
}

interface SystemSetting {
  key: string;
  value: string;
  description?: string;
}

interface SMTPConfig {
  host: string;
  port: number;
  username: string;
  password: string;
  from_email: string;
  from_name: string;
  use_tls: boolean;
}

const AdminSettings: React.FC = () => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<'users' | 'settings' | 'smtp'>('users');
  const [isLoading, setIsLoading] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Users state
  const [users, setUsers] = useState<User[]>([]);
  const [usersLoading, setUsersLoading] = useState(false);

  // System Settings state
  const [systemSettings, setSystemSettings] = useState<SystemSetting[]>([]);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [editValue, setEditValue] = useState('');

  // SMTP state
  const [smtpConfig, setSmtpConfig] = useState<SMTPConfig>({
    host: '',
    port: 587,
    username: '',
    password: '',
    from_email: '',
    from_name: '',
    use_tls: true,
  });
  const [smtpLoading, setSmtpLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  // Check if user is admin
  if (!user?.is_admin) {
    return <Navigate to="/dashboard" replace />;
  }

  // Load users
  const loadUsers = async () => {
    setUsersLoading(true);
    try {
      const response = await apiService.get<{ success: boolean; data: User[]; total: number }>('/admin/users');
      setUsers(response.data.data || []);
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to load users' });
    } finally {
      setUsersLoading(false);
    }
  };

  // Load system settings
  const loadSystemSettings = async () => {
    setSettingsLoading(true);
    try {
      const response = await apiService.get<{ success: boolean; settings: SystemSetting[] }>('/admin/settings');
      setSystemSettings(response.data.settings || []);
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to load system settings' });
    } finally {
      setSettingsLoading(false);
    }
  };

  // Load SMTP config
  const loadSMTPConfig = async () => {
    setSmtpLoading(true);
    try {
      const response = await apiService.get<{ success: boolean; settings: Record<string, string> }>('/admin/settings/smtp');
      const settings = response.data.settings || {};
      // Map backend field names to frontend field names
      setSmtpConfig({
        host: settings.smtp_host || '',
        port: parseInt(settings.smtp_port || '587', 10),
        username: settings.smtp_username || '',
        password: settings.smtp_password || '',
        from_email: settings.smtp_from || '',
        from_name: settings.smtp_from_name || '',
        use_tls: settings.smtp_use_tls === 'true',
      });
    } catch (error) {
      // 404 is expected if no SMTP settings exist yet
      setSmtpConfig({
        host: '',
        port: 587,
        username: '',
        password: '',
        from_email: '',
        from_name: '',
        use_tls: true,
      });
    } finally {
      setSmtpLoading(false);
    }
  };

  // Load data based on active tab
  useEffect(() => {
    setMessage(null);
    if (activeTab === 'users') {
      loadUsers();
    } else if (activeTab === 'settings') {
      loadSystemSettings();
    } else if (activeTab === 'smtp') {
      loadSMTPConfig();
    }
  }, [activeTab]);

  // Handle setting edit
  const handleEditSetting = (setting: SystemSetting) => {
    setEditingKey(setting.key);
    setEditValue(setting.value);
  };

  const handleSaveSetting = async (key: string) => {
    setIsLoading(true);
    setMessage(null);
    try {
      await apiService.put(`/admin/settings/${encodeURIComponent(key)}`, { value: editValue });
      setMessage({ type: 'success', text: 'Setting updated successfully' });
      setEditingKey(null);
      await loadSystemSettings();
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to update setting' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleCancelEdit = () => {
    setEditingKey(null);
    setEditValue('');
  };

  // Handle SMTP form submit
  const handleSMTPSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setMessage(null);

    try {
      // Map frontend field names to backend field names
      const backendConfig = {
        smtp_host: smtpConfig.host,
        smtp_port: String(smtpConfig.port),
        smtp_username: smtpConfig.username,
        smtp_password: smtpConfig.password,
        smtp_from: smtpConfig.from_email,
        smtp_from_name: smtpConfig.from_name,
        smtp_use_tls: String(smtpConfig.use_tls),
      };
      await apiService.put('/admin/settings/smtp', backendConfig);
      setMessage({ type: 'success', text: 'SMTP configuration updated successfully' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to update SMTP configuration' });
    } finally {
      setIsLoading(false);
    }
  };

  // Format date
  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  // Get tier badge color
  const getTierBadgeColor = (tier: string) => {
    switch (tier) {
      case 'whale':
        return 'bg-yellow-600 text-yellow-100';
      case 'pro':
        return 'bg-purple-600 text-purple-100';
      case 'trader':
        return 'bg-blue-600 text-blue-100';
      default:
        return 'bg-gray-600 text-gray-100';
    }
  };

  return (
    <div className="max-w-6xl mx-auto">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-white flex items-center gap-2">
          <Shield className="w-7 h-7 text-primary-400" />
          Admin Settings
        </h1>
        <p className="text-gray-400">Manage users, system settings, and SMTP configuration</p>
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
            <AlertCircle className="w-5 h-5" />
          )}
          {message.text}
        </div>
      )}

      {/* Tab Navigation */}
      <div className="flex border-b border-dark-700 mb-6">
        <button
          onClick={() => setActiveTab('users')}
          className={`px-4 py-3 font-medium transition-colors relative ${
            activeTab === 'users'
              ? 'text-primary-400'
              : 'text-gray-400 hover:text-white'
          }`}
        >
          <div className="flex items-center gap-2">
            <Users className="w-4 h-4" />
            Users
          </div>
          {activeTab === 'users' && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-500" />
          )}
        </button>
        <button
          onClick={() => setActiveTab('settings')}
          className={`px-4 py-3 font-medium transition-colors relative ${
            activeTab === 'settings'
              ? 'text-primary-400'
              : 'text-gray-400 hover:text-white'
          }`}
        >
          <div className="flex items-center gap-2">
            <Settings className="w-4 h-4" />
            System Settings
          </div>
          {activeTab === 'settings' && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-500" />
          )}
        </button>
        <button
          onClick={() => setActiveTab('smtp')}
          className={`px-4 py-3 font-medium transition-colors relative ${
            activeTab === 'smtp'
              ? 'text-primary-400'
              : 'text-gray-400 hover:text-white'
          }`}
        >
          <div className="flex items-center gap-2">
            <Mail className="w-4 h-4" />
            SMTP
          </div>
          {activeTab === 'smtp' && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-500" />
          )}
        </button>
      </div>

      {/* Users Tab */}
      {activeTab === 'users' && (
        <div className="bg-dark-800 rounded-lg border border-dark-700">
          <div className="p-6 border-b border-dark-700">
            <h2 className="text-lg font-semibold text-white">User Management</h2>
            <p className="text-sm text-gray-400">View and manage all registered users</p>
          </div>
          <div className="overflow-x-auto">
            {usersLoading ? (
              <div className="flex items-center justify-center p-12">
                <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : users.length === 0 ? (
              <div className="text-center p-12 text-gray-400">
                No users found
              </div>
            ) : (
              <table className="w-full">
                <thead className="bg-dark-700">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                      Name
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                      Email
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                      Tier
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                      Created
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                      Last Login
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-dark-700">
                  {users.map((u) => (
                    <tr key={u.id} className="hover:bg-dark-700/50">
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center">
                          <div className="text-sm font-medium text-white">{u.name}</div>
                          {u.is_admin && (
                            <span className="ml-2 px-2 py-0.5 text-xs font-medium bg-red-600 text-red-100 rounded">
                              Admin
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <div className="flex items-center gap-2">
                          <div className="text-sm text-gray-300">{u.email}</div>
                          {u.email_verified && (
                            <CheckCircle className="w-4 h-4 text-green-400" />
                          )}
                        </div>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`px-3 py-1 rounded-full text-xs font-medium ${getTierBadgeColor(u.subscription_tier)}`}>
                          {u.subscription_tier.toUpperCase()}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="text-sm text-green-400">Active</span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-400">
                        {formatDate(u.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-400">
                        {u.last_login_at ? formatDate(u.last_login_at) : 'Never'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}

      {/* System Settings Tab */}
      {activeTab === 'settings' && (
        <div className="bg-dark-800 rounded-lg border border-dark-700">
          <div className="p-6 border-b border-dark-700">
            <h2 className="text-lg font-semibold text-white">System Settings</h2>
            <p className="text-sm text-gray-400">Configure system-wide settings and parameters</p>
          </div>
          <div className="p-6">
            {settingsLoading ? (
              <div className="flex items-center justify-center p-12">
                <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : systemSettings.length === 0 ? (
              <div className="text-center p-12 text-gray-400">
                No settings configured
              </div>
            ) : (
              <div className="space-y-4">
                {systemSettings.map((setting) => (
                  <div key={setting.key} className="bg-dark-700 rounded-lg p-4">
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <h3 className="text-sm font-medium text-white">{setting.key}</h3>
                        {setting.description && (
                          <p className="text-xs text-gray-400 mt-1">{setting.description}</p>
                        )}
                        {editingKey === setting.key ? (
                          <div className="mt-2 flex items-center gap-2">
                            <input
                              type="text"
                              value={editValue}
                              onChange={(e) => setEditValue(e.target.value)}
                              className="flex-1 px-3 py-2 bg-dark-600 border border-dark-500 rounded-lg text-white text-sm focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                              autoFocus
                            />
                            <button
                              onClick={() => handleSaveSetting(setting.key)}
                              disabled={isLoading}
                              className="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors disabled:opacity-50"
                            >
                              <Save className="w-4 h-4" />
                            </button>
                            <button
                              onClick={handleCancelEdit}
                              disabled={isLoading}
                              className="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 transition-colors disabled:opacity-50"
                            >
                              Cancel
                            </button>
                          </div>
                        ) : (
                          <div className="mt-2 flex items-center justify-between">
                            <span className="text-sm text-gray-300 font-mono">{setting.value}</span>
                            <button
                              onClick={() => handleEditSetting(setting)}
                              className="text-primary-400 hover:text-primary-300 text-sm"
                            >
                              Edit
                            </button>
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* SMTP Tab */}
      {activeTab === 'smtp' && (
        <div className="bg-dark-800 rounded-lg border border-dark-700">
          <div className="p-6 border-b border-dark-700">
            <h2 className="text-lg font-semibold text-white">SMTP Configuration</h2>
            <p className="text-sm text-gray-400">Configure email server settings for notifications</p>
          </div>
          <div className="p-6">
            {smtpLoading ? (
              <div className="flex items-center justify-center p-12">
                <div className="w-8 h-8 border-2 border-primary-500 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : (
              <form onSubmit={handleSMTPSubmit} className="space-y-6">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="smtp_host" className="block text-sm font-medium text-gray-300 mb-2">
                      SMTP Host
                    </label>
                    <input
                      id="smtp_host"
                      type="text"
                      value={smtpConfig.host}
                      onChange={(e) => setSmtpConfig({ ...smtpConfig, host: e.target.value })}
                      className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                      placeholder="smtp.example.com"
                      required
                    />
                  </div>

                  <div>
                    <label htmlFor="smtp_port" className="block text-sm font-medium text-gray-300 mb-2">
                      Port
                    </label>
                    <input
                      id="smtp_port"
                      type="number"
                      value={smtpConfig.port}
                      onChange={(e) => setSmtpConfig({ ...smtpConfig, port: parseInt(e.target.value) })}
                      className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                      placeholder="587"
                      required
                    />
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="smtp_username" className="block text-sm font-medium text-gray-300 mb-2">
                      Username
                    </label>
                    <input
                      id="smtp_username"
                      type="text"
                      value={smtpConfig.username}
                      onChange={(e) => setSmtpConfig({ ...smtpConfig, username: e.target.value })}
                      className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                      placeholder="username@example.com"
                      required
                    />
                  </div>

                  <div>
                    <label htmlFor="smtp_password" className="block text-sm font-medium text-gray-300 mb-2">
                      Password
                    </label>
                    <div className="relative">
                      <input
                        id="smtp_password"
                        type={showPassword ? 'text' : 'password'}
                        value={smtpConfig.password}
                        onChange={(e) => setSmtpConfig({ ...smtpConfig, password: e.target.value })}
                        className="w-full px-4 py-2 pr-10 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                        placeholder="••••••••"
                        required
                      />
                      <button
                        type="button"
                        onClick={() => setShowPassword(!showPassword)}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
                      >
                        {showPassword ? <EyeOff className="w-5 h-5" /> : <Eye className="w-5 h-5" />}
                      </button>
                    </div>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label htmlFor="from_email" className="block text-sm font-medium text-gray-300 mb-2">
                      From Email
                    </label>
                    <input
                      id="from_email"
                      type="email"
                      value={smtpConfig.from_email}
                      onChange={(e) => setSmtpConfig({ ...smtpConfig, from_email: e.target.value })}
                      className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                      placeholder="noreply@example.com"
                      required
                    />
                  </div>

                  <div>
                    <label htmlFor="from_name" className="block text-sm font-medium text-gray-300 mb-2">
                      From Name
                    </label>
                    <input
                      id="from_name"
                      type="text"
                      value={smtpConfig.from_name}
                      onChange={(e) => setSmtpConfig({ ...smtpConfig, from_name: e.target.value })}
                      className="w-full px-4 py-2 bg-dark-700 border border-dark-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-transparent"
                      placeholder="Trading Bot"
                      required
                    />
                  </div>
                </div>

                <div className="flex items-center">
                  <input
                    id="use_tls"
                    type="checkbox"
                    checked={smtpConfig.use_tls}
                    onChange={(e) => setSmtpConfig({ ...smtpConfig, use_tls: e.target.checked })}
                    className="w-4 h-4 text-primary-600 bg-dark-700 border-dark-600 rounded focus:ring-primary-500"
                  />
                  <label htmlFor="use_tls" className="ml-2 text-sm text-gray-300">
                    Use TLS/STARTTLS (recommended)
                  </label>
                </div>

                <div className="pt-4">
                  <button
                    type="submit"
                    disabled={isLoading}
                    className="flex items-center gap-2 px-6 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isLoading ? (
                      <div className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                    ) : (
                      <Save className="w-5 h-5" />
                    )}
                    Save SMTP Configuration
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default AdminSettings;
