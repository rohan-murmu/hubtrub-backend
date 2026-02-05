package model

import "time"

// Room represents a room entity
type Room struct {
	RoomID          string    `json:"roomId"`
	RoomName        string    `json:"roomName"`
	RoomScene       string    `json:"roomScene"`
	RoomDescription string    `json:"roomDescription"`
	RoomAdmin       string    `json:"roomAdmin"`
	RoomCreatedAt   time.Time `json:"roomCreatedAt"`
}
