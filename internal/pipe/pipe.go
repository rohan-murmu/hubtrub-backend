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
	// Helper functions for delivery
	sendToAll := func(msg *message.Message) {
		for clientID := range p.Subscribers {
			sendToClient(clientID, msg)
		}
	}
	sendToAllExcept := func(msg *message.Message, exceptID string) {
		for clientID := range p.Subscribers {
			if clientID != exceptID {
				sendToClient(clientID, msg)
			}
		}
	}
	sendToSelf := func(msg *message.Message, selfID string) {
		if _, ok := p.Subscribers[selfID]; ok {
			sendToClient(selfID, msg)
		}
	}

	log.Printf("Pipe %s started", p.ID)
	for {
		select {
		case clientID := <-p.SubscribeC:
			p.Subscribers[clientID] = true
			log.Printf("Client %s subscribed to pipe %s", clientID, p.ID)

		case clientID := <-p.UnsubscribeC:
			if _, ok := p.Subscribers[clientID]; ok {
				delete(p.Subscribers, clientID)
				log.Printf("Client %s unsubscribed from pipe %s", clientID, p.ID)
			}

		case msg := <-p.BroadcastC:
			switch p.ID {
			case PlayerPipe:
				switch msg.Type {
				case message.PlayerJoin:
					pid, _ := msg.Payload["pid"].(string)
					isSelf, _ := msg.Payload["is_self"].(bool)
					if isSelf {
						sendToSelf(msg, pid)
					} else {
						sendToAllExcept(msg, pid)
					}
				case message.PlayerMovement, message.PlayerLeave:
					pid, _ := msg.Payload["pid"].(string)
					sendToAllExcept(msg, pid)
				default:
					sendToAll(msg)
				}
			case InterfacePipe, ChatPipe:
				sendToAll(msg)
			default:
				sendToAll(msg)
			}
		}
	}
}
