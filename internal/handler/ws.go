package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/message"
	"github.com/scythrine05/hubtrub-server/internal/room"
	"github.com/scythrine05/hubtrub-server/internal/service"
)

// RoomManager manages the lifecycle of all rooms on this pod.
type RoomManager struct {
	rooms    map[string]*room.Room
	mu       sync.RWMutex
	maxRooms int
}

// NewRoomManager creates a new room manager.
func NewRoomManager(maxRooms int) *RoomManager {
	return &RoomManager{
		rooms:    make(map[string]*room.Room),
		maxRooms: maxRooms,
	}
}

// GetOrCreate gets an existing room or creates a new one.
func (rm *RoomManager) GetOrCreate(roomID string) *room.Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if r, exists := rm.rooms[roomID]; exists {
		return r
	}

	if len(rm.rooms) >= rm.maxRooms {
		log.Printf("RoomManager: max rooms reached (%d), cannot create room %s", rm.maxRooms, roomID)
		return nil
	}

	r := room.NewRoom(roomID)
	rm.rooms[roomID] = r
	go r.Run()

	log.Printf("RoomManager: created and started room %s", roomID)
	return r
}

// DeleteRoom removes a room from the manager.
func (rm *RoomManager) DeleteRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.rooms[roomID]; exists {
		delete(rm.rooms, roomID)
		log.Printf("RoomManager: deleted room %s", roomID)
	}
}

// ActiveRoomCount returns the number of active rooms.
func (rm *RoomManager) ActiveRoomCount() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return len(rm.rooms)
}

// ServeWs handles WebSocket upgrade and client connection.
// Supports multiple connections per user (e.g., web client + game client).
// Query parameters:
//   - roomId: required, the room to join
//   - clientId: required, the user's unique ID
//   - type: optional (default: "default"), connection type (e.g., "client", "game")
func ServeWs(roomManager *RoomManager, roomService *service.RoomService, clientService *service.ClientService, w http.ResponseWriter, r *http.Request) {

	// Get roomID and clientID from query parameters
	roomID := r.URL.Query().Get("roomId")
	if roomID == "" {
		http.Error(w, "roomId is required", http.StatusBadRequest)
		return
	}

	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		http.Error(w, "clientId is required", http.StatusBadRequest)
		return
	}

	// Get connection type (optional, defaults to "default")
	connType := r.URL.Query().Get("type")
	if connType == "" {
		connType = "default"
	}

	// Verify room exists in database
	_, err := roomService.GetRoomByID(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	// Verify client exists in database
	_, err = clientService.GetClientByID(clientID)
	if err != nil {
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}

	log.Printf("User %s (%s connection) joining room %s", clientID, connType, roomID)

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := client.Upgrader().Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection to WebSocket", http.StatusInternalServerError)
		return
	}

	// Get or create room
	currentRoom := roomManager.GetOrCreate(roomID)
	if currentRoom == nil {
		conn.Close()
		http.Error(w, "Cannot create room, max rooms reached", http.StatusServiceUnavailable)
		return
	}

	// Create new client connection (each connection is independent)
	wsClient := client.NewClient(clientID, conn)
	log.Printf("Created new WebSocket client for user %s with connection type %s", clientID, connType)

	// Send registration message with both client and connection type
	// Room.Run() will handle creating User (if new) and adding connection atomically
	regMsg := &room.RegistrationMessage{
		Client:   wsClient,
		ConnType: connType,
	}
	currentRoom.RegisterC <- regMsg
	log.Printf("Room %s: sent registration message for user %s with connection type %s", roomID, clientID, connType)

	// Bridge to convert interface{} messages from ReadPump to room.Message
	msgBridgeC := make(chan interface{}, 256)
	go func() {
		for msg := range msgBridgeC {
			if msgPtr, ok := msg.(*message.Message); ok {
				select {
				case currentRoom.MessageC <- room.Message{
					ID:      msgPtr.ID,
					Type:    msgPtr.Type,
					Payload: msgPtr.Payload,
				}:
				default:
					log.Printf("Handler: MessageC buffer full, dropping message")
				}
			}
		}
	}()

	// Start the client's read and write pumps in separate goroutines
	// Each connection maintains its own goroutines
	go wsClient.ReadPump(currentRoom.UnregisterC, msgBridgeC)
	go wsClient.WritePump()
}
