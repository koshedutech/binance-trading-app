package api

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"binance-trading-bot/internal/events"
)

// TestNewUserWSHub tests hub creation
func TestNewUserWSHub(t *testing.T) {
	hub := NewUserWSHub()

	if hub == nil {
		t.Fatal("NewUserWSHub returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map not initialized")
	}

	if hub.userClients == nil {
		t.Error("userClients map not initialized")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}

	if hub.userCast == nil {
		t.Error("userCast channel not initialized")
	}
}

// TestBroadcastChainUpdateWithNilHub tests chain update with nil hub doesn't panic
func TestBroadcastChainUpdateWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	// Should not panic when hub is nil
	BroadcastChainUpdate("user123", map[string]interface{}{
		"chainId": "test-chain",
		"status":  "active",
	})
}

// TestBroadcastChainUpdateCreatesEvent tests that broadcast creates correct event structure
func TestBroadcastChainUpdateCreatesEvent(t *testing.T) {
	hub := NewUserWSHub()
	oldHub := userWSHub
	userWSHub = hub
	defer func() { userWSHub = oldHub }()

	// Create a test client to receive the message
	received := make(chan []byte, 1)
	client := &UserWSClient{
		send:      make(chan []byte, 256),
		hub:       hub,
		userID:    "test-user",
		closeChan: make(chan struct{}),
	}

	// Register client
	hub.mu.Lock()
	hub.clients[client] = true
	hub.userClients["test-user"] = map[*UserWSClient]bool{client: true}
	hub.mu.Unlock()

	// Start a goroutine to capture the message
	go func() {
		select {
		case msg := <-client.send:
			received <- msg
		case <-time.After(100 * time.Millisecond):
			received <- nil
		}
	}()

	// Start hub processing
	go hub.Run()

	// Broadcast event
	BroadcastChainUpdate("test-user", map[string]interface{}{
		"chainId": "chain-123",
		"status":  "active",
	})

	// Wait for message
	select {
	case msg := <-received:
		if msg == nil {
			t.Error("No message received within timeout")
			return
		}

		// Verify event structure
		var event map[string]interface{}
		if err := json.Unmarshal(msg, &event); err != nil {
			t.Errorf("Failed to unmarshal event: %v", err)
			return
		}

		if event["type"] != string(events.EventChainUpdate) {
			t.Errorf("Expected type %s, got %v", events.EventChainUpdate, event["type"])
		}

		if event["timestamp"] == nil {
			t.Error("Event missing timestamp")
		}

		if event["data"] == nil {
			t.Error("Event missing data")
		}

	case <-time.After(200 * time.Millisecond):
		t.Error("Timeout waiting for broadcast message")
	}
}

// TestBroadcastLifecycleEventWithNilHub tests lifecycle event with nil hub doesn't panic
func TestBroadcastLifecycleEventWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastLifecycleEvent("user123", map[string]interface{}{
		"tradeId":   "trade-123",
		"eventType": "POSITION_OPENED",
		"symbol":    "BTCUSDT",
	})
}

// TestBroadcastGinieStatusWithNilHub tests Ginie status with nil hub doesn't panic
func TestBroadcastGinieStatusWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastGinieStatus("user123", map[string]interface{}{
		"isRunning":       true,
		"currentMode":     "scalp",
		"activePositions": 2,
		"lastSignalTime":  time.Now().Format(time.RFC3339),
	})
}

// TestBroadcastCircuitBreakerWithNilHub tests circuit breaker with nil hub doesn't panic
func TestBroadcastCircuitBreakerWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastCircuitBreaker("user123", map[string]interface{}{
		"isTriggered":   true,
		"triggerReason": "max_loss_exceeded",
		"triggeredAt":   time.Now().Format(time.RFC3339),
	})
}

// TestBroadcastPnLWithNilHub tests P&L with nil hub doesn't panic
func TestBroadcastPnLWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastPnL("user123", map[string]interface{}{
		"totalPnL":      1234.56,
		"dailyPnL":      100.50,
		"unrealizedPnL": 50.25,
		"realizedPnL":   50.25,
	})
}

// TestBroadcastModeStatusWithNilHub tests mode status with nil hub doesn't panic
func TestBroadcastModeStatusWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastModeStatus("user123", map[string]interface{}{
		"mode":            "scalp",
		"enabled":         true,
		"activePositions": 1,
		"status":          "active",
	})
}

// TestBroadcastSystemStatusWithNilHub tests system status with nil hub doesn't panic
func TestBroadcastSystemStatusWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastSystemStatus("user123", map[string]interface{}{
		"binanceConnected":  true,
		"databaseConnected": true,
		"redisConnected":    true,
		"websocketClients":  5,
	})
}

// TestBroadcastSignalUpdateWithNilHub tests signal update with nil hub doesn't panic
func TestBroadcastSignalUpdateWithNilHub(t *testing.T) {
	oldHub := userWSHub
	userWSHub = nil
	defer func() { userWSHub = oldHub }()

	BroadcastSignalUpdate("user123", map[string]interface{}{
		"id":         "signal-123",
		"symbol":     "BTCUSDT",
		"direction":  "LONG",
		"confidence": 85.5,
	})
}

// TestUserWSHubBroadcastToUser tests user-specific broadcasting
func TestUserWSHubBroadcastToUser(t *testing.T) {
	hub := NewUserWSHub()

	event := events.Event{
		Type:      events.EventChainUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"test": "data",
		},
	}

	// Test broadcasting to non-existent user (should not block or panic)
	done := make(chan bool, 1)
	go func() {
		hub.BroadcastToUser("nonexistent-user", event)
		done <- true
	}()

	select {
	case <-done:
		// Success - broadcast completed without hanging
	case <-time.After(time.Second):
		t.Error("BroadcastToUser blocked for non-existent user")
	}
}

// TestUserWSHubBroadcastToAll tests global broadcasting
func TestUserWSHubBroadcastToAll(t *testing.T) {
	hub := NewUserWSHub()

	event := events.Event{
		Type:      events.EventSystemStatusUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"status": "healthy",
		},
	}

	// Test broadcasting to all (with no clients)
	done := make(chan bool, 1)
	go func() {
		hub.BroadcastToAll(event)
		done <- true
	}()

	select {
	case <-done:
		// Success - broadcast completed without hanging
	case <-time.After(time.Second):
		t.Error("BroadcastToAll blocked with no clients")
	}
}

// TestEventTypeConstants tests that all event type constants are properly defined
func TestEventTypeConstants(t *testing.T) {
	// Verify all Epic 12 event types are defined and unique
	eventTypes := map[events.EventType]string{
		events.EventChainUpdate:          "CHAIN_UPDATE",
		events.EventLifecycleEvent:       "LIFECYCLE_EVENT",
		events.EventGinieStatusUpdate:    "GINIE_STATUS_UPDATE",
		events.EventCircuitBreakerUpdate: "CIRCUIT_BREAKER_UPDATE",
		events.EventPnLUpdate:            "PNL_UPDATE",
		events.EventModeStatusUpdate:     "MODE_STATUS_UPDATE",
		events.EventSystemStatusUpdate:   "SYSTEM_STATUS_UPDATE",
		events.EventSignalUpdate:         "SIGNAL_UPDATE",
	}

	for eventType, expectedValue := range eventTypes {
		if string(eventType) != expectedValue {
			t.Errorf("Event type %s has incorrect value: expected %s, got %s",
				expectedValue, expectedValue, string(eventType))
		}
	}

	// Verify no duplicate values
	seen := make(map[string]bool)
	for eventType := range eventTypes {
		val := string(eventType)
		if seen[val] {
			t.Errorf("Duplicate event type value: %s", val)
		}
		seen[val] = true
	}
}

// TestEventMarshal tests that events can be marshaled to JSON
func TestEventMarshal(t *testing.T) {
	testCases := []struct {
		name      string
		eventType events.EventType
		data      map[string]interface{}
	}{
		{
			name:      "ChainUpdate",
			eventType: events.EventChainUpdate,
			data: map[string]interface{}{
				"chainId": "test-chain",
				"symbol":  "BTCUSDT",
				"status":  "active",
			},
		},
		{
			name:      "LifecycleEvent",
			eventType: events.EventLifecycleEvent,
			data: map[string]interface{}{
				"tradeId":   "trade-123",
				"eventType": "POSITION_OPENED",
			},
		},
		{
			name:      "GinieStatus",
			eventType: events.EventGinieStatusUpdate,
			data: map[string]interface{}{
				"isRunning":   true,
				"currentMode": "scalp",
			},
		},
		{
			name:      "CircuitBreaker",
			eventType: events.EventCircuitBreakerUpdate,
			data: map[string]interface{}{
				"isTriggered":   false,
				"triggerReason": nil,
			},
		},
		{
			name:      "PnLUpdate",
			eventType: events.EventPnLUpdate,
			data: map[string]interface{}{
				"totalPnL": 1234.56,
				"dailyPnL": 100.50,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := events.Event{
				Type:      tc.eventType,
				Timestamp: time.Now(),
				Data:      tc.data,
			}

			data, err := json.Marshal(event)
			if err != nil {
				t.Errorf("Failed to marshal event: %v", err)
			}

			// Verify JSON contains expected fields
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Errorf("Failed to unmarshal event: %v", err)
			}

			if parsed["type"] != string(tc.eventType) {
				t.Errorf("Event type mismatch: expected %s, got %v",
					tc.eventType, parsed["type"])
			}

			if _, ok := parsed["timestamp"]; !ok {
				t.Error("Event missing timestamp field")
			}

			if _, ok := parsed["data"]; !ok {
				t.Error("Event missing data field")
			}
		})
	}
}

// TestUserClientCounts tests client counting methods
func TestUserClientCounts(t *testing.T) {
	hub := NewUserWSHub()

	// Initially no clients
	if count := hub.GetTotalClientCount(); count != 0 {
		t.Errorf("Expected 0 total clients, got %d", count)
	}

	if count := hub.GetUserClientCount("user123"); count != 0 {
		t.Errorf("Expected 0 clients for user123, got %d", count)
	}

	// Test GetConnectedUsers with no users
	users := hub.GetConnectedUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 connected users, got %d", len(users))
	}
}

// TestBroadcastEmptyUserID tests broadcasting with empty user ID is safely ignored
func TestBroadcastEmptyUserID(t *testing.T) {
	hub := NewUserWSHub()
	oldHub := userWSHub
	userWSHub = hub
	defer func() { userWSHub = oldHub }()

	// These should be no-ops with empty userID (no panic, no broadcast)
	BroadcastChainUpdate("", map[string]interface{}{"test": "data"})
	BroadcastLifecycleEvent("", map[string]interface{}{"test": "data"})
	BroadcastGinieStatus("", map[string]interface{}{"test": "data"})
	BroadcastCircuitBreaker("", map[string]interface{}{"test": "data"})
	BroadcastPnL("", map[string]interface{}{"test": "data"})

	// Verify hub is still functional
	if hub.GetTotalClientCount() != 0 {
		t.Error("Hub state corrupted after empty userID broadcasts")
	}
}

// TestUserIsolation verifies events are only sent to the correct user
func TestUserIsolation(t *testing.T) {
	hub := NewUserWSHub()
	oldHub := userWSHub
	userWSHub = hub
	defer func() { userWSHub = oldHub }()

	// Create clients for two different users
	user1Received := make(chan []byte, 10)
	user2Received := make(chan []byte, 10)

	client1 := &UserWSClient{
		send:      make(chan []byte, 256),
		hub:       hub,
		userID:    "user-1",
		closeChan: make(chan struct{}),
	}
	client2 := &UserWSClient{
		send:      make(chan []byte, 256),
		hub:       hub,
		userID:    "user-2",
		closeChan: make(chan struct{}),
	}

	// Register clients
	hub.mu.Lock()
	hub.clients[client1] = true
	hub.clients[client2] = true
	hub.userClients["user-1"] = map[*UserWSClient]bool{client1: true}
	hub.userClients["user-2"] = map[*UserWSClient]bool{client2: true}
	hub.mu.Unlock()

	// Collect messages
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		select {
		case msg := <-client1.send:
			user1Received <- msg
		case <-time.After(100 * time.Millisecond):
		}
	}()

	go func() {
		defer wg.Done()
		select {
		case msg := <-client2.send:
			user2Received <- msg
		case <-time.After(100 * time.Millisecond):
		}
	}()

	// Start hub
	go hub.Run()

	// Broadcast to user-1 only
	BroadcastChainUpdate("user-1", map[string]interface{}{"target": "user-1"})

	wg.Wait()

	// User 1 should receive message
	select {
	case msg := <-user1Received:
		if msg == nil {
			t.Error("User 1 should have received a message")
		}
	default:
		t.Error("User 1 did not receive message")
	}

	// User 2 should NOT receive message
	select {
	case msg := <-user2Received:
		if msg != nil {
			t.Error("User 2 should NOT have received a message (isolation failed)")
		}
	default:
		// This is expected - user 2 should not receive anything
	}
}

// TestConcurrentBroadcasts tests thread safety with concurrent broadcasts
func TestConcurrentBroadcasts(t *testing.T) {
	hub := NewUserWSHub()
	oldHub := userWSHub
	userWSHub = hub
	defer func() { userWSHub = oldHub }()

	// Start hub
	go hub.Run()

	// Create test client
	client := &UserWSClient{
		send:      make(chan []byte, 1000),
		hub:       hub,
		userID:    "concurrent-user",
		closeChan: make(chan struct{}),
	}

	hub.mu.Lock()
	hub.clients[client] = true
	hub.userClients["concurrent-user"] = map[*UserWSClient]bool{client: true}
	hub.mu.Unlock()

	// Drain channel in background
	go func() {
		for range client.send {
		}
	}()

	// Launch concurrent broadcasts
	var wg sync.WaitGroup
	numGoroutines := 50
	numBroadcasts := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numBroadcasts; j++ {
				BroadcastChainUpdate("concurrent-user", map[string]interface{}{
					"goroutine": id,
					"broadcast": j,
				})
			}
		}(i)
	}

	// Should complete without deadlock or panic
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("Concurrent broadcasts timed out - possible deadlock")
	}
}
