package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotifySignal      NotificationType = "signal"
	NotifyTradeOpen   NotificationType = "trade_open"
	NotifyTradeClose  NotificationType = "trade_close"
	NotifyError       NotificationType = "error"
	NotifyInfo        NotificationType = "info"
)

// Notification represents a notification message
type Notification struct {
	Type      NotificationType
	Title     string
	Message   string
	Symbol    string
	Price     float64
	PnL       float64
	PnLPercent float64
	Timestamp time.Time
	Extra     map[string]interface{}
}

// Notifier interface for different notification providers
type Notifier interface {
	Send(notification *Notification) error
	Name() string
	IsEnabled() bool
}

// Manager manages multiple notification providers
type Manager struct {
	notifiers []Notifier
	enabled   bool
}

// NewManager creates a new notification manager
func NewManager() *Manager {
	return &Manager{
		notifiers: make([]Notifier, 0),
		enabled:   true,
	}
}

// AddNotifier adds a notification provider
func (m *Manager) AddNotifier(n Notifier) {
	m.notifiers = append(m.notifiers, n)
}

// Send sends a notification to all enabled providers
func (m *Manager) Send(notification *Notification) error {
	if !m.enabled {
		return nil
	}

	var lastErr error
	for _, n := range m.notifiers {
		if n.IsEnabled() {
			if err := n.Send(notification); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// SendSignal sends a trading signal notification
func (m *Manager) SendSignal(symbol, side, reason string, price, stopLoss, takeProfit float64) error {
	emoji := "🟢"
	if side == "SELL" {
		emoji = "🔴"
	}

	return m.Send(&Notification{
		Type:      NotifySignal,
		Title:     fmt.Sprintf("%s Signal: %s", emoji, symbol),
		Message:   fmt.Sprintf("%s %s @ %.4f\nSL: %.4f | TP: %.4f\nReason: %s", side, symbol, price, stopLoss, takeProfit, reason),
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now(),
		Extra: map[string]interface{}{
			"side":        side,
			"stop_loss":   stopLoss,
			"take_profit": takeProfit,
			"reason":      reason,
		},
	})
}

// SendTradeOpen sends a trade opened notification
func (m *Manager) SendTradeOpen(symbol, side string, price, quantity float64) error {
	return m.Send(&Notification{
		Type:      NotifyTradeOpen,
		Title:     fmt.Sprintf("📈 Trade Opened: %s", symbol),
		Message:   fmt.Sprintf("%s %s\nPrice: %.4f\nQuantity: %.8f", side, symbol, price, quantity),
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now(),
	})
}

// SendTradeClose sends a trade closed notification
func (m *Manager) SendTradeClose(symbol string, entryPrice, exitPrice, pnl, pnlPercent float64, reason string) error {
	emoji := "✅"
	if pnl < 0 {
		emoji = "❌"
	}

	return m.Send(&Notification{
		Type:       NotifyTradeClose,
		Title:      fmt.Sprintf("%s Trade Closed: %s", emoji, symbol),
		Message:    fmt.Sprintf("Entry: %.4f → Exit: %.4f\nP&L: %.4f (%.2f%%)\nReason: %s", entryPrice, exitPrice, pnl, pnlPercent, reason),
		Symbol:     symbol,
		Price:      exitPrice,
		PnL:        pnl,
		PnLPercent: pnlPercent,
		Timestamp:  time.Now(),
	})
}

// SendError sends an error notification
func (m *Manager) SendError(title, message string) error {
	return m.Send(&Notification{
		Type:      NotifyError,
		Title:     fmt.Sprintf("⚠️ %s", title),
		Message:   message,
		Timestamp: time.Now(),
	})
}

// =============================================================================
// TELEGRAM NOTIFIER
// =============================================================================

// TelegramNotifier sends notifications via Telegram
type TelegramNotifier struct {
	botToken string
	chatID   string
	enabled  bool
	client   *http.Client
}

// TelegramConfig holds Telegram configuration
type TelegramConfig struct {
	BotToken string
	ChatID   string
	Enabled  bool
}

// NewTelegramNotifier creates a new Telegram notifier
func NewTelegramNotifier(config TelegramConfig) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: config.BotToken,
		chatID:   config.ChatID,
		enabled:  config.Enabled && config.BotToken != "" && config.ChatID != "",
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TelegramNotifier) Name() string {
	return "telegram"
}

func (t *TelegramNotifier) IsEnabled() bool {
	return t.enabled
}

func (t *TelegramNotifier) Send(notification *Notification) error {
	if !t.enabled {
		return nil
	}

	message := fmt.Sprintf("*%s*\n\n%s", notification.Title, notification.Message)

	payload := map[string]interface{}{
		"chat_id":    t.chatID,
		"text":       message,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)
	resp, err := t.client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

// =============================================================================
// DISCORD NOTIFIER
// =============================================================================

// DiscordNotifier sends notifications via Discord webhook
type DiscordNotifier struct {
	webhookURL string
	enabled    bool
	client     *http.Client
}

// DiscordConfig holds Discord configuration
type DiscordConfig struct {
	WebhookURL string
	Enabled    bool
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier(config DiscordConfig) *DiscordNotifier {
	return &DiscordNotifier{
		webhookURL: config.WebhookURL,
		enabled:    config.Enabled && config.WebhookURL != "",
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *DiscordNotifier) Name() string {
	return "discord"
}

func (d *DiscordNotifier) IsEnabled() bool {
	return d.enabled
}

func (d *DiscordNotifier) Send(notification *Notification) error {
	if !d.enabled {
		return nil
	}

	// Create Discord embed
	color := 0x00FF00 // Green
	if notification.Type == NotifyError {
		color = 0xFF0000 // Red
	} else if notification.Type == NotifyTradeClose && notification.PnL < 0 {
		color = 0xFF0000 // Red
	}

	embed := map[string]interface{}{
		"title":       notification.Title,
		"description": notification.Message,
		"color":       color,
		"timestamp":   notification.Timestamp.Format(time.RFC3339),
	}

	// Add fields if available
	if notification.Symbol != "" {
		fields := []map[string]interface{}{
			{"name": "Symbol", "value": notification.Symbol, "inline": true},
		}
		if notification.Price > 0 {
			fields = append(fields, map[string]interface{}{
				"name": "Price", "value": fmt.Sprintf("%.4f", notification.Price), "inline": true,
			})
		}
		if notification.PnL != 0 {
			fields = append(fields, map[string]interface{}{
				"name": "P&L", "value": fmt.Sprintf("%.4f (%.2f%%)", notification.PnL, notification.PnLPercent), "inline": true,
			})
		}
		embed["fields"] = fields
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{embed},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discord payload: %w", err)
	}

	resp, err := d.client.Post(d.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send discord message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	return nil
}
