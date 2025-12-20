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
	Enabled              bool          `json:"enabled"`
	FearGreedEnabled     bool          `json:"fear_greed_enabled"`
	NewsEnabled          bool          `json:"news_enabled"`
	CryptoPanicAPIKey    string        `json:"cryptopanic_api_key"`
	UpdateInterval       time.Duration `json:"update_interval"`
	SentimentWeight      float64       `json:"sentiment_weight"` // Weight in overall decision
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

	// Fetch news sentiment
	if a.config.NewsEnabled && a.config.CryptoPanicAPIKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			news, err := a.fetchCryptoNews()
			if err == nil && len(news) > 0 {
				newsScore = calculateNewsScore(news)
				sources = append(sources, "crypto_news")
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

// fetchCryptoNews fetches news from CryptoPanic API
func (a *Analyzer) fetchCryptoNews() ([]NewsItem, error) {
	url := fmt.Sprintf("https://cryptopanic.com/api/v1/posts/?auth_token=%s&currencies=BTC,ETH&filter=hot",
		a.config.CryptoPanicAPIKey)

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
		Results []struct {
			Title   string `json:"title"`
			Source  struct {
				Title string `json:"title"`
			} `json:"source"`
			URL         string `json:"url"`
			PublishedAt string `json:"published_at"`
			Votes       struct {
				Positive int `json:"positive"`
				Negative int `json:"negative"`
			} `json:"votes"`
		} `json:"results"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse news: %w", err)
	}

	news := make([]NewsItem, 0, len(result.Results))
	for _, item := range result.Results {
		// Calculate sentiment from votes
		totalVotes := item.Votes.Positive + item.Votes.Negative
		sentiment := 0.0
		if totalVotes > 0 {
			sentiment = float64(item.Votes.Positive-item.Votes.Negative) / float64(totalVotes)
		}

		publishedAt, _ := time.Parse(time.RFC3339, item.PublishedAt)

		news = append(news, NewsItem{
			Title:       item.Title,
			Source:      item.Source.Title,
			URL:         item.URL,
			Sentiment:   sentiment,
			PublishedAt: publishedAt,
		})
	}

	return news, nil
}

// calculateNewsScore calculates aggregate news sentiment
func calculateNewsScore(news []NewsItem) float64 {
	if len(news) == 0 {
		return 0
	}

	// Weight recent news more heavily
	now := time.Now()
	totalWeight := 0.0
	weightedSum := 0.0

	for _, item := range news {
		age := now.Sub(item.PublishedAt).Hours()
		weight := 1.0
		if age < 1 {
			weight = 2.0 // Recent news weighted double
		} else if age < 6 {
			weight = 1.5
		} else if age > 24 {
			weight = 0.5
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

// IsEnabled returns if sentiment analysis is enabled
func (a *Analyzer) IsEnabled() bool {
	return a.config.Enabled
}

// GetWeight returns the sentiment weight for decision making
func (a *Analyzer) GetWeight() float64 {
	return a.config.SentimentWeight
}
