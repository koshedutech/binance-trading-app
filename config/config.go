package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	BinanceConfig  BinanceConfig  `json:"binance"`
	ScreenerConfig ScreenerConfig `json:"screener"`
	TradingConfig  TradingConfig  `json:"trading"`
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
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}
