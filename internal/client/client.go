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
	ID            string          // Unique client ID
	Conn          *websocket.Conn // The WebSocket connection
	Send          chan []byte     // Channel to send messages to the client
	Subscriptions map[string]bool // Pipe IDs the client is subscribed to
	RoomID        string          // The room ID this client belongs to

	// Channels for communication with room
	UnregisterC chan string           // Notify room to unregister
	MessageC    chan *message.Message // Send messages to room for processing
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
func NewClient(id string, conn *websocket.Conn, roomID string, unregisterC chan string, messageC chan *message.Message) *Client {
	return &Client{
		ID:            id,
		Conn:          conn,
		Send:          make(chan []byte, util.SendBufferSize),
		Subscriptions: make(map[string]bool),
		RoomID:        roomID,
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
				log.Printf("error: %v", err)
			}
			break
		}
		rawMessage = bytes.TrimSpace(rawMessage)

		// Parse incoming message
		var msg message.Message
		if err := json.Unmarshal(rawMessage, &msg); err != nil {
			log.Printf("Failed to parse message: %v", err)
			continue
		}

		// Add sender ID to the message
		msg.ID = c.ID

		// Send message to room for processing
		c.MessageC <- &msg
	}
}

// WritePump writes messages to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(util.PingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				)
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(util.WriteWait))
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(util.WriteWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
