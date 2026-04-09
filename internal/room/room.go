package room

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/scythrine05/hubtrub-server/internal/client"
	"github.com/scythrine05/hubtrub-server/internal/group"
	"github.com/scythrine05/hubtrub-server/internal/message"
	"github.com/scythrine05/hubtrub-server/internal/user"
	"github.com/scythrine05/hubtrub-server/internal/util"
)

// PlayerState holds the current position and state of a player.
type PlayerState struct {
	ClientID string  `json:"clientId"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Dir      string  `json:"dir"`
	IsMoving bool    `json:"is_moving"`
}

// RegistrationMessage wraps a client with its connection type
type RegistrationMessage struct {
	Client   *client.Client
	ConnType string
}

// Room represents a game room with multiple users, player states, and chat groups.
type Room struct {
	ID string

	Users       map[string]*user.User // User
	RegisterC   chan interface{}      // Client registration channel - accepts *RegistrationMessage
	UnregisterC chan *client.Client   // Client unregistration channel
	MessageC    chan Message          // Incoming messages from clients

	playerStates map[string]PlayerState // Player states keyed by client ID

	Groups map[string]*group.Group // Chat groups keyed by group ID

	PrivateChats map[string]map[string]bool // Private chat memberships keyed by sorted "clientA:clientB" string

	// Followers tracks who is following whom.
	// Key: followedID → value: set of followerIDs
	Followers map[string]map[string]bool

	// Idle timer for room deletion
	idleTimer    *time.Timer   // Timer that fires when room is empty for too long
	idleDuration time.Duration // How long to wait before deleting empty room
	onEmpty      func()        // Callback when room should be deleted (called by RoomManager)

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
func NewRoom(roomID string) *Room {
	return &Room{
		ID:           roomID,
		Users:        make(map[string]*user.User),
		RegisterC:    make(chan interface{}, 10),
		UnregisterC:  make(chan *client.Client, 10),
		MessageC:     make(chan Message, 256),
		playerStates: make(map[string]PlayerState),
		Groups:       make(map[string]*group.Group),
		PrivateChats: make(map[string]map[string]bool),
		Followers:    make(map[string]map[string]bool),
		idleTimer:    nil,
		idleDuration: 60 * time.Second, // Default: delete empty room after 60 seconds
		onEmpty:      nil,              // Will be set by RoomManager
		maxUsers:     100,
		tickRate:     2 * time.Millisecond, // Movement aggregation tick
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

		// Handle idle timer expiration (room delete)
		case <-r.getIdleTimerChan():
			if len(r.Users) == 0 {
				log.Printf("Room %s: idle timeout reached, room will be deleted", r.ID)
				if r.onEmpty != nil {
					r.onEmpty()
				}
				return
			}

		// Handle client registration
		case msg := <-r.RegisterC:
			// Only accept RegistrationMessage with "game" or "ui" types
			var wsClient *client.Client
			var connType string
			if regMsg, ok := msg.(*RegistrationMessage); ok {
				wsClient = regMsg.Client
				connType = regMsg.ConnType
			} else {
				log.Printf("Room %s: invalid or legacy registration message, ignoring", r.ID)
				continue
			}
			if len(r.Users) >= r.maxUsers {
				log.Printf("Room %s: max users reached, rejecting client %s", r.ID, wsClient.ID)
				wsClient.Close()
				continue
			}

			// If room was empty and idle timer was running, cancel it (user rejoined)
			wasEmpty := len(r.Users) == 0
			if wasEmpty && r.idleTimer != nil {
				r.idleTimer.Stop()
				r.idleTimer = nil
				log.Printf("Room %s: user rejoined, idle timer cancelled", r.ID)
			}

			// Get or create user
			u, exists := r.Users[wsClient.ID]
			if !exists {
				u = user.NewUser(wsClient.ID)
				r.Users[wsClient.ID] = u
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
			u.AddConnection(connType, wsClient)
			log.Printf("Room %s: connection added successfully for user %s, type %s", r.ID, wsClient.ID, connType)
			r.broadcastJoinMessage(connType, wsClient.ID)
		// Handle client unregistration
		case c := <-r.UnregisterC:
			u, exists := r.Users[c.ID]
			if !exists {
				continue
			}

			// Find which connection type this client had (only "game" or "ui")
			var connTypeRemoved string
			if u.GameConn == c {
				connTypeRemoved = util.ConnGame
			} else if u.UIConn == c {
				connTypeRemoved = util.ConnUI
			}
			// Remove this specific connection from the user
			if connTypeRemoved != "" {
				u.RemoveConnection(connTypeRemoved)
				log.Printf("Room %s: user %s lost %s connection", r.ID, c.ID, connTypeRemoved)
			}
			// Close the connection safely (handles channel close guard)
			c.Close()
			// If user has no more connections, remove user from room
			if !u.HasConnections() {
				delete(r.Users, c.ID)
				delete(r.playerStates, c.ID)
				log.Printf("Room %s: user %s unregistered completely. Total users: %d", r.ID, c.ID, len(r.Users))
				// Only broadcast leave if we know the connection type that left
				if connTypeRemoved != "" {
					r.broadcastLeaveMessage(connTypeRemoved, c.ID)
				}
				// Clean up group memberships for the departing user
				r.cleanupPlayerGroups(c.ID)
				// Clean up follow relationships for the departing user:
				// 1. Remove them as a follower from everyone they were following
				for followedID, followers := range r.Followers {
					if followers[c.ID] {
						delete(followers, c.ID)
						if len(followers) == 0 {
							delete(r.Followers, followedID)
						}
						// Notify the followed player (still in room) their list changed
						if _, stillIn := r.Users[followedID]; stillIn {
							r.notifyFollowerUpdate(followedID)
						}
					}
				}
				// 2. Clear the departing user's own follower list
				delete(r.Followers, c.ID)
			}

			// If room is now empty, start idle timer for deletion
			if len(r.Users) == 0 && r.idleTimer == nil {
				r.idleTimer = time.NewTimer(r.idleDuration)
				log.Printf("Room %s: idle timer started, will delete room after %v if no one rejoins", r.ID, r.idleDuration)
			}

		// Handle incoming messages from clients
		case msg := <-r.MessageC:
			r.handleMessage(&msg)
		}
	}
}

// handleMessage processes a message from a client.
func (r *Room) handleMessage(msg *Message) {
	switch msg.Type {
	case message.InternalIdleTimeout:
		log.Printf("Room %s: WARNING - idle timeout message reached handleMessage, this shouldn't happen", r.ID)

	case message.PlayerMovement:
		r.handleMovementMessage(msg)

	case message.ChatPublic:
		r.handlePublicChat(msg)

	case message.ChatPrivate:
		r.handlePrivateChat(msg)

	case message.ChatGroup:
		r.handleGroupChat(msg)

	case message.InterfacePanel:
		r.broadcastInterfaceMessage(msg)

	case message.PlayerFollow:
		r.handleFollow(msg)

	case message.PlayerUnfollow:
		r.handleUnfollow(msg)

	case message.PlayerStopAllFollowers:
		r.handleStopAllFollowers(msg)

	case message.PlayerLeave:
		r.handlePlayerLeave(msg)

	case message.GroupCreate:
		r.handleGroupCreate(msg)

	case message.GroupJoin:
		r.handleGroupJoin(msg)

	case message.GroupLeave:
		r.handleGroupLeave(msg)

	default:
		log.Printf("Room %s: unknown message type %s from %s", r.ID, msg.Type, msg.ID)
	}
}

// handleMovementMessage updates player state but does NOT broadcast immediately.
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
		r.playerStates[clientID] = state
	}
}

// BroadcastWorldState aggregates all player states and broadcasts once per tick.
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
	r.sendToAllUsersConnection(util.ConnGame, data)
}

// broadcastJoinMessage notifies all clients that a new client joined.
func (r *Room) broadcastJoinMessage(connType, clientID string) {
	// Only allow "game" or "ui" types
	if connType != util.ConnGame && connType != util.ConnUI {
		return
	}
	state, stateExists := r.playerStates[clientID]
	if !stateExists {
		log.Printf("Room %s: player state not found for %s, skipping join message", r.ID, clientID)
		return
	}

	if connType == util.ConnGame {
		// For game connections: send detailed player join message
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

		// Send all existing groups to the newly joined player so they can see group dialogs
		for _, grp := range r.Groups {
			groupUpdateMsg := message.NewGroupUpdateMessage(grp.ID, grp.Name, grp.Members, grp.X, grp.Y)
			groupData, gErr := json.Marshal(groupUpdateMsg)
			if gErr != nil {
				continue
			}
			r.sendToUserConnection(clientID, connType, groupData)
		}
	} else if connType == util.ConnUI {
		// For ui connections: send simple "Player Joined" message
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

	// Only allow "game" or "ui" types
	if connType != util.ConnGame && connType != util.ConnUI {
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

	// Broadcast public chat ONLY to ui connections
	r.sendToAllUsersConnection(util.ConnUI, data)
}

// handlePrivateChat handles private chat messages - simply sends messages between two clients
func (r *Room) handlePrivateChat(msg *Message) {
	// Extract senderId and receiverId
	senderID, ok := msg.Payload["senderId"].(string)
	if !ok {
		log.Printf("Room %s: invalid senderId in private chat", r.ID)
		return
	}

	receiverID, ok := msg.Payload["receiverId"].(string)
	if !ok {
		log.Printf("Room %s: invalid receiverId in private chat", r.ID)
		return
	}

	content, _ := msg.Payload["content"].(string)

	// Create or verify private chat exists
	privateChatID := r.createPrivateChatID(senderID, receiverID)
	if _, exists := r.PrivateChats[privateChatID]; !exists {
		// Auto-create private chat on first message
		r.PrivateChats[privateChatID] = map[string]bool{
			senderID:   true,
			receiverID: true,
		}
		log.Printf("Room %s: private chat created between %s and %s on first message", r.ID, senderID, receiverID)
	}

	// Send message to receiver
	chatMsg := message.NewChatPrivateMessage(senderID, receiverID, content)
	data, err := json.Marshal(chatMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal private chat message: %v", r.ID, err)
		return
	}

	// Send private message ONLY to ui connections
	r.sendToUserConnection(receiverID, util.ConnUI, data)
}

// handleGroupChat handles group chat messages
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

	content, _ := msg.Payload["content"].(string)
	senderName, _ := msg.Payload["senderName"].(string)

	// Send a message to the group
	_, exists := r.Groups[groupID]
	if !exists {
		log.Printf("Room %s: group %s does not exist", r.ID, groupID)
		return
	}

	chatMsg := map[string]interface{}{
		"type":       message.ChatGroup,
		"groupId":    groupID,
		"senderId":   senderID,
		"senderName": senderName,
		"content":    content,
	}
	data, err := json.Marshal(chatMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group chat message: %v", r.ID, err)
		return
	}
	// Send group message ONLY to ui connections
	r.sendToGroupMembersConnection(groupID, util.ConnUI, data)
}

// handleFollow records that followerID is now following followedID and notifies the followed player.
func (r *Room) handleFollow(msg *Message) {
	followerID, ok := msg.Payload["followerId"].(string)
	if !ok {
		log.Printf("Room %s: invalid followerId in follow message", r.ID)
		return
	}
	followedID, ok := msg.Payload["followedId"].(string)
	if !ok {
		log.Printf("Room %s: invalid followedId in follow message", r.ID)
		return
	}
	if followerID == followedID {
		log.Printf("Room %s: user %s tried to follow themselves, ignoring", r.ID, followerID)
		return
	}
	if _, exists := r.Users[followerID]; !exists {
		log.Printf("Room %s: follower %s not in room", r.ID, followerID)
		return
	}
	if _, exists := r.Users[followedID]; !exists {
		log.Printf("Room %s: followed user %s not in room", r.ID, followedID)
		return
	}

	if r.Followers[followedID] == nil {
		r.Followers[followedID] = make(map[string]bool)
	}
	r.Followers[followedID][followerID] = true
	log.Printf("Room %s: %s is now following %s", r.ID, followerID, followedID)

	r.notifyFollowerUpdate(followedID)
}

// handleUnfollow removes followerID from followedID's follower set and notifies the followed player.
// Used both when a follower stops voluntarily and when the followed player removes a specific follower.
func (r *Room) handleUnfollow(msg *Message) {
	followerID, ok := msg.Payload["followerId"].(string)
	if !ok {
		log.Printf("Room %s: invalid followerId in unfollow message", r.ID)
		return
	}
	followedID, ok := msg.Payload["followedId"].(string)
	if !ok {
		log.Printf("Room %s: invalid followedId in unfollow message", r.ID)
		return
	}

	if r.Followers[followedID] != nil {
		delete(r.Followers[followedID], followerID)
		if len(r.Followers[followedID]) == 0 {
			delete(r.Followers, followedID)
		}
	}
	log.Printf("Room %s: %s unfollowed %s", r.ID, followerID, followedID)

	// Only notify if the followed player is still in the room
	if _, exists := r.Users[followedID]; exists {
		r.notifyFollowerUpdate(followedID)
	}

	// If the followed player initiated this removal (not the follower themselves),
	// notify the follower so their game and banner can stop.
	if msg.ID != followerID {
		if _, stillIn := r.Users[followerID]; stillIn {
			r.sendStopFollowingNotification(followerID, followedID)
		}
	}
}

// handlePlayerLeave processes a player:leave message sent explicitly by the client
// (e.g. browser back button or tab close). It removes the player immediately without
// waiting for both WebSocket connections to time out naturally.
func (r *Room) handlePlayerLeave(msg *Message) {
	clientID := msg.ID
	if clientID == "" {
		log.Printf("Room %s: player:leave message missing client ID", r.ID)
		return
	}
	u, exists := r.Users[clientID]
	if !exists {
		log.Printf("Room %s: player:leave for unknown user %s, ignoring", r.ID, clientID)
		return
	}
	log.Printf("Room %s: explicit player:leave received for user %s", r.ID, clientID)

	// Close all active connections. The ReadPumps will exit and send to UnregisterC,
	// but since we delete the user from r.Users first those events become no-ops.
	if u.GameConn != nil {
		u.GameConn.Close()
	}
	if u.UIConn != nil {
		u.UIConn.Close()
	}

	// Remove the user from the room immediately.
	delete(r.Users, clientID)
	delete(r.playerStates, clientID)
	log.Printf("Room %s: user %s removed via player:leave. Total users: %d", r.ID, clientID, len(r.Users))

	// Broadcast leave to both connection types so all clients update.
	r.broadcastLeaveMessage(util.ConnGame, clientID)
	r.broadcastLeaveMessage(util.ConnUI, clientID)

	// Clean up group memberships.
	r.cleanupPlayerGroups(clientID)

	// Clean up follow relationships.
	for followedID, followers := range r.Followers {
		if followers[clientID] {
			delete(followers, clientID)
			if len(followers) == 0 {
				delete(r.Followers, followedID)
			}
			if _, stillIn := r.Users[followedID]; stillIn {
				r.notifyFollowerUpdate(followedID)
			}
		}
	}
	delete(r.Followers, clientID)

	// Start idle timer if room is now empty.
	if len(r.Users) == 0 && r.idleTimer == nil {
		r.idleTimer = time.NewTimer(r.idleDuration)
		log.Printf("Room %s: idle timer started after player:leave", r.ID)
	}
}

// handleStopAllFollowers clears every follower for the requesting player and notifies them.
func (r *Room) handleStopAllFollowers(msg *Message) {
	followedID, ok := msg.Payload["followedId"].(string)
	if !ok {
		log.Printf("Room %s: invalid followedId in stop_all_followers message", r.ID)
		return
	}
	if _, exists := r.Users[followedID]; !exists {
		log.Printf("Room %s: user %s not in room for stop_all_followers", r.ID, followedID)
		return
	}

	// Collect follower IDs before clearing so we can notify them
	followerIDs := make([]string, 0, len(r.Followers[followedID]))
	for fid := range r.Followers[followedID] {
		followerIDs = append(followerIDs, fid)
	}

	delete(r.Followers, followedID)
	log.Printf("Room %s: %s removed all followers", r.ID, followedID)

	r.notifyFollowerUpdate(followedID)

	// Notify every follower to stop following
	for _, followerID := range followerIDs {
		if _, stillIn := r.Users[followerID]; stillIn {
			r.sendStopFollowingNotification(followerID, followedID)
		}
	}
}

// notifyFollowerUpdate sends the current follower list for followedID to their ui connection.
func (r *Room) notifyFollowerUpdate(followedID string) {
	followers := make([]string, 0)
	for followerID := range r.Followers[followedID] {
		followers = append(followers, followerID)
	}
	updateMsg := message.NewPlayerFollowerUpdateMessage(followers)
	data, err := json.Marshal(updateMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal follower update: %v", r.ID, err)
		return
	}
	r.sendToUserConnection(followedID, util.ConnUI, data)
}

// sendStopFollowingNotification tells a follower's ui connection that the followed player
// has removed them, so they can stop their game following and clear their banner.
func (r *Room) sendStopFollowingNotification(followerID, followedID string) {
	stopMsg := map[string]interface{}{
		"type": message.PlayerStopFollowing,
		"payload": map[string]interface{}{
			"followedId": followedID,
		},
	}
	data, err := json.Marshal(stopMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal stop_following notification: %v", r.ID, err)
		return
	}
	r.sendToUserConnection(followerID, util.ConnUI, data)
}

// getIdleTimerChan returns the idle timer channel or a nil-receiving channel if timer is not active
func (r *Room) getIdleTimerChan() <-chan time.Time {
	if r.idleTimer != nil {
		return r.idleTimer.C
	}
	// Return a nil channel that never receives, so select will never take this case
	return nil
}

// SetIdleTimeout sets the idle duration before room deletion
func (r *Room) SetIdleTimeout(duration time.Duration) {
	r.idleDuration = duration
}

// SetOnEmpty sets the callback when room should be deleted
func (r *Room) SetOnEmpty(callback func()) {
	r.onEmpty = callback
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

	// Send interface messages ONLY to ui connections
	r.sendToAllUsersConnection(util.ConnUI, data)
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
	// Only allow "game" or "ui" types
	if connType != util.ConnGame && connType != util.ConnUI {
		return
	}
	if user, ok := r.Users[userID]; ok {
		user.BroadcastToConnectionType(connType, data)
	}
}

// sendToAllUsersConnection sends data to all users on a specific connection type
func (r *Room) sendToAllUsersConnection(connType string, data []byte) {
	// Only allow "game" or "ui" types
	if connType != util.ConnGame && connType != util.ConnUI {
		return
	}
	for _, user := range r.Users {
		user.BroadcastToConnectionType(connType, data)
	}
}

// sendToAllUsersConnectionButOne sends data to all users except the specified user on a specific connection type
func (r *Room) sendToAllUsersConnectionButOne(senderID string, connType string, data []byte) {
	// Only allow "game" or "ui" types
	if connType != util.ConnGame && connType != util.ConnUI {
		return
	}
	for userID, user := range r.Users {
		if userID == senderID {
			continue
		}
		user.BroadcastToConnectionType(connType, data)
	}
}

// ─── Group Lifecycle Handlers ───────────────────────────────────────────────

// handleGroupCreate creates a new group at the creator's current position
func (r *Room) handleGroupCreate(msg *Message) {
	creatorID, ok := msg.Payload["creatorId"].(string)
	if !ok {
		log.Printf("Room %s: invalid creatorId in group:create", r.ID)
		return
	}
	groupName, ok := msg.Payload["groupName"].(string)
	if !ok || groupName == "" {
		log.Printf("Room %s: invalid groupName in group:create", r.ID)
		return
	}

	// Check creator exists in room
	if _, exists := r.Users[creatorID]; !exists {
		log.Printf("Room %s: creator %s not in room", r.ID, creatorID)
		return
	}

	// Check if player is already in a group
	for _, grp := range r.Groups {
		if grp.HasMember(creatorID) {
			log.Printf("Room %s: user %s already in group %s", r.ID, creatorID, grp.ID)
			return
		}
	}

	// Get creator's current position
	state, stateExists := r.playerStates[creatorID]
	if !stateExists {
		log.Printf("Room %s: no player state for creator %s", r.ID, creatorID)
		return
	}

	// Create the group
	groupID := "grp-" + creatorID + "-" + groupName
	grp := group.NewGroup(groupID, groupName, creatorID, state.X, state.Y)
	grp.AddMember(creatorID)
	r.Groups[groupID] = grp

	log.Printf("Room %s: group '%s' created by %s at (%.0f, %.0f)", r.ID, groupName, creatorID, state.X, state.Y)

	// Notify the creator (UI) with the full group state
	createMsg := message.NewGroupCreateMessage(groupID, groupName, creatorID, state.X, state.Y)
	createData, err := json.Marshal(createMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group:create message: %v", r.ID, err)
		return
	}
	r.sendToUserConnection(creatorID, util.ConnUI, createData)

	// Broadcast group update to ALL game connections so everyone sees the dialog
	updateMsg := message.NewGroupUpdateMessage(groupID, groupName, grp.Members, state.X, state.Y)
	updateData, err := json.Marshal(updateMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group:update message: %v", r.ID, err)
		return
	}
	r.sendToAllUsersConnection(util.ConnGame, updateData)
	// Also send to creator's UI so they get the member list
	r.sendToUserConnection(creatorID, util.ConnUI, updateData)
}

// handleGroupJoin adds a player to an existing group
func (r *Room) handleGroupJoin(msg *Message) {
	clientID, ok := msg.Payload["clientId"].(string)
	if !ok {
		log.Printf("Room %s: invalid clientId in group:join", r.ID)
		return
	}
	groupID, ok := msg.Payload["groupId"].(string)
	if !ok {
		log.Printf("Room %s: invalid groupId in group:join", r.ID)
		return
	}

	// Check user exists
	if _, exists := r.Users[clientID]; !exists {
		log.Printf("Room %s: user %s not in room for group:join", r.ID, clientID)
		return
	}

	// Check if already in another group
	for _, grp := range r.Groups {
		if grp.HasMember(clientID) && grp.ID != groupID {
			log.Printf("Room %s: user %s already in group %s, cannot join %s", r.ID, clientID, grp.ID, groupID)
			return
		}
	}

	grp, exists := r.Groups[groupID]
	if !exists {
		log.Printf("Room %s: group %s does not exist", r.ID, groupID)
		return
	}

	if grp.HasMember(clientID) {
		log.Printf("Room %s: user %s already in group %s", r.ID, clientID, groupID)
		return
	}

	grp.AddMember(clientID)
	log.Printf("Room %s: user %s joined group %s (%d members)", r.ID, clientID, groupID, len(grp.Members))

	// Notify the joining player's UI
	joinMsg := message.NewGroupJoinMessage(groupID, clientID)
	joinData, err := json.Marshal(joinMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group:join message: %v", r.ID, err)
		return
	}
	r.sendToUserConnection(clientID, util.ConnUI, joinData)

	// Broadcast updated group state to all game connections (dialog member count)
	updateMsg := message.NewGroupUpdateMessage(groupID, grp.Name, grp.Members, grp.X, grp.Y)
	updateData, err := json.Marshal(updateMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group:update after join: %v", r.ID, err)
		return
	}
	r.sendToAllUsersConnection(util.ConnGame, updateData)

	// Send group update to ALL members' UI connections so they see the new member
	for _, memberID := range grp.Members {
		r.sendToUserConnection(memberID, util.ConnUI, updateData)
	}
}

// handleGroupLeave removes a player from a group, deletes the group if empty
func (r *Room) handleGroupLeave(msg *Message) {
	clientID, ok := msg.Payload["clientId"].(string)
	if !ok {
		log.Printf("Room %s: invalid clientId in group:leave", r.ID)
		return
	}
	groupID, ok := msg.Payload["groupId"].(string)
	if !ok {
		log.Printf("Room %s: invalid groupId in group:leave", r.ID)
		return
	}

	grp, exists := r.Groups[groupID]
	if !exists {
		log.Printf("Room %s: group %s does not exist for leave", r.ID, groupID)
		return
	}

	if !grp.HasMember(clientID) {
		log.Printf("Room %s: user %s not in group %s", r.ID, clientID, groupID)
		return
	}

	grp.RemoveMember(clientID)
	log.Printf("Room %s: user %s left group %s (%d members remaining)", r.ID, clientID, groupID, len(grp.Members))

	// Notify the leaving player's UI
	leaveMsg := message.NewGroupLeaveMessage(groupID, clientID)
	leaveData, err := json.Marshal(leaveMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group:leave message: %v", r.ID, err)
		return
	}
	r.sendToUserConnection(clientID, util.ConnUI, leaveData)

	// Also notify leaving player's game connection to restore free roam
	r.sendToUserConnection(clientID, util.ConnGame, leaveData)

	// If group is now empty, delete it and notify everyone
	if len(grp.Members) == 0 {
		delete(r.Groups, groupID)
		log.Printf("Room %s: group %s deleted (no members left)", r.ID, groupID)

		deleteMsg := message.NewGroupDeleteMessage(groupID)
		deleteData, err := json.Marshal(deleteMsg)
		if err != nil {
			log.Printf("Room %s: failed to marshal group:delete message: %v", r.ID, err)
			return
		}
		// Broadcast delete to all game connections to remove the dialog
		r.sendToAllUsersConnection(util.ConnGame, deleteData)
		return
	}

	// Group still has members — broadcast updated state
	updateMsg := message.NewGroupUpdateMessage(groupID, grp.Name, grp.Members, grp.X, grp.Y)
	updateData, err := json.Marshal(updateMsg)
	if err != nil {
		log.Printf("Room %s: failed to marshal group:update after leave: %v", r.ID, err)
		return
	}
	r.sendToAllUsersConnection(util.ConnGame, updateData)
	for _, memberID := range grp.Members {
		r.sendToUserConnection(memberID, util.ConnUI, updateData)
	}
}

// cleanupPlayerGroups removes a player from any groups they belong to.
// If a group becomes empty as a result, the group is deleted and all game clients are notified.
func (r *Room) cleanupPlayerGroups(clientID string) {
	for groupID, grp := range r.Groups {
		if !grp.HasMember(clientID) {
			continue
		}
		grp.RemoveMember(clientID)
		log.Printf("Room %s: removed disconnected user %s from group %s", r.ID, clientID, groupID)

		if len(grp.Members) == 0 {
			delete(r.Groups, groupID)
			log.Printf("Room %s: group %s deleted (last member disconnected)", r.ID, groupID)

			deleteMsg := message.NewGroupDeleteMessage(groupID)
			deleteData, _ := json.Marshal(deleteMsg)
			r.sendToAllUsersConnection(util.ConnGame, deleteData)
		} else {
			// Update remaining members
			updateMsg := message.NewGroupUpdateMessage(groupID, grp.Name, grp.Members, grp.X, grp.Y)
			updateData, _ := json.Marshal(updateMsg)
			r.sendToAllUsersConnection(util.ConnGame, updateData)
			for _, memberID := range grp.Members {
				r.sendToUserConnection(memberID, util.ConnUI, updateData)
			}
		}
	}
}
