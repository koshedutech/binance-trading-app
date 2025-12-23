import { useEffect, useState, useMemo } from 'react';
import {
  futuresApi,
  formatPercent,
  CoinPreferenceInfo,
  CoinClassificationSettings,
  CategoryAllocation,
} from '../services/futuresApi';
import {
  Coins,
  RefreshCw,
  CheckCircle,
  XCircle,
  TrendingUp,
  TrendingDown,
  Minus,
  Filter,
  Search,
  Save,
  ToggleLeft,
  ToggleRight,
  DollarSign,
  Activity,
  ChevronDown,
  ChevronUp,
  AlertTriangle,
} from 'lucide-react';

type SortField = 'symbol' | 'enabled' | 'priority' | 'volatility' | 'market_cap' | 'momentum' | 'atr_percent' | 'change_24h';
type SortDir = 'asc' | 'desc';
type FilterCategory = 'all' | 'enabled' | 'disabled' | 'stable' | 'medium' | 'high' | 'blue_chip' | 'large_cap' | 'mid_small' | 'gainer' | 'neutral' | 'loser';

interface CategoryAllocationEdit {
  category: string;
  enabled: boolean;
  allocation_percent: number;
  max_positions: number;
}

export default function CoinPreferencesPanel() {
  const [coins, setCoins] = useState<CoinPreferenceInfo[]>([]);
  const [settings, setSettings] = useState<CoinClassificationSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  // Filters and sorting
  const [searchTerm, setSearchTerm] = useState('');
  const [filterCategory, setFilterCategory] = useState<FilterCategory>('all');
  const [sortField, setSortField] = useState<SortField>('symbol');
  const [sortDir, setSortDir] = useState<SortDir>('asc');

  // Pending changes (for bulk save)
  const [pendingChanges, setPendingChanges] = useState<Map<string, { enabled: boolean; priority: number }>>(new Map());

  // Category allocation editing
  const [editingAllocation, setEditingAllocation] = useState<CategoryAllocationEdit | null>(null);

  // Expanded sections
  const [showAllocations, setShowAllocations] = useState(false);

  const fetchData = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await futuresApi.getCoinPreferences();
      setCoins(response.coins || []);
      setSettings(response.settings);
    } catch (err) {
      console.error('Failed to fetch coin preferences:', err);
      setError('Failed to load coin preferences');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleRefresh = async () => {
    try {
      setRefreshing(true);
      await futuresApi.refreshCoinClassifications();
      setSuccessMsg('Classification refresh triggered');
      setTimeout(() => setSuccessMsg(null), 3000);
      // Wait a bit then reload
      setTimeout(() => fetchData(), 2000);
    } catch (err) {
      setError('Failed to refresh classifications');
    } finally {
      setRefreshing(false);
    }
  };

  const handleToggleCoin = (symbol: string, currentEnabled: boolean, currentPriority: number) => {
    const newEnabled = !currentEnabled;
    const existing = pendingChanges.get(symbol);
    setPendingChanges(prev => {
      const updated = new Map(prev);
      updated.set(symbol, { enabled: newEnabled, priority: existing?.priority ?? currentPriority });
      return updated;
    });
  };

  const handlePriorityChange = (symbol: string, priority: number, currentEnabled: boolean) => {
    const existing = pendingChanges.get(symbol);
    setPendingChanges(prev => {
      const updated = new Map(prev);
      updated.set(symbol, { enabled: existing?.enabled ?? currentEnabled, priority });
      return updated;
    });
  };

  const handleSaveChanges = async () => {
    if (pendingChanges.size === 0) return;

    try {
      setSaving(true);
      const updates = Array.from(pendingChanges.entries()).map(([symbol, { enabled, priority }]) => ({
        symbol,
        enabled,
        priority,
      }));
      await futuresApi.bulkUpdateCoinPreferences(updates);
      setSuccessMsg(`Updated ${updates.length} coin preferences`);
      setPendingChanges(new Map());
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchData();
    } catch (err) {
      setError('Failed to save changes');
    } finally {
      setSaving(false);
    }
  };

  const handleEnableAll = async () => {
    try {
      setSaving(true);
      const result = await futuresApi.enableAllCoins();
      setSuccessMsg(`Enabled ${result.enabled} coins`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchData();
    } catch (err) {
      setError('Failed to enable all coins');
    } finally {
      setSaving(false);
    }
  };

  const handleDisableAll = async () => {
    try {
      setSaving(true);
      const result = await futuresApi.disableAllCoins();
      setSuccessMsg(`Disabled ${result.disabled} coins`);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchData();
    } catch (err) {
      setError('Failed to disable all coins');
    } finally {
      setSaving(false);
    }
  };

  const handleSaveAllocation = async () => {
    if (!editingAllocation) return;
    try {
      setSaving(true);
      await futuresApi.updateCategoryAllocation(
        editingAllocation.category,
        editingAllocation.enabled,
        editingAllocation.allocation_percent,
        editingAllocation.max_positions
      );
      setSuccessMsg('Category allocation updated');
      setEditingAllocation(null);
      setTimeout(() => setSuccessMsg(null), 3000);
      await fetchData();
    } catch (err) {
      setError('Failed to update category allocation');
    } finally {
      setSaving(false);
    }
  };

  // Get effective values (pending change or current)
  const getEffectiveValue = (coin: CoinPreferenceInfo) => {
    const pending = pendingChanges.get(coin.symbol);
    return {
      enabled: pending?.enabled ?? coin.enabled,
      priority: pending?.priority ?? coin.priority,
    };
  };

  // Filter and sort coins
  const filteredCoins = useMemo(() => {
    let result = [...coins];

    // Search filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      result = result.filter(c => c.symbol.toLowerCase().includes(term));
    }

    // Category filter
    if (filterCategory !== 'all') {
      result = result.filter(c => {
        const eff = getEffectiveValue(c);
        switch (filterCategory) {
          case 'enabled':
            return eff.enabled;
          case 'disabled':
            return !eff.enabled;
          case 'stable':
          case 'medium':
          case 'high':
            return c.volatility === filterCategory;
          case 'blue_chip':
          case 'large_cap':
          case 'mid_small':
            return c.market_cap === filterCategory;
          case 'gainer':
          case 'neutral':
          case 'loser':
            return c.momentum === filterCategory;
          default:
            return true;
        }
      });
    }

    // Sort
    result.sort((a, b) => {
      const effA = getEffectiveValue(a);
      const effB = getEffectiveValue(b);

      let cmp = 0;
      switch (sortField) {
        case 'symbol':
          cmp = a.symbol.localeCompare(b.symbol);
          break;
        case 'enabled':
          cmp = (effA.enabled ? 1 : 0) - (effB.enabled ? 1 : 0);
          break;
        case 'priority':
          cmp = effA.priority - effB.priority;
          break;
        case 'volatility':
          cmp = (a.volatility || '').localeCompare(b.volatility || '');
          break;
        case 'market_cap':
          cmp = (a.market_cap || '').localeCompare(b.market_cap || '');
          break;
        case 'momentum':
          cmp = (a.momentum || '').localeCompare(b.momentum || '');
          break;
        case 'atr_percent':
          cmp = (a.atr_percent || 0) - (b.atr_percent || 0);
          break;
        case 'change_24h':
          cmp = (a.change_24h || 0) - (b.change_24h || 0);
          break;
      }
      return sortDir === 'asc' ? cmp : -cmp;
    });

    return result;
  }, [coins, searchTerm, filterCategory, sortField, sortDir, pendingChanges]);

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir(prev => prev === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDir('asc');
    }
  };

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) return null;
    return sortDir === 'asc' ? <ChevronUp className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />;
  };

  const getVolatilityBadge = (volatility?: string) => {
    switch (volatility) {
      case 'stable':
        return <span className="px-2 py-0.5 text-xs rounded bg-green-900/50 text-green-400">Stable</span>;
      case 'medium':
        return <span className="px-2 py-0.5 text-xs rounded bg-yellow-900/50 text-yellow-400">Medium</span>;
      case 'high':
        return <span className="px-2 py-0.5 text-xs rounded bg-red-900/50 text-red-400">High</span>;
      default:
        return <span className="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-400">-</span>;
    }
  };

  const getMarketCapBadge = (marketCap?: string) => {
    switch (marketCap) {
      case 'blue_chip':
        return <span className="px-2 py-0.5 text-xs rounded bg-blue-900/50 text-blue-400">Blue Chip</span>;
      case 'large_cap':
        return <span className="px-2 py-0.5 text-xs rounded bg-purple-900/50 text-purple-400">Large Cap</span>;
      case 'mid_small':
        return <span className="px-2 py-0.5 text-xs rounded bg-orange-900/50 text-orange-400">Mid/Small</span>;
      default:
        return <span className="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-400">-</span>;
    }
  };

  const getMomentumBadge = (momentum?: string) => {
    switch (momentum) {
      case 'gainer':
        return <span className="px-2 py-0.5 text-xs rounded bg-green-900/50 text-green-400 flex items-center gap-1"><TrendingUp className="w-3 h-3" />Gainer</span>;
      case 'loser':
        return <span className="px-2 py-0.5 text-xs rounded bg-red-900/50 text-red-400 flex items-center gap-1"><TrendingDown className="w-3 h-3" />Loser</span>;
      case 'neutral':
        return <span className="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-400 flex items-center gap-1"><Minus className="w-3 h-3" />Neutral</span>;
      default:
        return <span className="px-2 py-0.5 text-xs rounded bg-gray-700 text-gray-400">-</span>;
    }
  };

  // Stats
  const stats = useMemo(() => {
    const enabled = coins.filter(c => getEffectiveValue(c).enabled).length;
    const byVolatility = {
      stable: coins.filter(c => c.volatility === 'stable').length,
      medium: coins.filter(c => c.volatility === 'medium').length,
      high: coins.filter(c => c.volatility === 'high').length,
    };
    const byMomentum = {
      gainer: coins.filter(c => c.momentum === 'gainer').length,
      neutral: coins.filter(c => c.momentum === 'neutral').length,
      loser: coins.filter(c => c.momentum === 'loser').length,
    };
    return { total: coins.length, enabled, byVolatility, byMomentum };
  }, [coins, pendingChanges]);

  const renderAllocationEditor = (
    title: string,
    category: string,
    allocation: CategoryAllocation | undefined
  ) => {
    const isEditing = editingAllocation?.category === category;
    const current = isEditing ? editingAllocation : allocation ? {
      category,
      enabled: allocation.enabled,
      allocation_percent: allocation.allocation_percent,
      max_positions: allocation.max_positions,
    } : null;

    if (!current) return null;

    return (
      <div className="p-3 bg-gray-700/30 rounded-lg">
        <div className="flex items-center justify-between mb-2">
          <span className="text-sm font-medium text-gray-300">{title}</span>
          <button
            onClick={() => current.enabled ? null : setEditingAllocation({
              ...current,
              enabled: !current.enabled,
            })}
            className={`flex items-center gap-1 text-xs px-2 py-1 rounded ${
              current.enabled ? 'bg-green-600 text-white' : 'bg-gray-600 text-gray-300'
            }`}
          >
            {current.enabled ? <ToggleRight className="w-3 h-3" /> : <ToggleLeft className="w-3 h-3" />}
            {current.enabled ? 'Enabled' : 'Disabled'}
          </button>
        </div>
        {isEditing ? (
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <label className="text-xs text-gray-400 w-24">Allocation %:</label>
              <input
                type="number"
                min="0"
                max="100"
                step="5"
                value={current.allocation_percent}
                onChange={(e) => setEditingAllocation({
                  ...current,
                  allocation_percent: parseFloat(e.target.value) || 0,
                })}
                className="flex-1 bg-gray-600 text-white text-sm rounded px-2 py-1"
              />
            </div>
            <div className="flex items-center gap-2">
              <label className="text-xs text-gray-400 w-24">Max Positions:</label>
              <input
                type="number"
                min="0"
                max="20"
                value={current.max_positions}
                onChange={(e) => setEditingAllocation({
                  ...current,
                  max_positions: parseInt(e.target.value) || 0,
                })}
                className="flex-1 bg-gray-600 text-white text-sm rounded px-2 py-1"
              />
            </div>
            <div className="flex gap-2 mt-2">
              <button
                onClick={handleSaveAllocation}
                disabled={saving}
                className="flex-1 bg-blue-600 hover:bg-blue-500 text-white text-xs py-1 rounded flex items-center justify-center gap-1"
              >
                <Save className="w-3 h-3" /> Save
              </button>
              <button
                onClick={() => setEditingAllocation(null)}
                className="flex-1 bg-gray-600 hover:bg-gray-500 text-white text-xs py-1 rounded"
              >
                Cancel
              </button>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-4 text-xs text-gray-400">
            <span>Allocation: {current.allocation_percent}%</span>
            <span>Max: {current.max_positions} positions</span>
            <button
              onClick={() => setEditingAllocation(current)}
              className="text-blue-400 hover:text-blue-300"
            >
              Edit
            </button>
          </div>
        )}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="bg-gray-800 rounded-lg p-4">
        <div className="flex items-center gap-2 mb-4">
          <Coins className="w-5 h-5 text-blue-400" />
          <h2 className="text-lg font-semibold text-white">Coin Preferences</h2>
        </div>
        <div className="flex items-center justify-center py-8">
          <RefreshCw className="w-6 h-6 text-blue-400 animate-spin" />
          <span className="ml-2 text-gray-400">Loading...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <Coins className="w-5 h-5 text-blue-400" />
          <h2 className="text-lg font-semibold text-white">Coin Preferences</h2>
          <span className="text-sm text-gray-400">({stats.enabled}/{stats.total} enabled)</span>
        </div>
        <div className="flex items-center gap-2">
          {pendingChanges.size > 0 && (
            <button
              onClick={handleSaveChanges}
              disabled={saving}
              className="flex items-center gap-1 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded"
            >
              <Save className="w-4 h-4" />
              Save ({pendingChanges.size})
            </button>
          )}
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center gap-1 px-3 py-1.5 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {/* Success/Error Messages */}
      {successMsg && (
        <div className="mb-4 p-2 bg-green-900/30 border border-green-700 rounded text-green-400 text-sm flex items-center gap-2">
          <CheckCircle className="w-4 h-4" />
          {successMsg}
        </div>
      )}
      {error && (
        <div className="mb-4 p-2 bg-red-900/30 border border-red-700 rounded text-red-400 text-sm flex items-center gap-2">
          <AlertTriangle className="w-4 h-4" />
          {error}
          <button onClick={() => setError(null)} className="ml-auto text-red-300 hover:text-white">×</button>
        </div>
      )}

      {/* Stats Row */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
        <div className="bg-gray-700/50 p-3 rounded-lg">
          <div className="flex items-center gap-2 text-gray-400 text-xs mb-1">
            <Activity className="w-3 h-3" />
            Volatility
          </div>
          <div className="flex gap-2 text-xs">
            <span className="text-green-400">{stats.byVolatility.stable} Stable</span>
            <span className="text-yellow-400">{stats.byVolatility.medium} Med</span>
            <span className="text-red-400">{stats.byVolatility.high} High</span>
          </div>
        </div>
        <div className="bg-gray-700/50 p-3 rounded-lg">
          <div className="flex items-center gap-2 text-gray-400 text-xs mb-1">
            <TrendingUp className="w-3 h-3" />
            Momentum
          </div>
          <div className="flex gap-2 text-xs">
            <span className="text-green-400">{stats.byMomentum.gainer} Up</span>
            <span className="text-gray-400">{stats.byMomentum.neutral} Flat</span>
            <span className="text-red-400">{stats.byMomentum.loser} Down</span>
          </div>
        </div>
        <div className="bg-gray-700/50 p-3 rounded-lg flex items-center justify-center">
          <button
            onClick={handleEnableAll}
            disabled={saving}
            className="flex items-center gap-1 text-xs text-green-400 hover:text-green-300"
          >
            <ToggleRight className="w-4 h-4" />
            Enable All
          </button>
        </div>
        <div className="bg-gray-700/50 p-3 rounded-lg flex items-center justify-center">
          <button
            onClick={handleDisableAll}
            disabled={saving}
            className="flex items-center gap-1 text-xs text-red-400 hover:text-red-300"
          >
            <ToggleLeft className="w-4 h-4" />
            Disable All
          </button>
        </div>
      </div>

      {/* Category Allocations (collapsible) */}
      <div className="mb-4">
        <button
          onClick={() => setShowAllocations(!showAllocations)}
          className="w-full flex items-center justify-between p-2 bg-gray-700/30 rounded-lg hover:bg-gray-700/50"
        >
          <span className="text-sm font-medium text-gray-300 flex items-center gap-2">
            <DollarSign className="w-4 h-4" />
            Category Allocations
          </span>
          {showAllocations ? <ChevronUp className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
        </button>
        {showAllocations && settings && (
          <div className="mt-2 space-y-3">
            <div className="text-xs text-gray-400 px-2">Volatility Categories</div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
              {renderAllocationEditor('Stable', 'volatility:stable', settings.volatility_allocations?.stable)}
              {renderAllocationEditor('Medium', 'volatility:medium', settings.volatility_allocations?.medium)}
              {renderAllocationEditor('High', 'volatility:high', settings.volatility_allocations?.high)}
            </div>
            <div className="text-xs text-gray-400 px-2 mt-3">Market Cap Categories</div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
              {renderAllocationEditor('Blue Chip', 'market_cap:blue_chip', settings.market_cap_allocations?.blue_chip)}
              {renderAllocationEditor('Large Cap', 'market_cap:large_cap', settings.market_cap_allocations?.large_cap)}
              {renderAllocationEditor('Mid/Small', 'market_cap:mid_small', settings.market_cap_allocations?.mid_small)}
            </div>
            <div className="text-xs text-gray-400 px-2 mt-3">Momentum Categories</div>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-2">
              {renderAllocationEditor('Gainers', 'momentum:gainer', settings.momentum_allocations?.gainer)}
              {renderAllocationEditor('Neutral', 'momentum:neutral', settings.momentum_allocations?.neutral)}
              {renderAllocationEditor('Losers', 'momentum:loser', settings.momentum_allocations?.loser)}
            </div>
          </div>
        )}
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-2 mb-4">
        <div className="relative flex-1 min-w-[200px]">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search symbol..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 bg-gray-700 text-white text-sm rounded border border-gray-600 focus:border-blue-500 focus:outline-none"
          />
        </div>
        <div className="flex items-center gap-1">
          <Filter className="w-4 h-4 text-gray-400" />
          <select
            value={filterCategory}
            onChange={(e) => setFilterCategory(e.target.value as FilterCategory)}
            className="bg-gray-700 text-white text-sm rounded px-2 py-1.5 border border-gray-600"
          >
            <option value="all">All Coins</option>
            <option value="enabled">Enabled Only</option>
            <option value="disabled">Disabled Only</option>
            <optgroup label="Volatility">
              <option value="stable">Stable</option>
              <option value="medium">Medium</option>
              <option value="high">High</option>
            </optgroup>
            <optgroup label="Market Cap">
              <option value="blue_chip">Blue Chip</option>
              <option value="large_cap">Large Cap</option>
              <option value="mid_small">Mid/Small</option>
            </optgroup>
            <optgroup label="Momentum">
              <option value="gainer">Gainers</option>
              <option value="neutral">Neutral</option>
              <option value="loser">Losers</option>
            </optgroup>
          </select>
        </div>
      </div>

      {/* Coins Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-gray-400 border-b border-gray-700">
              <th
                className="py-2 px-2 text-left cursor-pointer hover:text-white"
                onClick={() => handleSort('enabled')}
              >
                <span className="flex items-center gap-1">
                  Status <SortIcon field="enabled" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-left cursor-pointer hover:text-white"
                onClick={() => handleSort('symbol')}
              >
                <span className="flex items-center gap-1">
                  Symbol <SortIcon field="symbol" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-left cursor-pointer hover:text-white"
                onClick={() => handleSort('volatility')}
              >
                <span className="flex items-center gap-1">
                  Volatility <SortIcon field="volatility" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-left cursor-pointer hover:text-white"
                onClick={() => handleSort('market_cap')}
              >
                <span className="flex items-center gap-1">
                  Market Cap <SortIcon field="market_cap" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-left cursor-pointer hover:text-white"
                onClick={() => handleSort('momentum')}
              >
                <span className="flex items-center gap-1">
                  Momentum <SortIcon field="momentum" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-right cursor-pointer hover:text-white"
                onClick={() => handleSort('atr_percent')}
              >
                <span className="flex items-center justify-end gap-1">
                  ATR% <SortIcon field="atr_percent" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-right cursor-pointer hover:text-white"
                onClick={() => handleSort('change_24h')}
              >
                <span className="flex items-center justify-end gap-1">
                  24h Change <SortIcon field="change_24h" />
                </span>
              </th>
              <th
                className="py-2 px-2 text-center cursor-pointer hover:text-white"
                onClick={() => handleSort('priority')}
              >
                <span className="flex items-center justify-center gap-1">
                  Priority <SortIcon field="priority" />
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            {filteredCoins.length === 0 ? (
              <tr>
                <td colSpan={8} className="py-8 text-center text-gray-500">
                  No coins found
                </td>
              </tr>
            ) : (
              filteredCoins.map((coin) => {
                const eff = getEffectiveValue(coin);
                const hasPending = pendingChanges.has(coin.symbol);
                return (
                  <tr
                    key={coin.symbol}
                    className={`border-b border-gray-700/50 hover:bg-gray-700/30 ${hasPending ? 'bg-blue-900/20' : ''}`}
                  >
                    <td className="py-2 px-2">
                      <button
                        onClick={() => handleToggleCoin(coin.symbol, eff.enabled, coin.priority)}
                        className={`p-1 rounded ${
                          eff.enabled
                            ? 'bg-green-600/20 text-green-400 hover:bg-green-600/30'
                            : 'bg-red-600/20 text-red-400 hover:bg-red-600/30'
                        }`}
                      >
                        {eff.enabled ? <CheckCircle className="w-4 h-4" /> : <XCircle className="w-4 h-4" />}
                      </button>
                    </td>
                    <td className="py-2 px-2 font-medium text-white">
                      {coin.symbol.replace('USDT', '')}
                      <span className="text-gray-500 text-xs">/USDT</span>
                    </td>
                    <td className="py-2 px-2">{getVolatilityBadge(coin.volatility)}</td>
                    <td className="py-2 px-2">{getMarketCapBadge(coin.market_cap)}</td>
                    <td className="py-2 px-2">{getMomentumBadge(coin.momentum)}</td>
                    <td className="py-2 px-2 text-right text-gray-300">
                      {coin.atr_percent ? `${coin.atr_percent.toFixed(2)}%` : '-'}
                    </td>
                    <td className={`py-2 px-2 text-right ${
                      (coin.change_24h || 0) > 0 ? 'text-green-400' : (coin.change_24h || 0) < 0 ? 'text-red-400' : 'text-gray-400'
                    }`}>
                      {coin.change_24h ? formatPercent(coin.change_24h) : '-'}
                    </td>
                    <td className="py-2 px-2">
                      <input
                        type="number"
                        min="-10"
                        max="10"
                        value={eff.priority}
                        onChange={(e) => handlePriorityChange(coin.symbol, parseInt(e.target.value) || 0, eff.enabled)}
                        className="w-14 text-center bg-gray-700 text-white text-sm rounded px-1 py-0.5 border border-gray-600"
                      />
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Footer */}
      <div className="mt-4 text-xs text-gray-500 flex items-center justify-between">
        <span>Showing {filteredCoins.length} of {coins.length} coins</span>
        <span>Higher priority = more likely to be selected for trading</span>
      </div>
    </div>
  );
}
