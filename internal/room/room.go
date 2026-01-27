package room

import (
	"encoding/json"
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
	Pipes       map[string]*pipe.Pipe     // Pipes within the room (pipeID -> Pipe)
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

	// Create static pipes
	r.createStaticPipes()

	return r
}

// createStaticPipes creates default pipes that cannot be deleted
func (r *Room) createStaticPipes() {
	staticPipes := []string{pipe.DefaultPipe, pipe.MotionPipe, pipe.ChatPipe}

	for _, pipeID := range staticPipes {
		p := pipe.NewPipe(pipeID, true)
		r.Pipes[pipeID] = p
		// Start pipe in a separate goroutine
		go p.Run(r.sendToClient)
	}

	log.Printf("Room %s: Created static pipes: %v", r.ID, staticPipes)
}

// sendToClient is a helper function to send messages to a specific client
func (r *Room) sendToClient(clientID string, data []byte) {
	if client, ok := r.Clients[clientID]; ok {
		select {
		case client.Send <- data:
		default:
			log.Printf("Client %s send buffer full, message dropped", clientID)
		}
	}
}

// broadcastToAll sends a message to all clients in the room
func (r *Room) broadcastToAll(data []byte, excludeClientID string) {
	for id, c := range r.Clients {
		if id != excludeClientID {
			select {
			case c.Send <- data:
			default:
				log.Printf("Client %s send buffer full, message dropped", id)
			}
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

			// Auto-subscribe to default pipe
			if defaultPipe, ok := r.Pipes[pipe.DefaultPipe]; ok {
				defaultPipe.SubscribeC <- client.ID
				client.Subscriptions[pipe.DefaultPipe] = true
			}

			// Broadcast join message to all other clients via default pipe
			joinMsg := message.NewJoinMessage(client.ID, x, y)
			if data, err := json.Marshal(joinMsg); err == nil {
				client.Send <- data
				r.broadcastToAll(data, client.ID)
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

				// Broadcast leave message to all remaining clients
				leaveMsg := message.NewLeaveMessage(clientID)
				if data, err := json.Marshal(leaveMsg); err == nil {
					r.broadcastToAll(data, clientID)
				}

				// Remove client and close send channel
				delete(r.Clients, clientID)
				close(client.Send)
			}

		// Handle messages from clients
		case msg := <-r.MessageC:
			r.handleMessage(msg)
		}
	}
}

// handleMessage processes different types of messages
func (r *Room) handleMessage(msg *message.Message) {
	switch msg.Type {
	case message.TypeSubscribe:
		r.handleSubscribe(msg)
	case message.TypeUnsubscribe:
		r.handleUnsubscribe(msg)
	case message.TypePipeCreate:
		r.handlePipeCreate(msg)
	case message.TypePipeDelete:
		r.handlePipeDelete(msg)
	case message.TypeBroadcast:
		r.handleBroadcast(msg)
	default:
		// Default behavior: broadcast to default pipe
		if msg.PipeID == "" {
			msg.PipeID = pipe.DefaultPipe
		}
		r.handleBroadcast(msg)
	}
}

// handleSubscribe subscribes a client to a pipe
func (r *Room) handleSubscribe(msg *message.Message) {
	pipeID := msg.PipeID
	clientID := msg.ID

	if pipeID == "" {
		r.sendError(clientID, "pipe_id is required for subscription")
		return
	}

	p, exists := r.Pipes[pipeID]
	if !exists {
		r.sendError(clientID, "pipe not found: "+pipeID)
		return
	}

	if client, ok := r.Clients[clientID]; ok {
		client.Subscriptions[pipeID] = true
		p.SubscribeC <- clientID
	}
}

// handleUnsubscribe unsubscribes a client from a pipe
func (r *Room) handleUnsubscribe(msg *message.Message) {
	pipeID := msg.PipeID
	clientID := msg.ID

	if pipeID == "" {
		r.sendError(clientID, "pipe_id is required for unsubscription")
		return
	}

	p, exists := r.Pipes[pipeID]
	if !exists {
		r.sendError(clientID, "pipe not found: "+pipeID)
		return
	}

	if client, ok := r.Clients[clientID]; ok {
		delete(client.Subscriptions, pipeID)
		p.UnsubscribeC <- clientID
	}
}

// handlePipeCreate creates a new dynamic pipe
func (r *Room) handlePipeCreate(msg *message.Message) {
	pipeID := msg.PipeID
	clientID := msg.ID

	if pipeID == "" {
		r.sendError(clientID, "pipe_id is required for pipe creation")
		return
	}

	if _, exists := r.Pipes[pipeID]; exists {
		r.sendError(clientID, "pipe already exists: "+pipeID)
		return
	}

	// Create new dynamic pipe
	p := pipe.NewPipe(pipeID, false)
	r.Pipes[pipeID] = p
	go p.Run(r.sendToClient)

	log.Printf("Room %s: Dynamic pipe %s created by client %s", r.ID, pipeID, clientID)

	// Send confirmation to creator
	confirmMsg := message.NewConfirmationMessage(message.TypePipeCreate, pipeID, "created")
	if data, err := json.Marshal(confirmMsg); err == nil {
		r.sendToClient(clientID, data)
	}
}

// handlePipeDelete deletes a dynamic pipe
func (r *Room) handlePipeDelete(msg *message.Message) {
	pipeID := msg.PipeID
	clientID := msg.ID

	if pipeID == "" {
		r.sendError(clientID, "pipe_id is required for pipe deletion")
		return
	}

	p, exists := r.Pipes[pipeID]
	if !exists {
		r.sendError(clientID, "pipe not found: "+pipeID)
		return
	}

	if p.IsStatic {
		r.sendError(clientID, "cannot delete static pipe: "+pipeID)
		return
	}

	// Unsubscribe all clients from this pipe
	for cID, client := range r.Clients {
		if _, subscribed := client.Subscriptions[pipeID]; subscribed {
			delete(client.Subscriptions, pipeID)
			// Notify client about pipe deletion
			notifyMsg := message.NewConfirmationMessage(message.TypePipeDelete, pipeID, "deleted")
			if data, err := json.Marshal(notifyMsg); err == nil {
				r.sendToClient(cID, data)
			}
		}
	}

	delete(r.Pipes, pipeID)
	log.Printf("Room %s: Dynamic pipe %s deleted by client %s", r.ID, pipeID, clientID)
}

// handleBroadcast broadcasts a message to a specific pipe
func (r *Room) handleBroadcast(msg *message.Message) {
	pipeID := msg.PipeID
	if pipeID == "" {
		pipeID = pipe.DefaultPipe
	}

	p, exists := r.Pipes[pipeID]
	if !exists {
		r.sendError(msg.ID, "pipe not found: "+pipeID)
		return
	}

	// Marshal message and send to pipe
	if data, err := json.Marshal(msg); err == nil {
		select {
		case p.BroadcastC <- data:
		default:
			log.Printf("Pipe %s broadcast buffer full, message dropped", pipeID)
		}
	}
}

// sendError sends an error message to a client
func (r *Room) sendError(clientID string, errMsg string) {
	errorMsg := message.NewErrorMessage(errMsg)
	if data, err := json.Marshal(errorMsg); err == nil {
		r.sendToClient(clientID, data)
	}
}
