package message

import (
	"github.com/google/uuid"
)

type Message struct {
	ID      string                 `json:"id,omitempty"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// Message types
const (
	PlayerJoin             = "player:join"
	PlayerLeave            = "player:leave"
	PlayerMovement         = "player:movement"
	WorldState             = "world:state"
	InterfacePanel         = "interface:panel"
	InterfaceViewer        = "interface:viewer"
	ChatPrivate            = "chat:private"
	ChatGroup              = "chat:group"
	ChatPublic             = "chat:public"
	TypeError              = "error"
	InternalIdleTimeout    = "_internal_idle_timeout"
	PlayerFollow           = "player:follow"
	PlayerUnfollow         = "player:unfollow"
	PlayerStopAllFollowers = "player:stop_all_followers"
	PlayerFollowerUpdate   = "player:follower_update"
	PlayerStopFollowing    = "player:stop_following"

	// Group lifecycle
	GroupCreate = "group:create"
	GroupJoin   = "group:join"
	GroupLeave  = "group:leave"
	GroupDelete = "group:delete"
	GroupUpdate = "group:update" // Sent to members when group state changes
)

// NewPlayerJoinMessage creates a join message when a client joins
func NewPlayerJoinMessage(clientID string, x int, y int, is_self bool) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: PlayerJoin,
		Payload: map[string]interface{}{
			"pid":     clientID,
			"x":       x,
			"y":       y,
			"is_self": is_self,
		},
	}
}

// NewPlayerLeaveMessage creates a leave message when a client disconnects
func NewPlayerLeaveMessage(clientID string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: PlayerLeave,
		Payload: map[string]interface{}{
			"pid": clientID,
		},
	}
}

// NewPlayerMovementMessage creates a movement message when a client moves
func NewPlayerMovementMessage(clientID string, x int, y int, dir string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: PlayerMovement,
		Payload: map[string]interface{}{
			"pid": clientID,
			"x":   x,
			"y":   y,
			"dir": dir,
		},
	}
}

func NewInterfacePanelMessage(panelId string, senderId string, receiverId string, subtype string, status string) *Message {
	if status == "" {
		status = "requested"
	}
	return &Message{
		ID:   uuid.New().String(),
		Type: InterfacePanel,
		Payload: map[string]interface{}{
			"panelId":    panelId,
			"senderId":   senderId,
			"receiverId": receiverId,
			"subType":    subtype,
			"status":     status,
		},
	}
}

func NewChatGroupMessage(subtype string, senderId string, groupId string, content string) *Message {
	if groupId == "" {
		groupId = "public-group-chat"
	}
	return &Message{
		ID:   uuid.New().String(),
		Type: ChatGroup,
		Payload: map[string]interface{}{
			"subType":  subtype,
			"senderId": senderId,
			"groupId":  groupId,
			"content":  content,
		},
	}
}

func NewChatPrivateMessage(senderId string, receiverId string, content string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: ChatPrivate,
		Payload: map[string]interface{}{
			"senderId":   senderId,
			"receiverId": receiverId,
			"content":    content,
		},
	}
}

// NewChatPublicMessage creates a public chat message visible to all clients in the room
func NewChatPublicMessage(subType string, senderId string, content string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: ChatPublic,
		Payload: map[string]interface{}{
			"subType":  subType,
			"senderId": senderId,
			"content":  content,
		},
	}
}

// NewErrorMessage creates an error message
func NewErrorMessage(errMsg string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: TypeError,
		Payload: map[string]interface{}{
			"message": errMsg,
		},
	}
}

// NewPlayerFollowerUpdateMessage notifies a player about their current follower list.
// Sent to the followed player's interface connection whenever the set changes.
func NewPlayerFollowerUpdateMessage(followers []string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: PlayerFollowerUpdate,
		Payload: map[string]interface{}{
			"followers": followers,
			"count":     len(followers),
		},
	}
}

// NewGroupCreateMessage notifies about a newly created group
func NewGroupCreateMessage(groupID string, groupName string, creatorID string, x float64, y float64) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: GroupCreate,
		Payload: map[string]interface{}{
			"groupId":   groupID,
			"groupName": groupName,
			"creatorId": creatorID,
			"x":         x,
			"y":         y,
		},
	}
}

// NewGroupUpdateMessage notifies members of group state changes
func NewGroupUpdateMessage(groupID string, groupName string, members []string, x float64, y float64) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: GroupUpdate,
		Payload: map[string]interface{}{
			"groupId":   groupID,
			"groupName": groupName,
			"members":   members,
			"x":         x,
			"y":         y,
			"count":     len(members),
		},
	}
}

// NewGroupJoinMessage notifies that a player joined a group
func NewGroupJoinMessage(groupID string, clientID string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: GroupJoin,
		Payload: map[string]interface{}{
			"groupId":  groupID,
			"clientId": clientID,
		},
	}
}

// NewGroupLeaveMessage notifies that a player left a group
func NewGroupLeaveMessage(groupID string, clientID string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: GroupLeave,
		Payload: map[string]interface{}{
			"groupId":  groupID,
			"clientId": clientID,
		},
	}
}

// NewGroupDeleteMessage notifies that a group was deleted (no members left)
func NewGroupDeleteMessage(groupID string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: GroupDelete,
		Payload: map[string]interface{}{
			"groupId": groupID,
		},
	}
}
