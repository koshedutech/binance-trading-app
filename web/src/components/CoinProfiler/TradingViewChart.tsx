import { memo } from 'react';

// ============================================================================
// Epic 14: TradingView Chart Integration
// Embeds TradingView chart widget for real-time coin visualization
// ============================================================================

interface TradingViewChartProps {
  symbol: string;
  timeframe?: string;
  height?: number;
}

/**
 * TradingView Chart Component
 * Displays an embedded TradingView chart for the given Binance Futures symbol.
 *
 * Features:
 * - Real-time price updates from TradingView
 * - Volume displayed at bottom
 * - Dark theme matching our UI
 * - Configurable timeframe
 */
function TradingViewChartComponent({ symbol, timeframe = '15', height = 300 }: TradingViewChartProps) {
  // Convert our symbol format to TradingView format
  // BTCUSDT -> BINANCE:BTCUSDT.P (perpetual futures)
  const tvSymbol = `BINANCE:${symbol}.P`;

  // Build the TradingView widget URL with all parameters
  const widgetUrl = `https://s.tradingview.com/widgetembed/?frameElementId=tradingview_${symbol}&symbol=${encodeURIComponent(tvSymbol)}&interval=${timeframe}&hidesidetoolbar=1&symboledit=0&saveimage=0&toolbarbg=1f2937&studies=[]&theme=dark&style=1&timezone=Etc%2FUTC&studies_overrides={}&overrides={}&enabled_features=[]&disabled_features=[]&locale=en&utm_source=localhost&utm_medium=widget_new&utm_campaign=chart`;

  return (
    <div
      className="rounded overflow-hidden bg-gray-800"
      style={{ height: `${height}px`, minHeight: `${height}px` }}
    >
      <iframe
        id={`tradingview_${symbol}`}
        src={widgetUrl}
        style={{
          width: '100%',
          height: '100%',
          border: 'none',
        }}
        allowTransparency
        scrolling="no"
        allowFullScreen
        title={`TradingView Chart - ${symbol}`}
      />
    </div>
  );
}

// Memoize to prevent unnecessary re-renders
export const TradingViewChart = memo(TradingViewChartComponent);

/**
 * Mini TradingView Chart - smaller version for compact displays
 */
interface MiniChartProps {
  symbol: string;
  height?: number;
}

function MiniChartComponent({ symbol, height = 200 }: MiniChartProps) {
  const tvSymbol = `BINANCE:${symbol}.P`;

  // Mini symbol overview widget URL
  const widgetUrl = `https://s.tradingview.com/embed-widget/mini-symbol-overview/?symbol=${encodeURIComponent(tvSymbol)}&dateRange=1D&colorTheme=dark&isTransparent=true&autosize=true&chartOnly=true&locale=en`;

  return (
    <div
      className="rounded overflow-hidden"
      style={{ height: `${height}px` }}
    >
      <iframe
        src={widgetUrl}
        style={{
          width: '100%',
          height: '100%',
          border: 'none',
        }}
        allowTransparency
        scrolling="no"
        title={`Mini Chart - ${symbol}`}
      />
    </div>
  );
}

export const MiniChart = memo(MiniChartComponent);

export default TradingViewChart;
