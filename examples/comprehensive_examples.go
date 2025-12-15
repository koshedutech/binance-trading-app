package main

import (
	"binance-trading-bot/config"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/order"
	"binance-trading-bot/internal/strategy"
	"log"
	"time"
)

// This file contains comprehensive examples of how to use the trading bot
// with various strategies and order modification techniques

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize Binance client
	client := binance.NewClient(
		cfg.BinanceConfig.APIKey,
		cfg.BinanceConfig.SecretKey,
		cfg.BinanceConfig.BaseURL,
	)

	// Example 1: Basic Breakout Strategy
	example1_BasicBreakout(client)

	// Example 2: Advanced Multi-Strategy Setup
	example2_MultiStrategy(client)

	// Example 3: Order Management with Modifications
	example3_OrderModification(client)

	// Example 4: Custom Complex Conditions
	example4_ComplexConditions(client)

	// Example 5: Combining Strategies with Order Manager
	example5_AdvancedIntegration(client)
}

// Example 1: Simple breakout strategy on Bitcoin
func example1_BasicBreakout(client *binance.Client) {
	log.Println("\n=== Example 1: Basic Breakout Strategy ===")

	// Create a simple breakout strategy
	breakoutConfig := &strategy.BreakoutConfig{
		Symbol:       "BTCUSDT",
		Interval:     "15m",
		OrderType:    "LIMIT",
		OrderSide:    "BUY",
		PositionSize: 0.01,
		StopLoss:     0.02, // 2%
		TakeProfit:   0.05, // 5%
	}

	strat := strategy.NewBreakoutStrategy(breakoutConfig)

	// Fetch recent candles
	klines, err := client.GetKlines("BTCUSDT", "15m", 50)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Get current price
	currentPrice, err := client.GetCurrentPrice("BTCUSDT")
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Evaluate strategy
	signal, err := strat.Evaluate(klines, currentPrice)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if signal.Type != strategy.SignalNone {
		log.Printf("Signal detected: %s", signal.Reason)
		log.Printf("Entry: %.2f | Stop: %.2f | Target: %.2f", 
			signal.EntryPrice, signal.StopLoss, signal.TakeProfit)
	} else {
		log.Println("No signal at this time")
	}
}

// Example 2: Running multiple strategies simultaneously
func example2_MultiStrategy(client *binance.Client) {
	log.Println("\n=== Example 2: Multi-Strategy Setup ===")

	// Strategy 1: BTC Breakout on 15m
	btcBreakout := strategy.NewBreakoutStrategy(&strategy.BreakoutConfig{
		Symbol:       "BTCUSDT",
		Interval:     "15m",
		OrderType:    "LIMIT",
		OrderSide:    "BUY",
		PositionSize: 0.02,
		StopLoss:     0.02,
		TakeProfit:   0.05,
	})

	// Strategy 2: ETH Support on 5m
	ethSupport := strategy.NewSupportStrategy(&strategy.SupportConfig{
		Symbol:        "ETHUSDT",
		Interval:      "5m",
		OrderType:     "LIMIT",
		OrderSide:     "BUY",
		PositionSize:  0.02,
		StopLoss:      0.02,
		TakeProfit:    0.04,
		TouchDistance: 0.002,
	})

	// Strategy 3: SOL RSI
	solRSI := strategy.NewRSIStrategy(&strategy.RSIStrategyConfig{
		Symbol:          "SOLUSDT",
		Interval:        "15m",
		RSIPeriod:       14,
		OversoldLevel:   30,
		OverboughtLevel: 70,
		PositionSize:    0.02,
		StopLoss:        0.03,
		TakeProfit:      0.06,
	})

	// Strategy 4: AVAX Moving Average Crossover
	avaxMA := strategy.NewMovingAverageCrossoverStrategy(&strategy.MovingAverageCrossoverConfig{
		Symbol:       "AVAXUSDT",
		Interval:     "1h",
		FastPeriod:   9,
		SlowPeriod:   21,
		PositionSize: 0.02,
		StopLoss:     0.03,
		TakeProfit:   0.08,
	})

	// Strategy 5: LINK Volume Spike
	linkVolume := strategy.NewVolumeSpikeStrategy(&strategy.VolumeSpikeConfig{
		Symbol:           "LINKUSDT",
		Interval:         "15m",
		VolumeMultiplier: 2.5,
		MinPriceChange:   2.0,
		LookbackPeriod:   20,
		PositionSize:     0.02,
		StopLoss:         0.02,
		TakeProfit:       0.06,
	})

	strategies := []strategy.Strategy{btcBreakout, ethSupport, solRSI, avaxMA, linkVolume}

	log.Printf("Monitoring %d strategies...", len(strategies))

	for _, strat := range strategies {
		klines, err := client.GetKlines(strat.GetSymbol(), strat.GetInterval(), 50)
		if err != nil {
			log.Printf("Error fetching %s: %v", strat.GetSymbol(), err)
			continue
		}

		currentPrice, err := client.GetCurrentPrice(strat.GetSymbol())
		if err != nil {
			log.Printf("Error fetching price for %s: %v", strat.GetSymbol(), err)
			continue
		}

		signal, err := strat.Evaluate(klines, currentPrice)
		if err != nil {
			log.Printf("Error evaluating %s: %v", strat.Name(), err)
			continue
		}

		if signal.Type != strategy.SignalNone {
			log.Printf("✓ %s: %s", strat.Name(), signal.Reason)
		}
	}
}

// Example 3: Order modification based on conditions
func example3_OrderModification(client *binance.Client) {
	log.Println("\n=== Example 3: Order Modification ===")

	// Initialize order manager
	om := order.NewOrderManager(client)

	// Simulate an existing limit order
	managedOrder := &order.ManagedOrder{
		OrderID:   12345,
		Symbol:    "BTCUSDT",
		Side:      "BUY",
		Type:      "LIMIT",
		Price:     50000.0,
		Quantity:  0.001,
		Status:    "NEW",
		CreatedAt: time.Now(),
	}

	om.AddOrder(managedOrder)

	// Enable trailing stop at 1%
	if err := om.EnableTrailingStop(managedOrder.OrderID, 0.01); err != nil {
		log.Printf("Error enabling trailing stop: %v", err)
	}

	// Add time-based rule: Cancel if not filled in 30 minutes
	timeRule := order.TimeBasedRule{
		Name:        "30min_timeout",
		TriggerTime: time.Now().Add(30 * time.Minute),
		Action:      "CANCEL",
	}
	om.AddTimeBasedRule(managedOrder.OrderID, timeRule)

	// Add price action rule: Convert to market if price moves 2% away
	priceRule := order.PriceActionRule{
		Name:      "chase_price",
		Condition: "PRICE_DISTANCE",
		Threshold: 2.0,
		Action:    "MODIFY_TO_MARKET",
	}
	om.AddPriceActionRule(managedOrder.OrderID, priceRule)

	log.Println("Order modification rules configured:")
	log.Println("  - Trailing stop: 1%")
	log.Println("  - Auto-cancel after 30 minutes")
	log.Println("  - Convert to market if price moves 2%")

	// Process orders (would be called in a loop)
	om.ProcessOrders()
}

// Example 4: Complex multi-condition strategy
func example4_ComplexConditions(client *binance.Client) {
	log.Println("\n=== Example 4: Complex Conditions ===")

	symbol := "ETHUSDT"
	interval := "15m"

	// Fetch data
	klines, err := client.GetKlines(symbol, interval, 100)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	currentPrice, err := client.GetCurrentPrice(symbol)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if len(klines) < 50 {
		log.Println("Insufficient data")
		return
	}

	// Complex condition: All of the following must be true
	// 1. Price breaks above last candle high
	// 2. Volume is above 1.5x average
	// 3. RSI is between 50-70 (not overbought)
	// 4. Price is above 20-period MA
	// 5. Last 3 candles show higher lows (uptrend)

	lastCandle := klines[len(klines)-2]
	
	// Condition 1: Breakout
	breakout := currentPrice > lastCandle.High
	
	// Condition 2: Volume
	avgVolume := 0.0
	for i := len(klines) - 21; i < len(klines)-1; i++ {
		avgVolume += klines[i].Volume
	}
	avgVolume /= 20
	highVolume := lastCandle.Volume > avgVolume*1.5
	
	// Condition 3: RSI not overbought
	rsi := calculateRSI(klines, 14)
	rsiOk := rsi > 50 && rsi < 70
	
	// Condition 4: Above MA20
	ma20 := calculateSMA(klines, 20)
	aboveMA := currentPrice > ma20
	
	// Condition 5: Higher lows
	higherLows := klines[len(klines)-3].Low < klines[len(klines)-2].Low &&
		klines[len(klines)-2].Low < lastCandle.Low

	log.Println("Condition Analysis:")
	log.Printf("  Breakout: %v (current: %.2f, high: %.2f)", breakout, currentPrice, lastCandle.High)
	log.Printf("  High Volume: %v (current: %.0f, avg: %.0f)", highVolume, lastCandle.Volume, avgVolume)
	log.Printf("  RSI OK: %v (RSI: %.2f)", rsiOk, rsi)
	log.Printf("  Above MA20: %v (current: %.2f, MA: %.2f)", aboveMA, currentPrice, ma20)
	log.Printf("  Higher Lows: %v", higherLows)

	allConditions := breakout && highVolume && rsiOk && aboveMA && higherLows

	if allConditions {
		log.Println("✓ ALL CONDITIONS MET - STRONG BUY SIGNAL")
		log.Printf("Entry: %.2f | Stop: %.2f | Target: %.2f",
			currentPrice,
			currentPrice*0.97,
			currentPrice*1.08)
	} else {
		log.Println("✗ Conditions not met")
	}
}

// Example 5: Advanced integration of strategies with order management
func example5_AdvancedIntegration(client *binance.Client) {
	log.Println("\n=== Example 5: Advanced Integration ===")

	om := order.NewOrderManager(client)

	// Setup multiple strategies
	strategies := []struct {
		strategy strategy.Strategy
		rules    []order.PriceActionRule
	}{
		{
			strategy: strategy.NewBreakoutStrategy(&strategy.BreakoutConfig{
				Symbol:       "BTCUSDT",
				Interval:     "15m",
				OrderType:    "LIMIT",
				OrderSide:    "BUY",
				PositionSize: 0.01,
				StopLoss:     0.02,
				TakeProfit:   0.05,
			}),
			rules: []order.PriceActionRule{
				{
					Name:      "aggressive_fill",
					Condition: "PRICE_DISTANCE",
					Threshold: 1.0,
					Action:    "MODIFY_TO_MARKET",
				},
			},
		},
		{
			strategy: strategy.NewRSIStrategy(&strategy.RSIStrategyConfig{
				Symbol:          "ETHUSDT",
				Interval:        "15m",
				RSIPeriod:       14,
				OversoldLevel:   30,
				OverboughtLevel: 70,
				PositionSize:    0.02,
				StopLoss:        0.03,
				TakeProfit:      0.06,
			}),
			rules: []order.PriceActionRule{
				{
					Name:      "chase_entry",
					Condition: "PRICE_DISTANCE",
					Threshold: 0.5,
					Action:    "ADJUST_PRICE",
					Parameters: map[string]interface{}{
						"adjustment": -0.002,
					},
				},
			},
		},
	}

	// Evaluate each strategy and setup order management
	for _, s := range strategies {
		klines, err := client.GetKlines(s.strategy.GetSymbol(), s.strategy.GetInterval(), 50)
		if err != nil {
			continue
		}

		currentPrice, err := client.GetCurrentPrice(s.strategy.GetSymbol())
		if err != nil {
			continue
		}

		signal, err := s.strategy.Evaluate(klines, currentPrice)
		if err != nil || signal.Type == strategy.SignalNone {
			continue
		}

		log.Printf("Signal: %s - %s", s.strategy.Name(), signal.Reason)

		// In production, you would place the actual order here
		// For this example, we simulate an order ID
		simulatedOrderID := int64(time.Now().Unix())

		managedOrder := &order.ManagedOrder{
			OrderID:   simulatedOrderID,
			Symbol:    signal.Symbol,
			Side:      signal.Side,
			Type:      signal.OrderType,
			Price:     signal.EntryPrice,
			Quantity:  0.001,
			Status:    "NEW",
			CreatedAt: time.Now(),
		}

		om.AddOrder(managedOrder)

		// Enable trailing stop
		om.EnableTrailingStop(managedOrder.OrderID, 0.015) // 1.5%

		// Add strategy-specific rules
		for _, rule := range s.rules {
			om.AddPriceActionRule(managedOrder.OrderID, rule)
		}

		// Add timeout rule
		timeoutRule := order.TimeBasedRule{
			Name:        "order_timeout",
			TriggerTime: time.Now().Add(1 * time.Hour),
			Action:      "CANCEL",
		}
		om.AddTimeBasedRule(managedOrder.OrderID, timeoutRule)

		log.Printf("Order %d configured with advanced management", managedOrder.OrderID)
	}

	// Process all orders
	log.Println("\nProcessing managed orders...")
	om.ProcessOrders()

	log.Printf("Active orders: %d", len(om.GetActiveOrders()))
}

// Helper functions

func calculateRSI(klines []binance.Kline, period int) float64 {
	if len(klines) < period+1 {
		return 50.0
	}

	gains, losses := 0.0, 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		change := klines[i].Close - klines[i-1].Close
		if change > 0 {
			gains += change
		} else {
			losses += -change
		}
	}

	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)

	if avgLoss == 0 {
		return 100.0
	}

	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func calculateSMA(klines []binance.Kline, period int) float64 {
	if len(klines) < period {
		return 0
	}

	sum := 0.0
	for i := len(klines) - period; i < len(klines); i++ {
		sum += klines[i].Close
	}

	return sum / float64(period)
}
