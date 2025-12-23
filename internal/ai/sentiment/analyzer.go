package sentiment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SentimentConfig holds sentiment analyzer configuration
type SentimentConfig struct {
	Enabled           bool          `json:"enabled"`
	FearGreedEnabled  bool          `json:"fear_greed_enabled"`
	NewsEnabled       bool          `json:"news_enabled"`
	CryptoNewsAPIKey  string        `json:"cryptonews_api_key"` // CryptoNews API key from cryptonews-api.com
	UpdateInterval    time.Duration `json:"update_interval"`
	SentimentWeight   float64       `json:"sentiment_weight"` // Weight in overall decision
}

// DefaultSentimentConfig returns default configuration
func DefaultSentimentConfig() *SentimentConfig {
	return &SentimentConfig{
		Enabled:          true,
		FearGreedEnabled: true,
		NewsEnabled:      true,
		UpdateInterval:   15 * time.Minute,
		SentimentWeight:  0.2, // 20% weight in decisions
	}
}

// SentimentScore represents aggregated sentiment
type SentimentScore struct {
	Overall        float64   `json:"overall"`         // -1 (extreme fear) to +1 (extreme greed)
	FearGreedIndex int       `json:"fear_greed_index"` // 0-100
	FearGreedLabel string    `json:"fear_greed_label"` // "Extreme Fear", "Fear", "Neutral", "Greed", "Extreme Greed"
	NewsScore      float64   `json:"news_score"`       // -1 to +1
	TrendScore     float64   `json:"trend_score"`      // Market trend sentiment
	UpdatedAt      time.Time `json:"updated_at"`
	Sources        []string  `json:"sources"`
}

// FearGreedResponse from alternative.me API
type FearGreedResponse struct {
	Name string `json:"name"`
	Data []struct {
		Value               string `json:"value"`
		ValueClassification string `json:"value_classification"`
		Timestamp           string `json:"timestamp"`
		TimeUntilUpdate     string `json:"time_until_update"`
	} `json:"data"`
}

// NewsItem represents a news article
type NewsItem struct {
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	URL         string    `json:"url"`
	Sentiment   float64   `json:"sentiment"` // -1 to +1
	PublishedAt time.Time `json:"published_at"`
	Tickers     []string  `json:"tickers,omitempty"`
	Topic       string    `json:"topic,omitempty"`
	IsImportant bool      `json:"is_important,omitempty"`
}

// Analyzer performs market sentiment analysis
type Analyzer struct {
	config      *SentimentConfig
	httpClient  *http.Client
	lastScore   *SentimentScore
	newsCache   []NewsItem
	mu          sync.RWMutex
	stopChan    chan struct{}
}

// NewAnalyzer creates a new sentiment analyzer
func NewAnalyzer(config *SentimentConfig) *Analyzer {
	if config == nil {
		config = DefaultSentimentConfig()
	}
	return &Analyzer{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		newsCache: make([]NewsItem, 0),
		stopChan:  make(chan struct{}),
	}
}

// Start begins the sentiment analysis background updates
func (a *Analyzer) Start() {
	if !a.config.Enabled {
		return
	}

	// Initial fetch
	go a.updateSentiment()

	// Periodic updates
	go func() {
		ticker := time.NewTicker(a.config.UpdateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.updateSentiment()
			case <-a.stopChan:
				return
			}
		}
	}()
}

// Stop stops the sentiment analyzer
func (a *Analyzer) Stop() {
	close(a.stopChan)
}

// GetSentiment returns the current sentiment score
func (a *Analyzer) GetSentiment() *SentimentScore {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastScore
}

// GetTradingBias returns trading bias based on sentiment
func (a *Analyzer) GetTradingBias() (string, float64) {
	score := a.GetSentiment()
	if score == nil {
		return "neutral", 0
	}

	// Convert sentiment to trading bias
	if score.Overall > 0.3 {
		return "bullish", score.Overall
	} else if score.Overall < -0.3 {
		return "bearish", -score.Overall
	}
	return "neutral", 0
}

// ShouldAvoidTrading returns true if sentiment suggests avoiding trades
func (a *Analyzer) ShouldAvoidTrading() (bool, string) {
	score := a.GetSentiment()
	if score == nil {
		return false, ""
	}

	// Extreme fear or greed can be dangerous
	if score.FearGreedIndex <= 10 {
		return true, "Extreme fear - market panic, high risk"
	}
	if score.FearGreedIndex >= 90 {
		return true, "Extreme greed - potential bubble, high risk"
	}

	return false, ""
}

// updateSentiment fetches and updates sentiment data
func (a *Analyzer) updateSentiment() {
	var wg sync.WaitGroup
	var fearGreedIndex int
	var fearGreedLabel string
	var newsScore float64
	sources := make([]string, 0)

	// Fetch Fear & Greed Index
	if a.config.FearGreedEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			idx, label, err := a.fetchFearGreedIndex()
			if err == nil {
				fearGreedIndex = idx
				fearGreedLabel = label
				sources = append(sources, "fear_greed_index")
			}
		}()
	}

	// Fetch news sentiment from CryptoNews API
	if a.config.NewsEnabled && a.config.CryptoNewsAPIKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			news, err := a.fetchCryptoNews()
			if err == nil && len(news) > 0 {
				newsScore = calculateNewsScore(news)
				sources = append(sources, "cryptonews_api")
				a.mu.Lock()
				a.newsCache = news
				a.mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Calculate overall sentiment
	overall := a.calculateOverallSentiment(fearGreedIndex, newsScore)

	score := &SentimentScore{
		Overall:        overall,
		FearGreedIndex: fearGreedIndex,
		FearGreedLabel: fearGreedLabel,
		NewsScore:      newsScore,
		UpdatedAt:      time.Now(),
		Sources:        sources,
	}

	a.mu.Lock()
	a.lastScore = score
	a.mu.Unlock()
}

// fetchFearGreedIndex fetches the Fear & Greed Index
func (a *Analyzer) fetchFearGreedIndex() (int, string, error) {
	resp, err := a.httpClient.Get("https://api.alternative.me/fng/?limit=1")
	if err != nil {
		return 50, "Neutral", fmt.Errorf("failed to fetch fear/greed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 50, "Neutral", fmt.Errorf("failed to read response: %w", err)
	}

	var fgResp FearGreedResponse
	if err := json.Unmarshal(body, &fgResp); err != nil {
		return 50, "Neutral", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(fgResp.Data) == 0 {
		return 50, "Neutral", fmt.Errorf("no data in response")
	}

	var value int
	fmt.Sscanf(fgResp.Data[0].Value, "%d", &value)

	return value, fgResp.Data[0].ValueClassification, nil
}

// fetchCryptoNews fetches news from CryptoNews API (cryptonews-api.com)
func (a *Analyzer) fetchCryptoNews() ([]NewsItem, error) {
	// Fetch news for major trading tickers
	tickers := "BTC,ETH,SOL,XRP,AVAX,DOGE,ADA,DOT,LINK,MATIC,BNB,PEPE,SHIB,ARB,OP"

	// Fetch regular news sorted by rank (importance)
	// Paid plan: fetch up to 50 items for comprehensive coverage
	url := fmt.Sprintf("https://cryptonews-api.com/api/v1?tickers=%s&items=50&sortby=rank&token=%s",
		tickers, a.config.CryptoNewsAPIKey)

	news, err := a.fetchNewsFromURL(url, false)
	if err != nil {
		return nil, err
	}

	// Also fetch trending/breaking news
	trendingURL := fmt.Sprintf("https://cryptonews-api.com/api/v1/category?section=alltickers&items=10&sortby=rank&token=%s",
		a.config.CryptoNewsAPIKey)
	trendingNews, err := a.fetchNewsFromURL(trendingURL, true)
	if err == nil && len(trendingNews) > 0 {
		// Prepend trending news (marked as important)
		news = append(trendingNews, news...)
	}

	return news, nil
}

// fetchNewsFromURL fetches news from a specific CryptoNews API URL
func (a *Analyzer) fetchNewsFromURL(url string, isImportant bool) ([]NewsItem, error) {
	resp, err := a.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch news: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Data []struct {
			Title      string   `json:"title"`
			NewsURL    string   `json:"news_url"`
			SourceName string   `json:"source_name"`
			Date       string   `json:"date"`
			Sentiment  string   `json:"sentiment"` // "Positive", "Negative", "Neutral"
			Text       string   `json:"text"`
			Tickers    []string `json:"tickers"`
			Topics     []string `json:"topics"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse news: %w", err)
	}

	news := make([]NewsItem, 0, len(result.Data))
	for _, item := range result.Data {
		// Convert sentiment string to numeric value
		sentiment := 0.0
		switch item.Sentiment {
		case "Positive":
			sentiment = 0.7
		case "Negative":
			sentiment = -0.7
		case "Neutral":
			sentiment = 0.0
		}

		// Parse date - CryptoNews API uses format like "Sat, 20 Dec 2025 01:15:21 -0500"
		publishedAt, err := time.Parse(time.RFC1123Z, item.Date)
		if err != nil {
			// Try alternative formats
			publishedAt, err = time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", item.Date)
			if err != nil {
				publishedAt = time.Now() // Fallback to now if parsing fails
			}
		}

		topic := ""
		if len(item.Topics) > 0 {
			topic = item.Topics[0]
		}

		news = append(news, NewsItem{
			Title:       item.Title,
			Source:      item.SourceName,
			URL:         item.NewsURL,
			Sentiment:   sentiment,
			PublishedAt: publishedAt,
			Tickers:     item.Tickers,
			Topic:       topic,
			IsImportant: isImportant,
		})
	}

	return news, nil
}

// calculateNewsScore calculates aggregate news sentiment
func calculateNewsScore(news []NewsItem) float64 {
	if len(news) == 0 {
		return 0
	}

	// Weight news by recency and importance
	now := time.Now()
	totalWeight := 0.0
	weightedSum := 0.0

	for _, item := range news {
		age := now.Sub(item.PublishedAt).Hours()
		weight := 1.0

		// Time-based weighting
		if age < 1 {
			weight = 2.5 // Very recent news weighted heavily
		} else if age < 3 {
			weight = 2.0
		} else if age < 6 {
			weight = 1.5
		} else if age > 24 {
			weight = 0.5
		}

		// Important/trending news gets extra weight
		if item.IsImportant {
			weight *= 1.5
		}

		// Regulatory news can have major impact
		if item.Topic == "regulations" || item.Topic == "government" {
			weight *= 1.3
		}

		weightedSum += item.Sentiment * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

// calculateOverallSentiment aggregates all sentiment sources
func (a *Analyzer) calculateOverallSentiment(fearGreedIndex int, newsScore float64) float64 {
	// Convert fear/greed (0-100) to -1 to +1 scale
	fgNormalized := (float64(fearGreedIndex) - 50) / 50

	// If we have multiple sources, weight them
	if newsScore != 0 {
		// 70% fear/greed, 30% news
		return fgNormalized*0.7 + newsScore*0.3
	}

	return fgNormalized
}

// GetRecentNews returns recent news items
func (a *Analyzer) GetRecentNews(limit int) []NewsItem {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if len(a.newsCache) <= limit {
		return a.newsCache
	}
	return a.newsCache[:limit]
}

// GetNewsByTicker returns news filtered by a specific ticker
func (a *Analyzer) GetNewsByTicker(ticker string, limit int) []NewsItem {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]NewsItem, 0)
	for _, item := range a.newsCache {
		for _, t := range item.Tickers {
			if t == ticker {
				result = append(result, item)
				break
			}
		}
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetBreakingNews returns important/trending news items
func (a *Analyzer) GetBreakingNews(limit int) []NewsItem {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]NewsItem, 0)
	for _, item := range a.newsCache {
		if item.IsImportant {
			result = append(result, item)
		}
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetNewsByTopic returns news filtered by topic
func (a *Analyzer) GetNewsByTopic(topic string, limit int) []NewsItem {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]NewsItem, 0)
	for _, item := range a.newsCache {
		if item.Topic == topic {
			result = append(result, item)
		}
		if len(result) >= limit {
			break
		}
	}
	return result
}

// GetSentimentStats returns sentiment distribution statistics
func (a *Analyzer) GetSentimentStats() map[string]int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := map[string]int{
		"bullish": 0,
		"bearish": 0,
		"neutral": 0,
	}

	for _, item := range a.newsCache {
		if item.Sentiment > 0.2 {
			stats["bullish"]++
		} else if item.Sentiment < -0.2 {
			stats["bearish"]++
		} else {
			stats["neutral"]++
		}
	}

	return stats
}

// GetAvailableTickers returns unique tickers from cached news
func (a *Analyzer) GetAvailableTickers() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tickerMap := make(map[string]bool)
	for _, item := range a.newsCache {
		for _, t := range item.Tickers {
			tickerMap[t] = true
		}
	}

	tickers := make([]string, 0, len(tickerMap))
	for t := range tickerMap {
		tickers = append(tickers, t)
	}
	return tickers
}

// IsEnabled returns if sentiment analysis is enabled
func (a *Analyzer) IsEnabled() bool {
	return a.config.Enabled
}

// GetWeight returns the sentiment weight for decision making
func (a *Analyzer) GetWeight() float64 {
	return a.config.SentimentWeight
}
