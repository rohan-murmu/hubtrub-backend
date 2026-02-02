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
	PlayerJoin      = "player:join"
	PlayerLeave     = "player:leave"
	PlayerMovement  = "player:movement"
	InterfacePanel  = "interface:panel"
	InterfaceViewer = "interface:viewer"
	ChatPrivate     = "chat:private"
	ChatGroup       = "chat:group"
	TypeError       = "error"
)

// NewPlayerJoinMessage creates a join message when a client joins
func NewPlayerJoinMessage(clientID string, x, y int, is_self bool) *Message {
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
func NewPlayerMovementMessage(clientID string, x, y, dir string) *Message {
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

func NewInterfacePanelMessage(panelId string, senderId string, recieverId string, subtype string, status string) *Message {
	if status == "" {
		status = "requested"
	}
	return &Message{
		ID:   uuid.New().String(),
		Type: InterfacePanel,
		Payload: map[string]interface{}{
			"panelId":    panelId,
			"senderId":   senderId,
			"recieverId": recieverId,
			"subType":    subtype,
			"status":     status,
		},
	}
}

func NewChatGroupMessage(subtype string, senderId string, groupId string, content string) *Message {
	if groupId == "" {
		groupId = "main"
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

func NewChatPrivateMessage(subType string, senderId string, recieverId string, content string) *Message {
	return &Message{
		ID:   uuid.New().String(),
		Type: ChatPrivate,
		Payload: map[string]interface{}{
			"subType":    subType,
			"senderId":   senderId,
			"recieverId": recieverId,
			"content":    content,
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
