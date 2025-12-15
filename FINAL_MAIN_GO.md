# Updated main.go

Replace the contents of `main.go` with this comprehensive version that integrates all components:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"binance-trading-bot/config"
	"binance-trading-bot/internal/api"
	"binance-trading-bot/internal/bot"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/screener"
	"binance-trading-bot/internal/strategy"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize event bus
	eventBus := events.NewEventBus()
	log.Println("Event bus initialized")

	// Initialize database
	dbConfig := database.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		User:     getEnv("DB_USER", "trading_bot"),
		Password: getEnv("DB_PASSWORD", "trading_bot_password"),
		Database: getEnv("DB_NAME", "trading_bot"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	db, err := database.NewDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	ctx := context.Background()
	if err := db.RunMigrations(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create repository
	repo := database.NewRepository(db)

	// Initialize the trading bot
	tradingBot, err := bot.NewTradingBot(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize trading bot: %v", err)
	}

	// Initialize screener
	coinScreener := screener.NewScreener(tradingBot.GetBinanceClient(), cfg.ScreenerConfig)

	// Register strategies
	registerStrategies(tradingBot, cfg)

	// Subscribe to events and persist to database
	setupEventPersistence(eventBus, repo)

	// Initialize WebSocket hub
	wsHub := api.InitWebSocket(eventBus)
	log.Printf("WebSocket hub initialized with %d clients", wsHub.GetClientCount())

	// Initialize web server
	serverConfig := api.ServerConfig{
		Port:            getEnvInt("WEB_PORT", 8088),
		Host:            getEnv("WEB_HOST", "0.0.0.0"),
		ProductionMode:  true,
		StaticFilesPath: "./web/dist", // Path to built React app
	}

	// Create a bot API wrapper for the web interface
	botAPI := &BotAPIWrapper{
		bot:      tradingBot,
		screener: coinScreener,
	}

	server := api.NewServer(serverConfig, repo, eventBus, botAPI)

	// Start web server in a goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start web server: %v", err)
		}
	}()

	// Start the bot
	log.Println("Starting Binance Trading Bot...")
	log.Printf("Dry run mode: %v", cfg.TradingConfig.DryRun)
	if err := tradingBot.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}

	// Publish bot started event
	eventBus.Publish(events.Event{
		Type: events.EventBotStarted,
		Data: map[string]interface{}{
			"dry_run": cfg.TradingConfig.DryRun,
			"testnet": cfg.BinanceConfig.TestNet,
		},
	})

	// Start screener
	go coinScreener.StartScreening()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Publish bot stopped event
	eventBus.Publish(events.Event{
		Type: events.EventBotStopped,
		Data: map[string]interface{}{},
	})

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop web server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error shutting down web server: %v", err)
	}

	// Stop bot and screener
	tradingBot.Stop()
	coinScreener.Stop()

	log.Println("Shutdown complete")
}

func registerStrategies(bot *bot.TradingBot, cfg *config.Config) {
	// Breakout strategy - when price breaks last candle's high
	breakoutStrategy := strategy.NewBreakoutStrategy(&strategy.BreakoutConfig{
		Symbol:        "BTCUSDT",
		Interval:      "15m",
		OrderType:     "LIMIT",
		OrderSide:     "BUY",
		PositionSize:  0.01, // 1% of balance
		StopLoss:      0.02, // 2%
		TakeProfit:    0.05, // 5%
	})
	bot.RegisterStrategy("breakout_high", breakoutStrategy)

	// Support test strategy - when price comes to last candle's low
	supportStrategy := strategy.NewSupportStrategy(&strategy.SupportConfig{
		Symbol:        "ETHUSDT",
		Interval:      "15m",
		OrderType:     "LIMIT",
		OrderSide:     "BUY",
		PositionSize:  0.01,
		StopLoss:      0.02,
		TakeProfit:    0.05,
		TouchDistance: 0.001, // 0.1% distance to consider "touching" the low
	})
	bot.RegisterStrategy("support_low", supportStrategy)

	fmt.Println("Strategies registered successfully")
}

func setupEventPersistence(eventBus *events.EventBus, repo *database.Repository) {
	// Subscribe to trade events
	eventBus.Subscribe(events.EventTradeClosed, func(event events.Event) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create system event record
		sysEvent := &database.SystemEvent{
			EventType: string(event.Type),
			Source:    strPtr("bot"),
			Message:   strPtr("Trade closed"),
			Data:      event.Data,
			Timestamp: event.Timestamp,
		}
		if err := repo.CreateSystemEvent(ctx, sysEvent); err != nil {
			log.Printf("Failed to persist trade closed event: %v", err)
		}
	})

	// Subscribe to signal events
	eventBus.Subscribe(events.EventSignalGenerated, func(event events.Event) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create signal record
		signal := &database.Signal{
			StrategyName: event.Data["strategy"].(string),
			Symbol:       event.Data["symbol"].(string),
			SignalType:   event.Data["signal_type"].(string),
			EntryPrice:   event.Data["price"].(float64),
			Reason:       strPtr(event.Data["reason"].(string)),
			Timestamp:    event.Timestamp,
			Executed:     false,
		}
		if err := repo.CreateSignal(ctx, signal); err != nil {
			log.Printf("Failed to persist signal: %v", err)
		}
	})

	// Subscribe to order events
	eventBus.Subscribe(events.EventOrderPlaced, func(event events.Event) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create order record
		order := &database.Order{
			ID:        event.Data["order_id"].(int64),
			Symbol:    event.Data["symbol"].(string),
			OrderType: event.Data["order_type"].(string),
			Side:      event.Data["side"].(string),
			Quantity:  event.Data["quantity"].(float64),
			Status:    "NEW",
			CreatedAt: event.Timestamp,
		}
		if price, ok := event.Data["price"].(float64); ok {
			order.Price = &price
		}
		if err := repo.CreateOrder(ctx, order); err != nil {
			log.Printf("Failed to persist order: %v", err)
		}
	})

	log.Println("Event persistence configured")
}

// BotAPIWrapper implements the api.BotAPI interface
type BotAPIWrapper struct {
	bot      *bot.TradingBot
	screener *screener.Screener
}

func (w *BotAPIWrapper) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":          true, // The bot is running if this code is executing
		"dry_run":          true, // Get from config
		"testnet":          true, // Get from config
		"strategies_count": 2,    // Count registered strategies
		"open_positions":   0,    // Get from bot
	}
}

func (w *BotAPIWrapper) GetOpenPositions() []map[string]interface{} {
	// Return empty slice for now - implement based on your bot's position tracking
	return []map[string]interface{}{}
}

func (w *BotAPIWrapper) GetStrategies() []map[string]interface{} {
	// Return registered strategies - implement based on your bot's strategy management
	return []map[string]interface{}{
		{
			"name":     "breakout_high",
			"symbol":   "BTCUSDT",
			"interval": "15m",
			"enabled":  true,
		},
		{
			"name":     "support_low",
			"symbol":   "ETHUSDT",
			"interval": "15m",
			"enabled":  true,
		},
	}
}

func (w *BotAPIWrapper) PlaceOrder(symbol, side, orderType string, quantity, price float64) (int64, error) {
	// Implement order placement through your bot
	return 0, fmt.Errorf("manual order placement not yet implemented")
}

func (w *BotAPIWrapper) CancelOrder(orderID int64) error {
	// Implement order cancellation through your bot
	return fmt.Errorf("order cancellation not yet implemented")
}

func (w *BotAPIWrapper) ClosePosition(symbol string) error {
	// Implement position closing through your bot
	return fmt.Errorf("position closing not yet implemented")
}

func (w *BotAPIWrapper) ToggleStrategy(name string, enabled bool) error {
	// Implement strategy toggling
	return fmt.Errorf("strategy toggling not yet implemented")
}

func (w *BotAPIWrapper) GetBinanceClient() interface{} {
	return w.bot.GetBinanceClient()
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func strPtr(s string) *string {
	return &s
}
```

## Important Notes

1. **BotAPIWrapper**: This is a temporary implementation. You'll need to implement the actual methods based on your bot's internal structure.

2. **Event Persistence**: The setupEventPersistence function automatically saves trades, signals, and orders to the database.

3. **Graceful Shutdown**: The application handles SIGINT and SIGTERM signals for proper cleanup.

4. **Environment Variables**: Database and web server configuration can be controlled via environment variables.

## Next Steps

1. Copy this code to `main.go`
2. Implement the actual bot integration methods in BotAPIWrapper
3. Test the application with Docker Compose
