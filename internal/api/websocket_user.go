package api

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"binance-trading-bot/internal/auth"
	"binance-trading-bot/internal/events"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// UserWSClient represents a user-specific WebSocket client
type UserWSClient struct {
	conn      *websocket.Conn
	send      chan []byte
	hub       *UserWSHub
	userID    string
	mu        sync.Mutex
	closeChan chan struct{}
}

// UserWSHub manages user-specific WebSocket clients
type UserWSHub struct {
	// All connected clients (for global broadcasts)
	clients map[*UserWSClient]bool
	// User-specific client mappings
	userClients map[string]map[*UserWSClient]bool
	broadcast   chan []byte
	userCast    chan userMessage
	register    chan *UserWSClient
	unregister  chan *UserWSClient
	mu          sync.RWMutex
}

type userMessage struct {
	userID string
	data   []byte
}

// NewUserWSHub creates a new user-aware WebSocket hub
func NewUserWSHub() *UserWSHub {
	return &UserWSHub{
		clients:     make(map[*UserWSClient]bool),
		userClients: make(map[string]map[*UserWSClient]bool),
		broadcast:   make(chan []byte, 256),
		userCast:    make(chan userMessage, 256),
		register:    make(chan *UserWSClient),
		unregister:  make(chan *UserWSClient),
	}
}

// Run starts the user-aware WebSocket hub
func (h *UserWSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			// Add to user-specific map
			if client.userID != "" {
				if h.userClients[client.userID] == nil {
					h.userClients[client.userID] = make(map[*UserWSClient]bool)
				}
				h.userClients[client.userID][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// Remove from user-specific map
				if client.userID != "" {
					if userClients, ok := h.userClients[client.userID]; ok {
						delete(userClients, client)
						if len(userClients) == 0 {
							delete(h.userClients, client.userID)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Broadcast to all clients
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()

		case userMsg := <-h.userCast:
			// Broadcast to specific user's clients
			h.mu.RLock()
			if userClients, ok := h.userClients[userMsg.userID]; ok {
				for client := range userClients {
					select {
					case client.send <- userMsg.data:
					default:
						close(client.send)
						delete(userClients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastToUser sends an event to a specific user's connections
func (h *UserWSHub) BroadcastToUser(userID string, event events.Event) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal user event: %v", err)
		return
	}

	select {
	case h.userCast <- userMessage{userID: userID, data: data}:
	default:
		log.Printf("User broadcast channel full for user %s, dropping message", userID)
	}
}

// BroadcastToAll sends an event to all connected clients
func (h *UserWSHub) BroadcastToAll(event events.Event) {
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

// GetUserClientCount returns the number of connected clients for a user
func (h *UserWSHub) GetUserClientCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if userClients, ok := h.userClients[userID]; ok {
		return len(userClients)
	}
	return 0
}

// GetTotalClientCount returns the total number of connected clients
func (h *UserWSHub) GetTotalClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetConnectedUsers returns a list of user IDs with active connections
func (h *UserWSHub) GetConnectedUsers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	users := make([]string, 0, len(h.userClients))
	for userID := range h.userClients {
		users = append(users, userID)
	}
	return users
}

// writePump pumps messages from the hub to the websocket connection
func (c *UserWSClient) writePump() {
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
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
func (c *UserWSClient) readPump() {
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
	}
}

// Global user-aware WebSocket hub
var userWSHub *UserWSHub

// InitUserWebSocket initializes the user-aware WebSocket hub
func InitUserWebSocket(eventBus *events.EventBus) *UserWSHub {
	userWSHub = NewUserWSHub()

	// Start the hub
	go userWSHub.Run()

	log.Println("User-aware WebSocket hub initialized")

	return userWSHub
}

// handleUserWebSocket handles user-authenticated WebSocket connections
func (s *Server) handleUserWebSocket(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID := s.getUserID(c)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := &UserWSClient{
		conn:      conn,
		send:      make(chan []byte, 256),
		hub:       userWSHub,
		userID:    userID,
		closeChan: make(chan struct{}),
	}

	client.hub.register <- client

	// Start client goroutines
	go client.writePump()
	go client.readPump()

	// Send initial connection confirmation with user info
	welcomeMsg := map[string]interface{}{
		"type":      "CONNECTED",
		"message":   "WebSocket connection established",
		"timestamp": time.Now(),
		"user_id":   userID,
	}
	if data, err := json.Marshal(welcomeMsg); err == nil {
		select {
		case client.send <- data:
		default:
		}
	}
}

// BroadcastUserPositionUpdate broadcasts a position update to a specific user
func BroadcastUserPositionUpdate(userID string, positions []map[string]interface{}) {
	if userWSHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventPositionUpdate,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"positions": positions,
		},
	}

	userWSHub.BroadcastToUser(userID, event)
}

// BroadcastUserTradeUpdate broadcasts a trade update to a specific user
func BroadcastUserTradeUpdate(userID string, trade map[string]interface{}) {
	if userWSHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventTradeUpdate,
		Timestamp: time.Now(),
		Data:      trade,
	}

	userWSHub.BroadcastToUser(userID, event)
}

// BroadcastUserSignal broadcasts a signal to a specific user
func BroadcastUserSignal(userID string, signal map[string]interface{}) {
	if userWSHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventSignalGenerated,
		Timestamp: time.Now(),
		Data:      signal,
	}

	userWSHub.BroadcastToUser(userID, event)
}

// BroadcastUserOrderUpdate broadcasts an order update to a specific user
func BroadcastUserOrderUpdate(userID string, order map[string]interface{}) {
	if userWSHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventOrderUpdate,
		Timestamp: time.Now(),
		Data:      order,
	}

	userWSHub.BroadcastToUser(userID, event)
}

// BroadcastUserBalanceUpdate broadcasts a balance update to a specific user
func BroadcastUserBalanceUpdate(userID string, balance map[string]interface{}) {
	if userWSHub == nil {
		return
	}

	event := events.Event{
		Type:      events.EventBalanceUpdate,
		Timestamp: time.Now(),
		Data:      balance,
	}

	userWSHub.BroadcastToUser(userID, event)
}

// GetUserWSHub returns the global user WebSocket hub
func GetUserWSHub() *UserWSHub {
	return userWSHub
}

// AuthenticatedWSHandler creates a WebSocket handler that requires authentication
// Supports both Authorization header and query param token for WebSocket connections
func AuthenticatedWSHandler(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		if s.authEnabled {
			userID := auth.GetUserID(c)

			// If not authenticated via header, try query param token (for WebSocket)
			if userID == "" {
				token := c.Query("token")
				if token != "" && s.authService != nil {
					// Validate token from query param
					claims, err := s.authService.GetJWTManager().ValidateAccessToken(token)
					if err == nil && claims != nil {
						// Set user context from validated token
						c.Set(auth.ContextKeyUserID, claims.UserID)
						c.Set(auth.ContextKeyEmail, claims.Email)
						c.Set(auth.ContextKeyTier, claims.SubscriptionTier)
						c.Set(auth.ContextKeyAPIMode, claims.APIKeyMode)
						c.Set(auth.ContextKeyIsAdmin, claims.IsAdmin)
						c.Set(auth.ContextKeyClaims, claims)
						userID = claims.UserID
						log.Printf("[WS-AUTH] User %s authenticated via query token", userID)
					}
				}
			}

			if userID == "" {
				c.JSON(401, gin.H{
					"error":   "UNAUTHORIZED",
					"message": "authentication required for WebSocket connection",
				})
				return
			}
		}

		s.handleUserWebSocket(c)
	}
}
