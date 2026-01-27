package pipe

import (
	"encoding/json"
	"log"

	"github.com/scythrine05/hubtrub-server/internal/message"
)

// Pipe represents a communication channel within a room
type Pipe struct {
	ID           string
	Subscribers  map[string]bool // Client IDs subscribed to this pipe
	SubscribeC   chan string     // Channel for subscribing clients
	UnsubscribeC chan string     // Channel for unsubscribing clients
	BroadcastC   chan []byte     // Channel for broadcasting messages
	IsStatic     bool            // Static pipes cannot be deleted
}

// NewPipe creates a new pipe
func NewPipe(id string, isStatic bool) *Pipe {
	return &Pipe{
		ID:           id,
		Subscribers:  make(map[string]bool),
		SubscribeC:   make(chan string, 10),
		UnsubscribeC: make(chan string, 10),
		BroadcastC:   make(chan []byte, 256),
		IsStatic:     isStatic,
	}
}

// Run starts the pipe's event loop
func (p *Pipe) Run(sendToClient func(clientID string, data []byte)) {
	log.Printf("Pipe %s started", p.ID)
	for {
		select {
		case clientID := <-p.SubscribeC:
			p.Subscribers[clientID] = true
			log.Printf("Client %s subscribed to pipe %s", clientID, p.ID)

			// Send subscription confirmation
			confirmMsg := message.NewConfirmationMessage(message.TypeSubscribe, p.ID, "subscribed")
			if data, err := json.Marshal(confirmMsg); err == nil {
				sendToClient(clientID, data)
			}

		case clientID := <-p.UnsubscribeC:
			if _, ok := p.Subscribers[clientID]; ok {
				delete(p.Subscribers, clientID)
				log.Printf("Client %s unsubscribed from pipe %s", clientID, p.ID)

				// Send unsubscription confirmation
				confirmMsg := message.NewConfirmationMessage(message.TypeUnsubscribe, p.ID, "unsubscribed")
				if data, err := json.Marshal(confirmMsg); err == nil {
					sendToClient(clientID, data)
				}
			}

		case msg := <-p.BroadcastC:
			// Broadcast message to all subscribers
			for clientID := range p.Subscribers {
				sendToClient(clientID, msg)
			}
		}
	}
}

// Default static pipe names
const (
	DefaultPipe = "default"
	MotionPipe  = "motion"
	ChatPipe    = "chat"
)
