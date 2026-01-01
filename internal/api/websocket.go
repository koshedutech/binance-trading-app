package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"binance-trading-bot/internal/events"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// In production, you should check the origin
		return true
	},
}

// WSClient represents a WebSocket client
type WSClient struct {
	conn      *websocket.Conn
	send      chan []byte
	hub       *WSHub
	userID    string // User ID for tracking user-specific connections
	mu        sync.Mutex
	closeChan chan struct{}
}

// WSHub manages all WebSocket clients
type WSHub struct {
	clients     map[*WSClient]bool
	userClients map[string][]*WSClient // Maps userID to their active connections
	broadcast   chan []byte
	register    chan *WSClient
	unregister  chan *WSClient
	mu          sync.RWMutex
}

// NewWSHub creates a new WebSocket hub
func NewWSHub() *WSHub {
	return &WSHub{
		clients:     make(map[*WSClient]bool),
		userClients: make(map[string][]*WSClient),
		broadcast:   make(chan []byte, 4096),
		register:    make(chan *WSClient),
		unregister:  make(chan *WSClient),
	}
}

// Run starts the WebSocket hub
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			// Track user-to-client mapping
			if client.userID != "" {
				h.userClients[client.userID] = append(h.userClients[client.userID], client)
			}
			h.mu.Unlock()
			// Reduced logging - only log at debug level if needed

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// Remove from userClients map
				if client.userID != "" {
					h.removeClientFromUserMap(client)
				}
			}
			h.mu.Unlock()
			// Reduced logging - only log at debug level if needed

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client's send channel is full, unregister it
					// Don't close or delete here - let unregister handle it
					go func(c *WSClient) {
						h.unregister <- c
					}(client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastEvent broadcasts an event to all connected clients
func (h *WSHub) BroadcastEvent(event events.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	select {
	case h.broadcast <- data:
	default:
		log.Println("Broadcast channel full, dropping message")
	}
}

// GetClientCount returns the number of connected clients
func (h *WSHub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// removeClientFromUserMap removes a client from the userClients map
// Caller must hold the write lock (h.mu.Lock())
func (h *WSHub) removeClientFromUserMap(client *WSClient) {
	if clients, ok := h.userClients[client.userID]; ok {
		// Find and remove the client from the slice
		for i, c := range clients {
			if c == client {
				// Remove by replacing with last element and truncating
				h.userClients[client.userID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		// Clean up empty slices
		if len(h.userClients[client.userID]) == 0 {
			delete(h.userClients, client.userID)
		}
	}
}

// DisconnectUser disconnects all WebSocket connections for a specific user
// This is called when a user logs out to clean up their active sessions
func (h *WSHub) DisconnectUser(userID string) {
	if userID == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Get all clients for this user
	clients, ok := h.userClients[userID]
	if !ok || len(clients) == 0 {
		return
	}

	// Close all connections for this user
	for _, client := range clients {
		// Remove from clients map
		if _, exists := h.clients[client]; exists {
			delete(h.clients, client)
			// Close the send channel (will trigger connection close in writePump)
			close(client.send)
			// Signal the close channel
			select {
			case client.closeChan <- struct{}{}:
			default:
				// Channel already closed or blocked
			}
		}
	}

	// Remove all user's clients from the userClients map
	delete(h.userClients, userID)

	log.Printf("Disconnected %d WebSocket connections for user %s", len(clients), userID)
}

// writePump pumps messages from the hub to the websocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-c.closeChan:
			return
		}
	}
}

// readPump pumps messages from the websocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		close(c.closeChan)
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}
		// We don't expect messages from clients, but if we did, we'd process them here
	}
}

// Global WebSocket hub
var wsHub *WSHub

// InitWebSocket initializes the WebSocket hub and subscribes to events
func InitWebSocket(eventBus *events.EventBus) *WSHub {
	wsHub = NewWSHub()

	// Start the hub
	go wsHub.Run()

	// Subscribe to all events and broadcast them via WebSocket
	eventBus.SubscribeAll(func(event events.Event) {
		wsHub.BroadcastEvent(event)
	})

	log.Println("WebSocket hub initialized")

	return wsHub
}

// DisconnectUserWebSockets disconnects all WebSocket connections for a specific user
// This is a public function that can be called from other packages (e.g., auth during logout)
func DisconnectUserWebSockets(userID string) {
	if wsHub != nil {
		wsHub.DisconnectUser(userID)
	}
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	// Get user ID from context (empty string if not authenticated)
	userID := s.getUserID(c)

	client := &WSClient{
		conn:      conn,
		send:      make(chan []byte, 256),
		hub:       wsHub,
		userID:    userID,
		closeChan: make(chan struct{}),
	}

	client.hub.register <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	// Send initial connection confirmation
	welcomeMsg := map[string]interface{}{
		"type":      "CONNECTED",
		"message":   "WebSocket connection established",
		"timestamp": time.Now(),
	}
	if data, err := json.Marshal(welcomeMsg); err == nil {
		select {
		case client.send <- data:
		default:
		}
	}
}

// BroadcastPositionUpdate broadcasts a position update to all clients
func BroadcastPositionUpdate(positions []map[string]interface{}) {
	if wsHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventPositionUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"positions": positions,
		},
	}

	wsHub.BroadcastEvent(event)
}

// BroadcastPriceUpdate broadcasts a price update to all clients
func BroadcastPriceUpdate(symbol string, price float64) {
	if wsHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventPriceUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"symbol": symbol,
			"price":  price,
		},
	}

	wsHub.BroadcastEvent(event)
}
