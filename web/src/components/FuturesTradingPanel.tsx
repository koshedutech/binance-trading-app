import { useState, useEffect } from 'react';
import { useFuturesStore } from '../store/futuresStore';
import {
  TrendingUp,
  TrendingDown,
  Settings,
  ChevronDown,
  AlertTriangle,
  Loader2,
  DollarSign,
  Coins,
  Percent,
  Hash,
} from 'lucide-react';
import { formatUSD, formatPrice } from '../services/futuresApi';
import type { FuturesOrderType, TimeInForce } from '../types/futures';

interface LeverageModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentLeverage: number;
  onSave: (leverage: number) => void;
  maxLeverage?: number;
}

function LeverageModal({ isOpen, onClose, currentLeverage, onSave, maxLeverage = 125 }: LeverageModalProps) {
  const [leverage, setLeverage] = useState(currentLeverage);

  useEffect(() => {
    setLeverage(currentLeverage);
  }, [currentLeverage]);

  if (!isOpen) return null;

  const quickButtons = [1, 5, 10, 25, 50, 75, 100, 125].filter(l => l <= maxLeverage);

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-gray-800 rounded-lg p-6 w-96">
        <h3 className="text-lg font-semibold mb-4">Adjust Leverage</h3>

        <div className="mb-4">
          <label className="text-sm text-gray-400 mb-2 block">Leverage</label>
          <input
            type="range"
            min={1}
            max={maxLeverage}
            value={leverage}
            onChange={(e) => setLeverage(Number(e.target.value))}
            className="w-full h-2 bg-gray-700 rounded-lg appearance-none cursor-pointer"
          />
          <div className="text-center text-2xl font-bold mt-2 text-yellow-500">{leverage}x</div>
        </div>

        <div className="flex flex-wrap gap-2 mb-4">
          {quickButtons.map((l) => (
            <button
              key={l}
              onClick={() => setLeverage(l)}
              className={`px-3 py-1 rounded text-sm ${
                leverage === l ? 'bg-yellow-500 text-black' : 'bg-gray-700 hover:bg-gray-600'
              }`}
            >
              {l}x
            </button>
          ))}
        </div>

        <div className="bg-yellow-500/10 border border-yellow-500/30 rounded p-3 mb-4">
          <div className="flex items-center gap-2 text-yellow-500 text-sm">
            <AlertTriangle className="w-4 h-4" />
            <span>Higher leverage = higher risk</span>
          </div>
        </div>

        <div className="flex gap-3">
          <button
            onClick={onClose}
            className="flex-1 py-2 bg-gray-700 hover:bg-gray-600 rounded"
          >
            Cancel
          </button>
          <button
            onClick={() => {
              onSave(leverage);
              onClose();
            }}
            className="flex-1 py-2 bg-yellow-500 hover:bg-yellow-600 text-black font-semibold rounded"
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  );
}

export default function FuturesTradingPanel() {
  const {
    orderForm,
    updateOrderForm,
    placeOrder,
    setLeverage,
    setMarginType,
    positionMode,
    setPositionMode,
    markPrice,
    accountInfo,
    isLoading,
    error,
    selectedSymbol,
    fetchAccountSettings,
    tradingMode,
    fetchTradingMode,
  } = useFuturesStore();

  const [showLeverageModal, setShowLeverageModal] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);

  useEffect(() => {
    fetchAccountSettings(selectedSymbol);
    fetchTradingMode();
  }, [selectedSymbol, fetchAccountSettings, fetchTradingMode]);

  const orderTypes: { value: FuturesOrderType; label: string }[] = [
    { value: 'LIMIT', label: 'Limit' },
    { value: 'MARKET', label: 'Market' },
    { value: 'STOP', label: 'Stop-Limit' },
    { value: 'STOP_MARKET', label: 'Stop-Market' },
    { value: 'TAKE_PROFIT', label: 'Take-Profit' },
    { value: 'TAKE_PROFIT_MARKET', label: 'TP-Market' },
  ];

  const timeInForceOptions: { value: TimeInForce; label: string }[] = [
    { value: 'GTC', label: 'GTC' },
    { value: 'IOC', label: 'IOC' },
    { value: 'FOK', label: 'FOK' },
    { value: 'GTX', label: 'Post Only' },
  ];

  const handlePlaceOrder = async (side: 'BUY' | 'SELL') => {
    updateOrderForm({ side });
    const success = await placeOrder();
    if (success) {
      // Reset quantity/amount after successful order (keep TP/SL percentages)
      updateOrderForm({ quantity: '', usdAmount: '', price: '', stopPrice: '', takeProfit: '', stopLoss: '' });
    }
  };

  const currentPrice = markPrice?.markPrice || 0;
  const availableBalance = accountInfo?.available_balance || 0;

  // Calculate preview TP/SL prices based on current mode
  const calculateTPSLPreview = () => {
    if (orderForm.tpSlMode !== 'percent' || !currentPrice) return { tp: null, sl: null };

    const tpPercent = parseFloat(orderForm.takeProfitPercent) / 100 || 0;
    const slPercent = parseFloat(orderForm.stopLossPercent) / 100 || 0;
    const isLong = orderForm.side === 'BUY' || orderForm.positionSide === 'LONG';

    return {
      tp: tpPercent > 0 ? (isLong ? currentPrice * (1 + tpPercent) : currentPrice * (1 - tpPercent)) : null,
      sl: slPercent > 0 ? (isLong ? currentPrice * (1 - slPercent) : currentPrice * (1 + slPercent)) : null,
    };
  };

  const tpslPreview = calculateTPSLPreview();

  const handleLeverageSave = async (leverage: number) => {
    await setLeverage(selectedSymbol, leverage);
  };

  const handleMarginTypeChange = async (marginType: 'CROSSED' | 'ISOLATED') => {
    await setMarginType(selectedSymbol, marginType);
  };

  const handlePositionModeChange = async (hedge: boolean) => {
    await setPositionMode(hedge);
    if (hedge) {
      updateOrderForm({ positionSide: 'LONG' });
    } else {
      updateOrderForm({ positionSide: 'BOTH' });
    }
  };
  const maxQuantity = orderForm.leverage > 0 && currentPrice > 0
    ? (availableBalance * orderForm.leverage) / currentPrice
    : 0;

  const estimatedCost = orderForm.quantity && currentPrice
    ? (parseFloat(orderForm.quantity) * currentPrice) / orderForm.leverage
    : 0;

  const setQuantityPercent = (percent: number) => {
    const qty = (maxQuantity * percent) / 100;
    updateOrderForm({ quantity: qty.toFixed(4) });
  };

  const needsPrice = ['LIMIT', 'STOP', 'TAKE_PROFIT'].includes(orderForm.orderType);
  const needsStopPrice = ['STOP', 'STOP_MARKET', 'TAKE_PROFIT', 'TAKE_PROFIT_MARKET'].includes(orderForm.orderType);

  return (
    <div className="bg-gray-900 rounded-lg p-4 border border-gray-700">
      {/* Trading Mode Indicator */}
      <div className={`mb-3 px-3 py-2 rounded-lg flex items-center justify-between ${
        tradingMode.mode === 'live'
          ? 'bg-green-500/10 border border-green-500/30'
          : 'bg-yellow-500/10 border border-yellow-500/30'
      }`}>
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${
            tradingMode.mode === 'live'
              ? 'bg-green-500 animate-pulse'
              : 'bg-yellow-500'
          }`} />
          <span className={`text-sm font-medium ${
            tradingMode.mode === 'live' ? 'text-green-500' : 'text-yellow-500'
          }`}>
            {tradingMode.modeLabel}
          </span>
        </div>
        <span className="text-xs text-gray-400">
          {tradingMode.mode === 'live' ? 'Real Wallet Connected' : 'Simulated Balance'}
        </span>
      </div>

      {/* Header - Margin & Leverage */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          {/* Margin Type Toggle */}
          <div className="flex bg-gray-800 rounded">
            <button
              onClick={() => handleMarginTypeChange('CROSSED')}
              className={`px-3 py-1.5 text-sm rounded-l ${
                orderForm.marginType === 'CROSSED'
                  ? 'bg-yellow-500 text-black font-semibold'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Cross
            </button>
            <button
              onClick={() => handleMarginTypeChange('ISOLATED')}
              className={`px-3 py-1.5 text-sm rounded-r ${
                orderForm.marginType === 'ISOLATED'
                  ? 'bg-yellow-500 text-black font-semibold'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Isolated
            </button>
          </div>

          {/* Leverage Button */}
          <button
            onClick={() => setShowLeverageModal(true)}
            className="px-3 py-1.5 bg-gray-800 hover:bg-gray-700 rounded text-yellow-500 font-semibold text-sm"
          >
            {orderForm.leverage}x
          </button>
        </div>

        {/* Position Mode Toggle */}
        <div className="flex items-center gap-2">
          <span className="text-xs text-gray-500">Mode:</span>
          <div className="flex bg-gray-800 rounded">
            <button
              onClick={() => handlePositionModeChange(false)}
              className={`px-2 py-1 text-xs rounded-l ${
                positionMode === 'ONE_WAY'
                  ? 'bg-gray-600 text-white'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              One-Way
            </button>
            <button
              onClick={() => handlePositionModeChange(true)}
              className={`px-2 py-1 text-xs rounded-r ${
                positionMode === 'HEDGE'
                  ? 'bg-gray-600 text-white'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              Hedge
            </button>
          </div>
        </div>
      </div>

      {/* Order Type Selector */}
      <div className="mb-4">
        <div className="grid grid-cols-3 gap-1 bg-gray-800 p-1 rounded">
          {orderTypes.map((type) => (
            <button
              key={type.value}
              onClick={() => updateOrderForm({ orderType: type.value })}
              className={`py-1.5 text-xs rounded ${
                orderForm.orderType === type.value
                  ? 'bg-gray-600 text-white'
                  : 'text-gray-400 hover:text-white hover:bg-gray-700'
              }`}
            >
              {type.label}
            </button>
          ))}
        </div>
      </div>

      {/* Position Side (Hedge Mode) */}
      {positionMode === 'HEDGE' && (
        <div className="mb-4">
          <label className="text-xs text-gray-400 mb-1 block">Position Side</label>
          <div className="flex gap-2">
            <button
              onClick={() => updateOrderForm({ positionSide: 'LONG' })}
              className={`flex-1 py-2 rounded text-sm font-semibold ${
                orderForm.positionSide === 'LONG'
                  ? 'bg-green-500/20 text-green-500 border border-green-500'
                  : 'bg-gray-800 text-gray-400 hover:text-white'
              }`}
            >
              Long
            </button>
            <button
              onClick={() => updateOrderForm({ positionSide: 'SHORT' })}
              className={`flex-1 py-2 rounded text-sm font-semibold ${
                orderForm.positionSide === 'SHORT'
                  ? 'bg-red-500/20 text-red-500 border border-red-500'
                  : 'bg-gray-800 text-gray-400 hover:text-white'
              }`}
            >
              Short
            </button>
          </div>
        </div>
      )}

      {/* Price Input (for Limit orders) */}
      {needsPrice && (
        <div className="mb-3">
          <label className="text-xs text-gray-400 mb-1 block">Price</label>
          <div className="relative">
            <input
              type="number"
              value={orderForm.price}
              onChange={(e) => updateOrderForm({ price: e.target.value })}
              placeholder={currentPrice ? formatPrice(currentPrice) : '0.00'}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right pr-16 focus:border-yellow-500 focus:outline-none"
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 text-sm">USDT</span>
          </div>
        </div>
      )}

      {/* Stop Price Input (for Stop orders) */}
      {needsStopPrice && (
        <div className="mb-3">
          <label className="text-xs text-gray-400 mb-1 block">Stop Price</label>
          <div className="relative">
            <input
              type="number"
              value={orderForm.stopPrice}
              onChange={(e) => updateOrderForm({ stopPrice: e.target.value })}
              placeholder="0.00"
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right pr-16 focus:border-yellow-500 focus:outline-none"
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 text-sm">USDT</span>
          </div>
        </div>
      )}

      {/* Amount Input with USD/Coin Toggle */}
      <div className="mb-3">
        <div className="flex items-center justify-between mb-1">
          <label className="text-xs text-gray-400">Amount</label>
          {/* USD/Coin Toggle */}
          <div className="flex bg-gray-800 rounded text-xs">
            <button
              onClick={() => updateOrderForm({ amountMode: 'coin', usdAmount: '' })}
              className={`px-2 py-1 rounded-l flex items-center gap-1 ${
                orderForm.amountMode === 'coin'
                  ? 'bg-yellow-500 text-black font-semibold'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <Coins className="w-3 h-3" />
              Coin
            </button>
            <button
              onClick={() => updateOrderForm({ amountMode: 'usd', quantity: '' })}
              className={`px-2 py-1 rounded-r flex items-center gap-1 ${
                orderForm.amountMode === 'usd'
                  ? 'bg-green-500 text-black font-semibold'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <DollarSign className="w-3 h-3" />
              USD
            </button>
          </div>
        </div>

        {orderForm.amountMode === 'coin' ? (
          /* Coin Amount Input */
          <div className="relative">
            <input
              type="number"
              value={orderForm.quantity}
              onChange={(e) => updateOrderForm({ quantity: e.target.value })}
              placeholder="0.000"
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right pr-16 focus:border-yellow-500 focus:outline-none"
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 text-sm">
              {selectedSymbol.replace('USDT', '')}
            </span>
          </div>
        ) : (
          /* USD Amount Input */
          <div className="relative">
            <input
              type="number"
              value={orderForm.usdAmount}
              onChange={(e) => updateOrderForm({ usdAmount: e.target.value })}
              placeholder="0.00"
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right pr-16 focus:border-green-500 focus:outline-none"
            />
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 text-sm">
              USDT
            </span>
          </div>
        )}

        {/* Conversion Display */}
        {orderForm.amountMode === 'usd' && orderForm.usdAmount && currentPrice > 0 && (
          <div className="mt-1 text-xs text-gray-400 flex justify-between">
            <span>Equivalent:</span>
            <span className="text-yellow-500">
              {(parseFloat(orderForm.usdAmount) / currentPrice).toFixed(6)} {selectedSymbol.replace('USDT', '')}
            </span>
          </div>
        )}
        {orderForm.amountMode === 'coin' && orderForm.quantity && currentPrice > 0 && (
          <div className="mt-1 text-xs text-gray-400 flex justify-between">
            <span>Value:</span>
            <span className="text-green-500">
              {formatUSD(parseFloat(orderForm.quantity) * currentPrice)}
            </span>
          </div>
        )}

        {/* Quick Amount Buttons */}
        <div className="flex gap-2 mt-2">
          {[25, 50, 75, 100].map((percent) => (
            <button
              key={percent}
              onClick={() => {
                if (orderForm.amountMode === 'coin') {
                  setQuantityPercent(percent);
                } else {
                  // Set USD amount as percentage of available balance
                  const usdAmt = (availableBalance * percent) / 100;
                  updateOrderForm({ usdAmount: usdAmt.toFixed(2) });
                }
              }}
              className="flex-1 py-1 bg-gray-800 hover:bg-gray-700 rounded text-xs text-gray-400 hover:text-white"
            >
              {percent}%
            </button>
          ))}
        </div>
      </div>

      {/* TP/SL Inputs */}
      <div className="mb-3">
        {/* Mode Toggle */}
        <div className="flex items-center justify-between mb-2">
          <label className="text-xs text-gray-400">TP / SL</label>
          <div className="flex bg-gray-800 rounded text-xs">
            <button
              onClick={() => updateOrderForm({ tpSlMode: 'percent' })}
              className={`px-2 py-1 rounded-l flex items-center gap-1 ${
                orderForm.tpSlMode === 'percent'
                  ? 'bg-yellow-500 text-black font-semibold'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <Percent className="w-3 h-3" />
              %
            </button>
            <button
              onClick={() => updateOrderForm({ tpSlMode: 'price' })}
              className={`px-2 py-1 rounded-r flex items-center gap-1 ${
                orderForm.tpSlMode === 'price'
                  ? 'bg-yellow-500 text-black font-semibold'
                  : 'text-gray-400 hover:text-white'
              }`}
            >
              <Hash className="w-3 h-3" />
              Price
            </button>
          </div>
        </div>

        {orderForm.tpSlMode === 'percent' ? (
          /* Percentage Mode */
          <div className="grid grid-cols-2 gap-2">
            <div>
              <div className="relative">
                <input
                  type="number"
                  value={orderForm.takeProfitPercent}
                  onChange={(e) => updateOrderForm({ takeProfitPercent: e.target.value })}
                  placeholder="2"
                  step="0.1"
                  min="0"
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right text-sm pr-8 focus:border-green-500 focus:outline-none"
                />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-green-500 text-sm">%</span>
              </div>
              {tpslPreview.tp && (
                <div className="text-xs text-green-500 mt-1 text-right">
                  TP: {formatPrice(tpslPreview.tp)}
                </div>
              )}
            </div>
            <div>
              <div className="relative">
                <input
                  type="number"
                  value={orderForm.stopLossPercent}
                  onChange={(e) => updateOrderForm({ stopLossPercent: e.target.value })}
                  placeholder="1"
                  step="0.1"
                  min="0"
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right text-sm pr-8 focus:border-red-500 focus:outline-none"
                />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-red-500 text-sm">%</span>
              </div>
              {tpslPreview.sl && (
                <div className="text-xs text-red-500 mt-1 text-right">
                  SL: {formatPrice(tpslPreview.sl)}
                </div>
              )}
            </div>
          </div>
        ) : (
          /* Price Mode */
          <div className="grid grid-cols-2 gap-2">
            <div>
              <input
                type="number"
                value={orderForm.takeProfit}
                onChange={(e) => updateOrderForm({ takeProfit: e.target.value })}
                placeholder="TP Price"
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right text-sm focus:border-green-500 focus:outline-none"
              />
            </div>
            <div>
              <input
                type="number"
                value={orderForm.stopLoss}
                onChange={(e) => updateOrderForm({ stopLoss: e.target.value })}
                placeholder="SL Price"
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-right text-sm focus:border-red-500 focus:outline-none"
              />
            </div>
          </div>
        )}
      </div>

      {/* Advanced Options Toggle */}
      <button
        onClick={() => setShowAdvanced(!showAdvanced)}
        className="flex items-center gap-1 text-xs text-gray-400 hover:text-white mb-3"
      >
        <Settings className="w-3 h-3" />
        Advanced
        <ChevronDown className={`w-3 h-3 transition-transform ${showAdvanced ? 'rotate-180' : ''}`} />
      </button>

      {/* Advanced Options */}
      {showAdvanced && (
        <div className="space-y-3 mb-4 p-3 bg-gray-800 rounded">
          {/* Time in Force */}
          <div>
            <label className="text-xs text-gray-400 mb-1 block">Time in Force</label>
            <select
              value={orderForm.timeInForce}
              onChange={(e) => updateOrderForm({ timeInForce: e.target.value as TimeInForce })}
              className="w-full bg-gray-700 border border-gray-600 rounded px-3 py-2 text-sm focus:border-yellow-500 focus:outline-none"
            >
              {timeInForceOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          {/* Reduce Only */}
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={orderForm.reduceOnly}
              onChange={(e) => updateOrderForm({ reduceOnly: e.target.checked })}
              className="w-4 h-4 rounded bg-gray-700 border-gray-600 text-yellow-500 focus:ring-yellow-500"
            />
            <span className="text-sm text-gray-300">Reduce Only</span>
          </label>
        </div>
      )}

      {/* Order Summary */}
      <div className="text-xs text-gray-400 mb-4 space-y-1">
        {/* Mark Price with Real/Simulated indicator */}
        <div className="flex justify-between items-center">
          <span className="flex items-center gap-1">
            Mark Price
            <span className={`text-[10px] px-1 rounded ${
              tradingMode.mode === 'live'
                ? 'bg-green-500/20 text-green-400'
                : 'bg-yellow-500/20 text-yellow-400'
            }`}>
              {tradingMode.mode === 'live' ? 'LIVE' : 'SIM'}
            </span>
          </span>
          <span className="text-white font-medium">{formatPrice(currentPrice)}</span>
        </div>
        <div className="flex justify-between">
          <span>Available:</span>
          <span>{formatUSD(availableBalance)}</span>
        </div>
        <div className="flex justify-between">
          <span>Max ({orderForm.leverage}x):</span>
          <span>{maxQuantity.toFixed(4)} {selectedSymbol.replace('USDT', '')}</span>
        </div>
        {estimatedCost > 0 && (
          <div className="flex justify-between">
            <span>Cost:</span>
            <span>{formatUSD(estimatedCost)}</span>
          </div>
        )}
      </div>

      {/* Error Message */}
      {error && (
        <div className="mb-4 p-2 bg-red-500/10 border border-red-500/30 rounded text-red-500 text-sm">
          {error}
        </div>
      )}

      {/* Buy/Sell Buttons */}
      {positionMode === 'HEDGE' ? (
        // Hedge Mode - Open Long / Open Short
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => {
              updateOrderForm({ positionSide: 'LONG', side: 'BUY' });
              handlePlaceOrder('BUY');
            }}
            disabled={isLoading}
            className="py-3 bg-green-600 hover:bg-green-700 disabled:bg-green-600/50 rounded font-semibold flex items-center justify-center gap-2"
          >
            {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <TrendingUp className="w-4 h-4" />}
            Open Long
          </button>
          <button
            onClick={() => {
              updateOrderForm({ positionSide: 'SHORT', side: 'SELL' });
              handlePlaceOrder('SELL');
            }}
            disabled={isLoading}
            className="py-3 bg-red-600 hover:bg-red-700 disabled:bg-red-600/50 rounded font-semibold flex items-center justify-center gap-2"
          >
            {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <TrendingDown className="w-4 h-4" />}
            Open Short
          </button>
        </div>
      ) : (
        // One-Way Mode - Buy/Long, Sell/Short
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => handlePlaceOrder('BUY')}
            disabled={isLoading}
            className="py-3 bg-green-600 hover:bg-green-700 disabled:bg-green-600/50 rounded font-semibold flex items-center justify-center gap-2"
          >
            {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <TrendingUp className="w-4 h-4" />}
            Buy/Long
          </button>
          <button
            onClick={() => handlePlaceOrder('SELL')}
            disabled={isLoading}
            className="py-3 bg-red-600 hover:bg-red-700 disabled:bg-red-600/50 rounded font-semibold flex items-center justify-center gap-2"
          >
            {isLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <TrendingDown className="w-4 h-4" />}
            Sell/Short
          </button>
        </div>
      )}

      {/* Leverage Modal */}
      <LeverageModal
        isOpen={showLeverageModal}
        onClose={() => setShowLeverageModal(false)}
        currentLeverage={orderForm.leverage}
        onSave={handleLeverageSave}
      />
    </div>
  );
}
