package group

type Group struct {
	ID      string   `json:"group_id"`
	Name    string   `json:"group_name"`
	Members []string `json:"group_members"` // List of client IDs
}

func NewGroup(id string, name string) *Group {
	return &Group{
		ID:      id,
		Name:    name,
		Members: []string{},
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
