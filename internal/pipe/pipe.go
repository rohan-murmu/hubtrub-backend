package pipe

import (
	"log"
	"strings"

	"github.com/scythrine05/hubtrub-server/internal/message"
)

// Pipe type constants
const (
	PlayerPipe    = "player"
	InterfacePipe = "interface"
	ChatPipe      = "chat"
)

// Pipe represents a communication channel within a room
type Pipe struct {
	ID           string
	Subscribers  map[string]bool       // Client IDs subscribed to this pipe
	SubscribeC   chan string           // Channel for subscribing clients (clientID)
	UnsubscribeC chan string           // Channel for unsubscribing clients (clientID)
	BroadcastC   chan *message.Message // Channel for broadcasting messages
}

// NewPipe creates a new pipe
func NewPipe(id string) *Pipe {
	return &Pipe{
		ID:           id,
		Subscribers:  make(map[string]bool),
		SubscribeC:   make(chan string, 10),
		UnsubscribeC: make(chan string, 10),
		BroadcastC:   make(chan *message.Message, 256),
	}
}

// GetPipeTypeFromMessage extracts the pipe type from a message based on its Type field
func GetPipeTypeFromMessage(msgType string) string {
	if strings.HasPrefix(msgType, "player:") {
		return PlayerPipe
	} else if strings.HasPrefix(msgType, "interface:") {
		return InterfacePipe
	} else if strings.HasPrefix(msgType, "chat:") {
		return ChatPipe
	}
	return "" // Unknown pipe type
}

// Run starts the pipe's event loop
func (p *Pipe) Run(sendToClient func(string, *message.Message)) {
	log.Printf("Pipe %s started", p.ID)
	for {
		select {

		// Subscribe a client to the pipe
		case clientID := <-p.SubscribeC:
			p.Subscribers[clientID] = true
			log.Printf("Client %s subscribed to pipe %s", clientID, p.ID)

		// Unsubscribe a client from the pipe
		case clientID := <-p.UnsubscribeC:
			if _, ok := p.Subscribers[clientID]; ok {
				delete(p.Subscribers, clientID)
				log.Printf("Client %s unsubscribed from pipe %s", clientID, p.ID)
			}

		// Broadcast a message to all subscribers
		case msg := <-p.BroadcastC:
			for clientID := range p.Subscribers {
				sendToClient(clientID, msg)
			}
		}
	}
}
