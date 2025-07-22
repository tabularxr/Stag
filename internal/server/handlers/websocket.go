package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/tabular/stag-v2/internal/server/websocket"
	"github.com/tabular/stag-v2/pkg/logger"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub      *websocket.Hub
	upgrader websocket.Upgrader
	logger   logger.Logger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *websocket.Hub, logger logger.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// TODO: Implement proper origin check for production
				return true
			},
		},
		logger: logger,
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Get session ID from query parameter
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "session_id query parameter is required",
		})
		return
	}

	// Upgrade connection
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorf("Failed to upgrade connection: %v", err)
		return
	}

	// Create client
	client := websocket.NewClient(h.hub, conn, sessionID, h.logger.WithField("session_id", sessionID))

	// Register client
	h.hub.Register(client)

	// Start client goroutines
	go client.WritePump()
	go client.ReadPump()
}

// Register registers the client with the hub
func (h *websocket.Hub) Register(client *websocket.Client) {
	h.register <- client
}