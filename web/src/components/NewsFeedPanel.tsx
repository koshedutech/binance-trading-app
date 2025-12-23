import { useEffect, useState, useCallback } from 'react';
import { futuresApi } from '../services/futuresApi';

interface NewsItem {
  title: string;
  source: string;
  url: string;
  sentiment: number;
  published_at: string;
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

export default function NewsFeedPanel() {
  const [news, setNews] = useState<NewsItem[]>([]);
  const [sentiment, setSentiment] = useState<SentimentScore | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchNews = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await futuresApi.getSentimentNews(20);
      setNews(data.news || []);
      setSentiment(data.sentiment);
    } catch (err) {
      console.error('Failed to fetch news:', err);
      setError('Failed to load news');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchNews();
    const interval = setInterval(fetchNews, 5 * 60 * 1000); // 5 min refresh
    return () => clearInterval(interval);
  }, [fetchNews]);

  const getSentimentColor = (value: number) => {
    if (value > 0.3) return 'text-green-400';
    if (value < -0.3) return 'text-red-400';
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
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);

    if (diffMins < 60) return `${diffMins}m ago`;
    const diffHours = Math.floor(diffMins / 60);
    if (diffHours < 24) return `${diffHours}h ago`;
    const diffDays = Math.floor(diffHours / 24);
    return `${diffDays}d ago`;
  };

  return (
    <div className="bg-gray-900 rounded-lg border border-gray-700 p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <svg className="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 20H5a2 2 0 01-2-2V6a2 2 0 012-2h10a2 2 0 012 2v1m2 13a2 2 0 01-2-2V7m2 13a2 2 0 002-2V9a2 2 0 00-2-2h-2m-4-3H9M7 16h6M7 8h6v4H7V8z" />
          </svg>
          <span className="font-semibold text-white">Market News</span>
        </div>
        <button
          onClick={fetchNews}
          className="p-1 hover:bg-gray-700 rounded transition-colors"
          disabled={loading}
        >
          <svg className={`w-4 h-4 text-gray-400 ${loading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>

      {/* Sentiment Summary */}
      {sentiment && (
        <div className="mb-4 p-3 bg-gray-800 rounded-lg">
          <div className="grid grid-cols-2 gap-3 text-sm">
            <div>
              <div className="text-gray-500 text-xs mb-1">Fear & Greed</div>
              <div className={`font-semibold px-2 py-0.5 rounded inline-block text-sm ${getFearGreedColor(sentiment.fear_greed_index)}`}>
                {sentiment.fear_greed_index} - {sentiment.fear_greed_label}
              </div>
            </div>
            <div>
              <div className="text-gray-500 text-xs mb-1">News Sentiment</div>
              <div className={`font-semibold ${getSentimentColor(sentiment.news_score)}`}>
                {sentiment.news_score > 0 ? '+' : ''}{(sentiment.news_score * 100).toFixed(0)}%
                <span className="text-xs ml-1">
                  {sentiment.news_score > 0.2 ? 'Bullish' : sentiment.news_score < -0.2 ? 'Bearish' : 'Neutral'}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Error State */}
      {error && (
        <div className="text-center text-red-400 py-4 text-sm">
          {error}
        </div>
      )}

      {/* News List */}
      <div className="space-y-2 max-h-80 overflow-y-auto">
        {news.map((item, idx) => (
          <a
            key={idx}
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            className="block p-2 bg-gray-800 rounded hover:bg-gray-750 transition-colors group"
          >
            <div className="flex items-start justify-between gap-2">
              <div className="flex-1 min-w-0">
                <div className="text-sm text-gray-200 group-hover:text-white line-clamp-2">
                  {item.title}
                </div>
                <div className="flex items-center gap-2 mt-1 text-xs text-gray-500">
                  <span>{item.source}</span>
                  <span className="flex items-center gap-1">
                    <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    {formatTimeAgo(item.published_at)}
                  </span>
                </div>
              </div>
              <div className="flex items-center gap-1 flex-shrink-0">
                {item.sentiment > 0.1 ? (
                  <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
                  </svg>
                ) : item.sentiment < -0.1 ? (
                  <svg className="w-4 h-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 17h8m0 0V9m0 8l-8-8-4 4-6-6" />
                  </svg>
                ) : null}
                <svg className="w-3 h-3 text-gray-500 opacity-0 group-hover:opacity-100" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                </svg>
              </div>
            </div>
          </a>
        ))}
        {news.length === 0 && !loading && !error && (
          <div className="text-center text-gray-500 py-4 text-sm">
            No news available. Configure CRYPTONEWS_API_KEY env variable.
          </div>
        )}
        {loading && news.length === 0 && (
          <div className="text-center text-gray-500 py-4 text-sm">
            Loading news...
          </div>
        )}
      </div>
    </div>
  );
}
