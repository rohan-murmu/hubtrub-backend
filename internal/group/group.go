package group

type Group struct {
	ID        string   `json:"group_id"`
	Name      string   `json:"group_name"`
	Members   []string `json:"group_members"`    // List of client IDs
	CreatorID string   `json:"group_creator_id"` // Who created the group
	X         float64  `json:"x"`                // World position X where group was created
	Y         float64  `json:"y"`                // World position Y where group was created
}

func NewGroup(id string, name string, creatorID string, x float64, y float64) *Group {
	return &Group{
		ID:        id,
		Name:      name,
		Members:   []string{},
		CreatorID: creatorID,
		X:         x,
		Y:         y,
	}
}

func (g *Group) AddMember(clientID string) {
	for _, member := range g.Members {
		if member == clientID {
			return // Already a member
		}
	}
	g.Members = append(g.Members, clientID)
}

func (g *Group) RemoveMember(clientID string) {
	for i, member := range g.Members {
		if member == clientID {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)
			return
		}
	}
}

func (g *Group) HasMember(clientID string) bool {
	for _, member := range g.Members {
		if member == clientID {
			return true
		}
	}
	return false
}
