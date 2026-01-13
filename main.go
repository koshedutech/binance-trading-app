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
	"binance-trading-bot/internal/ai/llm"
	"binance-trading-bot/internal/ai/ml"
	"binance-trading-bot/internal/ai/sentiment"
	"binance-trading-bot/internal/api"
	"binance-trading-bot/internal/apikeys"
	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/autopilot"
	"binance-trading-bot/internal/billing"
	"binance-trading-bot/internal/binance"
	"binance-trading-bot/internal/bot"
	"binance-trading-bot/internal/cache"
	"binance-trading-bot/internal/circuit"
	"binance-trading-bot/internal/continuous"
	"binance-trading-bot/internal/database"
	"binance-trading-bot/internal/email"
	"binance-trading-bot/internal/events"
	"binance-trading-bot/internal/license"
	"binance-trading-bot/internal/logging"
	"binance-trading-bot/internal/notification"
	"binance-trading-bot/internal/risk"
	"binance-trading-bot/internal/scalping"
	"binance-trading-bot/internal/scanner"
	"binance-trading-bot/internal/screener"
	"binance-trading-bot/internal/strategy"
	"binance-trading-bot/internal/vault"
)

func main() {
	// Load configuration from environment variables
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

	// Run AI migrations
	if err := db.RunAIMigrations(ctx); err != nil {
		log.Printf("Warning: AI migrations failed (table may already exist): %v", err)
	}

	// Run Trade-AI link migrations (adds ai_decision_id to trades table)
	if err := db.RunTradeAILinkMigration(ctx); err != nil {
		log.Printf("Warning: Trade-AI link migrations failed: %v", err)
	}

	// Run Futures migrations if enabled
	if cfg.FuturesConfig.Enabled {
		if err := db.RunFuturesMigrations(ctx); err != nil {
			log.Printf("Warning: Futures migrations failed (table may already exist): %v", err)
		}
		logger.Info("Futures migrations completed")

		// Run Trade Lifecycle migrations for detailed trade event tracking
		if err := db.RunTradeLifecycleMigrations(ctx); err != nil {
			log.Printf("Warning: Trade lifecycle migrations failed: %v", err)
		}
		logger.Info("Trade lifecycle migrations completed")

		// Run Symbol Requirements migrations for order validation
		if err := db.RunSymbolRequirementsMigration(ctx); err != nil {
			log.Printf("Warning: Symbol requirements migrations failed: %v", err)
		}
		logger.Info("Symbol requirements migrations completed")
	}

	// Run Multi-Tenant migrations if auth is enabled
	if cfg.AuthConfig.Enabled {
		if err := db.RunMultiTenantMigrations(ctx); err != nil {
			log.Printf("Warning: Multi-tenant migrations failed: %v", err)
		}
		logger.Info("Multi-tenant migrations completed")

		// Seed admin user with proper password
		if err := auth.SeedAdminUser(ctx, db); err != nil {
			log.Printf("Warning: Failed to seed admin user: %v", err)
		} else {
			logger.Info("Admin user seeded successfully")
		}

		// Run Scan Source migrations for per-user coin source configuration
		if err := db.RunScanSourceMigrations(ctx); err != nil {
			log.Printf("Warning: Scan source migrations failed: %v", err)
		}
		logger.Info("Scan source migrations completed")

		// Run Global Circuit Breaker migrations (Story 5.3)
		if err := db.RunGlobalCircuitBreakerMigration(ctx); err != nil {
			log.Printf("Warning: Global Circuit Breaker migrations failed: %v", err)
		}
		logger.Info("Global Circuit Breaker migrations completed")

		// Run User Mode Config migrations (Epic 4 - per-user trading mode configurations)
		if err := db.MigrateUserModeConfigs(ctx); err != nil {
			log.Printf("Warning: User Mode Config migrations failed: %v", err)
		}
		logger.Info("User Mode Config migrations completed")

		// Run scalp_reentry mode removal migration (Story 9.4 Phase 2)
		// scalp_reentry is an optimization, not a trading mode
		if err := db.MigrateRemoveScalpReentryMode(ctx); err != nil {
			log.Printf("Warning: Scalp_reentry mode removal migration failed: %v", err)
		}
		logger.Info("Scalp_reentry mode removal migration completed")

		// Run User Settings migrations (013-019: LLM config, capital allocation, circuit breaker, etc.)
		if err := db.RunUserSettingsMigrations(ctx); err != nil {
			log.Printf("Warning: User Settings migrations failed: %v", err)
		}
		logger.Info("User Settings migrations completed")

		// Run Early Warning extended fields migration (Story 9.4 Phase 4)
		if err := db.MigrateEarlyWarningExtendedFields(ctx); err != nil {
			log.Printf("Warning: Early Warning extended fields migration failed: %v", err)
		}
		logger.Info("Early Warning extended fields migration completed")

		// Run Mode Config early_warning column migration (Story 9.4 Phase 4)
		if err := db.MigrateModeConfigEarlyWarning(ctx); err != nil {
			log.Printf("Warning: Mode Config early_warning migration failed: %v", err)
		}
		logger.Info("Mode Config early_warning migration completed")

		// Run User Safety Settings migration (023) - Story 9.4
		if err := db.RunUserSafetySettingsMigration(ctx); err != nil {
			log.Printf("Warning: User Safety Settings migration failed: %v", err)
		}
		logger.Info("User Safety Settings migration completed")
	}

	// Create repository early for API key service
	earlyRepo := database.NewRepository(db)

	// Initialize API Key Service for user-specific keys from database
	apiKeyService := apikeys.NewService(earlyRepo) // Used by UserAutopilotManager for per-user AI keys
	logger.Info("API Key Service initialized (using user-stored keys from database)")

	// Initialize Redis Cache Service for caching and instance control
	var cacheService *cache.CacheService
	if cfg.RedisConfig.Enabled {
		var err error
		cacheService, err = cache.NewCacheService(cfg.RedisConfig)
		if err != nil {
			logger.Warn("Failed to initialize Redis cache service", "error", err)
		} else {
			logger.Info("Redis cache service initialized",
				"address", cfg.RedisConfig.Address,
				"healthy", cacheService.IsHealthy())
		}
	} else {
		logger.Info("Redis cache service disabled - running without caching")
	}

	// Initialize Futures clients - NO GLOBAL API KEYS
	// All API keys are per-user and stored in the database
	// Real client will be created per-request based on user's stored keys
	var futuresMockClient binance.FuturesClient
	if cfg.FuturesConfig.Enabled {
		logger.Info("Futures enabled - real client will be created per-request from user database keys")

		// Create mock client for paper trading mode
		priceProvider := func(symbol string) (float64, error) {
			// Default mock prices for paper trading
			mockPrices := map[string]float64{
				"BTCUSDT": 45000.0,
				"ETHUSDT": 2500.0,
				"BNBUSDT": 300.0,
			}
			if price, ok := mockPrices[symbol]; ok {
				return price, nil
			}
			return 100.0, nil
		}
		futuresMockClient = binance.NewFuturesMockClient(10000.0, priceProvider)
		logger.Info("Futures mock client initialized for paper trading")
	}

	// Initialize Market Data Cache for WebSocket data
	var marketDataCache *binance.MarketDataCache
	if cfg.FuturesConfig.Enabled {
		marketDataCache = binance.NewMarketDataCache()
		logger.Info("Market data cache initialized")
	}

	// Initialize Spot clients - NO GLOBAL API KEYS
	// Real client will be created per-request from user database keys
	var spotMockClient binance.BinanceClient
	spotMockClient = binance.NewMockClient()
	logger.Info("Spot mock client initialized for paper trading")
	logger.Info("Spot enabled - real client will be created per-request from user database keys")

	// Create repository (reuse earlyRepo from above)
	repo := earlyRepo

	// Run License migrations
	if err := repo.CreateLicenseTable(ctx); err != nil {
		log.Printf("Warning: License migrations failed (table may already exist): %v", err)
	}

	// Initialize the trading bot
	tradingBot, err := bot.NewTradingBot(cfg, repo, eventBus)
	if err != nil {
		log.Fatalf("Failed to initialize trading bot: %v", err)
	}

	// Initialize Circuit Breaker for safety
	circuitBreakerConfig := &circuit.CircuitBreakerConfig{
		Enabled:              cfg.CircuitBreakerConfig.Enabled,
		MaxLossPerHour:       cfg.CircuitBreakerConfig.MaxLossPerHour,
		MaxConsecutiveLosses: cfg.CircuitBreakerConfig.MaxConsecutiveLosses,
		CooldownMinutes:      cfg.CircuitBreakerConfig.CooldownMinutes,
		MaxTradesPerMinute:   cfg.CircuitBreakerConfig.MaxTradesPerMinute,
		MaxDailyLoss:         cfg.CircuitBreakerConfig.MaxDailyLoss,
		MaxDailyTrades:       cfg.CircuitBreakerConfig.MaxDailyTrades,
	}
	circuitBreaker := circuit.NewCircuitBreaker(circuitBreakerConfig)
	circuitBreaker.OnTrip(func(reason string) {
		logger.Warn("Circuit breaker tripped", "reason", reason)
		// Notification can be added here if needed
	})
	circuitBreaker.OnReset(func() {
		logger.Info("Circuit breaker reset, trading resumed")
	})
	logger.Info("Circuit breaker initialized", "enabled", circuitBreakerConfig.Enabled)

	// Initialize ML Predictor
	var mlPredictor *ml.Predictor
	if cfg.AIConfig.Enabled && cfg.AIConfig.MLEnabled {
		mlConfig := &ml.PredictorConfig{
			MomentumWeight:      0.3,
			MeanReversionWeight: 0.25,
			VolumeWeight:        0.25,
			TrendWeight:         0.2,
			MinConfidence:       cfg.AutopilotConfig.MinConfidence,
		}
		mlPredictor = ml.NewPredictor(mlConfig)
		logger.Info("ML Predictor initialized")
	}

	// Initialize LLM Analyzer - NO GLOBAL KEYS
	// LLM Analyzer will be initialized per-user based on their stored AI keys in database
	// The llmAnalyzer will be nil at startup and injected per-request when user logs in
	var llmAnalyzer *llm.Analyzer
	logger.Info("Global LLM Analyzer disabled - using user-specific AI keys from database per-request")

	// Initialize Sentiment Analyzer
	var sentimentAnalyzer *sentiment.Analyzer
	if cfg.AIConfig.Enabled && cfg.AIConfig.SentimentEnabled {
		cryptoNewsKey := os.Getenv("CRYPTONEWS_API_KEY")
		sentimentConfig := &sentiment.SentimentConfig{
			Enabled:          true,
			FearGreedEnabled: true,
			NewsEnabled:      cryptoNewsKey != "",
			CryptoNewsAPIKey: cryptoNewsKey,
			UpdateInterval:   15 * time.Minute,
			SentimentWeight:  0.2,
		}
		sentimentAnalyzer = sentiment.NewAnalyzer(sentimentConfig)
		sentimentAnalyzer.Start()
		logger.Info("Sentiment Analyzer initialized", "news_enabled", cryptoNewsKey != "")
	}

	// Initialize Big Candle Detector
	var bigCandleDetector *continuous.BigCandleDetector
	if cfg.BigCandleConfig.Enabled {
		bigCandleConfig := &continuous.BigCandleConfig{
			Enabled:            true,
			SizeMultiplier:     cfg.BigCandleConfig.SizeMultiplier,
			LookbackPeriod:     cfg.BigCandleConfig.LookbackPeriod,
			VolumeConfirmation: cfg.BigCandleConfig.VolumeConfirmation,
			ReactImmediately:   cfg.BigCandleConfig.ReactImmediately,
			MinVolumeRatio:     cfg.BigCandleConfig.MinVolumeRatio,
		}
		bigCandleDetector = continuous.NewBigCandleDetector(bigCandleConfig)
		bigCandleDetector.OnBigCandle(func(event *continuous.BigCandleEvent) {
			logger.Info("Big candle detected",
				"symbol", event.Symbol,
				"direction", event.Direction,
				"size_multiplier", event.SizeMultiplier,
				"confidence", event.Confidence)
			eventBus.Publish(events.Event{
				Type: events.EventSignalGenerated,
				Data: map[string]interface{}{
					"strategy":    "big_candle",
					"symbol":      event.Symbol,
					"signal_type": event.Direction,
					"price":       event.ClosePrice,
					"reason":      fmt.Sprintf("Big %s candle: %.1fx average size", event.Direction, event.SizeMultiplier),
				},
			})
		})
		logger.Info("Big Candle Detector initialized", "multiplier", cfg.BigCandleConfig.SizeMultiplier)
	}

	// Initialize Scalping Strategy
	var scalpingStrategy *scalping.ScalpingStrategy
	if cfg.ScalpingConfig.Enabled {
		// Use first timeframe from config or default to 1m
		interval := "1m"
		if len(cfg.ScalpingConfig.Timeframes) > 0 {
			interval = cfg.ScalpingConfig.Timeframes[0]
		}
		scalpConfig := &scalping.ScalpingConfig{
			Enabled:          true,
			Symbol:           "BTCUSDT",
			Interval:         interval,
			MinProfitPercent: cfg.ScalpingConfig.MinProfitPercent,
			MaxLossPercent:   cfg.ScalpingConfig.MaxLossPercent,
			MaxHoldSeconds:   cfg.ScalpingConfig.MaxHoldSeconds,
			PositionSize:     cfg.AutopilotConfig.MaxPositionSize,
			MomentumPeriod:   10,
			VolumeMultiplier: 1.5,
			UseMarketOrders:  true,
		}
		scalpingStrategy = scalping.NewScalpingStrategy(scalpConfig)
		logger.Info("Scalping Strategy initialized",
			"min_profit", cfg.ScalpingConfig.MinProfitPercent,
			"max_hold", cfg.ScalpingConfig.MaxHoldSeconds)
	}

	// Initialize Autopilot Controller
	var autopilotController *autopilot.Controller
	if cfg.AutopilotConfig.Enabled {
		autopilotConfig := &autopilot.AutopilotConfig{
			Enabled:              true,
			RiskLevel:            cfg.AutopilotConfig.RiskLevel,
			MaxDailyLoss:         cfg.AutopilotConfig.MaxDailyLoss,
			MaxPositionSize:      cfg.AutopilotConfig.MaxPositionSize,
			MinConfidence:        cfg.AutopilotConfig.MinConfidence,
			RequireMultiSignal:   getEnvBool("AUTOPILOT_REQUIRE_MULTI_SIGNAL", cfg.AutopilotConfig.RequireConfluence >= 2),
			EnableScalping:       cfg.ScalpingConfig.Enabled,
			EnableBigCandle:      cfg.BigCandleConfig.Enabled,
			EnableLLM:            cfg.AIConfig.ClaudeAPIKey != "",
			EnableML:             cfg.AIConfig.MLEnabled,
			EnableSentiment:      cfg.AIConfig.SentimentEnabled,
			DecisionIntervalSecs: 5,
			DryRun:               cfg.TradingConfig.DryRun,
		}
		autopilotController = autopilot.NewController(autopilotConfig, tradingBot.GetBinanceClient(), circuitBreaker)

		// Wire up AI components
		if mlPredictor != nil {
			autopilotController.SetMLPredictor(mlPredictor)
		}
		if llmAnalyzer != nil {
			autopilotController.SetLLMAnalyzer(llmAnalyzer)
		}
		if sentimentAnalyzer != nil {
			autopilotController.SetSentimentAnalyzer(sentimentAnalyzer)
		}
		if bigCandleDetector != nil {
			autopilotController.SetBigCandleDetector(bigCandleDetector)
		}
		if scalpingStrategy != nil {
			autopilotController.SetScalpingStrategy(scalpingStrategy)
		}

		// Set repository for saving AI decisions
		autopilotController.SetRepository(repo)

		// Initialize order manager for automatic TP/SL and trailing stops
		orderManagerConfig := autopilot.DefaultOrderManagerConfig()
		// Override from env if needed
		if tpPercent := getEnvFloat("ORDER_TAKE_PROFIT_PERCENT", 5.0); tpPercent > 0 {
			orderManagerConfig.TakeProfitPercent = tpPercent
		}
		if slPercent := getEnvFloat("ORDER_STOP_LOSS_PERCENT", 2.0); slPercent > 0 {
			orderManagerConfig.StopLossPercent = slPercent
		}
		if trailPercent := getEnvFloat("ORDER_TRAILING_STOP_PERCENT", 1.0); trailPercent > 0 {
			orderManagerConfig.TrailingStopPercent = trailPercent
		}
		orderManagerConfig.TrailingStopEnabled = getEnvBool("ORDER_TRAILING_ENABLED", true)

		orderManager := autopilot.NewOrderManager(orderManagerConfig, tradingBot.GetBinanceClient(), repo)
		autopilotController.SetOrderManager(orderManager)
		orderManager.Start()
		logger.Info("Order manager started",
			"take_profit", orderManagerConfig.TakeProfitPercent,
			"stop_loss", orderManagerConfig.StopLossPercent,
			"trailing_enabled", orderManagerConfig.TrailingStopEnabled)

		// Set up callbacks
		autopilotController.OnDecision(func(decision *autopilot.TradingDecision) {
			logger.Info("Autopilot decision",
				"symbol", decision.Symbol,
				"action", decision.Action,
				"confidence", decision.Confidence,
				"approved", decision.Approved)
		})
		autopilotController.OnTrade(func(trade *autopilot.Trade) {
			logger.Info("Autopilot trade executed",
				"symbol", trade.Symbol,
				"side", trade.Side,
				"price", trade.Price)
			// Record trade in circuit breaker
			circuitBreaker.RecordTrade(trade.PnLPercent)
		})

		logger.Info("Autopilot Controller initialized",
			"risk_level", cfg.AutopilotConfig.RiskLevel,
			"dry_run", cfg.TradingConfig.DryRun)
	}

	// Initialize Futures Autopilot Controller
	var futuresAutopilotController *autopilot.FuturesController
	if cfg.FuturesConfig.Enabled && cfg.FuturesAutopilotConfig.Enabled {
		futuresLogger := logging.New(&logging.Config{
			Level:       cfg.LoggingConfig.Level,
			Output:      cfg.LoggingConfig.Output,
			JSONFormat:  cfg.LoggingConfig.JSONFormat,
			IncludeFile: cfg.LoggingConfig.IncludeFile,
			Component:   "futures_autopilot",
		})

		// Get the active futures client based on mode
		// Note: Real client is created per-request from user database keys
		// At startup, we only have mock client available
		var activeFuturesClient binance.FuturesClient
		if cfg.TradingConfig.DryRun {
			activeFuturesClient = futuresMockClient
		} else {
			// For live mode, start with mock but real client will be injected per-request
			// based on user's API keys from database
			activeFuturesClient = futuresMockClient
			logger.Info("Live mode: real client will be injected per-request from user API keys")
		}

		// Wrap with cached client if cache is available (for WebSocket data)
		if marketDataCache != nil {
			activeFuturesClient = binance.NewCachedFuturesClient(activeFuturesClient, marketDataCache)
			logger.Info("Futures client wrapped with cache for WebSocket data")
		}

		futuresAutopilotController = autopilot.NewFuturesController(
			&cfg.FuturesAutopilotConfig,
			activeFuturesClient,
			circuitBreaker,
			repo,
			futuresLogger,
		)

		// Wire up AI components to futures autopilot
		if mlPredictor != nil {
			futuresAutopilotController.SetMLPredictor(mlPredictor)
		}
		if llmAnalyzer != nil {
			futuresAutopilotController.SetLLMAnalyzer(llmAnalyzer)
		}
		if sentimentAnalyzer != nil {
			futuresAutopilotController.SetSentimentAnalyzer(sentimentAnalyzer)
		}

		// Set dry run mode
		futuresAutopilotController.SetDryRun(cfg.TradingConfig.DryRun)

		// Load saved settings from persistent storage (overrides config file defaults)
		futuresAutopilotController.LoadSavedSettings()

		// Swap client if the dry run mode changed due to saved settings
		// This ensures the correct client (real vs mock) is used based on persisted settings
		actualDryRun := futuresAutopilotController.GetDryRun()
		if actualDryRun != cfg.TradingConfig.DryRun {
			logger.Info("Dry run mode changed by saved settings, swapping futures client",
				"config_dry_run", cfg.TradingConfig.DryRun,
				"actual_dry_run", actualDryRun)

			if actualDryRun {
				futuresAutopilotController.SetFuturesClient(futuresMockClient)
			} else {
				// LIVE mode: Reload precision cache from REAL Binance API
				// The initial load used mock client which only has ~30 symbols
				logger.Info("Reloading symbol precisions from Binance API (switching to LIVE mode)")
				autopilot.SetPrecisionClient(activeFuturesClient)
				if err := autopilot.LoadSymbolPrecisions(activeFuturesClient); err != nil {
					logger.Warn("Failed to reload symbol precisions after client swap", "error", err)
				} else {
					logger.Info("Successfully reloaded symbol precisions from Binance API for LIVE trading")
				}
			}

			// CRITICAL FIX: Update cfg to match the actual dry_run mode from saved settings
			// This ensures GetDryRunMode() returns the correct persisted mode, not the config file default
			cfg.TradingConfig.DryRun = actualDryRun
			logger.Info("Updated cfg.TradingConfig.DryRun to match saved settings",
				"dry_run", actualDryRun)
		}

		logger.Info("Futures Autopilot Controller initialized",
			"risk_level", cfg.FuturesAutopilotConfig.RiskLevel,
			"leverage", cfg.FuturesAutopilotConfig.DefaultLeverage,
			"dry_run", actualDryRun)

		// Ensure precision cache is loaded from real client for LIVE mode
		// (even if no swap was needed, the initial load may have used mock client)
		if !actualDryRun {
			autopilot.SetPrecisionClient(activeFuturesClient)
			// Re-load to ensure we have ALL symbols from Binance, not just mock's 30
			if err := autopilot.LoadSymbolPrecisions(activeFuturesClient); err != nil {
				logger.Warn("Failed to load symbol precisions from Binance", "error", err)
			}
		}

		// Initialize SymbolValidator for robust order validation
		// This syncs symbol requirements from Binance and stores in database
		symbolReqRepo := database.NewSymbolRequirementsRepository(db)
		symbolValidator := autopilot.GetSymbolValidator()
		if err := symbolValidator.Initialize(symbolReqRepo, activeFuturesClient); err != nil {
			logger.Warn("Failed to initialize symbol validator", "error", err)
		} else {
			logger.Info("Symbol validator initialized",
				"symbols_cached", symbolValidator.GetCacheSize())
			// Start periodic sync in background
			symbolValidator.StartPeriodicSync()
		}

		// Initialize InstanceControl for active/standby coordination (Story 9.6)
		// Only if Redis is enabled and healthy
		if cacheService != nil && cacheService.IsHealthy() {
			instanceID := getEnv("INSTANCE_ID", "standalone")
			activeByDefault := getEnvBool("ACTIVE_BY_DEFAULT", true)

			instanceControl := autopilot.NewInstanceControl(
				cacheService.GetClient(),
				instanceID,
				activeByDefault,
			)

			// Start the instance control (heartbeat + pub/sub)
			if err := instanceControl.Start(ctx); err != nil {
				logger.Warn("Failed to start instance control", "error", err)
			} else {
				// Wire it to FuturesController
				futuresAutopilotController.SetInstanceControl(instanceControl)

				logger.Info("Instance control initialized",
					"instance_id", instanceID,
					"is_active", instanceControl.IsActive(),
					"active_by_default", activeByDefault)
			}
		} else {
			logger.Info("Instance control not initialized - Redis not available, running in standalone mode")
		}
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

	// Initialize Futures WebSocket client for real-time market data
	var futuresWSClient *api.FuturesWSClient
	var klineSubscriptionManager *binance.KlineSubscriptionManager
	if cfg.FuturesConfig.Enabled && marketDataCache != nil {
		futuresWSClient = api.InitFuturesWebSocket(cfg.FuturesConfig.TestNet, wsHub)
		futuresWSClient.SetMarketDataCache(marketDataCache)

		// Initialize kline subscription manager for multi-timeframe support
		klineSubscriptionManager = binance.NewKlineSubscriptionManager()
		klineSubscriptionManager.SetSubscriber(futuresWSClient)

		// Build stream list based on allowed symbols
		var streams []string
		// Subscribe to all mark prices stream for real-time price updates
		streams = append(streams, "!markPrice@arr")

		// Subscribe to klines for monitored symbols - now with multiple timeframes!
		symbols := cfg.FuturesAutopilotConfig.AllowedSymbols
		if len(symbols) == 0 {
			symbols = []string{"BTCUSDT", "ETHUSDT"} // Default symbols
		}

		// Use subscription manager to build multi-timeframe stream list
		// This subscribes to 1m, 5m, 15m, 1h, 4h for each symbol
		klineStreams := klineSubscriptionManager.BuildStreamList(symbols)
		streams = append(streams, klineStreams...)

		// Track subscriptions in the manager for later dynamic updates
		for _, symbol := range symbols {
			klineSubscriptionManager.SubscribeSymbol(symbol)
		}

		// Connect to streams
		if err := futuresWSClient.Connect(streams); err != nil {
			logger.Warn("Failed to connect Futures WebSocket", "error", err)
		} else {
			logger.Info("Futures WebSocket connected with multi-timeframe klines",
				"streams", len(streams),
				"symbols", len(symbols),
				"timeframes", []string{"1m", "5m", "15m", "1h", "4h"})
		}
	}

	// Initialize User Data Stream for real-time account/position/order updates
	// This eliminates REST API polling for position changes
	var userDataStream *binance.UserDataStream
	if cfg.FuturesConfig.Enabled && futuresAutopilotController != nil {
		// Get the active futures client from the controller
		activeClient := futuresAutopilotController.GetFuturesClient()
		if activeClient != nil {
			userDataStream = binance.NewUserDataStream(activeClient, cfg.FuturesConfig.TestNet)

			// Wire up callbacks to Ginie autopilot controller
			userDataStream.SetPositionUpdateCallback(func(update *binance.PositionUpdateEvent) {
				futuresAutopilotController.HandleStreamPositionUpdate(update)
			})

			userDataStream.SetOrderUpdateCallback(func(update *binance.OrderUpdateEvent) {
				futuresAutopilotController.HandleStreamOrderUpdate(update)
			})

			userDataStream.SetAccountUpdateCallback(func(update *binance.AccountUpdateEvent) {
				futuresAutopilotController.HandleStreamAccountUpdate(update)
			})

			// Start the stream (non-blocking)
			if err := userDataStream.Start(); err != nil {
				logger.Warn("Failed to start User Data Stream",
					"error", err,
					"dry_run", cfg.TradingConfig.DryRun)
			} else {
				logger.Info("User Data Stream started for real-time updates",
					"testnet", cfg.FuturesConfig.TestNet,
					"dry_run", cfg.TradingConfig.DryRun)
			}
		}
	}

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
		// AI components
		circuitBreaker:             circuitBreaker,
		autopilotController:        autopilotController,
		futuresAutopilotController: futuresAutopilotController,
		mlPredictor:                mlPredictor,
		llmAnalyzer:                llmAnalyzer,
		sentimentAnalyzer:          sentimentAnalyzer,
		// Futures trading - mock for paper, real created per-request from user keys
		futuresMockClient: futuresMockClient,
		futuresRealClient: nil, // No global real client - created per-request from user API keys
		// Spot trading - mock for paper, real created per-request from user keys
		spotMockClient: spotMockClient,
		spotRealClient: nil, // No global real client - created per-request from user API keys
		// Market data cache
		marketDataCache: marketDataCache,
		// API key service for per-user client creation during mode switch
		apiKeyService: apiKeyService,
		repo:          earlyRepo,
	}

	// Initialize email service
	emailService := email.NewService(repo)
	logger.Info("Email service initialized")

	// Initialize auth service if enabled
	var authService *auth.Service
	if cfg.AuthConfig.Enabled {
		if cfg.AuthConfig.JWTSecret == "" {
			log.Fatalf("AUTH_ENABLED is true but AUTH_JWT_SECRET is not set")
		}
		authConfig := auth.Config{
			JWTSecret:                cfg.AuthConfig.JWTSecret,
			AccessTokenDuration:      cfg.AuthConfig.AccessTokenDuration,
			RefreshTokenDuration:     cfg.AuthConfig.RefreshTokenDuration,
			PasswordResetDuration:    cfg.AuthConfig.PasswordResetDuration,
			MinPasswordLength:        cfg.AuthConfig.MinPasswordLength,
			RequireEmailVerification: cfg.AuthConfig.RequireEmailVerification,
			MaxSessionsPerUser:       cfg.AuthConfig.MaxSessionsPerUser,
		}
		authService = auth.NewServiceWithEmail(repo, authConfig, emailService, eventBus)
		logger.Info("Authentication service initialized", "email_verification", cfg.AuthConfig.RequireEmailVerification)
	} else {
		logger.Info("Authentication disabled - running in single-user mode")
	}

	// Initialize Vault client if enabled
	var vaultClient *vault.Client
	if cfg.VaultConfig.Enabled {
		var err error
		vaultClient, err = vault.NewClient(cfg.VaultConfig)
		if err != nil {
			log.Printf("Warning: Failed to initialize Vault client: %v", err)
		} else {
			logger.Info("Vault client initialized", "address", cfg.VaultConfig.Address)
		}
	}

	// Initialize Client Factory for per-user Binance clients
	var clientFactory *binance.ClientFactory
	if vaultClient != nil {
		var err error
		clientFactory, err = binance.NewClientFactory(vaultClient, cfg.BinanceConfig, cfg.FuturesConfig)
		if err != nil {
			log.Printf("Warning: Failed to initialize Client Factory: %v", err)
		} else {
			logger.Info("Client Factory initialized for per-user Binance clients")
		}
	}

	// Initialize UserAutopilotManager for multi-user simultaneous trading
	// This enables each user to have their own independent autopilot instance
	// Note: clientFactory is optional - manager can use apiKeyService directly from database
	var userAutopilotManager *autopilot.UserAutopilotManager
	if futuresAutopilotController != nil {
		// Create LLM config for per-user AI analyzers
		llmConfig := llm.DefaultAnalyzerConfig()
		llmConfig.Provider = llm.Provider(cfg.AIConfig.LLMProvider)
		llmConfig.Model = cfg.AIConfig.LLMModel

		// Create the multi-user autopilot manager
		userAutopilotLogger := logging.New(&logging.Config{
			Level:       cfg.LoggingConfig.Level,
			Output:      cfg.LoggingConfig.Output,
			JSONFormat:  cfg.LoggingConfig.JSONFormat,
			IncludeFile: cfg.LoggingConfig.IncludeFile,
			Component:   "user_autopilot",
		})

		userAutopilotManager = autopilot.NewUserAutopilotManager(
			repo,
			futuresAutopilotController.GetGinieAnalyzer(),
			clientFactory,
			apiKeyService,
			llmConfig,
			userAutopilotLogger,
		)

		// Set manager on FuturesController for access in handlers
		futuresAutopilotController.SetUserAutopilotManager(userAutopilotManager)

		logger.Info("UserAutopilotManager initialized for multi-user trading")
	}

	// Subscribe to user logout events for cleanup (after all components are initialized)
	eventBus.Subscribe(events.EventUserLogout, func(event events.Event) {
		userID, ok := event.Data["user_id"].(string)
		if !ok || userID == "" {
			logger.Warn("User logout event missing user_id")
			return
		}

		logger.Info("User logout cleanup triggered", "user_id", userID)

		// Stop user's autopilot if running
		if userAutopilotManager != nil {
			if err := userAutopilotManager.StopAutopilot(userID); err != nil {
				logger.WithError(err).Warn("Failed to stop user autopilot on logout", "user_id", userID)
			} else {
				logger.Info("Stopped user autopilot on logout", "user_id", userID)
			}
		}

		// Invalidate user's Binance client
		if clientFactory != nil {
			clientFactory.InvalidateClient(userID)
			logger.Info("Invalidated user Binance client on logout", "user_id", userID)
		}

		// Disconnect user's WebSocket
		if wsHub != nil {
			wsHub.DisconnectUser(userID)
			logger.Info("Disconnected user WebSocket on logout", "user_id", userID)
		}
	})

	// Initialize Billing service if enabled
	var billingService *billing.StripeService
	if cfg.BillingConfig.Enabled {
		billingService = billing.NewStripeService(&billing.StripeConfig{
			SecretKey:      cfg.BillingConfig.StripeSecretKey,
			PublishableKey: cfg.BillingConfig.StripePublishableKey,
			WebhookSecret:  cfg.BillingConfig.StripeWebhookSecret,
		}, repo)
		logger.Info("Billing service initialized")
	}

	// Initialize License validation
	licenseInfo, err := license.GetLicenseFromEnv()
	if err != nil {
		logger.Warn("License validation failed, running in trial mode", "error", err)
	} else if licenseInfo != nil && licenseInfo.IsValid {
		logger.Info("License validated",
			"type", licenseInfo.Type,
			"max_symbols", licenseInfo.MaxSymbols,
			"features", len(licenseInfo.Features),
		)
	}

	server := api.NewServer(serverConfig, repo, eventBus, botAPI, authService, vaultClient, billingService, licenseInfo)

	// Set the UserAutopilotManager on the server for per-user autopilot handling
	if userAutopilotManager != nil {
		server.SetUserAutopilotManager(userAutopilotManager)
		logger.Info("UserAutopilotManager set on API server")
	}

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

	// Start autopilot if enabled
	if autopilotController != nil {
		if err := autopilotController.Start(); err != nil {
			logger.Warn("Failed to start autopilot", "error", err)
		} else {
			logger.Info("Autopilot started")
		}
	}

	// Start futures autopilot if enabled
	if futuresAutopilotController != nil {
		if err := futuresAutopilotController.Start(); err != nil {
			logger.Warn("Failed to start futures autopilot", "error", err)
		} else {
			logger.Info("Futures autopilot started")
		}
	}

	// Auto-start Ginie for user if enabled in saved settings
	// This restores Ginie state from before server restart
	if userAutopilotManager != nil {
		go func() {
			// Wait a moment for all services to be fully initialized
			time.Sleep(3 * time.Second)
			ctx := context.Background()
			if err := userAutopilotManager.AutoStartFromSettings(ctx); err != nil {
				logger.Warn("Failed to auto-start Ginie from settings", "error", err)
			}
		}()
	}

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

	// Stop AI components
	if autopilotController != nil {
		autopilotController.Stop()
		logger.Info("Autopilot stopped")
	}
	if futuresAutopilotController != nil {
		// Stop instance control first (gracefully deactivate before shutdown)
		if ic := futuresAutopilotController.GetInstanceControl(); ic != nil {
			ic.Stop()
			logger.Info("Instance control stopped")
		}
		futuresAutopilotController.Stop()
		logger.Info("Futures autopilot stopped")
	}
	if userAutopilotManager != nil {
		userAutopilotManager.Shutdown()
		logger.Info("UserAutopilotManager stopped")
	}
	if sentimentAnalyzer != nil {
		sentimentAnalyzer.Stop()
		logger.Info("Sentiment analyzer stopped")
	}

	// Stop User Data Stream
	if userDataStream != nil {
		userDataStream.Stop()
		logger.Info("User Data Stream stopped")
	}

	// Stop Futures WebSocket
	if futuresWSClient != nil {
		futuresWSClient.Close()
		logger.Info("Futures WebSocket closed")
	}

	// Stop bot, screener, and scanner
	tradingBot.Stop()
	coinScreener.Stop()
	strategyScanner.Stop()

	// Close Redis cache service
	if cacheService != nil {
		if err := cacheService.Close(); err != nil {
			logger.Warn("Error closing cache service", "error", err)
		} else {
			logger.Info("Cache service closed")
		}
	}

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
	// Enhanced with EMA trend filter, volume confirmation, and RSI filter
	breakoutStrategy := strategy.NewBreakoutStrategy(&strategy.BreakoutConfig{
		Symbol:        "BTCUSDT",
		Interval:      "15m",
		OrderType:     "LIMIT",
		OrderSide:     "BUY",
		PositionSize:  0.01,  // 1% of balance
		StopLoss:      0.025, // 2.5% (increased from 2% to reduce noise stops)
		TakeProfit:    0.05,  // 5% (2:1 reward/risk ratio)
	})
	bot.RegisterStrategy("breakout_high", breakoutStrategy)

	// Support test strategy - when price comes to last candle's low
	// Enhanced with trend filter, RSI filter, and bounce confirmation
	supportStrategy := strategy.NewSupportStrategy(&strategy.SupportConfig{
		Symbol:        "ETHUSDT",
		Interval:      "15m",
		OrderType:     "LIMIT",
		OrderSide:     "BUY",
		PositionSize:  0.01,
		StopLoss:      0.025, // 2.5% (increased from 2% to reduce noise stops)
		TakeProfit:    0.05,  // 5% (2:1 reward/risk ratio)
		TouchDistance: 0.001, // 0.1% distance to consider "touching" the low
	})
	bot.RegisterStrategy("support_low", supportStrategy)

	fmt.Println("Enhanced strategies registered (EMA + RSI + Volume filters enabled)")
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
	// AI components
	circuitBreaker              *circuit.CircuitBreaker
	autopilotController         *autopilot.Controller
	futuresAutopilotController  *autopilot.FuturesController
	mlPredictor                 *ml.Predictor
	llmAnalyzer                 *llm.Analyzer
	sentimentAnalyzer           *sentiment.Analyzer
	// Futures trading - both clients for dynamic switching
	futuresMockClient binance.FuturesClient
	futuresRealClient binance.FuturesClient
	// Spot trading - both clients for dynamic switching
	spotMockClient binance.BinanceClient
	spotRealClient binance.BinanceClient
	// Market data cache for WebSocket data
	marketDataCache *binance.MarketDataCache
	// API key service for creating per-user clients during mode switch
	apiKeyService *apikeys.Service
	repo          *database.Repository
}

func (w *BotAPIWrapper) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running":          true,
		"dry_run":          w.cfg.TradingConfig.DryRun,
		"testnet":          w.cfg.BinanceConfig.TestNet,
		"strategies_count": 2,
		"open_positions":   0,
	}

	// Add AI component status
	aiStatus := map[string]interface{}{
		"enabled": w.cfg.AIConfig.Enabled,
	}

	if w.circuitBreaker != nil {
		aiStatus["circuit_breaker"] = w.circuitBreaker.GetStats()
	}

	if w.autopilotController != nil {
		aiStatus["autopilot"] = map[string]interface{}{
			"running": w.autopilotController.IsRunning(),
			"stats":   w.autopilotController.GetStats(),
		}
	}

	if w.sentimentAnalyzer != nil {
		if sentiment := w.sentimentAnalyzer.GetSentiment(); sentiment != nil {
			aiStatus["sentiment"] = sentiment
		}
	}

	status["ai"] = aiStatus
	return status
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

func (w *BotAPIWrapper) GetFuturesClient() binance.FuturesClient {
	// Use FuturesController's actual client if available
	// The client is already wrapped with cache when passed to FuturesController
	if w.futuresAutopilotController != nil {
		client := w.futuresAutopilotController.GetFuturesClient()
		if client != nil {
			return client
		}
	}

	// Get base client based on dry_run mode
	// In live mode, real client must be created per-request from user API keys
	var baseClient binance.FuturesClient
	if w.cfg.TradingConfig.DryRun {
		baseClient = w.futuresMockClient
	} else {
		// Live mode but no user-specific client was found
		// Return nil to indicate user must configure API keys
		return nil
	}

	// Wrap with cached client if cache is available
	if w.marketDataCache != nil {
		return binance.NewCachedFuturesClient(baseClient, w.marketDataCache)
	}
	return baseClient
}

// GetSpotClient returns the appropriate spot client based on trading mode
func (w *BotAPIWrapper) GetSpotClient() binance.BinanceClient {
	// Return appropriate client based on dry_run mode
	if w.cfg.TradingConfig.DryRun {
		return w.spotMockClient
	}
	// Live mode - real client must be created per-request from user API keys
	// Return nil to indicate user must configure API keys
	return nil
}

// SettingsAPI interface implementation

func (w *BotAPIWrapper) GetAutopilotController() *autopilot.Controller {
	return w.autopilotController
}

func (w *BotAPIWrapper) GetFuturesAutopilot() interface{} {
	if w.futuresAutopilotController == nil {
		return nil
	}
	return w.futuresAutopilotController
}

func (w *BotAPIWrapper) GetCircuitBreaker() *circuit.CircuitBreaker {
	return w.circuitBreaker
}

func (w *BotAPIWrapper) GetDryRunMode() bool {
	return w.cfg.TradingConfig.DryRun
}

func (w *BotAPIWrapper) SetDryRunMode(enabled bool) error {
	oldMode := w.cfg.TradingConfig.DryRun
	modeStr := "PAPER"
	if !enabled {
		modeStr = "LIVE"
	}

	// Log the mode change request
	w.logger.Info("Mode change requested",
		"from_mode", map[bool]string{true: "PAPER", false: "LIVE"}[oldMode],
		"to_mode", modeStr,
		"mode_changed", oldMode != enabled)

	// CRITICAL FIX: Load settings ONCE at the start to prevent race condition
	// Previously we were loading and saving twice, creating a window where
	// another thread could call LoadSavedSettings() and revert the mode
	sm := autopilot.GetSettingsManager()
	settings, err := sm.LoadSettings()
	if err != nil {
		w.logger.Warn("Failed to load settings for mode update", "error", err)
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Update both main dry run mode and Ginie dry run mode together
	oldDryRunMode := settings.DryRunMode
	oldGinieDryRunMode := settings.GinieDryRunMode

	settings.DryRunMode = enabled
	settings.GinieDryRunMode = enabled
	settings.SpotDryRunMode = enabled

	w.logger.Info("Updating settings file",
		"old_dry_run", oldDryRunMode,
		"new_dry_run", enabled,
		"old_ginie_dry_run", oldGinieDryRunMode,
		"new_ginie_dry_run", enabled)

	// Save to persistent storage IMMEDIATELY to prevent LoadSavedSettings from reading stale data
	if err := sm.SaveSettings(settings); err != nil {
		w.logger.Error("Failed to save settings after mode change", "error", err)
		return fmt.Errorf("failed to save settings: %w", err)
	}

	w.logger.Info("Successfully saved trading mode to settings file",
		"mode", modeStr,
		"dry_run", enabled,
		"dry_run_mode_saved", settings.DryRunMode,
		"ginie_dry_run_mode_saved", settings.GinieDryRunMode)

	// Update all mode fields AFTER persistent save to prevent race condition
	w.cfg.TradingConfig.DryRun = enabled
	w.logger.Info("Updated BotAPIWrapper config", "dry_run", enabled)

	// Update Spot autopilot if it exists
	if w.autopilotController != nil {
		w.autopilotController.SetDryRun(enabled)
		w.logger.Info("Updated Spot autopilot mode", "dry_run", enabled)
	}

	// Update Futures autopilot and switch client with timeout protection
	if w.futuresAutopilotController != nil {
		// Use a context with timeout to prevent hanging during client switch
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Run client switch in a goroutine with timeout
		done := make(chan error, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					w.logger.Error("Panic during futures client switch", "panic", r)
					done <- fmt.Errorf("panic during client switch: %v", r)
				}
			}()

			// Switch the futures client based on new mode
			var newClient binance.FuturesClient
			if enabled {
				// Switching to PAPER mode - use mock client
				if w.futuresMockClient != nil {
					newClient = w.futuresMockClient
					w.logger.Info("Selecting mock client for PAPER mode")
				}
			} else {
				// Switching to LIVE mode - create real client from owner's API keys
				ownerUserID := w.futuresAutopilotController.GetOwnerUserID()
				if ownerUserID == "" {
					w.logger.Warn("LIVE mode enabled but no autopilot owner set - trades will fail until user starts autopilot")
					newClient = nil
				} else if w.apiKeyService == nil {
					w.logger.Warn("LIVE mode enabled but no API key service available - using mock client")
					newClient = w.futuresMockClient
				} else {
					// Get owner's Binance API keys from database
					ctx := context.Background()
					keys, err := w.apiKeyService.GetActiveBinanceKey(ctx, ownerUserID, false)
					if err != nil {
						// Try testnet
						keys, err = w.apiKeyService.GetActiveBinanceKey(ctx, ownerUserID, true)
					}
					if err != nil || keys == nil || keys.APIKey == "" {
						w.logger.Error("LIVE mode enabled but failed to get owner's API keys",
							"owner_user_id", ownerUserID,
							"error", err)
						newClient = nil
					} else {
						// Create real client from owner's API keys
						newClient = binance.NewFuturesClient(keys.APIKey, keys.SecretKey, keys.IsTestnet)
						w.logger.Info("LIVE mode - created real client from owner's API keys",
							"owner_user_id", ownerUserID,
							"testnet", keys.IsTestnet)
					}
				}
			}

			// Wrap with cache if available
			if newClient != nil && w.marketDataCache != nil {
				newClient = binance.NewCachedFuturesClient(newClient, w.marketDataCache)
				w.logger.Info("Wrapped futures client with cache")
			}

			if newClient != nil {
				w.futuresAutopilotController.SetFuturesClient(newClient)
				w.logger.Info("Set futures controller client",
					"mode", modeStr,
					"client_type", map[bool]string{true: "mock", false: "real"}[enabled])
			}

			// Update dry run flag (this will also propagate to Ginie)
			w.futuresAutopilotController.SetDryRun(enabled)
			w.logger.Info("Updated FuturesController dry_run", "dry_run", enabled)

			done <- nil
		}()

		// Wait for client switch to complete with timeout
		select {
		case <-ctx.Done():
			w.logger.Error("Futures client switch TIMEOUT - exceeded 5 seconds",
				"timeout_reason", ctx.Err(),
				"mode", modeStr)
			// Don't fail entirely - the goroutine will continue in background
			// but we'll proceed with settings persistence
		case err := <-done:
			if err != nil {
				w.logger.Error("Futures client switch failed", "error", err)
				// Continue anyway - settings will still be persisted
			} else {
				w.logger.Info("Futures client switch completed successfully")
			}
		}
	}

	w.logger.Info("Trading mode changed successfully", "mode", modeStr, "dry_run", enabled)
	return nil
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

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func strPtr(s string) *string {
	return &s
}
