package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tabular/stag-v2/internal/metrics"
	"github.com/tabular/stag-v2/internal/spatial"
	"github.com/tabular/stag-v2/pkg/api"
	"github.com/tabular/stag-v2/pkg/logger"
)

// Hub manages WebSocket connections and message routing
type Hub struct {
	// Clients organized by session ID
	clients map[string]map[*Client]bool
	mu      sync.RWMutex

	// Channels for client management
	register   chan *Client
	unregister chan *Client
	broadcast  chan BroadcastMessage

	// Dependencies
	repository *spatial.Repository
	logger     logger.Logger
	metrics    *metrics.Metrics

	// Configuration
	maxClientsPerSession int
}

// Client represents a WebSocket client connection
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	sessionID string
	send      chan []byte
	logger    logger.Logger
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	SessionID string
	Message   []byte
	Exclude   *Client // Exclude this client from broadcast
}

// NewHub creates a new WebSocket hub
func NewHub(repository *spatial.Repository, logger logger.Logger, metrics *metrics.Metrics) *Hub {
	return &Hub{
		clients:              make(map[string]map[*Client]bool),
		register:             make(chan *Client),
		unregister:           make(chan *Client),
		broadcast:            make(chan BroadcastMessage),
		repository:           repository,
		logger:               logger,
		metrics:              metrics,
		maxClientsPerSession: 10,
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient adds a new client to the hub
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Initialize session map if needed
	if h.clients[client.sessionID] == nil {
		h.clients[client.sessionID] = make(map[*Client]bool)
	}

	// Check connection limit
	if len(h.clients[client.sessionID]) >= h.maxClientsPerSession {
		h.logger.Warnf("Session %s exceeded max connections (%d)", client.sessionID, h.maxClientsPerSession)
		close(client.send)
		return
	}

	// Add client
	h.clients[client.sessionID][client] = true
	h.metrics.WSConnectionsActive.WithLabelValues(client.sessionID).Inc()

	h.logger.Infof("Client connected to session %s (total: %d)", 
		client.sessionID, len(h.clients[client.sessionID]))
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.sessionID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)
			h.metrics.WSConnectionsActive.WithLabelValues(client.sessionID).Dec()

			// Clean up empty session
			if len(clients) == 0 {
				delete(h.clients, client.sessionID)
			}

			h.logger.Infof("Client disconnected from session %s (remaining: %d)",
				client.sessionID, len(clients))
		}
	}
}

// broadcastMessage sends a message to all clients in a session
func (h *Hub) broadcastMessage(msg BroadcastMessage) {
	h.mu.RLock()
	clients := h.clients[msg.SessionID]
	h.mu.RUnlock()

	if clients == nil {
		return
	}

	for client := range clients {
		// Skip excluded client
		if client == msg.Exclude {
			continue
		}

		select {
		case client.send <- msg.Message:
			// Message sent successfully
		default:
			// Client's send channel is full, close it
			h.logger.Warnf("Client send buffer full, closing connection")
			h.unregister <- client
		}
	}
}

// BroadcastToSession sends a message to all clients in a session
func (h *Hub) BroadcastToSession(sessionID string, message *api.WSMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	h.broadcast <- BroadcastMessage{
		SessionID: sessionID,
		Message:   data,
	}

	return nil
}

// GetActiveConnections returns the number of active connections
func (h *Hub) GetActiveConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, sessionID string, logger logger.Logger) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		sessionID: sessionID,
		send:      make(chan []byte, 256),
		logger:    logger,
	}
}

// ReadPump handles incoming messages from the WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Errorf("WebSocket error: %v", err)
			}
			break
		}

		// Parse message
		var wsMessage api.WSMessage
		if err := json.Unmarshal(message, &wsMessage); err != nil {
			c.logger.Errorf("Failed to parse message: %v", err)
			c.sendError("INVALID_MESSAGE", "Failed to parse message")
			continue
		}

		// Set session ID if not provided
		if wsMessage.SessionID == "" {
			wsMessage.SessionID = c.sessionID
		}

		// Record metric
		c.hub.metrics.WSMessagesTotal.WithLabelValues("inbound", wsMessage.Type, "received").Inc()

		// Handle different message types
		switch wsMessage.Type {
		case api.WSTypePing:
			c.handlePing(&wsMessage)

		case api.WSTypeAnchorUpdate, api.WSTypeMeshUpdate:
			c.handleDataUpdate(&wsMessage)

		default:
			c.logger.Warnf("Unknown message type: %s", wsMessage.Type)
			c.sendError("UNKNOWN_TYPE", "Unknown message type")
		}
	}
}

// WritePump handles sending messages to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write message
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Record metric
			c.hub.metrics.WSMessagesTotal.WithLabelValues("outbound", "data", "sent").Inc()

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handlePing responds to ping messages
func (c *Client) handlePing(msg *api.WSMessage) {
	pong := api.WSMessage{
		Type:      api.WSTypePong,
		SessionID: msg.SessionID,
		Timestamp: time.Now().UnixMilli(),
		TraceID:   msg.TraceID,
	}

	data, err := json.Marshal(pong)
	if err != nil {
		c.logger.Errorf("Failed to marshal pong: %v", err)
		return
	}

	select {
	case c.send <- data:
	default:
		c.logger.Warn("Send buffer full, dropping pong")
	}
}

// handleDataUpdate processes anchor and mesh updates
func (c *Client) handleDataUpdate(msg *api.WSMessage) {
	// Process the update
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.hub.repository.ProcessWebSocketMessage(ctx, msg); err != nil {
		c.logger.Errorf("Failed to process %s: %v", msg.Type, err)
		c.sendError("PROCESSING_ERROR", err.Error())
		c.hub.metrics.WSMessagesTotal.WithLabelValues("inbound", msg.Type, "error").Inc()
		return
	}

	c.hub.metrics.WSMessagesTotal.WithLabelValues("inbound", msg.Type, "success").Inc()

	// Broadcast to other clients in the session
	data, _ := json.Marshal(msg)
	c.hub.broadcast <- BroadcastMessage{
		SessionID: c.sessionID,
		Message:   data,
		Exclude:   c,
	}
}

// sendError sends an error message to the client
func (c *Client) sendError(code, message string) {
	errorMsg := api.WSMessage{
		Type:      api.WSTypeError,
		SessionID: c.sessionID,
		Data: mustMarshal(api.ErrorResponse{
			Code:    code,
			Message: message,
		}),
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(errorMsg)
	if err != nil {
		c.logger.Errorf("Failed to marshal error: %v", err)
		return
	}

	select {
	case c.send <- data:
		c.hub.metrics.WSMessagesTotal.WithLabelValues("outbound", "error", "sent").Inc()
	default:
		c.logger.Warn("Send buffer full, dropping error message")
	}
}

// mustMarshal marshals data to JSON or returns empty JSON on error
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}