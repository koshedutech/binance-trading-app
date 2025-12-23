import { useEffect, useState, useCallback } from 'react';
import { futuresApi } from '../services/futuresApi';

interface NewsItem {
  title: string;
  source: string;
  url: string;
  sentiment: number;
  published_at: string;
  tickers: string[];
  topic: string;
  is_important: boolean;
}

interface SentimentScore {
  overall: number;
  fear_greed_index: number;
  fear_greed_label: string;
  news_score: number;
  trend_score: number;
  updated_at: string;
  sources: string[];
}

interface SentimentStats {
  bullish: number;
  bearish: number;
  neutral: number;
}

export default function NewsDashboard() {
  const [news, setNews] = useState<NewsItem[]>([]);
  const [sentiment, setSentiment] = useState<SentimentScore | null>(null);
  const [stats, setStats] = useState<SentimentStats>({ bullish: 0, bearish: 0, neutral: 0 });
  const [availableTickers, setAvailableTickers] = useState<string[]>([]);
  const [selectedTicker, setSelectedTicker] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [expanded, setExpanded] = useState(false);

  const fetchNews = useCallback(async () => {
    setLoading(true);
    try {
      const newsData = await futuresApi.getSentimentNews(30, selectedTicker || undefined);
      setNews(newsData.news || []);
      setSentiment(newsData.sentiment);
      setStats(newsData.stats || { bullish: 0, bearish: 0, neutral: 0 });
      setAvailableTickers(newsData.tickers || []);
    } catch (err) {
      console.error('Failed to fetch news:', err);
    } finally {
      setLoading(false);
    }
  }, [selectedTicker]);

  useEffect(() => {
    fetchNews();
    const interval = setInterval(fetchNews, 3 * 60 * 1000);
    return () => clearInterval(interval);
  }, [fetchNews]);

  const getSentimentColor = (value: number) => {
    if (value > 0.2) return 'text-green-400';
    if (value < -0.2) return 'text-red-400';
    return 'text-gray-400';
  };

  const getFearGreedColor = (index: number) => {
    if (index <= 25) return 'text-red-500 bg-red-500/20';
    if (index <= 45) return 'text-orange-400 bg-orange-400/20';
    if (index <= 55) return 'text-yellow-400 bg-yellow-400/20';
    if (index <= 75) return 'text-green-400 bg-green-400/20';
    return 'text-green-500 bg-green-500/20';
  };

  const formatTimeAgo = (dateStr: string): string => {
    const date = new Date(dateStr);
    const now = new Date();
    const diffMins = Math.floor((now.getTime() - date.getTime()) / 60000);
    if (diffMins < 60) return `${diffMins}m`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h`;
    return `${Math.floor(diffHours / 24)}d`;
  };

  const totalNews = stats.bullish + stats.bearish + stats.neutral;
  const topTickers = ['BTC', 'ETH', 'SOL', 'XRP', 'BNB', 'DOGE'].filter(t => availableTickers.includes(t));

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700">
      {/* Compact Header Bar */}
      <div className="flex items-center justify-between px-4 py-2">
        <div className="flex items-center gap-6">
          {/* Title */}
          <div className="flex items-center gap-2">
            <svg className="w-4 h-4 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h10a2 2 0 012 2v1m2 13a2 2 0 01-2-2V7m2 13a2 2 0 002-2V9a2 2 0 00-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z" />
            </svg>
            <span className="text-sm font-medium text-white">News</span>
          </div>

          {/* Fear & Greed */}
          {sentiment && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500">F&G:</span>
              <span className={`px-2 py-0.5 rounded text-xs font-bold ${getFearGreedColor(sentiment.fear_greed_index)}`}>
                {sentiment.fear_greed_index} {sentiment.fear_greed_label}
              </span>
            </div>
          )}

          {/* News Sentiment */}
          {sentiment && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-gray-500">Sentiment:</span>
              <span className={`text-sm font-bold ${getSentimentColor(sentiment.news_score)}`}>
                {sentiment.news_score > 0 ? '+' : ''}{(sentiment.news_score * 100).toFixed(0)}%
              </span>
            </div>
          )}

          {/* Stats */}
          {totalNews > 0 && (
            <div className="flex items-center gap-3 text-xs">
              <span className="text-green-400">{stats.bullish} Bullish</span>
              <span className="text-gray-400">{stats.neutral} Neutral</span>
              <span className="text-red-400">{stats.bearish} Bearish</span>
            </div>
          )}

          {/* Ticker Filters */}
          <div className="flex items-center gap-1">
            <button
              onClick={() => setSelectedTicker('')}
              className={`px-2 py-0.5 text-xs rounded ${selectedTicker === '' ? 'bg-blue-500 text-white' : 'text-gray-400 hover:text-white'}`}
            >
              All
            </button>
            {topTickers.slice(0, 4).map(t => (
              <button
                key={t}
                onClick={() => setSelectedTicker(t)}
                className={`px-2 py-0.5 text-xs rounded ${selectedTicker === t ? 'bg-blue-500 text-white' : 'text-gray-400 hover:text-white'}`}
              >
                {t}
              </button>
            ))}
          </div>
        </div>

        {/* Controls */}
        <div className="flex items-center gap-2">
          <button
            onClick={fetchNews}
            disabled={loading}
            className="p-1 hover:bg-gray-700 rounded"
          >
            <svg className={`w-3 h-3 text-gray-400 ${loading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
          <button
            onClick={() => setExpanded(!expanded)}
            className="p-1 hover:bg-gray-700 rounded"
          >
            <svg className={`w-3 h-3 text-gray-400 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
        </div>
      </div>

      {/* Expanded News List */}
      {expanded && (
        <div className="border-t border-gray-700 p-3">
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-2 max-h-48 overflow-y-auto">
            {news.slice(0, 12).map((item, idx) => (
              <a
                key={idx}
                href={item.url}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-start gap-2 p-2 bg-gray-800 rounded hover:bg-gray-750 group"
              >
                <span className={`mt-1 w-2 h-2 rounded-full flex-shrink-0 ${
                  item.sentiment > 0.2 ? 'bg-green-500' : item.sentiment < -0.2 ? 'bg-red-500' : 'bg-gray-500'
                }`} />
                <div className="min-w-0 flex-1">
                  <div className="text-xs text-gray-200 group-hover:text-white line-clamp-2">{item.title}</div>
                  <div className="flex items-center gap-2 mt-1 text-xs text-gray-500">
                    <span>{item.source}</span>
                    <span>{formatTimeAgo(item.published_at)}</span>
                    {item.tickers?.slice(0, 2).map(t => (
                      <span key={t} className="px-1 bg-blue-500/20 text-blue-400 rounded text-xs">{t}</span>
                    ))}
                  </div>
                </div>
              </a>
            ))}
          </div>
          {news.length === 0 && !loading && (
            <div className="text-center text-gray-500 text-sm py-4">
              No news available. Check CRYPTONEWS_API_KEY.
            </div>
          )}
        </div>
      )}
    </div>
  );
}
