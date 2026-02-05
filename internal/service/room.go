package service

import (
	"github.com/scythrine05/hubtrub-server/internal/model"
	"github.com/scythrine05/hubtrub-server/internal/repository"
)

// RoomService handles room business logic
type RoomService struct {
	repo repository.RoomRepository
}

// NewRoomService creates a new room service
func NewRoomService(repo repository.RoomRepository) *RoomService {
	return &RoomService{repo: repo}
}

// GetAllRooms retrieves all rooms
func (s *RoomService) GetAllRooms() ([]model.Room, error) {
	return s.repo.GetAllRooms()
}

// GetRoomByID retrieves a room by ID
func (s *RoomService) GetRoomByID(roomID string) (*model.Room, error) {
	return s.repo.GetRoomByID(roomID)
}

// CreateRoom creates a new room
func (s *RoomService) CreateRoom(room *model.Room) error {
	return s.repo.CreateRoom(room)
}

// UpdateRoom updates a room
func (s *RoomService) UpdateRoom(roomID string, room *model.Room) error {
	return s.repo.UpdateRoom(roomID, room)
}

// DeleteRoom deletes a room
func (s *RoomService) DeleteRoom(roomID string) error {
	return s.repo.DeleteRoom(roomID)
}
