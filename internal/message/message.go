package message

type Message struct {
	Type     string                 `json:"type"`
	ID       string                 `json:"id,omitempty"`      // Sender client ID
	PipeID   string                 `json:"pipe_id,omitempty"` // Target pipe ID
	Payload  map[string]interface{} `json:"payload,omitempty"`
	ClientID string                 `json:"client_id,omitempty"` // For client-specific messages
}

// Message types
const (
	TypeJoin        = "join"
	TypeLeave       = "leave"
	TypeSubscribe   = "subscribe"
	TypeUnsubscribe = "unsubscribe"
	TypeBroadcast   = "broadcast"
	TypePipeCreate  = "pipe_create"
	TypePipeDelete  = "pipe_delete"
	TypeError       = "error"
)

// NewJoinMessage creates a join message when a client joins
func NewJoinMessage(clientID string, x, y int) *Message {
	return &Message{
		Type: TypeJoin,
		ID:   clientID,
		Payload: map[string]interface{}{
			"x": x,
			"y": y,
		},
	}
}

// NewLeaveMessage creates a leave message when a client disconnects
func NewLeaveMessage(clientID string) *Message {
	return &Message{
		Type: TypeLeave,
		ID:   clientID,
	}
}

// NewErrorMessage creates an error message
func NewErrorMessage(errMsg string) *Message {
	return &Message{
		Type: TypeError,
		Payload: map[string]interface{}{
			"message": errMsg,
		},
	}
}

// NewConfirmationMessage creates a confirmation message for pipe operations
func NewConfirmationMessage(msgType, pipeID, status string) *Message {
	return &Message{
		Type:   msgType,
		PipeID: pipeID,
		Payload: map[string]interface{}{
			"status": status,
		},
	}
}
