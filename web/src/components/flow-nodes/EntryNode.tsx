import { Handle, Position, NodeProps, Node } from '@xyflow/react';

export interface EntryNodeData extends Record<string, unknown> {
  label: string;
  action: string;
  conditionGroup: {
    operator: 'AND' | 'OR';
    conditions: any[];
  };
}

export type EntryNodeType = Node<EntryNodeData, 'entry'>;

export const EntryNode = ({ data, selected }: NodeProps<EntryNodeType>) => {
  const conditionCount = data.conditionGroup?.conditions?.length || 0;

  return (
    <div
      className={`px-4 py-3 rounded-lg border-2 bg-gray-800 min-w-[200px] ${
        selected ? 'border-green-500 shadow-lg shadow-green-500/50' : 'border-green-600/50'
      }`}
    >
      <Handle type="target" position={Position.Top} className="w-3 h-3 bg-green-500" />

      <div className="flex items-center gap-2 mb-2">
        <div className="w-2 h-2 rounded-full bg-green-500"></div>
        <div className="text-sm font-semibold text-white">{data.label}</div>
      </div>

      <div className="text-xs space-y-1">
        <div className="flex items-center justify-between">
          <span className="text-gray-400">Action:</span>
          <span className="text-green-400 font-medium">{data.action || 'BUY'}</span>
        </div>
        <div className="flex items-center justify-between">
          <span className="text-gray-400">Conditions:</span>
          <span className="text-gray-300">
            {conditionCount} ({data.conditionGroup?.operator || 'AND'})
          </span>
        </div>
      </div>

      <Handle type="source" position={Position.Bottom} className="w-3 h-3 bg-green-500" />
    </div>
  );
};
