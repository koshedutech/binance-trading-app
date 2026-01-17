package events

import (
	"sync"
	"time"
)

// EventType represents different types of events in the system
type EventType string

const (
	EventTradeOpened       EventType = "TRADE_OPENED"
	EventTradeClosed       EventType = "TRADE_CLOSED"
	EventTradeUpdate       EventType = "TRADE_UPDATE"
	EventOrderPlaced       EventType = "ORDER_PLACED"
	EventOrderFilled       EventType = "ORDER_FILLED"
	EventOrderCancelled    EventType = "ORDER_CANCELLED"
	EventOrderUpdate       EventType = "ORDER_UPDATE"
	EventSignalGenerated   EventType = "SIGNAL_GENERATED"
	EventStrategyToggled   EventType = "STRATEGY_TOGGLED"
	EventScreenerUpdate    EventType = "SCREENER_UPDATE"
	EventPriceUpdate       EventType = "PRICE_UPDATE"
	EventPositionUpdate    EventType = "POSITION_UPDATE"
	EventBotStarted        EventType = "BOT_STARTED"
	EventBotStopped        EventType = "BOT_STOPPED"
	EventError             EventType = "ERROR"
	EventTradingModeChanged EventType = "TRADING_MODE_CHANGED"
	EventAutopilotToggled EventType = "AUTOPILOT_TOGGLED"
	EventUserLogout       EventType = "USER_LOGOUT"
	EventBalanceUpdate    EventType = "BALANCE_UPDATE"

	// Epic 12: WebSocket Real-Time Data Migration - New Event Types
	EventChainUpdate         EventType = "CHAIN_UPDATE"
	EventLifecycleEvent      EventType = "LIFECYCLE_EVENT"
	EventGinieStatusUpdate   EventType = "GINIE_STATUS_UPDATE"
	EventCircuitBreakerUpdate EventType = "CIRCUIT_BREAKER_UPDATE"
	EventPnLUpdate           EventType = "PNL_UPDATE"
	EventModeStatusUpdate    EventType = "MODE_STATUS_UPDATE"
	EventSystemStatusUpdate  EventType = "SYSTEM_STATUS_UPDATE"
	EventSignalUpdate        EventType = "SIGNAL_UPDATE"
)

// Event represents a system event
type Event struct {
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Subscriber is a function that handles events
type Subscriber func(Event)

// EventBus manages event publishing and subscriptions
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]Subscriber
	allSubs     []Subscriber // Subscribers to all events
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]Subscriber),
		allSubs:     make([]Subscriber, 0),
	}
}

// Subscribe registers a subscriber for a specific event type
func (eb *EventBus) Subscribe(eventType EventType, subscriber Subscriber) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], subscriber)
}

// SubscribeAll registers a subscriber for all events
func (eb *EventBus) SubscribeAll(subscriber Subscriber) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.allSubs = append(eb.allSubs, subscriber)
}

// Publish sends an event to all subscribers
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Notify specific subscribers
	if subs, ok := eb.subscribers[event.Type]; ok {
		for _, sub := range subs {
			go sub(event) // Run in goroutine to avoid blocking
		}
	}

	// Notify all-event subscribers
	for _, sub := range eb.allSubs {
		go sub(event)
	}
}

// PublishTradeOpened publishes a trade opened event
func (eb *EventBus) PublishTradeOpened(symbol string, side string, entryPrice, quantity float64) {
	eb.Publish(Event{
		Type: EventTradeOpened,
		Data: map[string]interface{}{
			"symbol":      symbol,
			"side":        side,
			"entry_price": entryPrice,
			"quantity":    quantity,
		},
	})
}

// PublishTradeClosed publishes a trade closed event
func (eb *EventBus) PublishTradeClosed(symbol string, entryPrice, exitPrice, quantity, pnl, pnlPercent float64) {
	eb.Publish(Event{
		Type: EventTradeClosed,
		Data: map[string]interface{}{
			"symbol":      symbol,
			"entry_price": entryPrice,
			"exit_price":  exitPrice,
			"quantity":    quantity,
			"pnl":         pnl,
			"pnl_percent": pnlPercent,
		},
	})
}

// PublishSignal publishes a signal generated event
func (eb *EventBus) PublishSignal(strategyName, symbol, signalType, reason string, price float64) {
	eb.Publish(Event{
		Type: EventSignalGenerated,
		Data: map[string]interface{}{
			"strategy":    strategyName,
			"symbol":      symbol,
			"signal_type": signalType,
			"reason":      reason,
			"price":       price,
		},
	})
}

// PublishOrderPlaced publishes an order placed event
func (eb *EventBus) PublishOrderPlaced(orderID int64, symbol, orderType, side string, price, quantity float64) {
	eb.Publish(Event{
		Type: EventOrderPlaced,
		Data: map[string]interface{}{
			"order_id":   orderID,
			"symbol":     symbol,
			"order_type": orderType,
			"side":       side,
			"price":      price,
			"quantity":   quantity,
		},
	})
}

// PublishPriceUpdate publishes a price update event
func (eb *EventBus) PublishPriceUpdate(symbol string, price float64) {
	eb.Publish(Event{
		Type: EventPriceUpdate,
		Data: map[string]interface{}{
			"symbol": symbol,
			"price":  price,
		},
	})
}

// PublishPositionUpdate publishes a position update event
func (eb *EventBus) PublishPositionUpdate(symbol string, entryPrice, currentPrice, quantity, pnl, pnlPercent float64) {
	eb.Publish(Event{
		Type: EventPositionUpdate,
		Data: map[string]interface{}{
			"symbol":        symbol,
			"entry_price":   entryPrice,
			"current_price": currentPrice,
			"quantity":      quantity,
			"pnl":           pnl,
			"pnl_percent":   pnlPercent,
		},
	})
}

// PublishError publishes an error event
func (eb *EventBus) PublishError(source, message string, err error) {
	data := map[string]interface{}{
		"source":  source,
		"message": message,
	}
	if err != nil {
		data["error"] = err.Error()
	}
	eb.Publish(Event{
		Type: EventError,
		Data: data,
	})
}

// PublishUserLogout publishes a user logout event
func (eb *EventBus) PublishUserLogout(userID string) {
	eb.Publish(Event{
		Type: EventUserLogout,
		Data: map[string]interface{}{
			"user_id": userID,
		},
	})
}

// ============================================================================
// Epic 12: WebSocket Broadcast Callbacks
// These allow packages like database and orders to broadcast events without
// directly importing the api package, avoiding import cycles.
// ============================================================================

// BroadcastFunc is a callback function for broadcasting events to specific users
type BroadcastFunc func(userID string, data interface{})

// Global broadcast callbacks - wired up by api package at startup
var (
	broadcastLifecycleEvent  BroadcastFunc
	broadcastChainUpdate     BroadcastFunc
	broadcastCircuitBreaker  BroadcastFunc
	broadcastPnL             BroadcastFunc
	broadcastGinieStatus     BroadcastFunc
	broadcastModeStatus      BroadcastFunc
	broadcastSystemStatus    BroadcastFunc
	broadcastSignalUpdate    BroadcastFunc
	broadcastPositionUpdate  BroadcastFunc
)

// SetBroadcastLifecycleEvent sets the callback for lifecycle event broadcasts
func SetBroadcastLifecycleEvent(fn BroadcastFunc) {
	broadcastLifecycleEvent = fn
}

// SetBroadcastChainUpdate sets the callback for chain update broadcasts
func SetBroadcastChainUpdate(fn BroadcastFunc) {
	broadcastChainUpdate = fn
}

// SetBroadcastCircuitBreaker sets the callback for circuit breaker broadcasts
func SetBroadcastCircuitBreaker(fn BroadcastFunc) {
	broadcastCircuitBreaker = fn
}

// SetBroadcastPnL sets the callback for P&L broadcasts
func SetBroadcastPnL(fn BroadcastFunc) {
	broadcastPnL = fn
}

// SetBroadcastGinieStatus sets the callback for Ginie status broadcasts
func SetBroadcastGinieStatus(fn BroadcastFunc) {
	broadcastGinieStatus = fn
}

// SetBroadcastModeStatus sets the callback for mode status broadcasts
func SetBroadcastModeStatus(fn BroadcastFunc) {
	broadcastModeStatus = fn
}

// SetBroadcastSystemStatus sets the callback for system status broadcasts
func SetBroadcastSystemStatus(fn BroadcastFunc) {
	broadcastSystemStatus = fn
}

// SetBroadcastSignalUpdate sets the callback for signal update broadcasts
func SetBroadcastSignalUpdate(fn BroadcastFunc) {
	broadcastSignalUpdate = fn
}

// SetBroadcastPositionUpdate sets the callback for position update broadcasts
func SetBroadcastPositionUpdate(fn BroadcastFunc) {
	broadcastPositionUpdate = fn
}

// BroadcastLifecycleEvent broadcasts a lifecycle event to a user
func BroadcastLifecycleEvent(userID string, data interface{}) {
	if broadcastLifecycleEvent != nil && userID != "" {
		go broadcastLifecycleEvent(userID, data)
	}
}

// BroadcastChainUpdate broadcasts a chain update to a user
func BroadcastChainUpdate(userID string, data interface{}) {
	if broadcastChainUpdate != nil && userID != "" {
		go broadcastChainUpdate(userID, data)
	}
}

// BroadcastCircuitBreaker broadcasts circuit breaker state to a user
func BroadcastCircuitBreaker(userID string, data interface{}) {
	if broadcastCircuitBreaker != nil && userID != "" {
		go broadcastCircuitBreaker(userID, data)
	}
}

// BroadcastPnL broadcasts P&L updates to a user
func BroadcastPnL(userID string, data interface{}) {
	if broadcastPnL != nil && userID != "" {
		go broadcastPnL(userID, data)
	}
}

// BroadcastGinieStatus broadcasts Ginie status to a user
func BroadcastGinieStatus(userID string, data interface{}) {
	if broadcastGinieStatus != nil && userID != "" {
		go broadcastGinieStatus(userID, data)
	}
}

// BroadcastModeStatus broadcasts mode status to a user
func BroadcastModeStatus(userID string, data interface{}) {
	if broadcastModeStatus != nil && userID != "" {
		go broadcastModeStatus(userID, data)
	}
}

// BroadcastSystemStatus broadcasts system status to a user
func BroadcastSystemStatus(userID string, data interface{}) {
	if broadcastSystemStatus != nil && userID != "" {
		go broadcastSystemStatus(userID, data)
	}
}

// BroadcastSignalUpdate broadcasts signal update to a user
func BroadcastSignalUpdate(userID string, data interface{}) {
	if broadcastSignalUpdate != nil && userID != "" {
		go broadcastSignalUpdate(userID, data)
	}
}

// BroadcastPositionUpdate broadcasts position update to a user (used when positions close)
func BroadcastPositionUpdate(userID string, data interface{}) {
	if broadcastPositionUpdate != nil && userID != "" {
		go broadcastPositionUpdate(userID, data)
	}
}
