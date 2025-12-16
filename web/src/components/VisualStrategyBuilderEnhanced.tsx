import React, { useCallback, useState } from 'react';
import {
  ReactFlow,
  MiniMap,
  Controls,
  Background,
  useNodesState,
  useEdgesState,
  addEdge,
  Connection,
  Edge,
  Node,
  BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { IndicatorNodeEnhanced } from './flow-nodes/IndicatorNodeEnhanced';
import { EntryNode } from './flow-nodes/EntryNode';
import { ExitNode } from './flow-nodes/ExitNode';
import { ConditionBuilder, ConditionGroup } from './ConditionBuilder';
import type { VisualFlowDefinition } from '../types';
import { Plus, X } from 'lucide-react';

const nodeTypes = {
  indicator: IndicatorNodeEnhanced,
  entry: EntryNode,
  exit: ExitNode,
};

interface VisualStrategyBuilderEnhancedProps {
  initialFlow?: VisualFlowDefinition;
  onChange?: (flow: { nodes: Node[]; edges: Edge[] }) => void;
}

export const VisualStrategyBuilderEnhanced: React.FC<VisualStrategyBuilderEnhancedProps> = ({
  initialFlow,
  onChange,
}) => {
  const [nodes, setNodes, onNodesChange] = useNodesState(initialFlow?.nodes || []);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialFlow?.edges || []);
  const [showAddMenu, setShowAddMenu] = useState(false);
  const [selectedNode, setSelectedNode] = useState<Node | null>(null);

  const onConnect = useCallback(
    (params: Connection | Edge) => {
      const newEdges = addEdge(params, edges);
      setEdges(newEdges);
      if (onChange) {
        onChange({ nodes, edges: newEdges });
      }
    },
    [edges, nodes, onChange, setEdges]
  );

  const addNode = useCallback(
    (type: string, data: any) => {
      const newNode: Node = {
        id: `${type}-${Date.now()}`,
        type,
        position: {
          x: Math.random() * 400 + 100,
          y: Math.random() * 300 + 100,
        },
        data: { ...data, editable: true },
      };

      const newNodes = [...nodes, newNode];
      setNodes(newNodes);
      if (onChange) {
        onChange({ nodes: newNodes, edges });
      }
      setShowAddMenu(false);
    },
    [nodes, edges, onChange, setNodes]
  );

  const handleNodesChange = useCallback(
    (changes: any) => {
      onNodesChange(changes);
      if (onChange) {
        setTimeout(() => {
          onChange({ nodes, edges });
        }, 0);
      }
    },
    [onNodesChange, onChange, nodes, edges]
  );

  const handleEdgesChange = useCallback(
    (changes: any) => {
      onEdgesChange(changes);
      if (onChange) {
        setTimeout(() => {
          onChange({ nodes, edges });
        }, 0);
      }
    },
    [onEdgesChange, onChange, nodes, edges]
  );

  const handleNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
    setSelectedNode(node);
  }, []);

  const updateNodeConditions = useCallback(
    (conditionGroup: ConditionGroup) => {
      if (!selectedNode) return;

      const updatedNodes = nodes.map((node) =>
        node.id === selectedNode.id
          ? { ...node, data: { ...node.data, conditionGroup } }
          : node
      );

      setNodes(updatedNodes);
      setSelectedNode({ ...selectedNode, data: { ...selectedNode.data, conditionGroup } });
      if (onChange) {
        onChange({ nodes: updatedNodes, edges });
      }
    },
    [selectedNode, nodes, edges, onChange, setNodes]
  );

  // Indicator templates
  const indicatorTemplates = [
    { value: 'RSI', label: 'RSI', params: { period: 14 } },
    { value: 'SMA', label: 'SMA', params: { period: 20 } },
    { value: 'EMA', label: 'EMA', params: { period: 20 } },
    {
      value: 'MACD',
      label: 'MACD',
      params: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9, type: 'histogram' },
    },
    { value: 'BollingerBands', label: 'Bollinger Bands', params: { period: 20, stdDev: 2, band: 'lower' } },
    { value: 'Stochastic', label: 'Stochastic', params: { kPeriod: 14, dPeriod: 3, type: 'k' } },
    { value: 'ATR', label: 'ATR', params: { period: 14 } },
    { value: 'ADX', label: 'ADX', params: { period: 14 } },
    { value: 'Volume', label: 'Volume', params: { period: 20 } },
  ];

  return (
    <div className="h-[600px] w-full border border-gray-700 rounded-lg overflow-hidden bg-gray-900 flex flex-col">
      {/* Enhanced Toolbar */}
      <div className="bg-gray-800 border-b border-gray-700 p-3 flex gap-2 items-center flex-shrink-0">
        <div className="relative">
          <button
            onClick={() => setShowAddMenu(!showAddMenu)}
            className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium transition-colors flex items-center gap-1"
          >
            <Plus className="w-4 h-4" />
            Add Node
          </button>

          {showAddMenu && (
            <div className="absolute top-full left-0 mt-1 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-50 min-w-[200px]">
              <div className="p-2">
                <div className="text-xs font-semibold text-gray-400 mb-2">Entry/Exit</div>
                <button
                  onClick={() =>
                    addNode('entry', {
                      label: 'Entry Point',
                      action: 'BUY',
                      conditionGroup: {
                        operator: 'AND',
                        conditions: [],
                      },
                    })
                  }
                  className="w-full text-left px-3 py-2 hover:bg-gray-700 rounded text-sm text-white"
                >
                  + Entry Point
                </button>
                <button
                  onClick={() =>
                    addNode('exit', {
                      label: 'Stop Loss',
                      exitType: 'stop_loss',
                      params: { percentage: 2 },
                    })
                  }
                  className="w-full text-left px-3 py-2 hover:bg-gray-700 rounded text-sm text-white"
                >
                  + Stop Loss
                </button>
                <button
                  onClick={() =>
                    addNode('exit', {
                      label: 'Take Profit',
                      exitType: 'take_profit',
                      params: { percentage: 3 },
                    })
                  }
                  className="w-full text-left px-3 py-2 hover:bg-gray-700 rounded text-sm text-white"
                >
                  + Take Profit
                </button>
              </div>

              <div className="border-t border-gray-700 p-2">
                <div className="text-xs font-semibold text-gray-400 mb-2">Indicators</div>
                {indicatorTemplates.map((template) => (
                  <button
                    key={template.value}
                    onClick={() =>
                      addNode('indicator', {
                        label: template.label,
                        indicator: template.value,
                        params: template.params,
                      })
                    }
                    className="w-full text-left px-3 py-2 hover:bg-gray-700 rounded text-sm text-white"
                  >
                    + {template.label}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="flex-1"></div>
        <div className="text-xs text-gray-400 flex items-center">
          {nodes.length} nodes, {edges.length} connections
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex flex-1 overflow-hidden">
        {/* React Flow Canvas */}
        <div className={`${selectedNode?.type === 'entry' ? 'w-2/3' : 'w-full'} transition-all`}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={handleNodesChange}
            onEdgesChange={handleEdgesChange}
            onConnect={onConnect}
            onNodeClick={handleNodeClick}
            nodeTypes={nodeTypes}
            fitView
            className="bg-gray-900"
          >
            <Controls className="bg-gray-800 border border-gray-700" />
            <MiniMap
              className="bg-gray-800 border border-gray-700"
              nodeColor={(node) => {
                switch (node.type) {
                  case 'entry':
                    return '#10b981';
                  case 'exit':
                    return '#ef4444';
                  case 'indicator':
                    return '#3b82f6';
                  default:
                    return '#6b7280';
                }
              }}
            />
            <Background variant={BackgroundVariant.Dots} gap={16} size={1} color="#374151" />
          </ReactFlow>
        </div>

        {/* Configuration Panel */}
        {selectedNode?.type === 'entry' && (
          <div className="w-1/3 bg-gray-800 border-l border-gray-700 overflow-y-auto">
            <div className="p-4">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-lg font-semibold text-white">Entry Conditions</h3>
                <button
                  onClick={() => setSelectedNode(null)}
                  className="p-1 hover:bg-gray-700 rounded transition-colors"
                >
                  <X className="w-4 h-4 text-gray-400" />
                </button>
              </div>
              <div className="text-sm text-gray-400 mb-4">
                <div className="flex items-center justify-between mb-2">
                  <span>Node:</span>
                  <span className="text-white font-medium">{String(selectedNode.data.label || 'Entry Point')}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span>Action:</span>
                  <span className="text-green-400 font-medium">{String(selectedNode.data.action || 'BUY')}</span>
                </div>
              </div>
              <ConditionBuilder
                conditionGroup={selectedNode.data.conditionGroup as ConditionGroup}
                onChange={updateNodeConditions}
              />
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
