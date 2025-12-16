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
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/notification"
	"binance-trading-bot/internal/risk"
	"binance-trading-bot/internal/scanner"
	"binance-trading-bot/internal/screener"
	"binance-trading-bot/internal/strategy"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize structured logging
	logger := logging.New(&logging.Config{
		Level:       cfg.LoggingConfig.Level,
		Output:      cfg.LoggingConfig.Output,
		JSONFormat:  cfg.LoggingConfig.JSONFormat,
		IncludeFile: cfg.LoggingConfig.IncludeFile,
		Component:   "main",
	})
	logging.SetDefault(logger)
	logger.Info("Structured logging initialized")

	// Initialize event bus
	eventBus := events.NewEventBus()
	logger.Info("Event bus initialized")

	// Initialize notification manager
	var notifyManager *notification.Manager
	if cfg.NotificationConfig.Enabled {
		notifyManager = notification.NewManager()

		// Add Telegram notifier
		if cfg.NotificationConfig.Telegram.Enabled {
			telegramNotifier := notification.NewTelegramNotifier(notification.TelegramConfig{
				BotToken: cfg.NotificationConfig.Telegram.BotToken,
				ChatID:   cfg.NotificationConfig.Telegram.ChatID,
				Enabled:  cfg.NotificationConfig.Telegram.Enabled,
			})
			notifyManager.AddNotifier(telegramNotifier)
			logger.Info("Telegram notifications enabled")
		}

		// Add Discord notifier
		if cfg.NotificationConfig.Discord.Enabled {
			discordNotifier := notification.NewDiscordNotifier(notification.DiscordConfig{
				WebhookURL: cfg.NotificationConfig.Discord.WebhookURL,
				Enabled:    cfg.NotificationConfig.Discord.Enabled,
			})
			notifyManager.AddNotifier(discordNotifier)
			logger.Info("Discord notifications enabled")
		}
	}

	// Initialize risk manager
	riskManager := risk.NewRiskManager(&risk.Config{
		MaxRiskPerTrade:    cfg.RiskConfig.MaxRiskPerTrade,
		MaxDailyDrawdown:   cfg.RiskConfig.MaxDailyDrawdown,
		MaxOpenPositions:   cfg.RiskConfig.MaxOpenPositions,
		PositionSizeMethod: cfg.RiskConfig.PositionSizeMethod,
		FixedPositionSize:  cfg.RiskConfig.FixedPositionSize,
	})
	logger.Info("Risk manager initialized", "method", cfg.RiskConfig.PositionSizeMethod)

	// Initialize trailing stop manager
	trailingStopManager := risk.NewTrailingStopManager(&risk.TrailingConfig{
		Enabled:           cfg.RiskConfig.UseTrailingStop,
		TrailingPercent:   cfg.RiskConfig.TrailingStopPercent,
		ActivationPercent: cfg.RiskConfig.TrailingStopActivation,
	})
	logger.Info("Trailing stop manager initialized", "enabled", cfg.RiskConfig.UseTrailingStop)

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
	tradingBot, err := bot.NewTradingBot(cfg, repo, eventBus)
	if err != nil {
		log.Fatalf("Failed to initialize trading bot: %v", err)
	}

	// Initialize screener
	coinScreener := screener.NewScreener(tradingBot.GetBinanceClient(), cfg.ScreenerConfig, repo)

	// Migrate hardcoded strategies to database
	migrateDefaultStrategies(repo)

	// Register strategies first
	registerStrategies(tradingBot, cfg)

	// Initialize strategy scanner
	scannerConfig := scanner.ScannerConfig{
		Enabled:          cfg.ScannerConfig.Enabled,
		ScanInterval:     time.Duration(cfg.ScannerConfig.ScanInterval) * time.Second,
		MaxSymbols:       cfg.ScannerConfig.MaxSymbols,
		IncludeWatchlist: cfg.ScannerConfig.IncludeWatchlist,
		CacheTTL:         time.Duration(cfg.ScannerConfig.CacheTTL) * time.Second,
		WorkerCount:      cfg.ScannerConfig.WorkerCount,
	}

	// Get all registered strategies
	strategyList := tradingBot.GetRegisteredStrategies()

	strategyScanner := scanner.NewScanner(
		tradingBot.GetBinanceClient(),
		repo,
		strategyList,
		scannerConfig,
	)

	log.Printf("Strategy scanner initialized (enabled: %v, interval: %v)",
		scannerConfig.Enabled, scannerConfig.ScanInterval)

	// Subscribe to events and persist to database
	setupEventPersistence(eventBus, repo, notifyManager, logger)

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
		bot:                 tradingBot,
		screener:            coinScreener,
		scanner:             strategyScanner,
		cfg:                 cfg,
		riskManager:         riskManager,
		trailingStopManager: trailingStopManager,
		notifyManager:       notifyManager,
		logger:              logger,
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
	log.Printf("Web interface available at http://%s:%d", serverConfig.Host, serverConfig.Port)

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

	// Start strategy scanner
	strategyScanner.Start()

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

	// Stop bot, screener, and scanner
	tradingBot.Stop()
	coinScreener.Stop()
	strategyScanner.Stop()

	log.Println("Shutdown complete")
}

func migrateDefaultStrategies(repo *database.Repository) {
	ctx := context.Background()

	// Default strategies to migrate
	defaultStrategies := []*database.StrategyConfig{
		{
			Name:              "Breakout High",
			Symbol:            "BTCUSDT",
			Timeframe:         "15m",
			IndicatorType:     "breakout",
			Autopilot:         false,
			Enabled:           true,
			PositionSize:      0.01,
			StopLossPercent:   2.0,
			TakeProfitPercent: 5.0,
			ConfigParams: map[string]interface{}{
				"description": "Original breakout strategy - when price breaks last candle's high",
			},
		},
		{
			Name:              "Support Low",
			Symbol:            "ETHUSDT",
			Timeframe:         "15m",
			IndicatorType:     "support_test",
			Autopilot:         false,
			Enabled:           true,
			PositionSize:      0.01,
			StopLossPercent:   2.0,
			TakeProfitPercent: 5.0,
			ConfigParams: map[string]interface{}{
				"description": "Original support strategy - when price comes to last candle's low",
				"touch_distance": 0.001,
			},
		},
	}

	// Check and create each strategy if it doesn't exist
	for _, strategyConfig := range defaultStrategies {
		// Try to get all configs and check if this one exists
		existingConfigs, err := repo.GetAllStrategyConfigs(ctx)
		if err != nil {
			log.Printf("Warning: Failed to check existing strategies: %v", err)
			continue
		}

		// Check if strategy with this name already exists
		exists := false
		for _, existing := range existingConfigs {
			if existing.Name == strategyConfig.Name {
				exists = true
				log.Printf("Strategy '%s' already exists in database, skipping migration", strategyConfig.Name)
				break
			}
		}

		if !exists {
			if err := repo.CreateStrategyConfig(ctx, strategyConfig); err != nil {
				log.Printf("Warning: Failed to migrate strategy '%s': %v", strategyConfig.Name, err)
			} else {
				log.Printf("Migrated default strategy '%s' to database", strategyConfig.Name)
			}
		}
	}
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

func setupEventPersistence(eventBus *events.EventBus, repo *database.Repository, notifyManager *notification.Manager, logger *logging.Logger) {
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
			logger.WithError(err).Error("Failed to persist trade closed event")
		}

		// Send notification for closed trades
		if notifyManager != nil {
			symbol, _ := event.Data["symbol"].(string)
			pnl, _ := event.Data["pnl"].(float64)
			pnlPercent, _ := event.Data["pnl_percent"].(float64)
			entryPrice, _ := event.Data["entry_price"].(float64)
			exitPrice, _ := event.Data["exit_price"].(float64)

			if err := notifyManager.SendTradeClose(symbol, entryPrice, exitPrice, pnl, pnlPercent, "closed"); err != nil {
				logger.WithError(err).Warn("Failed to send trade notification")
			}
		}
	})

	// Subscribe to signal events
	eventBus.Subscribe(events.EventSignalGenerated, func(event events.Event) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create signal record
		strategyName, _ := event.Data["strategy"].(string)
		symbol, _ := event.Data["symbol"].(string)
		signalType, _ := event.Data["signal_type"].(string)
		price, _ := event.Data["price"].(float64)
		reason, _ := event.Data["reason"].(string)

		signal := &database.Signal{
			StrategyName: strategyName,
			Symbol:       symbol,
			SignalType:   signalType,
			EntryPrice:   price,
			Reason:       strPtr(reason),
			Timestamp:    event.Timestamp,
			Executed:     false,
		}
		if err := repo.CreateSignal(ctx, signal); err != nil {
			logger.WithError(err).Error("Failed to persist signal")
		}

		// Send notification for new signals
		if notifyManager != nil {
			stopLoss, _ := event.Data["stop_loss"].(float64)
			takeProfit, _ := event.Data["take_profit"].(float64)
			if err := notifyManager.SendSignal(symbol, signalType, reason, price, stopLoss, takeProfit); err != nil {
				logger.WithError(err).Warn("Failed to send signal notification")
			}
		}
	})

	// Subscribe to order events
	eventBus.Subscribe(events.EventOrderPlaced, func(event events.Event) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create order record
		orderID, _ := event.Data["order_id"].(int64)
		symbol, _ := event.Data["symbol"].(string)
		orderType, _ := event.Data["order_type"].(string)
		side, _ := event.Data["side"].(string)
		quantity, _ := event.Data["quantity"].(float64)

		order := &database.Order{
			ID:        orderID,
			Symbol:    symbol,
			OrderType: orderType,
			Side:      side,
			Quantity:  quantity,
			Status:    "NEW",
			CreatedAt: event.Timestamp,
		}
		if price, ok := event.Data["price"].(float64); ok {
			order.Price = &price
		}
		if err := repo.CreateOrder(ctx, order); err != nil {
			logger.WithError(err).Error("Failed to persist order")
		}

		// Send notification for new orders
		if notifyManager != nil {
			price, _ := event.Data["price"].(float64)
			if err := notifyManager.SendTradeOpen(symbol, side, price, quantity); err != nil {
				logger.WithError(err).Warn("Failed to send order notification")
			}
		}
	})

	logger.Info("Event persistence and notifications configured")
}

// BotAPIWrapper implements the api.BotAPI interface
type BotAPIWrapper struct {
	bot                 *bot.TradingBot
	screener            *screener.Screener
	scanner             *scanner.Scanner
	cfg                 *config.Config
	riskManager         *risk.RiskManager
	trailingStopManager *risk.TrailingStopManager
	notifyManager       *notification.Manager
	logger              *logging.Logger
}

func (w *BotAPIWrapper) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":          true,
		"dry_run":          w.cfg.TradingConfig.DryRun,
		"testnet":          w.cfg.BinanceConfig.TestNet,
		"strategies_count": 2,
		"open_positions":   0,
	}
}

func (w *BotAPIWrapper) GetOpenPositions() []map[string]interface{} {
	// Return virtual positions in dry run mode, actual positions otherwise
	return w.bot.GetOpenVirtualTrades()
}

func (w *BotAPIWrapper) GetStrategies() []map[string]interface{} {
	return w.bot.GetStrategyInfo()
}

func (w *BotAPIWrapper) PlaceOrder(symbol, side, orderType string, quantity, price float64) (int64, error) {
	client := w.bot.GetBinanceClient()
	if client == nil {
		return 0, fmt.Errorf("binance client not initialized")
	}

	params := map[string]string{
		"symbol":   symbol,
		"side":     side,
		"type":     orderType,
		"quantity": fmt.Sprintf("%.8f", quantity),
	}

	// Add price for LIMIT orders
	if orderType == "LIMIT" {
		params["price"] = fmt.Sprintf("%.8f", price)
		params["timeInForce"] = "GTC"
	}

	// In dry run mode, simulate the order
	if w.cfg.TradingConfig.DryRun {
		log.Printf("DRY RUN - Manual order: %s %s %.8f %s @ %.8f", side, symbol, quantity, orderType, price)
		// Return a fake order ID for dry run
		return time.Now().UnixNano(), nil
	}

	orderResp, err := client.PlaceOrder(params)
	if err != nil {
		return 0, fmt.Errorf("failed to place order: %w", err)
	}

	log.Printf("Manual order placed: %s %s %.8f @ %.8f (Order ID: %d)", side, symbol, quantity, orderResp.Price, orderResp.OrderId)
	return orderResp.OrderId, nil
}

func (w *BotAPIWrapper) CancelOrder(orderID int64) error {
	client := w.bot.GetBinanceClient()
	if client == nil {
		return fmt.Errorf("binance client not initialized")
	}

	// In dry run mode, just log and return success
	if w.cfg.TradingConfig.DryRun {
		log.Printf("DRY RUN - Cancel order: %d", orderID)
		return nil
	}

	// We need to find the symbol for this order - check open orders in database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get order from database to find the symbol
	order, err := w.bot.GetRepository().GetOrderByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to find order %d: %w", orderID, err)
	}

	if err := client.CancelOrder(order.Symbol, orderID); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	log.Printf("Order cancelled: %d for %s", orderID, order.Symbol)
	return nil
}

func (w *BotAPIWrapper) ClosePosition(symbol string) error {
	client := w.bot.GetBinanceClient()
	if client == nil {
		return fmt.Errorf("binance client not initialized")
	}

	// Get open trades for this symbol from database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	trades, err := w.bot.GetRepository().GetOpenTrades(ctx)
	if err != nil {
		return fmt.Errorf("failed to get open trades: %w", err)
	}

	// Find the trade for this symbol
	var targetTrade *database.Trade
	for _, trade := range trades {
		if trade.Symbol == symbol {
			targetTrade = trade
			break
		}
	}

	if targetTrade == nil {
		return fmt.Errorf("no open position found for %s", symbol)
	}

	// Get current price
	currentPrice, err := client.GetCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %w", err)
	}

	// In dry run mode, update the database directly
	if w.cfg.TradingConfig.DryRun {
		log.Printf("DRY RUN - Closing position: %s at %.8f", symbol, currentPrice)

		// Calculate P&L
		var pnl, pnlPercent float64
		if targetTrade.Side == "BUY" {
			pnl = (currentPrice - targetTrade.EntryPrice) * targetTrade.Quantity
			pnlPercent = ((currentPrice - targetTrade.EntryPrice) / targetTrade.EntryPrice) * 100
		} else {
			pnl = (targetTrade.EntryPrice - currentPrice) * targetTrade.Quantity
			pnlPercent = ((targetTrade.EntryPrice - currentPrice) / targetTrade.EntryPrice) * 100
		}

		// Update trade in database
		targetTrade.ExitPrice = &currentPrice
		now := time.Now()
		targetTrade.ExitTime = &now
		targetTrade.PnL = &pnl
		targetTrade.PnLPercent = &pnlPercent
		targetTrade.Status = "CLOSED"

		if err := w.bot.GetRepository().UpdateTrade(ctx, targetTrade); err != nil {
			return fmt.Errorf("failed to update trade: %w", err)
		}

		log.Printf("Position closed: %s - Entry: %.4f, Exit: %.4f, P&L: %.2f%%",
			symbol, targetTrade.EntryPrice, currentPrice, pnlPercent)
		return nil
	}

	// For live trading, place a market order in the opposite direction
	closeSide := "SELL"
	if targetTrade.Side == "SELL" {
		closeSide = "BUY"
	}

	params := map[string]string{
		"symbol":   symbol,
		"side":     closeSide,
		"type":     "MARKET",
		"quantity": fmt.Sprintf("%.8f", targetTrade.Quantity),
	}

	orderResp, err := client.PlaceOrder(params)
	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	log.Printf("Position closed via market order: %s %s %.8f @ %.8f (Order ID: %d)",
		closeSide, symbol, targetTrade.Quantity, orderResp.Price, orderResp.OrderId)

	return nil
}

func (w *BotAPIWrapper) ToggleStrategy(name string, enabled bool) error {
	if enabled {
		return w.bot.EnableStrategy(name)
	}
	return w.bot.DisableStrategy(name)
}

func (w *BotAPIWrapper) GetBinanceClient() interface{} {
	return w.bot.GetBinanceClient()
}

func (w *BotAPIWrapper) GetClient() interface{} {
	return w.bot.GetBinanceClient()
}

func (w *BotAPIWrapper) ExecutePendingSignal(signal *database.PendingSignal) error {
	return w.bot.ExecutePendingSignal(signal)
}

func (w *BotAPIWrapper) GetScanner() interface{} {
	return w.scanner
}

func (w *BotAPIWrapper) GetRiskManager() *risk.RiskManager {
	return w.riskManager
}

func (w *BotAPIWrapper) GetTrailingStopManager() *risk.TrailingStopManager {
	return w.trailingStopManager
}

func (w *BotAPIWrapper) GetNotificationManager() *notification.Manager {
	return w.notifyManager
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
