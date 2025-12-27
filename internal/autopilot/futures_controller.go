package autopilot

import (
	"binance-trading-bot/config"
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/ai/ml"
	"binance-trading-bot/internal/ai/sentiment"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// quantityPrecision defines the decimal precision for each symbol's quantity
// Based on Binance Futures trading rules
var quantityPrecision = map[string]int{
	// Top tier
	"BTCUSDT":   3, // Min qty: 0.001
	"ETHUSDT":   3, // Min qty: 0.001
	"SOLUSDT":   0, // Min qty: 1
	"BNBUSDT":   2, // Min qty: 0.01
	"XRPUSDT":   0, // Min qty: 1
	"DOGEUSDT":  0, // Min qty: 1
	"ADAUSDT":   0, // Min qty: 1
	"AVAXUSDT":  0, // Min qty: 1
	"LINKUSDT":  1, // Min qty: 0.1
	"DOTUSDT":   0, // Min qty: 1
	// High volume
	"LTCUSDT":   2, // Min qty: 0.01
	"ATOMUSDT":  1, // Min qty: 0.1
	"UNIUSDT":   0, // Min qty: 1
	"NEARUSDT":  0, // Min qty: 1
	"MATICUSDT": 0, // Min qty: 1
	"APTUSDT":   1, // Min qty: 0.1
	"ARBUSDT":   0, // Min qty: 1
	"OPUSDT":    0, // Min qty: 1
	"SUIUSDT":   0, // Min qty: 1
	"SEIUSDT":   0, // Min qty: 1
	// Medium-high volume
	"FTMUSDT":   0, // Min qty: 1
	"SANDUSDT":  0, // Min qty: 1
	"MANAUSDT":  0, // Min qty: 1
	"GRTUSDT":   0, // Min qty: 1
	"AAVEUSDT":  1, // Min qty: 0.1
	"MKRUSDT":   3, // Min qty: 0.001
	"SNXUSDT":   0, // Min qty: 1
	"CRVUSDT":   0, // Min qty: 1
	"LDOUSDT":   0, // Min qty: 1
	"RNDRUSDT":  0, // Min qty: 1
	// Popular alts
	"INJUSDT":   1, // Min qty: 0.1
	"TIAUSDT":   0, // Min qty: 1
	"PYTHUSDT":  0, // Min qty: 1
	"JUPUSDT":   0, // Min qty: 1
	"WUSDT":     0, // Min qty: 1
	"ENSUSDT":   1, // Min qty: 0.1
	"BLURUSDT":  0, // Min qty: 1
	"STXUSDT":   0, // Min qty: 1
	"IMXUSDT":   0, // Min qty: 1
	"FILUSDT":   0, // Min qty: 1
	// Additional
	"RUNEUSDT":   0, // Min qty: 1
	"FETUSDT":    0, // Min qty: 1
	"AGIXUSDT":   0, // Min qty: 1
	"OCEANUSDT":  0, // Min qty: 1
	"PENDLEUSDT": 0, // Min qty: 1 (whole numbers)
	"JASMYUSDT":  0, // Min qty: 1
	"GMXUSDT":    2, // Min qty: 0.01
	"WLDUSDT":    0, // Min qty: 1
	"ORDIUSDT":   1, // Min qty: 0.1
	"ONDOUSDT":   0, // Min qty: 1
}

// roundQuantity rounds a quantity to the proper precision for a symbol
func roundQuantity(symbol string, quantity float64) float64 {
	precision := getQuantityPrecision(symbol)
	multiplier := math.Pow(10, float64(precision))
	return math.Floor(quantity*multiplier) / multiplier
}

// ==================== FLOATING POINT COMPARISON HELPERS ====================
// For commercial-grade trading, direct float comparisons can cause issues due to
// floating point precision. These helpers use appropriate tolerances.

// priceGreaterOrEqual returns true if a >= b with tolerance based on symbol's tick size
func priceGreaterOrEqual(symbol string, a, b float64) bool {
	tolerance := getTickSize(symbol)
	return a >= b-tolerance
}

// priceLessOrEqual returns true if a <= b with tolerance based on symbol's tick size
func priceLessOrEqual(symbol string, a, b float64) bool {
	tolerance := getTickSize(symbol)
	return a <= b+tolerance
}

// getTickSize returns the minimum price increment for a symbol
func getTickSize(symbol string) float64 {
	precision := getPricePrecision(symbol)
	return 1.0 / math.Pow(10, float64(precision))
}

// ==================== END FLOATING POINT COMPARISON HELPERS ====================

// pricePrecision defines the decimal precision for each symbol's price
// Based on Binance Futures trading rules (fallback values if API fetch fails)
var pricePrecision = map[string]int{
	"BTCUSDT":   2, // Tick size: 0.10
	"ETHUSDT":   2, // Tick size: 0.01
	"BNBUSDT":   2, // Tick size: 0.01
	"SOLUSDT":   3, // Tick size: 0.001
	"XRPUSDT":   4, // Tick size: 0.0001
	"DOGEUSDT":  5, // Tick size: 0.00001
	"ADAUSDT":   5, // Tick size: 0.00001
	"AVAXUSDT":  3, // Tick size: 0.001
	"LINKUSDT":  3, // Tick size: 0.001
	"DOTUSDT":   3, // Tick size: 0.001
	"LTCUSDT":   2, // Tick size: 0.01
	"ATOMUSDT":  3, // Tick size: 0.001
	"UNIUSDT":   4, // Tick size: 0.0001
	"NEARUSDT":  4, // Tick size: 0.0001
	"CRVUSDT":   3, // Tick size: 0.001
	"SEIUSDT":   5, // Tick size: 0.00001
	"JUPUSDT":   5, // Tick size: 0.00001
	"BLURUSDT":  5, // Tick size: 0.00001
	"BCHUSDT":   2, // Tick size: 0.01
}

// SymbolPrecision holds precision data for a symbol
type SymbolPrecision struct {
	Price    int
	Quantity int
}

// Dynamic precision cache loaded from Binance API
var (
	symbolPrecisionCache   = make(map[string]SymbolPrecision)
	precisionCacheMu       sync.RWMutex
	precisionCacheLoaded   bool
)

// LoadSymbolPrecisions fetches precision data from Binance exchangeInfo API
// This should be called once during initialization
func LoadSymbolPrecisions(client binance.FuturesClient) error {
	info, err := client.GetFuturesExchangeInfo()
	if err != nil {
		return fmt.Errorf("failed to fetch exchange info: %w", err)
	}

	precisionCacheMu.Lock()
	defer precisionCacheMu.Unlock()

	count := 0
	for _, sym := range info.Symbols {
		if sym.Status == "TRADING" {
			symbolPrecisionCache[sym.Symbol] = SymbolPrecision{
				Price:    sym.PricePrecision,
				Quantity: sym.QuantityPrecision,
			}
			count++
		}
	}
	precisionCacheLoaded = true
	return nil
}

// getPricePrecision gets price precision from cache or falls back to hardcoded map
func getPricePrecision(symbol string) int {
	precisionCacheMu.RLock()
	defer precisionCacheMu.RUnlock()

	if prec, ok := symbolPrecisionCache[symbol]; ok {
		return prec.Price
	}
	// Fallback to hardcoded map
	if prec, ok := pricePrecision[symbol]; ok {
		return prec
	}
	// CRITICAL FIX: Default to 6 decimals (most common for low-priced coins)
	// Previous value of 2 was truncating prices like 0.0106 to 0.01 causing order rejections
	// 6 decimals handles coins down to $0.000001 tick size
	return 6
}

// getQuantityPrecision gets quantity precision from cache or falls back to hardcoded map
func getQuantityPrecision(symbol string) int {
	precisionCacheMu.RLock()
	defer precisionCacheMu.RUnlock()

	if prec, ok := symbolPrecisionCache[symbol]; ok {
		return prec.Quantity
	}
	// Fallback to hardcoded map
	if prec, ok := quantityPrecision[symbol]; ok {
		return prec
	}
	return 0 // Default fallback (whole numbers)
}

// roundPrice rounds a price to the proper precision for a symbol (uses standard rounding)
func roundPrice(symbol string, price float64) float64 {
	precision := getPricePrecision(symbol)
	multiplier := math.Pow(10, float64(precision))
	return math.Round(price*multiplier) / multiplier
}

// roundPriceForTP rounds price appropriately for take-profit orders
// For LONG: rounds DOWN (Floor) so TP triggers at or before target
// For SHORT: rounds UP (Ceil) so TP triggers at or before target
func roundPriceForTP(symbol string, price float64, side string) float64 {
	precision := getPricePrecision(symbol)
	multiplier := math.Pow(10, float64(precision))

	if side == "LONG" {
		// For LONG: round DOWN so TP triggers when price reaches/exceeds target
		return math.Floor(price*multiplier) / multiplier
	}
	// For SHORT: round UP so TP triggers when price drops to/below target
	return math.Ceil(price*multiplier) / multiplier
}

// roundPriceForSL rounds price appropriately for stop-loss orders
// For LONG: rounds UP (Ceil) so SL triggers before losses exceed threshold
// For SHORT: rounds DOWN (Floor) so SL triggers before losses exceed threshold
func roundPriceForSL(symbol string, price float64, side string) float64 {
	precision := getPricePrecision(symbol)
	multiplier := math.Pow(10, float64(precision))

	if side == "LONG" {
		// For LONG: round UP so SL triggers earlier (protects capital)
		return math.Ceil(price*multiplier) / multiplier
	}
	// For SHORT: round DOWN so SL triggers earlier (protects capital)
	return math.Floor(price*multiplier) / multiplier
}

// formatSignalBreakdown formats signal breakdown map into a readable string
func (fc *FuturesController) formatSignalBreakdown(breakdown map[string]SignalContribution) string {
	if len(breakdown) == 0 {
		return "no signals"
	}
	result := ""
	for source, signal := range breakdown {
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s:%s(%.0f%%)", source, signal.Direction, signal.Confidence*100)
	}
	return result
}

// FuturesController manages autonomous futures trading
type FuturesController struct {
	config         *config.FuturesAutopilotConfig
	futuresClient  binance.FuturesClient
	circuitBreaker *circuit.CircuitBreaker
	repo           *database.Repository
	logger         *logging.Logger

	// AI components (shared with spot autopilot)
	mlPredictor       *ml.Predictor
	llmAnalyzer       *llm.Analyzer
	sentimentAnalyzer *sentiment.Analyzer

	// Enhanced signal aggregation
	signalAggregator *SignalAggregator
	tradingStyle     TradingStyle
	styleConfig      *TradingStyleConfig

	// Coin classification
	coinClassifier   *CoinClassifier

	// Hedging
	hedgingManager   *HedgingManager

	// Adaptive decision engine (human-like decisions)
	adaptiveEngine   *AdaptiveDecisionEngine

	// Ginie analyzer (advanced multi-mode AI trader)
	ginieAnalyzer    *GinieAnalyzer

	// Ginie autopilot (autonomous multi-mode trading)
	ginieAutopilot   *GinieAutopilot

	// State
	running    bool
	dryRun     bool
	stopChan   chan struct{}
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Tracking
	dailyTrades     int
	dailyPnL        float64
	lastTradeTime   time.Time
	activePositions map[string]*FuturesAutopilotPosition

	// Profit tracking and reinvestment
	totalProfit        float64            // Total profit earned (lifetime)
	profitPool         float64            // Available profit for reinvestment
	totalUSDAllocated  float64            // Current USD allocated to positions
	currentRiskLevel   string             // Current risk level (can be changed dynamically)
	maxUSDAllocation   float64            // Maximum USD to allocate (can be updated)
	profitReinvestPct  float64            // Percentage of profit to reinvest
	profitRiskLevel    string             // Risk level for profit reinvestment

	// Recent decisions tracking for UI
	recentDecisions    []RecentDecisionEvent
	maxRecentDecisions int

	// Trade cooldown to prevent flip-flopping
	lastTradeSide map[string]string    // symbol -> last trade side ("LONG" or "SHORT")
	lastTradeAt   map[string]time.Time // symbol -> last trade time

	// Scalping mode tracking
	scalpingTradesToday int                  // Daily trade counter for scalping mode
	scalpingDayStart    time.Time            // When the current trading day started
	lastCloseTime       map[string]time.Time // symbol -> last close time (for quick re-entry)

	// AI Component Usage Tracking
	mlLastUsed            time.Time
	mlSuccessCount        int
	mlTotalCount          int
	llmLastUsed           time.Time
	llmSuccessCount       int
	llmTotalCount         int
	sentimentLastUsed     time.Time
	sentimentSuccessCount int
	sentimentTotalCount   int
	confluenceAgreements  int
	confluenceTotal       int
}

// RecentDecisionEvent tracks a decision event for display in UI
type RecentDecisionEvent struct {
	Timestamp       time.Time `json:"timestamp"`
	Symbol          string    `json:"symbol"`
	Action          string    `json:"action"`
	Confidence      float64   `json:"confidence"`
	Approved        bool      `json:"approved"`
	Executed        bool      `json:"executed"`
	RejectionReason string    `json:"rejection_reason,omitempty"`
	Quantity        float64   `json:"quantity,omitempty"`
	Leverage        int       `json:"leverage,omitempty"`
	EntryPrice      float64   `json:"entry_price,omitempty"`
}

// PositionEntry tracks individual entries within an averaged position
type PositionEntry struct {
	Price        float64   `json:"price"`
	Quantity     float64   `json:"quantity"`
	Time         time.Time `json:"time"`
	Confidence   float64   `json:"confidence"`
	NewsScore    float64   `json:"news_score"`
}

// FuturesAutopilotPosition tracks an autopilot-managed position
type FuturesAutopilotPosition struct {
	Symbol           string
	Side             string  // "LONG" or "SHORT"
	EntryPrice       float64 // Weighted average entry price
	Quantity         float64 // Total quantity across all entries
	Leverage         int
	TakeProfit       float64 // Legacy TP
	StopLoss         float64 // Current SL (moves to buy price after TP1)
	TakeProfit1      float64 // TP1 level (10% ROI)
	TakeProfit2      float64 // TP2 level (30% ROI)
	TP1SLMoveDone    bool    // Whether SL moved to buy price at TP1
	TrailingActivated bool
	HighestPrice     float64
	LowestPrice      float64
	EntryTime        time.Time
	AIDecisionID     int64
	// Position averaging fields
	EntryCount        int             // Number of entries (1-3)
	EntryHistory      []PositionEntry // History of all entries
	LastAveragingTime time.Time       // Cooldown tracking
	TotalCost         float64         // Total USD cost basis (for weighted avg calculation)
}

// FuturesAutopilotDecision represents an AI decision for futures
type FuturesAutopilotDecision struct {
	Symbol          string
	Action          string  // "open_long", "open_short", "close", "hold"
	Confidence      float64
	Leverage        int
	Quantity        float64
	TakeProfit      float64
	StopLoss        float64
	Reasoning       string
	SignalBreakdown map[string]SignalContribution
	Approved        bool
	RejectionReason string
	// For dynamic SL/TP calculation
	LLMAnalysis     *llm.MarketAnalysis
	Klines          []binance.Kline
	// Adaptive engine position sizing (0.5-1.5 multiplier based on context)
	PositionSizeMultiplier float64
}

// SignalContribution represents a single signal's contribution
type SignalContribution struct {
	Direction  string  // "long", "short", "neutral"
	Confidence float64
	Reasoning  string
}

// NewFuturesController creates a new futures autopilot controller
func NewFuturesController(
	cfg *config.FuturesAutopilotConfig,
	futuresClient binance.FuturesClient,
	circuitBreaker *circuit.CircuitBreaker,
	repo *database.Repository,
	logger *logging.Logger,
) *FuturesController {
	// Debug: Log circuit breaker initialization
	if circuitBreaker != nil {
		logger.Info("FuturesController initializing with circuit breaker",
			"circuit_breaker_enabled", circuitBreaker.IsEnabled())
	} else {
		logger.Warn("FuturesController initializing WITHOUT circuit breaker - this will cause issues!")
	}

	// Load symbol precisions from Binance API for accurate price/quantity rounding
	if err := LoadSymbolPrecisions(futuresClient); err != nil {
		logger.Warn("Failed to load symbol precisions from Binance API, using fallback values", "error", err)
	} else {
		precisionCacheMu.RLock()
		symbolCount := len(symbolPrecisionCache)
		precisionCacheMu.RUnlock()
		logger.Info("Loaded symbol precisions from Binance API", "symbols", symbolCount)
	}

	// Initialize signal aggregator
	signalAgg := NewSignalAggregator(futuresClient, logger)

	// Default to swing trading style
	defaultStyle := StyleSwing
	styleConfig := GetDefaultStyleConfig(defaultStyle)

	// Initialize coin classifier
	coinClassifier := NewCoinClassifier(futuresClient, logger)

	// Initialize hedging manager
	hedgingMgr := NewHedgingManager(cfg, futuresClient, logger)

	// Initialize adaptive decision engine
	adaptiveEng := NewAdaptiveDecisionEngine(signalAgg, futuresClient, logger)
	adaptiveEng.SetStyleConfig(styleConfig)

	// Initialize Ginie analyzer (advanced multi-mode AI trader)
	ginieAn := NewGinieAnalyzer(futuresClient, signalAgg, logger)

	// Initialize Ginie autopilot (autonomous multi-mode trading)
	ginieAuto := NewGinieAutopilot(ginieAn, futuresClient, logger, repo)

	// Load persisted PnL stats
	ginieAuto.LoadPnLStats()

	// Sync existing positions from Binance in background to avoid blocking API startup
	// This is a network operation that can take multiple seconds
	go func() {
		if syncCount, err := ginieAuto.SyncWithExchange(); err != nil {
			logger.Error("Failed to sync Ginie positions from Binance", "error", err)
		} else if syncCount > 0 {
			logger.Info("Synced Ginie positions from Binance", "count", syncCount)
		}
	}()

	return &FuturesController{
		config:             cfg,
		futuresClient:      futuresClient,
		circuitBreaker:     circuitBreaker,
		repo:               repo,
		logger:             logger,
		dryRun:             false, // Default to LIVE mode - will be overridden by LoadSavedSettings()
		stopChan:           make(chan struct{}),
		activePositions:    make(map[string]*FuturesAutopilotPosition),
		currentRiskLevel:   cfg.RiskLevel,
		maxUSDAllocation:   cfg.MaxUSDAllocation,
		profitReinvestPct:  cfg.ProfitReinvestPercent,
		profitRiskLevel:    cfg.ProfitReinvestRiskLevel,
		recentDecisions:    make([]RecentDecisionEvent, 0, 50),
		maxRecentDecisions: 50,
		lastTradeSide:      make(map[string]string),
		lastTradeAt:        make(map[string]time.Time),
		// Scalping mode tracking
		scalpingTradesToday: 0,
		scalpingDayStart:    time.Now().Truncate(24 * time.Hour),
		lastCloseTime:       make(map[string]time.Time),
		// Enhanced signal aggregation
		signalAggregator:    signalAgg,
		tradingStyle:        defaultStyle,
		styleConfig:         styleConfig,
		// Coin classification
		coinClassifier:      coinClassifier,
		// Hedging
		hedgingManager:      hedgingMgr,
		// Adaptive decision engine
		adaptiveEngine:      adaptiveEng,
		// Ginie analyzer
		ginieAnalyzer:       ginieAn,
		// Ginie autopilot
		ginieAutopilot:      ginieAuto,
	}
}

// LoadSavedSettings loads settings from the persistent settings file
// Call this after creating the controller to restore saved settings
func (fc *FuturesController) LoadSavedSettings() {
	sm := GetSettingsManager()
	settings, err := sm.LoadSettings()
	if err != nil {
		fc.logger.Warn("Failed to load saved settings, using defaults", "error", err)
		return
	}

	// CRITICAL: Only hold lock while updating fc fields, NOT for Ginie config or I/O operations
	// This prevents blocking API handlers that need the lock
	fc.mu.Lock()

	// Apply Dynamic SL/TP settings
	fc.config.DynamicSLTPEnabled = settings.DynamicSLTPEnabled
	fc.config.ATRPeriod = settings.ATRPeriod
	fc.config.ATRMultiplierSL = settings.ATRMultiplierSL
	fc.config.ATRMultiplierTP = settings.ATRMultiplierTP
	fc.config.LLMSLTPWeight = settings.LLMSLTPWeight
	fc.config.MinSLPercent = settings.MinSLPercent
	fc.config.MaxSLPercent = settings.MaxSLPercent
	fc.config.MinTPPercent = settings.MinTPPercent
	fc.config.MaxTPPercent = settings.MaxTPPercent

	// Apply Scalping settings
	fc.config.ScalpingModeEnabled = settings.ScalpingModeEnabled
	fc.config.ScalpingMinProfit = settings.ScalpingMinProfit
	fc.config.ScalpingQuickReentry = settings.ScalpingQuickReentry
	fc.config.ScalpingReentryDelaySec = settings.ScalpingReentryDelaySec
	fc.config.ScalpingMaxTradesPerDay = settings.ScalpingMaxTradesPerDay

	// Apply Circuit Breaker settings (if circuit breaker exists)
	if fc.circuitBreaker != nil {
		fc.circuitBreaker.SetEnabled(settings.CircuitBreakerEnabled)
		cbConfig := &circuit.CircuitBreakerConfig{
			MaxLossPerHour:       settings.MaxLossPerHour,
			MaxDailyLoss:         settings.MaxDailyLoss,
			MaxConsecutiveLosses: settings.MaxConsecutiveLosses,
			CooldownMinutes:      settings.CooldownMinutes,
			MaxTradesPerMinute:   settings.MaxTradesPerMinute,
			MaxDailyTrades:       settings.MaxDailyTrades,
		}
		fc.circuitBreaker.UpdateConfig(cbConfig)
	}

	// Apply Autopilot mode settings (risk level, dry run, allocation)
	if settings.RiskLevel != "" {
		fc.currentRiskLevel = settings.RiskLevel
	}
	fc.dryRun = settings.GinieDryRunMode
	if settings.MaxUSDAllocation > 0 {
		fc.maxUSDAllocation = settings.MaxUSDAllocation
	}
	if settings.ProfitReinvestPercent >= 0 {
		fc.profitReinvestPct = settings.ProfitReinvestPercent
	}
	if settings.ProfitReinvestRiskLevel != "" {
		fc.profitRiskLevel = settings.ProfitReinvestRiskLevel
	}

	fc.mu.Unlock()
	// LOCK RELEASED - API handlers can now proceed

	// Apply Ginie Autopilot settings AFTER releasing lock to prevent API timeouts
	if fc.ginieAutopilot != nil {
		ginieConfig := fc.ginieAutopilot.GetConfig()
		ginieConfig.DryRun = settings.GinieDryRunMode
		if settings.GinieRiskLevel != "" {
			ginieConfig.RiskLevel = settings.GinieRiskLevel
		}
		if settings.GinieMaxUSD > 0 {
			ginieConfig.MaxUSDPerPosition = settings.GinieMaxUSD
		}
		if settings.GinieLeverage > 0 {
			ginieConfig.DefaultLeverage = settings.GinieLeverage
		}
		if settings.GinieMinConfidence > 0 {
			ginieConfig.MinConfidenceToTrade = settings.GinieMinConfidence
		}
		if settings.GinieMaxPositions > 0 {
			ginieConfig.MaxPositions = settings.GinieMaxPositions
		}

		// Apply Ginie circuit breaker settings
		ginieConfig.CircuitBreakerEnabled = settings.CircuitBreakerEnabled
		if settings.MaxLossPerHour > 0 {
			ginieConfig.CBMaxLossPerHour = settings.MaxLossPerHour
		}
		if settings.MaxDailyLoss > 0 {
			ginieConfig.CBMaxDailyLoss = settings.MaxDailyLoss
		}
		if settings.MaxConsecutiveLosses > 0 {
			ginieConfig.CBMaxConsecutiveLosses = settings.MaxConsecutiveLosses
		}
		if settings.CooldownMinutes > 0 {
			ginieConfig.CBCooldownMinutes = settings.CooldownMinutes
		}

		// Apply Ginie TP percentages (multi-level take profit allocation)
		if settings.GinieUseSingleTP {
			// Single TP mode: book entire position at single TP
			ginieConfig.TP1Percent = 100.0
			ginieConfig.TP2Percent = 0
			ginieConfig.TP3Percent = 0
			ginieConfig.TP4Percent = 0
		} else {
			// Multi-TP mode: allocate across 4 levels
			ginieConfig.TP1Percent = settings.GinieTP1Percent
			ginieConfig.TP2Percent = settings.GinieTP2Percent
			ginieConfig.TP3Percent = settings.GinieTP3Percent
			ginieConfig.TP4Percent = settings.GinieTP4Percent
		}

		fc.ginieAutopilot.SetConfig(ginieConfig)
		fc.logger.Info("Applied Ginie autopilot settings",
			"dry_run", ginieConfig.DryRun,
			"risk_level", ginieConfig.RiskLevel,
			"max_usd", ginieConfig.MaxUSDPerPosition,
			"leverage", ginieConfig.DefaultLeverage,
			"min_confidence", ginieConfig.MinConfidenceToTrade,
			"max_positions", ginieConfig.MaxPositions,
			"circuit_breaker_enabled", ginieConfig.CircuitBreakerEnabled,
			"cb_max_loss_per_hour", ginieConfig.CBMaxLossPerHour,
			"cb_max_daily_loss", ginieConfig.CBMaxDailyLoss,
			"cb_max_consecutive_losses", ginieConfig.CBMaxConsecutiveLosses,
			"cb_cooldown_minutes", ginieConfig.CBCooldownMinutes,
			"tp_mode", map[bool]string{true: "SINGLE", false: "MULTI"}[settings.GinieUseSingleTP],
			"tp1_percent", ginieConfig.TP1Percent,
			"trend_timeframes", map[string]string{
				"ultrafast": settings.GinieTrendTimeframeUltrafast,
				"scalp":     settings.GinieTrendTimeframeScalp,
				"swing":     settings.GinieTrendTimeframeSwing,
				"position":  settings.GinieTrendTimeframePosition,
			},
			"block_on_divergence", settings.GinieBlockOnDivergence,
			"auto_start", settings.GinieAutoStart)

		// NOTE: Ginie analyzer automatically reads trend timeframes and divergence settings
		// from SettingsManager, so no additional configuration needed here.
		// The settings are applied when GinieAnalyzer calls GetCurrentSettings()

		// NOTE: Auto-start is disabled during LoadSavedSettings to avoid blocking API startup
		// The API handler /api/futures/ginie/toggle will handle auto-start on-demand
		if settings.GinieAutoStart {
			fc.logger.Info("Ginie auto-start is enabled in settings",
				"mode", map[bool]string{true: "PAPER", false: "LIVE"}[ginieConfig.DryRun],
				"note", "auto-start will be triggered via API or manual start")
		}
	}

	fc.logger.Info("Loaded saved autopilot settings",
		"dynamic_sltp_enabled", settings.DynamicSLTPEnabled,
		"scalping_enabled", settings.ScalpingModeEnabled,
		"circuit_breaker_enabled", settings.CircuitBreakerEnabled,
		"risk_level", settings.RiskLevel,
		"dry_run", settings.DryRunMode,
		"max_usd_allocation", settings.MaxUSDAllocation,
		"ginie_auto_start", settings.GinieAutoStart)
}

// SetMLPredictor sets the ML predictor
func (fc *FuturesController) SetMLPredictor(p *ml.Predictor) {
	fc.mlPredictor = p
	if fc.signalAggregator != nil {
		fc.signalAggregator.SetMLPredictor(p)
	}
}

// SetLLMAnalyzer sets the LLM analyzer
func (fc *FuturesController) SetLLMAnalyzer(a *llm.Analyzer) {
	fc.llmAnalyzer = a
	if fc.signalAggregator != nil {
		fc.signalAggregator.SetLLMAnalyzer(a)
	}
	// Pass to Ginie autopilot for adaptive SL/TP
	if fc.ginieAutopilot != nil {
		fc.ginieAutopilot.SetLLMAnalyzer(a)
	}
	// Pass LLM client to Ginie analyzer for AI-based coin selection
	if fc.ginieAnalyzer != nil && a != nil {
		fc.ginieAnalyzer.SetLLMClient(a.GetClient())
	}
}

// HasLLMAnalyzer returns true if an LLM analyzer is configured
func (fc *FuturesController) HasLLMAnalyzer() bool {
	return fc.llmAnalyzer != nil
}

// GetLLMProvider returns the current LLM provider name (e.g., "claude", "openai", "deepseek")
// Returns empty string if no LLM analyzer is configured
func (fc *FuturesController) GetLLMProvider() string {
	if fc.llmAnalyzer == nil {
		return ""
	}
	client := fc.llmAnalyzer.GetClient()
	if client == nil {
		return ""
	}
	return string(client.GetProvider())
}

// SetSentimentAnalyzer sets the sentiment analyzer
func (fc *FuturesController) SetSentimentAnalyzer(a *sentiment.Analyzer) {
	fc.sentimentAnalyzer = a
	if fc.signalAggregator != nil {
		fc.signalAggregator.SetSentimentAnalyzer(a)
	}
}

// SetTradingStyle sets the trading style
func (fc *FuturesController) SetTradingStyle(style TradingStyle) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.tradingStyle = style
	fc.styleConfig = GetDefaultStyleConfig(style)

	// Update adaptive engine with new style config
	if fc.adaptiveEngine != nil {
		fc.adaptiveEngine.SetStyleConfig(fc.styleConfig)
	}

	fc.logger.Info("Trading style updated",
		"style", string(style),
		"leverage", fc.styleConfig.DefaultLeverage,
		"min_confidence", fc.styleConfig.MinConfidence)
}

// GetTradingStyle returns the current trading style
func (fc *FuturesController) GetTradingStyle() TradingStyle {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.tradingStyle
}

// GetStyleConfig returns the current style configuration
func (fc *FuturesController) GetStyleConfig() *TradingStyleConfig {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.styleConfig
}

// GetCoinClassifier returns the coin classifier
func (fc *FuturesController) GetCoinClassifier() *CoinClassifier {
	return fc.coinClassifier
}

// GetHedgingManager returns the hedging manager
func (fc *FuturesController) GetHedgingManager() *HedgingManager {
	return fc.hedgingManager
}

// GetAdaptiveEngine returns the adaptive decision engine
func (fc *FuturesController) GetAdaptiveEngine() *AdaptiveDecisionEngine {
	return fc.adaptiveEngine
}

// GetGinieAnalyzer returns the Ginie AI analyzer
func (fc *FuturesController) GetGinieAnalyzer() *GinieAnalyzer {
	return fc.ginieAnalyzer
}

// GetGinieAutopilot returns the Ginie autonomous trading system
func (fc *FuturesController) GetGinieAutopilot() *GinieAutopilot {
	return fc.ginieAutopilot
}

// StartGinieAutopilot starts the Ginie autonomous trading
func (fc *FuturesController) StartGinieAutopilot() error {
	if fc.ginieAutopilot == nil {
		return fmt.Errorf("Ginie autopilot not initialized")
	}
	return fc.ginieAutopilot.Start()
}

// StopGinieAutopilot stops the Ginie autonomous trading
func (fc *FuturesController) StopGinieAutopilot() error {
	if fc.ginieAutopilot == nil {
		return fmt.Errorf("Ginie autopilot not initialized")
	}
	return fc.ginieAutopilot.Stop()
}

// GetDryRun returns the current dry run mode status
func (fc *FuturesController) GetDryRun() bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.dryRun
}

// GetFuturesClient returns the actual futures client being used by the controller
func (fc *FuturesController) GetFuturesClient() binance.FuturesClient {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.futuresClient
}

// SetDryRun sets dry run mode and propagates to Ginie
func (fc *FuturesController) SetDryRun(enabled bool) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// If mode is changing, reset all statistics and positions
	if fc.dryRun != enabled {
		fc.logger.Info("Switching futures autopilot mode",
			"from", map[bool]string{true: "PAPER", false: "LIVE"}[fc.dryRun],
			"to", map[bool]string{true: "PAPER", false: "LIVE"}[enabled])

		// Clear all tracked positions (they belong to the old mode)
		fc.activePositions = make(map[string]*FuturesAutopilotPosition)

		// Reset allocation counter (positions are cleared, so allocation should be 0)
		fc.totalUSDAllocated = 0

		// Reset daily statistics
		fc.dailyTrades = 0
		fc.dailyPnL = 0

		fc.logger.Info("Futures autopilot statistics reset for new mode")

		// Also update Ginie autopilot's dry run config
		if fc.ginieAutopilot != nil {
			ginieConfig := fc.ginieAutopilot.GetConfig()
			if ginieConfig != nil && ginieConfig.DryRun != enabled {
				ginieConfig.DryRun = enabled
				fc.ginieAutopilot.SetConfig(ginieConfig)
				fc.logger.Info("Ginie autopilot dry_run updated", "dry_run", enabled)

				// Clear Ginie positions when switching modes
				fc.ginieAutopilot.ClearPositions()
			}
		}
	}

	fc.dryRun = enabled
}

// SetFuturesClient updates the futures client (used when switching between paper/live modes)
func (fc *FuturesController) SetFuturesClient(client binance.FuturesClient) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.futuresClient = client
	fc.logger.Info("Futures controller client updated")

	// Also update Ginie's client
	if fc.ginieAutopilot != nil {
		fc.ginieAutopilot.SetFuturesClient(client)
		fc.logger.Info("Ginie autopilot client updated")
	}
}

// IsDryRun returns current dry run status
func (fc *FuturesController) IsDryRun() bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.dryRun
}

// Start begins the futures autopilot loop
func (fc *FuturesController) Start() error {
	fc.mu.Lock()
	if fc.running {
		fc.mu.Unlock()
		return fmt.Errorf("futures autopilot already running")
	}
	fc.running = true
	fc.stopChan = make(chan struct{})
	fc.mu.Unlock()

	// Sync with actual Binance positions on startup
	fc.syncWithActualPositions()

	// Start coin classifier
	if fc.coinClassifier != nil {
		fc.coinClassifier.Start()
	}

	fc.wg.Add(1)
	go fc.runLoop()

	fc.logger.Info("Futures autopilot started",
		"dry_run", fc.dryRun,
		"risk_level", fc.config.RiskLevel,
		"leverage", fc.config.DefaultLeverage,
		"trading_style", string(fc.tradingStyle))

	return nil
}

// Stop stops the futures autopilot
func (fc *FuturesController) Stop() {
	fc.mu.Lock()
	if !fc.running {
		fc.mu.Unlock()
		return
	}
	fc.running = false
	close(fc.stopChan)
	fc.mu.Unlock()

	// Stop coin classifier
	if fc.coinClassifier != nil {
		fc.coinClassifier.Stop()
	}

	fc.wg.Wait()
	fc.logger.Info("Futures autopilot stopped")
}

// IsRunning returns whether autopilot is running
func (fc *FuturesController) IsRunning() bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.running
}

// GetStatus returns the current autopilot status
func (fc *FuturesController) GetStatus() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	positions := make([]map[string]interface{}, 0)
	for symbol, pos := range fc.activePositions {
		positions = append(positions, map[string]interface{}{
			"symbol":       symbol,
			"side":         pos.Side,
			"entry_price":  pos.EntryPrice,
			"quantity":     pos.Quantity,
			"leverage":     pos.Leverage,
			"take_profit":  pos.TakeProfit,
			"stop_loss":    pos.StopLoss,
			"entry_time":   pos.EntryTime,
			"entry_count":  pos.EntryCount,       // Position averaging: number of entries
			"total_cost":   pos.TotalCost,        // Position averaging: total cost basis
		})
	}

	return map[string]interface{}{
		"running":          fc.running,
		"dry_run":          fc.dryRun,
		"risk_level":       fc.currentRiskLevel,
		"daily_trades":     fc.dailyTrades,
		"daily_pnl":        fc.dailyPnL,
		"active_positions": positions,
		// Profit tracking
		"total_profit":        fc.totalProfit,
		"profit_pool":         fc.profitPool,
		"total_usd_allocated": fc.totalUSDAllocated,
		"max_usd_allocation":  fc.maxUSDAllocation,
		// Profit reinvestment settings
		"profit_reinvest_percent":    fc.profitReinvestPct,
		"profit_reinvest_risk_level": fc.profitRiskLevel,
		"config": map[string]interface{}{
			"default_leverage":     fc.config.DefaultLeverage,
			"max_leverage":         fc.config.MaxLeverage,
			"margin_type":          fc.config.MarginType,
			"position_mode":        fc.config.PositionMode,
			"take_profit":          fc.config.TakeProfitPercent,
			"stop_loss":            fc.config.StopLossPercent,
			"min_confidence":       fc.config.MinConfidence,
			"require_confluence":   fc.config.RequireConfluence,
			"allow_shorts":         fc.config.AllowShorts,
			"trailing_stop":        fc.config.TrailingStopEnabled,
		},
	}
}

// RecordDecision records a decision event for the UI
func (fc *FuturesController) RecordDecision(decision *FuturesAutopilotDecision, executed bool, currentPrice float64) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	event := RecentDecisionEvent{
		Timestamp:       time.Now(),
		Symbol:          decision.Symbol,
		Action:          decision.Action,
		Confidence:      decision.Confidence,
		Approved:        decision.Approved,
		Executed:        executed,
		RejectionReason: decision.RejectionReason,
		Quantity:        decision.Quantity,
		Leverage:        decision.Leverage,
		EntryPrice:      currentPrice,
	}

	// Add to front of slice
	fc.recentDecisions = append([]RecentDecisionEvent{event}, fc.recentDecisions...)

	// Trim to max size
	if len(fc.recentDecisions) > fc.maxRecentDecisions {
		fc.recentDecisions = fc.recentDecisions[:fc.maxRecentDecisions]
	}
}

// GetRecentDecisions returns recent decision events for UI display
func (fc *FuturesController) GetRecentDecisions() []RecentDecisionEvent {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]RecentDecisionEvent, len(fc.recentDecisions))
	copy(result, fc.recentDecisions)
	return result
}

// GetActivePositionSymbols returns a list of symbols that the autopilot is tracking
func (fc *FuturesController) GetActivePositionSymbols() []string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	symbols := make([]string, 0, len(fc.activePositions))
	for symbol := range fc.activePositions {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// SetRiskLevel changes the current risk level dynamically
func (fc *FuturesController) SetRiskLevel(level string) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Validate risk level
	validLevels := map[string]bool{"conservative": true, "moderate": true, "aggressive": true}
	if !validLevels[level] {
		return fmt.Errorf("invalid risk level: %s (must be conservative, moderate, or aggressive)", level)
	}

	oldLevel := fc.currentRiskLevel
	fc.currentRiskLevel = level

	// Adjust parameters based on risk level
	switch level {
	case "conservative":
		fc.config.MinConfidence = 0.8
		fc.config.RequireConfluence = 3
		fc.config.DefaultLeverage = 3
		fc.config.TakeProfitPercent = 1.5
		fc.config.StopLossPercent = 0.5
	case "moderate":
		fc.config.MinConfidence = 0.65
		fc.config.RequireConfluence = 2
		fc.config.DefaultLeverage = 5
		fc.config.TakeProfitPercent = 2.0
		fc.config.StopLossPercent = 1.0
	case "aggressive":
		fc.config.MinConfidence = 0.35 // Lower threshold to allow more trades
		fc.config.RequireConfluence = 1
		fc.config.DefaultLeverage = 10
		fc.config.TakeProfitPercent = 3.0
		fc.config.StopLossPercent = 1.5
	}

	fc.logger.Info("Risk level changed",
		"from", oldLevel,
		"to", level,
		"leverage", fc.config.DefaultLeverage,
		"min_confidence", fc.config.MinConfidence)

	return nil
}

// GetRiskLevel returns the current risk level
func (fc *FuturesController) GetRiskLevel() string {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.currentRiskLevel
}

// SetMaxUSDAllocation sets the maximum USD allocation for trading
func (fc *FuturesController) SetMaxUSDAllocation(amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("max USD allocation must be positive")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldAmount := fc.maxUSDAllocation
	fc.maxUSDAllocation = amount

	fc.logger.Info("Max USD allocation updated",
		"from", oldAmount,
		"to", amount)

	return nil
}

// GetMaxUSDAllocation returns the current max USD allocation
func (fc *FuturesController) GetMaxUSDAllocation() float64 {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.maxUSDAllocation
}

// SetMaxPositionSize sets the maximum position size percentage
func (fc *FuturesController) SetMaxPositionSize(percent float64) error {
	if percent <= 0 || percent > 100 {
		return fmt.Errorf("max position size must be between 0 and 100 percent")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldPercent := fc.config.MaxPositionSize
	fc.config.MaxPositionSize = percent

	fc.logger.Info("Max position size updated",
		"from", oldPercent,
		"to", percent)

	return nil
}

// GetMaxPositionSize returns the current max position size percentage
func (fc *FuturesController) GetMaxPositionSize() float64 {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.config.MaxPositionSize
}

// RecalculateAllocation recalculates totalUSDAllocated based on actual active positions
// This is useful when positions are closed manually or there's a sync issue
func (fc *FuturesController) RecalculateAllocation() float64 {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldAllocation := fc.totalUSDAllocated

	// Calculate allocation from active positions
	var totalAllocated float64
	for _, pos := range fc.activePositions {
		positionValue := pos.EntryPrice * pos.Quantity / float64(pos.Leverage)
		totalAllocated += positionValue
	}

	fc.totalUSDAllocated = totalAllocated

	fc.logger.Info("Allocation recalculated",
		"old_allocation", oldAllocation,
		"new_allocation", totalAllocated,
		"active_positions", len(fc.activePositions))

	return totalAllocated
}

// ResetAllocation resets the allocation counter to zero and clears internal positions
func (fc *FuturesController) ResetAllocation() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldAllocation := fc.totalUSDAllocated
	oldPositions := len(fc.activePositions)

	fc.totalUSDAllocated = 0
	fc.activePositions = make(map[string]*FuturesAutopilotPosition)
	fc.dailyTrades = 0

	fc.logger.Info("Allocation and positions reset",
		"old_allocation", oldAllocation,
		"old_positions", oldPositions,
		"new_allocation", 0)
}

// SetProfitReinvestSettings configures profit reinvestment
func (fc *FuturesController) SetProfitReinvestSettings(percent float64, riskLevel string) error {
	if percent < 0 || percent > 100 {
		return fmt.Errorf("profit reinvest percent must be between 0 and 100")
	}

	validLevels := map[string]bool{"conservative": true, "moderate": true, "aggressive": true}
	if !validLevels[riskLevel] {
		return fmt.Errorf("invalid risk level: %s", riskLevel)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.profitReinvestPct = percent
	fc.profitRiskLevel = riskLevel

	fc.logger.Info("Profit reinvestment settings updated",
		"percent", percent,
		"risk_level", riskLevel)

	return nil
}

// SetTPSLPercent sets custom take profit and stop loss percentages
func (fc *FuturesController) SetTPSLPercent(takeProfit, stopLoss float64) error {
	if takeProfit <= 0 || takeProfit > 100 {
		return fmt.Errorf("take profit percent must be between 0 and 100")
	}
	if stopLoss <= 0 || stopLoss > 100 {
		return fmt.Errorf("stop loss percent must be between 0 and 100")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldTP := fc.config.TakeProfitPercent
	oldSL := fc.config.StopLossPercent

	fc.config.TakeProfitPercent = takeProfit
	fc.config.StopLossPercent = stopLoss

	fc.logger.Info("TP/SL percentages updated",
		"old_tp", oldTP,
		"new_tp", takeProfit,
		"old_sl", oldSL,
		"new_sl", stopLoss)

	return nil
}

// GetTPSLPercent returns the current take profit and stop loss percentages
func (fc *FuturesController) GetTPSLPercent() (takeProfit, stopLoss float64) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.config.TakeProfitPercent, fc.config.StopLossPercent
}

// GetDynamicSLTPConfig returns the dynamic SL/TP configuration
func (fc *FuturesController) GetDynamicSLTPConfig() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return map[string]interface{}{
		"enabled":           fc.config.DynamicSLTPEnabled,
		"atr_period":        fc.config.ATRPeriod,
		"atr_multiplier_sl": fc.config.ATRMultiplierSL,
		"atr_multiplier_tp": fc.config.ATRMultiplierTP,
		"llm_weight":        fc.config.LLMSLTPWeight,
		"min_sl_percent":    fc.config.MinSLPercent,
		"max_sl_percent":    fc.config.MaxSLPercent,
		"min_tp_percent":    fc.config.MinTPPercent,
		"max_tp_percent":    fc.config.MaxTPPercent,
	}
}

// SetDynamicSLTPConfig updates the dynamic SL/TP configuration
func (fc *FuturesController) SetDynamicSLTPConfig(
	enabled bool,
	atrPeriod int,
	atrMultiplierSL float64,
	atrMultiplierTP float64,
	llmWeight float64,
	minSL float64,
	maxSL float64,
	minTP float64,
	maxTP float64,
) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.config.DynamicSLTPEnabled = enabled
	if atrPeriod > 0 {
		fc.config.ATRPeriod = atrPeriod
	}
	if atrMultiplierSL > 0 {
		fc.config.ATRMultiplierSL = atrMultiplierSL
	}
	if atrMultiplierTP > 0 {
		fc.config.ATRMultiplierTP = atrMultiplierTP
	}
	if llmWeight >= 0 && llmWeight <= 1 {
		fc.config.LLMSLTPWeight = llmWeight
	}
	if minSL > 0 {
		fc.config.MinSLPercent = minSL
	}
	if maxSL > 0 {
		fc.config.MaxSLPercent = maxSL
	}
	if minTP > 0 {
		fc.config.MinTPPercent = minTP
	}
	if maxTP > 0 {
		fc.config.MaxTPPercent = maxTP
	}

	fc.logger.Info("Dynamic SL/TP config updated",
		"enabled", enabled,
		"atr_period", fc.config.ATRPeriod,
		"atr_multiplier_sl", fc.config.ATRMultiplierSL,
		"atr_multiplier_tp", fc.config.ATRMultiplierTP,
		"llm_weight", fc.config.LLMSLTPWeight)

	// Persist settings to file
	go func() {
		sm := GetSettingsManager()
		if err := sm.UpdateDynamicSLTP(
			enabled,
			atrPeriod,
			atrMultiplierSL,
			atrMultiplierTP,
			llmWeight,
			minSL,
			maxSL,
			minTP,
			maxTP,
		); err != nil {
			fc.logger.Warn("Failed to persist dynamic SL/TP settings", "error", err)
		}
	}()
}

// GetScalpingConfig returns the scalping mode configuration
func (fc *FuturesController) GetScalpingConfig() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return map[string]interface{}{
		"enabled":             fc.config.ScalpingModeEnabled,
		"min_profit":          fc.config.ScalpingMinProfit,
		"quick_reentry":       fc.config.ScalpingQuickReentry,
		"reentry_delay_sec":   fc.config.ScalpingReentryDelaySec,
		"max_trades_per_day":  fc.config.ScalpingMaxTradesPerDay,
		"today_scalp_trades":  fc.scalpingTradesToday,
	}
}

// SetScalpingConfig updates the scalping mode configuration
func (fc *FuturesController) SetScalpingConfig(
	enabled bool,
	minProfit float64,
	quickReentry bool,
	reentryDelaySec int,
	maxTradesPerDay int,
) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.config.ScalpingModeEnabled = enabled
	if minProfit > 0 {
		fc.config.ScalpingMinProfit = minProfit
	}
	fc.config.ScalpingQuickReentry = quickReentry
	if reentryDelaySec > 0 {
		fc.config.ScalpingReentryDelaySec = reentryDelaySec
	}
	if maxTradesPerDay >= 0 {
		fc.config.ScalpingMaxTradesPerDay = maxTradesPerDay
	}

	fc.logger.Info("Scalping config updated",
		"enabled", enabled,
		"min_profit", fc.config.ScalpingMinProfit,
		"quick_reentry", quickReentry,
		"reentry_delay_sec", fc.config.ScalpingReentryDelaySec,
		"max_trades_per_day", fc.config.ScalpingMaxTradesPerDay)

	// Persist settings to file
	go func() {
		sm := GetSettingsManager()
		if err := sm.UpdateScalping(
			enabled,
			minProfit,
			quickReentry,
			reentryDelaySec,
			maxTradesPerDay,
		); err != nil {
			fc.logger.Warn("Failed to persist scalping settings", "error", err)
		}
	}()
}

// SetDefaultLeverage sets custom default leverage for new positions
func (fc *FuturesController) SetDefaultLeverage(leverage int) error {
	if leverage < 1 || leverage > fc.config.MaxLeverage {
		return fmt.Errorf("leverage must be between 1 and %d", fc.config.MaxLeverage)
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldLeverage := fc.config.DefaultLeverage
	fc.config.DefaultLeverage = leverage

	fc.logger.Info("Default leverage updated",
		"old_leverage", oldLeverage,
		"new_leverage", leverage)

	return nil
}

// GetDefaultLeverage returns the current default leverage
func (fc *FuturesController) GetDefaultLeverage() int {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.config.DefaultLeverage
}

// SetMinConfidence sets the minimum confidence threshold for trades
func (fc *FuturesController) SetMinConfidence(confidence float64) error {
	if confidence < 0 || confidence > 1 {
		return fmt.Errorf("min confidence must be between 0 and 1 (e.g., 0.65 for 65%%)")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldConfidence := fc.config.MinConfidence
	fc.config.MinConfidence = confidence

	fc.logger.Info("Min confidence updated",
		"old_confidence", oldConfidence,
		"new_confidence", confidence)

	return nil
}

// GetMinConfidence returns the current minimum confidence threshold
func (fc *FuturesController) GetMinConfidence() float64 {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.config.MinConfidence
}

// SetConfluence sets the required confluence (number of agreeing signals)
// 0 = any signal is enough, 1+ = require that many signals to agree
func (fc *FuturesController) SetConfluence(confluence int) error {
	if confluence < 0 || confluence > 5 {
		return fmt.Errorf("confluence must be between 0 and 5")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	oldConfluence := fc.config.RequireConfluence
	fc.config.RequireConfluence = confluence

	fc.logger.Info("Confluence requirement updated",
		"old_confluence", oldConfluence,
		"new_confluence", confluence)

	return nil
}

// GetConfluence returns the current confluence requirement
func (fc *FuturesController) GetConfluence() int {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.config.RequireConfluence
}

// GetProfitStats returns profit tracking statistics
func (fc *FuturesController) GetProfitStats() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	return map[string]interface{}{
		"total_profit":               fc.totalProfit,
		"profit_pool":                fc.profitPool,
		"total_usd_allocated":        fc.totalUSDAllocated,
		"max_usd_allocation":         fc.maxUSDAllocation,
		"profit_reinvest_percent":    fc.profitReinvestPct,
		"profit_reinvest_risk_level": fc.profitRiskLevel,
		"daily_pnl":                  fc.dailyPnL,
	}
}

// addToProfit adds realized profit and updates the profit pool for reinvestment
func (fc *FuturesController) addToProfit(profit float64) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if profit > 0 {
		fc.totalProfit += profit
		// Add configured percentage of profit to reinvestment pool
		reinvestAmount := profit * (fc.profitReinvestPct / 100)
		fc.profitPool += reinvestAmount

		fc.logger.Info("Profit added to reinvestment pool",
			"profit", profit,
			"reinvest_amount", reinvestAmount,
			"pool_total", fc.profitPool)
	}
}

// getAvailableAllocation returns the available USD for new positions
// This includes base allocation and profit pool for aggressive trading
// It also checks actual Binance account balance to prevent insufficient margin errors
func (fc *FuturesController) getAvailableAllocation() (baseAmount float64, profitAmount float64, useAggressiveRisk bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	// Calculate remaining base allocation from internal tracking
	baseAmount = fc.maxUSDAllocation - fc.totalUSDAllocated
	if baseAmount < 0 {
		baseAmount = 0
	}

	// Check actual available balance from Binance (if not in dry run)
	if !fc.dryRun && fc.futuresClient != nil {
		accountInfo, err := fc.futuresClient.GetFuturesAccountInfo()
		if err == nil && accountInfo != nil {
			// Find USDT available balance
			var actualAvailable float64
			for _, asset := range accountInfo.Assets {
				if asset.Asset == "USDT" {
					actualAvailable = asset.AvailableBalance
					break
				}
			}

			// Use the minimum of internal tracking and actual balance
			// Leave 10% buffer for price fluctuations and fees
			actualWithBuffer := actualAvailable * 0.9
			if actualWithBuffer < baseAmount {
				baseAmount = actualWithBuffer
			}
		}
	}

	// Profit pool is available for aggressive trading
	profitAmount = fc.profitPool
	useAggressiveRisk = profitAmount > 0

	return baseAmount, profitAmount, useAggressiveRisk
}

// syncWithActualPositions syncs internal state with actual Binance positions
// Call this on startup and periodically to prevent drift
func (fc *FuturesController) syncWithActualPositions() {
	if fc.dryRun || fc.futuresClient == nil {
		return
	}

	positions, err := fc.futuresClient.GetPositions()
	if err != nil {
		fc.logger.Error("Failed to sync positions", "error", err.Error())
		return
	}

	// Track positions that need TP/SL orders placed (do this outside the lock)
	type positionNeedingTPSL struct {
		symbol     string
		side       string
		takeProfit float64
		stopLoss   float64
	}
	var positionsNeedingTPSL []positionNeedingTPSL

	fc.mu.Lock()

	// Calculate total allocated from actual positions and sync activePositions
	var actualAllocated float64
	activeCount := 0

	// Track which symbols have actual positions
	actualPositionSymbols := make(map[string]bool)

	for _, pos := range positions {
		if pos.PositionAmt == 0 {
			continue
		}

		leverage := pos.Leverage
		if leverage == 0 {
			leverage = 1
		}

		// Calculate position value (margin used)
		absAmt := pos.PositionAmt
		if absAmt < 0 {
			absAmt = -absAmt
		}
		positionValue := (absAmt * pos.EntryPrice) / float64(leverage)
		actualAllocated += positionValue
		activeCount++
		actualPositionSymbols[pos.Symbol] = true

		// Sync activePositions map - add or update position tracking
		// This ensures autopilot knows about existing positions after restart
		if _, exists := fc.activePositions[pos.Symbol]; !exists {
			// Determine side based on position amount
			side := "LONG"
			if pos.PositionAmt < 0 {
				side = "SHORT"
			}

			// Calculate proper TP/SL based on ROE for synced positions
			// ROE% = Price Change% × Leverage, so Price Change% = ROE% / Leverage
			tpPricePercent := fc.config.TakeProfitPercent / float64(leverage)
			slPricePercent := fc.config.StopLossPercent / float64(leverage)

			var takeProfit, stopLoss float64
			if side == "LONG" {
				takeProfit = pos.EntryPrice * (1 + tpPricePercent/100)
				stopLoss = pos.EntryPrice * (1 - slPricePercent/100)
			} else {
				takeProfit = pos.EntryPrice * (1 - tpPricePercent/100)
				stopLoss = pos.EntryPrice * (1 + slPricePercent/100)
			}

			// Calculate TP1 and TP2 for synced position
			var tp1, tp2 float64
			if side == "LONG" {
				tp1 = pos.EntryPrice * (1 + fc.config.TakeProfitPercent1/100)
				tp2 = pos.EntryPrice * (1 + fc.config.TakeProfitPercent2/100)
			} else {
				tp1 = pos.EntryPrice * (1 - fc.config.TakeProfitPercent1/100)
				tp2 = pos.EntryPrice * (1 - fc.config.TakeProfitPercent2/100)
			}

			fc.activePositions[pos.Symbol] = &FuturesAutopilotPosition{
				Symbol:       pos.Symbol,
				Side:         side,
				EntryPrice:   pos.EntryPrice,
				Quantity:     absAmt,
				Leverage:     leverage,
				TakeProfit:   roundPrice(pos.Symbol, takeProfit),
				TakeProfit1:  roundPrice(pos.Symbol, tp1),
				TakeProfit2:  roundPrice(pos.Symbol, tp2),
				StopLoss:     roundPrice(pos.Symbol, stopLoss),
				HighestPrice: pos.EntryPrice,
				LowestPrice:  pos.EntryPrice,
				EntryTime:    time.Now(), // Approximate - we don't know actual entry time
			}
			fc.logger.Info("Synced existing position to activePositions",
				"symbol", pos.Symbol,
				"side", side,
				"quantity", absAmt,
				"entry_price", pos.EntryPrice,
				"leverage", leverage)

			// Mark this position as needing TP/SL orders
			positionsNeedingTPSL = append(positionsNeedingTPSL, positionNeedingTPSL{
				symbol:     pos.Symbol,
				side:       side,
				takeProfit: roundPrice(pos.Symbol, takeProfit),
				stopLoss:   roundPrice(pos.Symbol, stopLoss),
			})
		} else {
			// Update existing tracked position with actual values
			fc.activePositions[pos.Symbol].Quantity = absAmt
			fc.activePositions[pos.Symbol].EntryPrice = pos.EntryPrice
		}
	}

	// Remove positions from activePositions that no longer exist on Binance
	// CRITICAL: Also cancel any orphaned algo orders to prevent them from opening new positions
	for symbol := range fc.activePositions {
		if !actualPositionSymbols[symbol] {
			pos := fc.activePositions[symbol]
			fc.logger.Info("Removing closed position from activePositions",
				"symbol", symbol)

			// Cancel all algo orders for this symbol to prevent orphan TP/SL from opening new positions
			if err := fc.futuresClient.CancelAllAlgoOrders(symbol); err != nil {
				fc.logger.Warn("Failed to cancel orphaned algo orders during sync",
					"symbol", symbol,
					"error", err.Error())
			} else {
				fc.logger.Info("Cancelled orphaned algo orders for closed position", "symbol", symbol)
			}

			// Record trade result to circuit breaker based on TP/SL levels
			// Since position was closed externally (Binance TP/SL), we need to determine outcome
			if fc.circuitBreaker != nil && pos != nil {
				currentPrice, err := fc.futuresClient.GetFuturesCurrentPrice(symbol)
				if err == nil && currentPrice > 0 {
					var pnlPercent float64
					if pos.Side == "LONG" {
						pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
					} else {
						pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
					}

					// Record to circuit breaker - this will reset consecutive losses if profitable
					fc.circuitBreaker.RecordTrade(pnlPercent)
					fc.logger.Info("Recorded externally closed position to circuit breaker",
						"symbol", symbol,
						"side", pos.Side,
						"entry_price", pos.EntryPrice,
						"current_price", currentPrice,
						"pnl_percent", pnlPercent)
				}
			}

			delete(fc.activePositions, symbol)
		}
	}

	// Update internal tracking if there's significant drift
	if fc.totalUSDAllocated != actualAllocated {
		fc.logger.Info("Syncing allocation with actual positions",
			"internal_allocation", fc.totalUSDAllocated,
			"actual_allocation", actualAllocated,
			"active_positions", activeCount)
		fc.totalUSDAllocated = actualAllocated
	}

	// Release lock before making API calls for TP/SL orders
	fc.mu.Unlock()

	// Place TP/SL orders for newly synced positions that don't have them
	for _, pos := range positionsNeedingTPSL {
		// Check if TP/SL orders already exist for this position
		existingAlgoOrders, err := fc.futuresClient.GetOpenAlgoOrders(pos.symbol)
		if err != nil {
			fc.logger.Warn("Failed to check existing algo orders for synced position",
				"symbol", pos.symbol,
				"error", err.Error())
			// Continue anyway - better to have duplicate orders than none
		}

		// Check if TP and SL orders already exist
		hasTP := false
		hasSL := false
		for _, order := range existingAlgoOrders {
			if order.OrderType == string(binance.FuturesOrderTypeTakeProfitMarket) {
				hasTP = true
			}
			if order.OrderType == string(binance.FuturesOrderTypeStopMarket) {
				hasSL = true
			}
		}

		// Only place orders if they don't exist
		if !hasTP || !hasSL {
			fc.logger.Info("Placing missing TP/SL orders for synced position",
				"symbol", pos.symbol,
				"side", pos.side,
				"has_tp", hasTP,
				"has_sl", hasSL,
				"tp_price", pos.takeProfit,
				"sl_price", pos.stopLoss)

			// Create a decision struct for placeTPSLOrders
			decision := &FuturesAutopilotDecision{
				TakeProfit: pos.takeProfit,
				StopLoss:   pos.stopLoss,
			}

			// Determine position side
			positionSide := binance.PositionSideLong
			if pos.side == "SHORT" {
				positionSide = binance.PositionSideShort
			}

			// Place only the missing orders
			fc.placeTPSLOrdersSelective(pos.symbol, decision, positionSide, !hasTP, !hasSL)
		} else {
			fc.logger.Debug("TP/SL orders already exist for synced position",
				"symbol", pos.symbol)
		}
	}
}

// runLoop is the main autopilot decision loop
func (fc *FuturesController) runLoop() {
	defer fc.wg.Done()

	// DEPRECATED: Old FuturesController trading loop is disabled.
	// All Futures trading is now handled by GinieAutopilot.
	// This loop only maintains position sync for compatibility.
	fc.logger.Info("FuturesController loop started (trading disabled - use Ginie instead)")

	// Sync positions every minute to prevent allocation drift
	syncTicker := time.NewTicker(60 * time.Second)
	defer syncTicker.Stop()

	// Reset daily counters at midnight
	go fc.resetDailyCounters()

	for {
		select {
		case <-fc.stopChan:
			return
		case <-syncTicker.C:
			fc.syncWithActualPositions()
		// NOTE: evaluateMarket() is intentionally disabled.
		// Old AI trading has been moved to SpotController for Spot trading.
		// Futures trading is now exclusively handled by GinieAutopilot.
		}
	}
}

// resetDailyCounters resets daily trading counters at midnight
func (fc *FuturesController) resetDailyCounters() {
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		time.Sleep(time.Until(next))

		fc.mu.Lock()
		fc.dailyTrades = 0
		fc.dailyPnL = 0
		fc.mu.Unlock()

		fc.logger.Info("Futures autopilot daily counters reset")
	}
}

// evaluateMarket evaluates trading opportunities
func (fc *FuturesController) evaluateMarket() {
	fc.logger.Info("Futures autopilot evaluating market", "symbols_count", len(fc.config.AllowedSymbols))

	// Check circuit breaker
	if fc.circuitBreaker != nil {
		canTrade, reason := fc.circuitBreaker.CanTrade()
		if !canTrade {
			fc.logger.Info("Futures autopilot BLOCKED by circuit breaker", "reason", reason)
			return
		}
	}

	// Check daily limits
	fc.mu.RLock()
	if fc.dailyTrades >= fc.config.MaxDailyTrades {
		fc.mu.RUnlock()
		fc.logger.Info("Futures autopilot BLOCKED: daily trade limit reached",
			"trades_today", fc.dailyTrades,
			"max_daily_trades", fc.config.MaxDailyTrades)
		return
	}
	if fc.dailyPnL <= -fc.config.MaxDailyLoss {
		fc.mu.RUnlock()
		fc.logger.Warn("Futures autopilot BLOCKED: daily loss limit reached",
			"daily_pnl", fmt.Sprintf("%.2f", fc.dailyPnL),
			"max_daily_loss", fc.config.MaxDailyLoss)
		return
	}
	fc.mu.RUnlock()

	// Get symbols to evaluate
	symbols := fc.config.AllowedSymbols
	if len(symbols) == 0 {
		// Default popular trading symbols for futures
		symbols = []string{
			"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT", "XRPUSDT",
			"DOGEUSDT", "ADAUSDT", "AVAXUSDT", "LINKUSDT", "MATICUSDT",
			"DOTUSDT", "LTCUSDT", "ATOMUSDT", "UNIUSDT", "NEARUSDT",
		}
	}

	// Evaluate each symbol
	for _, symbol := range symbols {
		select {
		case <-fc.stopChan:
			return
		default:
			fc.evaluateSymbol(symbol)
		}
	}

	// Monitor existing positions
	fc.monitorPositions()
}

// evaluateSymbol evaluates a single symbol for trading opportunities
func (fc *FuturesController) evaluateSymbol(symbol string) {
	// Check if we already have a position
	fc.mu.RLock()
	pos, hasPosition := fc.activePositions[symbol]
	fc.mu.RUnlock()

	if hasPosition {
		// If averaging is enabled, evaluate whether to add to position
		if fc.config.AveragingEnabled {
			fc.evaluateAveraging(symbol, pos)
		}
		return // Position managed by monitorPositions
	}

	// Get current price
	price, err := fc.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		fc.logger.Error("Failed to get price", "symbol", symbol, "error", err.Error())
		return
	}

	// Get klines for AI analysis
	klines, err := fc.futuresClient.GetFuturesKlines(symbol, "1m", 100)
	if err != nil {
		fc.logger.Error("Failed to get klines", "symbol", symbol, "error", err.Error())
		return
	}

	// Collect signals from AI components
	decision := fc.collectSignals(symbol, price, klines)

	// Log the decision with appropriate level based on outcome
	if decision.Approved && (decision.Action == "open_long" || decision.Action == "open_short") {
		fc.logger.Info("Futures autopilot decision APPROVED",
			"symbol", symbol,
			"action", decision.Action,
			"confidence", fmt.Sprintf("%.2f", decision.Confidence),
			"leverage", decision.Leverage,
			"quantity", decision.Quantity,
			"take_profit", decision.TakeProfit,
			"stop_loss", decision.StopLoss)
		executed := fc.executeDecision(symbol, decision, price)
		// Record decision for UI
		fc.RecordDecision(decision, executed, price)
	} else if decision.Action != "hold" || decision.RejectionReason != "" {
		// Log rejected decisions that had potential signals
		fc.logger.Debug("Futures autopilot decision REJECTED",
			"symbol", symbol,
			"action", decision.Action,
			"confidence", fmt.Sprintf("%.2f", decision.Confidence),
			"approved", decision.Approved,
			"rejection_reason", decision.RejectionReason,
			"signal_breakdown", fc.formatSignalBreakdown(decision.SignalBreakdown))
		// Record rejected decision for UI
		fc.RecordDecision(decision, false, price)
	}
}

// collectSignals collects signals from all AI sources using the enhanced signal aggregator
func (fc *FuturesController) collectSignals(symbol string, currentPrice float64, klines []binance.Kline) *FuturesAutopilotDecision {
	decision := &FuturesAutopilotDecision{
		Symbol:          symbol,
		Action:          "hold",
		SignalBreakdown: make(map[string]SignalContribution),
		Klines:          klines, // Store for dynamic SL/TP calculation
	}

	// Use the enhanced signal aggregator to collect from all 6 sources
	if fc.signalAggregator == nil {
		fc.logger.Warn("Signal aggregator not initialized", "symbol", symbol)
		decision.RejectionReason = "Signal aggregator not initialized"
		return decision
	}

	// Get current trading style for signal weighting
	fc.mu.RLock()
	tradingStyle := fc.tradingStyle
	fc.mu.RUnlock()

	// Collect signals from all sources (ML, LLM, Sentiment, Pattern, Technical, MTF)
	signals, llmAnalysis := fc.signalAggregator.CollectAllSignals(symbol, currentPrice, klines, tradingStyle)

	// Track AI component usage timestamps based on signals received
	now := time.Now()
	fc.mu.Lock()
	for _, sig := range signals {
		switch sig.Source {
		case "ml_predictor":
			fc.mlLastUsed = now
			fc.mlTotalCount++
		case "llm_analyzer":
			fc.llmLastUsed = now
			fc.llmTotalCount++
		case "sentiment":
			fc.sentimentLastUsed = now
			fc.sentimentTotalCount++
		}
	}
	fc.mu.Unlock()

	// Store LLM analysis for dynamic SL/TP calculation
	if llmAnalysis != nil {
		decision.LLMAnalysis = llmAnalysis
	}

	// Convert enhanced signals to legacy format for backward compatibility
	decision.SignalBreakdown = ConvertToSignalBreakdown(signals)

	// Count signals by direction
	longSignals := 0
	shortSignals := 0
	signalCount := len(signals)

	for _, s := range signals {
		switch s.Direction {
		case "long":
			longSignals++
		case "short":
			shortSignals++
		}
	}

	// Track confluence (when 2+ signals agree on direction)
	fc.mu.Lock()
	fc.confluenceTotal++
	if longSignals >= 2 || shortSignals >= 2 {
		fc.confluenceAgreements++
	}
	fc.mu.Unlock()

	// Use aggregator to determine action with weighted decision
	minConfidence := fc.config.MinConfidence
	confluenceRequired := fc.config.RequireConfluence

	// If using style-specific config, apply those settings
	if fc.styleConfig != nil {
		// Use the more conservative of the two
		if fc.styleConfig.MinConfidence > minConfidence {
			minConfidence = fc.styleConfig.MinConfidence
		}
		if fc.styleConfig.RequiredConfluence > confluenceRequired {
			confluenceRequired = fc.styleConfig.RequiredConfluence
		}
	}

	action, confidence, approved, reason := fc.signalAggregator.AggregateDecision(
		signals,
		tradingStyle,
		minConfidence,
		confluenceRequired,
	)

	decision.Action = action
	decision.Confidence = confidence
	decision.Approved = approved

	// Check if shorts are allowed
	if action == "open_short" && !fc.config.AllowShorts {
		decision.Approved = false
		decision.RejectionReason = "Short signals detected but shorts are disabled"
	} else if !approved && reason != "" {
		decision.RejectionReason = reason
	}

	// === ADAPTIVE ENGINE INTEGRATION ===
	// Use human-like decision making with market context
	if fc.adaptiveEngine != nil && decision.Approved {
		// Collect market context for adaptive decision
		fc.mu.RLock()
		recentWins := 0
		recentLosses := 0
		for _, d := range fc.recentDecisions {
			if d.Executed && d.Approved {
				recentWins++ // Count executed trades
			} else if d.Executed && !d.Approved {
				recentLosses++
			}
		}
		activePositionCount := len(fc.activePositions)
		fc.mu.RUnlock()

		// Get max positions from style config or use default
		maxPositions := 5 // Default max concurrent positions
		if fc.styleConfig != nil && fc.styleConfig.MaxEntries > 0 {
			maxPositions = fc.styleConfig.MaxEntries
		}

		marketCtx := &MarketContext{
			RecentWins:    recentWins,
			RecentLosses:  recentLosses,
			OpenPositions: activePositionCount,
			MaxPositions:  maxPositions,
		}

		// Get profit stats for context
		stats := fc.GetProfitStats()
		if totalProfit, ok := stats["total_profit"].(float64); ok {
			marketCtx.DailyPnL = totalProfit
		}

		// Set market context before making decision
		fc.adaptiveEngine.SetMarketContext(marketCtx)

		// Make adaptive decision (no existing position for new trades)
		adaptiveDecision := fc.adaptiveEngine.MakeDecision(symbol, currentPrice, klines, nil)

		// Apply adaptive decision adjustments
		if !adaptiveDecision.Approved {
			decision.Approved = false
			decision.RejectionReason = fmt.Sprintf("Adaptive engine: %s", adaptiveDecision.PrimaryReason)
			fc.logger.Info("Adaptive engine rejected decision",
				"symbol", symbol,
				"reason", adaptiveDecision.PrimaryReason,
				"risk_warnings", adaptiveDecision.RiskWarnings)
		} else {
			// Apply position size multiplier from adaptive engine
			decision.PositionSizeMultiplier = adaptiveDecision.PositionSizeMultiplier

			// Log adaptive reasoning
			fc.logger.Info("Adaptive engine enhanced decision",
				"symbol", symbol,
				"action", adaptiveDecision.Action,
				"size_multiplier", fmt.Sprintf("%.2f", adaptiveDecision.PositionSizeMultiplier),
				"primary_reason", adaptiveDecision.PrimaryReason,
				"context_factors", adaptiveDecision.ContextFactors)
		}
	}
	// === END ADAPTIVE ENGINE INTEGRATION ===

	fc.logger.Info("Enhanced signal collection complete",
		"symbol", symbol,
		"trading_style", string(tradingStyle),
		"total_signals", signalCount,
		"long_signals", longSignals,
		"short_signals", shortSignals,
		"confidence", fmt.Sprintf("%.2f", confidence),
		"action", action,
		"approved", approved)

	// Check for flip-flop: prevent reversing direction within 2 hours of last trade
	// Exception: scalping quick re-entry bypasses this for same-direction trades
	if decision.Approved {
		newSide := "LONG"
		if decision.Action == "open_short" {
			newSide = "SHORT"
		}

		fc.mu.RLock()
		lastSide, hasLastTrade := fc.lastTradeSide[symbol]
		lastTime := fc.lastTradeAt[symbol]
		lastCloseTime, hasCloseTime := fc.lastCloseTime[symbol]
		fc.mu.RUnlock()

		// Check if this is a scalping quick re-entry situation
		isQuickReentry := false
		if fc.config.ScalpingModeEnabled && fc.config.ScalpingQuickReentry && hasCloseTime {
			timeSinceClose := time.Since(lastCloseTime)
			reentryDelay := time.Duration(fc.config.ScalpingReentryDelaySec) * time.Second
			if timeSinceClose >= reentryDelay && timeSinceClose < 5*time.Minute {
				// Same direction re-entry within 5 minutes of closing is considered quick re-entry
				if !hasLastTrade || lastSide == newSide {
					isQuickReentry = true
					fc.logger.Info("Scalping quick re-entry detected",
						"symbol", symbol,
						"side", newSide,
						"time_since_close", timeSinceClose.String())
				}
			}
		}

		cooldownDuration := 2 * time.Hour // 2 hour cooldown for reversing direction

		// Apply flip-flop cooldown only if not a quick re-entry
		if hasLastTrade && lastSide != newSide && !isQuickReentry {
			timeSinceLastTrade := time.Since(lastTime)
			if timeSinceLastTrade < cooldownDuration {
				decision.Approved = false
				decision.RejectionReason = fmt.Sprintf("Flip-flop cooldown: Last trade was %s only %.0f min ago (need %.0f min)",
					lastSide, timeSinceLastTrade.Minutes(), cooldownDuration.Minutes())
				fc.logger.Warn("Trade rejected: flip-flop cooldown",
					"symbol", symbol,
					"new_side", newSide,
					"last_side", lastSide,
					"time_since_last", timeSinceLastTrade.String())
			}
		}
	}

	// Calculate position parameters
	if decision.Approved {
		fc.logger.Info("Decision pre-approved, calculating position",
			"symbol", symbol,
			"action", decision.Action,
			"confidence", decision.Confidence)

		decision.Leverage = fc.config.DefaultLeverage
		leverage := float64(fc.config.DefaultLeverage)

		// Determine position side for SL/TP calculation
		side := "LONG"
		if decision.Action == "open_short" {
			side = "SHORT"
		}

		// Check if dynamic SL/TP is enabled
		if fc.config.DynamicSLTPEnabled && len(decision.Klines) >= 20 {
			// Use dynamic SL/TP based on ATR + LLM
			dynamicConfig := &DynamicSLTPConfig{
				Enabled:         true,
				ATRPeriod:       fc.config.ATRPeriod,
				ATRMultiplierSL: fc.config.ATRMultiplierSL,
				ATRMultiplierTP: fc.config.ATRMultiplierTP,
				MinSLPercent:    fc.config.MinSLPercent,
				MaxSLPercent:    fc.config.MaxSLPercent,
				MinTPPercent:    fc.config.MinTPPercent,
				MaxTPPercent:    fc.config.MaxTPPercent,
				LLMWeight:       fc.config.LLMSLTPWeight,
			}

			// Apply defaults if not configured
			if dynamicConfig.ATRPeriod == 0 {
				dynamicConfig.ATRPeriod = 14
			}
			if dynamicConfig.ATRMultiplierSL == 0 {
				dynamicConfig.ATRMultiplierSL = 1.5
			}
			if dynamicConfig.ATRMultiplierTP == 0 {
				dynamicConfig.ATRMultiplierTP = 2.0
			}
			if dynamicConfig.MinSLPercent == 0 {
				dynamicConfig.MinSLPercent = 0.3
			}
			if dynamicConfig.MaxSLPercent == 0 {
				dynamicConfig.MaxSLPercent = 3.0
			}
			if dynamicConfig.MinTPPercent == 0 {
				dynamicConfig.MinTPPercent = 0.5
			}
			if dynamicConfig.MaxTPPercent == 0 {
				dynamicConfig.MaxTPPercent = 5.0
			}

			sltpResult := CalculateDynamicSLTP(symbol, currentPrice, decision.Klines, side, decision.LLMAnalysis, dynamicConfig)

			// Apply leverage adjustment: the SL/TP percents from dynamic calc are price-based
			// We need to adjust them for the leverage effect
			decision.TakeProfit = sltpResult.TakeProfitPrice
			decision.StopLoss = sltpResult.StopLossPrice

			fc.logger.Info("Dynamic SL/TP calculated",
				"symbol", symbol,
				"side", side,
				"atr_value", sltpResult.ATRValue,
				"atr_percent", sltpResult.ATRPercent,
				"sl_percent", sltpResult.StopLossPercent,
				"tp_percent", sltpResult.TakeProfitPercent,
				"used_llm", sltpResult.UsedLLM,
				"reasoning", sltpResult.Reasoning,
				"tp_price", decision.TakeProfit,
				"sl_price", decision.StopLoss)
		} else {
			// Use fixed percentage based on ROE (Return on Equity)
			// ROE = (Price Change %) * Leverage
			// So Price Change % = ROE % / Leverage
			// With 5x leverage: 10% ROE target = 2% price move
			tpPricePercent := fc.config.TakeProfitPercent / leverage // Convert ROE% to price%
			slPricePercent := fc.config.StopLossPercent / leverage   // Convert ROE% to price%

			decision.TakeProfit = currentPrice * (1 + tpPricePercent/100)
			decision.StopLoss = currentPrice * (1 - slPricePercent/100)

			if decision.Action == "open_short" {
				decision.TakeProfit = currentPrice * (1 - tpPricePercent/100)
				decision.StopLoss = currentPrice * (1 + slPricePercent/100)
			}

			fc.logger.Debug("TP/SL calculated using fixed ROE",
				"symbol", symbol,
				"leverage", leverage,
				"roe_tp_percent", fc.config.TakeProfitPercent,
				"roe_sl_percent", fc.config.StopLossPercent,
				"price_tp_percent", tpPricePercent,
				"price_sl_percent", slPricePercent,
				"tp_price", decision.TakeProfit,
				"sl_price", decision.StopLoss)
		}

		// Get available allocation (base + profit pool)
		baseAllocation, profitAllocation, useProfitRisk := fc.getAvailableAllocation()
		totalAvailable := baseAllocation + profitAllocation

		fc.logger.Info("Allocation check",
			"symbol", symbol,
			"base_allocation", baseAllocation,
			"profit_allocation", profitAllocation,
			"total_available", totalAvailable)

		// Check if we have enough allocation for minimum position ($15)
		minRequiredPosition := 15.0
		if totalAvailable < minRequiredPosition {
			decision.Approved = false
			decision.RejectionReason = fmt.Sprintf("Insufficient margin: $%.2f available, $%.0f required", totalAvailable, minRequiredPosition)
			fc.logger.Warn("Decision rejected: insufficient margin", "symbol", symbol, "available", totalAvailable)
			return decision
		}

		// Calculate position value respecting max allocation
		accountInfo, err := fc.futuresClient.GetFuturesAccountInfo()
		if err != nil {
			fc.logger.Error("Failed to get account info", "symbol", symbol, "error", err.Error())
			decision.Approved = false
			decision.RejectionReason = "Failed to get account info"
			return decision
		}

		fc.logger.Info("Account info",
			"symbol", symbol,
			"available_balance", accountInfo.AvailableBalance,
			"max_position_size_pct", fc.config.MaxPositionSize)

		if accountInfo.AvailableBalance > 0 {
			// Use the smaller of: account balance percentage OR available allocation
			balanceBasedPosition := accountInfo.AvailableBalance * (fc.config.MaxPositionSize / 100)
			positionValue := balanceBasedPosition
			if positionValue > totalAvailable {
				positionValue = totalAvailable
			}

			// If using profit allocation with aggressive risk
			if useProfitRisk && profitAllocation > 0 {
				// Apply aggressive parameters to profit portion
				profitLeverage := 10 // Aggressive leverage for profit trades
				if profitLeverage > fc.config.MaxLeverage {
					profitLeverage = fc.config.MaxLeverage
				}

				// Calculate how much comes from profit pool
				profitPortion := profitAllocation
				if profitPortion > positionValue {
					profitPortion = positionValue
				}

				// If more than half is from profit pool, use aggressive parameters
				if profitPortion > positionValue/2 {
					decision.Leverage = profitLeverage
					// Aggressive ROE targets: 30% TP, 15% SL
					// Convert to price % using leverage
					aggLeverage := float64(profitLeverage)
					aggTpPrice := 30.0 / aggLeverage / 100 // 30% ROE = price move
					aggSlPrice := 15.0 / aggLeverage / 100 // 15% ROE = price move

					decision.TakeProfit = currentPrice * (1 + aggTpPrice)
					decision.StopLoss = currentPrice * (1 - aggSlPrice)

					if decision.Action == "open_short" {
						decision.TakeProfit = currentPrice * (1 - aggTpPrice)
						decision.StopLoss = currentPrice * (1 + aggSlPrice)
					}

					fc.logger.Debug("Aggressive ROE targets for profit trade",
						"leverage", profitLeverage,
						"roe_tp", 30.0,
						"roe_sl", 15.0,
						"price_tp_pct", aggTpPrice*100,
						"price_sl_pct", aggSlPrice*100)
				}
			}

			// Reject if position value is below minimum notional requirements
			if positionValue < minRequiredPosition {
				decision.Approved = false
				decision.RejectionReason = fmt.Sprintf("Position value $%.2f below minimum $%.0f", positionValue, minRequiredPosition)
				return decision
			}

			// Apply adaptive engine position size multiplier (default 1.0)
			sizeMultiplier := decision.PositionSizeMultiplier
			if sizeMultiplier == 0 {
				sizeMultiplier = 1.0 // Default to 100% if not set
			}
			adjustedPositionValue := positionValue * sizeMultiplier

			// Log if adaptive sizing is being applied
			if sizeMultiplier != 1.0 {
				fc.logger.Info("Adaptive position sizing applied",
					"symbol", symbol,
					"original_value", positionValue,
					"multiplier", fmt.Sprintf("%.2f", sizeMultiplier),
					"adjusted_value", adjustedPositionValue)
			}

			rawQuantity := (adjustedPositionValue * float64(decision.Leverage)) / currentPrice
			decision.Quantity = roundQuantity(symbol, rawQuantity)

			// Verify quantity is valid
			if decision.Quantity <= 0 {
				decision.Approved = false
				decision.RejectionReason = "Calculated quantity is zero or negative"
			}

			// Verify notional value meets minimum requirement ($20 for most futures)
			notionalValue := decision.Quantity * currentPrice
			if notionalValue < 20 {
				decision.Approved = false
				decision.RejectionReason = fmt.Sprintf("Notional value %.2f is below minimum $20", notionalValue)
			}

			// Log position calculation results
			fc.logger.Info("Position calculation complete",
				"symbol", symbol,
				"approved", decision.Approved,
				"quantity", decision.Quantity,
				"notional", notionalValue,
				"rejection_reason", decision.RejectionReason)
		}
	}

	return decision
}

// executeDecision executes a trading decision
func (fc *FuturesController) executeDecision(symbol string, decision *FuturesAutopilotDecision, currentPrice float64) bool {
	// Calculate position value for allocation tracking
	positionValue := (currentPrice * decision.Quantity) / float64(decision.Leverage)

	if fc.dryRun {
		fc.logger.Info("DRY RUN: Would execute futures trade",
			"symbol", symbol,
			"action", decision.Action,
			"leverage", decision.Leverage,
			"quantity", decision.Quantity,
			"position_value", positionValue,
			"take_profit", decision.TakeProfit,
			"stop_loss", decision.StopLoss)

		// Track as virtual position in dry run
		fc.mu.Lock()
		tradeSide := map[string]string{"open_long": "LONG", "open_short": "SHORT"}[decision.Action]

		// Calculate TP1, TP2, and SL based on configuration
		var tp1, tp2, sl float64
		if tradeSide == "LONG" {
			tp1 = currentPrice * (1 + fc.config.TakeProfitPercent1/100)
			tp2 = currentPrice * (1 + fc.config.TakeProfitPercent2/100)
			sl = currentPrice * (1 - fc.config.StopLossPercent/100)
		} else {
			tp1 = currentPrice * (1 - fc.config.TakeProfitPercent1/100)
			tp2 = currentPrice * (1 - fc.config.TakeProfitPercent2/100)
			sl = currentPrice * (1 + fc.config.StopLossPercent/100)
		}

		fc.activePositions[symbol] = &FuturesAutopilotPosition{
			Symbol:       symbol,
			Side:         tradeSide,
			EntryPrice:   currentPrice,
			Quantity:     decision.Quantity,
			Leverage:     decision.Leverage,
			TakeProfit:   decision.TakeProfit,
			TakeProfit1:  tp1,
			TakeProfit2:  tp2,
			StopLoss:     sl,
			HighestPrice: currentPrice,
			LowestPrice:  currentPrice,
			EntryTime:    time.Now(),
			// Position averaging tracking
			EntryCount: 1,
			TotalCost:  currentPrice * decision.Quantity,
			EntryHistory: []PositionEntry{{
				Price:      currentPrice,
				Quantity:   decision.Quantity,
				Time:       time.Now(),
				Confidence: decision.Confidence,
			}},
		}
		fc.dailyTrades++
		// Track USD allocation
		fc.totalUSDAllocated += positionValue
		// Deduct from profit pool if using aggressive risk
		if fc.profitPool > 0 {
			deductFromProfit := positionValue
			if deductFromProfit > fc.profitPool {
				deductFromProfit = fc.profitPool
			}
			fc.profitPool -= deductFromProfit
		}
		// Track last trade for flip-flop prevention
		fc.lastTradeSide[symbol] = tradeSide
		fc.lastTradeAt[symbol] = time.Now()
		fc.mu.Unlock()
		return true // Dry run counts as executed
	}

	// Set leverage
	_, err := fc.futuresClient.SetLeverage(symbol, decision.Leverage)
	if err != nil {
		fc.logger.Error("Failed to set leverage", "symbol", symbol, "error", err.Error())
		return false
	}

	// Place order
	// Determine side and position side based on action and position mode
	side := "BUY"
	positionSide := binance.PositionSideLong
	if decision.Action == "open_short" {
		side = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Check actual Binance position mode (dualSidePosition: true = HEDGE, false = ONE_WAY)
	effectivePositionSide := positionSide
	posMode, err := fc.futuresClient.GetPositionMode()
	if err != nil {
		fc.logger.Warn("Failed to get position mode, using config",
			"error", err.Error(),
			"config_mode", fc.config.PositionMode)
		// Fall back to config
		if fc.config.PositionMode == "ONE_WAY" || fc.config.PositionMode == "" {
			effectivePositionSide = binance.PositionSideBoth
		}
	} else {
		fc.logger.Info("Binance position mode",
			"dual_side_position", posMode.DualSidePosition,
			"symbol", symbol)
		if !posMode.DualSidePosition {
			// ONE_WAY mode - use BOTH
			effectivePositionSide = binance.PositionSideBoth
		}
		// else HEDGE mode - keep LONG/SHORT as already set
	}

	fc.logger.Info("Placing order with params",
		"symbol", symbol,
		"side", side,
		"final_position_side", effectivePositionSide)

	orderParams := binance.FuturesOrderParams{
		Symbol:       symbol,
		Side:         side,
		PositionSide: effectivePositionSide,
		Type:         binance.FuturesOrderTypeMarket,
		Quantity:     decision.Quantity,
	}

	orderResp, err := fc.futuresClient.PlaceFuturesOrder(orderParams)
	if err != nil {
		fc.logger.Error("Failed to place futures order", "symbol", symbol, "error", err.Error())
		return false
	}

	fc.logger.Info("Futures order placed",
		"symbol", symbol,
		"order_id", orderResp.OrderId,
		"side", side,
		"quantity", decision.Quantity)

	// Place TP/SL orders
	fc.placeTPSLOrders(symbol, decision, positionSide)

	// Track position
	fc.mu.Lock()
	tradeSide := "LONG"
	if decision.Action == "open_short" {
		tradeSide = "SHORT"
	}

	// Calculate TP1, TP2, and SL based on configuration
	var tp1, tp2, sl float64
	if tradeSide == "LONG" {
		tp1 = currentPrice * (1 + fc.config.TakeProfitPercent1/100)
		tp2 = currentPrice * (1 + fc.config.TakeProfitPercent2/100)
		sl = currentPrice * (1 - fc.config.StopLossPercent/100)
	} else {
		tp1 = currentPrice * (1 - fc.config.TakeProfitPercent1/100)
		tp2 = currentPrice * (1 - fc.config.TakeProfitPercent2/100)
		sl = currentPrice * (1 + fc.config.StopLossPercent/100)
	}

	fc.activePositions[symbol] = &FuturesAutopilotPosition{
		Symbol:       symbol,
		Side:         tradeSide,
		EntryPrice:   currentPrice,
		Quantity:     decision.Quantity,
		Leverage:     decision.Leverage,
		TakeProfit:   decision.TakeProfit,
		TakeProfit1:  tp1,
		TakeProfit2:  tp2,
		StopLoss:     sl,
		HighestPrice: currentPrice,
		LowestPrice:  currentPrice,
		EntryTime:    time.Now(),
		// Position averaging tracking
		EntryCount: 1,
		TotalCost:  currentPrice * decision.Quantity,
		EntryHistory: []PositionEntry{{
			Price:      currentPrice,
			Quantity:   decision.Quantity,
			Time:       time.Now(),
			Confidence: decision.Confidence,
		}},
	}
	fc.dailyTrades++
	// Track USD allocation
	fc.totalUSDAllocated += positionValue
	// Deduct from profit pool if using aggressive risk
	if fc.profitPool > 0 {
		deductFromProfit := positionValue
		if deductFromProfit > fc.profitPool {
			deductFromProfit = fc.profitPool
		}
		fc.profitPool -= deductFromProfit
	}
	// Track last trade for flip-flop prevention
	fc.lastTradeSide[symbol] = tradeSide
	fc.lastTradeAt[symbol] = time.Now()
	fc.mu.Unlock()

	// Save to database
	fc.saveDecisionToDB(decision, orderResp.OrderId)
	return true
}

// placeTPSLOrders places take profit and stop loss orders
func (fc *FuturesController) placeTPSLOrders(symbol string, decision *FuturesAutopilotDecision, positionSide binance.PositionSide) {
	// Determine the correct position side based on actual Binance position mode
	// In ONE_WAY mode: positionSide should be BOTH
	// In HEDGE mode: positionSide should be LONG or SHORT
	effectivePositionSide := positionSide
	posMode, err := fc.futuresClient.GetPositionMode()
	if err != nil {
		fc.logger.Warn("Failed to get position mode for TP/SL, using config",
			"error", err.Error())
		if fc.config.PositionMode == "ONE_WAY" || fc.config.PositionMode == "" {
			effectivePositionSide = binance.PositionSideBoth
		}
	} else if !posMode.DualSidePosition {
		// ONE_WAY mode - use BOTH
		effectivePositionSide = binance.PositionSideBoth
	}
	// else HEDGE mode - keep LONG/SHORT as passed in

	// Take Profit order using NEW Algo Order API (mandatory since 2025-12-09)
	// For closing positions: SELL to close LONG, BUY to close SHORT
	tpSide := "SELL"
	if positionSide == binance.PositionSideShort {
		tpSide = "BUY"
	}

	fc.logger.Info("Placing TP/SL orders",
		"symbol", symbol,
		"position_mode", fc.config.PositionMode,
		"original_position_side", positionSide,
		"effective_position_side", effectivePositionSide,
		"close_side", tpSide,
		"tp_price", decision.TakeProfit,
		"sl_price", decision.StopLoss)

	tpParams := binance.AlgoOrderParams{
		Symbol:        symbol,
		Side:          tpSide,
		PositionSide:  effectivePositionSide,
		Type:          binance.FuturesOrderTypeTakeProfitMarket,
		TriggerPrice:  roundPrice(symbol, decision.TakeProfit),
		ClosePosition: true,
		WorkingType:   binance.WorkingTypeMarkPrice,
	}
	tpResp, tpErr := fc.futuresClient.PlaceAlgoOrder(tpParams)
	if tpErr != nil {
		fc.logger.Error("Failed to place take profit order",
			"symbol", symbol,
			"trigger_price", decision.TakeProfit,
			"position_side", effectivePositionSide,
			"error", tpErr.Error())
	} else if tpResp != nil {
		fc.logger.Info("Take profit order placed",
			"symbol", symbol,
			"algo_id", tpResp.AlgoId,
			"trigger_price", decision.TakeProfit)
	}

	// Stop Loss order using NEW Algo Order API (mandatory since 2025-12-09)
	slParams := binance.AlgoOrderParams{
		Symbol:        symbol,
		Side:          tpSide,
		PositionSide:  effectivePositionSide,
		Type:          binance.FuturesOrderTypeStopMarket,
		TriggerPrice:  roundPrice(symbol, decision.StopLoss),
		ClosePosition: true,
		WorkingType:   binance.WorkingTypeMarkPrice,
	}
	slResp, slErr := fc.futuresClient.PlaceAlgoOrder(slParams)
	if slErr != nil {
		fc.logger.Error("Failed to place stop loss order",
			"symbol", symbol,
			"trigger_price", decision.StopLoss,
			"position_side", effectivePositionSide,
			"error", slErr.Error())
	} else if slResp != nil {
		fc.logger.Info("Stop loss order placed",
			"symbol", symbol,
			"algo_id", slResp.AlgoId,
			"trigger_price", decision.StopLoss)
	}
}

// placeTPSLOrdersSelective places take profit and/or stop loss orders selectively
// Use this when you need to place only TP, only SL, or both
// Includes retry logic and handles "order would immediately trigger" errors
func (fc *FuturesController) placeTPSLOrdersSelective(symbol string, decision *FuturesAutopilotDecision, positionSide binance.PositionSide, placeTP bool, placeSL bool) {
	if !placeTP && !placeSL {
		return
	}

	// Determine the correct position side based on actual Binance position mode
	effectivePositionSide := positionSide
	posMode, err := fc.futuresClient.GetPositionMode()
	if err != nil {
		fc.logger.Warn("Failed to get position mode for TP/SL, using config",
			"error", err.Error())
		if fc.config.PositionMode == "ONE_WAY" || fc.config.PositionMode == "" {
			effectivePositionSide = binance.PositionSideBoth
		}
	} else if !posMode.DualSidePosition {
		effectivePositionSide = binance.PositionSideBoth
	}

	// For closing positions: SELL to close LONG, BUY to close SHORT
	closeSide := "SELL"
	isLong := true
	if positionSide == binance.PositionSideShort {
		closeSide = "BUY"
		isLong = false
	}

	// Get current price for TP trigger check
	currentPrice, priceErr := fc.futuresClient.GetFuturesCurrentPrice(symbol)
	if priceErr != nil {
		fc.logger.Warn("Failed to get current price for TP check (selective)",
			"symbol", symbol, "error", priceErr)
		currentPrice = 0
	}

	// Retry constants
	const maxRetries = 3
	retryDelay := 500 * time.Millisecond

	// Place Take Profit order if requested
	if placeTP && decision.TakeProfit > 0 {
		roundedTP := roundPrice(symbol, decision.TakeProfit)

		// Check if price already passed TP - would cause "order would immediately trigger" error
		tpAlreadyPassed := false
		if currentPrice > 0 {
			if isLong && currentPrice >= roundedTP {
				tpAlreadyPassed = true
			} else if !isLong && currentPrice <= roundedTP {
				tpAlreadyPassed = true
			}
		}

		if tpAlreadyPassed {
			// Price already passed TP - book profit immediately with market order
			// CRITICAL: MARKET orders cannot use ClosePosition=true (API error -4136)
			// Must use Quantity instead
			roundedQty := roundQuantity(symbol, decision.Quantity)
			fc.logger.Info("TP already passed (selective) - executing market order immediately",
				"symbol", symbol,
				"current_price", currentPrice,
				"tp_price", roundedTP,
				"qty", roundedQty,
				"side", closeSide)

			// Use Quantity for MARKET orders (not ClosePosition)
			orderParams := binance.FuturesOrderParams{
				Symbol:       symbol,
				Side:         closeSide,
				PositionSide: effectivePositionSide,
				Type:         binance.FuturesOrderTypeMarket,
				Quantity:     roundedQty,
			}

			for attempt := 1; attempt <= maxRetries; attempt++ {
				order, err := fc.futuresClient.PlaceFuturesOrder(orderParams)
				if err == nil && order != nil {
					fc.logger.Info("Immediate TP market order executed (selective)",
						"symbol", symbol,
						"order_id", order.OrderId,
						"qty", roundedQty,
						"attempt", attempt)
					break
				}
				fc.logger.Error("Failed to execute immediate TP market order (selective)",
					"symbol", symbol,
					"attempt", attempt,
					"max_retries", maxRetries,
					"error", err.Error())
				if attempt < maxRetries {
					time.Sleep(retryDelay * time.Duration(attempt))
				}
			}
		} else {
			// Normal case - place algo order with retry logic
			tpParams := binance.AlgoOrderParams{
				Symbol:        symbol,
				Side:          closeSide,
				PositionSide:  effectivePositionSide,
				Type:          binance.FuturesOrderTypeTakeProfitMarket,
				TriggerPrice:  roundedTP,
				ClosePosition: true,
				WorkingType:   binance.WorkingTypeMarkPrice,
			}

			var tpPlaced bool
			for attempt := 1; attempt <= maxRetries; attempt++ {
				tpResp, tpErr := fc.futuresClient.PlaceAlgoOrder(tpParams)
				if tpErr == nil && tpResp != nil && tpResp.AlgoId > 0 {
					fc.logger.Info("Take profit order placed (selective)",
						"symbol", symbol,
						"algo_id", tpResp.AlgoId,
						"trigger_price", roundedTP,
						"attempt", attempt)
					tpPlaced = true
					break
				}
				fc.logger.Error("Failed to place take profit order (selective)",
					"symbol", symbol,
					"trigger_price", roundedTP,
					"position_side", effectivePositionSide,
					"attempt", attempt,
					"max_retries", maxRetries,
					"error", tpErr.Error())
				if attempt < maxRetries {
					time.Sleep(retryDelay * time.Duration(attempt))
				}
			}
			if !tpPlaced {
				fc.logger.Error("CRITICAL: Take profit order NOT placed after all retries (selective)",
					"symbol", symbol,
					"tp_price", roundedTP)
			}
		}
	}

	// Place Stop Loss order if requested - with retry logic
	if placeSL && decision.StopLoss > 0 {
		slParams := binance.AlgoOrderParams{
			Symbol:        symbol,
			Side:          closeSide,
			PositionSide:  effectivePositionSide,
			Type:          binance.FuturesOrderTypeStopMarket,
			TriggerPrice:  roundPrice(symbol, decision.StopLoss),
			ClosePosition: true,
			WorkingType:   binance.WorkingTypeMarkPrice,
		}

		var slPlaced bool
		for attempt := 1; attempt <= maxRetries; attempt++ {
			slResp, slErr := fc.futuresClient.PlaceAlgoOrder(slParams)
			if slErr == nil && slResp != nil && slResp.AlgoId > 0 {
				fc.logger.Info("Stop loss order placed (selective)",
					"symbol", symbol,
					"algo_id", slResp.AlgoId,
					"trigger_price", decision.StopLoss,
					"attempt", attempt)
				slPlaced = true
				break
			}
			fc.logger.Error("Failed to place stop loss order (selective)",
				"symbol", symbol,
				"trigger_price", decision.StopLoss,
				"position_side", effectivePositionSide,
				"attempt", attempt,
				"max_retries", maxRetries,
				"error", slErr.Error())
			if attempt < maxRetries {
				time.Sleep(retryDelay * time.Duration(attempt))
			}
		}
		if !slPlaced {
			fc.logger.Error("CRITICAL: Stop loss order NOT placed after all retries (selective)",
				"symbol", symbol,
				"sl_price", decision.StopLoss)
		}
	}
}

// monitorPositions monitors and manages active positions
func (fc *FuturesController) monitorPositions() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	for symbol, pos := range fc.activePositions {
		// Get current price
		currentPrice, err := fc.futuresClient.GetFuturesCurrentPrice(symbol)
		if err != nil {
			continue
		}

		// Update highest/lowest
		if currentPrice > pos.HighestPrice {
			pos.HighestPrice = currentPrice
		}
		if currentPrice < pos.LowestPrice {
			pos.LowestPrice = currentPrice
		}

		// Check TP1 SL move and trailing stop logic
		if fc.config.TrailingStopEnabled {
			var profitPercent float64
			if pos.Side == "LONG" {
				profitPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
			} else {
				profitPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
			}

			// Move SL to buy price when TP1 (10% ROI) is reached
			if !pos.TP1SLMoveDone && profitPercent >= fc.config.TakeProfitPercent1 {
				pos.TP1SLMoveDone = true
				pos.StopLoss = pos.EntryPrice // Move SL to break-even
				fc.logger.Info("TP1 reached: Moving SL to break-even",
					"symbol", symbol,
					"profit_pct", profitPercent,
					"tp1", fc.config.TakeProfitPercent1,
					"new_sl_price", pos.StopLoss)
			}

			// Activate trailing stop at 60% of TP2 (18% ROI)
			tp2ActivationLevel := fc.config.TakeProfitPercent2 * (fc.config.TrailingStopActivationPercent / 100)
			if !pos.TrailingActivated && profitPercent >= tp2ActivationLevel {
				pos.TrailingActivated = true
				fc.logger.Info("Trailing stop activated at TP2 60%",
					"symbol", symbol,
					"profit_pct", profitPercent,
					"activation_level", tp2ActivationLevel)
			}

			// Check trailing stop trigger when activated
			if pos.TrailingActivated {
				var pullback float64

				if pos.Side == "LONG" {
					pullback = (pos.HighestPrice - currentPrice) / pos.HighestPrice * 100
				} else {
					pullback = (currentPrice - pos.LowestPrice) / pos.LowestPrice * 100
				}

				// Trigger close if pullback reaches configured trailing percent (8%)
				if pullback >= fc.config.TrailingStopPercent {
					fc.logger.Info("Trailing stop triggered",
						"symbol", symbol,
						"profit_pct", profitPercent,
						"pullback", pullback,
						"trailing_pct", fc.config.TrailingStopPercent,
						"highest_price", pos.HighestPrice,
						"current_price", currentPrice)
					fc.closePosition(symbol, pos, "trailing_stop")
				}
			}
		}

		// Check scalping mode - take quick profits before regular TP
		if fc.config.ScalpingModeEnabled {
			var profitPercent float64
			if pos.Side == "LONG" {
				profitPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
			} else {
				profitPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
			}

			// Check if we've hit minimum scalping profit threshold
			if profitPercent >= fc.config.ScalpingMinProfit {
				// Check daily trade limit (0 = unlimited)
				if fc.config.ScalpingMaxTradesPerDay == 0 || fc.scalpingTradesToday < fc.config.ScalpingMaxTradesPerDay {
					fc.logger.Info("Scalping mode: taking quick profit",
						"symbol", symbol,
						"profit_pct", profitPercent,
						"min_profit", fc.config.ScalpingMinProfit)
					fc.closePosition(symbol, pos, "scalping_profit")
					continue // Skip remaining checks for this position
				}
			}
		}

		// Check hedging for Position trading style
		if fc.hedgingManager != nil && fc.hedgingManager.IsHedgingEnabled() {
			// Only allow hedging for position/long-term trading style
			if fc.tradingStyle == StylePosition {
				// Check if this symbol already has a hedge
				if !fc.hedgingManager.IsSymbolHedged(symbol) {
					// Evaluate if hedging is needed
					// Note: we don't have LLM recommendation here, could add in the future
					needsHedge, trigger, hedgePercent := fc.hedgingManager.EvaluateHedgingNeed(
						symbol, pos, currentPrice, false, 0)

					if needsHedge {
						_, err := fc.hedgingManager.ExecuteHedge(symbol, pos, hedgePercent, trigger, fc.dryRun)
						if err != nil {
							fc.logger.Error("Failed to execute hedge",
								"symbol", symbol,
								"trigger", trigger,
								"error", err)
						}
					}
				} else {
					// Monitor existing hedge
					shouldClose, reason := fc.hedgingManager.MonitorHedge(symbol, pos, currentPrice)
					if shouldClose {
						pnl, err := fc.hedgingManager.CloseHedge(symbol, reason, fc.dryRun)
						if err != nil {
							fc.logger.Error("Failed to close hedge", "symbol", symbol, "error", err)
						} else {
							fc.logger.Info("Hedge closed", "symbol", symbol, "reason", reason, "pnl", pnl)
							// Add hedge profit to daily PnL
							fc.dailyPnL += pnl
						}
					}
				}
			}
		}

		// Check TP/SL for ALL modes (not just dry_run)
		// This provides a software safety net even if Binance TP/SL orders weren't placed
		if pos.TakeProfit > 0 && pos.StopLoss > 0 {
			if pos.Side == "LONG" {
				if currentPrice >= pos.TakeProfit {
					fc.logger.Info("Software TP triggered", "symbol", symbol, "price", currentPrice, "tp", pos.TakeProfit)
					fc.closePosition(symbol, pos, "take_profit")
				} else if currentPrice <= pos.StopLoss {
					fc.logger.Info("Software SL triggered", "symbol", symbol, "price", currentPrice, "sl", pos.StopLoss)
					fc.closePosition(symbol, pos, "stop_loss")
				}
			} else {
				if currentPrice <= pos.TakeProfit {
					fc.logger.Info("Software TP triggered", "symbol", symbol, "price", currentPrice, "tp", pos.TakeProfit)
					fc.closePosition(symbol, pos, "take_profit")
				} else if currentPrice >= pos.StopLoss {
					fc.logger.Info("Software SL triggered", "symbol", symbol, "price", currentPrice, "sl", pos.StopLoss)
					fc.closePosition(symbol, pos, "stop_loss")
				}
			}
		}
	}
}

// closePosition closes a position
func (fc *FuturesController) closePosition(symbol string, pos *FuturesAutopilotPosition, reason string) {
	currentPrice, _ := fc.futuresClient.GetFuturesCurrentPrice(symbol)

	var pnl float64
	var pnlPercent float64
	if pos.Side == "LONG" {
		pnl = (currentPrice - pos.EntryPrice) * pos.Quantity
		pnlPercent = (currentPrice - pos.EntryPrice) / pos.EntryPrice * 100
	} else {
		pnl = (pos.EntryPrice - currentPrice) * pos.Quantity
		pnlPercent = (pos.EntryPrice - currentPrice) / pos.EntryPrice * 100
	}

	fc.logger.Info("Closing futures position",
		"symbol", symbol,
		"reason", reason,
		"entry", pos.EntryPrice,
		"exit", currentPrice,
		"pnl", pnl,
		"pnl_percent", pnlPercent)

	// Update the futures trade record in database
	go fc.updateTradeOnClose(symbol, currentPrice, pnl, pnlPercent, reason)

	if !fc.dryRun {
		// CRITICAL: Cancel all outstanding TP/SL algo orders FIRST
		// This prevents the orphan order bug where remaining TP/SL opens a new position
		if err := fc.futuresClient.CancelAllAlgoOrders(symbol); err != nil {
			fc.logger.Warn("Failed to cancel algo orders on position close",
				"symbol", symbol,
				"error", err.Error())
		} else {
			fc.logger.Info("Cancelled all algo orders for closed position", "symbol", symbol)
		}

		// Place close order
		side := "SELL"
		positionSide := binance.PositionSideLong
		if pos.Side == "SHORT" {
			side = "BUY"
			positionSide = binance.PositionSideShort
		}

		closeParams := binance.FuturesOrderParams{
			Symbol:       symbol,
			Side:         side,
			PositionSide: positionSide,
			Type:         binance.FuturesOrderTypeMarket,
			Quantity:     pos.Quantity,
			ReduceOnly:   true,
		}
		fc.futuresClient.PlaceFuturesOrder(closeParams)
	}

	// Update daily PnL
	fc.dailyPnL += pnl

	// Calculate position value to release from allocation
	positionValue := pos.EntryPrice * pos.Quantity / float64(pos.Leverage)
	fc.totalUSDAllocated -= positionValue
	if fc.totalUSDAllocated < 0 {
		fc.totalUSDAllocated = 0
	}

	// Remove from active positions
	delete(fc.activePositions, symbol)

	// Track scalping statistics
	if reason == "scalping_profit" {
		// Reset daily counter if new day
		today := time.Now().Truncate(24 * time.Hour)
		if today.After(fc.scalpingDayStart) {
			fc.scalpingTradesToday = 0
			fc.scalpingDayStart = today
		}
		fc.scalpingTradesToday++
		fc.logger.Info("Scalping trade recorded",
			"symbol", symbol,
			"daily_trades", fc.scalpingTradesToday,
			"max_daily", fc.config.ScalpingMaxTradesPerDay)
	}

	// Record close time for quick re-entry
	if fc.config.ScalpingQuickReentry {
		fc.lastCloseTime[symbol] = time.Now()
	}

	// Record to circuit breaker (use percentage, not absolute PnL)
	if fc.circuitBreaker != nil {
		fc.circuitBreaker.RecordTrade(pnlPercent)
	}

	// Add profit to reinvestment pool (called without lock since addToProfit has its own lock)
	// We need to release our lock first to avoid deadlock
	if pnl > 0 {
		go func(profit float64) {
			fc.addToProfit(profit)
		}(pnl)

		// Track success for AI components (assumes all active components contributed)
		fc.mu.Lock()
		if fc.mlPredictor != nil {
			fc.mlSuccessCount++
		}
		if fc.llmAnalyzer != nil {
			fc.llmSuccessCount++
		}
		if fc.sentimentAnalyzer != nil {
			fc.sentimentSuccessCount++
		}
		fc.mu.Unlock()
	}
}

// GetCircuitBreaker returns the circuit breaker instance
func (fc *FuturesController) GetCircuitBreaker() *circuit.CircuitBreaker {
	return fc.circuitBreaker
}

// GetCircuitBreakerStatus returns the current circuit breaker status
func (fc *FuturesController) GetCircuitBreakerStatus() map[string]interface{} {
	if fc.circuitBreaker == nil {
		return map[string]interface{}{
			"available": false,
			"enabled":   false,
			"message":   "Circuit breaker not configured",
		}
	}

	canTrade, blockReason := fc.circuitBreaker.CanTrade()
	stats := fc.circuitBreaker.GetStats()
	config := fc.circuitBreaker.GetConfig()

	return map[string]interface{}{
		"available":          true,
		"enabled":            fc.circuitBreaker.IsEnabled(),
		"state":              stats["state"],
		"can_trade":          canTrade,
		"block_reason":       blockReason,
		"consecutive_losses": stats["consecutive_losses"],
		"hourly_loss":        stats["hourly_loss"],
		"daily_loss":         stats["daily_loss"],
		"trades_last_minute": stats["trades_last_minute"],
		"daily_trades":       stats["daily_trades"],
		"trip_reason":        stats["trip_reason"],
		"config": map[string]interface{}{
			"enabled":                config.Enabled,
			"max_loss_per_hour":      config.MaxLossPerHour,
			"max_daily_loss":         config.MaxDailyLoss,
			"max_consecutive_losses": config.MaxConsecutiveLosses,
			"cooldown_minutes":       config.CooldownMinutes,
			"max_trades_per_minute":  config.MaxTradesPerMinute,
			"max_daily_trades":       config.MaxDailyTrades,
		},
	}
}

// InvestigateStatus contains comprehensive diagnostic information
type InvestigateStatus struct {
	// Quick Health
	TradingStatus    string    `json:"trading_status"`    // "active", "blocked", "stopped"
	BlockReasons     []string  `json:"block_reasons"`     // Why trading is blocked
	LastDecisionTime time.Time `json:"last_decision_time"`
	ActivePositions  int       `json:"active_positions"`

	// Recent Rejections
	RecentRejections []RejectionSummary `json:"recent_rejections"`
	RejectionStats   RejectionStats     `json:"rejection_stats"`

	// Mode Status
	Modes map[string]ModeStatus `json:"modes"`

	// Resource Constraints
	Constraints ConstraintStatus `json:"constraints"`

	// Signal Health
	SignalHealth SignalHealthStatus `json:"signal_health"`

	// Unusual Activity
	Alerts []AlertItem `json:"alerts"`

	// API Health
	APIHealth map[string]string `json:"api_health"`
}

// RejectionSummary summarizes a rejected decision
type RejectionSummary struct {
	Timestamp time.Time `json:"timestamp"`
	Symbol    string    `json:"symbol"`
	Action    string    `json:"action"`
	Reason    string    `json:"reason"`
}

// RejectionStats contains aggregated rejection statistics
type RejectionStats struct {
	TotalDecisions    int            `json:"total_decisions"`
	TotalRejections   int            `json:"total_rejections"`
	RejectionRate     float64        `json:"rejection_rate"`
	CommonReasons     map[string]int `json:"common_reasons"`
	AvgConfidence     float64        `json:"avg_confidence"`
}

// ModeStatus represents the status of a mode/feature
type ModeStatus struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"` // "running", "stopped", "tripped", etc.
	Details string `json:"details"`
}

// ConstraintStatus contains resource constraint information
type ConstraintStatus struct {
	USDAllocation    ConstraintItem `json:"usd_allocation"`
	DailyTrades      ConstraintItem `json:"daily_trades"`
	DailyPnL         ConstraintItem `json:"daily_pnl"`
	HourlyLoss       ConstraintItem `json:"hourly_loss"`
	ConsecutiveLoss  ConstraintItem `json:"consecutive_loss"`
}

// ConstraintItem represents a single constraint with current/max values
type ConstraintItem struct {
	Current float64 `json:"current"`
	Max     float64 `json:"max"`
	Percent float64 `json:"percent"`
	Status  string  `json:"status"` // "ok", "warning", "critical"
}

// SignalHealthStatus contains AI signal component health
type SignalHealthStatus struct {
	MLPredictor       ComponentHealth `json:"ml_predictor"`
	LLMAnalyzer       ComponentHealth `json:"llm_analyzer"`
	SentimentAnalyzer ComponentHealth `json:"sentiment_analyzer"`
	AvgConfidence     float64         `json:"avg_confidence"`
	ConfluenceRate    float64         `json:"confluence_rate"`
}

// ComponentHealth represents the health of an AI component
type ComponentHealth struct {
	Available   bool   `json:"available"`
	LastUsed    string `json:"last_used"`
	SuccessRate float64 `json:"success_rate"`
}

// AlertItem represents an unusual activity alert
type AlertItem struct {
	Level   string `json:"level"`   // "info", "warning", "critical"
	Type    string `json:"type"`    // "rejection_rate", "consecutive_loss", "api_error", etc.
	Message string `json:"message"`
}

// GetInvestigateStatus returns comprehensive diagnostic information
func (fc *FuturesController) GetInvestigateStatus() *InvestigateStatus {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	status := &InvestigateStatus{
		BlockReasons:     make([]string, 0),
		RecentRejections: make([]RejectionSummary, 0),
		Modes:            make(map[string]ModeStatus),
		Alerts:           make([]AlertItem, 0),
		APIHealth:        make(map[string]string),
	}

	// === TRADING STATUS ===
	if !fc.running {
		status.TradingStatus = "stopped"
		status.BlockReasons = append(status.BlockReasons, "Autopilot is stopped")
	} else {
		// Check circuit breaker
		canTrade := true
		if fc.circuitBreaker != nil {
			var blockReason string
			canTrade, blockReason = fc.circuitBreaker.CanTrade()
			if !canTrade {
				status.BlockReasons = append(status.BlockReasons, blockReason)
			}
		}

		// Check daily limits
		if fc.dailyTrades >= fc.config.MaxDailyTrades {
			canTrade = false
			status.BlockReasons = append(status.BlockReasons, fmt.Sprintf("Daily trade limit reached: %d/%d", fc.dailyTrades, fc.config.MaxDailyTrades))
		}
		if fc.dailyPnL <= -fc.config.MaxDailyLoss {
			canTrade = false
			status.BlockReasons = append(status.BlockReasons, fmt.Sprintf("Daily loss limit reached: $%.2f", fc.dailyPnL))
		}

		if canTrade {
			status.TradingStatus = "active"
		} else {
			status.TradingStatus = "blocked"
		}
	}

	// === ACTIVE POSITIONS ===
	status.ActivePositions = len(fc.activePositions)

	// === LAST DECISION TIME ===
	if len(fc.recentDecisions) > 0 {
		status.LastDecisionTime = fc.recentDecisions[0].Timestamp
	}

	// === RECENT REJECTIONS & STATS ===
	rejectionCounts := make(map[string]int)
	totalConfidence := 0.0
	rejectedCount := 0
	for i, decision := range fc.recentDecisions {
		if i >= 20 { // Analyze last 20 decisions
			break
		}
		totalConfidence += decision.Confidence
		if !decision.Approved && decision.RejectionReason != "" {
			rejectedCount++
			if i < 10 { // Show last 10 rejections
				status.RecentRejections = append(status.RecentRejections, RejectionSummary{
					Timestamp: decision.Timestamp,
					Symbol:    decision.Symbol,
					Action:    decision.Action,
					Reason:    decision.RejectionReason,
				})
			}
			// Categorize rejection reason
			reason := categorizeRejection(decision.RejectionReason)
			rejectionCounts[reason]++
		}
	}

	decisionCount := len(fc.recentDecisions)
	if decisionCount > 20 {
		decisionCount = 20
	}
	status.RejectionStats = RejectionStats{
		TotalDecisions:  decisionCount,
		TotalRejections: rejectedCount,
		RejectionRate:   0,
		CommonReasons:   rejectionCounts,
		AvgConfidence:   0,
	}
	if decisionCount > 0 {
		status.RejectionStats.RejectionRate = float64(rejectedCount) / float64(decisionCount) * 100
		status.RejectionStats.AvgConfidence = totalConfidence / float64(decisionCount) * 100
	}

	// === MODE STATUS ===
	// Autopilot
	autopilotDetails := fc.config.RiskLevel
	if fc.dryRun {
		autopilotDetails += ", Paper"
	} else {
		autopilotDetails += ", Live"
	}
	status.Modes["autopilot"] = ModeStatus{
		Enabled: fc.running,
		Status:  map[bool]string{true: "running", false: "stopped"}[fc.running],
		Details: autopilotDetails,
	}

	// Circuit Breaker
	if fc.circuitBreaker != nil {
		cbStats := fc.circuitBreaker.GetStats()
		cbState := cbStats["state"].(string)
		cbDetails := fmt.Sprintf("%v/%d losses", cbStats["consecutive_losses"], fc.circuitBreaker.GetConfig().MaxConsecutiveLosses)
		status.Modes["circuit_breaker"] = ModeStatus{
			Enabled: fc.circuitBreaker.IsEnabled(),
			Status:  cbState,
			Details: cbDetails,
		}
	}

	// Dynamic SL/TP
	status.Modes["dynamic_sltp"] = ModeStatus{
		Enabled: fc.config.DynamicSLTPEnabled,
		Status:  map[bool]string{true: "enabled", false: "disabled"}[fc.config.DynamicSLTPEnabled],
		Details: fmt.Sprintf("ATR %d, SL %.1fx, TP %.1fx", fc.config.ATRPeriod, fc.config.ATRMultiplierSL, fc.config.ATRMultiplierTP),
	}

	// Scalping
	scalpDetails := fmt.Sprintf("%d trades today", fc.scalpingTradesToday)
	if fc.config.ScalpingMaxTradesPerDay > 0 {
		scalpDetails = fmt.Sprintf("%d/%d trades", fc.scalpingTradesToday, fc.config.ScalpingMaxTradesPerDay)
	}
	status.Modes["scalping"] = ModeStatus{
		Enabled: fc.config.ScalpingModeEnabled,
		Status:  map[bool]string{true: "enabled", false: "disabled"}[fc.config.ScalpingModeEnabled],
		Details: scalpDetails,
	}

	// Position Averaging
	status.Modes["averaging"] = ModeStatus{
		Enabled: fc.config.AveragingEnabled,
		Status:  map[bool]string{true: "enabled", false: "disabled"}[fc.config.AveragingEnabled],
		Details: fmt.Sprintf("Max %d entries", fc.config.MaxEntriesPerPosition),
	}

	// === RESOURCE CONSTRAINTS ===
	cbConfig := fc.circuitBreaker.GetConfig()
	cbStats := fc.circuitBreaker.GetStats()

	// USD Allocation
	allocPercent := 0.0
	if fc.maxUSDAllocation > 0 {
		allocPercent = fc.totalUSDAllocated / fc.maxUSDAllocation * 100
	}
	status.Constraints.USDAllocation = ConstraintItem{
		Current: fc.totalUSDAllocated,
		Max:     fc.maxUSDAllocation,
		Percent: allocPercent,
		Status:  getConstraintStatus(allocPercent),
	}

	// Daily Trades
	dailyTradesPercent := 0.0
	if cbConfig.MaxDailyTrades > 0 {
		dailyTradesPercent = float64(fc.dailyTrades) / float64(cbConfig.MaxDailyTrades) * 100
	}
	status.Constraints.DailyTrades = ConstraintItem{
		Current: float64(fc.dailyTrades),
		Max:     float64(cbConfig.MaxDailyTrades),
		Percent: dailyTradesPercent,
		Status:  getConstraintStatus(dailyTradesPercent),
	}

	// Daily PnL (loss)
	dailyLossPercent := 0.0
	if cbConfig.MaxDailyLoss > 0 {
		dailyLoss := cbStats["daily_loss"].(float64)
		dailyLossPercent = dailyLoss / cbConfig.MaxDailyLoss * 100
	}
	status.Constraints.DailyPnL = ConstraintItem{
		Current: fc.dailyPnL,
		Max:     -cbConfig.MaxDailyLoss,
		Percent: dailyLossPercent,
		Status:  getConstraintStatus(dailyLossPercent),
	}

	// Hourly Loss
	hourlyLossPercent := 0.0
	if cbConfig.MaxLossPerHour > 0 {
		hourlyLoss := cbStats["hourly_loss"].(float64)
		hourlyLossPercent = hourlyLoss / cbConfig.MaxLossPerHour * 100
	}
	status.Constraints.HourlyLoss = ConstraintItem{
		Current: cbStats["hourly_loss"].(float64),
		Max:     cbConfig.MaxLossPerHour,
		Percent: hourlyLossPercent,
		Status:  getConstraintStatus(hourlyLossPercent),
	}

	// Consecutive Losses
	consecutiveLossPercent := 0.0
	if cbConfig.MaxConsecutiveLosses > 0 {
		consecutiveLosses := cbStats["consecutive_losses"].(int)
		consecutiveLossPercent = float64(consecutiveLosses) / float64(cbConfig.MaxConsecutiveLosses) * 100
	}
	status.Constraints.ConsecutiveLoss = ConstraintItem{
		Current: float64(cbStats["consecutive_losses"].(int)),
		Max:     float64(cbConfig.MaxConsecutiveLosses),
		Percent: consecutiveLossPercent,
		Status:  getConstraintStatus(consecutiveLossPercent),
	}

	// === SIGNAL HEALTH ===
	// Helper to format timestamp for display
	formatTimestamp := func(t time.Time) string {
		if t.IsZero() {
			return "N/A"
		}
		return t.Format(time.RFC3339)
	}

	// Helper to calculate success rate
	calcSuccessRate := func(success, total int) float64 {
		if total == 0 {
			return 0
		}
		return float64(success) / float64(total)
	}

	// Calculate confluence rate
	confluenceRate := 0.0
	if fc.confluenceTotal > 0 {
		confluenceRate = float64(fc.confluenceAgreements) / float64(fc.confluenceTotal)
	}

	status.SignalHealth = SignalHealthStatus{
		MLPredictor: ComponentHealth{
			Available:   fc.mlPredictor != nil,
			LastUsed:    formatTimestamp(fc.mlLastUsed),
			SuccessRate: calcSuccessRate(fc.mlSuccessCount, fc.mlTotalCount),
		},
		LLMAnalyzer: ComponentHealth{
			Available:   fc.llmAnalyzer != nil,
			LastUsed:    formatTimestamp(fc.llmLastUsed),
			SuccessRate: calcSuccessRate(fc.llmSuccessCount, fc.llmTotalCount),
		},
		SentimentAnalyzer: ComponentHealth{
			Available:   fc.sentimentAnalyzer != nil,
			LastUsed:    formatTimestamp(fc.sentimentLastUsed),
			SuccessRate: calcSuccessRate(fc.sentimentSuccessCount, fc.sentimentTotalCount),
		},
		AvgConfidence:  status.RejectionStats.AvgConfidence,
		ConfluenceRate: confluenceRate,
	}

	// === ALERTS ===
	// High rejection rate
	if status.RejectionStats.RejectionRate > 80 {
		status.Alerts = append(status.Alerts, AlertItem{
			Level:   "warning",
			Type:    "rejection_rate",
			Message: fmt.Sprintf("High rejection rate: %.0f%% in recent decisions", status.RejectionStats.RejectionRate),
		})
	}

	// Consecutive losses approaching limit
	if consecutiveLossPercent >= 60 {
		status.Alerts = append(status.Alerts, AlertItem{
			Level:   "warning",
			Type:    "consecutive_loss",
			Message: fmt.Sprintf("Consecutive losses at %d/%d (%.0f%%)", cbStats["consecutive_losses"], cbConfig.MaxConsecutiveLosses, consecutiveLossPercent),
		})
	}

	// Circuit breaker tripped
	if cbStats["state"].(string) == "open" {
		status.Alerts = append(status.Alerts, AlertItem{
			Level:   "critical",
			Type:    "circuit_breaker",
			Message: fmt.Sprintf("Circuit breaker tripped: %s", cbStats["trip_reason"]),
		})
	}

	// No AI components configured
	if fc.mlPredictor == nil && fc.llmAnalyzer == nil && fc.sentimentAnalyzer == nil {
		status.Alerts = append(status.Alerts, AlertItem{
			Level:   "warning",
			Type:    "no_ai",
			Message: "No AI components configured - ML, LLM, and Sentiment analyzers are all disabled",
		})
	}

	// Low confidence pattern
	if status.RejectionStats.AvgConfidence < 50 && decisionCount >= 5 {
		status.Alerts = append(status.Alerts, AlertItem{
			Level:   "info",
			Type:    "low_confidence",
			Message: fmt.Sprintf("Average confidence is low: %.1f%%", status.RejectionStats.AvgConfidence),
		})
	}

	// === API HEALTH ===
	status.APIHealth["binance_futures"] = "ok"
	if fc.futuresClient == nil {
		status.APIHealth["binance_futures"] = "not_configured"
	}
	status.APIHealth["database"] = "ok"
	if fc.repo == nil {
		status.APIHealth["database"] = "not_configured"
	}

	return status
}

// categorizeRejection categorizes a rejection reason into a simple category
func categorizeRejection(reason string) string {
	reason = strings.ToLower(reason)
	switch {
	case strings.Contains(reason, "confidence"):
		return "Low Confidence"
	case strings.Contains(reason, "confluence"):
		return "Insufficient Confluence"
	case strings.Contains(reason, "margin") || strings.Contains(reason, "allocation"):
		return "Insufficient Margin"
	case strings.Contains(reason, "flip-flop") || strings.Contains(reason, "cooldown"):
		return "Cooldown Active"
	case strings.Contains(reason, "circuit"):
		return "Circuit Breaker"
	case strings.Contains(reason, "short"):
		return "Shorts Disabled"
	case strings.Contains(reason, "daily") || strings.Contains(reason, "limit"):
		return "Daily Limit"
	case strings.Contains(reason, "notional") || strings.Contains(reason, "minimum"):
		return "Below Minimum"
	case strings.Contains(reason, "no ai") || strings.Contains(reason, "no signal"):
		return "No Signals"
	default:
		return "Other"
	}
}

// getConstraintStatus returns status based on percentage used
func getConstraintStatus(percent float64) string {
	if percent >= 90 {
		return "critical"
	} else if percent >= 70 {
		return "warning"
	}
	return "ok"
}

// ClearFlipFlopCooldown clears the flip-flop cooldown for all symbols
func (fc *FuturesController) ClearFlipFlopCooldown() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.lastTradeSide = make(map[string]string)
	fc.lastTradeAt = make(map[string]time.Time)
	fc.logger.Info("Flip-flop cooldown cleared for all symbols")
}

// ForceSyncPositions forces a sync with Binance positions
func (fc *FuturesController) ForceSyncPositions() {
	fc.syncWithActualPositions()
	fc.logger.Info("Forced position sync completed")
}

// ResetCircuitBreaker resets the circuit breaker
func (fc *FuturesController) ResetCircuitBreaker() error {
	if fc.circuitBreaker == nil {
		return fmt.Errorf("circuit breaker not configured")
	}

	fc.circuitBreaker.ForceReset()
	fc.logger.Info("Futures circuit breaker manually reset")
	return nil
}

// UpdateCircuitBreakerConfig updates the circuit breaker configuration
func (fc *FuturesController) UpdateCircuitBreakerConfig(config *circuit.CircuitBreakerConfig) error {
	if fc.circuitBreaker == nil {
		return fmt.Errorf("circuit breaker not configured")
	}

	fc.circuitBreaker.UpdateConfig(config)
	fc.logger.Info("Futures circuit breaker config updated",
		"max_loss_per_hour", config.MaxLossPerHour,
		"max_daily_loss", config.MaxDailyLoss,
		"max_consecutive_losses", config.MaxConsecutiveLosses,
		"cooldown_minutes", config.CooldownMinutes)
	return nil
}

// SetCircuitBreakerEnabled enables or disables the circuit breaker
func (fc *FuturesController) SetCircuitBreakerEnabled(enabled bool) error {
	if fc.circuitBreaker == nil {
		return fmt.Errorf("circuit breaker not configured")
	}

	fc.circuitBreaker.SetEnabled(enabled)
	fc.logger.Info("Futures circuit breaker enabled status changed", "enabled", enabled)
	return nil
}

// saveDecisionToDB saves the AI decision to database
func (fc *FuturesController) saveDecisionToDB(decision *FuturesAutopilotDecision, orderID int64) {
	if fc.repo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Build signals map from signal breakdown
	signals := make(map[string]interface{})
	var mlDir, llmDir, sentimentDir, patternDir, bigcandleDir string
	var mlConf, llmConf, sentimentConf, patternConf, bigcandleConf float64
	confluenceCount := 0

	for k, v := range decision.SignalBreakdown {
		signals[k] = map[string]interface{}{
			"direction":  v.Direction,
			"confidence": v.Confidence,
			"reasoning":  v.Reasoning,
		}
		// Extract individual signal data for AIDecision fields
		switch k {
		case "ml", "ML":
			mlDir = v.Direction
			mlConf = v.Confidence
		case "llm", "LLM":
			llmDir = v.Direction
			llmConf = v.Confidence
		case "sentiment", "Sentiment":
			sentimentDir = v.Direction
			sentimentConf = v.Confidence
		case "pattern", "Pattern":
			patternDir = v.Direction
			patternConf = v.Confidence
		case "bigcandle", "BigCandle":
			bigcandleDir = v.Direction
			bigcandleConf = v.Confidence
		}
		if v.Confidence > 0.5 {
			confluenceCount++
		}
	}
	signals["leverage"] = decision.Leverage
	signals["quantity"] = decision.Quantity
	signals["take_profit"] = decision.TakeProfit
	signals["stop_loss"] = decision.StopLoss

	// Get current price for the decision
	currentPrice, _ := fc.futuresClient.GetFuturesCurrentPrice(decision.Symbol)

	// Determine risk level based on confidence
	riskLevel := "moderate"
	if decision.Confidence >= 0.8 {
		riskLevel = "conservative"
	} else if decision.Confidence <= 0.6 {
		riskLevel = "aggressive"
	}

	aiDecision := &database.AIDecision{
		Symbol:              decision.Symbol,
		CurrentPrice:        currentPrice,
		Action:              decision.Action,
		Confidence:          decision.Confidence,
		Reasoning:           decision.Reasoning,
		MLDirection:         &mlDir,
		MLConfidence:        &mlConf,
		SentimentDirection:  &sentimentDir,
		SentimentConfidence: &sentimentConf,
		LLMDirection:        &llmDir,
		LLMConfidence:       &llmConf,
		PatternDirection:    &patternDir,
		PatternConfidence:   &patternConf,
		BigCandleDirection:  &bigcandleDir,
		BigCandleConfidence: &bigcandleConf,
		ConfluenceCount:     confluenceCount,
		RiskLevel:           riskLevel,
		Executed:            true,
		Signals:             signals,
	}

	err := fc.repo.SaveAIDecision(ctx, aiDecision)
	if err != nil {
		fc.logger.Error("Failed to save AI decision", "error", err.Error())
		return
	}

	// Now create a FuturesTrade entry linked to the AI decision
	positionSide := "LONG"
	side := "BUY"
	if decision.Action == "open_short" {
		positionSide = "SHORT"
		side = "SELL"
	}

	futuresTrade := &database.FuturesTrade{
		Symbol:       decision.Symbol,
		PositionSide: positionSide,
		Side:         side,
		EntryPrice:   currentPrice,
		Quantity:     decision.Quantity,
		Leverage:     decision.Leverage,
		MarginType:   "CROSSED",
		StopLoss:     &decision.StopLoss,
		TakeProfit:   &decision.TakeProfit,
		Status:       "OPEN",
		EntryTime:    time.Now(),
		TradeSource:  "ai",
		AIDecisionID: &aiDecision.ID,
	}

	err = fc.repo.GetDB().CreateFuturesTrade(ctx, futuresTrade)
	if err != nil {
		fc.logger.Error("Failed to save futures trade", "error", err.Error())
	} else {
		fc.logger.Info("Saved futures trade with AI decision",
			"trade_id", futuresTrade.ID,
			"ai_decision_id", aiDecision.ID,
			"symbol", decision.Symbol)
	}
}

// updateTradeOnClose updates the futures trade record when a position is closed
func (fc *FuturesController) updateTradeOnClose(symbol string, exitPrice, pnl, pnlPercent float64, reason string) {
	if fc.repo == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Find the open trade for this symbol
	trade, err := fc.repo.GetDB().GetOpenFuturesTradeBySymbol(ctx, symbol)
	if err != nil {
		fc.logger.Error("Failed to find open trade for closing",
			"symbol", symbol,
			"error", err.Error())
		return
	}
	if trade == nil {
		fc.logger.Warn("No open trade found to close in database",
			"symbol", symbol,
			"reason", reason)
		return
	}

	// Update the trade with close information
	trade.ExitPrice = &exitPrice
	trade.RealizedPnL = &pnl
	trade.RealizedPnLPercent = &pnlPercent
	now := time.Now()
	trade.ExitTime = &now
	trade.Status = "CLOSED"

	// Add close reason to notes
	noteText := fmt.Sprintf("Closed by: %s", reason)
	trade.Notes = &noteText

	err = fc.repo.GetDB().UpdateFuturesTrade(ctx, trade)
	if err != nil {
		fc.logger.Error("Failed to update futures trade on close",
			"symbol", symbol,
			"trade_id", trade.ID,
			"error", err.Error())
	} else {
		fc.logger.Info("Updated futures trade on close",
			"trade_id", trade.ID,
			"symbol", symbol,
			"pnl", pnl,
			"pnl_percent", pnlPercent,
			"reason", reason)
	}
}

// ==================== POSITION AVERAGING ====================

// evaluateAveraging evaluates if an existing position should be averaged
func (fc *FuturesController) evaluateAveraging(symbol string, pos *FuturesAutopilotPosition) {
	// Check max entries limit
	if pos.EntryCount >= fc.config.MaxEntriesPerPosition {
		return
	}

	// Check cooldown
	if time.Since(pos.LastAveragingTime) < time.Duration(fc.config.AveragingCooldownMins)*time.Minute {
		return
	}

	// Get current price
	currentPrice, err := fc.futuresClient.GetFuturesCurrentPrice(symbol)
	if err != nil {
		fc.logger.Error("Failed to get price for averaging", "symbol", symbol, "error", err.Error())
		return
	}

	// Check price improvement requirement
	if !fc.isPriceImproved(pos, currentPrice) {
		return
	}

	// Get klines for AI analysis
	klines, err := fc.futuresClient.GetFuturesKlines(symbol, "1m", 100)
	if err != nil {
		fc.logger.Error("Failed to get klines for averaging", "symbol", symbol, "error", err.Error())
		return
	}

	// Collect signals with news integration for averaging decision
	decision := fc.collectAveragingSignals(symbol, currentPrice, klines, pos)

	if decision.Approved {
		fc.logger.Info("Averaging decision APPROVED",
			"symbol", symbol,
			"side", pos.Side,
			"entry_count", pos.EntryCount+1,
			"confidence", fmt.Sprintf("%.2f", decision.Confidence),
			"current_price", currentPrice,
			"avg_entry", pos.EntryPrice)
		fc.executeAveraging(symbol, pos, decision, currentPrice)
	} else {
		fc.logger.Debug("Averaging decision REJECTED",
			"symbol", symbol,
			"reason", decision.RejectionReason,
			"confidence", fmt.Sprintf("%.2f", decision.Confidence))
	}
}

// isPriceImproved checks if current price is better for averaging
func (fc *FuturesController) isPriceImproved(pos *FuturesAutopilotPosition, currentPrice float64) bool {
	minImprove := fc.config.AveragingMinPriceImprove / 100 // Convert to decimal

	if pos.Side == "LONG" {
		// For LONG: current price must be lower than entry
		return currentPrice < pos.EntryPrice*(1-minImprove)
	}
	// For SHORT: current price must be higher than entry
	return currentPrice > pos.EntryPrice*(1+minImprove)
}

// collectAveragingSignals collects AI signals with news sentiment for averaging decision
func (fc *FuturesController) collectAveragingSignals(
	symbol string,
	currentPrice float64,
	klines []binance.Kline,
	pos *FuturesAutopilotPosition,
) *FuturesAutopilotDecision {
	decision := &FuturesAutopilotDecision{
		Symbol:          symbol,
		Action:          "average_" + strings.ToLower(pos.Side),
		SignalBreakdown: make(map[string]SignalContribution),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	agreementCount := 0
	totalConfidence := 0.0
	signalCount := 0
	newsScore := 0.0

	// ML Predictor Signal
	if fc.mlPredictor != nil && len(klines) >= 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prediction, err := fc.mlPredictor.Predict(symbol, klines, currentPrice, ml.Timeframe60s)
			if err != nil || prediction == nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()

			direction := "neutral"
			// Check if prediction agrees with position direction
			if pos.Side == "LONG" && prediction.Direction == "up" && prediction.Confidence > 0.5 {
				direction = "long"
				agreementCount++
			} else if pos.Side == "SHORT" && prediction.Direction == "down" && prediction.Confidence > 0.5 {
				direction = "short"
				agreementCount++
			}

			decision.SignalBreakdown["ml_predictor"] = SignalContribution{
				Direction:  direction,
				Confidence: prediction.Confidence,
				Reasoning:  fmt.Sprintf("ML: %s (%.0f%% conf)", prediction.Direction, prediction.Confidence*100),
			}
			totalConfidence += prediction.Confidence
			signalCount++
		}()
	}

	// LLM Analyzer Signal
	if fc.llmAnalyzer != nil && fc.llmAnalyzer.IsEnabled() && len(klines) >= 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			analysis, err := fc.llmAnalyzer.AnalyzeMarket(symbol, "1m", klines)
			if err != nil || analysis == nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()

			direction := "neutral"
			// Check if analysis agrees with position direction
			if pos.Side == "LONG" && analysis.Direction == "long" && analysis.Confidence >= 0.5 {
				direction = "long"
				agreementCount++
			} else if pos.Side == "SHORT" && analysis.Direction == "short" && analysis.Confidence >= 0.5 {
				direction = "short"
				agreementCount++
			}

			decision.SignalBreakdown["llm_analyzer"] = SignalContribution{
				Direction:  direction,
				Confidence: analysis.Confidence,
				Reasoning:  analysis.Reasoning,
			}
			totalConfidence += analysis.Confidence
			signalCount++
		}()
	}

	// Sentiment/News - critical for averaging
	if fc.sentimentAnalyzer != nil && fc.sentimentAnalyzer.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			score := fc.sentimentAnalyzer.GetSentiment()
			if score == nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()

			newsScore = score.NewsScore

			// Check news alignment with position direction
			direction := "neutral"
			confidence := 0.5

			if pos.Side == "LONG" {
				if score.Overall > 0.2 { // Bullish sentiment
					direction = "long"
					confidence = math.Min(1.0, 0.5+score.Overall)
					agreementCount++
				}
			} else { // SHORT
				if score.Overall < -0.2 { // Bearish sentiment
					direction = "short"
					confidence = math.Min(1.0, 0.5-score.Overall)
					agreementCount++
				}
			}

			decision.SignalBreakdown["sentiment_news"] = SignalContribution{
				Direction:  direction,
				Confidence: confidence,
				Reasoning:  fmt.Sprintf("News: %.2f, Fear/Greed: %d (%s)", newsScore, score.FearGreedIndex, score.FearGreedLabel),
			}
			totalConfidence += confidence
			signalCount++
		}()
	}

	wg.Wait()

	// Calculate average confidence
	if signalCount > 0 {
		decision.Confidence = totalConfidence / float64(signalCount)
	}

	// Apply news weight adjustment
	newsWeight := fc.config.AveragingNewsWeight
	if newsWeight > 0 && newsScore != 0 {
		// Boost or penalize confidence based on news alignment
		if (pos.Side == "LONG" && newsScore > 0) || (pos.Side == "SHORT" && newsScore < 0) {
			decision.Confidence += math.Abs(newsScore) * newsWeight * 0.1
		} else if (pos.Side == "LONG" && newsScore < 0) || (pos.Side == "SHORT" && newsScore > 0) {
			decision.Confidence -= math.Abs(newsScore) * newsWeight * 0.1
		}
		// Clamp confidence to [0, 1]
		decision.Confidence = math.Max(0, math.Min(1, decision.Confidence))
	}

	// Approve only if confidence >= threshold AND at least 2 signals agree with position direction
	if decision.Confidence >= fc.config.AveragingMinConfidence && agreementCount >= 2 {
		decision.Approved = true
	} else {
		decision.RejectionReason = fmt.Sprintf(
			"confidence=%.2f (need %.2f), agreements=%d (need 2)",
			decision.Confidence, fc.config.AveragingMinConfidence, agreementCount,
		)
	}

	// === ADAPTIVE ENGINE INTEGRATION FOR AVERAGING ===
	// Use human-like logic to validate averaging decision
	if fc.adaptiveEngine != nil && decision.Approved {
		// Make adaptive decision with existing position
		adaptiveDecision := fc.adaptiveEngine.MakeDecision(symbol, currentPrice, klines, pos)

		// Check if adaptive engine recommends averaging
		if !adaptiveDecision.ShouldAverage {
			decision.Approved = false
			decision.RejectionReason = fmt.Sprintf("Adaptive engine: %s", adaptiveDecision.PrimaryReason)
			fc.logger.Info("Adaptive engine rejected averaging",
				"symbol", symbol,
				"reason", adaptiveDecision.PrimaryReason,
				"context", adaptiveDecision.ContextFactors)
		} else {
			// Apply adaptive position sizing to averaging
			decision.PositionSizeMultiplier = adaptiveDecision.PositionSizeMultiplier
			fc.logger.Info("Adaptive engine approved averaging",
				"symbol", symbol,
				"size_multiplier", fmt.Sprintf("%.2f", adaptiveDecision.PositionSizeMultiplier),
				"reason", adaptiveDecision.PrimaryReason)
		}

		// Check if adaptive engine recommends hedging instead
		if adaptiveDecision.ShouldHedge && fc.hedgingManager != nil && fc.hedgingManager.IsHedgingEnabled() {
			fc.logger.Info("Adaptive engine recommends hedging instead of averaging",
				"symbol", symbol,
				"position_pnl_percent", adaptiveDecision.ContextFactors)
			// Don't average - let the hedging monitor handle it
			decision.Approved = false
			decision.RejectionReason = "Adaptive engine recommends hedging - position too far underwater"
		}
	}
	// === END ADAPTIVE ENGINE INTEGRATION ===

	return decision
}

// executeAveraging executes position averaging
func (fc *FuturesController) executeAveraging(
	symbol string,
	pos *FuturesAutopilotPosition,
	decision *FuturesAutopilotDecision,
	currentPrice float64,
) bool {
	// Calculate additional quantity (use same sizing as initial entry)
	addQty := fc.calculateAveragingQuantity(pos, currentPrice)
	if addQty <= 0 {
		fc.logger.Warn("Averaging quantity too small", "symbol", symbol)
		return false
	}

	// Get news score for tracking
	newsScore := 0.0
	if fc.sentimentAnalyzer != nil {
		if score := fc.sentimentAnalyzer.GetSentiment(); score != nil {
			newsScore = score.NewsScore
		}
	}

	if fc.dryRun {
		fc.logger.Info("DRY RUN: Would average into position",
			"symbol", symbol,
			"side", pos.Side,
			"add_qty", addQty,
			"current_price", currentPrice,
			"old_avg", pos.EntryPrice)

		// Update position tracking in dry run
		fc.mu.Lock()
		fc.updatePositionAfterAveraging(pos, addQty, currentPrice, decision.Confidence, newsScore)
		fc.mu.Unlock()
		return true
	}

	// Step 1: Cancel existing TP/SL algo orders
	if err := fc.futuresClient.CancelAllAlgoOrders(symbol); err != nil {
		fc.logger.Warn("Failed to cancel existing algo orders for averaging",
			"symbol", symbol, "error", err.Error())
	} else {
		fc.logger.Info("Cancelled existing TP/SL orders for averaging", "symbol", symbol)
	}

	// Step 2: Place averaging order
	side := "BUY"
	positionSide := binance.PositionSideLong
	if pos.Side == "SHORT" {
		side = "SELL"
		positionSide = binance.PositionSideShort
	}

	// Check position mode
	effectivePositionSide := positionSide
	posMode, err := fc.futuresClient.GetPositionMode()
	if err == nil && !posMode.DualSidePosition {
		effectivePositionSide = binance.PositionSideBoth
	}

	orderParams := binance.FuturesOrderParams{
		Symbol:       symbol,
		Side:         side,
		PositionSide: effectivePositionSide,
		Type:         binance.FuturesOrderTypeMarket,
		Quantity:     addQty,
	}

	orderResp, err := fc.futuresClient.PlaceFuturesOrder(orderParams)
	if err != nil {
		fc.logger.Error("Failed to place averaging order", "symbol", symbol, "error", err.Error())
		// Re-place TP/SL since we cancelled them
		fc.placeTPSLOrders(symbol, &FuturesAutopilotDecision{
			TakeProfit: pos.TakeProfit,
			StopLoss:   pos.StopLoss,
		}, positionSide)
		return false
	}

	fc.logger.Info("Averaging order placed",
		"symbol", symbol,
		"order_id", orderResp.OrderId,
		"side", side,
		"quantity", addQty)

	// Step 3: Update position tracking
	fc.mu.Lock()
	fc.updatePositionAfterAveraging(pos, addQty, currentPrice, decision.Confidence, newsScore)
	fc.mu.Unlock()

	// Step 4: Place new TP/SL orders at updated levels
	fc.placeTPSLOrders(symbol, &FuturesAutopilotDecision{
		Symbol:     symbol,
		TakeProfit: pos.TakeProfit,
		StopLoss:   pos.StopLoss,
	}, positionSide)

	fc.logger.Info("Position averaged successfully",
		"symbol", symbol,
		"entry_count", pos.EntryCount,
		"new_avg_price", pos.EntryPrice,
		"total_qty", pos.Quantity,
		"new_tp", pos.TakeProfit,
		"new_sl", pos.StopLoss)

	return true
}

// calculateAveragingQuantity calculates quantity for averaging order
func (fc *FuturesController) calculateAveragingQuantity(pos *FuturesAutopilotPosition, currentPrice float64) float64 {
	// Use same position value as original entry
	// positionValue = (EntryPrice * Quantity) / Leverage
	originalPositionValue := (pos.EntryPrice * pos.Quantity) / float64(pos.Leverage) / float64(pos.EntryCount)

	// Calculate quantity for same position value at current price
	rawQty := (originalPositionValue * float64(pos.Leverage)) / currentPrice
	return roundQuantity(pos.Symbol, rawQty)
}

// updatePositionAfterAveraging updates position after successful averaging
func (fc *FuturesController) updatePositionAfterAveraging(
	pos *FuturesAutopilotPosition,
	addQty float64,
	currentPrice float64,
	confidence float64,
	newsScore float64,
) {
	// Calculate new weighted average entry price
	oldTotalCost := pos.EntryPrice * pos.Quantity
	addCost := currentPrice * addQty
	newTotalQty := pos.Quantity + addQty
	newAvgPrice := (oldTotalCost + addCost) / newTotalQty

	// Update position
	pos.EntryPrice = newAvgPrice
	pos.Quantity = newTotalQty
	pos.TotalCost = oldTotalCost + addCost
	pos.EntryCount++
	pos.LastAveragingTime = time.Now()

	// Add to entry history
	pos.EntryHistory = append(pos.EntryHistory, PositionEntry{
		Price:      currentPrice,
		Quantity:   addQty,
		Time:       time.Now(),
		Confidence: confidence,
		NewsScore:  newsScore,
	})

	// Recalculate TP/SL based on new average entry price
	fc.recalculateTPSL(pos, newAvgPrice)
}

// recalculateTPSL recalculates TP/SL based on new average entry price
func (fc *FuturesController) recalculateTPSL(pos *FuturesAutopilotPosition, newAvgPrice float64) {
	leverage := float64(pos.Leverage)

	// Check if dynamic SL/TP is enabled
	if fc.config.DynamicSLTPEnabled && fc.futuresClient != nil {
		// Fetch fresh klines for dynamic calculation
		klines, err := fc.futuresClient.GetFuturesKlines(pos.Symbol, "1m", 100)
		if err == nil && len(klines) >= 20 {
			dynamicConfig := &DynamicSLTPConfig{
				Enabled:         true,
				ATRPeriod:       fc.config.ATRPeriod,
				ATRMultiplierSL: fc.config.ATRMultiplierSL,
				ATRMultiplierTP: fc.config.ATRMultiplierTP,
				MinSLPercent:    fc.config.MinSLPercent,
				MaxSLPercent:    fc.config.MaxSLPercent,
				MinTPPercent:    fc.config.MinTPPercent,
				MaxTPPercent:    fc.config.MaxTPPercent,
				LLMWeight:       fc.config.LLMSLTPWeight,
			}

			// Apply defaults
			if dynamicConfig.ATRPeriod == 0 {
				dynamicConfig.ATRPeriod = 14
			}
			if dynamicConfig.ATRMultiplierSL == 0 {
				dynamicConfig.ATRMultiplierSL = 1.5
			}
			if dynamicConfig.ATRMultiplierTP == 0 {
				dynamicConfig.ATRMultiplierTP = 2.0
			}
			if dynamicConfig.MinSLPercent == 0 {
				dynamicConfig.MinSLPercent = 0.3
			}
			if dynamicConfig.MaxSLPercent == 0 {
				dynamicConfig.MaxSLPercent = 3.0
			}
			if dynamicConfig.MinTPPercent == 0 {
				dynamicConfig.MinTPPercent = 0.5
			}
			if dynamicConfig.MaxTPPercent == 0 {
				dynamicConfig.MaxTPPercent = 5.0
			}

			sltpResult := CalculateDynamicSLTP(pos.Symbol, newAvgPrice, klines, pos.Side, nil, dynamicConfig)

			pos.TakeProfit = roundPrice(pos.Symbol, sltpResult.TakeProfitPrice)
			pos.StopLoss = roundPrice(pos.Symbol, sltpResult.StopLossPrice)

			fc.logger.Info("Dynamic TP/SL recalculated after averaging",
				"symbol", pos.Symbol,
				"new_avg_price", newAvgPrice,
				"atr_percent", sltpResult.ATRPercent,
				"tp_price", pos.TakeProfit,
				"sl_price", pos.StopLoss)
			return
		}
	}

	// Fallback to fixed percentage
	tpPricePercent := fc.config.TakeProfitPercent / leverage
	slPricePercent := fc.config.StopLossPercent / leverage

	if pos.Side == "LONG" {
		pos.TakeProfit = roundPrice(pos.Symbol, newAvgPrice*(1+tpPricePercent/100))
		pos.StopLoss = roundPrice(pos.Symbol, newAvgPrice*(1-slPricePercent/100))
	} else {
		pos.TakeProfit = roundPrice(pos.Symbol, newAvgPrice*(1-tpPricePercent/100))
		pos.StopLoss = roundPrice(pos.Symbol, newAvgPrice*(1+slPricePercent/100))
	}
}

// GetAveragingStatus returns averaging configuration and position status
func (fc *FuturesController) GetAveragingStatus() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	positions := make([]map[string]interface{}, 0)
	for symbol, pos := range fc.activePositions {
		positions = append(positions, map[string]interface{}{
			"symbol":        symbol,
			"side":          pos.Side,
			"entry_count":   pos.EntryCount,
			"avg_entry":     pos.EntryPrice,
			"quantity":      pos.Quantity,
			"entry_history": pos.EntryHistory,
		})
	}

	return map[string]interface{}{
		"enabled": fc.config.AveragingEnabled,
		"config": map[string]interface{}{
			"max_entries":        fc.config.MaxEntriesPerPosition,
			"min_confidence":     fc.config.AveragingMinConfidence,
			"min_price_improve":  fc.config.AveragingMinPriceImprove,
			"cooldown_mins":      fc.config.AveragingCooldownMins,
			"news_weight":        fc.config.AveragingNewsWeight,
		},
		"positions": positions,
	}
}

// SetAveragingConfig updates averaging configuration
func (fc *FuturesController) SetAveragingConfig(
	enabled bool,
	maxEntries int,
	minConfidence float64,
	minPriceImprove float64,
	cooldownMins int,
	newsWeight float64,
) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.config.AveragingEnabled = enabled
	if maxEntries > 0 {
		fc.config.MaxEntriesPerPosition = maxEntries
	}
	if minConfidence > 0 {
		fc.config.AveragingMinConfidence = minConfidence
	}
	if minPriceImprove >= 0 {
		fc.config.AveragingMinPriceImprove = minPriceImprove
	}
	if cooldownMins > 0 {
		fc.config.AveragingCooldownMins = cooldownMins
	}
	if newsWeight >= 0 {
		fc.config.AveragingNewsWeight = newsWeight
	}

	fc.logger.Info("Averaging config updated",
		"enabled", enabled,
		"max_entries", fc.config.MaxEntriesPerPosition,
		"min_confidence", fc.config.AveragingMinConfidence)
}

// GetSentimentScore returns the current sentiment score
func (fc *FuturesController) GetSentimentScore() map[string]interface{} {
	if fc.sentimentAnalyzer == nil || !fc.sentimentAnalyzer.IsEnabled() {
		return nil
	}

	score := fc.sentimentAnalyzer.GetSentiment()
	if score == nil {
		return nil
	}

	return map[string]interface{}{
		"overall":          score.Overall,
		"fear_greed_index": score.FearGreedIndex,
		"fear_greed_label": score.FearGreedLabel,
		"news_score":       score.NewsScore,
		"trend_score":      score.TrendScore,
		"updated_at":       score.UpdatedAt,
		"sources":          score.Sources,
	}
}

// GetRecentNews returns recent news items
func (fc *FuturesController) GetRecentNews(limit int) []map[string]interface{} {
	if fc.sentimentAnalyzer == nil || !fc.sentimentAnalyzer.IsEnabled() {
		return []map[string]interface{}{}
	}

	news := fc.sentimentAnalyzer.GetRecentNews(limit)
	return fc.formatNewsItems(news)
}

// GetNewsByTicker returns news filtered by ticker
func (fc *FuturesController) GetNewsByTicker(ticker string, limit int) []map[string]interface{} {
	if fc.sentimentAnalyzer == nil || !fc.sentimentAnalyzer.IsEnabled() {
		return []map[string]interface{}{}
	}

	news := fc.sentimentAnalyzer.GetNewsByTicker(ticker, limit)
	return fc.formatNewsItems(news)
}

// GetBreakingNews returns important/trending news
func (fc *FuturesController) GetBreakingNews(limit int) []map[string]interface{} {
	if fc.sentimentAnalyzer == nil || !fc.sentimentAnalyzer.IsEnabled() {
		return []map[string]interface{}{}
	}

	news := fc.sentimentAnalyzer.GetBreakingNews(limit)
	return fc.formatNewsItems(news)
}

// GetSentimentStats returns sentiment distribution
func (fc *FuturesController) GetSentimentStats() map[string]int {
	if fc.sentimentAnalyzer == nil || !fc.sentimentAnalyzer.IsEnabled() {
		return map[string]int{"bullish": 0, "bearish": 0, "neutral": 0}
	}

	return fc.sentimentAnalyzer.GetSentimentStats()
}

// GetAvailableTickers returns tickers with news
func (fc *FuturesController) GetAvailableTickers() []string {
	if fc.sentimentAnalyzer == nil || !fc.sentimentAnalyzer.IsEnabled() {
		return []string{}
	}

	return fc.sentimentAnalyzer.GetAvailableTickers()
}

// formatNewsItems converts news items to API-friendly format
func (fc *FuturesController) formatNewsItems(news []sentiment.NewsItem) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(news))

	for _, item := range news {
		result = append(result, map[string]interface{}{
			"title":        item.Title,
			"source":       item.Source,
			"url":          item.URL,
			"sentiment":    item.Sentiment,
			"published_at": item.PublishedAt,
			"tickers":      item.Tickers,
			"topic":        item.Topic,
			"is_important": item.IsImportant,
		})
	}

	return result
}

// ==================== USER DATA STREAM INTEGRATION ====================

// HandleStreamPositionUpdate processes real-time position updates from User Data Stream
// This avoids REST API polling for position changes
func (fc *FuturesController) HandleStreamPositionUpdate(update *binance.PositionUpdateEvent) {
	if update == nil {
		return
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	symbol := update.Symbol
	positionKey := symbol

	// Position is closed when amount is 0
	if update.PositionAmt == 0 {
		if existingPos, exists := fc.activePositions[positionKey]; exists {
			fc.logger.Info("Stream: Position closed",
				"symbol", symbol,
				"side", existingPos.Side,
				"entry_price", existingPos.EntryPrice)
			delete(fc.activePositions, positionKey)
		}
		return
	}

	// Determine side from position amount (positive = LONG, negative = SHORT)
	side := "LONG"
	positionAmt := update.PositionAmt
	if positionAmt < 0 {
		side = "SHORT"
		positionAmt = -positionAmt // Make positive for storage
	}

	// Check if position exists
	if existingPos, exists := fc.activePositions[positionKey]; exists {
		// Update existing position
		existingPos.Quantity = positionAmt
		existingPos.EntryPrice = update.EntryPrice
		fc.logger.Debug("Stream: Position updated",
			"symbol", symbol,
			"side", side,
			"qty", positionAmt,
			"entry", update.EntryPrice,
			"pnl", update.UnrealizedPnL)
	} else {
		// New position detected - sync from stream
		fc.logger.Info("Stream: New position detected",
			"symbol", symbol,
			"side", side,
			"qty", positionAmt,
			"entry", update.EntryPrice)

		// Create minimal position tracking (SL/TP will be set by RecalculateSLTP)
		fc.activePositions[positionKey] = &FuturesAutopilotPosition{
			Symbol:     symbol,
			Side:       side,
			EntryPrice: update.EntryPrice,
			Quantity:   positionAmt,
			Leverage:   fc.config.DefaultLeverage,
			EntryTime:  time.Now(),
			EntryCount: 1,
		}
	}
}

// HandleStreamOrderUpdate processes real-time order updates from User Data Stream
func (fc *FuturesController) HandleStreamOrderUpdate(update *binance.OrderUpdateEvent) {
	if update == nil {
		return
	}

	order := update.Order

	// Log important order events
	switch order.ExecutionType {
	case "NEW":
		fc.logger.Debug("Stream: Order created",
			"symbol", order.Symbol,
			"type", order.OrderType,
			"side", order.Side,
			"qty", order.OriginalQuantity)

	case "TRADE":
		fc.logger.Info("Stream: Order filled",
			"symbol", order.Symbol,
			"type", order.OrderType,
			"side", order.Side,
			"qty", order.LastFilledQty,
			"price", order.LastFilledPrice,
			"pnl", order.RealizedProfit)

		// Record trade in circuit breaker if profit is significant
		if fc.circuitBreaker != nil && order.RealizedProfit != 0 {
			// Calculate PnL as percentage (rough estimate)
			if order.AveragePrice > 0 {
				pnlPercent := (order.RealizedProfit / (order.AveragePrice * order.CumulativeFilledQty)) * 100
				fc.circuitBreaker.RecordTrade(pnlPercent)
			}
		}

	case "CANCELED", "EXPIRED":
		fc.logger.Debug("Stream: Order cancelled/expired",
			"symbol", order.Symbol,
			"type", order.OrderType,
			"status", order.OrderStatus)
	}
}

// HandleStreamAccountUpdate processes real-time account updates from User Data Stream
func (fc *FuturesController) HandleStreamAccountUpdate(update *binance.AccountUpdateEvent) {
	if update == nil {
		return
	}

	// Log balance changes
	for _, balance := range update.AccountUpdate.Balances {
		if balance.Asset == "USDT" && balance.BalanceChange != 0 {
			fc.logger.Info("Stream: USDT balance changed",
				"wallet_balance", balance.WalletBalance,
				"change", balance.BalanceChange)
		}
	}
}
