package model

// Client represents a client entity (player)
type Client struct {
	ClientID       string `json:"clientId"`
	ClientUserName string `json:"clientUserName"`
	ClientAvatar   string `json:"clientAvatar"`
}
