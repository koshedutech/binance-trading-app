import React from 'react';
import { Plus, Trash2, Grip } from 'lucide-react';

export interface Condition {
  id: string;
  leftOperand: {
    type: 'price' | 'indicator' | 'value';
    indicator?: string;
    params?: Record<string, any>;
  };
  operator: '>' | '<' | '>=' | '<=' | '==' | '!=';
  rightOperand: {
    type: 'price' | 'indicator' | 'value';
    value?: number;
    indicator?: string;
    params?: Record<string, any>;
  };
}

export interface ConditionGroup {
  operator: 'AND' | 'OR';
  conditions: Condition[];
}

interface ConditionBuilderProps {
  conditionGroup: ConditionGroup;
  onChange: (conditionGroup: ConditionGroup) => void;
}

const OPERATORS = [
  { value: '>', label: '> (Greater Than)' },
  { value: '<', label: '< (Less Than)' },
  { value: '>=', label: '>= (Greater or Equal)' },
  { value: '<=', label: '<= (Less or Equal)' },
  { value: '==', label: '== (Equal To)' },
  { value: '!=', label: '!= (Not Equal)' },
];

const INDICATORS = [
  { value: 'RSI', label: 'RSI', defaultParams: { period: 14 } },
  { value: 'SMA', label: 'SMA', defaultParams: { period: 20 } },
  { value: 'EMA', label: 'EMA', defaultParams: { period: 20 } },
  { value: 'MACD', label: 'MACD', defaultParams: { fastPeriod: 12, slowPeriod: 26, signalPeriod: 9, type: 'histogram' } },
  { value: 'BollingerBands', label: 'Bollinger Bands', defaultParams: { period: 20, stdDev: 2, band: 'lower' } },
  { value: 'Stochastic', label: 'Stochastic', defaultParams: { kPeriod: 14, dPeriod: 3, type: 'k' } },
  { value: 'ATR', label: 'ATR', defaultParams: { period: 14 } },
  { value: 'ADX', label: 'ADX', defaultParams: { period: 14 } },
];

export const ConditionBuilder: React.FC<ConditionBuilderProps> = ({ conditionGroup, onChange }) => {
  const addCondition = () => {
    const newCondition: Condition = {
      id: Date.now().toString(),
      leftOperand: { type: 'price' },
      operator: '>',
      rightOperand: { type: 'indicator', indicator: 'EMA', params: { period: 20 } },
    };

    onChange({
      ...conditionGroup,
      conditions: [...conditionGroup.conditions, newCondition],
    });
  };

  const removeCondition = (id: string) => {
    onChange({
      ...conditionGroup,
      conditions: conditionGroup.conditions.filter((c) => c.id !== id),
    });
  };

  const updateCondition = (id: string, updates: Partial<Condition>) => {
    onChange({
      ...conditionGroup,
      conditions: conditionGroup.conditions.map((c) => (c.id === id ? { ...c, ...updates } : c)),
    });
  };

  const toggleGroupOperator = () => {
    onChange({
      ...conditionGroup,
      operator: conditionGroup.operator === 'AND' ? 'OR' : 'AND',
    });
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div className="text-sm font-medium text-white">Conditions</div>
        <div className="flex items-center gap-2">
          {conditionGroup.conditions.length > 1 && (
            <button
              onClick={toggleGroupOperator}
              className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                conditionGroup.operator === 'AND'
                  ? 'bg-blue-600 text-white'
                  : 'bg-purple-600 text-white'
              }`}
            >
              {conditionGroup.operator}
            </button>
          )}
          <button
            onClick={addCondition}
            className="px-3 py-1 bg-green-600 hover:bg-green-700 rounded text-xs text-white flex items-center gap-1 transition-colors"
          >
            <Plus className="w-3 h-3" />
            Add Condition
          </button>
        </div>
      </div>

      <div className="space-y-2">
        {conditionGroup.conditions.length === 0 ? (
          <div className="text-sm text-gray-400 text-center py-4 border border-dashed border-gray-600 rounded">
            No conditions yet. Click "Add Condition" to start building your strategy.
          </div>
        ) : (
          conditionGroup.conditions.map((condition, index) => (
            <div key={condition.id} className="bg-gray-700/30 border border-gray-600 rounded-lg p-3">
              <div className="flex items-center gap-2 mb-2">
                <Grip className="w-4 h-4 text-gray-500 cursor-move" />
                <span className="text-xs text-gray-400">Condition {index + 1}</span>
                <div className="flex-1"></div>
                <button
                  onClick={() => removeCondition(condition.id)}
                  className="p-1 hover:bg-red-600/20 rounded transition-colors"
                >
                  <Trash2 className="w-4 h-4 text-red-400" />
                </button>
              </div>

              <div className="grid grid-cols-[1fr,auto,1fr] gap-2 items-center">
                {/* Left Operand */}
                <div className="space-y-2">
                  <select
                    value={condition.leftOperand.type}
                    onChange={(e) =>
                      updateCondition(condition.id, {
                        leftOperand: {
                          type: e.target.value as any,
                          ...(e.target.value === 'indicator' && {
                            indicator: 'EMA',
                            params: { period: 20 },
                          }),
                        },
                      })
                    }
                    className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  >
                    <option value="price">Current Price (LTP)</option>
                    <option value="indicator">Indicator</option>
                  </select>

                  {condition.leftOperand.type === 'indicator' && (
                    <select
                      value={condition.leftOperand.indicator}
                      onChange={(e) => {
                        const indicator = INDICATORS.find((i) => i.value === e.target.value);
                        updateCondition(condition.id, {
                          leftOperand: {
                            ...condition.leftOperand,
                            indicator: e.target.value,
                            params: indicator?.defaultParams || {},
                          },
                        });
                      }}
                      className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                    >
                      {INDICATORS.map((ind) => (
                        <option key={ind.value} value={ind.value}>
                          {ind.label}
                        </option>
                      ))}
                    </select>
                  )}

                  {condition.leftOperand.type === 'indicator' && condition.leftOperand.indicator && (
                    <div className="text-[10px] text-gray-500">
                      {condition.leftOperand.indicator === 'RSI' && (
                        <input
                          type="number"
                          value={condition.leftOperand.params?.period || 14}
                          onChange={(e) =>
                            updateCondition(condition.id, {
                              leftOperand: {
                                ...condition.leftOperand,
                                params: { period: parseInt(e.target.value) },
                              },
                            })
                          }
                          placeholder="Period"
                          className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white"
                        />
                      )}
                      {(condition.leftOperand.indicator === 'EMA' ||
                        condition.leftOperand.indicator === 'SMA') && (
                        <input
                          type="number"
                          value={condition.leftOperand.params?.period || 20}
                          onChange={(e) =>
                            updateCondition(condition.id, {
                              leftOperand: {
                                ...condition.leftOperand,
                                params: { period: parseInt(e.target.value) },
                              },
                            })
                          }
                          placeholder="Period"
                          className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white"
                        />
                      )}
                    </div>
                  )}
                </div>

                {/* Operator */}
                <select
                  value={condition.operator}
                  onChange={(e) => updateCondition(condition.id, { operator: e.target.value as any })}
                  className="px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs font-mono"
                >
                  {OPERATORS.map((op) => (
                    <option key={op.value} value={op.value}>
                      {op.value}
                    </option>
                  ))}
                </select>

                {/* Right Operand */}
                <div className="space-y-2">
                  <select
                    value={condition.rightOperand.type}
                    onChange={(e) =>
                      updateCondition(condition.id, {
                        rightOperand: {
                          type: e.target.value as any,
                          ...(e.target.value === 'value' && { value: 0 }),
                          ...(e.target.value === 'indicator' && {
                            indicator: 'EMA',
                            params: { period: 20 },
                          }),
                        },
                      })
                    }
                    className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                  >
                    <option value="value">Fixed Value</option>
                    <option value="indicator">Indicator</option>
                    <option value="price">Current Price</option>
                  </select>

                  {condition.rightOperand.type === 'value' && (
                    <input
                      type="number"
                      value={condition.rightOperand.value || 0}
                      onChange={(e) =>
                        updateCondition(condition.id, {
                          rightOperand: { ...condition.rightOperand, value: parseFloat(e.target.value) },
                        })
                      }
                      step="0.01"
                      className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                      placeholder="Enter value"
                    />
                  )}

                  {condition.rightOperand.type === 'indicator' && (
                    <>
                      <select
                        value={condition.rightOperand.indicator}
                        onChange={(e) => {
                          const indicator = INDICATORS.find((i) => i.value === e.target.value);
                          updateCondition(condition.id, {
                            rightOperand: {
                              ...condition.rightOperand,
                              indicator: e.target.value,
                              params: indicator?.defaultParams || {},
                            },
                          });
                        }}
                        className="w-full px-2 py-1.5 bg-gray-700 border border-gray-600 rounded text-white text-xs"
                      >
                        {INDICATORS.map((ind) => (
                          <option key={ind.value} value={ind.value}>
                            {ind.label}
                          </option>
                        ))}
                      </select>

                      {condition.rightOperand.indicator && (
                        <div className="text-[10px] text-gray-500">
                          {(condition.rightOperand.indicator === 'EMA' ||
                            condition.rightOperand.indicator === 'SMA') && (
                            <input
                              type="number"
                              value={condition.rightOperand.params?.period || 20}
                              onChange={(e) =>
                                updateCondition(condition.id, {
                                  rightOperand: {
                                    ...condition.rightOperand,
                                    params: { period: parseInt(e.target.value) },
                                  },
                                })
                              }
                              placeholder="Period"
                              className="w-full px-2 py-1 bg-gray-700 border border-gray-600 rounded text-white"
                            />
                          )}
                        </div>
                      )}
                    </>
                  )}
                </div>
              </div>

              {/* Condition Summary */}
              <div className="mt-2 text-xs text-gray-400 bg-gray-800/50 rounded px-2 py-1">
                {condition.leftOperand.type === 'price' && 'Current Price'}
                {condition.leftOperand.type === 'indicator' &&
                  `${condition.leftOperand.indicator}(${condition.leftOperand.params?.period || ''})`}
                <span className="text-blue-400 mx-1 font-mono">{condition.operator}</span>
                {condition.rightOperand.type === 'value' && condition.rightOperand.value}
                {condition.rightOperand.type === 'price' && 'Current Price'}
                {condition.rightOperand.type === 'indicator' &&
                  `${condition.rightOperand.indicator}(${condition.rightOperand.params?.period || ''})`}
              </div>
            </div>
          ))
        )}
      </div>

      {conditionGroup.conditions.length > 1 && (
        <div className="text-xs text-gray-400 bg-gray-800/30 rounded px-3 py-2">
          <span className="font-semibold">Logic:</span> All conditions must be{' '}
          <span className={conditionGroup.operator === 'AND' ? 'text-blue-400' : 'text-purple-400'}>
            {conditionGroup.operator}
          </span>{' '}
          satisfied for entry
        </div>
      )}
    </div>
  );
};
