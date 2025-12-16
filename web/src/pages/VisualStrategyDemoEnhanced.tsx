import { useState } from 'react';
import { VisualStrategyBuilderEnhanced } from '../components/VisualStrategyBuilderEnhanced';
import { BacktestPanel } from '../components/BacktestPanel';
import { SymbolSearchPicker } from '../components/SymbolSearchPicker';
import { ChartViewer } from '../components/ChartViewer';
import type { Node, Edge } from '@xyflow/react';
import { Settings, TrendingUp, BarChart3, Save } from 'lucide-react';

export default function VisualStrategyDemoEnhanced() {
  const [activeTab, setActiveTab] = useState<'builder' | 'settings' | 'backtest'>('builder');
  const [strategyName, setStrategyName] = useState('My Visual Strategy');
  const [symbol, setSymbol] = useState('BTCUSDT');
  const [interval, setInterval] = useState('5m');
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);

  // Risk management settings
  const [stopLossEnabled, setStopLossEnabled] = useState(true);
  const [stopLossType, setStopLossType] = useState<'percentage' | 'absolute'>('percentage');
  const [stopLossValue, setStopLossValue] = useState(2.0);

  const [takeProfitEnabled, setTakeProfitEnabled] = useState(true);
  const [takeProfitType, setTakeProfitType] = useState<'percentage' | 'absolute'>('percentage');
  const [takeProfitValue, setTakeProfitValue] = useState(3.0);

  const handleFlowChange = (flow: { nodes: Node[]; edges: Edge[] }) => {
    setNodes(flow.nodes);
    setEdges(flow.edges);
  };

  const handleSave = async () => {
    const strategy = {
      name: strategyName,
      symbol,
      interval,
      nodes,
      edges,
      settings: {
        stopLoss: stopLossEnabled
          ? { enabled: true, type: stopLossType, value: stopLossValue }
          : { enabled: false },
        takeProfit: takeProfitEnabled
          ? { enabled: true, type: takeProfitType, value: takeProfitValue }
          : { enabled: false },
      },
    };

    console.log('Visual Strategy:', strategy);
    alert(`Strategy "${strategyName}" structure logged to console!`);
  };

  return (
    <div className="min-h-screen bg-gray-900 p-6">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <h1 className="text-3xl font-bold text-white mb-2 flex items-center gap-3">
            <TrendingUp className="w-8 h-8 text-blue-500" />
            Advanced Visual Strategy Builder
          </h1>
          <p className="text-gray-400">
            Build complex trading strategies with multiple indicators and advanced conditions
          </p>
        </div>

        {/* Strategy Header Info */}
        <div className="bg-gray-800 rounded-lg p-4 mb-6 flex items-center justify-between">
          <div className="flex items-center gap-4">
            <div>
              <div className="text-xs text-gray-400">Strategy Name</div>
              <div className="text-lg font-semibold text-white">{strategyName}</div>
            </div>
            <div className="h-8 w-px bg-gray-700"></div>
            <div>
              <div className="text-xs text-gray-400">Symbol</div>
              <div className="text-lg font-semibold text-blue-400">{symbol}</div>
            </div>
            <div className="h-8 w-px bg-gray-700"></div>
            <div>
              <div className="text-xs text-gray-400">Interval</div>
              <div className="text-lg font-semibold text-green-400">{interval}</div>
            </div>
            <div className="h-8 w-px bg-gray-700"></div>
            <div>
              <div className="text-xs text-gray-400">Nodes</div>
              <div className="text-lg font-semibold text-white">{nodes.length}</div>
            </div>
          </div>
          <button
            onClick={handleSave}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white font-medium flex items-center gap-2 transition-colors"
          >
            <Save className="w-4 h-4" />
            Save Strategy
          </button>
        </div>

        {/* Tabs */}
        <div className="bg-gray-800 rounded-lg overflow-hidden">
          <div className="flex border-b border-gray-700">
            <button
              onClick={() => setActiveTab('builder')}
              className={`flex-1 px-6 py-3 font-medium transition-colors flex items-center justify-center gap-2 ${
                activeTab === 'builder'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }`}
            >
              <TrendingUp className="w-4 h-4" />
              Strategy Builder
            </button>
            <button
              onClick={() => setActiveTab('settings')}
              className={`flex-1 px-6 py-3 font-medium transition-colors flex items-center justify-center gap-2 ${
                activeTab === 'settings'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }`}
            >
              <Settings className="w-4 h-4" />
              Settings
            </button>
            <button
              onClick={() => setActiveTab('backtest')}
              className={`flex-1 px-6 py-3 font-medium transition-colors flex items-center justify-center gap-2 ${
                activeTab === 'backtest'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }`}
            >
              <BarChart3 className="w-4 h-4" />
              Backtest & Results
            </button>
          </div>

          <div className="p-6">
            {activeTab === 'builder' && (
              <div>
                <div className="mb-4 p-4 bg-blue-900/20 border border-blue-500/50 rounded">
                  <h3 className="font-semibold text-blue-400 mb-2">Instructions:</h3>
                  <ul className="text-sm text-gray-300 space-y-1">
                    <li>• Click "Add Node" to add indicators, entry points, and exit strategies</li>
                    <li>• Drag nodes to reposition them on the canvas</li>
                    <li>• Connect nodes by dragging from one handle to another</li>
                    <li>• Double-click indicator nodes to edit their parameters</li>
                    <li>• Supports RSI, SMA, EMA, MACD, Bollinger Bands, Stochastic, ATR, ADX, Volume</li>
                  </ul>
                </div>
                <VisualStrategyBuilderEnhanced onChange={handleFlowChange} />
              </div>
            )}

            {activeTab === 'settings' && (
              <div className="space-y-6">
                <div>
                  <h3 className="text-lg font-semibold text-white mb-4">Strategy Settings</h3>
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <label className="block text-sm text-gray-400 mb-2">Strategy Name</label>
                      <input
                        type="text"
                        value={strategyName}
                        onChange={(e) => setStrategyName(e.target.value)}
                        className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
                        placeholder="My Strategy"
                      />
                    </div>
                    <div>
                      <label className="block text-sm text-gray-400 mb-2">Symbol</label>
                      <SymbolSearchPicker value={symbol} onChange={setSymbol} />
                    </div>
                    <div>
                      <label className="block text-sm text-gray-400 mb-2">Interval</label>
                      <select
                        value={interval}
                        onChange={(e) => setInterval(e.target.value)}
                        className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
                      >
                        <option value="1m">1 Minute</option>
                        <option value="5m">5 Minutes</option>
                        <option value="15m">15 Minutes</option>
                        <option value="30m">30 Minutes</option>
                        <option value="1h">1 Hour</option>
                        <option value="4h">4 Hours</option>
                        <option value="1d">1 Day</option>
                      </select>
                    </div>
                  </div>
                </div>

                <div>
                  <h3 className="text-lg font-semibold text-white mb-4">Risk Management</h3>
                  <div className="grid grid-cols-2 gap-6">
                    {/* Stop Loss */}
                    <div className="bg-gray-700/30 border border-gray-700 rounded-lg p-4">
                      <div className="flex items-center justify-between mb-3">
                        <label className="text-sm font-medium text-white">Stop Loss</label>
                        <input
                          type="checkbox"
                          checked={stopLossEnabled}
                          onChange={(e) => setStopLossEnabled(e.target.checked)}
                          className="w-4 h-4 text-blue-600 bg-gray-700 border-gray-600 rounded focus:ring-blue-500"
                        />
                      </div>
                      {stopLossEnabled && (
                        <div className="space-y-3">
                          <div>
                            <label className="block text-xs text-gray-400 mb-1">Type</label>
                            <select
                              value={stopLossType}
                              onChange={(e) => setStopLossType(e.target.value as 'percentage' | 'absolute')}
                              className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm"
                            >
                              <option value="percentage">Percentage</option>
                              <option value="absolute">Absolute</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-xs text-gray-400 mb-1">
                              Value {stopLossType === 'percentage' ? '(%)' : '($)'}
                            </label>
                            <input
                              type="number"
                              value={stopLossValue}
                              onChange={(e) => setStopLossValue(parseFloat(e.target.value))}
                              step="0.1"
                              className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm"
                            />
                          </div>
                        </div>
                      )}
                    </div>

                    {/* Take Profit */}
                    <div className="bg-gray-700/30 border border-gray-700 rounded-lg p-4">
                      <div className="flex items-center justify-between mb-3">
                        <label className="text-sm font-medium text-white">Take Profit</label>
                        <input
                          type="checkbox"
                          checked={takeProfitEnabled}
                          onChange={(e) => setTakeProfitEnabled(e.target.checked)}
                          className="w-4 h-4 text-blue-600 bg-gray-700 border-gray-600 rounded focus:ring-blue-500"
                        />
                      </div>
                      {takeProfitEnabled && (
                        <div className="space-y-3">
                          <div>
                            <label className="block text-xs text-gray-400 mb-1">Type</label>
                            <select
                              value={takeProfitType}
                              onChange={(e) => setTakeProfitType(e.target.value as 'percentage' | 'absolute')}
                              className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm"
                            >
                              <option value="percentage">Percentage</option>
                              <option value="absolute">Absolute</option>
                            </select>
                          </div>
                          <div>
                            <label className="block text-xs text-gray-400 mb-1">
                              Value {takeProfitType === 'percentage' ? '(%)' : '($)'}
                            </label>
                            <input
                              type="number"
                              value={takeProfitValue}
                              onChange={(e) => setTakeProfitValue(parseFloat(e.target.value))}
                              step="0.1"
                              className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-sm"
                            />
                          </div>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'backtest' && (
              <div className="space-y-6">
                <div className="p-4 bg-yellow-900/20 border border-yellow-500/50 rounded">
                  <h3 className="font-semibold text-yellow-400 mb-2">Backtest Note:</h3>
                  <p className="text-sm text-gray-300">
                    For this demo, use strategy_config_id = 1. The full integration will save visual flows to
                    strategy_configs and use the actual ID.
                  </p>
                </div>
                <ChartViewer defaultSymbol={symbol} defaultInterval={interval} />
                <BacktestPanel strategyConfigId={1} symbol={symbol} interval={interval} />
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="mt-6 text-center text-sm text-gray-500">
          Advanced Visual Strategy Builder - Full Feature Implementation
        </div>
      </div>
    </div>
  );
}
