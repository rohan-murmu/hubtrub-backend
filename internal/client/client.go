package client

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/scythrine05/hubtrub-server/internal/message"
	"github.com/scythrine05/hubtrub-server/internal/util"

	"github.com/gorilla/websocket"
)

// Client represents a WebSocket client connected to a room
type Client struct {
	ID            string                // Unique client ID
	Conn          *websocket.Conn       // The WebSocket connection
	Send          chan *message.Message // Channel to send messages to the client
	Subscriptions map[string]bool       // Track which pipes this client is subscribed to
	UnregisterC   chan string           // Channel to notify room to unregister
	MessageC      chan *message.Message // Channel to send messages to room
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO: tighten for production
	},
}

func Upgrader() *websocket.Upgrader {
	return &upgrader
}

// NewClient creates a new client
func NewClient(id string, conn *websocket.Conn, unregisterC chan string, messageC chan *message.Message) *Client {
	return &Client{
		ID:            id,
		Conn:          conn,
		Send:          make(chan *message.Message, util.SendBufferSize),
		Subscriptions: make(map[string]bool),
		UnregisterC:   unregisterC,
		MessageC:      messageC,
	}
}

// ReadPump reads messages from the client's WebSocket connection
func (c *Client) ReadPump() {
	defer func() {
		c.UnregisterC <- c.ID
		c.Conn.Close()
	}()

	// Set up connection parameters
	c.Conn.SetReadLimit(util.MaxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(util.PongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(util.PongWait))
		return nil
	})

	// Main read loop
	for {
		_, rawMessage, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for client %s: %v", c.ID, err)
			}
			break
		}
		rawMessage = bytes.TrimSpace(rawMessage)

		// Skip empty messages
		if len(rawMessage) == 0 {
			continue
		}

		// Parse incoming message
		var msg message.Message
		if err := json.Unmarshal(rawMessage, &msg); err != nil {
			log.Printf("Client %s: Failed to parse message: %v", c.ID, err)
			continue
		}

		// Validate message type is not empty
		if msg.Type == "" {
			log.Printf("Client %s: Received message with empty type", c.ID)
			continue
		}

		// Set message ID if not provided
		if msg.ID == "" {
			msg.ID = c.ID
		}

		// Send message to room for processing
		select {
		case c.MessageC <- &msg:
		default:
			log.Printf("Client %s: Room message buffer full, message dropped", c.ID)
		}
	}
}

// WritePump writes messages to the WebSocket connection
func (c *Client) WritePump() {
	// Set up ticker for ping messages
	ticker := time.NewTicker(util.PingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		// Send message to client
		case message, ok := <-c.Send:
			if !ok {
				// Channel closed, send close message and return
				c.Conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(util.WriteWait))

			// Marshal message to JSON
			messageBytes, err := json.Marshal(message)
			if err != nil {
				log.Printf("Client %s: Failed to marshal message: %v", c.ID, err)
				continue
			}

			// Send the message
			if err := c.Conn.WriteMessage(websocket.TextMessage, messageBytes); err != nil {
				log.Printf("Client %s: Failed to write message: %v", c.ID, err)
				return
			}

		// Handle ping messages
		case <-ticker.C:
			// Send a ping to the client
			c.Conn.SetWriteDeadline(time.Now().Add(util.WriteWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Client %s: Failed to send ping: %v", c.ID, err)
				return
			}
		}
	}
}
