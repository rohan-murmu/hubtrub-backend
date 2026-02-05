package repository

import (
	"github.com/scythrine05/hubtrub-server/internal/model"
	"github.com/scythrine05/hubtrub-server/internal/util"
)

// FileRoomRepository implements RoomRepository using file storage
type FileRoomRepository struct {
	fr *util.FileRepository
}

// NewFileRoomRepository creates a new file-based room repository
func NewFileRoomRepository(filePath string) RoomRepository {
	return &FileRoomRepository{
		fr: util.NewFileRepository(filePath),
	}
}

// GetAllRooms returns all rooms from file
func (r *FileRoomRepository) GetAllRooms() ([]model.Room, error) {
	var rooms []model.Room
	err := r.fr.ReadAll(&rooms)
	return rooms, err
}

// GetRoomByID returns a specific room by ID
func (r *FileRoomRepository) GetRoomByID(roomID string) (*model.Room, error) {
	var room model.Room
	err := r.fr.ReadByID(roomID, "roomId", &room)
	if err != nil {
		return nil, err
	}
	return &room, nil
}

// CreateRoom creates a new room
func (r *FileRoomRepository) CreateRoom(room *model.Room) error {
	return r.fr.Write(room)
}

// UpdateRoom updates an existing room
func (r *FileRoomRepository) UpdateRoom(roomID string, room *model.Room) error {
	return r.fr.Update(roomID, "roomId", room)
}

// DeleteRoom deletes a room by ID
func (r *FileRoomRepository) DeleteRoom(roomID string) error {
	return r.fr.Delete(roomID, "roomId")
}
