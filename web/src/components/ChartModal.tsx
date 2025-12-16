import React, { useEffect, useRef } from 'react';
import { X } from 'lucide-react';

interface ChartModalProps {
  isOpen: boolean;
  onClose: () => void;
  symbol: string;
  interval: string;
  backtestTrades?: any[]; // Optional, not used in TradingView integration
}

// Map our intervals to TradingView intervals
const intervalMap: { [key: string]: string } = {
  '1m': '1',
  '5m': '5',
  '15m': '15',
  '30m': '30',
  '1h': '60',
  '4h': '240',
  '1d': 'D',
  '1w': 'W',
  '1M': 'M',
};

export const ChartModal: React.FC<ChartModalProps> = ({
  isOpen,
  onClose,
  symbol,
  interval,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen || !containerRef.current) return;

    // Clear previous content
    containerRef.current.innerHTML = '';

    const tradingViewInterval = intervalMap[interval] || '60';

    // Create TradingView widget script
    const script = document.createElement('script');
    script.src = 'https://s3.tradingview.com/tv.js';
    script.async = true;
    script.onload = () => {
      if (containerRef.current && (window as any).TradingView) {
        new (window as any).TradingView.widget({
          autosize: true,
          symbol: `BINANCE:${symbol}`,
          interval: tradingViewInterval,
          timezone: 'Etc/UTC',
          theme: 'dark',
          style: '1',
          locale: 'en',
          toolbar_bg: '#1F2937',
          enable_publishing: false,
          hide_side_toolbar: false,
          allow_symbol_change: false,
          save_image: true,
          container_id: 'tradingview_chart',
          studies: [
            'MASimple@tv-basicstudies',
            'RSI@tv-basicstudies',
            'Volume@tv-basicstudies',
          ],
          disabled_features: ['use_localstorage_for_settings'],
          enabled_features: ['study_templates'],
          overrides: {
            'mainSeriesProperties.candleStyle.upColor': '#10B981',
            'mainSeriesProperties.candleStyle.downColor': '#EF4444',
            'mainSeriesProperties.candleStyle.borderUpColor': '#10B981',
            'mainSeriesProperties.candleStyle.borderDownColor': '#EF4444',
            'mainSeriesProperties.candleStyle.wickUpColor': '#10B981',
            'mainSeriesProperties.candleStyle.wickDownColor': '#EF4444',
          },
        });
      }
    };

    containerRef.current.appendChild(script);

    return () => {
      if (containerRef.current) {
        containerRef.current.innerHTML = '';
      }
    };
  }, [isOpen, symbol, interval]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/80 flex items-center justify-center z-50 p-4">
      <div className="bg-gray-900 rounded-lg shadow-2xl w-full max-w-7xl h-[90vh] flex flex-col">
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700 bg-gray-800">
          <div>
            <h2 className="text-xl font-bold text-white">{symbol}</h2>
            <p className="text-sm text-gray-400">Timeframe: {interval} • Powered by TradingView</p>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-700 rounded-lg transition-colors text-gray-400 hover:text-white"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <div className="flex-1 p-4">
          <div
            id="tradingview_chart"
            ref={containerRef}
            className="w-full h-full rounded-lg overflow-hidden"
          />
        </div>
      </div>
    </div>
  );
};
