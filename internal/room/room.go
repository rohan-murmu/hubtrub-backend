package room

import (
	"log"
	"math/rand"
	"sync"

	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/message"
	"github.com/scythrine05/hubtrub-server/internal/pipe"
)

type Room struct {
	ID          string                    // Unique room ID
	Clients     map[string]*client.Client // Registered clients (clientID -> Client)
	Pipes       map[string]*pipe.Pipe     // Pipes in the room (pipeID -> Pipe)
	RegisterC   chan *client.Client       // Channel for registering new clients
	UnregisterC chan string               // Channel for unregistering clients (clientID)
	MessageC    chan *message.Message     // Channel for processing messages
	isRunning   bool                      // Flag to check if room is already running
	mu          sync.Mutex                // Mutex to protect isRunning flag
}

// NewRoom creates a new room with static pipes
func NewRoom(roomID string) *Room {
	r := &Room{
		ID:          roomID,                           // Assign provided room ID
		Clients:     make(map[string]*client.Client),  // Initialize clients map
		Pipes:       make(map[string]*pipe.Pipe),      // Initialize pipes map
		RegisterC:   make(chan *client.Client, 10),    // Buffered channel for client registration
		UnregisterC: make(chan string, 10),            // Buffered channel for client unregistration
		MessageC:    make(chan *message.Message, 256), // Buffered channel for messages
		isRunning:   false,                            // Initially not running
	}

	r.createPipes()

	return r
}

// createPipes creates the three static pipes: PlayerPipe, InterfacePipe, and ChatPipe
func (r *Room) createPipes() {
	// Create the three static pipes
	pipes := []string{pipe.PlayerPipe, pipe.InterfacePipe, pipe.ChatPipe}

	for _, pipeType := range pipes {
		p := pipe.NewPipe(pipeType)
		r.Pipes[pipeType] = p
		// Start pipe in a separate goroutine
		go p.Run(r.sendToClient)
	}

	log.Printf("Room %s: Created static pipes: %v", r.ID, pipes)
}

// sendToClient sends a message to a specific client
func (r *Room) sendToClient(clientID string, data *message.Message) {
	if client, ok := r.Clients[clientID]; ok {
		select {
		case client.Send <- data:
		default:
			log.Printf("Client %s send buffer full, message dropped", clientID)
		}
	}
}

// Run starts the room's event loop
func (r *Room) Run() {
	r.mu.Lock()
	if r.isRunning {
		r.mu.Unlock()
		return
	}
	r.isRunning = true
	r.mu.Unlock()

	log.Printf("Room %s started", r.ID)

	for {
		select {
		// Handle client registration
		case client := <-r.RegisterC:
			r.Clients[client.ID] = client
			log.Printf("Room %s: Client %s registered", r.ID, client.ID)

			// Generate random position for the client
			x := rand.Intn(500)
			y := rand.Intn(500)

			// Auto-subscribe to all three static pipes
			for _, pipeType := range []string{pipe.PlayerPipe, pipe.InterfacePipe, pipe.ChatPipe} {
				if p, ok := r.Pipes[pipeType]; ok {
					p.SubscribeC <- client.ID
					client.Subscriptions[pipeType] = true
				}
			}

			// Send join message: to self (is_self true), to others (is_self false) via player pipe
			if p, ok := r.Pipes[pipe.PlayerPipe]; ok {
				joinMsgSelf := message.NewPlayerJoinMessage(client.ID, x, y, true)
				p.BroadcastC <- joinMsgSelf

				joinMsgOthers := message.NewPlayerJoinMessage(client.ID, x, y, false)
				p.BroadcastC <- joinMsgOthers
			}

		// Handle client unregistration
		case clientID := <-r.UnregisterC:
			if client, ok := r.Clients[clientID]; ok {
				log.Printf("Room %s: Client %s unregistering", r.ID, clientID)

				// Unsubscribe from all pipes
				for pipeID := range client.Subscriptions {
					if p, exists := r.Pipes[pipeID]; exists {
						p.UnsubscribeC <- clientID
					}
				}

				// Broadcast leave message to all remaining clients via player pipe
				if p, ok := r.Pipes[pipe.PlayerPipe]; ok {
					leaveMsg := message.NewPlayerLeaveMessage(clientID)
					p.BroadcastC <- leaveMsg
				}

				// Remove client and close send channel
				delete(r.Clients, clientID)
				close(client.Send)
				log.Printf("Room %s: Client %s unregistered", r.ID, clientID)
			}

		// Handle messages from clients
		case msg := <-r.MessageC:
			log.Printf("Room %s, Pipe %s: Received message of type %s", r.ID, pipe.GetPipeTypeFromMessage(msg.Type), msg.Type)
			r.handleMessage(msg)
		}
	}
}

// handleMessage routes messages to the correct pipe for broadcasting
func (r *Room) handleMessage(msg *message.Message) {
	if msg.Type == "" {
		log.Printf("Invalid message: missing type")
		return
	}

	pipeType := pipe.GetPipeTypeFromMessage(msg.Type)
	p, ok := r.Pipes[pipeType]
	if !ok {
		log.Printf("Pipe %s not found for message type %s", pipeType, msg.Type)
		return
	}

	p.BroadcastC <- msg
}

// sendError sends an error message to a client
func (r *Room) sendError(clientID string, errMsg string) {
	errorMsg := message.NewErrorMessage(errMsg)
	r.sendToClient(clientID, errorMsg)
}
