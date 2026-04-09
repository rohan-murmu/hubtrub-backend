package user

import (
	"log"

	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/util"
)

// User represents a logical user that can have multiple connections
// (e.g., web client + game client simultaneously)
type User struct {
	ID       string         // Unique user ID (clientId)
	GameConn *client.Client // "game" connection
	UIConn   *client.Client // "ui" connection
}

// NewUser creates a new user
func NewUser(userID string) *User {
	return &User{
		ID: userID,
	}
}

// AddConnection adds a new connection to this user (only "game" or "ui" types allowed)
func (u *User) AddConnection(connType string, c *client.Client) {
	switch connType {
	case util.ConnGame:
		if u.GameConn != nil {
			log.Printf("User %s: Closing old game connection, new connection incoming", u.ID)
			u.GameConn.Close()
		}
		u.GameConn = c
		log.Printf("User %s: Added game connection", u.ID)
	case util.ConnUI:
		if u.UIConn != nil {
			log.Printf("User %s: Closing old ui connection, new connection incoming", u.ID)
			u.UIConn.Close()
		}
		u.UIConn = c
		log.Printf("User %s: Added ui connection", u.ID)
	default:
		log.Printf("User %s: Unknown connection type '%s', ignoring", u.ID, connType)
	}
}

// RemoveConnection removes a connection from this user
func (u *User) RemoveConnection(connType string) bool {
	switch connType {
	case util.ConnGame:
		if u.GameConn != nil {
			u.GameConn.Close()
			u.GameConn = nil
			log.Printf("User %s: Removed game connection", u.ID)
			return true
		}
	case util.ConnUI:
		if u.UIConn != nil {
			u.UIConn.Close()
			u.UIConn = nil
			log.Printf("User %s: Removed ui connection", u.ID)
			return true
		}
	}
	return false
}

// HasConnections checks if user has any active connections
func (u *User) HasConnections() bool {
	return u.GameConn != nil || u.UIConn != nil
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
	if u.UIConn != nil {
		select {
		case u.UIConn.Send <- data:
		default:
			log.Printf("User %s: UIConn buffer full, dropping message", u.ID)
		}
	}
}

// BroadcastToConnectionType sends data to a specific connection type ("game" or "ui")
// Non-blocking: drops slow connections automatically
func (u *User) BroadcastToConnectionType(connType string, data []byte) {
	switch connType {
	case util.ConnGame:
		if u.GameConn != nil {
			select {
			case u.GameConn.Send <- data:
			default:
				log.Printf("User %s: GameConn buffer full, dropping message", u.ID)
			}
		}
	case util.ConnUI:
		if u.UIConn != nil {
			select {
			case u.UIConn.Send <- data:
			default:
				log.Printf("User %s: UIConn buffer full, dropping message", u.ID)
			}
		}
	default:
		// Ignore unknown types
	}
}

// GetConnection returns a specific connection type (for internal use)
func (u *User) GetConnection(connType string) *client.Client {
	switch connType {
	case util.ConnGame:
		return u.GameConn
	case util.ConnUI:
		return u.UIConn
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
	if u.UIConn != nil {
		count++
	}
	return count
}
