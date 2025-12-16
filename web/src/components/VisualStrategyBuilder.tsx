import React, { useCallback } from 'react';
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

import { IndicatorNode } from './flow-nodes/IndicatorNode';
import { EntryNode } from './flow-nodes/EntryNode';
import type { VisualFlowDefinition } from '../types';

const nodeTypes = {
  indicator: IndicatorNode,
  entry: EntryNode,
};

interface VisualStrategyBuilderProps {
  initialFlow?: VisualFlowDefinition;
  onChange?: (flow: { nodes: Node[]; edges: Edge[] }) => void;
}

export const VisualStrategyBuilder: React.FC<VisualStrategyBuilderProps> = ({
  initialFlow,
  onChange,
}) => {
  const [nodes, setNodes, onNodesChange] = useNodesState(initialFlow?.nodes || []);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialFlow?.edges || []);

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
    (type: string) => {
      const newNode: Node = {
        id: `${type}-${Date.now()}`,
        type,
        position: {
          x: Math.random() * 300 + 100,
          y: Math.random() * 200 + 100,
        },
        data: getDefaultNodeData(type),
      };

      const newNodes = [...nodes, newNode];
      setNodes(newNodes);
      if (onChange) {
        onChange({ nodes: newNodes, edges });
      }
    },
    [nodes, edges, onChange, setNodes]
  );

  const handleNodesChange = useCallback(
    (changes: any) => {
      onNodesChange(changes);
      // Notify parent of changes
      if (onChange) {
        // Get the updated nodes after the change
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
      // Notify parent of changes
      if (onChange) {
        setTimeout(() => {
          onChange({ nodes, edges });
        }, 0);
      }
    },
    [onEdgesChange, onChange, nodes, edges]
  );

  return (
    <div className="h-[600px] w-full border border-gray-700 rounded-lg overflow-hidden bg-gray-900">
      {/* Toolbar */}
      <div className="bg-gray-800 border-b border-gray-700 p-3 flex gap-2">
        <button
          onClick={() => addNode('entry')}
          className="px-3 py-1.5 bg-green-600 hover:bg-green-700 rounded text-sm text-white font-medium transition-colors"
        >
          + Entry Point
        </button>
        <button
          onClick={() => addNode('indicator')}
          className="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm text-white font-medium transition-colors"
        >
          + Indicator (RSI)
        </button>
        <div className="flex-1"></div>
        <div className="text-xs text-gray-400 flex items-center">
          {nodes.length} nodes, {edges.length} connections
        </div>
      </div>

      {/* React Flow Canvas */}
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={handleNodesChange}
        onEdgesChange={handleEdgesChange}
        onConnect={onConnect}
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
  );
};

function getDefaultNodeData(type: string): any {
  switch (type) {
    case 'indicator':
      return {
        label: 'RSI Indicator',
        indicator: 'RSI',
        params: { period: 14 },
      };
    case 'entry':
      return {
        label: 'Entry Point',
        action: 'BUY',
        conditionGroup: {
          operator: 'AND',
          conditions: [
            {
              type: 'indicator_comparison',
              indicator: 'RSI',
              params: { period: 14 },
              comparison: '<',
              value: 30,
            },
          ],
        },
      };
    default:
      return { label: 'Node' };
  }
}
