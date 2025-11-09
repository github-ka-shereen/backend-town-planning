// websocket/hub.go
package websocket

import (
	"sync"
	"time"

	applications_services "town-planning-backend/applications/services"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

type MessageType string

const (
	MessageTypeChat        MessageType = "CHAT_MESSAGE"
	MessageTypeTyping      MessageType = "TYPING_INDICATOR"
	MessageTypeReadReceipt MessageType = "READ_RECEIPT"
	MessageTypeMessageRead MessageType = "MESSAGE_READ"
	MessageTypeUserStatus  MessageType = "USER_STATUS"
	MessageTypeError       MessageType = "ERROR"
)

type WebSocketMessage struct {
	Type      MessageType `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
	ThreadID  string      `json:"threadId,omitempty"`
}

type Client struct {
    ID                uuid.UUID
    UserID            uuid.UUID
    Conn              *websocket.Conn
    Hub               *Hub
    Send              chan WebSocketMessage
    Threads           map[string]bool
    mu                sync.RWMutex
    readReceiptService applications_services.ReadReceiptService // Add this line
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan WebSocketMessage
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan WebSocketMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.broadcastToAll(message)
		}
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message WebSocketMessage) {
	h.broadcast <- message
}

// BroadcastToThread sends a message to clients subscribed to a specific thread
func (h *Hub) BroadcastToThread(threadID string, message WebSocketMessage, excludeUserID ...uuid.UUID) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	excludeMap := make(map[uuid.UUID]bool)
	for _, id := range excludeUserID {
		excludeMap[id] = true
	}

	for client := range h.clients {
		// Skip excluded users
		if excludeMap[client.UserID] {
			continue
		}

		// Check if client is subscribed to this thread
		client.mu.RLock()
		_, isSubscribed := client.Threads[threadID]
		client.mu.RUnlock()

		if isSubscribed {
			select {
			case client.Send <- message:
			default:
				close(client.Send)
				delete(h.clients, client)
			}
		}
	}
}

// broadcastToAll sends a message to all connected clients
func (h *Hub) broadcastToAll(message WebSocketMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.Send <- message:
		default:
			close(client.Send)
			delete(h.clients, client)
		}
	}
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetThreadSubscribers returns all clients subscribed to a thread
func (h *Hub) GetThreadSubscribers(threadID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var subscribers []*Client
	for client := range h.clients {
		client.mu.RLock()
		_, isSubscribed := client.Threads[threadID]
		client.mu.RUnlock()

		if isSubscribed {
			subscribers = append(subscribers, client)
		}
	}
	return subscribers
}

// SubscribeToThread adds a thread to client's subscription
func (c *Client) SubscribeToThread(threadID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Threads == nil {
		c.Threads = make(map[string]bool)
	}
	c.Threads[threadID] = true
}

// UnsubscribeFromThread removes a thread from client's subscription
func (c *Client) UnsubscribeFromThread(threadID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.Threads, threadID)
}

// IsSubscribedToThread checks if client is subscribed to a thread
func (c *Client) IsSubscribedToThread(threadID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.Threads[threadID]
	return exists
}