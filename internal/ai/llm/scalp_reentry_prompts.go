package llm

// ============ SCALP RE-ENTRY AI PROMPTS ============

// ScalpReentryDecisionPrompt is used to get AI decision for re-entry
const ScalpReentryDecisionPrompt = `You are an AI trading assistant analyzing a scalp re-entry opportunity.

## Current Position
- Symbol: {{.Symbol}}
- Side: {{.Side}}
- Entry Price: ${{.EntryPrice}}
- Current Price: ${{.CurrentPrice}}
- Breakeven: ${{.Breakeven}}
- Distance from BE: {{.DistanceFromBE}}%
- TP Level Just Hit: {{.TPLevel}} ({{.TPPercent}}%)
- Sold Quantity: {{.SoldQty}}
- Potential Re-entry Qty: {{.ReentryQty}} ({{.ReentryPercent}}% of sold)

## Market Context
- 5m Trend: {{.Trend5m}} (strength: {{.TrendStrength5m}})
- 15m Trend: {{.Trend15m}}
- RSI (14): {{.RSI14}}
- Volume Ratio: {{.VolumeRatio}}x average
- ADX: {{.ADX}}
- ATR: {{.ATR}}

## Recent Price Action
- 1m Change: {{.PriceChange1m}}%
- 5m Change: {{.PriceChange5m}}%
- 15m Change: {{.PriceChange15m}}%
- Distance to Support: {{.DistanceToSupport}}%
- Distance to Resistance: {{.DistanceToResistance}}%

## Task
Analyze whether we should execute this re-entry when price returns to breakeven.

Consider:
1. Is the trend still favorable for our position side?
2. Is there enough momentum for another push to the next TP level?
3. What's the risk/reward ratio of re-entering vs staying flat?
4. Are there any warning signs (divergences, exhaustion, reversal patterns)?
5. Volume profile - is it confirming the move?

Respond ONLY with valid JSON (no markdown, no code blocks):
{
  "should_reenter": true or false,
  "confidence": 0.0 to 1.0,
  "recommended_qty_pct": 0.5 to 1.0 (of configured reentry%),
  "reasoning": "brief 1-2 sentence explanation",
  "market_condition": "trending" or "ranging" or "volatile" or "calm",
  "trend_aligned": true or false,
  "risk_level": "low" or "medium" or "high",
  "caution_flags": ["list", "of", "warnings"] or []
}`

// TPTimingPrompt is used to get AI decision for optimal TP timing
const TPTimingPrompt = `You are an AI trading assistant optimizing take profit timing.

## Current Position
- Symbol: {{.Symbol}}
- Side: {{.Side}}
- Entry Price: ${{.EntryPrice}}
- Current Price: ${{.CurrentPrice}}
- Current Profit: {{.CurrentProfitPercent}}%
- Target TP Level: {{.TargetTPLevel}} at {{.TargetTPPercent}}%
- Remaining Quantity: {{.RemainingQty}}
- Accumulated Profit: ${{.AccumulatedProfit}}

## Market Momentum
- RSI: {{.RSI14}}
- MACD Histogram: {{.MACDHist}} ({{.MACDTrend}})
- Volume Trend: {{.VolumeTrend}}
- Trend Strength (ADX): {{.ADX}}

## Price Structure
- Distance to TP: {{.DistanceToTP}}%
- Near Resistance: {{.NearResistance}}
- Resistance Level: ${{.ResistanceLevel}}
- Support Level: ${{.SupportLevel}}

## Task
Decide if we should take profit NOW or wait for the configured TP level.

Consider:
1. Is momentum accelerating or decelerating?
2. Is there strong resistance preventing further upside?
3. Is volume confirming the move?
4. Are there signs of exhaustion or reversal?

Respond ONLY with valid JSON:
{
  "should_take_now": true or false,
  "confidence": 0.0 to 1.0,
  "optimal_percent": 0 to 100 (% to close now, 0 = wait),
  "reasoning": "brief explanation",
  "momentum_status": "accelerating" or "stable" or "decelerating" or "reversing",
  "volume_status": "increasing" or "stable" or "decreasing"
}`

// DynamicSLPrompt is used to calculate optimal dynamic stop loss
const DynamicSLPrompt = `You are an AI risk manager calculating dynamic stop loss.

## Position Details
- Symbol: {{.Symbol}}
- Side: {{.Side}}
- Entry Price: ${{.EntryPrice}}
- Current Price: ${{.CurrentPrice}}
- Accumulated Profit: ${{.AccumulatedProfit}}
- Protected Profit (60%): ${{.ProtectedProfit}}
- Max Allowable Loss (40%): ${{.MaxAllowableLoss}}
- Current SL: ${{.CurrentSL}}

## Market Volatility
- ATR (14): {{.ATR14}}
- ATR %: {{.ATRPercent}}%
- Volatility Regime: {{.VolatilityRegime}}
- Recent Swings: {{.RecentSwingRange}}%

## Support/Resistance
- Nearest Support: ${{.NearestSupport}}
- Nearest Resistance: ${{.NearestResistance}}
- Key Levels: {{.KeyLevels}}

## Task
Calculate the optimal dynamic stop loss that:
1. Protects at least 60% of accumulated profit
2. Allows room for normal volatility
3. Is placed at a logical technical level (support/resistance)
4. Balances protection with not getting stopped out prematurely

Respond ONLY with valid JSON:
{
  "recommended_sl": price as number,
  "protection_percent": 60 to 80 (% of profit protected),
  "reasoning": "brief technical explanation",
  "sl_basis": "atr" or "support" or "swing_low" or "hybrid",
  "volatility_buffer": 0.0 to 2.0 (ATR multiplier used),
  "confidence": 0.0 to 1.0
}`

// FinalExitPrompt is used for the final 20% trailing stop decision
const FinalExitPrompt = `You are an AI trading assistant managing the final position exit.

## Final Position
- Symbol: {{.Symbol}}
- Side: {{.Side}}
- Entry Price: ${{.EntryPrice}}
- Current Price: ${{.CurrentPrice}}
- Peak Price: ${{.PeakPrice}}
- Distance from Peak: {{.DistanceFromPeak}}%
- Final Quantity: {{.FinalQty}} (20% of original)
- Trailing Stop Level: ${{.TrailingSL}} (5% from peak)
- Total Accumulated Profit: ${{.TotalProfit}}

## Market Conditions
- Trend Status: {{.TrendStatus}}
- Momentum: {{.MomentumStatus}}
- RSI: {{.RSI14}}
- Volume: {{.VolumeStatus}}

## Task
Decide if we should exit the final position now or continue holding with trailing stop.

Consider:
1. Has the trend exhausted its momentum?
2. Are there reversal signs?
3. Is the risk/reward of holding further justified?
4. Should we tighten the trailing stop?

Respond ONLY with valid JSON:
{
  "should_exit_now": true or false,
  "exit_reason": "trailing_hit" or "momentum_loss" or "target_reached" or "continue_holding",
  "optimal_exit_price": price or 0 if continue holding,
  "confidence": 0.0 to 1.0,
  "reasoning": "brief explanation",
  "momentum_remaining": 0 to 100 (% of momentum left),
  "tighten_trailing": true or false,
  "new_trailing_percent": 1.0 to 5.0 (if tightening)
}`

// MarketSentimentPrompt analyzes overall market sentiment for a symbol
const MarketSentimentPrompt = `You are an AI market analyst assessing current market sentiment.

## Symbol: {{.Symbol}}

## Technical Indicators
- Price: ${{.CurrentPrice}}
- 24h Change: {{.Change24h}}%
- RSI: {{.RSI14}}
- MACD: {{.MACD}} (Signal: {{.MACDSignal}})
- ADX: {{.ADX}}
- Volume 24h: ${{.Volume24h}}
- Volume vs Average: {{.VolumeRatio}}x

## Trend Analysis
- 5m Trend: {{.Trend5m}}
- 15m Trend: {{.Trend15m}}
- 1h Trend: {{.Trend1h}}
- 4h Trend: {{.Trend4h}}
- Trend Alignment: {{.TrendAlignment}}

## Price Structure
- EMA20: ${{.EMA20}}
- EMA50: ${{.EMA50}}
- BB Upper: ${{.BBUpper}}
- BB Lower: ${{.BBLower}}
- BB Width: {{.BBWidth}}%

## Task
Provide comprehensive sentiment analysis for trading decisions.

Respond ONLY with valid JSON:
{
  "sentiment": "bullish" or "bearish" or "neutral" or "mixed",
  "score": -100 to 100,
  "confidence": 0.0 to 1.0,
  "technical_sentiment": -100 to 100,
  "momentum_sentiment": -100 to 100,
  "volume_sentiment": -100 to 100,
  "trend_sentiment": -100 to 100,
  "market_phase": "accumulation" or "markup" or "distribution" or "markdown",
  "volatility_env": "low" or "normal" or "high" or "extreme",
  "trend_phase": "early" or "mature" or "late" or "reversal",
  "reasoning": "brief analysis"
}`

// ReentryOrchestrationPrompt is used by the main orchestrator to coordinate sub-agents
const ReentryOrchestrationPrompt = `You are the lead AI trading coordinator for scalp re-entry decisions.

## Position Summary
- Symbol: {{.Symbol}}
- Side: {{.Side}}
- Current Profit: {{.CurrentProfitPercent}}%
- TP Level: {{.CurrentTPLevel}}
- Re-entry Status: {{.ReentryStatus}}
- Cycles Completed: {{.CyclesCompleted}}

## Sub-Agent Reports

### Sentiment Agent
{{.SentimentReport}}

### Risk Agent
{{.RiskReport}}

### TP Timing Agent
{{.TPTimingReport}}

### Re-entry Agent
{{.ReentryReport}}

## Task
Synthesize all agent reports and make the final decision.

Respond ONLY with valid JSON:
{
  "primary_action": "wait_for_reentry" or "execute_reentry" or "take_profit" or "close_position" or "hold",
  "action_params": {
    "quantity_percent": 0 to 100,
    "price": target price or 0,
    "stop_loss": SL price or 0
  },
  "confidence": 0.0 to 1.0,
  "reasoning": "synthesized explanation",
  "agent_agreement": 0 to 4 (how many agents agree),
  "risk_assessment": "low" or "medium" or "high",
  "next_check_seconds": 30 to 300
}`

// ============ PROMPT DATA STRUCTURES ============

// ScalpReentryPromptData holds data for re-entry decision prompt
type ScalpReentryPromptData struct {
	Symbol            string
	Side              string
	EntryPrice        float64
	CurrentPrice      float64
	Breakeven         float64
	DistanceFromBE    float64
	TPLevel           int
	TPPercent         float64
	SoldQty           float64
	ReentryQty        float64
	ReentryPercent    float64
	Trend5m           string
	TrendStrength5m   float64
	Trend15m          string
	RSI14             float64
	VolumeRatio       float64
	ADX               float64
	ATR               float64
	PriceChange1m     float64
	PriceChange5m     float64
	PriceChange15m    float64
	DistanceToSupport float64
	DistanceToResistance float64
}

// TPTimingPromptData holds data for TP timing decision prompt
type TPTimingPromptData struct {
	Symbol              string
	Side                string
	EntryPrice          float64
	CurrentPrice        float64
	CurrentProfitPercent float64
	TargetTPLevel       int
	TargetTPPercent     float64
	RemainingQty        float64
	AccumulatedProfit   float64
	RSI14               float64
	MACDHist            float64
	MACDTrend           string
	VolumeTrend         string
	ADX                 float64
	DistanceToTP        float64
	NearResistance      bool
	ResistanceLevel     float64
	SupportLevel        float64
}

// DynamicSLPromptData holds data for dynamic SL calculation prompt
type DynamicSLPromptData struct {
	Symbol            string
	Side              string
	EntryPrice        float64
	CurrentPrice      float64
	AccumulatedProfit float64
	ProtectedProfit   float64
	MaxAllowableLoss  float64
	CurrentSL         float64
	ATR14             float64
	ATRPercent        float64
	VolatilityRegime  string
	RecentSwingRange  float64
	NearestSupport    float64
	NearestResistance float64
	KeyLevels         string
}

// FinalExitPromptData holds data for final exit decision prompt
type FinalExitPromptData struct {
	Symbol           string
	Side             string
	EntryPrice       float64
	CurrentPrice     float64
	PeakPrice        float64
	DistanceFromPeak float64
	FinalQty         float64
	TrailingSL       float64
	TotalProfit      float64
	TrendStatus      string
	MomentumStatus   string
	RSI14            float64
	VolumeStatus     string
}

// MarketSentimentPromptData holds data for sentiment analysis prompt
type MarketSentimentPromptData struct {
	Symbol         string
	CurrentPrice   float64
	Change24h      float64
	RSI14          float64
	MACD           float64
	MACDSignal     float64
	ADX            float64
	Volume24h      float64
	VolumeRatio    float64
	Trend5m        string
	Trend15m       string
	Trend1h        string
	Trend4h        string
	TrendAlignment string
	EMA20          float64
	EMA50          float64
	BBUpper        float64
	BBLower        float64
	BBWidth        float64
}

// OrchestrationPromptData holds data for orchestrator decision prompt
type OrchestrationPromptData struct {
	Symbol              string
	Side                string
	CurrentProfitPercent float64
	CurrentTPLevel      int
	ReentryStatus       string
	CyclesCompleted     int
	SentimentReport     string
	RiskReport          string
	TPTimingReport      string
	ReentryReport       string
}
