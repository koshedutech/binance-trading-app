import React, { useState, useCallback } from 'react';
import {
  Layers,
  Play,
  Save,
  Settings,
  BarChart2,
  BookOpen,
  Zap,
  ChevronRight,
  ArrowLeft,
  Plus,
} from 'lucide-react';
import { AdvancedConditionBuilder, NestedConditionGroup, AdvancedCondition } from '../components/AdvancedConditionBuilder';
import { StrategyTemplatesLibrary, StrategyTemplate } from '../components/StrategyTemplatesLibrary';
import { StrategyPerformanceDashboard } from '../components/StrategyPerformanceDashboard';
import { apiService } from '../services/api';

type TabType = 'builder' | 'templates' | 'performance';

const EnhancedStrategyBuilder: React.FC = () => {
  const [activeTab, setActiveTab] = useState<TabType>('builder');
  const [showTemplates, setShowTemplates] = useState(false);
  const [saving, setSaving] = useState(false);
  const [backtesting, setBacktesting] = useState(false);

  // Strategy state
  const [strategyName, setStrategyName] = useState('My Strategy');
  const [symbol, setSymbol] = useState('BTCUSDT');
  const [timeframe, setTimeframe] = useState('1h');
  const [positionSize, setPositionSize] = useState(5);
  const [stopLossPercent, setStopLossPercent] = useState(2);
  const [takeProfitPercent, setTakeProfitPercent] = useState(4);
  const [autopilot, setAutopilot] = useState(false);

  // Entry conditions
  const [entryConditions, setEntryConditions] = useState<NestedConditionGroup>({
    id: 'entry-root',
    operator: 'AND',
    conditions: [],
  });

  // Exit conditions
  const [exitConditions, setExitConditions] = useState<NestedConditionGroup>({
    id: 'exit-root',
    operator: 'OR',
    conditions: [],
  });

  // Available symbols
  const [symbols] = useState([
    'BTCUSDT', 'ETHUSDT', 'BNBUSDT', 'SOLUSDT', 'XRPUSDT',
    'ADAUSDT', 'DOGEUSDT', 'AVAXUSDT', 'DOTUSDT', 'LINKUSDT',
  ]);

  const timeframes = [
    { value: '1m', label: '1 Min' },
    { value: '5m', label: '5 Min' },
    { value: '15m', label: '15 Min' },
    { value: '30m', label: '30 Min' },
    { value: '1h', label: '1 Hour' },
    { value: '4h', label: '4 Hour' },
    { value: '1d', label: '1 Day' },
  ];

  // Auto-select content on focus for easier value replacement
  const handleInputFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    e.target.select();
  };

  // Handle template selection
  const handleSelectTemplate = useCallback((template: StrategyTemplate) => {
    setStrategyName(template.name);
    setTimeframe(template.timeframes[0] || '1h');
    setPositionSize(template.riskSettings.positionSizePercent);
    setStopLossPercent(template.riskSettings.stopLossPercent);
    setTakeProfitPercent(template.riskSettings.takeProfitPercent);
    setEntryConditions(template.entryConditions);
    setExitConditions(template.exitConditions);
    setShowTemplates(false);
    setActiveTab('builder');
  }, []);

  // Save strategy
  const handleSave = async () => {
    setSaving(true);
    try {
      await apiService.createStrategyConfig({
        name: strategyName,
        symbol,
        timeframe,
        indicator_type: 'custom',
        autopilot,
        enabled: true,
        position_size: positionSize,
        stop_loss_percent: stopLossPercent,
        take_profit_percent: takeProfitPercent,
        config_params: {
          entry_conditions: entryConditions,
          exit_conditions: exitConditions,
        },
      });
      alert('Strategy saved successfully!');
    } catch (error) {
      console.error('Failed to save strategy:', error);
      alert('Failed to save strategy');
    } finally {
      setSaving(false);
    }
  };

  // Run backtest
  const handleBacktest = async () => {
    setBacktesting(true);
    try {
      // First save, then get ID and run backtest
      const config = await apiService.createStrategyConfig({
        name: `${strategyName}_backtest_${Date.now()}`,
        symbol,
        timeframe,
        indicator_type: 'custom',
        autopilot: false,
        enabled: false,
        position_size: positionSize,
        stop_loss_percent: stopLossPercent,
        take_profit_percent: takeProfitPercent,
        config_params: {
          entry_conditions: entryConditions,
          exit_conditions: exitConditions,
        },
      });

      const endDate = new Date();
      const startDate = new Date();
      startDate.setMonth(startDate.getMonth() - 3); // 3 months backtest

      const result = await apiService.runBacktest(config.id, {
        symbol,
        interval: timeframe,
        start_date: startDate.toISOString().split('T')[0],
        end_date: endDate.toISOString().split('T')[0],
      });

      alert(`Backtest complete!\nTrades: ${result?.total_trades || 0}\nWin Rate: ${result?.win_rate?.toFixed(1) || 0}%\nTotal P&L: $${result?.total_pnl?.toFixed(2) || 0}`);
    } catch (error) {
      console.error('Failed to run backtest:', error);
      alert('Failed to run backtest');
    } finally {
      setBacktesting(false);
    }
  };

  // Add quick condition presets
  const addQuickCondition = (type: string) => {
    let newCondition: AdvancedCondition;

    switch (type) {
      case 'rsi_oversold':
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '<',
          rightOperand: { type: 'value', value: 30 },
        };
        break;
      case 'rsi_overbought':
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'simple',
          leftOperand: { type: 'indicator', indicator: 'RSI', params: { period: 14 } },
          operator: '>',
          rightOperand: { type: 'value', value: 70 },
        };
        break;
      case 'ema_cross_up':
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'EMA', params: { period: 9 } },
          operator: 'crosses_above',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 21 } },
        };
        break;
      case 'ema_cross_down':
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'EMA', params: { period: 9 } },
          operator: 'crosses_below',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 21 } },
        };
        break;
      case 'price_above_ema':
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 50 } },
        };
        break;
      case 'macd_bullish':
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'crossover',
          leftOperand: { type: 'indicator', indicator: 'MACD_LINE', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 } },
          operator: 'crosses_above',
          rightOperand: { type: 'indicator', indicator: 'MACD_SIGNAL', params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 } },
        };
        break;
      default:
        newCondition = {
          id: `cond-${Date.now()}`,
          type: 'simple',
          leftOperand: { type: 'price' },
          operator: '>',
          rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 20 } },
        };
    }

    setEntryConditions({
      ...entryConditions,
      conditions: [...entryConditions.conditions, newCondition],
    });
  };

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      {/* Header */}
      <div className="bg-gray-800 border-b border-gray-700 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-xl font-semibold flex items-center gap-2">
              <Layers className="w-6 h-6 text-blue-400" />
              Enhanced Strategy Builder
            </h1>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowTemplates(true)}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 rounded-lg text-sm flex items-center gap-2"
            >
              <BookOpen className="w-4 h-4" />
              Templates
            </button>
            <button
              onClick={handleBacktest}
              disabled={backtesting || entryConditions.conditions.length === 0}
              className="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 rounded-lg text-sm flex items-center gap-2 disabled:opacity-50"
            >
              <Play className="w-4 h-4" />
              {backtesting ? 'Running...' : 'Backtest'}
            </button>
            <button
              onClick={handleSave}
              disabled={saving || entryConditions.conditions.length === 0}
              className="px-4 py-2 bg-green-600 hover:bg-green-700 rounded-lg text-sm flex items-center gap-2 disabled:opacity-50"
            >
              <Save className="w-4 h-4" />
              {saving ? 'Saving...' : 'Save Strategy'}
            </button>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-4 mt-4">
          {[
            { id: 'builder' as const, label: 'Strategy Builder', icon: Layers },
            { id: 'templates' as const, label: 'Templates Library', icon: BookOpen },
            { id: 'performance' as const, label: 'Performance', icon: BarChart2 },
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`px-4 py-2 rounded-lg text-sm flex items-center gap-2 transition-colors ${
                activeTab === tab.id
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
              }`}
            >
              <tab.icon className="w-4 h-4" />
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Content */}
      <div className="p-6">
        {activeTab === 'builder' && (
          <div className="grid grid-cols-12 gap-6">
            {/* Left Panel - Settings */}
            <div className="col-span-3 space-y-4">
              <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
                <h3 className="font-medium text-white mb-4 flex items-center gap-2">
                  <Settings className="w-4 h-4 text-gray-400" />
                  Strategy Settings
                </h3>

                <div className="space-y-4">
                  <div>
                    <label className="text-sm text-gray-400 block mb-1">Strategy Name</label>
                    <input
                      type="text"
                      value={strategyName}
                      onChange={(e) => setStrategyName(e.target.value)}
                      className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                    />
                  </div>

                  <div>
                    <label className="text-sm text-gray-400 block mb-1">Symbol</label>
                    <select
                      value={symbol}
                      onChange={(e) => setSymbol(e.target.value)}
                      className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                    >
                      {symbols.map((s) => (
                        <option key={s} value={s}>{s}</option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <label className="text-sm text-gray-400 block mb-1">Timeframe</label>
                    <select
                      value={timeframe}
                      onChange={(e) => setTimeframe(e.target.value)}
                      className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                    >
                      {timeframes.map((tf) => (
                        <option key={tf.value} value={tf.value}>{tf.label}</option>
                      ))}
                    </select>
                  </div>

                  <div className="border-t border-gray-700 pt-4">
                    <h4 className="text-sm font-medium text-white mb-3">Risk Management</h4>

                    <div className="space-y-3">
                      <div>
                        <label className="text-xs text-gray-400 block mb-1">Position Size (%)</label>
                        <input
                          type="number"
                          value={positionSize}
                          onChange={(e) => setPositionSize(parseFloat(e.target.value))}
                          onFocus={handleInputFocus}
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                          min="0"
                          max="100"
                          step="0.1"
                        />
                      </div>

                      <div>
                        <label className="text-xs text-gray-400 block mb-1">Stop Loss (%)</label>
                        <input
                          type="number"
                          value={stopLossPercent}
                          onChange={(e) => setStopLossPercent(parseFloat(e.target.value))}
                          onFocus={handleInputFocus}
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                          min="0"
                          max="50"
                          step="0.1"
                        />
                      </div>

                      <div>
                        <label className="text-xs text-gray-400 block mb-1">Take Profit (%)</label>
                        <input
                          type="number"
                          value={takeProfitPercent}
                          onChange={(e) => setTakeProfitPercent(parseFloat(e.target.value))}
                          onFocus={handleInputFocus}
                          className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg text-white"
                          min="0"
                          max="100"
                          step="0.1"
                        />
                      </div>
                    </div>
                  </div>

                  <div className="border-t border-gray-700 pt-4">
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={autopilot}
                        onChange={(e) => setAutopilot(e.target.checked)}
                        className="w-4 h-4 rounded bg-gray-700 border-gray-600"
                      />
                      <span className="text-sm text-white">Enable Autopilot</span>
                    </label>
                    <p className="text-xs text-gray-500 mt-1">
                      Automatically execute trades when conditions are met
                    </p>
                  </div>
                </div>
              </div>

              {/* Quick Conditions */}
              <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
                <h3 className="font-medium text-white mb-3 flex items-center gap-2">
                  <Zap className="w-4 h-4 text-yellow-400" />
                  Quick Add
                </h3>
                <div className="space-y-2">
                  {[
                    { id: 'rsi_oversold', label: 'RSI < 30 (Oversold)' },
                    { id: 'rsi_overbought', label: 'RSI > 70 (Overbought)' },
                    { id: 'ema_cross_up', label: 'EMA 9/21 Cross Up' },
                    { id: 'ema_cross_down', label: 'EMA 9/21 Cross Down' },
                    { id: 'price_above_ema', label: 'Price > EMA 50' },
                    { id: 'macd_bullish', label: 'MACD Bullish Cross' },
                  ].map((preset) => (
                    <button
                      key={preset.id}
                      onClick={() => addQuickCondition(preset.id)}
                      className="w-full px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded text-sm text-left flex items-center gap-2"
                    >
                      <Plus className="w-3 h-3 text-green-400" />
                      {preset.label}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Right Panel - Conditions */}
            <div className="col-span-9 space-y-6">
              {/* Entry Conditions */}
              <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
                <h3 className="font-medium text-white mb-4 flex items-center gap-2">
                  <ChevronRight className="w-5 h-5 text-green-400" />
                  Entry Conditions
                  <span className="text-xs text-gray-500 ml-2">
                    ({entryConditions.conditions.length} conditions)
                  </span>
                </h3>
                <AdvancedConditionBuilder
                  conditionGroup={entryConditions}
                  onChange={setEntryConditions}
                />
              </div>

              {/* Exit Conditions */}
              <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
                <h3 className="font-medium text-white mb-4 flex items-center gap-2">
                  <ArrowLeft className="w-5 h-5 text-red-400" />
                  Exit Conditions
                  <span className="text-xs text-gray-500 ml-2">
                    ({exitConditions.conditions.length} conditions)
                  </span>
                </h3>
                <AdvancedConditionBuilder
                  conditionGroup={exitConditions}
                  onChange={setExitConditions}
                />
                <p className="text-xs text-gray-500 mt-2">
                  Note: Stop Loss and Take Profit from settings will also trigger exits.
                </p>
              </div>

              {/* Strategy Summary */}
              {(entryConditions.conditions.length > 0 || exitConditions.conditions.length > 0) && (
                <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
                  <h3 className="font-medium text-white mb-3">Strategy Summary</h3>
                  <div className="grid grid-cols-3 gap-4 text-sm">
                    <div>
                      <span className="text-gray-400">Symbol:</span>
                      <span className="text-white ml-2">{symbol}</span>
                    </div>
                    <div>
                      <span className="text-gray-400">Timeframe:</span>
                      <span className="text-white ml-2">{timeframe}</span>
                    </div>
                    <div>
                      <span className="text-gray-400">Risk/Reward:</span>
                      <span className="text-white ml-2">{(takeProfitPercent / stopLossPercent).toFixed(2)}:1</span>
                    </div>
                    <div>
                      <span className="text-gray-400">Entry Conditions:</span>
                      <span className="text-green-400 ml-2">{entryConditions.conditions.length}</span>
                    </div>
                    <div>
                      <span className="text-gray-400">Exit Conditions:</span>
                      <span className="text-red-400 ml-2">{exitConditions.conditions.length}</span>
                    </div>
                    <div>
                      <span className="text-gray-400">Mode:</span>
                      <span className={`ml-2 ${autopilot ? 'text-green-400' : 'text-yellow-400'}`}>
                        {autopilot ? 'Autopilot' : 'Manual'}
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}

        {activeTab === 'templates' && (
          <StrategyTemplatesLibrary
            onSelectTemplate={handleSelectTemplate}
          />
        )}

        {activeTab === 'performance' && (
          <StrategyPerformanceDashboard />
        )}
      </div>

      {/* Template Modal */}
      {showTemplates && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-8">
          <div className="max-w-4xl w-full max-h-[80vh] overflow-y-auto">
            <StrategyTemplatesLibrary
              onSelectTemplate={handleSelectTemplate}
              onClose={() => setShowTemplates(false)}
            />
          </div>
        </div>
      )}
    </div>
  );
};

export default EnhancedStrategyBuilder;
