package room

import (
	"log"

	"github.com/scythrine05/hubtrub-server/internal/client"
)

// User represents a logical user that can have multiple connections
// (e.g., web client + game client simultaneously)
type User struct {
	ID            string         // Unique user ID (clientId)
	GameConn      *client.Client // "game" connection
	InterfaceConn *client.Client // "interface" connection
}

// NewUser creates a new user
func NewUser(userID string) *User {
	return &User{
		ID: userID,
	}
}

// AddConnection adds a new connection to this user (only "game" or "interface" types allowed)
func (u *User) AddConnection(connType string, c *client.Client) {
	switch connType {
	case "game":
		if u.GameConn != nil {
			log.Printf("User %s: Closing old game connection, new connection incoming", u.ID)
			u.GameConn.Close()
		}
		u.GameConn = c
		log.Printf("User %s: Added game connection", u.ID)
	case "interface":
		if u.InterfaceConn != nil {
			log.Printf("User %s: Closing old interface connection, new connection incoming", u.ID)
			u.InterfaceConn.Close()
		}
		u.InterfaceConn = c
		log.Printf("User %s: Added interface connection", u.ID)
	default:
		log.Printf("User %s: Unknown connection type '%s', ignoring", u.ID, connType)
	}
}

// RemoveConnection removes a connection from this user
func (u *User) RemoveConnection(connType string) bool {
	switch connType {
	case "game":
		if u.GameConn != nil {
			u.GameConn.Close()
			u.GameConn = nil
			log.Printf("User %s: Removed game connection", u.ID)
			return true
		}
	case "interface":
		if u.InterfaceConn != nil {
			u.InterfaceConn.Close()
			u.InterfaceConn = nil
			log.Printf("User %s: Removed interface connection", u.ID)
			return true
		}
	}
	return false
}

// HasConnections checks if user has any active connections
func (u *User) HasConnections() bool {
	return u.GameConn != nil || u.InterfaceConn != nil
}

// BroadcastToAllConnections sends data to all active connections of this user
// Non-blocking: drops slow connections automatically
func (u *User) BroadcastToAllConnections(data []byte) {
	if u.GameConn != nil {
		select {
		case u.GameConn.Send <- data:
		default:
			log.Printf("User %s: GameConn buffer full, dropping message", u.ID)
		}
	}
	if u.InterfaceConn != nil {
		select {
		case u.InterfaceConn.Send <- data:
		default:
			log.Printf("User %s: InterfaceConn buffer full, dropping message", u.ID)
		}
	}
}

// BroadcastToConnectionType sends data to a specific connection type ("game" or "interface")
// Non-blocking: drops slow connections automatically
func (u *User) BroadcastToConnectionType(connType string, data []byte) {
	switch connType {
	case "game":
		if u.GameConn != nil {
			select {
			case u.GameConn.Send <- data:
			default:
				log.Printf("User %s: GameConn buffer full, dropping message", u.ID)
			}
		}
	case "interface":
		if u.InterfaceConn != nil {
			select {
			case u.InterfaceConn.Send <- data:
			default:
				log.Printf("User %s: InterfaceConn buffer full, dropping message", u.ID)
			}
		}
	default:
		// Ignore unknown types
	}
}

// GetConnection returns a specific connection type (for internal use)
func (u *User) GetConnection(connType string) *client.Client {
	switch connType {
	case "game":
		return u.GameConn
	case "interface":
		return u.InterfaceConn
	default:
		return nil
	}
}

// GetConnectionCount returns the number of active connections
func (u *User) GetConnectionCount() int {
	count := 0
	if u.GameConn != nil {
		count++
	}
	if u.InterfaceConn != nil {
		count++
	}
	return count
}
