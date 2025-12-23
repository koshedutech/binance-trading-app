import { useState, useEffect } from 'react';
import { Settings } from 'lucide-react';
import {
  futuresApi,
  ClassificationSummary,
  TradingStyleConfig,
  HedgingStatus,
  VolatilityClass,
  MarketCapClass,
  MomentumClass,
} from '../services/futuresApi';

type Tab = 'classification' | 'style' | 'hedging';

export default function AdvancedTradingPanel() {
  const [activeTab, setActiveTab] = useState<Tab>('style');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Classification state
  const [summary, setSummary] = useState<ClassificationSummary | null>(null);

  // Trading style state
  const [currentStyle, setCurrentStyle] = useState<'scalping' | 'swing' | 'position'>('swing');
  const [styleConfig, setStyleConfig] = useState<TradingStyleConfig | null>(null);

  // Hedging state
  const [hedgingStatus, setHedgingStatus] = useState<HedgingStatus | null>(null);

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const [styleData, hedgingData, summaryData] = await Promise.all([
        futuresApi.getTradingStyle(),
        futuresApi.getHedgingStatus(),
        futuresApi.getCoinClassificationSummary().catch(() => null),
      ]);

      setCurrentStyle(styleData.style);
      setStyleConfig(styleData.config);
      setHedgingStatus(hedgingData);
      if (summaryData) setSummary(summaryData);
    } catch (err) {
      setError('Failed to load data');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleStyleChange = async (style: 'scalping' | 'swing' | 'position') => {
    try {
      const result = await futuresApi.setTradingStyle(style);
      if (result.success) {
        setCurrentStyle(style);
        setStyleConfig(result.config);
      }
    } catch (err) {
      setError('Failed to change trading style');
    }
  };

  const handleHedgingToggle = async () => {
    if (!hedgingStatus) return;
    try {
      await futuresApi.updateHedgingConfig({ enabled: !hedgingStatus.enabled });
      fetchData();
    } catch (err) {
      setError('Failed to toggle hedging');
    }
  };

  const tabs: { id: Tab; label: string }[] = [
    { id: 'style', label: 'Trading Style' },
    { id: 'classification', label: 'Coin Classification' },
    { id: 'hedging', label: 'Hedging' },
  ];

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700 p-3 h-full">
      {/* Header */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Settings className="w-4 h-4 text-blue-400" />
          <span className="text-sm font-semibold text-white">Advanced</span>
        </div>
        <button
          onClick={fetchData}
          disabled={loading}
          className="text-[10px] text-blue-400 hover:text-blue-300 px-1.5 py-0.5 bg-blue-900/30 rounded"
        >
          {loading ? '...' : 'Refresh'}
        </button>
      </div>

      {error && (
        <div className="mb-2 p-1.5 bg-red-900/50 border border-red-500 rounded text-red-300 text-xs">
          {error}
        </div>
      )}

      {/* Tab Navigation */}
      <div className="flex gap-1 mb-3">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`px-2 py-1 text-xs font-medium rounded transition-colors ${
              activeTab === tab.id
                ? 'bg-blue-900/50 text-blue-400'
                : 'text-gray-400 hover:text-gray-300 hover:bg-gray-700'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="min-h-[250px]">
        {activeTab === 'style' && (
          <TradingStyleTab
            currentStyle={currentStyle}
            config={styleConfig}
            onStyleChange={handleStyleChange}
          />
        )}

        {activeTab === 'classification' && (
          <CoinClassificationTab summary={summary} />
        )}

        {activeTab === 'hedging' && (
          <HedgingTab
            status={hedgingStatus}
            onToggle={handleHedgingToggle}
            onRefresh={fetchData}
          />
        )}
      </div>
    </div>
  );
}

// Trading Style Sub-Component
function TradingStyleTab({
  currentStyle,
  config,
  onStyleChange,
}: {
  currentStyle: 'scalping' | 'swing' | 'position';
  config: TradingStyleConfig | null;
  onStyleChange: (style: 'scalping' | 'swing' | 'position') => void;
}) {
  const styles: { id: 'scalping' | 'swing' | 'position'; label: string; description: string }[] = [
    { id: 'scalping', label: 'Scalping', description: 'Quick trades, high leverage, tight SL/TP' },
    { id: 'swing', label: 'Swing', description: 'Medium-term, position averaging allowed' },
    { id: 'position', label: 'Position', description: 'Long-term holds, hedging enabled, low leverage' },
  ];

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-3 gap-3">
        {styles.map((style) => (
          <button
            key={style.id}
            onClick={() => onStyleChange(style.id)}
            className={`p-3 rounded-lg border-2 transition-all ${
              currentStyle === style.id
                ? 'border-blue-500 bg-blue-500/20'
                : 'border-gray-600 bg-gray-700/50 hover:border-gray-500'
            }`}
          >
            <div className="text-sm font-medium text-white">{style.label}</div>
            <div className="text-xs text-gray-400 mt-1">{style.description}</div>
          </button>
        ))}
      </div>

      {config && (
        <div className="mt-4 p-4 bg-gray-700/50 rounded-lg">
          <h4 className="text-sm font-medium text-white mb-3">Current Configuration</h4>
          <div className="grid grid-cols-2 md:grid-cols-3 gap-3 text-xs">
            <ConfigItem label="Leverage" value={`${config.default_leverage}x (max ${config.max_leverage}x)`} />
            <ConfigItem label="SL ATR Multiple" value={`${config.sl_atr_multiple}x`} />
            <ConfigItem label="TP ATR Multiple" value={`${config.tp_atr_multiple}x`} />
            <ConfigItem label="Min Confidence" value={`${(config.min_confidence * 100).toFixed(0)}%`} />
            <ConfigItem label="Confluence Required" value={`${config.required_confluence} signals`} />
            <ConfigItem label="Averaging" value={config.allow_averaging ? `Yes (${config.max_avg_entries})` : 'No'} />
            <ConfigItem label="Hedging" value={config.allow_hedging ? 'Enabled' : 'Disabled'} />
            <ConfigItem label="Signal TF" value={config.signal_timeframe} />
            <ConfigItem label="Trend TFs" value={config.trend_timeframes.join(', ')} />
          </div>
        </div>
      )}
    </div>
  );
}

function ConfigItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col">
      <span className="text-gray-400">{label}</span>
      <span className="text-white font-medium">{value}</span>
    </div>
  );
}

// Coin Classification Sub-Component
function CoinClassificationTab({ summary }: { summary: ClassificationSummary | null }) {
  // Check if summary has actual data (not just a message)
  if (!summary || !summary.by_volatility) {
    return (
      <div className="text-center text-gray-400 py-8">
        Loading classifications...
      </div>
    );
  }

  const volatilityLabels: Record<VolatilityClass, string> = {
    stable: 'Stable (<3% ATR)',
    medium: 'Medium (3-6% ATR)',
    high: 'High (>6% ATR)',
  };

  const marketCapLabels: Record<MarketCapClass, string> = {
    blue_chip: 'Blue Chip',
    large_cap: 'Large Cap',
    mid_small: 'Mid/Small',
  };

  const momentumLabels: Record<MomentumClass, string> = {
    gainer: 'Gainers (>5%)',
    neutral: 'Neutral',
    loser: 'Losers (<-5%)',
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <span className="text-sm text-gray-400">Total Symbols: </span>
          <span className="text-white font-medium">{summary.total_symbols || 0}</span>
        </div>
        <div>
          <span className="text-sm text-gray-400">Enabled: </span>
          <span className="text-green-400 font-medium">{summary.enabled_symbols || 0}</span>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
        {/* Volatility */}
        <div className="p-3 bg-gray-700/50 rounded-lg">
          <h4 className="text-xs font-medium text-gray-300 mb-2">By Volatility</h4>
          <div className="space-y-1">
            {(Object.keys(volatilityLabels) as VolatilityClass[]).map((key) => (
              <div key={key} className="flex justify-between text-xs">
                <span className="text-gray-400">{volatilityLabels[key]}</span>
                <span className="text-white">{summary.by_volatility[key]?.length || 0}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Market Cap */}
        <div className="p-3 bg-gray-700/50 rounded-lg">
          <h4 className="text-xs font-medium text-gray-300 mb-2">By Market Cap</h4>
          <div className="space-y-1">
            {(Object.keys(marketCapLabels) as MarketCapClass[]).map((key) => (
              <div key={key} className="flex justify-between text-xs">
                <span className="text-gray-400">{marketCapLabels[key]}</span>
                <span className="text-white">{summary.by_market_cap[key]?.length || 0}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Momentum */}
        <div className="p-3 bg-gray-700/50 rounded-lg">
          <h4 className="text-xs font-medium text-gray-300 mb-2">By Momentum</h4>
          <div className="space-y-1">
            {(Object.keys(momentumLabels) as MomentumClass[]).map((key) => (
              <div key={key} className="flex justify-between text-xs">
                <span className="text-gray-400">{momentumLabels[key]}</span>
                <span className="text-white">{summary.by_momentum[key]?.length || 0}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Top Movers */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="p-3 bg-gray-700/50 rounded-lg">
          <h4 className="text-xs font-medium text-green-400 mb-2">Top Gainers (24h)</h4>
          <div className="space-y-1">
            {(summary.top_gainers || []).slice(0, 3).map((coin) => (
              <div key={coin.symbol} className="flex justify-between text-xs">
                <span className="text-white">{coin.symbol.replace('USDT', '')}</span>
                <span className="text-green-400">+{coin.momentum_24h_pct.toFixed(2)}%</span>
              </div>
            ))}
            {(!summary.top_gainers || summary.top_gainers.length === 0) && (
              <span className="text-gray-500 text-xs">No data</span>
            )}
          </div>
        </div>
        <div className="p-3 bg-gray-700/50 rounded-lg">
          <h4 className="text-xs font-medium text-red-400 mb-2">Top Losers (24h)</h4>
          <div className="space-y-1">
            {(summary.top_losers || []).slice(0, 3).map((coin) => (
              <div key={coin.symbol} className="flex justify-between text-xs">
                <span className="text-white">{coin.symbol.replace('USDT', '')}</span>
                <span className="text-red-400">{coin.momentum_24h_pct.toFixed(2)}%</span>
              </div>
            ))}
            {(!summary.top_losers || summary.top_losers.length === 0) && (
              <span className="text-gray-500 text-xs">No data</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// Hedging Sub-Component
function HedgingTab({
  status,
  onToggle,
  onRefresh,
}: {
  status: HedgingStatus | null;
  onToggle: () => void;
  onRefresh: () => void;
}) {
  const [manualSymbol, setManualSymbol] = useState('');
  const [manualPercent, setManualPercent] = useState(50);

  if (!status) {
    return <div className="text-center text-gray-400 py-8">Loading hedging status...</div>;
  }

  const handleManualHedge = async () => {
    if (!manualSymbol) return;
    try {
      await futuresApi.executeManualHedge(manualSymbol, manualPercent);
      onRefresh();
      setManualSymbol('');
    } catch (err) {
      console.error('Failed to execute manual hedge:', err);
    }
  };

  const handleCloseHedge = async (symbol: string) => {
    try {
      await futuresApi.closeHedge(symbol);
      onRefresh();
    } catch (err) {
      console.error('Failed to close hedge:', err);
    }
  };

  return (
    <div className="space-y-4">
      {/* Status and Toggle */}
      <div className="flex items-center justify-between p-3 bg-gray-700/50 rounded-lg">
        <div className="flex items-center gap-3">
          <span className="text-sm text-gray-300">Hedging</span>
          <span
            className={`px-2 py-0.5 rounded text-xs ${
              status.enabled ? 'bg-green-500/20 text-green-400' : 'bg-gray-600 text-gray-400'
            }`}
          >
            {status.enabled ? 'Enabled' : 'Disabled'}
          </span>
        </div>
        <button
          onClick={onToggle}
          className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
            status.enabled
              ? 'bg-red-500/20 text-red-400 hover:bg-red-500/30'
              : 'bg-green-500/20 text-green-400 hover:bg-green-500/30'
          }`}
        >
          {status.enabled ? 'Disable' : 'Enable'}
        </button>
      </div>

      {/* Hedge Mode Status */}
      <div className="flex items-center gap-2 text-xs">
        <span className="text-gray-400">Position Mode:</span>
        <span className={status.hedge_mode_enabled ? 'text-green-400' : 'text-yellow-400'}>
          {status.hedge_mode_enabled ? 'HEDGE Mode Active' : 'One-Way Mode'}
        </span>
      </div>

      {/* Configuration Summary */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs">
        <div className="p-2 bg-gray-700/30 rounded">
          <div className="text-gray-400">Price Drop Trigger</div>
          <div className="text-white font-medium">{status.price_drop_trigger}%</div>
        </div>
        <div className="p-2 bg-gray-700/30 rounded">
          <div className="text-gray-400">Default Hedge</div>
          <div className="text-white font-medium">{status.default_percent}%</div>
        </div>
        <div className="p-2 bg-gray-700/30 rounded">
          <div className="text-gray-400">Profit Take</div>
          <div className="text-white font-medium">{status.profit_take_pct}%</div>
        </div>
        <div className="p-2 bg-gray-700/30 rounded">
          <div className="text-gray-400">Active / Max</div>
          <div className="text-white font-medium">
            {status.active_count} / {status.max_simultaneous}
          </div>
        </div>
      </div>

      {/* Active Hedges */}
      {status.active_hedges.length > 0 && (
        <div className="p-3 bg-gray-700/50 rounded-lg">
          <h4 className="text-xs font-medium text-gray-300 mb-2">Active Hedges</h4>
          <div className="space-y-2">
            {status.active_hedges.map((hedge) => (
              <div
                key={hedge.symbol}
                className="flex items-center justify-between p-2 bg-gray-800/50 rounded"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-white font-medium">{hedge.symbol}</span>
                    <span
                      className={`text-xs px-1.5 py-0.5 rounded ${
                        hedge.side === 'LONG' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                      }`}
                    >
                      {hedge.side}
                    </span>
                  </div>
                  <div className="text-xs text-gray-400 mt-1">
                    Entry: ${hedge.entry_price.toFixed(2)} | Qty: {hedge.quantity}
                  </div>
                </div>
                <div className="text-right">
                  <div
                    className={`text-sm font-medium ${
                      hedge.current_pnl >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}
                  >
                    {hedge.current_pnl >= 0 ? '+' : ''}${hedge.current_pnl.toFixed(2)}
                  </div>
                  <button
                    onClick={() => handleCloseHedge(hedge.symbol)}
                    className="text-xs text-red-400 hover:text-red-300 mt-1"
                  >
                    Close
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Manual Hedge */}
      <div className="p-3 bg-gray-700/50 rounded-lg">
        <h4 className="text-xs font-medium text-gray-300 mb-2">Manual Hedge</h4>
        <div className="flex items-center gap-2">
          <input
            type="text"
            value={manualSymbol}
            onChange={(e) => setManualSymbol(e.target.value.toUpperCase())}
            placeholder="BTCUSDT"
            className="flex-1 px-2 py-1 bg-gray-800 border border-gray-600 rounded text-white text-sm"
          />
          <input
            type="number"
            value={manualPercent}
            onChange={(e) => setManualPercent(Number(e.target.value))}
            min={10}
            max={100}
            step={10}
            className="w-20 px-2 py-1 bg-gray-800 border border-gray-600 rounded text-white text-sm"
          />
          <span className="text-gray-400 text-sm">%</span>
          <button
            onClick={handleManualHedge}
            disabled={!manualSymbol}
            className="px-3 py-1 bg-blue-600 hover:bg-blue-500 disabled:bg-gray-600 disabled:cursor-not-allowed rounded text-white text-sm"
          >
            Hedge
          </button>
        </div>
      </div>
    </div>
  );
}
