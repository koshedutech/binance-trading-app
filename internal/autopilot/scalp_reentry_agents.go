package autopilot

import (
	"binance-trading-bot/internal/ai/llm"
	"context"
	"fmt"
	"sync"
	"time"
)

// ============ MULTI-AGENT ORCHESTRATOR ============

// ScalpReentryOrchestrator coordinates multiple specialized AI agents
type ScalpReentryOrchestrator struct {
	analyzer       *llm.Analyzer
	config         *ScalpReentryConfig

	// Specialized sub-agents
	reentryAgent   *ReentryDecisionAgent
	sentimentAgent *MarketSentimentAgent
	riskAgent      *RiskManagementAgent
	tpAgent        *TPTimingAgent

	// Concurrent execution
	mu             sync.RWMutex
	lastDecision   *OrchestratorDecision
	decisionCache  map[string]*OrchestratorDecision
	cacheExpiry    time.Duration
}

// OrchestratorDecision holds the combined decision from all agents
type OrchestratorDecision struct {
	PrimaryAction  string                 `json:"primary_action"`  // wait_for_reentry, execute_reentry, take_profit, close_position, hold
	ActionParams   map[string]interface{} `json:"action_params"`
	Confidence     float64                `json:"confidence"`
	Reasoning      string                 `json:"reasoning"`
	AgentAgreement int                    `json:"agent_agreement"` // 0-4 agents agree
	RiskAssessment string                 `json:"risk_assessment"` // low, medium, high
	NextCheckSecs  int                    `json:"next_check_secs"`
	Timestamp      time.Time              `json:"timestamp"`

	// Individual agent decisions
	ReentryDecision   *ReentryAIDecision       `json:"reentry_decision,omitempty"`
	SentimentResult   *MarketSentimentResult   `json:"sentiment_result,omitempty"`
	RiskDecision      *DynamicSLDecision       `json:"risk_decision,omitempty"`
	TPDecision        *TPTimingDecision        `json:"tp_decision,omitempty"`
}

// NewScalpReentryOrchestrator creates a new multi-agent orchestrator
func NewScalpReentryOrchestrator(analyzer *llm.Analyzer, config *ScalpReentryConfig) *ScalpReentryOrchestrator {
	o := &ScalpReentryOrchestrator{
		analyzer:      analyzer,
		config:        config,
		decisionCache: make(map[string]*OrchestratorDecision),
		cacheExpiry:   30 * time.Second,
	}

	// Initialize sub-agents
	o.reentryAgent = &ReentryDecisionAgent{analyzer: analyzer, config: config}
	o.sentimentAgent = &MarketSentimentAgent{analyzer: analyzer}
	o.riskAgent = &RiskManagementAgent{analyzer: analyzer, config: config}
	o.tpAgent = &TPTimingAgent{analyzer: analyzer, config: config}

	return o
}

// ProcessPosition runs all agents and returns a coordinated decision
func (o *ScalpReentryOrchestrator) ProcessPosition(pos *GiniePosition, marketData *ScalpReentryMarketData) (*OrchestratorDecision, error) {
	if !o.config.UseMultiAgent {
		// Fall back to single-agent decision
		return o.singleAgentDecision(pos, marketData)
	}

	// Check cache
	cacheKey := fmt.Sprintf("%s-%s-%d", pos.Symbol, pos.Side, pos.ScalpReentry.CurrentCycle)
	if cached := o.getCachedDecision(cacheKey); cached != nil {
		return cached, nil
	}

	// Run agents concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var reentryResult *ReentryAIDecision
	var sentimentResult *MarketSentimentResult
	var riskResult *DynamicSLDecision
	var tpResult *TPTimingDecision
	var mu sync.Mutex
	var errors []error

	// Launch re-entry agent
	wg.Add(1)
	go func() {
		defer wg.Done()
		if o.config.UseAIDecisions {
			result, err := o.reentryAgent.Analyze(ctx, pos, marketData)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("reentry agent: %w", err))
			} else {
				reentryResult = result
			}
		}
	}()

	// Launch sentiment agent
	wg.Add(1)
	go func() {
		defer wg.Done()
		if o.config.EnableSentimentAgent {
			result, err := o.sentimentAgent.Analyze(ctx, pos.Symbol, marketData)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("sentiment agent: %w", err))
			} else {
				sentimentResult = result
			}
		}
	}()

	// Launch risk agent
	wg.Add(1)
	go func() {
		defer wg.Done()
		if o.config.EnableRiskAgent && pos.ScalpReentry.DynamicSLActive {
			result, err := o.riskAgent.Analyze(ctx, pos, marketData)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("risk agent: %w", err))
			} else {
				riskResult = result
			}
		}
	}()

	// Launch TP timing agent
	wg.Add(1)
	go func() {
		defer wg.Done()
		if o.config.EnableTPAgent && o.config.AITPOptimization {
			result, err := o.tpAgent.Analyze(ctx, pos, marketData)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("tp agent: %w", err))
			} else {
				tpResult = result
			}
		}
	}()

	// Wait for all agents
	wg.Wait()

	// Synthesize decisions
	decision := o.synthesizeDecisions(pos, reentryResult, sentimentResult, riskResult, tpResult)
	decision.Timestamp = time.Now()

	// Store individual agent results
	decision.ReentryDecision = reentryResult
	decision.SentimentResult = sentimentResult
	decision.RiskDecision = riskResult
	decision.TPDecision = tpResult

	// Cache the decision
	o.cacheDecision(cacheKey, decision)

	return decision, nil
}

// synthesizeDecisions combines all agent decisions into a final decision
func (o *ScalpReentryOrchestrator) synthesizeDecisions(
	pos *GiniePosition,
	reentry *ReentryAIDecision,
	sentiment *MarketSentimentResult,
	risk *DynamicSLDecision,
	tp *TPTimingDecision,
) *OrchestratorDecision {
	decision := &OrchestratorDecision{
		PrimaryAction:  "hold",
		ActionParams:   make(map[string]interface{}),
		Confidence:     0.5,
		AgentAgreement: 0,
		RiskAssessment: "medium",
		NextCheckSecs:  60,
	}

	sr := pos.ScalpReentry
	agentsAgree := 0
	totalConfidence := 0.0
	confidenceCount := 0

	// Check if waiting for re-entry
	if sr.IsWaitingForReentry() && reentry != nil {
		if reentry.ShouldReenter {
			decision.PrimaryAction = "execute_reentry"
			decision.ActionParams["quantity_percent"] = reentry.RecommendedQtyPct * 100
			agentsAgree++
		}
		totalConfidence += reentry.Confidence
		confidenceCount++
	}

	// Check sentiment alignment
	if sentiment != nil {
		if (pos.Side == "LONG" && sentiment.Score > 0) || (pos.Side == "SHORT" && sentiment.Score < 0) {
			agentsAgree++
		}
		decision.ActionParams["sentiment"] = sentiment.Sentiment
		decision.ActionParams["sentiment_score"] = sentiment.Score
	}

	// Check risk recommendation
	if risk != nil {
		if risk.Confidence > 0.7 {
			agentsAgree++
		}
		decision.ActionParams["recommended_sl"] = risk.RecommendedSL
		totalConfidence += risk.Confidence
		confidenceCount++
	}

	// Check TP timing
	if tp != nil {
		if tp.ShouldTake {
			decision.PrimaryAction = "take_profit"
			decision.ActionParams["optimal_percent"] = tp.OptimalPercent
			agentsAgree++
		}
		totalConfidence += tp.Confidence
		confidenceCount++
	}

	// Calculate final confidence
	if confidenceCount > 0 {
		decision.Confidence = totalConfidence / float64(confidenceCount)
	}

	decision.AgentAgreement = agentsAgree

	// Determine risk level
	if agentsAgree >= 3 {
		decision.RiskAssessment = "low"
	} else if agentsAgree >= 2 {
		decision.RiskAssessment = "medium"
	} else {
		decision.RiskAssessment = "high"
	}

	// Build reasoning
	decision.Reasoning = fmt.Sprintf("%d/4 agents agree. ", agentsAgree)
	if reentry != nil {
		decision.Reasoning += fmt.Sprintf("Reentry: %v (%.0f%%). ", reentry.ShouldReenter, reentry.Confidence*100)
	}
	if sentiment != nil {
		decision.Reasoning += fmt.Sprintf("Sentiment: %s (%.0f). ", sentiment.Sentiment, sentiment.Score)
	}
	if tp != nil && tp.ShouldTake {
		decision.Reasoning += fmt.Sprintf("TP: take now (%.0f%%). ", tp.OptimalPercent)
	}

	return decision
}

// singleAgentDecision falls back to single-agent mode
func (o *ScalpReentryOrchestrator) singleAgentDecision(pos *GiniePosition, marketData *ScalpReentryMarketData) (*OrchestratorDecision, error) {
	decision := &OrchestratorDecision{
		PrimaryAction:  "hold",
		ActionParams:   make(map[string]interface{}),
		Confidence:     0.6,
		RiskAssessment: "medium",
		NextCheckSecs:  30,
		Timestamp:      time.Now(),
	}

	sr := pos.ScalpReentry
	if sr.IsWaitingForReentry() {
		ctx := context.Background()
		reentryResult, err := o.reentryAgent.Analyze(ctx, pos, marketData)
		if err == nil && reentryResult.ShouldReenter {
			decision.PrimaryAction = "execute_reentry"
			decision.Confidence = reentryResult.Confidence
			decision.ActionParams["quantity_percent"] = reentryResult.RecommendedQtyPct * 100
			decision.ReentryDecision = reentryResult
		}
	}

	return decision, nil
}

// getCachedDecision retrieves a cached decision if still valid
func (o *ScalpReentryOrchestrator) getCachedDecision(key string) *OrchestratorDecision {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if cached, ok := o.decisionCache[key]; ok {
		if time.Since(cached.Timestamp) < o.cacheExpiry {
			return cached
		}
	}
	return nil
}

// cacheDecision stores a decision in cache
func (o *ScalpReentryOrchestrator) cacheDecision(key string, decision *OrchestratorDecision) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.decisionCache[key] = decision
}

// ============ SPECIALIZED AGENTS ============

// ReentryDecisionAgent decides whether to execute a re-entry
type ReentryDecisionAgent struct {
	analyzer *llm.Analyzer
	config   *ScalpReentryConfig
}

// Analyze analyzes whether to re-enter
func (a *ReentryDecisionAgent) Analyze(ctx context.Context, pos *GiniePosition, data *ScalpReentryMarketData) (*ReentryAIDecision, error) {
	if a.analyzer == nil {
		return a.heuristicDecision(pos, data), nil
	}

	sr := pos.ScalpReentry
	cycle := sr.GetCurrentCycle()
	if cycle == nil {
		return nil, fmt.Errorf("no active cycle")
	}

	// Build request for LLM analyzer
	req := &llm.ScalpReentryAnalysisRequest{
		Symbol:            pos.Symbol,
		Side:              pos.Side,
		EntryPrice:        pos.EntryPrice,
		CurrentPrice:      data.CurrentPrice,
		Breakeven:         sr.CurrentBreakeven,
		DistanceFromBE:    data.DistanceFromBE,
		TPLevel:           cycle.TPLevel,
		TPPercent:         float64(cycle.TPLevel) * 0.3, // Approximate
		SoldQty:           cycle.SellQuantity,
		ReentryQty:        cycle.ReentryQuantity,
		ReentryPercent:    a.config.ReentryPercent,
		Trend5m:           data.Trend5m,
		TrendStrength5m:   data.TrendStrength,
		Trend15m:          data.Trend15m,
		RSI14:             data.RSI14,
		VolumeRatio:       data.VolumeRatio,
		ADX:               data.ADX,
		ATR:               data.ATR14,
		PriceChange1m:     data.PriceChange1m,
		PriceChange5m:     data.PriceChange5m,
		PriceChange15m:    data.PriceChange15m,
		DistanceToSupport: (data.CurrentPrice - data.NearestSupport) / data.CurrentPrice * 100,
		DistanceToResistance: (data.NearestResistance - data.CurrentPrice) / data.CurrentPrice * 100,
	}

	resp, err := a.analyzer.AnalyzeScalpReentry(req)
	if err != nil {
		return a.heuristicDecision(pos, data), nil
	}

	return &ReentryAIDecision{
		ShouldReenter:     resp.ShouldReenter,
		Confidence:        resp.Confidence,
		RecommendedQtyPct: resp.RecommendedQtyPct,
		Reasoning:         resp.Reasoning,
		MarketCondition:   resp.MarketCondition,
		TrendAlignment:    resp.TrendAligned,
		RiskLevel:         resp.RiskLevel,
		Timestamp:         time.Now(),
	}, nil
}

// heuristicDecision provides a fallback decision without LLM
func (a *ReentryDecisionAgent) heuristicDecision(pos *GiniePosition, data *ScalpReentryMarketData) *ReentryAIDecision {
	decision := &ReentryAIDecision{
		ShouldReenter:     true,
		Confidence:        0.6,
		RecommendedQtyPct: 1.0,
		MarketCondition:   "unknown",
		RiskLevel:         "medium",
		Timestamp:         time.Now(),
	}

	// Check trend alignment
	trendFavorable := false
	if pos.Side == "LONG" && (data.Trend5m == "bullish" || data.Trend15m == "bullish") {
		trendFavorable = true
	} else if pos.Side == "SHORT" && (data.Trend5m == "bearish" || data.Trend15m == "bearish") {
		trendFavorable = true
	}

	if !trendFavorable {
		decision.Confidence = 0.4
		decision.RiskLevel = "high"
		if decision.Confidence < a.config.AIMinConfidence {
			decision.ShouldReenter = false
		}
	}

	decision.TrendAlignment = trendFavorable
	decision.Reasoning = fmt.Sprintf("Heuristic: trend=%v, RSI=%.1f", trendFavorable, data.RSI14)

	return decision
}

// MarketSentimentAgent analyzes market sentiment
type MarketSentimentAgent struct {
	analyzer *llm.Analyzer
}

// Analyze analyzes market sentiment
func (a *MarketSentimentAgent) Analyze(ctx context.Context, symbol string, data *ScalpReentryMarketData) (*MarketSentimentResult, error) {
	result := &MarketSentimentResult{
		Symbol:     symbol,
		Sentiment:  "neutral",
		Score:      0,
		Confidence: 0.5,
		Timestamp:  time.Now(),
	}

	// Heuristic sentiment based on available data
	score := 0.0

	// Trend contribution (-50 to +50)
	if data.Trend5m == "bullish" {
		score += 25
	} else if data.Trend5m == "bearish" {
		score -= 25
	}
	if data.Trend15m == "bullish" {
		score += 25
	} else if data.Trend15m == "bearish" {
		score -= 25
	}

	// RSI contribution (-25 to +25)
	if data.RSI14 > 70 {
		score -= 25 // Overbought
	} else if data.RSI14 < 30 {
		score += 25 // Oversold
	} else if data.RSI14 > 50 {
		score += (data.RSI14 - 50) / 20 * 25
	} else {
		score -= (50 - data.RSI14) / 20 * 25
	}

	result.Score = score
	if score > 25 {
		result.Sentiment = "bullish"
	} else if score < -25 {
		result.Sentiment = "bearish"
	} else if score > 10 || score < -10 {
		result.Sentiment = "mixed"
	}

	result.TechnicalSentiment = score
	result.MomentumSentiment = float64(data.TrendStrength) - 50
	result.VolumeSentiment = (data.VolumeRatio - 1.0) * 50
	result.Confidence = 0.6

	return result, nil
}

// RiskManagementAgent manages dynamic stop loss
type RiskManagementAgent struct {
	analyzer *llm.Analyzer
	config   *ScalpReentryConfig
}

// Analyze analyzes and recommends dynamic SL
func (a *RiskManagementAgent) Analyze(ctx context.Context, pos *GiniePosition, data *ScalpReentryMarketData) (*DynamicSLDecision, error) {
	sr := pos.ScalpReentry

	// Calculate protection levels
	protectedProfit := sr.AccumulatedProfit * (a.config.DynamicSLProtectPct / 100)
	maxAllowableLoss := sr.AccumulatedProfit * (a.config.DynamicSLMaxLossPct / 100)

	// Calculate recommended SL
	var recommendedSL float64
	if pos.Side == "LONG" {
		priceDropAllowed := maxAllowableLoss / sr.RemainingQuantity
		recommendedSL = data.CurrentPrice - priceDropAllowed
		// Never below nearest support
		if recommendedSL < data.NearestSupport {
			recommendedSL = data.NearestSupport
		}
	} else {
		priceRiseAllowed := maxAllowableLoss / sr.RemainingQuantity
		recommendedSL = data.CurrentPrice + priceRiseAllowed
		// Never above nearest resistance
		if recommendedSL > data.NearestResistance {
			recommendedSL = data.NearestResistance
		}
	}

	return &DynamicSLDecision{
		RecommendedSL:    recommendedSL,
		ProtectionLevel:  a.config.DynamicSLProtectPct,
		Reasoning:        fmt.Sprintf("Protecting $%.2f (%.0f%%), max loss $%.2f", protectedProfit, a.config.DynamicSLProtectPct, maxAllowableLoss),
		VolatilityFactor: data.ATR14 / data.CurrentPrice * 100,
		TrendSupport:     data.NearestSupport,
		Confidence:       0.7,
		Timestamp:        time.Now(),
	}, nil
}

// TPTimingAgent optimizes take profit timing
type TPTimingAgent struct {
	analyzer *llm.Analyzer
	config   *ScalpReentryConfig
}

// Analyze analyzes optimal TP timing
func (a *TPTimingAgent) Analyze(ctx context.Context, pos *GiniePosition, data *ScalpReentryMarketData) (*TPTimingDecision, error) {
	sr := pos.ScalpReentry
	nextTPLevel := sr.TPLevelUnlocked + 1

	if nextTPLevel > 3 {
		return nil, fmt.Errorf("no more TP levels")
	}

	tpPercent, _ := a.config.GetTPConfig(nextTPLevel)

	// Calculate current profit percentage
	var currentProfitPct float64
	if pos.Side == "LONG" {
		currentProfitPct = (data.CurrentPrice - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		currentProfitPct = (pos.EntryPrice - data.CurrentPrice) / pos.EntryPrice * 100
	}

	distanceToTP := tpPercent - currentProfitPct

	decision := &TPTimingDecision{
		ShouldTake:     false,
		Confidence:     0.5,
		OptimalPercent: 0,
		Reasoning:      "Hold for configured TP level",
		MomentumStatus: "stable",
		VolumeStatus:   "stable",
		Timestamp:      time.Now(),
	}

	// Check momentum
	if data.VolumeRatio < 0.7 {
		decision.MomentumStatus = "decelerating"
		decision.VolumeStatus = "decreasing"
		// Consider taking early if momentum fading
		if currentProfitPct >= tpPercent*0.7 {
			decision.ShouldTake = true
			decision.OptimalPercent = 50
			decision.Confidence = 0.65
			decision.Reasoning = "Momentum fading, take partial profit early"
		}
	} else if data.VolumeRatio > 1.5 {
		decision.MomentumStatus = "accelerating"
		decision.VolumeStatus = "increasing"
	}

	// Check distance to resistance
	if distanceToTP < 0.1 && data.NearestResistance < data.CurrentPrice*(1+tpPercent/100) {
		decision.ShouldTake = true
		decision.OptimalPercent = 100
		decision.Confidence = 0.75
		decision.Reasoning = "Near resistance, take profit now"
	}

	return decision, nil
}
