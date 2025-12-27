package binance

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// KlineTimeframe represents a supported kline interval
type KlineTimeframe string

const (
	Timeframe1m  KlineTimeframe = "1m"
	Timeframe5m  KlineTimeframe = "5m"
	Timeframe15m KlineTimeframe = "15m"
	Timeframe1h  KlineTimeframe = "1h"
	Timeframe4h  KlineTimeframe = "4h"
)

// AllTimeframes returns all supported timeframes for multi-timeframe analysis
var AllTimeframes = []KlineTimeframe{Timeframe1m, Timeframe5m, Timeframe15m, Timeframe1h, Timeframe4h}

// TradingModeTimeframes returns timeframes needed for each trading mode
var TradingModeTimeframes = map[string][]KlineTimeframe{
	"scalp":    {Timeframe1m, Timeframe5m, Timeframe15m, Timeframe1h},
	"swing":    {Timeframe1m, Timeframe15m, Timeframe1h},
	"position": {Timeframe1m, Timeframe15m, Timeframe1h, Timeframe4h},
}

// SubscriptionStats tracks subscription statistics
type SubscriptionStats struct {
	TotalSubscriptions   int            `json:"total_subscriptions"`
	ActiveSymbols        int            `json:"active_symbols"`
	TimeframeBreakdown   map[string]int `json:"timeframe_breakdown"`
	UpdatesReceived      int64          `json:"updates_received"`
	LastUpdateTime       time.Time      `json:"last_update_time"`
	SubscriptionFailures int64          `json:"subscription_failures"`
}

// KlineSubscriber interface for WebSocket clients that can subscribe to kline streams
type KlineSubscriber interface {
	SubscribeKline(symbol, interval string) error
	UnsubscribeKline(symbol, interval string) error
}

// KlineSubscriptionManager manages multi-timeframe kline subscriptions
// It tracks which symbols need which timeframes and handles subscription lifecycle
type KlineSubscriptionManager struct {
	mu sync.RWMutex

	// Subscriptions: symbol -> set of timeframes
	subscriptions map[string]map[KlineTimeframe]bool

	// Active symbols with their trading modes
	activeSymbols map[string]string // symbol -> mode (scalp/swing/position)

	// WebSocket subscriber (injected)
	subscriber KlineSubscriber

	// Statistics
	updatesReceived      int64
	lastUpdateTime       time.Time
	subscriptionFailures int64

	// Configuration
	maxSubscriptionsPerSymbol int
	enabledTimeframes         []KlineTimeframe
}

// NewKlineSubscriptionManager creates a new subscription manager
func NewKlineSubscriptionManager() *KlineSubscriptionManager {
	return &KlineSubscriptionManager{
		subscriptions:             make(map[string]map[KlineTimeframe]bool),
		activeSymbols:             make(map[string]string),
		maxSubscriptionsPerSymbol: 5, // Limit timeframes per symbol
		enabledTimeframes:         AllTimeframes,
	}
}

// SetSubscriber sets the WebSocket subscriber for managing subscriptions
func (m *KlineSubscriptionManager) SetSubscriber(subscriber KlineSubscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriber = subscriber
}

// SetEnabledTimeframes configures which timeframes to subscribe to
func (m *KlineSubscriptionManager) SetEnabledTimeframes(timeframes []KlineTimeframe) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabledTimeframes = timeframes
}

// SubscribeSymbol subscribes to all enabled timeframes for a symbol
func (m *KlineSubscriptionManager) SubscribeSymbol(symbol string) error {
	return m.SubscribeSymbolWithMode(symbol, "")
}

// SubscribeSymbolWithMode subscribes to timeframes based on trading mode
func (m *KlineSubscriptionManager) SubscribeSymbolWithMode(symbol, mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	symbol = strings.ToUpper(symbol)

	// Determine which timeframes to subscribe based on mode
	var timeframes []KlineTimeframe
	if mode != "" {
		if modeTimeframes, ok := TradingModeTimeframes[mode]; ok {
			timeframes = modeTimeframes
		} else {
			timeframes = m.enabledTimeframes
		}
	} else {
		timeframes = m.enabledTimeframes
	}

	// Initialize subscription map for symbol if needed
	if m.subscriptions[symbol] == nil {
		m.subscriptions[symbol] = make(map[KlineTimeframe]bool)
	}

	// Track the mode
	if mode != "" {
		m.activeSymbols[symbol] = mode
	}

	// Subscribe to each timeframe
	var lastErr error
	for _, tf := range timeframes {
		if m.subscriptions[symbol][tf] {
			continue // Already subscribed
		}

		if m.subscriber != nil {
			if err := m.subscriber.SubscribeKline(symbol, string(tf)); err != nil {
				log.Printf("Failed to subscribe %s@kline_%s: %v", symbol, tf, err)
				m.subscriptionFailures++
				lastErr = err
				continue
			}
		}

		m.subscriptions[symbol][tf] = true
		log.Printf("Subscribed to kline stream: %s@%s", symbol, tf)
	}

	return lastErr
}

// UnsubscribeSymbol unsubscribes from all timeframes for a symbol
func (m *KlineSubscriptionManager) UnsubscribeSymbol(symbol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	symbol = strings.ToUpper(symbol)

	timeframes, exists := m.subscriptions[symbol]
	if !exists {
		return nil
	}

	var lastErr error
	for tf := range timeframes {
		if m.subscriber != nil {
			if err := m.subscriber.UnsubscribeKline(symbol, string(tf)); err != nil {
				log.Printf("Failed to unsubscribe %s@kline_%s: %v", symbol, tf, err)
				lastErr = err
				continue
			}
		}
		log.Printf("Unsubscribed from kline stream: %s@%s", symbol, tf)
	}

	delete(m.subscriptions, symbol)
	delete(m.activeSymbols, symbol)

	return lastErr
}

// SubscribeTimeframe subscribes to a specific timeframe for a symbol
func (m *KlineSubscriptionManager) SubscribeTimeframe(symbol string, timeframe KlineTimeframe) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	symbol = strings.ToUpper(symbol)

	if m.subscriptions[symbol] == nil {
		m.subscriptions[symbol] = make(map[KlineTimeframe]bool)
	}

	if m.subscriptions[symbol][timeframe] {
		return nil // Already subscribed
	}

	if m.subscriber != nil {
		if err := m.subscriber.SubscribeKline(symbol, string(timeframe)); err != nil {
			m.subscriptionFailures++
			return fmt.Errorf("failed to subscribe %s@kline_%s: %w", symbol, timeframe, err)
		}
	}

	m.subscriptions[symbol][timeframe] = true
	return nil
}

// UnsubscribeTimeframe unsubscribes from a specific timeframe for a symbol
func (m *KlineSubscriptionManager) UnsubscribeTimeframe(symbol string, timeframe KlineTimeframe) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	symbol = strings.ToUpper(symbol)

	if m.subscriptions[symbol] == nil || !m.subscriptions[symbol][timeframe] {
		return nil // Not subscribed
	}

	if m.subscriber != nil {
		if err := m.subscriber.UnsubscribeKline(symbol, string(timeframe)); err != nil {
			return fmt.Errorf("failed to unsubscribe %s@kline_%s: %w", symbol, timeframe, err)
		}
	}

	delete(m.subscriptions[symbol], timeframe)
	if len(m.subscriptions[symbol]) == 0 {
		delete(m.subscriptions, symbol)
		delete(m.activeSymbols, symbol)
	}

	return nil
}

// GetSubscribedSymbols returns all symbols with active subscriptions
func (m *KlineSubscriptionManager) GetSubscribedSymbols() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	symbols := make([]string, 0, len(m.subscriptions))
	for symbol := range m.subscriptions {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// GetSymbolTimeframes returns active timeframes for a symbol
func (m *KlineSubscriptionManager) GetSymbolTimeframes(symbol string) []KlineTimeframe {
	m.mu.RLock()
	defer m.mu.RUnlock()

	symbol = strings.ToUpper(symbol)
	timeframes := make([]KlineTimeframe, 0)

	if subs, exists := m.subscriptions[symbol]; exists {
		for tf := range subs {
			timeframes = append(timeframes, tf)
		}
	}
	return timeframes
}

// IsSubscribed checks if a symbol:timeframe is subscribed
func (m *KlineSubscriptionManager) IsSubscribed(symbol string, timeframe KlineTimeframe) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	symbol = strings.ToUpper(symbol)
	if subs, exists := m.subscriptions[symbol]; exists {
		return subs[timeframe]
	}
	return false
}

// RecordUpdate records that a kline update was received (for stats)
func (m *KlineSubscriptionManager) RecordUpdate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updatesReceived++
	m.lastUpdateTime = time.Now()
}

// GetStats returns subscription statistics
func (m *KlineSubscriptionManager) GetStats() SubscriptionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSubs := 0
	tfBreakdown := make(map[string]int)

	for _, timeframes := range m.subscriptions {
		for tf := range timeframes {
			totalSubs++
			tfBreakdown[string(tf)]++
		}
	}

	return SubscriptionStats{
		TotalSubscriptions:   totalSubs,
		ActiveSymbols:        len(m.subscriptions),
		TimeframeBreakdown:   tfBreakdown,
		UpdatesReceived:      m.updatesReceived,
		LastUpdateTime:       m.lastUpdateTime,
		SubscriptionFailures: m.subscriptionFailures,
	}
}

// BuildStreamList builds a list of stream names for initial connection
// Format: "btcusdt@kline_1m", "btcusdt@kline_5m", etc.
func (m *KlineSubscriptionManager) BuildStreamList(symbols []string) []string {
	m.mu.RLock()
	timeframes := m.enabledTimeframes
	m.mu.RUnlock()

	streams := make([]string, 0, len(symbols)*len(timeframes))

	for _, symbol := range symbols {
		lowerSymbol := strings.ToLower(symbol)
		for _, tf := range timeframes {
			streams = append(streams, fmt.Sprintf("%s@kline_%s", lowerSymbol, tf))
		}
	}

	return streams
}

// BuildStreamListForMode builds streams for symbols with a specific trading mode
func (m *KlineSubscriptionManager) BuildStreamListForMode(symbols []string, mode string) []string {
	var timeframes []KlineTimeframe
	if modeTimeframes, ok := TradingModeTimeframes[mode]; ok {
		timeframes = modeTimeframes
	} else {
		timeframes = AllTimeframes
	}

	streams := make([]string, 0, len(symbols)*len(timeframes))

	for _, symbol := range symbols {
		lowerSymbol := strings.ToLower(symbol)
		for _, tf := range timeframes {
			streams = append(streams, fmt.Sprintf("%s@kline_%s", lowerSymbol, tf))
		}
	}

	return streams
}

// SyncSubscriptions ensures subscriptions match the desired state
// This is useful after reconnection to re-subscribe to all streams
func (m *KlineSubscriptionManager) SyncSubscriptions() error {
	m.mu.RLock()
	toSubscribe := make(map[string][]KlineTimeframe)
	for symbol, timeframes := range m.subscriptions {
		for tf := range timeframes {
			toSubscribe[symbol] = append(toSubscribe[symbol], tf)
		}
	}
	subscriber := m.subscriber
	m.mu.RUnlock()

	if subscriber == nil {
		return fmt.Errorf("no subscriber set")
	}

	var lastErr error
	for symbol, timeframes := range toSubscribe {
		for _, tf := range timeframes {
			if err := subscriber.SubscribeKline(symbol, string(tf)); err != nil {
				log.Printf("Failed to re-subscribe %s@kline_%s: %v", symbol, tf, err)
				lastErr = err
			}
		}
	}

	return lastErr
}

// Clear removes all subscriptions
func (m *KlineSubscriptionManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscriptions = make(map[string]map[KlineTimeframe]bool)
	m.activeSymbols = make(map[string]string)
}
