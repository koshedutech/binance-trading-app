package order

import (
	"binance-trading-bot/internal/binance"
	"fmt"
	"log"
	"time"
)

// OrderManager handles order lifecycle management
type OrderManager struct {
	client        *binance.Client
	activeOrders  map[int64]*ManagedOrder
	orderHistory  []*ManagedOrder
}

// ManagedOrder represents an order with additional management data
type ManagedOrder struct {
	OrderID       int64
	Symbol        string
	Side          string
	Type          string
	Price         float64
	Quantity      float64
	Status        string
	CreatedAt     time.Time
	LastModified  time.Time
	ModifyCount   int
	
	// Conditions for modification
	TrailingStopActive bool
	TrailingStopPercent float64
	HighestPrice       float64
	LowestPrice        float64
	
	// Advanced conditions
	TimeBasedRules     []TimeBasedRule
	PriceActionRules   []PriceActionRule
}

// TimeBasedRule defines time-based order modifications
type TimeBasedRule struct {
	Name          string
	TriggerTime   time.Time
	Action        string // "MODIFY_PRICE", "CANCEL", "CONVERT_TO_MARKET"
	Parameters    map[string]interface{}
}

// PriceActionRule defines price-action based modifications
type PriceActionRule struct {
	Name          string
	Condition     string  // "PRICE_ABOVE", "PRICE_BELOW", "VOLUME_SPIKE", "MOMENTUM_CHANGE"
	Threshold     float64
	Action        string
	Parameters    map[string]interface{}
}

func NewOrderManager(client *binance.Client) *OrderManager {
	return &OrderManager{
		client:       client,
		activeOrders: make(map[int64]*ManagedOrder),
		orderHistory: make([]*ManagedOrder, 0),
	}
}

// AddOrder adds a new order to management
func (om *OrderManager) AddOrder(order *ManagedOrder) {
	om.activeOrders[order.OrderID] = order
	log.Printf("Order %d added to management for %s", order.OrderID, order.Symbol)
}

// EnableTrailingStop enables trailing stop for an order
func (om *OrderManager) EnableTrailingStop(orderID int64, trailingPercent float64) error {
	order, exists := om.activeOrders[orderID]
	if !exists {
		return fmt.Errorf("order %d not found", orderID)
	}

	order.TrailingStopActive = true
	order.TrailingStopPercent = trailingPercent
	
	// Initialize highest/lowest price
	currentPrice, err := om.client.GetCurrentPrice(order.Symbol)
	if err != nil {
		return err
	}
	
	if order.Side == "BUY" {
		order.HighestPrice = currentPrice
	} else {
		order.LowestPrice = currentPrice
	}
	
	log.Printf("Trailing stop enabled for order %d at %.2f%%", orderID, trailingPercent*100)
	return nil
}

// AddTimeBasedRule adds a time-based modification rule
func (om *OrderManager) AddTimeBasedRule(orderID int64, rule TimeBasedRule) error {
	order, exists := om.activeOrders[orderID]
	if !exists {
		return fmt.Errorf("order %d not found", orderID)
	}
	
	order.TimeBasedRules = append(order.TimeBasedRules, rule)
	log.Printf("Time-based rule '%s' added for order %d", rule.Name, orderID)
	return nil
}

// AddPriceActionRule adds a price-action based modification rule
func (om *OrderManager) AddPriceActionRule(orderID int64, rule PriceActionRule) error {
	order, exists := om.activeOrders[orderID]
	if !exists {
		return fmt.Errorf("order %d not found", orderID)
	}
	
	order.PriceActionRules = append(order.PriceActionRules, rule)
	log.Printf("Price action rule '%s' added for order %d", rule.Name, orderID)
	return nil
}

// ProcessOrders checks and processes all active orders
func (om *OrderManager) ProcessOrders() {
	for orderID, order := range om.activeOrders {
		if err := om.processOrder(order); err != nil {
			log.Printf("Error processing order %d: %v", orderID, err)
		}
	}
}

// processOrder processes a single order
func (om *OrderManager) processOrder(order *ManagedOrder) error {
	currentPrice, err := om.client.GetCurrentPrice(order.Symbol)
	if err != nil {
		return err
	}

	// Check trailing stop
	if order.TrailingStopActive {
		if err := om.checkTrailingStop(order, currentPrice); err != nil {
			return err
		}
	}

	// Check time-based rules
	for _, rule := range order.TimeBasedRules {
		if time.Now().After(rule.TriggerTime) {
			if err := om.executeTimeBasedRule(order, rule); err != nil {
				log.Printf("Error executing time-based rule '%s': %v", rule.Name, err)
			}
		}
	}

	// Check price action rules
	for _, rule := range order.PriceActionRules {
		if om.checkPriceActionCondition(order, rule, currentPrice) {
			if err := om.executePriceActionRule(order, rule, currentPrice); err != nil {
				log.Printf("Error executing price action rule '%s': %v", rule.Name, err)
			}
		}
	}

	return nil
}

// checkTrailingStop checks and updates trailing stop
func (om *OrderManager) checkTrailingStop(order *ManagedOrder, currentPrice float64) error {
	if order.Side == "BUY" {
		// For buy orders, trail upwards
		if currentPrice > order.HighestPrice {
			order.HighestPrice = currentPrice
			newStopPrice := currentPrice * (1 - order.TrailingStopPercent)
			
			// Modify stop loss order
			log.Printf("Trailing stop updated for order %d: new stop at %.2f (price: %.2f)", 
				order.OrderID, newStopPrice, currentPrice)
			
			// Here you would modify the actual stop loss order
			// This requires keeping track of the stop loss order ID
		}
	} else {
		// For sell orders, trail downwards
		if currentPrice < order.LowestPrice {
			order.LowestPrice = currentPrice
			newStopPrice := currentPrice * (1 + order.TrailingStopPercent)
			
			log.Printf("Trailing stop updated for order %d: new stop at %.2f (price: %.2f)", 
				order.OrderID, newStopPrice, currentPrice)
		}
	}
	
	return nil
}

// executeTimeBasedRule executes a time-based rule
func (om *OrderManager) executeTimeBasedRule(order *ManagedOrder, rule TimeBasedRule) error {
	switch rule.Action {
	case "CANCEL":
		log.Printf("Canceling order %d due to time-based rule '%s'", order.OrderID, rule.Name)
		return om.client.CancelOrder(order.Symbol, order.OrderID)
		
	case "MODIFY_PRICE":
		priceAdjustment, ok := rule.Parameters["price_adjustment"].(float64)
		if !ok {
			return fmt.Errorf("invalid price_adjustment parameter")
		}
		newPrice := order.Price * (1 + priceAdjustment)
		log.Printf("Modifying order %d price from %.2f to %.2f", order.OrderID, order.Price, newPrice)
		
		// Cancel and replace with new price
		if err := om.client.CancelOrder(order.Symbol, order.OrderID); err != nil {
			return err
		}
		
		// Place new order at modified price
		params := map[string]string{
			"symbol":      order.Symbol,
			"side":        order.Side,
			"type":        order.Type,
			"quantity":    fmt.Sprintf("%.8f", order.Quantity),
			"price":       fmt.Sprintf("%.2f", newPrice),
			"timeInForce": "GTC",
		}
		
		newOrder, err := om.client.PlaceOrder(params)
		if err != nil {
			return err
		}
		
		order.OrderID = newOrder.OrderId
		order.Price = newPrice
		order.ModifyCount++
		order.LastModified = time.Now()
		
	case "CONVERT_TO_MARKET":
		log.Printf("Converting order %d to market order", order.OrderID)
		
		// Cancel limit order
		if err := om.client.CancelOrder(order.Symbol, order.OrderID); err != nil {
			return err
		}
		
		// Place market order
		params := map[string]string{
			"symbol":   order.Symbol,
			"side":     order.Side,
			"type":     "MARKET",
			"quantity": fmt.Sprintf("%.8f", order.Quantity),
		}
		
		newOrder, err := om.client.PlaceOrder(params)
		if err != nil {
			return err
		}
		
		order.OrderID = newOrder.OrderId
		order.Type = "MARKET"
		order.ModifyCount++
		order.LastModified = time.Now()
	}
	
	return nil
}

// checkPriceActionCondition checks if a price action condition is met
func (om *OrderManager) checkPriceActionCondition(order *ManagedOrder, rule PriceActionRule, currentPrice float64) bool {
	switch rule.Condition {
	case "PRICE_ABOVE":
		return currentPrice > rule.Threshold
		
	case "PRICE_BELOW":
		return currentPrice < rule.Threshold
		
	case "PRICE_DISTANCE":
		// Check if price is within certain distance from order price
		distance := ((currentPrice - order.Price) / order.Price) * 100
		return distance > rule.Threshold
		
	// Add more conditions as needed
	default:
		return false
	}
}

// executePriceActionRule executes a price action rule
func (om *OrderManager) executePriceActionRule(order *ManagedOrder, rule PriceActionRule, currentPrice float64) error {
	switch rule.Action {
	case "CANCEL":
		log.Printf("Canceling order %d due to price action rule '%s'", order.OrderID, rule.Name)
		return om.client.CancelOrder(order.Symbol, order.OrderID)
		
	case "MODIFY_TO_MARKET":
		log.Printf("Converting order %d to market due to price action rule '%s'", order.OrderID, rule.Name)
		
		if err := om.client.CancelOrder(order.Symbol, order.OrderID); err != nil {
			return err
		}
		
		params := map[string]string{
			"symbol":   order.Symbol,
			"side":     order.Side,
			"type":     "MARKET",
			"quantity": fmt.Sprintf("%.8f", order.Quantity),
		}
		
		_, err := om.client.PlaceOrder(params)
		return err
		
	case "ADJUST_PRICE":
		adjustment, ok := rule.Parameters["adjustment"].(float64)
		if !ok {
			return fmt.Errorf("invalid adjustment parameter")
		}
		
		newPrice := currentPrice * (1 + adjustment)
		log.Printf("Adjusting order %d price to %.2f based on current price %.2f", 
			order.OrderID, newPrice, currentPrice)
		
		// Cancel and replace
		if err := om.client.CancelOrder(order.Symbol, order.OrderID); err != nil {
			return err
		}
		
		params := map[string]string{
			"symbol":      order.Symbol,
			"side":        order.Side,
			"type":        order.Type,
			"quantity":    fmt.Sprintf("%.8f", order.Quantity),
			"price":       fmt.Sprintf("%.2f", newPrice),
			"timeInForce": "GTC",
		}
		
		newOrder, err := om.client.PlaceOrder(params)
		if err != nil {
			return err
		}
		
		order.OrderID = newOrder.OrderId
		order.Price = newPrice
		order.ModifyCount++
		order.LastModified = time.Now()
	}
	
	return nil
}

// RemoveOrder removes an order from active management
func (om *OrderManager) RemoveOrder(orderID int64) {
	if order, exists := om.activeOrders[orderID]; exists {
		om.orderHistory = append(om.orderHistory, order)
		delete(om.activeOrders, orderID)
		log.Printf("Order %d removed from active management", orderID)
	}
}

// GetActiveOrders returns all active orders
func (om *OrderManager) GetActiveOrders() map[int64]*ManagedOrder {
	return om.activeOrders
}

// GetOrderHistory returns order history
func (om *OrderManager) GetOrderHistory() []*ManagedOrder {
	return om.orderHistory
}

// Example usage patterns:

// Pattern 1: Time-based order cancellation
// If order not filled within 30 minutes, cancel it
// rule := TimeBasedRule{
//     Name:        "30min_timeout",
//     TriggerTime: time.Now().Add(30 * time.Minute),
//     Action:      "CANCEL",
// }

// Pattern 2: Aggressive fill strategy
// If price moves 1% in our direction, convert to market order
// rule := PriceActionRule{
//     Name:      "aggressive_fill",
//     Condition: "PRICE_DISTANCE",
//     Threshold: 1.0, // 1%
//     Action:    "MODIFY_TO_MARKET",
// }

// Pattern 3: Price chase
// If price moves away by 0.5%, adjust our limit order
// rule := PriceActionRule{
//     Name:      "price_chase",
//     Condition: "PRICE_DISTANCE",
//     Threshold: 0.5,
//     Action:    "ADJUST_PRICE",
//     Parameters: map[string]interface{}{
//         "adjustment": 0.001, // Move closer by 0.1%
//     },
// }
