// Package decision provides Redis-first state management for the Position Decision Engine.
// Epic 11: Position Decision Engine - Story 11.1: Redis State Management
package decision

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Redis key prefix for coin state
const (
	// PrefixCoinState is the Redis key pattern for coin state hashes
	// Format: decision:user:{userID}:coin:{symbol}
	PrefixCoinState = "decision:user:%s:coin:%s"

	// DefaultCoinStateTTL is the default TTL for coin state data (5 minutes)
	DefaultCoinStateTTL = 5 * time.Minute
)

// MarketRegime represents the current market condition
type MarketRegime string

const (
	RegimeTrending      MarketRegime = "TRENDING"
	RegimeRanging       MarketRegime = "RANGING"
	RegimeVolatile      MarketRegime = "VOLATILE"
	RegimeConsolidating MarketRegime = "CONSOLIDATING"
)

// TrendDirection represents the trend direction on a timeframe
type TrendDirection string

const (
	TrendBullish TrendDirection = "BULLISH"
	TrendBearish TrendDirection = "BEARISH"
	TrendNeutral TrendDirection = "NEUTRAL"
)

// Decision represents the current decision state for a coin
type Decision string

const (
	DecisionReady   Decision = "READY"
	DecisionBlocked Decision = "BLOCKED"
	DecisionPending Decision = "PENDING"
)

// CoinState represents the complete state for a monitored coin.
// This struct is stored as a Redis hash with field-level updates.
type CoinState struct {
	// Symbol is the trading pair (e.g., BTCUSDT)
	Symbol string `json:"symbol"`

	// Price data
	Price float64 `json:"price"`

	// Market regime classification
	Regime         MarketRegime `json:"regime"`
	ActiveStrategy string       `json:"active_strategy"`

	// Technical indicators
	ADX        float64 `json:"adx"`
	ATR        float64 `json:"atr"`         // Average True Range for volatility classification
	ATRAverage float64 `json:"atr_average"` // Moving average of ATR for comparison
	RSI        float64 `json:"rsi"`
	EMA9       float64 `json:"ema_9"`
	EMA21      float64 `json:"ema_21"`

	// Trend analysis per timeframe
	Trend1H  TrendDirection `json:"trend_1h"`
	Trend15M TrendDirection `json:"trend_15m"`

	// Scoring components (0-100 scale)
	ScoreTechnical int `json:"score_technical"`
	ScoreContext   int `json:"score_context"`
	ScoreLLM       int `json:"score_llm"`
	ScoreHistory   int `json:"score_history"`
	ScoreFinal     int `json:"score_final"`

	// Blocking information
	BlockingReasons []string `json:"blocking_reasons"`

	// Timestamps
	LastUpdated int64 `json:"last_updated"` // Unix timestamp in milliseconds

	// Decision state
	Decision Decision `json:"decision"`
}

// NewCoinState creates a new CoinState with default values
func NewCoinState(symbol string) *CoinState {
	return &CoinState{
		Symbol:          symbol,
		Regime:          RegimeConsolidating,
		Decision:        DecisionPending,
		BlockingReasons: []string{},
		LastUpdated:     time.Now().UnixMilli(),
	}
}

// CoinStateKey generates the Redis key for a coin state.
// SECURITY: userID and symbol are validated to prevent key injection attacks.
func CoinStateKey(userID, symbol string) string {
	// Sanitize inputs to prevent Redis key injection
	userID = sanitizeKeyComponent(userID)
	symbol = sanitizeKeyComponent(symbol)
	return fmt.Sprintf(PrefixCoinState, userID, symbol)
}

// sanitizeKeyComponent removes characters that could be used for Redis key injection.
// This prevents attacks using : * ? [ ] \ or newlines in keys.
func sanitizeKeyComponent(s string) string {
	// Remove any characters that could interfere with Redis key patterns
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Allow alphanumeric, dash, and underscore only
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		}
	}
	return string(result)
}

// CoinStatePattern generates the Redis pattern for scanning all coin states for a user.
// SECURITY: userID is sanitized to prevent key injection attacks.
func CoinStatePattern(userID string) string {
	userID = sanitizeKeyComponent(userID)
	return fmt.Sprintf(PrefixCoinState, userID, "*")
}

// ToMap converts CoinState to a map suitable for Redis HSet.
// All values are converted to strings for Redis hash storage.
func (cs *CoinState) ToMap() map[string]interface{} {
	// Marshal blocking_reasons to JSON string
	blockingJSON, err := json.Marshal(cs.BlockingReasons)
	if err != nil {
		blockingJSON = []byte("[]")
	}

	return map[string]interface{}{
		"symbol":          cs.Symbol,
		"price":           formatFloat(cs.Price),
		"regime":          string(cs.Regime),
		"active_strategy": cs.ActiveStrategy,
		"adx":             formatFloat(cs.ADX),
		"atr":             formatFloat(cs.ATR),
		"atr_average":     formatFloat(cs.ATRAverage),
		"rsi":             formatFloat(cs.RSI),
		"ema_9":           formatFloat(cs.EMA9),
		"ema_21":          formatFloat(cs.EMA21),
		"trend_1h":        string(cs.Trend1H),
		"trend_15m":       string(cs.Trend15M),
		"score_technical": strconv.Itoa(cs.ScoreTechnical),
		"score_context":   strconv.Itoa(cs.ScoreContext),
		"score_llm":       strconv.Itoa(cs.ScoreLLM),
		"score_history":   strconv.Itoa(cs.ScoreHistory),
		"score_final":     strconv.Itoa(cs.ScoreFinal),
		"blocking_reasons": string(blockingJSON),
		"last_updated":    strconv.FormatInt(cs.LastUpdated, 10),
		"decision":        string(cs.Decision),
	}
}

// FromMap parses a Redis hash (map[string]string) into CoinState.
// Handles missing fields gracefully with zero values.
func FromMap(data map[string]string) (*CoinState, error) {
	cs := &CoinState{}

	// String fields
	cs.Symbol = data["symbol"]
	cs.ActiveStrategy = data["active_strategy"]

	// Parse float fields
	cs.Price = parseFloat(data["price"])
	cs.ADX = parseFloat(data["adx"])
	cs.ATR = parseFloat(data["atr"])
	cs.ATRAverage = parseFloat(data["atr_average"])
	cs.RSI = parseFloat(data["rsi"])
	cs.EMA9 = parseFloat(data["ema_9"])
	cs.EMA21 = parseFloat(data["ema_21"])

	// Parse enum fields
	cs.Regime = MarketRegime(data["regime"])
	cs.Trend1H = TrendDirection(data["trend_1h"])
	cs.Trend15M = TrendDirection(data["trend_15m"])
	cs.Decision = Decision(data["decision"])

	// Parse int fields (scores)
	cs.ScoreTechnical = parseInt(data["score_technical"])
	cs.ScoreContext = parseInt(data["score_context"])
	cs.ScoreLLM = parseInt(data["score_llm"])
	cs.ScoreHistory = parseInt(data["score_history"])
	cs.ScoreFinal = parseInt(data["score_final"])

	// Parse timestamp
	cs.LastUpdated = parseInt64(data["last_updated"])

	// Parse blocking_reasons JSON array
	if blockingJSON := data["blocking_reasons"]; blockingJSON != "" {
		var reasons []string
		if err := json.Unmarshal([]byte(blockingJSON), &reasons); err == nil {
			cs.BlockingReasons = reasons
		} else {
			cs.BlockingReasons = []string{}
		}
	} else {
		cs.BlockingReasons = []string{}
	}

	return cs, nil
}

// Validate checks if the CoinState has valid data
func (cs *CoinState) Validate() error {
	if cs.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}

	// Validate regime
	switch cs.Regime {
	case RegimeTrending, RegimeRanging, RegimeVolatile, RegimeConsolidating, "":
		// Valid or empty
	default:
		return fmt.Errorf("invalid regime: %s", cs.Regime)
	}

	// Validate decision
	switch cs.Decision {
	case DecisionReady, DecisionBlocked, DecisionPending, "":
		// Valid or empty
	default:
		return fmt.Errorf("invalid decision: %s", cs.Decision)
	}

	// Validate trend directions
	if err := validateTrend(cs.Trend1H, "trend_1h"); err != nil {
		return err
	}
	if err := validateTrend(cs.Trend15M, "trend_15m"); err != nil {
		return err
	}

	// Validate score ranges (0-100)
	if err := validateScore(cs.ScoreTechnical, "score_technical"); err != nil {
		return err
	}
	if err := validateScore(cs.ScoreContext, "score_context"); err != nil {
		return err
	}
	if err := validateScore(cs.ScoreLLM, "score_llm"); err != nil {
		return err
	}
	if err := validateScore(cs.ScoreHistory, "score_history"); err != nil {
		return err
	}
	if err := validateScore(cs.ScoreFinal, "score_final"); err != nil {
		return err
	}

	return nil
}

// IsBlocked returns true if the coin has blocking reasons
func (cs *CoinState) IsBlocked() bool {
	return len(cs.BlockingReasons) > 0 || cs.Decision == DecisionBlocked
}

// AddBlockingReason adds a reason to the blocking list
func (cs *CoinState) AddBlockingReason(reason string) {
	for _, r := range cs.BlockingReasons {
		if r == reason {
			return // Already exists
		}
	}
	cs.BlockingReasons = append(cs.BlockingReasons, reason)
	cs.Decision = DecisionBlocked
}

// ClearBlockingReasons removes all blocking reasons
func (cs *CoinState) ClearBlockingReasons() {
	cs.BlockingReasons = []string{}
	if cs.Decision == DecisionBlocked {
		cs.Decision = DecisionReady
	}
}

// UpdateTimestamp sets LastUpdated to current time
func (cs *CoinState) UpdateTimestamp() {
	cs.LastUpdated = time.Now().UnixMilli()
}

// Helper functions

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func parseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	i, _ := strconv.Atoi(s)
	return i
}

func parseInt64(s string) int64 {
	if s == "" {
		return 0
	}
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

func validateTrend(trend TrendDirection, field string) error {
	switch trend {
	case TrendBullish, TrendBearish, TrendNeutral, "":
		return nil
	default:
		return fmt.Errorf("invalid %s: %s", field, trend)
	}
}

func validateScore(score int, field string) error {
	if score < 0 || score > 100 {
		return fmt.Errorf("%s must be between 0 and 100, got %d", field, score)
	}
	return nil
}
