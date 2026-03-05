package room

import (
	"log"
	"sync"

	"github.com/scythrine05/hubtrub-server/internal/client"
)

// User represents a logical user that can have multiple connections
// (e.g., web client + game client simultaneously)
type User struct {
	ID          string                    // Unique user ID (clientId)
	Connections map[string]*client.Client // connectionType -> Client
	mu          sync.RWMutex              // Protects Connections map
}

// NewUser creates a new user
func NewUser(userID string) *User {
	return &User{
		ID:          userID,
		Connections: make(map[string]*client.Client),
	}
}

// AddConnection adds a new connection to this user
// If a connection of the same type exists, it closes the old one first
func (u *User) AddConnection(connType string, c *client.Client) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if oldConn, exists := u.Connections[connType]; exists {
		log.Printf("User %s: Closing old %s connection, new connection incoming", u.ID, connType)
		oldConn.Close()
		delete(u.Connections, connType)
	}

	// Add the new connection
	u.Connections[connType] = c
	log.Printf("User %s: Added %s connection (total: %d)", u.ID, connType, len(u.Connections))
}

// RemoveConnection removes a connection from this user
func (u *User) RemoveConnection(connType string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	if _, exists := u.Connections[connType]; exists {
		delete(u.Connections, connType)
		log.Printf("User %s: Removed %s connection (remaining: %d)", u.ID, connType, len(u.Connections))
		return true
	}
	return false
}

// HasConnections checks if user has any active connections
func (u *User) HasConnections() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return len(u.Connections) > 0
}

// BroadcastToAllConnections sends data to all active connections of this user
// Non-blocking: drops slow connections automatically
func (u *User) BroadcastToAllConnections(data []byte) {
	u.mu.RLock()
	connections := make([]*client.Client, 0, len(u.Connections))
	for _, conn := range u.Connections {
		connections = append(connections, conn)
	}
	u.mu.RUnlock()

	// Send to all connections without holding lock
	for _, conn := range connections {
		select {
		case conn.Send <- data:

		default:
			log.Printf("User %s: Connection buffer full, dropping message", u.ID)
		}
	}
}

// BroadcastToConnectionType sends data to all connections of a specific type
// Non-blocking: drops slow connections automatically
// Silently returns if the connection type doesn't exist (this is expected when not all users have all connection types)
func (u *User) BroadcastToConnectionType(connType string, data []byte) {
	u.mu.RLock()
	conn, exists := u.Connections[connType]
	u.mu.RUnlock()

	if !exists {
		// Silently return - it's normal for users to not have all connection types
		return
	}

	select {
	case conn.Send <- data:
	default:
		log.Printf("User %s: Connection buffer full for type %s, dropping message", u.ID, connType)
	}
}

// GetConnection returns a specific connection type (for internal use)
func (u *User) GetConnection(connType string) *client.Client {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.Connections[connType]
}

// GetConnectionCount returns the number of active connections
func (u *User) GetConnectionCount() int {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return len(u.Connections)
}
