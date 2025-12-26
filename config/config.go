package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BinanceConfig          BinanceConfig          `json:"binance"`
	FuturesConfig          FuturesConfig          `json:"futures"`
	ScreenerConfig         ScreenerConfig         `json:"screener"`
	TradingConfig          TradingConfig          `json:"trading"`
	ScannerConfig          ScannerConfig          `json:"scanner"`
	NotificationConfig     NotificationConfig     `json:"notification"`
	RiskConfig             RiskConfig             `json:"risk"`
	LoggingConfig          LoggingConfig          `json:"logging"`
	AIConfig               AIConfig               `json:"ai"`
	AutopilotConfig        AutopilotConfig        `json:"autopilot"`
	FuturesAutopilotConfig FuturesAutopilotConfig `json:"futures_autopilot"`
	ScalpingConfig         ScalpingConfig         `json:"scalping"`
	BigCandleConfig        BigCandleConfig        `json:"big_candle"`
	CircuitBreakerConfig   CircuitBreakerConfig   `json:"circuit_breaker"`
	// Multi-tenant SaaS configs
	ServerConfig  ServerConfig  `json:"server"`
	AuthConfig    AuthConfig    `json:"auth"`
	VaultConfig   VaultConfig   `json:"vault"`
	BillingConfig BillingConfig `json:"billing"`
	RedisConfig   RedisConfig   `json:"redis"`
}

// FuturesConfig holds Binance Futures trading configuration
type FuturesConfig struct {
	Enabled           bool   `json:"enabled"`
	TestNet           bool   `json:"testnet"`
	DefaultLeverage   int    `json:"default_leverage"`
	DefaultMarginType string `json:"default_margin_type"` // CROSSED or ISOLATED
	PositionMode      string `json:"position_mode"`       // ONE_WAY or HEDGE
	MaxLeverage       int    `json:"max_leverage"`
}

type LoggingConfig struct {
	Level       string `json:"level"`        // DEBUG, INFO, WARN, ERROR
	Output      string `json:"output"`       // stdout, stderr, or file path
	JSONFormat  bool   `json:"json_format"`  // Output as JSON
	IncludeFile bool   `json:"include_file"` // Include file and line number
}

type BinanceConfig struct {
	APIKey    string `json:"api_key"`
	SecretKey string `json:"secret_key"`
	BaseURL   string `json:"base_url"`
	TestNet   bool   `json:"testnet"`
	MockMode  bool   `json:"mock_mode"` // Use simulated data when Binance API is unavailable
}

type ScreenerConfig struct {
	Enabled          bool     `json:"enabled"`
	Interval         string   `json:"interval"`         // e.g., "15m", "1h"
	MinVolume        float64  `json:"min_volume"`       // Minimum 24h volume in USDT
	MinPriceChange   float64  `json:"min_price_change"` // Minimum price change %
	ExcludeSymbols   []string `json:"exclude_symbols"`
	QuoteCurrency    string   `json:"quote_currency"` // "USDT", "BTC", etc.
	MaxSymbols       int      `json:"max_symbols"`    // Max symbols to screen
	ScreeningInterval int     `json:"screening_interval"` // Seconds between screens
}

type TradingConfig struct {
	MaxOpenPositions int     `json:"max_open_positions"`
	MaxRiskPerTrade  float64 `json:"max_risk_per_trade"` // As percentage
	DryRun           bool    `json:"dry_run"`            // Test mode without real orders
}

type ScannerConfig struct {
	Enabled          bool   `json:"enabled"`            // Enable/disable scanner
	ScanInterval     int    `json:"scan_interval"`      // Seconds between scans
	MaxSymbols       int    `json:"max_symbols"`        // Max results to show
	IncludeWatchlist bool   `json:"include_watchlist"`  // Include watchlist symbols
	CacheTTL         int    `json:"cache_ttl"`          // Cache TTL in seconds
	WorkerCount      int    `json:"worker_count"`       // Concurrent worker count
}

type NotificationConfig struct {
	Enabled  bool           `json:"enabled"`
	Telegram TelegramConfig `json:"telegram"`
	Discord  DiscordConfig  `json:"discord"`
}

type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type DiscordConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
}

type RiskConfig struct {
	MaxRiskPerTrade        float64 `json:"max_risk_per_trade"`        // Percentage of account to risk per trade
	MaxDailyDrawdown       float64 `json:"max_daily_drawdown"`        // Max daily loss percentage before stopping
	MaxOpenPositions       int     `json:"max_open_positions"`        // Maximum concurrent positions
	PositionSizeMethod     string  `json:"position_size_method"`      // "fixed", "percent", "kelly"
	FixedPositionSize      float64 `json:"fixed_position_size"`       // Fixed position size in quote currency
	UseTrailingStop        bool    `json:"use_trailing_stop"`         // Enable trailing stop loss
	TrailingStopPercent    float64 `json:"trailing_stop_percent"`     // Trailing stop distance percentage
	TrailingStopActivation float64 `json:"trailing_stop_activation"`  // Profit % to activate trailing stop
}

// AIConfig holds AI/ML configuration
type AIConfig struct {
	Enabled          bool   `json:"enabled"`
	LLMProvider      string `json:"llm_provider"`      // "claude", "openai", or "deepseek"
	ClaudeAPIKey     string `json:"claude_api_key"`
	OpenAIAPIKey     string `json:"openai_api_key"`
	DeepSeekAPIKey   string `json:"deepseek_api_key"`
	LLMModel         string `json:"llm_model"`         // e.g., "claude-3-opus", "gpt-4", "deepseek-chat"
	MLEnabled        bool   `json:"ml_enabled"`        // Enable ML predictions
	SentimentEnabled bool   `json:"sentiment_enabled"` // Enable sentiment analysis
}

// AutopilotConfig holds autopilot trading configuration
type AutopilotConfig struct {
	Enabled            bool     `json:"enabled"`
	RiskLevel          string   `json:"risk_level"`           // "conservative", "moderate", "aggressive"
	MaxDailyTrades     int      `json:"max_daily_trades"`
	MaxDailyLoss       float64  `json:"max_daily_loss"`       // Percentage
	MaxPositionSize    float64  `json:"max_position_size"`    // Percentage of portfolio
	MinConfidence      float64  `json:"min_confidence"`       // Minimum AI confidence to trade (0-1)
	RequireConfluence  int      `json:"require_confluence"`   // Minimum signals that must agree
	AllowedSymbols     []string `json:"allowed_symbols"`      // Empty = all symbols
	// New allocation and profit reinvestment settings
	MaxUSDAllocation        float64 `json:"max_usd_allocation"`         // Maximum USD to allocate for trading
	ProfitReinvestPercent   float64 `json:"profit_reinvest_percent"`    // Percentage of profit to reinvest (0-100)
	ProfitReinvestRiskLevel string  `json:"profit_reinvest_risk_level"` // Risk level for reinvested profits
}

// FuturesAutopilotConfig holds futures autopilot trading configuration
type FuturesAutopilotConfig struct {
	Enabled              bool     `json:"enabled"`
	DryRun               bool     `json:"dry_run"`                // Test mode without real orders (overrides TradingConfig if set)
	RiskLevel            string   `json:"risk_level"`             // "conservative", "moderate", "aggressive"
	MaxDailyTrades       int      `json:"max_daily_trades"`
	MaxDailyLoss         float64  `json:"max_daily_loss"`         // Percentage
	MaxPositionSize      float64  `json:"max_position_size"`      // Percentage of portfolio
	MinConfidence        float64  `json:"min_confidence"`         // Minimum AI confidence (0-1)
	RequireConfluence    int      `json:"require_confluence"`     // Minimum signals that must agree
	AllowedSymbols       []string `json:"allowed_symbols"`        // Empty = all symbols
	DefaultLeverage      int      `json:"default_leverage"`       // Default leverage (1-125)
	MaxLeverage          int      `json:"max_leverage"`           // Maximum allowed leverage
	MarginType           string   `json:"margin_type"`            // "CROSSED" or "ISOLATED"
	PositionMode         string   `json:"position_mode"`          // "ONE_WAY" or "HEDGE"
	LiquidationBuffer    float64  `json:"liquidation_buffer"`     // % buffer from liquidation price
	MaxFundingRate       float64  `json:"max_funding_rate"`       // Max funding rate to hold (avoid high carry cost)
	AllowShorts          bool     `json:"allow_shorts"`           // Allow short positions
	AutoReduceRisk       bool     `json:"auto_reduce_risk"`       // Auto-reduce position near liquidation
	TakeProfitPercent    float64  `json:"take_profit_percent"`     // Default TP %
	TakeProfitPercent1   float64  `json:"take_profit_percent_1"`   // TP1 level (10% ROI)
	TakeProfitPercent2   float64  `json:"take_profit_percent_2"`   // TP2 level (30% ROI)
	StopLossPercent      float64  `json:"stop_loss_percent"`       // Default SL %
	TrailingStopEnabled  bool     `json:"trailing_stop_enabled"`   // Enable trailing stop
	TrailingStopPercent  float64  `json:"trailing_stop_percent"`   // Trailing stop pullback % (8%)
	TrailingStopActivationPercent float64 `json:"trailing_stop_activation_percent"` // Activate at % of TP2 (60%)
	DecisionIntervalSecs int      `json:"decision_interval_secs"` // Seconds between decisions
	// New allocation and profit reinvestment settings
	MaxUSDAllocation        float64 `json:"max_usd_allocation"`         // Maximum USD to allocate for trading
	ProfitReinvestPercent   float64 `json:"profit_reinvest_percent"`    // Percentage of profit to reinvest (0-100)
	ProfitReinvestRiskLevel string  `json:"profit_reinvest_risk_level"` // Risk level for reinvested profits
	// Position averaging settings
	AveragingEnabled         bool    `json:"averaging_enabled"`           // Enable AI-driven position averaging
	MaxEntriesPerPosition    int     `json:"max_entries_per_position"`    // Max entries per position (default 3)
	AveragingMinConfidence   float64 `json:"averaging_min_confidence"`    // Min confidence for averaging (default 0.80)
	AveragingMinPriceImprove float64 `json:"averaging_min_price_improve"` // Min % price improvement required
	AveragingCooldownMins    int     `json:"averaging_cooldown_mins"`     // Cooldown between averaging (default 15)
	AveragingNewsWeight      float64 `json:"averaging_news_weight"`       // Weight of news sentiment (0-1)
	// Dynamic SL/TP settings (volatility-based per coin)
	DynamicSLTPEnabled  bool    `json:"dynamic_sltp_enabled"`   // Enable dynamic SL/TP based on ATR+LLM
	ATRPeriod           int     `json:"atr_period"`             // ATR calculation period (default 14)
	ATRMultiplierSL     float64 `json:"atr_multiplier_sl"`      // ATR multiplier for stop loss (default 1.5)
	ATRMultiplierTP     float64 `json:"atr_multiplier_tp"`      // ATR multiplier for take profit (default 2.0)
	LLMSLTPWeight       float64 `json:"llm_sltp_weight"`        // Weight for LLM SL/TP suggestions (0-1)
	MinSLPercent        float64 `json:"min_sl_percent"`         // Minimum SL percentage floor (default 0.3)
	MaxSLPercent        float64 `json:"max_sl_percent"`         // Maximum SL percentage cap (default 3.0)
	MinTPPercent        float64 `json:"min_tp_percent"`         // Minimum TP percentage floor (default 0.5)
	MaxTPPercent        float64 `json:"max_tp_percent"`         // Maximum TP percentage cap (default 5.0)
	// Scalping mode settings (aggressive small profit booking)
	ScalpingModeEnabled     bool    `json:"scalping_mode_enabled"`      // Enable scalping mode for quick profits
	ScalpingMinProfit       float64 `json:"scalping_min_profit"`        // Minimum profit % to book (e.g., 0.1)
	ScalpingQuickReentry    bool    `json:"scalping_quick_reentry"`     // Re-enter immediately after close
	ScalpingReentryDelaySec int     `json:"scalping_reentry_delay_sec"` // Delay before re-entry in seconds
	ScalpingMaxTradesPerDay int     `json:"scalping_max_trades_per_day"`// Max scalping trades per day (0 = unlimited)
	// Hedging configuration (opposite position hedging)
	HedgingEnabled              bool      `json:"hedging_enabled"`                // Master toggle for hedging
	HedgePriceDropTriggerPct    float64   `json:"hedge_price_drop_trigger_pct"`   // Trigger hedge when position drops X% (e.g., 5.0)
	HedgeUnrealizedLossTrigger  float64   `json:"hedge_unrealized_loss_trigger"`  // Trigger hedge when unrealized loss exceeds $X
	HedgeAIEnabled              bool      `json:"hedge_ai_enabled"`               // Let LLM recommend hedges
	HedgeAIConfidenceMin        float64   `json:"hedge_ai_confidence_min"`        // Min confidence for AI hedge recommendation
	HedgeDefaultPercent         float64   `json:"hedge_default_percent"`          // Default hedge size as % of position (e.g., 50)
	HedgePartialSteps           []float64 `json:"hedge_partial_steps"`            // Graduated hedge steps [25, 50, 75, 100]
	HedgeProfitTakePct          float64   `json:"hedge_profit_take_pct"`          // Close hedge when it's X% profitable
	HedgeCloseOnRecoveryPct     float64   `json:"hedge_close_on_recovery_pct"`    // Close hedge when original recovers X%
	HedgeMaxSimultaneous        int       `json:"hedge_max_simultaneous"`         // Max simultaneous hedges
}

// ScalpingConfig holds scalping strategy configuration
type ScalpingConfig struct {
	Enabled          bool      `json:"enabled"`
	Timeframes       []string  `json:"timeframes"`        // ["1s", "2s", "5s", "10s", "30s", "60s"]
	MinProfitPercent float64   `json:"min_profit_percent"` // Minimum profit to take (e.g., 0.05)
	MaxLossPercent   float64   `json:"max_loss_percent"`   // Maximum loss before exit
	MaxHoldSeconds   int       `json:"max_hold_seconds"`   // Maximum time to hold position
	MaxConcurrent    int       `json:"max_concurrent"`     // Maximum concurrent scalp trades
	MinVolume        float64   `json:"min_volume"`         // Minimum volume requirement
	UseMLPrediction  bool      `json:"use_ml_prediction"`  // Use ML for entry timing
}

// BigCandleConfig holds big candle detection configuration
type BigCandleConfig struct {
	Enabled            bool    `json:"enabled"`
	SizeMultiplier     float64 `json:"size_multiplier"`      // 1.5x to 2x of average candle
	LookbackPeriod     int     `json:"lookback_period"`      // Candles to calculate average
	VolumeConfirmation bool    `json:"volume_confirmation"`  // Require volume spike
	ReactImmediately   bool    `json:"react_immediately"`    // Enter on detection
	MinVolumeRatio     float64 `json:"min_volume_ratio"`     // Minimum volume ratio (e.g., 1.5)
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Enabled              bool    `json:"enabled"`
	MaxLossPerHour       float64 `json:"max_loss_per_hour"`       // Max loss % per hour
	MaxConsecutiveLosses int     `json:"max_consecutive_losses"`  // Max losing trades in a row
	CooldownMinutes      int     `json:"cooldown_minutes"`        // Cooldown after trip
	MaxTradesPerMinute   int     `json:"max_trades_per_minute"`   // Rate limit
	MaxDailyLoss         float64 `json:"max_daily_loss"`          // Max daily loss %
	MaxDailyTrades       int     `json:"max_daily_trades"`        // Max trades per day
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port            int    `json:"port"`
	Host            string `json:"host"`
	AllowedOrigins  string `json:"allowed_origins"`  // CORS allowed origins
	TLSEnabled      bool   `json:"tls_enabled"`
	TLSCertFile     string `json:"tls_cert_file"`
	TLSKeyFile      string `json:"tls_key_file"`
	ReadTimeout     int    `json:"read_timeout"`     // Seconds
	WriteTimeout    int    `json:"write_timeout"`    // Seconds
	ShutdownTimeout int    `json:"shutdown_timeout"` // Seconds
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled                  bool          `json:"enabled"`
	JWTSecret                string        `json:"jwt_secret"`
	AccessTokenDuration      time.Duration `json:"access_token_duration"`
	RefreshTokenDuration     time.Duration `json:"refresh_token_duration"`
	PasswordResetDuration    time.Duration `json:"password_reset_duration"`
	MinPasswordLength        int           `json:"min_password_length"`
	RequireEmailVerification bool          `json:"require_email_verification"`
	MaxLoginAttempts         int           `json:"max_login_attempts"`
	LockoutDuration          time.Duration `json:"lockout_duration"`
	SessionCleanupInterval   time.Duration `json:"session_cleanup_interval"`
	AllowMultipleSessions    bool          `json:"allow_multiple_sessions"`
	MaxSessionsPerUser       int           `json:"max_sessions_per_user"`
}

// VaultConfig holds HashiCorp Vault configuration
type VaultConfig struct {
	Enabled    bool   `json:"enabled"`
	Address    string `json:"address"`
	Token      string `json:"token"`
	MountPath  string `json:"mount_path"`   // KV secrets engine mount path
	SecretPath string `json:"secret_path"`  // Path prefix for API keys
	TLSEnabled bool   `json:"tls_enabled"`
	CACert     string `json:"ca_cert"`
}

// BillingConfig holds billing and subscription configuration
type BillingConfig struct {
	Enabled               bool    `json:"enabled"`
	StripeSecretKey       string  `json:"stripe_secret_key"`
	StripePublishableKey  string  `json:"stripe_publishable_key"`
	StripeWebhookSecret   string  `json:"stripe_webhook_secret"`
	CryptoPaymentsEnabled bool    `json:"crypto_payments_enabled"`
	CryptoWalletAddress   string  `json:"crypto_wallet_address"`
	SettlementDayOfWeek   int     `json:"settlement_day_of_week"` // 0=Sunday, 1=Monday, etc.
	SettlementHourUTC     int     `json:"settlement_hour_utc"`
	MinimumPayout         float64 `json:"minimum_payout"`           // Minimum profit share to invoice
	GracePeriodDays       int     `json:"grace_period_days"`        // Days before late fees
	LateFeePercent        float64 `json:"late_fee_percent"`
	FreeTierProfitShare   float64 `json:"free_tier_profit_share"`   // Default 30%
	TraderTierProfitShare float64 `json:"trader_tier_profit_share"` // Default 20%
	ProTierProfitShare    float64 `json:"pro_tier_profit_share"`    // Default 12%
	WhaleTierProfitShare  float64 `json:"whale_tier_profit_share"`  // Default 5%
}

// RedisConfig holds Redis configuration for caching and rate limiting
type RedisConfig struct {
	Enabled  bool   `json:"enabled"`
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
	PoolSize int    `json:"pool_size"`
}

func Load() (*Config, error) {
	// First try to load base config from file
	cfg, err := loadFromFile("config.json")
	if err != nil {
		// If no config file, start with empty config
		cfg = &Config{}
	}

	// Apply environment variable overrides (these take precedence)
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config
// Note: BINANCE_API_KEY and BINANCE_SECRET_KEY are NOT read from environment.
// All API keys are per-user and stored in the database.
func applyEnvOverrides(cfg *Config) {
	// Binance config - only non-credential settings from environment
	cfg.BinanceConfig.BaseURL = getEnvOrDefault("BINANCE_BASE_URL", cfg.BinanceConfig.BaseURL)
	if cfg.BinanceConfig.BaseURL == "" {
		cfg.BinanceConfig.BaseURL = "https://api.binance.com"
	}
	cfg.BinanceConfig.TestNet = getEnvOrDefault("BINANCE_TESTNET", "false") == "true"
	cfg.BinanceConfig.MockMode = getEnvOrDefault("MOCK_MODE", "false") == "true"

	// Trading config
	cfg.TradingConfig.DryRun = getEnvOrDefault("TRADING_DRY_RUN", "false") == "true"

	// Scanner config
	cfg.ScannerConfig.Enabled = getEnvOrDefault("SCANNER_ENABLED", "true") == "true"

	// Notification config
	cfg.NotificationConfig.Enabled = getEnvOrDefault("NOTIFICATIONS_ENABLED", "false") == "true"
	cfg.NotificationConfig.Telegram.Enabled = getEnvOrDefault("TELEGRAM_ENABLED", "false") == "true"
	cfg.NotificationConfig.Telegram.BotToken = getEnvOrDefault("TELEGRAM_BOT_TOKEN", cfg.NotificationConfig.Telegram.BotToken)
	cfg.NotificationConfig.Telegram.ChatID = getEnvOrDefault("TELEGRAM_CHAT_ID", cfg.NotificationConfig.Telegram.ChatID)
	cfg.NotificationConfig.Discord.Enabled = getEnvOrDefault("DISCORD_ENABLED", "false") == "true"
	cfg.NotificationConfig.Discord.WebhookURL = getEnvOrDefault("DISCORD_WEBHOOK_URL", cfg.NotificationConfig.Discord.WebhookURL)

	// Logging config
	cfg.LoggingConfig.Level = getEnvOrDefault("LOG_LEVEL", "INFO")
	cfg.LoggingConfig.Output = getEnvOrDefault("LOG_OUTPUT", "stdout")
	cfg.LoggingConfig.JSONFormat = getEnvOrDefault("LOG_JSON", "true") == "true"
	cfg.LoggingConfig.IncludeFile = getEnvOrDefault("LOG_INCLUDE_FILE", "false") == "true"

	// AI config
	cfg.AIConfig.Enabled = getEnvOrDefault("AI_ENABLED", "true") == "true"
	cfg.AIConfig.LLMProvider = getEnvOrDefault("AI_LLM_PROVIDER", "claude")
	cfg.AIConfig.ClaudeAPIKey = getEnvOrDefault("AI_CLAUDE_API_KEY", cfg.AIConfig.ClaudeAPIKey)
	cfg.AIConfig.OpenAIAPIKey = getEnvOrDefault("AI_OPENAI_API_KEY", cfg.AIConfig.OpenAIAPIKey)
	cfg.AIConfig.DeepSeekAPIKey = getEnvOrDefault("AI_DEEPSEEK_API_KEY", cfg.AIConfig.DeepSeekAPIKey)
	cfg.AIConfig.LLMModel = getEnvOrDefault("AI_LLM_MODEL", "claude-3-haiku-20240307")
	cfg.AIConfig.MLEnabled = getEnvOrDefault("AI_ML_ENABLED", "true") == "true"
	cfg.AIConfig.SentimentEnabled = getEnvOrDefault("AI_SENTIMENT_ENABLED", "true") == "true"

	// Server config
	cfg.ServerConfig.Port = getEnvIntOrDefault("WEB_PORT", 8080)
	cfg.ServerConfig.Host = getEnvOrDefault("WEB_HOST", "0.0.0.0")
	cfg.ServerConfig.AllowedOrigins = getEnvOrDefault("SERVER_ALLOWED_ORIGINS", "*")
	cfg.ServerConfig.TLSEnabled = getEnvOrDefault("SERVER_TLS_ENABLED", "false") == "true"
	cfg.ServerConfig.TLSCertFile = getEnvOrDefault("SERVER_TLS_CERT", "")
	cfg.ServerConfig.TLSKeyFile = getEnvOrDefault("SERVER_TLS_KEY", "")
	cfg.ServerConfig.ReadTimeout = getEnvIntOrDefault("SERVER_READ_TIMEOUT", 30)
	cfg.ServerConfig.WriteTimeout = getEnvIntOrDefault("SERVER_WRITE_TIMEOUT", 30)
	cfg.ServerConfig.ShutdownTimeout = getEnvIntOrDefault("SERVER_SHUTDOWN_TIMEOUT", 10)

	// Auth config - ALWAYS apply from environment
	cfg.AuthConfig.Enabled = getEnvOrDefault("AUTH_ENABLED", "false") == "true"
	cfg.AuthConfig.JWTSecret = getEnvOrDefault("AUTH_JWT_SECRET", cfg.AuthConfig.JWTSecret)
	cfg.AuthConfig.AccessTokenDuration = getEnvDurationOrDefault("AUTH_ACCESS_TOKEN_DURATION", 15*time.Minute)
	cfg.AuthConfig.RefreshTokenDuration = getEnvDurationOrDefault("AUTH_REFRESH_TOKEN_DURATION", 7*24*time.Hour)
	cfg.AuthConfig.PasswordResetDuration = getEnvDurationOrDefault("AUTH_PASSWORD_RESET_DURATION", 1*time.Hour)
	cfg.AuthConfig.MinPasswordLength = getEnvIntOrDefault("AUTH_MIN_PASSWORD_LENGTH", 8)
	cfg.AuthConfig.RequireEmailVerification = getEnvOrDefault("AUTH_REQUIRE_EMAIL_VERIFICATION", "false") == "true"
	cfg.AuthConfig.MaxLoginAttempts = getEnvIntOrDefault("AUTH_MAX_LOGIN_ATTEMPTS", 5)
	cfg.AuthConfig.LockoutDuration = getEnvDurationOrDefault("AUTH_LOCKOUT_DURATION", 15*time.Minute)
	cfg.AuthConfig.SessionCleanupInterval = getEnvDurationOrDefault("AUTH_SESSION_CLEANUP_INTERVAL", 1*time.Hour)
	cfg.AuthConfig.AllowMultipleSessions = getEnvOrDefault("AUTH_ALLOW_MULTIPLE_SESSIONS", "true") == "true"
	cfg.AuthConfig.MaxSessionsPerUser = getEnvIntOrDefault("AUTH_MAX_SESSIONS_PER_USER", 10)

	// Vault config
	cfg.VaultConfig.Enabled = getEnvOrDefault("VAULT_ENABLED", "false") == "true"
	cfg.VaultConfig.Address = getEnvOrDefault("VAULT_ADDR", "http://localhost:8200")
	cfg.VaultConfig.Token = getEnvOrDefault("VAULT_TOKEN", cfg.VaultConfig.Token)
	cfg.VaultConfig.MountPath = getEnvOrDefault("VAULT_MOUNT_PATH", "secret")
	cfg.VaultConfig.SecretPath = getEnvOrDefault("VAULT_SECRET_PATH", "trading-bot/api-keys")
	cfg.VaultConfig.TLSEnabled = getEnvOrDefault("VAULT_TLS_ENABLED", "false") == "true"

	// Futures config
	cfg.FuturesConfig.Enabled = getEnvOrDefault("FUTURES_ENABLED", "true") == "true"
	cfg.FuturesConfig.TestNet = getEnvOrDefault("FUTURES_TESTNET", "false") == "true"
	cfg.FuturesConfig.DefaultLeverage = getEnvIntOrDefault("FUTURES_DEFAULT_LEVERAGE", 10)
	cfg.FuturesConfig.DefaultMarginType = getEnvOrDefault("FUTURES_DEFAULT_MARGIN_TYPE", "CROSSED")
	cfg.FuturesConfig.PositionMode = getEnvOrDefault("FUTURES_POSITION_MODE", "ONE_WAY")
	cfg.FuturesConfig.MaxLeverage = getEnvIntOrDefault("FUTURES_MAX_LEVERAGE", 125)

	// Futures autopilot config
	cfg.FuturesAutopilotConfig.Enabled = getEnvOrDefault("FUTURES_AUTOPILOT_ENABLED", "true") == "true"
	cfg.FuturesAutopilotConfig.RiskLevel = getEnvOrDefault("FUTURES_AUTOPILOT_RISK_LEVEL", "moderate")
	cfg.FuturesAutopilotConfig.MaxDailyTrades = getEnvIntOrDefault("FUTURES_AUTOPILOT_MAX_DAILY_TRADES", 10)
	cfg.FuturesAutopilotConfig.MaxDailyLoss = getEnvFloatOrDefault("FUTURES_AUTOPILOT_MAX_DAILY_LOSS", 3.0)
	cfg.FuturesAutopilotConfig.MaxPositionSize = getEnvFloatOrDefault("FUTURES_AUTOPILOT_MAX_POSITION_SIZE", 5.0)
	cfg.FuturesAutopilotConfig.MinConfidence = getEnvFloatOrDefault("FUTURES_AUTOPILOT_MIN_CONFIDENCE", 0.6)
	cfg.FuturesAutopilotConfig.DefaultLeverage = getEnvIntOrDefault("FUTURES_AUTOPILOT_DEFAULT_LEVERAGE", 5)
	cfg.FuturesAutopilotConfig.MaxLeverage = getEnvIntOrDefault("FUTURES_AUTOPILOT_MAX_LEVERAGE", 20)

	// Circuit breaker config
	cfg.CircuitBreakerConfig.Enabled = getEnvOrDefault("CIRCUIT_BREAKER_ENABLED", "true") == "true"
	cfg.CircuitBreakerConfig.MaxLossPerHour = getEnvFloatOrDefault("CIRCUIT_MAX_LOSS_PER_HOUR", 3.0)
	cfg.CircuitBreakerConfig.MaxConsecutiveLosses = getEnvIntOrDefault("CIRCUIT_MAX_CONSECUTIVE_LOSSES", 5)
	cfg.CircuitBreakerConfig.CooldownMinutes = getEnvIntOrDefault("CIRCUIT_COOLDOWN_MINUTES", 30)

	// Billing config
	cfg.BillingConfig.Enabled = getEnvOrDefault("BILLING_ENABLED", "false") == "true"
	cfg.BillingConfig.StripeSecretKey = getEnvOrDefault("STRIPE_SECRET_KEY", cfg.BillingConfig.StripeSecretKey)
	cfg.BillingConfig.StripePublishableKey = getEnvOrDefault("STRIPE_PUBLISHABLE_KEY", cfg.BillingConfig.StripePublishableKey)
	cfg.BillingConfig.StripeWebhookSecret = getEnvOrDefault("STRIPE_WEBHOOK_SECRET", cfg.BillingConfig.StripeWebhookSecret)

}


func loadFromFile(filename string) (*Config, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	return &config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// ToAuthConfig converts AuthConfig to the format expected by the auth package
func (c *AuthConfig) ToAuthConfig() AuthConfigExport {
	return AuthConfigExport{
		JWTSecret:                c.JWTSecret,
		AccessTokenDuration:      c.AccessTokenDuration,
		RefreshTokenDuration:     c.RefreshTokenDuration,
		PasswordResetDuration:    c.PasswordResetDuration,
		MinPasswordLength:        c.MinPasswordLength,
		RequireEmailVerification: c.RequireEmailVerification,
	}
}

// AuthConfigExport is the exported auth config format for the auth package
type AuthConfigExport struct {
	JWTSecret                string
	AccessTokenDuration      time.Duration
	RefreshTokenDuration     time.Duration
	PasswordResetDuration    time.Duration
	MinPasswordLength        int
	RequireEmailVerification bool
}

// GenerateSampleConfig creates a sample configuration file
func GenerateSampleConfig(filename string) error {
	config := Config{
		BinanceConfig: BinanceConfig{
			APIKey:    "your_api_key_here",
			SecretKey: "your_secret_key_here",
			BaseURL:   "https://api.binance.com",
			TestNet:   true,
		},
		ScreenerConfig: ScreenerConfig{
			Enabled:           true,
			Interval:          "15m",
			MinVolume:         100000,
			MinPriceChange:    2.0,
			ExcludeSymbols:    []string{"BUSDUSDT", "USDCUSDT"},
			QuoteCurrency:     "USDT",
			MaxSymbols:        50,
			ScreeningInterval: 60,
		},
		TradingConfig: TradingConfig{
			MaxOpenPositions: 5,
			MaxRiskPerTrade:  2.0,
			DryRun:           true,
		},
		ScannerConfig: ScannerConfig{
			Enabled:          true,
			ScanInterval:     30,
			MaxSymbols:       50,
			IncludeWatchlist: true,
			CacheTTL:         60,
			WorkerCount:      10,
		},
		NotificationConfig: NotificationConfig{
			Enabled: false,
			Telegram: TelegramConfig{
				Enabled:  false,
				BotToken: "",
				ChatID:   "",
			},
			Discord: DiscordConfig{
				Enabled:    false,
				WebhookURL: "",
			},
		},
		RiskConfig: RiskConfig{
			MaxRiskPerTrade:        2.0,
			MaxDailyDrawdown:       5.0,
			MaxOpenPositions:       5,
			PositionSizeMethod:     "percent",
			FixedPositionSize:      100.0,
			UseTrailingStop:        true,
			TrailingStopPercent:    1.0,
			TrailingStopActivation: 1.5,
		},
		LoggingConfig: LoggingConfig{
			Level:       "INFO",
			Output:      "stdout",
			JSONFormat:  true,
			IncludeFile: false,
		},
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}
