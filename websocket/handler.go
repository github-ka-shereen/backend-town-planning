// websocket/handler.go
package websocket

import (
	"fmt"
	"time"
	applications_services "town-planning-backend/applications/services"
	"town-planning-backend/config"
	"town-planning-backend/token"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AuthService defines a token validator interface
type AuthService interface {
	VerifyToken(token string) (*token.Payload, error)
}

// WsHandler manages WebSocket requests and connections
type WsHandler struct {
	hub                *Hub
	auth               AuthService
	readReceiptService applications_services.ReadReceiptService
}

// NewWsHandler creates a new WebSocket handler instance
func NewWsHandler(hub *Hub, auth AuthService, readReceiptService applications_services.ReadReceiptService) *WsHandler {
	return &WsHandler{
		hub:                hub,
		auth:               auth,
		readReceiptService: readReceiptService,
	}
}

// HandleWebSocket handles incoming WebSocket upgrade requests
func (h *WsHandler) HandleWebSocket(c *fiber.Ctx) error {
	// Check if it's a WebSocket connection
	if !websocket.IsWebSocketUpgrade(c) {
		return fiber.ErrUpgradeRequired
	}

	// SECURITY: Extract token from HTTPOnly cookie (not query parameter)
	tokenStr := c.Cookies("access_token")
	if tokenStr == "" {
		config.Logger.Warn("WebSocket connection attempted without access token cookie")
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication required - no access token cookie found",
		})
	}

	// Validate the token
	payload, err := h.auth.VerifyToken(tokenStr)
	if err != nil {
		config.Logger.Warn("Invalid access token for WebSocket",
			zap.Error(err),
		)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	// Get thread ID from query parameters (this is safe - not sensitive data)
	threadID := c.Query("thread")
	if threadID == "" {
		config.Logger.Warn("WebSocket connection attempted without thread ID",
			zap.String("userID", payload.UserID.String()),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "thread parameter is required",
		})
	}

	// Validate thread ID format
	if _, err := uuid.Parse(threadID); err != nil {
		config.Logger.Warn("Invalid thread ID format",
			zap.String("threadID", threadID),
			zap.String("userID", payload.UserID.String()),
		)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid thread ID format",
		})
	}

	// Log successful authentication
	config.Logger.Info("WebSocket connection authenticated",
		zap.String("userID", payload.UserID.String()),
		zap.String("threadID", threadID),
	)

	// Upgrade to WebSocket using Fiber's websocket package
	return websocket.New(func(conn *websocket.Conn) {
		client := &Client{
			ID:                 uuid.New(),
			UserID:             payload.UserID,
			Conn:               conn,
			Hub:                h.hub,
			Send:               make(chan WebSocketMessage, 256),
			Threads:            make(map[string]bool),
			readReceiptService: h.readReceiptService, // Add this line
		}

		// Auto-subscribe client to the thread they connected with
		client.Threads[threadID] = true

		// Register client
		h.hub.register <- client

		config.Logger.Info("WebSocket client registered",
			zap.String("clientID", client.ID.String()),
			zap.String("userID", client.UserID.String()),
			zap.String("threadID", threadID),
		)

		// Start goroutines for this client
		go client.writePump()
		client.readPump()
	})(c)
}

// readPump listens for incoming messages from the WebSocket
func (c *Client) readPump() {
	defer func() {
		config.Logger.Info("WebSocket client disconnecting",
			zap.String("clientID", c.ID.String()),
			zap.String("userID", c.UserID.String()),
		)
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	// Set connection limits
	c.Conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg WebSocketMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			// Log the error for debugging
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				config.Logger.Warn("WebSocket unexpected close",
					zap.String("clientID", c.ID.String()),
					zap.Error(err),
				)
			}
			break
		}

		// Validate message has timestamp
		if msg.Timestamp.IsZero() {
			msg.Timestamp = time.Now()
		}

		// Log received message
		config.Logger.Debug("WebSocket message received",
			zap.String("clientID", c.ID.String()),
			zap.String("type", string(msg.Type)),
			zap.String("threadID", msg.ThreadID),
		)

		switch msg.Type {
		case MessageTypeTyping:
			c.handleTypingIndicator(msg)
		case MessageTypeReadReceipt:
			c.handleReadReceipt(msg)
		case MessageTypeChat:
			c.broadcastMessageDelivery(msg)
		case MessageTypeUserStatus:
			c.handleUserStatus(msg)
		default:
			config.Logger.Warn("Unknown WebSocket message type",
				zap.String("type", string(msg.Type)),
				zap.String("clientID", c.ID.String()),
			)
			c.sendError("Unknown message type: " + string(msg.Type))
		}
	}
}

// writePump sends queued messages and keeps the connection alive
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteJSON(message); err != nil {
				config.Logger.Debug("WebSocket write error",
					zap.String("clientID", c.ID.String()),
					zap.Error(err),
				)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			// Send ping to keep connection alive
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				config.Logger.Debug("WebSocket ping error",
					zap.String("clientID", c.ID.String()),
					zap.Error(err),
				)
				return
			}
		}
	}
}

// handleTypingIndicator processes typing indicators from clients
func (c *Client) handleTypingIndicator(msg WebSocketMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		c.sendError("Invalid typing indicator payload")
		return
	}

	threadID, hasThread := payload["threadId"].(string)
	isTyping, hasTyping := payload["isTyping"].(bool)

	if !hasThread || !hasTyping {
		c.sendError("Missing required fields in typing indicator")
		return
	}

	// Validate thread ID format
	if _, err := uuid.Parse(threadID); err != nil {
		c.sendError("Invalid thread ID format")
		return
	}

	// Add user info to the payload
	payload["userId"] = c.UserID
	msg.Payload = payload
	msg.ThreadID = threadID

	// Broadcast to other clients in the same thread (excluding sender)
	c.Hub.BroadcastToThread(threadID, msg, c.UserID)

	config.Logger.Debug("Typing indicator handled",
		zap.String("threadId", threadID),
		zap.Bool("isTyping", isTyping),
		zap.String("userId", c.UserID.String()))
}

// handleReadReceipt processes read receipts from clients
func (c *Client) handleReadReceipt(msg WebSocketMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		c.sendError("Invalid read receipt payload")
		return
	}

	threadID, hasThread := payload["threadId"].(string)
	messageIDs, hasMessages := payload["messageIds"].([]interface{})

	if !hasThread || !hasMessages {
		c.sendError("Missing required fields in read receipt")
		return
	}

	// Validate thread ID format
	if _, err := uuid.Parse(threadID); err != nil {
		c.sendError("Invalid thread ID format")
		return
	}

	// Convert message IDs to string slice
	var messageIDStrings []string
	for _, id := range messageIDs {
		if str, ok := id.(string); ok {
			messageIDStrings = append(messageIDStrings, str)
		}
	}

	// USE THE INJECTED SERVICE: Save read receipts to database
	processedCount, err := c.readReceiptService.ProcessReadReceipts(
		threadID,
		c.UserID,
		messageIDStrings,
		true, // isRealtime
	)

	if err != nil {
		config.Logger.Error("Failed to process read receipts via WebSocket",
			zap.Error(err),
			zap.String("threadID", threadID),
			zap.String("userID", c.UserID.String()))
		c.sendError("Failed to save read receipts: " + err.Error())
		return
	}

	// Add user info and message IDs to the payload
	payload["userId"] = c.UserID
	msg.Payload = payload
	msg.ThreadID = threadID

	// Broadcast to other clients in the same thread
	c.Hub.BroadcastToThread(threadID, msg, c.UserID)

	config.Logger.Debug("Read receipt handled and saved to database",
		zap.String("threadId", threadID),
		zap.Int("messageCount", len(messageIDStrings)),
		zap.Int("processedCount", processedCount),
		zap.String("userId", c.UserID.String()))
}

// broadcastMessageDelivery broadcasts message delivery status
func (c *Client) broadcastMessageDelivery(msg WebSocketMessage) {
	config.Logger.Debug("Message delivery broadcast",
		zap.String("clientId", c.ID.String()),
		zap.String("threadId", msg.ThreadID))
}

// handleUserStatus processes user online/offline status
func (c *Client) handleUserStatus(msg WebSocketMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		c.sendError("Invalid user status payload")
		return
	}

	status, hasStatus := payload["status"].(string)
	if hasStatus {
		// Notify all threads this user is part of about their status
		c.mu.RLock()
		for threadID := range c.Threads {
			statusMsg := WebSocketMessage{
				Type: MessageTypeUserStatus,
				Payload: map[string]interface{}{
					"userId": c.UserID,
					"status": status,
				},
				Timestamp: time.Now(),
				ThreadID:  threadID,
			}
			c.Hub.BroadcastToThread(threadID, statusMsg, c.UserID)
		}
		c.mu.RUnlock()
	}
}

// sendError sends an error message back to the client
func (c *Client) sendError(message string) {
	errorMsg := WebSocketMessage{
		Type: MessageTypeError,
		Payload: map[string]interface{}{
			"message": message,
		},
		Timestamp: time.Now(),
	}

	c.Send <- errorMsg
}

// SendMessage sends a message to this specific client
func (c *Client) SendMessage(msg WebSocketMessage) error {
	select {
	case c.Send <- msg:
		return nil
	default:
		return fmt.Errorf("client send channel is full")
	}
}
