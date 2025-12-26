import React, { useState, useEffect } from 'react';
import { Brain, Plus, Trash2, Shield, AlertTriangle, CheckCircle, Eye, EyeOff, RefreshCw } from 'lucide-react';
import { apiService } from '../services/api';

interface AIKey {
  id: string;
  provider: string;
  key_last_four: string;
  is_active: boolean;
  created_at: string;
}

const AIKeys: React.FC = () => {
  const [aiKeys, setAIKeys] = useState<AIKey[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [showAddModal, setShowAddModal] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Form state for adding new key
  const [newKey, setNewKey] = useState({
    provider: 'claude',
    apiKey: '',
  });
  const [showAPIKey, setShowAPIKey] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    fetchAIKeys();
  }, []);

  const fetchAIKeys = async () => {
    try {
      setIsLoading(true);
      const keys = await apiService.getAIKeys();
      setAIKeys(keys);
    } catch (error) {
      setMessage({ type: 'error', text: 'Failed to load AI keys' });
    } finally {
      setIsLoading(false);
    }
  };

  const handleAddKey = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setMessage(null);

    try {
      await apiService.addAIKey({
        provider: newKey.provider,
        api_key: newKey.apiKey,
      });
      setMessage({ type: 'success', text: 'AI key added successfully!' });
      setShowAddModal(false);
      setNewKey({ provider: 'claude', apiKey: '' });
      fetchAIKeys();
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'Failed to add AI key' });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDeleteKey = async (keyId: string) => {
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

  const handleTestConnection = async (keyId: string) => {
    try {
      await apiService.testAIKey(keyId);
      setMessage({ type: 'success', text: 'AI key validation successful!' });
    } catch (error) {
      setMessage({ type: 'error', text: error instanceof Error ? error.message : 'AI key validation failed' });
    }
  };

  const getProviderIcon = (provider: string) => {
    switch (provider.toLowerCase()) {
      case 'claude':
        return '🤖';
      case 'openai':
        return '🧠';
      case 'deepseek':
        return '🔍';
      default:
        return '🤖';
    }
  };

  const getProviderName = (provider: string) => {
    switch (provider.toLowerCase()) {
      case 'claude':
        return 'Anthropic Claude';
      case 'openai':
        return 'OpenAI';
      case 'deepseek':
        return 'DeepSeek';
      default:
        return provider;
    }
  };

  return (
    <div className="max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">AI API Keys</h1>
          <p className="text-gray-400">Manage your AI provider API keys for Ginie autopilot</p>
        </div>
        <button
          onClick={() => setShowAddModal(true)}
          className="flex items-center gap-2 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors"
        >
          <Plus className="w-4 h-4" />
          Add AI Key
        </button>
      </div>

      {message && (
        <div
          className={`mb-4 p-4 rounded-lg flex items-center gap-2 ${
            message.type === 'success'
              ? 'bg-green-900/20 border border-green-500/50 text-green-400'
              : 'bg-red-900/20 border border-red-500/50 text-red-400'
          }`}
        >
          {message.type === 'success' ? (
            <CheckCircle className="w-5 h-5" />
          ) : (
            <AlertTriangle className="w-5 h-5" />
          )}
          <span>{message.text}</span>
        </div>
      )}

      {/* Info Box */}
      <div className="mb-6 p-4 bg-blue-900/20 border border-blue-500/50 rounded-lg">
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

      {/* API Keys List */}
      <div className="bg-dark-800 rounded-lg border border-gray-700 overflow-hidden">
        {isLoading ? (
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
                      onClick={() => handleTestConnection(key.id)}
                      className="p-2 text-blue-400 hover:bg-blue-900/20 rounded-lg transition-colors"
                      title="Test connection"
                    >
                      <Shield className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDeleteKey(key.id)}
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

      {/* Add Key Modal */}
      {showAddModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center p-4 z-50">
          <div className="bg-dark-800 rounded-lg border border-gray-700 max-w-md w-full">
            <div className="p-6">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-xl font-bold text-white">Add AI API Key</h2>
                <button
                  onClick={() => {
                    setShowAddModal(false);
                    setNewKey({ provider: 'claude', apiKey: '' });
                    setMessage(null);
                  }}
                  className="text-gray-400 hover:text-white"
                >
                  ×
                </button>
              </div>

              <form onSubmit={handleAddKey} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-gray-300 mb-2">Provider</label>
                  <select
                    value={newKey.provider}
                    onChange={(e) => setNewKey({ ...newKey, provider: e.target.value })}
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
                      type={showAPIKey ? 'text' : 'password'}
                      value={newKey.apiKey}
                      onChange={(e) => setNewKey({ ...newKey, apiKey: e.target.value })}
                      className="w-full px-4 py-2 bg-dark-700 border border-gray-600 rounded-lg text-white focus:outline-none focus:ring-2 focus:ring-primary-500"
                      placeholder="sk-ant-... or sk-..."
                      required
                    />
                    <button
                      type="button"
                      onClick={() => setShowAPIKey(!showAPIKey)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white"
                    >
                      {showAPIKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
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
                      setShowAddModal(false);
                      setNewKey({ provider: 'claude', apiKey: '' });
                      setMessage(null);
                    }}
                    className="flex-1 px-4 py-2 bg-dark-700 text-white rounded-lg hover:bg-dark-600 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSubmitting}
                    className="flex-1 px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isSubmitting ? 'Adding...' : 'Add Key'}
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

export default AIKeys;
