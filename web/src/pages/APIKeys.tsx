import React, { useState, useEffect } from 'react';
import { Key, Plus, Trash2, Shield, AlertTriangle, CheckCircle, Eye, EyeOff, RefreshCw } from 'lucide-react';
import { apiService } from '../services/api';

interface APIKey {
  id: string;
  exchange: string;
  api_key_last_four: string;
  is_testnet: boolean;
  is_active: boolean;
  created_at: string;
}

const APIKeys: React.FC = () => {
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Form state for adding new key
  const [newKey, setNewKey] = useState({
    apiKey: '',
    secretKey: '',
    isTestnet: true,
  });
  const [showSecretKey, setShowSecretKey] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    fetchAPIKeys();
  }, []);

  const fetchAPIKeys = async () => {
    try {
      setIsLoading(true);
      const keys = await apiService.getAPIKeys();
      setApiKeys(keys);
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to load API keys' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleAddKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setMessage(null);

    try {
      await apiService.addAPIKey({
        api_key: newKey.apiKey,
        secret_key: newKey.secretKey,
        is_testnet: newKey.isTestnet,
      });
      setMessage({ type: 'success', text: 'API key added successfully!' });
      setShowAddModal(false);
      setNewKey({ apiKey: '', secretKey: '', isTestnet: true });
      fetchAPIKeys();
      // Notify other components that API keys have changed
      window.dispatchEvent(new CustomEvent('api-key-changed'));
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to add API key' });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteKey = async (keyId: string) => {
    if (!confirm('Are you sure you want to delete this API key? This action cannot be undone.')) {
      return;
    }

    try {
      await apiService.deleteAPIKey(keyId);
      setMessage({ type: 'success', text: 'API key deleted successfully' });
      fetchAPIKeys();
      // Notify other components that API keys have changed
      window.dispatchEvent(new CustomEvent('api-key-changed'));
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to delete API key' });
    }
  };

  const handleTestConnection = async (keyId: string) => {
    try {
      await apiService.testAPIKey(keyId);
      setMessage({ type: 'success', text: 'Connection test successful!' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Connection test failed' });
    }
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">API Keys</h1>
          <p className="text-gray-400">Manage your Binance API keys for trading</p>
        </div>
        <button
          onClick={() => setShowAddModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
        >
          <Plus className="w-5 h-5" />
          Add API Key
        </button>
      </div>

      {/* Security Notice */}
      <div className="bg-yellow-900/30 border border-yellow-600/50 rounded-lg p-4 mb-6">
        <div className="flex items-start gap-3">
          <Shield className="w-5 h-5 text-yellow-400 mt-0.5" />
          <div>
            <h3 className="text-yellow-400 font-medium">Security Notice</h3>
            <p className="text-yellow-200/70 text-sm mt-1">
              Your API keys are encrypted and stored securely in our vault. We only support API keys with spot and futures trading permissions.
              Never share your API keys or secret keys with anyone.
            </p>
          </div>
        </div>
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

      {/* API Keys List */}
      {isLoading ? (
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
            onClick={() => setShowAddModal(true)}
            className="px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
          >
            Add Your First API Key
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {apiKeys.map((key) => (
            <div
              key={key.id}
              className="bg-dark-800 rounded-lg border border-dark-700 p-6"
            >
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
                    onClick={() => handleTestConnection(key.id)}
                    className="p-2 text-gray-400 hover:text-white hover:bg-dark-700 rounded-lg transition-colors"
                    title="Test Connection"
                  >
                    <RefreshCw className="w-5 h-5" />
                  </button>
                  <button
                    onClick={() => handleDeleteKey(key.id)}
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

      {/* Add API Key Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-dark-800 rounded-lg border border-dark-700 p-6 w-full max-w-md mx-4">
            <h2 className="text-xl font-bold text-white mb-4">Add API Key</h2>

            <form onSubmit={handleAddKey} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Network Type
                </label>
                <div className="flex gap-4">
                  <label className="flex items-center">
                    <input
                      type="radio"
                      name="network"
                      checked={newKey.isTestnet}
                      onChange={() => setNewKey({ ...newKey, isTestnet: true })}
                      className="w-4 h-4 text-primary-600 bg-dark-700 border-dark-600"
                    />
                    <span className="ml-2 text-gray-300">Testnet (Recommended)</span>
                  </label>
                  <label className="flex items-center">
                    <input
                      type="radio"
                      name="network"
                      checked={!newKey.isTestnet}
                      onChange={() => setNewKey({ ...newKey, isTestnet: false })}
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
                  value={newKey.apiKey}
                  onChange={(e) => setNewKey({ ...newKey, apiKey: e.target.value })}
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
                    value={newKey.secretKey}
                    onChange={(e) => setNewKey({ ...newKey, secretKey: e.target.value })}
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

              {!newKey.isTestnet && (
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
                    setShowAddModal(false);
                    setNewKey({ apiKey: '', secretKey: '', isTestnet: true });
                  }}
                  className="flex-1 px-4 py-2 bg-dark-700 text-gray-300 rounded-lg hover:bg-dark-600 transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting}
                  className="flex-1 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {isSubmitting ? (
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

      {/* Instructions */}
      <div className="mt-8 bg-dark-800 rounded-lg border border-dark-700 p-6">
        <h3 className="text-lg font-semibold text-white mb-4">How to get your Binance API Keys</h3>
        <ol className="list-decimal list-inside text-gray-400 space-y-2 text-sm">
          <li>Log in to your Binance account</li>
          <li>Go to Account &gt; API Management</li>
          <li>Click "Create API" and complete verification</li>
          <li>Enable "Enable Spot & Margin Trading" and "Enable Futures" permissions</li>
          <li>For testnet, visit testnet.binancefuture.com and create separate keys</li>
          <li>Copy your API Key and Secret Key (secret is only shown once!)</li>
          <li>Paste them here and click "Add Key"</li>
        </ol>
        <div className="mt-4 p-3 bg-dark-700 rounded-lg">
          <p className="text-gray-300 text-sm">
            <strong>Tip:</strong> Start with testnet keys to practice trading without risking real funds.
          </p>
        </div>
      </div>
    </div>
  );
};

export default APIKeys;
