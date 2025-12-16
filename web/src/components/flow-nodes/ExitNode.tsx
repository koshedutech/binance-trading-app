import { Handle, Position, NodeProps, Node } from '@xyflow/react';
import { LogOut } from 'lucide-react';

export interface ExitNodeData extends Record<string, unknown> {
  label: string;
  exitType: string; // stop_loss, take_profit, trailing_stop, condition
  params: Record<string, any>;
}

export type ExitNodeType = Node<ExitNodeData, 'exit'>;

export const ExitNode = ({ data, selected }: NodeProps<ExitNodeType>) => {
  const getExitTypeColor = (type: string) => {
    switch (type) {
      case 'stop_loss':
        return 'text-red-400';
      case 'take_profit':
        return 'text-green-400';
      case 'trailing_stop':
        return 'text-yellow-400';
      default:
        return 'text-purple-400';
    }
  };

  const getExitTypeLabel = (type: string) => {
    switch (type) {
      case 'stop_loss':
        return 'Stop Loss';
      case 'take_profit':
        return 'Take Profit';
      case 'trailing_stop':
        return 'Trailing Stop';
      case 'condition':
        return 'Condition Exit';
      default:
        return type;
    }
  };

  return (
    <div
      className={`px-4 py-3 rounded-lg border-2 bg-gray-800 min-w-[180px] ${
        selected ? 'border-red-500 shadow-lg shadow-red-500/50' : 'border-red-600/50'
      }`}
    >
      <Handle type="target" position={Position.Top} className="w-3 h-3 bg-red-500" />

      <div className="flex items-center gap-2 mb-2">
        <LogOut className="w-4 h-4 text-red-500" />
        <div className="text-sm font-semibold text-white">{data.label}</div>
      </div>

      <div className="text-xs space-y-1">
        <div className={`font-medium ${getExitTypeColor(data.exitType)}`}>
          {getExitTypeLabel(data.exitType)}
        </div>

        {Object.entries(data.params || {}).map(([key, value]) => (
          <div key={key} className="text-gray-500">
            {key}: <span className="text-gray-300">{String(value)}</span>
          </div>
        ))}
      </div>

      <Handle type="source" position={Position.Bottom} className="w-3 h-3 bg-red-500" />
    </div>
  );
};
