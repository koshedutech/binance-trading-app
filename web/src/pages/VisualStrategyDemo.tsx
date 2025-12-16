import { useState } from 'react';
import { VisualStrategyBuilder } from '../components/VisualStrategyBuilder';
import { BacktestPanel } from '../components/BacktestPanel';
import type { Node, Edge } from '@xyflow/react';

export default function VisualStrategyDemo() {
  const [activeTab, setActiveTab] = useState<'builder' | 'backtest'>('builder');
  const [strategyName, setStrategyName] = useState('My Visual Strategy');
  const [symbol, setSymbol] = useState('BTCUSDT');
  const [interval, setInterval] = useState('5m');
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);

  const handleFlowChange = (flow: { nodes: Node[]; edges: Edge[] }) => {
    setNodes(flow.nodes);
    setEdges(flow.edges);
  };

  const handleSave = async () => {
    // For MVP demo, we'll just show the flow structure
    console.log('Visual Strategy:', {
      name: strategyName,
      symbol,
      interval,
      nodes,
      edges,
    });

    alert(`Strategy "${strategyName}" structure logged to console!`);
  };

  return (
    <div className="min-h-screen bg-gray-900 p-6">
      <div className="max-w-7xl mx-auto">
        {/* Header */}
        <div className="mb-6">
          <h1 className="text-3xl font-bold text-white mb-2">
            Visual Strategy Builder - MVP Demo
          </h1>
          <p className="text-gray-400">
            Build and backtest trading strategies visually with drag-and-drop nodes
          </p>
        </div>

        {/* Strategy Settings */}
        <div className="bg-gray-800 rounded-lg p-4 mb-6">
          <h2 className="text-lg font-semibold text-white mb-4">Strategy Settings</h2>
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
              <select
                value={symbol}
                onChange={(e) => setSymbol(e.target.value)}
                className="w-full px-3 py-2 bg-gray-700 border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
              >
                <option value="BTCUSDT">BTCUSDT</option>
                <option value="ETHUSDT">ETHUSDT</option>
                <option value="BNBUSDT">BNBUSDT</option>
              </select>
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
                <option value="1h">1 Hour</option>
              </select>
            </div>
          </div>

          <button
            onClick={handleSave}
            className="mt-4 px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white font-medium transition-colors"
          >
            Save Strategy (Console Log for MVP)
          </button>
        </div>

        {/* Tabs */}
        <div className="bg-gray-800 rounded-lg overflow-hidden">
          <div className="flex border-b border-gray-700">
            <button
              onClick={() => setActiveTab('builder')}
              className={`flex-1 px-6 py-3 font-medium transition-colors ${
                activeTab === 'builder'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }`}
            >
              Strategy Builder
            </button>
            <button
              onClick={() => setActiveTab('backtest')}
              className={`flex-1 px-6 py-3 font-medium transition-colors ${
                activeTab === 'backtest'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }`}
            >
              Backtest & Results
            </button>
          </div>

          <div className="p-6">
            {activeTab === 'builder' && (
              <div>
                <div className="mb-4 p-4 bg-blue-900/20 border border-blue-500/50 rounded">
                  <h3 className="font-semibold text-blue-400 mb-2">MVP Instructions:</h3>
                  <ul className="text-sm text-gray-300 space-y-1">
                    <li>• Click "+ Entry Point" to add an entry condition node</li>
                    <li>• Click "+ Indicator (RSI)" to add an RSI indicator node</li>
                    <li>• Drag nodes to reposition them</li>
                    <li>• Connect nodes by dragging from one handle to another</li>
                    <li>• Default entry triggers when RSI &lt; 30 (oversold)</li>
                  </ul>
                </div>
                <VisualStrategyBuilder onChange={handleFlowChange} />
              </div>
            )}

            {activeTab === 'backtest' && (
              <div>
                <div className="mb-4 p-4 bg-yellow-900/20 border border-yellow-500/50 rounded">
                  <h3 className="font-semibold text-yellow-400 mb-2">MVP Backtest Note:</h3>
                  <p className="text-sm text-gray-300">
                    For this demo, use strategy_config_id = 1 (or any existing config). The full
                    integration will save visual flows to strategy_configs and use the actual ID.
                  </p>
                </div>
                <BacktestPanel
                  strategyConfigId={1}
                  symbol={symbol}
                  interval={interval}
                />
              </div>
            )}
          </div>
        </div>

        {/* Footer Info */}
        <div className="mt-6 text-center text-sm text-gray-500">
          MVP Demo - Visual Strategy Builder with Backtesting
        </div>
      </div>
    </div>
  );
}
