import { Handle, Position, NodeProps, Node } from '@xyflow/react';

export interface IndicatorNodeData extends Record<string, unknown> {
  label: string;
  indicator: string;
  params: Record<string, any>;
}

export type IndicatorNodeType = Node<IndicatorNodeData, 'indicator'>;

export const IndicatorNode = ({ data, selected }: NodeProps<IndicatorNodeType>) => {
  return (
    <div
      className={`px-4 py-3 rounded-lg border-2 bg-gray-800 min-w-[180px] ${
        selected ? 'border-blue-500 shadow-lg shadow-blue-500/50' : 'border-blue-600/50'
      }`}
    >
      <Handle type="target" position={Position.Top} className="w-3 h-3 bg-blue-500" />

      <div className="flex items-center gap-2 mb-2">
        <div className="w-2 h-2 rounded-full bg-blue-500"></div>
        <div className="text-sm font-semibold text-white">{data.label}</div>
      </div>

      <div className="text-xs text-gray-400 space-y-1">
        <div className="font-medium text-blue-400">Indicator: {data.indicator}</div>
        {Object.entries(data.params || {}).map(([key, value]) => (
          <div key={key} className="text-gray-500">
            {key}: <span className="text-gray-300">{String(value)}</span>
          </div>
        ))}
      </div>

      <Handle type="source" position={Position.Bottom} className="w-3 h-3 bg-blue-500" />
    </div>
  );
};
