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
