import React, { useState } from 'react';
import { Plus, Trash2, Grip, ChevronDown, ChevronRight, Copy, Layers } from 'lucide-react';

// Extended condition with multi-timeframe support
export interface AdvancedCondition {
  id: string;
  type: 'simple' | 'crossover' | 'threshold' | 'pattern';
  timeframe?: string; // For multi-timeframe analysis
  leftOperand: {
    type: 'price' | 'indicator' | 'value' | 'candle';
    indicator?: string;
    params?: Record<string, any>;
    timeframe?: string;
    candleProperty?: 'open' | 'high' | 'low' | 'close' | 'volume';
    offset?: number; // 0 = current, 1 = previous, etc.
  };
  operator: '>' | '<' | '>=' | '<=' | '==' | '!=' | 'crosses_above' | 'crosses_below' | 'between' | 'outside';
  rightOperand: {
    type: 'price' | 'indicator' | 'value' | 'candle';
    value?: number;
    value2?: number; // For 'between' operator
    indicator?: string;
    params?: Record<string, any>;
    timeframe?: string;
    candleProperty?: 'open' | 'high' | 'low' | 'close' | 'volume';
    offset?: number;
  };
}

export interface NestedConditionGroup {
  id: string;
  operator: 'AND' | 'OR';
  conditions: (AdvancedCondition | NestedConditionGroup)[];
  collapsed?: boolean;
}

interface AdvancedConditionBuilderProps {
  conditionGroup: NestedConditionGroup;
  onChange: (conditionGroup: NestedConditionGroup) => void;
  availableTimeframes?: string[];
}

const OPERATORS = [
  { value: '>', label: '> Greater Than', category: 'comparison' },
  { value: '<', label: '< Less Than', category: 'comparison' },
  { value: '>=', label: '>= Greater or Equal', category: 'comparison' },
  { value: '<=', label: '<= Less or Equal', category: 'comparison' },
  { value: '==', label: '== Equal To', category: 'comparison' },
  { value: '!=', label: '!= Not Equal', category: 'comparison' },
  { value: 'crosses_above', label: 'Crosses Above', category: 'crossover' },
  { value: 'crosses_below', label: 'Crosses Below', category: 'crossover' },
  { value: 'between', label: 'Between', category: 'range' },
  { value: 'outside', label: 'Outside Range', category: 'range' },
];

const INDICATORS = [
  { value: 'RSI', label: 'RSI', defaultParams: { period: 14 }, description: 'Relative Strength Index' },
  { value: 'SMA', label: 'SMA', defaultParams: { period: 20 }, description: 'Simple Moving Average' },
  { value: 'EMA', label: 'EMA', defaultParams: { period: 20 }, description: 'Exponential Moving Average' },
  { value: 'MACD', label: 'MACD', defaultParams: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9, output: 'histogram' }, description: 'Moving Average Convergence Divergence' },
  { value: 'MACD_SIGNAL', label: 'MACD Signal', defaultParams: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 }, description: 'MACD Signal Line' },
  { value: 'MACD_LINE', label: 'MACD Line', defaultParams: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9 }, description: 'MACD Line' },
  { value: 'BB_UPPER', label: 'Bollinger Upper', defaultParams: { period: 20, stdDev: 2 }, description: 'Bollinger Band Upper' },
  { value: 'BB_MIDDLE', label: 'Bollinger Middle', defaultParams: { period: 20, stdDev: 2 }, description: 'Bollinger Band Middle' },
  { value: 'BB_LOWER', label: 'Bollinger Lower', defaultParams: { period: 20, stdDev: 2 }, description: 'Bollinger Band Lower' },
  { value: 'STOCH_K', label: 'Stochastic %K', defaultParams: { kPeriod: 14, dPeriod: 3 }, description: 'Stochastic K Line' },
  { value: 'STOCH_D', label: 'Stochastic %D', defaultParams: { kPeriod: 14, dPeriod: 3 }, description: 'Stochastic D Line' },
  { value: 'ATR', label: 'ATR', defaultParams: { period: 14 }, description: 'Average True Range' },
  { value: 'ADX', label: 'ADX', defaultParams: { period: 14 }, description: 'Average Directional Index' },
  { value: 'VOLUME_SMA', label: 'Volume SMA', defaultParams: { period: 20 }, description: 'Volume Moving Average' },
  { value: 'VWAP', label: 'VWAP', defaultParams: {}, description: 'Volume Weighted Average Price' },
  { value: 'OBV', label: 'OBV', defaultParams: {}, description: 'On Balance Volume' },
];

const TIMEFRAMES = [
  { value: '1m', label: '1 Min' },
  { value: '5m', label: '5 Min' },
  { value: '15m', label: '15 Min' },
  { value: '30m', label: '30 Min' },
  { value: '1h', label: '1 Hour' },
  { value: '4h', label: '4 Hour' },
  { value: '1d', label: '1 Day' },
];

const CANDLE_PROPERTIES = [
  { value: 'open', label: 'Open' },
  { value: 'high', label: 'High' },
  { value: 'low', label: 'Low' },
  { value: 'close', label: 'Close' },
  { value: 'volume', label: 'Volume' },
];

// Helper to check if item is a nested group
const isNestedGroup = (item: AdvancedCondition | NestedConditionGroup): item is NestedConditionGroup => {
  return 'conditions' in item;
};

// Single condition row component
const ConditionRow: React.FC<{
  condition: AdvancedCondition;
  onUpdate: (updates: Partial<AdvancedCondition>) => void;
  onRemove: () => void;
  onDuplicate: () => void;
  index: number;
}> = ({ condition, onUpdate, onRemove, onDuplicate, index }) => {
  const [showAdvanced, setShowAdvanced] = useState(false);

  const updateLeftOperand = (updates: Partial<AdvancedCondition['leftOperand']>) => {
    onUpdate({ leftOperand: { ...condition.leftOperand, ...updates } });
  };

  const updateRightOperand = (updates: Partial<AdvancedCondition['rightOperand']>) => {
    onUpdate({ rightOperand: { ...condition.rightOperand, ...updates } });
  };

  const isCrossoverOperator = condition.operator === 'crosses_above' || condition.operator === 'crosses_below';
  const isRangeOperator = condition.operator === 'between' || condition.operator === 'outside';

  return (
    <div className="bg-gray-700/30 border border-gray-600 rounded-lg p-3">
      <div className="flex items-center gap-2 mb-2">
        <Grip className="w-4 h-4 text-gray-500 cursor-move" />
        <span className="text-xs text-gray-400">Condition {index + 1}</span>
        {condition.leftOperand.timeframe && (
          <span className="px-2 py-0.5 bg-purple-600/30 text-purple-300 rounded text-[10px]">
            {condition.leftOperand.timeframe}
          </span>
        )}
        {isCrossoverOperator && (
          <span className="px-2 py-0.5 bg-blue-600/30 text-blue-300 rounded text-[10px]">
            Crossover
          </span>
        )}
        <div className="flex-1"></div>
        <button
          onClick={() => setShowAdvanced(!showAdvanced)}
          className="p-1 hover:bg-gray-600/50 rounded text-gray-400 text-xs flex items-center gap-1"
        >
          <Layers className="w-3 h-3" />
          {showAdvanced ? 'Less' : 'More'}
        </button>
        <button
          onClick={onDuplicate}
          className="p-1 hover:bg-gray-600/50 rounded"
          title="Duplicate"
        >
          <Copy className="w-4 h-4 text-gray-400" />
        </button>
        <button
          onClick={onRemove}
          className="p-1 hover:bg-red-600/20 rounded"
        >
          <Trash2 className="w-4 h-4 text-red-400" />
        </button>
      </div>

      {/* Advanced Options */}
      {showAdvanced && (
        <div className="mb-3 p-2 bg-gray-800/50 rounded border border-gray-600">
          <div className="grid grid-cols-2 gap-2">
            <div>
              <label className="text-[10px] text-gray-400 block mb-1">Left Timeframe</label>
              <select
                value={condition.leftOperand.timeframe || ''}
                onChange={(e) => updateLeftOperand({ timeframe: e.target.value || undefined })}
                className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
              >
                <option value="">Current TF</option>
                {TIMEFRAMES.map((tf) => (
                  <option key={tf.value} value={tf.value}>{tf.label}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-[10px] text-gray-400 block mb-1">Right Timeframe</label>
              <select
                value={condition.rightOperand.timeframe || ''}
                onChange={(e) => updateRightOperand({ timeframe: e.target.value || undefined })}
                className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
              >
                <option value="">Current TF</option>
                {TIMEFRAMES.map((tf) => (
                  <option key={tf.value} value={tf.value}>{tf.label}</option>
                ))}
              </select>
            </div>
            {condition.leftOperand.type === 'candle' && (
              <div>
                <label className="text-[10px] text-gray-400 block mb-1">Left Offset</label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  value={condition.leftOperand.offset || 0}
                  onChange={(e) => updateLeftOperand({ offset: parseInt(e.target.value) })}
                  className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                />
              </div>
            )}
            {condition.rightOperand.type === 'candle' && (
              <div>
                <label className="text-[10px] text-gray-400 block mb-1">Right Offset</label>
                <input
                  type="number"
                  min="0"
                  max="100"
                  value={condition.rightOperand.offset || 0}
                  onChange={(e) => updateRightOperand({ offset: parseInt(e.target.value) })}
                  className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                />
              </div>
            )}
          </div>
        </div>
      )}

      <div className="grid grid-cols-[1fr,auto,1fr] gap-2 items-start">
        {/* Left Operand */}
        <div className="space-y-2">
          <select
            value={condition.leftOperand.type}
            onChange={(e) => {
              const type = e.target.value as any;
              updateLeftOperand({
                type,
                ...(type === 'indicator' && { indicator: 'EMA', params: { period: 20 } }),
                ...(type === 'candle' && { candleProperty: 'close', offset: 0 }),
              });
            }}
            className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
          >
            <option value="price">Current Price</option>
            <option value="indicator">Indicator</option>
            <option value="candle">Candle Data</option>
          </select>

          {condition.leftOperand.type === 'indicator' && (
            <>
              <select
                value={condition.leftOperand.indicator}
                onChange={(e) => {
                  const ind = INDICATORS.find((i) => i.value === e.target.value);
                  updateLeftOperand({
                    indicator: e.target.value,
                    params: ind?.defaultParams || {},
                  });
                }}
                className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
              >
                {INDICATORS.map((ind) => (
                  <option key={ind.value} value={ind.value}>{ind.label}</option>
                ))}
              </select>
              {/* Period input for applicable indicators */}
              {['RSI', 'SMA', 'EMA', 'ATR', 'ADX', 'VOLUME_SMA'].includes(condition.leftOperand.indicator || '') && (
                <input
                  type="number"
                  value={condition.leftOperand.params?.period || 14}
                  onChange={(e) => updateLeftOperand({ params: { ...condition.leftOperand.params, period: parseInt(e.target.value) } })}
                  className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  placeholder="Period"
                />
              )}
            </>
          )}

          {condition.leftOperand.type === 'candle' && (
            <select
              value={condition.leftOperand.candleProperty || 'close'}
              onChange={(e) => updateLeftOperand({ candleProperty: e.target.value as any })}
              className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
            >
              {CANDLE_PROPERTIES.map((prop) => (
                <option key={prop.value} value={prop.value}>{prop.label}</option>
              ))}
            </select>
          )}
        </div>

        {/* Operator */}
        <div className="space-y-1">
          <select
            value={condition.operator}
            onChange={(e) => onUpdate({ operator: e.target.value as any })}
            className="px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs font-mono min-w-[100px]"
          >
            <optgroup label="Comparison">
              {OPERATORS.filter(op => op.category === 'comparison').map((op) => (
                <option key={op.value} value={op.value}>{op.value}</option>
              ))}
            </optgroup>
            <optgroup label="Crossover">
              {OPERATORS.filter(op => op.category === 'crossover').map((op) => (
                <option key={op.value} value={op.value}>{op.label}</option>
              ))}
            </optgroup>
            <optgroup label="Range">
              {OPERATORS.filter(op => op.category === 'range').map((op) => (
                <option key={op.value} value={op.value}>{op.label}</option>
              ))}
            </optgroup>
          </select>
        </div>

        {/* Right Operand */}
        <div className="space-y-2">
          <select
            value={condition.rightOperand.type}
            onChange={(e) => {
              const type = e.target.value as any;
              updateRightOperand({
                type,
                ...(type === 'value' && { value: 0 }),
                ...(type === 'indicator' && { indicator: 'EMA', params: { period: 20 } }),
                ...(type === 'candle' && { candleProperty: 'close', offset: 0 }),
              });
            }}
            className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
          >
            <option value="value">Fixed Value</option>
            <option value="indicator">Indicator</option>
            <option value="price">Current Price</option>
            <option value="candle">Candle Data</option>
          </select>

          {condition.rightOperand.type === 'value' && (
            <div className="flex gap-1">
              <input
                type="number"
                value={condition.rightOperand.value || 0}
                onChange={(e) => updateRightOperand({ value: parseFloat(e.target.value) })}
                step="0.01"
                className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                placeholder="Value"
              />
              {isRangeOperator && (
                <input
                  type="number"
                  value={condition.rightOperand.value2 || 0}
                  onChange={(e) => updateRightOperand({ value2: parseFloat(e.target.value) })}
                  step="0.01"
                  className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  placeholder="Value 2"
                />
              )}
            </div>
          )}

          {condition.rightOperand.type === 'indicator' && (
            <>
              <select
                value={condition.rightOperand.indicator}
                onChange={(e) => {
                  const ind = INDICATORS.find((i) => i.value === e.target.value);
                  updateRightOperand({
                    indicator: e.target.value,
                    params: ind?.defaultParams || {},
                  });
                }}
                className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
              >
                {INDICATORS.map((ind) => (
                  <option key={ind.value} value={ind.value}>{ind.label}</option>
                ))}
              </select>
              {['RSI', 'SMA', 'EMA', 'ATR', 'ADX', 'VOLUME_SMA'].includes(condition.rightOperand.indicator || '') && (
                <input
                  type="number"
                  value={condition.rightOperand.params?.period || 14}
                  onChange={(e) => updateRightOperand({ params: { ...condition.rightOperand.params, period: parseInt(e.target.value) } })}
                  className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  placeholder="Period"
                />
              )}
            </>
          )}

          {condition.rightOperand.type === 'candle' && (
            <select
              value={condition.rightOperand.candleProperty || 'close'}
              onChange={(e) => updateRightOperand({ candleProperty: e.target.value as any })}
              className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
            >
              {CANDLE_PROPERTIES.map((prop) => (
                <option key={prop.value} value={prop.value}>{prop.label}</option>
              ))}
            </select>
          )}
        </div>
      </div>

      {/* Condition Summary */}
      <div className="mt-2 text-xs text-gray-400 bg-gray-800/50 rounded px-2 py-1 flex items-center gap-1 flex-wrap">
        {condition.leftOperand.timeframe && (
          <span className="text-purple-400">[{condition.leftOperand.timeframe}]</span>
        )}
        {condition.leftOperand.type === 'price' && 'Price'}
        {condition.leftOperand.type === 'candle' && `Candle.${condition.leftOperand.candleProperty}[${condition.leftOperand.offset || 0}]`}
        {condition.leftOperand.type === 'indicator' && (
          <span className="text-green-400">
            {condition.leftOperand.indicator}({condition.leftOperand.params?.period || ''})
          </span>
        )}
        <span className="text-blue-400 font-mono">{condition.operator}</span>
        {condition.rightOperand.timeframe && (
          <span className="text-purple-400">[{condition.rightOperand.timeframe}]</span>
        )}
        {condition.rightOperand.type === 'value' && (
          <>
            <span>{condition.rightOperand.value}</span>
            {isRangeOperator && <span>to {condition.rightOperand.value2}</span>}
          </>
        )}
        {condition.rightOperand.type === 'price' && 'Price'}
        {condition.rightOperand.type === 'candle' && `Candle.${condition.rightOperand.candleProperty}[${condition.rightOperand.offset || 0}]`}
        {condition.rightOperand.type === 'indicator' && (
          <span className="text-green-400">
            {condition.rightOperand.indicator}({condition.rightOperand.params?.period || ''})
          </span>
        )}
      </div>
    </div>
  );
};

// Recursive group component
const ConditionGroupComponent: React.FC<{
  group: NestedConditionGroup;
  onChange: (group: NestedConditionGroup) => void;
  onRemove?: () => void;
  depth?: number;
}> = ({ group, onChange, onRemove, depth = 0 }) => {
  const [collapsed, setCollapsed] = useState(group.collapsed || false);

  const addCondition = () => {
    const newCondition: AdvancedCondition = {
      id: `cond-${Date.now()}`,
      type: 'simple',
      leftOperand: { type: 'price' },
      operator: '>',
      rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 20 } },
    };
    onChange({ ...group, conditions: [...group.conditions, newCondition] });
  };

  const addNestedGroup = () => {
    const newGroup: NestedConditionGroup = {
      id: `group-${Date.now()}`,
      operator: 'AND',
      conditions: [],
    };
    onChange({ ...group, conditions: [...group.conditions, newGroup] });
  };

  const updateCondition = (index: number, updates: Partial<AdvancedCondition>) => {
    const newConditions = [...group.conditions];
    newConditions[index] = { ...newConditions[index], ...updates } as AdvancedCondition;
    onChange({ ...group, conditions: newConditions });
  };

  const updateNestedGroup = (index: number, updatedGroup: NestedConditionGroup) => {
    const newConditions = [...group.conditions];
    newConditions[index] = updatedGroup;
    onChange({ ...group, conditions: newConditions });
  };

  const removeItem = (index: number) => {
    onChange({ ...group, conditions: group.conditions.filter((_, i) => i !== index) });
  };

  const duplicateCondition = (index: number) => {
    const item = group.conditions[index];
    if (!isNestedGroup(item)) {
      const duplicated: AdvancedCondition = {
        ...item,
        id: `cond-${Date.now()}`,
      };
      const newConditions = [...group.conditions];
      newConditions.splice(index + 1, 0, duplicated);
      onChange({ ...group, conditions: newConditions });
    }
  };

  const borderColors = ['border-blue-500', 'border-purple-500', 'border-green-500', 'border-yellow-500'];
  const borderColor = borderColors[depth % borderColors.length];

  return (
    <div className={`${depth > 0 ? `border-l-2 ${borderColor} pl-3 ml-2` : ''}`}>
      <div className="flex items-center gap-2 mb-2">
        {depth > 0 && (
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="p-1 hover:bg-gray-700 rounded"
          >
            {collapsed ? <ChevronRight className="w-4 h-4 text-gray-400" /> : <ChevronDown className="w-4 h-4 text-gray-400" />}
          </button>
        )}
        <div className="text-sm font-medium text-white">
          {depth === 0 ? 'Conditions' : 'Nested Group'}
        </div>
        <div className="flex items-center gap-2">
          {group.conditions.length > 1 && (
            <button
              onClick={() => onChange({ ...group, operator: group.operator === 'AND' ? 'OR' : 'AND' })}
              className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                group.operator === 'AND' ? 'bg-blue-600 text-white' : 'bg-purple-600 text-white'
              }`}
            >
              {group.operator}
            </button>
          )}
          <button
            onClick={addCondition}
            className="px-3 py-1 bg-green-600 hover:bg-green-700 rounded text-xs text-white flex items-center gap-1"
          >
            <Plus className="w-3 h-3" />
            Condition
          </button>
          <button
            onClick={addNestedGroup}
            className="px-3 py-1 bg-purple-600 hover:bg-purple-700 rounded text-xs text-white flex items-center gap-1"
          >
            <Plus className="w-3 h-3" />
            Group
          </button>
          {onRemove && (
            <button
              onClick={onRemove}
              className="p-1 hover:bg-red-600/20 rounded"
            >
              <Trash2 className="w-4 h-4 text-red-400" />
            </button>
          )}
        </div>
      </div>

      {!collapsed && (
        <div className="space-y-2">
          {group.conditions.length === 0 ? (
            <div className="text-sm text-gray-400 text-center py-4 border border-dashed border-gray-600 rounded">
              No conditions yet. Add a condition or nested group.
            </div>
          ) : (
            group.conditions.map((item, index) => (
              <React.Fragment key={isNestedGroup(item) ? item.id : item.id}>
                {index > 0 && (
                  <div className="text-center text-xs text-gray-500 py-1">
                    <span className={`px-2 py-0.5 rounded ${group.operator === 'AND' ? 'bg-blue-600/20 text-blue-400' : 'bg-purple-600/20 text-purple-400'}`}>
                      {group.operator}
                    </span>
                  </div>
                )}
                {isNestedGroup(item) ? (
                  <ConditionGroupComponent
                    group={item}
                    onChange={(updated) => updateNestedGroup(index, updated)}
                    onRemove={() => removeItem(index)}
                    depth={depth + 1}
                  />
                ) : (
                  <ConditionRow
                    condition={item}
                    onUpdate={(updates) => updateCondition(index, updates)}
                    onRemove={() => removeItem(index)}
                    onDuplicate={() => duplicateCondition(index)}
                    index={index}
                  />
                )}
              </React.Fragment>
            ))
          )}
        </div>
      )}

      {group.conditions.length > 1 && (
        <div className="mt-2 text-xs text-gray-400 bg-gray-800/30 rounded px-3 py-2">
          <span className="font-semibold">Logic:</span> All conditions in this group must be{' '}
          <span className={group.operator === 'AND' ? 'text-blue-400' : 'text-purple-400'}>
            {group.operator === 'AND' ? 'TRUE (all must match)' : 'TRUE (any can match)'}
          </span>
        </div>
      )}
    </div>
  );
};

export const AdvancedConditionBuilder: React.FC<AdvancedConditionBuilderProps> = ({
  conditionGroup,
  onChange,
}) => {
  return (
    <div className="space-y-3">
      <ConditionGroupComponent
        group={conditionGroup}
        onChange={onChange}
      />
    </div>
  );
};
