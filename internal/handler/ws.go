package handler

import (
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/room"
)

var (
	roomsMu sync.RWMutex // Mutex to protect rooms map
)

func ServeWs(rooms map[string]*room.Room, w http.ResponseWriter, r *http.Request) {

	// Get room ID from query parameters
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "room_id is required", http.StatusBadRequest)
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := client.Upgrader().Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection to WebSocket", http.StatusInternalServerError)
		return
	}

	// Get or create room (thread-safe)
	roomsMu.Lock()

	currentRoom, exists := rooms[roomID]
	if !exists {
		currentRoom = room.NewRoom(roomID)
		rooms[roomID] = currentRoom
		// Start the room's event loop in a goroutine
		go currentRoom.Run()
		log.Printf("Created and started new room: %s", roomID)
	}
	roomsMu.Unlock()

	// Generate unique ID for the client
	clientID := uuid.New().String()

	log.Printf("New client %s connecting to room %s", clientID, roomID)

	// Create a new client
	c := client.NewClient(
		clientID,
		conn,
		currentRoom.UnregisterC,
		currentRoom.MessageC,
	)

	// Register client with the room
	currentRoom.RegisterC <- c

	// Start the client's read and write pumps in separate goroutines
	go c.WritePump()
	go c.ReadPump()
}
