import { Handle, Position, NodeProps, Node } from '@xyflow/react';
import { useState } from 'react';
import { TrendingUp } from 'lucide-react';

export interface IndicatorNodeData extends Record<string, unknown> {
  label: string;
  indicator: string;
  params: Record<string, any>;
  editable?: boolean;
}

export type IndicatorNodeType = Node<IndicatorNodeData, 'indicator'>;

const INDICATORS = [
  { value: 'RSI', label: 'RSI', params: [{ name: 'period', label: 'Period', default: 14, type: 'number' }] },
  { value: 'SMA', label: 'SMA', params: [{ name: 'period', label: 'Period', default: 20, type: 'number' }] },
  { value: 'EMA', label: 'EMA', params: [{ name: 'period', label: 'Period', default: 20, type: 'number' }] },
  {
    value: 'MACD',
    label: 'MACD',
    params: [
      { name: 'fastPeriod', label: 'Fast', default: 12, type: 'number' },
      { name: 'slowPeriod', label: 'Slow', default: 26, type: 'number' },
      { name: 'signalPeriod', label: 'Signal', default: 9, type: 'number' },
      { name: 'type', label: 'Type', default: 'histogram', type: 'select', options: ['macd', 'signal', 'histogram'] },
    ],
  },
  {
    value: 'BollingerBands',
    label: 'Bollinger Bands',
    params: [
      { name: 'period', label: 'Period', default: 20, type: 'number' },
      { name: 'stdDev', label: 'Std Dev', default: 2, type: 'number' },
      { name: 'band', label: 'Band', default: 'lower', type: 'select', options: ['upper', 'middle', 'lower'] },
    ],
  },
  {
    value: 'Stochastic',
    label: 'Stochastic',
    params: [
      { name: 'kPeriod', label: 'K Period', default: 14, type: 'number' },
      { name: 'dPeriod', label: 'D Period', default: 3, type: 'number' },
      { name: 'type', label: 'Type', default: 'k', type: 'select', options: ['k', 'd'] },
    ],
  },
  { value: 'ATR', label: 'ATR', params: [{ name: 'period', label: 'Period', default: 14, type: 'number' }] },
  { value: 'ADX', label: 'ADX', params: [{ name: 'period', label: 'Period', default: 14, type: 'number' }] },
  { value: 'Volume', label: 'Volume', params: [{ name: 'period', label: 'Period', default: 20, type: 'number' }] },
];

export const IndicatorNodeEnhanced = ({ data, selected }: NodeProps<IndicatorNodeType>) => {
  const [isEditing, setIsEditing] = useState(false);
  const [editedData, setEditedData] = useState(data);

  const selectedIndicator = INDICATORS.find((ind) => ind.value === data.indicator);

  const handleSave = () => {
    // Update node data (would need to be connected to React Flow's node state)
    setIsEditing(false);
  };

  if (isEditing && data.editable) {
    return (
      <div
        className={`px-4 py-3 rounded-lg border-2 bg-gray-800 min-w-[250px] ${
          selected ? 'border-blue-500 shadow-lg shadow-blue-500/50' : 'border-blue-600/50'
        }`}
      >
        <Handle type="target" position={Position.Top} className="w-3 h-3 bg-blue-500" />

        <div className="space-y-3">
          <div>
            <label className="text-xs text-gray-400 block mb-1">Indicator</label>
            <select
              value={editedData.indicator}
              onChange={(e) =>
                setEditedData({
                  ...editedData,
                  indicator: e.target.value,
                  params: {},
                })
              }
              className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
            >
              {INDICATORS.map((ind) => (
                <option key={ind.value} value={ind.value}>
                  {ind.label}
                </option>
              ))}
            </select>
          </div>

          {selectedIndicator?.params.map((param) => (
            <div key={param.name}>
              <label className="text-xs text-gray-400 block mb-1">{param.label}</label>
              {param.type === 'select' ? (
                <select
                  value={editedData.params[param.name] || param.default}
                  onChange={(e) =>
                    setEditedData({
                      ...editedData,
                      params: { ...editedData.params, [param.name]: e.target.value },
                    })
                  }
                  className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                >
                  {param.options?.map((opt) => (
                    <option key={opt} value={opt}>
                      {opt}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type="number"
                  value={editedData.params[param.name] || param.default}
                  onChange={(e) =>
                    setEditedData({
                      ...editedData,
                      params: { ...editedData.params, [param.name]: parseFloat(e.target.value) },
                    })
                  }
                  className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                />
              )}
            </div>
          ))}

          <div className="flex gap-2 pt-2">
            <button
              onClick={handleSave}
              className="flex-1 px-2 py-1 bg-blue-600 hover:bg-blue-700 rounded text-xs text-white"
            >
              Save
            </button>
            <button
              onClick={() => setIsEditing(false)}
              className="flex-1 px-2 py-1 bg-gray-600 hover:bg-gray-700 rounded text-xs text-white"
            >
              Cancel
            </button>
          </div>
        </div>

        <Handle type="source" position={Position.Bottom} className="w-3 h-3 bg-blue-500" />
      </div>
    );
  }

  return (
    <div
      className={`px-4 py-3 rounded-lg border-2 bg-gray-800 min-w-[180px] ${
        selected ? 'border-blue-500 shadow-lg shadow-blue-500/50' : 'border-blue-600/50'
      }`}
      onDoubleClick={() => data.editable && setIsEditing(true)}
    >
      <Handle type="target" position={Position.Top} className="w-3 h-3 bg-blue-500" />

      <div className="flex items-center gap-2 mb-2">
        <TrendingUp className="w-4 h-4 text-blue-500" />
        <div className="text-sm font-semibold text-white">{data.label}</div>
      </div>

      <div className="text-xs text-gray-400 space-y-1">
        <div className="font-medium text-blue-400">{data.indicator}</div>
        {Object.entries(data.params || {}).map(([key, value]) => (
          <div key={key} className="text-gray-500">
            {key}: <span className="text-gray-300">{String(value)}</span>
          </div>
        ))}
      </div>

      {data.editable && (
        <div className="text-[10px] text-gray-600 mt-2 italic">Double-click to edit</div>
      )}

      <Handle type="source" position={Position.Bottom} className="w-3 h-3 bg-blue-500" />
    </div>
  );
};
