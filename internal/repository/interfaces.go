package repository

import "github.com/scythrine05/hubtrub-server/internal/model"

// RoomRepository defines the interface for room data operations
type RoomRepository interface {
	GetAllRooms() ([]model.Room, error)
	GetRoomByID(roomID string) (*model.Room, error)
	CreateRoom(room *model.Room) error
	UpdateRoom(roomID string, room *model.Room) error
	DeleteRoom(roomID string) error
}

// ClientRepository defines the interface for client data operations
type ClientRepository interface {
	GetAllClients() ([]model.Client, error)
	GetClientByID(clientID string) (*model.Client, error)
	GetClientByUserName(userName string) (*model.Client, error)
	CreateClient(client *model.Client) error
	UpdateClient(clientID string, client *model.Client) error
	DeleteClient(clientID string) error
}
