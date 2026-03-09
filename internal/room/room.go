package room

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/group"
	"github.com/scythrine05/hubtrub-server/internal/message"
)

// PlayerState holds the current position and state of a player.
type PlayerState struct {
	ClientID string  `json:"clientId"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Dir      string  `json:"dir"`
	IsMoving bool    `json:"is_moving"`
}

// RegistrationMessage wraps a client with its connection type for atomic registration
type RegistrationMessage struct {
	Client   *client.Client
	ConnType string // "web", "game", "default", etc.
}

// Room represents a single room with all its users and state.
// CRITICAL: Only Room.Run() modifies state. No mutexes needed.
// This is the room-authoritative game state.
type Room struct {
	ID string

	// User management (one logical user can have multiple connections)
	Users       map[string]*User    // userID -> User
	RegisterC   chan interface{}    // Client registration channel - accepts *RegistrationMessage or *client.Client (buffered, 10)
	UnregisterC chan *client.Client // Client unregistration channel (buffered, 10)
	MessageC    chan Message        // Incoming messages from clients (buffered, 256)

	// Game state
	playerStates map[string]PlayerState // userID -> PlayerState

	// Groups
	Groups map[string]*group.Group

	// Private chats (temporary 1:1 chats before they become groups)
	// Map: "userA-userB" -> map of userID pairs
	PrivateChats map[string]map[string]bool // chatID -> {userA: true, userB: true}

	// Configuration
	maxUsers int
	tickRate time.Duration
}

// Message represents a message from a client to the room.
type Message struct {
	ID      string
	Type    string
	Payload map[string]interface{}
}

// NewRoom creates a new room.
// Each room runs exactly one goroutine: room.Run()
func NewRoom(roomID string) *Room {
	return &Room{
		ID:           roomID,
		Users:        make(map[string]*User),
		RegisterC:    make(chan interface{}, 10),
		UnregisterC:  make(chan *client.Client, 10),
		MessageC:     make(chan Message, 256),
		playerStates: make(map[string]PlayerState),
		Groups:       make(map[string]*group.Group),
		PrivateChats: make(map[string]map[string]bool),
		maxUsers:     100,
		tickRate:     50 * time.Millisecond, // Movement aggregation tick
	}
}

// Run is the main event loop for the room.
func (r *Room) Run() {
	log.Printf("Room %s: started", r.ID)

	// Ticker for periodic world state broadcasts
	ticker := time.NewTicker(r.tickRate)
	defer ticker.Stop()

	for {
		select {
		// Broadcast world state on tick
		case <-ticker.C:
			r.broadcastWorldState()

		// Handle client registration
		case msg := <-r.RegisterC:
			// Only accept RegistrationMessage with "game" or "interface" types
			var wsClient *client.Client
			var connType string
			if regMsg, ok := msg.(*RegistrationMessage); ok {
				wsClient = regMsg.Client
				connType = regMsg.ConnType
			} else {
				log.Printf("Room %s: invalid or legacy registration message, ignoring", r.ID)
				continue
			}
			if connType != "game" && connType != "interface" {
				log.Printf("Room %s: invalid connection type '%s' for user %s, closing connection", r.ID, connType, wsClient.ID)
				wsClient.Close()
				continue
			}
			if len(r.Users) >= r.maxUsers {
				log.Printf("Room %s: max users reached, rejecting client %s", r.ID, wsClient.ID)
				wsClient.Close()
				continue
			}
			// Get or create user
			user, exists := r.Users[wsClient.ID]
			if !exists {
				user = NewUser(wsClient.ID)
				r.Users[wsClient.ID] = user
				// Create player state for new user
				r.playerStates[wsClient.ID] = PlayerState{
					ClientID: wsClient.ID,
					X:        rand.Float64() * 100,
					Y:        rand.Float64() * 100,
					Dir:      "down",
					IsMoving: false,
				}
				log.Printf("Room %s: new user %s registered (new logical user). Total users: %d", r.ID, wsClient.ID, len(r.Users))
			} else {
				log.Printf("Room %s: user %s already exists, adding %s connection", r.ID, wsClient.ID, connType)
			}
			user.AddConnection(connType, wsClient)
			log.Printf("Room %s: connection added successfully for user %s, type %s", r.ID, wsClient.ID, connType)
			r.broadcastJoinMessage(connType, wsClient.ID)
		// Handle client unregistration
		case c := <-r.UnregisterC:
			user, exists := r.Users[c.ID]
			if !exists {
				continue
			}

			// Find which connection type this client had (only "game" or "interface")
			var connTypeRemoved string
			if user.GameConn == c {
				connTypeRemoved = "game"
			} else if user.InterfaceConn == c {
				connTypeRemoved = "interface"
			}
			// Remove this specific connection from the user
			if connTypeRemoved != "" {
				user.RemoveConnection(connTypeRemoved)
				log.Printf("Room %s: user %s lost %s connection", r.ID, c.ID, connTypeRemoved)
			}
			// Close the connection safely (handles channel close guard)
			c.Close()
			// If user has no more connections, remove user from room
			if !user.HasConnections() {
				delete(r.Users, c.ID)
				delete(r.playerStates, c.ID)
				log.Printf("Room %s: user %s unregistered completely. Total users: %d", r.ID, c.ID, len(r.Users))
				// Only broadcast leave if we know the connection type that left
				if connTypeRemoved != "" {
					r.broadcastLeaveMessage(connTypeRemoved, c.ID)
				}
			}

			// NOTE: We do NOT stop the room when empty - rooms persist so users can rejoin
			// The room will be garbage collected eventually when no more references exist

		// Handle incoming messages from clients
		case msg := <-r.MessageC:
			r.handleMessage(&msg)
		}
	}
}

// handleMessage processes a message from a client.
func (r *Room) handleMessage(msg *Message) {
	switch msg.Type {
	case message.PlayerMovement:
		r.handleMovementMessage(msg)
		// Movement is broadcast on ticker, not immediately

	case message.ChatPublic:
		r.handlePublicChat(msg)

	case message.ChatPrivate:
		r.handlePrivateChat(msg)

	case message.ChatGroup:
		r.handleGroupChat(msg)

	case message.InterfacePanel:
		r.broadcastInterfaceMessage(msg)

	default:
		log.Printf("Room %s: unknown message type %s from %s", r.ID, msg.Type, msg.ID)
	}
}

// handleMovementMessage updates player state but does NOT broadcast immediately.
// Movement is aggregated and broadcast on ticker.
func (r *Room) handleMovementMessage(msg *Message) {
	// Extract client ID from payload
	clientID, ok := msg.Payload["pid"].(string)
	if !ok {
		log.Printf("Room %s: invalid clientId in movement message", r.ID)
		return
	}
	if state, ok := r.playerStates[clientID]; ok {
		// Update state fields
		if x, xOk := msg.Payload["x"].(float64); xOk {
			state.X = x
		}
		if y, yOk := msg.Payload["y"].(float64); yOk {
			state.Y = y
		}
		if dir, dirOk := msg.Payload["dir"].(string); dirOk {
			state.Dir = dir
		}
		if isMoving, isMovingOk := msg.Payload["is_moving"].(bool); isMovingOk {
			state.IsMoving = isMoving
		}
		// IMPORTANT: Update the map with the modified state
		r.playerStates[clientID] = state
	}
}

// broadcastWorldState aggregates all player states and broadcasts once per tick.
//
// SCALE OPTIMIZATION:
// - Builds aggregated state once per tick
// - Marshals JSON once
// - Broadcasts same byte array to all clients
// - Non-blocking sends drop slow clients automatically
// - Prevents flooding at scale (1000+ concurrent connections)
func (r *Room) broadcastWorldState() {
	if len(r.Users) == 0 {
		return
	}

	// Build world state payload with player data
	var players []PlayerState
	for _, state := range r.playerStates {
		players = append(players, state)
	}

	// Create message envelope matching Godot client expectations
	worldStateMsg := map[string]interface{}{
		"type": message.WorldState,
		"payload": map[string]interface{}{
			"players": players,
		},
	}

	data, err := json.Marshal(worldStateMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal world state: %v", r.ID, err)
		return
	}
	// Broadcast world state ONLY to game connections
	r.sendToAllUsersConnection("game", data)
}

// broadcastJoinMessage notifies all clients that a new client joined.
// For "game" connections: sends detailed player state with position
// For "interface" connections: sends simple "Player Joined" message
func (r *Room) broadcastJoinMessage(connType, clientID string) {
	// Only allow "game" or "interface" types
	if connType != "game" && connType != "interface" {
		return
	}
	state, stateExists := r.playerStates[clientID]
	if !stateExists {
		log.Printf("Room %s: player state not found for %s, skipping join message", r.ID, clientID)
		return
	}

	if connType == "game" {
		// For game connections: send detailed player join message
		// Message for self (joining client) - is_self = true
		selfMsg := message.NewPlayerJoinMessage(clientID, int(state.X), int(state.Y), true)
		selfData, err := json.Marshal(selfMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal self join message: %v", r.ID, err)
			return
		}
		r.sendToUserConnection(clientID, connType, selfData)

		// Send all existing players to the new user
		for existingClientID, existingState := range r.playerStates {
			if existingClientID == clientID {
				continue
			}
			existingPlayerMsg := message.NewPlayerJoinMessage(existingClientID, int(existingState.X), int(existingState.Y), false)
			existingPlayerData, err := json.Marshal(existingPlayerMsg)
			if err != nil {
				log.Printf("Room %s: failed to marshal existing player message: %v", r.ID, err)
				continue
			}
			r.sendToUserConnection(clientID, connType, existingPlayerData)
		}

		// Message for others (all other clients) - the new player joining (is_self = false)
		othersMsg := message.NewPlayerJoinMessage(clientID, int(state.X), int(state.Y), false)
		othersData, err := json.Marshal(othersMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal others join message: %v", r.ID, err)
			return
		}
		for otherUserID, otherUser := range r.Users {
			if otherUserID == clientID {
				continue
			}
			if otherUser.GetConnection(connType) != nil {
				r.sendToUserConnection(otherUserID, connType, othersData)
			}
		}
	} else if connType == "interface" {
		// For interface connections: send simple "Player Joined" message
		// Message for self (joining client)
		selfMsg := map[string]interface{}{
			"type":    "player:join",
			"payload": "Player Joined",
		}
		selfData, err := json.Marshal(selfMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal interface join message: %v", r.ID, err)
			return
		}
		r.sendToUserConnection(clientID, connType, selfData)

		// Notify all other users on interface connections about the new player joining
		othersMsg := map[string]interface{}{
			"type":    "player:join",
			"payload": "Player Joined",
		}
		othersData, err := json.Marshal(othersMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal interface join notification: %v", r.ID, err)
			return
		}
		for otherUserID, otherUser := range r.Users {
			if otherUserID == clientID {
				continue
			}
			if otherUser.GetConnection(connType) != nil {
				r.sendToUserConnection(otherUserID, connType, othersData)
			}
		}
	}
}

// broadcastLeaveMessage notifies all clients that a client left.
func (r *Room) broadcastLeaveMessage(connType, clientID string) {
	leaveMsg := map[string]interface{}{
		"type": message.PlayerLeave,
		"payload": map[string]interface{}{
			"pid": clientID,
		},
	}

	data, err := json.Marshal(leaveMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal leave message: %v", r.ID, err)
		return
	}

	// Only allow "game" or "interface" types
	if connType != "game" && connType != "interface" {
		return
	}
	r.sendToAllUsersConnection(connType, data)
}

// handlePublicChat handles public chat messages (visible to all clients in the room)
func (r *Room) handlePublicChat(msg *Message) {
	senderID, ok := msg.Payload["senderId"].(string)
	if !ok {
		log.Printf("Room %s: invalid senderId in public chat", r.ID)
		return
	}

	subType, _ := msg.Payload["subType"].(string)
	content, _ := msg.Payload["content"].(string)

	chatMsg := map[string]interface{}{
		"type":     message.ChatPublic,
		"subType":  subType,
		"senderId": senderID,
		"content":  content,
	}

	data, err := json.Marshal(chatMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal public chat: %v", r.ID, err)
		return
	}

	// Broadcast public chat ONLY to interface connections
	r.sendToAllUsersConnection("interface", data)
}

// handlePrivateChat handles private chat messages with subtypes:
// - "request": Sender initiates a private chat with receiver
// - "respond": Receiver accepts/rejects a private chat request
// - "join": Convert private chat to group chat by adding a third member
// - "message": Send a message in the private/group chat
// - "leave": Member leaves the private/group chat
func (r *Room) handlePrivateChat(msg *Message) {
	senderID, ok := msg.Payload["senderId"].(string)
	if !ok {
		log.Printf("Room %s: invalid senderId in private chat", r.ID)
		return
	}

	receiverID, ok := msg.Payload["recieverId"].(string) // Note: typo in original code
	if !ok {
		log.Printf("Room %s: invalid receiverId in private chat", r.ID)
		return
	}

	subType, _ := msg.Payload["subType"].(string)
	content, _ := msg.Payload["content"].(string)

	switch subType {
	case message.ChatSubtypeRequest:
		// Sender requests a private chat with receiver
		chatMsg := map[string]interface{}{
			"type":       message.ChatPrivate,
			"subType":    message.ChatSubtypeRequest,
			"senderId":   senderID,
			"receiverId": receiverID,
		}
		data, err := json.Marshal(chatMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal private chat request: %v", r.ID, err)
			return
		}
		// Send private chat ONLY to web connections
		r.sendToUserConnection(receiverID, "web", data)

	case message.ChatSubtypeRespond:
		// Receiver responds to the private chat request
		status, _ := msg.Payload["status"].(string) // accepted or rejected
		chatMsg := map[string]interface{}{
			"type":       message.ChatPrivate,
			"subType":    message.ChatSubtypeRespond,
			"senderId":   senderID,
			"receiverId": receiverID,
			"status":     status,
		}
		data, err := json.Marshal(chatMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal private chat response: %v", r.ID, err)
			return
		}
		// Send to web connections only
		r.sendToUserConnection(receiverID, "web", data)

		// If accepted, create the private chat entry
		if status == "accepted" {
			privateChatID := r.createPrivateChatID(senderID, receiverID)
			r.PrivateChats[privateChatID] = map[string]bool{
				senderID:   true,
				receiverID: true,
			}
			log.Printf("Room %s: private chat created between %s and %s (ID: %s)", r.ID, senderID, receiverID, privateChatID)
		}

	case message.ChatSubtypeMessage:
		// Send a message in the private chat
		privateChatID := r.createPrivateChatID(senderID, receiverID)
		if _, exists := r.PrivateChats[privateChatID]; !exists {
			log.Printf("Room %s: private chat %s does not exist", r.ID, privateChatID)
			return
		}

		chatMsg := map[string]interface{}{
			"type":       message.ChatPrivate,
			"subType":    message.ChatSubtypeMessage,
			"senderId":   senderID,
			"receiverId": receiverID,
			"content":    content,
		}
		data, err := json.Marshal(chatMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal private chat message: %v", r.ID, err)
			return
		}
		// Send private message ONLY to web connections
		r.sendToUserConnection(receiverID, "web", data)

	case message.ChatSubtypeJoin:
		// Convert private chat to group chat by adding a new member
		privateChatID := r.createPrivateChatID(senderID, receiverID)
		if _, exists := r.PrivateChats[privateChatID]; !exists {
			log.Printf("Room %s: private chat %s does not exist, cannot convert to group", r.ID, privateChatID)
			return
		}

		// Create a new group chat
		newMemberID, ok := msg.Payload["newMemberId"].(string)
		if !ok {
			log.Printf("Room %s: invalid newMemberId in join request", r.ID)
			return
		}

		// Convert private chat to group
		groupID := r.createGroupChatID(senderID, receiverID)
		if _, groupExists := r.Groups[groupID]; !groupExists {
			grp := group.NewGroup(groupID, "group-"+groupID[:8])
			// Add the two original members
			grp.AddMember(senderID)
			grp.AddMember(receiverID)
			r.Groups[groupID] = grp
			// Remove the private chat entry
			delete(r.PrivateChats, privateChatID)
			log.Printf("Room %s: private chat converted to group %s", r.ID, groupID)
		}

		// Add the new member to the group
		grp := r.Groups[groupID]
		grp.AddMember(newMemberID)

		// Notify all group members about the new member joining
		joinMsg := map[string]interface{}{
			"type":      message.ChatGroup,
			"subType":   message.ChatSubtypeJoin,
			"groupId":   groupID,
			"senderId":  senderID,
			"newMember": newMemberID,
		}
		data, err := json.Marshal(joinMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal group join message: %v", r.ID, err)
			return
		}
		// Send group join ONLY to web connections
		r.sendToGroupMembersConnection(groupID, "web", data)

	case message.ChatSubtypeLeave:
		// Member leaves the private/group chat
		privateChatID := r.createPrivateChatID(senderID, receiverID)
		if privateChat, exists := r.PrivateChats[privateChatID]; exists {
			// Remove from private chat
			delete(privateChat, senderID)
			if len(privateChat) == 0 {
				delete(r.PrivateChats, privateChatID)
				log.Printf("Room %s: private chat %s closed (empty)", r.ID, privateChatID)
			}
		}

		// Also check in groups and remove if applicable
		for groupID, grp := range r.Groups {
			grp.RemoveMember(senderID)
			if len(grp.Members) == 0 {
				delete(r.Groups, groupID)
				log.Printf("Room %s: group %s closed (empty)", r.ID, groupID)
			} else {
				// Notify remaining members
				leaveMsg := map[string]interface{}{
					"type":     message.ChatGroup,
					"subType":  message.ChatSubtypeLeave,
					"groupId":  groupID,
					"senderId": senderID,
				}
				data, err := json.Marshal(leaveMsg)
				if err == nil {
					// Send leave notification ONLY to web connections
					r.sendToGroupMembersConnection(groupID, "web", data)
				}
			}
		}

	default:
		log.Printf("Room %s: unknown private chat subtype %s from %s", r.ID, subType, senderID)
	}
}

// handleGroupChat handles group chat messages with subtypes:
// - "request": Sender requests to create or join a group chat
// - "respond": Receiver accepts/rejects the group chat request
// - "join": Add a new member to an existing group chat
// - "message": Send a message to the group
// - "leave": Member leaves the group
func (r *Room) handleGroupChat(msg *Message) {
	senderID, ok := msg.Payload["senderId"].(string)
	if !ok {
		log.Printf("Room %s: invalid senderId in group chat", r.ID)
		return
	}

	groupID, ok := msg.Payload["groupId"].(string)
	if !ok {
		log.Printf("Room %s: invalid groupId in group chat", r.ID)
		return
	}

	subType, _ := msg.Payload["subType"].(string)
	content, _ := msg.Payload["content"].(string)

	switch subType {
	case message.ChatSubtypeRequest:
		// Sender requests to create a group chat with receiver(s)
		receiverID, ok := msg.Payload["receiverId"].(string)
		if !ok {
			log.Printf("Room %s: invalid receiverId in group chat request", r.ID)
			return
		}

		requestMsg := map[string]interface{}{
			"type":       message.ChatGroup,
			"subType":    message.ChatSubtypeRequest,
			"groupId":    groupID,
			"senderId":   senderID,
			"receiverId": receiverID,
		}
		data, err := json.Marshal(requestMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal group chat request: %v", r.ID, err)
			return
		}
		// Send group chat request ONLY to web connections
		r.sendToUserConnection(receiverID, "web", data)

	case message.ChatSubtypeRespond:
		// Receiver responds to the group chat request
		status, _ := msg.Payload["status"].(string) // accepted or rejected

		respondMsg := map[string]interface{}{
			"type":     message.ChatGroup,
			"subType":  message.ChatSubtypeRespond,
			"groupId":  groupID,
			"senderId": senderID,
			"status":   status,
		}
		data, err := json.Marshal(respondMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal group chat response: %v", r.ID, err)
			return
		}

		// Get the original requester from payload
		requesterID, ok := msg.Payload["requesterId"].(string)
		if !ok {
			log.Printf("Room %s: invalid requesterId in group chat response", r.ID)
			return
		}

		// Send group chat response ONLY to web connections
		r.sendToUserConnection(requesterID, "web", data)

		// If accepted, create the group chat
		if status == "accepted" {
			if _, groupExists := r.Groups[groupID]; !groupExists {
				grp := group.NewGroup(groupID, "group-"+groupID[:8])
				grp.AddMember(requesterID)
				grp.AddMember(senderID)
				r.Groups[groupID] = grp
				log.Printf("Room %s: group chat created: %s with members %s, %s", r.ID, groupID, requesterID, senderID)
			}
		}

	case message.ChatSubtypeJoin:
		// Add a new member to an existing group chat
		grp, exists := r.Groups[groupID]
		if !exists {
			log.Printf("Room %s: group %s does not exist", r.ID, groupID)
			return
		}

		newMemberID, ok := msg.Payload["newMemberId"].(string)
		if !ok {
			log.Printf("Room %s: invalid newMemberId in group chat join", r.ID)
			return
		}

		// Add the new member
		grp.AddMember(newMemberID)

		// Notify all group members about the new member joining
		joinMsg := map[string]interface{}{
			"type":      message.ChatGroup,
			"subType":   message.ChatSubtypeJoin,
			"groupId":   groupID,
			"senderId":  senderID,
			"newMember": newMemberID,
		}
		data, err := json.Marshal(joinMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal group join message: %v", r.ID, err)
			return
		}
		// Send group join ONLY to web connections
		r.sendToGroupMembersConnection(groupID, "web", data)

	case message.ChatSubtypeMessage:
		// Send a message to the group
		_, exists := r.Groups[groupID]
		if !exists {
			log.Printf("Room %s: group %s does not exist", r.ID, groupID)
			return
		}

		chatMsg := map[string]interface{}{
			"type":     message.ChatGroup,
			"subType":  message.ChatSubtypeMessage,
			"groupId":  groupID,
			"senderId": senderID,
			"content":  content,
		}
		data, err := json.Marshal(chatMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal group chat message: %v", r.ID, err)
			return
		}
		// Send group message ONLY to web connections
		r.sendToGroupMembersConnection(groupID, "web", data)

	case message.ChatSubtypeLeave:
		// Member leaves the group
		grp, exists := r.Groups[groupID]
		if !exists {
			log.Printf("Room %s: group %s does not exist", r.ID, groupID)
			return
		}

		grp.RemoveMember(senderID)
		if len(grp.Members) == 0 {
			delete(r.Groups, groupID)
			log.Printf("Room %s: group %s closed (empty)", r.ID, groupID)
		} else {
			// Notify remaining members
			leaveMsg := map[string]interface{}{
				"type":     message.ChatGroup,
				"subType":  message.ChatSubtypeLeave,
				"groupId":  groupID,
				"senderId": senderID,
			}
			data, err := json.Marshal(leaveMsg)
			if err == nil {
				// Send leave notification ONLY to web connections
				r.sendToGroupMembersConnection(groupID, "web", data)
			}
		}

	default:
		log.Printf("Room %s: unknown group chat subtype %s from %s", r.ID, subType, senderID)
	}
}

// Helper function to create a unique ID for a private chat between two clients
func (r *Room) createPrivateChatID(clientA, clientB string) string {
	if clientA < clientB {
		return clientA + "-" + clientB
	}
	return clientB + "-" + clientA
}

// Helper function to create a group chat ID from the original two members
func (r *Room) createGroupChatID(clientA, clientB string) string {
	if clientA < clientB {
		return "group-" + clientA + "-" + clientB
	}
	return "group-" + clientB + "-" + clientA
}

// Helper function to send a message to all members of a group
func (r *Room) sendToGroupMembers(groupID string, data []byte) {
	grp, exists := r.Groups[groupID]
	if !exists {
		return
	}

	for _, memberID := range grp.Members {
		r.sendToUser(memberID, data)
	}
}

// Helper function to send a message to all members of a group on a specific connection type
func (r *Room) sendToGroupMembersConnection(groupID string, connType string, data []byte) {
	grp, exists := r.Groups[groupID]
	if !exists {
		return
	}

	for _, memberID := range grp.Members {
		r.sendToUserConnection(memberID, connType, data)
	}
}
func (r *Room) broadcastInterfaceMessage(msg *Message) {
	// Extract sender ID from payload
	senderID, ok := msg.Payload["senderId"].(string)
	if !ok {
		log.Printf("Room %s: invalid senderId in interface message", r.ID)
		return
	}

	interfaceMsg := map[string]interface{}{
		"type":     message.InterfacePanel,
		"senderId": senderID,
		"payload":  msg.Payload,
	}

	data, err := json.Marshal(interfaceMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal interface message: %v", r.ID, err)
		return
	}

	// Send interface messages ONLY to web connections
	r.sendToAllUsersConnection("web", data)
}

// sendToUser sends data to a specific user (to all their connections)
func (r *Room) sendToUser(userID string, data []byte) {
	if user, ok := r.Users[userID]; ok {
		user.BroadcastToAllConnections(data)
	}
}

// sendToAllUsers sends data to all users (each user gets it on all their connections)
// This is essential for scaling to 1000+ concurrent connections.
func (r *Room) sendToAllUsers(data []byte) {
	for _, user := range r.Users {
		user.BroadcastToAllConnections(data)
	}
}

// sendToAllUsersButOne sends data to all users except the specified user
// This is essential for scaling to 1000+ concurrent connections.
func (r *Room) sendToAllUsersButOne(senderID string, data []byte) {
	for userID, user := range r.Users {
		if userID == senderID {
			continue
		}
		user.BroadcastToAllConnections(data)
	}
}

// sendToUserConnection sends data to a specific user's specific connection type
func (r *Room) sendToUserConnection(userID string, connType string, data []byte) {
	// Only allow "game" or "interface" types
	if connType != "game" && connType != "interface" {
		return
	}
	if user, ok := r.Users[userID]; ok {
		user.BroadcastToConnectionType(connType, data)
	}
}

// sendToAllUsersConnection sends data to all users on a specific connection type
func (r *Room) sendToAllUsersConnection(connType string, data []byte) {
	// Only allow "game" or "interface" types
	if connType != "game" && connType != "interface" {
		return
	}
	for _, user := range r.Users {
		user.BroadcastToConnectionType(connType, data)
	}
}

// sendToAllUsersConnectionButOne sends data to all users except the specified user on a specific connection type
func (r *Room) sendToAllUsersConnectionButOne(senderID string, connType string, data []byte) {
	// Only allow "game" or "interface" types
	if connType != "game" && connType != "interface" {
		return
	}
	for userID, user := range r.Users {
		if userID == senderID {
			continue
		}
		user.BroadcastToConnectionType(connType, data)
	}
}
