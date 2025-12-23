package autopilot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/strategy"
)

// StrategySignal represents a signal generated from a saved strategy
type StrategySignal struct {
	StrategyID   int64   `json:"strategy_id"`
	StrategyName string  `json:"strategy_name"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"` // "LONG" or "SHORT"
	EntryPrice   float64 `json:"entry_price"`
	StopLoss     float64 `json:"stop_loss"`
	TakeProfit   float64 `json:"take_profit"`
	PositionSize float64 `json:"position_size"` // Percentage of account
	Reason       string  `json:"reason"`
	Timestamp    time.Time
}

// LoadedStrategy represents a strategy loaded from database and ready for evaluation
type LoadedStrategy struct {
	ID               int64
	Name             string
	Symbol           string
	Timeframe        string
	Strategy         strategy.Strategy
	PositionSizePct  float64
	StopLossPercent  float64
	TakeProfitPercent float64
}

// StrategyEvaluator handles loading and evaluating saved strategies
type StrategyEvaluator struct {
	db            *database.Repository
	futuresClient binance.FuturesClient
	cache         map[int64]*LoadedStrategy
	cacheMu       sync.RWMutex
	lastLoad      time.Time
	cacheExpiry   time.Duration
	logger        *logging.Logger
}

// NewStrategyEvaluator creates a new strategy evaluator
func NewStrategyEvaluator(db *database.Repository, futuresClient binance.FuturesClient, logger *logging.Logger) *StrategyEvaluator {
	return &StrategyEvaluator{
		db:            db,
		futuresClient: futuresClient,
		cache:         make(map[int64]*LoadedStrategy),
		cacheExpiry:   5 * time.Minute, // Reload strategies every 5 minutes
		logger:        logger,
	}
}

// LoadEnabledStrategies fetches all enabled strategies from database
func (se *StrategyEvaluator) LoadEnabledStrategies() ([]LoadedStrategy, error) {
	se.cacheMu.Lock()
	defer se.cacheMu.Unlock()

	// Check if cache is still valid
	if time.Since(se.lastLoad) < se.cacheExpiry && len(se.cache) > 0 {
		strategies := make([]LoadedStrategy, 0, len(se.cache))
		for _, s := range se.cache {
			strategies = append(strategies, *s)
		}
		return strategies, nil
	}

	// Fetch from database
	ctx := context.Background()
	configs, err := se.db.GetAllStrategyConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch strategy configs: %w", err)
	}

	// Clear old cache
	se.cache = make(map[int64]*LoadedStrategy)
	var loadedStrategies []LoadedStrategy

	for _, config := range configs {
		// Only load enabled strategies with autopilot enabled
		if !config.Enabled || !config.Autopilot {
			continue
		}

		// Convert symbol for futures (ensure USDT suffix)
		symbol := config.Symbol
		if len(symbol) > 0 && symbol[len(symbol)-4:] != "USDT" {
			symbol = symbol + "USDT"
		}

		// Try to create a strategy from the config
		loaded, err := se.loadStrategyFromConfig(config)
		if err != nil {
			se.logger.Warn("Failed to load strategy %s: %v", config.Name, err)
			continue
		}

		loaded.Symbol = symbol
		se.cache[config.ID] = loaded
		loadedStrategies = append(loadedStrategies, *loaded)
	}

	se.lastLoad = time.Now()
	se.logger.Info("Loaded %d enabled strategies for evaluation", len(loadedStrategies))
	return loadedStrategies, nil
}

// loadStrategyFromConfig converts a database config to a loaded strategy
func (se *StrategyEvaluator) loadStrategyFromConfig(config *database.StrategyConfig) (*LoadedStrategy, error) {
	var strat strategy.Strategy

	// Check if this is a visual strategy (has visual_flow in config_params)
	if config.ConfigParams != nil {
		if visualFlow, ok := config.ConfigParams["visual_flow"]; ok {
			visualFlowMap, ok := visualFlow.(map[string]interface{})
			if ok {
				vs, err := strategy.NewVisualStrategy(config.Name, visualFlowMap)
				if err != nil {
					return nil, fmt.Errorf("failed to create visual strategy: %w", err)
				}
				strat = vs
			}
		}
	}

	// If not visual strategy, try to create based on indicator type
	if strat == nil {
		switch config.IndicatorType {
		case "breakout":
			strat = strategy.NewBreakoutStrategy(&strategy.BreakoutConfig{
				Symbol:     config.Symbol,
				Interval:   config.Timeframe,
				StopLoss:   config.StopLossPercent / 100,
				TakeProfit: config.TakeProfitPercent / 100,
			})
		case "rsi":
			strat = strategy.NewRSIStrategy(&strategy.RSIStrategyConfig{
				Symbol:          config.Symbol,
				Interval:        config.Timeframe,
				RSIPeriod:       14,
				OversoldLevel:   30,
				OverboughtLevel: 70,
				StopLoss:        config.StopLossPercent / 100,
				TakeProfit:      config.TakeProfitPercent / 100,
			})
		case "ma_crossover":
			strat = strategy.NewMovingAverageCrossoverStrategy(&strategy.MovingAverageCrossoverConfig{
				Symbol:     config.Symbol,
				Interval:   config.Timeframe,
				FastPeriod: 9,
				SlowPeriod: 21,
				StopLoss:   config.StopLossPercent / 100,
				TakeProfit: config.TakeProfitPercent / 100,
			})
		default:
			return nil, fmt.Errorf("unknown indicator type: %s", config.IndicatorType)
		}
	}

	return &LoadedStrategy{
		ID:                config.ID,
		Name:              config.Name,
		Symbol:            config.Symbol,
		Timeframe:         config.Timeframe,
		Strategy:          strat,
		PositionSizePct:   config.PositionSize,
		StopLossPercent:   config.StopLossPercent,
		TakeProfitPercent: config.TakeProfitPercent,
	}, nil
}

// EvaluateStrategy evaluates a single strategy and returns a signal if triggered
func (se *StrategyEvaluator) EvaluateStrategy(loaded LoadedStrategy) (*StrategySignal, error) {
	// Fetch klines for the strategy's symbol and timeframe
	klines, err := se.futuresClient.GetFuturesKlines(loaded.Symbol, loaded.Timeframe, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch klines for %s: %w", loaded.Symbol, err)
	}

	if len(klines) < 14 {
		return nil, fmt.Errorf("insufficient klines for %s: got %d, need at least 14", loaded.Symbol, len(klines))
	}

	// Get current price
	currentPrice := klines[len(klines)-1].Close

	// Convert futures klines to binance.Kline format for strategy evaluation
	spotKlines := make([]binance.Kline, len(klines))
	for i, k := range klines {
		spotKlines[i] = binance.Kline{
			OpenTime:  k.OpenTime,
			Open:      k.Open,
			High:      k.High,
			Low:       k.Low,
			Close:     k.Close,
			Volume:    k.Volume,
			CloseTime: k.CloseTime,
		}
	}

	// Evaluate the strategy
	signal, err := loaded.Strategy.Evaluate(spotKlines, currentPrice)
	if err != nil {
		return nil, fmt.Errorf("strategy evaluation failed for %s: %w", loaded.Name, err)
	}

	// Check if signal is triggered
	if signal == nil || signal.Type == strategy.SignalNone {
		return nil, nil // No signal
	}

	// Convert strategy signal type to side
	side := "LONG"
	if signal.Type == strategy.SignalSell {
		side = "SHORT"
	}

	// Calculate SL/TP if not provided by strategy
	stopLoss := signal.StopLoss
	takeProfit := signal.TakeProfit

	if stopLoss == 0 {
		if side == "LONG" {
			stopLoss = currentPrice * (1 - loaded.StopLossPercent/100)
		} else {
			stopLoss = currentPrice * (1 + loaded.StopLossPercent/100)
		}
	}

	if takeProfit == 0 {
		if side == "LONG" {
			takeProfit = currentPrice * (1 + loaded.TakeProfitPercent/100)
		} else {
			takeProfit = currentPrice * (1 - loaded.TakeProfitPercent/100)
		}
	}

	return &StrategySignal{
		StrategyID:   loaded.ID,
		StrategyName: loaded.Name,
		Symbol:       loaded.Symbol,
		Side:         side,
		EntryPrice:   currentPrice,
		StopLoss:     stopLoss,
		TakeProfit:   takeProfit,
		PositionSize: loaded.PositionSizePct,
		Reason:       signal.Reason,
		Timestamp:    time.Now(),
	}, nil
}

// EvaluateAllStrategies evaluates all enabled strategies and returns triggered signals
func (se *StrategyEvaluator) EvaluateAllStrategies() ([]StrategySignal, error) {
	strategies, err := se.LoadEnabledStrategies()
	if err != nil {
		return nil, err
	}

	if len(strategies) == 0 {
		return nil, nil
	}

	var signals []StrategySignal
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Evaluate strategies in parallel (max 5 concurrent)
	semaphore := make(chan struct{}, 5)

	for _, strat := range strategies {
		wg.Add(1)
		go func(s LoadedStrategy) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			signal, err := se.EvaluateStrategy(s)
			if err != nil {
				se.logger.Debug("Strategy %s evaluation error: %v", s.Name, err)
				return
			}

			if signal != nil {
				mu.Lock()
				signals = append(signals, *signal)
				mu.Unlock()
				se.logger.Info("Strategy %s triggered %s signal for %s at %.4f",
					s.Name, signal.Side, signal.Symbol, signal.EntryPrice)
			}
		}(strat)
	}

	wg.Wait()
	return signals, nil
}

// GetStrategyByID returns a cached strategy by ID
func (se *StrategyEvaluator) GetStrategyByID(id int64) (*LoadedStrategy, bool) {
	se.cacheMu.RLock()
	defer se.cacheMu.RUnlock()
	strat, ok := se.cache[id]
	return strat, ok
}

// InvalidateCache forces reload of strategies on next evaluation
func (se *StrategyEvaluator) InvalidateCache() {
	se.cacheMu.Lock()
	defer se.cacheMu.Unlock()
	se.lastLoad = time.Time{} // Reset to zero time
}
