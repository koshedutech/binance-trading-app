package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"binance-trading-bot/config"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/bot"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/strategy"
)

// Example: How to integrate pattern confluence strategies with the trading bot

func main() {
	log.Println("=== Pattern Confluence Strategy Example ===")

	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize database
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

	repo := database.NewRepository(db)

	// 3. Initialize event bus
	eventBus := events.NewEventBus()

	// 4. Create trading bot
	tradingBot, err := bot.NewTradingBot(cfg, repo, eventBus)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// 5. Get Binance client from bot
	binanceClient := tradingBot.GetBinanceClient()

	// 6. Create pattern confluence strategies for multiple symbols
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}

	for _, symbol := range symbols {
		strategyConfig := &strategy.PatternConfluenceConfig{
			Symbol:             symbol,
			Interval:           "1h",          // 1-hour timeframe
			StopLossPercent:    0.02,          // 2% stop loss
			TakeProfitPercent:  0.05,          // 5% take profit
			MinConfluenceScore: 0.70,          // 70% minimum confluence
			FVGProximityPercent: 5.0,          // 5% FVG proximity
		}

		patternStrategy := strategy.NewPatternConfluenceStrategy(binanceClient, strategyConfig)

		// Register strategy with bot
		tradingBot.RegisterStrategy(patternStrategy.Name(), patternStrategy)
		log.Printf("Registered pattern confluence strategy for %s", symbol)
	}

	// 7. Start the bot
	log.Println("Starting trading bot with pattern strategies...")
	if err := tradingBot.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}

	// 8. Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Bot is running. Press Ctrl+C to stop.")
	log.Println("Monitoring for pattern signals...")
	log.Println("")

	<-sigChan

	// 9. Graceful shutdown
	log.Println("\nShutting down...")
	tradingBot.Stop()
	log.Println("Bot stopped successfully")
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
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
