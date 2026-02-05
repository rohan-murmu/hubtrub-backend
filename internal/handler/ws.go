package handler

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/room"
	"github.com/scythrine05/hubtrub-server/internal/service"
	"github.com/scythrine05/hubtrub-server/internal/util"
)

var (
	roomsMu sync.RWMutex // Mutex to protect rooms map
	rooms   map[string]*room.Room
)

func ServeWs(rooms map[string]*room.Room, roomService *service.RoomService, w http.ResponseWriter, r *http.Request) {

	// Get room ID from query parameters
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		http.Error(w, "room_id is required", http.StatusBadRequest)
		return
	}

	// Extract token from query param or Authorization header
	token := r.URL.Query().Get("token")
	if token == "" {
		// Try to get from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
	}

	if token == "" {
		http.Error(w, "token is required", http.StatusUnauthorized)
		return
	}

	// Validate token and extract client info
	claims, err := util.ValidateToken(token)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	clientID := claims.ClientID
	clientUserName := claims.ClientUserName

	// Verify room exists in database
	_, err = roomService.GetRoomByID(roomID)
	if err != nil {
		http.Error(w, "Room not found", http.StatusNotFound)
		return
	}

	log.Printf("Client %s joining room %s", clientID, roomID)

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := client.Upgrader().Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection to WebSocket", http.StatusInternalServerError)
		return
	}

	// Get or create room in memory (thread-safe)
	roomsMu.Lock()

	currentRoom, exists := rooms[roomID]
	if !exists {
		currentRoom = room.NewRoom(roomID)
		rooms[roomID] = currentRoom
		// Start the room's event loop in a goroutine
		go currentRoom.Run()
		log.Printf("Created and started new room in memory: %s", roomID)
	}
	roomsMu.Unlock()

	log.Printf("New client %s (%s) connecting to room %s", clientID, clientUserName, roomID)

	// Create a new client using extracted clientID
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
