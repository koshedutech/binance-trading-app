package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	BinanceConfig      BinanceConfig      `json:"binance"`
	ScreenerConfig     ScreenerConfig     `json:"screener"`
	TradingConfig      TradingConfig      `json:"trading"`
	ScannerConfig      ScannerConfig      `json:"scanner"`
	NotificationConfig NotificationConfig `json:"notification"`
	RiskConfig         RiskConfig         `json:"risk"`
	LoggingConfig      LoggingConfig      `json:"logging"`
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
	MaxRiskPerTrade     float64 `json:"max_risk_per_trade"`      // Percentage of account to risk per trade
	MaxDailyDrawdown    float64 `json:"max_daily_drawdown"`      // Max daily loss percentage before stopping
	MaxOpenPositions    int     `json:"max_open_positions"`      // Maximum concurrent positions
	PositionSizeMethod  string  `json:"position_size_method"`    // "fixed", "percent", "kelly"
	FixedPositionSize   float64 `json:"fixed_position_size"`     // Fixed position size in quote currency
	UseTrailingStop     bool    `json:"use_trailing_stop"`       // Enable trailing stop loss
	TrailingStopPercent float64 `json:"trailing_stop_percent"`   // Trailing stop distance percentage
	TrailingStopActivation float64 `json:"trailing_stop_activation"` // Profit % to activate trailing stop
}

func Load() (*Config, error) {
	// Try to load from environment variables first
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		// Try to load from config file
		return loadFromFile("config.json")
	}

	// Create config from environment variables
	return &Config{
		BinanceConfig: BinanceConfig{
			APIKey:    apiKey,
			SecretKey: secretKey,
			BaseURL:   getEnvOrDefault("BINANCE_BASE_URL", "https://api.binance.com"),
			TestNet:   getEnvOrDefault("BINANCE_TESTNET", "false") == "true",
		},
		ScreenerConfig: ScreenerConfig{
			Enabled:           true,
			Interval:          "15m",
			MinVolume:         100000, // $100k
			MinPriceChange:    2.0,    // 2%
			QuoteCurrency:     "USDT",
			MaxSymbols:        50,
			ScreeningInterval: 60, // 1 minute
		},
		TradingConfig: TradingConfig{
			MaxOpenPositions: 5,
			MaxRiskPerTrade:  2.0, // 2%
			DryRun:           true,
		},
		ScannerConfig: ScannerConfig{
			Enabled:          true,
			ScanInterval:     30,   // 30 seconds
			MaxSymbols:       50,   // Top 50 results
			IncludeWatchlist: true,
			CacheTTL:         60,   // 60 seconds
			WorkerCount:      10,   // 10 concurrent workers
		},
		NotificationConfig: NotificationConfig{
			Enabled: getEnvOrDefault("NOTIFICATIONS_ENABLED", "false") == "true",
			Telegram: TelegramConfig{
				Enabled:  getEnvOrDefault("TELEGRAM_ENABLED", "false") == "true",
				BotToken: getEnvOrDefault("TELEGRAM_BOT_TOKEN", ""),
				ChatID:   getEnvOrDefault("TELEGRAM_CHAT_ID", ""),
			},
			Discord: DiscordConfig{
				Enabled:    getEnvOrDefault("DISCORD_ENABLED", "false") == "true",
				WebhookURL: getEnvOrDefault("DISCORD_WEBHOOK_URL", ""),
			},
		},
		RiskConfig: RiskConfig{
			MaxRiskPerTrade:        2.0,       // 2% per trade
			MaxDailyDrawdown:       5.0,       // 5% max daily drawdown
			MaxOpenPositions:       5,
			PositionSizeMethod:     "percent", // Use percentage-based sizing
			FixedPositionSize:      100.0,     // $100 if using fixed
			UseTrailingStop:        true,
			TrailingStopPercent:    1.0,       // 1% trailing distance
			TrailingStopActivation: 1.5,       // Activate after 1.5% profit
		},
		LoggingConfig: LoggingConfig{
			Level:       getEnvOrDefault("LOG_LEVEL", "INFO"),
			Output:      getEnvOrDefault("LOG_OUTPUT", "stdout"),
			JSONFormat:  getEnvOrDefault("LOG_JSON", "true") == "true",
			IncludeFile: getEnvOrDefault("LOG_INCLUDE_FILE", "false") == "true",
		},
	}, nil
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
