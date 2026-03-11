package client

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/scythrine05/hubtrub-server/internal/message"
	"github.com/scythrine05/hubtrub-server/internal/util"

	"github.com/gorilla/websocket"
)

// Client represents a WebSocket client connected to a room
type Client struct {
	ID       string          // Unique client ID
	Conn     *websocket.Conn // The WebSocket connection
	Send     chan []byte     // Buffered channel for sending raw JSON messages
	closedMu sync.Mutex      // Protects closed flag
	closed   bool            // Flag to prevent double-close of Send channel
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

// NewClient creates a new client.
func NewClient(id string, conn *websocket.Conn) *Client {
	return &Client{
		ID:     id,
		Conn:   conn,
		Send:   make(chan []byte, 256), // Buffered channel for raw []byte messages
		closed: false,
	}
}

// Close safely closes the client's send channel
func (c *Client) Close() {
	c.closedMu.Lock()
	defer c.closedMu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.Send)
		c.Conn.Close()
	}
}

// ReadPump reads messages from the client's WebSocket connection.
func (c *Client) ReadPump(unregisterC chan *Client, messageC chan interface{}) {
	defer func() {
		log.Printf("Client %s: ReadPump exiting, sending to unregisterC", c.ID)
		unregisterC <- c
		c.Conn.Close()
	}()

	log.Printf("Client %s: ReadPump started", c.ID)

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
			log.Printf("Client %s: ReadMessage returned error: %v, exiting ReadPump", c.ID, err)
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
		case messageC <- &msg:
		default:
			log.Printf("Client %s: Room message buffer full, message dropped", c.ID)
		}
	}
}

// WritePump writes messages from the Send channel to the WebSocket connection.
func (c *Client) WritePump() {
	// Set up ticker for ping messages
	ticker := time.NewTicker(util.PingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		// Send message to client
		case data, ok := <-c.Send:
			if !ok {
				// Channel closed, send close message and return
				c.Conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(util.WriteWait))

			// Send the raw []byte message directly
			if err := c.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
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
