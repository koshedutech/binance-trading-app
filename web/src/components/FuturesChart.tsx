import { useEffect, useRef, useState } from 'react';
import { useFuturesStore } from '../store/futuresStore';
import { BarChart3, ChevronDown } from 'lucide-react';

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

const TIMEFRAMES = ['1m', '5m', '15m', '30m', '1h', '4h', '1d'];

export default function FuturesChart() {
  const { selectedSymbol } = useFuturesStore();
  const containerRef = useRef<HTMLDivElement>(null);
  const [interval, setInterval] = useState('1h');
  const [showIntervalSelect, setShowIntervalSelect] = useState(false);

  useEffect(() => {
    if (!containerRef.current) return;

    // Clear previous content
    containerRef.current.innerHTML = '';

    const tradingViewInterval = intervalMap[interval] || '60';

    // Create unique container ID
    const containerId = `tradingview_futures_${Date.now()}`;
    const chartDiv = document.createElement('div');
    chartDiv.id = containerId;
    chartDiv.style.width = '100%';
    chartDiv.style.height = '100%';
    containerRef.current.appendChild(chartDiv);

    // Create TradingView widget script
    const script = document.createElement('script');
    script.src = 'https://s3.tradingview.com/tv.js';
    script.async = true;
    script.onload = () => {
      if ((window as any).TradingView && document.getElementById(containerId)) {
        new (window as any).TradingView.widget({
          autosize: true,
          symbol: `BINANCE:${selectedSymbol}.P`,
          interval: tradingViewInterval,
          timezone: 'Etc/UTC',
          theme: 'dark',
          style: '1',
          locale: 'en',
          toolbar_bg: '#111827',
          enable_publishing: false,
          hide_side_toolbar: true,
          hide_top_toolbar: false,
          allow_symbol_change: false,
          save_image: false,
          container_id: containerId,
          studies: [
            'MASimple@tv-basicstudies',
            'Volume@tv-basicstudies',
          ],
          disabled_features: [
            'use_localstorage_for_settings',
            'header_symbol_search',
            'header_compare',
            'header_undo_redo',
            'header_screenshot',
            'header_saveload',
          ],
          enabled_features: [],
          overrides: {
            'paneProperties.background': '#111827',
            'paneProperties.vertGridProperties.color': '#1f2937',
            'paneProperties.horzGridProperties.color': '#1f2937',
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

    document.body.appendChild(script);

    return () => {
      if (containerRef.current) {
        containerRef.current.innerHTML = '';
      }
      // Cleanup script
      script.remove();
    };
  }, [selectedSymbol, interval]);

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 overflow-hidden h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <BarChart3 className="w-4 h-4 text-blue-400" />
          <span className="text-sm font-semibold">Chart</span>
          <span className="text-xs text-gray-500">{selectedSymbol}</span>
        </div>

        {/* Interval Selector */}
        <div className="relative">
          <button
            onClick={() => setShowIntervalSelect(!showIntervalSelect)}
            className="flex items-center gap-1 px-2 py-1 bg-gray-800 hover:bg-gray-700 rounded text-xs border border-gray-700"
          >
            <span>{interval}</span>
            <ChevronDown className="w-3 h-3" />
          </button>

          {showIntervalSelect && (
            <div className="absolute right-0 top-full mt-1 z-50 bg-gray-800 border border-gray-700 rounded shadow-lg">
              {TIMEFRAMES.map((tf) => (
                <button
                  key={tf}
                  onClick={() => {
                    setInterval(tf);
                    setShowIntervalSelect(false);
                  }}
                  className={`block w-full px-4 py-2 text-xs text-left hover:bg-gray-700 ${
                    interval === tf ? 'bg-gray-700 text-blue-400' : 'text-gray-300'
                  }`}
                >
                  {tf}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Chart Container */}
      <div className="flex-1 min-h-0" ref={containerRef} />
    </div>
  );
}
